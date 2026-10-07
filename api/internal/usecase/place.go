package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/geocode"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
	"github.com/geo-prophet/geo-prophet/api/internal/twtext"
)

const maxPlaces = 8

type PlaceRepository interface {
	// FindAddresses returns door numbers matching the query exactly.
	FindAddresses(context.Context, domain.PlaceQuery) ([]domain.PlaceMatch, error)
	// FindRoads returns road centroids whose name contains the query's road.
	FindRoads(context.Context, domain.PlaceQuery) ([]domain.PlaceMatch, error)
	// DistrictCenter returns the centroid of a district's door numbers.
	DistrictCenter(ctx context.Context, cityCode, districtCode string) (lat, lng float64, found bool, err error)
	// FindStations returns rail stations whose name starts with name (臺 written as 台),
	// up to limit.
	FindStations(ctx context.Context, name string, limit int) ([]domain.Place, error)
}

// maxStations caps station results so they do not crowd out roads and addresses.
const maxStations = 3

type district struct {
	cityCode string
	code     string
	name     string
}

type PlaceService struct {
	repository PlaceRepository
	districts  []district
}

func NewPlaceService(repository PlaceRepository) *PlaceService {
	service := &PlaceService{repository: repository}
	for cityCode, source := range addresspoint.Sources {
		for name, code := range source.Districts {
			service.districts = append(service.districts, district{cityCode: cityCode, code: code, name: name})
		}
	}
	sort.Slice(service.districts, func(i, j int) bool { return service.districts[i].code < service.districts[j].code })
	return service
}

// Search resolves free text such as 大安區, 敦化南路, 台北市大安區敦化南路一段100號 or a
// station name such as 板橋站. Stations come first, then districts, roads and addresses.
func (service *PlaceService) Search(ctx context.Context, text string) ([]domain.Place, error) {
	text = strings.ReplaceAll(twtext.Normalize(text), " ", "")
	if utf8.RuneCountInString(text) < 2 {
		return nil, fmt.Errorf("%w: search needs at least 2 characters", ErrInvalidInput)
	}
	stations, err := service.repository.FindStations(ctx, stationName(text), maxStations)
	if err != nil {
		return nil, err
	}
	places, err := service.searchAreas(ctx, text)
	if err != nil {
		return nil, err
	}
	places = append(stations, places...)
	if len(places) > maxPlaces {
		places = places[:maxPlaces]
	}
	return places, nil
}

// stationName drops a trailing 站 or 車站 and writes 臺 as 台, as station names are matched:
// 臺北車站, 台北車站 and 台北 all find 台北車站.
func stationName(text string) string {
	text = strings.ReplaceAll(text, "臺", "台")
	if trimmed, ok := strings.CutSuffix(text, "車站"); ok && trimmed != "" {
		return trimmed
	}
	if trimmed, ok := strings.CutSuffix(text, "站"); ok && trimmed != "" {
		return trimmed
	}
	return text
}

func (service *PlaceService) searchAreas(ctx context.Context, text string) ([]domain.Place, error) {

	cityCode := ""
	for code, name := range lvr.Cities {
		if _, supported := addresspoint.Sources[code]; supported && strings.HasPrefix(text, name) {
			cityCode, text = code, strings.TrimPrefix(text, name)
			break
		}
	}
	var matched []district
	for _, candidate := range service.districts {
		if cityCode != "" && candidate.cityCode != cityCode {
			continue
		}
		if strings.HasPrefix(text, candidate.name) {
			matched = []district{candidate}
			text = strings.TrimPrefix(text, candidate.name)
			break
		}
		// 板橋 or 大安 finds the district even without 區.
		if text == strings.TrimSuffix(candidate.name, "區") {
			matched = append(matched, candidate)
		}
	}
	if len(matched) > 0 && (text == "" || text == strings.TrimSuffix(matched[0].name, "區")) {
		return service.districtPlaces(ctx, matched)
	}

	query := domain.PlaceQuery{CityCode: cityCode, Limit: maxPlaces}
	for _, candidate := range matched {
		query.DistrictCodes = append(query.DistrictCodes, candidate.code)
	}
	if address, ok := geocode.ParseAddress(text, "", ""); ok {
		query.Road, query.Lane, query.Alley = address.Road, address.Lane, address.Alley
		query.Number, query.SubNumber = address.Number, address.SubNumber
	} else {
		query.Road = text
	}

	var places []domain.Place
	if query.Number > 0 {
		matches, err := service.repository.FindAddresses(ctx, query)
		if err != nil {
			return nil, err
		}
		places = append(places, service.label(matches, domain.PlaceAddress)...)
	}
	if len(places) < maxPlaces {
		query.Limit = maxPlaces - len(places)
		matches, err := service.repository.FindRoads(ctx, query)
		if err != nil {
			return nil, err
		}
		places = append(places, service.label(matches, domain.PlaceRoad)...)
	}
	return places, nil
}

func (service *PlaceService) districtPlaces(ctx context.Context, districts []district) ([]domain.Place, error) {
	places := make([]domain.Place, 0, len(districts))
	for _, candidate := range districts {
		lat, lng, found, err := service.repository.DistrictCenter(ctx, candidate.cityCode, candidate.code)
		if err != nil {
			return nil, err
		}
		if found {
			places = append(places, domain.Place{Kind: domain.PlaceDistrict, Label: lvr.Cities[candidate.cityCode] + candidate.name, Lat: lat, Lng: lng})
		}
	}
	return places, nil
}

func (service *PlaceService) label(matches []domain.PlaceMatch, kind domain.PlaceKind) []domain.Place {
	places := make([]domain.Place, 0, len(matches))
	for _, match := range matches {
		label := lvr.Cities[match.CityCode] + service.districtName(match.DistrictCode) + match.Road
		if kind == domain.PlaceAddress {
			label += match.Lane + match.Alley + fmt.Sprintf("%d", match.Number)
			if match.SubNumber > 0 {
				label += fmt.Sprintf("之%d", match.SubNumber)
			}
			label += "號"
		}
		places = append(places, domain.Place{Kind: kind, Label: label, Lat: match.Lat, Lng: match.Lng})
	}
	return places
}

func (service *PlaceService) districtName(code string) string {
	for _, candidate := range service.districts {
		if candidate.code == code {
			return candidate.name
		}
	}
	return ""
}
