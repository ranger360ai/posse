# Contributing

Contributions are welcome — bug reports, fixes, and harness improvements
alike. Keep changes self-contained, and read this page first: the suite here
is not shaped like the suite in a module this size, and the difference costs
an afternoon if you meet it by surprise.

## What you need

Go, at the version in `go.mod`, plus `git`, `make` and `python3`. That is the
whole list. **herdr, bd and docker are needed to run posse, not to build or
test it** — the tests that drive a substrate skip when it is absent, and CI
proves it: the full gate runs on stock GitHub runners that install nothing but
Go. Set up a fleet only when you want to *use* the thing (INSTALL.md).

## The loop while you work

```sh
go build ./... && go vet ./... && make tree-check
```

Seconds, not minutes, and `tree-check` is the part that is easy to skip and
should not be. Some of this repo's checks are pins whose subject is the *tree*
— is it gofmt-clean, does the shipped tree name a role rather than a person —
and they live inside a long package that a focused `go test -run` never
compiles. No `-run` filter has ever named them, because formatting is nobody's
subject. `make tree-check` is all of them in one command, and `make fmt` fixes
what the formatting half reports.

## Before you open the pull request

Run the full suite if you can — `make test`, never a bare `go test ./...`, for
the reasons below. **If you cannot, open the PR anyway and let CI run it**: every
pull request runs the same three arms on both ubuntu and macOS, as six jobs,
and that is the same gate a maintainer would apply. A first contribution does
not need a local fleet, a spare half hour, or a machine quiet enough to give
the suite the whole box. Say in the PR what you ran.

Nothing here is a hidden bar. What follows is why the suite is shaped this way.

### `make test`, not a bare `go test ./...`

The target adds `-timeout 25m`, and it is load-bearing: `internal/posse` is a
long serial package that has been measured at and past `go test`'s default 10m
per-package ceiling, so on a busy machine the bare command times out and
reports a panic that names no test — a red belonging to the box, wearing your
diff's clothes.

`make test` is also **three runs, not one**. `internal/posse`'s tests carry a
build tag and are compiled as three separate binaries — `test-arm1` (which is
also `./...` and the tree's own gates), `test-arm2`, `test-arm3` — because half
that package's wall clock was a stream of tests that cannot take `t.Parallel`
and that only more than one binary can divide. A bare `go test ./...` builds
arm 1 and says `ok` over the two thirds it never compiled, so it is not the
suite even when it is green. To run one test that arm 1 does not carry, name
its arm: `go test -tags posse_arm2 -run TestFoo ./internal/posse`. If a `-run`
comes back `ok` having named no test, that is what happened.

Each arm is its own `go test` run under that 25-minute ceiling, so the three
of them are tens of minutes of wall clock on one machine. The split is what
lets CI spend three clocks instead of one, and it is why the honest local
answer is often `tree-check` plus the arm your change touches.

The target also runs the suite through `scripts/test-times.sh`, which prints
each package's seconds and, before any of them start, how much disk the run
has. Take that `DISK:` line seriously: out of space, `go test` reports ENOSPC
as an ordinary test failure once per test that wanted a temp dir, and ~80
unrelated-looking reds naming worktree, watch and dispatch are the box, not
your change (ranger-base-krra).

The target also queues. On a machine running several checkouts of this repo,
one full suite is already sized to the machine and two are deliberate
over-subscription, so `make test` takes one of `POSSE_SUITE_SLOTS` (2)
box-wide slots before it starts and a third full run waits, saying which
checkout it is waiting on. Filtered and single-package runs are never queued.
`POSSE_SUITE_LOCK=0` opts a run out; `scripts/suite-lock.sh --status` says
who holds the slots.

### On macOS, also `make test-linux`

It runs the same `go vet ./... && make test` inside a throwaway Linux
container (docker required, ~35s cold and a couple of seconds after that, and
it mounts the repo read-only so it cannot touch your tree). Some of this code
is filesystem- and shell-sensitive in ways darwin hides — inode reuse and the
path of a real `zsh` have each already cost us a defect that a green macOS
suite reported as fine. CI covers both platforms too, so this is the fast way
to learn it before the push rather than after.

## Where the rest is written down

[README.md](README.md) is the product, [INSTALL.md](INSTALL.md) is standing an
instance up, and [NOTES.md](NOTES.md) is the short map of the implementation —
the authoritative stores and who writes them, the launch→settle→landing order,
the build and test entry points, and what is still decided by hand. Decisions
are in [docs/adr/](docs/adr/), which has [its own reading
guide](docs/adr/README.md); procedures are in [docs/runbooks/](docs/runbooks/).

## Upstreaming from a private instance

A contribution is harness-worthy iff any deployer could have written it: mechanism or method that survives with your facts removed. Before opening a PR, de-instance it — measured numbers become config defaults with the rationale restated, never the measurement quoted; incident stories become the invariant the incident taught; persona and crew names become roles; operator names, hostnames, and machine paths go; private-tracker ids become a design-doc section reference, or drop. Cost, plan, spend, quota, and utilization figures never cross, in any form. Credential facts never cross — not values, and not the map: no storage names or locations, no auth topology, no account or plan identity. When a change needs your instance's numbers to justify itself, keep the numbers in your private tracker and let the PR carry the qualitative rationale. When in doubt, leave it out: a fact kept private can be published later; the reverse is a history purge.
