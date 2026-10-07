import type { BBox } from "./api";

const round = (value: number) => Math.round(value * 1000) / 1000;

/** Rounds a bounding box to ~100m so small pans reuse the same query. */
export function roundBBox([west, south, east, north]: BBox): BBox {
  return [round(west), round(Math.max(south, -85)), round(east), round(Math.min(north, 85))];
}

export function sameBBox(left: BBox | null, right: BBox | null): boolean {
  return left === right || (left !== null && right !== null && left.every((value, index) => value === right[index]));
}
