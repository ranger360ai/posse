# "Enabled" is not "running": the herdr-bob watcher had never started on this box (ranger-base-mz8ud)

*2026-10-03 · gilfoyle · bead ranger-base-mz8ud, discovered-from
ranger-base-5jjtn · the Bob-side half of the same launch failure is
ranger-base-f1ytb · ADR 0060 D2 (the detection route is upstream's; posse ships
the filing, the operator sends it), ADR 0061 D3.2/D4*

Environment for every MEASURED line below: macOS 26.4.1 / darwin 25.4.0, arm64 ·
herdr 0.9.1 client and server · bob 2.0.5, node 25.2.1 · herdr-bob
`@e53bd475` installed by `herdr plugin install`, `platforms = ["linux","macos"]`,
and `herdr plugin list` → **enabled** throughout · `/bin/bash` 3.2.57, the only
bash on the box · jq 1.8.1 · perl 5 at `/usr/bin/perl`.

## What was wrong

`bin/bob-watch` had never executed on this install. The evidence is the
plugin's own log, `~/.local/state/herdr/plugins/mloeper.herdr-bob/herdr-bob.log`:
**zero** `watch started` lines, **zero** `adopted hand-started bob` lines, for
the whole life of the install. Every line in it is `bob-hook` or
`bob-watchctl`'s synchronous `readopt`/`reap`, which herdr runs in the
foreground itself. `watch.pid` held a pid that `kill -0` says does not exist.

That is why no Bob pane is ever labelled without a typed prompt.
`adopt_unclaimed` is the only route that labels a pane with *no* prompt at all —
Bob fires `SessionStart` on the first prompt — and it lives in the process that
never ran.

Four darwin bugs, MEASURED 2026-10-02/03, all upstream's code:

| # | site | darwin says |
|---|---|---|
| 1 | `bin/bob-watchctl` start: `setsid nohup "$HERDR_BOB_BASH" bob-watch &` | `command -v setsid` → exit 1. util-linux; not on darwin |
| 2 | `lib/common.sh` `log()`: `date -Is` | `date: invalid argument 's' for -I`, exit 1 |
| 3 | `bin/bob-watch` `heartbeat_fresh()`: `stat -c %Y` | `stat: illegal option -- c` |
| 4 | `bin/bob-watch`: `declare -A last_state=()` | `declare: -A: invalid option` on bash 3.2 |

### 1 is the one that made it permanent

The subshell exits 127, `echo $!` records it, and `start` prints `herdr-bob:
watcher started (pid NNNNN)` over a process that does not exist — so
`running()` reads false forever. The only caller is the manifest's
`[[startup]]` hook: one-shot, unsupervised, nothing retries. **A single failed
start lasts until the herdr server restarts, and then fails again
identically.** Restarting the server would not have helped.

### 2 is why the log cannot be dated

The substitution yields the empty string, so every line the plugin has ever
written begins with a space and carries no time. darwin's `date` *does* take
`-I`, but not an abbreviated `FMT`; `date -Iseconds` is accepted by both GNU
and darwin (MEASURED: `2026-10-03T00:32:13-04:00`).

### 3 would have kept the hook bridge from ever owning idle/working

`|| echo 0` makes `age` the current epoch, so the heartbeat always reads stale
and the watcher never defers. The two spellings **cannot be probed in either
order**: on GNU, `stat -f %m` is a *filesystem* query that succeeds and prints a
mount point, so the BSD arm may only run after `-c` has failed — and the answer
needs a digit check before arithmetic sees it.

### 4 was not in the bead, and it is the one that would have survived a fix to 1

On bash 3.2 a string subscript is evaluated as **arithmetic**, so under
`set -u` the first *read* of `last_state[$pane]` — `w2KJ:p1` → the unset
variable `w2KJ` → `unbound variable` — exits the shell. That read is in the main
loop, reached for every alive registered pane with a non-empty screen.

MEASURED: with the detach fixed and bash 3.2 left in place, the watcher adopts
and labels the pane (`adopt_unclaimed` runs *before* the pane loop) and then
dies in the same cycle, on the first screen read. Its stderr is on `/dev/null`,
it writes no log line on the way out, and **its exit status is whatever the
previous command left** — 0 in a bare reproduction, 1 in bob-watch's loop, where
`match_rules` has just declined the screen. Nothing anywhere can tell that from
a clean stop.

