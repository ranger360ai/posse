#!/usr/bin/env bash
# queue-scrub.sh -- take named text out of a beads store of record AND out of
# the unpushed commits of the repo that exports it, so that a push carries
# neither. Written for ranger-base-mm15y; parameterised, so it names no
# instance and holds no scrubbed text of its own.
#
# THE SHAPE OF THE PROBLEM. A beads store is a SQLite db plus a generated
# export (.beads/issues.jsonl) that is committed. Scrubbing the db alone
# leaves every already-committed copy of the export carrying the text; and
# rewriting history alone leaves the db to re-export it at the next sync. So
# the two halves are one operation, in this order: db, re-export, history.
#
# WHAT IT NEEDS
#   --repo DIR       the repo that tracks the export        (required)
#   --db PATH        the SQLite store      (default: REPO/.beads/beads.db)
#   --file PATH      the export, repo-relative  (default: .beads/issues.jsonl)
#   --targets FILE   one `comment <rowid>` or `description <bead-id>` per line,
#                    with an OPTIONAL third field: a file holding the text that
#                    particular target is replaced by. An open bead's
#                    description wants a rewrite, not the blanket stub.
#   --stub FILE      the text a target with no stub of its own is replaced BY
#   --range A..B     the commits to rewrite  (default: origin/main..main)
#   --detect FILE    fixed strings that must read zero afterwards (grep -F -f)
#   --apply          do it. Without this the script only measures and proves.
#
# WHAT IT NEVER DOES: push, add a remote, touch any repo but --repo, or run
# while a bd daemon is up. It rewrites history with `git commit-tree`, which
# is the one plumbing route this shop's git shim sanctions, and it preserves
# every commit's author, committer, both dates and its message verbatim.
set -euo pipefail

REPO= DB= FILE=.beads/issues.jsonl TARGETS= STUB= RANGE=origin/main..main DETECT= APPLY=0
while [ $# -gt 0 ]; do
  case $1 in
    --repo) REPO=$2; shift 2;;
    --db) DB=$2; shift 2;;
    --file) FILE=$2; shift 2;;
    --targets) TARGETS=$2; shift 2;;
    --stub) STUB=$2; shift 2;;
    --range) RANGE=$2; shift 2;;
    --detect) DETECT=$2; shift 2;;
    --apply) APPLY=1; shift;;
    *) echo "queue-scrub: unknown argument: $1" >&2; exit 2;;
  esac
done
[ -n "$REPO" ] || { echo "queue-scrub: --repo is required" >&2; exit 2; }
[ -n "$TARGETS" ] || { echo "queue-scrub: --targets is required" >&2; exit 2; }
[ -n "$STUB" ] || { echo "queue-scrub: --stub is required" >&2; exit 2; }
[ -n "$DB" ] || DB="$REPO/.beads/beads.db"
HERE=$(cd "$(dirname "$0")" && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/posse-queue-scrub.XXXXXX")
trap 'rm -rf -- "${WORK:?}"' EXIT

say() { printf '%s\n' "$*"; }
rule() { printf -- '---- %s\n' "$*"; }

# ---------------------------------------------------------------- preflight
rule "preflight"
for p in "$REPO/.git" "$DB" "$TARGETS" "$STUB"; do
  [ -e "$p" ] || { echo "queue-scrub: missing: $p" >&2; exit 1; }
done
if [ -e "$REPO/.beads/daemon.pid" ]; then
  echo "queue-scrub: a bd daemon pid file is present -- refusing" >&2; exit 1
fi
dirty=$(git -C "$REPO" status --porcelain | grep -v " $FILE\$" || true)
if [ -n "$dirty" ]; then
  echo "queue-scrub: $REPO is dirty beyond $FILE -- refusing:" >&2
  printf '%s\n' "$dirty" >&2; exit 1
fi
base=${RANGE%%..*}; tip=${RANGE##*..}
git -C "$REPO" rev-parse --verify -q "$base" >/dev/null || { echo "queue-scrub: no such ref: $base" >&2; exit 1; }
git -C "$REPO" rev-parse --verify -q "$tip"  >/dev/null || { echo "queue-scrub: no such ref: $tip" >&2; exit 1; }
say "repo    $REPO"
say "db      $DB"
say "range   $RANGE  ($(git -C "$REPO" rev-list --count "$RANGE") commit(s))"

# ------------------------------------------------------- read the old values
rule "targets"
: > "$WORK/pairs"
tojson() { python3 -I -c 'import json,sys;print(json.dumps(open(sys.argv[1],encoding="utf-8").read()))' "$1"; }
n=0
while read -r kind id stub; do
  case $kind in
    ''|'#'*) continue;;
    comment)     hex=$(sqlite3 -readonly "$DB" "select hex(text) from comments where id=$id;");;
    description) hex=$(sqlite3 -readonly "$DB" "select hex(description) from issues where id='$id';");;
    *) echo "queue-scrub: unknown target kind: $kind" >&2; exit 1;;
  esac
  [ -n "$hex" ] || { echo "queue-scrub: $kind $id: no such row, or empty" >&2; exit 1; }
  [ -n "$stub" ] || stub=$STUB
  [ -f "$stub" ] || { echo "queue-scrub: $kind $id: no such stub file: $stub" >&2; exit 1; }
  stub_json=$(tojson "$stub")
  # sqlite3's text output would strip a trailing newline and re-encode; the
  # export holds the value as STORED, so the bytes come back through hex.
  printf '%s' "$hex" | python3 -I -c '
