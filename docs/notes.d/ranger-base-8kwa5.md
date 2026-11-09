# A relative `queue_repo:` is resolved before anything reads it (ranger-base-8kwa5)

ranger-base-l7sb2's finding 1, fixed 2026-11-08. It is an escape from a close
that was itself verified: ranger-base-ynzig's close comment named it as a line
and deferred it to ranger-base-k9x8n, which took the other call site only.

**Finding 2 of that bead is not mine.** The one-list pin
(`TestQATheWallRendererReadsTheSameCandidateList`) was fixed on main while this
bead was open, by the commit titled "ranger-base-l7sb2: the one-list pin is
asserted through WallRenderer, which is what it is named for" — and fixed
better than the source census this bead had written for it: it gets a real
in-package reading by setting `Build`, the linker-stamped var
`version_test.go` sets, so `VersionString()` names the rig's commit and
`FindLauncher` scans. That is the seam the bead's own fix shape asked for
first. This bead's census was dropped rather than merged; the last section
says what was kept from measuring it.

## `QueueRepo()` resolved no cwd

`QueueRepo()` was `filepath.Clean(ExpandTilde(v))`. `Clean` resolves no cwd, so
a RELATIVE `queue_repo:` stayed relative all the way to ADR 0081 D7's refusal
in `internal/posse/queuejsonl.go` — which named that spelling as the repo and
then prescribed it as a `beads_visibility:` key.

MEASURED 2026-11-08, darwin 25.6.0, `internal/posse` arm 3, scratch home and
scratch queue repo, cwd at the repo's parent and `queue_repo: 001`:

```
001's commit wall was rendered by home <abs home> and this home does not own
it ... remedy: add `001: <private|public>` under this home's
beads_visibility: ...
```

The relative spelling in BOTH places.

**That reading is pre-D4 and cannot be retaken** (ranger-base-83nrw, the
merge-back). ADR 0082 D4 landed ~3h after this bead's commits were authored
and put both of D7's path renders through `CanonicalAbbrevPath`
(ranger-base-w79sc), and `canonicalPath` goes through `resolvedPath`, which
`Abs`'s against the RENDERING process's cwd. So on today's main the refusal
prescribes an absolute key whether or not `QueueRepo()` resolves one, and the
block above is the record of the defect rather than a reproducible reading.
What still needs the `Abs` is the VALUE: the four predicates read it, and a
relative value names a different directory to every process that reads it from
a different cwd — which a render cannot fix, because a render is not what they
read.

That is the same class
ranger-base-k9x8n fixed one call site over, and it is more than wording for
the same reason: a `beads_visibility:` key is read by `samePath` →
`resolvedPath` → `filepath.Abs`, i.e. against the READING process's cwd. So
the remedy D7 prints is inert wherever it is read from another cwd, and where
it is not inert it is a cwd-dependent wildcard over the visibility verdict
that same wall enforces — and the remedy is the only thing that makes the
refusal actionable, because a non-owning home's closes stop reaching git until
it is taken.

The fix is one `filepath.Abs` in `QueueRepo()`, which is the one place that
reaches `underDir`, `OwnsCommitWall`, `InstallCommitGuardHook` and the refusal
together. Same act, same shape as k9x8n's.

### What is *not* wrong, so nobody re-measures it

The WRITE path. MEASURED the same day, owning home, same repo, same config
with only the spelling changed: the wall `InstallCommitGuardHook` renders
through a relative `queue_repo:` is BYTE-IDENTICAL to the absolute one
(64437 bytes). k9x8n's identity-derivation consequence does not reach this
call site — but for a reason neither close states: the queue repo HOLDS the
store, so it carries no `.beads/redirect` and `identityRedirectTarget` answers
`""`. It is not because the dir cannot be relative.

### The pin

