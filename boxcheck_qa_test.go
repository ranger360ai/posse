package posse

// QA pins for ranger-base-51z8j — the aggregate that makes the live-box checks
// run at all, and the census that stops its roster falling behind in silence.
//
// THE DEFECT. Every live-box verify-* check on this repo was written as a
// control, declared in the Makefile, and pinned by a QA test — and then
// invoked by nothing. Measured on the tree 2026-09-06, before verify-box and
// verify-box-self-test existed: of the 21 verify-* targets there were then,
// four are prerequisites of `make test` and so run in CI; two more have their
// SCRIPT run by a target a person types (verify-gate-freshness.sh --warn at
// the end of `make install`, verify-detection.sh --check-install at the end of
// `make install-detection`); the other fifteen executed only when a person
// typed them. No aggregate target listed one as a prerequisite, no workflow
// named one, no LaunchAgent ran one. So the codex cask moved on 2026-09-05 and
// nobody learned for a day (ranger-base-k4lza), and on the day this landed
// verify-hook-freshness was red in all three hooked repos with nothing tracking
// it (ranger-base-u8lmw).
//
// The QA tests that DO execute these scripts — bdpin_qa_test.go,
// grokpin_qa_test.go, hookfreshness_qa_test.go, credentialpaths_qa_test.go —
// run them against a scratch HOME and a stub PATH. That is right for pinning
// the logic and is exactly why none of them noticed: not one of them asks this
// machine anything.
//
// THE ARM THAT MATTERS IS THE CENSUS. An aggregate with a hand-written roster
// reproduces the defect one level up the moment someone adds a verify-* target
// and forgets it: the board stays green and the new check is unrun, which is
// the whole condition this bead is about. So TestQABoxCheckCensusCoversEvery
// VerifyTarget reads the verify-* targets out of the Makefile and the roster
// and the reasoned exclusions out of the script, and fails on any target in
// neither. It is a two-way assertion on purpose — a one-way "every roster
// entry is a real target" would pass forever on an empty roster.
//
// AND THE CENSUS IS OVER SCRIPTS, NOT TARGETS ALONE (ranger-base-bbl6r
// finding 1, escaped this bead's own close). Both of those lists are keyed on
// a MAKEFILE TARGET NAME, so the one shape the guard above could not see was
// the shape it exists for: a check shipped as scripts/verify-*.sh with no
// target at all is in neither list, is named by nothing, and has a green board
// over it. TWO were in the tree on the day this was written — verify-ghost-
// composer.sh, named by nothing but itself, and verify-orphan-report.sh, named
// only in three internal/posse test comments — and the close that wrote the
// census counted 21 verify-* TARGETS, swept the invocation channels, and never
// enumerated scripts/ at all, so neither script was ever classified. Neither is
// schedulable today, so nothing was unrun-and-needed; the gap was the guard's.
// TestQABoxCheckCensusCoversEveryVerifyScript closes it against the directory.
//
// THREE TABLES OF SENTENCES ARE NOW CHECKED RATHER THAN READ (findings 2 and
// 3). An exclusion that says `make test` runs it is checked against the `test:`
// prerequisite line — drop the prerequisite and the check runs NOWHERE while
// the table still says CI has it, which is this file's own defect one level in.
// A roster COMMAND is checked against that target's recipe — the runner
// hand-writes the invocation, and a recipe that gains or loses a flag would
// otherwise drift in silence (only a RENAMED script is loud today, because the
// runner's `[ ! -x ]` arm turns that into an ERROR).

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const bcScript = "scripts/verify-box.sh"

// makefileVerifyTargets is every `verify-*:` rule in the Makefile. The pattern
// is anchored at column 0 and requires the colon, so a mention inside a recipe
// or a comment is not a target — the `.PHONY` line names them all and must not
// be read as twenty declarations.
var bcTargetRe = regexp.MustCompile(`(?m)^(verify-[a-z0-9-]+):`)

