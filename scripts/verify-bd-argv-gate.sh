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
# The Go pins in bdargvgate_qa_test.go enumerate the spellings someone thought
# of. This walks the whole alphabet instead: every command word that can be
# spelled from {b, d, backslash, single quote, double quote, space} up to
# MAXLEN characters, each run through BOTH programs. Any command the parser
# refuses and the wrapper does not is an escape, and is printed.
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

python3 - <<'PY'
import importlib.util, itertools, json, os, subprocess, sys

WRAP = "scripts/bd-argv-gate.sh"
PARS = "scripts/bd-argv-gate.py"

# The parser is imported for the sweep — forking it 10k times would make this
# a coffee break — but every DISAGREEMENT is then re-checked by running both
# programs for real, so nothing is reported on the strength of the import.
spec = importlib.util.spec_from_file_location("gate", PARS)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

ALPHABET = "bd\\'\" "
MAXLEN = int(os.environ["MAXLEN"])
# Prefixes exercise the parser's other resolution paths: a path spelling
# (basename), a wrapper word, an assignment, and the segment splitter.
PREFIXES = ["", "/usr/local/bin/", "env ", "A=1 ", "cd /tmp && ", "echo hi | "]
TAIL = " daemon stop"


def payload(cmd):
    return json.dumps({"session_id": "verify", "tool_name": "Bash",
                       "tool_input": {"command": cmd}})


def wrapper_says(raw):
    p = subprocess.run(["sh", WRAP], input=raw, capture_output=True, text=True)
    if p.returncode == 2:
        return "refuse"
    if p.returncode != 0:
        return "rc%d" % p.returncode
    return "refuse" if p.stdout.strip() else "silent"


def parser_says_forked(raw):
    p = subprocess.run([sys.executable, "-S", "-E", PARS], input=raw,
                       capture_output=True, text=True)
    if p.returncode == 2:
        return "refuse"
    if p.returncode != 0:
        return "rc%d" % p.returncode
    return "refuse" if p.stdout.strip() else "silent"


tried = refusals = 0
escapes = []
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

print("verify-bd-argv-gate: %d spellings tried, %d refused by the parser, "
      "%d waved through by the wrapper" % (tried, refusals, len(escapes)))

# A sweep that found nothing to refuse measured nothing — the same "pass" a
# broken import would produce.
if refusals == 0:
    print("verify-bd-argv-gate: the parser refused NOTHING; the sweep is not "
          "discriminating (broken parser? empty alphabet?)", file=sys.stderr)
    sys.exit(2)

for cmd in escapes[:40]:
    print("  ESCAPE  wrapper waved through a parser refusal: %r" % cmd, file=sys.stderr)
if len(escapes) > 40:
    print("  ... and %d more" % (len(escapes) - 40), file=sys.stderr)
sys.exit(1 if escapes else 0)
PY
