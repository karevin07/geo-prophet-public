import { tabAvailable } from "./demo";
import { tabPath, type Tab } from "./url-state";

/** Shared information architecture for desktop disclosures and the mobile drawer. */
export const NAVIGATION_GROUPS = [
  { id: "market", tabs: ["feed", "analysis", "rents", "presales"] },
  { id: "region", tabs: ["area", "districts", "stations"] },
  { id: "tools", tabs: ["investment", "estimate"] },
] as const satisfies readonly { id: string; tabs: readonly Tab[] }[];

export type NavigationGroup = (typeof NAVIGATION_GROUPS)[number]["id"];

/** The groups with the pages available here; the online demo leaves some out. */
export const VISIBLE_NAVIGATION_GROUPS = NAVIGATION_GROUPS.map((group) => ({ id: group.id, tabs: group.tabs.filter(tabAvailable) }));
export function navigationGroup(tab: Tab): NavigationGroup | undefined {
  return NAVIGATION_GROUPS.find((group) => group.tabs.some((item) => item === tab))?.id;
}
export function navigationHref(tab: Tab): string {
  return tabPath(tab);
}
