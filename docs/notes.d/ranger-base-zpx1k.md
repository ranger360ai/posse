# A door on ADR 0097 D3's three exits, and why the parser stayed in the shell (ranger-base-zpx1k)

`scripts/posse-wall-check` maps a venue run to the three exits ADR 0097 D3
defines. Nothing in the suite read that mapping, and the one arm D5's "an
abstention is never a pass" exists for had crashed in draft. This is the door,
the choice behind its shape, and what it cost.

## What was undoored, and what it had already cost

The body's whole value over `go test; echo $?` is the mapping: a member FAIL
is 1, a member SKIP or zero members is 3, a build error is 1, green is 0 with
the mandated last line. `make verify-python-syntax` censuses `.py` FILES, so a
parser reached through `python3 -I -c` inside a shell script is outside its
corpus — as it is for `queue-scrub.sh`, `suite-lock.sh` and `ceiling-fill.sh`
— and a syntax check would not have caught the live defect anyway.

The live defect, recorded one bead earlier in
`docs/notes.d/ranger-base-lagas.md`: the first draft of the ABSTENTION arm
carried a format string with three conversions and two arguments, a `%s` typed
where `/var/tmp` was meant. It raised `TypeError` and the script exited 1 — a
RED where the contract says 3, which D5's ratchet would have filed as `wall
pins red at <tree>` against whoever landed last, over a tree whose pins had
merely gone dark. Reading the code did not find it; driving the parser over
nine synthetic `go test -json` streams did. Same class as ranger-base-ietka: a
closed-set pin whose failure message crashed on the one direction it existed
for.

## The choice: a `--self-test` arm, not `scripts/wall-check-parse.py`

The bead carried both shapes and called the choice non-obvious. It is
decidable, and what decides it is ADR 0088 D1's first line rather than D4's
literal drift:

> No arguments, and **cwd is the tree to judge**. The copy that RUNS comes
> from this check's last green tree, not from the tree being judged.

A sibling `scripts/wall-check-parse.py` can only be found two ways. Out of
`$BASH_SOURCE`/`$0`, which D1 forbids outright, because that names a DIFFERENT
tree — or through a relative path, which under D1 names the tree being JUDGED.
Take the second and the shell body comes from the last green tree while the
parser comes from the landing: a landing could weaken the half that decides
its own verdict, which is the exact hole D1's paragraph exists to shut. The
`.py` file would have bought `verify-python-syntax`'s compile — a check that
could not have caught a runtime `TypeError` — at the price of the contract.

So the parser stays in the shell and the door is the house pattern:
`--self-test` arms in the body, a `verify-*` target, its row in
`scripts/verify-box.sh`, and a Go pin that runs them
(`shell-syntax.sh`, `python-syntax.py`, `reap-test-tmp.sh` and `gotest.sh` all
have one).

## What landed

- `scripts/posse-wall-check`: the `python3 -I -c` invocation is now inside a
  `read_verdict()` function — one call site for the run and the arms — and
  `--self-test` drives it over **ten** planted streams. The nine the draft was
  found by, plus the mandated-last-line arm. It runs no `go test`, starts no
  bwrap, makes no temp file and needs no venue, which is what lets it be a
  prerequisite rather than a thing somebody types.
- **Every arm asserts a LINE and not just a status.** A status-only self-test
  catches the draft typo in the abstention arm and misses the same typo in the
  member-FAIL arm, where a crash and the verdict share exit 1. Every arm also
  refuses a traceback outright.
- `make verify-wall-check`, a prerequisite of `test:` and `test-arm1:`.
- `verify-wall-check` in `scripts/verify-box.sh`'s EXCLUDED table, with the
  reason, because it measures no box state. Its reason cites `make test`, so
  `TestQABoxCheckExclusionsThatCiteMakeTestAreTrue` holds it to that
  prerequisite line as well.
- `internal/treepins/wallcheckselftest_qa_test.go`, four arms. Three on
  `gotestreuse_qa_test.go`'s model: the target still runs the arms AND `test:`
  still names it; the arms still answer, required by label, with a floor and a
  check that all three exits are read by something that PASSED; and the
  self-test can fail — arm 3 puts the draft's `%s` back into a copy and
  requires a red that names the abstention arm and says the parser raised.
  Arm 4 is the exits the SHELL decides, below.
- `scripts/posse-wall-check` dispositioned in `twdExecDispositions`: in
  `--self-test` it reads nothing at all.

## The one interaction, and it is the reason the parser is not a variable

The first draft of this bead lifted the parser into a shell variable
(`parser_py='…'`) and called `python3 -I -c "$parser_py"`. That reds
`TestQANoEntryPointRunsGoTestOnTheDefaultTimeout`:

```
scripts/posse-wall-check:250 runs `go test` on the default 10m timeout:
    not go test framing. One rule for a failure line and for a skip line,
