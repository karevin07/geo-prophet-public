import { useEffect, useMemo, useState } from "react";
import { AlertCircle, ArrowUpRight, ChevronDown, Info, Ruler, RotateCcw, TrainFront, TrendingUp } from "lucide-react";
import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { STATION_RADII, type BuildingType, type DistanceBucket, type OpeningEffect, type StationRadius, type StationRanking, type TransactionAPI, type TransactionQuery, type TransitStation, type TransitSystem } from "../lib/api";
import { CITY_NAMES } from "../lib/data-status";
import { useLanguage } from "../lib/i18n";
import { formatWan, recentMonths } from "../lib/price-scale";
import { distanceRows, MIN_PREMIUM_SALES, nearStationPremium, openingChartRows, openingRows, rankRows, sparkline, type RankSort, type StationFilter } from "../lib/stations";
import { uniqueLines } from "../lib/transit";
import { cn, fill } from "../lib/utils";
import { RangeField, Switch, TypeChips } from "./ui";
import YoYBadge from "./YoYBadge";

const CARD = "rounded-2xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.04)]";
const SYSTEMS: TransitSystem[] = ["metro", "lrt", "tra", "thsr"];
const PERIODS = [12, 36] as const;
const PAGE = 10;
/** One line per city on the distance chart, from the price scale's palette. */
const CITY_COLORS: Record<string, string> = { a: "#2F4B7C", f: "#4FA3A5", h: "#E8704F" };
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
  /** Opens the map on the station with its card. */
  onShowStation: (station: TransitStation) => void;
}

/** A signed percentage-point difference such as +6.4 pp. */
function points(value: number) {
  const rounded = Math.round(value * 1000) / 10;
  return `${rounded > 0 ? "+" : ""}${rounded.toFixed(1)}`;
}

