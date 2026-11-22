package treepins

// QA census for ranger-base-dudu4 — EVERY WALL-CHECK MEMBER THAT LAUNCHES A
// BWRAP SEAT REACHES ADR 0096 D3'S NEST GATE, read as a census over the
// member set rather than one member at a time.
//
// THE CLASS, found as an instance by ranger-base-mklu0 (finding 1 of the
// ranger-base-2yt8s verify) and fixed there one file at a time: a live pin in
// internal/posse that execs a bwrap, sits inside ADR 0097 D3's `-run` filter
// by name, and reaches `bwrapNestSkip` through no route at all. Until this
// file, nothing in the tree could say so.
//
// WHAT SUCH A PIN COSTS, all three MEASURED 2026-11-22 on 027430b2 for
// wallunixlive_test.go's three (linux 7.2.15-1-omarchy, bwrap 0.13.1,
// go1.27.3, 16 cores):
//
//   - a red there prints as `note: FAIL <test>` under `wall check: green, N
//     pins measured` at EXIT 0 — ADR 0097 D3's member-FAIL exit 1, which ADR
//     0088 D5's ratchet files as one P1 carrying that line, never fires,
//     because membership is read off the MARKER and an ungated pin emits
//     none;
//   - a run in which it goes DARK is counted by neither D5 arm — not a member
//     abstention, so not exit 3 — which is the `/var/tmp` shape D5 exists
//     for;
//   - where bwrap cannot nest (the walled-seat shape, ADR 0096 D1) it FAILs
//     at exit 1 where every gated member SKIPs, which D3 forbids outright
//     ("IT NEVER FAILS FOR THE WALL, outside the venue"). Readings under a
//     `bwrap` shim that prints `setting up uid map: Permission denied`:
//     FAIL/FAIL/FAIL at exit 1 ungated, SKIP/SKIP/SKIP at exit 0 gated,
//     against SKIP for TestLiveWallClaudeProjectsRefusesPeerPlants and
//     TestLiveBwrapSeatTmpKeepsTheSeatOutOfTheHostsTmp throughout.
//
// WHY A CENSUS AND NOT PROSE. Membership is read off the marker and never off
// the name (ADR 0097 D4), and the marker's only emitter is the gate — so a
// pin's NAME buys it nothing and the one place a missing gate showed was a
// hand audit. Three of the four rows ranger-base-8ck6d folded were outside
// the set, and that bead's own description asserts the opposite ("Both files
// are members"); the verify found it by census and not by reading, which is
// this shop's standing argument for one ("a named read is nobody's census",
// wallmemberaccount_qa_test.go's head comment).
//
// ─── THE RULE, AND THE ONE NUMBER IN IT ────────────────────────────────────
//
// A member is a top-level test in a member file whose name the check's own
// `member_pattern` selects (wmaMemberPattern, read and never retyped). Its
// REACH is itself plus every declaration in the package's test sources that a
// call in a reached body names, to wngReachDepth hops. It LAUNCHES if any
// body in that reach starts a bwrap seat (wmaSeatLaunches, this package's one
// spelling of a launch and the key ranger-base-hhps5 widened), and it is
// GATED if any body in that reach calls bwrapNestSkip or bwrapNestSkipTo.
// Launch implies gated, and nothing else: gating a member that launches
// nothing is cheap and sometimes right (nine members do it today — the
// wall-watch family, whose seat is forked by runWallWatch in PRODUCTION code
// and so appears in no test body at all), so the converse is not a rule here.
//
// THE DEPTH IS 2 AND IT IS MEASURED, not chosen. TWO readings, taken on the
// same box over 767 files and 57 members, before and after ranger-base-mklu0's
// fix for the instance — they agree about the depth and differ only in what
// depth 2 finds, which is the whole of what that fix changed.
//
// ON THE TREE THIS FILE LANDS ON (2026-11-22, 5d11a45f, mklu0's fix in):
//
//	depth 1   6 ungated, and ALL SIX ARE FALSE — bwrapseatstorelive and
//	          storerepogitwalllive reach the gate at two hops, through
//	          bsLiveFixture/srLiveFixture to bgLiveSkip
//	depth 2   0 ungated: the rule is satisfied by the tree's own spellings
//	depth 3   0 ungated, identical — the rule is not perched on its own edge
//	depth 4   0 ungated, but the reach has blown out into the package: the
//	          launch count goes 45 → 54, so the nine gated non-launchers all
//	          read as launchers and "launches a seat" has stopped meaning
//	          anything at that depth
//
// BEFORE THAT FIX (2026-11-22, d5527330), which is the reading that found the
// instance and the one the depth was chosen against:
//
//	depth 1   9 ungated, and 6 of the 9 are FALSE — the same two files
//	depth 2   3 ungated: wallunixlive_test.go's three, which is the instance
//	          ranger-base-mklu0 found by hand, and nothing else
//	depth 3   3 ungated, identical
//	depth 4   3 ungated, with the same blow-out in the launch count
//
// So 2 is the smallest depth the tree's own spellings need, 3 agrees with it
// and 4 is where the reach stops being a reading — on either tree. A gate
// moved to the third hop reds here, and the message says to re-argue the depth
// rather than to deepen it quietly; arm 2 holds both directions over planted
// source, which is what keeps the rule honest now that the tree arm finds
// nothing.
//
// WHAT IT DOES NOT GRADE, both named here so the next reader does not come
// back to them.
//
// ORDER. ranger-base-mklu0 put the gate BEFORE wallunixlive's Landlock-ABI
// floor and said why: the gate logs the marker the moment it lets a pin
// proceed, so a skip AFTER it is a counted abstention (exit 3, loud) and a
// skip BEFORE it is a pin the venue never knew it had (a smaller N, silent).
// That reasoning is right and this census still does not read it, because the
// tree has no one order to read: bgLiveSkip puts `LookPath("git")` after the
// gate, bwraptmp_qa_test.go and sshidentity_qa_test.go put the gate at the
// call site after their own floors, and every pin's floors are its own
// judgement about what its body needs. Grading order is a second rule and
// needs its own argument (ranger-base-dudu4's judgement 2, undecided there
// and undecided here). The same sentence covers PLACEMENT within a member: a
// gate inside one `t.Run` arm and not its siblings reads as gated here.
//
// THE RUNTIME COUNT, which is the OTHER half and is no longer open. This
// census closes the SOURCE half — a gate deleted from a launching member reds
// here — and never the runtime half, which is a pin that reaches the gate in
// source and skips for its own reason before getting there. When this census
// was written `scripts/posse-wall-check` carried no floor on the member count,
// so pins going dark one at a time read as `green, N pins measured` with a
// smaller N and nothing to compare it to, and that was filed as
// ranger-base-c75wa. It has since LANDED (82b4adc2): the check now derives a
// floor for n from its own run and a dark pin is exit 3, so the two halves are
// complementary and both held. Keep them apart anyway — this file reads
// SOURCE and cannot see a runtime skip, that one reads a RUN and cannot see a
// deleted gate, and a reader who collapses them will trust whichever is
// greener.
//
// WHERE IT LIVES. internal/treepins, which puts it inside `make
// treepins-check`'s complement door — this package minus the priced table in
// treewidedoor_qa_test.go's `twdTreepinsExcluded` — so it needs no
// hand-written Makefile entry and cannot go undoored (ADR 0098 D1/D4). It is
// a separate file from wallmemberaccount_qa_test.go, whose subject is a
// member's one LINE on a cause, and reuses that file's member-file set and
// launch key rather than spelling either a second time: two spellings of
// "what a seat launch looks like" would be two answers about one member set
// (ranger-base-8752f's lesson, one census over).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"testing"
)

