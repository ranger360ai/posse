package posse_test

// QA pins for ranger-base-8zki — the detective control over L3 hook staleness,
// scripts/verify-hook-freshness.sh.
//
// The defect this came from: the hook bodies are compiled into the binary, so
// every hook on a box is a COPY. Only `posse gates install-hooks` and a session
// create re-render one, and a session's worktree shares the COMMON hooks dir of
// the repo it was cut from — so a session create refreshes that one repo and no
// other. Every other hooked repo (an instance's private beads repos never hold
// a session) is re-rendered by nothing at all, and one such pair ran a hook
// three days behind the binary. It still refused; what it had lost was the
// ranger-base-erba paragraph (b291784) prescribing
// `git diff HEAD -- <paths>`, so it sent every reader to the bare two-dot
// `git diff`, which is blind to another persona's staged edit. A wall that
// refuses with stale guidance fails silently, and reinstalling once fixes the
// day, not the class: the next change to the hook body re-stales those repos
// exactly the same way.
//
// Three arms are the whole point:
//   - the STALE arm. A body that carries our marker and still refuses is the
//     failure that shipped. If only presence were checked, it would pass.
//   - the STAMP arm. Identity is normalized over the one line the render
//     legitimately varies per repo (posse_beads_visibility=), so identity
//     alone cannot see a PRIVATE repo carrying a PUBLIC stamp — the half that
//     would leak ops-class beads. It is asserted separately, by name.
//   - the EMPTY arm. With no configured repo present the script has measured
//     nothing, and "no findings" would be a pass earned by looking at nothing.
//     It must exit 2, not 0.
//
// Every case runs against a scratch HOME and a scratch RHQ_HOME, so no case
// can read — or judge — the operator's live checkouts.

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

const hfScript = "scripts/verify-hook-freshness.sh"

