import { describe, expect, it } from "vitest";
import type { DataStatus, ImportRun } from "../lib/api";
import type { SystemStatus } from "../lib/api/admin";
import { backupTone, diskTone, problems, syncTone } from "./health";
import { formatBytes } from "./shared";

const now = new Date("2026-10-01T09:00:00Z");
const run = (patch: Partial<ImportRun>): ImportRun => ({
  id: 1, source: "tdx", dataset: "transit", season: "2026-09", cityCode: "", status: "succeeded",
  rowsRead: 0, rowsSkipped: 0, rowsUpserted: 0, rowsGeocodeFailed: 0,
  startedAt: "2026-09-30T13:00:00Z", finishedAt: "2026-09-30T13:01:00Z", ...patch,
});
const system = (patch: Partial<SystemStatus> = {}): SystemStatus => ({
  generatedAt: now.toISOString(), database: { bytes: 1, transactions: 1, tables: [] },
  disk: { totalBytes: 194, usedBytes: 169, freeBytes: 25 },
  backups: [{ name: "geoprophet-20261001T003435Z.dump", bytes: 127 << 20, modifiedAt: "2026-10-01T00:34:35Z" }],
  backupKeep: 7, backupIntervalHours: 24, ...patch,
});
const status = (patch: Partial<DataStatus> = {}): DataStatus => ({
  generatedAt: now.toISOString(), recentRuns: [], latestRuns: [], lastSuccessfulRuns: [], cities: [],
  coverage: [{ cityCode: "a", firstImported: "101S2", latestImported: "115S2", missing: [] }],
  quality: { metrics: [], alerts: [] }, ...patch,
});

describe("admin health", () => {
  it("rates disk use and backup age", () => {
    expect(diskTone(system())).toBe("amber"); // 87%
    expect(diskTone(system({ disk: { totalBytes: 100, usedBytes: 96, freeBytes: 4 } }))).toBe("red");
    expect(diskTone(system({ disk: null }))).toBe("grey");
    expect(backupTone(system(), now)).toBe("green");
    expect(backupTone(system(), new Date("2026-10-02T14:00:00Z"))).toBe("amber"); // 37 h old
    expect(backupTone(system({ backups: [] }), now)).toBe("red");
  });

  it("treats a failure followed by a success as recovered", () => {
    const failed = run({ id: 1, status: "failed", startedAt: "2026-09-30T12:52:00Z", errorKind: "rate_limited", error: "429 Too Many Requests" });
    const retried = run({ id: 2, startedAt: "2026-09-30T12:54:00Z" });
    expect(syncTone(status({ latestRuns: [retried], lastSuccessfulRuns: [retried] }))).toBe("green");
    expect(syncTone(status({ latestRuns: [failed], lastSuccessfulRuns: [run({ startedAt: "2026-09-23T00:00:00Z" })] }))).toBe("red");

    const found = problems(status({ recentRuns: [retried, failed] }), system(), now);
    expect(found.map((problem) => `${problem.severity}:${problem.kind}`)).toEqual(["warning:disk", "info:recovered", "info:seasonPending"]);
    expect(found[1].run?.error).toBe("429 Too Many Requests");
    expect(found[2].values.season).toBe("115S3");
  });

  it("lists unrecovered failures first", () => {
    const failed = run({ status: "failed", startedAt: "2026-09-25T00:00:00Z" });
    expect(problems(status({ recentRuns: [failed], coverage: [] }), system({ disk: null }), now).map((problem) => problem.kind)).toEqual(["failed"]);
  });

  it("formats sizes like df", () => {
    expect([formatBytes(132_781_039), formatBytes(1_390_000_000), formatBytes(512)]).toEqual(["127 MB", "1.3 GB", "512 B"]);
  });
});