func bcMakefileTargets(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range bcTargetRe.FindAllStringSubmatch(string(body), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

// bcCensus asks the script itself for its three lists rather than re-parsing
// its source here. A test that re-implements the parse can agree with a roster
// the script does not actually run (the fixture-must-plant-what-the-producer-
// renders shape); `--census` is the producer.
//
// EACH MAP CARRIES THE ROW'S SECOND FIELD, not just its name — a roster row's
// COMMAND, an exclusion's or an untargeted script's REASON. A census of names
// alone can only check names, and that is what let three tables of sentences
// go unmeasured (ranger-base-bbl6r findings 2 and 3): "already a prerequisite
// of make test, so CI runs it" was nobody's assertion to check, and the
// hand-written invocations beside the roster could drift off their recipes in
// silence.
func bcCensus(t *testing.T) (roster, excluded, untargeted map[string]string) {
	t.Helper()
	abs, err := filepath.Abs(bcScript)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(abs, "--census").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --census: %v\n%s", bcScript, err, out)
	}
	roster, excluded, untargeted = map[string]string{}, map[string]string{}, map[string]string{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			t.Fatalf("%s --census: unparsable line %q", bcScript, line)
		}
		name, second := parts[1], strings.TrimSpace(parts[2])
		// A row with an empty second field is a row that satisfies the census
		// by existing, which is the whole failure mode these tables have: an
		// exclusion with no reason, or a roster row with no command.
		if second == "" {
			t.Errorf("%s --census: the %s row %q carries an empty second field — a row must not satisfy the census by existing", bcScript, parts[0], name)
		}
		switch parts[0] {
		case "roster":
			roster[name] = second
		case "excluded":
			excluded[name] = second
		case "untargeted":
			untargeted[name] = second
		default:
			t.Fatalf("%s --census: unknown class %q", bcScript, parts[0])
		}
	}
	if len(roster) == 0 {
		t.Fatal("--census returned an empty roster: every arm below would pass on nothing")
	}
	return roster, excluded, untargeted
}

// bcIn is presence, spelled so an empty second field cannot read as absence.
func bcIn(m map[string]string, k string) bool {
	_, ok := m[k]
	return ok
}

// The census. Every verify-* target in the Makefile is either on the roster or
// excluded with a reason; nothing is in both, and nothing claimed is absent
// from the Makefile.
func TestQABoxCheckCensusCoversEveryVerifyTarget(t *testing.T) {
	targets := bcMakefileTargets(t)
	roster, excluded, _ := bcCensus(t)

	// Sanity on the reader before the arms trust it. Four is the count of
	// targets `make test` already carries, and they must all be found — a
	// regexp that matched nothing would make every arm below vacuous.
	if len(targets) < 15 {
		t.Fatalf("only %d verify-* targets parsed out of the Makefile — the reader is broken, not the roster: %v", len(targets), targets)
	}

	inMakefile := map[string]bool{}
	for _, tgt := range targets {
		inMakefile[tgt] = true
	}

	for _, tgt := range targets {
		switch {
		case bcIn(roster, tgt) && bcIn(excluded, tgt):
			t.Errorf("%s is both on the roster and excluded in %s — one place, one answer", tgt, bcScript)
		case !bcIn(roster, tgt) && !bcIn(excluded, tgt):
			t.Errorf("%s is a verify-* target in the Makefile and appears in neither the ROSTER nor the EXCLUDED table of %s.\n"+
				"That is the defect this file exists for, one level up: a check nothing runs, with a green board over it.\n"+
				"Put it on the roster if it asserts THIS MACHINE, or in EXCLUDED with the reason it is not on a clock.", tgt, bcScript)
		}
	}
	// And the other direction: a roster or exclusion naming a target that no
	// longer exists is a roster entry the runner will fail on (roster) or a
	// stale reason nobody will delete (excluded).
	for tgt := range roster {
		if !inMakefile[tgt] {
			t.Errorf("%s rosters %s, which is not a target in the Makefile", bcScript, tgt)
		}
	}
	for tgt := range excluded {
		if !inMakefile[tgt] {
			t.Errorf("%s excludes %s, which is not a target in the Makefile", bcScript, tgt)
		}
	}
}

