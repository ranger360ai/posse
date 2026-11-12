package main

// The CLI-home half of TestMain's fence (ranger-base-6sddu, then
// ranger-base-vxir7).
//
// configdirfence_test.go's TestMain keeps the operator's HOME on purpose and
// points every runtime home into a tempdir. The credential locator
// (credentialDirNamed, internal/posse/credential.go) reads
// CLAUDE_SECURESTORAGE_CONFIG_DIR AHEAD of CLAUDE_CONFIG_DIR, so on a seat
// whose launcher exported it a child built from os.Environ() resolved the
// operator's live credential store. MEASURED 2026-11-11 with that variable on
// a dummy store under inotify: TestRuntimesListsAYamlV2Profile and
// TestRuntimeProbeIsWiredAndRefusesWithoutHerdr opened it once each.
//
// The fence is posse.FenceCLIHomes now — the resolvers' own list, unset, and
// the resolvers ASKED afterwards — so this pin does not read TestMain's
// source. It puts every honoured variable into force itself, which is what
// keeps it from being green on CI whatever TestMain says: the outcome check
// must see the variable, and the fence must take it out again.
// internal/treepins/credentialfence_test.go is the same pin.

import (
	"os"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestTestMainFencesEveryCLIHomeOverride(t *testing.T) {
	root := cliHomeFenceRoot
	if root == "" {
		t.Fatal("cliHomeFenceRoot is empty — TestMain did not fence this binary")
	}
	if esc := posse.CLIHomeFenceEscapes(root); len(esc) > 0 {
		t.Fatalf("%s answer outside the fence in the running binary", strings.Join(esc, ", "))
	}
	for _, name := range posse.CLIHomeOverrideVars() {
		for _, v := range []string{t.TempDir(), ""} {
			t.Setenv(name, v)
			if esc := posse.CLIHomeFenceEscapes(root); v != "" && len(esc) == 0 {
				t.Errorf("%s points outside the fence and no resolver answered outside it — the outcome check is blind, or nothing honours %s any more", name, name)
			}
			if err := posse.FenceCLIHomes(root, cliHomeFenceRows(root)); err != nil {
				t.Errorf("TestMain's fence did not hold against %s (empty=%v): %v", name, v == "", err)
			}
			if _, set := os.LookupEnv(name); set && cliHomeFenceRows(root)[name] == "" {
				t.Errorf("%s survived the fence (empty=%v) — it must be UNSET, never cleared", name, v == "")
			}
		}
	}
}
