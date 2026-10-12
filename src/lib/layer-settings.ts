import type { FacilityGroup } from "./api";
import { IS_DEMO } from "./demo";
import type { Basemap } from "./basemap";
import { FACILITY_GROUPS } from "./facilities";

/** How the map is drawn; a per-viewer preference kept in localStorage. */
export interface LayerSettings {
  /** Hexagon and point fill opacity, 0.2–0.9. */
  opacity: number;
  basemap: Basemap;
  districts: boolean;
  /** Rail lines and stations, coloured by nearby price change. */
  transit: boolean;
  /** Facility groups shown at street zoom and counted by the density layer; empty is off. */
  facilityGroups: FacilityGroup[];
  /** Colour the map by prices or by facility density. */
  facilityView: "price" | "density";
  /** Presale projects with recent contracts. */
  presales: boolean;
}

export const DEFAULT_LAYER_SETTINGS: LayerSettings = {
  opacity: 0.5, basemap: "color", districts: false, transit: true, facilityGroups: [], facilityView: "price", presales: false,
};

const STORAGE_KEY = "geoprophet.layers";

export function loadLayerSettings(): LayerSettings {
  const settings = readLayerSettings();
  // The online demo's snapshot has no stations, presales or facilities to draw.
  return IS_DEMO ? { ...settings, transit: false, presales: false, facilityGroups: [], facilityView: "price" } : settings;
}

function readLayerSettings(): LayerSettings {
  try {
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "{}") as Partial<LayerSettings>;
    return {
      opacity: typeof stored.opacity === "number" ? Math.min(0.9, Math.max(0.2, stored.opacity)) : DEFAULT_LAYER_SETTINGS.opacity,
      basemap: stored.basemap === "light" ? "light" : "color",
      districts: stored.districts === true,
      transit: stored.transit !== false,
      facilityGroups: Array.isArray(stored.facilityGroups) ? FACILITY_GROUPS.filter((group) => stored.facilityGroups!.includes(group)) : [],
      facilityView: stored.facilityView === "density" ? "density" : "price",
      presales: stored.presales === true,
    };
  } catch {
    // Storage can be unavailable (private mode, blocked site data); fall back to defaults.
    return DEFAULT_LAYER_SETTINGS;
  }
}

export function saveLayerSettings(settings: LayerSettings) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  } catch {
    // Not persisting a preference is harmless.
  }
}
