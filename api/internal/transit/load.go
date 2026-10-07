package transit

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// operator describes one TDX rail operator. Operators are listed in platform priority
// order: a merged station takes its ID and name from the first, so 台北車站 keeps the
// MRT's name rather than 台鐵's 臺北.
type operator struct {
	code   string
	system System
	exits  bool // light rail publishes no exits
	color  string
}

var operators = []operator{
	{code: "TRTC", system: SystemMetro, exits: true},
	{code: "NTMC", system: SystemMetro, exits: true},
	{code: "TYMC", system: SystemMetro, exits: true},
	{code: "NTDLRT", system: SystemLRT},
	{code: "NTALRT", system: SystemLRT},
	{code: "TRA", system: SystemTRA, exits: true, color: "#1F4E9C"},
	{code: "THSR", system: SystemTHSR, exits: true, color: "#E8732A"},
}

// cityCodes maps TDX city names to 實價登錄 city codes; other cities are skipped.
var cityCodes = map[string]string{"臺北市": "a", "台北市": "a", "新北市": "f", "桃園市": "h"}

type localized struct {
	ZhTw string `json:"Zh_tw"`
	En   string `json:"En"`
}

type position struct {
	Lon float64 `json:"PositionLon"`
	Lat float64 `json:"PositionLat"`
}

type apiStation struct {
	StationID    string    `json:"StationID"`
	StationName  localized `json:"StationName"`
	Address      string    `json:"StationAddress"`
	Position     position  `json:"StationPosition"`
	LocationCity string    `json:"LocationCity"`
	LocationTown string    `json:"LocationTown"`
}

type apiLine struct {
	LineID    string    `json:"LineID"`
	LineName  localized `json:"LineName"`
	LineColor string    `json:"LineColor"`
}

type apiStationOfLine struct {
	LineID   string `json:"LineID"`
	Stations []struct {
		Sequence  int    `json:"Sequence"`
		StationID string `json:"StationID"`
	} `json:"Stations"`
}

type apiShape struct {
	LineID   string `json:"LineID"`
	Geometry string `json:"Geometry"`
}

type apiExit struct {
	StationID string    `json:"StationID"`
	ExitID    string    `json:"ExitID"`
	ExitName  localized `json:"ExitName"`
	Position  position  `json:"ExitPosition"`
}

// operatorData is one operator's responses, normalized across API versions.
type operatorData struct {
	stations       []apiStation
	lines          []apiLine
	stationsOfLine []apiStationOfLine
	shapes         []apiShape
	exits          []apiExit
}

// Load downloads every operator's stations, lines, routes and exits and keeps the
// stations in the covered cities, with the lines serving them.
func Load(ctx context.Context, getter Getter) (Network, error) {
	var platforms []Platform
	var lines []Line
	for _, operator := range operators {
		data, err := fetchOperator(ctx, getter, operator)
		if err != nil {
			return Network{}, fmt.Errorf("%s: %w", operator.code, err)
		}
		operatorPlatforms, operatorLines, err := build(operator, data)
		if err != nil {
			return Network{}, fmt.Errorf("%s: %w", operator.code, err)
		}
		platforms = append(platforms, operatorPlatforms...)
		lines = append(lines, operatorLines...)
	}
	return Network{Stations: mergeStations(platforms), Lines: lines}, nil
}

func fetchOperator(ctx context.Context, getter Getter, operator operator) (operatorData, error) {
	var data operatorData
	switch operator.code {
	case "TRA":
		// TRA's v3 API wraps each list in an object.
		var stations struct{ Stations []apiStation }
		var lines struct{ Lines []apiLine }
		var stationsOfLine struct{ StationOfLines []apiStationOfLine }
		var shapes struct{ Shapes []apiShape }
		var exits struct {
			StationExits []struct {
				StationID string    `json:"StationID"`
				Exits     []apiExit `json:"Exits"`
			}
		}
		for path, into := range map[string]any{
			"v3/Rail/TRA/Station": &stations, "v3/Rail/TRA/Line": &lines, "v3/Rail/TRA/StationOfLine": &stationsOfLine,
			"v3/Rail/TRA/Shape": &shapes, "v3/Rail/TRA/StationExit": &exits,
		} {
			if err := getter.GetJSON(ctx, path, into); err != nil {
				return data, err
			}
		}
		data = operatorData{stations: stations.Stations, lines: lines.Lines, stationsOfLine: stationsOfLine.StationOfLines, shapes: shapes.Shapes}
		for _, station := range exits.StationExits {
			for _, exit := range station.Exits {
				exit.StationID = station.StationID
				data.exits = append(data.exits, exit)
			}
		}
		return data, nil
	case "THSR":
		// 高鐵 is one line; its stations are listed from 南港 southwards.
		paths := map[string]any{"v2/Rail/THSR/Station": &data.stations, "v2/Rail/THSR/Shape": &data.shapes, "v2/Rail/THSR/StationExit": &data.exits}
		for path, into := range paths {
			if err := getter.GetJSON(ctx, path, into); err != nil {
				return data, err
			}
		}
		stops := apiStationOfLine{LineID: "HSR"}
		for index, station := range data.stations {
			stops.Stations = append(stops.Stations, struct {
				Sequence  int    `json:"Sequence"`
				StationID string `json:"StationID"`
			}{index + 1, station.StationID})
		}
		data.stationsOfLine = []apiStationOfLine{stops}
		data.lines = []apiLine{{LineID: "HSR", LineName: localized{ZhTw: "台灣高鐵", En: "Taiwan High Speed Rail"}}}
		for index := range data.shapes {
			data.shapes[index].LineID = "HSR"
		}
		return data, nil
	}
	paths := map[string]any{
		"v2/Rail/Metro/Station/" + operator.code: &data.stations, "v2/Rail/Metro/Line/" + operator.code: &data.lines,
		"v2/Rail/Metro/StationOfLine/" + operator.code: &data.stationsOfLine, "v2/Rail/Metro/Shape/" + operator.code: &data.shapes,
	}
	if operator.exits {
		paths["v2/Rail/Metro/StationExit/"+operator.code] = &data.exits
	}
	for path, into := range paths {
		if err := getter.GetJSON(ctx, path, into); err != nil {
			return data, err
		}
	}
	return data, nil
}

