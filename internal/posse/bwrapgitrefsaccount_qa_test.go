//go:build !posse_arm2 && !posse_arm3

package posse

// QA, ranger-base-axt1f — THE GIT-REFS WALL PINS SPEND THEIR ONE LINE ON A
// CAUSE, and these two pins are the behaviour half and the call-site half of
// that.
//
// THE CLASS IS docs/notes.d/ranger-base-a688i.md's, read a second time at a
// different member. `scripts/posse-wall-check` reports a failed member by
// `evidence()`: the first `file.go:NN:` line of that test's output which is
// not the venue marker, 300 characters, and nothing after it (ADR 0097 D3).
// The launcher puts that one line on a P1 bead. So a member's budget is ONE
// line, and a format string ending in `:\n%s` whose `%s` is the only account
// of WHY spends that budget on nothing.
//
// WHAT ARRIVED. ranger-base-axt1f's entire text was `bwrapgitrefslive_test.go:477:
// the seat's commit failed after an outside pack-refs --all --prune:` — a
// symptom, a colon, and a dropped tail. The red did not reproduce (the
// notes fragment has the counts), so the tail was the only account there was
// ever going to be, and it went out of reach in transit exactly as
// ranger-base-a688i's did.
//
// WHY A SOURCE PIN FOR THE CALL. The fold has an observable and is pinned on
// its own below; the CALL has none a behavioural test can reach. Every fatal
// it guards fires only when a real bwrap seat fails a real commit, which is
// the case neither of these two reds reproduced at any count. So what is read
// is the one thing that is always there: the call, in the functions that must
// make it — the same reasoning, and the same `qcs*` helpers, as
// callsite_qa_test.go (ranger-base-nqw8v).
//
// IT NAMES ITS ONE FILE AND ENUMERATES NOTHING, so it is not in
// internal/treepins' one-package-census class and needs no Makefile door
// (treewidedoor_qa_test.go's derived key: discovered file sets need a door,
// named ones do not). Neither pin is a member of the wall-check set and
// neither is named like one (`^Test(QA)?Live(Bwrap|Wall)`): they measure no
// wall and they need no bwrap, no venue and no fixture.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// bgaSubject is the one file both pins read. A rename has to bring them
// along, which is the point.
const bgaSubject = "bwrapgitrefslive_test.go"

// TestQABwrapGitRefsSeatAccountNamesItsStatusOnOneLine is the behaviour half:
// what a seat run in that file hands its caller.
//
// THE EXIT STATUS IS THE ROW THE RED DID NOT HAVE. Both runners there reduced
// `err` to `err == nil` before any message was formatted, so a git that
// exited 128 at `cannot lock ref` and a seat ended from outside produced
// byte-identical reports, and the P1 could not say which it had been. The
// two exec errors below are made by running a real `/bin/sh`, not
// hand-built, because the words are the product's and not this pin's:
// `exit status 7` is the wall or git, `signal: killed` is neither.
func TestQABwrapGitRefsSeatAccountNamesItsStatusOnOneLine(t *testing.T) {
	t.Parallel()
	exitErr := exec.Command("/bin/sh", "-c", "exit 7").Run()
	if exitErr == nil {
		t.Fatal("CONTROL: /bin/sh -c 'exit 7' exited 0, so this pin has no real exit error to render")
	}
	killErr := exec.Command("/bin/sh", "-c", "kill -9 $$").Run()
	if killErr == nil {
		t.Fatal("CONTROL: /bin/sh -c 'kill -9 $$' exited 0, so this pin has no real signal error to render")
	}

	for _, c := range []struct {
		name string
		err  error
		out  string
		want []string
	}{
		{"a git refusal carries the status and the words", exitErr,
			"fatal: cannot lock ref 'HEAD': unable to create directory\n",
			[]string{"exit status 7", "cannot lock ref"}},
		{"a seat ended from outside is named as a signal", killErr, "",
			[]string{"signal: killed", "NOTHING"}},
		{"an allowed run reads as exit 0", nil, "",
			[]string{"exit 0", "NOTHING"}},
		{"words with no error still carry them", nil, "it worked\n",
			[]string{"exit 0", "it worked"}},
	} {
		got := bgSeatAccount(c.err, c.out)
		for _, want := range c.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s: bgSeatAccount(%v, %q) = %q, want it to name %q", c.name, c.err, c.out, got, want)
			}
		}
		// THE WHOLE CONTRACT. Anything after a newline is what the wall
		// check drops, so an account that carries one has put part of
		// itself out of reach of the bead (ADR 0097 D3).
		if strings.ContainsAny(got, "\n\r") {
			t.Errorf("%s: bgSeatAccount(%v, %q) = %q — it carries a newline, so `scripts/posse-wall-check` drops what is after it", c.name, c.err, c.out, got)
		}
	}

	// A seat that said nothing reads as a finding and never as empty
	// quotes, because that absence IS the discriminator: no git error and
	// no bwrap refusal is the shape of a run ended from outside.
	for _, in := range []string{"", "\n", "  \n\t\n"} {
		if got := bgSeatAccount(nil, in); !strings.Contains(got, "NOTHING") {
			t.Errorf("bgSeatAccount(nil, %q) = %q, want the silence named as a finding", in, got)
		}
	}
}

