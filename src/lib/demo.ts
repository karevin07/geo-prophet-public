import type { Tab } from "./url-state";

/** The online demo: a static site reading a snapshot instead of the API (docs/DEMO.md). */
export const IS_DEMO = import.meta.env.VITE_DATA_SOURCE === "snapshot";

/** The repository the demo links to for its source and deployment guide. */
export const DEMO_REPOSITORY_URL: string = import.meta.env.VITE_DEMO_REPOSITORY_URL ?? "";

/** Pages whose data the snapshot does not hold yet. */
const DEMO_HIDDEN_TABS: readonly Tab[] = ["rents", "presales", "investment", "stations"];

export function tabAvailable(tab: Tab): boolean {
  return !IS_DEMO || !DEMO_HIDDEN_TABS.includes(tab);
}
