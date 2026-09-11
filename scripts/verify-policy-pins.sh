#!/usr/bin/env bash
# Detective control for the root-owned policy-tier drop-ins going stale
# (ranger-base-lle18).
#
# WHY THIS EXISTS AS A CHECK AND NOT AS A COPY
# `etc/claude/managed-settings.d/*.json` in this repo is SOURCE. The files that
# actually pin this box are the root-owned copies under the runtime's managed
# directory, and posse installs neither: "installing it is the operator's, per
# change" (inletpin_qa_test.go, TestQAThePolicyDropInMatchesTheInletPin). So a
# change to a shipped file moves nothing on the box, and until this script
# nothing anywhere noticed — the same shape verify-gate-freshness.sh exists for
# one tier down.
#
# THE OUTAGE IT IS WRITTEN FROM, and it is worth reading because the reasoning
# that produced it was careful rather than careless. ranger-base-888fv took the
# GIT_EXTERNAL_DIFF row out of inletPin() and out of the shipped drop-in on
# 2026-09-06 (operator ruling B on ranger-base-5sph1) and deliberately left the
# installed copy alone, closing with "Root drop-in NOT reinstalled (no-op at
# that tier), left to the operator" — resting on ranger-base-sn0w8's
# measurement that an empty string in the root-owned policy tier does not take.
#
# THAT INFERENCE DOES NOT HOLD, measured on this box 2026-09-11 (claude 2.1.x,
# darwin 25.4.0), and the measurement is a single-variable natural experiment
# rather than an argument: on 2026-09-11 the installed 10-posse-inlet-pin.json
# was byte-for-byte the shipped file PLUS the one `"GIT_EXTERNAL_DIFF": ""`
# row, the shipped binary at ~/.local/bin/posse contained the string
# GIT_EXTERNAL_DIFF zero times, and every dispatched session's `env` carried
# `GIT_EXTERNAL_DIFF=` set and empty. One key, present at exactly one end,
# reaching the child. sn0w8 measured that policy-tier "" does not OVERRIDE a
# name the process environment already carries; it is not a measurement that
# the row fails to REACH a name nothing else sets, and those are different
# facts. git reads set-but-empty as an external diff command of "" and execs
# it, so bare `git diff` died `error: cannot run : No such file or directory /
# external diff died` in every posse-launched seat on this box for five days.
#
# WHAT IT COMPARES, and in which direction. Three classes, kept apart because
# each takes a different repair and only one of them is what bit:
#   STALE    installed, and not the bytes HEAD ships. Reinstall.
#   MISSING  HEAD ships it, the box does not have it. Install.
#   EXTRA    an installed `*-posse-*.json` HEAD no longer ships. Remove. This
#            is the same defect as STALE with the whole file retired instead
#            of one row, and a name-keyed comparison that only walked the
#            shipped side could not see it.
# A drop-in in that directory that is NOT `*-posse-*.json` is the employer's
# and is never a finding: this repo's three are the only ones it may speak for.
#
# THE REFERENCE IS THE MAIN CHECKOUT'S HEAD, for the reason
# verify-gate-freshness.sh gives: a persona runs in a linked worktree whose
# HEAD carries work nobody has landed, and prescribing "install this" from
# there would install a persona's branch box-wide. A working tree that differs
# from HEAD for these paths is reported, so a reader knows the prescription
# below is HEAD's bytes and not the ones in front of them.
#
# WHAT IT WILL NOT DO. It installs nothing, removes nothing and never runs
# sudo. Writing the policy tier is a root write to a deployed system and is the
# operator's every time; a `--install` flag here would put that write one flag
# from a persona-writable tree, which is the whole reason the copy is a copy. A
# finding prints the exact line for a person to type.
#
# THE TWO FLAGS ARE READ SEAMS AND NOT POLICY. --policy-dir and --repo narrow
# what this reads so the QA arms can drive it against scratch fixtures instead
# of the operator's live box (credentialpaths' scratch-HOME discipline). Both
# have to be TYPED; neither is read from the environment, so nothing a session
# inherits can turn a finding into a silent pass.
#
# Exit 0 clean · 1 findings · 2 nothing measured.
set -uo pipefail

self=$(basename "$0")
policy_override=""
repo_override=""
while [ $# -gt 0 ]; do
  case "$1" in
    --policy-dir|--repo)
      [ $# -ge 2 ] || { echo "$self: $1 needs a value" >&2; exit 2; }
      # A flag whose value went missing must not fall through to the real box:
      # that is a typo reading the operator's machine under a test's name.
      case "$1" in
        --policy-dir) policy_override="$2" ;;
        --repo) repo_override="$2" ;;
      esac
      shift
      ;;
    *) echo "usage: $self [--policy-dir PATH] [--repo PATH]" >&2; exit 2 ;;
  esac
  shift
done

nothing() { echo "$self: $* — nothing measured, not a pass"; exit 2; }

