# The nest helper's venue contract, and the two literals a shipped script reads (ranger-base-q749c)

ADR 0097 D4 implemented, on ADR 0096 D3's helper (ranger-base-pvkhn).
2026-11-14, D1 host (linux 7.2.15 x86_64, 16 cores), bubblewrap 0.13.1,
go1.27.3, branched at 37a94cd9.

## 1. What the four bullets became

| D4 bullet | where it lives | how it is held |
|---|---|---|
| the membership marker | `wallVenueMarker`, logged by `bwrapNestSkip` | `TestQAWallVenueContractLiteralsAreADR0097s` against ADR 0097 |
| fail under `POSSE_WALL_VENUE=1` | `bwrapNestDecision`'s venue arm | `TestQAWallVenueGateRuleIsADR0097D4s`, and arm 3 below |
| the name prefix | `bwrapNestDecision`'s first branch | the same unit pin, and `TestQANoRetiredWallVenueMemberNameDanglesInTheTree` |
| arm 1 only | `//go:build !posse_arm2 && !posse_arm3` on the file | the BUILD (§4) |

The helper moved out of `internal/posse/wall_helpers_test.go` — which stays
untagged, because the rest of its helpers are read by tests in all three
arms — into `internal/posse/wallvenue_test.go`, which carries the tag.

**The rule is a pure function and the gate is four lines of wiring.**
`bwrapNestDecision(name, refusal, venue)` returns one of three actions and
the exact words; `bwrapNestSkip` reads `t.Name()`, the memoized nest probe
and the environment, and calls `t.Log`, `t.Skip` or `t.Fatal`. The split is
not tidiness: the gate can only be exercised by being a member of the class
it gates, so a test that called it with a bad name would FAIL rather than
assert, and a test that called it inside the venue would need a venue to be
in. Twelve rows read the rule with no wall, no bwrap and no environment
write; the tree's 32 members then exercise one row of it each.

`t.Fatal` and `t.Log`, never the `f` spellings: bwrap's stderr is pasted
into two of the three messages and a `%` in it would be read as a verb.

## 2. MEASURED: the three process states, one binary

`go test -c` once, then that binary run three ways. This seat is at cage
`shims` and a bwrap starts from it (the same thing ranger-base-pvkhn §3
found), so the caged column is the test BINARY inside D17's base by hand —
`--ro-bind / /` plus `--unshare-user`, `/tmp` and `/var/tmp` bound rw from a
run dir under `~/.cache` and not under `/tmp`, which the wall covers with its
own. It is the process state the gate reads, and not a posse launch.

