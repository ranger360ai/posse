# ranger-base-2smyx — four closes verified, and the three pins that pass for the wrong reason

*2026-11-11 · laurie · verify batch over ranger-base-6akzw, ranger-base-xjehw,
ranger-base-4arlo and ranger-base-w5z7u. All four closes hold. What did not
hold is three pins' account of themselves, and one measurement record whose
population moved under it.*

Every number below is MEASURED on this box (linux 7.2.15 x86_64, go1.27.3,
uid 1001, /tmp a 14G tmpfs mounted `usrquota`) on 2026-11-11 between 12:00
and 13:00, unless it says otherwise.

## The verdicts

| close | verdict | how |
|---|---|---|
| ranger-base-6akzw | VERIFIED | five parts present; four mutations kill; the live bwrap pin genuinely refuses |
| ranger-base-xjehw | VERIFIED | `equivalentOnBase` run against the live repo accounts for all three originals |
| ranger-base-4arlo | VERIFIED | eight gating arms reproduced from an independent rig; live dry run at HEAD |
| ranger-base-w5z7u | VERIFIED | four mutations kill; the refusal reproduces end to end in 0.34s |

## ranger-base-xjehw — the replay, read by the function that decides

The block's own observable is not "the keys pair" but "`equivalentOnBase`
accounts for every original, so a later pass does not re-file". So it was run,
not re-derived: a throwaway test in `internal/posse` calling the unexported
function against `/home/posse/src/posse`, base `main`, tip
`8d8fd17384359ce6545f5c29fff117c7943a3f35`. With empty arguments it returns
nil and the rig reds — the arm that shows it can refuse. Against the live
repo:

```
len(eq)=3
  8d8fd1738435 as an equivalent patch on main          byPatch=true
  46ba4998435b as 48a75c5b57b5 (same author, …)        replayed=true
  45f70fb4ab4f as an equivalent patch on main          byPatch=true
```

Two of the three landed as patch-id twins, which is stronger evidence than
the close claimed; only the commit that actually conflicted needed the
identity arm. That one is a faithful SUPERSET of its original — same three
`removeTestHome` call sites, same helper, plus a paragraph explaining the
`ownHome` guard main had added meanwhile. Nothing was dropped, and the
resolution was the correct one: the original would have taken the home down
unconditionally, which under main's `ownHome` is an adopting child removing
its parent's home.

`refs/posse/merge-blocked/` is empty and the branch is at
`refs/posse/retired/posse/jian-yang-posse-ranger-base-vw1cp`.

## ranger-base-6akzw — the live pin cannot tell a refusal from a missing parent

All five parts landed and work. `TestLiveWallExecConfigRefusesEveryRowUnder
TheRenderedWall` really does run here (it does not skip), and really does
discriminate: restoring `.local/share` to `homeCacheGrants` prints
`mise wrapper append: ALLOWED under the rendered wall`, and re-adding
`add(a.StateDir)` prints seven more.

Four mutations, each restored from a golden copy taken before any mutant:

| mutant | result |
|---|---|
| `.local/share` back in `homeCacheGrants` | red: 1 render row + 2 live rows |
| `add(a.StateDir)` restored in `SeatbeltWritable` | red: 5 render rows + 7 live rows |
| `PaneLine` back to `os.WriteFile` | red: the launch line is written THROUGH the planted link |
| `add(&c.Seed, f)` removed | red: 3 render rows + 2 live rows |

**The escape is the per-entry mutation.** `claudeExecConfigDirNames` is a
table of seven, and the pin that loops over it is reading the list under
test, so only the live pin's hand-typed rows can grade it. They cannot:

```
for n in agents commands hooks output-styles rules; do
  sed -i "s/\"$n\", //" internal/posse/wallexecconfig.go
  go test ./internal/posse -run WallExecConfig -count=1   # ok, every time
done
```

Five of the ten table entries survive the whole filter. `CLAUDE.md`,
`settings.json`, `settings.local.json`, `skills` and `plugins` red.

The reason is that the live pin's `try()` reads any nonzero shell exit as
"the wall refused", and those five directories exist **only because they are
denies** — a deny inside a write grant makes `BwrapArgs` emit a
`PrepMkdirAll`, and `weFixtureIn` creates only `skills/`,
`plugins/marketplaces/` and `projects/`. Drop a name and the deny and the
directory go together, so the row passes on the wrong errno. Measured with
the shell's own words, through a probe that prints them:

```
agents   fixture-dir-exists=true   refused=true   .../agents/x: Read-only file system
hooks    fixture-dir-exists=false  refused=true   .../hooks/x:  No such file or directory
```

`skills` and `plugins` discriminate precisely because the fixture makes them
independently of the carve. That is the shape the other five need: assert the
message is a wall refusal, or create all seven in `weFixtureIn`.

