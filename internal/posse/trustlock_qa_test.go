package posse

// QA pins for the config lock (ranger-base-5qnt, sibling of rangerhq-w4uf).
//
// trust_qa_test.go's pin is the reported shape at its smallest: two
// goroutines, two open file descriptions, one lost update. These are the
// two claims that pin does not reach —
//
//   - the merge survives CONTENTION, not just a single collision: N seeds
//     against one config leave N entries and the operator's own state, which
//     is what "last rename wins" fails at loudly.
//   - the merge survives a real PROCESS BOUNDARY. That is the reported
//     hazard and the only one the launcher lock does not already cover: a
//     hand-run `posse new` takes no launcher lock, and a fleet RHQ_HOME and
//     a scratch one hold two different launcher locks over one
//     ~/.claude.json (ADR 0011 §1 is per RHQ_HOME; this lock is per config
//     file). Two file descriptions in one process is a claim about flock;
//     two processes is the fact.
//
// Self-contained on purpose: they must survive whatever the next persona
// does to trust_test.go.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// qaSeedDir is the session dir the seeder at index i claims. Named rather
// than derived inline so parent and child cannot drift apart.
func qaSeedDir(root string, i int) string {
	return filepath.Join(root, "dir-"+strconv.Itoa(i))
}

// qaSeedRange creates and seeds [lo, lo+n) under root, the way a run of
// launches would. Shared by the in-process and cross-process pins.
func qaSeedRange(t *testing.T, cfg, root string, lo, n int) {
	t.Helper()
	rt := claudeRuntime(t)
	for i := lo; i < lo+n; i++ {
		d := qaSeedDir(root, i)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := SeedClaudeTrust(cfg, rt, d); err != nil {
			t.Fatal(err)
		}
	}
}

// qaAssertAllTrusted is the whole acceptance criterion: every dir landed,
// and the operator's pre-existing state is still there to land beside.
func qaAssertAllTrusted(t *testing.T, cfg, root string, n int, operatorDir string) {
	t.Helper()
	state := readConfig(t, cfg)
	var missing []string
	for i := 0; i < n; i++ {
		if !ClaudeTrusted(state, qaSeedDir(root, i)) {
			missing = append(missing, strconv.Itoa(i))
		}
	}
	if len(missing) > 0 {
		t.Errorf("lost update: %d/%d dirs missing from %s (indexes %s) — a merge that drops a concurrent launch's key is the trust dialog, one launcher over",
			len(missing), n, cfg, strings.Join(missing, ","))
	}
	if operatorDir != "" && !ClaudeTrusted(state, operatorDir) {
		t.Errorf("the operator's own trusted project was dropped by the merge — this file is their whole claude state, not a posse artifact")
	}
	if state["theme"] != "dark" {
		t.Errorf("operator state outside projects[] did not survive the merge: theme=%v", state["theme"])
	}
}

// qaOperatorConfig writes the config the operator already had: a theme and
// a project they answered the dialog for by hand, months ago.
func qaOperatorConfig(t *testing.T, cfg, operatorDir string) {
	t.Helper()
	writeConfig(t, cfg, map[string]any{
		"theme": "dark",
		"projects": map[string]any{
			ClaudeTrustKey(operatorDir): map[string]any{"hasTrustDialogAccepted": true},
		},
	})
}

// qaSeederEnv is the parent environment plus extra, cloned so each child
// gets its own backing array. append(env, extra...) aliases whenever env
// has spare cap — the grow-loop that built it almost always does — and the
// last child's RHQ_QA_TRUST_LO then overwrites the others. Every seeder
// writes the same slice; qaAssertAllTrusted reports a 60/80 lost-update
// that is the test, not the lock. gates_qa_test.go already clones this way.
func qaSeederEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RHQ_FAKE_HERDR=") || strings.HasPrefix(kv, "RHQ_QA_TRUST_") {
			continue
		}
		env = append(env, kv)
	}
	return append(append([]string(nil), env...), extra...)
}

// Contention, not a single collision. N=8 measured 6–7/8 missing before the
// lock landed (ranger-base-5qnt); every one of those is a session that opens
// on the modal.
func TestQASeedTrustManyConcurrentLaunchesKeepEveryDir(t *testing.T) {
	const n = 8
	root := t.TempDir()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	operatorDir := t.TempDir()
	qaOperatorConfig(t, cfg, operatorDir)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			qaSeedRange(t, cfg, root, i, 1)
		}(i)
	}
	wg.Wait()

	qaAssertAllTrusted(t, cfg, root, n, operatorDir)
}

