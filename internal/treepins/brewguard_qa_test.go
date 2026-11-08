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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
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

// bgBlockOpen, bgBlockClose — the shell words that open and close a block,
// counted per runnable line so that `export` inside one is not read as a
// top-level setting. `elif` matches neither (no word boundary before its
// `if`); `then`, `do` and `else` are counted by neither, because the opener
// they belong to already was.
var (
	bgBlockOpen  = regexp.MustCompile(`\b(if|case|while|until|for)\b`)
	bgBlockClose = regexp.MustCompile(`\b(fi|esac|done)\b`)
)

// bgHeredoc matches a heredoc redirection and captures its `-` (if any) and
// its terminator word. It cannot match `$((x<<2))` (a digit is not a
// terminator's first character) or a `<<<` herestring (a `<` is not either).
var bgHeredoc = regexp.MustCompile(`<<(-?)\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

// bgRead reads <script> and returns, for each setting in bgGuards, the 1-based
// line of the first TOP-LEVEL `export ...<setting>` (absent from the map if
// there is none), the line of the first one it SAW and refused as unreachable
// (`shadowed`, so the diagnostic can say "it is there, at line N, and it does
// not run" rather than "it is missing"), plus every runnable line that names
// brew, in order.
//
// Comment lines are dropped first — every one of these scripts explains brew's
// behaviour in prose at the top, and that prose is not a call.
//
// THE TWO SIDES ARE LOOSE IN OPPOSITE DIRECTIONS, and that is the whole of
// what makes a line-based reader safe here.
//
// The brew match is deliberately over-inclusive: any runnable line NAMING brew
// counts, including a `command -v brew`, a trailing comment that mentions one,
// and a `brew pin` quoted inside an operator-facing heredoc. Over-inclusive
// can only demand the exports sit higher, never let a real call slip above
// one, so the looseness cannot make this pin pass a script it should fail.
//
// The EXPORT side runs the other way, and has to be UNDER-inclusive, because
// on this side a line that merely APPEARS is not a setting that HAPPENS.
// Until ranger-base-mk6gw this side was line-based too and had no notion of
// reachability: MEASURED 2026-10-31, wrapping verify-grok-pin.sh's export line
// in a function nothing calls left all five arms green — over a gate that
// exports nothing at runtime and so POSTs a command_run event to Homebrew's
// InfluxDB on every brew read it makes. An export is therefore recognized only
// where it is reached unconditionally — column 0, block depth 0, above the
// first parenthesis, outside any heredoc — and anything this reader cannot
// place there is not counted. A guard it fails to see can only make the pin
// stricter; one it imagines makes the pin a lie.
//
// Depth is a COUNT of block words and braces and not a parse, so it drifts upward on a
// keyword inside a string (`echo "for the record"`), which costs a false FAIL
// naming the line and never a false pass. Drift DOWNWARD is the unsafe
// direction, so a negative depth does not quietly become zero: from that line
// on, this reader will not call anything top level, and it says where it lost
// the count in `lost`. It keeps READING — the brew mentions below that line
// are the census's subject and a scan that stopped there would report a file
// with calls in it as a file with none — and it leaves what to do about it to
// the caller: the assertion refuses a guard it cannot place, the census over
// scripts/ never asks about exports at all. Three of this tree's scripts trip
// the count today and none of them reads brew (MEASURED 2026-10-31:
// queue-cutover.sh, verify-herdr-bob-rules.sh, responsibility-census.py).
//
// A PARENTHESIS IS NOT COUNTED, IT STOPS THE PLACING (ranger-base-80zqb). The
// brace-and-block-word count had no notion of `(` at all, so an export in a
// top-level subshell, or in a function whose body is a subshell, sat at column
// 0 with depth 0 and was recorded as a top-level guard — the ranger-base-f5lu1
// survivor re-spelled with the other bracket, against the fix that was filed
// to close it. MEASURED 2026-10-31 over verify-grok-pin.sh:80 with a stub brew
// recording its own environment: `(\nexport …\n)` and
// `never_called() (\nexport …\n)` each left HOMEBREW_NO_ANALYTICS and
// HOMEBREW_NO_AUTO_UPDATE UNSET in the brew child, while a plain top-level
// export and even a top-level `{ … }` group set both. The two brackets were
// treated exactly backwards from what they do, and the permissive one was the
// one that exports nothing.
//
// Counting `(` and `)` beside the braces is the obvious repair and it is the
// wrong one. A `case` pattern ends in a bare `)`, so the count goes negative on
// the first pattern of every argument parser in the tree: MEASURED 2026-10-31,
// counting parens took the files under scripts/ that LOSE the count from 3 of
// 56 to 38 of 56, and all four of bgRoster's callers were among the 38. The
// count would still be lost rather than wrong — the strict direction — but a
// reader that can place nothing below line ~120 of every gate is a reader whose
// strictness has stopped meaning anything.
//
// So a line carrying `(` or `)` is recorded as a structure this reader cannot
// place, and nothing from that line on is placed — that line included, because
// `( export X=1 )` and `export X=$(date)` are the same text to a line-based
// reader and only one of them runs. In effect the export has to sit in the run
// of lines above the first bracket of any kind, which is where all four of
// bgRoster's callers put it (MEASURED 2026-10-31: exports at :80, :62, :75 and
// :87; first parenthesis at :82, :64, :77 and :100). It costs the
// refused-though-reachable shapes TestQATheBrewGuardReaderCanStillSayNo now
// documents, and it buys a rule with no next uncounted construct, which the
// counting repair does not have.
//
// AND THE STATIC READING IS NO LONGER THE ONLY ONE. The repair above is still a
// parser learning one more bracket, so TestQABrewGuardsReachTheBrewChild below
// proves the same claim BY CONSTRUCTION for every gate that can be run: it runs
// the gate against a stub brew that records its own environment and reads the
// guards out of the CHILD, where an export the shell never executed simply is
// not there. That is option (b) of ranger-base-mk6gw's own finding 1, which its
// close did not take and did not say why. One rostered caller cannot be reached
// that way — bgNoDynamic names it and says why — and that caller is the reason
// this static reader is repaired rather than deleted.
//
// It is a plain function and not the assertion so that the control below can
// run it over fixtures and read its answer. An assertion that only ever ran
// over three scripts that pass is a scan nobody has shown can say no.
func bgRead(script string) (exports, shadowed map[string]int, mentions []string, lost string, err error) {
	b, err := os.ReadFile(script)
	if err != nil {
		return nil, nil, nil, "", err
	}
	exports = make(map[string]int, len(bgGuards))
	shadowed = make(map[string]int, len(bgGuards))
	depth := 0
	hereTerm, hereDash := "", false
	for i, line := range strings.Split(string(b), "\n") {
		code := strings.TrimSpace(line)
		if hereTerm != "" {
			// A heredoc body is data this script hands to something else, so
			// no export in it is a setting and no word in it is structure.
			// Its brew mentions are still taken — the over-inclusive side —
			// and that is what makes the loose heredoc match safe in BOTH
			// directions. A `<<` inside a quoted string opens a heredoc that
			// is not there, and everything to the terminator (or to EOF) is
			// then read as a body: exports in it stop counting, which is the
			// strict direction, and its brew lines are still censused, so
			// nothing a phantom body swallows goes missing from the roster.
			if line == hereTerm || (hereDash && code == hereTerm) {
				hereTerm = ""
			} else if bgBrewWord.MatchString(code) {
				mentions = append(mentions, fmt.Sprintf("%s:%d: %s", script, i+1, code))
			}
			continue
		}
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		// A parenthesis is a structure this reader cannot place — a subshell,
		// a function body, a `case` pattern and a command substitution are
		// one character to it — so it stops the placing here instead of being
		// counted. Set BEFORE atTop, so the line carrying it is refused too.
		// The head comment carries the measurement for why `(` is not counted
		// the way `{` is (ranger-base-80zqb).
		if lost == "" && strings.ContainsAny(code, "()") {
			lost = fmt.Sprintf("%s:%d: %q", script, i+1, code)
		}
		// Read before this line's own openers are counted: a line that OPENS
		// a block does not put its own head inside one, and the `export` in
		// `export X=1; if ...` is executed either way.
		atTop := depth == 0 && lost == "" && line == strings.TrimLeft(line, " \t")
		depth += len(bgBlockOpen.FindAllString(code, -1)) - len(bgBlockClose.FindAllString(code, -1))
		depth += strings.Count(code, "{") - strings.Count(code, "}")
		if depth < 0 {
			if lost == "" {
				lost = fmt.Sprintf("%s:%d: %q", script, i+1, code)
			}
			depth = 0
		}
		if m := bgHeredoc.FindStringSubmatch(code); m != nil {
			hereTerm, hereDash = m[2], m[1] == "-"
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
				if atTop {
					if _, seen := exports[g.assign]; !seen {
						exports[g.assign] = i + 1
					}
				} else if _, seen := shadowed[g.assign]; !seen {
					shadowed[g.assign] = i + 1
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
	return exports, shadowed, mentions, lost, nil
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
// bgGuards on a top-level runnable line, and that each export sits ABOVE every
// runnable line that names brew.
//
// Position is one half that matters and the half a `strings.Contains` cannot
// see: an export at the bottom of the file would satisfy "the script sets it"
// and guard nothing. REACHABILITY is the other half, and it is the half this
// pin did not have until ranger-base-mk6gw: an export in a function nothing
// calls also satisfies "the script sets it", sits anywhere its author likes,
// and guards nothing either. bgRead's head comment carries the measurement.
func bgAssertBrewReadsAreGuarded(t *testing.T, script string) {
	t.Helper()
	exports, shadowed, mentions, lost, err := bgRead(script)
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
			where := ""
			if n, seen := shadowed[g.assign]; seen {
				where = fmt.Sprintf(" It IS written at line %d, and that line does not run unconditionally — it is indented, inside a block, or inside a heredoc, so the brew reads below it are unguarded at runtime whatever the text says.", n)
			} else if lost != "" {
				where = fmt.Sprintf(" This reader also stopped placing lines at %s — an unbalanced block word or brace, or a parenthesis of any kind — and placed nothing from there on, so if the export is down there, that is why it was not seen: move the export above that line.", lost)
			}
			t.Errorf("%s must `export %s` at top level — %s.%s First read is %s",
				script, g.assign, fmt.Sprintf(g.cost, len(mentions)), where, mentions[0])
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
		wantShadow  map[string]int // the guards the reader SAW and refused
		wantFirst   int            // 0 == no brew mention at all
	}{
		{"both guarded above the call", "#!/usr/bin/env bash\n" + guard + "\nv=$(brew info --cask codex)\n",
			map[string]int{analytics: 2, autoUpd: 2}, nil, 3},
		{"no export at all", "#!/usr/bin/env bash\nv=$(brew info --cask codex)\n",
			map[string]int{}, nil, 2},
		// Spelled without a command substitution on purpose: a `$( )` above
		// the export would stop the reader placing it at all (ranger-base-80zqb),
		// and this row's subject is the ORDERING check below a call it CAN place.
		{"both exported below the call", "#!/usr/bin/env bash\nbrew info --cask codex >/dev/null\n" + guard + "\n",
			map[string]int{analytics: 3, autoUpd: 3}, nil, 2},
		{"auto-update only — the pre-esz16 tree", "#!/usr/bin/env bash\n" + autoOnly + "\nv=$(brew list --pinned)\n",
			map[string]int{autoUpd: 2}, nil, 3},
		{"analytics only", "#!/usr/bin/env bash\nexport " + analytics + "\nv=$(brew list --pinned)\n",
			map[string]int{analytics: 2}, nil, 3},
		{"the two on separate lines, one of them below the call",
			"#!/usr/bin/env bash\nexport " + analytics + "\nbrew --prefix >/dev/null\nexport " + autoUpd + "\n",
			map[string]int{analytics: 2, autoUpd: 4}, nil, 3},
		{"a commented export is not a setting", "#!/usr/bin/env bash\n# " + guard + "\nv=$(brew list --pinned)\n",
			map[string]int{}, nil, 3},
		{"a bare assignment is not an export", "#!/usr/bin/env bash\n" + analytics + " " + autoUpd + "\nv=$(brew --prefix)\n",
			map[string]int{}, nil, 3},
		{"prose about brew is not a call", "#!/usr/bin/env bash\n# brew info is auto-update-able, it says here\n" + guard + "\n",
			map[string]int{analytics: 3, autoUpd: 3}, nil, 0},
		{"the export line is not itself a brew call", "#!/usr/bin/env bash\n" + guard + "\n",
			map[string]int{analytics: 2, autoUpd: 2}, nil, 0},

		// ranger-base-mk6gw: the reachability arms. Every one of these three
		// is a script that SAYS it exports the guards and exports nothing at
		// runtime, and every one of them was green before this bead — the
		// first is the measured survivor off ranger-base-f5lu1. `shadowed`
		// carries the line in each, which is what makes these rows a reading
		// rather than a reader that merely stopped finding things.
		{"an export inside a function nothing calls is not a setting",
			"#!/usr/bin/env bash\nnever_called() {\n\t" + guard + "\n}\nv=$(brew info --cask codex)\n",
			map[string]int{}, map[string]int{analytics: 3, autoUpd: 3}, 5},
		{"an export inside an `if false` branch is not a setting",
			"#!/usr/bin/env bash\nif false; then\n\t" + guard + "\nfi\nv=$(brew list --pinned)\n",
			map[string]int{}, map[string]int{analytics: 3, autoUpd: 3}, 5},
		{"an export inside a heredoc is not a setting",
			"#!/usr/bin/env bash\ncat <<'EOF' >/dev/null\n" + guard + "\nEOF\nv=$(brew --prefix)\n",
			map[string]int{}, map[string]int{}, 5},
		// Column 0 is demanded on top of the depth count, because the depth
		// count is a count: a function body written without indentation is
		// caught by the braces, and one whose braces the count has lost is
		// caught by the margin.
		{"an unindented export inside a function is still not a setting",
			"#!/usr/bin/env bash\nnever_called() {\n" + guard + "\n}\nv=$(brew --prefix)\n",
			map[string]int{}, map[string]int{analytics: 3, autoUpd: 3}, 5},
		{"an indented top-level export is refused — the reader errs strict",
			"#!/usr/bin/env bash\n\t" + guard + "\nv=$(brew --prefix)\n",
			map[string]int{}, map[string]int{analytics: 2, autoUpd: 2}, 3},
		{"an export below a block that CLOSED is top level again",
			"#!/usr/bin/env bash\nif false; then\n\t: nothing\nfi\n" + guard + "\nv=$(brew --prefix)\n",
			map[string]int{analytics: 5, autoUpd: 5}, map[string]int{}, 6},

		// ranger-base-80zqb: the same two shapes spelled with PARENTHESES,
		// which the brace-counting reader placed at column 0 with depth 0 and
		// recorded as top-level guards. Both were MEASURED 2026-10-31 to leave
		// both settings unset in the brew child while all eight brew-guard arms
		// stayed green, so these two rows are the pin and the brace rows above
		// are no longer the whole of it. Mutation-check this reader with these
		// spellings, not the brace one.
		{"an export inside a top-level subshell is not a setting",
			"#!/usr/bin/env bash\n(\n" + guard + "\n)\nv=$(brew info --cask codex)\n",
			map[string]int{}, map[string]int{analytics: 3, autoUpd: 3}, 5},
		{"an export inside a function whose body is a subshell is not a setting",
			"#!/usr/bin/env bash\nnever_called() (\n" + guard + "\n)\nv=$(brew --prefix)\n",
			map[string]int{}, map[string]int{analytics: 3, autoUpd: 3}, 5},

		// And what the rule COSTS, written down rather than discovered: this
		// export DOES run and DOES set both settings, and the reader refuses
		// it, because `( … )` opening a subshell and `$( … )` substituting a
		// command are the same character to a line-based scan. A false FAIL
		// naming its line is the safe direction and the diagnostic says to
		// move the export above that line; a guard this reader imagined would
		// be the pin lying.
		{"an export whose own line carries a command substitution is refused — strict, and the line is named",
			"#!/usr/bin/env bash\n" + guard + " NOW=$(date)\nv=$(brew --prefix)\n",
			map[string]int{}, map[string]int{analytics: 2, autoUpd: 2}, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "fixture.sh")
			if err := os.WriteFile(f, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			exports, shadowed, mentions, _, err := bgRead(f)
			if err != nil {
				t.Fatal(err)
			}
			if got := bgFmtExports(exports); got != bgFmtExports(c.wantExports) {
				t.Errorf("exports = %s, want %s\n%s", got, bgFmtExports(c.wantExports), c.body)
			}
			// Asserted and not incidental: a refused export must be one the
			// reader SAW and placed, not one it failed to find. The two
			// readings are indistinguishable from `exports` alone, and only
			// one of them is this reader working.
			if got := bgFmtExports(shadowed); got != bgFmtExports(c.wantShadow) {
				t.Errorf("shadowed = %s, want %s\n%s", got, bgFmtExports(c.wantShadow), c.body)
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

// The depth count is a COUNT, so it can be lost, and losing it has a
// direction: drift upward refuses an export that was fine (a false FAIL that
// names its line), drift downward would call a line top level that is not.
// So a negative depth is refused outright rather than clamped to zero. The
// fixture is the cheapest real shape — a brace in a string, with no opener.
func TestQATheBrewGuardReaderRefusesAStructureItCannotPlace(t *testing.T) {
	f := filepath.Join(t.TempDir(), "fixture.sh")
	body := "#!/usr/bin/env bash\necho \"}\"\n" +
		"export HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_AUTO_UPDATE=1\nv=$(brew --prefix)\n"
	if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	exports, _, mentions, lost, err := bgRead(f)
	if err != nil {
		t.Fatal(err)
	}
	if lost == "" {
		t.Fatalf("a script whose block count went negative must say so, not read on as if it were at top level:\n%s", body)
	}
	if !strings.Contains(lost, ":2:") {
		t.Errorf("the lost-count report must name the line that lost it, got %q", lost)
	}
	// The point of saying so: the export BELOW that line is not placed.
	if got := bgFmtExports(exports); got != "{}" {
		t.Errorf("exports = %s, want {} — an export below a lost block count is an export this reader cannot place, and placing it anyway is the one direction that turns the pin into a lie", got)
	}
	// And it read on: the call below the drift is still in the census.
	if len(mentions) != 1 {
		t.Errorf("mentions = %v, want the one brew read at line 4 — a reader that STOPPED at the drift would report a script full of brew calls as a script with none", mentions)
	}
}

// bgRoster — every file under scripts/ that reads brew, and the whole of what
// the guard is demanded of. The three verify-*-pin.sh gates also carry their
// own arm in their own file, where a seat looks for a pin about one script;
// this list is what makes the SET of scripts a pinned fact rather than three
// hand-typed paths (ranger-base-mk6gw finding 3).
//
// ranger-base-esz16's deliverable asked for the pin "so the next brew-calling
// gate cannot drop it". bgGuards made the list of SETTINGS undroppable — add
// or remove a row and all three arms move at once. The list of SCRIPTS was
// spelled by hand once per arm, so a FOURTH brew caller was pinned by nothing,
// and there already was one: scripts/macos-install-probe.sh reads the
// operator's own brew at :189 (`/opt/homebrew/bin/brew --prefix`) in `paths`
// mode. It carried both guards inside two functions and neither of them runs
// before that read — which is finding 1 and finding 3 arriving at the same
// line, and the reason they are one bead.
var bgRoster = []string{
	"scripts/macos-install-probe.sh",
	"scripts/verify-bd-pin.sh",
	"scripts/verify-codex-pin.sh",
	"scripts/verify-grok-pin.sh",
}

// bgExempt — a file the census finds NAMING brew that calls none, with the
// reason, in the two-list shape scripts/verify-box.sh's ROSTER/EXCLUDED census
// already uses and boxcheck_qa_test.go already enforces for its own class.
//
// An exemption is a claim, and the claim is checkable from the mentions the
// census prints: the reader's brew side is over-inclusive on purpose (bgRead's
// head comment), so prose, a trailing comment and heredoc text all land here
// and all have to be classified by a person. What the pin holds is that the
// classification EXISTS and is current — a new brew-naming file reds until it
// is on one of these two lists, and a row for a file that no longer names brew
// reds as stale.
var bgExempt = map[string]string{
	"scripts/release-artifacts.sh": "names brew once, in the trailing comment on `bottle_tag() {`, and in the prose above it. The point of that prose is that this script renders a bottle on a box with no brew at all — `brew bottle` is the command it deliberately does not run — so there is nothing here to guard.",
	"scripts/tap-formula.sh":       "names brew once, inside the caveats heredoc it writes INTO the rendered formula (`brew install beads`): text for a deployer's terminal, not a command this script runs. It renders a formula; it never execs brew.",
}

// Every file under scripts/ that names brew on a runnable line is on one of
// the two lists above. Two-way, like boxcheck's: nothing claimed by a list is
// absent from the census either, so neither list can go stale while staying
// green.
//
// IT WALKS, and until ranger-base-80zqb it did not. An os.ReadDir of scripts/
// reads one level, so "every file under scripts/" was a claim about the top of
// the directory only: a brew-calling script one directory down was on neither
// list and red nothing. MEASURED 2026-10-31 — an unguarded caller planted at
// scripts/zzprobe/newgate.sh, which bgRead reports as one runnable brew
// mention with no exports, left this arm green. scripts/ is flat today (56
// entries, 0 of them directories, MEASURED the same day), so nothing was
// escaping: the fix is the claim being made good rather than a file being
// caught, and it is what makes a roster row's path meaningful below the top
// level. Walking rather than asserting flatness is deliberate — an assertion
// that scripts/ must stay flat is a layout ruling this pin has no business
// making, and it would red on the first legitimate scripts/lib/.
func TestQAEveryBrewNamingScriptIsRosteredOrExempt(t *testing.T) {
	var files []string
	if err := filepath.WalkDir("scripts", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		t.Fatalf("walk scripts/: %v", err)
	}
	sort.Strings(files)
	// Reader sanity before the arms trust it. An assertion that every
	// brew-naming file is classified is satisfied by a walk that read none,
	// and a walk that read none is silent about it. The floor sits well under
	// the live count (56, MEASURED 2026-10-31) so an ordinary landing does not
	// move it. bgRoster's four rows are the other half of the same witness.
	if len(files) < 40 {
		t.Fatalf("the walk of scripts/ found only %d file(s) — the walk is broken, not the tree: %v", len(files), files)
	}
	rostered := map[string]bool{}
	for _, p := range bgRoster {
		rostered[p] = true
	}
	seen := map[string][]string{}
	for _, path := range files {
		_, _, mentions, _, err := bgRead(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if len(mentions) == 0 {
			continue
		}
		seen[path] = mentions
		if rostered[path] {
			if _, both := bgExempt[path]; both {
				t.Errorf("%s is on the brew-guard roster AND in the exemptions — one place, one answer", path)
			}
			continue
		}
		if reason, ok := bgExempt[path]; ok {
			if len(strings.TrimSpace(reason)) < 40 {
				t.Errorf("%s is exempted from the brew guard with no reason worth reading (%q) — an exemption nobody can check is the guard being dropped quietly", path, reason)
			}
			continue
		}
		t.Errorf("%s has %d runnable line(s) naming brew and is on neither bgRoster nor bgExempt in %s.\n"+
			"First is %s\n"+
			"Put it on the roster if it READS brew — it then has to export every bgGuards setting above its first read, because a crew-run check must not change the box it reads or report what it read (crew hard risk line 4) — or in bgExempt with the reason brew is only named here.",
			path, len(mentions), "internal/treepins/brewguard_qa_test.go", mentions[0])
	}
	for _, p := range bgRoster {
		if _, ok := seen[p]; !ok {
			t.Errorf("bgRoster names %s and the census finds no runnable brew line in it — a stale roster row is a pin over nothing, and it is also how this census would go blind without saying so", p)
		}
	}
	for p := range bgExempt {
		if _, ok := seen[p]; !ok {
			t.Errorf("bgExempt names %s and the census finds no runnable brew line in it — delete the row rather than leave a reason nobody will check again", p)
		}
	}
}

// The roster asserted, all of it. The three verify-*-pin.sh arms in their own
// files are the same assertion over three of these four; this is where
// macos-install-probe.sh is held, and where a fifth brew caller is held the
// moment the census above makes someone add it.
func TestQAEveryRosteredBrewCallerGuardsItsReads(t *testing.T) {
	for _, script := range bgRoster {
		t.Run(filepath.Base(script), func(t *testing.T) {
			bgAssertBrewReadsAreGuarded(t, script)
		})
	}
}

// bgUnset is what the stub brew prints for a setting it does not have, so that
// "the child had no such variable" and "the child had it set to empty" are two
// different readings rather than one empty string.
const bgUnset = "<unset>"

// bgGate — one gate the dynamic arm can RUN, with the minimum fixture that
// gets it as far as its first brew call.
type bgGate struct {
	script  string // run in place from the repo root; it cd's to its own ..
	subject string // the binary it refuses to start without, stubbed on PATH
	cfgDir  string // a $HOME-relative dir it wants a config.toml in; "" for none
}

// bgDynamic — the BY-CONSTRUCTION half of the reachability claim, and the half
// no reader of text can be (ranger-base-80zqb; option (b) of
// ranger-base-mk6gw's own finding 1, which that bead's close did not take).
//
// bgRead approximates "this export is reached before that brew call" and will
// always be an approximation: it got braces and block words, missed
// parentheses, and the repair is a rule about brackets rather than a parse.
// Running the gate measures the thing itself. An export the shell never
// executed — in a subshell, in a function nothing calls, in a branch not
// taken, below an `exit` — is simply not in the child's environment, whatever
// it is spelled like, so the two shapes MEASURED to survive the brace counter
// fail here by construction and so does the next spelling nobody has thought
// of.
//
// The fixtures are minimal on purpose and each row's two fields are the whole
// of what one gate needs to REACH brew: all three refuse to start without
// their subject binary on PATH, and two of them without a config.toml
// (MEASURED 2026-10-31: with neither, all three died at exit 2 having called
// brew zero times — a green arm over a gate that was never measured, which is
// what the "called brew zero times" fatal below exists to refuse). Each gate's
// own rows FAIL against a fixture this thin; its exit code is not this arm's
// subject and is not asserted.
var bgDynamic = []bgGate{
	{"scripts/verify-bd-pin.sh", "bd", ""},
	{"scripts/verify-codex-pin.sh", "codex", ".codex"},
	{"scripts/verify-grok-pin.sh", "grok", ".grok"},
}

// bgNoDynamic — a bgRoster caller this arm cannot run, with the reason, in the
// same two-list shape bgRoster/bgExempt already uses. A rostered caller on
// neither list reds: that is what stops the dynamic half covering three of
// four while reading as though it covered the roster.
var bgNoDynamic = map[string]string{
	"scripts/macos-install-probe.sh": "reads brew at an ABSOLUTE path — `/opt/homebrew/bin/brew --prefix`, falling back to /usr/local/bin/brew (:206) — because `paths` mode's whole subject is the OPERATOR's own brew and whether its prefix is on the default PATH. An absolute path consults neither $PATH nor the shell's functions, so nothing can stand in for that child and its environment cannot be observed hermetically; the alternative is running the real brew from the suite, which is the footprint this file exists to refuse (MEASURED 2026-10-31). This caller's guard is held by bgRead and by nothing else, and it is the reason that reader is repaired rather than deleted.",
}

// The guards, measured where they have to be true: inside brew's own
// environment. This is the pin bgRead can only approximate.
func TestQABrewGuardsReachTheBrewChild(t *testing.T) {
	// Two-way against bgRoster first, so neither list can go stale while the
	// arms below stay green.
	dyn := map[string]bool{}
	for _, d := range bgDynamic {
		dyn[d.script] = true
	}
	rostered := map[string]bool{}
	for _, p := range bgRoster {
		rostered[p] = true
		_, excused := bgNoDynamic[p]
		if dyn[p] && excused {
			t.Errorf("%s is both run by this arm and excused from it in bgNoDynamic — one place, one answer", p)
			continue
		}
		if !dyn[p] && !excused {
			t.Errorf("%s is on bgRoster and is neither run by this arm nor named in bgNoDynamic.\n"+
				"Add it to bgDynamic with the fixture that gets it to its first brew call, or to bgNoDynamic with the reason its brew child cannot be observed. The static reader alone passed two MEASURED subshell spellings of a guard that sets nothing (ranger-base-80zqb), so a caller held by text only is a caller whose guard is a claim.", p)
		}
	}
	for _, d := range bgDynamic {
		if !rostered[d.script] {
			t.Errorf("bgDynamic runs %s, which is not on bgRoster — this arm's subject is the roster, so a row outside it is a pin over nothing", d.script)
		}
	}
	for p := range bgNoDynamic {
		if !rostered[p] {
			t.Errorf("bgNoDynamic excuses %s, which is not on bgRoster — delete the row rather than leave a reason nobody will check again", p)
		}
	}

	for _, d := range bgDynamic {
		t.Run(filepath.Base(d.script), func(t *testing.T) {
			home, bin := t.TempDir(), t.TempDir()
			rec := filepath.Join(t.TempDir(), "brew-calls")

			if d.cfgDir != "" {
				dir := filepath.Join(home, d.cfgDir)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.toml"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// The subject binary. Without it every one of these gates exits 2
			// with "<subject> not on PATH" before it reads brew.
			subj := "#!/bin/sh\necho '" + d.subject + " 0.0.0'\nexit 0\n"
			if err := WriteExecutable(filepath.Join(bin, d.subject), []byte(subj), 0o755); err != nil {
				t.Fatal(err)
			}

			// The instrument: a brew that records one block per call, naming
			// the argv it was given and then every bgGuards setting as THIS
			// CHILD has it. The names are derived from bgGuards, so a row added
			// there is demanded here too and cannot be half-applied.
			body := "#!/bin/sh\n{\n\tprintf 'call %s\\n' \"$*\"\n"
			for _, g := range bgGuards {
				name, _, _ := strings.Cut(g.assign, "=")
				body += fmt.Sprintf("\tprintf '%s=%%s\\n' \"${%s-%s}\"\n", name, name, bgUnset)
			}
			body += "} >>\"$BG_BREW_REC\"\nexit 0\n"
			if err := WriteExecutable(filepath.Join(bin, "brew"), []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}

			// Built from this literal list and deliberately NOT from
			// os.Environ(), which is the whole validity of the arm: a guard
			// the parent `go test` already carried would arrive in the child
			// and read as the script's own export. It is also why the
			// CLAUDE_CONFIG_DIR fence in TestMain is not needed here — these
			// children are handed no path to the operator's home at all, HOME
			// included. Asserted rather than assumed, two lines down.
			env := []string{
				"PATH=" + bin + string(os.PathListSeparator) + "/usr/bin:/bin",
				"HOME=" + home,
				"TMPDIR=" + t.TempDir(),
				"BG_BREW_REC=" + rec,
			}
			for _, kv := range env {
				for _, g := range bgGuards {
					name, _, _ := strings.Cut(g.assign, "=")
					if strings.HasPrefix(kv, name+"=") {
						t.Fatalf("this arm's own child environment carries %q — then a child that HAS the setting proves nothing about %s", kv, d.script)
					}
				}
			}

			// Bounded, and the grace named with it: a context deadline alone
			// kills the process and still blocks on the pipes its children
			// hold (ranger-base-2cttm).
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, d.script)
			cmd.Env = env
			cmd.WaitDelay = 5 * time.Second
			out, _ := cmd.CombinedOutput()

			b, err := os.ReadFile(rec)
			if err != nil {
				t.Fatalf("%s called brew zero times, so this arm measured nothing and a green result here is not evidence (%v).\n"+
					"The minimum fixture in bgDynamic has gone stale — this gate now refuses to run before it reads brew. Its output was:\n%s", d.script, err, out)
			}
			type call struct {
				argv string
				env  map[string]string
			}
			var calls []call
			for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
				if argv, ok := strings.CutPrefix(line, "call "); ok {
					calls = append(calls, call{argv: argv, env: map[string]string{}})
					continue
				}
				if len(calls) == 0 {
					t.Fatalf("%s: the recording opens with %q, before any `call` line — the stub's own format changed, so this arm is reading something else", d.script, line)
				}
				k, v, ok := strings.Cut(line, "=")
				if !ok {
					t.Fatalf("%s: recording line %q is not name=value — the stub's own format changed", d.script, line)
				}
				calls[len(calls)-1].env[k] = v
			}
			if len(calls) == 0 {
				t.Fatalf("%s: the recording at %s is empty — the stub ran and wrote nothing, so this arm measured nothing.\n%s", d.script, rec, out)
			}

			// Every call, not the first: a gate that exported the guards, read
			// brew, and then unset or shadowed them before a later read would
			// pass a first-call-only check over a footprint it still leaves.
			for _, c := range calls {
				for _, g := range bgGuards {
					name, want, _ := strings.Cut(g.assign, "=")
					got, ok := c.env[name]
					if !ok {
						t.Errorf("%s: the stub recorded no %s line for `brew %s` — the stub's own format changed", d.script, name, c.argv)
						continue
					}
					if got == want {
						continue
					}
					t.Errorf("%s ran `brew %s` with %s=%s in its environment, want %s=%s.\n"+
						"  %s\n"+
						"  The export is in the text and it did not reach the child. A subshell, a function body, a branch not taken and anything below an `exit` each read as a top-level guard to a line-based scan and set nothing at runtime; bgRead is repaired for the two bracket spellings and this arm is what closes the class (ranger-base-80zqb).",
						d.script, c.argv, name, got, name, want, fmt.Sprintf(g.cost, len(calls)))
				}
			}
		})
	}
}
