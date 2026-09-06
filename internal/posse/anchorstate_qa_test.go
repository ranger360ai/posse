//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for the anchor-state line (ranger-base-xevp7, ADR 0015 §3
// verification item 9). anchorstate.go's whole product is bytes an operator
// reads, so what is pinned is the bytes of each state and the fact that the
// watch preamble prints exactly one of them — plus, at every state, that
// nothing about the launch moved.
//
// The (nil, nil) branch is the reason this line exists AND the reason it may
// never grow a verdict: an absent manifest still launches, and the arm below
// asserts that at the same moment it asserts the line.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The three states the ADR names, plus the two shapes a real manifest can
// take that none of the three describes — an unreadable file and a promotion
// that could not name its commit. Neither may render as one of the three.
//
// MUTATION: return the same string from any two arms → red.
func TestQAAnchorStateLineRendersEachState(t *testing.T) {
	t.Parallel()
	seeded := &PromoteManifest{Version: promoteManifestVersion, PromotedAt: "2026-08-31T09:00:00Z", Seeded: true}
	promoted := &PromoteManifest{Version: promoteManifestVersion, PromotedAt: "2026-09-01T10:11:12Z", SHA: "0123456789abcdef0123456789abcdef01234567"}
	for _, c := range []struct {
		name string
		m    *PromoteManifest
		err  error
		want string
	}{
		{"never promoted", nil, nil, "constitution: never promoted — no promoted.json"},
		{"seeded", seeded, nil, "constitution: seeded 2026-08-31T09:00:00Z"},
		{"promoted", promoted, nil, "constitution: promoted 0123456789ab 2026-09-01T10:11:12Z"},
		// The short sha is the one promote.go already prints everywhere
		// else (short()), so a manifest and a promote log name the same
		// prefix and one grep finds both.
		{"promoted with no commit recorded", &PromoteManifest{PromotedAt: "2026-09-01T10:11:12Z"}, nil,
			"constitution: promoted 2026-09-01T10:11:12Z — the manifest records no commit"},
		{"unreadable", nil, Die("promoted.json is not readable as a promote manifest: unexpected end of JSON input"),
			"constitution: promoted.json is not readable as a promote manifest: unexpected end of JSON input"},
		// A seeded manifest is seeded on Seeded alone. If a future writer
		// ever stamps both, the honest reading is still "no commit was
		// promoted here" — the seed is what the anchor records.
		{"seeded outranks a sha", &PromoteManifest{PromotedAt: "2026-08-31T09:00:00Z", Seeded: true, SHA: "0123456789abcdef"}, nil,
			"constitution: seeded 2026-08-31T09:00:00Z"},
		// Not a state of its own: a manifest with no timestamp must not
		// render a blank where a date belongs.
		{"no date recorded", &PromoteManifest{Seeded: true}, nil, "constitution: seeded (no date recorded)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := AnchorStateLine(c.m, c.err); got != c.want {
				t.Errorf("AnchorStateLine:\n got %q\nwant %q", got, c.want)
			}
		})
	}
	// And the error arm wins over a manifest, because a reader that reported
	// the state of a file it could not parse would be reporting a zero value.
	if got := AnchorStateLine(promoted, Die("boom")); !strings.Contains(got, "boom") {
		t.Errorf("an unreadable manifest must say so even when a value came back with it: %q", got)
	}
}

