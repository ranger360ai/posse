package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// ranger-base-w2uoe: the operator's report was "a lot of flicker on posse
// cockpit, seems like a refresh overboard". Three causes, and these pin two
// of them by their bytes — the third (DEC 2026) rides the same assertion
// because it is the same frame envelope.
//
// What flickers is the gap between an erase and the paint that follows it:
// render used to open every frame with ESC[2J ESC[H, so the terminal held a
// BLANK screen until the new frame arrived. The frame now homes and
// overwrites, erasing each row as it lands on it.

// flickerFixture is a cockpit with something in every section and a frozen
// clock. The clock matters: the header carries HH:MM:SS, so a frame is only
// equal to the one before it if both were rendered inside the same second —
// a real dedupe test cannot be left to the wall clock.
func flickerFixture() *cockpit {
	at := time.Date(2026, 9, 4, 11, 22, 33, 0, time.UTC)
	return &cockpit{
		now: func() time.Time { return at },
		sessions: []posse.HerdrSession{
			{Name: "developer-ranger-base-w2uoe", Agent: "developer", Status: "working", Dir: "/w"},
			{Name: "devops-ranger-base-h3n", Agent: "devops", Status: "idle", Dir: "/w"},
		},
		issues: []posse.RepoIssue{{BdIssue: posse.BdIssue{ID: "ranger-base-x01", Title: "a ready bead"}, Dir: "/r"}},
		status: "ready",
	}
}

// The frame never blanks: no ESC[2J anywhere in it, every line carries its
// own ESC[K, and the whole thing is wrapped in the DEC 2026 pair so the
// terminals that can present it atomically do.
func TestCockpitFrameErasesPerLineAndNeverClears(t *testing.T) {
	c := flickerFixture()
	for _, tc := range []struct {
		name  string
		frame string
		nl    string
	}{
		{"tty", c.render(80, 24), "\r\n"},
		{"plain", plainFrame(), "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.frame, "\033[2J") {
				t.Errorf("frame clears the whole display — that blank IS the flicker:\n%q", tc.frame)
			}
			if !strings.HasPrefix(tc.frame, "\033[?2026h\033[H") {
				t.Errorf("frame must open synchronized and homed, got %q", frameEnd(tc.frame, 24, false))
			}
			if !strings.Contains(tc.frame, "\033[J") {
				t.Error("frame must close with ESC[J — a shorter frame leaves the old tail on the glass")
			}
			if !strings.HasSuffix(strings.TrimSuffix(tc.frame, "\n"), "\033[?2026l") {
				t.Errorf("frame must end the synchronized update, got %q", frameEnd(tc.frame, 24, true))
			}
			body := strings.TrimSuffix(strings.TrimPrefix(tc.frame, "\033[?2026h\033[H"), "\n")
			body = strings.TrimSuffix(body, "\033[?2026l")
			body = strings.TrimSuffix(body, "\033[J")
			lines := strings.Split(body, tc.nl)
			if len(lines) < 4 {
				t.Fatalf("fixture drew %d lines — too few to be measuring anything", len(lines))
			}
			for i, ln := range lines {
				if !strings.HasSuffix(ln, "\033[K") {
					t.Errorf("line %d has no ESC[K, so the row it overwrites keeps its old tail: %q", i, ln)
				}
			}
		})
	}
}

// plainFrame is the non-tty frame's bytes. drawPlain writes through the same
// dedupe as every other path, so this asks a FRESH cockpit — reusing one
// already drawn would measure the dedupe rather than the frame.
func plainFrame() string {
	var b strings.Builder
	c := flickerFixture()
	c.out = &b
	c.drawPlain()
	return b.String()
}

// frameEnd is the first or last n bytes of a frame, for an error that must
// show the envelope without dumping the whole screen into the log.
func frameEnd(s string, n int, fromTail bool) string {
	if len(s) <= n {
		return s
	}
	if fromTail {
		return s[len(s)-n:]
	}
	return s[:n]
}

// An identical frame is an identical screen (ADR 0004 §5), so it is not
// written. draw has fourteen call sites and every one of them repainted
// unconditionally; on a quiet shop most of what they ask for is already up.
func TestCockpitIdenticalFrameIsNotWritten(t *testing.T) {
	var b strings.Builder
	c := flickerFixture()
	c.out = &b

	c.draw()
	first := b.Len()
	if first == 0 {
		t.Fatal("the first frame must be written — there is nothing on the glass yet")
	}

	b.Reset()
	c.draw()
	c.draw()
	c.draw()
	if b.Len() != 0 {
		t.Errorf("three unchanged repaints wrote %d bytes, want 0:\n%q", b.Len(), b.String())
	}

	// And the dedupe is not a mute: a model change still reaches the glass.
	b.Reset()
	c.status = "dispatched developer"
	c.draw()
	if b.Len() == 0 {
		t.Fatal("a changed frame was dropped — the dedupe is holding a stale screen")
	}
	if !strings.Contains(b.String(), "dispatched developer") {
		t.Errorf("the changed frame must carry the change:\n%q", b.String())
	}

	// Back to the frame before it: still a write, because the glass is
	// showing the one in between. lastFrame is the SCREEN, not a set.
	b.Reset()
	c.status = "ready"
	c.draw()
	if b.Len() == 0 {
		t.Error("reverting to an earlier frame must repaint — the glass holds the frame in between, not this one")
	}
}

// The non-tty loop pays the same rent: `posse cockpit` piped into a file
// with nothing moving must stop adding bytes to it.
func TestCockpitPlainFrameDedupes(t *testing.T) {
	var b strings.Builder
	c := flickerFixture()
	c.out = &b

	c.drawPlain()
	if b.Len() == 0 {
		t.Fatal("the first plain frame must be written")
	}
	b.Reset()
	c.drawPlain()
	c.drawPlain()
	if b.Len() != 0 {
		t.Errorf("two unchanged plain frames wrote %d bytes, want 0:\n%q", b.Len(), b.String())
	}
}

// sgrRE is the colour half of the escape alphabet — the SGR sequences, not
// the cursor and erase ones the frame envelope is made of.
var sgrRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// ESC[K erases with the CURRENT attributes, which is the one new coupling
// this frame shape creates (ranger-base-w2uoe): a line that ends with a
// colour still open would have the rest of its row erased in that colour,
// a coloured bar to the right edge that redraws on every frame. ESC[2J
// never showed it because it ran before any of the frame's own SGR.
//
// So every line must close what it opens. The fixture is the hostile one —
// emoji, a runtime tag, titles long enough to truncate at every width —
// swept across the widths where truncation happens, in all three modes,
// because a cut through the middle of a colour region is the way this
// breaks.
func TestCockpitNoLineEndsWithAnOpenColour(t *testing.T) {
	c := qaFixture()
	c.peekText = "peek body\nsecond line\nthird"
	cut := 0
	for w := 12; w <= 160; w++ {
		for _, mode := range []cockpitMode{modeNormal, modePeek, modePrompt} {
			c.mode = mode
			for i, ln := range c.renderLines(w, 24) {
				sgr := sgrRE.FindAllString(ln, -1)
				if len(sgr) == 0 {
					continue
				}
				cut++
				if last := sgr[len(sgr)-1]; last != "\033[0m" {
					t.Fatalf("w=%d mode=%d line %d ends on %q, so ESC[K paints its tail in that colour: %q",
						w, mode, i, last, ln)
				}
			}
		}
	}
	if cut < 100 {
		t.Fatalf("only %d coloured lines swept — the fixture stopped colouring anything and this pin measures nothing", cut)
	}
}
