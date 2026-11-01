# ADR 0078 built: the tree-wide class stops depending on how a test spells its root (ranger-base-00fyg)

The code half of `ADR 0078` (`ranger-base-er0gn` decided, from
`ranger-base-zm6yw`'s FINDING 1). Two rulings about ONE reading disagreed, and
what separated them was SPELLING: `internal/treepins/treewidedoor_qa_test.go`'s
`qibRepoRoot` arm ignores depth, so a subdirectory walk spelled with the helper
is in the class, while the same walk spelled through a hand-rolled bare-root
identifier (`repo := filepath.Join("..", "..")`) was scored by nothing at all.
`detectionRig` was that second spelling; the known-screen reader and the bob
tripwire enumerate directories under the SAME `etc/herdr/agent-detection` and
were both doored. All five decisions landed in one commit, because each half
alone reds the register — the widened fence fires on `detectionRig` until it is
remediated, and remediating it empties zm6yw's detector until the callers are
doored.

Environment for every measurement below: darwin 25.6.0, go 1.26.10, 2026-11-01,
worktree `dinesh-posse-ranger-base-00fyg`, branched from main `b52fa1b7`.

## What changed

| | |
|---|---|
| D2, the arm | `twdClimbSet` has a third half, `root` — a name bound to the bare repo root. Nothing in it makes `climbs()` answer true on its own (a walk OF the bare root is still the `hidden` arm's), but `joinsOntoBareRoot` reads it as the BASE a named segment joins onto. That is `twdJoinClimbsToASubpath`'s shape with the climb one hop back. |
| D2, one predicate | `twdBareRootExpr` is `twdRootExprs`' own `isRoot` lifted to a free function taking an identifier resolver, so the fence's notion of "the bare root" and the hidden arm's cannot drift. The jurisdiction line between the two arms is read from one definition. |
| D3, the root | `detectionRig` takes its root from `qibRepoRoot(t)`. `root && walk` is then true in that body and the reach carries it to the five callers. |
| D3, the arm move | `verifydetection_qa_test.go` and `verifydetectionscope_qa_test.go` moved from `//go:build posse_arm3` to the arm-1 line `//go:build !posse_arm2 && !posse_arm3` — a door takes no `-tags` and a `-run` that matches nothing exits 0 and prints `ok`. `cutRule` went with them, into an untagged `detectionmanifest_qa_helpers_test.go`, because `codexupdatemenu_qa_test.go` still reads it from arm 3. |
| D3, the door | the five join `$(QA_SEED_PINS)`, whose own comment already says to type it when you add a detection fixture. No new door: that variable's subject is the surface a publication carries, `etc/` included, and two readers of this very directory are already in it. |
| D4, the closure | `TestQATheClimbFenceStillHasItsBareRootJoinHole` keeps its name (ADR 0078 D4 cites it) and inverts: arm 1 holds the three live-defect fixtures as POSITIVE rows, arm 2 holds `twdBareRootJoinReadings` to EMPTY. |
| D5, the prose | `twdRootExprs`' head comment (sx2dq's "only a walk rooted AT the root is tree-wide" is reversed and says so), `twdClimbSet.bind`'s jurisdiction and residual paragraphs, the head comment's own residual parenthesis and jurisdiction paragraph, the treepins-half paragraph that called the posse exclusion "defensible", `twdJoinsTwoDotDots`' own closing sentence, and every `nine tests/callers` sentence. The last one is worth a line: the grep that finds them is case-INSENSITIVE, because `twdJoinsTwoDotDots` spelled it `Nine tests do that, none of them tree-wide` at the head of a sentence and a `grep -n nine` walks straight past it. |

## MEASURED

**Five callers, not nine.** The register's prose repeated "nine" in eight
places. `detectionRig` has five callers —
`TestQAVerifyDetection{FailsACommittedRuleDeletion, CheckInstallFailsAStaleInstall,
ReplaysEveryFixtureItShips, FailsAnOverrideDeletion, NamesTheManifestThatAnswered}`
— and the rewritten prose states no caller count anywhere. The remaining `nine`
in that file are the treepins half's "nine of those twenty", the fence's frozen
"six of nine", three timing readings and `twdSpelled`'s own vocabulary.

**The five, untagged and not skipped.** 1.60s, 1.95s, 2.25s, 2.56s and 2.71s,
three seconds of wall between them, five `--- PASS` lines from
`go test ./internal/posse -v -run '^(…)$'` with no `-tags`. They SKIP without
`herdr` on PATH; `herdr 0.9.1` was on it. `go test -tags posse_arm3 -run
'^TestQAVerifyDetection' ./internal/posse` now reports `no tests to run`.

**The class.** 119 pins behind 17 doors, up from 114 behind 17 — `internal/posse`
goes from 60 derived tree-wide tests to 65, `internal/treepins` stays at 51.

**`make tree-check`: 90-117s over two warm runs**, 116.6s then 90.4s. The first
at a one-minute load average of 15.57 (5-minute 16.22, 15-minute 21.43) with
BOTH suite slots held by sibling seats; the second at a one-minute 27.64
(5-minute 15.49) with one slot free. **The slower run was the one at the lower
load**, which is the register's own "what the spread tracks is how much the load
MOVES, not how high it is" arriving from the other side — both boxes were
moving, one down and one up. `register-check` was 44-49s of it and `seed-check`
4.8s, up from ~0.9s, which is the four seconds ADR 0078's consequences priced.
Neither reading is the 55-56s quiet-box number (`ranger-base-h1q5o`) or the
81-89s steady-busy one (`ranger-base-3e68k`).

**Mutation-checked, two of them**, because what this bead adds is an arm whose
live answer over the tree is zero by construction once the remediation lands —
the negative-control trap the register names everywhere else:

1. restoring `repo := filepath.Join("..", "..")` in `detectionRig` reds THREE
   arms, each naming its own remedy: the closure pin's census (three bodies,
   with file and line), the fence arm in
   `TestQAOneRepoRootHelperInTheTestPackage` (`take the root from qibRepoRoot`),
   and `TestQAEveryTreeWidePinHasADoor` (five `$(QA_SEED_PINS)` names "no longer
   derived as tree-wide"). The last is the one that matters: without the door
   half, a mutant that un-remediates the rig takes the five pins out of the
   class and the door still prints `ok`.
2. disabling `joinsOntoBareRoot` reds the closure pin's arm 1 on all three
   fixtures with the inversion message, and the fence's own fixture table
   beside it. Not "arm 1 alone": the five bare-root-join rows were added to the
   fence's table too, so the arm is typed in both places and either reds on its
   own.

## The residual, said out loud

`joinsOntoBareRoot` tests its segments with `twdClimbNamesASubpath` against an
EMPTY climb set, exactly as `twdJoinClimbsToASubpath` asks it — which is what
makes the two Join arms one matcher, and what makes both literal-only. A
segment that arrives through an identifier (`sub := "etc";
os.ReadDir(filepath.Join(repo, sub))`) is unread by either. Nothing in
`internal/posse` is spelled that way (MEASURED 2026-11-01 by the closure pin's
census, which reads the shape independently of `twdAnalyze`).
