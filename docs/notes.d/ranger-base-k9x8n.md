# install-hooks resolves its target before anything reads it (ranger-base-k9x8n)

ranger-base-hgq2k's live finding, fixed 2026-11-08: `posse gates install-hooks`
took `dir` as typed, and `hookRepo` hands a MAIN checkout's dir straight back
(`LinkedGitDirs(dir) == nil`), so the `.` default — the form every degraded
line, `scripts/verify-hook-freshness.sh` and most of INSTALL.md prescribes —
carried the literal `.` through everything downstream. One line in
`cmd/posse/main.go`, `filepath.Abs` over the parsed target, where the
`managed-hooks` arm above already does it with the same complaint written
beside it.

## Two consequences, not one

**The one that was filed.** ADR 0081 D4's refusal named the repo as `.` and
then prescribed that `.` as a `beads_visibility:` key. Such a key is read by
`samePath` → `resolvedPath` → `filepath.Abs`, i.e. against the READING
process's cwd, so the remedy was inert where it was printed and a wildcard
everywhere else (ranger-base-hgq2k's fragment has that measurement).

**The one the fix turned up.** `dir` also reaches `BeadsVisibility` and
`DeriveIdentityLiterals` through `hookRepo`, and `identityRedirectTarget`
JOINS a relative `.beads/redirect` target onto it — so ADR 0024 D2 check 3's
instance-path literal was derived from the cwd-relative spelling.

MEASURED 2026-11-08, darwin 25.6.0, posse built from `39e194cd`, scratch home
and scratch repo whose `.beads/redirect` reads `../store/.beads`:

| install | the literal it stamped, in all 3 places |
|---|---|
| no argument, cwd = the repo | `posse_check 'instance-path' '\.\./store'` |
| the same repo, absolute argument | `posse_check 'instance-path' '<abs>/store'` |

An identity check matching the two-character string `../store` and not the
instance it exists to catch — and a wall differing by those three lines from
the one a launch renders, which the L3 probe reads as "ours but stale" on a
wall nobody moved (ranger-base-up22's class, one case over). With the
absolutize the no-argument render is byte-identical to the absolute one.

This arm was the only entry point where a relative `dir` could reach that
derivation: `RenderSessionHooks` and the l3 probe take the session dir, and
`queuejsonl` the configured queue repo.

## The pins

- `TestQAInstallHooksRefusesAForeignWallInTheCurrentDirectoryToo`
  (`cmd/posse/commitwallowner_qa_test.go`) gains the `strings.Contains(out,
  repo)` assertion its explicit-argument sibling already made — the gap
  ranger-base-hgq2k named. The repo path is compared RESOLVED, because the
  binary absolutizes against its own cwd and `t.TempDir()` hands back a path
  through darwin's `/var` symlink.
- `TestQAInstallHooksRendersTheSameWallForARelativeAndAnAbsoluteTarget`
  (`cmd/posse/installhooksrelativetarget_qa_test.go`, new) holds the second
  consequence, and holds it as BYTE EQUALITY of the two rendered bodies rather
  than as a grep for the path: the identity half of ADR 0023 is byte-for-byte,
  so one equality covers every literal the derivation adds later.

Both mutation-checked: with the `filepath.Abs` removed, each reds with the
`.`-derived line named in its output; the explicit-argument sibling stays
green either way, which is why the gap survived its own close.

## What this does not settle — ranger-base-5wq6g

`filepath.Abs` does not resolve symlinks and nothing downstream does, so TWO
ABSOLUTE spellings of one repo still render two different walls. MEASURED the
same day, same binary with the fix, one real dir and a symlink to its parent,
both listed in `beads_visibility:`: `install-hooks <real>/repo` and
`install-hooks <link>/repo` wrote walls differing in exactly three lines, all
of them `posse_check 'instance-path'`. On darwin the case reaches a plain
`t.TempDir()` fixture too — `Getwd` is physical (`/private/var`) and a typed
`TMPDIR` path is not (`/var`) — which is how it was noticed, as a red on the
new pin before its root was resolved.

Not fixed here and not a cleanup: the three renderers of that wall
(`install-hooks`, `RenderSessionHooks`, the l3 probe) each take their `dir`
from a different place and must take the same direction, and the probe's
comparison is the property a direction could cost. Handed to architecture as
ranger-base-5wq6g with the three candidate shapes. Not security: the wall is
present and both spellings name the real instance path.