// wngGateNames are the gate's two spellings: the one every pin calls, and the
// composition a pin may drive with a sink of its own (ADR 0097 D4,
// internal/posse/wallvenue_test.go). Read as CALL NAMES, which is the rule's
// one blind spot and arm 2 says so: a gate handed to a helper as a function
// VALUE is invisible here, and the fix for that is to name the call.
var wngGateNames = map[string]bool{
	"bwrapNestSkip":   true,
	"bwrapNestSkipTo": true,
}

// wngGateSpellings names them for a message, off the map rather than typed a
// second time: a message that spells its own rule is a message that can
// disagree with it.
func wngGateSpellings() []string {
	out := make([]string, 0, len(wngGateNames))
	for n := range wngGateNames {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// wngReachDepth is the measured hop budget — see the head comment, which
// carries all four readings. It is a const so that deepening it is a diff a
// reader meets, and arm 2 holds the rule at both edges of it.
const wngReachDepth = 2

// wngDecl is one declaration in the package the check runs, with the file it
// is in — the file is what makes same-file resolution possible below.
type wngDecl struct {
	fn   *ast.FuncDecl
	path string
}

// wngPkg is the package's test sources as this census reads them: every file,
// and every declaration in them by name.
//
// EVERY FILE AND NOT ONLY THE MEMBER FILES, which is load-bearing: three of
// the seven gate helpers in today's tree are called from a file other than the
// one they are written in — wrcNeedBwrap from four, wwLiveFixture from three,
// and bgLiveSkip from bwrapseatstorelive and storerepogitwalllive, which is
// also where the second hop comes from. A census that resolved names inside
// one file would read every member behind those three as ungated. (A member
// file is where a MEMBER is declared; the gate's own declaration is in
// wallvenue_test.go, which declares none and so is not one. This census never
// needs that body — only the call — but it is the same glob either way.)
type wngPkg struct {
	files  []wmaFile
	byName map[string][]wngDecl
}

// wngParsePkg reads the package directory the check runs. wmaPkgGlob, the one
// spelling of that directory, shared with the census next door.
func wngParsePkg(t *testing.T) wngPkg {
	t.Helper()
	paths, err := filepath.Glob(wmaPkgGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("%s matched no files — this census has nothing to read, so every arm below would pass for the wrong reason", wmaPkgGlob)
	}
	var srcs []wmaFile
	for _, p := range paths {
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", p, perr)
		}
		srcs = append(srcs, wmaFile{path: p, fset: fset, ast: f})
	}
	return wngIndex(srcs)
}

// wngIndex builds the declaration index over already-parsed files, so the
// hermetic arm can drive the same rule over planted source.
func wngIndex(files []wmaFile) wngPkg {
	p := wngPkg{files: files, byName: map[string][]wngDecl{}}
	for _, f := range files {
		for _, fn := range wmaDecls(f.ast) {
			p.byName[fn.Name.Name] = append(p.byName[fn.Name.Name], wngDecl{fn: fn, path: f.path})
		}
	}
	return p
}

// wngCallees are the names a body calls: a plain `f(x)` by its identifier and
// an `x.M(y)` by its method name. `exec.Command` and `t.Fatalf` land here too
// and resolve to nothing, which costs a map lookup and keeps the rule from
// needing a type checker.
func wngCallees(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	if fn.Body == nil {
		return out
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := c.Fun.(type) {
		case *ast.Ident:
			out[f.Name] = true
		case *ast.SelectorExpr:
			out[f.Sel.Name] = true
		}
		return true
	})
	return out
}

