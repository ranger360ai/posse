# A derivation total over its readers was not total over its inputs, and two wall-clock pins became call counts (ranger-base-ghcx3)

2026-10-04. Four findings rode out of ranger-base-0hjrj's verify of four
closes. Three were live escapes, one was main's own suite. This file is what
each one turned out to be and the shape each fix took, because three of the
four are instances of classes this shop has filed before and the fourth was
already dead by the time it was read.

Environment for everything below: this worktree, go1.26.5, darwin 25.4.0,
`go test -count=1` filtered runs at main 093cfb1f. Every mutant was applied
in-tree, restored from a golden copy and md5-checked back, with
`git status --porcelain` read before and after.

## 1. A derivation is total over its READERS and nothing checked its INPUT

`TestSeedConfigDocumentedDurationDefaultsAreTheConstants` derives key/constant
pairings out of the tree's AST, and ranger-base-khqvr made its three hand
tables total from both ends — a reader, a constant or an unpairable key the
tree has and a table does not reds it by name. That half holds; nine mutants
died against it.

The census was still built in one direction: **walk the READERS, then ask the
seed about each key you found.** Nothing walked the seed. A documented duration
key whose reader matches neither derived shape was never compared, never
registered and never reported — `compared` stayed at 10, the floor passed, and
a fresh instance's spec could name a default by any factor in silence.

MEASURED, with the probe the verify lane wrote. `internal/posse/zzprobeseed.go`
held a THIRD reader shape: a method on `*App` with its own key literal and its
own `Default`-shaped constant like a body-form reader, plus one extra parameter
so `seedDurationReaderDecl`'s `(io.Writer) time.Duration` test rejects it. The
key was documented in `examples/config.yaml` at ten times its real default.

| tree | the three seed pins |
|---|---|
| the three-shape probe, `# probe_seed_ttl: 10h` | **SURVIVED**, ok 0.53s |
| the same reader one parameter closer to the body form | KILLED, two messages |
| the three-shape probe with NO documented line | green, correctly |
| after the fix: the three-shape probe, documented | **KILLED**, names file, line and value |

The fix is the direction that hurts: `seedDocumentedDurations` enumerates the
seed's own commented `key: <duration>` lines, and each one must be accounted
for in exactly one of three ways — compared, in
`seedDurationDefaultNotAConstant`, or in a new third register
`seedDocumentedDurationOutsideTheCensus`, whose two members
(`autostart_interval`, `autostart_max_interval`) were already named in the head
comment's prose and in nothing a machine reads.

**That turns a count into a set, which is the whole point.** Deleting a
documented line reds at `compared 9`; adding one is invisible to any count, and
adding one was how all six previous keys got in.

Two things the fix needed that are worth carrying forward:

- **The two line matchers have to be held against each other.** The
  enumeration and `seedDocumentedValue` both decide what a documented line is.
  Rather than a count as a witness, every key the pin COMPARED must be one the
  enumeration FOUND — derived, no magic number, and red the moment the two
  parsers drift.
- **The discriminator's limit is stated, not papered over.** `attnAge` accepts
  bare SECONDS, so `attn_parked_age: 1209600` is a documented duration — and
  nothing in such a line's text distinguishes it from `load_guard: 25` (a load
  average) or `grok_guard_week: 85` (a percentage). MEASURED: the seed has ten
  commented bare-number lines and not one is a duration, so reading them as
  durations would mean ten register rows saying "this is a percentage". The
  rule is therefore "a value with a unit", the uncovered case is a new duration
  key documented in bare seconds, and `seedLooksLikeDuration`'s comment says so
  where the next reader will meet it.

## 2. An arm's header claimed a refusal its body never reached

`meteredcred_qa_test.go` arm 4's header read "the write path still refuses it,
by name and by shape". The body called `checkSessionToken` twice — the SHAPE —
and nothing else. The NAME refusal is a different line in a different
function, `refreshSession` (refresh.go), and nothing in the tree held it.

MEASURED, `if IsMeteredCredentialName(key)` → `if false && …`:

| run | result |
|---|---|
| `-run '^TestQAMetered'` | SURVIVED |
| `-run '(Cage\|Cred\|Refresh\|Metered\|Launch\|Wrap\|Mint\|Token\|Session\|Runtime\|Gate)'` | SURVIVED, 61.2s |
| the UNFILTERED package | SURVIVED, 257.9s |

