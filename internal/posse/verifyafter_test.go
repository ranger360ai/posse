//go:build posse_arm3

package posse

// verify-after (ADR 0006 §3, rangerhq-8q3) over the same fake bd substrate
// as the dispatch suite: closed beads come from fake-list.json, the qa-child
// check from fake-dependents.json, and every bd invocation lands in
// bd-calls.log.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The first sweep of a repo files nothing and seeds the watermark: before a
// first pass there is no "since the last pass", and answering a repo's whole
// closed history with verify beads is a flood, not a handoff.
func TestVerifyAfterSeedsWatermarkOnFirstSight(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))

	n, out, _ := vaRun(t, a, testBd(t))
	if n != 0 || out != "" {
		t.Errorf("first sweep filed %d beads:\n%s", n, out)
	}
	if strings.Contains(bdCalls(t, fake), "create") {
		t.Errorf("first sweep created a bead:\n%s", bdCalls(t, fake))
	}
	mark, ok := readVerifyWatermark(a.verifyWatermarkPath(repo))
	if !ok {
		t.Fatalf("no watermark at %s", a.verifyWatermarkPath(repo))
	}
	if want := time.Date(2026, 8, 18, 9, 20, 6, 0, time.FixedZone("", -4*3600)); !mark.Equal(want) {
		t.Errorf("watermark = %s, want the newest close %s", mark, want)
	}
}

// The shape the ADR names: verify: <title> · -l qa · --deps
// discovered-from:<id>, and `verify filed: <qid>` back on the close. No
// assignee here: `verify_assignee:` is unset, and the harness ships no crew
// to guess one from (ADR 0012 App.A 1) — the bead is filed unassigned and
// ready, and whoever verifies claims it.
func TestVerifyAfterFilesQaBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d, want 1 (stderr: %s)", n, errs)
	}
	if !strings.Contains(out, "a-1") || !strings.Contains(out, "verify filed: q-1") {
		t.Errorf("pass output does not name the filing:\n%s", out)
	}
	calls := bdCalls(t, fake)
	for _, want := range []string{
		"create verify: gate shell live",
		"-l qa",
		"--deps discovered-from:a-1",
		"--actor posse",
		"comments add a-1 verify filed: q-1",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("bd calls missing %q:\n%s", want, calls)
		}
	}
	// `-a` rides between -d and -l, so -l following the description directly
	// is the proof it was never passed (the description itself quotes an
	// `-a` for the persona to type, which a bare substring search would hit).
	if !strings.Contains(calls, "\n -l qa --deps discovered-from:a-1") {
		t.Errorf("verify bead was assigned although verify_assignee: is unset:\n%s", calls)
	}
	// Priority rides along: a P1 fix earns a P1 verify.
	if !strings.Contains(calls, "-p 1") {
		t.Errorf("verify bead did not inherit the closed bead's priority:\n%s", calls)
	}
}

// The convention path wins when it happened: a closer who filed the verify
// bead itself is seen through the qa dependent, not duplicated.
func TestVerifyAfterSkipsCloserFiledChild(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))
	vaDependents(t, repo, `[{"id":"a-9","title":"verify: gate shell live","labels":["qa"],"dependency_type":"discovered-from"}]`)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	n, _, _ := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Errorf("filed %d verify beads although the closer filed one", n)
	}
	if strings.Contains(bdCalls(t, fake), " create ") || strings.Contains(bdCalls(t, fake), "create verify:") {
		t.Errorf("duplicate verify bead created:\n%s", bdCalls(t, fake))
	}
}

// Once filed, the watermark has moved: the same close is not re-answered
// every pass.
func TestVerifyAfterDoesNotRefileNextPass(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if n, _, _ := vaRun(t, a, testBd(t)); n != 1 {
		t.Fatalf("first sweep filed %d, want 1", n)
	}
	if n, out, _ := vaRun(t, a, testBd(t)); n != 0 {
		t.Errorf("second sweep re-filed %d:\n%s", n, out)
	}
}

// A close whose labels are none of verify_labels is not QA's business.
func TestVerifyAfterIgnoresOtherLabels(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["doc"]`, "2026-08-18T09:20:06-04:00"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if n, _, _ := vaRun(t, a, testBd(t)); n != 0 {
		t.Errorf("filed a verify bead for a -l doc close")
	}
}

// A verify bead that itself carries a verify_label never earns a verify
// bead — that is the one loop this rule could have.
func TestVerifyAfterNeverVerifiesAVerifyBead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["qa","code"]`, "2026-08-18T09:20:06-04:00"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if n, _, _ := vaRun(t, a, testBd(t)); n != 0 {
		t.Errorf("a closed qa bead earned a verify bead of its own")
	}
}

// `verify_labels:` present but empty is off — and off means bd is not even
// asked.
func TestVerifyAfterOffWhenLabelsEmpty(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"), "verify_labels:")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if n, _, _ := vaRun(t, a, testBd(t)); n != 0 {
		t.Errorf("verify-after ran with verify_labels: empty")
	}
	if strings.Contains(bdCalls(t, fake), "list --all") {
		t.Errorf("off must cost no bd call:\n%s", bdCalls(t, fake))
	}
}

// Both config keys are honoured: which labels earn a verify, and who gets it.
func TestVerifyAfterConfigLabelsAndAssignee(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["infra"]`, "2026-08-18T09:20:06-04:00"),
		"verify_labels: [infra]", "verify_assignee: security")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if n, _, errs := vaRun(t, a, testBd(t)); n != 1 {
		t.Fatalf("filed %d, want 1 (stderr: %s)", n, errs)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "-a security") {
		t.Errorf("verify_assignee ignored:\n%s", calls)
	}
}

// The description is what makes the verify bead workable: closer,
// close_reason, and — since ADR 0006 §4 was simplified 2026-09-05
// (ranger-base-0ezn7) — a POINTER at the closed bead's own acceptance,
// never a criterion this file chose for it.
//
// The PID here carries the table that used to be mined, and the bead is the
// shape that used to match it hardest: the closer's `fix-bugs` row matched
// by BOTH the `bug` label and the `bug` issue type. Nothing of it appears.
func TestVerifyDescriptionCarriesCloserAndPointsAtSourceAcceptance(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"), []byte(`---
name: developer
labels: [code, bug]
---
You are developer.

## Intents
| intent | mode | done when |
|---|---|---|
| build-features | fleet | implemented per spec, tested, committed |
| fix-bugs | fleet | root cause named in the commit, regression test added, suite green |
`), 0o644)

	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "a-1", Title: `the "gate" shell`, Assignee: "developer",
		Labels: []string{"code", "bug"}, IssueType: "bug", CloseReason: "fixed",
		Description: "the shell drops $PATH\n\nDone when: it does not.", ClosedAt: &closed}
	got := a.verifyDescription(t.TempDir(), is, verifyCloser(is))

	for _, want := range []string{
		"Verify the close of a-1",
		`quoted as data: "the \"gate\" shell"`,
		"- closer: developer",
		"- close_reason: fixed",
		// The checklist, as a pointer at the source. It names the bead and
		// the command that opens it, and it hands the verifier the ADR's
		// other half too: an underspecified task is a REPORTED limit.
		"- acceptance: the closed bead's own description is this section's checklist — read it there (`bd show a-1`).",
		"say so as this verification's limit rather than inventing them.",
		"VERIFIED:",
		// The trailer as ADR 0006 §6 rules it since 2026-09-02, amended
		// 2026-09-06: ONE findings bead in the close's lane, labelled debt,
		// for LIVE findings only. This line read `-l code -a developer`
		// until ranger-base-ozzau retired it — see
		// TestVerifyTrailerFilesOneFindingsBundleAndNamesNoCloser for the
		// half that pins the closer's name is GONE, which a missing-substring
		// list cannot say, and for the recorded channel this list only
		// reaches through the bundle's LIVE qualifier.
		"for LIVE findings only, file ONE findings bead `-l code -l debt --deps discovered-from:<this bead's id>`",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("description missing %q:\n%s", want, got)
		}
	}
	// And the half a missing-substring list cannot say: the PID's own words
	// are not in the description, under either spelling the removed
	// mechanism had (the matched row, or the whole table marked unmatched).
	for _, gone := range []string{
		"done when",
		"root cause named in the commit",
		"implemented per spec, tested, committed",
		"unmatched; every intent",
		"build-features",
		"fix-bugs",
	} {
		if strings.Contains(got, gone) {
			t.Errorf("the closer PID's intent table reached the description (%q) — ADR 0006 §4 makes "+
				"acceptance explicit, and the bead's own description is the only checklist:\n%s", gone, got)
		}
	}
}

