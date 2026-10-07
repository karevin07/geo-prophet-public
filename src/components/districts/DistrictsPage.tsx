import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { AlertCircle, ChevronDown, RotateCcw } from "lucide-react";
import { type BuildingType, type DistrictBoundaries, type DistrictComparison, type DistrictQuery, type DistrictRank, type Region, type TransactionAPI } from "../../lib/api";
import { useLanguage } from "../../lib/i18n";
import { formatWan, recentMonths } from "../../lib/price-scale";
import { cn } from "../../lib/utils";
import { RangeField, Switch, TypeChips } from "../ui";
import YoYBadge from "../YoYBadge";
import { DISTRICT_COLORS } from "./colors";

const DistrictTrendChart = lazy(() => import("./DistrictTrendChart"));
const DistrictMap = lazy(() => import("./DistrictMap"));
const PERIODS = [12, 36, 60] as const;
type Period = (typeof PERIODS)[number];
type RankSort = "median" | "count" | "total" | "yoy";
type Metric = "price" | "volume";
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

function sortedRanking(ranking: DistrictRank[], sort: RankSort) {
  const value = (row: DistrictRank) => sort === "count" ? row.count : sort === "total" ? row.medianTotalPrice
    : sort === "yoy" ? row.priceYoY ?? -Infinity : row.medianPricePerPing;
  return [...ranking].sort((a, b) => value(b) - value(a) || a.district.localeCompare(b.district));
}