// resolve answers which declarations a call by this name can reach, preferring
// the ones in the calling file.
//
// SAME FILE FIRST, and only methods can need it: Go forbids two package-level
// functions of one name, so an `Ident` callee resolves exactly, and the
// preference only decides between methods of the same name on different
// receivers. Crude on purpose and for the reason internal/posse's
// wsaChanIntNames states — the receiver's type at `f.in(t)` is not resolvable
// from an AST alone, and the alternative is a rule that reads no method call
// at all, which is where every launch through a fixture lives.
func (p wngPkg) resolve(name, from string) []*ast.FuncDecl {
	var same, other []*ast.FuncDecl
	for _, d := range p.byName[name] {
		if d.path == from {
			same = append(same, d.fn)
		} else {
			other = append(other, d.fn)
		}
	}
	if len(same) > 0 {
		return same
	}
	return other
}

// reach is the member's own body plus everything a call in a reached body
// names, to depth hops. Breadth-first, each declaration taken once.
func (p wngPkg) reach(fn *ast.FuncDecl, from string, depth int) []*ast.FuncDecl {
	seen := map[*ast.FuncDecl]bool{fn: true}
	out := []*ast.FuncDecl{fn}
	frontier := []*ast.FuncDecl{fn}
	for d := 0; d < depth; d++ {
		var next []*ast.FuncDecl
		for _, cur := range frontier {
			called := wngCallees(cur)
			names := make([]string, 0, len(called))
			for n := range called {
				names = append(names, n)
			}
			sort.Strings(names) // a stable reach, so a message reads the same twice
			for _, n := range names {
				for _, r := range p.resolve(n, from) {
					if seen[r] {
						continue
					}
					seen[r] = true
					next = append(next, r)
					out = append(out, r)
				}
			}
		}
		frontier = next
	}
	return out
}

