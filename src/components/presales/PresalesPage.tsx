import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, Info, MapPin, RotateCw, Search } from "lucide-react";
import { Bar, CartesianGrid, ComposedChart, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { PresaleProject, PresaleProjectDetail, PresaleProjects, Region, TransactionAPI } from "../../lib/api";
import { CITY_NAMES } from "../../lib/data-status";
import { useLanguage } from "../../lib/i18n";
import { formatWan, priceBreaks, PRICE_CLASSES } from "../../lib/price-scale";
import { PRESALE_MONTHS, type PresalesState } from "../../lib/url-state";
import { floorNumber } from "../../lib/floors";
import { cn, fill } from "../../lib/utils";

const ProjectsMap = lazy(() => import("./ProjectsMap"));

const CITY_CODES = ["a", "f", "h"] as const;
const PAGE_SIZE = 30;
const SHOWN_UNITS = 6;
const LEGEND_LABELS = ["legend.low", "legend.midLow", "legend.mid", "legend.high"];

interface Props {
  api: TransactionAPI;
  state: PresalesState;
  onChange: (update: Partial<PresalesState>) => void;
  onShowOnMap: (lat: number, lng: number) => void;
}

function wan(value: number | null, digits = 1): string {
  return value === null ? "—" : formatWan(value, digits);
}

function sortProjects(projects: PresaleProject[], sort: PresalesState["sort"]): PresaleProject[] {
  if (sort === "recent") return projects; // the API lists the latest contract first
  const metric = (project: PresaleProject) => sort === "sales" ? project.recentSales : project.medianPricePerPing ?? -1;
  return [...projects].sort((a, b) => metric(b) - metric(a) || b.lastContract.localeCompare(a.lastContract));
}

export default function PresalesPage({ api, state, onChange, onShowOnMap }: Props) {
  const { t } = useLanguage();
  const [list, setList] = useState<PresaleProjects | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [regions, setRegions] = useState<Region[]>([]);
  const [search, setSearch] = useState("");
  const [shown, setShown] = useState(PAGE_SIZE);
  const detailRef = useRef<HTMLDivElement>(null);
  // On narrow screens the detail sits below the list, so a picked project scrolls to it.
  const pick = (project: string) => {
    onChange({ project });
    if (window.innerWidth < 1024) window.setTimeout(() => detailRef.current?.scrollIntoView({ behavior: "smooth", block: "start" }), 50);
  };

  useEffect(() => {
    const controller = new AbortController();
    api.regions(controller.signal).then(setRegions).catch(() => undefined);
    return () => controller.abort();
  }, [api]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    setList(null);
    setShown(PAGE_SIZE);
    api.presaleProjects({ city: state.city, district: state.district, months: state.months }, controller.signal).then((value) => {
      if (!controller.signal.aborted) setList(value);
    }).catch((cause) => {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : t("presales.error"));
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [api, state.city, state.district, state.months, retry]);

  const projects = useMemo(() => {
    const text = search.trim();
    const all = list?.projects ?? [];
    return sortProjects(text ? all.filter((project) => project.name.includes(text)) : all, state.sort);
  }, [list, search, state.sort]);
  const breaks = useMemo(() => priceBreaks((list?.projects ?? []).flatMap((project) => project.medianPricePerPing ?? [])), [list]);

  // Open the first project when none is chosen, so the page is never half empty.
  useEffect(() => {
    if (!state.project && projects.length > 0) onChange({ project: projects[0].key });
  }, [state.project, projects, onChange]);

  const districts = regions.find((region) => region.code === state.city)?.districts ?? [];
  const select = "h-9 rounded-lg border border-slate-200 bg-white px-2 text-sm text-slate-800";

  return <main className="h-full overflow-y-auto bg-canvas">
    <div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
      <header>
        <p className="text-xs font-semibold tracking-wide text-brand-700">GEOPROPHET · {t("nav.presales")}</p>
        <h1 className="mt-1 text-2xl font-semibold text-slate-900">{t("presales.title")}</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-600">{fill(t("presales.subtitle"), { months: state.months })}</p>
      </header>

      <div className="flex gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
        <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <p>{t("presales.caution")}</p>
      </div>

      <section className="flex flex-wrap items-center gap-x-5 gap-y-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_4px_16px_rgba(15,23,42,0.04)]" aria-label={t("presales.filters")}>
        <label className="flex items-center gap-2 text-xs text-slate-600">{t("rents.city")}
          <select value={state.city} onChange={(event) => onChange({ city: event.target.value as PresalesState["city"], district: "", project: "" })} className={select}>
            {CITY_CODES.map((code) => <option key={code} value={code}>{t(`investment.city.${code}`)}</option>)}
          </select>
        </label>
        <label className="flex items-center gap-2 text-xs text-slate-600">{t("presales.district")}
          <select value={state.district} onChange={(event) => onChange({ district: event.target.value, project: "" })} className={select}>
            <option value="">{t("presales.allDistricts")}</option>
            {districts.map((district) => <option key={district} value={district}>{district}</option>)}
          </select>
        </label>
        <div className="flex items-center gap-2 text-xs text-slate-600">{t("presales.period")}
          <div className="flex rounded-lg bg-slate-100 p-0.5" role="group" aria-label={t("presales.period")}>
            {PRESALE_MONTHS.map((months) => <button key={months} type="button" aria-pressed={state.months === months} onClick={() => onChange({ months, project: "" })}
              className={cn("rounded-md px-3 py-1.5 text-xs font-medium", state.months === months ? "bg-brand-600 text-white shadow-sm" : "text-slate-600")}>
              {t(`presales.months.${months}`)}
            </button>)}
          </div>
        </div>
        <label className="flex items-center gap-2 text-xs text-slate-600">{t("investment.sort")}
          <select value={state.sort} onChange={(event) => onChange({ sort: event.target.value as PresalesState["sort"] })} className={select}>
            <option value="sales">{t("presales.sort.sales")}</option><option value="recent">{t("presales.sort.recent")}</option><option value="price">{t("presales.sort.price")}</option>
          </select>
        </label>
        <label className="relative ml-auto w-full sm:w-64">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t("presales.search")} aria-label={t("presales.search")}
            className="h-9 w-full rounded-lg border border-slate-200 bg-white pl-9 pr-3 text-sm text-slate-800" />
        </label>
      </section>

      {error && <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        <span className="flex items-center gap-2"><AlertCircle className="h-4 w-4" />{error}</span>
        <button type="button" onClick={() => setRetry((value) => value + 1)} className="flex items-center gap-1 font-semibold"><RotateCw className="h-4 w-4" />{t("investment.retry")}</button>
      </div>}

      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,44fr)_minmax(0,56fr)]">
        <div className="space-y-4">
          <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
            <div className="flex items-center justify-between gap-2 border-b border-slate-100 px-4 py-3">
              <h2 className="text-sm font-semibold text-slate-900">{t("presales.map")}</h2>
              <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] text-slate-600">{fill(t("presales.count"), { count: list?.projects.length ?? 0 })}</span>
            </div>
            <div className="relative h-[360px]">
              <Suspense fallback={<div className="h-full animate-pulse bg-slate-100" />}>
                <ProjectsMap projects={projects} breaks={breaks} selected={state.project} onSelect={pick} />
              </Suspense>
              <div className="absolute bottom-2 left-2 z-10 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-white/90 px-2.5 py-1.5 text-[11px] text-slate-600 shadow-sm">
                {[3, 2, 1, 0].map((index) => <span key={index} className="flex items-center gap-1">
                  <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: PRICE_CLASSES[index].hex }} />{t(LEGEND_LABELS[index])}
                  {breaks && <span className="tabular-nums text-slate-400">{
                    index === 3 ? `≥${formatWan(breaks[2], 0)}` : index === 0 ? `<${formatWan(breaks[0], 0)}` : `${formatWan(breaks[index - 1], 0)}–${formatWan(breaks[index], 0)}`
                  }</span>}
                </span>)}
                <span className="text-slate-400">{t("presales.sizeNote")}</span>
              </div>
            </div>
          </section>

          <section className="rounded-2xl border border-slate-200 bg-white p-3 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
            <p className="px-1 pb-2 text-xs text-slate-500">{loading ? t("status.loading") : fill(t("presales.listed"), { shown: Math.min(shown, projects.length), count: projects.length })}</p>
            {!loading && projects.length === 0 && <p className="px-1 py-6 text-center text-sm text-slate-500">{t("presales.empty")}</p>}
            <ul className="space-y-2">
              {projects.slice(0, shown).map((project) => {
                const active = project.key === state.project;
                return <li key={project.key}>
                  <button type="button" onClick={() => pick(project.key)} aria-pressed={active}
                    className={cn("flex w-full items-center justify-between gap-3 rounded-xl border px-3 py-2.5 text-left transition",
                      active ? "border-brand-600 bg-brand-50/60 shadow-[inset_3px_0_0_#1f6f78]" : "border-slate-100 hover:border-slate-300")}>
                    <span className="min-w-0">
                      <span className="flex items-center gap-2">
                        <span className="truncate font-semibold text-slate-900">{project.name}</span>
                        <Precision value={project.geocodePrecision} />
                      </span>
                      <span className="mt-0.5 block truncate text-xs text-slate-500">
                        {[project.district, project.category, project.totalFloors && fill(t("presales.floors"), { floors: project.totalFloors })].filter(Boolean).join(" · ")}
                      </span>
                    </span>
                    <span className="shrink-0 text-right">
                      <span className="block text-sm font-semibold tabular-nums text-slate-900">{wan(project.medianPricePerPing)} <span className="text-xs font-normal text-slate-500">{t("chart.unit")}</span></span>
                      <span className="block text-[11px] text-slate-500">{fill(t("presales.recent"), { count: project.recentSales })}</span>
                    </span>
                  </button>
                </li>;
              })}
            </ul>
            {projects.length > shown && <button type="button" onClick={() => setShown((value) => value + PAGE_SIZE)}
              className="mt-3 w-full rounded-xl border border-slate-200 py-2 text-sm font-medium text-brand-700 hover:bg-slate-50">
              {fill(t("presales.more"), { count: projects.length - shown })}
            </button>}
          </section>
        </div>

        <div ref={detailRef} className="scroll-mt-4 lg:sticky lg:top-4">
          {state.project ? <ProjectDetail api={api} projectKey={state.project} onShowOnMap={onShowOnMap} /> : null}
        </div>
      </div>
    </div>
  </main>;
}

