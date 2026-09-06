//go:build posse_arm2

package posse

// ranger-base-xw51s, found while verifying four closes under
// ranger-base-3k8pb.
//
// THE CLASS. `git diff` is not one format: an external diff driver replaces
// git's output with whatever a named program prints. It is reachable three
// ways — the GIT_EXTERNAL_DIFF environment variable, `diff.external` in any
// gitconfig the repo can see, and a `diff=<driver>` attribute paired with
// `diff.<driver>.command`. posse's own inlet pin exported the FIRST of them,
// as the EMPTY STRING, in every pinned seat (inletpin.go, and the shipped
// etc/claude/managed-settings.d/10-posse-inlet-pin.json). git does
// not read set-but-empty as unset: it tries to exec "" and dies with
// `error: cannot run : No such file or directory` / `fatal: external diff
// died`, so every reader without --no-ext-diff was broken for every seat.
//
// THAT ROW IS GONE — the operator ruled it out rather than pay `git diff`
// patch format fleet-wide (ranger-base-5sph1, applied on ranger-base-888fv)
// — and NOTHING IN THIS FILE WEAKENS BECAUSE OF IT. Two reasons, and the
// second is the one that matters. First, the fixtures never leaned on the
// env var: they plant `diff.external` in the fixture repo's own config, for
// the reason the next paragraph gives, so they measure the same thing in a
// tree with no pin at all. Second, dropping the row does not close the
// inlet, it ACCEPTS it: the name is now settable by anyone who can write a
// lower-scope settings `env` block, which is the whole threat model the pin
// was written for. A reader that states its format is immune to all three
// spellings; a reader that does not is now exposed to one more of them than
// it was.
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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
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

