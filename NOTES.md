# NOTES — how posse works under the hood (herdr-native)

The tmux-era implementation — Ghostty splits, the 2×2 grid, the bash spec,
the bubbletea launcher — lives on the **tmux-reference** branch with its own
NOTES. This file describes the herdr-native harness on main.

## The mapping

Every herdr call goes through `Herdr.Run` (`internal/rhq/herdr.go`), which
shells out to the `herdr` CLI and decodes its JSON envelope
(`{"result":…}` / `{"error":{code,message}}`). The mapping
(`internal/rhq/herdrback.go`):

| posse concept | herdr concept |
|---|---|
| session `name` | workspace with `label: name` |
| session dir | workspace `cwd` |
| env set values | `workspace create --env K=V` (per-workspace injection) |
| command / persona launch | `pane run` into the root pane's shell |
| attach / focus | `workspace focus` (re-aims the herdr UI) |
| kill | `workspace close` |

posse never wraps the agent process — `pane run` types the command into the
root pane's interactive shell, and herdr's own detection recognizes the
agent (claude, codex, …) and reports lifecycle state: `working`, `blocked`,
`idle`/`done`. `posse list` and the cockpit surface that state, but only when
herdr actually detects an agent — herdr says `unknown` at the workspace
level even for plain shells, so posse cross-references `agent list`.

## State lives in files, never in the multiplexer

Everything posse knows about a session — emoji, env-set *names*, persona,
and the workspace/root-pane ids herdr assigned — is a flat-YAML meta file
under `state/herdr/<name>.yaml`. That's the exit hatch DIRECTION.md demands:
if a better multiplexer ships, the backend shim is the only thing to
rewrite. Meta files whose workspace has died are pruned on read; workspaces
created directly in herdr still show up in listings, marked `(herdr)`.

That meta is also the session's *recipe*, which is what makes refresh a
one-liner: `posse relaunch <name>` (rangerhq-dxq) lands the plane, closes the
workspace, and recreates it from the same persona, dir, env sets, runtime,
tier, cage and degraded-consent. Landing is one bounded turn (`--timeout`,
default 10m): settle first — herdr's prompt does not track turns — then a
prompt telling the agent to append its lessons to `ORDERS.md`, commit, file
what's unfinished, and push only what its own guardrails permit. A session
that never settles inside the bound is **not** killed; the operator is told
to wait or pass `--no-land`. No agent, or an agent blocked on its own
dialog, is a note and the refresh continues. A persona line is re-rendered
from the PID at every launch (never replayed), so a plain session's `--cmd`
is the one thing the meta has to carry itself (`cmd:`).

The kill is irreversible and the meta it deletes is the only record of what
to recreate, so refresh **secures the replacement before destroying the
original** (rangerhq-v52t). A launch is two halves: `planLaunch` resolves
persona, runtime, tier, cage, parity, skills, seatbelt, gates, env sets and
dir without touching herdr, and `startPlanned` turns that plan into a
workspace. Relaunch plans first and prints what it resolved (`checked
<name>: …`), so every refusal knowable in advance — a dir that no longer
exists, a PID that no longer loads, a launch that would now be degraded —
arrives with the session still running and nothing to recover. One such
refusal does not live in the plan at all: a herdr workspace already wearing
the session's *label* that posse has no meta for (an earlier incident's
orphan, a second `RHQ_HOME` on one herdr, a label an operator typed). It is
still there on the far side of the kill and `nameFree` would refuse the name
then, so the preflight asks the listing for it and names it — the session
survives, and the operator is told which workspace to rename. The recreate
is then built from the very plan that was verified, so preflight and create
cannot disagree.

Every step of a relaunch is aimed by the session's own **record**, never by
its name: `Resolve` falls back to foreign workspaces by label, so on that
same board the kill closed a stranger wearing the name — unlanded, never
matched to the meta — while the live session it was asked to refresh kept
running (rangerhq-9jk1). The kill asks the listing whether it holds the
workspace the meta *names*, under an identity this pass may call its own
(`notOurWorkspace`); anything else is `clearDeadMeta`'s to prove dead or
refuse over. And the rollback below is the fourth meta-destroying step, not
a third: blanking `workspace:` is a *write* over a meta, as unrecoverable as
the delete, so it asks `mustNotOrphan` too and leaves a record it cannot
prove dead standing — naming the workspace in the error instead.

The one failure no ordering can prevent is herdr's own workspace create,
which is on the far side of the kill by construction:
there the session's recipe is written back with no `workspace:`/`pane:` (it
can orphan nothing and cannot be inferred dead and pruned), the error
carries both the retry (`posse relaunch <name>`) and the hand-rebuilt
`posse new …` line, and `posse list` reports the kept recipe as what it is.

### The launch line is typed, so it has a limit — 1023 bytes (rangerhq-ybec)

A launch is a *command typed into the pane's shell*: `herdr pane run` puts
it through the tty exactly as a keyboard would. A pane `workspace create` has
just returned has not started its ZLE yet — the shell is still reading
rc files — so the tty is in **canonical mode**, where the line discipline's
per-line buffer is `MAX_CANON`, 1024 bytes *including* the newline that
would submit it (`sys/syslimits.h`). Over that, the head is echoed raw, the
tail sits in the buffer without its newline, and **nothing runs** — the next
thing typed is appended to the leftover. Hence `PaneLineMax = 1023` in
`internal/rhq/paneline.go`, the last length that survives.

Waiting is not the fix, twice over. `herdr pane process-info` reports a
`shell_pid` from the very first sample and a shell-alone foreground group
within 0.03–0.35s, and a line typed the instant that predicate goes true is
still lost: there is no observable for "ZLE has the tty". And a settled pane
is bounded too, only further out — on a three-second-old pane 24000 B ran
3/3 and 28000 B ran 0/3 — so a long enough line is lost no matter how long
anyone waits, and "wait for the shell" could never have been the general
fix.

So the line stays short instead. Over the limit, `App.PaneLine` writes the
command to `state/launch/<session>.sh` and types `. <path>` — **sourced**,
not `sh <path>`, so the tty's foreground process is the runtime itself and
not a shell holding it, which is what herdr's argv0 detection,
`RelaunchAgent` typing into the surviving shell, and a kill all rest on. A
superseded script is removed the moment a line fits again: a rendering left
behind that nothing runs is how the next debugging session gets misled. It
is wired at both launch sites (`startPlanned`, `RelaunchAgent`), and
`KillSession` drops the script beside the meta.

Under the limit the command is still typed verbatim, because the pane's
scrollback and herdr's log are where an operator reads what a session was
launched with. Headroom today: every crew PID's line renders to **591–691
bytes** against the 1023 limit, so nothing spills (`state/launch/` does not
exist on a healthy fleet). That is ~330 bytes of slack, and anything that
grows a typed line spends it — another deny rule (`--disallowedTools` is
variadic), a longer `--settings`, more mounts. The container tier's ~1.6KB
engine line spent it in one go and had to render a file before this rule
existed; see *Container tier* below.

None of this is asserted from memory: `internal/rhq/panelinelive_test.go`
(`RHQ_LIVE_PANE_LINE=1`, against a scratch herdr server, no API turn) is the
live pin both sides of the cliff and the spill are measured from.

## Dispatch primitives

- `posse prompt <name> "<text>" [--wait] [--timeout ms]` — submit work to the
  session's detected agent (herdr `agent prompt`; `--wait` blocks until the
  first settled idle|done|blocked state).
- `posse wait <name> [--until <state>]…` — wait for an agent state.
- `posse peek <name> [lines]` — the session's terminal tail (`pane read`;
  the tail is computed client-side because herdr's `--lines` counts padded
  blank screen rows from the bottom).
- `posse ready [--dir] [--as]` — unblocked work via `bd ready --json` (the
  `Bd` runner in `internal/rhq/beads.go`). Without `--dir` it aggregates
  across the config `beads:` repo list; missing or unreadable repos are named
  as failed scans while readable repos still report their work.
- `posse crew <name> [--off]` — hand a session to the operator or back to the
  fleet (ADR 0008). **A crew session is one the operator talks to, and
  dispatch treats it as if it did not exist** — never prompted, never
  relaunched, never counted busy; `posse new`/recipes/cockpit `p`/`posse prompt`
  without `RHQ_PERSONA` set the mark, cockpit `o` and `--off` clear it, and
  a bead whose own session is crew is reported `held by crew session <name>
  (operator's) — skipped` (there is no timer and `--resume` does not
  override — release it first). `posse list` and the cockpit tag it `👤`.
- `posse claim <id> [--as <persona>] [--dir]` / `posse done <id> …` — atomic
  claim (`bd update --claim`) and close, with the persona as bd actor. The
  claim's outcome is read from the bead, never from bd's exit code (see the
  bd substrate section): `claimed`, `resumed` when the actor already held it,
  and a non-zero exit naming the holder when it went to somebody else.

Prompt targets resolve session → workspace → detected agent (root pane
preferred). Nothing depends on herdr agent *names*, which die with the
process — durable identity is the beads **assignee**: a persona is its
assignee name + `agents/<name>.md` + (future) a per-persona memory dir.

These compose into `posse dispatch` (`internal/rhq/dispatch.go`) — one pass
of the harness core:

1. Gather ready beads (config `beads:` repos, or `--dir`) and order them
   into a queue: **priority first** (P0 before P3), oldest first inside a
   priority — with `--resume`, `in_progress` beads ahead of all of it.
   `bd ready`'s own order is the query's, and `-n` takes the top of this
   list (rangerhq-1r2).
2. Route each to a **lane**, and then to a **seat** in it (ADR 0020 §2 —
   two questions the pass used to conflate). *Which lane*: bead
   **assignee** that is a persona wins and is a lane of one that never
   falls through, then every persona whose `labels:` overlap the bead's,
   ordered by PID `route_order:` (default 50, ties broken by persona
   name), then config `default_persona:` — also a lane of one. Unroutable
   beads are reported and skipped. Config `coordinator:` is returned by
   **no** path (ADR 0018): assignee-is-the-coordinator refuses loudly with
   no fallthrough to label routing, the label loop skips that PID, and a
   `default_persona:` naming her is a loud config error. Both launchers
   share `Route`, so the refusal covers the pass, `--watch` and the
   cockpit's `d`; no flag reaches past it. Unset key = no coordinator =
   pre-0018 behavior. All three refusals compare *identity*, not the
   string: `LoadAgent` resolves a path, so `Coordinator`, `./coordinator`
   and `coordinator/../coordinator` all reach `coordinator.md` while
   comparing unequal to `coordinator` — a hostile assignee walked past a
   name-equality refusal, and an operator writing `coordinator: Coordinator`
   against `coordinator.md` disabled all three at once (rangerhq-c6u6).
   `isCoordinator` case-folds and reduces a path spelling to the file it
   names; `App.CanonAgent` resolves an outside name (queue assignee,
   config value) to the agents-dir spelling, so a routed persona is named
   the way its PID is named — in the session name, `BD_ACTOR`,
   `RHQ_PERSONA` and the PID the launcher cats — and a name that is not
   `ValidName` routes nowhere.

   The label step used to be *the first* persona whose `labels:`
   overlapped, walking `ListAgents` — os.ReadDir order, i.e. alphabetical.
   Nobody chose alphabetical as a priority scheme, but on every unassigned
   bead it was one, and it favoured the seeded generics over the lanes the
   operator wrote: `developer` took `code`, `devops` took `infra`, and an
   audit of the crew found 11 of 37 unassigned open beads parked on PIDs
   with 14 and 0 lifetime closes (ranger-base-2yj5). `route_order:` makes the order sayable; its default
   leaves every existing instance's winner exactly where it was (all tie,
   the name decides), so this is a statement of the behavior plus a
   control, not a change of it. Retiring the unused generics is the other
   half, and that half is config — the operator's, not code's. `why` now
   carries the race — `label:code (first of 2: <winner>, <runner-up>)` —
   capped
   at four names with `+N more` and never a silent count, because the
   audit that found this needed a script to answer a question the pass
   line was one clause short of answering. It costs a `LoadAgent` per
   persona per bead instead of stopping at the first match — which is what
   makes the count honest.

   *Which seat* is the second question, and only the fire loop can answer
   it: the bead is seated on the first candidate in that order that is
   actually **free** — not made busy earlier in this pass, and with no
   working or blocked session of its own in the repo. So name order is a
   TIEBREAK and not a priority: it decides who takes the first bead while
   several seats are free, and the next bead in the same pass overflows to
   the next seat. A pass with three unassigned `code` beads and three free
   `code` seats fires all three. Every seat busy is the LANE being busy and
   the report says so — `code lane busy: developer, hopper, wren`, not one
   persona, because "wait" and "hire" are different answers and only the
   lane line tells them apart (lane concurrency *is* seat count, ADR 0020
   §4). When a lane is wider than one seat the pass prints why that seat
   won — `label:code (seat 2/3: hopper; developer busy)`. `--persona X`
   restricts *seating* to X: a bead whose lane contains X seats there, one
   whose lane does not is skipped without a line of its own and counted in
   a single closing line, so a filtered pass that dispatches nothing can
   still be told from an empty queue.

   The guards after the seat is chosen stay BEAD skips, not fallthroughs —
   a crew session holding the bead's own session, a holder that settled, a
   session prompted seconds ago. Each is a fact about *this bead*, and
   falling through any of them would hand one bead to a second persona.
3. Resolve the bead's **tier** (ADR 0003 §2): `--tier` > bead label
   `tier:strong|standard|fast` > config `tier_by_label:` (one-level map;
   when the key is absent the Dial B default applies: doc/groom/triage/
   scaffold/hygiene → fast, architecture/security/adr → strong; a present
   key — even empty — replaces it) > PID `tier:` > config `default_tier:`
   > strong. The pass output says which rule decided (`fast via
   tier_by_label doc`). Then create a **fresh session per bead** —
   `<persona>-<repobase>-<beadid>` (Dial F: no context accumulation, cost
   attributable per bead) — with persona command, env sets, `BD_ACTOR`,
   `RHQ_RUNTIME`/`RHQ_TIER` injected by CreateSession, and wait for herdr
   to detect the agent. Still one bead per persona per repo per pass (the
   busy key is the persona's repo slot, and any working/blocked session of
   that persona in that repo — per-bead or the pre-Dial-F `<persona>-<repo>`
   one — takes that SEAT out of the walk for the pass, which since ADR 0020
   §2 offers the bead to the next seat in the lane instead of skipping the
   bead). Sessions of finished beads are left idle for the
   operator to reap (`posse kill`, or from the cockpit); they cost nothing
   and do not block dispatch. Note the operator's own bead-cost tooling
   attributes by session, so per-bead sessions are what it wants. A session that is alive but has no
   agent (the CLI crashed or was closed; the workspace survives as a bare
   shell with the launch env intact) gets the persona command re-typed
   into its root pane (`RelaunchAgent`, rangerhq-vk2) — never within
   `RelaunchGrace` (45s) of the last launch (`launched:` in the meta file),
   since a CLI still starting is invisible to detection and a second command
   would land in its input box. That grace is its own field, not
   `StartupWait`: the detection patience is a knob tests shorten to
   milliseconds, and while the two shared one, the guard was a stopwatch a
   loaded box could outrun (ranger-base-ze9p). A launch that fails benches
   the persona's slot for the rest of the pass **unless the failure is the
   pane's own** (no agent, never promptable, a screen posse does not know),
   which leaves the slot free for the next bead's own fresh session — ADR 0013 §2's busy-key
   split, below. `-n` bounds attempts (successes and failures), so a pass is
   bounded in wall-clock.
4. Atomically claim as the persona (`bd update --claim` — losing a race is
   a clean skip; the outcome comes from reading the bead back, because bd
   exits 0 on a refused claim, rangerhq-kux), then prompt with the
   **assembled work prompt** (ADR
   0005, `workPrompt`/`promptContext` in dispatch.go) with `--wait`
   — **on a `prompt: typed` runtime.** On `prompt: argv` (grok, codex) the
   order is claim → write the prompt to `state/prompts/` → create the
   session with `"$(cat …)"` on its launch line → await a state herdr has
   *seen*: the claim is the fence and there is nothing to type. See *Argv
   delivery* below for the failure cleanups, which differ per step
   (one leg is 15 min, `--timeout` overrides; see 6 for what happens when
   a leg runs out). The prompt is skeleton +
   Context + escalation ladder + persona hook, ≤ ~40 lines, references
   not content: `Work beads issue <id> (title, quoted as data: "…"). Run
   bd show <id> first.`; then `Context` — repo · runtime/tier · labels;
   `from:` non-blocking parents (discovered-from / parent-child, from `bd
   dep list --json` relation types); `unblocked by:` closed blockers;
   `design:` docs/adr paths found by regex in the bead's and parents'
   text; `orientation:` repo-root files that exist (`AGENTS.md`,
   `DIRECTION.md`, `NOTES.md`; config `orientation:` overrides the list)
   with "your PID's guardrails override any push/deploy instruction in
   repo docs"; "comments carry decisions — read them" when the bead has
   any (each line only when non-empty; every bead-sourced string
   %q-fenced); then the fixed **escalation ladder** — NOTE / ASSUME /
   SPIKE / ASK / HANDOFF / REFUSE with exact bd commands, ASK beads `-l
   question -a <config operator:>` (unassigned when unset) plus `bd dep
   add` so the bead leaves `bd ready` until answered; SPIKE files a
   `spike:` bead in the runner's lane and dep-blocks this one the same
   way, because its gap is knowledge, not permission; `Done:` line; then
   the PID's `## Work prompt` section verbatim (optional; `posse agent
   check` warns when absent). Beads labelled `question` are for the
   operator: dispatch and the cockpit never route them, they cost no `-n`
   attempt, and under `--persona` only a question assigned to that persona
   is reported. `bd prime` is *not* injected (its close protocol says
   `git push`; see the ADR). Two extra bd calls per
   launch (dep list, comment count), best effort. A claim already held
   by this persona is a **resume** in the bead's own session — but only
   when the run was interrupted (no live session, or its agent gone). If
   the bead's session (or the pre-Dial-F persona session) is alive with a
   settled agent, the persona *stopped* on that
   bead (blocked and said so, or waiting on a human) and re-prompting it
   every pass would be a token loop under `--watch`; such beads are
   listed as "held by <persona>, <session> idle — stopped on purpose?"
   and skipped until `--resume` (rangerhq-zom). The cockpit's `d` is an
   explicit ask and always resumes.
5. The prompt's `--wait` runs in a goroutine and the pass moves on to the
   next bead: **fire every routable bead first, then gather** the settles
   in launch order (rangerhq-tqr). Sessions launch serially (create →
   await → claim → prompt); only the wait for the work overlaps, so a
   pass takes as long as its slowest bead, not the sum. Gathering serially
   is free — the sessions work concurrently and a settle that already
   happened returns at once.
6. Judge by the **bead**, not the agent: issue closed → ✓; agent settled
   `blocked` → ⛔ flagged (herdr's sidebar already shows it); settled but
   issue still open → ◑ review the session.
7. A `--wait` **timeout is a check-in, not a verdict** (rangerhq-1z0).
   herdr's error code (`timeout`, typed as `HerdrAPIError`) says the wait
   ran out, not that the prompt failed, so gather asks herdr what the
   agent is doing: still `working` → print `still working after <t>` and
   wait another leg, up to `--ceiling` (default 4h), then leave the bead
   with its agent — **claim kept**, counted dispatched, not judged this
   pass (a later pass sees it held by the persona, not free); `blocked` →
   ⛔ with the claim kept; `idle`/`done` → the agent settled between the
   leg running out and the check, so the bead is judged as on any settle.
   Anything else — no agent detected, a state nobody names, a herdr that
   does not answer — is **ignorance, not a verdict**: the status is
   re-asked for ten seconds (detection blinks through a modal or a
   redraw), and if it is still unreadable the claim is *kept* and the bead
   reported `wait timed out after <t> and <what herdr said> — claim kept`
   (rangerhq-khc: a 40-minute bead was handed back on one blank poll while
   its session worked on for another 40). **A `--wait` timeout never
   unclaims.** Every other prompt error (`agent_prompt_stalled`,
   `agent_not_ready`) still does: those mean the prompt never took.
   Unclaiming live work re-dispatched it into a second fresh session and
   lost the assignee for `posse cost`/`scorecard`.

**The `await` in that sequence is the readiness gate**, and it waits for a
state herdr can *see* (`awaitSettled`, `internal/rhq/dispatch.go`). herdr
answers `idle` for a pane it has identified as a known agent even when no rule
matched anything — `agent explain` calls that
`default_known_agent_idle_fallback`, `matched_rule: null` — and in a launch
that guess arrives before the CLI does: measured on a dispatch-shaped grok
launch, herdr said `idle` at **0.20s over the shell's own prompt line**, and
grok did not take the screen until 0.39s. A prompt typed into that window is
typed at a shell, buffered through the exec, and delivered somewhere nobody
chose — the loss behind rangerhq-37c and rangerhq-5on. So the gate holds
through the guess and returns on the first screen a rule matched (measured:
0.49s `osc_title_idle` on grok, 0.93s its `startup_splash`, and claude's
`live_prompt_box`); a pane herdr only ever guesses about fails the launch
**loudly** rather than being typed at (rangerhq-3hb5). Where `agent wait` and
`agent explain` disagree — one detection, sampled a beat apart — explain wins:
a fresh claude settled `idle` and had drawn its trust dialog by the time
explain answered, and that is a screen no work prompt may go into.

Personas already `working`/`blocked` are skipped for the pass; one bead
per session per pass. `--dry-run` shows routing without acting; `-n` caps
launch attempts; `--persona` limits to one persona.

**The launcher lock** (ADR 0011 §1, rangerhq-tzdf) makes *one launcher at a
time per RHQ_HOME* a kernel fact rather than an operating rule. A hand-run
`posse dispatch`, an autostart `--watch` loop and the cockpit's `d` are three
processes over one bd queue, one meta dir and one herdr, and every guard
above (crew-held, working/blocked, prompted-recently, the busy map) is a
check against state the other two mutate between the check and the launch it
authorizes — rangerhq-9nso is what that cost. `flock(2)` on
`$RHQ_HOME/state/dispatch-launch.lock` closes it: **Run's fire loop** (steps
2–5) holds it, the **gather** (6–7) does not — gathering only reads and
judges, so a pass holds for `-n` × (create + StartupWait), not for as long as
its beads run — and **`LaunchBead` holds it for its whole body**.
**verify-after holds it too** (rangerhq-th7l): it is the one write a pass
makes before the fire loop, its dedupe is check-then-act, and unserialized it
files the same verify bead twice. It takes and drops the lock inside its own
call — so `posse ready`, which files by the same rule, waits for it as well —
and Run's two acquisitions are sequential, never nested. A second
launcher blocks, saying `⏳ launcher lock held by pid <n> — waiting` first
(the cockpit's writer is `io.Discard`, so there it just keeps saying
`dispatching <id>…`). `--dry-run` never takes it: a dry pass acts on nothing
and a read-only command must not queue behind a live one.

Why flock and never a second pidfile: a pidfile records liveness in a file
whose truth decays and the reader has to infer (rangerhq-ct9/ppy9); an flock
lives on the open file description, so **release *is* process death** —
crash, `kill -9`, closed pane alike — kernel-owned, with no staleness class
to detect and nothing to reap. The `pid:`/`since:`/`cmd:` the holder stamps
into the file is read for exactly one purpose, printing that waiting line;
nothing decides anything from it, and a dead or unreadable pid degrades to
"another launcher". The file is created and **never removed** — unlinking it
would let the next launcher lock a fresh inode: two holders, one path, no
error anywhere. `internal/rhq/launchlock.go`.

**The session meta is the run record** (ADR 0011 §3, rangerhq-o2ki). The lock
serializes launchers; it does not give one a way to *see* what the last one
did, and the guards it protects read stores that lag a launch by seconds. Two
fields in the meta close that — `bead:`, the bead this session was created to
work (ADR 0013 §4), and `prompted:`, when a work prompt was last sent to it —
and wherever dispatch needs a fact about the run it reads the record it wrote
rather than inferring one from a name pattern or a snapshot.

- **`prompted:` is PromptGrace's memory.** It was `lastPrompt`, a map in one
  process, so the cockpit's `d` and a running pass could not see each other's
  prompts and both prompted one bead: the half of rangerhq-tzdf's
  "no double-claims" the lock alone did not close, because `Run` reads `bd
  ready` *before* the fire loop locks, so the waiting pass fires from a list
  the holder already consumed and every guard then abstains in turn — `busy`
  is per-pass, `personaActive` reads the fresh agent as idle rather than
  working, and the `in_progress` check reads the stale row. `prompted:` is the
  one store that has moved, because the holder wrote it before dropping the
  lock. `promptedRecently` believes the later of the file and the map (the map
  still covers a session with no meta) and reads the file **directly**, not
  through `Sessions()`: "when was this prompted" is the record's own content,
  where a listing answers liveness at a herdr round trip per bead per pass,
  and the two can disagree only for a meta the listing would drop — where this
  reads "prompted recently" over a session the caller then declines to prompt,
  which is the direction every guard here fails in. The pass's guard stands
  down wherever a launcher is *deciding* rather than missing: a holder join
  that found the session and an operator's `--resume` that answered for it, a
  row naming another actor (the claim answers that, and must be allowed to
  fail), a session herdr reports `done` in, and one herdr detects no agent in
  at all — a crashed CLI, which `RelaunchAgent` answers, not a lagging status.
- **`bead:` makes the holder join a lookup.** `RunHolder(dir, persona, bead)`
  finds the live session whose own record says it was created for this bead,
  and the two names the join used to walk — the bead's Dial F name, then the
  pre-Dial-F slot — stay behind it for sessions with no record to find. The
  persona is part of the key because it is part of both those names: a record
  pointing at somebody else's session is not the join's answer, it is a
  session running another PID. The checkout is compared, not the working
  directory, since a per-session worktree's `dir:` is not the repo dispatch
  names — `repo:` is (rangerhq-09o2).

Pins: `internal/rhq/runrecord_qa_test.go`, and the pass↔pass repro in
`launchlock_qa_test.go` (`TestTwoPassesDoNotDoubleClaimOneBead`).

**The plan-utilization guard** (rangerhq-jgm) takes one shared reading before
anything else in a pass — before bd is asked for ready work — then applies
that reading per bead once dispatch knows which runtime the bead will spend
(ADR 0013 §3). On a subscription plan the real budget is not API-equivalent
dollars (`posse cost`, ADR 0003 §4) but the plan's own rate windows: a rolling
5-hour one and a 7-day one, each a utilization percentage. The point of
watching them is the operator's interactive headroom — a fleet that eats the
5h window leaves the operator staring at a rate limit.

- **Config** `plan_guard_5h:` / `plan_guard_7d:` (percent; the analyst's
  suggested start 70 / 85). **Both unset by default** — guard off, and then *no
  request is made at all*: unset is exactly today's behaviour. A value that
  is not a percent 1–100 is named on stderr and that window stays ungated —
  a typo must be visible, not a silently dead guard.
- **Credential** the runtime CLI's own access token, read fresh per pass
  from the provider adapter's credential source and sent as
  `Authorization: Bearer` to the provider's usage endpoint (the adapter
  seam of ADR 0012 D4; the concrete keychain item, endpoint and headers
  are the adapter's business and documented instance-side). The token
  lives in memory for the one request and is written
  **nowhere** — not logs, not meta files, not bead comments; the errors in
  `internal/rhq/planusage.go` are deliberately generic so the credential
  cannot ride out in one.
- **Where that credential comes from** (ADR 0019, ranger-base-x584) one
  seam — `ReadCredential(runtime, purpose)` in
  `internal/rhq/credential.go` — is the only place posse acquires a
  credential at all. The `meter` purpose reads the RUNTIME's own store of
  record, picked by `runtime.GOOS` at run time and never by a build tag (so
  `make test-linux` compiles and tests both branches): the macOS keychain
  item on darwin, `~/.claude/.credentials.json` everywhere else, both fed
  through the SAME envelope parser so a shape diagnosis is written once and
  cannot fork. The `session` purpose wraps the env-set lookup that already
  existed (`CageCredential`) — the operator's own scoped mint, store of
  record is the home. posse never writes a rotating credential; `posse
  refresh` (ranger-base-h207) is the one credential write and it is
  human-gated. A store that does not exist for this (runtime, purpose,
  platform) is `*NoSource`, a distinct error class: the guard runs OFF with
  a witness rather than blind, because blindness has a clock on it and a
  structural condition reported as an outage would park the fleet forever
  (ranger-base-vmqg does that rendering). Until the Linux probe runs (ADR
  0019 V1) the non-darwin path says in its own error text that it is
  built-but-unconfirmed.
- **Refused by our own gate** (ranger-base-r64) on darwin posse reads that
  credential by running `security`, and a persona pane puts that persona's
  L1 shim dir first on PATH — so any `posse` command typed inside a pane
  whose PID denies `Bash(security:*)` (every crew PID does) has its own read refused.
  That is a distinct error, not "keychain item unreadable": the blind line
  and `plan-usage.log` name the deny rule, and the launch preflight — whose
  UNKNOWN branch is otherwise silent — says it once per process on stderr.
  The distinction is the point: the two strings used to be identical, and on
  2026-08-24 a refusal read as an outage and `plan_guard_blind_max: 0` was
  set for hours in response. Resolving `security` by absolute path is the
  other half of the fix and waits on ranger-base-17i.
- **Above either threshold** the pass still runs. Each bead whose resolved
  runtime is on the guarded meter faces the ADR 0010 ladder (overflow when
  configured and eligible, otherwise park) with a line naming the window and
  number — `plan 5h at 78% > 70% — skipped`. A bead on a different built-in
  runtime launches ungated. 5h is checked first (tighter window, the one the
  operator feels); strictly above, so exactly at the threshold still runs. A
  pass whose every bead parks dispatches zero, which makes `--watch` treat it
  as a quiet pass and back off.
- **Fail-open with a bounded blind window when unattended** (rangerhq-6h1,
  the analyst's ruling on rangerhq-30m). An unreadable credential or endpoint is a
  monitoring failure and the fleet never halts on one *while a human is
  watching*: a hand-run pass prints the one stderr line and runs, exactly as
  before. Under `--watch` there is no witness for that line, so blindness
  becomes a state with a clock on it — **time since the last successful
  reading**, seeded at loop start (a fresh loop gets the whole grace, not an
  instant skip):
  - **under `plan_guard_blind_max:`** (default **10m**) — the old behaviour
    unchanged: one stderr line, pass not gated, pass runs;
  - **over it** — the pass still gathers and routes work, and forks on
    whether Dial E is armed (ADR 0018 §1, below). With `budget_pass:`/
    `budget_day:` **unset** the plan guard is the last automated brake and it
    fails closed: on-meter beads park (`plan guard: blind 12m (usage endpoint
    unreachable) — skipped`) without being claimed; off-meter beads launch. A
    pass whose every bead parks dispatches zero, so `--watch` backs off toward
    `--max-interval` on its own;
  - **over it, with a cap set** — there is a floor under the blind meter, so
    the pass **degrades** instead of parking: one line per pass on the pass
    output (`plan guard: blind 4h00m (…) — degraded, running under ledger
    brake (pass $8.20/$30.00, day $146.00/$250.00)`), then Dial E's ordinary
    rungs decide per bead. The degrade is bounded **by the ledger, never by
    wall-clock** — run while something is still counting, never because the
    clock ran out. The classes of failure do not fork this (§2): a shape
    mismatch, a gate refusal, a 401 and a dead socket are one state — no
    reading. They shape the diagnostic and the cooldown, never the policy,
    because policy that reads diagnosis strings rots when the diagnosis
    improves. And if the cost scan itself cannot be read the floor is gone,
    so the pass parks with both failures named (`…, ledger unreadable (…) —
    skipped`);
  - **the first good reading** clears the clock and that same pass proceeds.
    No manual reset, no sticky state, no operator action.
  - **`plan_guard_blind_max: 0`** is the operator's escape hatch for on-meter
    work: never fail closed. It is not needed to keep off-meter work alive.
    Unsetting the thresholds also disables the guard entirely — then nothing
    is read. It is quiet tolerance without end, not a degrade: nothing is
    declared under it, because nothing has been decided. The knob's single
    meaning since ADR 0018 is exactly that — **how long quiet tolerance
    lasts before the fork**, never how long a degrade may run.
  - **Log noise**: the fail-open note is said when the reading first fails,
    at most once an hour after that, and once more when a reading comes back.
    Past the budget there is no separate pass-level repeat; each on-meter
    bead's park line names the current blind age and cause.
  Why 10 minutes, and why a duration rather than "N failures": `--watch`
  backs off 8×, so N means anywhere from 15 minutes to 2 hours. The 5h
  window was measured (instance-side) moving fast enough under a handful of
  concurrent persona sessions that the gap between a 70%-class threshold
  and a rate limit is on the order of half an hour of blind fanned-out
  dispatch; 10 minutes spends at most a third of it. The asymmetry: a wrong
  skip costs $0 and heals on the next reading, a wrong run costs a window
  that takes five hours to heal, at the exact moment nobody can be told.
  The cost of the trade, said out loud: a permanently unreadable endpoint
  parks the guarded-meter lanes until someone notices; other meters keep
  moving.
- **One reading, shared by every caller** (rangerhq-tdy8). The usage
  endpoint is a *metering* endpoint, and posse had three independent
  pollers on it: the cockpit every 2m for as long as it is open (~30
  requests/hour), one per dispatch pass, one per `posse cost`. On
  2026-08-22/23 it answered 429 for hours at a stretch (02:13–04:39 and
  17:21–20:35 in `$RHQ_HOME/state/dispatch-watch.log`) — and a 429 is a
  *blind* guard, so an unattended loop fails closed and a rate-limit storm
  stops the fleet. All three callers now read one snapshot,
  `$RHQ_HOME/state/plan-usage.json`, and only refresh it when it is older
  than `plan_usage_ttl:` (**5m** default, `0` = no sharing). The endpoint
  stays the store of record; the file is a snapshot with its age attached,
  and each caller says how stale a snapshot it can act on — the guard never
  decides on one older than **half** `plan_guard_blind_max:`, so a cache hit
  can never be what fails a pass closed. The blind clock counts from when
  the reading was *taken*, not from the hit that served it, so sharing buys
  no grace. Two rules the incident bought: **Retry-After is honoured across
  processes** (the cooldown lives in the same file, capped at an hour — past
  that the blind window decides, not a silent mute), and **every request
  that actually leaves the machine is logged** to
  `$RHQ_HOME/state/plan-usage.log` (`<RFC3339> <caller> ok|429 cooldown=…`,
  trimmed to the newest 1000 lines) — cache hits write nothing, so that
  file's cadence is exactly what the endpoint sees of us. No lock: writers
  only ever replace one snapshot with a newer one, and last-writer-wins is
  right for a snapshot — a success landing after a 429 clears the cooldown,
  which is what a success means.
- **Overflow: a second pool instead of a skipped pass** (ADR 0010). The
  guard's meter belongs to *one* provider, so the whole-pass skip had two
  costs: a lane whose runtime is not on that meter was skipped because
  somebody else's window was hot, and a pass that could have run at equal
  posture on a second pool ran nothing. `plan_guard_overflow: <runtime>`
  (unset = no pool move; on-meter beads park) makes a **tripped** guard decide
  per bead, at launch:
  - **resolved runtime not on the guarded meter** — the built-in runtimes
    that are their own pools — **launch as today, ungated**. A template-only
    `runtimes/<name>.yaml` is *unknown*, and unknown is gated: "this runtime
    is free" is the expensive guess to get wrong.
  - **eligible and the cap has room → launch on the overflow runtime.**
    Eligible means all three of: `CheckParityIn(pid, overflow, cage, tier,
    dir)` has **zero** `Degraded` entries — *clean* on the target, not "as
    degraded as the guarded runtime", because this move is dispatch's own
    choice and dispatch never holds `--allow-degraded` (the rule Dial E
    already uses for `fast`); the resolved tier is **not `strong`** (the tier
    table maps `strong` to "runtime default" on the overflow targets, which
    is not the model the tier meant — judged work never moves); and the PID
    has not said **`overflow: false`**, the opt-out for what a parity matrix
    cannot see (a lane that drives through repo shell scripts stalls on a
    target whose unattended mode refuses to run an unknown local script).
  - **otherwise → the guard's line, per bead** rather than per pass, naming
    which rung refused it.
  Only sessions **this pass creates** move; a session that already exists
  keeps the runtime it was created with, and so does a pass given an explicit
  `--runtime` (ADR 0002's precedence — the operator decided). Dial E is
  untouched: it still resolves the tier, and on a moved bead its step-down is
  judged against the pool the bead is actually going to.
- **The cap is required, and it is the whole difference** (ADR 0010 §3).
  `plan_guard_overflow_cap: N` — max beads sent to the overflow runtime in
  any **rolling 7 days**. An overflow runtime *without* a cap is overflow
  **off**: the pass is skipped as before and one stderr line says why, every
  pass. Beads and not dollars because the second pool has no meter posse can
  read and `posse cost` cannot see its spend; rolling and not calendar because
  the pool's reset day is the provider's secret and a rolling window
  upper-bounds every calendar week without knowing it. A weekly pool with no
  intra-week reset is exactly the shape a per-pass trigger over-drains.
  - **Ledger** `$StateDir/overflow.log`, append-only, one line per overflow
    launch: `RFC3339 runtime bead persona`. Read **once per pass** and only
    on a trip; counted **per runtime**, so changing the overflow target does
    not charge the new pool for the old one's week. Written **after** the
    launch, not after the decision — a bead that never reached its agent
    spent nothing. Unreadable → overflow off for that pass (a skipped pass
    heals itself; an uncounted week does not).
  - Cap reached → the bead's line names it: `plan 5h at 78% > 70%, overflow
    grok: 20/20 in 7d — skipped`. `--dry-run` shows a move as
    `[grok ← overflow]`, and so does the prompted line of a real launch.
- **A blind guard parks only the meter it guards; it never overflows** (ADR
  0010 §5, ADR 0013 §3). The blind state is not an over-threshold trip, so the
  overflow ladder does not run, cap or no cap: every rung is a judgement made
  *on a reading*, and blind there is none. Per bead: off the guarded meter →
  launch; on-meter and blind → park without claiming; on-meter and over a
  threshold → the normal overflow/skip ladder. The first good reading resumes
  on-meter service including overflow. Blindness writes nothing to
  `overflow.log`, including when off-meter work launches through it.
- **Display** `posse cost` ends with the current reading and the cockpit
  header carries `5h 42% · 7d 61%` (refreshed every 2 min, off the event
  loop). `posse cost --plan` is that one line on its own, without the
  transcript scan — the form a fleet persona or a shell asks in, since
  `Bash(posse:*)` is already granted and the windows are the number every
  plan question starts from (rangerhq-p3z). It reads the same shared
  snapshot at the same TTL, so asking costs the endpoint nothing extra;
  unlike the footer it is **not** silent when the reading is unavailable —
  the reading is the whole output, so it exits 1 and says why. Current reading only, no history; never a guessed number. An empty
  segment used to mean *either* "unreadable" or "no guard configured", which
  is what let a blind guard hide; so when the thresholds **are** set and the
  last read failed, the header says `5h — · guard blind 14m` instead
  (rangerhq-6h1). Unconfigured stays clean. `RHQ_PLAN_USAGE_URL` redirects
  the endpoint for tests, and **only to loopback**: an override naming any
  other host is refused by name rather than followed or silently ignored
  (`internal/rhq/credpin.go`, ranger-base-17i). Loopback buys the seam and
  nothing else (ranger-base-dr6u) — on this machine a `127.0.0.1` listener
  is something any caged persona can bind, so an override is asked with **no
  Authorization header** and the keychain is not read for it at all, and its
  answer is **never written to `state/plan-usage.json`**, the snapshot every
  posse process on the instance reads for the TTL. An override reads for the
  process that set it; it is not the fleet's fact, and neither is a 429 it
  answers. The preflight's twin of it, `RHQ_MODEL_LIST_URL`, is
  deleted outright — nothing but the vulnerability read it.
- The reader is exported as `PlanReader.Read` for rangerhq-25p, and Dial E
  took the offer: when the guard has read the windows this pass, those
  percentages compete with the dollar windows below. The cockpit's `d` key
  is an explicit operator ask, not a pass, and is not plan-gated.

**Budget caps and step-down** (ADR 0003 Dial E, rangerhq-25p,
`internal/rhq/budget.go`) are the dollar half of the same idea: where the
plan guard parks beads that spend the guarded rate windows, Dial E slows and
then stops dispatch on API-equivalent spend.

- **Config** `budget_pass:` / `budget_day:` in dollars (a leading `$` is
  allowed; the ADR's starting point is 25 / 100). **Both unset by default**,
  and then *nothing runs*: no transcript scan, no arithmetic — dormancy is
  exactly today's behaviour, and it is free. A value that is not a positive
  number is named on stderr and that window stays uncapped, the plan guard's
  rule for the same reason.
- **The windows.** `pass` is the spend of the beads *this pass* fired,
  measured from the moment `Run` starts — a fired bead burns tokens while
  the next one launches, so it genuinely grows within a pass and is not
  merely last pass's number. `day` is the local calendar day's bead spend,
  the same total the cockpit footer shows. Interactive sessions are in
  neither (Dial G: visible, never gated). One scan per bead feeds both.
- **The tightest window drives both rungs**, and when `plan_guard_*` is
  configured the plan's 5h/7d utilization joins the comparison as a third
  and fourth window — the soft landing before the guard's hard skip. No
  extra request is made for this: it reuses the reading the guard already
  took at the top of the pass.
- **At ≥80%** a session whose tier resolved to `standard` *by default* runs
  at `fast` instead, and the pass line says so
  (`[fast via budget step-down at day 85%, was standard via PID]`).
  `strong` never moves — Dial E is option (b): mechanical work slows first,
  and the quality of judged work is never traded silently. The step-down
  also yields to a tier pinned by `--tier` or a `tier:<x>` label (someone
  decided, and a budget is not an argument against a decision), to a PID's
  `tier_floor:`, and to §3's parity rule — no `--allow-degraded` at `fast`,
  and dispatch's own step-down has nobody to waive it. A blocked step-down
  is silent; the bead simply runs at the tier it resolved to.
- **At ≥100%** nothing more launches: each remaining bead gets one line
  naming the window, the numbers and the two ways out, and no bead is
  claimed or prompted. The reading is sticky for the pass — one scan says
  no, not one per bead.
- **Unlike the plan guard, the cockpit's `d` key is gated** by this one. The
  guard is fail-open monitoring, and halting the fleet on an unreadable
  endpoint would be absurd; a cap is a number the operator typed, and a cap
  the pass loop honours but one keystroke walks past is not a cap. The
  refusal names the way out, and raising the cap is a config edit away.
- **An unreadable ledger is not $0 spent** (ADR 0018 §3). `ScanCosts` used
  to drop every read failure on the floor (`segs, _ := ScanTranscript(…)`),
  so an unreadable transcript root read as a quiet day — an armed cap that
  counted nothing. The report now carries `ReadErr`/`Unread` beside what it
  did read, and `BudgetState.Unreadable` travels with the numbers, so nobody
  downstream mistakes a floor for a total. A root that does not *exist* is
  still no records: a machine that never ran the CLI has nothing to count,
  and calling that a fault would park a fresh instance on its first blind
  pass. Sighted passes name a read failure on stderr once per pass and run
  on the floor they could read; a **degraded** pass parks instead, because
  there its cap was the only thing counting. Same rule the overflow ledger
  already keeps.
- **And the listing is a walk, not a glob** (ranger-base-e06g). The first
  cut of the above guarded `filepath.Glob` with one `os.Stat` on the root —
  but Glob discards every I/O error by design (`path/filepath/match.go`
  glob(): "ignore I/O error"), and stat on a directory needs nothing but
  `+x` on its **parent**, so §3's own example (root `chmod 000`) walked
  straight past the guard and came back empty: $0 spent, honestly counted.
  `transcriptFiles` now walks with `os.ReadDir` at both levels and returns
  every failure that is not `IsNotExist`. A project dir that will not open
  costs its own spend, not the whole scan — the rest of the ledger is still
  the best floor available, and it travels with the fact that it is one.
  The general shape, worth the next reader's minute: **a listing whose
  failures are load-bearing cannot be a glob**, because Go's glob is
  specified to be silent about them.
- **Arming Dial E also chooses the blind policy** (ADR 0018 §1). Dial E
  computes from posse's own transcripts and needs no credential, so it works
  exactly when the plan guard cannot — which is what makes it the floor a
  blind guard can degrade onto. The coupling is deliberate and lives in
  code, not in a comment: the alternative was hand-tuning
  `plan_guard_blind_max:` whenever the caps changed, measured failing on
  2026-08-26 (three changes in two days, each on a wrong diagnosis).
- **Display** `posse cost` ends with the caps in force and the day spend
  against them (or "no caps set … dormant"); the cockpit footer shows
  `today $… of $… budget_day (NN%)`, and the header's blind segment names
  which policy is waiting — `guard blind 14m` parks, `guard blind 14m —
  ledger brake` degrades. Both read; only dispatch acts.

**verify-after** (ADR 0006 §3, `internal/rhq/verifyafter.go`) is the one
handoff the harness files rather than a persona. Every dispatch pass — right
after the plan guard, before ready work is gathered, so what it files is
dispatched by the same pass — and every `posse ready` sweeps each `beads:`
repo for beads that closed since that repo's watermark and carry a label in
config `verify_labels:`. Each one without a `qa` dependent earns
`bd create "verify: <title>" -l qa -a <verify_assignee> --deps
discovered-from:<id>`, and `verify filed: <qid>` goes back as a comment on
the close (or one bead per N closes — `verify_batch:`, below). The verify
bead's description is what makes it workable: the closer, the
`close_reason`, the commits `git log --grep <id>` finds in that repo, and the closer PID's `## Intents` "done when" row for the bead's
labels (`IntentDoneWhen` — a label matches an intent slug word, plural or
not, so `bug` finds `fix-bugs`; no match is an absent line).

