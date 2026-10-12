import { afterEach, describe, expect, it, vi } from "vitest";
import { snapshotKey } from "./snapshot-key";
import { SnapshotTransactionAPI, snapshotSales } from "./snapshot-transaction-api";

// Two sales as api/cmd/snapshot stores them, one value per column.
const columns = {
  id: ["A1", "A2"], district: ["大安區", "信義區"], address: ["台北市大安區敦化南路一段100號", "台北市信義區松仁路1號"], road: ["敦化南路一段", "松仁路"],
  transactionType: ["房地(土地+建物)", "房地(土地+建物)"], buildingType: ["住宅大樓(11層含以上有電梯)", "公寓(5樓含以下無電梯)"], mainUse: ["住家用", "住家用"],
  totalPrice: [30_000_000, 12_000_000], parkingPrice: [0, 0], buildingPrice: [30_000_000, 12_000_000], buildingAreaPing: [30, 20], pricePerPing: [1_000_000, 600_000],
  rooms: [3, 2], halls: [2, 1], baths: [2, 1], floorLabel: ["五層", "三層"], totalFloors: [12, null], completedAt: ["2001-05-01", null],
  transactedAt: ["2026-09-03", "2026-08-01"], remarks: ["", "含增建"], specialReasons: [[], ["親友交易"]], season: ["115S3", "115S3"],
  lat: [25.041235, 25.03], lng: [121.549877, 121.57], geocodePrecision: ["address", "road"], isNew: [1, 0],
};
const meta = {
  version: 1, generatedAt: "2026-10-12T00:00:00Z", cityCode: "a", from: "2023-11-01", through: "2026-09-03", completeMonth: "2026-06",
  latestRelease: null, region: { code: "a", name: "台北市", districts: ["信義區", "大安區"] }, sales: 2,
};

afterEach(() => vi.unstubAllGlobals());

describe("snapshotSales", () => {
  it("restores sales as the API serves them", () => {
    const [first, second] = snapshotSales(columns, "a");
    expect(first).toMatchObject({
      id: "A1", cityCode: "a", totalFloors: 12, completedAt: "2001-05-01T00:00:00Z", transactedAt: "2026-09-03T00:00:00Z",
      remarks: undefined, specialReasons: undefined, isNew: true, road: "敦化南路一段",
    });
    expect(second).toMatchObject({ totalFloors: undefined, completedAt: undefined, remarks: "含增建", specialReasons: ["親友交易"], isNew: undefined });
  });
});

describe("SnapshotTransactionAPI", () => {
  it("answers sales queries from the snapshot and replays saved reports", async () => {
    const report = { data: { cityCode: "a", districts: [] } };
    const reportURL = `/demo/api/${await snapshotKey("/api/v1/rentals/districts?city=a")}.json`;
    const fetched: string[] = [];
    vi.stubGlobal("fetch", async (url: string) => {
      fetched.push(url);
      const body = { "/demo/meta.json": meta, "/demo/sales.json": columns, [reportURL]: report }[url];
      return body ? new Response(JSON.stringify(body)) : new Response("", { status: 404 });
    });
    const api = new SnapshotTransactionAPI("/demo/");
    const page = await api.feed({ priceRange: [0, 1e10], areaRange: [0, 1000], types: [], city: "a", sort: "date_desc", includeSpecial: true, from: "2026-01", to: "2026-09" });
    expect(page.data.map((sale) => sale.id)).toEqual(["A1", "A2"]);
    expect(await api.regions()).toEqual([meta.region]);
    expect(await api.rentalDistricts({ city: "a", type: "", minArea: 0, maxArea: 0, includeSocial: false })).toEqual(report.data);
    // The snapshot is read once.
    expect(fetched.filter((url) => url.endsWith("sales.json"))).toHaveLength(1);
    // Unsaved reports fail rather than show nothing as if it were data.
    await expect(api.rentalDistricts({ city: "a", type: "公寓", minArea: 0, maxArea: 0, includeSocial: false })).rejects.toThrow("404");
    expect((await api.presaleProjects({ city: "a", months: 12, bbox: [121, 25, 122, 26] })).projects).toEqual([]);
  });

  it("refuses a snapshot of another shape", async () => {
    vi.stubGlobal("fetch", async (url: string) => new Response(JSON.stringify(url.endsWith("meta.json") ? { ...meta, version: 2 } : columns)));
    await expect(new SnapshotTransactionAPI("/demo/").regions()).rejects.toThrow("snapshot version 2");
  });
});
