package main

// QA pins for ranger-base-2vhqo: the SHARED backend's own diagnostic lines
// reach a surface of the cockpit's own, and never the process's stderr —
// which, with the alt screen up, is the frame the cockpit is drawing.
//
// THE DEFECT, and it is the half ranger-base-ws20a left behind. That bead
// routed the DISPATCH command's backend warn stream to the pass's own writer
// (Dispatcher.RouteBackendWarnings) and deliberately left the cockpit alone,
// because the cockpit's only line-shaped sink is Progress = c.note — a
// non-blocking send with a bare `default:` arm, which does not merely show
// one line at a time, it DROPS what will not fit. So `posse cockpit` kept
// HerdrBackend.Warn nil, nil is os.Stderr, and every non-launch b.warn site
// printed onto the glass:
//
//   - listSessions' five "session meta file(s) kept, not listed"
//     abstentions, two of them carrying a two-line repair recipe —
//     c.refreshSessions() takes that path on EVERY tick, so one withheld
//     meta wrote one of them every two seconds;
//   - killAndLand's "fold refusals spool for <name>" and
//     noteUnlandedOnKill's "<bead> left work unlanded in <tree> … and bd
//     could not say whether it is closed", from the cockpit's own `x`.
//
// THE FIX is a sink, a counted row and a key (cmd/posse/cockpitnotices.go):
// newCockpit sets HerdrBackend.Warn to a cockpitNotices, the row model
// counts it above SESSIONS, `w` peeks the whole text, and flush() hands it
// back to stderr after the alt screen is restored.
//
// Not a gate, like ws20a: nothing here changes what a listing withholds or
// what a kill lands. The arms that drive a real listing assert the meta is
// still kept.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// cnFake is a herdr that holds nothing: an empty board, which is all these
// arms need — the abstention they drive is decided from the meta FILE, before
// the board is consulted at all.
const cnFakeHerdr = `#!/bin/sh
if [ "$1" = "workspace" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"result":{"workspaces":[]}}'
  exit 0
fi
if [ "$1" = "agent" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"result":{"agents":[]}}'
  exit 0
fi
printf '%s\n' '{"error":{"code":"no","message":"unexpected '"$1 $2"'"}}'
exit 1
`

// cnCockpit builds a cockpit over a real backend, THROUGH newCockpit —
// which is the wiring under test, so no arm here may shortcut it with a
// struct literal the way this package's older tests do.
//
// stderrish is handed to the backend BEFORE the constructor runs, standing
// in for the os.Stderr a production backend has while nobody has assigned
// the field. A line in it is a line that would have been painted over the
// frame: the constructor is expected to take the field over.
func cnCockpit(t *testing.T) (c *cockpit, stderrish *bytes.Buffer, home string) {
	t.Helper()
	home = t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	herdr := filepath.Join(t.TempDir(), "herdr")
	if err := posse.WriteExecutable(herdr, []byte(cnFakeHerdr), 0o755); err != nil {
		t.Fatal(err)
	}
	a := &posse.App{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.yaml"),
		StateDir:   filepath.Join(home, "state"),
	}
	stderrish = &bytes.Buffer{}
	hb := &posse.HerdrBackend{App: a, H: posse.Herdr{Bin: herdr}, Warn: stderrish}
	c = newCockpit(a, hb, &bytes.Buffer{})
	// The header carries a clock (ADR 0004 §5), so two renders a second
	// apart differ for a reason that has nothing to do with this bead —
	// which the tick arm below compares frames across. Pinned, like the
	// golden tests.
	c.now = func() time.Time { return time.Date(2026, 10, 3, 23, 59, 0, 0, time.UTC) }
	return c, stderrish, home
}

