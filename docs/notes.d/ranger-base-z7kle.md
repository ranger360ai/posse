# posse's OSC 7501 reporter for codex, and what the route actually does (ranger-base-z7kle)

2026-11-06. The implementation slice of ADR 0076 Amendment 2 D6 (design bead
ranger-base-dok0a): a hidden posse subcommand that codex runs as a hook and
writes one OSC 7501 report per event, plus the `-c hooks={…}` declaration that
installs it. Code: `internal/posse/codexreport.go`, its unit pins next to it,
the fourth entry point in `cmd/posse/main.go`, and the acceptance probe
`docs/adr/0076-reporter.probe.sh`.

Everything below was taken on this box on 2026-11-06, from a `cage: seatbelt`
crew seat, with **Rex CLI 0.0.1793377904**, **codex-cli 0.153.4**, posse
`0.5.3+dev`. No model turn was made, and the three locks that made one
impossible are in the probe's header.

## `state=clear` is real, and it removes the record

D6's table maps SessionEnd to `state=clear`, which is not one of the five
states the protocol advertises. It is not a sixth state either.

**MEASURED** by hand, `printf` into a `rex new` block, reading
`block.method program_status` after each report:

| wrote | `program_status` after |
|---|---|
| `state=working:app=z7probe` | one record, `working`, `owner={76588,"bash"}` |
| `state=clear:app=z7probe` | `{"records":[]}` |
| `state=working` again | the record is back |
| `state=done` | one record, `done` |
| `state=clear` | `{"records":[]}` |
| `state=idle` | one record, `idle` |
| `state=` (empty) | **unchanged** — the whole report is dropped |

**STATIC**, same Rex binary: the OSC 7501 keyword table holds `idle done error
working blocked`, `state`, `title`, and no `clear`; the `7501;?` feature reply
advertises `states=idle,working,done,blocked,error`. So `clear` is undocumented
in the spec revision ADR 0076 read (0.4) and implemented anyway, and it is the
right thing for SessionEnd: the record goes away with the session instead of
sitting at a stale `idle` for whatever reads the block next. Rex's own
`program_status_removed` event is the push side of the same thing.

## codex's hook `command` is split on whitespace, with no shell

This is the one question the renderer had to answer before it could name
posse's own binary, and the refusal that used to stand in its place is gone.

**MEASURED**, probe arm `space`, a handler at `…/a dir/hook.sh` declared two
ways, each with its own scratch home and its own `hooks/list` trust:

| `command=` | result |
|---|---|
| `/…/a dir/hook.sh /…` (bare) | the turn printed `hook exited with code 127`; the hook never ran, the witness file stayed empty, and `program_status` stayed `{"records":[]}` |
| `'/…/a dir/hook.sh' '/…'` (single-quoted) | the hook RAN, twice (SessionStart and UserPromptSubmit), with `$0` the whole path, space included |

So the split honours single quotes and runs nothing through a shell — 127 is
`exec` failing on the first whitespace-delimited word. `codexHookCommand`
therefore quotes the path, always, one spelling; the verb and the app name are
plain tokens by construction and are left bare. What is still UNMEASURED is
refused rather than guessed at (a single quote, a double quote, a backslash, a
control byte in posse's own path), and each refusal names this arm as the
re-measurement that would lift it. A `'` would need the close-escape-reopen
idiom, which is a POSIX-shell rule, and what codex splits with is not a POSIX
shell.

The first run of this arm measured nothing: the probe's own
`-c '<declaration>'` spelling was eaten by the single quotes the arm
deliberately puts in the declaration, and codex answered `Usage: codex`. The
probe has a `shq` now.

## The acceptance: codex accepts posse's declaration and reports as codex

**MEASURED**, probe arm `codex`, twice — once with the bare handler and once
with the shipped quoted one, same result both times:

- `codex app-server`'s `hooks/list` answered in 0.1–0.3 s and reported **all
  eight** of posse's hooks, every one `source=sessionFlags`,
  `managed=false`, keyed `/<session-flags>/config.toml:<event>:0:0` — one
  group, one handler, for each of the eight events. That is codex's own parser
  accepting the string `posse hook-report --declare codex` printed, which is a
  better answer to "is it TOML codex takes" than any library gives.
- The launch drew **no review dialog**, and the record at t+2 s read
  `state=working`, `app=codex`, `owner={pid,"codex"}` — posse's binary wrote
  it, codex's own event caused it, and Rex stamped the tty's foreground
  program. That is the positive evidence D9 widens `ReportsOnRex` to admit.
- The scratch home's `config.toml` was **byte-identical** before and after
  (sha256 unchanged). No file was written anywhere.

## Two things the acceptance could NOT reach, for the next slice

1. **A record that settles.** The prompt was submitted (hence `working`) and
   the turn then failed on the missing credential the lock relies on. The
   record stayed `working` for the whole 60 s observed; no Stop or Interrupt
   fired. Whether codex fires `Stop` on a turn that errors before it starts,
   and what the record reads after a turn that succeeds, are both UNPROVEN and
   need a live turn — ranger-base-8gh58, whose done-when is the runtime one.
2. **`blocked`.** Still UNPROVEN live (D4 item 5), and still moot under
   `-a never`.

## A caged codex seat needs the IMAGE's posse, not this host's

`CodexHooksDeclaration` takes the handler path as a parameter rather than
reading `RunningPosse().Exe` itself, and this is why. On a container seat
(ADR 0002 §3 L4) the host's absolute path does not exist inside the cage: the
image carries its own Linux posse at **`/usr/local/bin/posse`**
(`etc/cage/Dockerfile`, `COPY bin/ /usr/local/bin/`), which is the same binary
`posse gates wrap` is reached by in there. A declaration rendered from the
host's `os.Executable` and typed onto a caged codex line would give a handler
that cannot exec — and what that looks like from outside is the 127 above: one
line inside the turn, no record, and a seat that reads as a runtime that went
silent. Whoever types the line decides which path to hand this function
(ranger-base-8gh58); the renderer refuses to guess, and nothing in this slice
assumes the host's.

## Two facts about running this from a caged seat

- **Nested seatbelt is refused.** `sandbox-exec -p '(version 1)(allow
  default)(deny network*)' /bin/echo` from a `cage: seatbelt` crew seat is
  `sandbox-exec: sandbox_apply: Operation not permitted`, whatever the
  profile. The same command inside a **Rex block** runs clean, because Rex
  spawns the block outside this seat's cage — so the probe's no-turn network
  lock goes on the codex that takes a prompt, and the `hooks/list` ask (which
  submits nothing) runs unwrapped. ADR 0076's sibling probe wraps that ask;
  from a caged seat that wrapper stops the arm rather than protecting it.
- **`/dev/tty` works from inside the cage.** ADR 0076 Amendment 2 lists it
  ASSUMED; the `bytes` arm wrote every one of D6's reports through it and read
  each one back as a record. MEASURED.

## The `bytes` arm's records carry no owner, and that is the arm's shape

Every report in the `bytes` arm landed with the right `state`, `kind` and
`app`, exit 0 each time, and **no `owner` field at all** — the reporter there
is a short-lived child in a pipeline the interactive shell has already
finished waiting on, so Rex has no foreground program to stamp. An unowned
record is one ADR 0076 §3 refuses for a seat, and rightly. The arm is about
the bytes becoming the record; the `codex` arm is the one that answers the
owner, and it answers `codex`.
