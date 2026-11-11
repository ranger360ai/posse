package treepins

// The credential half of TestMain's fence (ranger-base-6sddu), the same pin
// cmd/posse/credentialfence_test.go carries for that package.
//
// configdirfence_test.go's TestMain sets CLAUDE_CONFIG_DIR to a tempdir and
// keeps the operator's HOME. The credential locator (credentialDirNamed,
// internal/posse/credential.go) reads CLAUDE_SECURESTORAGE_CONFIG_DIR AHEAD
// of CLAUDE_CONFIG_DIR, so on a seat that exports it a child built from
// os.Environ() resolves the operator's live credential store. Here that was
// measured clean (ranger-base-6sddu's filing: 218 tests from the 24 files
// that spawn such a child, 0 opens), so the row is for consistency with
// cmd/posse — and a test added tomorrow inherits it.
//
// The source arm derives the class from credential.go's credentialDirVars,
// because the runtime arm is green on CI whatever TestMain says. This reads
// two named files, not an enumeration, so it is not a tree-wide pin and
// needs no Makefile door.

import "testing"

func TestTestMainFencesEveryCredentialDirVar(t *testing.T) {
	checkCredentialFence(t, "internal/posse/credential.go", "internal/treepins/configdirfence_test.go")
}