The arm now drives `refreshSession` itself, through the one route a metered
name has into this shop — a `cage_cred:` line in a promoted runtime profile,
which is the same route arm 2 uses. **It asserts the WRITE sentence** ("does
not write it") and refuses the UNDECIDED one by name, because an undecided
runtime errors two lines earlier and would pass a weaker arm for the wrong
reason. The same mutant now reds arm 4 alone.

**The general lesson is about headers.** An arm whose header claims more than
its body reaches is worse than no arm: it is the reason nobody writes the real
one. The refusal here predated the bead that pinned it; what was new was a pin
that said it was covered.

## 3. Already dead, and it was dead before it was read

The bwp7h branch carried a notes fragment that was not in `docs/notes.d/README.md`,
so main's notes door would red the moment it landed. ranger-base-eawjq owned
the landing and did it: `ca92fb64` ("a two-commit merge-back needs two twins,
and the notes index was never regenerated"). At 093cfb1f,
`python3 scripts/notes-index.py --check` is exit 0 and the README names
`ranger-base-bwp7h`. Nothing to do. Recorded because a finding that dies
between filing and reading still costs the reader a measurement, and the
cheapest place to leave the answer is here.

## 4. Two wall-clock pins became call counts, and one budget became derived

`make test` at clean main exits 2 on a loaded box: arm 3 red at 1260.6s
against 364.0s and 245.0s for the same arm, with a 15-minute load average of
27. Three failures, every one a wall-clock threshold, and green 3 of 3 at the
same commit when re-run filtered. This shop has filed the class at least seven
times (ranger-base-vstc, -fwuck, -5pkzd, -4pjw, -ehllm, -yl8j, -5fw5), every
one closed, the class still live. The two fixes are different because the two
pins are different.

**The promptready arms asserted a clock for a question about a COUNT.** Both
ask whether the gate LOOPED: "it must cost one explain", "it waits for a seen
screen, not for idle". A wall clock cannot see that, because one `agent
explain` is a fork and an exec of the test binary, and under load one of them
takes longer than the 250ms poll by itself. `AwaitPromptable` says exactly this
in its own comment — "News is a second `agent explain`, not a slow first one
(ranger-base-vstc)" — and returns a note that proves it, while the two arms
went on asserting the clock beside it. They now count `agent explain` lines in
the fake's own call log (`promptExplains`). **This substitution was already
house practice in the same package**: `fakeExplainErrorArmed`'s comment makes
it for the other direction, in the same words, over the same fake, after two
beads (-4pjw at the late end, -3wc7 at the early one). The pattern existed; the
two arms predated nobody applying it.

Mutation-checked: `case det.Seen() && attempts > 1` reds both on the count;
`case det.Seen() && det.State == "idle"` reds the idle arm alone, which is the
regression it is named for.

**The backup arm was right and its window was a guess.** `backuploop_test.go`
measures a control write, then waits a constant 2s for an absence. Under load
the control took 13.9s, so the absence arms waited a seventh of the time a
write needs — and the pin REFUSED rather than assert an absence over a window
nothing could have filled. That is a pin working. The cost is a red that names
no defect. So the window is derived from the control now
(`backupAbsenceGrace`): 4x, floored at the old 2s, capped at 30s.

Three things make this a fix and not a widening:

- **A wider absence window is a STRONGER assertion.** There is more time for an
  archive the loop must not write to appear, so scaling it up cannot make the
  arm vacuous. Confirmed: breaking the level trigger (`age < cfg.Interval` →
  `age < 0`) reds the arm at 3 archives.
- **The refusal is kept and kept REACHABLE.** The factor can never fire it (4x
  a number is past that number), so the ceiling is what fires it, at a control
  past 30s — and arm 2 gives up at 60s, so the live window for that refusal is
  a control of 30-60s. A derived budget whose assertion can no longer fail
  would have been the worse outcome; it is checked explicitly.
- **The arithmetic is pinned where the load is not.** The loaded box cannot be
  reproduced here (standing rule: never synthesize load on this box), so the
  rule is a function with a table pin at the floor, the factor, the ceiling and
  the 13.9s the incident measured — otherwise the derivation is only ever
  exercised at the floor, the single branch that behaves exactly like the
  constant it replaced.

On an idle box the control is ~116ms and the window is still 2s, so nothing
about the ordinary run changed.
