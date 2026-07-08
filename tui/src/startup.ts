import type { DevspaceClient } from "./client";
import { helloProblem, type Hello } from "./protocol";

export const USAGE = `devspace-tui — companion dashboard for devspace

Usage: devspace-tui [flags]

Flags:
  --no-watch   disable the workspace file watcher
  -h, --help   show this help and exit

Env:
  DEVSPACE_BIN   devspace binary to spawn (default: "devspace" on PATH)
`;

export function shouldPrintHelp(argv: string[]): boolean {
  return argv.includes("-h") || argv.includes("--help");
}

/** Short timeout for the startup handshake, well under DEFAULT_REQUEST_TIMEOUT_MS. */
export const STARTUP_HELLO_TIMEOUT_MS = 10_000;

/**
 * Requests `hello` with a short startup timeout and runs the protocol check.
 * Rejects with a ready-to-print message on timeout, transport failure, or
 * protocol mismatch; callers must not create the renderer until this resolves.
 */
export async function startupHello(client: DevspaceClient, timeoutMs = STARTUP_HELLO_TIMEOUT_MS): Promise<Hello> {
  let hello: Hello;
  try {
    hello = await raceTimeout(client.request("hello"), timeoutMs);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    throw new Error(`hello failed: ${message}`);
  }
  const problem = helloProblem(hello);
  if (problem) throw new Error(problem);
  return hello;
}

function raceTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`hello timed out after ${ms}ms`)), ms);
    promise.then(
      (v) => {
        clearTimeout(timer);
        resolve(v);
      },
      (err) => {
        clearTimeout(timer);
        reject(err);
      },
    );
  });
}