// hfBuild builds the binary the script renders its reference from. The
// reference is deliberately not a checked-in string: it comes out of the
// renderer under test, so this control cannot drift away from it.
func hfBuild(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// hfGit resolves the REAL git, ignoring any posse L1 shim on the session PATH:
// a persona whose PID denies `git commit` otherwise runs this suite through a
// wrapper that refuses the fixture commits (see suite-red notes on $RHQ_GATES_DIR).
func hfGit(t *testing.T) string {
	t.Helper()
	for _, p := range []string{"/usr/bin/git", "/opt/homebrew/bin/git", "/usr/local/bin/git"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	p, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	return p
}

// hfRig is a scratch box: a config home carrying a beads_visibility: block, and
// one git repo per entry with hooks freshly rendered by the binary under test.
type hfRig struct {
	bin, git, home, rhqHome string
	repos                   map[string]string // name -> path
	// gitconfig is a GIT_CONFIG_GLOBAL naming a managed core.hooksPath, set
	// by manage() and empty on an ordinary box. Every arm below that is not
	// about a managed box leaves it empty and is unaffected.
	gitconfig string
	// shimDir goes on the FRONT of the script's PATH when set, so a wrapper
	// stands where `git` is looked up. extraEnv is appended last.
	shimDir  string
	extraEnv []string
}

// prep runs against each repo after `git init` and BEFORE its hooks are
// rendered — the only window in which a fixture can give a repo the property
// the render reads (a .beads/redirect, a repo-local user.email) and still have
// the installed hook be the render for that property. Variadic so every arm
// that needs no such property is unchanged.
func hfNewRig(t *testing.T, vis map[string]string, prep ...func(t *testing.T, name, path string)) *hfRig {
	t.Helper()
	r := &hfRig{bin: hfBuild(t), git: hfGit(t), home: t.TempDir(), repos: map[string]string{}}
	r.rhqHome = filepath.Join(r.home, ".config", "posse")
	if err := os.MkdirAll(r.rhqHome, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("beads_visibility:\n")
	for name, v := range vis {
		p := filepath.Join(r.home, name)
		r.repos[name] = p
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(r.git, "-C", p, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		b.WriteString("  " + p + ": " + v + "\n")
	}
	if err := os.WriteFile(filepath.Join(r.rhqHome, "config.yaml"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, p := range r.repos {
		for _, fn := range prep {
			fn(t, name, p)
		}
	}
	for name, p := range r.repos {
		cmd := exec.Command(r.bin, "gates", "install-hooks", p)
		cmd.Env = append(os.Environ(), "HOME="+r.home, "RHQ_HOME="+r.rhqHome)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("install-hooks %s: %v %s", name, err, out)
		}
	}
	// The rig must prove it built the fixture. An assertion of absence is
	// satisfied by measuring nothing, and a plant that silently did nothing
	// would make every later arm green for the wrong reason.
	for name, want := range vis {
		got := hfRead(t, r.hook(name))
		if !strings.Contains(got, "posse_beads_visibility='"+want+"'") {
			t.Fatalf("rig never built: %s was not stamped %q", name, want)
		}
	}
	return r
}

func (r *hfRig) hook(name string) string {
	return filepath.Join(r.repos[name], ".git", "hooks", "prepare-commit-msg")
}

// manage turns the rig into the managed box of ADR 0052: one absolute hooks
// directory outside every repo, holding an employer hook, unwritable by this
// uid, named by a global core.hooksPath. Called AFTER hfNewRig, so each repo
// keeps the posse hooks already rendered into its own .git/hooks — which is
// the whole point: on this box those are files git never runs, and a control
// that reads them is reading a wall that is not armed.
func (r *hfRig) manage(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: a mode bit is not a wall for uid 0, so this fixture cannot be built")
	}
	managed := filepath.Join(r.home, "managed-hooks")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	// The employer's own hook, so the fixture is the shape the ADR describes
	// rather than an empty directory.
	if err := os.WriteFile(filepath.Join(managed, "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(managed, 0o555); err != nil {
		t.Fatal(err)
	}
	// Before t.TempDir's own cleanup, which cannot unlink through a
	// read-only directory (LIFO: registered later, runs first).
	t.Cleanup(func() { os.Chmod(managed, 0o755) })
	// Measured, not assumed. If this uid can write here after all, the
	// classification under test never fires and every arm below would be
	// green about a fixture that is not a managed box.
	probe := filepath.Join(managed, ".hf-fixture-probe")
	if err := os.WriteFile(probe, []byte("x"), 0o600); err == nil {
		os.Remove(probe)
		t.Skipf("%s is writable at mode 0555 — no managed path to classify here", managed)
	}
	r.gitconfig = filepath.Join(r.home, "gitconfig-managed")
	if err := os.WriteFile(r.gitconfig, []byte("[core]\n\thooksPath = "+managed+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The fixture must prove it aimed git somewhere: a global that did not
	// take would leave every repo unmanaged and the managed arms asserting
	// nothing.
	cmd := exec.Command(r.git, "-C", r.repos[anyName(r)], "rev-parse", "--git-path", "hooks")
	cmd.Env = append(os.Environ(), "HOME="+r.home, "GIT_CONFIG_GLOBAL="+r.gitconfig)
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != managed {
		t.Fatalf("rig never built: git dispatches from %q, want %s (%v)", strings.TrimSpace(string(out)), managed, err)
	}
	return managed
}

// anyName is a stable pick from the rig's repos for the fixture's own
// self-check — map order is random, and a self-check that reads a different
// repo each run is a flake waiting to be blamed on the code.
func anyName(r *hfRig) string {
	names := make([]string, 0, len(r.repos))
	for n := range r.repos {
		names = append(names, n)
	}
	sort.Strings(names)
	return names[0]
}

// escape gives one repo a hooks path of its own, the way a repo on a managed
// box opts back out: a RELATIVE core.hooksPath is resolved against the
// worktree, so it is not absolute, not outside the repo, and not managed.
// This is the repo the reference render is needed for.
func (r *hfRig) escape(t *testing.T, name string) {
	t.Helper()
	cmd := exec.Command(r.git, "-C", r.repos[name], "config", "core.hooksPath", ".git/hooks")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config core.hooksPath: %v %s", err, out)
	}
}

func (r *hfRig) run(t *testing.T) (string, int) {
	t.Helper()
	abs, err := filepath.Abs(hfScript)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(abs)
	path := filepath.Dir(r.git) + ":/usr/bin:/bin"
	if r.shimDir != "" {
		path = r.shimDir + ":" + path
	}
	cmd.Env = []string{
		"HOME=" + r.home,
		"RHQ_HOME=" + r.rhqHome,
		"POSSE=" + r.bin,
		"PATH=" + path,
	}
	if r.gitconfig != "" {
		cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL="+r.gitconfig)
	}
	cmd.Env = append(cmd.Env, r.extraEnv...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v\n%s", hfScript, err, out)
	}
	return string(out), code
}

func hfRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hfWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// A freshly rendered box is the control arm: without it, every red arm below
// could be red for a reason that has nothing to do with what it plants.
func TestQAHookFreshnessFreshBoxPasses(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private", "pub": "public"})
	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("fresh box must exit 0, got %d:\n%s", code, out)
	}
	for _, want := range []string{"fresh", "stamped  private", "stamped  public", "2 repo(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// THE ARM THAT SHIPPED. A hook that carries our marker and still refuses, but
// whose body is behind the binary — exactly the pair found stale on the box,
// where the drift was the erba `git diff HEAD -- <paths>` paragraph.
func TestQAHookFreshnessCatchesAStaleBodyThatStillRefuses(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	body := hfRead(t, r.hook("priv"))
	const erba = "'git diff HEAD -- <paths>'"
	if !strings.Contains(body, erba) {
		t.Fatalf("rig never built: the current render does not carry %s", erba)
	}
	// Drop the prescription the stale hook was missing, and nothing else: the
	// hook still carries the marker, still refuses, still exits 1.
	hfWrite(t, r.hook("priv"), strings.Replace(body, erba, "'git diff'", 1))
	out, code := r.run(t)
	if code != 1 {
		t.Fatalf("a stale body must be a finding (exit 1), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "prepare-commit-msg is STALE") {
		t.Errorf("staleness must be named as such:\n%s", out)
	}
}

// Identity normalizes the visibility stamp away, so it cannot see this on its
// own. A private repo wearing a public stamp is the leak the guard exists to
// prevent, so it is asserted separately, against config.
func TestQAHookFreshnessCatchesAStampThatDisagreesWithConfig(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	body := hfRead(t, r.hook("priv"))
	hfWrite(t, r.hook("priv"), strings.Replace(body,
		"posse_beads_visibility='private'", "posse_beads_visibility='public'", 1))
	out, code := r.run(t)
	if code != 1 {
		t.Fatalf("a mismatched stamp must be a finding (exit 1), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "visibility stamp is 'public' but config says 'private'") {
		t.Errorf("the mismatch must name both sides:\n%s", out)
	}
	// And the STALE arm must NOT fire on it: the bodies are identical once the
	// stamp is normalized. A stamp finding that also reports staleness would
	// mean the normalization is not doing its job and every private repo on
	// the box is one render away from a false alarm.
	if strings.Contains(out, "is STALE") {
		t.Errorf("the stamp line alone must not read as staleness:\n%s", out)
	}
}

// ADR 0023: a slot counts only when identity AND behavior hold. A file wearing
// our marker that refuses nothing is the planted-hook case.
func TestQAHookFreshnessCatchesAMarkedHookThatRefusesNothing(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	hfWrite(t, r.hook("priv"), "#!/bin/sh\n# posse-gate shared-index\nexit 0\n")
	out, code := r.run(t)
	if code != 1 {
		t.Fatalf("a hook that refuses nothing must be a finding, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "an unqualified commit is NOT refused") {
		t.Errorf("the behavior arm must fire:\n%s", out)
	}
}

// The other behavior arm, and the reason there are two: a hook that refuses
// EVERYTHING is not a wall either — it leaves the safe form no way through,
// which is how a guard gets turned off by whoever it blocks.
func TestQAHookFreshnessCatchesAHookThatRefusesTheSafeFormToo(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	hfWrite(t, r.hook("priv"), "#!/bin/sh\n# posse-gate shared-index\nexit 1\n")
	out, code := r.run(t)
	if code != 1 {
		t.Fatalf("a hook with no way through must be a finding, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "a path-limited commit is refused too") {
		t.Errorf("the safe-form arm must fire:\n%s", out)
	}
}

// ranger-base-ixv4, and the reason this control had to be fixed before it was
// pointed at anyone: the safe-form arm needs an INDEX, not an index NAME.
//
// git's own next-index-<pid> is a copy of the index with the named paths
// refreshed into it. Handing the hook a name that points at nothing is an
// EMPTY index, and `git diff --cached --name-only` against an empty index
// reports every tracked file — so once ranger-base-ak3e added the
// constitution arm, which reads exactly that, the fabricated safe form was
// refused for touching the whole class. Measured on the live box
// 2026-08-29: `a path-limited commit is refused too — the safe form has no
// way through` in the constitution repo and ~/src/posse, the two configured
// repos that carry class paths, and in neither of the two that do not.
//
// The rig's other repos have no commits, where an empty index is what git
// itself would hand the slot — which is why every pin above stayed green
// through the defect and none of them could have caught it. This one commits
// a class member first.
func TestQAHookFreshnessDoesNotCallTheSafeFormRefusedOverAClassPathInHEAD(t *testing.T) {
	// The identity BEFORE the render, not after (ranger-base-x5olh). A
	// repo-local user.email is one of check 3's literal sources, so setting
	// one after the hook is written leaves a hook that really is behind its
	// repo — and since the reference is rendered for the repo, the control
	// now says so. That is the right verdict and the wrong fixture: this arm
	// is about the safe form over a class path in HEAD, so it gives the repo
	// its committer identity first and stays a fresh box.
	r := hfNewRig(t, map[string]string{"priv": "private"},
		func(t *testing.T, _, path string) {
			t.Helper()
			for _, kv := range [][2]string{{"user.email", "t@t"}, {"user.name", "t"}} {
				out, err := exec.Command(hfGit(t), "-C", path, "config", kv[0], kv[1]).CombinedOutput()
				if err != nil {
					t.Fatalf("git config %s: %v %s", kv[0], err, out)
				}
			}
		})
	repo := r.repos["priv"]
	dir := filepath.Join(repo, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// .claude/settings.json is in the constitution class in EVERY hooked
	// repo (ranger-base-az93: it carries the session's own deny list), so it
	// is the whole trigger without needing a constitution-marker tree.
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", repo, "add", "--", ".claude/settings.json"},
		{"-C", repo, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "class member", "--", ".claude/settings.json"},
	} {
		if out, err := exec.Command(r.git, args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if out, err := exec.Command(r.git, "-C", repo, "ls-tree", "--name-only", "HEAD", "-r").Output(); err != nil ||
		!strings.Contains(string(out), ".claude/settings.json") {
		t.Fatalf("rig never built: HEAD does not carry the class path (%v)\n%s", err, out)
	}

	out, code := r.run(t)
	if strings.Contains(out, "a path-limited commit is refused too") {
		t.Fatalf("the safe form was called refused over a class path that HEAD already carries — "+
			"the arm is measuring its own empty index, not the wall:\n%s", out)
	}
	if !strings.Contains(out, "path-limited allowed (0)") {
		t.Fatalf("the safe-form arm did not run at all:\n%s", out)
	}
	if code != 0 {
		t.Fatalf("a fresh box with a class path in HEAD must still pass, got %d:\n%s", code, out)
	}
	// And the arm is still an arm: the wrong-arm control above
	// (CatchesAHookThatRefusesTheSafeFormToo) is what proves it can fail.
}

func TestQAHookFreshnessCatchesAForeignHook(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	hfWrite(t, r.hook("priv"), "#!/bin/sh\n# somebody else's hook\nexit 0\n")
	out, code := r.run(t)
	if code != 1 {
		t.Fatalf("a foreign hook must be a finding, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "is not posse's") {
		t.Errorf("a foreign slot must be named as foreign, not as stale:\n%s", out)
	}
}

// A checkout whose slot a foreign shim reached first does not carry our hook
// there at all: install-hooks moved the shim aside, wrote the dispatcher into
// the slot, and put ours in posse-<slot>. A control that only looked at the
// slot would call such a repo unguarded — including, commonly, the posse
// checkout the fleet runs in.
func TestQAHookFreshnessReadsTheChainedLayout(t *testing.T) {
	r := hfNewRig(t, map[string]string{"pub": "public"})
	hooks := filepath.Join(r.repos["pub"], ".git", "hooks")
	for _, slot := range []string{"prepare-commit-msg", "pre-push"} {
		ours := hfRead(t, filepath.Join(hooks, slot))
		hfWrite(t, filepath.Join(hooks, "posse-"+slot), ours)
		hfWrite(t, filepath.Join(hooks, "bd-"+slot), "#!/bin/sh\nexit 0\n")
		hfWrite(t, filepath.Join(hooks, slot), "#!/bin/sh\nd=$(dirname \"$0\")\n"+
			"\"$d/posse-"+slot+"\" \"$@\" || exit $?\n"+
			"[ -x \"$d/bd-"+slot+"\" ] || exit 0\n"+
			"exec \"$d/bd-"+slot+"\" \"$@\"\n")
	}
	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("a chained layout is installed, not a finding; got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "posse-prepare-commit-msg") {
		t.Errorf("the chained member must be the one reported:\n%s", out)
	}
}

// A clean report is evidence only if something was looked at.
func TestQAHookFreshnessRefusesToPassWhenNothingWasMeasured(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	if err := os.RemoveAll(r.repos["priv"]); err != nil {
		t.Fatal(err)
	}
	out, code := r.run(t)
	if code != 2 {
		t.Fatalf("nothing measured must exit 2, not %d:\n%s", code, out)
	}
	if !strings.Contains(out, "nothing measured, not a pass") {
		t.Errorf("it must say so:\n%s", out)
	}
}

// ranger-base-heyb. The block reader disagreed with YamlMapPairs
// (internal/posse/yamlflat.go) on three rules — the same class of split fqfw
// and k3yd already closed one level up, in cfg(): a comment starts at
// whitespace + '#', not '#' anywhere, so a hash with no space before it is
// data, not a truncated line; a matched pair of double quotes is dropped,
// not kept, so a quoted path is not a different path; and the value is the
// REST of the line after the first ':', trimmed — not the awk field after
// it, which stopped at the first space and truncated a value (or a key)
// with a space in it.
//
// Compared against YamlMapPairs directly, not a spelling: every corpus line
// is read by both, and the script's printed "(config says: ...)" line must
// name the same key and value YamlMapPairs read from the same file. The
// repos are fictitious — the point is what got parsed out of config, not
// whether anything was found on disk, so the run exits 2 (nothing measured)
// on purpose, and that is not a failure of this test.
func TestQAHookFreshnessSubkeyReaderAgreesWithYamlMapPairs(t *testing.T) {
	bin := hfBuild(t)
	git := hfGit(t)

	corpus := []string{
		"  /nowhere/plain: private",
		"  /nowhere/hash#nospace: private",      // rule 1: no space before '#' — data, not a comment
		`  /nowhere/quoted: "private"`,          // rule 2: a matched pair of quotes is dropped
		"  /nowhere/valspace: pub lic",          // rule 3: the rest of the line, not the first field
		"  /nowhere/keyspace with gap: private", // rule 3 on the key side too
		"  /nowhere/spacedhash: private # a real comment",
		"  /nowhere/tabhash: private\t# tab comment",
		`  /nowhere/lonequote: "priv`,  // an unmatched quote is not a pair
		`  /nowhere/nested: "pub#lic"`, // a hash inside a quoted value is data
	}
	config := "beads_visibility:\n" + strings.Join(corpus, "\n") + "\n"

	home := t.TempDir()
	rhqHome := filepath.Join(home, ".config", "posse")
	if err := os.MkdirAll(rhqHome, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(rhqHome, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	// The control: what posse itself reads. Without it a mismatch below
	// could mean the fixture is not what it claims to be, not that the
	// script disagrees.
	want := posse.YamlMapPairs(cfgPath, "beads_visibility")
	if len(want) != len(corpus) {
		t.Fatalf("fixture is not what it claims to be: YamlMapPairs read %d pairs from %d corpus lines", len(want), len(corpus))
	}

	abs, err := filepath.Abs(hfScript)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(abs)
	cmd.Env = []string{
		"HOME=" + home,
		"RHQ_HOME=" + rhqHome,
		"POSSE=" + bin,
		"PATH=" + filepath.Dir(git) + ":/usr/bin:/bin",
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v\n%s", hfScript, err, out)
	}
	if code != 2 {
		t.Fatalf("none of the corpus repos exist, so nothing measured (exit 2) is what proves the loop ran the whole corpus; got %d:\n%s", code, out)
	}

	for _, kv := range want {
		line := "  " + kv[0] + "  (config says: " + kv[1] + ")"
		if !strings.Contains(string(out), line) {
			t.Errorf("script did not read %q the way YamlMapPairs did — wanted %q in:\n%s", kv[0]+": "+kv[1], line, out)
		}
	}
}

// The script is read-only and wired into the Makefile: a control nobody runs
// is the same shape as the hook that went stale.
func TestQAHookFreshnessIsReadOnlyAndWired(t *testing.T) {
	b, err := os.ReadFile(hfScript)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	// It must never repair what it finds — a hook rewrite in someone else's
	// shared checkout is a change the operator types.
	//
	// `install-hooks "$repo"` was on this list and is not any more
	// (ranger-base-x5olh): the reference is rendered FOR the repo now, under
	// the core.hooksPath redirect, which is the only way it can carry that
	// repo's own identity literals. So the verb IS here and the ban became a
	// ban on the mechanism. What made it safe was never the absence of the
	// string, it is where the render lands — a fact a grep cannot see and a
	// run can, so it is pinned by
	// TestQAHookFreshnessWritesNothingIntoTheRepoItMeasures instead. These two
	// stay banned because neither has a landing place that could redeem it:
	// a render over the slot being judged, and a copy of the reference into
	// place.
	for _, banned := range []string{"install-hooks \"$m\"", "cp \"$ref"} {
		if strings.Contains(src, banned) {
			t.Errorf("the control must not repair what it finds: %q", banned)
		}
	}
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mk), "scripts/verify-hook-freshness.sh") {
		t.Error("scripts/verify-hook-freshness.sh is not wired into the Makefile")
	}
	if !strings.Contains(string(mk), "verify-hook-freshness") {
		t.Error("no verify-hook-freshness target in the Makefile")
	}
}

// ─── the managed box (ranger-base-1se2l, ADR 0052) ───────────────────────────
//
// THE DEFECT. On an employer's box a global core.hooksPath aims every git at
// one absolute, root-owned directory. The reference render is an
// `install-hooks` into a throwaway repo — which inherits that global, is
// classified managed (ADR 0052 D1), writes no hooks and prints no
// `visibility guard: public` line. So the script exited 2,
// `reference render is not public — nothing measured`, for the WHOLE box:
// the detective control for stale L3 hooks was dead on exactly the box ADR
// 0052 is about. MEASURED 2026-09-02 on this host, both binaries.
//
// Two arms, because the fix is two things and either one alone leaves a hole:
//
//   - a fully managed box is CLEAN, not unmeasured. Nothing of posse's is
//     installed there to go stale; the wall is the session hooks dir rendered
//     at each launch. The leftover copy in the repo's own .git/hooks is a file
//     git never runs, and reporting it `fresh` would be a green about a wall
//     that is not armed — so the arm asserts that word is absent too.
//   - a MIXED box still catches staleness. This is the arm that dies if the
//     reference render ever goes back to inheriting the global: without a
//     reference there is nothing to compare a hook to, and a stale one on the
//     repo that escaped the managed path sails past.

// A box where every configured repo dispatches from the managed directory.
// Exit 0 and say so — and say nothing about the dead copies in .git/hooks.
func TestQAHookFreshnessOnAFullyManagedBoxIsCleanNotUnmeasured(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	managed := r.manage(t)
	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("a managed box has no posse hook that can be stale, so it is clean; got exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "nothing measured") {
		t.Errorf("the control reported itself dead on the box ADR 0052 is about:\n%s", out)
	}
	// The line abbreviates $HOME, as every posse line does; derived from the
	// fixture rather than spelled, so the pin names this run's directory.
	abbrev := "~" + strings.TrimPrefix(managed, r.home)
	for _, want := range []string{
		"managed hooks path " + abbrev,
		"1 repo(s) dispatch from a managed hooks path",
		"no repo carries a posse-installed hook that could be stale",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// The leftover .git/hooks copy is not a wall here — git dispatches from
	// the managed dir — so it must not be reported as one. This is the false
	// GREEN the classification prevents, and it is a different failure from
	// the false finding below.
	if strings.Contains(out, "fresh    prepare-commit-msg") {
		t.Errorf("a hook git never runs was reported fresh — that is a pass about an unarmed wall:\n%s", out)
	}
	// And not the false FINDING either: the employer's slots are foreign to
	// posse, and a finding here would prescribe `posse gates install-hooks`,
	// the one write ADR 0052 says not to attempt.
	if strings.Contains(out, "FINDING") {
		t.Errorf("the employer's own hooks were reported as posse's wall gone missing:\n%s", out)
	}
	// INSTALL.md §9 shows this verdict to an operator who has to decide
	// whether their managed box is covered. Pinned against what the script
	// just PRINTED rather than against a line copied into the test, so a
	// reworded verdict reds here instead of leaving the recipe quietly
	// describing output nobody emits any more (the two lines that carry no
	// path — the managed line itself names a directory this fixture invents).
	doc, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "verify-hook-freshness: ") {
			continue
		}
		if !strings.Contains(string(doc), line) {
			t.Errorf("INSTALL.md does not show the verdict the script prints:\n  %s", line)
		}
	}
}

// THE ARM THE FIX IS FOR. One repo on the managed box keeps a hooks path of
// its own, and its hook is behind the binary. The reference render — taken
// with the redirect env in force — is the only thing that can see that, so
// this arm is red for any regression that lets the throwaway repo inherit the
// managed global again.
func TestQAHookFreshnessStillCatchesAStaleHookOnAManagedBox(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private", "pub": "public"})
	r.manage(t)
	r.escape(t, "pub")

	// The same drift the control was built for: a body carrying our marker,
	// still refusing, whose refusal prescribes the bare two-dot `git diff`.
	body := hfRead(t, r.hook("pub"))
	stale := strings.Replace(body, "git diff HEAD -- <paths>", "git diff", 1)
	if stale == body {
		t.Fatal("rig never built: the render no longer carries the erba paragraph to stale")
	}
	hfWrite(t, r.hook("pub"), stale)

	out, code := r.run(t)
	if code != 1 {
		t.Fatalf("a stale hook on the repo that escaped the managed path is a finding; got exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "prepare-commit-msg is STALE") {
		t.Errorf("the stale hook was not named:\n%s", out)
	}
	// Both halves in one pass: the managed repo skipped, the escaping one
	// measured. A fix that only classified repos would report the managed
	// one and still measure nothing here.
	if !strings.Contains(out, "1 repo(s) dispatch from a managed hooks path") {
		t.Errorf("the managed repo was not classified:\n%s", out)
	}
	if strings.Contains(out, "nothing measured") {
		t.Errorf("the reference render did not escape the managed global:\n%s", out)
	}
}

// The reference render must come from a directory this script owns, and be
// SHOWN to. The reach here is env-borne, and ADR 0052 M3 measured that an
// env-borne redirect is SHED with the environment: a git older than the
// config-in-env form (< 2.31), or any wrapper standing where git is looked up
// that does not pass the environment on, leaves the render landing in the
// managed directory's shadow — and the identity compare would then be against
// whatever was read from somewhere else. So the script asks git where it will
// dispatch, by the same `--git-path hooks` lookup posse's own hooksDir asks,
// and refuses to measure when the answer is not its own directory.
//
// The fixture is a shim on the front of PATH that scrubs GIT_CONFIG_* and
// execs the real git — a stand-in for every one of those, and the shape of an
// L1 gate shim, which is a thing that really does stand there on this box.
func TestQAHookFreshnessRefusesToMeasureWhenTheRedirectDoesNotTake(t *testing.T) {
	// MIXED, deliberately: on a fully managed box every repo is skipped and
	// the reference render is never needed, so nothing would be asserted.
	r := hfNewRig(t, map[string]string{"priv": "private", "pub": "public"})
	r.manage(t)
	r.escape(t, "pub")
	r.shimDir = hfShim(t, r, "env -u GIT_CONFIG_COUNT -u GIT_CONFIG_KEY_0 -u GIT_CONFIG_VALUE_0 -u GIT_CONFIG_KEY_1 -u GIT_CONFIG_VALUE_1 "+r.git+" \"$@\"")

	out, code := r.run(t)
	if code != 2 {
		t.Fatalf("a reference render that did not escape the managed path measured nothing; want exit 2, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "the redirect did not take, nothing measured") {
		t.Errorf("the script measured against a render it could not place:\n%s", out)
	}
	// Fail-safe, not fail-quiet: it says where git WOULD have dispatched, so
	// the reader can see it was the managed directory.
	if !strings.Contains(out, "dispatches hooks from") {
		t.Errorf("the refusal did not name the path it got:\n%s", out)
	}
}

// The operator's own GIT_CONFIG_COUNT is appended to, never clobbered: git
// reads exactly COUNT entries at indices 0..COUNT-1, so a count we overwrote
// would drop every entry they set. Observed rather than grepped — a shim logs
// the GIT_CONFIG_* it was handed.
func TestQAHookFreshnessAppendsItsRedirectToTheOperatorsConfigCount(t *testing.T) {
	r := hfNewRig(t, map[string]string{"pub": "public"})
	log := filepath.Join(r.home, "git-config-env.log")
	r.shimDir = hfShim(t, r, "env | grep '^GIT_CONFIG_' >> "+log+"\nexec "+r.git+" \"$@\"")
	// One entry already in the environment, at index 0. Harmless in itself
	// (it is the value git would use anyway) and it is the INDEX that is
	// under test.
	r.extraEnv = []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.abbrev",
		"GIT_CONFIG_VALUE_0=12",
	}

	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("an operator config entry must not break the render; got exit %d:\n%s", code, out)
	}
	got := hfRead(t, log)
	for _, want := range []string{
		"GIT_CONFIG_COUNT=2",              // theirs plus ours
		"GIT_CONFIG_KEY_0=core.abbrev",    // theirs, kept
		"GIT_CONFIG_KEY_1=core.hooksPath", // ours, appended after it
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the redirect did not append to the operator's count: missing %q in:\n%s", want, got)
		}
	}
}

// A skip is the one wrong answer that is silent: it reports no finding about
// a repo it never read. So the classification is trusted only when the binary
// answers with the verdict's OWN line — a posse predating the query verb reads
// `managed-hooks` as a persona name, and whatever that path exits with is not
// a statement about any hooks directory. Here it exits 0, the worst case, and
// the box must still be measured exactly as it was before.
func TestQAHookFreshnessDoesNotSkipEveryRepoOnABinaryWithoutTheQuery(t *testing.T) {
	r := hfNewRig(t, map[string]string{"pub": "public"})
	// Everything forwards to the real binary except the query, which answers
	// the way a persona lookup that found nothing does.
	old := filepath.Join(r.home, "posse-old")
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = gates ] && [ \"$2\" = managed-hooks ]; then\n" +
		"  echo \"posse: no such agent: managed-hooks\"; exit 0\n" +
		"fi\n" +
		"exec " + r.bin + " \"$@\"\n"
	if err := os.WriteFile(old, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	r.bin = old

	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("an old binary must leave the control measuring as it did before; got exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "1 repo(s) match this binary's render") {
		t.Errorf("the repo was skipped rather than measured:\n%s", out)
	}
	if strings.Contains(out, "dispatch from a managed hooks path") {
		t.Errorf("`no such agent` was read as a managed verdict:\n%s", out)
	}
}

// ─── the reference is per repo (ranger-base-x5olh) ───────────────────────────
//
// THE DEFECT. The reference was ONE render, into a throwaway repo, for the
// whole box. But check 3's identity literals (ADR 0024 D2) are derived from
// the repo the hook is for, so the render legitimately differs between two
// repos on the same box — and the identity compare normalized exactly one line
// away, the visibility stamp. Whichever side of such a branch the throwaway
// repo landed on, every repo on the other side read STALE forever. MEASURED
// 2026-09-01, minutes after the operator hand-typed `install-hooks` into all
// four configured repos: three of the four reported
// "prepare-commit-msg is STALE", the three carrying a `.beads/redirect`. The
// control's own header says a control that cries wolf in the constitution repo
// is the one place it must not, and it was crying wolf in three.
//
// Two arms, because there are two live sources and neither implies the other —
// a fix that normalized the redirect lines away would leave the second one
// firing, at the exact moment the operator sets up the flow-in checkout ADR
// yqstz describes:
//
//   - a .beads/redirect adds `posse_check 'instance-path'` and
//     `'instance-path-abs'`, at all three call sites.
//   - `git config --get-all user.email` reads EVERY scope, so a repo-local
//     contribution address adds a literal the box's other repos do not carry
//     (ranger-base-yqstz is the setup that requires it).
//
// Both arms plant the property BEFORE the hooks are rendered, so the installed
// hook is genuinely fresh and the only thing that could call it stale is the
// reference. Both assert the fixture actually differs from its sibling first:
// an arm where the plant did not reach the render would be green about
// nothing.

// hfBeadsRedirect is a prep that gives one named repo a .beads/redirect, the
// way an instance's own checkouts point at the beads repo.
func hfBeadsRedirect(who, target string) func(*testing.T, string, string) {
	return func(t *testing.T, name, path string) {
		t.Helper()
		if name != who {
			return
		}
		if err := os.MkdirAll(filepath.Join(path, ".beads"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, ".beads", "redirect"), []byte(target+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestQAHookFreshnessDoesNotCryWolfOverARedirectLiteral(t *testing.T) {
	r := hfNewRig(t, map[string]string{"redir": "private", "plain": "public"},
		hfBeadsRedirect("redir", "/nowhere/instance/.beads"))

	// The fixture's premise: the two renders DIFFER, and differ in exactly
	// the class this arm is about. Without this the arm passes on a box where
	// nothing derives the literal, which is the box the defect hid on.
	got, other := hfRead(t, r.hook("redir")), hfRead(t, r.hook("plain"))
	if !strings.Contains(got, "posse_check 'instance-path'") {
		t.Fatalf("rig never built: the redirect repo's render carries no instance-path literal")
	}
	if strings.Contains(other, "posse_check 'instance-path'") {
		t.Fatalf("rig never built: the repo with no .beads/redirect carries one anyway — nothing here varies")
	}

	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("two freshly rendered repos that differ only in their own identity literals must both be fresh; got exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "is STALE") {
		t.Errorf("a per-repo identity literal was read as staleness:\n%s", out)
	}
	if !strings.Contains(out, "2 repo(s) match this binary's render") {
		t.Errorf("both repos must be measured, not one:\n%s", out)
	}
}

func TestQAHookFreshnessDoesNotCryWolfOverARepoLocalEmail(t *testing.T) {
	const addr = "contrib@example.org"
	r := hfNewRig(t, map[string]string{"local": "private", "plain": "public"},
		func(t *testing.T, name, path string) {
			t.Helper()
			if name != "local" {
				return
			}
			cmd := exec.Command(hfGit(t), "-C", path, "config", "user.email", addr)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git config user.email: %v %s", err, out)
			}
		})

	// The literal renders as an escaped ERE, so the fixture looks for the
	// shape the renderer writes rather than the address as typed.
	const literal = `posse_check 'email' 'contrib@example\.org'`
	got, other := hfRead(t, r.hook("local")), hfRead(t, r.hook("plain"))
	if !strings.Contains(got, literal) {
		t.Fatalf("rig never built: the repo-local address did not reach the render")
	}
	if strings.Contains(other, literal) {
		t.Fatalf("rig never built: a repo with no local address carries it anyway — nothing here varies")
	}

	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("a repo-local contribution address is not staleness; got exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "is STALE") {
		t.Errorf("a per-repo email literal was read as staleness:\n%s", out)
	}
}

// AND THE OTHER DIRECTION. A per-repo reference that only ever made findings
// go away would be a normalization with extra steps — the same surrender the
// stamp's sed is, spelled as a render. It is not: because the reference
// carries the repo's identity as it is NOW, a repo whose identity moved AFTER
// its hook was written is caught, and that hook really is behind its repo. The
// literal check 3 would have walled the new address with is not in it, which
// is the ranger-base-yqstz leak the class exists to close. Nothing on the box
// asks this question today (measured 2026-09-05: no configured repo carries a
// repo-local user.email), which is exactly why it needs a pin.
func TestQAHookFreshnessCatchesAnIdentityThatMovedAfterTheRender(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	before := hfRead(t, r.hook("priv"))
	out, err := exec.Command(hfGit(t), "-C", r.repos["priv"], "config", "user.email", "later@example.org").CombinedOutput()
	if err != nil {
		t.Fatalf("git config user.email: %v %s", err, out)
	}
	// The hook is untouched — the plant is in the repo, not in the file, and
	// an arm that had rewritten the hook would be the STALE arm again.
	if hfRead(t, r.hook("priv")) != before {
		t.Fatal("rig never built: the plant changed the hook itself")
	}

	got, code := r.run(t)
	if code != 1 {
		t.Fatalf("a hook that predates the repo's identity is stale; want exit 1, got %d:\n%s", code, got)
	}
	if !strings.Contains(got, "prepare-commit-msg is STALE") {
		t.Errorf("the drift must be named as staleness:\n%s", got)
	}
}

// The reference render is now `posse gates install-hooks` aimed at the
// OPERATOR'S repo, and the one thing standing between that and a hook rewrite
// in someone else's checkout is the core.hooksPath redirect it is taken under.
// That is a property of where bytes land, which the source-string ban this
// replaced could not see, so it is measured: every file under the repo is
// hashed before and after a full run, .git included.
//
// The fixture is the repo shape that makes the render derive something —
// a .beads/redirect — because a render that derived nothing could be landing
// anywhere and this arm would not know.
func TestQAHookFreshnessWritesNothingIntoTheRepoItMeasures(t *testing.T) {
	r := hfNewRig(t, map[string]string{"redir": "private"},
		hfBeadsRedirect("redir", "/nowhere/instance/.beads"))
	repo := r.repos["redir"]

	before := hfSnapshot(t, repo)
	if len(before) == 0 {
		t.Fatal("rig never built: nothing under the repo to compare")
	}
	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("a fresh repo must exit 0, got %d:\n%s", code, out)
	}
	after := hfSnapshot(t, repo)

	for p, sum := range after {
		if was, ok := before[p]; !ok {
			t.Errorf("the control created %s in the repo it measures", p)
		} else if was != sum {
			t.Errorf("the control rewrote %s in the repo it measures", p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			t.Errorf("the control removed %s from the repo it measures", p)
		}
	}
}

// hfSnapshot hashes every regular file under dir, keyed by its path relative
// to dir. Symlinks and directories are recorded by name alone — the claim is
// about content the control could have written, and following a link would
// take the snapshot outside the repo.
func hfSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			out[rel] = "<" + d.Type().String() + ">"
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// hfShim writes a `git` on the front of PATH whose body is the given shell,
// and proves it is the one that gets found — a shim nothing resolves to is a
// fixture that plants nothing, and every arm using it would be green for the
// wrong reason.
func hfShim(t *testing.T, r *hfRig, body string) string {
	t.Helper()
	dir := filepath.Join(r.home, "shim")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", "command -v git")
	cmd.Env = []string{"PATH=" + dir + ":" + filepath.Dir(r.git) + ":/usr/bin:/bin"}
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != filepath.Join(dir, "git") {
		t.Fatalf("shim never took: git resolves to %q (%v)", strings.TrimSpace(string(out)), err)
	}
	return dir
}
