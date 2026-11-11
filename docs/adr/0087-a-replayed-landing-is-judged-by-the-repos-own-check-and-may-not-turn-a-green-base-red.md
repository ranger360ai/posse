# ADR 0087 — A replayed landing is judged by the repo's own check, and may not turn a green base red

*Status: accepted 2026-11-10 (ranger-base-qk7w6, from ranger-base-1tt2e /
ranger-base-s3a9g) · owner: architect · builds on ADR 0011 §1 (the launcher
lock) and ranger-base-7h8k4 (the generated-index precedent)*

## Context

`MergeSessionWork` (internal/posse/worktree.go) lands a session branch in one
of two ways. If the base has not moved, it fast-forwards to the tip the seat
committed. If the base moved, it replays the branch onto the new base in the
session's own tree and then fast-forwards to the replayed tip. **Nothing
compiles that replayed tip.** git only refuses textual conflicts. A semantic
conflict lands silently.

holden measured this on ranger-base-s3a9g. 4fbef672 (on 1089eb7f) and
ranger-base-r5131's 10d046f9 each built clean. The launcher replayed the
first onto the second as a2327230, which `go build ./...` refuses with two
errors and `go test -c` with seven more. Every seat's `make test`,
`tree-check` and `coldexec-check` went red, `make install` could not run, and
`checkorphans` exited 1. ci.yml runs after the landing, so it saw this only
once main was already red.

The bead asked four questions: what command, what it costs inside the lock,
blocking or advisory, and what a red result means. The field has a standard
answer to the shape. It is a merge queue: test the combined result, and
advance main only on a pass. bors-ng "implements a continuous-testing workflow
where the main branch never breaks", testing in a staging branch and
fast-forwarding main only on success. GitHub's merge queue tests each PR
"applied to the latest version of the target branch", and a failing PR "will
be removed from the queue" (sources below). What differs here is that main
also moves outside the queue (the operator's checkout, and the unreplayed
first arm). So main can already be red when a branch arrives, and the gate
has to handle that case.

