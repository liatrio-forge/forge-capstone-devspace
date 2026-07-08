import { describe, expect, test } from "bun:test";
import { DevspaceClient, type ClientTransport } from "../src/client";
import { PROTOCOL_VERSION } from "../src/protocol";
import { shouldPrintHelp, startupHello, USAGE } from "../src/startup";

function pair(): { client: DevspaceClient; sent: string[] } {
  const sent: string[] = [];
  const transport: ClientTransport = {
    write: (line) => sent.push(line),
    close: () => {},
  };
  return { client: new DevspaceClient(transport), sent };
}

describe("shouldPrintHelp", () => {
  test("recognizes -h and --help anywhere in argv", () => {
    expect(shouldPrintHelp(["bun", "main.tsx", "-h"])).toBe(true);
    expect(shouldPrintHelp(["bun", "main.tsx", "--help"])).toBe(true);
    expect(shouldPrintHelp(["bun", "main.tsx", "--no-watch", "--help"])).toBe(true);
  });

  test("is false when no help flag is present", () => {
    expect(shouldPrintHelp(["bun", "main.tsx"])).toBe(false);
    expect(shouldPrintHelp(["bun", "main.tsx", "--no-watch"])).toBe(false);
  });

  test("usage mentions the binary purpose, flags, and DEVSPACE_BIN", () => {
    expect(USAGE).toContain("devspace-tui");
    expect(USAGE).toContain("--no-watch");
    expect(USAGE).toContain("--help");
    expect(USAGE).toContain("DEVSPACE_BIN");
  });
});

describe("startupHello", () => {
  test("resolves with hello on a matching protocol version", async () => {
    const { client } = pair();
    const promise = startupHello(client, 1000);
    client.feed(
      `{"id":1,"result":{"protocol":${PROTOCOL_VERSION},"workspaceRoot":"/w","machineId":"m","machineName":"mac","syncMode":"off","watch":true}}\n`,
    );
    const hello = await promise;
    expect(hello.workspaceRoot).toBe("/w");
  });

  test("rejects on protocol mismatch without hanging", async () => {
    const { client } = pair();
    const promise = startupHello(client, 1000);
    client.feed(
      `{"id":1,"result":{"protocol":${PROTOCOL_VERSION + 1},"workspaceRoot":"/w","machineId":"m","machineName":"mac","syncMode":"off","watch":true}}\n`,
    );
    await expect(promise).rejects.toThrow(/protocol v/);
  });

  test("rejects with the close error (captured stderr) when the server dies first", async () => {
    const { client } = pair();
    const promise = startupHello(client, 1000);
    client.closed(new Error(`devspace ui-server exited: Unknown command "ui-server"`));
    await expect(promise).rejects.toThrow(/Unknown command "ui-server"/);
  });

  test("rejects with a timeout error if hello never arrives within the startup window", async () => {
    const { client } = pair();
    await expect(startupHello(client, 20)).rejects.toThrow(/timed out after 20ms/);
  });
});
