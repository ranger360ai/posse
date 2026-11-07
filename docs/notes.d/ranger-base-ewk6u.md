# The plan-usage snapshot is keyed by the login, not the home (ranger-base-ewk6u)

2026-11-07. Builds ADR 0080 D11. Design bead ranger-base-oz8nc.

## What moved

`$RHQ_HOME/state/plan-usage.{json,log}` →
`~/.cache/posse/plan-usage/<key>.{json,log}`, where `<key>` is the first 16
hex digits of the sha256 of the REALPATH of the resolved Claude config dir
(`$CLAUDE_CONFIG_DIR`, else `<home>/.claude` — `ClaudeConfigDirIn`, the one
spelling of that rule). `$XDG_CACHE_HOME` wins over `$HOME` exactly as it
does for the suite slots (`SuiteLockDir`) and the bd-write locks
(`bdWriteLockDir`), which are that directory's other tenants.

No migration. The first read misses, one fetch fills the file, and the old
per-home file is left where it is and never read again.

## Why, in one line

The usage endpoint's rate limit is per CREDENTIAL and ADR 0080 D6 puts every
crew home on one Claude login, so a snapshot under `$StateDir` makes N homes
N pollers on one limit and leaves a 429 cooldown home A earned unknown to
home B. That is rangerhq-tdy8 (2026-08-22, three and a quarter hours of
continuous 429 with every poller still asking) with its fix undone.

## Three things that are not in the ADR

**1. The key's third state.** With no `$HOME` and no `$CLAUDE_CONFIG_DIR`
there is nothing to key by, and `filepath.Join` DROPS an empty element — an
unguarded resolution would hand back a relative `.cache/posse/plan-usage`
keyed by a relative `.claude`, which is a different store per cwd. Both
halves answer `""` instead, and `PlanCache.Path == ""` already means "no
sharing, every read is a request". The guard is `runtimeHomeOverride`'s bit,
which is trust.go's and sentline.go's own spelling of the same test
(ranger-base-a3t1's rule).

**2. The store is now shared state a TEST can poison.** Under `$StateDir`
the file was per posse home and so per test by construction. Keyed by the
login it is one file per BOX — and internal/posse's `TestMain` gives the
whole binary one `$HOME` with `CLAUDE_CONFIG_DIR` cleared, so all ~2000
tests would have resolved one key and one file: a 429 cooldown one test's
fake endpoint writes would be honoured by the next test's read, and a test
asserting the snapshot is ABSENT would be red or green depending on who ran
before it. The seam is `App.PlanUsageDir` (empty = the shipped resolution),
set per test by `hermetic()` — the same shape and the same reason as
`WorktreeRootDefault`, whose default also names one process-wide directory.
Nothing product-facing reads it and no config key names it.

**3. cmd/posse needed a cache fence beside its config-dir fence.** That
package's `TestMain` already pins `CLAUDE_CONFIG_DIR` to a temp dir so
children built from `os.Environ()` cannot reach the operator's `~/.claude`
(ranger-base-jxuiy). Several of its children keep the operator's `$HOME`
deliberately (`readyEnv`), so after this change a `posse status` child would
append its read log to a file in the operator's own `~/.cache/posse`. The
key is the fence dir's, so it could never be the operator's LIVE snapshot;
what it would be is litter in a directory the suite has no business writing
to. One `XDG_CACHE_HOME` row in the same `TestMain` moves the plan store,
the suite slots and the bd-write locks for every child at once. Tests whose
siblings share the login (`planstale_qa_test.go`) type a per-test row over
it, because the fence keeps the suite out of the operator's cache and only a
per-test dir keeps each test out of its siblings'.

## What is pinned

`internal/posse/planlogin_qa_test.go`, five tests, both directions:

- two homes + one config dir → one store, and home B refuses to ask its own
  healthy endpoint through home A's cooldown (0 requests), then reads B's
  later reading back out of the same file;
- two config dirs → two stores in one cache dir, and the second login asks
  for itself and holds none of the first's cooldown;
- one config dir spelled through a symlink → one store (the realpath claim);
- no home / no cache root → `""`, at both halves, as a table;
- an `RHQ_PLAN_USAGE_URL` override writes no login store on a success OR on
  a 429, with the read log as the positive witness that the request happened
  (credpin.go rule 5 re-pinned here, because what changed is that the file
  it gates is now the box's rather than one test home's).

MEASURED 2026-11-07, darwin/arm64: the five pass in 0.95s; with
`App.PlanCache` restored to `filepath.Join(a.StateDir, "plan-usage.json")`
the first two fail and the override arm goes vacuous.
