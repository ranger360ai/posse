# ranger-base-4arlo — the harness scratchpads, and the gate that reaches them

`scripts/reap-test-tmp.sh` refused one class on purpose (ranger-base-y9t65):
`/tmp/claude-1001`, the Claude Code harness's per-session scratchpad root,
MEASURED at 44826 inodes — 43% of everything /tmp was holding and the single
biggest item on the filesystem. Neither of the reaper's two gates could reach
it and both failed in the unsafe direction. This bead measured a third gate
and added the class.

All numbers below were MEASURED on this box on 2026-11-11: linux, /tmp a 14G
tmpfs mounted `usrquota` with `nr_inodes=1048576`, 125308 inodes in use at the
time, every seat uid `posse`.

## The two gates really do fail, and they fail unsafe

**The holder scan is blind here.** The reaper reads every process's environ,
argv, cwd, exe and open fds. A live session mid-turn names its own scratchpad
in NONE of them: sweeping all of /proc for this session's own scratchpad path,
from inside this session while it was working, returned exactly one hit — the
sweeping `grep`'s own argv. The path is the harness's, not the shell's, and no
process on the box carries it.

**The age gate is blind the other way.** A session dir's mtime is set when the
session starts and never moves again. Session `7a2a9b8e` — the operator's own
`-home-posse` seat — read **15h stale** while its transcript had been written
to inside the hour and its registry entry said `status=idle`, `pid` alive.
That is a dir both gates call collectable while a person is typing into it.

The bead's own framing had the reaper's HELD list "accidentally hiding the
problem": `/tmp/claude-1001` shows up held whenever any seat is working. That
is the depth-1 fold doing it — one ref anywhere inside the root folds to the
root, and at depth 1 the root is the candidate, so one busy session protected
every dead session on the box. The class needed its own fold at its own depth
before anything else could matter.

## What works: the harness's own registry

`~/.claude/sessions/<pid>.json`, one file per LIVE session:

```json
{"pid":2994524,"sessionId":"3ff26f2e-...","cwd":"...","startedAt":1794406081024,
 "procStart":"29071474","version":"2.1.322","kind":"interactive","status":"idle",...}
```

and the scratchpad path `/tmp/claude-<uid>/<slug>/<uuid>` carries that same
`sessionId` as its last component.

- **11 registry files, 11 live `claude` processes, the same 11 pids.** No
  stale entry, none missing — the harness removes the file at exit. (A later
  reading was 10 and 10. The one run that read 11 processes against 10
  registry entries was a session exiting between the two reads; the process
  count is taken first so that race can only push the class toward refusing.)
- **`procStart` is `/proc/<pid>/stat` field 22 (starttime)**, verified on 2 of
  2, so a recycled pid cannot impersonate a session that is gone.
- **All 11 live sessions had both a transcript and a scratchpad dir.** So "no
  transcript" never means "live".
- **138 scratchpad dirs existed for those 11 sessions.** The other 127 were
  dead sessions' leavings. The harness cleans up the registry and nothing else.

## The transcript, and what it is for

`~/.claude/projects/<slug>/<uuid>.jsonl` is the same session's transcript and
its mtime moves on EVERY turn, which is the signal the bead proposed. It is
kept as the third holder rather than the only one: the registry answers
"is this process alive", the transcript answers "has anybody been here
lately", and they are wrong in different ways.

**Does a resumed session reuse its uuid?** Yes. Over 130 transcripts, not one
held a second `sessionId`, so a resume appends to the same file and therefore
to the same scratchpad path. The longest gap between consecutive entries
inside any one transcript was 1.5h. posse's own resume types into the LIVE
composer (`internal/posse/argvprompt_test.go`, `dispatch.go` ~3194) and a dead
pane is relaunched as a NEW session with a new uuid, so a dispatched session
never returns once its process is gone. What uuid reuse protects is the
operator's hand-typed `claude --resume`, and that is what the 24h grace is for.