```

ranger-base-enhtx widened that pin's corpus to extensionless shell files three
commits earlier, and it suspends its reading over an embedded program by
arming on ONE shape — a quote opened right after the `-c` of a command that is
not a shell (`embeddedProgramQuote`). A `parser_py='` opener is not that shape,
so 128 lines of python were read as shell and a docstring line came back as an
entry point on the 10m default. Keeping the `-c` spelling and wrapping it in a
function keeps that reader armed and needed no change to enhtx's pin. Its two
comments that quote the live spelling are updated — to `read_verdict` by name
rather than by line number, which moves on every edit to the header above it,
and to `printf '%s\n' "$1" | python3 -I -c '`.

A second line of the same shape: the EXCLUDED row's prose said "it runs no `go
test`", which that pin reads as an invocation because a heredoc body is not a
comment. It says "it runs no suite" and "ten planted test2json streams".

## MEASURED 2026-11-14

D1 host, Linux 7.2.15, bubblewrap 0.13.1, go1.27.3, 16 cores, seat at cage
`shims`, one-minute load average 4.89 at the probe arms.

| arm | result | wall |
|---|---|---|
| `make verify-wall-check` | 10 arms clean | **0.23s** over three runs (0.233, 0.228, 0.235) at a load of 3.97 |
| the four Go pins | PASS | 0.19s for arms 1-3, 0.01s for arm 4's six subtests |
| **mutation**: the draft `%s` put back | **2 of 10 arms FAILED**, both abstention arms, `TypeError` quoted | 0.4s |
| control, nesting process: `./scripts/posse-wall-check` | exit 0, `wall check: green, 35 pins measured` | 82.5s |
| venue: `bash docs/adr/0097-wall-venue.probe.sh venue+check` | exit 0, same 35 | 91s |
| seat-proc: `… seat-proc+check` | exit 3, `venue cannot nest: bwrap: setting up uid map: Read-only file system` | 0s |
| `make tree-check` | green | 45s then 45s, at a load of 8.34 and 7.07 |
| `make test` | green, silent-revert audit 2712 commits 0 untriaged | **1080s** end to end, 151s of it queued behind two sibling seats' suites. Arm 1 internal/posse 206.1s, internal/treepins 169.4s, cmd/posse 139.2s; arm 2 65.5s; arm 3 98.9s |

**35, where ranger-base-lagas read 32 ten days of landings ago** — the count is
the run's and not a constant anywhere, which is the point of reading membership
off the marker. The two readings either side of the refactor agree, so the move
of the parser into a function cost the judge nothing.

The mutation reds BOTH abstention arms, not one: the skip arm and the
no-verdict arm reach the same message. That is the arm-per-shape design paying
for itself — a single arm would have said "the abstention exit is broken" where
ten say which shapes reach the broken line.

## The shell's exits, and why they are the PIN's arms and not the self-test's

`--self-test` drives the parser, which is where the draft defect was. Six more
exits are decided ABOVE the parser and were read by nothing: the no-argument
contract's 2, `--plan`'s 0, the cwd guard's 3, a host with no `bwrap`, a host
with no `python3`, and a baseline that refuses.

They cannot be `--self-test` arms. Reaching them from inside the body means
re-execing it, and the only way to name the body from itself is `$0` or
`$BASH_SOURCE` — forbidden by ADR 0088 D1 in one sentence, for the same reason
the parser is not a sibling file. Weakening the line this file leans on
hardest, in order to test that line, is not a trade worth making.

A PIN has no such problem: it names the script already, so it can hand the
body a cwd, a PATH and an argument of its choosing. That is arm 4 of
`wallcheckselftest_qa_test.go` — six subtests, `0.01s` all told, each planting
at most a two-line `bwrap` stub on an otherwise empty PATH. The cwd arm is the
only one that resolves an absolute path, and that resolution is the pin's, not
the body's.

**And each arm asserts that the run never happened**, by the absence of
`go test exited` from the output. Both preflight asks are supposed to come
BEFORE the 83s run — a venue run that then exits 127 for want of a parser is
off D3's table entirely — and nothing held that ordering until now.

MEASURED 2026-11-14, the six arms: exit 2 with D1's sentence; exit 0 with the
plan and nothing run; exit 3 with `is not a tree this check can judge`; exit 3
with `bwrap: not found on PATH`; exit 3 with `the baseline bwrap did not
start`; exit 3 with `no python3 on PATH to read the run` after the baseline
passed. ranger-base-lagas measured four of these by hand and this is the first
time any of them is read by the suite.
