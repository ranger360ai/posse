package treepins

// QA pin for ranger-base-et7bs and ranger-base-esz16 — the three live-box pin
// gates must not move the Homebrew data they read, and must not report what
// they read off the box.
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
// HOMEBREW_NO_ANALYTICS is the second guard and the one whose footprint
// leaves the box (ranger-base-w2bha, ruled SUPPRESS; applied under
// ranger-base-esz16). Analytics are ON for brew here —
// `homebrew.analyticsdisabled` is unset in HOMEBREW_REPOSITORY's git config,
// MEASURED 2026-10-31, which is brew's default — so with the variable unset
// each brew read in these gates POSTs a command_run event to Homebrew's
// InfluxDB. The source audience of a crew-run check is this box, that
// endpoint is wider, and nothing on the box rules the wider one acceptable,
// so it does not move: crew hard risk line 4, which is the default the line
// already sets rather than an accepted risk. The box-wide `brew analytics`
// setting is the operator's own, for his own brew use, and no gate touches
// it — the fix is per-process and nothing else.
//
// Both guards are asserted the same way and for the same reason: a gate must
// not change the box it reads, and must not tell anyone else what it read.
// The three arms live in their subjects' own files (grokpin_qa_test.go,
// codexpin_qa_test.go, bdpin_qa_test.go) because that is where a seat looks
// for a pin about one script; the reasoning and the reading live here once,
// because three copies of it is how the wrong mechanism got told twice. Their
// names date from the first guard and are not renamed with the second: the
// pin is the helper below, and history that cites those names by spelling
// (docs/notes.d/ranger-base-et7bs.md) stays readable.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// bgBrewWord matches a lowercase `brew` as a word. It cannot match inside
// HOMEBREW_NO_AUTO_UPDATE (uppercase) or inside `homebrew` (no word boundary
// before the b), so the export line and the Homebrew prefix in a path are not
// read as invocations.
var bgBrewWord = regexp.MustCompile(`\bbrew\b`)

// bgGuards — every setting a brew-reading gate has to export, with what goes
// wrong when it does not. Adding a row here is the whole of adding a guard:
// the three arms and the control below read this list, so a new one is
// demanded of all three scripts at once and cannot be half-applied.
var bgGuards = []struct{ assign, cost string }{
	{"HOMEBREW_NO_ANALYTICS=1", "unguarded, each of its %d brew read(s) POSTs a command_run event to Homebrew's InfluxDB — analytics are on for brew on this box — and a crew-run check's data does not move to an audience wider than the box it came from (crew hard risk line 4; ranger-base-w2bha, ranger-base-esz16)"},
	{"HOMEBREW_NO_AUTO_UPDATE=1", "unguarded, its %d brew read(s) re-download Homebrew's ~16 MB package JSON when the cache is over 7 days old, and verify-box.sh runs this gate beside two others that answer out of that same cache (ranger-base-et7bs)"},
}

// bgRead reads <script> and returns, for each setting in bgGuards, the 1-based
// line of the first runnable `export ...<setting>` (absent from the map if
// there is none), plus every runnable line that names brew, in order.
//
// Comment lines are dropped first — every one of these scripts explains brew's
// behaviour in prose at the top, and that prose is not a call.
//
// The brew match is deliberately over-inclusive: any runnable line NAMING brew
// counts, including a `command -v brew` and a `brew pin` quoted inside an
// operator-facing heredoc. Over-inclusive can only demand the exports sit
// higher, never let a real call slip above one, so the looseness cannot make
// this pin pass a script it should fail.
//
// It is a plain function and not the assertion so that the control below can
// run it over fixtures and read its answer. An assertion that only ever ran
// over three scripts that pass is a scan nobody has shown can say no.
func bgRead(script string) (exports map[string]int, mentions []string, err error) {
	b, err := os.ReadFile(script)
	if err != nil {
		return nil, nil, err
	}
	exports = make(map[string]int, len(bgGuards))
	for i, line := range strings.Split(string(b), "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		if strings.HasPrefix(code, "export ") {
			// One export line may carry several of them, which is how all
			// three scripts spell it, so every setting on the line is
			// recorded — not the first match and done.
			isGuard := false
			for _, g := range bgGuards {
				if !strings.Contains(code, g.assign) {
					continue
				}
				isGuard = true
				if _, seen := exports[g.assign]; !seen {
					exports[g.assign] = i + 1
				}
			}
			if isGuard {
				continue
			}
		}
		if bgBrewWord.MatchString(code) {
			mentions = append(mentions, fmt.Sprintf("%s:%d: %s", script, i+1, code))
		}
	}
	return exports, mentions, nil
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

