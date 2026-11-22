# beadsHome answers a spelling, and every wall resolves it (ranger-base-1b8yv)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, from this bead's worktree.
Discovered from ranger-base-swdn0, which read the macos leg. Nine QA pins over
the store-repo git carve-out (ranger-base-kfrpv, ADR 0086 D3) read NOTHING on
darwin, and the bead that filed it asked a second question the pins could not
answer: does PRODUCTION render a wall rule, or decide containment, from the
unresolved spelling.

## What was red, and why it printed nothing a reader could use

Each of the nine guarded itself with `beadsHome(dir) != absResolve(<the store
this fixture built>)`. On darwin `t.TempDir()` lands under `/var/folders/…`,
`/var` is a link to `/private/var`, and `beadsHome` answers a SPELLING — so the
two sides were one directory named two ways and every arm abstained with
"measures nothing". MEASURED by READING ci run 44974829306 (b2509fba), job
137062263947, "test (macos-latest, 1)".

The abstentions printed the UNRESOLVED want beside the got, so **got and want
read identical in the log**: the one reader who followed the row was told the
two paths were the same.

REPRODUCED ON LINUX, which is what made this fixable here: `TMPDIR` pointed at
a symlink is the same condition with none of darwin's specifics. MEASURED
2026-11-21 — the same nine rows, at the same nine lines
(recordreachgrant_qa_test.go:94/153/292, seatbeltdenygitgrant_qa_test.go:256
and :402/:420, storerepogitwall_qa_test.go:132/167/217/250), then all nine
green after the fix and green again under an ordinary `TMPDIR`.

## The contract, which is the decision this bead owed

`beadsHome` returns **bd's own spelling and not a resolved path**, and that is
now written where it is implemented (beadloss.go) and pinned
(`TestQABeadsHomeAnswersBdsOwnSpellingAndNotAResolvedPath`). Its first readers
hand it to bd as `BEADS_DIR` and to `git -C`, which resolve it themselves and
for which the spelling is the operator's own — bd 0.49.1 refused every
`BEADS_DIR` under /tmp by name and 0.50.3 was measured refusing none, so
resolving here would hand the pinned bd a different path than the operator
configured, for a verdict that moves with the version. No wall would gain by
it, which is the next section.

## The production answer: no wall renders from the spelling

Every consumer whose answer is a wall literal or a containment decision
resolves first, and that is asserted rather than audited now
(`internal/posse/beadshomespelling_qa_test.go`, over a store reached through a
symlink on any host):

| producer | what resolves |
|---|---|
| `SeatbeltWritable` (L2, and L3 through it) | `add`'s `absResolve`, so the SBPL `subpath` and the `--bind` carry the real path |
| `CageMounts` (L4) | `cageOverlay`'s `src := absResolve(p)`, with `cageCovering` resolving every destination it compares |
| `recordTargets` / the reach gate | `recordGitTargets`' `underDir`, `anyUnderDir`, `containerReachRow`'s `absResolve(home)` |
| a self-sandboxing runtime's `--add-dir` | `codexWritableRoot`, where an unresolved component is a session that runs no command at all (ranger-base-k62e) |

So the bead's ASSUMED — "real darwin homes and stores are under /Users, which
is not a link, so live seats are unaffected" — is no longer load-bearing
either way: a store under `/tmp` or `/var` on darwin gets rules spelled at the
path the kernel matches. (This box cannot read a live darwin seat, so the
/Users claim itself is neither confirmed nor refuted here.)

MUTATION-CHECKED, three mutants, each run against the new pin and reverted:
`add` not resolving reds the set arm and the profile arm (and prints the dead
`(subpath "…/link/store/.beads")` line the hardening question was about);
`cageOverlay` not resolving reds the mount-spelling arm; `codexWritableRoot`
made an identity function reds the launch-line arm and the k62e refusal beside
it.

## Not closed here

- ranger-base-ayubz — at the container tier the store's MOUNT is resolved
  while the `BEADS_DIR` forwarded into the cage is `beadsHome`'s spelling, so a
  store behind a link is named inside the cage at a path no mount holds, and
  `containerReachRow` reads the row Enforced anyway. Read from source, not
  measured on a live L4 seat; L2 and L3 are unaffected, and the bead says why.
- The macos verdict itself. The nine are green here under both spellings of
  the condition, but the leg that files them is CI's: name the next
  macos-latest arm 1 run on this bead when it lands.
