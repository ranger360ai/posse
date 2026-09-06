//go:build !posse_arm2 && !posse_arm3

package posse

// Live pin for ranger-base-yu5 — ADR 0014's verification 3, run against the
// real engine. cageoverlay_test.go proves what is *rendered*; the probe
// script (docs/adr/0014-path-scoped-writes.probe.sh) proves the engine can
// hold an overlapping bind at all. This is the join of the two, and it is
// the only one of the three that can fail when both halves are right:
//
//	the mount list posse actually renders, handed to the real engine,
//	really denies the subtree the PID denied and really opens the one it
//	left open.
//
//	RHQ_LIVE_DOCKER=1 go test ./internal/rhq -run TestLiveCageOverlay -v
//
// Needs the engine on PATH. It needs no cage image and spends no API turn:
// every claim here is about the environment the runtime is handed, so the
// "runtime" is `sh`.
//
// Measured 2026-08-29, macOS 26.4.1, Docker 29.0.1 (VirtioFS).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// liveOverlayEngine is the built-in docker template with the TUI flags
// dropped, derived from the built-in rather than retyped so a change to it
// cannot leave this test driving an engine line nobody launches.
func liveOverlayEngine(t *testing.T, a *App) *Engine {
	t.Helper()
	d := builtinEngines[0]
	os.MkdirAll(a.CagesDir(), 0o755)
	if err := os.WriteFile(filepath.Join(a.CagesDir(), "notty.yaml"), []byte(strings.Join([]string{
		"command: " + strings.Replace(d.Command, " -i -t", "", 1),
		"mount: " + d.Mount, "mount_ro: " + d.MountRO, "env: " + d.Env, "env_set: " + d.EnvSet,
		"home: " + d.Home, "probe: " + d.Probe, "inner: " + d.Inner, "",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_engine: notty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := a.LoadEngine("notty")
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLiveCageOverlayShapesHoldOnTheRealEngine(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_DOCKER") == "" {
		t.Skip("set RHQ_LIVE_DOCKER=1 (needs docker; builds no image)")
	}
	home := t.TempDir()
	a := &App{
		Home: home, ConfigPath: filepath.Join(home, "config.yaml"),
		EnvsDir: filepath.Join(home, "envs"), StateDir: filepath.Join(home, "state"),
		AgentsDir: filepath.Join(home, "agents"),
	}
	e := liveOverlayEngine(t, a)
	if why := a.CageEngineNotReady(e); why != "" {
		t.Skip(why) // no image question here: the engine itself is the subject
	}
	// Short and resolved: mounts are same-path in and out, and a fixture
	// mixing /var/folders with its /private real path would be testing the
	// symlink rather than the boundary.
	dir := shortTempDir(t)
	for _, d := range []string{"internal", "docs/adr", ".beads", ".git"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The three questions, asked identically of every shape so the arms are
	// comparable. `sh -c … "$0"` rather than a cd, because the workdir the
	// engine sets is the repo and a shape that lost the repo mount would
	// then fail to start rather than answer.
	const probe = `w() { touch "$1/.probe" 2>/dev/null && echo "$2=writable" || echo "$2=refused"; }
w "$0/docs/adr" subtree
w "$0/internal" rest
w "$0/.beads" beads
w "$0/.git" git
touch "$0/.probe" 2>/dev/null && echo "root=writable" || echo "root=refused"`

	run := func(t *testing.T, front string) map[string]string {
		t.Helper()
		ag := cageAgent(t, a, front)
		if err := ag.EnsureMemoryDir(); err != nil {
			t.Fatal(err)
		}
		// The refusals spool is a FILE mount (ADR 0025 §4: never the
		// canonical log); a bind of a missing source is a directory the
		// engine makes, which is a different fixture from the one a real
		// launch has (EnsureCageSpool seeds it).
		if _, err := a.EnsureCageSpool(ag.Name, "s1"); err != nil {
			t.Fatal(err)
		}
		ms := a.CageMounts(ag, e, dir, "s1")
		argv := e.RenderArgv(CageRender{
			Name: "posse-yu5-live", Image: "busybox", Workdir: dir, Mounts: ms,
			Inner: []string{"sh", "-c", probe, dir},
		})
		c := exec.Command(argv[0], argv[1:]...)
		c.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("the cage did not run: %v\nargv: %q\n%s", err, argv, out)
		}
		got := map[string]string{}
		for _, ln := range strings.Split(string(out), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(ln), "="); ok {
				got[k] = v
			}
		}
		t.Logf("%s\nmounts:\n%s\nsaid: %v", strings.TrimSpace(front), showMounts(ms), got)
		return got
	}

	want := func(t *testing.T, got map[string]string, k, v string) {
		t.Helper()
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}

	// The CONTROL first, and it is not a formality: if the subtree were not
	// writable without the overlay, every refusal below would be the
	// fixture's and not the wall's.
	t.Run("control-no-rules", func(t *testing.T) {
		got := run(t, "cage: container\n")
		want(t, got, "subtree", "writable")
		want(t, got, "rest", "writable")
		want(t, got, "root", "writable")
	})

	// ADR 0014 §4 bullet 1, and ADR 0014 verification 3's "denied subtree
	// not writable, the rest of a rw repo is".
	t.Run("deny-list", func(t *testing.T) {
		got := run(t, "cage: container\ndeny: [Edit(docs/adr/**), Write(docs/adr/**)]\n")
		want(t, got, "subtree", "refused")
		want(t, got, "rest", "writable")
		want(t, got, "root", "writable")
	})

	// Bullet 2, and verification 3's "`writable:` extra writable on a `:ro`
	// repo" and "touch at repo root fails" — plus the two carve-outs a `:ro`
	// repo always carries, which is what keeps claim/comment/close at this
	// tier (the bd half of that is TestLiveInnerGatesHoldInsideTheCage,
	// which runs the real inner wrapper against a real store).
	t.Run("allow-list", func(t *testing.T) {
		got := run(t, "cage: container\ndeny: [Edit, Write]\nwritable: [docs/adr]\n")
		want(t, got, "subtree", "writable")
		want(t, got, "rest", "refused")
		want(t, got, "root", "refused")
		want(t, got, "beads", "writable")
		want(t, got, "git", "writable")
	})

	// Verification 5: a PID with only the bare rule is unchanged — same
	// `:ro` repo, plus the carve-outs, and no hole opened for it.
	t.Run("bare-edit-write-still-gets-a-ro-repo", func(t *testing.T) {
		got := run(t, "cage: container\ndeny: [Edit, Write]\n")
		want(t, got, "subtree", "refused")
		want(t, got, "rest", "refused")
		want(t, got, "root", "refused")
		want(t, got, "beads", "writable")
		want(t, got, "git", "writable")
	})

	// deny-wins (ADR 0001) through the engine, which is where it could
	// silently invert: the extra is DEEPER than the deny that contains it,
	// so an implementation that out-ordered the two instead of dropping the
	// extra would open exactly the path the PID denied.
	t.Run("deny-wins-over-a-writable-extra", func(t *testing.T) {
		got := run(t, "cage: container\ndeny: [Edit, Write, Edit(docs/**)]\nwritable: [docs/adr]\n")
		want(t, got, "subtree", "refused")
		want(t, got, "root", "refused")
	})
}
