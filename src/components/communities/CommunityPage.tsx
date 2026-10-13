import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { ArrowRight, Calculator, ChevronLeft, ChevronRight, Info, MapPin, TrendingUp } from "lucide-react";
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { CommunityDetail, TransactionAPI } from "../../lib/api";
import { CITY_NAMES } from "../../lib/data-status";
import { useLanguage } from "../../lib/i18n";
import { formatWan } from "../../lib/price-scale";
import { formatRent, formatYield } from "../../lib/rent";
import { cn, fill } from "../../lib/utils";
import { layoutText } from "../feed/FeedTable";
import { Switch } from "../ui";
import YoYBadge from "../YoYBadge";
import { buildingAge, doorAddress, fiveYearsBefore, floorBands, visibleSales, yearRows, type FloorBand } from "./community-data";

const CommunityMap = lazy(() => import("./CommunityMap"));

const COMMUNITY_COLOR = "#e8704f";
const DISTRICT_COLOR = "#1f6f78";
const BAND_COLORS: Record<FloorBand["key"], string> = { high: "#e8704f", mid: "#f2c85b", low: "#4fa3a5", ground: "#94a3b8" };
const PAGE_SIZE = 10;
const card = "rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.04)]";

/** What the estimate form needs to value a home in the community. */
export interface CommunityHome {
  address: string;
  lat: number;
  lng: number;
  type: string;
  age: number | null;
  totalFloors: number;
}

interface Props {
  api: TransactionAPI;
  id: string;
  onShowCommunity: (id: string) => void;
  onShowOnMap: (lat: number, lng: number) => void;
  onEstimate: (home: CommunityHome) => void;
}

export default function CommunityPage({ api, id, onShowCommunity, onShowOnMap, onEstimate }: Props) {
  const { t } = useLanguage();
  const [detail, setDetail] = useState<CommunityDetail | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    setDetail(null);
    setError(false);
    api.community(id, controller.signal).then((value) => {
      if (!controller.signal.aborted) setDetail(value);
    }).catch(() => {
      if (!controller.signal.aborted) setError(true);
    });
    return () => controller.abort();
  }, [api, id]);

  if (error) {
    return <main className="h-full overflow-y-auto bg-canvas"><div className="mx-auto max-w-[1360px] px-4 py-7 sm:px-6">
      <p className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">{t("community.error")}</p>
    </div></main>;
  }
  if (!detail) {
    return <main className="h-full overflow-y-auto bg-canvas" aria-busy><div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
      <div className="h-44 animate-pulse rounded-2xl bg-white" />
      <div className="grid grid-cols-2 gap-3 xl:grid-cols-5">{Array.from({ length: 5 }, (_, index) => <div key={index} className="h-28 animate-pulse rounded-2xl bg-white" />)}</div>
      <div className="h-80 animate-pulse rounded-2xl bg-white" />
    </div></main>;
  }
  return <CommunityView detail={detail} onShowCommunity={onShowCommunity} onShowOnMap={onShowOnMap} onEstimate={onEstimate} />;
}

