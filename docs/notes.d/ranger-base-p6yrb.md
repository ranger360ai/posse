# ranger-base-p6yrb — a comment that tracked config, and the row that replaced it

*2026-11-23 · from ranger-base-evx07, which paid for this one*

## The trap

`internal/posse/loadguardkill.go`'s header ended with a paragraph headed
**SHIPPED OFF**: `load_guard_kill:` defaults to false, the bead writing it does
not flip it, the ruling's first bar is field data the operator reads, and the
live flip is its own operator bead.

Every sentence was true of the bead that wrote it (ranger-base-gvp2p, shipped
2026-09-01). None was a reading of this box: `~/homes/ranger/config.yaml:271`
reads `load_guard_kill: true`, flipped under ranger-base-w3aet the following
day, 2026-09-02 — so the arm was ARMED here for the whole life of the comment
that said it was off.

That is the header of the file implementing an irreversible destructive act:
TERM and then KILL to processes chosen by a predicate. ranger-base-evx07
listed this arm as the **first** candidate to read for a walled seat killed
from outside, and the first thing the file told that reader was that it could
not have fired. It took a read of the live config to find otherwise. A reader
in a hurry, mid-incident, rules the arm out and looks elsewhere — the exact
false-confidence failure the comment's own long care about false positives
exists to prevent.

It did not change evx07's answer (the predicate cannot select a nested bwrap
seat, four independent ways), but the cost was real and it landed on whoever
read it next.

## Why a comment was the only answer available

Not carelessness — a gap. **The arm's only voice was the arm acting.**
`orphanReport` reaches `LoadGuardKill()` solely on a pass the load guard is
*already skipping* (loadavg over `load_guard:`) that *also* found an
undeclared leak. On every healthy pass an armed box and a disarmed box print
the same thing: nothing. ranger-base-w3aet's own close reason said so in as
many words — *"the arm prints only when it acts"* — and recorded that as the
verification shape. Which makes the implementing file's header the only
standing answer to "is this armed here", and a comment cannot track config.

The same gap hid the typo arm. `load_guard_kill: yes` leaves the arm OFF and
returns a sentence naming the typo — reaching a reader only through
`orphanReport`'s report-only branch, i.e. only once the box holds a leak. An
operator who meant `true` and typed `yes` learns nothing at all until the
moment the arm he thinks he armed declines to act.

## The fix

1. **The header states the default and points at the live value**, instead of
   asserting a state: `App.LoadGuardKill`, config key `load_guard_kill:`,
   re-read on every pass — and `posse status` for the reading.
2. **`posse status` prints that reading** (`LoadGuardKillLine`), on the
   inertness rule the backup and live-box rows keep: `""` on the shipped
   default, because off is the designed state and a line about it on every
   tick is furniture; loud on the two states that are not it. ARMED names the
   arm, the key, the file the value came from and the predicate's own floors,
   from the same constants `orphanReport`'s head renders — a reader
   mid-incident is ruling the arm in or out against a process he is holding,
   and a prose paraphrase of a floor is what this bead is about. The typo
   sentence is passed through **verbatim** rather than re-worded, so one grep
   finds both surfaces.
3. A **reading, not a condition**: no governance row, no exit code. The
   operator armed it on purpose and nothing here says that was wrong.

MEASURED 2026-11-23, this box, `go run ./cmd/posse status`:

```
load guard · kill arm ARMED · load_guard_kill: true in ~/homes/ranger/config.yaml · an UNDECLARED orphaned gate-shell child (ppid 1, over 20% CPU, over 1m) gets TERM and then KILL on a load-skipped pass; POSSE_KEEP= anywhere in its argv spares it
```

## What was checked and not changed

- **The operator bead exists and closed**: ranger-base-w3aet, closed
  2026-09-02 13:57Z, carrying the ruling, the commit, the promote and the
  restarted watch loop. The header's "the live flip is its own operator bead"
  was correct; its tense was not.
- `examples/config.yaml`'s `load_guard_kill:` block was **already** written
  the right way round — it states the default as a default and never as a
  reading of the running instance. Nothing to fix there.
- Not a wall and not a security finding: the arm is working as configured.

## The class

A comment in the tense of the bead that wrote it, in a file whose state an
incident reader has to know. The general rule is cheap — a comment may say
what the DEFAULT is and must not imply what this instance runs — but the
enforceable half is the second one: the live value gets a surface, and the
comment points at it. Where that surface does not exist, the comment is the
surface, and it will be wrong the first time an operator flips anything.
