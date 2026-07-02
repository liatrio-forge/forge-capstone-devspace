#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/demo-check.sh [--output-dir PATH]

Runs the full local DevDrop demo without network access:
  build, init, scan, push, pull, plan, apply, hydrate, env, and status.

Without --output-dir, all generated files live in a temporary directory that is
removed on exit. With --output-dir, the directory is left in place for proof
artifacts and must be empty or not exist.
USAGE
}

output_dir=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "error: --output-dir requires a path" >&2
        exit 2
      fi
      output_dir="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cleanup="true"

if [[ -n "$output_dir" ]]; then
  run_root="$(mkdir -p "$output_dir" && cd "$output_dir" && pwd)"
  if [[ -n "$(find "$run_root" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    echo "error: --output-dir must be empty: $run_root" >&2
    exit 2
  fi
  cleanup="false"
else
  run_root="$(mktemp -d "${TMPDIR:-/tmp}/devdrop-demo-check.XXXXXX")"
fi

if [[ "$cleanup" == "true" ]]; then
  trap 'rm -rf "$run_root"' EXIT
else
  trap 'echo "Proof artifacts kept at: $run_root"' EXIT
fi

log() {
  printf '\n==> %s\n' "$*"
}

fail() {
  echo "demo-check failed: $*" >&2
  exit 1
}

run() {
  log "$*"
  "$@"
}

assert_file_mode_600() {
  local path="$1"
  local mode
  if mode="$(stat -c '%a' "$path" 2>/dev/null)"; then
    :
  else
    mode="$(stat -f '%Lp' "$path")"
  fi
  [[ "$mode" == "600" ]] || fail "$path mode is $mode, expected 600"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  [[ "$haystack" == *"$needle"* ]] || fail "expected output to contain: $needle"
}

devspace="$run_root/bin/devspace"
workspace_a="$run_root/workspace-a"
workspace_b="$run_root/workspace-b"
home_a="$run_root/home-a"
home_b="$run_root/home-b"
remote_src="$run_root/remote-src"
project_remote="$run_root/client-a-api.git"
manifest_remote="$run_root/manifest-sync.git"

log "Using run directory: $run_root"

run mkdir -p "$run_root/bin" "$remote_src"
run go build -trimpath -o "$devspace" "$repo_root/cmd/devspace"

log "Creating local project remote"
run git -C "$remote_src" init -b main
run git -C "$remote_src" config user.email demo@example.com
run git -C "$remote_src" config user.name "Demo User"
printf '# client-a-api\n' > "$remote_src/README.md"
printf '{"scripts":{"dev":"vite"}}\n' > "$remote_src/package.json"
run git -C "$remote_src" add README.md package.json
run git -C "$remote_src" commit -m "initial"
run git clone --bare "$remote_src" "$project_remote"

log "Machine A: initialize, scan, and push manifest"
export DEV_DROP_HOME="$home_a"
run "$devspace" init --workspace "$workspace_a"
run mkdir -p "$workspace_a/work"
run git clone "$project_remote" "$workspace_a/work/client-a-api"
scan_a="$("$devspace" scan)"
printf '%s\n' "$scan_a"
assert_contains "$scan_a" "Found 1 projects"
run "$devspace" workspace remote create local "$manifest_remote"
push_a="$("$devspace" workspace push)"
printf '%s\n' "$push_a"
assert_contains "$push_a" "Pushed workspace manifest."
status_a="$("$devspace" status)"
printf '%s\n' "$status_a"
assert_contains "$status_a" "Projects tracked: 1"
git --git-dir "$manifest_remote" show main:manifest.json > "$run_root/pushed-manifest.json"

log "Machine B: pull manifest, plan, apply, and hydrate"
export DEV_DROP_HOME="$home_b"
run "$devspace" init --workspace "$workspace_b"
run "$devspace" workspace remote set "$manifest_remote"
pull_b="$("$devspace" workspace pull)"
printf '%s\n' "$pull_b"
assert_contains "$pull_b" "Pulled workspace manifest."
plan_b="$("$devspace" plan)"
printf '%s\n' "$plan_b"
assert_contains "$plan_b" "PLACEHOLDER work/client-a-api"
[[ -f "$workspace_b/.devdrop/last-plan.json" ]] || fail "expected saved plan at $workspace_b/.devdrop/last-plan.json"
apply_b="$("$devspace" apply)"
printf '%s\n' "$apply_b"
assert_contains "$apply_b" "Applied safe plan actions."
[[ -d "$workspace_b/work/client-a-api" ]] || fail "expected placeholder directory after apply"
[[ -z "$(find "$workspace_b/work/client-a-api" -mindepth 1 -print -quit)" ]] || fail "placeholder directory should be empty before hydrate"
status_placeholder="$("$devspace" status)"
printf '%s\n' "$status_placeholder"
assert_contains "$status_placeholder" "Placeholders: 1"
hydrate_b="$("$devspace" project hydrate client-a-api)"
printf '%s\n' "$hydrate_b"
assert_contains "$hydrate_b" "Hydrated client-a-api"
[[ -d "$workspace_b/work/client-a-api/.git" ]] || fail "expected hydrated Git repository"
[[ -f "$workspace_b/work/client-a-api/README.md" ]] || fail "expected hydrated README.md"

log "Machine B: encrypted env profile and final status"
printf 'postgres://demo\n' | "$devspace" env set client-a-api DATABASE_URL
env_list="$("$devspace" env list client-a-api)"
printf '%s\n' "$env_list"
assert_contains "$env_list" "DATABASE_URL=****"
env_pull="$("$devspace" env pull client-a-api)"
printf '%s\n' "$env_pull"
env_file="$workspace_b/work/client-a-api/.env"
[[ -f "$env_file" ]] || fail "expected generated .env file"
assert_file_mode_600 "$env_file"
grep -qx 'DATABASE_URL=postgres://demo' "$env_file" || fail "generated .env did not contain expected demo key"
status_b="$("$devspace" status)"
printf '%s\n' "$status_b"
assert_contains "$status_b" "Hydrated: 1"
assert_contains "$status_b" "Missing env files: 0"
project_status="$("$devspace" project status client-a-api)"
printf '%s\n' "$project_status"
assert_contains "$project_status" "Hydrated: true"
assert_contains "$project_status" "Missing env: false"

cat > "$run_root/demo-check-summary.txt" <<SUMMARY
DevDrop demo-check passed.
Workspace A: $workspace_a
Workspace B: $workspace_b
Manifest remote: $manifest_remote
Project remote: $project_remote
Generated env mode: 600
SUMMARY

printf '\nDevDrop demo-check passed.\n'
if [[ "$cleanup" == "false" ]]; then
  printf 'Proof artifacts: %s\n' "$run_root"
fi
