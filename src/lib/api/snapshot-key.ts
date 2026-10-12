/**
 * Names the demo snapshot's saved response for an API request: a hash of the path and the
 * decoded parameters in sorted order. api/internal/snapshot.Key computes the same.
 */
export async function snapshotKey(target: string): Promise<string> {
  const url = new URL(target, "http://snapshot.invalid");
  const pairs = [...url.searchParams].map(([name, value]) => `${name}=${value}`).sort();
  const text = `${decodeURIComponent(url.pathname)}?${pairs.join("&")}`;
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text));
  return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, "0")).join("").slice(0, 20);
}
