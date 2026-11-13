# ranger-base-hxenz — the reaper forked once per pid per route, and `--self-test` paid it seven times

`make verify-reap-test-tmp` priced itself at "a few seconds" in the Makefile
and was MEASURED 24.8-46.2s (ranger-base-7idk4), the costliest `make test`
prerequisite and therefore the biggest single line in the ADR 0088 D6 judge's
body. This is where the time was and what is left.

## Where it went

`scripts/reap-test-tmp.sh --self-test` runs the script seven more times —
four `--apply` sweeps (the claude-scratch table rig, and one per
`generic_rig` class) and three dry-run censuses (`claude_census`, called
before the `-p` fixture, with it, and with the non-print control). Each of
those seven invocations walked `/proc` three separate times, and every walk
forked once per process per route:

| loop | forks | MEASURED 2026-11-13, linux, 384 processes |
|---|---|---|
| `rawrefs` symlinks — `readlink` per cwd, per exe, per open fd | ~1,450 | 1.134s |
| `rawrefs` environ/cmdline — two `tr`s per pid | ~770 | 0.374s |
| blindness control — a `stat -c %u` and a `(exec 3< …)` subshell per pid | ~770 | 0.411s |
| claude census — a `readlink …/exe` per pid | ~385 | 0.384s |

~2,300 forks and 2.30s of scan per invocation, seven invocations, plus six
`sleep 1` settles in the rig: 23.2s, which is the number the door read.

Nothing about this was the self-test's doing. The same 2.3s was paid by every
real sweep too — the hourly one included — and the rig is simply the place it
became visible, because it is the only caller that runs the script eight
times in a row.

## What was changed

Three sites in `scripts/reap-test-tmp.sh`, each argued in place:

- **Symlinks → one `find -printf '%l'`** over every `/proc/<pid>/{cwd,exe,fd}`
  at once. `%l` reads the link without resolving it, which is `readlink`'s
  contract, so a dangling cwd and a `socket:[N]` fd come back spelled the same
  way; `-P` is find's default, so the starting points that are themselves
  symlinks are reported and not followed. 1.134s → 0.015s.
- **environ/cmdline → bash's own `read -r -d ''`.** The trailing token is
  printed after the loop as well: a cmdline that does not end in NUL (a
  process that rewrote its own argv) leaves its last token in `$tok` with
  `read` returning 1, and `tr` would have emitted it. Dropping a ref is the
  one direction this scan may not move in. 0.374s → 0.047s.
- **Blindness control → one `stat` over an array globbed in the current
  shell.** The array is not a style choice: expanded inside the `$( )` the
  glob would also match that subshell's own pid, and `others` — the number a
  reader is asked to trust when the refusal fires — would count the counter.
  The readability probe still OPENS the environ rather than testing `-r`,
  because a same-uid environ can be mode-readable and still denied by
  `ptrace_may_access`; it is now `{ true; } 2>/dev/null 3< …`, which needs no
  fork, with the stderr redirect first because redirections are applied left
  to right and the diagnostic is emitted while the failing one is set up.
  0.411s → 0.007s.
- **claude census → one `find -printf '%p %l'`** over every `…/exe`. The pid
  still comes from the entry the link was read from, and the count and the
  pid list are still one loop's answer, which is what ranger-base-0wvu6 made
  them for. 0.384s → 0.007s.

## Evidence

**Set-equality of the symlink arm.** Five interleaved pairs of the old
per-pid `readlink` loop and the new `find`, unioned per shape (so churn lands
in both unions and a route only one shape can read lands in one):
96 vs 103 paths, and every difference was the observing shape's own footprint
— each run's own output file, `/usr/bin/find`, and the `/proc/<pid>/fd` dirs
find holds open while it walks them. find's footprint is the larger of the
two and names nothing under a candidate root, so it can only ever hold a dir,
never free one. The census arm's two spellings produced byte-identical pid
lists.

**One invocation, interleaved before/after, 5 pairs** (load 4.3):
2.640 / 2.667 / 3.154 / 3.920 / 3.144s before, 0.216 / 0.208 / 0.229 / 0.269
/ 0.226s after.

**The whole `--self-test`, interleaved before/after, 4 pairs** (load 4.1,
the shipped `HEAD` copy made executable so its `"$0"` recursion works):
43.87 / 24.36 / 23.91 / 23.83s before, 7.39 / 7.26 / 7.26 / 7.28s after. All
arms pass on both sides and the two outputs are line-identical apart from the
script name and the fixture pids.

**The fd-only hold route, end to end.** `--self-test`'s `generic_rig` has an
arm for a dir named only by a live process's cwd and one for a dir named only
by its environ, and none for a dir named only by an OPEN FD — which is the
route the `find` rewrite touches most and the one nothing would have caught.
Asked directly: an aged `coldexec-pins*` dir holding a file that a `cd / &&
exec env -i sleep 60 > …/f` holder has open (argv `sleep 60`, cwd `/`, empty
environ, so the fd is its only mention) is KEPT, and an aged unheld sibling
beside it is collected. That gap in the rig is filed as ranger-base-rv65r.

**The door itself**, three warm runs at loadavg 4.7: `make
verify-reap-test-tmp` 7.3 / 7.4 / 7.5s; `make register-check` 22.7 / 23.1 /
33.7s (its comment said `~20s`). So register-check is now the costliest
`make test` prerequisite, and the Makefile says so on its own line.

## What is left, and why it was not taken

About 6s of the remaining 7.3 is six `sleep 1` settles inside the rig — one
per fixture population, each waiting for a backgrounded holder to exec and
become visible through the route its arm reads. A bounded poll on each arm's
own precondition (`readlink /proc/<pid>/cwd` equals the dir, the fixture's pid
appears in a census) would be both faster and strictly more honest than a
fixed wait, and it is the next cut available here.

It was not taken in this bead because the settles are the part of this rig
with a flake history — ranger-base-bttuj, -0wvu6 and -0whbv are all fixtures
that died, arrived or were counted at the wrong instant — and four distinct
preconditions is a design, not a cleanup. The door is no longer the costliest
prerequisite, so the cut buys latency nobody is currently waiting on.
