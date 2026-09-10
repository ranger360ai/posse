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