function CommunityView({ detail, onShowCommunity, onShowOnMap, onEstimate }: { detail: CommunityDetail } & Omit<Props, "api" | "id">) {
  const { t } = useLanguage();
  const { community, rent } = detail;
  const age = buildingAge(community.completedOn);
  const located = community.lat !== null && community.lng !== null;
  const years = useMemo(() => yearRows(detail.years), [detail.years]);
  const bands = useMemo(() => floorBands(detail.sales, community.totalFloors, fiveYearsBefore(detail.through)), [detail, community.totalFloors]);
  const latestDoor = detail.sales[0] ? doorAddress(detail.sales[0].address) : "";

  const subtitle = [
    `${CITY_NAMES[community.cityCode] ?? ""}${community.district}`, community.category,
    fill(t(age === null ? "community.completedOn" : "community.completed"), { year: community.completedOn.slice(0, 4), age: age ?? 0 }),
    community.totalFloors && fill(t("community.floors"), { floors: community.totalFloors }),
    fill(t("community.doors"), { count: community.doorNumbers }),
  ].filter(Boolean).join(" · ");

  return <main className="h-full overflow-y-auto bg-canvas">
    <div className="mx-auto max-w-[1360px] space-y-5 px-4 py-5 sm:px-6 sm:py-7">
      <header className={cn(card, "flex flex-wrap items-start justify-between gap-5")}>
        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold tracking-wide text-brand-700">GEOPROPHET · {t("community.title")}</p>
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-2">
            <h1 className="text-2xl font-semibold text-slate-900 sm:text-3xl">{community.name || community.address}</h1>
            <span title={t(community.name ? "community.namedHint" : "community.unnamedHint")}
              className="inline-flex items-center gap-1 rounded-full border border-brand-100 bg-brand-50 px-2.5 py-1 text-xs font-medium text-brand-700">
              <Info className="h-3.5 w-3.5" />{t(community.name ? "community.named" : "community.unnamed")}
            </span>
          </div>
          {community.name && <p className="mt-1 text-sm text-slate-600">{community.address}</p>}
          <p className="mt-2 text-sm text-slate-500">{subtitle}</p>
          <div className="mt-4 flex flex-wrap gap-2">
            {located && <button type="button" onClick={() => onShowOnMap(community.lat!, community.lng!)}
              className="flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
              <MapPin className="h-4 w-4" />{t("community.showOnMap")}
            </button>}
            {located && latestDoor && <button type="button"
              onClick={() => onEstimate({ address: latestDoor, lat: community.lat!, lng: community.lng!, type: community.category, age, totalFloors: community.totalFloors ?? 0 })}
              className="flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-2 text-sm font-medium text-white hover:bg-brand-700">
              <Calculator className="h-4 w-4" />{t("community.estimate")}
            </button>}
          </div>
        </div>
        {located && <div className="h-44 w-full sm:w-80">
          <Suspense fallback={<div className="h-full animate-pulse rounded-xl bg-slate-100" />}>
            <CommunityMap community={{ lat: community.lat!, lng: community.lng! }} nearby={detail.nearby} onSelect={onShowCommunity} />
          </Suspense>
        </div>}
      </header>

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-5">
        <Kpi label={t("community.kpi.price")} value={detail.medianPricePerPing === null ? "—" : formatWan(detail.medianPricePerPing)} unit={t("chart.unit")}
          badge={<YoYBadge value={detail.priceYoY} suffix={t("yoy.label")} />}
          note={detail.medianPricePerPing === null ? t("community.kpi.fewSales") : fill(t("community.kpi.priceNote"), { from: detail.from, through: detail.through })} />
        <Kpi label={t("community.kpi.premium")} value={detail.premium === null ? "—" : `${detail.premium >= 0 ? "+" : ""}${Math.round(detail.premium * 100)}%`} unit=""
          valueClass={detail.premium === null ? undefined : detail.premium >= 0 ? "text-red-600" : "text-green-600"}
          note={detail.districtPricePerPing === null ? "" : fill(t("community.kpi.premiumNote"), { district: community.district, category: community.category, price: formatWan(detail.districtPricePerPing) })} />
        <Kpi label={t("community.kpi.sales")} value={detail.recentSales.toLocaleString()} unit={t("estimate.unit.sales")}
          note={fill(t("community.kpi.salesNote"), { count: community.sales.toLocaleString() })} />
        <Kpi label={t("community.kpi.rent")} value={formatRent(rent?.medianRentPerPing ?? null)} unit={t("rents.unit.perPing")}
          note={rent?.medianRentPerPing != null
            ? fill(t("community.kpi.rentNote"), { yield: formatYield(rent.grossYield), count: rent.leases })
            : t("community.kpi.noRent")} />
        <Kpi label={t("community.kpi.layout")} value={detail.medianAreaPing === null ? "—" : detail.medianAreaPing.toFixed(1)} unit={t("community.unit.ping")}
          note={detail.mainRooms !== null && detail.mainRoomsShare !== null
            ? fill(t("community.kpi.layoutNote"), { rooms: detail.mainRooms, share: `${Math.round(detail.mainRoomsShare * 100)}%` }) : ""} />
      </div>

      <div className="grid gap-5 lg:grid-cols-5">
        <section className={cn(card, "lg:col-span-3")}>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-slate-900">{t("community.trend.title")}</h2>
              <p className="mt-1 text-xs text-slate-500">{fill(t("community.trend.hint"), { district: community.district, category: community.category })}</p>
            </div>
            <div className="flex items-center gap-3 text-xs text-slate-600">
              <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: COMMUNITY_COLOR }} />{t("community.trend.community")}</span>
              <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: DISTRICT_COLOR }} />{community.district}</span>
              <span className="flex items-center gap-1"><span className="w-4 border-t-2 border-dashed border-slate-400" />{t("community.trend.provisional")}</span>
            </div>
          </div>
          <div className="mt-3 h-64">
            {years.length === 0 ? <div className="flex h-full items-center justify-center text-xs text-slate-400">{t("chart.empty")}</div> :
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={years} margin={{ top: 8, right: 8, left: -12, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
                  {/* A numeric axis keeps years without sales in their place. */}
                  <XAxis dataKey="year" type="number" domain={["dataMin", "dataMax"]} allowDecimals={false} stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} minTickGap={16} />
                  <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["auto", "auto"]} tickFormatter={(value: number) => value.toFixed(0)} />
                  <Tooltip contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12 }}
                    labelFormatter={(year, payload) => `${year}（${fill(t("community.trend.sales"), { count: payload?.[0]?.payload.sales ?? 0 })}）`}
                    formatter={(value, name) => [`${Number(value).toFixed(1)} ${t("chart.unit")}`,
                      String(name).startsWith("community") ? t("community.trend.community") : community.district]} />
                  <Line type="monotone" dataKey="district" stroke={DISTRICT_COLOR} strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
                  <Line type="monotone" dataKey="districtTail" stroke={DISTRICT_COLOR} strokeWidth={2} strokeDasharray="4 4" dot={false} connectNulls isAnimationActive={false} legendType="none" />
                  <Line type="monotone" dataKey="community" stroke={COMMUNITY_COLOR} strokeWidth={2.5} dot={{ r: 2.5 }} connectNulls isAnimationActive={false} />
                  <Line type="monotone" dataKey="communityTail" stroke={COMMUNITY_COLOR} strokeWidth={2.5} strokeDasharray="4 4" dot={{ r: 2.5 }} connectNulls isAnimationActive={false} legendType="none" />
                </LineChart>
              </ResponsiveContainer>}
          </div>
          <p className="mt-2 text-[11px] text-slate-400">{t("community.trend.note")}</p>
        </section>

        <section className={cn(card, "lg:col-span-2")}>
          <h2 className="text-base font-semibold text-slate-900">{t("community.floors.title")}</h2>
          <p className="mt-1 text-xs text-slate-500">{t("community.floors.hint")}</p>
          <div className="mt-3 flex items-start gap-2 rounded-xl border border-brand-100 bg-brand-50 px-3 py-2.5 text-sm text-brand-800">
            <TrendingUp className="mt-0.5 h-4 w-4 shrink-0" />
            {detail.floorPremium === null ? <span className="text-xs text-slate-600">{t("community.floors.noPremium")}</span>
              : <span className="font-medium">{fill(t("community.floors.premium"), { value: `${detail.floorPremium >= 0 ? "+" : ""}${(detail.floorPremium * 100).toFixed(1)}%` })}</span>}
          </div>
          <FloorBands bands={bands} />
        </section>
      </div>

      <div className="grid gap-5 lg:grid-cols-3">
        <SalesTable detail={detail} />
        <section className={card}>
          <h2 className="text-base font-semibold text-slate-900">{t("community.nearby.title")}</h2>
          <p className="mt-1 text-xs text-slate-500">{t("community.nearby.hint")}</p>
          {detail.nearby.length === 0 ? <p className="mt-4 text-sm text-slate-500">{t("community.nearby.empty")}</p> :
            <ul className="mt-3 space-y-2.5">
              {detail.nearby.map((item) => <li key={item.id}>
                <button type="button" onClick={() => onShowCommunity(item.id)}
                  className="w-full rounded-xl border border-slate-200 px-3.5 py-3 text-left transition hover:border-brand-200 hover:bg-slate-50">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <p className="truncate font-semibold text-slate-900">{item.name || item.address}</p>
                      {item.name && <p className="truncate text-xs text-slate-500">{item.address}</p>}
                    </div>
                    <div className="shrink-0 text-right">
                      <p className="text-sm font-semibold tabular-nums text-slate-900">{item.medianPricePerPing === null ? "—" : `${formatWan(item.medianPricePerPing)} ${t("chart.unit")}`}</p>
                      <YoYBadge value={item.difference} />
                    </div>
                  </div>
                  <div className="mt-2 flex items-center justify-between gap-2 border-t border-slate-100 pt-2 text-xs text-slate-500">
                    <span>{fill(t("community.nearby.meta"), { year: item.completedOn.slice(0, 4), distance: item.distanceM, count: item.recentSales })}</span>
                    <span className="flex shrink-0 items-center gap-0.5 font-medium text-brand-700">{t("community.link")}<ArrowRight className="h-3.5 w-3.5" /></span>
                  </div>
                </button>
              </li>)}
            </ul>}
          {detail.nearby.length > 0 && <p className="mt-3 text-[11px] text-slate-400">{fill(t("community.nearby.note"), { price: detail.medianPricePerPing === null ? "—" : formatWan(detail.medianPricePerPing) })}</p>}
        </section>
      </div>

      <p className="px-1 text-center text-[11px] leading-5 text-slate-400">{t("community.footer")}</p>
    </div>
  </main>;
}

