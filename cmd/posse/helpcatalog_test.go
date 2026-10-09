package main

// rangerhq-6izv: the catalog's continuation lines carry no key of their own,
// so a description can drift under the wrong command and still read as
// well-formed. '(cross-builds the Linux posse and bd it carries)' describes
// `posse cage build` but was printed under `posse cage down`. The pin is
// ordering plus the hand-maintained indent: a continuation belongs to the
// last command header above it, and only lands in that column when its
// padding matches the description column.

import (
	"io"
	"os"
	"strings"
	"testing"
)

// helpText runs help() and returns what it printed.
func helpText(t *testing.T) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	help()
	os.Stdout = saved
	w.Close()
	out := <-done
	r.Close()
	if out == "" {
		t.Fatal("help() printed nothing — the capture, not the catalog, is what failed")
	}
	return out
}

// commandOf returns the catalog command each continuation line belongs to:
// the nearest 'posse ...' header at or above it.
func commandOf(t *testing.T, out, continuation string) string {
	t.Helper()
	cmd := ""
	for _, ln := range strings.Split(out, "\n") {
		if head := strings.TrimPrefix(ln, "  posse "); head != ln {
			cmd = "posse " + strings.TrimSpace(head)
			if i := strings.Index(cmd, "  "); i >= 0 {
				cmd = strings.TrimSpace(cmd[:i])
			}
		}
		if strings.Contains(ln, continuation) {
			return cmd
		}
	}
	t.Fatalf("catalog has no line containing %q", continuation)
	return ""
}

func TestCageBuildContinuationSitsUnderCageBuild(t *testing.T) {
	out := helpText(t)

	// The control: a continuation whose owner nobody has ever disputed. If
	// commandOf cannot place this one, it cannot place the one under test.
	if got, want := commandOf(t, out, "the launch's own watcher does this when a cage exits"), "posse cage down <persona>"; got != want {
		t.Fatalf("control: watcher line reads as %q, want %q — commandOf is broken, not the catalog", got, want)
	}

	if got, want := commandOf(t, out, "cross-builds the Linux posse and bd it carries"), `posse cage build [dir] [--runtimes "<npm pkgs>"]`; got != want {
		t.Errorf("'(cross-builds …)' reads as a continuation of %q, want %q", got, want)
	}
}

// The indent is hand-maintained: 'posse' is two characters wider than 'rhq'
// was, so the description column is 33 and every continuation matches it.
//
// ranger-base-j120n widened this from four named cage lines to every
// continuation in the catalog. Hand-listing the lines it reads made the pin
// green over a class it does not cover: `posse gates install-hooks`' nine
// continuations sat at column 36 from the day they were written, in the same
// block as a pin that only ever looked at four cage lines three screens
// away. A pin whose subject is a COLUMN has to read every line in it, or the
// next drift is noticed by eye again or not at all.
//
// Scope is the one column the catalog actually holds uniform: the
// description column of a `posse <command>` entry — MEASURED 2026-10-09 over
// the 212 lines this walk reads, 209 continuations and 3 wrapped headers.
// The other indents in the catalog are organic and deliberately out of
// scope — the flags block under an entry carries its own column per block
// (31 under `posse kill`, 33 under `posse dispatch`, 46 under `posse new`,
// each set by the widest flag spelling there), `config <key>:` sub-entries
// open at catalogConfigLead, and samples under a flag are indented deeper on
// purpose. So the walk ends an entry exactly where catalogBlock's grammar
// does, plus at the flags block: entry header, config lead, section header,
// blank line, or a flag line at flagIndent.
//
// A header whose spelling wraps ('posse runtime probe <name>' continued by
// '[--timeout 4m]') is read from the other end: its own description has to
// open in the column.
//
// Entry HEADERS are not pinned, and cannot be: four of the catalog's 44
// headers with a description on the same line run past column 33 and take a
// single gap instead (`posse claim <id> [--as <persona>] [--dir <repo>]`).
// It is the continuation under them that always has the column free.
func TestCatalogContinuationsMatchTheDescriptionColumn(t *testing.T) {
	const (
		col        = 33
		flagIndent = 6
		// Below the smallest honest reading of the catalog: 212 when this
		// was written. A parser that stops seeing continuations — a renamed
		// constant, a reshaped catalog — would otherwise pass by checking
		// nothing at all.
		least = 150
	)
	checked := 0
	in := false
	for _, ln := range strings.Split(helpText(t), "\n") {
		trimmed := strings.TrimLeft(ln, " ")
		indent := len(ln) - len(trimmed)
		switch {
		case strings.HasPrefix(ln, catalogEntryIndent+"posse "):
			in = true
			continue
		case strings.HasPrefix(ln, catalogConfigLead),
			strings.TrimSpace(ln) == "",
			!strings.HasPrefix(ln, " "),
			indent == flagIndent:
			in = false
			continue
		}
		if !in {
			continue
		}
		checked++
		// A header too long for one line carries the rest of its SPELLING
		// down the left margin ('posse runtime probe <name>' then
		// '[--timeout 4m]'), and the description beside it still opens in
		// the column. Same invariant, read from the other end.
		if strings.HasPrefix(trimmed, "[") && indent < col {
			if len(ln) <= col || ln[col] == ' ' || !strings.HasSuffix(ln[:col], "  ") {
				t.Errorf("wrapped header's description does not open in column %d: %q", col, ln)
			}
			continue
		}
		// Whole-line comparison, not a prefix test: one extra leading space
		// still has the shorter indent in front of it, and a pin that asked
		// for a prefix would pass over it.
		if indent != col {
			t.Errorf("continuation is indented to column %d, want %d: %q", indent, col, ln)
		}
	}
	if checked < least {
		t.Errorf("walked %d continuation lines, want at least %d — the walk stopped reading the catalog, so a green here means nothing", checked, least)
	}
}

// The control for the walk above: the four cage lines this pin read by hand
// before ranger-base-j120n widened it. If the walk ever stops reaching them,
// it has stopped reaching the catalog.
func TestCageContinuationsAreAmongTheLinesWalked(t *testing.T) {
	const col = 33
	lines := strings.Split(helpText(t), "\n")
	for _, want := range []string{
		"caged launch of that persona would mount and forward",
		"build the cage image from a posse checkout",
		"(cross-builds the Linux posse and bd it carries)",
		"(the launch's own watcher does this when a cage exits)",
	} {
		// Whole-line equality, not Contains: one extra leading space still
		// contains the shorter indent, and the pin would pass over it.
		padded := strings.Repeat(" ", col) + want
		found := false
		for _, ln := range lines {
			if strings.Contains(ln, want) {
				found = true
				if ln != padded {
					t.Errorf("continuation is indented to column %d, want %d: %q", len(ln)-len(strings.TrimLeft(ln, " ")), col, ln)
				}
			}
		}
		if !found {
			t.Errorf("catalog has no line containing %q", want)
		}
	}
}
