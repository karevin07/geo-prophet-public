package usecase

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/uber/h3-go/v4"
)

const (
	maxFacilitiesShown = 5000
	// maxFacilityBoxDegrees bounds a map request to roughly a city.
	maxFacilityBoxDegrees = 1.2
	// The nearest facility of each category is looked for within 2 km.
	nearbySearchM = 2000.0
	// densityLimit caps the facilities read for one density request.
	densityLimit = 100_000
)

// densityRings is how many rings of neighbouring cells a density cell pools, about 500 m
// at either resolution: a resolution 8 cell alone spans about 460 m from its centre.
var densityRings = map[int]int{8: 0, 9: 2}

type FacilityRepository interface {
	Facilities(ctx context.Context, query domain.FacilityQuery, limit int) ([]domain.Facility, error)
	NearbyFacilities(ctx context.Context, lat, lng, radius, search float64) ([]domain.NearbyCategory, error)
	DistrictFacilities(ctx context.Context, city string) ([]domain.DistrictFacilities, error)
}

type FacilityService struct {
	repository FacilityRepository
}

func NewFacilityService(repository FacilityRepository) *FacilityService {
	return &FacilityService{repository: repository}
}

// FacilityList is a map request's facilities; Truncated means more were in the bounds.
type FacilityList struct {
	Facilities []domain.Facility `json:"facilities"`
	Truncated  bool              `json:"truncated"`
}

// Facilities returns the facilities of some categories (every one when empty) on a map.
func (service *FacilityService) Facilities(ctx context.Context, bounds domain.Bounds, categories []string) (FacilityList, error) {
	categories, err := facilityCategories(categories)
	if err != nil {
		return FacilityList{}, err
	}
	if err := validateFacilityBox(bounds); err != nil {
		return FacilityList{}, err
	}
	facilities, err := service.repository.Facilities(ctx, domain.FacilityQuery{Bounds: bounds, Categories: categories}, maxFacilitiesShown+1)
	if err != nil {
		return FacilityList{}, err
	}
	list := FacilityList{Facilities: facilities}
	if len(facilities) > maxFacilitiesShown {
		list.Facilities, list.Truncated = facilities[:maxFacilitiesShown], true
	}
	return list, nil
}

// Nearby summarises every category around a point: how many lie within radius metres and
// the nearest within 2 km.
func (service *FacilityService) Nearby(ctx context.Context, lat, lng, radius float64) (domain.NearbyFacilities, error) {
	if !inCoveredArea(lat, lng) {
		return domain.NearbyFacilities{}, fmt.Errorf("%w: location outside the covered cities", ErrInvalidInput)
	}
	if radius < 100 || radius > nearbySearchM {
		return domain.NearbyFacilities{}, fmt.Errorf("%w: radius must be between 100 and %.0f m", ErrInvalidInput, nearbySearchM)
	}
	found, err := service.repository.NearbyFacilities(ctx, lat, lng, radius, nearbySearchM)
	if err != nil {
		return domain.NearbyFacilities{}, err
	}
	byCategory := map[string]domain.NearbyCategory{}
	for _, category := range found {
		if category.NearestM != nil {
			category.NearestM = new(math.Round(*category.NearestM))
		}
		byCategory[category.Category] = category
	}
	nearby := domain.NearbyFacilities{RadiusM: radius, SearchM: nearbySearchM, Categories: make([]domain.NearbyCategory, 0, len(domain.FacilityCategories))}
	for _, entry := range domain.FacilityCategories {
		category := byCategory[entry.Category]
		category.Category, category.Group = entry.Category, entry.Group
		nearby.Categories = append(nearby.Categories, category)
	}
	return nearby, nil
}