// wngCounts is the census this pin owes its reader, whatever it finds.
type wngCounts struct {
	files, members, launching, gated int
}

// wngUngated is THE RULE over a parsed package: every member that launches a
// bwrap seat and reaches the gate from nowhere, as `file:line test`. A
// function, so arm 2 drives it over planted source with no bwrap, no venue
// and no member set.
func wngUngated(p wngPkg, member *regexp.Regexp, depth int) (ungated []string, c wngCounts) {
	for _, f := range p.files {
		isMember := false
		for _, fn := range wmaDecls(f.ast) {
			if fn.Recv != nil || !member.MatchString(fn.Name.Name) {
				continue
			}
			isMember = true
			c.members++
			reach := p.reach(fn, f.path, depth)
			launches, gated := false, false
			for _, r := range reach {
				if wmaLaunchesSeat(r) {
					launches = true
				}
				for n := range wngCallees(r) {
					if wngGateNames[n] {
						gated = true
					}
				}
			}
			if launches {
				c.launching++
			}
			if gated {
				c.gated++
			}
			if launches && !gated {
				ungated = append(ungated, f.path+":"+strconv.Itoa(f.fset.Position(fn.Pos()).Line)+" "+fn.Name.Name)
			}
		}
		if isMember {
			c.files++
		}
	}
	sort.Strings(ungated)
	return ungated, c
}

// ARM 1 — THE TREE. Every member of ADR 0097 D3's set that launches a bwrap
// seat reaches ADR 0096 D3's gate.
func TestQAEveryWallMemberThatLaunchesASeatReachesTheNestGate(t *testing.T) {
	t.Parallel()
	p := wngParsePkg(t)
	re := wmaMemberPattern(t)
	ungated, c := wngUngated(p, re, wngReachDepth)
	for _, u := range ungated {
		t.Errorf("%s launches a bwrap seat and reaches none of %v within %d hop(s), so it is a member of ADR 0097 D3's `-run` filter that emits no membership MARKER: the venue counts it as nothing, a red in it prints as `note: FAIL` under `wall check: green, N pins measured` at EXIT 0 and never reaches ADR 0088 D5's ratchet, a run where it goes dark is counted by neither D5 arm, and on a caged seat it FAILs where every gated member SKIPs — which ADR 0096 D3 forbids outright (\"IT NEVER FAILS FOR THE WALL, outside the venue\"). Call the gate after this pin's own GOOS and `LookPath(\"bwrap\")` floors and before any other, as bgLiveSkip does.",
			u, wngGateSpellings(), wngReachDepth)
	}
	// THE CONTROLS, both directions. A census that found no member files, no
	// members in them, or no launches would pass having read nothing — the
	// failure mode an absence pin has (this package's absencerules_qa_test.go).
	if len(p.files) < 500 {
		t.Errorf("this census parsed %d file(s) from %s, want at least 500 (767 on 2026-11-22) — that glob has stopped naming the package the check runs and every reading below is over the wrong tree", len(p.files), wmaPkgGlob)
	}
	if c.files < 20 {
		t.Errorf("this census found %d member file(s), want at least 20 (24 on 2026-11-22) — the member pattern read from %s has stopped matching this package and the rule graded almost nothing", c.files, wmaCheck)
	}
	if c.members < 45 {
		t.Errorf("this census found %d member(s), want at least 45 (57 on 2026-11-22) — the pattern read from %s selects a set this small only if it has stopped meaning what the check types", c.members, wmaCheck)
	}
	if c.launching < 30 {
		t.Errorf("this census found %d member(s) that launch a bwrap seat, want at least 30 (45 on 2026-11-22) — a seat is started some way wmaSeatLaunches cannot see, so this rule has members it is not grading and an ungated one would read green", c.launching)
	}
	if c.gated < 35 {
		t.Errorf("this census found %d gated member(s), want at least 35 (54 on 2026-11-22) — either the gate has a spelling %v does not name, or it has been deleted from most of the set", c.gated, wngGateSpellings())
	}
	t.Logf("%d file(s) parsed, %d member file(s), %d member(s): %d launch a bwrap seat, %d reach the gate within %d hop(s), %d launch and do not.",
		len(p.files), c.files, c.members, c.launching, c.gated, wngReachDepth, len(ungated))
}

