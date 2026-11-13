package treepins

// QA pins for ranger-base-tq5tj — `scripts/reap-test-tmp.sh --self-test`'s
// finding-4 arms must stay insensitive to claude processes this box is running
// for somebody else.
//
// THE DEFECT, AND WHY IT TOOK SIX BEADS. Finding 4 asks whether a `claude -p`
// run is excluded from the blindness control's live-claude count. That count
// is BOX-WIDE on purpose — every seat here is a claude process and the control
// wants all of them — so a rig that grades a DIFFERENCE of two such counts
// grades whatever the shop did between its two samples. It did exactly that
// until ranger-base-0wvu6 (7725ba75), and because verify-reap-test-tmp is a
// prerequisite of `make test` and `make test-arm1`, it red the suite for seats
// that had changed nothing: 3 of 8 back-to-back runs on 2026-11-12 over a
// comment-only edit, naming the reaper and no test the reader could act on.
// The family is ranger-base-2smyx, -0wvu6, -om4i3, -bttuj, -0whbv and -2ceq3
// across four personas, and the last two were each filed by someone reading a
// red that main had already fixed.
//
// THE RIG GRADES MEMBERSHIP NOW: is the pid this rig planted itself in the pid
// list the counter reports from the same loop as its count. Nothing in the
// tree said it stays that way, which is what this file is.
//
// WHY A SHAPE PIN AND NOT THE BEHAVIOURAL ONE (the decision ranger-base-tq5tj
// was filed to make). `scripts/reap-selftest-ambient.sh` is the deterministic
// detector — monotonic ambient claude-exe arrivals all the way through the
// self-test, so any two box-wide samples a second apart differ by
// construction. MEASURED 2026-11-13 on this box (linux 7.2.15 x86_64, 16
// cores, one-minute load average 2.0-2.9, /tmp a tmpfs) at a8575914:
//
//	tree                       --runs 3 verdict     wall/run   arrivals/run
//	HEAD (membership arms)     3 of 3 green         8s         8
//	7725ba75^ (delta spelling) 3 of 3 RED           24-25s     24-25
//
// So the detector works and its positive control holds. It is still not the
// pin, for two reasons the numbers above are half of:
//
//   - It costs the box. 8s and 8 planted claude-exe processes per run, on
//     every seat's `make test`, to re-answer what this file answers in
//     milliseconds off one os.ReadFile. (The bead's own estimate was 25-35s
//     and 25-35 processes; that figure was taken before ranger-base-hxenz
//     cut the self-test from 23-35s to 7-9s, and the harness plants one
//     process per second of the run it perturbs, so both halves fell with
//     it. 7-9s MEASURED here over three runs of `make verify-reap-test-tmp`:
//     9s, 7s, 7s. The pre-fix column is 24-25s because that tree predates
//     hxenz, not because the ramp is slower.)
//   - A behavioural pin reds the BEHAVIOUR, and the behaviour is already
//     right. What is unprotected is the EDIT, and an edit is what the four
//     re-discoveries each began with. This file reds the edit.
//
// Type the harness anyway when you have edited that rig: it is the only thing
// that says the verdicts do not move, and arm 1 below is a proxy for that, not
// a substitute. docs/notes.d/ranger-base-tq5tj.md has the readings.
//
// THE RULE ARM 1 HOLDS, and why it is not a roster of variable names. A delta
// needs a BASELINE — a value read from one census and still readable after a
// later one. That is the one thing every spelling of this defect must do and
// the one thing the fixed spelling never does: each arm captures a census,
// derives its fields, decides, and the next capture destroys them. So arm 1
// asks nothing about names and everything about program order: no
// census-derived value is read across a later census. The pre-fix spelling
// reds it on `n0`, which is captured once and read after two more censuses;
// the fixed spelling passes with four derived names; a rename of any of them
// changes nothing. The sources are the region of scripts/reap-test-tmp.sh that
// holds the rig, found by its own banner, and (as arm 1's positive control)
// that region as it stood at 7725ba75^, embedded below.
//
// WHAT ARM 1 DOES NOT REACH: a verdict that reads the box WITHOUT going
// through the rig's census function at all — a `pgrep -c claude` spelled into
// an arm. Arm 3 covers the fixture arms (their verdict must be a membership
// test over the census's own pid list), and the harness above covers the rest.
//
// NO PIN HERE IS TREE-WIDE: the three live arms read one named file with
// os.ReadFile and enumerate no directory, and the fourth reads a string
// constant, so none of them needs a Makefile door row
// (treewidedoor_qa_test.go's enumeration rule, whose own negative case is
// `os.ReadFile("Makefile")`). They run in arm 1's `./...`, where
// internal/treepins already runs whole, and they cost milliseconds.

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
)

