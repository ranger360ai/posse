# A seat's launch line is exec'd, so no shell outlives its runtime (ranger-base-1l1en)

ranger-base-1l1en, 2026-11-15. The implementation of the architecture call on
ranger-base-ie4mx. ADR 0080's amendment of the same date carries the decision,
the full measurement tables and what is lost; this fragment is the working
record beside it — the two things that cost the most to get right, and the
readings nobody will want to take twice.

Environment for every reading here: Linux 7.2.15-1-omarchy, 16 cores,
go1.27.3, GNU bash 5.3.20, rex 0.0.1794069975-g6c6689, herdr
installed, posse built from this worktree. Every live reading was taken
against a SCRATCH Rex server on a private socket
(`--restore=false --max-sessions 6`) and a scratch herdr server on its own
session socket. No fleet seat was touched, no model ran, and both scratch
servers and the scratch herdr session dir were removed afterwards
(`go run ./cmd/checkorphans`: clean).

## `exec` goes on the command word, and that is a measurement, not a taste

The obvious spelling is to prefix the line. It does not work, because every
launch line posse renders is a simple command behind a run of `NAME=VALUE`
assignments. MEASURED here, in bash 5.3.20 and in sh:

```
FOO=bar exec /usr/bin/env      -> env prints FOO=bar, and the pid is the shell's
exec FOO=bar /usr/bin/env      -> exec: FOO=bar: not found
```

Both halves of the first row matter and only one of them is obvious. The
shell IS replaced — so the pane has no interpreter left — and the assignment
is still exported to the program that replaced it. A prefix-spelled `exec`
would have failed loudly; an `exec` placed anywhere that dropped the
assignments would have launched seats with no gate PATH, no runtime home and
no credential dir, which is a quiet failure. There is a live row for exactly
that second half (`TestLiveTheAssignmentPrefixSurvivesTheExec`).

So the insertion point has to be found, which means reading enough shell to
know where the command word is. `shCommandWord` (internal/posse/launchexec.go)
is that reader and it is SOUND rather than complete: every `true` is a line it
took apart whole, and anything it cannot model — an unterminated quote, a
backquote, a construct it does not know — answers `false` and costs the caller
the `exec /bin/sh -c '<line>'` fallback, never a wrong insertion point. The one
arm it genuinely needs is `$( … )` nesting inside double quotes, because every
claude line carries `--append-system-prompt "$(cat '<agents>/<persona>.md')"`,
and the one shape that surprised me is the bwrap launch: `3<'…/bwrap.args'`
is a redirection in the MIDDLE of the command, so the word `exec` lands on a
word that is not the program. `exec 3<file /path/bwrap --args 3 -- claude` is
valid and does what it says; the pin row is there so the next reader does not
have to re-derive it.

## The one place the exec is applied, and the one `Start` it must not reach

`App.PaneLine` is the single home: all three launch sites go through it
(`startPlanned` and `RelaunchAgent` in herdrback.go, `RuntimeProbe` in
runtimeprobe.go), and `exec` inside a sourced file replaces the sourcing
shell, so the spilled form needs no second spelling.

The reason it is NOT in the multiplexer slot's `Start` verb is the fourth
caller: `probeSessionExe` types `command -v <exe> > <part>; mv <part> <file>`
into the probe pane to record which binary that session resolves
(ranger-base-385x). It is compound by design, it is the pane's own shell that
must answer it, and that same shell has to survive to run the launch line
after it. An exec in `Start` would have exec'd the lookup and left the probe
with no shell to launch into.

That is one roster entry apart from three, with nothing in the source saying
so, which is why `internal/treepins/launchexec_qa_test.go` is a census and not
a comment: every two-argument `Start(seat, line)` in the tree is on a roster
with its kind, a launch entry must name `PaneLine` in the same function, and
the lookup entry must not. A fifth call site reds it and its author says which
of the two it is. (Two arguments is the discriminator because `exec.Cmd.Start()`
takes none — that is every other `.Start(` in the tree.)

## The gate prefix marker had to absorb the exec, and here is why that is not hiding it

`gatePrefixRe` (internal/posse/herdr_test.go) collapses the whole launcher
preamble to `GATES ` so that a pin whose subject is the claude line does not
have to spell PATH=, the runtime-home pin's two arms or the transcript
ceiling's two. The exec does not land in FRONT of that preamble, it lands
inside it — before whichever `/usr/bin/env -u` arm those pins chose, or
immediately before the CLI when neither chose one. Three positions, all of
them the same fact about the launcher.

Spelled at each assertion instead, fifteen pins would have read
`GATES exec /usr/bin/env -u CLAUDE_CODE_SKIP_PROMPT_HISTORY claude` where they
now read `GATES claude`. So the expression swallows it, exactly as it swallows
the two pins beside it, and the exec's own pins read the RAW line — which is
the rule that file already states for those two.

## The part that would have taken the fleet down