- **Config** `verify_labels:` (absent → `code, devops`; present but empty →
  off, and then bd is not even asked) and `verify_assignee:` (the persona
  verify beads are assigned to). The verify bead inherits the closed bead's priority — a P1 fix
  earns a P1 verify — and is filed with bd actor `posse`, so `created_by`
  distinguishes harness-filed beads from a persona's own.
- **Config `verify_batch: N`** (default 1 — the 1:1 gate, unchanged) files
  ONE verify bead per N closes: title `verify N closes: <ids>`, one
  description section per close (the same closer / `close_reason` / commits /
  "done when" block the 1:1 bead carries), a `discovered-from` edge per
  close, `verify filed: <qid>` commented back on all N, and the priority of
  the batch's most urgent close. Coverage is unchanged — the same work is
  verified, in one session instead of N. What is divided is the FILING
  amplification, which is the point: `qa → code` is 0.49, so at 1:1 each
  close begets about one more bead of work and the queue's branching factor
  measured rho = 1.14, above 1.0 and therefore unbounded at any headcount
  (ranger-base-1t7r; N=4 → 0.875, ~8.4 seats). N > 1 is the operator's call,
  so the seed config ships it commented out and `TestSeedConfigArmsNothing`
  pins that.
- **A partial batch is held**, not filed short: filing every pass's leftovers
  would make N a ceiling rather than a quantum, and since most passes see a
  single close that is the 1:1 gate wearing a config key. The hold is bounded
  by `verify_batch_age:` (default 24h) on the batch's OLDEST close, so a shop
  that goes quiet three closes into a batch of four still gets those three
  verified. Holding needs no new store: the watermark below does not advance
  past a held close, so the pending set IS the closes it has not passed.
- **Watermark** `RHQ_HOME/state/verify-after.<repo>` holds the newest
  `closed_at` the sweep has accounted for. **The first sight of a repo files
  nothing**: the watermark is seeded to that repo's newest close, because
  "closed since the last pass" has no answer before the first pass and the
  literal reading (since the epoch) would answer a repo's whole closed
  history with verify beads. The watermark only advances past beads the
  sweep actually handled, so a bd failure means one bead is re-seen next
  pass, not lost — filing is idempotent through the `qa`-dependent check.
- **The sweep holds the launcher lock** (ADR 0011 §1, rangerhq-th7l). Both
  halves of the dedupe are check-then-act over a store that cannot do it
  atomically — read the watermark … write it, and `bd dep list` … `bd
  create`, with no create-if-absent to fold the second into one call — so two
  launchers that start within the same second both see a close as new and
  both file for it. A duplicate verify bead is a duplicate dispatch: the
  verify assignee prompted twice for one close, at full token price, with the second left
  ready when the first is closed. Hold time is a `bd list --all` per repo
  plus a create per new close. A lock that cannot be taken means the sweep
  files nothing and says so on stderr; no watermark moves, so the closes are
  seen again next pass.
- **Not a gate.** The verify bead never holds the builder's close (`bd
  ready` would start lying about what is done), nothing here reopens
  anything, and a closed bead that already carries the `qa` label is never
  verified — that is the one loop the rule could have. `--dry-run` files
  nothing: it shows routing without acting.
- A closer who filed the verify bead first is seen through the dependent and
  not duplicated; the convention path stays the primary one and this is its
  backstop, which is why builders' PIDs say *nothing to file* rather than
  *hand to qa* (ADR 0006 §4).

`--watch <interval>` (30s, 2m, or bare seconds) keeps passing: pass,
sleep, repeat. A quiet pass (nothing dispatched) doubles the sleep up to
`--max-interval` (default 8× the interval); a busy pass snaps back to the
base (`NextInterval`). SIGINT/SIGTERM end the loop *between* passes — a
pass in flight (`prompt --wait`) finishes first, so a persona mid-turn is
never left with a half-run pass. Pass errors are reported and the loop
continues; only the signal stops it.

`--watch` is also what *defines* unattended: it is the only thing that sets
`Dispatcher.Unattended`, which is what lets the plan guard fail closed after
its blind window (rangerhq-6h1). Deliberately not a TTY check — the premise
is that a fail-open stderr line has a witness when a human typed the command
and none when a timer did, and that is knowable at the loop, not from a file
descriptor.

**Scheduled dispatch** (rangerhq-snd) is that loop, started for you. The
runner is a **herdr workspace running `posse dispatch --watch`**, created by
the cockpit plugin's `[[startup]]` hook (`plugin/autostart.sh`, run once per
herdr server start, cwd = plugin root). Not launchd: `posse dispatch` is
meaningless without a herdr socket and a login-time job would spin failed
passes until herdr came up, and — the deciding half — a launchd job has no
cockpit row, no `posse peek`, and no `x`. Unattended dispatch is precisely the
thing that must stay in the operator's field of view. "Survives reboot"
therefore means "comes back when herdr does", which is the right lifetime
for a fleet: no herdr, no fleet.

- **Disarmed unless the instance says otherwise.** The hook is a no-op
  without `autostart_interval:` in `$RHQ_HOME/config.yaml` — presence of
  that key is the arm switch. Beside it: `autostart_max_interval:`,
  `autostart_max_beads:` (`-n`), `autostart_dry_run:` (passes route and
  report, dispatch nothing — the observation gear), `autostart_resume:`,
  `autostart_session:` (default `dispatch`), `autostart_dir:`.
- **The default herdr server is the only owner.** The plugin registry is
  global, so herdr also runs `[[startup]]` on every
  `herdr --session <name> server`. Those named/non-default servers stand down
  before probing the watch lock or invoking `posse`, even if they
  inherited the fleet `RHQ_HOME` and its armed config (ranger-base-87q).
  `HERDR_SOCKET_PATH` is authoritative when present; with no socket path,
  `HERDR_SESSION` still identifies a named server. Running the hook explicitly
  by hand retains its existing targeted behavior.
- **`autostart_resume:` is the one autostart key that defaults ON**
  (ranger-base-f0g). `--resume` re-prompts an in_progress bead whose persona
  session is alive and idle; without it the loop prints `◑ … settled "done"
  but issue is "in_progress" — review <session>` and moves on. Measured
  2026-08-24: three dispatched sessions in a row finished, went idle without
  commenting or closing, and their beads sat open behind a busy key until the
  operator re-prompted by hand — one of them holding 353 uncommitted lines a
  reap would have destroyed. A warning is addressed to a reader; this loop's
  whole premise is that nobody is reading. The token-burn rangerhq-zom guarded
  against is bounded by bd's own readiness: a persona that files a question
  and deps its bead on it leaves the ready set and is never re-prompted. A
  persona that settles open with nothing filed is re-prompted every pass —
  which is the polite-infinite-retry ranger-base-9hm is about.
- **The fan-out cap is always present.** `autostart_max_beads:` raises or
  lowers `-n`; it does not switch it on. Absent, the hook passes `-n 3`
  (rangerhq-v83) — an armed loop that fired the whole ready queue in one
  pass would be the worst case reached by omission, and at the measured
  (instance-side) median cost per dispatched bead, firing a 20-bead queue
  in a single pass would eat a large fraction of a 5h window. `autostart_max_beads: 0` still means unbounded,
  for whoever wants it, explicitly. A value that is not a count is named on
  stderr and replaced with 3, because `-n` would otherwise parse it as 0.
- **The plan-utilization guard is the point.** `plan_guard_5h` /
  `plan_guard_7d` run at the top of every pass; arming without them is
  arming a token loop nobody is watching (the analyst, rangerhq-jgm). The guard is
  fail-open for the first `plan_guard_blind_max:` (10m) of blindness and
  then skips the pass, *because* this loop is the unattended one
  (rangerhq-6h1) — a guard that is off for an hour here is indistinguishable
  from a guard that is on and under threshold, which is the whole reason the
  clock exists. The cockpit header says `guard blind 14m` while it lasts.
- **One loop, and the husk problem.** herdr restores its workspaces when the
  server restarts but does *not* re-run what was in them, so at server start
  a session named `dispatch` is usually a bare shell wearing the loop's name.
  Usually — herdr runs `[[startup]]` hooks on a **live handoff** too, and
  there the workspace comes back *with* its command still running (same pid,
  PTY fd passed across). The two are indistinguishable from outside, so the
  hook asks the loop instead of the workspace — and since rangerhq-gir5 it
  asks the **kernel**, not a file. `posse dispatch --watch` holds `flock(2)`
  on `$RHQ_HOME/state/dispatch-watch.lock` for its whole life (one file per
  RHQ_HOME, because the invariant is one loop per *queue*, not per session
  name), and `posse dispatch --watch-status` reports it on one line:
  `watch-loop: running (pid N, since T)` or `watch-loop: none (… is free)`.
  Held is a running loop; free is none; the kernel releases the lock when
  the process dies, so a pane killed with `kill -9` reads dead in the same
  instant and there is no stale state to reap. A second `--watch` on the
  same RHQ_HOME now *refuses* rather than double-dispatching the queue.
  **The line is the contract, not the exit status**: a posse too old to know
  the subcommand prints nothing that matches, which the hook treats as
  "could not ask" and stands down on — unarmed is visible and recoverable,
  double dispatch is neither (rangerhq-ct9/mugy).
  `dispatch-watch.pid` (`pid:`, `started:`, `cmd:`) is still written and
  still cleared on a clean exit, demoted to the **identity** half: which pid,
  since when, under what argv. It is what the stand-down line quotes once
  the lock has answered, and a missing or stale one costs a name, never the
  answer. What that retired: `kill -0` plus a grep of `ps -o command=`, which
  had to reconstruct liveness from a file whose truth decays and leaked three
  ways doing it — a recycled pid read as alive, a one-shot
  `posse dispatch --persona` whose argv merely contained the word read as the
  watch loop (rangerhq-ppy9), and a `ps` that could not answer read as alive
  or dead depending on which arm you wrote (rangerhq-mugy, ranger-base-rmc).
  A live loop is never replaced, whatever the workspace looks like; run by
  hand the hook is conservative either way. A loop started by a posse old
  enough not to hold the lock reads as dead and is replaced once, at the next
  server start — both halves come from the same promoted `./bin/posse`, so
  that window is the one start after an upgrade.
- **Logs** `$RHQ_HOME/state/dispatch-watch.log`, tee'd from the pane and
  rotated to `.1` past 5 MiB; the pane's own scrollback is the live view
  (`posse peek dispatch`). The session wears 🛰️ and, being made by `posse new`,
  the crew mark — dispatch will never reach into its own engine.
- The manifest is read from disk at every server start (the cached
  `plugins.json` entry carries only the registration), so a manifest edit is
  live on the next herdr restart — no relink needed for `[[startup]]`.

**Metas are never pruned against the wrong server.** `Sessions()` deletes a
meta whose workspace is gone, which is right only when the herdr answering
is the herdr that held it. Three situations look identical to "everything
died" and never are: an **empty workspace listing**; a meta written against
a **different socket** (`socket:` in the meta, `$HERDR_SOCKET_PATH` at
creation); and a meta recording **no socket at all**, which names no server,
so no listing is evidence about it (rangerhq-8fq). All three keep the file,
leave the session out of the listing, and say so on stderr. A meta with no
socket would then be kept forever, so the server that finds its workspace
*live* backfills `socket:` on the way past — holding the workspace is the
proof the file never carried, and no session has to be recreated to get
under the guard. A pass whose `$HERDR_SOCKET_PATH` is unset stamps nothing:
"" is the default server, which on disk reads the same as unrecorded
(rangerhq-y4z). This is not hypothetical — a `--watch` pane inherits the
*herdr server's* environment, not that of the command that created it, so a
scratch server started without `RHQ_HOME` ran a pass against the fleet's
real state and deleted eleven live sessions' metas in one read (rangerhq-snd
incident). Anything testing against a scratch herdr sets a scratch `RHQ_HOME`
on **both** sides: the server's env and the caller's.

**A live id is not an identity.** Past the socket guards, `workspace get`
answering `alive` proves a workspace holds that id, never that it is the one
the meta recorded — herdr re-issues ids across a server restart and a handoff
(see "Workspace ids recycle across a server process boundary" below). So a
meta whose id a *stranger* now holds is left out of the listing and kept, and
the same predicate refuses the create: identity is checked with the
workspace's `label` (posse creates every workspace with `--label <session
name>`, so a workspace whose label is not the meta's name is not the meta's
workspace) fenced by `gen:` — the device+inode of the api socket, which herdr
recreates at exactly the moments the allocator resets. Inside one generation
ids are never re-issued, so a label mismatch there is a *rename* and the
session keeps its name; across generations the two readings are
indistinguishable and the ambiguous case does nothing to the file. `gen:` is
backfilled like `socket:`, but only on positive identity (the live workspace
wears the meta's own name) — a forged fence is worse than an absent one
(rangerhq-yt1p).

`make verify-prune-guard` is the behavioural check on a *binary*, and the
gate before a promote (rangerhq-m15): it plants four metas — one foreign
socket, one socket-less, one naming a workspace the live herdr really holds
*under that workspace's own label*, and one claiming that same live id under
a name that is not its label — in a scratch `RHQ_HOME` and lists them against
the live server, asserting both refusals, the backfills, and that the
stranger-id meta is kept but never listed. The live server is only ever read,
and every write lands in the scratch home; that is the one safe direction (scratch
`RHQ_HOME` on the process *we* run), and the exact reverse of the snd trap.
`RHQ=bin/posse-release make verify-prune-guard` tests a promote candidate
before `make install`; with no `RHQ=` it tests whatever PATH resolves, which
is the fleet's binary. A pre-abb2716 binary fails it on the socket-less arm.

## The cockpit is a herdr plugin

`plugin/herdr-plugin.toml` declares a popup pane running `posse cockpit` — a
session-modal overlay (like herdr's lazygit example). Register it against
the running herdr with `make link-plugin`; open it from herdr's plugin UI,
a bound key, or `herdr plugin pane open --plugin posse.cockpit
--entrypoint cockpit`. Plugins get the full herdr CLI as their API
(`HERDR_BIN_PATH`, `HERDR_SOCKET_PATH` injected), so the cockpit is just
`posse` running in a pane. Pane commands run with cwd = plugin root, and the
manifest runs `./bin/posse` — a gitignored symlink `make link-plugin` points at
the installed binary (`$BINDIR/posse`) — so the popup runs the promoted build,
not whatever PATH or a persona's last `make build` produced (rangerhq-8te).

The cockpit is a small raw-mode TUI (only dep: `golang.org/x/term`):
sessions sorted blocked-first with live agent state, the aggregated ready
queue beneath. Keys: `j/k`/arrows move, `tab` jumps sections, `enter`
focuses the selected session's workspace and exits (the popup closes,
revealing it), `p` prompts its agent inline, `v` peeks its terminal tail,
`x` kills (y confirms), `c` claims the selected bead, `r` refreshes, `q`
quits. Non-tty stdin falls back to a display-only refresh loop (that's
what tests and pipes see).

## The YAML subset

Config, recipes, meta files, and agent frontmatter all use the same flat
subset (`internal/rhq/yamlflat.go`, ~100 lines, no deps):

```yaml
key: value          # scalars; "null", "~" and empty mean unset
key: [a, b, c]      # inline lists
key:                # block lists
  - a
key:                # one-level maps
  sub: value
```

Top-level keys only (plus one nesting level), `#` comments, a wrapping
pair of double quotes stripped (a lone leading or trailing `"` is data —
`command: … "$(cat {file})"` keeps its quote), no anchors/multiline/deeper
nesting.

## Env sets and secrets

Env sets are `envs/<name>.env`, plain `KEY=VALUE` lines (leading `export `
tolerated, `#` comments skipped). Rules:

- `posse init` sets `envs/` to mode 700 and the files to 600, and every
  launch re-asserts it (`TightenEnvPerms`: a drifted 644 file is chmod'ed
  back and named on stderr — names only).
- Values go **only** into the workspace's environment (`--env` at creation);
  never echoed into a shell, never in titles or listings — those show
  env-set *names* only.
- **An env set is readable by the agent in that session** — and by every
  tool it runs. Treat every value in it as something the persona may
  quote, log, or commit; the harness cannot tell "secret for the tool"
  from "secret the agent may read". So persona sessions never receive
  config `default_env` implicitly: a persona gets exactly the sets its PID
  names in `envs:` plus whatever a recipe/`--env-file` adds explicitly
  (rangerhq-f2b). `default_env` applies to plain sessions only.
- Known exposure, accepted for a local single-user tool: `--env KEY=VALUE`
  is argv to herdr for milliseconds (visible in `ps` on a multi-user box).
  herdr does not persist the values (audited: none in its session.json or
  logs).
- If a secret shouldn't sit in a plaintext file at all, put
  `op run --env-file=… -- <cmd>` (1Password) or similar in the recipe's
  `command:` and let the secret manager inject at process start.

## Personas

`agents/<name>.md`: flat-YAML frontmatter (`name`, `description`,
`runtime`, `labels`; PID keys `intents`, `allow`, `deny`, `metrics` —
see `docs/adr/0001-persona-intent-documents.md`; `envs`, the env sets its
sessions receive; `tier`/`tier_floor` (ADR 0003); `cage`, `writable`,
`egress` (ADR 0002 §5 — `cage:` is the minimum wall tier, `egress:`
implies `container`; the example PIDs that deny Edit/Write declare
`cage: seatbelt`); `skills` (ADR 0007 — see **Skills** below);
`command`, the escape hatch) + a markdown body that
*is* the prompt. Body sections are the ADR 0001 contract order plus an
optional `## Work prompt` (ADR 0005): the persona's standing per-bead
instruction, appended verbatim to every dispatched work prompt.

**Runtimes (ADR 0002 §1–2, `internal/rhq/runtime.go`).** A persona
launches on a named launch profile, not a command string. Built-ins
`claude`, `codex`, `grok` each carry a template and a *native realizer*
for `{allow}`/`{deny}`: claude → `--allowedTools r…`/`--disallowedTools
r…`; grok → `--allow r`/`--deny r` per rule; codex → `-s read-only` when
the deny list covers `Edit`+`Write`, else `-s workspace-write` (always
emitted; codex has no verb rules). Native flags are politeness (L0), not
the wall — the wall is the gates bead (§3). `RHQ_HOME/runtimes/<name>.yaml`
(`command:` only) adds a template-only runtime with no realizer:
`{allow}`/`{deny}` render to nothing there and every gate goes to the
wall.

Claude's `{deny}` is *widened* on the way out (`L0Spellings`,
rangerhq-3mc). Claude matches a `Bash(...)` rule three ways — exact,
`:*` as a literal prefix of the command string, and `*` as a wildcard
(`.*`, anchored, whitespace collapsed) — so `Bash(git push:*)` never
matched `git -C <repo> push`, and the polite refusal was missing on
exactly the spellings the L1 shim has to catch. Each subcommand deny now
also ships its option-blind pair, `Bash(git -* push)` and `Bash(git -*
push *)`; a whole-verb deny (`Bash(bd)`, which claude reads as *exact*)
also ships `Bash(bd:*)`. Verified on claude 2.1.234: nine option
spellings refused, and `git -c … commit -m "push it"`, `git --no-pager
log -- push.txt` and `git pushy` still run — the token boundary in the
pair is what buys that, and is why it is a pair rather than one `git -*
push*`. `allow:` is never widened (that would grant more than the PID
says), `RHQ_TOOLS_DENY` still carries the PID's own rules, and parity is
unchanged: L0 is politeness, never the wall. Known false positive in the
pair: a command that starts with `git -` and *ends* with the bare word
`push` — `git -C <r> log --grep push` — is refused, while `--grep=push`
runs (rangerhq-ky3). Grok's realizer deliberately does **not** do this:
its dialect is verified now (rangerhq-625) and the wildcard is real, but
grok matches a shell-parsed segment with the quotes off, which turns the
pair into a false-positive generator there — see *Grok specifics*.
Templates:

```
claude: claude {model} --append-system-prompt "$(cat {file})" --add-dir {memory} --settings '<fleet>' {skills} {allow} {deny}
codex:  codex {model} {skills} {deny} -a never --disable hooks -c allow_login_shell=false -c "projects={\"$PWD\"={trust_level=\"trusted\"}}" -c developer_instructions="$(cat {file})"
grok:   grok {model} {skills} --permission-mode auto --rules="$(cat {file})" {allow} {deny}
```

**The dispatch contract (ADR 0013, `internal/rhq/runtimecheck.go`).** A
runtime that *launches* safely is not the same claim as a runtime that can
*take work*, and one evening of two non-claude runtimes in production broke
the second claim about once an hour. So dispatch names six stages —

```
launch → promptable → work → record → settle → account
```

— of which four are observed (herdr, the bead, the cost adapter) and two
are **declared** per runtime, in the built-in table or in
`runtimes/<name>.yaml`:

| key | values | what it says |
|---|---|---|
| `prompt:` | `typed` (default) / `argv` | how dispatch delivers the work prompt: type it into a promptable screen, or append the prompt file to the launch line so no screen is the delivery channel |
| `startup_wait:` | duration, default 45s | how long a launch may take to reach a promptable screen. Measured per runtime — 45s is a *claude* number |
| `record:` (+ `record_why:`) | `untrusted` (default) / `trusted` | whether a **dispatched** session of this runtime has been MEASURED to close its bead |
| `native_rules:` | file names | the rulebooks this CLI discovers by itself, ahead of anything posse types |

`posse runtime check <name>` prints the grid: each stage's observable, who
declared it, and what a missing one costs — always a named degrade or a
named refuse, never a patch. **This is how a runtime is onboarded**: fill
the grid rather than discover each quirk in production.

The unknown runtime is the case the command exists for, and the zero value
of every declaration is the expensive-to-be-wrong-about direction. A
`runtimes/<name>.yaml` with nothing but `command:` is `prompt: typed`,
`record: untrusted`, uncounted and tier-unmapped — **dispatchable and
loud**, every row naming the key that would change it. A *present but
misspelled* value is the opposite case and refuses at load: `record:
trused` silently reading as untrusted is exactly the silence this contract
removes.

`native_rules:` is a declaration, not a switch — and codex 0.147.0 *has*
a switch (`-c project_doc_max_bytes=0` drops the project `AGENTS.md` from
the model-visible prompt; measured, `ranger-base-cl7`) that dispatch
deliberately does not use (ADR 0013 §4, decided on `ranger-base-00f`):
grok has no equivalent so nothing fleet-wide would be gained, the key
could rename and silently re-enable discovery, and the file is the
operator's — suppressing it only under dispatch would make posse sessions
disagree with hand-run ones about which of the operator's documents
apply. An operator who wants codex doc-free owns that choice:
`project_doc_max_bytes = 0` in `~/.codex/config.toml` (instance-wide), or
the `-c` override in their own `runtimes/<name>.yaml` `command:`. Posse
documents the key and never writes it.

### Argv delivery: the work prompt rides in on the launch line

The probe (`ranger-base-cl7`, trace in `docs/adr/0013-argv-prompt-probe.md`)
held on both non-claude runtimes on 2026-08-25, so **grok and codex are
`prompt: argv`** and claude stays `prompt: typed` (it works; ADR 0013 calls
argv there an allowed later unify). Neither argv runtime carries a
`startup_wait:` — that number is the *typed* ladder's patience, and they do
not use that ladder.

Why it is not a nicety. A grok pane that has not had a turn emits no OSC
title, no OSC progress and no composer footer, so it matches **no** herdr
rule and herdr answers with its idle guess; the same pane after one turn
matches three rules at once. Detectability is a property of *having been
prompted* — so there is no value of `startup_wait:` that turns waiting into
a promptable screen there, and the typed fallback ADR 0013 §2 describes
could not have been made to work with a bigger number.

The path, in `dispatch.go` (`launchWithPrompt`) and `argvprompt.go`:

1. **Claim first** — the fence. A lost claim creates nothing: no workspace,
   no prompt file, no persona sitting in a repo working someone else's bead.
