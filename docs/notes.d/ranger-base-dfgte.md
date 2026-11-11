# ranger-base-dfgte — one resolver for the cage map, and the mapping it declines

ADR 0086 D2, as built. The decision is one sentence — **a
`cage_realize_<goos>:` mapping is declined for a PID it would leave with a
rule unrealized, when this host provides the demanded tier** — and the rule
itself is nine lines. What took the work is the word ONE in "one resolver",
and what the build adds to the decision is that D2's test has two conditions
and only one of them decides.

## The resolver, and why it is unexported underneath

`RealizeCage(demand)` became `RealizeCageFor(ag, rt, demand)` and the map's
raw half — ADR 0083 D7's resolution, the pure function of (config, OS,
provenance, demand) — went from `RealizeCageIn` to **`realizeCageIn`.**

That rename is the whole of what turns "one resolver" from advice into a
property. The decision asks the build to pin that every door reaches one
function, and a pin over call sites can only ever sweep the scope it walks:
the old name was exported, so a second reader assembled in `cmd/posse` would
have been outside a package sweep and invisible to it. Unexported, no package
outside `internal/posse` can reach the map's raw reading at all, and inside it
four QA pins hold the rest:

| pin | what it sweeps |
|---|---|
| `TestQAOneCageRealizeResolverAndEveryDoorReachesIt` | every call to `realizeCageIn` is inside `RealizeCageFor` |
| `TestQACageRealizeEntryRowsAreReadOnlyWhereTheKeyLives` | outside `cagerealize.go`, `CageRealizeEntries` is a VALIDATOR — its rows are discarded, never bound |
| `TestQATheMapFreeParityVariantIsTheDeclineTestsAlone` | `checkParityAtRealizedTier` has one caller, the decline test |
| `TestQAEveryCageMapDoorReachesTheOneResolver` | the four doors are still there: `herdrback.go`, both halves of `parity.go`, and `CheckTier` through `CheckParity` |

The fourth is the floor under the other three and it is not symmetry. Three
arms catch a SECOND reader; a door that stopped reading the map at ALL is the
same defect from the other side — the launch walls the session at the tier it
demanded while parity records the mapped one — and nothing else in the tree
would say so. All four are doored by `make pkgcensus-check`
(`$(QA_PKGCENSUS_PINS)`), registered in
`internal/treepins/treewidedoor_qa_test.go`'s head comment, and they report
0.33s between them.

Why prose was not enough: the defect is silent in both directions. A second
reader compiles, passes every table in `cagerealize_test.go` — they drive the
resolver directly — and produces a session walled at one tier and recorded at
another. The surface an operator reads, `posse gates`, is *parity's* answer,
which is the one that would be right. The wrong answer is the wall.

## The loop, which was real and not hypothetical

"Computing parity at a tier must not re-enter the map, or it loops" is the
bead's own sentence, and it understates it by one step. `CheckParity`'s body
was split so the map is resolved at the entry point and the body
(`checkParityResolved`) takes the pair already resolved; the decline's two
readings go through `checkParityAtRealizedTier`, which reads no
`cage_realize_*` key.

That is not sufficient on its own. `CheckParity` reads the map TWICE — the
demand, and the relaunch's by-PID half, which is how a mapped session's waiver
survives a refresh (ADR 0083 D7, ranger-base-pl3ck) — and the second reading
fires exactly when `ag.Cage` outranks the tier it was handed. **That is the
PID the decline test is about.** So the decline test's parity reading at the
mapped tier would have asked the map about `ag.Cage`, re-entered the decline,
and recursed without bound, for the one PID on this box the feature exists
for. A `readMap bool` on `checkParityResolved` is what stops it, and
`checkParityAtRealizedTier` is its only `false`.

## QUESTION 2 IS SUBSUMED BY QUESTION 3, and the pin is the subsumption

D2's test reads as two conditions: parity at the mapped tier leaves a rule
unrealized that the demanded tier realizes, **and** the demanded tier is in
`AvailableCages`. The code spells both. Only the first decides.

**MEASURED 2026-11-11** (D1 host, this worktree): deleting the
`AvailableCages` clause leaves every row of
`TestCageRealizeDeclinesAMappingThatWouldCostAPIDARule` green, the "cannot
provide the demanded tier" row included. The reason is structural rather than
lucky — the only tiers a map may name are `shims` and `seatbelt` (`container`
is refused on both sides), every row seatbelt realizes that shims does not
sits behind parity's own `seatbelt := a.cageAvailable(cage) && …`, and a tier
this host does not provide therefore realizes nothing extra for question 3 to
find.

The clause stays: it is the decision's own wording, and it is the cheap
question — an unavailable tier answers there instead of through two parity
readings. What is PINNED is the subsumption
(`TestCageRealizeAnUnavailableDemandRealizesNothingToDeclineFor`), because
pinning a guard no mutation can reach is a test of nothing, and a guard
documented as load-bearing when it is not is the thing a later reader deletes
on the strength of a green suite. If that pin ever reds, the clause is the
only thing standing between a decline and a tier that holds no rule.

Two mutations that DO red, for the record: dropping the "realized at the
demanded tier" half of the test reds 2 arms, and never declining reds 8.

