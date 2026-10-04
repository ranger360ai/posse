# A generated file is reproduced at landing, never replayed (ranger-base-7h8k4)

2026-10-03. `docs/notes.d/README.md` is generated: `python3
scripts/notes-index.py` writes it, `--check` asks whether it is current, and
`internal/treepins`' `TestNotesFragmentIndexIsCurrent` reds on `main` when it
is not. Every seat that writes a fragment is told to run the generator; seats
forget, and the landing — rebase, then fast-forward — had no step that
noticed.

## The measurement

MEASURED 2026-10-03, one day, one landing lane:

| shape | branches | what it cost |
|---|---|---|
| landed with a stale index | 3 (`rb05v` 09:1x, `knux2` 14:1x, `4ch00` 18:0x) | the tree pin red on `main` until a follow-up commit regenerated it |
| could not fast-forward | 5 (`wcy4s`, `sqxo1`, `f1ytb`, `99gww`, `q114b`/`sjwhr`) | the branch's index edit conflicted with `main`'s; each landed by hand with the same regenerate-then-continue |

Both rows are one defect read from two ends: a file nobody writes by hand was
being treated as a file somebody wrote. A hunk against a generated file is not
information — the inputs are, and they are on the branch.

## What landed

`internal/posse/generatedindex.go` and two seams in `MergeSessionWork`
(`worktree.go`):

- **before each fast-forward** (`refreshGeneratedIndex`) the index is
  reproduced on the branch and committed there, so what `main` takes is the
  branch's tip and the tip is current. The commit is the launcher's, path
  limited to the generated file, with the bead id in its body — the same shape
  `LandPersonaMemory` already uses to commit on a persona's behalf.
- **on a stopped replay whose only conflict is the generated file**
  (`continuePastGeneratedIndex`) it is reproduced, staged, and the rebase
  continued, so two fragment branches land back to back with no human in the
  middle.

Reported as a `⟳` line by the sweep, by the judged close, and by `posse
worktrees --land`: a commit nobody in the session typed is one a reader of
`git log` is owed a sentence about.

## Decisions worth the ink

- **Whose generator runs.** The program comes from the BASE's checkout and the
  inputs from the session tree. Running the branch's copy would be the
  launcher executing persona-authored code, uncaged, operator-side, before any
  human read the diff — the class the constitution belt in the same function
  exists for. And it runs only while the two copies are byte-identical, which
  is one question covering both ways they part: a branch that changes the
  generator, and an operator mid-edit on their own. Either way the landing
  writes nothing and the index lands as the seat committed it — not a gap for
  the branch case, because the tree pin runs the TREE's generator against the
  TREE's index, so such a branch has already been asked by its own suite. A
  byte compare and not a diff reading, because the question has to be
  answerable mid-replay, where `base...head` is a statement about nothing.
- **Refuse, never half-land.** An index the landing cannot reproduce — a
  generator that will not run, a commit the wall refuses — is a refusal that
  names the command, never a landing that takes whatever the seat committed;
  and the generated file is restored so the next pass does not read it as the
  persona's uncommitted work.
- **It writes nothing it does not have to.** Gated on the branch having
  changed an input, on the fast-forward being the next step, and on the
  generator's own `--check`. A session tree's mtime is ADR 0058 fact 4, and a
  landing that wrote every tree it looked at would keep the grace clock from
  ever reaching zero (ranger-base-9u5zy's shape).
- **Uncommitted is the persona's.** The generated file is reproducible and
  still theirs: while their copy is uncommitted the landing leaves it alone
  (ADR 0041 §1–§2), and the dirt report is what says it stayed behind.

MEASURED 2026-10-03, git 2.50.1, darwin: `git rebase --continue` OPENS THE
EDITOR for the replayed commit's message — with `GIT_EDITOR=false` it exits 1
and the rebase stays stopped — so the continue runs under
`-c core.editor=true` and keeps the original message. `--no-verify` skips
`pre-commit` and `prepare-commit-msg` still fires, which is exactly the split
the landing commit wants: bd's flush hook (which stages `.beads/issues.jsonl`
and leaves it staged over a tree that already matches HEAD, AGENTS.md's
`rangerhq-be7k`) is skipped, and posse's own commit wall is kept.

## The residual, and the measurement that narrowed it

A branch that already carries a merge-back block on record is re-probed only
when the base has moved past the one that replay ran against AND
`mergesCleanly` says the whole-branch merge would be taken
(`landsweep.go`'s `blockStillStands`).

MEASURED 2026-10-03, git 2.50.1, darwin, in the pin's own fixture: that filter
answers **clean** for exactly this shape. `merge-tree --write-tree` merges the
two branches' index insertions in one go, while the commit-by-commit REPLAY of
the same commits conflicts — the divergence `mergesCleanly`'s own header
states ("a filter and not an authority"), here as a reading rather than a
caveat. So a branch blocked before this landed is picked up by the next pass
whose base has moved; `mergesCleanly` was also the wrong instrument for the
pin's positive witness, which runs the replay and aborts it instead.

What this does not reach is a generated-file conflict `merge-tree` also
refuses: there the verdict stands until a person lands it with the regenerate
the block's own reason names.
