# The identity wall cannot wall the product's own name (ranger-base-c7i8t)

Built 2026-11-09 for ADR 0024 D2 check 3's amendment of the same date
(decided in ranger-base-87jw1, found on the way past in ranger-base-sydnc).

## What changed

`DeriveIdentityLiterals` skips the `username` class when the box's account
name is, case-folded, one of posse's own shipped names:
`path.Base(posseModule)` plus `retirableExampleNames(posse.Seed)`. The two
halves live in one function (`shippedPosseNames`) that the crew arm
(`DeriveCrewLiterals`) reads as well, so the two arms of check 3 cannot
drift — a lane named for the product broke the crew arm exactly as that
account name broke the username arm, over the same paths.

The set is a constant and the embed. It never reads the tracked tree: a name
excluded because the tree already carries it would let any leak that landed
license every later one.

The username also gained a seam (`currentUsername`, a package var over
`user.Current`), because it was the one check-3 source whose value no fixture
could choose — the e-mails are git config and the instance path is a
directory a fixture makes.

## The measurement, and why it needed two halves

MEASURED 2026-11-09, darwin 25.6.0 arm64, over the commit-wall cluster
ranger-base-sydnc defined — every test in the files that build a `visWall`,
`crewWall`, ceiling wall or instance wall, which is 23 files and 142 pins
before this bead's own pin, run per suite arm with the username forced
through an overlay over the seam (the tree untouched, the same method
ranger-base-sydnc used before the seam existed). Counted as unique
top-level pins across the three arms:

| configuration (account `posse` unless stated) | red |
|---|---|
| this box's own account, everything in place (control) | 0 |
| `posse`, everything in place | 0 |
| `posse`, exclusion AND fixture pin removed (the pre-fix code) | 29 |
| `posse`, exclusion removed, fixture pin kept | 6 |
| `posse`, exclusion kept, fixture pin removed | 16 |

The third row reproduces ranger-base-sydnc's 29 at 7a14a4f3 exactly, arm by
arm — 11, 12, 8 — which is the check that the cluster being measured is the
same cluster.

The fourth and fifth rows are the finding worth keeping: **the production
exclusion alone does not make that host green, and neither does the test
change.** Sixteen pins stage the derived username and assert a refusal over
it, so under an account whose name posse ships they stop having anything to
stage and fail on a fixture premise — the exclusion working correctly reds
them. They were only ever green because the account they ran under happened
not to be one of those names. So the fixture pins the username
(`qaFixtureUsername`), and those pins now say the same thing on every box;
the two censuses that must keep reading the real box identity
(`make identity-check`) do not go through the fixture and are unchanged.

## Mutation checks

Each by overlay, tree untouched:

- exclusion dropped: the four shipped-name subtests of
  `TestDeriveIdentityLiteralsSkipsAShippedUsername` red, its control passes.
- skip made unconditional: only that control reds, plus
  `TestQAAnUnshippedUsernameDerivesEvenWhenTheTreeIsFullOfIt`.
- skip rewritten to "whatever the tracked tree carries": both pins red. That
  is the rewrite the design bead said the fix must not be, it passes every
  other pin in the cluster and both identity censuses, and it is the one a
  reader is most likely to think is equivalent.

## Residual, unchanged from the ADR

A username that is a common word but NOT a shipped name (`dev`, `go`) is
still a boundary-free match, and refuses honest commits until it is renamed
or overridden. It does so loudly, at the first commit, naming the class and
the value.
