package treepins

// QA pin for ranger-base-et7bs — the three live-box pin gates must not move
// the Homebrew data they read.
//
// scripts/verify-box.sh runs verify-grok-pin.sh, verify-codex-pin.sh and
// verify-bd-pin.sh back to back (its roster, in that order), and all three
// answer their version rows out of ONE thing: Homebrew's cached package JSON.
// With HOMEBREW_NO_AUTO_UPDATE unset, a `brew info` whose cached copy is over
// 7 days old re-downloads that file (~16 MB) before answering, so an
// unguarded read in the first gate can move the data the next two read — a
// read-only check with a footprint, and a footprint that lands on its
// siblings.
//
// THE MECHANISM IS THE API CACHE, NOT A TAP CLONE, and this pin says so
// because its first telling did not. `brew info` and `brew list` are NOT
// auto-update commands: Homebrew's own `setup-auto-update` names install,
// outdated, upgrade, bundle, release, `tap` with arguments and the bump
// family, and nothing else. MEASURED 2026-10-31 (Homebrew 7.0.11, this box)
// by running brew's own function for each verb, and again by running the
// three read verbs with the variable deliberately unset: no "Auto-updating
// Homebrew", Homebrew's version unchanged, every tap HEAD unchanged. The
// banner that the earlier telling quoted was a `brew upgrade --cask` two
// commands later on the same line. The arm that IS real was measured against
// a scratch HOMEBREW_CACHE backdated 8 days: guarded answered from the cache
// and left its mtime alone, unguarded printed "==> Downloading Homebrew API
// data" and rewrote it. docs/notes.d/ranger-base-et7bs.md carries both.
//
// The three arms live in their subjects' own files (grokpin_qa_test.go,
// codexpin_qa_test.go, bdpin_qa_test.go) because that is where a seat looks
// for a pin about one script; the reasoning and the reading live here once,
// because three copies of it is how the wrong mechanism got told twice.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// bgBrewWord matches a lowercase `brew` as a word. It cannot match inside
// HOMEBREW_NO_AUTO_UPDATE (uppercase) or inside `homebrew` (no word boundary
// before the b), so the export line and the Homebrew prefix in a path are not
// read as invocations.
var bgBrewWord = regexp.MustCompile(`\bbrew\b`)

// bgRead reads <script> and returns the 1-based line of its first runnable
// `export ...HOMEBREW_NO_AUTO_UPDATE=1` (0 if there is none) and every runnable
// line that names brew, in order.
//
// Comment lines are dropped first — every one of these scripts explains brew's
// behaviour in prose at the top, and that prose is not a call.
//
// The brew match is deliberately over-inclusive: any runnable line NAMING brew
// counts, including a `command -v brew` and a `brew pin` quoted inside an
// operator-facing heredoc. Over-inclusive can only demand the export sit
// higher, never let a real call slip above it, so the looseness cannot make
// this pin pass a script it should fail.
//
// It is a plain function and not the assertion so that the control below can
// run it over fixtures and read its answer. An assertion that only ever ran
// over three scripts that pass is a scan nobody has shown can say no.
func bgRead(script string) (export int, mentions []string, err error) {
	b, err := os.ReadFile(script)
	if err != nil {
		return 0, nil, err
	}
	for i, line := range strings.Split(string(b), "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		if export == 0 && strings.HasPrefix(code, "export ") &&
			strings.Contains(code, "HOMEBREW_NO_AUTO_UPDATE=1") {
			export = i + 1
			continue
		}
		if bgBrewWord.MatchString(code) {
			mentions = append(mentions, fmt.Sprintf("%s:%d: %s", script, i+1, code))
		}
	}
	return export, mentions, nil
}

// bgFirstLine — the 1-based line number out of a `path:line: text` mention.
func bgFirstLine(t *testing.T, mention string) int {
	t.Helper()
	parts := strings.SplitN(mention, ":", 3)
	if len(parts) < 3 {
		t.Fatalf("mention %q is not path:line:text — the reader's own format changed", mention)
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("mention %q carries no line number: %v", mention, err)
	}
	return n
}

