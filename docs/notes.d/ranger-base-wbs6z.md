## A landing asked by SHA is a landing you cannot see (ranger-base-wbs6z)

ranger-base-wbs6z was filed P1 against the merge-back: a CLOSED bead's only
commit was "on no ref", its work "not on `main`", the object "reachable only
as a loose/packed object" and so TIME-SENSITIVE against a `git gc`. Every
command in that report ran and every output it quotes is real. The conclusion
is wrong, and the work had already landed twelve minutes before the bead was
created.

**Nothing was recovered, because nothing was lost.** ranger-base-2emry's work
is on `main` as `db608a65`, and the merge-back that put it there is in the
reflog of `main` under the branch it came from:

```
db608a65 main@{2026-11-08 07:30:26 -0500}: merge posse/jian-yang-posse-ranger-base-2emry: Fast-forward
82533ee5 main@{2026-11-08 07:25:02 -0500}: merge posse/dinesh-posse-ranger-base-vkmv3: Fast-forward
```

MEASURED 2026-11-08 from this worktree (HEAD = `main` = `db608a65`):

| question | answer |
|---|---|
| `git patch-id --stable` of `58dc4fa0` and of `db608a65` | `06fde9f0…` both — the same patch |
| `git cherry -v main 58dc4fa0` | `-` — git's own verdict: an equivalent commit is in `main` |
| `git log --grep ranger-base-2emry --oneline main` | `db608a65` |
| `git rev-parse 58dc4fa0:internal/posse/init.go` vs `db608a65:` | identical blob `9c1db67b` |
| the same for `initoperatorfence_qa_test.go` | identical blob `69ed9d34` |
| `git diff f04ecbcff 58dc4fa0 -- <the ADR>` vs `git diff 82533ee5 db608a65 -- <the ADR>` | the same patch, hunk for hunk |
| `go build ./...` at `db608a65` | exit 0 |
| `go test -tags posse_arm3 -count=1 -run TestQAInitFence ./internal/posse` | **8 arms, all PASS**, 0.416s |
| `internal/posse/init.go` on `main` | `underSameDir(origin, a.Home)` at line 144 |

So the P1 the bead says is "still live on main" is fixed on `main`, the two
case arms it says are missing are arms 7 and 8 of eight, and the ADR bullet it
says did not land is there with its text unchanged. The only blob that differs
between the pre- and post-rebase commit is the ADR, and it differs because
`82533ee5` had edited the same file elsewhere — the rebase merged both and
dropped nothing.

### Why the sha is on no ref, which is the part the bead asked for

Because that is what a merge-back IS. `MergeSessionWork`
(internal/posse/worktree.go) runs `git rebase <base>` in the session worktree
and then `git merge --ff-only <branch>` in the shared repo — "ONE
compare-and-swap on the base's ref", as the comment above the loop puts it.
When the base moved while the session worked, the rebase **rewrites every
commit on the branch**: new parent, new committer date, new sha. `58dc4fa0`
and `db608a65` carry the same author date to the second (07:25:05) and
committer dates five minutes apart (07:25:05 → 07:30:26), which is a replay
and nothing else.

`main` had moved: `82533ee5` landed at 07:25:02, three seconds before
`58dc4fa0` was authored on `f04ecbcf`. So the rebase was not optional and the
sha change was not a fault. **After any merge-back that rebased, the sha a
session saw in its own worktree is contained in no ref, forever, on the normal
path.** `git for-each-ref --contains <that sha>` is empty and
`git merge-base --is-ancestor <that sha> main` exits 1 for every such landing
— which is most of them on a box this busy. Both commands answered the bead
honestly; neither was a question about whether the work landed.

**This is written down already, for posse's own code.** From
`docs/notes.d/ranger-base-dybv.md`, on the postcondition that decides
`MergeOutcome.Merged`: *"Ask it of HEAD as it is now, never of a sha captured
on the way in. A rebase legitimately rewrites the work: afterwards the
pre-rebase sha is an ancestor of nothing and the tree's HEAD is on the base …
A postcondition written against the entry sha would refuse every
rebase-then-fast-forward, which is the normal path."* dybv fixed the code so
it stopped asking that question. What is new here is that a READER asked it,
by hand, and got the answer dybv predicted — a false strand.

The second half of the reading is simpler: the measurements were taken in a
tree at `82533ee5`, the commit immediately before the landing. At `82533ee5`
`init.go:126` is `if underDir(origin, a.Home)` and
`initoperatorfence_qa_test.go` has exactly six arms — which is, line number
for line number and count for count, what the bead reports. Every claim in it
is a true statement about `main`'s previous commit.

### The cost of believing it

A recovery would have cherry-picked `58dc4fa0` onto a `main` that already
holds its patch: a second copy of the ADR bullet, a second `underSameDir`
edit, two more arms 7 and 8. The bead routed this to the operator and the
launcher as the only parties who could act, so the false positive was aimed
at the one lane whose hand-applied fix nobody reviews.

### Asking it correctly

`git cherry -v <base> <sha-or-branch>` is the answer in one command — a `-` is
git saying the patch is already upstream, under whatever sha. `git log --grep
<bead-id> <base>` is the shop's own route (AGENTS.md, ADR 0022) and it found
`db608a65` immediately. `git merge-base --is-ancestor` is right only when its
subject is HEAD as it is now. A sha copied out of a retired worktree's log is
not a subject at all.

Census over the bead graph, 2026-11-08: two beads name a landing doubt in
their title, this one and `ranger-base-83nrw` ("merge-back blocked: … does not
land on main"), which is posse's own merge-blocked bead and a different, real
thing. So this is the first reader-side instance and it gets a fragment, not a
line in AGENTS.md.

No change to `main` was needed and none was made from this seat: no push, no
merge, no cherry-pick, no `gc`. `58dc4fa0` is left where it is — an
unreachable duplicate of a landed patch, which is what a rebase leaves behind
every time.
