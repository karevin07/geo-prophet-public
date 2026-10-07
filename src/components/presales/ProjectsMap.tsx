import { useEffect, useMemo, useRef } from "react";
import * as maplibregl from "maplibre-gl";
import { MapboxOverlay } from "@deck.gl/mapbox";
import { ScatterplotLayer } from "@deck.gl/layers";
import "maplibre-gl/dist/maplibre-gl.css";
import { applyLocalNames, MAP_STYLE } from "../../lib/basemap";
import type { PresaleProject } from "../../lib/api";
import { priceColor, type PriceBreaks } from "../../lib/price-scale";

type Located = PresaleProject & { lat: number; lng: number };

interface ProjectsMapProps {
  projects: PresaleProject[];
  breaks: PriceBreaks | null;
  selected: string;
  onSelect: (key: string) => void;
}

/** Projects coloured by price per ping and sized by recent sales; the selected one is ringed. */
export default function ProjectsMap({ projects, breaks, selected, onSelect }: ProjectsMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);
  const onSelectRef = useRef(onSelect);
  onSelectRef.current = onSelect;
  const located = useMemo(() => projects.filter((project): project is Located => project.lat !== null && project.lng !== null), [projects]);

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

  // Frame the projects whenever the list changes.
  useEffect(() => {
    if (located.length === 0) return;
    let west = 180, east = -180, south = 90, north = -90;
    for (const project of located) {
      west = Math.min(west, project.lng); east = Math.max(east, project.lng);
      south = Math.min(south, project.lat); north = Math.max(north, project.lat);
    }
    mapRef.current?.fitBounds([[west, south], [east, north]], { padding: 32, maxZoom: 15, duration: 0 });
  }, [located]);

  // Pan to a project picked from the list.
  useEffect(() => {
    const project = located.find((candidate) => candidate.key === selected);
    if (project) mapRef.current?.easeTo({ center: [project.lng, project.lat], duration: 400 });
  }, [selected, located]);

  const layers = useMemo(() => {
    const maxSales = Math.max(1, ...located.map((project) => project.recentSales));
    const chosen = located.filter((project) => project.key === selected);
    return [
      new ScatterplotLayer<Located>({
        id: "projects",
        data: located,
        pickable: true,
        getPosition: (project) => [project.lng, project.lat],
        getRadius: (project) => 4 + 10 * Math.sqrt(project.recentSales / maxSales),
        radiusUnits: "pixels",
        getFillColor: (project) => project.medianPricePerPing === null ? [148, 163, 184, 200] : priceColor(project.medianPricePerPing, breaks, 210),
        stroked: true,
        getLineColor: [255, 255, 255, 230],
        lineWidthUnits: "pixels",
        getLineWidth: 1,
        onClick: (info) => {
          if (info.object) onSelectRef.current(info.object.key);
        },
        updateTriggers: { getFillColor: breaks?.join(), getRadius: maxSales },
      }),
      new ScatterplotLayer<Located>({
        id: "selected",
        data: chosen,
        getPosition: (project) => [project.lng, project.lat],
        getRadius: 16,
        radiusUnits: "pixels",
        filled: false,
        stroked: true,
        getLineColor: [31, 111, 120, 255],
        lineWidthUnits: "pixels",
        getLineWidth: 3,
      }),
    ];
  }, [located, breaks, selected]);

  useEffect(() => {
    overlayRef.current?.setProps({ layers, getCursor: ({ isHovering }) => isHovering ? "pointer" : "grab" });
  }, [layers]);

  return <div ref={containerRef} className="h-full w-full" />;
}
