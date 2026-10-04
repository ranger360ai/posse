//go:build posse_arm2

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fake bd has TWO closed-row filters and they are not the same filter.
// Nothing read the difference, so a fold — giving `list` the `ready` half —
// was green everywhere, and the fold's near neighbour (deleting one as a
// duplicate of the other) has already reached main twice:
//
//	3075168 + c3ab918  two seats, one name, redeclared        (ranger-base-pju9t)
//	5b4e686            the survivor deleted as "dead code"    (ranger-base-5im1q)
//
// Both of those were caught by the COMPILER, and only after they were on
// main. A fold compiles. This is the reader that makes it fail instead.
//
// MEASURED (ranger-base-m4730): with `list` pointed at fakeBdReadyDropClosed
// — one call site, one identifier, the change 5im1q's "strict superset"
// reading argues for — every test in mergeblocked_test.go, settleopen_test.go
// and closeddirty_test.go still passed (30 tests, ok 6.858s), and so did the
// whole package: internal/posse ok 668.961s, 0 --- FAIL, with this file not
// yet in the binary. Nothing read the difference. This test fails on it.
//
// The distinction, in one row: a-1's own status is NOT closed, but it is
// claimed and this repo's `show` answers closed for it.
//   - `bd list` without `--all` is the STORE's default filter: it reads the
//     row's own status field and nothing else, so a-1 stays.
//   - `bd ready` is about DISPATCH: a bead the fake handed out whose show now
//     answers closed is done work, so a-1 goes (ranger-base-y3x6n).
//
// Fold them and `list` inherits the dispatch half, which blunts exactly the
// open-vs-`--all` distinction ranger-base-j8qmj's merge-back dedupe turns on:
// its two queries (OpenLabeledAny, AllLabeledAny) want opposite things from a
// closed row, and against a fake that answers both the same they cannot be
// told apart. It is the dedupe CODE that turns on it — j8qmj's five dedupe
// PINS pass either way (5/5 PASS on the mutant overlay below), which is the
// whole reason this file exists.
//
// KILLS BOTH DIRECTIONS (measured 2026-09-04, ranger-base-ntuen, `go test
// -overlay` so no mutant reached the tree; green unmutated in the same pair
// of runs):
//   - `list`'s call site → fakeBdReadyDropClosed: InProgress and
//     OpenLabeledAny both got [], the two assertions aimed at it.
//   - `ready`'s call site → fakeBdDropClosed: Ready got [a-1 a-2], the
//     claimed-and-shown-closed row still offered as work (ranger-base-y3x6n's
//     defect, in the mirror direction).
//
// So neither function is the other's superset in the direction anyone has
// tried to fold it, and the fold now reds a test instead of only a comment.
//
// CORRECTION 2026-10-03 (ranger-base-bknod): the SECOND of those two
// directions is no longer this test's to kill. `ready` composes a third
// filter now — fakeBdReadyOpenOnly, the store's own query — and a claimed
// row carries `in_progress` in the state the overlay applies, so pointing
// `ready` at fakeBdDropClosed still drops a-1 and this test stays green.
// MEASURED: that mutant kills only TestQAReadyFakeCannotServeAClaimedRow
// below, whose last subtest reaches the one row the status filter cannot
// answer for — unclaimed with `keepAssignee`, so the state says `open`
// while the fake has still handed it out. The `list` direction above is
// unchanged and is still killed here.
func TestQAListAndReadyFakesAreNotOneFake(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// a-1: open in its own row, claimed, and shown closed — the discriminator.
	// a-2: closed in its own row — what BOTH filters drop, and what `--all`
	// is the only way to see.
	write("fake-list.json", `[{"id":"a-1","status":"in_progress","labels":["merge-back"]},{"id":"a-2","status":"closed","labels":["merge-back"]}]`)
	write("fake-list-labeled.json", `[{"id":"a-1","status":"in_progress","labels":["merge-back"]},{"id":"a-2","status":"closed","labels":["merge-back"]}]`)
	write("fake-ready.json", `[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`)
	write("fake-show.json", `[{"id":"a-1","status":"closed"}]`)

	bd := Bd{Bin: fakeBinFor(t, "bd")}
	ids := func(is []BdIssue, err error) []string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, i := range is {
			out = append(out, i.ID)
		}
		return out
	}
	eq := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// Before the claim neither filter has anything to disagree about: a-1 is
	// not handed out, so `ready` keeps it too. Without this arm the test
	// would pass against a fake that dropped on fake-show.json alone.
	if got := ids(bd.Ready(repo, "")); !eq(got, []string{"a-1", "a-2"}) {
		t.Fatalf("an unclaimed bead is ready work whatever show says, got %v", got)
	}
	if _, err := bd.Claim(repo, "a-1", "ranger"); err != nil {
		t.Fatal(err)
	}

	// `ready`: the claimed-and-shown-closed row leaves the queue.
	if got := ids(bd.Ready(repo, "")); !eq(got, []string{"a-2"}) {
		t.Errorf("ready must drop a claimed bead its show answers closed, got %v", got)
	}
	// `list` without `--all`: the SAME row stays. This is the assertion the
	// fold breaks — it is the only one in the suite that does.
	if got := ids(bd.InProgress(repo)); !eq(got, []string{"a-1"}) {
		t.Errorf("list without --all filters on the row's own status and nothing else — "+
			"a-1 must stay and a-2 must go, got %v "+
			"(if a-1 is missing, `list` has been given `ready`'s dispatch half)", got)
	}
	if got := ids(bd.OpenLabeledAny(repo, "merge-back")); !eq(got, []string{"a-1"}) {
		t.Errorf("the labelled open query is the same default filter, got %v", got)
	}
	// And `--all` is the override, so the closed row is reachable exactly one
	// way. A fake that answered these two the same would make the merge-back
	// dedupe's two queries indistinguishable.
	if got := ids(bd.AllLabeledAny(repo, "merge-back")); !eq(got, []string{"a-1", "a-2"}) {
		t.Errorf("--all overrides the default filter and must show the closed row, got %v", got)
	}
}

