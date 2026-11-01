# The one-helper fence read only its own half (ranger-base-pruuq)

Filed and fixed 2026-11-01. Finding 1 of the ranger-base-2l21z verify,
against the close of ranger-base-k45gn (2026-09-10) — live for 52 days.

## What was wrong

`TestQAOneRepoRootHelperInTheTestPackage` (internal/treepins/treewidedoor_qa_test.go)
is the pin that keeps the tree-wide class derivable: the class is derived from
a body calling its package's ONE repo-root helper, so a SECOND helper is a
class member nothing can see. That is not hypothetical — it is why the pin
exists (ranger-base-sx2dq folded a byte-identical `qspRepoRoot`).

The pin had two halves, one per package, and each half fenced only the route
its own package's helper is written in:

| package | its helper | the route the half read |
|---|---|---|
| internal/posse | `qibRepoRoot` | `runtime.Caller` + a two-level climb (`f.caller && f.ascent`) |
| internal/treepins | `repoRoot` | a climb from `os.Getwd()` up to a `go.mod` (`f.getwdClimb`) |

So a helper in internal/posse written in internal/treepins' spelling was
invisible to the pin whose NAME says there is one helper. The tree carried
**two** of them:

- `k45gnRepoRoot` (internal/posse/verify_k45gn_qa_test.go), landed 2026-09-10
  by ranger-base-k45gn's own close.
- `cdmModuleSources` (internal/posse/credentialdenymove_qa_test.go), landed
  2026-09-05 by ranger-base-o05yg, which widened a package sweep to the whole
  module and spelled the module root for itself.

Both are internal/treepins' `repoRoot` written again under another name.

## Why no other arm could see them

MEASURED 2026-11-01 (darwin 25.6.0, go 1.26.10, this worktree at main
9465b3d7), through `twdAnalyze` itself:

```
k45gnRepoRoot     root=F rootedWalk=F walk=F ascent=F caller=F
                  handRolledTreeWalk=F relEnum=[] climbEnum=F
                  getwdClimb=T shellRoot=F
cdmModuleSources  getwdClimb=T, every other flag F
qibRepoRoot       ascent=T caller=T (the control)
```

Every derived arm reads false, and so did the twin arm — which is the pin
being green over exactly the thing it is named for. `TestQAEveryTreeWidePinHasADoor`
was green for the dependent reason: it demands a door only for a DERIVED
tree-wide pin, and neither of these was derived.

The reason is one line: both bodies ASK THE FILESYSTEM for the marker instead
of computing a climb. There is no `"../.."` for `f.ascent` to read, no
climbing literal at a walker for `climbEnum`, no relative literal for
`relEnum`, and the walk root is an absolute path so `handRolledTreeWalk` is
false too. The twin arms were the only arms that could ever have scored
either.

## What it cost

Two tree-wide pins with no Makefile door, reachable only by the ~950s arm-1
run of internal/posse:

- `TestQANoShippedFileSetsAStoreVarBdStoreEnvDoesNotShed` — WalkDirs `cmd/`
  and `internal/` and censuses every shipped non-test `.go` for a string
  literal that repoints bd's store (ranger-base-ub2x9 through a second
  spelling). Its twin one door over,
  `TestQANoShippedPosseCallHandsBdAStoreSelectingFlag`, is the same subject
  and was doored by `make corpus-check` all along.
- `TestQAEveryLaunchPathThatRendersASeatbeltRefusesACredentialDirEnvSet` —
  walks every non-test `.go` in the MODULE, the widest radius in
  `$(QA_PKGCENSUS_PINS)`. Its sibling in the same file,
  `TestQANoLaunchPathSpellsTheSeatbeltWallPredicateItself`, was in the class
  and in that door the whole time.

Both are discovered file sets at a depth below the root, which ADR 0078 D1
makes the criterion outright: a bead that adds a `.go` file anywhere in that
radius reds them without having read it.

## The fix

**The root, the house fix a sixth time.** Both bodies take it from
`qibRepoRoot`, which is the whole of what puts them in the class.
`cdmModuleSources` keeps the guarantee its old climb was written for —
`qibRepoRoot` stats `go.mod` at the root it computes and `t.Fatal`s if it is
absent, so a package that moves still fails loudly rather than sweeping
whatever subtree the walk started in.

**The fence, and this is the part worth reading.** `twdTwinRoutes` is now ONE
function that both halves of the pin call, so neither half can fence a
spelling the other does not. The asymmetry WAS the defect; neither rule had a
gap in it. Both routes carry a fixture control, positive and negative, driven
through `twdAnalyze`, because the live answer on both is zero once the twins
are folded and this bead is what a confident name over a half-read flag costs.

**The doors.** Both pins into `$(QA_PKGCENSUS_PINS)`, no new door — that
variable's subject is a reading of this repo's own Go sources whose file set is
discovered, which is these two at a wider radius (the same argument the three
ranger-base-g31e0 rows make). What the radius did earn is an edit to the
Makefile's advice beside that variable: it told a seat to type the door when
they touch a `.go` file in internal/posse or under `cmd/`, and now says
anywhere in the module.

## Residual, said out loud

The Getwd route keys on the literal `go.mod` — the marker both twins climbed
to and the marker `repoRoot` itself names. A climb to some other marker
(`AGENTS.md`, `.git`) is a third route and is unread, by this and by every
other rule in the register. It is nowhere in either package (MEASURED
2026-11-01) and is typed as a negative fixture row so the next reader does not
have to census it. Nothing over Go source can enumerate every way to name this
repo; what bounds the class is that there is one helper per package.

## Also measured

The bead's own census said only `verify_k45gn_qa_test.go` computed a repo root
from `os.Getwd()` in internal/posse's tests, and that it was therefore "one
helper and one pin, not a class with a tail". MEASURED false at 9465b3d7:
`cdmModuleSources` is the second, and it is spelled as an inline loop inside a
walking helper rather than as a function whose name ends in `RepoRoot`, which
is presumably what the census keyed on. The tail was one long.

`make tree-check`, as it LANDED — 122 pins, 17 doors, this bead's two onto a
main that had meanwhile landed `ranger-base-m8kfv`'s one: **63s, 65s then 63s**
warm, at a one-minute load average of 3.42, 11.52 and 6.90 over a five-minute
3.11, 5.05 and 4.96, both suite slots free throughout, one warm-up run
discarded. A 2.0s spread across a one-minute load that more than tripled
between the first run and the second — the tightest spread this command has
produced, and it cuts against the chain's "the spread tracks how much the load
MOVES". What the readings do agree on is a floor: 63-73s at this size whatever
the load does under about twelve. `register-check` is 27.7s of it, `pid-check`
8.8s. The two pins this bead doors report 0.21s between them measured alone, so
none of any spread is theirs.

On its own branch, at 121 pins — neither side of the merge, and so not the
reading of record (`docs/notes.d/ranger-base-v3tt4.md` is the rule): 62s then
65s at a one-minute load of 3.78 and 4.11, and 72s on the run before those with
two test binaries cold.
