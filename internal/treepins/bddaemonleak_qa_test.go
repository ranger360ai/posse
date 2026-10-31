package treepins

// QA pin for ranger-base-42mv: our own suite must not leak bd daemons.
//
// bd 0.49.1 auto-starts a per-database daemon on first use against a database
// and NOTHING ever stops it. A bd call in a throwaway directory therefore
// leaves a process behind that outlives its directory — holding a sqlite
// handle to a database nobody can reach, and (measured 2026-08-25 and again
// 2026-08-28) holding the directory open well enough to defeat t.TempDir's own
// RemoveAll, so it leaks a process AND a directory. Ten accumulated on the
// live box in twelve days that way; two of them were filed in a single evening
// by TestLiveWorktreeSharesOneGraph, which is what turned this from somebody
// else's mess into ours.
//
// The rule this pins is per FILE, because that is the granularity at which
// the two honest answers are legible:
//
//   - `--no-daemon` on the calls — prevention, nothing is ever started. The
//     right answer when the daemon is not part of the claim.
//   - a cleanup that stops what was started — the right answer when a RUNNING
//     daemon IS the claim (liveCageBeadStore in internal/posse: it imports a
//     newer JSONL before answering, and stopping it first turns the same read
//     into a staleness refusal). `.beads/daemon.pid` beside the fixture's own
//     database is that cleanup's handle, and never `bd daemon stop-all`, which
//     would take the canonical queue's daemon with it.
//
// A file that shells the real bd and does neither is the leak, and is what
// worktreelive_test.go was until 2026-08-28.
//
// WHAT COUNTS AS A CALL SITE IS PARSED, NOT MATCHED (ranger-base-9ojr5). This
// census read file BYTES for `exec.Command("bd"` until 2026-10-31, and a
// matcher over source that cannot tell code from prose gets both halves of
// its job wrong. The positive witness below — which exists so an absence
// assertion cannot pass over a scan that measured nothing — was inflated by
// every comment that named the call: 2 of the 11 sites it reported on
// 2026-10-31 were sentences, in two files that shell no bd at all. And a file
// that shells no bd was REPORTED AS LEAKY: internal/posse/bdgatesfreepath_test.go
// named the call in a comment explaining what Go's bare-name exec resolution
// does, and this pin failed the whole treepins package over it with a remedy
// (pass `--no-daemon`, or stop the daemon via `.beads/daemon.pid`) that had
// nothing to apply to. What cleared it was rewording a sentence. The
// workaround was an exclusion for this guard's OWN source, whose three
// matches were a doc comment, the matcher literal and the control's planted
// fixture text (ranger-base-athy) — a one-file special case for a class that
// was never one file. Same class as the pattern-kill census reading quoted and
// heredoc TEXT as kills that ran (ranger-base-zbg8o).
//
// So the scan parses each test file and asks the AST for a CALL: an
// `exec.Command("bd", …)` or `exec.CommandContext(ctx, "bd", …)` whose binary
// is spelled as a string literal. A comment is not a call, a Go string
// holding the same bytes is not a call, and the exclusion is gone — this
// file's own prose and fixtures now scan to nothing because of what they ARE,
// which is what the zero-witness arm at the foot of the file now proves.
//
// AND SO IS WHAT CLEARS A FILE (ranger-base-q82lc). The census stopped
// reading text on 2026-10-31; the half that decides whether a carrying file
// is EXCUSED kept reading it — a `strings.Contains` over the whole file body
// for the two tokens. That is the same defect pointed the other way, and the
// dangerous way round: a file that shells the real bd with no flag and no
// cleanup was cleared by a SENTENCE anywhere in it that happened to name
// `--no-daemon`, or by any text holding `.beads/daemon.pid`. A false GREEN
// over a real daemon leak, where the census defect was only a false red over
// a clean file — and the remedy the message prints ("pass `--no-daemon`")
// would already have appeared to be satisfied. So the clearing half is handed
// the file's parsed string LITERALS as well, the shape bddbflag_qa_test.go's
// Go arm uses, and the control below plants the leaky shape with each token
// present only as prose and asserts it is still flagged.
//
// It is a no-op on today's tree, which is why it is its own bead and not part
// of 9ojr5: MEASURED 2026-10-31 at 9ojr5's commit, 9 real-bd call sites in 7
// files, and every one of the 7 carries its token as a real string literal in
// code. A change to which files the rule JUDGES clean earns its own control
// arms rather than riding in on a matcher fix.
//
// RESIDUAL, said out loud, on both halves. A call whose binary name is not a
// literal — `exec.Command(bdPath, …)`, `exec.Command("bd"+suffix, …)` — is
// not seen, and never was. What bounds the class is that every real-bd call
// site in the corpus spells the name, and the positive witness fires if the
// census ever stops seeing them.
//
// And a token that is a literal but not a GUARD still clears its file: a Go
// string holding `.beads/daemon.pid` excuses the file whether or not anything
// SIGTERMs that pid, and a `"--no-daemon"` in an unrelated slice excuses it
// whether or not it reaches a bd. That is the floor of per-file granularity
// and not a thing this parse can lift — the cleanup's own handle IS a Go
// string literal, named far from the call it answers for, so no rule that can
// see the cleanup can refuse a string that merely looks like it. Per-file is
// the granularity ranger-base-42mv chose, because it is the one at which the
// two honest answers above are legible. What the parse does buy is that PROSE
// is no longer an alibi, which is the half that could be bought.
//
// THE DOOR IS `make corpus-check` (ranger-base-0ckgm). This pin walks the
// repo root and reads every _test.go in the tree, so it is squarely in
// ranger-base-rulbl's class — a QA test whose subject is the TREE, living
// inside a package nobody runs whole — and for its first two months it had
// no door at all, for the reason given at scanRealBdCallSites below. Its
// subject is this repo's own test corpus, which is corpus-check's subject,
// so it is a row in $(QA_CORPUS_PINS) rather than a door of its own.
//
// The five control arms below are NOT in that filter, and must not be: a
// door variable may only name a test the register derives as tree-wide
// (TestQAEveryTreeWidePinHasADoor is two-way), and the arms read no tree —
// they drive the same rule over planted fixtures, which is what makes them
// arms. They run in arm 1's `./...` like every other test here; what the
// door buys is the PIN, which is the part an unrelated bead can red.
//
// THE FIFTH ARM AND ONE PLANTED SOURCE ARE ranger-base-36nyh, and what they
// buy is two branches of this parse that nothing reached. Both arrived with
// the fix that made the census parse, which is the shape to watch for: a
// branch the AST made FREE is also free to be wrong. MEASURED 2026-10-31
// while ranger-base-9ojr5 was being verified under ranger-base-nqy1h, and
// again here before each was closed, every mutant md5-checked changed before
// its run and the golden restored after. "Every arm then in the file" below is the pin and the four
// controls that preceded this bead.
//
//   - `case "CommandContext": arg = 1` in bdRealBdCallName. Mutated to
//     `arg = 0` — which reads the CONTEXT as the binary name, so it can
//     never match a thing — every arm then in the file PASSED, "scanned 9
//     real-bd call sites in 7 files". Nothing reached the case: 0
//     `exec.CommandContext` calls naming bd anywhere in the corpus, and both
//     leaky controls plant `exec.Command` only. The remedy is one more
//     planted source in TestQABdDaemonGuardReadsCallsNotProse, which now
//     reds that mutant naming the spelling it could not see.
//   - `return err` on the parse, in bdScanVisitor. Mutated to `return nil`,
//     every arm then in the file PASSED, same 9 sites in 7 files. It
//     survived because no file in this tree fails to parse, so the invariant
//     stated at bdScanVisitor — a file that does not parse must not look
//     like a file with no calls in it — was an absence asserted with no
//     positive control. The live pin DOES red correctly on a tree that has
//     one: with `etc/zzz_unparseable_test.go` planted (etc/ because nothing
//     compiles a stray _test.go there, so the probe reds the pin and not the
//     build) it fails naming the parse error, while under the swallowing
//     mutant that same tree reads 9 sites in 7 files and every arm passes —
//     a file the census could not read indistinguishable from a file with no
//     bd in it, which is the pre-fix byte census's shape and the one this
//     bead's family removed everywhere else.
//     TestQABdDaemonGuardCannotSwallowAnUnreadableFile is the control, in a
//     planted root like every other arm, and it is also the first the walk's
//     OWN error return has ever had.
//
// This is the prevention half of the bead. The detection half — naming a
// daemon whose working directory is gone or throwaway, on the live box — is
// scripts/verify-bd-pin.sh's cwd layer, pinned in bdpin_qa_test.go.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// bdCallSite is one parsed call to the operator's real bd in the tree, as
// opposed to the fake ones the hermetic tests write onto a stub PATH, which
// start no daemon and are not this rule's business.
type bdCallSite struct {
	file string
	line int
}

