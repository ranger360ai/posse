## Open was the store's promise, not the query's (ranger-base-bwrp8)

`Bd.OpenLabeledAny` (`internal/posse/beads.go`) said OPEN in its name and its
doc comment and asked for nothing of the kind. It ran `bd list --label-any
<labels> --json --limit 0` and returned whatever came back, so the filter was
the STORE's default and not the query's — and bd 0.50.3 ships two store
classes that answer that default differently.

### The measurement

Measured 2026-09-04 on bd 0.50.3, both classes:

    the shop's own queue store (SQLite):
      bd --no-daemon list --label-any qa --json --limit 0        →   5 rows,   0 closed
      bd --no-daemon list --all --label-any qa --json --limit 0  → 396 rows, 391 closed

    a store `bd init` writes TODAY (no-db: true, JSONL only), throwaway repo:
      bd --no-daemon list --label-any ci-red --json --limit 0     → the CLOSED bead comes back
      only an explicit --status open drops it

Nothing on this box was affected, which is why it never showed: the shop
store is SQLite. A fresh instance is the other class, because a fresh
`bd init` writes it.

### Who would have mis-read, and in which direction

Both readers ask what is still WAITING, and both fail expensively when a
closed bead answers:

- **governance G3** (`govern.go`, `-l question` / `-l risk`) would count
  questions the operator already answered and hold a gate over them.
- **the closed-dirty handoff's dedupe** (`closeddirty.go`, `openPrefixedBead`
  → `openMatchedBead`) would adopt a handoff somebody already finished and
  never file the next one — the tree stays dirty and nobody is told. This is
  `ranger-base-17ui3`'s failure mode with the sign flipped: there, closing a
  block re-armed the filing; here, a closed bead suppresses it forever.
- `settleopen.go` and `settleescalation` read the same query and inherit the
  fix.

ci-watch had already defended itself locally (`ciOpenBeads` asserted
`Status != "closed"`), which is what found this: `ciwatch_live_test.go` runs
against a `bd init` store and failed on it.

### The fix, and why it is a filter and not `--status open`

The closed rows are dropped in `OpenLabeledAny`, where the doc comment
promises it, on both store classes. `--status open` — the obvious one-word
alternative — is a NARROWING, not a fix: bd's statuses are open, in_progress,
blocked, deferred and closed, and `--status open` answers with only the
first. Measured on the shop store, `--label-any qa` is 3 open and 2
in_progress, and a question somebody is holding is still a question nobody
answered. Only `closed` is an answer.

`AllLabeledAny` is untouched and still keeps its closed rows on both classes:
the merge-back dedupe wants the closed verdict (`ranger-base-j8qmj`), and a
fix that filtered both would take it away.

The local guard in `ciOpenBeads` came OUT with the general one going in. Two
guards for one invariant is not belt and braces here — the duplicate is
exactly what would have hidden a regression of the general fix from
`ciwatch_live_test.go`, the only arm that reads the second store class for
real.

### The pins, and the fake that could not see this

`internal/posse/openlabeledany_storeclass_test.go` — three hermetic pins
(the query itself, G3, the closed-dirty dedupe), all three on the
`fake-list-keep-closed` marker `ranger-base-x9e34` taught the fake, which is
the second store class. Plus one live arm in `ciwatch_live_test.go:229`
against real bd's own no-db store, which is what makes "both store classes" a
measurement rather than a model.

Two mutants, both killed (`go test -overlay`, so neither reached the tree;
green unmutated in the same runs):

    filter deleted            → all three hermetic pins fail, and the live one
    filter → --status open    → the query pin fails on the in_progress row

The second mutant needed a fake change of its own. `fakeBd`'s `list` ignored
`--status` entirely, so it served the narrowing the RIGHT answer and the
mutant survived — a fake blind to a filter flag cannot show a narrowing.
`fakeBdFilterStatus` (`herdr_test.go`) honours it exactly, and an explicit
`--status` overrides the default open filter the way `--all` does: measured,
`list --label-any qa --status closed` answers with 397 rows on the store
whose bare `--label-any qa` answers with 5.
