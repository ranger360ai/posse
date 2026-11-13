# A leaked scratch FILE is outside every class the reaper has (ranger-base-u1z89)

2026-11-12. Fifth bead in the family that asks what `scripts/reap-test-tmp.sh`
can and cannot reach — ranger-base-4dw7b, ranger-base-m0j9n,
ranger-base-i78k9, ranger-base-tbukc — and the first whose answer is not a
rename. tbukc's finding was "five roots match no row in the class table".
This one is one step worse and one file type over: **every candidate walk in
that script is `-type d`**, so the table is a table of DIRECTORY classes and a
leaked scratch FILE is outside all nine rows. Not a 24h quarantine. Not
tbukc's "no row globs this name" either. There is no walk that could reach it
whatever it is called — including the `tmp.??????????` row that globs exactly
the name a bare `mktemp` produces.

Found with a `mktemp` PATH shim recording the argv every script handed
mktemp(1), while fixing tbukc: `scripts/verify-gate-freshness.sh` produced two
calls with no `-d` and the template `${TMPDIR:-/tmp}/bd-argv-gate.XXXXXX`.

/tmp here is a 14G tmpfs with `nr_inodes=1048576` and one box-wide quota for
uid posse, and the near-outage this reaper exists for was inode exhaustion at
29% of bytes. An inode is an inode: a one-inode leak at high frequency costs
what a four-inode dir at low frequency costs.

## The producers, and why the gate was the one that mattered

MEASURED 2026-11-12 by grep over tracked sources — **10 live `mktemp`-FILE
call sites in 5 files** that land in `/tmp` or `$TMPDIR` (the bead's own
description said 11; recount says 10, and the lines had moved since it was
written):

| site | template | lifetime |
|---|---|---|
| `scripts/bd-argv-gate.sh:62` | `bd-argv-gate.XXXXXX` | one per bd-ish tool call, every seat |
| `scripts/gotest.sh:168` | bare, so mktemp(1)'s own namespace | one sha256 self-check |
| `scripts/reap-test-tmp.sh:932` | four bare ones | the reaper's own run |
| `scripts/reap-test-tmp.sh:933` | two more | ditto |
| `scripts/test-times.sh:1643` | `posse-test-times.XXXXXX` | the whole suite, up to 60m |
| `plugin/autostart.sh:217` | bare | one `dispatch --watch-status` |

`scripts/gotest.sh:304` is excluded: its file is under `$CACHE`
(`$HOME/.cache/posse/testbin`), which the reaper does not sweep.

The gate is a `PreToolUse` hook on every bd-ish tool call of every seat on
this box — by far the most frequent scratch creator here. It carries
`trap 'rm -f "$err"' EXIT HUP INT TERM`, so the ordinary path and every
catchable signal clean up; SIGKILL and the OOM killer do not, and nothing else
would ever have collected what they left.

And `reap-test-tmp.sh` itself was six of the ten. The reaper's own leavings
were in the one leak class the reaper cannot see, on the quota it exists to
protect.

## Shape (b): move the producers, do not widen the reaper

The bead offered two shapes. **(a)** give the reaper a `-type f` arm with the
same flock/holder/mtime gates. **(b)** have each site write its scratch file
inside a `posse-`named dir it already owns, so the existing dir classes
collect the parent. (b), for three reasons in the order they decided it:

1. **All three of ADR 0089 D2's gates are weaker on a file.** The lock is
   vacuous — no creator on this box takes a lock on its scratch file, so step
   1 stops being a positive proof of life and becomes a step that always says
   yes. The `/proc` fd that names the file is open only while something is
   writing to it. And a shell's `f=$(mktemp)` is in no argv, which is ADR
   0089's own measurement one file type over. The grace would be the whole
   gate — ADR 0089's own objection to grace — and it would be the only class
   where no creator could be made to fix that.
2. **The glob could not be `posse-*`.** MEASURED 2026-11-12 on the live /tmp:
   **38 depth-1 files against 10847 dirs**, and three of the 38 are
   deliberate, stable-named artifacts a person is meant to come back and read
   — `posse-test-signal.log` (726163 bytes: the process table at the instant a
   suite was signalled, which AGENTS.md sends the reader to),
   `posse-coldexec-report`, `posse-make-install.log`. A `posse-*` file arm
   collects the evidence. A template-shaped glob could tell them apart, which
   is one more rule to keep true for one inode of return —