| arm | pass | fail | skip | marker lines | D3's sentence | the venue clause |
|---|---|---|---|---|---|---|
| uncaged (D6's control) | 34 | 0 | 0 | 32 | 0 | 0 |
| D17 base, no env (a seat) | 0 | 0 | 34 | 0 | 32 | 0 |
| D17 base, `POSSE_WALL_VENUE=1` | 0 | **32** | 2 | 0 | 32 | 32 |

72s for the three arms together at a one-minute load average of 5-6.

**34 under the filter, 32 members, and the two are named.** `go tool
test2json` over the uncaged arm attributes the marker per test: 34 top-level
tests, all passing, **32 logging the marker exactly once** and never twice.
The two that log none are `TestLiveWallWitnessHoldsUnderTheRenderedWall` and
`TestLiveWallWitnessProbeDoesNotWriteTheLiveWall` — the pins that ask the
AVAILABILITY probe through shipped code, which ADR 0096 D3 leaves with their
own sentence, so they are inside D3's `-run` filter and outside D4's class.
That is the ADR's own count, confirmed by the third arm's complement: it fails
exactly 32 and skips exactly those two. Membership is read from the run and
never inferred from the name (D3), and these two are why that distinction is
not pedantry.

**The 32 are not 32 call sites.** Five fixtures carry 25 of them —
`bgLiveSkip` (6), `wwLiveFixture` (5), `bhLiveFixture` (3), `btLive` (2) and
`wrcNeedBwrap` (9, across five files) — and seven tests call the gate
directly. Every one of them was already arm 1, so the tag cost no file a move.

## 3. The two renames, and the shape of the pin that holds them

D4 renames the members whose names fall outside its own filter:

- `TestLiveStrictMCPStartsOnlyTheLineServersUnderTheRenderedWall`
  → `TestLiveWallStrictMCPStartsOnlyTheLineServers`
- `TestLiveGrokLeaderUseLeaderTUIUnderTheWallStartsNoLeader`
  → `TestLiveWallGrokLeaderUseLeaderTUIStartsNoLeader`

Those two and no others: the grep over every caller of the gate, resolved
through the five fixtures, is the 32 above, and 30 of them already matched.
`TestLiveGrokLineStartsNoCompatMCPOrHook` is NOT one — it execs `sh -c` and no
bwrap, it does not call the gate, and it is correctly left alone
(ranger-base-pvkhn §2 found the same).

**The stale-name rule is per FILE, not per line.** A file that still spells a
retired name must also spell its live one. A line-scoped rule is unwritable:
the record of the rename is itself a tracked file, so the pin would red on the
page that explains it — ADR 0096's D3 paragraph now names both spellings for
exactly that reason, and so does this fragment. What the rule still refuses is
the thing D4 is afraid of: a `-run` door or a sentence naming ONLY the retired
spelling, because a `-run` filter that selects nothing exits 0 and prints
`ok`. That also means `wallVenueRenames`'s rows are written whole rather than
assembled from halves the way `instancePathSpellings` are — the registry stays
greppable and passes its own census by naming both.

The table is held from both ends: a row whose retired name already matched the
filter was never a D4 rename, and a row whose LIVE name is declared nowhere in
the tree is a rename that renamed a test out of existence — which the census
would otherwise read clean over forever.

## 4. MEASURED: the arm tag is the enforcement, and it is the build

A member planted in arm 2 and in arm 3, each on its own:

```
vet: internal/posse/zz_armproof_posse_arm2_test.go:8:2: undefined: bwrapNestSkip
vet: internal/posse/zz_armproof_posse_arm3_test.go:8:2: undefined: bwrapNestSkip
```

and the same file tagged `!posse_arm2 && !posse_arm3` vets clean. This is the
one D4 bullet with no test, on purpose: `scripts/posse-wall-check` runs ONE
untagged `go test -run …`, and an untagged `-run` over a test behind a tag
selects nothing and prints `ok` — the hole `make seed-check` hit twice, where
two pins had to MOVE ARM to be reachable at all (Makefile, `seed-check`'s
comment). A compile error at the moment the file is written is the only report
that cannot be mistaken for a pass.

`armtags_qa_test.go` is green: arm1=2460 arm2=1818 arm3=1756 tests, 719
shared.

## 5. Three literals cross a boundary no compiler reaches

`scripts/posse-wall-check` (ranger-base-lagas, blocked on this bead) is shell.
It cannot be compiled against `internal/posse`, and three strings pass between
them:

- the MARKER it greps out of `go test -json` to count members,
- the member PATTERN it types as its `-run` filter,
- the environment VARIABLE ADR 0097 D1's venue render adds to the run.

Every one of them fails SILENTLY when it drifts. A reworded marker counts zero
members, so the body answers exit 3, and D5's watchdog reports that as "wall
check silent at <repo>" rather than as a wall regression — a wall that came
off would then read as a reporting fault. A filter that no longer selects a
member is a pin the venue never runs. A misspelled variable abstains every
member of a venue run.

The record is the only copy both sides can be held to, so
`TestQAWallVenueContractLiteralsAreADR0097s` holds all three to ADR 0097
itself. MEASURED, three mutants in the record, each restored: rewording the
marker, widening the `-run` filter to `^Test(QA)?Live`, and renaming the
variable each red it, and each failure prints which handshake broke and what
it costs. Two more mutants — a dangling retired name appended to NOTES.md and
to the Makefile — red the census with the path and the line number.

**The variable's spelling is shared with nothing yet.** D1's venue is rendered
by `boxLandedCage` through a flag and that work is its own bead; until it
lands, `wallVenueEnvVar` is the one place the name is written in this tree. The
bead that teaches `boxLandedCage` the flag has to read it from there or lift it
into shipped code. A second literal is precisely the drift above.

## 6. A value that is neither empty nor `1` is refused, not read as "no"

Not in D4, and it is the one place the deliverable goes past the bead.
`wallVenueActive` recognises `""` (every seat, every operator `make test`) and
`"1"` (D1's render) and FAILS on anything else:

```
POSSE_WALL_VENUE is set to "true", and ADR 0097 D1 renders the venue with
exactly POSSE_WALL_VENUE=1. Read as 'not the venue' this would abstain every
member of the wall check, which answers exit 3 and reaches a reader as D5's
"wall check silent" rather than as a wall regression — so it is refused here
instead. Set it to 1, or unset it.
```

A mis-rendered or hand-typed venue read as "not the venue" abstains all 32
members, and D5's watchdog then files silence rather than the wall regression
the run existed to find. The watchdog does catch it on the second consecutive
run, so this is a louder report of something already reported — but it is one
branch, and "an abstention is never a pass" (D5) is the rule this is the one
remaining way to lose quietly. Outside the venue, with the variable unset,
behaviour is exactly ADR 0096 D3's, which is what D4 asks for.

## 7. The doors

Two of the five new pins read the tree from `qibRepoRoot`, which puts them in
the tree-wide class (`internal/treepins/treewidedoor_qa_test.go`), so both are
behind **`make doc-check`** — prose pins over shipped code and docs, which is
what a record's literals and a sentence naming a test are. The class is 150
pins and seventeen doors now; `make tree-check` was 51.2s at a one-minute load
average of 6.48, inside the 55-62s band AGENTS.md prices a quiet box at.

The other three need no door: the rule pin is a pure function, the census's
control reads planted bodies, and the anchor pin reads a const.
