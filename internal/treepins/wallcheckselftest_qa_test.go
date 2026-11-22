package treepins

// QA pins for ranger-base-zpx1k — ADR 0097 D3's three exits, read by the suite.
//
// WHAT WAS UNDOORED. scripts/posse-wall-check maps a venue run to three exits
// (ADR 0097 D3): 0 green with the mandated last line, 1 for a member FAIL or a
// tree that does not build, 3 for a baseline that will not nest, zero members,
// or a member that abstained. That mapping is the whole of the file's value
// over `go test; echo $?`, and nothing in the suite read it. `make
// verify-python-syntax` censuses `.py` FILES, so a parser reached through
// `python3 -I -c` inside a shell script is outside its corpus — as it is for
// queue-scrub.sh, suite-lock.sh and ceiling-fill.sh — and a syntax check would
// not have caught what went wrong anyway.
//
// WHAT WENT WRONG, MEASURED 2026-11-14 (docs/notes.d/ranger-base-lagas.md).
// The first draft of the ABSTENTION arm — the one arm ADR 0097 D5's "an
// abstention is never a pass" exists for — carried a format string with three
// conversions and two arguments, a `%s` typed where `/var/tmp` was meant. It
// raised `TypeError` and the script exited 1: a RED where the contract says 3.
// D5's ratchet would have filed `wall pins red at <tree>` against whoever
// landed last, over a tree whose pins had merely gone dark. Reading the code
// did not find it; driving the parser over nine synthetic `go test -json`
// streams did. Same class as ranger-base-ietka — a closed-set pin whose
// failure message crashed on the one direction it existed for.
//
// SIX ARMS. The first three are gotestreuse_qa_test.go's three, for the same
// reasons — the door can be lost three ways:
//
//  1. the arms survive and nothing runs them. `make verify-wall-check` does,
//     and it is a prerequisite of `make test`; arm 1 pins both, because a
//     `--self-test` nobody invokes is the parked check this repo keeps
//     re-finding one location over.
//  2. the target survives and the arms are gutted. Arm 2 runs them and
//     requires each one BY LABEL — and, since ranger-base-h5iae, requires
//     D3's three exits off the exit each arm ASSERTS rather than off the
//     family its label names. So an arm that stopped being exercised goes red
//     rather than green by deletion, and a contract that stopped having an
//     exit goes red rather than green by relabelling.
//  3. arms 1 and 2 are both green over a self-test that cannot fail. Arm 3
//     reintroduces the exact TypeError above into a COPY of the script and
//     requires the self-test to notice AND to name the abstention arm while
//     doing it.
//
// Arm 5 is the second of those in the one direction a label can never show:
// it applies ranger-base-h5iae's own mutant — the parser stops answering 3
// and the four arms that wanted 3 stop asking — and requires the label list,
// the floor and the self-test itself all to stay GREEN over it while arm 2's
// exit check reds. Without that arm the bracket is a decoration that looks
// like a contract, which is what arm 2 was until this bead.
//
// And arm 6, which is about the one number in that body the suite could not
// otherwise read: `nonmember_ceiling`, the size of the deliberate non-member
// set and therefore the whole of the floor on n (ranger-base-c75wa). It has
// no other side in internal/posse, so it is held to the ADR instead.
//
// And arm 4, which is not about the door at all: six more exits are decided
// ABOVE the parser — the no-argument contract's 2, `--plan`'s 0, the cwd
// guard's 3, no bwrap, no python3, and a baseline that refuses — and they
// cannot be `--self-test` arms, because reaching them from inside the body
// means re-execing it and the only way to name the body from itself is `$0`
// or `$BASH_SOURCE`. A pin names the script already, so arm 4 drives all six
// from out here in 0.01s.
//
// WHY THE PARSER IS NOT A FILE OF ITS OWN, since that was the other shape the
// bead carried and a reader will ask. A `scripts/wall-check-parse.py` beside
// the body could only be found through a relative path — ADR 0088 D1 forbids
// resolving anything out of `$BASH_SOURCE` or `$0`, because that names a
// DIFFERENT tree — and a relative path under D1 names the tree being JUDGED.
// The shell would then come from the last green tree and the parser from the
// landing, so a landing could weaken its own judge. The variable keeps one
// file, one copy, and the two handshake literals still arriving as argv.
// docs/notes.d/ranger-base-zpx1k.md has the argument and what it cost.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The body, by path. Named rather than pattern-matched: this is one specific
// script carrying one specific contract.
const wcsScript = "scripts/posse-wall-check"

