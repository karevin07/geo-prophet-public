import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { ArrowRight, TriangleAlert } from "lucide-react";
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type {
  CityFacilities, CommunityRanking, DistrictBoundaries, DistrictComparison, PresaleProject, RentalReport, RentTrend, StationRanking, TransactionAPI, TransitStation,
} from "../../lib/api";
import { CITY_NAMES } from "../../lib/data-status";
import { IS_DEMO } from "../../lib/demo";
import { FACILITY_CATEGORIES, GROUP_COLORS, groupOf } from "../../lib/facilities";
import { useLanguage } from "../../lib/i18n";
import { formatWan, priceBreaks, priceColor, recentMonths } from "../../lib/price-scale";
import { formatRent, formatYield, rentRows } from "../../lib/rent";
import type { AreaState } from "../../lib/url-state";
import { cn, fill } from "../../lib/utils";
import RentTrendChart from "../rents/RentTrendChart";
import YoYBadge from "../YoYBadge";

const ChoroplethMap = lazy(() => import("../ChoroplethMap"));

const CITY_CODES = ["a", "f", "h"] as const;
/** Residential types, as on the rents and investment pages. */
const HOME_TYPES = ["住宅大樓", "華廈", "公寓", "透天厝"] as const;
const BASE_QUERY = { priceRange: [0, 100_000_000] as [number, number], areaRange: [0, 200] as [number, number], types: [...HOME_TYPES], includeSpecial: false };
/** Categories compared per km²; nuisances are listed separately. */
const DENSITY_CATEGORIES = ["convenience", "supermarket", "market", "elementary", "junior_high", "clinic", "park", "library"];
const DISTRICT_COLOR = "#e8704f";
const CITY_COLOR = "#4fa3a5";

interface Props {
  api: TransactionAPI;
  state: AreaState;
  onChange: (update: Partial<AreaState>) => void;
  onShowProject: (project: { key: string; cityCode: string }) => void;
  onShowStation: (station: TransitStation) => void;
  onShowCommunity: (id: string) => void;
}

/** Loads a value for the given dependencies; a failure leaves it null. */
function useLoad<T>(load: (signal: AbortSignal) => Promise<T>, deps: unknown[]): T | null {
  const [value, setValue] = useState<T | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    setValue(null);
    load(controller.signal).then((result) => {
      if (!controller.signal.aborted) setValue(result);
    }).catch(() => undefined);
    return () => controller.abort();
  }, deps);
  return value;
}

function rankOf<T>(rows: T[], key: (row: T) => number | null, target: (row: T) => boolean): { rank: number; total: number } | null {
  const valued = rows.filter((row) => key(row) !== null).sort((a, b) => key(b)! - key(a)!);
  const index = valued.findIndex(target);
  return index < 0 ? null : { rank: index + 1, total: valued.length };
}