export default function DistrictsPage(props: Props) {
  const { t } = useLanguage();
  const [regions, setRegions] = useState<Region[]>([]);
  const [regionsError, setRegionsError] = useState(false);
  const [regionsRetry, setRegionsRetry] = useState(0);
  const [city, setCity] = useState("a");
  const [selected, setSelected] = useState<string[]>([]);
  const [period, setPeriod] = useState<Period>(36);
  const [rankSort, setRankSort] = useState<RankSort>("median");
  const [metric, setMetric] = useState<Metric>("price");
  const [filtersOpen, setFiltersOpen] = useState(() => window.innerWidth >= 768);
  const [data, setData] = useState<DistrictComparison | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const to = useMemo(() => recentMonths(new Date(), 1)[0], []);
  const from = useMemo(() => recentMonths(new Date(), period)[0], [period]);
  const region = regions.find((item) => item.code === city);
  const [boundaries, setBoundaries] = useState<DistrictBoundaries | null>(null);

  // Boundaries only depend on the city; a failure just leaves the map empty.
  useEffect(() => {
    const controller = new AbortController();
    setBoundaries(null);
    props.api.districtBoundaries(city, controller.signal).then(setBoundaries).catch(() => undefined);
    return () => controller.abort();
  }, [props.api, city]);

  useEffect(() => {
    const controller = new AbortController();
    setRegionsError(false);
    props.api.regions(controller.signal).then((items) => {
      if (controller.signal.aborted) return;
      if (items.length === 0) {
        setRegionsError(true);
        setLoading(false);
        return;
      }
      setRegions(items);
      const initial = items.find((item) => item.code === "a") ?? items[0];
      if (initial) {
        setCity(initial.code);
        setSelected(initial.districts.slice(0, 3));
      }
    }).catch(() => { if (!controller.signal.aborted) { setRegionsError(true); setLoading(false); } });
    return () => controller.abort();
  }, [props.api, regionsRetry]);

  const query = useMemo<DistrictQuery | null>(() => region ? {
    city, districts: selected, from, to,
    priceRange: props.priceRange, areaRange: props.areaRange, types: props.selectedTypes, includeSpecial: props.includeSpecial,
  } : null, [region, city, selected, from, to, props.priceRange, props.areaRange, props.selectedTypes, props.includeSpecial]);

  useEffect(() => {
    if (!query) return;
    if (query.types.length === 0) {
      setData({ cityCode: city, yoyThrough: "", ranking: [], trends: [] });
      setLoading(false);
      setError(null);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setLoading(true);
      setError(null);
      try {
        setData(await props.api.compareDistricts(query, controller.signal));
      } catch (cause) {
        if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : t("districts.error"));
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }, 250);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [props.api, query, retry]);

  const ranking = sortedRanking(data?.ranking ?? [], rankSort);
  const total = data?.ranking.reduce((sum, row) => sum + row.count, 0) ?? 0;
  const toggle = (district: string) => setSelected((current) => current.includes(district)
    ? current.filter((item) => item !== district) : current.length < 4 ? [...current, district] : current);
  const switchCity = (next: string) => {
    setCity(next);
    setSelected(regions.find((item) => item.code === next)?.districts.slice(0, 3) ?? []);
    setData(null);
  };

  return (
    <main className="h-full overflow-y-auto bg-canvas">
      <div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p className="text-xs font-semibold tracking-wide text-brand-700">GEOPROPHET · {t("nav.districts")}</p>
            <h1 className="mt-1 text-2xl font-semibold text-slate-900">{t("districts.title")}</h1>
            <p className="mt-1 text-sm text-slate-500">{region?.name ?? "—"} · {from} – {to}</p>
          </div>
          <div className="flex rounded-lg border border-slate-200 bg-white p-1" aria-label={t("districts.period")}>
            {PERIODS.map((option) => <button key={option} onClick={() => setPeriod(option)} aria-pressed={period === option}
              className={cn("rounded-md px-3 py-1.5 text-xs font-medium transition sm:text-sm", period === option ? "bg-brand-600 text-white" : "text-slate-500 hover:bg-slate-50")}>{t(`districts.period${option}`)}</button>)}
          </div>
        </div>

        <section className="rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
          <button className="flex w-full items-center justify-between px-5 py-4 text-left text-sm font-semibold text-slate-800" aria-expanded={filtersOpen} onClick={() => setFiltersOpen(!filtersOpen)}>
            {t("districts.filters")}<ChevronDown className={cn("h-4 w-4 text-slate-400 transition", filtersOpen && "rotate-180")} />
          </button>
          {filtersOpen && <div className="grid gap-5 border-t border-slate-100 px-5 pb-5 pt-4 lg:grid-cols-[1.6fr_1fr_1fr]">
            <label className="text-xs font-medium text-slate-600">{t("districts.city")}
              <select value={city} onChange={(event) => switchCity(event.target.value)} className="mt-2 block h-9 w-full rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800">
                {regions.map((item) => <option key={item.code} value={item.code}>{item.name}</option>)}
              </select>
            </label>
            <RangeField label={t("filter.price")} unit="萬" value={[props.priceRange[0] / WAN, props.priceRange[1] / WAN]} onChange={([low, high]) => props.setPriceRange([low * WAN, high * WAN])} />
            <RangeField label={t("filter.area")} unit="坪" value={props.areaRange} onChange={props.setAreaRange} />
            <div className="lg:col-span-3"><TypeChips label={t("filter.type")} selected={props.selectedTypes} onChange={props.setSelectedTypes} /></div>
            <div className="flex items-center justify-between gap-4 lg:col-span-3">
              <label className="flex items-center gap-3 text-xs text-slate-600"><Switch checked={props.includeSpecial} onChange={props.setIncludeSpecial} />{t("filter.special")}</label>
              <button onClick={props.onReset} className="flex items-center gap-1 text-xs font-medium text-brand-700"><RotateCcw className="h-3.5 w-3.5" />{t("filter.reset")}</button>
            </div>
          </div>}
        </section>

        {regionsError && <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {t("districts.regionsError")}<button onClick={() => setRegionsRetry((value) => value + 1)} className="font-semibold underline">{t("analysis.retry")}</button>
        </div>}
        {error && <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          <span className="flex items-center gap-2"><AlertCircle className="h-4 w-4" />{error}</span>
          <button onClick={() => setRetry((value) => value + 1)} className="font-semibold underline">{t("analysis.retry")}</button>
        </div>}
        {loading && !data ? <div className="h-96 animate-pulse rounded-2xl bg-slate-100" /> : data && total === 0 ? (
          <div className="rounded-2xl border border-slate-200 bg-white px-6 py-16 text-center text-sm text-slate-500">{t("districts.empty")}</div>
        ) : data && <div className={cn("space-y-5 transition-opacity", loading && "opacity-50")}>
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            <Stat label={t("districts.sales")} value={total.toLocaleString("zh-TW")} unit={t("analysis.salesCount")} />
            <Stat label={t("districts.districtCount")} value={String(data.ranking.length)} unit={t("districts.districtUnit")} />
            <Stat label={t("districts.leader")} value={sortedRanking(data.ranking, "median")[0]?.district ?? "—"} unit="" />
            <Stat label={t("districts.yoyThrough")} value={data.yoyThrough || "—"} unit="" />
          </div>

          <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_4px_16px_rgba(15,23,42,0.04)] sm:p-6">
            <h2 className="text-base font-semibold text-slate-900">{t("districts.map")}</h2>
            <p className="mb-4 mt-1 text-xs text-slate-500">{t("districts.mapHint")}</p>
            <Suspense fallback={<div className="h-80 animate-pulse rounded-xl bg-slate-100 sm:h-96" />}>
              <DistrictMap boundaries={boundaries} ranking={data.ranking} selected={selected} onToggle={toggle} />
            </Suspense>
          </section>

          <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_4px_16px_rgba(15,23,42,0.04)] sm:p-6">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div><h2 className="text-base font-semibold text-slate-900">{t("districts.compare")}</h2><p className="mt-1 text-xs text-slate-500">{t("districts.compareHint")}</p></div>
              <div className="flex rounded-lg bg-slate-100 p-1">
                {(["price", "volume"] as const).map((option) => <button key={option} onClick={() => setMetric(option)} aria-pressed={metric === option}
                  className={cn("rounded-md px-3 py-1.5 text-xs font-medium", metric === option ? "bg-white text-brand-700 shadow-sm" : "text-slate-500")}>{t(`districts.metric.${option}`)}</button>)}
              </div>
            </div>
            <div className="mt-4 flex max-h-36 flex-wrap gap-2 overflow-y-auto" aria-label={t("districts.selection")}>
              {region?.districts.map((district) => {
                const position = selected.indexOf(district);
                const active = position >= 0;
                return <button key={district} onClick={() => toggle(district)} disabled={!active && selected.length >= 4} aria-pressed={active}
                  className={cn("rounded-full border px-3 py-1 text-xs font-medium transition disabled:cursor-not-allowed disabled:opacity-40", active ? "bg-white text-slate-800" : "border-slate-200 bg-slate-50 text-slate-500 hover:bg-white")}
                  style={active ? { borderColor: DISTRICT_COLORS[position], boxShadow: `inset 3px 0 ${DISTRICT_COLORS[position]}` } : undefined}>{district}</button>;
              })}
            </div>
            <p className="mt-2 text-xs text-slate-400">{selected.length}/4 {t("districts.selected")}</p>
            {selected.length === 0 || data.trends.length === 0 ? <div className="flex h-64 items-center justify-center text-sm text-slate-400">{t("districts.noTrend")}</div>
              : <Suspense fallback={<div className="h-72 animate-pulse rounded-lg bg-slate-100" />}><DistrictTrendChart districts={selected} trends={data.trends} metric={metric} from={from} to={to} yoyThrough={data.yoyThrough} /></Suspense>}
            {data.trends.some((point) => point.provisional) && <p className="text-xs text-amber-700">{t("analysis.provisionalHint")}</p>}
          </section>

          <section className="min-w-0 rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
            <div className="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
              <div><h2 className="text-base font-semibold text-slate-900">{t("districts.ranking")}</h2><p className="mt-1 text-xs text-slate-500">{t("districts.rankingHint")}</p></div>
              <label className="text-xs text-slate-500">{t("districts.sort")}
                <select value={rankSort} onChange={(event) => setRankSort(event.target.value as RankSort)} className="ml-2 rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-sm text-slate-700">
                  {(["median", "count", "total", "yoy"] as const).map((option) => <option key={option} value={option}>{t(`districts.sort.${option}`)}</option>)}
                </select>
              </label>
            </div>
            <div className="hidden overflow-x-auto md:block"><table className="w-full min-w-[760px] text-sm">
              <thead className="bg-slate-50 text-xs text-slate-500"><tr>{["rank", "name", "count", "median", "total", "yoy"].map((key) => <th key={key} className="border-y border-slate-100 px-4 py-3 text-right first:text-left">{t(`districts.col.${key}`)}</th>)}</tr></thead>
              <tbody>{ranking.map((row, index) => <tr key={row.district} onClick={() => toggle(row.district)}
                className={cn("border-b border-slate-100 last:border-0", selected.includes(row.district) && "bg-brand-50/40", selected.includes(row.district) || selected.length < 4 ? "cursor-pointer hover:bg-slate-50" : "cursor-not-allowed")}>
                <td className="px-4 py-3 tabular-nums">{index + 1}</td><td className="px-4 py-3 text-right font-medium">{row.district}{selected.includes(row.district) && <span className="ml-2 text-xs text-brand-700">●</span>}</td><td className="px-4 py-3 text-right tabular-nums">{row.count.toLocaleString("zh-TW")}</td><td className="px-4 py-3 text-right tabular-nums">{formatWan(row.medianPricePerPing)} {t("chart.unit")}</td><td className="px-4 py-3 text-right tabular-nums">{formatWan(row.medianTotalPrice, 0)} {t("analysis.wan")}</td><td className="px-4 py-3 text-right tabular-nums"><YoY value={row.priceYoY} /></td>
              </tr>)}</tbody>
            </table></div>
            <div className="divide-y divide-slate-100 md:hidden">{ranking.map((row, index) => <button key={row.district} onClick={() => toggle(row.district)} disabled={!selected.includes(row.district) && selected.length >= 4}
              className={cn("flex w-full items-center justify-between gap-3 px-4 py-3 text-left disabled:opacity-50", selected.includes(row.district) && "bg-brand-50/40")}>
              <span><span className="mr-2 text-xs tabular-nums text-slate-400">#{index + 1}</span><span className="text-sm font-semibold text-slate-800">{row.district}</span><span className="mt-1 block text-xs text-slate-500">{row.count.toLocaleString("zh-TW")} {t("analysis.salesCount")} · {t("districts.col.total")} {formatWan(row.medianTotalPrice, 0)} {t("analysis.wan")}</span></span>
              <span className="shrink-0 text-right"><strong className="block text-sm tabular-nums text-slate-800">{formatWan(row.medianPricePerPing)} {t("chart.unit")}</strong><YoY value={row.priceYoY} /></span>
            </button>)}</div>
          </section>
        </div>}
      </div>
    </main>
  );
}

function Stat({ label, value, unit }: { label: string; value: string; unit: string }) {
  return <div className="min-w-0 rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-[0_4px_16px_rgba(15,23,42,0.04)]"><div className="text-xs text-slate-500">{label}</div><div className="mt-1 flex flex-wrap items-baseline gap-1"><strong className="text-xl font-semibold tabular-nums text-slate-900">{value}</strong><span className="text-xs text-slate-500">{unit}</span></div></div>;
}

// Same ▲/▼ badge and colours as the map summary, so a change reads the same everywhere.
function YoY({ value }: { value: number | null }) {
  return value === null ? <span className="text-xs text-slate-400">—</span> : <YoYBadge value={value} />;
}
