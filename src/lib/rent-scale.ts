import type { RentCell } from "./api";
import type { PriceBreaks } from "./price-scale";

/** What colours the map: sale prices, or the rent layer's rents or gross yields. */
export type MapMetric = "price" | "rent" | "yield";

/** The value a rent layer cell is coloured by, or null when it has none. */
export function rentValue(cell: RentCell, metric: Exclude<MapMetric, "price">): number | null {
  return metric === "rent" ? cell.medianRentPerPing : cell.grossYield;
}

/**
 * Splits values at their quartiles, like priceBreaks, rounding rents to 50 元 and yields
 * to 0.1 percentage points so the legend stays readable.
 */
export function rentBreaks(values: number[], metric: Exclude<MapMetric, "price">): PriceBreaks | null {
  const sorted = values.filter((value) => Number.isFinite(value) && value > 0).sort((a, b) => a - b);
  if (sorted.length === 0) return null;
  const step = metric === "rent" ? 50 : 0.001;
  const quantile = (fraction: number) => {
    const position = fraction * (sorted.length - 1);
    const lower = Math.floor(position);
    const upper = Math.min(lower + 1, sorted.length - 1);
    return sorted[lower] + (position - lower) * (sorted[upper] - sorted[lower]);
  };
  const rounded = [0.25, 0.5, 0.75].map((fraction) => Math.round(quantile(fraction) / step) * step);
  return [rounded[0], Math.max(rounded[1], rounded[0]), Math.max(rounded[2], rounded[1])];
}

/** A rent in 元 with thousands separators, or a yield in percent with one decimal. */
export function formatRentValue(value: number, metric: Exclude<MapMetric, "price">): string {
  return metric === "rent" ? Math.round(value).toLocaleString() : `${(value * 100).toFixed(1)}%`;
}
