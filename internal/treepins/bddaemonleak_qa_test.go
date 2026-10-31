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
// RESIDUAL, said out loud: a call whose binary name is not a literal —
// `exec.Command(bdPath, …)`, `exec.Command("bd"+suffix, …)` — is not seen,
// and never was. What bounds the class is that every real-bd call site in the
// corpus spells the name, and the positive witness fires if the census ever
// stops seeing them.
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
	// CommandContext, whose first is the context.
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

// scanRealBdCallSites returns every real-bd call site under root, and the
// files that hold them. Errors are returned rather than swallowed: a walk
// that reads nothing must not look like a tree with nothing in it, and a file
// that does not parse must not look like a file with no calls in it.
func scanRealBdCallSites(root string) (sites []bdCallSite, files map[string]string, err error) {
	files = map[string]string{}
	fset := token.NewFileSet()
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !bdRealBdCallName(call) {
				return true
			}
			sites = append(sites, bdCallSite{file: rel, line: fset.Position(call.Pos()).Line})
			files[rel] = string(b)
			return true
		})
		return nil
	})
	return sites, files, err
}

// leakyBdTestFiles names the files that shell the real bd and neither pass
// `--no-daemon` nor stop the daemon they started.
func leakyBdTestFiles(files map[string]string) []string {
	var bad []string
	for rel, body := range files {
		if strings.Contains(body, "--no-daemon") || strings.Contains(body, "daemon.pid") {
			continue
		}
		bad = append(bad, rel)
	}
	return bad
}

func TestQANoTestShellsRealBdWithoutStoppingItsDaemon(t *testing.T) {
	sites, files, err := scanRealBdCallSites(".")
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
	sites, files, err := scanRealBdCallSites(root)
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
	_, files, err = scanRealBdCallSites(root)
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
	_, files, err = scanRealBdCallSites(root)
	if err != nil {
		t.Fatal(err)
	}
	if bad := leakyBdTestFiles(files); len(bad) != 0 {
		t.Fatalf("a daemon.pid cleanup must clear the file, got %v", bad)
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
	sites, files, err := scanRealBdCallSites(root)
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
	sites, files, err = scanRealBdCallSites(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || len(files) != 1 || sites[0].file != "caller_test.go" {
		t.Fatalf("the real call beside the prose must be the only site: %v in %d files", sites, len(files))
	}
	if bad := leakyBdTestFiles(files); len(bad) != 1 || bad[0] != "caller_test.go" {
		t.Fatalf("the real caller must still be judged, got %v", bad)
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
	sites, files, err := scanRealBdCallSites(root)
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
	sites, files, err = scanRealBdCallSites(root)
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
