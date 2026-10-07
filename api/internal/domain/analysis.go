package domain

// MonthlyAnalysis is one calendar month's market summary.
type MonthlyAnalysis struct {
	Month              string  `json:"month"`
	Count              int     `json:"count"`
	AvgPricePerPing    float64 `json:"avgPricePerPing"`
	MedianPricePerPing float64 `json:"medianPricePerPing"`
	MedianTotalPrice   float64 `json:"medianTotalPrice"`
	AvgArea            float64 `json:"avgArea"`
	Provisional        bool    `json:"provisional"`
}

// CategoryAnalysis compares one normalized building category with the others.
type CategoryAnalysis struct {
	Category           string  `json:"category"`
	Count              int     `json:"count"`
	AvgPricePerPing    float64 `json:"avgPricePerPing"`
	MedianPricePerPing float64 `json:"medianPricePerPing"`
	MedianTotalPrice   float64 `json:"medianTotalPrice"`
}

// BreakdownAnalysis summarizes a building-age or area band.
type BreakdownAnalysis struct {
	Key                string  `json:"key"`
	Count              int     `json:"count"`
	AvgPricePerPing    float64 `json:"avgPricePerPing"`
	MedianPricePerPing float64 `json:"medianPricePerPing"`
	MedianTotalPrice   float64 `json:"medianTotalPrice"`
}

// PriceBucket counts sales in [MinPricePerPing, MaxPricePerPing).
type PriceBucket struct {
	MinPricePerPing int64 `json:"minPricePerPing"`
	MaxPricePerPing int64 `json:"maxPricePerPing"`
	Count           int   `json:"count"`
}

// MarketAnalysis contains the datasets rendered by the market analysis page.
type MarketAnalysis struct {
	Monthly      []MonthlyAnalysis   `json:"monthly"`
	Categories   []CategoryAnalysis  `json:"categories"`
	AgeGroups    []BreakdownAnalysis `json:"ageGroups"`
	AreaGroups   []BreakdownAnalysis `json:"areaGroups"`
	Distribution []PriceBucket       `json:"distribution"`
}
