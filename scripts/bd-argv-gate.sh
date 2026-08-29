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

# Fast path: this hook runs on EVERY Bash call, and starting an interpreter
# for `go test` is a tax on the whole fleet. A payload that cannot SPELL bd
# cannot produce any refusal below, so it is answered here by a shell builtin,
# ~0 ms instead of ~30 ms.
#
# "Spell" is the load-bearing word, and the first cut of this file got it
# wrong: it tested for a literal `bd` substring, but the parser resolves the
# command word with shlex FIRST and only then asks `basename(word) == "bd"`.
# So every spelling the shell concatenates into bd — `b\d`, `b''d`, `b"d"`,
# `'b'd` — was refused by the parser and never reached it, because the payload
# carries no literal `bd` (MEASURED, ranger-base-hthx: `b\d ship --help` ran
# through the shipped fence one character away from a refusal).
#
# The arms below are the ways `bd` can be spelled in a payload:
#   *bd*    the literal.
#   *'\u'*  a JSON escape, which decodes after this test (`bd`).
#   *b\\*   *b\'*  *b\"*   a `b` adjacent to one of the shell's three quoting
#           characters — the NECESSARY condition for a concatenation, since
#           the character after the `b` of bd is either the `d` itself or the
#           quoting that hides it. (In a JSON payload a command's `"` arrives
#           as `\"`, so the b\\ arm already covers it; the b\" arm is there for
#           a payload that is not JSON-escaped, and has no witness under one.)
#
# Soundness is not an argument, it is measured: over every command word of
# length <= 6 spelled out of {b, d, \, ', "}, 55986 of them, the parser
# refuses 429 and this test sends all 429 to the parser (the old test waved
# 179 through). The cost is 32 extra parser starts in 12777 real command
# lines harvested from this repo — 0.25%. Still a SUBSTRING test, still
# deliberately looser than the parser's word match.
case $input in
  *bd*|*'\u'*|*b\\*|*b\'*|*b\"*) ;;
  *) exit 0 ;;
esac

err=$(mktemp "${TMPDIR:-/tmp}/bd-argv-gate.XXXXXX") || err=/dev/null
[ "$err" = /dev/null ] || trap 'rm -f "$err"' EXIT HUP INT TERM

# -S -E: no site dir, and PYTHON* env ignored — a PYTHONPATH pointing at a
# fake `shlex.py` would otherwise be a way to neuter the parser. Also the
# difference between 30 ms and 13 ms per call (measured, python 3.14).
out=$(printf '%s' "$input" | "$python" -S -E "$py" 2>"$err")
rc=$?
if [ "$rc" -eq 0 ]; then
  [ -n "$out" ] && printf '%s\n' "$out"
  exit 0
fi
if [ "$rc" -eq 2 ] && grep -q '^bd-argv-gate:' "$err" 2>/dev/null; then
  cat "$err" >&2                       # the parser's own reason, verbatim
  exit 2
fi

# The parser did not run at all. Refuse anything that mentions bd as a word —
# in the payload as it arrived, AND in the payload with the shell's quoting
# characters removed, so `b\d daemon stop` is refused here too (ranger-base-hthx).
# Both, not just the stripped one: deleting backslashes also eats the JSON
# escapes, and `a\nbd` would collapse to the single word `anbd`.
bd_word='(^|[^A-Za-z0-9_.-])bd([^A-Za-z0-9_-]|$)'
if printf '%s' "$input" | grep -Eq "$bd_word" ||
   printf '%s' "$input" | tr -d '\\'\''"' | grep -Eq "$bd_word"; then
  echo "bd-argv-gate: parser unavailable (${python} ${py}, exit ${rc}); refusing this bd call (fail closed)" >&2
  exit 2
fi
exit 0
