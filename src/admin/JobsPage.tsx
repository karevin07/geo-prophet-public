import { Fragment, useCallback, useEffect, useRef, useState } from "react";
import { AlertCircle, AlertTriangle, CalendarRange, CloudUpload, Download, Loader2, Play, RotateCw, TrainFront } from "lucide-react";
import type { DataStatus } from "../lib/api";
import { adminAPI, AdminError, JOB_KINDS, type AdminJob, type JobKind, type SeasonsParams, type SystemStatus } from "../lib/api/admin";
import { CITIES, CITY_NAMES } from "../lib/data-status";
import { useLanguage } from "../lib/i18n";
import { cn, fill } from "../lib/utils";
import { ACTIVE_POLL_MS, activeJob, describeSeasons, IDLE_POLL_MS, jobDuration, lastSuccess } from "./jobs";
import { CARD, Dot, formatBytes, shortTime } from "./shared";

const ICONS: Record<JobKind, typeof RotateCw> = { sync: RotateCw, current_release: Download, transit: TrainFront, backup: CloudUpload, seasons: CalendarRange };
const SEASONS_PATTERN = /^\d{3}S[1-4](\s*[-,]\s*\d{3}S[1-4])*$/;

interface Props {
  status: DataStatus;
  system: SystemStatus;
  /** Reloads the console's data after a job finishes, so other pages show its effect. */
  onJobFinished: () => void;
}