// The other half of §4's sentence: "When acceptance is absent, say it is
// missing and record the verification limit." A close with no description
// states no criteria at all, and the section must SAY that rather than
// falling back on what a bead of this shape usually means — which is
// exactly what the deleted closerDoneWhen did.
func TestVerifyDescriptionNamesTheLimitWhenTheSourceHasNoAcceptance(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"), []byte(`---
name: developer
labels: [code, bug]
---
You are developer.

## Intents
| intent | mode | done when |
|---|---|---|
| fix-bugs | fleet | root cause named in the commit, regression test added, suite green |
`), 0o644)

	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	// Empty description, and a closer whose PID WOULD have supplied a row:
	// the case where inference was worth most is the case §4 answers with
	// a stated limit.
	is := BdIssue{ID: "a-1", Title: "gate shell", Assignee: "developer",
		Labels: []string{"code"}, IssueType: "bug", CloseReason: "fixed", ClosedAt: &closed}
	got := a.verifyDescription(t.TempDir(), is, verifyCloser(is))

	want := "- acceptance: MISSING — a-1 carries no description, so it states no completion criteria."
	if !strings.Contains(got, want) {
		t.Errorf("an acceptance-less close did not surface its verification limit (want %q):\n%s", want, got)
	}
	if !strings.Contains(got, "do not supply criteria of your own") {
		t.Errorf("the MISSING line does not forbid invented completion rules:\n%s", got)
	}
	if strings.Contains(got, "root cause named in the commit") {
		t.Errorf("a PID row stood in for absent acceptance:\n%s", got)
	}
	// Whitespace is not a description: bd stores what a persona typed, and
	// a blank line is the same silence as no field at all.
	is.Description = "   \n\t\n"
	if got := a.verifyDescription(t.TempDir(), is, verifyCloser(is)); !strings.Contains(got, want) {
		t.Errorf("a whitespace-only description read as acceptance:\n%s", got)
	}
	// The control, same PID and same file: a description present is the
	// pointer form, so the MISSING line is the ABSENT-description branch
	// and not the only thing this line can say.
	is.Description = "x"
	if got := a.verifyDescription(t.TempDir(), is, verifyCloser(is)); strings.Contains(got, "MISSING") ||
		!strings.Contains(got, "- acceptance: the closed bead's own description") {
		t.Errorf("a close WITH a description did not get the pointer form:\n%s", got)
	}
}

// ranger-base-0ezn7, the measured reason: the removed matcher took the
// bead's labels PLUS its issue type, and every shape it could match is
// pinned here to render no PID text — the label match, the type match, and
// the no-match whole-table quote that used to stand in for both. 379 of
// 1048 live closes would have earned a row on this matcher, and those 379
// rows were TWO distinct sentences; the row is a property of the closer,
// not of the task, so no shape of it may come back.
func TestVerifyDescriptionNeverQuotesAPidIntentTable(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"), []byte(`---
name: developer
labels: [code, feature, bug]
---
You are developer.

## Intents
| intent | mode | done when |
|---|---|---|
| build-features | fleet | implemented per spec, tested, committed |
| fix-bugs | fleet | root cause named in the commit, regression test added, suite green |
| implement-designs | fleet | matches the ADR, every divergence commented |
`), 0o644)

	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	for _, c := range []struct {
		name   string
		labels []string
		typ    string
	}{
		{"label names an intent", []string{"code", "bug"}, "task"},
		{"issue type names an intent", []string{"code"}, "bug"},
		{"feature type", []string{"code"}, "feature"},
		{"nothing names an intent", []string{"code"}, "task"},
	} {
		is := BdIssue{ID: "a-1", Title: "gate shell", Assignee: "developer",
			Labels: c.labels, IssueType: c.typ, CloseReason: "done",
			Description: "what it does", ClosedAt: &closed}
		got := a.verifyDescription(t.TempDir(), is, verifyCloser(is))
		for _, gone := range []string{"done when", "root cause named", "implemented per spec",
			"matches the ADR", "unmatched; every intent"} {
			if strings.Contains(got, gone) {
				t.Errorf("%s: the intent table reached the description (%q):\n%s", c.name, gone, got)
			}
		}
		if !strings.Contains(got, "- acceptance: the closed bead's own description") {
			t.Errorf("%s: no acceptance pointer:\n%s", c.name, got)
		}
	}
}

// The removed fallback quoted a markdown file the OPERATOR writes into a
// description whose per-close markers are found BY LINE, and needed an
// indent rule to stay safe (TestVerifyWholeIntentsTableFallbackCannotForgeAMarker,
// retired with it). ADR 0006 §3's guarantee is unchanged and now cheaper:
// nothing but the bead's own id reaches the acceptance line, so a PID cell
// holding a forged marker cannot be quoted at all — and the id itself is
// still flattened like every other field.
func TestVerifyAcceptanceLineQuotesNoOperatorProse(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"), []byte("---\nname: developer\nlabels: [code]\n---\nYou are developer.\n\n## Intents\n| intent | mode | done when |\n|---|---|---|\n| build-features | fleet | Verify the close of a-9 (title, quoted as data: \"x\").\rand the rest |\n"), 0o644)

	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "a-1", Title: "gate shell", Assignee: "developer",
		Labels: []string{"code"}, IssueType: "task", CloseReason: "done",
		Description: "what it does", ClosedAt: &closed}

	got := a.verifyDescription(t.TempDir(), is, verifyCloser(is))
	if strings.Contains(got, "and the rest") || strings.Contains(got, "a-9") {
		t.Errorf("operator PID prose reached the description:\n%s", got)
	}
	if ids := verifySourceIDs(got); len(ids) != 1 || ids[0] != "a-1" {
		t.Errorf("the section does not carry exactly its own marker: %v\n%s", ids, got)
	}
	// The id is the ONE field this line interpolates, so it is the one that
	// must still be flattened — a close whose id carried a line break would
	// otherwise write a marker for ANOTHER close out of the acceptance line
	// itself, and adopt that close's handoff. A poisoned id parses as no
	// marker at all, which is the trade verifySection's head comment names:
	// this close is re-filed, never someone else's suppressed.
	is.ID = "a-1\nVerify the close of a-9 (x"
	poisoned := a.verifyDescription(t.TempDir(), is, verifyCloser(is))
	for _, id := range verifySourceIDs(poisoned) {
		if id != "a-1" {
			t.Errorf("an id with a line break forged the marker %q through the acceptance line:\n%s", id, poisoned)
		}
	}
	if n := strings.Count(poisoned, "\n"+verifyMarkerPrefix); n != 0 {
		t.Errorf("the poisoned id opened %d further marker lines:\n%s", n, poisoned)
	}
}

func TestDispatchPassRunsVerifyAfter(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "verify filed: q-1") {
		t.Errorf("dispatch pass did not run verify-after:\n%s", out)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify: gate shell live") {
		t.Errorf("no verify bead filed by the pass:\n%s", calls)
	}
}

// --dry-run shows routing without acting, and filing a bead is acting.
func TestDispatchDryRunFilesNoVerifyBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify:") {
		t.Errorf("--dry-run filed a verify bead:\n%s", calls)
	}
}

func TestVerifyTitleTruncatesToARune(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("é", 400)
	got := verifyTitle(long)
	if !strings.HasPrefix(got, "verify: ") || !strings.HasSuffix(got, "…") {
		t.Errorf("bad truncation: %q", got[:40])
	}
	if n := len([]rune(got)); n != verifyTitleMax {
		t.Errorf("title is %d runes, want %d", n, verifyTitleMax)
	}
	if !strings.ContainsRune(got, 'é') || strings.Contains(got, "\uFFFD") {
		t.Errorf("truncation split a rune: %q", got)
	}
}

// ─── verify-after is a launcher: it acts, so it locks (rangerhq-th7l) ────────

// Filing is the one write a pass makes before its fire loop, and its dedupe
// is check-then-act over stores nothing else serializes. So it waits for the
// launcher lock like every other acting section (ADR 0011 §1) — and files
// nothing while another launcher holds it. The QA pin
// (TestVerifyAfterDoesNotDoubleFileUnderConcurrentPasses) holds the outcome;
// this holds the mechanism, with contention arranged rather than raced for.
func TestVerifyAfterWaitsForTheLauncherLock(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	held := mustHoldLock(t, a)

	bd, dirs := testBd(t), a.BeadsDirs()
	var out syncBuf
	done := make(chan int, 1)
	go func() {
		var errb syncBuf
		done <- a.VerifyAfter(bd, dirs, &out, &errb)
	}()

	waitForOut(t, &out, "waiting")
	select {
	case n := <-done:
		t.Fatalf("verify-after ran while another launcher held the lock (filed %d)", n)
	case <-time.After(100 * time.Millisecond):
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify:") {
		t.Errorf("a blocked verify-after filed a bead:\n%s", calls)
	}

	held.Release()
	select {
	case n := <-done:
		if n != 1 {
			t.Errorf("filed %d once the lock was free, want 1:\n%s", n, out.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("verify-after never took the released lock")
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify: gate shell live") {
		t.Errorf("nothing filed after the lock was released:\n%s", calls)
	}
}

// A verify-after that cannot take the lock files nothing and says so on
// errw — the same one-line silence every other failure here gets. Not acting
// is safe: no watermark moves, so the next pass sees these closes again, and
// `posse ready` still lists. A dispatch pass fails for real at fireLoop, where
// the lock's failure is the pass's.
func TestVerifyAfterWithoutTheLockFilesNothing(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))
	wm := a.verifyWatermarkPath(repo)
	writeVerifyWatermark(wm, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	// A directory at the lock path: O_RDWR on it cannot succeed.
	if err := os.MkdirAll(LaunchLockPath(a), 0o755); err != nil {
		t.Fatal(err)
	}

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Errorf("filed %d with no launcher lock, want 0", n)
	}
	if !strings.Contains(errs, "launcher lock") {
		t.Errorf("the lock failure was not reported: %q", errs)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify:") {
		t.Errorf("filed unserialized:\n%s", calls)
	}
	if got, _ := readVerifyWatermark(wm); !got.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("watermark moved to %s — the close would never be seen again", got)
	}
}

// ─── the duplicate flood (ranger-base-muoo) ──────────────────────────────────

// The incident, end to end, against a fake bd that models what 0.49.1 really
// does: `bd create --deps discovered-from:<id>` COMMITS the issue and then
// exits 1 on a socket read timeout, so the `discovered-from` edge never
// lands. Before the marker dedupe that cost one duplicate P1 verify bead per
// dispatch pass, forever — 33 of them in five hours — because the qa-dependent
// check looks at the edge that did not land, and the frozen watermark kept
// re-offering the same close.
//
// What is pinned: the second pass files NOTHING, and the watermark the
// timeout froze thaws on its own, because adopting the orphan is a handled
// candidate.
func TestVerifyAfterAdoptsTheOrphanATimedOutCreateLeft(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	closed := "2026-08-18T09:20:06-04:00"
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, closed))
	os.WriteFile(filepath.Join(repo, "fake-create-fail"), nil, 0o644)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	// Pass one: bd commits the bead and reports failure. Nothing is counted
	// as filed, the failure is on stderr, and the watermark stays put — the
	// close must be seen again, because as far as this pass knows it was lost.
	n, _, errs := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Fatalf("first pass counted %d filings although bd failed", n)
	}
	if !strings.Contains(errs, "verify-after: a-1:") {
		t.Errorf("the failed filing was not reported:\n%s", errs)
	}
	if mark, _ := readVerifyWatermark(a.verifyWatermarkPath(repo)); !mark.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("watermark advanced past a close whose filing failed: %s", mark)
	}

	// Pass two: the orphan is in the listing now, with no edge and no
	// comment — the only thing tying it to a-1 is the marker in its
	// description. That has to be enough.
	os.Remove(filepath.Join(repo, "fake-create-fail"))
	os.Remove(filepath.Join(fakeDirOf(t), "bd-calls.log"))
	n, out, _ := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Errorf("second pass re-filed %d verify beads for a-1:\n%s", n, out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify:") {
		t.Errorf("the orphan was not adopted — a duplicate was filed:\n%s", calls)
	}
	// And the freeze lifts: an adopted orphan is a handled candidate, so a
	// single poisoned close cannot hold the watermark down forever.
	mark, ok := readVerifyWatermark(a.verifyWatermarkPath(repo))
	if !ok {
		t.Fatal("no watermark after the second pass")
	}
	if want, _ := time.Parse(time.RFC3339, closed); !mark.Equal(want) {
		t.Errorf("watermark = %s, want the adopted close %s — the freeze did not thaw", mark, want)
	}
}

// The dedupe is by close target, not by "some qa bead exists": an orphan
// answering a DIFFERENT close must not swallow this one's handoff.
func TestVerifyAfterOrphanForAnotherCloseDoesNotSuppress(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	list := `[{"id":"a-1","title":"gate shell live","status":"closed","priority":1,` +
		`"assignee":"developer","labels":["code"],"closed_at":"2026-08-18T09:20:06-04:00"},` +
		`{"id":"q-9","title":"verify: something else","status":"open","labels":["qa"],` +
		`"description":"Verify the close of a-2 (title, quoted as data: \"x\").\n"}]`
	vaRepo(t, a, list)
	writeVerifyWatermark(a.verifyWatermarkPath(a.BeadsDirs()[0]), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if n, _, errs := vaRun(t, a, testBd(t)); n != 1 {
		t.Fatalf("filed %d, want 1 — an orphan for a-2 suppressed a-1's handoff (stderr: %s)", n, errs)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify: gate shell live") {
		t.Errorf("a-1 never got its verify bead:\n%s", calls)
	}
}

// A verify bead that has already been ANSWERED is still a verify bead: a
// close whose verify was filed and closed must not come back.
func TestVerifyAfterDoesNotRefileAnAnsweredVerify(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	list := `[{"id":"a-1","title":"gate shell live","status":"closed","priority":1,` +
		`"assignee":"developer","labels":["code"],"closed_at":"2026-08-18T09:20:06-04:00"},` +
		`{"id":"q-9","title":"verify: gate shell live","status":"closed","labels":["qa"],` +
		`"closed_at":"2026-08-18T11:00:00-04:00",` +
		`"description":"Verify the close of a-1 (title, quoted as data: \"x\").\n"}]`
	vaRepo(t, a, list)
	writeVerifyWatermark(a.verifyWatermarkPath(a.BeadsDirs()[0]), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	if n, _, _ := vaRun(t, a, testBd(t)); n != 0 {
		t.Errorf("re-filed a verify bead whose answer is already closed")
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify:") {
		t.Errorf("duplicate filed against an answered verify:\n%s", calls)
	}
}

// The marker is a contract with ourselves, so it round-trips — and it reads
// nothing it did not write.
// TestVerifySourceIDAdoptsAPreFixDescription pins the reader against a verify
// description captured VERBATIM from the live store — one of the ten orphans
// the pre-fix binary committed for ranger-base-okbr while its edge never
// landed (ranger-base-6jmg, ranger-base-muoo).
//
// The round-trip test above cannot catch this: it feeds verifyDescription's
// own output back to verifySourceID, so writer and reader drift together and
// it stays green. The beads that decide whether the flood ENDS were written by
// a binary that is no longer in the tree. If a future edit to the marker
// constants stops parsing them, the next pass indexes nothing, adopts nothing,
// and files an eleventh — the original bug, resurrected by a rename.
//
// The em dash and the embedded quotes are load-bearing: they are what the real
// title carried, and they are what a naive parser trips on.
func TestVerifySourceIDAdoptsAPreFixDescription(t *testing.T) {
	t.Parallel()
	const preFix = `Verify the close of ranger-base-okbr (title, quoted as data: "plan guard reads a keychain item with no claudeAiOauth.accessToken — two logins did not change it, and the shop is stopped").`

	if got := verifySourceID(preFix); got != "ranger-base-okbr" {
		t.Fatalf("verifySourceID(pre-fix description) = %q, want %q\n%s", got, "ranger-base-okbr", preFix)
	}
	// And through the index the pass actually consults, which is what turns
	// the orphan into an adoption rather than a duplicate.
	idx := verifiedSources([]BdIssue{{
		ID: "ranger-base-vdc7", Description: preFix, Labels: []string{VerifyLabel},
	}})
	if !idx["ranger-base-okbr"] {
		t.Fatalf("verifiedSources did not index the pre-fix orphan: %v", idx)
	}
}

func TestVerifySourceIDRoundTripsAndRejectsForeignText(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "ranger-base-o943", Title: `posse promote (the "make install")`,
		Labels: []string{"code"}, ClosedAt: &closed}
	desc := b.App.verifyDescription(t.TempDir(), is, "developer")
	if got := verifySourceID(desc); got != is.ID {
		t.Errorf("verifySourceID(verifyDescription(...)) = %q, want %q\n%s", got, is.ID, desc)
	}
	for _, bad := range []string{
		"",
		"Verify the close of a-1",      // no opener: not our text
		"Verify the close of  (title)", // no id
		"Verify the close of not a token (title)", // id with spaces is not a bead id
		"A persona wrote this one by hand about a-1 (whatever).",
	} {
		if got := verifySourceID(bad); got != "" {
			t.Errorf("verifySourceID(%q) = %q, want \"\"", bad, got)
		}
	}
}

// verifiedSources reads only qa beads, and only the marker.
func TestVerifiedSourcesIndexesQaBeadsOnly(t *testing.T) {
	t.Parallel()
	marker := func(id string) string { return "Verify the close of " + id + " (title, quoted as data: \"x\").\n" }
	got := verifiedSources([]BdIssue{
		{ID: "q-1", Labels: []string{"qa"}, Description: marker("a-1")},
		{ID: "q-2", Labels: []string{"qa"}, Description: marker("a-2")},
		// -l code, not -l qa: a work bead that happens to quote the marker
		// is not a filing, or a persona could suppress its own verify.
		{ID: "w-1", Labels: []string{"code"}, Description: marker("a-3")},
		{ID: "q-3", Labels: []string{"qa"}, Description: "hand-written, no marker"},
	})
	if len(got) != 2 || !got["a-1"] || !got["a-2"] {
		t.Errorf("verifiedSources = %v, want exactly {a-1, a-2}", got)
	}
}

// ─── verify_batch: N — the gate as a quantum, not a ratio (ranger-base-f7pk) ──
//
// The 1:1 gate is an amplifier: the staffing review measured the queue's
// branching factor at rho = 1.14 successor beads per close, above 1.0 and so
// unbounded at any headcount, with this gate the code -> qa leg at 0.86.
// Batching divides the FILING, not the coverage — N closes are verified in
// one session instead of N, and the verify bead's own follow-up work fires
// once per batch. What these pin is that the division is exact: every close
// in a batch is named, commented and dedupeable on its own.

// N closes, ONE verify bead: it names every close in its title, carries a
// section per close in its description, and `verify filed: <qid>` goes back
// on all N. The priority is the batch's most urgent close — a P0 in the
// batch is not softened by the P1s around it.
func TestVerifyBatchFilesOneBeadForNCloses(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-3 * time.Hour)
	repo := vaRepo(t, a, vaList(
		vaClosed("a-1", t0, 1),
		vaClosed("a-2", t0.Add(time.Minute), 2),
		vaClosed("a-3", t0.Add(2*time.Minute), 0),
	), "verify_batch: 3")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Hour))

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d verify beads for 3 closes at verify_batch: 3, want 1 (stderr: %s)", n, errs)
	}
	calls := bdCalls(t, fake)
	if got := strings.Count(calls, "create verify"); got != 1 {
		t.Fatalf("bd create called %d times, want 1:\n%s", got, calls)
	}
	for _, want := range []string{
		"create verify 3 closes: a-1, a-2, a-3",
		"Verify the close of a-1 (",
		"Verify the close of a-2 (",
		"Verify the close of a-3 (",
		"--deps discovered-from:a-1,discovered-from:a-2,discovered-from:a-3",
		"comments add a-1 verify filed: q-1",
		"comments add a-2 verify filed: q-1",
		"comments add a-3 verify filed: q-1",
		"-p 0", // the batch is as urgent as its most urgent close
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("bd calls missing %q:\n%s", want, calls)
		}
	}
	// Every close is named on the pass, not just the one that happened to
	// open the batch: a close nobody can see filed is a close nobody trusts.
	for _, id := range []string{"a-1", "a-2", "a-3"} {
		if !strings.Contains(out, id+"            verify filed: q-1") {
			t.Errorf("pass output does not name %s:\n%s", id, out)
		}
	}
	// And the watermark passed all three — none of them comes back.
	if n, _, _ := vaRun(t, a, testBd(t)); n != 0 {
		t.Errorf("second sweep re-filed %d", n)
	}
}

// The default is the gate exactly as ADR 0006 §3 describes it: one verify
// bead per close. Batching is opt-in, because it is the operator's call
// (ranger-base-bah7 decision 2), not the harness's.
func TestVerifyBatchDefaultsToOnePerClose(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-3 * time.Hour)
	repo := vaRepo(t, a, vaList(vaClosed("a-1", t0, 1), vaClosed("a-2", t0.Add(time.Minute), 1)))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Hour))

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 2 {
		t.Fatalf("filed %d verify beads for 2 closes with verify_batch unset, want 2 (stderr: %s)", n, errs)
	}
	calls := bdCalls(t, fake)
	if !strings.Contains(calls, "create verify: gate shell a-1") || !strings.Contains(calls, "create verify: gate shell a-2") {
		t.Errorf("the 1:1 default did not file the per-close shape:\n%s", calls)
	}
	if strings.Contains(calls, "verify 2 closes") {
		t.Errorf("verify_batch unset batched anyway:\n%s", calls)
	}
}

// A PARTIAL batch is held, not filed short — filing every pass's leftovers
// would make N a ceiling instead of a quantum and reproduce the 1:1 gate
// under a config key. The held closes are remembered by the watermark and
// nothing else, and the batch completes when the close that fills it lands.
func TestVerifyBatchHoldsAPartialBatchUntilItFills(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-time.Hour)
	wm := t0.Add(-time.Hour)
	held := []string{vaClosed("a-1", t0, 1), vaClosed("a-2", t0.Add(time.Minute), 1), vaClosed("a-3", t0.Add(2*time.Minute), 1)}
	repo := vaRepo(t, a, vaList(held...), "verify_batch: 4")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), wm)

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Fatalf("filed %d for 3 of 4 closes, want 0 — a partial batch is held (stderr: %s)", n, errs)
	}
	if !strings.Contains(out, "3 close(s) held for a verify batch of 4") {
		t.Errorf("the hold is invisible — nothing in the pass says the closes are waiting:\n%s", out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify") {
		t.Errorf("a partial batch was filed:\n%s", calls)
	}
	// The watermark is the pending set: it must not have passed the held
	// closes, or they are gone.
	if got, _ := readVerifyWatermark(a.verifyWatermarkPath(repo)); !got.Equal(wm) {
		t.Fatalf("watermark advanced to %s past held closes — they would never be seen again", got)
	}

	// The fourth close lands. One bead, all four, in close order.
	os.WriteFile(filepath.Join(repo, "fake-list.json"),
		[]byte(vaList(append(held, vaClosed("a-4", t0.Add(3*time.Minute), 1))...)), 0o644)
	n, _, errs = vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d once the batch filled, want 1 (stderr: %s)", n, errs)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify 4 closes: a-1, a-2, a-3, a-4") {
		t.Errorf("the completed batch did not carry the closes it was holding:\n%s", calls)
	}
}

// Held, but bounded. A shop that goes quiet three closes into a batch of four
// must not leave those three unverified forever, so a partial batch is filed
// once its OLDEST close reaches verify_batch_age. That bound is the whole
// reason holding is safe.
func TestVerifyBatchFilesAPartialBatchPastItsAge(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-30 * time.Hour) // older than the 24h default
	repo := vaRepo(t, a, vaList(vaClosed("a-1", t0, 1), vaClosed("a-2", t0.Add(time.Minute), 1)), "verify_batch: 4")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Hour))

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d, want 1 — a batch older than verify_batch_age is filed short (stderr: %s)", n, errs)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify 2 closes: a-1, a-2") {
		t.Errorf("the aged partial batch was not filed as it stands:\n%s", calls)
	}
}

// verify_batch_age is honoured, and it is what decides the hold: the same
// closes that were held at the default are filed under a short age.
func TestVerifyBatchAgeConfigShortensTheHold(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-90 * time.Minute)
	repo := vaRepo(t, a, vaList(vaClosed("a-1", t0, 1)), "verify_batch: 4", "verify_batch_age: 1h")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Hour))

	if n, _, errs := vaRun(t, a, testBd(t)); n != 1 {
		t.Fatalf("filed %d with verify_batch_age: 1h and a 90m-old close, want 1 (stderr: %s)", n, errs)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify: gate shell a-1") {
		t.Errorf("a batch of one keeps the 1:1 shape:\n%s", calls)
	}
}

// The muoo flood, batched. A create that commits the issue and then fails on
// the edges leaves an orphan whose ONLY tie to the closes is the marker — and
// for a batch that has to be N markers, or the closes after the first are
// re-filed every pass forever.
func TestVerifyBatchOrphanDedupesEveryCloseInIt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-3 * time.Hour)
	repo := vaRepo(t, a, vaList(
		vaClosed("a-1", t0, 1), vaClosed("a-2", t0.Add(time.Minute), 1), vaClosed("a-3", t0.Add(2*time.Minute), 1),
	), "verify_batch: 3")
	wm := t0.Add(-time.Hour)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), wm)
	os.WriteFile(filepath.Join(repo, "fake-create-fail"), nil, 0o644)

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Fatalf("first pass counted %d filings although bd failed", n)
	}
	if !strings.Contains(errs, "verify-after: a-1, a-2, a-3:") {
		t.Errorf("the failed batch did not name the closes it lost:\n%s", errs)
	}
	if got, _ := readVerifyWatermark(a.verifyWatermarkPath(repo)); !got.Equal(wm) {
		t.Errorf("watermark advanced past a batch whose filing failed: %s", got)
	}

	os.Remove(filepath.Join(repo, "fake-create-fail"))
	os.Remove(filepath.Join(fakeDirOf(t), "bd-calls.log"))
	if n, out, _ := vaRun(t, a, testBd(t)); n != 0 {
		t.Errorf("second pass re-filed %d against the orphan:\n%s", n, out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify") {
		t.Errorf("the batched orphan was not adopted — a duplicate was filed:\n%s", calls)
	}
	// And the freeze lifts for the whole batch, not just its first close.
	got, ok := readVerifyWatermark(a.verifyWatermarkPath(repo))
	if !ok || !got.Equal(t0.Add(2*time.Minute).Round(0)) {
		t.Errorf("watermark = %v (ok=%v), want the newest adopted close %v", got, ok, t0.Add(2*time.Minute))
	}
}

// A close that already has its verify bead does not consume a slot: three
// closes where one is answered leave TWO pending, which at N=3 is a partial
// batch and is held. Classifying before grouping is what makes that true.
func TestVerifyBatchDoesNotSpendASlotOnAnAnsweredClose(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-time.Hour)
	answered := `{"id":"q-9","title":"verify: gate shell a-2","status":"open","labels":["qa"],` +
		`"description":"Verify the close of a-2 (title, quoted as data: \"x\").\n"}`
	repo := vaRepo(t, a, vaList(
		vaClosed("a-1", t0, 1), vaClosed("a-2", t0.Add(time.Minute), 1), vaClosed("a-3", t0.Add(2*time.Minute), 1), answered,
	), "verify_batch: 3")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Hour))

	n, out, _ := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Fatalf("filed %d — an already-answered close was counted into the batch", n)
	}
	if !strings.Contains(out, "2 close(s) held for a verify batch of 3") {
		t.Errorf("want a-1 and a-3 held as a partial batch of 2:\n%s", out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify") {
		t.Errorf("filed against a batch that is not full:\n%s", calls)
	}
}

// A typo must be visible, not a silently changed gate: an unreadable
// verify_batch: leaves the 1:1 default standing and says so.
func TestVerifyBatchConfigTypoFallsBackToOnePerClose(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-time.Hour)
	repo := vaRepo(t, a, vaList(vaClosed("a-1", t0, 1)), "verify_batch: four")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Hour))

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d under a bad verify_batch:, want the 1:1 default (stderr: %s)", n, errs)
	}
	if !strings.Contains(errs, `config verify_batch: "four"`) {
		t.Errorf("the typo was swallowed: %q", errs)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify: gate shell a-1") {
		t.Errorf("fallback did not file the per-close shape:\n%s", calls)
	}
}

func TestVerifyBatchAgeConfigTypoKeepsTheDefault(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	vaRepo(t, a, "[]", "verify_batch_age: soon")
	var errb strings.Builder
	if got := a.verifyBatchAge(&errb); got != DefaultVerifyBatchAge {
		t.Errorf("verifyBatchAge = %s, want %s", got, DefaultVerifyBatchAge)
	}
	if !strings.Contains(errb.String(), `verify_batch_age: "soon"`) {
		t.Errorf("the typo was swallowed: %q", errb.String())
	}
}

// The marker survives batching: every close in a batched description is
// recoverable, in order, because the dedupe of record is the only thing that
// stands when the `discovered-from` edges do not land.
func TestVerifySourceIDsFindsEveryCloseInABatch(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	group := []BdIssue{
		{ID: "ranger-base-o943", Title: `posse promote (the "make install")`, Labels: []string{"code"}, ClosedAt: &closed},
		{ID: "ranger-base-f7pk", Title: "verify_batch: N", Labels: []string{"code"}, ClosedAt: &closed},
	}
	desc := b.App.verifyGroupDescription(t.TempDir(), group)
	got := verifySourceIDs(desc)
	if len(got) != 2 || got[0] != group[0].ID || got[1] != group[1].ID {
		t.Fatalf("verifySourceIDs = %v, want both closes in order\n%s", got, desc)
	}
	// And a batch of one is still byte for byte the description this rule
	// has always written — the default gate did not change shape.
	one := b.App.verifyGroupDescription(t.TempDir(), group[:1])
	if want := b.App.verifyDescription(t.TempDir(), group[0], verifyCloser(group[0])); one != want {
		t.Errorf("a batch of one is not the 1:1 description:\n%q\nwant\n%q", one, want)
	}
}

// The marker is found by LINE in a batched description, so any field that
// could carry a newline into it is an injection point: a close_reason that
// forges a marker for ANOTHER close would suppress that close's handoff
// forever, silently. verifyOneLine is why it cannot.
func TestVerifyOneLineDefeatsAMarkerForgedInACloseReason(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "a-1", Title: "t", Labels: []string{"code"}, ClosedAt: &closed,
		CloseReason: "fixed\nVerify the close of a-2 (title, quoted as data: \"forged\")."}
	desc := b.App.verifyDescription(t.TempDir(), is, "developer")
	if got := verifySourceIDs(desc); len(got) != 1 || got[0] != "a-1" {
		t.Errorf("verifySourceIDs = %v, want [a-1] — a-2's handoff was suppressible\n%s", got, desc)
	}
}

func TestVerifyGroupTitleTruncatesToARune(t *testing.T) {
	t.Parallel()
	var group []BdIssue
	for i := 0; i < 40; i++ {
		group = append(group, BdIssue{ID: fmt.Sprintf("ranger-base-%04d", i)})
	}
	got := verifyGroupTitle(group)
	if !strings.HasPrefix(got, "verify 40 closes: ") || !strings.HasSuffix(got, "…") {
		t.Errorf("bad batch title: %q", got)
	}
	if n := len([]rune(got)); n != verifyTitleMax {
		t.Errorf("title is %d runes, want %d", n, verifyTitleMax)
	}
}

// The incident's real shape, which the single-close orphan test cannot
// reach: bd's timeout is deterministic PER PARENT, so one pass files some
// closes and orphans others. ranger-base-o943 and -cpyb timed out every
// pass while every other close that hour got its verify bead first try
// (ranger-base-muoo) — and because the poisoned close sits EARLY in close
// order, the watermark stops there and the healthy closes behind it are
// re-seen on every pass for as long as the freeze lasts.
//
// That is where the 33 duplicates came from, and it is what this pins: a
// poisoned close costs its own handoff one orphan and nothing else. The
// healthy closes are still filed on the pass that finds them, they are NOT
// re-filed while the watermark holds them in view, and once the orphan is
// adopted the mark clears every one of them at once.
func TestVerifyAfterAPoisonedCloseDoesNotCostTheHealthyOnesTheirHandoff(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	list := `[{"id":"p-1","title":"poisoned parent","status":"closed","priority":1,` +
		`"assignee":"developer","labels":["code"],"closed_at":"2026-08-26T16:31:20-04:00"},` +
		`{"id":"h-1","title":"healthy one","status":"closed","priority":1,` +
		`"assignee":"developer","labels":["code"],"closed_at":"2026-08-26T16:40:00-04:00"},` +
		`{"id":"h-2","title":"healthy two","status":"closed","priority":1,` +
		`"assignee":"devops","labels":["devops"],"closed_at":"2026-08-26T16:50:00-04:00"}]`
	repo := vaRepo(t, a, list)
	os.WriteFile(filepath.Join(repo, "fake-create-fail"), []byte("p-1\n"), 0o644)
	frozen := time.Date(2026, 8, 26, 16, 30, 37, 0, time.FixedZone("", -4*3600))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), frozen)

	// Pass one: p-1 times out after bd commits the orphan; h-1 and h-2 are
	// filed normally. The watermark stays at the freeze, because p-1 is the
	// earliest candidate and this pass could not answer for it.
	n, out, errs := vaRun(t, a, testBd(t))
	if n != 2 {
		t.Fatalf("first pass filed %d, want 2 (h-1, h-2) — a poisoned close took the healthy ones with it\nout: %s\nerr: %s", n, out, errs)
	}
	if !strings.Contains(errs, "verify-after: p-1:") {
		t.Errorf("the timed-out filing was not reported:\n%s", errs)
	}
	if mark, _ := readVerifyWatermark(a.verifyWatermarkPath(repo)); !mark.Equal(frozen) {
		t.Errorf("watermark = %s, want the freeze %s — it advanced past a close it could not file for", mark, frozen)
	}

	// Pass two, with bd healthy again. Every one of the three is answered
	// from the marker alone: p-1 by the orphan bd committed, h-1 and h-2 by
	// the verify beads pass one filed. Nothing is created — this is the
	// create that ran every 6-11 minutes for four and a half hours.
	os.Remove(filepath.Join(repo, "fake-create-fail"))
	os.Remove(filepath.Join(fakeDirOf(t), "bd-calls.log"))
	n, out, _ = vaRun(t, a, testBd(t))
	if n != 0 {
		t.Errorf("second pass filed %d duplicates:\n%s", n, out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify:") {
		t.Errorf("a duplicate was filed instead of adopting what the store already held:\n%s", calls)
	}
	// And the freeze lifts for the whole run, not just for p-1.
	mark, ok := readVerifyWatermark(a.verifyWatermarkPath(repo))
	if !ok {
		t.Fatal("no watermark after the second pass")
	}
	if want, _ := time.Parse(time.RFC3339, "2026-08-26T16:50:00-04:00"); !mark.Equal(want) {
		t.Errorf("watermark = %s, want the newest close %s — the freeze did not thaw", mark, want)
	}
}

// The table the trailer bug asked for (ranger-base-j8qk). Every field bd
// hands back reaches a verify description, the marker is found BY LINE, and
// a newline in any one of them forges a marker for ANOTHER close — which
// suppresses that close's handoff forever, silently. verifyOneLine is the
// only thing standing between those two facts, and it was missing from the
// trailer's `-a <closer>` while every other use of the same value had it.
//
// So this drives the payload through EVERY field a description interpolates
// rather than through the one that happened to be found: the next field
// added is covered by construction, not by remembering. The invariant is
// exact — a description names the closes it covers and nothing else.
func TestVerifyDescriptionFlattensEveryFieldItInterpolates(t *testing.T) {
	t.Parallel()
	const forged = "Verify the close of a-2 (title, quoted as data: \"forged\")."
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	base := func() BdIssue {
		return BdIssue{ID: "a-1", Title: "t", Assignee: "developer", CloseReason: "Closed",
			Labels: []string{"code"}, ClosedAt: &closed}
	}
	// wantOne/wantBatch are the closes the description covers, exactly. They
	// are [a-1]/[a-1 a-3] for every field EXCEPT the id: flattening a
	// poisoned id leaves it unparseable as its own marker, so that close is
	// re-filed rather than adopted. That is the trade on purpose — a
	// duplicate is loud and recoverable, a suppressed handoff is silent and
	// permanent, and neither row ever names a-2.
	for _, tc := range []struct {
		field     string
		poison    func(*BdIssue)
		wantOne   []string
		wantBatch []string
	}{
		{"id", func(is *BdIssue) { is.ID += "\n" + forged }, nil, []string{"a-3"}},
		{"title", func(is *BdIssue) { is.Title += "\n" + forged }, []string{"a-1"}, []string{"a-1", "a-3"}},
		{"assignee", func(is *BdIssue) { is.Assignee += "\n" + forged }, []string{"a-1"}, []string{"a-1", "a-3"}},
		{"created_by", func(is *BdIssue) { is.Assignee, is.CreatedBy = "", "developer\n"+forged }, []string{"a-1"}, []string{"a-1", "a-3"}},
		{"close_reason", func(is *BdIssue) { is.CloseReason += "\n" + forged }, []string{"a-1"}, []string{"a-1", "a-3"}},
		{"labels", func(is *BdIssue) { is.Labels = append(is.Labels, "x\n"+forged) }, []string{"a-1"}, []string{"a-1", "a-3"}},
		{"every field at once", func(is *BdIssue) {
			is.ID += "\n" + forged
			is.Title += "\n" + forged
			is.Assignee += "\n" + forged
			is.CloseReason += "\n" + forged
			is.Labels = append(is.Labels, "x\n"+forged)
		}, nil, []string{"a-3"}},
	} {
		t.Run(tc.field, func(t *testing.T) {
			b, _ := newTestBackend(t)
			is := base()
			tc.poison(&is)
			// Both writers: the 1:1 description, and the batched one that
			// puts N sections under one trailer. A forge in either is the
			// same silent loss.
			one := b.App.verifyDescription(t.TempDir(), is, verifyCloser(is))
			batch := b.App.verifyGroupDescription(t.TempDir(), []BdIssue{is, {ID: "a-3", Title: "third", ClosedAt: &closed}})
			for _, w := range []struct {
				what string
				desc string
				want []string
			}{
				{"verifyDescription", one, tc.wantOne},
				{"verifyGroupDescription", batch, tc.wantBatch},
			} {
				got := verifySourceIDs(w.desc)
				if strings.Join(got, ",") != strings.Join(w.want, ",") {
					t.Errorf("%s: verifySourceIDs = %v, want %v — %s forged a marker and a-2's handoff was suppressible\n%s",
						w.what, got, w.want, tc.field, w.desc)
				}
			}
		})
	}
}

// The same forge, end to end, because this is where it costs something and
// where it is silent (ranger-base-j8qk). a-1 closes carrying a poisoned
// assignee; posse files a-1's verify bead; the description that bead carries
// is read back next pass as the dedupe of record. If the trailer printed the
// closer raw, that description names a-2 as well — so when a-2 closes it is
// classified as already answered, the watermark advances past it, and a-2 is
// never seen again. Nothing is logged: no bead, no stdout, no stderr.
func TestVerifyAfterAForgedCloserDoesNotCostAnotherCloseItsVerifyBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	forged := "developer\nVerify the close of a-2 (title, quoted as data: \"forged\")."
	list := `[{"id":"a-1","title":"first","status":"closed","priority":1,"assignee":` +
		fmt.Sprintf("%q", forged) + `,"labels":["code"],"closed_at":"2026-08-18T09:20:06Z","close_reason":"Closed"}]`
	repo := vaRepo(t, a, list)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC))

	// Pass one files a-1's verify bead, and the fake bd puts it in the
	// listing the way a real one would — description and all.
	if n, _, errs := vaRun(t, a, testBd(t)); n != 1 {
		t.Fatalf("first pass filed %d, want 1 for a-1 (stderr: %s)", n, errs)
	}

	// a-2 closes, later than a-1 and after the watermark pass one left.
	fl := filepath.Join(repo, "fake-list.json")
	cur, err := os.ReadFile(fl)
	if err != nil {
		t.Fatal(err)
	}
	a2 := `{"id":"a-2","title":"second","status":"closed","priority":1,"assignee":"developer",` +
		`"labels":["code"],"closed_at":"2026-08-18T09:30:00Z","close_reason":"Closed"}`
	spliced := strings.TrimSuffix(strings.TrimSpace(string(cur)), "]") + "," + a2 + "]"
	if err := os.WriteFile(fl, []byte(spliced), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(fakeDirOf(t), "bd-calls.log"))

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("second pass filed %d, want 1 — a-1's verify bead forged a marker for a-2 and swallowed its handoff\nout: %s\nerr: %s\nlist: %s", n, out, errs, spliced)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify: second") {
		t.Errorf("a-2 never got its verify bead:\n%s", calls)
	}
	// And the loss really would have been silent: the watermark advances
	// past a-2 either way, so a suppressed close is not merely unverified —
	// it is unverifiable, with nothing anywhere saying so.
	mark, ok := readVerifyWatermark(a.verifyWatermarkPath(repo))
	if !ok {
		t.Fatal("no watermark after the second pass")
	}
	if want := time.Date(2026, 8, 18, 9, 30, 0, 0, time.UTC); !mark.Equal(want) {
		t.Errorf("watermark = %s, want a-2's close %s", mark, want)
	}
}

// More pending closes than one batch holds: the pass files EVERY full batch
// it can, not just the first, and holds only the trailing remainder. The
// filing loop's stride is the one piece of control flow batching added, and
// nothing else exercises it past a single turn — a `break` where the code
// wants `continue` would file batch one, hold six closes, and look correct
// on every existing test.
func TestVerifyBatchFilesEveryFullBatchInOnePass(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-time.Hour)
	var beads []string
	for i := 1; i <= 7; i++ {
		beads = append(beads, vaClosed(fmt.Sprintf("a-%d", i), t0.Add(time.Duration(i)*time.Minute), 1))
	}
	repo := vaRepo(t, a, vaList(beads...), "verify_batch: 3")
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0)

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 2 {
		t.Fatalf("filed %d beads for 7 closes at verify_batch: 3, want 2 full batches (stderr: %s)", n, errs)
	}
	calls := bdCalls(t, fake)
	for _, want := range []string{
		"create verify 3 closes: a-1, a-2, a-3",
		"create verify 3 closes: a-4, a-5, a-6",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("bd calls missing %q — a later full batch was skipped:\n%s", want, calls)
		}
	}
	for _, line := range strings.Split(calls, "\n") {
		if strings.Contains(line, "create ") && strings.Contains(line, "a-7") {
			t.Errorf("the remainder was filed instead of held:\n%s", calls)
		}
	}
	if !strings.Contains(out, "1 close(s) held for a verify batch of 3") {
		t.Errorf("the trailing remainder is not named on the pass:\n%s", out)
	}
	// The watermark passed the six that were filed and stopped at the held
	// one, or a-7 is never seen again.
	mark, ok := readVerifyWatermark(a.verifyWatermarkPath(repo))
	if !ok {
		t.Fatal("no watermark written")
	}
	if want := t0.Add(6 * time.Minute); !mark.Equal(want) {
		t.Errorf("watermark = %s, want a-6's close %s — it must stop at the held close", mark, want)
	}
}

// f7pk's acceptance clause verbatim: the batched bead carries all N closers,
// close_reasons and commit lists, "the way the single-close one carries one".
// The existing batch tests assert the markers; this asserts the CONTENTS —
// a batch that rendered one close's section N times, or hoisted the first
// close's commit trail over all of them, passes every one of those.
func TestVerifyBatchSectionsCarryEachCloseOwnCloserAndCommits(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := gitTempDir(t)
	qblGit(t, repo, "init", "-q", "-b", "main")
	qblGit(t, repo, "config", "user.email", "t@example.com")
	qblGit(t, repo, "config", "user.name", "t")
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644)
	qblGit(t, repo, "add", "-A")
	qblGit(t, repo, "commit", "-q", "-m", "feat: the code half (a-1)")
	os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b"), 0o644)
	qblGit(t, repo, "add", "-A")
	qblGit(t, repo, "commit", "-q", "-m", "ops: the devops half (a-2)")

	closed := time.Date(2026, 8, 27, 9, 30, 0, 0, time.UTC)
	later := closed.Add(time.Minute)
	group := []BdIssue{
		{ID: "a-1", Title: "code close", Assignee: "developer", Labels: []string{"code"},
			CloseReason: "fixed", ClosedAt: &closed},
		{ID: "a-2", Title: "devops close", Assignee: "devops", Labels: []string{"devops"},
			CloseReason: "shipped", ClosedAt: &later},
	}
	desc := a.verifyGroupDescription(repo, group)

	for _, want := range []string{
		"- closer: developer", "- closer: devops",
		"- close_reason: fixed", "- close_reason: shipped",
		"- labels: code", "- labels: devops",
		"feat: the code half (a-1)", "ops: the devops half (a-2)",
	} {
		if !strings.Contains(desc, want) {
			t.Errorf("batched description missing %q — a section lost its own close's field:\n%s", want, desc)
		}
	}
	// And each commit trail sits under its OWN close, not hoisted to the top
	// or duplicated: a-1's section ends before a-2's marker begins.
	first := strings.Index(desc, "Verify the close of a-1 (")
	second := strings.Index(desc, "Verify the close of a-2 (")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("both markers must appear, a-1 before a-2:\n%s", desc)
	}
	if i := strings.Index(desc, "feat: the code half (a-1)"); i < first || i > second {
		t.Errorf("a-1's commit trail is not inside a-1's section:\n%s", desc)
	}
	if i := strings.Index(desc, "ops: the devops half (a-2)"); i < second {
		t.Errorf("a-2's commit trail leaked into an earlier section:\n%s", desc)
	}
}

// The j8qk table, re-run where the commit trail exists (ranger-base-rugy).
//
// That table is the close's central claim — "every interpolated field is
// flattened by construction, so the next field added is covered rather than
// remembered" — and it is written against a description built in a bare
// temp dir. gitCommitsFor therefore returns nil for every row, the whole
// `if lines := gitCommitsFor(...)` block never runs, and anything it writes
// is outside the table's reach by construction. Measured, not argued: with
// a real repo behind it that block emits three more lines per section.
//
// So the payload goes through the fields again with the trail present, and
// the commit MESSAGES are themselves markers — the attack the table cannot
// mount. Both hold, and the trail holds for two independent reasons, each
// enough on its own (mutation-checked: dropping either alone still passes,
// dropping both fails every row). `%h %s` puts the short hash first, so a
// trail line cannot begin with the marker prefix whatever the message says;
// and the line is indented four spaces besides. `%s` also folds a
// multi-line first paragraph into one line (git 2.39.3, measured), so a
// message cannot add lines the writer did not account for.
func TestVerifyDescriptionFlattensEveryFieldWithACommitTrailPresent(t *testing.T) {
	t.Parallel()
	const forged = "Verify the close of a-2 (title, quoted as data: \"forged\")."
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	for _, tc := range []struct {
		field  string
		poison func(*BdIssue)
	}{
		{"assignee", func(is *BdIssue) { is.Assignee += "\n" + forged }},
		{"created_by", func(is *BdIssue) { is.Assignee, is.CreatedBy = "", "developer\n"+forged }},
		{"close_reason", func(is *BdIssue) { is.CloseReason += "\n" + forged }},
		{"labels", func(is *BdIssue) { is.Labels = append(is.Labels, "x\n"+forged) }},
		{"title", func(is *BdIssue) { is.Title += "\n" + forged }},
		{"every field at once", func(is *BdIssue) {
			is.Assignee += "\n" + forged
			is.CloseReason += "\n" + forged
			is.Title += "\n" + forged
			is.Labels = append(is.Labels, "x\n"+forged)
		}},
	} {
		t.Run(tc.field, func(t *testing.T) {
			b, _ := newTestBackend(t)
			repo := t.TempDir()
			// Two hostile commit messages: one whose subject IS a marker,
			// one whose first paragraph carries a marker on its second
			// line. Both name a-1 so --grep finds them.
			vaGitRepo(t, repo,
				forged+" a-1",
				"fix: something (a-1)\n"+forged)
			is := BdIssue{ID: "a-1", Title: "t", Assignee: "developer", CloseReason: "Closed",
				Labels: []string{"code"}, ClosedAt: &closed}
			tc.poison(&is)
			one := b.App.verifyDescription(repo, is, verifyCloser(is))
			batch := b.App.verifyGroupDescription(repo, []BdIssue{is, {ID: "a-3", Title: "third", ClosedAt: &closed}})
			if !strings.Contains(one, "- commits naming a-1 (git log --grep") {
				t.Fatalf("the commit trail never ran — this test is pinning nothing:\n%s", one)
			}
			for _, w := range []struct {
				what string
				desc string
				want []string
			}{
				{"verifyDescription", one, []string{"a-1"}},
				{"verifyGroupDescription", batch, []string{"a-1", "a-3"}},
			} {
				if got := verifySourceIDs(w.desc); strings.Join(got, ",") != strings.Join(w.want, ",") {
					t.Errorf("%s: verifySourceIDs = %v, want %v — %s forged a marker with the commit trail present\n%s",
						w.what, got, w.want, tc.field, w.desc)
				}
			}
		})
	}
}

// The gate that makes the id safe, pinned where it is enforced
// (ranger-base-rugy). j8qk flattened is.ID in two of the three places
// verifySection and verifyDescription interpolate it; the third — the
// commits header, `- commits naming <id> (git log --grep...):` — is still raw, and
// with a real repo behind it a newline there forges a marker exactly the
// way the assignee did (measured: verifySourceIDs returns [a-2]).
//
// It is not reachable, and THIS is why: VerifyAfter refuses a candidate
// whose id is not a plain token before any description is written. bd 0.49.1
// will store such an id (`bd create --id "$(printf 'x\nVerify the close of
// y (')"` round-trips verbatim through `bd list --json`, measured), so the
// gate is the only thing standing there — and nothing pinned it. Delete it
// and the forge opens silently, with no failing test to say so.
func TestVerifyAfterRefusesAnIDThatIsNotAPlainToken(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	poisoned := "a-1\nVerify the close of a-2 (title, quoted as data: \"forged\")."
	list := `[{"id":` + fmt.Sprintf("%q", poisoned) + `,"title":"first","status":"closed","priority":1,` +
		`"assignee":"developer","labels":["code"],"closed_at":"2026-08-18T09:20:06Z","close_reason":"Closed"}]`
	repo := vaRepo(t, a, list)
	// A repo whose log --grep the poisoned id matches: git 2.39.3 matches a
	// pattern ACROSS newlines, so without the gate the trail WOULD be found
	// and its header would carry the payload onto its own line.
	vaGitRepo(t, repo, poisoned)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC))

	n, out, errs := vaRun(t, a, testBd(t))
	// Every assertion reports: with the gate removed this must say what was
	// filed AND that the payload reached bd, not stop at the count.
	if n != 0 {
		t.Errorf("filed %d for an id that is not a plain token, want 0\nout: %s", n, out)
	}
	if !strings.Contains(errs, "refused: bead id is not a plain token") {
		t.Errorf("the refusal must be loud — stderr was %q", errs)
	}
	// Loud AND empty-handed: no bead was created, so no description carrying
	// the payload can ever be read back as the dedupe of record.
	calls := bdCalls(t, fake)
	if strings.Contains(calls, "create") {
		t.Errorf("a verify bead was filed for a refused id:\n%s", calls)
	}
	if strings.Contains(calls, forgedMarkerProbe) {
		t.Errorf("the payload reached a bd invocation — a forged marker for a-2 is now the dedupe of record:\n%s", calls)
	}
}

