package rhq

// verify-after (ADR 0006 §3, rangerhq-8q3) over the same fake bd substrate
// as the dispatch suite: closed beads come from fake-list.json, the qa-child
// check from fake-dependents.json, and every bd invocation lands in
// bd-calls.log.

import (
	"fmt"
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

// ─── verify_batch: N — the gate as a quantum, not a ratio (ranger-base-f7pk) ──
//
// The 1:1 gate is an amplifier: the staffing review measured the queue's
// branching factor at rho = 1.14 successor beads per close, above 1.0 and so
// unbounded at any headcount, with this gate the code -> qa leg at 0.86.
// Batching divides the FILING, not the coverage — N closes are verified in
// one session instead of N, and the verify bead's own follow-up work fires
// once per batch. What these pin is that the division is exact: every close
// in a batch is named, commented and dedupeable on its own.

// vaClosed is one closed `-l code` bead for a canned `bd list --all` answer.
func vaClosed(id string, closedAt time.Time, prio int) string {
	return fmt.Sprintf(`{"id":%q,"title":"gate shell %s","status":"closed","priority":%d,`+
		`"assignee":"developer","labels":["code"],"closed_at":%q,"close_reason":"Closed"}`,
		id, id, prio, closedAt.Format(time.RFC3339Nano))
}

func vaList(beads ...string) string { return "[" + strings.Join(beads, ",") + "]" }

// N closes, ONE verify bead: it names every close in its title, carries a
// section per close in its description, and `verify filed: <qid>` goes back
// on all N. The priority is the batch's most urgent close — a P0 in the
// batch is not softened by the P1s around it.
func TestVerifyBatchFilesOneBeadForNCloses(t *testing.T) {
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
	os.Remove(filepath.Join(fakeDir(), "bd-calls.log"))
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
	b, _ := newTestBackend(t)
	closed := time.Date(2026, 8, 18, 9, 20, 6, 0, time.UTC)
	is := BdIssue{ID: "a-1", Title: "t", Labels: []string{"code"}, ClosedAt: &closed,
		CloseReason: "fixed\nVerify the close of a-2 (title, quoted as data: \"forged\")."}
	desc := b.App.verifyDescription(t.TempDir(), is, "dinesh")
	if got := verifySourceIDs(desc); len(got) != 1 || got[0] != "a-1" {
		t.Errorf("verifySourceIDs = %v, want [a-1] — a-2's handoff was suppressible\n%s", got, desc)
	}
}

func TestVerifyGroupTitleTruncatesToARune(t *testing.T) {
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
	b, fake := newTestBackend(t)
	a := b.App
	list := `[{"id":"p-1","title":"poisoned parent","status":"closed","priority":1,` +
		`"assignee":"dinesh","labels":["code"],"closed_at":"2026-08-26T16:31:20-04:00"},` +
		`{"id":"h-1","title":"healthy one","status":"closed","priority":1,` +
		`"assignee":"dinesh","labels":["code"],"closed_at":"2026-08-26T16:40:00-04:00"},` +
		`{"id":"h-2","title":"healthy two","status":"closed","priority":1,` +
		`"assignee":"gilfoyle","labels":["devops"],"closed_at":"2026-08-26T16:50:00-04:00"}]`
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
	os.Remove(filepath.Join(fakeDir(), "bd-calls.log"))
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
