import type { StationLine, TransitStation, TrendPoint } from "./api";
import { calendarMonths } from "./utils";

type RGBA = [number, number, number, number];

/** Changes smaller than this (±1%) count as flat. */
const FLAT = 0.01;

/** Station ring colours follow YoYBadge: red rising, green falling, grey flat or unknown. */
export function yoyColor(value: number | null | undefined): RGBA {
  if (value == null || Math.abs(value) < FLAT) return [148, 163, 184, 255];
  return value > 0 ? [220, 38, 38, 255] : [22, 163, 74, 255];
}

export interface Opening {
  lineId: string;
  name: string;
  color: string;
  /** YYYY-MM */
  month: string;
}

/** Lines at the station that opened since 2015, oldest first, one entry per line. */
export function stationOpenings(station: Pick<TransitStation, "lines">): Opening[] {
  const openings = new Map<string, Opening>();
  for (const line of station.lines) {
    if (!line.openedOn) continue;
    const month = line.openedOn.slice(0, 7);
    const known = openings.get(line.lineId);
    if (!known || month < known.month) openings.set(line.lineId, { lineId: line.lineId, name: line.name, color: line.color, month });
  }
  return [...openings.values()].sort((a, b) => a.month.localeCompare(b.month));
}

/** Lines serving the station, once each, in the API's order (MRT first). */
export function uniqueLines(lines: StationLine[]): StationLine[] {
  const seen = new Set<string>();
  return lines.filter((line) => !seen.has(line.lineId) && seen.add(line.lineId));
}

export interface SmoothedPoint {
  month: string;
  /** 萬 per ping, averaged over the trailing three months weighted by sales. */
  price: number | null;
  count: number;
  provisional: boolean;
}

/** Fewest sales in a smoothing window for a price to be shown. */
const MIN_WINDOW_SALES = 3;

/**
 * Fills every calendar month between the first and last sale and smooths prices over a
 * trailing three-month window, since a few sales a month near a station are noisy. Windows
 * with fewer than three sales have no price.
 */
export function smoothTrend(points: TrendPoint[], window = 3): SmoothedPoint[] {
  if (points.length === 0) return [];
  const byMonth = new Map(points.map((point) => [point.month, point]));
  const months = calendarMonths(points[0].month, points[points.length - 1].month);
  return months.map((month, index) => {
    let weighted = 0;
    let count = 0;
    for (const recent of months.slice(Math.max(0, index - window + 1), index + 1)) {
      const point = byMonth.get(recent);
      if (point) {
        weighted += point.avgPricePerPing * point.count;
        count += point.count;
      }
    }
    return {
      month,
      price: count >= MIN_WINDOW_SALES ? Math.round(weighted / count / 1000) / 10 : null,
      count: byMonth.get(month)?.count ?? 0,
      provisional: byMonth.get(month)?.provisional ?? false,
    };
  });
}
