# The temp HOME that followed every process, and the 5.25G mise put in it (ranger-base-32bdv)

2026-11-11, Linux 7.2.15 x86_64, go1.27.3, 16 cores, from this bead's
worktree. From ranger-base-akk9f (test-times.sh's disk explainer misses
EDQUOT), filed by monica after main's `corpus-check` failed
`TestQAParallelClearanceDoesNotWaiveAReasonNobodyCleared` with `disk quota
exceeded`.

/tmp on this box is a 14G tmpfs mounted `usrquota`, and every seat runs as uid
posse — so it is not a per-seat scratch but ONE box-wide budget, and the seat
that spends it is never the seat that reds.

## The headline count and the actual cost are two different defects

The bead's title reads "423 leaked posse-testhome-* dirs spend the quota".
The count was right and the causation was not. MEASURED at 05:58, 427 dirs:

| shape | n | bytes |
|---|---|---|
| the codex credential fixture alone, 4.0K | 417 | ~1.7M |
| a mise cache | 4 | **5.25G** (2.3G / 1.9G / 915M / 174M) |
| other partial runs | 6 | ~300K |

So 417 of the 427 dirs hold 0.03% of the bytes between them. Reclaiming
*dirs* was never going to free the quota; four of them were the whole of it.
Both still matter — `/tmp` was at 25,708 top-level entries against
`nr_inodes=1048576`, so the count is its own budget — but they have different
causes and needed different fixes.

## Why the bytes: mise resolves its dirs under $HOME, and TestMain moves $HOME

internal/posse's `TestMain` replaces `$HOME` with a tempdir so no test can
reach the operator's live `~/.posse`. Anything that resolves its own storage
under `$HOME` follows it there. GOMODCACHE and GOCACHE were the first readers
caught doing it (ranger-base-6vvgs, which pinned them). mise is the second,
and it is dearer, because on this box `claude`, `codex`, `grok`, `gh` and
`node` are not binaries but mise command-wrappers on PATH:

```
claude -> /home/posse/.local/share/mise/command-wrappers/bin/claude
```

MEASURED, with `$HOME` moved and mise unpinned — today's TestMain:

```
$ env HOME=$tmp mise ls --installed
(nothing)
$ find $tmp -type d
<temphome>/.cache/mise/…   <temphome>/.local/state   …
```

…and with `MISE_{DATA,CACHE,STATE}_DIR` pinned to the real home's:

```
claude  2.1.322  /etc/mise/conf.d/omarchy-tools.toml  latest
codex   0.167.1  …   gh 2.103.3   grok 1.0.77   node 22.23.4
$ find $tmp -type d
<temphome>            ← nothing created
```

So every test that execs one of those by bare name — `skills_e2e_test.go`,
`skillsrules_parity_qa_test.go`, the claude live tests, each gated on a
`LookPath` that the WRAPPER satisfies — made mise re-install the toolchain
into the temp HOME before answering. Up to 2.3G and a network download per
run, with the box's own copy sitting right there.

It is also the more honest fixture. Unpinned, those tests exercise a copy
mise downloaded seconds earlier; pinned, they exercise the version
`/etc/mise/conf.d` pins the operator to. DATA, CACHE and STATE only — never
`MISE_CONFIG_DIR`, which is configuration and so an INPUT to the binary,
exactly what the temp HOME exists to fence out.

## Why the count: the removal is reached only when m.Run RETURNS

`TestMain` ends `code := m.Run(); …; os.RemoveAll(home); os.Exit(code)`. Three
exits never get there, and the probes are one line each:

| exit | leaks | measured |
|---|---|---|
| clean | no | `-run` matching nothing: 0 leftovers |
| `-timeout` panic | **yes** | `-timeout 3s -run '^TestQA'`: 1 leftover, 4.0K, `.codex` only |
| argv the flag parser rejects | **yes** | `posse.test status` → exit 1, 1 leftover; `--bogus-flag` → exit 2; `-version` → exit 2 |

