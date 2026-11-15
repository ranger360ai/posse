//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-np54s's class, held open by the key that FINDS it
// (ranger-base-hegka, from the ranger-base-eclrz verify).
//
// THE CLASS. A test that launches a writer of d.Out on a goroutine — Watch,
// Run, LaunchBead, any of the clock loops — and then reads that stream before
// joining it is reading a *strings.Builder while another goroutine appends to
// it. A torn read does not fail the test: Builder.String panics with
//
//	panic: runtime error: unsafe.String: ptr is nil and len is not zero
//
// and a panic in a parallel package takes the WHOLE arm's test binary with
// it, so every other test in that run reports nothing and names nothing. The
// fix at the call site is one word — dispatcherOutLive, which takes outMu
// (herdr_test.go) — and the whole difficulty is finding the sites.
//
// WHY THIS PIN EXISTS RATHER THAN THE CENSUS THAT CLOSED np54s. That census
// was keyed "a `for` loop POLLING dispatcherOut", and it was right about what
// it asked: exactly one loop in this package polls it. Two more sites were
// outside the question. Both poll the FILESYSTEM —
//
//	go func() { p, _ := d.Watch(ctx, …); done <- p }()
//	deadline := time.After(60 * time.Second)
//	for len(archives(t, b.App)) == 0 {
//	    select {
//	    case <-deadline:
//	        t.Fatalf("…:\n%s", dispatcherOut(d))   // ← the read, in the arm
//	    case <-time.After(10 * time.Millisecond):
//	    }
//	}
//	cancel()
//	select { case <-done: … }                      // ← the join, after it
//
// — and read the stream only in their deadline arm. So the key is not
// polling. It is A READ TAKEN BEFORE THE JOIN, whatever the loop around it
// polls, and that is what this pin asks. MEASURED 2026-11-15 on this box
// (Linux 7.2.15-1-omarchy, go1.27.3, 16 cores): with the deadline aimed into
// the loop's first milliseconds, so the arm actually executes,
// `go test -race -tags posse_arm3 -count=20` over
// TestWatchBackupClockWritesAnArchive reported WARNING: DATA RACE; over 120
// iterations across six deadlines it reports none now.
//
// THE READ IS ONLY HALF OF IT. dispatcherOutLive's lock is worth nothing
// against a write that takes no lock, and Watch's preamble had three: the
// Out/rawOut swap in teeWatchLog and the header lines after it, on the theory
// that the preamble runs before any other writer exists. It does not when
// Watch itself is the goroutine, which is the shape all 26 of this package's
// Watch launches have. Those are under outMu now (watch.go,
// dispatch.go's quietOutWriter), and the allowlist that used to excuse them
// in TestQAWatchStreamWritesGoThroughTheDispatcher is five entries shorter.
//
// Its door is `make pkgcensus-check`, under $(QA_PKGCENSUS_PINS) — a census
// over internal/posse's own sources, read as TEXT, so it sees every arm's
// test files from arm 1. Type it when you launch a dispatcher writer on a
// goroutine in a test.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The writers: a goroutine running any of these appends to d.Out until it
// returns. Three names and one shape — every clock loop in this package is
// a method whose name ends in Loop (backupLoop, guardLoop, watchdogLoop,
// pulseLoop), which is why the shape is cheaper to state than the list.
var watchOutWriterCall = regexp.MustCompile(`\bd\.(Watch|Run|LaunchBead)\(|\bd\.\w*Loop\(`)

// The join: the receive or Wait that orders everything after it against the
// goroutine's writes. `<-done`, `<-backupDone`, `<-guardDone` and `wg.Wait()`
// are every spelling in the package today; the names are not enumerated
// because a channel called anything else is still a join and a bead that
// invents one must not silently widen the window.
var watchOutJoin = regexp.MustCompile(`<-\s*\w*[Dd]one\b|\.Wait\(\)`)

// The read: dispatcherOut's own name, which does not match
// dispatcherOutLive's. A read through the live helper is the fix, so it is
// not a hit.
const watchOutUnlockedRead = "dispatcherOut("