import binascii,json,sys
raw=binascii.unhexlify(sys.stdin.read().strip())
print(json.dumps(raw.decode("utf-8")))' >> "$WORK/pairs"
  printf '%s\n' "$stub_json" >> "$WORK/pairs"
  say "target  $kind $id  ($((${#hex} / 2)) bytes) <- $(basename "$stub")"
  n=$((n+1))
done < "$TARGETS"
[ "$n" -gt 0 ] || { echo "queue-scrub: no targets" >&2; exit 1; }

hits_in() { # hits_in <rev>  -- how many target substitutions that blob would take
  git -C "$REPO" show "$1:$FILE" 2>/dev/null \
    | python3 -I "$HERE/queue-scrub-sub.py" "$WORK/pairs" >/dev/null 2>"$WORK/h" || true
  awk -F': ' '{s+=$2} END{print s+0}' "$WORK/h"
}

# --------------------------------------------------------------- the census
rule "census: commits in $RANGE whose $FILE carries a target"
carry=0
for c in $(git -C "$REPO" rev-list --reverse "$RANGE"); do
  k=$(hits_in "$c")
  if [ "$k" -gt 0 ]; then
    carry=$((carry+1))
    printf '  CARRIES %s  %-46s (%s hit(s))\n' \
      "$(git -C "$REPO" rev-parse --short=9 "$c")" \
      "$(git -C "$REPO" log -1 --format=%s "$c" | cut -c1-46)" "$k"
  fi
done
say "  $carry of $(git -C "$REPO" rev-list --count "$RANGE") commit(s) carry a target"

