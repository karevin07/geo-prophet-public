package importer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/districtmap"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/facility"
	"github.com/geo-prophet/geo-prophet/api/internal/geocode"
	"github.com/uber/h3-go/v4"
)

// Facility sources, each recorded as its own import run and replaced as a whole.
const (
	FacilitiesOSM = "osm" // shops, markets, parks, libraries and nuisances
	FacilitiesNHI = "nhi" // hospitals and clinics
	FacilitiesMOE = "moe" // schools
)

// FacilitySources lists every facility source.
var FacilitySources = []string{FacilitiesOSM, FacilitiesNHI, FacilitiesMOE}

var facilityDatasets = map[string]string{
	FacilitiesOSM: "osm_facilities", FacilitiesNHI: "nhi_facilities", FacilitiesMOE: "moe_schools",
}

// listedDataset is a data.gov.tw dataset of a government list and the category it holds.
type listedDataset struct{ id, category string }

var (
	// Health insurance contracted 醫學中心, 區域醫院, 地區醫院 and 診所.
	nhiDatasets = []listedDataset{{"39280", "hospital"}, {"39281", "hospital"}, {"39282", "hospital"}, {"39283", "clinic"}}
	// Education ministry 國民小學, 國民中學, 一般高級中等學校 and 大專校院 lists.
	moeDatasets = []listedDataset{{"6087", "elementary"}, {"6088", "junior_high"}, {"6089", "senior_high"}, {"6091", "university"}}
)

// coveredBox bounds 台北, 新北 and 桃園; facilities inside it are then assigned to a
// district or dropped.
var coveredBox = facility.Box{South: 24.55, West: 120.95, North: 25.35, East: 122.05}

// ImportFacilities replaces the facilities of each source. osmFile, when set, is read
// instead of downloading the OpenStreetMap extract.
func (service *Service) ImportFacilities(ctx context.Context, sources []string, osmFile string) error {
	for _, source := range sources {
		if !slices.Contains(FacilitySources, source) {
			return fmt.Errorf("unknown facility source %q", source)
		}
	}
	var errs []error
	for _, source := range sources {
		run := domain.ImportRun{Source: source, Dataset: facilityDatasets[source], Season: service.now().Format("2006-01")}
		_, err := service.record(ctx, run, func(runID int64) (domain.ImportStats, error) {
			switch source {
			case FacilitiesOSM:
				return service.importOSMFacilities(ctx, runID, osmFile)
			case FacilitiesNHI:
				return service.importListedFacilities(ctx, runID, source, nhiDatasets, facility.ReadNHI)
			}
			return service.importListedFacilities(ctx, runID, source, moeDatasets, facility.ReadSchools)
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("facilities %s: %w", source, err))
		}
	}
	return errors.Join(errs...)
}

