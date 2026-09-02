# Test-binary churn and Gatekeeper — what was measured, 2026-09-02

Bead: ranger-base-nw9zg. Operator ruling the same day: "cut the churn."

Everything below was taken by execution on the box that runs the fleet, darwin
25.4.0 / go1.26.5. The fleet was live throughout, which is the single biggest
caveat on every number and is quantified in section 6.

## 1. Baseline

`log show --predicate 'process == "syspolicyd" AND eventMessage CONTAINS "GK
performScan"'`, ten minutes, at session start:

| reading | value |
|---|---|
| assessments | 1822 |
| DISTINCT executables assessed | 1644 |
| new `*.test` files under `/var/folders` in the same 10 min | 4 |
| XprotectService CPU (60s delta) | 11.53s = 19.2% of a core |
| syspolicyd CPU (60s delta) | 5.29s = 8.8% of a core |

The bead's premise was "one assessment per NEW executable … that churn is
ours". The first half is right and the second is not: 1644 distinct
executables against 4 new test binaries is a ratio of 411:1.

The bead also carried "XprotectService at 50-80% CPU". Re-measured here it is
19-22% of one core, with syspolicyd another 9-13%. ~0.35 core between them, not
the 0.5-0.7 the bead states. Both readings are real; the load is not steady.

## 2. What Gatekeeper actually keys on

Four arms, 200 execs each, idle controls of the same wall length on either side
to subtract a background that ran 0.8-5.3 assessments/sec while these were
taken:

| arm | what | wall | scans | distinct paths |
|---|---|---|---|---|
| A | 200 execs of ONE path | 1s | 2 | 2 |
| B | 200 execs of 200 COPIES (200 inodes, identical bytes) | 39s | 217 | 214 |
| C | 200 execs of 200 HARD LINKS (1 inode) | 1s | 1 | 1 |
| E | 200 execs of the arm-B copies, a second time | 2s | 7 | 7 |
| D | 200 execs of 200 freshly written SHELL SCRIPTS | 47s | 247 | 242 |

**The verdict is cached against the FILE, not the path and not the content.**
200 paths over one inode cost one assessment (C). 200 inodes with byte-identical
content cost 200 (B) — so it is not a content hash either. And the verdict
persists: arm E is those same 200 files half a minute later for free, and copies
first assessed 25 minutes earlier still exec in 0.023-0.050s against 0.241s for
a fresh one, with a fresh-copy control taken in the same breath to prove the
instrument could still show the slow path.

Arm D matters for attribution: a shell script costs exactly what a Mach-O costs.

### What that kills

**Deliverable (3), the stable GOTMPDIR, buys nothing.** The bead asked whether
Gatekeeper re-assesses an unchanged path, "it may cache by inode/signature —
measure, that decides how much (2) buys". Measured: it caches by file. One
directory for every link still writes a new file per link, and a new file is a
new assessment whatever it is called. Setting GOTMPDIR is also not free:

- `go test` puts `t.TempDir()` under GOTMPDIR while a directly-run test binary
  puts it under `$TMPDIR` (measured three ways; `os.TempDir()` and `$TMPDIR`
  are unchanged in both, only `t.TempDir()` moves). That swap is the trap
  `docs/notes.d/ranger-base-krra.md` records greening a full-disk repro.
- the scratchpad paths on this box are ~150 chars and red launch-line tests on
  length alone.

No GOTMPDIR was set. That is a measured no, not a deferral.

## 3. What `go test` costs, and why

`go test <pkg>` COPIES the linked test binary out of the build cache into a
fresh work dir per invocation. Two cached runs of the identical command over
`./internal/posse`:

```
go-build1660568306/b001/posse.test   inode 243177760  links=1
go-build3490873990/b001/posse.test   inode 243178437  links=1
```

Two invocations, two inodes, one content — a brand-new executable to macOS each
time. The first exec of a fresh copy of that binary, `-test.run` set to a
pattern matching nothing so the figure is startup and nothing else:

```
fresh copy                       1.436s  1.877s  1.848s
fresh copy, page cache warmed    1.980s  1.685s  1.719s   <- not I/O
hard link (new path, one inode)  1.946s  0.204s  0.207s
re-exec of the same copy         0.016s  0.016s  0.016s
```

The page-warmed row is the control that rules out disk: warming the file first
does not make it cheaper, so the cost is the assessment.

## 4. The fix, and why it is not the fix the bead specified

`scripts/gotest.sh`, `make verify-gotest`, pins in `gotestreuse_qa_test.go`.

The bead specified `go test -c -o <cache>/<pkg>-<treehash>.test` with the tree
hash built from `git rev-parse` plus a hash of uncommitted diffs. That was
built and then rejected on a measurement: `go test -c` against a warm build
cache costs 0.59s and 0.69s on the 23 MB package, against the assessment it
avoids. Paying that buys correctness no hand-rolled key can promise — go's own
cache already accounts for every test file, every dependency, the toolchain and
the flags, and a tree key that is wrong ONCE runs a stale suite and reports it
green. Hashing the whole tracked tree is cheap enough (0.24s, 733 files) but
still cannot see a `.gitignore`d `.go` file in a package directory, which go
compiles.

So the key is the built binary's own sha256. Build every time, reuse the FILE:

```
go test -c -o <tmp>          # correctness stays go's
sha=$(shasum -a 256 <tmp>)
target=<cache>/<pkg>-<sha>.test
[ -f "$target" ] && rm <tmp> || mv <tmp> "$target"
exec "$target"               # the inode the box already assessed
```

Proven on `./cmd/posse` — a stale binary is never reused, and the control is
that the added test actually runs:

