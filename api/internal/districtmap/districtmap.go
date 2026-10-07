// Package districtmap approximates district areas from door-number points, because the
// official boundary files are only served to Taiwanese IP addresses.
package districtmap

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/uber/h3-go/v4"
)

const (
	// Resolution 8 cells are about 0.7 km², fine enough for district shapes while staying
	// mostly contiguous across parks and rivers that have no door numbers.
	Resolution = 8
	// minPoints ignores cells with a handful of stray (possibly mis-located) door numbers.
	minPoints = 3
)

// Owners assigns every H3 cell holding at least minPoints door numbers to the district
// owning most of them, keyed by district name.
func Owners(points []addresspoint.Point, districts map[string]string) (map[h3.Cell]string, error) {
	names := make(map[string]string, len(districts))
	for name, code := range districts {
		names[code] = name
	}

	counts := map[h3.Cell]map[string]int{}
	for _, point := range points {
		if _, known := names[point.DistrictCode]; !known {
			continue
		}
		cell, err := h3.LatLngToCell(h3.NewLatLng(point.Lat, point.Lng), Resolution)
		if err != nil {
			return nil, fmt.Errorf("locate door number: %w", err)
		}
		if counts[cell] == nil {
			counts[cell] = map[string]int{}
		}
		counts[cell][point.DistrictCode]++
	}

	owners := make(map[h3.Cell]string, len(counts))
	for cell, byDistrict := range counts {
		owner, most, total := "", 0, 0
		for code, count := range byDistrict {
			total += count
			// Ties go to the lower code so results do not depend on map order.
			if count > most || (count == most && code < owner) {
				owner, most = code, count
			}
		}
		if total >= minPoints {
			owners[cell] = names[owner]
		}
	}
	return owners, nil
}

// Locate returns the district of a point from cell owners: its own cell's, or else the
// most common among the neighbouring cells, which covers parks and rivers without door
// numbers. Points away from every owned cell report false.
func Locate(owners map[h3.Cell]string, lat, lng float64) (string, bool) {
	cell, err := h3.LatLngToCell(h3.NewLatLng(lat, lng), Resolution)
	if err != nil {
		return "", false
	}
	if district, ok := owners[cell]; ok {
		return district, true
	}
	neighbours, err := h3.GridDisk(cell, 1)
	if err != nil {
		return "", false
	}
	votes := map[string]int{}
	for _, neighbour := range neighbours {
		if district, ok := owners[neighbour]; ok {
			votes[district]++
		}
	}
	best, most := "", 0
	for district, count := range votes {
		if count > most || (count == most && district < best) {
			best, most = district, count
		}
	}
	return best, most > 0
}

// Build assigns every H3 cell to the district owning most of its door numbers (Owners),
// then merges each district's cells into polygons. Holes are dropped (they are areas
// without door numbers such as parks, not enclaves), as are single-cell islands of
// districts that also have larger areas.
func Build(points []addresspoint.Point, districts map[string]string) ([]domain.DistrictBoundary, error) {
	owners, err := Owners(points, districts)
	if err != nil {
		return nil, err
	}
	cellsByDistrict := map[string][]h3.Cell{}
	for cell, district := range owners {
		cellsByDistrict[district] = append(cellsByDistrict[district], cell)
	}

	boundaries := make([]domain.DistrictBoundary, 0, len(cellsByDistrict))
	for district, cells := range cellsByDistrict {
		polygons, err := h3.CellsToMultiPolygon(cells)
		if err != nil {
			return nil, fmt.Errorf("merge cells of %s: %w", district, err)
		}
		boundaries = append(boundaries, domain.DistrictBoundary{
			District:    district,
			Cells:       len(cells),
			Coordinates: outerRings(polygons),
		})
	}
	slices.SortFunc(boundaries, func(a, b domain.DistrictBoundary) int {
		return cmp.Compare(districts[a.District], districts[b.District])
	})
	return boundaries, nil
}

// singleCellVertices is the vertex count of one hexagon's ring.
const singleCellVertices = 6

func outerRings(polygons []h3.GeoPolygon) [][][][]float64 {
	keepIslands := len(polygons) == 1
	var coordinates [][][][]float64
	for _, polygon := range polygons {
		if !keepIslands && len(polygon.GeoLoop) <= singleCellVertices {
			continue
		}
		ring := make([][]float64, 0, len(polygon.GeoLoop)+1)
		for _, vertex := range polygon.GeoLoop {
			ring = append(ring, []float64{round(vertex.Lng), round(vertex.Lat)})
		}
		ring = append(ring, ring[0]) // GeoJSON rings are closed
		coordinates = append(coordinates, [][][]float64{ring})
	}
	return coordinates
}

// round keeps about 10 cm of precision to shrink the stored JSON.
func round(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}
