//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-53w1: scripts/verify-detection.sh replayed its fixtures through
// `herdr agent explain`, which resolves the manifest from the INSTALLED copy
// in ~/.config/herdr/agent-detection. Measured on the bug: deleting the
// `update_menu` rule from etc/herdr/agent-detection/codex.toml and running the
// script reported 9/9 fixtures OK, because the answer came from whatever the
// operator last installed. `make install-detection` copies and then verifies,
// so the thing under test had just been written over the thing it was tested
// against.
//
// The pin is the bead's own scenario, and its wrong arm is what makes it a
// pin: a complete manifest is planted in a scratch XDG_CONFIG_HOME — an
// operator who HAS installed the good copy — while the rule is cut from the
// tree the script reads. The script must fail. Revert the staging in
// verify-detection.sh and this arm goes green at 9/9, which is the bug.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// detectionRig copies the script and the manifests+fixtures it reads into a
// throwaway repo root, so an arm can cut a rule without touching the checkout.
// Returns the rig root; the script lives at <root>/scripts/verify-detection.sh
// and finds its manifests via its own `cd "$(dirname "$0")/.."`.
//
// THE ROOT COMES FROM qibRepoRoot (ADR 0078 D3, ranger-base-00fyg). It was
// `filepath.Join("..", "..")` held in this local, and the three ReadDirs
// below then read directories joined onto a bare-root identifier — a reading that
// enumerates etc/herdr/agent-detection and its testdata, which every detection
// bead writes into, and which no rule in
// internal/treepins/treewidedoor_qa_test.go could score. So this rig and its
// five callers were outside the tree-wide class by SPELLING alone, while the
// known-screen reader one door over enumerates the sibling directory and was
// in it. Taking the root from the package's one helper is what puts them in:
// `root && walk` here, propagated to the callers, doored in $(QA_SEED_PINS).
func detectionRig(t *testing.T) string {
	t.Helper()
	repo := qibRepoRoot(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	cp := func(from, to string, mode os.FileMode) {
		b, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := WriteExecutable(to, b, mode); err != nil {
			t.Fatal(err)
		}
	}
	cp(filepath.Join(repo, "scripts", "verify-detection.sh"),
		filepath.Join(root, "scripts", "verify-detection.sh"), 0o755)

	src := filepath.Join(repo, "etc", "herdr", "agent-detection")
	dst := filepath.Join(root, "etc", "herdr", "agent-detection")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	tomls := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		tomls++
		cp(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), 0o644)
	}
	if tomls == 0 {
		t.Fatalf("no manifests under %s — the rig would measure nothing", src)
	}
	agents, err := os.ReadDir(filepath.Join(src, "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	fixtures, declared := 0, 0
	for _, a := range agents {
		if !a.IsDir() {
			continue
		}
		fs, err := os.ReadDir(filepath.Join(src, "testdata", a.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range fs {
			// NO-OVERRIDE comes with the fixtures (ranger-base-72oz4). It is
			// not a .txt, so the glob above skipped it, and a rig that left
			// it behind made every claude fixture read as the ORPHAN class
			// — "no manifest" — which failed this file's own control arm
			// over a checkout that passes. The script is what the rig
			// measures, so the rig has to carry everything the script reads.
			if f.Name() == detectionNoOverride {
				declared++
				cp(filepath.Join(src, "testdata", a.Name(), f.Name()),
					filepath.Join(dst, "testdata", a.Name(), f.Name()), 0o644)
				continue
			}
			if !strings.HasSuffix(f.Name(), ".txt") {
				continue
			}
			fixtures++
			cp(filepath.Join(src, "testdata", a.Name(), f.Name()),
				filepath.Join(dst, "testdata", a.Name(), f.Name()), 0o644)
		}
	}
	if fixtures == 0 {
		t.Fatal("no fixtures copied — an empty rig passes every arm")
	}
	t.Logf("rig: %d manifests, %d fixtures, %d declaration(s)", tomls, fixtures, declared)
	return root
}

// detectionNoOverride is the per-directory declaration that an agent has no
// manifest in this checkout ON PURPOSE — the name verify-detection.sh holds in
// $NO_OVERRIDE and scripts/d3-reader-eval.py reads in declared_no_override.
const detectionNoOverride = "NO-OVERRIDE"

// runDetection runs the rig's script with install dir `installed` presented as
// the operator's XDG_CONFIG_HOME.
func runDetection(t *testing.T, root, installed string, args ...string) (out string, code int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(root, "scripts", "verify-detection.sh"), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"HOME="+filepath.Join(root, "home"),
		"XDG_CONFIG_HOME="+installed,
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("verify-detection.sh: %v\n%s", err, b)
		}
		code = exit.ExitCode()
	}
	return string(b), code
}

func TestQAVerifyDetectionFailsACommittedRuleDeletion(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root := detectionRig(t)
	manifest := filepath.Join(root, "etc", "herdr", "agent-detection", "codex.toml")
	good, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}

	// The installed copy an operator would have: complete, rule present. Every
	// arm below sees it, so a run that reads the install cannot fail.
	installed := filepath.Join(root, "installed")
	if err := os.MkdirAll(filepath.Join(installed, "herdr", "agent-detection"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "herdr", "agent-detection", "codex.toml"), good, 0o644); err != nil {
		t.Fatal(err)
	}

	// Control: an untouched tree passes. Without this the failing arm below
	// could be failing for any reason at all.
	out, code := runDetection(t, root, installed)
	if code != 0 || !strings.Contains(out, "fixtures OK") {
		t.Fatalf("intact tree: exit %d, want 0 with all fixtures OK\n%s", code, out)
	}
	if strings.Contains(out, "0/0 fixtures OK") {
		t.Fatalf("intact tree passed with no fixtures — the rig measured nothing\n%s", out)
	}

	// The bead: cut a rule from the TREE, leave the install whole. The script
	// explained against the install, so this reported every fixture OK.
	if err := os.WriteFile(manifest, []byte(cutRule(t, string(good), "update_menu")), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, installed)
	if code == 0 {
		t.Errorf("update_menu deleted from the checkout and verify-detection still passed (exit 0) — "+
			"it is reading the installed manifest, which is ranger-base-53w1\n%s", out)
	}
	if !strings.Contains(out, "blocked-update-menu") || !strings.Contains(out, "FAIL") {
		t.Errorf("the deletion did not fail the fixture written for it\n%s", out)
	}

	// And it must fail for the deletion, not because the rig's install drifted:
	// the install-side note is a note, and the tree is what sets the exit code.
	if err := os.WriteFile(manifest, good, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "herdr", "agent-detection", "codex.toml"),
		[]byte(cutRule(t, string(good), "update_menu")), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, installed)
	if code != 0 {
		t.Errorf("whole tree with a stale INSTALL failed (exit %d) — the install is reported, "+
			"not verified; `make install-detection` is what tests the install\n%s", code, out)
	}
	if !strings.Contains(out, "differs from the checkout") {
		t.Errorf("a stale install was not reported\n%s", out)
	}
}

