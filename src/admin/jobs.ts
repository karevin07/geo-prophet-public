import type { DataStatus } from "../lib/api";
import type { AdminJob, JobKind, SystemStatus } from "../lib/api/admin";

/** Poll quickly while work is pending so cards and history follow it. */
export const ACTIVE_POLL_MS = 5_000;
export const IDLE_POLL_MS = 30_000;

/** The queued or running job of a kind; the API allows at most one. */
export function activeJob(jobs: AdminJob[], kind: JobKind): AdminJob | undefined {
  return jobs.find((job) => job.kind === kind && (job.status === "queued" || job.status === "running"));
}

/** When each kind's work last succeeded, whether scheduled or requested. */
export function lastSuccess(kind: JobKind, status: DataStatus, system: SystemStatus): string | null {
  const latest = (times: (string | null | undefined)[]) => times.filter((time): time is string => Boolean(time)).sort().at(-1) ?? null;
  switch (kind) {
    case "sync":
      return latest(status.recentRuns.filter((run) => run.status === "succeeded").map((run) => run.finishedAt));
    case "current_release":
      return latest(status.lastSuccessfulRuns.filter((run) => run.dataset === "sales_release").map((run) => run.finishedAt));
    case "transit":
      return latest(status.lastSuccessfulRuns.filter((run) => run.dataset === "transit").map((run) => run.finishedAt));
    case "backup":
      return system.backups[0]?.modifiedAt ?? null;
    case "seasons":
      return latest(status.recentRuns.filter((run) => run.dataset === "sales" && run.status === "succeeded").map((run) => run.finishedAt));
  }
}

/** 105S1-115S2 → 105S1–115S2 · 台北市、桃園市, for the job history. */
export function describeSeasons(params: AdminJob["params"], cityNames: Record<string, string>): string {
  const cities = (params.cities ?? []).map((city) => cityNames[city] ?? city).join("、");
  return [params.seasons?.replace("-", "–"), cities].filter(Boolean).join(" · ");
}

/** Run time so far or in total: 18s, 2m14s, 1h05m. */
export function jobDuration(job: AdminJob, now: Date): string | null {
  if (!job.startedAt) return null;
  const seconds = Math.max(0, Math.round(((job.finishedAt ? Date.parse(job.finishedAt) : now.getTime()) - Date.parse(job.startedAt)) / 1000));
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m${String(seconds % 60).padStart(2, "0")}s`;
  return `${Math.floor(minutes / 60)}h${String(minutes % 60).padStart(2, "0")}m`;
}
