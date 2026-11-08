# Six findings against two sound closes (ranger-base-pl3ck)

2026-11-08. holden's verify of ranger-base-a24nf raised six live findings
against ranger-base-1155n (`cage_realize_<goos>:`) and ranger-base-eoxo1
(the L2 wall's mechanism). Both closes were sound and both shipped pins that
kill the mutants aimed at their own central claims; these are the edges those
pins did not reach. All six are fixed here, and each fix was MEASURED by
re-running the mutant it is for.

## 1 · A tier claimed on a host nothing can type it on

eoxo1 made the seatbelt tier AVAILABLE on Linux (bwrap resolves, the apply
probe passes) and **nothing between that table and the launch line asked
which mechanism realizes it**. `wallRendered` reads the table; both its call
sites then type `SeatbeltPrefix` — `sandbox-exec -f <prof>` — unconditionally.
So on the D1 host a `cage: seatbelt` launch would have reported its L2 rules
realized (parity's `seatbelt :=` reads only `cageAvailable`) and typed a
binary that host does not have, where before eoxo1 it was REFUSED with
"cage seatbelt is not available on this host/build". Not hypothetical: the
host probe measured bwrap 0.13.1 with B0 allowed (ranger-base-hft2q).

The file comment in `wall.go` said TWO questions. There are three, and the
third is **can posse type it**: `seatbeltTierProvidedOn` is what the table
fill reads now, and `wallMechanismRendered` admits `sandbox-exec` alone. ADR
0080 D18 (ranger-base-djl3c) lifts it in one line, in the commit that writes
the bwrap render; `TestQAWallTableClaimsNoTierThisBuildCannotType` reds if
either lands without the other. Kept apart from the other two questions for
their own reason: folded into availability it would make the apply probe
answer for a render, folded into the NAME it would print bare `seatbelt` on
that host again. Recorded in D16's *As built* note, which is where the window
was missing.

## 2 · An assertion satisfied by a different line of the same output

`gatescagerealize_qa_test.go`'s "the collapsed block must say why, naming the
key" asserted `Contains(mapped, "cage_realize:")` over the WHOLE output, and
the ⚖ ratified row in that same output already carries that string. MEASURED:
dropping the key from `main.go`'s Fprintf left it green. **Cut the line out
first** — `gcrLinesWith` — and assert over it. The surface prints that line
once per RUNTIME, not once per persona, so the arm asserts over all of them.
Second half of the same finding: the line named `cage_realize:`, the spelling
`cagerealize.go` refuses BY NAME, so an operator following it would edit a
line posse then refuses. It names `CageRealizeKey(goos)` now.

## 3 · An invariant stated in capitals with no pin

`govern.go`'s G17 arm says "A MAP THIS READER CANNOT PARSE IS NOT AN
ALL-CLEAR" — it goes to `failed`, so the set is PARTIAL. MEASURED: replacing
that append with a discard left every test green, including the two that look
like they cover it. The new arm reads `ShopCheck`'s two returns itself, because
`shopSet` fatals on a partial set and that is the thing being asserted.

## 4 · A map applied twice, with idempotence unchecked

A launch resolves `cage_realize_<goos>:` TWICE — `planLaunch` resolves the
demand, then hands the REALIZED tier to `checkParityIn`, which resolves again.
That second reading is load-bearing (it is how parity reports the waiver for a
relaunch carrying the realized tier), so correctness rests on the map being
idempotent, which the validation advertised as TOTAL did not check. MEASURED:
a member carrying both `shims: seatbelt` and `seatbelt: shims` was accepted,
and then the wall went up at the tier `planLaunch` resolved while parity, the
printed block and the session record all said the other — with a spurious
`cage_realize: darwin seatbelt→shims` waiver for a launch that asked for
shims. Refused now, in `cageRealizeRows` where every other "nothing defaults
here" refusal lives. The test is "a realized tier that is itself a demanded
key, answered with something OTHER than itself", so identity
(`seatbelt: seatbelt` — D7's "not degradation at all") still passes: it is a
key whose answer is itself, so a second application changes nothing.

## 5 · An exported field whose only reader was its own test

`Parity.CageDemanded` was written at one line and read by nothing in the tree
but the close's own test, with a doc comment claiming a purpose no printer
realized. The ⚖ ratified row already carries `<demanded>→<realized>` — the
pair, said where it is load-bearing — and where the map does not LOWER a tier
there is no waiver to be legible about. Deleted; the demand survives in
`ag.Cage`, which is where the relaunch reading reads it back from.

## 6 · A separator contract kept by its inputs rather than by its writer

`DegradedBeyondRatified` splits `degraded:` on `"; "` and asks each part for
the `cage_realize: ` prefix, and the comment above it rested on "the reason
this map writes carries no '; ' of its own". True of the map's own text — but
`cageRealizeReason` interpolates the RATIFIER, and one of its four states
quotes an os error. MEASURED: the reason
`cage_realize: linux seatbelt→shims (promote-manifest.json is unreadable:
open /x; y: permission denied)` reads as REAL degradation, so `RecreateOpts`
arms `AllowDegraded` — the exact direction that predicate exists to prevent.
It needs a home path or an error text carrying `"; "`, so it was unreachable
in practice; stripped at the writer anyway, because the reader's contract is
the separator and this is the one writer that can break it.

## The shape of all six

Five of the six are a claim made in one place and realized in another: a tier
claimed by a table and typed by a prefix, a key named by a printer and
accepted by a reader, an invariant written in a comment and held by an append,
a map resolved at plan and resolved again at parity, a separator promised by a
writer and relied on by a reader. The sixth is the same thing with no second
place at all — a field whose stated purpose had no realizer. A pin that reads
only one side of such a pair stays green while the two drift, which is how all
six survived two closes that each killed their own mutants.