Nothing else in the tree reads the table — `grep -rn claudeExecConfigDirNames`
is the production function and that one self-referential loop.

**The shipped behaviour is correct.** All seven names are in the table and all
seven are refused under the real wall. This is a pin defect, not a hole.

**What the fix stopped allowing**, checked rather than assumed: every
`filepath.Join(a.StateDir, …)` writer outside `_test.go` is the launcher or
the dispatcher. `readingslog.go` writes inside the session tree's git dir, not
the state dir; `seat-cadence.log` and `uncounted.log` are the dispatcher's
ledgers. The seat keeps its gates dir and its two audit logs. The census
holds.

## ranger-base-4arlo — the gating reproduces; the population it was measured on moved

Eight arms, from a rig that shares nothing with the closer's: a scratch root
with `claude-<uid>/<slug>/<uuid>` dirs backdated ten days so age never
explains a keep, its own session registry and its own transcripts dir, three
separate live holder processes so no two registry files collide on one pid,
and `--apply` against that root and never `/tmp`.

| arm | want | got |
|---|---|---|
| registered, pid live, `procStart` matches | KEEP | KEEP |
| registered, pid dead | COLLECT | COLLECT |
| registered, live pid, WRONG `procStart` | COLLECT | COLLECT |
| registered, `procStart` ABSENT from the file | — | KEEP |
| transcript fresh, dir old | KEEP | KEEP |
| dir old, no transcript at all | KEEP | KEEP |
| unregistered, dir + transcript old | COLLECT | COLLECT |
| a live process cwd'd inside it | KEEP | KEEP |

The fourth row is not in the closer's table. `[ -n "$rstart" ]` guards the
starttime comparison, so a registry file without the field keeps the dir on
pid-liveness alone. That is the safe direction and needs no fix; it is
recorded so the next reader does not have to re-derive it.

The depth-3 fold — this bead's whole point — is live: the dry run at HEAD
shows `HELD /tmp/claude-1001` in the depth-1 held set and still finds 39
collectable session dirs beneath it. At 12:40 that was **39 dirs, 268 MB,
9800 inodes**, against the close's 39 dirs / 11506 inodes four hours earlier.
Same dirs, fewer inodes — the class is stable, the inode count is not, which
is what a tmpfs under eleven seats looks like.

The blindness refusal fires: an empty registry directory with ten live
`claude` processes on the box prints
`class claude-scratch: REFUSED — 10 live claude processes and the registry … names none of them`
and collects nothing.

### `claude_procs` counts a population the registry does not

The script's header says *"MEASURED on this box: 11 registry files, 11 live
`claude` processes"*, and the SPIKE infers from the equality that the harness
removes the file at exit. The equality does not hold any more, and the reason
matters: **a `claude -p` run writes no registry file.** At 12:20 there were 11
processes whose `exe` basename is `claude` and 10 registry files; the extra
was pid 844414, `claude … -p hello` under bwrap, cwd
`/var/tmp/posse-ungranted-1063935243/proj` — a sibling seat's live-claude
pin fixture.

The inference survives (the unmatched process owns no scratchpad), but the
blindness control's premise does not: `claude_procs` at line 424 counts `-p`
runs, which never register, so the control can refuse the whole class for a
reason that is not blindness. This repo's own suite runs live `claude -p`
pins, so on a quiet box with the timer armed and no interactive session
alive, the class refuses while nothing is wrong. Fail-safe in direction, and
still wrong about what it measures.

### There is no test

`grep -rl reap-test-tmp` over the tree finds the script, `reap-timer-unit.sh`
and one comment in `herdr_test.go`. An `rm -rf` class gated on three signals
has no pin anywhere, and the eleven-row table that licensed it lives only in
a bead comment. The rig below is what a committed one would start from; it
found nothing wrong, which is the point — it is the control that was missing.

