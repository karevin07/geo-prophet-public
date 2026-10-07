import { useEffect, useState } from "react";
import { ArrowRight, X } from "lucide-react";
import { cellToLatLng, getResolution } from "h3-js";
import { useLanguage } from "../lib/i18n";
import { formatWan, priceClass, PRICE_CLASSES, type PriceBreaks } from "../lib/price-scale";
import { cn, fill } from "../lib/utils";
import type { CellDetail, H3Aggregate } from "../lib/api";
import FacilitySummary, { useNearbyFacilities, type FetchNearbyFacilities } from "./FacilitySummary";
import YoYBadge from "./YoYBadge";

export type FetchCellDetail = (hex: string, signal: AbortSignal) => Promise<CellDetail>;

interface HexCardProps {
  hex: H3Aggregate;
  breaks: PriceBreaks | null;
  /** Screen position of the anchor (cursor or pinned cell centre). */
  x: number;
  y: number;
  pinned: boolean;
  /** Label of the map's date window, e.g. 近 12 個月 or 2026/03. */
  windowLabel: string;
  fetchDetail: FetchCellDetail;
  /** Loads the facilities around the cell's centre for its 生活機能 section. */
  fetchNearby?: FetchNearbyFacilities;
  onClose: () => void;
  onViewDistrict: (cityCode: string, district: string) => void;
}

const CLASS_LABELS = ["legend.low", "legend.midLow", "legend.mid", "legend.high"];
const AGE_COLORS: Record<CellDetail["ageMix"][number]["key"], string> = {
  "0-5": "#4FA3A5", "5-15": "#9CCFD0", "15-30": "#F2C85B", "30+": "#1F6F78", unknown: "#E2E8F0",
};
// Details depend on the filters, which fetchDetail closes over, so cache per fetcher.
const cache = new WeakMap<FetchCellDetail, Map<string, CellDetail>>();

