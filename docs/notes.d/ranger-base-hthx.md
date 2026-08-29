## The fast path in front of a parser has to be looser than the parser (ranger-base-hthx)

`scripts/bd-argv-gate.sh` decides, in a shell builtin, whether to start
`scripts/bd-argv-gate.py` at all. The hook runs on every Bash call in the
fleet, so paying ~30 ms of interpreter for `go test` is a real tax, and the
fast path is how it is avoided. Its premise was written down in the file:

> A payload with no `bd` in it at all cannot produce any refusal below —
> every one of them requires bd to be named.

That is false, and the file that disproves it is the one sitting next to it.
The parser resolves the command word with `shlex` **first** and only then asks
`os.path.basename(word) == "bd"`. So every spelling the shell concatenates
into the name is a bd call whose payload carries no literal `bd` substring:

    b\d daemon stop        'b''d' daemon stop        b"d" daemon stop
    b''d daemon stop       'b'd  daemon stop         "b"d  daemon stop

The parser refuses all of them. None of them ever reached it.

**Measured against the shipped fence**, one character apart:

    bd   ship --help   ->  REFUSED
    b\d  ship --help   ->  RAN, printed the help, exit 0

`b\d mail --help` and `b\d duplicates --help` ran too — the two verbs whose
refusal reason in `WHY` names egress and bulk merges.

**What was actually exposed.** The harness's own permission matcher
normalizes `b\d` back to `bd`, so a verb *enumerated* in a PID deny set was
still refused (`b\d federation --help` was denied by the permission
pipeline). What escaped is exactly the class this gate exists for and no deny
list can close: the verbs nobody enumerated, which the allow-list posture was
the only fence for. The L1 shim is unaffected — it is a PATH executable and
never reads the typed line.

**The fix is a wider spelling test, not a stricter one.** The fast path's one
obligation is to be *looser* than the parser, so it now asks how bd can be
**spelled** in a payload rather than whether it appears in it:

    case $input in
      *bd*|*'\u'*|*b\\*|*b\'*|*b\"*) ;;
      *) exit 0 ;;
    esac

The last three arms are the necessary condition for a concatenation: the
character following the `b` of bd is either the `d` itself or the quoting
that hides it, and the shell has exactly three quoting characters. It stays a
pure builtin — no fork — which is the whole reason the fast path exists.

**Soundness measured, not argued.** Over every command word of length ≤ 6
spelled from `{b, d, \, ', "}` — 55986 of them — the parser refuses 429. The
new test sends all 429 to the parser; the old one waved 179 through. The cost
of the widening is 32 extra parser starts in 12777 real command lines
harvested from this repo, **0.25%**.

**The fail-closed fallback had the same blind spot.** With the parser
unavailable the wrapper refuses anything naming bd as a word, by grepping the
raw payload — which `b\d daemon stop` does not match either, so a broken
interpreter plus an escaped spelling was a second way through. It now greps
the payload *and* its quote-stripped form. Both, not just the stripped one:
deleting backslashes also eats JSON escapes, and `a\nbd` would collapse into
the single word `anbd` and stop matching.

**Pins.** The shell spellings are in the same refusal table as the literal
`bd` (`TestQABdArgvGateResolvesTheVerb`), together with the counterparts that
must still pass — `b\d show`, which is an escaped spelling of an *allowed*
verb, plus `b" "d` (runs `b d`) and `b\\d` (literal `b\d`) — because widening
the fast path must not widen the refusal.

`TestQABdArgvGateFastPathIsLooserThanTheParser` asserts **agreement** between
the two programs rather than "was refused": a table of expected refusals can
go green because the wrapper got stricter in some unrelated way, while
agreement can only go green if the fast path actually deferred. Every row
also carries a positive witness — the parser must genuinely refuse it — so a
fixture that stops discriminating fails instead of passing quietly.

Mutation-checked per arm, which is what the bead asked for. Reverting the
whole widening, dropping the `b\` arm, dropping the `b'` arm, dropping the
`\u` arm, dropping the literal `bd` arm, and un-stripping the fallback grep
each red the suite and name the spelling that escaped. **The fourth arm,
`*b\"*`, survives its mutation**, and that is reported rather than hidden: a
JSON string escapes a double quote, so a command's `b"` reaches the wrapper
as `b\"` and the backslash arm already covers it. The arm stays as defence
for a payload that is not JSON-escaped, and the *reason* it is redundant is
itself pinned — if the harness ever stops escaping, that assertion fails and
the arm becomes load-bearing with a witness.

**A standing control, because a table only knows the names someone thought
of.** `make verify-bd-argv-gate` (`scripts/verify-bd-argv-gate.sh`) walks the
whole quoting alphabet against both shipped programs — every command word up
to `MAXLEN` characters, under six command-word prefixes that exercise the
parser's path, wrapper, assignment and segment-splitting routes — and prints
any command the parser refuses and the wrapper does not. Disagreements found
by the imported parser are re-confirmed by running both programs for real, so
nothing is reported on the strength of the import. It exits **2** when the
sweep refused nothing at all, because a corpus with no refusals in it
measures the same "pass" a broken import would. ~23 s at the default
`MAXLEN=4`; red on the pre-fix wrapper (6 escapes at `MAXLEN=3`), green after.

**Not fixed here.** The copy that actually fences this box,
`~/.config/posse/gate/bd-argv-gate.sh`, is a manual install the operator owns
— "a PreToolUse hook the operator may install, not one posse renders"
(ADR 0015, amending ADR 0014 §5). It is byte-identical to *main*, so landing
this does not move it and the live escape stays open until one `cp`. That is
`ranger-base-b9j3`, which carries the command, the blast radius, and the two
discriminating checks that say it worked. The installed `bd-argv-gate.py` is
already identical to this branch — the parser was right the whole time and is
not touched by this fix.
