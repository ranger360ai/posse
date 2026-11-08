# One refusal, one spelling: the path a remedy tells you to paste (ranger-base-w79sc)

*2026-11-08 · ADR 0082 D4 (from ranger-base-wvoqe) · dinesh*

## What was wrong

Two refusals print a path and then tell the operator to paste it as a
`beads_visibility:` key:

- ADR 0081 D7's queue-commit refusal (`internal/posse/queuejsonl.go`)
- D4's `not installed: prepare-commit-msg` and its remedy
  (`cmd/posse/main.go`)

Both printed the repo through `AbbrevHome(hookRepo(dir))` — the caller's
spelling — and both printed the owning home out of the wall's
`# posse-home:` stamp, which ADR 0082 D3 had already made canonical. So one
line showed two paths in two spellings, and the key it prescribed was a
function of how a config happened to spell `queue_repo:`, or of which
symlink an operator walked in through, rather than of the directory it
names.

Matching was never the defect, and that is worth saying because it is why
this was a P3: the prescribed key is read by `samePath`, which answers by
identity, so every absolute spelling already matched. What was wrong is
that the identity wall *in that same repo* prints one spelling and the
refusal about that wall printed another, so an operator who greps for one
does not find the other.

## What landed

`CanonicalAbbrevPath(p)` in `internal/posse/visibility.go`, beside
`canonicalAbbrevHome`:

```go
func CanonicalAbbrevPath(p string) string {
	return canonicalAbbrevHome(canonicalPath(p))
}
```

That is the composition D2 already renders the wall's own instance path
with. It is exported for one reason: the two sites are one in `cmd/posse`
and one in `internal/posse`, and D4's requirement is that they cannot
drift. Both paths, in both lines, of both refusals now go through it.

Nothing else moved. `QueueRepo()` is still a VALUE — it feeds
`underSameDir`, `OwnsCommitWall` and `InstallCommitGuardHook` — and
`hookRepo`, `underDir` and `install-hooks`' target are untouched. Each new
pin asserts that premise directly, so a later pass that "fixes" this by
canonicalising the value reds on the premise and reads D4's reason there,
rather than passing because the refusal happens to look right.

## Measured

MEASURED 2026-11-08, darwin 25.6.0 (go1.26.5 darwin/arm64, APFS
case-insensitive). Four mutants, each run over the new names alone:

| mutant | what reds |
|---|---|
| `queuejsonl.go`'s render back to `AbbrevHome` | both queue arms, 3 assertions each — the repo named, the key prescribed, the config's spelling still there |
| `main.go`'s two `Fprintf` lines back to `posse.AbbrevHome` | both install-hooks arms (3 assertions, then the byte equality); all 11 sibling arms green |
| `kernelPath` returns an error on darwin (D1's platform half undone, so `canonicalPath` degrades to `resolvedPath` — the ADR's rejected option (a)) | the CASE arm, at its PREMISE; the SYMLINK arm green |
| `canonicalPath` inside `QueueRepo()` (D4's rejected alternative) | both PREMISES and nothing else — the refusals it prints are right |

The third row is the ADR's reason for rejecting (a), measured at the refusal
the way `canonicalwall_qa_test.go` measures it at the wall: with
`EvalSymlinks` alone the two spellings canonicalise to *two* strings
(`…/001/REPO` and `…/001/Repo`), so there is no single answer for a render
to print. The fourth is why the premises are asserted at all: that
alternative's cost is in the three comparisons downstream, where no arm
here can see it.

MEASURED separately: `install-hooks`' pre-push line reports the slot it
WROTE (`<target>/.git/hooks/pre-push`) and carries the caller's spelling,
correctly — it is a file this run created, not a path prescribed for
pasting. The pin filters the commit-guard lines rather than asserting over
the whole output, and `ihrsGuardLines`' comment says why.

## DONE WHEN (3): a pin that is not on main

`TestQAARelativeQueueRepoIsRefusedByItsAbsolutePath` is ranger-base-8kwa5's,
and 8kwa5's three commits do not land on `main` — the merge-back is blocked
as **ranger-base-83nrw** (P1, gwart). So it could not simply be run here.

Running the file as it stands on that branch reds at
`qwoForeignWall`'s own fixture premise, because that version predates
ADR 0082 D3 and compares the stamp against an unresolved `x.Home`. That
failure is the 83nrw conflict and not this bead's fix. The honest
experiment is the pin as it will exist once that conflict is resolved: the
pin function and its `qwoPrescribedKey` regexp grafted VERBATIM onto
`main`'s current `queuewallowner_qa_test.go`, with 8kwa5's one-line
`filepath.Abs` in `QueueRepo()`. **It passes, unedited** (MEASURED
2026-11-08, 1.08s), which is what the ADR predicted: that arm asserts a
property — absolute, and names this repo by `samePath` — and never one
spelling, deliberately, so that D4's answer could not fence it out.

Both files were restored; nothing of 8kwa5's is in this bead's commit.

## The third site

Re-measuring D4's own radius grep found one more site, so the Radius
paragraph's "these two are the only sites" is one short:
`QueueReadiness.GovRows`' G13 `queue-unmarked` row prescribes
`` `%s: private` `` in the caller's spelling. Filed as
**ranger-base-qm7eg** rather than taken: this bead's DO and DONE WHEN
enumerate two sites, and a governance row's Detail has a different audience
(`posse status`, the cockpit) than a refusal. Priced on the way past —
`govern_test.go`'s three G13 arms assert `Key`, `ID` and `Class` only, so
no pin constrains that wording and the fix is two format arguments.
