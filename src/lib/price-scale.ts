/** Four price classes, low to high, shared by the map and its legend. */
export const PRICE_CLASSES = [
  { key: "low", color: [47, 75, 124] as const, hex: "#2F4B7C" },
  { key: "midLow", color: [79, 163, 165] as const, hex: "#4FA3A5" },
  { key: "mid", color: [242, 200, 91] as const, hex: "#F2C85B" },
  { key: "high", color: [232, 112, 79] as const, hex: "#E8704F" },
];

/** Upper bounds of the lower three classes, in TWD per ping. */
export type PriceBreaks = [number, number, number];

const WAN = 10_000;

/**
 * Splits prices at their quartiles so the four classes stay balanced for whatever area the
 * map shows. Breaks are rounded to whole 萬 to keep the legend readable.
 */
export function priceBreaks(values: number[]): PriceBreaks | null {
  const sorted = values.filter((value) => Number.isFinite(value) && value > 0).sort((a, b) => a - b);
  if (sorted.length === 0) return null;
  const quantile = (fraction: number) => {
    const position = fraction * (sorted.length - 1);
    const lower = Math.floor(position);
    const upper = Math.min(lower + 1, sorted.length - 1);
    return sorted[lower] + (position - lower) * (sorted[upper] - sorted[lower]);
  };
  const rounded = [0.25, 0.5, 0.75].map((fraction) => Math.round(quantile(fraction) / WAN) * WAN);
  return [rounded[0], Math.max(rounded[1], rounded[0]), Math.max(rounded[2], rounded[1])];
}

export function priceClass(value: number, breaks: PriceBreaks | null): number {
  if (!breaks) return 1;
  if (value < breaks[0]) return 0;
  if (value < breaks[1]) return 1;
  if (value < breaks[2]) return 2;
  return 3;
}

export function priceColor(value: number, breaks: PriceBreaks | null, alpha: number): [number, number, number, number] {
  const [r, g, b] = PRICE_CLASSES[priceClass(value, breaks)].color;
  return [r, g, b, alpha];
}

/** Formats TWD in 萬 with up to one decimal, e.g. 725000 → "72.5". */
export function formatWan(value: number, maximumFractionDigits = 1): string {
  return new Intl.NumberFormat("zh-TW", { maximumFractionDigits }).format(value / WAN);
}

/** Months (YYYY-MM) of the 12-month window ending with the current month, oldest first. */
export function recentMonths(now = new Date(), count = 12): string[] {
  return Array.from({ length: count }, (_, index) => {
    const date = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - (count - 1 - index), 1));
    return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}`;
  });
}
