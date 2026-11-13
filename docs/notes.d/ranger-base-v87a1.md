# Which `posse status` rows are evidence about the binary (ranger-base-v87a1)

ADR 0092 D4 rolls a `posse upgrade` back when `posse status` gains a
condition "whose code is one the binary renders", and excludes the codes that
"move on their own". It named three — G12, G17 and the binary/launcher header
— and said so out loud: *"That code set is ASSUMED complete here. The build
bead must list every condition code by source and pin the set."*

This is that list, the measurement behind it, and the one place the build
read D4 differently from the way it is written.

## MEASURED 2026-11-13, at 70dc7b2c

By census over internal/posse's own shipped `.go` sources (the pin below is
the census): **18 condition row names, in five files.** Every row `posse
status` can print, with the store of record govern.go's own table names for
it, and the question D4 asks of each — *does that store move while the verb's
twelve-minute `make test` runs?*

| row | file | store of record | moves on its own | class |
|---|---|---|---|---|
| G1 | govern.go | herdr agent status | yes — seats block and unblock | shop |
| G2 | govern.go | bd + herdr | yes — beads settle, sessions end | shop |
| G3 | govern.go | bd + the clock | yes — an age crosses a threshold | shop |
| G4 | govern.go | the guard + the clock | yes — a skip streak lengthens | shop |
| G5 | govern.go | the plan endpoint | yes — a blind window lengthens | shop |
| G6 | govern.go | the cost scan vs caps | yes — spend accrues | shop |
| G7 | govern.go | config grammar; a flock; a log | **partly** — see below | **BINARY** |
| G8 | govern.go | state/pause.yaml | yes — a human writes it | shop |
| G9 | govern.go | bd + config | yes — a bead becomes ready | shop |
| G10 | verifybox.go | verify-box.yaml | yes — a periodic control writes it | shop |
| G11 | launcherlag.go | the build stamp | **this verb moves it** | **INSTALL** |
| G12 | govern.go | the hooks dirs | no — judged against this binary's own render | **BINARY** |
| G13 | queuejsonl.go | config + git | yes — git state moves | shop |
| G14 | memoryland.go | the personas dir | yes — memory lands | shop |
| G15 | govern.go | the session meta | yes — sessions open and close | shop |
| G16 | govern.go | the bd census | yes — the store grows | shop |
| G17 | govern.go | config + the promoted map | no — a static map, judged by this binary's own cage resolver | **BINARY** |
| — | govern.go | git, herdr, the clock | yes — the three pulse-era carry-overs | shop |

Fourteen excluded, three that trip a rollback, one excluded for a reason of
its own.

### G7 is the one the ADR did not name

Its `loop-dead` and log-mute keys are a flock and an mtime and move on their
own. Its **`arm-broken`** keys do not: they read `autostart_interval:` and
`autostart_via:` out of a config file nothing touches during an upgrade, and
refuse them through *this binary's* `ParseInterval` and `AutostartVia`. A new
binary whose interval grammar narrowed would disarm the fleet's only
instrument — URGENT, and silent by construction (govern.go says so at the
log-mute key: the log stopped on 2026-08-31 and was found two days later with
a live loop holding the lock) — and under the assumed set that install would
have landed clean.

So a code is classified by its **most binary predicate**, because the two
errors are not the same size. A false rollback costs one re-run of the verb
and leaves the fleet on the binary it was already running, with the reason
printed in one line of the report. A missed rollback leaves a broken binary
installed on a box dispatching a live fleet. The price of putting G7 on the
strict side is named rather than hidden: **a watch loop that dies, or goes
mute, inside the suite window rolls the install back and says it did.** The
window is narrow — a loop already dead before the snapshot is in both
readings and is not a gain — but it is real.

### G11 is excluded, and not as shop-state

G11 counts commits from the installed stamp to the tip of the source repo. It
is ABOUT which binary is installed, so the upgrade moves it by construction —
and in a direction the repo's tip decides, because origin/main may have moved
further forward than the install did while the suite ran. A good install can
therefore deepen it. That is a third class and not a quiet member of the
second: its store does not "move on its own", this verb moves it.

### G12 is a trigger, and D4 conflicts with D7 over it

