package rhq

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A FOURTH spelling of force-push, found verifying ranger-base-zs6b's close
// (ranger-base-c9l8).
//
// zs6b decided (b) — the rule that means "no force-push" is
// `Bash(git push:*)`, never the flag rule beside it — and that decision is
// right and is not what this pins. What it pins is that the residual written
// down beside it is an ENUMERATION, and an enumeration is a claim that can be
// short. gates.go, ADR 0001 and CHANGELOG all say THREE spellings, stamped
// **MEASURED**; this is a fourth, from the same git-push(1) the three were
// read out of (git 2.50.1 / Apple Git-155, `man git-push`, READ rather than
// run because every crew PID denies the verb):
//
//	--mirror
//	    Instead of naming each ref to push, specifies that all refs under
//	    refs/ ... be mirrored to the remote repository. Newly created local
//	    refs will be pushed to the remote end, LOCALLY UPDATED REFS WILL BE
//	    FORCE UPDATED ON THE REMOTE END, and deleted refs will be removed
//	    from the remote end. This is the default if the configuration option
//	    remote.<remote>.mirror is set.       [emphasis mine]
//
// It carries no `--force` token, so it walks past `Bash(git push --force:*)`
// exactly as the other three do — and its last sentence is worse than a
// fourth spelling, because under `remote.<remote>.mirror` the force-update is
// what a bare `git push origin` DOES, with no option and no refspec for any
// matcher to read at all. That is the same lesson `+main` teaches, one step
// further out: the effect is not in the argv.
//
// Green on both sides of the doc fix, deliberately. It asserts what git does
// and what the two rules do about it, not how many spellings some comment
// counts — a pin that asserted "the comment says four" would go red the day
// somebody found a fifth, which is the wrong direction to fail in. The
// enumeration's own correction is ranger-base-e7eo, filed on the devops
// lane that owns the rule.
func TestQAMirrorIsAFourthForcePushSpellingThatOnlyTheVerbRuleCloses(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	os.WriteFile(filepath.Join(realBin, "git"), []byte("#!/bin/sh\necho \"real git $*\"\n"), 0o755)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	shimFor := func(persona, deny string) string {
		_, binDir, _, err := a.RenderGates(persona, []string{deny})
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Join(binDir, "git")
	}
	run := func(shim string, args ...string) (string, string, int) {
		cmd := exec.Command(shim, args...)
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return out.String(), errb.String(), code
	}
	flagRule := shimFor("developer", "Bash(git push --force:*)")
	verbRule := shimFor("architect", "Bash(git push:*)")
	for _, args := range [][]string{
		{"push", "--mirror", "origin"},
		// and the shape `remote.<remote>.mirror` leaves in the argv: nothing
		// at all. No matcher over spellings can reach this one either.
		{"push", "origin"},
	} {
		line := strings.Join(args, " ")
		if out, errs, code := run(flagRule, args...); code != 0 || strings.TrimSpace(out) != "real git "+line {
			t.Errorf("residual moved: git %s is no longer past the flag rule — code=%d out=%q err=%q", line, code, out, errs)
		}
		out, errs, code := run(verbRule, args...)
		if code != 1 || !strings.Contains(errs, "refused by posse gate: git "+line+" (deny: Bash(git push:*))") || out != "" {
			t.Errorf("the prescribed remedy must refuse git %s: code=%d out=%q err=%q", line, code, out, errs)
		}
	}
	// The wrong arm, the same one zs6b's own pin carries and for the same
	// reason:
	// without it every "passes" row above reads identically against a shim
	// that was never rendered into that binDir at all.
	if out, errs, code := run(flagRule, "push", "--force", "origin", "main"); code != 1 || !strings.Contains(errs, "(deny: Bash(git push --force:*))") || out != "" {
		t.Errorf("the flag rule must still refuse the one spelling it names: code=%d out=%q err=%q", code, out, errs)
	}
}
