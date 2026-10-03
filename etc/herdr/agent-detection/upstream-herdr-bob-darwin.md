# herdr-bob: the watcher has never started on darwin — four portability bugs, and `blocked` never fires

**Not filed.** This is a draft for the operator to send to
`MartinLoeper/herdr-bob`; nothing here has been published. See `README.md` in
this directory — this is not a manifest question, but this is where posse keeps
the drafts it means to send. (posse-side record: bead `ranger-base-mz8ud`, and
`docs/notes.d/ranger-base-mz8ud.md` in the posse repo. The same bead's parent,
`ranger-base-5jjtn`, carries the Bob-side half.)

- herdr-bob `@e53bd475` ("Adopt panes by Bob's executable, not by the command
  line"), installed with `herdr plugin install`, `herdr plugin list` →
  **enabled** throughout
- herdr 0.9.1 client and server · bob 2.0.5 / node 25.2.1 · macOS 26.4.1,
  darwin 25.4.0, arm64 · `jq` 1.8.1 · the plugin's `platforms` includes `macos`
- all findings MEASURED 2026-10-02/03 on that box, by `scripts/verify-herdr-bob.sh`
  in the posse repo, which drives *copies* of the installed checkout under a
  fake `herdr` and carries a control arm on the pristine tree

## Summary

`bin/bob-watch` has never executed on this install. Not once: the plugin's log
(`~/.local/state/herdr/plugins/mloeper.herdr-bob/herdr-bob.log`) holds **zero**
`watch started` lines and **zero** `adopted hand-started bob` lines across its
whole life. Every line in it is `bob-hook` or `bob-watchctl`'s synchronous
`readopt`/`reap`, which herdr runs in the foreground itself. The `watch.pid`
file holds a pid that has been dead since the day it was written.

That matters more than a missing `blocked` reading, because `adopt_unclaimed`
is the only route by which a Bob pane is labelled **with no prompt typed**.
Bob fires `SessionStart` on the first prompt, so until the user says something
a freshly launched `bob chat` is invisible to herdr — which is exactly the gap
the adoption path was added to close. The predicate is sound: on a live pane
`herdr pane process-info` reads two foreground processes, argv
`['/opt/homebrew/Cellar/node/25.2.1/bin/node','/opt/homebrew/bin/bob','chat',…]`
and `['node','/opt/homebrew/bin/bob','chat',…]`, and both match the plugin's own
`BOB_INTERPRETER_REGEX` and `BOB_PROCESS_REGEX` — while `herdr agent get`
answers `agent_not_found`. The predicate would have succeeded. Nothing was
running to ask it.

Four bugs, in the order they bite. A patch for all four is attached below.

### 1. `setsid` is util-linux and does not exist on darwin — and nothing retries

`bin/bob-watchctl`, `start`:

```sh
setsid nohup "$HERDR_BOB_BASH" "$ROOT/bin/bob-watch" >/dev/null 2>&1 &
echo $! > "$PIDFILE"
```

`command -v setsid` exits 1 on darwin. So the subshell exits 127 at once, `echo
$!` records that dead subshell, and `start` prints `herdr-bob: watcher started
(pid NNNNN)` over a process that does not exist. `running()` then reads false
forever.

The one-shot shape is what makes it permanent: the only caller is the
manifest's `[[startup]]` hook, nothing supervises the result, and nothing
retries. A single failed start lasts until the herdr server restarts — and then
fails again, identically.

MEASURED, control arm: `start` reports a pid, and 2s later `kill -0` on it says
no such process.

Suggested fix (in the patch): keep the `setsid` arm where it exists, fall back
to perl's `POSIX::setsid`, which is in the darwin base system and preserves the
pid across `exec` so `$!` stays correct, and fall back again to a plain `nohup`,
which survives SIGHUP but not a kill of the startup hook's process group.

Worth considering separately: **supervise the watcher** rather than
fire-and-forget it from a one-shot startup hook. As written, any single crash is
permanent for the life of the server, and there is no signal anywhere that it
happened — which is how this went unnoticed through an entire install.

### 2. `date -Is` — the GNU abbreviation is not portable

`lib/common.sh`, `log()`:

```sh
printf '%s %s\n' "$(date -Is)" "$*" >> "$d/herdr-bob.log"
```

darwin's `date` does take `-I`, but it does not accept an abbreviated `FMT`:
`date -Is` → `date: invalid argument 's' for -I`, exit 1. The substitution
yields the empty string, so **every line the plugin has ever logged begins with
a space and carries no time** — which is why the log cannot be dated at all,
including for this report.

`date -Iseconds`, the unabbreviated spelling, is accepted by both GNU date and
darwin's. MEASURED: `2026-10-03T00:32:13-04:00`.

### 3. `stat -c %Y` is GNU-only, so the heartbeat always reads stale

`bin/bob-watch`, `heartbeat_fresh()`:

```sh
local age=$(( $(date +%s) - $(stat -c %Y "$f" 2>/dev/null || echo 0) ))
```

darwin: `stat: illegal option -- c`. The `|| echo 0` then makes `age` the
current epoch, so the heartbeat is *always* stale and the watcher never defers
`idle`/`working` to the hook bridge — it would re-report `idle` over the hook's
authority on every cleared block, which is the one thing the TTL exists to
prevent.

The two spellings cannot be probed in either order. On GNU, `stat -f %m` is a
**filesystem** query that succeeds and prints a mount point, so the BSD arm must
only run after `-c` has failed, and the result must be checked for digits before
arithmetic sees it. The patch adds a `file_mtime` helper in `lib/common.sh` that
does both.

### 4. `declare -A` on bash 3.2 — the watcher dies at the first screen read

`bin/bob-watch`:

```sh
declare -A last_state=()
```

A stock darwin ships exactly one bash, 3.2.57 at `/bin/bash`, and that is what
`command -v bash` resolves to — so it is what `lib/common.sh` puts in
`HERDR_BOB_BASH`, and what the generated `bob-hook` launcher bakes in on this
box. On 3.2:

- `declare -A` is an invalid option (the array stays indexed), and
- a string subscript is evaluated as *arithmetic*, so under the script's
  `set -u` the first **read** of `last_state[$pane]` — `w2KJ:p1` → the unset
  variable `w2KJ` — is an unbound variable and the shell exits.

That read is in the main loop, reached for every alive registered pane with a
non-empty screen, so the watcher dies on its first screen read. Its stderr is on
`/dev/null` and it writes no log line on the way out, and its status is whatever
the previous command left (0 in a bare reproduction, 1 after `match_rules`
declines a screen) — so nothing anywhere can tell this from a clean stop.

Note the interaction with bug 1: on a box where the watcher *did* detach, this
would still have let adoption fire once (`adopt_unclaimed` runs before the pane
loop) and then killed the process in the same cycle.

The patch drops the bash-4 dependency rather than requiring a newer bash,
because `platforms = ["macos"]` should not mean "macos plus Homebrew bash".
`last_state` only ever held `"blocked"` or `""`, i.e. a set, and herdr pane ids
contain no whitespace — so a space-delimited string and three small functions
replace it with no behaviour change. (The alternative, if a bash-4 dependency is
wanted, is to resolve a `bash` with `BASH_VERSINFO[0] -ge 4` in
`lib/common.sh` and have `bob-watch` refuse out loud when it has not got one.
Either is fine; dying silently is not.)

### 5. Separately: none of Bob 2.0.5's screens matches `rules.json`

Not a portability bug, and not fixed by the attached patch, but it means the
`blocked` half of the plugin is also dark on this box. Against the six redacted
Bob 2.0.5 captures (`etc/herdr/agent-detection/upstream/bob/` in the posse
repo), **no rule in the shipped `rules.json` matches any screen**. The closest
is `tool_approval_prompt` on the Execute Command dialog, which satisfies `all`
(`Approve Once`, `Always Allow Command for task`) and then fails every `any`
alternative:

| `any` pattern | why it misses |
|---|---|
| `\(y(es)?/n(o)?\)` | Bob's approval dialog is a selectable list, not a y/n prompt |
| `[[:space:]]*[>❯][[:space:]]*(yes\|allow\|approve)` | Bob marks the selected row with **`→`** (U+2192), which is in neither `>` nor `❯` |
| `esc to (cancel\|reject)` | Bob's footer is `Press Enter to confirm`; `Reject` is a *row*, not a footer verb |

So `→ Approve Once` + `Press Enter to confirm` is the pair that identifies this
dialog, and neither is reachable by the rules as written. posse's draft herdr
manifest for Bob (`etc/herdr/agent-detection/upstream/bob/bob.toml`, filed
separately in `upstream-bob.md`) keys on exactly those two.

## The patch

`etc/herdr/herdr-bob/darwin-portability.patch` in the posse repo, cut against
`e53bd475`. It is applied to this box's checkout by
`scripts/herdr-bob-install.sh --apply` and proved by
`scripts/verify-herdr-bob.sh`, which runs two arms — the pristine tree, which
must fail all four ways, and the patched tree, which must adopt an unclaimed
Bob pane, report `blocked` once and clear it once, and read a fresh heartbeat as
fresh. Both arms run against copies under a fake `herdr`; neither touches a
live server.

## What posse does in the meantime

Nothing that pretends this works. `runtimes/bob.yaml` declares `detection:
reported`, and until the watcher runs that is a declaration nothing can honour,
so the probe's own reading says so and names `herdr plugin list` first —
**because "enabled" is not "running"**, and on this box `herdr plugin list` said
`enabled` for the entire life of an install whose watcher had never started.
That distinction is the finding posse keeps from this (ADR 0061).