// ranger-base-72oz4: a testdata/<agent>/ for an agent this repo ships no
// manifest for AT ALL, declared by a NO-OVERRIDE file in that directory.
// claude is the first — its first-run notice screens are captured because
// every marker in internal/posse/knownscreen.go has to be read off a shipped
// fixture (ADR 0066 D3), and posse forks no claude manifest because herdr's
// bundled one already names both screens.
//
// THE RISK THIS ARM EXISTS FOR is that the clause reads as a SKIP. Before it,
// the script failed such a fixture outright ("no manifest"), which is
// ranger-base-j66o's answer for an override being retired and is the right
// answer there. The cheap way to make claude pass would have been to stop
// looking at it, and a fixture nobody replays is the vacuous shape
// rangerhq-uglc named — green forever, measuring nothing. So the arms below
// are, in order: the fixtures are REALLY replayed (break a state, it fails),
// the declaration is REQUIRED (remove it, the orphan failure comes back), and
// the declaration EXPIRES (write the manifest, the stale claim fails).
func TestQAVerifyDetectionReplaysADeclaredNoOverrideAgent(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root := detectionRig(t)
	det := filepath.Join(root, "etc", "herdr", "agent-detection")
	installed := filepath.Join(root, "installed")

	// The rig has to actually carry a declared agent, or every arm here is
	// vacuous. Found rather than named: the class is "a testdata dir with a
	// declaration", and a second one must not need this file edited.
	agents, err := os.ReadDir(filepath.Join(det, "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	var agent string
	for _, a := range agents {
		if !a.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(det, "testdata", a.Name(), detectionNoOverride)); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(det, a.Name()+".toml")); err == nil {
			t.Fatalf("%s carries both %s and a manifest in the checkout — the declaration is stale and the script fails on it; delete one",
				a.Name(), detectionNoOverride)
		}
		agent = a.Name()
		break
	}
	if agent == "" {
		t.Skip("no testdata directory declares " + detectionNoOverride + " — nothing for this arm to measure")
	}
	decl := filepath.Join(det, "testdata", agent, detectionNoOverride)
	t.Logf("declared-no-override agent: %s", agent)

	// Control: the whole tree passes, and the declared agent's fixtures are
	// COUNTED. "17/17 OK" over a run that skipped two of them would be the
	// defect this test is for, so the fixture names are asserted present.
	fixtures, err := os.ReadDir(filepath.Join(det, "testdata", agent))
	if err != nil {
		t.Fatal(err)
	}
	var bases []string
	for _, f := range fixtures {
		if strings.HasSuffix(f.Name(), ".txt") {
			bases = append(bases, strings.TrimSuffix(f.Name(), ".txt"))
		}
	}
	if len(bases) == 0 {
		t.Fatalf("%s/testdata/%s ships a declaration and no fixtures", det, agent)
	}
	out, code := runDetection(t, root, installed)
	if code != 0 || !strings.Contains(out, "fixtures OK") {
		t.Fatalf("intact tree with a declared-no-override agent: exit %d, want 0 with all fixtures OK\n%s", code, out)
	}
	for _, base := range bases {
		if !strings.Contains(out, base) {
			t.Errorf("%s was not in the run at all — a declared agent is REPLAYED, not skipped\n%s", base, out)
		}
	}
	if !strings.Contains(out, "no override in this checkout, by declaration") {
		t.Errorf("the run did not say the agent has no override here; the rows above it read as ordinary passes against a manifest this repo wrote\n%s", out)
	}

	// Arm 1: the fixtures are really replayed. Rename one so its filename
	// claims the opposite state — the one mutation that cannot be satisfied
	// by any manifest and so can only be caught by a run that explains it.
	var flipped, from, to string
	for _, base := range bases {
		want := base
		if i := strings.Index(base, "-"); i > 0 {
			want = base[:i]
		}
		switch want {
		case "blocked":
			flipped = "idle" + strings.TrimPrefix(base, "blocked")
		case "idle":
			flipped = "blocked" + strings.TrimPrefix(base, "idle")
		default:
			continue
		}
		from = filepath.Join(det, "testdata", agent, base+".txt")
		to = filepath.Join(det, "testdata", agent, flipped+".txt")
		break
	}
	if from == "" {
		t.Fatalf("no %s fixture encodes a state in its filename, so nothing here can mutate one", agent)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, installed)
	if code == 0 {
		t.Errorf("a %s fixture renamed to claim the opposite state still passed (exit 0) — the declaration turned the replay into a skip, which is the vacuous fixture rangerhq-uglc named\n%s", agent, out)
	}
	if !strings.Contains(out, flipped) {
		t.Errorf("the mutated fixture %s was not named in the output\n%s", flipped, out)
	}
	if err := os.Rename(to, from); err != nil {
		t.Fatal(err)
	}

	// Arm 2: the declaration is required. Without it the agent is
	// ranger-base-j66o's orphan — an override retired or moved aside — and
	// that is still a failure, by the manifest it cannot name.
	body, err := os.ReadFile(decl)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(decl); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, installed)
	if code == 0 {
		t.Errorf("%s/testdata/%s with no %s and no manifest passed — an override retired out from under its fixtures is ranger-base-j66o and must still fail\n%s",
			det, agent, detectionNoOverride, out)
	}
	if !strings.Contains(out, "no manifest") {
		t.Errorf("the undeclared orphan did not fail by its missing manifest\n%s", out)
	}
	if err := os.WriteFile(decl, body, 0o644); err != nil {
		t.Fatal(err)
	}

	// Arm 3: the declaration expires. Once the manifest exists the file is a
	// claim about the tree that is no longer true, and a declaration nobody
	// retires is how this clause would become a permanent excuse. The
	// manifest's CONTENT does not matter to the claim, so the cheapest real
	// file is another agent's.
	other, err := os.ReadFile(filepath.Join(det, "codex.toml"))
	if err != nil {
		t.Fatal(err)
	}
	planted := filepath.Join(det, agent+".toml")
	if err := os.WriteFile(planted, other, 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, installed)
	if code == 0 {
		t.Errorf("%s.toml exists and %s still declares the agent has no override, and the run passed — the declaration would outlive the thing it describes\n%s",
			agent, detectionNoOverride, out)
	}
	if !strings.Contains(out, detectionNoOverride) {
		t.Errorf("the stale declaration was not named in the failure\n%s", out)
	}
	if err := os.Remove(planted); err != nil {
		t.Fatal(err)
	}
}

