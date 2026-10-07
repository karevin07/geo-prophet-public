// Package geocode locates 實價登錄 addresses using door-number coordinate open data.
package geocode

import (
	"regexp"
	"strconv"
	"strings"
)

// Address is a parsed street address. Number is 0 when only the road could be parsed.
type Address struct {
	Road      string
	Lane      string
	Alley     string
	Number    int
	SubNumber int
}

const (
	villagePattern = `(?:[^\d路街巷弄號]{1,3}[里村](?:\d+鄰)?)?`
	roadPattern    = `(?P<road>.+?(?:路|街|大道)(?:[一二三四五六七八九十]+段)?)`
)

const numberPattern = `(?P<lane>(?:\d+|[^\d巷弄號]{1,4})巷)?(?P<alley>\d+弄)?(?P<number>\d+)(?:之(?P<sub>\d+))?號`

var (
	fullAddressPattern = regexp.MustCompile(`^` + villagePattern + roadPattern + numberPattern)
	// Rural addresses name a place (地區) instead of a road, e.g. 三芝區海尾13之67號.
	placeAddressPattern = regexp.MustCompile(`^(?P<road>[^\d巷弄號]+?)` + numberPattern)
	roadOnlyPattern     = regexp.MustCompile(`^` + villagePattern + roadPattern)
	dashNumberPattern   = regexp.MustCompile(`(\d+)-(\d+)號`)
	digitSectionPattern = regexp.MustCompile(`\d{1,2}段`)
)

// ParseAddress parses a normalized address (half-width digits, 台 for 臺) after removing
// its leading city and district names.
func ParseAddress(address, cityName, district string) (Address, bool) {
	address = strings.TrimPrefix(address, cityName)
	address = strings.TrimPrefix(address, district)
	address = dashNumberPattern.ReplaceAllString(address, "${1}之${2}號") // 221-4號 means 221之4號
	address = digitSectionPattern.ReplaceAllStringFunc(address, func(section string) string {
		number, _ := strconv.Atoi(strings.TrimSuffix(section, "段"))
		return chineseNumber(number) + "段" // older seasons write 和平西路2段 for 和平西路二段
	})

	for _, pattern := range []*regexp.Regexp{fullAddressPattern, placeAddressPattern} {
		match := pattern.FindStringSubmatch(address)
		if match == nil {
			continue
		}
		parsed := Address{
			Road:  match[pattern.SubexpIndex("road")],
			Lane:  match[pattern.SubexpIndex("lane")],
			Alley: match[pattern.SubexpIndex("alley")],
		}
		parsed.Number, _ = strconv.Atoi(match[pattern.SubexpIndex("number")])
		if sub := match[pattern.SubexpIndex("sub")]; sub != "" {
			parsed.SubNumber, _ = strconv.Atoi(sub)
		}
		return parsed, true
	}
	if match := roadOnlyPattern.FindStringSubmatch(address); match != nil {
		return Address{Road: match[roadOnlyPattern.SubexpIndex("road")]}, true
	}
	return Address{}, false
}

var chineseDigits = []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九"}

// chineseNumber spells 1-99 the way road sections are written, e.g. 十二.
func chineseNumber(number int) string {
	switch {
	case number <= 0 || number >= 100:
		return strconv.Itoa(number)
	case number < 10:
		return chineseDigits[number]
	case number < 20:
		return "十" + chineseDigits[number%10]
	default:
		return chineseDigits[number/10] + "十" + chineseDigits[number%10]
	}
}
