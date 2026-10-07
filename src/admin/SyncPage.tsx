import { Fragment, useMemo, useState, type ReactNode } from "react";
import { AlertTriangle, CalendarDays, CheckCircle2, Database, History, Layers3, Rows3 } from "lucide-react";
import type { DataStatus, ImportDataset, ImportRun } from "../lib/api";
import { CITIES, CITY_NAMES, DATASETS, datasetMatrix, periodLabel, runDuration, seasonCells, SHARED_DATASETS, type DatasetCell, type SeasonState } from "../lib/data-status";
import { useLanguage } from "../lib/i18n";
import { formatWan } from "../lib/price-scale";
import { cn, fill } from "../lib/utils";
import { CARD, Dot, runTone, shortTime, StatusPill } from "./shared";
const STATE_CLASSES: Record<SeasonState, string> = {
  imported: "bg-brand-600",
  missing: "bg-red-500",
  pending: "border border-slate-300 bg-white",
  before: "bg-slate-100",
};

/** Every dataset's sync results, season coverage and recent imports with their errors. */
export default function SyncPage({ status }: { status: DataStatus }) {
  const { t } = useLanguage();
  const [failedOnly, setFailedOnly] = useState(false);
  const matrix = useMemo(() => datasetMatrix(status.latestRuns, status.lastSuccessfulRuns), [status]);
  const runs = status.recentRuns.filter((run) => !failedOnly || run.status === "failed" || run.status === "interrupted");

  return (
    <div className="space-y-5">
      <Summary status={status} />
      <DatasetTable matrix={matrix} />
      <CoverageGrid status={status} />

      <section className={cn(CARD, "p-4 sm:p-5")}>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900">
            <History className="h-4 w-4 text-brand-600" />{t("data.runs")}
            <span className="text-xs font-normal text-slate-400">{t("data.runsHint")}</span>
          </h2>
          <div className="grid grid-cols-2 rounded-lg bg-slate-100 p-1 text-xs">
            {([false, true] as const).map((value) => (
              <button
                key={String(value)} onClick={() => setFailedOnly(value)} aria-pressed={failedOnly === value}
                className={cn("rounded-md px-3 py-1 font-medium", failedOnly === value ? "bg-white text-slate-900 shadow-sm" : "text-slate-500")}
              >
                {t(value ? "data.filter.failed" : "data.filter.all")}
              </button>
            ))}
          </div>
        </div>
        {runs.length === 0 ? <p className="py-8 text-center text-sm text-slate-500">{t("data.noFailures")}</p> : <RunTable runs={runs} />}
        <p className="mt-3 text-xs text-slate-400">{t("data.writtenNote")}</p>
      </section>
    </div>
  );
}

function Summary({ status }: { status: DataStatus }) {
  const { t } = useLanguage();
  const total = status.cities.reduce((sum, city) => sum + city.transactions, 0);
  const latestSale = status.cities.map((city) => city.latestSaleDate).filter(Boolean).sort().at(-1);
  const release = status.lastSuccessfulRuns.filter((run) => run.dataset === "sales_release").map((run) => run.season).sort().at(-1);
  const firsts = status.coverage.map((coverage) => coverage.firstImported).sort();
  const latests = status.coverage.map((coverage) => coverage.latestImported).sort();
  const missing = status.coverage.reduce((sum, coverage) => sum + coverage.missing.length, 0);
  const lastRun = status.recentRuns.find((run) => run.finishedAt);
  const future = status.cities.filter((city) => city.futureDated > 0);
  const futureCount = future.reduce((sum, city) => sum + city.futureDated, 0);

  return (
    <>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Kpi icon={<Rows3 className="h-4 w-4" />} label={t("data.kpi.sales")} value={total.toLocaleString("zh-TW")} unit={t("data.kpi.salesUnit")}
          note={status.cities.map((city) => `${CITY_NAMES[city.cityCode]?.slice(0, 2) ?? city.cityCode} ${formatWan(city.transactions)}萬`).join(" · ")} />
        <Kpi icon={<CalendarDays className="h-4 w-4" />} label={t("data.kpi.latestSale")} value={latestSale ? latestSale.slice(0, 10).replaceAll("-", "/") : "—"}
          note={release && <span className="flex items-center gap-1 text-emerald-700"><CheckCircle2 className="h-3.5 w-3.5" />{fill(t("data.kpi.latestRelease"), { period: periodLabel(release) })}</span>} />
        <Kpi icon={<Layers3 className="h-4 w-4" />} label={t("data.kpi.coverage")} value={firsts.length ? `${firsts[0]} – ${latests.at(-1)}` : "—"}
          note={<span className={cn("flex items-center gap-1.5", missing ? "text-red-600" : "text-emerald-700")}><Dot tone={missing ? "red" : "green"} />{fill(t("data.kpi.missing"), { count: missing })}</span>} />
        <Kpi icon={<Database className="h-4 w-4" />} label={t("data.kpi.lastSync")} value={lastRun ? shortTime(lastRun.finishedAt) : "—"}
          badge={lastRun && <StatusPill run={lastRun} />} note={t("data.kpi.schedule")} />
      </div>
      {futureCount > 0 && (
        <p className="flex items-start gap-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 text-sm text-amber-800">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          {fill(t("data.future"), { count: futureCount, cities: future.map((city) => `${CITY_NAMES[city.cityCode] ?? city.cityCode} ${city.futureDated}`).join("、") })}
        </p>
      )}
    </>
  );
}

