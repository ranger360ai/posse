# ranger-base-w6kke: the watcher's words on the one line the wall check carries

`scripts/posse-wall-check` reports a failed member by its first `file.go:NN:`
line and nothing after it — evidence(), 300 characters, ADR 0097 D3 — and the
launcher puts that one line on the bead. So a member's first failure line is
its whole budget, and a line that names a symptom and leaves the reason in a
captured buffer reaches the bead empty. That is ranger-base-a688i's class;
this bead was the instance a688i's closing section named and could not take.

## The prescribed fix landed under another bead

ranger-base-jqdm2 needed the same plumbing one line earlier — routing a
silent outside kill (exit 137, empty pane) needs the pane at the READ — so
`launch`/`launchOn` returning the `*wwSaid`, `wwWaitKilled` taking it, and all
five call sites (three in `wallwatchlive_test.go`, two in
`wallwatchtty_linux_test.go`) came with it. Exactly the five this bead counted.
Said here rather than re-done: jian-yang's comment on the bead has the detail.

## What changed here

`wallwatchbackstopwiring_linux_test.go` only — the residual, in
`TestLiveWallWatchKillsASeatWhoseBackstopRecordCannotBeWritten`, which is a
member and runs its seat through `runWallWatch` rather than through
`launchOn`, so the helper fix could not reach it.

- Its captured pane is a `wwSaid` and no longer a `strings.Builder`. This is
  a688i's race, not only its diagnostic: the 10s deadline branch reads the
  buffer with the watcher's goroutine still writing, and the Builder was
  shared across both readers. `-race` over the member is green.
- The timeout `t.Fatalf` and the `did not say why` `t.Errorf` carry
  `wwOneLine(said.String())` on their first line instead of `:\n%s`.
- The third failure line — `the seat's command RAN although its backstop
  record was never written` — keeps its line 2 and now carries a comment
  saying why: its first line is already the whole claim, which is the case
  a688i's class explicitly carves out. A reader should not have to wonder
  whether it was missed.

## MEASURED 2026-11-22 (linux 7.2.15, bwrap 0.13.1, 16 cores)

Three mutants, planted one at a time by
`internal/posse/wallwatchbackstopwiring_linux_test.go`'s own values, md5 taken
on both sides and equal after each revert. Each red on its FIRST line, with
the cause in it:

- `want 1` → `want 7`: `watcher exit = 1, want 1 (it killed the seat): "posse
  wall-watch: cannot write this seat's record for the dispatch backstop (mkdir
  …/wall-watch: not a directory) — killing the seat rather than running it
  where only this process can see it (ranger-base-0qpsw D3) …"`. Over 300
  characters, so evidence() does drop the tail; what survives inside the
  budget is the cause, at the front.
- the 10s deadline → `0 * time.Second`: `the watcher has not returned 10s
  after a record write that cannot succeed; watcher and bwrap said NOTHING —
  so no refusal of its own: bwrap ended before it reported a child, and
  without a word`. The absence reads as a finding, which is wwOneLine's
  contract and the one cause that leaves no words.
- the `why` constant → a sentence the watcher never writes: the Errorf names
  what it wanted AND what the pane said.

Green, all on the same box and date: the member alone under `-race` (0.06s);
`-run 'TestQAWallWatch|TestLiveWallWatch|TestWallWatch|TestQAWallSeatAbstain'`
(1.9s); `make fmt-check`; `go vet ./internal/posse`; `make tree-check` (123s);
`make test`, all three arms (arm 2 64.2s, arm 3 122.9s), and
`go run ./cmd/checkorphans` clean after it. And the route this bead is about,
`bash docs/adr/0097-wall-venue.probe.sh venue+check`: rc=0, `wall check: green,
50 pins measured`, 119s wall at a one-minute load of 8.67 — so the member still
MEASURES inside ADR 0097 D1's venue rather than skipping there, which is what
makes its one line a line that reaches a bead at all.

## The rest of the class, in the member set

Method, reproducible without a script: every `func Test(QA)?Live(Bwrap|Wall)…`
in `internal/posse` — the wall check's own member pattern,
`scripts/posse-wall-check:204` — read for a line carrying `:\n%s`. 31 hits: 2
are `t.Logf` and 1 is a comment, so 28 are failure messages. 22 are the
carve-out (first line is the claim, line 2 supports it: `the bead does not say
%q:`, `the durable verdict lacks %q:`, and the one named above). 2 are folded
here. The remaining 4 — one in `wallclaudeprojects_qa_test.go`, three in
`wallunixlive_test.go` — are filed as ranger-base-8ck6d rather than folded,
because the wallunixlive three carry a multi-line seat SELF-REPORT and not a
watcher's two lines: which row of a report to spend 300 characters on is a
choice, and the choice is that bead's deliverable.

Nothing pins either converted call site. a688i's two fatals got
`TestQAWallWatchLaunchFatalsCarryTheFoldedWordsAtTheCallSite` from
ranger-base-nqw8v, a verify bead, and that pin reads `launchOn`'s wait loop by
name; the two lines here are in a member's own body and are outside it. The
mechanism to extend is `callsite_qa_test.go`'s `qcsFuncDecl`.
