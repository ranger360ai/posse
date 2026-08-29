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
func TestCageContinuationsMatchTheDescriptionColumn(t *testing.T) {
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
