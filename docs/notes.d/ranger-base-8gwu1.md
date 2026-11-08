# The home stamp takes the kernel's spelling too (ranger-base-8gwu1)

ADR 0082 D3, built 2026-11-08, on ranger-base-yx73g's `canonicalPath`. ADR
0081 D2's `# posse-home: <absolute a.Home>` is now
`# posse-home: <canonicalPath(a.Home)>`, which is the same spelling check 3's
instance-path literals take one line further down the same body.

## Why the home was the same defect as the repo

`canonicalPath` landed for the REPO half: two absolute spellings of one repo
rendered two identity walls, and the second read as "ours but stale" on every
launch (the L3 probe's identity half is byte-for-byte, ADR 0023). The stamp
is the one other path the wall renders, and it had the same hole one argument
over — an `RHQ_HOME` typed through a symlink, or the `/var/folders` spelling
of a `/private/var/folders` home, which is what every test rig on this box
has. Same loop, reached through the home instead of the repo.

## What changed

`commitGuardHomeStamp` (internal/posse/gates.go), and nothing else in
production:

```go
abs := canonicalPath(home)
if strings.TrimSpace(abs) == "" || strings.ContainsAny(abs, "\n\r") {
	return ""
}
return commitGuardHomeStampPrefix + abs + "\n"
```

The empty-home rule and the newline guard stay, and the newline guard is now
asked TWICE — of the argument and of the rendered answer. That is not
tidiness: `canonicalPath` hands back a string the KERNEL chose, so a home
whose own path carries no newline can resolve onto one that does, and the
bytes that must fail closed are the bytes written into a file git execs on
every commit.

MUTATION-CHECKED 2026-11-08: with the second guard removed,
`TestQACommitGuardHomeStampIsAbsoluteOrAbsent`'s last arm reds and prints the
defect in full — a home spelled `<tmp>/link`, carrying no newline itself,
rendering `# posse-home: <tmp>/home\nrm -rf ` into the hook.

The sentence that stood over that function is withdrawn, in the ADR and in
the comment: "absolute by filepath.Abs and Clean and NOT by resolving
symlinks, because the operator acts on the spelling they configured". Its
operator half survives the change — a refusal that names the kernel's
spelling names the same directory.

**No reader changed.** `commitGuardHomeVerdict` still compares with
`samePath` and `scripts/verify-hook-freshness.sh` with `same_dir`, so a body
an earlier binary stamped non-canonically is still THIS home's wall: it
re-renders once through the stale→reinstall path that already exists. No
migration case was added — ADR 0081 D3 case 4 stays about an UNSTAMPED body.

## The pins

Both at the foot of internal/posse/commitguardhomestamp_qa_test.go, and the
split between them is the point: one home spelled two ways is now one wall at
the RENDER, and `samePath` is what still makes a pre-0082 body this home's to
re-render.

- `TestQAOneHomeSpelledTwoWaysRendersOneCommitWall` — the bead's pin. Two
  Apps whose `Home` is one directory under two spellings (a symlink, and on
  darwin a case-folding volume another case) render byte-identical
  `CommitGuardHook` bodies. Byte equality of the whole body, not a grep for
  the stamp line, for the reason canonicalwall_qa_test.go gives about the
  repo half.
- `TestQAAWallInstalledUnderAnotherSpellingOfThisHomeReadsAsHeld` — the same
  claim through the probe that pays for it: `l3Held`, and the write
  `sharedCommitWallWrite` then allows.
- `TestQAOneHomeSpelledTwoWaysIsOneHomeToTheCommitWallReader` —
  ranger-base-hgq2k's arm, rebuilt. It used to INSTALL through a symlinked
  spelling; the renderer no longer produces two spellings, so it now plants
  the pre-0082 stamp the way the shell sibling does (one line of the current
  render changed) and asserts the reader's `samePath` over it.

MUTATION-CHECKED 2026-11-08, darwin 25.6.0, go1.26.10 darwin/arm64, APFS
case-insensitive — the same two mutants the repo half is checked with, and
they red different spellings:

| mutant | what reds |
|---|---|
| `commitGuardHomeStamp` back to `Abs`+`Clean` (D3 undone) | BOTH spellings of the render pin, and the installed slot's stamp in the probe arm. The READER arm stays GREEN, which is what says it pins `samePath` and not the render. |
| `kernelPath` returning an error on darwin (D1's platform half undone, so `canonicalPath` degrades to `resolvedPath` — ADR 0082's rejected option (a)) | the CASE spelling only; the symlink spelling stays green. The ADR's reason for rejecting (a), measured. |

## The fixtures, which were the bulk of the diff

ADR 0082's Consequences calls this the build's work rather than a
divergence: a fixture that builds an expected literal from a `t.TempDir()`
passes it through `canonicalPath`. Three shapes of it, and the fix is spelled
once per package:

- internal/posse — `qaStampedHome(home)` / `qaStampedBy(home)` in
  gates_helpers_test.go (untagged, so every arm compiles them): the stamp's
  value, and how a reader PRINTS it (`AbbrevHome` over the same). Used by
  the probe's degraded line, the hook-wall sweep, the launch warning and the
  queue refusal — commitguardhomestamp, commitwallowner and queuewallowner
  (arm 3).
- cmd/posse — `cwoFixture` resolves its temp root, the way
  installhooksrelativetarget_qa_test.go already did for the mirror-image
  reason. Five arms were comparing `cwoStampOf` against the raw `/var/...`
  spelling.
- internal/treepins — hookfreshness's foreign-home arm asserts the finding
  names THIS home, which the script reads out of the REFERENCE render, so
  that one expectation resolves too. The three arms that PLANT a stamp and
  read it back were already correct: they take the value out of the render
  rather than recomputing it, which is the coupling to copy.

One incidental, which is the new pin's own door and not an adjacent fix: the
pin is `t.Parallel` and reuses `qcwDiffLines` for its failure output, whose
local closure was named `at`. `cmd/testparallel` resolves a called NAME
against every `at := func(...)` in internal/posse, one of which reaches the
plan-clock package var `blindT`, so the pin went UNCLEARED and
`TestQAParallelClearanceDoesNotWaiveAReasonNobodyCleared` red over a local
variable's name. Renamed to `lineAt`. A `parallelOK` line naming `blindT`
would have cleared a reason this test does not have.

`EvalSymlinks` rather than the kernel reading in the two outside packages:
`canonicalPath` is unexported and the only axis those rigs cross is darwin's
`/var` symlink, since every path under their roots is created in the case it
is named in and a temp root is not reached through a firmlink. Exporting a
function so a fixture could call it would be a wider change than the thing
it fixes.
