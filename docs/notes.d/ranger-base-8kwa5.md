# A relative `queue_repo:` is resolved before anything reads it, and the one-list pin asks the question its name makes (ranger-base-8kwa5)

ranger-base-l7sb2's two live findings, fixed 2026-11-08. Both are escapes from
closes that were themselves VERIFIED — the first was named as a line in
ranger-base-ynzig's close comment and deferred to ranger-base-k9x8n, which
took the other call site; the second shipped with ranger-base-eiwhk as a pin
whose name made a claim its body never asked.

## Finding 1 — `QueueRepo()` resolved no cwd

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

The relative spelling in BOTH places. That is the same class
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

MUTATION-CHECKED with `QueueRepo` back to `filepath.Clean(ExpandTilde(v))`:
reds four times — the premise (`QueueRepo() = "001"`), the repo the refusal
names, the key it should have prescribed, and the relative key it did — and
the refusal it prints is the one quoted above. The premise is an `Errorf` and
not a `Fatalf` for the reason the arm above it gives: a `Fatalf` would stop at
the premise and leave the two halves of the finding unread in the one run that
needed them.

## Finding 2 — a pin that was green under the duplication it was named for

`TestQATheWallRendererReadsTheSameCandidateList`
(`internal/posse/buildsource_qa_test.go`) stated the one-list claim in its
header — "`(*App).WallRenderer` composed its own copy of the candidates, and
`posse gates` ... is that copy" — and its body only called
`a.launcherCandidates(true, extra...)` twice. It never reached
`WallRenderer`, so the duplication it was named for was invisible to it. A pin
that is wrong about what it measures is live (ADR 0006 §6).

MEASURED 2026-11-08, darwin 25.6.0, with ranger-base-eiwhk's own named mutant
in place (`WallRenderer` composing `extra` + `a.BeadsDirs()` + `os.Getwd()`):

| run | result |
|---|---|
| `-run TestQATheWallRendererReadsTheSameCandidateList ./internal/posse` | **ok, 0.760s** |
| cmd/posse's two `gatesbuildsource` arms | FAIL, both |

So the property IS defended end to end, by
`cmd/posse/gatesbuildsource_qa_test.go` — which is why this was a pin-honesty
finding and not an undefended regression.

### Why the replacement is a source census

The claim is about which list a function SCANS, and no reading taken inside
`internal/posse` reaches the scan: `(*App).WallRenderer` goes through
`VersionString()`, a test binary carries no vcs stamp at all
(`cagestale.go`'s header), and `FindLauncher` returns on `!ok` before the
candidate loop. So the claim is asked of `wallrenderer.go`'s source, the way
`planmeteronce_qa_test.go`'s counts and the qib censuses already are, and the
behavioural end stays in `cmd/posse` where a stamped binary exists.

What it asserts: exactly one call to `launcherCandidates`, a variadic forward
of the caller's `extra` into it, and not one mention — ident or callee — of
the five sources `launcherCandidateList` composes (`PosseRepo`, `BeadsDirs`,
`Getwd`, `BuildSource`, `launcherCandidateList` itself). Idents as well as
callees because `BuildSource` is a var and not a call. Comments are dropped by
the parser (mode 0), because that function's doc comment names
`launcherCandidates` and the sources in prose and a line scan would read the
sentence about a call as the call. The method is found by receiver AND name,
because `WallRenderer` is also a type and a package-level function in that
file.

The ordering assertions that used to live under the wrong name are unchanged
and now sit under `TestQALauncherCandidatesLeadWithTheCallersExtra`, which is
what they ask.

MUTATION-CHECKED with the same mutant: the census reds three times — on the
count (`launcherCandidates` 0 times), on `BeadsDirs` and on `Getwd` — while
the renamed ordering pin stays green, which is exactly how the gap survived
its own close.

## Not fixed here

`filepath.Abs` does not resolve symlinks, so two ABSOLUTE spellings of one
queue repo are still two repos to `OwnsCommitWall` and to D7. Same open end
ranger-base-k9x8n left, already handed to architecture as
ranger-base-5wq6g; this bead adds one more call site to its radius and no new
question.
