package posse

// ranger-base-xw51s, found while verifying four closes under
// ranger-base-3k8pb.
//
// THE CLASS. `git diff` is not one format: an external diff driver replaces
// git's output with whatever a named program prints. It is reachable three
// ways — the GIT_EXTERNAL_DIFF environment variable, `diff.external` in any
// gitconfig the repo can see, and a `diff=<driver>` attribute paired with
// `diff.<driver>.command` — and posse's own inlet pin exports the FIRST of
// them, as the EMPTY STRING, in every pinned seat (inletpin.go, and the
// shipped etc/claude/managed-settings.d/10-posse-inlet-pin.json). git does
// not read set-but-empty as unset: it tries to exec "" and dies with
// `error: cannot run : No such file or directory` / `fatal: external diff
// died`, so every reader without --no-ext-diff was broken for every seat.
//
// memoryland.go's memoryDiff (ranger-base-r5wpk) already stated this list
// for the credential scan, and its comment names GIT_EXTERNAL_DIFF by name.
// Three other readers never got it. These pins are about the two PRODUCT
// ones and about the class not coming back.
//
// WHY THE FIXTURES PLANT diff.external AND NOT THE ENV VAR. TestMain now
// unsets GIT_EXTERNAL_DIFF for the whole binary — the operator's launch
// environment is not an input to this suite — so a pin that set it would be
// pinning TestMain, and a pin that relied on the ambient value would be
// green on any box that did not carry it. `diff.external` is git's config
// spelling of the same setting, it lives in the fixture repo's own config,
// it survives whatever the seat's environment does, and it is the form an
// operator's global gitconfig actually carries. Measured equivalent on git
// 2.50.1: bare `git diff` dies under either spelling, `--no-ext-diff`
// survives both.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// extDiffCmd is a driver git can find no program for. Either failure shape
// would do — this one is loud and cannot collide with a real binary.
const extDiffCmd = "/nonexistent/posse-ranger-base-xw51s-no-such-diff"

// plantExtDiff configures repo so every `git diff` without --no-ext-diff
// dies, and proves the fixture bites before any pin leans on it: a rig that
// cannot be shown to fail measures nothing.
func plantExtDiff(t *testing.T, repo string) {
	t.Helper()
	run := func(args ...string) ([]byte, error) {
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		c.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
		return c.CombinedOutput()
	}
	if out, err := run("config", "diff.external", extDiffCmd); err != nil {
		t.Fatalf("git config diff.external: %v\n%s", err, out)
	}
	// The witness, against the EMPTY TREE and not the worktree: git only
	// runs the driver when there is a patch to render, so a witness on a
	// clean fixture would report the driver inert when it is merely idle
	// (measured — this pin's own first draft passed for that reason). The
	// empty tree always differs from HEAD, and asking git for its oid keeps
	// this right in a sha256 repository too. Nothing in the worktree is
	// touched, so the caller's own state is never what was witnessed.
	empty, err := run("hash-object", "-t", "tree", os.DevNull)
	if err != nil {
		t.Fatalf("git hash-object -t tree: %v\n%s", err, empty)
	}
	rev := strings.TrimSpace(string(empty))
	out, err := run("diff", rev, "HEAD", "--")
	if err == nil {
		t.Fatalf("fixture is inert: `git diff %s HEAD` succeeded with diff.external=%s\n%s", rev, extDiffCmd, out)
	}
	if !strings.Contains(string(out), "external diff died") {
		t.Fatalf("fixture failed for some other reason than the driver:\n%s", out)
	}
	// …and the flag under test must be what rescues it, not something else
	// about this repo.
	if out, err := run("diff", "--no-ext-diff", rev, "HEAD", "--"); err != nil {
		t.Fatalf("--no-ext-diff did not survive the driver: %v\n%s", err, out)
	}
}

