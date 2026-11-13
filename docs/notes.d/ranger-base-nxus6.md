# Two pins that were green against the mutant they exist to catch (ranger-base-nxus6)

*2026-11-12 · dinesh · the CODE lane of ranger-base-lydqn, which is
ranger-base-2smyx's verify of ranger-base-6akzw / xjehw / 4arlo / w5z7u.
gilfoyle took that bundle's devops findings. Shipped behaviour was correct in
both of these: what was wrong was the pins' account of what they measure.*

Both are the shape ranger-base-2smyx's method note named — **a pin whose
fixture is supplied by the thing under test** — and in both the fix is one
witness the thing under test cannot move.

## 1. The wall pin could not tell a refusal from a missing parent

`TestLiveWallExecConfigRefusesEveryRowUnderTheRenderedWall` hands posse's own
bwrap render to the real bwrap and tries a write at every member of claude's
config-dir carve. Its `try()` read **any nonzero shell exit** as "the wall
refused".

Five of `claudeExecConfigDirNames`' ten members — `agents`, `commands`,
`hooks`, `output-styles`, `rules` — have no directory in `weFixtureIn`. They
exist at run time ONLY because they are denies: a deny inside a write grant
makes `BwrapArgs` emit a `PrepMkdirAll`. So taking a name out of the table
took the deny and the directory together, and the row passed on `ENOENT`
instead of on `EROFS`. `skills` and `plugins` discriminated precisely because
the fixture makes them independently of the carve. MEASURED 2026-11-11
(ranger-base-2smyx): a `sed` loop dropping each of those five names in turn
read `ok` every time. Two of the five — `output-styles` and `rules` — were
worse than mis-graded: the table had **no row for them at all**, so nothing
was tried.

The render pins cannot grade it either, and that is not an oversight that can
be fixed by looking harder: they `for … range claudeExecConfigDirNames`, so
every per-entry mutant of the table is equivalent to them.

### What landed

Three things, two witnesses, both independent of the carve.

- **The errno.** `try()` returns the shell's words and a refused row is graded
  on them. `wallRefusalWords` is an allowlist of the kernel's own strings and
  holds only what was measured: `Read-only file system` and
  `Device or resource busy`. MEASURED 2026-11-12 over all 28 rows on this box
  (linux 7.2.15 x86_64, bwrap): 26 read the first; the two `mv` rows read the
  second, because their destination IS a bind mount and a rename onto a mount
  point is refused by the mount rather than by the mode. **No row reads
  `No such file or directory`.** A row the wall refuses some other way reds
  here with the shell's words on the screen, and the fix then is to measure it
  and add it with its date — never to widen the list on a guess. A loose
  allowlist is not a witness.
- **The two missing rows**, so every one of the sixteen declared members is
  actually tried, plus a check that reads the tables to DEMAND a row and never
  to supply one. The asymmetry is the point: a mutant that drops a name drops
  its demand and keeps its hand-typed row.
- **The SBPL half, hand-typed and complete.** The render pin's deny-line check
  named four members; it now names all sixteen, written out rather than
  derived, with the reverse direction held too (a member in a table and not in
  the list reds, naming it). This is the half that runs on **every host**: the
  live pin skips where bwrap is not the mechanism, so on darwin the errno
  witness is not available and the hand-typed list is the whole of the grading.

MEASURED 2026-11-12, one mutation per declared member, graded at each
mechanism separately — sixteen members, both red, where five were green at
both before and two were not tried at all:

```
member                 render   live
agents                 RED      RED
commands               RED      RED
daemon                 RED      RED
hooks                  RED      RED
output-styles          RED      RED
plugins                RED      RED
routines               RED      RED
rules                  RED      RED
skills                 RED      RED
workflows              RED      RED
CLAUDE.md              RED      RED
daemon.json            RED      RED
launch.json            RED      RED
scheduled_tasks.json   RED      RED
settings.json          RED      RED
settings.local.json    RED      RED
```

and the live pin's two shapes read, under the mutant that drops `hooks` and
`workflows`:

