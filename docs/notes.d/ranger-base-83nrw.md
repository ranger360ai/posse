# ranger-base-83nrw — the first multi-commit merge-back replay, and a pin whose mutant got quieter under the merge

Landed 2026-11-08, git 2.50.1 / macOS 25.6.0, go 1.26.10.

`ranger-base-8kwa5` closed with **three** commits on
`posse/gwart-posse-ranger-base-8kwa5` (`193ce13`, `ef54685`, `93c3cec1`) and
`main` moved on underneath it by seventeen. The launcher's replay conflicted,
aborted, and filed this bead. `docs/notes.d/ranger-base-xea2y.md` is the
procedure — `merge-tree --write-tree`, `cat-file blob` per path, no sequencer,
nothing to leak — and it held. Two things are new, and they are why this entry
exists: **the branch carried more than one commit**, which changes what the
replay has to produce; and **the merge quietly narrowed a pin's mutant**, which
is the kind of thing a green suite cannot report.

## THREE commits in, three commits out — this is not a squash

`equivalentOnBase` (internal/posse/worktree.go) is what tells a later pass
that a blocked branch's work has reached `main` under other shas, and
`blockStillStands` ends the block on `len(equivalentOnBase(...)) > 0`. It walks
`rev-list base..tip` **one commit at a time** and `return nil`s on the first
one it cannot account for. A hand-resolved replay writes no `-x` trailer and
its resolution changes the patch, so `git cherry` says `+` and the trailer
lookup finds nothing: the only arm that can pair it is `replayKey` —
`(author email, AUTHOR date, subject)`.

**So a three-commit branch replayed as one commit pairs one commit and strands
two, and the block re-files forever.** Every previous entry of this class
(`ranger-base-xea2y`, `-3e68k`, `-h1q5o`, `-v3tt4`, `-yrag8`) replayed a
single commit, so nothing had said this yet. The recipe is per commit:

```
# ours starts at main; theirs walks the branch
T=$(git merge-tree --write-tree --merge-base=<prev original> HEAD <this original> | head -1)
git diff --diff-filter=D --name-only HEAD $T      # must be empty: the loop only writes
git cat-file blob "$T:<p>" > <p>                  # per path
# resolve, then:
GIT_AUTHOR_NAME=... GIT_AUTHOR_EMAIL=... \
  git commit -F <msg> --date="$(git log -1 --format=%aI <this original>)" -- <paths>
```

The merge base for commit *n* is **original commit *n-1***, not the branch's
own base — the whole point is that "ours" is the replay so far. Checked at the
end by comparing the key itself, which costs nothing and is the only thing
that matters:

```
diff <(git log --format='%ae%x1f%aI%x1f%s' main..<work> | sort) \
     <(git log --format='%ae%x1f%aI%x1f%s' main..HEAD   | sort)
```

All three identical. `--date` is enough and `--amend` would not be: amend
ignores `GIT_AUTHOR_DATE` (AGENTS.md says so for the shared checkout; it is
the same function here).

## The conflicts, and which ones were real

Whole-branch (`merge-tree main <tip>`): one conflict,
`docs/notes.d/README.md`. **Generated** — regenerated with
`scripts/notes-index.py`, never hand-merged, which is `xea2y`'s standing
lesson and held for the sixth time.

Per commit, three more appeared, and only one of them was a question:

- **`internal/posse/buildsource_qa_test.go`, in commits 1 and 2** — and this
  is the shape worth recognising, because it is a conflict that *is not one*.
  `193ce13` replaced `TestQATheWallRendererReadsTheSameCandidateList` with a
  go/ast census; `main` rewrote the same test in place
  ("ranger-base-l7sb2: the one-list pin is asserted through WallRenderer");
  `ef54685` withdrew the census precisely because main's fix is better, and
  **its body states the merge outcome in as many words** — *"this branch now
  leaves buildsource_qa_test.go byte-identical to its merge base, so the
  launcher's three-way merge takes main's side with no conflict"*. So the
  resolution was written down before the conflict existed: take main's side,
  one commit earlier than the branch did. Replaying the census would have
  meant hand-resolving against the better fix in order to delete it in the
  next commit — a tree nobody wants, in a commit that could not be green.
  **Read the branch's own commit bodies before resolving. A withdrawal commit
  often names the merge it was anticipating.**
