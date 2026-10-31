# Every binary posse execs: protocol or a persona's command (ranger-base-bm6j8)

2026-10-31. The census ADR 0077 D3's scope line asked for: "`Bd` now
(measured); the other exec sites are a census bead, each judged on whether it
is protocol or a persona's command." ranger-base-rzjie moved `bd` alone,
because `bd` was the only site where the hazard had been MEASURED. This is
the rest of the tree.

## The question, and why it is not the one WaitDelay asks

ADR 0009: **the walls bind what a persona TYPES.** A binary posse execs as
part of its own protocol is posse's, not the typing persona's — so it is
resolved and run as though no gates dir were on PATH at all (ADR 0077 D3).

A posse process that dispatches, launches, checks parity or renders a wall is
a child of whatever shell asked for it, and that is routinely another
persona's SEAT. MEASURED 2026-10-31 (ranger-base-c33j6): a probe session
found a gates dir mid-PATH that the launching process did not put there — on
a herdr home the multiplexer server had been autostarted hours earlier from
another seat and donated that seat's PATH to every session it would ever
open. The cost, measured twice on the same bead: posse took ADR 0073's
per-store write lock, exec'd `bd` by bare name, landed on the launching
persona's `bd` shim, the shim took the SAME lock on a new open file
description, and posse waited `BdWriteLockWait` on itself and refused the
write — **every dispatch from a gated seat**.

So the classification is **whose environment is this process**, which is a
fact about the posse process that reaches the line, not about the program
being run. That is the opposite key from `ewdExemptBinaries`
(internal/treepins/execwaitdelay_qa_test.go), which rules by BINARY because
"can this child leave a descendant holding the pipes" is a fact about the
program. The same `docker` argv is **protocol** when parity asks the engine a
question and the **seat's own** when `posse <runtime> --posse-cage-launch`
becomes it in the pane. Both rulings live beside their own census; neither
roster can answer the other's question.

## The mechanism

`internal/posse/protocolchild.go`, three functions:

- `protocolBin(name)` — a bare name resolved against the gates-free PATH
  (`resolveOutside`), because Go's `exec.LookPath` searches THIS process's
  `$PATH` and nothing else, not even the `cmd.Env` about to be set. A name
  that NAMES a path keeps precedence: an explicit binary is an answer, not a
  search. A bare name nothing outside the gates answers is an **error**. This
  is `Bd.bin`'s rule, lifted rather than restated — `Bd.bin` is now one call
  of it, and so are `WrapInCage`'s engine resolution and `runRuntimeMint`'s
  (each keeping its own sentence, which names the key an operator would fix).
