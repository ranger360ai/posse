//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-olr0g: the shared caches (`~/.cache`, `~/.npm`) left
// the caged write set, because later builds and tools execute what they hold
// (wallseatcache.go has the measured poisoning and the cost). A seat builds
// through a cache of its own, which only its own wall grants.
//
// Same split as the 6akzw pins: render pins at both mechanisms from any host,
// and a live pin that hands the render to the real bwrap and tries the plant.
//
//	go test ./internal/posse -run 'WallSeatCache' -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scFixture is weFixture plus the shared caches as a live box has them.
func scFixture(t *testing.T) weFixture {
	t.Helper()
	f := weFixtureIn(t, wallUngrantedDir(t))
	for _, d := range []string{
		".cache/go-build/aa", ".npm/_npx", ".npm/_cacache", ".cache/mise/downloads-cas",
		".cache/beads/wasm", ".cache/posse/plan-usage", ".cache/posse/bd-write",
		"Library/Caches/go-build",
	} {
		if err := os.MkdirAll(filepath.Join(f.home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.home, ".cache/posse/suite-slot.1.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

// No GOOS's list names a shared cache any more, and both name the lock dir.
func TestQAWallSeatCacheHomeGrantsNameNoSharedCache(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"darwin", "linux"} {
		g := homeCacheGrants(goos)
		for _, d := range g {
			switch d {
			case ".cache", ".npm", "Library/Caches", ".cache/go-build", "Library/Caches/go-build":
				t.Errorf("homeCacheGrants(%s) names %q — a cache later builds execute out of (ranger-base-olr0g)", goos, d)
			}
		}
		if !containsString(g, ".cache/posse") {
			t.Errorf("homeCacheGrants(%s) = %v and does not name `.cache/posse` — the suite slots and bd write locks fail OPEN when they cannot be opened", goos, g)
		}
	}
}

// The write set: the seat's own cache in, the shared ones out, plan-usage
// carved read-only over the lock-dir grant.
func TestQAWallSeatCacheWriteSetGrantsOwnCacheOnly(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := scFixture(t)
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, f.rt.StateDirs...)
	own := SeatCacheDir(f.home, f.dir)
	if own == "" || !strings.HasPrefix(own, filepath.Join(f.home, ".cache", "posse-seat")+string(filepath.Separator)) {
		t.Fatalf("SeatCacheDir = %q, want a dir under <home>/.cache/posse-seat", own)
	}
	if !writeGranted(w, absResolve(own)) {
		t.Errorf("the seat's own cache %s is not granted — its builds would fail: %v", own, w)
	}
	for _, p := range []string{
		filepath.Join(f.home, ".cache", "go-build", "aa", "x-d"),
		filepath.Join(f.home, ".npm", "_npx", "x"),
		filepath.Join(f.home, ".npm", "_cacache", "x"),
		filepath.Join(f.home, ".cache", "mise", "downloads-cas", "x"),
		filepath.Join(f.home, ".cache", "beads", "wasm", "x"),
		filepath.Join(f.home, "Library", "Caches", "go-build", "x"),
		SeatCacheDir(f.home, filepath.Join(filepath.Dir(f.dir), "other-seat")),
	} {
		if writeGranted(w, absResolve(p)) {
			t.Errorf("the writable set reaches %s — a cache another process executes out of (ranger-base-olr0g)\n set: %v", p, w)
		}
	}
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, f.rt.StateDirs...)
	pu := absResolve(filepath.Join(f.home, ".cache", "posse", "plan-usage"))
	if !containsString(c.Deny, pu) || c.FileShaped(pu) {
		t.Errorf("plan-usage is not a directory deny over the lock-dir grant: deny %v", c.Deny)
	}
}

// One per session dir: two dirs of one name in two repos get two caches.
func TestQAWallSeatCacheDirIsPerSessionDir(t *testing.T) {
	t.Parallel()
	a := SeatCacheDir("/h", "/r1/wt/devops-x")
	b := SeatCacheDir("/h", "/r2/wt/devops-x")
	if a == b {
		t.Errorf("SeatCacheDir names one cache for two session dirs: %s", a)
	}
	if !strings.HasPrefix(filepath.Base(a), "devops-x-") {
		t.Errorf("SeatCacheDir = %s, want the session dir's base in the name for a reader", a)
	}
	if SeatCacheDir("", "/x") != "" || SeatCacheDir("/h", "") != "" {
		t.Error("SeatCacheDir with no home or no session dir must name nothing")
	}
}

// Both mechanisms point the tools at the dir the write set grants.
func TestQAWallSeatCacheEnvRidesBothMechanisms(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := scFixture(t)
	own := SeatCacheDir(f.home, f.dir)
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(r.Argv, "\x00")
	for _, v := range seatCacheEnv(own) {
		if !strings.Contains(argv, "--setenv\x00"+v.Key+"\x00"+v.Value) {
			t.Errorf("bwrap argv sets no %s=%s: %q", v.Key, v.Value, r.Argv)
		}
	}
	w, err := f.a.renderWallOn(WallSandboxExec, f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "/usr/bin/env " + shellQuote("GOCACHE="+filepath.Join(own, "go-build")) + " "
	if !strings.HasPrefix(w.Prefix, want) || !strings.HasSuffix(w.Prefix, SeatbeltPrefix(w.Artifact)) {
		t.Errorf("sandbox-exec prefix = %q, want it to start %q and end with the profile's own prefix", w.Prefix, want)
	}
}

// The live join: the rendered wall, the real bwrap, a plant at every row.
func TestLiveWallSeatCacheRefusesSharedCachePlants(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	bwrapNestSkip(t)
	f := scFixture(t)
	other := SeatCacheDir(f.home, filepath.Join(filepath.Dir(f.dir), "other-seat"))
	if err := os.MkdirAll(filepath.Join(other, "go-build"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	try := func(script string) (bool, string) {
		cmd := exec.Command("bwrap", append(append([]string{}, r.Argv...), "/bin/sh", "-c", script)...)
		out, err := cmd.CombinedOutput()
		return err == nil, strings.TrimSpace(string(out))
	}
	q := shellQuote
	h := func(p string) string { return q(filepath.Join(f.home, p)) }
	refused := map[string]string{
		"go build cache plant":     ": > " + h(".cache/go-build/aa/x-d"),
		"go build cache new dir":   "mkdir " + h(".cache/go-build/bb"),
		"npx package plant":        "mkdir -p " + h(".npm/_npx/x"),
		"npm cacache plant":        ": > " + h(".npm/_cacache/x"),
		"mise download plant":      ": > " + h(".cache/mise/downloads-cas/x"),
		"bd wasm plant":            ": > " + h(".cache/beads/wasm/x"),
		"new cache beside them":    "mkdir " + h(".cache/newtool"),
		"plan-usage plant":         ": > " + h(".cache/posse/plan-usage/x.json"),
		"another seat's cache":     ": > " + q(filepath.Join(other, "go-build", "x")),
		"shared cache via GOCACHE": "GOCACHE=" + h(".cache/go-build") + " && : > \"$GOCACHE/aa/y\"",
	}
	for name, script := range refused {
		if ok, out := try(script); ok {
			t.Errorf("%s: ALLOWED under the rendered wall (ranger-base-olr0g row)\n script: %s\n out: %s", name, script, out)
		}
	}
	own := SeatCacheDir(f.home, f.dir)
	allowed := map[string]string{
		"own GOCACHE write":      "test \"$GOCACHE\" = " + q(filepath.Join(own, "go-build")) + " && mkdir -p \"$GOCACHE/aa\" && : > \"$GOCACHE/aa/x-d\"",
		"own npm cache write":    "test \"$npm_config_cache\" = " + q(filepath.Join(own, "npm")) + " && mkdir -p \"$npm_config_cache/_cacache\"",
		"own scratch via env":    "test \"$" + seatCacheEnvVar + "\" = " + q(own) + " && mkdir -p \"$" + seatCacheEnvVar + "/posse-tmp-1\"",
		"suite slot lock append": ": >> " + h(".cache/posse/suite-slot.1.lock"),
		"bd write lock create":   ": > " + h(".cache/posse/bd-write/0123456789abcdef.lock"),
		"plan-usage read":        "ls " + h(".cache/posse/plan-usage"),
	}
	for name, script := range allowed {
		if ok, out := try(script); !ok {
			t.Errorf("%s: REFUSED under the rendered wall — the seat needs this\n script: %s\n out: %s", name, script, out)
		}
	}
}