2. **Write** the assembled ADR 0005 work prompt to
   `$RHQ_HOME/state/prompts/<session>.txt` (0600). The prompt does not go on
   the line itself: it is a page of text and a fresh pane's shell is a tty
   with a line limit (rangerhq-ybec).
3. **Create** the session with `"$(cat <file>)"` appended to the *runtime's*
   rendered command — before the seatbelt prefix, the gates prefix and the
   cage wrap, because it is an argument to the CLI and not to `sandbox-exec`
   or `docker run`. There is no `{prompt}` placeholder: an unrendered token
   would be a literal argv (ADR 0001/0002's lesson, and ADR 0013 rejects it
   by name). At the container tier the file is mounted same-path read-only,
   since that `$(cat …)` is expanded by the container's own shell.
4. **Await a state herdr has SEEN** — a matched rule, not the idle fallback.
   This is not a readiness gate; nothing is waiting to be typed. It is the
   evidence that the launch line ran and a turn started.

Three failures, three different cleanups, and the differences are the point:

- **create fails after the claim** → unclaim. The claim went first so a race
  loses cleanly; the price of that is handing the bead back.
- **no agent pane ever appears** → the CLI never started, nothing read the
  prompt file, nobody is working the bead → unclaim.
- **a pane, but no rule matched in time** → the prompt IS with the CLI.
  Claim kept, bead not judged this pass, and **no settle-wait is started**:
  a wait over herdr's idle guess returns instantly and would read a session
  that never worked as one that settled.

The prompt files are left behind on purpose. A typed prompt at least echoes
into the pane's scrollback; an argv one is consumed by the exec before any
screen exists, so the file is the only record of what a session was asked.
One small file per bead, rewritten on re-dispatch.

**Resume is still typed.** `--resume` into a live session, and the cockpit's
`d` on a live holder, prompt the composer: the launch line has already been
typed, and the only way into a running CLI is through its screen.

### When a launch is not promptable, say what herdr was looking at

Argv retires the screen as the *delivery* channel; it does not retire the
screen. A launch can still end with herdr recognizing nothing, and until
`ranger-base-3j8` the only thing dispatch said about that was:

```
herdr never saw a screen it recognizes there, only "idle"
(default_known_agent_idle_fallback)
```

Honest, and useless. Three different screens produced that identical
sentence in one evening — a consent banner, a version splash, and a pane
whose OSC chrome grok had simply not emitted yet — and two of them needed
opposite fixes. Telling them apart cost a hand-launch and a `posse peek`
each time.

herdr already had the answer. `agent explain --json` carries
`evaluated_rules`: every rule it tried, the screen **region** that rule
reads, and how many bytes were in that region with a preview. Both
promptability failures (typed `awaitSettled`, argv `awaitDelivered`) now
append it, grouped by region — rules outnumber regions three to one and
share their evidence, so twelve rows of which eleven repeat is a
diagnostic nobody reads:

```
herdr evaluated 15 rules there and matched none. What it was reading:
  osc_title                      0 bytes  ""  — osc_title_blocked, osc_title_idle, osc_title_working
  bottom_non_empty_lines(2)    601 bytes  "╰── Grok 4.6 (high) ─╯ ..."  — permission_hints_blocked, +3 more
  whole_recent                3969 bytes  "\ue0a0 main ~/src/posse ╭──╮ │ ..."  — startup_splash, +4 more
```

Read it as: **an empty region means the CLI has not spoken yet**; a region
full of text means it is up and parked on a screen posse cannot name. On
grok those are the two halves of this bead. The row above is the real one,
and `╰── Grok 4.6 (high) ─╯` with no `· auto` is the tell the coordinator
found by hand.

Runs of four or more identical non-alphanumeric runes collapse to two
before the preview is cut: on the wide grok splash, 70 of the first 72
characters were one repeated box rule, so the row previewed nothing.
Letters and digits are never collapsed.

This is **diagnosis only**. Nothing decides on it and no key is pressed
because of it — interstitials stay on the argv sidestep and the operator's
own config (ADR 0013 §2, and the interstitial section below). A herdr that
does not emit `evaluated_rules` leaves the old sentence exactly as it was.

### The busy key: session failure vs persona failure (ADR 0013 §2)

Dial F gives every bead its own session, so a *pane* failing is not a fact
about the *persona*. Dispatch splits the three outcomes of a failed launch:

| outcome | what it is | slot |
|---|---|---|
| claim lost | the **bead**'s — somebody else holds it | free |
| session failure — no agent, never promptable, unknown screen | this **pane**'s | **free**; the next bead gets its own fresh session |
| anything else — runtime will not load, exe missing, cage credential, gates the wall cannot realize | the **persona** on this runtime | benched for the pass |

A pane the pass gave up on is also remembered for the pass, so the
working/blocked guard does not read a session left sitting on a splash
(which herdr reports `blocked`) as the persona being busy — that would put
the sterilise back one guard further down.

**The cost, and its ceiling.** Before the split, one failed launch benched
the persona and the pass paid one startup wait; the split let every bead's
own launch pay its own — deliberately, because one grok cold start taking a
persona's whole queue out of the pass was the failure being fixed
(`ranger-base-3j8`), superseding rangerhq-vk2's "one detection timeout per
pass" rule from the one-session-per-repo era. Unbounded, though, a persona
whose CLI is broken (exe missing, instant crash, auth exit — all of which
look exactly like "no agent detected" from herdr's side) spends a
serialized startup wait per ready bead, and `-n` defaults to unlimited. So
the pane-local explanation gets exactly one retry: the **second** session
failure of a slot in one pass benches the slot for the rest of the pass
(ADR 0013 §2 "Ceiling", `ranger-base-8h5p`). Two is the floor that keeps
the 3j8 fix and the ceiling that caps the drain; the benign slow-start case
lands in `awaitDelivered`'s seen=false outcome on argv runtimes, not here.
An exec-preflight gate was rejected in the same amendment — posse's PATH is
not the pane's — see the ADR's alternatives. (Implementation:
`ranger-base-4ctv`.)

`record:` is where grok and codex differ, and only for a measured reason:
the qa lane on grok closed a dispatched bead properly, and 3/3 dispatched
codex sessions did the work and left the bead `in_progress` with no comment
(`ranger-base-0fb`). Trust is a built-in/yaml edit after a measurement —
never a derived store that could disagree with the bead (ADR 0011).

### What a settle-without-close costs, and what it does not (ADR 0013 §4)

The tick is the **bead's** to give. A pass prints `✓` only when `bd show`
says `closed`, on every runtime, trusted or not — an agent going idle is a
hint (ADR 0011), and on codex that hint was wrong three times out of three.
Everything else settles as `◑ <id> settled "done" but issue is
"in_progress" — review <session>`, and on a `record: untrusted` runtime the
line adds whose declared degrade it is:

```
◑ a-1   settled "done" but issue is "in_progress" — review ranger-posse-a-1
        (codex is record: untrusted — the claim is kept and --resume re-prompts it)
```

Three things that clause is telling the operator, and each of them is a
decision made elsewhere in this file: the bead **keeps its claim** (nothing
is handed back on a settle a pass could not judge), unattended `--resume`
**re-prompts the same session** next pass rather than parking it behind a
busy key (`autostart_resume:` above), and the harness **does not close the
bead** on the agent's behalf — that hides the defect and puts a human back
in the loop dispatch exists to replace. A `record: trusted` runtime gets the
same honest `◑` and no clause: a runtime measured to close its beads that
has stopped closing them is a signal, not a footnote.
(`internal/rhq/recordskip_qa_test.go`.)

### The reap guard: dirty tree + open bead is not killed (ADR 0013 §4)

`posse kill <name>` — and the kill inside `posse relaunch` — **refuses** a
session whose `bead:` is still `in_progress` and whose working directory
holds uncommitted work:

```
NOT killed: ranger-posse-a-1 still holds a-1 (in_progress) and ~/src/posse has
uncommitted work (dispatch.go NOTES.md) — look first (posse attach ranger-posse-a-1),
or `posse kill ranger-posse-a-1 --force`
```

The near-miss it is built from is `ranger-base-0fb`: a dispatched session
that skipped its bookkeeping *and* left 353 uncommitted lines in the
**shared** checkout, one reap away from gone. Per-session worktrees do not
cover that board — a shared-checkout session has no branch, so the landing's
existing "a tree still holding work is kept" refusal never fires — and L3's
pathspec rule stops an unqualified commit, not a kill.

Both arms must fire, and neither is a new store:

- the **bead**, read from bd every time. `bead:` in the session meta is only
  a *pointer* to which one, stamped by dispatch at launch (and on the
  session it resumes into), so the meta can never disagree with the store
  about whether the work is done. A session with no pointer — `posse new`,
  a crew session, a recipe, anything from before this landed — is unguarded,
  exactly as it was.
- **git**, `status --porcelain` in the session's own cwd. Uncommitted is the
  only shape of loss a kill can cause: committed work survives on a branch,
  and a clean tree has nothing to lose however open its bead.

An open bead over a clean tree is a bookkeeping skip — gather's line to
print and `--resume`'s to retry, not a reason to keep a workspace alive. A
dirty tree under a closed bead is the operator's own scratch. Ignorance
*inside* the pair fails **closed**: a pointer, a dirty tree and a bd that
will not answer is refused too, on `RemoveSessionTree`'s rule — the safe
answer to an unanswerable question about destroying work is no, and the
costs are asymmetric (a wrong refusal is one `--force`; a wrong kill is
work with no other copy). `--force` on either command stands the guard
down and nothing else — the landing still refuses to *remove* a tree that
holds work. (`internal/rhq/reapguard.go`, `reapguard_qa_test.go`.)

Which runtime a session gets: `posse new --runtime` / `posse dispatch
--runtime` > recipe `runtime:` > PID `runtime:` > config
`default_runtime:` > `claude`. A PID's `command:` is the template for
*its own* runtime only; an override to another runtime uses that
runtime's built-in template (a claude-shaped `command:` on codex would be
nonsense). PIDs should say `runtime:` and drop `command:` — the scaffold
and `examples/agents/*` do. Persona sessions get `RHQ_RUNTIME` in the
env, `runtime:` in the session meta (`RelaunchAgent` re-renders for the
same runtime after a crash), and the runtime's emoji from config `emoji:`
so the cockpit shows what the persona runs on (`🎭name@codex` when not
claude). `posse runtimes` lists profiles. Verified flags on this machine:
codex — see **Codex specifics** below; grok — see **Grok specifics**.

**Skills (ADR 0007, `internal/rhq/skills.go`).** `skills: [dataviz,
code-review]` on a PID binds those skills to the persona on whatever it
launches as — the cross-agent binding posse owns, as against the
per-user-per-runtime accident of "whatever this machine installed into
this CLI". A name resolves to `RHQ_HOME/skills/<name>/SKILL.md` or it is
unknown; that directory *is* the registry (real Agent-Skills dirs or
symlinks to `~/.claude/skills/x`, a plugin's `skills/x`, a repo — posse
copies nothing and indexes nothing, so `posse skills` is `ls` and `posse
agent check` is `stat`).

At launch the binding is materialized fresh, exactly as the gates are, in
one of two shapes — which one is a property of the *runtime*, not of the
PID:

- **A flag at a rendered tree (claude).** `RHQ_HOME/state/skills/<persona>/claude/`
  gets `.claude-plugin/plugin.json` plus `skills/<name>` symlinks to the
  originals, and `{skills}` renders `--plugin-dir <that dir>`
  (session-only, additive, verified: `claude --plugin-dir <tree> plugin
  details posse-<persona>` lists the bound skills). `--add-dir` is CLAUDE.md
  dirs and does **not** load skills. A `runtimes/<name>.yaml` opts into the
  same shape with `skills_flag: --foo` (the flag name only, as
  `model_flag:`) and is handed the same dir — the layout inside it is the
  universal Agent-Skills shape and the plugin.json is inert to anything
  that does not read it.
- **No flag, symlinks in the session dir (codex, grok).** Both CLIs
  discover skills from their *working directory*, so the launch links
  `<session dir>/.agents/skills/<name>` → `RHQ_HOME/skills/<name>` and
  adds `/.agents/skills/` to the repo's `.git/info/exclude` — never the
  repo's own `.gitignore`, which is the operator's file. `{skills}` renders
  nothing there: the links *are* the realization
  (`Runtime.SkillsCwd`, `App.RenderAgentsSkills`). See **Skill surfaces**
  below for what else was tried.

An empty `skills:` renders nothing, placeholder and space alike.

**Declared means required.** `skills:` is a statement that the persona's
work depends on them, so it goes through the same parity gate as a wall
rule: a runtime with no surface adds `skills: <names> — <runtime> has no
per-session skill surface` to `Degraded` — today only a template-only
`runtimes/*.yaml` that names no `skills_flag:`, the three built-ins all
materialize — and the launch refuses unless
`--allow-degraded` (the session is then marked in meta and cockpit, like
any degraded launch). It is *not* filed under `Unrealized` — nothing is
being enforced here. A name that resolves to nothing refuses the launch
outright rather than binding a dangling symlink; `posse agent check` finds
it first, along with a PID whose own `command:` forgot `{skills}` while
`skills:` is non-empty (the `{model}` rule again: never leave a token
unrendered, never silently skip one). Binding is additive — the
runtime's global skills still load; posse guarantees presence, not absence.
Isolation is a cage question, named out of scope by the ADR.

**Skill surfaces (rangerhq-1qd, verified 2026-08-18 on codex-cli 0.147.0
and grok 1.0.5).** The cwd shape is not a fallback — it is the only
per-session surface either CLI has, and it happens to be the same one, so
one realizer serves both. What was checked and is *not* there, so nobody
re-checks it:

- **codex has no config key for extra skill roots.** `skills` is a real
  config table (`-c skills=1` errors with "expected struct `SkillsConfig`",
  which is how you tell a recognized key from a silently-dropped one), but
  its only field is `bundled.path` — where the shipped system skills live,
  not an added root. The app-server's `skills/extraRootsSet` JSON-RPC
  method exists and is not reachable from the CLI. `codex debug
  prompt-input` renders the `<skills_instructions>` block the model
  actually sees, which makes every one of these a zero-cost check.
- **codex reads `<cwd>/.agents/skills/`** (and `<cwd>/.codex/skills/`;
  `.agents` is the vendor-neutral one), follows symlinks out of the repo,
  and keeps reading them under the full fleet line (`--disable hooks`,
  `allow_login_shell=false`, the trust table, `-s read-only`). It does
  **not** climb from a subdirectory to the repo root — the links have to go
  in the directory the session starts in.
- **grok's `[skills] paths` cannot be injected per session.** It is a
  `config.toml` key, and the one config layer a launcher can inject —
  `GROK_CONFIG` / `GROK_CONFIG_PATH` — is allowlisted to `models`,
  `features`, `toolset` and `shell_environment_policy`, dropping every
  other table by design ("cannot add a discovery source"). Verified: the
  overlay leaves `grok inspect`'s skill list unchanged.
- **grok's `--agent <definition>` has no skill-path field.** A definition
  carrying `skills:` / `skill_paths:` parses fine and binds nothing
  (checked against a headless session's `init` line, which advertises the
  session's skills). `grok inspect [--json]` is the zero-cost probe here.
- **codex silently skips a `SKILL.md` with no `description:`.** It never
  reaches the prompt and nothing says so — grok and claude fall back to the
  body's first paragraph, codex does not. A bound skill missing that one
  frontmatter line is bound to nothing on codex (rangerhq-1qd; a lint for
  it is filed separately).
- **grok reads `<cwd>/.agents/skills/`** as `project` scope, and its skill
  discovery deliberately ignores git's ignore rules — so `.git/info/exclude`
  hides the dir from `git status` without hiding it from the CLI. (Both
  CLIs behave the same way about this; it is what makes the exclude honest
  rather than a trick.) `internal/rhq/skills_e2e_test.go` re-runs the whole
check against the installed CLIs (`RHQ_E2E=1 go test ./internal/rhq/ -run
E2ESkillSurfaces`) — worth doing when either updates, since this is a
discovery convention, not a documented flag.

The cwd dir belongs to the **repo**, not to the persona, which is the one
asymmetry with claude's tree worth remembering: a launch adds its own links
and leaves other personas' alone (binding is additive — presence, not
absence), sweeps only its own links whose skill has left `RHQ_HOME/skills`,
and *refuses* rather than overwrite an entry posse did not write. A name
dropped from a PID therefore keeps its link in a repo where another
persona still binds it — harmless by ADR 0007 §4, and the price of a
shared directory.

Every persona session carries `RHQ_SKILLS_DIR` (`RHQ_HOME/skills`) and,
when the PID binds any, `RHQ_SKILLS` (newline-separated names): the exit
hatch, so a runtime posse cannot point can still be *told* where they are,
by the PID's body or by a wrapper. `posse skills` lists the names with the
PIDs that bind each, and flags names a PID declares that nothing answers.

**Authoring a skill** (`distributed-systems` is the pattern to copy —
rangerhq-gsy3). `SKILL.md` frontmatter is `name:` + `description:`, and
the description is mandatory — codex silently drops a skill without one
(rangerhq-1qd) — and is also the *entire* per-session cost (~110 tokens:
the always-loaded advertisement), so write it trigger-shaped. The body is
a short index (one line per concept, a "how to use in a bead" note), and
the content lives in `references/<concept>.md` files loaded one at a
time on demand — never paste a textbook into the front matter; the whole
corpus should only ever load a file at a time (~0.8–1k tokens each).
Honesty rules from the first skill: every claim carries a primary source
verified *live* on the authoring date (the model's recall is not a
source), labeled [paper]/[docs]/[blog]; where the shop diverges from the
field's answer, the file says so and cites the ADR. A literature spike
precedes the writing (rangerhq-dfz8).

**Codex specifics (codex-cli 0.147.0, verified end-to-end in rangerhq-5oi).**
Everything in codex's template above is a fix, not decoration:

- **The PID rides as `developer_instructions:`, not as the prompt arg.**
  ADR 0002 assumed the first user turn; that never worked — a PID file
  starts with `---`, which codex's arg parser reads as a flag, so
  `codex "$(cat <pid>.md)"` dies with `error: unexpected argument '---…'`.
  `-c developer_instructions=…` is a real config key here: `codex debug
  prompt-input` (free, no API turn) shows the text prepended to the base
  developer message — the closest thing codex has to
  `--append-system-prompt` — and it survives multi-line markdown with
  quotes, brackets, braces and `=`. It also leaves the first *user* turn
  free, so a dispatched codex session starts idle and the work prompt is
  its first turn, exactly as on claude. **Silent-failure warning:** codex
  ignores unknown `-c` keys without a word, so a future rename would
  launch personas with no PID at all. Probe before trusting a new codex:
  `codex debug prompt-input x -c developer_instructions=MARK | grep MARK`.
