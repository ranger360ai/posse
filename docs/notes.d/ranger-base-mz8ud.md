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

`scripts/verify-herdr-bob.sh` — `make verify-herdr-bob`, ~45s. It copies the
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
verify-herdr-bob: ok
```

The fake `herdr`'s `pane process-info` answers with the two argv shapes measured
on a live pane, so the adoption predicate is exercised against the real thing it
has to match. It skips, loudly, when the plugin is not installed or when the
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

## What this does NOT discharge

Whether adoption labels a pane **within `startup_wait`** on a box where the
watcher can run. That is ADR 0061 ASSUMED 1's surviving half and it stays a line
on ranger-base-6wqe's live probe. This record measures that adoption fires and
labels — against a fake herdr, on a 1s poll with `BOB_ADOPT_EVERY=1`, i.e. under
settings chosen to make it fire fast. It says nothing about the latency of the
real thing on the real server at its default 2s interval and every-3rd-tick
adoption.

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
