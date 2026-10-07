// Package lvr parses the MOI actual price registration (實價登錄) open data.
package lvr

import (
	"archive/zip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/twtext"
)

// Cities maps the open data file prefix to its city or county.
var Cities = map[string]string{
	"a": "台北市", "b": "台中市", "c": "基隆市", "d": "台南市", "e": "高雄市", "f": "新北市",
	"g": "宜蘭縣", "h": "桃園市", "i": "嘉義市", "j": "新竹縣", "k": "苗栗縣", "m": "南投縣",
	"n": "彰化縣", "o": "新竹市", "p": "雲林縣", "q": "嘉義縣", "t": "屏東縣", "u": "花蓮縣",
	"v": "台東縣", "w": "金門縣", "x": "澎湖縣", "z": "連江縣",
}

// LegacyCounties are the county names used before a city became a special municipality
// (新北 2010, 台中/台南/高雄 2010, 桃園 2014); early 實價登錄 addresses still use them.
var LegacyCounties = map[string]string{"b": "台中縣", "d": "台南縣", "e": "高雄縣", "f": "台北縣", "h": "桃園縣"}

var seasonPattern = regexp.MustCompile(`^\d{3}S[1-4]$`)

// minPricePerPing rejects nominal prices such as NT$1 transfers between relatives.
const minPricePerPing = 1_000

type SkipReason string

const (
	SkipNoBuilding   SkipReason = "no_building"
	SkipParkingOnly  SkipReason = "parking_only"
	SkipInvalidDate  SkipReason = "invalid_date"
	SkipInvalidPrice SkipReason = "invalid_price"
	SkipInvalidArea  SkipReason = "invalid_area"
	SkipMalformed    SkipReason = "malformed"
)

// Special reasons flag transactions whose price is kept but should not drive market statistics.
const (
	SpecialRelatedParty       = "related_party"
	SpecialSplitRegistration  = "split_registration"
	SpecialGovernmentSale     = "government_sale"
	SpecialSuperficies        = "superficies"
	SpecialIncludesLease      = "includes_lease"
	SpecialRedevelopment      = "redevelopment"
	SpecialPublicFacilityLand = "public_facility_land"
	SpecialDistressedSale     = "distressed_sale"
	SpecialBuildingOnly       = "building_only"
	SpecialPriceOutlier       = "price_outlier"
	// SpecialInvalidDate marks a sale dated after its registration could have happened,
	// e.g. 2101-03-03 for a sale registered in 2021: a typo in the source.
	SpecialInvalidDate = "invalid_date"
)

var specialRemarks = []struct {
	keyword string
	reason  string
}{
	{"特殊關係", SpecialRelatedParty},
	{"親友", SpecialRelatedParty},
	{"親等", SpecialRelatedParty}, // 一親等, 三親等
	{"等親", SpecialRelatedParty}, // 二等親
	{"親屬", SpecialRelatedParty},
	{"贈與", SpecialRelatedParty},
	{"分件登記", SpecialSplitRegistration},
	{"政府機關標讓售", SpecialGovernmentSale},
	{"協議價購", SpecialGovernmentSale},
	{"地上權", SpecialSuperficies},
	{"含租約", SpecialIncludesLease},
	{"都更", SpecialRedevelopment},
	{"重劃", SpecialRedevelopment},
	{"權利變換", SpecialRedevelopment},
	{"公共設施保留地", SpecialPublicFacilityLand},
	{"急買急賣", SpecialDistressedSale},
	{"債權債務", SpecialDistressedSale},
	{"法拍", SpecialDistressedSale},
	{"有屋無地", SpecialBuildingOnly},
	{"無土地持分", SpecialBuildingOnly},
}

const (
	columnDistrict        = "鄉鎮市區"
	columnTransactionType = "交易標的"
	columnAddress         = "土地位置建物門牌"
	columnTransactedAt    = "交易年月日"
	columnFloor           = "移轉層次"
	columnTotalFloors     = "總樓層數"
	columnBuildingType    = "建物型態"
	columnMainUse         = "主要用途"
	columnCompletedAt     = "建築完成年月"
	columnBuildingArea    = "建物移轉總面積平方公尺"
	columnRooms           = "建物現況格局-房"
	columnHalls           = "建物現況格局-廳"
	columnBaths           = "建物現況格局-衛"
	columnTotalPrice      = "總價元"
	columnParkingArea     = "車位移轉總面積平方公尺"
	columnParkingPrice    = "車位總價元"
	columnRemarks         = "備註"
	columnID              = "編號"
)

