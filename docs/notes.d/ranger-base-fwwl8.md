# D7's trust ask, and the `-c` order that decides whether any of it works (ranger-base-fwwl8)

2026-11-06. The implementation slice of ADR 0076 Amendment 2 D7 (design bead
ranger-base-dok0a): posse asks `codex app-server` which hooks would load on a
seat's line, and renders the one `-c hooks.state={…}` that trusts its own
reporter hooks and switches every other one off. Code:
`internal/posse/codextrust.go`, its unit pins next to it, the `--ask` arm of
the hidden `hook-report` verb in `internal/posse/codexreport.go`, and the
acceptance probe `docs/adr/0076-trust-ask.probe.sh`.

Everything below was taken on this box on 2026-11-06, from a dispatched
`cage: seatbelt` seat, with **Rex CLI 0.0.1793377904-ge6592f**, **codex-cli
0.153.4**, posse `0.5.3+dev`. No model turn was made: every codex ran in a
scratch `CODEX_HOME` holding no credential, whose only provider is a dead
loopback port and whose `env_key` names a variable that is never set.

## The `-c` order is load-bearing, and the wrong one looks like nothing

This is the finding the design did not have, and it is the one that decides
whether the whole route works. **`hooks={…}` must be typed BEFORE
`hooks.state={…}`.** codex applies `-c` overrides in order, and `hooks={…}`
sets the whole `hooks` table — including the `state` subtable underneath it —
so a trust value that arrives first is wiped by the declaration that follows.

**MEASURED**, probe arm `order`: the same two values, both ways round, one
scratch home, read back through codex's own `hooks/list`.

| order on the line | posse's eight hooks | the user-layer hook |
|---|---|---|
| `-c hooks={…}` then `-c hooks.state={…}` | `trusted`, `enabled=true` | `trusted`, **`enabled=false`** |
| `-c hooks.state={…}` then `-c hooks={…}` | `untrusted`, `enabled=true` | `untrusted`, `enabled=true` |

The second row is a line carrying every byte of the right answer, a launch
that completes, and a codex seat sitting on the startup review dialog with no
record and no turn — the exact failure D7 exists to prevent. So the ask puts
the declaration first in its own argv as well, which is what makes a round
trip through `posse hook-report --ask codex -c <the state it printed>` ask
codex the same question a launch does. `ranger-base-8gh58` types the line and
this is the row it needs.

A second, smaller fact from the same arm: a declaration rendered by a posse at
a DIFFERENT path reads back `trustStatus: "modified"` rather than `untrusted`.
So `modified` is a third value of that field, the hash really does bind to the
declaration, and a trust line built against one posse binary is worthless for
another. (It is what you see if you render the declaration with `go run` and
the trust with a built binary, which is how it was found.)

## What `hooks/list` actually answers, and the notification in the middle

**MEASURED**, the real `codex app-server` under posse's own declaration, four
runs:

- The whole three-message conversation — `initialize`, the `initialized`
  notification, `hooks/list {cwds:[<cwd>]}` — answered in **0.41 s** wall.
  ADR 0076's own runs read 0.1-0.3 s for `hooks/list` alone.
- The answer is `result.data[]`, one entry per asked cwd, each
  `{cwd, hooks[], errors[], warnings[]}`. Both `errors` and `warnings` were
  empty in every run here. A hook declaration that FAILED to load is absent
  from `hooks[]`, so those two arrays are the only account of it there will
  be — which is why `CodexHooksAsk.Log` prints them and why a launch is worth
  pointing at its own log.
- Nine rows under posse's declaration plus one user-layer `hooks.json`:
  posse's eight as `source: "sessionFlags"`, `isManaged: false`, keyed
  `/<session-flags>/config.toml:<snake_case event>:0:0`, and the user's as
  `source: "user"` keyed by its own path. Every row `untrusted` before trust
  was typed.
- **An id-less notification arrives BETWEEN the two answers**:
  `remoteControl/status/changed` (`status: "disabled"`). A reader that takes
  the next line as its answer reads this and calls the answer malformed. The
  reference client in `0076-flag-trust.probe.sh` skips by id and posse's does
  too; the unit pin
  `TestCodexHooksAskSkipsEveryMessageThatIsNotItsAnswer` holds it, and the
  mutant that takes the first message instead reds every arm in the file.
