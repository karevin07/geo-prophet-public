import { Building, Grid2x2, SlidersHorizontal, TrainFront, X } from "lucide-react";
import type { FacilityGroup } from "../lib/api";
import { FACILITY_GROUPS, GROUP_COLORS } from "../lib/facilities";
import { useLanguage } from "../lib/i18n";
import { cn } from "../lib/utils";
import type { LayerSettings } from "../lib/layer-settings";
import FacilityIcon from "./FacilityIcon";
import { Switch } from "./ui";

interface LayersPanelProps {
  settings: LayerSettings;
  onChange: (update: Partial<LayerSettings>) => void;
  onClose: () => void;
  /** Facilities of each group drawn on the map now, when facilities are shown. */
  facilityCounts: Partial<Record<FacilityGroup, number>>;
  /** Presale projects drawn on the map now, when the layer is on. */
  presaleCount: number | null;
}

export default function LayersPanel({ settings, onChange, onClose, facilityCounts, presaleCount }: LayersPanelProps) {
  const { t } = useLanguage();
  const percent = Math.round(settings.opacity * 100);
  return (
    <div role="dialog" aria-label={t("layers.title")} className="absolute right-12 top-0 max-h-[calc(100dvh-20rem)] w-72 overflow-y-auto sm:max-h-[calc(100vh-9rem)] rounded-xl border border-slate-200 bg-white p-4 text-sm shadow-xl">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="flex items-center gap-2 font-semibold text-slate-800">
          <SlidersHorizontal className="h-4 w-4 text-brand-600" />
          {t("layers.title")}
        </h3>
        <button onClick={onClose} aria-label={t("drawer.close")} className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
          <X className="h-4 w-4" />
        </button>
      </div>

      <div className="mb-1 flex justify-between text-xs text-slate-600">
        <span>{t("layers.opacity")}</span>
        <span className="tabular-nums">{percent}%</span>
      </div>
      <input
        type="range" min={20} max={90} step={5} value={percent}
        onChange={(event) => onChange({ opacity: Number(event.target.value) / 100 })}
        aria-label={t("layers.opacity")}
        className="w-full"
      />

      <div className="mb-2 mt-4 text-xs text-slate-600">{t("layers.basemap")}</div>
      <div className="grid grid-cols-2 rounded-lg bg-slate-100 p-1">
        {(["color", "light"] as const).map((basemap) => (
          <button
            key={basemap}
            onClick={() => onChange({ basemap })}
            aria-pressed={settings.basemap === basemap}
            className={cn("rounded-md py-1.5 text-xs font-medium", settings.basemap === basemap ? "bg-brand-600 text-white shadow-sm" : "text-slate-600")}
          >
            {t(`layers.${basemap}`)}
          </button>
        ))}
      </div>

      <div className="mb-2 mt-4 text-xs text-slate-600">{t("layers.overlays")}</div>
      <div className="space-y-2.5">
        <label className="flex cursor-pointer items-center justify-between gap-3 text-slate-700">
          <span className="flex items-center gap-2 text-xs"><TrainFront className="h-3.5 w-3.5 text-slate-400" />{t("layers.transit")}</span>
          <Switch checked={settings.transit} onChange={(transit) => onChange({ transit })} />
        </label>
        <label className="flex cursor-pointer items-center justify-between gap-3 text-slate-700">
          <span className="flex items-center gap-2 text-xs"><Grid2x2 className="h-3.5 w-3.5 text-slate-400" />{t("layers.districts")}</span>
          <Switch checked={settings.districts} onChange={(districts) => onChange({ districts })} />
        </label>
        <label className="flex cursor-pointer items-center justify-between gap-3 text-slate-700">
          <span className="flex min-w-0 items-center gap-2 text-xs"><Building className="h-3.5 w-3.5 shrink-0 text-slate-400" />
            <span className="min-w-0">{t("layers.presales")}<span className="block text-[10px] text-slate-400">{t("layers.presalesHint")}</span></span>
          </span>
          <span className="flex items-center gap-2">
            {settings.presales && presaleCount !== null && <span className="rounded-full bg-slate-100 px-1.5 text-[10px] tabular-nums text-slate-600">{presaleCount}</span>}
            <Switch checked={settings.presales} onChange={(presales) => onChange({ presales })} />
          </span>
        </label>
      </div>

      <div className="mt-4 border-t border-slate-100 pt-4">
        <div className="mb-2 flex items-center justify-between gap-3">
          <span className="text-xs font-semibold text-slate-700">{t("facilities.title")}</span>
          <Switch
            checked={settings.facilityGroups.length > 0}
            onChange={(on) => onChange({ facilityGroups: on ? [...FACILITY_GROUPS] : [], facilityView: on ? settings.facilityView : "price" })}
          />
        </div>
        <div className="mb-3 grid grid-cols-2 rounded-lg bg-slate-100 p-1" role="group" aria-label={t("facilities.view")}>
          {(["price", "density"] as const).map((view) => (
            <button
              key={view}
              onClick={() => onChange({ facilityView: view, facilityGroups: view === "density" && settings.facilityGroups.length === 0 ? [...FACILITY_GROUPS] : settings.facilityGroups })}
              aria-pressed={settings.facilityView === view}
              className={cn("rounded-md py-1.5 text-xs font-medium", settings.facilityView === view ? "bg-brand-600 text-white shadow-sm" : "text-slate-600")}
            >
              {t(`facilities.view.${view}`)}
            </button>
          ))}
        </div>
        <ul className="space-y-1.5">
          {FACILITY_GROUPS.map((group) => {
            const on = settings.facilityGroups.includes(group);
            return (
              <li key={group}>
                <label className={cn("flex cursor-pointer items-center gap-2.5 rounded-lg px-1.5 py-1", group === "nuisance" && on && "bg-amber-50")}>
                  <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-white" style={{ backgroundColor: GROUP_COLORS[group] }}>
                    <FacilityIcon group={group} className="h-3.5 w-3.5" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block text-xs font-medium text-slate-800">{t(`facilities.group.${group}`)}</span>
                    <span className="block truncate text-[10px] text-slate-400">{t(`facilities.groupHint.${group}`)}</span>
                  </span>
                  {on && facilityCounts[group] !== undefined && (
                    <span className="rounded-full bg-slate-100 px-1.5 text-[10px] tabular-nums text-slate-600">{facilityCounts[group]}</span>
                  )}
                  <Switch
                    checked={on}
                    onChange={(checked) => onChange({ facilityGroups: FACILITY_GROUPS.filter((candidate) => candidate === group ? checked : settings.facilityGroups.includes(candidate)) })}
                  />
                </label>
              </li>
            );
          })}
        </ul>
        <p className="mt-2 text-[10px] leading-relaxed text-slate-400">{t("facilities.zoomNote")}</p>
        {settings.facilityView === "density" && <p className="mt-1 text-[10px] leading-relaxed text-slate-400">{t("facilities.densityNote")}</p>}
        <p className="mt-1 text-[10px] text-slate-400">{t("facilities.attribution")}</p>
      </div>
    </div>
  );
}
