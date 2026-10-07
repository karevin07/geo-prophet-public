package domain

// FacilityGroup groups facility categories for map layers and summaries.
type FacilityGroup string

const (
	GroupDaily     FacilityGroup = "daily"     // 生活便利
	GroupEducation FacilityGroup = "education" // 教育
	GroupMedical   FacilityGroup = "medical"   // 醫療
	GroupLeisure   FacilityGroup = "leisure"   // 休閒
	GroupNuisance  FacilityGroup = "nuisance"  // 嫌惡設施
)

// FacilityGroups lists the groups in display order.
var FacilityGroups = []FacilityGroup{GroupDaily, GroupEducation, GroupMedical, GroupLeisure, GroupNuisance}

// FacilityCategories maps each category to its group, in display order within a group.
var FacilityCategories = []struct {
	Category string
	Group    FacilityGroup
}{
	{"convenience", GroupDaily}, {"supermarket", GroupDaily}, {"market", GroupDaily},
	{"elementary", GroupEducation}, {"junior_high", GroupEducation}, {"senior_high", GroupEducation}, {"university", GroupEducation},
	{"hospital", GroupMedical}, {"clinic", GroupMedical},
	{"park", GroupLeisure}, {"library", GroupLeisure},
	{"gas_station", GroupNuisance}, {"substation", GroupNuisance}, {"funeral", GroupNuisance}, {"cemetery", GroupNuisance}, {"waste", GroupNuisance},
}

// FacilityGroupOf returns a category's group, or false for an unknown category.
func FacilityGroupOf(category string) (FacilityGroup, bool) {
	for _, entry := range FacilityCategories {
		if entry.Category == category {
			return entry.Group, true
		}
	}
	return "", false
}

// Facility is a place that makes an area convenient or undesirable to live in: a shop,
// school, clinic, park or a nuisance such as a substation.
type Facility struct {
	// ID is unique across sources, e.g. osm:node/123, nhi:0101090517 or moe:elementary:011601.
	ID       string  `json:"id"`
	Source   string  `json:"source"` // osm, nhi or moe
	Category string  `json:"category"`
	Name     string  `json:"name"`
	Brand    string  `json:"brand,omitempty"`
	CityCode string  `json:"cityCode"`
	District string  `json:"district"`
	Address  string  `json:"address,omitempty"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	// Area is the outline of a park, cemetery or similar as a closed [lng, lat] ring;
	// distances are measured to it rather than to the centre. AreaM2 is its size.
	Area   [][2]float64 `json:"-"`
	AreaM2 float64      `json:"areaM2,omitempty"`
	// GeocodePrecision is set for facilities located from an address.
	GeocodePrecision string `json:"geocodePrecision,omitempty"`
}

// FacilityQuery selects facilities of some categories inside a bounding box.
type FacilityQuery struct {
	Bounds     Bounds
	Categories []string
}

// NearbyCategory summarises one category around a point: how many lie within the radius
// and the nearest one within the search distance, if any.
type NearbyCategory struct {
	Category    string        `json:"category"`
	Group       FacilityGroup `json:"group"`
	Count       int           `json:"count"`
	NearestName string        `json:"nearestName,omitempty"`
	NearestM    *float64      `json:"nearestM"`
}

type NearbyFacilities struct {
	RadiusM float64 `json:"radiusM"`
	// SearchM is how far the nearest facility of each category is looked for.
	SearchM    float64          `json:"searchM"`
	Categories []NearbyCategory `json:"categories"`
}

// FacilityCell is an H3 cell of the density layer. Score counts the positive categories
// (every group but nuisances) found in the cell or its neighbours, a proxy for how many
// everyday needs are within a short walk; Nuisances counts nuisances in the same area.
type FacilityCell struct {
	Hex       string                `json:"hex"`
	Score     int                   `json:"score"`
	Nuisances int                   `json:"nuisances"`
	Groups    map[FacilityGroup]int `json:"groups"`
}

// DistrictFacilities counts each category's facilities in a district. AreaKm2 is the
// district's area from door-number cells (see districtmap): roughly its built-up area,
// since mountains and rivers without door numbers are left out.
type DistrictFacilities struct {
	District string         `json:"district"`
	AreaKm2  float64        `json:"areaKm2"`
	Counts   map[string]int `json:"counts"`
}

type CityFacilities struct {
	CityCode  string               `json:"cityCode"`
	City      DistrictFacilities   `json:"city"`
	Districts []DistrictFacilities `json:"districts"`
}
