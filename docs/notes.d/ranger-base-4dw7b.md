# The same fixture-root leak at its two remaining sites, in Go and in Python (ranger-base-4dw7b)

*2026-11-12 · from ranger-base-7hyn8, which fixed the third site
(`cmd/coldexec`'s `builtDoor`) and measured these two while it was there ·
beside ranger-base-y9t65 and ranger-base-q0yxp (the reaper's class table) and
ADR 0089 (why its gates are shaped the way they are)*

One fixture root allocated once per PROCESS, outside any test's scope, so no
`t.Cleanup` can remove it and none did:

- `cmd/testparallel/main_test.go` — `os.MkdirTemp("", "testparallel-selftest")`
  inside a `sync.Once`, no `TestMain`, no remover. This is the shape
  `cmd/coldexec` copied, and coldexec's own comment said so.
- `scripts/pattern-kill-census.py` — `tempfile.mkdtemp(prefix=
  "pkcensus-selftest.")` in `--self-test`, no `try`/`finally`, no
  `shutil.rmtree`.

Both are prerequisites of `make test` (`go test ./...` reaches the first,
`verify-pattern-kill-census` the second), so the rate is one dir of each per
`make test` of every seat on the box.

## What it cost

MEASURED 2026-11-12, this box, linux 7.2.15, `/tmp` a 14G tmpfs with
1,048,576 inodes:

| prefix | dirs | bytes | a day earlier (ranger-base-7hyn8) |
|---|---|---|---|
| `testparallel-selftest*` | 115 | 365M | 96 · 305M |
| `pkcensus-selftest.*` | 157 | 5.6M | 132 · 4.7M |

Bytes are not the subject: ranger-base-vw1cp measured the near-outage as an
INODE exhaustion with the byte gauge reading 29% throughout, and 272 dirs is
272 inodes nobody could reclaim.

## Both halves, both sites

**A remover that is not a `t.Cleanup`.** In Go, a `TestMain` that records the
dir in a package var and `os.RemoveAll`s it after `m.Run()` — not a `defer`,
since `os.Exit` runs none — and a failed removal reds the package rather than
returning quietly, because nothing else in the tree ever looks at this dir and
a dropped error is how this leak comes back silently. In Python, the planted
arms moved into `_endtoend_arms(root, check)` so that `self_test` can hold the
`try`/`finally` around them: a pure cut-and-paste, no re-indentation, and the
house shape already in `scripts/python-syntax.py`. A failed `shutil.rmtree`
there is a `BAD` arm rather than an exception, so it cannot mask the body's.

**A name the reaper can see.** `posse-testparallel-selftest-` and
`posse-pkcensus-selftest.`, inside posse's own temp namespace, where
`scripts/reap-test-tmp.sh`'s `posse-scratch` class collects what a remover
never reaches — a SIGKILL, a `-timeout` panic, a box that goes down — at a
120m grace, 2x the longest `-timeout` anything here carries. The legacy names
matched no class at all until ranger-base-q0yxp added one for each to drain
the backlog; those two rows are now dead names, like `coldexec-pins`, and the
table says so. (ranger-base-7hyn8's fragment says neither bead would propose
such rows. The rows landed anyway, for the backlog and not as coverage for a
live creator — which is the distinction the class table's own header draws: a
creator inside the namespace is covered by construction, a row is covered
until the next name.)

**A pin over the dir that was CREATED**, not over the constant, at both sites:
`TestTheToolIsBuiltInsidePossesOwnTempNamespace`, and one more `check` arm in
`--self-test`. Without it a rename out of the namespace leaves every signal
green — the pins pass, the dirs stop being collectable, and nobody finds out
until the inode table runs out.

## Controls

Each half was mutated and re-run, so neither arm is one-sided:

- The prefix moved back outside the namespace, in a COPY of each site (the Go
  one inside the worktree, since the pin needs the module): the Go pin reds
  naming the dir it found, `go test` exits 1; the Python arm prints `BAD` and
  `--self-test` exits 1 with `38 ok, 1 bad`.
- In both mutated runs the remover still ran — 0 `notposse-*` dirs left — so
  the two halves are independent, and the pin is the only thing that would
  have reported the rename.
- After the fix, a run of each leaves 0 dirs of either the new or the old
  name, against 1 of the old name per run before it.

## Not closed here

- ranger-base-m0j9n — the 272 dirs of these two names already on `/tmp`,
  plus coldexec's. No code change reaches them; `reap-test-tmp.sh --apply`
  or the armed timer does, and a bulk `rm` in shared `/tmp` is the operator's
  line (ranger-base-l23nb got that answer twice).
- ranger-base-qbg7c — the residual class: ten other `mkdtemp`/`mktemp -d`
  roots in this tree that sit outside posse's namespace, and the tree-wide
  pin that would refuse the eleventh. All ten have removers and none is
  accumulating (0 dirs of each on `/tmp`, MEASURED 2026-11-12), so that one
  is the killed-run case alone — but three sites have now been found by hand,
  one at a time, over two days, which is what the missing pin costs.
