#!/usr/bin/env bash
# Prune the symmetric `relates-to` pairs that make bd 0.49.1 diverge
# (ranger-base-nusr, mechanism in ranger-base-pkqn).
#
# Usage: scripts/prune-bd-relates-to.sh              # dry run: print the plan
#        scripts/prune-bd-relates-to.sh --apply      # record + remove the pairs
#        BEADS_DB=/path/to/beads.db scripts/prune-bd-relates-to.sh [--apply]
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
		bd --db "$DB" comments add "$x" "relates-to $y — $NOTE_TAG: $NOTE_WHY" >/dev/null
		bd --db "$DB" comments add "$y" "relates-to $x — $NOTE_TAG: $NOTE_WHY" >/dev/null
		bd --db "$DB" dep unrelate "$x" "$y" >/dev/null
		echo "pruned $x <-> $y (recorded as a comment on both)"
	else
		echo "would run:"
		echo "  bd comments add $x \"relates-to $y — $NOTE_TAG: $NOTE_WHY\""
		echo "  bd comments add $y \"relates-to $x — $NOTE_TAG: $NOTE_WHY\""
		echo "  bd dep unrelate $x $y"
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
