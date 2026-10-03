## Bob's launch line delivers no PID, and its pane is never labelled: two independent causes (ranger-base-5jjtn)

*bead: ranger-base-5jjtn (P1 bug) · relates ranger-base-6wqe (instance
lane), ranger-base-rx7l7, ADR 0060 D3, ADR 0061 D2/ASSUMED 1–2 · measurements
below are MEASURED on this box unless marked ASSUMED*

The symptom was one line on the probe record — "herdr saw no agent in the
probe pane within the timeout (startup_wait 45s)", 4/4 observables FAIL — and
it had **two unrelated causes that compound into no survivable route**. Either
alone would have been recoverable. Together there is no way for a Bob pane to
acquire a label at all, so a dispatched seat sits at an empty composer until
the wait runs out.

## 1. `-p` does not deliver a prompt in chat mode (MEASURED 2026-10-01)

Environment: posse 0.5.0+1b8ffe2d, bob 2.0.5 (Homebrew, node 25.2.1), herdr
0.9.1 client and server, darwin 25.4.0. Measured by monica on
ranger-base-5jjtn; recorded here because ADR 0060 D3 is written from the
opposite assumption.

| invocation | result |
|---|---|
| `bob -p '<text>' chat --accept-license --trust --auto-approve -w <dir>` | parses; **delivers nothing**. At 5/10/20/30/45s the pane holds the splash and an EMPTY composer ("Agent Mode (auto-approve)"), there is no reply, and the herdr-bob watcher log shows no `SessionStart`/`UserPromptSubmit` for the pane |
| `bob --accept-license --trust -p '<text>' -w <dir>` (no subcommand) | **headless**, and key-gated: exits at once with `Error: Bob API key is required.`, then names the API-key env var to set — the same refusal `bob run` gives. That env NAME is spelled in Go as `BobCageCred` and deliberately nowhere in markdown, which is why it is described here rather than quoted (the ops-residue scan reads a credential env name in prose as a finding) |
| the same through posse | never reaches bob: `EnsureUnattended` appends `--auto-approve`, which is a `chat`/`run` option that top-level bob refuses — `error: unknown option '--auto-approve'` — so the pane died at the shell prompt |

So **from argv there is no invocation that opens the TUI with a prompt already
submitted.** `bob chat` reads the program-level `-p` back through
`optsWithGlobals()` and ignores it.

ADR 0060 D3's reasoning about argv ORDER was correct and beside the point: the
order mattered only if the flag delivered, and its own Claims marked "a `-p`
value is submitted as the first turn when `chat` starts" as ASSUMED — read off
the flag's help text, never off a turn. **INSTALL's pair probe proved parsing
and nothing else**, which is exactly the gap between the two.

Why this was expensive to find: on a program built with
`.allowUnknownOption()`, *a swallowed flag and an accepted-then-ignored flag
look identical from outside*. Both give a clean launch, a healthy-looking
session and no error on any surface.

**What landed:** `BobCommand` carries no `-p` and no `{file}` — bob has no PID
channel, stated rather than hidden. The QA pin is INVERTED: it used to assert
`-p` before `chat` and was green every day the channel was dead; it now
refuses `-p`, `--prompt` and `$(cat …)` on that line, because any of them
reads to the next reader as "bob delivers a PID".

## 2. The herdr-bob watcher has never run on darwin: `setsid` (MEASURED 2026-10-02)

The other half, and the one nobody had looked at. Same box, same day.

```
command -v setsid                     → nothing, exit 1
```

`bin/bob-watchctl start` — which is the plugin's `[[startup]]` hook, so it is
the only thing that ever starts the watcher — runs:

```
setsid nohup "$HERDR_BOB_BASH" "$ROOT/bin/bob-watch" >/dev/null 2>&1 &
echo $! > "$PIDFILE"
```

`setsid` is util-linux and is not on darwin, so the child exits 127 at once and
`$!` records the dead subshell. Corroboration, all MEASURED:

