package main

// The credential half of TestMain's fence (ranger-base-6sddu).
//
// configdirfence_test.go's TestMain sets CLAUDE_CONFIG_DIR to a tempdir and
// leaves HOME as the operator's on purpose. The credential locator
// (credentialDirNamed, internal/posse/credential.go) reads a variable AHEAD
// of CLAUDE_CONFIG_DIR — CLAUDE_SECURESTORAGE_CONFIG_DIR — so on a seat whose
// launcher exports it, a child built from os.Environ() resolved the
// operator's live credential store. MEASURED 2026-11-11 with that variable
// on a dummy store under inotify: TestRuntimesListsAYamlV2Profile and
// TestRuntimeProbeIsWiredAndRefusesWithoutHerdr opened it once each, because
// `posse runtimes` with no catalog snapshot reads the model catalog, and the
// catalog lister takes its bearer token from that store and GETs the
// production models endpoint with it.
//
// Two arms, because the runtime one is blind on CI: ci.yml exports no such
// variable, so the binary's environment is clean there whatever TestMain
// says. The SOURCE arm reads credential.go's own list of the variables the
// locator honours (credentialDirVars) and requires TestMain to answer every
// one of them, so a third name added there reds on every box, before any
// seat exports it.

import "testing"

func TestTestMainFencesEveryCredentialDirVar(t *testing.T) {
	checkCredentialFence(t, "../../internal/posse/credential.go", "configdirfence_test.go")
}
