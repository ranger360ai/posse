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
