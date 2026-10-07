import { useCallback, useMemo } from "react";
import { useLanguage } from "../../lib/i18n";
import { formatWan, priceBreaks, priceColor, PRICE_CLASSES } from "../../lib/price-scale";
import type { DistrictBoundaries, DistrictRank } from "../../lib/api";
import ChoroplethMap from "../ChoroplethMap";
import YoYBadge from "../YoYBadge";
import { DISTRICT_COLORS } from "./colors";

interface DistrictMapProps {
  boundaries: DistrictBoundaries | null;
  ranking: DistrictRank[];
  selected: string[];
  onToggle: (district: string) => void;
}

const MAX_SELECTED = 4;
const LEGEND_LABELS = ["legend.low", "legend.midLow", "legend.mid", "legend.high"];

export default function DistrictMap({ boundaries, ranking, selected, onToggle }: DistrictMapProps) {
  const { t } = useLanguage();
  const byDistrict = useMemo(() => new Map(ranking.map((row) => [row.district, row])), [ranking]);
  const breaks = useMemo(() => priceBreaks(ranking.map((row) => row.medianPricePerPing)), [ranking]);
  const fill = useCallback((district: string) => {
    const row = byDistrict.get(district);
    return row ? priceColor(row.medianPricePerPing, breaks, 150) : null;
  }, [byDistrict, breaks]);
  const fillKey = useMemo(() => `${breaks?.join()}|${ranking.map((row) => `${row.district}:${row.medianPricePerPing}`).join()}`, [breaks, ranking]);

  return (
    <ChoroplethMap
      boundaries={boundaries}
      fill={fill}
      fillKey={fillKey}
      selected={selected.map((district, index) => ({ district, color: DISTRICT_COLORS[index] }))}
      onSelect={(district) => {
        if (selected.includes(district) || selected.length < MAX_SELECTED) onToggle(district);
      }}
      hoverContent={(district) => {
        const hovered = byDistrict.get(district);
        return hovered ? (
          <dl className="space-y-1">
            <div className="flex justify-between gap-2"><dt className="text-slate-500">{t("districts.col.median")}</dt><dd className="tabular-nums text-slate-800">{formatWan(hovered.medianPricePerPing)} {t("chart.unit")}</dd></div>
            <div className="flex justify-between gap-2"><dt className="text-slate-500">{t("districts.col.count")}</dt><dd className="tabular-nums text-slate-800">{hovered.count.toLocaleString()}</dd></div>
            <div className="flex items-center justify-between gap-2"><dt className="text-slate-500">{t("districts.col.yoy")}</dt><dd>{hovered.priceYoY == null ? "—" : <YoYBadge value={hovered.priceYoY} />}</dd></div>
          </dl>
        ) : <p className="text-slate-400">{t("districts.mapNoSales")}</p>;
      }}
      legend={[3, 2, 1, 0].map((index) => (
        <span key={index} className="flex items-center gap-1">
          <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: PRICE_CLASSES[index].hex }} />
          {t(LEGEND_LABELS[index])}
          {breaks && <span className="tabular-nums text-slate-400">{
            index === 3 ? `≥${formatWan(breaks[2], 0)}` : index === 0 ? `<${formatWan(breaks[0], 0)}` : `${formatWan(breaks[index - 1], 0)}–${formatWan(breaks[index], 0)}`
          }</span>}
        </span>
      ))}
    />
  );
}
