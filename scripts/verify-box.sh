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
# checks, classifies each, prints, records the verdict, and exits. It installs
# nothing, schedules nothing, files no bead and remediates nothing. Where a
# finding SURFACES, and whether this box runs it on a clock, were both asked on
# ranger-base-51z8j and both answered by the operator on 2026-09-06
# (ranger-base-0x1wc, "d plus b, yes to launchagent after g code lands"): the
# surface is the governance row G10 in `posse status` and the cockpit, and the
# schedule is a daily user LaunchAgent whose plist is versioned in
# $CONSTITUTION/scripts/launchd/, not here.
#
# SO THERE IS EXACTLY ONE WRITE, and this is the whole of it: the last run's
# verdict, to $RHQ_HOME/state/verify-box.yaml (write_state below). It is the
# only store G10 reads -- no second stamp, no index, nothing else writes it --
# and posse dates every reading against `verify_box_max_age:` so that a verdict
# nobody has refreshed since the schedule's interval renders STALE rather than
# green. That freshness rule is what makes a ONE-LINE state file honest: this
# script never has to report that it did not run, because not running is
# exactly what an ageing file says.
#
# AND A RUN THAT DIES BEFORE ITS VERDICT STILL LEAVES A LINE. The first thing
# a run prints is a start line carrying the same stamp it will write, and the
# plist points StandardOutPath/StandardErrorPath at
# $RHQ_HOME/state/verify-box.log -- so a killed, wedged or crashed run leaves
# the start line and whatever the shell said about it, in the file G10's stale
# row names. A job that logs only what it finished cannot say what killed it.
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
# AND THE CENSUS IS OVER SCRIPTS TOO, not over Makefile targets alone
# (ranger-base-bbl6r finding 1, escaped ranger-base-51z8j). Both lists above
# are keyed on a TARGET NAME, so the one shape this control could not see was
# the shape it exists for: a check shipped as scripts/verify-*.sh with no
# target at all is in neither list, is named by nothing, and has a green board
# over it. Two were in the tree on the day this was written and neither had
# ever been classified. So UNTARGETED below is the third list -- every
# scripts/verify-*.sh that is no target's recipe, with the reason -- and
# boxcheck_qa_test.go fails on a script in neither the Makefile nor that list.
#
# THREE MORE THINGS THE CENSUS NOW CHECKS RATHER THAN TAKES ON TRUST, because
# a table of sentences nothing measures is the same silence one level further
# in (ranger-base-bbl6r findings 2 and 3):
#   - an EXCLUDED reason that says `make test` runs it is checked against the
#     `test:` prerequisite line. Drop the prerequisite and the check runs
#     NOWHERE while this table still says CI has it.
#   - a ROSTER command is checked against that target's own recipe. A recipe
#     that gains or loses a flag would otherwise drift in silence, and the
#     failure is running the wrong invocation under the target's name.
#   - every reason and command is non-empty, so a row cannot satisfy the
#     census by existing.
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

# posse's home, resolved as internal/posse/app.go resolves it: an explicit
# RHQ_HOME, else POSSE_HOME, else ~/.config/posse, else the pre-0015
# ~/.config/rhq if that is what this box still has. The state file goes under
# it.
#
# POSSE_HOME WAS MISSING HERE (ranger-base-lvzm7 finding 3): app.go's newApp
# reads RHQ_HOME then POSSE_HOME before ever touching $HOME, and this script
# jumped straight from RHQ_HOME to the two-name fallback. On a box where
# POSSE_HOME is set and RHQ_HOME is not, that made the producer and G10's
# reader resolve different homes -- this script would write one file and
# posse status would read another, forever NEVER RUN. app.go's own comment
# says the RHQ_HOME read is the one to drop when the rename window closes, so
# the gap only widens with time if it is not read the same way here.
#
# AN EXPLICIT RHQ_HOME OR POSSE_HOME WINS WHETHER OR NOT THE DIRECTORY EXISTS
# YET, which is where this parts company with verify-hook-freshness.sh's
# one-liner. That one falls through to the legacy home when the named one is
# not there, and this script WRITES: applying that fallback to a named home
# would send a scratch run's verdict into a real state directory. --self-test
# runs every arm under a scratch RHQ_HOME for exactly that reason, and the
# fallback must not quietly undo it.
home="${RHQ_HOME:-}"
if [ -z "$home" ]; then
  home="${POSSE_HOME:-}"
