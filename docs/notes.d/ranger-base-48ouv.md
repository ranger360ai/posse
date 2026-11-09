# ranger-base-48ouv — verifying four closes, and the one pin gap they left

Verify bead over `ranger-base-4r8fw`, `ranger-base-gi1gi`, `ranger-base-ppnds`
and `ranger-base-yqmkr`. All four VERIFY. Evidence taken at
**0ea8248f** (= `main` at the time), darwin 25.6.0 arm64, git 2.50.1 (Apple
Git-155), one-minute load 7.2-12.8.

Everything below was re-measured here rather than read off the closes, because
a close's own mutation table is a claim like any other.

## ranger-base-4r8fw — the recipe, run literally, at n=3

The closes's pins drive the recipe from Go. A seat reads the TEXT and runs it
in a shell, so the text was extracted from `mergeBlockedReplay` and run as
written, on a **three**-commit branch — the bead's own motivating case, where
the pinned fixture replays two.

Fixture: `shared.mk` seeded; three originals on `posse/s-1` — a conflicting
edit, a second conflicting edit **plus a new file**, and an edit to a path
`main` never touches; then `main` moves in the same line of `shared.mk`.
`MergeSessionWork` reports Blocked. Replayed one commit per original from a
seat worktree on its own branch at `main`:

| reading | result |
|---|---|
| deletion guard, each original | empty — nothing deleted |
| `cat-file blob "$TREE:<p>"` for the NEW file | works; `git add --` then carries it |
| original 3 (path `main` never touched) | merges CLEAN, no markers — and still replays |
| whole-SET key read-back | **no diff** — every key paired |
| `equivalentOnBase` after the launcher's fast-forward | **3 pairings**: 1 `byPatch`, 2 by replayKey |

The same fixture replayed as ONE squashed commit — what the old copy
prescribed — gives `equivalentOnBase` = **0 pairings**, i.e. nil, so the block
stands and re-files forever with the work entirely on `main`. Both ends of the
bug reproduce independently.

Two things the recipe's audience makes load-bearing, worth knowing:

- **"In YOUR tree, on `<base>`" means a branch AT the base, not the base
  branch itself.** Run with `HEAD` literally on `main`, the read-back's
  `base..HEAD` is empty by construction and reports all N keys missing — a
  false alarm. A dispatched seat is on `posse/<session>` at `main`, where it
  reads correctly. Noted because the pinned fixture commits in the main
  checkout, so it never exercises the read-back.
- A clean-merging path is normal and fine, but it pairs `byPatch`. The honest-
  half pin `t.Fatalf`s on an unconflicted path on purpose, so its fixture
  forces every path to conflict; the mixed case is only measured here.

### MUTANTS — and four that survived

Against `internal/posse/dispatch.go`, each reverted alone, pins =
`MergeBlockedReplayNamesTheCount|MergeBlockedBodyTellsTheSeat|MergeBackPairsEveryCommit|RebaseRecommendationScannerDiscriminates`:

| mutant | verdict |
|---|---|
| `--merge-base=$C^` → the branch's base (**the actual historical bug**) | KILLED |
| drop `--reverse` from the originals' walk | KILLED |
| author date `%aI` → `%cI` on the commit line | KILLED |
| blunt `ONE REPLAY COMMIT PER ORIGINAL` | KILLED |
| the count never renders | KILLED |
| deletion guard operands reversed (`HEAD $TREE` → `$TREE HEAD`) | **SURVIVED** |
| the whole-SET read-back DELETED outright | **SURVIVED** |
| read-back measures `%cI` instead of `%aI` | **SURVIVED** |
| drop the `OLDEST FIRST` prose | SURVIVED (redundant — `--reverse` is pinned) |

The decisive mutant is killed, so the close delivers what the bead asked. The
three real survivors are operands the close ADDED beyond the ask and did not
pin: the only existing assertion is the bare substring `--diff-filter=D`, and
nothing anywhere named the read-back. **Closed here** by three assertions in
`TestMergeBlockedReplayNamesTheCountAndThePerCommitLoop`, each mutation-checked.

Why each matters: reversed, the deletion guard reports the paths the merge
ADDED and misses a real deletion — and a dropped deletion still *pairs*,
because replayKey is an identity and never a measurement of content, so the
branch reads as landed over a hunk that never arrived. The read-back is the
step that tells a seat it stranded a commit, which is the failure the whole
bead exists for. On `%cI` it would compare committer dates a replay always
rewrites, screaming on every correct replay and meaning nothing.

