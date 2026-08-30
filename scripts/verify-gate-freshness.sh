#!/usr/bin/env bash
# Detective control for the operator's copy of the bd argv gate going stale
# (ranger-base-d0jo).
#
# WHY THIS EXISTS AS A CHECK AND NOT AS A COPY
# `scripts/bd-argv-gate.{sh,py}` in this repo is source. The file that
# actually fences this box is the copy a PreToolUse hook names in the
# operator's Claude settings — "a PreToolUse hook the operator may install,
# not one posse renders" (ADR 0015 §3, quoting ADR 0014 §5 unamended), and
# ADR 0035's same principle: posse writes no config it does not own. So a
# change to the source moves nothing, and nothing anywhere notices — the copy
# goes stale at the next commit to either file and stays stale silently.
#
# That is not hypothetical: it happened to c892569 (ranger-base-1lvm, the
# fail-closed fallback decoding the payload before testing it), landing on a
# box whose copy predated it. ranger-base-hthx had already written the words
# "the live escape stays open until one `cp`" on its own close. A note on a
# close is not a control — it is read by whoever reads that bead. This runs at
# promote.
#
# WHAT IT COMPARES. Not "the file at ~/.config/posse/gate/" — the file the
# hook REACHES. The wrapper path comes out of the operator's settings.json
# (PreToolUse, matcher Bash), and the parser path comes out of the wrapper's
# own rule for finding it: `$(dirname $0)/bd-argv-gate.py`. Hard-coding the
# directory would pass while the hook pointed somewhere else entirely.
#
# The reference is HEAD, not the working tree. `make install` builds the
# binary it promotes from HEAD in a throwaway worktree (scripts/clean-build.sh)
# precisely because personas share this checkout, and the gate copy is
# promoted at the same moment for the same reason: nobody's unfinished edit
# gets installed box-wide. A working tree that differs from HEAD for these two
# files is reported, and the prescription changes to one that extracts HEAD.
#
# Then behavior, because identity alone cannot see a dead gate — a
# byte-perfect wrapper next to a missing python3 or an unreadable parser is a
# fence that passes everything. Three arms against the INSTALLED wrapper, each
# with a failing opposite:
#   allow      `bd list` — reaches the parser (the fast path sees a literal
#              `bd`), verdict None: rc 0, nothing on stdout. A dead parser
#              fails here, because the wrapper's fallback refuses instead.
#   deny       `bd daemon stop` — rc 0 with a deny decision on stdout. The
#              deny has to come from the PARSER (rc 0 + JSON); the wrapper's
#              fail-closed fallback exits 2, so this arm distinguishes a
#              working gate from a gate that is only failing closed.
#   pass-through  `go test ./...` — no `bd` to spell, answered by the fast
#              path: rc 0, nothing on stdout. A wrapper that wedges every Bash
#              call on the box is the worst failure mode and the cheapest to
#              catch.
#
# WHAT IT WILL NOT DO. It installs nothing. The gate is the operator's copy by
# ADR, and the whole point of it being a copy is that a persona-writable tree
# cannot move it — a `--install` flag here would be that tree, one flag away.
# A finding prints the exact command to type. That command is written to
# survive the gate it repairs: no `$(`, no heredoc, because a payload that
# mentions bd behind a construct this gate cannot read is refused, and a fix
# the fence refuses is not a fix.
#
# Exit 0 clean · 1 findings · 2 nothing measured.
set -uo pipefail

warn_only=0
wrapper_override=""
while [ $# -gt 0 ]; do
  case "$1" in
    --warn) warn_only=1 ;;   # report and exit 0 — `make install` uses this
    --wrapper) shift; wrapper_override="${1:-}" ;;
    *) echo "usage: $(basename "$0") [--warn] [--wrapper PATH]" >&2; exit 2 ;;
  esac
  shift
done

nothing() { echo "verify-gate-freshness: $* — nothing measured"; exit 2; }

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd) || nothing "cannot locate the repo"
git -C "$here" rev-parse --verify -q HEAD >/dev/null 2>&1 || nothing "$here has no HEAD"

