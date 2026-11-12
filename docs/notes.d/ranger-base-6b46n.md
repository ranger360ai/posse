# The landed-check cage gets the seat's secret masks, and a census holds every cage render to them (ranger-base-6b46n)

2026-11-11. Two findings off the verify bead ranger-base-4vium.

## What was open, and is closed here

`boxLandedCage` (internal/posse/landedcheck.go) rendered ADR 0088 D3's cage
with an empty `SeatbeltCarveOut{}`. Its comment argued that was deliberate:
the grants are a fresh worktree and a build cache, so there is nothing inside
them to narrow. That is right about the WRITE half and wrong about the READ
half. Under `--ro-bind / /` (and under a seatbelt profile that allows every
read it does not deny), an empty `DenyRead` is the host's whole read surface.
So the judge, which is landed code that nobody reviews and which D3 cages for
exactly that reason, could read every ssh key the box pushes with and every
runtime's credential file.

The fix: before the mechanism switch, the cage builds its carve's read half
from the same two sources a seat's wall uses:

- `credentialReadDenyLiterals(runtime.GOOS, nil)`. No state dirs means no
  runtime owns this child, so no credential is spared.
- `withSSHKeyReadDeny(nil, tree, &carve)`. A nil PID is not the pusher
  (`bwrapMasksSSHKeys`), so ssh's own key list is masked.

Both mechanism arms render that one carve.

## The census

`TestQAEveryCageRenderCarriesTheSecretMasks` (internal/posse/landedcheckcage_qa_test.go,
door: `make pkgcensus-check`) sweeps every non-test function in the module.
Any function that calls `BwrapArgs` or `SeatbeltProfile` must do two things
before its first render: call `withSSHKeyReadDeny`, and reach either
`SeatbeltCarveOut` or `credentialReadDenyLiterals`. Three paths render today
(bwrapRender, RenderSeatbelt and boxLandedCage), and three is the floor.

## Verified

MEASURED 2026-11-11 on this Linux box with bwrap:

- `TestLiveLandedCheckCageReadsNoSecret` runs a judge in the cage
  `boxLandedCage` builds. The judge reads no byte of eight fixture ssh keys
  (the ighvi fixture config) or the two runtime credential files the
  fixture plants for claude and codex. The control, the identical `BwrapArgs` call with the empty
  carve this replaced, reads all ten.
- Mutation 1: restoring the empty carve on the bwrap arm reds both the live
  pin and `TestQALandedCheckCageRendersTheSecretMasks`, 10 secrets each.
- Mutation 2: running the census against HEAD's landedcheck.go (before the
  fix) reds boxLandedCage on both the ssh and the credential arm.

## Not closed here

- ranger-base-hy1aj
