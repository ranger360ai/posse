# A false IDLE is an act, so the record is keyed on the act and not sampled from the readings (ranger-base-o1aoi)

Architecture bead, 2026-10-04, one seat, branch
`posse/richard-posse-ranger-base-o1aoi` off `2c328504`. Rules on the gap
dinesh found building ADR 0066 D1 (`docs/notes.d/ranger-base-3xt9y.md` §4):
the five consequences the readings log keys on are five ways of NOT acting,
so a pane-state reading that says *idle* when the screen is a dialog, a
splash or a shell — the reading that TYPES — leaves no record. The bead asked
for a SAMPLED record of non-consequential readings, gated on fourteen days of
census. This note says why the gate could never have yielded the number it
asked for, what is decided instead, and what each shape costs.

Every number is **MEASURED** on this box on 2026-10-04 (macOS 25.4.0, the
fleet's `dispatch-watch.log` and `seat-cadence.log` under the state dir) or
**ASSUMED** with the derivation shown. No model, no spend, no new egress.

---

## 1. The gate as written cannot produce the number

`scripts/readings-census.py` reads **0 readings in 0 logs** today: the fleet
runs posse 0.5.0+193c8782, which predates the log (ranger-base-qk9tr found
the same), and nothing accrues until the operator's next refresh. That is
the delay the bead expected. The defect is deeper:

- A sample rate for NON-consequential readings is priced against the TOTAL
  reading rate — how many screen readings a pass takes across its seats.
- The census counts CONSEQUENTIAL readings and nothing else; that is ADR
  0066 D1's selection rule, not an omission.
- So fourteen days of census would have given the size of the log that
  exists, which is a budget to compare against, and not the denominator a
  one-in-N needs. The one-in-N shape would have been decided blind anyway,
  on a day with more data that still did not contain the number.

Recorded in ORDERS.md: when a decision is deferred on a number, check that
the instrument being waited on counts THAT number.

## 2. What a false IDLE is, in this code

A reading can only type where posse types. The sites (`AgentPrompt` and the
launch line), with the gate each one passes:

| site | gate before the keystrokes | what judges the act afterwards |
|---|---|---|
| `fire` → typed work prompt (`dispatch.go`) | `awaitSettled`: a screen herdr has SEEN, in idle/done/blocked | `agent prompt --wait`; no turn in the grace → `judgeStall` (D5) |
| `fire` → prompt on the launch line (`prompt: argv` runtimes) | none — nothing is typed; `awaitDelivered` only watches | the same wait leg, asked for directly |
| cockpit `d` on a live holder (`dispatch.go`) | the same settle gate | nothing: `wait=false`, the operator is watching |
| the pulse's nudge (`pulse.go`) | `AwaitPromptable` + idle/done re-asked of ITS detection | nothing: `wait=false`, a nudge has no turn to observe |
| relaunch's landing turn (`relaunch.go`) | `agent wait` status idle/done, not a seen-screen gate | `agent_prompt_stalled` → refuses, keeps the tree |
| `posse prompt`, cockpit `p` | `AwaitPromptable` | the operator |

So a false IDLE on the pass's own path has a downstream judgment ALREADY:
the typed text produced no turn, the grace runs out, and `judgeStall` writes
a D5 record — hold or hand-back — which the census counts. What that record
lacks is the two things that would diagnose it: **which reading opened the
gate** (the rule or chrome that said idle) and **what the screen was when
the keys went in**. D5 reads no screen by design (`promptstall.go`), and
dinesh was right not to put a screen on it as D5's own evidence.

The incident record, re-read for this class with today's gate in place:

| incident | how herdr read it | caught by today's gate? |
|---|---|---|
| rangerhq-7ia, codex hooks dialog | no rule; `default_known_agent_idle_fallback` | yes — not `Seen`, refused, D3 record with capture |
| ranger-base-3p0, claude splash race | the same fallback guess | yes — the gate exists because of it |
| rangerhq-37c, grok first-run splash | "herdr reads it idle"; the splash drew chrome | **no, if the chrome made it `VisibleIdle`** — `Seen`, typed, the prompt vanished, a second one landed |
| the reword dinesh described (§4): a blocked rule stops matching and an idle rule matches instead | a matched rule, idle | **no** — zero incidents of this exact shape in the record |

One measured incident class in seven weeks (a transient splash that counts
as seen), one described class with no incident. Both end in a stall.

## 3. The decision: capture at the act, write on the stall

The pane-state reading the gate opened on is a D1 reading, and its
consequence is the act. So:

1. `launchSession` carries the `AgentDetection` that `awaitSettled` opened
   on out to `fire`, on the typed path only (`!delivered`).
2. Immediately before `AgentPrompt`, the typed path takes one pane capture
   (`PaneReadDetection`, the fixture-shaped read 76gc4 gave D3), best
   effort, and holds it with the detection on the `pendingBead` — in
   memory, in the one watch process that will judge the prompt. Nothing is
   written yet.
3. When the act is judged: a turn started → the capture is dropped with the
   bead's other in-flight state, as a rewait is dropped today. No turn →
   `logStall`'s D5 record, both verdicts that reach it, carries a second
   evidence block: the gate's detection as D1 evidence (herdr's state, rule,
   previews, `Seen`), the capture at type time, and a capture at stall time
   (`withPaneCapture`, the same call D3 makes), the two regions named apart.
4. `scripts/readings-census.py` gains one count: D5 records whose gate
   reading was `Seen` and idle — the false-IDLE candidates — printed beside
   the D5 row, so the rate ADR 0066 D1 exists to make a number is one line.