/** "二十三層,陽台" → "23F"; other labels such as 地下二層 are shortened to their first part. */
function floorLabel(label: string): string {
  const floor = floorNumber(label);
  return floor === null ? label.split(",")[0] || "—" : `${floor}F`;
}

function Precision({ value }: { value: string }) {
  const { t } = useLanguage();
  const exact = value === "address" || value === "nearby";
  return <span className={cn("shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium", exact ? "border border-brand-200 bg-brand-50 text-brand-700" : "bg-slate-100 text-slate-500")}>
    {t(`presales.precision.${value || "district"}`)}
  </span>;
}

function ProjectDetail({ api, projectKey, onShowOnMap }: { api: TransactionAPI; projectKey: string; onShowOnMap: (lat: number, lng: number) => void }) {
  const { t } = useLanguage();
  const [detail, setDetail] = useState<PresaleProjectDetail | null>(null);
  const [error, setError] = useState(false);
  const [showAll, setShowAll] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    setDetail(null);
    setError(false);
    setShowAll(false);
    api.presaleProject(projectKey, controller.signal).then((value) => {
      if (!controller.signal.aborted) setDetail(value);
    }).catch(() => {
      if (!controller.signal.aborted) setError(true);
    });
    return () => controller.abort();
  }, [api, projectKey]);

  if (error) return <section className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">{t("presales.detailError")}</section>;
  if (!detail) return <section className="h-[640px] animate-pulse rounded-2xl bg-white" aria-busy />;
  const { project, resale } = detail;
  const units = showAll ? detail.units : detail.units.slice(0, SHOWN_UNITS);
  const rate = project.sales + project.cancelled > 0 ? project.cancelled / (project.sales + project.cancelled) : 0;

  return <section className="space-y-5 rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)] sm:p-6 lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto">
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0">
        <h2 className="text-2xl font-semibold text-slate-900">{project.name}</h2>
        <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-slate-500">
          <span>{CITY_NAMES[project.cityCode]}{project.district}</span>
          {project.category && <span>· {project.category}</span>}
          {project.totalFloors && <span>· {fill(t("presales.floors"), { floors: project.totalFloors })}</span>}
          <span>· {t("presales.location")}</span><Precision value={project.geocodePrecision} />
        </p>
        <p className="mt-1 text-xs text-slate-400">{project.address}</p>
      </div>
      {project.lat !== null && project.lng !== null && (
        <button type="button" onClick={() => onShowOnMap(project.lat!, project.lng!)} className="flex items-center gap-1 text-sm font-medium text-brand-700 hover:underline">
          <MapPin className="h-4 w-4" />{t("presales.showOnMap")}
        </button>
      )}
    </div>

    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Tile label={t("presales.stat.sales")} value={project.sales.toLocaleString()} unit={t("estimate.unit.sales")}
        note={fill(t("presales.stat.cancelled"), { count: project.cancelled, rate: (rate * 100).toFixed(1) })} />
      <Tile label={t("presales.stat.recent")} value={project.recentSales.toLocaleString()} unit={t("estimate.unit.sales")}
        note={fill(t("presales.stat.monthly"), { count: (project.recentSales / 12).toFixed(1) })} />
      <Tile label={t("presales.stat.price")} value={wan(project.medianPricePerPing)} unit={t("chart.unit")}
        note={fill(t("presales.stat.range"), { low: wan(detail.lowPricePerPing), high: wan(detail.highPricePerPing) })} />
      <Tile label={t("presales.stat.total")} value={wan(project.medianTotalPrice, 0)} unit={t("rents.unit.wan")}
        note={[project.medianAreaPing !== null && fill(t("presales.stat.area"), { area: project.medianAreaPing.toFixed(1) }),
          detail.mainRooms !== null && detail.mainRooms > 0 && fill(t("presales.stat.rooms"), { rooms: detail.mainRooms })].filter(Boolean).join(" · ")} />
    </div>

    {resale && <ResaleCard resale={resale} />}

    <div className="grid gap-4 xl:grid-cols-2">
      <div className="rounded-xl border border-slate-100 p-4">
        <h3 className="text-sm font-semibold text-slate-900">{t("presales.pace")}</h3>
        <p className="mt-0.5 text-xs text-slate-500">{t("presales.paceHint")}</p>
        <div className="mt-3 h-56">
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart data={detail.months.map((month) => ({ ...month, price: month.medianPricePerPing === null ? null : month.medianPricePerPing / 10_000 }))} margin={{ top: 8, right: 0, left: -16, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
              <XAxis dataKey="month" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} minTickGap={24} />
              <YAxis yAxisId="sales" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} allowDecimals={false} />
              <YAxis yAxisId="price" orientation="right" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["auto", "auto"]} />
              <Tooltip contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12 }}
                formatter={(value, name) => name === "sales" ? [`${value} ${t("estimate.unit.sales")}`, t("presales.chart.sales")] : [`${Number(value).toFixed(1)} ${t("chart.unit")}`, t("presales.chart.price")]} />
              <Bar yAxisId="sales" dataKey="sales" fill="#4fa3a5" radius={[2, 2, 0, 0]} isAnimationActive={false} />
              <Line yAxisId="price" type="monotone" dataKey="price" stroke="#2f4b7c" strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
            </ComposedChart>
          </ResponsiveContainer>
        </div>
      </div>
      <div className="rounded-xl border border-slate-100 p-4">
        <h3 className="text-sm font-semibold text-slate-900">{t("presales.floorsTitle")}</h3>
        <p className="mt-0.5 text-xs text-slate-500">{t("presales.floorsHint")}</p>
        <div className="mt-3 h-56">
          {detail.floors.length === 0 ? <div className="flex h-full items-center justify-center text-xs text-slate-400">{t("chart.empty")}</div> :
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={detail.floors.map((floor) => ({ ...floor, price: floor.medianPricePerPing / 10_000, label: `${floor.floor}F` }))} margin={{ top: 8, right: 8, left: -16, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
                <XAxis dataKey="label" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} minTickGap={16} />
                <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["auto", "auto"]} />
                <Tooltip contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12 }}
                  formatter={(value, _name, item) => [`${Number(value).toFixed(1)} ${t("chart.unit")}（${item.payload.sales} ${t("estimate.unit.sales")}）`, t("presales.chart.price")]} />
                <Line type="monotone" dataKey="price" stroke="#1f6f78" strokeWidth={2} dot={{ r: 2.5 }} isAnimationActive={false} />
              </LineChart>
            </ResponsiveContainer>}
        </div>
      </div>
    </div>

    <div className="overflow-hidden rounded-xl border border-slate-100">
      <div className="flex items-center justify-between gap-2 border-b border-slate-100 px-4 py-3">
        <h3 className="text-sm font-semibold text-slate-900">{t("presales.units")}</h3>
        <span className="text-xs text-slate-500">{fill(t("presales.unitsCount"), { count: detail.units.length })}</span>
      </div>
      <div className="overflow-x-auto"><table className="w-full min-w-[640px] text-left text-sm">
        <thead className="bg-slate-50 text-xs text-slate-500"><tr>
          {(["date", "unit", "floor", "area", "total", "price", "parking", "status"] as const).map((key) =>
            <th key={key} className={cn("whitespace-nowrap px-2.5 py-2.5 font-medium", ["area", "total", "price", "parking"].includes(key) && "text-right")}>{t(`presales.col.${key}`)}</th>)}
        </tr></thead>
        <tbody className="divide-y divide-slate-100">{units.map((unit) => {
          const cancelled = Boolean(unit.cancellation);
          const special = !cancelled && (unit.specialReasons?.length ?? 0) > 0;
          return <tr key={unit.id} className={cn("[&>td]:whitespace-nowrap", cancelled && "text-slate-400")}>
            <td className="px-2.5 py-2.5 tabular-nums">{unit.transactedAt.slice(0, 10)}</td>
            <td className="px-2.5 py-2.5">{unit.unitLabel || "—"}</td>
            <td className="px-2.5 py-2.5 tabular-nums">{floorLabel(unit.floorLabel)}{unit.totalFloors ? ` / ${unit.totalFloors}` : ""}</td>
            <td className="px-2.5 py-2.5 text-right tabular-nums">{unit.buildingAreaPing.toFixed(1)}</td>
            <td className={cn("px-3 py-2.5 text-right tabular-nums", cancelled && "line-through")}>{formatWan(unit.totalPrice, 0)}</td>
            <td className={cn("px-3 py-2.5 text-right font-semibold tabular-nums", cancelled ? "line-through" : "text-brand-700")}>{formatWan(unit.pricePerPing)}</td>
            <td className="px-2.5 py-2.5 text-right tabular-nums">{unit.parkingPrice > 0 ? formatWan(unit.parkingPrice, 0) : "—"}</td>
            <td className="px-2.5 py-2.5">{cancelled
              ? <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-600">{unit.cancelledOn ? fill(t("presales.status.cancelledOn"), { date: unit.cancelledOn.slice(0, 10) }) : t("presales.status.cancelled")}</span>
              : special ? <span className="rounded bg-amber-50 px-1.5 py-0.5 text-[11px] text-amber-700">{t("presales.status.special")}</span>
                : <span className="flex items-center gap-1 text-[11px] text-emerald-700"><span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />{t("presales.status.valid")}</span>}</td>
          </tr>;
        })}</tbody>
      </table></div>
      {!showAll && detail.units.length > SHOWN_UNITS && <button type="button" onClick={() => setShowAll(true)}
        className="w-full border-t border-slate-100 py-2.5 text-sm font-medium text-brand-700 hover:bg-slate-50">{fill(t("presales.allUnits"), { count: detail.units.length })}</button>}
    </div>
  </section>;
}

