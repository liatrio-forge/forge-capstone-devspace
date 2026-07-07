import { createCliRenderer } from "@opentui/core";
import { createRoot } from "@opentui/react";
import { App } from "./app";
import { connect, type DevspaceClient } from "./client";

const noWatch = process.argv.includes("--no-watch");

let client: DevspaceClient;
try {
  client = connect({ noWatch });
} catch (err) {
  const message = err instanceof Error ? err.message : String(err);
  process.stderr.write(`devspace-tui: failed to start devspace ui-server: ${message}\n`);
  process.exit(1);
}

const renderer = await createCliRenderer({
  exitOnCtrlC: false, // we quit via our own handler so the terminal is always restored
  consoleMode: "disabled",
});

let quitting = false;
function quit(code = 0, message?: string): void {
  if (quitting) return;
  quitting = true;
  client.close();
  renderer.destroy();
  if (message) process.stderr.write(`devspace-tui: ${message}\n`);
  process.exit(code);
}

process.on("SIGINT", () => quit(130));
process.on("SIGTERM", () => quit(143));

createRoot(renderer).render(<App client={client} quit={(message) => quit(message === undefined ? 0 : 1, message)} />);