// And the record that contract is written in, for arm 6. If it moves, move
// this with it — a pin over a document it cannot find is a pin that measures
// nothing.
const wcsADR = "docs/adr/0097-the-walls-live-pins-measure-in-a-nesting-venue-after-each-landing.md"

// Every arm the self-test must still run, by the label it prints. The LABELS
// and not the script's own message strings: an arm that quietly stopped
// driving the parser would otherwise satisfy a scan of the file.
//
// The prefix names the family and is read as such below, against the exit
// that arm actually asserted — `green:`, `red:` and `abstention:` are D3's
// three exits (wcsFamilies), and a label that names one family over another's
// exit is a lie whichever half of it is wrong. It is NOT how the three exits
// are counted: a label is prose an author types, so counting them that way
// was ranger-base-h5iae's finding and wcsOKLine is the answer to it.
var wcsArms = []string{
	// Exit 0.
	"green: every member passed",
	"green: a NON-member failed and every member passed",
	"green: the mandated sentence is the LAST line",
	"green: the deliberate non-members printed no marker and n is the members alone",
	// Exit 1.
	"red: a member FAILed",
	"red: the tree did not build",
	"red: the marker and the failure come from a subtest",
	// Exit 3 — the arm that crashed in draft, and the three other ways a run
	// can be green by exit code and have measured nothing.
	"abstention: a member SKIPped after the marker",
	"abstention: tests ran and none printed the marker",
	"abstention: the filter selected no test at all",
	"abstention: a member printed the marker and reported no verdict",
	// And the floor on n (ranger-base-c75wa): a selected test that printed no
	// marker was counted by nothing, so a pin going dark left the run green
	// with a smaller n. The pair is both sides of one arithmetic — the
	// deliberate non-members are tolerated to the ceiling and the first one
	// past it is exit 3 — so a mutant that drops the floor has to lose a
	// `green:` label as well as an `abstention:` one.
	"abstention: a selected pin went dark, putting n under the derived floor",
	// And the ORDER of those two (ranger-base-5pkix F6). The floor arm sits
	// after the member arms on purpose — a member that abstained is the
	// diagnosis a reader can act on and the dark set is what is left over —
	// and until this arm nothing drove the placement: MEASURED 2026-11-22,
	// the floor block moved above the abstention arm left `--self-test` 12
	// arms clean, because no planted stream carried both shapes at once.
	// Both orders are exit 3, so the cost is a worse diagnosis and not a
	// wrong verdict, which is why the arm asserts the SENTENCE.
	"abstention: a member abstained AND a selected pin went dark",
}

// The three label families, and the exit each one claims. A label is prose
// and an exit is a fact, so the pair is worth asserting in its own right:
// an arm labelled `abstention:` over a want of 0 is a lie about the contract
// whichever half of it is wrong (ranger-base-h5iae).
var wcsFamilies = map[string]string{
	"green:":      "0",
	"red:":        "1",
	"abstention:": "3",
}

// A PASSING arm of the self-test, as `[<the exit it asserted>] <its label>`.
//
// THE BRACKET IS WHY THIS PIN IS A PARSE AND NOT A `strings.Contains`
// (ranger-base-h5iae). The labels in wcsArms are what an author types in an
// st_arm call, and nothing mechanical ties a label to the exit that arm
// drives — so a pin that read `ok    abstention: ...` was reading the author's
// intent and calling it the contract. ADR 0097 D3's exit 3 could leave the
// script entirely, in the one edit a future author narrowing D5 would make
// (drop the parser's `sys.exit(3)`, drop the arms' wants to match), and this
// file, `--self-test` and `make verify-wall-check` all stayed green: MEASURED
// 2026-11-14 at 61ec26f1, and arm 5 below is that mutant, kept.
//
// st_arm prints the bracket from `$want` — the same variable it compares the
// parser's answer against — so an arm can print `[3]` only by having asked
// for 3 and been answered 3. The labels still carry their own weight and are
// still required one by one: a rename reds the list while the brackets stay
// put, which is the half of this door that was already working.
var wcsOKLine = regexp.MustCompile(`(?m)^ok {4}\[([0-9]+)\] (.+)$`)

// wcsPassing parses one `--self-test` run's passing arms into
// (asserted exit, label) pairs, in the order the self-test printed them.
func wcsPassing(out string) [][2]string {
	var pairs [][2]string
	for _, m := range wcsOKLine.FindAllStringSubmatch(out, -1) {
		pairs = append(pairs, [2]string{m[1], strings.TrimSpace(m[2])})
	}
	return pairs
}

