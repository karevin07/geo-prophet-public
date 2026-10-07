package transit

import "testing"

func TestMergeKeepsNeighbouringStopsOfOneOperatorApart(t *testing.T) {
	// 台北小城 and 耕莘安康院區 on 安坑輕軌 are about 130 m apart.
	stations := mergeStations([]Platform{
		{ID: "NTALRT-K03", Operator: "NTALRT", Name: "台北小城", Lat: 24.96388, Lng: 121.52877},
		{ID: "NTALRT-K04", Operator: "NTALRT", Name: "耕莘安康院區", Lat: 24.96499, Lng: 121.52921},
	})
	if len(stations) != 2 {
		t.Fatalf("got %d stations, want 2", len(stations))
	}
}

func TestNames(t *testing.T) {
	for name, want := range map[string]string{"三重站": "三重", "台北車站": "台北車站", "鶯歌車站": "鶯歌車站", "板橋": "板橋"} {
		if got := displayName(name); got != want {
			t.Errorf("displayName(%s) = %s, want %s", name, got, want)
		}
	}
	for _, names := range [][2]string{{"臺北", "台北車站"}, {"三重站", "三重"}, {"新北產業園區站", "新北產業園區"}} {
		if matchName(names[0]) != matchName(names[1]) {
			t.Errorf("%s and %s should match", names[0], names[1])
		}
	}
}
