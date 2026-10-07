package domain

import "time"

// Rental is a registered lease from the actual price registration open data (租賃).
// Only leases handled by a rental management business or an agent must be registered,
// so the data under-represents landlords who rent out directly.
type Rental struct {
	ID              string `json:"id"`
	CityCode        string `json:"cityCode"`
	District        string `json:"district"`
	Address         string `json:"address"`
	TransactionType string `json:"transactionType"` // 交易標的, e.g. 租賃房屋+車位
	// RentalType is 出租型態 such as 整棟(戶)出租 or 分租套房; leases registered before
	// 2023 Q4 (season 112S4) have none.
	RentalType string `json:"rentalType,omitempty"`
	// WholeUnit marks leases of a whole home, the only ones whose rent per ping is
	// comparable. Without a rental type it is inferred from the area and remarks.
	WholeUnit         bool   `json:"wholeUnit"`
	WholeUnitInferred bool   `json:"wholeUnitInferred"`
	SocialHousing     bool   `json:"socialHousing"`           // leased through a social housing scheme, with capped rents
	RentalService     string `json:"rentalService,omitempty"` // 租賃住宅服務, e.g. 社會住宅代管
	BuildingType      string `json:"buildingType"`
	MainUse           string `json:"mainUse"`
	// MonthlyRent includes ParkingRent; BuildingRent excludes it. Many leases include a
	// parking space without pricing it separately.
	MonthlyRent      int64      `json:"monthlyRent"`
	ParkingRent      int64      `json:"parkingRent"`
	BuildingRent     int64      `json:"buildingRent"`
	BuildingAreaPing float64    `json:"buildingAreaPing"`
	RentPerPing      float64    `json:"rentPerPing"`
	Rooms            int        `json:"rooms"`
	Halls            int        `json:"halls"`
	Baths            int        `json:"baths"`
	FloorLabel       string     `json:"floorLabel"`
	TotalFloors      int        `json:"totalFloors,omitempty"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
	LeasedAt         time.Time  `json:"leasedAt"` // 租賃年月日, when the lease was signed
	LeaseStart       *time.Time `json:"leaseStart,omitempty"`
	LeaseEnd         *time.Time `json:"leaseEnd,omitempty"`
	Furnished        bool       `json:"furnished"`
	HasManager       *bool      `json:"hasManager,omitempty"`
	HasElevator      *bool      `json:"hasElevator,omitempty"`
	Equipment        string     `json:"equipment,omitempty"`
	Renewal          bool       `json:"renewal"` // 續租案件; renewals tend to lag the market
	Remarks          string     `json:"remarks,omitempty"`
	SpecialReasons   []string   `json:"specialReasons,omitempty"`
	Season           string     `json:"season"`
	Lat              float64    `json:"lat,omitempty"`
	Lng              float64    `json:"lng,omitempty"`
	GeocodePrecision string     `json:"geocodePrecision,omitempty"`
}

// IsSpecial reports whether the rent is unlikely to reflect a normal market lease.
func (rental Rental) IsSpecial() bool { return len(rental.SpecialReasons) > 0 }

// RentalQuery selects comparable homes for rent and gross yield by district. An empty
// Category means all four residential categories; MaxArea 0 means no upper bound.
type RentalQuery struct {
	CityCode             string
	Category             string
	MinArea              float64
	MaxArea              float64
	IncludeSocialHousing bool
}

// RentalAggregate holds one district's medians over a window; the empty district is the
// whole city. Rents are of new whole-home leases, prices of sales of the same homes.
type RentalAggregate struct {
	District           string
	Rentals            int
	MedianRentPerPing  *float64
	MedianMonthlyRent  *float64
	Sales              int
	MedianPricePerPing *float64
}

// RentalDistrict compares rents with sale prices. GrossYield is twelve months of the
// median rent per ping over the median sale price per ping, before costs, vacancy and
// tax; it is set only when both samples reach the minimum.
type RentalDistrict struct {
	District           string   `json:"district"`
	Rentals            int      `json:"rentals"`
	MedianRentPerPing  *float64 `json:"medianRentPerPing"`
	MedianMonthlyRent  *float64 `json:"medianMonthlyRent"`
	Sales              int      `json:"sales"`
	MedianPricePerPing *float64 `json:"medianPricePerPing"`
	GrossYield         *float64 `json:"grossYield"`
}

type RentalReport struct {
	CityCode             string           `json:"cityCode"`
	From                 string           `json:"from"`    // first month, YYYY-MM
	Through              string           `json:"through"` // last complete month, YYYY-MM
	MinimumSample        int              `json:"minimumSample"`
	IncludeSocialHousing bool             `json:"includeSocialHousing"`
	City                 RentalDistrict   `json:"city"`
	Districts            []RentalDistrict `json:"districts"`
}

// RentPoint is one month of a rent trend: the median rent per ping of the leases signed
// in that month and the two before it, since a month alone holds too few leases near a
// station. Provisional months are not completely registered yet.
type RentPoint struct {
	Month             string   `json:"month"` // YYYY-MM
	Rentals           int      `json:"rentals"`
	MedianRentPerPing *float64 `json:"medianRentPerPing"`
	Provisional       bool     `json:"provisional"`
}

// RentTrend is a city's rent trend and, when asked for, one district's.
type RentTrend struct {
	CityCode      string      `json:"cityCode"`
	District      string      `json:"district"`
	Through       string      `json:"through"` // last complete month
	MinimumSample int         `json:"minimumSample"`
	City          []RentPoint `json:"city"`
	Districts     []RentPoint `json:"districtPoints"`
}

// StationRentRing compares rents and sale prices within a radius of a station.
type StationRentRing struct {
	Radius             int      `json:"radius"`
	Rentals            int      `json:"rentals"`
	MedianRentPerPing  *float64 `json:"medianRentPerPing"`
	MedianMonthlyRent  *float64 `json:"medianMonthlyRent"`
	Sales              int      `json:"sales"`
	MedianPricePerPing *float64 `json:"medianPricePerPing"`
	GrossYield         *float64 `json:"grossYield"`
	// RelativeToDistrict compares the median rent per ping with the station's district.
	RelativeToDistrict *float64    `json:"relativeToDistrict"`
	Trend              []RentPoint `json:"trend"`
}

type StationRent struct {
	StationID                 string            `json:"stationId"`
	CityCode                  string            `json:"cityCode"`
	District                  string            `json:"district"`
	From                      string            `json:"from"`
	Through                   string            `json:"through"`
	MinimumSample             int               `json:"minimumSample"`
	DistrictMedianRentPerPing *float64          `json:"districtMedianRentPerPing"`
	Rings                     []StationRentRing `json:"rings"`
}
