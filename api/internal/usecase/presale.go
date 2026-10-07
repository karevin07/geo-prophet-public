package usecase

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

const (
	// defaultPresaleMonths lists projects with a contract in the last two years.
	defaultPresaleMonths = 24
	maxPresaleMonths     = 240
	// Resales within 1 km are compared; "new" ones are at most 5 years old.
	resaleRadiusM = 1000
	newResaleAge  = 5
	// minimumResaleSample is the fewest sales behind a compared median.
	minimumResaleSample = 10
	maxNearbyRadiusM    = 3000
)

type PresaleRepository interface {
	ImportedSalesByCity(context.Context, string) (seasons []string, releases []string, err error)
	PresaleProjects(ctx context.Context, query domain.PresaleProjectQuery, recentFrom time.Time) ([]domain.PresaleProject, error)
	NearbyPresaleProjects(ctx context.Context, near domain.NearPoint, from, recentFrom time.Time) ([]domain.PresaleProject, error)
	BoundedPresaleProjects(ctx context.Context, bounds domain.Bounds, from, recentFrom time.Time) ([]domain.PresaleProject, error)
	PresaleProject(ctx context.Context, key string, recentFrom time.Time) (domain.PresaleProject, bool, error)
	PresaleUnits(ctx context.Context, key string) ([]domain.Presale, error)
	ResalesNearby(ctx context.Context, lat, lng, radius float64, maxNewAge int, from, to time.Time) (int, *float64, int, *float64, error)
}

type PresaleService struct {
	repository PresaleRepository
	now        func() time.Time
}

func NewPresaleService(repository PresaleRepository) *PresaleService {
	return &PresaleService{repository: repository, now: time.Now}
}

// Projects lists a city's presale projects with a contract in the last months months,
// latest first; with near set, the projects near a point, nearest first; with bounds set,
// the projects on a map. RecentSales count the last 12 months.
func (service *PresaleService) Projects(ctx context.Context, city, district string, months int, near *domain.NearPoint, bounds *domain.Bounds) (domain.PresaleProjects, error) {
	if bounds != nil {
		if err := validateFacilityBox(*bounds); err != nil {
			return domain.PresaleProjects{}, err
		}
	}
	if _, ok := addresspoint.Sources[city]; !ok && near == nil && bounds == nil {
		return domain.PresaleProjects{}, fmt.Errorf("%w: unknown city %q", ErrInvalidInput, city)
	}
	if near != nil && (!inCoveredArea(near.Lat, near.Lng) || near.RadiusM <= 0 || near.RadiusM > maxNearbyRadiusM) {
		return domain.PresaleProjects{}, fmt.Errorf("%w: a point in the covered cities and a radius up to %d m", ErrInvalidInput, maxNearbyRadiusM)
	}
	if months == 0 {
		months = defaultPresaleMonths
	}
	if months < 1 || months > maxPresaleMonths {
		return domain.PresaleProjects{}, fmt.Errorf("%w: months must be between 1 and %d", ErrInvalidInput, maxPresaleMonths)
	}
	month := service.thisMonth()
	from := month.AddDate(0, -months+1, 0)
	var projects []domain.PresaleProject
	var err error
	switch {
	case near != nil:
		city = ""
		projects, err = service.repository.NearbyPresaleProjects(ctx, *near, from, month.AddDate(-1, 1, 0))
	case bounds != nil:
		city = ""
		projects, err = service.repository.BoundedPresaleProjects(ctx, *bounds, from, month.AddDate(-1, 1, 0))
	default:
		projects, err = service.repository.PresaleProjects(ctx, domain.PresaleProjectQuery{CityCode: city, District: district, From: from}, month.AddDate(-1, 1, 0))
	}
	if err != nil {
		return domain.PresaleProjects{}, err
	}
	return domain.PresaleProjects{CityCode: city, From: from.Format("2006-01"), Projects: projects}, nil
}

// Project describes one project: its monthly sales, prices by floor, every unit and a
// comparison with resales nearby.
func (service *PresaleService) Project(ctx context.Context, key string) (domain.PresaleProjectDetail, error) {
	month := service.thisMonth()
	project, found, err := service.repository.PresaleProject(ctx, key, month.AddDate(-1, 1, 0))
	if err != nil {
		return domain.PresaleProjectDetail{}, err
	}
	if !found {
		return domain.PresaleProjectDetail{}, fmt.Errorf("%w: presale project %q", ErrNotFound, key)
	}
	units, err := service.repository.PresaleUnits(ctx, key)
	if err != nil {
		return domain.PresaleProjectDetail{}, err
	}
	detail := domain.PresaleProjectDetail{Project: project, Months: presaleMonths(units), Floors: presaleFloors(units), Units: units}
	var prices []float64
	rooms := map[int]int{}
	for _, unit := range units {
		if validPresale(unit) {
			prices = append(prices, unit.PricePerPing)
			rooms[unit.Rooms]++
		}
	}
	if len(prices) > 0 {
		slices.Sort(prices)
		detail.LowPricePerPing = new(math.Round(quantileSorted(prices, 0.1)))
		detail.HighPricePerPing = new(math.Round(quantileSorted(prices, 0.9)))
		main := -1
		for count, sales := range rooms {
			if main < 0 || sales > rooms[main] || (sales == rooms[main] && count < main) {
				main = count
			}
		}
		detail.MainRooms = &main
	}
	if detail.Resale, err = service.resale(ctx, project, units); err != nil {
		return domain.PresaleProjectDetail{}, err
	}
	return detail, nil
}

