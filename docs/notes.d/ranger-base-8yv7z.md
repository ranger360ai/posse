## The fourth spelling, one hop out: an identifier and a parameter (ranger-base-8yv7z)

2026-10-31

ranger-base-nt0pi closed the climb spelled AT the walker — a
`filepath.Glob("../../docs/notes.d/notes-*.md")` reaches the tree while
computing no root, so neither rule the tree-wide class is derived from could
see it — and fenced that spelling as the fourth arm of
`TestQAOneRepoRootHelperInTheTestPackage` (`twdClimbRoot`). It said its own
residual out loud, in two sentences: a climb that arrives at the walker
through an IDENTIFIER is unread, and `shelfPIDs` is the same shape one hop
further out, behind a PARAMETER. This bead is those two.

The bead was filed naming one of each. The fence, once it reads an
identifier, finds **six**.

### What was live

MEASURED 2026-10-31, this worktree, by the widened fence over
`internal/posse/*_test.go`:

```
internal/posse/policydropins_qa_test.go   TestQAThePolicyDropInsDoNotSetTheSameKeyTwice
    const dir = "../../etc/claude/managed-settings.d"; os.ReadDir(dir)
internal/posse/d3knownscreen_qa_test.go   TestQAKnownScreenReaderNamesEveryFixtureExactly
    testdata := filepath.Join("..", "..", "etc", "herdr", "agent-detection", "testdata")
internal/posse/initembed_test.go          TestEmbeddedSeedMatchesExamplesDir
    root := filepath.Join("..", "..", "examples")      (WalkDir, every file)
internal/posse/promote_test.go            TestShippedPIDsDenyPromote
internal/posse/refresh_test.go            TestShippedPIDsDenyRefresh
    dir := filepath.Join("..", "..", "examples", "agents")
internal/posse/shelfguard_qa_test.go      shelfPIDs -> shelfPIDsIn -> statShelf
    carrying TestExampleAgentsArePIDs, TestExampleAgentsHandoffsAreShapes and
    TestShelfPIDsGuardFailsRatherThanSkipsWhenShort
```

Eight pins, none of them in any door variable, none derivable, every one of
them reachable only by the ~950s `internal/posse` run. Each reads a
population an unrelated bead adds to: a fourth policy drop-in, a sixteenth
detection fixture, any file under `examples/`, a tenth or an eighth reference
PID. `TestQAKnownScreenReaderNamesEveryFixtureExactly`'s own comment says so
in as many words — "the detection fixtures are exactly the directory a
detection bead adds to" — which is this class's definition typed out beside a
reading the class could not see.

