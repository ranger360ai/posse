#!/usr/bin/env bash
# Prune the symmetric `relates-to` pairs that make bd 0.49.1 diverge
# (ranger-base-nusr, mechanism in ranger-base-pkqn).
#
# Usage: scripts/prune-bd-relates-to.sh              # dry run: print the plan
#        scripts/prune-bd-relates-to.sh --apply      # record + remove the pairs
#        BEADS_DB=/path/to/beads.db scripts/prune-bd-relates-to.sh [--apply]
#
# BEADS_DB may name a store in ANOTHER repository; the writes are still made
# from inside the repo that owns it, never with a store-selecting flag from
# here (store_owner below, github issue 9 / ranger-base-9mjxb). A store whose
# owning repo cannot be named is refused rather than written to.
#
# WHY. bd writes a `relates-to` link as two rows, one per direction, so every
# one is a 2-cycle. bd's `AddDependency` cycle check is a `UNION ALL` recursive
# CTE with no visited set, depth 100, following every edge type — it enumerates
# walks, not nodes, and a pair makes it bounce ~7x per level. So `bd dep add`
# and `bd create --deps` onto ANY node upstream of a pair never return.
# Measured on a snapshot of the fleet db, 2026-08-27:
#
#   create --deps discovered-from:ranger-base-okbr   before: killed at 90s,
#                                                    issue committed, edge lost
#                                                    after prune: 0.43s, edge present
#
# The relations themselves carry no scheduling meaning here — bd's ready queue
# gates on `blocks` alone — so the edge is pure provenance, and provenance
# belongs in a comment. This records each link as a comment on BOTH beads
# before removing it, so nothing is lost, then unlinks the pair.
#
# DURABILITY. Two verbs plant a pair. `bd dep relate` (and its deprecated
# alias `bd relate`) writes both rows in one call. `bd dep add -t relates-to`
# writes a single row per call and is NOT harmless: two calls in opposite
# directions write both rows too — bd 0.49.1's cycle check does not consult
# direction, so the second call is accepted (measured, ranger-base-uw8g,
# correcting an earlier note that said the opposite). So the prune does NOT
# hold on a deny of `bd dep relate` alone; `scripts/verify-bd-dep-safety.sh
# --gate` is the detector that catches a pair from either verb.
#
# BLAST RADIUS of --apply: deletes the `relates-to` dependency rows of every
# symmetric pair (two rows per pair) and adds two comments per pair. It touches
# no issue, no status, and no `blocks` / `discovered-from` / `blocked-by` edge.
# Reversible: the removed rows are in the git history of .beads/issues.jsonl.
# Every one of those writes is made with the working directory inside the repo
# that owns the store, with no store-selecting flag on the argv, and with
# BEADS_DIR shed for the call — that variable outranks the cwd, so the chdir
# does not bind the store while it is set (see THE OWNING REPO below).
set -euo pipefail

APPLY=0
case "${1:-}" in
	--apply) APPLY=1 ;;
	--dry-run | "") ;;
	*) echo "usage: $(basename "$0") [--apply]" >&2; exit 2 ;;
esac

find_db() {
	if [ -n "${BEADS_DB:-}" ]; then
		printf '%s\n' "$BEADS_DB"
		return
	fi
	local dir=.beads
	# One redirect hop, the same as bd 0.49.1 allows.
	if [ -f "$dir/redirect" ]; then
		dir=$(tr -d '[:space:]' <"$dir/redirect")
	fi
	printf '%s\n' "$dir/beads.db"
}

DB=$(find_db)
[ -f "$DB" ] || { echo "prune-bd-relates-to: no beads db at $DB" >&2; exit 2; }
command -v sqlite3 >/dev/null || { echo "prune-bd-relates-to: sqlite3 not on PATH" >&2; exit 2; }

# ABSOLUTE from here on. find_db returns `.beads/beads.db` by default, every
# bd call below is made from a DIFFERENT working directory (see store_owner),
# and a relative path would then name a store that is not the one measured.
DB=$(cd "$(dirname "$DB")" && pwd -P)/$(basename "$DB")