/** Queue syncs, imports and backups, and follow them until they finish. */
export default function JobsPage({ status, system, onJobFinished }: Props) {
  const { t } = useLanguage();
  const [jobs, setJobs] = useState<AdminJob[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<JobKind | null>(null);
  const [now, setNow] = useState(() => new Date());
  const pending = useRef(new Set<number>());

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const next = await adminAPI.jobs(signal);
    // A job that was pending and no longer is has just finished.
    const stillPending = new Set(next.filter((job) => job.status === "queued" || job.status === "running").map((job) => job.id));
    if ([...pending.current].some((id) => !stillPending.has(id))) onJobFinished();
    pending.current = stillPending;
    setJobs(next);
  }, [onJobFinished]);

  const active = jobs?.some((job) => job.status === "queued" || job.status === "running") ?? false;
  useEffect(() => {
    const controller = new AbortController();
    refresh(controller.signal).catch(() => !controller.signal.aborted && setError(t("admin.loadError")));
    const timer = window.setInterval(() => {
      refresh(controller.signal).catch(() => undefined);
    }, active ? ACTIVE_POLL_MS : IDLE_POLL_MS);
    return () => { controller.abort(); window.clearInterval(timer); };
  }, [refresh, active, t]);

  // Tick once a second while something runs, for the elapsed time.
  useEffect(() => {
    if (!active) return;
    const timer = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, [active]);

  const run = async (kind: JobKind, params?: SeasonsParams) => {
    setConfirming(null);
    setError(null);
    try {
      await adminAPI.enqueue(kind, params);
    } catch (cause) {
      setError(cause instanceof AdminError && cause.status === 409 ? t("admin.job.conflict") : fill(t("admin.job.failedToQueue"), { message: cause instanceof Error ? cause.message : "" }));
    }
    await refresh().catch(() => undefined);
  };

  return (
    <div className="space-y-5">
      {active && <p className="flex items-center gap-2 text-xs text-emerald-700"><Dot tone="green" />{t("admin.jobs.live")}</p>}
      {error && (
        <div role="alert" className="flex items-center gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          <AlertCircle className="h-4 w-4 shrink-0" />{error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {JOB_KINDS.map((kind) => {
          const Icon = ICONS[kind];
          const pendingJob = jobs ? activeJob(jobs, kind) : undefined;
          const last = lastSuccess(kind, status, system);
          return (
            <section key={kind} className={cn(CARD, "flex flex-col p-4", pendingJob && "border-amber-300")}>
              <span className={cn("flex h-9 w-9 items-center justify-center rounded-lg", pendingJob ? "bg-amber-50 text-amber-600" : "bg-brand-50 text-brand-700")}>
                <Icon className="h-4.5 w-4.5" />
              </span>
              <h2 className="mt-3 text-base font-semibold text-slate-900">{t(`admin.job.${kind}`)}</h2>
              <p className="mt-1 min-h-[2.5rem] text-xs leading-relaxed text-slate-500">{t(`admin.job.${kind}.hint`)}</p>
              <p className="mt-3 flex items-center gap-1.5 border-t border-slate-100 pt-3 text-xs text-slate-600">
                <Dot tone={last ? "green" : "grey"} />{last ? fill(t("admin.job.last"), { time: shortTime(last) }) : t("admin.job.never")}
              </p>
              <div className="mt-auto pt-3">
                {pendingJob ? (
                  <>
                    <button disabled className="flex w-full cursor-not-allowed items-center justify-center gap-2 rounded-lg border border-slate-200 bg-slate-100 py-2 text-sm text-slate-500">
                      <Loader2 className="h-4 w-4 animate-spin" />{t(pendingJob.status === "running" ? "admin.job.running" : "admin.job.queued")}
                    </button>
                    {pendingJob.status === "running" && (
                      <p className="mt-2 text-center text-xs text-amber-700">{fill(t("admin.job.elapsed"), { time: jobDuration(pendingJob, now) ?? "" })}</p>
                    )}
                  </>
                ) : confirming === kind ? (
                  <div className="rounded-lg border border-amber-300 bg-amber-50/70 p-3 text-xs text-amber-900">
                    <p className="flex gap-1.5"><AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />{fill(t("admin.job.confirm"), { job: t(`admin.job.${kind}`) })}</p>
                    <div className="mt-2 flex gap-2">
                      <button onClick={() => run(kind)} className="rounded-md bg-brand-600 px-3 py-1.5 font-medium text-white hover:bg-brand-700">{t("admin.job.confirmRun")}</button>
                      <button onClick={() => setConfirming(null)} className="rounded-md px-3 py-1.5 text-slate-600 hover:bg-white">{t("admin.job.cancel")}</button>
                    </div>
                  </div>
                ) : (
                  <button onClick={() => setConfirming(kind)} disabled={!jobs}
                    className="flex w-full items-center justify-center gap-1.5 rounded-lg bg-brand-600 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50">
                    <Play className="h-4 w-4" />{t("admin.job.run")}
                  </button>
                )}
              </div>
            </section>
          );
        })}
      </div>

      <SeasonsCard
        pending={jobs ? activeJob(jobs, "seasons") : undefined} loaded={jobs != null} now={now}
        last={lastSuccess("seasons", status, system)} onRun={(params) => run("seasons", params)}
      />

      <section className={cn(CARD, "p-4 sm:p-5")}>
        <h2 className="text-base font-semibold text-slate-900">{t("admin.jobs.history")}</h2>
        <p className="mt-1 text-xs text-slate-500">{t("admin.jobs.historyHint")}</p>
        {!jobs ? <div className="mt-4 h-40 animate-pulse rounded-xl bg-slate-100" /> : jobs.length === 0
          ? <p className="py-8 text-center text-sm text-slate-500">{t("admin.jobs.empty")}</p>
          : <JobTable jobs={jobs} now={now} />}
      </section>
    </div>
  );
}