| | inode | sha | first exec |
|---|---|---|---|
| build 1, fresh | 243734228 | dbdeb069… | 0.389s |
| build 2, no source change | 243734228 | dbdeb069… | 0.153s |
| build 3, one test file ADDED | 243734666 | 77f90e89… | 0.421s |
| build 4, that file removed | 243735601 | dbdeb069… | 0.378s |

`-test.list` finds the added test after build 3 and no longer finds it after
build 4.

Eight self-test arms, each paired with a control that must come out the other
way, and eight mutants, every one killed by the arm it should be:

```
M1 never reuse an existing file    KILLED  reuse: unchanged package keeps one inode
M2 content-blind cache key         KILLED  control: a changed test file rebuilds
M3 return 0 instead of rc          KILLED  arm1 first run
M4 prune deletes nothing           KILLED  prune: keeps POSSE_TESTBIN_KEEP per package
M5 GUT: run() does nothing         KILLED  reuse: unchanged package keeps one inode
M6 self-test always says ok        KILLED  TestQATheWrapperSelfTestCanFail
M7 make target drops --self-test   KILLED  TestQAMakeVerifyGotestRunsTheWrapperSelfTest
M8 make target deleted             KILLED  TestQAMakeVerifyGotestRunsTheWrapperSelfTest
```

M5 and M6 are worth keeping. **M5 first read as a SURVIVOR**: gutting `run()`
left the cache directory uncreated, `find` over it failed the pipeline under
`set -o pipefail`, and the self-test died before any arm printed — so the
harness, which keyed a kill on the string `FAIL`, saw nothing and called it
green. An early abort is a kill. **M6 also first survived**: a `fail()` that
prints `ok` and drops `rc=1` satisfies every label the QA pin requires. That is
what `TestQATheWrapperSelfTestCanFail` exists for — it breaks a copy of the
script on purpose and requires the self-test to notice.

And the harness itself taught the lesson ORDERS already carries: the first
mutation pass ran `git checkout --` between mutants and silently reverted the
uncommitted half of this work. Everything was re-applied and committed before
the pass that produced the table above.

## 5. Before and after, same workload

40 invocations of `go test -run ZZZNONE -count=1 ./internal/posse` against 40
of the same through `scripts/gotest.sh`, idle controls of the same length
between each:

| arm | wall | scans | background from the flanking controls | excess |
|---|---|---|---|---|
| control | 37s | 45 | 1.22/s | — |
| BEFORE 40x `go test` | 44s | 83 | ~1.0/s → ~44 | **~39 ≈ one per invocation** |
| control | 36s | 31 | 0.86/s | — |
| AFTER 40x `gotest.sh` | 46s | 41 | ~0.89/s → ~41 | **~0** |
| control | 36s | 33 | 0.92/s | — |

A second round of the same pair is in the raw log and is **unusable**: the
background went to 4.6/s mid-run (control 167 scans in 36s) and swamped both
arms at 179 and 188. It is recorded here rather than dropped because it is the
honest shape of measuring anything on this box while the fleet is live.

**Wall clock is a wash.** 1.10s per invocation before, 1.15s after in round 1;
1.05s and 0.88s in round 2. Six interleaved pairs taken separately: 0.912s
mean before, 0.907s after. The assessment goes away; the clock does not move,
because the sha256 of a 23 MB binary and the extra `go list` cost about what
the assessment did.

So the honest claim is exactly one sentence: **it removes one Gatekeeper
assessment per `go test` invocation and nothing else.**

## 6. The churn is not ours, and this bead does not find whose

This is the part that matters to "cut the churn", and it is a negative result.

- **A 120-second census over all of `$HOME` found exactly ONE newly created
  executable file** — `scripts/gotest.sh`, which I had just written — while
  syspolicyd assessed ~100 executables in the same window. Earlier 60-second
  censuses over `/var/folders`, `/private/tmp`, `~/.posse`, `~/src`, the go
  build cache and `~/.claude` found **zero**, against 48 assessments.
  Both `-newermt` (mtime) and `-newercm` (ctime, which a hard link updates and
  mtime does not) were used, each with a planted control file to prove the
  predicate could match.
- **The signature field names the class.** Over five minutes: 590 assessments
  with `(id: (null))` — no code-signing identifier at all — against 21 with
  `(id: a.out)`, which is what a Go-linked binary reports (verified against a
  probe binary of my own, which showed `(id: C1)`, its filename). **The Go
  toolchain is ~3% of the rate.**
- This repo has **three** packages with test files. A full `go test ./...` is
  three assessments.

At ~100 fresh go-test executables/hour, the figure the bead opens with, the fix
in section 4 removes ~1.7 assessments/min from a floor that ran 48-280/min
across the day. **0.6% to 3.5%.** Nobody should close a bead claiming this cut
the Gatekeeper load, and the script's own header says so.

What was NOT established: where the other 97% comes from. The two things known
about it are that it is not a file being created anywhere under `$HOME`,
`/private/tmp` or `/var/folders`, and that it carries no code signature — and
arm D above proves a freshly written shell script assesses exactly like a
Mach-O does. Filed as a follow-up.

## 7. Also found, not part of this bead

The box was at **286 MiB free / 100% capacity** when this started, with 78 GB
of `Library/Caches/go-build` — all of it inside go's own 5-day trim window
(`find -atime +5` returned exactly one file), and go1.26.5 has no cache size
cap. Filed as ranger-base-b4yhr (`-l question`, P0) with both command options
and the blast radius, since clearing a shared box is the operator's call.
Something cleared it mid-session: the cache read 366 MB and 81 GiB free about
ten minutes later. The generation RATE — ~15.6 GB/day — is unchanged by that
and is the same churn this bead is about, counted in bytes.