`HERDR_BOB_BASH` is `command -v bash`, which on a stock darwin is `/bin/bash`;
the generated hook launcher on this box has `#!/bin/bash` baked in, which is the
resolution recorded in writing.

## The fix, and why it is shaped this way

`etc/herdr/herdr-bob/darwin-portability.patch`, cut against `e53bd475`:

- **1** keeps the `setsid` arm where it exists and falls back to perl's
  `POSIX::setsid` (darwin base system, pid preserved across `exec`, so `$!`
  stays correct), then to a plain `nohup`.
- **2** `date -Iseconds`.
- **3** a `file_mtime` helper in `lib/common.sh`, GNU arm first, digit-checked.
- **4** drops the bash-4 dependency instead of demanding a newer bash. There is
  no bash ≥ 4 on this box at all (MEASURED: nothing at
  `/opt/homebrew/bin/bash`, `/usr/local/bin/bash`, `/opt/pkg/bin/bash`,
  `/usr/bin/bash`; no `bash` keg in Homebrew), and `platforms = ["macos"]`
  should not mean "macos plus Homebrew bash". `last_state` only ever held
  `"blocked"` or `""` — a set — and herdr pane ids carry no whitespace, so a
  space-delimited string and three functions replace it with no behaviour
  change. The alternative, resolving a bash ≥ 4 and refusing out loud without
  one, is named in the filing; `brew install bash` was rejected because it
  moves `command -v bash` for every `#!/usr/bin/env bash` script on this box,
  which is a box-wide substrate change to fix one plugin.

## How it is verified

`scripts/verify-herdr-bob.sh` — `make verify-herdr-bob`, ~17s. It copies the
installed checkout twice into a temp dir and drives both copies under a fake
`herdr` with scratch state and config dirs. **Nothing in it reaches the live
server, the live plugin state, or a real Bob.**

The control arm is the point. A check that only ran the patched tree could not
tell a fix from a harness that never reproduced the bug — the lesson of
ranger-base-26y03 — so the pristine tree must fail all four ways before the
patched tree is allowed to pass:

```
control (pristine e53bd475, darwin 25.4.0):
  ok   bug 1: start reported "watcher started (pid 74321)" and pid 74321 is already dead
  ok   bug 4: bob-watch died on the first screen read (rc=1, "unbound variable"), reporting nothing
  ok   bug 4: 'declare -A' is an invalid option on 3.2.57(1)-release
  ok   bug 2: every pristine log line is untimestamped (date -Is failed)
  ok   bug 3: the pristine tree has no portable mtime helper (stat -c %Y inline)
patched (+ etc/herdr/herdr-bob/darwin-portability.patch):
  ok   bug 1: start left a live watcher, pid 75189 (pidfile agrees)
  ok   adoption: an unclaimed pane running Bob was adopted with no prompt typed
  ok   adoption: reported state idle through pane report-agent
  ok   adoption: reported the display name
  ok   bug 2: every log line carries an ISO timestamp (2026-10-03T00:45:43-04:00)
  ok   bug 4: blocked reported from the screen (the read that killed bash 3.2)
  ok   bug 4: blocked reported ONCE, not once per tick (the set remembers)
  ok   bug 3: a fresh heartbeat read as FRESH, so the hook keeps idle/working
  ok   bug 4: blocked fires again after the set was cleared
  ok   bug 3: a stale heartbeat read as STALE, so the watcher falls back to idle
  ok   release: a vanished pane is released and forgotten
  ok   stop: the watcher stopped
installer (scripts/herdr-bob-install.sh, scratch XDG):
  ok   control: with no HERDR_PLUGIN_*, the plugin resolves the fallback roots (herdr-bob)
  ok   --check reports the PLUGIN state root, read-only
  ok   --check reports the PLUGIN config root
  ok   --start left the pidfile under the PLUGIN state root (pid 19460), not the fallback
  ok   --start wrote nothing under the fallback root .../xdg-state/herdr-bob
  ok   --start's watcher logged the PLUGIN config root's rules, so it reads the installed override
  ok   --start PRINTS the three roots it chose (a root chosen in silence is this bead's shape)
  ok   --check warns about a fallback-root watcher and names the stop command
  ok   --start REFUSES rather than make a second watcher
verify-herdr-bob: ok
```