# THE OWNING REPO, and why every bd call below is made from inside it
# (github.com/ranger360ai/posse issue 9, ranger-base-9mjxb). The operator
# measured bd auto-importing the CWD repo's `issues.jsonl` into the database
# named by an explicit `--db` -- a read verb writing another repository's
# records into the named store. We are pinned at 0.50.3 and will not file
# upstream (standing ruling 2026-10-08), so the posse-side answer is to stop
# handing bd a `--db` that points outside the working directory at all: name
# the repo that OWNS the store, chdir into it, and let bd resolve its own.
#
# THE CHDIR IS HALF OF IT. bd resolves `$BEADS_DIR` BEFORE the cwd, and in
# no-db mode reads no redirect at all (ADR 0055), so a chdir binds nothing
# while that variable is set — and posse sets it in every session it
# launches, which is every session this script is ever typed in. MEASURED
# 2026-10-09, pinned bd 0.50.3, two scratch git repos each holding a touched
# copy of a real beads.db, controls both ways: with the variable shed each
# cwd resolves its own store; with cwd=repoA and the variable naming repoB,
# resolution AND the write go to repoB — a `comments add` from inside repoA
# left repoA at 0 marker rows and put 1 in repoB. So every bd call below is
# `env -u BEADS_DIR bd …`, which is the remedy AGENTS.md prescribes and the
# same one internal/posse/beads.go's bdStoreEnvShed makes for the Go runner
# (ranger-base-k45gn, ranger-base-7ebv6).
#
# `store_owner <db>` prints that repo, or fails. Three conditions, all of
# them load-bearing, because a chdir to the wrong directory silently reads a
# DIFFERENT database than the one the SQL above measured:
#
#   1. the db sits in a `.beads` directory          -> its parent is the repo
#   2. that parent is a git work tree's TOPLEVEL    -> bd resolves from there
#   3. that repo's own `.beads` resolves BACK to this store, one redirect hop
#      included -- this is the condition the redirect shape needs, since
#      `<repo>/.beads/redirect` can name a store in another repo entirely.
#
# Both sides of 2 and 3 are canonicalised with `pwd -P`: on darwin
# `/tmp/x` and `/private/tmp/x` are the same directory spelled two ways, and
# a string compare of the two refuses a store that is perfectly fine.
store_owner() {
	_so_db=$1
	_so_store=$(dirname "$_so_db")
	[ "$(basename "$_so_store")" = ".beads" ] || return 1
	_so_owner=$(dirname "$_so_store")
	[ -d "$_so_owner" ] || return 1
	_so_owner=$(cd "$_so_owner" && pwd -P) || return 1
	_so_top=$(git -C "$_so_owner" rev-parse --show-toplevel 2>/dev/null) || return 1
	[ -n "$_so_top" ] || return 1
	_so_top=$(cd "$_so_top" && pwd -P) || return 1
	[ "$_so_top" = "$_so_owner" ] || return 1
	_so_resolved=$_so_owner/.beads
	if [ -f "$_so_resolved/redirect" ]; then
		_so_resolved=$(tr -d '[:space:]' <"$_so_resolved/redirect")
	fi
	[ -d "$_so_resolved" ] || return 1
	_so_resolved=$(cd "$_so_resolved" && pwd -P) || return 1
	[ "$_so_resolved" = "$(cd "$_so_store" && pwd -P)" ] || return 1
	printf '%s\n' "$_so_owner"
}

# Resolved once, before the plan is printed, so a dry run says what --apply
# would do rather than leaving the operator to find out at the write. The
# safe failure direction differs between the two: a dry run writes nothing,
# so it says so and carries on; --apply refuses, because the only way to
# make those writes without an owning repo is the `--db` this exists to
# avoid.
OWNER=$(store_owner "$DB" || true)
if [ -z "$OWNER" ]; then
	echo "prune-bd-relates-to: cannot name the repo that owns $DB" >&2
	echo "  A store is <repo>/.beads/beads.db where <repo> is a git work tree's" >&2
	echo "  toplevel and <repo>/.beads resolves back to that store. Without one," >&2
	echo "  the only way to write it is a store-selecting flag from somewhere" >&2
	echo "  else, which is what auto-imports the cwd repo's own records into" >&2
	echo "  it, so it is refused here (github issue 9)." >&2
	echo "  Re-run from inside the owning repo, or prune it by hand there." >&2
	if [ "$APPLY" = 1 ]; then
		exit 2
	fi
	echo "  dry run continues: nothing below runs bd." >&2
fi

# Reading a beads db read-only is not always possible: a WAL-mode db whose
# `-shm` file is gone (no live writer) cannot be opened with `mode=ro` at all —
# sqlite refuses to create the shared-memory file and returns CANTOPEN(14).
# That state has no writer by definition, so a copy is a faithful snapshot, and
# reading the copy keeps the promise that this never writes the fleet db.
RO_TMPS=""
cleanup_ro() { [ -z "$RO_TMPS" ] || rm -rf $RO_TMPS; }
trap cleanup_ro EXIT
DB_READ=""
pick_reader() {
	if sqlite3 "file:${DB}?mode=ro" "SELECT 1" >/dev/null 2>&1; then
		DB_READ="file:${DB}?mode=ro"
		return
	fi
	local tmp
	tmp=$(mktemp -d) || { echo "$(basename "$0"): mktemp failed" >&2; exit 2; }
	RO_TMPS="$RO_TMPS $tmp"
	cp "$DB" "$tmp/beads.db"
	[ -f "$DB-wal" ] && cp "$DB-wal" "$tmp/beads.db-wal"
	DB_READ="$tmp/beads.db"
}