// ARM 6, the surface ARM 4 says it cannot read, and the class ARM 4's sweep
// was never asked about (ranger-base-l1ix2, verifying ranger-base-9m1gm).
// ARM 4 grades the `git diff` argvs this package BUILDS. It cannot see two
// other places the same command is spelled out:
//
//   - the hook SCRIPTS, which are shell text inside string literals and
//     assembled by concatenation, so no single literal holds a whole argv;
//   - the command a shipped surface PRESCRIBES — the prepare-commit-msg
//     refusal tells a persona whose commit was just refused to run
//     `git diff HEAD -- <paths>` before retrying, and AGENTS.md and NOTES.md
//     tell them the same thing about their own paths and about
//     `.beads/issues.jsonl`. Nothing runs those; a reader does.
//
// A prescribed check is the same defect as a broken reader, one indirection
// out: in a pinned seat the bare form exits 128 with nothing but the
// driver's death, on exactly the non-empty case the check exists to detect.
// It was the escape from ranger-base-xw51s.
//
// THE CENSUS reads the RENDERED hook bodies, not gates.go — the render is
// what ships, and it is past every concatenation. `git diff-tree` is a
// different command and measured immune (ARM 4's header), so the scan
// requires a word boundary and never matches it.
//
// SCOPE, stated: markdown is graded inside CODE SPANS only (backticks and
// fenced blocks). A prescription in this repo is always backticked, and
// grading prose would red NOTES.md the day somebody writes the words in a
// sentence. Two spans in NOTES.md name `git diff HEAD` as a MEASUREMENT of
// what a half-broken git still does rather than as a check to run; they are
// exempted by their own sentence, and the exemption is checked live below,
// so a rewritten paragraph cannot leave a dead one behind.
//
// A CODE SPAN IS NOT A LINE. It may open on one line and close on the next,
// and in these two docs it routinely does, so the span is joined before it is
// graded — see gitDiffSpans and extDiffJoinWrapped, and ARM 8 for the join's
// own rules. Without that, rewrapping a prescription un-pinned it silently
// (ranger-base-3ersc); with it, the same rewrap is graded and the same red
// names the same argv.
//
// THE POPULATION IS THE TWO HOOK RENDERS, THE TWO OPERATING DOCS AND EVERY
// SEED PID, and the rest of the tree was swept by hand once
// (ranger-base-l1ix2, at this commit) rather than pinned: README.md,
// INSTALL.md and the ADRs spell
// `git diff` only to DESCRIBE what a product reader does — promote's
// ratification diff, the hook's own --name-only reader — and those readers
// are what ARMs 1-4 hold. docs/notes.d/ is out too, deliberately: those
// fragments are frozen per-bead records (they still name `internal/rhq/`
// paths that no longer exist), and a record quoting the command as it was
// then is accurate.
//
// THE SEED PIDs ARE IN THE POPULATION because they are shipped instructions,
// not descriptions: `examples/agents/reviewer.md` told a reviewer to run a
// bare `git diff` over exactly the change they were sent to read, and that is
// the same prescription AGENTS.md:42 was. That hand-sweep did not name
// examples/ and so did not reach it (ranger-base-3ersc FINDING 1), which is
// the SECOND time a rule reached RHQ_HOME/agents/ and not the seed — the
// first was the L1 commit wall (ranger-base-09b7). So the population is
// DERIVED from the seed rather than enumerated: a PID added tomorrow is
// graded without anyone remembering this file.
//
// And read from posse.Seed, the EMBED, for the reason commitwallseed_qa_test
// gives: `//go:embed all:examples` is what a release binary carries and what
// `posse init` lays down in a fresh instance, so the embed is the artifact
// the gap was in. In this checkout it is the same bytes as ../../examples.
func TestQAPrescribedGitDiffChecksStateTheirFormat(t *testing.T) {
	t.Parallel()
	seen := map[string]int{}
	for _, s := range extDiffSurfaces(t) {
		for _, span := range gitDiffSpans(s.text, s.markdown) {
			seen[s.name]++
			if extDiffImmuneSpan(span.argv) {
				continue
			}
			if extDiffExempt(span.sentence) {
				continue
			}
			t.Errorf("%s prescribes `%s`, which dies rc 128 wherever an external diff driver is configured — by diff.external in any gitconfig, by a `diff=<driver>` attribute, or by GIT_EXTERNAL_DIFF, which posse stopped pinning shut on ranger-base-888fv and now leaves open — on exactly the non-empty case it is there to detect. State the format: %v", s.name, span.argv, extDiffImmune)
		}
	}
	// LIVENESS, and not a count of spans: a floor set to today's N reds the
	// day somebody legitimately rewords a paragraph. What must be true is
	// that the extractor reached each surface that HAS a `git diff` in it —
	// an extractor reading nothing grades nothing, and every green above is
	// an empty scan. (The pre-push render carries none, and eight of the
	// nine seed PIDs carry none, which is why this list is named rather than
	// derived from the population.) reviewer.md is here so that the fix to
	// its line is HELD: deleting the prescription, or rewording the file
	// until the extractor no longer finds it, is a red naming the file
	// rather than one fewer span in a total nobody counts — which is how
	// ranger-base-3ersc FINDING 2 escaped in NOTES.md.
	for _, name := range []string{"the prepare-commit-msg hook render", "AGENTS.md", "NOTES.md", "examples/agents/reviewer.md"} {
		if seen[name] == 0 {
			t.Errorf("the census found no `git diff` at all in %s — it is reading the wrong text, and the pass above measured nothing", name)
		}
	}
	// The exemptions must still describe something in the tree. A stale one
	// is a hole nobody can see: the sentence it named is gone, so the span
	// it covered would be graded by a rule nobody re-read.
	notes := extDiffReadDoc(t, "../../NOTES.md")
	for sentence, why := range extDiffDocExempt {
		if !strings.Contains(notes, sentence) {
			t.Errorf("NOTES.md no longer carries the exempted sentence %q (%s) — delete the exemption or re-anchor it; until then that span is ungraded", sentence, why)
		}
	}
}

