package posse

// Pin for ranger-base-x5cbz: the seatbelt's credential read-deny named
// `~/.codex/auth.json` and `~/.grok/auth.json` as HOME-shaped literals, but
// `$CODEX_HOME` / `$GROK_HOME` move those CLIs' homes — so on a box that
// exports either one the wall was over a file that is not there and the file
// the runtime actually reads was not walled.
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
//
// FIXED and UN-SKIPPED at ranger-base-3p8hx (spec on ranger-base-b52r3):
// credentialReadDenyLiterals now asks codexHomeIn / grokHomeIn and denies
// BOTH spellings per sibling. This file was parked here written and shown
// able to fail — both rows RED on the moved arm with both CONTROLs green at
// 48a3bf3 — because the fix was production code belonging to another lane.

import (
	"os"
	"path/filepath"
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
			// Decision 1 (ranger-base-b52r3): the HOME spelling stays
			// denied while the variable names another. Whatever the CLI
			// wrote before the move is still sitting there — ADR 0019 D2's
			// recurring unowned byproduct — and a deny over a path that is
			// not there costs nothing, the read being ENOENT either way.
			// This was a t.Logf while the file was parked; it is the
			// assertion that stops a "follow the resolver" fix from
			// dropping the half it replaces.
			if !walls(deny, atHome) {
				t.Errorf("the read-deny %v no longer names %s once %s moves the home — the file the CLI wrote BEFORE the move is still sitting there (ADR 0019 D2), and following the resolver must ADD a spelling rather than swap one for another", deny, atHome, tc.envVar)
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

// Decision 3 (ranger-base-b52r3): the NO-HOME arm names nothing rather than
// naming something in the session's working directory.
//
// THREE shapes reach that, and each was found one rung after the last:
// ExpandTilde with no HOME hands its `~/…` literal straight back;
// codexHomeIn("") is "", whose filepath.Join with the file name is the
// RELATIVE path `auth.json`; and codexHomeIn returns $CODEX_HOME VERBATIM,
// so a relative value in that variable is `mycodex/auth.json`. The deny loop
// absResolve's whatever it is handed, so any of the three lands under the
// session's cwd — a wall over an ordinary file the session may legitimately
// need, and no wall at all over a credential. One predicate closes all
// three, credentialDenyable, and it is the `add` credentialFileCandidates
// uses too; this is the pin that says so.
//
// The first two are reachable only where the environment is scrubbed
// (`env -i posse …`, a unit file with no HOME), which is ExpandTilde's own
// note — the arm is here because the cost of being wrong is a wall in the
// wrong place, not because the box is expected to hit it. The third needs no
// scrubbed environment at all, only a box that exports a relative
// $CODEX_HOME, which is why it gets its own arm below rather than riding on
// the empty-HOME one (ranger-base-o05yg; the guard that used to stand here
// asked whether the home was EMPTY, which a relative one is not).
func TestQACredentialReadDenyNamesNothingUnderTheCwdWithNoHome(t *testing.T) {
	unsetenvForTest(t, "HOME")
	unsetenvForTest(t, "CODEX_HOME")
	unsetenvForTest(t, "GROK_HOME")
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	nothingUnderCwd := func(when string) {
		t.Helper()
		for _, p := range credentialReadDenyLiterals("darwin", nil) {
			if !filepath.IsAbs(p) {
				t.Errorf("%s the deny names the RELATIVE path %q — the call site absResolve's it, so the wall lands on a file in whatever directory the session runs in", when, p)
				continue
			}
			if underDir(cwd, absResolve(p)) {
				t.Errorf("%s the deny names %s, inside the session's own working directory %s — a credential wall must never land there", when, p, cwd)
			}
		}
	}
	nothingUnderCwd("with no HOME")

	// The same hazard through the VARIABLE rather than through the empty
	// home, which is the rung the empty-home arm above cannot reach: a
	// relative $CODEX_HOME is not empty, so every guard that asks whether
	// there is a home to name answers yes and hands filepath.Join a
	// relative one (ranger-base-o05yg — measured before the fix as the
	// literal "mycodex/auth.json", resolving inside this very directory).
	// Both variables, because grokHomeIn is the same function.
	t.Setenv("CODEX_HOME", "mycodex")
	t.Setenv("GROK_HOME", "mygrok")
	nothingUnderCwd("with a RELATIVE $CODEX_HOME and $GROK_HOME")

	// And the arm that keeps this from passing vacuously: with no home but a
	// variable that names one, there IS a store to wall, and it is walled.
	// It runs last on purpose — it puts both variables back to ABSOLUTE
	// values, which is the direction that must still be denied, so the arm
	// above cannot be satisfied by a predicate that simply stopped naming
	// what the variables point at.
	moved := t.TempDir()
	t.Setenv("CODEX_HOME", moved)
	t.Setenv("GROK_HOME", t.TempDir())
	want := absResolve(filepath.Join(moved, "auth.json"))
	if deny := mapAbs(credentialReadDenyLiterals("darwin", nil)); !walls(deny, want) {
		t.Errorf("CONTROL: with $CODEX_HOME=%s and no HOME the deny %v does not name %s — the guard above has stopped naming the file it should, not just the one it should not", moved, deny, want)
	}
}
