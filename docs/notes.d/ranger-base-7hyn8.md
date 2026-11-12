# A fixture root the reaper could never see, and the two halves of removing it (ranger-base-7hyn8)

*2026-11-11 · from ranger-base-91fu2 (the door's own design bead), noticed
while surveying the exhausted `/tmp` tmpfs · beside ranger-base-y9t65 (the
reaper's class table) and ADR 0089 (why its gates are shaped the way they
are)*

`cmd/coldexec`'s pins build the door once per test binary, because the build
is shared by every test in it. That is also why nothing removed it: the dir
outlives each individual test, so there is no `t.Cleanup` that could, and
there was no `TestMain` either. Every run of the package left one behind.

## What it cost, and what it was not

MEASURED 2026-11-11, linux 7.2.15, `/tmp` a 14G tmpfs with 1,048,576 inodes:

| prefix | dirs | bytes | creator |
|---|---|---|---|
| `coldexec-pins*` | 278 | 914M | `cmd/coldexec`, fixed here |
| `testparallel-selftest*` | 96 | 305M | `cmd/testparallel`, ranger-base-4dw7b |
| `pkcensus-selftest.*` | 132 | 4.7M | `scripts/pattern-kill-census.py`, ditto |

248 of the 278 were from that one day. The bead's own reading, taken a few
hours earlier, was 118 dirs and 384M — the rate is per run of the package,
and the package is in `go test ./...`, so it is per `make test` of every seat
on the box.

Bytes are not what this is about. ranger-base-vw1cp measured the near-outage
as an INODE exhaustion with the byte gauge reading 29% throughout, and 506
dirs is 506 inodes nobody can reclaim.

## The half a remover cannot cover

`TestMain` now records the dir and `os.RemoveAll`s it after `m.Run()` — not a
`defer`, since `os.Exit` runs none — and a failed removal reds the package
rather than returning quietly, because nothing else in the tree ever looks at
this dir and a dropped error is how this leak comes back silently.

That covers every exit the binary chooses. It covers none of the exits it does
not: a SIGKILL, a `-timeout` panic, a box that goes down. ADR 0089's own
rejected-alternatives list says so in as many words ("A signal handler or
`t.Cleanup` in the parent. SIGKILL cannot be caught, and a `-timeout` panic
exits from the watchdog goroutine"), and on this box suites DO get killed.

What collects that run's dir is `scripts/reap-test-tmp.sh` — and it could
not, because of the NAME. Its class table is `posse-testhome-*`, `posse-*`,
`go-build[0-9]*`, `Test*[0-9]`, `tmp.??????????` and `claude-[0-9]*/*/*`, and
`--class` takes a name from that table rather than a glob. A root called
`coldexec-pins*` matches nothing in it, so it was not merely uncollected: it
was uncollectABLE, at any age, by any spelling of that script. ranger-base-l23nb's
reclaim took 428 `posse-testhome-*` dirs the same morning and walked past
these.

So the prefix moved into posse's own temp namespace, `posse-coldexec-pins-`,
where the `posse-scratch` class collects it at a 120m grace — 2x the longest
`-timeout` anything here carries. **The rule, for the next fixture root:
name it `posse-*` or the reaper can never see it.** The class table's own
header already argues for this over a table edit — a creator that announces
itself inside the namespace is covered by construction, an edit is covered
until the next name — which is also why neither this bead nor
ranger-base-4dw7b proposes rows for the legacy names.

A pin asserts the prefix over the dir that was actually created rather than
over the constant, because a rename out of the namespace leaves both halves
looking green: the pins pass, the dirs stop being collectable, and nobody
finds out until the inode table runs out.

## Controls

Two value mutations the code still compiles around, each run against a
`TMPDIR` of its own:

- `doorDir = ""`, so the remover is handed nothing: the package stays green
  (`ok 1.828s`) and leaves exactly one dir per run. That is the defect, and
  the suite never sees it — this one is measured, not asserted.
- the prefix back outside the namespace: the name pin reds, naming the dir it
  found.

After the fix, the same reading is 0 dirs over a green run (`ok 1.819s`).

## Not closed here

- ranger-base-4dw7b — the same defect at two more sites, same two-line fix.
- ranger-base-m0j9n — the 506 dirs already on disk. No code change reaches
  them, and a bulk `rm` in shared `/tmp` is the operator's line, which is the
  answer ranger-base-l23nb got twice.