function Kpi({ label, value, unit, note, badge, valueClass }: { label: string; value: string; unit: string; note: React.ReactNode; badge?: React.ReactNode; valueClass?: string }) {
  return <div className="min-w-0 rounded-2xl border border-slate-200 bg-white p-3 shadow-[0_4px_16px_rgba(15,23,42,0.04)] sm:p-4">
    <div className="flex flex-wrap items-start justify-between gap-x-2 gap-y-1">
      <p className="text-xs font-medium text-slate-500">{label}</p>
      {badge}
    </div>
    <p className="mt-2 flex items-baseline gap-1"><span className={cn("text-xl font-semibold tabular-nums text-slate-900 sm:text-2xl", valueClass)}>{value}</span><span className="text-xs text-slate-500">{unit}</span></p>
    <div className="mt-1 text-xs text-slate-500">{note}</div>
  </div>;
}

function FloorBands({ bands }: { bands: FloorBand[] }) {
  const { t } = useLanguage();
  if (bands.length === 0) return <p className="mt-4 text-sm text-slate-500">{t("community.floors.empty")}</p>;
  const scaleLow = Math.min(...bands.map((band) => band.low));
  const scaleHigh = Math.max(...bands.map((band) => band.high));
  const span = Math.max(scaleHigh - scaleLow, 1);
  const position = (value: number) => `${((value - scaleLow) / span) * 100}%`;
  return <ul className="mt-4 space-y-4">
    {bands.map((band) => <li key={band.key}>
      <div className="flex items-baseline justify-between gap-2 text-sm">
        <span className="flex items-center gap-1.5 font-medium text-slate-800">
          <span className="h-2 w-2 rounded-full" style={{ backgroundColor: BAND_COLORS[band.key] }} />
          {t(`community.floors.${band.key}`)}
          <span className="text-xs font-normal text-slate-400">{band.key === "ground" ? "1F" : band.from === band.to ? `${band.from}F` : `${band.from}–${band.to}F`}</span>
        </span>
        <span className="tabular-nums text-slate-900">{formatWan(band.low)}–{formatWan(band.high)} <span className="text-xs text-slate-500">{t("chart.unit")}</span></span>
      </div>
      <div className="relative mt-1.5 h-2 rounded-full bg-slate-100">
        <span className="absolute inset-y-0 rounded-full" style={{ left: position(band.low), width: `max(6px, calc(${position(band.high)} - ${position(band.low)}))`, backgroundColor: BAND_COLORS[band.key] }} />
      </div>
      <p className="mt-1 flex justify-between text-[11px] text-slate-400">
        <span>{fill(t("community.floors.count"), { count: band.sales })}</span>
        <span>{fill(t("community.floors.median"), { price: formatWan(band.median) })}</span>
      </p>
    </li>)}
    {bands.some((band) => band.key === "ground") && <li className="text-[11px] text-slate-400">{t("community.floors.groundNote")}</li>}
  </ul>;
}