const acScript = "scripts/reap-test-tmp.sh"

// acBanner opens the region and acNextBanner closes it. The region is found by
// its banner and not by line numbers, because ranger-base-w8n6w's ":625-632"
// had rotted inside a day (mktemproot_qa_test.go says so about the same file).
const (
	acBanner     = "finding 4's own rig"
	acNextBanner = "# ───"
)

// acLine is one line of a region with the line number a reader can open.
type acLine struct {
	num  int
	text string
}

func acRegionFromFile(t *testing.T) []acLine {
	t.Helper()
	all := strings.Split(readRepoFile(t, acScript), "\n")
	start := -1
	for i, l := range all {
		if strings.Contains(l, acBanner) {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s: no %q banner — this pin's region is gone or renamed, so it is reading nothing. "+
			"Point acBanner at whatever opens finding 4's rig now.", acScript, acBanner)
	}
	end := -1
	for i := start + 1; i < len(all); i++ {
		if strings.Contains(all[i], acNextBanner) {
			end = i
			break
		}
	}
	if end < 0 {
		t.Fatalf("%s: the %q banner is not followed by another %q banner, so the region has no end and this pin "+
			"would read the rest of the file.", acScript, acBanner, acNextBanner)
	}
	out := make([]acLine, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, acLine{num: i + 1, text: all[i]})
	}
	return out
}

// ─── the shell reading ──────────────────────────────────────────────────────

// acStripComment drops a trailing `#` comment, tracking quotes so that
// `${out#*"needle"}` and `${#cparr[@]}` keep their `#`: a comment's `#` is at
// the start of the line or preceded by whitespace, outside quotes.
func acStripComment(text string) string {
	inS, inD := false, false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if inS {
			if c == '\'' {
				inS = false
			}
			continue
		}
		if inD {
			if c == '\\' {
				i++
			} else if c == '"' {
				inD = false
			}
			continue
		}
		switch c {
		case '\'':
			inS = true
		case '"':
			inD = true
		case '\\':
			i++
		case '#':
			if i == 0 || text[i-1] == ' ' || text[i-1] == '\t' {
				return text[:i]
			}
		}
	}
	return text
}

// acSegments splits a line into command segments at `;`, `&&` and `||`,
// outside quotes. A single `&` or `|` is NOT a separator here, because `2>&1`
// is the commonest `&` in this script and splitting it would cut an
// assignment away from the command substitution it reads.
func acSegments(text string) []string {
	s := acStripComment(text)
	var segs []string
	inS, inD := false, false
	last := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inS {
			if c == '\'' {
				inS = false
			}
			continue
		}
		if inD {
			if c == '\\' {
				i++
			} else if c == '"' {
				inD = false
			}
			continue
		}
		switch c {
		case '\'':
			inS = true
		case '"':
			inD = true
		case '\\':
			i++
		case ';':
			segs = append(segs, s[last:i])
			if i+1 < len(s) && s[i+1] == ';' {
				i++
			}
			last = i + 1
		case '&', '|':
			if i+1 < len(s) && s[i+1] == c {
				segs = append(segs, s[last:i])
				i++
				last = i + 1
			}
		}
	}
	return append(segs, s[last:])
}

