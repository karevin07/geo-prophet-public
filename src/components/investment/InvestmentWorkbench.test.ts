import { describe, expect, it } from "vitest";
import { netScenario } from "./InvestmentWorkbench";

describe("investment cost scenario", () => {
  it("uses annual cash flow and total acquisition cost", () => {
    expect(netScenario(10_000_000, 30_000, 2, 60_000, 500_000)).toEqual({ annualCash: 240_000, yield: 240_000 / 10_500_000 });
  });
  it("rejects invalid and incomplete inputs", () => {
    expect(netScenario(0, 30_000, 1, 0, 0)).toBeNull();
    expect(netScenario(10_000_000, 30_000, 13, 0, 0)).toBeNull();
    expect(netScenario(10_000_000, -1, 1, 0, 0)).toBeNull();
  });
});
