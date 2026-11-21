# ADR 0088 — The landed tree is read after every landing, outside the lock, and a red is filed, not blocked

*Status: accepted 2026-11-11 (ranger-base-oe9ja, from ranger-base-qregu) ·
owner: architect · builds on ADR 0087 (the replay gate and its ratchet) and
ADR 0011 §1 (the launcher lock) · a second check, the wall venue, runs after
each green judgement under the same cage, ratchet and watchdog
([ADR 0097](0097-the-walls-live-pins-measure-in-a-nesting-venue-after-each-landing.md))*

## Context

ranger-base-s64a5 shipped a waiver register in
`internal/treepins/coldexecarms_qa_test.go` whose partner pin
(`TestQAColdexecArmRegisterIsNotStale`) reds once a registered package gets a
`coldexec-check` arm. ranger-base-in8dr added exactly those arms. Each seat
ran the doors on its own base, which held only its own half, and both runs
were green. s64a5's bead named the remedy in advance: "whoever lands second
deletes the matching row". No process does that. The second lander's seat
never sees both halves, and the one actor that does see them, the launcher's
replay, compiles and does not run tests (ADR 0087 D6).

MEASURED 2026-11-11 (git log on main): in8dr was based at 79ef83df (02:46).
s64a5 landed at 03:46. in8dr was replayed over it and landed at 04:44 (author
time 04:38). The replay's land check was green because the conflict
type-checks. On main, `make test` exits 2 in 31.9s for every seat (MEASURED
by holden on ranger-base-qregu). The red comes from `corpus-check`, a
`tree-check` door, which is a prerequisite of `make test`. The verify lane
found it about five hours later. Nothing else reads main's exit code:
ci.yml fires `on: push: branches: [main]`, and only the operator pushes.

This is the third case of "a gate that cannot reach the pin that reds"
(ranger-base-rulbl gofmt; ranger-base-quzcd/n8wek silent-revert audit). The
first two were answered with prose. This one is answered with a reader.

## Decision

**D1 — After every landing, the launcher reads main's new tree with the
repo's own landed check.** A repo opts in by shipping an executable at
`scripts/posse-landed-check`. The contract is the same as ADR 0087 D1: no
arguments, cwd is the tree to judge, exit 0 means green, and the last lines
of output give the cause. The copy that runs comes from the **last tree
recorded green**, not from the tree being judged. A landing that weakens the
script does not change what judges it, which is the same rule as 0087 D1.
This runs on both landing arms, the fast-forward and the replay. It covers
residual (1) of 0087, the arm that is never judged.

**D2 — It runs outside the lock, after the fast-forward, and never blocks.**
The landing has already happened. The check does not hold the lock, does not
hold a seat, and cannot strand the fleet. What it buys is detection with
attribution, measured in minutes instead of hours. This does not prevent a
red main. See Alternatives for the blocking form, and why it waits on this
one.