// ARM 2 — THE RULE SAYS NO. The reach, the launch and the gate driven over
// planted source, through the SAME functions arm 1 reads the tree with, each
// with its negative case. A rule that quietly stopped meaning what this file
// says it means reds here with no bwrap, no venue and no member set.
//
// THE TWO DEPTH ROWS ARE THE POINT. The tree needs two hops today and arm 1's
// floors cannot say so — a gate found at one hop is found at two, so no count
// over this tree distinguishes the depths. These rows do, hermetically, in
// both directions: a gate at the second hop is reached and a gate at the
// third is not.
func TestQATheWallNestGateReachRuleMatchesWhatItClaims(t *testing.T) {
	t.Parallel()
	member := regexp.MustCompile(`^Test(QA)?Live(Bwrap|Wall)`)
	for _, c := range []struct {
		name string
		// src is one planted file; extra is a second, so the cross-file
		// resolution arm 1 depends on is driven here too.
		src, extra string
		want       []string // the ungated members, as `path:line name`
		launching  int
	}{
		{
			name: "a member that launches and gates in its own body",
			src: `func TestLiveWallPlanted(t *testing.T) {
	bwrapNestSkip(t)
	cmd := exec.Command("bwrap", "--ro-bind", "/", "/", "/bin/true")
	_ = cmd.Run()
}`,
			launching: 1,
		},
		{
			name: "a member that launches and gates nowhere — the class",
			src: `func TestLiveWallPlanted(t *testing.T) {
	cmd := exec.Command("bwrap", "--ro-bind", "/", "/", "/bin/true")
	_ = cmd.Run()
}`,
			want:      []string{"planted_test.go:2 TestLiveWallPlanted"},
			launching: 1,
		},
		{
			name: "the gate one hop away, in a helper in the same file",
			src: `func plantedSkip(t *testing.T) {
	bwrapNestSkip(t)
}
func TestLiveWallPlanted(t *testing.T) {
	plantedSkip(t)
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`,
			launching: 1,
		},
		{
			name: "the gate one hop away, in a helper in ANOTHER file — bgLiveSkip's shape",
			src: `func TestLiveWallPlanted(t *testing.T) {
	plantedSkip(t)
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`,
			extra: `func plantedSkip(t *testing.T) {
	bwrapNestSkip(t)
}`,
			launching: 1,
		},
		{
			name: "the gate TWO hops away, through a fixture — bsLiveFixture's shape, and the reason the depth is 2",
			src: `func TestLiveWallPlanted(t *testing.T) {
	f := plantedFixture(t)
	cmd := exec.Command("bwrap", f.arg)
	_ = cmd.Run()
}`,
			extra: `func plantedFixture(t *testing.T) plantedFx {
	plantedSkip(t)
	return plantedFx{}
}
func plantedSkip(t *testing.T) {
	bwrapNestSkip(t)
}`,
			launching: 1,
		},
		{
			name: "the gate THREE hops away is NOT reached, and that is the depth asking to be re-argued",
			src: `func TestLiveWallPlanted(t *testing.T) {
	f := plantedFixture(t)
	cmd := exec.Command("bwrap", f.arg)
	_ = cmd.Run()
}`,
			extra: `func plantedFixture(t *testing.T) plantedFx {
	plantedFloors(t)
	return plantedFx{}
}
func plantedFloors(t *testing.T) {
	plantedSkip(t)
}
func plantedSkip(t *testing.T) {
	bwrapNestSkip(t)
}`,
			want:      []string{"planted_test.go:2 TestLiveWallPlanted"},
			launching: 1,
		},
		{
			name: "the LAUNCH one hop away, in a fixture method, with the gate in the member",
			src: `func TestLiveWallPlanted(t *testing.T) {
	bwrapNestSkip(t)
	f := plantedFx{}
	f.in(t)
}`,
			extra: `func (f plantedFx) in(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`,
			launching: 1,
		},
		{
			name: "the LAUNCH one hop away and NO gate anywhere",
			src: `func TestLiveWallPlanted(t *testing.T) {
	f := plantedFx{}
	f.in(t)
}`,
			extra: `func (f plantedFx) in(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`,
			want:      []string{"planted_test.go:2 TestLiveWallPlanted"},
			launching: 1,
		},
		{
			name: "a member that launches nothing needs no gate — the three deliberate non-members' shape",
			src: `func TestLiveWallPlanted(t *testing.T) {
	cmd := exec.Command("sh", "-c", ` + "`" + `"$@"; :` + "`" + `, "sh", "x")
	_ = cmd.Run()
}`,
		},
		{
			name: "a member launched through a shell IS a launch — ranger-base-hhps5's route",
			src: `func TestLiveWallPlanted(t *testing.T) {
	cmd := exec.Command("sh", "-c", ` + "`" + `bwrap "$@"; :` + "`" + `, "sh", "x")
	_ = cmd.Run()
}`,
			want:      []string{"planted_test.go:2 TestLiveWallPlanted"},
			launching: 1,
		},
		{
			name: "bwrapNestSkipTo is the gate too",
			src: `func TestLiveWallPlanted(t *testing.T) {
	bwrapNestSkipTo(t, t.Name(), "", false)
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`,
			launching: 1,
		},
		{
			name: "a gate handed through a function VALUE is invisible — the rule's one blind spot, named",
			src: `func TestLiveWallPlanted(t *testing.T) {
	plantedFloors(t, bwrapNestSkip)
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`,
			extra: `func plantedFloors(t *testing.T, nest func(*testing.T)) {
	nest(t)
}`,
			want:      []string{"planted_test.go:2 TestLiveWallPlanted"},
			launching: 1,
		},
		{
			name: "a test outside the member pattern is not this census's subject, gate or no gate",
			src: `func TestPlantedSomethingElse(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`,
		},
		{
			name: "a gate in an unrelated SIBLING member does not gate this one",
			src: `func TestLiveWallPlantedGated(t *testing.T) {
	bwrapNestSkip(t)
}
func TestLiveWallPlanted(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`,
			want:      []string{"planted_test.go:5 TestLiveWallPlanted"},
			launching: 1,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			files := []wmaFile{wngFixture(t, "planted_test.go", c.src)}
			if c.extra != "" {
				files = append(files, wngFixture(t, "extra_test.go", c.extra))
			}
			got, counts := wngUngated(wngIndex(files), member, wngReachDepth)
			if len(got) != len(c.want) {
				t.Fatalf("the rule found %v ungated, want %v — it no longer means what this file says it means:\n%s\n---\n%s", got, c.want, c.src, c.extra)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("the rule found %q, want %q", got[i], c.want[i])
				}
			}
			if counts.launching != c.launching {
				t.Errorf("the rule counted %d launching member(s), want %d — the launch key has stopped selecting what this row plants", counts.launching, c.launching)
			}
		})
	}

	// And the depths, over ONE fixture: the gate at hop 2 is reached at depth
	// 2 and not at depth 1, which is the whole of the reading the head
	// comment records and the one thing arm 1's floors cannot hold.
	twoHops := []wmaFile{
		wngFixture(t, "planted_test.go", `func TestLiveWallPlanted(t *testing.T) {
	plantedFixture(t)
	cmd := exec.Command("bwrap", "x")
	_ = cmd.Run()
}`),
		wngFixture(t, "extra_test.go", `func plantedFixture(t *testing.T) {
	plantedSkip(t)
}
func plantedSkip(t *testing.T) {
	bwrapNestSkip(t)
}`),
	}
	for _, c := range []struct {
		depth int
		want  int
	}{{1, 1}, {2, 0}, {3, 0}} {
		got, _ := wngUngated(wngIndex(twoHops), member, c.depth)
		if len(got) != c.want {
			t.Errorf("at depth %d the rule found %d ungated member(s), want %d — the hop budget no longer reaches what this file's head comment says it was measured to reach", c.depth, len(got), c.want)
		}
	}
}

// wngFixture parses one planted snippet through the same parser the tree arm
// uses, so a fixture cannot pass against a second, friendlier copy of a rule.
// `package posse` and the gate's own name, because what is planted is a file
// of that package.
func wngFixture(t *testing.T, path, src string) wmaFile {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, "package posse\n"+src+"\n", 0)
	if err != nil {
		t.Fatalf("parse %s: %v\n%s", path, err, src)
	}
	return wmaFile{path: path, fset: fset, ast: f}
}
