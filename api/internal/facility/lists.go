package facility

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

// Listed is a facility from a government list, located later from its address.
type Listed struct {
	ID       string
	Category string
	Name     string
	CityCode string
	District string
	Address  string
}

// cityAddress matches the city and district at the start of an address; MOE addresses
// start with a postcode such as [220].
var cityAddress = regexp.MustCompile(`^(?:\[\d+\])?(台北市|新北市|桃園市|桃園縣)(\S+?[區市鎮鄉])`)

var cityCodes = map[string]string{"台北市": "a", "新北市": "f", "桃園市": "h", "桃園縣": "h"}

// locateAddress returns the city code, district and normalized address, or false outside
// the covered cities.
func locateAddress(raw string) (string, string, string, bool) {
	address := twtext.Normalize(raw)
	match := cityAddress.FindStringSubmatch(address)
	if match == nil {
		return "", "", "", false
	}
	address = strings.TrimPrefix(address, match[0][:len(match[0])-len(match[1])-len(match[2])])
	address = strings.Replace(address, "桃園縣", "桃園市", 1)
	return cityCodes[match[1]], match[2], address, true
}

func readCSV(reader io.Reader, required ...string) ([][]string, map[string]int, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, errors.New("empty csv")
	}
	columns := map[string]int{}
	for index, name := range records[0] {
		columns[strings.TrimPrefix(strings.TrimSpace(name), "\ufeff")] = index
	}
	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return nil, nil, fmt.Errorf("csv is missing column %s", name)
		}
	}
	return records[1:], columns, nil
}

// ReadNHI reads a health insurance list of contracted hospitals or clinics (健保特約醫事
// 機構) as category, keeping those in the covered cities.
func ReadNHI(reader io.Reader, category string) ([]Listed, error) {
	rows, columns, err := readCSV(reader, "醫事機構代碼", "醫事機構名稱", "地址")
	if err != nil {
		return nil, err
	}
	var listed []Listed
	for _, row := range rows {
		if len(row) <= columns["地址"] {
			continue
		}
		city, district, address, ok := locateAddress(row[columns["地址"]])
		if !ok {
			continue
		}
		listed = append(listed, Listed{
			ID: "nhi:" + strings.TrimSpace(row[columns["醫事機構代碼"]]), Category: category,
			Name: twtext.Normalize(row[columns["醫事機構名稱"]]), CityCode: city, District: district, Address: address,
		})
	}
	return listed, nil
}

// ReadSchools reads an education ministry school list (學校名錄) as category, keeping the
// latest school year when the file holds several, and schools in the covered cities.
func ReadSchools(reader io.Reader, category string) ([]Listed, error) {
	rows, columns, err := readCSV(reader, "代碼", "學校名稱", "地址")
	if err != nil {
		return nil, err
	}
	yearColumn, hasYears := columns["學年度"]
	latest := 0
	if hasYears {
		for _, row := range rows {
			if len(row) > yearColumn {
				if year, err := strconv.Atoi(strings.TrimSpace(row[yearColumn])); err == nil && year > latest {
					latest = year
				}
			}
		}
	}
	var listed []Listed
	for _, row := range rows {
		if len(row) <= columns["地址"] {
			continue
		}
		if hasYears && strings.TrimSpace(row[yearColumn]) != strconv.Itoa(latest) {
			continue
		}
		city, district, address, ok := locateAddress(row[columns["地址"]])
		if !ok {
			continue
		}
		listed = append(listed, Listed{
			ID: "moe:" + category + ":" + strings.TrimSpace(row[columns["代碼"]]), Category: category,
			Name: twtext.Normalize(row[columns["學校名稱"]]), CityCode: city, District: district, Address: address,
		})
	}
	return listed, nil
}
