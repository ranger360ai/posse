//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-q749c — ADR 0097 D4's venue contract, read three
// ways: the gate's RULE (a pure function, nine rows, no wall and no
// environment), the three LITERALS the contract shares with the record, and
// the two test names D4 retired.
//
// WHY THE RULE NEEDS A PIN OF ITS OWN. The gate can only be exercised by
// being a member of the class it gates: a test that called it with a name
// outside D3's filter would FAIL, which is not an assertion, and a test that
// called it inside the venue would need a venue to be in. So the rule is
// split out as bwrapNestDecision and read here. The tree's 32 members then
// exercise one row of it each, which is coverage of the wiring and of nothing
// else — in any one process every one of them takes the SAME row, because
// the wall answer is memoized once per binary.
//
// WHY THE LITERALS NEED ONE. Three strings in wallvenue_test.go are read by
// something outside this package: the MARKER, which `scripts/posse-wall-check`
// greps out of `go test -json` to count members; the member PATTERN, which
// that same script types as its `-run` filter; and the ENVIRONMENT VARIABLE,
// which ADR 0097 D1's venue render adds to the run's allowlist. Each of them
// is a handshake with a reader that cannot be compiled against this package,
// so each of them fails SILENTLY when it drifts: a reworded marker counts
// zero members and answers exit 3, which D5's watchdog reports as "wall check
// silent" rather than as a wall regression; a filter that no longer selects a
// member is a pin the venue never runs; a misspelled variable abstains every
// member of a venue run. The record is the only common copy, so the pin holds
// the literals to the record.
//
// WHY A RETIRED NAME NEEDS ONE. D4 renamed two members into the filter. A
// `-run` door or a sentence still spelling the old name points at nothing —
// and a `-run` filter that selects nothing exits 0 and prints `ok`, which is
// the failure mode this whole class exists to refuse. The rule is per FILE
// and not per line: a file that still spells a retired name must also spell
// its live one, so a provenance sentence ("it was X, it is Y now") is allowed
// and a dangling reference is not.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// adr0097Path is the record these literals are shared with. Named, not
// globbed: if the file moves this pin should say so by name rather than
// quietly census nothing.
const adr0097Path = "docs/adr/0097-the-walls-live-pins-measure-in-a-nesting-venue-after-each-landing.md"

// wallVenueRename is one test name ADR 0097 D4 retired, and the live name it
// became. A func and not a package-level var, so no parallel reader of it
// needs a cmd/testparallel clearance (the reason wallVenueMarker's comment
// gives).
type wallVenueRename struct {
	retired string
	live    string
	bead    string
}

func wallVenueRenames() []wallVenueRename {
	return []wallVenueRename{
		{
			retired: "TestLiveStrictMCPStartsOnlyTheLineServersUnderTheRenderedWall",
			live:    "TestLiveWallStrictMCPStartsOnlyTheLineServers",
			bead:    "ranger-base-q749c",
		},
		{
			retired: "TestLiveGrokLeaderUseLeaderTUIUnderTheWallStartsNoLeader",
			live:    "TestLiveWallGrokLeaderUseLeaderTUIStartsNoLeader",
			bead:    "ranger-base-q749c",
		},
	}
}

// wallVenueDanglingLines returns the 1-based lines of body that spell r's
// retired name when body spells its live name NOWHERE. Split out from the
// census so the control below can feed it planted bodies: a pin whose only
// arm is the tree it is standing in cannot be shown able to fail.
func wallVenueDanglingLines(body string, r wallVenueRename) []int {
	if strings.Contains(body, r.live) {
		return nil
	}
	var out []int
	for i, line := range strings.Split(body, "\n") {
		if strings.Contains(line, r.retired) {
			out = append(out, i+1)
		}
	}
	return out
}

