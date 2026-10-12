import { latLngToCell } from "h3-js";
import { describe, expect, it } from "vitest";
import { LocalSales, type LocalSale } from "./local-sales";
import type { TransactionQuery } from "./types";

let next = 0;
function sale(overrides: Partial<LocalSale>): LocalSale {
  next++;
  return {
    id: `S${String(next).padStart(4, "0")}`, cityCode: "a", district: "大安區", address: "台北市大安區敦化南路一段1號", road: "敦化南路一段",
    transactionType: "房地(土地+建物)", buildingType: "住宅大樓(11層含以上有電梯)", mainUse: "住家用",
    totalPrice: 20_000_000, parkingPrice: 0, buildingPrice: 20_000_000, buildingAreaPing: 20, pricePerPing: 1_000_000,
    rooms: 2, halls: 1, baths: 1, floorLabel: "五層", completedAt: "2000-01-01T00:00:00Z", transactedAt: "2026-09-01T00:00:00Z",
    season: "115S3", lat: 25.04, lng: 121.55, geocodePrecision: "address", ...overrides,
  };
}

const region = { code: "a", name: "台北市", districts: ["松山區", "信義區", "大安區"] };
const now = () => new Date("2026-10-12T08:00:00Z");
const query: TransactionQuery = { priceRange: [0, 1e10], areaRange: [0, 1000], types: [] };
const local = (sales: LocalSale[], completeMonth: string | null = "2026-06") =>
  new LocalSales(sales, { region, completeMonth, latestRelease: null, now });