The third is the bulk, and the route to it is a deliberate design decision
one layer up. `reexecenv_qa_test.go` pins that every site re-execing this
binary to run a NAMED test must strip `RHQ_FAKE_HERDR` from the child's
environment — otherwise TestMain answers as the fake substrate and the child
never runs the test. Correct, and it means the child falls through to
`MkdirTemp` and makes a temp HOME **of its own**. Fifteen sites then kill
that child when the test that needed it is done, so its home is a home nobody
removes.

MEASURED: a GREEN three-test arm-1 run —
`TestLaunchLockHoldsAcrossProcesses`, `TestWatchStampsAndClearsPidfile`,
`TestWatchPidRoundTrip` — leaked exactly one 4.0K dir. `ok`, 2.0s, nothing
red. That is the shape of all 417, and the cost lands on whoever runs next.

**A child never needed one.** What it must share with its parent is handed
over explicitly — `RHQ_HOME` for the state tree, `RHQ_QA_HOLD_HOME` for the
lock under test — and nothing it reads wants a FRESH home; the parent's is
already private to this binary. So it adopts (`RHQ_TEST_HOME`), and only the
process that created the dir removes it. Same trio after the change: 0.

`RHQ_TEST_OPERATOR_HOME` goes with it, because `operatorHome` is read as "the
home TestMain replaced" and in an adopting child the inherited `$HOME` is the
PARENT's tempdir. Every reader runs in the parent today
(`ciwatch_live_test.go`, `credentialaccount_live_test.go`,
`worktree_test.go`) and two of them assert `operatorHome != $HOME`, so
without it the first child-run reader measures a tempdir and a guard that
cannot fail proves nothing.

## The own goal the first cut scored, which is this bead in miniature

With the adopt in place and the pins where they were, a clean run still left
one 4.0K dir — and it was the parent's OWN home, by name, re-created after
`os.RemoveAll` had removed it. The adopting child runs `TestMain`'s
`exec.Command("go", "env", GOMODCACHE, GOCACHE)` with `$HOME` ALREADY pointing
at the adopted temp home, and every `go` child writes
`$HOME/.config/go/telemetry`. `GOTELEMETRYDIR` is reported by `go env` but is
not an input that can move it: MEASURED on go1.27.3, 8 dirs appear under a
temp HOME with the variable set and 8 without, and `GOTELEMETRY=off` leaves
the dir too.

The fix is to remove the cause rather than chase the write: an adopter
INHERITS `GOMODCACHE`, `GOCACHE` and the three `MISE_*_DIR` from the process
that set them, so it needs no `go env` subprocess at all. Both pin blocks are
now the creator's work alone (`if ownHome`), and two consecutive runs leak 0.

## What still leaks, and what collects it

The parent's own abnormal exits. A `-timeout` panic exits from a watchdog
goroutine and a signal does not unwind at all, so no deferred cleanup and no
signal handler reaches them — one dir per run, and loud.
`scripts/reap-test-tmp.sh` collects those: dry-run by default, `--apply` to
remove, and a dir goes only when no live process references it (environ
`HOME`/`TMPDIR`/`RHQ_HOME`/`RHQ_QA_HOLD_HOME`, cwd, or any open fd) AND its
mtime is at least `--min-age` minutes old.

**Its control is the capability, not a count.** It reads /proc, which hands
over a process's environ only to that process's own uid, so on a box where
the runs belong to someone else every dir reads "unheld" and the sweep eats
live work while looking clean. It therefore counts the same-uid processes
whose environ it can actually OPEN and refuses outright if that is zero while
other same-uid processes exist — 47 and 45 on this box, so "nothing is held"
means nothing is held.

The first cut of that control asked a different question — "is a test binary
running while I see no temp HOME?" — and it fired on a perfectly sighted box,
twice: the binary running was `treepins.test`, which never makes one, and a
matcher over every process's argv found `.test` in the text of the script
ITSELF. AGENTS.md has the same lesson about pattern kills reading quoted text
(ranger-base-zbg8o), one tool over.