export default function AreaPage({ api, state, onChange, onShowProject, onShowStation, onShowCommunity }: Props) {
  const { t } = useLanguage();
  const city = state.city;
  const months = useMemo(() => recentMonths(new Date(), 12), []);
  const longMonths = useMemo(() => recentMonths(new Date(), 60), []);

  const regions = useLoad((signal) => api.regions(signal), [api]);
  const districts = regions?.find((region) => region.code === city)?.districts ?? [];
  const recent = useLoad<DistrictComparison>((signal) => api.compareDistricts({ ...BASE_QUERY, city, districts: [], from: months[0], to: months.at(-1)! }, signal), [api, city, months]);
  // Pick the busiest district when none is chosen.
  const district = state.district || (recent ? [...recent.ranking].sort((a, b) => b.count - a.count)[0]?.district ?? "" : "");

  // The comparison takes at most 4 districts, so the city's trend is read in chunks.
  const history = useLoad<DistrictComparison["trends"]>((signal) => {
    if (districts.length === 0) return Promise.reject(new Error("no districts"));
    const chunks = Array.from({ length: Math.ceil(districts.length / 4) }, (_, index) => districts.slice(index * 4, index * 4 + 4));
    return Promise.all(chunks.map((chunk) => api.compareDistricts({ ...BASE_QUERY, city, districts: chunk, from: longMonths[0], to: longMonths.at(-1)! }, signal)))
      .then((results) => results.flatMap((result) => result.trends));
  }, [api, city, districts.join(), longMonths]);
  const rentQuery = useMemo(() => ({ city, type: "", minArea: 0, maxArea: 0, includeSocial: false }), [city]);
  const rents = useLoad<RentalReport>((signal) => api.rentalDistricts(rentQuery, signal), [api, rentQuery]);
  const rentTrend = useLoad<RentTrend>((signal) => district ? api.rentalTrend(rentQuery, district, signal) : Promise.reject(new Error("no district")), [api, rentQuery, district]);
  const presales = useLoad<PresaleProject[]>((signal) => district ? api.presaleProjects({ city, district, months: 12 }, signal).then((result) => result.projects)
    : Promise.reject(new Error("no district")), [api, city, district]);
  // The demo's snapshot has no communities.
  const communities = useLoad<CommunityRanking>((signal) => district && !IS_DEMO ? api.communityRanking(city, district, signal)
    : Promise.reject(new Error("no district")), [api, city, district]);
  const facilities = useLoad<CityFacilities>((signal) => api.districtFacilities(city, signal), [api, city]);
  const boundaries = useLoad<DistrictBoundaries>((signal) => api.districtBoundaries(city, signal), [api, city]);
  const stations = useLoad<TransitStation[]>((signal) => api.transitStations(signal), [api]);
  const stationRanking = useLoad<StationRanking>((signal) => api.stationRanking(BASE_QUERY, 500, months[0], signal), [api, months]);

  const rank = recent?.ranking.find((row) => row.district === district);
  const rentRow = rents?.districts.find((row) => row.district === district);
  const breaks = useMemo(() => priceBreaks(recent?.ranking.map((row) => row.medianPricePerPing) ?? []), [recent]);

  // The city's monthly price is the sales-weighted average of its districts'.
  const priceRows = useMemo(() => {
    if (!history) return [];
    const byMonth = new Map<string, { month: string; weighted: number; count: number; district: number | null; districtCount: number; provisional: boolean }>();
    for (const trend of history) {
      const row = byMonth.get(trend.month) ?? { month: trend.month, weighted: 0, count: 0, district: null, districtCount: 0, provisional: trend.provisional };
      row.weighted += trend.avgPricePerPing * trend.count;
      row.count += trend.count;
      if (trend.district === district) {
        row.district = trend.avgPricePerPing / 10_000;
        row.districtCount = trend.count;
      }
      byMonth.set(trend.month, row);
    }
    return [...byMonth.values()].sort((a, b) => a.month.localeCompare(b.month))
      // Months still being registered are left off the price lines: their few early
      // registrations skew the average.
      .map((row) => ({ month: row.month, city: row.count > 0 && !row.provisional ? row.weighted / row.count / 10_000 : null,
        district: row.provisional ? null : row.district, count: row.districtCount, provisional: row.provisional }));
  }, [history, district]);

  const presaleSummary = useMemo(() => {
    const sold = (presales ?? []).filter((project) => project.recentSales > 0 && project.medianPricePerPing !== null);
    const total = sold.reduce((sum, project) => sum + project.recentSales, 0);
    if (total === 0) return null;
    // Median of project medians weighted by their recent sales.
    const sorted = [...sold].sort((a, b) => a.medianPricePerPing! - b.medianPricePerPing!);
    let cumulative = 0;
    const median = sorted.find((project) => (cumulative += project.recentSales) >= total / 2)!.medianPricePerPing!;
    return { median, projects: sold.length, sales: total };
  }, [presales]);

  const districtFacilities = facilities?.districts.find((row) => row.district === district);
  // The demo has no stations, which is not the same as a district without any.
  const noStations = t(IS_DEMO ? "demo.noStations" : "area.kpi.noStations");
  const districtStations = useMemo(() => {
    const ranks = new Map(stationRanking?.stations.map((station) => [station.stationId, station]) ?? []);
    return (stations ?? []).filter((station) => station.cityCode === city && station.district === district)
      .map((station) => ({ station, rank: ranks.get(station.id) }))
      .sort((a, b) => (b.rank?.count ?? 0) - (a.rank?.count ?? 0));
  }, [stations, stationRanking, city, district]);

  const fillOf = useCallback((name: string) => {
    const row = recent?.ranking.find((candidate) => candidate.district === name);
    return row ? priceColor(row.medianPricePerPing, breaks, 110) : null;
  }, [recent, breaks]);

  const priceRank = recent && rank ? rankOf(recent.ranking, (row) => row.medianPricePerPing, (row) => row.district === district) : null;
  const rentRank = rents && rentRow ? rankOf(rents.districts, (row) => row.medianRentPerPing, (row) => row.district === district) : null;
  const card = "rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]";
  const select = "h-9 rounded-lg border border-slate-200 bg-white px-2 text-sm text-slate-800";

  return <main className="h-full overflow-y-auto bg-canvas">
    <div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
      <header className={cn(card, "flex flex-wrap items-start justify-between gap-5")}>
        <div className="min-w-0">
          <p className="text-xs font-semibold tracking-wide text-brand-700">GEOPROPHET · {t("nav.area")}</p>
          <div className="mt-1 flex flex-wrap items-center gap-3">
            <h1 className="text-3xl font-semibold text-slate-900">{CITY_NAMES[city]} {district}</h1>
            <select value={city} onChange={(event) => onChange({ city: event.target.value as AreaState["city"], district: "" })} className={select} aria-label={t("rents.city")}>
              {CITY_CODES.map((code) => <option key={code} value={code}>{t(`investment.city.${code}`)}</option>)}
            </select>
            <select value={district} onChange={(event) => onChange({ district: event.target.value })} className={select} aria-label={t("presales.district")}>
              {districts.map((name) => <option key={name} value={name}>{name}</option>)}
            </select>
          </div>
          <p className="mt-2 text-xs text-slate-500">{fill(t("area.period"), { from: months[0], through: recent?.yoyThrough || months.at(-1)! })}</p>
          <p className="mt-1 max-w-2xl text-sm text-slate-600">{t("area.subtitle")}</p>
        </div>
        <div className="h-56 w-full sm:w-96">
          <Suspense fallback={<div className="h-full animate-pulse rounded-xl bg-slate-100" />}>
            <ChoroplethMap boundaries={boundaries} fill={fillOf} fillKey={`${breaks?.join()}|${recent?.ranking.length}`}
              selected={district ? [{ district, color: "#1f6f78" }] : []} onSelect={(name) => onChange({ district: name })}
              hoverContent={() => null} legend={<span>{t("area.mapHint")}</span>} className="h-full sm:h-full" />
          </Suspense>
        </div>
      </header>

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-5">
        <Kpi label={t("area.kpi.price")} rank={priceRank} value={rank ? formatWan(rank.medianPricePerPing) : "—"} unit={t("chart.unit")}
          note={rank && <span className="flex items-center gap-1.5">{fill(t("area.kpi.sales"), { count: rank.count.toLocaleString() })}<YoYBadge value={rank.priceYoY} suffix={t("yoy.label")} /></span>} />
        <Kpi label={t("area.kpi.rent")} rank={rentRank} value={formatRent(rentRow?.medianRentPerPing ?? null)} unit={t("rents.unit.perPing")}
          note={fill(t("area.kpi.yield"), { yield: formatYield(rentRow?.grossYield ?? null), city: formatYield(rents?.city.grossYield ?? null) })} />
        <Kpi label={t("area.kpi.presale")} value={presaleSummary ? formatWan(presaleSummary.median) : "—"} unit={t("chart.unit")}
          badge={presaleSummary && rank ? <PremiumBadge value={presaleSummary.median / rank.medianPricePerPing - 1} /> : null}
          note={presaleSummary ? fill(t("area.kpi.presaleNote"), { projects: presaleSummary.projects, sales: presaleSummary.sales }) : t("area.kpi.noPresale")} />
        <Kpi label={t("area.kpi.facilities")} value={districtFacilities ? ((districtFacilities.counts.convenience ?? 0) / districtFacilities.areaKm2).toFixed(1) : "—"} unit={t("area.kpi.perKm2")}
          note={districtFacilities && facilities ? fill(t("area.kpi.facilitiesNote"), { city: ((facilities.city.counts.convenience ?? 0) / facilities.city.areaKm2).toFixed(1), parks: districtFacilities.counts.park ?? 0 }) : ""} />
        <Kpi label={t("area.kpi.stations")} value={stations && !IS_DEMO ? String(districtStations.length) : "—"} unit={t("area.kpi.stationUnit")}
          note={districtStations.length > 0 ? districtStations.slice(0, 3).map((item) => item.station.name).join("、") : noStations} />
      </div>

      <div className="grid gap-5 lg:grid-cols-2">
        <div className="space-y-5">
          <section className={card}>
            <CardTitle title={t("area.price.title")} hint={t("area.price.hint")}
              legend={[[district, DISTRICT_COLOR], [CITY_NAMES[city], CITY_COLOR]]} />
            <div className="mt-3 h-64">
              {priceRows.length === 0 ? <div className="h-full animate-pulse rounded-xl bg-slate-100" /> :
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={priceRows} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
                    <XAxis dataKey="month" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false}
                      ticks={priceRows.map((row) => row.month).filter((month) => month.endsWith("-01"))} tickFormatter={(month: string) => month.slice(0, 4)} />
                    <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["auto", "auto"]} tickFormatter={(value: number) => value.toFixed(0)} />
                    <Tooltip contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12 }}
                      formatter={(value, name) => [`${Number(value).toFixed(1)} ${t("chart.unit")}`, name === "district" ? district : CITY_NAMES[city]]} />
                    <Line type="monotone" dataKey="city" stroke={CITY_COLOR} strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
                    <Line type="monotone" dataKey="district" stroke={DISTRICT_COLOR} strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
                  </LineChart>
                </ResponsiveContainer>}
            </div>
            <p className="mt-2 text-[11px] text-slate-400">{t("area.price.note")}</p>
          </section>

          <section className={card}>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div><h2 className="text-base font-semibold text-slate-900">{t("area.rent.title")}</h2><p className="mt-1 text-xs text-slate-500">{t("area.rent.hint")}</p></div>
              <div className="text-right"><p className="text-xs text-slate-500">{t("rents.kpi.yield")}</p><p className="text-xl font-semibold tabular-nums text-brand-700">{formatYield(rentRow?.grossYield ?? null)}</p></div>
            </div>
            {rentTrend ? <RentTrendChart rows={rentRows(rentTrend.city, rentTrend.districtPoints)} cityLabel={CITY_NAMES[city]} districtLabel={district} className="mt-3 h-56" />
              : <div className="mt-3 h-56 animate-pulse rounded-xl bg-slate-100" />}
            <p className="mt-2 text-[11px] text-slate-400">{t("rents.trendSocial")}</p>
          </section>

          <section className={card}>
            <CardTitle title={t("area.volume.title")} hint={t("area.volume.hint")} />
            <div className="mt-3 h-44">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={priceRows.slice(-24)} margin={{ top: 8, right: 0, left: -18, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
                  <XAxis dataKey="month" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} minTickGap={24} />
                  <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} allowDecimals={false} />
                  <Tooltip contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12 }}
                    formatter={(value) => [`${value} ${t("estimate.unit.sales")}`, t("map.count")]} />
                  <Bar dataKey="count" fill="#2f4b7c" radius={[2, 2, 0, 0]} isAnimationActive={false} />
                </BarChart>
              </ResponsiveContainer>
            </div>
            <p className="mt-2 text-[11px] text-slate-400">{t("area.volume.note")}</p>
          </section>
        </div>

        <div className="space-y-5">
          <section className={card}>
            <div className="flex items-start justify-between gap-3">
              <div><h2 className="text-base font-semibold text-slate-900">{t("area.presale.title")}</h2><p className="mt-1 text-xs text-slate-500">{t("area.presale.hint")}</p></div>
            </div>
            {presales === null ? <div className="mt-3 h-40 animate-pulse rounded-xl bg-slate-100" /> : presales.length === 0
              ? <p className="mt-3 text-sm text-slate-500">{t("area.kpi.noPresale")}</p>
              : <ul className="mt-3 divide-y divide-slate-100">
                {[...presales].sort((a, b) => b.recentSales - a.recentSales).slice(0, 5).map((project) => (
                  <li key={project.key}>
                    <button type="button" onClick={() => onShowProject(project)} className="flex w-full items-center justify-between gap-3 py-2.5 text-left hover:bg-slate-50">
                      <span className="min-w-0">
                        <span className="block truncate font-semibold text-slate-900">{project.name}</span>
                        <span className="text-xs text-slate-500">{fill(t("presales.recent"), { count: project.recentSales })} · {t(`presales.precision.${project.geocodePrecision || "district"}`)}</span>
                      </span>
                      <span className="shrink-0 text-sm font-semibold tabular-nums text-slate-900">{project.medianPricePerPing === null ? "—" : `${formatWan(project.medianPricePerPing)} ${t("chart.unit")}`}</span>
                    </button>
                  </li>
                ))}
              </ul>}
          </section>

          {!IS_DEMO && <section className={card}>
            <CardTitle title={t("area.communities.title")} hint={t("area.communities.hint")} />
            {communities === null ? <div className="mt-3 h-40 animate-pulse rounded-xl bg-slate-100" /> : communities.communities.length === 0
              ? <p className="mt-3 text-sm text-slate-500">{t("area.communities.empty")}</p>
              : <ul className="mt-3 divide-y divide-slate-100">
                {communities.communities.slice(0, 6).map((community) => (
                  <li key={community.id}>
                    <button type="button" onClick={() => onShowCommunity(community.id)} className="flex w-full items-center justify-between gap-3 py-2.5 text-left hover:bg-slate-50">
                      <span className="min-w-0">
                        <span className="block truncate font-semibold text-slate-900">{community.name || community.address}</span>
                        <span className="text-xs text-slate-500">{[community.name && community.address, community.category,
                          fill(t("area.communities.meta"), { year: community.completedOn.slice(0, 4), count: community.recentSales })].filter(Boolean).join(" · ")}</span>
                      </span>
                      <span className="shrink-0 text-sm font-semibold tabular-nums text-slate-900">{community.medianPricePerPing === null ? "—" : `${formatWan(community.medianPricePerPing)} ${t("chart.unit")}`}</span>
                    </button>
                  </li>
                ))}
              </ul>}
          </section>}

          <section className={card}>
            <CardTitle title={t("area.facilities.title")} hint={t("area.facilities.hint")} legend={[[district, "#1f6f78"], [t("area.facilities.cityAverage"), "#cbd5e1"]]} />
            {!districtFacilities || !facilities ? <div className="mt-3 h-48 animate-pulse rounded-xl bg-slate-100" /> : <>
              <ul className="mt-3 space-y-2.5">
                {DENSITY_CATEGORIES.map((category) => {
                  const own = (districtFacilities.counts[category] ?? 0) / districtFacilities.areaKm2;
                  const average = (facilities.city.counts[category] ?? 0) / facilities.city.areaKm2;
                  const max = Math.max(own, average, 0.01);
                  const group = groupOf(category) ?? "daily";
                  return <li key={category}>
                    <div className="flex items-baseline justify-between gap-2 text-xs">
                      <span className="flex items-center gap-1.5 text-slate-600"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: GROUP_COLORS[group] }} />{t(`facilities.category.${category}`)}</span>
                      <span className="tabular-nums text-slate-500"><b className="text-slate-900">{own.toFixed(1)}</b> vs {average.toFixed(1)} {t("area.kpi.perKm2")}</span>
                    </div>
                    <div className="mt-1 grid grid-cols-1 gap-0.5">
                      <span className="h-1.5 rounded-full bg-brand-600" style={{ width: `${own / max * 100}%` }} />
                      <span className="h-1.5 rounded-full bg-slate-300" style={{ width: `${average / max * 100}%` }} />
                    </div>
                  </li>;
                })}
              </ul>
              <p className="mt-3 flex items-start gap-1.5 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-900">
                <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                {FACILITY_CATEGORIES.nuisance.map((category) => `${t(`facilities.category.${category}`)} ${districtFacilities.counts[category] ?? 0}`).join(" · ")}
              </p>
              <p className="mt-2 text-[11px] text-slate-400">{fill(t("area.facilities.note"), { area: districtFacilities.areaKm2 })} · {t("facilities.attribution")}</p>
            </>}
          </section>

          <section className={card}>
            <CardTitle title={t("area.stations.title")} hint={t("area.stations.hint")} />
            {stations === null ? <div className="mt-3 h-32 animate-pulse rounded-xl bg-slate-100" /> : districtStations.length === 0
              ? <p className="mt-3 text-sm text-slate-500">{noStations}</p>
              : <ul className="mt-2 divide-y divide-slate-100">
                {districtStations.map(({ station, rank: stationRank }) => (
                  <li key={station.id}>
                    <button type="button" onClick={() => onShowStation(station)} className="flex w-full items-center justify-between gap-3 py-2.5 text-left hover:bg-slate-50">
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="flex gap-0.5">{[...new Map(station.lines.map((line) => [line.lineId, line.color])).values()].slice(0, 3).map((color) =>
                          <span key={color} className="h-3 w-1.5 rounded-full" style={{ backgroundColor: color }} />)}</span>
                        <span className="truncate font-medium text-slate-900">{station.name}</span>
                      </span>
                      <span className="flex shrink-0 items-center gap-2">
                        <span className="text-sm font-semibold tabular-nums text-slate-900">{stationRank && stationRank.count > 0 ? `${formatWan(stationRank.medianPricePerPing)} ${t("chart.unit")}` : "—"}</span>
                        <YoYBadge value={stationRank?.priceYoY} />
                        <ArrowRight className="h-3.5 w-3.5 text-slate-400" />
                      </span>
                    </button>
                  </li>
                ))}
              </ul>}
          </section>
        </div>
      </div>

      <aside className={cn(card, "text-xs leading-6 text-slate-600")}>
        <h2 className="text-sm font-semibold text-slate-900">{t("area.methodTitle")}</h2>
        <ul className="mt-2 list-disc space-y-1 pl-4">
          <li>{t("area.method.price")}</li>
          <li>{t("area.method.rent")}</li>
          <li>{t("area.method.presale")}</li>
          <li>{t("area.method.facilities")}</li>
        </ul>
      </aside>
    </div>
  </main>;
}

