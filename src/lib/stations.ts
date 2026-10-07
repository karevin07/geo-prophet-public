import type { DistanceBucket, OpeningEffect, StationRank, TransitStation, TransitSystem, TrendPoint } from "./api";
import { smoothTrend } from "./transit";

export interface StationFilter {
  /** 實價登錄 city code, or "" for every city. */
  city: string;
  /** Systems to include; empty includes all. */
  systems: TransitSystem[];
  /** A line ID such as TRTC-BL, or "" for every line. */
  lineId: string;
}

export function matchesStation(station: TransitStation, filter: StationFilter): boolean {
  return (!filter.city || station.cityCode === filter.city) &&
    (filter.systems.length === 0 || station.systems.some((system) => filter.systems.includes(system))) &&
    (!filter.lineId || station.lines.some((line) => line.lineId === filter.lineId));
}

export type RankSort = "median" | "yoy" | "count";

export interface RankRow {
  station: TransitStation;
  rank: StationRank;
}

/** Stations with sales in the window, joined to their figures and sorted descending. */
export function rankRows(stations: TransitStation[], ranks: StationRank[], filter: StationFilter, sort: RankSort): RankRow[] {
  const byId = new Map(ranks.map((rank) => [rank.stationId, rank]));
  const value = (rank: StationRank) => sort === "count" ? rank.count : sort === "yoy" ? rank.priceYoY ?? -Infinity : rank.medianPricePerPing;
  return stations
    .filter((station) => matchesStation(station, filter))
    .flatMap((station) => {
      const rank = byId.get(station.id);
      return rank && rank.count > 0 ? [{ station, rank }] : [];
    })
    .sort((a, b) => value(b.rank) - value(a.rank) || a.station.name.localeCompare(b.station.name, "zh-Hant"));
}

const STATUS_ORDER: Record<OpeningEffect["status"], number> = { ok: 0, too_recent: 1, insufficient: 2 };

/** Comparable openings first, largest relative change first; then too recent, then sparse. */
export function openingRows(effects: OpeningEffect[], stations: TransitStation[], filter: StationFilter): OpeningEffect[] {
  const byId = new Map(stations.map((station) => [station.id, station]));
  return effects
    .filter((effect) => {
      const station = byId.get(effect.stationId);
      return station ? matchesStation(station, filter) : !filter.city && !filter.lineId && filter.systems.length === 0;
    })
    .sort((a, b) => STATUS_ORDER[a.status] - STATUS_ORDER[b.status] ||
      (b.relative ?? 0) - (a.relative ?? 0) || b.openedOn.localeCompare(a.openedOn));
}

/** An SVG polyline for a trend in a width × height box; empty with fewer than two prices. */
export function sparkline(trend: TrendPoint[], width = 96, height = 24): string {
  const prices = trend.filter((point) => point.count > 0).map((point) => point.avgPricePerPing);
  if (prices.length < 2) return "";
  const low = Math.min(...prices);
  const span = Math.max(...prices) - low || 1;
  return prices.map((price, index) => {
    const x = (index / (prices.length - 1)) * width;
    const y = height - 2 - ((price - low) / span) * (height - 4);
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(" ");
}

export interface OpeningChartRow {
  month: string;
  offset: number;
  /** 萬 per ping, three-month moving averages weighted by sales. */
  station: number | null;
  baseline: number | null;
  stationCount: number;
  baselineCount: number;
}

/** Smooths the station and district series the same way as the station card's trend. */
export function openingChartRows(effect: OpeningEffect): OpeningChartRow[] {
  const toTrend = (price: (point: OpeningEffect["series"][number]) => number | null, count: (point: OpeningEffect["series"][number]) => number) =>
    effect.series.map((point) => ({ month: point.month, avgPricePerPing: price(point) ?? 0, count: price(point) == null ? 0 : count(point), provisional: false }));
  const station = smoothTrend(toTrend((point) => point.stationPrice, (point) => point.stationCount));
  const baseline = smoothTrend(toTrend((point) => point.baselinePrice, (point) => point.baselineCount));
  return effect.series.map((point, index) => ({
    month: point.month,
    offset: point.offset,
    station: station[index]?.price ?? null,
    baseline: baseline[index]?.price ?? null,
    stationCount: point.stationCount,
    baselineCount: point.baselineCount,
  }));
}

export interface DistanceRow {
  /** 0–100, …, 700–800, 800+ */
  label: string;
  fromM: number;
  /** 萬 per ping by city code, and sales by city code. */
  median: Record<string, number | null>;
  count: Record<string, number>;
}

/** Pivots distance buckets into one chart row per distance, with a value per city. */
export function distanceRows(buckets: DistanceBucket[], cities: string[]): DistanceRow[] {
  const rows = new Map<number, DistanceRow>();
  for (const bucket of buckets) {
    if (!cities.includes(bucket.cityCode)) continue;
    const row = rows.get(bucket.fromM) ?? {
      label: bucket.toM == null ? `${bucket.fromM}+` : `${bucket.fromM}–${bucket.toM}`,
      fromM: bucket.fromM, median: {}, count: {},
    };
    row.median[bucket.cityCode] = Math.round(bucket.medianPricePerPing / 1000) / 10;
    row.count[bucket.cityCode] = bucket.count;
    rows.set(bucket.fromM, row);
  }
  return [...rows.values()].sort((a, b) => a.fromM - b.fromM);
}

/** Fewest sales in each compared bucket for a near-station premium. */
export const MIN_PREMIUM_SALES = 30;

/**
 * How much higher the median price per ping is within 100 m of a station than 700–800 m
 * away, or null when either bucket has too few sales.
 */
export function nearStationPremium(buckets: DistanceBucket[], cityCode: string): number | null {
  const near = buckets.find((bucket) => bucket.cityCode === cityCode && bucket.fromM === 0);
  const far = buckets.find((bucket) => bucket.cityCode === cityCode && bucket.fromM === 700);
  if (!near || !far || near.count < MIN_PREMIUM_SALES || far.count < MIN_PREMIUM_SALES || far.medianPricePerPing <= 0) return null;
  return near.medianPricePerPing / far.medianPricePerPing - 1;
}
