import { afterEach, describe, expect, it, vi } from "vitest";
import { HTTPTransactionAPI } from "./http-transaction-api";

describe("HTTPTransactionAPI", () => {
  afterEach(() => vi.restoreAllMocks());

  it("serializes filters for the H3 endpoint", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: [], meta: { count: 0 } }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));

    await new HTTPTransactionAPI("https://example.test").aggregateH3({
      priceRange: [100, 900],
      areaRange: [10, 40],
      types: ["公寓", "其他"],
      bbox: [121.4, 24.9, 121.7, 25.2],
    }, 9);

    const url = new URL(String(fetchMock.mock.calls[0][0]));
    expect(url.pathname).toBe("/api/v1/aggregations/h3");
    expect(url.searchParams.get("resolution")).toBe("9");
    expect(url.searchParams.get("types")).toBe("公寓,其他");
    expect(url.searchParams.get("bbox")).toBe("121.4,24.9,121.7,25.2");
  });

  it("lists transactions and surfaces API error messages", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      error: { code: "invalid_request", message: "invalid input: invalid price range" },
    }), { status: 400, headers: { "Content-Type": "application/json" } }));

    await expect(new HTTPTransactionAPI().list({ priceRange: [9, 1], areaRange: [0, 100], types: ["公寓"] }))
      .rejects.toThrow("invalid input: invalid price range");
    const url = new URL(String(vi.mocked(fetch).mock.calls[0][0]), "http://localhost");
    expect(url.pathname).toBe("/api/v1/transactions");
    expect(url.searchParams.get("limit")).toBe("2000");
  });

  it("requests multi-year analysis with shared filters and dates", async () => {
    const data = { monthly: [], categories: [], ageGroups: [], areaGroups: [], distribution: [] };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data }), {
      status: 200, headers: { "Content-Type": "application/json" },
    }));

    await expect(new HTTPTransactionAPI("https://example.test").analysis({
      priceRange: [0, 100_000_000], areaRange: [0, 200], types: ["公寓"], includeSpecial: true,
      from: "2021-10", to: "2026-09",
    })).resolves.toEqual(data);
    const url = new URL(String(fetchMock.mock.calls[0][0]));
    expect(url.pathname).toBe("/api/v1/analysis");
    expect(url.searchParams.get("from")).toBe("2021-10");
    expect(url.searchParams.get("to")).toBe("2026-09");
    expect(url.searchParams.get("types")).toBe("公寓");
    expect(url.searchParams.get("include_special")).toBe("true");
  });

  it("serializes selected districts and shared filters", async () => {
    const data = { cityCode: "a", yoyThrough: "2026-02", ranking: [], trends: [] };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data }), {
      status: 200, headers: { "Content-Type": "application/json" },
    }));
    await expect(new HTTPTransactionAPI("https://example.test").compareDistricts({
      city: "a", districts: ["大安區", "信義區"], from: "2023-10", to: "2026-09",
      priceRange: [0, 100_000_000], areaRange: [0, 200], types: ["公寓"],
    })).resolves.toEqual(data);
    const url = new URL(String(fetchMock.mock.calls[0][0]));
    expect(url.pathname).toBe("/api/v1/districts/compare");
    expect(url.searchParams.get("districts")).toBe("大安區,信義區");
    expect(url.searchParams.get("city")).toBe("a");
    expect(url.searchParams.get("from")).toBe("2023-10");
    expect(url.searchParams.get("to")).toBe("2026-09");
  });

  it("requests a single comparable residential cohort", async () => {
    const data = { cityCode: "f", through: "2026-02", areas: [] };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data }), {
      status: 200, headers: { "Content-Type": "application/json" },
    }));
    await expect(new HTTPTransactionAPI("https://example.test").investmentAreas({
      city: "f", type: "公寓", minArea: 15, maxArea: 30, minAge: 5, maxAge: 30,
    })).resolves.toEqual(data);
    const url = new URL(String(fetchMock.mock.calls[0][0]));
    expect(url.pathname).toBe("/api/v1/investment/areas");
    expect(url.searchParams.get("city")).toBe("f");
    expect(url.searchParams.get("type")).toBe("公寓");
    expect(url.searchParams.get("min_age")).toBe("5");
    expect(url.searchParams.get("max_area")).toBe("30");
  });

  it("preserves liquidity coverage and forwards the cohort and abort signal", async () => {
    const data = { cityCode: "f", through: "2026-02", currentCoverageComplete: true, previousCoverageComplete: false, incompleteMonths: ["2024-03"], areas: [] };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data }), {
      status: 200, headers: { "Content-Type": "application/json" },
    }));
    const controller = new AbortController();
    await expect(new HTTPTransactionAPI("https://example.test").investmentLiquidity({
      city: "f", type: "華廈", minArea: 15, maxArea: 30, minAge: 5, maxAge: 15,
    }, controller.signal)).resolves.toEqual(data);
    const url = new URL(String(fetchMock.mock.calls[0][0]));
    expect(url.pathname).toBe("/api/v1/investment/liquidity");
    expect(Object.fromEntries(url.searchParams)).toEqual({ city: "f", type: "華廈", min_area: "15", max_area: "30", min_age: "5", max_age: "15" });
    expect(fetchMock.mock.calls[0][1]?.signal).toBe(controller.signal);
  });
});
