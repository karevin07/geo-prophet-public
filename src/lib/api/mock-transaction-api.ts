import { cellsToMultiPolygon, latLngToCell } from "h3-js";
import { generateMockTransactions } from "../mock-data";
import { LocalSales } from "./local-sales";
import { LocalTransactionAPI } from "./local-transaction-api";
import { buildingCategory } from "./types";
import type { DistrictBoundaries, InvestmentQuery, InvestmentReport, TransactionData } from "./types";

function mulberry32(seed: number): () => number {
  return () => {
    seed |= 0;
    seed = seed + 0x6d2b79f5 | 0;
    let value = Math.imul(seed ^ seed >>> 15, 1 | seed);
    value = value + Math.imul(value ^ value >>> 7, 61 | value) ^ value;
    return ((value ^ value >>> 14) >>> 0) / 4294967296;
  };
}

function median(values: number[]) {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
}

/** Generated Taipei sales for development without the API (VITE_DATA_SOURCE=mock). */
export class MockTransactionAPI extends LocalTransactionAPI {
  private readonly transactions: TransactionData[] = generateMockTransactions(3000, mulberry32(591));
  private readonly local: LocalSales;

  constructor(now = new Date()) {
    super();
    // Treat the last three months as still being registered, as the API would.
    const complete = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 3, 1)).toISOString().slice(0, 7);
    this.local = new LocalSales(
      this.transactions.map((sale) => ({ ...sale, road: sale.address.slice(`台北市${sale.district}`.length).replace(/\d+號$/, "") })),
      {
        region: { code: "a", name: "台北市", districts: [...new Set(this.transactions.map((sale) => sale.district))] },
        completeMonth: complete, latestRelease: null, now: () => now,
      },
    );
  }

  protected async sales() {
    return this.local;
  }

  // Like the API, derive district areas from where sales are, merging resolution-8 cells.
  async districtBoundaries(_city: string): Promise<DistrictBoundaries> {
    const cells = new Map<string, Set<string>>();
    for (const transaction of this.transactions) {
      const set = cells.get(transaction.district) ?? new Set<string>();
      set.add(latLngToCell(transaction.lat, transaction.lng, 8));
      cells.set(transaction.district, set);
    }
    return {
      type: "FeatureCollection",
      features: Array.from(cells, ([district, set]) => ({
        type: "Feature" as const,
        properties: { district, cells: set.size },
        geometry: { type: "MultiPolygon" as const, coordinates: cellsToMultiPolygon([...set], true).map((polygon) => [polygon[0]]) },
      })),
    };
  }

  async investmentAreas(query: InvestmentQuery): Promise<InvestmentReport> {
    const month = new Date();
    month.setUTCDate(1);
    month.setUTCMonth(month.getUTCMonth() - 4);
    const label = (date: Date) => date.toISOString().slice(0, 7);
    const shift = (date: Date, months: number) => new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + months, 1));
    const through = label(month), currentFrom = label(shift(month, -11)), previousFrom = label(shift(month, -23));
    const groups = new Map<string, { current: number[]; previous: number[] }>();
    groups.set("", { current: [], previous: [] });
    for (const sale of this.transactions) {
      const soldMonth = sale.transactedAt.slice(0, 7);
      if (sale.cityCode !== query.city || buildingCategory(sale.buildingType) !== query.type ||
        sale.mainUse !== "住家用" || !sale.transactionType.startsWith("房地") ||
        sale.specialReasons?.length || sale.buildingAreaPing < query.minArea || sale.buildingAreaPing >= query.maxArea ||
        !sale.completedAt || soldMonth < previousFrom || soldMonth > through) continue;
      const age = Math.floor((Date.parse(sale.transactedAt) - Date.parse(sale.completedAt)) / 86_400_000 / 365.25);
      if (age < query.minAge || age >= query.maxAge) continue;
      if (!groups.has(sale.district)) groups.set(sale.district, { current: [], previous: [] });
      for (const key of ["", sale.district]) groups.get(key)![soldMonth >= currentFrom ? "current" : "previous"].push(sale.pricePerPing);
    }
    const city = groups.get("")!;
    const cityMedian = city.current.length >= 30 ? median(city.current) : null;
    const areas = Array.from(groups).filter(([district]) => district).map(([district, group]) => {
      const currentCount = group.current.length, previousCount = group.previous.length;
      const sufficientCurrentSample = currentCount >= 30;
      const sufficientComparisonSample = sufficientCurrentSample && previousCount >= 30;
      const medianPricePerPing = sufficientCurrentSample ? median(group.current) : null;
      return {
        district, currentCount, previousCount, medianPricePerPing,
        volumeChange: sufficientComparisonSample ? currentCount / previousCount - 1 : null,
        priceChange: sufficientComparisonSample ? median(group.current) / median(group.previous) - 1 : null,
        relativeToCityMedian: medianPricePerPing !== null && cityMedian !== null ? medianPricePerPing / cityMedian - 1 : null,
        sufficientCurrentSample, sufficientComparisonSample,
      };
    }).sort((a, b) => b.currentCount - a.currentCount || a.district.localeCompare(b.district));
    const cityComparable = city.current.length >= 30 && city.previous.length >= 30;
    return { cityCode: query.city, through, currentFrom, previousFrom, minimumSample: 30,
      cityCurrentCount: city.current.length, cityPreviousCount: city.previous.length, cityMedianPricePerPing: cityMedian,
      cityVolumeChange: cityComparable ? city.current.length / city.previous.length - 1 : null,
      cityPriceChange: cityComparable ? median(city.current) / median(city.previous) - 1 : null, areas };
  }
}
