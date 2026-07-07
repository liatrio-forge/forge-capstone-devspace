import { describe, expect, test } from "bun:test";
import { DevspaceClient, type ClientTransport } from "../src/client";
import type { ServerEvent } from "../src/protocol";

function pair(requestTimeoutMs?: number): { client: DevspaceClient; sent: string[] } {
  const sent: string[] = [];
  const transport: ClientTransport = {
    write: (line) => sent.push(line),
    close: () => {},
  };
  return { client: new DevspaceClient(transport, requestTimeoutMs), sent };
}

describe("DevspaceClient", () => {
  test("matches responses to requests by id", async () => {
    const { client, sent } = pair();
    const hello = client.request("hello");
    const status = client.request("status");
    expect(sent).toHaveLength(2);
    const first = JSON.parse(sent[0]!);
    const second = JSON.parse(sent[1]!);
    expect(first).toEqual({ id: 1, method: "hello" });
    expect(second).toEqual({ id: 2, method: "status" });

    // Answer out of order.
    client.feed(`{"id":2,"result":{"configured":false,"localDiffers":false,"diffAdded":0,"diffRemoved":0,"diffChanged":0,"reconcileSaved":false,"conflictCount":0}}\n`);
    client.feed(`{"id":1,"result":{"protocol":1,"workspaceRoot":"/w","machineId":"m","machineName":"mac","syncMode":"off","watch":true}}\n`);
    expect((await status).configured).toBe(false);
    expect((await hello).workspaceRoot).toBe("/w");
  });

  test("sends params and rejects on error responses", async () => {
    const { client, sent } = pair();
    const hydrate = client.request("hydrate", { ref: "apps/api" });
    expect(JSON.parse(sent[0]!)).toEqual({ id: 1, method: "hydrate", params: { ref: "apps/api" } });
    client.feed(`{"id":1,"error":{"message":"project not found"}}\n`);
    await expect(hydrate).rejects.toThrow("project not found");
  });

  test("handles partial frames and multiple frames per chunk", async () => {
    const { client } = pair();
    const a = client.request("projects");
    const b = client.request("projects");
    client.feed(`{"id":1,"result":{"rows":[],"summary":{"foundProjects":0,"gitRepos":0,"untrackedFolders":0,"loc`);
    client.feed(`alOnlyProjects":0,"projectsWithEnv":0}}}\n{"id":2,"result":{"rows":[],"summary":{"foundProjects":3,"gitRepos":1,"untrackedFolders":0,"localOnlyProjects":2,"projectsWithEnv":0}}}\n`);
    expect((await a).summary.foundProjects).toBe(0);
    expect((await b).summary.foundProjects).toBe(3);
  });

  test("delivers unsolicited events to listeners", () => {
    const { client } = pair();
    const events: ServerEvent[] = [];
    const off = client.onEvent((ev) => events.push(ev));
    client.feed(`{"method":"event","params":{"type":"watch-error","message":"boom"}}\n`);
    expect(events).toEqual([{ type: "watch-error", message: "boom" }]);
    off();
    client.feed(`{"method":"event","params":{"type":"watch-error","message":"again"}}\n`);
    expect(events).toHaveLength(1);
  });

  test("rejects all in-flight requests when the server exits", async () => {
    const { client } = pair();
    const inflight = client.request("scan");
    client.closed();
    await expect(inflight).rejects.toThrow("ui-server exited");
  });

  test("ignores malformed lines without breaking the stream", async () => {
    const { client } = pair();
    const req = client.request("hello");
    client.feed("garbage\n");
    client.feed(`{"id":1,"result":{"protocol":1,"workspaceRoot":"/w","machineId":"m","machineName":"","syncMode":"off","watch":false}}\n`);
    expect((await req).protocol).toBe(1);
  });

  test("rejects a request that exceeds requestTimeoutMs", async () => {
    const { client } = pair(30);
    const req = client.request("hello");
    await expect(req).rejects.toThrow("timed out after 30ms");
  });

  test("resolves normally when the response arrives before the timeout", async () => {
    const { client } = pair(50);
    const req = client.request("hello");
    client.feed(
      `{"id":1,"result":{"protocol":1,"workspaceRoot":"/w","machineId":"m","machineName":"mac","syncMode":"off","watch":true}}\n`,
    );
    expect((await req).workspaceRoot).toBe("/w");
    // Wait past the timeout window to prove the cleared timer doesn't fire a stray rejection.
    await new Promise((resolve) => setTimeout(resolve, 80));
  });

  test("closed() with stderr context propagates the message to in-flight requests", async () => {
    const { client } = pair();
    const req = client.request("scan");
    client.closed(new Error("devspace ui-server exited: connection refused"));
    await expect(req).rejects.toThrow("devspace ui-server exited: connection refused");
  });
});