// ARM 1 — the rule, row by row over bwrapNestDecision: three wall answers
// (nests; refused outside the venue; refused inside it) against the name
// shapes that matter (inside D3's filter, outside it, and a subtest path of
// each). No count is written here, because the rows below are the count.
func TestQAWallVenueGateRuleIsADR0097D4s(t *testing.T) {
	t.Parallel()

	const refusal = "bwrap: setting up uid map: Read-only file system"
	for _, c := range []struct {
		why     string
		name    string
		refusal string
		venue   bool
		want    nestAction
		// carries is a fragment the message must hold, so a row cannot pass
		// on the action alone while printing the wrong sentence.
		carries string
	}{
		// Nesting works. The marker is logged and the pin proceeds, in the
		// venue and out of it alike — D4 says "whenever it lets a pin
		// proceed", and the uncaged operator run is still the control (D6).
		{"a Bwrap member, nesting works", "TestLiveBwrapHidesTheRuntimeDirsControlEndpoints", "", false, nestMeasure, wallVenueMarker},
		{"a Wall member, nesting works", "TestLiveWallWatchPassesTheSeatsOwnExitCode", "", false, nestMeasure, wallVenueMarker},
		{"a QA member, nesting works", "TestQALiveWallSomethingOrOther", "", false, nestMeasure, wallVenueMarker},
		{"in the venue, nesting works", "TestLiveWallSeatCacheRefusesSharedCachePlants", "", true, nestMeasure, wallVenueMarker},

		// Nesting refused. Outside the venue that is the wall working and
		// the pin abstains with ADR 0096 D3's sentence; inside it the
		// baseline has already proved the venue nests, so it is a red.
		{"a member on a caged seat", "TestLiveBwrapGitRefsCommitsOnTheSessionBranch", refusal, false, nestAbstain, bwrapNestSkipPrefix + refusal},
		{"a member inside the venue", "TestLiveBwrapGitRefsCommitsOnTheSessionBranch", refusal, true, nestFail, bwrapNestSkipPrefix + refusal},

		// The name. A caller outside D3's filter is a defect in the tree and
		// is refused whatever the wall answers — including on a caged seat,
		// where the wall's own refusal would otherwise hide it behind a skip.
		{"the retired StrictMCP spelling, nesting works", wallVenueRenames()[0].retired, "", false, nestFail, wallVenueMemberPattern},
		{"the retired GrokLeader spelling, on a caged seat", wallVenueRenames()[1].retired, refusal, false, nestFail, wallVenueMemberPattern},
		{"a Live test that is not Bwrap or Wall", "TestLiveGrokLineStartsNoCompatMCPOrHook", "", false, nestFail, wallVenueMemberPattern},
		{"a test that is not Live at all", "TestQABwrapArgsAreWhatTheyWere", "", false, nestFail, wallVenueMemberPattern},

		// The subtest path. `-run` and `go test -json` both see the
		// top-level name, so the rule is judged on that and not on the label.
		{"a member's subtest", "TestLiveWallWatchPassesTheSeatsOwnExitCode/the_seat_exits_7", "", false, nestMeasure, wallVenueMarker},
		{"an outsider's subtest", "TestLiveGrokLineStartsNoCompatMCPOrHook/arm", "", false, nestFail, wallVenueMemberPattern},
	} {
		got, msg := bwrapNestDecision(c.name, c.refusal, c.venue)
		if got != c.want {
			t.Errorf("%s: bwrapNestDecision(%q, refusal=%q, venue=%v) = %v, want %v\n  said: %s",
				c.why, c.name, c.refusal, c.venue, got, c.want, msg)
			continue
		}
		if !strings.Contains(msg, c.carries) {
			t.Errorf("%s: the message does not carry %q — a row that answers with the right action and the wrong words is a reader sent to the wrong rule:\n  %s",
				c.why, c.carries, msg)
		}
	}

	// The venue red keeps D3's sentence INTACT, which is what the bead asks
	// for ("t.Fatalf with ADR 0096 D3's sentence instead of t.Skip"): the
	// same words a reader greps, plus the clause that says why this one is
	// red. Held by prefix so the clause cannot migrate in front of it.
	_, red := bwrapNestDecision("TestLiveWallSeatCacheRefusesSharedCachePlants", refusal, true)
	if !strings.HasPrefix(red, bwrapNestSkipPrefix+refusal) {
		t.Errorf("the venue's red does not begin with ADR 0096 D3's sentence:\n  %s", red)
	}
	if !strings.Contains(red, wallVenueEnvVar) {
		t.Errorf("the venue's red does not name %s, so a reader cannot tell why the same answer that skips on a seat fails here:\n  %s", wallVenueEnvVar, red)
	}

	// And the marker is the WHOLE message when a pin proceeds, not a line
	// inside a longer log: the script matches the line, so a prefix or a
	// suffix the gate adds is a line that no longer reads as the marker.
	if _, m := bwrapNestDecision("TestLiveWallWatchPassesTheSeatsOwnExitCode", "", false); m != wallVenueMarker {
		t.Errorf("the gate logs %q where the marker is %q", m, wallVenueMarker)
	}

	// The top-level split, directly, including the shapes it must leave
	// alone. `go test` writes a subtest path with `/`; nothing else does.
	for in, want := range map[string]string{
		"TestLiveWallX":           "TestLiveWallX",
		"TestLiveWallX/sub":       "TestLiveWallX",
		"TestLiveWallX/sub/deep":  "TestLiveWallX",
		"TestLiveWallX/with_a_#":  "TestLiveWallX",
		"":                        "",
		"/leading":                "",
		"TestLiveWallUnderscore_": "TestLiveWallUnderscore_",
	} {
		if got := wallVenueTopLevel(in); got != want {
			t.Errorf("wallVenueTopLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

// ARM 2 — the three literals, against the record. See the header: each one is
// a handshake with a reader that cannot be compiled against this package, so
// the record is the only copy both sides can be held to.
//
// It takes the tree from qibRepoRoot, which puts it in the tree-wide class
// (internal/treepins/treewidedoor_qa_test.go) — hence `make doc-check`.
func TestQAWallVenueContractLiteralsAreADR0097s(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(adr0097Path)))
	if err != nil {
		t.Fatalf("read %s: %v — this pin holds three literals to that record, and it cannot do it from a tree that does not have it. If the record moved, move adr0097Path with it.", adr0097Path, err)
	}
	adr := string(b)

	for _, c := range []struct {
		what string
		want string
		why  string
	}{
		{
			"the membership marker",
			"`" + wallVenueMarker + "`",
			"`scripts/posse-wall-check` counts one member per test that printed this line (ADR 0097 D3: membership is read from the run, never inferred from the name). Reworded on one side only, the body counts zero members, answers exit 3, and reaches a reader as D5's \"wall check silent\" rather than as a wall regression",
		},
		{
			"the member pattern",
			"'" + wallVenueMemberPattern + "'",
			"the body SELECTS the venue's run with this filter and D4 holds every member's name to it, so the two coming apart is a pin the venue never runs and no ratchet ever reds — which is the drift D4 exists to prevent",
		},
		{
			"the venue's environment variable",
			wallVenueEnvVar,
			"D1 adds it to the venue run's environment allowlist and it is the ONLY signal that tells a test process it is the venue and not a seat; misspelled on either side, every member of a venue run abstains",
		},
	} {
		if !strings.Contains(adr, c.want) {
			t.Errorf("%s does not carry %s as %s — %s.\nThe code's spelling is in wallvenue_test.go; fix whichever side is wrong, not this pin.",
				adr0097Path, c.what, c.want, c.why)
		}
	}

	// The premise. A pin over a document's contents is satisfied by reading
	// an empty one, and the floor is far under what the record holds.
	if len(adr) < 2000 {
		t.Fatalf("%s is %d bytes — that is not the record this pin reads", adr0097Path, len(adr))
	}
}

// ARM 3 — the census. Every tracked file, read whole: a file that still
// spells a name D4 retired must also spell the live one, or the reference
// dangles. Per file and not per line, so a provenance sentence is allowed
// (header, last paragraph).
//
// Every tracked file and not just the doors, because the shapes that go stale
// are not enumerable: a `-run` filter in a Makefile recipe or a comment, a
// sentence in an ADR or a notes fragment, a probe script's regex, a bead
// title quoted into a README. The one thing they share is that they are
// tracked text.
func TestQANoRetiredWallVenueMemberNameDanglesInTheTree(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not `git rev-parse --show-toplevel`: any file added
	// anywhere can red this, which is the tree-wide class, and that class is
	// derived from calls to the one root helper (ranger-base-xndgk FINDING
	// 5). Spelled with git it would have no Makefile door.
	root := qibRepoRoot(t)
	qibSkipUnlessCheckout(t, root)

	renames := wallVenueRenames()
	if len(renames) == 0 {
		t.Fatal("wallVenueRenames is empty — a census over an empty table is satisfied by reading nothing. If D4's renames are truly all retired from memory, delete this pin and its door rather than leaving a green one")
	}

	// The table's own shape, before the tree: each row has to be a rename
	// INTO D3's filter, which is what D4 asked for and the only thing that
	// makes the row worth holding.
	re := wallVenueMemberRe()
	for _, r := range renames {
		if re.MatchString(r.retired) {
			t.Errorf("%s matches %s, so it was never outside D4's filter and is not one of its renames", r.retired, wallVenueMemberPattern)
		}
		if !re.MatchString(r.live) {
			t.Errorf("%s does not match %s, so %s renamed one outsider into another — the member is still a pin the venue never runs", r.live, wallVenueMemberPattern, r.bead)
		}
	}

	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files did not run: %v", err)
	}
	paths := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	// A census over a derived set is satisfied by deriving nothing.
	if len(paths) < 100 {
		t.Fatalf("census premise: git ls-files returned %d paths, which is not this repo", len(paths))
	}

	declared := map[string]bool{}
	scanned := 0
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			// A tracked path with no readable file is a checkout
			// mid-something, not a finding; skipping it silently would let a
			// census read clean over a tree it never opened, so say so.
			t.Logf("tracked but unreadable, not censused: %s (%v)", rel, err)
			continue
		}
		scanned++
		body := string(b)
		for _, r := range renames {
			if strings.Contains(body, "func "+r.live+"(") {
				declared[r.live] = true
			}
			lines := wallVenueDanglingLines(body, r)
			if len(lines) == 0 {
				continue
			}
			t.Errorf("%s spells %s on line(s) %v and never spells %s — %s retired that name into ADR 0097 D3's filter, so this reference resolves to nothing. A `-run` filter that selects nothing exits 0 and prints `ok`, which is the one failure this class refuses. Write the live name; a line that records the rename may keep both.",
				rel, r.retired, lines, r.live, r.bead)
		}
	}
	if scanned < 100 {
		t.Fatalf("census premise: only %d tracked files were readable, which is not this repo", scanned)
	}

	// The other direction, and it is the half a census cannot get by looking
	// harder: a table row whose LIVE name is declared nowhere is a rename
	// that renamed a test out of existence, and the census above would read
	// clean over it forever.
	for _, r := range renames {
		if !declared[r.live] {
			t.Errorf("wallVenueRenames says %s became %s, and no tracked file declares `func %s(` — the row is holding the tree to a name that is not in it", r.retired, r.live, r.live)
		}
	}
	t.Logf("censused %d tracked files against %d retired name(s)", scanned, len(renames))
}

