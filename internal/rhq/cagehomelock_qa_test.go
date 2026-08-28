package rhq

// QA pins for the cage HOME config lock (ranger-base-5cv7, sibling of
// ranger-base-5qnt / trustlock_qa_test.go). SeedCageHome has the same
// read-amend-write shape as SeedClaudeTrust — reads <cage home>/.claude.json,
// merges keys, writes it back — and until this bead it wrote with a plain
// os.WriteFile rather than trust.go's temp-file+rename, so a losing
// concurrent write there did not just drop a sibling's entry, it could
// leave the operator-visible cage HOME file truncated.
//
// Narrower exposure than 5qnt (a caged claude persona under one RHQ_HOME),
// but the mechanism and the pin are the same: N concurrent seeders in one
// process, and real seeders across a process boundary, must all land in
// the merged file with none lost.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// qaCageSeedRange seeds [lo, lo+n) of qaSeedDir(root, i) into the persona
// ag's cage HOME under app a, the way overlapping launches of the same
// caged persona would.
func qaCageSeedRange(t *testing.T, a *App, ag *AgentFile, rt *Runtime, root string, lo, n int) {
	t.Helper()
	for i := lo; i < lo+n; i++ {
		d := qaSeedDir(root, i)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := a.SeedCageHome(ag, rt, d); err != nil {
			t.Fatal(err)
		}
	}
}

// qaAssertCageAllTrusted is TestSeedCageHome's acceptance criterion at
// scale: every seeded dir landed as a trusted project, and the wizard-skip
// keys plus whatever the runtime had already written (userID here, standing
// in for claude's own account state) are still there beside them.
//
// Checks the literal dir string as the projects[] key — SeedCageHome's
// claudeSeedProject call (unlike SeedClaudeTrust's) keys on the raw dir,
// not ClaudeTrustKey(dir), so going through ClaudeTrusted here would
// symlink-resolve a key SeedCageHome never wrote (macOS /var → /private/var
// under t.TempDir(), measured).
func qaAssertCageAllTrusted(t *testing.T, cfg, root string, n int) {
	t.Helper()
	state := readConfig(t, cfg)
	projects, _ := state["projects"].(map[string]any)
	var missing []string
	for i := 0; i < n; i++ {
		proj, _ := projects[qaSeedDir(root, i)].(map[string]any)
		if proj["hasTrustDialogAccepted"] != true {
			missing = append(missing, strconv.Itoa(i))
		}
	}
	if len(missing) > 0 {
		t.Errorf("lost update: %d/%d dirs missing from %s (indexes %s) — a merge that drops a concurrent launch's key is the onboarding wizard, one caged launch over",
			len(missing), n, cfg, strings.Join(missing, ","))
	}
	if state["userID"] != "kept" {
		t.Errorf("pre-existing cage HOME state did not survive the merge: userID=%v", state["userID"])
	}
	if state["hasCompletedOnboarding"] != true || state["theme"] != "dark" || state["autoUpdates"] != false {
		t.Errorf("wizard-skip keys did not survive the merge: %v", state)
	}
}

// Contention, not a single collision — the same shape
// TestQASeedTrustManyConcurrentLaunchesKeepEveryDir pins for the host config,
// applied to a cage HOME's.
func TestQASeedCageHomeManyConcurrentLaunchesKeepEveryDir(t *testing.T) {
	const n = 8
	a := cageApp(t)
	ag := cageAgent(t, a, "")
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()

	// A prior launch already seeded this cage HOME and claude wrote its own
	// account state there since, the way TestSeedCageHome's "kept" case
	// does.
	home, err := a.SeedCageHome(ag, rt, qaSeedDir(root, -1))
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".claude.json")
	state := readConfig(t, cfg)
	state["userID"] = "kept"
	writeConfig(t, cfg, state)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			qaCageSeedRange(t, a, ag, rt, root, i, 1)
		}(i)
	}
	wg.Wait()

	qaAssertCageAllTrusted(t, cfg, root, n)
}

// The child half of TestQASeedCageHomeHoldsAcrossProcesses: seed its own
// slice of the dirs into the cage HOME it is handed, as a separate process
// — a hand-run `posse new` of the same caged persona overlapping a
// dispatch pass, which is the one shape dispatch's launcher lock does not
// already serialize (the bead's own SCOPE OF EXPOSURE).
func TestQASeedCageHomeChildSeeder(t *testing.T) {
	home := os.Getenv("RHQ_QA_CAGEHOME_HOME")
	if home == "" {
		t.Skip("child of TestQASeedCageHomeHoldsAcrossProcesses")
	}
	a := &App{
		Home:      home,
		StateDir:  os.Getenv("RHQ_QA_CAGEHOME_STATE"),
		AgentsDir: os.Getenv("RHQ_QA_CAGEHOME_AGENTS"),
	}
	ag, err := a.LoadAgent(os.Getenv("RHQ_QA_CAGEHOME_PERSONA"))
	if err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	lo, err := strconv.Atoi(os.Getenv("RHQ_QA_CAGEHOME_LO"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(os.Getenv("RHQ_QA_CAGEHOME_N"))
	if err != nil {
		t.Fatal(err)
	}
	qaCageSeedRange(t, a, ag, rt, os.Getenv("RHQ_QA_CAGEHOME_ROOT"), lo, n)
}

// Four real processes over one cage HOME's .claude.json, twenty dirs each.
// Unlocked, SeedCageHome's plain os.WriteFile means whichever write lands
// last is the only slice that survives — and can leave a truncated file if
// two renames-free writes interleave. Locked and rename-backed, all eighty
// entries and the pre-existing state are there when the last child exits.
func TestQASeedCageHomeHoldsAcrossProcesses(t *testing.T) {
	const children, each = 4, 20
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := cageApp(t)
	ag := cageAgent(t, a, "")
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()

	home, err := a.SeedCageHome(ag, rt, qaSeedDir(root, -1))
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".claude.json")
	state := readConfig(t, cfg)
	state["userID"] = "kept"
	writeConfig(t, cfg, state)

	var wg sync.WaitGroup
	out := make([]string, children)
	fail := make([]error, children)
	for c := 0; c < children; c++ {
		cmd := exec.Command(exe, "-test.run=TestQASeedCageHomeChildSeeder$", "-test.v")
		cmd.Env = qaSeederEnv(
			"RHQ_QA_CAGEHOME_HOME="+a.Home,
			"RHQ_QA_CAGEHOME_STATE="+a.StateDir,
			"RHQ_QA_CAGEHOME_AGENTS="+a.AgentsDir,
			"RHQ_QA_CAGEHOME_PERSONA="+ag.Name,
			"RHQ_QA_CAGEHOME_ROOT="+root,
			"RHQ_QA_CAGEHOME_LO="+strconv.Itoa(c*each),
			"RHQ_QA_CAGEHOME_N="+strconv.Itoa(each),
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
			t.Fatalf("seeder never claimed %s — the child did not run its slice (env aliasing, not a lost update): %v", qaSeedDir(root, i), err)
		}
	}

	qaAssertCageAllTrusted(t, cfg, root, children*each)

	// Same sidecar SeedClaudeTrust locks, keyed on this cage HOME's config
	// path rather than the operator's — lockClaudeConfig takes cfg
	// unchanged.
	if _, err := os.Stat(claudeConfigLockFile(cfg)); err != nil {
		t.Errorf("no lock sidecar beside the cage HOME config after %d seeding processes: %v", children, err)
	}
}