fi
if [ -z "$home" ]; then
  home="$HOME/.config/posse"
  [ -d "$home" ] || { [ -d "$HOME/.config/rhq" ] && home="$HOME/.config/rhq"; }
fi
state_file="$home/state/verify-box.yaml"

quiet=0
selftest=0
case "${1:-}" in
  --self-test) selftest=1 ;;
  --quiet) quiet=1 ;;
  --self-test-run) ;;   # internal; see the re-exec note at the foot
  --census) ;;          # the three lists, for boxcheck_qa_test.go
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

# ------------------------------------------------------- the untargeted scripts
#
# Every scripts/verify-*.sh that is NO target's recipe, with the reason it has
# no target. boxcheck_qa_test.go globs the directory and fails on a script that
# is neither named in a Makefile recipe nor listed here, so a check cannot
# arrive in scripts/ and be invoked by nothing in silence -- which is exactly
# what had happened to both rows below.
#
# The bar for a row here is that the script CANNOT be run by a target on this
# box, not that nobody got round to adding one. Anything a person could type
# `make` for gets a target instead.
#
# script<TAB>reason
UNTARGETED=$(cat <<'UNTARGETED_EOF'
verify-ghost-composer.sh	needs an UNCAGED shell: it drives a real claude in a scratch herdr pane, and from a `cage: seatbelt` seat claude never reaches a composer at all -- the header of that script says so and keeps the negative as the finding. Event-triggered, and the bead it was written for (ranger-base-2hvtv) was answered another way in the end (internal/posse/sentline.go). A target here would offer a `make` line that cannot work from the seat most likely to type it
verify-orphan-report.sh	runs in a throwaway CPU-limited container and MUST NOT run on this box: it plants busy loops on purpose, and the standing operator rule -- after sixteen leaked ones froze the fleet for 2.5 hours -- is that a persona generates no load here (ranger-base-teau). It also waits out the real orphan age floor, so it takes minutes. A make target for it would be a target whose only correct use is somewhere this Makefile does not run
UNTARGETED_EOF
)

# `--census` prints the three lists in one machine-readable form so the QA test
# reads what the script actually holds rather than re-parsing it. All three are
# read out of the same tables the runner uses; there is no fourth copy to drift.
#
# THE SECOND FIELD IS THE ROW'S OWN SECOND FIELD -- a roster row's COMMAND, an
# exclusion's or an untargeted script's REASON. It used to be dropped here, and
# a census that prints only names can only ever check names: that is what left
# "already a prerequisite of make test, so CI runs it" and the hand-written
# invocations measured by nothing (ranger-base-bbl6r findings 2 and 3).
if [ "${1:-}" = "--census" ]; then
  printf '%s\n' "$ROSTER" | while IFS=$'\t' read -r t rest; do
    [ -n "$t" ] && printf 'roster\t%s\t%s\n' "$t" "$rest"
  done
  printf '%s\n' "$EXCLUDED" | while IFS=$'\t' read -r t rest; do
    [ -n "$t" ] && printf 'excluded\t%s\t%s\n' "$t" "$rest"
  done
  printf '%s\n' "$UNTARGETED" | while IFS=$'\t' read -r t rest; do
    [ -n "$t" ] && printf 'untargeted\t%s\t%s\n' "$t" "$rest"
  done
  exit 0
fi

