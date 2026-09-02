## The residue the reaper skipped: crew-marked and stampless sessions (ranger-base-f6lk)

The end-of-pass auto-reap (`internal/posse/autoreap.go`) takes a session when
the store of record calls its bead **closed** and **nobody is working in it**.
Two populations could never reach that test at all, so they accumulated and
the operator reaped them by hand — the exact mechanism the sweep exists to
replace (operator, 2026-08-29 night; one session idle 12h+).

| population | why it was skipped | what takes it now |
|---|---|---|
| a crew mark on a session **dispatch** made | ADR 0008 §2 — a crew session is outside every sweep | closed bead + settled + empty tree + `reap_crew_after:` (4h) |
| a crew session the **operator** made | the same | nothing, at any age |
| `pulse_persona:`'s session | — | nothing (ADR 0027 has nowhere else to deliver) |
| a per-bead-named session with no `bead:` pointer | ranger-base-kftx — nothing can ever supply the id | settled + empty tree + `reap_unpointed_after:` (1h) |
| the persona's reusable `<persona>-<repo>` slot | it is what the next resume rejoins | nothing |

### The crew arm's discriminator: render the name, never invert it

kftx established that a session name is a **lossy** encoding of a bead id
(`sessionSanitizeRe` folds `.` into `-`), so a name can never be read back
into an id. This goes the other way and is therefore not that inference: the
session's own meta carries `agent:`, `dir:` and `bead:`, so the name dispatch
*would* have given it — `SessionForBead(persona, dir, bead)` — can be
rendered and compared to the name it has.

The pointer is half the evidence on its own. `Bead:` at creation is set at
exactly two call sites, both in `dispatch.go`, and `posse new` has no flag
that can set it (`parseNewFlags`: `--dir --env-file --cmd --emoji --agent
--runtime --tier --allow-degraded --cage`) and no way to cut a worktree
either. A hand-made session that carries a pointer got it later from `posse
prompt` (ADR 0008's adb7 amendment) — and *that* session wears the operator's
own name.

**The residual, stated rather than hidden:** an operator who types dispatch's
exact name into `posse new` and is then hand-dispatched that same bead is
indistinguishable from dispatch's own session. It pays the 4h grace, the
settled test, and the empty-tree test for it.
`TestAutoReapSkipsAHandLaunchedSessionOnTheCrewMarkNotThePointer` used to
construct exactly that shape as its fixture; it now constructs what `posse
new` actually produces, with the residual written into its comment.

### A kill must take nothing (`residueHolds`)

The narrow arm proceeds over a dirty tree and warns — a closed bead over
uncommitted work is ADR 0041's business and the landing prints `KEPT`. These
two arms do not: they rest on less evidence, so they demand that a kill take
nothing that nothing else holds. `residueHolds` asks `RemoveSessionTree`'s own
refusal as a **question** rather than performing it as an act, over the same
records:

- `git status --porcelain` in the session's cwd — the only shape of loss a
  kill can cause in a shared checkout (`reapguard.go`, ranger-base-0fb, the
  353 uncommitted lines);
- the session branch, when there is one: ahead by sha is not ahead by work
  (ranger-base-g2xf), and only the half that MEASURES content licenses a
  destroy — every commit patch-id equivalent (ranger-base-as19: git's `-x`
  trailer is somebody's decision, not a measurement) AND the base holding the
  branch's bytes for every path it touched (ranger-base-x8jp: patch-id
  normalises whitespace).

A branch that no longer exists is the last copy of nothing and passes. Every
question that cannot be answered fails **closed**.

This is stricter than the kill that follows needs to be — a crew arm session's
bead is closed, so `KillSessionAndLand` would land the branch itself. That is
deliberate: a reap that lands is a reap that decides, and these are the two
arms the operator did not previously trust to decide anything. Deferring costs
one pass — `landClosedTrees` lands the same branch at the head of the next one
and the sweep after that finds nothing held. The refusal prints a `◑ … NOT
reaped: …` line **every pass it is true**, on `landsweep.go`'s rule: the
silence is what read as a broken reaper and cost the hand-reaps.

### The graces are policy dials, not measurements

No number here was read off the fleet. `reap_crew_after:` (4h) has to outlast
a conversation's own gaps, and posse records nothing that says how long those
are — typing in a pane leaves no stamp, which ADR 0008 §1 accepted when it
refused a timer. `reap_unpointed_after:` (1h) protects much less: nobody is in
there, no bead exists to be unfinished, and the tree is provably holding
nothing, so past `RelaunchGrace` all it buys is not racing a session somebody
is about to use. Both are config; `off` / `never` / `0` on either restores the
permanent skip, spelled rather than numeric so a typo cannot read as "no
grace". An unparseable value is named on stderr and the default stands.

### The age itself

`residueIdle` is the later of `launched:` and `prompted:` — the only two
moments any record here is stamped with. It is **not** "how long since the
operator typed", and does not claim to be: what covers that gap is the guard
one step earlier, `settledForReap`, because typing is what herdr reports as
`working`. A meta with neither stamp has no age, and no age is not old enough.

### Verified

Seven mutants, each killed (`internal/posse/reapresidue_test.go`, plus the two
amended pins in `autoreap_test.go`/`autoreap_qa_test.go`):

| mutant | reds |
|---|---|
| crew arm stops rendering the name | `…NeverTakesACrewSessionTheOperatorMade`, `…KeepsACrewSession`, `…SkipsAHandLaunchedSession…` |
| the `pulse_persona:` exclusion goes | `…NeverTakesThePulsePersonasSession` |
| the grace never bites (`idle < grace-grace`) | both `…InsideItsGrace`, `…LeavesADialFNamedSession…`, `…UnreadableReapGrace…` |
| `residueHolds` always answers "" | `…OverAnUncommittedTree`, `…HoldingUnlandedCommits` |
| the unpointed arm goes | `…TakesAStamplessSessionPastItsGrace` + 4 |
| `off`/`never` not honoured | `…OffRestoreThePermanentSkip` |
| an undated record reads as infinitely old | `…UndatedResidueIsNotOldEnough` |

Deleting the grace check outright is a **[build failed]**, not a kill (`grace`
goes unused) — hence the `grace-grace` spelling above.