export default function StationsPage(props: Props) {
  const { t } = useLanguage();
  const [stations, setStations] = useState<TransitStation[]>([]);
  const [ranking, setRanking] = useState<StationRanking | null>(null);
  const [openings, setOpenings] = useState<OpeningEffect[] | null>(null);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  const [filter, setFilter] = useState<StationFilter>({ city: "", systems: [], lineId: "" });
  const [radius, setRadius] = useState<StationRadius>(500);
  const [period, setPeriod] = useState<(typeof PERIODS)[number]>(12);
  const [sort, setSort] = useState<RankSort>("median");
  const [shown, setShown] = useState(PAGE);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  // New developments delivering hundreds of units at once swamp a station's sales and sit
  // near new stations, so every section leaves them out by default.
  const [excludeNew, setExcludeNew] = useState(true);
  const [curve, setCurve] = useState<DistanceBucket[] | null>(null);

  const query = useMemo<TransactionQuery>(() => ({
    priceRange: props.priceRange, areaRange: props.areaRange, types: props.selectedTypes, includeSpecial: props.includeSpecial,
    excludeNewBuildings: excludeNew,
  }), [props.priceRange, props.areaRange, props.selectedTypes, props.includeSpecial, excludeNew]);

  useEffect(() => {
    const controller = new AbortController();
    props.api.transitStations(controller.signal).then(setStations).catch(() => !controller.signal.aborted && setError(true));
    return () => controller.abort();
  }, [props.api, retry]);

  // The ranking follows the period; the opening comparison always spans its own years.
  useEffect(() => {
    if (query.types.length === 0) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setError(false);
      props.api.stationRanking(query, radius, recentMonths(new Date(), period)[0], controller.signal)
        .then(setRanking).catch(() => !controller.signal.aborted && setError(true));
    }, 250);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [props.api, query, radius, period, retry]);

  useEffect(() => {
    if (query.types.length === 0) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      props.api.openingEffects(query, radius, controller.signal)
        .then((result) => setOpenings(result.events)).catch(() => !controller.signal.aborted && setError(true));
    }, 250);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [props.api, query, radius, retry]);

  useEffect(() => {
    if (query.types.length === 0) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      props.api.distanceCurve(query, recentMonths(new Date(), period)[0], controller.signal)
        .then(setCurve).catch(() => !controller.signal.aborted && setError(true));
    }, 250);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [props.api, query, period, retry]);

  const lines = useMemo(() => {
    const seen = new Map<string, { id: string; name: string }>();
    for (const station of stations) {
      if (filter.city && station.cityCode !== filter.city) continue;
      for (const line of station.lines) seen.set(line.lineId, { id: line.lineId, name: line.name });
    }
    return [...seen.values()];
  }, [stations, filter.city]);

  const rows = useMemo(() => rankRows(stations, ranking?.stations ?? [], filter, sort), [stations, ranking, filter, sort]);
  const events = useMemo(() => openingRows(openings ?? [], stations, filter), [openings, stations, filter]);
  const active = events.find((event) => `${event.stationId}|${event.lineId}` === selected) ?? events[0];
  const byId = useMemo(() => new Map(stations.map((station) => [station.id, station])), [stations]);

  const update = (patch: Partial<StationFilter>) => {
    setFilter((current) => ({ ...current, ...patch }));
    setShown(PAGE);
  };

  return (
    <main className="h-full overflow-y-auto bg-canvas">
      <div className="mx-auto max-w-6xl space-y-5 px-4 py-6 sm:px-6 sm:py-8">
        <header>
          <h1 className="text-xl font-semibold text-slate-900 sm:text-2xl">{t("stations.title")}</h1>
          <p className="mt-1 text-sm text-slate-500">{t("stations.subtitle")}</p>
        </header>

        <section className={cn(CARD, "space-y-4 p-4 sm:p-5")}>
          <div className="flex flex-wrap items-end gap-x-6 gap-y-4">
            <label className="text-xs font-medium text-slate-600">{t("stations.city")}
              <select value={filter.city} onChange={(event) => update({ city: event.target.value, lineId: "" })} className="mt-1.5 block h-9 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800">
                <option value="">{t("stations.allCities")}</option>
                {Object.entries(CITY_NAMES).map(([code, name]) => <option key={code} value={code}>{name}</option>)}
              </select>
            </label>
            <div className="text-xs font-medium text-slate-600">{t("stations.system")}
              <div className="mt-1.5 flex flex-wrap gap-1.5">
                {SYSTEMS.map((system) => {
                  const on = filter.systems.includes(system);
                  return (
                    <button key={system} aria-pressed={on}
                      onClick={() => update({ systems: on ? filter.systems.filter((item) => item !== system) : [...filter.systems, system] })}
                      className={cn("h-9 rounded-full border px-3 text-sm", on ? "border-brand-600 bg-brand-50 text-brand-700" : "border-slate-200 text-slate-600 hover:border-slate-300")}>
                      {t(`transit.system.${system}`)}
                    </button>
                  );
                })}
              </div>
            </div>
            <label className="text-xs font-medium text-slate-600">{t("stations.line")}
              <select value={filter.lineId} onChange={(event) => update({ lineId: event.target.value })} className="mt-1.5 block h-9 max-w-[12rem] rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800">
                <option value="">{t("stations.allLines")}</option>
                {lines.map((line) => <option key={line.id} value={line.id}>{line.name}</option>)}
              </select>
            </label>
            <Segmented label={t("stations.radius")} options={STATION_RADII.map((value) => [value, `${value} m`])} value={radius} onChange={setRadius} />
            <Segmented label={t("stations.period")} options={PERIODS.map((value) => [value, t(`stations.period${value}`)])} value={period} onChange={setPeriod} />
            <label className="flex h-9 items-center gap-2 self-end text-xs font-medium text-slate-600" title={t("stations.excludeNewHint")}>
              <Switch checked={excludeNew} onChange={setExcludeNew} />{t("stations.excludeNew")}
            </label>
          </div>
          {excludeNew && <p className="text-xs text-slate-400">{t("stations.excludeNewHint")}</p>}
          <div>
            <button onClick={() => setAdvancedOpen(!advancedOpen)} aria-expanded={advancedOpen} className="flex items-center gap-1 text-xs font-medium text-slate-500 hover:text-slate-800">
              {t("stations.advanced")}<ChevronDown className={cn("h-3.5 w-3.5 transition", advancedOpen && "rotate-180")} />
            </button>
            {advancedOpen && (
              <div className="mt-4 grid gap-5 lg:grid-cols-[1fr_1fr_1.6fr]">
                <RangeField label={t("filter.price")} unit="萬" value={[props.priceRange[0] / WAN, props.priceRange[1] / WAN]} onChange={([low, high]) => props.setPriceRange([low * WAN, high * WAN])} />
                <RangeField label={t("filter.area")} unit="坪" value={props.areaRange} onChange={props.setAreaRange} />
                <TypeChips label={t("filter.type")} selected={props.selectedTypes} onChange={props.setSelectedTypes} />
                <div className="flex items-center justify-between gap-4 lg:col-span-3">
                  <label className="flex items-center gap-3 text-xs text-slate-600"><Switch checked={props.includeSpecial} onChange={props.setIncludeSpecial} />{t("filter.special")}</label>
                  <button onClick={props.onReset} className="flex items-center gap-1 text-xs font-medium text-brand-700"><RotateCcw className="h-3.5 w-3.5" />{t("filter.reset")}</button>
                </div>
              </div>
            )}
          </div>
        </section>

        {error && (
          <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            <span className="flex items-center gap-2"><AlertCircle className="h-4 w-4" />{t("stations.error")}</span>
            <button onClick={() => setRetry((value) => value + 1)} className="font-semibold underline">{t("analysis.retry")}</button>
          </div>
        )}

        <section className={cn(CARD, "p-4 sm:p-5")}>
          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <h2 className="flex items-baseline gap-2 text-base font-semibold text-slate-900">
              <TrainFront className="h-4 w-4 self-center text-brand-600" />{t("stations.ranking")}
              <span className="text-xs font-normal text-slate-400">{fill(t("stations.rankingHint"), { radius })}</span>
            </h2>
            <Segmented label={t("stations.sortBy")} inline options={(["median", "yoy", "count"] as const).map((value) => [value, t(`stations.sort.${value}`)])} value={sort} onChange={setSort} />
          </div>
          {!ranking || stations.length === 0 ? <div className="h-80 animate-pulse rounded-xl bg-slate-100" /> : rows.length === 0
            ? <p className="py-10 text-center text-sm text-slate-500">{t("stations.noStations")}</p>
            : (
              <>
                <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
                  <table className="w-full min-w-[46rem] text-left text-sm">
                    <thead>
                      <tr className="border-b border-slate-100 text-xs text-slate-500">
                        <th className="w-10 py-2 font-medium">#</th>
                        <th className="py-2 font-medium">{t("stations.col.station")}</th>
                        <th className="py-2 font-medium">{t("stations.col.district")}</th>
                        <th className="py-2 text-right font-medium">{t("stations.sort.median")}</th>
                        <th className="py-2 text-right font-medium">{t("stations.sort.yoy")}</th>
                        <th className="py-2 text-right font-medium">{t("stations.sort.count")}</th>
                        <th className="py-2 pl-6 font-medium">{t("stations.col.trend")}</th>
                        <th className="w-14" />
                      </tr>
                    </thead>
                    <tbody>
                      {rows.slice(0, shown).map(({ station, rank }, index) => (
                        <tr key={station.id} className="border-b border-slate-100 last:border-0">
                          <td className="py-2.5 tabular-nums text-slate-400">{String(index + 1).padStart(2, "0")}</td>
                          <td className="py-2.5">
                            <div className="flex flex-wrap items-center gap-1.5">
                              <span className="mr-1 font-semibold text-slate-900">{station.name}</span>
                              {uniqueLines(station.lines).map((line) => (
                                <span key={line.lineId} className="flex items-center gap-1 rounded-full border border-slate-200 px-1.5 py-0.5 text-[11px] text-slate-600">
                                  <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: line.color }} />{line.name}
                                </span>
                              ))}
                            </div>
                          </td>
                          <td className="whitespace-nowrap py-2.5 text-slate-600">{CITY_NAMES[station.cityCode]} {station.district}</td>
                          <td className="whitespace-nowrap py-2.5 text-right font-semibold tabular-nums text-slate-900">{formatWan(rank.medianPricePerPing)} <span className="text-xs font-normal text-slate-400">萬</span></td>
                          <td className="py-2.5 text-right">{rank.priceYoY == null ? <span className="text-xs text-slate-400">—</span> : <YoYBadge value={rank.priceYoY} />}</td>
                          <td className="py-2.5 text-right tabular-nums text-slate-700">{rank.count.toLocaleString("zh-TW")}</td>
                          <td className="py-2.5 pl-6">
                            <svg width="96" height="24" aria-hidden className="overflow-visible">
                              <polyline points={sparkline(rank.trend)} fill="none" stroke="#1f6f78" strokeWidth="1.5" strokeLinejoin="round" />
                            </svg>
                          </td>
                          <td className="py-2.5 text-right">
                            <button onClick={() => props.onShowStation(station)} className="inline-flex items-center gap-0.5 text-xs font-medium text-brand-700 hover:text-brand-800">
                              {t("stations.onMap")}<ArrowUpRight className="h-3.5 w-3.5" />
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="mt-3 flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 pt-3 text-xs text-slate-500">
                  <span>{fill(t("stations.showing"), { shown: Math.min(shown, rows.length), total: rows.length })}</span>
                  {shown < rows.length && (
                    <button onClick={() => setShown((value) => value + 20)} className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50">{t("stations.showMore")}</button>
                  )}
                </div>
                <p className="mt-2 text-xs text-slate-400">{fill(t("stations.rankingNote"), { through: ranking.yoyThrough.replace("-", "/") || "—" })}</p>
              </>
            )}
        </section>

        <DistanceCard buckets={curve} cities={filter.city ? [filter.city] : Object.keys(CITY_NAMES)} />

        <section className={CARD}>
          <div className="border-b border-slate-100 px-4 py-4 sm:px-5">
            <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900"><TrendingUp className="h-4 w-4 text-brand-600" />{t("stations.openings")}</h2>
            <p className="mt-1 text-xs text-slate-500">{t("stations.openingsHint")}</p>
          </div>
          {!openings ? <div className="m-5 h-80 animate-pulse rounded-xl bg-slate-100" /> : events.length === 0
            ? <p className="py-10 text-center text-sm text-slate-500">{t("stations.noOpenings")}</p>
            : (
              <div className="grid lg:grid-cols-[2fr_3fr]">
                <div className="border-b border-slate-100 p-3 lg:max-h-[40rem] lg:overflow-y-auto lg:border-b-0 lg:border-r">
                  <h3 className="px-2 pb-2 text-xs text-slate-500">{t("stations.openingList")}（{events.length}）</h3>
                  <ul className="max-h-80 space-y-1 overflow-y-auto lg:max-h-none">
                    {events.map((event) => {
                      const key = `${event.stationId}|${event.lineId}`;
                      const isActive = active && key === `${active.stationId}|${active.lineId}`;
                      return (
                        <li key={key}>
                          <button onClick={() => setSelected(key)} aria-pressed={isActive}
                            className={cn("w-full rounded-xl border px-3 py-2.5 text-left transition", isActive ? "border-brand-600 bg-brand-50/60" : "border-transparent hover:bg-slate-50")}>
                            <div className="flex items-center justify-between gap-2">
                              <span className="flex min-w-0 items-center gap-2">
                                <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: event.lineColor }} />
                                <span className="truncate font-semibold text-slate-900">{event.stationName}</span>
                                <span className="truncate text-xs text-slate-500">{event.lineName}</span>
                              </span>
                              <RelativeTag event={event} />
                            </div>
                            <div className="mt-1 flex justify-between pl-4.5 text-xs text-slate-400">
                              <span>{fill(t("stations.opened"), { date: event.openedOn.slice(0, 7).replace("-", "/") })}</span>
                              <span>{CITY_NAMES[event.cityCode]} {event.district}</span>
                            </div>
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                </div>
                {active && <OpeningDetail event={active} radius={radius} station={byId.get(active.stationId)} onShowStation={props.onShowStation} />}
              </div>
            )}
        </section>
      </div>
    </main>
  );
}

function Segmented<T extends string | number>({ label, options, value, onChange, inline }: { label: string; options: [T, string][]; value: T; onChange: (value: T) => void; inline?: boolean }) {
  return (
    <div className={cn("text-xs font-medium text-slate-600", inline && "flex items-center gap-2")}>
      {label}
      <div className={cn("flex rounded-lg bg-slate-100 p-1", !inline && "mt-1.5")}>
        {options.map(([option, text]) => (
          <button key={option} onClick={() => onChange(option)} aria-pressed={value === option}
            className={cn("whitespace-nowrap rounded-md px-3 py-1 text-sm", value === option ? "bg-white font-medium text-brand-700 shadow-sm" : "text-slate-500 hover:text-slate-800")}>
            {text}
          </button>
        ))}
      </div>
    </div>
  );
}

function RelativeTag({ event }: { event: OpeningEffect }) {
  const { t } = useLanguage();
  if (event.status !== "ok" || event.relative == null) {
    return <span className="shrink-0 rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-500">{t(`stations.status.${event.status}`)}</span>;
  }
  const up = event.relative > 0;
  return (
    <span className={cn("shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold tabular-nums", up ? "bg-red-50 text-red-700" : "bg-green-50 text-green-700")}>
      {fill(t("stations.relative"), { value: `${points(event.relative)} pp` })}
    </span>
  );
}

function OpeningDetail({ event, radius, station, onShowStation }: { event: OpeningEffect; radius: StationRadius; station?: TransitStation; onShowStation: (station: TransitStation) => void }) {
  const { t } = useLanguage();
  const rows = useMemo(() => openingChartRows(event), [event]);
  const month = (offset: number) => rows.find((row) => row.offset === offset)?.month;
  const openedMonth = month(0);
  const price = (value: number | null) => value == null ? "—" : formatWan(value);

  return (
    <div className="space-y-4 p-4 sm:p-5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h3 className="text-lg font-semibold text-slate-900">{event.stationName} · {fill(t("stations.lineOpened"), { line: event.lineName })}</h3>
          <p className="mt-0.5 text-xs text-slate-500">{fill(t("stations.opened"), { date: event.openedOn.replaceAll("-", "/") })} · {CITY_NAMES[event.cityCode]} {event.district}</p>
        </div>
        {station && (
          <button onClick={() => onShowStation(station)} className="inline-flex items-center gap-0.5 text-xs font-medium text-brand-700 hover:text-brand-800">
            {t("stations.onMap")}<ArrowUpRight className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        {([[fill(t("stations.nearStation"), { radius }), event.station], [fill(t("stations.baseline"), { district: event.district }), event.baseline]] as const).map(([label, windows]) => (
          <div key={label} className="rounded-xl border border-slate-100 bg-slate-50/60 p-3">
            <div className="text-xs text-slate-500">{label}</div>
            <div className="mt-1 flex items-baseline gap-1 tabular-nums">
              <span className="text-xs text-slate-400">{price(windows.beforePrice)} →</span>
              <strong className="text-lg text-slate-900">{price(windows.afterPrice)}</strong>
              <span className="text-xs text-slate-500">{t("chart.unit")}</span>
            </div>
            <div className="mt-1 flex items-center gap-1.5 text-[11px] text-slate-400">
              {windows.change != null && <YoYBadge value={windows.change} />}
              {fill(t("stations.salesCounts"), { before: windows.beforeCount.toLocaleString("zh-TW"), after: windows.afterCount.toLocaleString("zh-TW") })}
            </div>
          </div>
        ))}
        <div className={cn("rounded-xl border p-3", event.relative == null ? "border-slate-100 bg-slate-50/60" : event.relative > 0 ? "border-red-100 bg-red-50/60" : "border-green-100 bg-green-50/60")}>
          <div className="text-xs text-slate-500">{t("stations.relativeTitle")}</div>
          {event.relative == null
            ? <div className="mt-1 text-sm text-slate-500">{t(`stations.status.${event.status}`)}</div>
            : (
              <div className={cn("mt-1 flex items-baseline gap-1", event.relative > 0 ? "text-red-700" : "text-green-700")}>
                <strong className="text-2xl tabular-nums">{points(event.relative)}</strong>
                <span className="text-xs">{t("stations.points")}</span>
              </div>
            )}
        </div>
      </div>

      <div>
        <h4 className="mb-1 text-xs font-medium text-slate-600">{t("stations.chart")}</h4>
        <div className="h-64 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={rows} margin={{ top: 16, right: 8, left: -16, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
              <XAxis dataKey="month" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} minTickGap={40} tickFormatter={(value: string) => value.replace("-", "/")} />
              <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["auto", "auto"]} tickFormatter={(value: number) => `${Math.round(value)}`} />
              <Tooltip
                contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12 }}
                formatter={(value, name) => [`${value} ${t("chart.unit")}`, name]}
              />
              <Legend verticalAlign="bottom" height={24} iconType="plainline" wrapperStyle={{ fontSize: 11 }} />
              {month(-12) && month(-1) && <ReferenceArea x1={month(-12)} x2={month(-1)} label={{ value: t("stations.yearBefore"), position: "insideTop", fontSize: 10, fill: "#64748b" }}
                shape={(area) => <rect x={area.x} y={area.y} width={area.width} height={area.height} fill="#94a3b8" fillOpacity={0.1} />} />}
              {openedMonth && month(11) && <ReferenceArea x1={openedMonth} x2={month(11)} label={{ value: t("stations.yearAfter"), position: "insideTop", fontSize: 10, fill: "#1f6f78" }}
                shape={(area) => <rect x={area.x} y={area.y} width={area.width} height={area.height} fill="#1f6f78" fillOpacity={0.08} />} />}
              {openedMonth && <ReferenceLine x={openedMonth} stroke={event.lineColor.toLowerCase() === "#fedb00" ? "#d4a800" : event.lineColor} strokeDasharray="4 3" />}
              <Line type="monotone" dataKey="station" name={fill(t("stations.chartStation"), { radius })} stroke="#1f6f78" strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
              <Line type="monotone" dataKey="baseline" name={t("stations.chartBaseline")} stroke="#94a3b8" strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      <p className="flex items-start gap-2 text-xs text-slate-400"><Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />{t("stations.causality")}</p>
    </div>
  );
}

