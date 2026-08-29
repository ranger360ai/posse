#!/bin/sh
# The bd argv gate's entry point — the command a PreToolUse hook names.
#
# Its whole job is the failure path. A Claude Code command hook that exits
# non-zero (other than 2) is FAIL-OPEN: "Other exit codes - show stderr to
# user only but continue with tool call" (claude 2.1.251, its own contract).
# So a missing python3, an unreadable script or a syntax error in it would
# silently remove the fence. Here the parser not running is itself a refusal
# — but only for a call that mentions bd, so a broken interpreter degrades bd
# rather than wedging every Bash call on the box (ranger-base-3bqn).
#
# The rc==2 arm is NOT taken on the exit status alone: `python3 <missing
# file>` also exits 2, and honouring that would turn a mistyped path in
# settings.json into a deny of every Bash call in the fleet (MEASURED here,
# the first cut of this file did exactly that). The parser's own refusals are
# recognized by the marker it writes on stderr.
#
# Exit codes out of this script: 0 with JSON on stdout = decision; 0 with no
# output = not our business, normal permission rules apply; 2 = block.
set -u

self_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
py=${BD_ARGV_GATE_PY:-"$self_dir/bd-argv-gate.py"}
python=${BD_ARGV_GATE_PYTHON:-python3}

input=$(cat)

err=$(mktemp "${TMPDIR:-/tmp}/bd-argv-gate.XXXXXX") || err=/dev/null
[ "$err" = /dev/null ] || trap 'rm -f "$err"' EXIT HUP INT TERM

out=$(printf '%s' "$input" | "$python" "$py" 2>"$err")
rc=$?
if [ "$rc" -eq 0 ]; then
  [ -n "$out" ] && printf '%s\n' "$out"
  exit 0
fi
if [ "$rc" -eq 2 ] && grep -q '^bd-argv-gate:' "$err" 2>/dev/null; then
  cat "$err" >&2                       # the parser's own reason, verbatim
  exit 2
fi

# The parser did not run at all. Refuse anything that mentions bd as a word.
if printf '%s' "$input" | grep -Eq '(^|[^A-Za-z0-9_.-])bd([^A-Za-z0-9_-]|$)'; then
  echo "bd-argv-gate: parser unavailable (${python} ${py}, exit ${rc}); refusing this bd call (fail closed)" >&2
  exit 2
fi
exit 0
