# NOTES — how posse works under the hood (herdr-native)

The tmux-era implementation — Ghostty splits, the 2×2 grid, the bash spec,
the bubbletea launcher — lives on the **tmux-reference** branch with its own
NOTES. This file describes the herdr-native harness on main.

## The mapping

Every herdr call goes through `Herdr.Run` (`internal/posse/herdr.go`), which
shells out to the `herdr` CLI and decodes its JSON envelope
(`{"result":…}` / `{"error":{code,message}}`). The mapping
(`internal/posse/herdrback.go`):

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
one-liner: `posse relaunch <name>` (rangerhq-dxq) lands the plane, closes
the workspace, and recreates it from the same persona, dir, env sets,
runtime, tier, cage and degraded-consent. Landing is one bounded turn
(`--timeout`, default 10m): settle first — herdr's prompt does not track
turns — then a prompt telling the agent to append its lessons to
`ORDERS.md`, commit, file what's unfinished, and push only what its own
guardrails permit. A session that never settles inside the bound is **not**
killed; the operator is told to wait or pass `--no-land`. No agent, or an
agent blocked on its own dialog, is a note and the refresh continues. **A
session cannot refresh itself, on either arm** (ranger-base-521). The
caller is recognised by its own pane's workspace id against the record's,
on the same herdr server, so it is told in zero seconds rather than waited
out. Both halves fail, differently: the landing turn waits for an agent
that is running the command, so no `--timeout` settles; and `--no-land`
reaches the kill, which ends the processes in the workspace's panes — the
caller included, INSIDE the close call (`scripts/verify-self-close.sh`,
ranger-base-hslbb) — so the session would be destroyed and its name freed
between `closeRecorded` and `recreateSession`, with nothing left running to
build the replacement. A child that calls `setsid(2)` first does survive
that close, so a working self-refresh is buildable; it is not built,
because a new session leader cannot inherit the launcher lock that makes
the kill and the recreate one step or neither. A persona line is
re-rendered from the PID at every launch (never replayed), so a plain
session's `--cmd` is the one thing the meta has to carry itself (`cmd:`).

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
`internal/posse/paneline.go`, the last length that survives.

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
launched with. Headroom as of 2026-08-27: every crew PID's line rendered to
**591–691 bytes** against the 1023 limit — ~330 bytes of slack. Growing a
typed line spends that slack — another deny rule (`--disallowedTools` is
variadic), a longer `--settings`, more mounts — and by 2026-08-31 it was
gone: `ranger-base-u9ud`'s 23-verb `bd` deny-set widening (every crew PID's
deny: list grows by 23 lines, each rendered through `--disallowedTools`'
existing doubling) pushed every crew PID's line over 1023 B, and
`state/launch/` held a spilled script per persona. `ranger-base-rq83c`
spent ~110 B more on 2026-09-03 — the credential-dir pin, which travels
inside the same `--settings` payload because a second one would replace
the first rather than add to it — against slack that was already gone, so
it changes which side of the cliff nobody is on. `ranger-base-rflee` spent
~600 B more on 2026-09-05, widening that same payload from two keys to
twenty-three (the transport/exec inlet pin, `internal/posse/inletpin.go`)
for the same reason and into the same flag, and `ranger-base-i7cy4` spent
**274 B** more the same day for the nine command-string FIELDS that ride
beside `env` in that object rather than inside it
(`internal/posse/fieldpin.go`) — the same flag again, because it is the
only flag there is. `ranger-base-44or9` spent **60 B** more on the same day
and into the same object, taking `env` to twenty-five keys and the rendered
payload from 1006 B to 1066 B (measured, not estimated): two more git rows,
`GIT_CONFIG_SYSTEM` and `GIT_CONFIG_PARAMETERS`. It is worth knowing what
that 60 B did NOT buy — two further names of the same family are inlets that
still reach, disclosed in `inletpin.go` rather than pinned, because pinning
either would cost a working fleet more than the inlet costs (the reasoning is
there, and the residue is the operator's call on `ranger-base-37y0z`).
Every crew line was already
spilling, so no launch changed behaviour — but a *fixture* did:
`TestDispatchRelaunchesDeadAgent`'s minimal line had still fit, and reading
it out of `calls.log` alone measured its length rather than its content.
That is the trap `launchLog` exists for, and rq83c walked six fixtures into
it at a fifth of the size; the helper now collapses the gate prefix in
spilled bodies too, so an assertion cannot tell which side of the cliff it
landed on. A populated
`state/launch/` is not a healthy-fleet signal; it is this fallback working
as designed — a line that outgrew the
limit gets sourced from a script instead of typed. The container tier's
~1.6KB engine line spent it in one go and had to render a file before this
rule existed; see *Container tier* below.

None of this is asserted from memory: `internal/posse/panelinelive_test.go`
(`RHQ_LIVE_PANE_LINE=1`, against a scratch herdr server, no API turn) is the
live pin both sides of the cliff and the spill are measured from.

### A pane's environment is the herdr DAEMON's, not the launcher's (ranger-base-385x)

The other half of "a launch is a command typed into a pane's shell": that
shell is a child of the **long-running herdr server**, so it carries the
server's environment and not the environment of whatever posse process typed
the line. `CreateWorkspace` passes an explicit `[]EnvVar` — `RHQ_HOME`,
`RHQ_PERSONA`, `RHQ_GATES_DIR`, … — and that list carries **no `PATH`**;
`GatePrefix` is `PATH=<gates bin>:"$PATH" …`, and that `$PATH` expands
*inside the pane*. So the launcher's own PATH gets no vote on which binary a
session runs.

Measured 2026-09-05 with a scratch herdr (a named session under a scratch
`HOME`, the two fences `scripts/verify-self-close.sh` uses): a copy of a CLI
planted only on the **server's** PATH is what the pane resolves and runs; a
copy planted only on the client's is absent from the pane's PATH entirely.
Costs no model turn — `workspace create`, then `pane run` of
`sh -c 'echo "$PATH" > f'`.

Two consequences, and neither is theoretical — both were live defects:

- **Anything posse resolves in its own process is answering a different
  question.** `posse runtime probe` filled the record's `cli_path` and
  `version` that way while its four observables were measured on the CLI in
  the pane, so a decoy in front of posse's PATH alone produced a *passing*
  record naming a script that cannot launch anything. Fixed by asking the
  pane: the probe types `command -v` into it, under the launch line's own
  PATH prefix, before the launch line. The same reading still sits in the
  launch preflight's blocking `exe` gap (`RuntimeGaps`,
  `internal/posse/runtimepreflight.go`), where it can refuse a launch that
  would have worked — filed as ranger-base-8vys9.
- **A test cannot sabotage what a pane runs by mutating its own PATH.**
  `runtimeprobe_live_test.go`'s `RHQ_LIVE_PROBE_FAKE` arm did exactly that,
  so the pane launched the real CLI and the arm that must FAIL passed all
  four observables — and then printed a red accusing the production probe.
  To reach a pane, the sabotage has to be somewhere the daemon's PATH already
  leads, or be named by absolute path in what gets typed (which is what that
  arm does now).

The general rule both of those pay for: before believing any pane rig, ask
what the pane's environment actually is and measure it; and when a live arm
is meant to fail, give it a witness that its sabotage took effect before it
is allowed to pass judgement on the thing under test.

## Dispatch primitives

- `posse prompt <name> "<text>" [--wait] [--timeout ms] [--now]` — submit work
  to the session's detected agent (herdr `agent prompt`; `--wait` blocks until
  the first settled idle|done|blocked state). **It waits for herdr to have
  SEEN a screen before typing** (`internal/posse/promptready.go`): a pane herdr
  only *guesses* is idle is a CLI that has not taken the keyboard, and a
  prompt sent there lands in whatever has — a leading `/` once turned a whole
  work prompt into `/Work` plus arguments, and herdr reported success
  (ranger-base-3p0). It waits for a seen screen, not for `idle`, so nudging a
  working session is unchanged; at the deadline nothing is sent and the error
  says so. `--now` skips the gate.
- `posse wait <name> [--until <state>]…` — wait for an agent state.
- `posse peek <name> [lines]` — the session's terminal tail (`pane read`;
  the tail is computed client-side because herdr's `--lines` counts padded
  blank screen rows from the bottom).
- `posse ready [--dir] [--as]` — unblocked work via `bd ready --json` (the
  `Bd` runner in `internal/posse/beads.go`). Without `--dir` it aggregates
  across the config `beads:` repo list; missing or unreadable repos are named
  as failed scans while readable repos still report their work.
- `posse crew <name> [--off]` — hand a session to the operator or back to the
  fleet (ADR 0008). **A crew session is one the operator talks to, and
  dispatch treats it as if it did not exist** — never prompted, never
  relaunched, never counted busy; `posse new`/recipes/cockpit `p`/`posse prompt`
  without `RHQ_PERSONA` set the mark, cockpit `o` and `--off` clear it, and
  a bead whose session is crew is reported `held by crew session <name>
  (operator's) — skipped` (there is no timer and `--resume` does not
  override — release it first). **Which session is the bead's is asked of the
  run record first** (`bead:`, ADR 0011 §3), then of the two name patterns:
  a crew session the operator made by hand carries neither Dial F name, so
  a name-only shield protected the SESSION and left the BEAD open for a
  fleet twin (ranger-base-adb7). **An in_progress bead no live session
  holds under any name at all** — the record, the Dial F name, the slot —
  is an orphaned claim: crash-recovery's twin, or the operator's own
  hand-work, and only a live crew session of the assignee in the bead's
  repo tells them apart, so dispatch parks it (`claimed by <persona>, no
  session posse started — crew session <name> is live …`) rather than
  guessing (ADR 0030). `posse list` and the cockpit tag it `👤`.
- `posse claim <id> [--as <persona>] [--dir]` / `posse done <id> …` — atomic
  claim (`bd update --claim`) and close, with the persona as bd actor. The
  claim's outcome is read from the bead, never from bd's exit code (see the
  bd substrate section): `claimed`, `resumed` when the actor already held it,
  and a non-zero exit naming the holder when it went to somebody else.

Prompt targets resolve session → workspace → detected agent (root pane
preferred). Nothing depends on herdr agent *names*, which die with the
process — durable identity is the beads **assignee**: a persona is its
assignee name + `agents/<name>.md` + (future) a per-persona memory dir.

