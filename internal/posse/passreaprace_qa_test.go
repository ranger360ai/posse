//go:build !posse_arm2 && !posse_arm3

package posse

// The flake class ranger-base-rviq5 closed, held open by two pins: a test
// that runs a full dispatch pass over a CLOSED bead and then reads the
// session that pass launched is reading a record the pass's own epilogue
// sweep may legitimately have reaped.
//
// THE SHAPE, in the words of the incident (ranger-base-191em, which found
// one test of it and filed the rest). The fixture marks its bead closed —
// which most dispatch fixtures do, because `closed by ranger`, the
// verify-after rungs and the merge-back are what a pass is read for — so the
// session the pass just launched is a legitimate Dial F reap candidate the
// moment it settles: bead closed, agent idle, per-bead name (autoreap.go,
// reapClassOf). The only guard holding the sweep off it is promptedRecently,
// and PromptGrace unset is 30 seconds against a pass that budgets itself
// DefaultGatherWindow — three minutes, with a DefaultPromptWaitMS leg of
// fifteen inside it. MEASURED 2026-11-03, with the fake herdr's
// prompt-delay-ms lever holding the gather's `pane prompt --wait` leg at
// 35s: the sweep prints `reaped ranger-003-a-1 (bead a-1 closed)` and
// REMOVES the record one line before the test reads it.
//
// The reap is correct and is nobody's bug. What the two pins hold is the
// test side of it:
//
//   - TestQATestDispatcherOutlastsItsOwnPass — newTestDispatcher's grace is
//     past anything a box can do to one pass, so the default is safe and a
//     test added tomorrow inherits that without knowing any of this.
//   - TestQANoPassRunningTestDiscardsAReadMetaOk — and where the record IS
//     gone, the test says so as its own fact instead of dereferencing nil.
//     A discarded `ok` made that case a SIGSEGV, and a panic in a parallel
//     package takes the WHOLE arm's binary down: every other test in the arm
//     reads as failed with no test named.
//
// This file is arm 1 because its door takes no `-tags` (make pkgcensus-check),
// and the census below reads the package's test files as TEXT, so it sees
// every arm's files whichever arm it is compiled into.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestQATestDispatcherOutlastsItsOwnPass is the behavioural half, and it is
// behavioural on purpose: the fact worth holding is a RELATION between two
// numbers — the grace every test dispatcher carries against the budgets that
// bound one pass — and a grep for the literal `time.Hour` in herdr_test.go
// would hold neither number and would go green on a grace of an hour against
// a gather window somebody later raised to two.
func TestQATestDispatcherOutlastsItsOwnPass(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// What bounds one pass when a test sets neither knob, which is what
	// every test that does not mention them does: the gather window, with
	// one --wait leg inside it. Summed rather than maxed because the sum is
	// the conservative reading and the point is a margin, not a fit.
	pass := DefaultGatherWindow + time.Duration(DefaultPromptWaitMS)*time.Millisecond
	if d.PromptGrace <= pass {
		t.Fatalf("a test dispatcher's PromptGrace is %s and one pass may legitimately take %s "+
			"(DefaultGatherWindow %s + a DefaultPromptWaitMS leg of %s) — so the epilogue sweep can "+
			"reap the session the same pass just launched, over a fixture's CLOSED bead, and remove "+
			"the record, the tree and the branch that the test is about to read (ranger-base-rviq5; "+
			"newTestDispatcher, herdr_test.go, says why the grace is what moves and not the pass)",
			d.PromptGrace, pass, DefaultGatherWindow,
			time.Duration(DefaultPromptWaitMS)*time.Millisecond)
	}
	// And the other direction, so this pin cannot be satisfied by disabling
	// the guard it is about: a zero or negative grace is "never recent" and
	// would have the sweep reap on every pass instead of none.
	if d.PromptGrace <= 0 {
		t.Fatalf("a test dispatcher's PromptGrace is %s — promptedRecently reads that as never recent, "+
			"so every pass's epilogue reaps the session it just launched", d.PromptGrace)
	}
}

// reapRaceRunCall is a dispatch pass: `Dispatcher.Run(dir, persona, max)`
// takes three arguments, where the two other `.Run(` in this package's tests
// take none (`exec.Cmd.Run`, `testing.M.Run`) — so "a receiver other than
// `t` with a non-empty argument list" is the whole discriminator, and it
// needs no knowledge of which local name a test gave its dispatcher. MEASURED
// 2026-11-04 over every *_test.go in this package: 370 dispatch calls on
// eleven receiver names (d 321, d2 38, d3 3, and one each of again, cd,
// dOther, dry, first, quiet, restart, second), against 27 zero-argument
// calls on cmd, c, done and m. `t.Run` is the one two-argument caller, and
// it is excluded by name.
var reapRaceRunCall = regexp.MustCompile(`\b([A-Za-z_]\w*)\.Run\([^)]`)

// reapRaceDiscardedOk matches a readMeta whose second result is thrown away,
// in both spellings tests use: the statement form and the `if m, _ := …;`
// one. The needle is assembled rather than written, because this file would
// otherwise be the census's own first hit.
var reapRaceDiscardedOk = regexp.MustCompile(`,\s*_\s*:?=\s*\w+\.` + "readMeta" + `\(`)

