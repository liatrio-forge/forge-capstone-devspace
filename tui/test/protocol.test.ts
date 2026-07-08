import { describe, expect, test } from "bun:test";
import { PROTOCOL_VERSION, helloProblem, isHello, isServerEvent, isSnapshot, isSyncStatus, isWorkspaceOverview } from "../src/protocol";

function fixture(name: string): Promise<unknown> {
  return Bun.file(new URL(`./fixtures/${name}`, import.meta.url)).json();
}

describe("protocol fixtures", () => {
  test("hello matches the TypeScript protocol version", async () => {
    const hello = await fixture("hello.json");
    expect(isHello(hello)).toBe(true);
    if (!isHello(hello)) throw new Error("invalid hello fixture");
    expect(hello.protocol).toBe(PROTOCOL_VERSION);
  });

  test("snapshot matches the rendered row contract", async () => {
    const snapshot = await fixture("snapshot.json");
    expect(isSnapshot(snapshot)).toBe(true);
    if (!isSnapshot(snapshot)) throw new Error("invalid snapshot fixture");
    expect(snapshot.rows).toHaveLength(2);
    for (const row of snapshot.rows) {
      expect(typeof row.name).toBe("string");
      expect(typeof row.status).toBe("string");
    }
  });

  test("snapshot accepts advisory warnings", async () => {
    const snapshot = await fixture("snapshot.json");
    expect(isSnapshot({ ...(snapshot as object), warnings: ["Access role advisory"] })).toBe(true);
    expect(isSnapshot({ ...(snapshot as object), warnings: [42] })).toBe(false);
  });

  test("sync status matches the client contract", async () => {
    expect(isSyncStatus(await fixture("sync-status.json"))).toBe(true);
  });

  test("workspace overview matches the client contract", async () => {
    expect(isWorkspaceOverview(await fixture("workspace.json"))).toBe(true);
  });

  test("watch events match the reducer contract", async () => {
    for (const name of ["event-watch-refresh.json", "event-watch-error.json"]) {
      const event = await fixture(name);
      expect(typeof event).toBe("object");
      expect(event).not.toBeNull();
      expect(isServerEvent((event as { params?: unknown }).params)).toBe(true);
    }
  });

  test("error responses expose id and message", async () => {
    const response = await fixture("response-error.json");
    expect(response).toEqual({ id: 7, error: { message: "nope" } });
  });

  test("hello rejects missing required fields", async () => {
    const hello = (await fixture("hello.json")) as Record<string, unknown>;
    const broken = { ...hello };
    delete broken.workspaceRoot;
    expect(isHello(broken)).toBe(false);
  });

  test("workspace overview rejects missing required fields", async () => {
    const workspace = (await fixture("workspace.json")) as Record<string, unknown>;
    const broken = { ...workspace };
    delete broken.summary;
    expect(isWorkspaceOverview(broken)).toBe(false);
  });

  test("helloProblem explains version skew", async () => {
    const hello = await fixture("hello.json");
    if (!isHello(hello)) throw new Error("invalid hello fixture");
    expect(helloProblem({ ...hello, protocol: PROTOCOL_VERSION + 1 })).toContain(`v${PROTOCOL_VERSION + 1}`);
    expect(helloProblem({ ...hello, protocol: PROTOCOL_VERSION + 1 })).toContain(`v${PROTOCOL_VERSION}`);
    expect(helloProblem(hello)).toBeNull();
  });
});
