// Package addresspoint parses the door-number coordinate open data (門牌位置坐標資料)
// that city governments publish, one row per floor in TWD97 TM2 coordinates.
package addresspoint

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/geo-prophet/geo-prophet/api/internal/twtext"
)

// Point is one door number without floor, e.g. 新生北路二段133巷12之1號.
type Point struct {
	DistrictCode string
	Road         string // road and section, or the place name (地區) when there is no road
	Lane         string
	Alley        string
	Number       int
	SubNumber    int // the N in 12之N號, 0 when absent
	Lat          float64
	Lng          float64
}

// Key identifies a door number within a city.
type Key struct {
	DistrictCode string
	Road         string
	Lane         string
	Alley        string
	Number       int
	SubNumber    int
}

func (point Point) Key() Key {
	return Key{point.DistrictCode, point.Road, point.Lane, point.Alley, point.Number, point.SubNumber}
}

type SkipReason string

const (
	SkipTemporaryNumber   SkipReason = "temporary_number"
	SkipInvalidNumber     SkipReason = "invalid_number"
	SkipInvalidCoordinate SkipReason = "invalid_coordinate"
	SkipMalformed         SkipReason = "malformed"
)

// ParseResult holds unique door numbers; floors of the same door number are merged.
type ParseResult struct {
	Points  []Point
	Rows    int
	Skipped map[SkipReason]int
}

// Every published city file shares this column order, although header names differ
// (Chinese for Taipei and Taoyuan, English for New Taipei).
const (
	columnDistrictCode = 1
	columnRoad         = 4
	columnArea         = 5 // place name used instead of a road in rural addresses, e.g. 三芝區海尾
	columnLane         = 6
	columnAlley        = 7
	columnNumber       = 8
	columnX            = 9
	columnY            = 10
	columnCount        = 11
)

var numberPattern = regexp.MustCompile(`^(\d+)(?:之(\d+))?號`)

// Parse reads a city's door-number coordinate CSV.
func Parse(reader io.Reader) (ParseResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	header, err := csvReader.Read()
	if err != nil {
		return ParseResult{}, fmt.Errorf("read header: %w", err)
	}
	if len(header) < columnCount {
		return ParseResult{}, fmt.Errorf("expected %d columns, header has %d", columnCount, len(header))
	}

	result := ParseResult{Skipped: map[SkipReason]int{}}
	seen := map[Key]struct{}{}
	for {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ParseResult{}, fmt.Errorf("read row %d: %w", result.Rows+1, err)
		}
		result.Rows++
		point, reason := parseRow(record)
		if reason != "" {
			result.Skipped[reason]++
			continue
		}
		if _, ok := seen[point.Key()]; ok {
			continue
		}
		seen[point.Key()] = struct{}{}
		result.Points = append(result.Points, point)
	}
	return result, nil
}

func parseRow(record []string) (Point, SkipReason) {
	if len(record) < columnCount {
		return Point{}, SkipMalformed
	}
	number := twtext.Normalize(record[columnNumber])
	if strings.HasPrefix(number, "臨") {
		return Point{}, SkipTemporaryNumber
	}
	match := numberPattern.FindStringSubmatch(number)
	if match == nil {
		return Point{}, SkipInvalidNumber
	}
	point := Point{
		DistrictCode: strings.TrimSpace(record[columnDistrictCode]),
		Road:         twtext.Normalize(record[columnRoad]),
		Lane:         twtext.Normalize(record[columnLane]),
		Alley:        twtext.Normalize(record[columnAlley]),
	}
	point.Number, _ = strconv.Atoi(match[1])
	if match[2] != "" {
		point.SubNumber, _ = strconv.Atoi(match[2])
	}
	if point.Road == "" {
		point.Road = twtext.Normalize(record[columnArea])
	}
	if point.DistrictCode == "" || point.Road == "" {
		return Point{}, SkipMalformed
	}

	x, xErr := strconv.ParseFloat(strings.TrimSpace(record[columnX]), 64)
	y, yErr := strconv.ParseFloat(strings.TrimSpace(record[columnY]), 64)
	// Rough TM2 extent of Taiwan's main island; rejects blanks, zeros and swapped axes.
	if xErr != nil || yErr != nil || x < 140_000 || x > 360_000 || y < 2_400_000 || y > 2_800_000 {
		return Point{}, SkipInvalidCoordinate
	}
	point.Lat, point.Lng = TWD97ToWGS84(x, y)
	return point, ""
}
