package posse

// QA pins for the shop pulse (ranger-base-dwlb1): the arithmetic behind the
// line every surface prints, and the unclassified bucket that line exists to
// keep visible.
//
// The arithmetic is worth pinning because every cell of it is a plausible
// wrong answer: a median that quietly includes this morning, a close counted
// by `status: closed` rather than by its stamp, a class census that folds the
// beads it cannot classify into the nearest bucket. Each of those renders a
// number the operator would read and act on, and none of them looks wrong.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pulseNow is the fixture clock: 11:30 local on 2026-09-02, the hour of the
// ruling this was built for. LOCAL and not UTC on purpose — the day the
// operator counts closes in is his own, and a fold that truncated in UTC
// would put a 20:00 close in tomorrow for half the planet.
var pulseNow = time.Date(2026, 9, 2, 11, 30, 0, 0, time.Local)

func pulseAt(day, hour int) *time.Time {
	t := time.Date(2026, 9, day, hour, 0, 0, 0, time.Local)
	return &t
}

// pulseFixture is one store: closes spread over the window, an open pile
// covering all four classes, and the two rows that break a naive fold.
func pulseFixture() []BdIssue {
	old := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	var out []BdIssue
	// Closes per day across the window: today 3, then the seven complete
	// days behind it. The complete days sort to [0 1 1 2 4 5 6] → median 2,
	// and today's 3 would move it to 3 if it were let in.
	closes := map[int]int{2: 3, 1: 5, 31: 1, 30: 6, 29: 1, 28: 4, 27: 2, 26: 0}
	for day, n := range closes {
		d := day
		mo := 9
		if d > 20 {
			mo = 8
		}
		for i := 0; i < n; i++ {
			at := time.Date(2026, time.Month(mo), d, 9, 0, 0, 0, time.Local)
			out = append(out, BdIssue{ID: "x", Status: "closed", Created: old, ClosedAt: &at})
		}
	}
	// A close with no stamp: its DAY is unknown, so it is neither today's
	// work nor any other day's — never today's by default.
	out = append(out, BdIssue{ID: "nostamp", Status: "closed", Created: old})
	// The open pile. in_progress and blocked are open work: a bead someone
	// is holding has not left the pile.
	open := []BdIssue{
		{ID: "f1", Status: "open", IssueType: "feature", Priority: 1},
		{ID: "f2", Status: "in_progress", IssueType: "feature", Priority: 3},
		{ID: "b1", Status: "open", IssueType: "bug", Priority: 1},
		{ID: "b2", Status: "open", IssueType: "bug", Priority: 2},
		{ID: "b3", Status: "blocked", IssueType: "bug", Priority: 2},
		// Type wins over the label: `-t bug` carrying `-l debt` is a filing
		// error the groom clears, and until it does the bead is a bug.
		{ID: "b4", Status: "open", IssueType: "bug", Priority: 2, Labels: []string{"debt", "code"}},
		{ID: "d1", Status: "open", IssueType: "task", Priority: 2, Labels: []string{"code", "debt"}},
		{ID: "u1", Status: "open", IssueType: "task", Priority: 3},
		{ID: "u2", Status: "open", Priority: 3, Labels: []string{"code"}},
	}
	for i := range open {
		open[i].Created = pulseNow.Add(-2 * time.Hour)
	}
	return append(out, open...)
}