// ARM 4 — the control: the dangling-reference matcher can still say no, and
// this file is not quietly exempting itself.
//
// It takes no repo root, so it is not a tree-wide pin and needs no door.
func TestQARetiredWallVenueNameCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	r := wallVenueRenames()[0]

	for _, c := range []struct {
		why  string
		body string
		want []int
	}{
		{"a bare stale reference", "see " + r.retired + " for the live join\n", []int{1}},
		{"a stale `-run` door", "\t$(GOBIN) test ./internal/posse -run '^" + r.retired + "$$'\n", []int{1}},
		{"two stale lines, reported where they are", "one\n" + r.retired + "\nthree\n" + r.retired + "\n", []int{2, 4}},
		{"a line that records the rename", r.retired + " is " + r.live + " now\n", nil},
		{"a file that names both, on different lines", "it was\n" + r.retired + "\nand it is\n" + r.live + "\nnow\n", nil},
		{"the live name alone", "see " + r.live + "\n", nil},
		{"a file about neither", "// nothing to do with the wall at all\n", nil},
		// The live name is a strict SUBSTRING of neither spelling, so this
		// row is the one that would catch a matcher rewritten to compare
		// prefixes: the retired name does not contain the live one.
		{"a stale reference in a file that names a different member", "TestLiveWallWatchPassesTheSeatsOwnExitCode\n" + r.retired + "\n", []int{2}},
	} {
		got := wallVenueDanglingLines(c.body, r)
		if len(got) != len(c.want) {
			t.Errorf("%s: wallVenueDanglingLines = %v, want %v", c.why, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: wallVenueDanglingLines = %v, want %v", c.why, got, c.want)
				break
			}
		}
	}

	// The matcher's own premise: the live name really is not a substring of
	// the retired one, for every row. If it were, "the file names the live
	// name" would be true of every file that names the retired one and this
	// census would be green over anything.
	for _, row := range wallVenueRenames() {
		if strings.Contains(row.retired, row.live) {
			t.Errorf("%s contains %s, so the per-file rule can never refuse a dangling reference to it — the row needs a different live spelling or this pin needs a different rule", row.retired, row.live)
		}
	}

	// THIS FILE EXEMPTS ITSELF FROM NOTHING, and it needs no arm to say so.
	// It is a tracked file of the tree arm 3 walks, and it spells every
	// retired name in the table — so it passes that census the only way any
	// file may: by spelling the live name beside it. Write a row here and
	// leave the live name out, and arm 3 reds on this file by name. That is
	// also why the retired spellings are written whole rather than assembled
	// from halves the way instancePathSpellings are: the per-file rule leaves
	// room for the registry to be readable, and a human grepping the retired
	// name should land here.
}