// ARM 1, the fail-open — cagestale.go's dirtyIdent. It hashed `git diff
// HEAD` under `if err == nil`, so under a driver NOTHING was written to the
// hash from the tracked half and every dirty tree digested to the same
// value. Two different dirty states then stamp identically and a cage image
// built from one reads CURRENT against the other, with no error printed
// anywhere — the false CURRENT cagestale.go's own header says it exists to
// prevent.
func TestQACageDirtyIdentIgnoresAnExternalDiffDriver(t *testing.T) {
	t.Parallel()
	src := tempGitTree(t)
	plantExtDiff(t, src)
	f := filepath.Join(src, "f")

	if err := os.WriteFile(f, []byte("x\ndirty edit A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	versionAtBuild := SourceBuildVersion(src)
	if versionAtBuild == "" {
		t.Fatal("no version for a dirty tree under a diff driver")
	}
	// The digest of nothing, derived rather than spelled: this is what the
	// swallowed error produced, and its 8-hex prefix (e3b0c442) is what the
	// bead's repro printed for BOTH states.
	nothing := hex.EncodeToString(sha256.New().Sum(nil))[:8]
	if strings.Contains(versionAtBuild, nothing) {
		t.Errorf("the dirty ident is the digest of nothing (%s): %s", nothing, versionAtBuild)
	}

	if err := os.WriteFile(f, []byte("x\ndirty edit A\nAND MORE, DIFFERENT RENDER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	versionNow := SourceBuildVersion(src)

	if got := cageAge("img", "this source", versionAtBuild, versionNow); got.State == CageImageCurrent {
		t.Errorf("false CURRENT under a diff driver: image built at dirty state A (%s) reads current against a since-edited dirty source (%s) — %s",
			versionAtBuild, versionNow, got)
	}
}

// ARM 2, what a failure that --no-ext-diff cannot rescue must do. The flag
// removes the reachable cause; it does not make `git diff` infallible, and
// the swallowed error was the deeper defect. An ident that could not read
// the tracked half is not a narrower ident, it is a COLLIDING one, so
// dirtyIdent now returns "" and SourceBuildStamp propagates that to the
// UNCLEAR verdict cageAge already has for "one of them could not be read at
// all" — which claims nothing either way, rather than claiming sameness.
//
// The fixture: a tracked file the owner cannot read. `git status
// --porcelain` only stats, so the tree still reports dirty and
// SourceBuildStamp still reaches dirtyIdent; `git diff HEAD` must open the
// blob to hash it and cannot, with or without --no-ext-diff (measured, git
// 2.50.1: `error: open("f"): Permission denied` / `fatal: cannot hash f`).
func TestQACageStaleIsUnclearWhenTheDirtyIdentCannotBeRead(t *testing.T) {
	t.Parallel()
	src := tempGitTree(t)
	f := filepath.Join(src, "f")
	if err := os.WriteFile(f, []byte("x\ndirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Readable first: the fixture must be shown to produce an ident at all,
	// or "" below would be the tree's, not the unreadable blob's.
	before := SourceBuildVersion(src)
	if before == "" {
		t.Fatal("no version for a readable dirty tree — the fixture is wrong before the arm starts")
	}

	if err := os.Chmod(f, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(f, 0o644) })
	if _, err := os.ReadFile(f); err == nil {
		t.Skip("this uid can read a 0000 file (root?), so git can too")
	}

	if id := dirtyIdent(src); id != "" {
		t.Errorf("dirtyIdent fingerprinted a tree whose tracked half it could not read: %q", id)
	}
	if v := SourceBuildVersion(src); v != "" {
		t.Errorf("SourceBuildVersion still named a build: %q", v)
	}
	got := cageAge("img", "this source", before, SourceBuildVersion(src))
	if got.State != CageImageUnclear {
		t.Errorf("an unreadable dirty tree is %q, want %q — %s", got.State, CageImageUnclear, got)
	}
}

// ARM 3, the loud one — promote.go's ratification diff. It fails with a
// message rather than silently, but the message is all the operator gets:
// the constitution promote's ONLY preview of what it is about to put in
// force is unavailable, every time, in every pinned seat.
func TestQAPromoteRatificationDiffIgnoresAnExternalDiffDriver(t *testing.T) {
	t.Parallel()
	a, src, git := promoteFixture(t)
	plantExtDiff(t, filepath.Dir(src))

	promote(t, a, PromoteOpts{Source: src})
	if err := os.WriteFile(filepath.Join(src, "agents", "dev.md"), []byte("---\nname: dev\n---\nbuild BETTER things\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git("commit", "-qam", "pid: dev builds better things"); err != nil {
		t.Fatalf("commit: %s", out)
	}

	second := promote(t, a, PromoteOpts{Source: src})
	if strings.Contains(second, "could not be read") {
		t.Errorf("the ratification diff was unreadable under a diff driver:\n%s", second)
	}
	if !strings.Contains(second, "build BETTER things") || !strings.Contains(second, "agents/dev.md") {
		t.Errorf("the ratification diff does not show the PID change under a diff driver:\n%s", second)
	}
}

// ARM 4, the sweep, kept as a pin instead of as a paragraph. This bead
// exists because memoryDiff hardened ONE reader and three more were missed,
// so the check that no further bare reader arrives is worth more than the
// three fixes: every `git diff` this package spells out in argv must either
// state the format (--no-ext-diff, whether by hand or through memoryDiff)
// or ask for a format an external driver cannot reach.
//
// WHAT IS IMMUNE, measured on git 2.50.1 rather than assumed: --name-only,
// --name-status and --numstat never run the driver (git needs no patch to
// answer them), and neither does the `git diff-tree -p` plumbing the hooks
// use for patch-id. A bare `git diff`, `git diff --cached` and
// `git diff -U0` all die.
//
// TWO BLIND SPOTS, stated. It reads the ENCLOSING STATEMENT of each "diff"
// literal, so an argv assembled across statements is invisible to it — the
// two sites this bead fixed are exactly that shape now (they call
// memoryDiff, and carry no "diff" literal at all), and the arms above are
// what hold those. And it cannot see the hook SCRIPTS in gates.go, which
// are shell text: their reader shape is diffReaderShape, which already
// carries --no-ext-diff, pinned separately.
func TestQANoProductGitDiffReaderIsBare(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	// The liveness check, and deliberately not a COUNT of readers: a floor
	// of "at least N" set to today's N reds the day somebody legitimately
	// converts one to memoryDiff, and a floor below N grades nothing. What
	// must be true is that the scan reached the file whose reader is the
	// known-good one — if memoryland.go's own `"diff"` never turns up, the
	// census is looking somewhere else and every green below is empty.
	reachedTheKnownGoodReader := false
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		bare, saw := bareDiffReaders(fset, file)
		if saw && name == "memoryland.go" {
			reachedTheKnownGoodReader = true
		}
		for _, b := range bare {
			t.Error(b)
		}
	}
	if !reachedTheKnownGoodReader {
		t.Error("the census never reached memoryland.go's own `git diff` argv — it is reading the wrong directory, and every arm above passed on an empty scan")
	}
}

// extDiffImmune is the formats an external diff driver cannot reach, plus
// the flag itself. One list for the census and for the pin below, so the
// thing measured and the thing described cannot drift.
var extDiffImmune = []string{"--no-ext-diff", "--name-only", "--name-status", "--numstat"}

// bareDiffReaders is ARM 4's census, extracted so a fixture can run the
// reader ITSELF rather than a second copy of its rules. It returns one
// message per `git diff` argv in file that neither states --no-ext-diff nor
// asks for a format a driver cannot reach, and whether the walk saw any
// `"diff"` literal at all — the caller's liveness check.
//
// SCOPE is the whole judgement, and it is what this function exists to get
// right. A literal is exempted only by an immune flag inside the SMALLEST
// node that bounds ONE argv list around it: the innermost enclosing
// statement, or — at package scope, where there is no statement — the
// innermost *ast.ValueSpec, which is `var x = []string{…}`'s one name and
// its value, the declaration-level twin of an assignment.
//
// Deliberately NOT the file, which is what this arm's first form fell back
// to for want of a statement. Grading a package-scope argv against every
// string in its file exempts a bare reader the moment any OTHER reader in
// the same file asks for --name-only, silently. Measured while verifying
// this bead's close (ranger-base-9m1gm): the identical
// `var a = []string{"diff", "HEAD", "--"}` was REPORTED when alone in a
// file and EXEMPTED once an unrelated `--name-only` reader was added
// beside it. TestQABareDiffCensusScopesToOneArgv holds both halves.
//
// A literal with no bounding scope at all is REPORTED rather than exempted:
// an argv this census cannot classify must not be the one it stays quiet
// about. Same direction as the fix this file pins — failing to read is not
// a narrower answer, it is a wrong one.
func bareDiffReaders(fset *token.FileSet, file *ast.File) (findings []string, sawDiffLiteral bool) {
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		stack = append(stack, n)
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || lit.Value != `"diff"` {
			return true
		}
		sawDiffLiteral = true
		var scope ast.Node
		for i := len(stack) - 1; i >= 0; i-- {
			if v, ok := stack[i].(*ast.ValueSpec); ok {
				scope = v
				break
			}
			if st, ok := stack[i].(ast.Stmt); ok {
				scope = st
				break
			}
		}
		var lits []string
		if scope != nil {
			ast.Inspect(scope, func(m ast.Node) bool {
				if b, ok := m.(*ast.BasicLit); ok && b.Kind == token.STRING {
					lits = append(lits, strings.Trim(b.Value, `"`))
				}
				return true
			})
			for _, f := range extDiffImmune {
				for _, l := range lits {
					if l == f {
						return true
					}
				}
			}
		}
		findings = append(findings, fmt.Sprintf("%s: bare `git diff` reader — an external diff driver replaces its output. Route it through memoryDiff (memoryland.go) or ask for one of %v.\n  argv here: %v",
			fset.Position(lit.Pos()), extDiffImmune, lits))
		return true
	})
	return findings, sawDiffLiteral
}

// ARM 5, the census's own scope rule — the pin ARM 4 did not have, added
// while verifying this bead's close (ranger-base-9m1gm). ARM 4 is the only
// thing standing between this package and the NEXT bare reader, so what it
// exempts has to be measured and not assumed. Every case here is source the
// census is run over directly, so it grades the shipped reader.
//
// Case "package var beside an unrelated immune reader" is the escape: the
// argv it must report is byte-identical to the one in the case above it,
// and the file-scope fallback passed it.
func TestQABareDiffCensusScopesToOneArgv(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		src  string
		want int
	}{
		{"package var, bare, alone in its file", `package p
var a = []string{"diff", "HEAD", "--"}
`, 1},
		{"package var, bare, beside an unrelated immune reader", `package p
var a = []string{"diff", "HEAD", "--"}
var b = []string{"diff", "--name-only", "HEAD"}
`, 1},
		{"package var, immune in its own spec", `package p
var a = []string{"diff", "--no-ext-diff", "HEAD", "--"}
`, 0},
		{"two specs of ONE var block, one bare", `package p
var (
	a = []string{"diff", "--name-only", "HEAD"}
	b = []string{"diff", "HEAD", "--"}
)
`, 1},
		{"in a function, bare", `package p
func f(r string) { g(r, "diff", "HEAD", "--") }
`, 1},
		{"in a function, immune", `package p
func f(r string) { g(r, "diff", "--no-ext-diff", "HEAD", "--") }
`, 0},
		{"in a function, bare, beside an immune sibling statement", `package p
func f(r string) {
	g(r, "diff", "HEAD", "--")
	g(r, "diff", "--numstat", "HEAD")
}
`, 1},
		{"assembled through memoryDiff, no literal at all", `package p
func f(r string) { g(r, memoryDiff("HEAD", "--")...) }
`, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "fixture.go", c.src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got, saw := bareDiffReaders(fset, file)
			if len(got) != c.want {
				t.Errorf("census reported %d bare readers, want %d:\n%s\n--- source ---\n%s",
					len(got), c.want, strings.Join(got, "\n"), c.src)
			}
			// Liveness, per case: a census that saw no `"diff"` at all would
			// report 0 for every want-0 case without reading anything.
			if !saw && strings.Contains(c.src, `"diff"`) {
				t.Error("the census saw no `\"diff\"` literal in source that carries one")
			}
		})
	}
}
