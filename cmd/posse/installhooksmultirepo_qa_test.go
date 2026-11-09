package main

// QA pins for ranger-base-jamt2 — `posse gates install-hooks` installs into
// EVERY repo on its command line, reports each one, and attempts the rest
// after one of them refuses.
//
// WHAT WAS WRONG. The arm's argument loop assigned a single `dir`:
//
//	for _, a2 := range args[1:] {
//		if a2 == "--chain" { chain = true; continue }
//		dir = posse.ExpandTilde(a2)   // ← the LAST one wins
//	}
//
// so N repos installed into the last one and said nothing whatever about the
// other N-1. MEASURED 2026-11-08 on the operator's box, posse 0.5.3+8433641d:
// one `posse gates install-hooks` over a brace expansion naming FOUR of that
// instance's repos — which the shell expands to four arguments, and which is
// the line the coordinator had been handing the operator for weeks — printed
// the LAST repo's two install lines and exited 0, with the other three repos'
// walls untouched (mtimes unchanged, and one of their ADR 0029 G12 stale rows
// still standing). The repos are that instance's own, so they are not named
// here; the fixture below reproduces the shape (ADR 0024 D3).
//
// WHY THAT SHAPE IS THE WORST ONE THIS COMMAND HAS. The two slots are the
// push gate and the shared commit wall, and this verb is the remedy every
// degraded line, every G12 row and `make verify-hook-freshness` prescribes. A
// run that installs one repo of four and exits 0 is indistinguishable, at the
// terminal, from a run that installed all four: the operator reads `installed
// <path>` twice and a zero exit and stops looking, and the repos that stayed
// stale are the ones nothing else re-renders (Makefile, verify-hook-freshness).
// Eleven minutes of a live bypass in ranger-base-l717c's class came from four
// repos going stale that way once; this bug put the same gap one hand-typed
// line away, every time.
//
// WHY THESE ARE BLACK-BOX RUNS. Every claim is about what the command DID to
// three repos and what it SAID about them, and the second arm's claim — the
// repos after a refusal are still attempted — was an `os.Exit(1)` in the
// middle of the arm, which no predicate's return value can be wrong about.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ihmFixture gives a home and three git repos, A B C in that order, all three
// declared in the home's `beads_visibility:` so ADR 0081 D1 makes it their
// owner and no arm here is measuring D4's refusal by accident.
func ihmFixture(t *testing.T) (bin, home string, repos []string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	bin = buildRhq(t)
	// RESOLVED for the reason the two sibling files resolve it: the binary
	// absolutizes against its own physical cwd, so an unresolved
	// `t.TempDir()` under darwin's /var -> /private/var symlink would make
	// the header assertions below measure path spelling instead of coverage.
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	home = filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	// THREE, not two: the bug kept the LAST argument, so a two-repo pin can
	// be passed by a command that drops everything but its tail. Three makes
	// the middle one — the one the second arm refuses — neither first nor
	// last.
	for _, n := range []string{"a-repo", "b-repo", "c-repo"} {
		r := filepath.Join(root, n)
		if err := os.MkdirAll(r, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "-C", r, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v %s", r, err, out)
		}
		repos = append(repos, r)
		// The premise cwoFixture asserts next door: AbbrevHome rewrites a
		// path under $HOME to `~/...`, and the headers this pin reads are
		// AbbrevHome's output.
		if r == home || strings.HasPrefix(r, home+string(filepath.Separator)) {
			t.Skipf("fixture premise: %s lies under the home %s, so the header would be abbreviated", r, home)
		}
	}
	ihmDeclare(t, home, repos...)
	return bin, home, repos
}

// ihmDeclare writes the home's `beads_visibility:` list. Which repos a home
// lists is what ADR 0081 D1 reads as ownership, so it is the one knob the
// second arm turns.
func ihmDeclare(t *testing.T, home string, listed ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("beads_visibility:\n")
	for _, r := range listed {
		fmt.Fprintf(&b, "  %s: private\n", r)
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ihmRun runs the command from `home`, from a cwd that is none of the repos —
// so a target this command silently dropped cannot be installed by the `.`
// default standing in for it.
func ihmRun(t *testing.T, bin, home, cwd string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"gates", "install-hooks"}, args...)...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+home, "RHQ_HOME="+home)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("gates install-hooks %v: %v %s", args, err, out)
	}
	return string(out), code
}

// ihmSlots reports whether both slots of the wall are present in a repo. Both,
// because the bug dropped the repo entirely and either one alone would read as
// half an install — which is a different failure (rangerhq-mgdk's).
func ihmSlots(t *testing.T, repo string) (prePush, commitGuard bool) {
	t.Helper()
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		_, err := os.Stat(filepath.Join(repo, ".git", "hooks", slot))
		if slot == "pre-push" {
			prePush = err == nil
		} else {
			commitGuard = err == nil
		}
	}
	return prePush, commitGuard
}