- **`--add-dir` rides with the sandbox mode, not with the template.**
  Under `-s read-only` codex *exits* on it ("the effective permissions do
  not allow additional writable roots"), so `realizeCodex` emits it only
  alongside `-s workspace-write` — where it is what makes the persona's
  memory dir writable (`touch $RHQ_PERSONA_DIR/x` → WRITABLE, verified).
  Read-only needs none: codex reads the whole disk regardless.
- **`allow_login_shell=false` is a wall flag, not a nicety.** Codex
  otherwise runs each shell command through a login shell that re-sources
  the operator's rc files, and that re-prepends the login PATH *ahead* of
  the one codex inherited — so `PATH='<gates>/bin':"$PATH" codex …` still
  resolves `git` to `/usr/bin/git` and the L1 shim is **silently
  bypassed** (observed: `git push --dry-run` reached real git, no
  `refusals.log` line). `--disable shell_snapshot` does not help; the
  login shell is the cause. With the flag off, `command -v git` →
  `gates/<persona>/bin/git` and the refusal lands. Any runtime that
  re-sources rc files deserves the same audit before its shims are
  believed — **and every runtime now gets the gate shell besides** (ADR
  0009): the typed line points `SHELL`/`GROK_SHELL` at
  `gates/<persona>/shell/<basename>`, so the flag is the belt and the
  wrapper the braces. Codex never invokes it while
  `allow_login_shell=false` stands — verified on 0.147 with the wrapper
  live: the session still runs `/bin/bash -c` directly and `command -v
  git` still answers the shim (rangerhq-e43).
- **Two dialogs stand between an unattended codex and idle.** Directory
  trust ("Do you trust the contents of this directory?") fires per exact
  path — a trusted parent does *not* cover a repo underneath it — and
  `-a never` does not suppress it; the TOML inline-table `-c` form above
  does, scoped to `$PWD`, i.e. only the directory posse launched in (the
  dotted form `projects."/p".trust_level=…` is silently ignored). A new
  or changed `~/.codex/hooks.json` then opens "Hooks need review", which
  **stock herdr detection reads as `idle`** — a prompt sent there goes
  into the dialog, silently. `--disable hooks` removes the dialog for
  fleet panes, and posse keeps the flag regardless: not trusting hooks
  blind is the posture, because the cage is ours, not the runtime's
  plugins'. The detection gap itself is fixed by a local herdr manifest
  override, `etc/herdr/agent-detection/codex.toml` (`make
  install-detection`), which also covers every other codex modal footed
  `esc to go back` — `/model` read as idle too. That protects panes the
  fleet did not launch. Upstream's fallback for a known agent whose
  screen matches no rule is `idle`, i.e. it fails toward "safe to
  prompt"; the override is a fork of herdr's manifest, so
  `make verify-detection` warns when upstream moves past our fork point
  (rangerhq-7ia).
- **What directory trust actually loads, and the launch check for it
  (rangerhq-pmz → rangerhq-b7m, verified twice on 0.147.0 with a scratch
  repo and `codex mcp list`, no API turn).** Trust is not only about the
  dialog: a trusted session also reads `$PWD/.codex/config.toml` —
  codex's own words for it are "settings for a trusted repository,
  including sandbox, MCP, hooks, model, and reasoning defaults". The
  split that matters:
  - **Keys posse types on the line win.** `-s`, `-a never` and
    `developer_instructions` are ours whatever the file says, so the
    sandbox mode and the PID cannot be overridden by repo content.
  - **Keys posse does not type are the repo's.** `[mcp_servers.*]` (a probe
    server with `command = "/bin/sh"` appears in `codex mcp list` **only**
    under trust), `notify`, `model_provider(s)` — whose `env_key` can name
    *any* session env var as the bearer sent to its `base_url` — and
    `shell_environment_policy`. `mcp_servers` and `notify` are spawned by
    codex itself, **outside its per-command sandbox, with the whole
    session env, before any model turn**: a file in the repo gets exec on
    the box with no model and no PID in front of it.

  Kept as the fleet default anyway (the security persona's verdict on rangerhq-pmz):
  opt-in would mean codex dispatch never works, the grant is one exact
  path and is not persisted, the attacker must already be able to write
  the repo, and claude has the same class of channel in
  `.claude/settings.json` project hooks — trust is parity, not a new
  floor. What posse does instead is **check at launch**:
  `Runtime.ProjectConfig` names the file and `ProjectConfigKeys` optionally
  narrows it to top-level JSON keys (`parity.go`). Codex keeps the original
  whole-file predicate: any `.codex/config.toml` is a hit. Claude names
  `.claude/settings.json` with `hooks` and `mcpServers`: either top-level key
  is a hit regardless of value, while a readable object carrying only
  `permissions` is clean. An existing keyed file that is unreadable,
  malformed, or not an object fails closed because the launch cannot prove
  the channel absent. A hit is a `Degraded` entry, so `posse new`/`posse
  dispatch`/the cockpit **refuse** and name the file plus the matched keys or
  classification failure unless the operator passes `--allow-degraded`
  (session then marked, as any degraded launch) or the PID carries
  `trust_project_config: true`. It is `Degraded` and never `Unrealized`:
  nothing here is an unenforced gate, it is what the launch gives away.
  `posse gates <persona>` computes the matrix for the cwd, so the line shows
  there too. `posse relaunch` preflights the same launch plan before replacing
  a session, so a repo that grows one of these surfaces after a clean launch
  refuses on relaunch.
- **Env: codex strips nothing by default.** ADR 0002 assumed
  `shell_environment_policy` drops `*KEY*`/`*TOKEN*`/`*SECRET*` names, so
  the template might need `ignore_default_excludes=true`. It does not: on
  0.147.0 the model's shell inherits the session env whole — `BD_ACTOR`
  and `RHQ_*` arrive intact (`bd` inside codex commented and closed a
  bead under the dispatched persona's own actor name), and so does every
  key the operator exports. No flag
  is needed and no naming rule applies; the honest statement is the one
  in the security posture below — env sets are inherited, not contained,
  on every runtime alike.

**Grok specifics (grok 1.0.x, verified end-to-end in rangerhq-vjl —
headless probes on 1.0.0, live fleet sessions on 1.0.5 after the CLI
self-updated mid-verification; no behaviour below differed).**

- **`--rules=`, never `--rules `.** A PID file starts with `---`, which
  grok's arg parser reads as a flag in the separated form: `grok --rules
  "$(cat <pid>.md)"` dies with `Usage: grok [OPTIONS] [PROMPT]` and the
  pane falls back to a shell prompt. The `=` form binds the value and the
  PID lands in the system prompt (`--rules` appends to it; asked its name,
  the session answers with the persona's). This is codex's `---` trap in a
  second parser — assume the next runtime has it too, and probe for it.
- **`--permission-mode auto` is what unattended means here.** Of the six
  modes, only `auto` and `bypassPermissions` approve a tool call with
  nobody watching; under `default`, `acceptEdits` and `dontAsk` even an
  *undenied* command sits unapproved forever. Both working modes still
  honour `--deny` — a denied command is refused under `bypassPermissions`
  too — so the fleet types `auto`, the lower-privilege of the two. The
  flag also beats the operator's `~/.grok/config.toml` `[ui]
  permission_mode`, which on this machine is `always-approve`: the launch
  is the same whatever the operator left in their config.
- **`--allow`/`--deny` are claude's rule *spellings*, on a different
  matcher.** `Bash(git push:*)` refuses `git push --dry-run` and leaves
  `git status` alone; bare tool names (`Edit`, `Write`) match every
  invocation; deny beats allow. The match is not anchored at argv[0]
  either: `env git push --dry-run origin HEAD` was refused too (`Denied by
  permission policy: deny rule on bash matching "git push"`). Still L0:
  `/usr/bin/git push` walked straight past the rule in a live session (the
  pre-push hook is what stopped it), and a denied verb reached from a
  Makefile recipe or a subprocess never enters the string grok matches —
  which is L1's job.
- **The dialect, probed rule by rule (grok 1.0.5, rangerhq-625).** `*` IS
  a wildcard mid-string, not a literal: with `Bash(git -* push)` +
  `Bash(git -* push *)` typed, all ten option spellings of a push —
  `-C`, `-c`, `--git-dir=`/`--work-tree=` and their separated forms, `-p`,
  `--no-pager`, `--literal-pathspecs`, a stacked `--namespace x -C <r>
  --no-optional-locks` — were refused, and nothing reached the throwaway
  bare repo. `?` and `[…]` work too. Three divergences from claude, each
  verified rather than read off grok's own shipped docs
  (`~/.grok/docs/user-guide/22-permissions-and-safety.md`, which describes
  all three correctly — but a doc is not a probe):
  - **A segment reaches the matcher shell-parsed** — quotes off, runs of
    whitespace collapsed to one. So `git -C <r>  push …` (two spaces)
    matches a rule written with one, and, the expensive half, a *quoted
    argument* carrying the word `push` matches as if it were the
    subcommand: `git -C <r> log --author "push me"` and `git -c
    user.name=t commit -m "push it upstream"` are both refused under the
    pair. Claude matches the raw string and runs them.
  - **`:*` is a prefix with no word boundary.** `Bash(git push:*)` refuses
    `git pushy --help`. Claude requires the boundary. That is the PID's
    own rule being read more broadly than we write it, not something the
    fleet spells around.
  - **A rule with no wildcard is a prefix, not exact.** `Bash(sha1sum)`
    refuses `sha1sum --version` unaided — the miss `L0Spellings` adds
    `Bash(<cmd>:*)` for on claude does not exist here.
  This is why **`realizeGrok` does not call `L0Spellings`**: the widening
  is not a no-op there, it is a false-positive generator, and at L0 a
  false positive is a hard block the model cannot ask its way past (the
  ground rangerhq-3mc rejected a single `Bash(git -* push*)` on). The true
  positive is not lost — L1 holds on grok (next bullet) and the shim
  refuses every one of those spellings. One false positive is *not*
  grok's: an unquoted trailing `push` word (`git -C <r> log --grep push`)
  is refused by the pair on claude too — rangerhq-ky3.
- **`git push` is on grok's own dangerous list.** Under `--permission-mode
  auto` it is cancelled outright with nobody there to approve it
  (`User cancelled the execution for tool run_terminal_command`), rule or
  no rule — so the fleet's launch mode already refuses it a second way,
  and a probe that needs a push to actually *run* has to be driven under
  `bypassPermissions` to isolate what the rules did.
- **L1 holds on grok — because the shell is ours now (ADR 0009,
  rangerhq-e43).** Grok runs every shell command through a *login* shell:
  it captures the login shell's state (rc-exported vars, aliases,
  functions) and replays it into each command. On macOS that hands PATH to
  `path_helper`, which re-orders it so `/usr/bin` precedes anything the
  launcher prepended — in a live developer-persona grok session `command -v git`
  answered `/usr/bin/git`, not the gates shim (rangerhq-vjl). This is
  codex's login-shell trap, but codex had `allow_login_shell=false` and
  grok has no equivalent knob: `GROK_LOGIN_ENV`, `GROK_SHELL` and a
  `[toolset] login_shell_capture = false` overlay via `GROK_CONFIG_PATH`
  were all read and ignored. So the fleet supplies the shell instead —
  `SHELL`/`GROK_SHELL` on the typed line point at the rendered **gate
  shell** (see *Gates L1* below), which re-prepends the gates dir inside
  the `-c` string and again in grok's user-command slot, after the replay.
  Verified live on grok 1.0.5, same box: `command -v git` →
  `gates/<persona>/bin/git`; a `git push --dry-run` reached from a Makefile
  recipe → `refused by posse gate: git push --dry-run origin HEAD` + a
  `refusals.log` line; `/usr/bin/git push origin HEAD` → the L3 pre-push
  refusal, nothing pushed; `rm -rf …` on a PID that denies it → refused,
  so a **non-`git push` shell deny is clean on grok, not degraded**.
  `Runtime.LoginShellPATH` and its parity special case are gone.
- **Auto mode will not run an unknown local script.** `sh probe.sh` and
  `sh check.sh` — the second one sitting in the session's own cwd — both
  came back "blocked by auto mode: … treated as an untrusted/unknown local
  script", nothing executed. A live gate probe therefore has to reach the
  shell through a *known binary* (`make <target>` works), not through a
  script the model is asked to run. Grok's own shell-command policy is
  narrower than claude's on this point, and it is why the shim's own
  refusal is easiest to demonstrate from a Makefile recipe.
- **No `--add-dir` equivalent, and none needed.** Grok's sandbox is off
  unless `--sandbox <profile>` is passed, so the persona reads and writes
  the whole disk; the memory dir rides on `RHQ_PERSONA_DIR` alone and the
  template has no `{memory}`. Under `cage: seatbelt` the memory dir is
  writable because *our* profile allows it — verified in a live
  security-persona grok seatbelt session: `touch $HOME/x` → `Operation not
  permitted`, `touch $RHQ_PERSONA_DIR/x` → `rc=0`. Grok's own
  `--sandbox` profiles (`workspace`/`read-only`/`strict`, custom ones in
  `~/.grok/sandbox.toml`) are real Seatbelt, and grok refuses to start if
  a named profile cannot be applied — but they are runtime-specific by
  definition, so ADR 0002 keeps them as L0 inside our L2, not as the tier.
- **The startup splash stands between an unattended grok and idle
  (rangerhq-37c).** No trust dialog: grok's folder trust gates *hooks*,
  not startup, so an untrusted session dir simply runs without the repo's
  hooks — the posture we want. Cross-session memory is off unless
  `--experimental-memory` is given, so the persona's memory stays the
  fleet's ORDERS.md. grok's `SessionStart` hook
  (`~/.grok/hooks/herdr.json`, installed by herdr) reports the session id.
  But a fresh `grok` pane opens on a **startup screen** — the New worktree
  / Resume session / Quit menu, a "<version> is here!" changelog line and
  the "Help improve Grok" consent banner — and that screen holds the
  keyboard before the composer does. Text sent to it is *buffered*, not
  delivered, and the submitting Enter is eaten by the splash: the work
  prompt never runs, it sits unsent in the composer. Stock herdr detection
  matched **no rule** there and fell through to
  `default_known_agent_idle_fallback` = `idle`, so `dispatch`'s
  wait-for-settled gate was satisfied by a screen that could not take the
  prompt. `etc/herdr/agent-detection/grok.toml` reports it `blocked`
  instead; the launch then fails loudly ("never settled idle") rather than
  silently losing the bead's prompt. This was foreseen right here — the
  note used to end "it still reads correctly, but that is luck, not a
  contract", and the luck ran out at grok 1.0.5. The earlier
  `posse dispatch --runtime grok` run that reached idle → prompted → done
  (and closed its bead under the dispatched persona's actor name, so
  `BD_ACTOR` does reach `bd` through
  grok's shell) was on a pane whose splash had already been consumed.
  Note also that the override only makes the failure *loud*: the splash
  never self-dismisses, so grok is not dispatchable again until the
  launcher dismisses it or the fleet moves to a startup mode that has no
  splash (`grok --minimal` has none and takes input immediately) —
  rangerhq-7sbo.
- **…and re-measured in rangerhq-7sbo, where the "holds the keyboard" half
  did not reproduce.** On live 1.0.5 panes today, launched both plain and
  with the fleet's flags: text sent to an untouched splash appears in the
  composer *immediately* (with the `Enter:send │ …` hint footer), one
  `agent send-keys <pane> enter` after it submits the turn (idle →
  `osc_progress_working` → done, menu still drawn), and **Esc undraws
  nothing** — the menu, changelog and banner stay, so detection keeps
  reporting `startup_splash`/`blocked` and `agent wait --until idle`
  simply times out. The splash is decoration over a live composer. Which
  means (a) the "send Esc and wait for idle" fix does not work, and the
  launcher instead presses Esc once and prompts past *the same* screen if
  it is still reported (`clearStartupScreen` in dispatch.go), and (b) the
  first-run state 37c measured is either already consumed on this machine
  or was the boot race all along — a prompt typed before grok's TUI took
  input, which fits the coordinator's incident too. Whether `startup_splash` should
  report `blocked` at all is rangerhq-1xsj (ops). Two lessons, both
  the same one: a screen that *looks* like it holds the keyboard is not
  evidence that it does — press a key and read the pane; and a detection
  rule's anchor must not include an optional element (37c's required the
  `[stable]` channel tag, which today's panes do not render, so the rule
  matched nothing at all until 7sbo widened it).

**Gates L1 (ADR 0002 §3, `internal/rhq/gates.go`).** Native flags are
politeness; the wall for shell-verb denies is ours and runtime-agnostic.
At every persona launch (create and crash-relaunch alike) the PID's
`deny:` rules of the shape `Bash(<cmd> <prefix>:*)`, `Bash(<cmd> <args>)`
(exact) and `Bash(<cmd>)` (whole verb) are rendered into
`RHQ_HOME/state/gates/<persona>/bin/<cmd>`: a POSIX sh shim that refuses
when argv matches — `refused by posse gate: git push --force (deny:
Bash(git push:*))`, exit 1, a line in `gates/<persona>/refusals.log` —
and otherwise `exec`s the real binary resolved at render time from PATH
minus any gates dir. Rendered fresh each launch (a rule dropped from the
PID stops being enforced; the log survives). The shim dir is prepended
**on the typed command line** — `PATH='<bin>':"$PATH" SHELL='<gate
shell>' GROK_SHELL='<gate shell>' <rendered command>` — not in the
workspace env, because macOS `path_helper` reorders PATH when the pane
shell starts. `RHQ_GATES_DIR` names the dir in the env. `posse gates <persona>` shows the shims and the log tail.
Known holes — `/usr/bin/git`, `command -p`, **git aliases** — are why L3
exists for the one verb that is a hard risk line; the parity check
(rangerhq-1po) says which denies the wall realizes.
`Edit`/`Write`/`WebFetch`/`mcp__*` denies are not shims.

- **Git aliases dodge L1 entirely** (rangerhq-3mc). `git p` where the
  operator's gitconfig says `alias.p = push` reaches the shim as the token
  `p`; matching it would mean running `git config --get alias.<token>` per
  invocation, in POSIX sh, against whatever repo the global options point
  at — a fork and a config read on every gated command, and still wrong
  for an alias defined in the *repo's* config. So it stays a documented
  hole: L3's pre-push hook catches it in hooked repos (the alias expands
  to a real push, which runs the hook), nothing catches it elsewhere, and
  the tier that closes it properly is `seatbelt`/`container`, where the
  gate is not a name lookup.

**The gate shell (ADR 0009, `renderGateShell`, verified live in
rangerhq-e43).** A PATH prefix on the typed line survives only while the
runtime uses the shell it inherited. Grok 1.0.5 does not: it re-execs the
operator's *login* shell, so `/etc/zprofile` runs `path_helper` and the
gates dir is demoted below `/usr/bin` before any command runs. So the
shell is ours as well: every launch renders
`RHQ_HOME/state/gates/<persona>/shell/<basename>`, a POSIX sh wrapper that
walks argv the way a shell does, prepends a PATH guard to the command
string after a `-c` and — behind a `--` argv0 — to the runtime's
user-command slot (which runs *after* the snapshot replay), then `exec`s
the real shell: `$SHELL` when its basename is `bash` or `zsh`, else
`/bin/zsh`. The wrapper is *named* with that basename, because grok picks
its snapshot dialect from the name. The typed line points `SHELL=` and
`GROK_SHELL=` at it on **every** runtime, so a runtime that starts
snapshotting after a self-update inherits the fix instead of a silent
regression; `Runtime.NoGateShell` (`gate_shell: false` in
`runtimes/<name>.yaml`) is the exit hatch and drops parity back to
unrealized for `Bash(…)` denies on that runtime. A mis-parse breaks the
persona's shell loudly; it never falls back to a silent bypass.

- **The guard tests precedence, not presence.** `case "$PATH:" in
  "<bin>":*) ;; *) PATH="<bin>:$PATH";; esac; export PATH;` — the obvious
  "idempotent" spelling (is the dir *anywhere* on PATH) is a no-op exactly
  when it is needed, because the typed line already put the gates dir on
  PATH and `path_helper` **re-orders** it rather than dropping it. With
  the presence test the wrapper rendered, ran on the right argv, and still
  left `command -v git` answering `/usr/bin/git` in a live grok session.
  Cost of the precedence test: PATH can carry the dir twice; lookup takes
  the first. The same trap waits for any "already set?" guard that runs
  after a reorder.
- **`gates/<persona>/shell.log`** gets a line only when the *user-command
  slot* guard had to re-prepend — the replayed snapshot had lost the gates
  dir entirely. A normal session leaves no file at all (verified across
  claude, codex and grok sessions); a line means a runtime's snapshot
  shape moved and is worth reading.
- **Consequence: the persona's own rc files now run gated.** The guard is
  prepended ahead of the login capture's `source ~/.zshrc`, so a denied
  verb *inside* an rc file is refused like any other — observed on a probe
  PID denying `rm -rf`: oh-my-zsh's `rm -rf ~/.oh-my-zsh/log/update.lock`
  landed in `refusals.log`. Honest, and worth knowing before reading a
  refusals log as "the model tried something".

Shell shapes verified 2026-08-18 by installing an argv logger as `$SHELL`
(repeat after any runtime self-update — ADR 0009 verification item 7):

| runtime | how it invokes the gate shell |
|---|---|
| grok 1.0.5 | login capture, twice: `-lc 'source "$HOME/.zshrc"; printf …"$PATH"; command env -0'` and `-lc '… builtin alias -L; builtin typeset -f'`. Per command: `-c '<replay: snap=$(command cat <&3); builtin eval "$snap"; …; __grok_user_cmd="$1"; builtin set --; builtin eval "$__grok_user_cmd" 2>&1>' -- '<user cmd>'` (bash dialect: `-O extglob -c … -- …`). The replay also shadows `find`→`bfs` and `grep`→`ugrep` as functions. |
| claude | `-c -l '<snapshot; eval cmd>'`; its snapshot restores the process PATH, so the typed prefix already won — the wrapper changes nothing (verified: `command -v git` → the shim). |
| codex 0.147 | never — `allow_login_shell=false` runs `/bin/bash -c` directly (verified: the shim still wins). |

**Gates L3 (rangerhq-8s4).** `.git/hooks/pre-push` (`# posse-gate`
marker) refuses when `RHQ_TOOLS_DENY` — newline-separated, exported into
every persona session by CreateSession — carries a rule matching git
push (`Bash(git push:*)`, `Bash(git push --force:*)`, `Bash(git:*)`,
`Bash(git)`), with the same message shape and a `[pre-push hook]` line in
`refusals.log`; without the variable (an interactive operator) it passes.
Catches `/usr/bin/git push` and pushes from subprocesses that kept the
env. It cannot see through `env -i` — nothing in-process can; that is
the container tier's job. Install: `posse gates install-hooks [repo]`
(replaces its own hook, refuses to overwrite a foreign one — chain by
hand). Persona launch reconciles the pre-push slot when the PID denies git
push and always reconciles `prepare-commit-msg`, then runs both gated
operations in one shell invocation and requires exit 1. A working foreign
chain therefore counts without a marker; a planted pass-through body does
not count even if it kept one. Failure is `DEGRADED`, visible before herdr
is touched, and L3 disappears from concrete parity. The probe is launch-time
evidence, not a permanent lock: at `cage: shims` the session can still edit
the slot after the probe (the TOCTOU residual); the L2/L4 hook carve-out is
what removes that capability. Chain foreign slots per INSTALL.md §9.
`posse init` does not touch repos.

**Tiers (ADR 0003 §1–2).** A tier is a name — `strong` / `standard` /
`fast` — mapped to a model per runtime in the built-in table: claude
`claude-fable-5` / `claude-opus-5` / `claude-sonnet-5`; codex
`gpt-5.6-sol` / `gpt-5.6-sol` / `gpt-5.6-luna`; grok unmapped (runtime
default; `fast` falls back to `standard` when only that is mapped). Codex
maps `strong` and `standard` to the same id on purpose: sol is what a
codex session here defaults to and codex offers nothing above it, so
naming it makes the launch a fact rather than a CLI default that can move
between releases, while `fast` = luna is the **cost** lever only —
MEASURED 2026-08-25, switching to luna did not lift an account-level usage
wall, because the wall is on the account and not on the model
(ranger-base-arm). Until that map existed `tier:` was inert on codex: no
`Models` at all, `{model}` empty, no warning. A runtime that maps nothing
now says so where it is read — `posse runtimes` prints `tiers: UNMAPPED —
ignores tier:` and `posse runtime check <name>` names the tiers that
render nothing, both off one rendering (`Runtime.TierMap`). A
`runtimes/*.yaml` still cannot override a **built-in**: `LoadRuntime`
returns the built-in as soon as the name matches, before it stats
`RHQ_HOME/runtimes/<name>.yaml`, so `model_<tier>:` / `model_flag:` reach
template-only runtimes only (undecided, ranger-base-arm). Built-in
templates carry `{model}` → `--model <id>` (claude),
`-c model=<id>` (codex), `-m <id>` (grok), rendered to nothing when
unmapped; a `runtimes/<name>.yaml` may set `model_<tier>:` and
`model_flag:`. A PID that keeps its own `command:` gets no model unless
it adds `{model}` — `posse agent check` warns. Precedence at a launch site:
`--tier` (new/dispatch) > PID `tier:` > config `default_tier:` >
`strong`; bead-label and `tier_by_label` resolution is dispatch's
(rangerhq-6eb), and `fast` is gated on full enforcement parity with
`tier_floor:` refusing anything cheaper (rangerhq-2uq — see the security
posture below). Dial D ("floor `standard` fleet-wide, `fast` only on an
explicit signal *and* full parity") needs no config key of its own:
nothing resolves to `fast` unless a bead label or `tier_by_label` says
so, and the parity rule is what makes it honest when one does. Sessions carry `RHQ_TIER` in the env and `tier:` in meta; listings
show `🎭name@runtime/tier` when either differs from claude/strong. Dial
A in `examples/agents`: architect/security/product `strong`, the rest
`standard`.

**Tier availability preflight (rangerhq-oay).** A tier is a name and the
launch turns it into a model id — but until this landed, nothing asked
whether the account can run that id. It came from a real morning: access
to the strongest model disappeared from the operator's own session, and a
persona resolving `tier: strong` would have gone on launching while the
CLI quietly served something else, with `posse cost` filing the spend
under whatever tier the substitute belongs to and no line anywhere saying
why. So `planLaunch` now checks, once per launch, on the pair it has just
resolved: `App.TierPreflight` (modelavail.go), before the parity check, so
what the wall and a PID's `tier_floor:` rule on is the pair that would
really launch. Unavailable prints one line — `richard: tier strong wants
claude-fable-5 — unavailable, falling back to claude-opus-5` — writes
`fallback:` into the session meta (so `posse list` and the cockpit wear
`⤵️fallback` beside a `@runtime/tier` tag that now names the substitute),
and dispatch reads that meta back so the work prompt tells the persona the
tier it is actually thinking at. `posse cost` needed no change and that is
the point: `TierForModel` reads the model out of the transcript, so the
spend was always counted honestly — what was missing was anyone knowing.

The probe is `GET api.anthropic.com/v1/models` with the same credential
the plan guard reads, zero tokens, shared through
`$RHQ_HOME/state/model-catalog.json` behind `model_probe_ttl:` (default
1h) exactly as `plan-usage.json` is — a successful reading is reused for
the TTL, and rate-limit cooldowns are shared across processes. Other failed
attempts remain UNKNOWN and may be retried by the next launch. Verified live
2026-08-23: the OAuth credential is
accepted there (the route exists and answers 401 unauthenticated;
`/api/oauth/models`, the shape the plan guard's endpoint would suggest, is
a 404), and the catalog it returns is the ten ids this account can use,
including all three the claude tier table names. A later installed-binary
launch on the same account produced no snapshot at all; before the request
log below, that launch left no evidence distinguishing credential context,
HTTP response, transport failure, or an empty answer. **Answered
2026-08-26 (ranger-base-r64): that launch was from a gated pane** — posse's
own `security` read resolved to the persona's `Bash(security:*)` shim and
was refused. The read now returns that as its own error instead of
"keychain item unreadable", and the preflight says so once per process on
stderr rather than only in the log.

Every cache miss that attempts a probe appends a generic outcome to
`$RHQ_HOME/state/model-catalog.log` (`ok models=N`, HTTP failure, empty
catalog, or cooldown); cache hits append nothing, and the log is bounded on
the same policy as `plan-usage.log`. It never records the credential or a
header. This is the evidence for UNKNOWN: a launch still fails open, but a
missing `model-catalog.json` no longer leaves "model available" and "probe
could not authenticate" observationally identical.

Three rules make it safe to leave on by default. **It fails open in one
direction only**: a catalog that was actually read and does not contain
the model is the ONLY thing that moves a launch — an unreadable
credential, an unreachable endpoint, a 429, an empty answer or a runtime
with no model mapping are all *unknown*, and unknown launches exactly what
it was asked to launch without a launch warning (the request outcome remains
in `model-catalog.log`). A preflight that guessed "unavailable"
would silently downgrade the whole shop, which is the failure it exists to
prevent one level up. **It never refuses**: rule (3) from the operator —
"a degraded model is worse than nothing" is their judgement, and the place
they record it in advance is `tier_floor:`, which still bites, on the
substituted pair. **Where a tier lands is config, not code**:
`tier_fallback:` is a one-level map whose key is a persona name or a tier
name (persona wins) and whose value is a tier (drop a tier on the same
runtime), a runtime (hop runtimes at the same tier), or `none`. The
per-persona half is rangerhq-u2p's requirement — a lane whose fallback
from the strongest model may be a different runtime rather than a cheaper
model. The default is `strong` → `standard`, and unlike `tier_by_label:`
above, naming one key does NOT take that default away from everyone else:
the operator's rule is that everyone falls back, so one persona line must
not be able to switch the rest of the shop off. `model_preflight: false`
turns the whole thing off; `posse gates <persona>` prints the verdict per
runtime, which is how you tell "the strong model is gone" from "the probe
never answers on this box" without launching anything.

Two consequences worth knowing. A session's meta records the tier it
*actually* launched at, the way `cage:` records the cage it got — so
`posse relaunch` and `RelaunchAgent` replay the substitute, and a session
degraded during an outage stays there until it is recreated after the
model returns. And the test seam is `App.ModelLister` (nil =
`NewModelLister`), the twin of `Dispatcher.Plan`: `newTestBackend` hands
every test an unconfigured one, which reads no credential and reaches no
network and is therefore also the fail-open path. That seam is the *only*
one: the catalog URL is compiled in, with no env override at all, because
the probe sends the account's OAuth token and a second way to point that
token is the same argument as a second way to hand it one (credpin.go).
Tests that want the preflight to *do* something seed
`state/model-catalog.json` — a reading off a seeded snapshot with an
unconfigured lister proves it never asked anyone.

Catalog membership and plan allotment are different facts. On 2026-08-24 a
strong-tier Claude session returned a synthetic assistant message saying
its Fable allotment was exhausted, then settled `idle` without doing work;
the model can remain in `/v1/models` throughout that condition. Dispatch now
checks the matching Claude transcript after a turn settles. That exact
provider refusal writes `turn_failure:` into the session meta, prints a loud
"no work ran" line, marks `posse list` with `🛑turn-failed`, and renders the
cockpit row red as `failed` instead of healthy `idle`. It does not guess a
fallback or replay the prompt: changing tiers remains an operator decision,
and the claimed bead stays attached to the failed session until that
decision is made. A later dispatched turn whose first assistant answer is
healthy clears the marker. The matching keys on the transcript's assistant
record after `Work beads issue <id>`, not on pane text, because bead data
may quote the provider message verbatim.
`posse scorecard [<persona>]` makes the PID metrics observable from bd
data, read-only (rangerhq-h2c): per persona, closed / reopened / open /
held (in_progress) / blocked, median age at close, and beads filed
(`created_by` = the persona's BD_ACTOR) vs rejected (close reason
reading invalid/duplicate/wontfix); then each PID's `metrics:` ids in
words. bd's snapshot has no status history, so *reopened* is read from
the git history of `.beads/issues.jsonl` (closed→open between two sync
commits) and shown as `?` when there is none. Honest gaps, printed as
such: `blocked-honestly` is a dispatch-side outcome, and
`designs-implemented-unchanged` / `spec-clarity` need a comment scan —
neither is computed yet.

`posse cost [--since <date>] [--project <substr>] | --plan` is ADR 0003 §4's
accounting: the analyst's `bead-cost.py` method in Go. Claude Code transcripts
(`~/.claude/projects/*/*.jsonl`) are segmented by the dispatcher's "Work
beads issue <id>" prompts; assistant records are deduped by message id
(streamed chunks repeat it — max per usage field) and priced at list
rates per MTok for the model each record names (fable 10/50, opus 5/25,
sonnet 3/15, haiku 1/5; cache write 1.25× input for 5m TTL and 2× for 1h
when the breakdown is present, else 1.25× flat as the script did; cache
read 0.1×; unknown ids fall back by family, then to fable — the expensive
assumption). Output: per bead (start, persona from the bead's assignee,
tier from the model that did the work, turns, tokens, api$), then by
tier / persona / day with median and per-bead, the interactive total
(never gated, shown for the ratio), and honest gaps: codex/grok sessions
leave no transcript here and are reported as *uncounted*, never $0; per
pass is not attributable until dispatch records a pass id
(rangerhq-25p). The cockpit shows each per-bead session's running cost
and the day total in the footer (rescanned every 30s off the event
loop). The metric `cost-per-closed-bead` has a scorecard answerer for
h2c — `posse cost` by bead id against closes — so a PID that declares it
reads as `computed`. `--plan` skips all of the above and prints only the
plan's own rate windows (the plan-guard section has the reading); it takes
no other flags, because there is nothing for a date or a project to select.

`posse agent new <name>` scaffolds the PID shape — every frontmatter key
present (lists empty, commented hints), every body heading in contract
order with a one-line hint, the four hard risk lines verbatim
(`HardRiskLines`) — and opens it in $EDITOR; `posse agent edit <name>`
reopens it. The scaffold parses with `LoadAgent` untouched. `posse agent
check <name>|--all` lints PIDs against the contract (identity line,
sections present and ordered, Intents table header, hard risk lines
verbatim, `{allow}`/`{deny}` last in `command:`, permission rules whose
parentheses an inline list split on a comma, metric ids that near-duplicate
another PID's spelling) and exits 1 on findings, so an instance repo can
run it in CI.

The metric catalog is **derived, not declared** (ADR 0001 amendment
2026-08-18): `App.MetricCatalog()` is the union of every loaded PID's
`metrics:` plus config `metric_ids:`, mapped to the personas that declare
each id. A persona naming how it is judged is the source of truth, so the
linter never rejects an id as "unknown"; what it does check is that the
vocabulary stays *one* spelling — `metricKey` lowercases, splits on
non-alphanumerics and stems each word loosely (-ing/-s/-e), so
`findings-survive-triage` and `findings-surviving-triage` collide and get a
"one spelling" finding naming the other PID. `posse scorecard --catalog`
lists the union with `computed` (the scorecard has an answerer) or
`declared` per id; per-persona lines read `declared, not yet computable:
<what bd would need>` for the rest, never "not in the catalog". Answerers
follow the PIDs' spelling (`findings-surviving-triage`; the ADR's original
stays a computed alias). The hint for *what bd would need* is specific only
for the ADR's own generic ids — the crew's ids are the instance's
vocabulary, not the product's, so anything else gets the honest general
answer.
`examples/agents/*.md` are all in PID shape (architect is the reference).
Runtime variants (codex, grok, …) are a `command:` choice, not an agent
file: pick the runtime by editing a PID's `command:` (or a recipe
`--cmd`); one persona, two runtimes = one PID and two recipes.

**The permission mode is a launch fact, never a default (rangerhq-qs5r).**
OPERATOR DIRECTIVE, 2026-08-22: all agents always start in auto mode.
Nothing on the claude line used to name a mode, so a persona session got
whatever the CLI defaulted to that week — and that moved under us:
two live persona sessions both landed in `manual` (footer `⏸
manual mode on`), blocked on consecutive approval dialogs nobody was
watching, and the operator cleared them by hand. The fleet's claude line
now types **`--permission-mode auto`** (`ClaudeFleetFlags`), the same mode
and the same reasoning as grok's: of the six modes (`acceptEdits`, `auto`,
`bypassPermissions`, `manual` — the old `default`, renamed — `dontAsk`,
`plan`) only `auto` and `bypassPermissions` approve a tool call with nobody
there, both still honour the deny list, and `auto` is the lower-privilege
of the two.

