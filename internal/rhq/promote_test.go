package rhq

// ADR 0015's verification checklist, items 1–4 and the promote-side half of
// item 7, as tests rather than as a runbook someone runs once. The one item
// not here is 5 (seatbelt), which is ranger-base-cpyb's, and the queue half
// of 6, which is the cutover rehearsal's.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// promoteFixture builds a constitution repo with the promoted set plus each
// of the three things promote must never touch, and a separate empty home.
func promoteFixture(t *testing.T) (a *App, src string, git func(args ...string) (string, error)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "constitution")
	src = filepath.Join(repo, "rhq")
	home := filepath.Join(root, "home")
	t.Setenv("RHQ_HOME", home)
	t.Setenv(EnvPersona, "")
	a = NewApp()

	mk := func(rel, body string, mode os.FileMode) {
		t.Helper()
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	mk("config.yaml", "default_env: default\ndefault_dir: ~\n", 0o644)
	mk("agents/dev.md", "---\nname: dev\n---\nbuild things\n", 0o644)
	mk("agents/qa.md", "---\nname: qa\n---\nverify things\n", 0o644)
	mk("recipes/scratch.yaml", "purpose: scratch\n", 0o644)
	mk("skills/thing/SKILL.md", "# thing\n", 0o644)
	mk("skills/thing/references/more.md", "more\n", 0o644)
	// The three exclusions, present in the source exactly as they are in
	// the live constitution today.
	mk("envs/default.env", "TOKEN=super-secret\n", 0o600)
	mk("state/ledger.json", "{}\n", 0o644)
	mk("personas/dev/ORDERS.md", "lessons\n", 0o644)

	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	git = func(args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	// The constitution repo's own policy: secrets and machine-local state
	// never enter git (the .gitignore line ADR 0015 §7 rests on).
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("rhq/envs/\nrhq/state/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git("add", "-A"); err != nil {
		t.Fatalf("git add: %s", out)
	}
	if out, err := git("commit", "-qm", "constitution"); err != nil {
		t.Fatalf("git commit: %s", out)
	}
	return a, src, git
}

func promote(t *testing.T, a *App, o PromoteOpts) string {
	t.Helper()
	var b bytes.Buffer
	if err := a.CmdPromote(&b, o); err != nil {
		t.Fatalf("promote: %v\n%s", err, b.String())
	}
	return b.String()
}

// Item 1: after the first promote the home is a real directory with the
// promoted set and a manifest naming the promoted SHA — and nothing else.
func TestPromoteWritesTheSetAndTheManifest(t *testing.T) {
	a, src, git := promoteFixture(t)
	out := promote(t, a, PromoteOpts{Source: src})

	if st, err := os.Lstat(a.Home); err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("home is not a real directory: %v %v", st, err)
	}
	for _, rel := range []string{"config.yaml", "agents/dev.md", "agents/qa.md",
		"recipes/scratch.yaml", "skills/thing/SKILL.md", "skills/thing/references/more.md"} {
		if _, err := os.Stat(filepath.Join(a.Home, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not promoted: %v", rel, err)
		}
	}
	head, _ := git("rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil {
		t.Fatalf("manifest: %v %v", m, err)
	}
	if m.SHA != head {
		t.Errorf("manifest sha %q, HEAD %q", m.SHA, head)
	}
	if m.Seeded {
		t.Error("a promote from a commit must not be marked seeded")
	}
	// The source is recorded resolved (absResolve), so the next promote can
	// find it again through a moved symlink.
	if m.Source != absResolve(src) {
		t.Errorf("manifest source %q, want %q", m.Source, absResolve(src))
	}
	if len(m.Files) != 6 {
		t.Errorf("manifest names %d files, want the 6 in the promoted set: %v", len(m.Files), m.Files)
	}
	if !strings.Contains(out, "promoted") {
		t.Errorf("promote said nothing about promoting:\n%s", out)
	}
}

// Item 7, and the operational clause that matters most: promote never
// creates, copies, or touches home/envs — nor state/ or personas/. A copy
// path that widens 0600 publishes tokens, so the cheapest copy that cannot
// widen modes is the one that does not exist.
func TestPromoteNeverTouchesEnvsStateOrPersonas(t *testing.T) {
	a, src, _ := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src})

	for _, rel := range NotPromoted {
		if _, err := os.Stat(filepath.Join(a.Home, rel)); err == nil {
			t.Errorf("promote created %s at the home — ADR 0015 §5/§7 say it never does", rel)
		}
	}
	m, _ := ReadPromoteManifest(a.PromoteManifestPath())
	for p := range m.Files {
		for _, ex := range NotPromoted {
			if p == ex || strings.HasPrefix(p, ex+"/") {
				t.Errorf("manifest names %s — %s is out of the promoted set", p, ex)
			}
		}
	}
	// And the same again with an env set already at the home: promote must
	// leave the file and its mode exactly as it found them.
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(a.EnvsDir, "default.env")
	if err := os.WriteFile(env, []byte("TOKEN=home-side\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	promote(t, a, PromoteOpts{Source: src})
	b, err := os.ReadFile(env)
	if err != nil || string(b) != "TOKEN=home-side\n" {
		t.Errorf("promote overwrote the home's env set: %q %v", b, err)
	}
	st, err := os.Stat(env)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("env mode is %v after promote, want 0600", st.Mode().Perm())
	}
}

