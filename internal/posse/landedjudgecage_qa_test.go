//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-4nzpo — the landed-check judge is never stronger than a seat.
//
// The process this protects: `posse dispatch`'s landing path →
// landedChecker.judge → boxLandedCage. The judge is landed code, run on every
// landing as no persona, and it does not go through RenderWall, so the
// host-scoped masks a seat gets there did not reach it. This pin drives the
// judge's own call site with the box's real renderer and asserts each mask
// on what it rendered: no network, the runtime dir hidden, a private /tmp
// inside the run dir, a build cache of the run's own, and no write grant on
// the box's shared build cache. Delete it and a regression puts landed code
// back inside the launcher's next build, or back on the session bus, with
// no test red.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestQALandedCheckJudgeIsNoStrongerThanASeat(t *testing.T) {
	// No t.Parallel: the fixture sets HOME, and this sets XDG_RUNTIME_DIR.
	f := newLandedCageFixture(t)
	// A runtime dir of the fixture's own, so the hide row is never vacuous
	// on a host that has none.
	rtd := filepath.Join(f.root, "runtime")
	if err := os.MkdirAll(rtd, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", rtd)

	repo := filepath.Join(f.root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, landedCheckScript), "#!/bin/sh\nexit 0\n")
	mustGit(t, repo, "add", "--", landedCheckScript)
	mustGit(t, repo, "commit", "-q", "-m", "ship", "--", landedCheckScript)
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	run := filepath.Join(f.root, "run-pin")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = git(repo, "worktree", "remove", "--force", filepath.Join(run, landedTreeName)) })

	// The renderer is the box's; the seam only keeps what it was handed and
	// what it rendered, and then refuses, so the judge never runs.
	var (
		rendered    landedCage
		renderErr   error
		gotWritable []string
		gotEnv      []string
	)
	c := &landedChecker{a: &App{}, out: &strings.Builder{}, pending: map[string]string{}}
	c.cage = func(a *App, judge, tree string, writable, env []string) (landedCage, error) {
		gotWritable, gotEnv = writable, env
		rendered, renderErr = boxLandedCage(a, judge, tree, writable, env)
		return landedCage{}, Die("pin: rendered and not run")
	}
	c.judge(repo, run, tip, revTreeID(repo, tip))
	if gotWritable == nil {
		t.Fatal("landedChecker.judge never reached the cage renderer, so nothing below was asked")
	}
	if renderErr != nil {
		t.Skip("NOTHING MEASURED: no landed-check cage renders on this host: " + renderErr.Error())
	}

	// D2: the run's own cache, and nothing granting the box's.
	gocache := filepath.Join(run, "gocache")
	if !containsString(gotEnv, "GOCACHE="+gocache) {
		t.Errorf("the judge's env is %v, want GOCACHE=%s — a cache of the run's own (ranger-base-4nzpo)", gotEnv, gocache)
	}
	shared := goBuildCache()
	if shared == "" {
		t.Fatal("goBuildCache resolved nothing, so the shared-cache row cannot be asked")
	}
	for _, w := range gotWritable {
		if writeGranted([]string{absResolve(w)}, absResolve(shared)) {
			t.Errorf("the judge's write set grants %s, which covers the box's shared build cache %s — landed code writing into the launcher's next build (ranger-base-4nzpo)", w, shared)
		}
	}

	switch WallMechanism() {
	case WallBwrap:
		argv, line := rendered.Argv, bwJoin(rendered.Argv)
		if bwIndex(argv, "--unshare-net") < 0 {
			t.Errorf("the judge's cage shares the host network namespace — want --unshare-net (ranger-base-4nzpo):\n  %s", line)
		}
		hide, err := liveBwrapRuntimeDirs()
		if err != nil {
			t.Fatal(err)
		}
		if !containsString(hide, absResolve(rtd)) {
			t.Fatalf("the fixture's runtime dir %s is not among the live ones %v, so the hide row would not test it", rtd, hide)
		}
		for _, h := range hide {
			if bwIndex(argv, "--tmpfs", h) < 0 {
				t.Errorf("the judge's cage does not hide the runtime dir %s, so it reaches the session bus and the user service manager (ranger-base-4nzpo):\n  %s", h, line)
			}
		}
		if bwIndex(argv, "--bind", filepath.Join(run, "tmp"), bwrapTmpMount) < 0 {
			t.Errorf("the judge's /tmp is not bound to %s, so it writes where the box's uncaged builds stage (ranger-base-4nzpo):\n  %s", filepath.Join(run, "tmp"), line)
		}
		if bwIndex(argv, "--setenv", "GOCACHE", gocache) < 0 {
			t.Errorf("the judge's cage does not set GOCACHE=%s (ranger-base-4nzpo):\n  %s", gocache, line)
		}
		for i := 0; i+2 < len(argv); i++ {
			if argv[i] == "--bind" && writeGranted([]string{argv[i+2]}, absResolve(shared)) {
				t.Errorf("the judge's cage binds %s writable, which covers the box's shared build cache %s (ranger-base-4nzpo)", argv[i+2], shared)
			}
		}
	case WallSandboxExec:
		b, err := os.ReadFile(filepath.Join(run, "landed-check.sb"))
		if err != nil {
			t.Fatal(err)
		}
		prof := string(b)
		// ASSUMED, NOT MEASURED: no darwin host runs this (ranger-base-bfla6).
		// What is pinned is that the rule is rendered, and last.
		if !strings.HasSuffix(prof, landedCheckSeatbeltNet(run)) {
			t.Errorf("the landed-check profile does not end with the network rule (ranger-base-4nzpo):\n%s", prof)
		}
		if strings.Contains(prof, "(subpath "+sbQuote(absResolve(shared))+")") {
			t.Errorf("the landed-check profile grants the box's shared build cache %s (ranger-base-4nzpo)", shared)
		}
	default:
		t.Fatalf("boxLandedCage rendered a cage on mechanism %q, which this pin does not know how to read", WallMechanism())
	}
}

