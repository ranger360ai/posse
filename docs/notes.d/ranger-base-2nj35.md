# The remedy nobody tested, one check over (ranger-base-2nj35)

ranger-base-ckcly closed check 4's second remedy sentence and filed this:
check 3's two — `IdentityWayThrough` and `IdentityMessageWayThrough`,
`internal/posse/visibility.go` — had no test reader either. 2026-11-13,
this box (linux 7.2.15 x86_64, go1.27.3, git 2.56.0, 16 cores).

The class is ranger-base-68631 finding 6, "the remedy promises more than
the set allows". A refused writer does what the sentence tells them; if the
sentence is wrong about what the rule allows, the remedy is a loop. Nothing
keeps a sentence and a rule in step but a test that reads both.

## The reader census

A grep for both names over every tracked `.go` file found the two
declarations and their three call sites in `gates.go`, and nothing else.
The check-3 cluster is large and every pin in it reads the RULE — what the
wall refuses, in which arm, with which flags. None read what the refusal
then tells the writer to do, and none of them did it.

`internal/posse/checkthreeremedy_qa_test.go` (arm 3, with the other pins
that drive a real hook through a `visWall`) is the two readers:

- `TestQAIdentityRemedyPromisesOnlyWhatCheckThreeAllows` over the content
  and path arms' shared sentence. Three rows for "generalize whatever
  carries it (a comment, a fixture, a hardcoded path)" — each carrier
  staged with the literal and refused, then the SAME file generalized and
  committed, because a way through that is still refused is the whole
  class. Then the rename clause, from the one starting position it is
  about: a path already in history, which `TestQAIdentityPathArmIsAdded
  EntriesOnly` shows may be modified and deleted freely. The sentence says
  a rename of it is scanned again, so a rename to another identity-bearing
  name is refused (naming the destination, not the grandfathered source)
  and a clean name commits.
- `TestQAIdentityMessageRemedyPromisesOnlyWhatTheRuleAllows` over the
  message arm's. PIN (r) in `checkthreemessage_qa_test.go` already measures
  two of that sentence's claims; what had no reader is the first clause —
  "rewrite the commit message", the way through itself, which no pin
  performed — and the three the writer's next command depends on: the index
  is where they left it, the rewritten message commits, and it overwrites
  the residual the previous sentence just told them about.

Both hold each clause from both ends: the noun must still be in the
sentence and the wall must still do what it says, so a narrowing of the
rule reds and a rewording that drops a promise reds too.

## Whitespace-collapsed, which check 4's pins are not

These two constants are hard-wrapped at the width a terminal refusal
prints, so a phrase worth naming straddles a line break — `this\ninstance's
path`, `a fresh added\nentry`. A raw `strings.Contains` would be a pin on
the wrapping. `remedyPromises` collapses both sides: re-wrapping a refusal
is not drift, re-wording it is.

## MEASURED

Nine mutants, `go test -overlay`, tracked tree never written. "before" is
the same mutant with the new file overlaid to an empty stub, run over every
test in `internal/posse` named
`Identity|CheckThree|Visibility|WayThrough|Remedy` in all three arms.

| # | mutant | before | after |
|---|---|---|---|
| M1 | content remedy exempts its own three carriers ("a comment, a fixture or a hardcoded path may keep it in a test") | GREEN | RED |
| M2 | content arm stops scanning `//` lines | RED (arm 1, 2) | RED |
| M3 | `--no-renames` dropped from the identity path listing | RED (arm 1, 2) | RED |
| M4 | message remedy stops saying "rewrite the commit message" | RED (arm 1, 3) | RED |
| M5 | message arm reads the SUBJECT line only | RED (arm 1, 3) | RED |
| M6 | rename clause reversed ("the same entry, so the new name is not scanned again") | GREEN | RED |
| M7 | message remedy drops "your tree and index are exactly where you left them" | GREEN | RED |
| M8 | message remedy stops bounding its residual ("that nothing ever clears") | GREEN | RED |
| M9 | a refusal clears the index | GREEN | GREEN |

The four that were GREEN before are all the same shape, and it is the shape
the bead was filed over: a sentence re-promising what the rule refuses. The
three rule-direction mutants were already caught by the pins that read the
rule; these are a second reader that catches them in the sentence's own
words. M4 is the one clause that had a reader of sorts — PIN (m) asserts
the refusal PRINTS "rewrite the commit message" — and nothing performed one.

## M9 is a no-op, and that is the finding

"Your tree and index are exactly where you left them" cannot be falsified
by anything the check-3 render does. MEASURED directly: at
prepare-commit-msg time git sets `GIT_INDEX_FILE` to
`.git/next-index-<pid>.lock`, so a hook that runs `git reset` writes a
throwaway and exits 0 with the real index untouched; with the variable
unset the same reset fails rc 128 on the lock. A refusing hook cannot
un-stage the writer's work whatever it does. The pin over that clause is
therefore a pin on git's protocol, and on the day somebody adds a layer
that mutates the index before refusing — not on our render. Stated at the
assertion so a reader does not mistake it for a render pin.

The same probe showed the hook's `$1` IS `.git/COMMIT_EDITMSG` for an
ordinary commit, which is why the residual PIN (r) measures exists at all:
git wrote that file before calling us. The half that is not tautological is
the bound — that the next commit replaces it — and that is the half this
file measures.

## Not closed here

- "move the content to the instance tree" / "write it in the instance
  tree" has no row. The instance tree is outside every stamped repo in the
  `visWall` fixture, so there is no commit a test can make in it; what the
  fixture can hold is the half that makes the advice true, the wall
  speaking only in a public repo, which is the private-repo control.
