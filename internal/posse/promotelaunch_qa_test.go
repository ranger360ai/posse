package posse

// ADR 0015 §3's launch verify, at the two launches that differ: a dispatched
// session refuses on a constitution that does not match its manifest, an
// interactive one warns DEGRADED and comes up.
//
// The asymmetry is the whole point and is worth stating where it is tested:
// a dispatched launch has nobody watching it, and the fix is one operator
// command (`posse promote`), so fail-closed is cheap. An interactive launch
// IS the operator — refusing to open the session they would fix it from is
// the failure mode, not the control.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// promotedTestHome gives a backend a home that verifies clean, and returns
// the path of a promoted PID a test can corrupt.
func promotedTestHome(t *testing.T, b *HerdrBackend) string {
	t.Helper()
	a := b.App
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	pid := filepath.Join(a.AgentsDir, "ranger.md")
	if err := os.WriteFile(pid, []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	// Pretend it came from a commit: the seeded flag changes nothing about
	// the check, but the state a real fleet is in is "promoted".
	m := &PromoteManifest{Version: promoteManifestVersion, Source: "/somewhere/rhq", SHA: strings.Repeat("a", 40)}
	m.Files, _ = HashPromotedSet(a.Home)
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("fixture does not verify: %s", v.Line())
	}
	return pid
}