// The darwin rule, read on any host: deny everything, then loopback IP and
// the run's own unix sockets back, in that order (last match wins in SBPL).
func TestQALandedCheckSeatbeltNetRule(t *testing.T) {
	t.Parallel()
	r := landedCheckSeatbeltNet("/var/landed/run-1")
	deny := strings.Index(r, "(deny network*)")
	if deny < 0 {
		t.Fatalf("the rule denies no network:\n%s", r)
	}
	for _, allow := range []string{
		`(allow network* (local ip "localhost:*"))`,
		`(allow network* (remote ip "localhost:*"))`,
		`(allow network* (local unix-socket (subpath "/var/landed/run-1")))`,
		`(allow network* (remote unix-socket (subpath "/var/landed/run-1")))`,
	} {
		i := strings.Index(r, allow)
		if i < 0 {
			t.Errorf("the rule lacks %s:\n%s", allow, r)
		} else if i < deny {
			t.Errorf("%s comes before the deny, so the deny outvotes it:\n%s", allow, r)
		}
	}
	if strings.Contains(r, "(allow network* (remote ip \"*") || strings.Contains(r, "(allow network*)") {
		t.Errorf("the rule allows non-loopback network:\n%s", r)
	}
}

// landedCheckTmp stands down where a private /tmp would hide the judge's
// own tree, and is the run's own dir everywhere else.
func TestQALandedCheckTmp(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ home, run, tree, want string }{
		{"/home/u", "/home/u/.posse/landed-check/run-1", "/home/u/.posse/landed-check/run-1/tree", "/home/u/.posse/landed-check/run-1/tmp"},
		{"/tmp/h", "/home/u/run", "/home/u/run/tree", ""},
		{"/home/u", "/tmp/run", "/tmp/run/tree", ""},
		{"/home/u", "/home/u/run", "/tmp/tree", ""},
		// No home is no home under /tmp: the run's own tmp, not a stand-down
		// (ranger-base-f1rpw).
		{"", "/home/u/run", "/home/u/run/tree", "/home/u/run/tmp"},
		{"/home/u", "", "/home/u/run/tree", ""},
		{"/home/u", "/home/u/run", "", ""},
	} {
		if got := landedCheckTmp(c.home, c.run, c.tree); got != c.want {
			t.Errorf("landedCheckTmp(%q, %q, %q) = %q, want %q", c.home, c.run, c.tree, got, c.want)
		}
	}
}

