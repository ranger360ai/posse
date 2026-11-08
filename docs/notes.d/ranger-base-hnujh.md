# install-hooks refuses another home's wall (ranger-base-hnujh)

ADR 0081 D4, implemented 2026-11-07. `posse gates install-hooks` was the
SECOND of the three writers that could rewrite any repo's shared commit wall
from any home on the box. D1/D3 took the launch (ranger-base-lh9nj); this
takes the verb. D7's queue commit (ranger-base-knb1v) is the third and is
still open as code.

The irony worth recording: install-hooks is the remedy every stale-wall
reader prescribes. So the verb that exists to put a wall right was also the
verb that took it away — a non-owning home reading `ours but stale`, doing
what the line told it, and handing the owning home the same finding back.
That is the two-writer loop ranger-base-q0np6 MEASURED (1897 lines against
1137, same binary, same repo, only `RHQ_HOME` changed), with the remedy
itself closing it.

## What the refusal is, exactly

Not "a non-owner may not install". That is the ADR's rejected alternative (c),
and it would stop arming every repo nobody listed — `~/code/...`, a `dirs:`
entry, a repo made seconds ago. The refusal is the conjunction:

    the slot holds another home's CURRENT render (the stamp says whose, D2)
    AND this home's `beads_visibility:` does not mark the repo (D1)

Every other state still installs, and each is a pin in
`cmd/posse/commitwallowner_qa_test.go`: an empty slot (D3 case 1), a body with
no stamp (case 4, the one-shot migration of everything written before the
ADR), and — once the scratch home marks the repo — another home's render
(case 2, with D6's two owners made visible by the stamp rather than refused).

`ForeignCommitWallOwner` (gates.go) is the whole of the new surface: it asks
`sharedCommitWallWrite`, the SAME predicate the launch asks, so there is one
rule and not two copies of one. It returns the stamping home and
`hookRepo(dir)` — the main checkout, since the file lands in the COMMON hooks
dir — because a refusal that named the session worktree it was typed in would
name a directory whose `beads_visibility:` line nobody can write
(ranger-base-up22, one verb over).

The probe is identity-only (`l3AskIdentity`). Ownership and the stamp are
per-repo identity facts; the behavior half execs this binary's own render over
the repo's index, which asks whether THIS BINARY's renderer still refuses — a
question about the binary, identical in every repo, and of no use to a write
rule. It also costs ~1s a repo (the `l3Ask` comment's own measurement).

## Why the target's spelling does not enter it

D4 says "an explicit target". Read as a second rule for `.` that would be a
hole a `cd` walks through — and q0np6's two scratch-home launches were
standing IN the repo, not pointed at it. Read as the ADR means it, the
sentence is about the operator's sweep over their own listed repos, and that
sweep is unaffected for a reason that has nothing to do with spelling: every
repo on this home's list is owned by D1, so the rule never fires there. So the
implementation asks the same question of the argument and of the `.` default,
and `TestQAInstallHooksRefusesAForeignWallInTheCurrentDirectoryToo` is the arm
that says so.

## What a refused run leaves behind

- The commit-guard slot: **bytes and mtime unchanged**. The slot is not
  re-read, not rendered and not opened on that path. Asserted as the
  done-when, mtime in nanoseconds, because bytes alone would pass a write
  that happened to render the same body.
- `pre-push`: **installed**, as on any other run. ADR 0081 D5 leaves that
  slot alone — `PrePushHook` is a constant with no home input, so a
  cross-home write produces identical bytes — and the two slots are reported
  independently (rangerhq-mgdk). A commit-guard refusal that took pre-push
  down with it would be the collateral that bug was about.
- Exit 1, and three lines: whose wall it is, both remedies (the
  `beads_visibility:` line, or installing from the stamping home), and that
  there is no `--force`. The last line is said out loud because the absence
  is a decision: the remedy is a config line, and a flag would put the second
  writer straight back the first time an operator reached for it (D4).

## Measured

- Five new pins (`cmd/posse/commitwallowner_qa_test.go`), all black-box runs
  of the built binary with `RHQ_HOME` pinned to one of two scratch homes.
  Black box because three of the four claims — the exit code, the untouched
  file, the surviving pre-push — are not a predicate's return value.
- MUTATION-CHECKED 2026-11-07, three mutants, each red in a different arm:

      the refusal removed (install-hooks writes unconditionally)
        → both refusal arms red, one on the exit code and one on the bytes
      ownership dropped (refuse on the stamp alone)
        → the "once this home owns the repo" arm reds; D6's case goes silent
      the stamp dropped (refuse on non-ownership alone — alternative (c))
        → the empty-slot and unstamped arms red, AND two pins that predate
          this bead (`ChainFlagTakesOverBdsShim`,
          `OnAWritableForeignPathStillInstalls`), which is the measurement
          of what (c) would have cost: ordinary installs into repos nobody
          declared

## The done-when, on this box

MEASURED 2026-11-07, darwin 25.6.0, with the built binary and a scratch repo
stamped by this box's OWN home (`RHQ_HOME=<box home>`, config and all — the
1892-line body, against the 1059 a scratch home renders for the same repo;
the ADR's own 1897/1137 one day earlier). A scratch repo rather than
`~/src/posse` on purpose: the stamping home is what has to be real for the
reading to mean anything, and pointing a possibly-wrong build at the box's
shared wall to prove it would not rewrite it is the hazard, not the
measurement.

The absolute home the command actually printed is elided as `<box home>`
below. `AbbrevHome` only shortens a path under the RUNNING process's `$HOME`,
and here `$HOME` is the scratch home, so the real line carries the owning
home spelled out — which is the point of it and is also why it cannot be
quoted verbatim in a tracked file (`make identity-check`, which caught this
paragraph's first draft).

    $ env RHQ_HOME=<scratch> HOME=<scratch> posse gates install-hooks <repo>
    installed <repo>/.git/hooks/pre-push (refuses git push when …)
    not installed: prepare-commit-msg — <repo>'s commit wall was rendered by
    home <box home>, and this home does not own it (ADR 0081 D1/D4)
      remedy: add `<repo>: <private|public>` under this home's
      beads_visibility:, which is the list that confers ownership — or install
      from home <box home>, which rendered what is there
      no --force: two homes writing one wall is the contention D1 removes, and
      the last writer would keep winning
    exit=1

`shasum` and `stat -f %Fm` identical across the run, to the nanosecond
(1794108421.690600249 both sides), 1892 lines before and after. pre-push
installed, per D5.

BOTH REMEDIES then exercised, because a refusal that names a door nobody can
open is worse than no refusal: installing from the stamping home exits 0 and
re-stamps as before; adding the repo to the scratch home's
`beads_visibility:` and installing again exits 0 and takes the wall to 1059
lines under the scratch home's stamp. That second one is D6 — two owners is
misconfiguration the stamp makes visible, not a refusal — and it is also the
measurement of what the refusal is worth: 833 lines of guard, which before
D4 any home could take off any repo by typing the remedy for a stale wall.

## Still open

D7 as code — `CommitQueueJSONL`'s `a.InstallCommitGuardHook(q)` before every
jsonl commit (`internal/posse/queuejsonl.go`), the third cross-home writer.
The ADR decided it (ranger-base-knb1v landed D7's text: the call site probes,
asks D3's rule, and a non-write refuses the projection commit); the call site
still installs unconditionally. `ForeignCommitWallOwner` is the predicate that
call site wants — it holds no probe of its own either.
