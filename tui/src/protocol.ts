// Mirrors the JSON DTOs served by `devspace ui-server` (internal/devspace/ui_server.go).

export const PROTOCOL_VERSION = 1;

export interface Hello {
  protocol: number;
  version?: string;
  workspaceRoot: string;
  machineId: string;
  machineName: string;
  syncMode: string;
  watch: boolean;
}

export interface ProjectRow {
  ref: string;
  name: string;
  path: string;
  type: string;
  status: "Hydrated" | "Placeholder" | "Missing" | string;
  dirty: boolean;
  branch?: string;
  env: boolean;
}

export interface ScanSummary {
  foundProjects: number;
  gitRepos: number;
  untrackedFolders: number;
  localOnlyProjects: number;
  projectsWithEnv: number;
}

export interface SyncStatus {
  configured: boolean;
  lastSyncAt?: string;
  localDiffers: boolean;
  diffAdded: number;
  diffRemoved: number;
  diffChanged: number;
  reconcileSaved: boolean;
  conflictCount: number;
  gitDiffUnavailable?: string;
  unavailableReason?: string;
}

export interface PlanAction {
  safety: "safe" | "skipped" | string;
  kind: string;
  path: string;
  reason?: string;
  project?: string;
}

export interface Plan {
  version: number;
  workspaceRoot: string;
  manifestHash: string;
  generatedAt: string;
  actions: PlanAction[] | null;
  warnings: string[] | null;
}

export interface Project {
  id: string;
  name: string;
  path: string;
  type: string;
  remote?: string;
  defaultBranch?: string;
  hydrateMode: string;
}

export interface Snapshot {
  rows: ProjectRow[];
  summary: ScanSummary;
  plan?: Plan;
  project?: Project;
}

export interface WatchRefresh {
  fullScan: boolean;
  refreshStartedAt?: string;
  watchedDirCount: number;
  syncChanged: boolean;
  syncMode?: string;
}

export type ServerEvent =
  | { type: "watch-refresh"; rows: ProjectRow[]; summary: ScanSummary; refresh: WatchRefresh }
  | { type: "watch-error"; message: string };

type JsonRecord = Record<string, unknown>;

function isRecord(v: unknown): v is JsonRecord {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function isStringArray(v: unknown): v is string[] {
  return Array.isArray(v) && v.every((item) => typeof item === "string");
}

function optionalString(v: JsonRecord, key: string): boolean {
  return v[key] === undefined || typeof v[key] === "string";
}

function optionalStringArray(v: JsonRecord, key: string): boolean {
  return v[key] === undefined || isStringArray(v[key]);
}

function isProjectRow(v: unknown): v is ProjectRow {
  return (
    isRecord(v) &&
    typeof v.ref === "string" &&
    typeof v.name === "string" &&
    typeof v.path === "string" &&
    typeof v.type === "string" &&
    typeof v.status === "string" &&
    typeof v.dirty === "boolean" &&
    optionalString(v, "branch") &&
    typeof v.env === "boolean"
  );
}

function isScanSummary(v: unknown): v is ScanSummary {
  return (
    isRecord(v) &&
    typeof v.foundProjects === "number" &&
    typeof v.gitRepos === "number" &&
    typeof v.untrackedFolders === "number" &&
    typeof v.localOnlyProjects === "number" &&
    typeof v.projectsWithEnv === "number"
  );
}

function isPlanAction(v: unknown): v is PlanAction {
  return (
    isRecord(v) &&
    typeof v.safety === "string" &&
    typeof v.kind === "string" &&
    typeof v.path === "string" &&
    optionalString(v, "reason") &&
    optionalString(v, "project")
  );
}

function isPlan(v: unknown): v is Plan {
  return (
    isRecord(v) &&
    typeof v.version === "number" &&
    typeof v.workspaceRoot === "string" &&
    typeof v.manifestHash === "string" &&
    typeof v.generatedAt === "string" &&
    (v.actions === null || (Array.isArray(v.actions) && v.actions.every(isPlanAction))) &&
    (v.warnings === null || isStringArray(v.warnings))
  );
}

function isProject(v: unknown): v is Project {
  return (
    isRecord(v) &&
    typeof v.id === "string" &&
    typeof v.name === "string" &&
    typeof v.path === "string" &&
    typeof v.type === "string" &&
    optionalString(v, "remote") &&
    optionalString(v, "defaultBranch") &&
    typeof v.hydrateMode === "string" &&
    optionalStringArray(v, "envProfiles")
  );
}

function isWatchRefresh(v: unknown): v is WatchRefresh {
  return (
    isRecord(v) &&
    typeof v.fullScan === "boolean" &&
    optionalString(v, "refreshStartedAt") &&
    typeof v.watchedDirCount === "number" &&
    typeof v.syncChanged === "boolean" &&
    optionalString(v, "syncMode")
  );
}

export function isHello(v: unknown): v is Hello {
  return (
    isRecord(v) &&
    typeof v.protocol === "number" &&
    optionalString(v, "version") &&
    typeof v.workspaceRoot === "string" &&
    typeof v.machineId === "string" &&
    typeof v.machineName === "string" &&
    typeof v.syncMode === "string" &&
    typeof v.watch === "boolean"
  );
}

export function isSnapshot(v: unknown): v is Snapshot {
  return (
    isRecord(v) &&
    Array.isArray(v.rows) &&
    v.rows.every(isProjectRow) &&
    isScanSummary(v.summary) &&
    (v.plan === undefined || isPlan(v.plan)) &&
    (v.project === undefined || isProject(v.project))
  );
}

export function isSyncStatus(v: unknown): v is SyncStatus {
  return (
    isRecord(v) &&
    typeof v.configured === "boolean" &&
    optionalString(v, "lastSyncAt") &&
    typeof v.localDiffers === "boolean" &&
    typeof v.diffAdded === "number" &&
    typeof v.diffRemoved === "number" &&
    typeof v.diffChanged === "number" &&
    typeof v.reconcileSaved === "boolean" &&
    typeof v.conflictCount === "number" &&
    optionalString(v, "gitDiffUnavailable") &&
    optionalString(v, "unavailableReason")
  );
}

export function isServerEvent(v: unknown): v is ServerEvent {
  if (!isRecord(v) || typeof v.type !== "string") return false;
  if (v.type === "watch-error") return typeof v.message === "string";
  return v.type === "watch-refresh" && Array.isArray(v.rows) && v.rows.every(isProjectRow) && isScanSummary(v.summary) && isWatchRefresh(v.refresh);
}

export function helloProblem(hello: Hello): string | null {
  if (hello.protocol === PROTOCOL_VERSION) return null;
  return `devspace ui-server speaks protocol v${hello.protocol}, this devspace-tui expects v${PROTOCOL_VERSION} (server version ${
    hello.version ?? "unknown"
  }). Update devspace and devspace-tui to matching releases.`;
}

export interface RequestMap {
  hello: { params?: undefined; result: Hello };
  projects: { params?: undefined; result: Snapshot };
  scan: { params?: undefined; result: Snapshot };
  refresh: { params?: undefined; result: Snapshot };
  plan: { params?: undefined; result: Snapshot };
  apply: { params?: undefined; result: Snapshot };
  hydrate: { params: { ref: string }; result: Snapshot };
  status: { params?: undefined; result: SyncStatus };
  lastPlan: { params?: undefined; result: Plan };
}

export type Method = keyof RequestMap;
