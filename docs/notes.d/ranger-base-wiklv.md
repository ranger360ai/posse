# Check 4's file boundary was already fixed; what still reproduced it was the install (ranger-base-wiklv)

No code change to the judge. ranger-base-wiklv describes
ranger-base-4og5l's bug exactly — a notes fragment ending in a bare-id
bullet, refused because the NEXT staged file's first added line continued
the bullet's unit — and asks for the symmetric escape to be pinned too.
Both were in the tree before this bead was dispatched: `bfc068fa` (4og5l)
keeps the diff headers so a file boundary blanks to a unit-closing blank
line, `eb3f34fc` (pdcf6) makes the header reading positional, `e3a389ac`
(tcho6) pins the guard, and `9c2a95d0` (cso5z) pins the upward direction
with its false-negative arm.

Box, 2026-11-22: Linux 7.2.15-1-omarchy, go1.27.3, 16 cores, uid 1001, tree at
262d7910.

## Chronology, because it decides what this bead was

| when (2026-11-21) | what |
|---|---|
| 00:33 | ranger-base-4og5l filed (found while landing ranger-base-zm03j) |
| 01:28 | **ranger-base-wiklv filed** (found while landing ranger-base-bncbk) |
| 08:17 | 4og5l closed — `bfc068fa`, the fix |
| 11:14 | ranger-base-cso5z filed: the same bug, against a hook whose binary predated the fix |
| 15:12 | cso5z closed — no code change, plus the upward pin |

So this bead is **concurrent discovery**, not a re-report: two seats found
one bug 55 minutes apart, 6h49m before either could have seen a fix. That
is worth no mechanism. ranger-base-cso5z, three hours after the fix landed,
is the one that cost a seat for an install skew — and on 2026-11-22 that
same skew is still the only thing on this box that reproduces the bug.

## The bead's own repro, through two binaries

The content arm's stream for a `## Not closed here` section whose last line
is a bare open id, followed by a new `internal/posse/*.go` whose first
added line is `package posse`. Fed to `posse gates vuln-private-scan` with
a live open `vuln-private` id in the store, MEASURED 2026-11-22:

| judge | stream | rc |
|---|---|---|
| `posse` on PATH, 0.5.3+00601dfe | no headers (pre-fix arm) | 1 — refused |
| `posse` on PATH, 0.5.3+00601dfe | with headers (this arm) | 1 — refused |
| built from this tree | no headers (pre-fix arm) | 1 — refused |
| built from this tree | with headers (this arm) | **0 — clean** |

Row 4 is the fix: the bead's refusal is gone. Row 3 is the hook-vs-binary
skew `vulnPrivateUndiff` already documents — an installed hook older than
the binary greps the headers away, so the judge reads the pre-fix stream
and the bug stays open until that hook is re-rendered. Rows 1 and 2 are the
live condition on this box: that binary renders the pre-fix hook *and*
judges it.

## The skew nothing in this file's comments had named

Neither order `vulnPrivateUndiff` discussed is the one a seat meets. A hook
is rendered FROM the posse on PATH at every session create, so hook and
binary agree and the hook wall reads green; the binary is what lags the
tree, and in this arm the binary is the judge. The refusal on screen and
the fix in the source can therefore both be current and still disagree.

MEASURED 2026-11-22 from the seat whose repro had just red: the installed
`posse` was 0.5.3+00601dfe, built 2026-11-21 06:26 — 150 commits behind
main — and `posse gates <persona>` broke those out by what they judge. Its
"judging a commit-time gate" row named `bfc068fa`, `eb3f34fc` and
`e3a389ac`: this fix and its two corrections, and nothing else. The remedy
it printed is `make install`, which is an operator's hand and no seat's
(guardrail 3), so the false refusal is live on this box until that runs.

That reading is what every commit-guard refusal already ends by pointing at
(`posse_judged_by`, ranger-base-li3up, whose cost was this same arm), and a
freshness door that would refuse instead was priced and declined
(ranger-base-v81vo §4: red for every seat for the whole gap between a
landing and an install, over a condition no seat can fix). So what landed
here is the pointer, in the two files a reporter of this bug reads: the
binary-vs-tree direction in `vulnPrivateUndiff`'s skew paragraphs, and a
line on the boundary table saying it grew no row for this bead and why.
