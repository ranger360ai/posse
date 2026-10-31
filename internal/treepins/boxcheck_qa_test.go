package treepins

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
	"slices"
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
// EXCLUDED with a plausible sentence — these eight assert this machine and the
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
		// ranger-base-gf0ln. A runtime that updates itself moves away from
		// the version its skill discovery was MEASURED on, and the detector
		// for that (ADR 0007 §6c) had two readers, both display, both exit 0
		// over a drift — so bob's 2.0.5 → 2.0.7 reached the tree as an mtime
		// a pulse happened to read (ranger-base-lf6it). The condition
		// regenerates in the strongest sense on this roster: the CLI
		// regenerates itself.
		"verify-skills-drift",
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

// The aggregate's own self-test, run for real. Arms with controls, ~1s; the
// floor asserted below is ten, deliberately a floor and not a count — the
// count moves whenever an arm is added and two places holding it would drift.
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
		t.Fatalf("%s --self-test reported only %d passing arm(s); the floor is ten:\n%s", bcScript, n, s)
	}
	// Four arms by name. The first two are the verdicts an aggregate gets
	// wrong: it stops at the first red, and it calls "nothing measured" a
	// pass. The third is load-bearing for a ROSTER ROW rather than for the
	// runner's arithmetic (ranger-base-gf0ln): verify-skills-drift's program
	// is an installed binary, so it is resolved on PATH, and deleting that
	// arm would leave the count above well clear of its floor while the one
	// row of that shape went unmeasured.
	//
	// The fourth is load-bearing for the STATE FILE (ranger-base-4dhcw).
	// write_state replaces the operator's verdict whole and SeatbeltWritable
	// grants a.StateDir to every caged seat, so the gate that keeps a
	// dispatched seat from recording is the only thing between a cage
	// artifact and the verdict G10 reads. Its arm is the only place that
	// behaviour is executed — and a deleted gate leaves the suite green,
	// because every OTHER arm here runs with RHQ_PERSONA shed and so takes
	// the recording path either way.
	for _, want := range []string{
		"a finding in the middle does not stop the run",
		"all-2 is NOTHING MEASURED, not a pass",
		"a bare command is resolved on PATH, not under the repo root",
		"a dispatched seat records no verdict",
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
//
// The capture is the path RELATIVE TO scripts/, directory components and all,
// because the census below walks the directory rather than reading its top
// level (ranger-base-qrjt2). The two halves of a two-way census have to be
// keyed alike: a scripts/<dir>/verify-foo.sh that some recipe DOES run would
// otherwise be found by the walk, missed by a scan that only ever matched
// `scripts/verify-…`, and reported as a check nothing invokes. A directory
// component may not START with a dot, so `scripts/../verify-x.sh` captures no
// climb as a component.
var bcRecipeScriptRe = regexp.MustCompile(`scripts/((?:[A-Za-z0-9_-][A-Za-z0-9._-]*/)*verify-[a-z0-9-]+\.sh)`)

// bcRecipeScripts is every verify-* script some Makefile recipe actually runs,
// keyed by its path relative to scripts/ — the same key bcVerifyScripts
// returns.
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

// bcVerifyScripts is every verify-*.sh under scripts/, at ANY depth, keyed by
// its path relative to scripts/ — the same key bcRecipeScriptRe captures.
//
// A WALK, not filepath.Glob("scripts/verify-*.sh") (ranger-base-qrjt2, the
// same defect ranger-base-80zqb fixed in the third census of this class). A
// glob pattern reads ONE directory level, so the census below — whose whole
// subject is "a check shipped as a script that no target invokes" — could not
// see one shipped a directory down: on neither the recipe scan nor UNTARGETED,
// and red nowhere, which is this test's own defect one level in.
//
// scripts/ is flat today — 56 entries, 0 of them directories, MEASURED
// 2026-10-31 — so this is a claim made true about a directory shape nothing
// pins, not a live miss. And deliberately NOT the other way of making it true:
// an assertion that scripts/ holds no subdirectory is a layout ruling a QA pin
// has no business making, and it reds on the first legitimate scripts/lib/.
func bcVerifyScripts(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir("scripts", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if name := d.Name(); !strings.HasPrefix(name, "verify-") || !strings.HasSuffix(name, ".sh") {
			return nil
		}
		rel, err := filepath.Rel("scripts", path)
		if err != nil {
			return err
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk scripts/: %v", err)
	}
	sort.Strings(out)
	return out
}

// The census over the DIRECTORY, which is the half a target-keyed reader
// cannot do (ranger-base-bbl6r finding 1). Every scripts/verify-*.sh is either
// some Makefile recipe's invocation, or listed in UNTARGETED with the reason
// it has no target. Two-way: an UNTARGETED row for a script that does not
// exist, or that a recipe does run, is a stale sentence nobody will delete.
func TestQABoxCheckCensusCoversEveryVerifyScript(t *testing.T) {
	scripts := bcVerifyScripts(t)
	// Reader sanity before the arms trust it, in both directions: a walk that
	// found nothing and a recipe scan that matched nothing each make every
	// arm below vacuous, and each is silent about it.
	if len(scripts) < 15 {
		t.Fatalf("only %d verify-*.sh found under scripts/ — the walk is broken, not the tree: %v", len(scripts), scripts)
	}
	named := bcRecipeScripts(t)
	if len(named) < 12 {
		t.Fatalf("only %d verify-* script(s) found on a Makefile recipe line — the recipe scan is broken, not the Makefile: %v", len(named), named)
	}

	_, _, untargeted := bcCensus(t)

	for _, name := range scripts {
		switch {
		case named[name] && bcIn(untargeted, name):
			t.Errorf("%s is run by a Makefile recipe AND listed in the UNTARGETED table of %s — one place, one answer", name, bcScript)
		case !named[name] && !bcIn(untargeted, name):
			t.Errorf("scripts/%s is a verify-* check that no Makefile recipe runs and %s does not list in UNTARGETED.\n"+
				"That is the defect this file exists for in the one shape a target-keyed census cannot see: a check nothing invokes, with a green board over it.\n"+
				"Give it a target (and then classify the target in ROSTER or EXCLUDED), or list it in UNTARGETED with the reason it cannot have one.", name, bcScript)
		}
	}
	for name := range untargeted {
		if _, err := os.Stat(filepath.Join("scripts", name)); err != nil {
			t.Errorf("%s lists %s as untargeted, and there is no such script: %v", bcScript, name, err)
		}
	}
}

// The roster's PROGRAM SHAPE: the first word of a roster command is one of the
// two things the runner's cannot_run can resolve, and nothing else
// (ranger-base-gf0ln).
//
//	a REPO-RELATIVE PATH — every scripts/verify-*.sh row. The subject is the
//	box; the apparatus is this checkout, so it is resolved against the repo
//	root and must be executable there.
//
//	a BARE COMMAND NAME — a row whose program IS an installed binary, resolved
//	on PATH. `posse runtime drift` is the first, and the schedule's plist
//	populates PATH for exactly these rows.
//
// An ABSOLUTE path is the shape that looks like it must work and does not:
// cannot_run reads anything containing a slash through the repo-root branch,
// so /opt/homebrew/bin/foo is looked for at <root>/opt/homebrew/bin/foo and
// the row ERRORs on a box that has the binary. A relative path that is not
// executable here is the same failure one spelling over. Both are findings a
// person reads as "the box is wrong" when the ROSTER is wrong, which is worse
// than a check that does not run — and neither is visible until the clock
// fires, because the runner is the only other thing that resolves these.
func TestQABoxCheckRosterProgramsAreAShapeTheRunnerResolves(t *testing.T) {
	roster, _, _ := bcCensus(t)
	bare := 0
	for tgt, cmd := range roster {
		fields := strings.Fields(cmd)
		if len(fields) == 0 {
			// bcCensus already refuses an empty second field; this is the
			// reader saying so rather than indexing into nothing.
			t.Errorf("%s rosters %s with no command at all", bcScript, tgt)
			continue
		}
		prog := fields[0]
		switch {
		case strings.HasPrefix(prog, "/"):
			t.Errorf("the %s roster runs %q for %s, whose program is an ABSOLUTE path.\n"+
				"run_roster resolves anything with a slash against the repo root, so this row looks for %s under the checkout and ERRORs on a box that has the binary.\n"+
				"Name it bare and let PATH resolve it, or ship a script in this checkout.", bcScript, cmd, tgt, prog)
		case strings.Contains(prog, "/"):
			info, err := os.Stat(prog)
			if err != nil {
				t.Errorf("the %s roster runs %q for %s and there is no %s in this checkout: %v", bcScript, cmd, tgt, prog, err)
				continue
			}
			if info.Mode().Perm()&0o111 == 0 {
				t.Errorf("the %s roster runs %q for %s and %s is not executable — the runner scores that ERROR every run", bcScript, cmd, tgt, prog)
			}
		default:
			bare++
		}
	}
	// The reader again, in the direction that would make this test vacuous:
	// with no bare-name row on the roster, the branch above is never taken
	// and the arm says nothing about the shape it was written for.
	if bare == 0 {
		t.Errorf("no roster row names a bare command, so the PATH branch of run_roster's cannot_run is measured by nothing here (verify-skills-drift was the first such row)")
	}
}

// mkPrereqs reads a `target:` line out of the Makefile and returns the line
// whole, plus its prerequisites as TOKENS. Two edits that a substring test on
// the line reads as naming a target, and this does not (ranger-base-hna69):
// a rename to a strict superstring — `verify-parallel` to `verify-parallelx` —
// leaves the old name's bytes on the line, and make treats everything from a
// `#` as a comment, so a name after one is on the line and is not a
// prerequisite. Either takes the check off `make test` with the pin green.
//
// It takes the FIRST such line, which make itself may not: a target named
// twice — once inside an `ifeq` guard, once bare — lets make pick whichever
// arm the box actually takes while this always reads the first line on the
// page, so the guarded rule could be inert on every box that matters and this
// helper would never know (ranger-base-fus3g finding 3). Rather than guess
// which line make reads, it refuses a Makefile that names the target twice:
// the lane that wants to allow that shape has to teach this helper to ask
// make (`make -pn` / `-qp`), not to keep reading line one.
func mkPrereqs(t *testing.T, makefile, target string) (line string, deps []string) {
	t.Helper()
	var lines []string
	for _, l := range strings.Split(makefile, "\n") {
		if !strings.HasPrefix(l, target+":") || strings.HasPrefix(l, target+":=") {
			continue
		}
		lines = append(lines, l)
	}
	switch len(lines) {
	case 0:
		t.Fatalf("the Makefile has no `%s` target", target)
		return "", nil
	case 1:
		l := lines[0]
		rest := strings.TrimPrefix(l, target+":")
		if i := strings.Index(rest, "#"); i >= 0 {
			rest = rest[:i]
		}
		return l, strings.Fields(rest)
	default:
		t.Fatalf("the Makefile names `%s:` %d times — this helper cannot tell which line make actually reads on this box (a conditional can guard one of them), so a check pinned to whichever one comes first may be pinned to a line make never runs: %q", target, len(lines), lines)
		return "", nil
	}
}

// mkRuns is membership in that token list. Named, because the whole finding is
// that this reads identically to `strings.Contains(line, name)` at a call site
// and answers a different question.
func mkRuns(deps []string, name string) bool { return slices.Contains(deps, name) }

// The tokeniser itself, on the lines a substring test got wrong. Without this
// the only thing measuring the `#` stop and the superstring arm is a Makefile
// mutant, and the suite never runs one.
func TestQAMakefilePrereqsAreTokensNotBytes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		{"plain line", "test: fmt-check verify-parallel tree-check\n", []string{"fmt-check", "verify-parallel", "tree-check"}},
		{"commented tail", "test: fmt-check verify-test-times # verify-parallel tree-check\n", []string{"fmt-check", "verify-test-times"}},
		{"comment against a name", "test: fmt-check# verify-parallel\n", []string{"fmt-check"}},
		{"an assignment is not the target", "test:=x\ntest: fmt-check\n", []string{"fmt-check"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, got := mkPrereqs(t, c.src, "test")
			if !slices.Equal(got, c.want) {
				t.Errorf("mkPrereqs(%q) = %q, want %q", c.src, got, c.want)
			}
		})
	}
	if mkRuns([]string{"verify-parallelx"}, "verify-parallel") {
		t.Error("mkRuns says the `test:` line names verify-parallel when it names verify-parallelx — that is the substring test this replaced")
	}
	if !mkRuns([]string{"verify-parallel"}, "verify-parallel") {
		t.Error("mkRuns cannot find a name that is there — every arm above would pass on nothing")
	}
}

// An exclusion that claims `make test` runs the check is checked against the
// `test:` prerequisite line (ranger-base-bbl6r finding 2). Four rows say it
// today. If the prerequisite goes, the check runs NOWHERE and the exclusion
// table still reads "so CI runs it" — the same silence this file exists for,
// one level in. Two memberships of that line were already pinned, each by the
// test that cares (fmt-check in gofmtdoor_qa_test.go, verify-suite-lock in
// suitelock_qa_test.go); this generalises the rule to whoever claims it next.
//
// It asks mkPrereqs, not the line's bytes: reading the line as bytes let the
// same silence back in one spelling over, and took three of these four rows
// dark in a single edit (ranger-base-hna69).
func TestQABoxCheckExclusionsThatCiteMakeTestAreTrue(t *testing.T) {
	_, excluded, _ := bcCensus(t)

	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	deps, prereqs := mkPrereqs(t, string(mk), "test")

	claimed := 0
	for tgt, reason := range excluded {
		if !strings.Contains(reason, "make test") {
			continue
		}
		claimed++
		if !mkRuns(prereqs, tgt) {
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
