import { describe, expect, it } from "vitest";
import { formatRentValue, rentBreaks } from "./rent-scale";

describe("rentBreaks", () => {
  it("rounds rent quartiles to 50 元", () => {
    expect(rentBreaks([900, 1010, 1180, 1260, 1475], "rent")).toEqual([1000, 1200, 1250]);
  });

  it("rounds yield quartiles to 0.1 percentage points", () => {
    const breaks = rentBreaks([0.0181, 0.0204, 0.0226, 0.0262, 0.031], "yield")!;
    expect(breaks.map((value) => Number(value.toFixed(4)))).toEqual([0.02, 0.023, 0.026]);
  });

  it("ignores missing values", () => {
    expect(rentBreaks([], "rent")).toBeNull();
    expect(rentBreaks([Number.NaN, 0], "yield")).toBeNull();
  });
});

describe("formatRentValue", () => {
  it("formats rents and yields", () => {
    expect(formatRentValue(1320.4, "rent")).toBe("1,320");
    expect(formatRentValue(0.0213, "yield")).toBe("2.1%");
  });
});