# ------------------------------------------------------------------ the run
# write_state records the run's verdict where posse's governance row G10 reads
# it (internal/posse/verifybox.go). One file, written once, at the end of a run.
#
# ATOMIC, and via a FIXED sibling name rather than a mktemp: a reader that
# catches a half-written file would report the box unverifiable, and this
# script deletes nothing (boxcheck_qa_test.go's read-only scan forbids `rm`),
# so a crashed write must leave one predictable file the next run overwrites
# rather than a growing pile of temporaries.
#
# A FAILURE TO WRITE IS NOT A FAILURE OF THE RUN. The checks already ran and
# their verdict is already on stdout; losing the record must not turn a clean
# box into exit 1. It says so on stderr and the file simply ages out, which is
# the same thing G10 will report either way.
write_state() { # write_state <stamp> <rc> <checks-block>
  local stamp=$1 rc=$2 checks=$3 tmp
  tmp="$state_file.new"
  mkdir -p "$(dirname "$state_file")" 2>/dev/null || {
    echo "$self: cannot create $(dirname "$state_file") -- no verdict recorded" >&2; return 0; }
  {
    echo "# The last live-box run's verdict. Written by scripts/verify-box.sh,"
    echo "# read by posse's governance row G10. Hand edits are pointless: the"
    echo "# next run overwrites this file whole."
    echo "at: $stamp"
    echo "rc: $rc"
    echo "checks:"
    printf '%s' "$checks"
  } > "$tmp" 2>/dev/null && mv "$tmp" "$state_file" 2>/dev/null || {
    echo "$self: cannot write $state_file -- no verdict recorded" >&2; return 0; }
  return 0
}

