#!/usr/bin/env bash
# The live-box detective checks, run as one command (ranger-base-51z8j).
#
# WHY THIS EXISTS. Every check below was written as a control, declared in the
# Makefile, and pinned by a QA test -- and then invoked by nothing. Measured
# 2026-09-06 across the whole tree, before this script's own two targets
# existed: of the 21 verify-* targets there were then, four are
# prerequisites of `make test` (verify-test-times, verify-parallel,
# verify-suite-lock, verify-silent-reverts) and so run in CI; two more have
# their SCRIPT run by a target a person types (verify-gate-freshness.sh --warn
# at the end of `make install`, verify-detection.sh --check-install at the end
# of `make install-detection`); the remaining fifteen execute only when a
# person types them. No aggregate target listed one as a prerequisite, no
# workflow named one, and ~/Library/LaunchAgents held exactly one posse job and
# it was not this. So the codex cask moved on 2026-09-05 and nobody learned for
# a day, because the only thing that would have said so was a command nobody
# ran (ranger-base-k4lza). A one-shot remediation of a condition that
# regenerates is not a control; a recurring detective check is.
#
# THIS SCRIPT IS THE BODY OF THAT CONTROL AND NOT ITS SCHEDULE. It runs the
# checks, classifies each, prints, and exits. It installs nothing, schedules
# nothing, files nothing and writes nothing anywhere. Where a finding SURFACES
# -- a bead, a log, the session-start checklist, the cockpit -- is one decision
# for all of these at once and it is the operator's; so is any on-box schedule,
# which is a launchd install. Both are asked on ranger-base-51z8j. Until one is
# answered this is still a command a person types, which is honest: what it
# fixes today is that the person types ONE.
#
# THE ROSTER IS THE LIVE-BOX CHECKS AND ONLY THOSE. A check earns a place here
# by asserting the state of THIS MACHINE -- a pin, a credential path, an
# installed hook, an operator-owned gate copy, the shared beads store. Every
# other verify-* target is listed in EXCLUDED below with the reason, and
# boxcheck_qa_test.go fails if a verify-* target appears in the Makefile and in
# neither list. That two-way census is the point: the defect this script exists
# for was a control nobody noticed was unrun, and a roster that can go quietly
# out of date reproduces it one level up.
#
# READ-ONLY, EVERY ARM. Each script below is read-only by its own contract and
# is run with no arguments that change that. Nothing here kills a process
# (`Bash(pkill:*)` and `Bash(killall:*)` are denied fleet-wide and the reaper
# shape is what ec0beaa removed), deletes a file, or remediates a finding. A
# finding prints the line for a person to type, which is how each of these
# already behaves alone.
#
# EXIT STATUS, and the reason it is not just "did anything fail":
#   0  every check that could measure came back clean
#   1  at least one FINDING, or a check that failed in a way it does not
#      define (an ERROR -- a missing script, a crash, an unexpected status)
#   2  NOTHING WAS MEASURED: every check answered 2. That is the case on a
#      machine with none of the runtimes installed, and it is not a pass.
#      A schedule that treats 2 as green is a green light on an empty room.
#
# Per-check statuses come straight from each script and mean what that script
# says they mean: 0 ok, 1 finding, 2 nothing measured. Anything else is ERROR.
#
# A RED IN THE MIDDLE DOES NOT STOP THE RUN. `set -e` is deliberately not on:
# the whole value of an aggregate is that check six still runs after check two
# comes back red, and the --self-test arm that proves it is the one worth
# keeping.
set -uo pipefail

self=$(basename "$0")
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd) || {
  echo "$self: cannot locate the repo from $0" >&2; exit 2; }

quiet=0
selftest=0
case "${1:-}" in
  --self-test) selftest=1 ;;
  --quiet) quiet=1 ;;
  --self-test-run) ;;   # internal; see the re-exec note at the foot
  --census) ;;          # the two lists, for boxcheck_qa_test.go
  "") ;;
  *) echo "usage: $self [--quiet|--self-test]" >&2; exit 2 ;;
esac

