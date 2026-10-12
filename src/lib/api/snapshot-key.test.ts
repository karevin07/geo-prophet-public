import { describe, expect, it } from "vitest";
import { snapshotKey } from "./snapshot-key";

describe("snapshotKey", () => {
  it("matches the Go snapshot's keys whatever the parameter order and encoding", async () => {
    // api/internal/snapshot/snapshot_test.go TestKey
    expect(await snapshotKey("/api/v1/presales/projects?city=a&months=12&district=%E5%A4%A7%E5%AE%89%E5%8D%80")).toBe("6c2d5a7d42e57a6b2044");
    expect(await snapshotKey(`/api/v1/presales/projects?${new URLSearchParams({ district: "大安區", city: "a", months: "12" })}`)).toBe("6c2d5a7d42e57a6b2044");
  });
});
