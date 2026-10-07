package lvr

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/twtext"
)

const squareMetersPerPing = 3.30579

// parseROCDate parses 民國 dates in YYYMMDD form, or YYYMM form as the first of that month.
func parseROCDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	var year, month, day int
	var err error
	switch len(value) {
	case 7:
		year, month, day, err = splitDigits(value, 3, 5)
	case 5:
		year, month, _, err = splitDigits(value+"01", 3, 5)
		day = 1
	default:
		return time.Time{}, fmt.Errorf("unexpected ROC date %q", value)
	}
	if err != nil || year <= 0 {
		return time.Time{}, fmt.Errorf("invalid ROC date %q", value)
	}
	date := time.Date(year+1911, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Month() != time.Month(month) || date.Day() != day {
		return time.Time{}, fmt.Errorf("invalid ROC date %q", value)
	}
	return date, nil
}

func splitDigits(value string, yearEnd, monthEnd int) (int, int, int, error) {
	year, err := strconv.Atoi(value[:yearEnd])
	if err != nil {
		return 0, 0, 0, err
	}
	month, err := strconv.Atoi(value[yearEnd:monthEnd])
	if err != nil {
		return 0, 0, 0, err
	}
	day, err := strconv.Atoi(value[monthEnd:])
	return year, month, day, err
}

var chineseDigits = map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '兩': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}

// parseFloorCount parses floor counts such as "23", "十二層" or "一百零一層".
func parseFloorCount(value string) (int, bool) {
	value = strings.TrimSuffix(twtext.Normalize(value), "層")
	if value == "" {
		return 0, false
	}
	if number, err := strconv.Atoi(value); err == nil {
		return number, number > 0
	}
	total, digit := 0, -1
	for _, character := range value {
		if number, ok := chineseDigits[character]; ok {
			digit = number
			continue
		}
		unit := 0
		switch character {
		case '十':
			unit = 10
		case '百':
			unit = 100
		default:
			return 0, false
		}
		if digit < 0 {
			digit = 1 // "十二" means twelve
		}
		total += digit * unit
		digit = -1
	}
	if digit > 0 {
		total += digit
	}
	return total, total > 0
}

// FloorNumber returns the first floor of a sale's 移轉層次, such as 2 for "二層,陽台".
// Basements, whole buildings (全) and unparsable labels report false.
func FloorNumber(label string) (int, bool) {
	first, _, _ := strings.Cut(strings.ReplaceAll(twtext.Normalize(label), "，", ","), ",")
	first = strings.TrimPrefix(first, "地上")
	if !strings.HasSuffix(first, "層") {
		return 0, false
	}
	return parseFloorCount(first)
}