// wallVenueMemberPatternIsARegexp is the last premise of the pattern const:
// it has to compile, and it has to be anchored. A pattern that silently
// stopped being anchored would match `AlsoTestLiveWallX` and would quietly
// widen both D3's filter and D4's rule.
func TestQAWallVenueMemberPatternIsAnchored(t *testing.T) {
	t.Parallel()
	if !strings.HasPrefix(wallVenueMemberPattern, "^") {
		t.Fatalf("%s is not anchored — `-run` anchors each path element and D4's rule must read the same way", wallVenueMemberPattern)
	}
	re := regexp.MustCompile(wallVenueMemberPattern)
	for _, in := range []string{"AlsoTestLiveWallX", "xTestQALiveBwrapY", " TestLiveWallZ"} {
		if re.MatchString(in) {
			t.Errorf("%s matches %q, which no `-run` filter would select", wallVenueMemberPattern, in)
		}
	}
}

// ─── ranger-base-ig79v: the gate's WIRING, which the five pins above left ──
// ─── unread (findings 2(a) and 2(b) of the ranger-base-66v31 verify) ───────
//
// The five pins above read the gate's pure RULE five ways and its wiring not
// at all, and two mutants measured that gap (MEASURED 2026-11-14, each one
// literal replacement run against all five):
//
//  2(a) wallVenueActive's refusal arm, `t.Fatalf` to `t.Logf`, so a
//       POSSE_WALL_VENUE that is neither empty nor `1` is read as a quiet no
//       — the ONE reading ADR 0097 D1's own comment says it must never get.
//       SURVIVED. The parse was inline in a helper that takes a *testing.T,
//       and such a helper can only be exercised by a test willing to be
//       failed by it, so none of its three arms was pinned.
//
//  2(b) bwrapNestSkip's nestMeasure arm, `t.Log(msg)` to `_ = msg`, so the
//       membership marker is DECIDED and never EMITTED. SURVIVED, and the
//       consequence was measured on the same prebuilt binary: 34 pass, 0
//       fail, rc=0 and ZERO marker lines. `scripts/posse-wall-check` reads
//       membership off that line and nothing else (ADR 0097 D3), so the run
//       a venue would read is green, silent and uncountable — exit 3, filed
//       by D5's watchdog as "wall check silent" rather than as the wall
//       regression the run existed to find. That is ADR 0097's own
//       `/var/tmp` row arriving one mechanism over.
//
// Both are now pinned the way the rule is: by splitting the part that has no
// *testing.T in it (wallVenueFromEnv) out of the part that is three lines of
// plumbing, and by giving the action half a sink (nestSink) a pin can hand a
// recorder.

