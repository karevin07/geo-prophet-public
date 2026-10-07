package districtmap_test

import (
	"testing"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/districtmap"
)

// grid places count door numbers per location, spaced about 1.1 km apart east-west.
func grid(code string, lat float64, lngs []float64, count int) []addresspoint.Point {
	var points []addresspoint.Point
	for _, lng := range lngs {
		for index := range count {
			points = append(points, addresspoint.Point{DistrictCode: code, Lat: lat + float64(index)*1e-5, Lng: lng})
		}
	}
	return points
}

func TestBuildAssignsCellsToMajorityDistrict(t *testing.T) {
	districts := map[string]string{"大安區": "63000030", "信義區": "63000020"}
	var points []addresspoint.Point
	points = append(points, grid("63000030", 25.03, []float64{121.53, 121.54}, 5)...)
	points = append(points, grid("63000020", 25.03, []float64{121.57, 121.58}, 5)...)
	// Two stray 信義區 door numbers inside a 大安區 cell do not flip it.
	points = append(points, grid("63000020", 25.03, []float64{121.53}, 2)...)
	// Too few door numbers: ignored.
	points = append(points, grid("63000020", 25.10, []float64{121.60}, 2)...)
	// Unknown district codes are ignored.
	points = append(points, grid("99999999", 25.03, []float64{121.55}, 9)...)

	boundaries, err := districtmap.Build(points, districts)
	if err != nil {
		t.Fatalf("Build returned an error: %v", err)
	}
	if len(boundaries) != 2 || boundaries[0].District != "信義區" || boundaries[1].District != "大安區" {
		t.Fatalf("expected 信義區 then 大安區 (code order), got %#v", boundaries)
	}
	for _, boundary := range boundaries {
		if boundary.Cells != 2 || len(boundary.Coordinates) == 0 {
			t.Fatalf("%s: expected 2 cells and polygons, got %d cells, %d polygons", boundary.District, boundary.Cells, len(boundary.Coordinates))
		}
		for _, polygon := range boundary.Coordinates {
			ring := polygon[0]
			if len(polygon) != 1 || len(ring) < 7 || ring[0][0] != ring[len(ring)-1][0] || ring[0][1] != ring[len(ring)-1][1] {
				t.Fatalf("%s: expected one closed outer ring per polygon, got %v", boundary.District, polygon)
			}
			if lng, lat := ring[0][0], ring[0][1]; lng < 121 || lng > 122 || lat < 24.9 || lat > 25.2 {
				t.Fatalf("%s: expected [lng, lat] order, got %v", boundary.District, ring[0])
			}
		}
	}
}