// syncFacilities refreshes each facility source older than maxAge.
func (service *Service) syncFacilities(ctx context.Context, maxAge time.Duration) error {
	var stale []string
	for _, source := range FacilitySources {
		last, done, err := service.store.LastSucceeded(ctx, domain.ImportRun{Source: source, Dataset: facilityDatasets[source]})
		if err != nil {
			return err
		}
		if !done || service.now().Sub(last) > maxAge {
			stale = append(stale, source)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	return service.ImportFacilities(ctx, stale, "")
}

// importOSMFacilities downloads the Taiwan OpenStreetMap extract (over 300 MB, read twice
// from a temporary file) and assigns its facilities to districts by the door numbers
// around them; facilities outside the covered districts are dropped.
func (service *Service) importOSMFacilities(ctx context.Context, runID int64, localFile string) (domain.ImportStats, error) {
	if localFile != "" {
		return service.readOSMFacilities(ctx, runID, localFile)
	}
	body, err := service.openSource(ctx, "", func() (string, error) { return service.fetcher.OSMExtractURL(), nil })
	if err != nil {
		return domain.ImportStats{}, err
	}
	file, err := os.CreateTemp("", "taiwan-*.osm.pbf")
	if err != nil {
		body.Close()
		return domain.ImportStats{}, err
	}
	defer os.Remove(file.Name())
	_, err = io.Copy(file, body)
	body.Close()
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return domain.ImportStats{}, fmt.Errorf("download extract: %w", err)
	}
	return service.readOSMFacilities(ctx, runID, file.Name())
}

func (service *Service) readOSMFacilities(ctx context.Context, runID int64, path string) (domain.ImportStats, error) {
	facilities, err := facility.ReadOSM(ctx, func() (io.ReadCloser, error) { return os.Open(path) }, coveredBox)
	if err != nil {
		return domain.ImportStats{}, err
	}
	owners := map[string]map[h3.Cell]string{}
	for _, cityCode := range sortedCities() {
		points, err := service.store.LoadAddressPoints(ctx, cityCode)
		if err != nil {
			return domain.ImportStats{}, err
		}
		if owners[cityCode], err = districtmap.Owners(points, addresspoint.Sources[cityCode].Districts); err != nil {
			return domain.ImportStats{}, err
		}
	}

	stats := domain.ImportStats{RowsRead: len(facilities), Detail: map[string]int{}}
	located := make([]domain.Facility, 0, len(facilities))
	for _, item := range facilities {
		for _, cityCode := range sortedCities() {
			if district, ok := districtmap.Locate(owners[cityCode], item.Lat, item.Lng); ok {
				item.CityCode, item.District = cityCode, district
				located = append(located, item)
				stats.Detail["category_"+item.Category]++
				break
			}
		}
	}
	stats.RowsSkipped = len(facilities) - len(located)
	stats.Detail["skipped_outside"] = stats.RowsSkipped
	if err = service.store.ReplaceFacilities(ctx, runID, FacilitiesOSM, located); err != nil {
		return stats, err
	}
	stats.RowsUpserted = len(located)
	return stats, nil
}

// importListedFacilities reads government lists from data.gov.tw and locates each entry
// by its address, keeping those found at a door number or nearby.
func (service *Service) importListedFacilities(ctx context.Context, runID int64, source string, datasets []listedDataset, read func(io.Reader, string) ([]facility.Listed, error)) (domain.ImportStats, error) {
	var listed []facility.Listed
	for _, dataset := range datasets {
		body, err := service.openSource(ctx, "", func() (string, error) { return service.fetcher.LatestCSVURL(ctx, dataset.id) })
		if err != nil {
			return domain.ImportStats{}, fmt.Errorf("dataset %s: %w", dataset.id, err)
		}
		entries, err := read(body, dataset.category)
		body.Close()
		if err != nil {
			return domain.ImportStats{}, fmt.Errorf("dataset %s: %w", dataset.id, err)
		}
		listed = append(listed, entries...)
	}

	matchers := service.newMatcherCache()
	stats := domain.ImportStats{RowsRead: len(listed), Detail: map[string]int{}}
	facilities := make([]domain.Facility, 0, len(listed))
	for _, entry := range listed {
		matcher, err := matchers(ctx, entry.CityCode)
		if err != nil {
			return domain.ImportStats{}, err
		}
		location, ok := matcher.Locate(entry.District, entry.Address)
		if ok {
			stats.Detail["geocode_"+string(location.Precision)]++
		}
		if !ok || (location.Precision != geocode.PrecisionAddress && location.Precision != geocode.PrecisionNearby) {
			stats.RowsGeocodeFailed++
			continue
		}
		stats.Detail["category_"+entry.Category]++
		facilities = append(facilities, domain.Facility{
			ID: entry.ID, Source: source, Category: entry.Category, Name: entry.Name, CityCode: entry.CityCode,
			District: entry.District, Address: entry.Address, Lat: location.Lat, Lng: location.Lng,
			GeocodePrecision: string(location.Precision),
		})
	}
	if err := service.store.ReplaceFacilities(ctx, runID, source, facilities); err != nil {
		return stats, err
	}
	stats.RowsUpserted = len(facilities)
	return stats, nil
}

func sortedCities() []string {
	cities := make([]string, 0, len(addresspoint.Sources))
	for cityCode := range addresspoint.Sources {
		cities = append(cities, cityCode)
	}
	slices.Sort(cities)
	return cities
}
