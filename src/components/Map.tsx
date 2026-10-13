import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import * as maplibregl from "maplibre-gl";
import { MapboxOverlay } from "@deck.gl/mapbox";
import type { PickingInfo } from "@deck.gl/core";
import { GeoJsonLayer, IconLayer, PolygonLayer, ScatterplotLayer, TextLayer } from "@deck.gl/layers";
import { cellToBoundary, cellToLatLng } from "h3-js";
import { Layers, Minus, Plus } from "lucide-react";
import "maplibre-gl/dist/maplibre-gl.css";
import { applyLocalNames, BASEMAPS } from "../lib/basemap";
import { buildingCategory, type BBox, type DistrictBoundaries, type Facility, type FacilityCell, type FacilityGroup, type FacilityList, type H3Aggregate, type PresaleProject, type RentCell, type RentLayer, type StationRadius, type TransactionData, type TransitLines, type TransitStation } from "../lib/api";
import { categoriesOf, DENSITY_CLASSES, densityClass, GROUP_COLORS, groupOf, POSITIVE_CATEGORY_COUNT, rgb } from "../lib/facilities";
import type { FetchNearbyFacilities } from "./FacilitySummary";
import { labelPoint } from "../lib/district-geo";
import type { LayerSettings } from "../lib/layer-settings";
import HexCard, { type FetchCellDetail } from "./HexCard";
import RentHexCard from "./RentHexCard";
import { rentValue, type MapMetric } from "../lib/rent-scale";
import StationCard, { type FetchNearbyProjects, type FetchStationRent, type FetchStationTrend } from "./StationCard";
import YoYBadge from "./YoYBadge";
import { yoyColor } from "../lib/transit";
import LayersPanel from "./LayersPanel";
import { useLanguage } from "../lib/i18n";
import { formatWan, priceClass, priceColor, PRICE_CLASSES, type PriceBreaks } from "../lib/price-scale";
import { roundBBox } from "../lib/viewport";
import type { MapView as MapCamera } from "../lib/url-state";
import { cn, fill } from "../lib/utils";

export interface FlyTarget {
  lat: number;
  lng: number;
  zoom: number;
}

interface MapViewProps {
  data: TransactionData[];
  hexData: H3Aggregate[];
  breaks: PriceBreaks | null;
  viewMode: "hex" | "points";
  /** Camera when the map is created, e.g. restored from a shared link. */
  initialView: MapCamera;
  /** Called once the camera settles with the visible area and the camera itself. */
  onCameraChange: (bbox: BBox, view: MapCamera) => void;
  flyTarget: FlyTarget | null;
  layerSettings: LayerSettings;
  onLayerSettingsChange: (update: Partial<LayerSettings>) => void;
  /** Districts of every supported city, when the overlay is on. */
  districtBoundaries: DistrictBoundaries | null;
  fetchCellDetail: FetchCellDetail;
  onViewDistrict: (cityCode: string, district: string) => void;
  /** Rents and yields by cell, drawn instead of prices when rentMetric is set. */
  rentLayer: RentLayer | null;
  rentMetric: Exclude<MapMetric, "price"> | null;
  rentBreaks: PriceBreaks | null;
  /** Opens the rents page on a district. */
  onViewRents: (cityCode: string, district: string) => void;
  windowLabel: string;
  /** Rail routes and stations, when the transit overlay is on. */
  transitLines: TransitLines | null;
  stations: TransitStation[];
  /** Each station's 500 m YoY by station ID. */
  stationYoY: Map<string, number | null>;
  fetchStationTrend: FetchStationTrend;
  fetchStationRent: FetchStationRent;
  fetchNearbyProjects: FetchNearbyProjects;
  onShowProject: (project: { key: string; cityCode: string }) => void;
  /** Opens a community page from a sale or a hexagon; without it, communities are not linked. */
  onShowCommunity?: (id: string) => void;
  fetchFacilities: (bbox: BBox, categories: string[], signal: AbortSignal) => Promise<FacilityList>;
  fetchFacilityDensity: (bbox: BBox, resolution: 8 | 9, categories: string[], signal: AbortSignal) => Promise<FacilityCell[]>;
  fetchNearbyFacilities: FetchNearbyFacilities;
  fetchPresaleProjects: (bbox: BBox, signal: AbortSignal) => Promise<PresaleProject[]>;
  /** Whether the filter panel covers the map's left edge, where the station card opens. */
  isPanelOpen: boolean;
  /** A station to select once stations are loaded; onFocusStationHandled clears it. */
  focusStationId: string | null;
  onFocusStationHandled: () => void;
  /** Called with the selected station's ID, or null when its card closes. */
  onSelectedStationChange: (stationId: string | null) => void;
}

const CLASS_LABELS = ["legend.low", "legend.midLow", "legend.mid", "legend.high"];
/** Facilities are drawn from this zoom; names of large ones from FACILITY_LABEL_ZOOM. */
export const FACILITY_ZOOM = 15;
const FACILITY_LABEL_ZOOM = 16;
/** Presale projects are drawn from this zoom, named from PRESALE_LABEL_ZOOM. */
export const PRESALE_ZOOM = 12;
const PRESALE_LABEL_ZOOM = 15;

