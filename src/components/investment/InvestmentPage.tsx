import { useEffect, useMemo, useState } from "react";
import { AlertCircle, ArrowDownUp, Info, RotateCw } from "lucide-react";
import type { InvestmentArea, InvestmentQuery, InvestmentReport, TransactionAPI } from "../../lib/api";
import { useLanguage } from "../../lib/i18n";
import { formatWan } from "../../lib/price-scale";
import { INVESTMENT_AGE_BANDS, INVESTMENT_AREA_BANDS, type InvestmentState } from "../../lib/url-state";
import { cn } from "../../lib/utils";
import InvestmentWorkbench from "./InvestmentWorkbench";
import LiquidityResearch from "./LiquidityResearch";

type Sort = "activity" | "price" | "priceChange";
const CITY_CODES = ["a", "f", "h"] as const;
const HOME_TYPES = ["住宅大樓", "華廈", "公寓", "透天厝"] as const;

interface Props {
  api: TransactionAPI;
  state: InvestmentState;
  onChange: (update: Partial<InvestmentState>) => void;
}

function percentage(value: number | null): string {
  if (value === null) return "—";
  const percent = value * 100;
  return `${percent > 0 ? "+" : ""}${percent.toFixed(1)}%`;
}

function unitPrice(value: number | null): string {
  return value === null ? "—" : formatWan(value);
}

function monthBefore(month: string): string {
  const [year, value] = month.split("-").map(Number);
  return new Date(Date.UTC(year, value - 1, 0)).toISOString().slice(0, 7);
}

function sortAreas(areas: InvestmentArea[], sort: Sort): InvestmentArea[] {
  if (sort === "activity") return [...areas].sort((a, b) => b.currentCount - a.currentCount || a.district.localeCompare(b.district));
  const metric = (area: InvestmentArea) => sort === "price" ? area.medianPricePerPing : area.priceChange;
  return [...areas].sort((a, b) => {
    const left = metric(a), right = metric(b);
    if (left === null) return right === null ? b.currentCount - a.currentCount : 1;
    if (right === null) return -1;
    return right - left || b.currentCount - a.currentCount;
  });
}

