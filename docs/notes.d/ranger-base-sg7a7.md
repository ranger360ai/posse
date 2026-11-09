# The launcher makes the session's furniture, not the host's (ranger-base-sg7a7)

2026-11-09. ADR 0080 D23, the decision ranger-base-temhf's reproduction
turned up and deliberately did not take: `bwrap` prep was creating host
directories out of a PID's declarations, and what it may create is a
decision, not a cleanup.

## The mechanism, in one paragraph

`sandbox-exec` grants a PATH. `bwrap` binds a SOURCE. So every grant that
seatbelt could hand out for free — a path the session might create later,
a directory this box happens not to have — costs a write at the bwrap
mechanism: `BwrapArgs` emits a `PrepMkdirAll` for it and `runBwrapPrep`
makes it before the wall goes up, **outside the namespace and as the seat's
user** (ADR 0080 D17/D18, `internal/posse/bwrap.go`). That is correct and
load-bearing for the session's own furniture: a `state/` that a fresh home
has not grown yet, the gates dir, the audit logs, D20's ref keep file. It
had no perimeter, so it also covered two things nobody decided it should.

## What that cost, both MEASURED

**1. One PID line meant three different things on three hosts.**
`parity_test`'s fixture declares `writable: [scratch, /opt/shared]`. On
darwin that is free — SBPL grants a path whether or not it is there and
nothing writes it. At the bwrap mechanism:

| host | what the one line does |
|---|---|
| darwin | nothing; a `(subpath "/opt/shared")` line over a path that is not there |
| this laptop, mechanism pinned | the launch REFUSES: `cage seatbelt (bwrap) cannot prepare /opt/shared: mkdir /opt/shared: permission denied` |
| ci.yml's ubuntu runner | `/opt/shared` is CREATED on the runner, silently, every red run |

and nothing anywhere said so before the launch.

**2. A launch materialized `<cwd>/.beads`, which moved a LATER launch's
parity row.** The store of record is in the writable set, so prep made it.
ADR 0013 §4's reachability row probes only a store that EXISTS
(`recordTargets`), and its abstain reads *"no store of record at X —
nothing to reach (bd creates it on first write)"*. In
`TestSeatbeltProfileAndLaunch` the grok seatbelt launch therefore created
`<repo>/.beads`, and the codex launch three arms later — same `repo`, a PID
denying bare `Edit` and `Write`, so codex renders `-s read-only` — then had
a REAL store behind a read-only sandbox where it had none before. The row
went from "nothing to reach" to unrealized and REFUSED that launch. Same
tree, same PIDs, a different answer depending on which seat launched there
first.

