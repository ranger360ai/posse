# The other 96-99% of the Gatekeeper churn, named

ranger-base-fq3hc, 2026-09-02. Follows ranger-base-nw9zg (`docs/notes.d/
ranger-base-nw9zg.md`, on main since ranger-base-2yaud re-landed it; the
`commit df434d8` this line used to cite is a branch-only sha that never
reached main), whose section 6 is a negative result:
a 120-second census over all of `$HOME` found ONE new executable file while
syspolicyd assessed ~100. This bead was to name the source before anyone
optimised the wrong thing. **It is our own `internal/posse` test suite.** And
the thing most visible to a naive census — `git init`'s 14 `.sample` hook
scripts, the large majority of every new executable file on this box — costs
exactly zero.

Everything below is passive log reading plus small local probes. Gatekeeper
and `spctl` were not touched (operator ruling, nw9zg, still standing).

## 1. Why the census could not have found it

The negative result was not wrong, it was blind, and the instrument says so:

```
$ time find "$(getconf DARWIN_USER_TEMP_DIR)" -type f -perm -100 | wc -l
  291907
find ... 1:55.63 total
```

**A single `find` pass over `$TMPDIR` takes 116 seconds and the tree already
holds 291,907 executable files** (31,413 top-level entries). A "120-second
census" over that root is one pass. `-newermt`/`-newercm` only ever match
files that still exist when `find` reaches them, and the whole population here
lives for milliseconds inside a `t.TempDir()` that is removed at test end.
syspolicyd says the same thing in its own log: `Failed to stat path: <private>
error: No such file or directory`, 12 times in a 2-minute sample — the target
was already gone when syspolicyd went to look at it.

The replacement instrument is a **top-level poller**, not a recursive `find`:
`os.listdir` the one directory (cheap, no `stat`), and the instant a new entry
appears, walk *only that* new subtree. Shipped as
`scripts/gk-inflight-census.py`; the reading below is 1041 polls in 30 s
(~35 Hz), taken with the first version of that poller — read it with the
caveat under the table:

| | |
|---|---|
| baseline top-level entries | 31,413 |
| new top-level entries in 30 s | 316 |
| **new executable files in 30 s** | **605** |
| control planted mid-run / seen | 1 / 1 |

**605 new executable files in 30 seconds** (1210/min) where the `find` census
found one in 120 seconds. The rig plants its own transient exec dir halfway
through and asserts it was caught — a poller that cannot be shown to detect is
not evidence (ORDERS, "a rig must be shown able to fail").

**What that control covered, and what it did not** — ranger-base-nunx9, and it
is the caveat on every count in this note that came from the poller. The
control was planted as an already-populated directory, so it graded the *walk*
and never the *race*. The poller walked each top-level entry **once**, at the
instant it was first listed, and was blind to every executable written into
that entry afterwards. Measured on a four-file fixture — one directory that
appears with its executable in it, one that appears empty and is populated 2 s
later — it reported **2 new exec files against 4 on disk, control green, exit
0**. The control could not fail on the bug.

The script now re-walks the entries it has seen, round-robin, one full sweep
every 2 s, deduping by `(st_dev, st_ino)` so a path `RenderGates` deletes and
rewrites counts as the new file it is (measured: three inodes at one path,
counted three times). It carries a **second** control, planted EMPTY and
populated a second later; with the re-walk disabled that arm reads
`control_late_seen=0` and exit 1, and the same fixture goes back to 2 of 4.

So **605 is a floor, not a count**, and so is every poller-derived number
below. How far below is measurable, and was: on the same real root on
2026-09-03 the fixed poller found **46% (12 s) and 21% (30 s) of all new
executable files by re-walk alone** — that fraction is what the reading above
could not see. §3 has the per-class split, which does not go the way you would
guess.

It is still a floor after the fix: only entries that APPEAR during the run get
re-walked, because the baseline here is 31,413 top-level entries and sweeping
those is the 116-second `find` this instrument exists to replace.

## 2. Who was running

Three `internal/posse` test binaries, concurrently, from three personas'
worktrees, observed live 17:33-17:46:

| pid | worktree | argv |
|---|---|---|
| 442 | gwart-…-8114t | `go test -timeout 20m ./internal/posse/ -run ^Test[A-D]` |
| 11838 | jian-yang-…-lpoui | `posse.test -test.timeout 25m -test.run ^Test[E-L]` |
| 66958 | dinesh-…(fxs60) | `go test -timeout 25m ./internal/posse` |

A 12-second `ps` poll (284 samples, positive control present) counted **464
distinct `posse.test` pids and 324 distinct `git` pids** — 38 and 27 per
second. `internal/posse` is 315 test files carrying 995 `t.TempDir()` calls.

The kernel counts the shells those tests launch, and this is the cheapest live
rate meter on the box, needing no root:

```
log show --predicate 'process == "kernel" AND eventMessage CONTAINS "keys-off mode"'
  → process_is_plugin_host: running binary "bash" in keys-off mode due to identity: com.apple.bash
```

2222 `bash` + 1473 `sh` in two minutes (17:19-17:21) — **~31 shell launches
per second**. Over 76 joined minutes, shell execs/min against assessments/min:

**Pearson r = 0.893**, mean 918 shell execs/min against 157 assessments/min —
about one Gatekeeper assessment per six shell launches.

### 2a. The removal arm, which happened by itself

Correlation would have been the weakest part of this bead. It did not have to
be: two of the three suites finished between 17:48 and 17:49 while the shipped
census was running, and both meters fell off a cliff in the same minute.

| minute | assessments/min | shell execs/min |
|---|---|---|
| 17:46 | 275 | 1601 |
| 17:47 | 326 | 1356 |
| 17:48 | 176 | 1205 |
| **17:49** | **25** | **272** |
| 17:50 | 41 | 371 |
| **17:51** | **6** | **113** |

**A 30-55x collapse in assessments and 12x in shell launches, at the moment
the concurrent `internal/posse` runs ended** — one narrow `-run ^TestQ` suite
was all that remained. The in-flight census, run over `$TMPDIR` for 20 s at
17:49, reported **1** new executable file (its own control) against 605 in
30 s while the suites were up. That is the attribution closed by removal, not
by correlation.

## 3. What the 605 files actually are, and which ones cost

| class | count / 30 s | assessed? |
|---|---|---|
| `git init`'s 14 `.sample` hooks (`commit-msg.sample`, `pre-push.sample`, …) | 535 | **no** |
| gate-shim scripts + fake tool stubs the tests **exec** | 70 | yes |

**Both counts are floors, by unknown and unequal amounts** — ranger-base-nunx9,
see §1. They came from the single-walk poller, which never re-visited a
directory it had listed. Two runs of the fixed poller over the same real root,
2026-09-03:

| run | new exec files | found only by re-walk | `.sample` late | other late |
|---|---|---|---|---|
| 12 s | 156 | 71 (46%) | 71 of 154 | 0 of 2 |
| 30 s | 237 | 49 (21%) | 41 of 145 (28%) | 8 of 92 (9%) |

**A fifth to a half of the population was invisible to the instrument that
produced the table above**, so 535 : 70 — "88% of the files, 0% of the cost" —
is not a ratio to quote. Nor is the direction of the error known: the obvious
guess is that `git init` bursts its 14 hooks into a directory it has just made
(caught) while `RenderGates` writes shims into a `t.TempDir()` that appeared
seconds earlier (missed), which would flatter this table — but the 30 s run
above missed the `.sample` class three times as often as the rest, because
`git init` runs inside a temp tree that already existed. Neither direction is
established; only the truncation is.

What survives does not rest on the poller at all: §4 measures the `.sample`
class at zero excess assessments across two flanked rounds, and §2a establishes
who by removal. Re-run the fixed poller under three concurrent `internal/posse`
suites to get a split that can be quoted.

Captured leaf paths, non-`.sample`, verbatim from the poller:

```
TestGateShellArgvWalk…/001/state/gates/developer/bin/git
TestGateShellArgvWalk…/001/state/gates/developer/shell/zsh
TestGateShellDropsAnotherPersonasGatesBin…/001/state/gates/beta/bin/date
TestGateShellDropsAnotherPersonasGatesBin…/001/state/gates/alpha/bin/ls
TestPruneOnReadIsBlindToTheRuntime…/002/herdr
TestQAGovG2DropsAHolderWaitingOnItsOwnWork…/002/bd
```

Two generators, both in `internal/posse/gates.go`, both writing **mode 0755
shell scripts, unsigned** — exactly nw9zg arm D's class, where a fresh shell
script costs what a fresh Mach-O costs:

- **`RenderGates` (gates.go:750-772)** — `os.RemoveAll(binDir)` and then one
  `os.WriteFile(…, 0o755)` per shimmed command, plus `renderGateShell`. The
  `RemoveAll` is the point: **every call produces brand-new inodes even at
  identical paths**, and nw9zg established the unit of cost is the file, not
  the path. Observed as `state/gates/<persona>/bin/<cmd>` and
  `state/gates/<persona>/shell/zsh` above. This is a production path too —
  every persona launch re-renders — not only a test fixture.
- **`writeTempRender` (gates.go:3531-3547)** — `os.CreateTemp("",
  "posse-l3-render-")`, `Chmod 0o755`, exec, remove. 46 per 30 s.
  Its sibling `os.CreateTemp("", "posse-prepare-commit-msg-probe-")`
  (gates.go:3496) ran 26 per 30 s and is a message file, not exec'd.

The `002/zsh`, `002/herdr`, `002/bd` entries are fake tools written onto a
scratch PATH per test.

## 4. `git init` costs nothing — measured, twice

This is the trap the bead was filed to prevent. `git init` copies 14
executable `.sample` hook scripts into every new repo. They are the large
majority of all new executable files on this box (§3 has the counts, and why
they are floors rather than a ratio) and **git never executes them**, so
Gatekeeper — which assesses on exec — never sees them.

Nothing in this section depends on the poller. It is two flanked rounds of
`git init` against idle controls, which is why it is the arm the §3 caveat
sends you back to.