# -- the reference ----------------------------------------------------------
if [ -n "$repo_override" ]; then
  repo=$(cd -- "$repo_override" 2>/dev/null && pwd) || nothing "no such --repo: $repo_override"
else
  here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd) || nothing "cannot locate the repo from $0"
  git -C "$here" rev-parse --verify -q HEAD >/dev/null 2>&1 || nothing "$here has no HEAD"
  # --git-common-dir is the MAIN .git for a linked worktree and a bare `.git`
  # for the main checkout itself, so this resolves to what promote installs
  # from either way.
  common=$(git -C "$here" rev-parse --git-common-dir) || nothing "cannot resolve the git common dir"
  case "$common" in /*) ;; *) common="$here/$common" ;; esac
  repo=$(cd -- "$common/.." 2>/dev/null && pwd) || repo="$here"
  git -C "$repo" rev-parse --verify -q HEAD >/dev/null 2>&1 || repo="$here"
fi
git -C "$repo" rev-parse --verify -q HEAD >/dev/null 2>&1 || nothing "$repo has no HEAD"

src="etc/claude/managed-settings.d"
shipped=()
while IFS= read -r p; do
  [ -n "$p" ] && shipped+=("${p##*/}")
done < <(git -C "$repo" ls-tree --name-only "HEAD:$src" 2>/dev/null)

# A reference that ships nothing would score every box green forever.
[ "${#shipped[@]}" -gt 0 ] || nothing "HEAD:$src names no drop-in in $repo"

# -- where the runtime reads them -------------------------------------------
# Darwin's managed directory, which is where this box's three are installed and
# the only one cost.go names. The resolver joins the managed dir with
# "managed-settings.d" and treats an absent one as "no drop-ins", so a box
# without the directory has no policy tier and this script has measured nothing
# rather than found it clean.
policy_dir="/Library/Application Support/ClaudeCode/managed-settings.d"
[ -z "$policy_override" ] || policy_dir="$policy_override"
[ -d "$policy_dir" ] || nothing "no policy drop-in directory at $policy_dir (this box has no policy tier installed)"

short=$(git -C "$repo" rev-parse --short HEAD)
dirty=0
git -C "$repo" diff --quiet HEAD -- "$src" 2>/dev/null || dirty=1

echo "$self: reference $repo @ $short · ${#shipped[@]} shipped drop-in(s) · policy dir $policy_dir"
[ "$dirty" -eq 0 ] || echo "  (the working tree at $repo differs from HEAD under $src — the reference below is HEAD, not what is in front of you)"

findings=0
finding() { findings=$((findings + 1)); echo "  FINDING  $*"; }

for n in "${shipped[@]}"; do
  inst="$policy_dir/$n"
  if [ ! -f "$inst" ]; then
    finding "MISSING  $n is not installed — HEAD ships it and this box is not carrying it"
    continue
  fi
  if git -C "$repo" show "HEAD:$src/$n" 2>/dev/null | cmp -s - "$inst"; then
    echo "  fresh    $n matches HEAD"
  else
    finding "STALE    $n is installed and is not HEAD's bytes — what is in force on this box is not what this repo says is in force"
  fi
done

# -- the retired-file half --------------------------------------------------
# Keyed on the directory rather than on the shipped list, because a file HEAD
# has stopped shipping is named by nothing on the shipped side and is exactly
# as in-force as a stale one.
extra=$(find -H "$policy_dir" -maxdepth 1 -name '*-posse-*.json' -print 2>/dev/null)
rc=$?
if [ "$rc" -ne 0 ]; then
  nothing "could not scan $policy_dir (find exit $rc)"
fi
while IFS= read -r f; do
  [ -n "$f" ] || continue
  b="${f##*/}"
  known=0
  for n in "${shipped[@]}"; do
    [ "$b" = "$n" ] && known=1
  done
  [ "$known" -eq 1 ] || finding "EXTRA    $b is installed and HEAD ships no such drop-in — a retired pin still in force"
done <<< "$extra"

if [ "$findings" -ne 0 ]; then
  echo ""
  echo "$self: $findings finding(s) against $repo @ $short"
  echo ""
  echo "Every line above is a difference between what this repo says pins this box and what"
  echo "pins it. Repairing it is a root write to a deployed system and is the operator's:"
  echo "file the ask, do not install it yourself. STALE or MISSING, per file:"
  echo ""
  echo "    git -C $repo show HEAD:$src/<file> | sudo tee '$policy_dir/<file>' >/dev/null"
  echo "    sudo chmod 0644 '$policy_dir/<file>'"
  echo ""
  echo "EXTRA is a deletion of that file from '$policy_dir', also by hand, also root."
  echo "Then re-run this check, and read a seat's own env back: the row that produced"
  echo "ranger-base-lle18 was invisible from every end except the environment it reached."
  exit 1
fi

echo "$self: clean — ${#shipped[@]} shipped drop-in(s) installed and matching HEAD, no retired pin left in force"
