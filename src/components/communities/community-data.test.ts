import { describe, expect, it } from "vitest";
import type { CommunityDetail, TransactionData } from "../../lib/api";
import { buildingAge, doorAddress, fiveYearsBefore, floorBands, visibleSales, yearRows } from "./community-data";

function sale(overrides: Partial<TransactionData>): TransactionData {
  return {
    id: "S1", cityCode: "a", district: "中山區", address: "台北市中山區敬業三路162巷100號五樓", transactionType: "房地(土地+建物)",
    buildingType: "住宅大樓(11層含以上有電梯)", mainUse: "住家用", totalPrice: 30_000_000, parkingPrice: 0, buildingPrice: 30_000_000,
    buildingAreaPing: 40, pricePerPing: 750_000, rooms: 3, halls: 2, baths: 2, floorLabel: "五層", totalFloors: 15,
    transactedAt: "2025-06-01T00:00:00Z", season: "114S3", lat: 25.07, lng: 121.55, geocodePrecision: "address", ...overrides,
  };
}

describe("community data", () => {
  it("continues the trend dashed from the last settled year", () => {
    const rows = yearRows([
      { year: 2024, sales: 10, medianPricePerPing: 700_000, districtMedianPricePerPing: 800_000, provisional: false },
      { year: 2025, sales: 12, medianPricePerPing: 720_000, districtMedianPricePerPing: null, provisional: false },
      { year: 2026, sales: 3, medianPricePerPing: 740_000, districtMedianPricePerPing: 820_000, provisional: true },
    ]);
    expect(rows.map((row) => [row.community, row.communityTail, row.district, row.districtTail])).toEqual([
      [70, null, 80, null], [72, 72, null, null], [null, 74, null, 82],
    ]);
    expect(yearRows([{ year: 2025, sales: 1, medianPricePerPing: 500_000, districtMedianPricePerPing: null, provisional: false }])[0].communityTail).toBeNull();
  });

  it("splits floors into the ground floor and thirds of the building", () => {
    const sales = [
      sale({ floorLabel: "一層", pricePerPing: 900_000 }),
      sale({ floorLabel: "二層", pricePerPing: 600_000 }), sale({ floorLabel: "六層", pricePerPing: 640_000 }),
      sale({ floorLabel: "十一層", pricePerPing: 700_000 }), sale({ floorLabel: "十五層", pricePerPing: 760_000 }),
      sale({ floorLabel: "十二層", pricePerPing: 740_000 }), sale({ floorLabel: "十四層", pricePerPing: 780_000 }),
      sale({ floorLabel: "十層", pricePerPing: 100_000, specialReasons: ["親友交易"] }),
      sale({ floorLabel: "九層", pricePerPing: 100_000, transactedAt: "2019-01-01T00:00:00Z" }),
      sale({ floorLabel: "地下一層", pricePerPing: 100_000 }),
    ];
    const bands = floorBands(sales, 15, "2021-03-01");
    expect(bands.map((band) => [band.key, band.from, band.to, band.sales, band.median])).toEqual([
      ["high", 12, 15, 3, 760_000], ["mid", 7, 11, 1, 700_000], ["low", 2, 6, 2, 620_000], ["ground", 1, 1, 1, 900_000],
    ]);
    expect(bands[0].low).toBe(750_000);
    expect(bands[0].high).toBe(770_000);
    // Without a recorded height, the highest floor sold sets the thirds.
    expect(floorBands([sale({ floorLabel: "四層" })], null, "2021-03-01").map((band) => [band.key, band.from, band.to])).toEqual([["high", 4, 4]]);
    expect(floorBands([], 12, "2021-03-01")).toEqual([]);
  });

  it("filters the sales table", () => {
    const detail = { through: "2026-02", sales: [
      sale({ id: "new", transactedAt: "2026-01-01T00:00:00Z" }),
      sale({ id: "special", transactedAt: "2025-01-01T00:00:00Z", specialReasons: ["親友交易"] }),
      sale({ id: "old", transactedAt: "2023-02-28T00:00:00Z" }),
    ] } as CommunityDetail;
    expect(visibleSales(detail, false, false).map((row) => row.id)).toEqual(["new", "special", "old"]);
    expect(visibleSales(detail, true, true).map((row) => row.id)).toEqual(["new"]);
  });

  it("reads door numbers, ages and windows", () => {
    expect(doorAddress("台北市中山區敬業三路162巷100號五樓")).toBe("台北市中山區敬業三路162巷100號");
    expect(doorAddress("台北市南港區興南街101之1號十二樓")).toBe("台北市南港區興南街101之1號");
    expect(buildingAge("2000-06-28", new Date("2026-10-12"))).toBe(26);
    expect(buildingAge("2027-01-01", new Date("2026-10-12"))).toBeNull();
    expect(fiveYearsBefore("2026-02")).toBe("2021-03-01");
  });
});