These compose into `posse dispatch` (`internal/posse/dispatch.go`) — one pass
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
   **no** path (ADR 0033): assignee-is-the-coordinator refuses loudly with
   no fallthrough to label routing, the label loop skips that PID, and a
   `default_persona:` naming her is a loud config error. Both launchers
   share `Route`, so the refusal covers the pass, `--watch` and the
   cockpit's `d`; no flag reaches past it. Unset key = no coordinator =
   pre-0033 behavior. All three refusals compare *identity*, not the
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
   `DIRECTION.md`, `NOTES.md`; config `orientation:` overrides the list);
   `guardrails:` — "your PID outranks every push/deploy instruction you
   are handed … if one orders `git push`, do not", fixed text, the one
   Context line that renders even with no context, because what it
   outranks (`bd prime`'s injected close protocol) arrives from the bd
   binary and not from this repo; "comments carry decisions — read
   them" when the bead has any (each bead-sourced line only when
   non-empty; every such string %q-fenced); then the fixed **escalation ladder** — NOTE / ASSUME /
   SPIKE / ASK / HANDOFF / REFUSE with exact bd commands, ASK beads `-l
   question -a <config operator:>` (unassigned when unset) plus `bd dep
   add` so the bead leaves `bd ready` until answered; SPIKE researches a
   bounded gap **in the deciding bead** and files a `spike:` bead in the
   runner's lane — with no `--deps`, see below — dep-blocking this one the
   same way, only for a distinct dependency or deliverable and never as
   proof that research happened (ADR 0026 as amended by the operator ruling
   of 2026-09-05, ranger-base-k5fnr); `Done:` line; then
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
   (rangerhq-tqr). Sessions launch serially (create → await → claim →
   prompt); only the wait for the work overlaps, so a pass takes as long
   as its slowest bead, not the sum. Gathering itself is one goroutine per
   pending bead, fanned into `Run`'s own control goroutine as each settles
   — completion order, not launch order (a bead that settles in three
   minutes is judged in three minutes even when it launched behind one
   still running at seventy-five) — but still one judgement at a time on
   that one goroutine, so `gather` and everything it calls (`mergeBack`,
   `commitQueue`, `fileMergeBlocked`) run exactly as before; only which
   goroutine gets there first changed. Under `--watch` (`Dispatcher.Refill`,
   ADR 0028 §1 as amended) each settle re-runs the whole fire path
   immediately, right there, before the loop looks at anything else
   pending — a fresh `bd ready` scan under the launcher flock, offered to
   **every free seat**, not only the one that settled (ranger-base-t8tq:
   the settle is the level-trigger while a rolling `Run` holds `--watch`'s
   loop, and it runs the reap sweep too) — sharing the `Run`'s occupancy
   map (ADR 0028 §3: seats this `Run` fired into, released at their
   settle; what a fire pass merely *reads* about a seat expires with that
   pass) — so a `Run` under `--watch` keeps refilling for as long as there
   is ready work for any free seat.
   **The gather is bounded, and what is still in flight is carried**
   (ranger-base-3ryit, `internal/posse/passcarry.go`). "Only returns once
   the cascade quiets" is what this used to say and it was the defect: every
   refill's own prompt joins the set the pass is draining, so on a busy shop
   the set is fed faster than it empties and the pass never comes round —
   MEASURED 2026-09-04, 2h20m of `4 prompt(s) in flight, gathering` with no
   pass summary, while the merge-back sweep, the hook wall, the backup and
   guard tickers, the plan read, the epoch accounting and **any offer of
   ready work to a seat that freed with no settle behind it** simply did not
   run, and nothing said so. A pass now gathers for `GatherWindow` (the
   loop's base interval), judges what landed, and returns; legs still
   outstanding stay in flight and the next pass takes them back — the wait
   goroutines and their fan-in belong to the loop, not the pass, so nothing
   is judged twice or dropped. The occupancy map is carried with them
   (a carried leg's seat is still occupied), a pass that carries anything
   says so in one line naming the beads and their sessions, and the silence
   watchdog gained a second reading: no completed pass inside its budget is
   a finding, said once, naming the set holding it (`watchdog.go`).
   A one-shot
   `dispatch` (no `--watch`) never sets `Refill` and never refires: it
   fires once, gathers, and returns, exactly as before this ADR. `--watch`
   also wakes the next pass the moment a leg it was CARRYING lands, instead
   of waiting out the backoff (`internal/posse/watch.go`) — the completion
   of a wait this process already owns, never an event socket (ADR 0016),
   and never the refill's own mechanism.
   A refill **says whose seat it is refilling** (ranger-base-59jd): the fire
   path enumerates per bead, so on the first live refill, under `--persona
   gwart`, every settle reprinted a wall of `– <bead> … lane busy` lines and
   `– 131 ready bead(s) outside gwart's lane — skipped by --persona`,
   attributed to nothing — true lines, unowned, and on a loop they read as a
   rogue persona-filtered process holding the watch. A refill now prints
   `↻ refill for settled seat <seat> (<bead> settled) — re-offering N ready
   bead(s) to every free seat` before the enumeration and one
   `↻ … : N launched, M skipped (<counts by reason>)` after it, its per-bead
   skips counted rather than printed because under a rolling `Run` they are
   the same lines at every settle. Launches, `✗` errors and every other
   report are untouched, and a fire pass that is NOT a refill — the head of a
   `Run`, every one-shot `dispatch`, every `--dry-run` — still enumerates per
   bead (`internal/posse/refillreport.go`).

   **Which arm a measured idle-to-next window belongs to is read off the call
   path, not off a constant.** ADR 0028 §5 observable 1 shipped before the
   refill and stamped every line "no refill has shipped, this is the control
   arm" — a string that could not become false, so the before/after
   comparison it exists for would have read as all-before. A window is a
   treatment window when the launch that CLOSED it was made by a refill
   (`SeatRefill.Rolling`, set from the live call path): the line reads
   `[ADR 0028 §5 obs.1 rolling]` against `[… baseline]`, and the pass line
   counts them — `no window here was closed by a refill — control arm`, or
   `K of N window(s) closed by a refill — treatment arm`. A rolling `Run`'s
   first launch into each seat still comes from the head of its pass, so that
   window is a baseline one and is stamped as one; keying the arm on the
   `Refill` flag instead would put "before" data inside the "after" figure
   (`internal/posse/seatidle.go`, `docs/notes.d/ranger-base-59jd.md`).
6. Judge by the **bead**, not the agent: issue closed → ✓; agent settled
   `blocked` → ⛔ flagged (herdr's sidebar already shows it); settled but
   issue still open → ◑ review the session. **A close that leaves
   uncommitted paths in its session tree is written on the bead** —
   `closed dirty [N path(s)]: … — nothing carries these`, once, whichever of
   the judged close, the landing sweep or the kill's landing reads that tree
   first, plus one P1 back at the closer (ADR 0041, `closeddirty.go`). The
   pass line beside it is retrospective; the comment sits under the close
   comment, which is where the next reader copies a false claim from.
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
   `agent_not_ready`, `agent_blocked`) still does: those mean the prompt
   never took. Unclaiming live work re-dispatched it into a second fresh
   session and lost the assignee for `posse cost`/`scorecard`. herdr 0.8.2
   added the third code: `agent prompt` refuses an agent already sitting at
   an approval or question dialog and sends neither text nor Enter, where
   0.8.0 typed into it and the text was swallowed (rangerhq-ejf).

**The `await` in that sequence is the readiness gate**, and it waits for a
state herdr can *see* (`awaitSettled`, `internal/posse/dispatch.go`). herdr
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
launcher blocks, saying `⏳ launcher lock held by pid <n> — waiting` first.
The cockpit's writer is `io.Discard` — a line printed straight at a TUI is
garbage on the frame — so that one line comes back to it through
`Dispatcher.Progress`, a line callback `LaunchBead` hands the lock in place
of `Out`, and the cockpit puts it on the status line instead of sitting on
`dispatching <id>…` for the length of the other launcher's hold
(rangerhq-ecl2). `Progress` is `LaunchBead`'s alone: `Run` has a terminal
`Out` by construction. The cockpit's sink never blocks — it drops a line
rather than hold a launch on a busy event loop — and it does not clear
`dispatching`, because the launch it describes is still running. `--dry-run`
never takes the lock at all: a dry pass acts on nothing and a read-only
command must not queue behind a live one.

Why flock and never a second pidfile: a pidfile records liveness in a file
whose truth decays and the reader has to infer (rangerhq-ct9/ppy9); an flock
lives on the open file description, so **release *is* process death** —
crash, `kill -9`, closed pane alike — kernel-owned, with no staleness class
to detect and nothing to reap. The `pid:`/`since:`/`cmd:` the holder stamps
into the file is read for exactly one purpose, printing that waiting line;
nothing decides anything from it, and a dead or unreadable pid degrades to
"another launcher". The file is created and **never removed** — unlinking it
would let the next launcher lock a fresh inode: two holders, one path, no
error anywhere. `internal/posse/launchlock.go`.

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

Pins: `internal/posse/runrecord_qa_test.go`, and the pass↔pass repro in
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
  `internal/posse/planusage.go` are deliberately generic so the credential
  cannot ride out in one.
- **Where that credential comes from** (ADR 0019, ranger-base-x584) one
  seam — `ReadCredential(runtime, purpose)` in
  `internal/posse/credential.go` — is the only place posse acquires a
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
- **Refused by our own gate** (ranger-base-r64), fixed in two halves and
  both are in. On darwin posse reads that credential by running `security`,
  and a persona pane puts that persona's L1 shim dir first on PATH — so
  while the read resolved on PATH, any `posse` command typed inside a pane
  whose PID denies `Bash(security:*)` (every crew PID does) had its own read
  refused, the plan guard blind and the preflight silently UNKNOWN. Part A
  made the refusal a distinct error rather than "keychain item unreadable":
  the blind line and `plan-usage.log` name the deny rule, and the launch
  preflight — whose UNKNOWN branch is otherwise silent — says it once per
  process on stderr. That distinction was the urgent half: the two strings
  used to be identical, and on 2026-08-24 a refusal read as an outage and
  `plan_guard_blind_max: 0` was set for hours in response. **Part B
  (ranger-base-ypf5) removed the cause**: the adapter execs
  `/usr/bin/security` ABSOLUTELY, so the shim is no longer in front of
  posse's own monitoring read. The deny aims at what a *persona* may run,
  not at the harness — and an absolute path is the documented way past L1
  (see *fleet security posture*; L1 matches the typed word, and the wall for
  a read is L4, which posse does not run inside). It landed after the
  endpoint pin (ranger-base-17i) so that removing the accidental tripwire
  did not leave the exfil path silent. `GateRefusal` and its tests stay as
  the regression guard: if any posse read ever goes back to a bare command
  name, the refusal is still told from an outage instead of repeating
  08-24.
- **Above either threshold** the pass still runs. Each bead whose resolved
  runtime is on the guarded meter parks, with a line naming the window and
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
    brake (pass $X.XX/$Y.YY, day $X.XX/$Y.YY)`), then Dial E's ordinary
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
  - **Log noise**: the hourly quiet belongs to the fail-open note alone —
    said when the reading first fails, at most once an hour after that, and
    once more when a reading comes back. Past the budget nothing is quiet
    (rangerhq-llse): a degraded pass says so on the pass output every pass,
    and a parked one has no pass-level line only because each on-meter
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
  `$RHQ_HOME/state/plan-usage.log`
  (`<RFC3339> <caller> ok|429 cooldown=… retry-after=… streak=…`, trimmed to
  the newest 1000 lines) — cache hits write nothing, so that
  file's cadence is exactly what the endpoint sees of us. No lock: writers
  only ever replace one snapshot with a newer one, and last-writer-wins is
  right for a snapshot — a success landing after a 429 clears the cooldown,
  which is what a success means.
- **The Nth 429 in a row is not honoured like the first** (ranger-base-rwwp6,
  off spike ranger-base-dvxac). Honouring `Retry-After` verbatim and asking
  again at the boundary is a loop that need not terminate. On 2026-09-02 this
  instance drew **fourteen consecutive 429s between 03:30Z and 16:35Z**, each
  naming `Retry-After: 3600`, and three of the asks that drew one were made
  *after* the window the previous 429 had stated: by 29s, by 28s, by 118s.
  Read with ranger-base-au0o4 — which watched the window *end move* when it
  asked — the likely shape is that **every ask re-arms the hour**, so the
  poller kept its own window alive and the plan guard was blind for thirteen
  hours. (Not proven: the competing reading is a real window longer than the
  header it sends. The clean experiment needs the poller stopped. Both
  readings give the same instruction.) So the honoured wait now **doubles per
  consecutive 429 and resets on the first success** — 1h, 2h, 4h, 8h, six
  requests in a day where the old cadence made twenty-four. The first 429 is
  still honoured exactly (an isolated rate limit costs what it asked for);
  the escalation doubles the wait *in force*, not the header, so a 429 that
  names nothing mid-storm cannot walk the schedule back down. It does not
  lift the hour cap — no single 429 is believed past `planCooldownMax`, the
  ceiling is 8h (three asks a day, so the guard never "stops asking for a
  day"), and the wait is on the blind line
  (`… 10 consecutive 429, next ask in 3h12m`) and in the cadence log with the
  raw header beside it, because a mute nobody can see is the thing that cap
  exists to refuse.
- **A meter reader may not re-arm the window it is draining**
  (ranger-base-4rfw1). The escalation above only helps if the box actually
  goes quiet. On 2026-09-02 the operator commented out both thresholds and
  restarted the watch to drain a re-arming 429 window; the box was silent
  for **94 minutes** — and then `posse cockpit`, opened in a herdr pane,
  asked the endpoint itself at 20:13:57Z and again at 21:15:39Z from a
  second instance. Both drew `429 Retry-After: 3600` and re-armed the hour
  (`plan-usage.log`, caller `cockpit`). The dispatcher and `posse status`
  each checked the thresholds for themselves and stayed quiet; the cockpit
  and `posse cost` never had that line — and a rule every caller must
  remember is a rule the next caller forgets. So the refusal moved to the
  **choke point**: `PlanCache` is the one path to the endpoint, and a quiet
  cache does not ask, whoever is holding it. Two things make it quiet — the
  guard being OFF (no `plan_guard_<window>:`: nothing is deciding anything
  on the reading), and `plan_usage_quiet: true`, the flag the quiet gap
  actually needed, since commenting out the thresholds also switches off the
  brake. Quiet is **guard-off, not guard-blind**: no clock, no park, no
  degrade — a fleet parked on the operator's own decision is a brake with no
  release. The last reading is still served everywhere, always with its age
  (`5h 46% · 7d 29% · guard off, last reading 3h27m ago`), and only while it
  is inside `plan_usage_ttl:` for callers that go through `Read` — past that
  the cache refuses rather than hand back a stale number as a fresh one
  (ranger-base-c3vqe is what that costs), and a surface that wants the old
  reading anyway asks for it by name with `LastReading`.
- **An unarmed guard mutes the guard, not the meter** (ranger-base-ddivo).
  The rule above is right and its guard-OFF arm was half a sentence too
  wide. "Nobody armed the guard, so nobody needs the number" holds only
  while nothing is spending, and measured 2026-09-03 this shop was the other
  case: thresholds commented out since 09-01 under a full-speed ruling,
  `budget_pass:`/`budget_day:` set, `dispatch --watch` hiring — so every
  `PlanCache` on the box was quiet, the last reading stayed stamped
  2026-09-01T23:23, two cockpit opens made **no request**, and the operator
  found the weekly window exhausted by hand. ADR 0018's ledger brake knows
  dollars and knows nothing about that window (ranger-base-c3vqe,
  ranger-base-wkai3 carry the other half); this is the half that made the
  blindness **invisible**. So the guard-off arm now asks one more question —
  is anything spending? — and quiet needs BOTH: no `plan_guard_<window>:`
  **and** no `budget_pass:`/`budget_day:` written **and** no
  `dispatch --watch` loop holding `state/dispatch-watch.lock`. A cap that
  will not parse counts as written (a typo must not mute the shop's only
  meter) and a lock that cannot be read counts as running. The
  `plan_usage_quiet:` flag stays the only full mute, because that one is a
  ruling somebody typed. Nothing is decided on the reading either way — the pass keeps it
  warm on `plan_usage_ttl:` and rules on nothing — and the loudness follows
  for free: `plan_usage_stale_after:` was gated on the same quiet rule, so
  the stale line now fires in exactly the exposed shape, with its middle
  clause forked because no headroom rule is running there:
  `plan meter BLIND 46h00m: last reading 2026-09-01T23:23Z (5h 46% · 7d 29%)
  — the plan guard is UNARMED (budget_pass:/budget_day: is set), so nothing
  is ruling on it; no request has left this machine since`.
  *Amended 2026-09-03 (ranger-base-67mdf, from the verify batch
  ranger-base-s5j1t):* the first build asked the spending question three
  times per `PlanStaleness` call — once in the quiet gate, once again for
  the string the line prints, and a third time inside the `PlanCache` it
  then built (MEASURED: `PlanMeterSpender` ×3, `PlanMeterQuiet` ×2 per
  call, each a config re-read and, capless, a re-open of the watch lock).
  Two readings of a decaying state inside one call can disagree, and the
  disagreeing shape renders the exact sentence this entry exists to
  forbid. So the verdict is computed ONCE and carried: the quiet decision
  and the spender it turned on are one function, `PlanCache` carries both
  (it already carried `Quiet`), and `PlanStaleness` reads the cache it
  builds instead of asking again. Nothing about the rule above changed —
  only how many times it is asked.
- **A tripped guard parks the work that would spend the meter it read, and
  nothing else** (ADR 0010 §1, ADR 0013 §3). The guard's meter belongs to
  *one* provider, so a whole-pass skip had two costs: a lane whose runtime is
  not on that meter was skipped because somebody else's window was hot, and a
  pass that could still have run some of its work ran nothing. So the pass
  runs and the verdict is per bead, at launch:
  - **resolved runtime not on the guarded meter** — the built-in runtimes
    that are their own pools — **launch, ungated**. A template-only
    `runtimes/<name>.yaml` is *unknown*, and unknown is gated: "this runtime
    is free" is the expensive guess to get wrong.
  - **on the guarded meter → park**, on the guard's own line, per bead rather
    than per pass. Nothing is claimed.
  The runtime a launch actually gets is read from the session where one
  already exists, and `--runtime` pins the pass (ADR 0002's precedence), so
  the meter question is asked about the pool the launch would really spend.
  Dial E is untouched: it still resolves the tier, judged against that same
  pool.
- **No provider is chosen automatically** (ADR 0010 §1, executed
  ranger-base-6xx37). Until 2026-09-06 a tripped guard could MOVE an eligible
  bead onto a second pool named by `plan_guard_overflow:`, braked by
  `plan_guard_overflow_cap:` or the target's own meter and ledgered in
  `$StateDir/overflow.log`. That mechanism is gone: the two config keys, the
  PID `overflow: false` opt-out, the parity/tier eligibility ladder and the
  rolling ledger are all removed, and old `overflow.log` files are left in
  place unread. Where paid work should continue past a trip, an operator says
  so explicitly — `runtime:` on the PID, or `--runtime` on the pass — and a
  config still setting either removed key gets one stderr line per pass
  saying it is no longer read, rather than having it taken for a threshold on
  a window named "overflow". What did NOT go with it: the §5 guard table
  below, the local pool meters, Dial E's caps, and `uncounted.log`, whose
  ledger shape and helpers moved to `ledger.go` because ADR 0013 §5 still
  reads them.
- **A blind guard parks the meter it guards and nothing else** (ADR 0010 §5,
  ADR 0013 §3), the same shape as a trip and differing only in the line it
  prints. Per bead: off the guarded meter → launch; on-meter and blind →
  park without claiming; on-meter and over a threshold → park on the trip.
  The first good reading resumes on-meter service.
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
  (`internal/posse/credpin.go`, ranger-base-17i). Loopback buys the seam and
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
`internal/posse/budget.go`) are the dollar half of the same idea: where the
plan guard parks beads that spend the guarded rate windows, Dial E slows and
then stops dispatch on API-equivalent spend.

- **Config** `budget_pass:` / `budget_day:` in dollars (a leading `$` is
  allowed; the ADR's starting point is 25 / 100). **Both unset by default**,
  and then *nothing runs*: no transcript scan, no arithmetic — dormancy is
  exactly today's behaviour, and it is free. A value that is not a positive
  number is named on stderr and that window stays uncapped, the plan guard's
  rule for the same reason.
- **The windows.** `epoch` is the spend of the beads fired since the current
  wall-clock epoch opened (`dispatch_epoch:`, default 1h — ADR 0028 §2; it
  was *this pass* until then, and the config key is still `budget_pass:`) —
  a fired bead burns tokens while the next one launches, so it genuinely
  grows within an epoch and is not merely the last one's number. `day` is
  the local calendar day's bead spend, the same total the cockpit footer
  shows. Interactive sessions are in neither (Dial G: visible, never gated).
  One scan per bead feeds both.
- **The epoch is on the wall clock, not on the loop's.** Local midnight plus
  whole epochs, so a dispatch loop that dies and restarts mid-epoch measures
  against the window it was already in. A window that opened at `Run` start
  handed a fresh `budget_pass:` to every crash — spend authority created by
  a restart, which is the one thing ADR 0028 §2 exists to close. The spend
  itself is never stored for this: it is re-derived from the transcripts
  against the recomputed opening.
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
  there its cap was the only thing counting. Same rule `uncounted.log`
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
- **…but the caps do not license a blind pass with no headroom left**
  (`internal/posse/blindheadroom.go`, ranger-base-c3vqe). 2026-08-31: the
  meter credential went stale at 23:09, every read after it was a 401 or a
  429, and the fleet spent nineteen hours on the degraded arm because the
  instance's Dial E caps were armed and counting correctly the whole way
  down. They were counting DOLLARS. The account's weekly window climbed 89% → 96%
  behind a frozen snapshot and the operator caught it by hand with 4% left.
  A cap on one store is not a brake on another store's ceiling — ADR 0011's
  own diagnosis, "one store's momentary reading taken as evidence about
  another store's durable fact", one store further out. So the licence is
  asked of the METER, from the last reading it managed: over a
  `plan_guard_<window>:` threshold (a sighted pass would have skipped) or
  at/past the 80% braking rung, the pass parks with the caps armed. It
  invents nothing to do it — the reading is never aged, scaled by spend or
  extrapolated, which is the alternative ADR 0018 rejected by name; it is
  asked one question about the past ("was there room when the lights went
  out?") and both numbers are ones already in force on a sighted pass. **No
  reading at all is deliberately left on §1's arm**: that is the 2026-08-26
  shape, and parking it cost a measured hour of zero dispatch. Park on
  evidence, not on ignorance.
- **Display** `posse cost` ends with the caps in force and the day spend
  against them (or "no caps set … dormant"); the cockpit footer shows
  `today $… of $… budget_day (NN%)`, and the header's blind segment names
  which policy is waiting — `guard blind 14m` parks, `guard blind 14m —
  ledger brake` degrades, and `guard blind 14m — ledger unreadable, parked`
  is §3's third state: caps armed over a ledger the cost scan came back
  short of, where the brake counts nothing and the pass parks exactly as an
  unarmed Dial E would (the footer hedges the same scan's dollars with `≥`
  and `a floor, not a total`). The clause is the SCAN's fact, not the
  config's — reading the third state as the second is a stopped shop whose
  header says it is running under the ledger (ranger-base-3nvt). `guard
  blind 19h00m — no headroom at last reading, parked` is the FOURTH, the
  same rule once more: the ledger reads fine and the pass parks anyway,
  because the meter's last reading was already braking (ranger-base-c3vqe —
  this header said "ledger brake" for nineteen hours over a frozen 89%).
  Both read; only dispatch acts.

**verify-after** (ADR 0006 §3, `internal/posse/verifyafter.go`) is the one
handoff the harness files rather than a persona. Every dispatch pass — right
after the plan guard, before ready work is gathered, so what it files is
dispatched by the same pass — and every `posse ready` sweeps each `beads:`
repo for beads that closed since that repo's watermark and carry a label in
config `verify_labels:`. Each one without a `qa` dependent earns
`bd create "verify: <title>" -l qa -a <verify_assignee> --deps
discovered-from:<id>`, and `verify filed: <qid>` goes back as a comment on
the close (or one bead per N closes — `verify_batch:`, below). The verify
bead's description is what makes it workable: the closer, the
`close_reason`, the commits `git log --grep <id>` finds in that repo, whether
the session branches cut FOR that bead reached their base, and a pointer at
the closed bead's OWN description as the checklist. Acceptance is explicit
(ADR 0006 §4, simplified 2026-09-05): the verifier reads it off the bead, and
a close with no description gets an `acceptance: MISSING` line that tells them
to record the gap as this verification's limit. Nothing is inferred from the
closer's PID. What that replaced was a `## Intents` "done when" row picked by
word-matching the bead's labels and issue type, and the reason it went is
measured: across the whole queue on 2026-09-06 (2229 beads, 1048 closes
carrying a verify label, 2026-08-12 .. 2026-09-06) the 203 verify beads the
rule actually filed carried the row ZERO times, and re-run against the last
matcher, the 379 closes that would have earned one earn TWO distinct
sentences between them — the cell is a property of the closer and the bead's
type, never of the task (ranger-base-0ezn7).

  The landing block is beside the trail, never instead of it, because the
  trail cannot answer the question it looks like it answers
  (ranger-base-hl0sp): `--grep` names every commit in the checkout's
  ancestry whose MESSAGE mentions the id, so a commit that merely CITES a
  bead is indistinguishable there from the commit that shipped it, and a
  close whose own work never left its session branch reads as landed. The
  record that knows is `branch.<b>.posseBead` (`beadKey`, written at every
  launch into the tree), and `gitBranchLandingFor` asks it: for each branch
  cut for the bead, has its tip reached the base `branch.<b>.posseBase`
  names? A MISSING record says nothing either way and prints nothing —
  `git branch -d` takes the branch's config with it, so a session that
  landed and was tidied up looks exactly like one cut before the stamp
  existed. The instance: ranger-base-5jdzh's verify bead listed d309e2b,
  which is ranger-base-wd4be's commit naming 5jdzh, while 5jdzh's own
  411e54f sat on `posse/dinesh-posse-ranger-base-5jdzh` off main.

- **Config** `verify_labels:` (absent → `code, devops`; present but empty →
  off, and then bd is not even asked) and `verify_assignee:` (the persona
  verify beads are assigned to). The verify bead inherits the closed bead's priority — a P1 fix
  earns a P1 verify — and is filed with bd actor `posse`, so `created_by`
  distinguishes harness-filed beads from a persona's own.
- **Config `verify_batch: N`** (default 1 — the 1:1 gate, unchanged) files
  ONE verify bead per N closes: title `verify N closes: <ids>`, one
  description section per close (the same closer / `close_reason` / commits /
  acceptance-pointer block the 1:1 bead carries), a `discovered-from` edge per
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
  persona that settles open with nothing filed used to be re-prompted every
  pass; since ranger-base-9hm it is re-prompted **once**.
- **The second settle-open escalates instead of re-prompting**
  (ranger-base-9hm, `settleopen.go`). A bead that settles without closing
  gets a `settled open [<status>]: …` comment from `posse` and one more
  nudge. If the same bead settles open again under the same status, the
  agent believes it is done and the bead disagrees — a standing
  disagreement, not a lost prompt — so dispatch files ONE `-l question`
  bead for the operator (`settled open twice: <id> — …`, P1, `-a` the
  `operator:` key) and `bd dep add`s the stuck bead onto it. `bd ready` is
  what a pass selects from, so the edge is the stop. The escalation names
  the session, both halves of the disagreement, and **whatever is
  uncommitted in the session's tree** — the fact that made this urgent
  (ranger-base-1cc's 353 lines). Only under `--resume`, and never under
  `--dry-run`. Dedupe is the escalation's own title, not a second write:
  one open question bead per stuck bead, so bd's non-atomic create
  (ranger-base-muoo) cannot start a flood. An escalation the operator
  answers and closes re-arms the rung.
- **The fan-out cap is always present, and it is now per epoch.**
  `autostart_max_beads:` raises or lowers `-n`; it does not switch it on.
  Absent, the hook passes `-n 3` (rangerhq-v83) — an armed loop that fired
  the whole ready queue at once would be the worst case reached by omission,
  and at the measured (instance-side) median cost per dispatched bead,
  firing a 20-bead queue would eat a large fraction of a 5h window.
  `autostart_max_beads: 0` still means unbounded, for whoever wants it,
  explicitly. A value that is not a count is named on stderr and replaced
  with 3, because `-n` would otherwise parse it as 0. Since ADR 0028 §2 the
  cap bounds attempts per `dispatch_epoch:` (default 1h) rather than per
  pass: the passes inside one epoch share it, a pass that finds it spent
  says so and launches nothing, and the next epoch hands it back. Unlike
  the spend cap it is bounded PER PROCESS — a loop restarted mid-epoch
  starts counting again, because nothing outside the process can re-derive
  a launch count, and money is the bound that had to survive a restart.
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
- **Logs** `$RHQ_HOME/state/dispatch-watch.log`, written by the loop itself
  and rotated to `.1` past 5 MiB; the pane's own scrollback is the live view
  (`posse peek dispatch`). It used to be tee'd from the pane, which made the
  record a property of the command line somebody typed: a loop restarted by
  hand wrote to wherever that terminal pointed, and the fleet's log sat
  frozen at 2026-08-31 18:08 for three days with a live loop holding the
  lock and nothing red anywhere (ranger-base-n00wn). The loop now opens the
  file and tees its own stdout and stderr into it, so no pipe and no hand
  restart can end the record — and **nothing should tee into it**, or every
  line lands twice. `posse dispatch --watch-status` names the file and its
  age, which is both the operator's reading and the token
  `plugin/autostart.sh` matches on to decide whether the posse it is arming
  is old enough to still need the tee. The session wears 🛰️ and, being made by `posse new`,
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
under the guard.

`SocketID()` **resolves** the socket, it does not read `$HERDR_SOCKET_PATH`
(rangerhq-y4z). herdr injects the concrete path into every pane it opens, so
a session created inside one recorded `~/.config/herdr/herdr.sock` while
`posse` from a plain terminal read `""` — the same server, compared as two,
which cost the default board both halves of the rule at once: a genuinely
dead workspace was never pruned, every listing printed a refusal that was not
true, and the backfill could never stamp anything, because it will not stamp
a socket the pass cannot name. Unset now resolves to the path herdr itself
would use (`~/.config/herdr/herdr.sock`, or
`~/.config/herdr/sessions/<name>/herdr.sock` under `HERDR_SESSION`), which is
the same resolution `gen:` already stated for the socket it stats. So
`socket: ""` means one thing only: **written before the field existed**, by a
binary that named no server. That is what let the third arm move into
`cannotAnswerFor` where both halves ask it — the create refuses a pre-field
meta now instead of asking whatever herdr it happens to be pointed at
(rangerhq-jeu2's open board). The one arm still the prune's alone is the
empty listing (rangerhq-7dn4).

None of this is hypothetical — a `--watch` pane inherits the *herdr
server's* environment, not that of the command that created it, so a
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

The script needs a live herdr, so the suite cannot run the whole of it — but
one arm is checked on every box. `TestVerifyPruneGuardScriptPinsThreeFieldGen`
extracts the script's *own* `gen:` arm (its top-level `gen_label=` through the
`fi` in column 0) along with its own `meta_field` and `digits`, stubs `check`
so the verdict is machine-readable, and runs it over a planted meta: it must
accept what `genToken` emits and reject the pre-fjj two-field `dev:ino` shape,
a field that is empty or not digits, a fourth field, and a meta carrying no
`gen:` at all. That pin used to assert the arm's *grep literal* instead —
dcbbee8c rewrote the arms in bash parameter expansion, the literal went with
the grep, and a correct script left `go test ./internal/posse` red for
every seat until ranger-base-mg7si. A pin on a script asserts what the script
decides, not how it spells it; and when the extraction cannot find its
subject it FAILs, because a pin that has lost its subject measures nothing.

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
subset (`internal/posse/yamlflat.go`, ~100 lines, no deps):

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
tolerated, `#` comments skipped). They are one of **two reader classes**, and
the split between them is the trust model rather than a mechanism (instance
ADR 0019 D1, accepted 2026-08-28 — that page names *this* instance's
credential topology and so stays in the private tree, ADR 0012 D6; the public
half is `docs/adr/0019-credential-architecture.md`, the seam, and it is the
later page where the two overlap).

- **Session credential** — `envs/<set>.env`. Injected into exactly the
  sessions whose PID `envs:` names, plus whatever a recipe or `--env-file`
  adds explicitly. The persona **and every tool it runs** may hold it.
- **Harness credential** — `secrets/<name>.env`. Read by posse's own
  processes, injected into no session, listed by no command, and nameable by
  no PID key (`internal/posse/secrets.go`, rangerhq-5s5d).

The one-hand rule, one sentence because it has to survive being quoted:
**everything under `envs/` may reach a session; nothing under `secrets/` ever
does.**

That is an **injection** claim, not a confidentiality one, and pretending
otherwise is how a wall gets trusted that isn't there: 700/600 is
same-user-void — below the container tier every session runs as this uid and
can `cat` a 600 file it was never handed. What the split buys is scoped mint
and individual revocation, i.e. blast radius: a leaked session token burns one
token, not the account. The wall for *secrecy* is the container tier, never
the file mode and never the directory name.

`secrets/` is seeded **empty**, and empty is the shipped state: `posse init`
chmods it 700 and puts nothing in it (a seed tree that grew a credential file
would ship one inside the binary). In particular there is no `plan-guard.env`
— the plan guard is not a consumer, because P1 measured HTTP 403 for every
credential the operator *can* mint against the usage endpoint. The directory
is the class split made real, waiting for the first harness credential that
isn't the meter token (a second provider's meter, a webhook).

### The three paths, and who reads each

1. **The macOS keychain** — item `Claude Code-credentials`, read by execing
   `/usr/bin/security` **absolutely** (a PATH lookup here resolved to the
   calling persona's own `Bash(security:*)` shim and refused posse's own
   monitoring read — ranger-base-ypf5). It is the store of record for the
   meter credential on this host in the sense that matters: Claude Code's own
   interactive login/refresh loop writes it, so posse is the **second reader**
   of somebody else's rotating token and never a writer of it. The plan guard
   and `modelavail.go` both reach it the one way anything reaches a credential
   here — `ReadCredential(runtime, CredMeter)` in `internal/posse/credential.go`.
   Its ACL is **per binary**: every `make install` can silently drop this
   binary's read. MEASURED three times on 2026-08-24, once while the
   operator's own interactive shell read the same item fine.
2. **`envs/*.env`** — session credentials, the class above. `CLAUDE_CODE_OAUTH_TOKEN`
   in an env set is a *session* credential, and belongs in exactly one of them
   (D7, below).
3. **The runtime's own stores** — `~/.claude/.credentials.json`,
   `~/.codex/auth.json`, `~/.grok/auth.json`. What an interactive `/login`
   actually updates, and **neither class**: not brought under `envs/`, not
   brought under `secrets/` — an accepted risk that is named rather than
   fixed. On this macOS box the plan guard does not read them at all; the
   keychain is the meter store here, and making a file that kiz already
   measured as a stale leftover into the adapter would invert the store of
   record. Off darwin, where there is no keychain, `~/.claude/.credentials.json`
   **is** the meter store, fed through the same envelope parser so the shape
   diagnosis is written once and cannot fork (the seam picks by `runtime.GOOS`
   at run time, never a build tag). ranger-base-zzc (the file keeps coming
   back) and ranger-base-9fl (narrow `SeatbeltWritable` to the launching
   runtime) are the posture work on this path; the standing check is `make
   verify-credential-paths`, whose matcher is a **glob** because on 2026-08-23
   the file was renamed rather than removed and the check as worded passed
   sitting beside a live credential.

#### What the shipped artifact actually does (MEASURED 2026-09-01, 2.1.258)

ADR 0019 V1 wanted a clean room and a login. Half of it needs neither: the
credential store is *in the release binary*, and the binary is public and
checksummed. Recipe, no container and no login —

    curl -sS https://downloads.claude.ai/claude-code-releases/<ver>/manifest.json
    curl -sS https://downloads.claude.ai/claude-code-releases/<ver>/linux-x64/claude -o claude-linux-x64
    shasum -a 256 claude-linux-x64          # must equal platforms["linux-x64"].checksum

`strings` emits nothing useful on a 200 MB binary (`grep -ac` / `grep -aob`
instead — the same trap as the `ps` census); take byte offsets with `grep -aob`,
`dd` a window out, and `tr ';' '\n'` it into readable minified JS. The control
that says you read the shipped thing: the darwin-arm64 checksum in the manifest
equals `shasum -a 256` of `~/.local/share/claude/versions/<ver>` on this box.
It did (`b63136194160791c…`).

MEASURED on linux-x64 2.1.258 (`704f1334ac65d3e8…`, verified):

- The secure-storage module defines **one** store, `{name:"plaintext"}`, and the
  store selector returns it unconditionally. No keychain store object and no
  fallback composite exist in that module. So off darwin the runtime's own
  credentials file — path (3) above — is not a fallback; it is the whole store.
- The specialization is at **build time, not `process.platform`**: the linux
  build's read-error mapper passes the constant `"linux"` where the darwin build
  passes `"darwin"`. Do not reason about linux behaviour from the darwin binary;
  the bundles differ.
- Its **directory** is `$CLAUDE_SECURESTORAGE_CONFIG_DIR` when that variable is
  present (present-but-empty means `~/.claude`), else the config dir —
  `$CLAUDE_CONFIG_DIR`, else `~/.claude`; the basename is the one path (3)
  already names. So the directory is **not** `$HOME/.claude` by definition,
  which `CredentialsFile()` in `internal/posse/credential.go` assumed until
  ranger-base-wd4be, where it became `credentialDir()` — the shipped source is
  quoted in that function's doc comment, and the same resolution is now spelled
  in `scripts/verify-credential-paths.sh`, which scans the union of the three
  candidate directories rather than the resolver's winner. Both spellings of
  the config-dir half go through one function (`ClaudeConfigDirIn`, trust.go),
  because posse used to hold two answers and only the trust file's was right.
  Re-measured on darwin-arm64 2.1.258 at byte 158045445 while landing that
  bead: the darwin bundle resolves it identically, and the same statement
  builds the KEYCHAIN item's name, which is not the constant posse hardcodes
  once either variable is set (ranger-base-ig4op).
- The writer does `mkdir(dir)`, writes mode `384` (= 0600), then `chmod` 0600.
- The envelope the login/refresh loop writes is
  `{claudeAiOauth:{accessToken, refreshToken, expiresAt, refreshTokenExpiresAt,
  scopes, subscriptionType, rateLimitTier, clientId}}` — so ADR 0019 D5's
  ASSUMED `expiresAt` field name is confirmed present in the shipped envelope.

MEASURED on darwin-arm64 2.1.258, and it changes path (1) and path (3) above:
the darwin store is a **composite named `keychain-with-plaintext-fallback`**, not
a keychain.

- Its `read` returns the file's contents when the keychain read is null.
- Its `update` writes the file when a keychain write fails **non**-transiently,
  and then deletes the keychain item if one existed.
- On a later keychain success it deletes the file only when the keychain was
  previously *empty* — which is why the file appears and then sits frozen for
  days (the ranger-base-1lza observation), and why "delete once" measured out a
  treadmill.

So the darwin file is not "some auth flow's byproduct on its own schedule". It is
the runtime's own documented fallback, claude reads it, and a run of keychain
write failures can move the record onto it *and delete the keychain item*. Posse
declines to read it on the premise that reading it would invert the store of
record; measured, the inversion runs both ways, and the failure mode is posse
reading a keychain item claude has deleted while claude is authenticated fine.
That premise is ADR 0019 D2's and is filed back to the ADR (ranger-base-v3qi4),
not patched here.

What this does **not** settle: liveness. An artifact cannot say whether a token
written by a real login returns 200 from `/api/oauth/usage` — that is V1's other
half, it still needs a login, and it stays with the off-laptop cleanroom and the
spike that inherited it. `meterUnconfirmed` in `credential.go` stays true as
worded.

An operator who logs in has fixed (3) and believes he has fixed everything.
On 2026-08-24 all three failed in one day. Which one broke is a named class
now — `unreadable` / `401 stale` / `403 wrong kind` / `429` — carried in the
cockpit header and the dispatch skip line rather than left as archaeology in
`state/plan-usage.log` (rangerhq-pwpx). `docs/runbooks/credential-rotation.md`
is the operator's page for all four rotation moves; its front door is `posse
refresh` with no arguments.

### A settings file beats an exported variable, so the launch pins the credential dirs in argv (ranger-base-rq83c)

MEASURED 2026-09-03 on claude 2.1.259, against a scratch `HOME` and a
**fake** envelope — never the operator's tree, never a live token. The
readout is `claude auth status --json`, which runs with no login and no
network turn: `loggedIn` is true exactly when the resolved credential
directory holds an envelope, so it is a direct answer to "where does this
process think its store is".

| what set the config dirs | where the store resolved |
|---|---|
| nothing | `$HOME/.claude` |
| the process environment | where the environment said |
| `~/.claude/settings.json`'s `env` block | where the **file** said |
| both, disagreeing | where the **file** said |
| the file, against posse's `--settings` payload | where **posse** said |

Row four is the one that decides a design: the runtime `Object.assign`s each
settings scope's `env` over `process.env` at startup, so a launcher that
merely exports `CLAUDE_CONFIG_DIR` has already lost to any file that names
it. Row five is why the pin travels in `--settings`: the scopes are applied
in the order `userSettings, projectSettings, localSettings, flagSettings,
policySettings`, and `--settings` is `flagSettings` — after the user's file,
and in argv, which is not a file. Two higher scopes were measured and are
not available to a launcher: `--managed-settings` (the SDK parent tier)
drops `env` through a restrictive-only filter, and
`CLAUDE_CODE_MANAGED_SETTINGS_PATH` is inert in 2.1.259 — the resolver's
override hook is a stub returning undefined, so only the root-owned OS path
(`/Library/Application Support/ClaudeCode/managed-settings.json` on macOS,
`/etc/claude-code/` elsewhere) feeds the policy tier.

Four traps worth keeping:

- **A second `--settings` REPLACES the first.** It is not an additional
  source. So the pin merges into the one payload the launch already carries
  (`ClaudeFleetSettingsJSON`), and `EnsureSettingsPin` appends only to a line
  that has none.
- **The value to pin is not "the safe path", it is "what this box already
  resolves".** The keychain item's name carries a hash of the configured
  directory, so pinning `CLAUDE_CONFIG_DIR=$HOME/.claude` on a box that set
  neither variable RENAMES the item and strands the operator's login. The one
  spelling that keeps it unsuffixed is a present-but-EMPTY
  `CLAUDE_SECURESTORAGE_CONFIG_DIR`, which the runtime reads as
  `$HOME/.claude`. `credentialDirPin` carries the whole rule.
- **`projectsDirectory` in that JSON is not a readout of the config dir.**
  It reads `null` whenever a flag-scope settings source sets
  `CLAUDE_CONFIG_DIR`, whatever the directory resolves to, so a pin written
  against it measures the flag's presence rather than the directory. Only
  `loggedIn` says what it means.
- **A test that sandboxes `HOME` has not sandboxed anything that reads the
  config dir.** `ClaudeConfigDirIn` answers `$CLAUDE_CONFIG_DIR` first and
  only falls to `$HOME/.claude` when it is unset, and every posse-dispatched
  seat exports it — so a child launched with `append(os.Environ(), "HOME="+…)`
  reads the OPERATOR's tree on exactly the box where the suite runs, and
  nowhere else. It cost the cost pins 540x: measured one variable apart, the
  same `posse cost` run took 0.06s and found 0 rows without the inherited
  value and 33s and 29 live rows with it, and a mutant printed the operator's
  real per-bead attribution into test output (ranger-base-t7hgi). Any test
  sandbox that fences `HOME` for a config-dir reader must name
  `CLAUDE_CONFIG_DIR` beside it; `cmd/posse/costplan_test.go`'s `planEnvAt`
  is the shape, pinned with a fixture ledger and a control arm that finds it.

  **And the fence belongs to the test BINARY, not to the sandbox.** Every
  verb resolves the config dir before it runs one — `posse --help` does,
  through `NewHerdrBackend` → `ClaudeConfigFile` → `ClaudeConfigDirIn` — so
  the population is not "tests that fence `HOME`", it is every child this
  suite launches. MEASURED with a probe inside `ClaudeConfigDirIn` over a
  full `go test ./cmd/posse`: 155 resolutions, **126** to the operator's live
  `~/.claude`, from 48 tests across 15 files; the 29 that landed in a sandbox
  were `planEnvAt`'s. With the fence: 161 resolutions, **0**, every one of
  them under the test tempdir (same probe, `EXIT=0`, 274.8s, 2026-09-06), and
  the same subset run both ways one variable apart says it is the fence and
  not the day — `-run TestBackup` resolves 9 times either way, 0 on the
  operator's dir with the row and 9 without it.

  Nothing read operator data on those 126 — those verbs
  compute a path and stat two files — but the reader was one verb away:
  `posse runtime check claude` reads `~/.claude/.claude.json` through the
  same resolution and prints a different row for a trusted directory than for
  an unreadable one. So `cmd/posse/configdirfence_test.go`'s `TestMain`
  points `CLAUDE_CONFIG_DIR` at an empty temp dir for the whole test binary,
  which is one row instead of 18 and is inherited by a site added tomorrow;
  a test that WANTS the leak overrides it with `t.Setenv`, as costplan's
  wrong arm does. `TestClaudeConfigDirIsFenced` is the pin, two arms one
  variable apart on that verb — the fence, and a planted config dir it must
  report as trusted — because a right arm alone goes green when the reader
  dies (ranger-base-scts5, from -jxuiy).

The live pin is `internal/posse/credentialdirpin_live_test.go`
(`RHQ_LIVE_CLAUDE=1`), and its first arm is a control: it asserts the
redirect still HAPPENS without the pin, so the test cannot go green on a rig
where nothing could have moved.

### The session credential answers the model list, and a 429 from the meter endpoint names nothing (ranger-base-au0o4)

MEASURED 2026-09-02 against the live endpoints, from a `cage: seatbelt` session
on darwin-arm64, presenting the **session credential from an env set** — the
variable `CageCredential(claude)` names and a launch injects, not the meter
token. The instance-side account, with the timings and this box's log, is in
the private tree under the same bead id (ADR 0024 D3: the mechanism restated
here, the measurement cited).

`GET /v1/models?limit=100` on `api.anthropic.com`, with the header pair
`modelavail.go`'s `getPage` sends — `Authorization: Bearer`,
`anthropic-beta: oauth-2025-04-20`, `anthropic-version: 2023-06-01`:

- **200**, one page, and the newest model id is in it. So ADR 0039 D3d's
  premise holds: the availability probe can ride the credential the launch is
  about to hand the session, instead of the meter token.
- Three failing control arms are what make that 200 the credential's rather
  than the endpoint's good mood: **no** `Authorization` header → 401
  (`x-api-key header is required`); a synthetic bogus bearer → 401 (`OAuth
  access token is invalid.`); the *same real* token moved to `x-api-key` → 401
  (`API key is invalid.`). Bearer is load-bearing, and `getPage`'s comment that
  a Code OAuth token never goes on `x-api-key` is now measured.
- Header sensitivity, same token: dropping `anthropic-beta` still answers
  **200** — the oauth beta header is not required on this endpoint, unlike on
  the usage endpoint. Dropping `anthropic-version` answers **400**
  (`anthropic-version: header is required`). Keep sending both; one shape for
  both callers is deliberate, and only the version header is measured as
  required.

`GET /api/oauth/usage` with that same session credential is the other half of
the question, and it came back **429** — which is not an answer about the
credential, and this is the part worth carrying:

- A request carrying **no `Authorization` header at all** gets 429 as well. The
  rate limit sits in front of, or beside, the auth check, so a 429 there is not
  evidence about the token presented. Do not read one as an entitlement
  verdict.
- A synthetic bogus bearer, by contrast, gets **401** immediately — the rate
  limiter does not swallow it. That is the arm that discriminates: a credential
  that gets 429 rather than 401 is at least not an *invalid* token. Whether it
  is an *entitled* one — ADR 0019 D2's 403 half — that arm cannot say.
- Every ask re-arms the window, including a probe's. Before asking this
  endpoint by hand, read `$StateDir/plan-usage.log` (rangerhq-tdy8's shared
  reading exists for exactly this): a live cooldown there means the ask will
  429 and will push the window out another hour for every caller on the box.
  The cooldown is one window with a fixed end, not a fresh hour per request —
  `Retry-After` decrements by real elapsed time between two asks.

So the practical rule for anything metering-shaped: **401 and 403 are about the
credential, 429 is about the source.** posse's `planusage_anthropic.go` already
splits them that way — `AuthFailure` for the first two, `RateLimit` for the
third — and this measurement is why that split has to stay.

### The contract

- **Writer: the operator**, by `$EDITOR` or the TUI. posse never mints,
  refreshes or copies a rotating credential. The one credential *write* in the
  binary is `posse refresh`, and it is the operator's hand by construction: it
  refuses under `RHQ_PERSONA` and refuses without a terminal — the
  no-argument report included, because the deny line that spells the same gate
  does not know about arguments. A persona reading this files the ask; it does
  not work around those two refusals.
- **Readers: the named sessions — the persona and every tool it runs.** Treat
  every value in a set as something the persona may quote, log, or commit; the
  harness cannot tell "secret for the tool" from "secret the agent may read".
  So nothing goes in an env set that the session itself shouldn't hold, and
  persona sessions never receive config `default_env` implicitly: a persona
  gets exactly the sets its PID names in `envs:` plus whatever a recipe or
  `--env-file` adds explicitly (rangerhq-f2b). `default_env` applies to plain
  sessions only.
- **Names only, never values.** `posse envs` prints set names and KEY names;
  titles and listings show env-set names. Values go only into the workspace's
  environment (`--env` at creation) and are never echoed into a shell. A set
  name is a file stem, never a path — `envs: [../secrets/plan-guard]` is
  refused rather than resolved (`storeName`), which is where the two
  directories stay two.
- **Modes are re-asserted, not assumed.** `posse init` sets both dirs to 700
  and their files to 600, and every launch re-asserts **both** stores
  (`TightenEnvPerms` / `TightenSecretPerms`: a drifted 644 file is chmod'ed
  back and named on stderr — names only).
- **Never committed.** Where the instance home is itself a git repo
  (INSTALL.md §4), `envs/` is gitignored: 0700/0600 do not survive a commit,
  so the ignore *is* the control and a fresh clone gets its sets back from
  `posse init` rather than from git (rangerhq-lti6). `secrets/` is the same
  surface one class up and is in that block too, ahead of the first resident:
  the store is empty on every box today, so the ignore was written against a
  prospective leak rather than a live one (ranger-base-13h3).
- **Integrity is honest, not absolute.** The seatbelt grants `envs/` and
  `secrets/` to **no** session's writable set (`HomeConstitutionPaths`, ADR
  0015 §7 plus 0019 D1: a store a session may not be handed is not a store a
  session may edit). But that is a *tier* property — below L2 a persona with a
  Write tool edits `RHQ_HOME` files, and an L1 deny on the path is a tripwire,
  not a wall. The wall for env-set integrity is the seatbelt and container
  tiers.
- **Scoped by mint.** A token in an env set is minted for that set's purpose
  and revocable on its own without touching any other credential. That, not
  the file mode, is what makes the class split worth having.
- Known exposure, accepted for a local single-user tool: `--env KEY=VALUE` is
  argv to herdr for milliseconds (visible in `ps` on a multi-user box). herdr
  does not persist the values (audited: none in its session.json or logs).
- If a secret shouldn't sit in a plaintext file at all, put
  `op run --env-file=… -- <cmd>` (1Password) or similar in the recipe's
  `command:` and let the secret manager inject at process start.

### `make install` is the credential wall, and it is not temporary (ADR 0019 D3)

Personas write `internal/posse/`. **Any code path that reads `secrets/`,
`envs/`, or the keychain ships only through the operator's `make install`
review.** That promotion gate is the only wall between persona-authored code
and a credential store, and it is load-bearing for credential access
**indefinitely** — not a note until something better lands. Original D2 (mint
into `secrets/plan-guard.env`, delete the keychain read) would have retired
the per-binary ACL along with the keychain; P1's 403 closed that off-ramp, so
the ACL is now the long-term shape of isolation from persona-authored code.

The policy that follows, restated from the ranger-base-2zt refusal: **no
standing approvals on persona-writable files that touch a credential store.**
No persona improves credential handling ad hoc — changes here arrive as beads
on an accepted ADR, and they reach the fleet when the operator installs a
binary, not when a session writes a file.

What keeps that sentence true rather than merely written is
`credentialgate_qa_test.go`: the keychain read is exec'd from exactly one file
of the binary, and **no shipped script** (`scripts/`, `plugin/`, `etc/`, the
Makefile) execs it or reads a store's contents. A script would be a credential
path with no promotion gate in front of it — the fleet edits scripts, and
running one takes no install.

### `CLAUDE_CODE_OAUTH_TOKEN` belongs in `container.env`, not `default.env` (ADR 0019 D7)

Uncaged claude's store of record for session auth is the runtime's own — path
3, and the keychain Claude Code itself owns. The env var is a **derived copy
that silently wins and silently rots**: Claude Code warns on `/login` that a
set `CLAUDE_CODE_OAUTH_TOKEN` makes new sessions keep using the old token, and
every crew PID names `envs: [default]`, so a fresh operator login does not
reach the fleet. That is the shape that 401'd the fleet on 2026-08-22 and
again on 2026-08-24.

The cage is the exception and keeps the variable in `envs/container.env`:
inside a container there is no keychain, and the mounted claude file is a
stale leftover (rangerhq-kiz). A session that needs a token the runtime store
doesn't have names its **own** dedicated env set in its PID — never `default`.

Editing either file is the operator's hand (ranger-base-0vd standing order):
personas do not edit credential files, and D7's verification — `grep
CLAUDE_CODE_OAUTH_TOKEN` empty in `default.env`, still present in
`container.env` — is operator-run.

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

**Runtimes (ADR 0002 §1–2, `internal/posse/runtime.go`).** A persona
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
rangerhq-3mc). Claude matches a `Bash(...)` rule three ways — exact, `:*`
as a prefix of the argv **tokens** (not of the command string:
`Bash(sed -n:*)` does not reach `sed -ni 1p f.txt` and does reach
`sed -n -i.bak …`, measured on 2.1.241, ranger-base-g8e), and `*` as a
wildcard (`.*`, anchored, whitespace collapsed). A whole-verb deny
(`Bash(bd)`, which claude reads as *exact*) therefore ships `Bash(bd:*)`
alongside, so `bd show x` is refused too, and a negative rule — which the
dialect cannot say at all — ships as one exact spelling (below). `allow:`
is never widened (that would grant more than the PID says),
`RHQ_TOOLS_DENY` still carries the PID's own rules, and parity is
unchanged: L0 is politeness, never the wall.

A *subcommand* deny gets nothing but itself, and that is the end of a
two-step retreat. It shipped as an option-blind **pair** —
`Bash(git -* push)` for the bare spelling and `Bash(git -* push *)` for
the same verb with further arguments — because `git -C <repo> push`
matches neither the string nor the token prefix `git push`. `*` is `.*`,
so `-*` is a `-` and then *anything*, not an option run, and each half
proved it:

- **`Bash(git -* push)`** matched any `git -…` command whose last word was
  one of them — `git -C <r> log --grep push` and `git -C <r> stash push`
  refused, `--grep=push` running. Gone in **rangerhq-ky3**.
- **`Bash(git -* push *)`** matched any ` push ` token after a leading
  global option, subcommand or not — `git -C <r> stash push -m wip` and
  `git -C push status -s` (where `push` is `-C`'s value) refused on claude
  2.1.239 and grok 1.0.5, with `git stash push` (no leading option)
  running as the control. Gone in **rangerhq-vr6j**.

The L1 shim refuses none of those four: it skips the global options and
matches the first non-option token, so the wildcard half was blocking argv
the wall deliberately lets through. And nothing in the dialect separates
them from a real push — no negation, no way to say "option tokens only" —
so the false positives go and their coverage with them: `git <globals>
push …`, arguments or not, draws no polite refusal now, only L1's hard one
(`TestShimSkipsGlobalOptionsBeforeSubcommand` holds every spelling). The
same shape had already been dropped from the negative rule's widening for
want of a fallback half (ranger-base-xll2). Grok's realizer deliberately
does **not** widen at all: its dialect is verified (rangerhq-625) and the
wildcard is real, but grok matches a shell-parsed segment with the quotes
off, which turned the pair into a false-positive generator there — and
with the pair retired the reason is still live, because a rule with no
wildcard is a *prefix* on grok, so the negative rule's `Bash(git commit)`
would refuse the qualified `git commit -- <path>` the PID allows. See
*Grok specifics*. Templates:

```
claude: claude {model} --append-system-prompt "$(cat {file})" --add-dir {memory} {settings} {skills} {allow} {deny}
codex:  codex {model} {skills} {deny} -a never --disable hooks -c allow_login_shell=false -c "projects={\"$PWD\"={trust_level=\"trusted\"}}" -c developer_instructions="$(cat {file})"
grok:   grok {model} {skills} --permission-mode auto --rules="$(cat {file})" {allow} {deny}
```

**The dispatch contract (ADR 0013, `internal/posse/runtimecheck.go`).** A
runtime that *launches* safely is not the same claim as a runtime that can
*take work*, and one evening of two non-claude runtimes in production broke
the second claim about once an hour. So dispatch names six stages —

```
launch → promptable → work → record → settle → account
```

— of which four are observed (herdr, the bead, the cost adapter) and two
are **declared** per runtime, in the built-in table or in
`runtimes/<name>.yaml`. The settle stage has a declared half too
(`turn_outcome:`): herdr sees the pane settle either way, and only the
runtime's own record says whether a turn actually ran.

| key | values | what it says |
|---|---|---|
| `prompt:` | `typed` (default) / `argv` | how dispatch delivers the work prompt: type it into a promptable screen, or append the prompt file to the launch line so no screen is the delivery channel |
| `startup_wait:` | duration, default 45s | how long a launch may take to reach a promptable screen. Measured per runtime — 45s is a *claude* number |
| `record:` (+ `record_why:`) | `untrusted` (default) / `trusted` | whether a **dispatched** session of this runtime has been MEASURED to close its bead |
| `native_rules:` | file names | the rulebooks this CLI discovers by itself, ahead of anything posse types |
| `turn_outcome:` | a reader name (`claude-transcript`, `grok-session-store`), default none | whether posse can read what this runtime's own first turn DID — the fact that separates an exhausted account from an agent that worked and skipped the bead |

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

`turn_outcome:` is a **registry key, not prose**: the value names a reader
that exists in `internal/posse/turnfailure.go` (today `claude-transcript`
and `grok-session-store`), and a value no reader implements refuses at load for
the same reason `record: trused` does — a declaration that promises a
reading nothing performs is worse than no declaration.

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
| session failure — no agent, never promptable, unknown screen | this **pane**'s | **free once**; the next bead gets its own fresh session, and the pass's *second* such failure benches the slot (the ceiling below) |
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
not the pane's — see the ADR's alternatives.

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
(`internal/posse/recordskip_qa_test.go`.)

### The other half of that line: what posse cannot see (ADR 0013 §1 settle)

A settled pane is not a turn. Claude writes an allotment refusal as a
synthetic assistant message, so a pass that reads the transcript can stop
the bead with `⛔ … refused the first turn … no work ran` and tag the
session — but that read is a per-runtime **declaration** (`turn_outcome:`),
not a property of the name `claude`. Until `ranger-base-02zr` it was keyed
on the name, so the reader was never even asked on codex or grok, and an
exhausted account there printed as an ordinary settle-without-close.
MEASURED the same day: grok's account was answering `402 Payment Required`
while a pass called it a settle.

A runtime that declares no reader now says so on the bead's own line:

```
◑ a-1   settled "idle" but issue is "in_progress" — review ranger-posse-a-1
        (codex is record: untrusted — the claim is kept and --resume re-prompts it;
         posse reads no turn outcome on codex — an account that refused the turn
         settles exactly like this, so posse peek ranger-posse-a-1 before reading
         it as work that ran)
```

Both clauses can be true at once and they say different things: the first
is a **declared degrade** (nothing was lost, `--resume` retries), the
second is a **fact posse does not have**. Neither is a verdict — the two
causes are still one `posse peek` apart, and a harness that guessed here
would be guessing exactly where it just admitted it cannot see. The
per-pass half of the same honesty is the account-degraded report (ADR 0013
§5); this is its per-bead half.

grok's reader was built once the artifact behind it was captured
(`ranger-base-e123`'s probe, then `ranger-base-fc8go`): grok does NOT write
its refusal into a transcript — a refused turn leaves `chat_history.jsonl`
silent — so `turn_outcome: grok-session-store` reads the typed record in
`$GROK_HOME/sessions/<cwd>/<id>/updates.jsonl` instead
(`internal/posse/turnfailure_grok.go`). codex writes
`~/.codex/sessions/*.jsonl` (`ranger-base-xaev`) and is reachable the same
way, but its refusal artifact has never been captured — its account was
alive when the probe ran, and forcing one means spending until a quota
trips — so it declares none, and the order matters: a reader written over a
shape nobody measured is worse than the blindness it replaces, which is why
codex's line above is still the honest one. (`internal/posse/turnoutcome_qa_test.go`,
`internal/posse/turnoutcomegrok_qa_test.go`.)

Reading grok's record is also what made the ⛔ line's `no work ran` a lie
worth fixing (`ranger-base-qcu4c`). That phrase was written for claude,
where the refusal IS the whole turn — a synthetic message in place of a
first answer, nothing before it. On grok it is not always true: 1 of the 7
refusals in this box's history carries a full `usage` object, six model
calls and 190,817 tokens into a turn that had been running for ninety
seconds when the account went out from under it, and that session may well
have edited files and commented on the bead. The line has two arms now, and
which one prints is the runtime's own record's to say:

```
⛔ a-1   grok refused the turn mid-flight: API error (status 402 Payment
         Required): … — the turn had already run (6 model calls, 5571 output
         tokens), so work may exist: posse peek ranger-posse-a-1 and check
         the worktree before relaunching at another tier
```

The flat `refused the first turn … no work ran` stays for a refusal with
nothing behind it, which is claude's shape and 6 of grok's 7. Absence of
`usage` is read as "nothing ran" because grok writes one for every turn that
served anything (186/186 on this box, all nonzero in every field, censused
2026-09-05); a reader that cannot tell must say so, since the line reads a
zero as a claim and not as ignorance. The claude mirror was filed as open and
capture-shaped — a refusal landing *after* a first answer being invisible to
`FindClaudeTurnOutcome`, which stopped at the first assistant record — and it
turned out not to need a capture at all (`ranger-base-4ldma`). Censusing all
1755 claude transcripts on this box on 2026-09-05 found the artifact already
on disk, eleven times over: of the 13 allotment refusals inside a dispatch
turn, **11 land after real work** and only 2 are the first answer. Six
dispatched beads — `vtyst` (33 model calls, 24,740 output tokens before the
refusal), `frqmn` (27 / 18,417), `felmj` (27 / 20,230), `oujxl` (25 /
11,415), `2dzsm` (17 / 28,070), `pwtix` (15 / 6,250) — each settled with the
reader reporting a healthy turn, so each printed an ordinary
settle-without-close and cleared any marker a previous pass had set. The two
first-answer refusals are the records the reader was built from, which is how
their shape became the rule. It reads the whole turn now: the turn is closed
by the next work prompt or by end of file, never by the tool_result records
its own tool calls put in the user channel, and claude's work fields are the
transcript's `usage` objects deduped by message id — one model call arrives
as several records repeating one growing usage object, so summing the records
would report three calls where one ran
(`internal/posse/turnoutcomeclaude_qa_test.go`).

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
holds work. (`internal/posse/reapguard.go`, `reapguard_qa_test.go`.)

### What the auto-reap takes, and what it will never take (ranger-base-f6lk)

The end-of-pass sweep (`internal/posse/autoreap.go`) has three arms. All three
first ask the same two questions — herdr says nobody is working in there
(`idle`/`done`, or no agent at all past `RelaunchGrace`), and no launcher
prompted it inside `PromptGrace` — and then differ in what evidence they need
that the session is *finished*:

| arm | population | needs |
|---|---|---|
| pointer | dispatch's own per-bead session | its bead reads **closed** |
| crew | a crew mark on a session **dispatch made** | closed bead + `reap_crew_after:` (4h) + a tree holding nothing |
| unpointed | a per-bead-named session with **no** `bead:` pointer | `reap_unpointed_after:` (1h) + a tree holding nothing, and only at a sweep **past routing** |

Never taken, at any age: a conversation the operator MADE (`posse new` — the
crew arm takes only the name `SessionForBead` renders from the session's own
`agent:`/`dir:`/`bead:` record, so an operator-chosen name is out of reach);
`pulse_persona:`'s session (ADR 0027 has nowhere else to deliver); the
persona's reusable `<persona>-<repo>` slot; a foreign row. `off` / `never` on
either grace restores the permanent skip those two arms used to be.

**The unpointed arm waits for routing**, and that is the one place the two
widened arms differ in kind. Dispatch reaches a session by NAME —
`SessionForBead` for the bead it is about to work — pointer or no pointer, so
a stampless session at a live bead's Dial F name is a seat this pass is about
to relaunch into and reuse (rangerhq-vk2), not a dead shell. The pass-start
sweep cannot tell those apart; a sweep past routing does not have to, because
anything the pass used was either prompted (`promptedRecently`) or resumed
into, and a resume stamps the pointer (`NoteBead`) and takes the session out
of the population. The cost is that a pass which dies in gather sweeps no
stampless residue — but a quiet pass, which is the steady state this residue
accumulates across, reaches its epilogue in seconds. The crew arm keeps both
sites: its bead is closed, and a closed bead is never dispatched again.

**"A tree holding nothing" is `RemoveSessionTree`'s refusal asked as a
question** (`residueHolds`): no uncommitted paths in the session's cwd, and —
for a worktree session — no commits the base does not hold, measured by
patch-id AND by content (ranger-base-as19/x8jp; git's `-x` trailer is
somebody's decision, not a measurement). Every unanswerable question fails
closed. It is stricter than the kill that follows needs to be: a crew-arm
session's bead is closed, so the kill would land the branch itself, but a reap
that lands is a reap that decides. Deferring costs one pass —
`landClosedTrees` lands it at the head of the next, a DETACHED tree included
(ranger-base-vavx2: that sweep asked `<base>..<branch>` alone too, and skipped
such a tree in silence before its bead was read — `nothingToLand` asks both
tips now) — and the refusal prints
`◑ <session> idle <d> over <why> and NOT reaped: <what it holds>` **every
pass**, because the silence is what read as a broken reaper and cost the
hand-reaps in the first place (ranger-base-kftx).

**Asked of BOTH of a tree's tips** (`removalTips`, ranger-base-v2rj7): the
branch, which is what `branch -D` deletes, and the tree's own HEAD, which
is what `worktree remove` drops. On a **detached** HEAD those are different
commits — a commit made there writes no ref at all, which is exactly why a
container-tier session is launched detached (`PrepareSessionHead`,
ranger-base-t4f1) — so `<base>..<branch>` is ZERO over a tree holding a whole
session's work. Both guards asked only that, and MEASURED 2026-09-05 before
the fix: `RemoveSessionTree(t, false)` over a stamped, clean, detached tree
with one commit on it returned nil, removed the worktree, deleted the branch
and left the commit referenced by nothing. Not a live loss the day it was
found — the settle path runs `MergeSessionWork` first and that splices — but
a guard that holds on its caller's evidence is not a guard, and ADR 0058 makes
the sweep a second caller. The same wrong question is fixed at the listing and
the merge report in ranger-base-d8o6 (`treeState`, `landed`). Asking the head
is a no-op for a tree whose HEAD is on its own branch, so this is the detached
case and only it; the branch is still asked in its own right, because a branch
holding a commit its worktree walked away from is `branch -D`'s to lose.

The two graces are policy dials, not measurements of anything: nothing posse
records says how long a conversation's gaps are (typing in a pane leaves no
stamp — ADR 0008 §1 accepted that when it refused a timer), so the crew grace
is set long and the unpointed one, which protects much less, short. The age
itself is the later of `launched:` and `prompted:`; a record with neither has
no age, and no age is not old enough.

### The ownership refusal: a foreign row is not this home's to kill

The kill's second refusal, and a different question from the first
(rangerhq-selx). `posse kill <name>` resolves by *label*, and `Resolve`
falls through to **foreign** rows — live herdr workspaces this `RHQ_HOME`
holds no session meta for. That fallback is deliberate: `posse peek`,
focus and the listings exist to show the whole herd, including rows posse
did not make. Following it into a *destructive* path is the bug. Measured
across two homes on one herdr server: instance A's `posse kill m1-collide`
closed instance B's live workspace — exit 0, no warning, no ownership check
— while the *create* one command earlier had refused the same name
correctly. The resolver could always see the row was not A's; only the
destructive path declined to act on it.

So `posse kill` and the cockpit's `x` ask `ForeignKillRefusal` before the
close, and name the **workspace id** in the refusal — the id is what an
operator can carry to `herdr workspace list` or to the other home, because
the *name* is precisely what is not unique across instances:

```
NOT killed: dispatch is a foreign workspace (w7) — this posse home holds no
session meta for it, so it belongs to another instance or was made in herdr by
hand; close it where it lives, or `posse kill dispatch --foreign` to close it
from here anyway
```

Two flags, because they are two facts and reading one refusal is no
evidence about the other: `--force` says "I have looked at *my* session's
unfinished work" (the reap guard above), `--foreign` says "I mean the row
that is not this home's". A foreign row carries no meta and so never
reaches the reap guard at all, which is exactly why `--force` must not
carry it — a flag typed from habit about one's own dirty tree would
otherwise close another instance's live agent. The cockpit has no override
key; `x` on a foreign row refuses at the keypress rather than asking to
confirm a kill the backend will refuse anyway, and points at the CLI.

`plugin/autostart.sh` is the reachable caller: its `--startup` husk
replacement kills the autostart session by name, so on a shared server it
was one restart away from closing the other instance's dispatch loop. It
already ignores a failed kill and re-checks, so the refusal surfaces as
`<session> still present after kill — not started` — a hook that does not
start beats a hook that kills the wrong loop.

What no override can repair is the other side's bookkeeping: the owning
home's `state/herdr/<name>.yaml` still points at the workspace that was
closed, and that file is outside this home. Its own next listing prunes it
(ADR 0011 §2). One more reason the refusal is the default and the flag is
the exception. (`ForeignKillRefusal` in `internal/posse/herdrback.go`,
`internal/posse/foreignkill_qa_test.go`; the launch half is rangerhq-ynx8's
`foreignHeld`.)

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

**Skills (ADR 0007, `internal/posse/skills.go`).** `skills: [dataviz,
code-review]` on a PID binds those skills to the persona on whatever it
launches as — the cross-agent binding posse owns, as against the
per-user-per-runtime accident of "whatever this machine installed into
this CLI". A name resolves to `RHQ_HOME/skills/<name>/SKILL.md` or it is
unknown; that directory *is* the registry (real Agent-Skills dirs or
symlinks to `~/.claude/skills/x`, a plugin's `skills/x`, a repo — posse
indexes nothing and copies nothing INTO it, so `posse skills` is `ls` and
`posse agent check` is `stat`).

At launch the binding is materialized fresh, exactly as the gates are, in
one of two shapes — which one is a property of the *runtime*, not of the
PID:

- **A flag at a rendered tree (claude).** `RHQ_HOME/state/skills/<persona>/claude/`
  gets `.claude-plugin/plugin.json` plus `skills/<name>` — a real directory
  of files, COPIED out of the registry at every launch — and `{skills}`
  renders `--plugin-dir <that dir>` (session-only, additive, verified:
  `claude --plugin-dir <tree> plugin details posse-<persona>` lists the
  bound skills). `--add-dir` is CLAUDE.md dirs and does **not** load skills.
  A `runtimes/<name>.yaml` opts into the same shape with `skills_flag:` (a
  printf form, as `model_flag:` — `--foo` renders separated, `--foo=%s`
  glued) and is handed the same dir — the layout inside it is the universal
  Agent-Skills shape and the plugin.json is inert to anything that does not
  read it.
  **Copied, not symlinked, and that is the whole of what a second runtime
  needs from this shape** (ranger-base-65rc). The entries used to be
  symlinks into `RHQ_HOME/skills`, which made the "universal layout" promise
  a claim about the READER: grok 1.0.5 validated the tree, installed it, and
  reported `Skills (0)` — a `skills_flag:` runtime whose loader behaves that
  way launches clean, with `skills:` in Realized and the persona holding
  nothing, which is exactly the failure ADR 0007 §3 spends a refusal on,
  arriving through the accepted path. The same tree with real files reports
  both. claude dereferences, so the one CLI the surface had been exercised
  on was the one that hides it. The copy keeps each file's mode (a skill may
  ship a script), refuses a skill dir that links back into itself, and costs
  one directory copy per launch, beside the gates render.
- **No flag, symlinks in the session dir (codex, grok).** Both CLIs
  discover skills from their *working directory*, so the launch links
  `<session dir>/.agents/skills/<name>` → `RHQ_HOME/skills/<name>` and
  adds `/.agents/skills/` to the repo's `.git/info/exclude` — never the
  repo's own `.gitignore`, which is the operator's file. `{skills}` renders
  nothing there: the links *are* the realization
  (`Runtime.SkillsCwd`, `App.RenderAgentsSkills`). A `runtimes/<name>.yaml`
  declares this shape with `skills_cwd: true`; a profile naming both keys
  refuses, because a runtime has one skill surface and two half-bindings
  are not a binding (ADR 0012 D4). See **Skill surfaces** below for what
  else was tried.

An empty `skills:` renders nothing, placeholder and space alike.

**Declared means required.** `skills:` is a statement that the persona's
work depends on them, so it goes through the same parity gate as a wall
rule: a runtime with no surface adds `skills: <names> — <runtime> has no
per-session skill surface` to `Degraded` — today only a template-only
`runtimes/*.yaml` that names neither `skills_flag:` nor `skills_cwd:`, the
three built-ins all materialize — and the launch refuses unless
`--allow-degraded` (the session is then marked in meta and cockpit, like
any degraded launch). It is *not* filed under `Unrealized` — nothing is
being enforced here. A name that resolves to nothing refuses the launch
outright rather than binding a dangling symlink; `posse agent check` finds
it first, along with a PID whose own `command:` forgot `{skills}` while
`skills:` is non-empty (the `{model}` rule again: never leave a token
unrendered, never silently skip one) and a bound `SKILL.md` carrying no
`description:` — the third of the same kind, because codex drops such a
skill in silence and the persona launches believing it has one
(rangerhq-3zr; `App.SkillDescription`). Binding is additive — the
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
  frontmatter line is bound to nothing on codex (rangerhq-1qd). `posse
  agent check` reports it (rangerhq-3zr), and the E2E arm below re-measures
  it against the installed CLIs.
- **grok reads `<cwd>/.agents/skills/`** as `project` scope, and its skill
  discovery deliberately ignores git's ignore rules — so `.git/info/exclude`
  hides the dir from `git status` without hiding it from the CLI. (Both
  CLIs behave the same way about this; it is what makes the exclude honest
  rather than a trick.) `internal/posse/skills_e2e_test.go` re-runs the whole
check against the installed CLIs (`RHQ_E2E=1 go test ./internal/posse/ -run
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
  Read-only needs none: codex reads the whole disk regardless. **Every root
  is rendered resolved** (`codexWritableRoot`): codex refuses a writable root
  with a symlink *component*, and refuses it when a command runs rather than
  at launch, so a symlinked `personas/` dir made every dispatched codex
  session come up and then fail every tool call, silently — measured on
  codex-cli 0.150.1, `docs/notes.d/ranger-base-c02a.md`. **Every root but the
  one that cannot be resolved**: a DANGLING link (or a loop) is walked past
  and re-joined verbatim, so the rendered root keeps its symlink component —
  and the launch refuses on it rather than opening a session in which no
  command can run, because codex validates its roots before it applies the
  sandbox and one bad root refuses the whole set (`writableRootRefusal`,
  `docs/notes.d/ranger-base-k62e.md`).
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
  **both** `.claude/settings.json` and `.claude/settings.local.json` — one
  scope, two files, checked in that order (rangerhq-9u8) — with `hooks` and
  `mcpServers`: either top-level key in either file is a hit regardless of
  value, while a readable object carrying only `permissions` is clean. An existing keyed file that is unreadable,
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
  `Bash(git -* push *)` typed (the pair as it stood then; neither half is
  emitted any more — rangerhq-ky3, rangerhq-vr6j), all ten option
  spellings of a push —
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
  refuses every one of those spellings. Two of the false positives were
  *not* grok's: an unquoted trailing `push` word (`git -C <r> log --grep
  push`, rangerhq-ky3) and a ` push ` token that is not the subcommand
  (`git -C <r> stash push -m wip`, `git -C push status -s`,
  rangerhq-vr6j) were refused by the pair on claude too, which is why
  claude emits neither half any more. The quoted-`push` divergence in the
  bullet above is measured against that retired rule text, and it is what
  keeps a future widening from being shared; what keeps grok unwidened
  *today* is the bullet below it — a rule with no wildcard is a prefix
  here, so the negative rule's lone claude spelling, `Bash(git commit)`,
  would refuse the qualified `git commit -- <path>` the PID allows.
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
  means (a) the "send Esc and wait for idle" fix does not work, and ~~the
  launcher instead presses Esc once and prompts past *the same* screen if
  it is still reported (`clearStartupScreen` in dispatch.go)~~ **the
  launcher presses nothing at any screen — the special case was retired in
  rangerhq-6723: `clearStartupScreen`, `startupScreenDismissals` and
  `startupScreen` are gone, and `awaitAgent` waits for a settled state and
  refuses anything that is not `idle`/`done`
  (`docs/notes.d/rangerhq-6723.md`)**, and (b) the
  first-run state 37c measured is either already consumed on this machine
  or was the boot race all along — a prompt typed before grok's TUI took
  input, which fits the coordinator's incident too. Whether `startup_splash` should
  report `blocked` at all is rangerhq-1xsj (ops). Two lessons, both
  the same one: a screen that *looks* like it holds the keyboard is not
  evidence that it does — press a key and read the pane; and a detection
  rule's anchor must not include an optional element (37c's required the
  `[stable]` channel tag, which today's panes do not render, so the rule
  matched nothing at all until 7sbo widened it).

**Gates L1 (ADR 0002 §3, `internal/posse/gates.go`).** Native flags are
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
env — cooperative class at every tier (ADR 0025 §1): it cannot see
through `env -i` (nothing in-process can), `--no-verify` skips it
outright, and `-c core.hooksPath=` redirects past it with zero writes
(measured, ranger-base-3csb). At `cage: container` the push's *effect*
can still die at an enforced layer (mount `:ro` / egress proxy) where the
launch is configured for it (ADR 0025 §3); the verb gate itself stays
cooperative. Install: `posse gates install-hooks [repo]`
(replaces its own hook, refuses to overwrite a foreign one — chain by
hand). Persona launch reconciles the pre-push slot when the PID denies git
push and always reconciles `prepare-commit-msg`, then decides each slot on
**byte identity** against its own current render — that render, or the
prescribed chain dispatcher with `posse-<slot>` byte-equal to it (ADR 0023).
The file at the dispatch path is never exec'd; the behavioral half runs
posse's own render from a private temp file, so it catches a renderer
regression and nothing about what is planted. A foreign chain therefore does
not count however well it behaves, and a planted pass-through body does not
count even if it kept a marker: markers gate replacement, never realization.
Failure is `DEGRADED`, visible before herdr is touched, and L3 disappears
from concrete parity. The probe is launch-time
evidence, not a permanent lock: at `cage: shims` the session can still edit
the slot after the probe (the TOCTOU residual); the L2/L4 hook carve-out is
what removes that capability. Chain foreign slots per INSTALL.md §9.
`posse init` does not touch repos. And because a launch reconciles only the
repo its worktree was cut from, a hooked repo that never holds a session is
re-rendered by nothing — `SweepHookWall` asks the same identity question of
every `beads_visibility:` key from `posse promote`'s epilogue and the
`dispatch --watch` preamble, and names the ones to re-render by hand
(ranger-base-ixv4).

**Gates L3 on a managed hooks path (ADR 0052).** An employer-managed
`core.hooksPath` — absolute, outside every repo, and unwritable by this uid,
all three measured before any write — is not foreign, it is a wall posse does
not touch: `install-hooks`, session create and the hook-wall sweep write
nothing there and print one line saying so. L3 is realized instead by a
per-session hooks dir, `state/hooks/<session>/`, that the session env aims
git at through `GIT_CONFIG_COUNT`/`_KEY_n`/`_VALUE_n` naming `core.hooksPath`:
posse's members plus one dispatcher for every executable in the managed dir
(the union, because a slot the redirect dir lacks is an employer hook git
skips), ours first with its exit final, then the employer's with git's own
argv and stdin. The identity probe moves to that dir and gains a
forward-completeness arm; parity says so (`session hooks dir, redirected by
env; managed hooks <dir> run after ours`); meta records `hooks_mode:
redirect`. Env-borne, so the same class as the rest of L3: survives an
absolute-path git, shed by `env -i`, which leaves the employer's hooks
running alone — nothing a persona or posse does weakens the employer's
control. The staleness sweep and `scripts/verify-hook-freshness.sh` classify
each configured repo before reading it and skip a managed one by name: the
box is CLEAN, not unmeasured, because nothing of posse's is installed there
to go stale. The script's reference render — a real `install-hooks` into a
throwaway repo — is taken with that same config-in-env redirect aimed at a
scratch dir of its own, and refuses to measure unless git confirms the
redirect took; without it the throwaway repo inherits the managed global and
the whole control goes dark (ranger-base-1se2l). `posse gates managed-hooks
[dir]` is the read-only form of the classification, exit 0 managed / 1 not.
Recipe and the two residuals: INSTALL.md §9, "A managed hooks path".

**Tiers (ADR 0003 §1–2).** A tier is a name — `strong` / `standard` /
`fast` — mapped to a model per runtime in the built-in table: claude
`claude-fable-5-1` / `claude-opus-5` / `claude-sonnet-5`; codex
`gpt-5.6-sol` / `gpt-5.6-sol` / `gpt-5.6-luna`; grok `grok-4.6` /
`grok-4.6` / `grok-4.5` (`fast` falls back to `standard` when only that is
mapped). Codex
maps `strong` and `standard` to the same id on purpose: sol is what a
codex session here defaults to and codex offers nothing above it, so
naming it makes the launch a fact rather than a CLI default that can move
between releases, while `fast` = luna is the **cost** lever only —
MEASURED 2026-08-25, switching to luna did not lift an account-level usage
wall, because the wall is on the account and not on the model
(ranger-base-arm). Until that map existed `tier:` was inert on codex: no
`Models` at all, `{model}` empty, no warning. **grok had the same defect
until rangerhq-jp6** — its template already carried `{model}` and
`ModelFlag: -m %s`, so only the map was missing — and it is filled the
same shape and for the same reason: grok-4.6 is what a grok session here
defaults to (`grok models`, 1.0.5, 2026-08-29: "Default model: grok-4.6")
and grok offers nothing above it, so `strong` and `standard` both name it.
`fast` = grok-4.5 is a **capability** step-down and, unlike codex's, NOT a
measured cost one: xAI publishes no per-model rate against the weekly pool
(the reason grokpool.go estimates the meter at all), and grok-4.5 has
never run on this box — 181 of 181 priced turns across 174 transcripts in
`~/.grok/sessions` carry `"modelId":"grok-4.6"` — so "4.5 is cheaper" is
not a small number, it is **no number**, and nothing may read a saving
into that row. `fast` is named explicitly rather than left to the
fallback for the reason the fallback would defeat: a `fast` rendering the
same id as `standard` leaves dispatch's budget step-down buying nothing.
`--reasoning-effort` (alias `--effort`) is arguably the bigger spend dial
on grok and is deliberately NOT in the map — **ruled out**, not deferred
(ranger-base-tg7c, ADR 0003 §1 amendment 2026-08-29): nothing here can
price an effort step against the weekly pool, and the two models do not
offer the same efforts — grok-4.6 has xhigh/high/medium/low, grok-4.5 only
high/medium/low, both defaulting to `high` (measured). So a per-tier
effort is not one key but a tier→model × model→efforts validity matrix
plus a second placeholder, and an unrendered placeholder is a literal
argv. A PID or declared runtime that wants one appends
`--reasoning-effort` to its own `command:` today. The same ruling ratified
the SHAPE of the column over the map rangerhq-jp6 originally asked for
(`standard` = grok-4.5).
A runtime that maps nothing
now says so where it is read — `posse runtimes` prints `tiers: UNMAPPED —
ignores tier:` and `posse runtime check <name>` names the tiers that
render nothing, both off one rendering (`Runtime.TierMap`). No built-in
reads `UNMAPPED` any more, so that rendering is now about a declared
`runtimes/<name>.yaml` with no `model_<tier>:`, a partial map, or a
runtime name posse has never heard of — and its pins are fixtured on a
declared runtime for exactly that reason. A
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
The claude ids above are a restatement of `claudeModels`, and the
fallback line quoted below is a quotation of what the preflight prints:
`internal/posse/notestier_qa_test.go` holds both against the code, in
BOTH directions, after three days in which this paragraph named a
superseded strong id that ADR 0003 had already flipped. ADR 0003 §1 no
longer names an id at all — "the current built-in model ids and exact
price rows live in runtime.go/cost.go" — so this paragraph is now the
ONLY prose in the tree restating that table, and a pin is the only thing
that can keep it true (ranger-base-1kvfr).

**Tier availability preflight (rangerhq-oay).** A tier is a name and the
launch turns it into a model id — but until this landed, nothing asked
whether the account can run that id. It came from a real morning: access
to the strongest model disappeared from the operator's own session, and a
persona resolving `tier: strong` would have gone on launching while the
CLI quietly served something else, with `posse cost` filing the spend
under whatever tier the substitute belongs to and no line anywhere saying
why. So `planLaunch` now checks, once per launch, on the pair it has just
resolved: `App.TierPreflightFrom` (modelavail.go — `TierPreflight` is the
same check for a caller that names no env sets), before the parity check.
Unavailable prints one line — `richard: tier strong wants claude-fable-5-1
— unavailable on this account; launching as asked, and only an explicit
--runtime/--tier/--model or a PID change moves it` — and that line is the
whole of what it does. `posse cost` needed no change and that is the point:
`TierForModel` reads the model out of the transcript, so the spend was
always counted honestly — what was missing was anyone knowing.

**What it used to do, and no longer does (ADR 0003 §3, ranger-base-hv2zr).**
Until 2026-09-06 an unavailable model was SUBSTITUTED: the launch walked
config `tier_fallback:` (default `strong` → `standard`), opened on whatever
it landed on, wrote a `fallback:` mark into the session meta, and `posse
list`, the cockpit, the relaunch receipt and dispatch all read that mark
back. Dial H is struck: availability is advisory, the runtime may refuse an
unavailable choice on its own, and choosing another pair is the operator's.
The measurement behind the removal is on the bead — over 2026-08-25 →
2026-09-06 (509 catalog reads, 137 of them successful) the mechanism
performed no substitution at all, and the 336 lines mentioning 401 are
UNKNOWN reads, which by construction never substituted anything. What
replaced the mark is a comparison: dispatch's `effectiveTier` reads the
meta's own `runtime:`/`tier:` against the pair the bead resolved, so a
session that opened on another pair — an operator's explicit `--tier`, or a
session created before the removal — still gets a work prompt naming what
it really runs.

The probe is `GET api.anthropic.com/v1/models`, zero tokens, shared through
`$RHQ_HOME/state/model-catalog.json` behind `model_probe_ttl:` (default
1h) exactly as `plan-usage.json` is — a successful reading is reused for
the TTL, and rate-limit cooldowns are shared across processes. Other failed
attempts remain UNKNOWN and may be retried by the next launch.

WHICH CREDENTIAL it presents changed twice on 2026-09-05 and the sentence
above used to answer "the same one the plan guard reads". It now PREFERS the
session mint of the env sets the launch being judged realizes — read out of
`envs/*.env` under the home by the ADR 0019 seam, selected by that launch's
own set list and taking the last assignment of the name across it (ADR 0039
D3d as amended, ranger-base-q3n4e) — because the meter credential rots in
hours and had left this probe failing since 2026-08-31. The plan guard's
credential is the FALLBACK, for the two ways the preference does not answer:
none of the named sets carries the variable (no request is spent), or the
endpoint refuses the one that did (exactly one more read per catalog read,
never a loop). A read no launch asked for — `posse runtimes`, `posse gates` —
uses the persona-less list, which is config `default_env` and nothing else; a
persona that names no env set gets no session credential rather than the
cockpit's, because an env set is an explicit choice and never a silent
default (rangerhq-f2b). Verified live
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
stderr rather than only in the log. Since ranger-base-ypf5 it is not refused
at all: the read execs `/usr/bin/security` absolutely, so a launch from a
gated pane probes normally.

Every cache miss that attempts a probe appends a generic outcome to
`$RHQ_HOME/state/model-catalog.log` (`ok models=N`, HTTP failure, empty
catalog, or cooldown); cache hits append nothing, and the log is bounded on
the same policy as `plan-usage.log`. It never records the credential or a
header. This is the evidence for UNKNOWN: a launch still fails open, but a
missing `model-catalog.json` no longer leaves "model available" and "probe
could not authenticate" observationally identical.

Three rules make it safe to leave on by default. **It fails open in one
direction only**: a catalog that was actually read and does not contain
the model is the ONLY thing that reaches a verdict at all — an unreadable
credential, an unreachable endpoint, a 429, an empty answer or a runtime
with no model mapping are all *unknown*, and unknown launches exactly what
it was asked to launch without a launch warning (the request outcome remains
in `model-catalog.log`). A preflight that guessed "unavailable"
would once have silently downgraded the whole shop; since dial H went it
would only shout, which is milder and still wrong. **It never refuses**:
rule (3) from the operator — "a degraded model is worse than nothing" is
their judgement, and the place they record it in advance is `tier_floor:`,
which still bites, now on the asked-for pair, because that is the only pair
a launch can open on. **And it never chooses**: `tier_fallback:` is gone
(ADR 0003 §3) and a config that still carries the key is inert — no walk,
no default `strong` → `standard`, no hops, no marks. `model_preflight:
false` turns the whole check off; `posse gates <persona>` prints the verdict
per runtime, which is how you tell "the strong model is gone" from "the
probe never answers on this box" without launching anything.

Two consequences worth knowing. A session's meta records the pair it
*actually* launched at, the way `cage:` records the cage it got — so
`posse relaunch` and `RelaunchAgent` replay THAT pair rather than
re-resolving it, which is what keeps a session the operator opened at
another tier where they put it. And the test seam is `App.ModelLister` (nil =
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
⛔ line, marks `posse list` with `🛑turn-failed`, and renders the
cockpit row red as `failed` instead of healthy `idle`. That line says "no
work ran" only where the runtime's own record says so — on a refusal that
landed mid-turn it names what had already run and sends the operator to the
worktree first (`ranger-base-qcu4c`, §"The other half of that line"). It does not guess a
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

The scorecard also prints the **harness-upkeep ratio** (rangerhq-ndi):
DIRECTION.md's caution that Gas Town died of harness self-refinement budgets
~20-25% of all work at harness upkeep, and this makes the number a fact
rather than a feeling — closed beads, harness vs everything else, over 7d
and 30d, per persona and total. **What counts as "harness"**: a bead whose
own id carries the same prefix as this bead's own id above (rangerhq-ndi)
— the harness's bd project, unchanged since this repo's prior name, so
issues filed against posse's own code and process still carry that prefix
regardless of the directory's current name. Everything else counts as
product/ops work, which is the bucket the budget is measured against. The
classifier reads the id (`IsHarnessBead`, scorecard.go), never the
configured `beads:` dir: a `.beads/redirect` can serve several repos'
issues out of one shared store (ADR 0015 §4) — this instance's own
`beads:` list holds a single entry whose redirect chain lands on the
shared queue db, and that db mixes the harness prefix and `ranger-base-`
ids — so the dir is not a repo boundary and the id prefix is the only
fact bd hands back that is. `IsHarnessBead` costs
nothing extra to compute: the ratio buckets the same `bd list --all --json`
rows the rest of the card already scanned, across the same repos and with
the same "scored N of M" caveat when one fails to read. In a single-repo
instance whose one bd project IS the harness, the ratio reads a trivial
100% — the number only means something once an instance's aggregation
spans a harness project and something else.

`posse cost [--since <date>] [--project <substr>] | --plan` is ADR 0003 §4's
accounting: the analyst's `bead-cost.py` method in Go. Every runtime with a
**cost adapter** (ADR 0012 D4, `internal/posse/costseam.go`) is segmented by the
dispatcher's "Work beads issue <id>" prompts; three ship, and a runtime with
no adapter is reported as *uncounted*, never $0.

- **claude** — `$CLAUDE_CONFIG_DIR/projects/*/*.jsonl`, `~/.claude`'s when the
  override is unset — the same config dir the trust file, `history.jsonl` and
  the credentials file resolve, because a walk rooted at `~/.claude`
  regardless (which this was until `ranger-base-yqdov`) lands on an absent
  root under an override, and an absent root is *never ran the CLI*: $0 with
  no error and no uncounted line, on the one runtime that carries dollars.
  Measured on 2.1.261 before the walk moved — the shipped bundle joins
  `projects` onto its config home in three places, and a headless run with
  `$HOME` moved wrote its transcript under the config dir, not the moved
  home. `FindClaudeTurnOutcome` reads the same locator, so it followed the
  override in the same commit. Assistant records are deduped by message id
  (streamed chunks repeat it — max per usage field) and priced
  at list rates per MTok for the model each record names (fable 10/50, opus
  5/25, sonnet 3/15, haiku 1/5; cache write 1.25× input for 5m TTL and 2× for
  1h when the breakdown is present, else 1.25× flat as the script did; cache
  read 0.1×). An id matching no family is **unpriced, not guessed**: the
  report says the total is a floor rather than putting an invented number in
  the same column as real money.
- **grok** — `$GROK_HOME/sessions/<url-encoded cwd>/<uuid>/updates.jsonl`,
  `~/.grok` when the override is unset — the same home the version probe and
  the turn-outcome reader resolve, because a walk rooted at `~/.grok`
  regardless (which this was until `ranger-base-z65xu`) lands on an absent
  root under an override, and an absent root is *never ran grok*: $0 with no
  error and no uncounted line. grok reports its own dollars (`costUsdTicks`,
  nano-dollars) per turn, so there is no rate card to keep current; the `modelUsage` breakdown restates the same
  spend and is deliberately not read (reading both is exactly 2×).
- **codex** — `$CODEX_HOME/sessions/YYYY/MM/DD/rollout-*.jsonl` (`~/.codex`
  unset, same rule and same fix as grok's above), decoded from the
  `token_count` events' **cumulative** `total_token_usage` by charging each
  segment the delta since the previous snapshot. Not by summing
  `last_token_usage`: codex re-emits a token_count with an identical snapshot
  a fraction of a second later, so summing the per-turn field reports ~2× —
  measured 2026-08-28 over the whole local history, 100 of 163 rollouts carry
  such duplicates and 15 of those were written by 0.147.0, the version running
  here, so it is not a version to wait out. Not by maxing the snapshot either:
  that is dedupe-proof but cannot attribute, so a session working two beads
  would charge the second one the first one's spend. Delta-from-cumulative is
  both; verified on all 159 local rollouts that carry token counts, the deltas
  sum to the file's final `total_token_usage` on 159 of 159 with no snapshot
  ever going backwards. Two field facts the shape depends on: `input_tokens`
  INCLUDES `cached_input_tokens`, and `reasoning_output_tokens` is a SUBSET of
  `output_tokens`.

**What is priced and what is not.** claude beads carry API-equivalent dollars.
grok beads carry the provider's own dollars. **codex beads carry no dollars at
all** and print a `—` in the `api$` column with a legend saying why: codex runs
on the operator's ChatGPT subscription, which reports no per-turn cost, and no
list rate applies to a plan seat. Their turns and tokens ARE the measurement.
A blank is deliberate — an invented figure at another provider's rates would
land in the same total as real money with nothing marking which dollars were
guessed, and would then move a budget window (ADR 0003 Dial E) on a number
nobody measured. Unpriced rows are excluded from the summary statistics and
from every group sum, and each group says so — a mixed group appends
`(3 of 12 unpriced — sum is a floor, median and per-bead are over the 9 priced)`,
and a group where `no bead here has a rate` prints a bare `—` for its sum.

Output: per bead (start, persona from the bead's assignee, **runtime**, tier
from the model that did the work, turns, tokens, api$), then by runtime /
tier / persona / day with median and per-bead, the interactive total (never
gated, shown for the ratio, and it names its own unpriced turns), and the
honest gaps — per pass is not attributable until dispatch records a pass id
(rangerhq-25p). The runtime column is what makes a mixed day readable: two
beads with the same tier and persona can have come out of two different pools,
and only one of them has a dollar figure. **`strong`/`standard` are `?` on
codex and grok**, and that is not an oversight: tier is re-derived from the
model id that did the work by `TierForModel`, and codex and grok both name
one id on two tiers (`gpt-5.6-sol` for codex strong and standard, `grok-4.6`
for grok's since rangerhq-jp6) — so the id does not identify a tier there
and the report says `?` rather than picking one, whichever tier a map
iteration happens to hit (ranger-base-3st5). `fast` DOES resolve on both:
`gpt-5.6-luna` and `grok-4.5` are each named by exactly one tier in their
runtime's built-in map, so a fast-tier turn on either identifies its own
tier unambiguously, same as claude. The rule stays symmetric across all
three runtimes — only an id ONE tier names may resolve — so a future
runtime whose ids collide the other way (`fast` shared, `strong`/`standard`
distinct) reports the shared one as `?` too. The cockpit shows
each per-bead session's running cost and the day total in the footer (rescanned
every 30s off the event loop).

**codex has a plan meter, and it is a hint, not a guard.** Every codex
`token_count` event also carries `payload.rate_limits`: `limit_id`,
`plan_type`, `primary`/`secondary` each `{used_percent, window_minutes,
resets_at}`, and `credits {has_credits, unlimited, balance}` plus
`spend_control_reached` — the same reading `planusage.go` gets from Claude's
endpoint, on disk, no network and no keychain. rangerhq-0va item 4 asked for
it in the cockpit header beside Claude's; **ADR 0034** decided the shape, and
it is not a second guard: the plan-window seam stays singular and codex enters
as a typed `PlanHint` that is DISPLAY only — D4, the decision that would
have let it refuse anything, is withdrawn, and every launch/brake decision
belongs to ADR 0010 —
because the reading is a snapshot outside its store of record whose staleness
is unbounded in the dangerous direction — the pool is account-wide, the
rollouts are box-local, so codex on another device drains it without this
file moving. Windows are named by duration (`codex_5h`, `codex_7d`), never by
slot: primary was the 5h window Jan–Jun 2026 and the weekly one in Aug, so a
slot-named threshold changes meaning under you — and `plan_type` moves too
(team → plus). Implementation is ranger-base-xb5f (the reader) and -ormb
(display, always with the reading's age); -3o10, the overflow advisory, went
with D4 and with the mechanism it advised.

The metric `cost-per-closed-bead` has a scorecard answerer for
h2c — `posse cost` by bead id against closes — so a PID that declares it
reads as `computed`. `--plan` skips all of the above and prints only the
plan's own rate windows (the plan-guard section has the reading); it takes
no other flags, because there is nothing for a date or a project to select.

`posse agent new <name>` scaffolds the PID shape — every frontmatter key
present (lists empty and commented, with one exception: `deny:` ships
`Bash(git commit unless --)` as a real entry, because a scaffolded persona
with no deny at all got the commit wall's L3 half and none of its L1 half,
ranger-base-09b7), every body heading in contract order with a one-line
hint, the four hard risk lines verbatim (`HardRiskLines`) — and opens it in
$EDITOR; `posse agent edit <name>`
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
CLI). It is materialized at launch, seeded with `ORDERS.md` and a
`.gitignore`, exposed to the process as `RHQ_PERSONA_DIR` and to the command
template as `{memory}` (e.g. `--add-dir {memory}` for claude).
`posse memory <persona>` opens the standing orders in $EDITOR.

The `.gitignore` is there because the landing below sweeps the WHOLE dir and
that dir is also where a persona works: five `myai-suite*.out` captures of
test stdout were committed as one persona's standing orders (ranger-base-c9m7).
The filter is git's own rather than a list of blessed filenames, because an
allowlist stops landing a persona's real notes silently — four of the 29 files
tracked under this instance's `personas/` are deliberate work that an
ORDERS.md-and-`pending/` allowlist would have dropped. Seeded with `*.out` and
`*.log`; it is the persona's to grow and nothing rewrites it.

## Leaked gate-shell children, and `POSSE_KEEP=` (ranger-base-apwr, -gvp2p)

Nothing on this box used to end a leaked gate-shell child. A non-interactive
zsh does not hang up its background jobs, so a Bash line that backgrounds
something and returns leaves it running with launchd as its parent; the load
guard then correctly declines to launch into the wreckage and waits forever,
because the wreckage has no living parent and nothing is looking for it.
That is teau's 2h30m freeze (sixteen spinners at ~30% of a core each), and
ranger-base-k6csq's second helping (forty, ~2h, and the session that started
them ran `pkill` and *believed it had cleaned up* — `jobs -l` and a %CPU
floor are both structurally blind to that shape).

**The predicate.** A leak is: `ppid == 1`, **and** at or over
`LoadCulpritOrphanCPU` (20% of one core), **and** at or over
`LoadOrphanMinAge` (1m — long enough to clear the second a process is
briefly ppid 1 in while the shell that forked it exits), **and** its argv
*opens with* the ADR 0009 gate-shell preamble. A forked subshell never
execs, so it carries its parent's whole `-c` string: that is what makes the
preamble a reliable "this came out of a persona's Bash line" marker, and why
"opens with" is enforced at the head — a process whose argv merely *talks
about* the preamble (a `grep`, an editor, a `ps` of the report itself) is
some other process's text about ours, and calling that a leak is the teau
misreading in a new costume.

**Arm 1 (shipped) names them and kills nothing.** It rides under the load
guard's culprit line, off the same single `ps`. Two readings reach it: a pass
the guard is skipping, and — since ranger-base-fxs60 — the **guard clock**,
one tick per `--watch` interval, on its own goroutine, **whether or not the
box is over the line**. Both of those are why: under ADR 0028 §1 a rolling
`Run` does not return while a bead is in flight, so on 2026-09-02 one loop
ran 1h40m on a single pass while eight orphans burned ~50% of a core each and
nothing evaluated the guard at all; and when that loop was restarted the load
at pass start had dipped to 44 under `load_guard: 60`, so the pass was not
skipped and the same eight went unreported again. **Load is not the
predicate. The leak is.** `go run ./cmd/checkorphans` is the same predicate
without the CPU floor and without the load-spike framing, for a persona
asking "did the thing I just backgrounded leak".

**Arm 2 (shipped OFF) ends them, and `POSSE_KEEP=` is how you keep one.**
Exactly one false positive exists and nothing in the process table separates
it: a persona that *deliberately* backgrounds a long-lived CPU-consuming
process and lets the tool call return has the identical signature. The
difference is intent, and intent is not in the process table — so the
operator's 2026-08-31 ruling put it there. **Declare or die:**

```sh
POSSE_KEEP=<reason> nohup ./long-thing.sh &     # the documented form
POSSE_KEEP=ranger-base-abcd; cd /repo && ./bench.sh &   # equally fine
```

The reason is conventionally the bead id that authorises it; the guard
prints it and does not judge it. Write it **at the head of the line** — the
guard will honour it anywhere in the argv (see below), but the head is where
the next person reads it. Undeclared is a leak and a leak is killed.

Three properties worth knowing before you rely on any of it:

- **The two anchors point opposite ways, on purpose.** The preamble
  ("is this ours") is matched at the *head*, because a loose match there
  kills a stranger's process. The marker ("was this declared") is matched
  *anywhere*, because it is the spare: a loose match there only means a leak
  survives — which arm 1 still reports and you can still kill by hand —
  while a tight match kills something somebody meant, and that is
  irreversible.
- **The signal ladder is measured, not assumed** (darwin 25.4.0, planted
  control, forked non-exec'd subshell): `TERM` ends `sh`/`zsh`/`bash`
  children in **24–31ms**. The same spinner behind `trap "" TERM` survives
  TERM indefinitely (3.0s and still going) and dies of `KILL` in 24ms. So:
  TERM, one **shared** 500ms grace for the whole batch, then KILL for
  whatever ignored it, then a shared 250ms confirm. Forty leaks cost what
  one costs.
- **It fails open on the pass and closed on the kill.** A pid that will not
  die is a word in a log line, never a pass that is late. But the reaper
  re-reads its targets' rows immediately before signalling, and a target it
  cannot re-verify — gone, recycled onto another process, no longer ppid 1,
  no longer ours, declared in between — is skipped rather than killed, with
  the reason printed. Non-positive pids are refused outright: `kill(2)`
  reads those as process *groups*.

`load_guard_kill:` in `config.yaml` arms it (`true`/`false`; absent is
false, and a typo is named in the report and leaves it off) — and since the
guard clock, that key is the ONLY thing gating the kill: `load_guard: 0`
turns off the launch gate and no longer turns off the census with it. It
ships **off**: the ruling's first bar for the live flip is arm-1 field data
showing real leaks and no deliberate process, and reading that is the
operator's, not the guard's.

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
- **The wall is L1–L3, ours on every runtime whose shell we type and in
  every repo whose hook is ours.** Neither condition is rhetorical and
  each has a name: **L1** holds on a runtime because ADR 0009's **gate
  shell** is typed there as `SHELL`/`GROK_SHELL`, and `gate_shell: false`
  (`Runtime.NoGateShell`) for a runtime that chokes on a wrapper drops
  that runtime straight back to unrealized; **L3 is per-repo**, counted
  only in the repo the session launches into and only while the hook file
  there is byte-for-byte the one we render (ADR 0023 — identity, then a
  behaviour probe of our own render). `CheckParity` — `CheckParityIn` for
  L3's directory-aware half — is the per-launch source of truth for both,
  and it recomputes rather than remembers; under `cage: container` it also
  answers no unless the image carries a Linux posse to render L1/L3
  *inside* (cageinner.go). Rendered fresh from the PID at every launch:
  **L1** PATH shims for every shell-verb deny
  (`Bash(git push:*)` → `gates/<persona>/bin/git` refuses, logs to
  `refusals.log`, execs the real binary otherwise; PATH prefixed on the
  typed line, and the **gate shell** typed as `SHELL`/`GROK_SHELL`);
  **L3** the git pre-push hook honouring `RHQ_TOOLS_DENY`
  for the one verb that is a hard risk line. Both cost nothing per
  session and are on for every persona session — claude included — but
  the hook is installed in the repo the session
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
  (rangerhq-e43), so parity counts L1 on every runtime again — and that
  is the rule for the next runtime we add: its PATH behaviour is
  **verified per runtime in a live session** (`command -v git` resolving
  into `gates/<persona>/bin`), never assumed from the fact that we typed
  the shell. Residual: on a *persistent-shell* backend
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
  persona's memory dir, the gates dir (refusals.log), the **launching**
  runtime's own state and no other runtime's (`state_dir:`, ADR 0012 D4 —
  a claude launch grants `~/.claude`/`~/.claude.json` and NOT `~/.codex`
  /`~/.grok`; it was the union of all three until ranger-base-9fl, which
  is a cross-runtime auth-store WRITE and so an exfil channel no read
  deny can close) — plus, where a `state_dir:` names a FILE, its
  atomic-write siblings as a dot-anchored `regex` (`<p>.lock`,
  `<p>.tmp.*` and nothing else): `subpath` is component-aware, so the
  grant on `~/.claude.json` alone covered neither of the two paths claude
  actually writes and EVERY caged write to it was dropped while the CLI
  printed success and exited 0, for as long as the tier existed
  (ranger-base-cypy1; 185 of one day's denials on this box) — the
  generic caches,
  posse's own `state/` **derived from the App's home** (it was the literal
  `~/.config/rhq/state` until ranger-base-cpyb, so a second `RHQ_HOME`'s
  sessions got no grant to their own state dir and one into the default
  instance's — rangerhq-qfzr), `$TMPDIR`/`/tmp`, `/dev`, and the PID's
  `writable:` extras (relative to the repo). **What it never grants is the
  rest of the home** — `agents/`, `config.yaml`, `recipes/`, `runtimes/`,
  `skills/`, `envs/`, `promoted.json`: after ADR 0015 §2 that is the promoted
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
  rendered profile: `touch` in `$CONSTITUTION/.beads` and `.git`
  succeeds, `touch $CONSTITUTION/x` → Operation not permitted, and
  `bd export -o` — the command that failed on the db file — exits 0.
  Path-scoped denies (`Edit(docs/adr/**)`) are **ADR 0014**: a
  subtree file-write deny, realized by a trailing SBPL `subpath` deny
  after the cwd allow (last match wins, measured 2026-08-25) and, at
  container, a `:ro` overlay of that directory — never by a hook.
  `writable:` is the allow-list dual at both L2 and L4. The matrix and
  both renderers are in, and so are the example PIDs that use them
  (ranger-base-ccd): **architect** is the allow-list — `deny: Edit, Write`
  plus `writable: [docs/adr]`, so the repo is unwritable except the one
  directory it writes; **developer** and **qa** are the deny-list —
  `Edit(docs/adr/**)` / `Write(docs/adr/**)`, so the repo is writable
  except the ADRs that constrain them. QA takes the developer's shape and
  not the reviewer's because `harden-suite` commits tests: it is
  mixed-intent, so it is not the verifier shape. All three declare `cage:
  seatbelt`, which is the whole point of the key — at `shims` a
  path-scoped write is not a tool-name deny, `posse gates` prints `✗ needs
  cage: seatbelt (or container)`, and the launch refuses (dispatch never
  passes `--allow-degraded`). Reviewer and security keep the bare
  `Edit`/`Write` wall; nothing path-scoped goes on them, they are already
  stricter. Verified on this host:
  `touch` in the repo → Operation not permitted, `ORDERS.md` append and
  `.beads/` writes succeed, claude/codex/grok all start under it. Measured
  on this host 2026-08-29 over the rendered profiles, each arm with the
  control that must NOT match: under developer's, `touch docs/adr/x`,
  `sed -i` on an ADR and a `python` `open().write` there all fail while `touch
  internal/x` **and `touch docs/x`** succeed — the sibling is what shows
  the deny is the subtree and not the parent; under architect's, `touch
  docs/adr/x` and `sed -i` on an ADR succeed while `touch internal/x` and
  `touch docs/x` fail, and `.beads/` stays writable so claim/comment/close
  survive the wall. **Codex cannot be wrapped**: it sandboxes its own
  child commands with `sandbox-exec`, and macOS refuses
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
  And **`(deny file-write*)` denies writes, and now three named files**:
  the profile carries no `mach-lookup`/IPC denies, so L2 stops no *read* by
  a session of anything its user can read — the macOS keychain included,
  because `security` asks `securityd` out-of-process and the item's own
  ACL, not the sandbox, answers — with one narrow exception
  (`ranger-base-hw18`): a trailing `(deny file-read*)` on the known
  credential-store literals (`~/.claude/.credentials.json` on darwin,
  `~/.codex/auth.json`, `~/.grok/auth.json`, minus whichever belongs to the
  launching runtime), because those three files are the one *read* whose
  cost was measured (ADR 0019 D2's unowned darwin byproduct). Everything
  else stays open. A deny aimed at a **read-only** tool is therefore
  realized by **L1 alone** below the container tier — which is still worth
  declaring, because L1 is the only layer that refuses deterministically
  *and* writes a line to `refusals.log`. Read it as a tripwire, not a
  wall: L1 matches the typed word, so `/usr/bin/<cmd>`, `sh -c`, and an
  exec by an allowlisted build tool all walk past it (the documented class
  in the `gates.go` header). The wall for an arbitrary read stays L4.
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
| `bd` inside a `:ro` repo | **works through one carve-out** (rangerhq-3nxk/abvm, measured 2026-08-22, re-run 2026-08-27): `{dir}/.beads` bind-mounted **read-write over** the `:ro` repo — a pre-existing DIRECTORY lands rw over a `:ro` bind of its parent, later mount wins, and `touch` in the repo still fails — plus a `bd --no-db --no-daemon` wrapper on the inner gates PATH. `create`/`comments add`/`dep add`/`q`/`show` answer sub-second, `close` still enforces the graph, and the host's daemon imports the JSONL, so a caged comment is on the ordinary host path with nothing typed. Direct SQLite through the same mount is ~5s a command, failing to start a daemon it has no socket for |
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
`internal/posse/cage.go`. The engine is a command template, never a
hard-coded `docker run`: built-in `docker`, `RHQ_HOME/cages/<name>.yaml`
for anything else (only `command:` is required — `mount:`/`mount_ro:`/
`env:`/`home:`/`build:`/`probe:`/`live:` default to docker's spellings,
which is what makes OrbStack a swap), chosen by config `default_engine:`.
`live:` (docker's `docker version`) is the engine's own liveness, asked
apart from `probe:` because an installed CLI whose daemon is not running
fails the image probe with a connect error and every reader used to call
that a missing image and advise a build through the engine that is not
answering (ranger-base-1mu9r). It is asked only once the image probe has
already said no, so it chooses the wording of a refusal and never causes
one; an engine that spells none answers yes, on `probe:`'s precedent. `posse
cage` prints engine, image, readiness and the image's AGE; `posse cage
<persona>` prints what would cross the boundary; `posse cage build [dir]`
cross-builds a
Linux `posse` and `bd` 0.49.1 out of a posse checkout and hands
`etc/cage/Dockerfile` plus those binaries to the engine (~45s, ~600MB;
`--runtimes` adds CLIs — claude is installed by default because it is
the only runtime whose container credential is decided).

Age, because the inner render below is the IMAGE's posse and until
ranger-base-nwj7 nothing but a live test's FAIL ever said the image was
behind it (`internal/posse/cagestale.go`). `posse cage build` stamps the
posse it bakes in with the checkout's identity — go's own `-buildvcs`
stamp is absent for a build from a linked worktree, which is where every
persona works, so without it an image cannot name the commit it came from
— and the host compares that against what the source in hand builds.
Three states, not two: the same build, a different one (**stale**: the
wall in the cage is that build's, so rebuild before reading anything in
there as a regression), or an image that could not be asked
(**unclear**, deliberately not stale — a live pin that skips on a probe
failure is one that goes green and stays there). The live pins whose
subject is the inner render skip on stale rather than failing one clause
of a claim they never measured.

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
`internal/posse/cagelauncher.go`. `state/cages/<persona>/bin/claude` is a
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
`internal/posse/egress.go`. Unconditional at `cage: container`, not
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
`sockets:`.** `internal/posse/cageinner.go`. The tiers are cumulative in
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
  two caged sessions of one persona cannot clear each other's shims. The one
  file mounted out at that same name — `refusals.log` — is never the
  canonical `gates/<p>/refusals.log` any more (ADR 0025 §4, ranger-base-l40c):
  it is this SESSION's own spool, `state/cages/<p>/<session>/refusals.log`
  (`CageSpoolPath`), because a bind mount cannot tell an append from a
  rewrite and mounting the canonical log rw let a caged persona erase its
  own refusals (`: > refusals.log`, measured live in rangerhq-pafo). A
  host-side FOLD (`internal/posse/refusalfold.go`) is the only path from a
  spool into the canonical log L1's host refusals and the egress proxy's
  403s already land in: a cursor per spool (byte offset + SHA-256 of the
  bytes up to it, `state/gates/<p>/spool-cursors/<session>`, host-only)
  makes the fold idempotent, and a spool shorter than its own cursor, or
  hashing different over the bytes the cursor covers, folds as a
  `tampered` line naming the session. That is the reach of it and no
  further: a spool cut back to its cursor or anywhere above it between two
  folds, or before its first fold, folds as "nothing new" and leaves no
  mark (measured, ranger-base-j3r6z) — what the fold guarantees is that
  the canonical log only grows and everything already folded is out of the
  cage's reach, not that an erasure leaves evidence (ADR 0025 §4 as
  amended). Folds ride host loops that already run — the dispatch sweep, a
  session close, a relaunch — never a new daemon.
- `realShell` now **resolves a shell that exists** (`$SHELL` if it is a
  real bash/zsh, else zsh, else bash, else `/bin/sh`). Inside the image
  `$SHELL` is unset and `/bin/zsh` is not there; the old hard-coded
  fallback would have rendered a wrapper that cannot `exec` its own REAL.

**The mount boundary** is the repo, and only the repo: `:ro` when the PID
denies `Edit`/`Write`/`NotebookEdit` (any one of the three — the boundary
is a property of the mount, not of a rule), while `{memory}`, the cage
HOME and this session's refusals spool stay writable — the canonical log
itself is never mounted (ADR 0025 §4). A **worktree** gets its git
common dir mounted alongside, because `.git` there is a *file* pointing at
the main repo's `.git` and L3's hooks live in it — a hook the container
cannot see is a `git push` this tier lost. That mount is `:ro` with
read-write overlays of `worktrees/<own>`, `objects` and `logs`, and the
session works on a detached HEAD so it needs no ref write at all
(ranger-base-t4f1; the section below).

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
`sockets: [herdr]`. Extended 2026-08-27 (rangerhq-abvm) with the carve-out:
`command -v bd` → `/posse/gates/p/bin/bd`, a write under `.beads` succeeds
on that same `:ro` repo, `bd comments add` inside answers *Comment added*,
and a plain host `bd show` reads the comment back with nothing imported by
hand — *while the repo's daemon is up*, which is the one qualification that
paragraph needs. Stop it and the same read fails: `bd show` waits 5s for a
daemon it cannot start, drops to direct mode and refuses with *Database out
of sync with JSONL. Run 'bd sync --import-only' to fix.* The import is the
daemon's, not the JSONL's.

Superseded 2026-08-31 (ADR 0025 §4, ranger-base-l40c): "both refusals land
in the host's `refusals.log`" above was true because the file was mounted
straight into the cage — that mount is gone. They land in this session's
spool now, and reach `refusals.log` only once something folds it (see the
bullet above); `TestQALiveCageEscapeAttemptsOnAWritableRepo` folds
explicitly rather than reading the log right after the container exits.

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
comment or close a bead**. **Answered, and now built** (ADR 0002's
amendment via rangerhq-3nxk, the same answer ADR 0014 §4 reaches from the
path-scoped side; rangerhq-abvm): the `:ro` repo carries **one carve-out**,
`{dir}/.beads` mounted back read-write over it, and the inner render puts a
`bd --no-db --no-daemon` wrapper on the gates PATH so bd uses the JSONL
rather than the SQLite it cannot open and the socket that cannot cross.
The wall is the rest of the tree; claim, comment and close survive it.
Overlapping binds on VirtioFS are no longer ASSUMED — a pre-existing
directory mounts rw over a `:ro` parent, later mount wins, and `touch` at
the repo root still fails (measured 2026-08-22, re-run 2026-08-27 in
`TestLiveInnerGatesHoldInsideTheCage`). Three costs come with it and are
accepted rather than hidden: the carve-out is a **directory, not a
protocol** (anything under `.beads/` is writable, bd's own files included);
there is a **lost-update window** on `issues.jsonl` between the host
daemon's export and a cage's no-db append (low frequency, git-visible);
and a **worktree** session's caged bd writes that worktree's own tracked
`.beads/issues.jsonl` — nearest `.beads` wins — so the write reaches the
shared store by merge/sync, unlike host worktree bd, which routes to the
main checkout's daemon. `.git` is the other carve-out ADR 0014 names, and it is
**built now** (ranger-base-yu5) — see the section below.

## Path-scoped writes at L4: the overlays, measured (ranger-base-yu5)

ADR 0014 §4 shipped with its central mechanism marked **ASSUMED** — "the
Docker probe was not run this session" — and named this bead's done-when as
that probe. It is now `docs/adr/0014-path-scoped-writes.probe.sh`: seven
probes, each with the control arm that has the overlay taken away, run
2026-08-29 on macOS 26.4.1 / Docker Desktop engine 29.0.1 (VirtioFS). All
seven answered their expect line, so nothing DIVERGED and the tier really
holds the rules the matrix has been printing since ranger-base-4ks.

What the engine actually does, as against what the ADR guessed:

- A bind of a directory lands over a bind of its parent in **both**
  directions — `:ro` over read-write *and* read-write over `:ro`.
- The ordering rule is **destination depth, not list order**. The overlay
  listed *before* the repo it sits on gives the same answers (probe 6). So
  "later mount wins" was the right answer for the wrong reason, and a
  renderer that relied on emitting parent-first would have been relying on
  nothing.
- A `:ro` overlay whose **source does not exist** still denies: the engine
  creates the source in the writable parent, and `touch` in it is refused
  (probe 7). That matters because a rule is about a PATH — `mkdir docs/adr`
  is exactly what a persona does next — and a Stat-guarded deny would have
  been a wall that disappears when the directory has not been made yet.

### Two things the renderer had to be built around

**deny-wins cannot be delivered by order here.** At L2 a `writable:` extra
inside a denied subtree loses because SBPL takes the last match and the
trailing deny block is below every grant. At L4 that extra is *deeper* than
the deny containing it, so depth-sorting makes it **win** — the exact
inversion of ADR 0001. `cage.go` therefore **drops** such an extra rather
than trying to out-order it. Same rule, opposite mechanics, and it is why
`posse agent check`'s warning about the pair is now true at both tiers.

There is a *second* pair with a different answer, added with ADR 0038
decision 4a (ranger-base-672zt): an extra naming the **same destination** as
a deny the code places — `writable: [.git/hooks]` against the `:ro` bind of
the hooks dir. Dropping cannot deliver it, because the extra is not inside
the deny, it *is* the deny's path, and `cageOverlay` answers a
same-destination overlay by EDITING the existing mount's mode (two binds on
one destination is an engine error). So whichever pass runs second wins, and
`cageGitIdentityBinds` runs **after** `cagePathScopedOverlays` for that
reason alone. Order delivers deny-wins for this pair and cannot for the
other one; both mechanics are load-bearing and neither generalizes.

**An overlay must be spelled the way the mount it lands on is spelled.**
This one is silent and was caught by mutating the code under the pins, not
by reading it. A session dispatched through a symlinked parent (`/tmp/x` on
macOS, really `/private/tmp/x`) mounts the repo at the path it was *given*;
inside the container there is no symlink to follow. An overlay resolved for
the host lands at a destination nothing mounts — the bind succeeds, `posse
cage` prints it, `posse gates` says `✓ L4 :ro overlay`, and the denied
subtree is writable at the only path the persona can reach it by. So
`cageCovering` finds the deepest mount containing the resolved path and
re-spells the overlay in *that mount's* words, source and destination both.

### The carve-outs on a `:ro` repo

`.beads` was already there (rangerhq-3nxk). `.git` is now beside it, which
is ADR 0014 §4 answering the same question the `.beads` clause answered: L2
grants a write-denying PID `cwd/.git` for index refresh and git's own locks,
and a tier that took it away would be enforcing more than the gate. In an
ordinary repo that is a directory inside the repo mount and an overlay says
it. In a **worktree** `.git` is a FILE and the index, HEAD and objects all
live in the common dir, which is a mount of its own.

### The worktree common dir is `:ro`, and narrower than L2 (ranger-base-t4f1)

ranger-base-yu5 shipped that common dir read-write WHOLE, which was **wider
than L2** and said so here. ranger-base-6q5e assessed what the width bought
an adversary at the one tier whose job is containing one: `refs/heads/main`
(an `update-ref` is not a push, so L1's shim never sees it, L3's pre-push
never fires, and the launcher's own ff fires no hook either), `packed-refs`
wholesale, `config` — where a planted `core.hooksPath` **dodges** the
hooks-`:ro` overlay of ranger-base-3c3/h15 rather than being stopped by it,
because moving the slot beats freezing it — and other sessions'
`worktrees/<name>`, their HEAD and index included.

What ships now is **narrower than L2 rather than wider**: the common dir
`:ro`, with read-write overlays of `worktrees/<own>`, `objects` and `logs`,
and **no ref write at all**. L2's grant is those three plus the session's own
`refs/heads/<branch>` and the `.lock` git renames onto it (ranger-base-m2wf),
plus that ref's parent directory for CREATION only (ranger-base-uuze — the
branch carries a slash, so git must `mkdir refs/heads/posse`, and `git gc`
prunes it as soon as a pack empties it). The ref half is not expressible as a
mount list and no amount of care makes it one: a bind mount's source must
EXIST, git creates the `.lock` at commit time, a pre-created `.lock` fails
every commit with "File exists", and the `rename(2)` that commits a ref update
cannot replace a bind mountpoint. Lock-then-rename and file-granular binds
fight by construction.

**So the session is launched on a DETACHED HEAD instead** (`PrepareSessionHead`
in worktree.go, called from `planLaunch` once the tier is resolved), and the
launcher splices the work back with `git -C <tree> branch -f <branch> HEAD`
at close — the exact command `landed()` has always printed for humans, run
under the launcher lock before `MergeSessionWork`'s guards. Detaching is what
buys the narrowing, and it is measured rather than reasoned: under exactly
those three writable regions the same commit lands detached and **fails on the
branch**, at `cannot lock ref 'HEAD': Unable to create
refs/heads/posse/s-1.lock` (2026-09-05, git 2.50.1, arm A4 of
`docs/adr/0014-l4-worktree-narrowing.probe.sh`).

Three consequences worth keeping:

- The splice runs only for a session posse RECORDED as detached
  (`branch.<b>.posseDetached`, git config on the branch beside `posseBase` and
  `posseBead`, so it survives a kill that removes the meta). Without that
  record it would silence the ranger-base-dybv guard — an off-branch HEAD is
  designed here and an anomaly everywhere else, and only the record tells them
  apart. It is fast-forward only: `branch -f` has no ancestry check, and a
  branch tip the tree's HEAD does not reach is work it would delete.
- `worktree list --porcelain` prints `detached` where it would have printed
  `branch refs/heads/…`, so the landing sweep, `posse worktrees` and the merge
  stopped seeing these trees entirely — over exactly the sessions whose work
  is in the tree and not on the branch. `SessionTreesIn` now recovers the name
  from the tree's directory (`SessionTreePath` makes it the session name,
  `SessionBranch` makes the branch from that) and confirms it against the
  repo. Residual, stated: a detached tree whose branch somebody deleted is
  invisible there — git refuses `branch -D` for a branch a worktree has
  CHECKED OUT, and a detached tree has none.
- `<common>/logs` is in the set because L2's is, not because a detached commit
  needs it. Measured (arms A5/A5b): a linked worktree's HEAD reflog is
  per-worktree (`worktrees/<own>/logs/HEAD`), and `<common>/logs/refs/heads/…`
  appears only when a commit moves a SHARED ref — the thing detaching removes.
  It is kept for the in-cage operation that does update one, and the launcher
  `mkdir -p`s it so the rendered mount set does not depend on whether this repo
  has ever written one.
- The launcher makes a second source for the same reason, and this one does
  not exist at all until it does: `worktrees/<own>/config.worktree` (ADR 0038
  decision 4b, ranger-base-p9h9d). The identity chain that selects WHICH
  config and hooks a later git reads gets `:ro` file binds over the
  read-write `worktrees/<own>` overlay — the pointer, `gitdir`, `commondir`
  and that file — but posse never sets `extensions.worktreeConfig`, so `git
  worktree add` writes no `config.worktree` and the deny direction's Stat
  (`cageOverlayFile`) would drop the bind for want of a source. It cannot
  simply bind the absent path: a `-v` of an absent source becomes a host
  DIRECTORY, and a `config.worktree` directory makes every git command in
  that tree fatal (MEASURED, git 2.50.1). **A wall keyed on a config key
  would read a different repo from the wall beside it**, so
  `PrepareSessionHead` creates the file EMPTY instead and the bind is
  unconditional. An empty one is inert in both directions, measured: with
  the extension off git never reads it, with it on there are no keys to
  read. Never truncated on a relaunch — that would be posse deleting the
  operator's own per-worktree config.

This **subsumes ADR 0038 decision 4** for worktrees (folded as
ranger-base-mugt2, operator-confirmed 2026-09-01): that asked for `:ro` file
binds of `<common>/config` and `<common>/hooks` over a read-write common
mount, and both paths are now inside the `:ro` mount and under none of the
three overlays. The ADR's own prose still describes the read-write-whole
mechanism and is stale; amending it is architecture's, filed as a handoff.
`.git/hooks` in an ORDINARY repo is still ranger-base-3c3 / h15.

Residual after the narrowing, stated: `objects` (content injection — inert
until a ref names it, and naming goes through the splice and the launcher's
ff), `logs` (the reflog), and the session's own worktree dir. Same class L2
accepts.

Measurement: the mount SET is pinned in Go
(`TestWorktreeGitCommonDirIsTheGitCarveOut`, cageoverlay_test.go, with the
`logs`-absent arm that shows the Stat guard dropping an overlay), and the
detach/splice round trip in worktree_test.go — fourteen mutants, all killed.
`docs/adr/0014-l4-worktree-narrowing.probe.sh` carries the rest in two parts:
Part A is MEASURED here (uid permissions, not the L4 wall, but it is the half
that is about git rather than about the engine), Part B is the bind-mount arms
and is **UNRUN** — Docker was abandoned on this box on 2026-08-30
(ranger-base-6mz7). Part B's foundation is measured: the seven probes above.

Every commit under a narrowed common dir — L2's `sessionGitGrants` and now
L4's mount set alike — also prints `error: Unable to create
'<common>/packed-refs.lock': Operation not permitted` on stderr and still
succeeds; git takes that lock speculatively on every ref update and falls
back cleanly when refused. Measured again at L4 on 2026-09-05 (arm A3), where
`gc --auto` after such a commit also exits 0 and leaves HEAD intact. DECLARED
rather than fixed (ranger-base-msex):
the tempting createOnly grant was measured and is worse than the noise —
create-only buys the create but not git's own cleanup unlink, so the lock
file is stranded in the shared common dir, which then hard-fails the
operator's own unsandboxed `git gc`/`git pack-refs` until someone notices
and removes it by hand. No session can write it away either. See the doc
comment beside `sessionRefDirs` in `seatbelt.go` for the measurement.

The **redirect target** joins them, unconditionally rather than only on a
`:ro` repo: when `.beads/redirect` puts the store of record in another repo,
`<dir>/.beads` holds a path and nothing else, so the carve-out above mounts
a directory nothing writes and every mutation lands outside the cage — the
L4 shape of the L2 failure ranger-base-rhw measured. The target alone, not
its `.git`: the inner wrapper is `--no-db --no-daemon`, appends JSONL and
never commits, so L2's git grant buys nothing here and would mount a second
repo's history read-write.

### A trap that is not the engine's

Re-running these binds by hand in **zsh**: `"$R:$R:ro"` is not what it looks
like. `:r` is a zsh modifier (strip the extension), so the word becomes
`$R:${R}o` — docker then binds an empty auto-created directory read-WRITE at
a destination one character off, and the probe reads as "the engine ignored
`:ro`". Spell it `"${R}:${R}:ro"`. The probe script is `/bin/sh` and is not
affected; this cost a measurement that briefly looked like a DIVERGED.

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
`internal/posse/egress.go` all rested on one assertion:

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
cage build .` over a harness checkout takes **11s** with a warm layer
cache (89a's ~45s was cold) and the image is **1.23GB** on disk by
`docker images`. The image was removed afterwards; `posse cage` reports
"image not built" here, as it did before.

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
on the next dispatch — *in a repo that gets a dispatch*. That qualifier is
the whole of ranger-base-ixv4: a launch refreshes the common hooks dir of
the repo its worktree was cut from and no other, so a repo nobody launches
into keeps whatever render it was given, stamp included, indefinitely. The
constitution repo is the extreme case — it holds the PIDs and holds no
session — and it ran a `prepare-commit-msg` without the constitution-path
arm for hours after that arm shipped. `SweepHookWall`
(`internal/posse/hookfresh.go`) is the standing answer: it walks the
`beads_visibility:` keys and asks each one the ADR 0023 question through
the same `probeL3Hooks`, rendering per repo from *that repo's* configured
visibility so a stamp that disagrees with config is a byte mismatch in
either direction. It runs from `posse promote`'s epilogue and once at the
top of a `dispatch --watch` loop — once, because the answer can only change
when the binary does and a loop is a binary. It reports and installs
nothing. The way through is to re-file the bead in the
private db and cite its id; the override is
`RHQ_VISIBILITY_OVERRIDE=i-mean-it`, operator-typed, never in a session's
environment (a test pins that), and it is logged to `refusals.log` when it
is used.

**And it is a lint, not a boundary** — same class as the allowlist. The
boundary is the routing rule plus repo visibility; the lint exists so a
mis-routed bead is a refusal at commit time instead of a public artifact.

The pattern list lives in one place (`OpsPatterns`, `internal/posse/visibility.go`)
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

**When an instance holds someone else's data** — a work laptop, a client
engagement — the bead-class rule above generalizes, and the generalization
is an obligation rather than a preference: **the data OWNER picks the
boundary**, and the only content that crosses between an operator's
boundaries (personal, employer, public) is what is already public in the
one it came from.

Assume such an instance's ENTIRE beads db is confidential to the data
owner. Titles and comments are the working medium — a bead that names no
secret still says what the owner is building and when — so per-bead
classification is theater there. Concretely:

- every repo of that instance is marked `private` in
  `beads_visibility:`, and unmarked-is-public does the rest: a repo
  someone forgets to mark comes back a refusal, not a leak;
- its db, config, env sets and crew memory never touch another
  boundary's infrastructure and never ride dotfile sync — one instance,
  one boundary, no shared `~/.config` between them;
- upstreaming generalized smarts to the public harness follows
  CONTRIBUTING's flow-in rule PLUS whatever contribution approval the
  data owner's policy requires. The harness rule is necessary, not
  sufficient;
- security findings about the data owner's systems never enter a public
  queue, regardless of severity or how design-level they look. Their
  disclosure is the owner's call, not this repo's disclosure rule;
- where the owner has a system of record — a tracker, a ticket queue —
  it stays the system of record: beads cite its ids rather than quote
  restricted content.

**Instance-defined patterns.** The shipped `OpsPatterns` list is what any
deployer needs; a name that is confidential HERE — a client, a project
codename — is not the public repo's to carry. Config `beads_visibility_patterns:`
appends to the list at stamp time, so an instance teaches the lint its own
vocabulary without patching the harness:

```yaml
beads_visibility_patterns:
  client-acme: Acme[[:space:]]*(Corp|Holdings)
  codename: (BLUEBIRD|REDSHIFT)
```

The key is the class the refusal names, the value an ERE in the same
two-reader dialect as the shipped list — POSIX ERE ∩ Go, no single quote,
no `\d`/`\s`/`\b`/`\t`/`\w` — and flat-YAML's limits apply to the value: one
line, a trailing ` #` starts a comment, a wrapping pair of double quotes is
stripped. An entry that breaks any of those is **refused at stamp time and
named**, in the hook file itself and in `posse gates install-hooks` output,
rather than dropped silently — a pattern the operator believes in and that
is not there is worse than no pattern. Rejection reasons never echo the
value, only the class name, so keep the class name something you are willing
to read in a terminal. Neither does a REFUSAL: an instance class is refused
by class and hit count alone — never the pattern, never the text it matched
— wherever it is scanned, because a refusal is read in a terminal, written
to `refusals.log` and pasted onto beads, and the text is the one thing the
key exists to keep out of a public tree (the shipped list keeps showing what
it matched; its text is in this repo's source already). The one thing a
refusal does print is the offending staged PATH, which is the writer's own
artifact and the only thing that says which file. The patterns live in the operator's config and get
stamped into `.git/hooks/`, both untracked: the vocabulary never enters the
public repo, which is the point of the key. Their SCOPE is wider than the
shipped list's since ADR 0048: an instance pattern is scanned over the added
lines of every staged file, code included (and a real blob's BYTES: the
reader carries `--text`, so a file git calls binary is scanned like any
other, and a genuine asset that trips a class goes through on the typed
override), over every added staged path, and — since ADR 0024 D2 check 3 and
ADR 0048 D2 as amended 2026-09-03 — over every line of the COMMIT MESSAGE,
which replicates with the branch exactly as a staged line does; the remedy
there is to rewrite the message, and what you typed is still in
`.git/COMMIT_EDITMSG` until the next commit overwrites it. The derived
identity literals of check 3 get the same three subjects. Check 3 derives one
more set with a scope of its own: this box's
CREW NAMES — the PIDs in your `agents/`, less every name posse itself ships
as an example role — matched over ADDED STAGED PATHS ALONE, case-insensitively
and with no word boundary (ADR 0012 D2 and App.A 5 at commit time,
ranger-base-cdxpf). A persona name in a staged line or a commit message is
left alone on purpose: the crew stand in `docs/` and the root narrative as
historical actors, and a message names whoever wrote it. A file NAME ships in
every clone with nothing to exempt it, and the way through is ADR 0012 D2's
own — name the file for the ROLE, not the seat.
The shipped list stays markdown-only and is NOT scanned over the
message — its own source is byte-identical to a hit, a config pattern is
never in source, and the message decision is its own census: of 29 hits
over the 1136 messages then on main, 22 were the software's own vocabulary
(fixture figures, blessed defaults, documented key values), and a message
has no shape table to disposition them by.

**The data ceiling** (ADR 0050) is the second key, `data_ceiling_patterns:`,
same shape and one class namespace with the first, and it answers a
different question: visibility says where content may go, the ceiling says
whether it may exist in a local file here at all. A visibility pattern is
inert in a repo stamped `private` — on purpose, the stamp is the visibility
record — which is exactly the repo an instance holding someone else's data
keeps its beads in. So the ceiling is scanned in every repo this instance
hooks whatever its stamp, above the visibility gate, over the same three
arms as check 3 — added lines of every staged file (a real blob's BYTES
included: the reader carries `--text`, and a genuine asset that trips a
class goes through on the typed override), added staged paths, and every
line of the COMMIT MESSAGE (ADR 0050 D2 as amended 2026-09-03: the message
lands in the commit object and replicates with the branch, and the hook is
already holding it as its first argument — check 3
gained the same subject the same day and reads it through the same
renderer, so what separates the two walls there is the gate and the remedy,
not the subject) — always by class alone, a refusal being itself a local
file. Its remedy is not "re-file it in the private db" but
remove the paste and keep the system of record's id; for the message arm,
rewrite the message, and the text you typed is still in
`.git/COMMIT_EDITMSG` until your next commit overwrites it. A message typed
in the EDITOR is the stated exclusion: `prepare-commit-msg` runs before the
editor opens, so the message does not exist yet. What the hook is handed on
that path is git's template, and the arm reads it the way git will — which
means it asks git's CLEANUP MODE, not the source argument. Under `strip` the
read is `git stripspace --strip-comments`, so the `#` status block (untracked
paths and a merge's conflict list included) is not a subject; `-m` and `-F`
are read whole, because there git's cleanup keeps a `#` line
(ranger-base-h3s6q); and under `commit.cleanup=verbatim`, `whitespace` or
`scissors` git strips no `#` line, so the whole file is read and
the block IS a subject (ranger-base-6y3z2 — keying on the source argument let
a branch name and an untracked path land in a public commit object
unscanned). On that path the refusal names the mode and what clears it —
clear the class out of the repo, or leave `commit.cleanup` at its default —
because the writer is refused before they have typed anything
(ranger-base-b21e0; the remedy first shipped as "delete git's block in the
editor", which is the same defect one turn later — the hook exits non-zero
before git launches an editor, so there is no editor session to do it in, and
ranger-base-sx2dq corrected it and pinned the remedy by TAKING it); `verbatim` and `whitespace` land that block, `scissors`
truncates it below its cut line and the refusal says which. `install-hooks` prints the
ceiling line for private-stamped repos too.

**And it is still a lint, not a boundary** — same class as the allowlist,
and the honesty is load-bearing here. An instance pattern is friction that
turns one mis-routed bead into a refusal at commit time. What keeps a data
owner's content out of a public repo is the routing rule plus repo
visibility; a confidential name nobody thought to add is exactly the case a
pattern list cannot see.

**The private repo's NAME is not a class; its PATH FORM is** (ruled
ranger-base-nhvr, operator ranger-base-292z, implemented ranger-base-l9ii).
The retired seed preflight's check 4 carried a prose arm — *the private
repo's name may not appear in public text* — that nothing has enforced since
the seed script was retired, and it is now retired as a rule rather than
revived. Three grounds, all of them the security reviewer's own: the bare
name grants no capability (the rangerhq-yv11 ruling, unchanged); it is
irrecoverably public and load-bearing here — every bead marker, every commit
subject and `HISTORY.md` carry it, 1679 inert markers at the sweep — and
yv11's forward-closed clause ("new public text cites public ids") is
acknowledged dead, because no public tracker was ever stood up and the
marker convention is live practice, not legacy. A check keyed on the bare
name would be measuring 1679 legitimate markers to find a handful of real
hits, which is the wrong instrument.

What stays binding, unweakened, on every commit into this repo is the
content classes above — no cost/plan/spend figures, no credential values or
topology, no operator identity, accounts or habits, and no live
deployment-posture assertions. **Instance particulars are the class; the
name never was.** And a live checkout path *is* a particular: `~/src/<the
instance repo>` is this deployment's topology, not any deployer's. So:

- **New doc, comment and script text names the instance's trees by
  variable, not by this box's path** — `$CONSTITUTION` for the constitution
  repo and `$QUEUE` for the shared queue, the spelling
  `scripts/queue-cutover.sh` already uses for its own overridable defaults;
  `examples/config.yaml` writes the same idea as a placeholder
  (`constitution: ~/src/<your instance repo>/posse`) and either is fine.
  This is the restrictive default made standing by the operator's ruling on
  ranger-base-292z, which called the cutover runbooks product docs: keep
  them public, de-instance the paths, accept the already-burned history,
  no purge. Two days later ADR 0024 D4 moved those three runbooks to the
  instance tree anyway, as one-deployment procedures (commit 92e67bd,
  triaged on ranger-base-yheoa) — so what the ruling's de-instance half
  covers here is everything ELSE the sweep found, and its restrictive
  default is what stands for new text.
- **The standing check is `make ops-check`**
  (`TestInstancePathFormNeverAppearsInTrackedContentUndispositioned`,
  `internal/posse/instancepath_qa_test.go`): the live path form in any
  tracked file, dispositioned per path with a reason, and red for anything
  past that set. It is a census and not a one-shot sweep on purpose — the
  class regenerated 58→60 occurrences in a single day while nhvr was open,
  so a sweep with nothing holding it is a measurement with a shelf life.
- The absolute spelling (`/Users/<you>/src/…`) is *not* this check's — it
  carries the box's username, so it is check 3's identity literal and is
  held by `make identity-check`, tree-wide, with its own dispositioned set.

## beads (bd) substrate: pinned at 0.50.3, 0.51+ is a migration (rangerhq-f49)

**What is running, 2026-09-01.** `bd` **0.50.3**, SQLite, no daemon. Bumped
from 0.49.1 that afternoon (ruling ranger-base-qrh1, executed
ranger-base-8ufhn, commit 291523c) — see *"OPERATOR RULING 2026-08-30: bump to
0.50.3"* below for the reasoning, what landed with it and what did not.
Measured here from `make verify-bd-pin`, not from the pin file, since the
whole point of the check is that the two can disagree:

```
  bd version               0.50.3                             ok
  command -v bd            …/state/gates/<persona>/bin/bd     GATED (execs ~/.local/bin/bd)
  homebrew beads           pinned 1.2.2                       ok
live bd daemons — the layer the 08-16 rollback never checked
  none running
verify-bd-pin: pin intact at 0.50.3 — command layer and process layer agree
```

Everything from here to that ruling subsection is the history that produced
it, left as it was written; corrections are appended in place rather than
rewritten, per this file's convention. Where a paragraph below made a
present-tense claim that the bump falsified, the correction is on the
paragraph itself.

The fleet ran `bd` **0.49.1** on purpose from 2026-08-16 to 2026-09-01 and runs
a pinned **0.50.3** on the same purpose today; the pin is the point, not the
number. beads removed the SQLite backend for embedded Dolt at **v0.51.0** —
not at 1.2.x, as this section said until
ranger-base-pkqn measured it: any binary ≥ 0.51 does not read
`.beads/beads.db` at all (`bd list` → "no beads database found") and, on first
invocation, silently
`bd init`s an embedded Dolt DB under `.beads/embeddeddolt/` seeded from
whatever `issues.jsonl` says — i.e. a stale fork of the fleet's state, at the
last flush. `brew upgrade beads` on 2026-08-16 broke `bd` for every persona
session for ~3 minutes; rolled back to the upstream v0.49.1 release binary
(sha256-verified) at `~/.local/bin/bd`, with Homebrew's 1.2.2 left installed
but unlinked. Both this repo and the instance repo were on 0.49.1 SQLite and
are on 0.50.3 SQLite since 2026-09-01 — the same `.beads/beads.db`, untouched
by the swap; 0.50.x reads it unchanged.

`make verify-bd-pin` asserts that pin against the live box — version, which
binary `bd` actually resolves to, homebrew's keg unlinked **or** brew-pinned,
and every live `bd daemon` running the pinned binary and younger than it.
Read-only: it never kills, links or installs anything. See *"The pin is not
enforced"* below for why the process-layer half exists.

Two of those rows read differently on 0.50.3 than the paragraphs below
describe, and both are current as of 2026-09-01. **The process layer passes
vacuously**: 0.50.x has no `bd daemon` command, so there is nothing to
enumerate and the run prints "none running". It is left standing rather than
deleted — a layer that can no longer fire is a decision, not a cleanup, and
nobody has ruled (ranger-base-5xhmz, ranger-base-fyzqf). Read every
process-layer paragraph below as the check's history and its shape on 0.49.x.
**And the resolution row now accepts a posse gate shim** that `exec`s the
pinned binary (`GATED`, ranger-base-43v1); under `RHQ_PERSONA` that shim is
what `command -v bd` finds, and a row that FAILed on it was failing the
harness, not the pin.

Also present as a brew-managed pin: a local tap formula `beads@0.49.1`
(operator-side), which installs the same release tarball; `brew install`
of it currently fails only because the Command Line Tools are older than
brew wants. **It still names 0.49.1 after the bump** — it is a never-installed
formula for the superseded version, so it pins nothing today; the belt that is
actually set is `brew pin beads` on the 1.2.2 keg (see *"What it still does not
do"* below, corrected).

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
bd sync --flush-only && git -C <the queue repo> commit -m 'bd: flush before 1.2 migration' -- .beads/issues.jsonl
rm ~/.local/bin/bd && brew link beads         # 1.2.2 becomes bd
bd metrics off
bd init --from-jsonl && bd list | head       # embedded Dolt, seeded from JSONL
bd config set export.auto true                # keep issues.jsonl alive for git/metrics (recommended)
bd doctor --fix
```
Then re-audit `.claude/settings.json` (allow/deny) against `bd help`.
Rollback: `brew unlink beads && install -m 0755 <the pinned bd> ~/.local/bin/bd`
— **v0.50.3 since 2026-09-01, not v0.49.1**, and the rollback target is
whatever `etc/bd/version-pin.toml` declares, never a number typed from memory;
the SQLite `beads.db` is untouched by 1.2 and still valid.
The flush line lost a `git add` it never needed (ranger-base-nor): a
path-limited commit takes the WORKING TREE version of the paths it names and
ignores what is staged for them, so the add did nothing and the unqualified
commit it fed is one the shared-index wall refuses. If that line reports `no
changes added to commit`, the tree already matches HEAD — check with
`git diff --no-ext-diff HEAD -- .beads/issues.jsonl`, never `git status`, which
also counts a stale index entry (`docs/notes.d/ranger-base-nor.md`).

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
  `/opt/homebrew/var/homebrew/pinned`). A local tap pin for `beads@0.49.1`
  exists and is **not installed**.
- `make` carries `verify-grok-pin`, `verify-bd-dep-safety` and
  `verify-bd-no-relate-pairs`. There is **no `verify-bd-pin`** — the one
  substrate whose unpin has already taken the fleet down twice.

**What it cost, priced.** A burst of daemon `ERROR`s concentrated in one
hour — a ~40-minute storm, landing on the leading edge of the shop's densest
block, a busy stretch of local closes with the heaviest cluster near the top
of the hour. Nothing was lost; counts clean before and after. Priced out
against bead-segments run that day, the API-equiv spend and the median
per-bead cost were unremarkable. So the bill was the operator's Wednesday
evening and a degraded prime shift, **not dollars** — and 12d21h of not
looking is the number to fix, not the 40 minutes.

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
- `bd version` == the declared version, exactly — **0.50.3 as of 2026-09-01**,
  0.49.1 when this was written. The script reads it out of
  `etc/bd/version-pin.toml`; there is no version literal in the check. Asked as
  `bd --no-daemon version`, so the check never spawns the thing it is checking;
  a 1.x on PATH rejects that flag (0.50.0 deleted the daemon, 0.51.0 the flag —
  ranger-base-db04) and the row fails anyway. On 0.50.x the flag survives as a
  deprecated no-op, so the same argv keeps working across the bump.
- `command -v bd` == the pinned binary, **or a posse gate shim that `exec`s
  it** (`GATED`, ranger-base-43v1 — under `RHQ_PERSONA` the shim is what PATH
  finds). Not "a bd reporting the pinned version" — *the* pinned inode. A
  shadowing binary of the right version in front of it still fails, because the
  claim is about which inode the fleet runs.
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

MEASURED second (ranger-base-zk8v): `comm` is the path **as invoked**, not a
resolved one, so a daemon started from its own directory reports `./bd`. The
script `cd`s to the repo root at the top, so a relative comm tested with `-e`
there asks whether the *repo* holds that file — and a binary sitting on disk
was called ORPHAN, which is the wrong runbook (reap and restart, for a foreign
build that is still there to be identified). A relative comm is resolved
against the daemon's own cwd, which the cwd layer reads anyway; with no cwd to
resolve against, the verdict keeps only the half that still holds — a relative
path is not the absolute pinned binary — and says the rest is unknown. Never a
false negative on this arm either way: a relative path cannot equal `$want_bin`.

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

**Corrected 2026-09-01 — the belt is on.** `brew pin beads` was run on
2026-08-28 22:28 (`/opt/homebrew/var/homebrew/pinned/beads ->
../../../Cellar/beads/1.2.2`, and `brew list --pinned` names it). `brew info
--json=v2 beads` now reads `linked_keg: None, pinned: True`, so the keg is
unlinked **and** pinned, the check's row prints `pinned 1.2.2 ok`, and the
"NOT BELT-AND-BRACES" advisory has stopped firing. The paragraph above stands
as the state it described; the pin no longer rests on the absence of a symlink
alone. Verified by running `make verify-bd-pin`, not by reading this file.


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
- *What the upgrade would genuinely fix, and it is real:* **0.50.0 deleted the
  daemon** outright — one minor earlier than this line said until
  ranger-base-db04 read the source zips; `--no-daemon` outlived it as a
  deprecated no-op and went at 0.51.0. Every mechanism in this incident — a
  daemon auto-spawned per call, `daemon.lock`, an orphan holding FDs on a
  deleted inode — is 0.49.x-only and **cannot recur on anything from 0.50.0
  up**. That is the strongest thing on the migrate side and it should be said
  out loud, not buried. It also means the fix is reachable without leaving
  SQLite; see *the ~5.3s daemon dial is fixed upstream* below.
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
  recorded by monica 08-18). **Superseded 2026-08-30, executed 2026-09-01: the
  pin is 0.50.3.** See *"OPERATOR RULING 2026-08-30"* below. The bullet is left
  as written because its reasoning is what the ruling had to answer, and
  because its last sentence is the one that fired: the re-open condition was
  never "the next release announcement", and what re-opened it was a measured
  defect class the HOLD had not priced. Nothing since moves the arithmetic
  toward migrating and the vendor's August moves it away. The uncomfortable half,
  plainly: 0.49.1 is **permanently unsupported** — the SQLite line ends at
  0.50.3 with the `dep add` landmine byte-identical — so we are choosing to
  carry known defects with known workarounds over an untested migration. That
  choice is correct today and must be re-opened on the first *new* 0.49.1
  defect that has **no** workaround, not on the next release announcement.

### the ~5.3s daemon dial is fixed upstream — by deletion, at 0.50.0 (ranger-base-db04)

ranger-base-cwu7 measured every store-touching `bd` call at ~5.6s, ~5.3s of it
spent dialling a daemon that is not there, and fixed the posse half by putting
`--no-daemon` in front of every argv `rhq.Bd.run` builds. This is the half we
do not own: is it fixed upstream, and report it if not. **It is fixed, nothing
is owed upstream, and the way it is fixed changes the version arithmetic
above.**

**The defect, read out of the pinned source.** `cmd/bd/daemon_autostart.go`,
`startDaemonProcess()`: bd spawns `bd daemon start` as a child, reaps it in
`go func() { _ = cmd.Wait() }()` with the result discarded, then blocks in
`waitForSocketReadinessFn(socketPath, 5*time.Second)` — a loop that polls the
socket every 100ms and watches nothing else, not the child, not the error file.
The child writes its fatal reason to `.beads/daemon-error` and exits; bd reads
that file **after** the wait, only to print it. Measured 2026-08-30 on a
throwaway rig (a `.beads/issues.jsonl` holding one row plus `git init`, no `bd
init`): client starts 09:02:33.044, the daemon's own log records the
fingerprint refusal at 09:02:34.739, the client returns at 09:02:39.320. The
answer was on disk 4.6s before the client stopped waiting for it. The fix has
precedent in the same function — the `isGitRepo()` early return just above it
exists verbatim to "avoid the 5-second wait".

**Present through 0.49.6, gone at 0.50.0.** `daemon_autostart.go` and both its
`5*time.Second` waits are present in every 0.49.x source zip through 0.49.6.
At **0.50.0** that file and all of `internal/rpc/` are gone; the vendor's
CHANGELOG for 0.50.0 (2026-02-14) reads "Removed daemon/RPC subsystem —
internal daemon, RPC layer, and `internal/rpc/` package deleted (~19,663
lines). All commands use direct embedded database access". `--no-daemon`
survives it as a deprecated no-op (`cmd/bd/daemon_compat.go`: "kept so existing
hooks and configs don't break") and is itself deleted at 0.51.0. So this file's
earlier reading was one minor late: 0.51.0 deleted the *flag*, 0.50.0 deleted
the daemon.

**Measured, not only read.** The v0.50.3 release binary
(`beads_0.50.3_darwin_arm64.tar.gz`, sha256 matching the release's own
`checksums.txt`, extracted to a scratch path, run only against throwaway rigs,
never installed and never on PATH) against the pinned 0.49.1 on the same rig
shape, `bd list --all --json --limit 0`, three runs each:

```
bd 0.49.1, plain              5.66 / 5.52 / 5.40s
bd 0.49.1 --no-daemon         0.28 / 0.24 / 0.22s
bd 0.50.3, no flag at all     0.70 / 0.17 / 0.19s   (0.70 materialises the db)
```

Identical rows out of all three. 0.50.3 leaves a **SQLite** `.beads/beads.db`
and no Dolt directory, and it accepts posse's `--no-daemon` argv verbatim at
rc=0. The backend cliff is still 0.51.0, and 0.50.3's cycle-check CTE is the
pkqn landmine unchanged (`UNION ALL`, no visited set), so the three guards
built around that defect stay valid across the bump.

**Which opens an option this file had not priced: 0.49.1 → 0.50.3 is an
in-substrate bump.** It ends the whole daemon class — this dial, the per-db
daemon leak, the orphan holding FDs on a deleted inode — with no Dolt
migration, no JSONL demotion and no `bd init --from-jsonl`. It is not free, and
the first cost is load-bearing:

- **0.50.x does not auto-flush the JSONL after a write.** Measured on the two
  rigs: `bd create` on 0.49.1 takes `issues.jsonl` from 1 row to 2 with no
  explicit sync; the same create on 0.50.3 leaves it at 1 row until `bd sync
  --flush-only`, which still works and still exports. That is the CHANGELOG's
  "Removed JSONL sync layer — `internal/importer/`,
  `markDirtyAndScheduleFlush()`". JSONL-as-truth would then rest entirely on
  the pre-commit flush hook and on every writer calling sync — a thinner thread
  than today's.
- 0.50.0 also makes **Dolt the default backend for new `bd init`** (existing
  SQLite projects keep SQLite, and a bare JSONL rig still materialises SQLite —
  measured), so `bd init` becomes the dangerous verb one minor earlier than the
  section above says.
- `bd daemon` as a command is gone, so `make verify-bd-pin`'s version row and
  its whole process layer need rewriting, and the fleet's `Bash(bd daemon:*)`
  denies become no-ops.

**No upstream report was filed and none is owed** — the bead asked for one only
if the defect was still live upstream. Whether to bump the pin is the
operator's call, not this bead's: filed separately with these numbers.

### OPERATOR RULING 2026-08-30: bump to 0.50.3 (ranger-base-qrh1, executed 2026-09-01 ranger-base-8ufhn)

The numbers in the section above went to the operator as ranger-base-qrh1.
Ruling, monica 2026-08-30: **bump approved, 0.49.1 → 0.50.3**, prerequisites
first, and the swap itself — `etc/bd/version-pin.toml` plus `~/.local/bin/bd`,
one bundled step — stays the operator's hand. It supersedes the *"HOLD at
0.49.1 stands"* bullet above and rescopes the *"not worth taking"* line below.

**What actually landed, and it is not what the prerequisites list said.**
ranger-base-yeg1 was to land three things ahead of the swap; its session
reported all three and committed none (ranger-base-k77sk). Where they stand
today:

- *`verify-bd-pin`'s process layer, to be dropped:* **not dropped, still
  standing.** Rows 4 and 5 of `scripts/verify-bd-pin.sh` and the daemon-shaped
  cases in `bdpin_qa_test.go` are unchanged. On 0.50.x they pass vacuously —
  there is no `bd daemon` command to enumerate, so the run prints "none
  running" and the verdict rests on the command layer. Deleting a layer that
  can no longer fire is a decision, not a cleanup, and ranger-base-5xhmz and
  ranger-base-fyzqf both left it standing on purpose. If someone rules "drop
  it", the verify-contract rows above move with it.
- *The flush-discipline pin:* written in that worktree, stranded, and landed
  for real by ranger-base-fyzqf as `internal/posse/bdflushdiscipline_qa_test.go`
  (commit b175e28).
- *This subsection:* never written until ranger-base-dep6x, four days after the
  bump, which is why the section above it read "pinned at 0.49.1" the whole
  time.

**The swap, 2026-09-01 16:51** (ranger-base-8ufhn, monica, operator present and
ruling "stick with 50.3, do the swap now"; commit 291523c covers
`etc/bd/version-pin.toml`, README.md and INSTALL.md). The pre-check earned its
place: **one live 0.49.1 daemon was still running** — pid 2584, `bd daemon
start`, cwd the queue repo's own `.beads/`, two days old, reparented to init —
and was `TERM`ed cleanly before the binary underneath it was replaced. That is the
08-16 incident's exact shape, caught this time because the runbook said to look.
The binary came from the sha256-verified v0.50.3 `darwin_arm64` release tarball
(`26c15b93…d3d3a`, matching the release's own `checksums.txt`); the 0.49.1 it
replaced was copied aside to a scratch path, which is a convenience and not the
rollback source — that is the upstream release tarball, re-fetched and
re-verified. Verified in order: `bd version` 0.50.3; `make verify-bd-pin`
exit 0; `bd list` clean in the constitution repo and in a persona worktree; no
daemon respawned.

**What the bump cost, measured rather than predicted.** Ten tests went red on
`main` for the rest of that afternoon (ranger-base-5xhmz): `bdpin_qa_test.go`
asserted `posse_pinned_version = "0.49.1"` and `quickstart_test.go` asserted
INSTALL.md's v0.49.1 download URL, so the declaration moved and the pins that
assert the declaration did not. The lesson is the general one and it is cheap
to state: **a version literal that is a coupling to the pin file is not a
claim, and bumping the pin means bumping every literal coupled to it in the
same commit** — the check's own code has no version literal in it (it reads
`posse_pinned_version`), which is exactly why its tests did.

**Re-measured on the installed binary, 2026-09-01, not read from the vendor:**

- No `daemon` command exists — `bd --help`'s command list has none, and
  `--no-daemon` is there as `(deprecated) All operations use direct mode`, so
  every posse argv keeps working verbatim across the bump.
- A git repo holding a bare `.beads/issues.jsonl` and no database materialises
  **SQLite** (`beads.db`, `metadata.json`), not Dolt. The Dolt-default-init
  hazard is real and lands at 0.50.0 rather than 0.51.0, but it is a hazard for
  `bd init` on a repo with no `.beads/` at all — an uncommitted draft of this
  subsection claimed the bare-JSONL "fresh clone" shape produced Dolt too, and
  that claim is wrong on this binary. `bd init`, `bd import` and `bd migrate`
  stay in every persona's deny block, so the verb is fenced off from personas
  either way.
- **No auto-flush after a write, confirmed on a copy of the live queue db**
  (25MB, `.beads/` copied into a scratch git repo, never written back): `bd
  create` lands the issue in `beads.db` and leaves `issues.jsonl` at 1573 rows;
  `bd sync --flush-only` then exports it. Note that `bd --help` advertises a
  `--no-auto-flush` flag whose name implies the opposite — the flag is
  vestigial, the behaviour is what is measured here. JSONL-as-truth now rests
  on two carriers and no third: the launcher's own writer (`Bd.Flush`, held by
  `TestQueueCommitFlushesBeforeItCommits`) and bd's pre-commit hook, pinned at
  `internal/posse/bdflushdiscipline_qa_test.go`. A repo where that hook was
  never installed commits whatever the jsonl held at the last flush — named,
  not closed.

**HOLD at 0.50.3 now stands**, on the same terms the 0.49.1 hold had: 0.51+ is
a migration, not a version bump, and re-opens on a measured defect with no
workaround rather than on a release announcement.

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
reverse edge on the next relate.~~ **The second half of that sentence is
wrong — measured, ranger-base-nusr below: the pairs prune cleanly and stay
pruned as long as nothing re-plants one.** The first half is *not* wrong,
and an intervening correction that said so was itself wrong (measured,
ranger-base-uw8g): `bd dep relate` / `bd relate` write both rows in one
call, but two `bd dep add -t relates-to` calls in opposite directions plant
the identical pair — bd's cycle check does not consult direction, so the
second call is accepted. **Any single `-t relates-to` row is one bd command
away from being a pair.** What grows on its own, independent of all of this,
is the *unsafe target* set, as the preceding paragraph says.

**Standing rule: do not create `relates-to` edges in this fleet, and never
`dep add` onto an unsafe target — record the provenance as a comment
instead.** This is the same landmine as ranger-base-muoo from the other end:
`bd create --deps discovered-from:<parent>` starts the same CTE at `<parent>`,
so a poisoned parent loses its edge to the 30s timeout while the issue itself
commits — which is how 33 edgeless duplicate verify beads got filed. The
The HANDOFF rung of a dispatch prompt (`internal/posse/dispatch.go`) has
that exact shape. The ASK rung does **not**: its target is a freshly created
question bead with no outgoing edges, so the CTE is empty and returns at once.
Leave it alone. SPIKE used to have HANDOFF's shape and no longer does — see
below.

Since ranger-base-qbwt the ladder carries the caveat itself: a trailing
`Provenance:` line (not a seventh rung) tells the persona that the create is
two writes, to confirm with `bd dep list <new-id>`, to find the bead by title
rather than re-run a create that failed — the re-run is what files the
duplicate — and to record `discovered-from: <parent>` as a comment when the
edge is gone. It is a check-after rather than a preflight because the
safe/unsafe answer belongs to the graph at create time, hours after the prompt
renders, and because reading the graph back also catches a create that failed
for some other reason. ADR 0005 §2 has the reasoning; `verifyafter.go` is the
harness applying the same rule to itself.

Since ranger-base-k5fnr (2026-09-05) the SPIKE rung only files a separate
bead at all for a distinct dependency or deliverable — another lane's work, an
experiment needing its own venue, findings needing their own handoff — and a
bounded gap is researched in the deciding bead instead, with the findings
committed. The mandate it replaced was priced first:
`docs/notes.d/ranger-base-k5fnr.md` counts four separate spikes in the eleven
days the mandatory rung shipped, against 1,470 beads created, all four with a
distinct dependency or deliverable and only two carrying the block the rung
exists for. What follows is the mechanics for the spike that does get filed.

Since ranger-base-rs8j (2026-08-30) that bead is filed with **no**
`discovered-from` edge at all, and that is a different bd defect from the one
above rather than the same one. bd's cycle check spans *every* dependency
type, so a spike carrying `discovered-from:<deciding>` makes the
`bd dep add <deciding> <spike>` that rung exists for a cycle, in either
order (measured against real bd on a copy of the queue db; the harness's own
settle-open escalation had the identical pair, ranger-base-23oo). The block is
the deliverable, so the provenance is a comment on the spike. Note which edge
the caveat's check points at: reading the *spike* back shows a
`discovered-from` edge and looks fine even when the block never landed, so
SPIKE confirms `bd dep list <deciding>` and HANDOFF confirms `bd dep list
<new-id>`.

**The refusal belongs to the STORE, not to the bd version, and the silent
shape is worse (ranger-base-lpz0o).** Measured 2026-09-01 with one bd 0.50.3
binary against two stores, same argv, same process:

| store | `dep add <trigger> <blocker>` over a `discovered-from` edge | `<trigger>` in `bd ready` | in `bd blocked` |
|---|---|---|---|
| SQLite `beads.db` (the operator's queue) | refused, `cannot add dependency: would create a cycle`, exit 1 | yes | no |
| `bd init`'s store today (`.beads/config.yaml` → `no-db: true`, JSONL only, no `beads.db`) | **accepted**, err nil, edge in `dep list` | **yes** | **yes** |

The control arm is the same pair with no `discovered-from` edge: accepted in
both stores, `<trigger>` out of `bd ready` and in `bd blocked`, and back in
`bd ready` the moment the blocker closes. So the block itself works — it is
the mixed pair that does not, and the JSONL store fails it silently while the
SQLite one fails it loudly. Every "bd refuses a cycle" line in this tree was
written against the loud half and is now qualified in place; do not restore
one.

The harness stopped depending on either answer: **`Bd.Ready` is `bd ready`
minus `bd blocked`** (`beads.go`), so nothing dispatches a bead the store
itself calls stuck. That is safe against the obvious over-reach — closing the
blocker takes the bead out of `bd blocked` at once, measured in both stores
and both arms — and it costs one extra read, `bd blocked --json` at
0.13–0.17s against the 1551-bead queue db, the same as `bd ready`. A
`bd blocked` that fails makes the repo's queue *unknown*, not ready
(rangerhq-llse), so `Bd.Ready` returns the error rather than serving the raw
set.

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

**Corrected 2026-08-30, and the fleet moved on it 2026-09-01.** Read "not worth
taking" as scoped to the Dolt question this paragraph is about: 0.50.x buys
none of the 1.2 feature set, and on that question it is still worth nothing.
It was separately worth taking for what it *removes* — the whole daemon class,
with no Dolt migration attached — which is not "fixes nothing here". The pin is
0.50.3; see *"OPERATOR RULING 2026-08-30: bump to 0.50.3"* above. The v0.51.0
cliff in this paragraph is unchanged and is still where an existing SQLite
store stops being readable at all.

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

**Two corrections to the section above, both measured on the snapshot — and
one of them retracted below, so read ranger-base-uw8g before trusting either:**

- ~~`bd dep add <x> <y> -t relates-to` writes **one** row and is harmless. Only
  `bd dep relate` (and its deprecated alias `bd relate`) writes both
  directions. The poison has exactly one source verb, not two.~~ **WRONG,
  measured ranger-base-uw8g 2026-08-27: one call writes one row, but two
  `bd dep add -t relates-to` calls in opposite directions write both rows —
  bd 0.49.1 accepts the second unconditionally. The poison has one MECHANISM
  (a symmetric pair) reachable through two verbs, not one.** `Bash(bd dep
  relate:*)` / `Bash(bd relate:*)` in `.claude/settings.json` therefore deny
  only one of the two reachable paths; `bd dep add -t relates-to` is not
  denyable by pattern (every ordinary `dep add` shares its prefix), so the
  gate (`scripts/verify-bd-dep-safety.sh --gate`, wired up by
  ranger-base-z3s3) is the control, not the deny list.
- "The set only grows … it cannot be pruned back" is wrong, and it was the
  reason nobody tried. It cannot be pruned back only if someone keeps
  relating — **by either verb, not just `bd dep relate`**. Prune, then gate:
  `make verify-bd-no-relate-pairs` (`scripts/verify-bd-dep-safety.sh --gate`)
  exits 1 the moment any symmetric pair — of any type, planted by either verb
  — is back. What *does* grow on its own is the *unsafe target* set: an
  ordinary bead landing upstream of a pair joins it with no new `relates-to`
  edge, which is why gating on the pairs and not on the count is the durable
  check.

Fixed alongside: both scripts used to die with a bare `unable to open database
file (14)` against a WAL-mode db whose `-shm` is gone — the state bd leaves
after a write with no daemon holding the store. sqlite refuses a `mode=ro` open
there outright. They now fall back to reading a copy, which keeps "never writes
the fleet db" true. A checker that errors instead of answering gets ignored.

**All three were gated on the operator (ranger-base-kbus, "I approve all").
Where they landed:**

1. **APPLIED 2026-08-27, by the operator, projection committed at `f9894bf`.**
   The runbook was `scripts/prune-bd-relates-to.sh` (read the plan) →
   `--apply` from `$CONSTITUTION` → `bd sync --flush-only` → commit
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
posse owes instead is that a loss cannot stay quiet (`internal/posse/beadloss.go`):

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
D3-C already uses. The runbook moved to the instance tree (ADR 0024 D4;
one-deployment procedure) and the script it drives is
`scripts/queue-cutover.sh`; both were rehearsed on a
full copy of the live store (ranger-base-tjfw). What is worth knowing here
rather than there:

- **The launcher commits the projection.** While the store lived in the
  constitution repo, every commit anyone made carried
  `.beads/issues.jsonl` along — bd's own pre-commit hook stages it. A repo
  nobody commits in for any other reason has no such free ride, and `bd
  sync` exports without committing while `bd sync --full` commits *and*
  pushes (measured, 0.49.1). So a dispatch pass that judges a close flushes
  and commits it, path-limited, in the repo config `queue_repo:` names —
  `internal/posse/queuejsonl.go`. Absent key = no commits, which is what
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
**copy** of it, taken from a commit: `posse promote` (`internal/posse/promote.go`,
ranger-base-o943). The runbook moved to the instance tree (ADR 0024 D4;
one-deployment procedure). What is worth knowing here rather than there:

- **The promoted set is five paths** — `agents/`, `config.yaml`, `recipes/`,
  `runtimes/` (ADR 0039 D2, 2026-09-01: the per-key runtime overlay is read at
  every launch and decides a tier's model, so it is promoted prose too),
  `skills/` — and the exclusions are a symbol, not a sentence: `PromotedPaths`
  and `NotPromoted` in promote.go, with a test that reads both. `envs/`,
  `state/` and `personas/` are never created, copied or touched by promote,
  each for its own reason (§7 secrets with no commit behind them, machine-local
  state, persona memory).
- **A sentence that SPELLS the set out is not a reader of it** (ranger-base-b22vq,
  the drift ADR 0039 D2 left behind). The token widened the copy, the removal,
  the manifest, the launch verify, the seatbelt grants and the wall's path
  class in one edit — and left four shipped sentences naming four of the five
  paths, because each spelled the list by hand. The sharpest was the commit
  wall's own refusal: it refused `rhq/runtimes/claude.yaml`, printed the class
  line naming `rhq/runtimes`, and one line above explained the class with a
  list that omitted the path it had just refused. The other three were the
  `posse promote` help, the `posse init` stamp and the `posse gates`
  all-clear. Nothing in the suite asserted any of them, so the sweep and the
  suite both stayed green. All four now render `PromotedProse` (promote.go),
  and the pins (`internal/posse/promotedprose_qa_test.go`,
  `cmd/posse/promotehelp_qa_test.go`) read the shipped text BACK and measure
  it against a spec that is **written out** — a case list generated from
  `PromotedPaths` deletes its own case when a member goes and passes.
  Two traps worth carrying forward: the wall's refusal has to be read as a
  REASON SPAN, because the class line below it prints the staged path
  verbatim and a whole-message `Contains` is satisfied by the very line the
  reason contradicts; and the same shape bit the help pin for real — with the
  whole promote block as its subject it stayed GREEN under the mutation it
  exists to catch, because the removal sentence four lines down names
  `runtimes/` as its worked example.
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
- **And absence is now SAID, once per watch loop** (ranger-base-xevp7,
  `internal/posse/anchorstate.go`). The line above is why: a missing
  `promoted.json` verifies clean by design, so an anchor deleted by accident
  — a cleanup script, a botched restore — was invisible on every surface,
  forever. The `--watch` preamble now prints one read-only line beside the
  hook-wall sweep: `constitution: promoted <sha> <date>` / `seeded <date>` /
  `never promoted — no promoted.json`. It refuses nothing, degrades nothing
  and never fires on absence, and it claims nothing against a session that
  means it — one that re-stamps the manifest leaves a home that reads
  `promoted` here (ADR 0015 §3's tier-conditioned claim, ranger-base-zio33).
  What it buys is an ACCIDENTAL deletion visible at the operator's next
  touch point instead of never.
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
auto_update = false                    # kills the update check and the leader's self-update
maximum_version = "1.0.5"              # soft ceiling: the updater never INSTALLS above this
required_maximum_version = "1.0.5"     # hard ceiling: grok refuses to START above this
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

**The harder gate, set 2026-08-28 (operator ruling, rangerhq-iy3y).**
`required_maximum_version` is a *hard* bound: grok exits at startup above it,
where `maximum_version` alone only refuses to *install* — a gate with a hinge on
it, since a hand-run `grok update`, a managed-config push from xAI, or
`auto_update` flipped back on in `/settings` can all land a binary past a soft
ceiling and it will still run. The ruling is the same philosophy as the bd pin:
an unreviewed upgrade must refuse to start rather than run silently.

Probed live before it was applied, both arms, and the **config key** gates — not
only the `GROK_*` env override:

```
# required_maximum_version = "1.0.4" in config.toml, binary 1.0.5:
$ grok -p "x" -m no-such-model-xyz
This version of Grok (1.0.5) is newer than the maximum allowed by your organization (1.0.4).
Install an approved version through your organization's approved method …

# required_maximum_version = "1.0.5": starts, and fails later on auth/model —
# past the version gate. Lowering only the SOFT ceiling refuses nothing.
```

**Only the agent path is gated**: `--version`, `update`, `inspect`, `doctor` and
`models` all run normally above the ceiling, so an out-of-range install stays
diagnosable and recoverable (`grok update --version 1.0.5` is allowed above a
ceiling by design, and the old binary is still in `downloads/`). So the failure
is loud, self-describing — grok names the ceiling in the refusal — and about two
minutes to undo.

**Know the blast radius before you raise the binary**: the moment grok is out of
range, every dispatched grok pane *and* the operator's own interactive grok stop
starting, at once. That is the gate working. Recovery is
`grok update --version 1.0.5`, or the symlink rollback below.

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
# then raise ALL THREE in etc/grok/version-pin.toml and ~/.grok/config.toml:
#   posse_pinned_version / maximum_version / required_maximum_version
# (raise required_maximum_version FIRST or grok will not start on the new build)
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
   (today: none — grok's splash was the only entry, retired in
   `rangerhq-6723` once detection stopped calling it a blocker). Any
   future table presses each key once, **Enter is not in the table**,
   and `TestDispatchPathPressesNoKeys` is edited in the same commit
   that revives it (ADR 0013 §2, amended ranger-base-xqft).

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

**What enforces it, since `ranger-base-9r33`.** For eleven months that rule
was a sentence in three documents and nothing in the code: `Interstitial.Danger`
was read by `posse runtime check` and by nobody on a launch path, so codex
launched onto the menu the contract said must gate it. Three surfaces now
agree, all reading `DangerUnsilenced` (`internal/posse/interstitial.go`):

- **dispatch refuses above the claim.** `launchSession` asks before it
  creates a session, so a refused bead is never claimed and no workspace is
  made — the argv path claims *first*, so a refusal raised any later would
  hand back a bead it had already taken. The refusal is a persona/runtime
  failure by the §2 busy-key split, so the slot is benched for the pass
  rather than the refusal being printed once per bead.
- **every other launch path refuses from `planLaunch`** — the cockpit's `d`
  on a session it must create, a recipe, a relaunch — *if it carries a
  bead*. An **interactive** launch warns `DEGRADED` and proceeds. That is
  ADR 0015 §3's asymmetry, and here it is not merely analogous: the remedy
  for codex's menu is to **answer it**, in a codex session, so a posse that
  refused interactive launches too would have walled off the only way to
  clear its own refusal. It is also the escape hatch, and the reason there
  is no config key for one.
- **`posse runtime check` reports it as a BLOCKING gap**, and `posse runtime
  probe` refuses to probe — the probe launches a scratch session, which
  would meet the same screen.

**A refusal is a reading, never ignorance.** The probes are tri-state
(`Silence`: silenced / not silenced / **unknown**), and only the middle one
refuses. posse cannot read `~/.codex/version.json` on a box codex has never
checked for a release on, and a refusal whose own words are "cannot tell
whether the update menu is silenced" walls a box for something nobody
measured. The screen is not unguarded meanwhile: herdr names it `blocked`
by its own `update_menu` rule, so a launch that *does* meet it fails by name
instead of being typed into.

**A declared screen with `danger:` refuses too** (ranger-base-vbp3). An
interstitial declared in a `runtimes/<name>.yaml` has no probe at all, and
9r33 read that as "declaring a screen documents it and never walls the
declarer's own launches" — which made the whole rule unreachable for the
only runtimes that can newly meet it. The built-ins are measured, claude's
screen is `Seeded` and codex/grok deliver by argv, so the **first
typed-delivery runtime with a machine-mutating dialog is by construction a
declared one**, and it was dispatched onto that dialog while the grid
printed LAUNCH REFUSE about it. `danger:` is not posse guessing at a config
it cannot parse — it is the operator's own written statement that the
default action mutates their machine, so refusing on it is still a reading.
Declaring it is choosing the wall; a declared screen **without** `danger:`
walls nothing. The refusal does not lift by silencing the screen, because
posse cannot read the key: what lifts it is dropping `danger:` from the
profile, and the refusal line and the grid both say so.

**The keys, per runtime.** `posse runtime check <name>` prints each with
its file and whether it is silenced on this box — read-only probes, which
is the only thing posse does to these files.

| runtime | screen | key | who silences it, and how |
|---|---|---|---|
| grok | `Help improve Grok  [Opt out] [Opt in]` consent banner above the composer | `[privacy] privacy_banner_acked` in `~/.grok/config.toml` | the **operator**, clicking `[Opt out]` once in their own grok session. The value is an RFC3339 stamp, not a bool, and it records only *that* the banner was answered — never which way. In 1.0.5 the consent RPC has no server handler, so even an accidental `[Opt in]` cannot persist; that defense is version-verified and evaporates the day xAI ships the handler (`rangerhq-sz7u`). |
| grok | New worktree / Resume session / Quit startup menu, plus the changelog line | `[cli] auto_update = false`, `maximum_version` in `~/.grok/config.toml` | **already applied** — the fleet pin, declared in `etc/grok/version-pin.toml`, kills the update check *and* the shared leader's mid-life self-update. `make verify-grok-pin`; runbook in *grok substrate* above. |
| codex | `Update available! → 1. Update now  2. Skip  3. Skip until next version` | `check_for_update_on_startup = false` in `~/.codex/config.toml` (declared in `etc/codex/version-pin.toml`) | **nothing — already handled by the fleet pin**, which stops the menu being drawn at all. `make verify-codex-pin` asserts it, together with the `brew pin --cask codex` that makes `1. Update now` *fail* rather than upgrade. Without the pin there are two silences and both expire: picking **3. Skip until next version** (arrow **Down** twice, *verify the caret moved*, **then** Enter), which lasts exactly one release, and being at the latest release already, which lasts until the next one ships (ranger-base-cohw). |
| claude | `Quick safety check: Is this a project you created or one you trust?` — full screen, `1. Yes, I trust this folder / 2. No, exit`, footed `Enter to confirm · Esc to cancel` | `projects["<session dir>"].hasTrustDialogAccepted` in `~/.claude.json` (or `$CLAUDE_CONFIG_DIR/.claude.json`, or the config dir's `.config.json` where that exists) | **the LAUNCH**, per session directory — the one exception, below. |

**The codex dismissal has a shelf life; the fleet pin does not
(ranger-base-poj5).** `dismissed_version` silences one release: the menu
returns the moment `latest_version` moves past it. That is why `runtime check`
prints both numbers rather than a bare yes — a probe whose answer expires
should say when. It is also why the probe reads
`check_for_update_on_startup = false` *first*: with the startup check off,
what `version.json` says is not a reading about any screen the operator will
meet, and on 2026-08-30 the expired dismissal alone refused every codex
dispatch on this box. codex has no version-ceiling key to pin with — the
declaration, the measurement and the runbook for lifting it are in
`docs/notes.d/ranger-base-poj5.md`.

**A box already at the latest release is silenced too, and reading only
`version.json` never noticed (ranger-base-cohw).** Those two fields answer
*did the operator dismiss THIS release*, not *will the menu draw*: codex
offers an update only when a release **newer than the running one** exists,
so an operator who **updated** instead of dismissing read un-silenced
forever. MEASURED 2026-08-29 on codex-cli 0.150.1 against
`{"latest_version":"0.150.1","dismissed_version":"0.149.1"}` — the probe said
"the menu is back" while a peeked launch pane carried no `Update available`
and herdr read both live codex sessions `idle` rather than `blocked` on its
own `update_menu` rule. ADR 0013 §2 turns that reading into a launch refuse,
so the most up-to-date box was the one that could not launch. The probe now
asks codex what it is running (`codex --version`, the same reader the parity
drift check uses) as a **third** arm, after the two that cost no subprocess.
It stays honest the other way too: a codex that is not there, or will not say
what it is, reads UNKNOWN and refuses nothing.

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

So the launch seeds it (`SeedClaudeTrust`, `internal/posse/trust.go`), which
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
- **`.claude/settings.local.json` is the same channel, not a safer one**
  (measured 2026-08-28 on 2.1.251, a fresh `CLAUDE_CONFIG_DIR` per arm and
  `ANTHROPIC_BASE_URL` on a dead port, no API turn — rangerhq-9u8). A
  `SessionStart` hook declared only there ran before the first turn and
  before the CLI resolved credentials at all: the run ended on "Not logged
  in - Please run /login" with the hook's witness file already written,
  while the same dirs with no settings file ran nothing. The fleet's own
  `--settings` JSON suppresses none of it — `--settings` adds a source,
  hooks merge across sources, and there is no flag posse can type that
  closes this channel. The trust gate above is one early return per hook
  *event*, taken before any source is consulted, so it covers this file
  identically. Gitignored is not a security property: whoever can write the
  repo can write that path, and `git status` will not show it afterwards.
  Hence both files are in the check (ADR 0002 amendment 2026-08-28).

None of that moves the launch check, and it makes it *more* load-bearing:
the check fires on settings content, so it does not depend on which of
claude's gates is holding, or on the docs and the binary agreeing next
release. Claude declares those settings paths with
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

## herdr substrate: upgrading the fleet's herdr

The step-by-step upgrade procedure for **this** deployment — pre-flight,
parking the armed dispatch loop, the handoff command and its failure prompt,
post-flight, and rollback — is an instance procedure, not any-deployer
mechanism, so it lives in the instance tree and not here (ADR 0024 Decision 1,
genre arm; operator ruling on `ranger-base-vbcs`, 2026-08-28). It was public
until then; git history keeps that copy. Private path label:
`docs/runbooks/herdr-upgrade.md` in the instance repo. Origin beads
`rangerhq-0ao` / `rangerhq-7t4` / `rangerhq-61u`.

What is any-deployer's about it is already stated in this file and stays:
`[[startup]]` hooks fire on a live handoff too and the loop must be asked, not
the workspace ("One loop, and the husk problem"); metas key on `workspace:` +
`socket:`, both of which a handoff preserves; and the id-recycling fence
below, which is what the runbook's post-flight leans on.

## Workspace ids recycle across a server process boundary (rangerhq-6bg7)

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

A live handoff does preserve pane and workspace ids — exactly, and more
narrowly than that reads: ids survive for workspaces that are *live* at the
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

**What landed** (rangerhq-yt1p, `internal/posse/herdrback.go`). `gen:` in the
session meta — `dev:inode` of the api socket, stamped at create, backfilled
on positive identity — plus one predicate, `notOurWorkspace`, asked by all
three destructive paths through `idEvidence`: the prune (`Sessions`), the
create (`mustNotOrphan`, which `posse relaunch`'s unlink also calls), and the
listing itself. The listing is where the damage actually was: a stale meta
whose id a stranger now holds used to be listed *over* that workspace, and
every addressing path (`Resolve`, `AgentTarget`, `KillSession`,
`RelaunchAgent`) reads that listing — so the name prompted into somebody
else's pane and `posse kill` closed it. Such a meta is now kept, left out of
the listing, and reported with the repair recipe (the instance runbook's
post-flight carries it).

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
path-limited form commits only B's path and A's
`git diff --no-ext-diff --cached` still shows its own staged file afterwards.

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
(`internal/posse/worktree.go`), and the merge-back is the operator's option A
(rangerhq-jbyr): **the launcher merges**.

What a dispatched session gets:

| | |
|---|---|
| main checkout | `~/src/<repo>` — the operator's, on `main`, unchanged |
| session worktree | `<worktrees root>/<repo>/<session>` — its own tree, index, HEAD |
| session branch | `posse/<session>`, cut from the repo's own branch |
| merge-back | the launcher fast-forwards it onto that branch when the bead closes, under the ADR 0011 §1 launcher lock |
| retirement | `posse kill` lands the branch then removes tree and branch — and REFUSES while either holds work. Since ADR 0058 the landing sweep also retires a tree whose bead is closed, whose work is measured on the base, whose session herdr proves gone, and whose git dir has not been written inside `retire_tree_after:` (1h) — the four facts read fresh under the launcher lock; `posse worktrees --retire` runs the same predicate on demand |

The kill takes the launcher lock **without waiting**: the cockpit's `k` runs
it on the TUI's single select loop, and blocking there behind a firing pass
freezes the cockpit. Losing that race costs only time — the workspace is
closed, the tree and branch are kept, and the line says
`posse worktrees --land` finishes it. `--land` merges every landable branch
under one blocking lock and **never removes a tree**: it reads git, so it
cannot tell a dead session's tree from one a persona is working in this
second, and removing the second is the exact damage this feature exists to
prevent. That sentence was read for two weeks as "trees are permanent", and
MEASURED 2026-09-05 it had made them so: 70 trees standing, 8 with a live
session, 38 dead and fully landed (36 of them by plain fast-forward — the
kill path is the only remover and a workspace herdr lost never reaches it).
ADR 0058 gives the retire to the one site that holds more than git — the
landing sweep reads the bead fresh, asks herdr, and holds the lock — under
the safe-reclamation rule (proof of death at reclaim time plus a grace over
tree writes); `--land` itself stays exactly this paragraph, and
`--retire` beside it is the human's run of the same predicate.

`posse worktrees --retire` runs those four facts over every tree under one
blocking launcher lock and prints one line per tree — `⌫ … retired: <why it
was safe>` or `◑ … kept: <the fact that failed>`, including the two keeps the
unattended sweep is silent about (a tree inside its grace, and the dial
turned off): a pass keeps those quiet because 36 of them per pass forever is
how a board stops being read, and a person who just asked is owed an answer
about every tree. **It takes no `--force`**, and `posse worktrees --retire
--force` is refused as an unknown flag rather than accepted and ignored —
`--force` stands down the one refusal that exists to say no while something
would be lost, and it stays the two-command hand recipe that refusal prints,
typed at one tree by somebody who has just read why.

The plain listing says the same verdict without acting: a third line per tree
that reads `retirable — the next pass takes it`, or `kept: <the fact>`, or —
for a tree no bead record accounts for — ADR 0006's sentence, because that
tree is not the predicate's population at all and no pass will ever take it.
It replaces `a human can retire the tree`, which every one of these surfaces
used to end with and which nobody had ever acted on.

`--land` also **reads the branch record before it merges**. A tree holding
commits its base does not have, whose branch names no bead
(`branch.<branch>.posseBead`), is reported and skipped; `--land --force`
lands it anyway. From git alone that tree is indistinguishable from one whose
work already landed under another bead id — measured in the field, one commit
byte-identical to something on main that no patch-id and no `-x` trailer
connects to it — and merging it blind replays stale work onto the operator's
branch. `posse worktrees` names the bead beside the count for the same
reason, so the judgement is available before the command is typed:
`docs/notes.d/ranger-base-atxe.md`. **"Holding commits its base does not
have" is asked of the tip the work is on, not of the branch**
(ranger-base-qihvt): `<base>..<branch>` is zero over a detached tree, so the
gate saw nothing to guard over the whole of a caged session's work while the
merge behind it spliced that work onto the branch and landed it — ADR 0006's
rule waived by asking the wrong tip, the same blindness ranger-base-vavx2
fixed in the sweep's copy of the question. The branch is asked first, because
the refusal's `git log <base>..<branch>` is true only of a commit on the
branch; when the branch has nothing the tree's HEAD is asked, and the refusal
names that HEAD and hands you a `git log` in the tree. An *accidental* detach
is left alone: no splice moves that work, no land can take it, and the merge
already answers it in a sentence carrying the `git branch -f` cure.

**Both surfaces ask the TREE's tip, not the branch's, and both ask patch
equivalence** (ranger-base-d8o6). A container-tier session works on a
detached HEAD by design, so its commits are on no branch and
`<base>..<branch>` counts zero over the whole of its work — `posse worktrees`
read that as "nothing unlanded", its one phrase for a tree that is safe to
retire, while the merge on the same tree said the work was on neither the
base nor the branch. The listing now asks `workHead` like every other surface
here, and prints the `git branch -f` that puts the work back when it is off
the branch. The merge had the mirror of it: on the path taken when the branch
never moved it answered by ancestry alone, so a detached tree already picked
onto the base was called unreferenced. Both answer in the same three words
now — settled, recorded but not measured, or unlanded — and the pin asserts
they AGREE about one tree rather than checking each sentence apart.

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

A blocked merge's work is **pinned under `refs/posse/merge-blocked/<branch>`**
(ranger-base-m3195). A merge-back block is a bead that outlives the reading it
was filed from: the tree is retired as soon as the merge stops being refused,
and `RemoveSessionTree`'s refusals hand the operator a `worktree remove &&
branch -D` to run by hand — so the branch can go between the filing and the
dispatch. It did, twice: `ranger-base-g7br6` and `ranger-base-nr3eq` were both
worked against a deleted branch and a worktree path that no longer existed,
with their beads still saying *"The branch is untouched and still holds every
commit."* The commits survived as objects reachable from **no ref**, alive only
because nothing on this box runs `gc --prune` on a schedule. A ref is the fix
rather than a re-check at dispatch, because a re-check only narrows the window
where a ref closes it. The bead names the pin and the sha and tells the seat to
read the branch before believing anything above it; the pass drops the pin once
no OPEN block names that branch (`prunePinnedBlocks`, run at pass start off the
repo — by then the tree is gone and the session walk cannot reach it), and a
store that will not answer leaves every pin standing.

The promise is gone from **every** refusal, not only the one that was pinned
(`ranger-base-eq3ba`). `noteMergeBlocked` embeds `o.Reason` verbatim, and
`MergeSessionWork` has a dozen spellings of it: two of them — the constitution
refusal and the replay-exhausted arm — still said *"<branch> still holds every
commit"* after m3195 closed, in sentences carrying no "untouched" for its pin
to grep. Each now reports what the launcher DID ("nothing was landed"), which
stays true however long the bead waits, and
`TestMergeBlockedReasonsNeverPromiseTheBranch` drives all twelve arms and
asserts the CLAIM rather than one word — each with a substring only that arm's
sentence carries, so a fixture that fell through to another refusal reds
instead of passing for an arm nobody drove.

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
- **CORRECTED 2026-08-28 (ranger-base-vczf): a redirect-less linked worktree
  does NOT fork the graph.** This bullet used to say it did — that bd would
  read the checked-out `issues.jsonl`, report "fresh clone detected", and
  build a second database beside it, and that preventing that was what the
  seeding was for. Re-measured on bd 0.49.1 and it does not hold. **bd
  resolves a linked worktree to the MAIN checkout's `.beads` itself**, and
  while the main checkout has one it does not read the worktree's `redirect`
  at all — a redirect pointing at a different LIVE database is ignored and bd
  goes on reading the main graph. The "fresh clone" shape does not fork
  either: with a tracked jsonl and no database yet, bd builds `beads.db` in
  the MAIN checkout's `.beads` and the worktree reads it. bd falls back to
  the worktree's own redirect only when the main checkout has no `.beads` at
  all, which is the one shape `seedBeadsRedirect` declines to write for.
  `TestLiveWorktreeBdResolvesTheWorktreeItself` pins that, so the day it
  changes is a red test.
- **So the redirect is read by posse, not by bd.** `beadsHome` (beadloss.go)
  resolves it, and the seatbelt writable set and the codex launch line are
  built from what it answers (ADR 0012 D3-C). With no redirect
  `beadsHome(tree)` answers the worktree's own `.beads` — a directory bd
  never opens — so a caged persona is granted the wrong path and denied the
  right one, and bd reports `failed to open database: … operation not
  permitted` out of a resolution that was correct all along. That is
  ranger-base-0fb verbatim. The redirect is not belt-and-braces for the graph
  on today's bd; it is the cage's only account of where the store is, and it
  stays the belt for a bd that loses the worktree resolution. The full
  five-row table, and what it does to the pin, are in
  `docs/notes.d/ranger-base-vczf.md`.
- **The redirect posse writes is ABSOLUTE.** bd's relative form resolves
  against the worktree ROOT, not against `.beads/` — one `..` off and bd warns
  once and silently falls back to a stale path. An absolute path has no such
  arithmetic to get wrong, and it resolves the main checkout's OWN redirect
  first so a chain is never built (this repo's `.beads` is itself a redirect
  to ranger-base's database).
- **A store sitting BESIDE that redirect is now named on two surfaces**
  (ranger-base-dj3k2, `internal/posse/secondstore.go`; ADR 0012 D3, the
  September 2026 adherence audit's finding 6). D3 bought one store of record
  and rejected "a gitignored *local* `.beads/` inside the public tree" by
  name; the audit found one anyway — a `beads.db` from 2026-08-24 beside the
  redirect, its `-shm` file touched that morning. Nothing was wrong that
  day, because `beadsHome` and bd both resolve the redirect first. The day
  the redirect file is lost, that store answers `bd ready` at exit 0 with a
  three-week-old graph, and a wrong graph that exits 0 is the whole failure:
  the loop dispatches, the beads are stale, every surface reads as a working
  shop. `posse status` and each `dispatch --watch` pass now print the same
  line naming the path, what is in it and the fix. **It reports** — no
  refusal, no delete, no exit code: the remedy is one `rm` the operator
  types, and posse deleting a database nobody asked it to delete is worse
  than what it prevents. A `.beads/` holding its own database and no
  redirect is every ordinary bd repo and says nothing; a redirect bd will
  NOT follow takes the other sentence, because there bd is already reading
  the local store. `beadsRedirectHop` (beadloss.go) is the one reader both
  the census and this guard project from, so they cannot disagree about
  which directory bd is in.
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
  **UPDATE (ranger-base-p969)**: the "no `bd sync --import-only` left sitting
  as a persona's obvious next step" line above is still right — that stays a
  human's call, never a suggested command — but it turned out to describe
  the wrong target for the same recovery run *programmatically*, inside
  `Bd.run` (beads.go). Measured 2026-08-30: a `--no-daemon` dispatch pass
  refused the ready scan on this exact message nine times in twenty minutes,
  triggered by *any* daemon-path bd write from another actor in the
  preceding ~10 minutes, and a `sync --import-only` run by hand right after
  each failure reported "0 created / 0 updated" every time — the refusal is
  a timestamp/marker check, not a content one, so a daemon flush that
  rewrites `issues.jsonl` without changing it still trips it. `Bd.run` now
  imports once and retries once on this one message before giving up;
  `WarnLostBeads` (beadloss.go, rangerhq-fuom) is unchanged and remains the
  backstop for the case this staleness check exists to catch — an import
  that silently drops rows.
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
- **Put session worktrees under `$HOME` — bd holds a net, but a partial one.**
  This note has been wrong twice in opposite directions: first that bd refuses
  a `.beads` under `/tmp`, then (rangerhq-80fx) that it holds no such net at
  all. What is measured on bd 0.49.1 — throwaway repo under `$HOME`, a `$HOME`
  control on every arm (ranger-base-9ypc) — is in between, and the second
  version is the one that fails dangerous.
  **The check is real and every tmp path fails it.** `FindBeadsDir` puts
  `BEADS_DIR` through `CanonicalizePath`, which `EvalSymlinks` it, *before*
  `isPathInSafeBoundary` judges it — and on macOS `/tmp` resolves to
  `/private/tmp`, whose `/private` **is** in `unsafePrefixes`. The
  `os.TempDir()` branch does not admit `/tmp` either (`os.TempDir()` here is
  `/var/folders/…/T`). So `BEADS_DIR=/tmp/… bd worktree create` fails with
  "BEADS_DIR points to unsafe location: /private/tmp/…", `/var/tmp` likewise,
  while the `$HOME` control creates the worktree.
  **But only the ~50 commands that call `GetRepoContext` ever ask.** `bd list`
  and `bd status` accept a `BEADS_DIR` under `/tmp`, `/private/tmp`, `/var/tmp`
  and even `/etc` — the check is unreached, not satisfied. And `bd worktree
  create /tmp/<name>` (no `BEADS_DIR`) **succeeds and writes a redirect that
  does not resolve**: the relative path is computed from the unresolved `/tmp`
  target while the tree lands at `/private/tmp`, one component deeper, so
  `../../../Users/…/repo/.beads` resolves to `/private/Users/…` and bd's own
  `worktree list` warns "redirect target does not exist or is not a directory".
  The `$HOME` control writes `../repo/.beads` and warns about nothing. Nothing
  notices sooner because `FindBeadsDir`'s worktree branch reaches the main repo
  through git, never the redirect — so the breakage is silent until something
  reads that file.
  `$HOME` is therefore **still ours to enforce**: because the net is partial
  and silent exactly where it leaks, not because it is absent. The reason
  stands on its own feet either way — a session scratchpad is reaped, and a
  reaped worktree under a live session is exactly the failure. `WorktreeRoot()`
  refuses a configured root outside `$HOME`, resolving symlinks first.
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
into the one **exact** shape that is unsafe whatever follows —
`Bash(git commit)` — and nothing longer, because anything longer might be
the safe form. It used to carry a second, option-blind shape,
`Bash(git -* commit)`, with rangerhq-ky3's false positive attached
(`git -C <r> log --grep commit` refused, `--grep=commit` ran); unlike the
verb branch, this one had no ` *` half to fall back on — a trailing
wildcard would also have caught the safe form. ranger-base-xll2 applied the
rangerhq-3mc/ky3 standard (a false positive is a hard block the model
cannot ask its way past) and dropped it: `git <globals> commit -m x` with
no `--` now draws no polite L0 refusal, only L1's hard one. The fallback
did not save the verb branch either — rangerhq-vr6j found the ` *` half
matching any later `push` token, and it went the same way — so both
branches now agree: L0 says the spelling the PID wrote, and nothing about
what may sit in front of it.

*L3, the hook — and the slot is `prepare-commit-msg`, not `pre-commit`.*
Two measured reasons. `pre-commit` is bd's, bd reinstalls it, and a wall a
third-party tool silently replaces on its next install is not a wall. And
`git commit --no-verify` **skips `pre-commit` while `prepare-commit-msg`
still runs** — so the free slot is also the stronger one. Both hooks see the
same `GIT_INDEX_FILE`, verified across all four forms. The guard is **not**
keyed on `RHQ_PERSONA` — it covers every shell in the shared checkout, the
operator's own included (rangerhq-lt2w; it was keyed that way, the way the
pre-push gate keys on `RHQ_TOOLS_DENY`, until the exemption was retired). It
is installed at every persona session create and by `posse gates install-hooks`, and is *not*
keyed on a deny rule, because what makes the commit unsafe is the tree the
session was dispatched into, not anything the PID says. Session create then
executes a persona probe against the slot and counts L3 only when the hook
refuses with exit 1; the same single shell invocation probes pre-push when
that PID denies it. A failed commit probe degrades every persona launch into
the repo because this slot carries both the shared-index wall and the beads
visibility guard.

Commits git drives itself are let through **when git leaves a marker to
see** — merge (`$2` = `merge`), cherry-pick, rebase, squash, and a revert
being finished by hand all write one before `prepare-commit-msg` runs, and
during a conflicted merge git refuses a pathspec outright ("cannot do a
partial commit during a merge"), so refusing them would leave no way through
rather than a safer one. `git commit --amend` is not one of them; it takes a
pathspec and sweeps without one, so it is refused.

**A clean `git revert` is not one of them either, and it is refused
(rangerhq-lrnp).** Measured on git 2.39.3: it writes no `REVERT_HEAD`, no
sequencer, no `GIT_REFLOG_ACTION` before the hook runs — `$2` is `message`
and `GIT_INDEX_FILE` is `.git/index`, so at this slot a clean revert is
*indistinguishable* from `git commit -m`. The two signals it does leave are
both unusable as an exemption, and each would take the wall down silently
rather than narrow it: `AUTO_MERGE` outlives the revert that wrote it (still
there for the next plain commit — pinned in the tests), and `MERGE_MSG`
outlives a revert this hook refuses. Widening the `case "$2"` arm to
`revert` is worse still: `$2` is `message`, so that arm would wave every
unqualified commit through. The way through is named in the refusal instead,
and it is two steps:

```sh
$ git revert --no-commit <sha>
$ git commit -F - -- <the paths it touched>
```

The second command needs no exemption — a path-limited commit gets its own
`next-index-<pid>` temp index even mid-revert, so it passes on its own
merits. And because git stages the revert *before* the hook can refuse it,
the refusal names what is sitting in the shared index and the path-limited
undo (`git restore --source=HEAD --staged --worktree -- <paths>`, never
`git reset --hard` in a shared tree). That dirt is bounded: `git revert`
only starts from an index matching HEAD, so what a refusal leaves staged is
the revert and nobody else's work. The hook does not clean it up itself —
a hook running `git reset` behind a persona would be the destructive act the
wall exists to prevent.

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
| `git commit -F - -- <mine>` | commits only `<mine>`; the other persona's staged entry is still in `git diff --no-ext-diff --cached` afterwards |
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
form — verified against the live hook, with `RHQ_PERSONA` set and with it
unset. The table's third column is the half rangerhq-lt2w closed: springing a stale
index takes an *unqualified* commit, the operator's hand-typed `bd sync:` was
exactly that form and exactly the shell the wall exempted, so the whole chain
reproduced end to end from it (fix lands, index stale, next `bd sync` reverts
it). No shell in the checkout makes that commit now. Pinned in
`TestSharedIndexCommitHookRefusesHandRolledNextIndex` (every private-index
spelling refused with no persona in the environment) and in the residual test
named below.

**That was true of the recipe's spelling before it was true of the recipe
(rangerhq-cqq1).** As written on 2026-08-22 this paragraph said the class was
"out of the crew's reach"; it was one filename wide — `GIT_INDEX_FILE` ending
in `next-index-anything` walked straight through, and the whole chain
reproduced under a persona. Fixed by matching git's temp index by location and
pid shape (see the discriminator note above), landed with a mutation-tested pin
in `TestSharedIndexCommitHookRefusesHandRolledNextIndex`. Read the sentence
below as true from that fix forward, and as **false for anything measured
before it**.

**The residual is one form, and it no longer reaches the end of the chain
(rangerhq-lt2w).** The operator *was* the second form — exempt from the wall
by design, with `bd sync:` commits that are exactly the unqualified shape —
and the operator ruled that exemption away on 2026-08-28: the wall now covers
every shell in the checkout, and `bd sync` itself never committed anything
("Does NOT stage or commit - that's the user's job", bd 0.49.1), so the fix
was the wall, not bd. What is left is the private index placed *inside*
`$GIT_DIR`: `GIT_INDEX_FILE=$GIT_DIR/next-index-1` passes, because the
discriminator asks where git puts its temp index rather than what the variable
is spelled (see the discriminator note above, and its residual stated there).
That is a gate working as specified, not a hole to plug — and it still MAKES a
stale shared index under a persona who chooses it deliberately. What it no
longer does is SPRING one: the only form that restages every path from the
shared index is the unqualified commit, and there is now no shell that gets to
make one here. Kept measured rather than asserted:
`TestQAPrivateIndexInsideTheGitDirIsTheMeasuredResidual`
(`internal/posse/privateindex_qa_test.go`) lands a fix through
`$GIT_DIR/next-index-1` with `RHQ_PERSONA` set, pins the shared index at the
pre-fix blob afterwards, pins the follow-up `bd sync:` commit being REFUSED
with no persona in the environment and HEAD keeping the fix, and carries the
control that the path-limited way through does not revert it either. It goes
red the day the making half closes too, and this paragraph can be rewritten
then.

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
matches only, so a *partial* revert of some hunks is invisible — it sees only
what was committed, never work reverted in the working tree before it landed,
and the deletion rule needs the path's add inside the scanned range, so a short
`a..b` range under-reports.

**The move exception asks git, at a chosen 50%** (ranger-base-en75). It used to
be exact-blob, so a rename that *also edited* the file was reported as a
deletion and cost a triage line — three commits in ~460 paid it (631bda7,
e82338c, 2eae58a) and the rate was not falling. `raw_log` passes
`--find-renames=50% -l0` now and the state machine decomposes git's `R` line
into the two entries it stands for: the source leaves (excused) and the
destination arrives (still compared, so a re-land *through* a rename is still
caught). The number is chosen, not inherited: the two live strikes measure R060
and R097, so 60% is the tightest value that clears both — zero margin — and over
this repo's 504 commits git pairs exactly three deletions with an add even at
`-M30%`, all three real renames. The cost is a false-*negative* widening: a
stale-index commit that deletes a newly-landed file while adding a ≥50%-similar
one in the same commit goes quiet. The exact-blob rule it replaced had the same
hazard at 100%. `--self-test` grew three arms for it — a rename-that-edits is
not flagged, a deletion plus an *unrelated* add still fires, and a re-land
through a rename is still caught — and each is mutation-pinned separately in
`internal/posse/silentrevert_qa_test.go`, because deleting the `R` handling makes
a rename *invisible*, which an arm asserting only silence cannot tell from
*excused*.

### `docs/notes.d/`: NOTES.md has one writer per file, by construction (ADR 0022)

Personas in the shared checkout do not edit or commit NOTES.md — it is the
one file every session touches, so a path-limited commit sweeps whoever else
had a hunk in flight and lands it under the wrong bead id (ranger-base-yuwy,
both directions in one afternoon). Two routes keep `git log --grep <id>`
true instead:

- **Fragment.** Write `docs/notes.d/<bead-id>.md` — a file the bead creates,
  sole writer by construction — and commit it path-limited under the bead
  id. No waiting on other writers, exact provenance, and the content is live
  documentation right where it sits. `docs/notes.d/` exists from the first
  fragment; there is no scaffolding to run first.
- **Worktree.** Edit NOTES.md itself from a dispatched session's own tree.
  Same-file divergence then surfaces at land time as a rebase conflict the
  launcher reports, never a silent sweep, and each commit keeps its own
  bead id through the replay.

**Folding a fragment into NOTES.md is ordinary work** — a docs bead, worked
in a worktree, that reads the fragment, merges its content into the right
section here, and deletes the fragment. Nothing about a fragment's existence
promises it stays a separate file.

## The live-box checks, and the one command that runs them (ranger-base-51z8j)

Most `verify-*` targets in the Makefile assert **this checkout**. Seven assert
**this machine**, and until 2026-09-06 nothing ran any of them.

Measured across the tree that day, before this section's own two targets
existed: of the 21 `verify-*` targets there were then, four are
prerequisites of `make test` (`verify-test-times`, `verify-parallel`,
`verify-suite-lock`, `verify-silent-reverts`) and so run in CI; two more have
their *script* run by a target a person types (`verify-gate-freshness.sh
--warn` at the end of `make install`, `verify-detection.sh --check-install` at
the end of `make install-detection`); the remaining fifteen executed only when
a person typed them. No aggregate target listed one as a prerequisite, no
workflow named one, no LaunchAgent ran one. The QA tests that *do* execute
these scripts — `bdpin_qa_test.go`, `grokpin_qa_test.go`,
`hookfreshness_qa_test.go`, `credentialpaths_qa_test.go` — run them against a
scratch `HOME` and a stub `PATH`, which is right for pinning the logic and is
exactly why none of them noticed: not one asks the box anything.

That is how a version pin can lapse and stay quiet for a day (ranger-base-k4lza).
A one-shot remediation of a condition that **regenerates** is not a control;
only a recurring detective check is.

```
make verify-box              # the seven live-box checks, ~40s, read-only
make verify-box-self-test    # ten arms proving the aggregate can still fail
```

**Why it is not a `for` loop over `&&`.** Each of these scripts already
separates three verdicts — `0` ok, `1` finding, `2` nothing measured — and the
third is the one an aggregate gets wrong. A machine with no codex answers `2`
correctly, and scoring that green is the same silence one level up. So
`verify-box` classifies each check, never stops at the first red, exits `1` on
any finding or unexpected status, and exits `2` saying **NOTHING MEASURED**
when every check answered `2`.

**CI cannot be its home.** A GitHub runner has none of these things installed
and would answer `2` to all of them — correctly, and uselessly. The home has to
be on-box, which is a launchd install, and *where a finding surfaces* is one
decision for all seven rather than seven decisions. Both are the operator's and
both are asked on ranger-base-0x1wc. Until one is answered this is still a
command a person types; what changed is that it is **one** command.

**The roster cannot fall behind in silence.** `scripts/verify-box.sh` carries
the roster *and* a table of every other `verify-*` target with the reason it is
not on a clock, and `boxcheck_qa_test.go` fails on a target in neither. Two
exclusions are load-bearing rather than housekeeping:

- `verify-runtime-walk` **spends a real turn** on the runtime under test. It is
  event-triggered by design — before switching a lane back onto a runtime, and
  after a version bump — and a schedule would be spending on a clock.
- `verify-pid-deny-set` as a *target* reads `HOME_DIR=examples`, i.e. this
  repo's own seed PIDs: a tree check wearing a live-box name. Its live readers
  (`--live`, `--settings`) are off the target on purpose, and `--live` answers
  `1` whenever a session is mid-bead behind a PID edit — correct, and a
  nuisance generator on a clock.

**And the census is over `scripts/` too** (ranger-base-bbl6r, which is this
section's own defect in the one shape its guard could not see). Both lists
above are keyed on a **Makefile target name**, so a check shipped as
`scripts/verify-*.sh` with *no target at all* was in neither list, named by
nothing, with a green board over it. Two were in the tree the day this landed —
`verify-ghost-composer.sh` and `verify-orphan-report.sh` — and the close that
wrote the census counted 21 `verify-*` *targets* and never enumerated
`scripts/`, so neither had ever been classified. Neither is schedulable today
(one needs an uncaged seat, one needs a container), so nothing was
unrun-and-needed; the gap was the guard's. A third table, `UNTARGETED`, now
carries every `verify-*` script that is no target's recipe with the reason it
cannot have one, and the QA test globs the directory against it both ways.

Three tables of *sentences* are checked rather than read, for the same reason:

- an exclusion that says `make test` runs it is checked against the `test:`
  prerequisite line. Drop the prerequisite and the check runs **nowhere** while
  the table still says CI has it.
- a roster **command** is checked against that target's own recipe. A recipe
  that gains or loses a flag would otherwise drift in silence; only a *renamed*
  script is loud today, because the runner's `[ ! -x ]` arm makes that an ERROR.
- every row's reason or command must be non-empty, so a row cannot satisfy the
  census by existing.

**It remediates nothing, and that is pinned.** Every rostered script is
read-only by its own contract; the aggregate kills no process, deletes no file
and fixes nothing. A finding prints the line for a person to type. This is
`verify-gate-freshness`'s rule applied one level up — a persona-writable tree
that could repair the box would be that tree, one flag away.

## Testing

**The suite command is `make test`, not a bare `go test ./...`**
(ranger-base-2ggb, with gilfoyle's ranger-base-2ad3 and 7xla on the same
invariant). The target adds `-timeout 25m` and the flag is load-bearing: go's
default is 10m PER PACKAGE, and `internal/rhq` has been measured on darwin
between 484.6s and 623.2s standalone — the worst reading already past the
default — and at 600.8s / 601.0s / 601.1s under `./...`, which is not an
assertion but the ceiling arriving as a timeout panic, because `./...` runs
the three packages concurrently and starves the long one. That red belongs to
the box, lands on whoever ran the suite to verify an unrelated diff, and names
NO TEST through the house filter (`| grep -E '^(---|ok|FAIL)'` prints a bare
`FAIL … 601.010s`). There is no long pole to cut instead — 1442 tests, none
over 10.3s, the top ten only 14% of the run — and the one structural lever
(`t.Parallel`; the package is a single serial stream at two-thirds of one
core) is blocked by 55 test files calling `t.Setenv`/`t.Chdir`. (Since
lifted in two halves: ranger-base-i7fa put `t.Parallel` on 758 tests; ADR 0047
decides the `newTestBackend` half — one HOME per binary, one worktrees root
per test via an App field — and stamps the package split NO: the directed
graph has a 67-file cycle and the serial time lives inside it.) 25m rather
than 20m because `make test-linux` runs this same target and its
`PLATFORM=linux/amd64` arm — emulated — is over 600s every time, while native
linux/arm64 is 112.3s and nowhere near. `suitetimeout_qa_test.go` keeps the
flag on, keeps its value above the measurement, and sweeps for new entry
points that route around the target. Numbers and the distribution:
`docs/notes.d/ranger-base-2ggb.md`.

**The clock is not the only thing this box runs out of** (ranger-base-krra).
On 2026-08-29 `make test` came back exit 2 with ~80 reds in `internal/rhq`,
every one of them `TempDir: mkdir …: no space left on device`, with 231Mi
free, a 41G go build cache and 670 leaked `Test*` dirs going back two days.
`t.TempDir()` calls `t.Fatal` on ENOSPC, so ONE full filesystem is reported
ONCE PER TEST that wanted a temp dir: through the house filter it is a list of
unrelated test names — worktree, watch, dispatch, merge — and reads exactly
like a broken change. `scripts/test-times.sh` now says it in words at both
moments a reader is present: a `DISK: <n> MB free on <fs>` line BEFORE the
packages run (a warning, never a failure, below a measured floor), and a block
AFTER any run whose log carries ENOSPC, naming the cause and the three places
the space went. A second block covers the same hour's other face — `package
iter is not in std` with a working toolchain, which is what a build cache
emptied under a running build looks like. It deletes nothing: `go clean
-cache` slows every concurrent session and deleting from `$TMPDIR` can take a
live test's TempDir away, so what to clear on a shared box stays the
operator's call. `suitedisk_qa_test.go` pins that `make test` still runs
THROUGH the wrapper — without that the explainers are dead code — and runs the
script's own `--self-test` inside the suite. Numbers:
`docs/notes.d/ranger-base-krra.md`.

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

**A pin that names a clock-derived token cannot hold it with a margin**
(ranger-base-nmab1). `cmd/posse`'s `TestCostPlanAndTheCostFooterAreOneRendering`
seeded a plan reading 3m30s old — "3m30s sits in the middle of the '3m' bucket,
so the two runs below cannot straddle its edge" — and then compared two
`runPosse` launches of the BUILT binary to each other. That is a 30-second
margin against an apparatus nothing bounds the runtime of: on 2026-09-05 the
pair took 48.18s on a box running two other seats' suites, `--plan` rendered
`read 3m ago`, the footer rendered `read 4m ago`, and `make test` was red for
every seat with nothing wrong in the rendering it was pinning. A wider constant
is not the fix — the wall grew 2.4x in four days (ranger-base-pj87l) and would
eat it again. Two shapes work. Freeze the clock IN PROCESS, which is what
`PlanCache.Now` is for and where the age's exact bytes are in fact already
pinned (`TestPlanCacheLineSaysHowOldTheReadingIs`). Or, when the subject really
is the built binary, BRACKET the run: seed immediately before the launch and
accept every value the formatter could honestly have produced between that seed
and the process exiting — `runAgedPlan` in `cmd/posse/costplan_test.go`, which
has one accepted answer on an idle box and two on a loaded one, with every other
byte of both renderings still exact. Four single-launch fixtures of the same
shape are still in the tree, filed as ranger-base-boafa.

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

**`--platform` is passed on every run, not only when `PLATFORM=` is set**
(ranger-base-1qm5). Docker's classic image store keys `golang:<minor>` as one
local image, so the documented amd64 override *replaced* what that tag pointed
at: one `PLATFORM=linux/amd64` run, and every later default run qemu-emulated
amd64 — announced by nothing but a platform WARNING — while NOTES.md, this
paragraph, said it tested the host arch. A one-shot override was a persistent
retarget. The default is now `linux/arm64` or `linux/amd64` read from
`uname -m`, so the request is explicit in both directions and the run after an
override resolves back to the host instead of inheriting it. Docker's
containerd image store (`Storage Driver: overlayfs`,
`io.containerd.snapshotter.v1`) keeps both platforms under the tag and never
poisoned — measured 2026-08-27 on Docker 29.0.1, where the repro no longer
reproduces — which is exactly why naming the platform beats relying on the
store: the fix is in the `docker run` argv, and `release_qa_test.go` pins it
there against a fake docker, on any store and any host.

The release workflow is still the last gate, not the first one: run this before
you push, because on a tag is the worst place to learn that Linux disagrees.

### The two runners are an ENVIRONMENT split, and it reaches the pins (ranger-base-tiidc)

`ci.yml` runs `make test` on ubuntu-latest and macos-latest, and ten
consecutive pushes to main were red there for three reasons that had nothing
to do with the commits carrying them. All three are one shape: a pin that
stood in for something the ENVIRONMENT supplies — a make, a git, a shell — and
was right about the one this box has. Worth knowing before writing the next
one.

**GNU make 4.x prints `make[1]: Entering directory` on STDOUT, 3.81 does not.**
`TestQATheGofmtDoorReportsRealDrift` and `TestQATheTreeWideDoorsReportRealDrift`
capture `make -n <door>` and hand the result to `sh -c`, which is what keeps
them from carrying a second copy of the recipe. `make test` exports MAKELEVEL,
every `make` the suite spawns therefore believes it is a sub-make, and 4.x
turns on `--print-directory` there — so the captured "recipe" came back wrapped
in two lines of make's own chatter and the shell ran them: `sh: 1: make[1]::
not found`, `sh: 8:`, and a door that had found no drift at all failed its
clean arm. ubuntu-latest carries 4.x; macOS ships 3.81, which is why this was
red on exactly one runner. Reproduced on this box against a GNU make 4.4.1
bottle with `MAKELEVEL=1` — byte for byte, including the line numbers. The fix
is `--no-print-directory` on the command line (`makeExpandFlag`), which beats
an inherited `MAKEFLAGS=w` as well and which 3.81 accepts; `assertRecipeIsOnlyRecipe`
makes its absence a named failure instead of a garbled shell.

**The runners moved to git 2.55.0, and `git commit` grew three options.**
`-U`/`--unified` and `--inter-hunk-context` (the context width of the `-v`
diff) do not exist on this box's Apple Git-155 / 2.50.1. They take a value as a
separate word, so `qualifierSpoilers["git commit"].ValueOpts` has to carry
them or the L1 wall reads the value as an option and refuses a safe
path-limited commit — which is what `TestQAValueOptsAreGitsRequiredValueOptions`
said, on both runners, in the words of the fix.

That makes the table the UNION of two gits, and two pins then name a spelling
the local git cannot be asked about. `qaCommitOptsSince` is that fact on the
pin side: option → the version that has it, exempting it from the "this git
never showed us that spelling" arms and from nothing else, and only after
making git call the option unknown (`git commit -h` hides some it still
parses — `--ahead-behind` on 2.50.1).

**Do not ask `qaGitResolves` whether this git has an option.** It reports that
a prefix resolved, never WHAT it resolved to. On 2.50.1 `--u` and `--un`
resolve cleanly — to `--untracked-files`, which is a different option in
neither list — so a `len(got) == 0` test reads them as abbreviations of
`--unified` and demands wall arms for them, which is the hole the pin exists to
prevent, rendered by the pin itself. Ask `qaCommitOptions` (git's own `-h`
list) instead.

**A path an operator PASTES is a shell's input, not argv.**
`TestTheOffBranchPrescriptionIsRunnableAndRescuesTheWork` takes the `git -C
<tree> branch -f <branch> HEAD` out of `posse worktrees`' listing and runs it,
which is the arm that catches a prescription naming the wrong directory. It
ran it with `exec.Command`, splitting the words itself — a shell in every
respect but the one that mattered. The listing `AbbrevHome`s the path, so on
any box whose worktree root is under `$HOME` the line reads `git -C
~/worktrees/…` and only a shell turns that `~` back into a directory: ubuntu
red with `fatal: cannot change to '~/worktrees/…'`, macOS green, same commit.
macOS was the accident — the test binary's temp `$HOME` is `/var/folders/…`
and the tree path resolves to `/private/var/folders/…`, so `AbbrevHome` finds
no prefix and prints the absolute path — and ubuntu was reproducing the real
operator's shape. Reproduced here by resolving the temp `$HOME` in TestMain,
which reds it on macOS too. It runs through `sh -c` now.

**Measure their version, do not infer it.** `brew fetch git make` DOWNLOADS
the bottles and does not install, link, or touch PATH, so `/usr/bin/git` stays
what it was; untar into a scratch dir and put a shim in front. Two things make
an extracted bottle actually usable and both fail silently otherwise: `DYLD_*`
must be exported INSIDE the shim script (macOS strips it entering any
SIP-protected binary, `/bin/sh` included), and a bottle outside its prefix
finds none of its own data files, so git needs `GIT_TEMPLATE_DIR` or every
`git init` makes a repo with no `hooks/`. Both root causes here were then
reproduced on this box, and each fix has a mutant that reds without it.

And the `toolchain identity` step in ci.yml prints `git --version` and
`make --version` on both runners now, for the same reason it prints awk's:
the next userland split should be a log line rather than an expedition.

**Making one half of git fail: `git status` and `git diff` break on different
things** (ranger-base-2asm5, owed by ranger-base-xw51s). Both cagestale.go
readers swallow a failed git call into a wrong verdict, so both need a fixture
that FAILS a specific call — and a fixture that breaks everything cannot say
which read a pin caught. Measured on git 2.50.1 / darwin 25.4.0:

- **An invalid `status.*` config value** — `git config status.showUntrackedFiles
  <not-a-mode>` — kills every `git status` at config-parse time (rc 128, and
  an explicit `--untracked-files=all` on the command line does NOT rescue it),
  while `git diff HEAD` still renders a full patch. This is the status-only
  fixture, and the one that pins a status branch on its own.
- **A garbage `.git/index`** kills status AND `git diff HEAD` (both `fatal:
  index file smaller than expected`) while `git rev-parse HEAD` still answers,
  since it reads refs and not the index. Use it for "nothing but HEAD can be
  read", never to pin one call.
- **An unreadable tracked file** (`chmod 000`) fails the diff and NOT the
  status, which only stats — extdiff_qa_test.go's arm 2.
- **An unreadable untracked DIRECTORY** (`chmod 000`) is not a fixture at all:
  status WARNS and exits 0.

The fixture helper witnesses both directions — the call it means to break, and
the call that must still work — because a plant that broke both would leave
the arm measuring the other reader.

**A "shipped surfaces" population must be DERIVED from the embed, and it has
been missed twice by a hand-written list** (ranger-base-kox69, verifying
ranger-base-3ersc). `embed.go:17` is `//go:embed all:examples`, so everything
under `examples/` is inside every release binary and `posse init` lays it down
in a fresh instance. That makes a seed PID *more* shipped than AGENTS.md, not
less — AGENTS.md is a file in this repository, the seed is what a deployer
gets — and it is exactly the surface a sweep forgets, because the PIDs a
persona reads all day live in `RHQ_HOME/agents/`:

- ranger-base-09b7: the L1 commit wall stood on the crew's own files and on no
  PID anyone created from what the binary ships.
- ranger-base-l1ix2 hardened a broken command everywhere it was PRESCRIBED,
  listed the hand-sweep as "README.md, INSTALL.md and the ADRs … docs/notes.d/
  is out too", and closed with "that is the whole rest of the tree".
  `examples/agents/reviewer.md:69` still told a reviewer to run the broken
  form, on exactly the change they were sent to read.

So build the population with `exampleAgentNames(posse.Seed)` and
`fs.ReadFile`, not a literal list: a PID added tomorrow is then graded with
nobody remembering the test file. Read the EMBED rather than `../../examples`
— in a checkout they are the same bytes, and the embed is the artifact both
gaps were in. Put a corpus floor on it, because `fs.ReadDir` over a subtree
that moved returns nothing AND no error, so a census over zero surfaces is
green. `extdiff_qa_test.go`'s `extDiffSeedSurfaces` and
`commitwallseed_qa_test.go` are the two worked examples.

Editing a seed PID is two edits, never one: `exampledigests.go`'s contract is
APPEND the new sha256 for that path and never replace one, or a home holding
the old bytes stops being a home posse recognises its own file in.
`TestEveryEmbeddedExamplePIDIsInTheShippedTable` reds and prints the line.

**A count FLOOR is not a liveness check — measure how slack yours is.**
`extdiff_qa_test.go`'s ARM 7 ran seven prescriptions against a floor of three,
so four of them could stop being found without a word. Ask the question
per NAMED surface instead (`ran[name] == 0`), so a prescription that
disappears reds by file rather than shrinking a total nobody reads. That is
ranger-base-3ersc FINDING 2 one level up: there, three exempt spans satisfied
`seen[name] == 0` on their own while every real NOTES.md prescription could
vanish. Before writing any floor, count the real population once with a
throwaway `t.Logf` census; a floor set below what is there measures nothing.

**A tree-walking pin must skip what git skips.** `TestSeedSurfaceNameCountIsZero`
walked the repo root skipping only `.git` and `.beads`, so `make build` — which
writes the gitignored `bin/posse-go` — put a 13MB Mach-O on the "seed surface"
and the pin found the banned token in its string table. `make build && make
test` was red for everyone, the printed "line" was a binary offset, and the
failure text sent that reader after a commit-time wall that was working. Two
arms of the same file had disagreed about it since both were written:
`TestPublicationRootCommitOmitsExcludedPaths` already excludes `bin/` from the
surface it checks. The scan takes its ignore set from one `git ls-files -z
--others --ignored --exclude-standard --directory` — one call, not one `git
check-ignore` per path — and git reports BOTH shapes, a wholly-ignored
directory collapsed to one entry and a single ignored file inside a directory
that is otherwise on the surface. They are skipped by different lines in the
walk, so a fixture carrying only the first leaves the second unpinned (measured:
that mutant survived).

**And it takes no ignore set at all unless the root is that checkout's
toplevel.** An export unpacked INSIDE some other repo — the `git archive`
scratch tree the house mutation rig runs in, when it lands under a checkout
rather than in /tmp — would otherwise be scanned against rules written about
that repo's paths, and the failure is a false SKIP, not a false hit: the parent
ignoring `notes/` silently takes the export's `notes/` off the surface. Empty is
the right answer for a tarball or a scratch tree because an export carries
tracked files only, so nothing under it is ignored and the walk loses no
coverage. (ranger-base-n0v6o)

**Amended 2026-09-06 (ranger-base-chd6w): that last sentence was true of a
PRISTINE export and false the moment anything writes into one.** Empty is the
right ignore set there; what it is not is protection. Measured under
ranger-base-5htxx and again here: `git archive main | tar -x` gives 951 files
and zero hits — and one `go build -o bin/posse-go ./cmd/posse` inside that tree
puts the same 13MB Mach-O back on the surface — a hit reported as
`bin/posse-go:8189:`, a string-table offset with the banned name run together
between `reopenedrejected` and `verify: username`, which is ranger-base-n0v6o
byte for byte in the tree whose safety the paragraph above asserts. (The name
itself is not quoted here for the same reason the pin exists.) `make build` is
exactly what writes into it and `git archive | tar -x` is the house mutation
rig. So the scan now skips **two** things, as a union: what git ignores, and
what is not text (a NUL byte anywhere in the body — git's own test, and 0 of
951 tracked files carry one). The ignore set still earns its place: it collapses
whole directories and it keeps text-shaped build output off the surface. Both
arms are pinned and each reds alone —
`TestSeedSurfaceScanSkipsBuildOutputWhereThereIsNoIgnoreSet` for the second.
ranger-base-n0v6o offered both shapes ("skip paths git itself ignores … or skip
non-text files") and only the first shipped; this is the other half.

## The fleet writes no auto-memory (ranger-base-7uhip)

claude 2.1.263 keeps a per-project "auto-memory" — a directory of one-fact
markdown files plus a `MEMORY.md` index it loads into every session's context.
**It resolves that directory from the git WORKING-COPY root, not from the
session's cwd.** Every posse seat launches into
`~/.posse/worktrees/posse/<session>`, so every seat resolved to the *operator's* own
`~/.claude/projects/<sanitized MAIN checkout path>/memory/` and appended there.

MEASURED by monica 2026-09-06: 114 memory files under that one directory
carrying ~100 distinct `originSessionId` values, `MEMORY.md` at 199 lines
against a harness cap of **200 lines / 25,000 chars** past which the tail is cut
(the model is told so — the cut is not silent, but the memories are gone from
context either way). Not one of the 1470 per-worktree project dirs
(`~/.claude/projects/<sanitized WORKTREE path>-*`) holds a
`memory/` subdir at all, which is the whole proof: the transcript dir keys on
cwd, the memory dir does not.

So the fix is one key in the launch payload — `"autoMemoryEnabled": false` in
`ClaudeFleetSettings` (`internal/posse/agents.go`), which `{settings}` renders
as `--settings` on every claude launch line.

**Why off and not redirected.** claude also takes `autoMemoryDirectory`, so
pointing a seat at its own persona dir was available and was not taken
(ranger-base-bmr1c's ruling). `ORDERS.md` is the persona memory the constitution
names and the launcher already commits it (`memoryland.go`); the auto-memory was
a second channel beside it that no persona owned. A redirect keeps that second
channel and only moves it, and it makes a payload that is a `const` on purpose
into a per-persona render. The COST is real and is named here rather than
discovered later: a seat also stops READING that index, and what is in it today
is largely posse engineering lore written by earlier seats. `ORDERS.md`,
`AGENTS.md` and `docs/` are the channels that survive, and they are the ones
with an owner and a commit.

**Do not reach for `CLAUDE_CODE_DISABLE_AUTO_MEMORY`.** It exists, and a
launcher-exported variable loses to any settings scope naming the same key
(ranger-base-rq83c) — which is the reason the credential and inlet pins already
travel inside this payload. `--settings` is flagSettings. For the sibling key
`autoMemoryDirectory` the resolver's own scope list ranks flag above
local/project/user and below policy alone, and project scope is ignored for that
key outright ("Ignored if set in projectSettings … for security"). For
`autoMemoryEnabled` the arms below measured flag scope against the DEFAULT — the
operator's `~/.claude` names the key nowhere today. **Flag versus a user-scope
`true` — what the `/config` toggle writes — is UNMEASURED**, and it is not
measurable from a seat: the root-owned policy file on this box pins
`CLAUDE_CONFIG_DIR`, so there is no scratch user scope to plant in
(ranger-base-i7cy4). If auto-memory ever comes back on for the fleet, measure
that contest first.

**The measurement, reusable.** There is no zero-turn readout for this — `doctor`,
`mcp list` and `project` print nothing about the memory dir, and `mcp list` does
not even create it. It takes a headless turn, which a seat can have by handing
the child the session's own mint from the instance's env set — that recipe is
instance-ops content and stays in the instance tree (ranger-base-bm9cd) — then
one prompt, *"Reply with ONLY the absolute
path of your persistent file-based memory directory, or the single word NONE if
you have no memory directory"*, on `--model claude-haiku-4-5-20251001`, arms
differing in exactly one settings key. Four arms were run:

| arm | payload | cwd | answer |
|---|---|---|---|
| A | `autoMemoryDirectory` = a scratch dir | scratch repo | that scratch dir |
| B | A + `autoMemoryEnabled:false` | scratch repo | `NONE` |
| C | the real rendered payload, key removed | this worktree | the operator's `…/projects/<main checkout>/memory/` |
| D | the real rendered payload | this worktree | `NONE` |

A is the failing wrong arm for B (the rig can produce a directory) and C is the
defect reproduced by execution, in the real shape, with the payload the launcher
actually renders rather than a hand-written replica. Nothing was written to the
operator's memory dir by any arm: the prompt asks the session to *name* the
directory, not to save anything in it.