## Verifying it: neither root gives a clean suite on this box today

Both halves of that are somebody else's measurement, and together they mean a
green `make test` was not available here:

- on /tmp, the quota kills the run at the linker (ranger-base-akk9f) — and it
  killed one of mine: `compile: writing output: write $WORK/b132/_pkg_.a:
  disk quota exceeded`, which is this bead's own symptom biting the fix for
  it;
- under $HOME, the workaround reds tests that assume the default root.
  ranger-base-akk9f measured 8 of those in 4 packages.

MEASURED here, arm 1 (`go test ./...`) with TMPDIR and GOTMPDIR under
`/home/posse/.cache`: **12** red in 4 packages, 36 packages ok. akk9f's 8
reproduce exactly; the four it had not surfaced are

    internal/posse    TestQABwrapReportPrintsTheWallItWroteThroughTheSameMarkers
    internal/posse    TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate
    internal/treepins TestQABdPinFlagsDaemonInAThrowawayDirectory
    internal/treepins TestQABdPinSparesTheCanonicalDaemonAndNamesOnlyTheLeaked

and all four PASS on the default /tmp root, so they are the class akk9f named
rather than anything new — a bare /tmp is in every rendered seatbelt, so a
fixture planted outside every granted root reds the pin that reads it. The
first of the 8, `cmd/coldexec`'s
`TestFirstExecOfAPlantedScriptPaysDAndTheSecondDoesNot`, is deterministic
under $HOME (3/3) and green on /tmp — worth saying because it reads exactly
like a red on main, and it was nearly filed as one from this bead.

Arm 2 is GREEN under $HOME (exit 0), which is akk9f's reading too. Arm 3
reds exactly two, `TestQAGrantStillReachesTheGitConfigInEveryShape/deniesFiles`
and `TestSeatbeltWritableReachesGoTelemetryCountersAndNotTheModeSwitch` —
both named in akk9f's fragment, both green on /tmp.

So the verification that stands behind this change is the targeted
before/after in the tables above, `make tree-check` green (67s), and the
three arms read with the known-class reds discounted. Not one of them is in
this diff's blast radius.

## The race this change introduced, and the guard that is the whole of it

Sharing the home is not free, and the cost is not where the leak was. The
codex credential fixture TestMain plants "once, before any test runs"
(herdr_test.go names the path)
was per-process when every process had its own home; with one shared home,
every adopting child re-planted it into a directory the parent and its
siblings were already reading. `os.WriteFile` truncates before it writes, so
two children starting together leave a third reader — `codexSigninProbe` is
one — seeing a 0-byte file where `{}` is expected, and the probe answers
"cannot authenticate" for a reason no test names. Per-process homes hid that
by construction.

The plant is therefore the creator's work too (`if ownHome`), which makes
"once" literal rather than aspirational. That trade needs its own pin, because
guarding the write could as easily have left every child WITHOUT the
credential: `TestTempHomeChild` now reports the bytes it reads and the parent
asserts 2, so an absence and a truncation are distinguishable and neither
reads as success. Shown able to fail — pointed at a path the creator never
plants, it reports `CHILDCRED error … no such file or directory` and reds.
Nothing in the package removes or rewrites that file, so no test depended on
a child re-planting it (grepped, not assumed).

## The reclaim, and the structural choice

Neither is in this commit. The sweep of the 402 collectable dirs
(916M at the 60-minute gate) is refused by auto mode as a shared-scratch
sweep — correctly, and for monica before me — so it needs the operator, with
the exact command on ranger-base-l23nb. The structural question, raising the
quota against moving seats' TMPDIR to /home (871G, no quota), is on the same
bead with the recommendation: move TMPDIR. This bead's own suite ran with
`TMPDIR` under `/home/posse/.cache`, because on /tmp it could not compile —
`compile: writing output: write $WORK/b132/_pkg_.a: disk quota exceeded`,
which is the bead's own symptom biting the fix for it.