// ARM 5 — the env parse, over every value a renderer, an operator or a
// mis-render can hand it. Pure: no environment write, so it is parallel and
// needs no door.
//
// WHAT IS IN FORCE AND NOT WHAT D4 SAYS, deliberately. D4 writes what unset
// and `1` mean and is silent on the rest, so the refusal is an ASSUMED
// ranger-base-q749c flagged for the architecture lane. This pin holds the
// answer that is in force today, so narrowing it back to a quiet no is a diff
// that reds a row here — a decision someone makes — rather than a change
// nobody notices.
func TestQAWallVenueEnvParseIsADR0097D1s(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		why     string
		v       string
		venue   bool
		refused bool
	}{
		// The two values D1 is about. Unset is the ordinary case — every
		// seat and every operator `make test` — and the only quiet no.
		{"unset", "", false, false},
		{"the venue, exactly as landedcheck.go renders it", "1", true, false},

		// Everything else is a refusal and not a quiet no, because reading
		// it as "not the venue" abstains all 32 members, answers exit 3 and
		// reaches a reader as D5's "wall check silent".
		{"a hand-typed off", "0", false, true},
		{"a hand-typed yes", "yes", false, true},
		{"a hand-typed true", "true", false, true},
		{"a shouted true", "TRUE", false, true},
		{"a value nobody meant", "2", false, true},
		// The near misses, which is why the match is exact: no trimming and
		// no case folding, so a mis-render that appended a newline or padded
		// the value is refused rather than guessed at.
		{"a trailing newline", "1\n", false, true},
		{"a leading space", " 1", false, true},
		{"a trailing space", "1 ", false, true},
		{"a zero-padded one", "01", false, true},
		// A `%` in the value, which is the reason wallVenueActive hands the
		// refusal to t.Fatal and not to t.Fatalf: a verb in a format call
		// with no arguments prints `%!v(MISSING)` over the operator's own
		// typo.
		{"a value with a format verb in it", "%d%s", false, true},
	} {
		venue, refusal := wallVenueFromEnv(c.v)
		if venue != c.venue {
			t.Errorf("%s: wallVenueFromEnv(%q) venue = %v, want %v", c.why, c.v, venue, c.venue)
		}
		if (refusal != "") != c.refused {
			t.Errorf("%s: wallVenueFromEnv(%q) refused = %v, want %v (said: %q)", c.why, c.v, refusal != "", c.refused, refusal)
			continue
		}
		if !c.refused {
			continue
		}
		// A refusal has to be readable by the operator who typed the value:
		// it names the variable, quotes what was there, and says what D1
		// renders. A row that only asserted "non-empty" would pass over a
		// refusal that says nothing.
		for _, want := range []string{wallVenueEnvVar, fmt.Sprintf("%q", c.v), wallVenueEnvVar + "=1"} {
			if !strings.Contains(refusal, want) {
				t.Errorf("%s: the refusal for %q does not carry %s:\n  %s", c.why, c.v, want, refusal)
			}
		}
		if venue {
			t.Errorf("%s: wallVenueFromEnv(%q) refused AND answered venue — there is no arm in which a caller may carry on with both", c.why, c.v)
		}
	}
}