// ─── the bug ─────────────────────────────────────────────────────────────────

func TestQAInstallHooksInstallsIntoEveryRepoOnItsCommandLine(t *testing.T) {
	t.Parallel()
	bin, home, repos := ihmFixture(t)
	root := filepath.Dir(repos[0])

	out, code := ihmRun(t, bin, home, root, repos...)

	if code != 0 {
		t.Fatalf("three owned repos, exit %d:\n%s", code, out)
	}
	for i, r := range repos {
		pre, guard := ihmSlots(t, r)
		if !pre || !guard {
			// The failure names the position, because the bug had exactly
			// one survivor and it was the last.
			t.Errorf("repo %d of %d (%s) has pre-push=%v prepare-commit-msg=%v — every positional is a target, and this is the shape of the bug: only the LAST argument was installed into\n%s",
				i+1, len(repos), r, pre, guard, out)
		}
		// SAID, as well as done: four unheaded blocks are the same page as
		// one, and the operator reading this is checking off repos.
		if !strings.Contains(out, r) {
			t.Errorf("the report does not name %s, so a reader cannot tell which block is which repo:\n%s", r, out)
		}
	}
	// The count, which is the one line that reads wrong if a target was
	// dropped before the loop ever saw it.
	if want := "install-hooks: 3 repo(s), 3 ok, 0 refused or failed"; !strings.Contains(out, want) {
		t.Errorf("the report does not carry the line\n  %s\n%s", want, out)
	}
}

// --chain is a flag wherever it stands, and it is not a repo. The old loop
// skipped it correctly and this pin keeps that true now that every OTHER
// positional is a target — a flag read as a path would be a repo nobody asked
// for, installed into or reported on.
func TestQAInstallHooksTakesChainFromAnyPositionAndAppliesItToEveryRepo(t *testing.T) {
	t.Parallel()
	bin, home, repos := ihmFixture(t)
	root := filepath.Dir(repos[0])

	out, code := ihmRun(t, bin, home, root, repos[0], "--chain", repos[1])

	if code != 0 {
		t.Fatalf("two owned repos with --chain between them, exit %d:\n%s", code, out)
	}
	for _, r := range repos[:2] {
		if pre, guard := ihmSlots(t, r); !pre || !guard {
			t.Errorf("%s has pre-push=%v prepare-commit-msg=%v:\n%s", r, pre, guard, out)
		}
	}
	// The third repo was not on the line, so nothing may have reached it —
	// the mirror of the claim above, and what catches a loop that installed
	// into every repo it could find rather than every repo it was given.
	if pre, guard := ihmSlots(t, repos[2]); pre || guard {
		t.Errorf("%s was named on no command line and has pre-push=%v prepare-commit-msg=%v", repos[2], pre, guard)
	}
	if want := "install-hooks: 2 repo(s), 2 ok, 0 refused or failed"; !strings.Contains(out, want) {
		t.Errorf("the report does not carry the line\n  %s\n%s", want, out)
	}
}

// ─── a refusal is one repo's, not the command's ──────────────────────────────

