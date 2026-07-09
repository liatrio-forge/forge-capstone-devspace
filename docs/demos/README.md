# DevSpace Demos

Recorded [VHS](https://github.com/charmbracelet/vhs) demos covering the full
`devspace` command surface. Every GIF is generated from a `.tape` file in this
directory — deterministic, reviewable in git, and regenerable on demand. All
demos run against throwaway `DEVSPACE_HOME` temp dirs and temp workspaces;
your real `~/.devspace` is never touched.

## Regenerating

```bash
make build            # tapes use the freshly built bin/devspace
cd docs/demos
vhs getting-started.tape        # one tape
ls *.tape | xargs -n1 vhs       # or everything
```

Requires `vhs` and `ffmpeg` (`brew install vhs ffmpeg`).

## The demos

### Getting started — `init`, `doctor`, `scan`, `status`, `plan`, `apply`

The core loop on a fresh workspace: initialize, check readiness, discover
projects, and apply the safe plan.

![getting-started](getting-started.gif)

### Two-machine sync — `workspace remote/push/pull/diff`

Machine A publishes its workspace manifest through a local bare Git remote;
Machine B joins, pulls, and stays in sync as new projects appear.

![workspace-sync](workspace-sync.gif)

### Capstone walkthrough — `pull`, `plan`, `apply`, `project hydrate`, `status`

The hero sequence from the [demo runbook](capstone-runbook.md): a brand-new
machine pulls the workspace shape, applies safe placeholders, and hydrates a
real checkout.

![capstone-walkthrough](capstone-walkthrough.gif)

### Reconcile — `workspace diff`, `workspace reconcile [--apply]`

Both machines change the manifest independently; `diff` previews the drift and
`reconcile` three-way merges it (review artifact first, `--apply` to commit).

![reconcile](reconcile.gif)

### Project lifecycle — `project add/status/update/remove`

Track a project, inspect it, fast-forward it when its origin moves ahead, and
untrack it — files on disk are never touched.

![project-lifecycle](project-lifecycle.gif)

### Watch — `watch --once`, `watch --debounce`

Event-driven metadata refresh: a new project lands in the workspace and watch
picks it up live. Watch never pulls, applies, hydrates, or uploads.

![watch](watch.gif)

### Setup commands — `setup plan/run/apply`

Detected per-project install/dev commands, shown and dry-run only — devspace
never auto-executes project commands.

![setup-commands](setup-commands.gif)

### Env secrets — `env set/list/pull`, `env recipient export/list`

Encrypted per-project env profiles (native `age`): set a value, pull a `0600`
`.env`, and see who can decrypt it.

![env-secrets](env-secrets.gif)

### Hosted sync — `hosted config/push/pull/reconcile/serve`

The opt-in hosted control-plane prototype: a local `hosted serve` instance
receives and returns the manifest over bearer-token HTTP.

![hosted-sync](hosted-sync.gif)

### Mount preview — `mount --preview [--json]`

The FUSE lazy-mount prototype's projected view, no FUSE required: tracked
projects appear as entries whether hydrated or not.

![mount-preview](mount-preview.gif)

### UI dashboard — `ui --legacy`

The built-in full-screen dashboard: tracked projects, scan counts, and
safe actions only.

![ui-dashboard](ui-dashboard.gif)

## Helper scripts

| Script | Used by | Purpose |
| --- | --- | --- |
| `capstone-rehearsal.sh` | `capstone-walkthrough.tape`, live demos | Seed/rehearse the two-machine capstone sandbox (`clean`/`full`/`seed`) |
| `mkgitproject.sh` | `workspace-sync.tape`, `reconcile.tape` | Create a throwaway git-init'd project with one commit |
| `project-remote.sh` | `project-lifecycle.tape` | Seed a project cloned from a local bare origin; `advance` moves the origin ahead |
| `spawn-project-later.sh` | `watch.tape` | Drop a new project into the workspace after a delay, in the background |
| `set-api-key.sh` | `env-secrets.tape` | Pipe a placeholder value into `devspace env set` without fragile tape quoting |
| `hosted-sync-serve.sh` | `hosted-sync.tape` | Start a background `hosted serve` on a scratch port with a placeholder token |

Not recorded: `devspace mount` (real FUSE mount — needs a kernel extension),
`devspace tui` (companion install management), and `devspace ui` with the
OpenTUI companion (see [`tui/`](../../tui/) — the tape forces `--legacy`).
