# The readings log, and the one incident class a consequence-keyed log cannot see (ranger-base-3xt9y)

Built 2026-10-04, one seat, against ADR 0066 D1 (accepted the same day,
operator ruling A on `ranger-base-our1e`). The evidence and the decision map
this implements are that spike's §3 and §5 R1.

Every number below is **MEASURED** on this box on 2026-10-04 (macOS 25.4.0,
git 2.50.1, go 1.26.5) or **ASSUMED** with the derivation shown. No model was
called, nothing was spent, no new egress.

---

## 1. What was built

| piece | where |
|---|---|
| the record, the path rule, the redaction, the appender, the in-process replay | `internal/posse/readingslog.go` |
| the data-ceiling redactor | `internal/posse/visibility.go` (`OpsPatternSet.RedactCeiling`, `OpsPattern.Redact`) |
| the evidence carried out of a composer-hold reading | `internal/posse/panework.go` (`PaneHold.Read`), `internal/posse/ghostbox.go` (`composerGhostRead`) |
| the quiet-tree clock's skip | `internal/posse/retire.go` (`lastTreeWrite` → `skipsReadingsLog`) |
| the census and the corpus export | `scripts/readings-census.py` |
| the pins | `internal/posse/readingslog_qa_test.go` |

Ten consequence sites are instrumented. Each one is a place the harness
SPENDS a reading, which is ADR 0066 D1's whole selection rule — a pass takes
hundreds of readings and the ones anybody has ever had to diagnose are the
ones that refused, handed back, commented, retired a claim or parked a seat.

| decision | site | consequence |
|---|---|---|
| D1 pane state | `gather`'s hand-back on a herdr code that is not `timeout`; `LaunchBead`'s | `hand-back` |
| D2 dialog | the same two sites, keyed `D2` when herdr's code is `agent_blocked` | `hand-back` |
| D3 unknown screen | `AwaitPromptable`'s two refusals; `awaitSettled`'s "never promptable" | `refusal` |
| D3 unknown screen | `awaitDelivered`'s "delivered but never recognized" | `hold` |
| D4 composer hold | `--resume`'s waiting skip; `gather`'s "waiting, not judged this pass" | `hold` |
| D4 composer hold | `--resume`'s ghost-box skip | `ghost-retirement` |
| D5 stall verdict | `judgeStall`, both arms that keep the claim and the one that does not | `hold` / `hand-back` |
| D6 turn delivered | `gather`'s "refused the turn" (both arms) | `refusal` |
| D6 turn delivered | `noteSettleOpen` | `settle-open` |

## 2. Where the log lives, and why not the three other places

`<repo>/.git/worktrees/<session>/posse-readings.jsonl` — inside the session
tree's own git admin dir. ADR 0066 D1 asks for three properties and that
directory is the only place on the box that has all three.

- **Not in the working tree.** `dirtyPaths` reads `git status --porcelain`,
  which counts untracked files, and six callers act on the count: ADR 0041's
  closed-dirty comment, `RemoveSessionTree`'s refusal, the retire guard, the
  land sweep, the reap guard. MEASURED: with the log written to the git dir,
  `git status --porcelain` in a tree that has just taken a reading is empty
  (pin arm 2). A log one directory over would make every close in every tree
  a closed-dirty close.
- **Not under the instance's own stores.** `a.StateDir` and `a.Home` outlive
  every session, so nothing in them is rotated with a tree, and ADR 0011's
  rule against a fifth store is the other half. `ReadingsLogPath` REFUSES
  such a path rather than falling back to one, measured against a real repo
  initialised inside the state dir.
- **"Never under $HOME" cannot mean the home directory.** `WorktreeRoot`
  refuses a worktree root outside `$HOME` on purpose — a reaped worktree
  under a live session destroys the work in it — so every session tree there
  will ever be is under the home. ASSUMED, and stated on the bead: the rule
  means the log's lifetime is the TREE's and not the INSTANCE's. The pin
  holds that spelling: in the tree's git dir, refused in the instance's
  stores, and **no tree means no log** rather than a default.
