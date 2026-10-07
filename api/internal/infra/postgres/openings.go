package postgres

import (
	"context"
	"fmt"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Openings lists every line with its opening date, the platforms with their own date, and
// all platforms as candidates for one.
func (repository *TransactionRepository) Openings(ctx context.Context) (domain.Openings, error) {
	var openings domain.Openings
	rows, err := repository.pool.Query(ctx, `SELECT l.id, l.name, l.system, l.color, to_char(o.opened_on, 'YYYY-MM-DD'),
            COALESCE(o.source, ''), COALESCE(o.updated_by, ''), o.updated_at
        FROM transit_lines l LEFT JOIN transit_openings o ON o.target_kind = 'line' AND o.target_id = l.id
        ORDER BY array_position(ARRAY['metro', 'lrt', 'tra', 'thsr'], l.system), l.id`)
	if err != nil {
		return openings, fmt.Errorf("query line openings: %w", err)
	}
	if openings.Lines, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.LineOpening, error) {
		var line domain.LineOpening
		err := row.Scan(&line.LineID, &line.Name, &line.System, &line.Color, &line.OpenedOn, &line.Source, &line.UpdatedBy, &line.UpdatedAt)
		return line, err
	}); err != nil {
		return openings, fmt.Errorf("scan line openings: %w", err)
	}

	// One row per platform; a platform on several lines (some TRA stations) lists them all.
	const platforms = `SELECT sl.platform_id, MIN(sl.platform_name), MIN(s.name),
            string_agg(DISTINCT l.name, '、'), MIN(l.color),
            to_char(CASE WHEN bool_or(lo.opened_on IS NULL) THEN NULL ELSE MIN(lo.opened_on) END, 'YYYY-MM-DD')
        FROM transit_station_lines sl
        JOIN transit_stations s ON s.id = sl.station_id
        JOIN transit_lines l ON l.id = sl.line_id
        LEFT JOIN transit_openings lo ON lo.target_kind = 'line' AND lo.target_id = sl.line_id
        GROUP BY sl.platform_id`
	rows, err = repository.pool.Query(ctx, `WITH platforms AS (`+platforms+`)
        SELECT p.*, to_char(o.opened_on, 'YYYY-MM-DD'), o.source, o.updated_by, o.updated_at
        FROM transit_openings o JOIN platforms p ON p.platform_id = o.target_id
        WHERE o.target_kind = 'platform' ORDER BY o.opened_on, p.platform_id`)
	if err != nil {
		return openings, fmt.Errorf("query platform openings: %w", err)
	}
	if openings.Platforms, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PlatformOpening, error) {
		var platform domain.PlatformOpening
		err := row.Scan(&platform.PlatformID, &platform.PlatformName, &platform.StationName, &platform.Lines, &platform.LineColor,
			&platform.LineOpenedOn, &platform.OpenedOn, &platform.Source, &platform.UpdatedBy, &platform.UpdatedAt)
		return platform, err
	}); err != nil {
		return openings, fmt.Errorf("scan platform openings: %w", err)
	}

	rows, err = repository.pool.Query(ctx, `SELECT platform_id, MIN(sl.platform_name), MIN(s.name), string_agg(DISTINCT l.name, '、')
        FROM transit_station_lines sl JOIN transit_stations s ON s.id = sl.station_id JOIN transit_lines l ON l.id = sl.line_id
        GROUP BY platform_id ORDER BY platform_id`)
	if err != nil {
		return openings, fmt.Errorf("query platforms: %w", err)
	}
	if openings.Options, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PlatformOption, error) {
		var option domain.PlatformOption
		err := row.Scan(&option.PlatformID, &option.PlatformName, &option.StationName, &option.Lines)
		return option, err
	}); err != nil {
		return openings, fmt.Errorf("scan platforms: %w", err)
	}
	if openings.Lines == nil {
		openings.Lines = []domain.LineOpening{}
	}
	if openings.Platforms == nil {
		openings.Platforms = []domain.PlatformOpening{}
	}
	if openings.Options == nil {
		openings.Options = []domain.PlatformOption{}
	}
	return openings, nil
}

// SaveOpening applies an edit and copies the dates onto the stored network in one
// transaction, failing with domain.ErrUnknownTarget for a line or platform not stored.
func (repository *TransactionRepository) SaveOpening(ctx context.Context, edit domain.OpeningEdit, updatedBy string) error {
	return pgx.BeginFunc(ctx, repository.pool, func(tx pgx.Tx) error {
		exists := `SELECT EXISTS (SELECT 1 FROM transit_lines WHERE id = $1)`
		if edit.Kind == "platform" {
			exists = `SELECT EXISTS (SELECT 1 FROM transit_station_lines WHERE platform_id = $1)`
		}
		var found bool
		if err := tx.QueryRow(ctx, exists, edit.TargetID).Scan(&found); err != nil {
			return fmt.Errorf("check opening target: %w", err)
		}
		if !found {
			return domain.ErrUnknownTarget
		}
		var err error
		if edit.OpenedOn == nil {
			_, err = tx.Exec(ctx, `DELETE FROM transit_openings WHERE target_kind = $1 AND target_id = $2`, edit.Kind, edit.TargetID)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO transit_openings (target_kind, target_id, opened_on, source, updated_by, updated_at)
                VALUES ($1, $2, $3::DATE, $4, $5, NOW())
                ON CONFLICT (target_kind, target_id) DO UPDATE SET opened_on = EXCLUDED.opened_on,
                    source = EXCLUDED.source, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
				edit.Kind, edit.TargetID, *edit.OpenedOn, edit.Source, updatedBy)
		}
		if err != nil {
			return fmt.Errorf("save opening: %w", err)
		}
		if _, err = tx.Exec(ctx, `SELECT refresh_transit_openings()`); err != nil {
			return fmt.Errorf("apply opening dates: %w", err)
		}
		return nil
	})
}