- the pidfile held `54030` from Sep 28; `kill -0 54030` → no such process;
  `pgrep -fl bob-watch` → nothing.
- the plugin's log, `~/.local/state/herdr/plugins/mloeper.herdr-bob/herdr-bob.log`,
  has **zero `watch started` lines** and **zero `adopted hand-started bob`
  lines** across the whole install. Every line in it is `bob-hook`
  (SessionStart / UserPromptSubmit / PreToolUse / PostToolUse / Stop) or
  `bob-watchctl`'s synchronous `readopt`/`reap`, which herdr runs itself in the
  foreground. So `bob-watch` has never executed once on this box, and
  `adopt_unclaimed` — the route that labels a pane with **no prompt at all** —
  has never fired here.
- `herdr plugin list` says the plugin is **enabled**, throughout.

Live before/after, scratch workspace `w2MS`, no Bob turn and no spend:

```
herdr workspace create --label posse-5jjtn-probe --no-focus --cwd /tmp   → w2MS:p1
herdr pane run w2MS:p1 "bob chat --accept-license --trust --auto-approve -w /tmp"
(12s) herdr pane process-info --pane w2MS:p1
   → shell_pid 2121
     foreground [pid 2923, argv ['/opt/homebrew/Cellar/node/25.2.1/bin/node',
                                '/opt/homebrew/bin/bob','chat','--accept-license',
                                '--trust','--auto-approve','-w','/tmp']]
     foreground [pid 2727, argv ['node','/opt/homebrew/bin/bob','chat', …]]
      herdr agent get w2MS:p1                      → agent_not_found
(started the watcher by hand, 10s)
      herdr agent get w2MS:p1                      → agent_not_found
      pgrep -fl bob-watch                          → nothing (setsid again)
herdr workspace close w2MS ; go run ./cmd/checkorphans → clean
```

Note what this rules out: **both** foreground argvs match the plugin's own
`BOB_INTERPRETER_REGEX` (`(^|/)node(js)?$`) and `BOB_PROCESS_REGEX`
(`(^|/)bob(shell)?(\.js)?$`), and the CLI is plainly live. The adoption
predicate would have succeeded. Nothing was running to ask it.

Two further darwin bugs in the same plugin, both observed live while starting
it by hand:

- `date -Is` → `date: invalid argument 's' for -I`. It is the only thing
  `log()` puts in front of every line, so **every log line the plugin has ever
  written is untimestamped** — which is why that log cannot be dated.
- `heartbeat_fresh` reads `stat -c %Y`, a GNU spelling; on darwin it is
  `stat -f %m`. So the heartbeat always computes `now - 0` and always reads
  stale, and the watcher would never defer idle/working to the hook bridge.

All three are upstream's (MartinLoeper/herdr-bob) and none is posse's code.

## 3. What this says about the bead's own ASK

The bead asked for two things. The first was right; the second rested on a
premise this measurement overturns.

> (2) for `detection: reported` runtimes whose reporter labels the pane only
> AFTER the first prompt, the readiness gate must allow the first typed
> delivery into an unlabelled pane …

**The reporter does not label only after the first prompt.** It has an adoption
path that labels from `pane process-info` with no prompt at all — exactly as
ADR 0061's Claims describe from reading the plugin — and that path has simply
never run on darwin. So the gate must **not** be opened onto an unlabelled
pane: that is ranger-base-3p0's incident verbatim (a work prompt typed into
whatever holds the keyboard, arriving as `Unknown command: /Work`), and ADR
0061 D3.2 exists to prevent it. Loosening the gate would have converted a
45-second timeout into a work prompt executed by a shell, on the strength of a
cause that was never diagnosed.

What posse owed here was the right **door**, not a looser gate — and it was
not giving one. `posse runtime probe` is the surface ADR 0061 D2 property 3
deliberately keeps RUNNING on a reported runtime (its detection gap is a
non-blocking degrade precisely so the probe can measure what detection here
reads), so it is the surface an operator meets first, and it had kept the
DETECTED route's wording for both of its no-label failures, because
`evalProbe` is pure and had no declaration in hand:

```
#3  "the turn did not complete — herdr saw no agent in the probe pane within
     the timeout (startup_wait 45s on bob)"
#4  "herdr saw no agent in the probe pane — agent_not_found … Author a
     detection manifest (docs/runbooks/agent-detection-manifest.md)"
```

Both are the wrong door twice over on a runtime declaring `reported`: the
manifest is upstream's to ship and not posse's to write (ADR 0060 D2), and the
actual cause was an authority that had never started — so the reader was sent
to author a manifest and to consider a larger wait, while the one command that
answers (`herdr plugin list`) appeared on neither line. ADR 0061 D2 property 2
had already decided that sentence and `NoAgentLine` was already its one copy;
the probe just never called it. **Both arms now go through `NoAgentLine`**, so
the probe cannot drift from the launch's own wait.

## 4. Claims, labelled

**MEASURED** (2026-10-01 and 2026-10-02, this box: darwin 25.4.0, herdr 0.9.1
client and server, bob 2.0.5 / node 25.2.1, posse 0.5.0, plugin
mloeper.herdr-bob @e53bd475 reported ENABLED):

1. `bob chat` accepts the program-level `-p` and does not submit it as a turn.
2. Top-level `bob -p …` is headless and refuses without the API-key env var
   (`BobCageCred` in Go, never spelled in prose); it also refuses
   `--auto-approve`, so posse cannot reach it.
3. `setsid` does not exist on darwin, so `bob-watchctl start` cannot start
   `bob-watch`, and `bob-watch` has never run on this box.
4. `adopt_unclaimed` has never fired on this box (zero such log lines ever).
5. A live `bob chat` pane with the watcher dead reads `agent_not_found` while
   two foreground processes match the plugin's own adoption regexes.
6. `date -Is` and `stat -c %Y` both fail on darwin inside that plugin.
7. The probe's no-label observables printed the detected route's wording over a
   runtime declaring `reported` (the live record, `probe.json`, 2026-10-01).

**Discharged by the above**: ADR 0060 Claims/ASSUMED "a `-p` value is submitted
as the first turn when `chat` starts" → FALSE (1). ADR 0061 ASSUMED 1 "a
dispatched Bob pane is labelled within the default `startup_wait`" → FALSE on
darwin (3, 4, 5) — and not by timing: the authority cannot start. ADR 0061
ASSUMED 2 (where the typed work prompt lands relative to Bob's own `-p` turn)
→ MOOT: there is no `-p` turn to queue behind, because `-p` starts none.

**Vindicated**: ADR 0061's rejected alternative "name the reporter plugin in
the declaration and verify it is enabled at launch", which was priced
`"enabled" is not "running"`. `herdr plugin list` said enabled while the
watcher had never run, so that check would have passed exactly when it should
fail. Its named trigger — "a measured spent launch with the plugin disabled" —
has fired, and the label-within-`startup_wait` observable remains the only
reading that catches this.

**Still ASSUMED, deliberately not measured here**: whether adoption labels a
pane within `startup_wait` on a box where the watcher CAN run. That needs a
supervised watcher, which is upstream's fix, and it stays a line on
ranger-base-6wqe's live probe.

## 5. What this bead did not decide

Which channel replaces `-p` is an amendment to ADR 0060 D3, whose own revisit
trigger has now fired, and whose rejected-alternatives list already prices both
candidates (PID typed as the first message; PID materialized as a workspace
mode/rules file, "file it the day a PID-as-first-turn measurably misbehaves").
Whether a dispatched launch onto a runtime that can deliver no PID should
refuse rather than spend a seat is the same decision's other half. Both are
filed as architecture, not guessed here: **ranger-base-f1ytb**.

The other cause — the herdr-bob watcher that has never started — is
**ranger-base-mz8ud**, upstream's code and the instance side's to carry.