**A "KILLED" with no assertion named is a build failure.** Deleting the
read-back first scored as a kill; deleting it by line instead compiles clean
and leaves every pin green. Counted as a survivor only after `go build` was
shown to pass.

## ranger-base-gi1gi — D20, re-measured

All five DONE-WHEN render shapes are pinned and green (`posse/<s>`, nested
`a/b/c`, slashless, detached HEAD, main checkout), and the live arms skip off
Linux as asked. Both of the close's findings reproduce **exactly**, with a
clean positive control (every verb at exit 0 before the file is planted):

| keep file in `refs/heads/posse/` | `fsck` | `gc` | `log --all` | `show-ref` |
|---|---|---|---|---|
| `.posse-keep` (shipped) | **exit 8** `badRefName` + `badRefContent` | 0 | 0 | 0 |
| `posse-keep` (no dot) | exit 10 | **128** `failed to run repack` | **128** | **128** |

`pack-refs --all --prune` leaves the directory in **both** arms, so the keep
file does its job either way — the dot is what keeps the OPERATOR's git
working. ADR 0080's amendment, its measured-versus-assumed ledger (narrowed to
just the mountpoint question) and the probe's corrected T4d row all agree with
this. The `(ASSUMED, probe T4c/T4d)` still in the D20 decision letter is the
letter, amended by *As built* below it — the same convention D17 is read under.

## ranger-base-ppnds — all three findings, with positive controls

Findings 1-3 were filed off `ranger-base-vnp01`; each fix was reproduced here
from the bead's own REPRO rather than from the close's fixtures.

- **Finding 1 (wrong-kind prep).** REPRO (a) main checkout and (b) linked
  worktree: **0** wrong-kind steps, `DenyFile` carrying 4 and 9 entries.
  Positive control — the `carve.FileShaped(d)` skip removed — puts the defect
  straight back: `mkdir -p .git/config.lock` in (a), and in (b) also
  `config.worktree` and `config.worktree.lock`, i.e. exactly the paths the
  finding named. 3 and 5 wrong-kind steps against 0 and 0.
- **Finding 2 (own() asks containment).** The five MEASURED rows re-run against
  the shipped `credentialReadDenyLiterals`: the absolute spelling and the
  trailing slash are now **spared** on Linux (both were denied), the two
  built-in spellings stay spared, the overlay stays denied, and the darwin
  column is unchanged in all five. The close's defence of the overlay row
  holds: with `state_dir: [/srv/claude-state]` the overlay's OWN credential is
  spared in all three spellings on both OSes, so the denied path is the
  undeclared tree and the seat can still authenticate.
- **Finding 3 (the arm that hid finding 1).** Both of the close's mutants
  KILLED: removing the `FileShaped` skip reds `RendersTheTiersOwnLists` and the
  focused file row; restoring `own()` to the original `~/.claude` literal
  equality reds the absolute-spelling, trailing-slash and dropped-tree rows and
  leaves the controls green.

## ranger-base-yqmkr — closed on a decline, and the citations hold

Items 1 and 3 are MEASURED. Item 2 is **not**, and cannot be: the spend ask
`ranger-base-uzb3s` came back DECLINE (operator, 2026-11-08, via monica), with
the direction that this bead "closes on what is built and probed, with the live
grok settle recorded as unproven". The close does exactly that.

The citation chain was verified rather than repeated: ADR 0083 line 65 reads
`claude is the only runtime on the host for now (ADR 0080 D6)` verbatim, ADR
0083 names grok **zero** times, and ADR 0080 D6 is indeed where codex and grok
are left to their own CLI logins. `make verify-grok-pin` green, pin intact at
1.0.52. No stale "the shakedown measures it" claim survives in any `.go`,
`.toml` or `.sh`; the one hit in ADR 0076 is the decision letter, with its
*MEASURED* amendment immediately below naming the decline.

**Stated as this verification's limit:** the bead's DONE WHEN — "grok
dispatches and settles on both multiplexers" — is NOT met, and the ADR is
correct that a dispatched settle was reached on *neither* multiplexer. That is
an operator decision, not a gap in the work, and it is recorded as UNPROVEN
where a reader will find it.