// A test that runs a dispatch pass must read readMeta's `ok`.
//
// Scoped to pass-running tests and not to every readMeta caller (73 of this
// package's test files read a meta), because a pass is where the record can
// go away UNDER the test: everywhere else a missing record means the fixture
// never wrote one, which is a different and immediately obvious failure. The
// census is over the FILES, so it covers all three suite arms from arm 1.
func TestQANoPassRunningTestDiscardsAReadMetaOk(t *testing.T) {
	t.Parallel()
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files, scanned, passFuncs := 0, 0, 0
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		files++
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, fn := range reapRaceFuncs(string(body)) {
			scanned++
			if !fn.runsAPass() {
				continue
			}
			passFuncs++
			for _, hit := range fn.discardedOks() {
				t.Errorf("%s:%d: %s runs a dispatch pass and discards readMeta's ok — the pass's own "+
					"epilogue sweep may have REAPED that session (a fixture's closed bead, an idle "+
					"agent, a per-bead name: autoreap.go), and this line then dereferences nil, which "+
					"is a SIGSEGV that takes the whole arm's test binary down and names no test "+
					"(ranger-base-rviq5). Read the ok and t.Fatalf that the record is gone — a record "+
					"that is GONE and a record carrying the wrong field are different facts:\n\t%s",
					filepath.Join(".", name), hit.line, fn.name, hit.text)
			}
		}
	}
	// Control: a census whose subject has left the package passes for the
	// wrong reason, and this one's subject is the ordinary shape of a
	// dispatch test. MEASURED 2026-11-04: 572 test files, 5,326 top-level
	// funcs, 319 of them running a pass. The floors are a long way under
	// those, because what they have to catch is a reading that has broken
	// (a renamed suffix, a split that stopped splitting, a needle that
	// matches nothing) and not a week's drift in the population.
	if files < 400 || passFuncs < 150 {
		t.Fatalf("this census read %d test files and %d top-level funcs and found only %d running a "+
			"dispatch pass — it has stopped reading what it is about (MEASURED 2026-11-04: 572 files, "+
			"5,326 funcs, 319 pass-running)", files, scanned, passFuncs)
	}
	// And the needles themselves, against text this test owns, so a regexp
	// edited to green the census reds here instead.
	fixture := reapRaceFuncs("func TestFixture(t *testing.T) {\n" +
		"\td.Run(\"\", \"\", 0)\n" +
		"\tm, _ := b.read" + "Meta(session)\n" +
		"\tif m2, _ = b.read" + "Meta(other); m2 != nil {\n" +
		"\t}\n" +
		"\tm3, ok := b.read" + "Meta(third)\n" +
		"\tcmd.Run()\n" +
		"}\n")
	if len(fixture) != 1 || !fixture[0].runsAPass() || len(fixture[0].discardedOks()) != 2 {
		t.Fatalf("the census's own needles no longer read their fixture: %d funcs, pass=%v, hits=%d",
			len(fixture), len(fixture) == 1 && fixture[0].runsAPass(),
			func() int {
				if len(fixture) == 1 {
					return len(fixture[0].discardedOks())
				}
				return -1
			}())
	}
	clean := reapRaceFuncs("func TestClean(t *testing.T) {\n\tcmd.Run()\n\tm, _ := b.read" + "Meta(s)\n}\n")
	if len(clean) != 1 || clean[0].runsAPass() {
		t.Fatalf("`cmd.Run()` reads as a dispatch pass — the census is scoped to the wrong tests")
	}
}

// ─── the census's reading of a Go test file ──────────────────────────────────

type reapRaceLine struct {
	line int
	text string
}

type reapRaceFunc struct {
	name  string
	lines []reapRaceLine
}

var reapRaceFuncHead = regexp.MustCompile(`^func (?:\([^)]*\) )?(\w+)\(`)

// reapRaceFuncs splits a file into top-level funcs. A brace-counting parse
// would be the same answer at more cost: in gofmt'd source `func` at column
// zero is the only thing that opens one and `}` at column zero is the only
// thing that closes one, and `make fmt-check` is a prerequisite of every
// door this pin is behind. Ending at that brace is what keeps the package
// vars and doc comments BETWEEN two funcs out of the preceding func's body,
// which a run-to-the-next-`func` split would have folded in. A func literal
// inside a t.Run stays part of its enclosing func, which is what this census
// wants — the pass and the read are often in different subtests of one test.
func reapRaceFuncs(src string) []reapRaceFunc {
	var out []reapRaceFunc
	inFunc := false
	for i, line := range strings.Split(src, "\n") {
		if m := reapRaceFuncHead.FindStringSubmatch(line); m != nil {
			out = append(out, reapRaceFunc{name: m[1]})
			inFunc = true
		}
		if !inFunc {
			continue
		}
		f := &out[len(out)-1]
		f.lines = append(f.lines, reapRaceLine{line: i + 1, text: strings.TrimSpace(line)})
		if line == "}" {
			inFunc = false
		}
	}
	return out
}

func (f reapRaceFunc) runsAPass() bool {
	for _, l := range f.lines {
		for _, m := range reapRaceRunCall.FindAllStringSubmatch(l.text, -1) {
			if m[1] != "t" {
				return true
			}
		}
	}
	return false
}

func (f reapRaceFunc) discardedOks() []reapRaceLine {
	var out []reapRaceLine
	for _, l := range f.lines {
		if reapRaceDiscardedOk.MatchString(l.text) {
			out = append(out, l)
		}
	}
	return out
}