// The install arm ranger-base-neyn asked for, which explaining against the
// tree would otherwise drop: right after `make install-detection` the two
// copies must be byte-identical, and the target runs the script with
// --check-install so they are compared with an exit code and not a note.
// Without a mode like this, an install that lands somewhere herdr does not
// read is invisible — the fixtures pass, because they no longer touch it.
func TestQAVerifyDetectionCheckInstallFailsAStaleInstall(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root := detectionRig(t)
	manifest := filepath.Join(root, "etc", "herdr", "agent-detection", "codex.toml")
	good, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	det := filepath.Join(root, "installed", "herdr", "agent-detection")
	if err := os.MkdirAll(det, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"codex.toml", "grok.toml"} {
		b, err := os.ReadFile(filepath.Join(root, "etc", "herdr", "agent-detection", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(det, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Control: a fresh install matches, --check-install passes.
	out, code := runDetection(t, root, filepath.Join(root, "installed"), "--check-install")
	if code != 0 {
		t.Fatalf("--check-install on a matching install: exit %d, want 0\n%s", code, out)
	}

	// Drift the INSTALL only — the tree is untouched, so every fixture still
	// passes and only the install comparison can produce the failure.
	if err := os.WriteFile(filepath.Join(det, "codex.toml"),
		[]byte(cutRule(t, string(good), "update_menu")), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, filepath.Join(root, "installed"), "--check-install")
	if code == 0 {
		t.Errorf("--check-install passed with the installed codex.toml out of step with the checkout\n%s", out)
	}
	if !strings.Contains(out, "fixtures OK") {
		t.Errorf("--check-install failed on something other than the install; the tree was untouched\n%s", out)
	}

	// And the same drift is only a note without the flag: `make verify-detection`
	// on a checkout nobody installed is not a red suite.
	if out, code = runDetection(t, root, filepath.Join(root, "installed")); code != 0 {
		t.Errorf("plain run failed on install drift (exit %d) — the exit code is the tree's\n%s", code, out)
	}
}