export default function HexCard({ hex, breaks, x, y, pinned, windowLabel, fetchDetail, fetchNearby, onClose, onViewDistrict }: HexCardProps) {
  const { t } = useLanguage();
  const [centerLat, centerLng] = cellToLatLng(hex.hex);
  const { nearby, failed } = useNearbyFacilities(fetchNearby, centerLat, centerLng, pinned ? 0 : 300);
  const cached = cache.get(fetchDetail)?.get(hex.hex) ?? null;
  const [detail, setDetail] = useState<CellDetail | null>(cached);

  // Fetch the place, YoY and age mix; hovering waits briefly so sweeping the mouse is cheap.
  useEffect(() => {
    const known = cache.get(fetchDetail)?.get(hex.hex);
    setDetail(known ?? null);
    if (known) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      fetchDetail(hex.hex, controller.signal).then((result) => {
        if (!cache.has(fetchDetail)) cache.set(fetchDetail, new Map());
        cache.get(fetchDetail)!.set(hex.hex, result);
        setDetail(result);
      }).catch(() => undefined);
    }, pinned ? 0 : 150);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [hex.hex, fetchDetail, pinned]);

  const level = priceClass(hex.avgPricePerPing, breaks);
  const place = detail?.district ? `${detail.district}${detail.road ? ` · ${detail.road}${t("hex.around")}` : ""}` : null;
  const ageTotal = detail?.ageMix.reduce((sum, bucket) => sum + bucket.count, 0) ?? 0;
  const main = detail && ageTotal > 0
    ? detail.ageMix.filter((bucket) => bucket.key !== "unknown").sort((a, b) => b.count - a.count)[0]
    : undefined;

  // Keep the card on screen by flipping it to the anchor's other side near the edges.
  const flipX = x > window.innerWidth - 360;
  const flipY = y > window.innerHeight - 560;

  return (
    <div
      role={pinned ? "dialog" : undefined}
      aria-label={place ?? t(CLASS_LABELS[level])}
      className={cn("absolute z-40 w-80 rounded-xl border border-slate-200 bg-white p-4 shadow-xl", !pinned && "pointer-events-none")}
      style={{
        left: flipX ? undefined : x + 16,
        right: flipX ? `calc(100% - ${x - 16}px)` : undefined,
        top: flipY ? undefined : y + 16,
        bottom: flipY ? `calc(100% - ${y - 16}px)` : undefined,
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-1.5 text-xs font-medium" style={{ color: PRICE_CLASSES[level].hex }}>
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: PRICE_CLASSES[level].hex }} />
            {t(CLASS_LABELS[level])}
            <span className="font-normal text-slate-400">· H3 Res-{getResolution(hex.hex)} · {windowLabel}</span>
          </div>
          {place
            ? <h3 className="mt-1 text-sm font-semibold leading-snug text-slate-900">{place}</h3>
            : <div className="mt-1.5 h-4 w-40 animate-pulse rounded bg-slate-100" />}
        </div>
        {pinned && (
          <button onClick={onClose} aria-label={t("drawer.close")} className="-mr-1 -mt-1 rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
            <X className="h-4 w-4" />
          </button>
        )}
      </div>

      <div className="mt-3 flex flex-wrap items-baseline gap-x-2 gap-y-1">
        <span className="text-2xl font-semibold tabular-nums text-slate-900">{formatWan(hex.avgPricePerPing)}</span>
        <span className="text-xs text-slate-500">{t("chart.unit")}</span>
        {detail?.priceYoY != null && <YoYBadge value={detail.priceYoY} suffix={t("yoy.label")} />}
      </div>
      {detail?.priceYoY != null && (
        <p className="mt-0.5 text-[11px] text-slate-400">{fill(t("hex.yoyNote"), { through: detail.yoyThrough.replace("-", "/") })}</p>
      )}

      <dl className="mt-3 grid grid-cols-3 gap-2 border-y border-slate-100 py-3 text-center">
        {([[t("map.avgTotal"), `${formatWan(hex.price, 0)} 萬`], [t("map.count"), `${hex.count.toLocaleString()} ${t("chart.count")}`], [t("map.avgArea"), `${hex.avgArea.toFixed(1)} 坪`]] as const).map(([label, value]) => (
          <div key={label}>
            <dt className="text-[11px] text-slate-500">{label}</dt>
            <dd className="mt-0.5 text-sm font-semibold tabular-nums text-slate-800">{value}</dd>
          </div>
        ))}
      </dl>

      <div className="mt-3">
        <div className="mb-1.5 flex items-center justify-between text-xs">
          <span className="font-medium text-slate-600">{t("hex.ageMix")}</span>
          {main && <span className="text-slate-500">{fill(t("hex.ageMain"), { bucket: t(`age.${main.key}`) })}</span>}
        </div>
        {detail && ageTotal > 0 ? (
          <>
            <div className="flex h-2 overflow-hidden rounded-full bg-slate-100">
              {detail.ageMix.filter((bucket) => bucket.count > 0).map((bucket) => (
                <span key={bucket.key} style={{ width: `${(bucket.count / ageTotal) * 100}%`, backgroundColor: AGE_COLORS[bucket.key] }} />
              ))}
            </div>
            <ul className="mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[11px] text-slate-500">
              {detail.ageMix.filter((bucket) => bucket.count > 0).map((bucket) => (
                <li key={bucket.key} className={cn("flex items-center gap-1", bucket.key === main?.key && "font-semibold text-slate-700")}>
                  <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: AGE_COLORS[bucket.key] }} />
                  {t(`age.${bucket.key}`)} {Math.round((bucket.count / ageTotal) * 100)}%
                </li>
              ))}
            </ul>
          </>
        ) : <div className="h-2 animate-pulse rounded-full bg-slate-100" />}
      </div>

      {fetchNearby && <FacilitySummary nearby={nearby} failed={failed} className="mt-3 border-t border-slate-100 pt-3" />}

      {pinned && detail?.district && (
        <button
          onClick={() => onViewDistrict(detail.cityCode, detail.district)}
          className="mt-3 flex items-center gap-1 text-sm font-medium text-brand-600 hover:text-brand-700"
        >
          {fill(t("hex.viewSales"), { district: detail.district })}
          <ArrowRight className="h-4 w-4" />
        </button>
      )}
      {!pinned && <p className="mt-3 text-[11px] text-slate-400">{t("hex.clickToPin")}</p>}
    </div>
  );
}
