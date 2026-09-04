package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The open-only promise belongs to OpenLabeledAny, not to the store it
// happens to be pointed at (ranger-base-bwrp8).
//
// MEASURED 2026-09-04 on bd 0.50.3, both store classes:
//
//	shop store (SQLite):  list --label-any qa        →   5 rows,   0 closed
//	                      list --all --label-any qa  → 396 rows, 391 closed
//	no-db JSONL store `bd init` writes TODAY:
//	                      list --label-any <l>       → the CLOSED bead comes back
//
// So `--label-any` filtered closed rows on this box for reasons that have
// nothing to do with the query, and every reader here — governance G3, the
// closed-dirty handoff's dedupe — would have mis-read on a fresh instance,
// silently and in the expensive direction: G3 holds a gate on questions
// somebody already answered, and the handoff adopts a bead somebody already
// finished and never files the next one.
//
// The fake's `fake-list-keep-closed` marker is the second store class
// (herdr_test.go), and every test in this file is that arm. Without the
// marker they would pass against the filter and against no filter at all,
// which is what made this defect invisible: NOTHING in the package read the
// difference, because the fake modelled only the store underneath us.
//
// KILLS, both mutants (measured 2026-09-04, `go test -overlay` so neither
// reached the tree; green unmutated in the same runs):
//
//   - the filter deleted → all three fail, and so does the LIVE pin against
//     real bd's own no-db store (TestLiveCIWatchFiresOnceAndClears,
//     ciwatch_live_test.go:229) — which is the arm that says "both store
//     classes" by measuring the second one rather than modelling it.
//   - the filter traded for `--status open` → the first fails on a-2, the
//     in_progress row. That mutant is only visible because the fake honours
//     `--status` (fakeBdFilterStatus, added with this): while it ignored the
//     flag it served the narrowing the right answer, which is the fake-blind-
//     to-a-filter shape.
func TestOpenLabeledAnyIsOpenOnBothStoreClasses(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rows := `[{"id":"a-1","status":"open","labels":["question"]},` +
		`{"id":"a-2","status":"in_progress","labels":["question"]},` +
		`{"id":"a-3","status":"closed","labels":["question"]}]`
	if err := os.WriteFile(filepath.Join(repo, "fake-list-labeled.json"), []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	// THE store class under test: this one hands `--label-any` its closed
	// rows and only `--status open` would drop them.
	if err := os.WriteFile(filepath.Join(repo, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bd := Bd{Bin: fakeBinFor(t, "bd")}

	ids := func(is []BdIssue, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, i := range is {
			out = append(out, i.ID)
		}
		return strings.Join(out, " ")
	}

	// a-2 is the second half of the assertion and not decoration: the fix
	// cannot be `--status open`, which answers with a-1 alone. bd's statuses
	// are open, in_progress, blocked, deferred and closed, and a question
	// somebody is holding is still a question nobody answered.
	if got := ids(bd.OpenLabeledAny(repo, "question")); got != "a-1 a-2" {
		t.Errorf("OpenLabeledAny on the keep-closed store = %q, want %q "+
			"(a-3 present: the open filter is the store's, not this function's; "+
			"a-2 missing: the filter narrowed to --status open)", got, "a-1 a-2")
	}
	// The other query keeps them, on this store class as on the other one —
	// the merge-back dedupe wants the closed verdict (ranger-base-j8qmj), and
	// a fix that filtered both would take it away.
	if got := ids(bd.AllLabeledAny(repo, "question")); got != "a-1 a-2 a-3" {
		t.Errorf("AllLabeledAny on the keep-closed store = %q, want all three", got)
	}
}

// G3, the first reader: a closed question is answered, and counting it holds
// a governance gate open over a decision the operator already made.
func TestGovG3ClosedQuestionIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		{"id": "bd-q", "status": "closed", "title": "answered long ago",
			"labels": []string{"question"}, "created_at": govNow.Add(-9 * time.Hour)},
	})
	if err := os.WriteFile(filepath.Join(dir, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if g := find(shopSet(t, govIn(t, b)), "G3"); g != nil {
		t.Errorf("a CLOSED question is answered, however the store lists it: %+v", *g)
	}
}

// The closed-dirty handoff, the second reader: its dedupe adopts the open
// handoff for this bead, and a closed one is a handoff somebody already did.
// Adopting it is the silent failure — the tree stays dirty and nothing is
// ever filed for it again.
func TestClosedDirtyDedupeIgnoresAClosedHandoff(t *testing.T) {
	t.Parallel()
	d, repo, _ := wtqaPassWithWork(t, func(repo, tree string) {
		commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
		write(t, filepath.Join(tree, "forgotten.txt"), "never committed\n")
		// A handoff for THIS bead, in this lane, with this dedupe key — and
		// closed, because a previous dirty tree was dealt with.
		write(t, filepath.Join(repo, "fake-list-labeled.json"), `[{"id":"m-8","title":`+
			`"`+closedDirtyTitle("a-1", 2, "posse/ranger-posse-a-1")+`",`+
			`"status":"closed","labels":["`+MergeBlockedLabel+`"]}]`)
		write(t, filepath.Join(repo, "fake-list-keep-closed"), "")
	})
	out := dispatcherOut(d)

	if strings.Contains(out, "m-8 already filed") {
		t.Errorf("a CLOSED handoff deduped the live one away — this tree is dirty and nobody is told:\n%s", out)
	}
	if filed := closedDirtyBeads(t, repo, "a-1"); len(filed) != 1 {
		t.Fatalf("a-1 got %d handoffs of its own, want 1:\n%s", len(filed), out)
	}
}
