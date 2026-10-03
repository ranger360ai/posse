## A path-keyed config lookup compared spellings, and one repo had two (ranger-base-99gww)

*bead: ranger-base-99gww (P1 bug) · subject `internal/posse/visibility.go`
(`BeadsVisibility`, `resolvedPath`), `hookfresh.go`, `secondstore.go` ·
relates rangerhq-hrz (the guard), rangerhq-qm6c (the stamp is written at
install time), ADR 0023 (identity at the dispatch path), ADR 0024 D2 ·
2026-10-03, this box (darwin 25.4.0 arm64, posse 0.5.0+1b8ffe2d, APFS
case-insensitive) · everything below is MEASURED on that date unless marked
ASSUMED*

`beads_visibility:` is keyed by filesystem path, and the key was matched by
STRING. `resolvedPath` normalized a spelling — `ExpandTilde`, `Clean`, `Abs`,
`EvalSymlinks` — and none of those folds case. So on this box a key the
operator typed as `~/src/hcn` never met the directory git spells `~/src/HCN`,
and the repo fell through `BeadsVisibility`'s last line: *unmarked is public
(fail closed)*.

Fail-closed held — public is the closed side and nothing was widened — but
the two readers disagreed about which repo they were talking about, and they
write the same file:

| who | the path it derives | what it stamped |
|---|---|---|
| `posse gates install-hooks ~/src/hcn` | the operator's config spelling | `private` |
| a dispatched launch / `posse relaunch` (L3 pre-heal) | git's, so `~/src/HCN` | `public`, reporting it had healed a WRONG wall |

Three flips in one morning on the operator's box (08:40 private, 09:06 public
after relaunches, 09:1x private). `scripts/verify-hook-freshness.sh` could
SEE it — it reads the config spelling and stats it, and a stat succeeds under
either spelling — and reported `visibility stamp is 'public' but config says
'private'` while the stamp itself could not reach the same conclusion. On a
private repo the public stamp also refuses the seats' own ops-class beads,
which is functional breakage for the people writing there.

## Three candidate shapes, and why identity won

The bead proposed two: canonicalize the case (darwin `F_GETPATH`), or
`strings.EqualFold` when the volume is case-insensitive. Both were measured
before a third was chosen.

MEASURED, by probe, on this box:

The directory is spelled `HCN` on disk; `$HOME` is written as `~` below and
the home's own components are the real spelling in both columns.

| path handed in | `filepath.EvalSymlinks` | `fcntl(F_GETPATH)` |
|---|---|---|
| `~/src/hcn` | `~/src/hcn` — the spelling it was given | `~/src/HCN` |
| `~/src/HCN` | `~/src/HCN` | `~/src/HCN` |
| `~/SRC/hCn` | `~/SRC/hCn` — every component wrong | `~/src/HCN` |
| `~` with the username in caps, `/src/HCN/.git` | unchanged, caps and all | the home's real spelling, `/src/HCN/.git` |
| `/tmp`, `/var`, `/etc/hosts`, a symlinked dir | `/private/…`, target | agrees, every case |
| a dangling symlink | error | error |

So `F_GETPATH` is correct, and correct at every component, not just the last.
It was still not taken, for two reasons that are about this codebase rather
than about the syscall:

1. It needs an **open**, and a pointer-taking `fcntl` has no helper in
   `golang.org/x/sys/unix` — it needs `unix.Syscall` with `unsafe.Pointer`.
   This repo contains no `unsafe` import and no raw syscall anywhere
   (MEASURED: `grep -rn '"unsafe"'` and `unix.Syscall` over the tree, zero
   hits each). A case fix is a poor reason to be the first.
2. It is darwin-only, so it also costs the `_darwin.go`/`_other.go` split
   that `loadavg` and `backupvolume` carry.
   MEASURED: a mode-`0000` directory answers `F_GETPATH` *permission denied*
   where `os.Stat` still succeeds — the one case where the stat-based answer
   is strictly stronger.

`EqualFold` was rejected outright: it is wrong on a case-SENSITIVE volume,
where `hcn` and `HCN` are two directories, and "is this mount
case-sensitive?" is a per-mount question that neither `statfs`'s
`f_fstypename` (MEASURED: `apfs` for both kinds) nor a filesystem name
answers.

**What shipped is the third shape: `samePath`, which asks by IDENTITY
(`os.SameFile` — same device, same inode) and not by spelling.** The question
was never "what is the canonical string" but "are these two spellings the
same directory", and identity answers it on both kinds of volume with no
probe, no `unsafe`, no syscall and no build tags. `resolvedPath` survives as
its string fast path, documented as nothing more.