**D3 — Caged, in a checkout of the landed commit.** The launcher creates a
detached worktree of the landed commit. A worktree is needed, not a
`git archive` export, because `history-check` and the silent-revert audit read
git history. The launcher runs only git itself there. The check then runs
inside the box's own cage tier (bwrap on Linux, seatbelt on darwin). The
worktree is writable, and so is the shared build cache. Nothing else is. ADR
0087 D6's rule, "the launcher executes nothing a branch wrote", is about the
actor and not the lock. Landing adds no human review, so the code is no more
trusted after it lands. If no cage renders, the result is "no answer": print
⚠ and do nothing more. It never falls back to running uncaged.
*(Amended 2026-11-12, below: the shared build cache is no longer granted,
and the cage is never stronger than a seat's.)*

**D4 — One run at a time, and runs coalesce.** At most one landed check is in
flight. When it finishes, if main's tree has moved, the next run judges the
newest tip only. The verdict is cached by tree id in one repo-level git
config record, `posse.landedCheck = <tree-id> <green|red>`. It is keyed on the
tree for the same reason as 0087 D4.

**D5 — Attribution follows the ratchet.** A bead is filed only when the
verdict goes from green to red.

| last recorded | this tree | action |
|---|---|---|
| green | green | record it |
| green | **red** | file one `-t bug` P1, `-l code`, titled "main red at <short tree>", with the check's last lines. It names every landing between the last green tree and this one (commit, bead id, persona). Assign it to the persona of the **newest** landing in that window, because that landing combined the halves, as with 0087 D5's merge-back. |
| red | red | one comment on the open bead naming the new tree; no new bead |
| red | green | comment "green at <tree>" on the open bead; closing it is the fixer's job |
| no answer | any | ⚠ and nothing recorded |

One open bead per red stretch replaces several seats each filing a P1 for the
same exit code, which happened with ranger-base-4v4r6.

**D6 — This repo's body: what reds every seat before an arm runs.** The
script runs the prerequisites of `make test`, which are `tree-check`,
`verify-silent-reverts` and the other `verify-*` doors on that line. They are
derived from the Makefile's `test:` line, not listed by hand, as the land
check derives its arms. That covers all three instances in Context. The
suite arms are not run: about 15 minutes, three suite slots per landing.
Ceiling: if the warm run exceeds **120s**, the price goes back to
architecture. (ASSUMED ~50–60s: `tree-check` alone MEASURED 46.4s on
ranger-base-qregu and 55–56s on a quiet box in AGENTS.md. The other
prerequisites are unmeasured.)

**D7 — A coupling known in advance is a dependency edge, not a remedy
addressed to "whoever lands second".** That phrase names no reader. When a
bead knowingly ships a pin that a named in-flight sibling's landing will red,
the coupled half (here, the stale-register direction) is cut into its own
bead with `bd dep add <that bead> <sibling>`. That bead is then dispatched
into a tree that holds both halves. The independent half ships now. D1–D5 is
the backstop for couplings nobody saw. D7 is the cheap rule for the ones
somebody did.

## Consequences

- Main can still go red. A red is now found when the check finishes,
  ASSUMED 1–2 minutes after the landing, plus one queued run when landings
  bunch up. It arrives as one attributed bead. Today it is found whenever
  someone next reads the exit code, which was five hours here.
- Cost: one caged ~1-minute run per landing (fewer, because runs coalesce),
  on the box, outside the lock. This adds no lock time.
- Residuals, stated: (1) a semantic conflict that only a suite arm reaches
  is still found by the next seat's `make test`; (2) between the landing and
  the verdict, a dispatch can branch from a red main; (3) a repo with no
  `scripts/posse-landed-check` gets nothing, as with 0087.
- Exit hatch: everything here is one script path and one config record.
  Deleting the script turns it off for that repo. No state is held hostage.

## Alternatives rejected

- **Block the landing on this check, inside the lock.** That would execute
  branch code inside the lock, and it costs about a minute of lock time per
  replay. The clever form, the one I wanted, is ADR 0087's exit hatch: replay
  and check outside the lock, then take the lock only for the `--ff-only`
  compare-and-swap. That makes the ratchet total and keeps main green. It is
  deferred, not refused. It restructures the sweep and the replay, and it
  needs exactly the caged runner and verdict cache that D3–D4 build. Revisit
  it when D5 has filed **two or more green→red beads in 30 days**. That is
  the measured cost a block would buy back, and nobody has counted it yet.
- **(b) A line in "Landing the plane": rebase and re-run `tree-check` before
  closing.** It is cheap, and it would have caught this case, because in8dr's
  base was two hours stale. But it is prose, and prose has failed twice in
  this class. It also leaves the window between the seat's last run and its
  landing. Seats may still do it. Nothing relies on it.
- **(c) Forbid deliberate cross-bead red pins outright.** Stale-waiver pins
  are worth keeping, because a register nobody prunes rots. The narrow form,
  D7, keeps the pin and moves only its timing.
- **(d) Let the verify lane be the reader.** It reads after closes, in
  batches. It is not a reader of main's exit code, and five hours is the
  measured price.
- **ci.yml as the reader.** It fires on the operator's push, which comes
  hours later, and its red is public. It stays the backstop for the suite
  arms and for the darwin leg.
- **Run the whole `make test` after landing.** About 15 minutes and three
  suite slots per landing, competing with the seats it exists to protect.
- **Run it uncaged as the launcher, since the code is now on main.** Landing
  is a fast-forward, not a review. The operator chooses to run main uncaged.
  The launcher would do it automatically on every landing.

## Amendment: the judge is never stronger than a seat (ranger-base-ze6ie, from ranger-base-hy1aj, 2026-11-12)

D3 said the worktree is writable, "and so is the shared build cache", and
nothing else. That sentence was about FILES. It was true of files and still
left the judge stronger than any seat. `boxLandedCage` renders through
`BwrapArgs` and `SeatbeltProfile`, not `RenderWall`, because the judge has no
persona (a synthetic one would be a seat posse announces and does not have).
So every host-scoped mask that `RenderWall` gives a seat missed it. MEASURED
2026-11-11 from inside the D3 cage on the Linux host (bwrap 0.13.1):

- the **user runtime dir** was in view: the session bus, the user service
  manager (a same-uid client can start a unit, and that unit runs uncaged),
  and the agent sockets;
- the **host `/tmp`** was in view;
- the **network namespace** was the host's;
- the **shared Go build cache** was writable. ranger-base-olr0g measured that
  an archive planted there runs inside the next uncaged build that reads it,
  posse's own included. That puts code from a landing inside the launcher.
- the **environment** was the launcher's whole `os.Environ()`, and the
  check's output tail is filed into a bead that other seats read.

The question that opened this asked only about the network. A netns closes
none of the other four: they are path sockets, a writable dir, and the
environment.

**D3.1 — The rule: the judge is never stronger than a seat.** It gets every
host-scoped mask a seat gets. Since no persona owns it, it also keeps none of
the egress a seat keeps. On bwrap: `Hide` = the live runtime dirs, `Tmp` = a
`tmp` dir inside the run dir (it stands down, as a seat's does, when the
home, run dir or tree is itself under `/tmp`), and `--unshare-net`, added at
the judge's call site only. bwrap brings the new namespace up with loopback,
so a check that listens on 127.0.0.1 still runs. On seatbelt, a network rule
appended last: `(deny network*)`, then loopback IP and unix sockets under the
run dir allowed back. Seats keep "egress is the container tier's".
(ranger-base-4nzpo.)

**D3.2 — The judge builds through a cache of its own, fresh per run.** This
reverses D3's shared-cache grant. `GOCACHE=<run>/gocache` lives inside the
run dir, which is already the grant and is already removed after the run.
`goBuildCache()` is out of the write set. (ranger-base-4nzpo.)

**D3.3 — The environment is an allowlist, not the launcher's.** `cmd.Env` is
built from nothing: the names in `landedJudgeEnvKeys` (`PATH HOME USER LOGNAME
LANG LC_ALL TZ TERM GOROOT GOPATH GOMODCACHE GOTOOLCHAIN GOFLAGS`), each only
if set, then the cage's `TMPDIR` and `GOCACHE`, then `protocolEnv`. It is
`cmd.Env` rather than bwrap's `--clearenv` because `cmd.Env` works on both
mechanisms. The repo still owns what green means over the environment it is
handed. What crosses the wall is the wall's call. A check that needs one more
variable reds once, and the fix is to name it in that one list. This
supersedes the residual in `docs/notes.d/ranger-base-5fond.md`.
(ranger-base-5m5yq.)

**D3.4 — Pinned at the real call site.** `TestQALandedCheckJudgeIsNoStrongerThanASeat`
and `TestQALandedCheckJudgeEnvIsTheAllowlist`
(`internal/posse/landedjudgecage_qa_test.go`, arm 1) drive
`landedChecker.judge`, the function `posse dispatch` runs after every landing.
If they are deleted, a regression can put landed code back inside the
launcher's next build, or back on the session bus, and no test reds. Each
was mutation-red, one change at a time (the notes for 4nzpo and 5m5yq list
the mutants).

**Measurements.** MEASURED, Linux host, go1.27.3, `scripts/posse-landed-check`
on posse's own tree:

| cage | wall | load (1m) | source |
|---|---|---|---|
| D3.1 + shared cache, built by hand | 73s | 4.0 | hy1aj, 2026-11-11 |
| D3.1 + D3.2, cold / warm, built by hand | 78s / 72s | 5.9 / 5.5 | hy1aj, 2026-11-11 |
| D3.1 + D3.2 + cleared env, built by hand | 70s | 4.0 | hy1aj, 2026-11-11 |
| as shipped, 4nzpo | 76s | 2.2 | 4nzpo, 2026-11-11 |
| as shipped, 4nzpo + 5m5yq | 106s | 4.8 → 12.5 | 5m5yq, 2026-11-12 (one reading) |

The check needs no network, no bus and no inherited variable. The private
cache costs about 6s cold. Every reading is under D6's 120s ceiling. The last
one is the closest, and it is one reading on a loaded box. If a series of
quiet-box readings crosses 120s, D6 sends the price back here.

**Consequences.**
- ASSUMED: a landing that raises `go` past the box's toolchain, or adds a
  module no seat on the box has downloaded, reds the judge with go's own
  "cannot download" line. That red is honest, because the box cannot build it
  offline. The exit hatch is that the operator fetches, and the next landing
  re-judges.
- Residual: a host mask added to `RenderWall` later does not reach the judge
  unless D3.4's pin is extended. Exit hatch: factor one `hostCarve()` that
  both call, the day a third host mask appears.
- The seatbelt rule is rendered and pinned. It is not measured here, because
  the box has no darwin host.

**Alternatives rejected.**
- *`--unshare-net` alone, as asked.* It closes egress, loopback and abstract
  sockets, and it leaves the runtime dir, `/tmp`, the cache and the
  environment open. It would look like the hole was closed while the largest
  channel stayed open.
- *Route the judge through `RenderWall` with a synthetic persona.* This is
  the clever one: every mask, present and future, for free. It is a fake
  seat, though, and it would hand the judge a seat's egress.
- *A persistent judge-only cache.* It saves about 6s, and it lets landing N
  plant objects that decide landing N+1's verdict.
- *Inherit the environment and scrub it by pattern* (`*TOKEN*`, `*KEY*`). That
  is a denylist, and it rots: the next credential's name is not on it.
- *Leave the environment alone, because the launcher has no tokens today.*
  That was MEASURED true. It is a property of whoever started the launcher,
  not of posse.

Not closed here:
- ranger-base-bfla6

## Amendment: the deadline bounds a hang, the ceiling is a reading, and a silent judge is filed (ranger-base-7idk4, 2026-11-12)

D6's ceiling was crossed (ranger-base-7idk4). The judge took 125.3s twice and
was killed at `landedCheckLimit` = `generatorTimeout` = 120s. MEASURED by holden
the same day, uncaged on the same tree, the check itself is GREEN at 110-150s.
A deadline is spelled as silence (D3), so every landing went unjudged, D5 never
filed, and nothing said so. The ⚠ line reached dispatch's output, which nobody
reads.

The defect is not the 120s. **One number was doing two jobs.** It was the
price at which the design comes back to architecture, and it was also the
kill. D6 derives the body on purpose, so the body grows with every door. A
fixed kill on a body that grows will be crossed, and when it was, the crossing
was silent. That made the ceiling unreachable by design: the only signal it
could produce was the absence of one.

MEASURED 2026-11-12 by richard: linux 7.2.15-1-omarchy, 16 cores, uncaged,
warm shared cache, `7725ba75`, one `make` per door, loadavg 6.4-7.2:

| door | wall |
|---|---|
| `verify-reap-test-tmp` | 24.8s (alone again: 46.2s, 30.1s) |
| `register-check` | 23.6s |
| `verify-test-times` | 6.4s |
| `corpus-check` | 5.6s |
| the other 21 | 0.1-4.1s each, 24.7s together |
| **total** | **85.1s** |

Two doors make up 57% of the run. `verify-reap-test-tmp` became the tenth
prerequisite on 2026-11-12 (`24bfa8d9`). Its Makefile comment prices it at "a
few seconds". Landings on main, MEASURED with `git log --first-parent` over 3
days: 167 landings, gap median 713s, p25 253s, p10 4s (bunches, which D4
coalesces).

**D6.1 — `landedCheckLimit` is its own constant: 10 minutes, and it bounds a
hang, not a price.** It is no longer `generatorTimeout`. 600s is 4× the
highest reading (150s). The judge holds no lock and no seat (D2), so a long
bound costs one thing: a hung judge delays the next coalesced judgement by at
most one median landing gap. A deadline still answers silence, never red. D3's
doctrine is unchanged.

**D6.2 — The ceiling is 240s, and it is read off answered runs, never enforced
by a kill.** Every verdict line prints its wall. The number is chosen for
attribution, not for box time. D5 names every landing in the window, and a
check that finishes under the p25 landing gap (253s) keeps that window to one
landing most of the time (ASSUMED: that is the property worth a price review;
the gaps are MEASURED). The price of 110-150s per landing is accepted: about
52 landings a day, coalesced, outside the lock.

**D6.3 — The judge's own health is filed, because silence was the failure
here.** This is a watchdog on the detector. It needs one more repo-level config
record, `posse.landedCheckQuiet = <tree> <n>`: the count of consecutive judged
runs that returned no answer. Any answered verdict resets it. D4's record is
untouched, so D5's left-hand column still names the last green tree.

| event | action |
|---|---|
| 2nd consecutive unanswered run | file one `-t bug` P1, `-l architecture`, "landed check silent at <repo>", carrying both `said` lines |
| further unanswered, bead open | nothing (the bead already says so) |
| answered run over 240s, no such bead open | file the same bead class, "landed check over its D6 ceiling at <repo>", with the wall |
| answered run, bead open | one comment "answered at <tree> in <wall>"; closing is architecture's |

Two runs, not one. A single silence is a stranded lock or a cage hiccup, which
is exactly what D3's ⚠ is for. Two in a row is a pattern, and holden's two of
two would have filed on the second landing. Dedupe follows D5: one open bead
per stretch, found by a marker in its description.

**D6.4 — A failure must name its own cause (code lane).**
`landedchecklive_test.go` reads a deadline as "D3'S WRITE SET BEING WRONG". The
verdict already distinguishes "no answer in <limit>". The test must say
deadline when it is one.

**Consequences.**
- The 110-150s check answers again. Detection is about 2.5 minutes after a
  landing, against five hours before ADR 0088 and never during the crossing.
- The next crossing is a bead, at 240s answered or at the second silence. It
  is no longer something a verify lane happens to find.
- Exit hatch: one constant and one config key. Deleting the key's reader turns
  D6.3 off, and it holds nothing hostage.
- Not changed: the derivation (D6), the fresh private cache (D3.2), the
  allowlist (D3.3).
- As built (ranger-base-lhiy1): the table's last row is reached only by an
  answered run that has something to say — a counter that was not zero, or a
  wall over the ceiling. Asking the store on every answered run would be one
  `bd list` per landing on a healthy main, on the pass that joins the judge's
  goroutine, which D5's own cost pin refuses. Every row still fires: a bead
  filed for silence is commented on by the run that ends its stretch, and a
  ceiling bead by the next crossing. What is given up is a comment on an
  answered run inside the price with nothing on the counter, where the open
  bead already carries that line.
- Residual (ranger-base-lhiy1): the two `said` lines a silent bead carries are
  held in memory, because D6.3 spells the record `<tree> <n>` and a sentence is
  neither. A crossing is two runs of one long-lived launcher by construction
  (ADR 0028 §1); a launcher that restarted between the two files a bead saying
  the earlier sentence is not recorded, rather than inventing one.
  **And a restart is not the only way to get there** (ranger-base-4jroi): an
  answered run whose reset of `posse.landedCheckQuiet` could not be written —
  a `.git/config.lock` lost to a concurrent persona — clears that sentence and
  leaves the count standing, so the next silence crosses the threshold over a
  count that spans an ANSWERED run. The bead now says which of the two it is,
  and over the second it claims no consecutive pattern: the mark is in memory
  beside the sentence, so a restart carries neither and reads as the restart
  it is.

**Alternatives rejected.**
- *(a) alone: raise the limit and stop.* That fixes today and keeps the defect.
  The next door added crosses the new number just as silently.
- *(b) Narrow the body: drop `register-check` and `verify-reap-test-tmp` from
  the judge, or give it a hand-listed goal.* It undoes D6's derivation, the one
  property that covers tomorrow's door. The two doors' cost is filed against
  the doors instead (ranger-base-7idk4's code beads).
- *(b′) `make -j` with `--output-sync`, then re-run the failed door serially
  so it lands in the tail.* This is the clever one. MEASURED, the longest door
  is 25-46s, so the judge would take about 50s. It breaks D1's "the last lines
  are the cause" unless the serial re-run is added, which makes it a second
  runner. It also takes several cores per landing beside seats' suites. Once
  the kill is a hang bound, it buys latency nobody has asked for. Revisit if
  D6.3 files an over-ceiling bead.
- *(c) A warm cache for the judge.* It reverses D3.2. MEASURED by holden, a
  fresh cache is not the driver (138s fresh against 150s warm).
- *(d) alone: keep 120s and only make the silence loud.* Then every landing
  files the silence and none is judged. Loud and blind together.
- *Record "unanswered" in D4's own key.* That would overwrite the last green
  tree, which D5 needs as the far edge of a red window.

## Amendment: the judge takes the seat's socket wall, and the ADR 0097 venue explicitly does not (ranger-base-i92tj, from ranger-base-1xyso, 2026-11-14)

*This amendment's bead and the finding behind it are `vuln-private`. They are
named by bare id in the Beads block below and nowhere else (AGENTS.md,
"Landing the plane"). What is described here is what this change SHUTS, which
that ruling allows.*

**Context.** D3.1 is one sentence — *the judge is never stronger than a seat*
— and it is checked against whatever a seat has at the time. Since
[ADR 0080](0080-linux-cage.md)'s amendment "the seat gets a socket wall" (D27)
a bwrap seat's launch line carries `posse wall-unix --` after bwrap's own
`--`: a Landlock domain handling `LANDLOCK_ACCESS_FS_RESOLVE_UNIX` and no
other right, so `connect(2)` and `sendmsg(2)` to a host pathname socket are
EACCES, **including to one an uncaged process binds after launch**. The judge
did not go through `renderWallInto`, so that step did not reach it, and D3.1
became false the moment D27 landed.

`--unshare-net` does not cover this. A pathname socket resolves through the
filesystem, not the net namespace — which is already why
`landedJudgeHides` exists (ranger-base-8ukiy) — and a hide cannot reach a
socket inside a **write grant**, because an empty tmpfs over a grant empties
the grant.

**D3.5. The judge's cage carries ADR 0080 D27's step, in D27's own position.**
`boxLandedCage`'s bwrap arm renders
`bwrap <args> --unshare-net … --ro-bind <self> <self> -- <self> wall-unix --
<judge>` whenever
`wallUnixAvailableFor(WallBwrap)`. `self` is the LAUNCHER's installed binary,
resolved through `wallWatchBin` exactly as the wall-watch sidecar's is, and
never a path out of the tree being judged: landing reviews nothing (D3), so a
step resolved from that tree would let the code under judgement choose its own
wall, which is worse than having no step.

The step's binary is **bound back read-only after every mask**
(`--ro-bind <self> <self>`). That is not belt-and-braces: this render's own
private `/tmp` and its hides can cover the file the wall is typed from.
MEASURED 2026-11-14 — with the bind absent and the launcher standing in from
`/tmp`, the cage answered `bwrap: execvp …: No such file or directory`, exit
1, a judge that never ran. The bind adds no read surface, because
`--ro-bind / /` already gives the judge every byte of the host no mask hid.

**D3.6. Below ABI 9 every verdict line carries the DEGRADED sentence.** The
step is not rendered there (ADR 0080 D31), and the judge has no persona and so
no `posse gates` line of its own — these three lines are the only place its
wall is ever reported. All three say it: green, red, and the one that says
there is no verdict. `WallUnixDegraded`'s own words, never a bare alarm.

**D3.7. The ADR 0097 venue must NOT take the step, and that is a decision.**
A Landlock filesystem domain forbids `mount(2)` — ADR 0080's probe measured it
(U9, U10) — and the venue exists so that the wall's live pins can nest a bwrap
of their own. Under `POSSE_WALL_VENUE=1` a refusal to nest is a FAIL on every
member (ADR 0097 D4), so a step riding in the shared argv would red all 32
pins and answer exit 3 at every landing. [ADR 0097](0097-the-walls-live-pins-measure-in-a-nesting-venue-after-each-landing.md)
D1 is amended to say so, and its subtraction pin reads "the judge's argv minus
the step, plus the three" rather than "plus the three and nothing removed".

The venue's own answer to this class is the hide it inherits, and it is the
only layer a cage that must nest can have.

**Measurements.** MEASURED 2026-11-14, D1 host, Linux 7.2.15-1-omarchy,
bubblewrap 0.13.1, Landlock ABI 10. Four cages, one nested
`bwrap --ro-bind / / /bin/true` in each:

| cage | socket-wall step | nested bwrap |
|---|---|---|
| D17 base (`--ro-bind / /`, no `--proc`) — a seat, and the judge | no | `bwrap: setting up uid map: Read-only file system` |
| the same | **yes** | the same, byte for byte |
| that base + `--unshare-pid --proc /proc` — the venue | no | **it nests (exit 0)** |
| the venue's shape | **yes** | `bwrap: Failed to make / slave: Operation not permitted` |

- **Rows 1 and 2 answer ranger-base-1xyso's question (5).** The nest helper
  (ADR 0096 D3) abstains in the judge exactly as before, and the refusal's
  WORDS do not change either: bwrap dies writing its own `uid_map` under the
  read-only base long before it reaches a mount, because D17 leaves `--proc`
  out. The Landlock wording exists only in a cage carrying both the step and
  a writable `/proc`, which is the one combination D3.7 forbids. The helper
  is a function of WHETHER a bwrap started and never of why not, so either
  refusal abstains outside the venue and reds inside it.
- **Row 4 is the trap, priced.** It is what the venue would have done had the
  step ridden in the shared argv, and D1's subtraction pin would have stayed
  green through all of it, because it compares two argvs and not what runs.
- MEASURED, this tree: the step on the judge's argv in that one position, both
  arms of D3.6 driven over a stubbed ABI, and the venue's argv carrying no
  `wall-unix` verb. Each was mutation-red one change at a time;
  `docs/notes.d/ranger-base-i92tj.md` lists the five mutants and what each
  printed.
- MEASURED, this tree: the judge, run in the cage this render builds and
  behind the real Landlock domain, still reads no byte of any planted
  credential and still runs
  (`TestLiveLandedCheckCageReadsNoSecret`, and `exit 7` passing through the
  step unchanged).

**ASSUMED, not measured.** That the host census ADR 0080 D28 rests on still
holds for the judge's own run dir: the venue has no socket wall and cannot
have one, so a pathname socket bound inside its write grant would be
reachable from it. Nothing binds one — the run dir is created per run by the
launcher under a name nobody else knows — and the hide answers every socket
directory the census found. A daemon that grew one would be visible as a
`--bind` in the verdict line's argv.

**Alternatives rejected.**
- **Leave the judge at parity with a seat by taking the step off the seat.**
  That is the l2gk9 class back on every seat on the box, to spare one judge.
- **Give the venue the step and let the pins abstain.** An abstention is never
  a pass (ADR 0097 D5), so this is 32 pins measuring nothing and a watchdog
  filing silence at every landing — the state ADR 0097 exists to leave.
- **Resolve `self` on the judge's PATH.** `exec.LookPath` is right inside the
  step, where the thing being resolved is the runtime (D29); here the thing
  being resolved is posse's own wall, and the PATH it would read belongs to
  the tree under judgement.
- **Guard the bind-back's absence with a refusal instead of binding.** A
  refusal costs the judgement; the bind costs one read-only mount of a file
  the cage could already execute.

**Beads.**

- ranger-base-i92tj
- ranger-base-1xyso
- ranger-base-q9bqu

## Amendment: a judge its own launcher's stop ended has not been silent, and a new launcher reads the tip it inherits (ranger-base-gihmm, 2026-11-21)

D6.3 filed "landed check silent at ~/src/posse" (ranger-base-gihmm) on two
runs that said `it was signalled (signal: terminated)`, after 82.9s and
50.0s. Those are far under the 10m bound, so neither was a hang. The judge
was not sick. MEASURED 2026-11-21 by richard (linux, 16 cores, uncaged, warm
shared cache, load 9.9): `scripts/posse-landed-check` over the same tree
`2722c3fb` is **green in 146s**, inside D6.2's 240s. The two answered runs
between the silences were green at 176.2s and 190.3s.

**What sent the signal.** Read off the unit journal and the watch log, both
runs ended in the same second as `systemd: Stopping posse dispatch --watch
(ranger)` (23:36:44 and 00:07:32 EST). Those were operator restarts of
`posse-dispatch-ranger.service`, issued after each delegated `posse upgrade`
so the loops would pick up the new binary (ranger-base-rrgnc). The unit
renders systemd's default `KillMode=control-group`, so a stop sends SIGTERM
to every process in the cgroup, and the judge's bwrap is in that cgroup. The
launcher never cancelled anything. It was waiting on the judge
(`defer d.landedChecks().Wait()`), but the judge was already dead. A terminal
does the same thing: ctrl-c sends SIGINT to the foreground process group,
judge included. So **every supervisor this loop runs under delivers its stop
to the judge as well**.

What that cost:
1. A false P1 in architecture's lane, filed by the watchdog whose job is to
   be believed.
2. An unjudged tip that nothing comes back for. `once()` runs only when a
   landing schedules it, so the new loop does not read main until the next
   landing. When this was written, main's tip `16c844cb` had been unjudged
   for about four minutes and would stay that way until something landed.
   rrgnc's remedy makes a loop restart part of every install, and installs
   follow landings, so this shape recurs by design rather than by bad luck.

**D6.5: a run ended by its own launcher's stop is INTERRUPTED. It is not
silence, not counted and not filed.** This is a third non-verdict next to
D3's silence. It records nothing in D4 (no answer was given) and nothing in
D6.3's `posse.landedCheckQuiet` (the judge was not the one that failed). It
prints one `◷` line: `~/src/posse landed at <tip> and was not judged: this
launcher is stopping (<said>) — the next launcher reads the tip`. A run is
INTERRUPTED in either of two cases:
- **The launcher stopped it.** When the watch loop's stop context fires, the
  checker cancels its in-flight run and drops its pending set. Today `Wait()`
  holds a stop for up to 10m, and under systemd that ends in TimeoutStopSec's
  SIGKILL anyway. After this change a stop is bounded by
  `generatorKillGrace`.
- **The stop's own signal reached it first.** The cgroup or process group
  delivers SIGTERM or SIGINT to the judge and to the launcher together, so
  the judge's `Wait` can return before `signal.NotifyContext` has cancelled.
  A run that comes back signalled therefore waits up to
  `landedInterruptGrace` (5s) for the stop context before it is classified.
  If the stop arrives, the run is INTERRUPTED. If not, it is D3's silence,
  as today. ASSUMED, not measured: the two deliveries land within
  milliseconds of each other, so 5s is a grace and not a guess at the gap.
  The only cost is up to 5s of latency on a real silence's ⚠ line.

Both D3 rules (the judge and ADR 0097's wall check) go through the same
checker, so this covers both. A signal from anyone OTHER than this
launcher's stop (an OOM kill, the load guard, an operator's `kill <judge
pid>`) still counts as silence. Those are exactly what D6.3 exists to see.

**D6.6: a launcher that arms reads the tip it inherits.** When a watch loop
arms, it schedules one landed check per repo it lands into, at that repo's
base. The cost is D4's cache: a tip that is already recorded costs one
`git config` read and returns. A tip that is not recorded (the restart case
above, or any landing made while no loop was running) gets judged about
2.5 minutes after the loop arms instead of at the next landing. A one-shot
`posse dispatch` does not do this. It has no loop to arm, and its landings
already schedule their own checks.

**Consequences.**
- Restarting the loops after an install no longer files architecture a P1,
  and no longer leaves main unjudged until the next landing.
- A stop no longer waits up to 10m for a judge. It cancels the judge and
  says so.
- D6.3's counter means what its title says again: consecutive runs in which
  the JUDGE could not answer.
- Exit hatch: one more verdict value, one grace constant and one arm-time
  sweep. Deleting the arm-time sweep restores today's behaviour (judged at
  the next landing). Nothing new is persisted.

**Alternatives rejected.**
- **Render `KillMode=mixed` in the unit so that only the main process gets
  SIGTERM.** That is the clever one, and it fixes only systemd. A terminal's
  ctrl-c still signals the whole process group, and so would any future
  supervisor. It also needs the launcher to cancel the judge on stop anyway
  (otherwise TimeoutStopSec SIGKILLs it after 90s, and that is still a signal
  the counter would count). The launcher-side rule covers every supervisor,
  so the unit stays as it is.
- **Let the judge outlive the stop (`setsid`, or a `systemd-run --scope` of
  its own).** The verdict would have no reader. D3 keeps the record with the
  launcher, and moving it into the judge would hand the record to the tree
  under judgement. A detached cage would also be a process ADR 0009's
  orphan rules then have to treat as wanted.
- **Make the restart wait for the judge (a drain in `ExecStop`).** That
  holds an install's follow-through for up to 10m to finish a judgement D6.6
  can redo in about 2.5m.
- **Raise D6.3's threshold to three.** That hides this case only until three
  restarts in a row, and it also hides real silences for one more landing.
  The counter was counting the wrong thing, not too few things.
- **Treat every SIGTERM as interrupted.** The load guard's kill arm also
  sends SIGTERM (loadguardkill.go), and a judge killed by the guard is a real
  silence. What matters is who stopped the judge, not which signal it got.

**Beads.**

- ranger-base-gihmm

## Amendment: the launcher that judged is named, and a window is not attributed to a wall that moved (ranger-base-7or9p, from ranger-base-q8j8x, 2026-11-21)

D1 is careful about where the judge's BYTES come from: the copy that runs is
the last tree recorded green's, never the tree under judgement, so a landing
that weakens the script does not change what judges it. D5's bead is written
as though that were the whole of it — "The newest landing is simply the one
whose tree first held them all" — and it is not. The judge's WALL comes from
the running launcher: the socket-wall step's binary is `wallWatchBin()`'s
answer (the 2026-11-14 amendment says so, and says why it must not come out of
the tree), the seatbelt profile is rendered by this process's code, and ADR
0097's venue is rendered by the same function. So a landing that changes the
wall is judged by the OLD wall until somebody reinstalls the launcher, and on
that install one tree's verdict flips with no landing at all.

MEASURED 2026-11-20 (ranger-base-ufd7z, re-read under ranger-base-q8j8x; this
box, 16 cores), with the TREE held constant at main's `edfe7a79` and only the
step's binary changed:

- a posse built at `b29c95b5` judged it **exit 2 in 139s**, with
  `FAIL github.com/ranger360ai/posse/internal/treepins 79.775s` and four
  bdargvgate pins failing on `branch -M` with `Invalid cross-device link`;
- the installed `0.5.3+edfe7a79` judged the same tree **exit 0 in 132s**, zero
  `--- SKIP`.

Production said it from its side: the judge commented `still red at
853242ca2dc2 (f1509bcfb970)` over a tree that already held the fix. What that
cost: one P1 filed and dispatched against a window of **eleven innocent
landings** and the persona of the newest one, a seat's session to establish
that no tree in the window could clear it, and a close whose stated timeline
is wrong in the record. Neither the verdict line nor the filed bead named the
binary that judged, so no reader could tell a tree red from a judge red.

**D7 — Every verdict line, and the bead D5 files, names the launcher that ran
the check.** Version, path and mtime, on the green, the red and the ⚠ that
says there is no verdict — D6.2's rule about the wall, over the other thing a
reader of those lines cannot otherwise get. Version AND mtime because they
fail differently: the version is the RUNNING code, which renders the cage and
the profile; the mtime is the FILE at the step's path, which is what the
socket-wall step execs, and a reinstall under a running launcher moves the
second and not the first. One reading per judgement, memoized, so a
judgement's line, record and bead cannot name three launchers.

**D8 — A green→red window is not attributed when the launcher is not the one
that judged the green tree.** The record is this rule's own `git config` line,
`posse.landedCheckBy` (ADR 0097 D5's rule reads `posse.wallCheckBy`), holding
one opaque token: version, mtime, size, path. It is written beside a GREEN
verdict and only there, and AFTER it — so what it holds is "the launcher that
judged the tree currently on record green", which is exactly the far edge of
every window D5 attributes, and it is still readable from inside the red run
that needs it. Three answers, not a bool, for `landedStretchState`'s reason:

| the record says | the bead |
| --- | --- |
| this launcher | D5's attribution, unchanged: assigned to the newest landing's persona |
| another launcher | **filed UNASSIGNED**, and the window is listed as what moved rather than as the cause. It names both launchers and says the wall is the launcher's and not the tree's |
| nothing, or a launcher that cannot name itself | D5's attribution, plus one sentence: whether the launcher moved could not be checked |

Withholding on `unknown` was rejected: the token is absent on every repo until
the first green lands under D8, so that reading would withhold every
attribution for one stretch and teach nobody anything. Saying so is what the
third answer is for.

**A third field on D4's record was rejected outright.** `landedRecorded` reads
a record in any other shape as NO record, which is deliberate — a half-parsed
verdict must never read green — and a record read as absent makes every red a
green→red transition again, which is one P1 per landing and the
ranger-base-4v4r6 incident D5 exists to end. A separate key is also the house
pattern for exactly this: D6.3's counter is its own key for the same reason.

**Consequences.**
- A reader of a red main's bead can tell a tree red from a judge red, in the
  bead, without rerunning anything.
- The ufd7z shape files the same bead and dispatches nobody: no persona is
  assigned over a wall that moved, and the bead says which two launchers
  disagree so that the cheap thing gets ruled out first.
- One more `git config` read per filed red and one more write per green. No
  new record shape, no change to D4's, D5's dedupe or D6.3's counter.
- ADR 0097 D5's rule inherits both rules through the shared parameter
  (`landedRule.LauncherKey`) rather than a copy, as D5 requires there.
- Exit hatch: one record key per rule and one drift reading. Deleting the key
  leaves every red reading `unknown`, which is today's attribution plus a
  sentence; deleting D7's note returns today's verdict lines.
- Not changed: what judges (D1's copy from the last green tree), the cage
  (D3), the coalescing and the cache (D4), the transition table (D5), the
  derivation and the two numbers (D6).
- Residual: D6.3's health beads name no launcher. Their subject is two or
  three RUNS, and a launcher restart can split those between two binaries
  (`docs/notes.d/ranger-base-4jroi.md` is that case as it was filed), so the
  reading this run took would be a claim about runs it did not take. Naming
  the launcher per REMEMBERED run is the shape that would answer it, and the
  sentences those beads carry are already held in memory rather than in the
  record, so that is a change to what is remembered and not to a record.

**Alternatives rejected.**
- **Judge with the launcher the green tree shipped** (build posse out of the
  tree under judgement, or out of the last green one). That hands the wall to
  the code under judgement, which the 2026-11-14 amendment refuses by name —
  "a step resolved from that tree would let the code under judgement pick its
  own wall" — and it would put a `go build` inside every landing's judgement.
- **Re-judge the last green tree when the launcher changes, and move the
  record forward.** It answers the attribution honestly and costs a full
  judgement per install, on a path D2 built to cost nothing and hold nothing.
  D8's reading is free and says the same thing to the one reader who needs it.
- **File the red into another lane, or not at all, when the launcher moved.**
  The red is real on this pair of launcher and tree: a seat that reinstalls
  will see it. Withholding the ATTRIBUTION is the narrow correction; moving
  the bead would hide a reading somebody has to make.
- **Name only the version.** A reinstall under a running launcher leaves the
  version alone and replaces the file the step execs, which is the half of
  this that flips a verdict with no landing.

**Beads.**

- ranger-base-7or9p
- ranger-base-q8j8x
- ranger-base-ufd7z
