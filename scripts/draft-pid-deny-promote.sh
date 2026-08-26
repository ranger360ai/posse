#!/bin/sh
# draft-pid-deny-promote.sh — DRAFT the ADR 0015 §3 fence into a
# constitution's crew PIDs: add `Bash(posse promote:*)` to every PID's
# `deny:` list. Written for ranger-base-o943 item 5.
#
# IT IS A DRAFT AND IT STAYS A DRAFT. It edits files in the constitution
# repo's working tree and stops there: it never stages, never commits, never
# pushes, and never runs `posse promote`. The operator reads `git diff`,
# commits, and promotes — which makes this the first change ratified through
# the step ADR 0015 adds, which is the point of doing it this way.
#
# WHY A SCRIPT AND NOT A PATCH FILE: posse is a public repo and the
# constitution it fences is a private one. A patch would carry that repo's
# PID prose across the boundary in its context lines. This carries the rule
# instead, and reads the prose only on the operator's own machine.
#
# usage: scripts/draft-pid-deny-promote.sh [<constitution agents dir>]
#        default: the `constitution:` dir posse resolves, then ./agents
set -eu

RULE='Bash(posse promote:*)'
dir="${1:-}"
if [ -z "$dir" ]; then
    if [ -d ./agents ]; then dir=./agents; else
        echo "usage: $0 <constitution agents dir>" >&2
        echo "  the directory holding the crew PIDs, e.g. ~/src/<instance>/rhq/agents" >&2
        exit 2
    fi
fi
[ -d "$dir" ] || { echo "not a directory: $dir" >&2; exit 2; }

changed=0
skipped=0
for pid in "$dir"/*.md; do
    [ -e "$pid" ] || continue
    if grep -qF -- "$RULE" "$pid"; then
        skipped=$((skipped + 1))
        continue
    fi
    # The deny: list, block style (`deny:` then `  - <rule>` lines), is the
    # one shape posse's own PID reader and every shipped PID use. A PID
    # written in flow style (`deny: [a, b]`) or carrying no deny: at all is
    # REPORTED, never rewritten by regex — a mangled PID is prose in force.
    if ! grep -q '^deny:[[:space:]]*$' "$pid"; then
        echo "  SKIP $(basename "$pid"): no block-style 'deny:' — add '  - $RULE' by hand" >&2
        skipped=$((skipped + 1))
        continue
    fi
    tmp="$pid.draft.$$"
    awk -v rule="  - $RULE" '
        BEGIN { indeny = 0; done = 0 }
        {
            if (indeny && substr($0, 1, 4) != "  - ") { print rule; indeny = 0; done = 1 }
            print
            if (!done && $0 ~ /^deny:[[:space:]]*$/) { indeny = 1 }
        }
        END { if (indeny) print rule }
    ' "$pid" > "$tmp"
    mv "$tmp" "$pid"
    echo "  drafted $(basename "$pid")"
    changed=$((changed + 1))
done

echo
echo "drafted into $changed PID(s), $skipped left alone — nothing staged, nothing committed."
echo "next, as the operator:"
echo "  git -C \"\$(git -C '$dir' rev-parse --show-toplevel)\" diff -- '$dir'"
echo "  git -C \"\$(git -C '$dir' rev-parse --show-toplevel)\" commit -m 'pid: deny Bash(posse promote:*) — ADR 0015 §3' -- '$dir'"
echo "  posse promote"
