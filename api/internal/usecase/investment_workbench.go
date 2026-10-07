package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
)

const minimumWorkbenchRentals = 20

type InvestmentWorkbenchRepository interface {
	InvestmentRepository
	ImportedRentalsByCity(context.Context, string) ([]string, []string, error)
	InvestmentRentAggregates(context.Context, domain.InvestmentQuery, time.Time, time.Time) ([]domain.InvestmentRentAggregate, error)
}

type InvestmentWorkbenchService struct{ repository InvestmentWorkbenchRepository }

func NewInvestmentWorkbenchService(repository InvestmentWorkbenchRepository) *InvestmentWorkbenchService {
	return &InvestmentWorkbenchService{repository: repository}
}

// Compare joins observed sales and new whole-home leases in a shared, complete
// 12-month window. The yield is a ratio of district medians, not a property's return.
func (service *InvestmentWorkbenchService) Compare(ctx context.Context, query domain.InvestmentQuery) (domain.InvestmentWorkbenchReport, error) {
	// Reuse the existing cohort validation before querying coverage.
	if err := validateInvestmentQuery(query); err != nil {
		return domain.InvestmentWorkbenchReport{}, err
	}
	report := domain.InvestmentWorkbenchReport{CityCode: query.CityCode, MinimumSales: minimumInvestmentSample, MinimumRentals: minimumWorkbenchRentals, Areas: []domain.InvestmentWorkbenchArea{}}
	saleSeasons, saleReleases, err := service.repository.ImportedSalesByCity(ctx, query.CityCode)
	if err != nil {
		return report, err
	}
	rentSeasons, rentReleases, err := service.repository.ImportedRentalsByCity(ctx, query.CityCode)
	if err != nil {
		return report, err
	}
	saleThrough, saleKnown := registeredThrough(saleSeasons, saleReleases)
	rentThrough, rentKnown := registeredThrough(rentSeasons, rentReleases)
	if !saleKnown || !rentKnown {
		return report, nil
	}
	if rentThrough.Before(saleThrough) {
		saleThrough = rentThrough
	}
	complete := completeThrough(saleThrough)
	window := domain.InvestmentWindow{CurrentFrom: complete.AddDate(0, -11, 0), EndExclusive: complete.AddDate(0, 1, 0)}
	window.PreviousFrom = window.CurrentFrom.AddDate(-1, 0, 0)
	report.Through, report.CurrentFrom, report.PreviousFrom = complete.Format("2006-01"), window.CurrentFrom.Format("2006-01"), window.PreviousFrom.Format("2006-01")
	sales, err := service.repository.InvestmentAggregates(ctx, query, window)
	if err != nil {
		return report, err
	}
	rents, err := service.repository.InvestmentRentAggregates(ctx, query, window.CurrentFrom, window.EndExclusive)
	if err != nil {
		return report, err
	}
	rentByDistrict := make(map[string]domain.InvestmentRentAggregate, len(rents))
	for _, rent := range rents {
		rentByDistrict[rent.District] = rent
	}
	var cityMedian *float64
	for _, sale := range sales {
		if sale.District == "" && sale.CurrentCount >= minimumInvestmentSample {
			cityMedian = sale.MedianPricePerPing
			break
		}
	}
	for _, sale := range sales {
		if sale.District == "" {
			continue
		}
		area := domain.InvestmentWorkbenchArea{InvestmentArea: domain.InvestmentArea{District: sale.District, CurrentCount: sale.CurrentCount, PreviousCount: sale.PreviousCount,
			SufficientCurrentSample:    sale.CurrentCount >= minimumInvestmentSample,
			SufficientComparisonSample: sale.CurrentCount >= minimumInvestmentSample && sale.PreviousCount >= minimumInvestmentSample}}
		if area.SufficientCurrentSample {
			area.MedianPricePerPing = sale.MedianPricePerPing
			if sale.MedianPricePerPing != nil && cityMedian != nil {
				area.RelativeToCityMedian = change(*sale.MedianPricePerPing, *cityMedian)
			}
		}
		if area.SufficientComparisonSample {
			area.VolumeChange = change(float64(sale.CurrentCount), float64(sale.PreviousCount))
			if sale.MedianPricePerPing != nil && sale.PreviousMedianPricePerPing != nil {
				area.PriceChange = change(*sale.MedianPricePerPing, *sale.PreviousMedianPricePerPing)
			}
		}
		rent := rentByDistrict[sale.District]
		area.RentalCount = rent.Count
		if rent.Count >= minimumWorkbenchRentals {
			area.MedianRentPerPing = rent.MedianRentPerPing
			if area.MedianPricePerPing != nil && rent.MedianRentPerPing != nil && *area.MedianPricePerPing > 0 {
				value := *rent.MedianRentPerPing * 12 / *area.MedianPricePerPing
				area.GrossYield = &value
			}
		}
		report.Areas = append(report.Areas, area)
	}
	sort.Slice(report.Areas, func(i, j int) bool {
		return report.Areas[i].CurrentCount > report.Areas[j].CurrentCount || report.Areas[i].CurrentCount == report.Areas[j].CurrentCount && report.Areas[i].District < report.Areas[j].District
	})
	return report, nil
}
