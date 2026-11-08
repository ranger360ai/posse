# The provenance row that could never be green off the laptop (ranger-base-eiwhk)

Found on the ADR 0080 host, 2026-11-07 — a fresh home whose only bead repo was
a project repo. Same class as ranger-base-ctath, reached from the other side.

## The reading

`posse gates <persona>` and `posse status`, MEASURED 2026-11-07:

```
wall renderer (gates) · 0.5.3+<sha> · UNKNOWN whether this binary carries the
wall renders and the launch-time checks that landed: no checkout of
github.com/ranger360ai/posse here holds <sha> (looked in <project repo>)
```

A posse checkout holding exactly that sha was in the account's src dir — the
one `make install` had been run from. Nothing was stale, nothing was missing,
and the remedy the line names (`make install`) had just been run. The reading
simply never looked there.

## Why

The candidate list `FindLauncher` scans was `beads:` plus the process cwd
(`launcherCandidates`, internal/posse/launcherlag.go), and the wall renderer
composed a second copy of the same two (`(*App).WallRenderer`). Both of those
sources answer a different question — *which repos does this instance
dispatch* — and that question has no reason to mention posse.

On the laptop this crew was built on they coincide, because posse's own repo is
one of the queues. On an instance whose crew works on something else, which is
the normal case off the laptop, the checkout is on disk and in no list the
reading reads, so the row can never be green: a permanent abstention, on every
pass, forever. G11 goes with it — `GovRows` raises nothing from an unreadable
reading, so the governance surface is silent too, by design and for the wrong
reason.

## The fix: two candidates, and the order is the safety argument

```
extra (promote's source) · posse_repo: · beads: · cwd · BuildSource
```

- **`BuildSource`** — the build's own answer, which needs no configuration.
  `make build` (Makefile `BUILD_SOURCE`) and `scripts/clean-build.sh` (`make
  release`, and so `make install`) know the directory they are building from
  and stamp it beside the sha through
  `internal/posse.BuildSourceSymbol`. clean-build stamps `$repo`, the
  operator's checkout, and not `$src`, the temp worktree it compiles in and
  deletes on the way out.
- **`posse_repo:`** — the operator's answer, for the install the stamp cannot
  name: a binary copied in from another box, a release tarball beside a
  checkout that should be counted, or a box with two clones where the reading
  must prefer one. Read AHEAD of `beads:`, because it is the candidate that was
  typed on purpose; a dial that loses to whatever `beads:` lists first has no
  authority.

**The stamp is LAST, and that is load-bearing.** A candidate appended at the
end can only change the outcome when every earlier one failed, so it turns an
UNKNOWN into an answer and can never move a reading that `beads:` or the cwd
already settled. That is what makes baking a path into a binary safe to read
at all.

**And a candidate is tested, never trusted** — unchanged from before: it has to
be a git checkout, declare the module, and HOLD the commit the binary names.
So a stale stamp (the checkout moved, was deleted, or was replaced by an
unrelated tree) degrades to the abstention that was already there and cannot
produce a wrong number.

**Not stamped on anything that leaves this box.**
`scripts/release-artifacts.sh` cuts from a throwaway worktree on a machine the
reader does not have, so the path would be dead on arrival, and a published
binary should not carry a stranger's home directory. A release's answer is its
TAG (ranger-base-ctath). The cage's inner binary is the same argument from the
other side: a host path means nothing inside a container.

**Whitespace is refused, not quoted.** `go` splits the `-ldflags` VALUE on
spaces, so a checkout path containing one would become a second linker flag —
the injection shape ranger-base-ctath hardened the release script against. The
Makefile guards with `$(words $(BUILD_SOURCE))` and clean-build.sh with a
`*[[:space:]]*)` case; neither stamps, and the reading is then exactly what it
is today.

## One list

`(*App).WallRenderer` now reads `launcherCandidates` instead of composing its
own copy. That duplication is how this bead would have been fixed on `posse
status` and left open on `posse gates` — which is the surface the symptom was
read off.

## The second half: a rejected candidate is named

With five sources feeding the list, a silently dropped candidate makes
`posse_repo:` a dial whose failure to take effect is invisible — an operator
who mistypes it, or whose build directory has moved, would read `looked in <the
other repos>` and conclude the setting was ignored. `FindLauncher` now names
every non-empty candidate with what was wrong with it:

```
(not a git checkout) · (not a posse checkout) · (posse, but it does not hold that commit)
```

The bead's own line gains the middle one: the single directory it looked in was
not a posse checkout at all, which is the fact that says the answer was never
going to arrive from `beads:`.

## Pins

| where | holds |
|---|---|
| `internal/posse/buildsource_qa_test.go` | the order, the bead at the `FindLauncher` seam with its unstamped control, the three stale-stamp shapes, `posse_repo:` ahead of the queues, and the one list |
| `internal/treepins/buildsource_qa_test.go` | the Makefile and clean-build.sh stamp through the symbol constant, `$repo` and not `$src`, both whitespace guards — and release-artifacts.sh, cagestale.go and ci.yml do not stamp it |
| `cmd/posse/gatesbuildsource_qa_test.go` | `posse gates` end to end on the bead's box, stamped and unstamped, plus a stamped directory that is not posse |

Both of the last two are needed, and neither is redundant: `-X` at a symbol
that does not exist is SILENT (exit 0, the variable keeps its default —
`cagestale_test.go` measured it for the `Build` stamp), so the text pin says
the build lines stamp the constant and the executable pin says the constant
names a real symbol.

MUTATION-CHECKED 2026-11-08. Dropping the `buildSource` append from
`launcherCandidateList` reds the seam arm and all three stale-stamp arms (the
latter report "looked in no directory this instance knows about", which is the
honest reading of a list with nothing in it). Restoring `WallRenderer`'s own
candidate list reds the `posse gates` arm alone, with the bead's exact line in
the failure output.