q() {
	[ -n "$DB_READ" ] || pick_reader
	sqlite3 "$DB_READ" "$1"
}

# One row per symmetric pair, ordered so the output is stable.
PAIRS_SQL="
SELECT a.type || ' ' || a.issue_id || ' ' || a.depends_on_id
  FROM dependencies a
  JOIN dependencies b
    ON a.issue_id = b.depends_on_id
   AND a.depends_on_id = b.issue_id
   AND a.type = b.type
 WHERE a.issue_id < a.depends_on_id
 ORDER BY 1"

pairs=$(q "$PAIRS_SQL")
if [ -z "$pairs" ]; then
	echo "clean: no symmetric dependency pair in $DB — nothing to prune"
	exit 0
fi

# `bd dep unrelate` only knows relates_to. A symmetric pair of any other type
# is a different animal and is not this script's to guess at.
foreign=$(printf '%s\n' "$pairs" | grep -v '^relates-to ' || true)
if [ -n "$foreign" ]; then
	echo "prune-bd-relates-to: symmetric pair(s) of a type this cannot unlink:" >&2
	printf '%s\n' "$foreign" | sed 's/^/  /' >&2
	echo "  Remove those by hand (bd dep remove, both directions) and re-run." >&2
	exit 2
fi

n=$(printf '%s\n' "$pairs" | grep -c . || true)
echo "db: $DB"
echo "symmetric relates-to pairs: $n  (rows to delete: $((n * 2)))"
echo

NOTE_TAG="edge pruned $(date +%F) (ranger-base-nusr)"
NOTE_WHY="bd 0.49.1's cycle check diverges on symmetric pairs; the relation is this note."

while read -r _type x y; do
	[ -n "${x:-}" ] || continue
	if [ "$APPLY" = 1 ]; then
		# cwd = the owning repo, and no --db: see store_owner above.
		# --no-daemon is the house form for every bd argv (ADR 0015) and
		# keeps this from leaving a daemon behind (ranger-base-42mv).
		# `env -u BEADS_DIR`: see THE OWNING REPO above — the chdir alone
		# does not bind the store while that variable is set, and posse sets
		# it in every session (ADR 0055).
		(cd "$OWNER" && env -u BEADS_DIR bd --no-daemon comments add "$x" "relates-to $y — $NOTE_TAG: $NOTE_WHY") >/dev/null
		(cd "$OWNER" && env -u BEADS_DIR bd --no-daemon comments add "$y" "relates-to $x — $NOTE_TAG: $NOTE_WHY") >/dev/null
		(cd "$OWNER" && env -u BEADS_DIR bd --no-daemon dep unrelate "$x" "$y") >/dev/null
		echo "pruned $x <-> $y (recorded as a comment on both)"
	else
		echo "would run (cwd ${OWNER:-<no owning repo>}):"
		echo "  env -u BEADS_DIR bd --no-daemon comments add $x \"relates-to $y — $NOTE_TAG: $NOTE_WHY\""
		echo "  env -u BEADS_DIR bd --no-daemon comments add $y \"relates-to $x — $NOTE_TAG: $NOTE_WHY\""
		echo "  env -u BEADS_DIR bd --no-daemon dep unrelate $x $y"
	fi
done <<EOF
$pairs
EOF

echo
if [ "$APPLY" != 1 ]; then
	echo "dry run — nothing changed. Re-run with --apply to prune."
	echo "Then flush and commit the projection in the beads repo:"
	echo "  bd sync --flush-only && git -C <beads repo> commit .beads/issues.jsonl -m 'bd: prune relates-to pairs'"
	exit 0
fi

# bd has written since the first read; re-decide how to read.
DB_READ=""
left=$(q "$PAIRS_SQL")
if [ -n "$left" ]; then
	echo "prune-bd-relates-to: pairs remain after --apply:" >&2
	printf '%s\n' "$left" | sed 's/^/  /' >&2
	exit 1
fi
echo "clean: no symmetric dependency pair left in $DB"
echo "Flush and commit the projection so an import cannot resurrect them:"
echo "  bd sync --flush-only && git -C <beads repo> commit .beads/issues.jsonl -m 'bd: prune relates-to pairs'"
