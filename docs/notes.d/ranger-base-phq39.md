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

## 5. The live verify: what was MEASURED, and the one half that needs another seat

2026-10-31, on a scratch `multiplexer: rex` instance (`instance: phq39`) with
a throwaway repo, two beads and a `worktrees:` under `$HOME`. Rex CLI
0.0.1793377904, claude 2.1.314.

**MEASURED, and each of these is a done-when of this bead:**

1. **The occupant is selected by the key alone.** `posse list` printed
   `REX SESSIONS` with the sentence under it and tagged the one foreign row
   `(rex)`; `posse gates builder` printed the same sentence, `launches here:
   claude`, and `REFUSED here … codex grok bob`. The identical home with the
   key removed reads `HERDR SESSIONS`.
2. **A grok dispatch is refused before the claim, with the herdr message.**
   `✗ rexrepo-2 grok launch refused: grok does not report its own state on
   Rex, and this instance runs `multiplexer: rex`` … `the way to run grok is
   herdr`. The busy-key arm fired beside it (`grokker-rexrepo skipped for the
   rest of this pass`). **Nothing was spent**: the bead read back `OPEN` with
   no owner, no worktree was cut, no meta file was written, and `rex ls` still
   held only the one pre-existing foreign session.
3. **A Rex seat is created and the runtime's line is typed into it.** Two
   launches: `rex new` opened a session with one shell block, the launch
   script was sourced in it, and `posse peek` read the seat's screen back
   through `rex capture`. So Create, Start and Read all work against a real
   server from the dispatch path.
4. **The first-report gate refuses with nothing typed.** Both launches ended
   `claude launch refused in builder-rexrepo-rexrepo-1: no program_status
   record from the runtime within 45s (saw: no program_status record)` …
   `NOTHING WAS TYPED into it: this runtime is `typed``. The bead stayed
   untouched, and the session was left running — which is the clause that
   made the cause findable at all.
5. **`posse kill` destroys the Rex session.** After each kill the session was
   gone from `rex ls` and the worktree and branch were retired
   (`had no commits — worktree and posse/… removed`).

**The half that is NOT measured, and why it needs a different seat.** claude
reading `idle`, taking the prompt, reading `working` then settling, closing
the bead and posse landing the branch. Filed as ranger-base-c33j6 with the
recipe.

> **MEASURED 2026-10-31 under ranger-base-c33j6, and the diagnosis below is
> wrong.** The positive pass ran from an ordinary gated crew seat — bead
> closed, branch fast-forwarded, Rex session retired — so no seat outside
> this box's wall was needed. "Not logged in" was a CHILD shell's view, not
> the seat's: posse injects the session mint and claude does not pass it to
> the shells it spawns, so a working seat answers that too (ADR 0042 D1
> working as designed, and `posse gates` prints no credential refusal at
> all). What actually refused the launch was the trust-file defect this
> section files three paragraphs down as "one narrow defect found and not
> fixed" — the seat sat on claude's trust dialog and so wrote no
> program_status record. docs/notes.d/ranger-base-c33j6.md has the reading.
> Leave the paragraphs below as they stand; they are the record of what was
> believed, and the correction is here rather than in them.

The cause is not Rex and it is not new. It is ranger-base-eupf's finding,
already MEASURED and already written down in gates.go: **the launching
persona's gates dir leads its PATH and the child inherits it**, so a claude
hired from inside a gated seat reads its own OAuth credential through that
seat's `Bash(security:*)` refusal shim. MEASURED here, in a shell carrying
exactly the PATH the seat inherits: `claude config list` answers `Not logged
in · Please run /login`, and the shim it hit says so in its own words — "this
shell is dinesh's pane … a persona has no way past that. operator: run
security in a terminal outside posse."

posse **refused that launch**, by name, before anything was created:
`refused: claude at cage shims, tier standard does not realize every gate —
PATH carries another persona's gates dir … relaunch with a clean PATH`.
`--allow-degraded` overrode it, and the override is what produced the
unreadable seat the first-report gate then correctly refused. Stripping the
gates dir out of PATH would be one seat removing its own L1 wall, so it was
not done. That is the whole of the gap: **this feature's own gates all fired
correctly, and the box's wall is what the positive pass needs a seat outside
of.**

**One narrow defect found and not fixed** (on ranger-base-c33j6, to be
promoted to its own bug bead if it reproduces from a clean shell): posse
seeds claude directory trust into `ClaudeConfigFile()`, which honours the
LAUNCHER's `CLAUDE_CONFIG_DIR`, while the seat's claude — started from a
multiplexer server's own environment — reads the default `~/.claude.json`.
MEASURED: `projects["<repo root>"].hasTrustDialogAccepted` landed in
`$CLAUDE_CONFIG_DIR/.claude.json` and was absent from `~/.claude.json`, which
is the file the seat read, and the seat opened on the trust dialog with
nothing saying why. The key itself was right — `scripts/claude-trust-key.sh`
over that worktree answers the repo root, which is what posse wrote — so this
is the FILE and not the key. Not Rex-specific: a herdr pane is a child of the
herdr server the same way.

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
