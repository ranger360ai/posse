# Runbook — a session's Bash tool is wedged

*ranger-base-urnj, cut from the 2026-08-27 fleet-freeze RCA (ranger-base-ernt,
private tree: `$CONSTITUTION/docs/rca/2026-08-27-fleet-freeze.md`).
Fix: ranger-base-f0ay (ADR 0009 §1). Standing detection: ranger-base-urnj
(`internal/posse/gateaudit.go`, run every dispatch pass). Load guard:
ranger-base-innx (`internal/posse/loadguard.go`).*

This page exists because the diagnosis that ended the 08-27 outage took
**four and a half hours**, almost all of it eliminating hypotheses that felt
urgent — process caps, ptys, `bd sync`, `.zshrc` — before anyone read the two
numbers that actually answer it. Both are cheap. Read this page instead of
re-deriving the elimination tree from a per-operator memory file, which is
where the answer lived until now and which is exactly what does not survive
past the session that wrote it.

**The signature**: every Bash tool call hangs with **zero bytes of output**
— even `echo`, even backgrounded, even with `dangerouslyDisableSandbox:
true`. `TaskStop` frees nothing. Read, Edit, Write, `SendMessage`, and a
`Monitor` loop already running all keep working, because none of them spawn
a subprocess. That asymmetry — spawning tools dead, in-process tools alive
— is the tell that you are reading this page and not chasing something
else.

If two consecutive trivial Bash calls (a bare `echo`) both time out with zero
bytes, stop retrying and go straight to step 1. Retrying costs a full timeout
each time and clears nothing.

## Step 1 — the REAL audit (needs no shell)

The gate wrappers under `state/gates/<persona>/shell/<base>` are the only
posse-owned mutable state on the spawn path (ADR 0009). Their invariant is
one grep with zero hits:

```sh
grep -l '^REAL=.*state/gates' $RHQ_HOME/state/gates/*/shell/*
```

A hit names a wrapper whose `REAL=` line points at *another gate wrapper*
instead of a real shell. Two wrappers pointing at each other is a cycle:
every spawn that enters it exec-loops, re-prepending that hop's PATH guard
onto its own `-c` string on every hop, until the string hits `E2BIG` —
`Argument list too long` — around 40 minutes in. That is this bug's exact
signature, and it is also this bug's exact repair, in the same read.

**Run this with the Read tool, not Bash** — that is the entire point of step
1 existing. A wedged session has no shell, but Read and Edit never needed
one:

1. Read `$RHQ_HOME/state/gates/<persona>/shell/<base>` for every persona you
   can name (`$RHQ_PERSONA_DIR/ORDERS.md`, if you have it, or ask a peer
   session over `SendMessage`, which also does not need a shell).
2. Look at each file's `REAL=` line. Healthy: `REAL='/bin/zsh'` (or
   `/bin/bash`, `/bin/sh`) — a real interpreter, outside every `gates` dir.
   Broken: `REAL='.../state/gates/<other-persona>/shell/...'`.
3. If you find a chain or a cycle, **Edit the bad line to a real shell**
   (`REAL='/bin/zsh'`) on every wrapper in it. This is the actual fix, not
   a mitigation — the 08-27 repair was exactly this edit, on two files, and
   recovery was instant, in-flight loops included: each hop re-execs the
   file from disk, so an edit clears a *running* loop, not just future ones.

Since ranger-base-f0ay, `writeGateShell` refuses to render this defect in
the first place, and every dispatch pass now runs the same check itself
(`RealAuditWitness` in `gateaudit.go`) and logs any hit loudly rather than
waiting for someone to think to look. So a hit here today most likely means
either a wrapper older than that fix, or a hand edit — but the invariant is
what matters, not the provenance, and the repair is the same either way.

If the audit is clean, this is not the 08-27 bug. Go to step 2.

## Step 2 — load, from OUTSIDE the fleet

```sh
sysctl -n vm.loadavg
```

**This has to come from the operator's own terminal, or Activity Monitor —
not from inside any dispatched session.** A session asking a peer session to
run this for it fails twice over: the peer's shell is exactly as wedged as
yours if the cause is machine-wide, and if it is not wedged yet, recruiting
it into diagnostics is how the second occurrence of this bug looked from the
outside on 08-27. If you are a dispatched session reading this with no
working shell, your move is to say so — report blocked, in the terminal,
immediately — and let the operator run this command himself.

Baseline for this fleet, measured 08-27 post-repair: **load 2.5–7 at
19–23 live sessions.** Anything in the 20–30s and up is the incident band; a
reading of 95–260 is what actually happened. `fork()` cannot get scheduled at
that load, so a shell never starts — no bytes, no error, no exit, just a
timeout, which is why in-process tools are unaffected and spawning ones are
dead in exactly the way step 1's signature describes.

If load is high and the REAL audit from step 1 was clean, this is
*exogenous* load (an OS update storm, a neighbour process, a reboot's own
relaunch spike all did this at least once on 08-27) rather than the gate
wrapper cycle. There is nothing to edit — it drains. The dispatcher's own
load guard (`load_guard:`, default 25, ranger-base-innx) already declines to
launch new sessions into a box in this state; it does not touch sessions
already running.

## What to do while you wait

The wedge **is not permanent** — it has been observed to self-heal, measured
at roughly 40 minutes for the exec-cycle cause and faster for a load spike
that is already draining (a 15-minute average higher than the 1-minute one
means the peak has passed). Two dead `echo`s means stop retrying *for now*,
not stop working and not abandon the bead:

- **Report blocked, in the terminal, immediately.** This is a machine-level
  condition, not a defect in your session, and it is not yours to fix from
  inside it.
- **Switch to Read/Write work.** The codebase can still be read by exact
  path (there is no Grep/Glob tool in this harness, so `ORDERS.md` — or
  whatever names files for you — is what makes this possible with a dead
  shell), memory files can still be written, and a bead that only needs
  `bd comments`/`bd close` at the very end can still be worked right up to
  that point.
- **Re-probe with one cheap `echo` after a long interval — tens of minutes,
  not four.** A four-minute recheck looks like "still dead" even when the
  wedge is genuinely healing; that is what made the first sighting of this
  bug take longer to read than it needed to.
- **Do not delegate the diagnostics to a peer session.** See step 2 above —
  it recruits the peer into the outage instead of answering the question.
  `SendMessage` still works and is worth using to tell peers what you are
  seeing, which is different from asking one of them to run a shell command
  for you.
- **When the shell returns, it returns fully** — no partial recovery
  observed. Bead work that was blocked can be landed then; nothing here asks
  you to declare the bead dead.

## Why this page exists rather than a memory file

A per-operator memory file answers this exactly as well the day it is
written, and not at all to a session that has never read it — and a wedged
session, by construction, cannot go looking for one it does not already
know about. This page is checked into the repo so `$RHQ_PERSONA_DIR/
ORDERS.md`-style path knowledge (or a plain "read
docs/runbooks/shell-wedge.md") is enough to reach it with no shell, the same
property step 1 depends on. Keep the two in sync going forward: this page is
the durable copy, the memory file is where the incident narrative and the
raw measurements live.