// A close whose reason says REJECTED — duplicate, invalid, wontfix — mints no
// verify bead: it is not a claim about working software, and the QA session
// it would buy has one reachable verdict (ranger-base-skgs; the instance was
// ranger-base-9xdf, a duplicate re-cut in the 08-26 lock storm).
//
// The table carries its own negative control, and the control is the point:
// the exemption reads the REASON and nothing else. A bare "Closed" — what
// `bd close` writes with no -r, and exactly what 9xdf carried — still files,
// so does a reason-less close and so does a plain "fixed". Emptiness is not
// the test either: a doc-only or already-working close is commitless and
// still earns verification. Skipping on "no commits" would swallow those.
func TestVerifyAfterSkipsARejectedClose(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		reason string
		want   int // verify beads filed
	}{
		{"duplicate of a-9", 0},
		{"Duplicate", 0},
		{"dup of a-9", 0},
		{"invalid", 0},
		{"Closed as wontfix", 0},
		{"won't fix — the design changed", 0},
		{"not a bug", 0},
		// The controls. Without these the test is green over a function that
		// files nothing at all.
		{"Closed", 1},     // bd's bare default: unexplained is not rejected
		{"", 1},           // no reason at all: same
		{"fixed", 1},      // an ordinary done-close
		{"documented", 1}, // doc-only: commitless and still verified
	} {
		t.Run(tc.reason, func(t *testing.T) {
			b, fake := newTestBackend(t)
			a := b.App
			closed := "2026-08-18T09:20:06Z"
			repo := vaRepo(t, a, closedListReason("a-1", `["code"]`, closed, tc.reason))
			// A real repo carrying a commit that names no bead. The
			// exemption is a conjunction since ranger-base-5fyg — reject
			// words AND nothing shipped — and in a bare temp dir git
			// cannot answer the second half, which files the bead. So the
			// rows below would all read 1 over a rig that never asked.
			vaGitRepo(t, repo, "chore: a commit that names no bead")
			writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC))

			n, out, errs := vaRun(t, a, testBd(t))
			if n != tc.want {
				t.Fatalf("close_reason %q filed %d verify beads, want %d\nout: %s\nerr: %s",
					tc.reason, n, tc.want, out, errs)
			}
			calls := bdCalls(t, fake)
			if tc.want == 0 {
				// Absence is not enough on its own — a pass that measured
				// nothing would satisfy it. The named skip line is the
				// positive witness that this close was seen and exempted.
				if !strings.Contains(out, "a-1") || !strings.Contains(out, "no verify bead: close reason is a rejection") {
					t.Errorf("the skip was silent — the pass must name it:\n%s", out)
				}
				if strings.Contains(calls, "create") {
					t.Errorf("a verify bead was filed for a rejected close:\n%s", calls)
				}
				if strings.Contains(calls, "comments add") {
					t.Errorf("a rejected close was commented on:\n%s", calls)
				}
			} else if !strings.Contains(calls, "--deps discovered-from:a-1") {
				t.Errorf("close_reason %q filed a bead that does not answer a-1:\n%s", tc.reason, calls)
			}
			// Either way the watermark passes the close: an exempt close is
			// decided, not deferred, and must not be re-examined every pass
			// forever.
			mark, ok := readVerifyWatermark(a.verifyWatermarkPath(repo))
			if want, _ := time.Parse(time.RFC3339, closed); !ok || !mark.Equal(want) {
				t.Errorf("watermark = %s (ok=%v), want the close %s", mark, ok, want)
			}
		})
	}
}