`TestQAARelativeQueueRepoIsRefusedByItsAbsolutePath`
(`internal/posse/queuewallowner_qa_test.go`, arm 3), beside
`TestQAAQueueCommitFromANonOwningHomeLeavesTheWallAndRefuses`. Ownership and
the foreign stamp are set up through the absolute spelling, so the only
variable is how the config spells the repo: `t.Chdir` to the repo's parent and
`queue_repo:` rewritten to `filepath.Base(repo)`, which is the `001` the
measurement above was taken with.

The expected absolute path comes from `os.Getwd()` and not from the parent
directory the fixture built, because `filepath.Abs` resolves against
`os.Getwd()` and the two therefore cannot disagree. Which spelling that IS
depends on how the cwd was reached: `os.Getwd` prefers `$PWD` when it names
the same directory and `testing.T.Chdir` sets it, so here it is
`t.TempDir()`'s `/var/...` and not darwin's physical `/private/var/...`. That
is the opposite reading from ranger-base-k9x8n's fragment, whose pin reaches
its cwd through the binary's own `exec` and therefore sees the physical path —
both are right about their own route, which is why neither pin hard-codes a
spelling.

### It asserts the property, not a string

Load-bearing rather than taste, and the thing this fragment most exists to
record. ADR 0082 D1 landed `canonicalPath` the same day
(ranger-base-yx73g) — the kernel's spelling of a directory, resolving
symlinks, on-disk case and firmlinks on top of `Abs` — and whether THIS call
site should canonicalise on top of absolutising was an open question
(ranger-base-wvoqe) when this was written. A pin demanding one exact spelling
would red the day that question is answered yes: a pin fencing out the better
fix. It was answered hours later; the two paragraphs under the table below are
what that cost and did not cost.

So the premise asks only that the value be absolute and `samePath` the repo,
and the refusal assertions parse the prescribed key back out of the message
(`qwoPrescribedKey`) and ask it the same two things. The key is captured
rather than substring-matched because the refusal already names two absolute
homes, so "the message contains an absolute path" would pass on a relative
key.

MUTATION-CHECKED both ways:

| mutant | result |
|---|---|
| `QueueRepo` back to `filepath.Clean(ExpandTilde(v))` | reds twice — the premise (`QueueRepo() = "001"`) and the prescribed key, printing the refusal quoted above |
| `QueueRepo` as `resolvedPath(v)` — `Abs` + `EvalSymlinks`, i.e. `canonicalPath` minus its kernel half | **green**, which is the half that says the pin does not fence out ranger-base-wvoqe's answer |

**Re-measured on main, where the first row's figure is one and not two**
(ranger-base-83nrw). D4's render `Abs`'s the path it prints, so the refusal
prescribes an absolute key even over a relative value and every assertion
below the premise passes under that mutant. The premise is this pin's subject
and the premise is what reds, so nothing is uncovered — the refusal's spelling
is pinned where D4 decided it, in
`internal/posse/queuerefusalspelling_qa_test.go`, and reverting D4's render
reds those two arms while leaving this one green. Each fix is red in one file
only, in both directions, which is ranger-base-w79sc's own "both pins hold;
neither subsumes the other" measured on the tree where the two first met.

The second row is now a statement about a settled question rather than an open
one: **ranger-base-wvoqe was answered yes for the RENDER and no for the
VALUE** (closed 2026-11-08). `QueueRepo()` stays on `filepath.Abs`, because it
feeds three comparisons on one side only and because a config and a store each
spelled differently from the disk is a setup that works today. The pin's
property form was written to avoid fencing out that answer and it did exactly
that — the spelling form this bead's first commit wrote reds against D4's
renderer, which is the hazard the paragraph above it named, happening.

The first draft of this pin compared `QueueRepo()` against
`filepath.Join(os.Getwd(), base)` exactly. It passed, it reddened correctly on
the first mutant, and it would have reddened on the second — on darwin
`EvalSymlinks` yields `/private/var/...` where `os.Getwd` yielded `/var/...`.
That is recorded because a string-exact pin is the easy thing to write here
and it is wrong for a reason no single run shows.

The premise is an `Errorf` and not a `Fatalf` for the reason the arm above it
gives: a `Fatalf` would stop at the premise and leave the finding's other half
unread in the one run that needed it.

