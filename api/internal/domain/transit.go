package domain

import "encoding/json"

// StationRings are the distances (metres to the nearest exit) that station figures are
// computed within. Each ring includes the smaller ones.
var StationRings = []int{300, 500, 800}

// TransitLine is a rail route; Geometry is a GeoJSON MultiLineString.
type TransitLine struct {
	ID       string          `json:"id"`
	Operator string          `json:"operator"`
	System   string          `json:"system"`
	Name     string          `json:"name"`
	NameEn   string          `json:"nameEn"`
	Color    string          `json:"color"`
	OpenedOn *string         `json:"openedOn"` // YYYY-MM-DD; null if before 2015
	Geometry json.RawMessage `json:"-"`
}

// TransitStation is a station with the lines of all its platforms.
type TransitStation struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	NameEn   string        `json:"nameEn"`
	CityCode string        `json:"cityCode"`
	District string        `json:"district"`
	Lat      float64       `json:"lat"`
	Lng      float64       `json:"lng"`
	Systems  []string      `json:"systems"`
	OpenedOn *string       `json:"openedOn"`
	Lines    []StationLine `json:"lines"`
}

type StationLine struct {
	LineID       string  `json:"lineId"`
	Name         string  `json:"name"`
	Color        string  `json:"color"`
	PlatformID   string  `json:"platformId"`
	PlatformName string  `json:"platformName"`
	Sequence     int     `json:"sequence"`
	OpenedOn     *string `json:"openedOn"`
}

// StationRing summarizes sales within Radius metres of a station in the requested window;
// Trends span all years so they cross the station's opening.
type StationRing struct {
	Radius             int          `json:"radius"`
	Count              int          `json:"count"`
	AvgPricePerPing    float64      `json:"avgPricePerPing"`
	MedianPricePerPing float64      `json:"medianPricePerPing"`
	PriceYoY           *float64     `json:"priceYoY"`
	Trends             []TrendPoint `json:"trends"`
}

type StationTrend struct {
	Station    TransitStation `json:"station"`
	YoYThrough string         `json:"yoyThrough"`
	Rings      []StationRing  `json:"rings"`
}

// StationYoY is one station's change over the last 12 complete months, for colouring the
// map; Count is the sales in that window.
type StationYoY struct {
	StationID string   `json:"stationId"`
	Count     int      `json:"count"`
	PriceYoY  *float64 `json:"priceYoY"`
}

type StationYoYs struct {
	Radius     int          `json:"radius"`
	YoYThrough string       `json:"yoyThrough"`
	Stations   []StationYoY `json:"stations"`
}

// StationRank is one station's figures within a radius for a date window, with its
// monthly trend over that window and YoY over the last 12 complete months.
type StationRank struct {
	StationID          string       `json:"stationId"`
	Count              int          `json:"count"`
	AvgPricePerPing    float64      `json:"avgPricePerPing"`
	MedianPricePerPing float64      `json:"medianPricePerPing"`
	PriceYoY           *float64     `json:"priceYoY"`
	Trend              []TrendPoint `json:"trend"`
}

type StationRanking struct {
	Radius     int           `json:"radius"`
	YoYThrough string        `json:"yoyThrough"`
	Stations   []StationRank `json:"stations"`
}

// OpeningEvent is a line starting service at a station since 2015, e.g. 環狀線 at 板橋.
type OpeningEvent struct {
	StationID   string `json:"stationId"`
	StationName string `json:"stationName"`
	CityCode    string `json:"cityCode"`
	District    string `json:"district"`
	LineID      string `json:"lineId"`
	LineName    string `json:"lineName"`
	LineColor   string `json:"lineColor"`
	OpenedOn    string `json:"openedOn"` // YYYY-MM-DD
}

// OpeningWindows compares the 12 months before an opening with the 12 months from it.
type OpeningWindows struct {
	BeforeCount int      `json:"beforeCount"`
	AfterCount  int      `json:"afterCount"`
	BeforePrice *float64 `json:"beforePrice"` // area-weighted price per ping
	AfterPrice  *float64 `json:"afterPrice"`
	Change      *float64 `json:"change"`
}

// OpeningPoint is one month around the opening; Offset is months from the opening month.
type OpeningPoint struct {
	Month         string   `json:"month"`
	Offset        int      `json:"offset"`
	StationPrice  *float64 `json:"stationPrice"`
	StationCount  int      `json:"stationCount"`
	BaselinePrice *float64 `json:"baselinePrice"`
	BaselineCount int      `json:"baselineCount"`
}

// OpeningEffect sets a station's price change around an opening against its district's
// sales farther than 800 m from the station. Relative is Station.Change minus
// Baseline.Change; it is nil when a window has too few sales or the opening is too recent.
type OpeningEffect struct {
	OpeningEvent
	Station  OpeningWindows `json:"station"`
	Baseline OpeningWindows `json:"baseline"`
	Relative *float64       `json:"relative"`
	// Status is "ok", "too_recent" (under a year of sales after opening) or "insufficient".
	Status string         `json:"status"`
	Series []OpeningPoint `json:"series"`
}

type OpeningEffects struct {
	Radius int             `json:"radius"`
	Events []OpeningEffect `json:"events"`
}

// MonthlySums are the additive parts of an area-weighted monthly price.
type MonthlySums struct {
	Count         int
	BuildingPrice int64
	Area          float64
}

// DistanceBucket summarizes a city's located sales by distance to the nearest station
// exit: [FromM, ToM) metres, or FromM and beyond when ToM is nil.
type DistanceBucket struct {
	CityCode           string  `json:"cityCode"`
	FromM              int     `json:"fromM"`
	ToM                *int    `json:"toM"`
	Count              int     `json:"count"`
	AvgPricePerPing    float64 `json:"avgPricePerPing"`
	MedianPricePerPing float64 `json:"medianPricePerPing"`
}

// DistanceBucketM is the width of each distance bucket up to the farthest stored pair.
const DistanceBucketM = 100
