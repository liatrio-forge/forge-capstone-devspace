# DevSpace Demo Runbook

**The story:** "Dropbox for developer workspaces." A developer's project layout —
repos, remotes, setup hints, encrypted secrets — becomes portable and reproducible
on a second machine, **without ever deleting or overwriting anything**.

**Total time:** ~5 min live. Everything runs in a throwaway sandbox; your real
`~/.devspace` is never touched (each "machine" gets its own `DEVSPACE_HOME`).

**Setup before you present:**

```bash
cd docs/demos
./capstone-rehearsal.sh clean     # start fresh
./capstone-rehearsal.sh full      # rehearse once — confirm it's green end-to-end
```

For a live drive, run `./capstone-rehearsal.sh seed` (sets up Machine A + a manifest remote,
leaves Machine B ready at the pull step), then type the Act 2 commands yourself.

---

## Act 0 — The problem (say it, don't type it) · 30s

> "New laptop. Twenty repos. Half-remembered clone URLs, `.env` files I emailed
> myself, setup steps rotting in a README. DevSpace makes the *shape* of a
> workspace portable — safely."

Three tiers of state, all plain JSON, no database:

- **App home** (`~/.devspace/`): machine identity + age key + runtime state
- **Workspace manifest** (`.devspace/manifest.json`): the shared, syncable source of truth
- **Secrets**: per-project env profiles, encrypted with native `age`

---

## Act 1 — Machine A (the laptop) · ~90s

### `devspace init --workspace <ws>` → `devspace scan`

```bash
Machine name/ID generated, age identity written.
Scan: 3 projects, 3 git repos, 3 with remotes.
```

**Talking point:** "`init` and `scan` are idempotent — re-run all day, they never
rotate your machine ID or age key. `scan` only reads the filesystem; it never runs
your project's code."

### `devspace project`

Shows the three tracked projects with their git remotes.

### `devspace workspace remote create local <path>` → `devspace workspace push`

```bash
Created local manifest remote.
Pushed manifest (committed N change(s)).
```

**Talking point:** "Sync is *your* Git repo — a bare repo here, but normally a
private GitHub repo (`workspace remote create github owner/repo --private`). No
DevSpace server required. The manifest is just a committed JSON file."

### `echo '…' | devspace env set web-store DATABASE_URL` → `devspace env list web-store`

```bash
env list shows: DATABASE_URL = ****  (masked)
```

**Talking point:** "Secrets are encrypted at rest with `age`, per project, per
profile. They live in app home — **never in the manifest**, never synced as
plaintext. When we generate a `.env`, it's written `0600`."

---

## Act 2 — Machine B (the new machine) · ~2min · **the payoff**

### `devspace init --workspace <ws-b>` → `devspace workspace remote set <path>`

Empty workspace, fresh home, pointed at the same manifest remote.

### `devspace workspace pull`

```bash
Pulled manifest.  Next: devspace plan && devspace apply
```

**Talking point:** "The *shape* of my workspace just arrived. No code yet — just
the plan of what should exist."

### `devspace plan` ← **the centerpiece. Slow down here.**

```bash
SAFE:
  PLACEHOLDER api-gateway
  PLACEHOLDER web-admin
  PLACEHOLDER web-store
No destructive changes will be performed.
```

**Talking point (the differentiator):** "`plan` diffs the desired manifest against
what's actually on disk. Every action is tagged with a **Safety level**. The only
thing it will ever do on apply is create **empty placeholder folders** — it never
clones over your work, never deletes, never overwrites. A dirty repo or a
remote-mismatch gets *skipped* with a reason, not clobbered."

### `devspace apply`

```bash
Applied safe plan actions.
→ workspace-b now has empty folders: api-gateway  web-admin  web-store
```

**Talking point:** "Placeholders. Zero bytes of your code touched. `apply` even
re-checks each destination is still empty at apply time."

### `devspace project hydrate web-store` ← **the magic moment**

```bash
Hydrating web-store...  Hydrated web-store
Suggested setup: npm install
→ web-store/ is now a real checkout (.git, package.json, README.md)
```

**Talking point:** "*Now* it clones — into a sibling temp dir, then atomically
renames into place. It refuses a non-empty destination and validates the remote
first. Placeholder → real repo, on demand. And it read the setup hint from the
manifest: `npm install`."

### `devspace status`

```bash
Projects tracked: 3 · Hydrated: 1 · Placeholders: 2
```

**Talking point:** "One command, whole-workspace truth. Hydrate the rest when you
need them — lazy materialization."

---

## Closer · 30s

> "That's the loop: **scan → plan → apply → hydrate**, safe at every step, synced
> through a repo you own. No server, no database, secrets encrypted, nothing
> destructive — ever."

**Optional flourish (if solid on your machine):** `devspace ui` launches the
OpenTUI/React companion (`--legacy` for the built-in Bubble Tea dashboard) — same
safe actions, live.

---

## What to SKIP live (and why)

- **`devspace mount`** — real FUSE lazy-mount, but a runtime-guarded *prototype*
  (needs macFUSE). Mention as "where this is going," don't demo cold.
- **`devspace hosted serve`** — a working control-plane *prototype*. Demo the
  Git-remote sync (rock solid); only show hosted if you've rehearsed it.
- Don't narrate the `devdrop → devspace` rename — it's plumbing.

## If something breaks

- Re-run `./capstone-rehearsal.sh clean && ./capstone-rehearsal.sh full` — it's fully idempotent.
- `hydrate` needs the upstream reachable; the sandbox uses local bare repos so it
  works offline.
- Every command is safe to re-run; nothing here can damage real data.