// The reader, against real files: the states are reachable from a home on
// disk and not only from a struct literal a test built.
//
// MUTATION: drop the ReadPromoteManifest call from ReportAnchorState → red.
func TestQAReportAnchorStateReadsTheHome(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := hermetic(t, NewAppAt(home))

	var b strings.Builder
	a.ReportAnchorState(&b)
	if got := strings.TrimSpace(b.String()); got != "constitution: never promoted — no promoted.json" {
		t.Errorf("a home with no manifest: %q", got)
	}

	// Seeded, written by the real seeder rather than by hand.
	if err := os.WriteFile(a.ConfigPath, []byte("default_dir: ~\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil || !m.Seeded {
		t.Fatalf("the seeder wrote no seeded manifest: %v %v", m, err)
	}
	b.Reset()
	a.ReportAnchorState(&b)
	if got := strings.TrimSpace(b.String()); got != "constitution: seeded "+m.PromotedAt {
		t.Errorf("a seeded home: %q", got)
	}

	// Promoted, and then unreadable — the two remaining shapes, on the same
	// home, so nothing here can pass because a fixture failed to build.
	writeAnchorManifest(t, a, &PromoteManifest{Version: promoteManifestVersion, PromotedAt: "2026-09-02T00:00:00Z", SHA: "abcdefabcdefabcdefabcdef"})
	b.Reset()
	a.ReportAnchorState(&b)
	if got := strings.TrimSpace(b.String()); got != "constitution: promoted abcdefabcdef 2026-09-02T00:00:00Z" {
		t.Errorf("a promoted home: %q", got)
	}
	if err := os.WriteFile(a.PromoteManifestPath(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	a.ReportAnchorState(&b)
	got := strings.TrimSpace(b.String())
	if !strings.HasPrefix(got, "constitution: ") || !strings.Contains(got, "not readable as a promote manifest") {
		t.Errorf("an unreadable manifest: %q", got)
	}
	if strings.Count(got, "\n") != 0 {
		t.Errorf("the error must be one line, not the whole complaint:\n%s", got)
	}
}

// The line is really ON the watch preamble, once, in every state — and the
// state it reports changes nothing about what the loop does. A cancelled
// context runs zero passes, so everything in d.Out is the preamble's.
//
// MUTATION: drop the ReportAnchorState call from Watch → red.
func TestQAWatchPreamblePrintsExactlyOneAnchorStateLine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		man  *PromoteManifest // nil writes no manifest at all
		want string
	}{
		{"never promoted", nil, "constitution: never promoted — no promoted.json"},
		{"seeded", &PromoteManifest{Version: promoteManifestVersion, PromotedAt: "2026-08-31T09:00:00Z", Seeded: true},
			"constitution: seeded 2026-08-31T09:00:00Z"},
		{"promoted", &PromoteManifest{Version: promoteManifestVersion, PromotedAt: "2026-09-01T10:11:12Z", SHA: "0123456789abcdef0123456789abcdef01234567"},
			"constitution: promoted 0123456789ab 2026-09-01T10:11:12Z"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			qaRepo(t, b.App, `[]`, "")
			if c.man != nil {
				writeAnchorManifest(t, b.App, c.man)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 10*time.Millisecond); err != nil {
				t.Fatalf("the anchor state must not decide whether the loop runs: %v", err)
			}
			out := dispatcherOut(d)
			var said []string
			for _, ln := range strings.Split(out, "\n") {
				if strings.HasPrefix(ln, "constitution: ") {
					said = append(said, ln)
				}
			}
			if len(said) != 1 {
				t.Fatalf("the preamble says the anchor state %d times, want exactly 1:\n%s", len(said), out)
			}
			if said[0] != c.want {
				t.Errorf("preamble line:\n got %q\nwant %q", said[0], c.want)
			}
		})
	}
}

// Item 9's other half, and the boundary the ADR draws in capitals: the line
// reports, and the launch verify is what decides. Absence still launches
// (the (nil, nil) branch), and a mismatch under a manifest the line reports
// as `promoted` still DEGRADEs — so this reader can never be read as an
// all-clear or as a refusal.
//
// MUTATION: make ReportAnchorState return a verdict Watch acts on, or make
// AnchorStateLine's nil arm degrade → this and the arm above go red together.
func TestQAAnchorStateChangesNoLaunchBehaviour(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := hermetic(t, NewAppAt(home))
	if err := os.WriteFile(a.ConfigPath, []byte("default_dir: ~\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Absence: reported loudly, verified silently. Both at once, because the
	// whole risk in this bead is one becoming the other.
	var b strings.Builder
	a.ReportAnchorState(&b)
	if !strings.Contains(b.String(), "never promoted") {
		t.Fatalf("fixture: the home is not in the absent state: %q", b.String())
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("an absent manifest must still launch: %s", v.Line())
	}

	// And a mismatch under a manifest that reads `promoted` is still a
	// mismatch: the line names the state, VerifyPromoted judges it.
	writeAnchorManifest(t, a, &PromoteManifest{
		Version:    promoteManifestVersion,
		PromotedAt: "2026-09-01T10:11:12Z",
		SHA:        "0123456789abcdef0123456789abcdef01234567",
		Set:        append([]string{}, PromotedPaths...),
		Files:      map[string]string{"config.yaml": strings.Repeat("0", 64)},
	})
	b.Reset()
	a.ReportAnchorState(&b)
	if !strings.Contains(b.String(), "constitution: promoted 0123456789ab") {
		t.Fatalf("fixture: the tampered home does not read as promoted: %q", b.String())
	}
	if v := a.VerifyPromoted(); v.OK() {
		t.Error("a home whose config.yaml does not match its manifest must not verify clean")
	}
}

func writeAnchorManifest(t *testing.T, a *App, m *PromoteManifest) {
	t.Helper()
	if m.Files == nil {
		m.Files = map[string]string{}
	}
	blob, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(a.PromoteManifestPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.PromoteManifestPath(), append(blob, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
