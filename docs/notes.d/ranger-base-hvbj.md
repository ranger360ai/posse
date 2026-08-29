## Probe fixtures: the cleanup is what the audit flags (ranger-base-hvbj)

Two session-fixture probe pairs reddened `make test` on main on 2026-08-29 —
c4f5296/a7b80a4 and 21b82e8/71fa30f, both landing a three-line `PROBE.md` at
the repo root and then removing it, under fixture beads ranger-base-a4lz and
ranger-base-3j3t (the live half of ranger-base-i0qp). Third audit red in one
day. `go test ./...` was green on all three packages; the audit is what exits
non-zero, and the release workflow runs it.

**The add is never flagged. Only the removal is.** The deletion rule
(rangerhq-ypn1) fires on a path returning to a state it held before its
immediately preceding change, and absence is one of those states — which is the
add-only half of the rangerhq-8rtf mechanism exactly. So the fix is not to
teach the detector about probes, it is to stop deleting them:
**a probe fixture that lands STAYS**, at `docs/probes/<bead-id>.md`.
`docs/probes/README.md` is the convention; the two hits are triaged in
`scripts/silent-reverts.allow` with their reasons taken from the fixture beads'
own comments, and each line was controlled by stripping it and watching its hit
come back.

**The pair exception was implemented and measured, and it is rejected.**
"A path added and deleted with nothing touching it in between" is not a probe
signature, it is `plant_addonly`: implementing it turns the self-test's addonly
arm into `self-test FAIL: planted addonly revert not detected` and reds
`TestAuditFlagsAddOnlySilentRevert` (exit 0, want 1),
`TestSilentRevertSelfTestStillFires` and
`TestSilentRevertSelfTestHasTheStrnumArm`. It does not even do the job: on this
history it cleared one of the two probe hits and left the other, because
`PROBE.md` was added and deleted twice and so held four states rather than two —
and it silently un-flagged e82338c, a rename+edit already triaged. A detector
change whose failure direction is a false NEGATIVE has to be measured against
the pins that exist before it is proposed, not after.

**Also worth keeping: the audit's own self-test cannot run under the L1 commit
wall.** `plant_repo` spells `git commit -qm`, which any persona carrying
`Bash(git commit unless --)` refuses, and the wall lives in
`$RHQ_GATES_DIR/bin` — a directory whose name is the *persona*, not `gates/bin`,
so a PATH strip that greps for `gates/bin` misses it and the self-test reports
`modify rig did not reproduce the mechanism` for a reason that has nothing to do
with the detector. Strip `"$RHQ_GATES_DIR/bin"` by its full value. The harness
is right to fail loudly there (ranger-base-z4vx put that guard in), but the
message names the rig, not the cause.
