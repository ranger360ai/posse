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
# MEASURED 2026-09-10, darwin/arm64, go1.26.5, `-count=1`, over the whole
# candidate set: 51.4s without `-race`, 670.9s with it — ~13x, ~11.4 minutes
# in three commands. The cost is NOT the detector's per-access instrumentation:
# the same run sits at 13-14% CPU. It is wall clock — fixed intervals,
# backstops and barrier timeouts inside these tests stretching under the
# `-race` build. So this arm buys its signal with WAIT, and it is typed by hand
# when a change touches dispatch/passcarry/watch or a race is suspected. It is
# not wired into `make test`, `test-arm{1,2,3}` or CI.
#
# It takes no suite slot: a `-run` filter is not a full suite and is not queued
# (AGENTS.md, ranger-base-uvzjk). At 13% CPU over three binaries it is not what
# saturates this box.
#
# THE ARM IS NOT GREEN TODAY, and this script does not hide that. Three arm-2
# dispatch tests fail under `-race` and pass without it, with `DATA RACE` count
# 0 — filed as ranger-base-0dt50. Arms 1 and 3 are `ok` since
# ranger-base-khhnd took the watch-loop backstop off its 30s constant and onto
# the binary's own deadline. A red here is a result, not a broken script; read
# the logs it names.

set -euo pipefail

cd "$(dirname "$0")/.."

GOBIN=${GOBIN:-go}

# The candidate set, BY FILE. "Concurrency-carrying" is a judgement about the
# product code — dispatch.go's gather fan-in, passcarry.go's fan-out and wake
# channel, watch.go's pulse/backup/guard/watchdog goroutines — so it is a list
# a person maintains, not something to infer from a test's name. The tests
# inside these files are read fresh on every run.
CANDIDATE_FILES=${POSSE_RACE_FILES:-"
internal/posse/dispatch_qa_test.go
internal/posse/dispatchparity_qa_test.go
internal/posse/passcarry_qa_test.go
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
