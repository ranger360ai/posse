#!/bin/sh
# verify-bd-argv-gate — the sh wrapper must never answer a call the parser
# would have refused.
#
# The bd argv gate is two programs: scripts/bd-argv-gate.sh decides, in a
# shell builtin, whether to start scripts/bd-argv-gate.py at all. That fast
# path exists so `go test` does not pay for an interpreter on every Bash call,
# and its ONE obligation is to be LOOSER than the parser.
#
# It has been wrong once (ranger-base-hthx): it tested the payload for a
# literal `bd` substring, while the parser resolves the command word with
# shlex FIRST and only then asks whether the basename is bd. So `b\d`, `b''d`,
# `b"d"` and `'b'd` were refused by the parser and never reached it — `b\d
# ship --help` ran through the shipped fence, one character from a refusal.
#
# The wrapper has a SECOND obligation, and this script used to sweep only the
# first: when the parser cannot run at all, the wrapper's own fail-closed
# fallback must refuse everything the parser would have refused. That one was
# wrong too (ranger-base-1lvm) — the fallback read the payload's TEXT, where
# the harness spells a newline as the two characters `\` `n`, so any bd call
# that was not on the FIRST line of the command was waved through. 228 escapes
# in 47275 real command lines, and no pin or sweep could see it because every
# command in every corpus was a single line.
#
# So both obligations are swept here, and the alphabet carries a NEWLINE.
#
# The Go pins in bdargvgate_qa_test.go enumerate the spellings someone thought
# of. This walks the whole alphabet instead: every command word that can be
# spelled from {b, d, backslash, single quote, double quote, space, newline}
# up to MAXLEN characters, each run through BOTH programs, and — where the
# parser refuses it — through the wrapper again with the parser broken. Any
# command the parser refuses and the wrapper does not is an escape, and is
# printed.
#
# Exit 0 = agreed on every spelling. 1 = escapes found. 2 = nothing was
# measured (no python3, a corpus with no refusals in it), which is not a pass.
set -eu

cd "$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"

MAXLEN=${MAXLEN:-4}
export MAXLEN

command -v python3 >/dev/null 2>&1 || {
  echo "verify-bd-argv-gate: no python3; nothing measured" >&2
  exit 2
}

# -B: importing the parser below must not leave a __pycache__ in scripts/.
python3 -B - <<'PY'
import importlib.util, itertools, json, os, subprocess, sys

sys.dont_write_bytecode = True          # belt to -B's braces

WRAP = "scripts/bd-argv-gate.sh"
PARS = "scripts/bd-argv-gate.py"

# The parser is imported for the sweep — forking it 10k times would make this
# a coffee break — but every DISAGREEMENT is then re-checked by running both
# programs for real, so nothing is reported on the strength of the import.
spec = importlib.util.spec_from_file_location("gate", PARS)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

# The newline is load-bearing, not decoration: it is the one character the
# harness cannot pass through literally, so it is the one the wrapper has to
# reason about encoded (ranger-base-1lvm). Removing it from this alphabet
# re-opens the blind spot that hid that bug from every corpus.
ALPHABET = "bd\\'\" \n"
MAXLEN = int(os.environ["MAXLEN"])
# Prefixes exercise the parser's other resolution paths: a path spelling
# (basename), a wrapper word, an assignment, and the segment splitter. The
# last one puts bd on the SECOND line of an otherwise ordinary command.
PREFIXES = ["", "/usr/local/bin/", "env ", "A=1 ", "cd /tmp && ", "echo hi | ",
            "echo hi\n"]
TAIL = " daemon stop"

# A parser that cannot be started, so the wrapper has to fall back on its own
# text test. `python3 <missing file>` exits 2, which is the mode the wrapper
# must NOT read as a refusal — it recognizes the parser's refusals by the
# marker on stderr, not by the exit code.
BROKEN = dict(os.environ, BD_ARGV_GATE_PY=os.path.join(os.getcwd(), "no-such-parser.py"))


