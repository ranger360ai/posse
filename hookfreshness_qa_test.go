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
	"os"
	"os/exec"
	"path/filepath"
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
}

func hfNewRig(t *testing.T, vis map[string]string) *hfRig {
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

func (r *hfRig) run(t *testing.T) (string, int) {
	t.Helper()
	abs, err := filepath.Abs(hfScript)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(abs)
	cmd.Env = []string{
		"HOME=" + r.home,
		"RHQ_HOME=" + r.rhqHome,
		"POSSE=" + r.bin,
		"PATH=" + filepath.Dir(r.git) + ":/usr/bin:/bin",
	}
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
// way through` in ~/src/ranger-base and ~/src/posse, the two configured
// repos that carry class paths, and in neither of the two that do not.
//
// The rig's other repos have no commits, where an empty index is what git
// itself would hand the slot — which is why every pin above stayed green
// through the defect and none of them could have caught it. This one commits
// a class member first.
func TestQAHookFreshnessDoesNotCallTheSafeFormRefusedOverAClassPathInHEAD(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
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
		{"-C", repo, "config", "user.email", "t@t"},
		{"-C", repo, "config", "user.name", "t"},
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
	for _, banned := range []string{"install-hooks \"$repo\"", "install-hooks \"$m\"", "cp \"$ref"} {
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
