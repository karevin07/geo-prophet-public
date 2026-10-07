import { useEffect, useMemo, useRef } from "react";
import * as maplibregl from "maplibre-gl";
import { MapboxOverlay } from "@deck.gl/mapbox";
import { ScatterplotLayer } from "@deck.gl/layers";
import "maplibre-gl/dist/maplibre-gl.css";
import { applyLocalNames, MAP_STYLE } from "../../lib/basemap";
import type { Comparable } from "../../lib/api";

interface ComparablesMapProps {
  home: { lat: number; lng: number };
  comparables: Comparable[];
}

const SAME_BUILDING: [number, number, number, number] = [232, 112, 79, 230];
const NEARBY: [number, number, number, number] = [47, 75, 124, 200];

/** The home and its comparable sales, sized by weight; same-building sales are coral. */
export default function ComparablesMap({ home, comparables }: ComparablesMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);

  // The map is created once at the first home; later homes are framed below.
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const map = new maplibregl.Map({
      container, style: MAP_STYLE, center: [home.lng, home.lat], zoom: 15,
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

  // Frame the home and its comparables.
  useEffect(() => {
    let west = home.lng, east = home.lng, south = home.lat, north = home.lat;
    for (const item of comparables) {
      west = Math.min(west, item.lng); east = Math.max(east, item.lng);
      south = Math.min(south, item.lat); north = Math.max(north, item.lat);
    }
    mapRef.current?.fitBounds([[west, south], [east, north]], { padding: 32, maxZoom: 16, duration: 0 });
  }, [home, comparables]);

  const layers = useMemo(() => {
    const maxWeight = Math.max(...comparables.map((item) => item.weight), 0.0001);
    return [
      new ScatterplotLayer<Comparable>({
        id: "comparables",
        data: [...comparables].reverse(), // heaviest drawn last, on top
        getPosition: (item) => [item.lng, item.lat],
        getRadius: (item) => 4 + 8 * Math.sqrt(item.weight / maxWeight),
        radiusUnits: "pixels",
        getFillColor: (item) => item.sameBuilding ? SAME_BUILDING : NEARBY,
        stroked: true,
        getLineColor: [255, 255, 255, 230],
        lineWidthUnits: "pixels",
        getLineWidth: 1,
      }),
      new ScatterplotLayer<{ lat: number; lng: number }>({
        id: "home",
        data: [home],
        getPosition: (item) => [item.lng, item.lat],
        getRadius: 9,
        radiusUnits: "pixels",
        getFillColor: [31, 111, 120, 255],
        stroked: true,
        getLineColor: [255, 255, 255, 255],
        lineWidthUnits: "pixels",
        getLineWidth: 3,
      }),
    ];
  }, [home, comparables]);

  useEffect(() => {
    overlayRef.current?.setProps({ layers });
  }, [layers]);

  return <div ref={containerRef} className="h-full w-full" />;
}