**MEASURED 2026-11-10 on the D1 host** (Linux 7.2.15, 16 cores, go 1.27.3,
`GOTOOLCHAIN=local`, load 12–17, from this bead's worktree):

| what | time |
|---|---|
| `go build ./...` + `go vet ./...` + `go vet -tags posse_arm2` + `-tags posse_arm3` over `./internal/posse`, tree 20 commits / 86 files away from the warm cache | 2.5 + 1.4 + 1.1 + 0.9 = **5.9s** |
| the same four, empty `GOCACHE` | 8.8 + 2.6 + 2.0 + 2.6 = **15.9s** |
| `GOOS=darwin go vet ./...`, first ever, then again | 17.7s, then 0.9s |
| commits on main in the last 7 days, and how many were replayed (author time ≠ committer time) | 220, of which **147** |

## Decision

**D1 — The command is the repo's, found by a shipped file.** A repo opts in
by shipping an executable at `scripts/posse-land-check`. This follows the
precedent of `generatorShipped` and needs no config key. The name carries the
posse prefix so a repo's own `scripts/` cannot match it by accident. The
contract is: no arguments, cwd is the tree to judge, exit 0 means green, any
other exit means red, and the last lines of combined output are the cause.
posse always runs **the base checkout's copy**, the copy a human reviewed,
against the session tree. A branch that edits the script does not change what
judges it. A repo without the script lands exactly as it does today.

**D2 — The check runs on the replay arm only, immediately before the
fast-forward.** That is after the second `notOnBase` and the second
`refreshGeneratedIndex`, on the exact tip the fast-forward will take. This is
correct by construction, not narrowed. `merge --ff-only` is a compare-and-swap
on the base ref. If it succeeds, the base is still the `wasAt` the replay was
built on, so the tree that lands is the tree that was judged. If the base moves
during the check, the swap fails and the existing bounded loop replays and
judges again. The first fast-forward arm stays unchecked (see Alternatives).

**D3 — It blocks only when it would turn a green base red (the ratchet).**

| replay tip | base | outcome |
|---|---|---|
| green | any | land; record the tip's tree as green |
| red | green | **block**: reset the branch to its pre-replay tip, then file the merge-back block with the check's output |
| red | red | land, print ⚠ "base was already red, landed without judging" |
| no answer (timeout, cannot exec) | any | land, print ⚠; never block |

That one rule answers questions 3 and 4 together. Only "the two sides
disagree" (this incident) is the branch's fault, and only that case blocks. A
base that someone else broke strands nobody. A partial fix still lands, which
matters because the fix for a red main comes through this same path.
ranger-base-17puk did. A gate that cannot answer never strands the fleet,
because ci.yml stays the backstop.

**D4 — The base's verdict is cached by tree id, and normally costs nothing.**
One repo-level git config record, `posse.landCheck = <tree-id> <green|red>`.
After a green landing the record already holds the new base's tree, so in the
common case "is the base green?" is a lookup. Only a red replay over a base
whose tree is not on record pays a second run. That run checks out `wasAt`
detached in the session tree, runs the check, and restores the tree. It keys
on the **tree**, not the commit, because the check reads bytes. The script
lives in the tree, so changing it changes the key.

**D5 — A block leaves the branch where the seat left it.** `rebase --abort`
already gives that guarantee for a conflict. A replay that succeeded has
rewritten the branch, so the block is `git -C <session tree> reset --hard
<pre-replay tip>`. That tree is clean (the dirt check ran first), it is not
the shared checkout, and the reset discards only the launcher's own replay and
any index commit it made. The block reuses the existing machinery: `o.Reason`,
then `noteMergeBlocked`, and the landing sweep's `probedBase` record re-asks
it once per base movement, as it does for a conflict. No new mark is needed.

**D6 — Bounded like the generator, and executing nothing the branch wrote.**
The check runs operator-side, uncaged, and under the launcher lock. So it gets
`generatorTimeout`'s bound and `generatorKillGrace`'s reaping, and its contract
forbids executing branch-authored programs. For this repo that rules out
`go test` and `make`. `go vet` type-checks test files without running them,
which is how it catches the seven `go test -c` errors. This repo has no cgo,
and `GOTOOLCHAIN=local` stops go.mod from fetching and running another
toolchain. This repo's script is `go build ./...`, `go vet` over every
`posse_arm*` tag the tree declares, and `GOOS=darwin go vet ./...` (the darwin
files compile only there).

**Cost (question 2).** About 6s warm and 16s cold per replay (MEASURED above),
paid inside the lock, only on the replay arm. Up to ×3 on the retry loop, and
×2 on the rare red-over-unknown-base. Replayed commits ran about 21 a day this
week, and landings are fewer than commits. That gives an upper bound of about
two minutes of lock time a day warm (ASSUMED from the commit count; landings
were not counted directly). That is affordable. The exit hatch if it stops
being affordable is in Alternatives.

## Consequences

- Main cannot go from green to red through a replayed landing that the check
  covers. The night of a2327230 becomes one merge-back bead for dinesh's
  branch.
- Residuals, stated: (1) the unreplayed arm can still land a tip the seat
  never built; that is a seat-discipline failure, and ci.yml catches it. (2)
  A red-over-red landing can add errors that nobody attributes; the ci-red
  bead covers it. (3) A toolchain upgrade does not change the tree key, so the
  cached verdict can outlive it by one landing. (4) The check sees only what
  it compiles: a semantic conflict that type-checks still lands. Tests are not
  in the lock (D6).
- posse stays repo-agnostic. The only new contract is one script path.
- **This repo's own check shipped under ranger-base-pn621, and the price was
  re-measured on it.** `scripts/posse-land-check` is D6's body, with two things
  D6 left to the build: the arm tags AND the packages to vet them in are
  derived from the tree's own build-constraint lines rather than listed — BOTH
  spellings the toolchain honours, `//go:build` and the legacy `// +build`,
  which the derivation read as one until ranger-base-n2yaw — so a
  fourth arm is covered by the commit that declares it and a fourth arm in
  another package is not vetted into a hole; and the cross-GOOS step reads
  "whichever of darwin/linux is not native", which on the D1 host is
  `GOOS=darwin go vet ./...` verbatim and on a darwin launcher is the mirror
  image rather than a redundant native vet beside an unvetted linux arm.
  MEASURED 2026-11-11 on the D1 host, the script run against trees exported
  with `git archive` against the fleet-warm build cache: **7.1-9.0s warm** at
  loadavgs of 3.4 to 13.8, and **30.1s cold**. Warm holds against the 5.9s in
  the table above — it is 7s rather than 6s because that table timed its steps
  over a cache warm for all of them, and a replay's tree has files in it
  nothing has compiled — and the cold figure is the same table's 15.9s plus
  the darwin vet measured beside it, so the cost paragraph stands and the
  exit hatch stays shut. The check exits 1 in 297ms at a2327230 naming both
  build errors, and 0 at 10d046f9, so the first Consequence above is witnessed
  rather than inferred. docs/notes.d/ranger-base-pn621.md carries the numbers,
  the derivation witnesses, and one residual D6 does not state: the
  `!darwin && !linux` files compile under neither GOOS, here or in ci.yml.

## Alternatives rejected

- **Advisory only (land, file a P1).** It costs no lock time, but it is
  exactly what ci.yml already does, one step earlier. Main still goes red for
  every seat. The ratchet blocks the one case advisory cannot afford and is
  advisory everywhere else.
- **Blocking on any red.** A base that someone else broke would strand every
  branch, including the fix. That is the failure question 3 was afraid of.
- **Build the base on every replay to attribute.** It always doubles the cost.
  D4's cache gets the same answer for free whenever the base came through the
  gate.
- **A `land_check:` config key in the home.** It coins a surface the home has
  to keep in sync with each repo's toolchain. The repo knows its own build, and
  the shipped-file pattern already exists.
- **Hardcode `go build ./...` in posse.** That ties the launcher to one
  toolchain, and it misses the test-file and darwin errors that this incident
  and ci's macos leg care about.
- **Run the full suite (or `go test -c` plus running it).** That executes
  branch-authored code uncaged as the launcher, and it costs minutes inside the
  fleet lock.
- **Check the first fast-forward arm too.** The clever one: it would make the
  ratchet total. But that arm has no clean replay tree. The session tree may be
  dirty there (the dirt check comes later), so the check would judge bytes that
  do not land. It would need a scratch checkout per landing for a case that is
  the seat's own tested tip.
- **Replay and check outside the lock, then take it only for the swap.** This
  is the exit hatch, not today's design. The ff is already a CAS, so the
  correctness argument carries over. But the sweep and the replay both write the
  session tree under the lock today, and moving that is a restructure that 6s
  does not justify. Revisit it if lock time is ever measured as the fleet's
  bottleneck.
- **Not-rocket-science as a separate queue process (bors/Zuul style).**
  Batching and speculative parallel runs buy throughput this fleet does not
  need at roughly 21 replays a day. They add a second actor that owns main,
  which ADR 0011 §1's single lock exists to avoid.

## Sources (verified live 2026-11-10)

- **[docs]** GitHub, "Managing a merge queue" —
  https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue
- **[docs]** bors-ng README (archived 2024-04-04) —
  https://github.com/bors-ng/bors-ng
- Graydon Hoare's "Not Rocket Science Rule" (2014) is the usual citation for
  the rule, but its known URL returned 404 on 2026-11-10, so it is named here
  and not cited as verified.