```bash
#!/usr/bin/env bash
# claude-scratch gating rig. Dry/apply against ITS OWN root, never /tmp.
set -u
SCRIPT="$1"; BASE="$2"
mkdir -p "$BASE/root" "$BASE/sessions" "$BASE/projects"
ME=$(id -u); OLD='202611010000'; SLUG='-home-posse-proj'
mk() {                        # mk <uuid> <transcript?> <transcript-fresh?>
  local u="$1" tx="$2" fresh="$3"
  local d="$BASE/root/claude-$ME/$SLUG/$u"
  mkdir -p "$d"; : > "$d/marker"
  if [ "$tx" = yes ]; then
    mkdir -p "$BASE/projects/$SLUG"; : > "$BASE/projects/$SLUG/$u.jsonl"
    [ "$fresh" = yes ] || touch -t "$OLD" "$BASE/projects/$SLUG/$u.jsonl"
  fi
  touch -t "$OLD" "$d/marker" "$d"
}
reg() {                       # reg <uuid> <pid> <procStart|OMIT>
  local u="$1" p="$2" s="$3"
  if [ "$s" = OMIT ]; then
    printf '{"pid":%s,"sessionId":"%s","status":"idle"}\n' "$p" "$u"
  else
    printf '{"pid":%s,"sessionId":"%s","procStart":"%s","status":"idle"}\n' "$p" "$u" "$s"
  fi > "$BASE/sessions/$p.json"
}
# ONE PID PER REGISTRY FILE: the files are keyed by pid, so three arms sharing
# a holder overwrite each other and the live arm reads as unregistered.
st() { sed -E 's/.*\) //' "/proc/$1/stat" | awk '{ print $(20) }'; }
sleep 45 >/dev/null 2>&1 </dev/null & PA=$!
sleep 45 >/dev/null 2>&1 </dev/null & PC=$!
sleep 45 >/dev/null 2>&1 </dev/null & PD=$!
mk A yes no;  reg A "$PA" "$(st "$PA")"   # KEEP
mk B yes no;  reg B 999999 1              # COLLECT: pid dead
mk C yes no;  reg C "$PC" 1               # COLLECT: wrong starttime
mk D yes no;  reg D "$PD" OMIT            # KEEP: no procStart, pid alive
mk E yes yes                              # KEEP: transcript fresh
mk F no  no                               # KEEP: no transcript is no signal
mk G yes no                               # COLLECT: unregistered, both old
mk H yes no                               # KEEP: a process is cwd'd inside
( cd "$BASE/root/claude-$ME/$SLUG/H" && exec sleep 45 ) >/dev/null 2>&1 </dev/null & PH=$!
sleep 1
"$SCRIPT" --root "$BASE/root" --class claude-scratch --apply \
  --claude-sessions "$BASE/sessions" --claude-projects "$BASE/projects" --no-bead
for u in A B C D E F G H; do
  [ -d "$BASE/root/claude-$ME/$SLUG/$u" ] && echo "  $u KEPT" || echo "  $u COLLECTED"
done
kill "$PA" "$PC" "$PD" "$PH" 2>/dev/null
```

Two traps it cost a run each to find, both the rig's and not the script's:
`local u="$1" d="…$u"` expands `$u` before the assignment lands, and under
`set -u` that is an unbound-variable death with the output swallowed by a
backgrounded child still holding the pipe.

## ranger-base-w5z7u — the one property the pin names is the one it cannot see

The door works. `make coldexec-check COLDEXEC_TMP_FLOOR_MB=99999999` refuses
in **0.34 s**, exit 2, with the binding number on the screen
(`the temp room is 6060 MB and this arm needs 99999999`) and leaves no
`/tmp/posse-coldexec.*` behind. `requireRoom` is called before `supervise`,
so nothing runs. `test-times.sh` reads `${TMPDIR:-/tmp}`, which the arm has
already pointed at its own directory, so the `DISK` line reads where the
tests will actually write — the bead's last requirement.

Six mutations:

| mutant | treepins pin | coldexec tests |
|---|---|---|
| drop `$(COLDEXEC_ROOM)` from arm 3 | **red** | ok |
| `COLDEXEC_ROOM :=` emptied | **red** | ok |
| drop `$(COLDEXEC_TMP)` from arm 5 | **red** | ok |
| `COLDEXEC_TMP` rooted at `/var/tmp`, not `$${TMPDIR:-/tmp}` | ok | ok |
| `requireRoom(*minTempMB)` never called | ok | **red** |
| `COLDEXEC_TMP_FLOOR_MB` 2048 → 1 | ok | ok |

The last row is not a defect: the pin's claim about the floor is that it is a
number and not zero, and it holds exactly that.

The fourth is. `cxaVarMustHold["COLDEXEC_TMP"]` is
`{"mktemp -d", "TMPDIR=", "trap "}`, and a value rooted at a hard-coded
`/var/tmp` satisfies all three. The pin's own comment names the property it
is failing to hold — *"the arm's own temp directory. Nested under $TMPDIR,
never replacing it"* — and that mutant is precisely the edit this bead argues
against: a TMPDIR off `/tmp` buys the room and costs the four pins
ranger-base-u8xk4 holds, so the pin green and four unrelated pins red is the
exact confusion the close was written to prevent.

## Method note

Three of the four findings are the same shape: **a pin whose fixture is
supplied by the thing under test.** The config-dir rows exist because the
deny created them; the table loop reads the table; the substring set is
satisfied by any value containing the word. In each case the pin is green
against a mutant that would ship the defect, and in each case one independent
witness — the shell's own errno, a hand-typed directory, the path the mktemp
template is rooted at — is the whole fix.