# ---------------------------------------------------------------- the roster
#
# name<TAB>command. The command is run from the repo root.
#
#   the three version pins       grok / codex / beads, each asserted against
#                                the live binary and the live config rather
#                                than against its own declaration file.
#   verify-credential-paths      ADR 0019 path 3: a credential file that
#                                REGENERATES, which is why deleting it once
#                                was never the control.
#   verify-hook-freshness        the L3 hooks in every configured repo against
#                                a fresh render from the binary on PATH. Red in
#                                all three repos the day this script was
#                                written, tracked by nothing (ranger-base-u8lmw).
#   verify-gate-freshness        the operator-owned argv gate copy. Run WITHOUT
#                                --warn here: `make install` uses --warn because
#                                a stale gate must not fail a promote, but a
#                                detective run has no promote to protect and a
#                                finding is a finding.
#   verify-bd-no-relate-pairs    the drift detector for the symmetric-pair
#                                landmine, against the live store.
# The key is the MAKEFILE TARGET NAME, not a short label. There was a second
# list here mapping short keys back to targets, and it was a silent-drift path
# of exactly the kind this script exists for: dropping a roster row and leaving
# its name in the map would have had --census claim a check was rostered while
# the runner never ran it. One list, and the census reads it.
ROSTER=$(cat <<'ROSTER_EOF'
verify-grok-pin	scripts/verify-grok-pin.sh
verify-codex-pin	scripts/verify-codex-pin.sh
verify-bd-pin	scripts/verify-bd-pin.sh
verify-credential-paths	scripts/verify-credential-paths.sh
verify-hook-freshness	scripts/verify-hook-freshness.sh
verify-gate-freshness	scripts/verify-gate-freshness.sh
verify-bd-no-relate-pairs	scripts/verify-bd-dep-safety.sh --gate
ROSTER_EOF
)

# -------------------------------------------------------------- the excluded
#
# Every other verify-* target in the Makefile, with the reason it is not on a
# clock. boxcheck_qa_test.go reads BOTH lists and fails on a target in neither,
# so this table cannot fall behind the Makefile in silence.
#
# target<TAB>reason
EXCLUDED=$(cat <<'EXCLUDED_EOF'
verify-parallel	tree check; already a prerequisite of make test, so CI runs it
verify-test-times	tree check (--self-test); already a prerequisite of make test
verify-suite-lock	tree check (--self-test); already a prerequisite of make test
verify-silent-reverts	tree check (--self-test); already a prerequisite of make test
verify-gotest	tree check (--self-test) of the reusing wrapper; measures no box state
verify-detection	promote-time probe: replays fixtures against THIS CHECKOUT in a throwaway XDG_CONFIG_HOME. install-detection already runs its --check-install half
verify-prune-guard	promote-time probe of a BINARY against scratch state; POSSE= names the candidate, so it belongs to a promote and not to a clock
verify-id-recycle	promote-time probe; scratch --session herdr server
verify-self-close	promote-time probe; scratch HOME and scratch --session, and it ends processes it started
verify-govern-honesty	promote-time probe; kills its own scratch watch loop with -9 by design
verify-bd-argv-gate	tree check of the gate SOURCE in this checkout; ~23s and it measures no box state. verify-gate-freshness is the box half and is on the roster
verify-pid-deny-set	the TARGET reads HOME_DIR=examples, i.e. this repo own seed PIDs -- a tree check. Its live readers --live and --settings are off the target on purpose (Makefile): --live answers 2 on an idle box and 1 whenever a session is mid-bead behind a PID edit, both correct, so on a clock it is a nuisance generator rather than a control. Whether this box wants an advisory tier for checks like that is asked on ranger-base-51z8j
verify-bd-dep-safety	the reporting half of the same script; verify-bd-no-relate-pairs is its --gate and is on the roster as no-relate-pairs
verify-runtime-walk	SPENDS A REAL TURN on the runtime under test. Event-triggered by design -- before switching a lane back onto a runtime, and after a version bump. A schedule would spend money on a clock, which crosses crew guardrail 1
verify-box	this script -- the aggregate itself
verify-box-self-test	the arms of the aggregate itself; a tree check, and putting it on the roster would have verify-box run itself
EXCLUDED_EOF
)

# `--census` prints the two lists in one machine-readable form so the QA test
# reads what the script actually holds rather than re-parsing it. Both are read
# out of the same tables the runner uses; there is no third copy to drift.
if [ "${1:-}" = "--census" ]; then
  printf '%s\n' "$ROSTER" | while IFS=$'\t' read -r t _; do
    [ -n "$t" ] && echo "roster	$t"
  done
  printf '%s\n' "$EXCLUDED" | while IFS=$'\t' read -r t _; do
    [ -n "$t" ] && echo "excluded	$t"
  done
  exit 0
fi

