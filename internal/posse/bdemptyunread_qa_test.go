//go:build posse_arm2

package posse

// QA pins for ranger-base-i00xh / ADR 0071 — an empty `--json` answer is not
// an empty queue until bd's own count says so.
//
// THE DEFECT, MEASURED 2026-10-09 on the pinned bd 0.50.3 (the eight-fixture
// table is docs/notes.d/ranger-base-a5st4.md): a store whose `beads.db` holds
// ZERO issues over a `.beads/issues.jsonl` that holds N answers `list --json`,
// `ready --json` and `blocked --json` with `[]`, exit 0, **stderr empty**. So
// every reader posse ships printed "no ready work" over a queue it could not
// read, and no stream the pass can see said otherwise. Two ways into the
// state and neither is exotic: a read over an uncommitted jsonl builds the
// FILE and not the GRAPH, and a read-only auto-import over an existing
// zero-row database dies at its first config write.
//
// Every arm here is offline, through the package's fake bd, which gained an
// `info` case for this (herdr_test.go). The LIVE arm — the five-step rig
// a5st4 records, under `RHQ_LIVE_BD=1` — is ranger-base-dkrwi's, and the
// repair verb is pinned by nobody: `bd import` is denied at every seat that
// would run the suite, which is why the error's command text is pinned
// HERE, as text (TestQABdStoreUnreadErrorNamesTheOperatorsRepair below).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unreadRepo is the zero-row store as posse meets it: a repo whose `ready`
// and `list` answer `[]` with exit 0, a census carrying one record, and a bd
// that says how many issues it holds. count is the `issue_count` the fake's
// `info` serves; a "" count leaves fake-info.json absent, which is the
// fake's default of 1 — a genuinely empty queue.
func unreadRepo(t *testing.T, count, census string) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake-ready.json"), []byte("[]"), 0o644)
	if census != "" {
		if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte(census), 0o644)
	}
	if count != "" {
		os.WriteFile(filepath.Join(dir, "fake-info.json"), []byte(count), 0o644)
	}
	return dir
}

// oneRecord is a census line: one bead, the shape `bd sync --flush-only`
// writes. The check asks "is there anything at all", never how many, so one
// is the whole fixture.
const oneRecord = `{"id":"a-1","title":"one","status":"open"}` + "\n"

// bdCallsAsking is bd-calls.log with the vacuity closed: the shared bdCalls
// helper swallows a missing file, so an arm asserting that `info` was NOT
// called would pass over a fake that was never run at all. want is a verb
// the log must hold for the absence of another to mean anything.
func bdCallsAsking(t *testing.T, fake, want string) string {
	t.Helper()
	got := bdCalls(t, fake)
	if !strings.Contains(got, want) {
		t.Fatalf("the fake bd was never asked %q — bd-calls.log reads: %q", want, got)
	}
	return got
}

// The pin the bead was filed for: bd holding zero issues over a census that
// carries rows is the store being unreadable, and Ready says so instead of
// serving the empty list it was handed.
func TestQAReadyRefusesAZeroRowStoreAsAnEmptyQueue(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := unreadRepo(t, `{"issue_count": 0}`, oneRecord)
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	issues, err := bd.Ready(dir, "")
	if err == nil {
		t.Fatalf("a zero-row store over a census with rows must not read as empty, got %v issues", len(issues))
	}
	if issues != nil {
		t.Errorf("a refused read serves no rows, got %+v", issues)
	}
	var se *BdStoreUnreadError
	if !errors.As(err, &se) {
		t.Fatalf("want a typed BdStoreUnreadError, got %T: %v", err, err)
	}
	if !IsBdStoreUnread(err) {
		t.Error("IsBdStoreUnread must agree with errors.As")
	}
	// The store, not the repo the read was made from: on this instance the
	// store of record is reached through a `.beads/redirect`, and the dir a
	// caller passes is not the dir bd reads.
	store := filepath.Join(dir, ".beads")
	if se.Store != store || se.Dir != dir {
		t.Errorf("want Store=%s Dir=%s, got Store=%s Dir=%s", store, dir, se.Store, se.Dir)
	}
	if !strings.Contains(err.Error(), store) {
		t.Errorf("the error must name the store bd is actually reading: %v", err)
	}
	// And it asked bd, rather than deciding from the census alone.
	if !strings.Contains(bdCallsAsking(t, fake, "ready"), "info --json") {
		t.Errorf("the check must ask bd its own count: %s", bdCallsAsking(t, fake, "ready"))
	}
}

// The error carries the operator's one repair, verbatim, and names the repo
// to type it in. This is the arm that holds the sentence: the verb is in
// every persona PID's deny set, so no suite on this box can run it.
func TestQABdStoreUnreadErrorNamesTheOperatorsRepair(t *testing.T) {
	t.Parallel()
	e := &BdStoreUnreadError{Dir: "/x/work", Store: "/x/store/.beads"}
	got := e.Error()
	for _, want := range []string{
		"bd import -i .beads/issues.jsonl", // the repair, verbatim
		"/x/store",                         // the repo that owns the store
		"/x/store/.beads",                  // the store itself
		"/x/store/.beads/issues.jsonl",     // the census it was weighed against
		"0 issues",
		"ADR 0071",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the error must carry %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "\n") != 0 {
		t.Errorf("one line, so a per-repo scan line stays one line:\n%s", got)
	}
}