// bgAssertBrewReadsAreGuarded asserts that <script> exports every setting in
// bgGuards on a runnable line, and that each export sits ABOVE every runnable
// line that names brew.
//
// Position is the half that matters and the half a `strings.Contains` cannot
// see: an export at the bottom of the file would satisfy "the script sets it"
// and guard nothing.
func bgAssertBrewReadsAreGuarded(t *testing.T, script string) {
	t.Helper()
	exports, mentions, err := bgRead(script)
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

	for _, g := range bgGuards {
		line, ok := exports[g.assign]
		if !ok {
			t.Errorf("%s must `export %s` — %s. First read is %s",
				script, g.assign, fmt.Sprintf(g.cost, len(mentions)), mentions[0])
			continue
		}
		if line > first {
			t.Errorf("%s exports %s at line %d, BELOW its first brew read at line %d — the export has to precede the calls it guards or it guards nothing:\n  %s",
				script, g.assign, line, first, mentions[0])
		}
	}
}

// The control. The three arms above all pass, which tells a reader nothing
// about what this pin would catch, so each way of failing gets a fixture and
// each way of being allowed gets one too. The wrong arms are the real shapes:
// no export at all (what verify-codex-pin.sh and verify-bd-pin.sh were before
// ranger-base-et7bs), an export that is present but sits below the calls —
// which a `strings.Contains` of the tree would have called green — and one
// guard set while the other is missing, which is what the whole tree looked
// like between ranger-base-et7bs and ranger-base-esz16.
func TestQATheBrewGuardReaderCanStillSayNo(t *testing.T) {
	const (
		guard     = "export HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1"
		autoOnly  = "export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1"
		analytics = "HOMEBREW_NO_ANALYTICS=1"
		autoUpd   = "HOMEBREW_NO_AUTO_UPDATE=1"
	)
	cases := []struct {
		name        string
		body        string
		wantExports map[string]int
		wantFirst   int // 0 == no brew mention at all
	}{
		{"both guarded above the call", "#!/bin/bash\n" + guard + "\nv=$(brew info --cask codex)\n",
			map[string]int{analytics: 2, autoUpd: 2}, 3},
		{"no export at all", "#!/bin/bash\nv=$(brew info --cask codex)\n",
			map[string]int{}, 2},
		{"both exported below the call", "#!/bin/bash\nv=$(brew info --cask codex)\n" + guard + "\n",
			map[string]int{analytics: 3, autoUpd: 3}, 2},
		{"auto-update only — the pre-esz16 tree", "#!/bin/bash\n" + autoOnly + "\nv=$(brew list --pinned)\n",
			map[string]int{autoUpd: 2}, 3},
		{"analytics only", "#!/bin/bash\nexport " + analytics + "\nv=$(brew list --pinned)\n",
			map[string]int{analytics: 2}, 3},
		{"the two on separate lines, one of them below the call",
			"#!/bin/bash\nexport " + analytics + "\nv=$(brew --prefix)\nexport " + autoUpd + "\n",
			map[string]int{analytics: 2, autoUpd: 4}, 3},
		{"a commented export is not a setting", "#!/bin/bash\n# " + guard + "\nv=$(brew list --pinned)\n",
			map[string]int{}, 3},
		{"a bare assignment is not an export", "#!/bin/bash\n" + analytics + " " + autoUpd + "\nv=$(brew --prefix)\n",
			map[string]int{}, 3},
		{"prose about brew is not a call", "#!/bin/bash\n# brew info is auto-update-able, it says here\n" + guard + "\n",
			map[string]int{analytics: 3, autoUpd: 3}, 0},
		{"the export line is not itself a brew call", "#!/bin/bash\n" + guard + "\n",
			map[string]int{analytics: 2, autoUpd: 2}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "fixture.sh")
			if err := os.WriteFile(f, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			exports, mentions, err := bgRead(f)
			if err != nil {
				t.Fatal(err)
			}
			if got := bgFmtExports(exports); got != bgFmtExports(c.wantExports) {
				t.Errorf("exports = %s, want %s\n%s", got, bgFmtExports(c.wantExports), c.body)
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

// bgFmtExports — a stable rendering of the export map, so the control can
// compare two of them as text and print the difference in one line.
func bgFmtExports(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s@%d", k, m[k]))
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// Every setting in bgGuards must be demanded of the live scripts by name, and
// the two the scripts carry today are the two the reasoning above explains. A
// row added to bgGuards without a reason written down is a gate nobody can
// audit, and a row deleted is a guard that silently stops being asserted
// while all three arms stay green.
func TestQABrewGuardListIsTheTwoMeasuredSettings(t *testing.T) {
	want := []string{"HOMEBREW_NO_ANALYTICS=1", "HOMEBREW_NO_AUTO_UPDATE=1"}
	if len(bgGuards) != len(want) {
		t.Fatalf("bgGuards has %d entries, want %d — a guard was added or removed without updating this file's head comment, which is the only place the reason for each one is written (%v)", len(bgGuards), len(want), bgGuards)
	}
	for i, g := range bgGuards {
		if g.assign != want[i] {
			t.Errorf("bgGuards[%d] = %q, want %q", i, g.assign, want[i])
		}
		if !strings.Contains(g.cost, "%d") {
			t.Errorf("bgGuards[%d] (%s) cost text must carry one %%d for the brew-read count: %q", i, g.assign, g.cost)
		}
	}
}
