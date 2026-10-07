import { describe, expect, it } from "vitest";
import { formatWan, priceBreaks, priceClass, recentMonths } from "./price-scale";

describe("priceBreaks", () => {
  it("splits at quartiles rounded to 萬", () => {
    const values = [400_000, 550_000, 610_000, 720_000, 800_000, 910_000, 1_200_000, 1_500_000, 0, Number.NaN];
    expect(priceBreaks(values)).toEqual([600_000, 760_000, 980_000]);
  });

  it("returns null without data", () => {
    expect(priceBreaks([])).toBeNull();
  });

  it("keeps breaks ascending when values are identical", () => {
    expect(priceBreaks([700_000, 700_000, 700_000])).toEqual([700_000, 700_000, 700_000]);
  });
});

describe("priceClass", () => {
  it("assigns the four classes", () => {
    const breaks: [number, number, number] = [550_000, 750_000, 950_000];
    expect([400_000, 550_000, 800_000, 1_000_000].map((value) => priceClass(value, breaks))).toEqual([0, 1, 2, 3]);
  });
});

describe("formatting helpers", () => {
  it("formats 萬", () => {
    expect(formatWan(725_000)).toBe("72.5");
    expect(formatWan(16_800_000, 0)).toBe("1,680");
  });

  it("lists the last 12 months", () => {
    const months = recentMonths(new Date(Date.UTC(2026, 8, 30)));
    expect(months).toHaveLength(12);
    expect(months[0]).toBe("2025-10");
    expect(months[11]).toBe("2026-09");
  });
});
