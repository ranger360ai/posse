package posse

// Helpers lifted out of relaunch_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"path/filepath"
	"testing"
)

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