// The class helper is ONE reader (ADR 0006 §1, amended 2026-09-02) — the
// scorecard, the pulse line and verify-after's filer all read it, so the rule
// is pinned here rather than re-derived at each call site. The conflict row
// is the one that matters: type wins.
func TestQABeadClassIsOneReaderOfTheClassRule(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		is   BdIssue
		want string
	}{
		{"feature type", BdIssue{IssueType: "feature"}, ClassFeature},
		{"bug type", BdIssue{IssueType: "bug"}, ClassBug},
		{"debt label", BdIssue{IssueType: "task", Labels: []string{"code", "debt"}}, ClassDebt},
		{"debt label, no type", BdIssue{Labels: []string{"debt"}}, ClassDebt},
		{"conflict: type wins", BdIssue{IssueType: "bug", Labels: []string{"debt"}}, ClassBug},
		{"conflict: feature wins", BdIssue{IssueType: "feature", Labels: []string{"debt"}}, ClassFeature},
		{"task, no debt", BdIssue{IssueType: "task", Labels: []string{"code"}}, ClassUnclassified},
		{"nothing at all", BdIssue{}, ClassUnclassified},
		// Never inferred from any other label, or from the title.
		{"a debt-shaped title", BdIssue{Title: "pay down the debt", Labels: []string{"code"}}, ClassUnclassified},
		{"a chore is not debt", BdIssue{IssueType: "chore"}, ClassUnclassified},
	} {
		if got := BeadClass(tc.is); got != tc.want {
			t.Errorf("%s: BeadClass = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The arithmetic, cell by cell, against a fixed clock.
func TestQAShopPulseArithmetic(t *testing.T) {
	t.Parallel()
	p := FoldBeadPulse(pulseFixture(), pulseNow)

	if p.ClosedToday != 3 {
		t.Errorf("closes today = %d, want 3", p.ClosedToday)
	}
	// The median is over the COMPLETE days only. Today closed 3, which is
	// above the median of the days behind it: a fold that let today in
	// answers 3 and the line stops saying anything about "typical".
	if !p.MedianKnown || p.Median != 2 {
		t.Errorf("median = %d (known %v), want 2 — today must not be in the window", p.Median, p.MedianKnown)
	}
	for _, tc := range []struct {
		class string
		want  ClassCensus
	}{
		{ClassFeature, ClassCensus{Open: 2, P1: 1}},
		{ClassBug, ClassCensus{Open: 4, P1: 1, P2: 3}},
		{ClassDebt, ClassCensus{Open: 1, P2: 1}},
		{ClassUnclassified, ClassCensus{Open: 2}},
	} {
		if got := p.Class[tc.class]; got != tc.want {
			t.Errorf("%s census = %+v, want %+v", tc.class, got, tc.want)
		}
	}
	if p.Open() != 9 || p.P1() != 2 || p.P2() != 4 {
		t.Errorf("totals: open %d P1 %d P2 %d, want 9/2/4", p.Open(), p.P1(), p.P2())
	}
	// The per-day table: the median's window, then today marked partial.
	if len(p.Days) != PulseDays+1 {
		t.Fatalf("days = %d rows, want %d complete + today", len(p.Days), PulseDays)
	}
	last := p.Days[len(p.Days)-1]
	if !last.Partial || !last.Day.Equal(time.Date(2026, 9, 2, 0, 0, 0, 0, time.Local)) {
		t.Errorf("last row must be today, partial: %+v", last)
	}
	for _, d := range p.Days[:len(p.Days)-1] {
		if d.Partial {
			t.Errorf("%s is a complete day, marked partial", d.Day.Format("2006-01-02"))
		}
	}
	wantClosed := map[string]int{
		"2026-08-26": 0, "2026-08-27": 2, "2026-08-28": 4, "2026-08-29": 1,
		"2026-08-30": 6, "2026-08-31": 1, "2026-09-01": 5, "2026-09-02": 3,
	}
	for _, d := range p.Days {
		if got := wantClosed[d.Day.Format("2006-01-02")]; d.Closed != got {
			t.Errorf("%s closed = %d, want %d", d.Day.Format("2006-01-02"), d.Closed, got)
		}
	}
	// Every open bead was created two hours ago, so today's row carries
	// them and no other row does: created-vs-closed is what says whether
	// the pile grew, and it is the pair the raw total pretended to be.
	if last.Created != 9 {
		t.Errorf("today created = %d, want 9", last.Created)
	}
	for _, d := range p.Days[:len(p.Days)-1] {
		if d.Created != 0 {
			t.Errorf("%s created = %d, want 0", d.Day.Format("2006-01-02"), d.Created)
		}
	}
	// A close with no closed_at belongs to no day — and in particular it is
	// not silently today's, which is the shape that inflates this morning.
	total := 0
	for _, d := range p.Days {
		total += d.Closed
	}
	if total != 22 {
		t.Errorf("closes across the window = %d, want 22 (the unstamped close belongs to no day)", total)
	}
}

// The unclassified bucket is REPORTED, never folded (ADR 0006 §1). On the
// day this landed 0 of 153 open beads carried `debt`, so a census that
// rounded the gap away would have reported a tidy fiction.
func TestQAShopPulseCountsTheUnclassifiedBucketSeparately(t *testing.T) {
	t.Parallel()
	// A store with nothing classified at all: the shape this instance was
	// actually in when the operator asked for the numbers.
	var issues []BdIssue
	for i := 0; i < 5; i++ {
		issues = append(issues, BdIssue{ID: "u", Status: "open", IssueType: "task", Priority: 2, Labels: []string{"code"}})
	}
	p := FoldBeadPulse(issues, pulseNow)
	if p.Class[ClassUnclassified].Open != 5 {
		t.Errorf("unclassified = %d, want 5", p.Class[ClassUnclassified].Open)
	}
	for _, cl := range []string{ClassFeature, ClassBug, ClassDebt} {
		if got := p.Class[cl].Open; got != 0 {
			t.Errorf("%s = %d — an unclassifiable bead must never be folded into a class", cl, got)
		}
	}
	// The line names it, and names the empty classes too: a slot that
	// disappears at zero is a slot nobody notices is missing, and debt and
	// unclassified are exactly the two that sit at zero until a groom runs.
	line := p.Line()
	for _, want := range []string{"0F/", "0B/", "0D/", "5U", "P1 0", "P2 5", "closes today 0"} {
		if !strings.Contains(line, want) {
			t.Errorf("line missing %q: %s", want, line)
		}
	}
	// And the section says what the bucket means rather than printing a
	// number with no name on it.
	var out strings.Builder
	WriteBeadPulse(&out, p)
	for _, want := range []string{"unclassified", "5", "ADR 0006", "groom"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("section missing %q:\n%s", want, out.String())
		}
	}
}

// The line is the one rendering `posse status`, the watch log and the
// cockpit share, so its shape is a pin: no raw open total, and the four
// class slots plus P1/P2 always present.
func TestQAShopPulseLineReplacesTheRawOpenCount(t *testing.T) {
	t.Parallel()
	p := FoldBeadPulse(pulseFixture(), pulseNow)
	line := p.Line()
	for _, want := range []string{"closes today 3", "7d median 2", "2F/4B/1D/2U", "P1 2", "P2 4"} {
		if !strings.Contains(line, want) {
			t.Errorf("line missing %q: %s", want, line)
		}
	}
	// The pile is 9 open. The number the operator ruled out must not be
	// standing on the line beside the ones that replaced it.
	if strings.Contains(line, "9 open") || strings.Contains(line, "open 9") {
		t.Errorf("the raw open total is back on the line: %s", line)
	}
	// An unread repo makes every number a floor, and the line says so —
	// the same rule the scorecard's table keeps for an unreadable store.
	p.Unread = 2
	if !strings.Contains(p.Line(), "partial, 2 repo(s) unread") {
		t.Errorf("an unread repo must be on the line: %s", p.Line())
	}
	// A store with no history at all: the median is unknown, not zero.
	empty := FoldBeadPulse(nil, pulseNow)
	if !empty.MedianKnown {
		t.Skip("PulseDays is 0")
	}
	if empty.Median != 0 || !strings.Contains(empty.Line(), "median 0") {
		t.Errorf("an empty store closes nothing: %s", empty.Line())
	}
}

// ReadBeadPulse over the configured repos: a repo whose scan fails is
// counted UNREAD, never dropped into the fold as an empty one.
func TestQAShopPulseNamesARepoItCouldNotRead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	good, bad := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(good, "fake-list.json"), []byte(`[
		{"id":"a-1","status":"open","issue_type":"bug","priority":1},
		{"id":"a-2","status":"open","issue_type":"task","priority":2}]`), 0o644)
	// The `list` mirror of a locked database: the repo resolves, bd fails.
	os.WriteFile(filepath.Join(bad, "fake-list-fail"), nil, 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+good+"\n  - "+bad+"\n"), 0o644)

	p, failed := b.App.ReadBeadPulse(bd, pulseNow)
	if len(failed) != 1 {
		t.Fatalf("want one failed repo, got %v", failed)
	}
	if p.Repos != 1 || p.Unread != 1 {
		t.Errorf("repos %d unread %d, want 1/1", p.Repos, p.Unread)
	}
	if p.Class[ClassBug].Open != 1 || p.Class[ClassUnclassified].Open != 1 {
		t.Errorf("the readable repo's beads must still be counted: %+v", p.Class)
	}
	if !strings.Contains(p.Line(), "partial, 1 repo(s) unread") {
		t.Errorf("line must say the reading is partial: %s", p.Line())
	}
	if lines := PulseFailureLines(failed); len(lines) != 1 || !strings.Contains(lines[0], "database is locked") {
		t.Errorf("the failure must name its reason: %v", lines)
	}
}