Every verdict decides without forking a matcher — see "What a branch owes the
tree it was away from" below — and the fake `herdr`'s `pane process-info` answers
with the two argv shapes measured on a live pane, so the adoption predicate is
exercised against the real thing it has to match. It skips, loudly, when the plugin is not installed or when the
checkout has moved off `e53bd475` — the fork-point contract `verify-detection`
already uses.

Two of the first run's failures were the harness's own, and both are worth
keeping:

- the control's bug-4 arm asserted `rc=0`, because a bare reproduction exits 0.
  bob-watch exits 1. bash 3.2 exits with the status the *previous* command left,
  and in the loop that is `match_rules` declining the screen. The pin is now
  that it died on the read and reported nothing, not that it died with a
  particular number.
- the stale-heartbeat arm waited for the *presence* of a `-> blocked` line the
  watcher had logged two transitions earlier, so it returned instantly and
  flipped the screen back to idle before the watcher ever read it. Waits count
  occurrences now, not presence.

And a third, found on 2026-10-03 the first time the verify ran *after* the live
apply, which is the one worth carrying furthest:

- **the control arm stopped being a control the moment the fix was installed.**
  `prepare` staged both copies with `cp -R "$SRC"`, and `$SRC` is the LIVE
  checkout — so once `install-herdr-bob --apply` had patched it, the "pristine"
  copy carried the patch. The fork-point check did not notice, because
  `git rev-parse HEAD` is still `e53bd475`: the patch is in the working tree,
  not in a commit. And it did not *fail*, it **hung** — a patched `bob-watch`
  no longer dies at the first screen read, so the foreground run the control arm
  expects to exit ran the watch loop instead: 300s, two live watchers, and no
  output at all, because the arm's assertions are all downstream of a process
  that was supposed to be dead. `prepare` now stages from the pinned COMMIT
  (`git checkout --force $PINNED_SHA -- .`) and then *proves* the copy is
  pristine by requiring that the patch still applies to it, and the control's
  foreground run is bounded at 10s so that a regression here fails loudly rather
  than hanging silently. The general shape: **a control arm staged from the live
  tree expires the day the fix lands**, and it expires without saying so.

## The live install, and the roots a `--start` must hand the watcher

Applied and started on 2026-10-03 by the operator, under ruling A on
ranger-base-knikn (apply + start + rules). MEASURED, this box:

```
$ scripts/herdr-bob-install.sh --check
commit:   e53bd475…  (the pinned one)
patch:    applied=yes appliable=no
state:    ~/.local/state/herdr/plugins/mloeper.herdr-bob
config:   ~/.config/herdr/plugins/config/mloeper.herdr-bob
watcher:  running (pid 45226)
```

and the first `watch started` line in the whole life of the install:

```
2026-10-03T15:03:13-04:00 watch started (interval 2s, rules …/plugins/config/mloeper.herdr-bob/rules.json)
```

**The operator's finding, and the second half of this bead's work.** The first
`--start` ran `bob-watchctl` with no `HERDR_PLUGIN_CONFIG_DIR`,
`HERDR_PLUGIN_STATE_DIR` or `HERDR_BIN_PATH` in its environment. The plugin's
`lib/common.sh` resolves those with fallbacks — `~/.local/state/herdr-bob` and
`~/.config/herdr-bob` — so the watcher started on the FALLBACK roots: a
different pidfile, a different log, and *upstream's* `rules.json` rather than
the override `scripts/herdr-bob-rules.sh` installs. Both halves of that cost
real money:

- the wrong rules are the ones ranger-base-0sa5a proved **false-positive** on
  Bob's `/` picker, and a wrong `blocked` is acted on — it stops every wait on
  the pane — while a missed one merely falls back to the hook's state;
- and because `bob-watchctl stop` reads the pidfile under whichever root it is
  *given*, the wrong-root watcher is invisible to a stop run under the right
  one. The operator hit exactly that: two watchers, and the plugin's own stop
  could not see one of them.

