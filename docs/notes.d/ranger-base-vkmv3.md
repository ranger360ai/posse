# ranger-base-vkmv3 — what a refusal prints for a directory nobody can name

ADR 0082 D7. The bead was filed as a merge-back break and is one; the
deliverable is the decision underneath it.

## The break

Two beads landed off one base, `e3deada6`, and did not conflict:

- **ranger-base-8j070** gave `canonicalPath` and `canonicalAbbrevHome` an
  error return. ADR 0082 D5/D6: a directory that is THERE and this process
  may not name it has no canonical spelling, so the function refuses rather
  than degrading to the caller's spelling, because the fallback would let two
  renderers with different reach write two walls for one repo and the L3 probe
  would read the second as "ours but stale" on every launch.
- **ranger-base-w79sc** added `CanonicalAbbrevPath`, D4's ONE helper for the
  path a refusal prints, as `canonicalAbbrevHome(canonicalPath(p))`.

8j070 changed two signatures; w79sc changed neither and only added a caller
below them. Nothing is a textual conflict, so the merge succeeded quietly and
the BUILD is what broke.

MEASURED 2026-11-08, go1.26.10 darwin/arm64, on main at `9ac5f656`:

```
$ go build ./...
internal/posse/visibility.go:576:9: too many return values
        have (string, error)
        want (string)
internal/posse/visibility.go:576:29: too many arguments in call to canonicalAbbrevHome
        have (string, error)
        want (string)
```

**And a second break the product build cannot see.** `go vet` per arm, once
`visibility.go` compiled again:

```
internal/posse/queuerefusalspelling_qa_test.go:113:5: multiple-value canonicalPath(spelling)
        (value of type (string, error)) in single-value context
```

Three one-value `canonicalPath` calls in that pin's fixture premise, behind
`//go:build posse_arm3`. `go build ./...` builds no tests and arm 1 does not
compile arm 3, so the break was reachable only by `go vet -tags posse_arm3
./internal/posse` or by the arm itself — which is the AGENTS.md note about a
bare `go test ./...` no longer being the whole suite, met from the other side.
They now go through `qaCanonicalPath(t, …)`, which Fatals on the refusal arm
rather than leaving the premise comparing `""` against a spelling and passing
for the wrong reason.

## The decision (ADR 0082 D7)

The signature was never the decision. Both of the helper's call sites are
refusal lines — ADR 0081 D7's queue-commit refusal and D4's `not installed:
prepare-commit-msg` with its remedy — and a refusal has to print something. So
landing this decides what a line prescribes as a `beads_visibility:` key for a
repo whose directory this process may not name, and every later prescribing
site inherits it.

`CanonicalAbbrevPath` keeps one return value and degrades per half:

```go
func CanonicalAbbrevPath(p string) string {
	c, err := canonicalPath(p)
	if err != nil {
		return AbbrevHome(p)
	}
	a, err := canonicalAbbrevHome(c)
	if err != nil {
		return c
	}
	return a
}
```

Three things carry it, and the third is the one that settles it:

1. **The consumer differs from the wall's.** D5 refuses because ADR 0023's
   verdict compares one process's wall bytes against another's. This answer is
   printed once and never stored, re-derived or compared, so there is no second
   render to disagree with.
2. **`""` is the one answer this site cannot give.** `add : <private|public>`
   prescribes nothing, and both call sites are the last thing on their path
   (one returns the error, one `os.Exit(1)`).
