import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import * as maplibregl from "maplibre-gl";
import { MapboxOverlay } from "@deck.gl/mapbox";
import { GeoJsonLayer, TextLayer } from "@deck.gl/layers";
import "maplibre-gl/dist/maplibre-gl.css";
import { applyLocalNames, MAP_STYLE } from "../lib/basemap";
import type { DistrictBoundaries } from "../lib/api";
import { labelPoint } from "../lib/district-geo";
import { cn } from "../lib/utils";

type Feature = DistrictBoundaries["features"][number];
type Color = [number, number, number, number];

/** Fill of districts without a value. */
const EMPTY_FILL: Color = [203, 213, 225, 90];

interface ChoroplethMapProps {
  boundaries: DistrictBoundaries | null;
  /** A district's fill, or null when it has no value. */
  fill: (district: string) => Color | null;
  /** Outlined districts and the hex colour of each outline. */
  selected: { district: string; color: string }[];
  onSelect: (district: string) => void;
  /** Body of the hover card under the district's name. */
  hoverContent: (district: string) => ReactNode;
  legend: ReactNode;
  /** Changes whenever fill would return something new, so deck.gl redraws. */
  fillKey: string;
  className?: string;
}

function bounds(boundaries: DistrictBoundaries): maplibregl.LngLatBoundsLike | null {
  let west = 180, south = 90, east = -180, north = -90;
  for (const feature of boundaries.features) {
    for (const polygon of feature.geometry.coordinates) {
      for (const [lng, lat] of polygon[0]) {
        west = Math.min(west, lng); east = Math.max(east, lng);
        south = Math.min(south, lat); north = Math.max(north, lat);
      }
    }
  }
  return west < east ? [[west, south], [east, north]] : null;
}

function rgba(hex: string): Color {
  return [parseInt(hex.slice(1, 3), 16), parseInt(hex.slice(3, 5), 16), parseInt(hex.slice(5, 7), 16), 255];
}

/** A city's districts on a light basemap, filled by value, with a hover card and legend. */
export default function ChoroplethMap({ boundaries, fill, selected, onSelect, hoverContent, legend, fillKey, className }: ChoroplethMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);
  const [hover, setHover] = useState<{ x: number; y: number; district: string } | null>(null);
  const onSelectRef = useRef(onSelect);
  onSelectRef.current = onSelect;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    // cooperativeGestures keeps page scrolling from zooming the map by accident.
    const map = new maplibregl.Map({
      container, style: MAP_STYLE, center: [121.5, 25.03], zoom: 10,
      dragRotate: false, pitchWithRotate: false, touchPitch: false, cooperativeGestures: true,
      attributionControl: { compact: true },
    });
    map.touchZoomRotate.disableRotation();
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-right");
    map.on("style.load", () => applyLocalNames(map));
    const overlay = new MapboxOverlay({ interleaved: false, layers: [] });
    map.addControl(overlay);
    mapRef.current = map;
    overlayRef.current = overlay;
    return () => {
      mapRef.current = null;
      overlayRef.current = null;
      map.remove();
    };
  }, []);

  // Frame the city whenever its boundaries change.
  useEffect(() => {
    const box = boundaries && bounds(boundaries);
    if (box) mapRef.current?.fitBounds(box, { padding: 24, duration: 0 });
  }, [boundaries]);

  // deck.gl compares update triggers shallowly, so selections are keyed as strings.
  const selectedKey = selected.map((item) => `${item.district}:${item.color}`).join("|");
  const layers = useMemo(() => {
    if (!boundaries) return [];
    const outlines = new Map(selected.map((item) => [item.district, rgba(item.color)]));
    return [
      new GeoJsonLayer<Feature["properties"]>({
        id: "districts",
        data: boundaries as never,
        pickable: true,
        stroked: true,
        filled: true,
        getFillColor: (feature) => fill(feature.properties.district) ?? EMPTY_FILL,
        getLineColor: (feature) => outlines.get(feature.properties.district) ?? [255, 255, 255, 230],
        getLineWidth: (feature) => outlines.has(feature.properties.district) ? 3 : 1,
        lineWidthUnits: "pixels",
        autoHighlight: true,
        highlightColor: [31, 111, 120, 70],
        onHover: (info) => setHover(info.object ? { x: info.x, y: info.y, district: info.object.properties.district } : null),
        onClick: (info) => {
          const district = info.object?.properties.district;
          if (district) onSelectRef.current(district);
        },
        updateTriggers: { getFillColor: fillKey, getLineColor: selectedKey, getLineWidth: selectedKey },
      }),
      new TextLayer<Feature>({
        id: "district-labels",
        data: boundaries.features,
        getPosition: labelPoint,
        getText: (feature) => feature.properties.district,
        getSize: 12,
        getColor: [30, 41, 59, 230],
        fontFamily: '"Noto Sans TC", sans-serif',
        characterSet: "auto",
        outlineWidth: 3,
        outlineColor: [255, 255, 255, 220],
        fontSettings: { sdf: true },
      }),
    ];
    // fill and selected are captured through their keys.
  }, [boundaries, fillKey, selectedKey]);

  useEffect(() => {
    overlayRef.current?.setProps({ layers });
  }, [layers]);

  return (
    <div className={cn("relative h-80 overflow-hidden rounded-xl border border-slate-200 sm:h-96", className)}>
      <div className="absolute inset-0">
        <div ref={containerRef} className="h-full w-full" />
      </div>

      {hover && (
        <div
          className="pointer-events-none absolute z-10 w-56 rounded-xl border border-slate-200 bg-white p-3 text-xs shadow-lg"
          // Flip to the cursor's left near the right edge so the card stays visible.
          style={{ left: hover.x > (containerRef.current?.clientWidth ?? 0) - 250 ? hover.x - 236 : hover.x + 12, top: hover.y + 12 }}
        >
          <div className="mb-1.5 font-semibold text-slate-900">{hover.district}</div>
          {hoverContent(hover.district)}
        </div>
      )}

      <div className="absolute bottom-2 left-2 z-10 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-white/90 px-2.5 py-1.5 text-[11px] text-slate-600 shadow-sm">
        {legend}
      </div>
    </div>
  );
}
