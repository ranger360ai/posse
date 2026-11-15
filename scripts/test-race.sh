#!/usr/bin/env bash
# test-race.sh — the on-demand `-race` arm over internal/posse's
# concurrency-carrying tests, as the THREE tag-aware commands it actually is
# (ranger-base-nhc23, correcting the one-command recipe in
# docs/notes.d/ranger-base-d0xvw.md).
#
# Usage:
#   scripts/test-race.sh            the census, then the three arms under -race
#   scripts/test-race.sh --census   the census alone — seconds, no -race build
#
# Environment:
#   GOBIN               which go to run (default: go)
#   POSSE_RACE_FILES    override the candidate file list, space separated,
#                       repo-relative. The negative control in
#                       internal/treepins/racearm_qa_test.go drives it.
#
# ─── WHY THIS IS A SCRIPT AND NOT A LINE IN A NOTES FILE ─────────────────────
#
# ranger-base-d0xvw decided a narrow on-demand `-race` arm over the tests that
# carry concurrency — the dispatch gather, passcarry, and the watch loop — and
# wrote the recipe as ONE `go test ./internal/posse -run <regex>`. The day
# before it was measured, ranger-base-qp1hm split internal/posse's tests into
# three build-tag binaries, and the candidate set straddles all three. So that
# one command compiled arm 1 and the shared files only, and 86 of the 106 names
# in its regex matched nothing. MEASURED 2026-09-10 by a `-list` census: the
# default build holds 20 of them, `-tags posse_arm2` 75, `-tags posse_arm3` 27.
# It priced 19% of the arm it describes and said `ok`.
#
# That is rot #4 in internal/treepins/armtags_qa_test.go's own header, verbatim:
# "a `go test -run` that matches no test exits 0". A hand-typed recipe cannot
# defend against it — ranger-base-y3x6n's recorded repro was eaten by the same
# trap and still prints `ok … [no tests to run]` today. So the recipe is a
# program, and the program does the two things prose cannot:
#
#   1. it derives the names from the FILES, every run. A test added to
#      dispatch_qa_test.go is in the arm the day it lands, and no list of 106
#      names goes stale in a document.
#   2. it CENSUSES before it runs. Every candidate name must be compiled into
#      at least one of the three arms; a name that is in none — a renamed file,
#      a fourth tag, a test deleted out from under the set — fails loudly in
#      seconds instead of being skipped in silence for eleven minutes.
#
# ─── WHAT IT COSTS, AND WHY IT IS NOT IN `make test` ─────────────────────────
#
# MEASURED 2026-11-15, D1 host (Linux 7.2.15, go1.27.3, 16 cores), warm build
# cache, over the whole 23-file candidate set, GREEN: **1106s — 18.4 minutes in
# three commands**, arm 1 366s, arm 2 302s, arm 3 438s, at a one-minute load
# average swinging 4.5-22 with BOTH suite slots held by two sibling seats
# throughout. `DATA RACE` 0 in all three.
#
# That is ~95s over the 2026-11-14 reading below, and the four tests
# ranger-base-1hnqc fixed are most of it: a budget that fires ends its test
# early, so the red arm was CHEAPER than the green one. Those four cost 152s
# more run to completion than they did dying on their deadlines (MEASURED
# 2026-11-15 filtered: 200.5s for arm 3's three against 108.3s, 59.2s for arm
# 2's one against 43.7s). Read the two totals as the same arm, not as a
# regression — and read neither against the other's load.
#
# Before that: MEASURED 2026-11-14, same host and cache, over the same 23-file
# set while it was RED: 1011s — 16.9 minutes, arm 1 317s, arm 2 348s, arm 3
# 345s, at a one-minute load average of 13.45 falling to 9.15 with two sibling
# seats' suites on the box for part of it. The eight-file set that preceded it
# read 707s on the same box an hour earlier (315 / 319 / 53, load 4.9-7.4), and
# the fifteen files ranger-base-87p6i added read 301s on their own
# (120 / 74 / 106, load 4.5-19.9) — so nearly doubling the set cost about 40%
# more wall, because what the old set spent was concentrated in a few
# long-backstop tests.
#
# Before that: MEASURED 2026-09-10, darwin/arm64, go1.26.5, over the EIGHT-file
# set: 51.4s without `-race`, 670.9s with it — ~13x, ~11.4 minutes. That ~13x
# is the number to reason with, and the cost is NOT the detector's per-access
# instrumentation: the same run sits at 13-14% CPU. It is wall clock — fixed
# intervals, backstops and barrier timeouts inside these tests stretching under
# the `-race` build. So this arm buys its signal with WAIT, and it is typed by
# hand when a change touches dispatch/passcarry/watch or a race is suspected.
# It is not wired into `make test`, `test-arm{1,2,3}` or CI.
#
# It takes no suite slot: a `-run` filter is not a full suite and is not queued
# (AGENTS.md, ranger-base-uvzjk). At 13% CPU over three binaries it is not what
# saturates this box.
#
# THE ARM IS GREEN, in all three arms, as of 2026-11-15 (ranger-base-1hnqc).
# It had never been green over the full 23-file set before that: the day the
# set grew, arm 2 had one red and arm 3 three, with `DATA RACE` 0 in all three
# — four tests that passed without `-race` and failed with it, at the same
# timings run alone as under load, because each was a budget spelled in WALL
# SECONDS against an apparatus `-race` stretches. Three of the four were in
# launchlock_test.go, outside this arm until ranger-base-87p6i, so nobody had
# seen them.
#
# If this arm reds again, that history is the first thing to rule out, and it
# is ruled out by the two readings the script gives you free: `DATA RACE` 0 in
# the summary, and the same `-run` filter green without `-race`. The fix is
# never a bigger constant. It is one of two mechanisms, chosen by asking
# whether the deadline is the test's OBSERVABLE or a backstop:
# `backstop(t)` in internal/posse/backstop_test.go, derived from the binary's
# own -timeout (ranger-base-khhnd, generalized by ranger-base-1hnqc — it is
# also why arm 1 was already `ok`), or `fakeCallCost`, a budget counted in
# fake-herdr CALLS (ranger-base-0dt50). That file's head comment states the
# class and what is NOT in it: a deadline bounded by something else in the
# fixture is an observable and stays a constant.
#
# ranger-base-0dt50's three arm-2 dispatch tests, which this paragraph used to
# name, are fixed and that bead is closed: its close recorded that this line
# and the Makefile's "not gate-clean today" had gone false and asked whoever
# owns the target to strike them. Struck, and this bead's line with it.
#
# A red here is a result, not a broken script; read the logs it names.

