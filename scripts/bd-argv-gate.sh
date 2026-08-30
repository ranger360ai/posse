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

# The parser did not run at all. Refuse anything that mentions bd as a word.
#
# The parser reasons about the DECODED command; everything down here reasons
# about the payload's TEXT, and that gap has been the fail-OPEN hole twice now.
# The second time (ranger-base-1lvm) the counter-example was a bd call that is
# simply not on the FIRST LINE of the command: the harness encodes the newline
# as the two characters `\` `n`, so the character in front of `bd` was the `n`
# of the escape — which is a word character — and the raw grep missed it, while
# deleting backslashes joined that same `n` on to make `anbd` and the stripped
# grep missed it too. MEASURED: 228 escapes in 47275 real command lines, ALL of
# them ordinary multi-line commands.
#
# So decode first, then ask the question. Decoding a JSON string is a
# left-to-right scan and sed's `s///g` IS one — it consumes non-overlapping
# matches in order, so `\\n` is eaten as the escaped backslash `\\` (leaving a
# literal `n`) and is never mistaken for the newline escape `\n`. That one
# property is what makes this sound rather than one more character class.
#
# Two arms, because bd has two shapes in a payload:
#   A. a literal `bd` in the decoded command. Every escape becomes a SEPARATOR,
#      which can only create word boundaries — it can never hide a `bd`.
#   B. a `bd` the shell concatenates — `b\d`, `b''d`, `b"d"` (ranger-base-hthx).
#      Here `\\` and `\"` decode to a backslash and a quote, which the strip
#      then deletes, so the concatenation survives the decode.
#
# `\uXXXX` is decoded exactly, not approximated: `\u0062` and `\u0064` are the
# ONLY four-hex spellings that can yield a `b` or a `d` (the hex letters a-f
# never occur in `0062`/`0064`), so every other one becomes a separator with
# nothing lost. That keeps the answer identical whichever encoder produced the
# payload — node's JSON.stringify leaves `&<>` alone, Go's encoding/json spells
# them `\u0026`, and this arm no longer cares (MEASURED: same verdict on all
# 47275, both encodings).
#
# Cost of decoding, measured on the same corpus: 279 of 47275 commands (0.59%)
# are refused here that the old text test let through — every one of them a
# command that already had to reach the parser, refused only while the parser
# is broken. Collapsing this test into the fast path instead would have cost
# 13.5%, which is the "wedges every Bash call on the box" that ranger-base-3bqn
# forbids.
bd_word='(^|[^A-Za-z0-9_.-])bd([^A-Za-z0-9_-]|$)'

# Shared by both arms, so the two spellings cannot drift apart.
decode_u() { sed -e 's/\\u0062/b/g' -e 's/\\u0064/d/g' -e 's/\\u..../ /g'; }

#   C. a backslash down here can be a JSON escape (A and B) or, in a payload
#      nobody encoded, the shell's own quoting — and having failed to reach the
#      parser we cannot tell which. So ask under the other reading too. This is
#      the test that shipped before, kept for exactly that case: it adds
#      nothing on top of A and B for a real JSON payload (MEASURED: A|B is a
#      strict superset over all 47275 real command lines, both encodings), and
#      it is what still catches `b\d` in a payload that is not JSON at all.
if printf '%s' "$input" | sed -e 's/\\\\/ /g' | decode_u |
     sed -e 's/\\./ /g' | grep -Eq "$bd_word" ||
   printf '%s' "$input" | sed -e 's/\\\\//g' | decode_u |
     sed -e 's/\\"//g' -e 's/\\./ /g' | tr -d '\\'\''"' | grep -Eq "$bd_word" ||
   printf '%s' "$input" | tr -d '\\'\''"' | grep -Eq "$bd_word"; then
  echo "bd-argv-gate: parser unavailable (${python} ${py}, exit ${rc}); refusing this bd call (fail closed)" >&2
  exit 2
fi
exit 0