// ARM 6 — the wiring of that rule to the environment: which VARIABLE
// wallVenueActive reads, and that it returns the rule's answer unchanged.
//
// NOT PARALLEL, because it writes the environment (t.Setenv refuses a
// parallel test), and that is the whole reason this arm is separate from arm
// 5 rather than a row in it. Go resumes parallel tests only after the last
// sequential top-level test returns, so no member of the venue class is
// reading the variable while this arm holds it.
//
// THE REFUSAL ARM IS NOT HERE and cannot be: reaching it means failing this
// test with the words under test. Its words are arm 5's, and what is left
// unread is one `t.Fatal` call — the smallest residual the split leaves.
func TestQAWallVenueActiveReadsADR0097D1sVariable(t *testing.T) {
	for _, c := range []struct {
		why   string
		v     string
		venue bool
	}{
		// Empty is what os.Getenv answers for unset too, which is the one
		// shape t.Setenv cannot tell apart — and wallVenueFromEnv is a
		// function of the value, so neither can the rule.
		{"empty", "", false},
		{"the venue", "1", true},
	} {
		t.Run(c.why, func(t *testing.T) {
			t.Setenv(wallVenueEnvVar, c.v)
			if got := wallVenueActive(t); got != c.venue {
				t.Errorf("with %s=%q, wallVenueActive = %v, want %v — the helper reads some other variable, or drops the rule's answer", wallVenueEnvVar, c.v, got, c.venue)
			}
		})
	}
}

