package domain

// Community is a group of residential buildings completed together: the door numbers on
// one road, lane or alley whose 住宅大樓, 華廈 or 公寓 sales share a completion date.
// 實價登錄 does not name communities, so Name is the presale project sold at one of its
// door numbers, or empty; Address always describes its door numbers, e.g.
// 敬業三路162巷 100–248號.
type Community struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	CityCode    string   `json:"cityCode"`
	District    string   `json:"district"`
	DoorNumbers int      `json:"doorNumbers"`
	Category    string   `json:"category"`
	CompletedOn string   `json:"completedOn"` // YYYY-MM-DD
	TotalFloors *int     `json:"totalFloors"`
	Sales       int      `json:"sales"`
	LastSale    string   `json:"lastSale"` // YYYY-MM-DD
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
}

// CommunityRef links to a community from a sale, a hexagon or an address.
type CommunityRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	// Sales counts the sales behind the link, such as those in a hexagon; zero otherwise.
	Sales int `json:"sales,omitempty"`
}

// CommunityRecord is a community as stored, before its address is described.
type CommunityRecord struct {
	Community
	Street string
	Doors  []string
}

// CommunityYear compares a community's median price per ping in a year with its district's,
// over the same building category. Provisional marks a year that runs past the last fully
// registered month.
type CommunityYear struct {
	Year                       int      `json:"year"`
	Sales                      int      `json:"sales"`
	MedianPricePerPing         float64  `json:"medianPricePerPing"`
	DistrictMedianPricePerPing *float64 `json:"districtMedianPricePerPing"`
	Provisional                bool     `json:"provisional"`
}

// CommunityRent summarises whole-home market leases in a community from From.
type CommunityRent struct {
	From              string   `json:"from"` // YYYY-MM
	Leases            int      `json:"leases"`
	MedianRentPerPing *float64 `json:"medianRentPerPing"`
	MedianMonthlyRent *float64 `json:"medianMonthlyRent"`
	// GrossYield is twelve months of median rent per ping over the median price per ping of
	// the last 12 complete months; nil without enough of either.
	GrossYield *float64 `json:"grossYield"`
}

// NearbyCommunity is a community of the same category completed within five years, near
// another one, with its median over the same 12 months.
type NearbyCommunity struct {
	Community
	DistanceM          float64  `json:"distanceM"`
	RecentSales        int      `json:"recentSales"`
	MedianPricePerPing *float64 `json:"medianPricePerPing"`
	// Difference is its median over the other community's, minus one.
	Difference *float64 `json:"difference"`
}

// CommunityStats are a community's market sales over a window.
type CommunityStats struct {
	Sales              int
	MedianPricePerPing *float64
}

// CommunityDetail describes a community. Recent figures cover market sales (house and
// land, not special) in the 12 months from From to Through, the last complete month.
type CommunityDetail struct {
	Community   Community `json:"community"`
	From        string    `json:"from"`
	Through     string    `json:"through"`
	RecentSales int       `json:"recentSales"`
	// MedianPricePerPing is nil with fewer than 3 recent sales; PriceYoY compares it with
	// the 12 months before when both have at least 5.
	MedianPricePerPing *float64 `json:"medianPricePerPing"`
	PriceYoY           *float64 `json:"priceYoY"`
	// DistrictPricePerPing is the district's median for the same category and months;
	// Premium is the community's over it, minus one.
	DistrictPricePerPing *float64 `json:"districtPricePerPing"`
	Premium              *float64 `json:"premium"`
	// The median area and the most common number of rooms of market sales in five years,
	// with MainRoomsShare its share of them.
	MedianAreaPing *float64        `json:"medianAreaPing"`
	MainRooms      *int            `json:"mainRooms"`
	MainRoomsShare *float64        `json:"mainRoomsShare"`
	Years          []CommunityYear `json:"years"`
	// FloorPremium is the change in price per ping per floor above the ground floor over
	// five years, net of the year; nil with fewer than 10 sales on 3 floors.
	FloorPremium *float64          `json:"floorPremium"`
	Rent         *CommunityRent    `json:"rent"`
	Nearby       []NearbyCommunity `json:"nearby"`
	// Sales are every recorded sale, latest first.
	Sales []Transaction `json:"sales"`
}

// CommunityRanking lists a district's communities with the most recent market sales.
type CommunityRanking struct {
	CityCode    string            `json:"cityCode"`
	District    string            `json:"district"`
	From        string            `json:"from"`
	Through     string            `json:"through"`
	Communities []RankedCommunity `json:"communities"`
}

type RankedCommunity struct {
	Community
	RecentSales        int      `json:"recentSales"`
	MedianPricePerPing *float64 `json:"medianPricePerPing"`
}
