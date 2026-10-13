import type { FacilityGroup } from "../lib/api";
import { IS_DEMO } from "../lib/demo";
import { DENSITY_CLASSES, GROUP_COLORS } from "../lib/facilities";
import { useLanguage } from "../lib/i18n";
import FacilityIcon from "./FacilityIcon";
import { formatWan, PRICE_CLASSES, type PriceBreaks } from "../lib/price-scale";
import { yoyColor } from "../lib/transit";
import { formatRentValue, type MapMetric } from "../lib/rent-scale";
import { fill } from "../lib/utils";

const labels = ["legend.low", "legend.midLow", "legend.mid", "legend.high"];

const STATION_RINGS = [["transit.up", 0.05], ["transit.down", -0.05], ["transit.flat", null]] as const;

interface LegendProps {
  breaks: PriceBreaks | null;
  showTransit: boolean;
  /** The density view replaces the price classes. */
  density: boolean;
  facilityGroups: FacilityGroup[];
  presales: boolean;
  /** Rents or yields replace the price classes. */
  rent?: { metric: Exclude<MapMetric, "price">; breaks: PriceBreaks | null; minimumSample: number } | null;
}

export default function Legend({ breaks, showTransit, density, facilityGroups, presales, rent }: LegendProps) {
  const { t } = useLanguage();
  const ranges = breaks
    ? [`< ${formatWan(breaks[0], 0)}`, `${formatWan(breaks[0], 0)} – ${formatWan(breaks[1], 0)}`, `${formatWan(breaks[1], 0)} – ${formatWan(breaks[2], 0)}`, `≥ ${formatWan(breaks[2], 0)}`]
    : ["—", "—", "—", "—"];

  return (
    <div className="absolute bottom-24 right-3 z-20 w-48 rounded-2xl border border-slate-200/80 bg-white/95 px-4 py-3 shadow-[0_4px_16px_rgba(15,23,42,0.08)] backdrop-blur sm:bottom-10 sm:right-4 sm:w-56">
      {density ? (
        <>
          <h3 className="mb-2 text-xs font-semibold text-slate-700">{t("facilities.legend")}</h3>
          <ul className="space-y-1.5">
            {[...DENSITY_CLASSES].reverse().map((entry) => (
              <li key={entry.label} className="flex items-center gap-2 text-xs">
                <span className="h-3 w-3 shrink-0 rounded-sm border border-slate-200" style={{ backgroundColor: entry.hex }} />
                <span className="flex-1 text-slate-600">{entry.label} {t("facilities.kinds")}</span>
              </li>
            ))}
            <li className="flex items-center gap-2 text-xs text-slate-600">
              <span className="flex h-3 w-3 items-center justify-center"><span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: GROUP_COLORS.nuisance }} /></span>
              {t("facilities.legendNuisance")}
            </li>
          </ul>
        </>
      ) : rent ? (
        <RentLegend {...rent} />
      ) : (
        <>
          <h3 className="mb-2 text-xs font-semibold text-slate-700">{t("legend.title")}</h3>
          <ul className="space-y-1.5">
            {[3, 2, 1, 0].map((index) => (
              <li key={index} className="flex items-center gap-2 text-xs">
                <span className="h-3 w-3 shrink-0 rounded-full" style={{ backgroundColor: PRICE_CLASSES[index].hex, opacity: 0.85 }} />
                <span className="flex-1 text-slate-600">{t(labels[index])}</span>
                <span className="font-medium tabular-nums text-slate-800">{ranges[index]}</span>
              </li>
            ))}
          </ul>
          <p className="mt-2 hidden border-t border-slate-100 pt-2 text-[11px] text-slate-400 sm:block">{t("legend.caption")}</p>
        </>
      )}
      {presales && (
        <div className="mt-2 border-t border-slate-100 pt-2">
          <h4 className="mb-1.5 text-[11px] font-medium text-slate-600">{t("layers.presales")}</h4>
          <ul className="space-y-1 text-[11px] text-slate-500">
            <li className="flex items-center gap-1.5"><span className="h-3 w-3 rounded-[3px] border-2 border-white bg-slate-400 shadow-[0_0_0_1px_#cbd5e1]" />{t("presales.legend.precise")}</li>
            <li className="flex items-center gap-1.5"><span className="h-3 w-3 rounded-[3px] border-2 border-dashed border-slate-600 bg-slate-400" />{t("presales.legend.road")}</li>
          </ul>
          <p className="mt-1 text-[10px] text-slate-400">{t("presales.legend.note")}</p>
        </div>
      )}
      {facilityGroups.length > 0 && (
        <div className="mt-2 border-t border-slate-100 pt-2">
          <h4 className="mb-1.5 text-[11px] font-medium text-slate-600">{t("facilities.legendIcons")}</h4>
          <ul className="grid grid-cols-2 gap-x-2 gap-y-1 text-[11px] text-slate-500">
            {facilityGroups.map((group) => (
              <li key={group} className="flex items-center gap-1">
                <span className="flex h-3.5 w-3.5 items-center justify-center rounded-full text-white" style={{ backgroundColor: GROUP_COLORS[group] }}><FacilityIcon group={group} className="h-2.5 w-2.5" /></span>
                {t(`facilities.group.${group}`)}
              </li>
            ))}
          </ul>
          <p className="mt-1 text-[10px] text-slate-400">{t("facilities.attribution")}</p>
        </div>
      )}
      {showTransit && (
        <div className="mt-2 border-t border-slate-100 pt-2">
          <h4 className="mb-1.5 text-[11px] font-medium text-slate-600">{t("transit.legend")}</h4>
          <ul className="flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-slate-500">
            {STATION_RINGS.map(([label, value]) => (
              <li key={label} className="flex items-center gap-1">
                <span className="h-2.5 w-2.5 rounded-full border-2 bg-white" style={{ borderColor: `rgb(${yoyColor(value).slice(0, 3).join(",")})` }} />
                {t(label)}
              </li>
            ))}
          </ul>
        </div>
      )}
      {IS_DEMO && <p className="mt-2 border-t border-slate-100 pt-2 text-[10px] leading-relaxed text-slate-400">{t("demo.attribution")}</p>}
    </div>
  );
}

function RentLegend({ metric, breaks, minimumSample }: NonNullable<LegendProps["rent"]>) {
  const { t } = useLanguage();
  const format = (value: number) => formatRentValue(value, metric);
  const ranges = breaks
    ? [`< ${format(breaks[0])}`, `${format(breaks[0])} – ${format(breaks[1])}`, `${format(breaks[1])} – ${format(breaks[2])}`, `≥ ${format(breaks[2])}`]
    : ["—", "—", "—", "—"];
  return (
    <>
      <h3 className="mb-2 text-xs font-semibold text-slate-700">{t(`rentLegend.${metric}`)}</h3>
      <ul className="space-y-1.5">
        {[3, 2, 1, 0].map((index) => (
          <li key={index} className="flex items-center gap-2 text-xs">
            <span className="h-3 w-3 shrink-0 rounded-full" style={{ backgroundColor: PRICE_CLASSES[index].hex, opacity: 0.85 }} />
            <span className="flex-1 text-slate-600">{t(`rentLegend.level${index}`)}</span>
            <span className="font-medium tabular-nums text-slate-800">{ranges[index]}</span>
          </li>
        ))}
      </ul>
      <p className="mt-2 hidden border-t border-slate-100 pt-2 text-[11px] text-slate-400 sm:block">
        {fill(t(metric === "rent" ? "rentLegend.caption" : "rentLegend.yieldCaption"), { count: minimumSample })}
      </p>
    </>
  );
}
