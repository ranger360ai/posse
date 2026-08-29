## The move exception asks git, at a chosen 50% (ranger-base-en75)

`scripts/audit-silent-reverts.sh` excused a deletion only when the **identical
blob** appeared at another path in the same commit. A rename that *also edits*
the file is a different blob, so the exception could not see it and the deletion
half was reported as a silent revert. Three commits in ~460 paid a triage line
for that shape — 631bda7 (the rhq → posse rename, four of them), e82338c
(`etc/cleanroom/{Dockerfile => Dockerfile.debian}`) and 2eae58a
(`examples/agents/{ranger.md => ops.md}`) — and the rate was not falling.
e82338c's own triage line named the trigger: *"a third one means fixing the move
exception to ask git for rename similarity instead of exact blob identity."*
2eae58a was the third; it was triaged under ranger-base-ae6k because that bead
was a P1 release unblock, and a change whose failure direction is a false
*negative* does not land inside one.

**The fix.** `raw_log` passes `--find-renames=50% -l0` where it passed
`--no-renames`, and `states_awk` learns the `R` status. A raw rename line is
`:<mode> <mode> <src> <dst> R<sim>\t<srcpath>\t<dstpath>` — **two paths on one
line**, which the single-tab path parse read as one path literally named
`src<tab>dst`. It decomposes into the two entries it stands for: the source
leaves (a deletion, but a moved one, suppressed) and the destination arrives
with the possibly-edited blob, **still compared** against every state that path
has held, so a re-land *through* a rename is still caught. The old exact-blob
rule is kept underneath and now also reads rename destinations, so a commit that
deletes three identical files while adding one copy behaves as it did before.

**The number is chosen, not inherited.** This is a false-negative widening and
the threshold is the whole of its width:

| | |
|---|---|
| the two live strikes | R060 (e82338c) and R097 (2eae58a) — 60% is the *tightest* value that clears both, i.e. zero margin, and the next rename+edit at 55% buys the triage line back |
| measured cost at the wide end | over this repo's 504 commits git pairs exactly **three** deletions with an add even at `-M30%` — the two above plus one R100 exact move — and all three are real renames. Observed false-pairing rate at 50%: 0/504 |
| what stays at risk | a stale-index commit that deletes a newly-landed file while adding a ≥50%-similar one **in the same commit** goes quiet. The exact-blob rule it replaced had the identical hazard at 100% |
| `-l0` | git skips inexact rename detection once the file count passes `diff.renameLimit`, whose default has moved across git versions, and warns only on stderr. That is the ranger-base-hhcu shape again: one history, two verdicts, decided by the toolchain. Unlimited costs 0.55s over full history. Documented, **not** pinned — reproducing it needs a commit with >1000 files |

**Verification.** Flagged set over 504 commits went `{e82338c, 2eae58a,
a7b80a4, 71fa30f}` → `{a7b80a4, 71fa30f}`. The two that dropped are the two
rename+edits; nothing else changed and nothing new appeared. Their triage lines
are commented out rather than deleted in `scripts/silent-reverts.allow`, so a
regression brings them back as UNTRIAGED and fails the build. The same captured
raw log was fed to four awks — BWK 20200816 (darwin), mawk 1.3.4, busybox and
gawk 5.3.2 — and all four name the same two commits (ranger-base-hhcu's split
is not reintroduced).

**Three new self-test arms, and why it takes three.** `renameedit` (a
20-line file renamed with 5 lines rewritten, which git scores R065 —
deliberately e82338c's R060 shape and not 2eae58a's easy R097, so raising the
threshold to 75% reds it) asserts silence. `delplusadd` is its wrong arm: a
deletion of a previously-added file plus an *unrelated* add in one commit must
still fire, or the exception is not an exception, it is an off switch. `reland`
is the arm `states_awk`'s `R` branch actually has — **deleting that branch makes
a rename invisible, and invisible is indistinguishable from excused to an arm
that only asserts silence**, so `renameedit` stays green over it. Each plant
carries its own fixture witness (the rename must be *inexact*; the delete+add
must be *unpaired*) on top of the harness-wide SCANNED count.

Four mutations, four pins in `internal/rhq/silentrevert_qa_test.go`, no two
reddening the same arm:

| mutation | reds |
|---|---|
| `--find-renames=50% -l0` → `--no-renames` | rename-that-edits |
| threshold `50%` → `75%` | rename-that-edits |
| `moved=emoved[i]` → `moved=0` | rename-that-edits |
| `if (m[5] ~ /^[RC]/)` → `if (0)` | **re-land, and nothing else** |

Deleting the three arms outright exits 0 and is caught only by
`TestSilentRevertSelfTestHasTheRenameArms` — measured, which is why that pin
exists.
