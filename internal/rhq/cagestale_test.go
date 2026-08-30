package rhq

// The cage image's third state (ranger-base-nwj7), pinned — and the gate
// the live pins use to act on it.
//
// ranger-base-jada is half an hour spent proving that a red live pin was a
// stale image and not a regression. The fix is not a rebuild; it is that
// the classification exists at all, is computed the same way everywhere,
// and cannot silently move to the other arm. So three things are pinned
// here, and each has a wrong arm that fails:
//
//   - the classification itself, including that "cannot tell" is NEITHER
//     current nor stale;
//   - what a live pin DOES with it — a stale image SKIPS, a current one
//     RUNS, and an unclear one runs too. Driven through the real
//     CageImagePosse against a fake engine, so the plumbing from an
//     engine's `inner:` to the decision is executed and not assumed;
//   - that the stamp `posse cage build` passes actually lands in the
//     binary. go does not diagnose an -X naming a symbol that does not
//     exist: it stamps nothing and exits 0, so a typo would leave every
//     image calling itself "+dev" and every comparison unclear forever.
//     Only a build can answer that, so this file builds one.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the gate the live pins use ──────────────────────────────────────────────

// requireCurrentCageImage is the third state at the one place a live pin
// can act on it. An image that is not the one this source builds cannot
// answer a question about this source's render, so the pin skips with both
// idents named rather than failing one clause of a claim it never measured.
//
// Called only where the assertions really read the inner render, and
// deliberately NOT from the shared live guards: a skip that also fired for
// the pins whose subject is host-side — the mounts, the remount refusal,
// the parity wiring — would buy the classification back by never running,
// which is the other way to lose a live pin.
func requireCurrentCageImage(t *testing.T, a *App, e *Engine, image string) {
	t.Helper()
	cageAgeGate(t, a.CageAgeVsSource(e, image, PosseCheckout(".")))
}

// cageAgeGate is what a live pin does with the classification, apart from
// how the classification was obtained — so the decision can be executed in
// a test that has no docker.
func cageAgeGate(t *testing.T, g CageAge) {
	t.Helper()
	if g.Stale() {
		t.Skip(g)
	}
	t.Log(g)
}

// ─── the classification ──────────────────────────────────────────────────────

func TestCageAgeIsThreeStatesAndUnclearIsNotStale(t *testing.T) {
	for _, c := range []struct {
		name, has, want, state string
	}{
		{"same build", "0.4.0+7e92337", "0.4.0+7e92337", CageImageCurrent},
		{"different commit", "0.4.0+0c0607b", "0.4.0+7e92337", CageImageStale},
		{"different version", "0.3.0+7e92337", "0.4.0+7e92337", CageImageStale},
		{"dirty on one side only", "0.4.0+7e92337", "0.4.0+7e92337-dirty", CageImageStale},
		{"image could not be asked", "", "0.4.0+7e92337", CageImageUnclear},
		{"source could not be read", "0.4.0+7e92337", "", CageImageUnclear},
		{"neither side", "", "", CageImageUnclear},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := cageAge("posse-cage:latest", "this source", c.has, c.want)
			if g.State != c.state {
				t.Errorf("(%q vs %q) is %s, want %s", c.has, c.want, g.State, c.state)
			}
			// The one that matters most: an image nobody could ask is not
			// stale. A skip on a probe failure is how a live pin goes
			// silently green and stays there.
			if g.Stale() != (c.state == CageImageStale) {
				t.Errorf("Stale() is %v for state %s", g.Stale(), g.State)
			}
			// Whatever the state, the line names both sides and the image,
			// because it is the whole of what a reader gets in one glance.
			if s := g.String(); !strings.Contains(s, "posse-cage:latest") {
				t.Errorf("the line must name the image: %s", s)
			}
		})
	}
	// And the stale line says what to do about it, which is the difference
	// between a classification and an instruction.
	stale := cageAge("posse-cage:latest", "this source", "0.4.0+aaaaaaa", "0.4.0+bbbbbbb").String()
	for _, want := range []string{"STALE", "posse cage build", "0.4.0+aaaaaaa", "0.4.0+bbbbbbb"} {
		if !strings.Contains(stale, want) {
			t.Errorf("the stale line must name %q:\n%s", want, stale)
		}
	}
}

// ─── what a live pin does with it ────────────────────────────────────────────