Filed as **ranger-base-qfhph** (`-l architecture`, P1). Built as D4 is
*written* — G12 trips a rollback — because D4 names it in as many words and
nothing of ADR 0092 is live until the operator promotes the drafts (D7).

G12's predicate is byte-for-byte identity between each repo named by
`beads_visibility:` and *this binary's* own render of the git hook
(hookfresh.go, `SweepHookWallIdentity` → `l3AskIdentity`). MEASURED on this
box 2026-11-13: `scripts/verify-hook-freshness.sh --quiet` says "fresh — 4
repo(s) match this binary's render", and `posse status` shows no G12 row.

So a commit that changes the hook render makes all four repos read "ours but
stale" the instant the new binary is in place. D7 keeps `posse gates
install-hooks` OUT of this verb's scope on purpose ("repo hooks are an L3
wall install, and G12 lists them for the operator"), so the designed sequence
is: install → G12 goes stale → the operator re-renders. **D4 reads step two
as a new red and undoes step one.** The verb refuses exactly the landings
that change the wall.

It is worse than a frequency problem. `l3AskIdentity` cannot be the evidence
D4 wants even in principle: it drops the behaviour half deliberately, and its
own header says what the drop gives up — "the behavior half catches a
RENDERER regression … already asked at every launch and once per watch loop".
What is left answers "these bytes are not this binary's render", and cannot
tell a binary that renders a BROKEN hook from one that renders a DIFFERENT
one.

Two resolutions, both the architect's: D4 excludes G12 and classes it with
G11 as "the row is about the install having happened"; or D7 lets the verb
re-render the repo hooks as a third arm beside D5's gate copy, under the same
keep/swap/rollback discipline. The flip is one line of
`upgradeConditionSource` plus one test-table row either way.

### An unknown code is binary-rendered

`upgradeCondSource` answers BINARY for a row the table does not name, and
that is the correct answer rather than a fallback. The table is complete for
the binary doing the judging — the pin says so — so a row the NEW binary
printed and this one cannot name is a condition the new binary invented.

## The pin

`TestQAEveryStatusConditionCodeHasAnUpgradeClass`
(internal/posse/upgradeclass_qa_test.go), door **`make pkgcensus-check`**.

It walks internal/posse's own shipped `.go` with the file set DISCOVERED
rather than spelled, so a row added tomorrow in a file that does not exist
today is still found. Two shapes, both load-bearing:

1. the four-argument row closure, `add("G12", GovLane, key, detail)` —
   matched on its SIGNATURE and not on the callee's name, because `add` names
   something else in five other files of this package. The class in the
   second position is asked for as an **identifier** rather than as
   `GovUrgent`/`GovLane` by name: G3 and G16 compute their class into a local
   first, and a row whose urgency depends on the reading is still a row.
2. the `GovCondition{ID: …}` composite literal — bare, through the package
   selector, **and elided inside a `[]GovCondition`**. The elided form is not
   a corner case: G11 is written that way, and the first cut of this census
   found everything except the one row `posse upgrade` is itself the subject
   of.

It fails in both directions — an unclassified row, and a class for a row
nothing can print any more — and carries a floor (12) that says a matcher
which stopped matching cannot pass over an empty set. Its control arm,
`TestQAUpgradeClassCensusSeesBothRowShapes`, runs the same census over a
fixture under `t.TempDir()` and is therefore NOT in the tree-wide class and
NOT in the door (the register derived that and said so).

Cost: 0.07s for the census over 18 rows in five files, 0.00s for the control
(MEASURED 2026-11-13, one warm filtered run).

## DIVERGED from D4's words: the trigger is COUNTED, not keyed by subject

ADR 0092 D4 says *"The trigger is a diff of the condition lines keyed by code
and subject."* It is realized as a **count per (class, code)**.

The reason is the one thing that makes this diff unlike every other diff in
this tree: **its two sides are two different binaries.**

- The subject lives in `GovCondition.Key`, which `posse status` does not
  print. The only subject a reader of that output has is the DETAIL — and
  govern.go documents the detail as the half that may move freely
  ("percentages, ages — nothing keys on it"). Diffing it rolls back a good
  install because a watch log aged four minutes between the two readings, or
  because a landing reworded a sentence.
- Keying on `Key` itself would not save it even if it were printed. G11's key
  carries a lag BUCKET and G7's a log-age class, so the key moves with the
  reading too.
- Adding a machine-readable rendering (`posse status --keys`) would make the
  trigger depend on two binaries agreeing about a new output format, which is
  a fragility and not a fix: the new binary is a descendant and may
  legitimately change it.

What IS stable across two binaries of this tree is the pair D4 itself names:
the class (URGENT | LANE, ADR 0029) and the row (G1…G17, or the carry-over's
em dash). So the trigger counts those. This is **inside** D4's stated intent —
"It must not merely be a line that moved since the snapshot" — and quieter
than a subject-keyed diff rather than louder.

**What it therefore cannot see**, said out loud because it is the price: a
SUBSTITUTION inside one code — the new binary clearing one G12 hook-wall row
and raising a different one in the same breath — leaves the count unchanged
and does not trip. A binary that makes a wall worse makes MORE rows, which is
the shape that trips; one that makes a different wall worse while fixing the
first is exotic, and the epilogue's `verify-gate-freshness` report still
prints under it.

The URGENT arm of D4's third bullet is subsumed by the class arm and is not
written separately: every URGENT-capable code in the table above is
shop-state except G7, which is already BINARY. The arm that remains and
matters is the unknown-code one.

## What else D4 reads, and what it deliberately does not

- **`posse status`'s exit code is not read.** It exits 1 on any shop with any
  open condition (MEASURED 2026-11-13; cmd/posse's status arm exits 1 for
  `len(set) > 0`), so on a live instance it is 1 before the swap and 1 after
  it whatever happened in between. The OUTPUT is the reading.
- **`posse gates <p>`'s exit code IS read**, 0 → non-zero, which is D4's own
  second bullet. Its not-held rows are counted by ADR 0017 §2's three
  presentation words: the parity block's ` DEGRADED` header, its `✗` rows,
  and the `REFUSED here` lines above it. `RATIFIED DEGRADATION` is
  deliberately not one — it is standing state on every reading of a host
  whose cage map lowers a tier, so counting it would put a permanent non-zero
  on both sides of every diff.
- **The `?` lines are reported and never a trigger.** They are `posse
  status`'s unreadable-store lines. D4 lists three triggers and an unreadable
  store is none of them, and most of what produces one — herdr unreachable,
  bd mid-write — is shop-state that would roll back good installs. The counts
  are printed in the verify block so the delegate sees them.
- **A status that answered before the swap and does not answer after it IS a
  trigger**, which is not one of D4's three bullets and belongs with them:
  every condition test would otherwise read the new binary's silence as an
  all-clear. The anchor is `shop check ·`, the one line that command prints
  unconditionally.

## Where the verify sits in the order

Between the swap and the epilogue, not after it. The epilogue runs two
scripts out of the new worktree and hands one of them the just-installed
binary as `POSSE` — so on a binary this step is about to undo, it is work
spent through the thing under suspicion, and its gate word would describe a
binary that is no longer there by the time D6's line prints. D6's record line
claims an install, so a rolled-back run does not print it at all.

The snapshot is taken after the build and before the keep: as close to the
swap as a reading can be, and on a run that is already going to swap, so it
never delays a refusal. Its cost is one `posse status` and one `posse gates`
per roster persona, seconds against the suite above.

## The rollback is a rename, not the `install(1)` it prints

Both are "not through the new binary" — this is the old binary's own code,
running in a process exec'd from the old inode, shelling out to nothing, so
there is no PATH, no shim and no gate between the decision and the file. The
printed command is for a HUMAN typing it after this process has exited. For
the arm that runs while a fleet may be launching, ranger-base-zhw0h's
measurement applies with its full force: `install(1)` leaves the target
absent or short for roughly 16 ms per install on this box's binary.

`<target>.prev` is left exactly where it is. After a rollback the live binary
and `.prev` hold the same bytes, which costs one binary of disk and keeps the
command this run printed before the swap true for whoever reads the report.
Overwriting it with the binary being rolled back would destroy the only copy
of the one being rolled back TO.

## Not closed here

- ranger-base-qfhph
- ranger-base-41l0r
