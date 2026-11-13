# A fixture marshaller that ranged a map blamed the code under test (ranger-base-fpny6)

`TestUpgradeRollsBackWhenAParityBlockGainsANotHeldRow` red roughly one run in
three and said the rollback had not restored the binary. The rollback was
never involved. `fakePosse.bytes()` in `internal/posse/upgradeverify_test.go`
rendered its `GATES` rows by ranging `f.Gates`, a `map[string]fakeGates`, and
every rollback pin in that file calls it **twice**: once through
`newD4Fixture`, which writes the bytes to the target, and once through
`rolledBack`, which compares the live file to a fresh rendering. With one
persona in the map the two calls cannot disagree, so the whole family passed;
that one pin is the only one with two, and it was the only flake.

MEASURED 2026-11-13, D1 host, at 2dc8dd75, `go test -count=20 -run
'^TestUpgradeRollsBackWhenAParityBlockGainsANotHeldRow$' ./internal/posse`: 7
of 20 red. **Both** orders appear among those seven —
`GATES ops 0 0\nGATES dev 0 1` and `GATES dev 0 1\nGATES ops 0 0` — which is
the tell. Neither order was wrong, and no sorting of the *written* file would
have been the fix on its own: the defect is that the marshaller was not a
function of its receiver, so the two renderings agreed only by luck. Sorting
makes both calls agree, which is the property the pins actually rely on.
After the fix, `-count=25` over that pin and the new one: green; `-count=3`
over `-run 'Upgrade|FakePosse'`: green.

The failure message is the expensive part and worth noting on its own. "the
target holds %q after the rollback, want the binary that was live before the
run" points a reader at `upgradeRollback`, which copies bytes and is
byte-faithful — so the search starts in the one file that could not be at
fault. A fixture that renders what it also asserts against should be pinned
as deterministic in its own right, which is what
`TestFakePosseBytesIsAFunctionOfItsReceiver` now does: four personas, 200
renderings, one expected byte string.

Nothing in `internal/posse/upgradeverify.go` changed. This was a test-only
defect that red `make test` for every seat.
