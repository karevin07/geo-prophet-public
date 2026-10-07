package usecase_test

import (
	"context"
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

type placeStub struct {
	addressQuery *domain.PlaceQuery
	roadQuery    *domain.PlaceQuery
	stationName  string
}

// FindStations knows one station, 板橋.
func (stub *placeStub) FindStations(_ context.Context, name string, limit int) ([]domain.Place, error) {
	stub.stationName = name
	if name != "板橋" || limit < 1 {
		return nil, nil
	}
	return []domain.Place{{Kind: domain.PlaceStation, Label: "板橋｜新北市板橋區", StationID: "TRTC-BL07", Lat: 25.0138, Lng: 121.4623}}, nil
}

func (stub *placeStub) FindAddresses(_ context.Context, query domain.PlaceQuery) ([]domain.PlaceMatch, error) {
	stub.addressQuery = &query
	return []domain.PlaceMatch{{CityCode: "a", DistrictCode: "63000030", Road: query.Road, Number: query.Number, SubNumber: query.SubNumber, Lat: 25.04, Lng: 121.55}}, nil
}

func (stub *placeStub) FindRoads(_ context.Context, query domain.PlaceQuery) ([]domain.PlaceMatch, error) {
	stub.roadQuery = &query
	return []domain.PlaceMatch{{CityCode: "a", DistrictCode: "63000030", Road: query.Road, Lat: 25.041, Lng: 121.551}}, nil
}

func (*placeStub) DistrictCenter(_ context.Context, cityCode, districtCode string) (float64, float64, bool, error) {
	return 25.0, 121.5, cityCode != "" && districtCode != "", nil
}

func TestSearchFindsDistrictByShortName(t *testing.T) {
	places, err := usecase.NewPlaceService(&placeStub{}).Search(context.Background(), "大安")
	if err != nil || len(places) != 1 || places[0].Kind != domain.PlaceDistrict || places[0].Label != "台北市大安區" {
		t.Fatalf("unexpected result %#v, %v", places, err)
	}
}

func TestSearchListsStationsFirst(t *testing.T) {
	for _, text := range []string{"板橋", "板橋站", "板橋車站"} {
		stub := &placeStub{}
		places, err := usecase.NewPlaceService(stub).Search(context.Background(), text)
		if err != nil || stub.stationName != "板橋" || len(places) == 0 || places[0].StationID != "TRTC-BL07" {
			t.Fatalf("%s: unexpected result %#v (station name %q), %v", text, places, stub.stationName, err)
		}
	}
	places, _ := usecase.NewPlaceService(&placeStub{}).Search(context.Background(), "板橋")
	if len(places) != 2 || places[1].Label != "新北市板橋區" {
		t.Fatalf("expected the station then the district, got %#v", places)
	}
}

func TestSearchParsesFullAddress(t *testing.T) {
	stub := &placeStub{}
	places, err := usecase.NewPlaceService(stub).Search(context.Background(), "臺北市大安區敦化南路一段１００之２號")
	if err != nil {
		t.Fatalf("Search returned an error: %v", err)
	}
	query := stub.addressQuery
	if query == nil || query.CityCode != "a" || len(query.DistrictCodes) != 1 || query.DistrictCodes[0] != "63000030" ||
		query.Road != "敦化南路一段" || query.Number != 100 || query.SubNumber != 2 {
		t.Fatalf("unexpected address query %#v", query)
	}
	if len(places) != 2 || places[0].Label != "台北市大安區敦化南路一段100之2號" || places[1].Kind != domain.PlaceRoad {
		t.Fatalf("expected the address first, then its road: %#v", places)
	}
}

func TestSearchRoadKeywordSkipsAddressLookup(t *testing.T) {
	stub := &placeStub{}
	if _, err := usecase.NewPlaceService(stub).Search(context.Background(), "敦化"); err != nil {
		t.Fatalf("Search returned an error: %v", err)
	}
	if stub.addressQuery != nil || stub.roadQuery == nil || stub.roadQuery.Road != "敦化" {
		t.Fatalf("expected a road keyword search, got address %#v road %#v", stub.addressQuery, stub.roadQuery)
	}
}
