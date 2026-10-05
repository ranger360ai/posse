# ranger-base-2xez0 — a merge-back whose stalest file was a number in prose

Landed 2026-10-05, git 2.50.1 / macOS 26.4.1. The replay mechanics are
`docs/notes.d/ranger-base-xea2y.md` and nothing here changes them; where the
replay bead's own id goes is `docs/notes.d/ranger-base-o6ka5.md` and nothing
here changes that either.

`ranger-base-dckhf` closed with one commit, `6d64ed75`, on
`posse/dinesh-posse-ranger-base-dckhf`. `main` moved to `de2ef463` under it —
`ranger-base-r5546`, `-o6ka5`, `-h9925`, three commits — the launcher's rebase
conflicted on `scripts/readings-census.py` and aborted, and this bead was
filed. Replayed with `git merge-tree --write-tree` against the merge base
`4384ac7f`: no rebase, no cherry-pick, no sequencer state, nothing to leak.
Two things about THIS conflict are worth keeping.

## Two beads extended the same census loop, and the union is the resolution

Three conflicts, one file, and all three are the same event read three times.
`ranger-base-r5546` added a truncation count to `census()`; `ranger-base-dckhf`
added the false-IDLE count and the gate block's regions to the same loop, at
the same two statements, and both rewrote the `replayable` comment above the
same `if`. The third is `export_corpus`'s docstring, where r5546 widened the
"A REDACTED CASE IS EXPORTED AND MARKED" paragraph into "A LOSSY CASE" and
dckhf left that paragraph alone and appended one about the gate block.

Nothing here is a disagreement. The counters are independent — `truncated` is
a quality field, `d5`/`gated`/`candidates` are a rate — so `census()` keeps
both, and `replayable` reads

```python
if regs and not cut and not rec.get("redacted"):
```

which is r5546's condition with dckhf's `regs`. The thing worth saying once
is that **the two beads' reasons were the same reason**: r5546 keyed `cut` on
`regs`, dckhf keyed `replayable` on `herdr` alone, and both because a quality
claim is about THIS record's own verdict. The merged comment says it once and
names `cut` in the same breath, rather than carrying two paragraphs that
agree.

`internal/posse/herdr_test.go` was the one Go file both sides really edited —
`+36/-1` from the branch, `+32` from `main`, adjacent regions of one fixture
file — and it auto-merged. Checked per xea2y, which cost one command:

```
diff <(git diff --no-ext-diff 6d64ed75 -- internal/posse/herdr_test.go) \
     <(git diff --no-ext-diff 4384ac7f main -- internal/posse/herdr_test.go)
```

Three lines of output — the blob hashes and two `@@` offsets. That is "the
merged file is the branch's version plus exactly `main`'s hunks", said by a
tool rather than by a compile.

## A recorded output is a derived artifact of the OTHER side's code

`docs/notes.d/ranger-base-dpvlh.md` names the class "a merge-back whose branch
adds a gate that derives its own membership" and the rule "re-run the derived
gate; a clean merge is not an answer about it". This pass is the same rule one
step weaker, and the weakness is the whole finding.

`ranger-base-dckhf.md` §7 records a census run over a four-record fixture,
verbatim, as the bead's own measure-before-closing arm. Its first line is
`print_table`'s header — and `main` grew a column in that exact line while
the branch sat blocked:

```
readings: 4 in 1 log(s) · 1 replayable · 0 redacted          written 2026-10-04
readings: 4 in 1 log(s) · 1 replayable · 0 truncated · 0 redacted
```

**No gate reads it.** The fragment is prose in a file git merged without a
conflict, because only one side wrote the file at all; `make notes-check`
checks the index, not a code block inside a fragment; `make doc-check`'s prose
pins do not know this line is an output. A derived GATE reds and sends someone
to look. A derived NUMBER in prose stays wrong, under a heading that says
MEASURED, until a reader trusts it.

So: re-run, never patch the number in. The re-run has a problem the gate case
does not — the fixture was never committed, so there is nothing to re-run
over. What makes a rebuilt one admissible is the check that comes free with
it: a fixture rebuilt from §7's own specification (three D5 and one D3; the
three D5 cases being a seen-idle gate, a gate herdr did not see, and no gate
block at all) reproduced **every other figure in the block identically** —
the per-day row, both per-decision rows, `1 of 3 D5 record(s) (2 carrying a
settle-gate reading)`, and the regions table `2 pane_capture / 1 footer / 1
pane_capture_at_prompt` — and added `0 truncated`. A wrong reconstruction
would have missed one of those. That the figures agree is what makes the
rebuilt fixture the same fixture, and it is the same argument xea2y's `diff`
of two diffs makes about a file.

`0` is also derivable, which is the cheap half of the answer: `truncated` is
set from `RegionBytes > len(RegionPreview)` in `AppendReading` and is
`omitempty`, so a hand-built fixture written before the field was censused
carries it nowhere. `replayable` is therefore still 1 and §6's rule still
reads the way it did.

**The correction went in the REPLAY commit, not this bead's.** It is a
resolution of a conflict git could not see, the same kind as the regenerated
notes index, and the file is one only the branch brings — so the merged tree
is self-consistent at every commit. It costs the pairing nothing:
`replayKey` is `%ae`/`%aI`/`%s` (`internal/posse/worktree.go`) and reads no
content, and the replay commit's body names this bead, so
`git log --grep ranger-base-2xez0` finds the edit.

The class is "a notes fragment that records the output of a program the other
side of the merge changed", and on this box that is any `--check`, census or
table output pasted into a fragment — which is most of the MEASURED blocks.
The check is one grep of the branch's fragments for the commands `main`
touched.

## Verified

On `de2ef463`, the base this landed on. `make fmt-check`; `go build ./...`;
`go vet ./internal/posse` in all three arm tags (untagged, `posse_arm2`,
`posse_arm3`); `python3 -m py_compile scripts/readings-census.py`;
`scripts/notes-index.py --check` exit 0; `make notes-check` and
`make doc-check` exit 0 — the latter is the door over
`TestQAEveryNotesFragmentCitationResolves`, which is what reads this
fragment's own citations.

`make test` whole, **exit 0, no FAIL** — arm 1 `cmd/posse` 173.3s,
`cmd/testparallel` 1.5s, `internal/posse` 305.7s, `internal/treepins` 339.8s;
arm 2 `internal/posse` 244.3s; arm 3 `internal/posse` 271.0s; silent-revert
audit 2017 commits, 0 untriaged. `go run ./cmd/checkorphans` clean after it.
Arm 2 is the one that matters most here — `internal/posse/falseidle_qa_test.go`
holds this bead's five pins, including the pair that drive a whole pass, and
they are green over the merged `readings-census.py` and the merged
`herdr_test.go` fixture lever.

`make tree-check` was run TWICE, because the first run was `make test`'s own
prerequisite and this fragment did not exist yet: fourteen doors green, 62s,
at a one-minute load average of 19.5 falling to 13.8, both suite slots free
(the load is other work on the box, not a suite — `scripts/suite-lock.sh
--status` was read for that).

`main` was `de2ef463` before the replay and `de2ef463` after the suite, and
`git merge-base --is-ancestor main HEAD` holds, so the branch fast-forwards
and nothing had to be redone for `ranger-base-o6ka5`'s reason.
