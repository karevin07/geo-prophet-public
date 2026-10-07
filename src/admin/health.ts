import type { DataStatus, ImportRun } from "../lib/api";
import type { SystemStatus } from "../lib/api/admin";
import { seasonInProgress } from "../lib/data-status";
import type { Tone } from "./shared";

/** Disk use at which the console warns, then alarms. */
export const DISK_WARN = 0.85;
export const DISK_ALARM = 0.95;
/** Recent failures are listed for this long, even after a later success. */
const RECENT_HOURS = 48;

export interface Problem {
  severity: "error" | "warning" | "info";
  /** An i18n key under admin.problem. */
  kind: "disk" | "noBackup" | "staleBackup" | "failed" | "recovered" | "seasonPending" | "futureDated";
  values: Record<string, string | number>;
  run?: ImportRun;
}

const runKey = (run: ImportRun) => `${run.dataset}|${run.cityCode}`;

/** Share of the disk in use, or null when the backup directory is not mounted. */
export function diskShare(system: SystemStatus): number | null {
  return system.disk && system.disk.totalBytes > 0 ? system.disk.usedBytes / system.disk.totalBytes : null;
}

export function diskTone(system: SystemStatus): Tone {
  const share = diskShare(system);
  return share == null ? "grey" : share >= DISK_ALARM ? "red" : share >= DISK_WARN ? "amber" : "green";
}

/** Backups are stale once the newest is half an interval overdue. */
export function backupTone(system: SystemStatus, now: Date): Tone {
  if (!system.disk) return "grey";
  const newest = system.backups[0];
  if (!newest) return "red";
  const ageHours = (now.getTime() - Date.parse(newest.modifiedAt)) / 3_600_000;
  return ageHours > system.backupIntervalHours * 1.5 ? "amber" : "green";
}

/** Red while any dataset's latest attempt failed after its last success. */
export function syncTone(status: DataStatus): Tone {
  const lastSuccess = new Map(status.lastSuccessfulRuns.map((run) => [runKey(run), run]));
  return status.latestRuns.some((run) => (run.status === "failed" || run.status === "interrupted") &&
    (!lastSuccess.get(runKey(run)) || Date.parse(lastSuccess.get(runKey(run))!.startedAt) < Date.parse(run.startedAt)))
    ? "red" : "green";
}

/** The season before the one being registered, which should be published by now or soon. */
function previousSeason(today: Date): string {
  const [year, quarter] = seasonInProgress(today).split("S").map(Number);
  return quarter === 1 ? `${String(year - 1).padStart(3, "0")}S4` : `${String(year).padStart(3, "0")}S${quarter - 1}`;
}

/** Everything an operator should look at, most serious first. */
export function problems(status: DataStatus, system: SystemStatus, now: Date): Problem[] {
  const found: Problem[] = [];
  const share = diskShare(system);
  if (share != null && share >= DISK_WARN) {
    found.push({ severity: share >= DISK_ALARM ? "error" : "warning", kind: "disk", values: { percent: Math.round(share * 100) } });
  }
  if (system.disk && system.backups.length === 0) found.push({ severity: "error", kind: "noBackup", values: {} });
  else if (backupTone(system, now) === "amber") found.push({ severity: "warning", kind: "staleBackup", values: { name: system.backups[0].name } });

  // A failure is recovered when the same dataset and city succeeded after it.
  const cutoff = now.getTime() - RECENT_HOURS * 3_600_000;
  for (const run of status.recentRuns) {
    if (run.status !== "failed" && run.status !== "interrupted") continue;
    const later = status.recentRuns.find((other) => runKey(other) === runKey(run) && other.status === "succeeded" && Date.parse(other.startedAt) > Date.parse(run.startedAt));
    if (later && Date.parse(run.startedAt) < cutoff) continue;
    found.push({ severity: later ? "info" : "error", kind: later ? "recovered" : "failed", values: { time: run.startedAt, recoveredAt: later?.finishedAt ?? "" }, run });
  }

  const expected = previousSeason(now);
  const behind = status.coverage.filter((coverage) => coverage.latestImported < expected);
  if (behind.length > 0) found.push({ severity: "info", kind: "seasonPending", values: { season: expected } });
  const future = status.cities.reduce((sum, city) => sum + city.futureDated, 0);
  if (future > 0) found.push({ severity: "info", kind: "futureDated", values: { count: future } });

  const order = { error: 0, warning: 1, info: 2 };
  return found.sort((a, b) => order[a.severity] - order[b.severity]);
}