// nestRecorder is a nestSink that records instead of exiting, so a pin can
// read what the gate DID to its caller. The real sinks are a *testing.T's:
// two of them never return, and that is exactly why the mapping from action
// to sink could not be read from inside the class the gate gates.
type nestRecorder struct {
	helpers int
	logged  []string
	skipped []string
	failed  []string
}

func (r *nestRecorder) Helper()           { r.helpers++ }
func (r *nestRecorder) Log(args ...any)   { r.logged = append(r.logged, fmt.Sprint(args...)) }
func (r *nestRecorder) Skip(args ...any)  { r.skipped = append(r.skipped, fmt.Sprint(args...)) }
func (r *nestRecorder) Fatal(args ...any) { r.failed = append(r.failed, fmt.Sprint(args...)) }
func (r *nestRecorder) calls() (int, int, int) {
	return len(r.logged), len(r.skipped), len(r.failed)
}

// ARM 7 — the action half: each of D4's three actions reaches its own sink,
// carrying the words the rule decided, and reaches no other sink. This is
// finding 2(b)'s pin: `_ = msg` in the nestMeasure arm now reds here, and so
// does a wiring that logs where it should skip.
func TestQAWallVenueGateEmitsTheMarkerItDecides(t *testing.T) {
	t.Parallel()

	const refusal = "bwrap: setting up uid map: Read-only file system"
	for _, c := range []struct {
		why  string
		act  nestAction
		msg  string
		log  int
		skip int
		fail int
	}{
		{"nestMeasure logs the marker and does not exit", nestMeasure, wallVenueMarker, 1, 0, 0},
		{"nestAbstain skips with D3's sentence", nestAbstain, bwrapNestSkipPrefix + refusal, 0, 1, 0},
		{"nestFail is a red", nestFail, "the tree did this", 0, 0, 1},
	} {
		rec := &nestRecorder{}
		bwrapNestAct(rec, c.act, c.msg)
		log, skip, fail := rec.calls()
		if log != c.log || skip != c.skip || fail != c.fail {
			t.Errorf("%s: bwrapNestAct(%v) called Log/Skip/Fatal %d/%d/%d times, want %d/%d/%d",
				c.why, c.act, log, skip, fail, c.log, c.skip, c.fail)
			continue
		}
		// The words, not just the sink: a gate that reaches the right sink
		// with the wrong string is a marker line the venue cannot count and
		// an abstention a reader cannot act on.
		got := append(append(append([]string{}, rec.logged...), rec.skipped...), rec.failed...)
		if len(got) != 1 || got[0] != c.msg {
			t.Errorf("%s: the gate passed %q, want exactly %q", c.why, got, c.msg)
		}
		// t.Helper, so the marker line `go test -json` carries names the
		// MEMBER's own file and line — which is what the body's fixtures
		// spell and what a reader greps back to.
		if rec.helpers != 1 {
			t.Errorf("%s: Helper called %d times, want 1 — the gate's own file and line would stand where the member's belongs", c.why, rec.helpers)
		}
	}

	// And the composition, end to end over the one arm that is not an exit:
	// the marker the GATE emits is the marker the RULE decided, which is the
	// literal `scripts/posse-wall-check` greps. Arm 1 reads the rule's half
	// of this; the mutant finding 2(b) measured sat between the two.
	rec := &nestRecorder{}
	act, msg := bwrapNestDecision("TestLiveWallWatchPassesTheSeatsOwnExitCode", "", false)
	bwrapNestAct(rec, act, msg)
	if len(rec.logged) != 1 || rec.logged[0] != wallVenueMarker {
		t.Errorf("a member the gate lets proceed emitted %q, and the venue counts members by the line %q and by nothing else (ADR 0097 D3) — a run that decides the marker and never prints it is green, silent and uncountable", rec.logged, wallVenueMarker)
	}
}