```
hooks plant: refused, and NOT BY THE WALL — the shell's words are none of
  [Read-only file system Device or resource busy].
  out: /bin/sh: line 1: …/.claude/hooks/x: No such file or directory
workflows plant: ALLOWED under the rendered wall (ranger-base-y438o row)
```

The reverse direction, adding `novel-member` to `claudeExecConfigDirNames`,
reds both pins and names it.

**NOT A HOLE.** All sixteen names are in the shipped tables and all sixteen
are refused under the real wall, re-measured here.

## 2. The coldexec arm pin could not see where the temp directory is

`cxaVarMustHold["COLDEXEC_TMP"]` required the value to contain `mktemp -d`,
`TMPDIR=` and `trap `. None of the three holds the one property the pin's own
comment claims — *"the arm's own temp directory. Nested under $TMPDIR, never
replacing it"* — because a value rooted at a hard-coded path satisfies all
three substrings. MEASURED 2026-11-11 (ranger-base-2smyx): rooting
`COLDEXEC_TMP` at `/var/tmp` read `ok` at the pin and `ok` at
`./cmd/coldexec` too.

That mutant is not a theoretical one. It is exactly the edit ranger-base-w5z7u
argues against and filed ranger-base-u8xk4 over: a TMPDIR off `/tmp` buys the
room and **costs a class** — four pins whose premise is the unconditional bare
`/tmp` in every rendered seatbelt go red, and surfaces that abbreviate `$HOME`
go red when TMPDIR is under `$HOME` (ranger-base-akk9f). This pin green while
four unrelated pins are red is the confusion the close exists to prevent.

### What landed

The root is read as a **shape**, not as a word: `cxaMktempTemplate` takes the
template argument of the first `mktemp -d` in the value, strips one layer of
shell quoting, and the pin requires it to be nested under
`cxaTempRoot` = `$${TMPDIR:-/tmp}`. `"mktemp -d"` came OUT of
`cxaVarMustHold` in the same change and deliberately: the root check cannot
pass without finding that token, so leaving the row would make one dropped
guard read as two findings — the discipline that file already states for a
dropped supervisor.

MEASURED 2026-11-12, re-running ranger-base-2smyx's six mutants plus three new
root mutants. `treepins` is the pin, `coldexec` is
`go test ./cmd/coldexec -run 'Room|Quota|Temp'`:

| mutant | treepins | coldexec |
|---|---|---|
| UNMUTATED | ok | ok |
| `COLDEXEC_TMP` rooted at `/var/tmp` | **red** | ok |
| `COLDEXEC_TMP` rooted at `$$HOME/t` | **red** | ok |
| `COLDEXEC_TMP` loses the `/tmp` fallback (`$${TMPDIR}`) | **red** | ok |
| `COLDEXEC_ROOM` loses `$(COLDEXEC_TMP_FLOOR_MB)` | **red** | ok |
| `COLDEXEC_ROOM :=` emptied | **red** | ok |
| arm 5 drops `$(COLDEXEC_TMP)` | **red** | ok |
| arm 3 drops `$(COLDEXEC_ROOM)` | **red** | ok |
| `COLDEXEC_TMP_FLOOR_MB` 2048 → 1 | ok | ok |
| `COLDEXEC_TMP_FLOOR_MB` 2048 → 0 | **red** | ok |

Row 2 is the escape, now closed. Rows 3 and 4 are new and were not measured
before: a root under `$HOME` and a `$${TMPDIR}` with no fallback both red, so
the check holds the fallback as well as the nesting. The 2048 → 1 row is
**still green and is still not a defect** — the pin's claim about the floor is
that it is a number and not zero, and it holds exactly that; 2048 → 0 is the
row that says so. The four sound arms ranger-base-2smyx recorded as controls
still red, so the pin was not rewritten wholesale.

## The shape, for the next reader

In both cases the pin had a reader whose input the thing under test supplies:
a directory the deny created, a word the value happens to contain. Neither
pin was careless and neither was narrow — they were **circular**, which looks
exactly like green. The question that finds this class is not "does the pin
check enough things" but **"who supplies what this pin reads"**, and the fix
is always the same move: one witness from outside — the kernel's errno, a
hand-typed list, the path a template is rooted at.
