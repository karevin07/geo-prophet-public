package domain

import "time"

// EstimateQuery describes a home to value from nearby comparable sales.
type EstimateQuery struct {
	// Address is the door number such as 台北市大安區仁愛路四段27巷3號; it identifies
	// sales in the same building and, with its city and district, the price index.
	Address  string
	Lat, Lng float64
	Category string  // 住宅大樓, 華廈, 公寓 or 透天厝
	AreaPing float64 // building area without parking
	Age      *int    // years since completion, when known
	// Floor and TotalFloors are 0 when unknown.
	Floor       int
	TotalFloors int
	Parking     bool
}

// ComparableSale is a candidate sale near the home, as stored.
type ComparableSale struct {
	ID           string
	Address      string
	Lat, Lng     float64
	AreaPing     float64
	Age          *int
	FloorLabel   string
	PricePerPing float64
	TransactedAt time.Time
	DistanceM    float64
}

// PriceIndexPoint is a month of a city's price index: the median price per ping of the
// sales in that month and the two before it.
type PriceIndexPoint struct {
	Month  time.Time
	Sales  int
	Median *float64
}

// Comparable is a sale used for an estimate, with its price adjusted to the home.
type Comparable struct {
	ID                   string  `json:"id"`
	Address              string  `json:"address"`
	Lat                  float64 `json:"lat"`
	Lng                  float64 `json:"lng"`
	TransactedAt         string  `json:"transactedAt"` // YYYY-MM-DD
	AreaPing             float64 `json:"areaPing"`
	Age                  *int    `json:"age"`
	FloorLabel           string  `json:"floorLabel"`
	PricePerPing         float64 `json:"pricePerPing"`
	AdjustedPricePerPing float64 `json:"adjustedPricePerPing"`
	DistanceM            float64 `json:"distanceM"`
	SameBuilding         bool    `json:"sameBuilding"`
	// Weight is this sale's share of all weights, summing to 1.
	Weight float64 `json:"weight"`
}

// Estimate values a home. Prices are TWD; ranges are where 80% of sales fell in a
// backtest of estimates with the same confidence.
type Estimate struct {
	CityCode string `json:"cityCode"`
	District string `json:"district"`
	// AsOf is the month prices are adjusted to: the last completely registered one.
	AsOf string `json:"asOf"`
	// Sufficient is false with fewer than the minimum comparable sales; the prices and
	// confidence are then empty.
	Sufficient       bool     `json:"sufficient"`
	MinimumSample    int      `json:"minimumSample"`
	PricePerPing     *float64 `json:"pricePerPing"`
	LowPricePerPing  *float64 `json:"lowPricePerPing"`
	HighPricePerPing *float64 `json:"highPricePerPing"`
	// BuildingPrice is PricePerPing times the area; TotalPrice adds ParkingPrice.
	BuildingPrice *float64 `json:"buildingPrice"`
	ParkingPrice  *float64 `json:"parkingPrice"`
	TotalPrice    *float64 `json:"totalPrice"`
	LowPrice      *float64 `json:"lowPrice"`
	HighPrice     *float64 `json:"highPrice"`
	// Confidence is high, medium or low; Spread is the weighted interquartile range of
	// the adjusted prices over the estimate.
	Confidence string  `json:"confidence"`
	Spread     float64 `json:"spread"`
	// BacktestMedianError is the median absolute error of backtested estimates with the
	// same confidence; RangeLow and RangeHigh are the range's ratios to the estimate.
	BacktestMedianError float64 `json:"backtestMedianError"`
	RangeLow            float64 `json:"rangeLow"`
	RangeHigh           float64 `json:"rangeHigh"`
	// Comparables lists the heaviest ComparableSales sales, at most 100.
	Comparables       []Comparable `json:"comparables"`
	ComparableSales   int          `json:"comparableSales"`
	SameBuildingSales int          `json:"sameBuildingSales"`
	MaxDistanceM      float64      `json:"maxDistanceM"`
	// Rent is nil without a price estimate.
	Rent *RentEstimate `json:"rent"`
}

// BuildingProfile summarises the recorded sales of one building, to fill in a form.
type BuildingProfile struct {
	Address string `json:"address"`
	Sales   int    `json:"sales"`
	// The most common category and completion year, and the highest total floors.
	Category      string `json:"category"`
	CompletedYear *int   `json:"completedYear"`
	TotalFloors   *int   `json:"totalFloors"`
	LastSale      string `json:"lastSale"` // YYYY-MM-DD, empty without sales
	// Community is the community at the door number, if any.
	Community *CommunityRef `json:"community"`
}

// LeaseCandidate is a whole-home lease near a home, as stored.
type LeaseCandidate struct {
	ID          string
	Address     string
	Lat, Lng    float64
	AreaPing    float64
	Age         *int
	MonthlyRent float64
	RentPerPing float64
	LeasedAt    time.Time
	DistanceM   float64
}

// ComparableLease is a lease used for a rent estimate, with its rent adjusted to the
// latest complete month.
type ComparableLease struct {
	ID                  string  `json:"id"`
	Address             string  `json:"address"`
	Lat                 float64 `json:"lat"`
	Lng                 float64 `json:"lng"`
	LeasedAt            string  `json:"leasedAt"` // YYYY-MM-DD
	AreaPing            float64 `json:"areaPing"`
	Age                 *int    `json:"age"`
	MonthlyRent         float64 `json:"monthlyRent"`
	RentPerPing         float64 `json:"rentPerPing"`
	AdjustedRentPerPing float64 `json:"adjustedRentPerPing"`
	DistanceM           float64 `json:"distanceM"`
	SameBuilding        bool    `json:"sameBuilding"`
	Weight              float64 `json:"weight"`
}

// RentEstimate is the monthly rent of a home from comparable whole-home leases nearby, and
// its gross yield against the price estimate.
type RentEstimate struct {
	AsOf string `json:"asOf"`
	// Supported is false for categories with too few leases to estimate (透天厝).
	Supported       bool     `json:"supported"`
	Sufficient      bool     `json:"sufficient"`
	MinimumSample   int      `json:"minimumSample"`
	RentPerPing     *float64 `json:"rentPerPing"`
	MonthlyRent     *float64 `json:"monthlyRent"`
	LowMonthlyRent  *float64 `json:"lowMonthlyRent"`
	HighMonthlyRent *float64 `json:"highMonthlyRent"`
	Confidence      string   `json:"confidence"`
	Spread          float64  `json:"spread"`
	// Backtest figures of rent estimates with the same confidence, as for prices.
	BacktestMedianError float64 `json:"backtestMedianError"`
	RangeLow            float64 `json:"rangeLow"`
	RangeHigh           float64 `json:"rangeHigh"`
	// GrossYield is twelve months of rent over the estimated price without parking;
	// DistrictGrossYield is the same kind of homes' in the district (see RentalDistrict).
	GrossYield         *float64          `json:"grossYield"`
	DistrictGrossYield *float64          `json:"districtGrossYield"`
	Leases             []ComparableLease `json:"leases"`
	LeaseCount         int               `json:"leaseCount"`
	SameBuildingLeases int               `json:"sameBuildingLeases"`
}