## What was kept from finding 2

The census this bead wrote for it is not in the tree — main's fix is better
and reverting mine keeps the merge clean (this branch leaves
`buildsource_qa_test.go` identical to the merge base, so the three-way merge
takes main's side with no conflict). Two measurements from it are worth
keeping, because they are about the ESCAPE and not about either fix:

MEASURED 2026-11-08, darwin 25.6.0, with ranger-base-eiwhk's own named mutant
in place (`WallRenderer` composing `extra` + `a.BeadsDirs()` + `os.Getwd()`):

| run | result |
|---|---|
| `-run TestQATheWallRendererReadsTheSameCandidateList ./internal/posse` | **ok, 0.760s** |
| the same arm after its rename, `TestQALauncherCandidatesLeadWithTheCallersExtra` | **ok, 0.456s** |

Both green under the mutant, independently reproducing the bead's reading. The
second is the one that explains the escape rather than just restating it: the
assertions that body makes are a true and useful claim about
`launcherCandidates`, and they are green either way, so no amount of care
spent on them could have caught the duplication. The defect was the NAME over
them, and a pin that is wrong about what it measures is live (ADR 0006 §6).

## Not fixed here

`filepath.Abs` does not resolve symlinks, so two ABSOLUTE spellings of one
queue repo are still two repos to `OwnsCommitWall` and to D7. That is no
longer an open end in general — ADR 0082 D1 and ranger-base-yx73g answered it
for the identity wall's RENDER with `canonicalPath`, and ranger-base-5wq6g is
closed. What is open is whether this call site is in that radius, and it was
left alone here on three grounds:

- `QueueRepo()` feeds four consumers from one value, and `underDir` compares
  it against a store dir that is NOT canonicalised. Canonicalising only this
  side changes a comparison nobody asked to change — and ADR 0082's own rule
  is that `samePath` is the COMPARISON and `canonicalPath` is the RENDER, and
  the two must not merge. A value that is both is the case the ADR does not
  cover.
- The remedy is not inert without it. The key D7 prescribes is matched by
  `samePath`, which resolves both sides and then falls back to
  `os.SameFile`, so a key in the operator's own case and through their own
  symlinks still matches. The case axis is ranger-base-99gww's and `samePath`
  answers it. What a relative spelling broke was different in kind: a
  relative key names a different directory to every reader.
- ADR 0082 scopes `canonicalPath` to `DeriveIdentityLiterals` and leaves
  `hookRepo`, its three callers and `commitGuardLiterals` on the value as
  given. Widening that is the ADR's decision.

Filed as ranger-base-wvoqe (`-l architecture`, P3), which the production
comment in `queuejsonl.go` names.

**Answered the same day, and this bead's three grounds are two of the
decision's three** (ranger-base-wvoqe closed by richard 2026-11-08; recorded
here by ranger-base-83nrw, the merge-back that landed this fragment). The
answer is **yes for the RENDER and no for the VALUE**: a path a refusal prints
for the operator to paste as config is spelled `canonicalPath` /
`canonicalAbbrevHome` at the FORMAT site, through one exported helper, which
ADR 0082 D4 then applied to both prescribing lines — `queuejsonl.go`'s D7
refusal and `cmd/posse/main.go`'s install-hooks D4 (ranger-base-w79sc, and
`GovRows`' queue-unmarked row under ranger-base-qm7eg). `QueueRepo()` stays on
`filepath.Abs`. The alternatives were priced and the two rejected ones are the
first ground above: canonicalising in `QueueRepo()` changes three comparisons
on one side only and would break a config and a store each spelled differently
from the disk, a setup that works today; canonicalising both sides of
`underDir` is the string folding `samePath` says cannot be done, and would put
an `fcntl` inside a seatbelt predicate. The decision also turned up a live
comparison defect next door — `underDir` is case-blind-wrong, `os.SameFile`
true where `filepath.Rel` says no — which is ranger-base-oczuq's
`underSameDir`, on main and in the same function as this bead's fix.
