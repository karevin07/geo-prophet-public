package lvr

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/twtext"
)

// Rental special reasons; related-party leases reuse SpecialRelatedParty.
const (
	// SpecialPartialArea marks a lease of part of the registered area, such as a room,
	// whose rent does not match the area.
	SpecialPartialArea = "partial_area"
	// SpecialMultipleUnits marks one rent covering several door numbers.
	SpecialMultipleUnits = "multiple_units"
	// SpecialVariableRent marks leases whose rent changes during the term.
	SpecialVariableRent = "variable_rent"
)

// WholeUnitRentalType is the 出租型態 of a whole home; the others are 獨立套房, 分租套房,
// 分租雅房 and 分層出租.
const WholeUnitRentalType = "整棟(戶)出租"

// minWholeUnitPing is the smallest area inferred to be a whole home when a lease has no
// rental type. On leases from 112S4 to 115S2 that do have one, the rule agrees with it
// for about 92% of whole homes, mistaking mostly 獨立套房 for them.
const minWholeUnitPing = 15

// minMonthlyRent rejects nominal rents such as NT$1 leases.
const minMonthlyRent = 1_000

var rentalSpecialRemarks = []struct {
	keyword string
	reason  string
}{
	{"特殊關係", SpecialRelatedParty},
	{"親友", SpecialRelatedParty},
	{"親屬", SpecialRelatedParty},
	{"部分範圍", SpecialPartialArea},
	{"部分租賃", SpecialPartialArea},
	{"部分承租", SpecialPartialArea},
	{"多人共同承租", SpecialPartialArea},
	{"多個門牌", SpecialMultipleUnits},
	{"約定不同租金", SpecialVariableRent},
}

// partOfHomeRemarks mark leases of a room or part of a home when the rental type is unknown.
var partOfHomeRemarks = []string{"部分範圍", "部分租賃", "部分承租", "多人共同承租", "套房", "雅房"}

const (
	columnLeasedAt          = "租賃年月日"
	columnLeaseFloor        = "租賃層次"
	columnTotalArea         = "建物總面積平方公尺"
	columnRent              = "總額元"
	columnParkingLeaseArea  = "車位面積平方公尺"
	columnParkingRent       = "車位總額元"
	columnFurnished         = "有無附傢俱"
	columnRentalType        = "出租型態"
	columnManager           = "有無管理員"
	columnLeasePeriod       = "租賃期間"
	columnElevator          = "有無電梯"
	columnEquipment         = "附屬設備"
	columnRentalHousingServ = "租賃住宅服務"
)

// requiredRentalColumns are present in every season; the six columns added in 112S4
// (出租型態 to 租賃住宅服務) are optional.
var requiredRentalColumns = []string{
	columnDistrict, columnTransactionType, columnAddress, columnLeasedAt, columnLeaseFloor, columnTotalFloors,
	columnBuildingType, columnMainUse, columnCompletedAt, columnTotalArea, columnRooms, columnHalls, columnBaths,
	columnFurnished, columnRent, columnParkingLeaseArea, columnParkingRent, columnRemarks, columnID,
}

// RentalsResult holds the usable leases of one rental file and why the other rows were skipped.
type RentalsResult struct {
	Rentals []domain.Rental
	Rows    int
	Skipped map[SkipReason]int
}

// RentalsFileName returns the main rental (租賃) file name inside the season archive.
func RentalsFileName(cityCode string) string { return cityCode + "_lvr_land_c.csv" }

// OpenRentals opens a city's main rental file inside a season archive.
func OpenRentals(archive *zip.Reader, cityCode string) (io.ReadCloser, error) {
	if _, ok := Cities[cityCode]; !ok {
		return nil, fmt.Errorf("unknown city code %q", cityCode)
	}
	file, err := archive.Open(RentalsFileName(cityCode))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", RentalsFileName(cityCode), err)
	}
	return file, nil
}

// ParseRentals reads a main rental CSV and returns normalized building leases.
func ParseRentals(reader io.Reader, cityCode, season string) (RentalsResult, error) {
	cityName, ok := Cities[cityCode]
	if !ok {
		return RentalsResult{}, fmt.Errorf("unknown city code %q", cityCode)
	}
	if !seasonPattern.MatchString(season) {
		return RentalsResult{}, fmt.Errorf("invalid season %q, expected e.g. 115S2", season)
	}
	result := RentalsResult{Skipped: map[SkipReason]int{}}
	rows, malformed, err := scanRows(reader, requiredRentalColumns, "rental", func(row rowFunc) {
		rental, reason := normalizeRental(row, cityCode, cityName, season)
		if reason != "" {
			result.Skipped[reason]++
			return
		}
		result.Rentals = append(result.Rentals, rental)
	})
	if err != nil {
		return RentalsResult{}, err
	}
	result.Rows = rows
	if malformed > 0 {
		result.Skipped[SkipMalformed] += malformed
	}
	return result, nil
}