// Density scores the H3 cells of a map: each cell counts the positive categories found in
// it or within about 500 m (densityRings), and the nuisances there. Only the given
// categories count (every one when empty).
func (service *FacilityService) Density(ctx context.Context, bounds domain.Bounds, resolution int, categories []string) ([]domain.FacilityCell, error) {
	rings, ok := densityRings[resolution]
	if !ok {
		return nil, fmt.Errorf("%w: resolution must be 8 or 9", ErrInvalidInput)
	}
	if err := validateFacilityBox(bounds); err != nil {
		return nil, err
	}
	// Read facilities slightly beyond the bounds so edge cells see their neighbours.
	const margin = 0.01
	padded := domain.Bounds{West: bounds.West - margin, South: bounds.South - margin, East: bounds.East + margin, North: bounds.North + margin}
	categories, err := facilityCategories(categories)
	if err != nil {
		return nil, err
	}
	facilities, err := service.repository.Facilities(ctx, domain.FacilityQuery{Bounds: padded, Categories: categories}, densityLimit)
	if err != nil {
		return nil, err
	}

	// Categories and nuisance counts found in each cell.
	type contents struct {
		categories map[string]bool
		nuisances  int
	}
	cells := map[h3.Cell]*contents{}
	for _, facility := range facilities {
		cell, err := h3.LatLngToCell(h3.NewLatLng(facility.Lat, facility.Lng), resolution)
		if err != nil {
			continue
		}
		if cells[cell] == nil {
			cells[cell] = &contents{categories: map[string]bool{}}
		}
		if group, _ := domain.FacilityGroupOf(facility.Category); group == domain.GroupNuisance {
			cells[cell].nuisances++
		} else {
			cells[cell].categories[facility.Category] = true
		}
	}

	// Every cell near a facility gets a score from its disk.
	candidates := map[h3.Cell]bool{}
	for cell := range cells {
		disk, err := h3.GridDisk(cell, rings)
		if err != nil {
			continue
		}
		for _, neighbour := range disk {
			candidates[neighbour] = true
		}
	}
	scored := make([]domain.FacilityCell, 0, len(candidates))
	for cell := range candidates {
		center, err := h3.CellToLatLng(cell)
		if err != nil || center.Lat < bounds.South || center.Lat > bounds.North || center.Lng < bounds.West || center.Lng > bounds.East {
			continue
		}
		disk, err := h3.GridDisk(cell, rings)
		if err != nil {
			continue
		}
		present := map[string]bool{}
		nuisances := 0
		for _, neighbour := range disk {
			if found := cells[neighbour]; found != nil {
				for category := range found.categories {
					present[category] = true
				}
				nuisances += found.nuisances
			}
		}
		if len(present) == 0 && nuisances == 0 {
			continue
		}
		groups := map[domain.FacilityGroup]int{}
		for category := range present {
			group, _ := domain.FacilityGroupOf(category)
			groups[group]++
		}
		scored = append(scored, domain.FacilityCell{Hex: cell.String(), Score: len(present), Nuisances: nuisances, Groups: groups})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Hex < scored[j].Hex })
	return scored, nil
}

// facilityCategories validates categories, defaulting to every one.
func facilityCategories(categories []string) ([]string, error) {
	if len(categories) == 0 {
		for _, entry := range domain.FacilityCategories {
			categories = append(categories, entry.Category)
		}
		return categories, nil
	}
	for _, category := range categories {
		if _, ok := domain.FacilityGroupOf(category); !ok {
			return nil, fmt.Errorf("%w: unknown facility category %q", ErrInvalidInput, category)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(categories))), nil
}

func validateFacilityBox(bounds domain.Bounds) error {
	if bounds.West >= bounds.East || bounds.South >= bounds.North {
		return fmt.Errorf("%w: invalid bbox", ErrInvalidInput)
	}
	if bounds.East-bounds.West > maxFacilityBoxDegrees || bounds.North-bounds.South > maxFacilityBoxDegrees {
		return fmt.Errorf("%w: bbox is larger than %.1f°", ErrInvalidInput, maxFacilityBoxDegrees)
	}
	return nil
}

// Districts counts a city's facilities by district and category, with each district's
// area and the city's totals, for densities per km².
func (service *FacilityService) Districts(ctx context.Context, city string) (domain.CityFacilities, error) {
	if _, ok := addresspoint.Sources[city]; !ok {
		return domain.CityFacilities{}, fmt.Errorf("%w: unknown city %q", ErrInvalidInput, city)
	}
	districts, err := service.repository.DistrictFacilities(ctx, city)
	if err != nil {
		return domain.CityFacilities{}, err
	}
	sort.Slice(districts, func(i, j int) bool { return districts[i].District < districts[j].District })
	result := domain.CityFacilities{CityCode: city, City: domain.DistrictFacilities{Counts: map[string]int{}}, Districts: districts}
	for _, district := range districts {
		result.City.AreaKm2 += district.AreaKm2
		for category, count := range district.Counts {
			result.City.Counts[category] += count
		}
	}
	result.City.AreaKm2 = math.Round(result.City.AreaKm2*100) / 100
	return result, nil
}
