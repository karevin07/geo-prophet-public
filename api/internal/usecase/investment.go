package usecase

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

const minimumInvestmentSample = 30

type InvestmentRepository interface {
	ImportedSalesByCity(context.Context, string) (seasons []string, releases []string, err error)
	InvestmentAggregates(context.Context, domain.InvestmentQuery, domain.InvestmentWindow) ([]domain.InvestmentAggregate, error)
}

type InvestmentService struct {
	repository InvestmentRepository
}

func NewInvestmentService(repository InvestmentRepository) *InvestmentService {
	return &InvestmentService{repository: repository}
}

// CompareAreas describes observed sales, not sale time, fair value, or future returns.
// Both periods end before the last month whose registrations are complete for this city.
func (service *InvestmentService) CompareAreas(ctx context.Context, query domain.InvestmentQuery) (domain.InvestmentReport, error) {
	if err := validateInvestmentQuery(query); err != nil {
		return domain.InvestmentReport{}, err
	}

	report := domain.InvestmentReport{CityCode: query.CityCode, MinimumSample: minimumInvestmentSample, Areas: []domain.InvestmentArea{}}
	seasons, releases, err := service.repository.ImportedSalesByCity(ctx, query.CityCode)
	if err != nil {
		return domain.InvestmentReport{}, err
	}
	registered, known := registeredThrough(seasons, releases)
	if !known {
		return report, nil
	}
	complete := completeThrough(registered)
	window := domain.InvestmentWindow{
		CurrentFrom:  complete.AddDate(0, -11, 0),
		EndExclusive: complete.AddDate(0, 1, 0),
	}
	window.PreviousFrom = window.CurrentFrom.AddDate(-1, 0, 0)
	report.Through = complete.Format("2006-01")
	report.CurrentFrom = window.CurrentFrom.Format("2006-01")
	report.PreviousFrom = window.PreviousFrom.Format("2006-01")

	rows, err := service.repository.InvestmentAggregates(ctx, query, window)
	if err != nil {
		return domain.InvestmentReport{}, err
	}
	var city domain.InvestmentAggregate
	for _, row := range rows {
		if row.District == "" {
			city = row
			break
		}
	}
	report.CityCurrentCount = city.CurrentCount
	report.CityPreviousCount = city.PreviousCount
	if city.CurrentCount >= minimumInvestmentSample {
		report.CityMedianPricePerPing = city.MedianPricePerPing
	}
	if city.CurrentCount >= minimumInvestmentSample && city.PreviousCount >= minimumInvestmentSample {
		report.CityVolumeChange = change(float64(city.CurrentCount), float64(city.PreviousCount))
		if city.MedianPricePerPing != nil && city.PreviousMedianPricePerPing != nil {
			report.CityPriceChange = change(*city.MedianPricePerPing, *city.PreviousMedianPricePerPing)
		}
	}
	for _, row := range rows {
		if row.District == "" {
			continue
		}
		area := domain.InvestmentArea{
			District: row.District, CurrentCount: row.CurrentCount, PreviousCount: row.PreviousCount,
			SufficientCurrentSample:    row.CurrentCount >= minimumInvestmentSample,
			SufficientComparisonSample: row.CurrentCount >= minimumInvestmentSample && row.PreviousCount >= minimumInvestmentSample,
		}
		if area.SufficientCurrentSample {
			area.MedianPricePerPing = row.MedianPricePerPing
			if row.MedianPricePerPing != nil && report.CityMedianPricePerPing != nil {
				area.RelativeToCityMedian = change(*row.MedianPricePerPing, *report.CityMedianPricePerPing)
			}
		}
		if area.SufficientComparisonSample {
			area.VolumeChange = change(float64(row.CurrentCount), float64(row.PreviousCount))
			if row.MedianPricePerPing != nil && row.PreviousMedianPricePerPing != nil {
				area.PriceChange = change(*row.MedianPricePerPing, *row.PreviousMedianPricePerPing)
			}
		}
		report.Areas = append(report.Areas, area)
	}
	sort.Slice(report.Areas, func(i, j int) bool {
		if report.Areas[i].CurrentCount != report.Areas[j].CurrentCount {
			return report.Areas[i].CurrentCount > report.Areas[j].CurrentCount
		}
		return report.Areas[i].District < report.Areas[j].District
	})
	return report, nil
}

func validateInvestmentQuery(query domain.InvestmentQuery) error {
	if _, ok := addresspoint.Sources[query.CityCode]; !ok {
		return fmt.Errorf("%w: unknown city %q", ErrInvalidInput, query.CityCode)
	}
	if !slices.Contains(domain.BuildingCategories[:len(domain.BuildingCategories)-1], query.Category) {
		return fmt.Errorf("%w: choose one residential building type", ErrInvalidInput)
	}
	if query.MinArea < 0 || query.MinArea >= query.MaxArea || query.MaxArea > 300 ||
		query.MinAge < 0 || query.MinAge >= query.MaxAge || query.MaxAge > 100 {
		return fmt.Errorf("%w: invalid area or building-age range", ErrInvalidInput)
	}
	return nil
}
