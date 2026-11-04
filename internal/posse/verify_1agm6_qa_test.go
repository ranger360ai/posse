//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-1agm6, verifying the close of ranger-base-z8kym.
//
// WHAT PROCESS BEHAVES DIFFERENTLY IF THIS PIN IS DELETED (ADR 0006 §7): the
// L1 git shim's hook-skip arm lets a seat commit past an employer-managed
// pre-commit scanner — the thing ADR 0052 says posse must never cause to be
// skipped — whenever an alias already in config spells its flag inside
// quotes. No refusal, no refusals.log line, rc 0, commit landed. Deleting
// this pin takes the only executable witness of that with it.
//
// THE HOLE. z8kym item 2 shipped a resolver (hookskip:alias) that looks up a
// non-builtin verb, refuses a `!` expansion outright, and splices any other
// expansion into argv to be re-walked by every arm. hookskip.go's header
// says of that splice: "it splits on IFS and does not unquote, which can
// only refuse more". The second half is false, and in the unsafe direction.
// git's own split_cmdline UNQUOTES an alias value, so:
//
//	alias.x = commit '-n' -q -m x --     git sees: commit -n …   (hook skipped)
//	                                     the arm sees: commit '-n' …
//
// and `'-n'` is not `-n`, so no scanner in the arm matches it. The same gap
// swallows a quoted `--no-verify` and a quoted `-c core.hooksPath=` key
// inside an expansion.
//
// MEASURED 2026-11-04, this box (darwin 26.7.1, git 2.50.1 Apple Git-155),
// at main fe39111a, through a real rendered claude seat shim with the crew
// deny list: five legs each exited 0, the commit LANDED (HEAD subject is the
// alias's own -m), and the managed pre-commit hook fired ZERO times, while
// the plain commit fired it and the unquoted `pci` alias was refused. The
// shipped TestQAHookSkipRefusedOnEveryRuntime was green throughout: its
// alias legs are all unquoted, so no shipped leg reaches this.
//
// REACHABILITY, and it is why this is P2 and not P1: a seat cannot PLANT
// such an alias through the shim. All eight `git config` write spellings
// below are refused under hookskip:config-write, and a quoted `-c` KEY is
// rejected by git itself ("invalid key"). The hole needs an alias already in
// a config file — a pre-existing operator or employer alias, which is the
// very shape ranger-base-d4bk4 is filed about, or a hand-edited config,
// which ADR 0025 declares a residual. The resolver's whole job is to judge
// an expansion it did not write, so this sits inside z8kym's scope, not past
// its edge.
//
// THIS PIN SHIPS GREEN ASSERTING THE HOLE. When the hole is fixed the five
// legs below start refusing and this test REDS, naming the inversion — that
// is the handover, not a regression. The scratch repo is not the tree, so
// this is not a tree-wide pin and owes the Makefile no door.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// v1agm6QuotedAliases are expansions git unquotes and the arm's splice does
// not. The value is what `git config alias.<k>` is given, past the shim.
var v1agm6QuotedAliases = map[string]string{
	"q1agmN":     "commit '-n' -q -m q1agmN --",
	"q1agmNdq":   `commit "-n" -q -m q1agmNdq --`,
	"q1agmNoVer": "commit '--no-verify' -q -m q1agmNoVer --",
	"q1agmHooks": "-c 'core.hooksPath=/dev/null' commit -q -m q1agmHooks --",
	"q1agmAlias": "-c 'alias.q1agmInner=commit -n' q1agmInner -q -m q1agmAlias --",
}

// v1agm6AliasWrites are the spellings that would PLANT one. Every one must
// stay refused: they are the boundary that keeps this a P2.
var v1agm6AliasWrites = []string{
	`config alias.w1 "commit '-n' -q -m w1 --"`,
	`config --global alias.w2 "commit '-n' -q -m w2 --"`,
	`config --add alias.w3 "commit '-n' -q -m w3 --"`,
	`config set alias.w4 "commit '-n' -q -m w4 --"`,
	`config --local alias.w5 "commit '-n' -q -m w5 --"`,
	`config --replace-all alias.w6 "commit '-n' -q -m w6 --"`,
	`config -- alias.w7 "commit '-n' -q -m w7 --"`,
	`config --file .git/config alias.w8 "commit '-n' -q -m w8 --"`,
}

func TestQAHookSkipAliasSpliceDoesNotUnquote(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
	wrapped, _, _, err := a.WrapWithGates("hs-1agm6", rt, []string{"Bash(git push:*)", "Bash(git commit unless --)"}, "")
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range v1agm6QuotedAliases {
		hsRealGit(t, fx, "-C", fx.repo, "config", "alias."+k, v)
	}

	// CONTROL 1: the managed hook is reachable at all — without this the
	// "fired 0 times" readings below are true of nothing.
	if errs, code := hsRun(t, fx, wrapped, hsLeg{args: "commit -q -m plain -- f"}); code != 0 || hsFired(fx) != 1 {
		t.Fatalf("CONTROL: a plain commit must run the managed hook: code=%d fired=%d (%s)", code, hsFired(fx), errs)
	}
	// CONTROL 2: the arm is live, and judges an UNQUOTED persisted alias.
	before := hsFired(fx)
	if errs, code := hsRun(t, fx, wrapped, hsLeg{args: "pci -q -m pci -- f"}); code == 0 || !strings.Contains(errs, "hookskip:") {
		t.Fatalf("CONTROL: the unquoted alias `pci` must be refused: code=%d (%s)", code, errs)
	} else if hsFired(fx) != before {
		t.Fatalf("CONTROL: `pci` was refused but the hook still ran")
	}

	// THE BOUNDARY: a seat may not plant a quoted alias through the shim.
	for _, w := range v1agm6AliasWrites {
		errs, code := hsRun(t, fx, wrapped, hsLeg{args: w})
		if !strings.Contains(errs, "hookskip:") {
			t.Errorf("THE HOLE GOT WORSE: `git %s` was not refused (code=%d), so a seat can now PLANT the quoted alias this pin is about, not merely inherit one. Re-price ranger-base-1agm6's finding above P2.\n%s", w, code, errs)
		}
	}

	// THE HOLE, asserted as it stands.
	for name, expansion := range v1agm6QuotedAliases {
		before := hsFired(fx)
		errs, code := hsRun(t, fx, wrapped, hsLeg{args: name + " f"})
		fired := hsFired(fx) - before
		landed := v1agm6Head(t, fx)
		if code == 0 && fired == 0 && landed == name {
			continue // the hole, as measured
		}
		if strings.Contains(errs, "hookskip:") && fired == 0 {
			t.Errorf("FIXED — invert this assertion: `git %s` (alias %q) is now REFUSED by the hook-skip arm. ranger-base-z8kym's quoted-expansion hole is closed: rewrite this pin to require the refusal and its rule, drop the \"ships green asserting the hole\" framing in the header, and correct hookskip.go's \"can only refuse more\" sentence if it still stands.\n%s", name, expansion, errs)
			continue
		}
		t.Errorf("neither the hole nor the fix: `git %s` (alias %q) code=%d hookfired=%d HEAD=%q errs=%q — read this before trusting either arm", name, expansion, code, fired, landed, errs)
	}
}

func v1agm6Head(t *testing.T, fx *hsFixture) string {
	t.Helper()
	cmd := exec.Command(fx.realGit, "-C", fx.repo, "log", "-1", "--format=%s")
	cmd.Env = hsEnv(fx)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	return strings.TrimSpace(string(out))
}
