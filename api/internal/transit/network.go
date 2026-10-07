// Package transit loads rail stations (MRT, light rail, 台鐵 and 高鐵) in 台北市, 新北市 and
// 桃園市 from TDX, the MOTC transport data platform, and merges transfer stations.
package transit

import (
	"math"
	"slices"
	"strings"
)

// System groups operators by the kind of service riders see on the map.
type System string

const (
	SystemMetro System = "metro"
	SystemLRT   System = "lrt"
	SystemTRA   System = "tra"
	SystemTHSR  System = "thsr"
)

// Line is one route, e.g. TRTC-BL (板南線) or TRA-WL (西部幹線).
type Line struct {
	ID       string // operator-LineID
	Operator string
	System   System
	Name     string
	NameEn   string
	Color    string
	// Geometry is the route as WKT (LINESTRING or MULTILINESTRING), as TDX publishes it.
	// Rail routes may extend beyond the covered cities; storage clips them.
	Geometry string
}

// Platform is one operator's station, e.g. TRTC-BL12 for 板南線 台北車站.
type Platform struct {
	ID       string // operator-StationID
	Operator string
	System   System
	Name     string
	NameEn   string
	Lat, Lng float64
	CityCode string // 實價登錄 city code: a, f or h
	District string
	Lines    []LineStop
	Exits    []Exit
}

// LineStop places a platform on a line; TRA platforms may serve several lines.
type LineStop struct {
	LineID   string
	Sequence int
}

// Exit is a station entrance. Platforms without published exits get one at the platform.
type Exit struct {
	ID       string
	Name     string
	Lat, Lng float64
}

// Station is what riders call one station: platforms with the same name close together,
// e.g. 板橋 serves 台鐵, 高鐵, 板南線 and 環狀線.
type Station struct {
	ID        string // the first platform's ID in platform priority order
	Name      string
	NameEn    string
	Lat, Lng  float64
	CityCode  string
	District  string
	Platforms []Platform
}

// Systems lists the station's systems in priority order.
func (station Station) Systems() []System {
	var systems []System
	for _, platform := range station.Platforms {
		if !slices.Contains(systems, platform.System) {
			systems = append(systems, platform.System)
		}
	}
	return systems
}

// Exits lists every entrance of the station, prefixed with its platform to stay unique.
func (station Station) Exits() []Exit {
	var exits []Exit
	for _, platform := range station.Platforms {
		for _, exit := range platform.Exits {
			exit.ID = platform.ID + ":" + exit.ID
			exits = append(exits, exit)
		}
	}
	return exits
}

// Network is every station and line in the covered cities.
type Network struct {
	Stations []Station
	Lines    []Line
}

const (
	// sameNameRadius merges platforms of one name, e.g. 台北車站 of 板南線 and 機場捷運.
	sameNameRadius = 800.0
	// adjacentRadius merges differently named platforms of different operators sharing a
	// building, e.g. 高鐵桃園 (機場捷運 A18) and 高鐵 桃園. Neighbouring light rail stops of
	// one operator can be this close too, so they are kept apart.
	adjacentRadius = 150.0
)

// mergeStations groups platforms, which must be sorted by priority, into stations.
func mergeStations(platforms []Platform) []Station {
	var stations []Station
	for _, platform := range platforms {
		index := slices.IndexFunc(stations, func(station Station) bool {
			for _, other := range station.Platforms {
				distance := distanceMeters(platform.Lat, platform.Lng, other.Lat, other.Lng)
				sameName := matchName(platform.Name) == matchName(other.Name)
				if (sameName && distance <= sameNameRadius) || (platform.Operator != other.Operator && distance <= adjacentRadius) {
					return true
				}
			}
			return false
		})
		if index < 0 {
			stations = append(stations, Station{
				ID: platform.ID, Name: displayName(platform.Name), NameEn: platform.NameEn,
				Lat: platform.Lat, Lng: platform.Lng, CityCode: platform.CityCode, District: platform.District,
			})
			index = len(stations) - 1
		}
		stations[index].Platforms = append(stations[index].Platforms, platform)
	}
	return stations
}

// displayName drops the 站 that 機場捷運 appends to every name, keeping 車站 as in 台北車站.
func displayName(name string) string {
	if strings.HasSuffix(name, "站") && !strings.HasSuffix(name, "車站") {
		return strings.TrimSuffix(name, "站")
	}
	return name
}

// matchName normalizes names across operators: 臺北, 台北 and 台北車站 all match.
func matchName(name string) string {
	name = strings.ReplaceAll(name, "臺", "台")
	if trimmed, ok := strings.CutSuffix(name, "車站"); ok {
		return trimmed
	}
	return strings.TrimSuffix(name, "站")
}

// distanceMeters is the haversine distance, accurate enough for merging nearby platforms.
func distanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6_371_000.0
	toRadians := math.Pi / 180
	dLat, dLng := (lat2-lat1)*toRadians, (lng2-lng1)*toRadians
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*toRadians)*math.Cos(lat2*toRadians)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadius * math.Asin(math.Sqrt(a))
}