function Kpi({ label, value, unit, note, rank, badge }: {
  label: string; value: string; unit: string; note: React.ReactNode; rank?: { rank: number; total: number } | null; badge?: React.ReactNode;
}) {
  const { t } = useLanguage();
  return <div className="min-w-0 rounded-2xl border border-slate-200 bg-white p-3 shadow-[0_4px_16px_rgba(15,23,42,0.04)] sm:p-4">
    <div className="flex flex-wrap items-start justify-between gap-x-2 gap-y-1">
      <p className="text-xs font-medium text-slate-500">{label}</p>
      {rank && <span className="shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-600">{fill(t("area.rank"), rank)}</span>}
      {badge}
    </div>
    <p className="mt-2 flex items-baseline gap-1"><span className="text-xl font-semibold tabular-nums text-slate-900 sm:text-2xl">{value}</span><span className="text-xs text-slate-500">{unit}</span></p>
    <div className="mt-1 text-xs text-slate-500">{note}</div>
  </div>;
}

function PremiumBadge({ value }: { value: number }) {
  const { t } = useLanguage();
  return <span className={cn("shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold", value >= 0 ? "bg-red-50 text-red-700" : "bg-green-50 text-green-700")}>
    {fill(t("area.kpi.premium"), { value: `${value >= 0 ? "+" : ""}${Math.round(value * 100)}%` })}
  </span>;
}

function CardTitle({ title, hint, legend }: { title: string; hint: string; legend?: [string, string][] }) {
  return <div className="flex flex-wrap items-start justify-between gap-3">
    <div><h2 className="text-base font-semibold text-slate-900">{title}</h2><p className="mt-1 text-xs text-slate-500">{hint}</p></div>
    {legend && <div className="flex items-center gap-3 text-xs text-slate-600">
      {legend.map(([name, color]) => <span key={name} className="flex items-center gap-1"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: color }} />{name}</span>)}
    </div>}
  </div>;
}
