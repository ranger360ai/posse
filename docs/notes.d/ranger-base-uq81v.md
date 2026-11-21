# A scratch Rex server is reached by `-S` and by nothing else (ranger-base-uq81v)

ranger-base-uq81v, 2026-11-20, from the incident of 2026-11-15. A live test
that meant to run against a scratch Rex server ran against the fleet's own and
destroyed eight live seats. This fragment is the working record beside the
fix: the measurements that say why the isolation the script expected does not
exist, what landed instead, and what the mutation battery found in it.

Environment for every reading here: Linux 7.2.15-1-omarchy, 16 cores, rex CLI
0.0.1794069975-g6c6689, posse built from this worktree, python3 3.14.

## The incident, in one paragraph

2026-11-15 08:31:19-22 local. A scratchpad script started a scratch rex server
on a private socket, exported `XDG_RUNTIME_DIR` to its directory, and expected
the rex CLI and posse to resolve only that socket. Its own sanity line printed
`"endpoint": "unix:///run/user/<uid>/rex/server.sock", "session_count": 12` —
the FLEET server, with twelve live seats on it — and the run continued: it
created seats there through `posse new`, and its cleanup loop ran `rex kill` on
every session in `rex ls`. Eight sessions destroyed, across five personas and
the operator's own seat, one of them the seat running the loop, which is what
ended it. Three survived only because the loop died first. The reading that
would have stopped it was taken, printed, and passed over.

## MEASURED: what the rex CLI reads for its endpoint, and what it does not

Read-only `whoami --json` under `--autostart=false`, one variable changed per
row, 2026-11-20:

| how the endpoint was given | what answered |
| --- | --- |
| nothing | the box's default socket |
| `XDG_RUNTIME_DIR=<scratch>` | the box's default socket — **ignored** |
| `XDG_RUNTIME_DIR=<scratch>` + `HOME=<fake>` | the box's default socket — **ignored** |
| `REX_RUNTIME_DIR=<scratch>` | `<scratch>/server.sock` |
| `REX_SERVER=unix://<scratch>/…` | the box's default socket — **ignored** |
| `-S unix://<scratch>/…` | `<scratch>/…` |

So rex's runtime dir is `REX_RUNTIME_DIR` else a `/run/user/<uid>` path it
builds from `getuid()`; `XDG_RUNTIME_DIR` is not read for it at all, and
`/run/user/` is a literal in the binary. Note also the socket is
`<runtime dir>/server.sock` and not `<runtime dir>/rex/server.sock`, which is
the shape the incident's script assumed.

**`REX_SERVER` is the dangerous one.** It is a variable rex EXPORTS into a
pane, beside `REX_SESSION` and `REX_BLOCK` — not one the CLI reads. A bogus
value changes nothing and raises nothing; a bogus HOST LABEL changes nothing
and raises nothing. A harness that "passes the endpoint explicitly" that way
addresses the fleet server and reports that it did not.

## And posse has no endpoint door whatever

`NewRexMux` builds `Rex{Bin: a.RexBin()}` with an empty `Server`, and
`rexGlobals` emits `--server` only when that field is set (internal/posse/
muxrex.go, internal/posse/rex.go). No config key and no environment variable
moves posse off the box's default socket. The ONE seam is the binary —
`rex_bin:` / `RHQ_REX_BIN` — which is why the harness below interposes on it
rather than hoping, and why `posse new` in the incident's script reached the
fleet server even though the script had set every variable it could think of.

## What landed: scripts/scratch-rex.sh and its three guards

`scripts/scratch-rex.sh [--sessions N] -- COMMAND …` starts the server, then:

1. **IDENTITY.** `whoami --json` is read back through `-S <the socket this run
   started>` and the run aborts, exit 70, unless the reported `endpoint` is
   byte-equal to it and `session_count` is 0. An empty read is a mismatch.
   Nothing runs before this passes.
