import type { ReactNode } from "react";
import { ChevronDown, RotateCcw, SlidersHorizontal } from "lucide-react";
import { useLanguage } from "../../lib/i18n";
import { fill, isoDate } from "../../lib/utils";
import { FEED_PERIODS, type FeedPeriod, type FeedState } from "../../lib/url-state";
import type { BuildingType, Region, Release } from "../../lib/api";
import { RangeField, Switch, TypeChips } from "../ui";

const WAN = 10_000;

interface FeedFiltersProps {
  regions: Region[];
  feed: FeedState;
  setFeed: (update: Partial<FeedState>) => void;
  latestRelease: Release | null;
  priceRange: [number, number];
  setPriceRange: (value: [number, number]) => void;
  areaRange: [number, number];
  setAreaRange: (value: [number, number]) => void;
  selectedTypes: BuildingType[];
  setSelectedTypes: (value: BuildingType[]) => void;
  includeSpecial: boolean;
  setIncludeSpecial: (value: boolean) => void;
  onReset: () => void;
}

export default function FeedFilters(props: FeedFiltersProps) {
  const { t } = useLanguage();
  const { feed, setFeed, regions } = props;
  const districts = regions.find((region) => region.code === feed.city)?.districts ?? [];

  return (
    <aside className="rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_4px_16px_rgba(15,23,42,0.06)]">
      <div className="mb-5 flex items-center justify-between">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-800">
          <SlidersHorizontal className="h-4 w-4 text-brand-600" />
          {t("feed.filters")}
        </h2>
        <button onClick={props.onReset} className="flex items-center gap-1 text-xs font-medium text-brand-600 hover:text-brand-700">
          <RotateCcw className="h-3.5 w-3.5" />
          {t("filter.reset")}
        </button>
      </div>

      <div className="space-y-5">
        <div>
          <div className="mb-2 text-xs font-medium text-slate-600">{t("feed.scope")}</div>
          <div className="grid grid-cols-2 gap-2">
            <Select value={feed.city} onChange={(city) => setFeed({ city, district: "" })} label={t("feed.scope")}>
              <option value="">{t("feed.allCities")}</option>
              {regions.map((region) => <option key={region.code} value={region.code}>{region.name}</option>)}
            </Select>
            <Select value={feed.district} onChange={(district) => setFeed({ district })} disabled={!feed.city} label={t("feed.allDistricts")}>
              <option value="">{t("feed.allDistricts")}</option>
              {districts.map((district) => <option key={district} value={district}>{district}</option>)}
            </Select>
          </div>
        </div>

        <div>
          <div className="mb-2 text-xs font-medium text-slate-600">{t("feed.period")}</div>
          <Select value={String(feed.period)} onChange={(value) => setFeed({ period: Number(value) as FeedPeriod })} label={t("feed.period")}>
            {FEED_PERIODS.map((period) => <option key={period} value={period}>{t(`feed.period${period}`)}</option>)}
          </Select>
        </div>

        <RangeField
          label={t("filter.price")}
          unit="萬"
          value={[props.priceRange[0] / WAN, props.priceRange[1] / WAN]}
          onChange={([low, high]) => props.setPriceRange([low * WAN, high * WAN])}
        />
        <RangeField label={t("filter.area")} unit="坪" value={props.areaRange} onChange={props.setAreaRange} />
        <TypeChips label={t("filter.type")} selected={props.selectedTypes} onChange={props.setSelectedTypes} />

        <label className="flex cursor-pointer items-start justify-between gap-3">
          <span>
            <span className="block text-xs font-medium text-slate-600">{t("feed.latestOnly")}</span>
            {props.latestRelease && (
              <span className="block text-[11px] text-slate-400">
                {fill(t("feed.latestHint"), { from: isoDate(props.latestRelease.registeredFrom), to: isoDate(props.latestRelease.registeredTo) })}
              </span>
            )}
          </span>
          <Switch checked={feed.latestOnly} onChange={(latestOnly) => setFeed({ latestOnly })} />
        </label>
        <label className="flex cursor-pointer items-start justify-between gap-3">
          <span>
            <span className="block text-xs font-medium text-slate-600">{t("filter.special")}</span>
            <span className="block text-[11px] text-slate-400">{t("filter.specialHint")}</span>
          </span>
          <Switch checked={props.includeSpecial} onChange={props.setIncludeSpecial} />
        </label>
      </div>
    </aside>
  );
}

function Select({ value, onChange, disabled, label, children }: { value: string; onChange: (value: string) => void; disabled?: boolean; label: string; children: ReactNode }) {
  return (
    <div className="relative">
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        aria-label={label}
        className="h-9 w-full appearance-none rounded-lg border border-slate-200 bg-white pl-3 pr-8 text-sm text-slate-800 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100 disabled:bg-slate-50 disabled:text-slate-400"
      >
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
    </div>
  );
}