Worth its own section because it was found by reading rather than by a red
test, and because the exec is wrong without it.

A runtime that replaced the block's shell INHERITS ITS PID. So on Rex it is
the block's `child`, and `child == foreground` with no `ancestors` — the
exact process-table shape a shell at its own prompt has. ADR 0076 §3's
"owner must not be the pane's shell" guard was `owner == process.child` and
nothing else. With the exec in place and nothing else changed, every
reporting runtime would have read as *its owner is the block's own shell* →
`MuxUnknown`: no launch confirms, every seat lists as unreadable, dispatch
can never prompt one. Not a degradation — a stop.

MEASURED here, two blocks on the scratch server, one sent the line with
`exec` and one without:

```
exec'd      child == foreground == {2938494,"sleep","/usr/bin/sleep"}, no ancestors
not exec'd  child {2938568,"bash"}, foreground {2939221,"sleep"}, ancestors [the bash]
```

The guard asks a second question now: is that child the shell posse STARTS a
block with (`rexShell`, the path it handed `rex new`)? `RexProc` reads
`path` for that and for nothing else — identity stays pid AND name, which is
Rex's own rule. Deliberately not a list of shell names: a closed vocabulary
of "things that are shells" rots on the first box that resolves a third one,
and what posse knows for certain is which shell it started.

The fixtures had the same bug in mirror image. They spelled the block's own
shell as the literal `zsh` — the shell of the box the shape was first
measured on — so six files' worth of them now ask `rexShell()` the way the
predicate does. Which is the better fixture anyway: it says "the shell posse
starts a block with" rather than naming one.

**And the same class is open on the herdr route** (ranger-base-m5rdy, filed,
not fixed here). `AtShellPrompt` reads "every foreground pid IS the pane's own
shell pid", and MEASURED on a scratch herdr server the exec'd row is
byte-identical to the shell-at-its-prompt row in the two fields posse parses:

```
before anything typed   shell_pid 3854566, foreground [{3854566, "bash"}]
exec'd                  shell_pid 3854566, foreground [{3854566, "sleep"}]
not exec'd              shell_pid 3854572, foreground [{3854782, "sleep"}]
```

The Rex answer does not transfer, which is why it is a bead and not a line in
this commit: posse CHOOSES the Rex block's shell and can compare against it,
and it does not choose herdr's. What herdr reports and posse discards is each
foreground process's `name`, `argv` and `cmdline`.

## Readings worth not taking twice

The tables are in the ADR amendment. Three of them are worth a second mention
because they were the ones in doubt:

- **A closed block takes its scrollback with it.** `rex capture` of the block
  a finished exec'd program left answers `block target "<id>" not found`. That
  is what `posse peek` of a seat that died reads now, and the thing the
  amendment's "what is lost" section is about.
- **D10's destroy keeps up.** One `posse list` over a scratch home whose seat
  had ended and whose meta was backdated past `PruneGrace`: the seat left the
  listing, `rex ls` lost the label, the server went `sessions: 3` → `2`, and
  the meta was unlinked. The state D10 was measured on and the state an exec'd
  seat ends in are the same state, which is why no new mechanism was needed —
  what changed is that it is now the ONLY way a seat ends.
- **herdr closes the pane too** (this was NOT MEASURED when the work was
  handed over). `herdr pane run w1:p1 "exec …"`: when the program ends the
  pane answers `pane_not_found` and the one-pane workspace is gone from
  `workspace list`, while the same line without `exec` leaves the workspace
  listed with its shell at a prompt. A dead herdr workspace is gone, so there
  is no husk and posse's prune reads it through the empty-listing arm it
  always had.

## Mutation-checked

Each mutant was one edit over an otherwise unmodified tree, with the file
restored from a byte copy afterwards and the build re-checked:

| mutant | what reds |
|---|---|
| `ExecLaunchLine` returns `cmd` unchanged | all four behavioural arms in internal/posse (every built-in runtime's create line, the re-typed line, the spilled script, and the unit rows) |
| `ExecLaunchLine` returns `cmd + "; exit"` | the live pty pin's `kill -INT $$` row ALONE — the other two rows stay green, which is the 2026-11-14 measurement reproduced |
| the roster loses its `probeSessionExe` entry | the treepins census, on both halves at once (an unrostered site, and a roster entry the tree no longer has) |
| that entry's kind flipped to `launch` | the treepins census's second arm |
| `rexOwnerIsShell` back to `owner == child` alone | the new §3 reading pin ALONE — the shell's-own-record pin beside it stays green, so the pair can tell the two apart |

One thing I did to myself, recorded so the next person does not: the first
mutation was planted while a `make test-arm1` was running in the background,
so cmd/posse's tests reported 20 failures that were all
`declared and not used: off` from the mutant. Do not mutate a tree a suite is
reading.

## Not closed here

- ranger-base-pfcci
- ranger-base-ybrk6
- ranger-base-m5rdy
