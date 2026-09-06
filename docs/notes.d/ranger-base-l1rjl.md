## Two seats wrote two pins for one red; main's is a superset (ranger-base-l1rjl)

`merge-back blocked: posse/gwart-posse-ranger-base-ve1g5 does not land on main`
names a real rebase conflict, and the conflict is a **duplicate fix with a
different mechanism**. `ranger-base-ve1g5` and `ranger-base-mg7si` were both
cut for one red — `TestVerifyPruneGuardScriptPinsThreeFieldGen` asserting the
regex literal `dcbbee8c` deleted (`ranger-base-s8b4g`) — and each replaced that
literal with a pin that RUNS the script. `mg7si` landed on `main` as `1674846b`;
`ve1g5`'s `1602cf28` did not. **Verdict: DO-NOT-LAND.** This is residual class 2,
*superseded*, of `docs/notes.d/ranger-base-d91mf.md` — replaying reds the suite,
and here it reds it by construction rather than by accident.

### Why replaying reds `main`, measured

`1602cf28` is two files, and only one of them is the pin. It also refactors the
script: the `gen:` arm's decision is lifted into a new `gen_shape()` beside
`digits()`, and the arm becomes `gen_shape "$gen"; case $? in …`. `main`'s pin
extracts the arm's own text and runs it under a prelude of `meta_field` and
`digits` — **not** `gen_shape`, which did not exist when it was written. So the
script half alone, applied to `main` in this worktree, reds `main`'s pin:

```
git show 1602cf28 -- scripts/verify-prune-guard.sh | git apply -
go test ./internal/posse/ -run TestVerifyPruneGuardScriptPinsThreeFieldGen

--- FAIL: …/what_genToken_emits
    verify-prune-guard's gen: arm returned 1 for "gen: 66:587500:1787577362616440001",
    want 0 (detail "gen: present but not N:N:N — …")
--- FAIL: …/the_pre-fjj_two-field_token
    … detail "gen: present but not N:N:N — gen: 66:587500" does not carry "two-field"
```

`gen_shape` is undefined inside the extracted harness, so bash exits 127 and
every token falls to the `*)` branch. Restored with `git checkout --`, and the
same command is green at `main`'s tip (7/7 subtests, 0.67s).

### The residual is a rename, not coverage

`git diff main 1602cf28 -- <the two paths>` is the whole question. Both pins
feed the ACCEPT arm `genToken`'s own output; the difference is what else they
reach.

| case | `main` (`1674846b`) | branch (`1602cf28`) |
|---|---|---|
| what `genToken` emits | ✓ | ✓ |
| the pre-fjj two-field token | ✓ **+ detail says "two-field"** | ✓ |
| one field / non-digit / empty third / fourth field | ✓ **+ detail says "not N:N:N"** | ✓ |
| **no `gen:` stamped at all** — the arm's `else` | ✓ | **absent** |
| the refusal MESSAGES the script owes | ✓ | **absent** |
| `gen_shape` writes nothing to the terminal | n/a | ✓ |

`main` runs the whole arm, over planted metas, through `meta_field`, and grades
the detail; the branch runs one lifted classifier over tokens and grades exit
codes only. **`main`'s pin is a strict superset of the branch's, plus two arms
the branch has not got.** What the branch carries alone is `gen_shape()` itself
— a naming refactor of parameter expansion that already forked nothing, so it
changes no behaviour, and landing it would mean re-aiming a pin that landed
hours earlier to buy nothing measurable. Left out deliberately; if the script
should read that way it is a fresh bead against `1674846b`, not this one.

### The branch, afterwards

Every arm of `equivalentOnBase` is blind to this, correctly — nothing of
`1602cf28` is on `main` under any sha. An in-package probe against the live
repo (read-only, deleted after):

```
equivalentOnBase(repo, "main", "posse/gwart-posse-ranger-base-ve1g5")
  len(eq)=0   measuredOnBase=false      # same for the sha directly
git cherry -v main posse/gwart-posse-ranger-base-ve1g5   → "+"
```

The in-force binary agrees, which is `docs/notes.d/ranger-base-8nsc6.md`'s
reading order done rather than assumed — `posse worktrees` names the count and
no pairing at all, where a paired branch prints the arm that paired it:

```
posse/gwart-posse-ranger-base-ve1g5
  ~/.posse/worktrees/… → main  ·  1 commit(s) not on main, for ranger-base-ve1g5
```

- **Not re-filed.** `c3ab9188` (`ranger-base-j8qmj`) reads closed blocks in the
  dedupe, and `git merge-base --is-ancestor c3ab9188 12dd7b9` is true, so the
  arm is in the in-force binary (`posse 0.4.0+12dd7b9`). The branch tip's
  committer date is `2026-09-05 20:57:48 -0400`, before this verdict, so
  `tip.After(prior.Verdict)` is false. If it files again, the branch moved or
  the binary regressed — check those two first.
- **No delete licence.** `measuredOnBase` is false, so `RemoveSessionTree` will
  keep refusing `…/worktrees/posse/gwart-posse-ranger-base-ve1g5` with
  "holds 1 commit(s) main does not". That refusal is right — the branch is the
  last copy of `gen_shape()` — and retiring the tree stays a human's act:
  `git worktree remove` + `git branch -D`.

### Verified

Read-only against `main` throughout; nothing of the branch is landed. This
commit adds one Markdown file and cannot move a Go build.

| | |
|---|---|
| `TestVerifyPruneGuardScriptPinsThreeFieldGen` at `b96a1080` | ok 0.673s, 7/7 |
| the same with the branch's script half applied | **2 FAIL** (above), restored |
| `git status --porcelain` after restore | empty |

`make test` on the note commit at `b96a1080`: **every Go package `ok`** — root
354.7s, `cmd/posse` 211.5s, `cmd/testparallel` 2.3s, `internal/posse` 922.6s,
zero `FAIL` lines anywhere — and **`RC=2` from a red that is not this diff**.
The whole of it is the last step, `scripts/audit-silent-reverts.sh`, on ONE
untriaged hit: `b96a108`, which is `main`'s own tip at the time, deleting the 16
`docs/adr/history/` copies the same ADR-simplification batch had just created.
This commit adds one Markdown file under `docs/notes.d/` and cannot reach it.

`main` moved twice during that 922s run — `0642fa28` (ranger-base-rq689) triages
exactly that hit, and `aab26b76` — so this branch was rebased onto `aab26b76`
and re-measured there, on the FINAL bytes:

```
scripts/audit-silent-reverts.sh --quiet   1447 commits, 0 untriaged   rc 0
go test -run TestVerifyPruneGuardScriptPinsThreeFieldGen              ok 0.396s
  the same with the branch's script half applied                     2 FAIL
  git checkout -- ; git status --porcelain                            empty
go build ./…  ·  go vet ./…                                          rc 0 · rc 0
make fmt-check verify-test-times verify-parallel verify-suite-lock \
     verify-silent-reverts tree-check                                rc 0
```

The Go half is not re-run at `aab26b76`: those two commits are a triage line in
`scripts/silent-reverts.allow` and one `internal/posse` test file, and the arms
above are the ones a new Markdown file plus a moved base can actually move —
`docs/notes.d/ranger-base-fm23s.md`'s rule, and the gap is stated rather than
hidden.
