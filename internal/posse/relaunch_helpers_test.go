package posse

// Helpers lifted out of relaunch_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"path/filepath"
	"testing"
)

// hermeticGen names a socket path that is not there, which makes
// ServerGen() unknown for the whole test. Two reasons, and both matter:
// the default path is the operator's own live herdr, so a test that let it
// through would read machine state and be red per-box; and an unknown
// generation is the board on which notOurWorkspace's LABEL arm is the only
// fence there is. A test run under a generation that matched would pass
// whatever the label said (the rename arm short-circuits it) and pin
// nothing.
//
// It is HERE rather than beside its first caller because the arms that do
// not name that file cannot compile it: it was arm 2's, and the fence has a
// reader in arm 1 now (relaunchaim_qa_test.go, ranger-base-nc91t). That is
// this file's whole reason for existing (ranger-base-qp1hm).
func hermeticGen(t *testing.T) {
	t.Helper()
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(t.TempDir(), "herdr.sock"))
}

// devSession writes a PID whose gates the wall fully realizes on claude at
// shims, plus an env set, and creates one session from them.
func devSession(t *testing.T, b *HerdrBackend, name string) string {
	t.Helper()
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	os.MkdirAll(b.App.EnvsDir, 0o700)
	os.WriteFile(filepath.Join(b.App.EnvsDir, "test.env"), []byte("FOO=bar\n"), 0o600)
	repo := t.TempDir()
	if err := b.CreateSession(NewSessionOpts{
		Name: name, Agent: "dev", Dir: repo, Envs: []string{"test"}, Tier: "standard",
	}); err != nil {
		t.Fatal(err)
	}
	return repo
}