// guardFile is this file. Nothing excludes it from the scan any more; the
// name is kept because the zero-witness arm reads this source back as the one
// test file in a tree with no bd caller left in it.
const guardFile = "bddaemonleak_qa_test.go"

// bdRealBdCallName reports whether a call expression runs the real bd, and is
// deliberately spelled without the call's own text: `exec`, `Command` and
// `bd` are asked for one at a time, off the AST, so this matcher is not its
// own fixture.
func bdRealBdCallName(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "exec" {
		return false
	}
	// The binary is the first argument of Command and the second of
	// CommandContext, whose first is the context. Nothing in the corpus
	// spells CommandContext, so the arm that reaches that case is
	// TestQABdDaemonGuardReadsCallsNotProse's planted one (ranger-base-36nyh).
	var arg int
	switch sel.Sel.Name {
	case "Command":
		arg = 0
	case "CommandContext":
		arg = 1
	default:
		return false
	}
	if len(call.Args) <= arg {
		return false
	}
	lit, ok := call.Args[arg].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	v, err := strconv.Unquote(lit.Value)
	return err == nil && v == "bd"
}

// bdScanVisitor is the walk function both scans below share: which files it
// opens, the parse, the AST match, and the collection. Only the ROOT differs
// between the two, and each says why it spells its root the way it does.
// What it collects per carrying file is that file's parsed string LITERALS,
// which is what the clearing half judges in place of the file's bytes
// (ranger-base-q82lc). Errors are returned rather than swallowed: a walk
// that reads nothing must not look like a tree with nothing in it, and a
// file that does not parse must not look like a file with no calls in it.
// Both of those returns are controlled by
// TestQABdDaemonGuardCannotSwallowAnUnreadableFile (ranger-base-36nyh).
func bdScanVisitor(root string, fset *token.FileSet, sites *[]bdCallSite, files map[string][]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if name := info.Name(); name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, path, b, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		var found []bdCallSite
		var lits []string
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if bdRealBdCallName(n) {
					found = append(found, bdCallSite{file: rel, line: fset.Position(n.Pos()).Line})
				}
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					if v, err := strconv.Unquote(n.Value); err == nil {
						lits = append(lits, v)
					}
				}
			}
			return true
		})
		// Only a file that actually shells bd is a candidate for the rule, so
		// only such a file's literals are kept — a file that names the call in
		// prose is not judged at all.
		if len(found) == 0 {
			return nil
		}
		*sites = append(*sites, found...)
		files[rel] = lits
		return nil
	}
}

