package domain

import "time"

// InvestmentQuery keeps the cohort comparable: one city, one building category,
// and half-open area and building-age bands.
type InvestmentQuery struct {
	CityCode string
	Category string
	MinArea  float64
	MaxArea  float64
	MinAge   int
	MaxAge   int
}

// InvestmentAggregate is a database-level 12-month comparison. The empty
// district is the city-wide reference cohort, not an investment recommendation.
type InvestmentAggregate struct {
	District                   string
	CurrentCount               int
	PreviousCount              int
	MedianPricePerPing         *float64
	PreviousMedianPricePerPing *float64
}

type InvestmentArea struct {
	District                   string   `json:"district"`
	CurrentCount               int      `json:"currentCount"`
	PreviousCount              int      `json:"previousCount"`
	MedianPricePerPing         *float64 `json:"medianPricePerPing"`
	VolumeChange               *float64 `json:"volumeChange"`
	PriceChange                *float64 `json:"priceChange"`
	RelativeToCityMedian       *float64 `json:"relativeToCityMedian"`
	SufficientCurrentSample    bool     `json:"sufficientCurrentSample"`
	SufficientComparisonSample bool     `json:"sufficientComparisonSample"`
}

type InvestmentReport struct {
	CityCode               string           `json:"cityCode"`
	Through                string           `json:"through"`
	CurrentFrom            string           `json:"currentFrom"`
	PreviousFrom           string           `json:"previousFrom"`
	MinimumSample          int              `json:"minimumSample"`
	CityCurrentCount       int              `json:"cityCurrentCount"`
	CityPreviousCount      int              `json:"cityPreviousCount"`
	CityMedianPricePerPing *float64         `json:"cityMedianPricePerPing"`
	CityVolumeChange       *float64         `json:"cityVolumeChange"`
	CityPriceChange        *float64         `json:"cityPriceChange"`
	Areas                  []InvestmentArea `json:"areas"`
}

// InvestmentWindow contains half-open sale-date bounds for two equal periods.
type InvestmentWindow struct {
	PreviousFrom time.Time
	CurrentFrom  time.Time
	EndExclusive time.Time
}

// InvestmentRentAggregate uses the same cohort and current 12-month window as sales.
type InvestmentRentAggregate struct {
	District          string
	Count             int
	MedianRentPerPing *float64
}

type InvestmentWorkbenchArea struct {
	InvestmentArea
	RentalCount       int      `json:"rentalCount"`
	MedianRentPerPing *float64 `json:"medianRentPerPing"`
	GrossYield        *float64 `json:"grossYield"`
}

type InvestmentWorkbenchReport struct {
	CityCode       string                    `json:"cityCode"`
	Through        string                    `json:"through"`
	CurrentFrom    string                    `json:"currentFrom"`
	PreviousFrom   string                    `json:"previousFrom"`
	MinimumSales   int                       `json:"minimumSales"`
	MinimumRentals int                       `json:"minimumRentals"`
	Areas          []InvestmentWorkbenchArea `json:"areas"`
}