// resale compares the project's last 12 months of contracts with resales nearby over the
// last 12 complete months; nil when the project is located no better than its district.
func (service *PresaleService) resale(ctx context.Context, project domain.PresaleProject, units []domain.Presale) (*domain.ResaleComparison, error) {
	if project.Lat == nil || project.Lng == nil || project.GeocodePrecision == "district" || project.GeocodePrecision == "" {
		return nil, nil
	}
	seasons, releases, err := service.repository.ImportedSalesByCity(ctx, project.CityCode)
	if err != nil {
		return nil, err
	}
	registered, known := registeredThrough(seasons, releases)
	if !known {
		return nil, nil
	}
	through := completeThrough(registered)
	from := through.AddDate(0, -11, 0)
	comparison := &domain.ResaleComparison{RadiusM: resaleRadiusM, From: from.Format("2006-01"), Through: through.Format("2006-01")}
	comparison.Resales, comparison.ResalePricePerPing, comparison.NewResales, comparison.NewPricePerPing, err =
		service.repository.ResalesNearby(ctx, *project.Lat, *project.Lng, resaleRadiusM, newResaleAge, from, through.AddDate(0, 1, 0))
	if err != nil {
		return nil, err
	}
	if comparison.Resales < minimumResaleSample {
		comparison.ResalePricePerPing = nil
	}
	if comparison.NewResales < minimumResaleSample {
		comparison.NewPricePerPing = nil
	}

	var last time.Time
	for _, unit := range units {
		if validPresale(unit) && unit.TransactedAt.After(last) {
			last = unit.TransactedAt
		}
	}
	if last.IsZero() {
		return comparison, nil
	}
	lastMonth := time.Date(last.Year(), last.Month(), 1, 0, 0, 0, 0, time.UTC)
	projectFrom := lastMonth.AddDate(0, -11, 0)
	comparison.ProjectFrom, comparison.ProjectThrough = projectFrom.Format("2006-01"), lastMonth.Format("2006-01")
	var prices []float64
	for _, unit := range units {
		if validPresale(unit) && !unit.TransactedAt.Before(projectFrom) {
			prices = append(prices, unit.PricePerPing)
		}
	}
	comparison.ProjectSales = len(prices)
	if len(prices) > 0 {
		comparison.ProjectPricePerPing = new(math.Round(median(prices)))
		if comparison.ResalePricePerPing != nil {
			comparison.Premium = change(*comparison.ProjectPricePerPing, *comparison.ResalePricePerPing)
		}
		if comparison.NewPricePerPing != nil {
			comparison.NewPremium = change(*comparison.ProjectPricePerPing, *comparison.NewPricePerPing)
		}
	}
	return comparison, nil
}

func (service *PresaleService) thisMonth() time.Time {
	now := service.now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// validPresale reports whether a presale counts in project figures: a house-and-land
// sale that is neither cancelled nor special.
func validPresale(unit domain.Presale) bool {
	return strings.HasPrefix(unit.TransactionType, "房地") && unit.Cancellation == "" && !unit.IsSpecial()
}

// presaleMonths counts valid sales by contract month, oldest first.
func presaleMonths(units []domain.Presale) []domain.PresaleMonth {
	prices := map[string][]float64{}
	for _, unit := range units {
		if validPresale(unit) {
			month := unit.TransactedAt.Format("2006-01")
			prices[month] = append(prices[month], unit.PricePerPing)
		}
	}
	months := make([]domain.PresaleMonth, 0, len(prices))
	for month, values := range prices {
		months = append(months, domain.PresaleMonth{Month: month, Sales: len(values), MedianPricePerPing: new(math.Round(median(values)))})
	}
	sort.Slice(months, func(i, j int) bool { return months[i].Month < months[j].Month })
	return months
}

// presaleFloors gives the median price per ping of valid sales on each floor, lowest first.
func presaleFloors(units []domain.Presale) []domain.PresaleFloor {
	prices := map[int][]float64{}
	for _, unit := range units {
		if floor, ok := lvr.FloorNumber(unit.FloorLabel); ok && validPresale(unit) {
			prices[floor] = append(prices[floor], unit.PricePerPing)
		}
	}
	floors := make([]domain.PresaleFloor, 0, len(prices))
	for floor, values := range prices {
		floors = append(floors, domain.PresaleFloor{Floor: floor, Sales: len(values), MedianPricePerPing: math.Round(median(values))})
	}
	sort.Slice(floors, func(i, j int) bool { return floors[i].Floor < floors[j].Floor })
	return floors
}

// quantileSorted interpolates a quantile of sorted values.
func quantileSorted(sorted []float64, quantile float64) float64 {
	position := quantile * float64(len(sorted)-1)
	lower := int(position)
	if lower+1 >= len(sorted) {
		return sorted[lower]
	}
	return sorted[lower] + (position-float64(lower))*(sorted[lower+1]-sorted[lower])
}

func median(values []float64) float64 {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}
