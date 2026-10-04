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
| `make notes-check` | `docs/notes.d/README.md` lists every fragment (a tool) |
| `make adr-check` | ADR citations resolve, exemptions name real files |
| `make corpus-check` | censuses over this repository's own Go and test sources |
| `make register-check` | the register itself: every tree-wide pin has a door |
| `make scripts-check` | censuses over `scripts/` |
| `make pid-check` | the shipped PIDs under `examples/agents/` |

`make tree-check` is every one of them in one command, and it is a
prerequisite of `make test` for the reason `fmt-check` is: a full run fails
on it in seconds rather than at the end of a long package. MEASURED
2026-10-04 on the development box: 46.5–77.2s warm over three runs, 92.6s
cold, the spread being box load rather than the build cache
([ranger-base-g6sb1](../../docs/notes.d/ranger-base-g6sb1.md)). Two doors are
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