// scanRealBdCallSites returns every real-bd call site in THIS tree, and the
// files that hold them.
//
// The root is spelled as a literal AT THE WALK, and this function takes no
// root at all, because that spelling is the whole of what makes the pin
// below a MEMBER of the tree-wide class the Makefile doors
// (ranger-base-0ckgm). The register derives the internal/treepins half of
// that class from an enumeration rooted at a relative literal
// (twdRelTreePath over twdWalkers, treewidedoor_qa_test.go), and
// twdRelTreePath cannot read a variable. This scan took `root string` and
// the test body called `scanRealBdCallSites(".")`, so the one literal was
// spelled one hop away from where the register looks: the pin was not
// derived, TestQAEveryTreeWidePinHasADoor never asked for a door, and none
// existed — a reading of every _test.go in the tree whose only route was an
// unfiltered 589.965s package run. Spelling it here is the same fix
// twdTestFiles' own head comment prescribes for the same mistake made
// inside the register itself.
func scanRealBdCallSites() (sites []bdCallSite, files map[string][]string, err error) {
	files = map[string][]string{}
	err = filepath.Walk(".", bdScanVisitor(".", token.NewFileSet(), &sites, files))
	return sites, files, err
}

// scanBdCallSitesUnder is the same scan over a planted FIXTURE tree, for the
// control arms below. Its root IS a parameter, which is correct here: nothing
// hands it a tree path, so it is not a reading of the tree at all and no door
// is owed. A parameter handed a tree path is read by the register's hop now
// (twdParamHops, ranger-base-pyl22) and so would be doored rather than
// hidden; what holds that over this whole package, mechanically, is
// TestQAEveryTreePathHandedThroughAParameterIsDerived (rootparam_qa_test.go).
func scanBdCallSitesUnder(root string) (sites []bdCallSite, files map[string][]string, err error) {
	files = map[string][]string{}
	err = filepath.Walk(root, bdScanVisitor(root, token.NewFileSet(), &sites, files))
	return sites, files, err
}

