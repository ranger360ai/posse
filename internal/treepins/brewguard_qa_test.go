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
// where it is reached unconditionally — column 0, block depth 0, outside any
// heredoc — and anything this reader cannot place there is not counted. A
// guard it fails to see can only make the pin stricter; one it imagines makes
// the pin a lie.
//
// Depth is a COUNT of block words and not a parse, so it drifts upward on a
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
				where = fmt.Sprintf(" This reader also lost its block count at %s and placed nothing below that line, so if the export is down there, that is why it was not seen: balance the block words on that line, or move the export above it.", lost)
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
		{"both guarded above the call", "#!/bin/bash\n" + guard + "\nv=$(brew info --cask codex)\n",
			map[string]int{analytics: 2, autoUpd: 2}, nil, 3},
		{"no export at all", "#!/bin/bash\nv=$(brew info --cask codex)\n",
			map[string]int{}, nil, 2},
		{"both exported below the call", "#!/bin/bash\nv=$(brew info --cask codex)\n" + guard + "\n",
			map[string]int{analytics: 3, autoUpd: 3}, nil, 2},
		{"auto-update only — the pre-esz16 tree", "#!/bin/bash\n" + autoOnly + "\nv=$(brew list --pinned)\n",
			map[string]int{autoUpd: 2}, nil, 3},
		{"analytics only", "#!/bin/bash\nexport " + analytics + "\nv=$(brew list --pinned)\n",
			map[string]int{analytics: 2}, nil, 3},
		{"the two on separate lines, one of them below the call",
			"#!/bin/bash\nexport " + analytics + "\nv=$(brew --prefix)\nexport " + autoUpd + "\n",
			map[string]int{analytics: 2, autoUpd: 4}, nil, 3},
		{"a commented export is not a setting", "#!/bin/bash\n# " + guard + "\nv=$(brew list --pinned)\n",
			map[string]int{}, nil, 3},
		{"a bare assignment is not an export", "#!/bin/bash\n" + analytics + " " + autoUpd + "\nv=$(brew --prefix)\n",
			map[string]int{}, nil, 3},
		{"prose about brew is not a call", "#!/bin/bash\n# brew info is auto-update-able, it says here\n" + guard + "\n",
			map[string]int{analytics: 3, autoUpd: 3}, nil, 0},
		{"the export line is not itself a brew call", "#!/bin/bash\n" + guard + "\n",
			map[string]int{analytics: 2, autoUpd: 2}, nil, 0},

		// ranger-base-mk6gw: the reachability arms. Every one of these three
		// is a script that SAYS it exports the guards and exports nothing at
		// runtime, and every one of them was green before this bead — the
		// first is the measured survivor off ranger-base-f5lu1. `shadowed`
		// carries the line in each, which is what makes these rows a reading
		// rather than a reader that merely stopped finding things.
		{"an export inside a function nothing calls is not a setting",
			"#!/bin/bash\nnever_called() {\n\t" + guard + "\n}\nv=$(brew info --cask codex)\n",
			map[string]int{}, map[string]int{analytics: 3, autoUpd: 3}, 5},
		{"an export inside an `if false` branch is not a setting",
			"#!/bin/bash\nif false; then\n\t" + guard + "\nfi\nv=$(brew list --pinned)\n",
			map[string]int{}, map[string]int{analytics: 3, autoUpd: 3}, 5},
		{"an export inside a heredoc is not a setting",
			"#!/bin/bash\ncat <<'EOF' >/dev/null\n" + guard + "\nEOF\nv=$(brew --prefix)\n",
			map[string]int{}, map[string]int{}, 5},
		// Column 0 is demanded on top of the depth count, because the depth
		// count is a count: a function body written without indentation is
		// caught by the braces, and one whose braces the count has lost is
		// caught by the margin.
		{"an unindented export inside a function is still not a setting",
			"#!/bin/bash\nnever_called() {\n" + guard + "\n}\nv=$(brew --prefix)\n",
			map[string]int{}, map[string]int{analytics: 3, autoUpd: 3}, 5},
		{"an indented top-level export is refused — the reader errs strict",
			"#!/bin/bash\n\t" + guard + "\nv=$(brew --prefix)\n",
			map[string]int{}, map[string]int{analytics: 2, autoUpd: 2}, 3},
		{"an export below a block that CLOSED is top level again",
			"#!/bin/bash\nif false; then\n\t: nothing\nfi\n" + guard + "\nv=$(brew --prefix)\n",
			map[string]int{analytics: 5, autoUpd: 5}, map[string]int{}, 6},
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
	body := "#!/bin/bash\necho \"}\"\n" +
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
func TestQAEveryBrewNamingScriptIsRosteredOrExempt(t *testing.T) {
	ents, err := os.ReadDir("scripts")
	if err != nil {
		t.Fatalf("read scripts/: %v", err)
	}
	rostered := map[string]bool{}
	for _, p := range bgRoster {
		rostered[p] = true
	}
	seen := map[string][]string{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		path := filepath.Join("scripts", e.Name())
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