3. — because **it would have bought one inode**. Exactly one of those 38 files
   was a leaked scratch file: `/tmp/posse-test-times.n3Ffyo`, 0 bytes,
   2026-11-11, from a suite killed past its traps. All ten producers are ours,
   so moving them closes the class rather than quarantining it.

What was already on the floor stays there until the box reboots: /tmp is a
tmpfs, which is the same collector of last resort the aged-out
`claude-scratch` dirs already rely on. The one name is above.

### The cost on the hot path, which is what wanted measuring

The bead's worry about (b) was "it costs the gate an extra mkdir on the hot
path, which wants measuring before it is chosen". It does not cost a PROCESS:
`mktemp -d` is the same single fork `mktemp` was, and `rm -rf` the same single
fork as `rm -f`. What it does cost was measured twice, on the gate itself and
on the two shapes alone.

**The gate, old against new, interleaved** — three pairs of 200 calls on a
payload that reaches the parser, MEASURED 2026-11-12 (linux, this box) on a
quiet box, loadavg 1.7-1.9, old resolved from `git show HEAD:` and both
pointed at the same parser:

| arm | per call |
|---|---|
| old, a `mktemp` FILE | 26735, 26557, 26465 us — mean 26586 |
| new, a `mktemp -d` | 26968, 26662, 26711 us — mean 26780 |

**+194 us, 0.73%, the same sign in all three pairs.** (Per-call wall here
includes the `sh <gate>` fork and the payload pipe the rig pays for every
call; the comparison is paired, so that is common to both arms.)

**The two shapes alone**, create-write-remove, 4 interleaved pairs of 300,
MEASURED 2026-11-12 at loadavg 15-20:

| arm | per iteration |
|---|---|
| A, a `mktemp` FILE + `rm -f` | 1323, 1374, 1317, 1319 us — mean 1333 |
| B, a `mktemp -d` + `rm -rf` | 1376, 1338, 1305, 1409 us — mean 1357 |

**+24 us**, against a 57 us spread inside arm A itself — so most of the 194 us
above is `rm -rf`'s extra syscalls over a directory, and not the mkdir the
bead expected to pay for. Neither number is visible against the gate's own
spread when the box is busy: three consecutive readings of the UNCHANGED gate
at a load of 17-19 read 36.9, 48.5 and 63.4 ms per call, a 26 ms spread
between readings of identical code.

## What each site looks like now

Every one of them is `mktemp -d "${TMPDIR:-/tmp}/posse-<what>.XXXXXX"` plus a
**fixed-name** file inside it — which is why the file census below finds
nothing: a dir that is already unique needs no second mktemp for its child.

- `scripts/bd-argv-gate.sh` — `posse-bd-argv-gate.XXXXXX/stderr`.
- `scripts/gotest.sh` — `posse-gotest-hashcheck.XXXXXX/probe`.
- `scripts/test-times.sh` — `posse-test-times.XXXXXX/log`, the same template
  its own self-test root at :855 already used. The dir is also HELD by the
  `/proc` scan while the run lives, because `tee` holds the log open and the
  fold takes that fd up to the depth-1 dir — which the old file path could
  never reach, since no candidate matched it.
