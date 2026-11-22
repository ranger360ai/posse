# The refusals trail's parent directories are walked through no link (ranger-base-byqv0)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, from this bead's worktree.
Residual of ranger-base-legou, which guarded the LEAF opens only.

## What was open

`O_NOFOLLOW` guards the last component of a path and nothing above it. The
leaves legou guarded sat under directories made by `os.MkdirAll` and
resolved by the kernel's ordinary walk:

- `openCanonicalRefusals` (egress.go): `MkdirAll(gates/<p>)`, then the
  guarded open of `gates/<p>/refusals.log`. A link at `gates` or `gates/<p>`
  made the host create and append to a `refusals.log` in a directory the
  planter chose.
- `EnsureCageSpool` and the fold's spool read (refusalfold.go): the same
  shape under `cages/<p>/<s>`.
- `writeSpoolCursor`: `MkdirAll` plus `os.WriteFile` on
  `gates/<p>/spool-cursors/<s>` — unguarded at the leaf too, and
  `os.WriteFile` TRUNCATES what a leaf link names (hoover's note on the bead).

## The fix

`openRefusalLeaf(root, path, flag, create)` now takes a root the caller
trusts — the App's `StateDir`, the operator's own, which may itself be a
link the operator made — and walks every component below it with
`openat(O_DIRECTORY|O_NOFOLLOW)` relative to the previous directory's
descriptor, making a missing one with `mkdirat` when creating. The leaf is
then opened relative to the last directory's descriptor and judged on the fd
exactly as before. Nothing below the root is resolved by name from the top
again, so there is no window between judging a parent and using it. A link
or a non-directory at a parent is refused with a second sentinel, "not a
real directory", naming the path.

The cursor's read and write go through the same open. The write truncates on
the fd AFTER the fd is judged: `O_TRUNC` in the open would cut a hardlinked
target before the refusal, so `openRefusalLeaf` refuses that flag outright.

## Pins and mutants

`internal/posse/refusalparent_test.go` (arm 2): a link to a real directory, a
dangling link and a regular file at every parent component of every site
(42 rows), the leaf plants on the cursor's read and write (10 rows), and a
cursor round trip. MEASURED 2026-11-21, linux: dropping the directory
open's `O_NOFOLLOW` reds all 28 link rows; dropping its `O_DIRECTORY` reds
the regular-file row at each site's deepest component (5); truncating in the
cursor's open reds the cursor write's hardlink row; dropping the cursor's
`Truncate` reds the round trip.
