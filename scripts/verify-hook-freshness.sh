#!/usr/bin/env bash
# Detective control for the L3 commit wall going stale in repos no session
# ever enters (ranger-base-8zki).
#
# WHY THIS EXISTS AS A SCRIPT AND NOT AS A REINSTALL
# The hook bodies are compiled into the posse binary, so every hook on the box
# is a COPY that was correct at the moment it was written. Only two things
# re-render one: `posse gates install-hooks`, typed by hand, and a session
# create — and a session's worktree shares the COMMON hooks dir of the repo it
# was cut from, so a session create refreshes that one repo and no other. Every
# other hooked repo (an instance's private beads repos never hold a session) is
# re-rendered by nothing at all. One such pair ran a hook three days behind the
# binary: it still refused, but its refusal prescribed the bare two-dot
# `git diff`, which is blind to another persona's staged edit — exactly the
# wrong advice ranger-base-erba landed b291784 to remove. A wall that refuses
# with stale guidance fails silently, which is why staleness is a finding here
# and not a warning. Reinstalling once fixes the day; this fixes the class,
# because the next change to the hook body re-stales those repos the same way.
#
# WHAT IT COMPARES. The reference is not a checked-in string: it is a fresh
# `posse gates install-hooks` render from the binary on PATH into a throwaway
# repo, so the control cannot drift from the renderer. Two assertions per slot,
# both exact:
#   identity  the repo's hook equals that render, with the one line the render
#             legitimately varies (the visibility stamp) normalized on BOTH
#             sides. A future body change that is NOT that line grows the diff
#             and fails here rather than being waved through.
#   stamp     that line's value equals what config beads_visibility: says for
#             the repo. Identity alone cannot see a private repo carrying a
#             public stamp: it is the half that would leak.
# Then behavior, because ADR 0023 counts a slot only when identity AND
# behavior hold: the installed file is exec'd twice, and BOTH arms are
# asserted — an unqualified commit must exit 1, and a path-limited one
# (GIT_INDEX_FILE = $GIT_DIR/next-index-<pid>, which is what git itself hands
# the slot) must exit 0. A hook that refuses everything is not a wall either.
#
# WHAT IT WILL NOT DO. It installs nothing and repairs nothing: the repos are
# the operator's, and a hook rewrite in a shared checkout is a change someone
# should type. A finding prints the exact command that fixes it.
#
# Exit 0 clean · 1 findings · 2 nothing measured.
set -uo pipefail

quiet=0
case "${1:-}" in
  --quiet) quiet=1 ;;
  "") ;;
  *) echo "usage: $(basename "$0") [--quiet]" >&2; exit 2 ;;
esac

say() { [ "$quiet" -eq 1 ] || echo "$@"; }

POSSE="${POSSE:-$(command -v posse 2>/dev/null)}"
[ -n "$POSSE" ] || { echo "verify-hook-freshness: no posse on PATH — nothing measured"; exit 2; }

home="${RHQ_HOME:-$HOME/.config/posse}"
[ -d "$home" ] || home="${HOME}/.config/rhq"
config="$home/config.yaml"
[ -r "$config" ] || { echo "verify-hook-freshness: $config unreadable — nothing measured"; exit 2; }

# The repos posse claims to guard are exactly the keys of beads_visibility:,
# which is also where the stamp each hook must carry comes from. Reading one
# block for both keeps the two halves from disagreeing.
#
# This has to be posse's rule (yamlClean, internal/rhq/yamlflat.go), not a
# spelling invented here (ranger-base-heyb; the same split fqfw and k3yd
# already closed one level up, in cfg()): a comment starts at whitespace
# followed by '#', not '#' anywhere (a hash with no space before it is data,
# not a truncated line); a matched pair of double quotes is dropped, not
# kept; and the value is the REST of the line after the first ':', trimmed —
# not the awk field after it, which stops at the first space. Key and value
# split on the first ':' only, so a repo path or a value with a space in it
# does not get misread either.
#
# Read with a while-loop, not mapfile: the bash this box runs from
# /usr/bin/env is 3.2, where mapfile does not exist and `set -u` then turns
# its absence into an unbound-variable crash that exits 1 — a "findings"
# code for a script that measured nothing.
entries=()
while IFS= read -r line; do
  [ -n "$line" ] && entries+=("$line")
