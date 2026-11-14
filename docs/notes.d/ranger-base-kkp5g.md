# A cleanup handler trapped on INT/TERM that RETURNS lets a signalled run print FAILs it never measured (ranger-base-kkp5g)

discovered-from: ranger-base-myjcz, whose fix was one line above the defect.

## The defect

`trap cleanup EXIT INT TERM`, with a `cleanup` that returns. On EXIT that is
right and it is the whole of what the handler is for. On INT or TERM it is
not: bash runs the handler and then **resumes** the script at the statement
after the one the signal interrupted, with whatever the handler just gave back
already gone.

MEASURED 2026-11-13, linux 7.2.15 x86_64, bash 5.3.20, this box. The shape
under test, signalled 1s into a 5s first arm:

| spelling | signal | what happened | exit |
|---|---|---|---|
| `trap cleanup EXIT INT TERM` | TERM | cleanup ran, `$T` gone, **arm 2 reached**, printed its FAIL | 1 |
| `trap cleanup EXIT INT TERM` | INT | identical | 1 |
| `trap cleanup EXIT` + `trap 'exit 130' INT` + `trap 'exit 143' TERM` | TERM | cleanup ran once, arm 2 never reached | 143 |
| the same | INT | the same | 130 |

Two readings worth keeping from building that rig:

- **A background child of a non-interactive shell cannot be INT-tested.** POSIX
  has such a shell set INT and QUIT to `SIG_IGN` in background children, and
  bash will not let a script trap a signal that was ignored on entry — so the
  first INT arm measured *nothing at all* (no handler, apparatus still
  present) and looked like a pass. `set -m` in the runner is what makes the
  INT arm real. Any later rig for this class needs it.
- **`trap 'cleanup; exit 130' INT TERM`** (reap-selftest-ambient.sh:129, the
  spelling already in the tree) runs the handler **twice** — once on the
  signal and once on the EXIT its own `exit` triggers. Harmless for an
  idempotent handler, and all six under `scripts/` are idempotent, but
  `queue-cutover.sh:405-408`'s split is better: the EXIT trap does the work,
  each signal's trap only `exit`s with that signal's 128+signo, the handler
  runs once and the status names the signal.

And on the script the bead was filed from. A `kill -TERM` of
`scripts/verify-herdr-bob-rules.sh` during its seeding arms ran `cleanup`,
which `rm -rf "$T"`-ed the scratch dir, and the run carried straight on into
the second arm, re-made `$T` by mkdir, and printed

```
FAIL fresh seeding: watcher read 'nothing', seeded file does NOT match the plugin's
FAIL override not read: watcher read 'nothing', and the file was OVERWRITTEN
verify-herdr-bob-rules: 2 check(s) failed
```

with exit 1. **Neither property was false.** Both were unmeasured. That is the
exact failure that script's `apparatus()` and `harness()` exist to prevent
(ranger-base-vf3pf), one level up — and the trap was the one arm that could
not say "that was the apparatus".

## The census, and a verdict per script

`grep -rn 'trap .* INT TERM' scripts/` found six handlers that return against
one that exits. Not all six cost the same, which is why this was a roster with
prose and not a `sed`. Five were split; one must **not** be.

| script | verdict |
|---|---|
| `clean-build.sh:83` | split. A `kill -TERM` of this pid does not reach the `go build` child, so the build completes, cleanup removes `$src` under it, and the run reaches `echo "clean-build: built $out"` and exits **0**. An interrupted `make install` would then install. |
| `release-artifacts.sh:229` | split. Same shape, more at stake — it runs on a tag in `.github/workflows/release.yml`. Most placements die on a `cd` into a removed dir under `set -e`; a signal in the tail does not, and a cancelled release reports a complete artifact set. |
| `verify-gate-freshness.sh:139` | split, through the script's own `nothing` (exit 2). `$ref` is the HEAD copy every probe compares the installed gate against; resuming reports `stale`/`broken` findings from a removed reference, to a reader that turns them into a box-wide freshness verdict (ADR 0092). |
| `verify-herdr-bob.sh:141` | split. Same class as the one below. |
| `verify-herdr-bob-rules.sh:164` | split. The script the defect was measured on. |
| `bd-argv-gate.sh:93` | **keep resuming**, rostered with its reason. See below. |

### Why bd-argv-gate.sh is the exemption

It is a PreToolUse hook, so **its exit status is its verdict**, and every
spelling that ends the shell on a signal is worse than resuming. Claude's
contract, MEASURED against the real binary under ranger-base-3bqn (property
2): exit 2 blocks the tool call, *any other* nonzero shows stderr and **runs
it** — a hook that exited 1 let the refused command through. So:

- `exit 130`/`exit 143` is **fail-open**, on the one script in this census
  where failing open is the whole stake.
- An unconditional `exit 2` would refuse every signalled Bash call on the box
  whether or not it names bd — the fleet-wide false refusal ranger-base-3bqn
  forbids.
- Dropping INT/TERM/HUP from the trap is worse again: bash's default
  disposition kills the gate before it can answer, which is **fail open**.
  That is the whole of the reason and it is enough. It does **not** leak
  `$errd`, which this bullet used to claim: MEASURED 2026-11-13 (linux
  7.2.15, bash 5.3.20 — ranger-base-5owgm, and
  [ranger-base-kbaah](ranger-base-kbaah.md), "bash runs the EXIT trap on an
  UNTRAPPED terminating signal"), an untrapped terminating signal still
  **runs** bash's EXIT trap, exits 128+signo and does not resume, so
  `rm -rf -- "${errd:?}"` happens anyway.

Resuming is what lets the gate still reach its own fail-closed decision, and
MEASURED 2026-11-13 it does: **bash restores `$?` across a signal handler**,
so `rc=$?` at :102 still reads the parser's status and not the handler's
(verified both ways — a child exiting 2 while the handler, which exits 0, runs
between it and the `rc=$?`). What resumption costs is one line of prose and no
verdict: the `grep -q '^bd-argv-gate:' "$err"` reads a file inside the removed
`$errd`, so a parser that refused *with a reason* is reported by the generic
"parser unavailable" line instead of its own. Still exit 2, still fail closed.

## What was run

Real scripts, real signals, with controls. `set -m` throughout, for the reason
above.

**clean-build.sh** — TERM at 3s: exit **143**, `clean-build: interrupted
(SIGTERM) — this run did not finish`, no `built` line. INT: exit **130**.
Control: exit 0, `clean-build: built … from 34bbe053`. The 18MB binary *was*
written in both signalled runs — the build had finished before the deferred
trap ran — which is exactly the pre-fix hazard: the old spelling would have
printed `built` and exited 0 over it.

**release-artifacts.sh** — `--version v0.5.3 --out <tmp>`, warm cache (the
control is 6s warm, 33s cold; a 10s delay missed the window entirely on the
first attempt and measured nothing). TERM and INT at 2s and 4s, all four
signals delivered: exit **143**/**130**, `release-artifacts: interrupted
(SIGxxx) — <out> is incomplete`, 3 or 6 files in `--out` and **no
checksums.txt**. Control: exit 0, 9 files, checksums.txt present. No leaked
`posse-release.*` scratch and no stray worktree in any run. Nothing in that
script touches the network, tags or pushes — it writes `$OUT` and a mktemp dir
— so the blast radius of running it was one temp dir and four cross-compiles
of CPU.

**verify-gate-freshness.sh** — it does *not* skip on this box (exit 0, "fresh"
unsignalled). 0.14s is too fast to land a signal by sleep, so the run was held
in `probe()`'s command substitution with `--wrapper <a script that sleeps
10s>`. TERM, INT and HUP at 1s: exit **2** and `verify-gate-freshness:
interrupted (SIGxxx) — nothing measured` on all three, no leaked scratch dir.
The same run unsignalled exits **1** with findings. That the two are now
*distinguishable* is the point — exit 2 is what ADR 0092 renders as `unknown`.
Partial findings do reach stdout before the signal; the exit status is what the
consumer reads, which is already that ADR's contract.

**NOT MEASURED HERE: verify-herdr-bob.sh and verify-herdr-bob-rules.sh.** Both
still SKIP on this box — the plugin is not installed at the pinned path, and
`PINNED_SHA` means the rig cannot be faked up. Their controls were run (exit 0,
`SKIP: herdr-bob is not installed at …`), which says the new trap lines break
nothing on the path this box can reach, and nothing more. Filed for the
operator; it can ride along with ranger-base-5b3lq, which already asks for
`make verify-herdr-bob-rules` on the mac for ranger-base-myjcz's hoist.

`go run ./cmd/checkorphans` clean after every batch.

## The pin, and the hole in the reader it found

`internal/treepins/trapsignalexit_qa_test.go`, three arms plus two controls,
behind `make scripts-check`. It shares ranger-base-wzo36's reader rather than
walking `scripts/` a second time — a second walk is a second answer to "what
does this script trap", and the two could come to disagree. `tsRead` now
records *every* registration with its signal list (`tsScript.Traps`); the EXIT
reading is unchanged.

Census after the five splits: 48 shell files of the 64 under `scripts/`, **49**
trap registrations, **18** on a catchable deadly signal, **17** ending the
shell, **1** exempted — which is the whole of the rule's negative side.

The rule accepts two spellings, both already in the tree: an `exit` in command
position, or a reset-and-re-raise (`trap - SIG; kill -SIG $$`,
`test-times.sh:1674`, which is not interchangeable — it exists so the wrapper's
*wait status* is a real signal death). It follows a call **one level**, so
`verify-gate-freshness.sh`'s `trap 'nothing "…"' INT` passes: a rule blind to
that would push the next author off the script's own channel and onto a bare
`exit 143` the consumer has no reading for. `trap '' SIG` and `trap - SIG` run
no code and are dropped before the rule sees them. ERR/DEBUG/RETURN are out of
scope by the rule and not by convenience — running the handler and carrying on
*is* their contract, and `verify-id-recycle.sh:28` and its two siblings are
exactly that shape.

### 908 lines nothing was reading

Writing the census turned up that `verify-gate-freshness.sh:139` was invisible
to the shared reader — and so, therefore, was that file's EXIT trap, to
ranger-base-wzo36's own pin. The cause, in `tsReadLines`: a heredoc opened on a
line that **also opens a bracket frame** makes the line joiner swallow the
heredoc body *and its terminator* looking for the close —

```
done < <("$PYTHON" - "$config_dir" <<'PY'    :107, depth 1
…23 lines of python…
PY                                           :131, eaten by the join
)                                            :132, depth 0
```

— and the opener is still in the joined text, so the heredoc skip armed for a
`PY` that had already gone by. The reader then skipped every remaining line of
the file looking for a second one, to EOF, **with `lost` empty**, because no
join had run away.

MEASURED 2026-11-13, four files and 908 physical lines dropped in silence:

| file | read to | code to | unread |
|---|---|---|---|
| `verify-box.sh` | 271 | 898 | **627** |
| `verify-bd-argv-gate.sh`\* | 50 | 182 | 132 |
| `verify-gate-freshness.sh` | 132 | 262 | 130 |
| `reap-timer-unit.sh`\* | 244 | 263 | 19 |

\* these two are correct and not defects: the rest of those files *is* the
heredoc body, terminated on the last line. The two real ones are
`verify-box.sh` — whose `trap selftest_cleanup EXIT` at :554 and whose
`self_test` trap-state setter were both past the blind point — and
`verify-gate-freshness.sh`.

**Nothing was red over any of it.** Every floor in wzo36's census stayed
comfortably clear while the reader could not see four files, which is the
argument for the repair's second half. Both halves are in `tsReadLines`:

1. Arm the skip only when the opener sits on the **last** physical line the
   join consumed. Exact, not a heuristic: bash reads a heredoc body starting at
   the line after the opener's, so if that line is not the last one consumed,
   those lines are already in the joined text and nothing is left to skip.
2. Commit a heredoc's body lines to `Here` only when its **terminator** is
   found, and set `lost` when one is still open at EOF. A phantom heredoc
   swallows real lines and the reader cannot tell its own phantom from a real
   body, so the lines it skipped are not evidence that they were data.

`TestQATheShellReaderReadsEveryLineUnderScripts` is what holds it: every line
of code under `scripts/` is inside some logical line's extent or inside a
*terminated* heredoc body, and there is no third place for one to be. It is a
separate assertion rather than a floor **because no floor could have caught
this** — the first cut of it trusted the reader's own `Here` claim and passed
over the re-planted defect, which is how the second half of the repair got
found.

Five figures in wzo36's head comment moved the same day and not because the
tree did: 25→27 EXIT traps, 46→48 handler-read names, 426→436 functions,
16→17 setters. Updated there, with why.

### Mutation

Because a census that stopped reading also produces a clean answer.

| mutation | arms that red |
|---|---|
| revert the heredoc-arm fix (re-blind the reader) | the coverage arm names both real files, their openers and their skipped ranges; **and** wzo36's own census reds on `Lost` for both |
| put `trap cleanup EXIT INT TERM` back in `verify-herdr-bob-rules.sh` | the signal census, naming the line and the split to type |
| give `bd-argv-gate.sh`'s handler an `exit` | the roster: "an exemption over nothing", on both the census arm and the two-way arm |
| rename the exempted script in the roster | the roster: stale row |

All four red, the tree green as committed, every file's md5 identical before
and after. The first attempt at the first mutation did not apply (a `perl -0pi`
substitution that silently matched nothing) and its "pass" was void — the run
was redone with the condition neutralised to `true` and verified by reading the
mutated line back. The sixteen planted fixtures of
`TestQATheSignalTrapReaderCanStillSayNo` carry the bead's own shape, both
accepted spellings, both sides of the one-level follow, the `SIGTERM` and
numeric spellings, and `trap ''`/`trap -`/ERR.