// The roster's membership rule, stated as the specific targets that must be on
// it. A census alone cannot catch someone moving a live-box check into
// EXCLUDED with a plausible sentence — these seven assert this machine and the
// reason each one exists is that its condition REGENERATES.
func TestQABoxCheckRostersTheLiveBoxChecks(t *testing.T) {
	roster, excluded, _ := bcCensus(t)
	for _, want := range []string{
		"verify-grok-pin",           // the runtime rolls itself forward
		"verify-codex-pin",          // the cask moves on any brew upgrade
		"verify-bd-pin",             // a daemon outlives the binary it came from
		"verify-credential-paths",   // the file regenerates on a login flow
		"verify-hook-freshness",     // every hook on the box is a COPY
		"verify-gate-freshness",     // the gate copy is operator-owned, not rendered
		"verify-bd-no-relate-pairs", // one verb plants a pair, any day
	} {
		if !bcIn(roster, want) {
			t.Errorf("%s is a live-box check and must be on the %s roster (its EXCLUDED reason, if any: %q)", want, bcScript, excluded[want])
		}
	}
}

// Two exclusions that are not housekeeping, and would be real defects if
// someone "tidied" them onto the roster.
func TestQABoxCheckExcludesTheTwoThatMustNotBeScheduled(t *testing.T) {
	roster, excluded, _ := bcCensus(t)

	// It SPENDS A REAL TURN on the runtime under test (Makefile: RHQ_LIVE_
	// RUNTIME=... TestLiveRuntimeContractWalk). On a clock that is money on a
	// schedule, which crosses crew guardrail 1. It is event-triggered by
	// design: before switching a lane back onto a runtime, and after a bump.
	if bcIn(roster, "verify-runtime-walk") {
		t.Error("verify-runtime-walk spends a real turn — a schedule would be autonomous spending (crew guardrail 1). It stays typed.")
	}
	if !bcIn(excluded, "verify-runtime-walk") {
		t.Error("verify-runtime-walk must be excluded WITH ITS REASON, not merely absent")
	}

	// The TARGET reads HOME_DIR=examples, i.e. this repo's own seed PIDs — a
	// tree check wearing a live-box name. Its live readers (--live, --settings)
	// are off the target on purpose and the Makefile says why.
	if bcIn(roster, "verify-pid-deny-set") {
		t.Error("verify-pid-deny-set as a target reads HOME_DIR=examples — the repo, not the box. Rostering it would schedule a tree check.")
	}
	if !bcIn(excluded, "verify-pid-deny-set") {
		t.Error("verify-pid-deny-set must be excluded WITH ITS REASON, not merely absent")
	}
}

// The aggregate's own self-test, run for real. Ten arms with controls; ~1s.
// Without this the roster could be perfect and the runner could still stop at
// the first red, or score "nothing measured" as a pass — which is the exact
// silence the bead is about, rebuilt inside its own fix.
func TestQABoxCheckSelfTestStillFires(t *testing.T) {
	abs, err := filepath.Abs(bcScript)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(abs, "--self-test").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --self-test failed:\n%s", bcScript, out)
	}
	s := string(out)
	// A self-test that ran no arms exits 0 too. Count them.
	if n := strings.Count(s, "  ok   "); n < 10 {
		t.Fatalf("%s --self-test reported only %d passing arm(s); it has ten:\n%s", bcScript, n, s)
	}
	// Two arms by name, because these are the two verdicts an aggregate gets
	// wrong: it stops at the first red, and it calls "nothing measured" a pass.
	for _, want := range []string{
		"a finding in the middle does not stop the run",
		"all-2 is NOTHING MEASURED, not a pass",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("%s --self-test no longer runs the arm %q:\n%s", bcScript, want, s)
		}
	}
}

// The wiring. A script nobody is told to run is the same parked check in a new
// location, which is the sentence credentialpaths_qa_test.go already carries.
func TestQABoxCheckIsWiredIntoTheMakefile(t *testing.T) {
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	body := string(mk)
	for _, want := range []string{
		"verify-box:\n\tscripts/verify-box.sh\n",
		"verify-box-self-test:\n\t@scripts/verify-box.sh --self-test\n",
		"verify-box ",           // .PHONY
		"verify-box-self-test ", // .PHONY
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Makefile does not carry %q", want)
		}
	}

	info, err := os.Stat(bcScript)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s is not executable", bcScript)
	}
}

