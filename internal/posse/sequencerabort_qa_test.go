//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-71g2f — the git shim's sequencer-audit arm.
//
// Claim: a seat cannot get a ZERO exit out of `git cherry-pick|revert|rebase`
// while the operation is still in progress. See renderSequencerAudit for the
// live measurement this stands on; what this file owns is that the rendered
// shell really behaves that way, and that the arm stays as narrow as the
// measurement says it must be.
//
// WHY THE PIN IS ABOUT SILENCE. The defect it guards is not a crash, it is a
// zero exit — the working tree is restored, HEAD is right, and since ADR 0059
// granted the lock there is not even a stderr line to read: the two routes
// that still strand a blocker (a STRAY lock someone else left in the shared
// git dir, and L4, where nothing binds one) each end in git retrying for
// `core.packedRefsTimeout` and then exiting 0, silently. If this arm stops
// firing, nothing goes red anywhere: the next seat to cherry-pick simply
// loses its commit to `fatal: cannot do a partial commit during a
// cherry-pick` and reads that as its own PID refusing it.
//
// Hermetic, and deliberately so: the real failure needs a stray lock sitting
// in another repo's common dir, or an L4 cage, neither of which a test can
// arrange. The fake git below reproduces the only thing the shim can actually
// see — a verb that exits 0 having left a pseudo-ref behind — so these arms
// test the wall, not the cage. It reads no repo root, so it is not a
// tree-wide pin and owes the Makefile no door (AGENTS.md, "A -run filter
// cannot reach a tree-wide pin").
//
// MUTATION-CHECKED (2026-09-11), each mutation applied to gates.go alone and
// the six arms below run against it:
//
//	drop AUTO_MERGE from sequencerLeftovers   reds the recipe arm alone
//	add AUTO_MERGE to sequencerBlockers       reds the AUTO_MERGE arm alone
//	add `merge` to sequencerVerbs             reds the leaves-alone arm alone
//	drop the globalValueOpts scan             reds the `-C` arm alone
//	`[ $posse_src -eq 0 ]` -> `-ne 0`         reds FOUR arms
//	add packed-refs.lock to sequencerLeftovers  reds the common-dir arm alone
//	`--absolute-git-dir` -> `--git-common-dir`  reds the common-dir arm alone
//
// The `-ne 0` one reds four and that is the honest reading, not a leak:
// inverted, the audit fires on every failing run and on no succeeding one, so
// the three arms that expect an alarm go quiet and the one that expects quiet
// alarms. A mutation that breaks the invariant itself is supposed to be
// visible everywhere the invariant is asserted.
//
// The last two are ranger-base-o0dr4's (ADR 0059 D3), and the common-dir arm
// is the one that can go vacuous with nothing else noticing: no other arm in
// this file looks at WHICH dir the recipe points into.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGitForSequencer writes a git stand-in that honours the leading global
// options the way real git does, answers `rev-parse --absolute-git-dir` and
// `--git-common-dir` (differently, for a worktree-shaped git dir), and leaves
// behind whatever $FAKE_LEAK names — which is the whole of what a ref delete
// that could not take the lock looks like from outside.
func fakeGitForSequencer(t *testing.T, dir string) {
	t.Helper()
	body := `#!/bin/sh
gd=$FAKE_GITDIR
while [ $# -gt 0 ]; do
  case "$1" in
    -C) gd="$2/.git"; shift 2 ;;
    -c|--git-dir|--work-tree|--namespace|--super-prefix|--config-env|--attr-source) shift 2 ;;
    -*) shift ;;
    *) break ;;
  esac
done
if [ "$1" = rev-parse ]; then
  # A linked worktree's own git dir is <common>/worktrees/<name>, so these two
  # answers differ there and nowhere else, which is the whole of what the
  # common-dir arm varies. A fake that answered both the same could not refuse.
  case "$2" in
    --git-common-dir) case "$gd" in */worktrees/*) echo "${gd%/worktrees/*}" ;; *) echo "$gd" ;; esac ;;
    *) echo "$gd" ;;
  esac
  exit 0
fi
echo "real git $*"
if [ -n "$FAKE_LEAK" ]; then
  mkdir -p "$gd"
  for m in $FAKE_LEAK; do
    case "$m" in sequencer) mkdir -p "$gd/$m" ;; *) : > "$gd/$m" ;; esac
  done
fi
exit ${FAKE_RC:-0}
`
	if err := WriteExecutable(filepath.Join(dir, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// sequencerShim renders a crew-shaped git shim in front of the fake git and
// returns the shim's path, the gates dir and the git dir the fake reports.
func sequencerShim(t *testing.T) (shim, gatesDir, gitDir string) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	realBin := t.TempDir()
	fakeGitForSequencer(t, realBin)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	// The deny list every crew PID carries — the shim this arm rides on.
	gatesDir, binDir, _, err := a.RenderGates("security", []string{"Bash(git push:*)", "Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(binDir, "git"), gatesDir, filepath.Join(t.TempDir(), "gitdir")
}

// runSequencerShim runs the shim with the fake's knobs set, and returns
// stdout, stderr and the exit code.
func runSequencerShim(t *testing.T, shim, gitDir, leak, rc string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(shim, args...)
	cmd.Env = append(os.Environ(), "FAKE_GITDIR="+gitDir, "FAKE_LEAK="+leak, "FAKE_RC="+rc)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), errb.String(), code
}

// The bead's own case, and the wider one the measurement found: a sequencer
// verb that exits 0 with a blocker still in place is refused, loudly, with
// the recipe that ends the operation.
func TestQASequencerVerbExitingZeroWithoutEndingIsRefused(t *testing.T) {
	shim, gatesDir, gitDir := sequencerShim(t)

	for _, c := range []struct {
		args     []string
		leak     string
		blockers string
	}{
		{[]string{"cherry-pick", "--abort"}, "CHERRY_PICK_HEAD AUTO_MERGE", "CHERRY_PICK_HEAD"},
		{[]string{"cherry-pick", "--quit"}, "CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD"},
		{[]string{"cherry-pick", "--continue"}, "CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD"},
		// The case the bead did not name: a CLEAN cherry-pick leaks too, so
		// the arm must key on the verb and not on --abort.
		{[]string{"cherry-pick", "deadbeef"}, "CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD"},
		{[]string{"revert", "--abort"}, "REVERT_HEAD", "REVERT_HEAD"},
		{[]string{"rebase", "--abort"}, "CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD"},
		{[]string{"rebase", "--continue"}, "CHERRY_PICK_HEAD MERGE_HEAD", "CHERRY_PICK_HEAD MERGE_HEAD"},
	} {
		os.RemoveAll(gitDir)
		out, errs, code := runSequencerShim(t, shim, gitDir, c.leak, "0", c.args...)
		line := "git " + strings.Join(c.args, " ")
		if code != 1 {
			t.Errorf("%s: code=%d, want 1 (err=%q)", line, code, errs)
		}
		if !strings.Contains(out, "real git "+c.args[0]) {
			t.Errorf("%s: the real git must still run: out=%q", line, out)
		}
		for _, m := range strings.Fields(c.blockers) {
			if !strings.Contains(errs, m) {
				t.Errorf("%s: refusal must name the surviving %s: %q", line, m, errs)
			}
		}
		for _, want := range []string{
			"STILL IN PROGRESS",
			"ranger-base-71g2f",
			"cannot do a partial commit during a cherry-pick",
			"that is THIS, not your PID",
			"rm -rf --",
			filepath.Join(gitDir, strings.Fields(c.blockers)[0]),
		} {
			if !strings.Contains(errs, want) {
				t.Errorf("%s: refusal must carry %q: %q", line, want, errs)
			}
		}
	}

	// Every alarm is on the record, in the same log L1's refusals land in.
	logb, err := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(logb), "(alarm: ranger-base-71g2f)"); n != 7 {
		t.Errorf("refusals.log must carry one alarm line per firing, got %d:\n%s", n, logb)
	}
	if !strings.Contains(string(logb), "git cherry-pick --abort exited 0 leaving CHERRY_PICK_HEAD") {
		t.Errorf("alarm line must name the verb and what survived:\n%s", logb)
	}
}

// The recipe removes the STALE state as well as the blocker, so one line ends
// the operation instead of leaving the seat to find AUTO_MERGE and the
// sequencer dir on its next commit.
//
// The wanted set is written out LITERALLY rather than read off
// sequencerLeftovers. Ranging over the variable under test made this pin
// agree with every edit to it — dropping AUTO_MERGE from the list dropped it
// from the assertion too, and the arm stayed green over a recipe that no
// longer ends the operation (caught in this bead's own mutation check, which
// is the only reason it is not still written that way).
func TestQASequencerRecipeNamesEveryLeftoverNotJustTheBlocker(t *testing.T) {
	shim, _, gitDir := sequencerShim(t)
	_, errs, code := runSequencerShim(t, shim, gitDir,
		"CHERRY_PICK_HEAD AUTO_MERGE MERGE_MSG sequencer", "0", "cherry-pick", "--abort")
	if code != 1 {
		t.Fatalf("code=%d err=%q", code, errs)
	}
	for _, m := range []string{"CHERRY_PICK_HEAD", "AUTO_MERGE", "MERGE_MSG", "sequencer"} {
		if p := filepath.Join(gitDir, m); !strings.Contains(errs, "'"+p+"'") {
			t.Errorf("recipe must name %s, quoted: %q", p, errs)
		}
	}
	// And only what is actually there: a path the operation did not leave
	// behind has no business in an `rm -rf` a seat is told to paste.
	for _, m := range []string{"REVERT_HEAD", "MERGE_HEAD"} {
		if p := filepath.Join(gitDir, m); strings.Contains(errs, p) {
			t.Errorf("recipe must not name the absent %s: %q", m, errs)
		}
	}
}

// AUTO_MERGE is not a blocker. A tree holding only AUTO_MERGE commits
// path-limited exactly as it always did (measured), so alarming on it would
// fire where nothing is wrong — which on a verb that leaves it behind on
// EVERY run would be an alarm nobody could act on.
func TestQASequencerAuditIgnoresAutoMergeAlone(t *testing.T) {
	shim, gatesDir, gitDir := sequencerShim(t)
	out, errs, code := runSequencerShim(t, shim, gitDir, "AUTO_MERGE", "0", "cherry-pick", "--abort")
	if code != 0 || strings.Contains(errs, "ranger-base-71g2f") {
		t.Errorf("AUTO_MERGE alone must not alarm: code=%d out=%q err=%q", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(gatesDir, "refusals.log")); err == nil {
		t.Error("a quiet run must write no alarm line")
	}
}

// git's own failure is git's to report. The arm fires only on a ZERO exit —
// a sequencer verb that STOPS (a conflict, a rebase pausing at `edit`) exits
// nonzero with a blocker legitimately in place, and that is not this defect.
func TestQASequencerAuditPassesAFailureThroughUntouched(t *testing.T) {
	shim, _, gitDir := sequencerShim(t)
	for _, rc := range []string{"1", "128"} {
		os.RemoveAll(gitDir)
		_, errs, code := runSequencerShim(t, shim, gitDir, "CHERRY_PICK_HEAD", rc, "cherry-pick", "deadbeef")
		if code != map[string]int{"1": 1, "128": 128}[rc] {
			t.Errorf("rc %s: git's own exit code must survive, got %d", rc, code)
		}
		if strings.Contains(errs, "ranger-base-71g2f") {
			t.Errorf("rc %s: a nonzero exit is not this defect: %q", rc, errs)
		}
	}
}

// The audit asks rev-parse about the repo the VERB ran against, so a leading
// global option that takes a separate value cannot point it at the wrong git
// dir — the same table posse_verb_match consumes (globalValueOpts).
func TestQASequencerAuditFollowsTheGlobalOptionsToTheRepo(t *testing.T) {
	shim, _, gitDir := sequencerShim(t)
	elsewhere := t.TempDir()
	for _, args := range [][]string{
		{"-C", elsewhere, "cherry-pick", "--abort"},
		{"-c", "core.editor=true", "-C", elsewhere, "cherry-pick", "--abort"},
		{"--no-pager", "-C", elsewhere, "cherry-pick", "--abort"},
	} {
		os.RemoveAll(gitDir)
		_, errs, code := runSequencerShim(t, shim, gitDir, "CHERRY_PICK_HEAD", "0", args...)
		want := filepath.Join(elsewhere, ".git")
		if code != 1 || !strings.Contains(errs, want) {
			t.Errorf("git %s: audit must read %s: code=%d err=%q",
				strings.Join(args, " "), want, code, errs)
		}
	}
	// And a global option consuming its value cannot be mistaken for the verb.
	os.RemoveAll(gitDir)
	if _, errs, code := runSequencerShim(t, shim, gitDir, "", "0", "-C", "cherry-pick", "status"); code != 0 || errs != "" {
		t.Errorf("`git -C cherry-pick status` is a status: code=%d err=%q", code, errs)
	}
}

// Narrow by construction: only the three verbs that end by deleting a
// pseudo-ref, and only the `git` shim.
func TestQASequencerAuditLeavesEverythingElseAlone(t *testing.T) {
	shim, _, gitDir := sequencerShim(t)
	// merge and am were MEASURED clean — merge unlinks MERGE_HEAD out of
	// `reset --merge` and am ends by removing a directory, neither through
	// the ref transaction that needs packed-refs.lock.
	// `commit` is absent on purpose: the PID's own deny refuses the
	// unqualified form before the audit is reached, which the last arm here
	// pins directly.
	for _, verb := range []string{"merge", "am", "status", "log", "stash"} {
		os.RemoveAll(gitDir)
		out, errs, code := runSequencerShim(t, shim, gitDir, "CHERRY_PICK_HEAD MERGE_HEAD", "0", verb, "--abort")
		if code != 0 || strings.Contains(errs, "ranger-base-71g2f") {
			t.Errorf("git %s must pass through: code=%d err=%q", verb, code, errs)
		}
		if !strings.Contains(out, "real git "+verb) {
			t.Errorf("git %s must reach the real binary: %q", verb, out)
		}
	}
	// A non-git shim carries no arm at all, and neither does a git shim with
	// no real binary to ask (the runtime PATH search has no name to run).
	if s := renderSequencerAudit("bd", "/usr/local/bin/bd", "/bin/date"); s != "" {
		t.Errorf("the arm is git's alone: %q", s)
	}
	if s := renderSequencerAudit("git", "", "/bin/date"); s != "" {
		t.Errorf("no real binary, no arm: %q", s)
	}
	// The deny rules the arm rides on still refuse first: the audit is
	// appended AFTER posse_verb_match, so a refused push never reaches it.
	if _, errs, code := runSequencerShim(t, shim, gitDir, "", "0", "push", "origin", "main"); code != 1 ||
		!strings.Contains(errs, "refused by posse gate: git push origin main") {
		t.Errorf("the deny arm must still fire first: code=%d err=%q", code, errs)
	}
}

// ADR 0059 D3: the recipe a seat is told to paste names the pseudo-refs in
// the session's OWN git dir and nothing in the shared part of the common dir
// — above all not the `packed-refs.lock` that is now the only thing standing
// between the seat and a finished cherry-pick. Git's lock files carry no
// holder identity and no expiry, so a session cannot tell a stranded lock
// from the one a live `gc` is holding; an `rm` on that guess is the
// destructive act the wall exists to prevent, and removing it is the
// operator's.
//
// "Under the common dir" cannot be a substring test, and that is why this arm
// is shaped the way it is. A linked worktree's own git dir IS
// `<common>/worktrees/<name>` (MEASURED on this box, git 2.50.1: `rev-parse
// --absolute-git-dir` answers it and `--git-common-dir` answers the dir two
// levels up), so every CORRECT recipe names paths under the common dir
// textually. What D3 forbids is the SHARED part — the common dir outside that
// private subtree — and the fixture below is worktree-shaped so the two
// answers differ and the distinction is measurable at all.
func TestQASequencerRecipeStaysOutOfTheCommonDir(t *testing.T) {
	shim, _, _ := sequencerShim(t)

	// The two structural halves run FIRST, so a change that stops the arm
	// firing at all still reports WHY here rather than only that it went
	// quiet. The first is structural because outside a worktree-shaped git dir
	// the two rev-parse answers are the same string, so no execution can tell
	// them apart.
	rendered := renderSequencerAudit("git", "/usr/bin/git", "/bin/date")
	if !strings.Contains(rendered, "rev-parse --absolute-git-dir") {
		t.Error("the audit no longer asks rev-parse for the session's own git dir")
	}
	if strings.Contains(rendered, "--git-common-dir") {
		t.Error("the audit asks git for the COMMON dir, the one place ADR 0059 D3 says its recipe may not point")
	}
	// The lists the recipe is built from hold this session's leftovers only.
	for _, m := range append(append([]string{}, sequencerLeftovers...), sequencerBlockers...) {
		if strings.Contains(m, "packed-refs") {
			t.Errorf("%q is shared state, not a leftover of this session's operation", m)
		}
	}

	common := filepath.Join(t.TempDir(), ".git")
	own := filepath.Join(common, "worktrees", "w1")
	stray := filepath.Join(common, "packed-refs.lock")
	if err := os.MkdirAll(common, 0o755); err != nil {
		t.Fatal(err)
	}
	// Case 1's cause, sitting in plain sight for a shim that decided to tidy.
	if err := os.WriteFile(stray, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// A lock in the session's OWN git dir as well, which real git never puts
	// there — deliberately, so the mutation this arm exists to catch is
	// REACHABLE by execution and not only by reading the list. The recipe
	// names what it finds, so a `packed-refs.lock` added to sequencerLeftovers
	// prints only if there is one to find.
	_, errs, code := runSequencerShim(t, shim, own,
		"CHERRY_PICK_HEAD AUTO_MERGE MERGE_MSG sequencer packed-refs.lock", "0", "cherry-pick", "--abort")
	if code != 1 {
		t.Fatalf("the arm did not fire, so this pin is reading nothing: code=%d err=%q", code, errs)
	}
	// The positive half first: it does still end the operation, in the dir
	// that is the session's own. An arm that only asserts absence is happy
	// with a recipe that names nothing at all.
	if want := "'" + filepath.Join(own, "CHERRY_PICK_HEAD") + "'"; !strings.Contains(errs, want) {
		t.Fatalf("recipe does not name the blocker in the session's own git dir (%s): %q", want, errs)
	}
	// Every path it tells the seat to remove is inside that dir.
	for _, line := range strings.Split(errs, "\n") {
		if !strings.Contains(line, "rm ") {
			continue
		}
		// Not by path alone: a `packed-refs.lock` the recipe reached by any
		// spelling is the removal D3 forbids.
		if strings.Contains(line, "packed-refs.lock") {
			t.Errorf("the recipe tells a seat to remove a packed-refs.lock (ADR 0059 D3): %q", line)
		}
		for _, q := range strings.Split(line, "'")[1:] {
			if !strings.HasPrefix(q, "/") {
				continue // the quoted `fatal: ...` text, not a path
			}
			if !strings.HasPrefix(q, own+string(os.PathSeparator)) {
				t.Errorf("the recipe tells a seat to remove %q, outside its own git dir %s (ADR 0059 D3): %q", q, own, errs)
			}
		}
	}
	// And the shared part of the common dir is not named anywhere in the
	// refusal, quoted or not, recipe or prose — the stray lock included.
	for _, shared := range []string{stray,
		filepath.Join(common, "packed-refs"),
		filepath.Join(common, "packed-refs.new"),
		filepath.Join(common, "refs"),
		filepath.Join(common, "HEAD"),
	} {
		if strings.Contains(errs, shared) {
			t.Errorf("the refusal names %s, which is the operator's to touch and not this session's (ADR 0059 D3): %q", shared, errs)
		}
	}
	// It audits, it does not repair: the stray lock is still there.
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("the stray lock is gone — the audit removed shared state on a guess: %v", err)
	}
}
