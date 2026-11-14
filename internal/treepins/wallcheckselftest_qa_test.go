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
// FOUR ARMS. The first three are gotestreuse_qa_test.go's three, for the same
// reasons — the door can be lost three ways:
//
//  1. the arms survive and nothing runs them. `make verify-wall-check` does,
//     and it is a prerequisite of `make test`; arm 1 pins both, because a
//     `--self-test` nobody invokes is the parked check this repo keeps
//     re-finding one location over.
//  2. the target survives and the arms are gutted. Arm 2 runs them and
//     requires each one BY LABEL, so an arm that stopped being exercised goes
//     red rather than green by deletion.
//  3. arms 1 and 2 are both green over a self-test that cannot fail. Arm 3
//     reintroduces the exact TypeError above into a COPY of the script and
//     requires the self-test to notice AND to name the abstention arm while
//     doing it.
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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The body, by path. Named rather than pattern-matched: this is one specific
// script carrying one specific contract.
const wcsScript = "scripts/posse-wall-check"

// Every arm the self-test must still run, by the label it prints. The LABELS
// and not the script's own message strings: an arm that quietly stopped
// driving the parser would otherwise satisfy a scan of the file.
//
// The prefix is load-bearing and is read as such below. `green:`, `red:` and
// `abstention:` are D3's three exits, and the bead's whole finding is that
// nothing read them — so a label set that lost one of the three would be this
// defect back with a full-looking list over it.
var wcsArms = []string{
	// Exit 0.
	"green: every member passed",
	"green: a NON-member failed and every member passed",
	"green: the mandated sentence is the LAST line",
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

// Arm 2: the arms still run, and each one still answers. The self-test itself
// asserts the exit STATUS and a line of the message per arm — a status-only
// check would have caught the draft TypeError in the abstention arm and missed
// the same typo in the member-FAIL arm, where a crash and the verdict share
// exit 1.
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

	for _, want := range wcsArms {
		if !strings.Contains(got, "ok    "+want) {
			t.Errorf("%s --self-test no longer proves %q — the arm is gone, failing, or renamed\n%s",
				wcsScript, want, got)
		}
	}

	// A floor as well as the list, so an arm cannot be dropped at the same
	// time as its row here: ten is what this file names, and the count only
	// ever goes up.
	if n := strings.Count(got, "ok    "); n < len(wcsArms) {
		t.Errorf("%s --self-test reported %d passing arm(s); this file names %d:\n%s",
			wcsScript, n, len(wcsArms), got)
	}

	// And each of D3's three exits is read by at least one arm that PASSED.
	// Without this the list above could be replaced wholesale by a set that
	// reads one exit well and the other two not at all, which is the shape
	// the bead was filed for.
	for _, exit := range []string{"green:", "red:", "abstention:"} {
		if !strings.Contains(got, "ok    "+exit) {
			t.Errorf("no passing arm of %s --self-test reads the %s exit — ADR 0097 D3 has three and the suite must read all three (ranger-base-zpx1k)\n%s",
				wcsScript, strings.TrimSuffix(exit, ":"), got)
		}
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