// ranger-base-5m5yq — the judge's environment is an allowlist, not the
// launcher's. Driven through landedChecker.judge itself, with a judge that
// writes its own environment where the test can read it: the pin reads what
// the child GOT, not what a helper returns, so a call site that went back to
// os.Environ() reds here even if landedJudgeEnv were left intact.
//
// The cage is the one thing stubbed (the judge runs as the argv, the way
// plainCage runs it in arm 2), because cmd.Env is built after the cage
// renders and is the same list on both mechanisms.
//
// TWO ARMS ARE WRITTEN AGAINST A STATE THIS BINARY DOES NOT HAVE BY DEFAULT,
// and both were holes until ranger-base-o80tw (ranger-base-epvoy's verify of
// ranger-base-5m5yq found them):
//
//   - The PATH row — ADR 0077 D3's half of the two the call site calls "two
//     halves of ONE rule" — could not discriminate. This package's TestMain
//     does os.Setenv("PATH", PathOutsideGates("")) process-wide
//     (herdr_test.go), so os.Getenv("PATH") here ALREADY equalled the value
//     the row wants and PathOutsideGates is idempotent over it: dropping
//     protocolEnv from landedJudgeEnv left this test green (MEASURED
//     2026-11-12). A gates-shaped element is prepended below so the wanted
//     and the got values differ when the strip is absent. The PROPERTY was
//     pinned elsewhere even then — the same mutant reds internal/treepins'
//     TestQAEveryProtocolChildRunsOutsideTheGates, naming the line — but
//     that census says of itself that it is syntactic, so this row was a pin
//     claiming a check it did not make.
//
//   - An allowlisted name that is UNSET must be ABSENT, not spelled empty.
//     landedJudgeEnv's os.LookupEnv guard is the whole of that, and nothing
//     held it: the membership loop ALLOWS every allowlisted name and the
//     value rows read only TMPDIR, GOCACHE, GOFLAGS and PATH, so replacing
//     the guard with os.Getenv left this test green too (MEASURED
//     2026-11-12). `LC_ALL=` and `GOFLAGS=` are not the same instruction as
//     absent, and what reads them is a repo's `make`.
func TestQALandedCheckJudgeEnvIsTheAllowlist(t *testing.T) {
	// No t.Parallel: Setenv.
	root := gitTempDir(t)
	const planted = "FOO_TOKEN"
	t.Setenv(planted, "planted-by-the-pin")
	t.Setenv("GOFLAGS", "-mod=mod") // an allowlisted name that is set reaches it

	// An allowlisted name that is NOT set. t.Setenv first and then Unsetenv,
	// in that order: the Setenv is what registers the restore, so this
	// absence cannot leak into a later test in this binary.
	const unset = "LC_ALL"
	t.Setenv(unset, "")
	os.Unsetenv(unset)

	// AND THE PREMISE OF THAT ARM, WHICH NOTHING HELD (ranger-base-bdjot, the
	// verify of ranger-base-o80tw's close). The absence assertion below reads
	// "an ALLOWLISTED name that is unset is ABSENT", and the only thing making
	// `unset` allowlisted is landedJudgeEnvKeys — which this test reads to
	// BUILD its own `allowed` map, so the fixture comes from the list under
	// test and every mutant of that list is equivalent to it. MEASURED: drop
	// "LC_ALL" from landedJudgeEnvKeys and this test is green, because the
	// name is then absent for the wrong reason; drop it AND replace
	// landedJudgeEnv's os.LookupEnv guard with os.Getenv — the mutant the arm
	// below exists to kill — and the pair is green too. landedJudgeEnvKeys has
	// no reader anywhere else in this module (census 2026-11-13: landedcheck.go
	// and this file), so no other arm could have said otherwise. One line, so
	// that the arm reds when its premise leaves rather than going quiet.
	if !slices.Contains(landedJudgeEnvKeys, unset) {
		t.Fatalf("%s is not in landedJudgeEnvKeys, so the unset-name arm below has no premise: it asserts that an ALLOWLISTED name which is not set is ABSENT from the judge's environment, and a name that is not allowlisted is absent for a different reason. Name another allowlisted value here, do not delete the arm — it is the only one over landedJudgeEnv's os.LookupEnv guard (ranger-base-bdjot)", unset)
	}

	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, landedCheckScript), "#!/bin/sh\nenv > \"$TMPDIR/judge.env\"\n")
	mustGit(t, repo, "add", "--", landedCheckScript)
	mustGit(t, repo, "commit", "-q", "-m", "ship", "--", landedCheckScript)
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	run := filepath.Join(root, "run-env")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = git(repo, "worktree", "remove", "--force", filepath.Join(run, landedTreeName)) })

	// A gates-shaped element at the head of this process's PATH, so the PATH
	// row below can discriminate. Last of the Setenvs and after the git
	// fixture, so nothing above resolves a binary against it; the dir need
	// not exist, because isGatesPath judges a `gates` path ELEMENT and runs
	// no filesystem call. Without this, see the comment above: the row's want
	// and the judge's got are the same string whether or not protocolEnv ran.
	// bdgatesfreepath_test.go is the house spelling of the same plant.
	t.Setenv("PATH", filepath.Join(root, "state", "gates", "launcher", "bin")+
		string(os.PathListSeparator)+os.Getenv("PATH"))

	c := &landedChecker{a: &App{}, out: &strings.Builder{}, pending: map[string]string{}}
	c.cage = func(_ *App, judge, _ string, _ []string, env []string) (landedCage, error) {
		return landedCage{Argv: []string{judge}, Env: env}, nil
	}
	if v, said := c.judge(repo, run, tip, revTreeID(repo, tip)); v != landGreen {
		t.Fatalf("the env-dumping judge did not run green: %v — %s", v, said)
	}
	b, err := os.ReadFile(filepath.Join(run, "tmp", "judge.env"))
	if err != nil {
		t.Fatalf("the judge wrote no environment: %v", err)
	}

	allowed := map[string]bool{"TMPDIR": true, "GOCACHE": true}
	for _, k := range landedJudgeEnvKeys {
		allowed[k] = true
	}
	// What /bin/sh sets for ITSELF before `env` runs, and so not something
	// the launcher handed over: the cwd and the nesting level, and `_`.
	shellOwn := map[string]bool{"PWD": true, "OLDPWD": true, "SHLVL": true, "_": true}
	got := map[string]string{}
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, v, ok := strings.Cut(ln, "=")
		if !ok {
			continue // a continuation of a multi-line value
		}
		got[k] = v
		if !allowed[k] && !shellOwn[k] {
			t.Errorf("the judge's environment carries %s, which is not in landedJudgeEnvKeys — the launcher's environment crossing the wall (ranger-base-5m5yq)", k)
		}
	}
	if _, ok := got[planted]; ok {
		t.Errorf("the judge saw %s, planted in the launcher's environment (ranger-base-5m5yq)", planted)
	}
	if v, ok := got[unset]; ok {
		t.Errorf("the judge's environment carries %s=%q, and %s is UNSET in the launcher — an allowlisted name that is not set must be ABSENT and not present-and-empty, because `%s=` is a different instruction from no %s at all. landedJudgeEnv's os.LookupEnv guard is the whole of that property and this is the only arm over it (ranger-base-o80tw, from ranger-base-epvoy's verify of ranger-base-5m5yq)", unset, v, unset, unset, unset)
	}
	for k, want := range map[string]string{
		"TMPDIR":  filepath.Join(run, "tmp"),
		"GOCACHE": landedCheckGoCache(run),
		"GOFLAGS": "-mod=mod",
		"PATH":    PathOutsideGates(""),
	} {
		if got[k] != want {
			t.Errorf("the judge's %s is %q, want %q", k, got[k], want)
		}
	}
}