// Item 7's tripwire: config.yaml naming a default_env: the home has no set
// for is the state the cutover window passes through, and the one that
// otherwise comes up silent. Names only, never values.
func TestPromoteWarnsOnDanglingDefaultEnv(t *testing.T) {
	a, src, _ := promoteFixture(t)
	out := promote(t, a, PromoteOpts{Source: src})
	if !strings.Contains(out, "default_env") || !strings.Contains(out, `"default"`) {
		t.Errorf("no dangling default_env warning naming the set:\n%s", out)
	}
	if strings.Contains(out, "super-secret") {
		t.Fatalf("promote printed an env VALUE:\n%s", out)
	}
	// Carry the env set the way the runbook's step 3 does: the warning goes.
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.EnvsDir, "default.env"), []byte("TOKEN=x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := promote(t, a, PromoteOpts{Source: src}); strings.Contains(out, "default_env: names") {
		t.Errorf("the warning survived the carry step:\n%s", out)
	}
}

// Item 4, half one: a dirty promoted path is a refusal that names it.
func TestPromoteRefusesDirtyPromotedPath(t *testing.T) {
	a, src, _ := promoteFixture(t)
	if err := os.WriteFile(filepath.Join(src, "agents", "dev.md"), []byte("edited, uncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := a.CmdPromote(&bytes.Buffer{}, PromoteOpts{Source: src})
	if err == nil {
		t.Fatal("promote accepted a dirty constitution")
	}
	if !strings.Contains(err.Error(), "agents/dev.md") {
		t.Errorf("the refusal does not name the dirty path: %v", err)
	}
	if _, err := os.Stat(a.PromoteManifestPath()); err == nil {
		t.Error("a refused promote still wrote a manifest")
	}
}

// The divergence from §3's literal wording, pinned so it is a decision and
// not a drift: the two things ADR 0015 itself carves out of the promoted
// set — the queue (§4) and persona memory (§5) — are dirty in the live
// constitution repo essentially always, and must not block a promote. They
// are reported instead.
func TestPromoteReportsButAllowsDirtyOutsideThePromotedSet(t *testing.T) {
	a, src, _ := promoteFixture(t)
	if err := os.WriteFile(filepath.Join(src, "personas", "dev", "ORDERS.md"), []byte("a new lesson\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := promote(t, a, PromoteOpts{Source: src})
	if !strings.Contains(out, "personas/dev/ORDERS.md") {
		t.Errorf("dirty-outside-the-set was not reported:\n%s", out)
	}
	if !strings.Contains(out, "promoted") {
		t.Errorf("dirty persona memory blocked the promote:\n%s", out)
	}
}

// Item 4, half two: under the persona env marker promote refuses before it
// reads anything. The other spelling of the fence — deny Bash(posse
// promote:*) on every crew PID — is L1's, and TestShippedPIDsDenyPromote
// pins it in the seed.
func TestPromoteRefusesUnderThePersonaMarker(t *testing.T) {
	a, src, _ := promoteFixture(t)
	t.Setenv(EnvPersona, "dinesh")
	err := a.CmdPromote(&bytes.Buffer{}, PromoteOpts{Source: src})
	if err == nil || !strings.Contains(err.Error(), EnvPersona) {
		t.Fatalf("promote ran under %s: %v", EnvPersona, err)
	}
}

// The home may not BE the constitution — the pre-cutover shape, where
// ~/.config/rhq is a symlink onto the instance repo and "copy the source
// over the home" would be a tree copying onto itself.
func TestPromoteRefusesWhenTheHomeIsTheSource(t *testing.T) {
	a, src, _ := promoteFixture(t)
	a.Home = src
	if err := a.CmdPromote(&bytes.Buffer{}, PromoteOpts{Source: src}); err == nil {
		t.Fatal("promote accepted the constitution as its own home")
	}
	link := filepath.Join(t.TempDir(), "linked-home")
	if err := os.Symlink(src, link); err != nil {
		t.Skip("no symlinks here")
	}
	a.Home = link
	if err := a.CmdPromote(&bytes.Buffer{}, PromoteOpts{Source: src}); err == nil {
		t.Fatal("promote accepted a symlinked home (ADR 0015 §2 wants a real directory)")
	}
}

// A promoted path git is forbidden to carry has no commit behind it, so it
// cannot be promoted from one — the shape ranger-base-h56a found in envs/,
// caught here for any path that grows the same problem.
func TestPromoteRefusesAnIgnoredPromotedPath(t *testing.T) {
	a, src, git := promoteFixture(t)
	repo := filepath.Dir(src)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("rhq/envs/\nrhq/state/\nrhq/recipes/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git("rm", "-r", "-q", "--cached", "rhq/recipes"); err != nil {
		t.Fatalf("git rm --cached: %s", out)
	}
	if out, err := git("commit", "-qam", "ignore recipes"); err != nil {
		t.Fatalf("commit: %s", out)
	}
	err := a.CmdPromote(&bytes.Buffer{}, PromoteOpts{Source: src})
	if err == nil || !strings.Contains(err.Error(), "recipes") {
		t.Fatalf("an ignored promoted path was promoted anyway: %v", err)
	}
}

// Item 2's mechanism: promote puts a NEW commit in force, and prints the
// diff between the commit in force and the one arriving — the thing being
// ratified.
func TestPromotePrintsTheDiffSinceTheLastPromote(t *testing.T) {
	a, src, git := promoteFixture(t)
	first := promote(t, a, PromoteOpts{Source: src})
	if !strings.Contains(first, "no previous promote") {
		t.Errorf("first promote did not say it had nothing to diff against:\n%s", first)
	}
	if err := os.WriteFile(filepath.Join(src, "agents", "dev.md"), []byte("---\nname: dev\n---\nbuild BETTER things\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git("commit", "-qam", "pid: dev builds better things"); err != nil {
		t.Fatalf("commit: %s", out)
	}
	second := promote(t, a, PromoteOpts{Source: src})
	if !strings.Contains(second, "build BETTER things") || !strings.Contains(second, "agents/dev.md") {
		t.Errorf("the ratification diff does not show the PID change:\n%s", second)
	}
	b, _ := os.ReadFile(filepath.Join(a.Home, "agents", "dev.md"))
	if !strings.Contains(string(b), "BETTER") {
		t.Error("the promoted copy did not change")
	}
	// Same commit twice is not a diff, and says so rather than printing
	// nothing at all.
	if third := promote(t, a, PromoteOpts{Source: src}); !strings.Contains(third, "same commit") {
		t.Errorf("re-promoting the same commit said nothing:\n%s", third)
	}
}

// A PID deleted in the constitution must leave the home, or a retired
// persona stays in force forever. Bounded to the promoted set, and printed.
func TestPromoteRemovesWhatTheConstitutionNoLongerHas(t *testing.T) {
	a, src, git := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src})
	if err := os.Remove(filepath.Join(src, "agents", "qa.md")); err != nil {
		t.Fatal(err)
	}
	if out, err := git("commit", "-qam", "retire qa"); err != nil {
		t.Fatalf("commit: %s", out)
	}
	out := promote(t, a, PromoteOpts{Source: src})
	if _, err := os.Stat(filepath.Join(a.Home, "agents", "qa.md")); err == nil {
		t.Error("a retired PID is still in force at the home")
	}
	if !strings.Contains(out, "removed agents/qa.md") {
		t.Errorf("the removal was silent:\n%s", out)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("the home does not match the manifest promote just wrote: %s", v.Line())
	}
}

// Promote is the fix for a broken manifest, so a manifest it cannot read
// must not be the thing that stops it — by then the launch verify is
// already refusing every dispatch.
func TestPromoteWorksOverAnUnreadableManifest(t *testing.T) {
	a, src, _ := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src})
	if err := os.WriteFile(a.PromoteManifestPath(), []byte("{ truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); v.OK() {
		t.Fatal("a truncated manifest verified clean")
	}
	out := promote(t, a, PromoteOpts{Source: src})
	if !strings.Contains(out, "without a baseline") {
		t.Errorf("promote did not say it lost its baseline:\n%s", out)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("promote did not repair the manifest: %s", v.Line())
	}
}

// --dry-run is the ratification read with the act left out.
func TestPromoteDryRunWritesNothing(t *testing.T) {
	a, src, _ := promoteFixture(t)
	out := promote(t, a, PromoteOpts{Source: src, DryRun: true})
	if !strings.Contains(out, "dry run") {
		t.Errorf("no dry-run line:\n%s", out)
	}
	if _, err := os.Stat(a.PromoteManifestPath()); err == nil {
		t.Error("--dry-run wrote a manifest")
	}
	if _, err := os.Stat(filepath.Join(a.Home, "agents")); err == nil {
		t.Error("--dry-run wrote the promoted set")
	}
}

// Item 3, the detection half: one changed byte, one added file and one
// deleted file are each a mismatch the verdict names.
func TestVerifyPromotedCatchesEveryClassOfDrift(t *testing.T) {
	a, src, _ := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src})
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("a fresh promote does not verify: %s", v.Line())
	}

	pid := filepath.Join(a.Home, "agents", "dev.md")
	b, _ := os.ReadFile(pid)
	if err := os.WriteFile(pid, append(b, ' '), 0o644); err != nil {
		t.Fatal(err)
	}
	v := a.VerifyPromoted()
	if v.OK() || len(v.Changed) != 1 || v.Changed[0] != "agents/dev.md" {
		t.Errorf("one changed byte was not caught: %+v", v)
	}
	if !strings.Contains(v.Line(), "changed agents/dev.md") {
		t.Errorf("the line does not name it: %s", v.Line())
	}
	if err := os.WriteFile(pid, b, 0o644); err != nil {
		t.Fatal(err)
	}

	// The 6ne shape: a session writes itself a new PID. It is in NO
	// manifest, so it is the one drift a digest of known files would miss.
	rogue := filepath.Join(a.Home, "agents", "rogue.md")
	if err := os.WriteFile(rogue, []byte("---\nname: rogue\ncage: shims\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); v.OK() || len(v.Added) != 1 {
		t.Errorf("an unpromoted PID was not caught: %+v", v)
	}
	os.Remove(rogue)

	if err := os.Remove(filepath.Join(a.Home, "skills", "thing", "references", "more.md")); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); v.OK() || len(v.Missing) != 1 {
		t.Errorf("a deleted promoted file was not caught: %+v", v)
	}

	// Re-promote is the whole fix, which is what makes fail-closed cheap.
	promote(t, a, PromoteOpts{Source: src})
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("re-promote did not clear the mismatch: %s", v.Line())
	}
}