// TestQABwrapGitRefsFailuresSpendTheirFirstLineOnTheCause is the call-site
// half, and it reads three things in that one file:
//
//   - no format string anywhere in it carries a newline. That is the shape
//     ranger-base-a688i named — `:\n%s` where the `%s` is the only account of
//     WHY — pinned as the simple rule a member file can hold rather than as a
//     judgement about which second lines are supporting output. A member that
//     genuinely needs a second line is a member whose second line the bead
//     will not get.
//   - both seat runners build their answer with bgSeatAccount. That call is
//     what puts the exit status into every message at once, and deleting it
//     leaves every live arm green.
//   - bgSeatAccount folds with wwOneLine. Two spellings of "put this on one
//     line" are two answers waiting to disagree about what a member's line may
//     contain, and wallwatchlive_test.go's is the one that is already pinned.
func TestQABwrapGitRefsFailuresSpendTheirFirstLineOnTheCause(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(bgaSubject)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, bgaSubject, src, 0)
	if err != nil {
		t.Fatal(err)
	}

	// The format verbs of testing.TB, which are the calls whose first
	// argument reaches a reader through evidence().
	verbs := map[string]bool{"Fatalf": true, "Errorf": true, "Logf": true, "Skipf": true}
	read := 0
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || len(c.Args) == 0 {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || !verbs[sel.Sel.Name] {
			return true
		}
		read++
		// Every string literal of the format expression, so a format
		// spelled as a concatenation is read too. Unquoted and not read as
		// source, so a raw-string literal carrying a real newline is caught
		// and a `\n` written inside one is not miscounted: what matters is
		// the byte the message ends up with.
		ast.Inspect(c.Args[0], func(x ast.Node) bool {
			lit, ok := x.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text, uerr := strconv.Unquote(lit.Value)
			if uerr != nil || !strings.ContainsAny(text, "\n\r") {
				return true
			}
			t.Errorf("%s:%d: this t.%s format string carries a newline. `scripts/posse-wall-check` reports a failed member by its FIRST `file.go:NN:` line and nothing after it (ADR 0097 D3, 300 chars), and the launcher puts that one line on a P1 bead — so a message that names a symptom and leaves the reason on line 2 reaches the bead as `the seat's commit failed after an outside pack-refs --all --prune:` with no cause in it, which is ranger-base-axt1f exactly. Fold it onto the first line (bgSeatAccount).",
				bgaSubject, fset.Position(lit.Pos()).Line, sel.Sel.Name)
			return true
		})
		return true
	})
	// The control: a pin that found nothing to read would pass for the
	// wrong reason, and this file's arms are built out of these calls.
	if read < 10 {
		t.Errorf("%s: this pin read %d testing format call(s), want at least 10 — it is meant to read every message in the file, so a count this low means it is no longer finding them", bgaSubject, read)
	}

	// EVERY RETURN OF EACH RUNNER, and not "the function calls it
	// somewhere". MEASURED while grading this pin: a mutant that left
	// packUnderALiveSeat's deadline fatal calling bgSeatAccount and took it
	// out of the RETURN — the normal path, the one every arm in the file
	// reads — passed an "is it called in this body" check with every live
	// arm green. What has to hold is that the value handed BACK is the
	// account, so that is what is read.
	for _, c := range []struct{ recv, name string }{
		{"bgLive", "walled"},
		{"bgLive", "packUnderALiveSeat"},
	} {
		fn, rfset := qcsFuncDecl(t, bgaSubject, c.recv, c.name)
		returns := 0
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			r, ok := n.(*ast.ReturnStmt)
			if !ok || len(r.Results) != 2 {
				return true
			}
			returns++
			if !qcsHas(qcsCalls(r.Results[1]), "bgSeatAccount") {
				t.Errorf("%s:%d: %s returns a second value that is not built by bgSeatAccount. That call is the whole of ranger-base-axt1f's fix — the exit status and the seat's folded words, on the one line `scripts/posse-wall-check` carries to the bead (ADR 0097 D3) — and this is the value every failure message in this file interpolates. Without it a git that exited 128 and a seat ended from outside report the same words, which is what made that P1 unanswerable.",
					bgaSubject, rfset.Position(r.Pos()).Line, qcsName(c.recv, c.name))
			}
			return true
		})
		// The control: a runner whose two-value return this pin cannot
		// find is a runner it is not reading.
		if returns == 0 {
			t.Errorf("%s has no two-value return for this pin to read — it is the seat runner whose answer every arm in %s interpolates, so a reshaped signature has to bring this pin along", qcsName(c.recv, c.name), bgaSubject)
		}

		// AND EACH RUNNER ROUTES THE ABSTENTION (ranger-base-evx07, pinned
		// here by ranger-base-2unmh). bgSeatEndedFromOutside decides whether
		// a member REDS or ABSTAINS, and the criterion's own pin
		// (TestBgSeatEndedFromOutsideReadsOnlyASupervisorSignalOverSilence)
		// calls the function DIRECTLY — so it grades the criterion and says
		// nothing about whether the two runners ask it. Neither can a live
		// run: the branch is reached only by a real silent supervisor kill,
		// which no fixture manufactures inside a runner. MEASURED
		// 2026-11-22: deleting BOTH call sites and running ./internal/posse
		// arm 1 UNFILTERED left it green (PASS, 0 FAIL), and the deletion is
		// the pre-fix behaviour — walled's refusal arms (`if ok { fail }`)
		// read a silent outside kill as the wall refusing, a FALSE PASS, and
		// its success arms read it as the red that ADR 0097 D5's ratchet
		// files as a P1 against whoever landed last. This is the positive
		// control that was missing.
		for _, want := range []string{"bgSeatEndedFromOutside", "bgSeatAbstain"} {
			if !qcsHas(qcsCalls(fn.Body), want) {
				t.Errorf("%s does not call %s. A seat the box SIGKILLs says nothing and exits 128+9, and without this guard that status reaches both of this runner's polarities as an answer about the wall: a refusal arm passes on it and a success arm reds on it (ranger-base-evx07, ADR 0086 D5's false pass). The abstention is ADR 0097 D3's exit 3 — never a pass, never a red.",
					qcsName(c.recv, c.name), want)
			}
		}
	}

	// And the fold is wwOneLine's, wherever in the body it is reached:
	// there is one call and a body that stopped making it is the mutant.
	if fn, _ := qcsFuncDecl(t, bgaSubject, "", "bgSeatAccount"); !qcsHas(qcsCalls(fn.Body), "wwOneLine") {
		t.Errorf("bgSeatAccount does not call wwOneLine. Two spellings of \"put this on one line\" are two answers waiting to disagree about what a member's line may contain, and wallwatchlive_test.go's fold is the one that is already pinned (TestQAWallWatchFixtureFoldsWhatTheWatcherSaidOntoOneLine, ranger-base-a688i).")
	}
}
