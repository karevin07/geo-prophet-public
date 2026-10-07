import type { RentPoint } from "./api";
import type { PriceBreaks } from "./price-scale";

/** Quartile breaks of gross yields, rounded to 0.1 percentage point, for priceColor. */
export function yieldBreaks(values: (number | null)[]): PriceBreaks | null {
  const sorted = values.filter((value): value is number => value !== null && Number.isFinite(value)).sort((a, b) => a - b);
  if (sorted.length === 0) return null;
  const quantile = (fraction: number) => {
    const position = fraction * (sorted.length - 1);
    const lower = Math.floor(position);
    const upper = Math.min(lower + 1, sorted.length - 1);
    return sorted[lower] + (position - lower) * (sorted[upper] - sorted[lower]);
  };
  const [low, mid, high] = [0.25, 0.5, 0.75].map((fraction) => Math.round(quantile(fraction) * 1000) / 1000);
  return [low, Math.max(mid, low), Math.max(high, mid, low)];
}

/** 0.02347 → "2.35%"; null → "—". */
export function formatYield(value: number | null, digits = 2): string {
  return value === null ? "—" : `${(value * 100).toFixed(digits)}%`;
}

/** Rent in TWD with thousands separators; null → "—". */
export function formatRent(value: number | null): string {
  return value === null ? "—" : Math.round(value).toLocaleString("zh-TW");
}

export interface RentRow {
  month: string;
  /** Settled values, and the provisional tail drawn dashed from the last settled month. */
  city: number | null;
  cityTail: number | null;
  district: number | null;
  districtTail: number | null;
  cityCount: number;
  districtCount: number;
}

/** Joins a city's and a district's rent trends by month, splitting each at its first provisional month. */
export function rentRows(city: RentPoint[], district: RentPoint[] = []): RentRow[] {
  const split = (points: RentPoint[]) => {
    const first = points.findIndex((point) => point.provisional);
    return new Map(points.map((point, index) => [point.month, {
      settled: first < 0 || index < first ? point.medianRentPerPing : null,
      tail: first >= 0 && index >= first - 1 ? point.medianRentPerPing : null,
      count: point.rentals,
    }]));
  };
  const cityByMonth = split(city);
  const districtByMonth = split(district);
  const months = [...new Set([...cityByMonth.keys(), ...districtByMonth.keys()])].sort();
  return months.map((month) => {
    const cityPoint = cityByMonth.get(month);
    const districtPoint = districtByMonth.get(month);
    return {
      month,
      city: cityPoint?.settled ?? null, cityTail: cityPoint?.tail ?? null, cityCount: cityPoint?.count ?? 0,
      district: districtPoint?.settled ?? null, districtTail: districtPoint?.tail ?? null, districtCount: districtPoint?.count ?? 0,
    };
  });
}