done < <(awk '
  /^beads_visibility:/ { in_block = 1; next }
  in_block && /^[^[:space:]#]/ { in_block = 0 }
  in_block {
    line = $0
    trimmed = line
    sub(/^[ \t]+/, "", trimmed)
    if (trimmed == "" || trimmed == line || substr(trimmed, 1, 1) == "#") next
    i = index(trimmed, ":")
    if (i <= 1) next
    key = substr(trimmed, 1, i - 1)
    val = substr(trimmed, i + 1)
    sub(/[[:blank:]]#.*$/, "", val)
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
    if (length(val) >= 2 && substr(val, 1, 1) == "\"" && substr(val, length(val), 1) == "\"") {
      val = substr(val, 2, length(val) - 2)
    }
    printf "%s\t%s\n", key, val
  }
' "$config")

[ "${#entries[@]}" -gt 0 ] ||
  { echo "verify-hook-freshness: no beads_visibility: entries in $config — nothing measured"; exit 2; }

tmp=$(mktemp -d) || { echo "verify-hook-freshness: mktemp failed — nothing measured"; exit 2; }
# The behavior arm needs a temporary index INSIDE the repo's git dir (see
# below), which is the one thing this script writes outside its own tmpdir.
# Named here so the trap can take it with everything else if we are killed
# mid-repo; git leaves its own next-index-<pid> behind on a crash too, and
# ignores them, so a stray costs nothing but tidiness.
stray_idx=""
trap 'rm -rf "$tmp"; [ -n "$stray_idx" ] && rm -f "$stray_idx"' EXIT

# The reference render. A throwaway repo is unmarked in config, and unmarked is
# public (fail closed) — assert that rather than assume it, so a change to the
# default cannot silently turn every private repo into a finding.
git init -q "$tmp/ref" 2>/dev/null || { echo "verify-hook-freshness: git init failed — nothing measured"; exit 2; }
if ! ref_out=$("$POSSE" gates install-hooks "$tmp/ref" 2>&1); then
  echo "verify-hook-freshness: reference render failed — nothing measured"
  echo "$ref_out" | sed 's/^/    /'
  exit 2
fi
case "$ref_out" in
  *"visibility guard: public"*) ;;
  *) echo "verify-hook-freshness: reference render is not public — nothing measured"
     echo "$ref_out" | sed 's/^/    /'; exit 2 ;;
esac
ref_commit="$tmp/ref/.git/hooks/prepare-commit-msg"
ref_prepush="$tmp/ref/.git/hooks/pre-push"
[ -r "$ref_commit" ] && [ -r "$ref_prepush" ] ||
  { echo "verify-hook-freshness: reference render produced no hooks — nothing measured"; exit 2; }

# The one line the render varies per repo. Normalized away for the identity
# compare and asserted separately by name.
norm() { sed "s/^posse_beads_visibility='.*'\$/posse_beads_visibility='NORMALIZED'/" "$1" | shasum -a 256 | cut -d' ' -f1; }
plain() { shasum -a 256 "$1" | cut -d' ' -f1; }

ref_commit_sha=$(norm "$ref_commit")
ref_prepush_sha=$(plain "$ref_prepush")

# The posse-owned member of a slot: the slot itself when it carries our marker,
# else posse-<slot> behind the chain dispatcher install-hooks writes when a
# foreign shim got there first. Anything else is not ours to judge.
member() { # <hooks> <slot> <marker>
  local hooks=$1 slot=$2 marker=$3
  if [ -f "$hooks/$slot" ] && grep -qF -- "$marker" "$hooks/$slot"; then
    echo "$hooks/$slot"; return 0
  fi
  if [ -f "$hooks/posse-$slot" ] && grep -qF -- "$marker" "$hooks/posse-$slot" &&
     [ -f "$hooks/$slot" ] && grep -q '^exec "\$d/' "$hooks/$slot"; then
    echo "$hooks/posse-$slot"; return 0
  fi
  return 1
}

measured=0
findings=0
finding() { findings=$((findings + 1)); echo "  FINDING  $*"; }

for e in "${entries[@]}"; do
  repo=${e%%$'\t'*}; want=${e#*$'\t'}
  repo=${repo/#\~/$HOME}
  short=${repo/#$HOME/\~}
  say ""
  say "  $short  (config says: $want)"
  if [ ! -d "$repo" ]; then
    say "    absent — no such directory, nothing to check"
    continue
  fi
  hooks=$(git -C "$repo" rev-parse --git-path hooks 2>/dev/null) || {
    say "    not a git repo — nothing to check"; continue; }
  case "$hooks" in /*) ;; *) hooks="$repo/$hooks" ;; esac
  [ -d "$hooks" ] || { finding "$short: no hooks dir at $hooks"; continue; }
  measured=$((measured + 1))

  # -- prepare-commit-msg: identity, stamp, both behavior arms --------------
  if ! m=$(member "$hooks" prepare-commit-msg "# posse-gate shared-index"); then
    finding "$short: prepare-commit-msg is not posse's — the commit wall is not installed here"
  else
    if [ "$(norm "$m")" = "$ref_commit_sha" ]; then
      say "    fresh    prepare-commit-msg  ${m##*/}"
    else
      finding "$short: prepare-commit-msg is STALE — it does not match this binary's render"
      say "             $m"
    fi
    got=$(sed -n "s/^posse_beads_visibility='\(.*\)'\$/\1/p" "$m" | head -1)
    if [ "$got" = "$want" ]; then
      say "    stamped  $got"
    else
      finding "$short: visibility stamp is '${got:-<none>}' but config says '$want'"
    fi

    # Behavior. Skipped where the hook itself exempts by design, so an exempt
    # tree reads as "not measured" rather than as a wall that failed.
    gitdir=$(git -C "$repo" rev-parse --git-dir 2>/dev/null)
    case "$gitdir" in /*) ;; *) gitdir="$repo/$gitdir" ;; esac
    common=$(git -C "$repo" rev-parse --git-common-dir 2>/dev/null)
    case "$common" in /*) ;; *) common="$repo/$common" ;; esac
    exempt=""
    [ "$(cd -P "$gitdir" && pwd -P)" = "$(cd -P "$common" && pwd -P)" ] ||
      exempt="linked worktree (no shared index)"
    for f in MERGE_HEAD CHERRY_PICK_HEAD rebase-merge rebase-apply; do
      [ -e "$gitdir/$f" ] && exempt="$f present"
    done
    # THE PATH-LIMITED ARM NEEDS AN INDEX, NOT AN INDEX NAME (ranger-base-ixv4).
    # git's own next-index-<pid> is a COPY of the index with the named paths
    # refreshed into it. A name pointing at a file that does not exist is an
    # EMPTY index, and `git diff --cached --name-only` against an empty index
    # reports every tracked file — so since ranger-base-ak3e added the
    # constitution arm, which reads exactly that, the fabricated safe form is
    # refused for touching the whole class. MEASURED 2026-08-29: this reported
    # "a path-limited commit is refused too — the safe form has no way through"
    # in ~/src/ranger-base and ~/src/posse, the two repos that carry class
    # paths (rhq/agents/** and .claude/settings.json), and nowhere else. A
    # control that cries wolf in the constitution repo is the one place it
    # must not.
    # Seeded from HEAD rather than from the live index on purpose: the arm is
    # a question about the WALL, and seeding from the index would make the
    # answer depend on whatever another persona happens to have staged.
    # In a repo with no commits git's own next-index IS empty, so leaving the
    # file absent there is the accurate emulation, not a fallback.
    tmpidx="$gitdir/next-index-$$"
    if git -C "$repo" rev-parse --verify -q HEAD >/dev/null 2>&1; then
      if GIT_INDEX_FILE="$tmpidx" git -C "$repo" read-tree HEAD 2>/dev/null; then
        stray_idx="$tmpidx"
      else
        exempt="cannot seed a temporary index from HEAD"
      fi
    fi
    if [ -n "$exempt" ]; then
      say "    behavior not measured — $exempt"
    else
      ( cd "$repo" && RHQ_PERSONA=verify-hook-freshness /bin/sh "$m" /dev/null message ) >/dev/null 2>&1
      refused=$?
      ( cd "$repo" && GIT_INDEX_FILE="$tmpidx" \
          RHQ_PERSONA=verify-hook-freshness /bin/sh "$m" /dev/null message ) >/dev/null 2>&1
      allowed=$?
      [ -n "$stray_idx" ] && rm -f "$stray_idx"
      stray_idx=""
      if [ "$refused" -ne 1 ]; then
        finding "$short: an unqualified commit is NOT refused (hook exited $refused)"
      elif [ "$allowed" -ne 0 ]; then
        finding "$short: a path-limited commit is refused too (hook exited $allowed) — the safe form has no way through"
      else
        say "    behaves  unqualified refused (1) · path-limited allowed (0)"
      fi
    fi
  fi

  # -- pre-push: identity only. It carries no stamp, and its refusal is keyed
  # on RHQ_TOOLS_DENY, which this script has no business forging.
  if ! p=$(member "$hooks" pre-push "# posse-gate"); then
    finding "$short: pre-push is not posse's — the push wall is not installed here"
  elif [ "$(plain "$p")" = "$ref_prepush_sha" ]; then
    say "    fresh    pre-push  ${p##*/}"
  else
    finding "$short: pre-push is STALE — it does not match this binary's render"
    say "             $p"
  fi
done

say ""
if [ "$measured" -eq 0 ]; then
  echo "verify-hook-freshness: 0 of ${#entries[@]} configured repos were present and git — nothing measured, not a pass"
  exit 2
fi
if [ "$findings" -ne 0 ]; then
  cat <<'EOF'
verify-hook-freshness: findings above

Each stale or missing slot is fixed by re-rendering it from this binary:

  posse gates install-hooks <repo>      # add --chain if a foreign shim holds a slot

That is a hook rewrite in someone else's checkout, so type it rather than
scripting it, and re-run this to confirm. A FOREIGN slot is not fixed by that
command — it refuses rather than overwrite, and prints the chain to paste
(INSTALL.md §9).
EOF
  exit 1
fi
echo "verify-hook-freshness: fresh — $measured repo(s) match this binary's render, stamps agree with config"