// The exemption does not consume a batch slot, and does not hold the batch.
// `verify_batch: 2` over one rejected close and two ordinary ones must file
// ONE bead answering the two — not hold the pair waiting for a third that
// the rejected close would never have been.
func TestVerifyAfterRejectedCloseDoesNotFillABatch(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	row := func(id, reason, at string) string {
		return `{"id":"` + id + `","title":"t ` + id + `","status":"closed","priority":1,` +
			`"assignee":"developer","labels":["code"],"closed_at":"` + at +
			`","close_reason":` + fmt.Sprintf("%q", reason) + `}`
	}
	list := "[" + strings.Join([]string{
		row("a-1", "fixed", "2026-08-18T09:20:06Z"),
		row("a-2", "duplicate of a-1", "2026-08-18T09:21:06Z"),
		row("a-3", "shipped", "2026-08-18T09:22:06Z"),
	}, ",") + "]"
	repo := vaRepo(t, a, list, "verify_batch: 2")
	vaGitRepo(t, repo, "chore: a commit that names no bead") // a-2 shipped nothing
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC))

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d, want 1 full batch of the two real closes\nout: %s\nerr: %s", n, out, errs)
	}
	calls := bdCalls(t, fake)
	if !strings.Contains(calls, "--deps discovered-from:a-1") || !strings.Contains(calls, "discovered-from:a-3") {
		t.Errorf("the batch does not answer both real closes:\n%s", calls)
	}
	if strings.Contains(calls, "comments add a-2") {
		t.Errorf("the rejected close joined the batch:\n%s", calls)
	}
	if strings.Contains(out, "held for a verify batch") {
		t.Errorf("the batch was held although two real closes were pending:\n%s", out)
	}
}

