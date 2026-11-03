# The landing commit stops skipping pre-commit (ranger-base-7yvo4)

2026-11-03

`commitGeneratedIndex` (internal/posse/generatedindex.go) used to commit the
regenerated notes index with `git commit --no-verify` to dodge bd's flush
hook. That flag skips every pre-commit, including an ADR 0052 employer
scanner, and since ranger-base-r3pri a seat's L1 git shim refuses it — so a
landing run from a persona pane could not land the index at all.

Now every hook runs, and the one cost the flag dodged is cleared the way
AGENTS.md documents it (rangerhq-be7k).

MEASURED 2026-11-03, git 2.50.1 (Apple Git-155), darwin 25.6.0, scratch
repo: a pre-commit that rewrites and `git add`s an unnamed tracked path during
`git commit -m x -- <other path>` puts the flushed file in the commit and
leaves the real index holding the pre-flush blob — `MM` with `git diff HEAD`
empty. `clearHookLeftovers` unstages an entry only when it was not staged
before the commit, is staged after it, and `git diff --no-ext-diff HEAD` over
it is empty; anything else is someone's work and stays.

Consequence worth knowing: if the flush changed `.beads/issues.jsonl`, the
landing commit carries that change, as every persona's path-limited commit in
this repo already does.

Not taken: bd's `--no-auto-flush` is a CLI flag the hook shim never receives
(`exec bd hook pre-commit "$@"`), and no env switch was found in bd 0.50.3.
An absolute git path past the shim is the same skip with the wall removed.

Pin: `TestTheLandingCommitRunsTheManagedHookAndLeavesNoFlushBehind`
(landgeneratedindex_test.go, arm 2) — reds on `--no-verify` (marker never
fires) and on a missing clear (`MM .beads/issues.jsonl`), both observed.
