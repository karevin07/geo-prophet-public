package domain

// DistrictComparisonQuery scopes the ranking to one city and the trend to selected districts.
type DistrictComparisonQuery struct {
	Filter    TransactionFilter
	CityCode  string
	Districts []string
}

type DistrictRank struct {
	District           string   `json:"district"`
	Count              int      `json:"count"`
	AvgPricePerPing    float64  `json:"avgPricePerPing"`
	MedianPricePerPing float64  `json:"medianPricePerPing"`
	MedianTotalPrice   float64  `json:"medianTotalPrice"`
	PriceYoY           *float64 `json:"priceYoY"`
}

type DistrictTrend struct {
	District        string  `json:"district"`
	Month           string  `json:"month"`
	Count           int     `json:"count"`
	AvgPricePerPing float64 `json:"avgPricePerPing"`
	Provisional     bool    `json:"provisional"`
}

type DistrictComparison struct {
	CityCode   string          `json:"cityCode"`
	YoYThrough string          `json:"yoyThrough"`
	Ranking    []DistrictRank  `json:"ranking"`
	Trends     []DistrictTrend `json:"trends"`
}

// DistrictYoYTotals are repository aggregates used to compare equal 12-month windows.
type DistrictYoYTotals struct {
	CurrentCount          int
	PreviousCount         int
	CurrentBuildingPrice  int64
	PreviousBuildingPrice int64
	CurrentArea           float64
	PreviousArea          float64
}

// DistrictBoundary is a district's area approximated from its door-number points, as a
// GeoJSON MultiPolygon ([polygon][ring][point][lng, lat]) without holes.
type DistrictBoundary struct {
	District    string          `json:"district"`
	Cells       int             `json:"cells"`
	Coordinates [][][][]float64 `json:"coordinates"`
}
