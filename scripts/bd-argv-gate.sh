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

# THE SCRATCH FILE LIVES INSIDE A DIR, AND THE DIR IS WHAT GETS COLLECTED
# (ranger-base-u1z89). This was a bare `mktemp` FILE in /tmp, made once per
# bd-ish tool call of every seat on this box — by far the most frequent
# scratch creator here — and every candidate walk in
# scripts/reap-test-tmp.sh is `-type d`. So a leaked one was outside all
# nine of the reaper's classes: not the 24h quarantine the generic
# `tmp.??????????` namespace gets, but no walk that could ever reach it
# whatever it was named. The trap covers the ordinary path and every
# catchable signal; SIGKILL and the OOM killer are what leak, and on a 14G
# tmpfs with one box-wide inode budget for uid posse an inode is an inode.
#
# A `posse-*` dir is collected by the reaper's posse-scratch class at 120m,
# so the leak becomes two inodes with an owner instead of one with none, and
# it costs no extra PROCESS: `mktemp -d` is the same single fork `mktemp`
# was, and `rm -rf` the same single fork as `rm -f`.
#
# WHAT IT DOES COST, measured on the gate itself rather than argued from the
# syscalls. MEASURED 2026-11-12 (linux, this box), this file against its
# predecessor, interleaved, three pairs of 200 calls on a payload that
# reaches the parser, at a one-minute load average of 1.7-1.9: 26.7, 26.6
# then 26.5 ms per call for the file and 27.0, 26.7 then 26.7 for the dir —
# +194 us, 0.73%, the same sign in all three pairs. A create-write-remove
# micro-benchmark of the two shapes on their own read +24 us (1333 against
# 1357 us over four interleaved pairs of 300, at a load of 15-20), so most
# of the 194 us is `rm -rf`'s extra syscalls over a directory and not the
# mkdir the bead expected to pay for. Neither number is visible against this
# gate's own spread on a busy box: three consecutive readings of the
# UNCHANGED gate at a load of 17-19 read 36.9, 48.5 and 63.4 ms per call.
#
# THE SIGNAL ARMS OF THIS TRAP DELIBERATELY RETURN, WHICH IS THE OPPOSITE OF
# THE HOUSE RULE (ranger-base-kkp5g). Everywhere else under scripts/, a
# handler trapped on a deadly signal must EXIT, because bash otherwise runs it
# and RESUMES the script over a torn-down apparatus. Here resuming is the
# better answer, and every alternative is worse:
#
#   - This is a PreToolUse hook, so its EXIT STATUS IS ITS VERDICT. Claude's
#     contract, MEASURED against the real binary under ranger-base-3bqn: exit
#     2 blocks the tool call, ANY OTHER nonzero shows stderr and RUNS IT — a
#     hook that exited 1 let the refused command through. So `exit 130`/`exit
#     143` here is fail-OPEN, on the one script in that census where failing
#     open is the whole stake.
#   - An unconditional `exit 2` instead would refuse every signalled Bash call
#     on the box whether or not it names bd, which is the fleet-wide false
#     refusal the same bead forbids.
#   - Dropping INT/TERM/HUP from the trap is worse again: bash's default
#     disposition then kills the gate before it can answer, which is fail
#     open. That is the whole of the reason and it is enough. It does NOT
#     leak $errd, which this bullet used to claim: MEASURED 2026-11-13 (linux
#     7.2.15, bash 5.3.20; ranger-base-5owgm, docs/notes.d/ranger-base-kbaah.md),
#     an UNTRAPPED terminating signal still RUNS bash's EXIT trap, exits
#     128+signo and does not resume — so `rm -rf -- "${errd:?}"` happens
#     anyway and the scratch dir goes either way.
#
# Resuming is what lets the gate still reach its own fail-closed decision, and
# MEASURED 2026-11-13 it does: bash RESTORES $? across a signal handler, so
# `rc=$?` below still reads the parser's status and not the handler's. What
# resumption costs is one line of PROSE and no verdict — the `grep -q
# '^bd-argv-gate:' "$err"` below reads a file inside the removed $errd, so a
# parser that refused WITH A REASON is reported by the generic "parser
# unavailable" line instead of its own. Still exit 2, still fail closed.
#
# Rostered as the one exemption in internal/treepins/trapsignalexit_qa_test.go
# (tsgExempt), which reds if this trap ever starts ending the shell.
errd=$(mktemp -d "${TMPDIR:-/tmp}/posse-bd-argv-gate.XXXXXX") || errd=
if [ -n "$errd" ]; then
  err="$errd/stderr"
  trap 'rm -rf -- "${errd:?}"' EXIT HUP INT TERM
else
  err=/dev/null
fi

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
