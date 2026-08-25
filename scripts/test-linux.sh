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
# Usage:
#   scripts/test-linux.sh                 vet + make test, on linux
#   scripts/test-linux.sh <cmd...>        run <cmd> in the container instead
#   scripts/test-linux.sh --shell         interactive shell in there
#
# Environment:
#   IMAGE=golang:1.26     override the toolchain image (default: golang:<go.mod>)
#   PLATFORM=linux/amd64  test the other architecture (slow, emulated)
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
docker info >/dev/null 2>&1 || die "docker daemon is not running — start Docker Desktop"

# Track go.mod rather than pinning a tag here: the major.minor stream always
# exists upstream and always carries a patch >= the one go.mod asks for, so
# this cannot silently test an older toolchain than the release builds with.
go_minor=$(sed -n 's/^go \([0-9][0-9]*\.[0-9][0-9]*\).*$/\1/p' "$REPO_ROOT/go.mod" | head -1)
[ -n "$go_minor" ] || die "could not read the go directive from go.mod"
IMAGE="${IMAGE:-golang:$go_minor}"

mkdir -p "$CACHE/build" "$CACHE/mod"

run_flags=(--rm -i)
if [ -n "${PLATFORM:-}" ]; then run_flags+=(--platform "$PLATFORM"); fi

gate='go vet ./... && make test'
case "${1:-}" in
  --shell) gate='exec bash'; shift ;;
  '')      ;;
  *)       gate="$*" ;;
esac
# A tty when there is one to give: --shell, or a human watching a normal run.
if [ -t 0 ] && [ -t 1 ]; then run_flags+=(-t); fi

echo "test-linux: $IMAGE${PLATFORM:+ ($PLATFORM)} — $gate"

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
  -e GIT_CONFIG_COUNT=1 \
  -e GIT_CONFIG_KEY_0=safe.directory \
  -e GIT_CONFIG_VALUE_0=/repo \
  "$IMAGE" bash -c "$gate"
