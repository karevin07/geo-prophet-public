package lvr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Season is a quarterly 實價登錄 archive such as 115S2. Seasons follow the publication
// cycle on the 1st, 11th and 21st of each month, so S2 holds sales registered from
// March 11 to June 10 (民國 year).
type Season struct {
	Year    int // 民國 year
	Quarter int // 1-4
}

var seasonTextPattern = regexp.MustCompile(`^(\d{3})S([1-4])$`)

func ParseSeason(value string) (Season, error) {
	match := seasonTextPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return Season{}, fmt.Errorf("invalid season %q, expected e.g. 115S2", value)
	}
	year, _ := strconv.Atoi(match[1])
	quarter, _ := strconv.Atoi(match[2])
	return Season{Year: year, Quarter: quarter}, nil
}

// ParseSeasons accepts a single season, a comma-separated list, or an inclusive range
// such as 105S1-115S2.
func ParseSeasons(value string) ([]Season, error) {
	var seasons []Season
	for _, part := range strings.Split(value, ",") {
		first, last, isRange := strings.Cut(strings.TrimSpace(part), "-")
		start, err := ParseSeason(first)
		if err != nil {
			return nil, err
		}
		end := start
		if isRange {
			if end, err = ParseSeason(last); err != nil {
				return nil, err
			}
			if end.Before(start) {
				return nil, fmt.Errorf("season range %q ends before it starts", part)
			}
		}
		for season := start; !end.Before(season); season = season.Next() {
			seasons = append(seasons, season)
		}
	}
	return seasons, nil
}

func (season Season) String() string { return fmt.Sprintf("%03dS%d", season.Year, season.Quarter) }

func (season Season) Before(other Season) bool {
	return season.Year < other.Year || season.Year == other.Year && season.Quarter < other.Quarter
}

func (season Season) Next() Season {
	if season.Quarter == 4 {
		return Season{Year: season.Year + 1, Quarter: 1}
	}
	return Season{Year: season.Year, Quarter: season.Quarter + 1}
}

func (season Season) Previous() Season {
	if season.Quarter == 1 {
		return Season{Year: season.Year - 1, Quarter: 4}
	}
	return Season{Year: season.Year, Quarter: season.Quarter - 1}
}

// SeasonOfRegistration returns the season whose archive includes sales registered on date.
// Registrations from the 11th onward belong to the next month's publication, which maps
// the Mar 11 - Jun 10 window onto calendar Q2, and so on.
func SeasonOfRegistration(date time.Time) Season {
	year, month := date.Year(), int(date.Month())
	if date.Day() >= 11 {
		month++
	}
	if month > 12 {
		year, month = year+1, 1
	}
	return Season{Year: year - 1911, Quarter: (month-1)/3 + 1}
}

// RegistrationWindow returns the first and last registration dates the season's archive
// covers, e.g. 115/03/11 to 115/06/10 for 115S2.
func (season Season) RegistrationWindow() (time.Time, time.Time) {
	year := season.Year + 1911
	endMonth := time.Month(season.Quarter * 3)
	end := time.Date(year, endMonth, 10, 0, 0, 0, 0, time.UTC)
	start := time.Date(year, endMonth-3, 11, 0, 0, 0, 0, time.UTC)
	return start, end
}

// ParseReleaseLabel reverses Release.Label, e.g. 1150901-1150910.
func ParseReleaseLabel(label string) (Release, error) {
	from, to, ok := strings.Cut(label, "-")
	if !ok || len(from) != 7 || len(to) != 7 {
		return Release{}, fmt.Errorf("invalid release label %q", label)
	}
	start, err := parseROCDate(from)
	if err != nil {
		return Release{}, err
	}
	end, err := parseROCDate(to)
	if err != nil {
		return Release{}, err
	}
	return Release{RegisteredFrom: start, RegisteredTo: end}, nil
}

// Release is one 10-day publication (本期資料) described by its build_time.xml.
type Release struct {
	RegisteredFrom time.Time
	RegisteredTo   time.Time
}

// Label identifies the release by its registration window in 民國 dates, e.g. 1150901-1150910.
func (release Release) Label() string {
	return rocDate(release.RegisteredFrom) + "-" + rocDate(release.RegisteredTo)
}

func (release Release) Season() Season { return SeasonOfRegistration(release.RegisteredTo) }

var releasePattern = regexp.MustCompile(`登記日期\s*(\d+)年(\d+)月(\d+)日至\s*(\d+)年(\d+)月(\d+)日之買賣案件`)

// ParseRelease reads the sales registration window from an archive's build_time.xml.
func ParseRelease(buildTime []byte) (Release, error) {
	match := releasePattern.FindStringSubmatch(string(buildTime))
	if match == nil {
		return Release{}, fmt.Errorf("build_time.xml has no sales registration window")
	}
	numbers := make([]int, 6)
	for index := range numbers {
		numbers[index], _ = strconv.Atoi(match[index+1])
	}
	release := Release{
		RegisteredFrom: time.Date(numbers[0]+1911, time.Month(numbers[1]), numbers[2], 0, 0, 0, 0, time.UTC),
		RegisteredTo:   time.Date(numbers[3]+1911, time.Month(numbers[4]), numbers[5], 0, 0, 0, 0, time.UTC),
	}
	if release.RegisteredTo.Before(release.RegisteredFrom) {
		return Release{}, fmt.Errorf("build_time.xml registration window ends before it starts")
	}
	return release, nil
}

func rocDate(date time.Time) string {
	return fmt.Sprintf("%03d%02d%02d", date.Year()-1911, int(date.Month()), date.Day())
}
