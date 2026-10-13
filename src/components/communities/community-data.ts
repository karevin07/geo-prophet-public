import type { CommunityDetail, CommunityYear, TransactionData } from "../../lib/api";
import { floorNumber } from "../../lib/floors";

/** A market sale: house and land, not flagged as special, as the API counts them. */
export function isMarketSale(sale: TransactionData): boolean {
  return sale.transactionType.startsWith("房地") && !(sale.specialReasons?.length);
}

export interface YearRow {
  year: number;
  sales: number;
  /** Settled values in 萬, and the provisional tail drawn dashed from the last settled year. */
  community: number | null;
  communityTail: number | null;
  district: number | null;
  districtTail: number | null;
}

/** Splits the years at the first provisional one, so its line continues dashed. */
export function yearRows(years: CommunityYear[]): YearRow[] {
  const firstProvisional = years.findIndex((year) => year.provisional);
  return years.map((year, index) => {
    const settled = firstProvisional < 0 || index < firstProvisional;
    const tail = firstProvisional >= 0 && index >= firstProvisional - 1;
    const community = year.medianPricePerPing / 10_000;
    const district = year.districtMedianPricePerPing === null ? null : year.districtMedianPricePerPing / 10_000;
    return {
      year: year.year, sales: year.sales,
      community: settled ? community : null, communityTail: tail ? community : null,
      district: settled ? district : null, districtTail: tail ? district : null,
    };
  });
}

export type FloorBandKey = "high" | "mid" | "low" | "ground";

export interface FloorBand {
  key: FloorBandKey;
  /** Floors covered, inclusive. */
  from: number;
  to: number;
  sales: number;
  median: number;
  low: number;
  high: number;
}

function quantile(sorted: number[], q: number): number {
  const position = q * (sorted.length - 1);
  const lower = Math.floor(position);
  return lower + 1 >= sorted.length ? sorted[lower] : sorted[lower] + (position - lower) * (sorted[lower + 1] - sorted[lower]);
}

/**
 * Groups market sales from `from` (YYYY-MM-DD) by floor: the ground floor alone, then the
 * floors above it split into thirds of the building's height, highest first. Each band has
 * its median and middle half of price per ping; bands without sales are left out.
 */
export function floorBands(sales: TransactionData[], totalFloors: number | null, from: string): FloorBand[] {
  const located = sales.flatMap((sale) => {
    const floor = floorNumber(sale.floorLabel);
    return floor !== null && isMarketSale(sale) && sale.transactedAt.slice(0, 10) >= from ? [{ floor, price: sale.pricePerPing }] : [];
  });
  const top = Math.max(totalFloors ?? 0, ...located.map((item) => item.floor), 1);
  const upper = top - 1;
  const lowTo = 1 + Math.ceil(upper / 3);
  const midTo = 1 + Math.ceil((upper * 2) / 3);
  const ranges: [FloorBandKey, number, number][] = [["high", midTo + 1, top], ["mid", lowTo + 1, midTo], ["low", 2, lowTo], ["ground", 1, 1]];
  return ranges.flatMap(([key, low, high]) => {
    const prices = located.filter((item) => item.floor >= low && item.floor <= high).map((item) => item.price).sort((a, b) => a - b);
    if (prices.length === 0 || low > high) return [];
    return [{ key, from: low, to: high, sales: prices.length, median: quantile(prices, 0.5), low: quantile(prices, 0.25), high: quantile(prices, 0.75) }];
  });
}

/** The address up to its door number, e.g. 台北市中山區敬業三路162巷100號 for a sale on its third floor. */
export function doorAddress(address: string): string {
  return address.match(/^(.*?\d+(?:之\d+)?號)/)?.[1] ?? address;
}

/** Whole years from completion to today, or null for a future date. */
export function buildingAge(completedOn: string, today = new Date()): number | null {
  const years = (today.getTime() - Date.parse(completedOn)) / (365.25 * 86_400_000);
  return years >= 0 ? Math.floor(years) : null;
}

/** The first day of the month five years before the month after `through` (YYYY-MM). */
export function fiveYearsBefore(through: string): string {
  const [year, month] = through.split("-").map(Number);
  const date = new Date(Date.UTC(year - 5, month, 1));
  return date.toISOString().slice(0, 10);
}

/** The sales the table shows: optionally only the last three years, optionally without special sales. */
export function visibleSales(detail: CommunityDetail, recentOnly: boolean, excludeSpecial: boolean): TransactionData[] {
  const [year, month] = detail.through.split("-").map(Number);
  const since = new Date(Date.UTC(year - 3, month, 1)).toISOString().slice(0, 10);
  return detail.sales.filter((sale) => (!recentOnly || sale.transactedAt.slice(0, 10) >= since)
    && (!excludeSpecial || !(sale.specialReasons?.length)));
}
