import { useEffect, useState } from "react";
import { TriangleAlert } from "lucide-react";
import type { NearbyCategory, NearbyFacilities } from "../lib/api";
import { FACILITY_CATEGORIES, GROUP_COLORS, POSITIVE_CATEGORY_COUNT } from "../lib/facilities";
import { useLanguage } from "../lib/i18n";
import { cn, fill } from "../lib/utils";
import FacilityIcon from "./FacilityIcon";

export type FetchNearbyFacilities = (lat: number, lng: number, signal: AbortSignal) => Promise<NearbyFacilities>;

// Nearby facilities do not depend on any filter, so one cache serves every card.
const cache = new Map<string, NearbyFacilities>();

/** Loads the facilities around a point, waiting delay ms so sweeping the mouse is cheap. */
export function useNearbyFacilities(fetchNearby: FetchNearbyFacilities | undefined, lat: number | null, lng: number | null, delay = 0) {
  const key = lat === null || lng === null ? "" : `${lat.toFixed(5)},${lng.toFixed(5)}`;
  const [nearby, setNearby] = useState<NearbyFacilities | null>(() => cache.get(key) ?? null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setFailed(false);
    if (!fetchNearby || lat === null || lng === null) {
      setNearby(null);
      return;
    }
    const known = cache.get(key);
    setNearby(known ?? null);
    if (known) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      fetchNearby(lat, lng, controller.signal).then((result) => {
        cache.set(key, result);
        if (!controller.signal.aborted) setNearby(result);
      }).catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    }, delay);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [fetchNearby, key, delay]);
  return { nearby, failed };
}

interface FacilitySummaryProps {
  nearby: NearbyFacilities | null;
  failed?: boolean;
  className?: string;
}

/** Positive facilities within the radius by category, then nuisances, as on the map's cards. */
export default function FacilitySummary({ nearby, failed, className }: FacilitySummaryProps) {
  const { t } = useLanguage();
  if (failed) return null;
  if (!nearby) return <div className={cn("h-24 animate-pulse rounded-lg bg-slate-100", className)} aria-busy />;
  if (nearby.categories.length === 0) return null;

  const byCategory = new Map(nearby.categories.map((category) => [category.category, category]));
  const positive = nearby.categories.filter((category) => category.group !== "nuisance");
  const present = positive.filter((category) => category.count > 0).length;
  const nuisances = FACILITY_CATEGORIES.nuisance.map((category) => byCategory.get(category)).filter((category): category is NearbyCategory => Boolean(category && category.count > 0));

  return (
    <div className={className}>
      <div className="mb-2 flex items-center justify-between gap-2">
        <h4 className="text-xs font-medium text-slate-600">{fill(t("facilities.within"), { radius: nearby.radiusM })}</h4>
        <span className="rounded-full bg-brand-50 px-2 py-0.5 text-[10px] font-semibold text-brand-700">{fill(t("facilities.score"), { score: present, max: POSITIVE_CATEGORY_COUNT })}</span>
      </div>
      <ul className="grid grid-cols-2 gap-x-3 gap-y-1.5 text-[11px]">
        {positive.map((category) => (
          <li key={category.category} className="flex min-w-0 items-center gap-1.5">
            <span style={{ color: GROUP_COLORS[category.group] }}><FacilityIcon group={category.group} className="h-3 w-3 shrink-0" /></span>
            <span className="shrink-0 text-slate-500">{t(`facilities.category.${category.category}`)}</span>
            <span className="min-w-0 flex-1 truncate text-right tabular-nums text-slate-800" title={category.nearestName}>
              <Value category={category} />
            </span>
          </li>
        ))}
      </ul>
      <div className={cn("mt-2 flex items-start gap-1.5 rounded-lg px-2 py-1.5 text-[11px]", nuisances.length > 0 ? "bg-amber-50 text-amber-900" : "bg-slate-50 text-slate-500")}>
        <TriangleAlert className="mt-0.5 h-3 w-3 shrink-0" />
        <span>
          {nuisances.length > 0
            ? nuisances.map((category) => fill(t("facilities.nuisanceItem"), {
              category: t(`facilities.category.${category.category}`), count: category.count, distance: category.nearestM ?? 0,
            })).join(" · ")
            : fill(t("facilities.noNuisance"), { radius: nearby.radiusM })}
        </span>
      </div>
    </div>
  );
}

/** A count within the radius with the nearest distance, or else the nearest one by name. */
function Value({ category }: { category: NearbyCategory }) {
  const { t } = useLanguage();
  if (category.count > 0) {
    return <>{fill(t("facilities.count"), { count: category.count })}<span className="ml-1 text-slate-400">{category.nearestM} m</span></>;
  }
  if (category.nearestM !== null) {
    return <span className="text-slate-500">{category.nearestName ? `${category.nearestName} ` : ""}{category.nearestM} m</span>;
  }
  return <span className="text-slate-400">{t("facilities.none")}</span>;
}