// Item 7's other half, and the one that decides whether anybody keeps
// listening to the check: corrupting an env file must NOT trip the verify.
// Editing an env set at the home is a supported live path (WriteEnvSet), so
// a verify that fired on it would refuse dispatch on routine correct
// behaviour until a re-promote that cannot even see the values.
func TestVerifyPromotedIgnoresEnvsEntirely(t *testing.T) {
	a, src, _ := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src})
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"TOKEN=one\n", "\x00\x00 corrupted \xff", ""} {
		if err := os.WriteFile(filepath.Join(a.EnvsDir, "default.env"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if v := a.VerifyPromoted(); !v.OK() {
			t.Fatalf("an env file tripped the launch verify (ADR 0015 §7 puts it out of scope): %s", v.Line())
		}
	}
	// Same for state/ and the persona memory symlink §5 keeps.
	if err := os.MkdirAll(filepath.Join(a.StateDir, "herdr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.StateDir, "herdr", "x.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "personas"), filepath.Join(a.Home, "personas")); err == nil {
		if v := a.VerifyPromoted(); !v.OK() {
			t.Errorf("state/ or personas/ tripped the verify: %s", v.Line())
		}
	}
}

// No manifest = nothing was ever promoted here = nothing to check. Every
// home that predates ADR 0015 is in that state and must keep launching.
func TestVerifyPromotedIsSilentWithNoManifest(t *testing.T) {
	a, src, _ := promoteFixture(t)
	_ = src
	if v := a.VerifyPromoted(); !v.OK() || v.Manifest != nil {
		t.Errorf("an unpromoted home does not verify clean: %+v", v)
	}
	// A manifest posse cannot read is NOT clean — it is the trust anchor
	// gone, which is exactly what a launch must not shrug at.
	if err := os.MkdirAll(a.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.PromoteManifestPath(), []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); v.OK() {
		t.Error("an unreadable manifest verified clean")
	}
}

// Item 4's `posse init` clause: a fresh box gets a manifest of its own, so
// the launch verify never fires on an install nobody promoted — and it says
// `seeded`, because there is no commit behind it.
func TestInitSeedsAManifestMarkedSeeded(t *testing.T) {
	a := initTestApp(t)
	if err := a.CmdInit(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil {
		t.Fatalf("init wrote no manifest: %v %v", m, err)
	}
	if !m.Seeded || m.SHA != "" {
		t.Errorf("a seeded manifest must be marked seeded and name no commit: %+v", m)
	}
	if len(m.Files) < 10 {
		t.Errorf("the seeded manifest covers only %d files", len(m.Files))
	}
	for p := range m.Files {
		if strings.HasPrefix(p, "envs/") {
			t.Errorf("init's manifest names %s — envs/ is out of the promoted set (§7)", p)
		}
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("a freshly initialized home does not verify: %s", v.Line())
	}
	// init seeds envs/ separately, with modes — that path is untouched.
	if st, err := os.Stat(a.EnvsDir); err != nil || st.Mode().Perm() != 0o700 {
		t.Errorf("init no longer seeds envs/ 0700: %v %v", st, err)
	}
	// Running init twice must not double-write or invalidate the manifest.
	if err := a.CmdInit(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("a second init broke the manifest: %s", v.Line())
	}
}

// ADR 0015 marks the launch-time hash ASSUMED-negligible and asks for a
// measurement, because it sits on the refusal path of every dispatch. This
// is that measurement, at more than twice the live constitution's size, and
// it fails if the assumption stops holding.
func TestVerifyPromotedCostIsNegligible(t *testing.T) {
	t.Setenv("RHQ_HOME", filepath.Join(t.TempDir(), "home"))
	a := NewApp()
	body := strings.Repeat("prose that a persona identity document is made of.\n", 200) // ~10KB
	write := func(rel string) {
		p := filepath.Join(a.Home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("config.yaml")
	for i := 0; i < 40; i++ { // the live instance has 17 PIDs
		write(fmt.Sprintf("agents/p%02d.md", i))
	}
	for i := 0; i < 20; i++ {
		write(fmt.Sprintf("recipes/r%02d.yaml", i))
	}
	for i := 0; i < 30; i++ {
		write(fmt.Sprintf("skills/s%02d/SKILL.md", i))
		write(fmt.Sprintf("skills/s%02d/references/a.md", i))
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	n := len(mustManifest(t, a).Files)

	var worst time.Duration
	for i := 0; i < 20; i++ {
		v := a.VerifyPromoted()
		if !v.OK() {
			t.Fatalf("fixture does not verify: %s", v.Line())
		}
		if v.Elapsed > worst {
			worst = v.Elapsed
		}
	}
	t.Logf("VerifyPromoted over %d files (~%dKB): worst of 20 runs = %v", n, n*len(body)/1024, worst)
	// A launch already costs seconds (herdr, the runtime's own start, the
	// availability preflight). The bound is deliberately far above the
	// measurement: it is a regression alarm — someone hashing per-launch in
	// a loop, or reading the whole tree twice — not a benchmark.
	if worst > 250*time.Millisecond {
		t.Errorf("the launch verify costs %v — ADR 0015's ASSUMED-negligible no longer holds", worst)
	}
}

func mustManifest(t *testing.T, a *App) *PromoteManifest {
	t.Helper()
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil {
		t.Fatalf("manifest: %v %v", m, err)
	}
	return m
}

// The fence's other spelling, in the seed the harness ships: every PID an
// instance starts from denies the verb. ADR 0015 §3 — politeness against a
// determined session, and the thing that makes a promote from a persona a
// refused turn rather than a discovered fact.
func TestShippedPIDsDenyPromote(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "agents")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seed := &App{AgentsDir: dir}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		n++
		ag, err := seed.LoadAgent(strings.TrimSuffix(e.Name(), ".md"))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		found := false
		for _, d := range ag.Deny {
			if d == "Bash(posse promote:*)" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not deny Bash(posse promote:*) (ADR 0015 §3)", e.Name())
		}
		// And it has to be a rule L1 can actually realize, or the fence is
		// prose on the tier most of the fleet runs at — AND parity has to be
		// able to say so, or every PID carrying it launches DEGRADED on every
		// runtime × cage, which is how a fence turns into --allow-degraded
		// muscle memory (measured on the live gates report before
		// globalValueOpts learned that posse takes no global value options).
		rules := ParseShimRules(ag.Deny)
		if len(rules["posse"]) == 0 {
			t.Errorf("%s: the promote deny does not parse into a posse shim rule", e.Name())
			continue
		}
		for _, r := range rules["posse"] {
			if kind, faithful := matcherFor("posse", r); !faithful {
				t.Errorf("%s: %s realizes only as %q — parity will call every launch DEGRADED", e.Name(), r.Rule, kind)
			}
		}
	}
	if n < 5 {
		t.Fatalf("read only %d shipped PIDs", n)
	}
}
