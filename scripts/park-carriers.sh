#!/usr/bin/env bash
# park-carriers.sh — which LIVE refs still reach a parked commit
# (ranger-base-zlvyk, from ranger-base-8h17w).
#
#   scripts/park-carriers.sh                 census every refs/heads/parked/*
#   scripts/park-carriers.sh <park>...       census just these parks
#   scripts/park-carriers.sh --self-test     prove the census can still say YES,
#                                            name the carrier's worktree, and say NO
#
#   exit 0  no landable branch reaches any parked commit
#   exit 1  at least one does — every carrier is named, with the worktree
#           holding it when a seat has it checked out
#   exit 2  refused: not a git repo, no base branch, or no park to census
#
# WHY THIS EXISTS. A push hold parks the held commits on a
# `refs/heads/parked/<name>` ref and resets the base branch out from under
# them. That removes ONE ref's claim on those commits and leaves every OTHER
# ref already cut from that tip pointing straight at them — every live session
# branch included, because a session branch is cut from the base's tip at
# dispatch and the park does not go back and move it.
#
# Nothing enumerated those refs. MEASURED 2026-11-22 over ranger-base-8h17w's
# park: the hold bead named two carrying branches by hand; there were three,
# and the third (posse/richard-posse-ranger-base-uymh6) was found by this
# census in seconds and has since been parked itself. A park that does not run
# this leaves the held commits reachable from an unknown number of live seats,
# and when one of those seats closes its bead the launcher lands the branch on
# the base — its fast-forward refuses, and the REPLAY behind that does not
# (ranger-base-3s2ig, measured). So the held commits come back under new shas
# and the hold is paid for a second time.
#
# THE CARRYING SET IS CLOSED AT PARK TIME, which is what makes this cheap and
# complete: a seat dispatched AFTER the park cuts from the reset base and
# carries nothing. So the census is worth the most in the minute after the
# park, and it is still correct whenever it is run.
#
# THE HELD SET IS `<park> --not <base>`, not the park's whole history: a park
# ref holds the base it was cut from too, and everything on that base is on
# every branch by construction. What makes a commit held is that the park is
# the only intended way to reach it.
#
# A CARRIER IS A REF A LANDING CAN READ, which is refs/heads/<branch> outside
# the park namespace and nothing else. Two other classes reach held commits
# and neither sets the exit status, because neither can put them back on the
# base:
#
#   ANOTHER PARK. parked/uymh6-902b36a8 reaches parked/repzh-a50aa688's pair
#   because the second park was cut from a tip that already carried the first.
#   Both are held; that containment is the point.
#
#   POSSE'S OWN RECORDS, refs/posse/*. MergeSessionWork lands refs/heads/<the
#   session branch> and reads no other namespace, so a refs/posse/retired/*
#   ref cannot land anything. What it does is keep the objects reachable, so
#   gc will not prune held content a park's own ref no longer names. MEASURED
#   2026-11-22: posse/retired/posse/dinesh-posse-ranger-base-oe884 reaches the
#   commits two parks hold, which is what retiring a branch whose tip was
#   parked leaves behind. Printed, not counted.
#
# WHAT THIS IS NOT. It is a census, not a gate: it reads refs and nothing
# else, it moves nothing, and a carrier it names is a human's to resolve — a
# branch checked out by a live seat cannot have its ref moved anyway, and the
# base cannot be reset under a seat that is mid-flight, which is why each
# carrier prints with the worktree holding it. Enforcement at landing time is
# ranger-base-3s2ig's.
#
# NO ARRAYS AND NO mapfile: the sets here are newline-delimited strings,
# because macOS ships bash 3.2, where `"${a[@]}"` over an empty array is an
# unbound-variable error under `set -u` and `mapfile` does not exist at all
# (the same reason scripts/verify-pid-deny-set.sh carries its sets as strings).
set -euo pipefail

# THE BASE IS THE BRANCH THE PARK'S OWN RESET MOVED, and the default is it.
# The override is for a repository whose base branch is named otherwise, and
# for the self-test — not for naming a remote-tracking ref. A base BEHIND the
# reset point pulls commits that are legitimately on the base branch into the
# held set below, and every live ref that reaches one of them then reads as a
# carrier: MEASURED 2026-11-22 (ranger-base-3bhxb), `origin/main` two landings
# behind the base turned one held commit and one carrier into three and three.
# Both runs exit 1 and look alike, which is why every line prints $base.
base=${PARK_CARRIERS_BASE:-main}
park_glob='refs/heads/parked/*'