Verified live on claude 2.1.239, on a herdr pane running the full
fleet-shaped line (`--model --append-system-prompt --add-dir --settings
--disallowedTools`): `--permission-mode manual` → `⏸ manual mode on`,
`--permission-mode auto` → `⏵⏵ auto mode on`. The flag takes in both
directions, which is the whole point — the mode is now typed, not
inherited. Two things this is *not*: it is not typed keystrokes (a mode
cycled with shift+tab lands in whatever screen the pane is drawing —
cf. grok's splash, rangerhq-7sbo), and it is not a `.claude/**` permission
file (those are the operator's keys). Nothing in the launch reads a CLI
default any more, so a session cannot be one CLI release away from
blocking.

The guarantee is a *runtime* property, not a template detail:
`Runtime.Unattended` names the flag (claude `--permission-mode auto`, grok
the same, codex `-a never`) and `RenderCommandFor` appends it when the
rendered line does not name it. That covers the one template posse did not
write — a PID's own `command:` — and stops short of guessing: it appends
only when the line actually starts that runtime's executable, and a
template-only runtime (`Unattended` empty) is left alone, because a flag
typed at a CLI whose dialect nobody probed is a launch that does not start
at all. A `command:` that names a mode explicitly keeps the mode it named,
where `ps` can see it.

**Unattended sessions and Claude Code dialogs (rangerhq-4e5).** Claude
Code opens its "Set up auto mode for your environment?" dialog *after a
turn ends* whenever the session is in auto mode (which every fleet session
now is, by the flag above), the operator has never answered it, and it is
not otherwise
suppressed — with "Set it up" preselected. In a fleet pane nobody is
looking, so the next dispatched prompt (text + Enter) selects "Set it up"
and subsequent keystrokes operate a wizard that configures shell-history
and repo scanning. Reproduced on demand in a tmux pane; the gate (read
from the 2.1.233 binary) is: skill `auto-mode-setup` not disabled via
`skillOverrides` AND no `autoMode.environment` configured AND
`~/.claude.json` `autoModeEnvSetup` not `dismissed` (and no "Not now" in
the last 7 days — which the accidental "Set it up" path *clears*, hence
the recurrence on fresh sessions).

The fix posse ships: every claude persona command carries
`--settings '{"skillOverrides":{"auto-mode-setup":"off"}}'` (the
`ClaudeFleetSettings` constant; the default command and
`examples/agents/*` include it). Verified: same tmux repro, dialog gone
across turns, auth untouched. Alternatives, for the record: choosing
"Don't show again" once in any interactive session (writes
`autoModeEnvSetup.dismissed` to `~/.claude.json`), or the same
`skillOverrides` entry in `~/.claude/settings.json` — both are per-user
config outside the repo, so they are not what the harness relies on.
Rejected: `CLAUDE_CODE_SIMPLE=1` / `--bare` does suppress onboarding but
never reads OAuth/keychain credentials — a subscription-authenticated
fleet lands on "Not logged in" (verified). Also rejected: dispatch
sending Esc on a stalled prompt and retrying — that is more automated
keystrokes into an unknown modal; dispatch instead unclaims and benches
the session (rangerhq-81d), and the root cause is suppressed here.

**Identity.** A persona's durable identity is its beads assignee name.
Launching a persona session injects `BD_ACTOR=<persona>` into the workspace
environment, so every bd command the agent runs — claims, closes, mail —
is attributed to the persona automatically. `posse claim/done --as <persona>`
does the same from outside. herdr agent names are never load-bearing.

**BD_ACTOR is attribution, not authorization — accepted (rangerhq-pnp).**
bd has no authentication: the actor is an env var or `--actor` flag, and
any process in any session can run `BD_ACTOR=other bd close X`. That is
fine *at this scale* because all personas run as one OS user under one
operator — the OS user is the real principal, the actor is an audit
label, and the commit author on `.beads/issues.jsonl` is a second,
harder-to-forge signal for forensics. Nothing in posse grants privilege by
actor: `Route` honouring a bead's assignee and `launchSession` treating
"already claimed by this persona" as a resume are routing conveniences.
The line — any one of these flips the verdict, and the fix past it is
OS-level separation (per-persona user/container), not a smarter
BD_ACTOR: (1) beads start gating outward actions on assignee ("only
devops may close deploy beads", merge slots, `bd gate`); (2) personas
ingest untrusted input, because prompt injection → identity spoof →
close/reroute other personas' work; (3) more than one human or org
shares the database. Related guard already in place: `workPrompt` fences
bead-sourced text — the title is `%q`-quoted and labelled as data, and a
bead id that is not a plain token is refused before it is embedded in a
`bd show` command — so a title written by one persona (or a public
repo's `issues.jsonl`) cannot be another persona's first instruction.

**Memory.** `personas/<name>/` (under RHQ_HOME) is the persona-private
memory dir — the one memory kind posse owns (project memory belongs to
beads: `bd remember` / `bd prime`; runtime memory belongs to the agent
CLI). It is materialized at launch, seeded with `ORDERS.md`, exposed to the
process as `RHQ_PERSONA_DIR` and to the command template as `{memory}`
(e.g. `--add-dir {memory}` for claude). `posse memory <persona>` opens the
standing orders in $EDITOR.

## Fleet security posture (read this honestly)

Three different things get called "permissions"; keep them apart
(ADR 0002, findings from the jb2 audit kept where still true):

- **The allowlist is friction, not privilege.** `.claude/settings.json`
  (fleet floor) and a PID's `allow:` decide what runs *without
  prompting*. A persona with `Write` plus `go test`/`make` can execute
  arbitrary code as your user — writing a test *is* writing a program.
  Runtime-native flags (`--allowedTools`/`--disallowedTools`, grok
  `--allow/--deny`, codex `-a`) are **L0: politeness** — the polite
  refusal in front of the wall, never counted as enforcement.
- **Read-only text tools: measured, not assumed (rangerhq-kbvm).**
  Verified live on claude **2.1.241** against an empty settings file, in
  both `--permission-mode default` and `auto`: claude's built-in
  read-only classifier already runs `grep`, `sed -n`, `head`, `tail`,
  `wc` and `cat` — flags and all — with no prompt and no allow rule, and
  it **decomposes** a command on `|` and `&&` and decides each stage on
  its own (`cd . && awk … | head -1` is three decisions, not one). So a
  fleet-floor entry for those six buys **zero** prompts today. What still
  reaches the auto-mode failsafe is `awk` in every form — 168 of the
  12,690 Bash calls in the three fleet repos' transcripts (1.3%) — any
  pipeline containing it, and any real `>` redirect. Two corrections to
  the prefix-match folklore, both verified: rules match **on tokens**, so
  `Bash(sed -n:*)` refuses `sed -ni '1p' f` (`-ni` is not the token
  `-n`) — but it *does* admit `sed -n -i.bak 's/…/…/' f`, which with `-n`
  truncates the file to nothing; and an allow rule is not the last word,
  because under `Bash(awk:*)` claude still refuses `awk
  'BEGIN{system("…")}'` and `awk -f prog.awk`, its own injection check
  firing first. `Bash(sed:*)` unqualified, by contrast, runs `sed -i` and
  edits the file — so the `-n` in the rule is doing real work even though
  it does not close the hole. Net: the seven lines are worth adding as a
  **pin**, not as a saving — they hold the friction down if a CLI upgrade
  narrows the built-in list, which would otherwise put the 10,468 calls
  (82%) that touch one of the seven back in front of the failsafe. The
  `sed -n -i` residual is accepted on 5w6's posture (`Write` is allowed
  unconditionally, `go test`/`make` are arbitrary exec), and no pattern
  can close it: the matcher cannot express "no `-i` anywhere in the
  argv".
- **The wall is L1–L3, ours, on every runtime.** Rendered fresh from the
  PID at every launch: **L1** PATH shims for every shell-verb deny
  (`Bash(git push:*)` → `gates/<persona>/bin/git` refuses, logs to
  `refusals.log`, execs the real binary otherwise; PATH prefixed on the
  typed line, and the **gate shell** typed as `SHELL`/`GROK_SHELL`);
  **L3** the git pre-push hook honouring `RHQ_TOOLS_DENY`
  for the one verb that is a hard risk line. Both cost nothing per
  session and are on for every persona session — claude included — but
  **L3 is per-repo**: the hook is installed in the repo the session
  launches into (skipped when a foreign `pre-push` hook is already
  there — we never overwrite one), and is absent from every *other*
  repo reachable from that pane unless `posse gates install-hooks <repo>`
  was run there. Known holes: `command -p`/`/usr/bin/git` (stopped by
  L3 in a hooked repo, and by nothing below L4 in an unhooked one),
  `env -i` (nothing in-process can see through it), and everything that
  is not a shell verb. A PATH prefix is also only as good as the shell
  the runtime starts: a runtime that re-execs the operator's *login*
  shell hands PATH to `path_helper`, which demotes the gates dir (codex
  did until `allow_login_shell=false`; grok 1.0.5 still does, with no
  knob to turn it off). The answer is to own that shell too — ADR 0009's
  gate shell, rendered per persona and typed on every runtime,
  re-prepends the gates dir inside the command string and inside the
  runtime's user-command slot. Verified live on claude, codex and grok
  (rangerhq-e43), so parity counts L1 on every runtime again;
  `gate_shell: false` for a runtime that chokes on a wrapper drops it
  back to unrealized there. Residual: on a *persistent-shell* backend
  only the login-capture guard applies, and
  `gates/<persona>/shell.log` is the detector.
- **The boundary is L2/L4, only when declared.** `Edit`/`Write`-class
  denies are OS-enforced only by the **seatbelt tier** (`cage: seatbelt`
  in the PID or `--cage seatbelt`; `seatbelt.go`) or codex's `-s
  read-only` (which counts — it is a seatbelt). Seatbelt: the runtime
  command is typed as `PATH=… sandbox-exec -f
  state/gates/<persona>/seatbelt.sb <runtime cmd>` (the pane shell
  expands `$(cat {file})` first); the profile is `(allow default) (deny
  file-write*)` plus an allow-list rendered from the PID at every launch
  and relaunch: the repo — **unless the PID denies Edit/Write, then only
  `.beads/` and `.git/`** so bd can still claim/comment/close — the
  persona's memory dir, the gates dir (refusals.log), the runtimes' own
  state (`~/.claude`, `~/.claude.json`, `~/.codex`, `~/.grok`, caches),
  posse's own `state/` **derived from the App's home** (it was the literal
  `~/.config/rhq/state` until ranger-base-cpyb, so a second `RHQ_HOME`'s
  sessions got no grant to their own state dir and one into the default
  instance's — rangerhq-qfzr), `$TMPDIR`/`/tmp`, `/dev`, and the PID's
  `writable:` extras (relative to the repo). **What it never grants is the
  rest of the home** — `agents/`, `config.yaml`, `recipes/`, `skills/`,
  `envs/`, `promoted.json`: after ADR 0015 §2 that is the promoted
  constitution, and a promoted copy stays in force precisely because no
  session can write it. `posse gates <persona>` prints the writable set
  with that check over it, so the property is read rather than audited —
  `✗ GRANT REACHES THE CONSTITUTION: <path>` when it fails.
  `personas/<self>` IS granted: §5's named exception, memory is not law.
  The home's `personas` is a symlink into the constitution repo and the
  grant is resolved through it (`resolveExisting`), so the profile matches
  the real directory and neither spelling reaches another persona's
  memory. **Plus the store of record when `.beads/redirect` moves it**
  (ADR 0012 D3-C, rangerhq-k5ny): `<repo>/.beads` is then a pointer, and
  the database, jsonl, socket and lock are in the instance repo, so the
  profile grants the *resolved* `.beads` and that repo's git dirs (the
  per-worktree one and the common one) — and nothing else in that tree.
  Without it `bd sync`/`bd export` fail on the db file and a commit fails
  on `.git/index.lock`, which is how a caged persona's ORDERS.md sat 203
  lines uncommitted in a shared checkout (measured, ranger-base-rhw); the
  bd calls that go over the daemon socket keep working, which is what
  makes the failure quiet. Same resolver `beadsHome` gives the census and
  codex's `--add-dir` (ranger-base-0fb). Verified on this host under the
  rendered profile: `touch` in `~/src/ranger-base/.beads` and `.git`
  succeeds, `touch ~/src/ranger-base/x` → Operation not permitted, and
  `bd export -o` — the command that failed on the db file — exits 0.
  Path-scoped denies (`Edit(docs/adr/**)`) are **ADR 0014**: a
  subtree file-write deny, realized by a trailing SBPL `subpath` deny
  after the cwd allow (last match wins, measured 2026-08-25) and, at
  container, a `:ro` overlay of that directory — never by a hook.
  `writable:` is the allow-list dual at both L2 and L4. The matrix and
  renderers land with that ADR's implementation beads; until they do, a
  parametrized rule still hits parity's default arm. Verified on this host:
  `touch` in the repo → Operation not permitted, `ORDERS.md` append and
  `.beads/` writes succeed, claude/codex/grok all start under it. **Codex cannot be wrapped**: it
  sandboxes its own child commands with `sandbox-exec`, and macOS refuses
  to nest (`sandbox_apply: Operation not permitted`) — the parity check
  reports `cage: seatbelt` on codex as incompatible (use `shims`; codex's
  read-only is OS-enforced there); the launcher never wraps a
  self-sandboxing runtime. Likewise do not turn on claude's/grok's own
  sandbox settings inside the seatbelt. `sandbox-exec` is deprecated by
  Apple but is what codex itself ships on; its successor is the container
  tier. `egress:` and `WebFetch`/`WebSearch` denies are realized only by
  the container tier, which is now built end to end — launch, egress
  route and wall (L1/L3 rendered inside, the repo `:ro`) — see *Container
  tier (L4)* for what it costs and what it really buys. `mcp__*` and other
  tool-name denies are realized by no tier: an allowlist has nothing to
  say about a stdio MCP server, which is a child process inside the cage.
  Below those tiers such gates are **unrealized**, and the launcher says so.
  And **`(deny file-write*)` denies writes, nothing else**: the profile
  carries no `mach-lookup`/IPC denies, so L2 stops no *read* by a session
  of anything its user can read — the macOS keychain included, because
  `security` asks `securityd` out-of-process and the item's own ACL, not
  the sandbox, answers. A deny aimed at a **read-only** tool is therefore
  realized by **L1 alone** below the container tier — which is still worth
  declaring, because L1 is the only layer that refuses deterministically
  *and* writes a line to `refusals.log`. Read it as a tripwire, not a
  wall: L1 matches the typed word, so `/usr/bin/<cmd>`, `sh -c`, and an
  exec by an allowlisted build tool all walk past it (the documented class
  in the `gates.go` header). The wall for a read stays L4.
- **Enforcement parity — refuse, or degrade out loud (`parity.go`).** At
  every persona launch `CheckParity` computes, for (runtime × cage),
  which PID gates at least one wall layer realizes. Anything unrealized
  (or a `cage:` the host cannot provide — `shims` always, `seatbelt` when
  `sandbox-exec` exists, `container` when the engine's binary is on PATH)
  **refuses** the launch with the list — `posse new`, `posse dispatch`, recipes
  and the cockpit alike — unless `--allow-degraded` is passed; then it
  launches, prints the list, records `degraded:` in the session meta, and
  `posse list`/the cockpit show `⚠️degraded`. Dispatch never passes it on
  its own. Concretely today: a persona that denies `Edit`/`Write` (the
  security and reviewer skeletons) launches clean on **codex** at shims,
  and on **claude/grok** only at `cage: seatbelt` (declare it in the PID
  or pass `--cage seatbelt`); at shims it is refused unless
  `--allow-degraded`. `posse gates
  <persona>` prints the matrix per runtime. `RHQ_CAGE` in the env, `cage:`
  / `degraded:` in meta. One part of the check depends on *where* the
  session starts rather than on the PID: a runtime that posse makes able to read
  the session dir's own executable config degrades the launch unless the PID
  says `trust_project_config: true` — any `.codex/config.toml` for codex;
  top-level `hooks` or `mcpServers` (or a classification failure) in
  `.claude/settings.json` for claude. `CheckParityIn` is `CheckParity` plus
  that directory check, and `CheckParity` itself stats nothing.
- **The tier is part of parity (ADR 0003 §3, rangerhq-2uq).** A cheaper
  model follows the PID's prose less reliably, and the wall does not care
  what model is behind it — so a launch resolved to `fast` runs **only**
  where every gate is realized on that (runtime × cage), and
  `--allow-degraded` is *never* accepted there: it is the operator's
  consent to a weak wall, not to a weak reader. A PID may also pin
  `tier_floor:` (e.g. the analyst persona pins `standard` — "no commitments" is prose-only);
  below it the launch refuses in the same shape as an unrealized gate,
  and, like a `cage:` shortfall, only an explicit `--allow-degraded`
  above `fast` takes it. Dispatch checks both **per bead, before the
  claim** (`CheckTier`), because the tier comes from the bead's labels:
  the same live persona takes a `standard` bead and refuses a `fast` one,
  which is reported like a degraded launch and skips that bead alone.
  Concretely: the security persona at `fast` on claude is refused until `cage: seatbelt`
  (or codex, whose `-s read-only` is OS-enforced); the analyst refuses a
  `tier:fast` label anywhere. `posse agent check` flags a PID whose own
  `tier:` sits below its own `tier_floor:`.
- **`BD_ACTOR` is attribution, not authorization** (see Identity above):
  any process can claim or close beads as any persona; the identity is an
  audit trail. Accepted at one-OS-user scale.
- **Env sets are inherited, not contained.** Values injected into a
  session's environment are readable by everything in it — an agent
  could echo or commit them; persona sessions get only what their PID
  names (`envs:`). Secrets that matter belong in a secret manager invoked
  at process start (see below), not in broadly-applied sets.
- **Real isolation** (containers/jails for every session) is
  disproportionate for a local single-operator fleet whose beads the
  operator writes; it is a tier a PID opts into (`cage: container`), not
  the fleet path. Deploy accordingly: every persona you dispatch below
  that tier is trusted with everything your user can do, minus the wall.

## Container tier (L4): what the spike measured (rangerhq-89a)

ADR 0002 §3 reserves `cage: container` for `egress:`, hostile input and
untrusted runtimes, and marked three things unassumed. All three were
probed on the development host on **2026-08-18** — macOS 26.4 with
Docker Desktop (VirtioFS). `docs/adr/0002-container-tier.probe.sh`
re-runs every probe below and prints the versions it ran against. The
tier is built end to end (rangerhq-9fv, rangerhq-1k1, rangerhq-9d0,
rangerhq-6so — *What is built* at the end of this section).

| question ADR 0002 left open | measured answer |
|---|---|
| repo + `{memory}` bind mounts | work, and VirtioFS maps ownership — a file written as root inside lands owned by the operator outside. `git` inside wants one `safe.directory` line |
| herdr socket passthrough | **works, and only in one shape**: a socket bind-mounted as a **file** (`-v …/herdr.sock:…/herdr.sock`) carries a full `agent.list` round-trip. A socket reached through a bind-mounted **directory** does not: `connect(2)` answers `ENOTSUP` on VirtioFS, read-write or read-only alike (re-measured 2026-08-22, Docker 29.0.1, rangerhq-6so). That is why `sockets:` mounts the file, and why `.beads/bd.sock` on the repo mount is **not** a route into the cage |
| herdr detection through `docker run` | **breaks, and the fix is one launcher** (below) |
| egress allowlist proxy | **realized, and enforced by routing, not by env var** (below) |
| VirtioFS build tax | ~8% on a cold `go build ./...` of this repo (4.84s bind-mounted vs 4.49s on the container's own fs; the host itself takes 5.33s). Warm image start is ~0.2s. Not the tax ADR 0002 assumed |
| auth for claude/codex/grok inside | **the actual blocker** (below) |

**Detection: herdr identifies by argv0, then scrapes the terminal.**
`herdr agent explain` names the manifest and rule behind every state, and
the rules are pure screen-scraping (`osc_title`, prompt-box text) — so the
*scraping* half crosses the boundary intact: an OSC title printed from
inside a container arrives in herdr's pane state verbatim. The *identity*
half does not. herdr picks the manifest from the pane's foreground
`argv0`; with `docker run …` in the pane, `argv0` is `docker`, and
`agent explain` answers `agent_not_found` — no agent, so no dispatch, no
`posse list` status. Proven both ways: a real claude inside a container is
not found, while the *same* claude screen on the host (fresh `HOME`, same
onboarding wizard) resolves to `agent: claude` by
`default_known_agent_idle_fallback`; and `/bin/sleep` symlinked as
`claude` is identified as claude with no claude anywhere in sight.

The fix is a per-persona **launcher named after the runtime's canonical
executable** — `state/cages/<persona>/bin/claude` → a small binary that
`exec`s the engine with `argv[0]` set back to `claude`. Verified: with
the launcher in front, `herdr agent list` shows the containerized session
as `claude`/`idle` and dispatch can target it. Two things it must be:
a **binary or a symlink to one** (a `#!/bin/sh` wrapper hands herdr
`argv0=sh`), and a **separate dir from `gates/<persona>/bin`**, whose
entries are refusing shims — a launcher named `claude` next to a gate
named `git` is a name collision waiting to happen. Built in
rangerhq-1k1 — *What is built* below.

**Egress is the one gate only this tier realizes, and it is real.** The
realization is the route: the agent's container joins a `--internal`
docker network (no default route, no external DNS) whose only other
member is a CONNECT proxy holding the PID's `egress:` list; the proxy is
also on a normal network. Measured inside that cage: direct HTTPS → curl
exit 6 (nothing to resolve, nowhere to route), allowlisted host through
the proxy → 401 from Anthropic (it arrived), any other host → the proxy's
403. An agent that ignores `HTTPS_PROXY` reaches *nothing*, which is the
property that makes this a boundary and not politeness. All three
runtimes do honour `HTTPS_PROXY`/`HTTP_PROXY`, verified live against a
logging proxy, and the hosts each one needs are:

| runtime | hosts it opens | on denial |
|---|---|---|
| claude | `api.anthropic.com`, `platform.claude.com` (OAuth refresh); `http-intake.logs.us5.datadoghq.com` telemetry | degrades quietly — the turn completed with MCP and telemetry hosts denied |
| codex | `chatgpt.com` (incl. `wss://` over CONNECT), `ab.chatgpt.com` | fails **loudly**: ~70 CONNECT attempts in 35s, websocket→HTTPS fallback, then a hard error. An `egress:` typo is a retry storm |
| grok | `cli-chat-proxy.grok.com`, `grok.com`; `api.mixpanel.com` telemetry | quiet |

Add whatever the persona's work needs (`github.com`, `proxy.golang.org`,
`registry.npmjs.org`, any MCP host). Two honest limits: the proxy sees
only the CONNECT authority, so it stops *unknown* hosts, not
exfiltration through an allowed one; and it is not the runtime's own
`--disable-web-search`, which stays L0. Built in rangerhq-9d0 — *What is
built* below.

**Auth is what actually gates the tier.** None of the three needs the
macOS keychain — each keeps credentials in a file (`~/.claude/.credentials.json`,
`~/.codex/auth.json`, `~/.grok/auth.json`) — but on a keychain-authenticated
host claude's file can be a stale leftover of the keychain login: a
container claude with a stale file mounted read-only answers an expired-token
401, and read-only would kill the refresh even if it were live. The supported
way in is an env credential the operator mints once
(`CLAUDE_CODE_OAUTH_TOKEN`, or `ANTHROPIC_API_KEY` — which is metered
spending and therefore the operator's call, not a persona's); both names
are in the shipped binary. A container also starts with no `~/.claude.json`,
so a fresh claude opens the theme/onboarding wizard and the workspace is
untrusted (`projects["/work"].hasTrustDialogAccepted`) — the launch has to
seed that file the way ADR 0002 already seeds codex's `trust_level`.

**Engine: stay on Docker, keep the engine a template.** Docker Desktop is
installed and answered every question above; the two alternatives were
not installed and were not installed *for* the spike. OrbStack is
docker-CLI compatible (a swap, not a rewrite) but needs a paid licence
for commercial use — an operator decision, not a persona's. Apple
`container` (macOS 26) is the plausible successor to the whole seatbelt
lineage, but its per-container VM changes exactly the two things this
spike had to verify: whether a host unix socket survives its mount layer
(the herdr socket), and how an internal-network-plus-proxy topology is
expressed on vmnet. Both re-open, so the engine belongs behind a
one-line command template — the same shape as `runtimes/<name>.yaml` —
rather than hard-coded `docker run`.

**Still open, deliberately.** `bd` and `posse` are macOS binaries; a Linux
container needs its own builds before a caged persona can claim or close
anything (both are in the image — *What is built* below). The
herdr socket is a fleet-wide capability — a persona holding it can prompt
or close every *other* session, so the
cage leaks sideways unless the mount is opt-in. And L1/L3 do not follow
the process in: a shim `exec`s a host path resolved at render time and
the gate shell points at the host's zsh. Both were decided in ADR 0002 as
amended (rangerhq-rm5): the socket is off unless the PID says `sockets:
[herdr]`; the tiers stay cumulative in what they realize, so gates are
**rendered inside** the container by the image's Linux `posse` (`posse gates
wrap`), the pre-push hook rides in on the repo mount, and the repo goes
`:ro` for `Edit`/`Write` denies — all built in rangerhq-6so, below.

**What is built (rangerhq-9fv): the engine, the image, the launch.**
`internal/rhq/cage.go`. The engine is a command template, never a
hard-coded `docker run`: built-in `docker`, `RHQ_HOME/cages/<name>.yaml`
for anything else (only `command:` is required — `mount:`/`mount_ro:`/
`env:`/`home:`/`build:`/`probe:` default to docker's spellings, which is
what makes OrbStack a swap), chosen by config `default_engine:`. `posse
cage` prints engine, image and readiness; `posse cage <persona>` prints
what would cross the boundary; `posse cage build [dir]` cross-builds a
Linux `posse` and `bd` 0.49.1 out of a posse checkout and hands
`etc/cage/Dockerfile` plus those binaries to the engine (~45s, ~600MB;
`--runtimes` adds CLIs — claude is installed by default because it is
the only runtime whose container credential is decided).

Mounts are **same-path in and out**, so the one rendering of the runtime
command works on both sides: the session dir, the persona's memory, its
PID (`:ro`), its bound skills (`:ro`), and a per-persona cage HOME
(`state/cages/<p>/home`, mounted at the image's HOME) — and nothing else
of the host. Env crosses as **names only** (`-e NAME`), so the
operator's credential is never on a line that lands in pane scrollback.
That credential is a *precondition*, not a gate: no
`CLAUDE_CODE_OAUTH_TOKEN` in the session env, no launch (rangerhq-kiz),
and codex/grok refuse outright until theirs is decided. The cage HOME is
seeded with `~/.claude.json` (onboarding, theme, `autoUpdates: false`,
`projects[<dir>].hasTrustDialogAccepted`) the way ADR 0002 seeds codex's
`trust_level`, merged rather than overwritten.

**The pane runs the argv0 launcher (rangerhq-1k1), not the engine.**
`internal/rhq/cagelauncher.go`. `state/cages/<persona>/bin/claude` is a
symlink to the running `posse`, which recognizes its own second entry point
(`<launcher> --posse-cage <plan>`) and `execve`s the engine with `argv[0]`
reset to the runtime's name — so herdr identifies the session as `claude`
instead of `docker`, and dispatch can target it. The binary is posse itself
because the launcher has to be a binary (a `#!/bin/sh` wrapper hands herdr
`argv0=sh`) and nothing then has to be built, shipped or kept in step with
posse's version; the directory is its own, never the gates bin. Because it
is a real exec, nothing of posse lingers in the pane and the pane's process
dies with the container — which is what `RelaunchAgent` re-types the same
short line for.

That is also why the engine rendering is an **argv**, not a shell line: a
launcher that handed the line to `sh -c` would be replaced by the engine
on that shell's own exec and hand herdr `docker` right back. Nothing is
shell-quoted on this path (a mount path with a space is simply one
argument), and an engine `command:` is read as an argv template — pipes
and redirections have no shell to read them. The inner runtime command
*does* keep a shell, `sh -c 'exec <cmd>'` **inside** the container, where
the same-path mounts make `$(cat {file})` read the same file the host's
shell would have.

The argv itself lives in `state/cages/<persona>/<session>.argv` (JSON:
the engine to exec, the argv to exec it with, and the line as a human
reads it) — per *session*, because a persona holds one per bead. It is a
file and not a typed line because an engine argv carries every mount and
every forwarded name: rendered as a line it runs past 1.5KB, and a
command that long **does not survive being typed into a freshly created
workspace** — a fresh pane's tty is still in canonical mode and takes
1023 bytes, so the head is echoed raw and nothing runs (*The launch line
is typed, so it has a limit* above; rangerhq-ybec). This tier met that
first and rendered a file for its own reason as well — the argv0 the
launch must leave behind — and the general rule now covers every launch,
so nothing here needs to keep it alone. Launcher and plan are rendered
fresh from the PID at every launch, like the gates.

And parity says only what the tier holds
*today*, which since rangerhq-6so it asks the **image** rather than a
constant: a caged PID with a shell-verb or `Edit`/`Write` gate launches
clean only when the image answers `posse gates wrap --probe`, and is
**refused** with the reason when it does not. That is the ADR's own rule —
the strongest cage is never the one that silently loses `git push`.

**The egress route (rangerhq-9d0): the one gate only this tier realizes.**
`internal/rhq/egress.go`. Unconditional at `cage: container`, not
conditional on the PID having typed `egress:` — the ADR describes L4 as a
container joined to a `--internal` network whose only other member is the
proxy, and a cage whose network is open *by omission* is exactly the
politeness this tier exists to stop being. A PID that names no hosts still
gets the runtime's own (`Runtime.Egress`: claude → `api.anthropic.com` +
`platform.claude.com`, codex → `chatgpt.com` + `ab.chatgpt.com`, grok →
`cli-chat-proxy.grok.com` + `grok.com`; telemetry hosts are deliberately
absent). `posse cage <persona>` prints the effective list.

The allowlist is rendered fresh from the PID at every launch, like the
gates — `state/cages/<p>/<session>.egress.hosts`, per *session* — and a
host that is not a host (a URL **path**, above all: the proxy matches the
CONNECT authority and never sees one) refuses the launch instead of being
silently widened. The proxy is `state/cages/<p>/egress.js` running on the
cage image's own node, so the tier needs no second image; it mounts the
script and the allowlist `:ro` and `gates/<p>/refusals.log` writable, and
**nothing else** — no repo, no memory, no HOME, and no `-e` at all, so the
process that terminates the agent's TLS handshakes never holds the
operator's credential. Denials land in that log in L1's own shape
(`… CONNECT example.com:443 [egress proxy] (deny: not in egress: …)`), so
`posse gates <persona>` is where an `egress:` typo shows up — which matters
because codex answers a denied host with ~70 retries in 35s and then a
hard error. Only CONNECT crosses: a plain-HTTP proxy request is refused
and named rather than forwarded.

The engine spells the route, the way it spells everything else:
`net:`/`net_create:`/`net_join:`/`net_remove:`/`proxy_up:`/`proxy_down:`,
and — unlike `mount:`/`env:` — a `cages/<name>.yaml` does **not** inherit
docker's spelling for them, because an engine whose reason to exist is
that it expresses topology differently (Apple `container` on vmnet) must
not silently launch a cage with no boundary. An engine that says nothing
realizes no `egress:`, and parity refuses the launch with that named.

Lifecycle is a pid, not an engine event. The launcher `execve`s the
engine, so nothing of posse is left in the pane to notice the container
exiting — so just before that exec it brings the route up and forks one
watcher, `posse --posse-cage-reap <plan> <pid>`, in its own session, holding
the launcher's pid *which the exec does not change*. When that parent goes
away the route comes down. Two consequences worth knowing: killing the
engine's client leaves its container running (docker's behaviour, not
ours) and the proxy still dies, which leaves that container with **no**
route out — fail-closed, the right direction; and a watcher killed with
its pane leaves a proxy behind, which `posse cage down <persona>` removes.

Verified live 2026-08-22 (ADR 0002 verification 9,
`TestLiveEgressBoundaryIsTheRouteNotTheEnvVar`, `RHQ_LIVE_DOCKER=1`):
inside the cage, direct HTTPS → curl exit 6, external DNS resolves
nothing, an allowlisted host through the proxy → 401 from Anthropic (it
arrived), any other host → curl exit 56 with the denial in
`refusals.log`; and the watcher really does take network and proxy with it
when its parent exits.

**The wall inside (rangerhq-6so): L1/L3 in the cage, the mount boundary,
`sockets:`.** `internal/rhq/cageinner.go`. The tiers are cumulative in
*gates realized*, and the mechanism is a fourth entry point rather than a
mount: the inner command of a container launch is

```
posse gates wrap <persona> [--no-gate-shell] -- sh -c 'exec <runtime cmd>'
```

and the image's own Linux `posse` runs the **same renderer** (`RenderGates` +
`renderGateShell`) against the image's PATH and shell, then `execve`s what
follows with the same prefix the host types — `PATH=<bin>:$PATH`,
`SHELL`/`GROK_SHELL` at the gate shell, and `RHQ_GATES_DIR` at the inner
render. Mounting the host's `gates/<p>/` instead was rejected in the ADR
and the reason is load-bearing: those shims `exec` `/opt/homebrew/bin/git`
and that gate shell `exec`s the host's zsh, and a shim that dies is a
shell verb that is **not refused**. Three consequences worth knowing:

- The deny list crosses as **`RHQ_TOOLS_DENY`**, not as the PID: `RHQ_HOME`
  is not in the cage at all, and that var is already the launch's own
  rendering of `deny:` — the same one the pre-push hook reads. One source,
  both layers.
- The gates render to **`/posse/gates/<persona>`, a path of the image's**, so
  two caged sessions of one persona cannot clear each other's shims. The
  one file that must outlive the container — `refusals.log` — is
  bind-mounted out to `gates/<p>/refusals.log`, where L1's host refusals
  and the egress proxy's 403s already land.
- `realShell` now **resolves a shell that exists** (`$SHELL` if it is a
  real bash/zsh, else zsh, else bash, else `/bin/sh`). Inside the image
  `$SHELL` is unset and `/bin/zsh` is not there; the old hard-coded
  fallback would have rendered a wrapper that cannot `exec` its own REAL.

**The mount boundary** is the repo, and only the repo: `:ro` when the PID
denies `Edit`/`Write`/`NotebookEdit` (any one of the three — the boundary
is a property of the mount, not of a rule), while `{memory}`, the cage
HOME and the refusals log stay writable. A **worktree** gets its git
common dir mounted alongside, because `.git` there is a *file* pointing at
the main repo's `.git` and L3's hooks live in it — a hook the container
cannot see is a `git push` this tier lost.

**`sockets:`** is the PID's opt-in to a host socket (`herdr` is the only
name; an unknown one refuses the launch and `posse agent check` flags it).
Off by default because a caged persona holding the herdr socket can prompt
or close every other pane. When declared: the socket is bind-mounted as a
**file**, `HERDR_SOCKET_PATH` names it (HOME inside is the image's, so
posse's own default would look in `/root`), the meta records `sockets:`, and
`posse list` and the cockpit show `🔒container+herdr`. Declared, so not
refused — but stated where the operator reads sessions.

**Parity asks the image, not a constant.** `ContainerInnerGates` is now
`true`, but it is a necessary condition: `CageInnerGatesReady` runs
`posse gates wrap --probe` in the image through a new engine key `inner:`
(docker's `docker run --rm {image} {cmd}`; an engine that spells none
cannot be asked and answers yes, on `probe:`'s precedent), cached per
process. An image with no Linux `posse` leaves every shell-verb and
`Edit`/`Write` deny **unrealized** at this tier, named with `posse cage
build` as the fix — and the launch renders no `posse gates wrap` on the
inner line either, so a waived launch still starts.

Verified live 2026-08-22 (ADR 0002 verification 8 and 10,
`TestLiveInnerGatesHoldInsideTheCage`, `RHQ_LIVE_DOCKER=1`): inside a real
cage, `command -v git` → `/posse/gates/p/bin/git`, `SHELL` →
`/posse/gates/p/shell/bash`, `git push` prints the L1 refusal,
`/usr/bin/git push` prints the L3 hook's, `touch` fails on the `:ro` repo,
**both** refusals land in the host's `refusals.log`, the same holds from a
worktree, and the herdr socket is absent by default and *connectable* with
`sockets: [herdr]`.

**What it costs, measured on the day (2026-08-22, Docker 29.0.1):** a unix
socket reached through a bind-mounted **directory** is not connectable —
`connect(2)` answers `ENOTSUP` on VirtioFS, and read-only has nothing to
do with it (a read-write mount fails identically). So **`.beads/bd.sock`
is not a route into the cage** — ADR 0002's verification-8 line assumed it
was — and `bd` inside falls back to direct storage mode. On a read-write
repo mount that works (measured: `bd q`, `bd comments add`, `bd close` all
succeed, ~5s per command while bd tries and fails to start a daemon). On a
**`:ro`** repo mount it does not: SQLite cannot open a database read-only,
so a caged persona denying `Edit`/`Write` — the security persona, the
reviewer skeletons, exactly the PIDs this tier is for — **cannot claim,
comment or close a bead**. **Answered by ADR 0014 §4:** L4's `:ro` repo
carries L2's `.beads`/`.git` carve-outs as read-write overlays, so the
wall is the rest of the tree and bd still works. Lands with that ADR's
L4 implementation bead; overlapping binds on VirtioFS are ASSUMED until
that bead measures them.

## Cage engine re-evaluation: still Docker (rangerhq-rli)

The 89a spike recommended Docker on the grounds that it was installed and
answered every question, and handed the two alternatives forward as a
decision about whether they were worth a machine change. Re-evaluated
**2026-08-23**; verified live against each project's own tracker on that
date, and the one executable half measured on this host (macOS 26.4.1,
Docker 29.0.1). **The engine stays `docker` and no `cages/<name>.yaml`
was written.** Neither candidate earns the change today, and the reasons
are different in kind.

**Apple `container` — the topology is expressible; it does not hold.**
Version 1.2.2 (2026-08-08); 1.0.0 landed 2026-06-09. Not installed here,
and deliberately not: the blocking defect is reproduced upstream with
exact commands, so installing to re-confirm it buys nothing.

The two questions 89a said would re-open:

| re-opened question | answer, 2026-08-23 |
|---|---|
| does a host unix socket survive its mount layer? | **yes, and by a better mechanism than docker's.** `container` does not virtiofs-share the socket — `container-runtime-linux` runs a socket **relay**, and 1.1.0 (issue #1750, PR #1751 "Propagate permissions for all host-to-container socket mounts") made it carry the host's permission bits so non-root workloads can use it. The single-file virtiofs limitation that would otherwise sink `sockets: [herdr]` (apple/containerization#79 — "virtiofs doesn't support sharing single files") does not apply to sockets. Unmeasured here; nothing is installed |
| how is "internal network whose only route is an allowlist proxy" expressed on vmnet? | **exactly the way we already express it — and it leaks.** `container network create --internal` exists (`"mode": "hostOnly"`), and upstream discussion #1170 describes the same dual-homed-proxy shape posse built. But **apple/container#2062** (open, filed 2026-08-03, reproduced on the signed 1.2.0 release): a `hostOnly` network **NATs arbitrary outbound TCP**. Raw-IP HTTPS to GitHub answers `HTTP 200`, and `1.1.1.1/cdn-cgi/trace` reports the host's public IP — identical to the default NAT network. UDP and ICMP are blocked. Fix PR **#2072** is open and unmerged as of 2026-08-13, so it is in no release |

Stated as our own property: at L4 the thing that makes the cage a
boundary is *an agent that ignores `HTTPS_PROXY` reaches nothing*. On
Apple `container` today an agent that ignores `HTTPS_PROXY` reaches any
host **by IP**. `posse cage <persona>` would print the same effective
allowlist and the cage would be politeness. `SpellsEgress` asks only
whether an engine can *spell* the route; it cannot know whether the
engine holds it, which is why the probe below matters more than the
template.

Two more, both from upstream's own tracker:

- **The bead's hypothesis — "egress control moves to host `pf` rules" — is
  answered no.** apple/container#1320 (open since 2026-03-17): "macOS `pf`
  firewall rules don't seem to filter vmnet-bridged traffic", and
  guest-side iptables "is bypassable by a root process inside the VM".
  The same issue records that on a `hostOnly` network the **host gateway
  stays reachable**, so any host service bound to `0.0.0.0` is in reach
  from inside the cage — something docker's `--internal` does not give
  away. Its filer's use case is, word for word, sandboxing autonomous AI
  coding agents.
- **Posture.** 1.2.0 shipped four security advisories, one of them
  host-file disclosure out of the build context via symlinks
  (CVE-2026-64777). An engine adopted *for* isolation, ten weeks past
  1.0.0, with an open isolation defect in its isolation flag, is not
  where a security tier moves.

Re-open when #2062 ships in a release — filed as a bead, so it is not
left to anyone remembering.

**OrbStack — no engine work to do, and no measured problem to solve.** It
answers docker's CLI, so the **built-in `docker` template already is its
template**: the swap is an install and a `default_engine:` that does not
even change, not a `cages/orbstack.yaml`. What OrbStack sells over Docker
Desktop is file-sharing speed — and the tax it would cut is the one 89a
measured at ~8% on a cold `go build ./...` of this repo (4.84s
bind-mounted vs 4.49s on the container fs; the *host* is slower still at
5.33s). That is noise, and ADR 0002 had already dressed that number as
larger than it is. Its commercial licence is the operator's line and it
is not worth asking to fix an 8% nobody has felt. If the operator ever
installs OrbStack for their own reasons, posse needs no change.

**What the re-evaluation actually found was in our house.** Our boundary
check had upstream's bug. Probe 2 of `0002-container-tier.probe.sh`, check
1 of `TestLiveEgressBoundaryIsTheRouteNotTheEnvVar`, and the header of
`internal/rhq/egress.go` all rested on one assertion:

```
curl https://api.anthropic.com/v1/models  →  exit 6
```

curl's exit 6 is **"couldn't resolve host."** It proves the resolver is
gone. It says *nothing about the route* — which is the identical false
positive that let apple/container's own `testIsolatedNetwork` pass over a
live leak for months. An engine that blocks UDP/DNS while still NAT-ing
outbound TCP would have passed every check we had.

Measured on this host 2026-08-23, docker 29.0.1:

| from a container | on the cage's `--internal` net | on the default bridge |
|---|---|---|
| `https://api.anthropic.com` (hostname) | curl exit 6 | — |
| `https://1.1.1.1/` (raw IP) | curl **exit 7** | exit 0, **http 301** |
| `https://140.82.121.4/` `Host: github.com` | curl **exit 7** | **http 200** |
| `http://8.8.8.8:53/` | curl exit 7 | — |

Docker's `--internal` is sound: it takes the route, not just the
resolver. The claim in this file was right; only the test could not tell
the difference — and the right-hand column is what makes exit 7 mean
something. All three sites now assert the raw-IP result too, non-zero
rather than exactly 7 (docker refuses and gives 7; an engine that drops
would give 28 — both are the boundary holding, 0 is the boundary gone).
`TestLiveEgressBoundaryIsTheRouteNotTheEnvVar` passes with the new checks
against a freshly built `posse-cage:latest`.

**So the deliverable of an engine re-evaluation was not an engine.** It
was the probe that can tell a real boundary from a DNS outage — which is
what any future candidate now has to survive.

Also measured while verifying, since the numbers are in this file: `posse
cage build ~/src/rangerhq` takes **11s** with a warm layer cache (89a's
~45s was cold) and the image is **1.23GB** on disk by `docker images`.
The image was removed afterwards; `posse cage` reports "image not built"
here, as it did before.

## Privacy model

The boundary is structural (ADR 0012): **the harness repo is the
product; an operator's instance lives in its own private repo** — the
real RHQ_HOME: tuned personas/PIDs, crew memory, config values, recipes,
env sets, skills bindings, and the operational beads database. Nothing
that describes what the operator works on, spends, or authenticates
with belongs in the harness repo. In brief:

- **Beads inherit their repo's visibility — and bead CLASS, not codebase,
  picks the repo.** A bead belongs in a public repo's db only when any
  deployer of this software could have written it: work on the repo's own
  code/design/docs, findings about the public source (disclosure rule
  below), QA of that work. Everything that describes ONE deployment goes
  in that instance's private db, even when the code it touches is this
  repo's:
    - instance ops: arming/scheduling state, guard thresholds, live
      config values, degraded-mode acceptances
    - cost and plan data: any dollar figure, plan name or size, window
      calibration, usage percentages, spend history or per-persona breakdowns
    - credentials: names, storage locations, liveness, auth topology —
      even with no secret values; "where the keys live" is a map
    - security posture of the deployment (what is set/unset/accepted
      HERE); posture of the software itself stays public
    - anything naming the operator's accounts, plans, or usage habits
  When a public code bead needs instance numbers to justify itself, the
  numbers live in a private bead and the public bead cites its id. When
  in doubt: private. A private bead can be re-filed public later; the
  reverse is a history purge.
  The rule covers the whole `.beads/` sibling set, not just the db:
  `deleted.jsonl`, the git-tracked deletion ledger (rangerhq-fuom), holds
  whole bead records — descriptions and comments included — and is written
  only from its own repo's census, so it inherits that repo's visibility
  exactly as `issues.jsonl` does.
  The config `beads:` list aggregates across repos *locally*, nothing is
  ever copied between repos. `.beads/interactions.jsonl` is a runtime log
  and stays untracked.
- **Security findings** about software others might run go in a private
  database until the fix lands, then get disclosed. When in doubt,
  private first.
- **Env sets, personas/memory, skills** are config under `~/.config/posse/`,
  never repo content; `examples/` holds generic placeholders only. An existing
  `~/.config/rhq/` remains the home while the new path is absent.
- **Local settings** — `.claude/settings.local.json` (personal
  permissions) is gitignored; the committed `.claude/settings.json`
  carries only the fleet allowlist plus a `deny` block (verb-scoped:
  see *fleet security posture* above; prefix-match patterns are an
  accident guard, not a security boundary — rangerhq-lkf, rangerhq-721).

Before making any harness-managed repo public, check the same three:
its `.beads/`, any committed env files, and agent/persona content.

**The visibility guard (rangerhq-hrz)** is the seatbelt under that first
rule, not a second copy of it. `posse gates install-hooks` stamps each
repo's `prepare-commit-msg` hook with what config `beads_visibility:` says
about that repo's beads db, and in a **public**-stamped repo the hook greps
the ADDED lines of `.beads/*.jsonl` for instance-ops content — dollar
figures, plan names and sizes, live `plan_guard_*` / `budget_*` /
`autostart_*` values, credential locations — and refuses the commit naming
the pattern and the rule. Marks live in the operator's config as a
one-level map beside `beads:`, explicit and local (no `gh` or network
lookup at guard time):

```yaml
beads_visibility:
  ~/src/myproject: public
  ~/src/myproject-instance: private
```

**UNMARKED IS PUBLIC** — fail closed, so a newly added repo is guarded
until someone states it is private rather than silent until someone
remembers to mark it public. A typo'd path, an unreadable config and a
value that is neither word all come back public too: every way of being
unsure fails the same way. The commit hook is the enforcement point
because it catches every entry path — `bd create`, comments, `bd sync`,
hand edits — unlike wrapping bd's argv; the slot is `prepare-commit-msg`
and not `pre-commit` for the reason the shared-index guard already
documents (pre-commit is bd's own flush hook, which bd reinstalls
silently, and it is skipped by `--no-verify`). The verdict is *stamped* at
install rather than read at commit time — a commit-time read would mean a
flat-YAML parser in POSIX sh, the one thing this repo stopped doing — and
the stamp is refreshed by `posse gates install-hooks` **and by every
persona launch into the repo**, so a mark the operator changes is in force
on the next dispatch. The way through is to re-file the bead in the
private db and cite its id; the override is
`RHQ_VISIBILITY_OVERRIDE=i-mean-it`, operator-typed, never in a session's
environment (a test pins that), and it is logged to `refusals.log` when it
is used.

**And it is a lint, not a boundary** — same class as the allowlist. The
boundary is the routing rule plus repo visibility; the lint exists so a
mis-routed bead is a refusal at commit time instead of a public artifact.

The pattern list lives in one place (`OpsPatterns`, `internal/rhq/visibility.go`)
and is read twice — by Go and by `grep -E` in the hook — so it is written in
the intersection of both dialects. The start set from the bead was narrowed
against this repo's own 402-bead db rather than argued: bare `\$[0-9]` hit 37
beads, 22 of them shell positional parameters quoted in beads about these
hooks; the config-key class needs a *value* after the colon, because
`budget_pass:` and `plan_guard_5h:` are this harness's public vocabulary and
appear in prose constantly; bare `keychain` and `~/.config/rhq` are the
software's own documented mechanism and were dropped. Residual, stated: a
vendor's public list price (`$3/MTok`) trips the cost class and a
single-digit amount (`$0`) does not.

## beads (bd) substrate: pinned at 0.49.1, 0.51+ is a migration (rangerhq-f49)

The fleet runs `bd` **0.49.1** on purpose. beads removed the SQLite backend
for embedded Dolt at **v0.51.0** — not at 1.2.x, as this section said until
ranger-base-pkqn measured it: any binary ≥ 0.51 does not read
`.beads/beads.db` at all (`bd list` → "no beads database found") and, on first
invocation, silently
`bd init`s an embedded Dolt DB under `.beads/embeddeddolt/` seeded from
whatever `issues.jsonl` says — i.e. a stale fork of the fleet's state, at the
last flush. `brew upgrade beads` on 2026-08-16 broke `bd` for every persona
session for ~3 minutes; rolled back to the upstream v0.49.1 release binary
(sha256-verified) at `~/.local/bin/bd`, with Homebrew's 1.2.2 left installed
but unlinked. Both this repo and the instance repo are on 0.49.1 SQLite.

`make verify-bd-pin` asserts that pin against the live box — version, which
binary `bd` actually resolves to, homebrew's keg still unlinked, and every live
`bd daemon` running the pinned binary and younger than it. Read-only: it never
kills, links or installs anything. See *"The pin is not enforced"* below for
why the process-layer half exists.

Also present as a brew-managed pin: a local tap formula `beads@0.49.1`
(operator-side), which installs the same release tarball; `brew install`
of it currently fails only because the Command Line Tools are older than
brew wants.

**What 1.2 changes for this harness (decide before migrating):**
- Storage is Dolt (git-like DB with its own history and remotes). The
  git-tracked `.beads/issues.jsonl` is demoted to an optional export
  (`export.auto`, off by default, throttled to 60s) — "for viewers and
  interchange, not backup". The pre-commit flush hook, the JSONL merge
  driver, `bd sync` semantics, the metrics-from-JSONL-history idea (see
  Personas), and the fleet allowlist verbs all assume JSONL-as-truth.
- New surface the allowlist has never seen: `bd dolt …` (remotes, auto-push
  policy), `bd compact`/`bd flatten`/`bd gc`, `--sandbox` (disables Dolt
  auto-push), `beads.role` git config, `bd setup <tool>` integrations, and
  anonymous usage metrics **on by default** (`bd metrics off`).
- Migration is `bd init --from-jsonl` per repo (after a fresh 0.49.1 flush:
  `bd sync --flush-only`), not `bd migrate`. `bd init`, `bd import`, and
  `bd migrate` are in the persona deny block, correctly — this is an
  operator change.

**Operator runbook (forward), per repo:**
```
bd sync --flush-only && git add .beads/issues.jsonl && git commit -m 'bd: flush before 1.2 migration'
rm ~/.local/bin/bd && brew link beads         # 1.2.2 becomes bd
bd metrics off
bd init --from-jsonl && bd list | head       # embedded Dolt, seeded from JSONL
bd config set export.auto true                # keep issues.jsonl alive for git/metrics (recommended)
bd doctor --fix
```
Then re-audit `.claude/settings.json` (allow/deny) against `bd help`.
Rollback: `brew unlink beads && install -m 0755 <v0.49.1 bd> ~/.local/bin/bd`;
the SQLite `beads.db` is untouched by 1.2 and still valid.

**Still open from the 0.49.1 `bd doctor` (operator, all in the deny block):**
- pre-push hook: `bd hooks install` (each repo).
- sync-branch: `bd config set sync.branch beads-sync` — *not* recommended for
  a single-clone-per-repo setup; it turns `bd sync` into a committer/pusher
  on a side branch, which is the exact surface rangerhq-lkf fenced off.
- instance repo fingerprint (origin URL changed): `bd migrate --update-repo-id`.

Lesson, now standing orders for devops: a substrate upgrade that any live
session depends on gets canaried first — run the new binary from the fetched
bottle against a *copy* of state in scratch, then upgrade.

### The pin is not enforced, and the 08-16 rollback left a live process (ranger-base-31md)

The 08-26 lock storm was not a new failure. It was the tail of the 08-16 one.
Measured 2026-08-27, all of it live on this box:

**The orphan's provenance.** `.beads/daemon.log` line 1 is
`2026-08-13T23:28:26` — the orphan's own start, and it matches the 12d21h age
at diagnosis. Its binary was `/opt/homebrew/bin/bd`, deleted underneath it by
the `brew upgrade beads` of 08-16 that caused the first outage. The rollback
that day was verified at the **command** layer (`bd version`, `bd ready`,
`dispatch --dry-run`, all green) and never at the **process** layer. *A
rollback that leaves a live process from the reverted artifact is not a
rollback.* That process ran unnoticed for 12d21h and then detonated.

**The pin rests on two soft facts and nothing asserts either.**
- `bd version` 0.49.1 from `~/.local/bin/bd` — a hand-placed binary.
- Homebrew `beads` **1.2.2 installed, `linked_keg` None** — an unlinked keg at
  `/opt/homebrew/Cellar/beads/1.2.2` (135.6MB), pulled in as a dependency of
  the *linked* `gastown` 0.5.0, next to `dolt` 2.3.0.
- `/opt/homebrew/bin` precedes `~/.local/bin` in the fleet PATH. Relink that
  keg — `brew upgrade`, `brew link beads`, a `gastown` reinstall — and 1.2.2
  wins silently, which is precisely the 08-16 outage re-armed.
- `beads` is **not `brew pin`ned** (`brew list --pinned` empty, no
  `/opt/homebrew/var/homebrew/pinned`). The operator's own tap formula
  `davidstacy/local/beads@0.49.1` exists and is **not installed**.
- `make` carries `verify-grok-pin`, `verify-bd-dep-safety` and
  `verify-bd-no-relate-pairs`. There is **no `verify-bd-pin`** — the one
  substrate whose unpin has already taken the fleet down twice.

**What it cost, priced.** 27 daemon `ERROR`s in the 21:00 hour and 1 in
22:00 — a ~40-minute storm, landing on the leading edge of the shop's densest
block: 08-26 closed **100 beads local**, **38 of them between 21:00 and
23:59**. Nothing was lost; counts clean before and after. The day ran 138
bead-segments / **$491.65 API-equiv**, median $3.27. So the bill was the
operator's Wednesday evening and a degraded prime shift, **not dollars** —
and 12d21h of not looking is the number to fix, not the 40 minutes.

**Detection is one `test -e`, and it is all we can have.** The running
daemon's own binary path either still exists or it does not; `bd version`
either is 0.49.1 or it is not. Remediation is *not* available to any persona —
`Bash(bd daemon:*)` is denied repo-wide and killing a pid is in no PID — so
the check reports and the operator acts. A detector that cannot act still
turns 12d21h into one dispatch pass. Do not build a monitoring subsystem for
a subsystem the vendor deleted; build the `verify-grok-pin`-shaped target
(ranger-base-tdwy; the one-command guard is the operator's, ranger-base-8auf).

**Built, 2026-08-27 — `make verify-bd-pin` (ranger-base-tdwy).** Declaration is
`etc/bd/version-pin.toml` (pinned version, pinned binary, the homebrew formula
whose keg must stay unlinked); the assertion is `scripts/verify-bd-pin.sh`.
Four rows, all read-only, exit 1 on any failure and exit 2 when it cannot
check at all:
- `bd version` == 0.49.1 exactly. Asked as `bd --no-daemon version`, so the
  check never spawns the thing it is checking; a 1.x on PATH rejects that flag
  (0.51.0 deleted the daemon and the flag with it) and the row fails anyway.
- `command -v bd` == the pinned binary. Not "a bd reporting 0.49.1" — *the*
  pinned one. A shadowing 0.49.1 in front of it still fails, because the claim
  is about which inode the fleet runs.
- homebrew's `beads` keg is **unlinked or brew-pinned**. LINKED fails. Parsed
  from `brew info --json=v2` with python3, not sed — `verify-grok-pin`'s sed
  cannot survive pretty-printed JSON (ranger-base-ocfh) and that defect was
  not worth reproducing.
- **the process layer.** For each live `bd daemon` — matched on argv[0]
  basename `bd` with argv[1] `daemon`, so `bd list` and a grep for the phrase
  are not daemons — one of four verdicts: `ORPHAN` (its binary is gone from
  disk), `FOREIGN` (a different, existing binary), `STALE` (started *before*
  the pinned binary's mtime, so it is running an artifact that has since been
  replaced — the 08-16 orphan's exact shape: started 08-13, binary rewritten
  08-16 23:54), or ok.

MEASURED while building it, and the detector rests on it: on macOS
`ps -o comm=` returns an **empty string** once a running process's executable
is unlinked — not a stale path. argv[0] survives in `ps -o args=`, which is
why enumeration reads `args` and identification reads `comm`. Empty and
"path that no longer exists" are the same verdict, so it holds either way.

It acts on nothing. No kill, no `brew pin`, no relink — `Bash(bd daemon:*)` is
denied fleet-wide and killing a pid is in no PID. A failing run prints the
operator's next step and stops; `bdpin_qa_test.go` asserts the script's *code*
(not its report text) contains no such verb. Wired into `scripts/clean-build.sh`
ahead of `git worktree add`, because that is what fires the bd post-checkout
hook and that is how the storm broke `make install` — **advisory there**, never
fatal: the pin's state does not make a build wrong, and a failed check must not
be what stands between the operator and a working binary.

What it still does not do: `brew pin beads` is the belt and remains unset. The
check says so on every clean run ("NOT BELT-AND-BRACES") and names the command
without running it, because installing or pinning a formula is the operator's
hand. Today the pin holds by the absence of a symlink, not by policy.


**The version question, answered — and the money column is a tie at $0.**
beads is MIT, Dolt is Apache-2.0, both already installed here; no vendor, no
plan, no renewal, nothing expires. Cost is never the argument on this chain.
- *"Latest is 1.2.2" is misleading.* From the vendor's own CHANGELOG shipped
  in the keg (`/opt/homebrew/Cellar/beads/1.2.2/CHANGELOG.md`): 1.2.0 and
  1.2.1 were "published by accident on 2026-08-11 without release testing";
  1.2.1 **migrated user databases on first command** ("running any command
  once was enough"), leaving them unopenable by the older binary. 1.2.2
  (08-15) is **the v1.1.2 code re-released under a higher version number**,
  with the 1.2 features (work leases, events journal, `bd sync`, `bd serve`)
  taken back out. Upgrading to "latest" buys July's code and August's
  release-engineering record.
- *What the upgrade would genuinely fix, and it is real:* **0.51.0 Phase 2
  deleted the daemon** outright (and `--no-daemon` with it). Every mechanism
  in this incident — a daemon auto-spawned per call, `daemon.lock`, an orphan
  holding FDs on a deleted inode — is 0.49.x-only and **cannot recur on 1.x**.
  That is the strongest thing on the migrate side and it should be said out
  loud, not buried.
- *What it costs:* the runbook above, twice, with writers frozen; a re-audit
  of the bd allowlist against a surface it has never seen; and re-verifying
  the three posse guards built around a 0.49.1 bug
  (`verify-bd-dep-safety`, `prune-bd-relates-to`, `verify-bd-no-relate-pairs`)
  against a different cycle-check engine. None of that is priced in dollars
  either — it is operator hands and a frozen queue.
- *The "what if we leave" column, both directions:* truth of record is
  `.beads/issues.jsonl` — 4.1MB, git-tracked, flushed continuously. Forward is
  `bd init --from-jsonl`; out of beads entirely is parsing a documented JSONL
  file. **Lock-in is near zero today, and 1.2 makes it worse, not better**:
  JSONL is demoted to an optional export (`export.auto`, off by default,
  throttled to 60s). The runbook's `bd config set export.auto true` is
  load-bearing, not a nicety.
- **HOLD at 0.49.1 stands** — the operator's call of 2026-08-17 (rangerhq-f49,
  recorded by monica 08-18). Nothing since moves the arithmetic toward
  migrating and the vendor's August moves it away. The uncomfortable half,
  plainly: 0.49.1 is **permanently unsupported** — the SQLite line ends at
  0.50.3 with the `dep add` landmine byte-identical — so we are choosing to
  carry known defects with known workarounds over an untested migration. That
  choice is correct today and must be re-opened on the first *new* 0.49.1
  defect that has **no** workaround, not on the next release announcement.

### `bd dep add` never terminates when the target can reach a `relates-to` pair (ranger-base-pkqn)

MEASURED 2026-08-27 on a scratch copy of the fleet db: `bd --no-daemon dep add
ranger-base-x584 ranger-base-x6ic -t discovered-from` was killed at **300s**,
never having completed. It is not "slow" — it does not finish. The same
command, same db, with the *reverse* `relates-to` edges deleted, completes in
**0s**. That difference is the whole trigger, and it holds the sqlite write
lock the entire time, which is how one edge soft-locks every other bd client
at their 30s lock timeout.

Root cause, read out of the pinned source
(`internal/storage/sqlite/dependencies.go:106`, duplicated verbatim at
`transaction.go:766`): the cycle check is a recursive CTE that walks the
dependency graph outward from the *target*, written with `UNION ALL` — so it
enumerates every **walk**, not every node. No visited set, no memoisation,
bounded only by a fixed depth of 100. It follows **all** dependency types, and
`relates-to` edges are always symmetric (`cmd/bd/relate.go` writes both
directions; bd's own comment calls them "inherently bidirectional"), so every
`relates-to` edge is a 2-cycle the walk bounces across until the depth cap.

Measured expansion of that CTE, run verbatim against this graph from
`ranger-base-x6ic`:

| depth | walks enumerated |
|---|---|
| 2 | 19 |
| 6 | 1,243 |
| 10 | 63,551 |
| 14 | 3,228,573 |
| 18 | 163,915,501 |

≈7.1× per level. bd runs it at **depth 100**.

The asymmetry that makes this read as intermittent: the query is `SELECT
EXISTS(…)`, so a walk that *does* reach the cycle short-circuits and returns
at once. The expensive case is the one where the answer is "no cycle" — the
ordinary, legitimate `dep add`. A **rejected** `dep add` is instant; an
**accepted** one hangs.

bd already knows this walk has to skip `relates-to`: `loadDependencyGraph`
(`dependencies.go:799`) selects `WHERE type != 'relates-to'`, and says why.
The `AddDependency` cycle check is simply inconsistent with it.

**Blast radius, measured 2026-08-27:** 13 nodes sit in a symmetric pair (10
pairs, 20 edges) and **27 nodes were unsafe as a `dep add` target** — unsafe
meaning some 2-cycle node is reachable from it, so the walk explodes. Forty
minutes later it was 28, with no new `relates-to` edge: an ordinary bead had
landed upstream of the cluster. The count is a snapshot; read it, do not
memorise it.
`make verify-bd-dep-safety` prints both lists;
`scripts/verify-bd-dep-safety.sh <id>` exits 1 when that id is unsafe as a
target. ~~The set only grows: every `bd relate` / `bd dep add -t relates-to`
plants another pair, and it cannot be pruned back, because bd rewrites the
reverse edge on the next relate.~~ **Both halves of that sentence are wrong —
measured, ranger-base-nusr, below:** only `bd dep relate` / `bd relate` plants
a pair (`dep add -t relates-to` writes one row), and the pairs prune cleanly
and stay pruned. What grows on its own is the *unsafe target* set, as the
preceding paragraph says.

**Standing rule: do not create `relates-to` edges in this fleet, and never
`dep add` onto an unsafe target — record the provenance as a comment
instead.** This is the same landmine as ranger-base-muoo from the other end:
`bd create --deps discovered-from:<parent>` starts the same CTE at `<parent>`,
so a poisoned parent loses its edge to the 30s timeout while the issue itself
commits — which is how 33 edgeless duplicate verify beads got filed. The
HANDOFF rung of a dispatch prompt (`internal/rhq/dispatch.go`) has that exact
shape. The ASK rung does **not**: its target is a freshly created question
bead with no outgoing edges, so the CTE is empty and returns at once. Leave it
alone.

**There is no drop-in fix.** Upstream did fix it, but only past the end of the
SQLite line:

- v0.49.6 and v0.50.3 carry the **byte-identical** CTE. Canaried 2026-08-27
  against a copy: the v0.50.3 release binary hangs on the same command (killed
  at 180s) and prints `Note: bd v0.50+ defaults to Dolt`.
- v0.51.0 **removes `internal/storage/sqlite` outright**. Measured: the v0.51.0
  binary, run in a directory holding a valid `.beads/beads.db`, answers `Error:
  no beads database found`; it has also dropped `--db` and `--no-daemon`.
- v0.63.3 (`internal/storage/issueops/dependencies.go`) runs the cycle check
  only when the new edge is `blocks`, and traverses only `blocks` edges —
  either change alone kills this bug. The CTE there is still `UNION ALL` with
  no visited set, so the shape survives; it is just no longer reachable from a
  `relates-to` pair.

So "upgrade bd" and "migrate to Dolt" are one decision (rangerhq-f49), not two
options. **Correction to this section's header, and to README.md and
INSTALL.md: the SQLite backend goes away at v0.51.0, not at 1.2.x.** Every
release ≥ 0.51 silently forks the queue, twelve minors earlier than this file
used to claim; 0.50.x still reads `beads.db` but is not worth taking, because
it fixes nothing here.

**`bd` exit codes do not carry the claim result (rangerhq-kux).** 0.49.1
refuses a claim it cannot grant by printing to STDERR and **exiting 0** with
empty stdout:

```
$ bd --actor persona-a update <id> --claim --json     # held by persona-b
stderr: Error updating <id>: operation failed: already claimed by persona-b
exit  : 0
```

The same shape appears when the holder is the actor itself — including a bead
merely *assigned* to it and still `open`, which is bd's first-choice routing
case: `--claim` is refused and the status never moves. So `Bd.Claim` never
trusts the exit code: a won claim is the one where bd hands back the updated
issue (`[{… "status":"in_progress" …}]`); anything else is decided by reading
the bead back, and an `open` bead already assigned to the actor is moved to
`in_progress` explicitly. Losing to another holder is a typed `ClaimLostError`
so dispatch can skip the bead without benching the session.

Audited alongside it (0.49.1): `close`, `comments add`, `update --status`, and
`show` all exit **1** on an unresolvable id, so only the claim-conflict path
has this property — but the error text arrives on *stdout* as `{"error": …}`
for the `--json` verbs, which `Bd.run` does not read (rangerhq-aas). Treat any
new bd verb as guilty until probed: assert on state, not on exit status.

### The pairs are prunable, and pruning is the only fix on the pin (ranger-base-nusr)

`bd create --deps` is **not atomic**, and the fix is not a timeout. dinesh saw
`create --deps discovered-from:<poisoned parent>` exit 1 after 30.9s with
`i/o timeout` on the daemon socket, the issue committed and the edge missing.
That 30s is the client's socket read deadline and it is **incidental**: run the
same create in direct storage mode, where there is no socket at all, and it
does not fail — it never returns.

MEASURED 2026-08-27 against a `VACUUM INTO` snapshot of the fleet db,
`bd --no-daemon --db <snapshot> create … --deps discovered-from:<parent>`:

| parent | before the prune | after the prune |
|---|---|---|
| ranger-base-okbr | killed at 90s, issue committed, `dependencies` empty | 0.43s, edge present |
| ranger-base-o943 | (same shape, dinesh measured 30.9s via the daemon) | 0.45s, edge present |
| ranger-base-x6ic | — | 0.42s, edge present |
| ranger-base-cpyb | — | 0.40s, edge present |

So **raising bd's client timeout is the wrong fix** — it buys a longer hang and
a longer held write lock, never an edge. The write the caller asked for is one
statement behind a cycle check that diverges; nothing downstream of the
deadline can help. Ruled out; do not spend the pin on it.

**The decision: `relates-to` is not stored as an edge in this fleet.** The
relations carry no scheduling meaning — bd's ready queue gates on `blocks`
alone, and posse never reads the type — so the edge is pure provenance, and
provenance goes in a comment, which is what the standing rule already told
callers to do at the other end. `scripts/prune-bd-relates-to.sh` records each
link as a comment on **both** beads and then `bd dep unrelate`s the pair; it is
dry unless `--apply` is typed, refuses a symmetric pair of any type it cannot
unlink, and re-checks itself afterwards. Measured on the snapshot: 10 pairs,
20 rows, ~0.2s per pair, 20 comments written, and `blocks` (153),
`discovered-from` (566), `related` (3) and `blocked-by` (1) all untouched.

**Two corrections to the section above, both measured on the snapshot:**

- `bd dep add <x> <y> -t relates-to` writes **one** row and is harmless. Only
  `bd dep relate` (and its deprecated alias `bd relate`) writes both
  directions. The poison has exactly one source verb, not two.
- "The set only grows … it cannot be pruned back" is wrong, and it was the
  reason nobody tried. It cannot be pruned back only if someone keeps
  relating. Prune, then gate: `make verify-bd-no-relate-pairs`
  (`scripts/verify-bd-dep-safety.sh --gate`) exits 1 the moment any symmetric
  pair — of any type — is back. What *does* grow on its own is the *unsafe
  target* set: an ordinary bead landing upstream of a pair joins it with no new
  `relates-to` edge, which is why gating on the pairs and not on the count is
  the durable check.

Fixed alongside: both scripts used to die with a bare `unable to open database
file (14)` against a WAL-mode db whose `-shm` is gone — the state bd leaves
after a write with no daemon holding the store. sqlite refuses a `mode=ro` open
there outright. They now fall back to reading a copy, which keeps "never writes
the fleet db" true. A checker that errors instead of answering gets ignored.

**All three were gated on the operator (ranger-base-kbus, "I approve all").
Where they landed:**

1. **APPLIED 2026-08-27, by the operator, projection committed at `f9894bf`.**
   The runbook was `scripts/prune-bd-relates-to.sh` (read the plan) →
   `--apply` from `~/src/ranger-base` → `bd sync --flush-only` → commit
   `.beads/issues.jsonl`, without which an import can put the rows back.
   The classifier denies `--apply` from a persona session — it is a deletion
   on live state — so this is the operator's own shell, by design, and the
   next one will be too.
   **Verified against the live store after the fact:** 13 pairs / 28 unsafe
   targets → **0 / 0**; and on a fresh snapshot of the pruned db, `bd
   --no-daemon create … --deps discovered-from:` ran 0.39–0.46s with the edge
   present for okbr, o943, cpyb and x6ic — the four that hung. monica's
   independent check on the live store: `bd dep add ranger-base-x584
   ranger-base-x6ic -t discovered-from`, a 90s+ hang the night before,
   completed in 0.21s, and the five ADR 0019 provenance edges the outage lost
   were restored.
2. **APPLIED by the operator at `9d209bd`:** `Bash(bd dep relate:*)` and
   `Bash(bd relate:*)` are in `.claude/settings.json`'s deny list. That is the
   prevention half, and it is the one that matters — the gate only reports a
   pair *after* someone plants it, and one `bd dep relate` re-arms the whole
   failure. Personas cannot add those lines themselves (`Edit(.claude/**)` is
   denied, and editing the file through Bash instead would be defeating the
   deny rather than satisfying it), which is why it had to be the operator.
   **Read the deny list in `~/src/posse/.claude/settings.json`, not a
   worktree's copy** — a persona branch cut before `9d209bd` carries the old
   file and will tell you the lines are missing when they are not. The deny
   covers persona sessions in this repo; it does not cover the operator's own
   shell or another repo, so the detector below still earns its keep.
3. Upstream report approved as a courtesy; monica drafted it on kbus and it
   posts under the operator's account. Both halves are fixed past the SQLite
   line anyway (v0.63.3 checks cycles only for `blocks`), so it is a courtesy,
   not a route to a fix here.

### bd can delete a bead with no record, and does (rangerhq-fuom)

The store of record is `.beads/beads.db`; `.beads/issues.jsonl` is a
**projection** of it. bd's daemon runs both directions unattended — export
after every mutation, `git pull` then import on every file change — and the
import direction can DELETE rows. `bd import --no-git-history` documents the
behaviour it turns off: a *git history backfill for deletions* that reads the
JSONL's git log and removes from the database whatever a commit dropped.
Deletes cascade: `events`, `comments` and `dependencies` all carry `ON DELETE
CASCADE` on `issue_id`, so a bead's close comment, its work record and its
provenance go with the row.

The daemon's mutation log names `create`/`status`/`comment`/`update` and has
**no delete event at all**. So the whole thing is silent — no bd comment, no
log line, no sync commit. On 2026-08-21 rangerhq-b8i (closed, with committed
work at `33f9645`) and rangerhq-ja2 left the database that way; the check
below found a third, rangerhq-7p5, lost two days earlier and never noticed.

Three losses in the repo's whole history, and all three are the same shape:
each was one of a set of byte-identical verify beads filed milliseconds apart
by the duplicate-verify bug (rangerhq-th7l), and in every case the
last-created twin survived (b8i, ja2 → bzw; 7p5 → 2fe). *Which* of bd's paths
fired is not established — bd is a pinned third-party binary and logs nothing
here — but the selection is not random: it is duplicates that vanish. The
upstream trigger is fixed (verify-after files under the launcher lock now), so
new losses of this shape should stop.

posse never deletes a bead — no call site passes `delete` to bd, and
verify-after only reads and creates — so there is nothing here to guard. What
posse owes instead is that a loss cannot stay quiet (`internal/rhq/beadloss.go`):

- **The census.** Every id a committed `issues.jsonl` ever carried is
  git-durable, and the diff history is cheap to walk (~0.8s over 237 commits
  of a 900KB file). The walk takes removed `-{…}` lines newest-first, keeps
  the last removal per id, and drops every id `bd list --all` still resolves —
  bd is the authority, so a bead that left the file and came back is not lost.
  A repo with no git checkout, or a JSONL git has never seen, has no census:
  no findings, no error, nothing said.
- **The alarm.** A dispatch pass runs the census on every configured repo and
  prints one `bead-loss:` line per finding. Read-only, so it runs under
  `--dry-run` too and needs no lock, and it never gates the pass — a lost bead
  is already lost, and refusing to dispatch would not bring it back.
- **The ledger.** `.beads/deleted.jsonl` is the record a deletion owes:
  `{id, reason, by, at, commit, record}`, one line each, appended and never
  rewritten, tracked by git (nothing in `.beads/.gitignore` covers it) and
  never read by bd, so writing it cannot perturb the database it is
  protecting. `posse beads check --record "<reason>"` moves the current findings
  into it — which both silences the alarm for ids somebody owns and keeps the
  bead's last JSONL line, because once the row is gone from bd, git history
  and this file are all that is left of it.
- **A record owns one removal, not an id** (rangerhq-6he5). `commit` is the
  removal the record accounts for, and the ledger silences a finding only when
  the census names that same commit. Keyed by id alone the record exempted the
  id for the life of the repo: restore a bead with `bd import` and lose it
  again and the alarm never rang — which matters precisely because "recorded
  rather than restored" makes a later `bd import` the documented next step for
  the three ids already in there. It is the sha and not a clock because commit
  times are second-granular and `at` comes from whoever called the recorder.
  A record with no `commit` predates the field and still exempts its id; the
  three lines already written were backfilled from the census (e1902812 for
  b8i and ja2, 32d0ab1c for 7p5), so nothing live rides on that arm.

`posse beads check [--dir <repo>] [--record "<reason>"] [--as <who>]` exits
non-zero on findings, so an instance repo can run it in CI.

**b8i, ja2 and 7p5 are recorded, not restored.** Restoring means `bd import`,
which is in the persona deny block and is the operator's call; and of the
three only b8i has provenance worth recovering — the other two are duplicates
whose surviving twins are live work, so putting them back would re-create the
duplicate dispatch th7l just fixed. The ledger line carries b8i's whole
record, so `git log --grep rangerhq-b8i` still lands somewhere that explains
what the commit did.

### the queue's own repo, and who commits its jsonl (ADR 0015 §4)

The store of record moves out of the constitution repo into
`~/src/ranger-queue`, reached from every repo the crew touches — the
constitution repo now included — by the same `.beads/redirect` mechanism
D3-C already uses. The runbook is `docs/runbooks/queue-cutover.md` and the
script it drives is `scripts/queue-cutover.sh`; both were rehearsed on a
full copy of the live store (ranger-base-tjfw). What is worth knowing here
rather than there:

- **The launcher commits the projection.** While the store lived in the
  constitution repo, every commit anyone made carried
  `.beads/issues.jsonl` along — bd's own pre-commit hook stages it. A repo
  nobody commits in for any other reason has no such free ride, and `bd
  sync` exports without committing while `bd sync --full` commits *and*
  pushes (measured, 0.49.1). So a dispatch pass that judges a close flushes
  and commits it, path-limited, in the repo config `queue_repo:` names —
  `internal/rhq/queuejsonl.go`. Absent key = no commits, which is what
  every instance that never moves its store keeps doing.
- **Never a push, structurally.** Nothing on this path runs `git push`, and
  the cutover leaves the queue repo with no remote at all, so a future bd
  flag has nowhere to send it either.
- **`bd daemon start --auto-commit` exists** (with a separate `--auto-push`)
  and would do the committing with no posse code. Measured and not used: it
  commits on a 5s timer with no bead to name, and its git failures — a
  visibility-gate refusal included — land in `daemon.log`.
- **A fresh queue repo would disarm the bead-loss census.** `LostBeads` IS
  the git log of `issues.jsonl` in whatever repo the redirect lands in, so
  the cutover replays the `.beads/` history rather than starting clean.
  Because the replay renames every commit, it also rewrites
  `deleted.jsonl`'s recorded shas — the ledger silences a finding only when
  it names the same commit (rangerhq-6he5), and without the rewrite every
  owned deletion alarms again. Both measured in the rehearsal, both silent
  in production if missed.
- **bd stamps the database with a repo id**, and the queue repo is a
  different repo. Until `bd migrate --update-repo-id` runs, bd refuses to
  start its daemon and drops `.beads/daemon-error` warning that the
  git-history backfill "may treat your local issues as deleted" — the
  rangerhq-fuom mechanism, armed. It fails closed, which is why the check is
  "is `daemon-error` gone", not "did anything break".

### the constitution's promote, and what it will never carry (ADR 0015 §2/§3/§7)

`~/.config/posse` stops being a symlink onto the instance repo and becomes a
**copy** of it, taken from a commit: `posse promote` (`internal/rhq/promote.go`,
ranger-base-o943). The runbook is `docs/runbooks/home-cutover.md`. What is
worth knowing here rather than there:

- **The promoted set is four paths** — `agents/`, `config.yaml`, `recipes/`,
  `skills/` — and the exclusions are a symbol, not a sentence: `PromotedPaths`
  and `NotPromoted` in promote.go, with a test that reads both. `envs/`,
  `state/` and `personas/` are never created, copied or touched by promote,
  each for its own reason (§7 secrets with no commit behind them, machine-local
  state, persona memory).
- **The manifest is the trust anchor** ranger-base-5na lacked, and it sits at
  `home/promoted.json` — **beside** the promoted copy, deliberately not under
  `state/`, which every session can write. It records source, repo, commit and
  a sha256 per file.
- **Every launch re-hashes the promoted set against it.** A dispatched launch
  **refuses** on a mismatch (nobody is watching it, and the fix is one operator
  command); an interactive launch warns DEGRADED and comes up (refusing to open
  the session the operator would fix it from is the failure, not the control).
  The hook is at the top of `planLaunch`, so it is one place and not nine.
  **No manifest = nothing was promoted here = nothing to check**, which is what
  keeps every pre-0015 home and every test home launching.
- **Measured, since ADR 0015 marked it ASSUMED and it sits on the refusal
  path**: 7ms for 121 files / ~1.2MB, worst of 20 runs — several times the live
  constitution's size (`TestVerifyPromotedCostIsNegligible`, which fails above
  250ms so the assumption cannot quietly stop holding).
- **An env file can never trip it**, and that is a design decision rather than
  an oversight: `WriteEnvSet` is a live TUI path, so a verify covering `envs/`
  would refuse dispatch after any routine credential edit, until a re-promote
  that cannot even see the values. A verify that fires on correct behaviour is
  a verify everyone learns to ignore.
- **The clean gate is narrower than §3's wording, on purpose.** §3 says "the
  constitution repo's working tree is clean"; as built, a dirty *promoted path*
  refuses and anything else dirty is reported and allowed. The reason is ADR
  0015's own two carve-outs: `.beads` (§4) and `personas/` (§5) are dirty in
  that repo essentially always — both were, the hour this was built — and
  neither is prose a promote puts in force. A gate nobody can pass is a gate
  that gets bypassed. The amendment is filed; `promoteCleanGate` carries the
  argument.
- **A promote also deletes.** A PID the constitution no longer has leaves the
  home, or a retired persona stays in force forever. Bounded to the promoted
  set — it can no more reach `envs/` than the copy can — and every removal is
  printed.
- **The fence is spelled twice and neither spelling is the wall**: every
  shipped PID denies `Bash(posse promote:*)`, and promote itself refuses under
  `RHQ_PERSONA`. Both verified live (ranger-base-o943): the marker refusal
  against the real constitution repo, and the rendered shim refusing `posse
  promote`, `posse promote <dir>` and `posse promote --dry-run` while `posse
  version` passes through, with the refusal in `refusals.log`. What *notices*
  a constitution that changed anyway is the manifest.
- **`globalValueOpts` gained a `posse` entry with NO options**, and that empty
  entry is load-bearing: it declares that posse parses no global flag before
  its subcommand (**MEASURED**, `main()` reads argv[1] directly), which is what
  lets parity call the rule realized. Without it every PID carrying the deny
  launched DEGRADED on every runtime × cage — measured on the live gates
  report — and a fence that costs `--allow-degraded` on every launch is a
  fence that gets turned off.
- **The `rhq` alias is outside the shim.** The gate shims are keyed on the
  command NAME and no PID denies `Bash(rhq promote:*)`, so while an `rhq` on
  `$PATH` reaches this binary, `rhq promote` walks past L1. It is politeness
  that leaks, not the wall: the manifest still notices. Closed at the source on
  2026-08-27 (ranger-base-igup) — `make install` and `make link-plugin` no
  longer write the alias, so no fresh install has a second spelling. The two
  inodes already on the operator's box are his to remove inside
  ranger-base-3rv9's window (ranger-base-6y83); until then the leak is live
  there and nowhere else.

## grok substrate: pinned at 1.0.5, upgrades are a security re-audit (rangerhq-y7jr)

The fleet's grok shipped configured to replace itself. `~/.grok/config.toml`
had `[cli] auto_update = true` with `installer = "internal"`, and
`which grok` resolves to `~/.grok/bin/grok` → a symlink into
`~/.grok/downloads/` — i.e. **the managed install**, which is exactly the one
the updater is allowed to touch (`stdio auto-update skipped: not the managed
install` is the negative case, and it does not apply here). Three binaries are already
stacked in `downloads/` — the June install, 1.0.0 (Aug 8) and 1.0.5 (Aug 18) —
so this has happened before, unattended.

**The vector is the leader, not the launch.** `grok` runs a long-lived leader
process shared by every session (`cli.use_leader`), and the leader updates
itself *mid-life*: `xai-grok-update/src/auto_update.rs` carries
"Leader auto-update: v<x> installed successfully" and "Leader auto-update:
newer binary already on disk, relaunching without download". So "we only
upgrade when someone runs `grok update`" was never true — a fleet that never
restarts anything can still be on a new binary by lunchtime.

**Why that is a security problem and not a version-hygiene problem.** Nearly
everything the fleet knows about grok is version-VERIFIED, not contractual:

- **Coding-data consent** (rangerhq-sz7u). The defense there is that in 1.0.5
  the consent-record RPC `x.ai/consent/record` has **no server handler** —
  the binary says so ("consent record not sent; no server handler yet";
  "failed to persist consent answer; the notice will re-arm next launch") —
  so even an accidental `[Opt in]` on the startup splash cannot persist. That
  defense evaporates the day xAI ships the handler, and the splash is the
  same screen that eats a dispatched Enter (rangerhq-37c). This is the
  operator's data line.
- **Permission-mode precedence** (rangerhq-vjl). CLI
  `--permission-mode` beats `[ui] permission_mode` in config.toml
  (runtime.go:361-369). That precedence is the only thing keeping fleet grok
  launches off this machine's always-approve config fallback.
- The rule dialect, the `--rules="$(cat …)"` `=` form, and the login-shell
  capture L1 rides on — all of "Grok specifics" above.

An unreviewed roll-forward retires all of it silently, which is the actual
loss: not a broken binary, an un-rechecked one.

**The pin, applied 2026-08-22.** Declared in `etc/grok/version-pin.toml`,
live in `~/.grok/config.toml`:

```toml
[cli]
auto_update = false           # kills the update check and the leader's self-update
maximum_version = "1.0.5"     # soft ceiling: the updater never installs above this
```

`maximum_version` is the belt: a soft bound caps the updater's *target* even if
`auto_update` gets flipped back on by `/settings`, a managed-config push, or a
hand edit, and **soft bounds never block startup**, so it cannot strand a pane.
The declaration is not installed over config.toml — that file also holds the
operator's own preferences and grok rewrites it itself (`dismissed_version`,
`marketplace.*`). `make verify-grok-pin` asserts the live config still matches.

- **Verified by execution, not by reading the config**: `grok update --check
  --json` answers `{"currentVersion":"1.0.5","latestVersion":"1.0.5",
  "updateAvailable":false,"autoUpdate":false,…}`. That `autoUpdate` field is
  grok's own answer about what the updater will do, and it is what the verify
  script checks — the config file is only the second opinion.
- **`grok inspect --json` cannot see any of this.** Its keys stop at
  `grokVersion`/`channel`/`permissions`/…; there is no update or version-policy
  section. `update --check --json` is the only cheap probe.
- **A non-TTY launch skips the update check on its own** ("Non-TTY stderr
  (auto-detected)"), which is why you cannot demonstrate the pin by watching
  `version.json`'s `checked_at` from a script — it would not have moved either
  way. Don't use that as evidence.

**The harder gate, and why it is not set.** `required_maximum_version` is a
*hard* bound: grok exits at startup above it. Probed live —

```
$ GROK_REQUIRED_MAXIMUM_VERSION=1.0.4 grok -p "x" -m no-such-model-xyz
This version of Grok (1.0.5) is newer than the maximum allowed by your organization (1.0.4).
Install an approved version through your organization's approved method …
```

— and, importantly, **only the agent path is gated**: `--version`, `update`,
`inspect`, `doctor` and `models` all run normally above the ceiling, so an
out-of-range install stays diagnosable and recoverable
(`grok update --version 1.0.5` is allowed above a ceiling by design). That
makes it a clean gate: an unreviewed upgrade becomes a loud fleet-wide refusal
instead of a silent surface change. It is deliberately **not** set, because the
blast radius — every dispatched grok pane and the operator's own grok stop
starting — is the operator's to accept, not a persona's: rangerhq-iy3y.

Bounds resolve across layers by tightening only (floor takes the highest,
ceiling the lowest), the `GROK_*_VERSION` env overrides can only tighten
further, and an invalid value is ignored rather than blocking startup
("ignoring invalid version bound", `config/resolve/version.rs`). Do **not** set
`minimum_version`/`required_minimum_version`: those are anti-downgrade floors
and would block rolling back to a known-good build.

**`--no-auto-update` exists but is hidden.** It is in grok's own docs
(`14-headless-mode.md`) and absent from `grok --help` in 1.0.5; it is
nonetheless accepted (`grok --no-auto-update --version` → 0, while a genuinely
unknown flag errors). It was not added to `GrokFleetFlags`: it is per-session,
so it would leave the *interactive* operator sessions and the shared leader
unpinned — the config pin covers every entry point, including the leader.
`GROK_DISABLE_AUTOUPDATER=1` is the per-process equivalent.

**Operator runbook — lifting the pin (a deliberate upgrade):**
```
make verify-grok-pin                       # prints the re-audit list when upstream has moved
cp -a ~/.grok/config.toml ~/.grok/config.toml.before   # grok rewrites this file itself
cp -a ~/.grok/downloads/grok-1.0.5-macos-aarch64 /tmp/ # THE rollback binary
# --- the security persona re-audits the new build FIRST: consent-record
#     handler (sz7u), --permission-mode precedence (vjl), rule dialect,
#     splash detection ---
grok update --check --json                 # confirm the target
grok update --version <new>                # explicit; allowed above the ceiling
# then raise BOTH in etc/grok/version-pin.toml and ~/.grok/config.toml:
#   posse_pinned_version / maximum_version
make verify-grok-pin && make verify-detection
```
Rollback is `ln -sfn ../downloads/grok-1.0.5-macos-aarch64 ~/.grok/bin/grok`
(and `bin/agent`) — unlike `herdr update`, grok keeps the old binaries in
`downloads/`, so there is a rollback target without a re-fetch. Keep it that
way: do not prune `downloads/`. `grok du` reports it as the largest thing in the
grok home and is pure temptation — it only reports, and `grok worktree gc` does
not touch it, so nothing prunes those binaries but a person.

## Instance interstitials: the keys posse names, and the one it writes (ADR 0013 §2)

Every CLI draws something on its first run in a fresh pane — a consent
banner, an update menu, a splash. herdr recognizes none of those screens,
so a *dispatched* session lands on one, reports `idle` /
`default_known_agent_idle_fallback`, never becomes promptable, and burns
its startup wait for nothing. Measured on both non-claude runtimes in one
evening (`ranger-base-3j8`), and the layer that fixes it is not the clever
one.

**Three layers, cheapest first (ADR 0013 §2).**

1. **Sidestep.** Deliver the work prompt on the launch line (`prompt:
   argv`), so the screen is not the delivery channel at all. Landed for
   grok and codex (`ranger-base-cl7`/`ranger-base-dg5`); see *Argv
   delivery* above. This layer does not make the banner go away — it
   makes it stop being in the way.
2. **Instance config.** Operator-owned facts posse **documents and never
   writes**. This section.
3. **Declared keystrokes.** Last resort, keyed on a herdr *rule id*
   (today: `startup_splash` → Esc only). Keys are pressed once, and
   **Enter is not in the table**.

**Why layer 2 is a document and not a `os.WriteFile`.** Two of the three
answers are not a harness's to give:

- grok's `[Opt in]` lets xAI retain prompts and traces from sessions
  working in the operator's *private* repos. That is a visibility line
  (crew guardrail 4), and the privacy-preserving answer — `[Opt out]` — is
  a click, not a config posse can write on the operator's behalf.
- codex's update menu has `1. Update now` **default-selected**, and it runs
  `brew upgrade --cask codex`. Enter on arrival is an unreviewed
  roll-forward of a pinned tool, through a Homebrew this box has had broken
  (`rangerhq-y5on`), and the pinning precedent is `rangerhq-y7jr`.

Hence the rule, and it is the whole of layer 2: **a first-run dialog whose
default action mutates the machine is a launch refuse until that config
silences it, and nothing blind-sends Enter.** The coordinator's
string-match Escape watchdog is a stopgap, not the architecture.

**The keys, per runtime.** `posse runtime check <name>` prints each with
its file and whether it is silenced on this box — read-only probes, which
is the only thing posse does to these files.

| runtime | screen | key | who silences it, and how |
|---|---|---|---|
| grok | `Help improve Grok  [Opt out] [Opt in]` consent banner above the composer | `[privacy] privacy_banner_acked` in `~/.grok/config.toml` | the **operator**, clicking `[Opt out]` once in their own grok session. The value is an RFC3339 stamp, not a bool, and it records only *that* the banner was answered — never which way. In 1.0.5 the consent RPC has no server handler, so even an accidental `[Opt in]` cannot persist; that defense is version-verified and evaporates the day xAI ships the handler (`rangerhq-sz7u`). |
| grok | New worktree / Resume session / Quit startup menu, plus the changelog line | `[cli] auto_update = false`, `maximum_version` in `~/.grok/config.toml` | **already applied** — the fleet pin, declared in `etc/grok/version-pin.toml`, kills the update check *and* the shared leader's mid-life self-update. `make verify-grok-pin`; runbook in *grok substrate* above. |
| codex | `Update available! → 1. Update now  2. Skip  3. Skip until next version` | `dismissed_version` in `~/.codex/version.json` | the **operator**, picking `3. Skip until next version`: Down twice, *verify the caret moved by re-peeking*, then Enter. Two steps with a verification between them, because getting it wrong upgrades their tooling. |
| claude | `Quick safety check: Is this a project you created or one you trust?` — full screen, `1. Yes, I trust this folder / 2. No, exit`, footed `Enter to confirm · Esc to cancel` | `projects["<session dir>"].hasTrustDialogAccepted` in `~/.claude.json` (or `$CLAUDE_CONFIG_DIR/.claude.json`, or the config dir's `.config.json` where that exists) | **the LAUNCH**, per session directory — the one exception, below. |

**The codex dismissal has a shelf life.** It silences one release: the menu
returns the moment `latest_version` moves past `dismissed_version`. That is
why `runtime check` prints both numbers rather than a bare yes — a probe
whose answer expires should say when.

**The one key the launch writes, and why it is not the same kind of key
(rangerhq-w4uf).** Claude's directory-trust dialog is not a first-*run*
dialog at all — it is a first-run-*here* dialog. It fires per session
directory, so the fleet's long-lived checkouts never show it and every new
repo, worktree, container HOME and scratch dir draws it again. There is
nothing an operator can answer once: MEASURED on claude 2.1.241, `claude
--help` names no flag for it, `claude project` manages only `purge`,
`--settings` takes no such key, and the single env that skips the check
(`CLAUDE_CODE_SANDBOXED`) claims a confinement a shims-tier launch does not
have. The CLI itself names the supported non-interactive path, in the error
it prints when it drops a project's hooks for want of trust: *"Run Claude
Code interactively here once and accept the trust dialog, or set
projects[<dir>].hasTrustDialogAccepted: true in ~/.claude.json"*.

So the launch seeds it (`SeedClaudeTrust`, `internal/rhq/trust.go`), which
is the same grant posse already types on codex's line (`-c
"projects={\"$PWD\"={trust_level=\"trusted\"}}"`, ADR 0002 §4) and the
same one `SeedCageHome` already writes into a container HOME. What it does
NOT do is bend the layer-2 rule: it is scoped to the one directory posse
launched in, merged into the operator's file and never rewritten from a
template, skipped entirely when the directory is already trusted, and it
refuses the launch rather than replacing a config it cannot parse. Nothing
here presses a key: the seed is written before the line is typed.

Measured on the pane, four herdr scratch panes, no API turn:

- fresh dir, real config → the modal, and `herdr agent explain` reports
  **`blocked`** (`live_blocked_form`), not idle. So dispatch does not type
  a work prompt into the menu the way stock detection let it type into
  codex's *Hooks need review* — it waits out its patience and the session
  never becomes promptable. (Consistent with *Dispatch primitives* above:
  `agent wait` can settle `idle` a beat before the dialog is drawn, which
  is why explain wins.)
- same config dir, `hasTrustDialogAccepted: true` seeded → straight to a
  live composer, `herdr` `idle`, `live_prompt_box`.
- the key alone leaves claude's "Welcome back!" project panel drawn above
  the composer — harmless (pane read `idle`, composer live), silenced
  anyway with `hasCompletedProjectOnboarding`, because "harmless splash" is
  what grok's startup menu was called too.

**What the grant hands the session dir, and the keyed launch check.**
REMEASURED 2026-08-27 on claude 2.1.247 and re-run unchanged on
2.1.241/.245/.246 (ranger-base-i0s8), because the first telling of this
paragraph was wrong in its detail and the current claude docs say the
opposite shape. What the shipped binary does:

- **Directory trust gates two keys, and they are permission keys.**
  `permissions.allow` and `permissions.additionalDirectories` out of
  `.claude/settings.json` are dropped while the workspace is untrusted,
  and claude says so on stderr ("Ignoring 1 permissions.allow entry from
  .claude/settings.json: this workspace has not been trusted…"). The debug
  line quoted here before — "Dropped N project-scoped … entries — workspace
  not yet trusted" — is real, but its template is dynamic and those two
  permission keys are its only call sites in the bundle. It never said
  `hooks`. That misquote, not a version drift, is the whole disagreement:
  2.1.241 measured today behaves exactly like 2.1.247.
- **`hooks` are gated one layer down, at execution, and only where the
  dialog is a live screen.** The binary's own line is "Skipping <event>
  hook execution — workspace trust not accepted". Interactive and
  untrusted, the session parks on the trust dialog and a project
  SessionStart hook never runs; interactive and trusted — what the seed
  buys — it runs. Headless does not hold that line: `claude -p` in an
  untrusted directory runs the project's hooks, in the same run that drops
  that file's `permissions.allow`, and writes no trust entry. So the seed
  is the enabling act for repo hooks in a posse LAUNCH, which is
  interactive; it is not what gates hooks in general.
- **`mcpServers` under `.claude/settings.json` is inert** on these builds:
  never listed by `claude mcp list`, never named in a trusted session's
  debug log, never spawned. The live project-MCP channel is `.mcp.json`,
  and it sits behind its own "⏸ Pending approval" gate that reads
  identically trusted and untrusted.

None of that moves the launch check, and it makes it *more* load-bearing:
the check fires on settings content, so it does not depend on which of
claude's gates is holding, or on the docs and the binary agreeing next
release. Claude declares that settings path with
`ProjectConfigKeys: [hooks, mcpServers]`: presence of either key degrades
before trust is seeded, while this repo's permission-only file stays clean.
Naming `mcpServers` is deliberately conservative — a key claude ignores
today is a key it may honor tomorrow. An existing file that cannot be
decoded as a top-level JSON object also degrades; failing open there would
turn a classification error into a claim that no executable channel exists.

**Trust keys on the repo root, and worktrees inherit it.** Measured in the
same pass: with only a repo root carrying `hasTrustDialogAccepted`, a
subdirectory of it and a `git worktree add` linked worktree of it both
opened on a live composer, an untrusted sibling repo drew the dialog, and
claude wrote no new `projects` entry for either. The fleet's per-worktree
seed entries are belt, not load-bearing.

**What posse does NOT do, so nobody re-proposes it:** pre-answer the other
runtimes' dialogs from the harness (write `privacy_banner_acked`, write
`version.json`). Rejected in ADR 0013 — coding-data consent is a visibility
line and update-skip is the operator's pin. Posse documents those keys, and
writes no answer the operator could have given once.

## herdr substrate: upgrading with `herdr update --handoff` (rangerhq-0ao)

The fleet runs herdr **0.8.0** from `~/.local/bin/herdr` (a direct install, not
Homebrew — `brew list herdr` is empty, and self-update refuses to touch
Homebrew/mise/Nix-managed installs, pointing at the package manager instead).
`herdr update` reads `https://herdr.dev/latest.json` for the configured
channel (`herdr channel show` → `stable`; `preview.json` is the other),
downloads the pinned per-platform asset from the herdr GitHub release,
verifies the manifest's SHA-256, and **renames the new binary over the running
one** — `install_downloaded_update` is a `fs::rename`, so *there is no backup
of the old binary*. Target at the time of writing: **0.8.2** (protocol 19 →
20; 0.8.1 was never released on stable), published 2026-08-19.

**What `--handoff` actually does.** Without it, the new binary is installed and
nothing else happens: the running 0.8.0 server keeps every pane until it is
stopped, and stopping it exits every pane process. With it, the old server
spawns the *new* binary as `herdr server --handoff-import <socket> <token>`,
passes the live PTY master fds over a unix socket with `SCM_RIGHTS`, hands
across the session snapshot, waits for the import server to report
`ready`/`restored`, commits, and exits. The pane processes are never
re-parented to a new shell — they keep running, same pids.

- **Survives**: every pane process (agents, shells, the dispatch loop), pane
  and workspace ids, workspace labels, layout, cwd, agent-session refs, the
  api socket path (the new server rebinds `~/.config/herdr/herdr.sock` after
  waiting up to 5s for the old sockets to close), and everything in
  `~/.config/herdr` and `~/.local/state/herdr` (config, plugin registry,
  local detection overrides, cached remote manifests).
- **Does not survive**: attached clients — the terminal you are looking at is
  a *0.8.0 client process*, and after the old server exits it cannot reattach
  to a protocol-20 server ("this client needs protocol 19, but this server
  speaks protocol 20"). Quit it and run `herdr` again; the binary on disk is
  now 0.8.2 and every workspace is still there. Scrollback replay is capped at
  `MAX_REPLAY_BYTES_PER_PANE = 8 KiB` per pane, so long pane histories come
  back truncated — the processes are fine, the transcript above the fold is
  not.
- **Hard cap**: `MAX_FDS_PER_HANDOFF = 64` panes ("live handoff supports at
  most 64 panes in one update; close panes or restart herdr normally"). The
  fleet moved between 10 and 19 panes while this was written — headroom, but
  it is a fleet-size limit, not a constant to forget, and the count to check is
  the live one.
- **`herdr update` refuses to run inside herdr** (`HERDR_ENV=1` in every pane):
  "run `herdr update` outside herdr after detaching from the session". It must
  be run from a terminal that is not a herdr pane.

**Pre-flight** (all of it before the command — with `--handoff` and a server
that advertises `live_handoff`, there is *no* confirmation prompt on the happy
path; it downloads, installs and hands off in one go):
```
herdr status --json                    # server running, protocol 19, capabilities.live_handoff: true
posse list > /tmp/posse-list.before    # the baseline every post-check compares against
cp -a ~/.config/rhq/state/herdr /tmp/herdr-metas.before   # session metas (workspace/pane ids)
grep -H '^workspace:\|^socket:' ~/.config/rhq/state/herdr/*.yaml | sort > /tmp/herdr-metakeys.before
                                       # the two fields a handoff PRESERVES. THIS, not a
                                       # byte-for-byte diff of the directory, is the post-flight
                                       # check: `gen:` is expected to change (window section).
                                       # `| sort` is load-bearing — ugrep (the `grep` on this box)
                                       # walks a glob in parallel, so unsorted output reorders
                                       # between runs and the post-flight diff falsely fails.
cp -a ~/.local/bin/herdr /tmp/herdr-0.8.0                 # THE rollback binary — update keeps no backup
herdr api snapshot | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["result"]["snapshot"]["panes"]))'   # must be < 64
posse version                          # note the sha; two ancestor checks below depend on it
git -C <posse checkout> merge-base --is-ancestor 04e9256 <fleet sha>   # exit 0: ct9 fix (pidfile) — see below
git -C <posse checkout> merge-base --is-ancestor 41fa735 <fleet sha>   # exit 0: identity fence (gen:) — see below
posse dispatch --watch-status          # `watch-loop: running (pid N, since T)` — N is what the
                                       # post-flight compares. `none` means no loop is running,
                                       # from the lock and not from any file (rangerhq-gir5)
```
Fleet drained first (the coordinator): no dispatch pass mid-flight. The window between
`installed` and `live handoff complete` is the dangerous one — during it the
binary on disk is 0.8.2 while the server is still 0.8.0, so any `posse` call
that shells out to `herdr` gets a protocol-20 client talking to a protocol-19
server and fails.

**The armed dispatch loop is the one pane that must not be "drained".**
`autostart_interval:` is armed (5m, and since 2026-08-22 `autostart_dry_run:
false` — see the window section below, which supersedes what this bullet used
to say), so the cockpit plugin's `[[startup]]` hook fires on the *import*
server too and decides that loop's fate (rangerhq-ct9). Two conditions before
pressing enter:

- **`posse` must be new enough to stamp the pidfile** — `04e9256`, the ct9 fix.
  The fleet ran `0.3.0+757e13d` when this was written, 71 commits *before* it:
  a loop that binary starts leaves no record, the fixed hook reads it as dead
  and replaces it, which is the ct9 symptom on the substrate. Promote first
  (rangerhq-cpo7) or accept that the loop is killed and restarted.
- **Leave the loop running, but park it first.** Draining means no
  *dispatch*, not no loop — a live loop is the only way to observe the check
  below, so killing it costs the check. But "no dispatch" is no longer free:
  with `autostart_dry_run: false` the loop dispatches for real and the window
  hands live work back. Park it with `plan_guard_5h`/`plan_guard_7d`, wait for
  the in-flight prompts to drain, then hand off — the next section is the whole
  procedure. Note its pid first; an empty pidfile means no loop is running and
  the hook will simply start one on the far side.

### The install→handoff window is a LIVE dispatch window (rangerhq-7t4)

`autostart_dry_run` went to `false` on 2026-08-22, which removed the premise
the paragraph above used to rest on. Under dry-run a pass returned before
`fire()`: no claim, no prompt, nothing in flight, so "a pass in the window
fails and is reported" was the whole story. Armed, it is not.

**The config flip is inert against the loop that is already running.**
`--dry-run` is argv (`cmd/posse/main.go`, set once), and `plugin/autostart.sh`
builds that argv from the config *at hook time*. The live loop's argv is what
`state/dispatch-watch.pid` records (`cmd:` — still the place to read it; the
file lost its liveness job in rangerhq-gir5, not its identity one). Editing `autostart_dry_run: true` back
changes nothing until the loop restarts — and restarting it is exactly what
flips the handoff's hook branch from "left alone" to "replacing", i.e. costs
the ct9 check. Anyone who "restores dry-run for the window" by editing the
config has changed nothing and believes they are safe.

**`plan_guard_5h:` IS live, and is the parking brake.** `planGuard()` runs at
the top of every pass, before bd, before herdr, before the launcher lock, and
it reads the config off disk every time (`YamlGet` → `os.ReadFile`, no cache).
Verified on a running `--watch` loop: pass 1 under `plan_guard_5h: 99`, the
file edited to `1` mid-flight, pass 2 of the *same process* printed
`plan 5h at 46% > 1%, pass skipped`. Same pid, no restart, and restoring the
threshold resumes the very next pass.

```
# park (before the handoff) — set BOTH: planPercent treats <= 0 as "unset,
# guard off", so 1 is the floor, and the comparison is strictly `>`; two
# windows cover the case where one reads exactly 0.
plan_guard_5h: 1
plan_guard_7d: 1
# ... hand off ...
plan_guard_5h: 70      # restore after the post-flight
plan_guard_7d: 85
```

**Parking stops new passes; it does not cancel prompts already fired.** A
fired bead holds a live `herdr agent prompt --wait` client process for up to
`WaitCeiling` — **4 hours**, re-waiting in 15-minute legs. When the old server
exits at the handoff commit, that client's call fails, and the failure is
typed: a call to a server that is not there returns
`{"error":{"code":"server_not_running",...}}` on stderr with exit 1 (measured
against a scratch server). `gather()` branches on one code only —
`IsHerdrCode(err, "timeout")` — so anything else takes
`unclaimAfterPromptFailure` -> `Bd.Unclaim(..., keepAssignee: resumed)`.
**The handoff therefore hands back every in-flight bead while its agent keeps
running**, since the panes survive with the same pids. *How* it hands them back
depends on how the pass acquired them, and the common case is not the loud one:

- **freshly claimed this pass** — `bd update <id> --status open --assignee ""`.
  Open and unassigned, which is what it looks like.
- **resumed** (`already claimed by <persona> — resuming` in the log) — `Unclaim`
  omits `--assignee ""` (beads.go, rangerhq-kux: the routing was somebody else's
  decision, usually the operator's). Status goes to open, **assignee intact**.
  This is the usual case for a parked fleet: every one of the three prompts in
  flight at 12:49 on 2026-08-22 logged "resuming". Do not grep for unassigned
  beads to find the damage — you will find none and conclude nothing happened.

Either way the hazard is the same and undiminished: open means ready, so five
minutes later the bead is ready again with only `personaActive` — a
detection-dependent guard — between it and a second agent on the same bead. That is the rangerhq-khc failure mode
arriving through the one door the khc fix does not cover.

So the safe moment is a **gap**, not a config value: park, then watch
`state/dispatch-watch.log` until the pass reports no `prompt(s) in flight`.
While a pass is gathering no new pass starts (`Watch` blocks in `Run`), so the
"5m interval" is a floor, not a cadence — the gap opens when the current beads
settle, which for 15–40 minute beads can be a while.

**A pass that merely *starts* inside the window is survivable**, and that is
worth separating from the above. Every herdr call fails; `launchSession` calls
`awaitAgent` *before* `Bd.Claim`, so nothing is claimed, and `startPlanned`
calls `CreateWorkspace` *before* `writeMeta`, so nothing is half-written. The
pass prints `✗` lines, benches the slot, and `Watch` continues. But note what
is doing the protecting: `mustNotOrphan` reading silence as "refuse", not the
guards — `crewHeld`, `personaActive` and the in_progress-held check all
`return ""` on a herdr error, i.e. they fail **open**.

**Widening the interval is the wrong lever, twice.** `autostart_max_interval`
is `--max-interval`, argv, read by the hook at loop start: inert against a
running loop, same as `autostart_dry_run`. And `NextInterval` only backs off on
*quiet* passes, snapping straight back to base the moment anything dispatches —
a busy fleet never leaves 5m. It also does nothing about the in-flight prompts,
which are the actual hazard.

**One more thing `dry_run: false` changed: recycled workspace ids.** herdr
recomputes its id allocator as max(live)+1 at a handoff (rangerhq-6bg7), so the
ids it re-issues are exactly the ones stale metas hold. Under dry-run nothing is
created after the handoff, so no recycled id is ever handed out. Armed, the
first pass creates 1–3 workspaces from precisely that range, within five minutes.

**The fence for this is live — and that is a fact with an expiry date, so check
it rather than reading it here.** The check is the pre-flight's

```
git merge-base --is-ancestor 41fa735 <fleet sha>    # 0 = fenced, 1 = not
```

- **exit 0 — fenced.** `notOurWorkspace` (rangerhq-yt1p) refuses a workspace
  whose `label` is not the meta's name unless the meta's `gen:` is this
  server's, so a recycled id is not listed as ours. The post-flight's manual
  label-vs-meta compare is then **redundant** — spend that attention on the
  meta-key diff instead. As of 2026-08-22 08:29 the fleet binary is
  `0.3.0+baa881f`, which is fenced (exit 0; against the previous `66f9f44` it
  exits 1). rangerhq-gmyo — "promote posse at HEAD before the handoff" — is
  satisfied by that promotion, though HEAD has moved on since.
- **exit 1 — not fenced.** That binary's `Sessions()` has no label check at
  all: present in the listing *is* ours. Promote posse first, or the post-flight
  has to compare labels by hand.

**Being fenced costs you a post-flight check — the one you were relying on.**
`gen:` is `dev:inode` of the api socket (`ServerGen`, herdrback.go), the socket
file is recreated by a handoff (rangerhq-6bg7, measured), and `Sessions()` takes
`sock, gen := SocketID(), ServerGen()` and calls `backfillServer` for **every**
live meta on **every** listing — every `posse list`, every cockpit read, every
dispatch pass. `backfillServer` stamps when `gen != "" && m.Gen != gen &&
ws.Label == m.Name`, and posse creates every workspace with `--label <session
name>`, so that predicate holds for all of them. **The first post-handoff
listing rewrites the `gen:` line of every meta file, by design.** Not
hypothetical: on 2026-08-22 08:29 the promotion of a gen-carrying binary
rewrote two session metas — both created
hours earlier — the moment it first listed them. A handoff does the same to
all of them. So a byte-for-byte meta diff is **no longer the check** and will
falsely fail; the post-flight below compares `workspace:` + `socket:` instead.

**The command** (outside herdr, in a plain terminal):
```
herdr update --handoff
```
A successful run prints, in order:
```
checking stable channel for updates...
running herdr targets:
  <label>: server v0.8.0 (handoff supported)
  update: v0.8.2
downloading v0.8.2...
downloaded v0.8.2
installed v0.8.2
asking server <label> to hand off live panes to the updated server...
live handoff complete for server <label>; pane processes should still be running.
```
A **failed** handoff is not the disaster it sounds like: the old server keeps
running with its panes, and herdr says so and asks —
`live handoff failed, but server <label> is still running with your panes.` …
`stop the old server now? [y/N]`. **Answer `n`.** `y` exits every pane
process; `n` leaves the fleet alive on 0.8.0 with 0.8.2 on disk (a state to
leave *immediately*, since new `herdr`/`posse` calls now speak protocol 20 —
either retry the handoff with `herdr server live-handoff --import-exe
~/.local/bin/herdr` or roll the binary back). Budget ~4.5 min worst case: the
handoff request times out at 240s, the confirm poll at 30s.

**Post-flight**:
```
herdr status --json                                   # client+server 0.8.2, protocol 20, socket unchanged
posse list | diff /tmp/posse-list.before -            # same sessions, same state; NO pruned-meta warnings
grep -H '^workspace:\|^socket:' ~/.config/rhq/state/herdr/*.yaml | sort | diff /tmp/herdr-metakeys.before -
                                       # the meta check. `gen:` IS EXPECTED to differ on every
                                       # file after the first post-handoff listing (new server
                                       # generation = new socket inode) — a byte-for-byte
                                       # `diff -r /tmp/herdr-metas.before ...` falsely fails and
                                       # is NOT the check. `workspace:` + `socket:` unchanged is.
                                       # /tmp/herdr-metas.before is still the thing you restore
                                       # FROM if a repair goes wrong; it is not a comparand.
herdr agent explain <pane> ; make verify-detection    # override still active + fixtures still pass
herdr plugin list                                     # posse.cockpit still registered
posse dispatch --dry-run -n 1                         # routes without dispatching

# the dispatch loop (rangerhq-61u) — pid unchanged, hook stood down
posse dispatch --watch-status                                   # `running`, and the SAME pid as pre-flight.
                                                                # Held lock = the process itself survived the
                                                                # handoff; no `ps` and no argv match involved
herdr plugin log list --plugin posse.cockpit                 # the hook's own line, read it NOW
grep -c 'dispatch --watch armed' ~/.config/rhq/state/dispatch-watch.log   # unchanged: no second banner
posse peek dispatch                                             # one loop, passes continuing across the handoff

# the window's own damage (rangerhq-7t4)
herdr status --json | grep compatible    # server.compatible is the in-window detector: false while
                                         # the binary is 0.8.2 and the server still 0.8.0
grep -n 'server_not_running' ~/.config/rhq/state/dispatch-watch.log   # each hit is a bead handed back
                                         # mid-flight; re-claim it by hand (bd update <id> --status
                                         # in_progress --assignee <persona>) — its agent never stopped.
                                         # A RESUMED bead keeps its assignee, so status is the only
                                         # thing that moved: search by status, not by assignee.
herdr workspace list                     # ONLY IF the pre-flight's `merge-base 41fa735` exited 1
                                         # (unfenced binary): after the FIRST post-handoff pass,
                                         # compare each workspace's label against the meta filename
                                         # that names its id — an unfenced `Sessions()` lists a
                                         # recycled id as ours and a later pass prompts a stranger's
                                         # pane. Fenced (exit 0), `notOurWorkspace` does this for
                                         # you and the manual compare is redundant: skip it, and
                                         # watch `posse list` for the fence's own warning line instead.
```
`posse list` is the real test: metas key on `workspace:` + `socket:`, both of
which the handoff preserves, so a clean listing means nothing needs repair.
**On a fenced binary, "clean" has a second half**: the fence is stricter than
what it replaced, and a meta it cannot prove is ours is left *out* of the
listing with a warning rather than listed under the wrong workspace. So a
session going MISSING from `posse list | diff` is the fence talking, not a lost
session — the file is still there (the prune keeps it), and the repair is the
label-match rewrite below. A session listed but wrong is the failure the fence
exists to prevent; a session withheld is the fence working.
**If ids ever did move**, the repair is mechanical and must be done before any
`posse list` against the new server (the prune guard keeps the files, but a
listing is where a wrong id shows up): for each meta in
`$RHQ_HOME/state/herdr/*.yaml`, match its filename against the workspace
`label` in `herdr workspace list --json`, rewrite `workspace:` to that id, and
rewrite `pane:` to the `pane_id` that `herdr agent list --json` reports for
that workspace. Never delete a meta to "fix" it — that is the state the
session cannot be rebuilt from.

**The loop check, and what it is worth.** Rehearsed end to end, not reasoned:
a scratch session server (`herdr --session hoprobe server`) with scratch
`RHQ_HOME`, `RHQ_BIN` and `HERDR_SOCKET_PATH` *on the server process* — the
rangerhq-snd trap is the reverse of that — armed the loop through the real
`[[startup]]` hook, then took `herdr server live-handoff --import-exe` on
0.8.0 → 0.8.0 (rangerhq-61u). The loop came through with **the same pid**,
`posse peek` showed its next pass four seconds after the handoff completed, and
the hook on the *import* server logged, exit 0:
```
dispatch autostart: loop already running (pid 84662) — left alone
```
Negative control, same handoff with the loop killed and its pidfile left
stale: `dispatch restored by herdr without its loop — replacing`, a new pid, a
second banner. The two outcomes are distinguishable from the log alone, which
is the whole point of the check. Reading it:

- **pid unchanged + "left alone"** — the carry-over case. Nothing to do.
- **new pid + "replacing"** — the loop did not survive. That is still a
  correct hook decision (a dead loop must be replaced), but then "pane
  processes survive" does not hold for our loop and *this runbook* is what
  needs narrowing, not the hook.
- `herdr plugin log list` is **per-server and in-memory**: the handoff resets
  it, so the hook's run is `plugin-log-1` on the new server and is gone at the
  next restart. Read it right after the handoff, not tomorrow.

Not verified, and honest about it: the rehearsal was **0.8.0 → 0.8.0**. The
handoff path and `run_plugin_startup_hooks` are identical in both versions
(read from source above), but a 0.8.2 *import* server running the hook is
first observed at the real upgrade. A cross-version rehearsal was declined
rather than skipped — a 0.8.2 server shares `~/.config/herdr` and
`~/.local/state/herdr` with the fleet (there is no config-dir env override;
the binary knows only `HERDR_SOCKET_PATH`, `HERDR_CLIENT_SOCKET_PATH`,
`HERDR_SESSION`, `HERDR_BIN_PATH`, `HERDR_ENV`, `HERDR_WORKSPACE_ID`,
`HERDR_TAB_ID`, `HERDR_PANE_ID`), so it could migrate the shared config or the
plugin registry under the running fleet. That is the operator's line.

**`herdr server live-handoff` mechanics** (hidden subcommand, no `--help` in
`herdr server`'s command list):
- It takes **no target flag**. It goes wherever `herdr --session <name>` /
  `HERDR_SOCKET_PATH` resolves — aim it before firing, and confirm the aim by
  which *server log* the `starting live handoff` line lands in.
- `--expected-version <impossible>` is a **safe aim probe**: the import server
  refuses, the old server rolls back (`handoff import server reaped during
  rollback … SIGKILL`), and every pane keeps running. One rolled-back attempt
  buys certainty about which server you are pointed at.
- A failed handoff prints a JSON error envelope
  (`{"error":{"code":"handoff_failed","message":"handoff stream closed while
  reading line"}}`) and **exits 0** — assert on the server log and the panes,
  never on the exit status. The interactive `stop the old server now? [y/N]`
  prompt belongs to `herdr update --handoff`, not to this subcommand.

**Rollback** — the old binary is gone unless it was copied (above):
```
install -m 0755 /tmp/herdr-0.8.0 ~/.local/bin/herdr        # or re-fetch:
curl -fL -o /tmp/herdr-0.8.0 https://github.com/herdrdev/herdr/releases/download/v0.8.0/herdr-macos-aarch64
shasum -a 256 /tmp/herdr-0.8.0   # d53a9f93fccfdfcc55632927bf51002f5add0aa7990bcdf508ffbd84ac658178
herdr server live-handoff --import-exe ~/.local/bin/herdr   # hand 0.8.2's panes back to 0.8.0
```
Downgrading by handoff is the same mechanism in reverse and is not a path
herdr advertises; if it refuses, the honest rollback is stop-and-restart,
which costs every pane process.

**What 0.8.2 changes under posse specifically** (the rest of the changelog is
Windows, desktop UI, and copy-mode):
- **`agent prompt` now refuses an agent sitting at an approval or question
  dialog** with a new error code `agent_blocked`, instead of typing into the
  dialog. That is the `agent_prompt_stalled` failure closed off upstream.
  `Herdr.Run` surfaces it as a typed `HerdrAPIError`, and dispatch's
  "anything that is not `timeout` means the prompt never landed" rule already
  handles it correctly (claim released, bead unclaimed) — but the code is not
  in any comment or QA fixture yet (rangerhq-ejf).
- **`agent start` now waits for pane shells and first-run prompts** instead of
  reporting premature readiness. posse does not use `agent start` (it types into
  the pane and waits with `agent wait --until idle,done`), so this is upstream
  agreeing with `dispatch.go`'s "detection is not readiness".
- **Headless servers now size new panes 120×40 instead of 80×24** when no
  client is attached. Every pane dispatch creates from now on is wider; pane
  reads and any detection rule that depends on wrapping see different text.
- **Agent hooks now invoke the running herdr binary** instead of the first
  `herdr` on `PATH`.
- **Detection manifests are refreshed independently of the binary**, from
  `https://herdr.dev/agent-detection/`, into
  `~/.local/state/herdr/agent-detection/remote/` — so some of 0.8.2's
  detection fixes (the claude spinner-frame strip) are already live on 0.8.0.
  Upstream codex is still `2026.08.09.1`, our fork point, so the override
  stays and `make verify-detection` reports no drift.
- **The plugin `[[startup]]` hook fires on a live handoff too**
  (`run_handoff_import_server` calls `run_plugin_startup_hooks`, in both
  versions) — while the panes it thinks it is seeding are still alive. That
  made `plugin/autostart.sh`'s "a restored session is a husk" assumption
  false in exactly this case; it was armed on 2026-08-21 (`autostart_interval:
  5m`, dry-run), so it is no longer latent. Fixed in rangerhq-ct9: the hook
  now reads the loop's pidfile and replaces only what is provably not running
  — see "One loop, and the husk problem" above — and that fix is now rehearsed
  against a real live handoff (rangerhq-61u, the post-flight check above).

### Workspace ids recycle across a server process boundary (rangerhq-6bg7)

`w5V` is not a name, it is a **cursor**. herdr's workspace-id allocator is
`max(live workspace id) + 1`, recomputed from the live set every time a server
process starts — including a `--handoff` import. Nothing on disk carries a
high-water mark: `~/.config/herdr/session.json` (schema `version: 3`) persists
`id` per workspace plus `next_public_pane_number` / `next_public_tab_number`
*inside* each workspace, and no counter at the top level. Ids are base-36
after the `w` (`w5`=5, `w57`=187, `w5V`=211), monotonic **within one server
process** and only there.

Measured on a scratch session server (`herdr --session idprobe server`, its
own socket under `~/.config/herdr/sessions/idprobe/`, deleted after; the live
fleet server was never touched):

| step | result |
|---|---|
| fresh server, create ×4 | `w1 w2 w3 w4` — a new server starts at `w1` |
| close `w3` `w4`, create ×2 | `w5 w6` — **no reuse within a lifetime** |
| stop + start (live `w1 w2 w5 w6`), create ×2 | `w7 w8` — resumes past the live max |
| close `w7` `w8`, stop + start, create | **`w7` — recycled** |
| close everything, stop + start, create | **`w1` — counter fully reset** |
| close `w3`, `herdr server live-handoff --import-exe …`, create | **`w3` — recycled** |
| control: close `w3`, create in the *same* process | `w4` — still no reuse |

So the "**Survives**: pane and workspace ids" line above is exact and also
narrower than it reads: ids survive for workspaces that are *live* at the
handoff. Every id **above the live high-water is free real estate** on the far
side, and that set is precisely the set of ids stale metas hold — a meta whose
workspace died before the restart names an id the next server will hand to
somebody else. Worst case is the quiet one: bring the fleet down, restart
herdr, and the allocator restarts at `w1` and climbs back through the whole
range every old meta points into.

**What this costs ADR 0011 §2.** `WorkspaceAlive(id)` proves a workspace holds
that id, never that it is the one the meta recorded — the pid-recycling trap
with a different counter. And `workspace_not_found` is a **snapshot, not a
stable fact**: an id that answers not-found today can answer alive tomorrow,
for a stranger. The consequences are the ones §2 exists to prevent — the meta
is spared forever, `Sessions()` lists the wrong workspace under the old name,
and a prompting pass types into somebody else's pane.

**The socket guard does not catch this.** `cannotAnswerFor` compares the meta's
`socket:` against `SocketID()`, and the path is unchanged across both a restart
and a handoff (`~/.config/herdr/herdr.sock` either way) — same string, different
generation, different id space.

**What herdr will and will not tell you about identity.** `workspace get`
returns only `workspace_id`, `label`, `number`, `tab_count`, `pane_count`,
`active_tab_id`, `agent_status`, `focused`: **no creation time**, and
`api snapshot` carries no server pid, boot id or start time either — there is
no generation token in the API. Two anchors are real:

- **`label`** — posse already creates every workspace with `--label <session
  name>` (`herdr.go:231`) and the meta's own filename is that name, so the
  identity check needs no new field: a workspace whose `label` is not the
  meta's name is not the meta's workspace. (`terminal_id` and the root pane's
  `shell_pid` are *not* usable: both are regenerated when the pane's terminal
  is rebuilt, so they mark a legitimately restarted workspace as a stranger.)
- **the api socket's inode** — the socket file is recreated (new inode, new
  btime) by *both* a restart and a live handoff, verified. `stat(2)` on
  `socket:` is therefore an exact "same server generation" fence, purely local,
  no herdr call: same path + different inode ⇒ this server did not issue that
  id, so the id is evidence about nothing.

**What landed** (rangerhq-yt1p, `internal/rhq/herdrback.go`). `gen:` in the
session meta — `dev:inode` of the api socket, stamped at create, backfilled
on positive identity — plus one predicate, `notOurWorkspace`, asked by all
three destructive paths through `idEvidence`: the prune (`Sessions`), the
create (`mustNotOrphan`, which `posse relaunch`'s unlink also calls), and the
listing itself. The listing is where the damage actually was: a stale meta
whose id a stranger now holds used to be listed *over* that workspace, and
every addressing path (`Resolve`, `AgentTarget`, `KillSession`,
`RelaunchAgent`) reads that listing — so the name prompted into somebody
else's pane and `posse kill` closed it. Such a meta is now kept, left out of
the listing, and reported with the repair recipe above.

Two decisions worth keeping. The fence is **not** a third arm of
`cannotAnswerFor`: a generation mismatch there would keep every meta forever
after any restart, and it is not even true — `workspace_not_found` is still
proof of death in any generation, because a workspace never changes its id
while it exists (the ids that survive a restart or a handoff are the live
ones, unchanged). And the create **refuses** rather than repairing itself
onto the label-matched workspace: a repair is only ever right if the session
were alive under a *different* id, which cannot happen, so the post-flight
repair stays the operator's deliberate act.

One migration cost, and it is one-time and visible: a meta written before
`gen:` existed whose workspace was renamed in herdr reads as a stranger until
it is repaired or the session is recreated — kept, not listed, warned about
by name.

## The shared working tree: one index, seven personas (rangerhq-nyqj)

Every persona dispatched into a repo gets `Dir: is.Dir` — the *same*
checkout: one working tree and one `.git/index` shared by everyone
the loop launches, and as of 2026-08-22 that loop is armed and unattended
(`autostart_interval: 5m`, `autostart_dry_run: false`, `autostart_max_beads: 3`).
Concurrency here is the design, not an accident.

**The failure, reproduced.** Persona A stages its paths; persona B runs
`git add` + `git commit` in the window before A commits; B's commit carries
A's staged files and A is told *"nothing to commit, working tree clean"*.
That is d15c55a — a `bd sync:` commit carrying eight files another persona
had staged for rangerhq-3hb5
— and it reproduces in four lines in a throwaway repo. `bd sync` is not
the culprit: it "does NOT stage or commit - that's the user's job" (`bd sync
--help`); the sweeper is the persona's own unqualified commit. Staging only
your own paths does not help, because the index is one file and the commit
takes whatever is in it.

**The commit form that is immune, measured.** A path-limited commit —
`git commit -m … -- <paths>` — builds a *temporary* index, commits only the
named paths, and leaves the other persona's staged entries untouched in the
shared index. Verified both directions: the sweeping form loses A's work, the
path-limited form commits only B's path and A's `git diff --cached` still
shows its own staged file afterwards.

**A hook can tell the forms apart — but not the obvious way.** git exports
`GIT_INDEX_FILE` to hooks, and only a genuine path-limited commit gets a
`next-index-*` temp index:

| commit form | `GIT_INDEX_FILE` | verdict |
|---|---|---|
| `git add … && git commit` | `.git/index` | sweeps |
| `git commit -- <paths>` | `.git/next-index-<pid>.lock` | safe |
| `git commit -a` | `.git/index.lock` | **sweeps** |

Do not match on "is it a temp index": `-a` gets one too, and it is the *worst*
form (it takes every persona's modified tracked file). Match on `next-index-*`
specifically. The discriminator was verified against all three forms.

**And do not match on the NAME ALONE — rangerhq-cqq1.** `GIT_INDEX_FILE` is
the caller's environment variable, so a glob on its name is a wall exactly one
spelling wide: the same private-index recipe was refused as `<tmp>/index` and
waved through as `<tmp>/next-index-mine`, landing the commit and leaving the
shared index stale (the full rangerhq-8rtf chain, under a persona). The
exemption now asks what git actually *does*, measured on git 2.39.3 in a main
repo and a linked worktree: git's temp index is an **absolute path**, it lives
in `git rev-parse --git-dir` — the **per-worktree** dir, not the common one —
and it is named for git's own pid. So the hook requires basename
`next-index-<digits>[.lock]` **and** its directory, resolved with `pwd -P`,
equal to the resolved git dir. Not the pid itself: the hook's `$PPID` does
equal it, but under the hook chain INSTALL.md prescribes the gate is the
dispatcher's child, so a pid check would refuse the crew's only safe route in
every chained install (verified). Residual, stated plainly:
`GIT_INDEX_FILE=$GIT_DIR/next-index-1` is still exempt — a private index
deliberately placed inside the repo's own git dir, which is a decision rather
than a temp file that happened to be spelled right.

**Isolation: per-session worktrees, LANDED (rangerhq-09o2).** The decision
(rangerhq-nyqj) was per-session `git worktree` — one tree, one index, one HEAD
each — because it is the only candidate that also removes `git checkout <path>`
discarding another persona's edits, `git stash` taking someone else's WIP,
half-written files landing under a green message, and two personas racing
writes to one file. A commit lock fixes only the sweep. It is now built
(`internal/rhq/worktree.go`), and the merge-back is the operator's option A
(rangerhq-jbyr): **the launcher merges**.

What a dispatched session gets:

| | |
|---|---|
| main checkout | `~/src/<repo>` — the operator's, on `main`, unchanged |
| session worktree | `<worktrees root>/<repo>/<session>` — its own tree, index, HEAD |
| session branch | `posse/<session>`, cut from the repo's own branch |
| merge-back | the launcher fast-forwards it onto that branch when the bead closes, under the ADR 0011 §1 launcher lock |
| retirement | `posse kill` lands the branch then removes tree and branch — and REFUSES while either holds work |

The kill takes the launcher lock **without waiting**: the cockpit's `k` runs
it on the TUI's single select loop, and blocking there behind a firing pass
freezes the cockpit. Losing that race costs only time — the workspace is
closed, the tree and branch are kept, and the line says
`posse worktrees --land` finishes it. `--land` merges every landable branch
under one blocking lock and **never removes a tree**: it reads git, so it
cannot tell a dead session's tree from one a persona is working in this
second, and removing the second is the exact damage this feature exists to
prevent.

Crew sessions (`posse new`, recipes) keep the operator's checkout: a session
the operator opened to talk to is theirs (ADR 0008). A dir that is not a git
repo, or a repo on a detached HEAD with no session branch yet, warns and falls
back to the shared checkout — a launch must not die because somebody is
bisecting. A detached HEAD only removes the branch a tree would be **cut**
from; a session whose branch already exists keeps its tree, and the launch
says the landing waits for a branch rather than that the tree is shared. That
distinction was ranger-base-q5p1: a relaunch while the operator bisected
blanked `repo:`/`branch:` out of the recreated run record, and every later
close and kill then read a live private tree as the shared checkout and
skipped its landing — losing the work quietly, which is worse than deferring
it loudly. One thing the same bead measured and did **not** change:
`MainCheckout` answers in the caller's own spelling, and git gives it
`.git` relatively from the main checkout but an already-resolved absolute
path from inside a linked worktree — so under a symlinked path one repo has
two spellings, and two of its answers must be compared with `resolveExisting`
rather than as strings. Normalizing it there instead rewrites the
`.beads/redirect` posse seeds into every session tree, which is an
operator-visible file changed to buy nothing outside a symlinked checkout.

The base is **recorded on the branch** when the tree is cut
(`branch.posse/<session>.posseBase` in the repo's git config), and merge-back
refuses unless the main checkout still has that branch out. Reading the repo's
HEAD at merge time instead was ranger-base-5s2o: `git merge --ff-only` moves
whatever is CHECKED OUT, so an operator who ran `git checkout -b` mid-session
got the persona's commits fast-forwarded onto their own branch while the pass
reported `main` merged. The operator's checkout is the one store on this path
the launcher lock does not govern, so the question is asked immediately before
the merge — and refusing costs only time: the branch still holds the work, the
pass says which branch is in the way, and `posse worktrees --land` finishes it
once the base is checked out again. Git keeps the record rather than the run
record, for the reason `posse worktrees` reads git: a kill that could not land
removes the meta and leaves the tree standing.

Config: `worktrees:` (default `~/.posse/worktrees`, and it **must** be under
`$HOME`) and `worktree_link:`, a declared list of repo-relative gitignored
paths symlinked from the main checkout into each fresh tree (`plugin/bin`,
a local settings file). Declared rather than guessed — linking every
gitignored path links build output two sessions then race over. `posse
worktrees` lists every session tree and what has not landed; it reads **git**,
not the meta dir, so a tree orphaned by a kill that could not land is still
findable.

What was measured, before and during the build:

- `bd worktree create <name>` is a first-class subcommand: it makes the
  worktree, writes `.beads/redirect`, and gitignores the path, so **all
  worktrees share one beads database**. The graph does not fork. posse writes
  the redirect itself rather than shelling out to it — deterministic, no
  mutation of the repo's `.gitignore`, and testable without bd.
- **Without a redirect the graph DOES fork, silently.** Measured on bd 0.49.1:
  a linked worktree with no redirect makes bd read the checked-out
  `issues.jsonl`, report "fresh clone detected", and build a second database
  beside it. That is the failure the seeding exists to prevent.
- **The redirect posse writes is ABSOLUTE.** bd's relative form resolves
  against the worktree ROOT, not against `.beads/` — one `..` off and bd warns
  once and silently falls back to a stale path. An absolute path has no such
  arithmetic to get wrong, and it resolves the main checkout's OWN redirect
  first so a chain is never built (this repo's `.beads` is itself a redirect
  to ranger-base's database).
- **The staleness trap does not fire through a correct redirect.** A worktree
  checkout does materialize a tracked `.beads/issues.jsonl` with a fresh
  mtime, but bd compares the mtime of the jsonl beside the database it
  RESOLVED to. Measured: touching the worktree's copy forward changes nothing
  (an hour into the future changes nothing); touching the MAIN repo's copy
  forward is what raises "Database out of sync with JSONL. Run 'bd sync
  --import-only'". So the worktree's copy is inert and is deliberately left
  alone — deleting or back-dating it would dirty a tree the persona is about
  to commit from. No `--allow-stale`, and no `bd sync --import-only` left
  sitting as a persona's obvious next step. **Stated precisely, because the
  warning has not been abolished**: a genuinely stale main jsonl — after a
  `git pull`, or after bd's own pre-commit hook rewrote it — still raises it,
  in the worktree and in the main checkout alike, because it is one database
  and the fact is true of both. What worktrees do not add is a *new* source
  of it. The live pin (`RHQ_LIVE_BD=1 go test ./internal/rhq -run
  TestLiveWorktreeSharesOneGraph`) asserts exactly that discrimination.
- `redirect` is in bd's own bundled `.beads/.gitignore`, so seeding one leaves
  the worktree clean in any bd-initialised repo.
- bd's bundled `pre-commit` and `post-merge` hooks are already worktree-aware
  (they resolve `.beads` via `--git-common-dir`).
- **`git merge --ff-only` in the main checkout survives an unrelated dirty
  tree** and refuses rather than clobbering when the changes collide, so the
  operator having work in flight is not a reason to skip the merge and never
  a reason it loses anything. When the base has moved the launcher rebases in
  the SESSION's tree — the operator's checkout is untouched by the risky half
  — and a conflict is `rebase --abort`ed, leaving branch, tree and repo
  exactly as they were, and files a bead (ADR 0006 §1) at the persona.
- `git commit -- <path>` on a path git has never seen fails with "pathspec did
  not match any file(s) known to git", so the blessed commit form needs an
  `add` in front of it for a NEW file. Not new behaviour; newly written down.
- **Put session worktrees under `$HOME` — and do not expect bd to enforce it.**
  An earlier version of this note claimed bd refuses a `.beads` under `/tmp`
  ("BEADS_DIR points to unsafe location"). That is **false** on bd 0.49.1
  (corrected in rangerhq-80fx). Measured on both arms: `BEADS_DIR` set to
  `/tmp`, `/private/tmp` or `/var/tmp` is accepted by `bd list`, `bd status`
  and `bd sync`; and `bd worktree create /tmp/<name>` **succeeds** — it makes
  the worktree and writes a working `redirect`. The refusal string is real but
  guards a different thing: `isPathInSafeBoundary` has exactly two call sites
  (`internal/beads/context.go`), both validating the *resolved* `BEADS_DIR`,
  and neither the init nor the worktree-create arm validates the worktree's own
  path. It structurally cannot: `bd worktree create` points the redirect at the
  **main** repo's `.beads` under `$HOME`, so the path that gets validated is
  always the safe one no matter where the worktree sits. `/tmp` is permitted by
  design besides — `unsafePrefixes` omits it, and an explicit branch admits
  `os.TempDir()`. So `$HOME` is **our** constraint to keep, not a net bd holds
  under us; a session scratchpad is reaped, and a reaped worktree under a live
  session is exactly the failure that imagined net was assumed to prevent.
  `WorktreeRoot()` refuses a configured root outside `$HOME`.
- The `pre-commit` slot is **already taken by bd's hook**, and the posse gate
  precedent is "foreign hooks are never overwritten" (pre-push, ADR 0002 §3).
  A posse commit gate has to chain or use `core.hooksPath`, not claim the slot.

**The shared-index wall now asks whether an index is actually shared.** laurie
measured (on rangerhq-09o2) that the `prepare-commit-msg` guard installs into
the COMMON git dir, so every linked worktree inherits it and is refused there
too — under a message saying the index is "shared by every persona", which is
exactly what a session worktree's index is not. The arm now exits 0 when
`git rev-parse --git-dir` and `--git-common-dir` differ (resolved with
`pwd -P`; they differ only in a linked worktree). Two consequences worth
knowing: in your own session tree the ordinary `git add` + `git commit` is
fine, and in the shared checkout nothing changed. The **L3 parity probe** moved
with it — it now runs in the MAIN checkout, because probing inside a worktree
would read a wall that is right to be quiet as a wall that is not there and
degrade every launch into the repo.

**The wall, landed (rangerhq-lmq9).** In a SHARED tree the commit form *is*
the rule — `git commit … -- <your paths>`, never `git add`+`git commit` and
never `git commit -a` — and it is enforced in two layers rather than asked
for. Since rangerhq-09o2 that is what it says: the wall stands in the
operator's checkout and stands down in a session worktree, where there is no
shared index to sweep.

*L1, the typed line.* The shim's rule grammar gained a **negative match**:
`Bash(git commit unless --)` refuses `git commit` UNLESS argv carries `--`
with at least one operand, matched after git's leading global options are
skipped, so `git -C <repo> commit -m x` is caught too. The operand is part
of the rule because `git commit --` with an empty pathspec reaches git with
the *shared* index (measured — it is a fourth row for the table above,
verdict: sweeps). `-a` needs no case of its own: git itself refuses
`commit -a` together with a pathspec (`fatal: paths … with -a does not make
sense`), so a rule that demands a pathspec excludes `-a` by construction.
Claude's dialect has no negation, so `L0Spellings` widens a negative rule
into the two **exact** shapes that are unsafe whatever follows —
`Bash(git commit)` and `Bash(git -* commit)` — and nothing longer, because
anything longer might be the safe form.

*L3, the hook — and the slot is `prepare-commit-msg`, not `pre-commit`.*
Two measured reasons. `pre-commit` is bd's, bd reinstalls it, and a wall a
third-party tool silently replaces on its next install is not a wall. And
`git commit --no-verify` **skips `pre-commit` while `prepare-commit-msg`
still runs** — so the free slot is also the stronger one. Both hooks see the
same `GIT_INDEX_FILE`, verified across all four forms. The guard keys on
`RHQ_PERSONA`, the way the pre-push gate keys on `RHQ_TOOLS_DENY`, so the
operator's own commits in the same tree are untouched; it is installed at
every persona session create and by `posse gates install-hooks`, and is *not*
keyed on a deny rule, because what makes the commit unsafe is the tree the
session was dispatched into, not anything the PID says. Session create then
executes a persona probe against the slot and counts L3 only when the hook
refuses with exit 1; the same single shell invocation probes pre-push when
that PID denies it. A failed commit probe degrades every persona launch into
the repo because this slot carries both the shared-index wall and the beads
visibility guard.

Commits git drives itself — merge (`$2` = `merge`), cherry-pick, revert,
rebase, squash — are let through: git refuses a pathspec outright during
those ("cannot do a partial commit during a merge"), so refusing them would
leave no way through rather than a safer one. `git commit --amend` is not
one of them; it takes a pathspec and sweeps without one, so it is refused.

Two holes worth naming. A pathspec of `.` satisfies both layers and still
sweeps the tree — the refusal message says "name your own paths, not `.`",
which is manners again, and only worktrees close it; per-session worktrees
(rangerhq-09o2) close it for every DISPATCHED session, and it stays open in
the shared checkout. And a PID must carry
`Bash(git commit unless --)` for the L1 half to render; the crew's PIDs live
in `RHQ_HOME/agents/`, outside every persona's working dirs, so as of
2026-08-22 the L3 hook is the layer actually standing.

**What the wall covers, and what it does not (rangerhq-2f5r).** rangerhq-2f5r
is this same failure reported a day earlier, from the other side: one
persona staged exactly its six paths, `git status --short` confirmed them,
and the commit seconds later carried another persona's whole in-flight test
and none of the six. Verified against the live `prepare-commit-msg` hook in
this repo, 2026-08-22, all four forms of that incident:

| form | verdict |
|---|---|
| `git add <mine>` then `git commit -m …` — the incident itself | refused |
| `git commit -a` | refused, named in the message as `git commit -a` |
| `git commit -F - -- <mine>` | commits only `<mine>`; the other persona's staged entry is still in `git diff --cached` afterwards |
| `GIT_INDEX_FILE=<private> …` — the workaround that bead recorded | refused |

**The residual, measured: the blessed form still takes another persona's
in-flight edit.** `git commit -F - -- <path>` commits the **working tree**
content of the named path, not what you staged. So when another persona is
editing a file you name — rangerhq-2f5r's own step 1, the other persona's
edit landing in `launchlock_qa_test.go` *after* the first's — their
half-written lines ride into
your commit under your message, and both layers pass it, because the form is
correct. Measured: two personas append to one file, neither stages,
`git commit -F - -- shared.txt` lands both. No refusal fixes this; it is the
"half-written files under a green message" case that only rangerhq-09o2's
isolation removes. Until then the gate's "name your own paths" is narrower
than it reads — it protects the **index**, not the **file**.

**The private-index workaround stays refused, and should.** ef8d35f was
committed with `export GIT_INDEX_FILE=$(mktemp -d)/index; git read-tree HEAD;
git add …; git commit`. That index is genuinely private — no other session can
reach it — and it is the only form that can commit a *subset of hunks*, which
the path-limited form cannot do at all. The hook refuses it anyway (it is not
git's own temp index — wrong location, and since rangerhq-cqq1 the name alone
no longer buys past that), and that is the right trade: measured, the same
recipe with `read-tree HEAD` forgotten commits a tree containing **only** the
added path, recording every other file in the repo as deleted (in a
three-file repo: `other1.txt | 1 +`, `other2.txt | 1 -`, `shared.txt | 3 ---`).
An escape hatch a persona reaches for *because* it just hit a refusal, one
forgotten line from a tree-wiping commit on main, is worse than the capability
is worth. Do not revive the recipe from that bead's description.

**The record it left, permanently.** Both commits are pushed, so nothing is
rewritable and nothing should be. `git log --grep rangerhq-th7l` returns two
commits carrying the *identical* message "verify-after: take the launcher lock
— filing is acting": **ef8d35f is the fix**; **3b5f665 is that same message
over the other persona's content** — the `launchlock_qa_test.go` pin from
rangerhq-s7qz,
swept out of the shared index. A verify step that greps the log finds the
wrong diff first (ADR 0006 §2). That ambiguity is the permanent cost of one
unqualified commit, and it is the argument for the wall in one line.

**The other half of the private index: it does not just mislabel your commit,
it lets the next one REVERT you (rangerhq-8rtf).** This is the half
rangerhq-2f5r's "WORKAROUND IN USE" never stated, and it is the worse half.
A private `GIT_INDEX_FILE` makes *your* commit correct and leaves the SHARED
`.git/index` untouched — still holding the **pre-fix blobs for every path you
just committed**. The next commit taken from the shared index therefore commits
them back. That is what a `bd sync` is. Measured in scratch repos, all three
forms, 2026-08-22:

| the commit that lands the fix | shared `.git/index` afterwards | next unqualified commit |
|---|---|---|
| `GIT_INDEX_FILE=<private> … git commit` | **stale** (pre-fix blobs) | **silently reverts the fix** |
| `git commit -F - -- <paths>` (the blessed form) | refreshed for the named paths | fix survives |
| `git commit -F - -- <paths>` + bd's pre-commit hook | stale **for `.beads/issues.jsonl` only** | bd's hook re-flushes it, so it self-heals (rangerhq-be7k) |

So the blessed form is clean against this, and the wall refuses the private
form for personas — verified against the live hook: with `RHQ_PERSONA` set the
private-index recipe is refused, and with it unset the full chain reproduces
end to end (fix lands, index stale, next `bd sync` reverts it).

**That was true of the recipe's spelling before it was true of the recipe
(rangerhq-cqq1).** As written on 2026-08-22 this paragraph said the class was
"out of the crew's reach"; it was one filename wide — `GIT_INDEX_FILE` ending
in `next-index-anything` walked straight through, and the whole chain
reproduced under a persona. Fixed by matching git's temp index by location and
pid shape (see the discriminator note above), landed with a mutation-tested pin
in `TestSharedIndexCommitHookRefusesHandRolledNextIndex`. Read the sentence
below as true from that fix forward, and as **false for anything measured
before it**.

**The residual is the operator**, who is exempt from the wall by design and
whose `bd sync` commits are exactly the unqualified form. That is a gate
working as specified, not a hole to plug — but it means the class is not
extinct, only out of the crew's reach.

**Why this one is nastier than the mislabel.** The three-hour window it opened
on main was green: dcca7b5 removed the lock AND restored
`t.Skip("rangerhq-th7l: …")` on the regression pin in the same act, because the
pin's unskip was part of the same reverted commit. `go test ./...` passes, the
defect is live, and `git log --grep <bead>` shows the fix landing. A verify step
dispatched inside that window (ADR 0006 §3) would have read three commits, a
green suite and a skipped pin, and reached the wrong verdict honestly. **A
silent revert takes the detector with it** — that is the general shape, and it
is why the mislabel (visible in `git log`) is the lesser failure.

**The audit, and it is a standing one: `make audit-silent-reverts`.** Nothing
in the loop reports this class; it was found by a human reading history for an
unrelated reason. `scripts/audit-silent-reverts.sh` is one `git log --raw` pass
that flags any commit setting a path back to a state it held *before* its
immediately preceding change — and **absence is a state**, so deleting a file
that was added earlier in the range counts. That second half is rangerhq-ypn1:
when the change landed from the private index is an ADD, the stale shared index
has no entry for the path, so the undo is a DELETION rather than a rollback, and
the scan used to skip deletions on the rationale that "a removal is visible in
review" — which rangerhq-8rtf disproves, since nobody reviewed dcca7b5. That
shape is the worse one: the regression pin does not need re-skipping to keep the
suite green, it stops existing, and `go test ./...` says "[no test files]" and
exits 0. `--self-test` plants the real mechanism in a throwaway repo in both
shapes and asserts the flag, plus a plain move and asserts silence, so a clean
run means the detector still works rather than that the script still runs.
Triage lives in `scripts/silent-reverts.allow` (`<sha> <reason>`); anything
untriaged exits 1, so clearing a hit means writing down why.

Full history as of the ypn1 fix: **449 commits, five hits, one real** —
dcca7b5 (the incident), 1cc432e (its repair, flagged by construction since
re-landing a byte-identical diff *is* a rollback of the revert), 21653f9 (the
pre-crew herdr-native pivot, where `examples/config.yaml` returns to an older
shape because `slot_preference` was removed), and the two the deletion rule
added: 9daf91f (untracking `.beads/interactions.jsonl`, which is what that
commit is *for*) and 631bda7 (the rhq → posse rename, four renames that also
edit the file, so the exact-blob move exception does not cover them).
**dcca7b5 is still the only silent revert on main** — the deletion rule found no
new incident, which was measured independently under rangerhq-jkhb before it was
written. What the audit cannot see, so do not read it as more: exact-blob
matches only — a *partial* revert of some hunks is invisible, and a rename that
also edits the file reads as a deletion and needs a triage line — it sees only
what was committed, never work reverted in the working tree before it landed,
and the deletion rule needs the path's add inside the scanned range, so a short
`a..b` range under-reports.

## Testing

`go test ./...` (also `make test`) is hermetic: the test binary re-execs as
a fake `herdr` when `RHQ_FAKE_HERDR=1` (state under `RHQ_FAKE_DIR`), so no
real herdr server is involved. `RHQ_HERDR_BIN` / `RHQ_BD_BIN` point the
runners at any binary — that's how the fakes are injected.

Hermetic against *our own wall*, too: the suite usually runs inside a
persona pane, whose PATH leads with that session's gates bin. Its `git`
shim was answering `TestPrePushHook`'s real `git -C <repo> push` before
the hook under test ever ran — one assertion failed, and the one that
"passed" was reading the shim's refusal, not the hook's (rangerhq-8sd).
`TestMain` now sets PATH to `PathOutsideGates("")` for the whole test
binary; a test that wants a shim on PATH renders one and prepends it.

### The suite on Linux — `make test-linux`

The suite ran on darwin and only on darwin until 2026-08-24, and two defects
lived in that gap until a release rehearsal found them: `ServerGen` fenced herdr
generations on an inode number, which ext4 and overlayfs recycle and APFS does
not (ranger-base-fjj — a live bug in the linux tarballs, not merely nine red
tests), and one gate test asserted macOS's `/bin/zsh` where the contract is a
PATH search (ranger-base-gaf). Neither is exotic: this code reads filesystem
identity and resolves shells, so it is platform-sensitive by nature and darwin
hides one whole half of it.

`make test-linux` closes that gap without CI: `go vet ./... && make test` — the
same two commands `.github/workflows/release.yml` runs — inside a throwaway
`golang:<go.mod>` container. ~35s cold, ~2s warm. The repo is mounted
**read-only** and the container runs as *you* rather than root, so a run cannot
leave a root-owned artifact or a rewritten `go.sum` behind; a test that needs to
write must use `t.TempDir()`, which is what CI requires of it anyway. The build
and module caches are the one writable thing and they live in
`~/.cache/posse/test-linux`, outside the tree.

**It works from a session worktree** (ranger-base-v0gm). It did not, at first:
the script mounted `$REPO_ROOT` and nothing else, and in a linked worktree
`.git` is a *file* reading `gitdir: /Users/…/src/posse/.git/worktrees/<session>`
— a path outside the mount, so git in the container resolved nothing and the
three seedpub publication-boundary pins (ADR 0012) failed with `fatal: not a
git repository` 40s in, looking exactly like product failures. Since every
dispatched session works in a worktree, the gate ORDERS tells personas to run
was unrunnable-clean for every persona, and the honest report was "green except
three known env failures" — one step from "green enough". The script now asks
`git rev-parse --path-format=absolute --git-common-dir --git-dir` and mounts
whatever those name outside `$REPO_ROOT` **at the same absolute path**, because
that pointer is baked into the `.git` file and cannot be relocated. Those
mounts are `:ro` like the repo, and are added to `safe.directory`; an ordinary
checkout has its git dir inside `/repo` already and gets no extra mount.
`release_qa_test.go` pins all of it against a fake git, so both branches are
exercised wherever the suite runs. Skipping the seedpub tests was never the
fix: they *are* the publication boundary.

It tests the host's architecture (arm64 on an Apple-Silicon box).
`PLATFORM=linux/amd64 make test-linux` crosses that under emulation, slowly.
`IMAGE=` overrides the toolchain image; `scripts/test-linux.sh --shell` drops
you in there, and `scripts/test-linux.sh '<cmd>'` runs one command.

The release workflow is still the last gate, not the first one: run this before
you push, because on a tag is the worst place to learn that Linux disagrees.