// leakyBdTestFiles names the files that shell the real bd and neither pass
// `--no-daemon` nor stop the daemon they started. It reads the file's parsed
// string literals, never its bytes: a comment naming either token is prose,
// and prose excuses nothing (ranger-base-q82lc). Sorted, so a failure names
// the same files in the same order twice running.
func leakyBdTestFiles(files map[string][]string) []string {
	var bad []string
	for rel, lits := range files {
		cleared := false
		for _, v := range lits {
			if strings.Contains(v, "--no-daemon") || strings.Contains(v, "daemon.pid") {
				cleared = true
				break
			}
		}
		if !cleared {
			bad = append(bad, rel)
		}
	}
	sort.Strings(bad)
	return bad
}

func TestQANoTestShellsRealBdWithoutStoppingItsDaemon(t *testing.T) {
	sites, files, err := scanRealBdCallSites()
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	// The positive witness. An assertion of pure absence is satisfied by a
	// scan that measured nothing — a renamed helper, a walk that hit the
	// wrong root — so say out loud what was actually read, and fail if the
	// answer is "no live bd anywhere", which has not been true since
	// rangerhq-09o2 shipped.
	if len(sites) == 0 {
		t.Fatal("scanned the tree and parsed no call to the real bd at all — this guard is measuring nothing, not passing")
	}
	t.Logf("scanned %d real-bd call sites in %d files", len(sites), len(files))

	if bad := leakyBdTestFiles(files); len(bad) > 0 {
		t.Errorf("these files shell the real bd and leave its daemon running: %s\n"+
			"pass `--no-daemon` on the calls, or stop the daemon in a t.Cleanup by SIGTERMing "+
			"the pid in the fixture's own .beads/daemon.pid — never `bd daemon stop-all`, "+
			"which would take the canonical queue's daemon with it (ranger-base-42mv)", strings.Join(bad, ", "))
	}
}

