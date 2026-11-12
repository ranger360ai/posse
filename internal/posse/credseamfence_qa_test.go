package posse

// TestMain's CLI-home fence (ranger-base-u3kft, then ranger-base-vxir7).
//
// WHAT HAPPENED. TestMain replaces $HOME with a tempdir and takes the CLI
// home overrides out of force so that no locator in this binary reads the
// OPERATOR's live store. It used to NAME them — CLAUDE_CONFIG_DIR,
// CODEX_HOME, GROK_HOME — and the credential locator honours a fourth,
// CLAUDE_SECURESTORAGE_CONFIG_DIR, which credentialDirNamed reads FIRST. A
// seat whose launcher exports it ran this package's tests against the
// directory it names. MEASURED 2026-11-11 on this box:
// TestOneEnvelopeReadsIdenticallyThroughBothStores (credseam_test.go) and
// TestTheReportRendersStructuralAbsenceAsSomethingToDo (refresh_test.go)
// red, and CI — which exports nothing of the sort — could not see either.
//
// WHAT HOLDS NOW. TestMain no longer spells the list: FenceCLIHomes
// (clihomefence.go) unsets every name in CLIHomeOverrideVars, which is the
// resolvers' own enumeration, and then ASKS the resolvers where they land,
// stopping the binary on an answer outside the temp HOME. So a fifth name
// added to credentialDirVars is fenced by construction, and a variable no
// list knows about stops the binary on any box that exports it.
//
// WHY THE PINS BELOW ARE NOT CI-BLIND. The old source arms read TestMain's
// literals because the runtime arm is green wherever the variable is
// absent. These pins put every honoured variable INTO force themselves
// (t.Setenv, a scratch dir outside the fence), so they fire on every box:
// the check must see it, and the fence must take it out again.

import (
	"os"
	"strings"
	"testing"
)

// ─── arm 1: the running binary ───────────────────────────────────────────────

// What every test in this binary is entitled to assume: no honoured variable
// is in force at all — unset, not "" — and every resolver answers inside the
// temp HOME.
func TestQATheCredentialLocatorIsFencedInsideTheTempHome(t *testing.T) {
	t.Parallel()

	home := os.Getenv("HOME")
	if home == "" {
		t.Fatal("no $HOME — TestMain always sets one")
	}

	for _, key := range CLIHomeOverrideVars() {
		if _, set := os.LookupEnv(key); set {
			t.Errorf("%s is SET in the test binary — TestMain's fence must os.Unsetenv every CLI home override (an empty CLAUDE_SECURESTORAGE_CONFIG_DIR is itself in force)", key)
		}
	}
	if esc := CLIHomeFenceEscapes(home); len(esc) > 0 {
		t.Errorf("%s answer outside the temp HOME — this binary resolves an operator store", strings.Join(esc, ", "))
	}

	dir, named, err := credentialDirNamed()
	if err != nil {
		t.Fatalf("credentialDirNamed: %v", err)
	}
	if named {
		t.Errorf("credentialDirNamed reports the directory %q as NAMED — some variable in the environment is moving this binary's credential reads off the temp HOME", dir)
	}
	// The seam the two reds above actually failed on: keychainItem suffixes
	// the item with a sha256 of the directory when it is NAMED, so a fenced
	// binary must produce the bare service name.
	if got := keychainItem(); got != KeychainService {
		t.Errorf("keychainItem() = %q, want the unsuffixed %q — a named directory is being hashed into the item, and a test that derives the expected name after unsetting the variable compares two different stores", got, KeychainService)
	}
}

// ─── arm 2: every honoured variable, put into force on purpose ───────────────

// For each name the resolvers honour: pointed at a scratch dir outside the
// fence, the outcome check SEES it (else the check is blind, or the name is
// no longer honoured and should leave the list); then the fence TestMain
// runs takes it out of force and the resolvers answer inside the temp HOME
// again. The "" arm is the third state: the fence must UNSET it, not leave
// it present.
//
// Not parallel: t.Setenv, and it moves process env the parallel tests read.
// They are paused while a sequential top-level test runs.
func TestQAEveryHonouredVariableIsSeenAndFenced(t *testing.T) {
	home := os.Getenv("HOME")
	vars := CLIHomeOverrideVars()
	if len(vars) < 4 {
		t.Fatalf("CLIHomeOverrideVars() = %v — fewer names than the four this fence was written against; the derivation lost one", vars)
	}
	for _, name := range vars {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, t.TempDir())
			if esc := CLIHomeFenceEscapes(home); len(esc) == 0 {
				t.Errorf("%s points outside the temp HOME and no resolver answered outside it — either the outcome check is blind or no resolver honours %s any more", name, name)
			}
			if err := FenceCLIHomes(home, nil); err != nil {
				t.Errorf("FenceCLIHomes did not take %s out of force: %v", name, err)
			}
			if _, set := os.LookupEnv(name); set {
				t.Errorf("%s survived FenceCLIHomes", name)
			}

			t.Setenv(name, "")
			if err := FenceCLIHomes(home, nil); err != nil {
				t.Errorf("FenceCLIHomes with %s present-but-empty: %v", name, err)
			}
			if _, set := os.LookupEnv(name); set {
				t.Errorf("%s present-but-empty survived FenceCLIHomes — it must be UNSET, not cleared", name)
			}
		})
	}
}

// The list is the resolvers' own, not a third copy: credentialDirVars plus
// the two runtime-home consts, and nothing else.
func TestQACLIHomeOverrideVarsIsDerived(t *testing.T) {
	t.Parallel()

	want := append(append([]string{}, credentialDirVars...), CodexHomeVar, GrokHomeVar)
	got := CLIHomeOverrideVars()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("CLIHomeOverrideVars() = %v, want %v", got, want)
	}
	got[0] = "MUTATED"
	if CLIHomeOverrideVars()[0] == "MUTATED" {
		t.Error("CLIHomeOverrideVars hands out the resolvers' own backing array — a caller can edit credentialDirVars through it")
	}
}
