#!/usr/bin/env bash
# test-linux.sh — run this repo's own release gate on LINUX, in a throwaway
# container, from a mac (ranger-base-dbe).
#
# WHY THIS EXISTS. The suite had only ever been run on darwin. Two defects
# survived that way and were found on a release rehearsal, one of them a real
# runtime bug in the linux tarballs we ship:
#   ranger-base-fjj  ServerGen fences herdr generations on inode number, which
#                    ext4/overlayfs recycle and APFS does not — 9 tests red,
#                    and the stale-meta guard silently open on linux builds.
#   ranger-base-gaf  a test hardcoding macOS's /bin/zsh.
# Before this script the FIRST thing that ran the suite on Linux was
# .github/workflows/release.yml — on a tag, which is the worst place to learn
# it. This is the same gate, one command, no CI minutes, runnable before you
# push.
#
# It runs exactly what the release workflow runs — `go vet ./...` then
# `make test` (go test + the silent-revert audit) — so green here means green
# there. Nothing else: it does not build artifacts and does not touch GitHub.
#
# ONE MEASURED LIMIT ON THAT CLAIM (ranger-base-hhcu): "linux" is not one
# userland. This image is debian-based and its /usr/bin/awk is mawk 1.3.4
# (measured 2026-08-29 in golang:1.26); ubuntu-latest's coerces numeric-looking
# strings where mawk does not, and the difference was enough to make
# scripts/audit-silent-reverts.sh report a silent revert on ubuntu-latest and
# none here, over the identical 422 commits. So a green run here rules out the
# platform split this script was built for — syscalls, filesystem semantics,
# /bin/zsh — and does NOT rule out a difference between two linux DISTRIBUTIONS'
# shell tools. Anything that shells out to awk/sed/stat wants a pin that does
# not depend on which one is installed, the way that audit's `numeric` self-test
# arm now does.
#
# Usage:
#   scripts/test-linux.sh                 vet + make test, on linux
#   scripts/test-linux.sh <cmd...>        run <cmd> in the container instead
#   scripts/test-linux.sh --shell         interactive shell in there
#
# Environment:
#   IMAGE=golang:1.26     override the toolchain image (default: golang:<go.mod>)
#   PLATFORM=linux/amd64  test the other architecture (slow, emulated)
#                         default: the host's, from `uname -m` — always passed
#
# THE REPO IS MOUNTED READ-ONLY and the container runs as YOU, not root, so a
# run cannot leave anything in the working tree — not a root-owned artifact,
# not a rewritten go.sum. A test that needs to write must use t.TempDir(), the
# same as it must in CI. The build/module cache is the one writable thing, and
# it lives outside the repo (see CACHE below) so repeat runs are fast.
set -euo pipefail

REPO_ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/posse/test-linux"