# ------------------------------------------------------------------ the run
run_roster() {
  local ok=0 finding=0 nothing=0 error=0 lines="" detail=""
  local name cmd rc out
  while IFS=$'\t' read -r name cmd; do
    [ -n "$name" ] || continue
    if [ ! -x "$root/${cmd%% *}" ]; then
      lines+=$(printf '  %-26s %s' "$name" "ERROR    ${cmd%% *} is missing or not executable")
      lines+=$'\n'
      error=$((error + 1))
      detail+="--- $name (could not run)"$'\n'"$root/${cmd%% *} is missing or not executable"$'\n\n'
      continue
    fi
    out=$( cd "$root" && eval "$cmd" 2>&1 )
    rc=$?
    case $rc in
      0) ok=$((ok + 1));           lines+=$(printf '  %-26s %s' "$name" "ok") ;;
      1) finding=$((finding + 1)); lines+=$(printf '  %-26s %s' "$name" "FINDING") ;;
      2) nothing=$((nothing + 1)); lines+=$(printf '  %-26s %s' "$name" "not measured") ;;
      *) error=$((error + 1));     lines+=$(printf '  %-26s %s' "$name" "ERROR    exit $rc") ;;
    esac
    lines+=$'\n'
    # A finding's own output is the whole product -- it names the file, the
    # version, the repo, and the line to type. An ERROR's output is how anyone
    # learns what broke. Both are kept; a clean check's output is kept only
    # when not --quiet, because a clock that mails six clean reports teaches
    # its reader to stop opening them.
    if [ $rc -ne 0 ] || [ $quiet -eq 0 ]; then
      detail+="--- $name (exit $rc)"$'\n'"$out"$'\n\n'
    fi
  done <<< "$ROSTER"

  local total=$((ok + finding + nothing + error))
  printf '%s' "$detail"
  echo "$self: live-box checks, $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '%s' "$lines"
  echo

  if [ $((finding + error)) -gt 0 ]; then
    echo "$self: $finding finding(s), $error error(s) of $total check(s) -- the box is not what the repo says it is"
    return 1
  fi
  if [ "$total" -gt 0 ] && [ "$nothing" -eq "$total" ]; then
    echo "$self: NOTHING MEASURED -- all $total check(s) exited 2. That is not a pass."
    return 2
  fi
  echo "$self: $ok ok, $nothing not measured, of $total check(s)"
  return 0
}

# ---------------------------------------------------------------- self-test
#
# What can go wrong with an aggregate, and the arm for each:
#
#   1. it stops at the first red, so checks after a finding never run. The
#      classic shape, and the reason `set -e` is off. The arm plants a red in
#      the MIDDLE of a roster and asserts the last check still ran.
#   2. it calls an exit 2 a pass. "Nothing measured" is the answer a runner
#      with no codex gives, and it is exactly the answer this control must not
#      launder -- the whole bead is about a check that was silent.
#   3. it calls a mixed run green because SOME check passed.
#   4. an unexpected exit status (a crash, 127, a signal) is scored as one of
#      the three known verdicts instead of ERROR.
#   5. a missing or non-executable script is scored silently rather than as an
#      ERROR -- the shape that would let someone delete a check and keep a
#      green board.
#
# Each arm has a control that must come out the other way; a rig that only
# ever produces the verdict it is looking for measures nothing.
selftest_tmp=""
selftest_cleanup() { [ -n "${selftest_tmp:-}" ] && rm -rf "$selftest_tmp"; }

