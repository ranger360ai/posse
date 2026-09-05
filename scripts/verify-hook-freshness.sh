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
# `posse gates install-hooks` render from the binary on PATH, so the control
# cannot drift from the renderer. It is taken PER REPO, against the repo being
# measured, with the hooks path redirected to a scratch directory this script
# owns — so the render lands there and nothing of the repo's is touched.
#
# PER REPO, and not once for the box, because the render legitimately varies
# with the repo it is for and the visibility stamp is not the only line that
# does (ranger-base-x5olh). ADR 0024 D2 check 3's identity literals are derived
# from the repo: a `.beads/redirect` adds `posse_check 'instance-path'` and
# `'instance-path-abs'` — MEASURED 2026-09-05, six lines, the pair at each of
# the three call sites — and `git config --get-all user.email` adds one literal
# per scope, so a repo-local contribution address (the ranger-base-yqstz
# work-instance setup, measured the same day) adds a line the box's other repos
# do not carry. Against one shared reference every repo on the far side of any
# of those branches reads STALE forever: on 2026-09-01 that was three of four,
# immediately after a hand-typed install-hooks into all four, and a control that
# cries wolf in the constitution repo is the one place it must not. A reference
# rendered FOR the repo has every one of those variations already in it, without
# this script having to know what they are — the renderer stays the only thing
# that knows, which is the same reason the reference is a render and not a
# string.
#
# Two assertions per slot, both exact:
#   identity  the repo's hook equals that render, with the one line the render
#             varies WITHOUT the repo being able to tell it to (the visibility
#             stamp, which comes from config) normalized on BOTH sides. A
#             future body change that is NOT that line grows the diff and
#             fails here rather than being waved through.
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
# should type. A finding prints the exact command that fixes it. The per-repo
# reference render is an `install-hooks` AT the operator's repo and that is
# exactly what it must not be allowed to become — it is taken under the
# core.hooksPath redirect below, so every byte it writes lands in this script's
# tmpdir. MEASURED 2026-09-05 against a repo carrying a `.beads/redirect` and
# its own `.git/hooks`: no slot written, no config line added, no tree change.
# Pinned by behaviour rather than by grepping this file for the verb
# (TestQAHookFreshnessWritesNothingIntoTheRepoItMeasures) — the verb is here,
# and what makes it safe is where the render lands, which only a run can show.
#
# A MANAGED BOX (ADR 0052, ranger-base-1se2l). An employer's box points every
# git on it at one absolute, root-owned hooks directory by a global
# `core.hooksPath`. Two things follow, and this script got both wrong:
#
#   - The reference render is an `install-hooks` into a throwaway repo, and
#     that repo inherits the global too — so the render is classified managed,
#     writes nothing, and prints no `visibility guard: public` line. The script
#     then exited 2 for the WHOLE box. It is taken with a redirect env in force
#     now (git's own GIT_CONFIG_COUNT/KEY/VALUE form, appended to any count the
#     operator's environment already carries), aiming core.hooksPath at a
#     scratch directory this script owns. That is the same mechanism ADR 0052
#     D2 realizes the wall with, and M1 is the measurement it rests on: the env
#     form outranks the global value. The render is byte-identical to one
#     written into a repo's own `.git/hooks` — MEASURED 2026-09-05, both slots.
#     It is taken lazily, because a fully managed box needs no reference at all.
#
#   - A configured repo on such a box dispatches from the managed directory,
#     so whatever sits in its `.git/hooks` is a file git never runs. Measuring
#     it is worse than useless in both directions: a leftover posse hook there
#     reads as FRESH — a green about a wall that is not armed — and the
#     employer's slots read as foreign, which is a finding prescribing
#     `posse gates install-hooks`, the one write ADR 0052 says not to attempt.
#     So the repo is classified first and skipped, the way SweepHookWall does
#     it, and by the same code: `posse gates managed-hooks` is the binary's own
#     verdict, not a second implementation of its three legs in shell. Nothing
#     of posse's is installed on a managed repo to go stale — the session hooks
#     dir is rendered fresh at every launch — so a box that is entirely managed
#     is CLEAN, not unmeasured.
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
# This has to be posse's rule (yamlClean, internal/posse/yamlflat.go), not a
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

# The one line normalized away for the identity compare, and asserted
# separately by name. It is the stamp and ONLY the stamp: every other per-repo
# variation is in the reference already, because the reference is rendered for
# the repo. Normalizing a line here is giving up on it — whatever this sed
# covers, no compare can see again — so the list stays at the one line whose
# separate assertion (against config, not against the render) is stronger than
# what identity could say about it.
norm() { sed "s/^posse_beads_visibility='.*'\$/posse_beads_visibility='NORMALIZED'/" "$1" | shasum -a 256 | cut -d' ' -f1; }
plain() { shasum -a 256 "$1" | cut -d' ' -f1; }

# refhooks is set per repo by need_ref, one fresh empty directory each: a
# render that writes nothing then leaves nothing to compare against, instead of
# the previous repo's reference sitting there reading as this one's.
refhooks=""

# ref_env runs one command with core.hooksPath aimed at whichever $refhooks
# need_ref has just made, in git's config-in-env form — the same form ADR 0052
# D2's session env carries, and the one M1 measured as outranking a global
# value. Appended after whatever count the operator's environment already
# holds, never clobbering it: a GIT_CONFIG_COUNT we overwrote would drop their
# entries silently. A count that is not a number is not one we can append to,
# and git will not read it either, so we start our own and say nothing about
# theirs.
ref_env() {
  local n=${GIT_CONFIG_COUNT:-0}
  case "$n" in ''|*[!0-9]*) n=0 ;; esac
  env "GIT_CONFIG_COUNT=$((n + 1))" \
      "GIT_CONFIG_KEY_$n=core.hooksPath" \
      "GIT_CONFIG_VALUE_$n=$refhooks" \
      "$@"
}

