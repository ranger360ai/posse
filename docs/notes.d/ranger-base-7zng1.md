# The heaviest named-package run on the box took no suite slot (ranger-base-7zng1)

2026-10-04. Found while verifying ranger-base-5no4s; filed by the verify lane
with both halves measured, and ruled here.

## What happened

A clean `make test` on an unrelated change went red in arm 3:

```
--- FAIL: TestWatchPulseArmedLogsBlockedSession (135.11s)
    pulse_test.go:303: watch never returned after cancel
    pulse_test.go:374: watch never returned   (TestWatchPulseUnarmedNoTicker)
FAIL  github.com/ranger360ai/posse/internal/posse  1459.560s
```

Both failures are one shape: a 30s deadline on `Watch` returning after its
context is cancelled, driven at 20ms/40ms intervals. Alone under their tag the
same two tests pass in 0.93s and 0.55s — 3% of the deadline they blew through.
Arm 3 took 1459.6s under the load and 424.1s without it, 3.4x. Nothing in the
change was near pulse or watch.

The load was one concurrent `go test ./internal/treepins`.

## Why that package is a full suite wearing a named package's clothes

`internal/treepins` holds the pins whose subject is the repository TREE, so
they fork `go build`, `go vet`, `git` and the gate scripts over the whole
repo; `TestQAEverySuiteArmTypeChecks` alone type-checks and vets all three of
`internal/posse`'s suite arms, compiling that package three times over.

`scripts/suite-lock.sh` serializes a full package TREE — `./...`, `all`,
anything ending `/...` — and a tagged arm, and deliberately exempts a `-run`
filter and a named package, because those "are the runs a person does while
thinking, they are seconds long". This one is not seconds long, and that
exemption made it invisible to the runs that queue while making nobody else
queue — the class ranger-base-uvzjk built the lock for.

## Measured

