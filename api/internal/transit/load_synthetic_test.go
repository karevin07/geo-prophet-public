package transit_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/transit"
)

// syntheticGetter supplies authored API contracts, not saved TDX responses.
type syntheticGetter struct{ fail bool }

func (getter syntheticGetter) GetJSON(_ context.Context, path string, into any) error {
	if getter.fail {
		return errors.New("synthetic source unavailable")
	}
	content := "[]"
	if strings.HasPrefix(path, "v3/") {
		content = "{}"
	}
	switch path {
	case "v2/Rail/Metro/Station/TRTC":
		content = `[{"StationID":"DEMO01","StationName":{"Zh_tw":"示範站","En":"Demo"},"LocationCity":"臺北市","LocationTown":"中山區","StationPosition":{"PositionLat":25.06,"PositionLon":121.53}}]`
	case "v2/Rail/Metro/Line/TRTC":
		content = `[{"LineID":"DEMO","LineName":{"Zh_tw":"示範線"},"LineColor":"#1F6F78"}]`
	case "v2/Rail/Metro/StationOfLine/TRTC":
		content = `[{"LineID":"DEMO","Stations":[{"Sequence":1,"StationID":"DEMO01"}]}]`
	case "v2/Rail/Metro/Shape/TRTC":
		content = `[{"LineID":"DEMO","Geometry":"LINESTRING(121.53 25.06,121.54 25.07)"}]`
	case "v2/Rail/Metro/StationExit/TRTC":
		content = `[{"StationID":"DEMO01","ExitID":"E1","ExitName":{"Zh_tw":"示範出口"},"ExitPosition":{"PositionLat":25.0601,"PositionLon":121.5301}}]`
	}
	return json.Unmarshal([]byte(content), into)
}

func TestLoadSyntheticNetwork(t *testing.T) {
	network, err := transit.Load(context.Background(), syntheticGetter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(network.Stations) != 1 || network.Stations[0].ID != "TRTC-DEMO01" || len(network.Stations[0].Exits()) != 1 {
		t.Fatalf("unexpected synthetic network: %+v", network)
	}
}

func TestLoadSyntheticSourceFailure(t *testing.T) {
	if _, err := transit.Load(context.Background(), syntheticGetter{fail: true}); err == nil {
		t.Fatal("expected source failure")
	}
}