3. **The reason to canonicalise is void in exactly the case the degrade is
   reached.** Matching was never the point — the key is read by `samePath`, so
   every absolute spelling already matched (D4's own consequence). What
   canonicalising buys is agreement with the identity wall's spelling of that
   repo. And over an instance path it cannot name, D6 refuses the whole
   derivation, so that repo's wall HAS no instance-path literal. There is
   nothing left to agree with, and the caller's spelling is the only spelling
   of that directory in existence.

## Measured

MEASURED 2026-11-08, darwin 25.6.0, go1.26.10 darwin/arm64, APFS
case-insensitive, by `internal/posse/canonicalabbrevdegrade_qa_test.go`. Both
arms reached their readings on this box; neither skipped.

| fixture | reading |
|---|---|
| repo readable | the kernel's spelling, through both the on-disk and the case-changed spelling |
| repo `chmod 0` | `canonicalPath` -> `errPathUnnameable`; the degrade is `AbbrevHome`'s of the caller's spelling |
| the same, asked case-changed | `…/001/INSTANCE` degraded against `…/001/instance` with reach — a second spelling, measured |
| `$HOME` at **0111**, repo under it at 0755 | `canonicalAbbrevHome` -> `errPathUnnameable` while `canonicalPath(repo)` answers; the degrade is the canonical absolute path, `~` lost |

**Mode 0111 is the finding worth keeping.** F_GETPATH's reading is an
`open(dir, O_RDONLY)` — READ on the directory. Reaching a path BELOW it needs
only SEARCH. So a directory at 0111 is one this process may not NAME and may
still reach THROUGH, which is:

- the only state where the home half refuses while the path half answers, so
  the second arm is writable at all; and
- an ordinary permission setting, not an exotic one. D5's "latent rather than
  live" stands for the three cages, and it is `chmod 0` that is contrived —
  the class is not.

## Mutation record

MUTATION-CHECKED 2026-11-08, four mutants, each run over the two new names
alone, baseline asserted clean (`git status --porcelain`) before and after
each and HEAD asserted unmoved at the end. Every one red; the two that matter
are red in ONE arm each, which is the discrimination the pair was written for.

| mutant | what reds |
|---|---|
| **M1** both halves `return ""` — D5's own answer, printed | **both arms.** Arm 1 on the `= ""` assertion and on `want AbbrevHome's …`; arm 2 on `= ""` and on `want canonicalPath's own …` |
| **M2** one-line degrade: `AbbrevHome(p)` whenever EITHER half refused | **the home arm only.** It answers `~/Repo` — a `~` form taken against a home this process may not name — and reds on both `want canonicalPath's own …` and the explicit "the kernel reading was taken and then thrown away". Arm 1 is GREEN, which is the whole reason arm 2 exists |
| **M3** drop the path half's degrade, keep the home half's | **the path arm only.** Arm 2 passes untouched, so neither arm is carrying the other |
| **M4** `kernelPath` always errors on darwin — D1's platform half gone | **both arms, at a PREMISE rather than at the decision**, which is right: with no kernel reading there is no refusal for D7 to degrade. Arm 1 reds at its with-reach premise (the two spellings render two strings, so there is no single answer a degrade could be compared against); arm 2 reds at the `kernelPath … is gone` door |

**M4 is why that door was added.** Before it, `qcwNoReach` and the home arm's
`canonicalAbbrevHome` premise both answered a broken platform half with a
SKIP — which is this file passing vacuously over the one mutant that removes
the thing it pins. One `kernelPath` call of a readable directory the test
created tells a box that will not deny the open (nothing to measure) from a
`kernelPath` that has stopped answering (D1's darwin reading gone). Arm 1
turned out to red one premise earlier than that door, at the with-reach
reference; the door is what catches arm 2, and it catches arm 1 in the
configuration where reach cannot be withdrawn.

## What this does not touch

`QueueRepo()`, `hookRepo`, `underDir`, `OwnsCommitWall` and `install-hooks`'
target are all untouched, as D4 requires — a render never feeds a comparison.
Nor is D6 moved: the wall's own path still goes through `canonicalPath`
directly and propagates (`commitGuardLiterals`, `DeriveIdentityLiterals`), so
nothing here can put a second spelling into a wall. w79sc's two refusal-
spelling pins and 8j070's two wall pins are unchanged and green.

ranger-base-qm7eg's G13 `queue-unmarked` row inherits this answer, which is
what it said it was blocked on. Same helper, no second decision.
