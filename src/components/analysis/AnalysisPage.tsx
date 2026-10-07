import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { AlertCircle, BarChart3, ChevronDown, RotateCcw } from "lucide-react";
import { BUILDING_TYPES, type AnalysisQuery, type BuildingType, type MarketAnalysis, type TransactionAPI } from "../../lib/api";
import { useLanguage } from "../../lib/i18n";
import { formatWan, recentMonths } from "../../lib/price-scale";
import { cn } from "../../lib/utils";
import { RangeField, Switch, TypeChips } from "../ui";

const AnalysisChart = lazy(() => import("./AnalysisChart"));
const PERIODS = [36, 60, 0] as const;
type Period = (typeof PERIODS)[number];
type Breakdown = "type" | "age" | "area";
const WAN = 10_000;

interface Props {
  api: TransactionAPI;
  priceRange: [number, number];
  setPriceRange: (value: [number, number]) => void;
  areaRange: [number, number];
  setAreaRange: (value: [number, number]) => void;
  selectedTypes: BuildingType[];
  setSelectedTypes: (value: BuildingType[]) => void;
  includeSpecial: boolean;
  setIncludeSpecial: (value: boolean) => void;
  onReset: () => void;
}

export default function AnalysisPage(props: Props) {
  const { t } = useLanguage();
  const [period, setPeriod] = useState<Period>(60);
  const [breakdown, setBreakdown] = useState<Breakdown>("type");
  const [data, setData] = useState<MarketAnalysis | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [filtersOpen, setFiltersOpen] = useState(() => window.innerWidth >= 768);
  const to = useMemo(() => recentMonths(new Date(), 1)[0], []);
  const from = useMemo(() => period === 0 ? "2000-01" : recentMonths(new Date(), period)[0], [period]);
  const query = useMemo<AnalysisQuery>(() => ({
    priceRange: props.priceRange,
    areaRange: props.areaRange,
    types: props.selectedTypes,
    includeSpecial: props.includeSpecial,
    from,
    to,
  }), [props.priceRange, props.areaRange, props.selectedTypes, props.includeSpecial, from, to]);

  useEffect(() => {
    if (query.types.length === 0) {
      setData({ monthly: [], categories: [], ageGroups: [], areaGroups: [], distribution: [] });
      setError(null);
      setLoading(false);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setLoading(true);
      setError(null);
      try {
        setData(await props.api.analysis(query, controller.signal));
      } catch (cause) {
        if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : t("analysis.error"));
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }, 250);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [props.api, query, retry]);

  const total = data?.monthly.reduce((sum, month) => sum + month.count, 0) ?? 0;
  const latest = data?.monthly.at(-1);
  const groups = breakdown === "type" ? data?.categories.map((group) => ({ key: group.category, ...group }))
    : breakdown === "age" ? data?.ageGroups : data?.areaGroups;
  const maxGroup = Math.max(1, ...(groups?.map((group) => group.medianPricePerPing) ?? []));
  const maxBucket = Math.max(1, ...(data?.distribution.map((bucket) => bucket.count) ?? []));
  const sortedMonths = [...(data?.monthly ?? [])].reverse();

  return (
    <main className="h-full overflow-y-auto bg-canvas">
      <div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p className="text-xs font-semibold tracking-wide text-brand-700">GEOPROPHET · {t("nav.analysis")}</p>
            <h1 className="mt-1 text-2xl font-semibold text-slate-900">{t("analysis.title")}</h1>
            <p className="mt-1 text-sm text-slate-500">{from} – {to} · {t("analysis.scope")}</p>
          </div>
          <div className="flex rounded-lg border border-slate-200 bg-white p-1" aria-label={t("analysis.period")}>
            {PERIODS.map((option) => (
              <button key={option} onClick={() => setPeriod(option)} aria-pressed={period === option}
                className={cn("rounded-md px-3 py-1.5 text-xs font-medium transition sm:text-sm", period === option ? "bg-brand-600 text-white" : "text-slate-500 hover:bg-slate-50")}>
                {t(`analysis.period${option}`)}
              </button>
            ))}
          </div>
        </div>

        <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
          <button className="flex w-full items-center justify-between px-5 py-4 text-left" aria-expanded={filtersOpen} onClick={() => setFiltersOpen(!filtersOpen)}>
            <span className="flex items-center gap-2 text-sm font-semibold text-slate-800"><BarChart3 className="h-4 w-4 text-brand-600" />{t("analysis.filters")}</span>
            <ChevronDown className={cn("h-4 w-4 text-slate-400 transition", filtersOpen && "rotate-180")} />
          </button>
          {filtersOpen && (
            <div className="grid gap-5 border-t border-slate-100 px-5 pb-5 pt-4 lg:grid-cols-[1.6fr_1fr_1fr]">
              <TypeChips label={t("filter.type")} selected={props.selectedTypes} onChange={props.setSelectedTypes} />
              <RangeField label={t("filter.price")} unit="萬" value={[props.priceRange[0] / WAN, props.priceRange[1] / WAN]} onChange={([low, high]) => props.setPriceRange([low * WAN, high * WAN])} />
              <RangeField label={t("filter.area")} unit="坪" value={props.areaRange} onChange={props.setAreaRange} />
              <div className="flex items-center justify-between gap-4 lg:col-span-3">
                <label className="flex items-center gap-3 text-xs text-slate-600"><Switch checked={props.includeSpecial} onChange={props.setIncludeSpecial} />{t("filter.special")}</label>
                <button onClick={props.onReset} className="flex items-center gap-1 text-xs font-medium text-brand-700"><RotateCcw className="h-3.5 w-3.5" />{t("filter.reset")}</button>
              </div>
            </div>
          )}
        </section>

        {error && (
          <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            <span className="flex items-center gap-2"><AlertCircle className="h-4 w-4" />{error}</span>
            <button onClick={() => setRetry((value) => value + 1)} className="font-semibold underline">{t("analysis.retry")}</button>
          </div>
        )}
        {loading && !data ? <LoadingState /> : data && total === 0 ? (
          <div className="rounded-2xl border border-slate-200 bg-white px-6 py-16 text-center text-sm text-slate-500">
            {t("analysis.empty")}
            <button onClick={props.onReset} className="ml-3 font-medium text-brand-700 underline">{t("filter.reset")}</button>
          </div>
        ) : data && (
          <div className={cn("space-y-5 transition-opacity", loading && "opacity-50")}>
            <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
              <Stat label={t("analysis.totalSales")} value={total.toLocaleString("zh-TW")} unit={t("analysis.salesCount")} />
              <Stat label={t("analysis.months")} value={String(data.monthly.length)} unit={t("analysis.monthUnit")} />
              <Stat label={t("analysis.latestPrice")} value={latest ? formatWan(latest.avgPricePerPing) : "—"} unit={t("chart.unit")} />
              <Stat label={t("analysis.latestMonth")} value={latest?.month ?? "—"} unit={latest?.provisional ? t("analysis.provisional") : ""} />
            </div>

            <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_4px_16px_rgba(15,23,42,0.04)] sm:p-6">
              <h2 className="text-base font-semibold text-slate-900">{t("analysis.trend")}</h2>
              <p className="mt-1 text-xs text-slate-500">{t("analysis.trendHint")}</p>
              <Suspense fallback={<div className="h-72 animate-pulse rounded-lg bg-slate-100" />}><AnalysisChart monthly={data.monthly} /></Suspense>
              {data.monthly.some((month) => month.provisional) && <p className="text-xs text-amber-700">{t("analysis.provisionalHint")}</p>}
            </section>

            <div className="grid min-w-0 gap-5 lg:grid-cols-2">
              <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
                <h2 className="text-base font-semibold text-slate-900">{t("analysis.categories")}</h2>
                <p className="mt-1 text-xs text-slate-500">{t("analysis.categoryHint")}</p>
                <div className="mt-4 flex flex-wrap gap-1 rounded-lg bg-slate-100 p-1" aria-label={t("analysis.breakdown")}>
                  {(["type", "age", "area"] as const).map((option) => <button key={option} aria-pressed={breakdown === option} onClick={() => setBreakdown(option)}
                    className={cn("rounded-md px-3 py-1.5 text-xs font-medium", breakdown === option ? "bg-white text-brand-700 shadow-sm" : "text-slate-500 hover:text-slate-700")}>{t(`analysis.breakdown.${option}`)}</button>)}
                </div>
                <div className="mt-5 space-y-4">
                  {groups?.map((group) => (
                    <div key={group.key}>
                      <div className="mb-1 flex justify-between gap-3 text-sm"><span>{t(breakdown === "type" ? `type.${group.key}` : `analysis.${breakdown}.${group.key}`)}</span><span className="tabular-nums font-semibold">{formatWan(group.medianPricePerPing)} {t("chart.unit")}</span></div>
                      <div className="h-2 rounded-full bg-slate-100"><div className="h-full rounded-full bg-brand-600" style={{ width: `${Math.max(2, group.medianPricePerPing / maxGroup * 100)}%` }} /></div>
                      <div className="mt-1 text-right text-xs tabular-nums text-slate-400">{group.count.toLocaleString("zh-TW")} {t("analysis.salesCount")}</div>
                    </div>
                  ))}
                </div>
              </section>
              <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
                <h2 className="text-base font-semibold text-slate-900">{t("analysis.distribution")}</h2>
                <p className="mt-1 text-xs text-slate-500">{t("analysis.distributionHint")}</p>
                <div className="mt-5 overflow-x-auto">
                  <div className="flex h-48 min-w-max items-end gap-2 border-b border-slate-200 pb-1">
                    {data.distribution.map((bucket) => (
                      <div key={bucket.minPricePerPing} className="flex w-12 flex-col items-center gap-1" title={`${formatWan(bucket.minPricePerPing, 0)}–${formatWan(bucket.maxPricePerPing, 0)} ${t("chart.unit")}: ${bucket.count.toLocaleString("zh-TW")}`}>
                        <span className="text-[10px] tabular-nums text-slate-500">{bucket.count.toLocaleString("zh-TW")}</span>
                        <div className="w-9 rounded-t-sm bg-brand-500" style={{ height: `${Math.max(3, bucket.count / maxBucket * 150)}px` }} />
                        <span className="text-[10px] tabular-nums text-slate-500">{formatWan(bucket.minPricePerPing, 0)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </section>
            </div>

            <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
              <div className="flex items-center justify-between px-5 py-4"><h2 className="text-base font-semibold text-slate-900">{t("analysis.monthlyTable")}</h2><span className="text-xs text-slate-400">{data.monthly.length} {t("analysis.monthUnit")}</span></div>
              <div className="hidden max-h-[520px] overflow-auto md:block"><table className="w-full min-w-[780px] text-sm">
                <thead className="sticky top-0 bg-slate-50 text-xs text-slate-500"><tr>{["month", "count", "meanUnit", "medianUnit", "medianTotal", "avgArea"].map((key) => <th key={key} className="border-y border-slate-100 px-4 py-3 text-right first:text-left">{t(`analysis.col.${key}`)}</th>)}</tr></thead>
                <tbody>{sortedMonths.map((row) => <tr key={row.month} className="border-b border-slate-100 last:border-0"><td className="px-4 py-3 tabular-nums">{row.month}{row.provisional && <span className="ml-2 rounded bg-amber-50 px-1.5 py-0.5 text-[10px] text-amber-700">{t("analysis.provisional")}</span>}</td><td className="px-4 py-3 text-right tabular-nums">{row.count.toLocaleString("zh-TW")}</td><td className="px-4 py-3 text-right tabular-nums">{formatWan(row.avgPricePerPing)}</td><td className="px-4 py-3 text-right tabular-nums">{formatWan(row.medianPricePerPing)}</td><td className="px-4 py-3 text-right tabular-nums">{formatWan(row.medianTotalPrice, 0)}</td><td className="px-4 py-3 text-right tabular-nums">{row.avgArea.toFixed(1)}</td></tr>)}</tbody>
              </table></div>
              <div className="divide-y divide-slate-100 md:hidden">{sortedMonths.map((row) => <div key={row.month} className="px-4 py-3"><div className="flex justify-between text-sm font-semibold"><span>{row.month}{row.provisional && <span className="ml-2 text-xs font-normal text-amber-700">{t("analysis.provisional")}</span>}</span><span>{formatWan(row.avgPricePerPing)} {t("chart.unit")}</span></div><div className="mt-1 text-xs text-slate-500">{row.count.toLocaleString("zh-TW")} {t("analysis.salesCount")} · {t("analysis.col.medianUnit")} {formatWan(row.medianPricePerPing)} · {t("analysis.col.medianTotal")} {formatWan(row.medianTotalPrice, 0)} {t("analysis.wan")}</div></div>)}</div>
            </section>
          </div>
        )}
      </div>
    </main>
  );
}

function Stat({ label, value, unit }: { label: string; value: string; unit: string }) {
  return <div className="min-w-0 rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-[0_4px_16px_rgba(15,23,42,0.04)]"><div className="text-xs text-slate-500">{label}</div><div className="mt-1 flex flex-wrap items-baseline gap-1"><strong className="text-xl font-semibold tabular-nums text-slate-900">{value}</strong><span className="text-xs text-slate-500">{unit}</span></div></div>;
}

function LoadingState() {
  return <div className="space-y-5" aria-label="Loading"><div className="grid grid-cols-2 gap-3 md:grid-cols-4">{Array.from({ length: 4 }, (_, index) => <div key={index} className="h-24 animate-pulse rounded-2xl bg-slate-100" />)}</div><div className="h-96 animate-pulse rounded-2xl bg-slate-100" /><div className="grid gap-5 lg:grid-cols-2"><div className="h-72 animate-pulse rounded-2xl bg-slate-100" /><div className="h-72 animate-pulse rounded-2xl bg-slate-100" /></div></div>;
}