function DistanceCard({ buckets, cities }: { buckets: DistanceBucket[] | null; cities: string[] }) {
  const { t } = useLanguage();
  const rows = useMemo(() => distanceRows(buckets ?? [], cities), [buckets, cities]);
  return (
    <section className={cn(CARD, "p-4 sm:p-5")}>
      <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900"><Ruler className="h-4 w-4 text-brand-600" />{t("stations.distance")}</h2>
      <p className="mt-1 text-xs text-slate-500">{t("stations.distanceHint")}</p>
      {!buckets ? <div className="mt-4 h-64 animate-pulse rounded-xl bg-slate-100" /> : (
        <div className="mt-4 grid gap-5 lg:grid-cols-[3fr_1fr]">
          <div className="h-64 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={rows} margin={{ top: 8, right: 8, left: -16, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
                <XAxis dataKey="label" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} />
                <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["auto", "auto"]} tickFormatter={(value: number) => `${Math.round(value)}`} />
                <Tooltip
                  contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12 }}
                  labelFormatter={(label) => `${label} m`}
                  formatter={(value, name, item) => {
                    const city = String(item.dataKey ?? "").replace("median.", "");
                    return [`${value} ${t("chart.unit")}（${(item.payload as { count: Record<string, number> }).count[city]?.toLocaleString("zh-TW") ?? 0} ${t("chart.count")}）`, name];
                  }}
                />
                <Legend verticalAlign="bottom" height={24} iconType="plainline" wrapperStyle={{ fontSize: 11 }} />
                {cities.map((city) => (
                  <Line key={city} dataKey={`median.${city}`} name={CITY_NAMES[city]} stroke={CITY_COLORS[city]} strokeWidth={2} dot={{ r: 3 }} connectNulls isAnimationActive={false} />
                ))}
              </LineChart>
            </ResponsiveContainer>
          </div>
          <div className="space-y-2">
            <h3 className="text-xs font-medium text-slate-600">{t("stations.premium")}</h3>
            {cities.map((city) => {
              const premium = nearStationPremium(buckets, city);
              return (
                <div key={city} className="flex items-center justify-between gap-2 rounded-xl border border-slate-100 px-3 py-2.5">
                  <span className="flex items-center gap-2 text-sm text-slate-700"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: CITY_COLORS[city] }} />{CITY_NAMES[city]}</span>
                  {premium == null
                    ? <span className="text-xs text-slate-400">{fill(t("stations.premiumSparse"), { count: MIN_PREMIUM_SALES })}</span>
                    : <strong className={cn("text-sm tabular-nums", premium > 0 ? "text-red-700" : "text-green-700")}>{premium > 0 ? "+" : ""}{(premium * 100).toFixed(1)}%</strong>}
                </div>
              );
            })}
            <p className="text-[11px] leading-relaxed text-slate-400">{t("stations.premiumNote")}</p>
          </div>
        </div>
      )}
    </section>
  );
}
