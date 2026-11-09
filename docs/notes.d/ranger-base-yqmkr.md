# The box-wide cost of `program_status = "on"`, measured; the turn-end state still unbought (ranger-base-yqmkr)

2026-11-07. The shakedown slice of ADR 0076 Amendment 3 (design bead
ranger-base-1vgrw, build bead ranger-base-nhxdx). D12 admits grok on Rex
behind one line in the operator's own grok config, and that line is **box
wide**: every grok the operator runs anywhere then writes OSC 7501 to its
terminal, herdr panes included. Amendment 3's Consequences left two things
ASSUMED and one UNPROVEN, and this bead was cut to measure all three.

**Two of the three are measured here, with no model turn.** The third needs a
real billed grok turn on the operator's own account, which is spend and is
never a persona's call; it was filed as an authorization ask
(ranger-base-uzb3s) and **DECLINED by the operator on 2026-11-08** — claude is
the only runtime on the host for now (ADR 0083 D1, ADR 0080 D6), so turns
bought here would buy evidence for a runtime the host will not field. Section
4 is therefore what this slice leaves UNPROVEN by decision, not a measurement
anyone is waiting on. Nothing below cost a turn: every arm ran under its own fixture
grok home with a junk API key and every `*_PROXY` at `127.0.0.1:9`, so each
turn died locally before it was a request (the ranger-base-1g8gf rig, as
Amendment 3's own probe uses it).

Probe: `docs/adr/0076-grok-shakedown.probe.sh`, arms `osc detect gates`.
MEASURED 2026-11-07, this box (darwin 25.6.0), **grok 1.0.52
(0920c8169e27)**, **herdr 0.9.1** with grok detection manifest
**2026.10.05.1**, **Rex CLI 0.0.1793377904-ge6592f**, posse built from this
worktree.

## 1. herdr consumes the OSC unseen — ASSUMED becomes MEASURED

The claim: "A terminal that does not know an OSC is meant to consume it unseen
(ECMA-48 string controls). That herdr's emulator does so ... is ASSUMED."

The reading needed a POSITIVE half, because "no garbage on the screen" over a
pane nothing ever wrote to is a green light from a rig that could never fail.
So grok runs under `script(1)`, which keeps a copy of every byte it writes to
its terminal while passing them through to the pane, and the control arm is
the shipped `auto` default in an otherwise byte-identical fixture.

| arm | OSC 7501 strings grok wrote | rendered herdr screen |
|---|---|---|
| `program_status = "on"` | **28** (`state=idle` at launch, then `state=working` per retry, each `app=grok-build`, ST-terminated) | 0 matches of `7501` / `n,auth` / `grok-build`; 0 stray control bytes |
| `auto` (shipped default) | **0** | the same, 0 and 0 |

And the reading that settles it: **the two rendered screens are identical**
once grok's own spinner frame and its tenths-of-a-second stopwatch are
normalized — which are the only two fields that differ, and which differ
between two reads of the SAME pane. On a longer hand run earlier the same day
the two screens were identical to the byte, 1141 bytes each, zero diff lines,
after 28 OSC strings had gone into one of them.

A second finding fell out of the control arm: **under `auto`, in a herdr pane,
grok writes no `7501` bytes at all** — not even the `7501;?` feature query
whose stalled reply is the `n,auth` composer leak on Rex. So the leak
Amendment 3 fixes is Rex-specific, and on herdr the line buys nothing and
costs nothing. That is the honest shape of "box wide" here: 28 OSC strings
that herdr swallows.

## 2. herdr's own grok detection does not move

The other half of the same ASSUMED: "that herdr's own grok detection still
settles". Measured on plain grok panes — NOT the `script(1)` ones, because
herdr identifies an agent by the pane's foreground `argv0` and a wrapper makes
the pane `script`, at which point `agent explain` answers `agent_not_found`
and there is no reading at all. Two questions, two panes.

| reading | `on` | `auto` |
|---|---|---|
| mid-turn | `working`, matched `spinner_status_working` | the same |
| after one Ctrl-C, nothing queued | `idle`, matched `osc_title_idle` | the same |
| `osc_progress` region | `4;1;-1` then `4;0;0` | the same |
| `osc_title` region | grok's own title line | the same |
| manifest | 2026.10.05.1 | the same |

The rule evaluation is identical arm for arm at every reading, and the
`osc_progress` region holds `4;1;-1` — that is OSC 9;4, grok's progress
report, which herdr has always read. **The 7501 OSC enters neither detection
region.** herdr's settled state here is `idle` from a MATCHED rule
(`osc_title_idle`), which is positive evidence and not the idle-fallback guess
ADR 0013 §1 refuses; `done` on herdr comes from the agent-lifecycle report a
posse launch makes, which is the part section 4 cannot reach without a turn.

So: if (1) had shown garbage this bead was to stop and hand the design back.
It did not. The gate is clear in both halves.

## 3. The D12 admission gate, read on a real box

Five fixture grok homes through the real binary on a scratch
`multiplexer: rex` instance, read through `posse gates` — which launches
nothing, claims nothing and creates nothing. `GROK_HOME` is the HOME pin of
ADR 0077 D1, the same value posse types onto a seat it hires, so the file
posse reads here is the file that seat's grok would open.

| the home's key | `launches here` | the refusal's reading |
|---|---|---|
| absent | claude codex | `no <path>/config.toml at all` |
| `auto` | claude codex | `ui.notifications.program_status = "auto" in <path>` |
| `off` | claude codex | `ui.notifications.program_status = "off" in <path>` |
| unreadable (mode 000) | claude codex | `cannot read <path> (permission denied)` + the cage note |
| `on` | **claude codex grok** | — |

All four refusals name the line to add and say posse never writes it. The
unreadable arm carries ranger-base-m928w's cage diagnosis — it says the
refused read is a fact about THIS SEAT and not about the box, and to ask again
from outside the cage — which is the right answer from a `cage: seatbelt`
reader and is the arm a section-blind or errno-blind reader would have got
wrong. The `on` arm also passes the D13 version pin, since this box's grok
really is 1.0.52 (0920c8169e27).

**What this arm is NOT**: the dispatch-site refusal. The bead asked for the
refusal through a live `dispatch` pass, and the rig for it was built and ready
— a scratch no-db beads store holding one rig bead routed by ASSIGNEE to a
seat whose `labels:` match nothing, so only an explicit assignee could route
there (`laneFor`: an assignee that canon-resolves is a lane of one), and
`--dry-run` confirmed the routing. The pass itself was refused by this
harness's own classifier as interference with live workloads, and that refusal
was not worked around. What the live pass would have added over the table
above is the PLACEMENT, and the placement is already pinned and
mutation-checked under ranger-base-nhxdx
(`TestQADispatchRefusesGrokOnRexForTheProgramStatusPreconditionBeforeTheClaim`
reds, with the refused launch running `rex new` and `rex send`, when the arm
is disabled) — the same reading, the same function, the same message text, one
call site over. Both readings are the same `grokProgramStatusRead`.

## 4. What is NOT measured, and what it would cost

Amendment 3's UNPROVEN, verbatim: "How a turn ENDS, `done` or `error`, is in
grok's docs ... but UNPROVEN live: the rig cannot finish a turn without a
model. ... Settling does, so the slice's live shakedown (one real turn,
operator-run) is where it is measured."