# THE REFERENCE IS THE MAIN CHECKOUT'S HEAD, NOT THIS TREE'S. A persona runs
# in a linked worktree (~/.posse/worktrees/...) whose HEAD carries work that
# has not landed; the operator promotes from ~/src/posse. Measuring against
# the worktree would call the box fresh for a commit nobody else has, and
# prescribe installing a persona's branch box-wide — the precise thing the
# copy exists to prevent. --git-common-dir is the main .git for a linked
# worktree and a bare `.git` for the main checkout itself.
common=$(git -C "$here" rev-parse --git-common-dir) || nothing "cannot resolve the git common dir"
case "$common" in /*) ;; *) common="$here/$common" ;; esac
repo=$(cd -- "$common/.." 2>/dev/null && pwd) || repo="$here"
git -C "$repo" rev-parse --verify -q HEAD >/dev/null 2>&1 || repo="$here"
linked=0
[ "$repo" = "$here" ] || linked=1

PYTHON="${BD_ARGV_GATE_PYTHON:-python3}"
command -v "$PYTHON" >/dev/null 2>&1 || nothing "no $PYTHON on PATH (the gate itself needs it)"

# -- which file does the hook actually reach? -------------------------------
#
# The settings hold a shell command, not a path: `'/abs/bd-argv-gate.sh'` here,
# but `bash '/abs/...'` is just as legal. shlex is the right reader for a
# string the shell will read, and it is the same module the parser resolves
# command words with — one less way for this check to disagree with the gate.
config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
wrappers=()
if [ -n "$wrapper_override" ]; then
  wrappers=("$wrapper_override")
else
  while IFS= read -r line; do
    [ -n "$line" ] && wrappers+=("$line")
  done < <("$PYTHON" - "$config_dir" <<'PY'
import json, os, shlex, sys
seen, out = set(), []
for name in ("settings.json", "settings.local.json"):
    p = os.path.join(sys.argv[1], name)
    try:
        with open(p) as fh:
            data = json.load(fh)
    except Exception:
        continue
    for block in (data.get("hooks") or {}).get("PreToolUse") or []:
        for hook in block.get("hooks") or []:
            cmd = hook.get("command")
            if not isinstance(cmd, str) or "bd-argv-gate.sh" not in cmd:
                continue
            try:
                words = shlex.split(cmd)
            except ValueError:
                words = cmd.split()
            for w in words:
                if w.endswith("bd-argv-gate.sh") and w not in seen:
                    seen.add(w)
                    out.append(w)
print("\n".join(out))
PY
  )
fi

[ "${#wrappers[@]}" -gt 0 ] || nothing "no PreToolUse hook in $config_dir names bd-argv-gate.sh (the gate is not installed on this box)"

# -- the reference ----------------------------------------------------------
ref=$(mktemp -d "${TMPDIR:-/tmp}/gate-head.XXXXXX") || nothing "cannot make a scratch dir"
trap 'rm -rf "$ref"' EXIT HUP INT TERM
for f in bd-argv-gate.sh bd-argv-gate.py; do
  git -C "$repo" show "HEAD:scripts/$f" > "$ref/$f" 2>/dev/null \
    || nothing "HEAD carries no scripts/$f"
done

# Bare `git diff HEAD -- <paths>`, never the two-dot form: another persona's
# staged edit to these files is exactly the state that must not be installed
# box-wide (ADR 0022, and the same trap ranger-base-erba landed b291784 for).
dirty=0
git -C "$repo" diff --quiet HEAD -- scripts/bd-argv-gate.sh scripts/bd-argv-gate.py || dirty=1

# Two classes, kept apart because they take different repairs. `stale` is an
# identity or presence finding and a reinstall fixes it. `broken` is a
# behaviour finding on files that ARE HEAD's — reinstalling those same bytes
# fixes nothing, and prescribing it would be advice that cannot work.
findings=0 stale=0 broken=0
finding()  { findings=$((findings + 1)); stale=$((stale + 1));  echo "  FINDING  $*"; }
finding_b() { findings=$((findings + 1)); broken=$((broken + 1)); echo "  FINDING  $*"; }

# -- one payload builder, no command substitution ---------------------------
# These three commands are plain ASCII with nothing JSON must escape, so the
# payload is built by printf rather than by starting a second interpreter.
probe() {                                # probe <wrapper> <command>; sets rc/out
  out=$(printf '{"tool_name":"Bash","tool_input":{"command":"%s"}}' "$2" | "$1" 2>/dev/null)
  rc=$?
}

echo "verify-gate-freshness: reference $repo @ $(git -C "$repo" rev-parse --short HEAD) · ${#wrappers[@]} installed gate(s)"
[ "$linked" -eq 0 ] || echo "  (run from the linked worktree $here — the reference is still the main checkout, which is what promote installs)"

for w in "${wrappers[@]}"; do
  echo ""
  echo "  $w"
  py="${w%/*}/bd-argv-gate.py"

  # -- identity
  if [ ! -f "$w" ]; then
    finding "the hook names a file that is not there — every Bash call on this box runs with no gate"
    continue
  fi
  [ -x "$w" ] || finding "not executable — the hook execs it directly, with no interpreter in front"
  if cmp -s "$w" "$ref/bd-argv-gate.sh"; then
    echo "    fresh    bd-argv-gate.sh matches HEAD"
  else
    finding "bd-argv-gate.sh is STALE — it is not HEAD's"
  fi
  if [ ! -r "$py" ]; then
    finding "bd-argv-gate.py is missing or unreadable beside the wrapper ($py)"
  elif cmp -s "$py" "$ref/bd-argv-gate.py"; then
    echo "    fresh    bd-argv-gate.py matches HEAD"
  else
    finding "bd-argv-gate.py is STALE — it is not HEAD's"
  fi

  # -- behavior, against the installed file
  probe "$w" "bd list"
  if [ "$rc" -ne 0 ] || [ -n "$out" ]; then
    finding_b "an allowed verb does not get through (rc $rc) — the parser is not running, the wrapper is only failing closed"
  else
    probe "$w" "bd daemon stop"
    if [ "$rc" -ne 0 ]; then
      finding_b "a denied verb is refused by the FALLBACK, not the parser (rc $rc) — the gate has no allow-list any more"
    elif [ "${out#*permissionDecision}" = "$out" ] || [ "${out#*deny}" = "$out" ]; then
      finding_b "a denied verb is NOT refused — the fence is open"
    else
      probe "$w" "go test ./..."
      if [ "$rc" -ne 0 ] || [ -n "$out" ]; then
        finding_b "an unrelated command is answered (rc $rc) — this wedges every Bash call on the box"
      else
        echo "    behaves  bd list allowed · bd daemon stop denied by the parser · go test untouched"
      fi
    fi
  fi
done

echo ""
if [ "$findings" -eq 0 ]; then
  [ "$dirty" -eq 0 ] || echo "verify-gate-freshness: note — scripts/bd-argv-gate.* differ from HEAD in this checkout; the installed copies match HEAD, which is what promote installs"
  echo "verify-gate-freshness: fresh — the installed gate is HEAD's and all three arms hold"
  exit 0
fi

gate_dir="${wrappers[0]%/*}"
echo "verify-gate-freshness: findings above"
echo ""
if [ "$stale" -eq 0 ]; then
  # Behaviour only: the files ARE HEAD's. Reinstalling the same bytes is the
  # one prescription that cannot work, so it is not printed.
  cat <<EOF
The installed files are HEAD's — a reinstall would copy the same bytes and
change nothing. What failed is the gate RUNNING, so the break is under it:
python3 (the wrapper's default, or BD_ARGV_GATE_PYTHON), the parser's mode, or
the settings entry pointing somewhere unexpected. Reproduce the arm by hand:

  printf '{"tool_name":"Bash","tool_input":{"command":"bd daemon stop"}}' | $gate_dir/bd-argv-gate.sh

A working gate answers that with a deny decision on stdout and exit 0.
EOF
elif [ "$dirty" -eq 0 ]; then
  cat <<EOF
Refresh the operator's copy from the main checkout — one line, and it is the
operator's to type (posse renders no box-wide hook):

  install -m 0755 $repo/scripts/bd-argv-gate.sh $repo/scripts/bd-argv-gate.py $gate_dir/

Then re-run: make verify-gate-freshness
EOF
else
  cat <<EOF
scripts/bd-argv-gate.* in this checkout DIFFER FROM HEAD, so the working tree
is not what to install — promote installs HEAD (scripts/clean-build.sh does
the same for the binary). Extract HEAD first:

  mkdir -p /tmp/gate-head
  git -C $repo archive HEAD scripts/bd-argv-gate.sh scripts/bd-argv-gate.py | tar -x -C /tmp/gate-head
  install -m 0755 /tmp/gate-head/scripts/bd-argv-gate.sh /tmp/gate-head/scripts/bd-argv-gate.py $gate_dir/

Then re-run: make verify-gate-freshness
EOF
fi

[ "$warn_only" -eq 1 ] && exit 0
exit 1
