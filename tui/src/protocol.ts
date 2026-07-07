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
