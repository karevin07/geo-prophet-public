package domain

import "time"

// Transaction is a completed real-estate sale from the actual price registration open data.
type Transaction struct {
	ID               string     `json:"id"`
	CityCode         string     `json:"cityCode"`
	District         string     `json:"district"`
	Address          string     `json:"address"`
	TransactionType  string     `json:"transactionType"`
	BuildingType     string     `json:"buildingType"`
	MainUse          string     `json:"mainUse"`
	TotalPrice       int64      `json:"totalPrice"`
	ParkingPrice     int64      `json:"parkingPrice"`
	BuildingPrice    int64      `json:"buildingPrice"`
	BuildingAreaPing float64    `json:"buildingAreaPing"`
	PricePerPing     float64    `json:"pricePerPing"`
	Rooms            int        `json:"rooms"`
	Halls            int        `json:"halls"`
	Baths            int        `json:"baths"`
	FloorLabel       string     `json:"floorLabel"`
	TotalFloors      int        `json:"totalFloors,omitempty"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
	TransactedAt     time.Time  `json:"transactedAt"`
	Remarks          string     `json:"remarks,omitempty"`
	SpecialReasons   []string   `json:"specialReasons,omitempty"`
	Season           string     `json:"season"`
	// Location is filled by geocoding; GeocodePrecision is empty when the address was not located.
	Lat              float64 `json:"lat,omitempty"`
	Lng              float64 `json:"lng,omitempty"`
	GeocodePrecision string  `json:"geocodePrecision,omitempty"`
	// CommunityID identifies the community of a 住宅大樓, 華廈 or 公寓 sale with a door
	// number and completion date; empty otherwise.
	CommunityID string `json:"communityId,omitempty"`
	// IsNew marks sales first published in the latest 10-day release.
	IsNew bool `json:"isNew,omitempty"`
}

// IsSpecial reports whether the price is unlikely to reflect a normal market transaction.
func (transaction Transaction) IsSpecial() bool { return len(transaction.SpecialReasons) > 0 }