# ------------------------------------------- prove the blob transformation
# This half needs no commit objects, so it runs at a persona seat. It proves
# the substitution is exact (the detector reads zero on the RESULT) and
# surgical (nothing but the target records changed), one commit at a time.
rule "blob proof over $RANGE"
bfail=0
for c in $(git -C "$REPO" rev-list --reverse "$RANGE"); do
  git -C "$REPO" show "$c:$FILE" > "$WORK/before" 2>/dev/null || continue
  python3 -I "$HERE/queue-scrub-sub.py" "$WORK/pairs" < "$WORK/before" > "$WORK/after" 2>/dev/null
  if [ -n "$DETECT" ]; then
    kb=$(grep -c -F -f "$DETECT" "$WORK/before" || true)
    ka=$(grep -c -F -f "$DETECT" "$WORK/after" || true)
  else kb=0; ka=0; fi
  kt=$(python3 -I "$HERE/queue-scrub-sub.py" "$WORK/pairs" < "$WORK/after" >/dev/null 2>"$WORK/h"; awk -F': ' '{s+=$2} END{print s+0}' "$WORK/h")
  changed=$(python3 -I -c '
import json,sys
def recs(p):
    d={}
    for ln in open(p,encoding="utf-8"):
        ln=ln.strip()
        if ln:
            o=json.loads(ln); d[o.get("id")]=o
    return d
a,b=recs(sys.argv[1]),recs(sys.argv[2])
ch=[k for k in a if a[k]!=b.get(k)]
print(",".join(sorted(ch)) if ch else "-")
print(len(a),len(b))' "$WORK/before" "$WORK/after" | tr '\n' ' ')
  st=ok
  [ "$ka" -eq 0 ] || { st=DETECTOR-STILL-HITS; bfail=1; }
  [ "$kt" -eq 0 ] || { st=TARGET-STILL-PRESENT; bfail=1; }
  if [ "$kb" -gt 0 ] || [ "$(cut -d' ' -f1 <<<"$changed")" != "-" ]; then
    printf '  %s  detector %s->%s  records changed: %s  [%s]\n' \
      "$(git -C "$REPO" rev-parse --short=9 "$c")" "$kb" "$ka" "$changed" "$st"
  fi
done
if [ "$bfail" -eq 0 ]; then say "  BLOB PROOF OK"; else say "  BLOB PROOF FAILED"; exit 1; fi

[ "$APPLY" -eq 0 ] && rule "DRY RUN: building the rewritten chain; nothing is moved"

# ------------------------------------------------------- rewrite the commits
# `git commit-tree` is REFUSED at a persona seat by posse's git shim
# (hookskip:plumbing-commit, ranger-base-r3pri): a rewrite writes commit
# objects no pre-commit hook ever sees, which is the whole point of a history
# rewrite and exactly why this half is the operator's. A seat that cannot get
# past this line has not failed; it has hit the gate working.
rule "rewrite"
export GIT_INDEX_FILE="$WORK/index"
if ! git -C "$REPO" commit-tree -p "$(git -C "$REPO" rev-parse "$base")" \
     "$(git -C "$REPO" rev-parse "$base^{tree}")" -m probe >/dev/null 2>&1; then
  unset GIT_INDEX_FILE
  say "  git commit-tree is REFUSED here (hookskip:plumbing-commit, ADR 0052)."
  say "  That is the gate working, not a failure: a history rewrite writes"
  say "  commit objects no pre-commit hook can see, so this half is the"
  say "  operator's to run. Everything above it is proved."
  [ "$APPLY" -eq 0 ] && exit 0
  exit 1
fi
prev=$(git -C "$REPO" rev-parse "$base")
for c in $(git -C "$REPO" rev-list --reverse "$RANGE"); do
  git -C "$REPO" read-tree "$c"
  blob=$(git -C "$REPO" rev-parse "$c:$FILE")
  newblob=$(git -C "$REPO" cat-file blob "$blob" \
    | python3 -I "$HERE/queue-scrub-sub.py" "$WORK/pairs" 2>/dev/null \
    | git -C "$REPO" hash-object -w --stdin)
  git -C "$REPO" update-index --cacheinfo "100644,$newblob,$FILE"
  newtree=$(git -C "$REPO" write-tree)
  GIT_AUTHOR_NAME=$(git -C "$REPO" log -1 --format=%an "$c")
  GIT_AUTHOR_EMAIL=$(git -C "$REPO" log -1 --format=%ae "$c")
  GIT_AUTHOR_DATE=$(git -C "$REPO" log -1 --format=%aI "$c")
  GIT_COMMITTER_NAME=$(git -C "$REPO" log -1 --format=%cn "$c")
  GIT_COMMITTER_EMAIL=$(git -C "$REPO" log -1 --format=%ce "$c")
  GIT_COMMITTER_DATE=$(git -C "$REPO" log -1 --format=%cI "$c")
  export GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_AUTHOR_DATE
  export GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL GIT_COMMITTER_DATE
  newc=$(git -C "$REPO" log -1 --format=%B "$c" | git -C "$REPO" commit-tree "$newtree" -p "$prev")
  printf '  %s -> %s  %s\n' \
    "$(git -C "$REPO" rev-parse --short=9 "$c")" "$(git -C "$REPO" rev-parse --short=9 "$newc")" \
    "$(git -C "$REPO" log -1 --format=%s "$c" | cut -c1-44)"
  prev=$newc
done
unset GIT_INDEX_FILE
say "  new tip $(git -C "$REPO" rev-parse --short=9 "$prev")"

# ------------------------------------------------------------------- verify
rule "verify (against the rewritten chain, before anything is moved)"
fail=0
for c in $(git -C "$REPO" rev-list --reverse "$base..$prev"); do
  if [ -n "$DETECT" ]; then
    k=$(git -C "$REPO" show "$c:$FILE" | grep -c -F -f "$DETECT" || true)
    [ "$k" -eq 0 ] || { printf '  DETECTOR STILL HITS in %s: %s\n' "$(git -C "$REPO" rev-parse --short=9 "$c")" "$k"; fail=1; }
  fi
  k=$(hits_in "$c")
  [ "$k" -eq 0 ] || { printf '  TARGET STILL PRESENT in %s: %s\n' "$(git -C "$REPO" rev-parse --short=9 "$c")" "$k"; fail=1; }
done
# the rewrite must have changed NOTHING but the one file
difffiles=$(git -C "$REPO" diff --name-only "$tip" "$prev" || true)
if [ -n "$difffiles" ]; then
  printf '  tree differs from the original tip in: %s\n' "$(printf '%s' "$difffiles" | tr '\n' ' ')"
  [ "$difffiles" = "$FILE" ] || fail=1
fi
say "  commits on the new chain: $(git -C "$REPO" rev-list --count "$base..$prev") (was $(git -C "$REPO" rev-list --count "$RANGE"))"
if [ "$fail" -eq 0 ]; then say "  VERIFY OK"; else say "  VERIFY FAILED"; exit 1; fi

if [ "$APPLY" -eq 0 ]; then
  rule "DRY RUN ends here. Nothing moved; the new objects are unreferenced."
  say "to apply: re-run with --apply (scrubs the db, re-exports, moves $tip)"
  exit 0
fi

# -------------------------------------------------------------------- apply
rule "apply: db"
while read -r kind id stub; do
  [ -n "${stub:-}" ] || stub=$STUB
  case $kind in
    ''|'#'*) continue;;
    comment)
      sqlite3 "$DB" "update comments set text=readfile('$stub') where id=$id;"
      b=$(sqlite3 -readonly "$DB" "select issue_id from comments where id=$id;");;
    description)
      sqlite3 "$DB" "update issues set description=readfile('$stub') where id='$id';"
      b=$id;;
    *) continue;;
  esac
  # make the owning bead re-export: bd skips a bead whose content hash is unchanged
  sqlite3 "$DB" "insert or ignore into dirty_issues(issue_id) values('$b');"
  sqlite3 "$DB" "delete from export_hashes where issue_id='$b';"
  rm -f -- "$REPO/.beads/export-state/"*.json.tmp 2>/dev/null || true
done < "$TARGETS"

rule "apply: re-export"
( cd "$REPO" && bd sync )

rule "apply: move the branch"
br=$(git -C "$REPO" rev-parse --abbrev-ref HEAD)
git -C "$REPO" update-ref -m "queue-scrub" "refs/heads/$br" "$prev"
git -C "$REPO" reset --mixed HEAD >/dev/null

rule "apply: final verify"
if [ -n "$DETECT" ]; then
  k=$(grep -c -F -f "$DETECT" "$REPO/$FILE" || true)
  say "  fresh export: $k detector hit(s)"
  [ "$k" -eq 0 ] || exit 1
  for c in $(git -C "$REPO" rev-list "$base..$br"); do
    k=$(git -C "$REPO" show "$c:$FILE" | grep -c -F -f "$DETECT" || true)
    [ "$k" -eq 0 ] || { say "  history still hits at $c"; exit 1; }
  done
  say "  $(git -C "$REPO" rev-list --count "$base..$br") commit(s) and the fresh export: zero"
fi
say "  DONE. The push is still the operator's."