func TestQADispatchRefusesAConstitutionThatDoesNotMatchItsManifest(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	pid := promotedTestHome(t, b)
	dir := t.TempDir()

	// Clean: the dispatched launch plans without complaint.
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger", Bead: "x-1"}); err != nil {
		t.Fatalf("a matching constitution refused a dispatch: %v", err)
	}
	// One byte, the way a session that edited a PID leaves it.
	body, _ := os.ReadFile(pid)
	if err := os.WriteFile(pid, append(body, []byte("\ncage: shims\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: dir, Agent: "ranger", Bead: "x-1"})
	if err == nil {
		t.Fatal("dispatch launched on a constitution nobody promoted")
	}
	for _, want := range []string{"agents/ranger.md", "posse promote"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

func TestQAInteractiveLaunchWarnsDegradedAndComesUp(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	pid := promotedTestHome(t, b)
	var warn bytes.Buffer
	b.Warn = &warn

	body, _ := os.ReadFile(pid)
	if err := os.WriteFile(pid, append(body, ' '), 0o644); err != nil {
		t.Fatal(err)
	}
	// No Bead: this is `posse new` / a recipe / a relaunch of a crew session.
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("an interactive launch was refused: %v", err)
	}
	got := warn.String()
	if !strings.Contains(got, "DEGRADED") || !strings.Contains(got, "agents/ranger.md") {
		t.Errorf("no DEGRADED line naming the drift:\n%s", got)
	}
	if !strings.Contains(got, "posse promote") {
		t.Errorf("the warning does not name the one-command fix:\n%s", got)
	}
}

// An env file must not reach this path at all — the launch it would refuse
// is every dispatch after any routine credential edit (ADR 0015 §7).
func TestQAALaunchIsNotRefusedByAnEnvSet(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	promotedTestHome(t, b)
	if err := os.MkdirAll(b.App.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.EnvsDir, "default.env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger", Bead: "x-1"}); err != nil {
		t.Fatalf("an env set at the home refused a dispatch: %v", err)
	}
}

// A home nobody ever promoted (every install predating ADR 0015, and every
// test home) launches exactly as before.
func TestQAUnpromotedHomeLaunchesUnchanged(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.App.ConfigPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger", Bead: "x-1"}); err != nil {
		t.Fatalf("an unpromoted home refused a dispatch: %v", err)
	}
}

// The home-cutover runbook, walked end to end at the live constitution's
// shape (docs/runbooks/home-cutover.md steps 2–6, ADR 0015 verification
// items 1, 2, 3 and 7). It is one test rather than seven because the thing
// the window can get wrong is the ORDER: promote before the env carry comes
// up warning, promote after it does not, and a launch in between must not be
// refused for a reason that is not the constitution.
func TestQAHomeCutoverRehearsal(t *testing.T) {
	wtqaHome(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "ranger-base")
	src := filepath.Join(repo, "rhq")
	home := filepath.Join(root, ".config", "posse")

	pidBody := "---\nname: %s\nlabels: [code]\ndeny:\n  - Bash(git push:*)\n  - Bash(posse promote:*)\n---\n" +
		strings.Repeat("standing prose the fleet reads at every launch.\n", 120)
	write := func(rel, body string, mode os.FileMode) {
		t.Helper()
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	// The live instance as of 2026-08-26: 17 PIDs, 7 recipes, a skills tree,
	// an 8KB config, and the three directories promote must not carry.
	names := []string{"architect", "architect-2", "business-manager", "coordinator",
		"developer", "developer-2", "developer-3", "devops", "devops-2", "product",
		"qa", "qa-2", "ranger", "reviewer", "security", "security-2", "support"}
	for _, n := range names {
		write("agents/"+n+".md", fmt.Sprintf(pidBody, n), 0o644)
	}
	for i := 0; i < 7; i++ {
		write(fmt.Sprintf("recipes/r%d.yaml", i), "purpose: a recipe\n", 0o644)
	}
	write("skills/distributed-systems/SKILL.md", "# distributed systems\n", 0o644)
	write("skills/distributed-systems/references/leases.md", "leases\n", 0o644)
	write("config.yaml", "default_env: default\n"+strings.Repeat("# a commented key\n", 500), 0o644)
	write("envs/default.env", "TOKEN=live-secret\n", 0o600)
	write("envs/container.env", "OAUTH=live-secret\n", 0o600)
	write("state/herdr/meta.json", "{}\n", 0o644)
	write("personas/developer/ORDERS.md", "lessons\n", 0o644)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("rhq/envs/\nrhq/state/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	gitIn("init", "-q", "-b", "main")
	gitIn("add", "-A")
	gitIn("commit", "-qm", "the constitution")

	t.Setenv("RHQ_HOME", home)
	t.Setenv(EnvPersona, "")
	a := NewApp()

	// Step 1's precondition, as the runbook now words it: persona memory is
	// dirty (it is written at every session end) and must not block.
	if err := os.WriteFile(filepath.Join(src, "personas", "developer", "ORDERS.md"), []byte("a lesson learned today\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Step 2 — first promote.
	var out bytes.Buffer
	if err := a.CmdPromote(&out, PromoteOpts{Source: src}); err != nil {
		t.Fatalf("first promote: %v\n%s", err, out.String())
	}
	// Item 1: a real directory, no symlink, manifest naming the promoted SHA.
	if st, err := os.Lstat(home); err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("home is not a real directory: %v %v", st, err)
	}
	m := mustManifest(t, a)
	if m.SHA != strings.TrimSpace(gitIn("rev-parse", "HEAD")) {
		t.Errorf("manifest does not name HEAD: %q", m.SHA)
	}
	// Item 7, promote side: no envs anywhere, and the values never printed.
	if _, err := os.Stat(a.EnvsDir); err == nil {
		t.Error("promote created home/envs")
	}
	if strings.Contains(out.String(), "live-secret") {
		t.Fatal("promote printed an env value")
	}
	// The expected in-window warning, before the carry step.
	if !strings.Contains(out.String(), "default_env") {
		t.Errorf("no dangling default_env warning between promote and the carry:\n%s", out.String())
	}

	// Steps 3 and 4 — the carry, by hand, exactly as the runbook does it.
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"default.env", "container.env"} {
		b, err := os.ReadFile(filepath.Join(src, "envs", n))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.EnvsDir, n), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "personas"), filepath.Join(home, "personas")); err != nil {
		t.Skip("no symlinks here")
	}

	// Step 6 — verify. Clean, and a re-promote now says nothing about envs.
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("the carried home does not verify: %s", v.Line())
	}
	out.Reset()
	if err := a.CmdPromote(&out, PromoteOpts{Source: src}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "default_env: names") {
		t.Errorf("the tripwire still fires after the carry:\n%s", out.String())
	}
	if got := a.ListEnvSets(); len(got) != 2 {
		t.Errorf("env sets after the window: %v, want the two that were carried", got)
	}

	// Item 2: an edited PID is NOT in force until it is promoted.
	inForce := func() string {
		t.Helper()
		ag, err := a.LoadAgent("developer")
		if err != nil {
			t.Fatal(err)
		}
		return ag.Body
	}
	before := inForce()
	if err := os.WriteFile(filepath.Join(src, "agents", "developer.md"),
		[]byte(fmt.Sprintf(pidBody, "developer")+"\nAN EDIT NOBODY RATIFIED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if inForce() != before {
		t.Error("an unpromoted PID edit reached the home")
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("editing the CONSTITUTION tripped the HOME's verify: %s", v.Line())
	}
	gitIn("commit", "-qam", "pid: developer")
	out.Reset()
	if err := a.CmdPromote(&out, PromoteOpts{Source: src}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "AN EDIT NOBODY RATIFIED") {
		t.Errorf("the ratification diff did not show the edit:\n%s", out.String())
	}
	if !strings.Contains(inForce(), "AN EDIT NOBODY RATIFIED") {
		t.Error("promote did not put the edit in force")
	}

	// Item 3: one byte at the HOME — dispatch refuses, interactive warns,
	// re-promote clears, and the whole check costs what the ADR assumed.
	b, _ := newTestBackend(t)
	// The App the launches run on is the runbook's own, built from RHQ_HOME
	// above — rehearsing the real home is the point of this test. But
	// swapping it in throws away the defaults newTestBackend installed, and
	// both of those fall back to the operator's live box (app.go): until
	// ranger-base-w4fb the two launch arms below read the machine's real
	// 1-minute loadavg, so the whole rehearsal went red whenever the box
	// was busy — which is exactly when someone runs the full suite, the
	// suite being its own load source. hermetic re-arms them; the check is
	// here so that dropping it fails on any box, not only a loaded one.
	b.App = hermetic(a)
	if b.App.Load1 == nil || b.App.ModelLister == nil {
		t.Fatal("the swapped-in App is not hermetic: these launches would read the operator's live box")
	}
	var warn bytes.Buffer
	b.Warn = &warn
	pid := filepath.Join(a.AgentsDir, "devops.md")
	body, _ := os.ReadFile(pid)
	if err := os.WriteFile(pid, append(body, []byte("cage: shims\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "developer", Bead: "x-1"}); err == nil {
		t.Error("dispatch launched on a corrupted promoted set")
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: t.TempDir(), Agent: "developer"}); err != nil {
		t.Errorf("an interactive launch was refused: %v", err)
	}
	if !strings.Contains(warn.String(), "DEGRADED") || !strings.Contains(warn.String(), "agents/devops.md") {
		t.Errorf("no DEGRADED line naming the drift:\n%s", warn.String())
	}
	if err := a.CmdPromote(&bytes.Buffer{}, PromoteOpts{Source: src}); err != nil {
		t.Fatal(err)
	}
	v := a.VerifyPromoted()
	if !v.OK() {
		t.Fatalf("re-promote did not clear the mismatch: %s", v.Line())
	}
	t.Logf("live-shape rehearsal: %d promoted files, verify cost %v", len(v.Manifest.Files), v.Elapsed)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s3", Dir: t.TempDir(), Agent: "developer", Bead: "x-1"}); err != nil {
		t.Errorf("dispatch still refused after the re-promote: %v", err)
	}
}