var (
	acFuncDef  = regexp.MustCompile(`^(\s*)([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{\s*$`)
	acFuncHead = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(\)`)
	acAssign   = regexp.MustCompile(`(?:^|[\s{(])(?:local\s+)?(?:-a\s+)?([A-Za-z_][A-Za-z0-9_]*)=`)
	acIdent    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	acCaseHead = regexp.MustCompile(`^\s*case\s+(.*?)\s+in\s*$`)
	acNumCmp   = regexp.MustCompile(`\s-(?:eq|ne|gt|lt|ge|le)\s`)
)

// acFunc is a shell function defined inside the region. body is [from, to),
// the lines between its `() {` and the `}` at the same indent.
type acFunc struct {
	name     string
	def      int
	from, to int
}

func acFuncs(t *testing.T, region []acLine) []acFunc {
	t.Helper()
	var fns []acFunc
	for i, l := range region {
		m := acFuncDef.FindStringSubmatch(l.text)
		if m == nil {
			// A one-line `f() { …; }` would be read as no function at all,
			// and every arm below would then be reading a region with no
			// census in it and passing. Say so instead.
			if h := acFuncHead.FindStringSubmatch(acStripComment(l.text)); h != nil {
				t.Fatalf("%s:%d: %q is a function definition this pin cannot read — it wants `name() {` alone on "+
					"the line, with its `}` at the same indent. Spell it that way or teach acFuncs the new shape; "+
					"left as it is, every arm here reads a region with no census in it and passes vacuously.",
					acScript, l.num, strings.TrimSpace(l.text))
			}
			continue
		}
		indent, name := m[1], m[2]
		end := -1
		for j := i + 1; j < len(region); j++ {
			if region[j].text == indent+"}" {
				end = j
				break
			}
		}
		if end < 0 {
			t.Fatalf("%s:%d: %s() has no closing `}` at its own indent inside the region.", acScript, l.num, name)
		}
		fns = append(fns, acFunc{name: name, def: i, from: i + 1, to: end})
	}
	return fns
}

// acInside reports whether a region index is a function's definition line, its
// closing brace, or inside its body.
func acInside(fns []acFunc, i int) bool {
	for _, f := range fns {
		if i >= f.def && i <= f.to {
			return true
		}
	}
	return false
}

// acCensusFns are the region's functions that RUN the production counter. The
// test is `"$0"` in the body: a census is an invocation of this very script,
// whatever the function is called. That is what makes arm 1's positive control
// possible at all — the pre-fix spelling's census was `count_claude_procs`.
func acCensusFns(fns []acFunc, region []acLine) []acFunc {
	var out []acFunc
	for _, f := range fns {
		for i := f.from; i < f.to; i++ {
			if strings.Contains(acStripComment(region[i].text), `"$0"`) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// The cache is shared by every arm in this file and the arms run in parallel,
// so it is locked. An unlocked map here is `fatal error: concurrent map read
// and map write` — not a FAIL anyone can read, and not recoverable.
var (
	acReMu    sync.Mutex
	acReCache = map[string]*regexp.Regexp{}
)

func acRe(key, pattern string) *regexp.Regexp {
	acReMu.Lock()
	defer acReMu.Unlock()
	re, ok := acReCache[key]
	if !ok {
		re = regexp.MustCompile(pattern)
		acReCache[key] = re
	}
	return re
}

// acMentions reports whether a segment READS a variable: `$n`, `${n…}`,
// `${#n…}`, or a bare `n` inside `$(( … ))`, which is how `$((n0 + 1))` reads
// n0 with no sigil of its own.
func acMentions(seg, name string) bool {
	q := regexp.QuoteMeta(name)
	return acRe("read:"+name, `\$\{?#?`+q+`\b|\$\(\(.*\b`+q+`\b`).MatchString(seg)
}

func acCalls(seg, name string) bool {
	return acRe("call:"+name, `\b`+regexp.QuoteMeta(name)+`\b`).MatchString(seg)
}

// acAssignTargets are the names a segment assigns: `n=…`, `local n=…`, and a
// `read`'s own targets, which is how `read -r -a cparr <<< "$cpids"` assigns.
func acAssignTargets(t *testing.T, seg string, num int) []string {
	t.Helper()
	var out []string
	for _, m := range acAssign.FindAllStringSubmatch(seg, -1) {
		out = append(out, m[1])
	}
	// `read` must be the segment's own COMMAND, not a word inside it: the
	// pre-fix fixture's `echo "  FAIL could not read claude_procs after the
	// -p run"` otherwise hands this scanner `claude_procs`, `after` and `the`
	// as variables it assigns, and the junk names then shadow the real ones.
	fs := strings.Fields(seg)
	k := -1
	for i, f := range fs {
		if acAssign.MatchString(f) {
			continue // an `IFS= read …` style prefix
		}
		if f == "read" {
			k = i
		}
		break
	}
	if k < 0 {
		return out
	}
	takesArg := map[string]bool{"-a": true, "-d": true, "-i": true, "-n": true, "-N": true, "-p": true, "-t": true, "-u": true}
	before := len(out)
	for i := k + 1; i < len(fs); i++ {
		f := fs[i]
		if strings.HasPrefix(f, "<") || strings.HasPrefix(f, ">") {
			break
		}
		if strings.HasPrefix(f, "-") {
			if takesArg[f] && i+1 < len(fs) {
				if f == "-a" && acIdent.MatchString(fs[i+1]) {
					out = append(out, fs[i+1])
				}
				i++
			}
			continue
		}
		if acIdent.MatchString(f) {
			out = append(out, f)
		}
	}
	if len(out) == before {
		t.Fatalf("%s:%d: a `read` this pin cannot name a target for: %q. A read whose target is unread is a "+
			"census value this arm cannot follow, so teach acAssignTargets the spelling rather than leaving the "+
			"arm blind to it.", acScript, num, strings.TrimSpace(seg))
	}
	return out
}

// acSeg is one segment of the region's straight-line code, outside every
// function body, in program order.
type acSeg struct {
	num  int
	idx  int // index into the region slice, for span arithmetic
	text string
}

func acArmSegs(region []acLine, fns []acFunc) []acSeg {
	var segs []acSeg
	for i, l := range region {
		if acInside(fns, i) {
			continue
		}
		for _, s := range acSegments(l.text) {
			if strings.TrimSpace(s) == "" {
				continue
			}
			segs = append(segs, acSeg{num: l.num, idx: i, text: s})
		}
	}
	return segs
}

// ─── arm 1: no census value is read across a later census ───────────────────

type acFinding struct {
	num       int
	name      string
	seg       string
	censusNum int
}

// acCrossCensusReads walks the region's straight-line segments in program
// order, tracking which names hold a census value and when each was last
// assigned. One forward pass is enough: a shell line reads what earlier lines
// assigned, so derivation only ever flows forward.
func acCrossCensusReads(t *testing.T, region []acLine, label string) (findings []acFinding, captures int, derivedSeen []string) {
	t.Helper()
	fns := acFuncs(t, region)
	census := acCensusFns(fns, region)
	if len(census) == 0 {
		t.Fatalf("%s: no function in finding 4's region runs the production counter (no `\"$0\"` in any function "+
			"body), so this arm has no census to measure a read against and would pass over anything.", label)
	}
	segs := acArmSegs(region, fns)

	assignedAt := map[string]int{}
	derived := map[string]bool{}
	lastCapture := -1
	lastCaptureNum := 0

	for i, s := range segs {
		// Reads first: a capture that also reads its own earlier value is
		// reading it BEFORE this census, not across it.
		for name := range derived {
			if !acMentions(s.text, name) {
				continue
			}
			if lastCapture > assignedAt[name] {
				findings = append(findings, acFinding{num: s.num, name: name, seg: strings.TrimSpace(s.text), censusNum: lastCaptureNum})
			}
		}
		isCapture := false
		for _, f := range census {
			if acCalls(s.text, f.name) {
				isCapture = true
				break
			}
		}
		if isCapture {
			lastCapture = i
			lastCaptureNum = s.num
			captures++
		}
		for _, name := range acAssignTargets(t, s.text, s.num) {
			fromCensus := isCapture
			if !fromCensus {
				for d := range derived {
					if acMentions(s.text, d) {
						fromCensus = true
						break
					}
				}
			}
			if fromCensus {
				if !derived[name] {
					derivedSeen = append(derivedSeen, name)
				}
				derived[name] = true
				assignedAt[name] = i
			}
			// A non-census assignment does NOT clear the taint, and does not
			// move the name's census date. The reading is MAY-hold, not
			// must-hold, because a static pass cannot follow paths: the
			// pre-fix spelling resets its baseline with `n0=0` on the
			// unreadable-census branch only, so on every other path n0 still
			// dates from the census two arms back — and that is the path the
			// verdict is wrong on.
		}
	}
	return findings, captures, derivedSeen
}

func TestQANoFindingFourVerdictReadsACensusValueAcrossALaterCensus(t *testing.T) {
	t.Parallel()
	region := acRegionFromFile(t)
	findings, captures, derived := acCrossCensusReads(t, region, acScript)

	// Non-vacuity, the defect ranger-base-bttuj found one layer down in these
	// same arms: an arm that proves nothing passes exactly like one that holds.
	if captures < 2 {
		t.Fatalf("%s: finding 4's region takes %d census(es). With fewer than two there is no later census for a "+
			"value to be read across, so this arm proves nothing about the rig it is pinning.", acScript, captures)
	}
	if len(derived) == 0 {
		t.Fatalf("%s: no name in finding 4's region holds a census value, over %d censuses. Either the rig stopped "+
			"capturing what the counter says, or acAssignTargets/acMentions stopped following it — both leave this "+
			"arm reading nothing.", acScript, captures)
	}

	for _, f := range findings {
		t.Errorf("%s:%d: %s is read here, but it was last assigned before the census at line %d:\n\t%s\n"+
			"A verdict in this rig must not be a delta of a population the rig does not own. The live-claude count "+
			"is BOX-WIDE, so a value carried across a census is a baseline that every sibling seat's launch moves: "+
			"3 of 8 back-to-back runs of this target red that way on 2026-11-12 over a comment-only edit "+
			"(ranger-base-0whbv), and this target is a prerequisite of `make test`. Capture one census per arm and "+
			"grade MEMBERSHIP of the pid the rig planted itself, in the pid list the counter reports from the same "+
			"loop as its count (ranger-base-0wvu6). `scripts/reap-selftest-ambient.sh` is the detector that says it "+
			"out loud.", acScript, f.num, f.name, f.censusNum, f.seg)
	}
}

// acPreFixRegion is finding 4's rig as it stood at 7725ba75^ — the last tree
// with the delta spelling — verbatim from scripts/reap-test-tmp.sh:463-518
// there, less three comment lines whose backticks a Go raw string cannot hold
// and which no arm here reads. It is this file's positive control: a rule that
// does not red THIS is not holding anything, and that is not a hypothetical —
// a clean rerun of the target passes the broken rig 8 times out of 8 on a
// quiet minute, which is the evidence two beads in this family were closed on
// (ranger-base-om4i3, ranger-base-0whbv).
const acPreFixRegion = `  local sleepbin fakeclaude n0 n1 n2 pfake
  sleepbin=$(command -v sleep) || { echo "  FAIL no sleep(1) on PATH to copy for the fake claude binary"; fails=$((fails + 1)); sleepbin=""; }
  fakeclaude="$tmp/claude"
  count_claude_procs() {
    local out rest
    out=$("$0" --root "$tmp/root" --class claude-scratch --quiet \
      --claude-sessions "$tmp/sessions" --claude-projects "$tmp/projects" --no-bead 2>&1) || true
    rest=${out#*"live claude processes: "}
    [ "$rest" = "$out" ] && { printf '\n'; return 1; }
    printf '%s\n' "${rest%%$'\n'*}"
  }
  if [ -n "$sleepbin" ]; then
    cp "$sleepbin" "$fakeclaude"
    chmod +x "$fakeclaude"

    n0=$(count_claude_procs)
    case $n0 in
      ''|*[!0-9]*)
        echo "  FAIL could not read 'live claude processes: N' back from the script's own output"
        fails=$((fails + 1))
        n0=0
        ;;
    esac

    "$fakeclaude" -p 30 >/dev/null 2>&1 </dev/null & pfake=$!
    sleep 1
    n1=$(count_claude_procs)
    kill "$pfake" 2>/dev/null || true
    wait "$pfake" 2>/dev/null || true
    case $n1 in ''|*[!0-9]*) echo "  FAIL could not read claude_procs after the -p run"; fails=$((fails + 1)); n1="($n1)" ;; esac

    if [ "$n1" = "$n0" ]; then
      echo "  ok   a 'claude -p' run is excluded from claude_procs ($n0 -> $n1)"
    else
      echo "  FAIL a 'claude -p' run changed claude_procs from $n0 to $n1 -- it must be excluded"
      fails=$((fails + 1))
    fi

    "$fakeclaude" 30 >/dev/null 2>&1 </dev/null & pfake=$!
    sleep 1
    n2=$(count_claude_procs)
    kill "$pfake" 2>/dev/null || true
    wait "$pfake" 2>/dev/null || true
    case $n2 in ''|*[!0-9]*) echo "  FAIL could not read claude_procs after the non-print run"; fails=$((fails + 1)); n2="($n2)" ;; esac

    if [ "$n2" = "$((n0 + 1))" ]; then
      echo "  ok   control: the same process without -p IS counted ($n0 -> $n2)"
    else
      echo "  FAIL control: a non-print claude process changed claude_procs from $n0 to $n2, want $((n0 + 1))"
      fails=$((fails + 1))
    fi
  fi
`

func acFixtureRegion() []acLine {
	all := strings.Split(strings.TrimSuffix(acPreFixRegion, "\n"), "\n")
	out := make([]acLine, 0, len(all))
	for i, l := range all {
		out = append(out, acLine{num: i + 1, text: l})
	}
	return out
}

func TestQATheCrossCensusReadRuleRedsTheDeltaSpellingItWasWrittenFor(t *testing.T) {
	t.Parallel()
	findings, captures, derived := acCrossCensusReads(t, acFixtureRegion(), "7725ba75^ fixture")
	if captures != 3 {
		t.Errorf("the 7725ba75^ fixture takes %d censuses, want 3 — the fixture or the reading has drifted.", captures)
	}
	if len(findings) == 0 {
		t.Fatalf("the cross-census read rule passes the delta spelling at 7725ba75^ (%d censuses, derived names %v). "+
			"That spelling red `make test` for seats that had changed nothing, 3 of 8 runs on 2026-11-12, so a rule "+
			"that does not red it is holding nothing and arm 1 above is decoration.", captures, derived)
	}
	// The baseline is the whole defect: n0 is captured once and read after two
	// later censuses, which is what makes both verdicts deltas.
	var sawN0 bool
	for _, f := range findings {
		if f.name == "n0" {
			sawN0 = true
		}
	}
	if !sawN0 {
		t.Errorf("the rule reds the 7725ba75^ fixture, but not on `n0`: %+v. n0 is the surviving baseline both "+
			"verdicts subtract from, so a red that misses it is reading something else.", findings)
	}
	if testing.Verbose() {
		for _, f := range findings {
			fmt.Printf("7725ba75^ control: line %2d reads %-3s across the census at line %2d — %s\n",
				f.num, f.name, f.censusNum, f.seg)
		}
	}
}

// ─── arm 2: one census is one run of the counter ────────────────────────────

func TestQAFindingFoursCensusRunsTheProductionCounterExactlyOnce(t *testing.T) {
	t.Parallel()
	region := acRegionFromFile(t)
	fns := acFuncs(t, region)
	census := acCensusFns(fns, region)
	if len(census) != 1 {
		t.Fatalf("%s: finding 4's region defines %d function(s) that run the production counter, want 1. Two "+
			"censuses are two populations, and the arms below them can then differ one from the other without any "+
			"name surviving a capture — which is what arm 1 watches for.", acScript, len(census))
	}
	f := census[0]
	runs, where := 0, 0
	for i := f.from; i < f.to; i++ {
		n := strings.Count(acStripComment(region[i].text), `"$0"`)
		if n > 0 && where == 0 {
			where = region[i].num
		}
		runs += n
	}
	if runs != 1 {
		t.Errorf("%s:%d: %s() invokes this script %d times, want 1 (first at line %d). The count and the pid list "+
			"it hands back are only each other's evidence when they come from ONE loop over the process table; two "+
			"runs inside one census are two populations, and a membership test over the second against a count from "+
			"the first is the same defect ranger-base-0wvu6 fixed, one scope smaller.",
			acScript, region[f.def].num, f.name, runs, where)
	}
}

// ─── arm 3: each fixture arm grades membership of its own planted pid ───────

// acPidNames are the names a function assigns from `$!` — the pids the rig
// planted itself, derived rather than spelled so that a rename of $pfake does
// not red this arm.
func acPidNames(t *testing.T, region []acLine, f acFunc) []string {
	t.Helper()
	var out []string
	for i := f.from; i < f.to; i++ {
		for _, s := range acSegments(region[i].text) {
			if !strings.Contains(s, "$!") {
				continue
			}
			out = append(out, acAssignTargets(t, s, region[i].num)...)
		}
	}
	return out
}

func TestQAEachFindingFourFixtureArmGradesMembershipOfItsOwnPlantedPid(t *testing.T) {
	t.Parallel()
	region := acRegionFromFile(t)
	fns := acFuncs(t, region)

	// The fixture-starting function is the one that plants a process and keeps
	// its pid; the stopping one is the one that kills those pids.
	var startFn, stopFn *acFunc
	var pids []string
	for i := range fns {
		if p := acPidNames(t, region, fns[i]); len(p) > 0 {
			startFn, pids = &fns[i], p
			break
		}
	}
	if startFn == nil {
		t.Fatalf("%s: no function in finding 4's region keeps the pid of a process it started (`$!`), so this arm "+
			"cannot find the fixture arms, let alone read their verdicts. Until 7725ba75 the rig started its "+
			"fixtures inline and graded a box-wide count — a region with no planted-pid function is that shape.",
			acScript)
	}
	for i := range fns {
		if fns[i].name == startFn.name {
			continue
		}
		var kills bool
		for j := fns[i].from; j < fns[i].to; j++ {
			seg := acStripComment(region[j].text)
			if !strings.Contains(seg, "kill") {
				continue
			}
			for _, p := range pids {
				if acMentions(seg, p) {
					kills = true
				}
			}
		}
		if kills {
			stopFn = &fns[i]
			break
		}
	}
	if stopFn == nil {
		t.Fatalf("%s: %s() plants %v, but no function in the region kills them, so a fixture arm has no end this "+
			"pin can read — and an unkilled claude-exe fixture is a process this rig leaves on the box.",
			acScript, startFn.name, pids)
	}

	segs := acArmSegs(region, fns)
	_, _, derived := acCrossCensusReads(t, region, acScript)
	isDerived := func(seg string) bool {
		for _, d := range derived {
			if acMentions(seg, d) {
				return true
			}
		}
		return false
	}

	type span struct{ from, to int } // indices into segs, exclusive
	var spans []span
	open := -1
	for i, s := range segs {
		switch {
		case open < 0 && acCalls(s.text, startFn.name):
			open = i
		case open >= 0 && acCalls(s.text, stopFn.name):
			spans = append(spans, span{open + 1, i})
			open = -1
		}
	}
	if len(spans) < 2 {
		t.Fatalf("%s: finding 4's region has %d complete %s…%s span(s), want at least 2 — the `-p` arm and the "+
			"control that says the exclusion is not excluding everything (ranger-base-bttuj). An arm that is not "+
			"there is an arm this pin reads as passing.", acScript, len(spans), startFn.name, stopFn.name)
	}

	for n, sp := range spans {
		var sawCase, sawPattern, caseOpen bool
		for i := sp.from; i < sp.to; i++ {
			s := segs[i]
			txt := acStripComment(s.text)
			trimmed := strings.TrimSpace(txt)

			if m := acCaseHead.FindStringSubmatch(txt); m != nil && isDerived(m[1]) {
				sawCase, caseOpen = true, true
			} else if trimmed == "esac" {
				caseOpen = false
			} else if caseOpen && strings.HasSuffix(trimmed, ")") {
				for _, p := range pids {
					if acMentions(trimmed, p) {
						sawPattern = true
					}
				}
			}

			// A test that reads the census COUNT inside a fixture arm is a
			// count-shaped verdict unless it is the sentinel for a census
			// that could not be read at all.
			if (strings.Contains(txt, "[ ") || strings.Contains(txt, "[[ ")) && isDerived(txt) &&
				!strings.Contains(txt, "ABSENT") {
				t.Errorf("%s:%d: span %d of the fixture arms tests a census value that is not the unreadable-census "+
					"sentinel:\n\t%s\nThe verdict of a fixture arm is whether the pid this rig planted (%v) is in the "+
					"census's own pid list — not what the count is, because the count is every seat on this box "+
					"(ranger-base-0wvu6).", acScript, s.num, n+1, trimmed, pids)
			}
			if (strings.Contains(txt, "$((") || acNumCmp.MatchString(txt)) && isDerived(txt) {
				t.Errorf("%s:%d: span %d of the fixture arms does arithmetic on a census value:\n\t%s\nThat is the "+
					"delta spelling: `want $((n0 + 1))` over a BOX-WIDE count is what red 3 of 8 runs on 2026-11-12 "+
					"(ranger-base-0whbv).", acScript, s.num, n+1, trimmed)
			}
		}
		if !sawCase {
			t.Errorf("%s: span %d of the fixture arms (lines %d-%d) has no `case` over a census value, so it does "+
				"not decide through the pid list the counter reported. Membership of %v in that list is the verdict "+
				"(ranger-base-0wvu6).", acScript, n+1, segs[sp.from].num, segs[sp.to-1].num, pids)
		}
		if sawCase && !sawPattern {
			t.Errorf("%s: span %d of the fixture arms (lines %d-%d) decides through a census pid list, but no "+
				"pattern in it names a pid this rig planted (%v) — so whatever it is grading, it is not its own "+
				"process.", acScript, n+1, segs[sp.from].num, segs[sp.to-1].num, pids)
		}
	}

	if testing.Verbose() {
		fmt.Printf("finding 4: %d fixture span(s), planted pids %v, census-derived names %v\n", len(spans), pids, derived)
	}
}
