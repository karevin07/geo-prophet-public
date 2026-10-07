import * as maplibregl from "maplibre-gl";
import maplibreWorkerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url";

// maplibre-gl 6 locates its worker next to its own module, which bundling breaks.
maplibregl.setWorkerUrl(maplibreWorkerUrl);

export const BASEMAPS = {
  color: "https://basemaps.cartocdn.com/gl/voyager-gl-style/style.json",
  light: "https://basemaps.cartocdn.com/gl/positron-gl-style/style.json",
} as const;
export type Basemap = keyof typeof BASEMAPS;

export const MAP_STYLE = BASEMAPS.color;

/** Voyager labels many places in English; prefer the local (Chinese) name everywhere. */
export function applyLocalNames(map: maplibregl.Map) {
  for (const layer of map.getStyle().layers ?? []) {
    if (layer.type !== "symbol") continue;
    const field = JSON.stringify(map.getLayoutProperty(layer.id, "text-field") ?? "");
    if (field.includes("name")) {
      map.setLayoutProperty(layer.id, "text-field", ["coalesce", ["get", "name"], ["get", "name_en"]]);
    }
  }
}