// `bd ready` is OPEN ROWS AND NOTHING ELSE, and for the life of every test
// this fake served fake-ready.json whole — in_progress rows included — so a
// fixture could put a claimed bead in the ready queue, and 49 test files
// did. Every in_progress branch of the fire loop (the holder join of ADR
// 0004 §2, ADR 0008's crew shield, ADR 0030's orphaned-claim tiebreak,
// rangerhq-zom's settled skip, `--resume`'s override) was green against a
// queue real bd cannot produce, and ranger-base-eh1kr is what that cost: two
// stranded claims (ranger-base-4mrmc, ranger-base-mz8ud) invisible to every
// pass while the pins stayed green.
//
// MEASURED 2026-10-03, bd 0.50.3, both store classes, one binary, the same
// argv (`ready --json --limit 0`): the shop's SQLite queue answered with its
// 10 open rows and not the in_progress bead of the moment, and a
// hand-written `no-db: true` JSONL store holding one open and one
// in_progress bead answered with the open one. bd's own help: "Excludes
// in_progress, blocked, deferred, and hooked issues."
//
// THREE filters now, composed in the `ready` case, and this grades each of
// them plus the ORDER:
//
//	fakeBdApplyState      the claim this pass made, overlaid on the file
//	fakeBdReadyDropClosed dispatch's half: handed out, and `show` says closed
//	fakeBdReadyOpenOnly   the store's own query: the row's status is open
//
// The last subtest is the fold guard in the new direction. fakeBdReadyOpenOnly
// looks like a strict superset of its neighbour — a closed row is not an open
// row — and it is not: Bd.Unclaim writes `--status open` and, with
// keepAssignee, leaves the assignee standing, so the state this filter reads
// says `open` for a row the fake has handed out and whose `show` answers
// closed. Delete fakeBdReadyDropClosed as redundant and that row is ready
// work again, which is ranger-base-y3x6n's defect returning through the
// fixture.
func TestQAReadyFakeCannotServeAClaimedRow(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	ids := func(is []BdIssue, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, i := range is {
			out = append(out, i.ID)
		}
		return strings.Join(out, ",")
	}

	// One row per status bd's help names, plus the suite's own convention:
	// a row with NO status field is open. Real bd always renders the field;
	// the fixtures omit it to mean open, and a filter that read the omission
	// as "some other status" would empty every ready queue in the package.
	const rows = `[{"id":"a-open","status":"open"},{"id":"a-bare"},` +
		`{"id":"a-held","status":"in_progress","assignee":"ranger"},` +
		`{"id":"a-blocked","status":"blocked"},{"id":"a-deferred","status":"deferred"},` +
		`{"id":"a-closed","status":"closed"}]`
	write("fake-ready.json", rows)
	write("fake-list.json", rows)
	write("fake-show.json", rows)

	if got := ids(bd.Ready(repo, "")); got != "a-open,a-bare" {
		t.Errorf("`ready` must serve open rows and nothing else, got %v "+
			"(an in_progress, blocked or deferred row in a ready queue is a store no bd can be)", got)
	}
	// The claimed half is STILL answered — by the other query, which is the
	// whole point: the rows did not become unreachable, they moved to the
	// door ranger-base-eh1kr's scan reads (interrupted.go).
	if got := ids(bd.InProgress(repo)); got != "a-held" {
		t.Errorf("`list --status in_progress` must still answer with the claimed row, got %v", got)
	}

	// A claim moves a bead between the two queries, which is what lets a
	// two-pass fixture exist without a store saying one bead is both
	// ready-open and in_progress-claimed. Both halves are the fake's writes:
	// the state overlay (read by the status filter) and fakeBdRecordStatus.
	if _, err := bd.Claim(repo, "a-open", "ranger"); err != nil {
		t.Fatal(err)
	}
	if got := ids(bd.Ready(repo, "")); got != "a-bare" {
		t.Errorf("a bead this pass claimed must leave `ready`, got %v "+
			"(if a-open is still there, the state overlay runs AFTER the status filter)", got)
	}
	// a-open keeps its place in the file: it is already a row of this store
	// and the claim UPDATES it rather than appending a second copy.
	if got := ids(bd.InProgress(repo)); got != "a-open,a-held" {
		t.Errorf("a bead this pass claimed must appear in the claimed listing, got %v", got)
	}
	// ...and handing it back puts it where it started. Without this arm
	// every drop above is satisfied by a filter that latches.
	if err := bd.Unclaim(repo, "a-open", "ranger", false); err != nil {
		t.Fatal(err)
	}
	if got := ids(bd.Ready(repo, "")); got != "a-open,a-bare" {
		t.Errorf("an unclaimed bead is ready work again, got %v", got)
	}
	if got := ids(bd.InProgress(repo)); got != "a-held" {
		t.Errorf("an unclaimed bead leaves the claimed listing, got %v", got)
	}

	// The fold guard. a-bare is claimed and its `show` now answers closed:
	// the dispatch half drops it. Then it is UNCLAIMED with the assignee
	// kept — Bd.Unclaim's `keepAssignee`, the resumed-bead handback — which
	// makes its status `open` again, so the status filter has nothing to say
	// about it and only the dispatch half is left holding the line.
	if _, err := bd.Claim(repo, "a-bare", "ranger"); err != nil {
		t.Fatal(err)
	}
	write("fake-show.json", `[{"id":"a-bare","status":"closed"},{"id":"a-open","status":"open"}]`)
	if got := ids(bd.Ready(repo, "")); got != "a-open" {
		t.Errorf("a claimed bead its show answers closed must leave `ready`, got %v", got)
	}
	if err := bd.Unclaim(repo, "a-bare", "ranger", true); err != nil {
		t.Fatal(err)
	}
	if got := ids(bd.Ready(repo, "")); got != "a-open" {
		t.Errorf("got %v — a-bare is `open` in the state and still handed out with its show "+
			"answering closed, so the STATUS filter cannot drop it and fakeBdReadyDropClosed "+
			"is the only thing that can. If a-bare is back, that filter has been deleted as "+
			"a redundant superset (ranger-base-y3x6n's defect, through the fixture)", got)
	}
}