# Local branches and posse's own ref namespaces. refs/heads is where a carrier
# that can re-reach the base lives — including a session branch whose WORKTREE
# is gone but whose ref survives, which is not inert: ranger-base-zlvyk
# measured that posse prunes the stale registration and then runs
# `git worktree add PATH BRANCH` when the branch exists (worktree.go), so a
# re-dispatch into that bead lands the new seat on the stale branch's TIP.
# refs/posse is scanned for the reachability it keeps, not for a landing it
# could make; the classes are in the header.
carrier_namespaces="refs/heads refs/posse"

die2() {
	echo "park-carriers: $*" >&2
	exit 2
}

# The worktree that has this branch checked out, if any — the first thing a
# reader of this census needs to know about a carrier, because that ref cannot
# be moved while a seat holds it.
worktree_of() {
	local want="$1" path="" line
	while read -r line; do
		case "$line" in
		"worktree "*)
			path="${line#worktree }"
			;;
		"branch refs/heads/$want")
			echo "  [checked out at $path]"
			return 0
			;;
		esac
	done < <(git worktree list --porcelain)
	return 0
}

# Prints every ref outside the park namespace that reaches one of $held.
# Returns 1 when at least one did, so the caller's status is the answer.
report_carriers() {
	local park="$1" held="$2"
	local live="" parked="" records="" ref short c n

	while read -r ref short; do
		if [ "$ref" = "$park" ] || [ "$short" = "${park#refs/heads/}" ]; then
			continue
		fi
		for c in $held; do
			if git merge-base --is-ancestor "$c" "$ref" 2>/dev/null; then
				case "$ref" in
				$park_glob) parked="$parked$short"$'\n' ;;
				refs/posse/*) records="$records$short"$'\n' ;;
				*) live="$live$short"$'\n' ;;
				esac
				break
			fi
		done
	done < <(git for-each-ref --format='%(refname) %(refname:short)' $carrier_namespaces)

	if [ -n "$parked" ]; then
		echo "  also parked (held by design, not a carrier):"
		printf '%s' "$parked" | while read -r short; do echo "    $short"; done
	fi
	if [ -n "$records" ]; then
		echo "  also reachable from posse's own records (lands nothing; keeps the objects from gc):"
		printf '%s' "$records" | while read -r short; do echo "    $short"; done
	fi
	if [ -z "$live" ]; then
		echo "  carried by no live ref"
		return 0
	fi
	n=$(printf '%s' "$live" | grep -c . || true)
	echo "  CARRIED BY $n live ref(s):"
	for short in $live; do
		echo "    $short $(git rev-parse --short "$short")$(worktree_of "$short")"
	done
	return 1
}

census() {
	local found=0 park held n c
	for park in "$@"; do
		if ! git rev-parse --verify --quiet "$park^{commit}" >/dev/null; then
			die2 "$park does not name a commit"
		fi
		held=$(git rev-list "$park" --not "$base")
		n=$(printf '%s' "$held" | grep -c . || true)
		if [ "$n" -eq 0 ]; then
			echo "park $park holds nothing that is not already on $base"
			continue
		fi
		echo "park $park holds $n commit(s) not on $base:"
		for c in $held; do
			echo "    $(git log -1 --format='%h %s' "$c")"
		done
		if ! report_carriers "$park" "$held"; then
			found=1
		fi
	done
	return "$found"
}

# Prove the census can say YES, can name the worktree holding the carrier, and
# can say NO. Hermetic: a scratch repo under
# a fresh temp root, nothing on this box read or touched. The root is spelled
# `${TMPDIR:-/tmp}/posse-<what>.XXXXXX` and not `mktemp -t`, because
# scripts/reap-test-tmp.sh's one shell class globs `posse-*` under /tmp and
# $TMPDIR and a root spelled any other way is reaped by nothing
# (internal/treepins/mktemproot_qa_test.go, which refused the first draft of
# this script). It is removed on the pass and on every fail path rather than
# from an EXIT trap.
self_test() {
	local d repo me out rc wt
	me="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
	d=$(mktemp -d "${TMPDIR:-/tmp}/posse-park-carriers.XXXXXX")
	repo=$d/repo
	git init -q -b main "$repo"
	git -C "$repo" config user.email park@example.invalid
	git -C "$repo" config user.name park
	: >"$repo/a"
	git -C "$repo" add -- a
	git -C "$repo" commit -q -m base -- a
	# A commit on main, a branch cut from that tip, then the park: the hold's
	# exact shape — the park keeps the commit, the branch still reaches it,
	# and main is reset out from under both.
	: >"$repo/held"
	git -C "$repo" add -- held
	git -C "$repo" commit -q -m held -- held
	git -C "$repo" branch seat
	git -C "$repo" branch parked/held main
	git -C "$repo" reset -q --hard HEAD~1

	rc=0
	out=$(cd "$repo" && PARK_CARRIERS_BASE=main bash "$me") || rc=$?
	echo "$out"
	if [ "$rc" != 1 ]; then
		echo "SELF-TEST FAIL: carried park exited $rc, want 1" >&2
		rm -rf -- "${d:?}"
		return 1
	fi
	case "$out" in
	*"CARRIED BY 1 live ref"*seat*) ;;
	*)
		echo "SELF-TEST FAIL: carrier 'seat' not named" >&2
		rm -rf -- "${d:?}"
		return 1
		;;
	esac

	# Give that same carrier a worktree of its own: the row must now carry the
	# `[checked out at <path>]` annotation, naming that directory.
	#
	# This is the one field on a carrier row a reader ACTS on — a ref a live
	# seat holds cannot be moved at all (this script's header, and step 4 of
	# docs/runbooks/push-hold-park.md) — and until ranger-base-3bhxb no arm
	# ran its producer at all, because the fixture built `seat` in the scratch
	# repo and never checked it out anywhere. MEASURED 2026-11-22 by
	# ranger-base-j7ekm, two blindings with the other arms green through both:
	# the `[checked out at ...]` echo deleted, and worktree_of matched against
	# a branch name that cannot exist. Six other mutants of this script were
	# killed by those arms, so this was the door's one hole.
	wt=$d/park-seat-wt
	git -C "$repo" worktree add -q "$wt" seat
	rc=0
	out=$(cd "$repo" && PARK_CARRIERS_BASE=main bash "$me") || rc=$?
	echo "$out"
	if [ "$rc" != 1 ]; then
		echo "SELF-TEST FAIL: carried park with a worktree exited $rc, want 1" >&2
		rm -rf -- "${d:?}"
		return 1
	fi
	# Matched on the trailing path component rather than byte-equal to $wt:
	# git records the worktree path as IT resolves it, which on a box whose
	# temp root is a symlink (darwin's /tmp -> /private/tmp) is not the string
	# mktemp printed. The basename is what a reader of the row would check.
	case "$out" in
	*"seat "*"[checked out at "*"/park-seat-wt]"*) ;;
	*)
		echo "SELF-TEST FAIL: carrier 'seat' is checked out at $wt and its row does not say so" >&2
		rm -rf -- "${d:?}"
		return 1
		;;
	esac

	# Remove the only carrier: the same park must read clean and exit 0.
	git -C "$repo" worktree remove --force "$wt"
	git -C "$repo" branch -q -D seat
	rc=0
	out=$(cd "$repo" && PARK_CARRIERS_BASE=main bash "$me") || rc=$?
	echo "$out"
	if [ "$rc" != 0 ]; then
		echo "SELF-TEST FAIL: uncarried park exited $rc, want 0" >&2
		rm -rf -- "${d:?}"
		return 1
	fi

	rm -rf -- "${d:?}"
	echo "SELF-TEST OK: the census says YES when a live ref carries a park, names the worktree holding that ref, and says NO when none does"
}

if [ "${1-}" = "--self-test" ]; then
	self_test
	exit $?
fi

git rev-parse --git-dir >/dev/null 2>&1 || die2 "not a git repository"
git rev-parse --verify --quiet "$base^{commit}" >/dev/null ||
	die2 "base branch '$base' does not exist (PARK_CARRIERS_BASE names it)"

if [ $# -gt 0 ]; then
	census "$@"
else
	parks=$(git for-each-ref --format='%(refname)' "$park_glob")
	[ -n "$parks" ] || die2 "no $park_glob ref to census"
	census $parks
fi
