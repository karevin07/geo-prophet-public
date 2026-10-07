import { Cross, GraduationCap, ShoppingBag, TreePine, TriangleAlert } from "lucide-react";
import type { FacilityGroup } from "../lib/api";

const ICONS = { daily: ShoppingBag, education: GraduationCap, medical: Cross, leisure: TreePine, nuisance: TriangleAlert } as const;

/** A facility group's glyph, used on legends, cards and the layers panel. */
export default function FacilityIcon({ group, className }: { group: FacilityGroup; className?: string }) {
  const Icon = ICONS[group];
  return <Icon className={className} aria-hidden="true" />;
}