## What the decision did not say and the build had to settle

**The rules the decline line names come from PARITY's rows, not from a list of
tool names.** `cageRulesLostByMapping` walks `ag.Deny` and asks membership in
`Realized` at both tiers, so a path-scoped subtree write deny
(`Edit(docs/adr/**)`, ADR 0014 §1) is declined for exactly as a bare `Edit`
is — a table row asserts it, and a hand list of Edit/Write/NotebookEdit would
red. Membership and not the `Unrealized` list, because those entries are
`<gate> — <why>` sentences built for a reader and recovering a rule from one
would be a parse of prose.

**The reading is tier-independent**, which is why the resolver's signature
takes no model tier and reads parity at `DefaultTier`. The model tier reaches
`Degraded` (`BelowFloor`) and `NoDegrade` and touches no row of `Realized`;
the table asserts the decline over every tier in `Tiers`.

**A decline and a waiver are never both set.** `Realized` is the demand,
`Ratified` is cleared, nothing is marked degraded, and the one thing left
behind is the `Declined` line, which parity prints as a DeclaredDifference —
safe by design, never counted toward `Degraded` (ADR 0017 §2). So a declined
session's verdict line reads `DECLARED DIFFERENCE` and never `RATIFIED
DEGRADATION`.

**`posse status`'s standing row names the PIDs and does not just count them.**
G17's sentence is "every PID demanding a mapped tier launches at the realized
one and its session is marked degraded", and for a declined PID every clause
of that is false. The clause is absent entirely where nothing is declined — a
row saying "declined for 0 PID(s)" would be a reader's problem, not a fact —
and the PID walk is skipped where the map lowers nothing, so a home with no
map costs what it always did.

## MEASURED on the D1 host

Linux 7.2.15, bubblewrap 0.13.1, 2026-11-11. The home is a byte copy of the
live `~/homes/ranger` under this session's scratchpad, because `posse gates`
RENDERS — it writes the persona's gates dir and the wall artifact — and a
deployed home is not this seat's to write with an uninstalled build. The copy
differs from the live home in its PATH and nothing else; the path reaches
parity only through the writable set (the gates dir, the memory dir) and no
row's verdict.

`posse gates hoover`, cwd this session's linked worktree — which is R0's own
shape, a tree whose `.git` is a FILE:

```
claude @ seatbelt (bwrap)/strong: DEGRADED
  ✓ Edit   → enforced (L2 seatbelt (bwrap), witnessed at launch)
  ✓ Write  → enforced (L2 seatbelt (bwrap), witnessed at launch)
  ✓ record: store of record → enforced (… probed at launch: 2 targets created and removed under bwrap)
  ⊘ cage_realize: linux seatbelt→shims declined for this PID: shims leaves
    Edit, Write unrealized; launched at seatbelt (bwrap) (ADR 0086 D2)
  ✗ L3 prepare-commit-msg hook — …/.git/hooks/prepare-commit-msg — rendered by
    home ~/homes/ranger, not by this one
```

The one `✗` is the COPY's own cost and not D2's: the hook in `~/src/posse` was
rendered by the live home, so a report run from a copy reads it as another
home's (ADR 0081 D2). The row names `~/homes/ranger` as its owner, which is
the home a live reading would be — STATIC, not measured here.

And the resolver and dispatch's own door, over all 11 PIDs of that home —
`CheckTier` is directory-independent, so this reading is the live home's
reading:

| PID | demand | realized | ratified | declined | CheckTier, no `--allow-degraded` | degraded rows |
|---|---|---|---|---|---|---|
| the security role | seatbelt | **seatbelt** | no | **yes** | ok | 0 |
| five seatbelt PIDs (code, verify, three others) | seatbelt | shims | yes | no | ok | 0 |
| five shims PIDs | shims | shims | no | no | ok | 0 |

`G17 standing: lowered=[seatbelt→shims] declined=[<the security role>]`, and
the row's detail carries `declined for 1 PID(s)`.

This reproduces ADR 0086's own census exactly — one of eleven host PIDs has a
rule shims cannot hold — and it is the first reading of it through the
resolver rather than through two hand-run parities.

## Suite

The three D7 tests that pinned the old consequence were amended rather than
deleted, because D7's sentence stands and only what it concluded moved:

- `TestCageRealizeParityIsComputedAtTheRealizedTier`'s wall arm now runs on
  the far side of `AvailableCages`, which is the only thing separating a
  decline from a mapping for the same PID and the same map.
- `TestCageRealizeLaunchMarksTheSessionWithoutAWaiver` keeps its refusal arm
  for the unavailable-tier shape and gains a real launch for the declined one:
  `CreateSession` at the demanded tier, in a `/var/tmp` fixture outside every
  blanket temp grant, with `CLAUDE_CONFIG_DIR` set — and the verdict is the
  session RECORD read back off disk, `cage: seatbelt` with an empty
  `degraded:`. A pin over call sites cannot see a launch that resolves
  correctly and then records something else, which is the exact failure D2
  names.
- the table's own arms are the three questions and the shapes that answer each
  of them "no", including a map that RAISES a tier, which D2 has nothing to
  say about: it only ever declines to lower.