// ADR 0081 D4's refusal used to be an os.Exit(1) in the middle of this arm, so
// with N targets it would have ended the command where it stood: the repos
// AFTER the refused one would go untouched, with a nonzero exit and a refusal
// about a different repo as the only trace. That is the same silence this bead
// is about, read from the other side, so the fix is not complete without it.
func TestQAInstallHooksAttemptsTheReposAfterOneItRefuses(t *testing.T) {
	t.Parallel()
	bin, home, repos := ihmFixture(t)
	root := filepath.Dir(repos[0])
	a, b, c := repos[0], repos[1], repos[2]

	// B's wall, rendered and stamped by a home that OWNS it. A second home
	// is what D1/D2 need: the refusal reads the `# posse-home:` stamp of a
	// wall this home did not render in a repo this home does not list.
	other := filepath.Join(root, "other-home")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	ihmDeclare(t, other, b)
	if out, code := ihmRun(t, bin, other, root, b); code != 0 {
		t.Fatalf("fixture premise: the other home's own install into %s exited %d:\n%s", b, code, out)
	}
	slot := filepath.Join(b, ".git", "hooks", "prepare-commit-msg")
	before, err := os.ReadFile(slot)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(slot)
	if err != nil {
		t.Fatal(err)
	}
	beforeTime := st.ModTime().UnixNano()
	if got := cwoStampOf(string(before)); got != other {
		t.Fatalf("fixture premise: %s is stamped %q, want the other home %q — without D2's stamp there is nothing for D4 to refuse", slot, got, other)
	}
	// This home owns A and C and has never listed B.
	ihmDeclare(t, home, a, c)

	out, code := ihmRun(t, bin, home, root, a, b, c)

	// THE CLAIM: C is after the refusal on the command line, and C is
	// installed.
	if pre, guard := ihmSlots(t, c); !pre || !guard {
		t.Errorf("%s stands AFTER the refused %s and has pre-push=%v prepare-commit-msg=%v — a refusal in one repo must not end the command:\n%s", c, b, pre, guard, out)
	}
	if pre, guard := ihmSlots(t, a); !pre || !guard {
		t.Errorf("%s stands before the refused %s and has pre-push=%v prepare-commit-msg=%v:\n%s", a, b, pre, guard, out)
	}
	// And the refusal is still a refusal: nonzero, named, and the slot
	// byte-for-byte and mtime-for-mtime where it was (ADR 0081 D4's
	// done-when, which commitwallowner_qa_test.go pins for one target).
	if code == 0 {
		t.Errorf("exit 0 over a repo whose wall this home refused to write:\n%s", out)
	}
	after, err := os.ReadFile(slot)
	if err != nil {
		t.Fatal(err)
	}
	st, err = os.Stat(slot)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) || st.ModTime().UnixNano() != beforeTime {
		t.Errorf("%s moved: bytes equal=%v, mtime %d → %d", slot, string(after) == string(before), beforeTime, st.ModTime().UnixNano())
	}
	if !strings.Contains(out, "not installed: prepare-commit-msg") {
		t.Errorf("the report does not say the commit wall was not installed in %s:\n%s", b, out)
	}
	// The count says how many, so a reader who skims to the last line is
	// told that one of the three did not take.
	if want := "install-hooks: 3 repo(s), 2 ok, 1 refused or failed"; !strings.Contains(out, want) {
		t.Errorf("the report does not carry the line\n  %s\n%s", want, out)
	}
}

// ─── the two edges the N-repo reading creates ────────────────────────────────

// A positional that looks like a flag is a usage error. The old loop assigned
// it to `dir` like any other positional, so where it landed decided what
// happened: `install-hooks <repo> --chian` absolutized the typo to
// `$PWD/--chian`, reported THAT as the repo it could not install into, and
// left the repo the operator named untouched — while `install-hooks --chian
// <repo>` silently worked, because the repo came last. Now that every
// positional is a target, a swallowed flag would be a repo nobody asked for,
// so it is refused before anything is written.
//
// The run below puts the flag FIRST, which is the spelling the old loop let
// through, so this arm reds on the old code for the refusal and not for a
// coincidence of ordering.
func TestQAInstallHooksRefusesAFlagItDoesNotKnowRatherThanTakingItForARepo(t *testing.T) {
	t.Parallel()
	bin, home, repos := ihmFixture(t)
	root := filepath.Dir(repos[0])

	out, code := ihmRun(t, bin, home, root, "--chian", repos[0])

	if code == 0 {
		t.Errorf("exit 0 over an unknown flag:\n%s", out)
	}
	if !strings.Contains(out, "--chian") {
		t.Errorf("the usage error does not name the flag it refused:\n%s", out)
	}
	// The one-repo form, named in the error, because that is what the reader
	// has to retype — and `[dir...]`, because the grammar is now plural and
	// an operator reading `[dir]` would go back to typing one command per
	// repo.
	if !strings.Contains(out, "install-hooks [dir...]") {
		t.Errorf("the usage error does not print the grammar:\n%s", out)
	}
	if pre, guard := ihmSlots(t, repos[0]); pre || guard {
		t.Errorf("%s has pre-push=%v prepare-commit-msg=%v — a usage error installs nothing", repos[0], pre, guard)
	}
}

// ONE target reports exactly what it always reported: no header, no count.
// Those two lines exist because N blocks cannot be told apart, and a header
// over a single block would change the output of the form every degraded line,
// scripts/verify-hook-freshness.sh and most of INSTALL.md prescribes — read by
// the pins next door and by every operator who has typed this command.
func TestQAInstallHooksAddsNoHeaderOrCountForASingleTarget(t *testing.T) {
	t.Parallel()
	bin, home, repos := ihmFixture(t)
	root := filepath.Dir(repos[0])

	out, code := ihmRun(t, bin, home, root, repos[0])

	if code != 0 {
		t.Fatalf("one owned repo, exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "install-hooks: ") {
		t.Errorf("a single target printed the multi-repo count line:\n%s", out)
	}
	if strings.Contains(out, repos[0]+":\n") {
		t.Errorf("a single target printed a repo header:\n%s", out)
	}
	// Still the install it always was, so this arm cannot pass by printing
	// nothing at all.
	if pre, guard := ihmSlots(t, repos[0]); !pre || !guard {
		t.Errorf("%s has pre-push=%v prepare-commit-msg=%v:\n%s", repos[0], pre, guard, out)
	}
}
