#!/usr/bin/env bash
# verify-orphan-report.sh — the planted positive control for the load guard's
# orphan report (ranger-base-apwr), in a throwaway CPU-limited container.
#
# WHY THIS EXISTS. Arm 1 of the orphan report names leaked gate-shell children
# when the load guard skips a pass. A detector that has never fired has not
# been shown able to fire, and the unit pins in internal/rhq/loadguard_test.go
# only hand the predicate rows a test wrote. The control that measures
# something plants the real thing: a gate shell forks a subshell, the parent
# exits, the subshell lands on ppid 1 and burns a core, and the real census —
# real `ps`, real columns, real argv — has to name it and NOT name the two
# look-alikes beside it.
#
# It runs in a container and not on the box for the operator's standing
# reason (ranger-base-teau, and monica's ORDERS): the last time a persona
# generated load here it leaked sixteen busy loops and froze the fleet for
# 2.5 hours. Everything this script plants dies with `--rm`.
#
#   gated + orphaned + burning     must be NAMED
#   ungated + orphaned + burning   must be SILENT — a leak, but not ours
#   gated + burning + parent alive must be SILENT — ordinary fleet work
#
# It also waits out the real LoadOrphanMinAge rather than lowering it, so the
# run takes a couple of minutes. That is the point: the age floor is one of
# the things under test.
#
# Usage:
#   scripts/verify-orphan-report.sh
#
# Environment:
#   IMAGE=golang:1.26      override the toolchain image (default: golang:<go.mod>)
#   PLATFORM=linux/amd64   test the other architecture (slow, emulated)
#   CPUS=1                 the container's CPU limit (default 1)
set -euo pipefail

REPO_ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/posse/test-linux"

die() { printf 'verify-orphan-report: %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "docker not found on PATH"
docker info >/dev/null 2>&1 || die "docker daemon is not running — start Docker Desktop"

go_minor=$(sed -n 's/^go \([0-9][0-9]*\.[0-9][0-9]*\).*$/\1/p' "$REPO_ROOT/go.mod" | head -1)
[ -n "$go_minor" ] || die "could not read the go directive from go.mod"
IMAGE="${IMAGE:-golang:$go_minor}"

# --platform is always passed, for the reason test-linux.sh spells out: the
# classic image store keys golang:<minor> as ONE local image, so an override
# poisons every later default run unless the default names itself too.
if [ -z "${PLATFORM:-}" ]; then
  case "$(uname -m)" in
    arm64 | aarch64) PLATFORM=linux/arm64 ;;
    x86_64 | amd64)  PLATFORM=linux/amd64 ;;
    *) die "unknown host architecture '$(uname -m)' — set PLATFORM=linux/<arch> explicitly" ;;
  esac
fi

mkdir -p "$CACHE/build" "$CACHE/mod"

# The repo is mounted read-only and the run is not root, the same as
# test-linux.sh: a control that plants processes must not also be able to
# leave anything in the working tree.
echo "verify-orphan-report: $IMAGE ($PLATFORM), --cpus ${CPUS:-1} — planting a leak and reading it back"
exec docker run --rm -i \
  --platform "$PLATFORM" \
  --cpus "${CPUS:-1}" \
  --user "$(id -u):$(id -g)" \
  -v "$REPO_ROOT:/repo:ro" \
  -v "$CACHE/build:/gocache" \
  -v "$CACHE/mod:/gomodcache" \
  -w /repo \
  -e HOME=/tmp \
  -e GOCACHE=/gocache \
  -e GOMODCACHE=/gomodcache \
  -e GOFLAGS=-count=1 \
  -e RHQ_ORPHAN_CONTROL=1 \
  "$IMAGE" bash -c 'go test ./internal/rhq -run TestOrphanReportControlNamesAPlantedLeak -v -timeout 20m'