`scripts/herdr-bob-install.sh` now resolves the three, exports them, and
**prints** them — in `--start` and in read-only `--check` both. The printing is
not decoration: this entire bead is one failure that reported nothing, and a
root chosen in silence is the same shape. `--check` additionally warns when a
watcher is running under the fallback root, naming the stop command that can
reach it, and `--start` refuses outright rather than make the box's second
watcher.

How each is resolved:

| var | from | kind |
|---|---|---|
| `HERDR_PLUGIN_CONFIG_DIR` | `herdr plugin config-dir mloeper.herdr-bob`, else the `herdr plugin list` spelling `scripts/herdr-bob-rules.sh` already uses, else the XDG default | asked |
| `HERDR_PLUGIN_STATE_DIR` | `${XDG_STATE_HOME:-~/.local/state}/herdr/plugins/<plugin-id>` | derived |
| `HERDR_BIN_PATH` | `command -v herdr`, absolute because the watcher outlives the shell that started it | asked |

The state dir is the one with no herdr verb behind it, so it is **ASSUMED
2026-10-03** and corroborated two ways: all four plugins on this box keep their
state at `<state>/herdr/plugins/<plugin-id>`, and the watcher herdr's own
`[[startup]]` hook started wrote its pidfile and its log exactly there. The
installer arm of `verify-herdr-bob` pins all of it behaviourally — where the
pidfile landed and which rules path the watcher logged — rather than by grepping
the script, with a control that shows the fallbacks are what an unexported
`--start` would have got.

## Adoption labels a pane in 5s of a 45s `startup_wait` — ADR 0061 ASSUMED 1, discharged

MEASURED 2026-10-03 17:06 EDT, on the LIVE herdr 0.9.1 server at the watcher's
default settings (`interval 2s`, `BOB_ADOPT_EVERY=3`, i.e. an adoption sweep
every 6s) — not the 1s/every-tick harness the arms above use.

A pane split off this session's own pane, `bob chat` run in it, and **nothing
typed**:

```
t0      herdr pane split … --no-focus     agent at birth: null
t0      herdr pane run w2PK:p2 bob chat
t+5s    agent=bob  agent_status=idle  display_agent="⬡ Bob"
        2026-10-03T17:06:45-04:00 adopted hand-started bob in w2PK:p2
```

5s against posse's 45s `DefaultStartupWait`, which `runtimes/bob.yaml` does not
override. The pane row carried `agent`, `agent_status` and `display_agent`, and
`herdr agent get` answered with the agent rather than `agent_not_found` — the
reading that had never once been true on this box. `pane process-info` on it
showed the two argv shapes the predicate was written against
(`…/node …/bob chat` and `node …/bob chat`).

Two further things that only a live run could say:

- **the bash 3.2 fix holds in production.** The pane stayed registered for ~2
  minutes with the watcher reading its screen every 2s — on the pristine tree
  that read is what killed the process — and the watcher was still alive
  (pid 45226) afterwards.
- **the installed rules do not false-positive on Bob's welcome screen**:
  `agent_status` held `idle` throughout, never `blocked`.

And the lifecycle closes: on `herdr pane close`, `pane w2PK:p2 gone -> released`
at t+25s, the pane file forgotten, the watcher alive, and no other pane on the
box touched.

## What a branch owes the tree it was away from

This bead's work was committed on 2026-10-03 and `main` moved 50 commits before
it landed, which is worth recording because two of those commits were **new
obligations on files this branch already held**, and neither is reachable from
the doors AGENTS.md sends you to after a filtered run:

- `internal/treepins/selftestforkarm_qa_test.go` sweeps every `scripts/verify-*.sh`
  whole, and **in an assertion arm the matcher must not fork** (ranger-base-t07yx,
  ranger-base-7hx87). `verify-herdr-bob.sh` had 24 verdicts deciding through
  `grep`/`sed`/`cat`, which is what it was written with before that pin existed.
  All 24 are swept: the script now carries the `has` / `file_has` / `line_has2` /
  `count_lines` / `iso_lines` / `text_line_is` block the sibling
  `verify-herdr-bob-rules.sh` carries, and decides with `case` and `${...}` over
  bytes already in a variable. The tools *under test* still fork — the plugin's
  own `bob-watch` greps its rules and the fake herdr runs `jq` — because that
  fork is the measurement. What is gone is every fork between the bytes and the
  verdict, including the ones inside `ok`/`bad` *messages*, which can be emptied
  by the very failure the arm is reporting.