// It must stay read-only. Each rostered script is read-only by its own
// contract; the aggregate must not be the place a remediation grows, because
// a persona-writable tree that can repair the box is the shape
// verify-gate-freshness refuses to become (its comment: an --install flag here
// would be that tree, one flag away).
func TestQABoxCheckRemediatesNothing(t *testing.T) {
	src, err := os.ReadFile(bcScript)
	if err != nil {
		t.Fatal(err)
	}
	// CODE ONLY. This file's own header names pkill and killall in the
	// sentence explaining why it runs neither, and a scan that reads prose as
	// execution fails on its own documentation — so comment lines are dropped
	// first. The scan then stops at the self-test, which legitimately makes and
	// removes its own scratch tree.
	var code []string
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.Contains(line, "selftest_cleanup() {") {
			break
		}
		code = append(code, line)
	}
	body := strings.Join(code, "\n")
	if !strings.Contains(body, "run_roster()") {
		t.Fatal("the code scan cut before run_roster — it is measuring nothing")
	}
	for _, forbidden := range []string{
		"pkill", "killall",
		"brew pin", "brew upgrade", "brew unpin",
		"install-hooks",
		"rm -rf", "rm -f",
		"launchctl",
		"curl", "git push",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("%s must stay read-only — it must not run %q. A finding prints the line for a person to type.", bcScript, forbidden)
		}
	}
}

// bcRecipeScriptRe finds a scripts/verify-*.sh named on a RECIPE line. Only
// tab-indented lines are read: a mention in a Makefile comment is a mention,
// not an invocation, and reading one as a target's recipe is how a script goes
// on looking wired while nothing runs it.
var bcRecipeScriptRe = regexp.MustCompile(`scripts/(verify-[a-z0-9-]+\.sh)`)

// bcRecipeScripts is every verify-* script some Makefile recipe actually runs.
func bcRecipeScripts(t *testing.T) map[string]bool {
	t.Helper()
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	for _, line := range strings.Split(string(mk), "\n") {
		if !strings.HasPrefix(line, "\t") {
			continue
		}
		for _, m := range bcRecipeScriptRe.FindAllStringSubmatch(line, -1) {
			named[m[1]] = true
		}
	}
	return named
}

