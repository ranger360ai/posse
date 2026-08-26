package rhq

// QA pin for ranger-base-5qnt (found verifying ranger-base-s83 / rangerhq-w4uf).
// SeedClaudeTrust is a read-amend-rename of the operator's ~/.claude.json.
// Two concurrent calls on different session dirs are a lost-update: last
// rename wins with only the dir it read, so the sibling launch's
// hasTrustDialogAccepted is missing and that session opens on the trust
// modal. Dispatch serializes via lockLaunches; posse new does not.
//
// FIXED by the lock in trust.go (lockClaudeConfig): the read, the trusted
// check and the rename run under an flock on a sidecar beside the config,
// so the merge is atomic against a sibling launch and against a second
// process. Unskipped when that landed.

import (
	"sync"
	"testing"
)

func TestQASeedTrustConcurrentLaunchesKeepBothDirs(t *testing.T) {
	cfg := t.TempDir() + "/.claude.json"
	rt := claudeRuntime(t)
	dirA, dirB := t.TempDir(), t.TempDir()

	var wg sync.WaitGroup
	errc := make(chan error, 2)
	for _, d := range []string{dirA, dirB} {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			_, err := SeedClaudeTrust(cfg, rt, d)
			errc <- err
		}(d)
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		if err != nil {
			t.Fatal(err)
		}
	}
	state := readConfig(t, cfg)
	missing := 0
	for _, d := range []string{dirA, dirB} {
		if !ClaudeTrusted(state, d) {
			missing++
			t.Errorf("lost update: %s not trusted after concurrent seed", d)
		}
	}
	if missing > 0 {
		t.Fatalf("%d/2 dirs missing from the config after concurrent seeds — last rename dropped a sibling launch's trust key", missing)
	}
}

// Same dir, many launchers: the lock serializes them onto one key, and the
// second-and-later seeds are the already-trusted no-op. A merge that
// duplicated or dropped the entry would be a different hole than 5qnt.
func TestQASeedTrustSameDirConcurrentIsIdempotent(t *testing.T) {
	cfg := t.TempDir() + "/.claude.json"
	rt := claudeRuntime(t)
	dir := t.TempDir()

	const n = 8
	var wg sync.WaitGroup
	errc := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := SeedClaudeTrust(cfg, rt, dir)
			errc <- err
		}()
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		if err != nil {
			t.Fatal(err)
		}
	}
	state := readConfig(t, cfg)
	if !ClaudeTrusted(state, dir) {
		t.Fatal("dir not trusted after concurrent same-dir seeds")
	}
	projects, _ := state["projects"].(map[string]any)
	if len(projects) != 1 {
		t.Errorf("want one project entry, got %d: %v", len(projects), projects)
	}
}