// ARM 7, the prescriptions RUN. The census above grades a string; this runs
// the command the surfaces actually print, against a repo whose driver is
// planted, and requires a patch out of it.
//
// THE FIXTURE IS DIRTY IN BOTH DIRECTIONS, and that is the whole rig. git
// runs the driver only when it has a patch to render, so the same recipe on
// a clean path exits 0 under a broken driver and the arm measures nothing —
// the trap plantExtDiff's own comment names, which is why the tracked file
// is modified on disk AND has different content staged: `git diff HEAD`,
// `git diff --cached` and the two-dot form are each non-empty here.
//
// THE CONTROL is per command and not per suite: the same argv with
// --no-ext-diff removed must die. That is what says the driver was live for
// THIS argv on THIS path, rather than idle — a green above it would
// otherwise be a green earned by rendering nothing.
func TestQAPrescribedGitDiffChecksRunUnderADiffDriver(t *testing.T) {
	t.Parallel()
	repo := tempGitTree(t)
	run := func(args ...string) ([]byte, error) {
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		c.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
		return c.CombinedOutput()
	}
	// staged ≠ HEAD and worktree ≠ staged: every form the surfaces spell has
	// something to render.
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := run("add", "--", "f"); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("on disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plantExtDiff(t, repo)

	ran := map[string]int{}
	for _, s := range extDiffSurfaces(t) {
		for _, span := range gitDiffSpans(s.text, s.markdown) {
			if !span.prescribed || !extDiffImmuneSpan(span.argv) {
				continue
			}
			argv := extDiffRunnable(span.argv, "f")
			ran[s.name]++
			out, err := run(argv...)
			if err != nil {
				t.Errorf("%s prescribes `%s`; it does not run in a seat with a diff driver: %v\n%s", s.name, span.argv, err, out)
				continue
			}
			if len(strings.TrimSpace(string(out))) == 0 {
				t.Errorf("%s prescribes `%s`; it ran but rendered nothing over a tree that differs from HEAD, from the index and from both — a check that prints nothing over real work is the failure it exists to catch:\n%q", s.name, span.argv, string(out))
				continue
			}
			// The control, on this exact argv and this exact path.
			bare := extDiffStripFlag(argv, "--no-ext-diff")
			if len(bare) == len(argv) {
				t.Errorf("control not built for `%s`: nothing to strip", span.argv)
				continue
			}
			if out, err := run(bare...); err == nil {
				t.Errorf("control is inert for `%s`: the same argv without --no-ext-diff SUCCEEDED, so the driver rendered nothing here and the pass above measured nothing:\n%s", span.argv, out)
			}
		}
	}
	// A run over zero prescriptions is a green earned by finding nothing —
	// and a FLOOR does not say that, which is the lesson of
	// ranger-base-3ersc FINDING 2 read a second time. This arm ran 7
	// prescriptions the day the floor was 3, so four of them could vanish
	// without a word. What must be true is the same thing ARM 6 asserts:
	// each surface that PRESCRIBES one had one RUN. A prescription that
	// stops being found is then a red naming its file, not a smaller number
	// in a total nobody reads.
	for _, name := range []string{"the prepare-commit-msg hook render", "AGENTS.md", "NOTES.md", "examples/agents/reviewer.md"} {
		if ran[name] == 0 {
			t.Errorf("no prescribed check was RUN from %s — either it prescribes none any more or the extractor stopped finding them, and every green above was earned somewhere else", name)
		}
	}
}

// extDiffDocExempt maps a NOTES.md sentence to why the `git diff` it spells
// is not a check. Keyed on the SENTENCE, not on the command, so a new
// prescription that happens to be spelled the same way is still graded.
var extDiffDocExempt = map[string]string{
	"still renders a full patch":  "a measurement of what an invalid `status.*` value does NOT break, not a check to run",
	"kills status AND":            "a measurement of what a garbage `.git/index` breaks, not a check to run",
	"Making one half of git fail": "the paragraph header names the two commands as its subject; it prescribes neither",
}

// extDiffExempt answers whether a span's own sentence is one of the two
// NOTES.md measurements, by the phrase that identifies it.
func extDiffExempt(sentence string) bool {
	for phrase := range extDiffDocExempt {
		if strings.Contains(sentence, phrase) {
			return true
		}
	}
	return false
}

// extDiffSurface is one shipped place a `git diff` is spelled out.
type extDiffSurface struct {
	name     string
	text     string
	markdown bool
}

// extDiffSurfaces is the population: the two hook bodies posse installs, as
// RENDERED, the two docs that prescribe the same check in prose, and every
// PID the seed ships.
func extDiffSurfaces(t *testing.T) []extDiffSurface {
	t.Helper()
	out := []extDiffSurface{
		{name: "the prepare-commit-msg hook render", text: CommitGuardHook(VisibilityPublic, OpsPatternSet{})},
		{name: "the pre-push hook render", text: PrePushHook},
		{name: "AGENTS.md", text: extDiffReadDoc(t, "../../AGENTS.md"), markdown: true},
		{name: "NOTES.md", text: extDiffReadDoc(t, "../../NOTES.md"), markdown: true},
	}
	return append(out, extDiffSeedSurfaces(t)...)
}

// extDiffSeedSurfaces is every example PID, read from the embed. The name is
// the path an operator sees in the repo, so a red says which file to open.
//
// The corpus floor is not decoration: fs.ReadDir on a subtree that moved
// returns nothing and no error, and a census over zero surfaces is a green
// earned by reading nothing — the failure mode this whole file exists to
// refuse. Nine is what the seed ships today (commitwallseed_qa_test.go asks
// the same question of the same corpus).
func extDiffSeedSurfaces(t *testing.T) []extDiffSurface {
	t.Helper()
	names := exampleAgentNames(posse.Seed)
	if len(names) < 9 {
		t.Fatalf("the seed ships %d example PIDs (%v) — a census over a corpus this small is measuring nothing", len(names), names)
	}
	var out []extDiffSurface
	for _, n := range names {
		// A slash path, which is what an fs.FS takes; path.Join is not
		// imported here because extDiffReadDoc's own parameter is named
		// `path` and shadowing the package in one function while using it
		// in another reads badly.
		rel := "agents/" + n + ".md"
		b, err := fs.ReadFile(posse.Seed, rel)
		if err != nil {
			// Not a skip: the embed is compiled into this binary, so a
			// read that fails is a broken build, not a missing checkout.
			t.Fatalf("read %s from the embed: %v", rel, err)
		}
		out = append(out, extDiffSurface{name: "examples/" + rel, text: string(b), markdown: true})
	}
	return out
}

func extDiffReadDoc(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		// Not a skip: these two docs are in the repo this package ships
		// from, and a missing one means the arm graded nothing.
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// extDiffSpan is one `git diff` command found in a surface. prescribed marks
// the ones a HUMAN is told to run — a quoted or backticked command — as
// opposed to a reader the hook runs itself, whose operands are shell
// variables and cannot be executed here.
type extDiffSpan struct {
	argv       string
	sentence   string // the line it sits on, for the exemption lookup
	prescribed bool
}

// gitDiffSpans returns every `git diff` command spelled out in text. In
// markdown only CODE SPANS are read (see ARM 6's SCOPE note); in shell text
// everything is, since it is all code.
//
// A WRAPPED SPAN IS ONE SPAN. These docs wrap INSIDE a backtick span as a
// matter of course — AGENTS.md:22, :31 and :46 each carry a git command whose
// span opens on one line and closes on the next — so a scan that reads a line
// at a time grades a prescription only while it happens to fit. That is not a
// theory: rewrapping NOTES.md's `.beads/issues.jsonl` prescription across the
// break and dropping `--no-ext-diff` from it left ARMs 4, 6, 7 and 8 ALL GREEN
// (ranger-base-3ersc, verifying ranger-base-l1ix2, which had to rewrap that
// very line to keep the longer command on one line). NOTES.md is where it
// bites and neither liveness guard reaches it: the three exempted measurement
// spans satisfy ARM 6's `seen[name] == 0` on their own, and ARM 7's floor of 3
// AS IT THEN WAS is met by what is left — that floor is gone now, replaced by
// the same per-surface question ARM 6 asks. So the continuation is joined
// before anything is graded, and joined NARROWLY — only when the still-open
// span has already begun a `git diff`, so no other wrapped command in either
// doc is touched.
func gitDiffSpans(text string, markdown bool) []extDiffSpan {
	var out []extDiffSpan
	fenced := false
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if markdown && strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if markdown && !fenced {
			if joined, used := extDiffJoinWrapped(lines, i); used > 0 {
				line = joined
				i += used
			}
		}
		for _, seg := range gitDiffSegments(line, markdown && !fenced) {
			out = append(out, extDiffSpan{argv: seg.argv, sentence: line, prescribed: seg.quoted})
		}
	}
	return out
}

// extDiffJoinWrapped answers what lines[i] reads as once the inline code span
// it leaves OPEN is closed by the lines under it, and how many of those it
// took. It returns used == 0 — and the caller keeps the raw line — unless all
// three hold, because an unclosed backtick that is not a wrapped prescription
// must go on being read exactly as it was:
//
//   - the line's backtick count is odd, so a span is open at the end of it;
//   - that open span has already begun a `git diff`, which is the only command
//     this file grades. A wrapped `git commit -F - -- <paths>` (AGENTS.md:22)
//     is left alone;
//   - the span CLOSES within extDiffWrapLines continuations, stopping at a
//     blank line or a fence, neither of which an inline span may cross.
//
// The continuation is trimmed and joined with one space, which is how the
// rendered markdown reads it.
const extDiffWrapLines = 3

func extDiffJoinWrapped(lines []string, i int) (string, int) {
	line := lines[i]
	if strings.Count(line, "`")%2 == 0 {
		return "", 0
	}
	if open := line[strings.LastIndex(line, "`"):]; !strings.Contains(open, "git diff") {
		return "", 0
	}
	joined := line
	for n := 1; n <= extDiffWrapLines && i+n < len(lines); n++ {
		next := lines[i+n]
		if strings.TrimSpace(next) == "" || strings.HasPrefix(strings.TrimSpace(next), "```") {
			return "", 0
		}
		joined += " " + strings.TrimSpace(next)
		if strings.Count(joined, "`")%2 == 0 {
			return joined, n
		}
	}
	return "", 0
}

type gitDiffSegment struct {
	argv   string
	quoted bool
}

// gitDiffSegments finds the `git diff` commands in one line. codeSpansOnly
// restricts the search to backticked spans, which is what a markdown
// prescription always is here.
func gitDiffSegments(line string, codeSpansOnly bool) []gitDiffSegment {
	var out []gitDiffSegment
	for i := 0; ; {
		j := strings.Index(line[i:], "git diff")
		if j < 0 {
			return out
		}
		start := i + j
		i = start + len("git diff")
		// `git diff-tree` is plumbing and measured immune; the boundary is
		// what keeps it out.
		if i < len(line) && !strings.ContainsRune(" `'\"", rune(line[i])) {
			continue
		}
		// The command runs to the first thing that cannot be part of it: a
		// quote that closes it, or a shell operator.
		end := len(line)
		if k := strings.IndexAny(line[start:], "`'\"|;)&"); k >= 0 {
			end = start + k
		}
		// PRESCRIBED means the command is QUOTED AS A COMMAND — a backtick
		// span in markdown, a single-quoted one inside the hook's echo text
		// — which is only true when the same delimiter opens and closes it.
		// Ending at a quote is not enough on its own: shell code is full of
		// quotes, and `$(git diff … "$posse_base" …)` would otherwise read
		// as something a human was told to type.
		var opener byte
		if start > 0 && strings.ContainsRune("`'\"", rune(line[start-1])) {
			opener = line[start-1]
		}
		quoted := opener != 0 && end < len(line) && line[end] == opener
		if codeSpansOnly && !(quoted && opener == '`') {
			i = end
			continue
		}
		argv := strings.TrimSpace(line[start:end])
		// Redirections belong to the shell, not to the command's format.
		if k := strings.Index(argv, " 2>"); k >= 0 {
			argv = strings.TrimSpace(argv[:k])
		}
		out = append(out, gitDiffSegment{argv: argv, quoted: quoted})
		i = end
	}
}

// extDiffImmuneSpan grades one command text: it must state the format, or
// ask for one an external driver cannot reach.
func extDiffImmuneSpan(argv string) bool {
	for _, f := range extDiffImmune {
		if strings.Contains(argv, f) {
			return true
		}
	}
	return false
}

// extDiffRunnable turns a prescription into an argv this fixture can run:
// its placeholder operands (`<paths>`, `.beads/issues.jsonl`) name files
// that do not exist here, so everything after `--` becomes the one tracked
// file the fixture keeps dirty.
func extDiffRunnable(argv, path string) []string {
	fields := strings.Fields(argv)
	for i, f := range fields {
		if f == "--" {
			return append(append([]string{}, fields[1:i+1]...), path)
		}
	}
	return fields[1:]
}

func extDiffStripFlag(argv []string, flag string) []string {
	out := make([]string, 0, len(argv))
	for _, a := range argv {
		if a != flag {
			out = append(out, a)
		}
	}
	return out
}

// ARM 8, the extractor's own rules — ARM 5's trick applied to ARM 6. The
// census above is only as good as what it reads, and two of its rules have
// no live case in the tree today: nothing in AGENTS.md or NOTES.md names
// `git diff` outside a code span, and no hook script spells `git diff-tree`
// next to something that would be graded. Measured, not assumed: widening
// the code-span filter to prose leaves ARM 6 green, so the filter is pinned
// here instead of by the tree happening to have a case for it.
//
// PRESCRIBED is the judgement that decides what ARM 7 RUNS, and it is the
// one worth stating twice: a command is prescribed when the same delimiter
// opens and closes it — a backtick span in markdown, a single-quoted one in
// the hook's echo text. Shell code is full of quotes, so "ends at a quote"
// would read `$(git diff … "$posse_base")` as something a human was told to
// type, and ARM 7 would then try to run a shell variable.
func TestQAGitDiffSpanExtractorReadsCodeAndNotProse(t *testing.T) {
	t.Parallel()
	// The narrowing case below is written against the live exemption table,
	// so a reworded phrase reds here rather than leaving a fixture that
	// tests nothing.
	const exemptPhrase = "Making one half of git fail"
	if _, ok := extDiffDocExempt[exemptPhrase]; !ok {
		t.Fatalf("fixture stale: %q is no longer a key of extDiffDocExempt — re-anchor the narrowing case on a live phrase", exemptPhrase)
	}
	for _, c := range []struct {
		name       string
		text       string
		markdown   bool
		wantArgv   []string
		prescribed []bool
		wantExempt []bool // nil means none of them
	}{
		{name: "markdown prose mention is not a prescription", markdown: true,
			text:     "a bare git diff HEAD is blind to a staged edit, and so is git diff\n",
			wantArgv: nil},
		{name: "markdown code span is", markdown: true,
			text:       "Check with `git diff --no-ext-diff HEAD -- <paths>`, which compares\n",
			wantArgv:   []string{"git diff --no-ext-diff HEAD -- <paths>"},
			prescribed: []bool{true}},
		{name: "a fenced block is code, not prose", markdown: true,
			text:       "```\ngit diff --no-ext-diff HEAD -- .beads/issues.jsonl\n```\n",
			wantArgv:   []string{"git diff --no-ext-diff HEAD -- .beads/issues.jsonl"},
			prescribed: []bool{false}},
		{name: "shell reader: graded, never prescribed",
			text:       "posse_x=$(git diff --cached --name-only --no-renames HEAD 2>/dev/null)\n",
			wantArgv:   []string{"git diff --cached --name-only --no-renames HEAD"},
			prescribed: []bool{false}},
		{name: "shell reader whose operand is a quoted variable",
			text:       "  posse_added=$(git diff --cached -U0 --no-ext-diff \"$posse_base\" -- 'docs/*' 2>/dev/null |\n",
			wantArgv:   []string{"git diff --cached -U0 --no-ext-diff"},
			prescribed: []bool{false}},
		{name: "the hook's own prescription, inside an echo",
			text:       "  echo \"  'git diff --no-ext-diff HEAD -- <paths>' first: it shows what the\"\n",
			wantArgv:   []string{"git diff --no-ext-diff HEAD -- <paths>"},
			prescribed: []bool{true}},
		{name: "git diff-tree is a different command",
			text:     "  git diff-tree -p \"$1\" 2>/dev/null | git patch-id --stable\n",
			wantArgv: nil},
		{name: "two on one line, both read",
			text:       "  echo \"  'git diff --no-ext-diff HEAD' or 'git diff --no-ext-diff --cached'\"\n",
			wantArgv:   []string{"git diff --no-ext-diff HEAD", "git diff --no-ext-diff --cached"},
			prescribed: []bool{true, true}},
		// THE JOIN, six cases. It has no live case in the tree either — the
		// two docs keep every `git diff` span on one line today, which is
		// precisely why rewrapping one was invisible.
		{name: "a span wrapped across the line break is ONE span", markdown: true,
			text:       "Check with `git diff --no-ext-diff HEAD --\n  <paths>`, which compares the tree\n",
			wantArgv:   []string{"git diff --no-ext-diff HEAD -- <paths>"},
			prescribed: []bool{true}},
		{name: "a bare one wrapped the same way is still read, and still bare", markdown: true,
			text:       "Check with `git diff HEAD --\n  <paths>`, which compares the tree\n",
			wantArgv:   []string{"git diff HEAD -- <paths>"},
			prescribed: []bool{true}},
		{name: "a wrapped span of ANOTHER command is left alone, and a real one beside it still reads", markdown: true,
			text:       "- Close the bead (`git commit -F - --\n  <paths>`). Then `git diff --no-ext-diff HEAD -- x` after.\n",
			wantArgv:   []string{"git diff --no-ext-diff HEAD -- x"},
			prescribed: []bool{true}},
		{name: "the join stops at a blank line, which an inline span cannot cross", markdown: true,
			text:     "Check with `git diff HEAD --\n\n  <paths>`, which compares the tree\n",
			wantArgv: nil},
		{name: "the join stops at a fence", markdown: true,
			text:     "Check with `git diff HEAD --\n```\n<paths>\n```\n",
			wantArgv: nil},
		{name: "a span that never closes is left exactly as it was", markdown: true,
			text:     "Check with `git diff HEAD --\n  a\n  b\n  c\n  d`\n",
			wantArgv: nil},
		// WHY THE JOIN IS NARROW, and the one thing that measures it. A span
		// carries the line it sits on as its SENTENCE, and the sentence is
		// what the exemption is keyed on. Join every unclosed span and a
		// graded prescription inherits the sentence of the line above it —
		// here, an exempted one — and goes silently unread. Joining only a
		// span that has already begun a `git diff` keeps that line's own
		// sentence its own.
		{name: "a wrapped span of ANOTHER command does not lend its sentence to a real one", markdown: true,
			text:       exemptPhrase + " needs (`git status --porcelain --\n  <p>`). Then `git diff HEAD -- x` here.\n",
			wantArgv:   []string{"git diff HEAD -- x"},
			prescribed: []bool{true},
			wantExempt: []bool{false}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := gitDiffSpans(c.text, c.markdown)
			if len(got) != len(c.wantArgv) {
				t.Fatalf("extractor returned %d span(s), want %d:\n%+v\n--- text ---\n%s", len(got), len(c.wantArgv), got, c.text)
			}
			for i, want := range c.wantArgv {
				if got[i].argv != want {
					t.Errorf("span %d = %q, want %q", i, got[i].argv, want)
				}
				if got[i].prescribed != c.prescribed[i] {
					t.Errorf("span %d (%q) prescribed = %v, want %v — prescribed is what ARM 7 tries to RUN", i, got[i].argv, got[i].prescribed, c.prescribed[i])
				}
				wantExempt := false
				if c.wantExempt != nil {
					wantExempt = c.wantExempt[i]
				}
				if exempt := extDiffExempt(got[i].sentence); exempt != wantExempt {
					t.Errorf("span %d (%q) exempt = %v, want %v — the sentence it carried was %q, and an exemption it inherits is a span nobody grades", i, got[i].argv, exempt, wantExempt, got[i].sentence)
				}
			}
		})
	}
}
