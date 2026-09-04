## A hand RE-LAND under another bead is a fourth landing shape, and no arm sees it (ranger-base-fmift)

`merge-back blocked: posse/dinesh-posse-ranger-base-vbp3 does not land on main`
names a real rebase conflict and reaches the wrong conclusion. Every line of
work on that branch is already on `main` — re-landed by hand on 2026-09-01 as
`0375f22` under **ranger-base-tq93**, squashed 4→1 and rewritten across the
`internal/rhq` → `internal/posse` rename (`631bda7`). **Verdict: DO-NOT-LAND.**

This is the class `docs/notes.d/ranger-base-4ri4n.md` named,
`docs/notes.d/ranger-base-avq12.md` measured and
`docs/notes.d/ranger-base-gn0ch.md` sharpened. What is new here is the landing
shape, not the verdict.

### Why all three arms of `equivalentOnBase` are blind to it

`worktree.go`'s three arms each key on something the re-land destroyed:

| arm | what it keys on | why it missed |
|---|---|---|
| patch-id (`git cherry`) | content of one commit | 3 commits squashed into 1, and every path rewritten |
| `-x` trailer | `cherry picked from commit <sha>` | a by-hand re-land writes no trailer |
| replay identity | author + AUTHOR date + subject | new subject, new author date |

`git cherry -v main posse/dinesh-posse-ranger-base-vbp3` prints `+` on all
four, so `equivalentOnBase` returns nil, which is its honest default — "one
commit it cannot account for" — and the filer read that as a strand. The
`-x`-trailer arm comes closest and still misses by grep: `0375f22`'s body
**names all four shas**, but as prose, not as a trailer.

A fourth arm keyed on "the base names this sha" is **not** proposed here, and
`0375f22` is its own counterexample: that body names the four shas inside the
sentence *"none of 8a93d1b 7e72f50 c381956 05dbcce were ancestors of HEAD"* —
an assertion they did **not** land. The same string is evidence in both
directions, so the arm would pair a note that says "stranded" as proof of a
landing. Left to a human deliberately, which is what the tool already does.

### The accounting, per file, across the rename

Every line the branch ADDED against its merge base (`e3c5cc3`), against
`main`'s copy of the same file at its post-rename path:

```sh
B=posse/dinesh-posse-ranger-base-vbp3
comm -23 <(git diff e3c5cc3..$B -- "$branchpath" | sed -n 's/^+\([^+]\)/\1/p' | sort -u) \
         <(git show main:"$mainpath" | sort -u)
```

| branch path | `main` path | added | branch-only |
|---|---|---|---|
| `NOTES.md` | same | 16 | **0** |
| `docs/notes.d/rangerhq-tr8k.md` | same | 8 | **0** |
| `docs/adr/0013-runtime-dispatch-contract.md` | same | 21 | **0** |
| `internal/rhq/interstitial.go` | `internal/posse/…` | 55 | **0** |
| `internal/rhq/interstitial_qa_test.go` | `internal/posse/…` | 214 | **0** |
| `internal/rhq/runtimecheck.go` | `internal/posse/…` | 11 | **0** |
| `internal/rhq/runtimepreflight.go` | `internal/posse/…` | 15 | **0** |

340 added lines, none of them branch-only. The ADR hunk `0375f22` says it
DROPPED is in that table at zero too: `main`'s §2 carries the amendment plus
the `ranger-base-mzmv` ratification, so the conflict was a duplicate and not a
decision, exactly as the re-land claimed.

A line census is the weak reading (gn0ch's specimen defeats it in the other
direction), so two stronger ones:

- **Every test the branch wrote is on `main`.** All 8 `Test*`/`Benchmark*`
  names in the branch's `interstitial_qa_test.go` are present in `main`'s
  `internal/posse`, and `main`'s copy is a strict SUPERSET: the branch's
  inline `codexHome` fixture is the extracted `codexMenuBack(t)`, planting the
  identical `{"latest_version":"0.150.0","dismissed_version":"0.149.1"}`, plus
  a `stubCodexInstalled` pin the branch never had and four `t.Parallel()`
  marks. Nothing was weakened in the move.
- **The shipped behaviour is on `main`.** `DangerUnsilenced` at
  `internal/posse/interstitial.go:430` no longer SKIPS `in.Probe == nil`; it
  appends the refusal line naming the declaring yaml. That reversal *is* the
  bead's fix, and it is what the branch existed to deliver.

### This close is expected to stick, and that is new

`ranger-base-gn0ch` recorded that closing a block do-not-land is what lets the
next sweep re-file it, because the dedupe read only OPEN beads. That is fixed:
`c3ab918` (**ranger-base-j8qmj**, landed 2026-09-04 05:51) reads closed blocks
too and lets a verdict stand while the branch has not moved. The in-force
binary is `posse 0.4.0+9920e75` (built 06:35, 44 minutes later) and
`git merge-base --is-ancestor c3ab918 9920e75` is true, so the arm is actually
in the binary that files — the reading order `docs/notes.d/ranger-base-8nsc6.md`
asks for, done first, not assumed.

`posse/dinesh-posse-ranger-base-vbp3`'s tip `05dbcce` has a committer date of
**2026-08-30 08:51:55**, five days before this close, and `workHeadTime` reads
that same field off the repo (not the retired tree). So `tip.After(prior.Verdict)`
is false and this block should not be filed a second time. If it is, the branch
moved or the binary regressed — check those two before re-deriving the table
above.

No delete licence follows. `measuredOnBase` is still false for this branch —
none of the four commits is a patch-id twin — so `RemoveSessionTree` will keep
refusing it, and retiring the tree stays a human's act.

### Verified where it has to be true

`make test` on this worktree at `main`'s `e2757c0`: **RC=0**, no `FAIL` line
anywhere, `internal/posse` **ok in 858.0s — 57% of budget**. That is the
package the whole accounting above is about, and its green includes all 8 of
the branch's tests.

One apparatus note for the next reader, because it cost a run: a bare
`go test ./internal/posse/` **reds this package at 600.8s** with a goroutine
dump naming no test — `go test`'s ten-minute-per-package default arriving as a
timeout panic, exactly the shape the `Makefile`'s `-timeout 25m` comment
(ranger-base-2ggb, ranger-base-7xla) says it is. The package needs ~858s here,
so the flag is the difference between a red and a green and the red is the
apparatus, not the tree. Run `make test`, never a bare `go test`, for anything
you intend to read as a verdict.
