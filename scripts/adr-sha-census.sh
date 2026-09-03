#!/bin/sh
# adr-sha-census.sh — the ADR 0051 census (D3 verify), per record.
#
# A 7-40 hex token that resolves to a commit here and is NOT an ancestor of
# the base branch is refused unless an ancestor somewhere in the SAME FILE has
# the same patch-id (its landed twin). That is D2's bracket shape made
# checkable — a record about stale shas carries the twin beside each one (ADR
# 0051 D5); the radius is the record because prose wraps a bracket onto the
# next line, and a twin cannot exist before the launcher lands it, so the
# radius carries no safety and the twin carries all of it.
# Tokens that do not resolve are not judged (prose, or another repo's).
# Base: the branch the main checkout has checked out; detached judges nothing.
#
# usage: sh scripts/adr-sha-census.sh [files...]   (default docs/adr/*.md)
# exit 1 on any refusal. Last line: judged/admitted/refused, so a 0 over a
# pruned object store reads as "0 judged", not as clean.
set -u
base=$(git --git-dir="$(git rev-parse --git-common-dir)" symbolic-ref -q HEAD) || {
  echo "adr-sha-census: main checkout is detached; judged nothing" >&2; exit 0; }
[ $# -gt 0 ] || set -- docs/adr/*.md
# AN EMPTY PATCH-ID IS NO ANSWER (ranger-base-glewr, measured on git 2.50.1).
# git diff-tree -p prints nothing for a commit with no diff of its own — a
# root commit, a merge, an --allow-empty commit — so patch-id prints nothing,
# and two empties compare EQUAL. Without this the census admits an empty stale
# commit beside any empty ancestor, and a repo's own ROOT commit is an empty
# ancestor any record may legitimately cite. The caller tests for emptiness.
pid() { git diff-tree -p "$1" | git patch-id --stable | cut -d' ' -f1; }
out=$(for f in "$@"; do
  anc=""; non=""
  for t in $(grep -oE '\b[0-9a-f]{7,40}\b' "$f" | sort -u); do
    git cat-file -e "$t^{commit}" 2>/dev/null || continue
    if git merge-base --is-ancestor "$t" "$base"; then anc="$anc $t"; else non="$non $t"; fi
  done
  for a in $anc; do echo "JUDGED $f $a ancestor"; done
  for n in $non; do
    pn=$(pid "$n"); ok=""
    [ -n "$pn" ] && for a in $anc; do [ "$(pid "$a")" = "$pn" ] && ok=$a && break; done
    if [ -n "$ok" ]; then echo "ADMITTED $f $n twin $ok"
    else echo "REFUSE $f:$(grep -nE "\b$n\b" "$f" | cut -d: -f1 | tr '\n' ',' | sed 's/,$//') $n resolves here but is not on ${base#refs/heads/} and no landed twin is in the record — cite the bead id (git log --grep), or put the twin beside it (ADR 0051 D2/D5)"; fi
  done
done)
printf '%s\n' "$out" | grep -E '^(ADMITTED|REFUSE)'
j=$(printf '%s\n' "$out" | grep -c '^JUDGED'); a=$(printf '%s\n' "$out" | grep -c '^ADMITTED'); r=$(printf '%s\n' "$out" | grep -c '^REFUSE')
echo "adr-sha-census: base ${base#refs/heads/} judged $((j+a+r)) distinct tokens: $j ancestors, $a admitted by twin, $r refused"
[ "$r" -eq 0 ]