function Kpi({ icon, label, value, unit, note, badge }: { icon: ReactNode; label: string; value: string; unit?: string; note?: ReactNode; badge?: ReactNode }) {
  return (
    <div className={cn(CARD, "px-4 py-4")}>
      <div className="flex items-center justify-between text-xs text-slate-500">{label}<span className="text-slate-400">{badge ?? icon}</span></div>
      <div className="mt-2 flex items-baseline gap-1">
        <strong className="text-2xl font-semibold tabular-nums text-slate-900">{value}</strong>
        {unit && <span className="text-xs text-slate-500">{unit}</span>}
      </div>
      {note && <div className="mt-2 border-t border-slate-100 pt-2 text-xs text-slate-500">{note}</div>}
    </div>
  );
}

/** What a successful run produced: seasons and releases show sales written, others their items. */
function runSummary(run: ImportRun, t: (key: string) => string) {
  const unit = ["door_numbers", "district_boundaries", "transit"].includes(run.dataset) ? t(`data.unit.${run.dataset}`) : t("data.unit.rows");
  const count = run.dataset === "door_numbers" && run.rowsUpserted >= 10_000 ? `${formatWan(run.rowsUpserted)} 萬` : run.rowsUpserted.toLocaleString("zh-TW");
  return `${count} ${unit}`;
}

function DatasetCellView({ cell, dataset }: { cell: DatasetCell | undefined; dataset: ImportDataset }) {
  const { t } = useLanguage();
  const { latest, lastSuccess } = cell ?? {};
  if (!latest && !lastSuccess) return <span className="flex items-center gap-1.5 text-sm text-slate-400"><Dot tone="grey" />{t("data.never")}</span>;
  const failedLast = latest && latest.id !== lastSuccess?.id && latest.status !== "succeeded";
  const shown = lastSuccess ?? latest!;
  return (
    <div className="min-w-0 space-y-1">
      <div className="flex items-center gap-1.5 text-sm text-slate-800">
        <Dot tone={lastSuccess ? "green" : runTone(latest)} />
        <span className="font-medium">{periodLabel(shown.season)}</span>
        {lastSuccess && <span className="text-slate-500">· {runSummary(lastSuccess, t)}</span>}
      </div>
      <div className="pl-3.5 text-xs text-slate-400">
        {lastSuccess ? shortTime(lastSuccess.finishedAt) : shortTime(latest?.startedAt)}
        {dataset === "transit" && ` · ${t("data.weekly")}`}
      </div>
      {failedLast && (
        <div className="ml-3.5 inline-flex flex-wrap gap-x-1 rounded-md bg-red-50 px-2 py-0.5 text-xs text-red-700">
          {fill(t("data.lastAttemptFailed"), { time: shortTime(latest.startedAt) })}
          {latest.errorKind && `（${t(`data.errorKind.${latest.errorKind}`)}）`}
        </div>
      )}
    </div>
  );
}