var requiredColumns = []string{
	columnDistrict, columnTransactionType, columnAddress, columnTransactedAt, columnFloor, columnTotalFloors,
	columnBuildingType, columnMainUse, columnCompletedAt, columnBuildingArea, columnRooms, columnHalls, columnBaths,
	columnTotalPrice, columnParkingArea, columnParkingPrice, columnRemarks, columnID,
}

// SalesResult holds the usable transactions of one sales file and why the other rows were skipped.
type SalesResult struct {
	Transactions []domain.Transaction
	Rows         int
	Skipped      map[SkipReason]int
}

// SalesFileName returns the main sales (不動產買賣) file name inside the season archive.
func SalesFileName(cityCode string) string { return cityCode + "_lvr_land_a.csv" }

// OpenSales opens a city's main sales file inside a season archive.
func OpenSales(archive *zip.Reader, cityCode string) (io.ReadCloser, error) {
	if _, ok := Cities[cityCode]; !ok {
		return nil, fmt.Errorf("unknown city code %q", cityCode)
	}
	file, err := archive.Open(SalesFileName(cityCode))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", SalesFileName(cityCode), err)
	}
	return file, nil
}

// ParseSales reads a main sales CSV and returns normalized building transactions.
func ParseSales(reader io.Reader, cityCode, season string) (SalesResult, error) {
	cityName, ok := Cities[cityCode]
	if !ok {
		return SalesResult{}, fmt.Errorf("unknown city code %q", cityCode)
	}
	if !seasonPattern.MatchString(season) {
		return SalesResult{}, fmt.Errorf("invalid season %q, expected e.g. 115S2", season)
	}

	result := SalesResult{Skipped: map[SkipReason]int{}}
	rows, malformed, err := scanRows(reader, requiredColumns, "sales", func(row rowFunc) {
		transaction, reason := normalizeSale(row, cityCode, cityName, season)
		if reason != "" {
			result.Skipped[reason]++
			return
		}
		result.Transactions = append(result.Transactions, transaction)
	})
	if err != nil {
		return SalesResult{}, err
	}
	result.Rows = rows
	if malformed > 0 {
		result.Skipped[SkipMalformed] += malformed
	}
	return result, nil
}

// scanRows reads a 實價登錄 CSV and calls handle for each data row. It skips the second
// header line of English column names and counts rows shorter than the header as
// malformed without handling them; rows counts both.
func scanRows(reader io.Reader, required []string, kind string, handle func(row rowFunc)) (rows, malformed int, err error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	header, err := csvReader.Read()
	if err != nil {
		return 0, 0, fmt.Errorf("read header: %w", err)
	}
	columns, err := indexColumns(header, required, kind)
	if err != nil {
		return 0, 0, err
	}
	for {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			return rows, malformed, nil
		}
		if err != nil {
			return 0, 0, fmt.Errorf("read row %d: %w", rows+1, err)
		}
		if len(record) < len(header) {
			rows++
			malformed++
			continue
		}
		row := rowFunc(func(column string) string {
			index, ok := columns[column]
			if !ok {
				return "" // an optional column missing from older seasons
			}
			return strings.TrimSpace(record[index])
		})
		if row(columnID) == "serial number" {
			continue // the second header line holds English column names
		}
		rows++
		handle(row)
	}
}

type rowFunc func(column string) string

