//go:build !posse_arm2 && !posse_arm3

package posse

// Extension legs for the hook-skip read arm's option skip
// (ranger-base-c1pqo, verified under ranger-base-rrklg).
//
// ranger-base-c1pqo taught posse_hs_readverb to skip the leading options
// before `remote`'s subcommand, and its shipped legs
// (hookskip_qa_test.go, hsRefused/hsAllowed) carry the seven spellings the
// bead measured: -v, --verbose, -v -v, -vv, --verb, `-v update` and
// `-v -- prune`. git's own option surface is wider than that list, and the
// skip's whole claim is that it does not depend on the list — so these are
// the spellings the bead did NOT name, each one MEASURED against plain git
// first so the leg states a real premise and not a guess.
//
// MEASURED 2026-11-06, git 2.50.1 (Apple Git-155), darwin 25.6.0, in a
// scratch clone with a stale remote-tracking ref, plain git and no shim
// (`git remote -h` prints the surface: `-v, --[no-]verbose    be verbose;
// must be placed before a subcommand`, and it is the ONLY top-level option
// `remote` has, a flag that takes no value):
//
//	remote --no-verbose prune origin     rc 0, the stale ref was DELETED
//	remote --no-verb prune origin        rc 0, the stale ref was DELETED
//	remote -v -v -v prune origin         rc 0, the stale ref was DELETED
//	remote -v update --prune             rc 0, the stale ref was DELETED
//	remote --no-verbose                  rc 0, a read — the ref survived
//	remote -v set-url origin /nowhere    rc 0, a config WRITE, no ref moved
//	worktree -v list                     rc 129, `unknown switch` — worktree
//	                                     takes no option before its subverb
//
// So `--no-verbose` and its abbreviations are the half of git's own
// `--[no-]verbose` spelling that no shipped leg reaches, and the arm must
// refuse every one of them for the same reason it refuses `-v prune`: each
// performs a reference update, which is what rule (a) of
// hookSkipReadSubverbs' table admits none of.
//
// `remote -v set-url` is here for the other edge: a leading option in front
// of a subcommand that writes CONFIG rather than a ref. The table admits
// only "", get-url and show, so it is refused — the read arm narrows to
// reads, it does not enumerate writers.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestQAHookSkipOptionSkipHoldsPastItsPinnedSpellings(t *testing.T) {
	t.Parallel()
	// The argv shapes, with the plain-git reading above as the reason.
	refused := []hsLeg{
		{"", "-c core.hooksPath=/dev/null remote --no-verbose prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote --no-verb prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote -v -v -v prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote -v update --prune", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote -v set-url origin /nowhere", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote --no-verbose remove origin", "hookskip:config-override"},
		// A global git option between the override and the verb: the verb
		// walk has to reach `remote` before the subcommand skip can run at
		// all (ranger-base-ap4p6 deferred the refusal to the verb).
		{"", "-c core.hooksPath=/dev/null --no-pager remote -v prune origin", "hookskip:config-override"},
		{"", "--no-pager -c core.hooksPath=/dev/null remote --no-verbose prune origin", "hookskip:config-override"},
	}
	allowed := []hsLeg{
		{"", "-c core.hooksPath=/dev/null remote --no-verbose", ""},
		{"", "-c core.hooksPath=/dev/null remote --no-verb", ""},
		{"", "-c core.hooksPath=/dev/null remote -v -v get-url origin", ""},
		{"", "-c core.hooksPath=/dev/null remote --no-verbose show -n origin", ""},
		{"", "-c core.hooksPath=/dev/null --no-pager remote -v get-url origin", ""},
	}

	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			home := t.TempDir()
			a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
			rt, err := a.LoadRuntime(rtName)
			if err != nil {
				t.Fatal(err)
			}
			fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
			wrapped, _, _, err := a.WrapWithGates("hs-ext-"+rtName, rt,
				[]string{"Bash(git push:*)", "Bash(git commit unless --)"}, "")
			if err != nil {
				t.Fatal(err)
			}

			// The control the whole arm rests on: a plain commit through
			// this shim runs the managed hook. Without it a shim that
			// refused everything would pass every refused leg below.
			if errs, code := hsRun(t, fx, wrapped, hsLeg{args: "commit -q -m plain -- f"}); code != 0 || hsFired(fx) != 1 {
				t.Fatalf("plain commit: code=%d fired=%d, want 0 and 1 (%s)", code, hsFired(fx), errs)
			}

			for _, leg := range refused {
				before := hsFired(fx)
				errs, code := hsRun(t, fx, wrapped, leg)
				line := leg.env + "git " + leg.args
				if code == 0 {
					t.Errorf("%s: exited 0, want refused by %s — this spelling updates a ref with hooks off", line, leg.rule)
					continue
				}
				if !strings.Contains(errs, "rule: "+leg.rule+",") || !strings.Contains(errs, "refused by posse gate") {
					t.Errorf("%s: refusal must name %s: %q", line, leg.rule, errs)
				}
				if hsFired(fx) != before {
					t.Errorf("%s: the hook ran — the refusal came after git, not before", line)
				}
			}
			for _, leg := range allowed {
				errs, _ := hsRun(t, fx, wrapped, leg)
				line := leg.env + "git " + leg.args
				if strings.Contains(errs, "hookskip:") {
					t.Errorf("%s: refused by the hook-skip arm, want allowed — it updates no ref: %q", line, errs)
				}
			}
		})
	}
}
