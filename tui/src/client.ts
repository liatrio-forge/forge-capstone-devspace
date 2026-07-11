import type { Subprocess } from "bun";
import { parseResult, isServerEvent, type Method, type RequestMap, type ServerEvent } from "./protocol";

interface Pending {
  method: Method;
  resolve: (value: unknown) => void;
  reject: (err: Error) => void;
  timer?: ReturnType<typeof setTimeout>;
}

/** hydrate does a real `git clone`, so the default timeout is generous. */
const DEFAULT_REQUEST_TIMEOUT_MS = 300_000;

export interface ClientTransport {
  write(line: string): void;
  close(): void;
}

/**
 * NDJSON JSON-RPC client for `devspace ui-server`. Transport-agnostic so tests
 * can drive it with in-memory lines; `connect()` wires it to a spawned process.
 */
export class DevspaceClient {
  private nextId = 1;
  private pending = new Map<number, Pending>();
  private eventListeners = new Set<(ev: ServerEvent) => void>();
  private closeListeners = new Set<(err?: Error) => void>();
  private earlyEvents: ServerEvent[] = [];
  private buffer = "";
  private isClosed = false;
  private closeError?: Error;

  constructor(
    private transport: ClientTransport,
    private requestTimeoutMs = DEFAULT_REQUEST_TIMEOUT_MS,
  ) {}

  request<M extends Method>(
    method: M,
    ...args: RequestMap[M]["params"] extends undefined ? [] : [RequestMap[M]["params"]]
  ): Promise<RequestMap[M]["result"]> {
    if (this.isClosed) return Promise.reject(this.closeError ?? new Error("devspace ui-server exited"));
    const id = this.nextId++;
    const params = args[0];
    return new Promise((resolve, reject) => {
      const pending: Pending = { method, resolve: resolve as (v: unknown) => void, reject };
      if (this.requestTimeoutMs > 0) {
        pending.timer = setTimeout(() => {
          this.pending.delete(id);
          reject(new Error(`${method} timed out after ${this.requestTimeoutMs}ms`));
        }, this.requestTimeoutMs);
      }
      this.pending.set(id, pending);
      this.transport.write(JSON.stringify({ id, method, ...(params === undefined ? {} : { params }) }) + "\n");
    });
  }

  onEvent(listener: (ev: ServerEvent) => void): () => void {
    this.eventListeners.add(listener);
    for (const event of this.earlyEvents) listener(event);
    this.earlyEvents = [];
    return () => this.eventListeners.delete(listener);
  }

  onClose(listener: (err?: Error) => void): () => void {
    if (this.isClosed) {
      listener(this.closeError);
      return () => {};
    }
    this.closeListeners.add(listener);
    return () => this.closeListeners.delete(listener);
  }

  /** Feed raw stdout data from the server; splits NDJSON frames. */
  feed(chunk: string): void {
    this.buffer += chunk;
    let idx: number;
    while ((idx = this.buffer.indexOf("\n")) >= 0) {
      const line = this.buffer.slice(0, idx).trim();
      this.buffer = this.buffer.slice(idx + 1);
      if (line) this.dispatch(line);
    }
  }

  /** Signal that the server went away; rejects all in-flight requests. Idempotent. */
  closed(err?: Error): void {
    if (this.isClosed) return;
    this.isClosed = true;
    this.closeError = err;
    const failure = err ?? new Error("devspace ui-server exited");
    for (const [, pending] of this.pending) {
      if (pending.timer) clearTimeout(pending.timer);
      pending.reject(failure);
    }
    this.pending.clear();
    for (const listener of this.closeListeners) listener(err);
  }

  close(): void {
    this.transport.close();
  }

  private dispatch(line: string): void {
    let msg: {
      id?: number;
      method?: string;
      params?: unknown;
      result?: unknown;
      error?: { message: string };
    };
    try {
      msg = JSON.parse(line);
    } catch {
      return; // ignore non-JSON noise on the stream
    }
    if (msg.method === "event" && msg.params !== undefined) {
      if (!isServerEvent(msg.params)) return; // drop malformed/unknown events; keep the stream alive
      const event = msg.params;
      if (this.eventListeners.size === 0) {
        this.earlyEvents.push(event);
        this.earlyEvents = this.earlyEvents.slice(-20);
        return;
      }
      for (const listener of this.eventListeners) listener(event);
      return;
    }
    if (typeof msg.id !== "number") return;
    const pending = this.pending.get(msg.id);
    if (!pending) return;
    this.pending.delete(msg.id);
    if (pending.timer) clearTimeout(pending.timer);
    if (msg.error) {
      pending.reject(new Error(msg.error.message));
      return;
    }
    try {
      pending.resolve(parseResult(pending.method, msg.result));
    } catch (err) {
      pending.reject(err instanceof Error ? err : new Error(String(err)));
    }
  }
}

/** Feed an async byte stream through a stateful UTF-8 decode into onText. */
export async function pumpText(stream: AsyncIterable<Uint8Array>, onText: (text: string) => void): Promise<void> {
  const decoder = new TextDecoder();
  for await (const chunk of stream) onText(decoder.decode(chunk, { stream: true }));
  const tail = decoder.decode();
  if (tail) onText(tail);
}

export interface ConnectOptions {
  bin?: string;
  noWatch?: boolean;
  syncMode?: string;
}

/** Spawn `devspace ui-server` and return a client bound to its stdio. */
export function connect(options: ConnectOptions = {}): DevspaceClient {
  const bin = options.bin ?? process.env["DEVSPACE_BIN"] ?? "devspace";
  const args = [bin, "ui-server"];
  if (options.noWatch) args.push("--no-watch");
  if (options.syncMode) args.push("--sync", options.syncMode);

  let proc: Subprocess<"pipe", "pipe", "pipe">;
  try {
    proc = Bun.spawn(args, {
      stdin: "pipe",
      stdout: "pipe",
      stderr: "pipe",
    });
  } catch (err) {
    const cause = err instanceof Error ? err.message : String(err);
    throw new Error(`failed to spawn "${bin} ui-server" (bin=${bin}; check DEVSPACE_BIN / PATH): ${cause}`);
  }

  const MAX_STDERR_LINES = 20;
  const stderrLines: string[] = [];
  const pushStderrLine = (line: string) => {
    stderrLines.push(line);
    if (stderrLines.length > MAX_STDERR_LINES) stderrLines.shift();
  };

  const client = new DevspaceClient({
    write: (line) => {
      proc.stdin.write(line);
      proc.stdin.flush();
    },
    close: () => {
      proc.stdin.end();
      proc.kill();
    },
  });

  void (async () => {
    let buffer = "";
    await pumpText(proc.stderr, (text) => {
      buffer += text;
      let idx: number;
      while ((idx = buffer.indexOf("\n")) >= 0) {
        pushStderrLine(buffer.slice(0, idx));
        buffer = buffer.slice(idx + 1);
      }
    });
    if (buffer) pushStderrLine(buffer);
  })();

  void (async () => {
    await pumpText(proc.stdout, (text) => client.feed(text));
    const tail = stderrLines.join("\n").trim();
    // The spawned server going away is always unexpected termination — never
    // silently look like a clean close. In-memory/manual callers can still
    // pass `undefined` to closed() directly (see DevspaceClient.closed()).
    client.closed(new Error(tail ? `devspace ui-server exited: ${tail}` : "devspace ui-server exited"));
  })();
  return client;
}
