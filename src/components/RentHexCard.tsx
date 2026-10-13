import { useEffect, useState } from "react";
import { ArrowRight, X } from "lucide-react";
import { getResolution } from "h3-js";
import type { BuildingType, CellDetail, RentCell } from "../lib/api";
import { useLanguage } from "../lib/i18n";
import { formatWan, priceClass, PRICE_CLASSES, type PriceBreaks } from "../lib/price-scale";
import { formatRentValue, rentValue, type MapMetric } from "../lib/rent-scale";
import { cn, fill } from "../lib/utils";
import type { FetchCellDetail } from "./HexCard";

interface RentHexCardProps {
  cell: RentCell;
  metric: Exclude<MapMetric, "price">;
  breaks: PriceBreaks | null;
  /** The month rents are adjusted to, YYYY-MM. */
  through: string;
  minimumSample: number;
  /** Screen position of the anchor (cursor or pinned cell centre). */
  x: number;
  y: number;
  pinned: boolean;
  /** Names the cell's district and road. */
  fetchDetail: FetchCellDetail;
  onClose: () => void;
  onViewRents: (cityCode: string, district: string) => void;
}

const LEVELS = ["rentLegend.level0", "rentLegend.level1", "rentLegend.level2", "rentLegend.level3"];
const CATEGORY_COLORS: Record<BuildingType, string> = {
  住宅大樓: "#1F6F78", 華廈: "#4FA3A5", 公寓: "#F2C85B", 透天厝: "#94A3B8", 其他: "#CBD5E1",
};
// Places do not depend on the filters, so cache them by cell for every fetcher.
const places = new Map<string, Pick<CellDetail, "cityCode" | "district" | "road">>();

/** The map's card for a cell of the rent layer, on hover or pinned. */
export default function RentHexCard({ cell, metric, breaks, through, minimumSample, x, y, pinned, fetchDetail, onClose, onViewRents }: RentHexCardProps) {
  const { t } = useLanguage();
  const [place, setPlace] = useState(places.get(cell.hex) ?? null);

  // Hovering waits briefly so sweeping the mouse is cheap.
  useEffect(() => {
    const known = places.get(cell.hex);
    setPlace(known ?? null);
    if (known) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      fetchDetail(cell.hex, controller.signal).then((detail) => {
        const found = { cityCode: detail.cityCode, district: detail.district, road: detail.road };
        places.set(cell.hex, found);
        setPlace(found);
      }).catch(() => undefined);
    }, pinned ? 0 : 150);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [cell.hex, fetchDetail, pinned]);

  const value = rentValue(cell, metric);
  const level = value === null ? 1 : priceClass(value, breaks);
  const title = place?.district ? `${place.district}${place.road ? ` · ${place.road}${t("hex.around")}` : ""}` : null;
  const flipX = x > window.innerWidth - 360;
  const flipY = y > window.innerHeight - 480;

  return (
    <div
      role={pinned ? "dialog" : undefined}
      aria-label={title ?? t(`layers.metric.${metric}`)}
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
            {t(`layers.metric.${metric}`)} {t(LEVELS[level])}
            <span className="font-normal text-slate-400">· H3 Res-{getResolution(cell.hex)} · {t("rentCard.window")}</span>
          </div>
          {title
            ? <h3 className="mt-1 text-sm font-semibold leading-snug text-slate-900">{title}</h3>
            : <div className="mt-1.5 h-4 w-40 animate-pulse rounded bg-slate-100" />}
        </div>
        {pinned && (
          <button onClick={onClose} aria-label={t("drawer.close")} className="-mr-1 -mt-1 rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
            <X className="h-4 w-4" />
          </button>
        )}
      </div>

      <div className="mt-3 flex items-baseline gap-2">
        <span className="text-2xl font-semibold tabular-nums text-slate-900">{Math.round(cell.medianRentPerPing).toLocaleString()}</span>
        <span className="text-xs text-slate-500">{t("rentCard.unit")}</span>
      </div>
      <p className="mt-0.5 text-xs text-slate-500">
        {fill(t("rentCard.monthly"), { rent: formatWan(cell.medianMonthlyRent), area: cell.medianAreaPing.toFixed(0) })}
      </p>

      <dl className="mt-3 grid grid-cols-3 gap-2 rounded-lg bg-slate-50 py-2.5 text-center">
        <div>
          <dt className="text-[11px] text-slate-500">{t("rentCard.leases")}</dt>
          <dd className="mt-0.5 text-sm font-semibold tabular-nums text-slate-800">{cell.rentals.toLocaleString()} {t("chart.count")}</dd>
        </div>
        <div>
          <dt className="text-[11px] text-slate-500">{t("rentCard.rooms")}</dt>
          <dd className="mt-0.5 text-sm font-semibold tabular-nums text-slate-800">{cell.mainRooms === null ? "—" : fill(t("rentCard.roomsValue"), { rooms: cell.mainRooms })}</dd>
        </div>
        <div>
          <dt className="text-[11px] text-slate-500">{t("rentCard.yield")}</dt>
          <dd className="mt-0.5 text-sm font-semibold tabular-nums text-brand-700">{cell.grossYield === null ? "—" : formatRentValue(cell.grossYield, "yield")}</dd>
        </div>
      </dl>
      <p className="mt-1.5 text-[11px] text-slate-500">
        {cell.medianPricePerPing === null
          ? fill(t("rentCard.noPrice"), { count: minimumSample })
          : fill(t("rentCard.price"), { price: formatWan(cell.medianPricePerPing) })}
      </p>

      {cell.categories.length > 0 && (
        <div className="mt-3">
          <div className="mb-1.5 text-xs font-medium text-slate-600">{t("rentCard.mix")}</div>
          <div className="flex h-2 overflow-hidden rounded-full bg-slate-100">
            {cell.categories.map((entry) => (
              <span key={entry.category} style={{ width: `${entry.share * 100}%`, backgroundColor: CATEGORY_COLORS[entry.category] }} />
            ))}
          </div>
          <ul className="mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[11px] text-slate-500">
            {cell.categories.map((entry) => (
              <li key={entry.category} className="flex items-center gap-1">
                <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: CATEGORY_COLORS[entry.category] }} />
                {t(`type.${entry.category}`)} {Math.round(entry.share * 100)}%
              </li>
            ))}
          </ul>
        </div>
      )}

      <p className="mt-3 text-[10px] text-slate-400">{fill(t("rentCard.adjusted"), { through: through.replace("-", "/") })}</p>
      {pinned && place?.district && (
        <button
          onClick={() => onViewRents(place.cityCode, place.district)}
          className="mt-2 flex items-center gap-1 text-sm font-medium text-brand-600 hover:text-brand-700"
        >
          {fill(t("rentCard.viewRents"), { district: place.district })}
          <ArrowRight className="h-4 w-4" />
        </button>
      )}
      {!pinned && <p className="mt-2 text-[11px] text-slate-400">{t("hex.clickToPin")}</p>}
    </div>
  );
}
