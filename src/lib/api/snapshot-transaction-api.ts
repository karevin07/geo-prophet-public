import { HTTPTransactionAPI } from "./http-transaction-api";
import { LocalSales, type LocalSale } from "./local-sales";
import { LocalTransactionAPI } from "./local-transaction-api";
import { snapshotKey } from "./snapshot-key";
import { readJSON, readSnapshotMeta } from "./snapshot-meta";
import type { BBox, CityFacilities, DistrictBoundaries, PresaleProjects, RentalQuery, RentalReport, RentTrend, TransactionData } from "./types";

/** The snapshot shape this build reads; api/internal/snapshot.Version. */
const SNAPSHOT_VERSION = 1;

/** Each sale field as an array, in the same order (api/internal/snapshot salesColumns). */
type SalesColumns = {
  [K in "id" | "district" | "address" | "road" | "transactionType" | "buildingType" | "mainUse" | "floorLabel" | "transactedAt" | "remarks" | "season" | "geocodePrecision"]: string[];
} & {
  [K in "totalPrice" | "parkingPrice" | "buildingPrice" | "buildingAreaPing" | "pricePerPing" | "rooms" | "halls" | "baths" | "lat" | "lng" | "isNew"]: number[];
} & { totalFloors: (number | null)[]; completedAt: (string | null)[]; specialReasons: string[][] };

/** Turns the stored columns back into sales shaped as the API serves them. */
export function snapshotSales(columns: SalesColumns, cityCode: string): LocalSale[] {
  return columns.id.map((id, index) => ({
    id, cityCode, district: columns.district[index], address: columns.address[index], road: columns.road[index],
    transactionType: columns.transactionType[index], buildingType: columns.buildingType[index], mainUse: columns.mainUse[index],
    totalPrice: columns.totalPrice[index], parkingPrice: columns.parkingPrice[index], buildingPrice: columns.buildingPrice[index],
    buildingAreaPing: columns.buildingAreaPing[index], pricePerPing: columns.pricePerPing[index],
    rooms: columns.rooms[index], halls: columns.halls[index], baths: columns.baths[index], floorLabel: columns.floorLabel[index],
    totalFloors: columns.totalFloors[index] ?? undefined,
    completedAt: columns.completedAt[index] ? `${columns.completedAt[index]}T00:00:00Z` : undefined,
    transactedAt: `${columns.transactedAt[index]}T00:00:00Z`,
    remarks: columns.remarks[index] || undefined,
    specialReasons: columns.specialReasons[index].length ? columns.specialReasons[index] : undefined,
    season: columns.season[index], lat: columns.lat[index], lng: columns.lng[index],
    geocodePrecision: columns.geocodePrecision[index] as TransactionData["geocodePrecision"],
    isNew: columns.isNew[index] === 1 || undefined,
  }));
}

/**
 * The online demo's data, read from a snapshot of one city (see docs/DEMO.md): sales are
 * filtered and aggregated in the browser, and reports with fixed parameters are replayed
 * from saved API responses.
 */
export class SnapshotTransactionAPI extends LocalTransactionAPI {
  private local?: Promise<LocalSales>;
  /** Builds the same requests as the API client, read from saved responses. */
  private readonly saved: HTTPTransactionAPI;

  constructor(private readonly base: string) {
    super();
    this.saved = new HTTPTransactionAPI("", async <T>(url: string, signal?: AbortSignal) =>
      readJSON<T>(`${base}api/${await snapshotKey(url)}.json`, signal));
  }

  protected sales(): Promise<LocalSales> {
    if (!this.local) {
      this.local = this.load();
      // A failed download is retried on the next request.
      this.local.catch(() => { this.local = undefined; });
    }
    return this.local;
  }

  private async load(): Promise<LocalSales> {
    const [meta, columns] = await Promise.all([readSnapshotMeta(this.base), readJSON<SalesColumns>(`${this.base}sales.json`)]);
    if (meta.version !== SNAPSHOT_VERSION) throw new Error(`snapshot version ${meta.version}, want ${SNAPSHOT_VERSION}`);
    return new LocalSales(snapshotSales(columns, meta.cityCode), {
      region: meta.region, completeMonth: meta.completeMonth || null, latestRelease: meta.latestRelease,
    });
  }

  async districtBoundaries(city: string, signal?: AbortSignal): Promise<DistrictBoundaries> {
    return this.saved.districtBoundaries(city, signal);
  }

  async rentalDistricts(query: RentalQuery, signal?: AbortSignal): Promise<RentalReport> {
    return this.saved.rentalDistricts(query, signal);
  }

  async rentalTrend(query: RentalQuery, district: string, signal?: AbortSignal): Promise<RentTrend> {
    return this.saved.rentalTrend(query, district, signal);
  }

  async districtFacilities(city: string, signal?: AbortSignal): Promise<CityFacilities> {
    return this.saved.districtFacilities(city, signal);
  }

  // A district's projects are saved; the map's projects in view are not.
  async presaleProjects(query: { city: string; district?: string; months: number; near?: { lat: number; lng: number; radius: number }; bbox?: BBox }, signal?: AbortSignal): Promise<PresaleProjects> {
    if (query.near || query.bbox || !query.district) return super.presaleProjects(query);
    return this.saved.presaleProjects(query, signal);
  }
}
