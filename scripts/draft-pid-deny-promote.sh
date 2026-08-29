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
    # Two deny: shapes are in the wild and BOTH are drafted into:
    #
    #   block  `deny:` on its own line, then `  - <rule>` lines.
    #          Every PID shipped in examples/agents/ is this shape.
    #   inline `deny: [a, b]`, the whole list on one line. Every LIVE crew
    #          PID is this shape, which is why the block-only version of
    #          this script drafted 0 of 11 at the retirement window and the
    #          edit went in by hand (ranger-base-j2io, MEASURED 2026-08-28).
    #
    # Anything else — a flow list broken across lines, a `deny:` with a
    # value that is neither, no `deny:` at all — is REPORTED, never
    # rewritten by regex. A mangled PID is prose in force.
    if grep -q '^deny:[[:space:]]*\[.*\][[:space:]]*$' "$pid"; then
        shape=inline
    elif grep -q '^deny:[[:space:]]*$' "$pid"; then
        shape=block
    else
        echo "  SKIP $(basename "$pid"): no block-style or single-line inline 'deny:' — add '$RULE' by hand" >&2
        skipped=$((skipped + 1))
        continue
    fi

    tmp="$pid.draft.$$"
    if [ "$shape" = inline ]; then
        # Append inside the brackets, unquoted, which is the spelling the
        # live PIDs already carry and posse's PID reader already parses.
        awk -v rule="$RULE" '
            BEGIN { done = 0 }
            !done && $0 ~ /^deny:[[:space:]]*\[.*\][[:space:]]*$/ {
                open = index($0, "[")
                shut = 0
                for (i = length($0); i > open; i--) {
                    if (substr($0, i, 1) == "]") { shut = i; break }
                }
                if (shut > 0) {
                    inner = substr($0, open + 1, shut - open - 1)
                    sub(/[ \t]+$/, "", inner)
                    if (inner ~ /^[ \t]*$/) sep = ""; else sep = ", "
                    print substr($0, 1, open) inner sep rule substr($0, shut)
                    done = 1
                    next
                }
            }
            { print }
        ' "$pid" > "$tmp"
    else
        awk -v rule="  - $RULE" '
            BEGIN { indeny = 0; done = 0 }
            {
                if (indeny && substr($0, 1, 4) != "  - ") { print rule; indeny = 0; done = 1 }
                print
                if (!done && $0 ~ /^deny:[[:space:]]*$/) { indeny = 1 }
            }
            END { if (indeny) print rule }
        ' "$pid" > "$tmp"
    fi

    # A rewrite that did not land the rule is a rewrite that did nothing we
    # can vouch for: drop the draft rather than move a file we cannot name.
    if ! grep -qF -- "$RULE" "$tmp"; then
        rm -f "$tmp"
        echo "  SKIP $(basename "$pid"): $shape rewrite did not land the rule — add '$RULE' by hand" >&2
        skipped=$((skipped + 1))
        continue
    fi
    mv "$tmp" "$pid"
    echo "  drafted $(basename "$pid") ($shape)"
    changed=$((changed + 1))
done

echo
echo "drafted into $changed PID(s), $skipped left alone — nothing staged, nothing committed."
echo "next, as the operator:"
echo "  git -C \"\$(git -C '$dir' rev-parse --show-toplevel)\" diff -- '$dir'"
echo "  git -C \"\$(git -C '$dir' rev-parse --show-toplevel)\" commit -m 'pid: deny Bash(posse promote:*) — ADR 0015 §3' -- '$dir'"
echo "  posse promote"
