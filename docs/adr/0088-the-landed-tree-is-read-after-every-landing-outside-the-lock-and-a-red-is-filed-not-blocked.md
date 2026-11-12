# ADR 0088 — The landed tree is read after every landing, outside the lock, and a red is filed, not blocked

*Status: accepted 2026-11-11 (ranger-base-oe9ja, from ranger-base-qregu) ·
owner: architect · builds on ADR 0087 (the replay gate and its ratchet) and
ADR 0011 §1 (the launcher lock)*

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