- `command` comes back **byte-identical** to what posse typed, single quotes
  and all. That is what makes the drift guard honest: posse compares codex's
  echo with `codexHookCommand`'s output, and an answer about a different line
  refuses before that line is typed.

## codex is its own acceptance test, with no Rex and no prompt

Probe arm `state`. Ask once, then ask AGAIN with the rendered trust on the
line, and read each hook's `trustStatus` out of codex both times. Round 1: all
nine `untrusted`. Round 2: all nine `trusted`, the user-layer hook
`enabled=false`. The home's `config.toml` sha256 was byte-identical before and
after, both rounds — nothing was written anywhere.

That is the cheapest check this route has, it needs neither Rex nor a turn,
and it is the first thing to run when a codex seat stalls.

## The live arm

Probe arm `ask`, a Rex block, the line posse would type (declaration, then
trust), with an untrusted user-layer `hooks.json` sitting in the home:

- review dialog: **no**
- the user-layer hook: **silent** — its witness file was never written
- the record at t+2 s: `state=working`, `app=codex`,
  `owner={94510,"codex"}`, and the same at t+5, 10, 20, 40 and 60 s
- the home's `config.toml` sha256: unchanged

The record staying `working` for the whole 60 s is z7kle's open finding again,
not a new one: the turn died on the missing credential the no-turn lock relies
on, and codex fired no Stop and no Interrupt. What a record does after a turn
that ENDS is still UNPROVEN and is `ranger-base-8gh58`'s.

## Two things the unit pins learned the hard way

**A fake app-server, not a fake reader.** The thing under test is a
conversation with a child process and a string that goes on a command line, so
every arm starts a real child — a shell script that speaks the protocol — and
the rows are the script's own answer. The seam is the BINARY
(`CodexHooksAsk.Bin`), which is the seam the probe uses too. A test that
injected the rows would pin the rendering and nothing about the argv, the
environment, the order, the skipping or the kill.

**Ending a child is two readings, and the first cut had a flake.** A leaked
app-server can be STILL RUNNING or killed-and-never-reaped, and neither
reading sees the other: the pin's tick file catches the first, `kill -0`
catches the second, and the mutant that drops `cmd.Wait()` while leaving the
deferred cancel in place is exactly the second — an arm with only the tick
passes over it. The flake was separate and worth recording: the fake wrote its
own pid AFTER printing the answer, posse cancels as soon as it has read that
answer, and on a loaded box the SIGKILL outran the fake's bookkeeping. The arm
then failed on its own precondition, with a sentence about the fixture, over a
tree that was correct — once in about twenty-five runs of the loaded suite and
never once in 16 solo runs. The pid and the first tick are written at startup
now, before posse has anything to cancel.

## Mutation-checked

Twelve mutants, each against the arms it should red, 2026-11-06:

| mutant | reds |
|---|---|
| declaration appended last instead of first | the argv/cwd/home arm alone |
| foreign hooks not disabled (`off := ""`) | rendering cases 2 and 3 alone |
| the managed refusal switched off | the managed arm alone |
| the CODEX_HOME arm dropped from the child's env | both home arms alone |
| await takes the first message it sees | every arm in the file |
| `cmd.Wait()` dropped (the deferred cancel left) | the kill arm's ZOMBIE reading |
| the session-flags count guard dropped | the three drift arms alone |
| the `command` equality guard dropped | "a session-flags hook posse did not declare" alone |
| the quote/backslash refusal dropped | the four key/hash-byte arms alone |
| the duplicate-key refusal dropped | "the same key twice" alone |
| the empty-hash refusal dropped | "a hook with no hash" alone |
| the absolute-cwd refusal dropped | "a relative seat cwd" alone |

## Not done here

The LAUNCH. `ranger-base-8gh58` types the line, and this slice deliberately
hands it a pair rather than a renderer to call twice: `CodexHooksFlags` holds
the declaration and the trust together, because trust binds to the hash of the
declaration asked about, and two call sites rendering the same string is how
they come to render different ones. The caged case is unchanged from z7kle: a
container seat runs the image's Linux posse at `/usr/local/bin/posse`, so
whoever types the line hands that path to `CodexHooksAsk.Exe` and not
`os.Executable`'s answer.