That is still true, and it is the whole of what this bead did not deliver:

- **the turn-end state on Rex** — the record reading `done` (or `error`) after
  a turn that actually completes. Every arm of every probe so far has ended a
  turn by killing it, cancelling it, or letting it die at a dead proxy, and a
  dead turn stays `working` through 8 min 20 s of retries (Amendment 3,
  retry 9 of 15). This is the one state no rig reaches.
- **a dispatched grok seat that SETTLES**, on either multiplexer — posse
  claiming a bead, reading the settle off the record (Rex) or off herdr's
  detection, and the seat closing its own bead.
- **`posse kill` clearing the Rex session** afterwards, which is cheap but
  only exists after a seat does.

Each needs a real billed grok turn on the operator's own account. Filed as an
authorization ask on the codex precedent (ranger-base-x7ojx, where the
operator authorized two codex turns by name and by count), rather than
guessing.

**The answer was DECLINE, operator direction 2026-11-08 (ranger-base-uzb3s):
no billed grok turns, because claude is the only runtime on the host for now
(ADR 0083 D1's last bullet, on ADR 0080 D6), and ranger-base-yqmkr closes on
what is built and probed with the live grok settle recorded as unproven.**
That was named in the ask as the cheap answer and it is: the two ASSUMEDs
above are measured either way, and what stays open is the turn-end state,
which Amendment 3 already records as UNPROVEN and which admission does not
rest on — D2's argv arm accepts a first record of any state. What it costs is
real and bounded: a posse settle on Rex would be reading a grok turn-end state
no one has seen, so the first seat to try it is the one that finds out. The
decline is "for now" and names its own revisit — if grok is admitted on the
host, the three bullets above are the shopping list, and the probe
(`docs/adr/0076-grok-shakedown.probe.sh`) is the rig that gets everything else
back for free.

## 5. Two rig findings, both of which faked a result first

**Pointing only `HERDR_SOCKET_PATH` at a new path is not isolation, and it
looks exactly like it is.** A `herdr server` started that way comes up on the
new socket and answers `herdr status` as `running` — and serves the DEFAULT
session's state. The "scratch" socket's `workspace list` returned the fleet's
fifteen live workspaces, every one of them a live crew seat, where a
`workspace close` would have closed somebody's session. herdr keys session
state on HOME, not on the socket: `HOME=<scratch>` is what forks it, and the
socket path only has to stay short enough to bind (sockaddr_un caps it near
104 bytes, so a scratchpad path is already too long). `--session <name>` is
the documented spelling and is unusable from a caged seat — its state dir is
under the operator's own herdr config dir, which the seatbelt refuses with
`Operation not permitted`. The probe now asserts the fresh server answers
with ZERO workspaces and refuses to run otherwise.

**The first workspace a fresh herdr server creates gets a pty one column
narrower than every later one.** MEASURED with `stty size` in three panes and
no grok anywhere near it: 119 columns, then 120, then 120. Whichever arm runs
first therefore renders every box-drawing row one character short, the two
screens differ on every bordered line, and the diff reads exactly like the
stray bytes this probe exists to look for — which is how the first run of the
`osc` arm reported a difference that had nothing to do with the key. The probe
burns a warm-up pane on nothing before either arm. The earlier hand run was
byte-identical for the same reason without knowing it: neither of its panes
was that server's first.

And one probe-authoring trap worth the line: `python3 -I - ARGS <<'PY'` cannot
also read piped JSON. The heredoc IS stdin, so the program text is what
`json.load(sys.stdin)` gets, and every arm dies on an empty read with a
traceback that looks like the tool returned nothing. Pass the reading as a
file path in argv.

## Measured versus assumed

MEASURED (this box, 2026-11-07, versions above): every row of sections 1, 2
and 3, and both rig findings in section 5. ASSUMED: nothing new — the two
ASSUMEDs this bead was cut to measure are measured. UNPROVEN: the turn-end
state and a dispatched settle, section 4 — not pending, but DECLINED
2026-11-08 and left unproven by that decision; grok's `blocked` live,
unchanged from Amendment 3.
