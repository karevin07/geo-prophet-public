package lvr

import (
	"archive/zip"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/twtext"
)

// Presale columns after the sales ones; older seasons lack them.
const (
	columnProjectName  = "建案名稱"
	columnUnit         = "棟及號"
	columnCancellation = "解約情形"
)

// lotCount drops the 共13筆 that some addresses end with, which defeats geocoding.
var lotCount = regexp.MustCompile(`\s*共\d+筆$`)

// PresalesResult holds the usable presales of one file and why the other rows were skipped.
type PresalesResult struct {
	Presales []domain.Presale
	Rows     int
	Skipped  map[SkipReason]int
}

// PresalesFileName returns the presale (預售屋) file name inside the season archive.
func PresalesFileName(cityCode string) string { return cityCode + "_lvr_land_b.csv" }

// OpenPresales opens a city's presale file inside a season archive.
func OpenPresales(archive *zip.Reader, cityCode string) (io.ReadCloser, error) {
	if _, ok := Cities[cityCode]; !ok {
		return nil, fmt.Errorf("unknown city code %q", cityCode)
	}
	file, err := archive.Open(PresalesFileName(cityCode))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", PresalesFileName(cityCode), err)
	}
	return file, nil
}

// ParsePresales reads a presale CSV, which has the sales columns plus the project, unit
// and cancellation, and returns normalized building presales.
func ParsePresales(reader io.Reader, cityCode, season string) (PresalesResult, error) {
	cityName, ok := Cities[cityCode]
	if !ok {
		return PresalesResult{}, fmt.Errorf("unknown city code %q", cityCode)
	}
	if !seasonPattern.MatchString(season) {
		return PresalesResult{}, fmt.Errorf("invalid season %q, expected e.g. 115S2", season)
	}
	result := PresalesResult{Skipped: map[SkipReason]int{}}
	rows, malformed, err := scanRows(reader, requiredColumns, "presale", func(row rowFunc) {
		transaction, reason := normalizeSale(row, cityCode, cityName, season)
		if reason != "" {
			result.Skipped[reason]++
			return
		}
		transaction.Address = lotCount.ReplaceAllString(transaction.Address, "")
		presale := domain.Presale{
			Transaction:  transaction,
			ProjectName:  twtext.Normalize(row(columnProjectName)),
			UnitLabel:    twtext.Normalize(row(columnUnit)),
			Cancellation: row(columnCancellation),
		}
		if presale.ProjectName != "" {
			presale.ProjectKey = strings.Join([]string{cityCode, transaction.District, presale.ProjectName}, "|")
		}
		// 解約情形 starts with the date, e.g. 1150329全部解約.
		if date, err := parseROCDate(firstRunes(presale.Cancellation, 7)); err == nil {
			presale.CancelledOn = &date
		}
		result.Presales = append(result.Presales, presale)
	})
	if err != nil {
		return PresalesResult{}, err
	}
	result.Rows = rows
	if malformed > 0 {
		result.Skipped[SkipMalformed] += malformed
	}
	return result, nil
}

func firstRunes(value string, count int) string {
	runes := []rune(value)
	return string(runes[:min(count, len(runes))])
}

// MarkPresaleOutliers applies the sales date and price checks to presales.
func MarkPresaleOutliers(presales []domain.Presale, latest time.Time) {
	transactions := make([]domain.Transaction, len(presales))
	for index := range presales {
		transactions[index] = presales[index].Transaction
	}
	MarkInvalidDates(transactions, latest)
	MarkPriceOutliers(transactions)
	for index := range presales {
		presales[index].SpecialReasons = transactions[index].SpecialReasons
	}
}
