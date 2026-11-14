# Two pin gaps from the ranger-base-66v31 verify, and why one of them is a pin and not a deletion

ranger-base-ig79v, 2026-11-14. Both findings were pin gaps found by mutation
rather than behaviour defects: nothing on main misbehaved, and both mutants
left `internal/posse` and `cmd/posse` green whole.

Environment for every reading below: Linux 7.2.15-1-omarchy, go1.27.3, this
worktree, one-minute load 2-6, warm build cache.

## Finding 1 — ADR 0095 D1's second conjunct stays, and now discriminates

The key, unchanged:

```go
if exists && writeGranted(own, d) && bwrapGitConfigRepo(d) == absResolve(cwd) {
```

MEASURED (ranger-base-66v31): with the last conjunct replaced by
`bwrapGitConfigRepo(d) != ""` — the seat's-own-grants narrowing kept, the
session-dir narrowing dropped — both packages were green whole. The pin named
for exactly that half,
`TestBwrapArgsGitConfigRefusalIsTheSessionDirsOwnGitDir`, cannot fail when it
goes: its fixture puts the blanket grant at the fixture ROOT, above the cwd,
so the `own` filter one conjunct earlier has already excluded it and the row
passes for the other discriminator's reason.

The finding offered deleting the conjunct as the cheaper answer, on the
ground that it is redundant given `own`. **That reading is correct about the
tree today and is not a reason to drop it.** `sessionGitConfigFiles`
(seatbelt.go) is indeed the only producer that can put a `<X>/.git/config`
into the Deny list, and it names exactly two paths: the cwd's own
repository's config, for which the conjunct is true by construction, and the
beads store of record's, added only when the store is NOT under the cwd
(`!underDir(cwd, home)`) — where no grant at or under the cwd can contain it
and `writeGranted(own, d)` is already false. So on every Deny list production
builds, the conjunct decides nothing, which is why the mutant is green.

Kept, for three reasons, and the decision is recorded here rather than in the
ADR because no line of ADR 0095 changes with it:

1. D1's own sentence is "the check reads the render, not the launch path".
   `BwrapArgs` is exported and takes the carve-out as an argument, so its
   contract is over the Deny lists it can be HANDED, not over the two entries
   one producer emits this week.
