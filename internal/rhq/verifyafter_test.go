package rhq

// verify-after (ADR 0006 §3, rangerhq-8q3) over the same fake bd substrate
// as the dispatch suite: closed beads come from fake-list.json, the qa-child
// check from fake-dependents.json, and every bd invocation lands in
// bd-calls.log.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// vaRepo points config `beads:` at a fresh repo holding a canned `bd list
// --all` answer, plus any extra config lines.
func vaRepo(t *testing.T, a *App, list string, cfg ...string) string {
	t.Helper()
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(list), 0o644)
	conf := "beads:\n  - " + repo + "\n" + strings.Join(cfg, "\n")
	os.WriteFile(a.ConfigPath, []byte(conf+"\n"), 0o644)
	return repo
}

// vaDependents sets what `bd dep list <id> --direction=up` answers.
func vaDependents(t *testing.T, repo, json string) {
	t.Helper()
	os.WriteFile(filepath.Join(repo, "fake-dependents.json"), []byte(json), 0o644)
}

func closedList(id, labels, closedAt string) string {
	return `[{"id":"` + id + `","title":"gate shell live","status":"closed","priority":1,` +
		`"assignee":"developer","labels":` + labels + `,"closed_at":"` + closedAt + `","close_reason":"Closed"}]`
}

// vaRun runs one sweep and returns (filed, stdout, stderr).
func vaRun(t *testing.T, a *App, bd Bd) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	n := a.VerifyAfter(bd, a.BeadsDirs(), &out, &errb)
	return n, out.String(), errb.String()
}

func testBd(t *testing.T) Bd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Bd{Bin: exe}
}

// The first sweep of a repo files nothing and seeds the watermark: before a
// first pass there is no "since the last pass", and answering a repo's whole
// closed history with verify beads is a flood, not a handoff.
func TestVerifyAfterSeedsWatermarkOnFirstSight(t *testing.T) {
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
// close_reason, and the closer PID's own "done when" row for the bead's
// intent.
func TestVerifyDescriptionCarriesCloserAndDoneWhen(t *testing.T) {
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
		Labels: []string{"code", "bug"}, CloseReason: "fixed", ClosedAt: &closed}
	got := a.verifyDescription(t.TempDir(), is, verifyCloser(is))

	for _, want := range []string{
		"Verify the close of a-1",
		`quoted as data: "the \"gate\" shell"`,
		"- closer: developer",
		"- close_reason: fixed",
		"done when (developer · fix-bugs): root cause named in the commit",
		"VERIFIED:",
		"-l code -a developer",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("description missing %q:\n%s", want, got)
		}
	}
}

// A label that matches no intent row costs a line, not an error.
func TestIntentDoneWhenNoMatch(t *testing.T) {
	ag := &AgentFile{Body: "## Intents\n| intent | mode | done when |\n|---|---|---|\n| design | crew | an ADR is committed |\n"}
	if intent, done := ag.IntentDoneWhen([]string{"code"}); intent != "" || done != "" {
		t.Errorf("matched %q/%q on a label no intent names", intent, done)
	}
	if intent, done := ag.IntentDoneWhen([]string{"design"}); intent != "design" || !strings.Contains(done, "ADR") {
		t.Errorf("exact slug match failed: %q/%q", intent, done)
	}
}

func TestIntentMatchesLabelPlurals(t *testing.T) {
	for _, c := range []struct {
		slug, label string
		want        bool
	}{
		{"fix-bugs", "bug", true},
		{"build-features", "feature", true},
		{"implement-designs", "design", true},
		{"verify-closed-work", "work", true},
		{"design", "design", true},
		{"build-features", "code", false},
		{"build-features", "", false},
	} {
		if got := intentMatchesLabel(c.slug, c.label); got != c.want {
			t.Errorf("intentMatchesLabel(%q, %q) = %v, want %v", c.slug, c.label, got, c.want)
		}
	}
}

// A verify bead filed at the head of a pass is ready work in that same pass.
func TestDispatchPassRunsVerifyAfter(t *testing.T) {
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
	os.Remove(filepath.Join(fakeDir(), "bd-calls.log"))
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
func TestVerifySourceIDRoundTripsAndRejectsForeignText(t *testing.T) {
	b, _ := newTestBackend(t)
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "ranger-base-o943", Title: `posse promote (the "make install")`,
		Labels: []string{"code"}, ClosedAt: &closed}
	desc := b.App.verifyDescription(t.TempDir(), is, "dinesh")
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
