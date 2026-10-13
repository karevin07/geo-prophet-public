import { useEffect, useMemo, useRef } from "react";
import * as maplibregl from "maplibre-gl";
import { MapboxOverlay } from "@deck.gl/mapbox";
import { ScatterplotLayer } from "@deck.gl/layers";
import "maplibre-gl/dist/maplibre-gl.css";
import { applyLocalNames, MAP_STYLE } from "../../lib/basemap";
import type { NearbyCommunity } from "../../lib/api";

interface CommunityMapProps {
  community: { lat: number; lng: number };
  nearby: NearbyCommunity[];
  onSelect: (id: string) => void;
}

const COMMUNITY: [number, number, number, number] = [232, 112, 79, 240];
const NEARBY: [number, number, number, number] = [100, 116, 139, 200];

/** A community, coral, and the similar communities nearby, grey; clicking one opens it. */
export default function CommunityMap({ community, nearby, onSelect }: CommunityMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);
  const onSelectRef = useRef(onSelect);
  onSelectRef.current = onSelect;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const map = new maplibregl.Map({
      container, style: MAP_STYLE, center: [community.lng, community.lat], zoom: 15,
      dragRotate: false, pitchWithRotate: false, touchPitch: false, cooperativeGestures: true,
      attributionControl: { compact: true },
    });
    map.touchZoomRotate.disableRotation();
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

  // Frame the community and its neighbours.
  useEffect(() => {
    let west = community.lng, east = community.lng, south = community.lat, north = community.lat;
    for (const item of nearby) {
      if (item.lat === null || item.lng === null) continue;
      west = Math.min(west, item.lng); east = Math.max(east, item.lng);
      south = Math.min(south, item.lat); north = Math.max(north, item.lat);
    }
    mapRef.current?.fitBounds([[west, south], [east, north]], { padding: 28, maxZoom: 16, duration: 0 });
  }, [community, nearby]);

  const layers = useMemo(() => [
    new ScatterplotLayer<NearbyCommunity>({
      id: "nearby-communities",
      data: nearby.filter((item) => item.lat !== null && item.lng !== null),
      pickable: true,
      getPosition: (item) => [item.lng!, item.lat!],
      getRadius: 5,
      radiusUnits: "pixels",
      getFillColor: NEARBY,
      stroked: true,
      getLineColor: [255, 255, 255, 255],
      lineWidthUnits: "pixels",
      getLineWidth: 1.5,
      onClick: (info) => {
        if (info.object) onSelectRef.current(info.object.id);
      },
    }),
    new ScatterplotLayer<{ lat: number; lng: number }>({
      id: "community",
      data: [community],
      getPosition: (item) => [item.lng, item.lat],
      getRadius: 8,
      radiusUnits: "pixels",
      getFillColor: COMMUNITY,
      stroked: true,
      getLineColor: [255, 255, 255, 255],
      lineWidthUnits: "pixels",
      getLineWidth: 2,
    }),
  ], [community, nearby]);

  useEffect(() => {
    overlayRef.current?.setProps({ layers, getCursor: ({ isHovering }) => isHovering ? "pointer" : "grab" });
  }, [layers]);

  return <div ref={containerRef} className="h-full w-full overflow-hidden rounded-xl border border-slate-200" />;
}
