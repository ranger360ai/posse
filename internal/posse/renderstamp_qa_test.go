//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-vso72 item 1 — every wall rendered at launch names
// the binary that rendered it.
//
// THE GAP. `;; posse seatbelt for bob — rendered from the PID at launch`
// names the PID and the launch and not the renderer, so a seat that reads its
// own profile and does not find the deny it expects cannot tell "old wall,
// relaunch pending" from "old wall, INSTALL pending". Three closes on
// ranger-base-prjck said the fix "reaches a seat at its next launch"; twelve
// caged launches then rendered the old wall, because the installed binary
// predated it by 34 commits (ranger-base-mmhhc).
//
// The pin is the literal: each render's header must carry VersionString().
// internal/treepins holds the other half — that a NEW render site cannot
// ship without the stamp.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renderedBySubstrings is what every stamped header must contain: the clause
// one writer produces, and the version inside it. Both, because a header that
// carried the clause with an empty version would pass a test for either one.
func renderedBySubstrings(t *testing.T) []string {
	t.Helper()
	v := VersionString()
	if v == "" {
		t.Fatal("VersionString() is empty; there is nothing for a header to name")
	}
	return []string{RenderedByPosse(), v}
}

func TestSeatbeltHeaderNamesTheBinaryThatRenderedIt(t *testing.T) {
	t.Parallel()
	prof := SeatbeltProfile("bob", []string{"/tmp/x"}, nil, SeatbeltCarveOut{})
	head := strings.SplitN(prof, "\n", 2)[0]
	for _, want := range renderedBySubstrings(t) {
		if !strings.Contains(head, want) {
			t.Fatalf("seatbelt header does not name the renderer (%q): %s", want, head)
		}
	}
	// The header the seat already knows how to find stays findable: it is
	// still an SBPL comment, still line one, and still names the persona and
	// the launch. A stamp that displaced any of those would be a second
	// change wearing this one's bead.
	for _, want := range []string{";; posse seatbelt for bob", "from the PID at launch", "do not edit", "rangerhq-5vt"} {
		if !strings.Contains(head, want) {
			t.Fatalf("seatbelt header lost %q: %s", want, head)
		}
	}
}

func TestGateShimHeaderNamesTheBinaryThatRenderedIt(t *testing.T) {
	t.Parallel()
	body := renderShim("bob", "git", "/usr/bin/git", "/tmp/refusals.log", "/bin/date",
		ParseShimRules([]string{"Bash(git push:*)"})["git"])
	for _, want := range renderedBySubstrings(t) {
		if !strings.Contains(body, want) {
			t.Fatalf("shim header does not name the renderer (%q):\n%s", want, body)
		}
	}
	// THE CONSTRAINT this stamp had to fit inside. gateShimTarget reads the
	// first 512 bytes and looks for the marker in the first TWO lines, and
	// scripts/verify-bd-pin.sh greps `head -2` for the same needle — so the
	// clause goes BEFORE the marker on line 2, never on a line of its own,
	// and line 2 stays short enough to be inside the window.
	lines := strings.Split(body, "\n")
	if len(lines) < 2 {
		t.Fatalf("shim has no header line: %q", body)
	}
	if !strings.Contains(lines[1], gateShimMarker) {
		t.Fatalf("the marker left line 2: %q", lines[1])
	}
	if n := len(lines[0]) + 1 + len(lines[1]); n > 512 {
		t.Fatalf("the first two lines are %d bytes; gateShimTarget only reads 512", n)
	}
	// And the reader still reads it: a shim whose header no longer parses is
	// a `posse` that RunningPosse calls a foreign binary on every PATH check.
	dir := t.TempDir()
	shim := filepath.Join(dir, "git")
	if err := WriteExecutable(shim, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	target, ok := gateShimTarget(shim)
	if !ok || target != "/usr/bin/git" {
		t.Fatalf("gateShimTarget(%s) = %q, %v; want /usr/bin/git, true", shim, target, ok)
	}
}

func TestGateShellHeaderNamesTheBinaryThatRenderedIt(t *testing.T) {
	t.Parallel()
	gatesDir := t.TempDir()
	wrapper, err := writeGateShell("bob", gatesDir, filepath.Join(gatesDir, "bin"), "/bin/sh", "sh")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, want := range renderedBySubstrings(t) {
		if !strings.Contains(body, want) {
			t.Fatalf("gate shell does not name the renderer (%q):\n%s", want, rsHead(body, 3))
		}
	}
	// A placeholder that survives the render is a header claiming to name a
	// renderer and naming a token — worse than the half-sentence it replaced.
	if strings.Contains(body, "__RENDERER__") {
		t.Fatalf("__RENDERER__ was not substituted:\n%s", rsHead(body, 3))
	}
	if !strings.Contains(body, gateShellMarker) {
		t.Fatalf("the gate shell marker is gone — isGateWrapper would stop recognising it:\n%s", rsHead(body, 3))
	}
}

// One writer for the clause, so one grep over a box's state dir finds every
// wall a stale binary rendered. This is why the header sites call
// RenderedByPosse() rather than each composing "posse " + VersionString().
func TestRenderedByPosseIsOneSpelling(t *testing.T) {
	t.Parallel()
	if got := RenderedByPosse(); got != "rendered by posse "+VersionString() {
		t.Fatalf("RenderedByPosse() = %q", got)
	}
}

func rsHead(s string, n int) string {
	parts := strings.SplitN(s, "\n", n+1)
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, "\n")
}