die() { printf 'test-linux: %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "docker not found on PATH"
# Do NOT tell the caller to start Docker Desktop: OPERATOR RULING 2026-08-30
# (ranger-base-6mz7) abandoned Docker on this box permanently. Run this
# rehearsal on a box that still has a container engine, or wait on CI.
docker info >/dev/null 2>&1 || die "docker daemon unavailable — Docker is abandoned on this box (operator ruling, ranger-base-6mz7); run this rehearsal elsewhere, or rely on .github/workflows/release.yml"

# Track go.mod rather than pinning a tag here: the major.minor stream always
# exists upstream and always carries a patch >= the one go.mod asks for, so
# this cannot silently test an older toolchain than the release builds with.
go_minor=$(sed -n 's/^go \([0-9][0-9]*\.[0-9][0-9]*\).*$/\1/p' "$REPO_ROOT/go.mod" | head -1)
[ -n "$go_minor" ] || die "could not read the go directive from go.mod"
IMAGE="${IMAGE:-golang:$go_minor}"

# THE GIT DIR IS NOT ALWAYS INSIDE THE REPO (ranger-base-v0gm). Every
# dispatched session works in a linked worktree (AGENTS.md, "Landing the
# plane"), and there `.git` is a FILE reading `gitdir: <abs path outside the
# worktree>`. Mount only the worktree and git inside the container cannot
# resolve the repo at all: `fatal: not a git repository`, three seedpub
# publication-boundary tests red 40s in, looking exactly like product
# failures. So mount whatever git dirs live outside $REPO_ROOT as well — at
# the SAME absolute path git names, since that pointer is baked into the .git
# file — and keep them :ro, which is the property that must not move.
git_mounts=""   # newline-delimited absolute paths, outside $REPO_ROOT
git_mount_add() {
  local p=$1 seen
  [ -n "$p" ] || return 0
  case $p in "$REPO_ROOT" | "$REPO_ROOT"/*) return 0 ;; esac   # already under /repo
  while IFS= read -r seen; do
    [ -n "$seen" ] || continue
    case $p in "$seen" | "$seen"/*) return 0 ;; esac           # already covered
  done <<EOF
$git_mounts
EOF
  git_mounts="${git_mounts}${p}
"
}
# --git-common-dir first: it normally contains the worktree's own git dir, so
# the second add dedupes into it and one mount covers both.
if git_paths=$(cd -- "$REPO_ROOT" && git rev-parse --path-format=absolute --git-common-dir --git-dir 2>/dev/null); then
  while IFS= read -r p; do git_mount_add "$p"; done <<EOF
$git_paths
EOF
elif [ -f "$REPO_ROOT/.git" ]; then
  die "$REPO_ROOT is a linked worktree ($(cat "$REPO_ROOT/.git")) but git here cannot resolve its git dir, so the container could not either — every git-reading test would fail"
fi

mkdir -p "$CACHE/build" "$CACHE/mod"

# --platform IS ALWAYS PASSED, defaulting to this host (ranger-base-1qm5).
# Docker's classic image store keys `golang:<minor>` as ONE local image, so a
# platform-specific run replaces whatever that tag pointed at: one documented
# `PLATFORM=linux/amd64` run left the tag holding the amd64 blob, and every
# later DEFAULT run then qemu-emulated amd64 — silently, apart from a platform
# WARNING — instead of testing the host arch NOTES.md says it tests. Naming the
# platform every time makes the request explicit in both directions, so the run
# after an override resolves back to the host's arch instead of inheriting the
# override's. (Docker's containerd image store keeps both platforms under the
# tag and does not poison; this does not depend on which store is configured.)
if [ -z "${PLATFORM:-}" ]; then
  case "$(uname -m)" in
    arm64 | aarch64) PLATFORM=linux/arm64 ;;
    x86_64 | amd64)  PLATFORM=linux/amd64 ;;
    *) die "unknown host architecture '$(uname -m)' — set PLATFORM=linux/<arch> explicitly" ;;
  esac
fi

run_flags=(--rm -i --platform "$PLATFORM")

# /repo plus every git dir mounted at its own path: mounted read-only, and
# marked safe.directory so git does not refuse them on an ownership mismatch.
safe_n=1
git_env=(-e GIT_CONFIG_KEY_0=safe.directory -e GIT_CONFIG_VALUE_0=/repo)
while IFS= read -r p; do
  [ -n "$p" ] || continue
  run_flags+=(-v "$p:$p:ro")
  git_env+=(-e "GIT_CONFIG_KEY_$safe_n=safe.directory" -e "GIT_CONFIG_VALUE_$safe_n=$p")
  safe_n=$((safe_n + 1))
done <<EOF
$git_mounts
EOF
git_env+=(-e "GIT_CONFIG_COUNT=$safe_n")

gate='go vet ./... && make test'
case "${1:-}" in
  --shell) gate='exec bash'; shift ;;
  '')      ;;
  *)       gate="$*" ;;
esac
# A tty when there is one to give: --shell, or a human watching a normal run.
if [ -t 0 ] && [ -t 1 ]; then run_flags+=(-t); fi

echo "test-linux: $IMAGE ($PLATFORM) — $gate"
if [ -n "$git_mounts" ]; then
  printf 'test-linux: linked worktree — also mounting %s read-only at its own path so git resolves in the container\n' \
    "$(printf '%s' "$git_mounts" | tr '\n' ' ' | sed 's/ $//')"
fi

exec docker run "${run_flags[@]}" \
  --user "$(id -u):$(id -g)" \
  -v "$REPO_ROOT:/repo:ro" \
  -v "$CACHE/build:/gocache" \
  -v "$CACHE/mod:/gomodcache" \
  -w /repo \
  -e HOME=/tmp \
  -e GOCACHE=/gocache \
  -e GOMODCACHE=/gomodcache \
  -e GOFLAGS=-count=1 \
  "${git_env[@]}" \
  "$IMAGE" bash -c "$gate"
