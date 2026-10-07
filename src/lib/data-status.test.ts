import { describe, expect, it } from "vitest";
import type { ImportRun } from "./api";
import { datasetMatrix, periodLabel, runDuration, seasonCells, seasonInProgress } from "./data-status";

const run = (patch: Partial<ImportRun>): ImportRun => ({
  id: 1, source: "lvr", dataset: "sales", season: "115S2", cityCode: "a", status: "succeeded",
  rowsRead: 0, rowsSkipped: 0, rowsUpserted: 0, rowsGeocodeFailed: 0,
  startedAt: "2026-09-30T06:16:00Z", finishedAt: "2026-09-30T06:16:06.1Z", ...patch,
});

describe("data status helpers", () => {
  it("finds the season being registered", () => {
    expect(seasonInProgress(new Date(2026, 8, 30))).toBe("115S4");
    expect(seasonInProgress(new Date(2026, 8, 10))).toBe("115S3");
    expect(seasonInProgress(new Date(2026, 2, 10))).toBe("115S1");
    expect(seasonInProgress(new Date(2026, 11, 11))).toBe("116S1");
  });

  it("marks imported, missing, pending and earlier seasons", () => {
    const cells = seasonCells({ cityCode: "a", firstImported: "101S2", latestImported: "115S2", missing: ["110S3"] }, new Date(2026, 8, 30));
    const state = (season: string) => cells.find((cell) => cell.season === season)?.state;
    expect(cells[0]).toEqual({ season: "101S1", state: "before" });
    expect(cells.at(-1)?.season).toBe("115S4");
    expect([state("101S2"), state("110S3"), state("115S2"), state("115S3")]).toEqual(["imported", "missing", "imported", "pending"]);
    expect(seasonCells(undefined, new Date(2026, 8, 30)).every((cell) => cell.state === "pending")).toBe(true);
  });

  it("pairs each dataset's latest attempt with its latest success", () => {
    const failed = run({ id: 2, dataset: "transit", cityCode: "", status: "failed", errorKind: "rate_limited" });
    const succeeded = run({ id: 1, dataset: "transit", cityCode: "" });
    const matrix = datasetMatrix([failed], [succeeded]);
    expect(matrix.get("transit|")).toEqual({ latest: failed, lastSuccess: succeeded });
  });

  it("formats release periods and durations", () => {
    expect(periodLabel("1150901-1150910")).toBe("115/09/01–09/10");
    expect(periodLabel("115S2")).toBe("115S2");
    expect(runDuration(run({}))).toBe("6.1s");
    expect(runDuration(run({ finishedAt: "2026-09-30T06:22:20Z" }))).toBe("6m20s");
    expect(runDuration(run({ finishedAt: null }))).toBeNull();
  });
});