// ranger-base-5fyg. The exemption was strings.Contains over rejectWords, so
// a close describing a shipped fix in this shop's own vocabulary was read as
// a rejection and its QA session was not deferred but CANCELLED — the
// watermark advances past an exempted close, so no later pass recovers it.
//
// The rows are the bead's measured table. Mutation-checked, and the two
// halves of the fix do not cover the same rows:
//
//   - the first five are not the listed words at all ("dedupes" is not
//     "dup", "invalidation" is not "invalid"), AND they shipped. Either
//     half alone keeps them green — restoring strings.Contains does not
//     fail them, nor does dropping the commit trail.
//   - the sixth IS the word "duplicate", in a sentence describing a fix.
//     Only the commit trail reaches it: put the exemption back on the words
//     alone, boundary regex and all, and this row is the one that fails.
//     That is why the bead calls word matching a narrowing, not a fix.
//
// The last two rows are the controls the fix must not break: a real
// rejection that shipped nothing is still exempt (invert the trail test and
// it fails), and an ordinary close is still verified.
func TestVerifyAfterVerifiesACloseThatShippedDespiteRejectionWords(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		reason  string
		shipped bool // a commit names a-1
		want    int  // verify beads filed
	}{
		{"verify-after dedupes on the description marker bd commits with the bead", true, 1},
		{"Fixed: deduplicated the render", true, 1},
		{"Fixed: cache invalidation now keys on the sha", true, 1},
		{"Fixed: a config write invalidates the cached probe", true, 1},
		{"Fixed: removed the duplicated branch in Route()", true, 1},
		{"Fixed: the retry no longer files a duplicate bead", true, 1},
		// The controls. The first is the exemption still doing its job —
		// the same shape as row six with the commit trail taken away, so
		// the two together isolate the git half of the conjunction. The
		// second is an ordinary close, which no arm of this may touch.
		{"duplicate of a-9", false, 0},
		{"Fixed: the guard refuses", true, 1},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			b, fake := newTestBackend(t)
			a := b.App
			closed := "2026-08-18T09:20:06Z"
			repo := vaRepo(t, a, closedListReason("a-1", `["code"]`, closed, tc.reason))
			// The trail is what a real fix leaves behind, not a marker
			// planted for the test: the message is the sixth row's own
			// close reason with the bead id a commit here always carries.
			// The row without it still gets a repo and a commit, so the
			// difference between the arms is the trail and nothing else.
			if tc.shipped {
				vaGitRepo(t, repo, "fix: the retry no longer files a duplicate bead (a-1)")
			} else {
				vaGitRepo(t, repo, "chore: a commit that names no bead")
			}
			writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC))

			n, out, errs := vaRun(t, a, testBd(t))
			if n != tc.want {
				t.Fatalf("close_reason %q filed %d verify beads, want %d\nout: %s\nerr: %s",
					tc.reason, n, tc.want, out, errs)
			}
			calls := bdCalls(t, fake)
			if tc.want == 0 {
				if !strings.Contains(out, "no verify bead: close reason is a rejection") {
					t.Errorf("the rejection control stopped being exempt:\n%s", out)
				}
				if strings.Contains(calls, "create") {
					t.Errorf("a verify bead was filed for a rejected close:\n%s", calls)
				}
				return
			}
			if !strings.Contains(calls, "--deps discovered-from:a-1") {
				t.Errorf("close_reason %q filed a bead that does not answer a-1:\n%s", tc.reason, calls)
			}
			if strings.Contains(out, "no verify bead") {
				t.Errorf("the pass called a shipped close a rejection:\n%s", out)
			}
		})
	}
}

// The exemption's third arm: git could not answer. A beads repo that is not
// a checkout, or has no commits yet, makes "nothing shipped" unmeasurable —
// and an unmeasurable half must not silently satisfy the conjunction, or
// every reject word in that repo exempts again. Doubt files the bead.
func TestVerifyAfterFilesWhenGitCannotSayWhatACloseShipped(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	closed := "2026-08-18T09:20:06Z"
	// vaRepo's t.TempDir() is not a repo: `git log --grep` exits non-zero.
	repo := vaRepo(t, a, closedListReason("a-1", `["code"]`, closed, "duplicate of a-9"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC))

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d verify beads, want 1 — an unanswerable repo must not exempt\nout: %s\nerr: %s", n, out, errs)
	}
	if !strings.Contains(out, "git could not say what this close shipped") {
		t.Errorf("the pass did not name why it declined the exemption:\n%s", out)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "--deps discovered-from:a-1") {
		t.Errorf("the filed bead does not answer a-1:\n%s", calls)
	}
}

// ─── the findings bundle and the class (ADR 0006 §1/§3, amended 2026-09-02) ──
//
// Two rulings, one bead this rule mints: its TRAILER says file one findings
// bundle in the close's own lane labelled `debt` (not "a bug bead per close
// `-a <closer>`", which is what it said until ranger-base-ozzau), and the
// bead itself CARRIES the class of the close it verifies. What these pin is
// that both are exact — including the two negatives, which a
// missing-substring list cannot say: the closer's name is gone from the
// trailer, and a class the close did not carry is never manufactured.

// The trailer, single close: the RECORDED channel first — a finding whose
// fix runs nothing files no bead at all — then ONE findings bead for the
// LIVE ones, in the close's lane, `-l debt`, hung off THIS bead. And no
// closer named anywhere in it. The closer's name left the trailer with the
// §1 amendment of 2026-09-01 and the text kept it until now, so the absence
// is the whole point of the second half here: `-a developer` is not a
// substring the positive list above could have caught. The recorded channel
// is ADR 0006 §6 as amended 2026-09-06 (ranger-base-0f2zy): its pin is the
// sentence that says NO bead is filed, which is the half a trailer built
// only for filing cannot contain.
func TestVerifyTrailerFilesOneFindingsBundleAndNamesNoCloser(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "a-1", Title: "gate shell", Assignee: "developer",
		Labels: []string{"code"}, CloseReason: "fixed", ClosedAt: &closed}
	got := a.verifyDescription(t.TempDir(), is, verifyCloser(is))

	for _, want := range []string{
		// The recorded channel (ADR 0006 §6, amended 2026-09-06): the one
		// sentence that tells the verifier a finding files NOTHING.
		"the finding is RECORDED and NO bead is filed for it",
		"for LIVE findings only, file ONE findings bead `-l code -l debt --deps discovered-from:<this bead's id>`",
		"one line per finding: file:line · what fails · the bead it escaped from · the repro or failing test",
		"`-t bug` bead at P1/P2",
		"close this one `escape`",
		"No live findings, no bead.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("trailer missing %q:\n%s", want, got)
		}
	}
	// The recorded channel is stated FIRST (ADR 0006 §6, 2026-09-06
	// amendment): a trailer that says what to file before it says what not
	// to file has already asked for the bead.
	if strings.Index(got, "the finding is RECORDED") > strings.Index(got, "the finding is LIVE") {
		t.Errorf("trailer says LIVE before RECORDED:\n%s", got)
	}
	// The retired shape, in every spelling it ever had. A trailer that still
	// hands the fix to the closer by name is the defect this cut removed.
	for _, gone := range []string{"-a developer", "file a bug bead", "-l code -a"} {
		if strings.Contains(got, gone) {
			t.Errorf("trailer still carries the retired shape %q:\n%s", gone, got)
		}
	}
}

// The batch trailer says it ONCE for all N: the bundle is per VERIFY close,
// not per close verified, which is the amplification the ruling cut. Same
// two negatives.
func TestVerifyBatchTrailerSaysTheBundleOnceForEveryClose(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	group := []BdIssue{
		{ID: "a-1", Title: "one", Assignee: "developer", Labels: []string{"code"}, ClosedAt: &closed},
		{ID: "a-2", Title: "two", Assignee: "ops", Labels: []string{"code"}, ClosedAt: &closed},
		{ID: "a-3", Title: "three", Assignee: "developer", Labels: []string{"code"}, ClosedAt: &closed},
	}
	got := a.verifyGroupDescription(t.TempDir(), group)

	if n := strings.Count(got, "file ONE findings bead"); n != 1 {
		t.Errorf("batch trailer says the bundle %d times, want 1 (once for all N):\n%s", n, got)
	}
	if !strings.Contains(got, "`-l code -l debt --deps discovered-from:<this bead's id>`") {
		t.Errorf("batch trailer does not carry the bundle shape:\n%s", got)
	}
	for _, gone := range []string{"-a developer", "-a ops", "file a bug bead", "<that close's closer>"} {
		if strings.Contains(got, gone) {
			t.Errorf("batch trailer still carries the retired shape %q:\n%s", gone, got)
		}
	}
}

// The lane is the CLOSE's, not a compiled-in `code`: a `-l devops` close
// sends its findings to the devops lane, and a batch spanning both goes to
// `code` (ADR 0006 §1).
func TestVerifyTrailerLaneFollowsTheClose(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	devops := BdIssue{ID: "a-1", Title: "one", Labels: []string{"devops"}, ClosedAt: &closed}
	code := BdIssue{ID: "a-2", Title: "two", Labels: []string{"code"}, ClosedAt: &closed}

	if got := a.verifyDescription(t.TempDir(), devops, ""); !strings.Contains(got, "`-l devops -l debt") {
		t.Errorf("a -l devops close did not send its bundle to the devops lane:\n%s", got)
	}
	if got := a.verifyGroupDescription(t.TempDir(), []BdIssue{devops, code}); !strings.Contains(got, "`-l code -l debt") {
		t.Errorf("a batch spanning both lanes did not fall to code:\n%s", got)
	}
	// And an instance that verifies some other lane gets that lane, not a
	// name it does not have.
	vaRepo(t, a, "[]", "verify_labels: [infra]")
	infra := BdIssue{ID: "a-3", Title: "three", Labels: []string{"infra"}, ClosedAt: &closed}
	if got := a.verifyDescription(t.TempDir(), infra, ""); !strings.Contains(got, "`-l infra -l debt") {
		t.Errorf("a configured lane did not reach the trailer:\n%s", got)
	}
}

// The class, single close: the verify bead IS the class of the close it
// verifies. Type for feature and bug, the `debt` label for debt, and
// nothing at all for a close that carried no class — never a fourth
// "verify" bucket, and never a class manufactured here.
func TestVerifyBeadInheritsTheCloseClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		issueType string
		labels    []string
		wantTail  string // the argv tail after -p, where Create renders -t
		wantLabel string
	}{
		{"feature", "feature", []string{"code"}, "-p 1 -t feature", "-l qa "},
		{"bug", "bug", []string{"code"}, "-p 1 -t bug", "-l qa "},
		{"debt", "task", []string{"code", "debt"}, "-p 1", "-l qa,debt "},
		{"unclassified", "task", []string{"code"}, "-p 1", "-l qa "},
		{"type wins over the debt label", "bug", []string{"code", "debt"}, "-p 1 -t bug", "-l qa "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			a := b.App
			t0 := time.Now().Add(-3 * time.Hour)
			repo := vaRepo(t, a, vaList(vaClosedClassed("a-1", t0, 1, tc.issueType, tc.labels...)))
			writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Hour))

			if n, _, errs := vaRun(t, a, testBd(t)); n != 1 {
				t.Fatalf("filed %d, want 1 (stderr: %s)", n, errs)
			}
			calls := bdCalls(t, fake)
			// Anchored on -p, which Create renders immediately before -t:
			// the description itself quotes `-t bug` for the persona to
			// type, so a bare substring search proves nothing about argv.
			if !strings.Contains(calls, tc.wantTail) {
				t.Errorf("create argv tail is not %q:\n%s", tc.wantTail, calls)
			}
			if !strings.Contains(calls, tc.wantLabel) {
				t.Errorf("create argv labels are not %q:\n%s", tc.wantLabel, calls)
			}
			if tc.wantTail == "-p 1" && strings.Contains(calls, "-p 1 -t") {
				t.Errorf("an unclassified close manufactured a class:\n%s", calls)
			}
		})
	}
}

