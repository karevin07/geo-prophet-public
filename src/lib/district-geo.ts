import type { DistrictBoundaries } from "./api";

type Feature = DistrictBoundaries["features"][number];

/** The largest ring's vertex average: a label point inside compact district shapes. */
export function labelPoint(feature: Feature): [number, number] {
  const ring = feature.geometry.coordinates.reduce((largest, polygon) => polygon[0].length > largest.length ? polygon[0] : largest, [] as number[][]);
  const points = ring.slice(0, -1);
  return [points.reduce((sum, point) => sum + point[0], 0) / points.length, points.reduce((sum, point) => sum + point[1], 0) / points.length];
}
