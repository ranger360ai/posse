# canonicalPath answers by reach, not only by path (ranger-base-g6bs7)

2026-11-08. Verifying four closes — ranger-base-ib61s, ranger-base-yx73g,
ranger-base-oxk81, ranger-base-8kwa5. All four hold. One escape came out of
it, and it is the only thing here worth reading if you read nothing else.

## The escape: ADR 0082 D1's fallback clause is right about the wrong thing

D1 says that when the path cannot be opened or resolved — "it does not
exist, or **the cage refuses it**" — `canonicalPath` takes `resolvedPath`'s
string answer, and that "that answer is deterministic for a given input".
`internal/posse/visibility.go:382-384` carries the same sentence, and
`canonicalPath` is implemented exactly as written.

The claim is true and it is not what the byte compare needs. The L3 probe's
ours-vs-stale verdict (ADR 0023) compares a wall rendered by ONE process
against a wall rendered by ANOTHER. `canonicalPath`'s input is therefore not
the path string alone: it is the path string **plus the rendering process's
filesystem reach**. A renderer that can open the instance directory gets the
kernel's spelling; a renderer that cannot gets a different string for the
same directory — and the fallback is not an error, so nothing says so.

**MEASURED 2026-11-08, darwin 25.6.0, go1.26.10 darwin/arm64, APFS
case-insensitive**, `canonicalPath` and `kernelPath` reproduced verbatim
with stdlib only (holden's session scratchpad, `reachprobe/main.go`), one
directory on disk as `<root>/instance`, asked through the case-changed
spelling `<root>/INSTANCE`:

```
CONTROL  same directory under both spellings: true
with reach   : canonicalPath -> <root>/instance     (the kernel's spelling)
no reach     : kernelPath    -> permission denied   (instance chmod 0o000)
no reach     : canonicalPath -> <root>/INSTANCE     (resolvedPath's, case unfolded)
no reach     : resolvedPath  -> <root>/INSTANCE     (it still ANSWERS)
```

`chmod 0o000` is the cheap local stand-in for the cage: both land on the
same branch, an `open` that returns an error.

**And a degraded `canonicalPath` moves the wall by six lines**, which is the
other half and was measured as a mutation arm rather than assumed: with
`kernelPath` forced to error on darwin — the same degradation, reached from
inside — `TestQAOneRepoRendersOneWallHoweverItsPathIsSpelled` reds on the
case arm with six differing lines (two literals in each of check 3's three
subjects) while the symlink arm stays byte-identical. That is exactly
ranger-base-k9x8n's "ours but stale" on every launch, reached by a route
nobody has closed.

**Latent on this box, firing on two spellings it would take to make it
live.** The redirect here names the queue repo by a plain absolute path with
no symlink, no firmlink prefix and the on-disk case, so its `resolvedPath` and
`kernelPath` answers are the same string and reach cannot
move the literal as configured. Asked through the two spellings D1 exists to
fold, both diverge:

```
/System/Volumes/Data/Users/<u>/src/ranger-queue  resolvedPath keeps it, kernelPath folds the firmlink
/Users/<u>/SRC/ranger-queue                      resolvedPath keeps the case, kernelPath folds it
```

so a redirect or an `install-hooks` argument written either way, plus one
renderer that cannot open that directory, is the firing configuration. Two
of the wall's renderers run outside the launcher —
`cmd/posse/main.go:1872` (`posse gates install-hooks`, typeable from a
caged seat) and `internal/posse/queuejsonl.go:171`
(`CommitQueueJSONL` → `InstallCommitGuardHook`, which runs in whichever
process commits the queue jsonl). Which cage loses which `open` is NOT
measured here.

Not dinesh's error: D1 was implemented as the ADR wrote it, and the ADR is
where the sentence is. Filed for the code lane with the ADR amendment named
in its first line, because the fix is a decision (is `EACCES`/`EPERM`
distinguishable from `ENOENT` here, and should an existing-but-unreadable
directory degrade at all?) before it is a diff.

## What else the four closes were asked

Every stated DONE WHEN holds, and every mutation-check claim in the four
close comments reproduced here:

- **ranger-base-ib61s** — its close claimed no mutation check at all, so
  four were run: the bead's own defect (the template's `Environment=PATH=`
  line removed) reds four pins, including `unitPathDirs`' own sentence;
  `systemdFallbackPath` not appended reds `TestSystemdUnitPathFindsBd` and
  `TestSystemdUnitRenderIsPinned`; the gates rule not applied inside the
  render reds `TestSystemdUnitPathDropsWhatAUnitMustNotCarry`; `unitQuote`
  never quoting reds three escape pins. Landed as `fd83a639`, a rebase of
  the `9859b805` the close names — `internal/posse/systemdunit.go` and
  `systemdunit_test.go` are byte-identical across the two.
- **ranger-base-yx73g** — both claimed mutants reproduce, and they red
  DIFFERENT arms, which is what says the pin measures D1 and D2 separately:
  D2 undone reds both wall arms with six differing lines each and leaves
  `TestQACanonicalPathIsTheKernelsSpelling` green; D1's platform half undone
  leaves the symlink arm green and reds the case arm and that test's case
  row. Both wall arms RUN on this box (APFS case-insensitive). The four
  fixture rewrites each name why the OLD fixture was passing for a reason it
  was not claiming, which is the shape of a real fix rather than a green
  chased.
- **ranger-base-oxk81** — a state, so re-measured at verify time rather
  than read off the close: `git grep -Iil -w` at HEAD lists six files, and
  the unbounded `git grep -Ic -i` lists the same six, so no non-word-bounded
  spelling was missed. Five are the bead's class-2 list and the sixth is the
  id line the close found and filed. `ranger-base-owbmt`'s line list is
  exact, path/prose lines included, and it blocks on the ruling.
- **ranger-base-8kwa5** — the fix is NOT on main (`ranger-base-83nrw`, P1,
  open). Its two code files apply to HEAD cleanly, and over them both
  claimed mutants reproduce: the revert reds with the bead's own measured
  refusal verbatim, and `resolvedPath` in place of `filepath.Abs` stays
  GREEN — the arm that says the pin does not fence out ranger-base-wvoqe's
  answer. The withdrawal claim checks too:
  `internal/posse/buildsource_qa_test.go` at `93c3cec1` is byte-identical to
  the branch's merge base `fd83a639`, and main carries the better fix as
  `62b0fee1`.

One recorded line, no bead (ADR 0006 §6): the pin header at
`internal/posse/queuewallowner_qa_test.go:341` says the revert mutant "reds
four times over" and names four assertions. It reds TWICE — the premise at
`:396` and the prescribed key at `:431`. The other two pass deterministically
under that mutant: `:422` asks `strings.Contains(text, AbbrevHome(resolved))`
with `resolved` = `"001"`, and the refusal opens `001's commit wall`, so it
is satisfied; `:433` sits in an `else if` the relative key never reaches.
Nothing a reader would act on differently, so it is recorded here and not
filed.