- **`internal/posse/queuewallowner_qa_test.go`, in commit 2** — my own
  commit-1 stopgap against `ef54685`'s rewrite of the same region. Took
  theirs; it supersedes the stopgap, which is that commit's whole subject.
- **The one that nearly ate main's work.** Taking `ef54685`'s version of that
  file *wholesale* — `git cat-file blob ef546853:<p>` rather than the merged
  blob — compiled, looked clean, and silently dropped main's
  `qaStampedHome`/`qaStampedBy` hunks in the **shared fixture** two hundred
  lines above the region in conflict. It reddened two arms immediately, so it
  cost minutes rather than a landing; a side whose hunks were only prose would
  have landed. **"Take theirs" is a resolution for a conflicted REGION and
  never for a file.** Take the merged blob; resolve between the markers.

## The evidence reading, in both directions, because both sides edited both files

`xea2y`'s rule is that a textually clean auto-merge of a file both sides
changed is not evidence. Both surviving files qualified — `main` had landed
three commits into `queuejsonl.go` and one into the test — so each was read
against both parents:

| file | vs `main` | vs the branch tip `93c3cec1` |
|---|---|---|
| `internal/posse/queuejsonl.go` | 1 hunk, exactly 1 deleted line (the old `return filepath.Clean(ExpandTilde(v))`) | exactly main's 3 hunks: `underSameDir`, D7's `CanonicalAbbrevPath` render, `GovRows` |
| `internal/posse/queuewallowner_qa_test.go` | 2 hunks, **no** deleted line at all | exactly main's 3 lines: `qaStampedHome` twice in `qwoForeignWall`, `qaStampedBy` in the arm above |

A deletion count against `main` is the cheap half of this and catches the
dropped-hunk case directly: the replay only ADDS the branch's work, so any `-`
line that is not one the branch's own diff removes is main's work going out.

## The pin whose mutant got quieter, which is the finding

`ranger-base-8kwa5` finding 1 is one `filepath.Abs` in `QueueRepo()`, pinned
by `TestQAARelativeQueueRepoIsRefusedByItsAbsolutePath`. While that bead was
open, `main` landed **ADR 0082 D4** — `ranger-base-w79sc` at 05:51 and
`ranger-base-qm7eg` at 06:15, against branch commits authored 02:56–03:06 —
putting both of ADR 0081 D7's path renders through `CanonicalAbbrevPath`.

`canonicalPath` goes through `resolvedPath`, which `Abs`'s against the
**rendering** process's cwd. So D4's render absolutizes the printed key on its
own, and three things that were true of this pin stopped being true:

1. **The MEASURED refusal the bead quotes cannot be reproduced.** `queue_repo:
   001` with the cwd at the repo's parent no longer prints `remedy: add
   `001: <private|public>`` — it prints the absolute key, fix or no fix. The
   quote is now a record of the defect, not a reading. Both the production
   comment and the fragment say so where they quote it.
2. **The mutant reds once, not two to four times.** MEASURED 2026-11-08 on the
   merged tree: with `QueueRepo` back to `filepath.Clean(ExpandTilde(v))` only
   the premise (`IsAbs` + `samePath`) reds. Every assertion below it passes,
   because the refusal prescribes an absolute key regardless. Commit 1's
   header claimed four reds and commit 2's fragment claimed two; the figure on
   main is one, and both are corrected in place.
3. **And three of those assertions could never have measured a spelling.**
   They are unanchored `strings.Contains` of a `/var/...` fixture path, and
   `/private/var/...` *contains* `/var/...`. Only the key read out with its
   backticks (`qwoPrescribedKey`) is a reading of the spelling at all — which
   is why exactly one assertion reddened when D4's renderer first met commit
   1's version, and why the property form passes it unchanged.

**Nothing is uncovered by (2), and that is the part a reader should not have to
re-derive.** The premise *is* the pin's subject — the value must be absolute
before the four predicates read it — and the premise is what reds. The
refusal's *spelling* is pinned where D4 decided it, in
`queuerefusalspelling_qa_test.go`. MEASURED here, both directions, each red in
one file only:

| mutant | reds |
|---|---|
| `QueueRepo`'s `filepath.Abs` removed | `queuewallowner_qa_test.go`, the premise, 1 assertion |
| D4's render reverted to `AbbrevHome` | `queuerefusalspelling_qa_test.go`, 2 arms, 3 assertions each; `queuewallowner` stays **green** |

That is `ranger-base-w79sc`'s own claim — *"both pins hold; neither subsumes
the other, and a revert of either is red in one file only"* — checked on the
tree where the two first met, which is the one place it could not have been
checked when it was written.

**The general lesson is about the fence, not about these two beads.**
`ranger-base-8kwa5` wrote the pin to assert a PROPERTY rather than a spelling,
explicitly because `ranger-base-wvoqe` ("should `QueueRepo()` canonicalise?")
was open and a spelling pin would red the day it was answered yes. It was
answered hours later — **yes for the RENDER, no for the VALUE** — and the
commit that first wrote the pin as a spelling is exactly the commit that
reddened. The fence worked. What it does not do, and cannot, is tell you that
your mutant got weaker: a property pin survives a renderer change by being
indifferent to it, and indifference reads the same as coverage. **Re-run the
mutation check on the merged tree, not only on the base you wrote it against.**

## ranger-base-wvoqe is closed, so three places stopped being deferrals

`QueueRepo()`'s production comment, the pin's header and the fragment all said
the canonicalise question was "handed to architecture" / "an open architecture
question". It is closed: yes for the render, no for the value, `QueueRepo()`
stays on `filepath.Abs`, `underDir` keeps the comparison it makes, and
`canonicalPath` never feeds a comparison. All three now state the decision.
The fragment also records the two rejected alternatives, because they are its
own three grounds read back: canonicalising in `QueueRepo()` changes three
comparisons on one side only and breaks a config and store each spelled
differently from the disk; canonicalising both sides of `underDir` is the
string folding `samePath` says cannot be done.

## The silent-revert allow line is deliberately unclaimed, and says so

`93c3cec1` added a `scripts/silent-reverts.allow` line triaging `ef54685`'s
rollback of `buildsource_qa_test.go`. **That rollback is not in main's
history** — the replay took main's side one commit early — so the line triages
nothing here. It is kept, because it is the statement of intent the convention
requires and because the audit tolerates an unclaimed line by construction
(the sha match is tried first, the token is simply unclaimed; ADR 0054 D2/D3),
and the line itself now opens that clause rather than leaving a reader to
discover it with a `git show` that resolves only in the shared repo. MEASURED:
`scripts/audit-silent-reverts.sh` exits 0 and prints no untriaged hit.

## Verified

`make fmt-check`, `make crew-check`, `make notes-check`, `make tree-check`
(green, 82.6s at a one-minute load average of ~4), `scripts/notes-index.py
--check`, `go vet` untagged and `-tags posse_arm3`, and **`make test` whole**.
The three replay commits were each checked at their own tip with `go vet -tags
posse_arm3` and the two queue-wall arms before the next was written.

`make test` is proven whole by its LAST recipe line rather than by an exit
code, which is worth saying because the exit code was not available: the run
was piped to `tail`, so the status the harness reported was `tail`'s. make
aborts a recipe on its first non-zero exit and
`@scripts/audit-silent-reverts.sh --quiet` is the final line of the `test`
target, so `silent-revert audit: 2381 commits, 0 untriaged` in the output is
the proof that `tree-check`, the arm-tags type-check door, `./...`,
`-tags posse_arm2` and `-tags posse_arm3` all passed before it. Zero `FAIL`
lines; `internal/posse` ok three times at 414.794s, 434.818s and 688.412s.

**And a first attempt at that run was killed, which is the harness lesson.**
`nohup make test &` inside a Bash call that was ITSELF backgrounded reports the
wrapper's exit — "completed (exit code 0)" after ~3 minutes, over a log whose
last line is a `go test` invocation with no result under it — while the real
`make` dies with the wrapper's process group. Pass `make test` to the tool with
`run_in_background` and no inner `&`. That first green was not a green.

`go run ./cmd/checkorphans` exits **1**, over one leaked gate-shell child that
is **not this session's**: pid 2444, ppid 1, its argv sourcing
`snapshot-zsh-1794165372051-yn6vpv.sh`, whose embedded timestamp is 14:16:12
against this session's scratchpad created 18:42:38 — 4h26m earlier, and
`pgrep -fl <this worktree path>` is empty. It is named rather than passed over:
an undeclared orphan on this box is the load guard's kill arm's business and
some other seat's to own. `ps` is refused from inside the cage
(`operation not permitted`), which is the documented reason checkorphans reads
`sysctl kern.proc.all`; the dating above is the fallback path AGENTS.md
prescribes when the tool's own verdict has to be split from someone else's.