// Presale projects are rounded squares, unlike circular sales and facilities: a frame
// (white, or dashed dark for projects only located to a road) under a price-coloured fill.
const squareIcon = (body: string) => ({
  url: `data:image/svg+xml;charset=utf-8,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32">${body}</svg>`)}`,
  width: 32, height: 32, mask: true,
});
const PRESALE_ICONS = {
  frame: { id: "presale-frame", ...squareIcon('<rect x="1" y="1" width="30" height="30" rx="8" fill="black"/>') },
  dashed: { id: "presale-dashed", ...squareIcon('<rect x="2.5" y="2.5" width="27" height="27" rx="7" fill="none" stroke="black" stroke-width="4" stroke-dasharray="6 4"/>') },
  fill: { id: "presale-fill", ...squareIcon('<rect x="5" y="5" width="22" height="22" rx="5" fill="black"/>') },
};
const isPrecise = (project: PresaleProject) => project.geocodePrecision === "address" || project.geocodePrecision === "nearby";

/** Categories whose names are worth labelling on the map. */
const LABELLED = new Set(["market", "elementary", "junior_high", "senior_high", "university", "hospital", "park", "library", "funeral", "substation"]);
/** Station names appear from this zoom; transfer stations one level earlier. */
const LABEL_ZOOM = 13;
const TEAL: [number, number, number] = [31, 111, 120];

function hexToRgb(hex: string): [number, number, number] {
  const value = parseInt(hex.replace("#", ""), 16);
  return [(value >> 16) & 255, (value >> 8) & 255, value & 255];
}

function isTransfer(station: TransitStation) {
  return new Set(station.lines.map((line) => line.lineId)).size > 1;
}

type Hover = PickingInfo<H3Aggregate | TransactionData> & { kind: "hex" | "point" };

export default function MapView(props: MapViewProps) {
  const { data, hexData, breaks, viewMode, initialView, onCameraChange, flyTarget, layerSettings, districtBoundaries, transitLines, stations, stationYoY } = props;
  const { t, language } = useLanguage();
  const [hover, setHover] = useState<Hover | null>(null);
  const [layersOpen, setLayersOpen] = useState(false);
  // A clicked hexagon keeps its card open; its screen position follows the camera.
  const [pinned, setPinned] = useState<H3Aggregate | null>(null);
  const [pinnedPoint, setPinnedPoint] = useState<{ x: number; y: number } | null>(null);
  const [pinnedRent, setPinnedRent] = useState<RentCell | null>(null);
  const [hoverRent, setHoverRent] = useState<{ cell: RentCell; x: number; y: number } | null>(null);
  const [selectedStation, setSelectedStation] = useState<TransitStation | null>(null);
  const [stationRadius, setStationRadius] = useState<StationRadius>(500);
  const [hoverStation, setHoverStation] = useState<{ station: TransitStation; x: number; y: number } | null>(null);
  const [zoom, setZoom] = useState(initialView.zoom);
  const [viewBox, setViewBox] = useState<BBox | null>(null);
  const [facilities, setFacilities] = useState<Facility[]>([]);
  const [density, setDensity] = useState<FacilityCell[]>([]);
  const [hoverFacility, setHoverFacility] = useState<{ facility: Facility; x: number; y: number } | null>(null);
  const [hoverCell, setHoverCell] = useState<{ cell: FacilityCell; x: number; y: number } | null>(null);
  const [presaleProjects, setPresaleProjects] = useState<PresaleProject[]>([]);
  const [hoverProject, setHoverProject] = useState<{ project: PresaleProject; x: number; y: number } | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);
  const onCameraChangeRef = useRef(onCameraChange);
  onCameraChangeRef.current = onCameraChange;
  const initialViewRef = useRef(initialView);

  // deck.gl draws on top of a MapLibre base map; the map owns the (flat) camera.
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const map = new maplibregl.Map({
      container,
      style: BASEMAPS[layerSettings.basemap],
      center: [initialViewRef.current.lng, initialViewRef.current.lat],
      zoom: initialViewRef.current.zoom,
      pitch: 0,
      maxPitch: 0,
      dragRotate: false,
      pitchWithRotate: false,
      touchPitch: false,
      attributionControl: { compact: true },
    });
    map.touchZoomRotate.disableRotation();
    map.on("style.load", () => applyLocalNames(map));
    const overlay = new MapboxOverlay({ interleaved: false, layers: [] });
    map.addControl(overlay);
    mapRef.current = map;
    overlayRef.current = overlay;

    // moveend fires once the camera settles, including after load and resize.
    const reportBounds = () => {
      const bounds = map.getBounds();
      const center = map.getCenter();
      const bbox = roundBBox([bounds.getWest(), bounds.getSouth(), bounds.getEast(), bounds.getNorth()]);
      setViewBox(bbox);
      onCameraChangeRef.current(bbox, { lat: center.lat, lng: center.lng, zoom: map.getZoom() });
    };
    map.on("load", reportBounds);
    map.on("moveend", reportBounds);
    map.on("zoomend", () => setZoom(map.getZoom()));
    map.on("move", () => setCameraTick((tick) => tick + 1));
    return () => {
      mapRef.current = null;
      overlayRef.current = null;
      map.remove();
    };
  }, []);

  const [cameraTick, setCameraTick] = useState(0);
  const basemapRef = useRef(layerSettings.basemap);

  // Switching basemaps restyles MapLibre; the deck.gl overlay is unaffected.
  useEffect(() => {
    if (basemapRef.current === layerSettings.basemap) return;
    basemapRef.current = layerSettings.basemap;
    mapRef.current?.setStyle(BASEMAPS[layerSettings.basemap]);
  }, [layerSettings.basemap]);

  const pinnedHex = pinned?.hex ?? pinnedRent?.hex ?? null;
  useEffect(() => {
    const map = mapRef.current;
    if (!pinnedHex || !map) {
      setPinnedPoint(null);
      return;
    }
    const [lat, lng] = cellToLatLng(pinnedHex);
    const point = map.project([lng, lat]);
    setPinnedPoint({ x: point.x, y: point.y });
  }, [pinnedHex, cameraTick]);

  // Unpin when the grid changes shape or the pinned cell leaves the data.
  useEffect(() => {
    setPinned((current) => current && viewMode === "hex" && hexData.find((hex) => hex.hex === current.hex) || null);
  }, [hexData, viewMode]);

  useEffect(() => {
    const unpin = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setPinned(null);
      setPinnedRent(null);
      setSelectedStation(null);
    };
    window.addEventListener("keydown", unpin);
    return () => window.removeEventListener("keydown", unpin);
  }, []);

  useEffect(() => {
    if (flyTarget) mapRef.current?.flyTo({ center: [flyTarget.lng, flyTarget.lat], zoom: flyTarget.zoom, essential: true });
  }, [flyTarget]);

  // Hide the station card when the overlay is turned off.
  useEffect(() => {
    if (!layerSettings.transit) setSelectedStation(null);
  }, [layerSettings.transit]);

  // The station card covers the map's left side on wide screens; move a selected station
  // that would sit under it into the remaining area.
  const isPanelOpenRef = useRef(props.isPanelOpen);
  isPanelOpenRef.current = props.isPanelOpen;
  // Where a station should sit relative to the map centre to stay clear of its card: right
  // of the card on wide screens, above the bottom sheet on phones.
  const cardOffset = (): [number, number] => {
    if (window.innerWidth >= 640) return [((isPanelOpenRef.current ? 352 : 16) + 360) / 2, 0];
    return [0, -(containerRef.current?.clientHeight ?? 0) * 0.3];
  };
  const revealBesideCard = (station: TransitStation) => {
    const map = mapRef.current;
    if (!map || window.innerWidth < 640) return;
    const point = map.project([station.lng, station.lat]);
    if (point.x > cardOffset()[0] * 2 + 48) return;
    map.easeTo({ center: [station.lng, station.lat], offset: cardOffset(), duration: 600 });
  };

  // Open a station requested from elsewhere (a shared link, the search box or the station
  // ranking) once stations are loaded, flying to it beside its card.
  const { focusStationId, onFocusStationHandled, onSelectedStationChange } = props;
  useEffect(() => {
    const station = focusStationId && stations.find((candidate) => candidate.id === focusStationId);
    const map = mapRef.current;
    if (!station || !map) return;
    setSelectedStation(station);
    onFocusStationHandled();
    const fly = () => map.flyTo({ center: [station.lng, station.lat], zoom: Math.max(map.getZoom(), 15), offset: cardOffset(), essential: true });
    if (map.loaded()) fly();
    else map.once("load", fly);
  }, [focusStationId, stations]);

  useEffect(() => {
    onSelectedStationChange(selectedStation?.id ?? null);
  }, [selectedStation?.id]);

  // Facilities of the chosen groups at street zoom, and the density of the view.
  const facilityCategories = useMemo(() => categoriesOf(layerSettings.facilityGroups), [layerSettings.facilityGroups]);
  const { fetchFacilities, fetchFacilityDensity } = props;
  const showFacilities = facilityCategories.length > 0 && zoom >= FACILITY_ZOOM;
  const showDensity = layerSettings.facilityView === "density" && facilityCategories.length > 0;
  const densityResolution: 8 | 9 = zoom >= 11 ? 9 : 8;
  useEffect(() => {
    if (!showFacilities || !viewBox) {
      setFacilities([]);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      fetchFacilities(viewBox, facilityCategories, controller.signal).then((list) => setFacilities(list.facilities)).catch(() => undefined);
    }, 150);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [showFacilities, viewBox, facilityCategories, fetchFacilities]);
  useEffect(() => {
    if (!showDensity || !viewBox) {
      setDensity([]);
      return;
    }
    // Wider views would read too many facilities; they show no density.
    if (viewBox[2] - viewBox[0] > 1.2 || viewBox[3] - viewBox[1] > 1.2) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      fetchFacilityDensity(viewBox, densityResolution, facilityCategories, controller.signal).then(setDensity).catch(() => undefined);
    }, 150);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [showDensity, viewBox, densityResolution, facilityCategories, fetchFacilityDensity]);
  const showPresales = layerSettings.presales && zoom >= PRESALE_ZOOM;
  const { fetchPresaleProjects } = props;
  useEffect(() => {
    if (!showPresales || !viewBox) {
      setPresaleProjects([]);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      fetchPresaleProjects(viewBox, controller.signal).then(setPresaleProjects).catch(() => undefined);
    }, 150);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [showPresales, viewBox, fetchPresaleProjects]);
  const onShowProjectRef = useRef(props.onShowProject);
  onShowProjectRef.current = props.onShowProject;
  const onShowCommunityRef = useRef(props.onShowCommunity);
  onShowCommunityRef.current = props.onShowCommunity;
  const presaleLayers = useMemo(() => {
    const located = presaleProjects.filter((project): project is PresaleProject & { lat: number; lng: number } => project.lat !== null && project.lng !== null);
    if (located.length === 0) return [];
    const position = (project: PresaleProject): [number, number] => [project.lng!, project.lat!];
    // Label the busiest projects first and skip names that would overlap one already
    // placed, judged in screen pixels at the current zoom.
    const labelled: PresaleProject[] = [];
    const map = mapRef.current;
    if (map && zoom >= PRESALE_LABEL_ZOOM) {
      const placed: { x: number; y: number; width: number }[] = [];
      for (const project of [...located].sort((a, b) => b.recentSales - a.recentSales)) {
        const point = map.project(position(project));
        const width = project.name.length * 11 + 8;
        if (placed.every((other) => Math.abs(other.y - point.y) > 16 || Math.abs(other.x - point.x) > (other.width + width) / 2)) {
          placed.push({ x: point.x, y: point.y, width });
          labelled.push(project);
        }
      }
    }
    return [
      new IconLayer<PresaleProject>({
        id: "presale-frames",
        data: located,
        getPosition: position,
        getIcon: (project) => isPrecise(project) ? PRESALE_ICONS.frame : PRESALE_ICONS.dashed,
        getSize: 22,
        sizeUnits: "pixels",
        getColor: (project) => isPrecise(project) ? [255, 255, 255, 255] : [51, 65, 85, 255],
      }),
      new IconLayer<PresaleProject>({
        id: "presale-fills",
        data: located,
        pickable: true,
        getPosition: position,
        getIcon: () => PRESALE_ICONS.fill,
        getSize: 22,
        sizeUnits: "pixels",
        getColor: (project) => project.medianPricePerPing === null ? [148, 163, 184, 255] : priceColor(project.medianPricePerPing, breaks, 255),
        onHover: (info) => setHoverProject(info.object ? { project: info.object, x: info.x, y: info.y } : null),
        onClick: (info) => {
          if (info.object) onShowProjectRef.current(info.object);
        },
        updateTriggers: { getColor: [breaks] },
      }),
      new ScatterplotLayer<PresaleProject>({
        id: "presale-dots",
        data: located,
        getPosition: position,
        getRadius: 1.8,
        radiusUnits: "pixels",
        getFillColor: [30, 41, 59, 230],
      }),
      new TextLayer<PresaleProject>({
        id: "presale-names",
        data: labelled,
        getPosition: position,
        getText: (project) => project.name,
        getSize: 11,
        getColor: [30, 41, 59, 240],
        getPixelOffset: [0, -17],
        fontFamily: '"Noto Sans TC", sans-serif',
        fontWeight: 600,
        characterSet: "auto",
        outlineWidth: 3,
        outlineColor: [255, 255, 255, 235],
        fontSettings: { sdf: true },
      }),
    ];
  }, [presaleProjects, breaks, zoom]);

  const facilityCounts = useMemo(() => {
    const counts: Partial<Record<FacilityGroup, number>> = {};
    if (!showFacilities) return counts;
    for (const group of layerSettings.facilityGroups) counts[group] = 0;
    for (const facility of facilities) {
      const group = groupOf(facility.category);
      if (group) counts[group] = (counts[group] ?? 0) + 1;
    }
    return counts;
  }, [facilities, showFacilities, layerSettings.facilityGroups]);

  const densityLayers = useMemo(() => {
    if (!showDensity) return [];
    return [
      new PolygonLayer<FacilityCell>({
        id: "facility-density",
        data: density,
        pickable: true,
        filled: true,
        stroked: true,
        getPolygon: (cell) => cellToBoundary(cell.hex, true),
        getFillColor: (cell) => rgb(DENSITY_CLASSES[densityClass(cell.score)].hex, cell.score > 0 ? 180 : 0),
        getLineColor: [255, 255, 255, 150],
        lineWidthMinPixels: 0.5,
        autoHighlight: true,
        highlightColor: [31, 111, 120, 90],
        onHover: (info) => setHoverCell(info.object ? { cell: info.object, x: info.x, y: info.y } : null),
      }),
      new ScatterplotLayer<FacilityCell>({
        id: "facility-nuisance-dots",
        data: zoom >= 14 ? density.filter((cell) => cell.nuisances > 0) : [],
        getPosition: (cell) => { const [lat, lng] = cellToLatLng(cell.hex); return [lng, lat]; },
        getRadius: 2.5,
        radiusUnits: "pixels",
        getFillColor: rgb(GROUP_COLORS.nuisance, 220),
      }),
    ];
  }, [showDensity, density, zoom]);

  const facilityLayers = useMemo(() => {
    if (!showFacilities) return [];
    return [
      new ScatterplotLayer<Facility>({
        id: "facilities",
        data: facilities,
        pickable: true,
        getPosition: (facility) => [facility.lng, facility.lat],
        getRadius: (facility) => facility.category === "park" || facility.category === "hospital" ? 7 : 5.5,
        radiusUnits: "pixels",
        getFillColor: (facility) => rgb(GROUP_COLORS[groupOf(facility.category) ?? "daily"], 235),
        stroked: true,
        getLineColor: (facility) => groupOf(facility.category) === "nuisance" ? [251, 191, 36, 255] : [255, 255, 255, 255],
        getLineWidth: (facility) => groupOf(facility.category) === "nuisance" ? 2 : 1.5,
        lineWidthUnits: "pixels",
        autoHighlight: true,
        highlightColor: [31, 111, 120, 200],
        onHover: (info) => setHoverFacility(info.object ? { facility: info.object, x: info.x, y: info.y } : null),
      }),
      new TextLayer<Facility>({
        id: "facility-names",
        data: zoom >= FACILITY_LABEL_ZOOM ? facilities.filter((facility) => facility.name && LABELLED.has(facility.category)) : [],
        getPosition: (facility) => [facility.lng, facility.lat],
        getText: (facility) => facility.name,
        getSize: 11,
        getColor: (facility) => rgb(GROUP_COLORS[groupOf(facility.category) ?? "daily"], 255),
        getPixelOffset: [0, -13],
        fontFamily: '"Noto Sans TC", sans-serif',
        fontWeight: 500,
        characterSet: "auto",
        outlineWidth: 3,
        outlineColor: [255, 255, 255, 230],
        fontSettings: { sdf: true },
      }),
    ];
  }, [showFacilities, facilities, zoom]);

  // deck.gl compares update triggers by value, and every Map looks alike, so key the
  // station colours by their contents.
  const yoyKey = useMemo(() => [...stationYoY].map(([id, value]) => `${id}:${value}`).join(","), [stationYoY]);

  // Rail lines, distance rings of the selected station, and stations ringed by their
  // 500 m price change, drawn above the price layer.
  const transitLayers = useMemo(() => {
    if (!layerSettings.transit) return [];
    const labelled = stations.filter((station) => zoom >= LABEL_ZOOM || (zoom >= LABEL_ZOOM - 1 && isTransfer(station)));
    return [
      transitLines && new GeoJsonLayer<TransitLines["features"][number]["properties"]>({
        id: "transit-lines",
        data: transitLines as never,
        stroked: true,
        filled: false,
        getLineColor: (feature) => [...hexToRgb(feature.properties.color), 230],
        getLineWidth: (feature) => feature.properties.system === "tra" || feature.properties.system === "thsr" ? 2.5 : 3,
        lineWidthUnits: "pixels",
        lineCapRounded: true,
        lineJointRounded: true,
      }),
      selectedStation && new ScatterplotLayer<StationRadius>({
        id: "station-rings",
        data: [800, 500, 300],
        getPosition: () => [selectedStation.lng, selectedStation.lat],
        getRadius: (radius) => radius,
        radiusUnits: "meters",
        stroked: true,
        filled: true,
        getFillColor: (radius) => radius === stationRadius ? [...TEAL, 28] : [0, 0, 0, 0],
        getLineColor: (radius) => [...TEAL, radius === stationRadius ? 255 : 110],
        getLineWidth: (radius) => radius === stationRadius ? 2 : 1,
        lineWidthUnits: "pixels",
        updateTriggers: { getFillColor: stationRadius, getLineColor: stationRadius, getLineWidth: stationRadius },
      }),
      new ScatterplotLayer<TransitStation>({
        id: "station-yoy",
        data: stations,
        getPosition: (station) => [station.lng, station.lat],
        getRadius: (station) => isTransfer(station) ? 9 : 7.5,
        radiusUnits: "pixels",
        stroked: true,
        filled: false,
        getLineColor: (station) => yoyColor(stationYoY.get(station.id)),
        lineWidthUnits: "pixels",
        getLineWidth: 2.5,
        updateTriggers: { getLineColor: yoyKey },
      }),
      new ScatterplotLayer<TransitStation>({
        id: "stations",
        data: stations,
        pickable: true,
        getPosition: (station) => [station.lng, station.lat],
        getRadius: (station) => isTransfer(station) ? 5.5 : 4,
        radiusUnits: "pixels",
        stroked: true,
        filled: true,
        getFillColor: (station) => station.id === selectedStation?.id ? [...TEAL, 255] : [255, 255, 255, 255],
        getLineColor: [30, 41, 59, 255],
        getLineWidth: (station) => isTransfer(station) ? 2 : 1.5,
        lineWidthUnits: "pixels",
        autoHighlight: true,
        highlightColor: [...TEAL, 200],
        onHover: (info) => setHoverStation(info.object ? { station: info.object, x: info.x, y: info.y } : null),
        onClick: (info) => {
          const station = info.object;
          if (!station) return;
          setSelectedStation((current) => current?.id === station.id ? null : station);
          revealBesideCard(station);
        },
        updateTriggers: { getFillColor: selectedStation?.id },
      }),
      new TextLayer<TransitStation>({
        id: "station-names",
        data: labelled,
        getPosition: (station) => [station.lng, station.lat],
        getText: (station) => station.name,
        getSize: (station) => isTransfer(station) ? 12 : 11,
        getColor: [30, 41, 59, 240],
        getPixelOffset: [0, -14],
        fontFamily: '"Noto Sans TC", sans-serif',
        fontWeight: 500,
        characterSet: "auto",
        outlineWidth: 3,
        outlineColor: [255, 255, 255, 230],
        fontSettings: { sdf: true },
      }),
    ].filter(Boolean);
  }, [layerSettings.transit, transitLines, stations, stationYoY, yoyKey, selectedStation, stationRadius, zoom]);

  const fillAlpha = Math.round(layerSettings.opacity * 255);
  const { rentLayer, rentMetric, rentBreaks } = props;
  const showRent = rentMetric !== null && !showDensity;
  // Cells without the value shown (a yield needs sales too) are left out.
  const rentCells = useMemo(
    () => rentLayer && rentMetric ? rentLayer.cells.filter((cell) => rentValue(cell, rentMetric) !== null) : [],
    [rentLayer, rentMetric],
  );
  // Keep a pinned rent cell while it is still drawn, with its new figures.
  useEffect(() => {
    setPinnedRent((current) => current && showRent && rentCells.find((cell) => cell.hex === current.hex) || null);
  }, [rentCells, showRent]);
  const rentLayers = useMemo(() => {
    if (!showRent || !rentMetric) return [];
    return [
      new PolygonLayer<RentCell>({
        id: "rent-hexagons",
        data: rentCells,
        pickable: true,
        filled: true,
        stroked: true,
        getPolygon: (cell) => cellToBoundary(cell.hex, true),
        getFillColor: (cell) => priceColor(rentValue(cell, rentMetric)!, rentBreaks, fillAlpha),
        getLineColor: [255, 255, 255, 180],
        lineWidthMinPixels: 1,
        autoHighlight: true,
        highlightColor: [31, 111, 120, 90],
        onHover: (info) => setHoverRent(info.object ? { cell: info.object, x: info.x, y: info.y } : null),
        onClick: (info) => {
          if (info.object) setPinnedRent((current) => current?.hex === info.object!.hex ? null : info.object!);
        },
        updateTriggers: { getFillColor: [rentBreaks, fillAlpha, rentMetric] },
      }),
      pinnedRent && new PolygonLayer<RentCell>({
        id: "pinned-rent-hexagon",
        data: [pinnedRent],
        filled: false,
        stroked: true,
        getPolygon: (cell) => cellToBoundary(cell.hex, true),
        getLineColor: [31, 111, 120, 255],
        lineWidthMinPixels: 2.5,
      }),
    ].filter(Boolean);
  }, [showRent, rentMetric, rentCells, rentBreaks, fillAlpha, pinnedRent]);
  const layers = useMemo(() => {
    const dataLayer = viewMode === "hex"
      ? new PolygonLayer<H3Aggregate>({
          id: "h3-hexagons",
          data: hexData,
          pickable: true,
          filled: true,
          stroked: true,
          extruded: false,
          getPolygon: (d) => cellToBoundary(d.hex, true),
          getFillColor: (d) => priceColor(d.avgPricePerPing, breaks, fillAlpha),
          getLineColor: [255, 255, 255, 180],
          lineWidthMinPixels: 1,
          autoHighlight: true,
          highlightColor: [31, 111, 120, 90],
          onHover: (info) => setHover(info.object ? { ...info, kind: "hex" } : null),
          onClick: (info) => {
            if (info.object) setPinned((current) => current?.hex === info.object!.hex ? null : info.object!);
          },
          updateTriggers: { getFillColor: [breaks, fillAlpha] },
        })
      : new ScatterplotLayer<TransactionData>({
          id: "transactions",
          data,
          pickable: true,
          stroked: true,
          filled: true,
          radiusUnits: "pixels",
          getRadius: (d) => 3 + Math.sqrt(d.buildingAreaPing) * 0.6,
          getPosition: (d) => [d.lng, d.lat],
          getFillColor: (d) => priceColor(d.pricePerPing, breaks, Math.min(255, fillAlpha + 60)),
          getLineColor: [255, 255, 255, 230],
          lineWidthMinPixels: 1,
          autoHighlight: true,
          highlightColor: [31, 111, 120, 160],
          onHover: (info) => setHover(info.object ? { ...info, kind: "point" } : null),
          onClick: (info) => {
            if (info.object?.communityId) onShowCommunityRef.current?.(info.object.communityId);
          },
          updateTriggers: { getFillColor: [breaks, fillAlpha] },
        });
    const pinnedLayer = pinned && new PolygonLayer<H3Aggregate>({
      id: "pinned-hexagon",
      data: [pinned],
      filled: false,
      stroked: true,
      getPolygon: (d) => cellToBoundary(d.hex, true),
      getLineColor: [31, 111, 120, 255],
      lineWidthMinPixels: 2.5,
    });
    const districtLayers = layerSettings.districts && districtBoundaries ? [
      new GeoJsonLayer({
        id: "district-outlines",
        data: districtBoundaries as never,
        stroked: true,
        filled: false,
        getLineColor: [51, 65, 85, 190],
        lineWidthMinPixels: 1.2,
      }),
      new TextLayer<DistrictBoundaries["features"][number]>({
        id: "district-names",
        data: districtBoundaries.features,
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
    ] : [];
    // The density and rent views replace the price layer; facilities sit above everything.
    const base = showDensity ? densityLayers : showRent ? rentLayers : [dataLayer];
    return [...base, ...districtLayers, showDensity || showRent ? null : pinnedLayer, ...transitLayers, ...facilityLayers, ...presaleLayers].filter(Boolean);
  }, [data, hexData, breaks, viewMode, fillAlpha, pinned, layerSettings.districts, districtBoundaries, transitLayers, densityLayers, rentLayers, facilityLayers, presaleLayers, showDensity, showRent]);

  // The price card has nothing to show over the density or rent layers.
  useEffect(() => {
    if (showDensity || showRent) setPinned(null);
  }, [showDensity, showRent]);

  useEffect(() => {
    overlayRef.current?.setProps({ layers });
    // The hovered object may no longer be drawn (new data, view mode or colour classes).
    setHover(null);
    setHoverRent(null);
  }, [layers]);

  return (
    <div className="relative h-full w-full bg-canvas">
      {/* maplibre-gl.css forces position: relative on the map element, so it fills a positioned wrapper. */}
      <div className="absolute inset-0">
        <div ref={containerRef} className="h-full w-full" />
      </div>

      <div className="absolute right-3 top-24 z-30 flex flex-col gap-2 sm:right-4 sm:top-4">
        <div className="flex flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.08)]">
          <MapButton label={t("map.zoomIn")} onClick={() => mapRef.current?.zoomIn()}><Plus className="h-4 w-4" /></MapButton>
          <div className="h-px bg-slate-100" />
          <MapButton label={t("map.zoomOut")} onClick={() => mapRef.current?.zoomOut()}><Minus className="h-4 w-4" /></MapButton>
        </div>
        <div className="relative">
          <div className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_4px_16px_rgba(15,23,42,0.08)]">
            <MapButton label={t("map.layers")} onClick={() => setLayersOpen((open) => !open)} active={layersOpen}><Layers className="h-4 w-4" /></MapButton>
          </div>
          {layersOpen && <LayersPanel settings={layerSettings} onChange={props.onLayerSettingsChange} onClose={() => setLayersOpen(false)} facilityCounts={facilityCounts}
            presaleCount={showPresales ? presaleProjects.filter((project) => project.lat !== null).length : null}
            rentThrough={rentLayer?.through || null} />}
        </div>
      </div>

      {selectedStation && (
        <StationCard
          station={selectedStation} radius={stationRadius} onRadiusChange={setStationRadius}
          windowLabel={props.windowLabel} fetchTrend={props.fetchStationTrend} fetchRent={props.fetchStationRent}
          fetchNearbyProjects={props.fetchNearbyProjects} onShowProject={props.onShowProject}
          fetchNearbyFacilities={props.fetchNearbyFacilities} onClose={() => setSelectedStation(null)}
          className={cn(
            "absolute inset-x-3 bottom-3 z-40 max-h-[70%] sm:inset-x-auto sm:bottom-auto sm:top-4 sm:max-h-[calc(100%-7rem)] sm:w-[360px]",
            props.isPanelOpen ? "sm:left-[22rem]" : "sm:left-4",
          )}
        />
      )}
      {hoverStation && hoverStation.station.id !== selectedStation?.id && (
        <div
          className="pointer-events-none absolute z-40 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs shadow-lg"
          style={{ left: hoverStation.x + 12, top: hoverStation.y + 12 }}
        >
          <div className="font-semibold text-slate-900">{hoverStation.station.name}</div>
          <div className="mt-1 flex items-center gap-1.5 text-slate-500">
            500 m <YoYBadge value={stationYoY.get(hoverStation.station.id)} suffix={t("yoy.label")} />
          </div>
        </div>
      )}
      {pinned && pinnedPoint && (
        <HexCard
          hex={pinned} breaks={breaks} x={pinnedPoint.x} y={pinnedPoint.y} pinned
          windowLabel={props.windowLabel} fetchDetail={props.fetchCellDetail} fetchNearby={props.fetchNearbyFacilities}
          onClose={() => setPinned(null)} onViewDistrict={props.onViewDistrict} onShowCommunity={props.onShowCommunity}
        />
      )}
      {showRent && rentMetric && rentLayer && pinnedRent && pinnedPoint && (
        <RentHexCard
          cell={pinnedRent} metric={rentMetric} breaks={rentBreaks} through={rentLayer.through} minimumSample={rentLayer.minimumSample}
          x={pinnedPoint.x} y={pinnedPoint.y} pinned fetchDetail={props.fetchCellDetail}
          onClose={() => setPinnedRent(null)} onViewRents={props.onViewRents}
        />
      )}
      {showRent && rentMetric && rentLayer && hoverRent && !pinnedRent && (
        <RentHexCard
          cell={hoverRent.cell} metric={rentMetric} breaks={rentBreaks} through={rentLayer.through} minimumSample={rentLayer.minimumSample}
          x={hoverRent.x} y={hoverRent.y} pinned={false} fetchDetail={props.fetchCellDetail}
          onClose={() => setHoverRent(null)} onViewRents={props.onViewRents}
        />
      )}
      {hover?.object && hover.kind === "hex" && !pinned && !showDensity && !showRent && (
        <HexCard
          hex={hover.object as H3Aggregate} breaks={breaks} x={hover.x} y={hover.y} pinned={false}
          windowLabel={props.windowLabel} fetchDetail={props.fetchCellDetail} fetchNearby={props.fetchNearbyFacilities}
          onClose={() => setHover(null)} onViewDistrict={props.onViewDistrict}
        />
      )}
      {hoverProject && (
        <div className="pointer-events-none absolute z-40 w-64 rounded-xl border border-slate-200 bg-white p-3 text-xs shadow-lg" style={{ left: hoverProject.x + 14, top: hoverProject.y + 14 }}>
          <div className="flex items-center gap-1.5">
            {hoverProject.project.medianPricePerPing !== null && <span className="h-2.5 w-2.5 shrink-0 rounded-sm" style={{ backgroundColor: PRICE_CLASSES[priceClass(hoverProject.project.medianPricePerPing, breaks)].hex }} />}
            <span className="truncate text-sm font-semibold text-slate-900">{hoverProject.project.name}</span>
            <span className="ml-auto shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-[10px] text-slate-500">{t("presales.badge")}</span>
          </div>
          <p className="mt-0.5 text-[11px] text-slate-500">
            {[hoverProject.project.district, hoverProject.project.category, hoverProject.project.totalFloors && fill(t("presales.floors"), { floors: hoverProject.project.totalFloors }),
              `${t("presales.location")} ${t(`presales.precision.${hoverProject.project.geocodePrecision || "district"}`)}`].filter(Boolean).join(" · ")}
          </p>
          <dl className="mt-2 space-y-1 border-t border-slate-100 pt-2">
            <div className="flex justify-between gap-2"><dt className="text-slate-500">{t("presales.stat.price")}</dt><dd className="font-semibold tabular-nums text-slate-900">{hoverProject.project.medianPricePerPing === null ? "—" : `${formatWan(hoverProject.project.medianPricePerPing)} ${t("chart.unit")}`}</dd></div>
            <div className="flex justify-between gap-2"><dt className="text-slate-500">{t("presales.stat.recent")}</dt><dd className="tabular-nums text-slate-800">{hoverProject.project.recentSales}</dd></div>
            <div className="flex justify-between gap-2"><dt className="text-slate-500">{t("presales.stat.sales")}</dt><dd className="tabular-nums text-slate-800">{hoverProject.project.sales} · {fill(t("presales.cancelledShort"), { count: hoverProject.project.cancelled })}</dd></div>
            <div className="flex justify-between gap-2"><dt className="text-slate-500">{t("presales.lastContract")}</dt><dd className="tabular-nums text-slate-800">{hoverProject.project.lastContract.slice(0, 7)}</dd></div>
          </dl>
          <p className="mt-2 font-medium text-brand-700">{t("presales.openProject")}</p>
        </div>
      )}
      {hoverFacility && (
        <div className="pointer-events-none absolute z-40 max-w-56 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs shadow-lg" style={{ left: hoverFacility.x + 12, top: hoverFacility.y + 12 }}>
          <div className="font-semibold text-slate-900">{hoverFacility.facility.name || t(`facilities.category.${hoverFacility.facility.category}`)}</div>
          <div className="mt-0.5 text-slate-500">{t(`facilities.category.${hoverFacility.facility.category}`)}{hoverFacility.facility.address ? ` · ${hoverFacility.facility.address}` : ""}</div>
        </div>
      )}
      {hoverCell && showDensity && (
        <div className="pointer-events-none absolute z-40 w-56 rounded-xl border border-slate-200 bg-white p-3 text-xs shadow-lg" style={{ left: hoverCell.x + 14, top: hoverCell.y + 14 }}>
          <div className="mb-1.5 font-semibold text-slate-900">{fill(t("facilities.score"), { score: hoverCell.cell.score, max: POSITIVE_CATEGORY_COUNT })}</div>
          <dl className="space-y-1">
            {(["daily", "education", "medical", "leisure"] as const).map((group) => (
              <div key={group} className="flex justify-between gap-2">
                <dt className="flex items-center gap-1.5 text-slate-500"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: GROUP_COLORS[group] }} />{t(`facilities.group.${group}`)}</dt>
                <dd className="tabular-nums text-slate-800">{hoverCell.cell.groups[group] ?? 0} / {categoriesOf([group]).length}</dd>
              </div>
            ))}
            <div className={cn("flex justify-between gap-2 rounded px-1 -mx-1", hoverCell.cell.nuisances > 0 && "bg-amber-50")}>
              <dt className="flex items-center gap-1.5 text-slate-500"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: GROUP_COLORS.nuisance }} />{t("facilities.group.nuisance")}</dt>
              <dd className="tabular-nums text-slate-800">{fill(t("facilities.count"), { count: hoverCell.cell.nuisances })}</dd>
            </div>
          </dl>
          <p className="mt-1.5 text-[10px] text-slate-400">{t("facilities.cellNote")}</p>
        </div>
      )}
      {hover?.object && hover.kind === "point" && !showDensity && !showRent && (
        <HoverCard hover={hover} breaks={breaks} isZh={language === "zh"} t={t} linksCommunities={Boolean(props.onShowCommunity)} />
      )}
    </div>
  );
}

function MapButton({ label, onClick, active, children }: { label: string; onClick: () => void; active?: boolean; children: ReactNode }) {
  return (
    <button
      onClick={onClick}
      aria-label={label}
      title={label}
      className={cn("flex h-10 w-10 items-center justify-center text-slate-600 hover:bg-slate-50 hover:text-slate-900", active && "text-brand-600")}
    >
      {children}
    </button>
  );
}

function HoverCard({ hover, breaks, isZh, t, linksCommunities }: { hover: Hover; breaks: PriceBreaks | null; isZh: boolean; t: (key: string) => string; linksCommunities: boolean }) {
  const rows: [string, ReactNode][] = [];
  let title: string;
  let subtitle: string | null = null;
  let footer: string | null = null;
  let pricePerPing: number;

  if (hover.kind === "hex") {
    const hex = hover.object as H3Aggregate;
    pricePerPing = hex.avgPricePerPing;
    title = t(CLASS_LABELS[priceClass(pricePerPing, breaks)]);
    rows.push(
      [t("map.avgPrice"), <><b className="text-base text-slate-900">{formatWan(pricePerPing)}</b> 萬/坪</>],
      [t("map.avgTotal"), `${formatWan(hex.price, 0)} 萬`],
      [t("map.count"), `${hex.count.toLocaleString()} ${isZh ? "筆" : ""}`],
      [t("map.avgArea"), `${hex.avgArea.toFixed(1)} 坪`],
    );
  } else {
    const sale = hover.object as TransactionData;
    pricePerPing = sale.pricePerPing;
    title = sale.address;
    subtitle = `${t("map.transactedAt")} ${sale.transactedAt.slice(0, 10)}${sale.geocodePrecision !== "address" ? ` · ${t(`precision.${sale.geocodePrecision}`)}` : ""}`;
    rows.push(
      [t("map.pricePerPing"), <><b className="text-base text-slate-900">{formatWan(pricePerPing)}</b> 萬/坪</>],
      [t("map.price"), `${formatWan(sale.totalPrice, 0)} 萬`],
    );
    if (sale.parkingPrice > 0) rows.push([t("map.parking"), `${formatWan(sale.parkingPrice, 0)} 萬`]);
    rows.push(
      [t("map.area"), `${sale.buildingAreaPing.toFixed(1)} 坪`],
      [t("map.layout"), isZh ? `${sale.rooms}房${sale.halls}廳${sale.baths}衛` : `${sale.rooms}BR ${sale.halls}LR ${sale.baths}BA`],
      [t("map.type"), t(`type.${buildingCategory(sale.buildingType)}`)],
      [t("map.floor"), `${sale.floorLabel || "-"}${sale.totalFloors ? ` / ${sale.totalFloors}` : ""}`],
    );
    if (linksCommunities && sale.communityId) footer = t("map.pointCommunity");
  }

  // Keep the card inside the viewport by flipping it to the cursor's other side.
  const flipX = typeof window !== "undefined" && hover.x > window.innerWidth - 340;
  const flipY = typeof window !== "undefined" && hover.y > window.innerHeight - 280;
  const color = PRICE_CLASSES[priceClass(pricePerPing, breaks)].hex;

  return (
    <div
      className="pointer-events-none absolute z-40 w-72 rounded-xl border border-slate-200 bg-white p-4 shadow-xl"
      style={{
        left: flipX ? undefined : hover.x + 16,
        right: flipX ? `calc(100% - ${hover.x - 16}px)` : undefined,
        top: flipY ? undefined : hover.y + 16,
        bottom: flipY ? `calc(100% - ${hover.y - 16}px)` : undefined,
      }}
    >
      <div className="mb-3 border-b border-slate-100 pb-2">
        <div className="flex items-center gap-2">
          <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: color }} />
          <h3 className="text-sm font-semibold leading-snug text-slate-900">{title}</h3>
        </div>
        {subtitle && <p className="mt-1 text-[11px] text-slate-500">{subtitle}</p>}
      </div>
      <dl className="space-y-1.5 text-xs">
        {rows.map(([label, value]) => (
          <div key={label} className="flex items-baseline justify-between gap-3">
            <dt className="text-slate-500">{label}</dt>
            <dd className="tabular-nums text-slate-700">{value}</dd>
          </div>
        ))}
      </dl>
      {footer && <p className="mt-3 border-t border-slate-100 pt-2 text-[11px] text-brand-700">{footer}</p>}
    </div>
  );
}