// wcsRange is the exit set of a run's passing arms as the self-test's summary
// line spells it: single digits in numeric order, anything else appended in
// the order it was seen. The mirror of the loop at the foot of run_self_test,
// and it is a mirror on purpose — the summary line used to print the literal
// "(ADR 0097 D3 exits 0/1/3, D5 abstention)" whatever the arms drove, which
// stated the contract's range in prose while proving nothing about it and
// said it loudest in the one case it was wrong (ranger-base-h5iae). It is
// derived there now, and this is what holds it derived: a literal put back
// disagrees with the brackets the moment an exit leaves the contract, which
// is the only moment it matters.
func wcsRange(pairs [][2]string) string {
	seen := map[string]bool{}
	var order []string
	for _, p := range pairs {
		if !seen[p[0]] {
			seen[p[0]] = true
			order = append(order, p[0])
		}
	}
	var out []string
	for _, d := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"} {
		if seen[d] {
			out = append(out, d)
			delete(seen, d)
		}
	}
	for _, e := range order {
		if seen[e] {
			out = append(out, e)
		}
	}
	return strings.Join(out, "/")
}

// wcsExitFindings returns every complaint ADR 0097 D3's three-exit contract
// makes about one run's passing arms: an exit no passing arm asserted, and an
// arm whose label claims a family its asserted exit contradicts.
//
// A function and not inline, because arm 5 requires it to be NON-empty over
// the mutant that removes exit 3 — so the check arm 2 makes and the check arm
// 5 proves can fail are the same code and cannot drift apart. That is the
// ranger-base-ietka lesson applied to this pin's own new half: a closed-set
// check that has never been seen to red in the direction it exists for is a
// comment.
func wcsExitFindings(pairs [][2]string) []string {
	var findings []string
	asserted := map[string]bool{}
	for _, p := range pairs {
		asserted[p[0]] = true
	}
	for _, exit := range []string{"0", "1", "3"} {
		if !asserted[exit] {
			findings = append(findings, fmt.Sprintf(
				"no passing arm of %s --self-test asserted exit %s — ADR 0097 D3 has three (0 green, 1 a member FAIL or a tree that does not build, 3 an abstention) and the suite must read all three (ranger-base-zpx1k, ranger-base-h5iae)",
				wcsScript, exit))
		}
	}
	for _, p := range pairs {
		prefix, _, ok := strings.Cut(p[1], ":")
		if !ok {
			continue
		}
		// An unknown family is left alone on purpose: a fourth exit on D3's
		// table would arrive as a fourth family, and this arm must not be the
		// thing that refuses it. What it refuses is a KNOWN family over the
		// wrong exit.
		want, known := wcsFamilies[prefix+":"]
		if !known || p[0] == want {
			continue
		}
		findings = append(findings, fmt.Sprintf(
			"the arm labelled %q asserts exit %s, and a %s: arm asserts %s (ADR 0097 D3) — the label and the exit disagree, so one of them is wrong",
			p[1], p[0], prefix, want))
	}
	return findings
}

// Arm 1: the self-test is invoked, and by a target `make test` runs.
//
// Both halves, because either one alone leaves the door shut. A target whose
// recipe no longer names the script runs nothing; a recipe that is right but
// off the `test:` line runs only when somebody types it, which is the state
// every check in scripts/verify-box.sh's EXCLUDED table was in before that
// table existed. The membership is read with mkPrereqs and not as bytes
// (ranger-base-hna69): a rename to a strict superstring leaves the old name on
// the line, and make treats everything after a `#` as a comment.
func TestQAMakeVerifyWallCheckRunsTheBodysSelfTest(t *testing.T) {
	t.Parallel()
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	recipe := makeRecipe(string(makefile), "verify-wall-check")
	if len(recipe) == 0 {
		t.Fatal("the Makefile has no `verify-wall-check` target — ADR 0097 D3's three exits are read by nothing again (ranger-base-zpx1k)")
	}
	var found bool
	for _, l := range recipe {
		if isComment(l) {
			continue
		}
		if strings.Contains(l, wcsScript) && strings.Contains(l, "--self-test") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("`make verify-wall-check` no longer runs %s --self-test:\n%s",
			wcsScript, strings.Join(recipe, "\n"))
	}

	deps, prereqs := mkPrereqs(t, string(makefile), "test")
	if !mkRuns(prereqs, "verify-wall-check") {
		t.Errorf("the `test:` line does not name verify-wall-check, so the parser's three exits are read only when somebody types the target: %q", deps)
	}
}