function SalesTable({ detail }: { detail: CommunityDetail }) {
  const { t, language } = useLanguage();
  const [recentOnly, setRecentOnly] = useState(true);
  const [excludeSpecial, setExcludeSpecial] = useState(true);
  const [page, setPage] = useState(0);
  const all = useMemo(() => visibleSales(detail, false, excludeSpecial), [detail, excludeSpecial]);
  const recent = useMemo(() => visibleSales(detail, true, excludeSpecial), [detail, excludeSpecial]);
  const rows = recentOnly ? recent : all;
  const pages = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
  useEffect(() => setPage(0), [recentOnly, excludeSpecial, detail]);
  const shown = rows.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);
  const chip = (active: boolean) => cn("rounded-full border px-3 py-1 text-xs font-medium", active ? "border-brand-200 bg-brand-50 text-brand-700" : "border-slate-200 text-slate-600 hover:bg-slate-50");

  return <section className={cn(card, "lg:col-span-2")}>
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 className="text-base font-semibold text-slate-900">{t("community.sales.title")}</h2>
        <p className="mt-1 text-xs text-slate-500">{t("community.sales.hint")}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <button type="button" aria-pressed={!recentOnly} onClick={() => setRecentOnly(false)} className={chip(!recentOnly)}>{fill(t("community.sales.all"), { count: all.length })}</button>
        <button type="button" aria-pressed={recentOnly} onClick={() => setRecentOnly(true)} className={chip(recentOnly)}>{fill(t("community.sales.recent"), { count: recent.length })}</button>
        <label className="ml-1 flex items-center gap-2 text-xs text-slate-600"><Switch checked={excludeSpecial} onChange={setExcludeSpecial} />{t("community.sales.excludeSpecial")}</label>
      </div>
    </div>
    <div className="mt-3 overflow-x-auto"><table className="w-full min-w-[680px] text-left text-sm">
      <thead className="border-b border-slate-200 text-xs text-slate-500"><tr>
        {(["date", "floor", "area", "layout", "total", "price", "parking", "note"] as const).map((key) =>
          <th key={key} className={cn("whitespace-nowrap px-2.5 py-2.5 font-medium", ["area", "total", "price", "parking"].includes(key) && "text-right")}>{t(`community.col.${key}`)}</th>)}
      </tr></thead>
      <tbody className="divide-y divide-slate-100">
        {shown.length === 0 && <tr><td colSpan={8} className="px-2.5 py-6 text-center text-sm text-slate-400">{t("community.sales.empty")}</td></tr>}
        {shown.map((sale) => {
          const special = (sale.specialReasons?.length ?? 0) > 0;
          return <tr key={sale.id} className={cn("[&>td]:whitespace-nowrap", special && "bg-amber-50/40")}>
            <td className="px-2.5 py-2.5 tabular-nums">{sale.transactedAt.slice(0, 10)}</td>
            <td className="px-2.5 py-2.5">{sale.floorLabel || "—"}{sale.totalFloors ? ` / ${sale.totalFloors}` : ""}</td>
            <td className="px-2.5 py-2.5 text-right tabular-nums">{sale.buildingAreaPing.toFixed(1)}</td>
            <td className="px-2.5 py-2.5">{layoutText(sale, language === "zh")}</td>
            <td className="px-2.5 py-2.5 text-right tabular-nums">{formatWan(sale.totalPrice, 0)}</td>
            <td className="px-2.5 py-2.5 text-right font-semibold tabular-nums text-brand-700">{formatWan(sale.pricePerPing)}</td>
            <td className="px-2.5 py-2.5 text-right tabular-nums">{sale.parkingPrice > 0 ? formatWan(sale.parkingPrice, 0) : "—"}</td>
            <td className="px-2.5 py-2.5">{special
              ? sale.specialReasons!.map((reason) => <span key={reason} className="mr-1 rounded bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-600">{t(`special.${reason}`)}</span>)
              : !sale.transactionType.startsWith("房地") ? <span className="text-[11px] text-slate-500">{sale.transactionType}</span> : null}</td>
          </tr>;
        })}
      </tbody>
    </table></div>
    <div className="mt-3 flex items-center justify-between gap-2 text-xs text-slate-500">
      <span>{fill(t("community.sales.page"), { page: page + 1, pages, count: rows.length })}</span>
      <div className="flex items-center gap-1">
        <button type="button" disabled={page === 0} onClick={() => setPage(page - 1)} aria-label={t("community.sales.previous")}
          className="rounded-md border border-slate-200 p-1.5 disabled:opacity-40"><ChevronLeft className="h-4 w-4" /></button>
        <button type="button" disabled={page >= pages - 1} onClick={() => setPage(page + 1)} aria-label={t("community.sales.next")}
          className="rounded-md border border-slate-200 p-1.5 disabled:opacity-40"><ChevronRight className="h-4 w-4" /></button>
      </div>
    </div>
  </section>;
}
