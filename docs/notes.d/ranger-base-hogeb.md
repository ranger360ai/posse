# The replay landed before the block's seat was dispatched (ranger-base-hogeb)

`posse/dinesh-posse-ranger-base-ap4p6` was filed merge-blocked: main had
moved and replaying 72f5fa69 onto it conflicted. By the time this bead was
dispatched the replay was already on main — done by the seat that landed
ranger-base-c1pqo, which carried it as its first commit so main never
received the ap4p6 wall with the `remote -v prune` hole in it.

So this bead had no diff to write. What it owed instead is the one thing
posse asks for by name and cannot do itself.

## What posse says, and what it does not say

`posse worktrees`, 2026-11-06:

```
posse/dinesh-posse-ranger-base-ap4p6
  ... 1 commit(s) not on main by sha, replayed onto main as 72f5fa697a18
  as fc72606f9940 (same author, author date and subject), which is an
  identity match and not a measurement of what the replay kept — compare
  before retiring
```

That is `equivalentOnBase`'s third arm (`replayKey`, worktree.go): author
email, AUTHOR date, subject. A replay writes no `-x` trailer and its
resolution changes the patch, so the trailer and `git cherry` arms both
find nothing and this key is the only pairing left. MEASURED here: the two
keys are byte-identical, and exactly one commit in `main --not 72f5fa69`
carries it, so the pairing is unambiguous and the branch reads as landed.

The sentence is careful for a reason. A key made of three header fields
says a seat MEANT this commit to be that one; it says nothing about what
survived the conflict resolution. Nobody but the seat on this bead is
going to read the content, so here it is.

## The comparison, MEASURED 2026-11-06

Method: every line 72f5fa69 ADDED to each of its four files, checked for a
verbatim occurrence in main's copy of that file. Not a diff of the two
trees — main carries three later commits over the same two files
(ranger-base-c1pqo, ranger-base-e09l0, ADR 0079's `renderHookSkipArm`
parameter), so a tree diff is mostly their work and says nothing about
ap4p6's.

| file | added | absent from main |
|---|---|---|
| `docs/notes.d/ranger-base-ap4p6.md` | 119 | 0 (byte-identical) |
| `docs/notes.d/README.md` | 1 | 0 |
| `internal/posse/hookskip.go` | 144 | 15 |
| `internal/posse/hookskip_qa_test.go` | 87 | 4 |

All 19 absent lines are supersessions with a named successor on main, and
none of them is ap4p6's subject matter:

- the old call site `! posse_hs_readverb "$1" "${2:-}"` and the comment
  naming "the verb-plus-next-word read" → `"$@"` (ranger-base-c1pqo);
- `"remote": {"", "-v", "--verbose", "get-url", "show"}` and the comment
  above it → the table without the two options, plus the option skip
  (ranger-base-c1pqo — the whole point of landing the two together);
- `posse_hs_verb=$1; shift` → `posse_hs_ng=$((posse_hs_all - $#));
  posse_hs_verb=$1; shift` (ranger-base-e09l0's alias re-walk);
- both `-c|--config-env` writes → the same lines with
  `"moves or adds a hook, or defines an alias"` (ranger-base-e09l0);
- in the test, `renderHookSkipArm(real, dateBin, hooksRoot)` → the same
  call with ADR 0079's `hookSkipGlobalRef{}`, and the call-site assertion
  rewritten to `"$@"`.

Every line ap4p6 added that is ABOUT ap4p6 — `hookSkipReadVerbs`, the
deferred `-c` refusal, `posse_hs_cover`/`posse_hs_coverwhy`, the
`hookskip:config-override` reason — is on main verbatim.

## And by execution

`go test -run 'TestQAHookSkip|TestQAPIDDenyAlias' ./internal/posse`, at
main (fd2f7e24), 2026-11-06: PASS, 9.295s, claude and codex both.
`TestQAHookSkipRefusedOnEveryRuntime` runs ap4p6's four reads
(`status`, `rev-parse`, `ls-files`, `write-tree`, each under
`-c core.hooksPath=/dev/null`) as ALLOWED legs through a rendered shim, and
c1pqo's seven `remote` spellings as REFUSED ones. The bead's subject is
pinned on main, not merely present in it.

## The residue

The pin `refs/posse/merge-blocked/posse/dinesh-posse-ranger-base-ap4p6` and
that worktree are kept by this bead being open — `posse worktrees` says so
in as many words ("a handoff in flight keeps the tree it names"). A
launcher pass drops both once it closes. Nothing here was pushed and
nothing merged to main by hand.

Related: ranger-base-ap4p6, ranger-base-c1pqo, ranger-base-jvwcb,
ranger-base-e09l0, docs/notes.d/ranger-base-xea2y.md (the replay recipe),
docs/notes.d/ranger-base-1agm6.md.

ranger-base-e09l0 was merge-blocked over the same file when c1pqo closed,
and is not any more: 160d6bd9 paired onto main as 2f1d59aa (replayKey
identical), its branch is gone and its pin is under `refs/posse/retired/`.
So all three hook-skip commits — e09l0, ap4p6, c1pqo — are on main, and
`hookskip.go` has no merge-blocked work outstanding against it. What that
leaves is ranger-base-8t93w / ranger-base-onwi1: the posse the seats RUN is
still 1f4da269, which predates all three, so the landed wall is not live on
any seat until the operator runs `make install`. Being on main and being
installed are different facts here.
