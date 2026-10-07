package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

type InvestmentLiquidityRepository interface {
	ImportedSalesByCity(context.Context, string) ([]string, []string, error)
	InvestmentMonthlyCounts(context.Context, domain.InvestmentQuery, time.Time, time.Time) ([]domain.InvestmentMonthlyCount, error)
}

type InvestmentLiquidityService struct{ repository InvestmentLiquidityRepository }

func NewInvestmentLiquidityService(repository InvestmentLiquidityRepository) *InvestmentLiquidityService {
	return &InvestmentLiquidityService{repository: repository}
}

// Compare counts observed registered sales. It cannot measure listing-to-sale
// time, inventory turnover, or probability of selling without listing data.
func (service *InvestmentLiquidityService) Compare(ctx context.Context, query domain.InvestmentQuery) (domain.InvestmentLiquidityReport, error) {
	if err := validateInvestmentQuery(query); err != nil {
		return domain.InvestmentLiquidityReport{}, err
	}
	report := domain.InvestmentLiquidityReport{CityCode: query.CityCode, MinimumSample: minimumInvestmentSample, Areas: []domain.InvestmentLiquidityArea{}, IncompleteMonths: []string{}}
	seasons, releases, err := service.repository.ImportedSalesByCity(ctx, query.CityCode)
	if err != nil {
		return report, err
	}
	registered, known := registeredThrough(seasons, releases)
	if !known {
		return report, nil
	}
	complete := completeThrough(registered)
	currentFrom := complete.AddDate(0, -11, 0)
	previousFrom := currentFrom.AddDate(-1, 0, 0)
	report.Through, report.CurrentFrom, report.PreviousFrom = complete.Format("2006-01"), currentFrom.Format("2006-01"), previousFrom.Format("2006-01")
	coverage := liquidityRegistrationWindows(seasons, releases)
	monthCoverage := make(map[string]bool, 24)
	report.CurrentCoverageComplete, report.PreviousCoverageComplete = true, true
	for index := 0; index < 24; index++ {
		month := previousFrom.AddDate(0, index, 0)
		// Cover registrations from the transaction month's start through its
		// final day plus the same 90-day lag used to choose the report window.
		covered := liquidityWindowCovered(coverage, month, month.AddDate(0, 1, 0).Add(registrationLag))
		label := month.Format("2006-01")
		monthCoverage[label] = covered
		if !covered {
			report.IncompleteMonths = append(report.IncompleteMonths, label)
			if index < 12 {
				report.PreviousCoverageComplete = false
			} else {
				report.CurrentCoverageComplete = false
			}
		}
	}
	rows, err := service.repository.InvestmentMonthlyCounts(ctx, query, previousFrom, complete.AddDate(0, 1, 0))
	if err != nil {
		return report, err
	}
	byDistrict := map[string]map[string]int{}
	for _, row := range rows {
		if byDistrict[row.District] == nil {
			byDistrict[row.District] = map[string]int{}
		}
		byDistrict[row.District][row.Month] = row.Count
	}
	for district, counts := range byDistrict {
		area := domain.InvestmentLiquidityArea{District: district, Monthly: make([]domain.InvestmentMonthlyCount, 0, 24)}
		active, longest, zeroRun := 0, 0, 0
		for index := 0; index < 24; index++ {
			month := previousFrom.AddDate(0, index, 0).Format("2006-01")
			count := counts[month]
			area.Monthly = append(area.Monthly, domain.InvestmentMonthlyCount{Month: month, Count: count, CoverageComplete: monthCoverage[month]})
			if index < 12 {
				area.PreviousCount += count
				continue
			}
			area.CurrentCount += count
			if count > 0 {
				active++
				zeroRun = 0
			} else {
				zeroRun++
				if zeroRun > longest {
					longest = zeroRun
				}
			}
		}
		area.SufficientCurrentSample = area.CurrentCount >= minimumInvestmentSample
		area.SufficientComparisonSample = area.SufficientCurrentSample && area.PreviousCount >= minimumInvestmentSample
		if area.SufficientCurrentSample && report.CurrentCoverageComplete {
			area.ActiveMonths, area.LongestZeroRun = &active, &longest
		}
		if area.SufficientComparisonSample && report.CurrentCoverageComplete && report.PreviousCoverageComplete {
			area.VolumeChange = change(float64(area.CurrentCount), float64(area.PreviousCount))
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

type liquidityRegistrationWindow struct{ from, to time.Time }

func liquidityRegistrationWindows(seasons, releases []string) []liquidityRegistrationWindow {
	windows := make([]liquidityRegistrationWindow, 0, len(seasons)+len(releases))
	for _, label := range seasons {
		if season, err := lvr.ParseSeason(label); err == nil {
			from, through := season.RegistrationWindow()
			windows = append(windows, liquidityRegistrationWindow{from, through.AddDate(0, 0, 1)})
		}
	}
	for _, label := range releases {
		if release, err := lvr.ParseReleaseLabel(label); err == nil && !release.RegisteredTo.Before(release.RegisteredFrom) {
			windows = append(windows, liquidityRegistrationWindow{release.RegisteredFrom, release.RegisteredTo.AddDate(0, 0, 1)})
		}
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i].from.Before(windows[j].from) })
	return windows
}

// Bounds are half-open. Overlapping season/release archives count only once.
func liquidityWindowCovered(windows []liquidityRegistrationWindow, from, to time.Time) bool {
	cursor := from
	for _, window := range windows {
		if !window.to.After(cursor) {
			continue
		}
		if window.from.After(cursor) {
			return false
		}
		cursor = window.to
		if !cursor.Before(to) {
			return true
		}
	}
	return false
}
