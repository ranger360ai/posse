#!/usr/bin/env bash
# Audit the work graph for the bd 0.49.1 `dep add` landmine (ranger-base-pkqn).
#
# Usage: scripts/verify-bd-dep-safety.sh                 # print the audit, exit 0
#        scripts/verify-bd-dep-safety.sh <target-id>     # exit 1 if unsafe
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
# Read-only: the db is opened `mode=ro`, and the recursive queries here use
# UNION (memoised), so they are cheap where bd's own is not.
set -euo pipefail

TARGET=${1:-}

find_db() {
	if [ -n "${BEADS_DB:-}" ]; then
		printf '%s\n' "$BEADS_DB"
		return
	fi
	local dir=.beads
	# One redirect hop, the same as bd 0.49.1 allows (rangerhq: chains are not).
	if [ -f "$dir/redirect" ]; then
		dir=$(tr -d '[:space:]' <"$dir/redirect")
	fi
	printf '%s\n' "$dir/beads.db"
}

DB=$(find_db)
[ -f "$DB" ] || { echo "verify-bd-dep-safety: no beads db at $DB" >&2; exit 2; }
command -v sqlite3 >/dev/null || { echo "verify-bd-dep-safety: sqlite3 not on PATH" >&2; exit 2; }

q() { sqlite3 "file:${DB}?mode=ro" "$1"; }

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
echo "This list only grows: 'bd relate' plants a new pair and bd rewrites the"
echo "reverse edge, so removing one does not hold. See NOTES.md (ranger-base-pkqn)."
