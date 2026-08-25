package rhq

// QA pin for ranger-base-5qnt (found verifying ranger-base-s83 / rangerhq-w4uf).
// SeedClaudeTrust is a read-amend-rename of the operator's ~/.claude.json.
// Two concurrent calls on different session dirs are a lost-update: last
// rename wins with only the dir it read, so the sibling launch's
// hasTrustDialogAccepted is missing and that session opens on the trust
// modal. Dispatch serializes via lockLaunches; posse new does not.

import (
	"sync"
	"testing"
)

func TestQASeedTrustConcurrentLaunchesKeepBothDirs(t *testing.T) {
	t.Skip("ranger-base-5qnt: concurrent SeedClaudeTrust is a lost-update; last rename drops the sibling dir")
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
