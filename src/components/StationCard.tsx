import { useEffect, useMemo, useState } from "react";
import { X } from "lucide-react";
import { CartesianGrid, Line, LineChart, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { STATION_RADII, type PresaleProject, type StationRadius, type StationRent, type StationTrend, type TransitStation } from "../lib/api";
import { useLanguage } from "../lib/i18n";
import { formatWan } from "../lib/price-scale";
import { CITY_NAMES } from "../lib/data-status";
import { formatRent, formatYield, rentRows } from "../lib/rent";
import { smoothTrend, stationOpenings, uniqueLines, type Opening } from "../lib/transit";
import { cn, fill } from "../lib/utils";
import FacilitySummary, { useNearbyFacilities, type FetchNearbyFacilities } from "./FacilitySummary";
import RentTrendChart from "./rents/RentTrendChart";
import YoYBadge from "./YoYBadge";

export type FetchStationTrend = (stationId: string, signal: AbortSignal) => Promise<StationTrend>;
export type FetchStationRent = (stationId: string, signal: AbortSignal) => Promise<StationRent>;
export type FetchNearbyProjects = (lat: number, lng: number, signal: AbortSignal) => Promise<PresaleProject[]>;

interface StationCardProps {
  station: TransitStation;
  radius: StationRadius;
  onRadiusChange: (radius: StationRadius) => void;
  /** Label of the map's date window, e.g. 近 12 個月 or 2026/03. */
  windowLabel: string;
  fetchTrend: FetchStationTrend;
  fetchRent: FetchStationRent;
  fetchNearbyProjects: FetchNearbyProjects;
  onShowProject: (project: { key: string; cityCode: string }) => void;
  fetchNearbyFacilities?: FetchNearbyFacilities;
  onClose: () => void;
  className?: string;
}

export default function StationCard({ station, radius, onRadiusChange, windowLabel, fetchTrend, fetchRent, fetchNearbyProjects, onShowProject, fetchNearbyFacilities, onClose, className }: StationCardProps) {
  const { t } = useLanguage();
  const [trend, setTrend] = useState<StationTrend | null>(null);
  const [error, setError] = useState(false);

  // The filters live in fetchTrend, so a new function means new figures.
  useEffect(() => {
    const controller = new AbortController();
    setTrend(null);
    setError(false);
    fetchTrend(station.id, controller.signal).then(setTrend).catch(() => {
      if (!controller.signal.aborted) setError(true);
    });
    return () => controller.abort();
  }, [station.id, fetchTrend]);

  // Rents ignore the map's filters, so they only reload for another station.
  const [rent, setRent] = useState<StationRent | null>(null);
  const [rentError, setRentError] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    setRent(null);
    setRentError(false);
    fetchRent(station.id, controller.signal).then(setRent).catch(() => {
      if (!controller.signal.aborted) setRentError(true);
    });
    return () => controller.abort();
  }, [station.id, fetchRent]);

  // Presale projects within 800 m; a failure just hides the section.
  const [projects, setProjects] = useState<PresaleProject[] | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    setProjects(null);
    fetchNearbyProjects(station.lat, station.lng, controller.signal).then(setProjects).catch(() => undefined);
    return () => controller.abort();
  }, [station.lat, station.lng, fetchNearbyProjects]);

  const facilities = useNearbyFacilities(fetchNearbyFacilities, station.lat, station.lng);

  const ring = trend?.rings.find((candidate) => candidate.radius === radius);
  const rentRing = rent?.rings.find((candidate) => candidate.radius === radius);
  const openings = useMemo(() => stationOpenings(station), [station]);
  const lines = uniqueLines(station.lines);

  return (
    <section
      role="dialog"
      aria-label={station.name}
      className={cn("flex flex-col overflow-hidden rounded-2xl border border-slate-200/80 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.08)]", className)}
    >
      <header className="flex items-start justify-between gap-2 border-b border-slate-100 px-5 pb-3 pt-4">
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-slate-900">{station.name}</h2>
          <p className="mt-0.5 text-xs text-slate-500">
            {CITY_NAMES[station.cityCode] ?? ""} {station.district} · {station.systems.map((system) => t(`transit.system.${system}`)).join("、")}
          </p>
        </div>
        <button onClick={onClose} aria-label={t("drawer.close")} className="-mr-1 rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
          <X className="h-4 w-4" />
        </button>
      </header>

      <div className="flex-1 space-y-4 overflow-y-auto px-5 py-4">
        <ul className="flex flex-wrap gap-1.5">
          {lines.map((line) => {
            const opening = openings.find((candidate) => candidate.lineId === line.lineId);
            return (
              <li key={line.lineId} className="flex items-center gap-1.5 rounded-full border border-slate-200 px-2.5 py-1 text-xs text-slate-700">
                <span className="h-2 w-2 rounded-full" style={{ backgroundColor: line.color }} />
                {line.name}
                {opening && <span className="text-[10px] text-slate-400">{fill(t("transit.openedShort"), { month: opening.month.replace("-", "/") })}</span>}
              </li>
            );
          })}
        </ul>

        <div>
          <div className="mb-1.5 text-xs text-slate-500">{t("transit.radius")}</div>
          <div className="grid grid-cols-3 rounded-lg bg-slate-100 p-1">
            {STATION_RADII.map((value) => (
              <button
                key={value}
                onClick={() => onRadiusChange(value)}
                aria-pressed={radius === value}
                className={cn("rounded-md py-1.5 text-xs font-medium", radius === value ? "bg-brand-600 text-white shadow-sm" : "text-slate-600")}
              >
                {value} m
              </button>
            ))}
          </div>
        </div>

        {error ? (
          <p className="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">{t("transit.error")}</p>
        ) : !trend || !ring ? (
          <div className="space-y-3" aria-busy>
            <div className="h-20 animate-pulse rounded-xl bg-slate-100" />
            <div className="h-36 animate-pulse rounded-xl bg-slate-100" />
          </div>
        ) : (
          <>
            <div className="rounded-xl border border-slate-100 bg-slate-50/60 p-4">
              <div className="flex items-center justify-between gap-2 text-xs text-slate-500">
                <span>{t("transit.median")} · {windowLabel}</span>
                <YoYBadge value={ring.priceYoY} suffix={t("yoy.label")} />
              </div>
              {ring.count > 0 ? (
                <div className="mt-1 flex items-baseline gap-1.5">
                  <span className="text-3xl font-semibold tabular-nums text-slate-900">{formatWan(ring.medianPricePerPing)}</span>
                  <span className="text-xs text-slate-500">{t("chart.unit")}</span>
                </div>
              ) : <p className="mt-2 text-sm text-slate-500">{t("transit.noSales")}</p>}
              {trend.yoyThrough && ring.priceYoY != null && (
                <p className="mt-1 text-[11px] text-slate-400">{fill(t("transit.yoyNote"), { through: trend.yoyThrough.replace("-", "/") })}</p>
              )}
            </div>

            <dl className="grid grid-cols-3 gap-2 text-center">
              {([
                [t("transit.count"), `${ring.count.toLocaleString()} ${t("chart.count")}`],
                [t("transit.avg"), ring.count > 0 ? `${formatWan(ring.avgPricePerPing)} 萬` : "—"],
                [t("transit.range"), fill(t("transit.within"), { radius })],
              ] as const).map(([label, value]) => (
                <div key={label} className="rounded-lg border border-slate-100 px-1 py-2">
                  <dt className="text-[11px] text-slate-500">{label}</dt>
                  <dd className="mt-0.5 text-sm font-semibold tabular-nums text-slate-800">{value}</dd>
                </div>
              ))}
            </dl>

            <StationRentSection rent={rent} ring={rentRing} error={rentError} radius={radius} />

            <div>
              <h3 className="mb-1 text-xs font-medium text-slate-600">{t("transit.trend")}</h3>
              <StationTrendChart points={ring.trends} openings={openings} />
            </div>

            {rentRing && rentRing.trend.some((point) => point.medianRentPerPing !== null) && (
              <div>
                <h3 className="mb-1 text-xs font-medium text-slate-600">{t("rents.station.trend")}</h3>
                <RentTrendChart rows={rentRows(rentRing.trend)} cityLabel={fill(t("transit.within"), { radius })} className="h-40" />
              </div>
            )}

            {fetchNearbyFacilities && <FacilitySummary nearby={facilities.nearby} failed={facilities.failed} />}

            {projects && projects.length > 0 && (
              <div>
                <h3 className="mb-1.5 text-xs font-medium text-slate-600">{fill(t("presales.station.title"), { count: projects.length })}</h3>
                <ul className="space-y-1.5">
                  {projects.slice(0, 3).map((project) => (
                    <li key={project.key}>
                      <button onClick={() => onShowProject(project)} className="flex w-full items-center justify-between gap-2 rounded-lg border border-slate-100 px-3 py-2 text-left hover:border-brand-300">
                        <span className="min-w-0">
                          <span className="block truncate text-sm font-medium text-slate-800">{project.name}</span>
                          <span className="text-[11px] text-slate-500">{fill(t("presales.station.meta"), { distance: project.distanceM ?? 0, count: project.recentSales })}</span>
                        </span>
                        <span className="shrink-0 text-sm font-semibold tabular-nums text-slate-800">{project.medianPricePerPing === null ? "—" : `${formatWan(project.medianPricePerPing)} 萬`}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            )}

            <div>
              <h3 className="mb-1.5 text-xs font-medium text-slate-600">{t("transit.rings")}</h3>
              <div className="grid grid-cols-3 gap-2">
                {trend.rings.map((candidate) => (
                  <button
                    key={candidate.radius}
                    onClick={() => onRadiusChange(candidate.radius)}
                    className={cn(
                      "rounded-lg border px-1 py-2 text-center",
                      candidate.radius === radius ? "border-brand-600 bg-brand-50/60" : "border-slate-100 hover:border-slate-300",
                    )}
                  >
                    <div className="text-[11px] text-slate-500">{candidate.radius} m</div>
                    <div className="text-sm font-semibold tabular-nums text-slate-800">{candidate.count > 0 ? `${formatWan(candidate.medianPricePerPing)} 萬` : "—"}</div>
                    <div className="mt-0.5 flex justify-center"><YoYBadge value={candidate.priceYoY} /></div>
                  </button>
                ))}
              </div>
            </div>
          </>
        )}

        <p className="text-[11px] leading-relaxed text-slate-400">{t("transit.footnote")}</p>
      </div>
    </section>
  );
}

function StationRentSection({ rent, ring, error, radius }: { rent: StationRent | null; ring: StationRent["rings"][number] | undefined; error: boolean; radius: StationRadius }) {
  const { t } = useLanguage();
  const heading = (
    <h3 className="mb-2 flex items-center gap-1.5 text-xs font-medium text-slate-600">
      {t("rents.station.title")}
      <span className="rounded border border-brand-200 bg-brand-50 px-1 text-[10px] font-semibold text-brand-700">{t("rents.station.new")}</span>
    </h3>
  );
  if (error) return <div>{heading}<p className="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">{t("rents.error")}</p></div>;
  if (!rent) return <div>{heading}<div className="h-20 animate-pulse rounded-xl bg-slate-100" aria-busy /></div>;
  if (!rent.through || !ring) return <div>{heading}<p className="text-xs text-slate-500">{t("rents.noCoverage")}</p></div>;
  return (
    <div>
      {heading}
      {ring.medianRentPerPing === null ? (
        <p className="rounded-lg border border-slate-100 px-3 py-2 text-xs text-slate-500">{fill(t("rents.station.few"), { count: ring.rentals, minimum: rent.minimumSample })}</p>
      ) : (
        <>
          <dl className="grid grid-cols-3 gap-2 text-center">
            {([
              [t("rents.col.rent"), formatRent(ring.medianRentPerPing), t("rents.unit.perPing"), false],
              [t("rents.kpi.monthly"), ring.medianMonthlyRent === null ? "—" : formatWan(ring.medianMonthlyRent), t("rents.unit.wan"), false],
              [t("rents.col.yield"), formatYield(ring.grossYield), "", true],
            ] as const).map(([label, value, unit, highlight]) => (
              <div key={label} className="rounded-lg border border-slate-100 bg-slate-50/60 px-1 py-2">
                <dt className="text-[11px] text-slate-500">{label}</dt>
                <dd className={cn("mt-0.5 text-sm font-semibold tabular-nums", highlight ? "text-brand-700" : "text-slate-800")}>{value}</dd>
                {unit && <dd className="text-[10px] text-slate-400">{unit}</dd>}
              </div>
            ))}
          </dl>
          {ring.relativeToDistrict !== null && (
            <p className="mt-2 flex items-center gap-1.5 text-xs text-slate-600">{fill(t("rents.station.vsDistrict"), { district: rent.district })}<YoYBadge value={ring.relativeToDistrict} /></p>
          )}
        </>
      )}
      <p className="mt-1 text-[11px] text-slate-400">{fill(t("rents.station.caption"), { radius, count: ring.rentals, from: rent.from, through: rent.through })}</p>
    </div>
  );
}

function StationTrendChart({ points, openings }: { points: StationTrend["rings"][number]["trends"]; openings: Opening[] }) {
  const { t } = useLanguage();
  // Split the smoothed line at the first provisional month so its tail is dashed.
  const rows = useMemo(() => {
    const smoothed = smoothTrend(points);
    const firstProvisional = smoothed.findIndex((point) => point.provisional);
    return smoothed.map((point, index) => ({
      ...point,
      settled: firstProvisional < 0 || index < firstProvisional ? point.price : null,
      tail: firstProvisional >= 0 && index >= firstProvisional - 1 ? point.price : null,
    }));
  }, [points]);
  const first = rows[0]?.month;
  const last = rows.at(-1)?.month;

  if (rows.length === 0) {
    return <div className="flex h-36 items-center justify-center rounded-lg border border-dashed border-slate-200 text-xs text-slate-400">{t("chart.empty")}</div>;
  }
  return (
    <div className="h-40 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={rows} margin={{ top: 18, right: 8, left: -18, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
          <XAxis dataKey="month" stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} minTickGap={32} tickFormatter={(month: string) => month.slice(0, 4)} />
          <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} axisLine={false} domain={["auto", "auto"]} tickFormatter={(value: number) => `${Math.round(value)}`} />
          <Tooltip
            contentStyle={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, fontSize: 12, boxShadow: "0 4px 16px rgba(15,23,42,0.08)" }}
            labelStyle={{ color: "#64748b", marginBottom: 2 }}
            formatter={(value, name, item) => name === "tail" && item.payload.settled != null
              ? [null, null]
              : [`${value} ${t("chart.unit")}（${item.payload.count} ${t("chart.count")}）`, t("chart.price")]}
          />
          {openings.filter((opening) => first && last && opening.month >= first && opening.month <= last).map((opening) => (
            <ReferenceLine
              key={opening.lineId} x={opening.month} stroke={opening.color.toLowerCase() === "#fedb00" ? "#d4a800" : opening.color} strokeDasharray="4 3"
              label={{ value: fill(t("transit.opened"), { line: opening.name }), position: "insideTopLeft", fontSize: 10, fill: "#475569" }}
            />
          ))}
          <Line type="monotone" dataKey="settled" stroke="#1f6f78" strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
          <Line type="monotone" dataKey="tail" stroke="#1f6f78" strokeWidth={2} strokeDasharray="4 4" dot={false} connectNulls isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
