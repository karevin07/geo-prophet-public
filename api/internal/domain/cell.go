package domain

import "time"

// CellRow is the subset of a transaction needed to describe one H3 cell.
type CellRow struct {
	CityCode      string
	District      string
	Address       string
	Lat           float64
	Lng           float64
	TransactedAt  time.Time
	TotalPrice    int64
	BuildingPrice int64
	AreaPing      float64
	BuildingAge   *int
}

// AgeBucket counts sales by building age at sale; Key is 0-5, 5-15, 15-30, 30+ or unknown.
type AgeBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// CellDetail describes the sales inside one hexagon for the map's hover card.
type CellDetail struct {
	Hex             string  `json:"hex"`
	Count           int     `json:"count"`
	AvgPricePerPing float64 `json:"avgPricePerPing"`
	AvgTotalPrice   float64 `json:"avgTotalPrice"`
	AvgArea         float64 `json:"avgArea"`
	// District and Road are the most common among the cell's sales; Road may be empty.
	CityCode string `json:"cityCode"`
	District string `json:"district"`
	Road     string `json:"road"`
	// PriceYoY compares area-weighted price per ping over 12 fully registered months with
	// the year before; nil when either period has fewer than 5 sales.
	PriceYoY   *float64    `json:"priceYoY"`
	YoYThrough string      `json:"yoyThrough"`
	AgeMix     []AgeBucket `json:"ageMix"`
}