func indexColumns(header, required []string, kind string) (map[string]int, error) {
	columns := make(map[string]int, len(header))
	for index, name := range header {
		columns[columnKey(name)] = index
	}
	var missing []string
	for _, column := range required {
		if _, ok := columns[column]; !ok {
			missing = append(missing, column)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s file is missing columns: %s", kind, strings.Join(missing, ", "))
	}
	return columns, nil
}

// columnKey drops the BOM and parentheses; older seasons name a column
// 車位移轉總面積(平方公尺) instead of 車位移轉總面積平方公尺.
func columnKey(name string) string {
	return strings.NewReplacer("\ufeff", "", "(", "", ")", "", "（", "", "）", "").Replace(strings.TrimSpace(name))
}

func normalizeSale(row rowFunc, cityCode, cityName, season string) (domain.Transaction, SkipReason) {
	remarks := twtext.Normalize(row(columnRemarks))
	transactionType := row(columnTransactionType)
	if transactionType == "車位" || row(columnMainUse) == "停車空間" || strings.Contains(remarks, "僅車位") {
		return domain.Transaction{}, SkipParkingOnly
	}
	if !strings.Contains(transactionType, "建物") {
		return domain.Transaction{}, SkipNoBuilding
	}

	transactedAt, err := parseROCDate(row(columnTransactedAt))
	if err != nil {
		return domain.Transaction{}, SkipInvalidDate
	}

	totalPrice, totalErr := parseInt(row(columnTotalPrice))
	parkingPrice, parkingErr := parseInt(row(columnParkingPrice))
	if totalErr != nil || parkingErr != nil || parkingPrice < 0 || totalPrice-parkingPrice <= 0 {
		return domain.Transaction{}, SkipInvalidPrice
	}

	buildingArea, buildingErr := parseFloat(row(columnBuildingArea))
	parkingArea, parkingAreaErr := parseFloat(row(columnParkingArea))
	areaPing := (buildingArea - parkingArea) / squareMetersPerPing
	if buildingErr != nil || parkingAreaErr != nil || parkingArea < 0 || areaPing < 1 {
		return domain.Transaction{}, SkipInvalidArea
	}

	if float64(totalPrice-parkingPrice)/areaPing < minPricePerPing {
		return domain.Transaction{}, SkipInvalidPrice
	}

	rooms, roomsErr := parseInt(row(columnRooms))
	halls, hallsErr := parseInt(row(columnHalls))
	baths, bathsErr := parseInt(row(columnBaths))
	if roomsErr != nil || hallsErr != nil || bathsErr != nil {
		return domain.Transaction{}, SkipMalformed
	}

	district := twtext.Normalize(row(columnDistrict))
	transaction := domain.Transaction{
		ID:               row(columnID),
		CityCode:         cityCode,
		District:         district,
		Address:          normalizeAddress(row(columnAddress), cityName, LegacyCounties[cityCode], district),
		TransactionType:  transactionType,
		BuildingType:     row(columnBuildingType),
		MainUse:          row(columnMainUse),
		TotalPrice:       totalPrice,
		ParkingPrice:     parkingPrice,
		BuildingPrice:    totalPrice - parkingPrice,
		BuildingAreaPing: math.Round(areaPing*100) / 100,
		PricePerPing:     math.Round(float64(totalPrice-parkingPrice) / areaPing),
		Rooms:            int(rooms),
		Halls:            int(halls),
		Baths:            int(baths),
		FloorLabel:       twtext.Normalize(row(columnFloor)),
		TransactedAt:     transactedAt,
		Remarks:          remarks,
		SpecialReasons:   specialReasons(remarks),
		Season:           season,
	}
	if transaction.ID == "" {
		return domain.Transaction{}, SkipMalformed
	}
	if totalFloors, ok := parseFloorCount(row(columnTotalFloors)); ok {
		transaction.TotalFloors = totalFloors
	}
	if completedAt, err := parseROCDate(row(columnCompletedAt)); err == nil {
		transaction.CompletedAt = &completedAt
	}
	return transaction, ""
}

// normalizeAddress makes the address geocodable: half-width digits, a leading city and
// district, and no pre-municipality names such as 桃園縣龜山鄉 (now 桃園市龜山區).
func normalizeAddress(address, cityName, legacyCounty, district string) string {
	address = strings.TrimPrefix(twtext.Normalize(address), cityName)
	if legacyCounty != "" {
		address = strings.TrimPrefix(address, legacyCounty)
	}
	// Old townships share the district's name with 鄉, 鎮 or 市; some rows repeat them.
	base := strings.TrimSuffix(district, "區")
	for trimmed := true; trimmed; {
		trimmed = false
		for _, prefix := range []string{district, base + "鄉", base + "鎮", base + "市"} {
			if base != "" && strings.HasPrefix(address, prefix) {
				address, trimmed = strings.TrimPrefix(address, prefix), true
			}
		}
	}
	return cityName + district + address
}

func specialReasons(remarks string) []string {
	var reasons []string
	for _, rule := range specialRemarks {
		if strings.Contains(remarks, rule.keyword) && !slices.Contains(reasons, rule.reason) {
			reasons = append(reasons, rule.reason)
		}
	}
	return reasons
}

func parseInt(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func parseFloat(value string) (float64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}