export default function InvestmentPage({ api, state, onChange }: Props) {
  const { t } = useLanguage();
  const [report, setReport] = useState<InvestmentReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [sort, setSort] = useState<Sort>("activity");
  const query = useMemo<InvestmentQuery>(() => {
    const [minArea, maxArea] = INVESTMENT_AREA_BANDS[state.areaBand];
    const [minAge, maxAge] = INVESTMENT_AGE_BANDS[state.ageBand];
    return { city: state.city, type: state.type, minArea, maxArea, minAge, maxAge };
  }, [state]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    setReport(null);
    api.investmentAreas(query, controller.signal).then((value) => {
      if (!controller.signal.aborted) setReport(value);
    }).catch((cause) => {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : t("investment.error"));
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [api, query, retry]);

  const areas = useMemo(() => sortAreas(report?.areas ?? [], sort), [report, sort]);
  const sufficientDistricts = report?.areas.filter((area) => area.sufficientCurrentSample).length ?? 0;

  return <main className="h-full overflow-y-auto bg-canvas">
    <div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
      <header>
        <p className="text-xs font-semibold tracking-wide text-brand-700">GEOPROPHET · {t("nav.investment")}</p>
        <h1 className="mt-1 text-2xl font-semibold text-slate-900">{t("investment.title")}</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-600">{t("investment.subtitle")}</p>
      </header>

      <div className="flex gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
        <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <p>{t("investment.caution")}</p>
      </div>

      <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]" aria-label={t("investment.filters")}>
        <h2 className="mb-4 text-sm font-semibold text-slate-900">{t("investment.filters")}</h2>
        <div className="grid gap-4 md:grid-cols-3">
          <label className="text-xs font-medium text-slate-600">{t("investment.city")}
            <select value={state.city} onChange={(event) => onChange({ city: event.target.value as InvestmentState["city"] })} className="mt-2 block h-10 w-full rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800">
              {CITY_CODES.map((code) => <option key={code} value={code}>{t(`investment.city.${code}`)}</option>)}
            </select>
          </label>
          <label className="text-xs font-medium text-slate-600">{t("investment.area")}
            <select value={state.areaBand} onChange={(event) => onChange({ areaBand: Number(event.target.value) })} className="mt-2 block h-10 w-full rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800">
              {INVESTMENT_AREA_BANDS.map(([min, max], index) => <option key={`${min}-${max}`} value={index}>{min}–{max} {t("investment.ping")}</option>)}
            </select>
          </label>
          <label className="text-xs font-medium text-slate-600">{t("investment.age")}
            <select value={state.ageBand} onChange={(event) => onChange({ ageBand: Number(event.target.value) })} className="mt-2 block h-10 w-full rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800">
              {INVESTMENT_AGE_BANDS.map(([min, max], index) => <option key={`${min}-${max}`} value={index}>{min}–{max} {t("investment.years")}</option>)}
            </select>
          </label>
        </div>
        <div className="mt-4">
          <p className="mb-2 text-xs font-medium text-slate-600">{t("investment.type")}</p>
          <div className="flex flex-wrap gap-2" role="group" aria-label={t("investment.type")}>
            {HOME_TYPES.map((type) => <button key={type} type="button" onClick={() => onChange({ type })} aria-pressed={state.type === type}
              className={cn("rounded-full border px-3 py-1.5 text-sm transition", state.type === type ? "border-brand-600 bg-brand-50 font-medium text-brand-700" : "border-slate-200 text-slate-600 hover:border-brand-300")}>
              {t(`type.${type}`)}
            </button>)}
          </div>
        </div>
      </section>

      <InvestmentWorkbench api={api} query={query} />
      <LiquidityResearch api={api} query={query} />

      {loading && <p role="status" className="text-sm text-slate-500">{t("status.loading")}</p>}
      {error && <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        <span className="flex items-center gap-2"><AlertCircle className="h-4 w-4" />{error}</span>
        <button type="button" onClick={() => setRetry((value) => value + 1)} className="flex items-center gap-1 font-semibold"><RotateCw className="h-4 w-4" />{t("investment.retry")}</button>
      </div>}

      {report && !report.through && <div className="rounded-xl border border-slate-200 bg-white p-5 text-sm text-slate-600">{t("investment.noCoverage")}</div>}
      {report?.through && <>
        <p className="text-xs text-slate-500">{t("investment.period")}: {report.currentFrom} – {report.through} · {t("investment.previous")}: {report.previousFrom} – {monthBefore(report.currentFrom)}</p>
        <div className="grid gap-3 md:grid-cols-3">
          <Metric title={t("investment.activity")} value={report.cityCurrentCount.toLocaleString()} note={`${t("investment.previousCount")}: ${report.cityPreviousCount.toLocaleString()} · ${percentage(report.cityVolumeChange)}`} />
          <Metric title={t("investment.cityMedian")} value={unitPrice(report.cityMedianPricePerPing)} note={`${t("investment.priceUnit")} · ${t("investment.priceChange")}: ${percentage(report.cityPriceChange)}`} />
          <Metric title={t("investment.sufficientDistricts")} value={`${sufficientDistricts} / ${report.areas.length}`} note={`${t("investment.minimumSample")}: ${report.minimumSample}`} />
        </div>

        <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-5 py-4">
            <div><h2 className="text-base font-semibold text-slate-900">{t("investment.table")}</h2><p className="mt-1 text-xs text-slate-500">{t("investment.tableHint")}</p></div>
            <label className="flex items-center gap-2 text-xs text-slate-600"><ArrowDownUp className="h-4 w-4" />{t("investment.sort")}
              <select value={sort} onChange={(event) => setSort(event.target.value as Sort)} className="h-9 rounded-lg border border-slate-200 bg-white px-2 text-sm text-slate-800">
                <option value="activity">{t("investment.sort.activity")}</option><option value="price">{t("investment.sort.price")}</option><option value="priceChange">{t("investment.sort.priceChange")}</option>
              </select>
            </label>
          </div>
          {areas.length === 0 ? <p className="p-5 text-sm text-slate-500">{t("investment.empty")}</p> : <>
            <div className="hidden overflow-x-auto md:block"><table className="w-full min-w-[900px] text-left text-sm">
              <thead className="bg-slate-50 text-xs text-slate-500"><tr>
                {["district", "count", "prior", "volume", "price", "cityRelative", "priceChange", "evidence"].map((key) => <th key={key} className="px-4 py-3 font-medium">{t(`investment.col.${key}`)}</th>)}
              </tr></thead>
              <tbody className="divide-y divide-slate-100">{areas.map((area) => <tr key={area.district}>
                <td className="px-4 py-3 font-semibold text-slate-900">{area.district}</td><td className="px-4 py-3 tabular-nums">{area.currentCount}</td><td className="px-4 py-3 tabular-nums">{area.previousCount}</td>
                <td className="px-4 py-3 tabular-nums">{percentage(area.volumeChange)}</td><td className="px-4 py-3 tabular-nums">{unitPrice(area.medianPricePerPing)}</td>
                <td className="px-4 py-3 tabular-nums">{percentage(area.relativeToCityMedian)}</td><td className="px-4 py-3 tabular-nums">{percentage(area.priceChange)}</td>
                <td className="px-4 py-3"><Evidence area={area} t={t} /></td>
              </tr>)}</tbody>
            </table></div>
            <div className="divide-y divide-slate-100 md:hidden">{areas.map((area) => <article key={area.district} className="space-y-3 p-4">
              <div className="flex items-center justify-between"><h3 className="font-semibold text-slate-900">{area.district}</h3><Evidence area={area} t={t} /></div>
              <div className="grid grid-cols-2 gap-3 text-xs"><Value label={t("investment.col.count")} value={String(area.currentCount)} /><Value label={t("investment.col.prior")} value={String(area.previousCount)} />
                <Value label={t("investment.col.volume")} value={percentage(area.volumeChange)} /><Value label={t("investment.col.price")} value={unitPrice(area.medianPricePerPing)} />
                <Value label={t("investment.col.cityRelative")} value={percentage(area.relativeToCityMedian)} /><Value label={t("investment.col.priceChange")} value={percentage(area.priceChange)} /></div>
            </article>)}</div>
          </>}
        </section>
        <aside className="rounded-2xl border border-slate-200 bg-white p-5 text-xs leading-6 text-slate-600">
          <h2 className="text-sm font-semibold text-slate-900">{t("investment.methodTitle")}</h2>
          <p className="mt-2">{t("investment.method")}</p>
        </aside>
      </>}
    </div>
  </main>;
}

function Metric({ title, value, note }: { title: string; value: string; note: string }) {
  return <div className="rounded-2xl border border-slate-200 bg-white p-4"><p className="text-xs font-medium text-slate-500">{title}</p><p className="mt-2 text-2xl font-semibold tabular-nums text-slate-900">{value}</p><p className="mt-1 text-xs text-slate-500">{note}</p></div>;
}

function Evidence({ area, t }: { area: InvestmentArea; t: (key: string) => string }) {
  const label = area.sufficientComparisonSample ? t("investment.evidence.full") : area.sufficientCurrentSample ? t("investment.evidence.current") : t("investment.evidence.low");
  return <span className={cn("inline-flex rounded-full px-2 py-1 text-[11px] font-medium", area.sufficientComparisonSample ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-700")}>{label}</span>;
}

function Value({ label, value }: { label: string; value: string }) {
  return <div><p className="text-slate-500">{label}</p><p className="mt-0.5 font-semibold tabular-nums text-slate-900">{value}</p></div>;
}