// The control. The rule above is a rule about calls, so it is only worth
// anything if a file that breaks it is actually caught — this plants the
// shape worktreelive_test.go had before the fix and asserts both arms.
func TestQABdDaemonGuardCatchesALeakyFile(t *testing.T) {
	root := t.TempDir()
	leaky := "package x\n\nfunc f() { cmd := exec.Command(\"bd\", \"list\"); _ = cmd }\n"
	if err := os.WriteFile(filepath.Join(root, "leaky_test.go"), []byte(leaky), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err := scanBdCallSitesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 {
		t.Fatalf("the planted call site was not seen: %d sites — the scan, not the rule, is what failed", len(sites))
	}
	if bad := leakyBdTestFiles(files); len(bad) != 1 || bad[0] != "leaky_test.go" {
		t.Fatalf("a file that shells real bd with neither guard must be flagged, got %v", bad)
	}

	// …and the other arm, or "flagged" would just mean "flags everything":
	// the same file with the flag is clean.
	fixed := strings.Replace(leaky, `exec.Command("bd", "list")`, `exec.Command("bd", "--no-daemon", "list")`, 1)
	if err := os.WriteFile(filepath.Join(root, "leaky_test.go"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, files, err = scanBdCallSitesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if bad := leakyBdTestFiles(files); len(bad) != 0 {
		t.Fatalf("`--no-daemon` must clear the file, got %v", bad)
	}

	// …and so does stopping the daemon in cleanup, which is the answer for a
	// fixture whose claim needs a running one.
	stopped := strings.Replace(leaky, "_ = cmd", "_ = cmd; _ = \".beads/daemon.pid\"", 1)
	if err := os.WriteFile(filepath.Join(root, "leaky_test.go"), []byte(stopped), 0o644); err != nil {
		t.Fatal(err)
	}
	_, files, err = scanBdCallSitesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if bad := leakyBdTestFiles(files); len(bad) != 0 {
		t.Fatalf("a daemon.pid cleanup must clear the file, got %v", bad)
	}
}

// The clearing control (ranger-base-q82lc), and the reason the EXCUSE is
// parsed too. Each case is the leaky shape — a real bd call, no flag, no
// cleanup — with one of the two tokens present only as prose, which is
// exactly what the byte matcher accepted as an alibi. Each asserts first that
// the old matcher WOULD have cleared the file (the token is in the bytes), so
// a typo in a fixture cannot pass this test for the wrong reason, and then
// that the rule flags it anyway. The without-arm is TestQABdDaemonGuardCatches…
// above, which proves the same token as a real literal still clears.
func TestQABdDaemonGuardProseDoesNotClearALeakyFile(t *testing.T) {
	const call = "\nfunc f() { cmd := exec.Command(\"bd\", \"list\"); _ = cmd }\n"
	for _, tc := range []struct {
		name  string
		alibi string
		token string
	}{
		{
			name:  "flag named in a line comment",
			alibi: "// This fixture deliberately does not pass --no-daemon.\n",
			token: "--no-daemon",
		},
		{
			name:  "flag named in a block comment",
			alibi: "/*\nThe flag --no-daemon is named here, in prose, and nowhere else.\n*/\n",
			token: "--no-daemon",
		},
		{
			name:  "pid handle named in a line comment",
			alibi: "// Nothing below SIGTERMs the pid in .beads/daemon.pid.\n",
			token: "daemon.pid",
		},
		{
			name:  "flag named in a doc comment on the call itself",
			alibi: "// f shells bd. It should pass --no-daemon and does not.\n",
			token: "--no-daemon",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			body := "package x\n\n" + tc.alibi + call
			if !strings.Contains(body, tc.token) {
				t.Fatalf("the fixture does not carry %q in its bytes, so it is not the "+
					"shape the byte matcher cleared — the fixture, not the rule, is what failed", tc.token)
			}
			if err := os.WriteFile(filepath.Join(root, "leaky_test.go"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			sites, files, err := scanBdCallSitesUnder(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(sites) != 1 {
				t.Fatalf("the planted call site was not seen: %d sites — the scan, not the rule, is what failed", len(sites))
			}
			if bad := leakyBdTestFiles(files); len(bad) != 1 || bad[0] != "leaky_test.go" {
				t.Fatalf("a %q that is only prose must not clear a file that shells real bd "+
					"with neither guard, got %v (ranger-base-q82lc)", tc.token, bad)
			}
		})
	}
}

// The prose arm (ranger-base-9ojr5), and the reason the scan parses. A
// comment naming the call is not a call, and neither is a Go string holding
// the same bytes — the two shapes that inflated the witness and reported a
// file that shells no bd as leaky. Both planted here, in one file, with
// nothing else in the tree: the census must read it as empty.
func TestQABdDaemonGuardReadsCallsNotProse(t *testing.T) {
	root := t.TempDir()
	prose := "package x\n\n" +
		"// A sentence naming exec.Command(\"bd\", \"list\") is prose, not a call site.\n" +
		"func f() { s := `exec.Command(\"bd\", \"list\")`; _ = s }\n"
	if err := os.WriteFile(filepath.Join(root, "prose_test.go"), []byte(prose), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err := scanBdCallSitesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 || len(files) != 0 {
		t.Fatalf("a comment and a string naming the call are not call sites: %d sites in %d files — "+
			"the census is reading text again, which both inflates the witness above and reports a "+
			"file that shells no bd as leaky (ranger-base-9ojr5)", len(sites), len(files))
	}
	// A prose-only file is not even a candidate, so the per-file rule has
	// nothing to judge and cannot name it.
	if bad := leakyBdTestFiles(files); len(bad) != 0 {
		t.Fatalf("a file that shells no bd must not be judged at all, got %v", bad)
	}

	// …and the other direction, or "not counted" would just mean the parse
	// is broken: a real call beside the prose is still one call site, and the
	// prose file is still not one of them.
	caller := "package x\n\nfunc g() { _ = exec.Command(\"bd\", \"list\") }\n"
	if err := os.WriteFile(filepath.Join(root, "caller_test.go"), []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err = scanBdCallSitesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || len(files) != 1 || sites[0].file != "caller_test.go" {
		t.Fatalf("the real call beside the prose must be the only site: %v in %d files", sites, len(files))
	}
	if bad := leakyBdTestFiles(files); len(bad) != 1 || bad[0] != "caller_test.go" {
		t.Fatalf("the real caller must still be judged, got %v", bad)
	}

	// …and the OTHER call spelling, or the matcher's CommandContext arm is
	// free to be wrong: its binary is the SECOND argument, after the context,
	// and nothing in the corpus reaches that case (ranger-base-36nyh).
	ctxCaller := "package x\n\nfunc h(ctx context.Context) { _ = exec.CommandContext(ctx, \"bd\", \"list\") }\n"
	if err := os.WriteFile(filepath.Join(root, "ctxcaller_test.go"), []byte(ctxCaller), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err = scanBdCallSitesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || len(files) != 2 {
		t.Fatalf("`exec.CommandContext(ctx, \"bd\", …)` is a call site too, and its binary is the "+
			"argument AFTER the context: %v in %d files (ranger-base-36nyh)", sites, len(files))
	}
	if bad := leakyBdTestFiles(files); len(bad) != 2 || bad[0] != "caller_test.go" || bad[1] != "ctxcaller_test.go" {
		t.Fatalf("both call spellings must be judged, got %v", bad)
	}
}

// A file the census cannot READ must not read as a file with nothing in it
// (ranger-base-36nyh). This is the positive control bdScanVisitor's two error
// returns never had: with the parse's `return err` read as `return nil`, a
// tree holding an unparseable _test.go scanned to "9 real-bd call sites in 7
// files" and every other arm passed. The walk's own error return is the same
// swallow one step earlier — a root that does not exist, which without it
// scans to nothing with a nil error, and is one of the two ways the
// zero-witness above was written to catch ("a walk that hit the wrong root").
func TestQABdDaemonGuardCannotSwallowAnUnreadableFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "unparseable_test.go"), []byte("package x\n\nfunc f( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err := scanBdCallSitesUnder(root)
	if err == nil {
		t.Fatalf("a _test.go that does not parse must be an error, not a file with no calls in it: "+
			"%d sites in %d files (ranger-base-36nyh)", len(sites), len(files))
	}
	if !strings.Contains(err.Error(), "unparseable_test.go") {
		t.Errorf("the error must name the file the census could not read, got %v", err)
	}

	// The walk's own error return, which is the same swallow one step earlier.
	if _, _, err := scanBdCallSitesUnder(filepath.Join(root, "no-such-dir")); err == nil {
		t.Error("a root that does not exist must be an error, not a tree with nothing in it")
	}
}

// The zero-witness must be able to FIRE. TestQANoTestShellsRealBd…'s
// `len(sites) == 0` guard exists so an empty PASS cannot masquerade as a
// clean tree — a renamed helper, a walk rooted somewhere wrong. Until
// 2026-08-30 it could not fire at all: the byte census counted this file's
// own comment, matcher literal and fixture text as three call sites, so a
// tree with no bd caller whatsoever still reported "3 sites in 1 file" and
// passed. That was held off by excluding this file by name until
// 2026-10-31, and is now held off by the parse — so this arm has become the
// test of that, which is why it plants no exclusion and reads this source as
// an ordinary test file. It is also a without-arm for the control above:
// that one proves a leaky file is caught, this one proves an EMPTY tree is
// not silently blessed.
func TestQABdDaemonGuardZeroWitnessCanFire(t *testing.T) {
	root := t.TempDir()
	b, err := os.ReadFile(filepath.Join("internal", "treepins", guardFile))
	if err != nil {
		t.Fatal(err)
	}
	// A tree holding only this guard: every real bd caller gone.
	if err := os.WriteFile(filepath.Join(root, guardFile), b, 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err := scanBdCallSitesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 || len(files) != 0 {
		t.Fatalf("a tree with no bd caller but this guard must scan to nothing, "+
			"or the zero-witness is satisfied by the guard reading its own source: "+
			"%d sites in %d files", len(sites), len(files))
	}

	// …and the other direction, or "scans to nothing" would just mean the
	// walk is broken: one real caller beside it is still seen.
	caller := "package x\n\nfunc f() { _ = exec.Command(\"bd\", \"list\") }\n"
	if err := os.WriteFile(filepath.Join(root, "other_test.go"), []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err = scanBdCallSitesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || len(files) != 1 {
		t.Fatalf("this guard's own prose and fixtures must read as what they are, not blind the walk: %d sites in %d files", len(sites), len(files))
	}
	if bad := leakyBdTestFiles(files); len(bad) != 1 || bad[0] != "other_test.go" {
		t.Fatalf("the real caller must still be judged, got %v", bad)
	}
}
