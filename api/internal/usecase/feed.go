package usecase

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
)

const (
	FeedPageSize  = 50
	MaxExportRows = 50_000
)

type FeedRepository interface {
	Feed(ctx context.Context, query domain.FeedQuery, latestRunIDs []int64, limit int) ([]domain.Transaction, error)
	FeedCounts(ctx context.Context, query domain.FeedQuery, latestRunIDs []int64) (total, fresh int, err error)
	ExportFeed(ctx context.Context, query domain.FeedQuery, latestRunIDs []int64, limit int, emit func(domain.Transaction) error) error
	// LatestRelease returns the newest imported release label and the runs that wrote it.
	LatestRelease(ctx context.Context) (string, []int64, error)
}

type FeedService struct {
	repository FeedRepository
}

func NewFeedService(repository FeedRepository) *FeedService {
	return &FeedService{repository: repository}
}

// Page returns one page of the feed. Totals are only computed for the first page, since
// the client keeps them while loading more.
func (service *FeedService) Page(ctx context.Context, query domain.FeedQuery) (domain.FeedPage, error) {
	if err := validateFeedQuery(&query); err != nil {
		return domain.FeedPage{}, err
	}
	label, runIDs, err := service.repository.LatestRelease(ctx)
	if err != nil {
		return domain.FeedPage{}, err
	}
	page := domain.FeedPage{Transactions: []domain.Transaction{}, LatestRelease: releaseInfo(label)}

	rows, err := service.repository.Feed(ctx, query, runIDs, FeedPageSize+1)
	if err != nil {
		return domain.FeedPage{}, err
	}
	if len(rows) > FeedPageSize {
		rows = rows[:FeedPageSize]
		page.NextCursor = cursorAfter(rows[len(rows)-1], query.Sort)
	}
	page.Transactions = rows

	if query.Cursor == nil {
		total, fresh, err := service.repository.FeedCounts(ctx, query, runIDs)
		if err != nil {
			return domain.FeedPage{}, err
		}
		page.Total, page.NewCount = &total, &fresh
	}
	return page, nil
}

// Export streams the feed in order, up to MaxExportRows rows.
func (service *FeedService) Export(ctx context.Context, query domain.FeedQuery, emit func(domain.Transaction) error) error {
	query.Cursor = nil
	if err := validateFeedQuery(&query); err != nil {
		return err
	}
	_, runIDs, err := service.repository.LatestRelease(ctx)
	if err != nil {
		return err
	}
	return service.repository.ExportFeed(ctx, query, runIDs, MaxExportRows, emit)
}

// Regions lists the supported cities and their districts in district-code order.
func Regions() []domain.Region {
	regions := make([]domain.Region, 0, len(addresspoint.Sources))
	for code, source := range addresspoint.Sources {
		districts := make([]string, 0, len(source.Districts))
		for name := range source.Districts {
			districts = append(districts, name)
		}
		slices.SortFunc(districts, func(a, b string) int {
			return cmp.Compare(source.Districts[a], source.Districts[b])
		})
		regions = append(regions, domain.Region{Code: code, Name: lvr.Cities[code], Districts: districts})
	}
	slices.SortFunc(regions, func(a, b domain.Region) int { return cmp.Compare(a.Code, b.Code) })
	return regions
}

func validateFeedQuery(query *domain.FeedQuery) error {
	if err := validateFilter(query.Filter); err != nil {
		return err
	}
	if query.Sort == "" {
		query.Sort = domain.SortDateDesc
	}
	if !slices.Contains(domain.FeedSorts, query.Sort) {
		return fmt.Errorf("%w: unknown sort %q", ErrInvalidInput, query.Sort)
	}
	if query.CityCode != "" {
		source, ok := addresspoint.Sources[query.CityCode]
		if !ok {
			return fmt.Errorf("%w: unknown city %q", ErrInvalidInput, query.CityCode)
		}
		if _, ok = source.Districts[query.District]; query.District != "" && !ok {
			return fmt.Errorf("%w: unknown district %q", ErrInvalidInput, query.District)
		}
	} else if query.District != "" {
		return fmt.Errorf("%w: district requires city", ErrInvalidInput)
	}
	return nil
}

func cursorAfter(last domain.Transaction, sort domain.FeedSort) *domain.FeedCursor {
	cursor := &domain.FeedCursor{ID: last.ID}
	switch sort {
	case domain.SortPricePerPingDesc, domain.SortPricePerPingAsc:
		cursor.Value = strconv.FormatFloat(last.PricePerPing, 'g', -1, 64)
	case domain.SortTotalPriceDesc:
		cursor.Value = strconv.FormatInt(last.TotalPrice, 10)
	default:
		cursor.Value = last.TransactedAt.Format("2006-01-02")
	}
	return cursor
}

func releaseInfo(label string) *domain.Release {
	release, err := lvr.ParseReleaseLabel(label)
	if err != nil {
		return nil
	}
	return &domain.Release{Label: label, RegisteredFrom: release.RegisteredFrom, RegisteredTo: release.RegisteredTo}
}
