# An exec'd runtime holds the pane's shell pid, so the herdr liveness reading asks for its executable too (ranger-base-m5rdy)

2026-11-15. The herdr half of the class ranger-base-1l1en opened and fixed on
Rex: every launch line is `exec`'d now, so a seat's runtime REPLACES the
pane's shell and inherits its pid, and "every foreground pid IS the pane's own
shell pid" stopped being a reading about shells.

The code is `PaneProcesses.AtShellPrompt` and `paneShellBase`
(internal/posse/reportedagent.go); the arms are
internal/posse/reportedreading_qa_test.go (the predicate, and the resolver)
and internal/posse/promptstalelabel_qa_test.go (the delivery verb).

## The venue

MEASURED 2026-11-15 on this box: Linux 7.2.15-1-omarchy, 16 cores, go1.27.3,
GNU bash 5.3.20, herdr 0.9.1 (client protocol 22, server private protocol 22),
posse built from this worktree. This is a `multiplexer: rex` instance and no
herdr server of the fleet's was touched: each reading was taken against a
SCRATCH herdr server on its own session socket
(`herdr --session m5rdy-qa[2] server`, `HERDR_SOCKET_PATH` pointed at it),
with scratch workspaces under a `mktemp -d` cwd, all closed and the session
dir removed afterwards. `go run ./cmd/checkorphans`: clean.

## The three rows, with the field posse was not parsing

```
before anything typed   shell_pid 1619869, fg [{pid 1619869, name bash, argv ["/bin/bash"]}]
exec'd                  shell_pid 1619869, fg [{pid 1619869, name sleep, argv ["/usr/bin/sleep","300"]}]
not exec'd              shell_pid 1619876, fg [{pid 1620906, name sleep, argv ["/usr/bin/sleep","300"]}]
```

The first two rows are byte-identical in `shell_pid` and in every foreground
`pid`, which were the only two fields `PaneProcessInfo` parsed — so
`AtShellPrompt` answered TRUE over a live runtime. On a `detection: reported`
seat (ADR 0061, the only route that takes this reading) every typed prompt
would then be refused `agent_not_ready … is a <x> pane herdr labels but does
not detect`, with dispatch correctly handing the claim back, for as long as
the seat lived — and the sentence would blame a stale label.

The full `foreground_processes` row carries `pid`, `name`, `argv0`, `argv`,
`cmdline` and `cwd`, and `process_info` carries `shell_pid`,
`foreground_process_group_id` and `tty` (herdr 0.9.1's own
`herdr api schema --json`). `foreground_process_group_id` equals `shell_pid`
in BOTH of the first two rows, so it is no discriminator either.

## Why `name` and not `argv0`

ADR 0061 rejected "liveness by matching argv against the label" for two
reasons, and only one of them is about interpreters. The other is that herdr
DETECTS from argv0, so argv0 is not independent evidence about a label derived
from it — MEASURED 2026-09-28 (ranger-base-mx5x9 #1): `(exec -a codex sleep
25)` reads `argv0 "codex"` beside `name "sleep"`, and `codex` is the label
herdr gave the pane.

`name` is the executable the kernel has. No reporter writes it, no label is
derived from it, and an `exec -a` cannot move it. So the reading added the one
field of that row that answers the question without touching the field the
rejection was about.

## What it compares against, and the honest limit

On Rex the second question is "is that process the shell posse STARTS a block
with" (`rexProcIsBlockShell`, ranger-base-1l1en) — posse chooses that shell
and can compare against it. **posse does not choose herdr's pane shell**, and
herdr exposes none to ask: `pane process-info` carries `shell_pid` and no
shell name, `pane get` carries none (its `terminal_title` is the shell's
prompt string and ranger-base-mx5x9 #3 measured it contradicting the label it
sits beside), and there is no `herdr config get` at all — herdr 0.9.1's
`config` subcommands are `check` and `reset-keys`.

So `paneShellBase` is `realShell("")`'s basename: the same resolver `rexShell`
hands `rex new` and the gate wrapper is rendered from. The BASENAME, because
that is the disagreement that actually happens — herdr started this pane's
shell as `/bin/bash` while realShell answers `/usr/bin/bash` on the same box,
and both are `bash`.

It is a GUESS, and the direction it fails in is the whole reason it is
allowed to be one:

- herdr's pane shell is a shell this resolver does not name (a third shell, or
  a `$SHELL` the herdr server has and this process does not): a bare prompt
  reads as "something else is in the foreground", the guard stays quiet, and
  the label stands on its own exactly as it did before ranger-base-j687x. That
  is the pre-guard state, not a stop, and it is the direction every reading in
  reportedagent.go fails in.
- the other direction — a live seat refused forever — is the bug this bead is,
  and it is now reachable only by a launch line whose command word execs into
  a process the box also resolves as its shell.

## That residual, measured closed

The nearest thing posse renders is the `exec /bin/sh -c '<line>'` compound
fallback (launchexec.go), on a box whose realShell answers `/bin/sh`:

```
exec /bin/sh -c 'sleep 300; true'
  → fg [{pid 2268653 (== shell_pid), name sh}, {pid 2269534, name sleep}]
exec /bin/bash -c 'sleep 300'
  → fg [{pid 2272147 (== shell_pid), name sleep}]
FOO=bar exec /usr/bin/env -u NOPE /usr/bin/sleep 300
  → fg [{pid 2270681 (== shell_pid), name sleep}]
```

Row 1 is the fallback and it does not reach the hazard: `sh` is not
interactive, so it forks the list into its own process group and herdr reports
BOTH rows — the predicate answers false on the second. Row 2 is the same shape
one step further: bash execs a lone simple command, so not even the wrapper
survives. Row 3 is what a real launch line looks like, assignments and `env
-u` and all: `env` execs through, and the pane's one foreground row is the
runtime's own executable.

## The ADR line that no longer holds

ADR 0061 Claims fact 2 and D3.2's rationale say the reading is "a pid compared
to a pid, no argv matched" and that "at a bare prompt the only foreground
process IS the shell (pid == shell_pid); under any CLI it is not". The second
half of that sentence is false after ranger-base-1l1en, by the table above.
The record is the architecture lane's to amend and is filed — the code, its
comments and these arms carry the measurement meanwhile.

## Mutation-checked

Each mutant was one edit over an otherwise unmodified tree, with the file
restored from a byte copy afterwards and the build re-checked:

| mutant | what reds |
|---|---|
| `AtShellPrompt` back to the pid comparison alone | the two new arms ALONE — the predicate's exec'd row, and the delivery verb's concession — with every pre-existing arm green, which is the pair telling the two shapes apart |
| `paneShellBase` returns a shell no row can match | nothing, and that is the fixtures' own trade: `atShell` asks the predicate's resolver (ranger-base-1l1en's fixture lesson), so no fixture can see the resolver answer wrongly. The arm that holds what MUST not drift is in reportedreading_qa_test.go — an empty or path-spelled basename is a guard that cannot fire on any box, and that reds |

## Not closed here

Nothing of this bead's. The complete answer to "which shell is in this pane"
is one of ADR 0061 D4(b)'s upstream asks (scope a reported label to the
foreground process group), and a posse-side one would be to read the pane's
shell at create and keep it on the seat's record — neither is this fix, and
the guess above is honest about being one.