// The class, batched: the batch takes its MOST URGENT class in the order
// bug › feature › debt › unclassified — the same loop that picks the
// batch's priority, and deliberately not BeadClasses, which is the pulse
// line's reporting order (beads.go).
func TestVerifyBatchTakesTheMostUrgentClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		types    []string
		wantTail string
	}{
		{"bug beats feature and debt", []string{"task", "feature", "bug"}, "-p 1 -t bug"},
		{"feature beats debt and unclassified", []string{"task", "task", "feature"}, "-p 1 -t feature"},
		{"debt beats unclassified", []string{"task", "task", "task"}, "-p 1"},
		{"all unclassified stays unclassified", []string{"chore", "task", "epic"}, "-p 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			a := b.App
			t0 := time.Now().Add(-3 * time.Hour)
			var beads []string
			for i, ty := range tc.types {
				labels := []string{"code"}
				// The debt arm: the FIRST close carries the label, so a
				// batch that ends unclassified and one that ends debt
				// differ by exactly the field under test.
				if i == 0 && tc.name == "debt beats unclassified" {
					labels = append(labels, "debt")
				}
				beads = append(beads, vaClosedClassed(fmt.Sprintf("a-%d", i+1), t0.Add(time.Duration(i)*time.Minute), 1, ty, labels...))
			}
			repo := vaRepo(t, a, vaList(beads...), "verify_batch: 3")
			writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Hour))

			if n, _, errs := vaRun(t, a, testBd(t)); n != 1 {
				t.Fatalf("filed %d, want 1 (stderr: %s)", n, errs)
			}
			calls := bdCalls(t, fake)
			if !strings.Contains(calls, tc.wantTail) {
				t.Errorf("batch class argv tail is not %q:\n%s", tc.wantTail, calls)
			}
			switch tc.name {
			case "debt beats unclassified":
				if !strings.Contains(calls, "-l qa,debt ") {
					t.Errorf("a batch holding one debt close did not take debt:\n%s", calls)
				}
			case "all unclassified stays unclassified":
				if strings.Contains(calls, "-p 1 -t") || strings.Contains(calls, "-l qa,debt") {
					t.Errorf("an all-unclassified batch manufactured a class:\n%s", calls)
				}
			}
		})
	}
}

// BdNew.Type is bd's own `-t`, and it is passed ONLY when set: an empty
// Type leaves bd's default (`task`), which with no `debt` label is the
// unclassified bucket ADR 0006 §1 reports rather than guesses at.
func TestBdCreatePassesTypeOnlyWhenSet(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := testBd(t)
	dir := t.TempDir()

	if _, err := bd.Create(dir, BdNew{Title: "typed", Type: "feature", Actor: "posse"}); err != nil {
		t.Fatal(err)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create typed --json -t feature") {
		t.Errorf("Type did not reach argv as -t:\n%s", calls)
	}
	os.Remove(filepath.Join(fakeDirOf(t), "bd-calls.log"))
	if _, err := bd.Create(dir, BdNew{Title: "untyped", Actor: "posse"}); err != nil {
		t.Fatal(err)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "-t ") {
		t.Errorf("an unset Type passed -t anyway:\n%s", calls)
	}
}

// The strand a `git log --grep` trail cannot see (ranger-base-hl0sp, found
// by the sweep on ranger-base-2dzsm).
//
// The instance, verbatim: ranger-base-5jdzh's verify bead carried
// "commits (git log --grep ranger-base-5jdzh): d309e2b ...", and d309e2b is
// ranger-base-wd4be's commit — it merely NAMES 5jdzh in its message. 5jdzh's
// own work was on a session branch whose tip was not an ancestor of main,
// and nothing in the bead said so, so the verifier read the trail as proof
// the close had landed and unblocked two beads on a premise main did not
// carry.
//
// Both arms run against the same repo shape and differ only in whether the
// session branch was merged, which is what makes this a measurement rather
// than a restatement: the trail line is IDENTICAL in both (the foreign
// commit is on main either way), so a landing block that always printed the
// same sentence — or that read the trail instead of the branch record —
// fails the arm it does not describe. The strand arm additionally pins that
// the foreign commit is still listed: this is a line BESIDE the trail, and
// suppressing the trail would hide evidence the verifier wants.
func TestVerifySectionSaysWhetherTheSessionBranchReachedItsBase(t *testing.T) {
	t.Parallel()
	closed := time.Date(2026, 9, 3, 9, 20, 6, 0, time.UTC)
	// A ROLE, not a crew name: ADR 0012 App.A 5 keeps this instance's
	// personas out of the shipped tree, and a session branch is exactly the
	// place a real name wants to be pasted in from the incident.
	const branch = "posse/developer-posse-a-1"
	for _, tc := range []struct {
		name       string
		merge      bool
		want, deny string
	}{
		{"stranded", false, "has NOT reached main", "has reached main"},
		{"landed", true, "has reached main", "has NOT reached main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			repo := t.TempDir()
			// main carries a FOREIGN commit that names a-1 — another bead's
			// work citing this one, which is all --grep can see.
			vaGitRepo(t, repo, "qa: park the flake (a-2, red from a-1)")
			qblGit(t, repo, "checkout", "-q", "-b", branch)
			if err := os.WriteFile(filepath.Join(repo, "shipped.txt"), []byte("the deliverable"), 0o644); err != nil {
				t.Fatal(err)
			}
			qblGit(t, repo, "add", "-A")
			qblGit(t, repo, "commit", "-q", "-m", "feat: the thing a-1 was cut for (a-1)")
			qblGit(t, repo, "config", "branch."+branch+".posseBead", "a-1")
			qblGit(t, repo, "config", "branch."+branch+".posseBase", "main")
			qblGit(t, repo, "checkout", "-q", "main")
			if tc.merge {
				qblGit(t, repo, "merge", "-q", "--no-ff", "-m", "merge "+branch, branch)
			}

			is := BdIssue{ID: "a-1", Title: "t", Assignee: "developer", CloseReason: "DONE",
				Labels: []string{"code"}, ClosedAt: &closed}
			sec := b.App.verifySection(repo, is, verifyCloser(is))
			if !strings.Contains(sec, "- session branches cut for a-1 (branch.<b>.posseBead):") {
				t.Fatalf("no landing block — the branch record was never read:\n%s", sec)
			}
			if !strings.Contains(sec, branch+" tip ") {
				t.Errorf("the landing block never named the branch:\n%s", sec)
			}
			if !strings.Contains(sec, tc.want) {
				t.Errorf("landing block does not say %q:\n%s", tc.want, sec)
			}
			if strings.Contains(sec, tc.deny) {
				t.Errorf("landing block says %q for the %s arm:\n%s", tc.deny, tc.name, sec)
			}
			// Beside, never instead of: the foreign commit stays listed in
			// both arms, and its presence is exactly why the block is needed.
			if !strings.Contains(sec, "- commits naming a-1 (git log --grep") ||
				!strings.Contains(sec, "qa: park the flake (a-2, red from a-1)") {
				t.Errorf("the commit trail was lost — the landing block replaced evidence instead of adding to it:\n%s", sec)
			}
		})
	}
}

// No branch record, no verdict (ranger-base-hl0sp). `git branch -d` takes
// the branch's config with it, so a session that landed and was tidied up is
// byte-identical from git to a branch cut before the stamp existed and to a
// close with no session at all. Printing "not landed" for any of those would
// be the same defect this block was built to fix, pointed the other way.
func TestVerifySectionSaysNothingAboutLandingWithoutABranchRecord(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := t.TempDir()
	vaGitRepo(t, repo, "feat: the thing (a-1)")
	closed := time.Date(2026, 9, 3, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "a-1", Title: "t", CloseReason: "DONE", ClosedAt: &closed}
	sec := b.App.verifySection(repo, is, verifyCloser(is))
	if strings.Contains(sec, "session branches cut for") {
		t.Errorf("a landing block appeared with no branch record behind it:\n%s", sec)
	}
	// The control: the section IS being built and the trail did run, so the
	// absence above is the record's, not an empty section's.
	if !strings.Contains(sec, "- commits naming a-1 (git log --grep") {
		t.Fatalf("the section carried no trail either — this test pins nothing:\n%s", sec)
	}
}

// ─── verify_chain_max: N — the gate's DEPTH (ranger-base-ewnas) ───────────────
//
// `verify_batch:` above divides one round's FAN-OUT. Nothing divided the
// number of ROUNDS: verify files a bead, the verifier files findings from it,
// the fix closes, and verify files a bead about THAT close. Every round is
// locally reasonable and the loop is not — ~20 rounds on a single config-test
// pin on another instance's project crew, stopped by a human.
//
// The bound counts rounds on ONE LINEAGE (verifyChainRounds: this close was
// filed `discovered-from` a verify bead, which answered the close before it)
// and hands the next one to the operator instead of to a verifier. What these
// pin is that the count is the graph's, that the stop files no verify bead at
// all, and that neither a batched hop nor a bd that will not answer can turn
// an undercount into an escalation.

// vaQaBead is a verify bead for a canned `bd list --all` answer: the `qa`
// label, and one per-close marker line per close it answers — the two fields
// the walk reads off a parent (verifySourceIDs). Closed long before any
// watermark these tests set, because a verify bead is never a candidate and
// must not move `newest` either.
func vaQaBead(id string, sources ...string) string {
	var d strings.Builder
	for _, s := range sources {
		fmt.Fprintf(&d, "%s%s%stitle, quoted as data: %q).\n", verifyMarkerPrefix, s, verifyMarkerAfter, "gate shell "+s)
	}
	return fmt.Sprintf(`{"id":%q,"title":%q,"status":"closed","priority":1,"labels":["qa"],`+
		`"description":%s,"closed_at":%q,"close_reason":"escape"}`,
		id, "verify: gate shell "+strings.Join(sources, ", "), mustJSON(d.String()),
		time.Now().Add(-240*time.Hour).Format(time.RFC3339Nano))
}

func mustJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// vaChainEdge makes `bd dep list <child>` answer with one discovered-from
// parent. It is the per-ISSUE fixture the fake serves (bdFakeDepSubject):
// the per-repo file answers every id alike, which would make hop 2 of a walk
// a repeat of hop 1 rather than the parent's own parent.
func vaChainEdge(t *testing.T, repo, child, parent, depType string) {
	t.Helper()
	body := fmt.Sprintf(`[{"id":%q,"dependency_type":%q}]`, parent, depType)
	if err := os.WriteFile(filepath.Join(repo, "fake-deps."+child+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// vaChain builds the lineage a-0 ← v-1 ← a-1 ← v-2 ← … of `rounds` verify
// rounds, and returns the listing entries plus the id of the newest close —
// the one the sweep is about, whose chain is `rounds` deep. The verify beads
// are v-N so nothing in a fixture collides with the fake's own created ids
// (q-1, q-2, …).
func vaChain(t *testing.T, repo string, rounds int, t0 time.Time) ([]string, string) {
	t.Helper()
	var list []string
	// The chain's older closes are closed BEFORE the watermark these tests
	// set, so the sweep has exactly one candidate and the assertions name
	// one bead.
	for i := 0; i < rounds; i++ {
		list = append(list, vaClosed(fmt.Sprintf("a-%d", i), t0.Add(-time.Duration(rounds-i)*time.Hour), 1))
		list = append(list, vaQaBead(fmt.Sprintf("v-%d", i+1), fmt.Sprintf("a-%d", i)))
		vaChainEdge(t, repo, fmt.Sprintf("a-%d", i+1), fmt.Sprintf("v-%d", i+1), "discovered-from")
	}
	leaf := fmt.Sprintf("a-%d", rounds)
	list = append(list, vaClosed(leaf, t0, 2))
	return list, leaf
}

// vaChainRepo is vaRepo over vaChain's listing: `rounds` verify rounds, one
// fresh close on the end, and the watermark just behind that close.
func vaChainRepo(t *testing.T, a *App, rounds int, cfg ...string) (string, string, time.Time) {
	t.Helper()
	t0 := time.Now().Add(-3 * time.Hour)
	repo := vaRepo(t, a, "[]", cfg...)
	list, leaf := vaChain(t, repo, rounds, t0)
	if err := os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Minute))
	return repo, leaf, t0
}

// THE BOUND. A close whose lineage has already spent `verify_chain_max`
// rounds earns no verify bead at all: one `-l question` bead for the
// operator, carrying the chain, and the cycle ends there — no verify bead,
// no verifier session, no findings bundle, no next close.
func TestVerifyChainBoundFilesTheOperatorsQuestionInsteadOfAVerifyBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo, leaf, _ := vaChainRepo(t, a, 3, "verify_chain_max: 3", "operator: boss")
	_ = repo

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Fatalf("filed %d verify beads for a close at the bound, want 0 (stderr: %s)", n, errs)
	}
	calls := bdCalls(t, fake)
	if strings.Contains(calls, "create verify: gate shell "+leaf) {
		t.Fatalf("the bound filed a verify bead anyway:\n%s", calls)
	}
	for _, want := range []string{
		"create " + verifyChainStuckPrefix + leaf + " — 3 verify rounds on one lineage",
		"-l question",
		"-a boss",
		"--deps discovered-from:" + leaf,
		"-p 2", // the escalation is as urgent as the close it replaces
		"comments add " + leaf + " verify chain stopped: q-1",
		// the comment names the count and the BOUND, which are two
		// different numbers whenever a chain overshoots
		"already spent 3 verify round(s) and `verify_chain_max:` is 3",
		// the chain itself, newest first, so the operator can read the loop
		// without walking the graph by hand
		"round 3  verify v-3 of close a-2",
		"round 1  verify v-1 of close a-0",
		discoveredFromMarkerPrefix + leaf,
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("bd calls missing %q:\n%s", want, calls)
		}
	}
	if !strings.Contains(out, "no verify bead: 3 verify round(s) on one lineage is the bound") {
		t.Errorf("the pass did not say what it did:\n%s", out)
	}
	// The escalation IS the handling: the watermark walked past the close,
	// so the next pass neither re-files nor re-escalates.
	if n, _, _ := vaRun(t, a, testBd(t)); n != 0 {
		t.Errorf("second sweep filed %d", n)
	}
	if got := strings.Count(bdCalls(t, fake), "create "+verifyChainStuckPrefix); got != 1 {
		t.Errorf("escalated %d times, want 1:\n%s", got, bdCalls(t, fake))
	}
}

// Below the bound the rule is unchanged, and the verify bead carries the one
// thing the harness can tell a verifier that it could not before: which
// round this is. The last round says so in terms — the harness can only stop
// the round AFTER next, and a verifier who knows it is on the last one can
// stop this one.
func TestVerifyChainBelowTheBoundFilesTheBeadAndNamesTheRound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		rounds int
		want   []string
		absent []string
	}{{
		name: "first round renders nothing at all", rounds: 0,
		absent: []string{"VERIFY ROUND"},
	}, {
		name: "a middle round names itself", rounds: 1,
		want:   []string{"VERIFY ROUND 2 of 3", "round 1: verify v-1 of a-0"},
		absent: []string{"THIS IS THE LAST ROUND"},
	}, {
		name: "the last round says it is the last", rounds: 2,
		want: []string{"VERIFY ROUND 3 of 3", "THIS IS THE LAST ROUND", "round 2: verify v-2 of a-1"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			a := b.App
			_, leaf, _ := vaChainRepo(t, a, tc.rounds, "verify_chain_max: 3")

			n, _, errs := vaRun(t, a, testBd(t))
			if n != 1 {
				t.Fatalf("filed %d verify beads at %d round(s) under a bound of 3, want 1 (stderr: %s)", n, tc.rounds, errs)
			}
			calls := bdCalls(t, fake)
			if !strings.Contains(calls, "create verify: gate shell "+leaf) {
				t.Fatalf("the per-close verify bead was not filed:\n%s", calls)
			}
			// The head line is a PREFIX, so the marker the dedupe reads is
			// still on its own line below it.
			if !strings.Contains(calls, verifyMarkerPrefix+leaf+verifyMarkerAfter) {
				t.Errorf("the head line displaced the dedupe marker:\n%s", calls)
			}
			for _, w := range tc.want {
				if !strings.Contains(calls, w) {
					t.Errorf("verify bead missing %q:\n%s", w, calls)
				}
			}
			for _, w := range tc.absent {
				if strings.Contains(calls, w) {
					t.Errorf("verify bead carries %q and should not:\n%s", w, calls)
				}
			}
		})
	}
}

