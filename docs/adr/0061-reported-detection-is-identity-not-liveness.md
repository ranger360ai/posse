# ADR 0061 — Reported detection: a runtime declares that a report-agent authority labels its panes; the label is identity, and herdr's process reading is liveness

*Status: accepted 2026-09-28 · owner: architect · source bead
ranger-base-qa73t (from ranger-base-8eqaa) · amends ADR 0013 §1's launch
row (its detection refusal, ranger-base-i3q6g) and ADR 0060 D1's "cannot
dispatch" · sits under ADR 0017 §3 (declare the dimension, never key on
the name) and ADR 0013 §2 (delivery) · measurements in Claims, recipe
inline*

## Context

herdr detects an agent from argv0 and a manifest. A pane it has no
manifest for can still be labelled by an outside authority through `herdr
pane report-agent` — herdr's documented "integrate your own agent" route,
the route every herdr plugin takes. The MartinLoeper/herdr-bob plugin
labels Bob panes that way today (ranger-base-p8afi), and ranger-base-8eqaa
built typed delivery to such a pane (`pane send-text` + Enter, chosen from
the pane's own label and herdr's manifest answer, never from a runtime
name). What a reported pane supplies, MEASURED 2026-09-28 on herdr 0.9.1
(p8afi, 8eqaa's notes fragment): a label and a lifecycle state that `agent
get`, `agent list` and `posse list` read; working/done transitions that
`agent wait --until` returns on; **not** `agent explain`
(`agent_explain_unavailable`), so `Seen()` — the positive-evidence gate
every settle and readiness read rests on — has nothing to read; **not**
`agent prompt` (`agent_not_ready`).

What still refuses is the launch. ADR 0013 §1 (i3q6g) refuses a
bead-carrying launch when herdr answers that it has no manifest for the
runtime's argv0, because such a session is `agent_not_found` and cannot be
addressed at all. That reason is false of a pane a reporter labels — it
can be addressed after its first report — and the question this record
answers is whether the label is enough to launch on, and what the launch
observes instead of `agent explain`.

Two facts measured here decide it (Claims):

1. **A reported label outlives its process.** A scratch pane reported
   `working`, ran `sleep 6`, returned to a shell prompt — and `agent get`
   still read `working`. herdr ties a reported label to the pane, not to
   the process; only the reporter (or the pane's close) clears it. The
   herdr-bob plugin releases on `pane.exited` and on nothing else. So a
   Bob that exits to its shell keeps reading `idle`, and a work prompt
   typed at that reading is typed at a shell — which *executes* every
   line of it. That is `liveness-and-identity`: the label is a pidfile.
2. **herdr already carries the liveness half.** `herdr pane process-info`
   returns the pane's `shell_pid` and its foreground processes with their
   pids, in 0.00s. At a bare prompt the only foreground process *is* the
   shell (`pid == shell_pid`); under any CLI it is not. That is liveness
   from state the kernel ties to the process's existence — the field's
   answer — read from herdr, with no argv matched and no shell list kept.

## Decision

**D1 — A runtime declares `detection: reported` when a report-agent
authority labels its panes; the declaration is an instance fact and
overlays a built-in.** Values: `herdr` (the default — a manifest must
exist, and the launch refuses by name when it does not, ADR 0013 §1
unchanged) or `reported`. `reported` requires `detection_why:` naming the
authority (the plugin id and install date is the honest form), because
that sentence is what the failure line prints when the label never comes.
Both keys join `runtimeYamlKeys()` and `builtinOverlayKeys` (ADR 0013
§8): whether *this box* has a reporter is measured here, not shipped in
the binary. **The bob built-in stays `herdr`** and keeps saying it has no
detection (ADR 0060 D1, D2's tripwire untouched); `runtimes/bob.yaml` on
an instance that installed the plugin declares `reported`. A `reported`
runtime whose argv0 herdr *does* have a manifest for is an inert
declaration: herdr's own detection wins, `runtime check` says so and
tells the operator to drop the key — that is the day herdr ships the
kind, and nothing else changes.

Present-but-wrong refuses the load (`detection: reproted`), as `prompt:`
and `record:` do.

**D2 — The launch row's observable for a `reported` runtime is a
reported label within `startup_wait`, and the launch reads one thing
first: that this herdr can carry a report.** Properties, not helper
names (v4923):

1. The i3q6g refusal (no manifest for argv0) does **not** fire for a
   runtime declaring `reported`. In its place the same shared reading —
   one function behind `runtime check`'s launch row and every
   bead-carrying launch path — asks whether herdr has the `pane
   report-agent` surface (a `--help` that exits 0; herdr 0.8.2 has none,
   0.9.0+ does, and both were on seats of this shop the same day,
   ranger-base-v1yrt). Missing → refuse by name, before anything is
   spent, with the same shape as the detection refusal: dispatched
   refuses, interactive warns, the line names argv0, the declaration and
   the door. Present → proceed, and print one line saying detection here
   is by report and whose. UNKNOWN (herdr cannot be asked) refuses
   nothing, as everywhere.
2. The observable is the label appearing: the existing wait for herdr to
   list an agent for the workspace is the launch's readiness on this
   runtime, bounded by `startup_wait`. Its failure line on a `reported`
   runtime says the runtime declared reported detection, quotes
   `detection_why`, and says no authority labelled the pane within the
   wait — never "check the session" and never "raise startup_wait" as
   the first remedy, because the first remedy is `herdr plugin list`.
3. `runtime check`'s detection gap on a `reported` runtime is a
   **non-blocking degrade**, not a refusal: every state on this runtime
   is the reporter's word, herdr reads no screen, and `blocked` is
   whatever the reporter can see (on herdr-bob today: nothing — moot for
   a dispatched seat under `--auto-approve`, real for a hand-launched
   one, p8afi). `posse runtime probe` therefore runs on it, and its
   detection observable reads through D3's door.

**D3 — One door for the reading, and a reported state is positive
evidence only while something other than the shell is in the
foreground.** Every consumer of `AgentDetection` — the dispatch settle
ladder, the delivered-wait, the promptable gate, the pulse, the
pane-holding read, the probe — takes a reported pane's reading from the
same function that reads a detected one; none of them branches, and none
names a runtime (ADR 0017 §3). Properties:

1. When `agent explain` answers `agent_explain_unavailable` and the pane
   carries a label herdr has no manifest for (the discriminator 8eqaa
   already built), the reading is the reporter's state with the label
   recorded as its evidence. `idle`, `working`, `done` and `blocked` are
   *seen*: an authority does not report a lifecycle state about a CLI it
   has not watched. `unknown`, or no state, is not seen and waits exactly
   as herdr's idle guess does.
2. **And it is seen only if the pane is live**: the reading asks herdr's
   process reading, and a pane whose every foreground process is its own
   shell is a stale label, whatever state it carries. That reading is not
   seen, and the line says so by name: the label, the state the reporter
   left, and that the foreground is the shell. This is the guard the
   detected route gets for free — herdr drops a detected label when
   argv0 leaves (MEASURED 2026-09-28, Claims fact 3: `pane get`, `agent
   list`, `agent explain` and `agent wait` all say so at once; `agent
   get` alone keeps describing the gone agent for a few seconds) — and
   the one keystroke-shaped hazard the reported route adds. It runs
   inside the reading, so it runs before every keystroke
   the promptable gate admits and inside every settle the ladder judges,
   at one `process-info` per read (MEASURED 0.00s).
3. `blocked` is handed back as given, as today. `working` waits, as
   today. The failure lines that today print herdr's working
   (`WhatHerdrSaw`) print the reporter's state and `detection_why`
   instead; there is no screen evidence to print.
4. Typed delivery is 8eqaa's route, unchanged, with its `/` refusal and
   its mirrored stall contract (`agent_prompt_stalled` within herdr's own
   5000ms hands the claim back, rangerhq-1z0). The in-place relaunch arm
   keeps ADR 0013 §1 property 3's rule on a `reported` runtime — refuse
   by name rather than type — because "no agent listed" there is still
   the reporter's silence, not the CLI's death. On a *detected* runtime
   that arm's "CLI died" read is `agent list` (`AgentTarget`), the verb
   that drops the label first; `agent get` is the one that lags, and
   nothing in posse reads liveness from `agent get` alone (Claims fact
   3). Trigger to extend it: the
   first stranded reported pane a pass meets whose foreground is the
   shell, which D3.2 can now tell apart; that is a relaunch onto a
   shell, and correct.

**D4 — Two upstream asks, posse ships neither.** (a) herdr-bob: release
the label when Bob's process leaves the pane's foreground — its watcher
already reads `process-info` to *adopt* panes, so the symmetric release
is the same read. (b) herdr: clear or scope a reported label when the
pane's foreground process group changes, and say on `agent get` that a
label was reported (today it carries no source, so posse infers "reported"
from "no manifest"). Until (a) or (b) lands, D3.2 is the guard; after
either, it is a redundant read that costs 0.00s and stays.

*Snapshot 2026-10-02 (ranger-base-5jjtn): a THIRD upstream ask, and it
outranks both of the above because it is why neither has mattered yet.
(c) herdr-bob: start the watcher on darwin, and supervise it. Its
`[[startup]]` hook runs `setsid nohup … bin/bob-watch &`; `setsid` is
util-linux and absent on darwin, so the child exits 127, `$!` records a
dead subshell, and the watcher has never run once on this box — zero
`watch started` lines in its log across the whole install, and therefore
zero adoptions. Nothing supervises or re-starts it either, so one dead
start is permanent until the herdr server restarts. Two more darwin
spellings in the same plugin are broken for the same reason: `date -Is`
(so every log line it has ever written is untimestamped) and `stat -c %Y`
in `heartbeat_fresh` (so the heartbeat always reads stale). None of this
is posse's code; all of it is measured in
`docs/notes.d/ranger-base-5jjtn.md` §2 and filed as ranger-base-mz8ud.
What it VINDICATES is this
record's own rejected alternative "name the reporter plugin in the
declaration and verify it is enabled at launch", priced `"enabled" is not
"running"`: `herdr plugin list` said ENABLED throughout, so that check
would have passed exactly when it should fail. Its named trigger — a
measured spent launch with the plugin not reporting — has fired, and the
label-within-`startup_wait` observable is still the only reading that
catches this. What posse owed was the right door, and the probe was not
giving one; that half landed under this bead.*

## Consequences

- Built: two overlayable keys and their validation; one branch in the
  shared launch reading (report surface present / absent / unknown); a
  reported arm inside the one detection reading, carrying the process
  reading; gap and failure lines; the runbook section. No new actor, no
  flag, no cache, no reporter, no runtime name in code.
- The instance side declares `detection: reported` in `runtimes/bob.yaml`
  with the plugin as `detection_why`, and its live probe measures the two
  ASSUMED items below before the first dispatched Bob close.
- The parity track (ranger-base-gz8h) reads Bob's launch row as DEGRADED
  (reported) rather than UNRECOGNIZED once the overlay exists; account
  stays UNCOUNTED (ADR 0060 D4).
- The `-p` ordering hazard (Claims, ASSUMED 2) is ADR 0060 D3's, not this
  record's: if the probe measures the typed work prompt landing before
  Bob's own `-p` turn, D3's revisit trigger has fired and the fix is in
  delivery, not detection.

## Alternatives rejected

- **A. Refuse until herdr compiles the kind** (do nothing beyond i3q6g).
  Zero build, and honest — but it makes posse dispatch only to herdr's
  compiled-in kinds while herdr's own extension model is the report, so
  ADR 0017's "switch on a dime" would have a hole the size of upstream's
  roadmap, and 8eqaa's delivery would have no dispatch caller.
- **B as filed: lift the gap when the PANE carries a label.** Unbuildable
  where the refusal runs — before any pane exists (0013 §1 property 3).
  Its second half, a reported arm in the settle wait, is D3 through one
  door rather than a second ladder.
- **C as filed: "a name-keyed mechanism".** Mischaracterized. ADR 0017
  §3's shadow predicate is `if rt.Name == …` in code; a declared key read
  generically is the remedy that section prescribes, and it is what D1
  is. What was worth refusing is shipping the declaration in the binary
  (a claim about every instance) — so it overlays.
- **Name the reporter plugin in the declaration and verify it is enabled
  at launch** (`herdr plugin list --json` exists). Priced: a third-party
  id in config, and "enabled" is not "running" — the plugin's watcher
  needs a hand start today (p8afi #3), so the reading would pass exactly
  when it should fail. The label-within-`startup_wait` observable catches
  both. Trigger: a second reported runtime, or a measured spent launch
  with the plugin disabled.
- **Liveness by matching argv against the label** (the plugin's own
  adoption regex). Fails on interpreter-launched CLIs (`node …/bob`) and
  is a second detection keyed on a name — the shape i3q6g rejected. The
  shell-pid comparison needs no name.
- **A probe record of "the last launch got labelled".** A second store of
  a fact herdr owns, stale exactly when the plugin is off.
- **Refuse on the `Blocking` bit.** Rejected by 0013 §1 property 2
  already; here it would refuse the yaml gap too.
- **posse runs the reporter.** Rejected in ADR 0060 (a daemon in
  costume); the plugin is somebody else's process and stays so.
- **Trust the reported state without the process reading.** The clever
  one — fewer calls, and every measurement before this record said the
  reporter's word was enough. Fact 1 is why not: the word outlives the
  CLI, and the failure is a shell running a work prompt.

## Claims

**MEASURED 2026-09-28, this box, herdr 0.9.1 client and server, scratch
workspace, no CLI and no plugin involved** (recipe verbatim; workspace
closed after):

```
herdr workspace create --label posse-qa73t-probe --no-focus --cwd /tmp   → w2KM:p1, agent_status unknown
herdr pane send-text w2KM:p1 "sleep 6"; herdr pane send-keys w2KM:p1 enter
herdr pane report-agent --source posse-qa73t --agent fakeqa --state working --seq 1 w2KM:p1
herdr agent get w2KM:p1        (during sleep)  → agent fakeqa, agent_status working, terminal_title "sleep 6"
                                                  no source field, no agent_session
herdr agent get w2KM:p1        (7s later, shell prompt on screen)
                                               → agent fakeqa, agent_status working   ← fact 1
herdr pane report-agent … --state unknown --seq 2 w2KM:p1
herdr agent get w2KM:p1                        → agent fakeqa, agent_status unknown  (label kept)
herdr agent wait w2KM:p1 --until idle --until done --until blocked --timeout 1500
                                               → error timeout                       (unknown is not settled)
herdr agent explain w2KM:p1 --json             → agent_explain_unavailable
herdr pane process-info --pane w2KM:p1  (at the prompt)
   → shell_pid 34661, foreground_processes [{pid 34661, argv0 zsh}]                  ← fact 2
herdr pane process-info --pane w2KM:p1  (during sleep 5)
   → shell_pid 34661, foreground_processes [{pid 82630, argv0 sleep}]
/usr/bin/time herdr pane process-info …        → real 0.00
```

Also MEASURED, by reading the installed plugin (herdr-bob at the commit
`herdr plugin list` prints, 2026-09-28): the hook bridge maps
SessionStart→idle, UserPromptSubmit/PreToolUse/PostToolUse→working,
Stop→idle; SessionStart fires only on the first prompt, so a fresh `bob
chat` is adopted by the watcher from `process-info` (argv0 `node`, argv1
`…/bob`) and reported `idle` before any turn; the watcher releases a pane
only when `pane get` fails (the pane is gone), never when Bob leaves the
foreground.

3. **A detected label IS released when argv0 leaves the pane — and the
   verbs do not agree on the second.** MEASURED 2026-09-28 under
   ranger-base-mx5x9 (herdr 0.9.1 client and server, darwin 25.4.0,
   scratch workspace, no agent CLI and no plugin; recipe and full table
   in docs/notes.d/ranger-base-mx5x9.md). The rig: `(exec -a codex sleep
   25)` in a scratch pane — herdr detects from argv0, so it reads `agent
   codex, idle` with no credentials, no network and no turn; that is the
   cheap rig for every label-lifecycle question after this one. After
   the process exits, at the pane's shell prompt: `pane get` answers
   `agent: null`, `agent list` does not list the pane, `agent explain`
   and `agent wait` answer `agent_not_found`. **The wrinkle:** `agent
   get` keeps returning the old row (`agent codex, idle`) for a few
   seconds after those four already say the agent is gone. Nothing in
   posse depends on that window — the detected route's gate is `Seen()`
   over `agent explain`, and the in-place relaunch arm reads `agent
   list` (D3.4) — but a caller polling `agent get` alone would read a
   state for a pane that has no agent, and the lag points the safe way
   only for the reading that treats "no agent" as "died": it fires
   later, never sooner. Read beside fact 1: the detected label expires
   with its process, the reported one by nothing; that asymmetry is what
   D4's two asks are about.

**ASSUMED** (each a line on the instance-side live probe, ranger-base-6wqe):
1. *Discharged, FALSE on darwin* — was "a dispatched Bob pane is labelled
   within the default `startup_wait` (45s): adoption runs every third
   watcher tick at a 2s interval, ~6s, plus Bob's own start — never timed
   on a dispatched launch". MEASURED 2026-10-02 (ranger-base-5jjtn,
   `docs/notes.d/ranger-base-5jjtn.md` §2), and it fails for a reason the
   timing never reaches: **the watcher cannot start on darwin.** The
   plugin's `[[startup]]` hook runs `setsid nohup … bin/bob-watch &`, and
   `setsid` is util-linux — `command -v setsid` exits 1 here — so the child
   exits 127 and `$!` records the dead subshell. Corroborated: zero
   `watch started` and zero `adopted hand-started bob` lines in the
   plugin's log across the whole install, so `adopt_unclaimed` has never
   fired on this box; a live `bob chat` pane reads `agent_not_found` while
   two foreground processes match the plugin's own adoption regexes. The
   claim about adoption's SHAPE still stands — it was read off the plugin
   and the predicate would have succeeded; what is false is that anything
   runs it. Whether adoption lands inside `startup_wait` on a box where the
   watcher CAN run is still ASSUMED and still a 6wqe probe line; the fix is
   upstream's, and D4 gains a third ask.
2. *Discharged, MOOT* — was "the typed work prompt and Bob's own `-p` first
   turn (ADR 0060 D3): the adoption `idle` arrives before the `-p` turn
   runs, so the promptable gate may open on a composer that is about to be
   used by Bob itself. Where the typed text then lands is unmeasured … and
   nothing bounds one that is queued behind the PID turn". There is no
   `-p` turn to queue behind: `bob chat` accepts the flag and submits
   nothing (ADR 0060 Claims/ASSUMED 1, MEASURED FALSE 2026-10-01). The
   hazard this line named cannot occur while that holds, and it comes back
   the day a PID channel does — as a property of whatever channel wins, not
   of this record. Consequences already routed it: "if the probe measures
   the typed work prompt landing before Bob's own `-p` turn, D3's revisit
   trigger has fired and the fix is in delivery, not detection." It has
   fired, and the fix is in delivery.
3. *Discharged* — was "herdr drops a *detected* label when argv0 leaves
   the pane; not re-measured here". MEASURED 2026-09-28 and it holds,
   with the `agent get` lag: Claims fact 3 (ranger-base-mx5x9,
   ranger-base-zcevd). The number is kept so 1, 2 and 4 still match the
   6wqe probe's lines.
4. Bob's sign-in screen discards typed text (ADR 0060's assumption,
   unchanged).

## Verification (the closer's observables)

1. `runtimes/<rt>.yaml` with `detection: reported` and no
   `detection_why:` refuses to load, naming both keys; `detection: x`
   refuses naming the two values.
2. `posse runtime check <rt>` on a fake herdr answering `unknown_agent`:
   `detection: herdr` → blocking gap, exit 1 (today's line); `detection:
   reported` → non-blocking line quoting `detection_why`, exit 0; the
   same fake with a manifest → the inert-declaration line.
3. A dispatch onto that fake with `reported` declared creates the
   workspace and claims (the i3q6g pins inverted); the same dispatch on a
   fake whose `pane report-agent --help` fails creates nothing, claims
   nothing, and names the surface and the declaration.
4. The one detection reading, fed `agent_explain_unavailable` plus an
   `agent get` label with no manifest: `idle` with a foreground pid ≠
   shell pid is seen; `idle` with foreground pid == shell pid is not seen
   and the line names the stale label; `unknown` is not seen; `blocked`
   is returned as given. No file outside the reading names a runtime.
5. The settle ladder opens on reported `done` and waits on reported
   `working` through the same fake, with no change to any consumer.

*Snapshot 2026-09-28: at this record's acceptance, ranger-base-8eqaa (the
delivery route, `Reported` on the detection, the promptable gate's reported
arm) is in its seat tree with zero commits; ranger-base-d8riq and
ranger-base-enmu2 (the i3q6g launch refusal this record carves an arm
into) are open and unbuilt. The beads cut from this record depend on those
landings; `git log --grep <id>` on main is the record, this sentence is a
snapshot.*
