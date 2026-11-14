# internal/treepins

**Tree pins.** Some tests in this repository have the repository itself as
their fixture. They read the tree — source, docs, Makefile, the ADRs — and
fail when a decision we wrote down has stopped being true: the notes index
lists every fragment, every ADR that names a source file names one that
exists, no shipped file names a person where it should name a role. Elsewhere
these are called *architecture fitness functions* or *architecture tests*
(ArchUnit is the usual example; Go's own `deps_test.go` is an older one).
Ours differ in two ways. Each pin cites the ADR or issue that made the rule,
so a red test tells you which decision you are about to reverse, not just
which line. And each has a fast door — `make fmt-check`, `make notes-check`,
`make adr-check` … — because the package they live in takes minutes and a
`-run` filter never names a test that is nobody's subject; `make tree-check`
runs every door in about a minute. They live in `internal/treepins` and
`internal/posse`. A tree pin is not a *pinning test* in the
characterization-test sense: it asserts a rule we chose, not the behaviour we
happen to have.

## Why the doors exist

A pin whose subject is the tree belongs to no feature, so no `-run` filter
ever names it. Both packages that hold one are minutes long whole —
`internal/posse` and this one — which is longer than a seat spends in a
single foreground call, so the standing advice is a focused filter. Four
commits reached `main` not gofmt-clean straight through that gap
(ranger-base-rulbl): the close that shipped the last one named five patterns,
all green, and none of them was `TreeIsGofmtClean`, because formatting is
nobody's subject.

So every tree-wide pin has a `make` door, and that is itself pinned —
`treewidedoor_qa_test.go` reds until a Makefile door variable names the new
pin.

## The doors

| door | subject |
| --- | --- |
| `make fmt-check` | gofmt over the whole tree (a tool, not a filter) |
| `make crew-check` | does the shipped tree name this instance's crew |
| `make seed-check` | the published seed surface and `examples/config.yaml` |
| `make history-check` | this repository's publication history |
| `make doc-check` | prose pins over shipped code and docs |
| `make identity-check` | this box's identity literals, in paths and content |
| `make ops-check` | ops residue and live instance paths in tracked files |
| `make execwrite-check` | executable writes routed through `WriteExecutable` |
| `make pkgcensus-check` | censuses over `internal/posse`'s own sources (arm 1 and shared) |
| `make pkgcensus-arm2-check` | the same, for the pins built only under `posse_arm2` |
| `make pkgcensus-arm3-check` | the same, for the pins built only under `posse_arm3` |
| `make treepins-check` | every pin in this package, minus a priced exclusion table |
| `make pid-check` | the shipped PIDs under `examples/agents/` |
| `make land-check` | the launcher's own land gate, `scripts/posse-land-check`, whose fifth step vets the not-native GOOS |

The `treepins-check` row is the one door named by what it does **not** run
([ADR 0098](../../docs/adr/0098-the-treepins-door-is-the-package-minus-a-priced-exclusion-list.md)).
Every other door is a `-run` filter, so a door is the pins it names — and in
this package that key stopped paying for itself: a pin here reads a tracked
file it names as often as it walks a directory, and a named read is the
enumeration rule's own negative case, so the pins that most needed a door
were the ones no derivation could find. Seven of the nine hand-written
exemptions in `treewidedoor_qa_test.go` arrived for that one cause. Named the
other way round — the package minus `$(QA_TREEPINS_EXCLUDED)` — a pin written
tomorrow is behind the door the moment it is written, and the only hand-kept
state is the exclusion table, where every row carries its measured seconds,
the box, the date and the route that still runs it.

Six doors came off `tree-check`'s line when it arrived, and all six are still
here to type, each a subset of the one that replaced them:

- `make notes-check` — `docs/notes.d/README.md` lists every fragment (a tool)
- `make orientation-check` — no push mandate in `AGENTS.md`, and the
  `INSTALL.md` recipe that keeps it out
- `make adr-check` — ADR citations resolve, exemptions name real files
- `make corpus-check` — censuses over this repository's own Go, test and
  shipped-runnable sources
- `make register-check` — the register itself: every tree-wide pin has a door
- `make scripts-check` — censuses over `scripts/`

They are not on the table above because that table is held to
`tree-check`'s prerequisite line, membership and order, by
`treewidedoor_qa_test.go`'s arm 4 — and a reader pricing a partial run is
owed the list `make tree-check` actually runs.

The last row of that table is not a tree-wide pin and is the one door here
that runs no test. It is the gate that judges a replayed landing, and it is
on this list
because its cross-GOOS step is the only thing in the repository that compiles
the tree for the platform the development box is not: a file that builds here
and not on darwin was a tree with a green suite, a green `make tree-check`
and a green `go vet ./...`, found first by the coordinator's pre-push gate
([ADR 0087](../../docs/adr/0087-a-replayed-landing-is-judged-by-the-repos-own-check-and-may-not-turn-a-green-base-red.md)
residual 1). The Makefile's own comment on that door says the rest of it.

`make tree-check` is every one of them in one command, and it is a
prerequisite of `make test` for the reason `fmt-check` is: a full run fails
on it in seconds rather than at the end of a long package. MEASURED
2026-11-14 with the complement door on the line: 83–92s warm over three runs,
against 42–43s for the door list it replaced in the same window, so about
forty seconds for a door that cannot go stale
([ranger-base-3cpcg](../../docs/notes.d/ranger-base-3cpcg.md), which has every
reading and the twelve priced rows). It read 46.5–77.2s warm and 92.6s cold
before `treepins-check` and before five doors were added after it (MEASURED
2026-10-04,
[ranger-base-g6sb1](../../docs/notes.d/ranger-base-g6sb1.md)); the spread in
both is box load rather than the build cache. Two doors are
worth typing alone — `make crew-check` when a change touched `cmd/`,
`internal/`, `etc/`, `examples/` or any `*_test.go`, and `make notes-check`
whenever a `docs/notes.d/<bead-id>.md` fragment is added.

The Makefile is the live list: the doors are `tree-check`'s prerequisites and
nothing else, so `make -n tree-check` prints exactly what a seat would
otherwise have to type, and no count of them is written down twice.

Most doors run their pins under a `-run` filter rather than reimplementing
the check in shell, so a door cannot disagree with the pin it guards.
`fmt-check` and `notes-check` are the exceptions, and they earn it: gofmt is a
tool, and the notes pin's whole body is the same `scripts/notes-index.py
--check` the door runs.

## Running this package

Run a pin with a `-run` filter, as the NOTES.md entry-points table does. An
unfiltered run of this package type-checks and vets all three build-tag arms
and costs what a full suite arm does — 589.965s MEASURED — so type
`make treepins`, which goes through the suite wrapper and takes one of the
box-wide suite-lock slots
([ranger-base-7zng1](../../docs/notes.d/ranger-base-7zng1.md)). A bare
`go test ./internal/treepins` reaches no wrapper and therefore no slot, and is
invisible to the runs that queue.

## Adding a pin

1. Cite the decision. Name the ADR or the bead that made the rule in a
   comment, so a red tells the next reader which decision they are about to
   reverse ([ADR 0051](../../docs/adr/0051-landed-is-a-bead-id.md), "Naming a
   source file"). `make adr-check` resolves those citations.
2. Give it a door. Add the test to the matching `QA_*_PINS` variable in the
   Makefile, or add a door if none fits; `make register-check` fails until
   every tree-wide pin is named by one.
3. Name the process it protects (ADR 0006 §7). A pin that merely freezes
   current behaviour is a characterization test, and one of those once froze
   a live bug here.
4. Invert it before believing it. A pin that cannot fail on the path it runs
   is worse than none — plant the violation it claims to catch and watch it
   red.

The class is derived mechanically, and by two rules, because the two packages
reach the tree in ways with no spelling in common. In `internal/posse` a test
runs with the package directory as cwd, so reading the tree costs a climb and
the key is the one repo-root helper. Here, `TestMain`
(`configdirfence_test.go`) chdirs the binary to the repository root first, so
a pin reads the tree through a plain relative path and names no helper at
all — the key is instead a directory enumeration (`WalkDir`, `Walk`,
`ReadDir`, `Glob`) rooted at a relative path, because a reading whose file set
is not spelled in the test is a reading an unrelated change can turn red.

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for the contributor's loop and
[NOTES.md](../../NOTES.md) for the rest of the build and test entry points.