- `internal/treepins/boxcheck_qa_test.go` requires every `verify-*` target in the
  Makefile to appear in `scripts/verify-box.sh`'s ROSTER or its EXCLUDED table.
  `verify-herdr-bob` is EXCLUDED, with the reason: it skips with exit 0 when the
  plugin is absent or off `e53bd475`, so on a clock it would print green over a
  box it did not measure — and it starts and kills real `bob-watch` processes.
  The live half is `scripts/herdr-bob-install.sh --check`, which is read-only.

**Neither `make tree-check` nor a `-run` filter reaches either of them.**
`tree-check` is four named tests in `internal/posse`; these two live in
`internal/treepins`, which is arm 1 (`go test ./...`). So the door for "my branch
has been away a while" is `go test ./internal/treepins`, and on a 240s package
that is cheap next to what it catches. Both failures were in files this branch
had not touched since the pins landed, and both would have reached `main` green
by every check the close was otherwise going to run.

## What this does NOT discharge

Adoption-within-`startup_wait` **is** discharged now, above, on the live server
at default settings. What is not:

- **a dispatched Bob seat, end to end.** The 5s above is a pane this session
  split by hand and a `bob chat` it ran by hand. posse's own launch path — its
  workspace, its built-in launch line, its readiness gate reading the label it
  now can get — has not been run since the watcher started, and the other
  independent cause behind ranger-base-5jjtn (the PID channel, ranger-base-f1ytb)
  is a separate fix. That measurement stays a line on ranger-base-6wqe's live
  probe.
- **whether the watcher survives a herdr server restart in the hands of the
  `[[startup]]` hook.** Fixing `setsid` is what makes the hook's one-shot start
  work, and the hook is now the only thing that starts it on a fresh server —
  but no server restart has happened since the apply, so that path is reasoned,
  not measured.
- **supervision.** Still a one-shot hook with no retry. The patch makes the
  start *work*; it does not make a later death visible, and the upstream filing
  carries that as the third ask.

## The reading that generalizes

ADR 0061 rejected "name the reporter plugin in the declaration and verify it is
enabled at launch", priced on the argument that **"enabled" is not "running"**.
Its named trigger has now fired, and the alternative is vindicated rather than
revived: `herdr plugin list` said `enabled` for the entire life of an install
whose watcher had never started once. That check would have passed at exactly
the moment it needed to fail. The label-within-`startup_wait` observable remains
the only reading that catches this.

And the supervision shape is the general lesson for anything posse hangs off a
plugin's `[[startup]]` hook: a one-shot start with no retry and no signal turns
*one* failure into a permanent, invisible one. There is no line anywhere on this
box that says the watcher failed — only the absence of lines that say it
succeeded, which is a thing nobody reads.

## Also found: the `blocked` rules never fire on Bob 2.0.5 either

Separate bug, not in the patch, handed off. Against the six redacted Bob 2.0.5
captures in `etc/herdr/agent-detection/upstream/bob/`, **no rule in the plugin's
shipped `rules.json` matches any screen.** `tool_approval_prompt` satisfies
`all` on the Execute Command dialog (`Approve Once`, `Always Allow Command for
task`) and then misses every `any` alternative: Bob's dialog is a selectable
list, not y/n; it marks the selected row with `→` (U+2192), which is in neither
`>` nor `❯`; and its footer is `Press Enter to confirm`, while `Reject` is a row
rather than a footer verb. `→ Approve Once` plus `Press Enter to confirm` is the
pair that identifies the dialog, and posse's own draft herdr manifest
(`upstream/bob/bob.toml`) already keys on exactly those two.

`rules.json` is seeded into `config_root()` as a user-tunable copy, so this one
is fixable instance-side without patching upstream at all.