func TestQANoUnlockedWatchStreamReadBeforeItsJoin(t *testing.T) {
	t.Parallel()
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files, funcs, launches := 0, 0, 0
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		files++
		for _, fn := range watchOutFuncs(string(src)) {
			funcs++
			for _, w := range fn.windows() {
				launches++
				for _, hit := range w.unlockedReads() {
					t.Errorf("%s:%d: %s reads the watch stream through dispatcherOut while the goroutine launched at line %d is still writing it — the join is at %s. "+
						"A *strings.Builder read racing an append observes a torn slice header and Builder.String PANICS, which ends the whole arm's test binary and names no test "+
						"(ranger-base-np54s, ranger-base-hegka). Read it through dispatcherOutLive, which takes outMu (herdr_test.go), or move the read after the join:\n\t%s",
						filepath.Join(".", name), hit.line, fn.name, w.launchedAt, w.joinWhere, hit.text)
				}
			}
		}
	}
	// Controls. The floors say the sweep still reads the package it thinks
	// it does — a renamed suffix, a split that stopped splitting, a needle
	// that matches nothing — and are a long way under the population rather
	// than tracking it. MEASURED 2026-11-15: 716 test files, 6,820 top-level
	// funcs, and 36 goroutine launches of a d.Out writer among them.
	//
	// The launches floor is the one that matters and 20 is not timid: the
	// census's own control run, with the four known sites flipped back to
	// dispatcherOut, reported all four (2026-11-15) — so the number this
	// floor guards against is a launch needle that has stopped matching the
	// package's one-line `go func() { … }()`, which would take it to zero.
	if files < 400 || funcs < 3000 || launches < 20 {
		t.Fatalf("this census read %d test files and %d top-level funcs and found only %d goroutine launches of a watch-stream writer — "+
			"it has stopped reading what it is about (MEASURED 2026-11-15: 716 files, 6,820 funcs, 36 launches)", files, funcs, launches)
	}
	// And the needles against text this test owns, so a regexp edited to
	// green the census reds here instead. The fixture is the escaped shape:
	// a one-line `go func()`, a read in a deadline arm, the join after it.
	fixture := watchOutFuncs("func TestFixture(t *testing.T) {\n" +
		"\tgo func() { p, _ := d.Watch(ctx, \"\"); done <- p }()\n" +
		"\tfor len(archives(t, b.App)) == 0 {\n" +
		"\t\tselect {\n" +
		"\t\tcase <-deadline:\n" +
		"\t\t\tt.Fatalf(\"x:\\n%s\", dispatcher" + "Out(d))\n" +
		"\t\t}\n" +
		"\t}\n" +
		"\tcancel()\n" +
		"\tselect { case <-done: }\n" +
		"\tif out := dispatcher" + "Out(d); out == \"\" {\n" +
		"\t}\n" +
		"}\n")
	if len(fixture) != 1 {
		t.Fatalf("the func split no longer reads its own fixture: %d funcs", len(fixture))
	}
	ws := fixture[0].windows()
	if len(ws) != 1 {
		t.Fatalf("the launch needle no longer reads a one-line `go func() { … d.Watch … }()`: %d windows", len(ws))
	}
	if got := ws[0].unlockedReads(); len(got) != 1 {
		t.Fatalf("the window is not the launch-to-join one: %d hits, want the read in the deadline arm and NOT the one after the join (%v)", len(got), got)
	}
	// The negative cases, each the reason a hit is not one: the live helper
	// is the fix, and a goroutine that writes nothing on this stream is not
	// this pin's business.
	live := watchOutFuncs("func TestLive(t *testing.T) {\n" +
		"\tgo func() { d.Watch(ctx, \"\"); close(done) }()\n" +
		"\tfor !strings.Contains(dispatcher" + "OutLive(d), \"x\") {\n\t}\n" +
		"\t<-done\n}\n")
	if len(live) != 1 || len(live[0].windows()) != 1 || len(live[0].windows()[0].unlockedReads()) != 0 {
		t.Fatalf("a read through dispatcherOutLive is counted as a hit — the fix would not green this census")
	}
	other := watchOutFuncs("func TestOther(t *testing.T) {\n" +
		"\tgo func() { cmd.Run(); close(done) }()\n" +
		"\tif out := dispatcher" + "Out(d); out == \"\" {\n\t}\n" +
		"\t<-done\n}\n")
	if len(other) != 1 || len(other[0].windows()) != 0 {
		t.Fatalf("`cmd.Run()` on a goroutine reads as a watch-stream writer — the census is scoped to the wrong launches")
	}
}

