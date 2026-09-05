package posse

// Pin for ranger-base-x5cbz: the seatbelt's credential read-deny names
// `~/.codex/auth.json` and `~/.grok/auth.json` as HOME-shaped literals, but
// `$CODEX_HOME` / `$GROK_HOME` move those CLIs' homes — so on a box that
// exports either one the wall is over a file that is not there and the file
// the runtime actually reads is not walled.
//
// This is ranger-base-x5f6p's defect ("a wall over a path the runtime does
// not use") on the two runtimes x5f6p's fix did not cover. x5f6p made the
// CLAUDE half follow the resolver (credentialFileCandidates, which reads
// CLAUDE_SECURESTORAGE_CONFIG_DIR / CLAUDE_CONFIG_DIR); the two siblings
// beside it in the same function kept the hardcoded spelling.
//
// posse already holds the rule these two literals are meant to be: grokHomeIn
// and codexHomeIn (interstitial.go), whose own comment records that the cost
// adapters held a SECOND, hardcoded `~/.grok` / `~/.codex` until
// ranger-base-z65xu swept them. credentialReadDenyLiterals is the third
// holder of that spelling and was not swept.
//
// FOUND verifying ranger-base-r68d8 at ranger-base-zkunj. Not an escape from
// that close, whose subject is the claude names: this is the same class one
// function over, and it is filed on its own.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQATheCredentialReadDenyFollowsTheCliHomeVarsTheRestOfPosseReads asserts
// the deny a claude-launched session renders covers the auth file each
// sibling runtime ACTUALLY reads, whatever moved its home.
//
// stateDirs is nil: a claude-launched session owns neither `~/.codex` nor
// `~/.grok`, so both siblings are denied (hw18's rule). goos is "darwin" as a
// parameter for the reason credentialReadDenyLiterals states.
//
// Each row carries its CONTROL: the unmoved home, where the hardcoded literal
// and the resolver agree and the wall is correct. A row whose moved arm
// equals its control has measured nothing and says so rather than passing.
func TestQATheCredentialReadDenyFollowsTheCliHomeVarsTheRestOfPosseReads(t *testing.T) {
	t.Skip("ranger-base-x5cbz (found verifying ranger-base-r68d8 at ranger-base-zkunj): credentialReadDenyLiterals (internal/posse/seatbelt.go) names ~/.codex/auth.json and ~/.grok/auth.json as HOME-shaped literals, so CODEX_HOME / GROK_HOME move the file the runtime reads out from under the deny. PARKED because the fix is production code and belongs to the lane that takes x5cbz: un-skip with it. Shown able to fail (both rows, both CONTROLs intact) and able to pass (go test -overlay of the 6-line candidate fix on the bead) on 2026-09-05.")

	home := t.TempDir()
	t.Setenv("HOME", home)

	for _, tc := range []struct {
		name, envVar string
		resolve      func(string) string
	}{
		{"codex: $CODEX_HOME moves the store the deny names", "CODEX_HOME", codexHomeIn},
		{"grok: $GROK_HOME moves the store the deny names", "GROK_HOME", grokHomeIn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// CONTROL: nothing moved. The wall is right, and this is the
			// arm that proves the assertion below can pass at all.
			unsetenvForTest(t, tc.envVar)
			atHome := absResolve(filepath.Join(tc.resolve(home), "auth.json"))
			if !walls(mapAbs(credentialReadDenyLiterals("darwin", nil)), atHome) {
				t.Fatalf("CONTROL: with %s unset the deny %v does not even name %s — this row cannot measure the move", tc.envVar, credentialReadDenyLiterals("darwin", nil), atHome)
			}

			// The box an operator actually has: the CLI's home is moved,
			// and a real auth.json is sitting in it.
			moved := t.TempDir()
			t.Setenv(tc.envVar, moved)
			authFile := absResolve(filepath.Join(tc.resolve(home), "auth.json"))
			if authFile == atHome {
				t.Fatalf("CONTROL: %s=%s did not move %s's answer off %s — this row measures nothing", tc.envVar, moved, tc.envVar, atHome)
			}
			if err := os.WriteFile(authFile, []byte(`{"token":"not-a-real-one"}`), 0o600); err != nil {
				t.Fatal(err)
			}

			deny := mapAbs(credentialReadDenyLiterals("darwin", nil))
			if !walls(deny, authFile) {
				t.Errorf("the read-deny %v does not name %s, the file the runtime reads once %s moves its home — a caged session reads that credential, and the wall is over a path that is not there (the ranger-base-x5f6p defect, one runtime over). posse's own rule for this home is %s (interstitial.go); credentialReadDenyLiterals (seatbelt.go) does not ask it",
					deny, authFile, tc.envVar, tc.envVar)
			}
			for _, d := range deny {
				if strings.HasSuffix(d, "auth.json") && d != authFile && underDir(home, d) {
					t.Logf("and it still names %s, which %s moved the runtime off", d, tc.envVar)
				}
			}
		})
	}
}

// mapAbs applies the call site's own absResolve to every literal, so the
// comparison is the one the sandbox makes (seatbelt.go's note on underDir).
func mapAbs(ps []string) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, absResolve(p))
	}
	return out
}