Two rounds, each arm flanked by an idle control of the same wall length, per
nw9zg discipline:

| round 1 | assessments | | round 2 | assessments |
|---|---|---|---|---|
| idle 30 s | 108 | | idle 30 s | 143 |
| **`git init` ×50 → 700 fresh 0755 files** | **55** | | **`git init` ×100 → 1400 fresh 0755 files** | **42** |
| idle 30 s | 41 | | idle 30 s | 33 |
| 50 fresh scripts, each exec'd once | 128 | | 150 fresh scripts, each exec'd once (40 s) | 199 |
| idle 30 s | 48 | | idle 30 s | 135 |

**1400 brand-new executable files produced 42 assessments — the lowest window
of the round, below both flanking idles.** Zero excess, both rounds. Writing
an executable file is free; executing a file nothing has executed before is
what costs.

The fresh-exec arms are positive but under-recover against nw9zg's calibrated
1.235 assessments/script (round 2: 199 over 40 s against flanking idles of 33
and 135). The trailing idle at 135 is the tell — syspolicyd saturates near
350/min and the queue spills into the next window. Same in the poller window:
~142 freshly-written exec'd files against 56 assessments in 29 s. Both arms
were taken with three test suites already running; the idle controls swung 4x
inside two minutes (33 → 143). The direction is unambiguous, the coefficient
here is not, and nw9zg's quieter reading is the one to quote for per-file cost.

## 5. The PST `path:` value is not invertible — stop trying

Two lines in one sample shared an 8-hex prefix, which looked like a
`(directory, leaf)` pair that could be attacked with a dictionary of every
path on the box. It is not. Over a 10-minute window, 2307 scans:

| | |
|---|---|
| distinct full 64-bit values | 2067 |
| distinct high 32-bit words | 2057 |
| distinct low 32-bit words | 2067 |

Both halves are as distinct as the whole. It is a flat opaque hash. The prefix
match was a coincidence. **The operator `log stream` unredaction profile
remains the only route to literal paths** — and after this bead nobody needs
it, because the top-level poller gets the names directly.

## 6. What this does and does not license

- `/opt`, `/Applications`, `/Library`, `/Volumes` were never censused and are
  now moot. The source is inside `$TMPDIR`, and it is ours.
- **Do not "optimise" `git init`.** Suppressing hook samples
  (`init.templateDir`) would remove most of the new executable files and 0% of
  the assessments. The 0% is §4's, measured directly; the "most" is §3's floor
  and does not need to be exact for this to hold.
- The lever that would actually cut it is the one nw9zg measured: 200 execs of
  200 hard links to one inode = 1 assessment, 200 byte-identical copies = 217.
  A per-test gate-shim tree that is rebuilt from scratch pays per file; one
  built once per test binary and hard-linked pays once. That is a change to
  `RenderGates` and its test fixtures, it trades against test isolation, and
  it is code — handed to dinesh, not decided here.
- The `~30%` of a core that syspolicyd and XprotectService burn is a
  consequence of our own suite running three-up, not of a system daemon
  misbehaving. **It ends when the suite ends** — measured in §2a, not asserted:
  326 assessments/min at 17:47, 6/min at 17:51.
- So "cut the churn" has a cheaper answer than any code change: three personas
  running `./internal/posse` concurrently is what produces the floor. Whether
  that concurrency is worth its cost is a scheduling question for the operator,
  not something to fix in `RenderGates`.

## 7. Instruments, for the next person

```sh
# assessments per minute
log show --last 1h --style compact \
  --predicate 'process == "syspolicyd" AND eventMessage CONTAINS "GK performScan"' \
  | awk 'NR>1{t = $2; print substr(t, 1, 5)}' | uniq -c

# shell launches per minute — the cheap proxy, r = 0.893 against the above
log show --last 1h --style compact \
  --predicate 'process == "kernel" AND eventMessage CONTAINS "keys-off mode"' \
  | awk 'NR>1{t = $2; print substr(t, 1, 5)}' | uniq -c

# new transient executables, with its own control (see §1)
scripts/gk-inflight-census.py '' 30

# who is loud right now. Use ndjson + processImagePath: the compact format's
# columns shift, so a positional awk field silently censuses the wrong thing.
/usr/bin/log show --start '<t0>' --end '<t1>' --style ndjson | python3 -c '
import sys, json, collections
c = collections.Counter()
for line in sys.stdin:
    line = line.strip()
    if line[:1] != "{": continue
    try: d = json.loads(line.rstrip(","))
    except Exception: continue
    c[d.get("processImagePath") or d.get("process")] += 1
for k, v in c.most_common(20): print(v, k)'
```

`log` is a zsh builtin — spell `/usr/bin/log` inside a Bash tool call.
Never use a recursive `find` as a census over `$TMPDIR`; it is a 116-second
pass over 291,907 files and it cannot see anything transient. Poll the top
level, walk what is new **and keep re-walking it**, and plant one control per
thing the instrument claims to do — an already-populated directory grades the
walk and is silent about the race (§1). Both `control_seen` and
`control_late_seen` must read 1; the script exits 1 and says which arm missed
if either does not.