// bgAssertBrewReadsAreGuarded asserts that <script> exports
// HOMEBREW_NO_AUTO_UPDATE=1 on a runnable line, and that the export sits ABOVE
// every runnable line that names brew.
//
// Position is the half that matters and the half a `strings.Contains` cannot
// see: an export at the bottom of the file would satisfy "the script sets it"
// and guard nothing.
func bgAssertBrewReadsAreGuarded(t *testing.T, script string) {
	t.Helper()
	export, mentions, err := bgRead(script)
	if err != nil {
		t.Fatalf("read %s: %v", script, err)
	}

	// The positive witness. A script that names brew nowhere needs no guard,
	// but these three all do, and a reader that stopped finding their calls
	// would leave the ordering check below vacuously true.
	if len(mentions) == 0 {
		t.Fatalf("%s: the scan found no runnable line naming brew — these gates read brew, so the scan has gone blind and a clean result here is not evidence", script)
	}
	first := bgFirstLine(t, mentions[0])

	if export == 0 {
		t.Errorf("%s must `export HOMEBREW_NO_AUTO_UPDATE=1` — unguarded, its %d brew read(s) re-download Homebrew's ~16 MB package JSON when the cache is over 7 days old, and verify-box.sh runs this gate beside two others that answer out of that same cache (ranger-base-et7bs). First read is %s",
			script, len(mentions), mentions[0])
		return
	}
	if export > first {
		t.Errorf("%s exports HOMEBREW_NO_AUTO_UPDATE=1 at line %d, BELOW its first brew read at line %d — the export has to precede the calls it guards or it guards nothing:\n  %s",
			script, export, first, mentions[0])
	}
}

// The control. The three arms above all pass, which tells a reader nothing
// about what this pin would catch, so each way of failing gets a fixture and
// each way of being allowed gets one too. The wrong arms are the two real
// shapes: no export at all (what verify-codex-pin.sh and verify-bd-pin.sh were
// before ranger-base-et7bs), and an export that is present but sits below the
// calls — which a `strings.Contains` of the tree would have called green.
func TestQATheBrewGuardReaderCanStillSayNo(t *testing.T) {
	const guard = "export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1"
	cases := []struct {
		name       string
		body       string
		wantExport int
		wantFirst  int // 0 == no brew mention at all
	}{
		{"guarded above the call", "#!/bin/bash\n" + guard + "\nv=$(brew info --cask codex)\n", 2, 3},
		{"no export at all", "#!/bin/bash\nv=$(brew info --cask codex)\n", 0, 2},
		{"export below the call", "#!/bin/bash\nv=$(brew info --cask codex)\n" + guard + "\n", 3, 2},
		{"a commented export is not a setting", "#!/bin/bash\n# " + guard + "\nv=$(brew list --pinned)\n", 0, 3},
		{"a bare assignment is not an export", "#!/bin/bash\nHOMEBREW_NO_AUTO_UPDATE=1\nv=$(brew --prefix)\n", 0, 3},
		{"prose about brew is not a call", "#!/bin/bash\n# brew info is auto-update-able, it says here\n" + guard + "\n", 3, 0},
		{"the export line is not itself a brew call", "#!/bin/bash\n" + guard + "\n", 2, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "fixture.sh")
			if err := os.WriteFile(f, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			export, mentions, err := bgRead(f)
			if err != nil {
				t.Fatal(err)
			}
			if export != c.wantExport {
				t.Errorf("export line = %d, want %d\n%s", export, c.wantExport, c.body)
			}
			first := 0
			if len(mentions) > 0 {
				first = bgFirstLine(t, mentions[0])
			}
			if first != c.wantFirst {
				t.Errorf("first brew line = %d, want %d (mentions %v)\n%s", first, c.wantFirst, mentions, c.body)
			}
		})
	}
}
