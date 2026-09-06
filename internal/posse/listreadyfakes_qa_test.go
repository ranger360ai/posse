//go:build posse_arm2

package posse

import (
	"os"
	"path/filepath"
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
