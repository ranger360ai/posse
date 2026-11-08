# One repo, one identity wall, however its path is spelled (ranger-base-yx73g)

ADR 0082 D1+D2, built 2026-11-08. `canonicalPath` lands beside `resolvedPath`
and `samePath` in `internal/posse/visibility.go`, and ADR 0024 D2 check 3's
instance-path literal is rendered through it.

ranger-base-k9x8n's fragment ends with the half it could not close: it
absolutized `install-hooks`' argument, so a relative `dir` stopped reaching
the derivation — but two ABSOLUTE spellings of one repo still rendered two
walls, and the second reads as "ours but stale" on every launch (the L3
probe's identity half is byte-for-byte, ADR 0023). That is this bead.

## What changed

`DeriveIdentityLiterals` now reads

```go
repo := canonicalPath(filepath.Dir(target))
add("instance-path", canonicalAbbrevHome(repo))
add("instance-path-abs", repo)
```

— D2's "applied once, at the literal, not at the caller". Canonicalising
`dir` at `commitGuardLiterals`' entry would fix a relative redirect and leave
an absolute one written through a symlink, and would move the `dir` that
`hookRepo`, `BeadsVisibility` and `installHook` see. `hookRepo`, the three
callers and `commitGuardLiterals` are untouched; every renderer already
funnels through that one derivation, so canonicalising inside it covers all
three by construction.

`canonicalPath` is the RENDER ("which one string do we write?");  `samePath`
stays the COMPARISON ("do these name one directory?"), answered by inode
identity. They are not merged: the first has to pick one of two strings the
second calls equal.

`AbbrevHome`'s body moved into `abbrevHomeAgainst(home, p)` so
`canonicalAbbrevHome` can ask the same arithmetic against the home's
canonical spelling. One implementation, because the two `~` forms are
compared byte for byte.

## The platform halves

`canonicalpath_darwin.go` opens the directory and reads
`fcntl(fd, F_GETPATH)` — one name per vnode. `canonicalpath_other.go` has no
reading of its own, because D1's non-darwin half IS the fallback
(`EvalSymlinks` over the absolute cleaned path, i.e. `resolvedPath`).

MEASURED 2026-11-08, darwin 25.6.0, go1.26.10 darwin/arm64, APFS
case-insensitive, scratch dirs `real/Repo` and `link -> real` — reproducing
ADR 0082's table row for row:

| spelling handed in | `EvalSymlinks` | `fcntl(F_GETPATH)` |
|---|---|---|
| `link/Repo` | `real/Repo` | `real/Repo` |
| `real/repo` | `real/repo` (case kept) | `real/Repo` |
| `LINK/REPO` | `real/REPO` (case kept) | `real/Repo` |
| `/tmp` | `/private/tmp` | `/private/tmp` |
| `/var/folders` | `/private/var/folders` | `/private/var/folders` |
| a missing path | error | error (the open fails) |

Two details that are not tidiness:

- **`O_NONBLOCK`.** This is reached from `DeriveIdentityLiterals` on the
  launch path, and `open` of a FIFO for read with no writer never returns —
  ranger-base-fvfve's wedge, one call below, at `.beads/redirect`.
  `identityRedirectTarget`'s `isRegularFile` guard protects the redirect
  FILE; nothing protected the DIRECTORY this opens.
- **stdlib `syscall.Syscall` for the fcntl, `x/sys/unix` for the rest.**
  F_GETPATH's third argument is a buffer, and x/sys/unix exports only
  `FcntlInt`/`FcntlFlock`/`FcntlFstore` on darwin — it has no `func Syscall`
  in its darwin build at all. The pointer-to-uintptr conversion is written
  inline in the call, the one form unsafe's rules and `go vet`'s `unsafeptr`
  check sanction. This is the repo's first `unsafe`.

## The pin, and the two mutants that red different arms

`internal/posse/canonicalwall_qa_test.go` (arm 1). One repo with a RELATIVE
`.beads/redirect`, rendered through `a.commitGuardLiterals` from three
spellings — the real dir, a symlink to its parent, and on darwin a
case-changed spelling — asserting the three `CommitGuardHook` bodies are
byte-identical. Byte equality and not a grep, for the same reason the sibling
in `cmd/posse/installhooksrelativetarget_qa_test.go` gives.

The redirect is `../store/.beads` and not `../../store/.beads`:
`filepath.Clean` collapses a `..` against the segment in front of it, so a
redirect that climbs ABOVE the varied segment erases the spelling the pin
varies and all three renders agree before the fix too. The fixture asserts
that premise rather than trusting it. The case arm skips itself where
`samePath` says the two spellings are two directories (a case-sensitive
volume), because there the render would be over a repo with no redirect in it.

MEASURED 2026-11-08, both mutants run in this worktree:

| mutant | symlink arm | case arm | `TestQACanonicalPathIsTheKernelsSpelling` |
|---|---|---|---|
| `filepath.Dir(target)` + `AbbrevHome` (D2 undone) | RED, 6 lines | RED, 6 lines | green |
| `kernelPath` errors on darwin (D1's half undone) | green | RED | RED (case row) |
| `AbbrevHome` for the `~` form only | green | green | green |

Six differing lines and not k9x8n's three because here BOTH literals differ,
and each is rendered in each of check 3's three subjects. The second mutant
is ADR 0082's rejected option (a) — `EvalSymlinks` everywhere — measured
rather than asserted: it closes the symlink axis and leaves the case axis
open. The third is held next door, by `TestDeriveIdentityLiterals`' "the
fixture must make them differ" premise.

## The fixtures that moved, and why it is the build's work

ADR 0082's Consequences says a fixture building an expected literal from
`t.TempDir()` must pass it through `canonicalPath`. The live shape was
narrower and sharper than that: four tests named an instance directory they
never CREATED. `canonicalPath` falls back to `resolvedPath`'s string for a
path it cannot open, `$HOME` (an existing `t.TempDir()`) canonicalised, the
prefix test failed, and the `~` literal deduped away — so the failure read
`fixture premise: this box must derive an "instance-path-abs" literal`.

- `hookcluster_qa_helpers_test.go` (`visWall`) and
  `visibility_test.go`'s `TestIdentityLiteralGuardHook`: create
  `<instance>/.beads`.
- `TestDeriveIdentityLiterals`: create it, canonicalise the expectation, and
  assert the literal is `~/instance` **spelled out** rather than
  `AbbrevHome(instance)`. Its `$HOME` is left UNRESOLVED on purpose — that is
  what makes this fixture the one holding D2's home half, and resolving it
  "to be tidy" would leave the half unmeasured.
- `redirectfifo_test.go`'s CONTROL 2 passed for a reason it was not claiming:
  the two strings agreed only because the store did not exist yet. It now
  creates the store first and keeps two names — `store`, the target as
  `beadsHome` joins it, and `want`, what the wall renders — because those
  two differ on darwin and that difference is the whole of ADR 0082.

`internal/treepins/hookfreshness_qa_test.go` needed nothing: its redirect
names `/nowhere/instance/.beads`, outside `$HOME`, so both forms coincide and
dedupe to one literal exactly as before.

## Residual

A missing instance directory renders the unresolved spelling while an
existing `$HOME` renders the canonical one, so the `~` form is lost in that
one case. It needs a HOME whose canonical spelling differs from its literal
one AND an instance directory that is not there — which ADR 0082 already
calls a broken redirect. Not closed; the fallback's only promise is that it
is deterministic, which is all the byte compare needs.