// bd holding issues means the list was empty on its own terms. This is the
// arm that keeps the check from firing on every quiet queue in the shop.
func TestQAReadyServesAnEmptyQueueWhenBdHoldsIssues(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := unreadRepo(t, `{"issue_count": 1}`, oneRecord)
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	issues, err := bd.Ready(dir, "")
	if err != nil {
		t.Fatalf("a store that holds issues and lists none is empty, not unreadable: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("want the empty list served, got %+v", issues)
	}
	if !strings.Contains(bdCallsAsking(t, fake, "ready"), "info --json") {
		t.Errorf("the empty answer is still a question: %s", bdCallsAsking(t, fake, "ready"))
	}
}

// No census, no contradiction: a brand-new store before its first bead, or
// a repo that is not a queue at all. The census is the CHEAPER read and it
// comes first, so such a repo costs no extra fork — which is the half of
// D2's ordering a green return alone cannot show.
func TestQANoCensusMakesAnEmptyAnswerHonestAndAsksBdNothing(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := unreadRepo(t, `{"issue_count": 0}`, "")
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	issues, err := bd.Ready(dir, "")
	if err != nil {
		t.Fatalf("a repo with no census has no census to contradict bd with: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("want the empty list served, got %+v", issues)
	}
	if calls := bdCallsAsking(t, fake, "ready"); strings.Contains(calls, "info") {
		t.Errorf("a repo with no issues.jsonl must never reach `info`: %s", calls)
	}
}

// A census file that exists but holds no record — a touched file, a store
// flushed before its first bead — is not a census carrying rows.
//
// ADR 0071 D2a is "absent, or no line beginning `{`", and the two halves
// fail differently, so each needs a row (ranger-base-wjqeu, verifying this
// bead's close). With only the blank-line row here, censusHasRecord's
// `t[0] == '{'` was unpinned: MEASURED 2026-10-09, dropping it for a bare
// `len(t) > 0` — any non-empty line is a record — left all twelve arms of
// this file GREEN, while a census holding a line that is not a record would
// then be weighed against bd's count and a HEALTHY empty read refused. The
// rows below are the two ways that file comes about on this box.
func TestQAAnEmptyCensusFileIsNotACensusWithRows(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	for _, c := range []struct {
		why, census string
	}{
		{"blank lines — a touched file, or a store flushed before its first bead", "\n\n"},
		{"a line that is not a record — a truncated write, or bd's own error text landing in the file", "Error: no beads database found\n"},
		{"an empty `--json` answer redirected into the census, which is the very shape this record is about", "[\n]\n"},
	} {
		t.Run(c.census, func(t *testing.T) {
			dir := unreadRepo(t, `{"issue_count": 0}`, c.census)
			os.Remove(filepath.Join(fake, "bd-calls.log"))

			issues, err := bd.Ready(dir, "")
			if err != nil {
				t.Fatalf("a census with no record is no census (%s): %v", c.why, err)
			}
			if len(issues) != 0 {
				t.Errorf("want the empty list served, got %+v", issues)
			}
			if calls := bdCallsAsking(t, fake, "ready"); strings.Contains(calls, "info") {
				t.Errorf("a census holding no `{` line must never reach `info` (%s): %s", c.why, calls)
			}
		})
	}
}

// A census record longer than the reader's buffer is still a record. Beads
// carry descriptions of no bounded length — this bead's own is over 2 KB —
// so a reader that gave up on a long first line would read the live census
// as empty and the check would never fire on the one store it exists for.
func TestQAACensusRecordLongerThanTheBufferIsStillARecord(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	long := `{"id":"a-1","description":"` + strings.Repeat("x", 100_000) + `"}` + "\n"
	dir := unreadRepo(t, `{"issue_count": 0}`, long)

	if _, err := bd.Ready(dir, ""); !IsBdStoreUnread(err) {
		t.Errorf("a 100 KB first record must read as a record, got: %v", err)
	}
}

// A list that found something is returned exactly as it was before any of
// this existed: no second call, and no cost on the reads that are the
// overwhelming majority.
func TestQAANonEmptyListAsksBdNothing(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake-ready.json"), []byte(`[{"id":"a-1","title":"one"}]`), 0o644)
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte(oneRecord), 0o644)
	os.WriteFile(filepath.Join(dir, "fake-info.json"), []byte(`{"issue_count": 0}`), 0o644)
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	issues, err := bd.Ready(dir, "")
	if err != nil {
		t.Fatalf("a non-empty list is an answer: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "a-1" {
		t.Errorf("want the repo's own ready list, got %+v", issues)
	}
	if calls := bdCallsAsking(t, fake, "ready"); strings.Contains(calls, "info") {
		t.Errorf("a non-empty answer must ask no question: %s", calls)
	}
}

// An `info` that cannot answer is the store unreadable by a SECOND route,
// and it is returned as THAT error. The whole of ADR 0071 is that an
// unanswerable question does not read as "empty".
func TestQAAFailedInfoIsAnErrorAndNeverAnEmptyQueue(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	// bd's --json failure shape: the payload on stdout, stderr empty, exit
	// 1 (rangerhq-aas).
	dir := unreadRepo(t, `{"error": "x"}`, oneRecord)
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	issues, err := bd.Ready(dir, "")
	if err == nil {
		t.Fatalf("a store that cannot answer its own count is not a store holding zero, got %v issues", len(issues))
	}
	if issues != nil {
		t.Errorf("a refused read serves no rows, got %+v", issues)
	}
	if !strings.Contains(err.Error(), "x") {
		t.Errorf("want bd's own words carried, got: %v", err)
	}
	if IsBdStoreUnread(err) {
		t.Error("an unanswerable count is not the zero-row verdict — the two name different repairs")
	}
}

// `issue_count` MISSING is not `issue_count: 0`. The field is read through a
// pointer for exactly this: a payload bd answered without it would otherwise
// unmarshal to zero and be announced as the defect, with a repair that is
// not the repair.
func TestQAAnInfoWithoutIssueCountIsAnErrorNotZero(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := unreadRepo(t, `{"mode": "direct"}`, oneRecord)
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	_, err := bd.Ready(dir, "")
	if err == nil {
		t.Fatal("a payload with no issue_count cannot be read as a count of zero")
	}
	if !strings.Contains(err.Error(), "issue_count") {
		t.Errorf("the error must name the field it could not read: %v", err)
	}
	if IsBdStoreUnread(err) {
		t.Error("a missing field is not the zero-row verdict")
	}
}

// D1: all five list readers go through the one door. A sixth reader added
// past it would put the silent shape straight back, and nothing else in the
// suite would notice — each of these returns an empty list on its own
// fixture-free default, so this arm is the whole of the guard.
func TestQAEveryListReaderRefusesAZeroRowStore(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := unreadRepo(t, `{"issue_count": 0}`, oneRecord)

	for _, c := range []struct {
		name string
		read func() ([]BdIssue, error)
	}{
		{"Ready", func() ([]BdIssue, error) { return bd.Ready(dir, "") }},
		{"ListAll", func() ([]BdIssue, error) { return bd.ListAll(dir) }},
		{"InProgress", func() ([]BdIssue, error) { return bd.InProgress(dir) }},
		{"OpenLabeledAny", func() ([]BdIssue, error) { return bd.OpenLabeledAny(dir, "question") }},
		{"AllLabeledAny", func() ([]BdIssue, error) { return bd.AllLabeledAny(dir, "merge-blocked") }},
	} {
		issues, err := c.read()
		if !IsBdStoreUnread(err) {
			t.Errorf("%s must refuse the zero-row store, got (%+v, %v)", c.name, issues, err)
		}
	}
}

// The caller side, unchanged by design (D5) and pinned anyway: the error
// ReadyAll already knew how to carry reaches `failed` as a ScanError naming
// the repo.
func TestQAReadyAllReportsAZeroRowStoreAsAFailedScan(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}

	good := scanRepo(t, `[{"id":"a-1","title":"one"}]`)
	unread := unreadRepo(t, `{"issue_count": 0}`, oneRecord)
	scanConfig(t, b.App, good, unread)

	issues, failed := bd.ReadyAll(b.App, "")
	if len(issues) != 1 || issues[0].ID != "a-1" {
		t.Errorf("the readable repo still reports its work: %+v", issues)
	}
	if len(failed) != 1 {
		t.Fatalf("want the unreadable store reported once, got %v", failed)
	}
	var se ScanError
	if !errors.As(failed[0], &se) || se.Dir != unread {
		t.Errorf("a scan failure must name its repo: %v", failed[0])
	}
	if !IsBdStoreUnread(failed[0]) {
		t.Errorf("the typed error must survive the ScanError wrap: %v", failed[0])
	}
}

// And the behaviour change the bead asked for, at the surface the operator
// reads: the pass says the queue is unknown and names the repair, where it
// used to say "no ready work" over a store nobody could read.
func TestQAPassRefusesAZeroRowStoreAsAnEmptyQueue(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	unread := unreadRepo(t, `{"issue_count": 0}`, oneRecord)
	scanConfig(t, b.App, unread)

	n, err := d.Run("", "", 0)
	if n != 0 {
		t.Errorf("nothing is dispatchable from a store that cannot be read: %d", n)
	}
	out := dispatcherOut(d)
	if err == nil || !strings.Contains(err.Error(), "unknown, not empty") {
		t.Fatalf("a zero-row store must fail the pass as an unknown queue: %v\n%s", err, out)
	}
	if strings.Contains(out, "no ready work") {
		t.Errorf("the silent shape is the whole defect:\n%s", out)
	}
	if !strings.Contains(out, "ready scan failed") || !strings.Contains(out, "bd import -i .beads/issues.jsonl") {
		t.Errorf("the pass must name the failed scan and the operator's repair:\n%s", out)
	}
}