run_roster() {
  local ok=0 finding=0 nothing=0 error=0 lines="" detail="" checks=""
  local name cmd rc out started
  # ONE stamp for the whole run, taken before the first check and used by the
  # start line, the report header and the state file alike. The header used to
  # take its own `date` at report time; two stamps for one run is two answers
  # to "when was this box last checked", and the freshness rule is decided on
  # that answer. The START is the conservative one -- a run that takes twenty
  # minutes is dated from when it began looking.
  started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  # The line a dying run leaves behind. It is printed BEFORE any check runs
  # and it is not suppressed by --quiet: the plist points stdout and stderr at
  # $RHQ_HOME/state/verify-box.log, and a run that is killed before it can
  # write a verdict leaves exactly this and whatever the shell said after it.
  echo "$self: run started $started"
  while IFS=$'\t' read -r name cmd; do
    [ -n "$name" ] || continue
    if [ ! -x "$root/${cmd%% *}" ]; then
      lines+=$(printf '  %-26s %s' "$name" "ERROR    ${cmd%% *} is missing or not executable")
      lines+=$'\n'
      error=$((error + 1))
      checks+="  $name: error"$'\n'
      detail+="--- $name (could not run)"$'\n'"$root/${cmd%% *} is missing or not executable"$'\n\n'
      continue
    fi
    out=$( cd "$root" && eval "$cmd" 2>&1 )
    rc=$?
    # The state file's token per check is the same classification, spelled
    # for a machine: posse's reader keys on it (internal/posse/verifybox.go's
    # four VerifyBox* constants), so the tokens and the human column are set
    # in one `case` and cannot come to mean different things.
    case $rc in
      0) ok=$((ok + 1));           lines+=$(printf '  %-26s %s' "$name" "ok")
         checks+="  $name: ok"$'\n' ;;
      1) finding=$((finding + 1)); lines+=$(printf '  %-26s %s' "$name" "FINDING")
         checks+="  $name: finding"$'\n' ;;
      2) nothing=$((nothing + 1)); lines+=$(printf '  %-26s %s' "$name" "not measured")
         checks+="  $name: not-measured"$'\n' ;;
      *) error=$((error + 1));     lines+=$(printf '  %-26s %s' "$name" "ERROR    exit $rc")
         checks+="  $name: error"$'\n' ;;
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
  echo "$self: live-box checks, $started"
  printf '%s' "$lines"
  echo

  # A CHECKLESS ROSTER IS NOTHING MEASURED, NOT A PASS (ranger-base-lvzm7
  # finding 1). With total=0, nothing=0 too, so the `[ "$total" -gt 0 ] &&`
  # guard this line used to carry made an empty roster fall all the way
  # through to verdict=0 -- a green board over a room with nothing in it, the
  # producer's half of the same defect internal/posse/verifybox.go's
  # verifyBoxVerdict refused. Dropping the guard leaves `[ "$nothing" -eq
  # "$total" ]` vacuously true at zero and zero, so an empty roster reads 2
  # like every other run that measured nothing.
  local verdict=0
  if [ $((finding + error)) -gt 0 ]; then
    verdict=1
  elif [ "$nothing" -eq "$total" ]; then
    verdict=2
  fi
  # RECORDED BEFORE IT IS ANNOUNCED. The write is the thing a scheduled run
  # exists to leave behind -- the summary line goes to a log nobody may read,
  # the verdict goes to the surface -- so it happens before the return and on
  # every arm, including the two red ones.
  write_state "$started" "$verdict" "$checks"

  case $verdict in
    1) echo "$self: $finding finding(s), $error error(s) of $total check(s) -- the box is not what the repo says it is"
       return 1 ;;
    2) echo "$self: NOTHING MEASURED -- all $total check(s) exited 2. That is not a pass."
       return 2 ;;
  esac
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

  # EVERY ARM RUNS AGAINST A SCRATCH RHQ_HOME. run_roster now WRITES -- one
  # state file, the verdict G10 reads -- so an arm inheriting the operator's
  # home would publish a fixture's verdict as this box's own, and the surface
  # would report a planted red or a two-check roster as the live-box answer.
  local state state_text
  mkdir -p "$tmp/home/state"
  state="$tmp/home/state/verify-box.yaml"

  # read_state slurps that file WITHOUT FORKING, for the reason the matcher
  # below does not fork: a `cat` that is signalled or cannot be exec'd under
  # load would make every state arm report the file empty, which is precisely
  # the verdict these arms exist to distinguish from a real one.
  read_state() {
    local __l
    state_text=""
    [ -f "$state" ] || return 0
    while IFS= read -r __l; do state_text+="$__l"$'\n'; done < "$state"
    return 0
  }

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
    out=$( ROSTER_OVERRIDE="$roster" RHQ_HOME="$tmp/home" "$tmp/scripts/$self" --self-test-run 2>&1 ); got=$?
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

  # 6 and its controls: the run RECORDS its verdict where G10 reads it.
  #
  # The record is the whole reason a scheduled run is worth anything -- the
  # summary goes to a log nobody opens, the verdict goes to the surface -- and
  # it is written by a code path no other arm above touches. Three properties,
  # each with a control that must come out the other way, because a state file
  # that always says the same thing pins nothing:
  #   - the per-check TOKENS are the run's own classification, not a constant
  #   - the RUN VERDICT (rc:) is recorded on the red arms too, not only green
  #   - the STAMP is there and is the one the start line announced
  state_arm() { # state_arm <label> <want-substring>...
    local label=$1; shift
    local want
    for want in "$@"; do
      case "$state_text" in
        *"$want"*) ;;
        *) echo "  FAIL $label: $state does not carry [$want]"; indent "$state_text"
           fails=$((fails + 1)); return ;;
      esac
    done
    echo "  ok   $label"
  }

  arm 'a mixed run still reports 1' 1 'FINDING' \
      "red"$'\t'"scripts/arm1.sh" "clean"$'\t'"scripts/arm0.sh" "blind"$'\t'"scripts/arm2.sh"
  read_state
  state_arm 'the verdict file records each check by name' \
      'red: finding' 'clean: ok' 'blind: not-measured' 'rc: 1'
  # The control: the SAME check names, all green, must write different tokens
  # and a different rc. Without it every assertion above passes on a writer
  # that hard-codes one table.
  arm 'control: the same names all green' 0 'ok' \
      "red"$'\t'"scripts/arm0.sh" "clean"$'\t'"scripts/arm0.sh" "blind"$'\t'"scripts/arm0.sh"
  read_state
  state_arm 'control: the tokens follow the run, not the roster' \
      'red: ok' 'blind: ok' 'rc: 0'
  # NOTHING MEASURED is recorded as its own verdict and not as a pass -- the
  # one classification this whole control exists to refuse to launder.
  arm 'all-2 is recorded, not laundered' 2 'NOTHING MEASURED' \
      "a"$'\t'"scripts/arm2.sh" "b"$'\t'"scripts/arm2.sh"
  read_state
  state_arm 'the verdict file records rc 2' 'a: not-measured' 'rc: 2'
  # The stamp: present, and the one the start line announced. A record whose
  # stamp does not match its own run is a freshness rule reading someone
  # else's clock.
  read_state
  local stamp=""
  case "$state_text" in
    *"at: "*) stamp=${state_text#*at: }; stamp=${stamp%%$'\n'*} ;;
  esac
  if [ -z "$stamp" ]; then
    echo "  FAIL the verdict file carries an at: stamp"; indent "$state_text"
    fails=$((fails + 1))
  else
    out=$( ROSTER_OVERRIDE="a"$'\t'"scripts/arm0.sh"$'\n' RHQ_HOME="$tmp/home" "$tmp/scripts/$self" --self-test-run 2>&1 )
    read_state
    local started=""
    case "$out" in
      *"run started "*) started=${out#*run started }; started=${started%%$'\n'*} ;;
    esac
    # An EMPTY $started is checked before it is compared, and this is not
    # belt-and-braces: `at: ` is a prefix of every record this script writes,
    # so a run that printed no start line at all would match the case below
    # and the arm would report the two halves agreeing when one of them was
    # missing. Measured -- with the start line deleted, this arm passed.
    if [ -z "$started" ]; then
      echo "  FAIL the run printed no start line, so a run that dies before its verdict leaves nothing"; indent "$out"
      fails=$((fails + 1))
    else
      case "$state_text" in
        *"at: $started"*) echo "  ok   the stamp on the record is the one the start line announced" ;;
        *) echo "  FAIL the start line said [$started] and the record does not carry it"; indent "$state_text"
           fails=$((fails + 1)) ;;
      esac
    fi
  fi

  # A CHECKLESS ROSTER IS NOTHING MEASURED, NOT A PASS (ranger-base-lvzm7
  # finding 1, the producer's half of the same defect
  # internal/posse/verifybox.go's verifyBoxVerdict refused). An empty roster
  # must not compute verdict=0 the way `[ "$total" -gt 0 ] && ...` used to
  # leave it.
  arm 'an empty roster is NOTHING MEASURED, not a pass' 2 'NOTHING MEASURED'

  # --quiet KEEPS THE START LINE (ranger-base-lvzm7 finding 4). The claim
  # beside run_roster's start-line echo is that it is "not suppressed by
  # --quiet" -- the operator's whole ruling (b) on how a killed run leaves a
  # trace -- and every arm above runs at quiet=0, so nothing had ever run
  # the substituted roster with quiet=1. QUIET_OVERRIDE is that seam.
  out=$( ROSTER_OVERRIDE="a"$'\t'"scripts/arm0.sh"$'\n' QUIET_OVERRIDE=1 RHQ_HOME="$tmp/home" "$tmp/scripts/$self" --self-test-run 2>&1 )
  case "$out" in
    *"run started "*) echo "  ok   --quiet keeps the start line" ;;
    *) echo "  FAIL --quiet suppressed the start line -- a killed run under this flag would leave nothing"; indent "$out"
       fails=$((fails + 1)) ;;
  esac
  case "$out" in
    *"arm-0 speaking"*)
      echo "  FAIL --quiet did not suppress a clean check's own output"; indent "$out"
      fails=$((fails + 1)) ;;
    *) echo "  ok   --quiet drops a clean check's own output" ;;
  esac
  # The control: the SAME roster at quiet=0 keeps the clean check's output --
  # without it, a runner that always suppressed it (or always kept it) would
  # pass one of the two lines above for the wrong reason.
  out=$( ROSTER_OVERRIDE="a"$'\t'"scripts/arm0.sh"$'\n' RHQ_HOME="$tmp/home" "$tmp/scripts/$self" --self-test-run 2>&1 )
  case "$out" in
    *"arm-0 speaking"*) echo "  ok   control: quiet=0 keeps a clean check's own output" ;;
    *) echo "  FAIL control: quiet=0 dropped a clean check's own output"; indent "$out"
       fails=$((fails + 1)) ;;
  esac

  # POSSE_HOME RESOLVES THE SAME WAY app.go's newApp DOES (ranger-base-lvzm7
  # finding 3): RHQ_HOME unset, POSSE_HOME named, must land the state file
  # under POSSE_HOME -- the exact split that let this script write one file
  # while G10 read another on any box where only POSSE_HOME is set.
  local phome pfake
  phome="$tmp/posse-home"; pfake="$tmp/fake-home"
  mkdir -p "$phome" "$pfake"
  out=$( env -u RHQ_HOME POSSE_HOME="$phome" HOME="$pfake" ROSTER_OVERRIDE="a"$'\t'"scripts/arm0.sh"$'\n' "$tmp/scripts/$self" --self-test-run 2>&1 )
  if [ -f "$phome/state/verify-box.yaml" ]; then
    echo "  ok   POSSE_HOME resolves the state file the way app.go resolves posse's home"
  else
    echo "  FAIL POSSE_HOME did not receive the verdict"; indent "$out"
    fails=$((fails + 1))
  fi
  # The control: with NEITHER name set, the file must fall back to
  # $HOME/.config/posse rather than the POSSE_HOME directory from the arm
  # above -- without this, a script that always wrote to POSSE_HOME's path
  # regardless of the environment would pass the assertion above for the
  # wrong reason.
  rm -rf "$phome/state"
  out=$( env -u RHQ_HOME -u POSSE_HOME HOME="$pfake" ROSTER_OVERRIDE="a"$'\t'"scripts/arm0.sh"$'\n' "$tmp/scripts/$self" --self-test-run 2>&1 )
  if [ -f "$pfake/.config/posse/state/verify-box.yaml" ] && [ ! -f "$phome/state/verify-box.yaml" ]; then
    echo "  ok   control: with neither name set, the file falls back to \$HOME/.config/posse"
  else
    echo "  FAIL control: with neither name set, the fallback resolved to the wrong home"; indent "$out"
    fails=$((fails + 1))
  fi

  echo
  if [ "$fails" -gt 0 ]; then echo "$self --self-test: $fails arm(s) FAILED"; return 1; fi
  echo "$self --self-test: all arms pass"
  return 0
}

# The self-test re-execs THIS script with a substituted roster rather than
# refactoring the runner into something the real path does not use: the arms
# must exercise the same run_roster the box run does, or they pin a replica
# and the defect walks between the two.
#
# QUIET_OVERRIDE is that same seam for --quiet (ranger-base-lvzm7 finding 4):
# without it this re-exec hard-set quiet=0 always, and the flag parser above
# is a single-arg `case` with no way to hand --self-test-run a --quiet of its
# own, so the claim beside the start line's echo -- "not suppressed by
# --quiet" -- was true and measured by nothing. A regression moving that echo
# behind the quiet gate would take the claim out with the suite green.
if [ "${1:-}" = "--self-test-run" ]; then
  ROSTER="${ROSTER_OVERRIDE:-}"
  quiet="${QUIET_OVERRIDE:-0}"
  run_roster
  exit $?
fi

if [ "$selftest" = 1 ]; then
  self_test
  exit $?
fi

run_roster
exit $?