// Arm 2: the arms still run, each one still answers, and the exits they
// answered for are still D3's three. The self-test itself asserts the exit
// STATUS and a line of the message per arm — a status-only check would have
// caught the draft TypeError in the abstention arm and missed the same typo
// in the member-FAIL arm, where a crash and the verdict share exit 1.
//
// Four things are asked of the run, and only the first was asked before
// ranger-base-h5iae: every label in wcsArms is proved by a passing arm; the
// floor is met; each of D3's three exits is asserted by a passing arm, read
// off the BRACKET and not off the label (wcsOKLine says why that distinction
// is the whole bead); and the label's family agrees with the exit it asserted.
// The last two are wcsExitFindings, which arm 5 proves can red.
func TestQATheWallCheckSelfTestStillReadsTheThreeExits(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat(wcsScript); err != nil {
		t.Fatalf("%s is gone: %v", wcsScript, err)
	}
	// THE LITERAL AND NOT wcsScript, with a fence so the two cannot drift
	// apart. twdTrackedProgram resolves string literals and filepath ops over
	// them and never an identifier, so an exec written through the const is
	// invisible to TestQAEveryTrackedProgramATreePinRunsIsDispositioned —
	// which leaves this pin's program undispositioned, and that census's
	// two-way arm red over a row nothing appears to hand it. The literal at
	// the exec site is the shape suitelock_qa_test.go already uses for
	// scripts/gotest.sh, and it is why that program has a row at all.
	if wcsScript != "scripts/posse-wall-check" {
		t.Fatalf("wcsScript is %q and the exec below names scripts/posse-wall-check — one path, two spellings", wcsScript)
	}
	out, err := exec.Command("bash", "scripts/posse-wall-check", "--self-test").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --self-test failed: %v\n%s", wcsScript, err, out)
	}
	got := string(out)
	pairs := wcsPassing(got)
	byLabel := make(map[string]string, len(pairs))
	for _, p := range pairs {
		byLabel[p[1]] = p[0]
	}

	for _, want := range wcsArms {
		if _, ok := byLabel[want]; !ok {
			t.Errorf("%s --self-test no longer proves %q — the arm is gone, failing, or renamed\n%s",
				wcsScript, want, got)
		}
	}

	// A floor as well as the list, so an arm cannot be dropped at the same
	// time as its row here: thirteen is what this file names, and the count
	// only ever goes up. Over the PARSED lines rather than a substring count, so
	// an arm whose `ok` line lost its exit field counts for nothing here
	// instead of counting as proof.
	if n := len(pairs); n < len(wcsArms) {
		t.Errorf("%s --self-test reported %d passing arm(s) carrying an asserted exit; this file names %d:\n%s",
			wcsScript, n, len(wcsArms), got)
	}

	// And each of D3's three exits is asserted by at least one arm that
	// PASSED, read off the bracket and not off the label — see wcsOKLine for
	// what that distinction cost. Without this the list above could be
	// replaced wholesale by a set that reads one exit well and the other two
	// not at all, which is the shape ranger-base-zpx1k was filed for, and the
	// labels could keep saying otherwise, which is the shape
	// ranger-base-h5iae was.
	for _, f := range wcsExitFindings(pairs) {
		t.Errorf("%s\n%s", f, got)
	}

	// And the summary line a reader gets says the exits the arms DROVE. See
	// wcsRange: this is the second half of the same finding, and the half a
	// flattened bead tail actually carries.
	if want := "(ADR 0097 D3 exits " + wcsRange(pairs) + ","; !strings.Contains(got, want) {
		t.Errorf("%s --self-test's summary line does not say %q — it is claiming a range the arms above did not assert (ranger-base-h5iae)\n%s",
			wcsScript, want, got)
	}
}