// The scorecard carries the section: the line, the per-class P1/P2 table and
// created-vs-closed per day — the arithmetic behind the one-liner, so the
// operator can check the median by eye rather than believe it.
func TestQAScorecardCarriesTheShopPulseSection(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	writePersona(t, b.App, "dev", "[code]")
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(`[
		{"id":"a-1","status":"open","issue_type":"feature","priority":1},
		{"id":"a-2","status":"open","issue_type":"bug","priority":2},
		{"id":"a-3","status":"open","labels":["debt"],"priority":2},
		{"id":"a-4","status":"open","priority":3},
		{"id":"a-5","status":"closed","closed_at":"`+pulseNow.Format(time.RFC3339)+`"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	var out strings.Builder
	if err := b.App.Scorecard(bd, &out, "", pulseNow); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"shop pulse", "closes today 1", "1F/1B/1D/1U", "P1 1", "P2 2",
		"open by class", "feature", "bug", "debt", "unclassified",
		"created", "closed", "net", pulseNow.Format("2006-01-02"),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("scorecard missing %q:\n%s", want, got)
		}
	}
}

// ─── the watch log ───────────────────────────────────────────────────────────

// The pulse tick's rendering: its own line above the conditions, never
// appended to them. The condition line is the stable-token rendering the
// blocked-time-to-intervention metric is greped out of (pulse.go), and a
// moving number on it breaks every reader of that log.
func TestQAPulseTickLogsTheShopPulseOnItsOwnLine(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	repo := unpushedRepo(t, b)
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(`[
		{"id":"a-1","status":"open","issue_type":"bug","priority":1},
		{"id":"a-2","status":"open","priority":2}]`), 0o644)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	d.pulseOnce(PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute, RenagMax: 4 * time.Hour})

	out := dispatcherOut(d)
	var shop, cond string
	for _, ln := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(ln, "pulse: shop "):
			shop = ln
		case strings.HasPrefix(ln, "pulse: ") && cond == "":
			cond = ln
		}
	}
	if !strings.Contains(shop, "open 0F/1B/0D/1U") || !strings.Contains(shop, "P1 1") {
		t.Fatalf("the tick must log the reading:\n%s", out)
	}
	if cond == "" {
		t.Fatalf("no condition line at all — this tick is supposed to have one:\n%s", out)
	}
	if strings.Contains(cond, "closes today") || strings.Contains(cond, "open 0F/") {
		t.Errorf("the reading must not ride the condition line, which is greped for stable tokens:\n%s", cond)
	}
}