2. **INTERPOSITION.** The body runs with a shim at the head of `PATH` and in
   `RHQ_REX_BIN`. The shim is the same script invoked through a symlink named
   `rex`, so there is one copy of the fences and the self-test drives the code
   the shim runs. It prepends `-S <the scratch socket> --autostart=false` to
   every call, refuses a `-S`/`--server` of the body's own, and refuses the
   `server` verb. A bare `rex`, and posse called from that body, cannot reach
   the fleet server by any spelling.
3. **THE KILL FENCE.** `kill` with no id is refused, and so is an id not in the
   record the harness keeps — refreshed from the scratch server's own `ls -q`
   after every successful `new`, posse's `new` included, since that goes
   through the shim too. So the incident's cleanup loop kills what the run
   made and nothing else, and the harness's own cleanup goes through the same
   fence rather than beside it.

`--autostart=false` is injected on every shimmed call, `new` included, where
posse itself lets the default stand. The scratch server is already up, so
autostart never fires and the behaviour observed is posse's; what it buys is
that a scratch server that DIED fails honestly instead of being replaced by an
empty one, which is ADR 0076 §5's reclaim hazard.

## MEASURED: the live run, 2026-11-20

`scripts/scratch-rex.sh --sessions 6 -- bash <probe>` on this box, with the
fleet holding 7 sessions throughout:

- a bare `rex whoami` from inside the body answered the scratch socket,
  `session_count 0`.
- the body's own `-S <the fleet socket>`, `rex server run`, `rex kill` with no
  id, and `rex kill <an id this run did not create>` were each refused by the
  shim, exit 70, before the real binary was reached.
- `posse new husk --cmd …` through `RHQ_REX_BIN` created its session ON THE
  SCRATCH SERVER and that session landed in the record, so the cleanup loop —
  typed in the shape live.sh had it, `for s in $(rex ls -q); do rex kill "$s"`
  — killed all three sessions of the run and left the server empty.
- `rex ls -q` on the fleet server before and after: 7 sessions, byte-identical
  id lists. `go run ./cmd/checkorphans`: clean. No `posse-scratch-rex.*` root
  left under the runtime dir.

And the negative, with a fake rex that listens where it is told and then
answers for the box's default socket with `session_count 12` — the incident's
own sentence, made deterministic: `REFUSING:` and exit 70, with the body never
reached. That fake is now arm 2 of the pin.

## The pins, and the two gaps the mutation battery found

`internal/treepins/scratchrex_qa_test.go`, doored by `make treepins-check`
through ADR 0098's complement: the `--self-test` table (the guards' predicates,
opening nothing), the identity abort over a fake rex in three modes, the shim's
fences at their CALL SITES, the harness's own `-S` discipline read out of its
text, and a census over every tracked `.sh` and the Makefile for a rex call
that names no server, with a control arm of must-flag and must-pass lines.

Ten mutants, one exact-string edit each over an otherwise unmodified
`scripts/scratch-rex.sh`, the file restored from a byte copy between them and
the anchor asserted to occur exactly once before each plant. The first run
reported two anchors "not unique" and they were nothing of the kind: the check
was `grep -cF`, which counts matching LINES, so a multi-line anchor can never
count 1 that way. (ranger-base-ybrk6's battery had the same class of apparatus
bug one tool over — a `sed` whose delimiter appeared in the pattern — and the
same consequence: a row that plants nothing reads exactly like a row that
found no hole.)

| # | mutant | result |
| --- | --- | --- |
| — | baseline | green |
| 1 | `endpoint_is` always matches | KILLED by arms 1, 2 |
| 2 | the identity abort is removed | **SURVIVED**, then killed |
| 3 | the `session_count` check is removed | KILLED by arm 2 |
| 4 | the kill fence stops consulting the record | KILLED by arm 3 |
| 5 | a `kill` with no id is let through | KILLED by arm 3 |
| 6 | a body may name its own server | KILLED by arm 3 |
| 7 | the `server` verb is let through | **SURVIVED**, then killed |
| 8 | the identity read loses `-S` | KILLED by arms 2, 3, 4 |
| 9 | the shim pass-through loses `-S` | KILLED by arm 4 |
| 10 | the record is never refreshed | **SURVIVED**, then killed |

