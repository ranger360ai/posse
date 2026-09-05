#!/usr/bin/env bash
# verify-orphan-report.sh — the planted positive control for the load guard's
# orphan report (ranger-base-apwr), in a throwaway CPU-limited container.
#
# WHY THIS EXISTS. Arm 1 of the orphan report names leaked gate-shell children
# when the load guard skips a pass. A detector that has never fired has not
# been shown able to fire, and the unit pins in internal/posse/loadguard_test.go
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
# ARM 2, THE KILL (ranger-base-gvp2p). The same four planted processes are
# then read a second time with `load_guard_kill: true` — the only thing that
# changes between the two passes is the config key, which is what makes this
# a measurement of the kill arm rather than of a second test setup:
#
#   gated + orphaned + burning              KILLED, and gone from the NEXT census
#   ungated + orphaned + burning            SURVIVES
#   gated + burning + parent alive          SURVIVES
#   gated + orphaned + burning + POSSE_KEEP SURVIVES — declared, and named as spared
#
# The fourth arm is the one that carries the ruling. It is identical to the
# first in every field the guard can read — same wrapper, same spinner, same
# ppid, same CPU, same age — and separated from it by the declare-or-die
# marker alone. Without it the control cannot tell a reaper that reads the
# marker from a reaper that kills whatever it finds.
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
# Do NOT tell the caller to start Docker Desktop: OPERATOR RULING 2026-08-30
# (ranger-base-6mz7) abandoned Docker on this box permanently. This control
# cannot run locally until an off-laptop cleanroom exists — skip it, do not
# retry and do not hang waiting for a daemon that will not appear.
docker info >/dev/null 2>&1 || die "docker daemon unavailable — Docker is abandoned on this box (operator ruling, ranger-base-6mz7); this planted control is parked until an off-laptop cleanroom exists, not runnable here"

# The go directive read with bash's own, not `sed | head` (ranger-base-s8b4g):
# a matcher that is signalled or cannot be exec'd under load leaves $go_minor
# empty, and the `die` below then blames go.mod for the fork. First `go
# <major>.<minor>` line wins, as `head -1` had it, and anything after the
# minor (a patch, a toolchain suffix) is dropped the way the ERE dropped it.
go_minor=
while IFS= read -r line || [ -n "$line" ]; do
  case $line in
  'go '[0-9]*.[0-9]*)
    rest=${line#go }
    rest=${rest%%[![:digit:].]*}
    case $rest in
    *.*)
      go_major=${rest%%.*}
      rest=${rest#*.}
      go_min=${rest%%.*}
      case $go_major$go_min in
      '' | *[![:digit:]]*) ;;
      *)
        go_minor=$go_major.$go_min
        break
        ;;
      esac
      ;;
    esac
    ;;
  esac
done <"$REPO_ROOT/go.mod"
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
echo "verify-orphan-report: $IMAGE ($PLATFORM), --cpus ${CPUS:-1} — planting a leak, reading it back, then arming the kill"
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
  "$IMAGE" bash -c 'go test ./internal/posse -run TestOrphanReportControlNamesAPlantedLeak -v -timeout 20m'
