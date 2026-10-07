package lvr_test

import (
	"archive/zip"
	"bytes"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

// testdata/a_lvr_land_a_sample.csv contains authored synthetic parser scenarios, not downloaded transaction records.
func parseSample(t *testing.T) lvr.SalesResult {
	t.Helper()
	file, err := os.Open("testdata/a_lvr_land_a_sample.csv")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()
	result, err := lvr.ParseSales(file, "a", "115S2")
	if err != nil {
		t.Fatalf("ParseSales returned an error: %v", err)
	}
	return result
}

func findTransaction(t *testing.T, result lvr.SalesResult, id string) domain.Transaction {
	t.Helper()
	for _, transaction := range result.Transactions {
		if transaction.ID == id {
			return transaction
		}
	}
	t.Fatalf("transaction %s not found", id)
	return domain.Transaction{}
}

func TestParseSalesSkipsNonBuildingRows(t *testing.T) {
	result := parseSample(t)
	if result.Rows != 9 {
		t.Fatalf("expected 9 data rows (English header skipped), got %d", result.Rows)
	}
	if len(result.Transactions) != 7 {
		t.Fatalf("expected 7 transactions, got %d", len(result.Transactions))
	}
	if result.Skipped[lvr.SkipNoBuilding] != 1 || result.Skipped[lvr.SkipParkingOnly] != 1 {
		t.Fatalf("unexpected skipped counts: %#v", result.Skipped)
	}
}

func TestParseSalesNormalizesPlainSale(t *testing.T) {
	transaction := findTransaction(t, parseSample(t), "SYN-S-001")
	completedAt := time.Date(1980, 1, 10, 0, 0, 0, 0, time.UTC)
	want := domain.Transaction{
		ID:               "SYN-S-001",
		CityCode:         "a",
		District:         "中山區",
		Address:          "台北市中山區示範一路133巷12號六樓之11",
		TransactionType:  "房地(土地+建物)",
		BuildingType:     "華廈(10層含以下有電梯)",
		MainUse:          "住家用",
		TotalPrice:       8_000_000,
		BuildingPrice:    8_000_000,
		BuildingAreaPing: 10.14,
		PricePerPing:     788_736,
		Rooms:            1,
		Halls:            1,
		Baths:            1,
		FloorLabel:       "六層",
		TotalFloors:      6,
		CompletedAt:      &completedAt,
		TransactedAt:     time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		Season:           "115S2",
	}
	if !reflect.DeepEqual(transaction, want) {
		t.Fatalf("unexpected transaction:\n got %#v\nwant %#v", transaction, want)
	}
}

func TestParseSalesExcludesParkingFromUnitPrice(t *testing.T) {
	transaction := findTransaction(t, parseSample(t), "SYN-S-002")
	if transaction.ParkingPrice != 1_800_000 || transaction.BuildingPrice != 11_960_000 {
		t.Fatalf("unexpected prices: %#v", transaction)
	}
	if transaction.BuildingAreaPing != 12.65 || transaction.PricePerPing != 945_189 {
		t.Fatalf("unexpected area or unit price: %v ping, %v per ping", transaction.BuildingAreaPing, transaction.PricePerPing)
	}
	if transaction.TotalFloors != 13 {
		t.Fatalf("expected 13 total floors, got %d", transaction.TotalFloors)
	}
}

func TestParseSalesFlagsSpecialRemarks(t *testing.T) {
	result := parseSample(t)
	cases := map[string][]string{
		"SYN-S-003": {lvr.SpecialRelatedParty},
		"SYN-S-004": {lvr.SpecialSplitRegistration},
		"SYN-S-005": nil, // 其他增建 does not distort the price
	}
	for id, want := range cases {
		if got := findTransaction(t, result, id).SpecialReasons; !slices.Equal(got, want) {
			t.Fatalf("%s: expected special reasons %v, got %v", id, want, got)
		}
	}
}

func TestParseSalesFillsMissingCityAndDistrict(t *testing.T) {
	transaction := findTransaction(t, parseSample(t), "SYN-S-006")
	if transaction.Address != "台北市士林區示範四路280之1號" {
		t.Fatalf("unexpected address %q", transaction.Address)
	}
}

func TestNormalizesPreMunicipalityAddresses(t *testing.T) {
	header := "鄉鎮市區,交易標的,土地位置建物門牌,交易年月日,移轉層次,總樓層數,建物型態,主要用途,建築完成年月," +
		"建物移轉總面積平方公尺,建物現況格局-房,建物現況格局-廳,建物現況格局-衛,總價元,車位移轉總面積平方公尺,車位總價元,備註,編號\n"
	rows := strings.Join([]string{
		"龜山區,房地(土地+建物),桃園縣龜山鄉德明路77巷9號,1020105,三層,五層,公寓,住家用,,100,3,2,1,5000000,0,0,,H1",
		"平鎮區,房地(土地+建物),桃園縣平鎮市平鎮市平安南街67號4F-1,1020105,四層,五層,公寓,住家用,,100,3,2,1,5000000,0,0,二等親買賣,H2",
		"桃園區,房地(土地+建物),桃園市桃園區中正路1號,1080105,四層,五層,公寓,住家用,,100,3,2,1,5000000,0,0,,H3",
		"桃園區,房地(土地+建物),桃園市中正路2號,1080105,四層,五層,公寓,住家用,,100,3,2,1,1,0,0,有民情風俗因素之交易,H4",
	}, "\n")
	result, err := lvr.ParseSales(strings.NewReader(header+rows), "h", "102S1")
	if err != nil {
		t.Fatalf("ParseSales returned an error: %v", err)
	}
	want := map[string]string{"H1": "桃園市龜山區德明路77巷9號", "H2": "桃園市平鎮區平安南街67號4F-1", "H3": "桃園市桃園區中正路1號"}
	for id, address := range want {
		if got := findTransaction(t, result, id); got.Address != address {
			t.Fatalf("%s: expected %q, got %q", id, address, got.Address)
		}
	}
	if reasons := findTransaction(t, result, "H2").SpecialReasons; !slices.Equal(reasons, []string{lvr.SpecialRelatedParty}) {
		t.Fatalf("expected 二等親 to flag a related party, got %v", reasons)
	}
	if result.Skipped[lvr.SkipInvalidPrice] != 1 {
		t.Fatalf("expected the NT$1 transfer to be skipped, got %#v", result.Skipped)
	}
}

func TestParseSalesHandlesPartialCompletionDates(t *testing.T) {
	result := parseSample(t)
	if completedAt := findTransaction(t, result, "SYN-S-005").CompletedAt; completedAt != nil {
		t.Fatalf("expected no completion date, got %v", completedAt)
	}
	transaction := findTransaction(t, result, "SYN-S-007")
	if transaction.CompletedAt == nil || !transaction.CompletedAt.Equal(time.Date(1921, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 1921-09-01 from YYYMM, got %v", transaction.CompletedAt)
	}
	if transaction.TotalFloors != 0 {
		t.Fatalf("expected unknown total floors, got %d", transaction.TotalFloors)
	}
}

func TestParseSalesSkipsInvalidRows(t *testing.T) {
	header := "鄉鎮市區,交易標的,土地位置建物門牌,交易年月日,移轉層次,總樓層數,建物型態,主要用途,建築完成年月," +
		"建物移轉總面積平方公尺,建物現況格局-房,建物現況格局-廳,建物現況格局-衛,總價元,車位移轉總面積平方公尺,車位總價元,備註,編號\n"
	rows := []string{
		"大安區,房地(土地+建物)+車位,臺北市大安區x,1150101,,,其他,停車空間,,30,0,0,0,1000000,30,1000000,,parking-use",
		"大安區,房地(土地+建物),臺北市大安區x,1150231,,,其他,住家用,,30,0,0,0,1000000,0,0,,bad-date",
		"大安區,房地(土地+建物),臺北市大安區x,1150101,,,其他,住家用,,30,0,0,0,0,0,0,,zero-price",
		"大安區,房地(土地+建物),臺北市大安區x,1150101,,,其他,住家用,,3,0,0,0,1000000,0,0,,tiny-area",
		"大安區,房地(土地+建物),臺北市大安區x,1150101,,,其他,住家用,,30,x,0,0,1000000,0,0,,bad-rooms",
	}
	result, err := lvr.ParseSales(strings.NewReader(header+strings.Join(rows, "\n")), "a", "115S1")
	if err != nil {
		t.Fatalf("ParseSales returned an error: %v", err)
	}
	want := map[lvr.SkipReason]int{
		lvr.SkipParkingOnly: 1, lvr.SkipInvalidDate: 1, lvr.SkipInvalidPrice: 1, lvr.SkipInvalidArea: 1, lvr.SkipMalformed: 1,
	}
	if len(result.Transactions) != 0 || result.Rows != 5 || !maps.Equal(result.Skipped, want) {
		t.Fatalf("unexpected result: %d transactions, %d rows, skipped %#v", len(result.Transactions), result.Rows, result.Skipped)
	}
}

func TestParseSalesAcceptsOlderColumnNames(t *testing.T) {
	header := "鄉鎮市區,交易標的,土地位置建物門牌,交易年月日,移轉層次,總樓層數,建物型態,主要用途,建築完成年月," +
		"建物移轉總面積平方公尺,建物現況格局-房,建物現況格局-廳,建物現況格局-衛,總價元,車位移轉總面積(平方公尺),車位總價元,備註,編號\n"
	row := "中正區,房地(土地+建物)+車位,臺北市中正區和平西路2段70巷8弄16號,1050105,三層,五層,公寓(5樓含以下無電梯),住家用,0700101,100,3,2,1,20000000,30,2000000,,OLD1"
	result, err := lvr.ParseSales(strings.NewReader(header+row), "a", "105S1")
	if err != nil {
		t.Fatalf("ParseSales returned an error: %v", err)
	}
	if len(result.Transactions) != 1 || result.Transactions[0].BuildingAreaPing != 21.17 {
		t.Fatalf("expected parking area to be subtracted, got %#v", result.Transactions)
	}
}

func TestParseSalesRejectsInvalidInput(t *testing.T) {
	cases := map[string]struct {
		input, city, season string
	}{
		"missing columns": {input: "鄉鎮市區,編號\n", city: "a", season: "115S2"},
		"unknown city":    {input: "", city: "l", season: "115S2"},
		"invalid season":  {input: "", city: "a", season: "2026Q2"},
	}
	for name, testCase := range cases {
		if _, err := lvr.ParseSales(strings.NewReader(testCase.input), testCase.city, testCase.season); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

func TestOpenSalesReadsCityFileFromArchive(t *testing.T) {
	fixture, err := os.ReadFile("testdata/a_lvr_land_a_sample.csv")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	buffer := bytes.Buffer{}
	writer := zip.NewWriter(&buffer)
	entry, _ := writer.Create(lvr.SalesFileName("a"))
	_, _ = entry.Write(fixture)
	if err = writer.Close(); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}

	file, err := lvr.OpenSales(archive, "a")
	if err != nil {
		t.Fatalf("OpenSales returned an error: %v", err)
	}
	defer file.Close()
	result, err := lvr.ParseSales(file, "a", "115S2")
	if err != nil || len(result.Transactions) != 7 {
		t.Fatalf("expected 7 transactions from archive, got %d (err %v)", len(result.Transactions), err)
	}
	if _, err = lvr.OpenSales(archive, "f"); err == nil {
		t.Fatal("expected an error for a city missing from the archive")
	}
}
