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

### The second run, and a red that is not this diff

`main` moved from `e2757c0` to `607fc32` during that 858s run — the shelf-life
trap `docs/notes.d/` has recorded before — so this branch was rebased onto
`607fc32` and `make test` re-run there, because a green measured at the old sha
does not describe the tree the launcher actually lands.

That second run is **RED, and not from anything here**:

```
# github.com/ranger360ai/posse/internal/posse [.../internal/posse.test]
internal/posse/herdr_test.go:245:11: undefined: fakeBdDropClosed
FAIL	github.com/ranger360ai/posse/internal/posse [build failed]
```

Every other package stayed green in the same run (root 347.6s, `cmd/posse`
231.6s). This commit adds one Markdown file and cannot affect a Go build, and
the red reproduces on `607fc32` with nothing of this branch applied.

The definition count on `main`, one call per commit
(`git grep -c "func fakeBdDropClosed" <sha> -- internal/posse/herdr_test.go`):

| commit | definitions | |
|---|---|---|
| `455d344` | 1 | fixed the redeclaration two seats had created |
| `e2757c0` | 1 | where the green above was measured |
| `c9820d1` | 1 | |
| `5b4e686` | **0** | took out the last remaining definition |
| `607fc32` | **0** | |

`5b4e686` (ranger-base-5im1q) was written against a `main` where BOTH copies
were live and says so in its own message; by the time it landed, `455d344`
(ranger-base-pju9t) had already removed one. It then removed the other,
believing it was the surviving duplicate — a stale fix that was correct when
written and wrong when it landed, over an already-corrected defect. The call
at line 245 and all three doc comments survived, which is the tell.

Filed as **ranger-base-44jvd** (P0) with the recovery, rather than fixed here
under strict scope. Note `ranger-base-jhyiv` is open against the same symbol
and file in the OPPOSITE direction ("duplicate fakeBdDropClosed"), a condition
that stopped being true at `455d344`.

Worth keeping for the next reader: `go build ./...` is **green** on this
defect, because the missing symbol lives in a `_test.go` file. Only a TEST
build asks — `go vet ./internal/posse/` or `go test -c ./internal/posse/`.

**Resolved while this was being written, by another lane.** `6ecb521`
(ranger-base-jzoci, gwart, 2026-09-04 08:20:17) restored the definition —
`git grep -c` gives 1 again at that commit. That bead was filed independently
from the push lane as an escape from ranger-base-hlv21 about seven minutes
before this one, so **ranger-base-44jvd is a duplicate** and is closed
pointing at `6ecb521`. Kept in this note anyway, because the diagnosis is the
durable part and the shape recurs: the count table above is one `git grep -c`
loop and it names the over-removal in a single reading.

The reason two beads exist for one red is worth its own line. Both were filed
inside a ten-minute window by seats that could not see each other, which is
what happens when `main` goes red on a busy queue — every seat rebasing onto
it discovers the same break at once. Checking the open list first is
necessary and was done here (122 open beads scanned, no hit for this symbol in
this direction), and it is still not sufficient, because the duplicate had not
been filed yet at the time of the scan.