func build(operator operator, data operatorData) ([]Platform, []Line, error) {
	stops := map[string][]LineStop{}
	for _, line := range data.stationsOfLine {
		for _, stop := range line.Stations {
			stops[stop.StationID] = append(stops[stop.StationID], LineStop{LineID: operator.code + "-" + line.LineID, Sequence: stop.Sequence})
		}
	}
	exits := map[string][]Exit{}
	for _, exit := range data.exits {
		if exit.Position.Lat == 0 || exit.Position.Lon == 0 {
			continue
		}
		exits[exit.StationID] = append(exits[exit.StationID], Exit{ID: exit.ExitID, Name: exit.ExitName.ZhTw, Lat: exit.Position.Lat, Lng: exit.Position.Lon})
	}

	var platforms []Platform
	served := map[string]bool{}
	for _, station := range data.stations {
		// TRA lists virtual stations such as 臺北-環島 for round-island tickets.
		if strings.Contains(station.StationName.ZhTw, "-") || station.Position.Lat == 0 {
			continue
		}
		cityCode, district := locate(station)
		if cityCode == "" {
			continue
		}
		id := operator.code + "-" + station.StationID
		platform := Platform{
			ID: id, Operator: operator.code, System: operator.system, Name: station.StationName.ZhTw, NameEn: station.StationName.En,
			Lat: station.Position.Lat, Lng: station.Position.Lon, CityCode: cityCode, District: district,
			Lines: stops[station.StationID], Exits: exits[station.StationID],
		}
		if len(platform.Lines) == 0 {
			return nil, nil, fmt.Errorf("station %s (%s) is on no line", id, platform.Name)
		}
		if len(platform.Exits) == 0 {
			platform.Exits = []Exit{{ID: "platform", Name: platform.Name, Lat: platform.Lat, Lng: platform.Lng}}
		}
		for _, stop := range platform.Lines {
			served[stop.LineID] = true
		}
		platforms = append(platforms, platform)
	}

	shapes := map[string]string{}
	for _, shape := range data.shapes {
		shapes[operator.code+"-"+shape.LineID] = shape.Geometry
	}
	var lines []Line
	for _, line := range data.lines {
		id := operator.code + "-" + line.LineID
		if !served[id] {
			continue
		}
		if shapes[id] == "" {
			return nil, nil, fmt.Errorf("line %s has no route shape", id)
		}
		lines = append(lines, Line{
			ID: id, Operator: operator.code, System: operator.system, Name: line.LineName.ZhTw, NameEn: line.LineName.En,
			Color: cmp.Or(operator.color, line.LineColor), Geometry: shapes[id],
		})
	}
	slices.SortFunc(platforms, func(a, b Platform) int { return strings.Compare(a.ID, b.ID) })
	return platforms, lines, nil
}

var addressPattern = regexp.MustCompile(`(臺北市|台北市|新北市|桃園市)\s*(\S+?[區市鎮鄉])`)

// locate returns the station's city code and district from its location fields or, for
// TRA which has none, its address such as 221041新北市汐止區….
func locate(station apiStation) (string, string) {
	if code, ok := cityCodes[station.LocationCity]; ok && station.LocationTown != "" {
		return code, station.LocationTown
	}
	match := addressPattern.FindStringSubmatch(station.Address)
	if match == nil {
		return "", ""
	}
	return cityCodes[match[1]], match[2]
}
