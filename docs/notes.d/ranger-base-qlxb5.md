# Rex in the multiplexer slot: four decisions ADR 0076 §2 left to the occupant (ranger-base-qlxb5)

2026-10-31. The ten verbs of ADR 0076 §2 over the rex CLI, with agent state
from OSC 7501 (§3). Everything below is measured against **Rex CLI
0.0.1793377904-ge6592f (API revision 8), Rex Beta 0.1.0, this box**, by hand
and with throwaway sessions that were killed afterwards.

Four things were decided here rather than read off the ADR, because §2's
table is one column wide per verb and each of these is a property the table
has no room for. Three of them are measurements that change what the code
has to do.

## 1. Create opens a SHELL, not the runtime's line — and that closes the ordering question

§2's Create row reads "session.create with a terminal block `{command,
cwd}`", and it was written when Create and Start were one verb. They are not
(ranger-base-csnzn §1 split them), and two call sites are why:

- a launch types the runtime's line only AFTER the meta that records the
  session is on disk, because a failed meta write must not leave a runtime
  running that posse has no record of (`mustNotOrphan`, rangerhq-v52t);
- `RestartAgent` types a line into a shell that settled hours ago.

ranger-base-csnzn flagged that as unresolved and ranger-base-q5gc2 §5 left it
to this bead: "on Rex the meta cannot be written between the create and the
launch the way `startPlanned` writes it today". **It can.** Rex's `command`
option is optional — "A program and its arguments, to run instead of an
interactive shell" — so the block opens as an interactive shell exactly the
way herdr's pane does, `Start` types into it, and neither call site moves.
There is no ordering problem on this occupant; there was one only for a Create
that had to carry the command.

MEASURED: `rex new L --cwd /tmp -- /usr/bin/env K=V zsh -i` gives a block
whose `process.child` and `process.foreground` are both that interactive zsh,
with `K` in its environment, the cwd honoured, and `PATH` from the login
profile.

## 2. The env rides the SHELL's argv, and `initial_input` is refused

Rex's blocks take no env (ADR 0076, MEASURED), so §2 says to put it in the
argv as `env K=V …`. The question §2 does not answer is *whose* argv, and the
answer is the shell's and not the typed line's.

On herdr the env is set on the WORKSPACE, so every line typed into that pane
inherits it — including `RestartAgent`'s, which re-reads the env sets by name
but types only the line (`herdrback.go`). Env prefixed onto each typed line
would therefore give a relaunched seat no credentials at all: a crashed CLI
coming back unauthenticated, on the unattended path.

So the block's command is `/usr/bin/env K=V … $SHELL -i`, and the exposure is
the argv — which is what `Mux.Create`'s own doc already concedes ("the same
known exposure `tmux -e` has") and what herdr's `workspace create --env K=V`
already is.

**Rex's `initial_input` was the other way to reach the same shell, and it is
rejected.** It is documented as "Text to type into the terminal once its
program starts", so `export K='V'` lines would work — and would put every
value on the block's screen, in the shell's history, and into posse's own
reading records, since a D3/D5 record carries a pane capture
(`readingslog.go`). That is a durable leak the argv route does not have:
`ps` shows a live process, a reading log is a file.

`/usr/bin/env` absolute rather than the bare name, because rex runs a block's
command through the user's login shell, so a bare `env` would resolve against
whatever PATH that profile sets. The inner shell is `$SHELL`, falling back to
`/bin/sh`.

## 3. Three readings that are not a state, and the one that bites

§3 says the runtime reports its own state and posse reads the report. What it
takes to turn a REPORT into a READING is three checks, and all three answer
`MuxUnknown` — never `MuxIdle`, which is §3's last line:

| the record | why it is not a reading | source |
|---|---|---|
| absent | nothing has reported | ADR 0076 §3 |
| owner is the block's shell | the seat's own plumbing reported, not a runtime | ADR 0076 §3 |
| owner is not a live process here | the record outlived its writer | ranger-base-ch2rb |
| owner absent | Rex "couldn't tell", or the shell was in the foreground | ADR 0076's spike |

The third is the one a reader would not think to ask for, and it is
reproducible in three commands. Rex drops only `working` and `blocked` when a
process exits, so **an `idle` record outlives its writer**. MEASURED
2026-10-31, one block, in order:

```
owner={82975,"zsh"}     a report written from the shell's own prompt;
                        child == foreground == that zsh
owner={91369,"Python"}  a report from a program the shell started;
                        foreground == that program, child == zsh,
                        ancestors == [zsh]