// The child half of TestQASeedTrustHoldsAcrossProcesses: seed its own slice
// of the dirs into the config it is handed, as a separate process holding no
// launcher lock — a hand-run `posse new` while a pass is mid-launch.
func TestQASeedTrustChildSeeder(t *testing.T) {
	cfg := os.Getenv("RHQ_QA_TRUST_CFG")
	if cfg == "" {
		t.Skip("child of TestQASeedTrustHoldsAcrossProcesses")
	}
	lo, err := strconv.Atoi(os.Getenv("RHQ_QA_TRUST_LO"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(os.Getenv("RHQ_QA_TRUST_N"))
	if err != nil {
		t.Fatal(err)
	}
	qaSeedRange(t, cfg, os.Getenv("RHQ_QA_TRUST_ROOT"), lo, n)
}

// Four real processes over one config, twenty dirs each. Unlocked, this is
// the reported bug at scale: whichever rename lands last is the only slice
// that survives. Locked, all eighty entries and the operator's own state are
// in the file when the last child exits.
func TestQASeedTrustHoldsAcrossProcesses(t *testing.T) {
	const children, each = 4, 20
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	operatorDir := t.TempDir()
	qaOperatorConfig(t, cfg, operatorDir)

	var wg sync.WaitGroup
	out := make([]string, children)
	fail := make([]error, children)
	for c := 0; c < children; c++ {
		cmd := exec.Command(exe, "-test.run=TestQASeedTrustChildSeeder$", "-test.v")
		cmd.Env = qaSeederEnv(
			"RHQ_QA_TRUST_CFG="+cfg,
			"RHQ_QA_TRUST_ROOT="+root,
			"RHQ_QA_TRUST_LO="+strconv.Itoa(c*each),
			"RHQ_QA_TRUST_N="+strconv.Itoa(each),
		)
		wg.Add(1)
		go func(c int, cmd *exec.Cmd) {
			defer wg.Done()
			b, err := cmd.CombinedOutput()
			out[c], fail[c] = string(b), err
		}(c, cmd)
	}
	wg.Wait()
	for c := range fail {
		if fail[c] != nil {
			t.Fatalf("seeder %d failed (%v):\n%s", c, fail[c], out[c])
		}
	}
	for i := 0; i < children*each; i++ {
		if _, err := os.Stat(qaSeedDir(root, i)); err != nil {
			t.Fatalf("seeder never created %s — the child did not run its slice (env aliasing, not a lost update): %v", qaSeedDir(root, i), err)
		}
	}

	qaAssertAllTrusted(t, cfg, root, children*each, operatorDir)

	// The lock is a sidecar beside the config and never the config itself:
	// the write ends in a rename, and a renamed-over inode is a lock two
	// processes can hold at once.
	if _, err := os.Stat(claudeConfigLockFile(cfg)); err != nil {
		t.Errorf("no lock sidecar beside the config after %d seeding processes: %v", children, err)
	}
}

// A live wedged holder must refuse the next launch, not hang it. The wait
// is LOCK_NB + sleep so a second Open+Flock in this process times out
// instead of blocking forever (flock is per OFD; nested SeedClaudeTrust
// would otherwise wait on itself until the test deadline).
func TestQASeedTrustLockFailsLoudlyWhenHeld(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	unlock, err := lockClaudeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	start := time.Now()
	_, err = SeedClaudeTrust(cfg, claudeRuntime(t), t.TempDir())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a launch that cannot take the lock must refuse, not merge unserialized")
	}
	if !strings.Contains(err.Error(), "held by another posse launch") {
		t.Errorf("refusal must name the wedged holder: %v", err)
	}
	if elapsed < claudeConfigLockWait {
		t.Errorf("waited %s, want at least the bound %s", elapsed, claudeConfigLockWait)
	}
}

// The child half of TestQASeedTrustLockFreesOnProcessDeath.
func TestQASeedTrustChildHoldLock(t *testing.T) {
	cfg := os.Getenv("RHQ_QA_TRUST_HOLD")
	if cfg == "" {
		t.Skip("child of TestQASeedTrustLockFreesOnProcessDeath")
	}
	unlock, err := lockClaudeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	os.Stdout.WriteString("HELD " + strconv.Itoa(os.Getpid()) + "\n")
	time.Sleep(2 * time.Minute)
}

// flock releases on process death — ADR 0011 §1's argument over a pidfile,
// applied to the config sidecar. A killed `posse new` must not strand the
// next launch behind a 10s refusal.
func TestQASeedTrustLockFreesOnProcessDeath(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	child := exec.Command(exe, "-test.run=TestQASeedTrustChildHoldLock$", "-test.v")
	child.Env = qaSeederEnv("RHQ_QA_TRUST_HOLD=" + cfg)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	defer func() {
		if !reaped && child.Process != nil {
			child.Process.Kill()
			child.Wait()
		}
	}()

	buf, seen := make([]byte, 4096), ""
	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(seen, "HELD ") && time.Now().Before(deadline) {
		n, err := stdout.Read(buf)
		seen += string(buf[:n])
		if err != nil {
			break
		}
	}
	if !strings.Contains(seen, "HELD ") {
		t.Fatalf("the holding process never took the lock:\n%s", seen)
	}

	child.Process.Kill()
	if err := child.Wait(); err != nil && !strings.Contains(err.Error(), "killed") {
		t.Fatalf("holder exit: %v", err)
	}
	reaped = true

	start := time.Now()
	if _, err := SeedClaudeTrust(cfg, claudeRuntime(t), t.TempDir()); err != nil {
		t.Fatalf("seed after holder death: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= claudeConfigLockWait {
		t.Errorf("seed waited %s after the holder died — flock did not release on process death", elapsed)
	}
}