| reading | value | environment |
| --- | --- | --- |
| the package, warm build cache | **363.159s**, 639 pins green | 2026-10-04, this box, go1.26.5, `-count=1`, one sibling treepins run alongside, loadavg ~20-30 |
| the package, cold arm builds | **679.089s** | 2026-10-03, same box and toolchain, `-count=1`, uncontended (ranger-base-7zng1's filing) |
| the arm-tags door alone (`-run '^TestQAEverySuiteArmTypeChecks$'`) | **4.22s** real (1.16 + 1.11 + 2.10s of vet) | 2026-10-04, warm build cache |
| wall readings for this package at or above 60s, across every session transcript on the box | **n=327**, median **393.5s**, mean 431.5s, max **1501.6s**; 274 at or above 300s, 38 at or above 600s, 11 at or above 900s | 2026-10-04, corpus of 7 days |
| unfiltered `go test ./internal/treepins` commands typed | **140**, from **52 sessions**, over 7 days (41 on 09-11, 26 on 09-28, 47 on 10-03) | same corpus |
| minutes in which two DIFFERENT seats each started one | **5** (the last of them 2026-10-04T02:40, while this bead was being worked) | same corpus |

300s is `scripts/test-times.sh`'s own `SLOW_PACKAGE_SECONDS` line, so the
median unfiltered run of this package is a package the suite wrapper is
already built to complain about — and it was costing that outside the queue.

Re-run the wall distribution (the corpus grows):

```sh
# the two backslashes are deliberate: the stamp is a JSON \t in the file,
# so what grep must match is a backslash followed by a t.
grep -ho 'posse/internal/treepins\\t[0-9.]*s' ~/.claude/projects/*/*.jsonl |
	sed 's/.*\\t//;s/s$//' | sort -n
```

Readings below 60s are the filtered runs (median 1.65s over 585 of them);
nothing filtered came within an order of magnitude of 60s, which is why the
threshold separates the two populations rather than cutting one.

The command census reads the same corpus for Bash `tool_use` blocks, splits
each command on the separators a shell would split on, keeps the segments
whose head word is a command and that name this package, and sorts them by
whether they carry `-run`/`-test.run`. That is `scripts/suite-entry-census.py`'s
method narrowed to one package; the three traps it documents (segments not
lines, a `-run` filter is not a suite, the trend not the total) apply here
unchanged.

## Ruled

**The slot.** `suite_lock_wanted` now names this one package, so an
unfiltered run of it takes a slot. The exemption is not withdrawn: a `-run`
filter returns from the first loop and never reaches the rule, so `make
tree-check`, the arm-tags door inside `make test`'s own recipe and every
focused pin run are exactly as unqueued as they were — which is also why
there is no nested-acquire case to handle, nothing that runs inside a held
slot reaches the rule.

It is a NAME and not a cost heuristic. argv cannot be asked what a run will
cost: the same command measured 363.2s warm and 679.1s cold, and no pattern
can read the build cache. A measured name is a claim a reader can check.

The sibling candidate — a bare `go test ./internal/posse`, which is arm 1 of
a ~950s package and unqueued for the same reason — is deliberately left
alone. The asymmetry paragraph in `suite_lock_wanted` argues it (ADR
ranger-base-qp1hm's decision about the run a person does while thinking), and
reversing it is an architecture call, not a line added beside this one.

Arm 5c of `scripts/suite-lock.sh --self-test` is the evidence, and arm 5 —
"a single-package run takes no slot", over `./internal/posse` — is its
control. Either one alone is green over a rule that is a withdrawn exemption
or no rule at all. `internal/treepins/suitelock_qa_test.go` requires both arms
by name.

**The deadlines.** The five 30s bounds in `internal/posse/pulse_test.go` are
one named constant, `pulseHangGuard`, at 90s. They are HANG GUARDS, not
budgets: the distinction and the number are
`hookwallsweep_qa_test.go`'s (ranger-base-isq3, which left an identical
`Watch`-never-returned guard at 90s deliberately), and the reasoning is that a
ceiling ~100x the real cost is no longer a number a loaded box can reach while
still turning a wedged `Watch` into a tap dump naming the line instead of a
package-wide `go test -timeout` panic. Two of those five were the red above;
the other three are the same shape in the same file, and leaving them at 30s
would have been filing the next flake rather than fixing this one.

The pair are not a budget in the other direction either: the tests end on
their own evidence (a tap line, a pass count, a released park), never on the
timer, so a slower box spends no extra wall clock for the wider ceiling.

## Reach: the rule queued nothing, and what it took to fix (ranger-base-1a0hi)

2026-10-04, found by the verify lane under ranger-base-l3035 and ruled here.
The rule above is right and its pin is honest; the gap was that nothing in the
tree could ASK it, while AGENTS.md and NOTES.md both said a bare
`go test ./internal/treepins` took a slot.

`suite_lock_wanted` is a shell function inside `scripts/suite-lock.sh`, which
is sourced by exactly two callers — `scripts/gotest.sh` and
`scripts/test-times.sh` — and reached from `make`. There is no `go` shim:
`which -a go` answers `/opt/homebrew/bin/go` and nothing else, and the gates
bin holds five shims (bd, git, killall, pkill, posse, security) and no `go`.
And no target routed this package through a caller: at the parent HEAD the
only treepins lines in the Makefile were the arm-tags door in `test`,
`test-arm2` and `test-arm3`, all three filtered, and a filtered run returns
from `suite_lock_wanted`'s first loop before the rule.

### The callers, measured

| reading | value | environment |
| --- | --- | --- |
| unfiltered `go test ./internal/treepins` segments, bare `go` | **122**, from **48 sessions** (09-10=1, 09-11=28, 09-20=6, 09-28=23, 09-29=7, 10-03=43, 10-04=14) | 2026-10-04, 1,125 transcripts under `~/.claude/projects` |
| `go vet ./internal/treepins`, unfiltered, bare `go` | **65** | same corpus |
| the same, through `make`, `scripts/gotest.sh` or `scripts/test-times.sh` | **0** | same corpus |

So the exposure the rule was written for was 100% the one spelling the rule
cannot see. (The filing read this as 203 of 206 with a wider verb set and a
different prose filter; the headline — no run reaches a wrapper — is the same
on both readings.)

The by-door census is `scripts/suite-entry-census.py`'s reader with one
predicate changed, because a NAMED package is not a `./...` tree and
`is_full_suite()` is blind to exactly the run this counts. Re-run it; the
corpus grows:

```python
import collections, glob, importlib.util, json, os, re
spec = importlib.util.spec_from_file_location("sec", "scripts/suite-entry-census.py")
sec = importlib.util.module_from_spec(spec); spec.loader.exec_module(sec)
PKG = re.compile(r"(?:^|\s)[\w./@-]*internal/treepins/?(?:\s|$)")
rows = []
for path in glob.glob(os.path.expanduser("~/.claude/projects/*/*.jsonl")):
    for line in open(path, encoding="utf-8", errors="replace"):
        if "internal/treepins" not in line:
            continue
        try:
            rec = json.loads(line)
        except Exception:
            continue
        for blk in (rec.get("message") or {}).get("content") or []:
            if not isinstance(blk, dict) or blk.get("type") != "tool_use" or blk.get("name") != "Bash":
                continue
            for s in sec.segments((blk.get("input") or {}).get("command") or ""):
                # the HEAD WORD decides the door, so a bd body quoting a
                # command is not a run — the trap that made the pattern-kill
                # census read text as kills (ranger-base-zbg8o).
                if not PKG.search(s) or sec.FILTER.search(s):
                    continue
                head = s.split()[0].rsplit("/", 1)[-1]
                rows.append((head, s.split()[1:2], path))
print(collections.Counter((h, tuple(v)) for h, v, _ in rows))
```

It is conservative in the bare direction: a run whose segment begins with an
inline `PATH=$(...)` rather than a command falls out of the count, and one
such `go test ./internal/treepins/` is in the corpus.

### Ruled

**A route, and the honest sentence — both, because neither is complete
alone.** `make treepins` is the route: `scripts/test-times.sh $(GOBIN) test
./internal/treepins -timeout 25m -count=1`, which hands the rule the argv it
names. PROVED BY EXECUTION 2026-10-04 (one slot, already held, a stub `go` so
no suite ran): `make treepins` printed `suite-lock: waiting for suite lock
held by another worktree`, then `slot 1 of 1 acquired after 25s`. Not a
prerequisite of anything — `make test` reaches the package through `./...`
already, and a ~400s door belongs where a person is waiting for its answer,
which is `test-race`'s reasoning.

**And the documents say what is true.** AGENTS.md and NOTES.md name
`make treepins` as the spelling that queues, and say that a bare
`go test ./internal/treepins` escapes the lock exactly as a bare
`go test ./...` does. That is the same sentence the `./...` paragraph three
below has carried since ranger-base-uvzjk, and before this the one bullet
carried both claims about one lock: a seat reading the new one would not
route the run and would believe it was queued, which is the false comfort
ranger-base-7zng1 was filed against.

**Not withdrawn, and not widened.** The `-run` exemption is untouched, so
`make tree-check` and every focused pin run are as unqueued as they were. The
sibling candidate — a bare `go test ./internal/posse`, arm 1 of a ~950s
package — is still deliberately left alone for the reason the section above
gives.