- **And it must not hold the quiet-tree clock open.** `lastTreeWrite` takes
  the newest mtime of every file in that git dir, and a reading of a settled
  seat is appended on every pass that holds it — exactly the tree a retire is
  for. Unskipped, a tree that ever took a reading could never be retired:
  the same shape as `git status`'s index refresh holding the clock open, which
  this repo has already paid for twice (ranger-base-9u5zy, -a8tqz). The walk
  skips the log by name, with a control in the pin proving the walk reaches
  that directory at all.

## 3. Replaying this weekend's incidents (the bead's measure-before-closing)

The claim to check is "each would have been captured with enough bytes to
reproduce". Reproduce means two different things, because there are two
reader families and posse owns only one of them:

- **posse's own Go rules** (D4, D6) are re-run IN PROCESS over the logged
  bytes — `Reading.ReplayHold`, `composerGhostRead`, `claudeAllotmentLimit`.
  The pin is an equality against today's rules and reds the day one changes
  meaning.
- **herdr's TOML manifest** (D1, D2, D3) is not re-implemented in Go and must
  not be: a second implementation to keep in sync by hand is the thing the
  tree-wide door register exists to refuse. What is checkable here is that
  the bytes round-trip EXACTLY, and the corpus export writes each region to
  its own file so `herdr agent explain --file <capture> --agent <name>` can be
  pointed at a case with no live pane.

MEASURED 2026-10-04, over a seven-record fixture log in the corpus shape plus
one deliberately torn line:

| incident | decision / consequence | captured | reproduced from the record |
|---|---|---|---|
| ranger-base-htafy — idle behind its own suite run | D4 / hold | yes | **yes, in process.** The footer round-trips and `ReplayHold` re-derives `Work = "1 shell, 1 monitor"` and `Waiting() == true`. |
| ranger-base-6o7wm — box drawing claude's own suggestion | D4 / ghost-retirement | yes | **yes, in process.** The ansi composer line round-trips with its SGR 2 run intact (`od -c`: `❯ ** ** \033[0m\033[2mping me when it's green\033[0m`) and `composerGhostRead` re-derives the ghost verdict. Control: the same line with `\033[2m` removed reproduces *not a ghost*. |
| ranger-base-0sa5a — a blocked rule that turns on one codepoint | D2 / hand-back | yes | **bytes only, and that is the whole of what the incident needed.** `│ Do you want to proceed? → 1. Yes  2. No, and tell Claude what to do differently │` round-trips byte for byte, `→` included. Each of 0sa5a's three misses was one codepoint, and that bead found all three by replaying six captures. |
| ranger-base-uauvn — the stall unclaim | D5 / hand-back | yes | **no bytes, by design.** This decision reads no screen (the spike's §3 D5 row), so a record here carrying a region would be carrying somebody else's reading. What it carries instead is both halves of the verdict — herdr's answer and git's commit count — because only the pair reaches an unclaim. |
| ranger-base-qcu4c — the mid-flight allotment refusal | D6 / refusal | yes | **yes, in process.** The refusal message IS the region here, because the rule is a `Contains` over three literal phrasings of a vendor's wording; `claudeAllotmentLimit` re-derives "refused" from the logged message. |
| ranger-base-9hm — the settle-open loop | D6 / settle-open | yes | the record carries which of the three turn-outcome states the reader was in (`blind` / `unobserved` / `answered`) and the prior count, which is the pair that tells a nudge from a loop. |
| **rangerhq-7ia — a footer reword flipped blocked→idle** | D1 | **no, in the case that matters** | see §4. |

Census output over that log, verbatim:

```
readings: 7 in 1 log(s) · 5 replayable · 0 redacted
  1 line(s) would not parse and were skipped

per day
  day         total  refusal  hand-back  settle-open  ghost-retirement  hold
  2026-10-02      3        0          2            0                 0     1
  2026-10-03      4        1          1            1                 1     0

per decision
  decision                                         total  refusal  hand-back  settle-open  ghost-retirement  hold
  D1  pane state (idle/working/blocked)                1        0          1            0                 0     0
  D2  dialog (is this screen a permission prompt)      1        0          1            0                 0     0
  D4  composer hold (working/typed/sent/ghost)         2        0          0            0                 1     1
  D5  stall verdict (rewait/keep/hand back)            1        0          1            0                 0     0
  D6  turn delivered                                   2        1          0            1                 0     0

regions read
       2  after_last_horizontal_rule
       2  prompt_box_body
       1  turn_outcome_message
       1  whole_recent
       1  prompt_box_ansi_line
```

`--export-corpus` over the same log wrote 7 cases and 7 region files; the
`replayable` column counts the 5 that carry bytes and lost no ceiling class,
because a record with no region bytes reproduces nothing and must not be
counted as if it did.

## 4. The finding: a consequence-keyed log cannot see a false IDLE

ADR 0066 D1's five consequences — refusal, hand-back, settle-open, ghost
retirement, hold — are **five ways of NOT acting**. The spike's own D1 row
names two costs of a wrong answer and they are not symmetric:

> false blocked hands a claim back; false idle types a prompt into a shell

The first is a refusal and is logged. **The second is an act, and there is no
consequence to key a record on.** Worked through for rangerhq-7ia, which is
the incident in the bead's own measure list:

- `AgentDetection.Seen()` is `Rule.ID != "" || VisibleIdle`. A footer reword
  that stops a BLOCKED rule matching leaves claude's `live_prompt_box` rule
  matching instead — a matched rule, state `idle`.
- So the reading is `Seen` and `idle`, the readiness gate opens, posse types,
  and nothing refuses, hands back, comments, retires or holds. Nothing is
  appended.
- The sub-case that IS captured is the one where the reword drops herdr all
  the way to its idle FALLBACK (`default_known_agent_idle_fallback`, no rule
  matched). That is not `Seen`, the gate refuses with `WhatHerdrSaw`, and the
  record lands as a D3 refusal carrying the reworded footer — which is the
  shape row 2 of the fixture log above exercises.

This is a property of the ADR's selection rule and not of the code, so it is
written down here rather than filed: the log is correct as specified. What
would close it is a SAMPLED record of non-consequential D1 readings — one in
N, or one per seat per pass — which changes the log's volume and therefore
the census's denominator, and is the architect's call on ADR 0066 D1 rather
than a code bead's. The number that decides it is now takeable for the first
time: the consequential rate is what this log counts, and a sample rate can
be priced against it.

**Two other stated gaps, both deliberate:**

- **The pulse's G rows are not logged** (`govern.go`'s hold, ghost and echo
  clauses). They are a RENDER of the same readings for a human, taken on
  every `posse status`, every pulse and every cockpit tick — so logging them
  would make readings-per-day a function of how often somebody looks at the
  board, which is the one thing the denominator must not be. What would log
  them honestly is the pulse's FIRE (a fingerprint change or a renag), where
  the reading is actually spent on a billed turn.
- **`ConfirmSubmitted`'s ghost arm is not logged** (`cmd/posse/main.go`'s
  hand-typed `posse prompt`). The consequence there is a line on the
  operator's own terminal with the operator standing in front of it, and the
  path has no bead and no tree in hand — resolving one would cost a
  `Sessions()`, which is posse's one destructive read.

## 5. Redaction

Region bytes are a capture of somebody's terminal, so they go through the
data ceiling on the way in (ADR 0050) and not the visibility list. The
distinction is the ceiling's own: *may this be public* against *may this
exist in a local file at all*, and the log is a local file. Scanning the
visibility list here would strip a dollar figure out of a diagnostic that
never leaves the box.

`[redacted:<class>]` names the class and never the text, which is ADR 0050
D2's class-only rule — a marker quoting what it removed would breach the
ceiling by the redactor's own hand. The classes taken ride on the record, so
a case that cannot be replayed byte for byte says so instead of looking
complete, and the census's `replayable` column excludes it.

MEASURED: with two ceiling classes configured, a footer carrying one arrives
with the hit replaced and the class named; the composer region beside it,
carrying none, arrives untouched; and an instance with no
`data_ceiling_patterns:` at all redacts nothing — an empty list is not a wall.

## 6. What this is not

No model, no network call, no new config key, no new CLI command, no
credential. ADR 0066 D1's last sentence is the scope and the code holds to
it: every verdict in a record was already made and already spent by the time
it is written down, and nothing here decides anything.