def payload(cmd):
    return json.dumps({"session_id": "verify", "tool_name": "Bash",
                       "tool_input": {"command": cmd}})


def _says(raw, env):
    p = subprocess.run(["sh", WRAP], input=raw, capture_output=True, text=True,
                       env=env)
    if p.returncode == 2:
        return "refuse"
    if p.returncode != 0:
        return "rc%d" % p.returncode
    return "refuse" if p.stdout.strip() else "silent"


def wrapper_says(raw):
    return _says(raw, None)


def fallback_says(raw):
    """What the wrapper answers with the parser unavailable."""
    return _says(raw, BROKEN)


def parser_says_forked(raw):
    p = subprocess.run([sys.executable, "-S", "-E", PARS], input=raw,
                       capture_output=True, text=True)
    if p.returncode == 2:
        return "refuse"
    if p.returncode != 0:
        return "rc%d" % p.returncode
    return "refuse" if p.stdout.strip() else "silent"


tried = refusals = 0
escapes = []                            # fast path answered a parser refusal
fallback_escapes = []                   # fail-closed fallback waved one through
for prefix in PREFIXES:
    for n in range(1, MAXLEN + 1):
        for tup in itertools.product(ALPHABET, repeat=n):
            cmd = prefix + "".join(tup) + TAIL
            tried += 1
            try:
                refused = gate.verdict(cmd) is not None
            except Exception:
                refused = True          # the parser fails closed on its own bugs
            if not refused:
                continue
            refusals += 1
            raw = payload(cmd)
            if wrapper_says(raw) != "refuse":
                # Confirm against the real parser process, not the import.
                if parser_says_forked(raw) == "refuse":
                    escapes.append(cmd)
            # Obligation two: with the parser unavailable the wrapper answers
            # out of its own text test, and that test must not be looser than
            # the parser either.
            if fallback_says(raw) != "refuse":
                if parser_says_forked(raw) == "refuse":
                    fallback_escapes.append(cmd)

print("verify-bd-argv-gate: %d spellings tried, %d refused by the parser, "
      "%d waved through by the fast path, %d waved through by the fail-closed "
      "fallback" % (tried, refusals, len(escapes), len(fallback_escapes)))

# A sweep that found nothing to refuse measured nothing — the same "pass" a
# broken import would produce.
if refusals == 0:
    print("verify-bd-argv-gate: the parser refused NOTHING; the sweep is not "
          "discriminating (broken parser? empty alphabet?)", file=sys.stderr)
    sys.exit(2)

# Likewise, a fallback sweep that never actually reached the fallback proves
# nothing: if BROKEN stopped being broken, every row would agree for the wrong
# reason. One control, and it must come back refused.
control = payload("bd daemon stop")
if fallback_says(control) != "refuse" or wrapper_says(control) != "refuse":
    print("verify-bd-argv-gate: the fallback control (`bd daemon stop` with no "
          "parser) was not refused; the fallback sweep measured nothing",
          file=sys.stderr)
    sys.exit(2)
# …and the inverse control: the broken-parser env must really break it, or
# `fallback_says` is silently just `wrapper_says` a second time.
if parser_says_forked(payload("go test ./...")) != "silent":
    print("verify-bd-argv-gate: the silent control was not silent",
          file=sys.stderr)
    sys.exit(2)

for cmd in escapes[:20]:
    print("  ESCAPE  fast path waved through a parser refusal: %r" % cmd, file=sys.stderr)
if len(escapes) > 20:
    print("  ... and %d more" % (len(escapes) - 20), file=sys.stderr)
for cmd in fallback_escapes[:20]:
    print("  ESCAPE  fallback waved through a parser refusal: %r" % cmd, file=sys.stderr)
if len(fallback_escapes) > 20:
    print("  ... and %d more" % (len(fallback_escapes) - 20), file=sys.stderr)
sys.exit(1 if escapes or fallback_escapes else 0)
PY