func normalizeRental(row rowFunc, cityCode, cityName, season string) (domain.Rental, SkipReason) {
	remarks := twtext.Normalize(row(columnRemarks))
	transactionType := row(columnTransactionType)
	if transactionType == "車位" || row(columnMainUse) == "停車空間" {
		return domain.Rental{}, SkipParkingOnly
	}
	// 租賃房屋 since 112S4, 建物 or 房地(土地+建物) before.
	if !strings.Contains(transactionType, "建物") && !strings.Contains(transactionType, "房屋") {
		return domain.Rental{}, SkipNoBuilding
	}

	leasedAt, err := parseROCDate(row(columnLeasedAt))
	if err != nil {
		return domain.Rental{}, SkipInvalidDate
	}

	rent, rentErr := parseInt(row(columnRent))
	parkingRent, parkingErr := parseInt(row(columnParkingRent))
	if rentErr != nil || parkingErr != nil || parkingRent < 0 || rent < minMonthlyRent || rent-parkingRent <= 0 {
		return domain.Rental{}, SkipInvalidPrice
	}

	totalArea, areaErr := parseFloat(row(columnTotalArea))
	parkingArea, parkingAreaErr := parseFloat(row(columnParkingLeaseArea))
	areaPing := (totalArea - parkingArea) / squareMetersPerPing
	if areaErr != nil || parkingAreaErr != nil || parkingArea < 0 || areaPing < 1 {
		return domain.Rental{}, SkipInvalidArea
	}

	rooms, roomsErr := parseInt(row(columnRooms))
	halls, hallsErr := parseInt(row(columnHalls))
	baths, bathsErr := parseInt(row(columnBaths))
	if roomsErr != nil || hallsErr != nil || bathsErr != nil {
		return domain.Rental{}, SkipMalformed
	}

	district := twtext.Normalize(row(columnDistrict))
	service := row(columnRentalHousingServ)
	rental := domain.Rental{
		ID:               row(columnID),
		CityCode:         cityCode,
		District:         district,
		Address:          normalizeAddress(row(columnAddress), cityName, LegacyCounties[cityCode], district),
		TransactionType:  transactionType,
		RentalType:       row(columnRentalType),
		RentalService:    service,
		SocialHousing:    strings.HasPrefix(service, "社會住宅") || strings.Contains(remarks, "社會住宅") || strings.Contains(remarks, "社宅"),
		BuildingType:     row(columnBuildingType),
		MainUse:          row(columnMainUse),
		MonthlyRent:      rent,
		ParkingRent:      parkingRent,
		BuildingRent:     rent - parkingRent,
		BuildingAreaPing: math.Round(areaPing*100) / 100,
		RentPerPing:      math.Round(float64(rent-parkingRent)/areaPing*10) / 10,
		Rooms:            int(rooms),
		Halls:            int(halls),
		Baths:            int(baths),
		FloorLabel:       twtext.Normalize(row(columnLeaseFloor)),
		LeasedAt:         leasedAt,
		Furnished:        row(columnFurnished) == "有",
		HasManager:       yesNo(row(columnManager)),
		HasElevator:      yesNo(row(columnElevator)),
		Equipment:        row(columnEquipment),
		Renewal:          strings.Contains(remarks, "續租"),
		Remarks:          remarks,
		Season:           season,
	}
	if rental.ID == "" {
		return domain.Rental{}, SkipMalformed
	}
	for _, rule := range rentalSpecialRemarks {
		if strings.Contains(remarks, rule.keyword) && !slices.Contains(rental.SpecialReasons, rule.reason) {
			rental.SpecialReasons = append(rental.SpecialReasons, rule.reason)
		}
	}
	if rental.RentalType != "" {
		rental.WholeUnit = rental.RentalType == WholeUnitRentalType
	} else {
		rental.WholeUnitInferred = true
		rental.WholeUnit = areaPing >= minWholeUnitPing && !slices.ContainsFunc(partOfHomeRemarks, func(keyword string) bool {
			return strings.Contains(remarks, keyword)
		})
	}
	if totalFloors, ok := parseFloorCount(row(columnTotalFloors)); ok {
		rental.TotalFloors = totalFloors
	}
	if completedAt, err := parseROCDate(row(columnCompletedAt)); err == nil {
		rental.CompletedAt = &completedAt
	}
	// 租賃期間 reads 1150401~1160331.
	if start, end, ok := strings.Cut(row(columnLeasePeriod), "~"); ok {
		startAt, startErr := parseROCDate(start)
		endAt, endErr := parseROCDate(end)
		if startErr == nil && endErr == nil && !endAt.Before(startAt) {
			rental.LeaseStart, rental.LeaseEnd = &startAt, &endAt
		}
	}
	return rental, ""
}

func yesNo(value string) *bool {
	switch value {
	case "有":
		return new(true)
	case "無":
		return new(false)
	}
	return nil
}

// MarkRentOutliers flags whole-home leases whose rent per ping lies beyond 3 IQRs of
// their district's quartiles, computed like MarkPriceOutliers from whole-home leases
// that are not special. Rooms are left alone: their rent per ping is not comparable.
func MarkRentOutliers(rentals []domain.Rental) {
	flags := outlierFlags(len(rentals), func(index int) (string, float64, bool, bool) {
		rental := rentals[index]
		return rental.CityCode + rental.District, rental.RentPerPing, rental.WholeUnit && !rental.IsSpecial(), rental.WholeUnit
	})
	for index, outlier := range flags {
		if outlier && !slices.Contains(rentals[index].SpecialReasons, SpecialPriceOutlier) {
			rentals[index].SpecialReasons = append(rentals[index].SpecialReasons, SpecialPriceOutlier)
		}
	}
}

// MarkInvalidLeaseDates flags leases dated after latest, the last day their registration
// could have been made.
func MarkInvalidLeaseDates(rentals []domain.Rental, latest time.Time) {
	for index := range rentals {
		rental := &rentals[index]
		if rental.LeasedAt.After(latest) && !slices.Contains(rental.SpecialReasons, SpecialInvalidDate) {
			rental.SpecialReasons = append(rental.SpecialReasons, SpecialInvalidDate)
		}
	}
}
