package geocode

import (
	"regexp"
	"slices"
	"strings"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
)

type Precision string

const (
	PrecisionAddress  Precision = "address"  // the exact door number
	PrecisionNearby   Precision = "nearby"   // closest door number on the same road, lane and alley
	PrecisionRoad     Precision = "road"     // centroid of the road section
	PrecisionDistrict Precision = "district" // centroid of the district
)

// maxNearbyGap bounds how far apart door numbers may be for a PrecisionNearby match.
const maxNearbyGap = 10

type Location struct {
	Lat       float64
	Lng       float64
	Precision Precision
}

// Characters that 實價登錄 and the door-number data spell with different variants.
var variantCharacters = map[rune]rune{'館': '舘', '舘': '館', '峰': '峯', '峯': '峰', '雙': '双', '双': '雙'}

type segmentKey struct {
	districtCode string
	road         string
	lane         string
	alley        string
}

type roadKey struct {
	districtCode string
	road         string
}

type centroid struct {
	latSum float64
	lngSum float64
	count  int
}

func (point *centroid) add(lat, lng float64) {
	point.latSum += lat
	point.lngSum += lng
	point.count++
}

func (point *centroid) location(precision Precision) Location {
	return Location{Lat: point.latSum / float64(point.count), Lng: point.lngSum / float64(point.count), Precision: precision}
}

// Matcher resolves addresses of one city against its door-number points.
type Matcher struct {
	cityName  string
	districts map[string]string
	points    map[addresspoint.Key]addresspoint.Point
	segments  map[segmentKey][]addresspoint.Point
	roads     map[roadKey]*centroid
	roadNames map[string][]string
	centroids map[string]*centroid
}

// NewMatcher indexes points; districts maps district names to their 鄉鎮市區代碼.
func NewMatcher(cityName string, districts map[string]string, points []addresspoint.Point) *Matcher {
	matcher := &Matcher{
		cityName:  cityName,
		districts: districts,
		points:    make(map[addresspoint.Key]addresspoint.Point, len(points)),
		segments:  map[segmentKey][]addresspoint.Point{},
		roads:     map[roadKey]*centroid{},
		roadNames: map[string][]string{},
		centroids: map[string]*centroid{},
	}
	for _, point := range points {
		matcher.points[point.Key()] = point
		segment := segmentKey{point.DistrictCode, point.Road, point.Lane, point.Alley}
		matcher.segments[segment] = append(matcher.segments[segment], point)

		road := roadKey{point.DistrictCode, point.Road}
		if matcher.roads[road] == nil {
			matcher.roads[road] = &centroid{}
			matcher.roadNames[point.DistrictCode] = append(matcher.roadNames[point.DistrictCode], point.Road)
		}
		matcher.roads[road].add(point.Lat, point.Lng)

		if matcher.centroids[point.DistrictCode] == nil {
			matcher.centroids[point.DistrictCode] = &centroid{}
		}
		matcher.centroids[point.DistrictCode].add(point.Lat, point.Lng)
	}
	return matcher
}

// Locate finds the most precise location available for a normalized address.
func (matcher *Matcher) Locate(district, address string) (Location, bool) {
	districtCode, ok := matcher.districts[district]
	if !ok || matcher.centroids[districtCode] == nil {
		return Location{}, false
	}
	parsed, ok := ParseAddress(address, matcher.cityName, district)
	if !ok {
		return matcher.centroids[districtCode].location(PrecisionDistrict), true
	}
	road, ok := matcher.resolveRoad(districtCode, parsed)
	if !ok {
		return matcher.centroids[districtCode].location(PrecisionDistrict), true
	}

	if parsed.Number > 0 {
		key := addresspoint.Key{
			DistrictCode: districtCode, Road: road, Lane: parsed.Lane, Alley: parsed.Alley,
			Number: parsed.Number, SubNumber: parsed.SubNumber,
		}
		if point, ok := matcher.points[key]; ok {
			return Location{Lat: point.Lat, Lng: point.Lng, Precision: PrecisionAddress}, true
		}
		if point, ok := matcher.nearest(segmentKey{districtCode, road, parsed.Lane, parsed.Alley}, parsed.Number); ok {
			return Location{Lat: point.Lat, Lng: point.Lng, Precision: PrecisionNearby}, true
		}
	}
	return matcher.roads[roadKey{districtCode, road}].location(PrecisionRoad), true
}

func (matcher *Matcher) nearest(segment segmentKey, number int) (addresspoint.Point, bool) {
	var best addresspoint.Point
	bestGap := maxNearbyGap + 1
	for _, point := range matcher.segments[segment] {
		gap := point.Number - number
		if gap < 0 {
			gap = -gap
		}
		if gap < bestGap || (gap == bestGap && point.SubNumber < best.SubNumber) {
			best, bestGap = point, gap
		}
	}
	return best, bestGap <= maxNearbyGap
}

// resolveRoad maps a road name to the spelling used by the door-number data.
//   - 實價登錄 stores rare characters as Private Use Area code points (e.g. U+E16D for 舘),
//     so those match any character.
//   - Addresses sometimes omit the section (安和路 for 安和路一段); the section whose
//     lanes include the address's lane is chosen.
//
// A candidate is only accepted when it is unique.
func (matcher *Matcher) resolveRoad(districtCode string, address Address) (string, bool) {
	if _, ok := matcher.roads[roadKey{districtCode, address.Road}]; ok {
		return address.Road, true
	}
	wanted := []rune(address.Road)
	var spellings, sections []string
	for _, name := range matcher.roadNames[districtCode] {
		if roadMatches(wanted, []rune(name)) {
			spellings = append(spellings, name)
		} else if strings.HasPrefix(name, address.Road) && sectionPattern.MatchString(name[len(address.Road):]) {
			sections = append(sections, name)
		}
	}
	if len(spellings) == 1 {
		return spellings[0], true
	}
	if len(sections) > 1 && address.Lane != "" {
		sections = slices.DeleteFunc(sections, func(name string) bool {
			_, ok := matcher.segments[segmentKey{districtCode, name, address.Lane, address.Alley}]
			return !ok
		})
	}
	if len(spellings) == 0 && len(sections) == 1 {
		return sections[0], true
	}
	return "", false
}

var sectionPattern = regexp.MustCompile(`^[一二三四五六七八九十]+段$`)

func roadMatches(wanted, candidate []rune) bool {
	if len(wanted) != len(candidate) {
		return false
	}
	for index, character := range wanted {
		if character != candidate[index] && !isPrivateUse(character) && variantCharacters[character] != candidate[index] {
			return false
		}
	}
	return true
}

func isPrivateUse(character rune) bool { return character >= 0xE000 && character <= 0xF8FF }