function DatasetTable({ matrix }: { matrix: Map<string, DatasetCell> }) {
  const { t } = useLanguage();
  return (
    <section className={cn(CARD, "p-4 sm:p-5")}>
      <h2 className="mb-3 flex items-center gap-2 text-base font-semibold text-slate-900"><Database className="h-4 w-4 text-brand-600" />{t("data.datasets")}</h2>
      {/* Wide screens: a dataset × city table. Narrow screens: one card per dataset. */}
      <table className="hidden w-full table-fixed text-left md:table">
        <thead>
          <tr className="border-b border-slate-100 text-xs text-slate-500">
            <th className="w-48 py-2 font-medium">{t("data.col.dataset")}</th>
            {CITIES.map((city) => <th key={city} className="py-2 font-medium">{CITY_NAMES[city]}</th>)}
          </tr>
        </thead>
        <tbody>
          {DATASETS.map((dataset) => (
            <tr key={dataset} className="border-b border-slate-100 last:border-0 align-top">
              <th className="py-3 pr-3 text-sm font-medium text-slate-700">{t(`data.dataset.${dataset}`)}</th>
              {SHARED_DATASETS.includes(dataset)
                ? <td colSpan={CITIES.length} className="py-3"><DatasetCellView cell={matrix.get(`${dataset}|`)} dataset={dataset} /></td>
                : CITIES.map((city) => <td key={city} className="py-3 pr-3"><DatasetCellView cell={matrix.get(`${dataset}|${city}`)} dataset={dataset} /></td>)}
            </tr>
          ))}
        </tbody>
      </table>
      <div className="space-y-3 md:hidden">
        {DATASETS.map((dataset) => (
          <div key={dataset} className="rounded-xl border border-slate-100 p-3">
            <h3 className="mb-2 text-sm font-medium text-slate-700">{t(`data.dataset.${dataset}`)}</h3>
            <div className="space-y-2">
              {SHARED_DATASETS.includes(dataset)
                ? <DatasetCellView cell={matrix.get(`${dataset}|`)} dataset={dataset} />
                : CITIES.map((city) => (
                    <div key={city} className="grid grid-cols-[3.5rem_1fr] gap-2">
                      <span className="text-xs text-slate-500">{CITY_NAMES[city]}</span>
                      <DatasetCellView cell={matrix.get(`${dataset}|${city}`)} dataset={dataset} />
                    </div>
                  ))}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function CoverageGrid({ status }: { status: DataStatus }) {
  const { t } = useLanguage();
  const today = new Date();
  const rows = CITIES.map((city) => ({ city, cells: seasonCells(status.coverage.find((coverage) => coverage.cityCode === city), today) }));
  const years = [...new Set(rows[0].cells.map((cell) => cell.season.slice(0, 3)))];

  return (
    <section className={cn(CARD, "p-4 sm:p-5")}>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900"><Layers3 className="h-4 w-4 text-brand-600" />{t("data.coverage")}</h2>
        <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500">
          {(["imported", "missing", "pending", "before"] as const).map((state) => (
            <li key={state} className="flex items-center gap-1.5"><span className={cn("h-2.5 w-2.5 rounded-sm", STATE_CLASSES[state])} />{t(`data.state.${state}`)}</li>
          ))}
        </ul>
      </div>
      <div className="overflow-x-auto">
        <table className="border-separate border-spacing-y-1.5">
          <thead>
            <tr className="text-[11px] text-slate-400">
              <th className="w-16" />
              {years.map((year) => <th key={year} className="px-1 text-left font-normal tabular-nums">{Number(year)}</th>)}
            </tr>
          </thead>
          <tbody>
            {rows.map(({ city, cells }) => (
              <tr key={city}>
                <th className="whitespace-nowrap pr-3 text-left text-xs font-medium text-slate-600">{CITY_NAMES[city]}</th>
                {years.map((year) => (
                  <td key={year} className="px-1">
                    <div className="flex gap-0.5">
                      {cells.filter((cell) => cell.season.startsWith(year)).map((cell) => (
                        <span key={cell.season} title={`${cell.season} · ${t(`data.state.${cell.state}`)}`} className={cn("h-3.5 w-3.5 rounded-sm", STATE_CLASSES[cell.state])} />
                      ))}
                    </div>
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-2 text-xs text-slate-400">{t("data.coverageNote")}</p>
    </section>
  );
}

function RunTable({ runs }: { runs: ImportRun[] }) {
  const { t } = useLanguage();
  const number = (value: number) => value.toLocaleString("zh-TW");
  return (
    <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
      <table className="w-full min-w-[56rem] text-left text-sm">
        <thead>
          <tr className="border-b border-slate-100 text-xs text-slate-500">
            {["time", "source", "dataset", "city", "period", "status"].map((column) => (
              <th key={column} className="py-2 pr-3 font-medium">{t(column === "dataset" ? "data.col.dataset" : `data.col.${column}`)}</th>
            ))}
            {["read", "skipped", "written", "geocodeFailed", "duration"].map((column) => (
              <th key={column} className="py-2 pl-3 text-right font-medium">{t(`data.col.${column}`)}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {runs.map((run) => (
            <Fragment key={run.id}>
            <tr className={cn("border-b border-slate-100 last:border-0", run.status === "failed" && "border-b-0 bg-red-50/40")}>
              <td className="whitespace-nowrap py-2.5 pr-3 tabular-nums text-slate-700">{shortTime(run.startedAt)}</td>
              <td className="py-2.5 pr-3"><span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">{t(`data.source.${run.source}`)}</span></td>
              <td className="py-2.5 pr-3 text-slate-700">{t(`data.dataset.${run.dataset}`)}</td>
              <td className="py-2.5 pr-3 text-slate-600">{run.cityCode ? CITY_NAMES[run.cityCode] ?? run.cityCode : t("data.allCities")}</td>
              <td className="whitespace-nowrap py-2.5 pr-3 tabular-nums text-slate-600">{periodLabel(run.season)}</td>
              <td className="py-2.5 pr-3"><StatusPill run={run} /></td>
              {run.status === "succeeded"
                ? [run.rowsRead, run.rowsSkipped, run.rowsUpserted, run.rowsGeocodeFailed].map((value, index) => (
                    <td key={index} className={cn("py-2.5 pl-3 text-right tabular-nums", index === 2 ? "font-medium text-slate-800" : "text-slate-500")}>{number(value)}</td>
                  ))
                : [0, 1, 2, 3].map((index) => <td key={index} className="py-2.5 pl-3 text-right text-slate-300">—</td>)}
              <td className="py-2.5 pl-3 text-right tabular-nums text-slate-500">{runDuration(run) ?? "—"}</td>
            </tr>
            {run.error && (
              <tr className="border-b border-slate-100 bg-red-50/40 last:border-0">
                <td colSpan={11} className="px-2 pb-3">
                  <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-lg bg-slate-900 px-3 py-2 font-mono text-xs leading-relaxed text-slate-100">{run.error}</pre>
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