set -euo pipefail

cd "$(dirname "$0")/.."

GOBIN=${GOBIN:-go}

# The candidate set, BY FILE. "Concurrency-carrying" is a judgement about the
# product code — dispatch.go's gather fan-in, passcarry.go's fan-out and wake
# channel, watch.go's pulse/backup/guard/watchdog goroutines — so it is a list
# a person maintains, not something to infer from a test's name. The tests
# inside these files are read fresh on every run.
#
# A list a person maintains, and NOT a list nobody checks (ranger-base-87p6i).
# That judgement has a mechanical floor: a test file that runs one of those
# four loops, or `Watch` (which starts all four), on a goroutine of its own is
# carrying that concurrency whatever the file is called and whatever it
# asserts. internal/treepins/racearm_qa_test.go's arm 4 derives that floor from
# the test sources and reds when a file it proposes is in neither this list nor
# its priced `raceArmExcluded` table. It had to be written: for two months this
# list was eight files while FOURTEEN others launched one of those four loops
# on a goroutine, and a fifteenth the pass itself. The race ranger-base-np54s
# fixed was in one of the fourteen — suspend_qa_test.go, where
# `go d.watchdogLoop(...)` sits four lines above the poll that raced it — so
# the instrument built for that class was green over the file that had the
# race, and `make test` found it by luck under load instead.
#
# The derivation is a FLOOR and not the list, which is what keeps the judgement
# a person's: dispatch_qa_test.go and dispatchparity_qa_test.go launch nothing
# on a goroutine and are in the arm for dispatch.go's gather fan-in, and no
# reading of the test sources proposes them. The floor reaches the pass itself
# as `d.Run` on a goroutine — which is how launchlock_test.go got here, a
# fifteenth file the bead's own census did not name — but only for the three
# dispatcher spellings the package uses, because `Run` is also `t.Run` and
# `exec.Cmd.Run`. A dispatcher spelled some other way is a file somebody has
# to put in this list by hand, as ever.
#
# A row in `raceArmExcluded` is priced in the seconds it would add, like an
# ADR 0098 D2 treepins row, and nothing is priced out today. Note what an
# exclusion costs HERE, before writing one: a treepins exclusion moves a test
# to another route that still runs it, and this arm is the only route that runs
# anything under `-race`, so a row is a coverage hole and not a schedule.
CANDIDATE_FILES=${POSSE_RACE_FILES:-"
internal/posse/backoffhold_qa_test.go
internal/posse/backupjoin_qa_test.go
internal/posse/backuploop_test.go
internal/posse/dispatch_qa_test.go
internal/posse/dispatchparity_qa_test.go
internal/posse/drain_qa_test.go
internal/posse/guardclock_qa_test.go
internal/posse/hookwallsweep_qa_test.go
internal/posse/launcherlag_qa_test.go
internal/posse/launchlock_qa_test.go
internal/posse/launchlock_test.go
internal/posse/passcarry_qa_test.go
internal/posse/planstale_qa_test.go
internal/posse/pulse_test.go
internal/posse/pulse_verify_test.go
internal/posse/readyscan_test.go
internal/posse/secondstore_qa_test.go
internal/posse/suspend_qa_test.go
internal/posse/watch_test.go
internal/posse/watchhang_qa_test.go
internal/posse/watchlock_test.go
internal/posse/watchlog_test.go
internal/posse/watchpid_test.go
"}

die() { printf 'test-race.sh: %s\n' "$*" >&2; exit 2; }

census_only=0
case ${1:-} in
"") ;;
--census) census_only=1 ;;
*) die "unknown argument: $1 (usage: $0 [--census])" ;;
esac
[ $# -le 1 ] || die "too many arguments (usage: $0 [--census])"

work=${TMPDIR:-/tmp}/posse-race-$(date +%Y%m%d-%H%M%S)-$$
mkdir -p "$work"

# ─── THE NAMES ───────────────────────────────────────────────────────────────
names=$work/names.txt
: >"$names"
for f in $CANDIDATE_FILES; do
	[ -f "$f" ] || die "$f is in the candidate set and not in the tree — a renamed or deleted file takes its tests out of this arm silently, which is the whole reason this script exists"
	before=$(wc -l <"$names")
	grep -o '^func Test[A-Za-z0-9_]*' "$f" | sed 's/^func //' >>"$names"
	after=$(wc -l <"$names")
	[ "$after" -gt "$before" ] || die "$f contributes no top-level Test function — either the file lost its tests or the reader that finds them has gone blind"
done
sort -u -o "$names" "$names"
total=$(wc -l <"$names" | tr -d ' ')

# ─── THE CENSUS ──────────────────────────────────────────────────────────────
# Which of those names each arm actually COMPILES. `go test -list` builds the
# binary and asks it, which is the only answer that cannot be argued with; the
# tags decide the partition, so this runs without -race (the -race build is
# the same partition and ten minutes more expensive).
arm_tag=("" "posse_arm2" "posse_arm3")
covered=$work/covered.txt
: >"$covered"
arm_count=("" "" "")
for i in 0 1 2; do
	arm=$((i + 1))
	tag=${arm_tag[$i]}
	listed=$work/listed-arm$arm.txt
	if [ -z "$tag" ]; then
		$GOBIN test ./internal/posse -list '.*' -timeout 25m >"$listed"
	else
		$GOBIN test -tags "$tag" ./internal/posse -list '.*' -timeout 25m >"$listed"
	fi
	in_arm=$work/in-arm$arm.txt
	grep -x -F -f "$names" "$listed" | sort -u >"$in_arm" || :
	arm_count[$i]=$(wc -l <"$in_arm" | tr -d ' ')
	cat "$in_arm" >>"$covered"
done
sort -u -o "$covered" "$covered"

missing=$work/missing.txt
comm -23 "$names" "$covered" >"$missing"
printf '\ncandidate set: %s tests across %s files\n' "$total" "$(printf '%s\n' $CANDIDATE_FILES | grep -c .)"
printf '  arm 1 (default build)   %4s\n  arm 2 (-tags posse_arm2) %3s\n  arm 3 (-tags posse_arm3) %3s\n' \
	"${arm_count[0]}" "${arm_count[1]}" "${arm_count[2]}"
if [ -s "$missing" ]; then
	printf '\n%s\n' "$(cat "$missing")" >&2
	die "$(wc -l <"$missing" | tr -d ' ') of $total candidate tests are compiled into NO arm (named above). A \`go test -run\` over them exits 0 having run nothing — retag the file, fix the candidate list, or the arm is a green that measures nothing"
fi
printf '  every one of the %s reachable in some arm\n\n' "$total"

if [ "$census_only" = 1 ]; then
	rm -rf "$work"
	exit 0
fi

# ─── THE THREE ARMS ──────────────────────────────────────────────────────────
alts=$(tr '\n' '|' <"$names")
regex="^(${alts%|})$"

status=0
summary=$work/summary.txt
: >"$summary"
for i in 0 1 2; do
	arm=$((i + 1))
	tag=${arm_tag[$i]}
	log=$work/arm$arm.log
	if [ -z "$tag" ]; then
		printf '── arm %s · %s of %s tests · go test -race ./internal/posse -run <%s names> -count=1\n' \
			"$arm" "${arm_count[$i]}" "$total" "${arm_count[$i]}"
	else
		printf '── arm %s · %s of %s tests · go test -race -tags %s ./internal/posse -run <%s names> -count=1\n' \
			"$arm" "${arm_count[$i]}" "$total" "$tag" "${arm_count[$i]}"
	fi
	start=$SECONDS
	set +e
	if [ -z "$tag" ]; then
		$GOBIN test -race ./internal/posse -run "$regex" -count=1 -timeout 25m 2>&1 | tee "$log"
	else
		$GOBIN test -race -tags "$tag" ./internal/posse -run "$regex" -count=1 -timeout 25m 2>&1 | tee "$log"
	fi
	rc=${PIPESTATUS[0]}
	set -e
	elapsed=$((SECONDS - start))
	races=$(grep -c 'DATA RACE' "$log" || :)
	verdict=ok
	[ "$rc" = 0 ] || { verdict=FAIL; status=1; }
	# The rot this script exists for, caught on the run itself and not only in
	# the census: a filter that selected nothing still exits 0.
	if grep -q 'no tests to run' "$log"; then
		verdict='RAN NOTHING'
		status=1
	fi
	printf 'arm %s  %-11s %4ss  %s of %s tests  DATA RACE %s  %s\n' \
		"$arm" "$verdict" "$elapsed" "${arm_count[$i]}" "$total" "$races" "$log" >>"$summary"
done

printf '\n── the arm, three commands, %s tests\n' "$total"
cat "$summary"
printf '\nlogs: %s\n' "$work"
exit $status
