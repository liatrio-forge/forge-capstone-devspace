# Task 3 Proof: Secondary Command Workflows

## RED

Before implementation, the new contract tests failed on every renamed or moved path:

```text
--- FAIL: TestEnvWriteMaterializesSelectedProfileSafely
    env write error: unknown flag: --profile
--- FAIL: TestEnvWriteRejectsRemovedPullPath
    env pull error = project "api" not found, want unknown command
--- FAIL: TestSetupCommandShowAndRunContract
    setup show --json error: unknown flag: --json
--- FAIL: TestExperimentalCommandOwnsHostedServeAndMount
    hosted --help still exposes serve
FAIL
```

Command:

```sh
go test ./internal/devspace -run 'TestEnvWrite|TestSetupCommand|TestExperimentalCommand' -count=1
```

## GREEN

```text
ok github.com/liatrio-forge/devdrop-capstone/internal/devspace 0.868s
```

Command:

```sh
go test ./internal/devspace -run 'TestEnvWrite|TestSetup(Command|Run|Show)|TestExperimental(Command|HostedServe|Mount)' -count=1
```

The full Go suite also passed:

```sh
go test ./...
```

```text
? github.com/liatrio-forge/devdrop-capstone/cmd/devspace [no test files]
ok github.com/liatrio-forge/devdrop-capstone/internal/devspace
```

## Env write

An isolated temporary `DEVSPACE_HOME` and workspace were initialized, a `staging` profile containing `TOKEN=[MASKED]` was encrypted, and the canonical command wrote the selected profile:

```sh
devspace env write demo --profile staging
```

```text
Wrote <workspace>/apps/demo/.env
env-mode=600 env-size=25
```

Captured stdout and stderr contained neither the decrypted value nor the `.env` contents. `TestEnvWriteMaterializesSelectedProfileSafely` additionally proves atomic symlink replacement, unchanged symlink targets, profile selection, state refresh, exact decrypted file content, and `0600` permissions. `TestEnvWriteRejectsRemovedPullPath` proves `env pull` is absent.

## Setup show and run

```sh
devspace setup show --json
devspace setup run demo --dry-run
devspace setup run --all --dry-run
```

```text
{
  "projects": [
    {
      "project": "demo",
      "path": "apps/demo",
      "packageManager": "npm",
      "installCommand": "npm install",
      "devCommand": "npm run dev",
      "runnable": true
    }
  ]
}
Would run `npm install` in apps/demo
Would run `npm install` in apps/demo
```

The mutually exclusive invocation is rejected:

```sh
devspace setup run demo --all --dry-run
```

```text
Use either --all or <project>, not both.
```

The command tests also prove project and all-project confirmation, unknown-command review, global-install review, JSON output, and removal of `setup plan` and `setup apply`.

All-project execution is command-kind aware and validates every selected project before executing any command. The regression tests prove `--command install` selects install-capable projects, `--command dev` includes dev-only projects, and a later invalid project cannot cause an earlier valid project to run partially:

```sh
go test ./internal/devspace -run 'TestRunAllProjectSetups' -count=1
```

```text
ok github.com/liatrio-forge/devdrop-capstone/internal/devspace
```

## Hosted and experimental boundaries

`devspace hosted --help` lists only the supported client operations:

```text
config
pull
push
reconcile
```

`devspace experimental --help` labels the namespace as unsupported and lists:

```text
hosted [command]              Explore the hosted sync server prototype
mount <mountpoint> [--flags]  Mount a prototype lazy workspace view (unsupported)
```

`devspace experimental hosted serve --help` retains `--addr`, `--store`, `--token`, `--trusted-proxy`, and `--allow-public-http`. The existing address tests plus `TestExperimentalCommandOwnsHostedServeAndMount` prove the public-bind opt-in guard remains active, `--allow-public-http` reaches that guard, and invalid nested `--trusted-proxy` values remain rejected.

## FUSE-free mount preview

```sh
devspace experimental mount <mountpoint> --preview
```

```text
DevSpace lazy mount preview
FUSE library: github.com/hanwen/go-fuse/v2/fs

PATH       TYPE   HYDRATE MODE   STATUS   DIRTY   ENV
apps/demo  local  manual         local    false   true
```

The experimental mount help retains `--preview`, `--json`, `--hydrate-on-lookup`, and `--debug`, directs on-demand repository maintenance to `devspace project update`, and labels stale-mount cleanup as a previous `devspace experimental mount`.
