# The fence that fences every seat was refreshed by hand, and nothing noticed when it was not (ranger-base-41l0r)

2026-11-14. ADR 0092 D5, the second arm of `posse upgrade`. The class is
ranger-base-2tnbv's: `scripts/bd-argv-gate.{sh,py}` in this repo is SOURCE,
what fences this box is the copy a PreToolUse hook in the operator's Claude
settings names, and posse renders no box-wide hook (ADR 0015 §3) — so a
commit to either file moved nothing and nothing anywhere noticed. The copy
went stale at the next landing and stayed stale silently.

MEASURED 2026-11-14, this seat, on the box as it stood: `make
verify-gate-freshness` read `bd-argv-gate.sh is STALE — it is not HEAD's`
against `~/src/posse @ db4dc29b`, with the parser fresh and all three
behaviour arms holding. One file of the pair, stale in the live fence, with
the detective control that says so wired into `make install` since
ranger-base-d0jo and a bead (2tnbv) that had already been closed once by hand
for the same reason.

## What the arm does

In the epilogue of the same verb, under the same guardrails — the commit is
on origin/main, its `make test` passed, its binary is already installed and
already judged by D4:

1. **Resolve** where the gate is installed, by asking
   `verify-gate-freshness --print-wrappers`.
2. **Compare** the installed pair against `<sha>`'s own
   `scripts/bd-argv-gate.{sh,py}`, and stop here when they are identical.
3. **Keep** `.prev` copies of BOTH files, then **swap** each by `rename(2)`.
4. **Judge** by `verify-gate-freshness --behaviour-only --wrapper <w>` — the
   three arms, allow and deny and untouched — and put both `.prev` copies
   back on a red.

D6's last line carries the outcome as one word: `unchanged`, `installed`,
`refused`, `unknown`.

## The two decisions worth keeping

**Where the location and the verdict come from.** Both are read out of
`scripts/verify-gate-freshness.sh`, which grew two read-only modes for it,
rather than answered again in Go. That script already holds the one reading
of the operator's hook entry in this tree ("not `the file at
~/.config/posse/gate/` — the file the hook REACHES", its own header) and the
one spelling of the three behaviour arms. A second reader of the settings, or
a second copy of the arms, drifts from those silently — and the direction it
drifts in is a verb that installs a box-wide fence and calls it green. The
script still installs nothing, which is its own standing refusal ("an
`--install` flag here would be that tree, one flag away"): the half that
WRITES is in the binary, where a persona's worktree cannot edit it, which is
D1's whole argument one file over.

`--behaviour-only` reads no reference and asks git nothing. That is not a
convenience — the arm runs the script out of a throwaway worktree the verb
deletes minutes later, so a mode that demanded a resolvable HEAD would tie a
reading about the BOX to the state of a tree that is about to be gone.

**The refusal set.** D5 names two cases ("the hook path did not resolve or
had moved"). Enumerated, they are five, and each is a shape the operator's
settings can really be in:

| shape | why it is a refusal and not a write |
|---|---|
| no PreToolUse hook names `bd-argv-gate.sh` | there is no destination; installing one would be choosing which Bash calls on this box are fenced |
| the hook command names a relative path | what it reaches depends on the cwd of whatever ran it |
| the settings name more than one wrapper | this arm refreshes the one pair the hook reaches and will not choose |
| the named path holds no file | "points somewhere new": the box is running unfenced and which directory it belongs in is a settings question |
| the named path is a symlink | somebody's deliberate arrangement, most plausibly into a checkout; a write-beside-then-rename would replace it with a copy, silently |

Each prints the settings entry this arm reads and the one install command,
and writes nothing. The verb never writes `settings.json` (D5, D7).

## Two outcomes D6 does not name, decided here

**An installed copy that is already `<sha>`'s and whose behaviour arms are
red** reads `refused`. It is not `unchanged`, because that word claims the
arms were green; it is not `unknown`, because the arms measured something.
The break is UNDER the bytes — no python3, a bad mode, a settings entry
pointing at something unexpected — and reinstalling the same bytes is the one
prescription that cannot work, which the script says in those words. The arm
has nothing of its own to put back, so it reports and stops.

**Exit 2 after a write** reads `unknown` and LEAVES THE INSTALL STANDING. The
arms measured nothing, which is not evidence of a break, and every path that
answers 2 in this mode is a box property the installed bytes cannot cause: no
python3 for the wrapper to run, or a signalled probe. A gate whose
interpreter is gone is equally dead on either version of the pair, so a
rollback would buy nothing and would hide, behind a word that means "I
measured nothing", which copy is in place. The `.prev` pair is beside it and
the two rollback commands are already in the report.

## Why rename(2) here too

ranger-base-zhw0h measured `install(1)` leaving its target ABSENT for
1,791-2,473 of ~84,000 observations and SHORT for 12,606-13,384 of them over
a 58 ms window on tmpfs, against zero and zero for
write-beside-then-`rename(2)`. That argument was about a binary a launch
might exec. It is stronger for this file: a PreToolUse hook execs the wrapper
on EVERY Bash call of every seat on the box, so the window is one in which
the fleet's bd calls fail closed or run unfenced. Both writes go through
`WriteExecutable`, and this is the site execwrite.go's reason was written
for — a fence exec'd continuously is a fence an inherited write descriptor
answers with ETXTBSY.

## What the pins cost, and one mutation that said so

The arm's pins (`internal/posse/upgradegate_test.go`) stub the two seams and
drive the arm over a planted gate dir; the behaviour reading is stubbed with
a REAL probe — the deny arm, handing the installed wrapper the payload the
script hands it — so "a stub parser that fails the deny arm rolls both files
back" is pinned as that sentence reads. That the real script answers those
arms through the flags the seams pass is pinned where the script can be run
against a planted gate: `internal/treepins/gatearmmodes_qa_test.go`, which
EXTRACTS the flags from the arm's own source and hands them to the script, in
the shape `TestQAHookFreshnessAcceptsTheFlagsTheInstallRecipePasses` already
uses one directory over. Without it, a rename of either mode leaves every
install reporting `gate unknown` with the box's copy never refreshed, and
nothing in Go can see it.

MEASURED by mutation 2026-11-14, three one-line changes:

- `if same` → never taken: the "nothing written" pin reds on the word and on
  the `.prev` files that appeared.
- restore the wrapper only, not the pair: reds on the parser, with the
  message that a half-restored pair is a combination nobody ran.
- delete the symlink arm: **green**. The not-a-regular-file arm catches that
  shape too, so the box is equally safe and the only thing lost is the one
  thing the operator needs from it — why their arrangement was not
  refreshed. The pin now asserts the word `symlink` in the refusal, which is
  what makes that arm load-bearing.

## A census blind spot found on the way

`twdParseDir` (internal/treepins/treewidedoor_qa_test.go) skips every
declaration with a receiver, so an `exec.Command` inside a METHOD is
invisible to `TestQAEveryTrackedProgramATreePinRunsIsDispositioned` — the
census that holds every tracked program a pin runs to a disposition. The
register's own residual paragraph names two escapes (a variable path, a path
inside a `[]string`) and not this one. This bead's rig takes the shape that
lets the census see the call — a plain function, not a method — and the hole
is filed.

Not closed here:
- ranger-base-b3w9x