function ResaleCard({ resale }: { resale: NonNullable<PresaleProjectDetail["resale"]> }) {
  const { t } = useLanguage();
  const rows = [
    { label: fill(t("presales.resale.project"), { from: resale.projectFrom, through: resale.projectThrough }), value: resale.projectPricePerPing, color: "#4fa3a5" },
    { label: fill(t("presales.resale.all"), { radius: resale.radiusM }), value: resale.resalePricePerPing, color: "#2f4b7c" },
    { label: t("presales.resale.new"), value: resale.newPricePerPing, color: "#2f4b7c" },
  ];
  const max = Math.max(...rows.map((row) => row.value ?? 0), 1);
  const premium = resale.premium;
  return <div className="rounded-xl border border-slate-100 p-4">
    <div className="flex flex-wrap items-center justify-between gap-2">
      <h3 className="text-sm font-semibold text-slate-900">{t("presales.resale.title")}</h3>
      {premium !== null && <span className={cn("rounded-full px-2.5 py-0.5 text-xs font-semibold", premium >= 0 ? "bg-red-50 text-red-700" : "bg-green-50 text-green-700")}>
        {fill(t("presales.resale.premium"), { value: `${premium >= 0 ? "+" : ""}${Math.round(premium * 100)}%` })}
      </span>}
    </div>
    <div className="mt-3 space-y-3">
      {rows.map((row) => <div key={row.label}>
        <div className="flex items-baseline justify-between gap-2 text-xs"><span className="text-slate-600">{row.label}</span>
          <span className={cn("tabular-nums", row.value === null ? "text-slate-400" : "font-semibold text-slate-900")}>{row.value === null ? t("rents.lowSample") : `${formatWan(row.value)} ${t("chart.unit")}`}</span></div>
        <div className="mt-1 h-2 rounded-full bg-slate-100">{row.value !== null && <div className="h-full rounded-full" style={{ width: `${row.value / max * 100}%`, backgroundColor: row.color }} />}</div>
      </div>)}
    </div>
    <p className="mt-3 text-[11px] text-slate-400">{fill(t("presales.resale.note"), { from: resale.from, through: resale.through, radius: resale.radiusM, count: resale.resales, newCount: resale.newResales })}</p>
  </div>;
}

function Tile({ label, value, unit, note }: { label: string; value: string; unit: string; note: string }) {
  return <div className="rounded-xl border border-slate-100 bg-slate-50/60 p-3">
    <p className="text-xs text-slate-500">{label}</p>
    <p className="mt-1 flex items-baseline gap-1"><span className="text-2xl font-semibold tabular-nums text-slate-900">{value}</span><span className="text-xs text-slate-500">{unit}</span></p>
    {note && <p className="mt-0.5 text-[11px] text-slate-500">{note}</p>}
  </div>;
}