No sixth consequence, no new record kind, no sample rate, no change to the
census's denominator: the D5 row counts what it counts today and carries
more. ADR 0066 D1's five consequences stand.

**What the two captures buy, against the measured class.** For 37c the
screen at type time is the splash with its banner, and the screen at stall
time is a composer with the text gone — the pair that says "eaten", where
either alone says "idle" or "empty". For a modal dialog read idle, the
type-time capture IS the candidate fixture (capture → fixture → rule, how
every real D3 was closed), and the stall-time one shows the text sitting in
the dialog. For a shell, the stall-time capture shows the shell's answer.

**What it does not catch, said plainly.** A false IDLE whose typed text is
ACCEPTED as a turn — keys landing in a working composer and queued, or a
dialog whose Enter confirms something and the CLI then starts a turn on the
rest — never stalls, so nothing is written. Measured incidents of that
shape: none; k99a's own wording for the pulse was "unmeasured — nobody has
caught a mangled pulse". It is the reopen condition in §5.

## 4. Prices

Denominators, MEASURED from the state dir on 2026-10-04:

| quantity | value | source |
|---|---|---|
| passes, current watch generation | 321 in ~30.6 h (armed 10-03 15:28, read 10-04 22:06) | `dispatch-watch.log` |
| typed work prompts, same generation | 75 (0 on the launch line) | `(prompted,` vs `(prompt on the launch line` |
| pulse nudges to the coordinator, same generation | 121 | `→ prompted monica` |
| seats in flight per gather, same generation | 1–6; mode 1, median 2 (136/104/92/80/44/10) | `N prompt(s) in flight, gathering` |
| launches per day, 2026-08-28 → 10-05 | 1,961 in 38 days, mean 52; 62 and 46 on 10-03/04; peak 270 on 09-07 | `seat-cadence.log` |
| typed work prompts, all time in both log generations | 658 | same greps |
| stall lines, all time, both generations | 57 (a loose grep for `stall` / `no turn started`; an upper bound on the D5 rate) | same |

Record sizes, MEASURED by 76gc4 (`readingslog.go`, `AppendReading`): six
previews and no capture 2,218 bytes; with the tallest fixture 6,297; the
worst `--source detection` shape 38,388. ASSUMED: a `pane read` costs what
an `agent explain` costs, 28.7 ms mean (ranger-base-qk9tr), since both are
one herdr round trip over one pane.

| shape | records/day (fleet) | bytes/day | herdr calls/day | catches the measured class (transient splash)? | catches a persistent dialog read idle? | needs the census number? |
|---|---|---|---|---|---|---|
| **A** do nothing | 0 new | 0 | 0 | counted as a D5, not reproducible | counted, not reproducible | no |
| **B1** one-in-N non-consequential readings | total readings / N — total readings UNKNOWN and not counted by the census | unknown | 0 | with probability 1/N | with probability → 1 over the passes it persists | yes, and the census cannot supply it |
| **B2** one per seat per pass | in-flight × passes ≈ 2 × 250 = ~500 (range 250–1,500) | ~1 MB without captures, ~5–20 MB with | 0 (the pass already read) | no — the splash is gone by the next pass | yes | no, but the denominator becomes a function of the pass interval (dinesh's §4 objection to logging the G rows) |
| **C** capture at the act, write on the stall — **decided** | 0 new; the existing D5 records grow | D5 rate × ~10–80 KB; at the 57-in-38-days upper bound, ~100 KB/day | one per typed prompt, ~75/day ≈ 2 s/day; one more per stall | **yes, both screens** | yes, type-time screen is the fixture | no |
| **D** a sixth consequence `prompted`, written on every typed prompt | ~75 dispatch + ~121 pulse | 2–15 MB; the pulse's share lands in the coordinator's checkout git dir, which no tree rotation ever removes | the same ~75 (+121 if the pulse joins) | yes | yes | no |

A over C: A's failure is measured — 37c was diagnosed by a human at the
pane, and the record it would leave today says "hand back, no commit on the
branch" and nothing else. C over D: D's extra territory is the accepted-turn
class with zero incidents, and its cost is the one log in the fleet that
never rotates (the pulse's target is the shared checkout, `LogReading`
resolves it through the meta's `Dir`, and `ReadingsLogPath` refuses only
the harness's own stores). C over B2: B2 writes the reading at the wrong
instant for the one class that has an incident, and costs the denominator
its meaning. B1 is not decidable by the instrument that exists.

## 5. What reopens it

- An incident where a prompt typed on a seen-idle reading was ACCEPTED as a
  turn on the wrong screen. C's capture is already taken at type time; the
  change is to write it instead of holding it, which is shape D with the
  pulse still excluded — one bead, no new measurement needed.
- The pulse and the relaunch landing are excluded from C by cost and by
  shape (no stall judgment to key on; the pulse's log never rotates). A
  mangled nudge or a lost landing turn with a screen to show would open
  them, each on its own bead.
- The first fourteen days of real census are still worth reading, for the
  reason the bead gave and one more: the D5 row's false-IDLE count against
  the typed-prompt count is the error rate of the idle reading, and that is
  the number ADR 0066 opened with as UNKNOWN.

## 6. Out of scope, by rule

No model, no network, no new config key, no new CLI surface, no new
consequence. The regex and TOML readers remain the readers (ADR 0066
Consequences); this adds bytes to a record that already exists and decides
nothing.