// Arm 3: the self-test can actually fail, and fails where it should.
//
// A `--self-test` that prints its labels and exits 0 whatever happens
// satisfies arms 1 and 2 exactly as well as a working one does — that is the
// mutant that survived the first pass over gotestreuse_qa_test.go, and it is
// not hypothetical here either, because the defect this door exists for is a
// message that crashes rather than a verdict that is wrong.
//
// So: take the real body, put the draft's typo back — the `%s` where
// `/var/tmp` was meant, which makes the abstention message three conversions
// over two arguments — and require the self-test to go red AND to name the
// abstention arm. That is the one mutation whose live consequence is already
// MEASURED: `TypeError`, exit 1, where ADR 0097 D3 says 3.
func TestQATheWallCheckSelfTestCanFail(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(wcsScript)
	if err != nil {
		t.Fatal(err)
	}

	// The abstention message's own words. If this is gone the mutation lands
	// nowhere and the arm measures nothing, so say so rather than passing.
	const abstentionMessage = "the /var/tmp row of that record"
	if !strings.Contains(string(src), abstentionMessage) {
		t.Fatalf("%s no longer contains the abstention message this arm mutates (%q) — "+
			"the arm cannot fail and is therefore not evidence", wcsScript, abstentionMessage)
	}
	broken := strings.Replace(string(src), abstentionMessage, "the %s row of that record", 1)

	dir := t.TempDir()
	path := filepath.Join(dir, "posse-wall-check-broken")
	if err := WriteExecutable(path, []byte(broken), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("bash", path, "--self-test").CombinedOutput()
	if err == nil {
		t.Fatalf("%s --self-test passed with the draft TypeError put back: the self-test cannot fail, "+
			"so the other arms in this file prove nothing\n%s", wcsScript, out)
	}
	s := string(out)
	if !strings.Contains(s, "FAIL  abstention: a member SKIPped after the marker") {
		t.Errorf("the self-test failed, but not on the arm that was broken — it is refusing for "+
			"some other reason and is not reading the abstention exit\n%s", s)
	}
	// And it says WHY in the one way a reader of a flattened bead tail can
	// act on: the parser raised instead of answering. Without this the arm
	// above is satisfied by a self-test that reds the right label for the
	// wrong reason.
	if !strings.Contains(s, "the parser RAISED instead of answering") {
		t.Errorf("the self-test reddened the abstention arm without saying the parser raised — "+
			"a crash and a wrong verdict are the two failures this arm has to tell apart\n%s", s)
	}
}

// Arm 4: the exits the SHELL decides, driven from out here.
//
// `--self-test` covers the parser, which is where the draft defect was. It
// cannot cover the arms above the parser — the no-argument contract, the cwd
// guard, a host with no bwrap, a baseline that refuses, a host with no python3
// — because reaching them from inside the script means re-execing it, and the
// only way to name the script from itself is `$0` or `$BASH_SOURCE`, which ADR
// 0088 D1 forbids in one sentence. A PIN has no such problem: it names the
// script already, so it can hand it a cwd, a PATH and an argument of its
// choosing. Every one of these decides an exit on ADR 0097 D3's table, and
// until this arm not one of them was read by anything but a hand run.
//
// NONE OF THEM REACHES THE 83s RUN, which is what makes this cheap and is
// asserted rather than assumed: each arm checks that `step 2:` was never
// printed. That ordering is the script's own claim — both preflight asks
// happen BEFORE the run, because a venue run that then exits 127 for want of
// a parser is off D3's table entirely.
func TestQATheWallCheckShellArmsDecideTheirExits(t *testing.T) {
	t.Parallel()

	// A `bwrap` on PATH and nothing else, so the arms below can say whether
	// the baseline starts without needing the real one. `exit 0` is a venue
	// that nests; `exit 1` with a line on stderr is the seat-proc shape the
	// ADR's own probe measures.
	stubDir := func(body string) string {
		t.Helper()
		dir := t.TempDir()
		if err := WriteExecutable(filepath.Join(dir, "bwrap"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	for _, c := range []struct {
		name string
		args []string
		dir  string // "" is the repo root, which is this package's cwd
		path string // "" leaves the inherited PATH alone
		want int
		say  []string
	}{{
		name: "an unknown argument is exit 2, off D3's table",
		args: []string{"--bogus"},
		want: 2,
		say:  []string{"unknown argument --bogus", "takes no arguments and judges $PWD"},
	}, {
		name: "--plan runs nothing and is exit 0",
		args: []string{"--plan"},
		want: 0,
		say:  []string{"plan only: 2 steps, nothing run", "bwrap --ro-bind / / /bin/true"},
	}, {
		name: "a cwd that is not a tree is no answer, not a red",
		dir:  t.TempDir(),
		want: 3,
		say:  []string{"no internal/posse here", "is not a tree this check can judge"},
	}, {
		name: "a host with no bwrap is no answer",
		path: t.TempDir(),
		want: 3,
		say:  []string{"venue cannot nest: bwrap: not found on PATH"},
	}, {
		name: "a baseline that refuses is no answer, not a red",
		path: stubDir("#!/bin/sh\necho 'bwrap: setting up uid map: Read-only file system' >&2\nexit 1\n"),
		want: 3,
		say:  []string{"the baseline bwrap did not start", "venue cannot nest: bwrap: setting up uid map"},
	}, {
		name: "a host with no python3 is no answer, asked before the run",
		path: stubDir("#!/bin/sh\nexit 0\n"),
		want: 3,
		say:  []string{"a bwrap starts from here", "no answer — no python3 on PATH to read the run"},
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			argv := append([]string{"scripts/posse-wall-check"}, c.args...)
			cmd := exec.Command("bash", argv...)
			if c.dir != "" {
				// The cwd arm has to name the script absolutely, because the
				// whole point of it is that the cwd is NOT this tree. That is
				// the pin resolving the path, not the script resolving its
				// own — the thing ADR 0088 D1 forbids is the body doing it.
				abs, err := filepath.Abs("scripts/posse-wall-check")
				if err != nil {
					t.Fatal(err)
				}
				cmd = exec.Command("bash", append([]string{abs}, c.args...)...)
				cmd.Dir = c.dir
			}
			if c.path != "" {
				cmd.Env = append(os.Environ(), "PATH="+c.path)
			}
			out, err := cmd.CombinedOutput()
			got := 0
			if ee, ok := err.(*exec.ExitError); ok {
				got = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("running the body: %v\n%s", err, out)
			}
			s := string(out)
			if got != c.want {
				t.Errorf("exit %d, and ADR 0097 D3 says %d\n%s", got, c.want, s)
			}
			for _, want := range c.say {
				if !strings.Contains(s, want) {
					t.Errorf("the output does not say %q — the exit is right and the line a reader gets is not, which is the half D1 makes load-bearing (the runner keeps the last few lines)\n%s", want, s)
				}
			}
			// No arm here may pay for the venue run. `step 2:` is printed by
			// `--plan` too, so the marker is the line the run itself prints
			// afterwards; the preflight order is the script's own claim and
			// this is what holds it.
			if strings.Contains(s, "go test exited") {
				t.Errorf("this arm actually ran the pins — a preflight that answers AFTER the run costs a venue and still answers nothing (ADR 0097 D3)\n%s", s)
			}
		})
	}
}

// Arm 5: the exit field is load-bearing — the mutant arms 1-4 could not see.
//
// ranger-base-h5iae's finding was that arm 2 read the LABEL PREFIXES an
// author types and called that reading D3's three exits. The mutant it was
// filed with is the one a future author narrowing D5 would write, and it is
// deliberately not a typo: drop the parser's `sys.exit(3)` sites to 0, drop
// every st_arm want that asserts 3 to match, and the contract has two exits.
// MEASURED 2026-11-14 at 61ec26f1, before the bracket: `--self-test` reported
// "10 arms clean", exit 0, and the pin PASSED in 0.25s. Nothing else in the
// suite reads the parser's exit 3 — arm 4 above asserts 3 four times, but
// only for the exits the SHELL decides before the run, and
// internal/posse/wallcheck_test.go plants its own fixture script and never
// execs the real one.
//
// So this arm asserts the whole shape of that escape, not just its tail:
// the mutated self-test still passes, every label in wcsArms is still proved
// by a passing arm, the floor is still met — and wcsExitFindings, which is
// the exact code arm 2 runs, now complains, and complains about exit 3. An
// arm that only checked the last of those would pass over a mutant that
// reddened the self-test for some unrelated reason, which is how
// ranger-base-ietka's pin looked right for as long as it did.
func TestQATheWallCheckSelfTestExitFieldIsLoadBearing(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(wcsScript)
	if err != nil {
		t.Fatal(err)
	}
	mutant := string(src)

	// Half one: the parser stops answering 3 at all. Three sites — zero or no
	// members, a member that abstained, and (since ranger-base-c75wa) more
	// selected tests dark than the ceiling allows — and if any is gone the
	// mutant is not the mutant this arm is named for, so say so rather than
	// passing.
	const parserExit3 = "\n    sys.exit(3)"
	if n := strings.Count(mutant, parserExit3); n != 3 {
		t.Fatalf("%s has %d `sys.exit(3)` site(s) in its parser and this arm's mutant is written for 3 — "+
			"re-read the mutation below against the script before trusting it", wcsScript, n)
	}
	mutant = strings.ReplaceAll(mutant, parserExit3, "\n    sys.exit(0)")

	// Half two: the arms' wants follow it down, which is what keeps the
	// self-test honest with itself and is why the old pin stayed green. By
	// shape and not by the four stream variables' names: `st_arm <label>
	// <go test status> <want exit> <stream>`, and only the arms that want 3.
	want3 := regexp.MustCompile(`(?m)^(\s*st_arm .* [0-9]+) 3 ("\$[a-z_]+")`)
	if n := len(want3.FindAllString(mutant, -1)); n != 6 {
		t.Fatalf("%s has %d st_arm call(s) asserting exit 3 and this arm's mutant is written for 6 — "+
			"re-read the mutation below against the script before trusting it", wcsScript, n)
	}
	mutant = want3.ReplaceAllString(mutant, "${1} 0 ${2}")

	dir := t.TempDir()
	path := filepath.Join(dir, "posse-wall-check-two-exits")
	if err := WriteExecutable(path, []byte(mutant), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("bash", path, "--self-test").CombinedOutput()
	got := string(out)
	if err != nil {
		t.Fatalf("the mutated self-test FAILED, so this arm measured nothing: the mutant is the one the "+
			"self-test cannot see by itself — a parser that no longer answers 3 and arms that no longer "+
			"ask for it. If it reds now, this arm is not evidence for the bracket: %v\n%s", err, got)
	}

	pairs := wcsPassing(got)
	for _, label := range wcsArms {
		var seen bool
		for _, p := range pairs {
			if p[1] == label {
				seen = true
				break
			}
		}
		if !seen {
			t.Errorf("the mutant lost the arm labelled %q, so the label list reds on it and this arm is "+
				"measuring that instead of the exit field\n%s", label, got)
		}
	}
	if n := len(pairs); n < len(wcsArms) {
		t.Errorf("the mutant reported %d passing arm(s) against this file's %d, so the floor reds on it and "+
			"this arm is measuring that instead of the exit field\n%s", n, len(wcsArms), got)
	}

	// And the finding arm 2 makes. Both halves: that it complains at all, and
	// that it complains about the exit the mutant removed — a complaint about
	// a label family would be the right answer to the wrong question here.
	findings := wcsExitFindings(pairs)
	if len(findings) == 0 {
		t.Fatalf("a self-test whose parser cannot answer 3 and whose arms never ask for it is still "+
			"%d arms clean and raises no finding — the bracket is decoration and ADR 0097 D3's exit 3 "+
			"can leave the contract unread again (ranger-base-h5iae)\n%s", len(pairs), got)
	}
	var namesExit3 bool
	for _, f := range findings {
		if strings.Contains(f, "asserted exit 3") {
			namesExit3 = true
			break
		}
	}
	if !namesExit3 {
		t.Errorf("the mutant raised %d finding(s) and none of them says exit 3 went unasserted, which is "+
			"the one thing it removed:\n  %s\n%s", len(findings), strings.Join(findings, "\n  "), got)
	}

	// And the summary line, which is the half of this a reader gets without
	// running anything: it has to have stopped claiming the exit the mutant
	// removed: `exits 0/1` and not the `exits 0/1/3` the line this replaced
	// printed unconditionally. A literal put back fails this, because the
	// mutant's arms assert two exits and the literal names three.
	if want := "exits " + wcsRange(pairs) + ","; !strings.Contains(got, want) {
		t.Errorf("the mutant's summary line does not say %q, so the range is a literal again rather than "+
			"the exits the arms drove — the second half of ranger-base-h5iae's finding\n%s", want, got)
	}
}

// Arm 6: the floor on n is a number, and the record says which number
// (ranger-base-c75wa).
//
// WHY A PIN AND NOT A COMMENT. `nonmember_ceiling` is the one number in that
// body with no other side in internal/posse — D4's three literals are a
// handshake with wallvenue_test.go and are held to the ADR by
// TestQAWallVenueContractLiteralsAreADR0097s, and this one is held to nothing
// by that route because the code it would handshake with does not exist. What
// it IS is the size of a set somebody has to maintain by hand: the selected
// tests that print no marker on purpose, three on 2026-11-22 (MEASURED in the
// venue: 57 selected, 54 members). Raised without the record it is a floor
// quietly lowered — the script still says `green, <n> pins measured` and
// nothing says n may now be two pins short of the tree.
//
// The direction that matters is RAISED, because that is the one that loses
// coverage while every signal stays green. So: read the number out of the
// body and require the record to state it, which reds whichever side moved
// alone.
func TestQATheWallCheckNonMemberCeilingIsTheRecords(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(wcsScript)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^nonmember_ceiling=([0-9]+)$`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("%s no longer carries a `nonmember_ceiling=<n>` line, so n has no floor again and a pin "+
			"going dark reads as `wall check: green, <n> pins measured` with a smaller n (ranger-base-c75wa). "+
			"If the floor was deliberately retired, say so in %s and delete this arm rather than leaving a "+
			"green one.", wcsScript, wcsADR)
	}
	ceiling := string(m[1])

	adr, err := os.ReadFile(wcsADR)
	if err != nil {
		t.Fatalf("read %s: %v — this arm holds the body's floor to that record and cannot do it from a tree "+
			"that does not have it. If the record moved, move wcsADR with it.", wcsADR, err)
	}
	if want := "`nonmember_ceiling=" + ceiling + "`"; !strings.Contains(string(adr), want) {
		t.Errorf("%s says %s and %s does not carry %s. The number is the size of the deliberate non-member "+
			"set and it is what gives the member count a floor at all: raised on one side only, the floor "+
			"drops by the difference and every reader of the run stays green. Fix whichever side is wrong "+
			"— and if it is the body, the commit that raises it is the commit that adds the non-member.",
			wcsScript, want, wcsADR, want)
	}
	// AND THE ROSTER THE NUMBER IS THE SIZE OF (ranger-base-5pkix F5).
	//
	// The check above holds the two SPELLINGS of the number together, and a
	// bump that moves both sides at once satisfies it while naming no new
	// non-member — which is the one move this arm exists to catch. MEASURED
	// 2026-11-22: `nonmember_ceiling=3` -> `4` in the body AND in the ADR,
	// the roster still naming three, and this arm was GREEN. The floor had
	// dropped by one pin with every door in the tree green, which is
	// ranger-base-c75wa's own finding one layer up.
	//
	// So count the roster: the amendment's non-member list must name at
	// least `ceiling` tests, and a bump has to say WHICH test it is making
	// room for. Scoped to that section and not to the document, because the
	// ADR names a live pin in a bullet elsewhere (D2's false-pass row) and a
	// whole-document count would read it as a fourth non-member — the
	// 2026-11-22 bump would then have passed a four-name count as well.
	want, err := strconv.Atoi(ceiling)
	if err != nil {
		t.Fatalf("%s says nonmember_ceiling=%q, which is not a number", wcsScript, ceiling)
	}
	section := wcsRosterSection(string(adr))
	if section == "" {
		t.Errorf("%s no longer carries the non-member roster paragraph (the one anchored on %q), so the %s in the body is the size of a set this record does not name. The number is held to nothing again: a bump raises it on both sides, the floor drops by one pin, and every reader of the run stays green (ranger-base-5pkix F5, from ranger-base-c75wa).",
			wcsADR, wcsRosterAnchor, "nonmember_ceiling="+ceiling)
	} else {
		named := map[string]bool{}
		for _, m := range wcsTestName.FindAllStringSubmatch(section, -1) {
			named[m[1]] = true
		}
		if len(named) < want {
			names := make([]string, 0, len(named))
			for n := range named {
				names = append(names, n)
			}
			sort.Strings(names)
			t.Errorf("%s says nonmember_ceiling=%d and %s's roster names %d test(s) (%s). The number is the SIZE of that roster, so a ceiling past it is a floor lowered by the difference with nothing to audit it against: the commit that raises the ceiling is the commit that names the non-member it is making room for (ADR 0097's amendment, ranger-base-c75wa; the hole is ranger-base-5pkix F5).",
				wcsScript, want, wcsADR, len(named), strings.Join(names, ", "))
		}
	}

	// The premise. A pin over a document is satisfied by reading an empty one.
	if len(adr) < 2000 {
		t.Fatalf("%s is %d bytes — that is not the record this arm reads", wcsADR, len(adr))
	}
}

// The roster paragraph's anchor, and the test names inside it.
//
// The anchor is the HALF of that sentence that does not carry the count: the
// paragraph opens "**The three, named here because the body may not name
// them.**", and a ceiling bump is expected to rewrite "three" — so anchoring
// on the number would red on the one edit this arm is asking for.
const wcsRosterAnchor = "named here because the body may not name them"

var wcsTestName = regexp.MustCompile("`(Test[A-Za-z0-9_]+)`")

// wcsRosterSection returns the ADR's non-member roster paragraph and the
// bullet list under it: from the anchor to the next bold paragraph opener,
// which is how that record separates its paragraphs. "" when the anchor is
// gone, which the caller reports rather than passing over.
func wcsRosterSection(adr string) string {
	i := strings.Index(adr, wcsRosterAnchor)
	if i < 0 {
		return ""
	}
	section := adr[i:]
	if j := strings.Index(section, "\n**"); j > 0 {
		section = section[:j]
	}
	return section
}
