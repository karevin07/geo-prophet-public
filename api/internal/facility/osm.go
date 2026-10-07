// Package facility reads facilities (shops, schools, clinics, parks and nuisances) from
// OpenStreetMap and government lists.
package facility

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
)

// osmCategories maps OpenStreetMap tags to categories. Schools, hospitals and clinics come
// from the education ministry and health insurance lists instead, which are complete.
var osmCategories = []struct{ key, value, category string }{
	{"shop", "convenience", "convenience"},
	{"shop", "supermarket", "supermarket"},
	{"amenity", "marketplace", "market"},
	{"leisure", "park", "park"},
	{"amenity", "library", "library"},
	{"amenity", "fuel", "gas_station"},
	{"power", "substation", "substation"},
	{"amenity", "funeral_hall", "funeral"},
	{"amenity", "crematorium", "funeral"},
	{"landuse", "cemetery", "cemetery"},
	{"amenity", "grave_yard", "cemetery"},
	{"amenity", "waste_transfer_station", "waste"},
	{"landuse", "landfill", "waste"},
}

func osmCategory(tags osm.Tags) string {
	for _, rule := range osmCategories {
		if tags.Find(rule.key) == rule.value {
			return rule.category
		}
	}
	return ""
}

// Box bounds the area to keep, in degrees.
type Box struct{ South, West, North, East float64 }

func (box Box) contains(lat, lng float64) bool {
	return lat >= box.South && lat <= box.North && lng >= box.West && lng <= box.East
}

// OSMReader reopens the same OpenStreetMap PBF file for each pass.
type OSMReader func() (io.ReadCloser, error)

// ReadOSM returns the facilities inside box from an OpenStreetMap PBF extract. Tagged
// nodes become points; tagged closed ways become areas located at their centre. Relations
// (multipolygons) are skipped. The file is read twice: first for the ways and the IDs of
// their nodes, then for the nodes, so only the coordinates needed are kept in memory.
func ReadOSM(ctx context.Context, open OSMReader, box Box) ([]domain.Facility, error) {
	type pendingWay struct {
		id       osm.WayID
		category string
		tags     osm.Tags
		nodes    []osm.NodeID
	}
	var ways []pendingWay
	needed := map[osm.NodeID][2]float64{}
	err := scan(ctx, open, func(scanner *osmpbf.Scanner) { scanner.SkipNodes, scanner.SkipRelations = true, true }, func(object osm.Object) {
		way, ok := object.(*osm.Way)
		if !ok {
			return
		}
		category := osmCategory(way.Tags)
		if category == "" || len(way.Nodes) < 2 {
			return
		}
		pending := pendingWay{id: way.ID, category: category, tags: way.Tags, nodes: way.Nodes.NodeIDs()}
		for _, id := range pending.nodes {
			needed[id] = [2]float64{}
		}
		ways = append(ways, pending)
	})
	if err != nil {
		return nil, fmt.Errorf("read ways: %w", err)
	}

	var facilities []domain.Facility
	err = scan(ctx, open, func(scanner *osmpbf.Scanner) { scanner.SkipWays, scanner.SkipRelations = true, true }, func(object osm.Object) {
		node, ok := object.(*osm.Node)
		if !ok {
			return
		}
		if _, ok := needed[node.ID]; ok {
			needed[node.ID] = [2]float64{node.Lon, node.Lat}
		}
		if category := osmCategory(node.Tags); category != "" && box.contains(node.Lat, node.Lon) {
			facilities = append(facilities, osmFacility("node/"+strconv.FormatInt(int64(node.ID), 10), category, node.Tags, node.Lat, node.Lon))
		}
	})
	if err != nil {
		return nil, fmt.Errorf("read nodes: %w", err)
	}

	for _, way := range ways {
		ring := make([][2]float64, 0, len(way.nodes))
		for _, id := range way.nodes {
			if point := needed[id]; point != [2]float64{} {
				ring = append(ring, point)
			}
		}
		if len(ring) < 2 {
			continue
		}
		lat, lng := centre(ring)
		if !box.contains(lat, lng) {
			continue
		}
		facility := osmFacility("way/"+strconv.FormatInt(int64(way.id), 10), way.category, way.tags, lat, lng)
		if len(ring) >= 4 && ring[0] == ring[len(ring)-1] {
			facility.Area, facility.AreaM2 = ring, math.Round(ringArea(ring))
		}
		facilities = append(facilities, facility)
	}
	return facilities, nil
}

func osmFacility(id, category string, tags osm.Tags, lat, lng float64) domain.Facility {
	name := tags.Find("name")
	if name == "" {
		name = tags.Find("name:zh")
	}
	return domain.Facility{
		ID: "osm:" + id, Source: "osm", Category: category, Name: name, Brand: tags.Find("brand"),
		Lat: math.Round(lat*1e7) / 1e7, Lng: math.Round(lng*1e7) / 1e7,
	}
}

func scan(ctx context.Context, open OSMReader, configure func(*osmpbf.Scanner), visit func(osm.Object)) error {
	file, err := open()
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := osmpbf.New(ctx, file, 4)
	defer scanner.Close()
	configure(scanner)
	for scanner.Scan() {
		visit(scanner.Object())
	}
	return scanner.Err()
}

// centre averages a ring's distinct vertices.
func centre(ring [][2]float64) (float64, float64) {
	points := ring
	if len(ring) > 1 && ring[0] == ring[len(ring)-1] {
		points = ring[:len(ring)-1]
	}
	var lat, lng float64
	for _, point := range points {
		lng += point[0]
		lat += point[1]
	}
	return lat / float64(len(points)), lng / float64(len(points))
}

// ringArea is the area of a small [lng, lat] ring in square metres (shoelace formula on
// an equirectangular projection, accurate enough for parks).
func ringArea(ring [][2]float64) float64 {
	lat, _ := centre(ring)
	const metresPerDegree = 111_320.0
	scaleX := metresPerDegree * math.Cos(lat*math.Pi/180)
	sum := 0.0
	for index := 0; index < len(ring)-1; index++ {
		x1, y1 := ring[index][0]*scaleX, ring[index][1]*metresPerDegree
		x2, y2 := ring[index+1][0]*scaleX, ring[index+1][1]*metresPerDegree
		sum += x1*y2 - x2*y1
	}
	return math.Abs(sum) / 2
}