function JobTable({ jobs, now }: { jobs: AdminJob[]; now: Date }) {
  const { t } = useLanguage();
  const tone = { queued: "grey", running: "amber", succeeded: "green", failed: "red" } as const;
  const pill = { queued: "bg-slate-100 text-slate-600", running: "bg-amber-50 text-amber-700", succeeded: "bg-emerald-50 text-emerald-700", failed: "bg-red-50 text-red-700" };
  const result = (job: AdminJob) => {
    if (job.status === "queued") return <span className="text-slate-400">{t("admin.jobs.waiting")}</span>;
    if (job.status === "running") return null;
    if (job.result.file) return <span className="font-mono text-xs">{job.result.file} · {formatBytes(job.result.bytes ?? 0)}</span>;
    if (job.result.runs == null) return null;
    if (job.result.runs === 0) return <span className="text-slate-500">{t("admin.jobs.noImports")}</span>;
    return (
      <span>
        {fill(t("admin.jobs.importResult"), { runs: job.result.runs, rows: (job.result.rowsUpserted ?? 0).toLocaleString("zh-TW") })}
        {(job.result.failedRuns ?? 0) > 0 && <span className="text-red-600"> · {fill(t("admin.jobs.importFailures"), { count: job.result.failedRuns ?? 0 })}</span>}
      </span>
    );
  };
  return (
    <div className="-mx-4 mt-3 overflow-x-auto px-4 sm:mx-0 sm:px-0">
      <table className="w-full min-w-[52rem] text-left text-sm">
        <thead>
          <tr className="border-b border-slate-100 text-xs text-slate-500">
            {["status", "job", "by", "queued", "started", "duration", "result"].map((column) => <th key={column} className="py-2 pr-3 font-medium">{t(`admin.jobs.col.${column}`)}</th>)}
          </tr>
        </thead>
        <tbody>
          {jobs.map((job) => (
            <Fragment key={job.id}>
              <tr className={cn("border-b border-slate-100", job.error && "border-b-0 bg-red-50/40")}>
                <td className="py-2.5 pr-3">
                  <span className={cn("inline-flex items-center gap-1 whitespace-nowrap rounded-full px-2 py-0.5 text-xs font-medium", pill[job.status])}>
                    {job.status === "running" ? <Loader2 className="h-3 w-3 animate-spin" /> : <Dot tone={tone[job.status]} />}
                    {t(`admin.jobs.status.${job.status}`)}
                  </span>
                </td>
                <td className="py-2.5 pr-3 font-medium text-slate-800">
                  {t(`admin.job.${job.kind}`)}
                  {job.kind === "seasons" && <div className="text-xs font-normal text-slate-500">{describeSeasons(job.params, CITY_NAMES)}</div>}
                </td>
                <td className="py-2.5 pr-3 text-xs text-slate-600">{job.requestedBy}</td>
                <td className="whitespace-nowrap py-2.5 pr-3 tabular-nums text-slate-600">{shortTime(job.requestedAt)}</td>
                <td className="whitespace-nowrap py-2.5 pr-3 tabular-nums text-slate-600">{shortTime(job.startedAt)}</td>
                <td className="py-2.5 pr-3 tabular-nums text-slate-600">{jobDuration(job, now) ?? "—"}</td>
                <td className="py-2.5 pr-3 text-slate-700">{result(job)}</td>
              </tr>
              {job.error && (
                <tr className="border-b border-slate-100 bg-red-50/40">
                  <td colSpan={7} className="px-2 pb-3">
                    <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-lg bg-slate-900 px-3 py-2 font-mono text-xs leading-relaxed text-slate-100">{job.error}</pre>
                  </td>
                </tr>
              )}
            </Fragment>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Re-import chosen seasons for chosen cities, e.g. after a source correction. */
function SeasonsCard({ pending, loaded, last, now, onRun }: {
  pending: AdminJob | undefined; loaded: boolean; last: string | null; now: Date; onRun: (params: SeasonsParams) => void;
}) {
  const { t } = useLanguage();
  const [seasons, setSeasons] = useState("");
  const [cities, setCities] = useState<string[]>([...CITIES]);
  const [confirming, setConfirming] = useState(false);
  const trimmed = seasons.trim().toUpperCase();
  const valid = SEASONS_PATTERN.test(trimmed) && cities.length > 0;
  const params = { seasons: trimmed, cities };
  return (
    <section className={cn(CARD, "p-4 sm:p-5", pending && "border-amber-300")}>
      <div className="flex flex-wrap items-start gap-3">
        <span className={cn("flex h-9 w-9 shrink-0 items-center justify-center rounded-lg", pending ? "bg-amber-50 text-amber-600" : "bg-brand-50 text-brand-700")}>
          <CalendarRange className="h-4.5 w-4.5" />
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="text-base font-semibold text-slate-900">{t("admin.job.seasons")}</h2>
          <p className="mt-1 text-xs leading-relaxed text-slate-500">{t("admin.job.seasons.hint")}</p>
          <p className="mt-2 flex items-center gap-1.5 text-xs text-slate-600">
            <Dot tone={last ? "green" : "grey"} />{last ? fill(t("admin.job.last"), { time: shortTime(last) }) : t("admin.job.never")}
          </p>
        </div>
      </div>
      <div className="mt-4 flex flex-wrap items-end gap-x-6 gap-y-3 border-t border-slate-100 pt-4">
        <label className="text-xs font-medium text-slate-600">{t("admin.job.seasons.label")}
          <input value={seasons} onChange={(event) => { setSeasons(event.target.value); setConfirming(false); }} placeholder={t("admin.job.seasons.placeholder")}
            disabled={Boolean(pending)} aria-invalid={seasons !== "" && !SEASONS_PATTERN.test(trimmed)}
            className="mt-1.5 block h-9 w-56 rounded-lg border border-slate-200 px-3 font-mono text-sm text-slate-800 aria-[invalid=true]:border-red-300" />
        </label>
        <fieldset className="text-xs font-medium text-slate-600">
          <legend>{t("admin.job.seasons.cities")}</legend>
          <div className="mt-1.5 flex h-9 items-center gap-4">
            {CITIES.map((city) => (
              <label key={city} className="flex items-center gap-1.5 text-sm font-normal text-slate-700">
                <input type="checkbox" checked={cities.includes(city)} disabled={Boolean(pending)}
                  onChange={(event) => { setCities((current) => event.target.checked ? [...current, city] : current.filter((item) => item !== city)); setConfirming(false); }} />
                {CITY_NAMES[city]}
              </label>
            ))}
          </div>
        </fieldset>
        <div className="ml-auto min-w-[12rem]">
          {pending ? (
            <button disabled className="flex w-full cursor-not-allowed items-center justify-center gap-2 rounded-lg border border-slate-200 bg-slate-100 px-4 py-2 text-sm text-slate-500">
              <Loader2 className="h-4 w-4 animate-spin" />
              {pending.status === "running" ? fill(t("admin.job.elapsed"), { time: jobDuration(pending, now) ?? "" }) : t("admin.job.queued")}
            </button>
          ) : (
            <button onClick={() => setConfirming(true)} disabled={!loaded || !valid}
              className="flex w-full items-center justify-center gap-1.5 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50">
              <Play className="h-4 w-4" />{t("admin.job.run")}
            </button>
          )}
        </div>
      </div>
      {confirming && !pending && (
        <div className="mt-3 rounded-lg border border-amber-300 bg-amber-50/70 p-3 text-xs text-amber-900">
          <p className="flex gap-1.5"><AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />{fill(t("admin.job.seasons.confirm"), { what: describeSeasons(params, CITY_NAMES) })}</p>
          <div className="mt-2 flex gap-2">
            <button onClick={() => { setConfirming(false); onRun(params); }} className="rounded-md bg-brand-600 px-3 py-1.5 font-medium text-white hover:bg-brand-700">{t("admin.job.confirmRun")}</button>
            <button onClick={() => setConfirming(false)} className="rounded-md px-3 py-1.5 text-slate-600 hover:bg-white">{t("admin.job.cancel")}</button>
          </div>
        </div>
      )}
    </section>
  );
}
