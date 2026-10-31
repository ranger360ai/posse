# Selecting Rex: one instance key, two gates, and what the live run measured (ranger-base-phq39)

2026-10-31. ADR 0076 §6's `multiplexer: rex`, §3's launch-stage first-report
check, and amendment D1/D2's admission split, over the occupant
ranger-base-qlxb5 built. Everything below is against **Rex CLI
0.0.1793377904 (API revision 8), Rex Beta 0.1.0, claude 2.1.314, this box**.

ranger-base-qlxb5 §7 named exactly what it left: "nothing selects this
occupant yet — `HerdrBackend.Slot` is the injection point and it stays nil".
This fills it, and the three things that took a decision rather than a read
are below.

## 1. The key is read in one place, and the default is a NIL slot

`App.Multiplexer()` validates (`herdr` | `rex`, empty = herdr) and
`App.MultiplexerName()` is its quiet half for the read paths. The asymmetry
is `App.Instance()`'s, verbatim and for its reason: a malformed value reads
as herdr so a listing cannot die of a config typo, and `planLaunch` refuses
it so no home can ever CREATE a seat under a name the read paths ignore.

`NewHerdrBackend` fills `Slot` **only for a non-herdr occupant**. That is not
a micro-optimisation — it is the mechanical form of §6's "the default stays
herdr and herdr's code paths do not change". A herdr home leaves the field
nil, `Mux()` builds `HerdrMux` over the backend's own `H`, and that is
byte-identical to what every backend did before the key existed, including
every test backend that sets `H` and nothing else. Setting `Slot` to a
`HerdrMux` built from a second `NewHerdr()` would have been the same
behaviour by accident and a divergence waiting to happen: two clients where
there was one.

What the occupant mismatch already cost nothing to handle: a meta written
`mux: rex` that a later herdr-reading pass meets is refused by
`cannotAnswerFor`'s first arm (ADR 0076 §5) — "the meta was written by rex
and this pass is talking to herdr" — so a key typo cannot make a prune act
on a seat it cannot see. That arm landed with qlxb5; this bead is the first
configuration that can reach it.

## 2. Admission is `ReportsOnRex`, and the field is built-in only on purpose

ADR 0076's amendment D1 named the bool; the decision here is that it is not
a yaml key, and the reason is not symmetry with `DetectionFiling`:

Every other admission fact on `Runtime` is an **instance** fact. `detection:
reported` is "whether THIS box installed a reporter"; `model_strong:` is
"what THIS account serves". Those overlay onto a built-in because the box is
what changed. Whether a CLI emits OSC 7501 is a property of the **binary**,
and the only way to learn it is to launch that binary in Rex and read the
record back. A yaml key would let a profile claim it without that
measurement — and the cost of a wrong claim is precisely what D1 exists to
remove: a claim, a worktree, a Rex session and (on an argv runtime) a billed
turn spent on a seat whose every state is `unknown`.

`runtimes/*.yaml` already refuses to be silent about this: a yaml that spells
`reports_on_rex: true` draws the unknown-key warning with the known-key list
beside it (pinned, `TestQANoRuntimeYamlCanClaimItReportsOnRex`), so an author
who tries it is told the declaration never arrived rather than left believing
it did.

`true` on claude alone. The three `false`s are three measurements (ADR 0076's
Spike and Amendment), not three omissions, and D3/D4/D5 say what each would
take to flip.

## 3. The two gates are in different functions because they answer at
different instants

| gate | where | when it can be answered | what a miss costs |
|---|---|---|---|
| admission (D1) | `launchSession` (above the claim) + `planLaunch` (backstop) | before anything is spent — it is a table read | refusal, naming herdr |
| confirmation (D2) | `startPlanned`, after `mux.Start` | only after the runtime's line has been typed — the writer does not exist until then | `prompt: typed` → refusal with nothing typed · `prompt: argv` → HOLD |

`confirmReports` is in `startPlanned` and nowhere else, and the door is the
type assertion `rexMux()` — §4's assertion pointed the other way. A herdr
launch therefore pays **nothing**: no extra call, no extra second, no line
(pinned). An interface method would have made both occupants pretend to
answer a question only one of them has.

Three decisions inside it, each costing a line:

- **The bound is `rt.Wait()`** (`startup_wait:`, 45 s by default) and not a
  number coined here. It is the same patience the typed ladder already gives
  a composer to appear, it is per runtime and MEASURED wherever anybody has
  measured it, and a box slow enough to need longer needs longer for both.
- **A rex call that FAILED is ignorance, never a refusal.** This is why
  `AwaitFirstReport` returns `(reading, reported, error)` and not Wait's
  typed timeout: Wait's callers branch on an API code and cannot tell "the
  bound ran out with nothing to read" from "rex would not answer", and those
  are the refusal and the unknown respectively (ADR 0013 §1). The gate stands
  down with a line, because a gate that stands down silently is a gate nobody
  knows was skipped.
- **The refused session is LEFT RUNNING**, which is the one place the message
  departs from `DetectionRefusal` ("it never says check the session: there is
  none to check"). Here there is one, because the typing is what the report
  would have been evidence of — and it is the only copy of why no record
  came. The line says `posse peek <name>`, then `posse kill <name>`.

## 4. What the surfaces say

`posse list`'s header has said `HERDR SESSIONS` since before the slot
existed; it now says the occupant's name, so a herdr home prints that exact
line and a Rex home prints `REX SESSIONS` plus one sentence. `posse gates
<persona>` prints the same sentence above the parity table — and on Rex, the
two runtime lists, admitted and refused, because on that instance those lists
decide which rows of the parity table can launch at all. `posse new` says
`rex session` instead of `herdr workspace`. Every herdr-home line is
unchanged to the byte, which is the rule §6 sets for all of this.

## 5. The live verify (MEASURED)

See the bead's closing comment for the transcript. In summary, on a scratch
`multiplexer: rex` instance on this box:

- a `claude` seat launched, its first `program_status` record read `idle`
  with `owner={pid,"claude"}` and `app=claude-code`, it took the dispatch
  prompt, read `working` and then settled;
- the bead was closed by the seat and `posse` landed the branch
  (fast-forward) and retired the tree;
- the Rex session was gone from `List()` after the kill;
- a `grok` dispatch on the same instance was refused with the herdr message,
  with no bead claimed, no worktree cut and no Rex session created.

## Pins

`internal/posse/muxslot_qa_test.go` (arm 1), over the fake rex of
`internal/posse/fakerex_test.go`. MUTATION-CHECKED, each against the line it
is about:

| mutation | reds |
|---|---|
| `Multiplexer()` falls back to herdr on an unknown name | the third-name arm |
| `NewHerdrBackend` always fills `Slot` | the nil-default arm |
| `ReportsOnRex` set on codex or grok too | the admission table |
| `reports_on_rex` added as a yaml key | the no-yaml-can-claim-it arm |
| `confirmReports` refuses the argv arm | the HOLD arm |
| `confirmReports` refuses on a rex call error | the unknown-is-not-no arm |
| `confirmReports` refuses an interactive launch | the `posse new` warn arm |
| `AwaitFirstReport` accepts the shell's own record | the shell arm |
| the D1 refusal moved below the claim in `launchSession` | the no-`--claim` arm |
| the D1 refusal wrapped in `sessionFailure` | the busy-key arm |