// cnUnreadableMeta plants the cheapest of the five abstentions that carries a
// REPAIR RECIPE (ranger-base-82e40): a session record whose bytes hold no
// `name:` line at all. listSessions can then say neither that the session is
// alive nor that it is gone, so it withholds the row, keeps the file and
// reports it in two lines — the line, and the line saying what the file costs
// until somebody repairs it. Two lines is the whole reason this screen could
// not use its status line.
func cnUnreadableMeta(t *testing.T, home, name string) string {
	t.Helper()
	dir := filepath.Join(home, "state", "herdr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(p, []byte("truncated before the first line landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Arm 1, end to end through a real tick: the listing abstention a refresh
// takes lands in the cockpit's notices and on its row, and NOT on the writer
// a production backend would have — which under the alt screen is the frame
// itself.
func TestQACockpitListingAbstentionLandsInItsNoticesNotOnTheFrame(t *testing.T) {
	t.Parallel()
	c, stderrish, home := cnCockpit(t)
	meta := cnUnreadableMeta(t, home, "ranger-repo-a-1")

	c.refreshSessions()

	if got := c.notices.count(); got != 2 {
		t.Fatalf("the abstention's two lines are not both in the notices (count %d):\n%s", got, c.notices.text())
	}
	if body := c.notices.text(); !strings.Contains(body, "carries no record") {
		t.Errorf("the notices do not hold the abstention:\n%s", body)
	}
	if got := stderrish.String(); got != "" {
		t.Errorf("the abstention went to the backend's own writer — os.Stderr in production, which with the alt screen up is the frame being drawn:\n%s", got)
	}
	// The row: a COUNT where the operator is looking, and the key that
	// reads the rest. The text itself must NOT be on the frame — five
	// abstentions and their recipes are what this screen has no room for.
	frame := qaPlain(c.render(120, 24))
	if !strings.Contains(frame, "2 backend notice(s)") || !strings.Contains(frame, "w reads them") {
		t.Errorf("the frame neither counts the notices nor says how to read them:\n%s", frame)
	}
	if strings.Contains(frame, "carries no record") {
		t.Errorf("the abstention's text is on the frame, which is the body it would push off:\n%s", frame)
	}
	// Not a gate: the meta is still KEPT, which is the whole of what the
	// line reports.
	if _, err := os.Stat(meta); err != nil {
		t.Errorf("the withheld meta was deleted — this bead moves where a line goes and nothing else: %v", err)
	}
}

// Arm 2: the REPAIR RECIPE's second line, which is the half the status line
// could never have carried and the reason modePeek is the surface. `w` shows
// both lines, the banner names what is being read, and any key returns.
func TestQACockpitNoticesKeepTheRepairRecipesSecondLine(t *testing.T) {
	t.Parallel()
	c, _, home := cnCockpit(t)
	cnUnreadableMeta(t, home, "ranger-repo-a-1")
	c.refreshSessions()
	before := c.status

	if quit, err := c.handleKey([]byte("w")); quit || err != nil {
		t.Fatalf("w quit or failed: %v %v", quit, err)
	}
	if c.mode != modePeek {
		t.Fatalf("w did not open the notices (mode %v)", c.mode)
	}
	if c.status != before {
		t.Errorf("the notices went onto the status line as well (%q): it is one field and it is the launch's (ADR 0004 §3)", c.status)
	}
	frame := qaPlain(c.render(200, 24))
	for _, want := range []string{
		"── notices (any key to return) ──",
		"carries no record",
		"holds its dispatch seat until the file is repaired",
	} {
		if !strings.Contains(frame, want) {
			t.Errorf("the peek does not carry %q:\n%s", want, frame)
		}
	}
	// The dir to repair in is asserted on the TEXT and not on the frame:
	// every line in this screen is truncated to the pane (truncCells), and
	// an absolute path is the part that goes over the edge first. That is
	// what the row's count and this peek are for — the screen is never the
	// last copy of these lines, the flush on the way out is (arm 6).
	if !strings.Contains(c.notices.text(), filepath.Join(home, "state", "herdr")) {
		t.Errorf("the notices do not name the dir to repair in:\n%s", c.notices.text())
	}
	// Any key returns, like the `v` peek — and the banner goes back to
	// naming a terminal tail the next time `v` opens one.
	if _, err := c.handleKey([]byte("j")); err != nil {
		t.Fatal(err)
	}
	if c.mode != modeNormal {
		t.Errorf("a key did not leave the notices (mode %v)", c.mode)
	}
	if !strings.Contains(qaPlain(c.render(120, 24)), "2 backend notice(s)") {
		t.Error("the row is gone after reading the notices: nothing was repaired, so the count still stands")
	}
}

// Arm 3: the per-tick repeat, which is this surface's whole arithmetic. The
// cockpit refreshes every two seconds and the abstention is written on every
// one of them, so a sink that appended would hold 1,800 identical lines an
// hour — and a count that moved every tick would change the frame on every
// tick, defeating the identical-frame drop (c.lastFrame,
// cockpitflicker_test.go). Distinct lines only: the count moves when
// something NEW happens, and at no other time.
func TestQACockpitNoticesDoNotGrowOnEveryTick(t *testing.T) {
	t.Parallel()
	c, _, home := cnCockpit(t)
	cnUnreadableMeta(t, home, "ranger-repo-a-1")

	c.refreshSessions()
	first := c.render(120, 24)
	for i := 0; i < 20; i++ {
		c.refreshSessions()
	}
	if got := c.notices.count(); got != 2 {
		t.Errorf("21 ticks over ONE withheld meta left %d notices, want the 2 distinct lines it writes:\n%s", got, c.notices.text())
	}
	if got := c.render(120, 24); got != first {
		t.Errorf("the frame moved under a board that did not: an identical frame is dropped (c.lastFrame) and this one would not be\nfirst:\n%s\nlater:\n%s",
			qaPlain(first), qaPlain(got))
	}
	// A SECOND withheld meta is news, and news moves the count — by ONE,
	// not by two: the abstention now reads "2 session meta file(s) … — a-1,
	// a-2", which is a new line, while the repair recipe under it is
	// byte-for-byte the line already held. Three distinct lines, and the
	// recipe is kept once however many metas ask for it.
	cnUnreadableMeta(t, home, "ranger-repo-a-2")
	c.refreshSessions()
	if got := c.notices.count(); got != 3 {
		t.Errorf("a second withheld meta left the count at %d, want 3 — a repeat is a no-op, a new line is not:\n%s", got, c.notices.text())
	}
	if body := c.notices.text(); !strings.Contains(body, "ranger-repo-a-1, ranger-repo-a-2") {
		t.Errorf("the new abstention naming both metas is not there:\n%s", body)
	}
}

// Arm 4: a clean shop keeps every byte it had. The row draws only when there
// is something to say, it is FILLER when it does (not cursor space, so tab,
// reselect and every key are untouched), and `w` over nothing says so rather
// than opening an empty screen.
func TestQACockpitWithNothingToSayDrawsNoNoticeRow(t *testing.T) {
	t.Parallel()
	c, _, _ := cnCockpit(t)
	c.refreshSessions()

	if got := c.notices.count(); got != 0 {
		t.Fatalf("premise: a quiet board warns about nothing, got %d:\n%s", got, c.notices.text())
	}
	if frame := qaPlain(c.render(120, 24)); strings.Contains(frame, "backend notice") {
		t.Errorf("a clean shop spends a row saying so:\n%s", frame)
	}
	if quit, err := c.handleKey([]byte("w")); quit || err != nil {
		t.Fatalf("w quit or failed: %v %v", quit, err)
	}
	if c.mode != modeNormal {
		t.Errorf("w opened an empty peek (mode %v)", c.mode)
	}
	if c.status != "no backend notices" {
		t.Errorf("w over nothing said %q", c.status)
	}

	// And with something to say, the row is filler: a condition heals by
	// itself or by a repair in a shell, so there is no key that acts on it
	// and it must not take cursor space (ADR 0004 §2, the GOVERNANCE
	// block's own reasoning).
	fmt.Fprint(c.hb.Warn, "posse: 1 session meta file(s) kept, not listed: this herdr (sock) does not hold their workspaces\n")
	items := c.items()
	c.buildRows()
	if c.items() != items {
		t.Errorf("the notice row moved cursor space from %d to %d", items, c.items())
	}
	found := false
	for _, r := range c.rows {
		for _, col := range r.cols {
			if !strings.Contains(col.text, "backend notice(s)") {
				continue
			}
			found = true
			if r.kind != rowFiller {
				t.Errorf("the notice row is kind %v, not rowFiller — it would be selectable and no key acts on it", r.kind)
			}
		}
	}
	if !found {
		t.Error("the row model does not carry the notice row at all")
	}
}

// Arm 5: every line survives, which is the claim c.note could not make. The
// lines are written through HerdrBackend.Warn, which is exactly what each
// b.warn site does (b.warn is one Fprintf to this writer) — so this arm
// covers the sites a herdr fixture cannot cheaply reach from here:
// killAndLand's fold refusal and noteUnlandedOnKill's unlanded-work line,
// both of them the cockpit's own `x` (cockpit.go, c.hb.KillSessionAndLand),
// plus the five listing abstentions at once.
//
// Five abstentions through Progress would arrive as ONE surviving line and
// four silent discards: c.note is a non-blocking send onto a buffered
// channel with a bare default: arm. Nothing here may route through it.
func TestQACockpitNoticesKeepEveryLineTheStatusLineWouldDrop(t *testing.T) {
	t.Parallel()
	c, stderrish, _ := cnCockpit(t)
	before := c.status

	lines := []string{
		"posse: 2 session meta file(s) kept, not listed: this herdr (sock) does not hold their workspaces\n",
		"posse: 1 session meta file(s) kept, not listed: missing from this listing but not dead (rangerhq-9nso) — a-1: no\n",
		"posse: 1 session meta file(s) kept, not listed: another workspace holds the id they recorded (rangerhq-yt1p) — a-2\n  repair by matching each meta's filename against the workspace label\n",
		"posse: 1 session meta file(s) kept, not listed: the file is there and carries no record (ranger-base-82e40) — a-3\n  a session whose meta reads like this holds its dispatch seat\n",
		"posse: 1 session(s) closed without a replacement, recipe kept: a-4 — rebuild with `posse relaunch <name>`\n",
		"fold refusals spool for ranger-repo-a-5: is a directory\n",
		"posse: ranger-base-abcd left work unlanded in ~/wt/a-6 (3 uncommitted path(s)) and bd could not say whether it is closed (exit 1) — not noted on the bead\n",
	}
	for _, ln := range lines {
		fmt.Fprint(c.hb.Warn, ln)
	}

	// Nine entries: seven writes, two of which carry a second line.
	if got, want := c.notices.count(), 9; got != want {
		t.Errorf("the notices hold %d of the %d lines written:\n%s", got, want, c.notices.text())
	}
	body := c.notices.text()
	for _, ln := range lines {
		for _, want := range strings.Split(strings.TrimRight(ln, "\n"), "\n") {
			if !strings.Contains(body, strings.TrimRight(want, " ")) {
				t.Errorf("a line was dropped:\n  %q\nnotices:\n%s", want, body)
			}
		}
	}
	// Not through the status line's channel, and not onto the status line.
	if n := len(c.progress); n != 0 {
		t.Errorf("%d notice(s) went onto the Progress channel, which drops what will not fit", n)
	}
	if c.status != before {
		t.Errorf("a backend notice took the status line (%q), which belongs to the operator's last keypress", c.status)
	}
	if stderrish.Len() != 0 {
		t.Errorf("a notice reached the writer the constructor was supposed to take over:\n%s", stderrish.String())
	}
}

// Arm 6: nothing is lost on quit. A cockpit the operator closed having never
// pressed `w` still hands the lines back — to stderr, where they were going
// before this sink existed, and AFTER the alt screen is restored so they are
// in the scrollback rather than in a frame about to be overwritten.
//
// The ordering is read as source because the only observable of it is a byte
// sequence on a terminal this test does not have: the flush is deferred
// ABOVE the alt-screen defers, so LIFO runs it last.
func TestQACockpitNoticesGoBackToStderrOnTheWayOut(t *testing.T) {
	t.Parallel()
	c, _, home := cnCockpit(t)
	cnUnreadableMeta(t, home, "ranger-repo-a-1")
	c.refreshSessions()

	var out bytes.Buffer
	c.notices.flush(&out)
	got := out.String()
	if !strings.Contains(got, "carries no record") || !strings.Contains(got, "2 backend notice(s)") {
		t.Errorf("the flush does not hand back what the screen collected:\n%s", got)
	}
	// A cockpit with nothing to say prints nothing at all.
	c2, _, _ := cnCockpit(t)
	c2.refreshSessions()
	var quiet bytes.Buffer
	c2.notices.flush(&quiet)
	if quiet.Len() != 0 {
		t.Errorf("a quiet cockpit printed on the way out:\n%s", quiet.String())
	}

	src, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	at := strings.Index(body, "func runCockpit(")
	if at < 0 {
		t.Fatal("no runCockpit in cockpit.go — this pin is reading the wrong file")
	}
	body = body[at:]
	flush := strings.Index(body, "defer c.notices.flush(os.Stderr)")
	alt := strings.Index(body, "1049h")
	switch {
	case flush < 0:
		t.Error("runCockpit does not flush the notices on the way out: a cockpit quit without a `w` loses every line the backend wrote (ranger-base-2vhqo)")
	case alt < 0:
		t.Error("runCockpit no longer enters the alt screen — the premise of this whole bead is gone, re-read it")
	case flush > alt:
		t.Error("the flush is deferred BELOW the alt screen's restore, so LIFO runs it while the screen is still up and the lines land in a frame that is then thrown away")
	}
}