// fakeCageEngine is an engine with no docker behind it: its image probe
// passes and its `inner:` runs a script printing the line a `posse version`
// inside the image would print. Everything from the engine template to the
// decision is then real — only the container is not.
func fakeCageEngine(t *testing.T, a *App, name, versionLine string) *Engine {
	t.Helper()
	script := filepath.Join(t.TempDir(), "engine")
	// argv reaching it is `<script> <image> posse version`; it answers for
	// the image without caring, because what is pinned here is the walk
	// from the answer to the decision.
	body := "#!/bin/sh\n"
	if versionLine != "" {
		body += "printf '%s\\n' " + shQuote(versionLine) + "\n"
	} else {
		body += "exit 1\n" // an image that cannot answer at all
	}
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.CagesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "command: " + script + " {image} {cmd}\nprobe: true\ninner: " + script + " {image} {cmd}\n"
	if err := os.WriteFile(filepath.Join(a.CagesDir(), name+".yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_engine: "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := a.LoadEngine(name)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func cageStaleApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	return &App{
		Home: home, ConfigPath: filepath.Join(home, "config.yaml"),
		EnvsDir: filepath.Join(home, "envs"), StateDir: filepath.Join(home, "state"),
		AgentsDir: filepath.Join(home, "agents"),
	}
}

func TestAStaleImageSkipsTheLivePinAndACurrentOneRunsIt(t *testing.T) {
	src := tempGitTree(t)
	want := SourceBuildVersion(src)
	if want == "" {
		t.Fatal("the fixture checkout must name a build, or this pins nothing")
	}
	for _, c := range []struct {
		name, line string
		run        bool
	}{
		// The image this source builds: the pin's subject is present, so
		// the pin RUNS and a failure in it is a regression.
		{"current image runs the pin", "posse " + want + " (herdr-native)", true},
		// An older image: the render being asserted is not in there.
		{"stale image skips the pin", "posse 0.1.0+0c0607b (herdr-native)", false},
		// An image that cannot be asked is not evidence of staleness, so
		// the pin runs exactly as it did before any of this existed.
		{"unaskable image still runs the pin", "", true},
		// …and neither is an answer that is not a version line.
		{"unparseable answer still runs the pin", "bash: posse: command not found", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := cageStaleApp(t)
			e := fakeCageEngine(t, a, "fake", c.line)
			ran := false
			ok := t.Run("pin", func(t *testing.T) {
				cageAgeGate(t, a.CageAgeVsSource(e, "posse-cage:latest", src))
				ran = true
			})
			if !ok {
				t.Fatal("the gate FAILED the pin — the whole point is that staleness is not a failure")
			}
			if ran != c.run {
				t.Errorf("the pin ran = %v, want %v (image said %q, source builds %q)", ran, c.run, c.line, want)
			}
		})
	}
}

// tempGitTree is a throwaway checkout: somewhere with a commit, so
// SourceBuildStamp has a real sha to read and the fixture is not the suite's
// own tree.
func tempGitTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		// PathOutsideGates: this suite runs behind the L1 shims, and the
		// fixture's commits are not what any of them are about.
		c.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "--", "f")
	run("commit", "-qm", "one", "--", "f")
	return dir
}

// ─── the source side ─────────────────────────────────────────────────────────

