# ranger-base-dw2du — D2 pins every bind, and D2(a) reads the wall root in both directions

ADR 0100's D2 came out of the re-read of its own D2/D4 with two holes in the
rule rather than in the code, and one wording fix in D4. This is what each
cost to close and what was measured closing it.

## 1. D2(b) was asked of the wrong set

The rule said "every source that DIFFERS from its destination must be
pinned". The in-place binds — the `--bind p p` shape 1 renders for each write
grant, and every in-place `--ro-bind` — were judged by neither arm of the
decision, because D4(a)'s destination arm skips the same equality.

They are not safe for being in place. bwrap resolves the same string again at
`mount(2)`, so a component of a grant's path that a walled seat can rename is
togglable between a directory and a link, and the render's own `absResolve`
an instant earlier is the check-then-use the record rejects A2 for. A grant
whose loose component is pointed at the wall root is a read-write mount of
every seat's private sources at the grant's own name, at the next launch,
with no race to win.

The fix is one condition in `bwrapRefuseUnpinnedSource`: skip a `--tmpfs`
(no source at all) and nothing else.

**Wider than the re-read asked for, deliberately.** It asked for "every rw
bind". This covers every bind: an in-place `--ro-bind` redirected the same
way hands out the same tree read-only, which for private history, shell
snapshots and the seat store is a confidentiality break, and a mode exemption
is an exemption to maintain. The existing rule already made no mode
distinction for the sources that differ.

MEASURED 2026-11-21, this box (Linux 7.2.15), over all five argvs the package
renders: **150 in-place binds, every one pinned.**

| render | in-place binds | loose |
|---|---|---|
| seat, claude | 55 | 0 |
| seat, codex | 31 | 0 |
| seat, grok | 50 | 0 |
| landed-check judge | 7 | 0 |
| wall venue | 7 | 0 |

Read at `da79a784`. The COUNT is not a property of the decision and moved
twice while this bead was in flight: it was 160 over the same five renders at
`16c844cb` and `4fe452df`, and `ranger-base-bncbk` turned two in-place binds
per render into masks. What does not move is that none of them is loose. Quote
the count with its base or not at all.

That is also the answer to the one thing the re-read left ASSUMED — whether
any render today has a grant nested two or more components inside another
with a non-mount component between them. No. What the rule asks of an
in-place bind is only about its ancestors: the path itself is the destination
of the mount that binds it, and a grant one component inside another has the
outer grant for its parent. The shape that reds is a nest **two** deep with
nothing bound between, which is what the refusal pin renders.

## 2. D2(a) read the wall root in one direction

"Refuse any write grant at or above the wall ROOT" was the whole rule while
the attack in view was a grant holding a COMPONENT of a source. A grant
INSIDE the root is neither at nor above it, and it is the same hole from
below: a grant on another seat's wall dir, or on a directory within one, is a
write grant on that seat's private bind SOURCE. For a member a seat reads
before every command it runs, that is code execution in the victim — no
rename, no race, no leftover.

`bwrapRefuseWallRootGrant` now refuses `insideDir(wallRoot, G)` as well as
`insideDir(G, wallRoot)`, G resolved and the root composed, which is the same
two sides the first direction already compared.

MEASURED 2026-11-21 over the same five argvs: **0 of 32 grants lie inside the
wall root**, while every one of the five has bind sources that do. The
negative case is why the widening reds nothing: a seat's own private
DESTINATIONS are grants, but its own private SOURCES are granted by no
render — they are bound, which is the whole of D1. Both halves are pinned
together, because either alone is vacuous.

## 3. D4's wording

(b) said a qualifying render "binds that table". It now says it makes every
member PINNED in that render. The readings part when the grant is an
ANCESTOR of `<cfg>` rather than `<cfg>` itself: a mount on each member does
not stop a rename of `<cfg>`. For the default home D2(a) already refuses `~`
and `~/.cache`; a `CLAUDE_CONFIG_DIR` elsewhere has no such cover, and there
the predicate is what says so. No code moved — the gate already asks the
predicate.

(c) lists two kinds of `<common>` writer, and it still does. The re-read
named a third — a seat linked in another repository whose store of record is
this one, granted that repository's git dirs whole and so holding no mount of
its own over `<common>/objects`. It was true when the re-read was written, on
2026-11-15, and it was not true by the time this landed: `ranger-base-kfrpv`
removed the grant and `ranger-base-gdgbb` put a dated note in ADR 0099 saying
D1's session-repo scope is complete and is not to be reopened for the store
repo. Writing the third kind in would have reintroduced, as a premise of
this record, the sentence that landing had just struck from five other ADRs.
So (c) carries a dated READ instead: the class, why it has no members, and
which paragraph gains a third kind if a seat is ever granted write into
another repository's git again.

**That is the whole reason this bead's work was rebuilt once.** The branch
was cut at `16c844cb`, both commits were made, the suite was green, and then
`main` moved three commits — `kfrpv`, `nywsk`, `gdgbb` — one of which made a
sentence in the deliverable false. The two counts below were re-measured on
`4fe452df` and are unchanged (the fixtures' grant sets are the same, since
their store of record is not in another repo); the item-3 text was not.
A design item that was read and approved six days earlier is worth re-reading
against `main` the hour it lands.

## Mutations

Each applied alone to `internal/posse/bwrap.go` and restored with the file
checksummed back to its baseline (`scripts`-free, a script by path under the
session scratchpad).

| mutant | reds | survivors |
|---|---|---|
| D2(a)'s inside-the-root arm disabled | `TestQAWallRootGrantIsRefusedInsideTheWallRootToo` alone | none |
| the `Src == Dst` skip restored in the source arm | `TestQAWallPinnedSourceRefusesATwoDeepUnpinnedGrant` alone | none |

The second is the one worth reading: the in-place CENSUS stays green under
it, which is exactly what a census is for — it measures today's renders and
cannot red a rule that is not being asked. The refusal pin is what holds the
rule.

## Note on the commit this branch also carries

The branch re-commits the parked `ff876c6d` (ranger-base-gn050) onto main
with its D4(a) reading reworded, which is ranger-base-o2pjf's ask: the
census miss is named by bead id and nothing else, in one place, because the
surface it lies on is an open residual. Two things found while doing it are
on that bead — the shipped judge reads the staged diff as ONE stream, so a
markdown list item in an early file swallows the added lines of every later
file until a blank line, and a `//`-prefixed line opens no block at all.