owner={99245,"Python"}  the same, after the writer EXITED: the record is
                        still there with its dead pid, while foreground
                        has fallen back to zsh
```

So liveness is "the owner is `process.foreground`, `process.child`, or one of
the `ancestors` between them", compared on **pid and name both** because Rex
says to in those words ("Compare it as well as pid, because the system reuses
process IDs"). All three places, not just the foreground: `ancestors` is left
out entirely when foreground IS child, and a runtime that has a foreground
child of its own would read as dead against the foreground alone.

And "owner is the shell" is `owner == process.child`, which is what the shell
row above measures. That is §3's "owner must not be the pane's shell" made
mechanical, and it is the reason Create's shell is `child`: a block whose
program were the runtime itself would have no shell to compare against.

**One divergence from the protocol, stated here because it is deliberate.**
The program_status schema advises a client to "Treat a value you don't know as
'idle'". `rexMapState` maps an unrecognized state to `MuxUnknown` instead.
That advice is right for a UI badge and wrong for a dispatcher: posse's
readers branch on a settled state to judge a turn and close a bead, so calling
an unrecognized state idle would settle work on a guess. Unknown costs a kept
claim; idle costs a bead closed on a turn nobody saw.

## 4. A Rex session outlives its blocks, so `List` drops a session holding none

MEASURED 2026-10-31, and it contradicts the obvious reading of §5. §5's
liveness rule is "a seat is dead when its session id is missing from
`session.list`". It is not missing:

```
$ rex send -b <block> 'exit' && rex send-key -b <block> enter
$ rex ls
LABEL        ID
qlxb5-a      6a2047        <- still there
$ rex session inspect session:…
windows: 0                 <- and holding nothing
```

The block closes when its program exits (no `--keep-open`, deliberately — see
below) but the session stays, indefinitely, with zero windows. A `List` row
for such a session would make every finished Rex seat **immortal**: nothing to
type into, nothing to read, and a claim no prune could ever settle.

So `List` does not list a session holding no terminal block. That is not
reading absence as death, which ADR 0011 §2 forbids — the block is gone, which
is positive evidence that the seat is. The session id is still addressable for
a `Close`.

`--keep-open` is the other direction and is worse: with it, the block stays
open after its program exits and the seat never ends at all.

## 5. The smaller decisions, each with its reason

- **`--autostart=false` on every READ.** The flag defaults TRUE and starts a
  missing local server. A fresh server holds no sessions, so a read that
  autostarted one would answer "no such session" for every recorded seat — and
  §5 reads absence from the listing as death. One autostarted read could land
  and reclaim a whole fleet. Only `NewSession`, which is posse asking for a
  seat, lets the default stand.
- **`block call` and not `api call block.method`**, which is the same request
  one layer down. `api` is the verb the fake-substrate dispatcher already
  gives gh (`herdr_test.go`'s TestMain), and a second substrate answering to
  it would make the fakes overlap on argv alone.
- **`SeatInfo.Focused` is false on every Rex row**, and it is "not available"
  rather than "no": `session.list` carries no focus and neither does
  `client.list` (MEASURED — the server has no notion of which session a UI is
  pointed at; `focus` acts WITHIN one). Its one reader is the cockpit's
  marker.
- **`MuxServer.Gen` is empty**, which is §5 read literally. Rex's ids are
  UUIDs and are never reused, so there is nothing for an identity fence to
  tell apart. The server does publish an `instance_id` that changes per
  process (MEASURED) and it is deliberately not stamped: a fence posse does
  not need is a second reason a live seat can be withheld from its own name,
  which is the cost ranger-base-q5gc2 §3 measured on the label half of
  herdr's. `Addr` is `rex whoami --json`'s `endpoint`.
- **An empty `until`, like a zero timeout, gets a default the occupant
  names.** herdr passes no `--until` and lets its server decide; a poller
  with an empty set would watch the whole deadline out and report a timeout
  over a seat that settled immediately. The set is the settled triple every
  caller of the verb in this repo actually passes.
- **A turn shorter than one poll is invisible**, which is the one thing
  herdr's server-side observer has that this does not. The cost is bounded by
  what already happens to a stall: `agent_prompt_stalled`, then
  `judgeStall`'s re-read and commit check, then a kept claim. A model turn is
  seconds to minutes against a one-second interval, so the only shape this
  can really miss is an instant refusal — which the screen then shows anyway.
- **Key names pass through.** Rex refuses one it does not know — MEASURED,
  `send-key notakey` printed `invalid key "notakey"` and exited 2 — so a
  caller spelling a key herdr's way gets a refusal and not a silent no-op,
  which is the only property a translation table would have bought. (`esc`
  and `escape` both land.)
- **`Prompt` refuses a leading `/`.** ADR 0060 D5's hazard is a property of
  the CLI's composer, not of herdr: the picker eats the Enter and the text
  sits in the composer while the call reports success. On Rex there is no
  envelope and no detection to catch it with, so the refusal is the whole
  guard.
- **A blown `Wait` deadline is `HerdrAPIError{Code: "timeout"}`, and a
  `Prompt` miss is `agent_prompt_stalled`.** The TYPE's name is herdr's and
  its meaning is the slot's: `IsHerdrCode` is what `promptstall.go`,
  `relaunch.go`, `reportedagent.go` and `dispatch.go` branch on, and a Rex
  instance answering with a code of its own would hand those sites a silence
  they have no arm for. Renaming the type is the same mechanical bead as
  `HerdrBackend` → `Backend` (ADR 0076 §1). A hang is `HerdrHangError` for the
  same reason — `IsHerdrHang` is how gather's unclaim rules tell "said no"
  from "said nothing".

## 6. What it costs, so nobody measures it twice

MEASURED 2026-10-31, `rex block call … program_status`, five runs: **50, 60,
70, 60, 60 ms** wall, ~20 ms of CPU each.

- `State` is 2 calls — and 1 when the block has never reported, which is every
  block holding a plain shell and most of the blocks a `List` walks.
- `Wait` polls at `rexPollInterval = 1s`: ~12% of the wall and ~4% of one core
  per WAITING seat. ADR 0076 left the rate to the occupant; the shortest
  deadline on the verb is `Prompt`'s 5 s stall window, which still gets five
  samples, so a shorter interval buys granularity nothing asks for and
  multiplies the only cost. `RexMux.Poll` overrides it, because a Wait pin
  that watched three samples go by at the production rate would spend three
  seconds of suite wall clock.
- `List` is 1 + N + M calls for N sessions holding M terminal blocks, plus one
  more for each block that has reported anything — about 2.5 s for a
  twenty-seat instance whose seats are all reporting, against herdr's two
  calls. Rex's API has
  no listing that carries a block's status (`session.list` is
  `{session_id,label}` and nothing else, `block ls --json` refuses more than
  one session at a time — both MEASURED), so the only cheaper shape is the
  event subscriber ADR 0076 priced and rejected.

## 7. What this bead does not reach

- **Nothing selects this occupant yet.** `HerdrBackend.Slot` is the injection
  point and it stays nil: the `multiplexer: rex` instance key, the
  launch-stage first-report check and the live verify are
  ranger-base-phq39's. `App.RexBin()` reads `rex_bin:` and `RHQ_REX_BIN`
  today; the key is deliberately NOT added to `examples/config.yaml`, because
  an operator reads it beside `multiplexer:` and that line is phq39's.
- **`holdsRecorded` (relaunch.go) still reads herdr's raw listing**, so a
  relaunch on a Rex instance fails closed exactly as ranger-base-q5gc2 §5
  described. It is relaunch's AIMING question, never a liveness proof, and
  converting it needs a `List`-shaped answer rather than a verb.
- **Create's "unfocused" is ASSUMED, not measured.** `session.create` takes no
  focus argument and the CLI does not attach as a UI client, so nothing here
  asks for focus; whether the Rex APP follows a new session anyway needs eyes
  on it, which is phq39's live verify.
- **A FOREIGN session whose block runs a program directly** (not through a
  shell) has that program as `process.child`, so a record it writes reads as
  "the shell's" and the row shows no state. posse's own seats always have a
  shell as `child` (§1), so the cost is a foreign row in the cockpit.

## Pins

`internal/posse/muxrex_qa_test.go` (arm 1), over the fake rex of
`internal/posse/fakerex_test.go` (shared, because TestMain's dispatcher is).
The fake's program_status state is a QUEUE per block, one set taken per call:
a one-element queue is a constant answer and a longer one is the sequence a
poller walks, which is the only way to drive a poller from outside it.

MUTATION-CHECKED, each against the line it is about:

| mutation | reds |
|---|---|
| drop `rexReading`'s liveness arm | the-record-outlived-its-writer |
| drop `rexReading`'s shell arm | the-shell's-own-record, and List's no-runtime row |
| `rexMapState` falls back to `MuxIdle` (the protocol's advice) | the `paused` case of the mapping table |
| `List` keeps a session holding no terminal block | List's row count |
| `NewSession` passes `--autostart=false` too | reads-refuse-autostart-and-only-Create-allows-it |
