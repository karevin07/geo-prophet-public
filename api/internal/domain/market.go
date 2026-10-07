package domain

import (
	"strings"
	"time"
)

type Bounds struct {
	West  float64
	South float64
	East  float64
	North float64
}

// Building categories are the part of 建物型態 before the parenthesis, e.g. 住宅大樓 for
// 住宅大樓(11層含以上有電梯). Every other type falls into BuildingCategoryOther.
const BuildingCategoryOther = "其他"

var BuildingCategories = []string{"住宅大樓", "華廈", "公寓", "透天厝", BuildingCategoryOther}

func BuildingCategory(buildingType string) string {
	category, _, _ := strings.Cut(buildingType, "(")
	for _, known := range BuildingCategories[:len(BuildingCategories)-1] {
		if category == known {
			return category
		}
	}
	return BuildingCategoryOther
}

// NewBuildingYears is the building age below which a sale counts as a new building.
const NewBuildingYears = 2

type TransactionFilter struct {
	Bounds     *Bounds
	MinPrice   int64
	MaxPrice   int64
	MinArea    float64
	MaxArea    float64
	Categories []string
	From       *time.Time
	To         *time.Time
	// IncludeSpecial keeps related-party, split-registration and outlier transactions.
	IncludeSpecial bool
	// PreciseLocation drops transactions located only at a district centroid, which would
	// otherwise pile up as false hotspots on the map.
	PreciseLocation bool
	// ExcludeNewBuildings drops sales of buildings completed under NewBuildingYears before
	// the sale. A new development delivering hundreds of units at once swamps a small area,
	// so price comparisons over time measure the mix rather than the market. Sales with an
	// unknown completion date are kept.
	ExcludeNewBuildings bool
	Limit               int
}

// PricePoint is the subset of a transaction needed for spatial aggregation.
type PricePoint struct {
	Lat              float64
	Lng              float64
	TotalPrice       int64
	BuildingPrice    int64
	BuildingAreaPing float64
}

type H3Aggregate struct {
	Hex             string  `json:"hex"`
	Lat             float64 `json:"lat"`
	Lng             float64 `json:"lng"`
	Count           int     `json:"count"`
	Price           float64 `json:"price"`
	AvgArea         float64 `json:"avgArea"`
	AvgPricePerPing float64 `json:"avgPricePerPing"`
}

type TrendPoint struct {
	Month           string  `json:"month"`
	AvgPricePerPing float64 `json:"avgPricePerPing"`
	Count           int     `json:"count"`
	// Provisional marks months whose sales may still be registered and published.
	Provisional bool `json:"provisional"`
}

// MarketSummary describes the transactions matching a filter.
type MarketSummary struct {
	Count            int     `json:"count"`
	AvgPricePerPing  float64 `json:"avgPricePerPing"`
	MedianTotalPrice float64 `json:"medianTotalPrice"`
}

// YearOverYear holds relative changes (0.041 is +4.1%) against the same window a year
// earlier. A nil change means either period had too few sales to compare.
type YearOverYear struct {
	// Basis is the compared window, e.g. "12m" or "3m" when a single month is selected.
	Basis string `json:"basis"`
	// Through is the last month (YYYY-MM) of the compared window; it stops at the last
	// month whose sales are fully registered, so recent months never skew the change.
	Through          string   `json:"through"`
	Count            *float64 `json:"count"`
	AvgPricePerPing  *float64 `json:"avgPricePerPing"`
	MedianTotalPrice *float64 `json:"medianTotalPrice"`
}

type SummaryWithYoY struct {
	MarketSummary
	YoY *YearOverYear `json:"yoy"`
}

type PlaceKind string

const (
	PlaceDistrict PlaceKind = "district"
	PlaceRoad     PlaceKind = "road"
	PlaceAddress  PlaceKind = "address"
	PlaceStation  PlaceKind = "station"
)

// Place is a search result the map can fly to.
type Place struct {
	Kind  PlaceKind `json:"kind"`
	Label string    `json:"label"`
	Lat   float64   `json:"lat"`
	Lng   float64   `json:"lng"`
	// StationID identifies a rail station (Kind PlaceStation), e.g. TRTC-BL07.
	StationID string `json:"stationId,omitempty"`
}

// PlaceQuery is a parsed search: an optional district and a road, with the door number
// when the user typed one.
type PlaceQuery struct {
	CityCode      string
	DistrictCodes []string
	Road          string
	Lane          string
	Alley         string
	Number        int
	SubNumber     int
	Limit         int
}

// PlaceMatch is a door number or road found by a place search.
type PlaceMatch struct {
	CityCode     string
	DistrictCode string
	Road         string
	Lane         string
	Alley        string
	Number       int
	SubNumber    int
	Lat          float64
	Lng          float64
}