- `protocolEnv(env)` — `env` (nil meaning this process's) with `PATH` put
  over it. SET and not merely resolved past, because a child's own children
  call binaries by name too: bd's git hooks call `bd`, a `sh -c` predicate
  calls `git`. Only `PATH`: `RHQ_GATES_DIR` and `RHQ_TOOLS_DENY` stay, since
  they are how a shim that IS reached knows whose wall it is, and shedding
  them would be a way to run a shim with no rules rather than not to run one.
- `protocolCmd(cmd)` — both halves on one child, handed back, so a chained
  `.Output()` stays one expression. It WRAPS the `exec` call rather than
  replacing it: a `protocolCommandContext()` of our own would have hidden
  every bounded child from `TestQAEveryBoundedChildNamesItsWaitDelay`, which
  censuses `exec.CommandContext` by name.

`cmd.Args[0]` is left alone — only `cmd.Path` moves — so the child's own
argv0 and every error message built from `cmd.Args` read exactly as before.

**The corner that used to be open is closed, and not by me.** A bare name
that ONLY a gates dir answers — a shim rendered for a denied command whose
real binary is not on this box — was going to that shim, because
`exec.Command` has looked the name up on the gated PATH before any of this
runs and `resolveOutside` then returns "". ranger-base-rzjie's fallthrough
said it "goes to exec unresolved so that exec reports it the way it always
has"; this bead first shipped that sentence forward into `protocolBin` with
the corner labelled UNMEASURED and deliberately not papered over.

**It was measured the same day, by ranger-base-cwsi8 (the verifying seat of
ranger-base-nqy1h), and the sentence was wrong.** exec does not report it —
it RUNS the shim: with the real bd behind the fixture shim taken away and one
ordinary non-gates PATH entry holding nothing called bd, the shim ran with
posse's own argv and `runOnce` came back after 2.011s refused on its own ADR
0073 store lock. ranger-base-c33j6's deadlock, reinstated on the production
spelling, since `NewBd`'s `Bin` is the literal `"bd"`. It had not fired only
because a real bd sits at one PATH entry outside every gates dir — there is
none in `/opt/homebrew/bin`, `/usr/local/bin`, `/usr/bin` or `/bin`, and that
entry was the entire margin.

So `protocolBin` returns `(string, error)`, and **the same thin margin is all
that stands behind every other name in the census** — `git`, `sh`, `sqlite3`,
`python3`, `gh`, a container engine. That is the reason the error is the rule
in the shared function rather than a special case for bd. `protocolCmd` puts
it on `cmd.Err`, os/exec's own seam: `Start` returns it having forked
nothing, so `Run`/`Output` hand the caller the sentence instead of running
whatever `$PATH` answered with. A caller that must decide before doing
something expensive asks `protocolBin` directly — `runOnce` resolves before
it takes the store lock.

## The census

46 `exec.Command`/`exec.CommandContext` sites in non-test Go under `cmd/` and
`internal/` (MEASURED 2026-10-31 at this bead's commit, by the pin below over
158 files). **44 are protocol and now go through `protocolCmd`; 2 are not.**

### Protocol — resolved and run outside every gates dir

| site | binary | what posse is doing |
|---|---|---|
| `adrcensus.go` runAdrCensus | `git` | the census's repo toplevel |
| `adrcensus.go` runAdrCensus | `sh` | ADR 0051's predicate, which drives `git` itself |
| `backup.go` stageQueue | `sqlite3` | the beads db through sqlite's online backup API |
| `beadloss.go` gitBead | `git` | the JSONL's history, from inside a `.beads` dir |
| `beadpairs.go` PairCheck | `sqlite3` | the symmetric-pair query |
| `beadpairs.go` pairCheckReader | `sqlite3` | the readable-source probe |
| `beads.go` runOnce | `bd` | ADR 0073's serialized write — ranger-base-rzjie |
| `cage.go` CageImageBuilt | engine `probe:` | can the engine see the image |
| `cage.go` CageEngineLive | engine `live:` | does the engine answer |
| `cage.go` BuildCageImage | `go` ×3 | cross-build posse, `go install` bd, `go env GOMODCACHE` |
| `cage.go` BuildCageImage | engine `build:` | build the image |
| `cageinner.go` CageInnerGatesReady | engine `inner:` | can the image render the gates inside |
| `cagestale.go` CageImagePosse | engine `inner:` | which posse the image carries |
| `ciwatch.go` ghRunJobs / ghRunList / ciCauses | `gh` | the CI reading in a dispatch pass |
| `credential.go` keychainCmd | `/usr/bin/security` | the monitoring credential read |
| `gates.go` posseSupportsBdWrite | `posse` | does the live binary know `gates bd-write` |
| `gates.go` gitPathRaw | `git` | `rev-parse --git-path` |
| `gates.go` execOwnRenders | `sh` | the L3 hook probe, which runs posse's own renders |
| `generatedindex.go` runNotesIndex | `python3` | the notes-index generator at a merge-back |
| `githang.go` gitCapture | `git` | **posse's one bounded git runner** |
| `herdr.go` capture | `herdr` | the multiplexer client |
| `loadguard.go` ×2, `loadguardkill.go` | `ps` | the load guard's CPU census, argv read and kill re-read |
| `personamode.go` gitTracks | `git` | `ls-files --error-unmatch` |
| `promote.go` (object batch) | `git` | `cat-file --batch` |
| `reachability.go` seatbeltReachProbe | `sandbox-exec` | ADR 0013 §4's writability probe |
| `refresh.go` runRuntimeMint | the runtime's mint | `posse refresh`'s human-gated browser flow |
| `rex.go` capture | `rex` | the multiplexer client |
| `runtimeprobe.go` readCLIVersionWithin | the runtime CLI | `--version` in front of a launch |
| `seatbelt.go` sandboxApplyRefusal | `sandbox-exec` | can this process apply a profile at all |
| `seatbelt.go` worktreeTop, (git-dir read) | `git` | `rev-parse` |
| `skills.go` (worktree prefix) | `git` ×2 | `--show-prefix`, `--git-common-dir` |
| `skillsdirtrust.go` grokInspectOutput | `grok` | the dir-trust reading |
| `verifyafter.go` (commit search) | `git` | `log --grep <bead id>` |
| `worktree.go` gitIgnores | `git` | `check-ignore` |
| `worktree.go` patchIDsVerbatimWithin | `git` ×2 | the `log -p` → `patch-id` pipe pair |

Three of these were already right and are now reads of the one rule rather
than hand-typed copies of it: `runtimeprobe.go` and `skillsdirtrust.go` each
carried `cmd.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))`, and
`credential.go` reaches `/usr/bin/security` by an absolute constant on
purpose (ranger-base-ypf5 — a persona's `Bash(security:*)` shim had refused
posse's own monitoring read). `refresh.go` was the one site that asked
`exec.LookPath` whether the binary existed at all; it asks `resolveOutside`
now, and its refusal says "not on PATH outside posse's gates dirs", because a
shim answering on PATH is not the CLI being asked for.

### Not protocol — the seat's own environment, and the point

| site | why it keeps the wall |
|---|---|
| `cagelauncher.go` runCageSteps | the engine steps of a cage launch. `RunCageLaunch` **is** the pane's argv0 — the launcher symlink herdr identifies the session by — so this runs IN the seat: its environment is the one posse TYPED onto that seat (ADR 0077 D1/D2) plus the seat's own `GatePrefix`, and a seat's walls are its own PID's (ADR 0017). Reached from `StartCageEgress` and from `RunCageReap`, the watcher that launch forks. |
| `cagelauncher.go` watchCageEgress | that watcher, re-exec'ing THIS binary by the path `os.Executable` named — an absolute path, so there is no name for a shim to answer to. Same process, same reason. |

### Not in the census, and why

`syscall.Exec` is not `exec.Command`, and the four shipped uses of it are all
on the far side of this question: `cagelauncher.go` becoming the engine and
`cageinner.go` becoming the runtime inside the container are the seat;
`bdwriteshim.go` becoming `bd` IS the wall (ADR 0073 D2 — it holds the lock
it passes down on an inherited open file description); `cmd/posse/main.go`
becoming the operator's `$EDITOR` is the operator's own terminal.

**`exec.LookPath` is the named adjacent class. One member of it IS fixed
here; the rest are not, and the division is what the answer is USED for.**

Fixed: `cage.go`'s `WrapInCage` resolved the container engine with
`exec.LookPath` and wrote that path into the launch plan, which the SEAT then
execs (`cagelauncher.go`, `plan.Path`). A gates dir leading the LAUNCHER's
PATH would have baked the launching persona's engine shim into the hired
seat's plan — the seat exec'ing it with the launcher's rules and the
launcher's refusals log. The exec is the seat's and stays exempt; the
RESOLUTION is posse's, which is the other half D3 names ("and its binary is
resolved against it"). It takes `resolveOutside` now. UNMEASURED: no PID on
this box denies a container engine, so nothing renders a shim to be found.

Not fixed: nine probes that ask only whether a binary EXISTS —
`Bd.Available`, `Herdr.Available`, `SeatbeltAvailable`, `ContainerAvailable`
and the two other engine-binary reads in `cage.go` (`cageNoBinary`,
`BuildCageImage` — `WrapInCage`'s is the one above), `backup.go`'s tool
check, `runtimepreflight.go`'s. `Bd.Available` is filed as
**ranger-base-g1xnt** by ranger-base-cwsi8, which measured that it can now
answer true where `bin()` refuses; the rest of the class is the same shape. A
gates
shim is rendered for every command a PID denies, whether or not a real binary
stands behind it, so on a box with a shim and no real binary these answer
"available" about the shim. ranger-base-rzjie left `Bd.Available` alone with
the reasoning that "a rendered bd shim always stands in front of a real bd
`resolveOutside` can find", and that reasoning is sound for every binary this
box actually has — and cwsi8 measured how thin "actually has" is. The cost
of each being wrong is a WORD, though: a gap row, a degrade sentence, with no
exec following from the answer, which is the difference from `WrapInCage` and
the reason the class is named here rather than swept in.

Also not fixed, and worth its own line because the name IS shimmed on this
box: `possebinary.go`'s `RunningPosse` falls back to `exec.LookPath("posse")`
when `os.Executable` fails, and every crew PID denies `Bash(posse promote:*)`
and `Bash(posse refresh:*)`, so a `posse` shim is rendered in every gates
dir. The fallback would then name that shim as the running posse. It sits
behind `os.Executable`, which does not fail on darwin outside a deleted-binary
race, so this has never been reached — and `RenderGates` already resolves the
`posse` its bd arm calls back into with `resolveOutside` (gates.go), which is
the one place that answer is exec'd.

## The pin

`internal/treepins/protocolchildpath_qa_test.go` —
`TestQAEveryProtocolChildRunsOutsideTheGates`, doored by `make corpus-check`.
Two arms over the same corpus the WaitDelay census reads (`ewdGoFiles`):

1. every `exec.Command`/`exec.CommandContext` site is `protocolCmd`'s own
   argument, or its enclosing function is in `pcExemptFuncs` with the reason
   the table above gives;
2. every `.Env` assignment on a child one of those functions exec'd reaches
   an env binder. **This arm is the half this bead nearly shipped wrong.**
   `cmd.Env` REPLACES the environment, so a site wrapped in `protocolCmd` and
   then handed `append(os.Environ(), …)` two lines later has lost the PATH
   with the wrapper still on the line above saying otherwise —
   `cage.go`'s `BuildCageImage` assigns `.Env` three times after constructing
   its child. The env binders are DERIVED (any function returning a
   `[]string` that reaches `protocolEnv`, to a fixed point), so
   `bdChildEnv` — which adds ADR 0073's store binding first — counts without
   a roster row.

`TestQATheProtocolChildCensusCanStillSayNo` drives both arms over nine
planted sources, and `TestQATheEnvBinderDerivationFindsThisTreesBinders`
reads the derivation off THIS tree, since arm 2's quietness rests on it.

**And the wrapper broke the WaitDelay census, which is worth recording
because it is the general hazard of putting a choke point in front of
everything.** `ewdScan` collected its bindings from an `AssignStmt` whose
right-hand side was the `exec.CommandContext` call itself. With the wrapper
in front, `cmd := protocolCmd(exec.CommandContext(…))` bound nothing, every
wrapped site read as CHAINED — "no variable to set `cmd.WaitDelay` on" — and
`make corpus-check` reported **twelve sites** that each set it on the very
next line, plus its own "not one site was read as naming its grace" arm.
MEASURED 2026-10-31, and the fix is `ewdPassThrough`: a DERIVED set of
helpers that take an `*exec.Cmd` and hand one back, unwrapped once at the
binding. Three two-way arms were added for it, including the negative — a
helper that takes an `*exec.Cmd` and returns a bool has handed the caller no
child to set anything on.