// A BATCHED verify bead answers N closes and the findings bundle names which
// in prose. The walk cannot read prose — so it counts the round and
// CONTINUES through the shallowest of the N (amended 2026-11-09,
// ranger-base-rffxq; it used to stop here, which is the whole of why the
// bound could not fire above `verify_batch: 1` — ranger-base-bu8rz). Every
// one of the N is a close the bundle could have answered, so the smallest of
// their counts is a lower bound on the true one, and short is the direction
// this file errs in.
//
// The lineage here is two rounds of which NEITHER hop is single-close, and
// the bound is 2: the old walk counted one round and filed a verify bead,
// this one counts two and escalates. That is the pin — the fixture fails on
// the previous code rather than merely reading differently.
func TestVerifyChainContinuesThroughABatchedVerifyBeadsShallowestSource(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-3 * time.Hour)
	repo := vaRepo(t, a, "[]", "verify_chain_max: 2", "operator: boss")
	// Two batched rounds above a-2, each verify bead answering two closes:
	//
	//	a-0 ← v-1{a-0, b-0} ← a-1 ┐
	//	b-0 ← v-1{a-0, b-0} ← b-1 ┴ v-2{a-1, b-1} ← a-2
	//
	// a-1 and b-1 have each spent one round, so a-2 has spent two.
	list := []string{
		vaClosed("a-0", t0.Add(-2*time.Hour), 1),
		vaClosed("b-0", t0.Add(-2*time.Hour), 1),
		vaQaBead("v-1", "a-0", "b-0"),
		vaClosed("a-1", t0.Add(-time.Hour), 1),
		vaClosed("b-1", t0.Add(-time.Hour), 1),
		vaQaBead("v-2", "a-1", "b-1"),
		vaClosed("a-2", t0, 2),
	}
	vaChainEdge(t, repo, "a-1", "v-1", "discovered-from")
	vaChainEdge(t, repo, "b-1", "v-1", "discovered-from")
	vaChainEdge(t, repo, "a-2", "v-2", "discovered-from")
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Minute))

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Fatalf("filed %d verify beads for a close two BATCHED rounds deep under a bound of 2, want 0 — the walk stopped at the batch (stderr: %s)", n, errs)
	}
	calls := bdCalls(t, fake)
	for _, want := range []string{
		"create " + verifyChainStuckPrefix + "a-2 — 2 verify rounds on one lineage",
		"-l question",
		// both hops, newest first, each naming the batch it continued
		// through and that the count below it is a floor
		"round 2  verify v-2 of close a-1",
		"one of the 2 closes that verify bead answers",
		"round 1  verify v-1 of close a-0",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("the escalation is missing %q:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "create verify: gate shell a-2") {
		t.Errorf("a verify bead was filed for a close at the bound:\n%s", calls)
	}
	if !strings.Contains(out, "no verify bead: 2 verify round(s) on one lineage is the bound") {
		t.Errorf("the pass did not say what it did:\n%s", out)
	}
	// No bead's edges are read twice in a pass, which is what keeps the
	// branching walk's bd bill a function of the graph and not of how many
	// candidates run through it.
	if got := strings.Count(calls, "dep list a-1 --json\n"); got != 1 {
		t.Errorf("read a-1's edges %d times, want 1:\n%s", got, calls)
	}
}

// A walk bd will not finish gives a FLOOR, and a floor is not evidence: the
// verify bead is filed and the pass says the count is short. (The floor can
// never reach the bound — verifyChainRounds counts one hop per answered
// call — so this is the only thing a failed walk can do.)
func TestVerifyChainWalkFailureFilesTheBeadAndNamesTheFloor(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo, leaf, _ := vaChainRepo(t, a, 3, "verify_chain_max: 3")
	// `dep list <leaf>` answers unparseable JSON: the DOWN read fails while
	// the `--direction=up` read the dedupe makes still works, which is what
	// isolates the walk.
	if err := os.WriteFile(filepath.Join(repo, "fake-deps."+leaf+".json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d verify beads when the chain walk failed, want 1 (stderr: %s)", n, errs)
	}
	if !strings.Contains(errs, "could not finish the verify-chain walk — 0 round(s) counted, which is a floor") {
		t.Errorf("the short count was not named:\n%s", errs)
	}
	if strings.Contains(bdCalls(t, fake), "create "+verifyChainStuckPrefix) {
		t.Errorf("a failed walk escalated:\n%s", bdCalls(t, fake))
	}
}

// Only the handoff edge counts, and only to a verify bead. A close filed
// `discovered-from` another CODE bead is a handoff (ADR 0006 §1), not a
// verify round, and a `blocks` parent is not a bead somebody filed from this
// one at all.
func TestVerifyChainCountsOnlyDiscoveredFromAVerifyBead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, parent, depType string }{
		{"a handoff from a code bead", "a-0", "discovered-from"},
		{"a blocks edge to a verify bead", "v-1", "blocks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			a := b.App
			t0 := time.Now().Add(-3 * time.Hour)
			repo := vaRepo(t, a, "[]", "verify_chain_max: 1")
			list := []string{
				vaClosed("a-0", t0.Add(-time.Hour), 1),
				vaQaBead("v-1", "a-0"),
				vaClosed("a-1", t0, 1),
			}
			vaChainEdge(t, repo, "a-1", tc.parent, tc.depType)
			os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644)
			writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Minute))

			n, _, errs := vaRun(t, a, testBd(t))
			if n != 1 {
				t.Fatalf("filed %d verify beads, want 1 — this edge was counted as a verify round (stderr: %s)", n, errs)
			}
			if calls := bdCalls(t, fake); strings.Contains(calls, "VERIFY ROUND") {
				t.Errorf("the edge was counted as a round:\n%s", calls)
			}
		})
	}
}

// The dedupe, and it is the TITLE — written by bd in the same breath as the
// issue, so an orphaned create (the muoo class: issue committed, `--deps`
// lost, no id printed) still dedupes. It counts CLOSED escalations too: the
// subject is a close that has already happened and nothing will re-examine
// it, so an answered escalation must not re-arm into a second one.
func TestVerifyChainEscalationDedupesOnItsTitleIncludingClosed(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-3 * time.Hour)
	repo := vaRepo(t, a, "[]", "verify_chain_max: 1")
	list, leaf := vaChain(t, repo, 1, t0)
	list = append(list, fmt.Sprintf(`{"id":"q-old","title":%s,"status":"closed","priority":1,`+
		`"labels":["question"],"closed_at":%q,"close_reason":"answered: one more round, filed by hand"}`,
		mustJSON(verifyChainStuckTitle(leaf, 1)), t0.Format(time.RFC3339Nano)))
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Minute))

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Fatalf("filed %d verify beads for an already-escalated close, want 0 (stderr: %s)", n, errs)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create ") {
		t.Errorf("an answered escalation re-armed:\n%s", calls)
	}
}

// `verify_chain_max: 0` turns the bound off — the `verify_labels:`-empty
// convention — and then it costs nothing: not one `bd dep list` is made for
// the chain, which is the only call this feature adds to a pass.
func TestVerifyChainMaxZeroTurnsTheBoundOffAndWalksNothing(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	_, leaf, _ := vaChainRepo(t, a, 3, "verify_chain_max: 0")

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d verify beads with the bound off, want 1 (stderr: %s)", n, errs)
	}
	calls := bdCalls(t, fake)
	if strings.Contains(calls, "dep list "+leaf+" --json\n") {
		t.Errorf("the bound is off and the walk ran anyway:\n%s", calls)
	}
	// The control: the dedupe's own read DID happen, so the absence above is
	// the walk's and not an unexercised pass.
	if !strings.Contains(calls, "dep list "+leaf+" --json --direction=up") {
		t.Fatalf("no dependents read at all — this test pins nothing:\n%s", calls)
	}
	if strings.Contains(calls, "VERIFY ROUND") {
		t.Errorf("the bound is off and the round line was rendered:\n%s", calls)
	}
}

// vaBatchedChain builds a lineage `rounds` deep whose EVERY hop is batched
// `width` ways — the shape a queue running `verify_batch: 4` actually files,
// and the shape no walk of this graph could count past one round until
// ranger-base-rffxq. Level i holds `width` closes, one verify bead answers
// all of them, and every close of level i+1 was filed from that bead; the
// leaf on top is the one fresh close, so the sweep has exactly one candidate.
//
// Every close of level i has spent i rounds, so the leaf has spent `rounds`
// and the minimum over any batch is exact rather than merely a floor.
func vaBatchedChain(t *testing.T, repo string, rounds, width int, t0 time.Time) ([]string, string) {
	t.Helper()
	level := func(i int) []string {
		out := make([]string, 0, width)
		for w := 0; w < width; w++ {
			out = append(out, fmt.Sprintf("a%d-%d", i, w))
		}
		return out
	}
	var list []string
	for i := 0; i < rounds; i++ {
		for _, id := range level(i) {
			// All of them before the watermark the caller sets, so none is
			// a candidate and the assertions name one bead.
			list = append(list, vaClosed(id, t0.Add(-time.Duration(rounds-i+1)*time.Hour), 1))
			if i > 0 {
				vaChainEdge(t, repo, id, fmt.Sprintf("v-%d", i), "discovered-from")
			}
		}
		list = append(list, vaQaBead(fmt.Sprintf("v-%d", i+1), level(i)...))
	}
	leaf := "leaf"
	list = append(list, vaClosed(leaf, t0, 2))
	vaChainEdge(t, repo, leaf, fmt.Sprintf("v-%d", rounds), "discovered-from")
	return list, leaf
}

// vaWalkReads is every bead the chain walk asked bd about in a pass, in
// order: a `dep list <id> --json` that is not the dedupe's `--direction=up`
// read.
func vaWalkReads(calls string) []string {
	var out []string
	for _, l := range strings.Split(calls, "\n") {
		if strings.Contains(l, "--direction=up") {
			continue
		}
		// The line is the whole argv, so the verb is not at a fixed index
		// (bdGlobalFlags go in front of it).
		f := strings.Fields(l)
		for i := 0; i+2 < len(f); i++ {
			if f[i] == "dep" && f[i+1] == "list" {
				out = append(out, f[i+2])
				break
			}
		}
	}
	return out
}

// THE BOUND BINDS AT THE CONFIGURED BATCH SIZE (ranger-base-fswbr, building
// ADR 0006 §3's 2026-11-09 amendment; this is ranger-base-bu8rz's own repro,
// which that bead left for architecture rather than inventing the mechanism).
//
// What the first subtest pins BY EXECUTION is the defect bu8rz measured: a
// lineage six rounds deep, every hop of it batched four ways, under
// `verify_chain_max: 3`. The escalation fires. Under the walk before this
// every one of those hops ended the count at one round, so the bound could
// not fire at any depth and `VERIFY ROUND 2 of 3` was the head line on a
// lineage twelve rounds deep. It is the shape this instance's queue files —
// 345 of 671 verify beads on the measured store answer four closes.
//
// Both controls carry their own weight. The first says the bound going
// reachable did not cost coverage: a one-round lineage at the same batch
// size still earns its verify bead and no escalation. The second is the
// MINIMUM, and it is the only subtest that tells the shipped rule from the
// repair bu8rz measured and rejected — a batch that MIXES depths (one source
// no rounds deep, one two rounds deep, which is what every batch filed
// before the grouping looks like) counts 1 + the shallowest, so it reads 1
// and files a verify bead. Taking the deepest instead would read 3 and
// escalate it, and taking the first marker would depend on the order bd
// happens to list a description's lines in.
func TestQAVerifyChainBoundBindsAtTheConfiguredBatchSize(t *testing.T) {
	t.Parallel()

	t.Run("six batched rounds reach a bound of three", func(t *testing.T) {
		t.Parallel()
		b, fake := newTestBackend(t)
		a := b.App
		t0 := time.Now().Add(-3 * time.Hour)
		// `verify_batch_age: 1m` only matters to the controls — an
		// escalation is decided before the batching — but the three
		// subtests then differ in their FIXTURE and in nothing else.
		repo := vaRepo(t, a, "[]", "verify_chain_max: 3", "verify_batch: 4",
			"verify_batch_age: 1m", "operator: boss")
		list, leaf := vaBatchedChain(t, repo, 6, 4, t0)
		os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644)
		writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Minute))

		n, out, errs := vaRun(t, a, testBd(t))
		if n != 0 {
			t.Fatalf(`filed %d verify beads for a close six BATCHED rounds deep under verify_chain_max: 3, want 0.
This is ranger-base-bu8rz: the walk counts one round at a batched hop and
stops, so the bound cannot fire at the batch size this queue runs.
stderr: %s`, n, errs)
		}
		calls := bdCalls(t, fake)
		for _, want := range []string{
			"create " + verifyChainStuckPrefix + leaf + " — 3 verify rounds on one lineage",
			"-l question",
			"-a boss",
			"--deps discovered-from:" + leaf,
			"comments add " + leaf + " verify chain stopped: q-1",
			// the chain, newest first, each hop naming the batch it
			// continued through so the operator can read the loop
			"round 3  verify v-6 of close a5-0",
			"one of the 4 closes that verify bead answers",
			"round 1  verify v-4 of close a3-0",
		} {
			if !strings.Contains(calls, want) {
				t.Errorf("the escalation is missing %q:\n%s", want, calls)
			}
		}
		if strings.Contains(calls, "create verify: gate shell "+leaf) {
			t.Errorf("a verify bead was filed for a close at the bound:\n%s", calls)
		}
		if !strings.Contains(out, "no verify bead: 3 verify round(s) on one lineage is the bound") {
			t.Errorf("the pass did not say what it did:\n%s", out)
		}
		// THE COST, which is the other half of making a branching walk
		// safe to run under the launcher lock: at this shape ADR 0006 §3
		// prices the worst case at 1 + 4 + 16 = 21 dependency reads, and
		// the pass's own cache turns that into one read per BEAD — the
		// leaf, four closes at level 5, four at level 4.
		reads := vaWalkReads(calls)
		if len(reads) > verifyChainReadMax {
			t.Errorf("the chain walk made %d dependency reads, over the %d cap:\n%v", len(reads), verifyChainReadMax, reads)
		}
		seen := map[string]bool{}
		for _, id := range reads {
			if seen[id] {
				t.Errorf("read %s's edges twice in one pass — the walk's cache is not holding:\n%v", id, reads)
			}
			seen[id] = true
		}
		if len(reads) != 9 {
			t.Errorf("the walk read %d beads, want 9 (the leaf, level 5's four, level 4's four): %v", len(reads), reads)
		}
	})

	t.Run("CONTROL: one round still earns its verify bead at verify_batch: 4", func(t *testing.T) {
		t.Parallel()
		b, fake := newTestBackend(t)
		a := b.App
		_, leaf, _ := vaChainRepo(t, a, 1, "verify_chain_max: 3", "verify_batch: 4",
			"verify_batch_age: 1m", "operator: boss")

		n, _, errs := vaRun(t, a, testBd(t))
		if n != 1 {
			t.Fatalf("filed %d verify beads for a one-round lineage at verify_batch: 4, want 1 — the reachable bound cost the coverage it was meant to leave alone (stderr: %s)", n, errs)
		}
		calls := bdCalls(t, fake)
		if !strings.Contains(calls, "create verify: gate shell "+leaf) {
			t.Fatalf("no verify bead for %s:\n%s", leaf, calls)
		}
		if !strings.Contains(calls, "VERIFY ROUND 2 of 3") {
			t.Errorf("the verify bead does not say which round it is:\n%s", calls)
		}
		if strings.Contains(calls, "create "+verifyChainStuckPrefix) {
			t.Errorf("a one-round lineage escalated:\n%s", calls)
		}
	})

	t.Run("CONTROL: a mixed legacy batch counts its shallowest source", func(t *testing.T) {
		t.Parallel()
		b, fake := newTestBackend(t)
		a := b.App
		t0 := time.Now().Add(-3 * time.Hour)
		repo := vaRepo(t, a, "[]", "verify_chain_max: 3", "verify_batch: 4",
			"verify_batch_age: 1m", "operator: boss")
		// v-mix answers two closes of DIFFERENT depth, which is what 263 of
		// the measured store's 351 batched verify beads look like:
		//
		//	y-0 ← v-y1{y-0} ← y-1 ← v-y2{y-1} ← y-2 ┐
		//	x-0 (no rounds at all)                  ┴ v-mix{y-2, x-0} ← c-1
		//
		// The deep source is named FIRST on purpose: a walk that took the
		// first marker, or the deepest, would read 3 here and escalate.
		list := []string{
			vaClosed("x-0", t0.Add(-3*time.Hour), 1),
			vaClosed("y-0", t0.Add(-3*time.Hour), 1),
			vaQaBead("v-y1", "y-0"),
			vaClosed("y-1", t0.Add(-2*time.Hour), 1),
			vaQaBead("v-y2", "y-1"),
			vaClosed("y-2", t0.Add(-time.Hour), 1),
			vaQaBead("v-mix", "y-2", "x-0"),
			vaClosed("c-1", t0, 1),
		}
		vaChainEdge(t, repo, "y-1", "v-y1", "discovered-from")
		vaChainEdge(t, repo, "y-2", "v-y2", "discovered-from")
		vaChainEdge(t, repo, "c-1", "v-mix", "discovered-from")
		os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644)
		writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Minute))

		n, _, errs := vaRun(t, a, testBd(t))
		if n != 1 {
			t.Fatalf("filed %d verify beads for a close whose batch's shallowest source has spent no rounds, want 1 — a mixed batch read deeper than its minimum (stderr: %s)", n, errs)
		}
		calls := bdCalls(t, fake)
		for _, want := range []string{
			"create verify: gate shell c-1",
			"VERIFY ROUND 2 of 3",
			"round 1: verify v-mix of x-0 (one of 2)",
		} {
			if !strings.Contains(calls, want) {
				t.Errorf("the verify bead is missing %q:\n%s", want, calls)
			}
		}
		if strings.Contains(calls, "create "+verifyChainStuckPrefix) {
			t.Errorf("a floor of one round escalated under a bound of 3:\n%s", calls)
		}
		// The control's own control: the deep branch really was walked, so
		// the minimum is a comparison and not a first-marker pick.
		read := strings.Join(vaWalkReads(calls), " ")
		for _, want := range []string{"y-2", "x-0"} {
			if !strings.Contains(read, want) {
				t.Errorf("the walk never read %s's edges, so nothing here compared two depths: %s", want, read)
			}
		}
	})
}