func TestSourceBuildStampNamesTheCommitEvenFromAWorktree(t *testing.T) {
	src := tempGitTree(t)
	git := func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	head := git(src, "rev-parse", "HEAD")
	if got, want := SourceBuildStamp(src), head[:7]; got != want {
		t.Errorf("stamp of a clean checkout is %q, want the short sha %q", got, want)
	}
	if got, want := SourceBuildVersion(src), Version+"+"+head[:7]; got != want {
		t.Errorf("version of a clean checkout is %q, want %q", got, want)
	}

	// The reason this is composed here at all instead of read out of go's
	// own build info: go looks for a `.git` DIRECTORY, so a build from a
	// LINKED WORKTREE carries no vcs stamp at all and names no commit
	// (ranger-base-bzu; re-measured go1.26.5, 2026-08-30). Every persona
	// works in a worktree, so if this followed go the answer there would be
	// "+dev" — which two different worktrees would BOTH give, and two
	// images built from them would compare as the same build.
	wt := filepath.Join(t.TempDir(), "wt")
	git(src, "worktree", "add", "-q", wt)
	t.Cleanup(func() { git(src, "worktree", "remove", "--force", wt) })
	if got, want := SourceBuildStamp(wt), head[:7]; got != want {
		t.Errorf("stamp of a linked worktree is %q, want the short sha %q — go stamps nothing here, which is the whole reason this is computed", got, want)
	}

	// An edit makes the ident stop claiming to be the commit it sits on —
	// the Makefile's spelling, and go's own meaning for "+dirty" — and a
	// SECOND, DIFFERENT edit at the same HEAD must stamp differently
	// (ranger-base-b6fh: a bare "-dirty" bit could not tell them apart, so
	// an image built at the first edit read as still current against the
	// second). cagestaledirty_qa_test.go pins the false-CURRENT read this
	// closes; this is the stamp shape itself.
	if err := os.WriteFile(filepath.Join(src, "untracked"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stampA := SourceBuildStamp(src)
	if prefix := head[:7] + "-dirty-"; !strings.HasPrefix(stampA, prefix) || len(stampA) != len(prefix)+8 {
		t.Errorf("stamp of an edited checkout is %q, want %q plus an 8-hex-char fingerprint", stampA, prefix)
	}
	if err := os.WriteFile(filepath.Join(src, "untracked"), []byte("x\ndifferent content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stampB := SourceBuildStamp(src); stampB == stampA {
		t.Errorf("two different dirty edits at the same HEAD both stamped %q — a moved target reads as still", stampB)
	}

	// And a directory that is not a checkout names nothing, which is
	// unclear rather than stale.
	if got := SourceBuildStamp(t.TempDir()); got != "" {
		t.Errorf("a non-checkout stamps %q, want \"\"", got)
	}
}

// ─── the stamp actually reaching the binary ──────────────────────────────────

// The load-bearing half of `posse cage build`'s stamp, and the only one a
// build can answer: `go build -ldflags -X <symbol>=<stamp>` is silent when
// <symbol> does not exist — it stamps nothing, exits 0, and every image
// then reports "+dev". So this builds cmd/posse the way BuildCageImage
// does and asks the binary what it thinks it is.
func TestTheCageBuildStampReachesTheBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds cmd/posse")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	build := func(stamp string) string {
		t.Helper()
		bin := filepath.Join(t.TempDir(), "posse")
		c := exec.Command("go", cagePosseBuildArgv(bin, stamp)...)
		c.Dir = root
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("go build %v: %v\n%s", cagePosseBuildArgv(bin, stamp), err, b)
		}
		b, err := exec.Command(bin, "version").Output()
		if err != nil {
			t.Fatalf("posse version: %v", err)
		}
		return posseVersionWord(string(b))
	}
	// The exact string the comparison expects, produced by the binary the
	// image would carry.
	const stamp = "deadbee"
	if got, want := build(stamp), Version+"+"+stamp; got != want {
		t.Errorf("a build stamped %q reports %q, want %q — an -X for a symbol that does not exist stamps nothing and exits 0", stamp, got, want)
	}
	// The wrong arm, so a stamp that never lands cannot pass the line
	// above by accident: with nothing stamped the binary must NOT say it.
	if got := build(""); got == Version+"+"+stamp {
		t.Errorf("an unstamped build also reports %q — the assertion above is measuring something else", got)
	}
	// …and SourceBuildVersion is exactly what a stamped build prints, which
	// is what makes comparing them meaningful.
	if got, want := build(SourceBuildStamp(root)), SourceBuildVersion(root); got != want {
		t.Errorf("a build stamped from this checkout reports %q, want SourceBuildVersion's %q", got, want)
	}
}

// posseVersionWord is what the host reads back out of the image; it must
// not invent a version out of a line that is not one.
func TestPosseVersionWordReadsOnlyAVersionLine(t *testing.T) {
	for _, c := range []struct{ out, want string }{
		{"posse 0.4.0+0c0607b (herdr-native)\n", "0.4.0+0c0607b"},
		{"warning: something\nposse 0.4.0+dev (herdr-native)\n", "0.4.0+dev"},
		{"", ""},
		{"posse\n", ""},
		{"bash: posse: command not found\n", ""},
		{"Unable to find image 'posse-cage:latest' locally\n", ""},
	} {
		if got := posseVersionWord(c.out); got != c.want {
			t.Errorf("posseVersionWord(%q) = %q, want %q", c.out, got, c.want)
		}
	}
}

// Which side `posse cage` compares against, since the two answer different
// questions: a checkout is what a `posse cage build` typed there would put
// in the image, and outside one the running binary is the only thing there
// is to be behind.
func TestCageAgeHereComparesAgainstTheCheckoutWhenThereIsOne(t *testing.T) {
	a := cageStaleApp(t)
	e := fakeCageEngine(t, a, "fake", "posse 0.0.1+aaaaaaa (herdr-native)")
	const image = "posse-cage:latest"

	src := PosseCheckout(".")
	if src == "" || !IsPosseSource(src) {
		t.Fatalf("the suite must run inside a posse checkout for this to mean anything (got %q)", src)
	}
	if g := a.CageAgeHere(e, image, "."); g.Whose != "this source" || g.Want != SourceBuildVersion(src) {
		t.Errorf("inside a posse checkout the comparison is %s = %q, want this source = %q", g.Whose, g.Want, SourceBuildVersion(src))
	}
	// A git repository that is not a posse checkout is not something `posse
	// cage build` would accept, so it is not what the image is measured
	// against either.
	for _, dir := range []string{t.TempDir(), tempGitTree(t)} {
		if g := a.CageAgeHere(e, image, dir); g.Whose != "this posse" || g.Want != VersionString() {
			t.Errorf("outside a posse checkout (%s) the comparison is %s = %q, want this posse = %q", dir, g.Whose, g.Want, VersionString())
		}
	}
}