self_test() {
  local tmp fails=0
  tmp=$(mktemp -d) || { echo "$self --self-test: mktemp failed" >&2; exit 2; }
  selftest_tmp=$tmp
  trap selftest_cleanup EXIT

  mkdir -p "$tmp/scripts"
  local rc
  for rc in 0 1 2 9; do
    printf '#!/bin/sh\necho "arm-%s speaking"\nexit %s\n' "$rc" "$rc" > "$tmp/scripts/arm$rc.sh"
    chmod 0755 "$tmp/scripts/arm$rc.sh"
  done
  printf '#!/bin/sh\nexit 0\n' > "$tmp/scripts/notexec.sh"
  chmod 0644 "$tmp/scripts/notexec.sh"
  cp "$0" "$tmp/scripts/$self"
  chmod 0755 "$tmp/scripts/$self"

  # THE MATCHER DOES NOT FORK, and neither does the reporting beside it
  # (ranger-base-t07yx, ranger-base-7hx87, swept by ranger-base-s8b4g and
  # enforced tree-wide by selftestforkarm_qa_test.go). A `grep -q` that is
  # signalled, or that cannot be exec'd under load, makes an arm report the
  # property false when the apparatus is what failed -- and this file is
  # scanned whole, not just its self-test. `case` is a shell builtin and
  # cannot fail to run; the indent below is a `read` loop for the same reason.
  indent() {
    local __l
    while IFS= read -r __l; do echo "        $__l"; done <<< "$1"
  }

  arm() { # arm <label> <want-rc> <want-substring> <roster rows...>
    local label=$1 want=$2 want_sub=$3; shift 3
    local roster="" got out r
    for r in "$@"; do roster+="$r"$'\n'; done
    out=$( ROSTER_OVERRIDE="$roster" "$tmp/scripts/$self" --self-test-run 2>&1 ); got=$?
    if [ "$got" -ne "$want" ]; then
      echo "  FAIL $label: exit $got, want $want"; indent "$out"
      fails=$((fails + 1)); return
    fi
    if [ -n "$want_sub" ]; then
      case "$out" in
        *"$want_sub"*) ;;
        *) echo "  FAIL $label: output does not carry [$want_sub]"; indent "$out"
           fails=$((fails + 1)); return ;;
      esac
    fi
    echo "  ok   $label"
  }

  echo "$self --self-test:"
  # 1 and its control: a red in the middle must not eat the checks after it.
  #
  # The witness is the TOTAL, not the last check's output. "arm-0 speaking"
  # was the first witness here and it is ambiguous -- the FIRST check in this
  # roster is also arm0.sh, so a runner that broke at the middle red still
  # printed it and the arm passed a `break` mutant (measured). Only a count of
  # three separates "all three ran" from "the first one did".
  arm 'a finding in the middle does not stop the run' 1 'of 3 check(s)' \
      "first"$'\t'"scripts/arm0.sh" "middle"$'\t'"scripts/arm1.sh" "last"$'\t'"scripts/arm0.sh"
  arm 'control: the same roster with no red is 0' 0 'last' \
      "first"$'\t'"scripts/arm0.sh" "middle"$'\t'"scripts/arm0.sh" "last"$'\t'"scripts/arm0.sh"
  # 2 and its control: every check saying "nothing measured" is 2, not 0.
  arm 'all-2 is NOTHING MEASURED, not a pass' 2 'NOTHING MEASURED' \
      "a"$'\t'"scripts/arm2.sh" "b"$'\t'"scripts/arm2.sh"
  arm 'control: one real pass among the 2s is 0' 0 'not measured' \
      "a"$'\t'"scripts/arm2.sh" "b"$'\t'"scripts/arm0.sh"
  # 3: a pass anywhere does not launder a finding.
  arm 'a pass does not launder a finding' 1 'FINDING' \
      "a"$'\t'"scripts/arm0.sh" "b"$'\t'"scripts/arm1.sh"
  # 4 and its control: an unexpected status is ERROR, not a verdict.
  arm 'exit 9 is ERROR' 1 'ERROR    exit 9' \
      "a"$'\t'"scripts/arm9.sh"
  arm 'control: exit 1 is a FINDING and not an ERROR' 1 'FINDING' \
      "a"$'\t'"scripts/arm1.sh"
  # 5 and its control: a check that cannot run is ERROR, never silence.
  arm 'a missing script is ERROR' 1 'is missing or not executable' \
      "a"$'\t'"scripts/gone.sh"
  arm 'a non-executable script is ERROR' 1 'is missing or not executable' \
      "a"$'\t'"scripts/notexec.sh"
  arm 'control: the same shape made executable is ok' 0 'ok' \
      "a"$'\t'"scripts/arm0.sh"

  echo
  if [ "$fails" -gt 0 ]; then echo "$self --self-test: $fails arm(s) FAILED"; return 1; fi
  echo "$self --self-test: all arms pass"
  return 0
}

# The self-test re-execs THIS script with a substituted roster rather than
# refactoring the runner into something the real path does not use: the arms
# must exercise the same run_roster the box run does, or they pin a replica
# and the defect walks between the two.
if [ "${1:-}" = "--self-test-run" ]; then
  ROSTER="${ROSTER_OVERRIDE:-}"
  quiet=0
  run_roster
  exit $?
fi

if [ "$selftest" = 1 ]; then
  self_test
  exit $?
fi

run_roster
exit $?