**A missing transcript never collects the dir.** It is not a death certificate
— and 11 of the 138 dirs had none, at 2-3 inodes each, which is not worth a
rule that guesses. The honest consequence is named in the script's header: a
dir whose transcript the harness has aged out (its own retention, 30 days by
default) has no signal left and becomes permanently uncollectable. /tmp is a
tmpfs here, so a reboot is the collector of last resort for those.

## The class

```
claude-scratch  claude-[0-9]*/*/*  grace 1440m  depth 3
```

Held by three things, any one of which keeps a dir: the /proc scan folded at
depth 3 (which is what catches a NESTED session whose cwd is inside another
session's scratchpad — this box makes those, the `-tmp-claude-1001-...` slugs);
the live registry; and its own transcript. The dir and the transcript must
BOTH be past the grace.

It has its own blindness control, for the reason the /proc scan has one: a
registry the sweep cannot read reads as "nothing is live", which is the unsafe
answer. If live `claude` processes exist and the registry names none of them,
this class refuses by name and the rest of the sweep goes on.

**Why 1440m**, the same as `mktemp`'s and for the same reason: it is not our
namespace. The pressure arm drops it to 60m with every other class, which is
the right shape — under pressure the registry is doing the work and the grace
is only insurance.

Reclaim at HEAD, against the live /tmp:

| grace | dirs | inodes | % of the inode table |
|---|---|---|---|
| 4h | 98 | 48983 | 39.1 |
| 8h | 79 | 17011 | 13.6 |
| 12h | 53 | 13717 | 10.9 |
| **24h** | **39** | **11506** | **9.2** |

## One bug the live box found that the rig could not

The deep walk was first written as `find "$root" -mindepth 3 -maxdepth 3 -path
"$root/$cglob"`. On a rig root that works. On /tmp it descends into every
other uid's directories to reach depth 3, hits the first `Permission denied`,
and **exits 1** — and under `set -o pipefail` that took the whole sweep down:
exit 1, the four depth-1 classes already reported, nothing collected, no error
message. It is now rooted at the matching top-level dir (so it never leaves
the harness root) and the `find` carries `|| true`, which is the other half of
the lesson: a walk that cannot read something must collect LESS, never abort.

## Verified by execution

One rig, every channel at once, every fixture backdated 3 days so age never
explains a keep, and an unreadable mode-000 dir at depth 1 so the walk is
tested against the thing that broke it:

| channel | fixture | expected | got |
|---|---|---|---|
| registry, live pid, matching starttime | `slug-live` | KEEP | KEEP |
| registry, pid dead | `slug-dead` | COLLECT | COLLECT |
| registry, live pid, WRONG starttime (pid reuse) | `slug-reuse` | COLLECT | COLLECT |
| transcript fresh, dir old | `slug-txfresh` | KEEP | KEEP |
| dir fresh, transcript old | `slug-dirfresh` | KEEP | KEEP |
| no transcript at all | `slug-notx` | KEEP | KEEP |
| unregistered, both old | `slug-plain` | COLLECT | COLLECT |
| a process cwd'd inside it | `slug-cwd` | KEEP | KEEP |
| empty `<slug>` dir after the sweep | `slug-empty` | `rmdir`'d | `rmdir`'d |
| depth-1 regression | `posse-testhome-regression` | COLLECT | COLLECT |
| matched by no class | `keepme`, `unreadable` | KEEP | KEEP |

Two refusal arms: a registry directory that is empty, and one that does not
exist, each with live `claude` processes on the box — both printed
`class claude-scratch: REFUSED` and left all five survivors untouched.

Live dry run at HEAD, `--root /tmp`: 2642 dirs, 1609M, 30942 inodes
collectable, of which `claude-scratch` is 39 dirs — the same 39 the
measurement predicted. Exit 0.

`scripts/shell-syntax.sh`: 69/69. `go run ./cmd/checkorphans`: clean.