// The census over the DIRECTORY, which is the half a target-keyed reader
// cannot do (ranger-base-bbl6r finding 1). Every scripts/verify-*.sh is either
// some Makefile recipe's invocation, or listed in UNTARGETED with the reason
// it has no target. Two-way: an UNTARGETED row for a script that does not
// exist, or that a recipe does run, is a stale sentence nobody will delete.
func TestQABoxCheckCensusCoversEveryVerifyScript(t *testing.T) {
	scripts, err := filepath.Glob("scripts/verify-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	// Reader sanity before the arms trust it, in both directions: a glob that
	// matched nothing and a recipe scan that matched nothing each make every
	// arm below vacuous, and each is silent about it.
	if len(scripts) < 15 {
		t.Fatalf("only %d scripts/verify-*.sh found — the glob is broken, not the tree: %v", len(scripts), scripts)
	}
	named := bcRecipeScripts(t)
	if len(named) < 12 {
		t.Fatalf("only %d verify-* script(s) found on a Makefile recipe line — the recipe scan is broken, not the Makefile: %v", len(named), named)
	}

	_, _, untargeted := bcCensus(t)

	for _, path := range scripts {
		base := filepath.Base(path)
		switch {
		case named[base] && bcIn(untargeted, base):
			t.Errorf("%s is run by a Makefile recipe AND listed in the UNTARGETED table of %s — one place, one answer", base, bcScript)
		case !named[base] && !bcIn(untargeted, base):
			t.Errorf("scripts/%s is a verify-* check that no Makefile recipe runs and %s does not list in UNTARGETED.\n"+
				"That is the defect this file exists for in the one shape a target-keyed census cannot see: a check nothing invokes, with a green board over it.\n"+
				"Give it a target (and then classify the target in ROSTER or EXCLUDED), or list it in UNTARGETED with the reason it cannot have one.", base, bcScript)
		}
	}
	for name := range untargeted {
		if _, err := os.Stat(filepath.Join("scripts", name)); err != nil {
			t.Errorf("%s lists %s as untargeted, and there is no such script: %v", bcScript, name, err)
		}
	}
}

// An exclusion that claims `make test` runs the check is checked against the
// `test:` prerequisite line (ranger-base-bbl6r finding 2). Four rows say it
// today. If the prerequisite goes, the check runs NOWHERE and the exclusion
// table still reads "so CI runs it" — the same silence this file exists for,
// one level in. Two memberships of that line were already pinned, each by the
// test that cares (fmt-check in gofmtdoor_qa_test.go, verify-suite-lock in
// suitelock_qa_test.go); this generalises the rule to whoever claims it next.
func TestQABoxCheckExclusionsThatCiteMakeTestAreTrue(t *testing.T) {
	_, excluded, _ := bcCensus(t)

	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	var deps string
	for _, line := range strings.Split(string(mk), "\n") {
		if strings.HasPrefix(line, "test:") {
			deps = line
			break
		}
	}
	if deps == "" {
		t.Fatal("the Makefile has no `test` target")
	}

	claimed := 0
	for tgt, reason := range excluded {
		if !strings.Contains(reason, "make test") {
			continue
		}
		claimed++
		if !strings.Contains(deps, tgt) {
			t.Errorf("%s excludes %s on the grounds that `make test` runs it — %q — and the `test:` line does not name it: %q.\n"+
				"Either put it back on that line, or stop citing it: an exclusion nothing runs is a check on no clock at all.", bcScript, tgt, reason, deps)
		}
	}
	// The reader again: a phrase match that stopped matching would pass this
	// test on nothing at all, silently, which is the shape being pinned.
	if claimed < 4 {
		t.Fatalf("only %d exclusion(s) cite `make test`; four do — the reason scan is broken, not the table", claimed)
	}
}

// The roster hand-writes a COMMAND per target, and this is the arm that says it
// is still that target's own recipe (ranger-base-bbl6r finding 3). A recipe
// that gains or loses a flag drifts in silence otherwise; only a RENAMED script
// is loud today, because the runner's `[ ! -x ]` arm turns that into an ERROR.
// The failure this catches is running the wrong invocation under the target's
// name — a check that reports on something other than what `make <target>` does.
func TestQABoxCheckRosterCommandsAreTheirTargetsRecipe(t *testing.T) {
	roster, _, _ := bcCensus(t)

	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	// target -> its recipe lines, `@` and `-` prefixes stripped: those are
	// make's echo and ignore-errors modifiers and not part of the command.
	recipes := map[string][]string{}
	var cur string
	for _, line := range strings.Split(string(mk), "\n") {
		if m := bcTargetRe.FindStringSubmatch(line); m != nil {
			cur = m[1]
			recipes[cur] = nil
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			cur = ""
			continue
		}
		if cur != "" {
			recipes[cur] = append(recipes[cur], strings.TrimLeft(strings.TrimPrefix(line, "\t"), "@-"))
		}
	}
	if len(recipes) < 15 {
		t.Fatalf("only %d verify-* recipe(s) parsed out of the Makefile — the reader is broken, not the roster", len(recipes))
	}

	for tgt, cmd := range roster {
		got, ok := recipes[tgt]
		if !ok {
			t.Errorf("%s rosters %s, which has no recipe in the Makefile", bcScript, tgt)
			continue
		}
		if len(got) != 1 || got[0] != cmd {
			t.Errorf("the %s roster runs %q for %s; `make %s` runs %q.\n"+
				"The roster invocation must BE the target's recipe — a check reporting under a target's name while running something else is worse than one that does not run.",
				bcScript, cmd, tgt, tgt, strings.Join(got, " && "))
		}
	}
}