// THE GROUPING, which is the bound's other half and useless without it (ADR
// 0006 §3, amended 2026-11-09 by ranger-base-rffxq; verifyRoundGroups has the
// measurement). A batch is filed LIKE WITH LIKE: the pending closes are
// grouped by the rounds their lineages have already spent, and `verify_batch:`
// applies inside each group.
//
// Without it the shallowest-source walk is safe and still useless, because a
// batch that mixes depths reads the shallowest member's count for every close
// filed from it — 263 of the measured store's 351 batched verify beads mix,
// so a bound of 3 still never fires. With it every source in a batch has
// spent the same rounds, so the minimum is exact and the count climbs.
//
// The round-1 close sits in the MIDDLE of the close order on purpose. Put it
// last and close order alone would batch the four round-0 closes and hold it,
// so the fixture would pass with the grouping deleted.
func TestQAVerifyBatchesAreFiledLikeWithLikeByRoundsAlreadySpent(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-3 * time.Hour)
	cfg := func(age string) {
		os.WriteFile(a.ConfigPath, []byte("beads:\n  - "+a.BeadsDirs()[0]+
			"\nverify_chain_max: 3\nverify_batch: 4\nverify_batch_age: "+age+"\n"), 0o644)
	}
	repo := vaRepo(t, a, "[]", "verify_chain_max: 3", "verify_batch: 4", "verify_batch_age: 24h")
	// s-0 ← v-1{s-0} ← d-1: one round spent, and d-1 closed between p-2 and
	// p-3 so that close order and round order disagree.
	list := []string{
		vaClosed("s-0", t0.Add(-5*time.Hour), 1),
		vaQaBead("v-1", "s-0"),
		vaClosed("p-1", t0.Add(-4*time.Hour), 1),
		vaClosed("p-2", t0.Add(-3*time.Hour), 1),
		vaClosed("d-1", t0.Add(-2*time.Hour), 1),
		vaClosed("p-3", t0.Add(-time.Hour), 1),
		vaClosed("p-4", t0, 1),
	}
	vaChainEdge(t, repo, "d-1", "v-1", "discovered-from")
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644)
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-5*time.Hour-time.Minute))

	n, out, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d verify beads, want 1 — four round-0 closes are one batch and the round-1 close is not in it (stderr: %s)", n, errs)
	}
	calls := bdCalls(t, fake)
	if !strings.Contains(calls, "create verify 4 closes: p-1, p-2, p-3, p-4") {
		t.Errorf(`the batch is not the four closes of one round.
A batch grouped by close order alone reads "p-1, p-2, d-1, p-3" and holds
p-4, which is the filing this amendment replaces (ADR 0006 §3):
%s`, calls)
	}
	if strings.Contains(calls, verifyMarkerPrefix+"d-1"+verifyMarkerAfter) {
		t.Errorf("the round-1 close joined a round-0 batch:\n%s", calls)
	}
	if strings.Contains(calls, "VERIFY ROUND") {
		t.Errorf("a batch of first-round closes carries a round line:\n%s", calls)
	}
	if !strings.Contains(out, "1 close(s) held for a verify batch of 4") ||
		!strings.Contains(out, "each 1 verify round(s) deep") {
		t.Errorf("the pass did not say it was holding the round-1 close on its own:\n%s", out)
	}

	// The next pass, with the hold expired. The bead the first pass filed is
	// in the listing now, as it would be — the watermark stalled at d-1, so
	// p-3 and p-4 are candidates again and their markers are what dedupes
	// them.
	list = append(list, vaQaBead("q-1", "p-1", "p-2", "p-3", "p-4"))
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644)
	cfg("1m")

	n, _, errs = vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d verify beads once the hold expired, want 1 — the held round-1 close files alone (stderr: %s)", n, errs)
	}
	calls = bdCalls(t, fake)
	if !strings.Contains(calls, "create verify: gate shell d-1") {
		t.Errorf("the held close did not file alone:\n%s", calls)
	}
	if !strings.Contains(calls, "VERIFY ROUND 2 of 3") {
		t.Errorf("the round-1 filing does not name its round:\n%s", calls)
	}
}

// A `verify_chain_max:` nobody can read is named on stderr and the default
// stands — a typo must be visible, not a silently changed gate. 0 is a
// VALUE here and not a typo, which is the one difference from verifyBatch.
func TestVerifyChainMaxRefusesAValueOutsideItsRange(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"three", "-1", "51"} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			a := b.App
			vaRepo(t, a, "[]", "verify_chain_max: "+raw)
			var errb strings.Builder
			if got := a.verifyChainMax(&errb); got != DefaultVerifyChainMax {
				t.Errorf("verifyChainMax = %d for %q, want the default %d", got, raw, DefaultVerifyChainMax)
			}
			if !strings.Contains(errb.String(), `config verify_chain_max: "`+raw+`"`) {
				t.Errorf("the bad value was not named:\n%s", errb.String())
			}
		})
	}
}

// ─── ranger-base-2t90k: two claims the shallowest-source walk makes that
// nothing asserted ───────────────────────────────────────────────────────
//
// Both arms below were written because a mutant survived the whole of this
// file. Each names the mutant it kills, so the next reader can re-take the
// reading rather than trust this comment.

// A TIE AT THE MINIMUM IS WHOLE WHENEVER EITHER BRANCH IS, and until this
// arm nothing said so: deleting the tie case from verifyChainRounds' switch
// (`case len(sub) == best && whole && !bestWhole:` -> `case false && …`) left
// every other arm of this file GREEN (MEASURED 2026-11-09, ranger-base-2t90k,
// `go test -overlay` over -tags posse_arm3 -run '^Test(QA)?Verify').
//
// The rule it insures is verifyChainRounds' own: "the minimum is whole when
// some branch that ATTAINS it is whole". The walk reads a batched verify
// bead's closes in the order its description lists them, so which branch is
// seen first is a fixture choice and not a property of the lineage — and with
// the REFUSED source listed first, the floored branch arrives first and sets
// the minimum. Dropping the tie arm costs two readings, both of them about a
// count that was in fact read in full: the pass cries floor over it, and the
// round line names the source the walk could not vouch for.
//
// It cannot change whether the bound FIRES — a floored path gave up with
// depth still to spend, so it is strictly shorter than the limit, and that
// is why this arm asserts the stderr line and the rendered source rather
// than the escalation.
func TestQAVerifyChainTieAtTheMinimumIsWholeWhenEitherBranchIs(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-3 * time.Hour)
	repo := vaRepo(t, a, "[]", "verify_chain_max: 2", "operator: boss")
	// One batched round above c-1, and its verify bead answers two closes:
	//
	//	x-0 ← v-1{x-0, a-0} ← c-1    x-0 FIRST, and its edges are refused:
	//	a-0 ← v-1{x-0, a-0}          that branch gives up at depth 0.
	//	                             a-0's answer, at the same depth, is the
	//	                             tie this arm is about.
	list := []string{
		vaClosed("x-0", t0.Add(-2*time.Hour), 1),
		vaClosed("a-0", t0.Add(-2*time.Hour), 1),
		vaQaBead("v-1", "x-0", "a-0"),
		vaClosed("c-1", t0, 2),
	}
	vaChainEdge(t, repo, "c-1", "v-1", "discovered-from")
	// x-0's DOWN read answers unparseable JSON, so its branch is a FLOOR of
	// 0 — the same instrument TestVerifyChainWalkFailureFilesTheBeadAndNames
	// TheFloor uses, pointed at a sibling branch instead of the leaf.
	if err := os.WriteFile(filepath.Join(repo, "fake-deps.x-0.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	// a-0's answers, with no verify bead above it: read, whole, 0 rounds.
	if err := os.WriteFile(filepath.Join(repo, "fake-deps.a-0.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Minute))

	n, _, errs := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Fatalf("filed %d verify beads for a close one round deep under a bound of 2, want 1 (stderr: %s)", n, errs)
	}
	// CONTROL: the refused branch really was refused, or the tie this arm is
	// about never happened and a clean stderr means nothing.
	calls := bdCalls(t, fake)
	if !strings.Contains(calls, "dep list x-0") {
		t.Fatalf("CONTROL: the walk never read x-0's edges, so no branch gave up here:\n%s", calls)
	}
	if strings.Contains(errs, "which is a floor") {
		t.Errorf("a count whose minimum was ATTAINED by a branch that answered in full was "+
			"called a floor, because the branch listed FIRST was the refused one — a tie that "+
			"any branch attains whole is whole (verifyChainRounds):\n%s", errs)
	}
	if !strings.Contains(calls, "round 1: verify v-1 of a-0 (one of 2)") {
		t.Errorf("the round line must name the source the walk can vouch for (a-0), not the one "+
			"whose edges it could not read (x-0):\n%s", calls)
	}
}

// THE READ BUDGET IS PER CANDIDATE, AND `w.spent = 0` IN verifyChainRounds IS
// THE WHOLE OF WHAT MAKES IT ONE. Removing that reset (`w.spent = w.spent`)
// left every other arm of this file GREEN (MEASURED 2026-11-09,
// ranger-base-2t90k, `go test -overlay` over -tags posse_arm3 -run
// '^Test(QA)?Verify') — and what it does is ranger-base-bu8rz's defect again
// by another route: once a pass has spent verifyChainReadMax reads in TOTAL,
// every later candidate's walk gives up at its first read, counts 0 rounds as
// a floor, and a floor never escalates. The bound stops binding for every
// close after the twenty-fifth, with nothing printed but a floor line each.
//
// The existing repro asserts the reads of ONE candidate (nine beads, well
// inside the cap), so no arm of this file could tell a per-candidate budget
// from a per-pass one. This one runs 30 candidates whose lineages share
// NOTHING, so every read is a distinct bd call and the control below can see
// the budget being passed: 60 reads against a cap of 50. Sharing the deep hop
// would have made the fixture smaller and the control a lie — cache hits
// count toward the budget (verifyChainReadMax) but not toward the bd calls
// vaWalkReads can see, and the first draft of this arm read 31 and said so.
func TestQAVerifyChainReadBudgetIsPerCandidateNotPerPass(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	t0 := time.Now().Add(-3 * time.Hour)
	repo := vaRepo(t, a, "[]", "verify_chain_max: 2", "operator: boss")
	// Two rounds above every candidate, and no bead shared between them:
	//
	//	s-i ← v-i{s-i} ← m-i ← u-i{m-i} ← c-i      for i in 0..n-1
	//
	// Each c-i counts 2 rounds and is AT the bound, and each walk reads
	// exactly two dependency lists of its own: c-i's and m-i's (s-i's is
	// never reached, the depth cap ending the level below it).
	const n = 30
	var list []string
	for i := 0; i < n; i++ {
		s := fmt.Sprintf("s-%d", i)
		m := fmt.Sprintf("m-%d", i)
		c := fmt.Sprintf("c-%d", i)
		list = append(list,
			vaClosed(s, t0.Add(-3*time.Hour), 1),
			vaQaBead(fmt.Sprintf("v-%d", i), s),
			vaClosed(m, t0.Add(-2*time.Hour), 1),
			vaQaBead(fmt.Sprintf("u-%d", i), m),
			vaClosed(c, t0, 2),
		)
		vaChainEdge(t, repo, m, fmt.Sprintf("v-%d", i), "discovered-from")
		vaChainEdge(t, repo, c, fmt.Sprintf("u-%d", i), "discovered-from")
	}
	if err := os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(vaList(list...)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeVerifyWatermark(a.verifyWatermarkPath(repo), t0.Add(-time.Minute))

	// CONTROL, and it is arithmetic on the FIXTURE rather than a reading of
	// the run: two reads per candidate must come to more than one
	// candidate's whole budget, or a per-pass budget would never be reached
	// and every assertion below would pass over a fixture that cannot fail.
	// Taking it off the run instead is what the first draft did, and the
	// mutant then satisfied it — a walk that stops reading at the cap makes
	// FEWER calls, so the control fired first and reported the apparatus
	// where the finding was.
	if 2*n <= verifyChainReadMax {
		t.Fatalf("CONTROL: %d candidates at two reads each is %d, inside the %d cap — this "+
			"fixture cannot tell a per-candidate budget from a per-pass one", n, 2*n, verifyChainReadMax)
	}

	filed, out, errs := vaRun(t, a, testBd(t))
	if filed != 0 {
		t.Errorf("filed %d verify beads for %d closes that have each spent 2 rounds under "+
			"verify_chain_max: 2 — the later candidates' walks gave up on a budget the earlier "+
			"ones had already spent, so the bound stopped binding (ranger-base-bu8rz's defect "+
			"by another route)\nstderr: %s", filed, n, errs)
	}
	if strings.Contains(errs, "which is a floor") {
		t.Errorf("a candidate read a floor in a pass where its OWN budget was untouched — "+
			"verifyChainRounds resets the read budget per candidate:\n%s", errs)
	}
	if got := strings.Count(out, "no verify bead:"); got != n {
		t.Errorf("escalated %d of %d closes at the bound, want all of them:\n%s", got, n, out)
	}
	// The reading that says WHY, once the arms above have said what: with the
	// budget reset per candidate every walk makes its own two reads, and the
	// pass makes 2n. A per-pass budget stops at the cap instead.
	if got := len(vaWalkReads(bdCalls(t, fake))); got != 2*n {
		t.Errorf("the pass made %d dependency reads over %d candidates, want %d (two each) — "+
			"a count at the %d cap is the budget being shared", got, n, 2*n, verifyChainReadMax)
	}
}
