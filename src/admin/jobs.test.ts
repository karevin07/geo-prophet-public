import { describe, expect, it } from "vitest";
import type { DataStatus, ImportRun } from "../lib/api";
import type { AdminJob, SystemStatus } from "../lib/api/admin";
import { activeJob, describeSeasons, jobDuration, lastSuccess } from "./jobs";

const job = (patch: Partial<AdminJob>): AdminJob => ({
  id: 1, kind: "transit", status: "succeeded", requestedBy: "admin@example.com", requestedAt: "2026-10-01T02:00:00Z", params: {},
  startedAt: "2026-10-01T02:00:00Z", finishedAt: "2026-10-01T02:06:20Z", result: {}, ...patch,
});
const run = (patch: Partial<ImportRun>): ImportRun => ({
  id: 1, source: "lvr", dataset: "sales_release", season: "1150911-1150920", cityCode: "a", status: "succeeded",
  rowsRead: 0, rowsSkipped: 0, rowsUpserted: 0, rowsGeocodeFailed: 0, startedAt: "2026-10-01T00:36:00Z", finishedAt: "2026-10-01T00:36:40Z", ...patch,
});

describe("admin jobs", () => {
  it("finds the pending job of a kind", () => {
    const jobs = [job({ id: 3, status: "running" }), job({ id: 2, kind: "backup", status: "failed" }), job({ id: 1, kind: "backup", status: "queued" })];
    expect(activeJob(jobs, "transit")?.id).toBe(3);
    expect(activeJob(jobs, "backup")?.id).toBe(1);
    expect(activeJob(jobs, "sync")).toBeUndefined();
  });

  it("measures finished and running jobs", () => {
    expect(jobDuration(job({}), new Date())).toBe("6m20s");
    expect(jobDuration(job({ finishedAt: null }), new Date("2026-10-01T02:00:18Z"))).toBe("18s");
    expect(jobDuration(job({ startedAt: null, finishedAt: null }), new Date())).toBeNull();
  });

  it("knows when each kind last succeeded", () => {
    const status = {
      recentRuns: [run({ dataset: "sales", finishedAt: "2026-10-01T07:47:40Z" }), run({ status: "failed", finishedAt: "2026-10-01T09:00:00Z" })],
      lastSuccessfulRuns: [run({}), run({ dataset: "transit", finishedAt: "2026-09-30T13:01:10Z" })],
    } as DataStatus;
    const system = { backups: [{ name: "geoprophet-20261001T003435Z.dump", bytes: 1, modifiedAt: "2026-10-01T00:34:35Z" }] } as SystemStatus;
    expect(lastSuccess("sync", status, system)).toBe("2026-10-01T07:47:40Z");
    expect(lastSuccess("current_release", status, system)).toBe("2026-10-01T00:36:40Z");
    expect(lastSuccess("transit", status, system)).toBe("2026-09-30T13:01:10Z");
    expect(lastSuccess("backup", status, system)).toBe("2026-10-01T00:34:35Z");
  });

  it("describes a seasons job's input", () => {
    expect(describeSeasons({ seasons: "105S1-115S2", cities: ["a", "h"] }, { a: "台北市", h: "桃園市" })).toBe("105S1–115S2 · 台北市、桃園市");
  });
});