# The reference render for ONE repo, taken only for a repo actually measured —
# a managed repo is classified and skipped before it gets here, and a fully
# managed box needs no reference at all.
#
# Nothing about the render is inferred from the repo: `install-hooks` is
# pointed at the repo itself, so the literals check 3 derives from it (ADR 0024
# D2) and the stamp config gives it are the ones this repo's hook must carry.
# The only thing this script arranges is WHERE the render lands.
refn=0
ref_commit_sha=""
ref_prepush_sha=""
need_ref() { # <repo> <short>
  local repo=$1 short=$2
  refn=$((refn + 1))
  refhooks="$tmp/refhooks/$refn"
  mkdir -p "$refhooks" 2>/dev/null ||
    { echo "verify-hook-freshness: cannot create $refhooks — nothing measured"; exit 2; }
  # The redirect has to be shown to have taken, and now it is load-bearing
  # twice: without it the render could land in the managed directory's shadow —
  # or fail to land at all — and the identity compare below would be against
  # whatever was read from somewhere else, but it is ALSO the only thing
  # keeping an install-hooks aimed at the operator's repo out of that repo's
  # own .git/hooks. Asked of git rather than assumed, and of the same lookup
  # posse's hooksDir asks (`--git-path hooks`), so this is the dispatch path
  # and not a path derived from one.
  local dispatch
  dispatch=$(ref_env git -C "$repo" rev-parse --git-path hooks 2>/dev/null)
  if [ "$dispatch" != "$refhooks" ]; then
    echo "verify-hook-freshness: the reference render for $short dispatches hooks from '${dispatch:-<git could not say>}', not $refhooks — the redirect did not take, nothing measured"
    exit 2
  fi
  local ref_out
  if ! ref_out=$(ref_env "$POSSE" gates install-hooks "$repo" 2>&1); then
    echo "verify-hook-freshness: reference render for $short failed — nothing measured"
    echo "$ref_out" | sed 's/^/    /'
    exit 2
  fi
  local ref_commit="$refhooks/prepare-commit-msg"
  local ref_prepush="$refhooks/pre-push"
  [ -r "$ref_commit" ] && [ -r "$ref_prepush" ] ||
    { echo "verify-hook-freshness: reference render for $short produced no hooks — nothing measured"; exit 2; }
  # That the render is posse's and complete, asked of the artifact rather than
  # of the message that announced it. This is the assertion the throwaway-repo
  # reference made as `visibility guard: public` — on a managed path
  # install-hooks writes no slot and prints no such line, which is how the
  # whole control died on an employer's box (ADR 0052, ranger-base-1se2l). Its
  # per-repo replacement cannot be "public": the render for a private repo is
  # stamped private and should be. What is still true of every posse render is
  # that the stamp line is IN it, so that is what is asked.
  grep -q "^posse_beads_visibility='" "$ref_commit" ||
    { echo "verify-hook-freshness: reference render for $short carries no visibility stamp — it is not a posse render, nothing measured"; exit 2; }
  ref_commit_sha=$(norm "$ref_commit")
  ref_prepush_sha=$(plain "$ref_prepush")
}

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
managed=0
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
  # ADR 0052 D1, asked before anything here is read — the same order
  # SweepHookWall asks it in, and for the same reason: on a managed hooks path
  # the two slots are the employer's, whatever is left in the repo's own
  # `.git/hooks` is a file git never runs, and neither one is a fact about
  # posse's wall. The verdict comes out of the binary rather than out of three
  # legs re-spelled here, and the line it prints is the line every other posse
  # caller prints about the same directory.
  #
  # BOTH the exit code and the line, because a skip is the one wrong answer
  # that is silent: a binary predating the query verb reads `managed-hooks` as
  # a persona name, and what it exits with then is that path's business, not a
  # verdict about any hooks directory. MEASURED 2026-09-05 on the installed
  # binary: `posse: no such agent: managed-hooks`, exit 1 — which would fall
  # through to today's behaviour anyway. Asking for the verdict's own line
  # keeps that true if the exit code ever moves, so an old posse under a new
  # script measures every repo the old way instead of skipping the whole box.
  if mline=$("$POSSE" gates managed-hooks "$repo" 2>/dev/null) &&
     [ "${mline#L3: managed hooks path }" != "$mline" ]; then
    say "    managed  $mline"
    say "             nothing of posse's is installed here to go stale — the session hooks dir is rendered at every launch"
    managed=$((managed + 1))
    continue
  fi
  [ -d "$hooks" ] || { finding "$short: no hooks dir at $hooks"; continue; }
  need_ref "$repo" "$short"
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
    # in the constitution repo and ~/src/posse, the two repos that carry class
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
# A managed repo is a measurement, not a gap: posse installs nothing there, so
# there is nothing there to have gone stale. Said out loud rather than counted
# silently — it is exactly the thing a reader would otherwise go looking for a
# missing verdict about (the wording ReportHookWall uses for the same fact).
if [ "$managed" -gt 0 ]; then
  echo "verify-hook-freshness: $managed repo(s) dispatch from a managed hooks path — posse writes nothing there, and the L3 wall is realized by the session hooks dir rendered at each launch (ADR 0052)"
fi
if [ "$measured" -eq 0 ]; then
  if [ "$managed" -gt 0 ]; then
    echo "verify-hook-freshness: no repo carries a posse-installed hook that could be stale — nothing to re-render"
    exit 0
  fi
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