// The fake's writes stay INSIDE the fixture repo, and the one that can
// create a file rather than rewrite one is the reason this exists.
//
// `bd` is per-repo, so the fake keeps a store's rows in its cwd — and bd's
// cwd is not always a fixture repo. A test whose config carries no `beads:`
// falls back to the PROCESS cwd (App.BeadsDirs says so out loud: "no
// `beads:` in … — using the process cwd … as the only beads source"), which
// for a test binary is the package directory. fakeBdMarkClosed and
// fakeBdSetDescription are safe there by construction — they rewrite rows
// a file already holds and create nothing — and fakeBdRecordStatus is the
// first of the family that ADDS one. Unguarded it wrote
// `internal/posse/fake-list.json` into the source tree, where the next run
// with the same fallback would have read it as a store and where `git
// status` found it (ranger-base-bknod).
//
// Graded directly rather than by sweeping the package directory for stray
// files: a sweep of a shared tree reds on whatever another test wrote a
// moment ago, in whichever order the package happens to run.
func TestQAFakeBdWritesNoStoreOutsideAFixtureRepo(t *testing.T) {
	t.Parallel()
	bd := Bd{Bin: fakeBinFor(t, "bd")}

	// A directory that is NOT a fixture store: no fake-ready.json, which is
	// the one file every repo in the suite declares.
	bare := t.TempDir()
	if _, err := bd.run(bare, "--actor", "ranger", "update", "a-1", "--claim", "--json"); err != nil {
		t.Fatalf("the fake refused the call itself, so nothing is being graded: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bare, "fake-list.json")); err == nil {
		b, _ := os.ReadFile(filepath.Join(bare, "fake-list.json"))
		t.Errorf("a claim wrote a store into a directory no fixture declared one in: %s", b)
	}

	// The control, and it is what makes the absence evidence: the same call
	// in a directory that IS a fixture store records the claim.
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(`[{"id":"a-1","title":"t"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := bd.run(repo, "--actor", "ranger", "update", "a-1", "--claim", "--json"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(repo, "fake-list.json"))
	if err != nil {
		t.Fatalf("the claim recorded nothing in a real fixture repo either — the guard is refusing everything: %v", err)
	}
	if !strings.Contains(string(b), `"in_progress"`) {
		t.Errorf("the claimed listing does not carry the claim: %s", b)
	}
}