The shelf's spelling is worth stating because it is the one the bead hoped was
already covered. `shelfPIDs()` handed
`filepath.Join("..", "..", "examples", "agents")` to `shelfPIDsIn(t, dir)`,
which handed its own `dir` to `statShelf(dir)`, which globbed
`filepath.Join(dir, "*.md")`: the literal is **two call sites** from the walk.
`twdParamHops` resolves one argument at one call site, so it stopped one hop
short — and in `internal/posse` it would not have mattered anyway, because
that half of the class reads no `relEnum` at all (a relative literal there is
the package's own directory, not a tree path).

### The fixes are the root, a third time

Six take the root from `qibRepoRoot`. The shelf's helper takes it as a
PARAMETER its callers spell `qibRepoRoot(t)`:

```go
func shelfPIDs(t *testing.T, root string) []string {
	return shelfPIDsIn(t, filepath.Join(root, "examples", "agents"))
}
```

which puts the three tests that read the shelf in the class by the rule half
of it is already derived from — the one repo-root helper — with no rule
widened and no chain followed in the derivation. The register's own lesson,
read a fourth time: the class is not bounded by how a test spells its root, it
is bounded by there being ONE.

All eight went into `seed-check`, no new door. The door's subject is the
surface a publication carries, which is `examples/` and the configuration the
shipped tree carries beside it under `etc/` — the same surface
`TestSeedSurfaceNameCountIsZero` has always walked whole. The Makefile says
that once rather than eight times.

Two MOVED ARM to get a door, into files of their own:
`policydropins_qa_test.go` (from `posse_arm3`) and
`shippedpidsdenyrefresh_qa_test.go` (from `posse_arm2`). A door's `-run` takes
no `-tags` and a filter that matches nothing exits 0, so behind a tag a door
is green over a pin it never ran. That is `TestShelfPIDsLintCleanAsASet`'s
move under nt0pi, twice more, and it prejudges nothing: whether a door learns
to pass a tag is still ranger-base-m94g2's, and two tests changing files is
not a mechanism.

### What widened is the FENCE, and why it may go where the derivation may not

`twdClimbSet` reads a climb held in a local, a local const or a package const.
`twdClimbHops` closes the param-rooted-walker relation over calls, so a climb
handed any number of hops from the walk is refused — the CHAIN
`rootparam_qa_test.go` and `twdRelEnv` both refuse to follow.

Those two refusals still stand, and this does not contradict them, because
they are refusals made on behalf of a DERIVATION. A wrong resolution there is
a door demanded for a fixture walk, or a pin quietly leaving the class — the
exact failure the register exists to prevent. A wrong resolution in a
PROHIBITION costs a seat one sentence of advice, "take the root from
qibRepoRoot and join onto it", which is the right advice for every body the
rule can match. So the fence closes the chain and the derivation does not, and
neither is a copy of the other. The relation that propagates is a parameter
POSITION, not a path: the climbing literal is still resolved at exactly one
place, the outermost call site.

### The jurisdiction line, and the nine tests it is for

The first version of the identifier fence also fenced `detectionRig`
(`verifydetection_qa_test.go`), which holds `filepath.Join("..", "..")` in a
local and then `ReadDir`s `filepath.Join(repo, "etc", "herdr", ...)`. That is
a SUBDIRECTORY walk, and the `internal/posse` half scores it as not tree-wide
on purpose — `twdRootExprs`' head comment names it and the number of tests
that reach it: nine. Fencing it would have been bad advice in a way worth
spelling out, because the advice is the whole output of a prohibition: taking
the root from `qibRepoRoot` there makes `root && walk` true in that body, and
`rootedWalk` carries no subdirectory precision, so all nine callers would have
joined the class and demanded doors.

So an identifier binds into the climb set only when the climb it holds NAMES a
path below the root. The climb arm owns a climb that names a path; the twin
and hidden arms own the climb that names the ROOT, with the dataflow that
distinction needs. A literal bare `../..` written AT the walker is the one
place both can fire, and nothing is lost either way.

This is the one live answer in the bead that moved by argument rather than by
measurement, so both sides of it are typed into the fixture table that drives
`twdAnalyze` — `repo := filepath.Join("..", ".."); os.ReadDir(filepath.Join(repo, "etc", "herdr"))`
reads false, `const dir = "../../etc/claude/managed-settings.d"` reads true.

RESIDUAL, said out loud. A named path built by JOINING onto a bare-root
identifier — `dir := filepath.Join(repo, "etc"); os.ReadDir(dir)` — is unread
here, by the same line, and is the hidden arm's shape. And the inconsistency
that line leaves standing, which is inherited and not created here:
`TestQAKnownScreenReaderNamesEveryFixtureExactly` and `detectionRig`'s nine
callers read the SAME directory, `etc/herdr/agent-detection`, and only the
first is in the class — because one names the path in its climb and the other
joins onto a bare root. Whether the posse half's subdirectory exclusion is
right at all is ranger-base-sx2dq's decision over nine tests and was not
reopened here.

### Cost

MEASURED 2026-10-31, this worktree, at a one-minute load average of 24-30 with
sibling seats on the box.

```
the eight new pins alone   0.464s, 0.493s, 0.514s
make seed-check            0.519s, 0.531s, 0.611s   (0.62-1.23s at eight pins,
                                                     ranger-base-nt0pi)
```

So the door's wall is now mostly these eight — `TestEmbeddedSeedMatchesExamplesDir`
walks every file under `examples/` — and still inside the spread the door
already had. AGENTS.md's figure for it stays at ~0.2s for the reason nt0pi
left it there: a price tag nobody can read to two digits is worse than a stale
one.

`make tree-check` over the finished tree: **81s, 68s and 106s**, three warm
runs, rc 0, with a one-minute load average of 11.15, 9.45 and 10.76 before
them and BOTH suite slots held by sibling seats throughout. That is a 38s
spread read at a load that never left 9-11 and barely moved across the three,
which is worth one sentence because it narrows what the pinned sentence can
claim: ranger-base-nt0pi's pair of readings showed the spread tracking the
MOVEMENT of the load rather than its level, and these three move the wall by
38s with neither the level nor the movement to hang it on. So what the spread
is remains unmeasured; price the command from the quiet end and leave room for
half as much again. The eight pins cost 0.464-0.514s measured alone, so none of
it is theirs. The live count and cost sentence lives in
`internal/treepins/treewidedoor_qa_test.go`, which is the one place that may
say how many pins there are.

MUTATION-CHECKED, four ways, 2026-10-31:

- restoring `shelfPIDs`' climbing literal reds the fence alone, naming
  `internal/posse/shelfguard_qa_test.go:92 shelfPIDs` — so the chain closure
  really is what reaches that spelling;
- restoring `policydropins_qa_test.go`'s `const dir` reds it naming
  `policydropins_qa_test.go:49 TestQAThePolicyDropInsDoNotSetTheSameKeyTwice`;
- stopping `twdClimbSet.bind` from binding anything reds **six** rows of the
  fixture table rather than passing in silence, which is the arm that makes
  this fence's live zero evidence;
- stopping `twdClimbHops` from iterating to a fixed point (the base case
  only, one hop) reds exactly the chain row — "a climb through an identifier
  handed down a chain" — and nothing else, which is the measurement that the
  closure is load-bearing and not decoration.