MEASURED, the same two spellings under each candidate:

| volume | the truth | string `==` (the defect) | `EqualFold` | `os.SameFile` |
|---|---|---|---|---|
| case-insensitive (APFS default) | one directory | **two** ✗ | one ✓ | one ✓ |
| case-sensitive (APFS, disk image) | two directories | two ✓ | **one** ✗ | two ✓ |

Identity is the only column right in both rows. It also subsumes what
`EvalSymlinks` was there for — `/tmp` and `/private/tmp` are one inode — and
firmlinked spellings of one directory (a path under `/Users` and the same
path under `/System/Volumes/Data/Users`) now match, which they did not
before.

## The pin needs two volumes, and says so when it only has one

`internal/posse/visibilitypathcase_qa_test.go`. Whether two spellings are one
directory is a property of the MOUNT, so no single volume can run the whole
file: four arms need a case-insensitive one (the lookup under either
spelling, the reverse lookup, the fail-closed control, and the stamp — two
installs under the two spellings rendering byte-identical hooks), and
`TestBeadsVisibilityKeepsTwoCaseDistinctReposApart` needs a case-sensitive
one. Each **skips with a line naming the volume it did not get**, after
proving the fixture by `os.SameFile`, rather than passing vacuously.

MEASURED on both, green either way and neither way green by accident:

| `TMPDIR` on | folding arms | distinct-repos arm |
|---|---|---|
| this box's APFS (case-insensitive) | 4 PASS | SKIP |
| a case-sensitive APFS `hdiutil` image | 4 SKIP | PASS |

MUTATION-CHECKED, both arms, which is the point of the two-volume shape:

- `samePath` → `resolvedPath(a) == resolvedPath(b)`: reds the three folding
  arms on the case-insensitive volume. The fail-closed controls stay green,
  as controls must.
- `samePath` → `strings.EqualFold(resolvedPath(a), resolvedPath(b))`: green
  on every arm the case-insensitive volume can run — which is why that
  mutation would have shipped — and **reds the distinct-repos arm on the
  case-sensitive image**.

The case-sensitive volume was a 2 GB `hdiutil create -fs "Case-sensitive
APFS"` image in the session scratchpad, attached `-nobrowse`, detached and
deleted afterwards. A 20 MB one is too small: `internal/posse`'s test binary
links into `$TMPDIR` and the run fails as `[build failed]`, not as a test
failure.

## Verified end to end, through the shipped binary

The pin is a unit of the package; this is the operator's own command. A
throwaway posse home in the session scratchpad (`RHQ_HOME` points at it), a
config carrying one key — the repo spelled in LOWER case — and a git repo
whose directory is spelled `HCN`, which is the real instance's shape. Then
`posse gates install-hooks` under each spelling, against a binary built from
this change and one built from `HEAD`. MEASURED 2026-10-03:

| binary | `install-hooks <…>/hcn` (the operator's) | `install-hooks <…>/HCN` (git's, so the launch's) |
|---|---|---|
| `HEAD` | `private — config beads_visibility:` | **`public — unmarked in config beads_visibility:`** |
| this change | `private — config beads_visibility:` | `private — config beads_visibility:` |

The pre-fix row is the flip itself, in two commands: the hook file left
behind reads `posse_beads_visibility='public'` after the second one. With the
fix it reads `private` whichever spelling installed it, and the two renders
are byte-identical — which is what stops the L3 probe reading the wall as
"ours but stale" and re-stamping it on the next launch.

Nothing live was touched: the fixture repo, the config and both binaries were
in the session scratchpad, and no `install-hooks` ran against a real
checkout.

## The same comparison shape elsewhere

Every reader keyed by a config path now compares by identity:
`BeadsVisibility` (and so every renderer of the commit-guard stamp, which all
reach it through `hookRepo`), `SweepHookWall`'s dedup of
`beads_visibility:` keys, and `SweepSecondStores`' dedup of `beads:` stores
and its redirect-points-at-itself check. The two dedups moved from a
`map[string]bool` to a slice and `anySamePath`, because the key a map needs
is a canonical string and the whole finding is that no canonical string
exists for this question; both iterate config entries, of which an instance
has a handful.

**`absResolve` (seatbelt.go) was deliberately left alone.** It is documented
as "the path the sandbox will match on", and seatbelt profiles match the
literal strings posse renders into them — folding there changes what the
kernel is asked, not what posse concludes, and is a different question from
this one. Noted as unexamined, not as clean.
