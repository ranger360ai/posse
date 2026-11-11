//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// init.templateDir is in hookSkipRegexp, and that regexp is what the global
// scope compare reads (ranger-base-crlty). The -c, env and write arms use
// their own key glob, so the legs that type init.templateDir stay green with
// the key dropped from the regexp. MEASURED 2026-11-11 (ranger-base-sbfop):
// with it dropped, `HOME=<elsewhere> git clone` hid a global init.templateDir
// at rc 0 with no refusal. This pin is the one leg where init.templateDir is
// the ONLY difference between the launcher's global scope and the call's.
func TestQAHookSkipMovedHomeHidingTemplateDirIsRefused(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			global, err := os.ReadFile(filepath.Join(fx.home, ".gitconfig"))
			if err != nil {
				t.Fatal(err)
			}
			// The launcher's global scope carries the template; the moved
			// HOME carries the same file minus that one line.
			write(t, filepath.Join(fx.home, ".gitconfig"), string(global)+"[init]\n\ttemplateDir = "+fx.cleanTpl+"\n")
			moved := filepath.Join(fx.home, "moved")
			write(t, filepath.Join(moved, ".gitconfig"), string(global))

			home := t.TempDir()
			a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
			rt, err := a.LoadRuntime(rtName)
			if err != nil {
				t.Fatal(err)
			}
			wrapped, _, _, err := a.WrapWithGates("hs-tpl-"+rtName, rt, []string{"Bash(git push:*)"}, t.TempDir(), "")
			if err != nil {
				t.Fatal(err)
			}

			// Control: the same clone with HOME left alone is allowed.
			errs, code := hsRun(t, fx, wrapped, hsLeg{args: "clone -q " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "ok"))})
			if code != 0 || strings.Contains(errs, "hookskip:") {
				t.Fatalf("plain clone: code=%d, want 0 and no hookskip refusal (%s)", code, errs)
			}
			errs, code = hsRun(t, fx, wrapped, hsLeg{env: "HOME=" + shQuote(moved) + " ", args: "clone -q " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "hid"))})
			if code == 0 || !strings.Contains(errs, "rule: hookskip:config-env,") {
				t.Errorf("HOME moved to hide init.templateDir: code=%d, want refused by hookskip:config-env (%s)", code, errs)
			}
		})
	}
}
