#!/usr/bin/env bash
# Audit the work graph for the bd 0.49.1 `dep add` landmine (ranger-base-pkqn).
#
# Usage: scripts/verify-bd-dep-safety.sh                 # print the audit, exit 0
#        scripts/verify-bd-dep-safety.sh <target-id>     # exit 1 if unsafe
#        scripts/verify-bd-dep-safety.sh --gate          # exit 1 if ANY pair exists
#        BEADS_DB=/path/to/beads.db scripts/verify-bd-dep-safety.sh [<id>]
#
# bd 0.49.1's cycle check is a recursive CTE over the WHOLE dependency graph,
# written with UNION ALL — it enumerates walks, not nodes, to a fixed depth of
# 100, following every dependency type. `relates-to` edges are always symmetric
# (bd writes both directions), so each one is a 2-cycle the walk bounces across
# ~7x per level. A `dep add` whose TARGET can reach such a pair does not
# terminate, and holds the sqlite write lock while it doesn't. See NOTES.md,
# *beads (bd) substrate*.
#
# "Target" is the second argument of `bd dep add <issue> <target>` and the id
# in `bd create --deps <type>:<target>` — the node the walk starts from. A
# brand-new bead is always safe; anything with outgoing edges may not be.
#
# The pairs CAN be pruned and the prune DOES hold (measured, ranger-base-nusr),
# and `scripts/prune-bd-relates-to.sh` removes them. TWO verbs plant a pair,
# not one: `bd dep relate` / the deprecated `bd relate` writes both rows in a
# call, and two `bd dep add -t relates-to` calls in opposite directions plant
# the identical pair — bd accepts the second unconditionally (measured,
# ranger-base-uw8g, correcting an earlier note that called `dep add` harmless).
# `dep add -t relates-to` is not denyable by pattern, so `--gate` here, not a
# settings.json deny list, is what keeps the store at zero; wire it into CI or
# run `make verify-bd-no-relate-pairs`.
#
# Read-only: the db is opened `mode=ro`, and the recursive queries here use
# UNION (memoised), so they are cheap where bd's own is not.
set -euo pipefail

GATE=0
TARGET=${1:-}
if [ "$TARGET" = "--gate" ]; then
	GATE=1
	TARGET=
fi

find_db() {
	if [ -n "${BEADS_DB:-}" ]; then
		printf '%s\n' "$BEADS_DB"
		return
	fi
	local dir=.beads
	# One redirect hop, the same as bd 0.49.1 allows (measured: chains are not).
	if [ -f "$dir/redirect" ]; then
		dir=$(tr -d '[:space:]' <"$dir/redirect")
	fi
	printf '%s\n' "$dir/beads.db"
}

DB=$(find_db)
[ -f "$DB" ] || { echo "verify-bd-dep-safety: no beads db at $DB" >&2; exit 2; }
command -v sqlite3 >/dev/null || { echo "verify-bd-dep-safety: sqlite3 not on PATH" >&2; exit 2; }

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

# Nodes sitting in a symmetric pair — the 2-cycles the walk explodes on.
CYCLE_NODES_SQL="
SELECT DISTINCT a.issue_id
  FROM dependencies a
  JOIN dependencies b
    ON a.issue_id = b.depends_on_id
   AND a.depends_on_id = b.issue_id
   AND a.type = b.type"

# Unsafe targets: every node from which some 2-cycle node is reachable by
# following depends_on edges — i.e. every node bd's walk would start from and
# then fall into a pair. UNION, so this one terminates.
UNSAFE_SQL="
WITH RECURSIVE cyc(n) AS ($CYCLE_NODES_SQL),
unsafe(n) AS (
  SELECT n FROM cyc
  UNION
  SELECT d.issue_id FROM dependencies d JOIN unsafe u ON d.depends_on_id = u.n
)
SELECT n FROM unsafe ORDER BY n"

if [ "$GATE" = 1 ]; then
	cyc=$(q "$CYCLE_NODES_SQL ORDER BY 1")
	n=$(printf '%s\n' "$cyc" | grep -c . || true)
	if [ "$n" -gt 0 ]; then
		echo "UNSAFE: $n node(s) sit in a symmetric dependency pair in $DB." >&2
		printf '%s\n' "$cyc" | sed 's/^/  /' >&2
		echo "  Each pair is a 2-cycle that makes bd 0.49.1's cycle check diverge," >&2
		echo "  so 'bd dep add' / 'bd create --deps' onto anything upstream of one" >&2
		echo "  never returns. Prune: scripts/prune-bd-relates-to.sh --apply" >&2
		echo "  Do not run 'bd dep relate' / 'bd relate' in this fleet." >&2
		exit 1
	fi
	echo "clean: no symmetric dependency pair in $DB"
	exit 0
fi

if [ -n "$TARGET" ]; then
	hit=$(q "$UNSAFE_SQL" | grep -Fx -- "$TARGET" || true)
	if [ -n "$hit" ]; then
		echo "UNSAFE: $TARGET is a bd 0.49.1 dep-add landmine."
		echo "  A 2-cycle is reachable from it, so 'bd dep add <x> $TARGET' and"
		echo "  'bd create --deps <type>:$TARGET' will hang holding the write lock."
		echo "  Record the provenance as a comment instead: bd comments add <x> ..."
		exit 1
	fi
	echo "safe: $TARGET (no symmetric relates-to pair reachable from it)"
	exit 0
fi

pairs=$(q "SELECT count(*) FROM ($CYCLE_NODES_SQL)")
echo "db: $DB"
echo
echo "nodes in a symmetric (2-cycle) dependency pair: $pairs"
q "$CYCLE_NODES_SQL ORDER BY 1" | sed 's/^/  /'
echo
unsafe=$(q "$UNSAFE_SQL")
echo "unsafe as a 'dep add' / '--deps' TARGET: $(printf '%s\n' "$unsafe" | grep -c . || true)"
printf '%s\n' "$unsafe" | sed 's/^/  /'
echo
echo "Never dep-add onto one of those. Comment the provenance instead."
echo "The list grows on its own: an ordinary bead landing upstream of a pair"
echo "joins it, with no new relates-to edge. TWO verbs plant a pair: 'bd dep"
echo "relate' / 'bd relate' in one call, and two 'bd dep add -t relates-to'"
echo "calls in opposite directions (bd accepts the second). Prune:"
echo "scripts/prune-bd-relates-to.sh, then run --gate again to confirm."
echo "See NOTES.md (ranger-base-pkqn, ranger-base-nusr, ranger-base-uw8g)."