describe("LocalSales", () => {
  it("filters as the API does, by default over the 12 months ending this month", async () => {
    const sales = [
      sale({ transactedAt: "2025-11-01T00:00:00Z" }),
      sale({ transactedAt: "2025-10-31T00:00:00Z" }), // before the window
      sale({ specialReasons: ["親友交易"] }),
      sale({ buildingType: "公寓(5樓含以下無電梯)" }),
      sale({ completedAt: "2025-06-01T00:00:00Z" }), // a year old
      sale({ geocodePrecision: "district" }),
      sale({ totalPrice: 90_000_000 }),
    ];
    const engine = local(sales);
    expect((await engine.summary(query)).count).toBe(5);
    expect((await engine.summary({ ...query, includeSpecial: true })).count).toBe(6);
    expect((await engine.summary({ ...query, types: ["公寓"] })).count).toBe(1);
    expect((await engine.summary({ ...query, excludeNewBuildings: true })).count).toBe(4);
    expect((await engine.summary({ ...query, priceRange: [0, 50_000_000] })).count).toBe(4);
    // The map leaves out sales located only to their district.
    expect((await engine.list(query)).map((row) => row.id)).not.toContain(sales[5].id);
    expect((await engine.aggregateH3(query, 9)).reduce((sum, cell) => sum + cell.count, 0)).toBe(4);
  });

  it("compares a summary with the same months a year earlier, ending at the last complete month", async () => {
    const sales = [
      ...Array.from({ length: 5 }, () => sale({ transactedAt: "2026-05-10T00:00:00Z", buildingPrice: 22_000_000, totalPrice: 22_000_000 })),
      ...Array.from({ length: 4 }, () => sale({ transactedAt: "2025-05-10T00:00:00Z" })),
      sale({ transactedAt: "2025-06-30T00:00:00Z" }),
      sale({ transactedAt: "2026-09-01T00:00:00Z" }), // after the last complete month
    ];
    const summary = await local(sales).summary(query);
    expect(summary.count).toBe(6);
    expect(summary.yoy).toMatchObject({ basis: "12m", through: "2026-06", count: 0 });
    expect(summary.yoy?.avgPricePerPing).toBeCloseTo(0.1);
    // One month compares three; March to May 2025 has four sales, fewer than the five needed.
    const month = await local(sales).summary({ ...query, month: "2026-05" });
    expect(month.yoy).toMatchObject({ basis: "3m", through: "2026-05", count: null });
    expect((await local(sales.slice(1)).summary(query)).yoy?.count).toBeNull();
  });

  it("marks months after the last complete one as provisional", async () => {
    const trends = await local([sale({ transactedAt: "2026-06-30T00:00:00Z" }), sale({ transactedAt: "2026-07-01T00:00:00Z" })]).trends(query);
    expect(trends.map((point) => [point.month, point.provisional])).toEqual([["2026-06", false], ["2026-07", true]]);
    expect((await local([sale({})], null).trends(query))[0].provisional).toBe(false);
  });

  it("ranks districts by median price with YoY over the last 12 complete months", async () => {
    const sales = [
      ...Array.from({ length: 5 }, () => sale({ district: "信義區", transactedAt: "2026-01-05T00:00:00Z", buildingPrice: 30_000_000, pricePerPing: 1_500_000 })),
      ...Array.from({ length: 5 }, () => sale({ district: "信義區", transactedAt: "2025-01-05T00:00:00Z" })),
      sale({ transactedAt: "2026-01-05T00:00:00Z", pricePerPing: 900_000 }),
    ];
    const comparison = await local(sales).compareDistricts({ ...query, city: "a", districts: ["信義區"], from: "2025-01", to: "2026-09" });
    expect(comparison.yoyThrough).toBe("2026-06");
    expect(comparison.ranking.map((rank) => [rank.district, rank.count, rank.priceYoY])).toEqual([["信義區", 10, 0.5], ["大安區", 1, null]]);
    expect(comparison.trends.map((point) => [point.month, point.count])).toEqual([["2025-01", 5], ["2026-01", 5]]);
    expect((await local(sales).compareDistricts({ ...query, city: "f", districts: [], from: "2025-01", to: "2026-09" })).ranking).toEqual([]);
    await expect(local(sales).compareDistricts({ ...query, city: "a", districts: ["a", "b", "c", "d", "e"], from: "2025-01", to: "2026-09" })).rejects.toThrow();
  });

  it("describes a hexagon with its main road, age mix and YoY", async () => {
    const sales = [
      ...Array.from({ length: 5 }, () => sale({ transactedAt: "2026-03-01T00:00:00Z", buildingPrice: 24_000_000 })),
      ...Array.from({ length: 5 }, () => sale({ transactedAt: "2025-03-01T00:00:00Z", completedAt: undefined })),
      sale({ road: "仁愛路四段", lat: 25.0401, lng: 121.5501 }),
    ];
    const hex = latLngToCell(25.04, 121.55, 9);
    const detail = await local(sales).cellDetail(query, hex);
    expect(detail).toMatchObject({ count: 6, district: "大安區", cityCode: "a", road: "敦化南路一段", yoyThrough: "2026-06" });
    expect(detail.priceYoY).toBeCloseTo(0.2);
    expect(detail.ageMix.find((bucket) => bucket.key === "15-30")?.count).toBe(6);
    expect(await local([]).cellDetail(query, hex)).toMatchObject({ count: 0, road: "", ageMix: [], priceYoY: null });
  });

  it("pages the feed in order with totals on the first page", async () => {
    const sales = Array.from({ length: 60 }, (_, index) =>
      sale({ transactedAt: `2026-0${1 + (index % 9)}-01T00:00:00Z`, pricePerPing: 500_000 + index, isNew: index < 3 || undefined }));
    const engine = local(sales);
    const first = await engine.feed({ ...query, sort: "price_per_ping_desc" });
    expect(first.meta).toMatchObject({ total: 60, newCount: 3 });
    expect(first.data[0].pricePerPing).toBe(500_059);
    const second = await engine.feed({ ...query, sort: "price_per_ping_desc" }, first.meta.nextCursor);
    expect(second.data.map((row) => row.pricePerPing)).toEqual(Array.from({ length: 10 }, (_, index) => 500_009 - index));
    expect(second.meta.total).toBeUndefined();
    expect(second.meta.nextCursor).toBeUndefined();
    expect((await engine.feed({ ...query, sort: "date_desc", latestOnly: true })).meta.total).toBe(3);
  });

  it("finds districts and roads at the centre of their sales", async () => {
    const engine = local([sale({}), sale({ lat: 25.05, lng: 121.56 }), sale({ district: "松山區", road: "敦化北路", lat: 25.06, lng: 121.55 })]);
    expect(await engine.searchPlaces("大安")).toEqual([{ kind: "district", label: "台北市大安區", lat: 25.045, lng: 121.555 }]);
    expect((await engine.searchPlaces("臺北市敦化")).map((place) => place.label)).toEqual(["台北市大安區敦化南路一段", "台北市松山區敦化北路"]);
    expect((await engine.searchPlaces("敦化南路一段100號")).map((place) => place.label)).toEqual(["台北市大安區敦化南路一段"]);
    expect(await engine.searchPlaces("敦")).toEqual([]);
  });
});
