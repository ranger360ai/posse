# The host's opens of the refusals trail go through no link (ranger-base-legou)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, from this bead's worktree.
Discovered from ranger-base-xab99, which closed the RENDER's half (the Keep
leaf's bind). This is the host's own half: four uncaged Go opens of the same
two leaves, none of which passes through that render.

## What was open

- `RefusalsLog` (egress.go) opened `gates/<p>/refusals.log` with
  `O_CREATE|O_APPEND|O_WRONLY`. A link at the leaf was followed, and over a
  DANGLING link the host created the link's target, 0644. It runs on every
  container launch (`RenderEgress`).
- `appendCanonicalRefusal` (refusalfold.go) appended to the same leaf through
  a link. What it appends is `session=<s> <seat bytes>` verbatim — a
  canonical log linked to a shell rc file turned a spool line into a command
  the operator's next login shell runs.
- `EnsureCageSpool` created the spool through a link, and
  `FoldRefusalsSpool` READ it through one, copying a link target's bytes
  into the canonical log.

## The fix

One helper, `openRefusalLeaf` (internal/posse/refusalleaf.go), used by all
four: `O_NOFOLLOW|O_NONBLOCK` on the leaf, `O_CREATE|O_EXCL` on the create
path with a retry without `O_CREATE` on EEXIST, then `fstat` of the
descriptor actually opened and a refusal unless it is a regular file with one
link. The judgement is of the fd, never of an earlier lstat. Every refusal
wraps one sentinel, "not a regular single-link file", and names the path. A
refused render fails the launch; a refused fold returns before
`writeSpoolCursor`, so the cursor does not move and the next fold over a
repaired leaf still carries the lines.

`O_NONBLOCK` is for a planted FIFO, which `O_NOFOLLOW` still opens: a write
open with no reader now fails ENXIO instead of blocking the fold, and a read
open returns at once and is refused by the fstat.

## Mutants (MEASURED 2026-11-21, linux)

`internal/posse/refusalleaf_test.go` plants a symlink, a dangling symlink, a
hardlink and a FIFO at each site, under a timeout that opens the FIFO's other
end if the call blocks.

| mutant | reds |
|---|---|
| drop `O_NOFOLLOW` | symlink + dangling rows at all four sites, the cursor pin |
| drop the nlink check | every hardlink row |
| drop `O_NONBLOCK` | every fifo row (by the timeout, not a hang) |
| drop the IsRegular check | the spool-read fifo row — the one site whose open succeeds |
| drop `O_EXCL` | nothing — `O_CREATE\|O_NOFOLLOW` over a dangling link is already ELOOP; kept as the second wall, PrepTouch's shape |

`GOOS=darwin go vet` is clean for both arms the change touches; the darwin
behaviour of the plants is not measured here.

## Not closed here

- ranger-base-byqv0