Neither was reported by anything. On ci.yml's ubuntu arms the first was a
silent host write and the second reddened three lines of a test that is
about something else entirely, which is how it looked like the wall was
broken three ways (ranger-base-temhf's own reading).

## The decision

**Prep's authority is posse's own perimeter.** The session tree, the gates
dir, posse's state, the persona's memory dir, the launching runtime's
declared state dirs, the generic caches, `TMPDIR` — all still created.
Outside it, two classes are **bound where the host has them, render no
mount where it does not, and are never created**:

- **an absolute `writable:` extra** (`~` counts). A host path the operator
  declared, not one posse derived; nothing inside the session can bring it
  into being either. A RELATIVE extra keeps its prep — it resolves inside
  the session tree posse made and retires, and for a persona that denies
  file writes cwd itself is not granted, so without the mkdir that grant
  would be dead.
- **the store of record.** Its EXISTENCE is read as a fact by the row
  above, and an empty `.beads` is not inert: in no-db mode bd resolves
  `$BEADS_DIR` else `$cwd/.beads` and reads no redirect at all (ADR 0055),
  so a directory posse makes there is a second store of record that can
  take writes nothing reads back. `bd init` is denied to every persona
  because creating a store is a decision; it is not the launcher's either.

The alternatives, and why not:

- **Refuse the launch** on an absent absolute grant (today's behaviour
  where the parent is unwritable). A stale path in one PID then makes every
  seat on that PID unlaunchable on one host, over a grant that is nobody's
  gate. The wall is otherwise fine and narrower.
- **Degrade the launch** (`Parity.Degraded`). That is the refusal again
  with a waiver, and it refuses dispatch, which never allows degradation on
  its own. ADR 0017 §2 is explicit about the class: a mechanism that
  differs is a declared difference; a wall posse asked for and did not get
  is degradation. This is the first.
- **Create it and report it.** That is the status quo plus a line, and the
  line does not un-create the directory.

## Where it is said

A grant that silently disappears is the wall-with-a-note shape this tier
refuses, so each absence surfaces where it is read before the launch:

| surface | what it says |
|---|---|
| `bwrap.txt`, `posse gates` | a `·` line per absent grant, below the prep list — `Seal`'s marker, which already means "in a list, renders no mount". The two lists together are the whole of what the launcher did and did not do on the host. |
| parity | the PID half as a **DeclaredDifference**, never degradation (`bwrapAbsoluteWritableDifference`). Silent at the other mechanism, at `shims`, at `container`, and where the tier is unavailable. |
| ADR 0013 §4's no-store row | one clause naming the launcher, at this mechanism only (`recordReachNoStoreClause`). No second row: that row already owns "there is no store at this launch dir". |
| `posse agent check` | a warning on an absolute `writable:`, where the operator writing the PID is standing. Box-INDEPENDENT on purpose — the PID may launch on a host that has the path, and the launch row answers for the box it is on. |

## Four things the decision's text did not settle

- **The store has one spelling that can be absent**, `<cwd>/.beads`.
  `SeatbeltWritable` grants it under two names, but a redirect bd FOLLOWS is
  one whose target `beadsRedirectHop` already stat'd as a directory — so the
  resolved target is there by construction and an entry for it would be a
  control with no failing arm.
  `TestQABwrapTheStoreHasOneSpellingThatCanBeAbsent` is that measurement
  rather than a sentence asserting it.
- **A FILE at a no-create grant stays in the set**, so B8's refusal is still
  `BwrapArgs`' and still names the path and the remedy. Answering it in the
  drop would be a second refusal for one fact, and a quieter one.
- **The drop is a producer step, not a sixth arm in the renderer.**
  `bwrapNoCreate` + `bwrapBindExisting` run in `bwrapRender` after D20's
  substitution and before `BwrapArgs`, which is `bwrapGitSwap`'s own move:
  the renderer never learns this question exists, and its signature does not
  grow a parameter at thirty call sites. The re-stat inside `BwrapArgs` can
  in principle prep a path that vanished between two statements of one
  render — a path that was on the host when the render began, which is not a
  path posse invented.
- **Stated residual.** Where a redirect names a store in a directory that is
  not a git repository, `SeatbeltWritable` grants `<root>/.git` and prep
  still makes it. That path is not on the list because in the common case
  `<root>` IS the session tree, where `.git` is the session's own and
  creating it is the furniture this decision keeps. Closing it needs a
  producer that knows which of the two it is naming, the way D20 answered
  the ref pair one step earlier.

## A correction to ranger-base-temhf's record

That close, and `docs/notes.d/ranger-base-temhf.md`'s last section, both say
the `/opt/shared` fixture "names a path under its own tempdir now". That
edit never landed — `parity_test.go` still declares it, and the ubuntu arms
have been creating the directory ever since. Nothing is rewritten: the
declaration now STAYS on purpose, because ci.yml's ubuntu arms are the only
reader on the planet that runs that test at this mechanism, so the one line
that was making the directory is the live witness that nothing makes it any
more (`hadOptShared`, measured before the launch so a runner that genuinely
has `/opt/shared` cannot red it). The `.beads` half of the same incident
gets the same treatment one line down. The codex arm keeps the tree of its
own temhf gave it — its subject is the cage shortfall and it is better off
not reading a tree three other launches have been in — so these two lines
pin the cause where that one pinned the symptom.

## Verification

MEASURED 2026-11-09, darwin/arm64, `wallMechanismHost` pinned to `linux`
and `bwrapBin` stubbed. Before the fix a launch creates both members and the
reachability row moves; after it, neither. Five mutations, each red in the
pin whose name is the behaviour:

| mutation | pins red |
|---|---|
| `bwrapRender` stops asking the producer | 3 (prep, launch+row, readable copy) |
| the store dropped from `bwrapNoCreate` | 3 (launch+row, the list, the one-spelling pin) |
| `pidWritableIsHostPath` answers false | 6 (both parity pins, agent check, the list, prep, readable copy) |
| `recordReachNoStoreClause` silenced | 2 (launch+row, the clause table) |
| the drop stops asking whether the path is there | 3 (both controls and B8's refusal) |

That last one is the other direction the fix can be wrong — a drop that
takes a grant the host HAS — and it is why the prep pin carries a
present-directory arm beside the absent one.

What stays unverified is unchanged by this bead: a darwin-driven Linux arm
is not ci.yml's ubuntu arms. `SeatbeltWritable` still reads `runtime.GOOS`
for the home-cache grants and for `/private/tmp`, so which PATHS the wall
covers is `homeCacheGrants`' own pin and what the LAUNCHER MAKES is this
one.