**Mutant 2: the two halves of guard 1 were covering for each other.** The
first table had two rows, and both of them made the endpoint AND the count
wrong — because the incident's reading was wrong both ways at once. So
deleting the endpoint comparison left the count check to abort the run, exit
70, body unreached, every assertion satisfied. The fix is a third row that is
wrong one way only (`fleetzero`: the fleet's socket answering for an empty
server) and a per-row assertion on what the refusal NAMES, so a refusal for
the other half's reason cannot stand in for this row's.

**Mutant 7: a counter of failures cannot tell a fence from a broken command.**
The fences arm counted nonzero exits. With the `server` verb let through, the
call reached the fake rex, which tried to bind a socket in a directory that
does not exist and failed — so the count still reached five. The arm reads the
shim's own refusal prefix now, and a call that failed for any other reason
prints `NOT FENCED:` and fails the arm.

**Mutant 10: nothing held the positive half of the fence.** Fix direction 2 of
this bead is "cleanup kills ONLY session ids that the script itself created",
and the arms held the ONLY. A record that is never refreshed makes the fence
refuse the run's own sessions too, which no arm noticed — and a fence that
refuses the run's own work is the one a later seat deletes in frustration. The
fake rex keeps a session list in a file beside itself now, so the arm creates a
session through the shim, asserts it reaches the harness's record, and kills it
through the fence.

## Three more holes, found after the pins were green

**`-S` has three spellings and the fence knew one.** MEASURED 2026-11-20: rex
takes `-Sunix:///…` with the value ATTACHED exactly as it takes `-S
unix:///…` and `--server unix:///…`, and pflag clusters shorthands, so
`-vSunix:///…` is the same flag a third time. The shim's refusal tested the
whole word against `-S`, so two of the three went straight through to a call
that would then address whatever they named — guard 2 open, every arm green.
It reads the short cluster character by character now, stopping at `-C` whose
own value follows it in the same word, with all three spellings in the
self-test table and in the live fences arm.

**An assertion arm's matcher must not fork**
(internal/treepins/selftestforkarm_qa_test.go, ranger-base-t07yx): two rows of
the self-test flattened a newline-joined list with `| tr`, which reports the
property false when it is the `tr` that could not be exec'd. They use
`${ids//$'\n'/ }` instead. The pin named the rule, the two lines and the
shapes already in the tree, which is what that class of pin is for.

**A new AGENTS.md "Landing the plane" bullet is a two-file change**
(internal/treepins/landingcopy_qa_test.go, ranger-base-69nqp): INSTALL.md §9
plants that section into a fresh work repo, so a bullet either goes into its
heredoc too or into `landingDropRegister` with the reason a fresh work repo
cannot resolve it. This one is registered: the harness and the posse source
its measurements are about live only in this repo, and a fresh work repo has
no fleet of seats on a shared Rex server to protect.

## Residuals

- **A seat's own scratchpad script cannot be pinned**, and the incident's was
  one. What binds it is the harness being there and being reachable: AGENTS.md
  carries the bullet, which is prose for the same reason the `go test ./...`
  bullet beside it is.
- **The fence matches a session id exactly** as `rex ls -q` prints it
  (`session:<uuid>`; that CLI prints the same string for `--short` and
  `--short=false`). A unique prefix, which the rex CLI itself accepts, is
  refused, with the record's path in the refusal. Widening it to prefixes
  would be a matcher to get subtly right in the one place whose whole value is
  that it cannot be.
- **The harness gives the body no scratch `RHQ_HOME`, `HOME` or cwd.** A
  scratch socket is not isolation on its own — scripts/verify-ghost-composer.sh
  says the same thing about herdr — so a probe that writes posse records sets
  `RHQ_HOME` itself. What belongs in those is the caller's question.
- **`REX_RUNTIME_DIR` is left alone.** It does move the CLI, so a harness could
  have used it instead of `-S`; it is not used here because it is inference
  again — one more variable whose non-effect is silent if a future rex renames
  it, where a missing `-S` is a flag that is simply not there and a pin can
  read it.