- `scripts/reap-test-tmp.sh` — one `posse-reap.XXXXXX` holding all six lists
  (and `stale.body`, the bead arm's seventh), so six forks become one. The six
  are then created empty, which mktemp did for free and which two of them are
  read before anything writes to them.
- `plugin/autostart.sh` — `posse-autostart.XXXXXX/stderr`.

### The reaper now has to hold itself out

Its scratch dir is a `posse-*` dir at depth 1 in a root it sweeps, and the
`/proc` scan genuinely cannot see it: `rawrefs` is read before the dir exists,
and the path then lives in a shell variable, which is ADR 0089's "invisible to
the scan" one step over. The grace covers it — 120m against a dir that lives
seconds — but `--min-age 0`, or any `--min-age` a run can outlive, would
nominate it, take its lock and `rm -rf` the candidate list the run is reading
from. **Until this bead the hole was what protected it**: the six files were
`-type f` and no walk could see them. So the dir is appended to the held set,
below the "candidates a live process names" report rather than into it, and
printed on its own line — an exclusion nothing prints is an exclusion nobody
can check, and the window it covers is one no ordinary run reaches.

## The pin, so this is not found by hand a fifth time

`internal/treepins/mktemproot_qa_test.go` grew a second arm, doored by `make
scripts-check`:

- **The file rule is a prohibition, not a template.** A live `mktemp` with no
  `-d` is on `mfExempt` with its reason, or it reds. There is no conforming
  spelling to offer, because the house answer is not to make a scratch file in
  a swept root at all.
- **It walks `plugin/` as well as `scripts/`,** and only this arm does. The
  root census's subject is the directory three beads walked by hand; widening
  it would be a claim about roots nobody censused. The file rule is new, has
  no legacy, and starts at every directory in this tree holding shipped shell.
  `docs/adr/*.probe.sh` is out of both on purpose: operator-run artifacts, and
  their bare `mktemp -d` roots do land in the reaper's `tmp.??????????` class
  at 24h.
- **Two live file calls remain, both rostered** — `reap-test-tmp.sh`'s own
  class-table row (`mktemp|tmp.??????????|1440|1`, which a line reader cannot
  tell from a call) and `gotest.sh`'s `$CACHE` build temp. That pair is also
  the arm's blindness control: a census with one or two live members cannot
  prove it is sighted by counting the way the root arm's `roots < 20` floor
  does, so the roster is checked both ways and a reader that stopped finding
  anything reds on a stale row instead of going quiet.
- **One reader, two rules.** `mtParse` used to answer `ok=false` for a call
  with no `-d` — "a file and not a root in any of the reaper's classes" — and
  that sentence was the hole, not a scope. It reports both kinds now and
  `isDir` tells the arms apart, so the file census and the root census cannot
  drift about what a call is. The price is one `mfExempt` row for prose, which
  is the trade this pin's own head comment already made: over-counting costs a
  roster row and a sentence, under-counting is the next bead in the family.

### Both arms spell their own walk, and that is deliberate

MEASURED 2026-11-12: with the walk behind a `roots ...string` helper, `make
register-check` reported BOTH arms "no longer derived as tree-wide ... the pin
is back to being a package-tree run away". `treewidedoor_qa_test.go` derives
this class from an enumeration rooted at a path spelled as a LITERAL in the
test, and a variadic parameter hop is a spelling that rule cannot see. So
everything that is a RULE is shared — the reader, the floor, the rosters, the
`fs.WalkDirFunc` body — and what is repeated is the `filepath.WalkDir` call
itself, which carries no claim.

## MEASURED 2026-11-12, the census after the fix

43 grep hits for `mktemp -d` under `scripts/`: 11 comment lines, 1 prose row
inside `verify-box.sh`'s quoted `ROSTER_EOF` heredoc, **31 live roots** — 28
the house spelling, 3 on `mtExempt`. The walk sees 63 files under `scripts/`
and 65 with `plugin/`, and **2 live `mktemp` FILE calls**, both rostered and
neither in a swept root.

## Verified by running it

- The gate on all five of its decision paths — allow, an allow-list refusal,
  the parser's own exit-2 refusal (the one path that READS the stderr file),
  parser-unavailable fail-closed, and the fast path — each with the right exit
  code and reason, and `0` leftovers in /tmp afterwards.
- `make verify-reap-test-tmp` (24 arms), and a rig that plants a DEAD reaper's
  aged `posse-reap.*` dir plus an aged depth-1 `tmp.ABCDEFGHIJ` FILE in a
  scratch root and runs `--apply` with `TMPDIR` inside it: the dead run's dir
  is collected, this run's own dir is named on stdout and survives, and the
  file is untouched — which is shape (a) refused, demonstrated.
- `scripts/test-times.sh --self-test` and a real timed run; `scripts/gotest.sh`
  over a filtered package (its `hash_selfcheck` is fatal if the probe fails);
  `go test -tags posse_arm3 -run TestAutostart ./internal/posse` for the hook.
- `make tree-check` whole, and the new arm's own ability to say no: it red on
  the one unrostered file call in the tree before that row was written.

## Not closed here

The live gate on this box is the operator's copy at
`~/.config/posse/gate/bd-argv-gate.sh` (ADR 0015 §3) — `scripts/bd-argv-gate.sh`
here is SOURCE, and landing a fix to it moves nothing until someone installs
it. `make verify-gate-freshness` compares the installed copy against the MAIN
checkout's HEAD and will report drift once this lands; that reinstall is the
operator's line to type, and until it is typed the hot-path producer is still
making one file per call.

`internal/posse/gates.go`'s `chainDispatcher` renders a probe line containing
`t=$(mktemp); … rm -f "$t"` for an operator to TYPE, matching INSTALL.md §9.
It is documentation rather than a path this tree runs, and it is in neither
census.
