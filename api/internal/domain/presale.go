package domain

import "time"

// Presale is a 預售屋 sale from the actual price registration open data. Before season
// 110S3 (July 2021) presales were registered in bulk after the sales agency's contract
// ended, often years late; since then within 30 days of signing. TransactedAt is the
// contract date. Addresses are often only a road or lane, so presales are analysed by
// project.
type Presale struct {
	Transaction
	// ProjectName is 建案名稱, from about season 108S1; ProjectKey identifies the project
	// as city|district|name and is empty without a name.
	ProjectName string `json:"projectName"`
	ProjectKey  string `json:"projectKey,omitempty"`
	UnitLabel   string `json:"unitLabel,omitempty"` // 棟及號, e.g. A5號
	// Cancellation is 解約情形 such as 1150329全部解約; CancelledOn is its date.
	Cancellation string     `json:"cancellation,omitempty"`
	CancelledOn  *time.Time `json:"cancelledOn,omitempty"`
}

// PresaleProjectQuery lists the projects of a city with a contract since From, or with
// Near set, the projects within Near.RadiusM of a point, nearest first.
type PresaleProjectQuery struct {
	CityCode string
	District string
	From     time.Time
	Near     *NearPoint
}

type NearPoint struct {
	Lat, Lng, RadiusM float64
}

// PresaleProject summarises a project's presales. Medians and Sales count valid
// house-and-land sales: not cancelled, not special.
type PresaleProject struct {
	Key              string   `json:"key"`
	Name             string   `json:"name"`
	CityCode         string   `json:"cityCode"`
	District         string   `json:"district"`
	Address          string   `json:"address"`
	Lat              *float64 `json:"lat"`
	Lng              *float64 `json:"lng"`
	GeocodePrecision string   `json:"geocodePrecision"`
	Category         string   `json:"category"`
	TotalFloors      *int     `json:"totalFloors"`
	Sales            int      `json:"sales"`
	Cancelled        int      `json:"cancelled"`
	// RecentSales are valid sales in the last 12 months before the query's end.
	RecentSales        int      `json:"recentSales"`
	FirstContract      string   `json:"firstContract"` // YYYY-MM-DD
	LastContract       string   `json:"lastContract"`
	MedianPricePerPing *float64 `json:"medianPricePerPing"`
	MedianTotalPrice   *float64 `json:"medianTotalPrice"`
	MedianAreaPing     *float64 `json:"medianAreaPing"`
	// DistanceM is set for projects listed near a point.
	DistanceM *float64 `json:"distanceM,omitempty"`
}

type PresaleProjects struct {
	CityCode string           `json:"cityCode"`
	From     string           `json:"from"`
	Projects []PresaleProject `json:"projects"`
}

// PresaleMonth counts a project's valid sales in a month.
type PresaleMonth struct {
	Month              string   `json:"month"`
	Sales              int      `json:"sales"`
	MedianPricePerPing *float64 `json:"medianPricePerPing"`
}

// PresaleFloor is the median price per ping of a project's valid sales on one floor.
type PresaleFloor struct {
	Floor              int     `json:"floor"`
	Sales              int     `json:"sales"`
	MedianPricePerPing float64 `json:"medianPricePerPing"`
}

// ResaleComparison compares a project with resales within RadiusM over the last 12
// complete months: all ages, and homes at most 5 years old.
type ResaleComparison struct {
	RadiusM int    `json:"radiusM"`
	From    string `json:"from"`
	Through string `json:"through"`
	// ProjectPricePerPing is the project's median over its last 12 months of contracts,
	// ProjectFrom to ProjectThrough.
	ProjectPricePerPing *float64 `json:"projectPricePerPing"`
	ProjectSales        int      `json:"projectSales"`
	ProjectFrom         string   `json:"projectFrom"`
	ProjectThrough      string   `json:"projectThrough"`
	Resales             int      `json:"resales"`
	ResalePricePerPing  *float64 `json:"resalePricePerPing"`
	NewResales          int      `json:"newResales"`
	NewPricePerPing     *float64 `json:"newPricePerPing"`
	// Premiums are the project's price over each resale median, minus one.
	Premium    *float64 `json:"premium"`
	NewPremium *float64 `json:"newPremium"`
}

type PresaleProjectDetail struct {
	Project PresaleProject `json:"project"`
	// The 10th and 90th percentile price per ping and the most common number of rooms of
	// valid sales; nil without any.
	LowPricePerPing  *float64       `json:"lowPricePerPing"`
	HighPricePerPing *float64       `json:"highPricePerPing"`
	MainRooms        *int           `json:"mainRooms"`
	Months           []PresaleMonth `json:"months"`
	Floors           []PresaleFloor `json:"floors"`
	Units            []Presale      `json:"units"`
	// Resale is nil when the project is located no better than its district.
	Resale *ResaleComparison `json:"resale"`
}