// ─── the census's reading of a Go test file ──────────────────────────────────

type watchOutLine struct {
	line int
	text string
}

type watchOutFunc struct {
	name  string
	lines []watchOutLine // raw, in order, with 1-based file line numbers
}

// watchOutWindow is one goroutine launch and the lines between it and the
// join: exactly the region in which a read of d.Out is unordered against
// that goroutine's writes.
type watchOutWindow struct {
	launchedAt int
	joinWhere  string
	lines      []watchOutLine
}

var watchOutFuncHead = regexp.MustCompile(`^func (?:\([^)]*\) )?(\w+)\(`)

// watchOutFuncs splits a file into top-level funcs, keeping each line RAW
// (indentation included — the block scan below reads it) with its file line
// number. Same reading as reapRaceFuncs' (passreaprace_qa_test.go) and for
// the same reason: in gofmt'd source `func` at column zero is the only thing
// that opens a top-level func and `}` at column zero the only thing that
// closes one, and `make fmt-check` is a prerequisite of every door this pin
// is behind. A func literal inside a t.Run stays part of its enclosing func,
// which is what this census wants — the launch and the read are often in
// different subtests of one test.
func watchOutFuncs(src string) []watchOutFunc {
	var out []watchOutFunc
	inFunc := false
	for i, line := range strings.Split(src, "\n") {
		if m := watchOutFuncHead.FindStringSubmatch(line); m != nil {
			out = append(out, watchOutFunc{name: m[1]})
			inFunc = true
		}
		if !inFunc {
			continue
		}
		f := &out[len(out)-1]
		f.lines = append(f.lines, watchOutLine{line: i + 1, text: line})
		if line == "}" {
			inFunc = false
		}
	}
	return out
}

// windows is every goroutine launch in this func that writes d.Out, paired
// with the lines from the launch to its join.
//
// A `go` statement is read whole, because the writer call is on the launch
// line in the one-line form and inside the body in the block form, and both
// spellings are in this package. The block's end is `}(` at the launch's own
// indent, which is what gofmt writes and nothing else is.
//
// NO JOIN FOUND IS THE WHOLE REST OF THE FUNC, deliberately: a goroutine
// nothing waits for is writing until the test returns, so that is the honest
// window and not a reason to report nothing.
func (f watchOutFunc) windows() []watchOutWindow {
	var out []watchOutWindow
	for i := 0; i < len(f.lines); i++ {
		trim := strings.TrimSpace(f.lines[i].text)
		if !strings.HasPrefix(trim, "go ") {
			continue
		}
		indent := f.lines[i].text[:len(f.lines[i].text)-len(strings.TrimLeft(f.lines[i].text, " \t"))]
		stmt := []watchOutLine{f.lines[i]}
		end := i
		if !strings.Contains(trim, "}(") {
			for j := i + 1; j < len(f.lines); j++ {
				stmt = append(stmt, f.lines[j])
				end = j
				if t := f.lines[j].text; strings.HasPrefix(t, indent+"}(") {
					break
				}
			}
		}
		writes := false
		for _, l := range stmt {
			if watchOutWriterCall.MatchString(l.text) {
				writes = true
				break
			}
		}
		if !writes {
			continue
		}
		w := watchOutWindow{launchedAt: f.lines[i].line, joinWhere: "nothing in this func joins it"}
		for j := end + 1; j < len(f.lines); j++ {
			if watchOutJoin.MatchString(f.lines[j].text) {
				w.joinWhere = "line " + strings.TrimSpace(strings.Split(f.lines[j].text, "//")[0])
				break
			}
			w.lines = append(w.lines, f.lines[j])
		}
		out = append(out, w)
		i = end
	}
	return out
}

func (w watchOutWindow) unlockedReads() []watchOutLine {
	var out []watchOutLine
	for _, l := range w.lines {
		if strings.Contains(l.text, watchOutUnlockedRead) {
			out = append(out, watchOutLine{line: l.line, text: strings.TrimSpace(l.text)})
		}
	}
	return out
}