2. The redundancy is a cross-file invariant that nothing compares. A third
   producer — or a PID-scoped deny resolving to a nested repository's git dir
   — widens the key silently. The pin a deletion would need ("no producer may
   name a non-cwd `.git/config` inside an own grant") is a harder thing to
   write than the row that was missing.
3. Dropping it makes the refusal WIDER. That is the direction
   ranger-base-hi2lt measured refusing seven renders in this package, and a
   refusal is a launch that does not happen.

The row: `TestBwrapArgsGitConfigRefusalIsNotANestedRepositorysConfig`
(bwrapargs_qa_test.go) — a repository UNDER the cwd whose `.git/config` is
denied, inside a write grant at or under the cwd, over two grantees (the cwd
itself, and a grant under it holding only the nested repo). The other two
conjuncts hold; only the session-dir one says no. It asserts the render is
not refused, that the mount IS rendered and watched (so the row cannot pass
by the deny having been skipped), and keeps the session dir's own config as
the control in the same tree and the same grants, including both denied at
once.

MEASURED: with the conjunct dropped as above, that pin reds
(`--- FAIL: TestBwrapArgsGitConfigRefusalIsNotANestedRepositorysConfig`,
rc=1); unmutated the whole `TestBwrapArgsGitConfig` family is green in
0.01s. ADR 0095 D1's realized note already states the residual this row is
the other side of ("a main checkout whose `.git` is writable through some
other grant of its own ... `bwrapGitConfigRepo` is where it would join"), and
it is unchanged.

## Finding 2 — ADR 0097 D4's nest gate: the rule was pinned five ways, the wiring by nothing

Two surviving mutants, both now dead.

**2(a) the env parse.** `wallVenueActive`'s refusal arm as `t.Logf` instead
of `t.Fatalf` — a `POSSE_WALL_VENUE` that is neither empty nor `1` read as a
quiet no, the one reading D1 says it must never get — survived all five of
ranger-base-q749c's pins. A helper that takes a `*testing.T` can only be
exercised by a test willing to be failed by it, so none of its three arms was
pinned. The parse is now `wallVenueFromEnv(v string) (venue bool, refusal
string)`, pure over the VALUE, with `wallVenueActive` three lines of plumbing
over it. `TestQAWallVenueEnvParseIsADR0097D1s` drives it over `""`, `"1"`,
`"0"`, `"yes"`, `"true"`, `"TRUE"`, `"2"`, `"1\n"`, `" 1"`, `"1 "`, `"01"`
and `"%d%s"`, and asserts each refusal names the variable, quotes the value
and says what D1 renders. `TestQAWallVenueActiveReadsADR0097D1sVariable`
(not parallel — it writes the environment) pins the wiring: which variable
is read, and that the rule's answer comes back unchanged.

The refusal itself stays an ASSUMED past D4's letter, which
ranger-base-q749c flagged for the architecture lane. The pin holds what is
IN FORCE, so narrowing it back to a quiet no is a diff that reds a row
rather than a change nobody notices.

MEASURED: with the refusal arm returning a quiet no,
`TestQAWallVenueEnvParseIsADR0097D1s` reds on all ten refusal rows, rc=1.

**2(b) the emission.** `t.Log(msg)` in the gate's `nestMeasure` arm as
`_ = msg` — the membership marker DECIDED and never EMITTED — survived the
same five pins, and the run a venue would read was green, silent and
uncountable (ranger-base-66v31's reading: 34 pass, 0 fail, rc=0, ZERO marker
lines). `scripts/posse-wall-check` has landed since (ranger-base-lagas,
doors under ranger-base-zpx1k), and it reads membership off that line and
nothing else, so the consequence is live: exit 3, filed by ADR 0097 D5's
watchdog as "wall check silent" rather than as the wall regression the run
existed to find.

The switch moved out of `bwrapNestSkip` into `bwrapNestAct(s nestSink, act
nestAction, msg string)`, where `nestSink` is the gate's caller as the gate
uses it — `Helper`, `Log`, `Skip`, `Fatal`. `*testing.T` satisfies it; so
does `nestRecorder` in the pin. An interface and not three `func(string)`
parameters, because the mapping from action to sink is the half that was
unpinned and named methods cannot be swapped silently.
`TestQAWallVenueGateEmitsTheMarkerItDecides` drives all three actions,
asserting the sink, the exact words, that no other sink was reached, and that
`Helper` was called once (so a marker line still reports the MEMBER's file
and line), plus the composition `bwrapNestDecision` → `bwrapNestAct` over the
one arm that is not an exit.

MEASURED: `s.Log(msg)` → `_ = msg` reds it (rc=1), and so does
`s.Log(msg)` → `s.Skip(msg)`, which is the wiring mutant three `func` values
would have let pass.

## What is still unpinned, stated

`wallVenueActive`'s single `t.Fatal` call, and `bwrapNestSkip`'s argument
plumbing (`t.Name()`, `liveBwrapNestRefusal()`, `wallVenueActive(t)`).
Reaching either from a pin means failing the pin with the words under test.
The 32 members exercise the plumbing, one row each; the words are pinned on
the pure functions.

## Doors

None added. All four new pins are unit pins over package functions — no
`qibRepoRoot`, no tree read — so they are outside the tree-wide class
(internal/treepins/treewidedoor_qa_test.go) and need no Makefile door. They
live in arm 1, named outside ADR 0097 D3's `^Test(QA)?Live(Bwrap|Wall)`
filter, so no venue run counts them as members.
