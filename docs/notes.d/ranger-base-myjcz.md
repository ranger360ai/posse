# ranger-base-myjcz — verify-herdr-bob-rules.sh's watcher pid, and the box that could not run it

`scripts/verify-herdr-bob-rules.sh`'s `watch_once` started the real bob-watch
in the background and recorded its pid in `WATCH_PIDS`, the list the script's
`trap cleanup EXIT INT TERM` kills. Both callers were command substitutions —
`got="$(watch_once "$T/seed" seed)"` — so that assignment, and `drop_pid`'s
`WATCH_PIDS=$keep` one frame further in, were made in a subshell and died with
it. The parent's list was empty for the whole run. Fixed by the house pattern
already written down in `scripts/macos-install-probe.sh:320-334`: the function
answers in a global, `$WATCH_ANSWER`, and both callers run it at top level.

Found by the census in `internal/treepins/trapstatepick_qa_test.go`
(ranger-base-wzo36), where the two functions were the only real-tree members
of that class and sat on `tsExempt` with this bead's id. Both rows are gone and
`tsExempt` is empty; the pin is two-way, so it reds if either comes back.

## The box problem, and what was done about it

The script SKIPs here — herdr-bob is not installed at
`~/.config/herdr/plugins/github/mloeper.herdr-bob-de923c8f068d`, that path does
not exist on this box, and `github.com/mloeper/herdr-bob` is not readable
unauthenticated (`Repository not found`, 2026-11-13). The script also pins the
plugin's HEAD (`PINNED_SHA`), so the real rig cannot be assembled here at all.
The bead was filed rather than committed for exactly that reason: a fix nobody
has run is not a fix.

So the fix was run against a **stand-in checkout**, which is not the same claim
and is written down as a smaller one. `HERDR_BOB_ROOT` already exists as the
override hook (`scripts/verify-herdr-bob.sh:450` uses it). The stand-in's
`bin/bob-watch` does the only three things the two seeding arms exercise:
resolve the config dir from `HERDR_PLUGIN_CONFIG_DIR`, run
`[ -e "$RULES" ] || cp "$ROOT/rules.json" "$RULES"` (that line is quoted
verbatim from the real file by `etc/herdr/herdr-bob/darwin-portability.patch`,
the only copy of bob-watch's text this tree holds), and log one
`watch started (… rules <p>)` line before looping on `herdr pane list`.

The script itself ran **unmodified except for two literals**: `REPO=` (the
harness does not live in `scripts/`, and an untracked file there would be
walked by the treepins census, which reads the directory and not the index) and
`PINNED_SHA=` (there is no way to forge the real plugin's). `watch_once` and
both call sites were taken byte-for-byte from the file under test on every run.
The rule-replay and jq arms in the middle need the real `match_rules` and were
not run; nothing in them touches `WATCH_PIDS` or the trap.

**This is not a substitute for the run the bead asks for.** The DONE WHEN's
"green ON A BOX WHERE IT DOES NOT SKIP" is still outstanding and is the
operator's to do, on the mac where herdr-bob is installed: ranger-base-5b3lq
carries the one command, what "verified" means, and the blast radius.

## MEASURED 2026-11-13, linux 7.2.15 x86_64, bash 5.3.20, stand-in checkout

Normal path, before and after: both seeding verdicts `ok`, exit 0. The two
verdicts the bead required to be unchanged are unchanged.

`kill -TERM` of the script's own pid, 1s into `watch_once`'s poll, with the
stand-in held before its `watch started` line so the window stays open
(sampled every 1s; "bob-watch" is the pid of the live watcher, `$T` is the
scratch dir the trap removes):

| | t+1s | t+9s | t+10s |
|---|---|---|---|
| before | watcher alive, `$T` present | watcher alive, `$T` present | trap has run; watcher was ended by the SUBSHELL's own `kill`, and arm 2's watcher is up |
| after | **watcher gone, `$T` REMOVED** | — | arm 2's watcher is up |

So the pre-fix cost is bigger than the bead's description, in a way that is
worth keeping: the TERM did not reach the handler for **nine more seconds**,
because the parent was blocked inside the command substitution and bash cannot
run a trap there until the child returns. When the handler finally ran it
killed nothing — the list was the parent's. Hoisted, the handler is reached
inside the 1s `sleep` and kills the pid it registered: one sample instead of
ten.

`kill -9` of the script's own pid, same window — **the fix does not help here,
and makes one case worse**:

- before: the orphaned command-substitution subshell survives the parent, runs
  out its poll and kills the watcher at t+10s. `$T` is left behind (no trap
  runs under SIGKILL).
- after: the watcher is the script's own child, so it is orphaned onto the
  reaper and **still running at t+16s** — `1969590 ppid 1140 /bin/bash …/bin/bob-watch`.

Said plainly rather than buried: pre-fix had an accidental babysitter, and the
hoist removes it. Three things make it the right trade anyway. SIGKILL is
uncatchable and no shell script defends against it — what changes is which
process is left, not whether something is. The babysitter only ever ran because
the pid was in the wrong shell, which is the defect. And on THIS box it does
not survive either: the moment the parent dies, that subshell is an orphaned
gate-shell child carrying the script's argv with no `POSSE_KEEP` marker, which
is precisely what the fleet load guard's kill arm ends (NOTES.md, "Leaked
gate-shell children") — after which the watcher is orphaned exactly as it is
now, two steps instead of one. The reachable signal is TERM, and TERM is the
one the fix fixes.

## The pin was shown to say no, over this file, after the fix

Deleting a `tsExempt` row proves nothing on its own — an empty roster is also
what a census that stopped reading produces. So the two call sites were put
back to `got="$(watch_once …)"` and the pin re-run (the mutant restored from a
byte copy afterwards; the file's md5 is the same before and after):

| | want | got |
|---|---|---|
| both call sites reverted | red | **RED**, and on both halves: `watch_once` by its own call site at `:670`, and `drop_pid` `reached only through watch_once` — the transitive arm, over the real tree |
| as committed here | green | GREEN |

So the roster is empty because the tree is clean, not because the reader went
quiet.

## Not closed here

- ranger-base-kkp5g
- ranger-base-5b3lq

The rig (`mkrig.sh`, `harness.sh` and the four transcripts) stayed in the
session scratchpad. It is a reproduction for a dependency this box does not
have, not a gate: shipping it under `scripts/` would put a stand-in bob-watch
in the tree for the verify script to find, which is the one thing the
`PINNED_SHA` check exists to prevent.
