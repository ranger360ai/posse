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
	"fmt"
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
	c.cage = func(a *App, kind landedCageKind, judge, tree string, writable, env []string) (landedCage, error) {
		gotWritable, gotEnv = writable, env
		rendered, renderErr = boxLandedCage(a, kind, judge, tree, writable, env)
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
		// ranger-base-i92tj — THE SOCKET WALL, IN THE SAME POSITION THE
		// SEAT'S LAUNCH LINE PUTS IT. Since ranger-base-6rndn a bwrap seat
		// runs behind `posse wall-unix --` typed after bwrap's own `--` (ADR
		// 0080 D27, renderWallInto). D3.1's rule is that the judge is never
		// stronger than a seat, so the judge's own argv has to carry it in
		// that one position: a step in FRONT of bwrap would apply a
		// filesystem Landlock domain to the process that then calls mount(2),
		// which is refused (probe U9/U10), and a step after the judge is an
		// argument to the judge.
		//
		// THE POSITION IS ASSERTED AS A WHOLE TAIL rather than as a
		// membership, because that is the only form that says the step wraps
		// the JUDGE and nothing else: what follows the step's own `--` must be
		// the child and must be the end of the line.
		//
		// BOTH ARMS ARE WRITTEN, driven by this host's real availability. On
		// this fleet the first is taken (ABI 10); the second is the one that
		// must never be silence, and it is a host decision and not a defect
		// (ADR 0080 D31).
		judgePath := filepath.Join(run, landedJudgeRule.Extract)
		if argv[len(argv)-1] != judgePath {
			t.Fatalf("the judge's cage does not end with the child %s, so the tail rows below are reading something else:\n  %s", judgePath, line)
		}
		if wallUnixAvailableFor(WallBwrap) {
			self, err := wallWatchBin()
			if err != nil {
				t.Fatalf("wallWatchBin, which is how the step's binary is resolved, refused: %v", err)
			}
			// The bind-back comes FIRST and immediately in front of BWRAP's
			// own `--`, because it has to outvote every mask above it (this
			// render's private /tmp covers a launcher under /tmp, and
			// landedJudgeHides' tmpfs covers one under /run) and because the
			// file it puts back is the one the step then names. So the tail
			// is: the mount, bwrap's `--`, the step's three words, the child,
			// end of line.
			tail := append([]string{"--ro-bind", self, self, "--"}, WallUnixStepArgv(self)...)
			tail = append(tail, judgePath)
			if i := bwIndex(argv, tail...); i < 0 || i+len(tail) != len(argv) {
				t.Errorf("this host renders the socket wall and the judge's cage does not END with `%s` — a seat's launch line carries that step and ADR 0088 D3.1 says the judge is never stronger than a seat, so landed code here could connect(2) to any host pathname socket in the judge's view, including one an uncaged process binds after the run started (ranger-base-l2gk9). `--unshare-net` does not reach a pathname socket (ranger-base-i92tj):\n  %s",
					strings.Join(tail, " "), line)
			}
			// AND THE BINARY IS THE LAUNCHER'S, not one out of the tree
			// under judgement. The judge runs landed code that nothing
			// reviewed; a step resolved from that tree would let the code
			// being judged choose its own wall, which is worse than having
			// no step at all.
			if tree := filepath.Join(run, landedTreeName); underDir(tree, self) {
				t.Errorf("the socket-wall step names %s, which is inside the tree being judged (%s) — the code under judgement would be choosing its own wall (ranger-base-i92tj)", self, tree)
			}
		} else {
			if i := bwIndex(argv, WallUnixVerb); i >= 0 {
				t.Errorf("the judge's cage carries the socket-wall step at argv[%d] on a host that cannot realize it — the step refuses rather than execs when the domain will not apply (ADR 0080 D30), so this is a judge that never runs:\n  %s", i, line)
			}
			if landedJudgeSocketWallNote() == "" {
				t.Errorf("no socket wall renders on this host and the judge's verdict line would say NOTHING about it — ADR 0080 D31 is DEGRADED and never silence, and the judge has no persona and so no `posse gates` line of its own (ranger-base-i92tj)")
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

// ranger-base-i92tj — ADR 0080 D31 AT THE JUDGE'S VERDICT LINE: on a host
// whose Landlock ABI is below 9 the judge's cage carries no socket-wall step,
// and every verdict SAYS SO. Never silence.
//
// WHY THIS IS A SEPARATE PIN, AND WHY IT DRIVES `once` AND NOT THE HELPER.
// The arm above reads the real render on the real host, so on this fleet
// (ABI 10) it only ever takes the branch where the step IS there, and its
// degraded row is unreachable here. MEASURED 2026-11-14 by mutation:
// replacing landedJudgeSocketWallNote's body with `return ""` — the sentence
// DECIDED and never EMITTED, which is ranger-base-ig79v's finding 2(b) one
// mechanism over — left every pin in this file and both venue argv pins
// green. `wallUnixABI` is a var for exactly this (wallunixrender_qa_test.go
// drives the render from darwin the same way), so the host answer is a
// parameter here.
//
// AND IT READS THE RUNNER'S OUTPUT, not the helper's return. The helper
// returning the right sentence is worth nothing if no verdict line carries
// it, and all THREE arms are read — green, red, and the one that says there
// is no verdict — because "never say nothing" is a claim about the line a
// reader actually gets, whichever one that is, and the unanswered arm is the
// one whose own sentence is longest and most likely to be thought complete.
//
// AND THE MECHANISM IS A PARAMETER TOO, FOR THE SAME REASON THE ABI IS
// (ranger-base-whpxv). The ABI rows were driven and the mechanism was
// inherited from the box, so every row asserted bwrap's answer on whatever
// host ran it — and the DEGRADED sentence is bwrap's alone: WallUnixDegraded
// returns "" for any mech != WallBwrap (wallunix.go), which landedcheck.go
// says on purpose at landedJudgeSocketWallNote ("EMPTY ON SEATBELT … there is
// nothing degraded about a mechanism this layer was never part of"). So on
// darwin all six degraded rows red while the product was doing the
// darwin-correct thing: MEASURED 2026-11-21 on macos-latest arm 1, run
// 44974829306, job 137062263947, at b2509fba — "this host renders no socket
// wall (Landlock ABI -38 …) and the <v> verdict says NOTHING about it", six
// times, {green,red,no answer} x {ABI 8, ABI -38}, with ubuntu arms 1-3 green
// on the same run. `wallMechanismHost` is a var for exactly this (wall.go,
// ranger-base-temhf), so the rows that read bwrap's sentence now SAY bwrap.
//
// THE DARWIN ROW IS A READING AND NOT A SKIP. It takes the ABI that reds on
// linux and asserts the verdict says NO DEGRADED there — because the darwin
// judge's socket wall is landedCheckSeatbeltNet, which is on the profile at
// every ABI, so silence is the honest line and a seatbelt host that STARTED
// printing bwrap's alarm would be an operator told to go and look at a
// Landlock version darwin has no syscall for (ranger-base-q9bqu holds the
// darwin half of D31 itself).
//
// Each row controls that landedJudgeSocketWallNote agrees with what the row
// is named for before the runner is driven, so neither the ABI nor the
// mechanism can stop deciding and leave the output rows looking like they
// still measure the two arms.
//
// SERIAL, no t.Parallel: wallUnixABI and wallMechanismHost are package-level
// vars.
func TestQALandedCheckVerdictSaysWhenThereIsNoSocketWall(t *testing.T) {
	restore := wallUnixABI
	t.Cleanup(func() { wallUnixABI = restore })

	for _, v := range []struct {
		verdict string
		body    string // the judge's own exit, or "" for a cage that refuses
	}{
		{"green", "exit 0\n"},
		{"red", "echo 'a door said no' >&2\nexit 1\n"},
		{"no answer", ""},
	} {
		for _, abi := range []struct {
			goos string
			n    int
			want bool // the verdict must carry the DEGRADED sentence
		}{
			{"linux", 10, false}, // the D1 host: the layer is up and the line stays quiet
			{"linux", WallUnixABIMin, false},
			{"linux", WallUnixABIMin - 1, true}, // handled_access_fs would be refused outright
			{"linux", -38, true},                // no landlock_create_ruleset at all
			// THE SEATBELT HOST, at the ABI that reds on linux: this layer
			// was never part of that mechanism, so the line is quiet and
			// that is the reading (ranger-base-whpxv).
			{"darwin", WallUnixABIMin - 1, false},
		} {
			t.Run(fmt.Sprintf("%s on %s at ABI %d", v.verdict, abi.goos, abi.n), func(t *testing.T) {
				wallMechanismForTest(t, abi.goos)
				wallUnixABI = func() int { return abi.n }
				// THE ROW'S OWN CONTROL: the note the three verdict lines
				// below carry has to be the one this row is named for
				// BEFORE the runner runs, or the reads below are measuring
				// a helper that stopped deciding rather than a line that
				// stopped printing.
				if note := landedJudgeSocketWallNote(); (note != "") != abi.want {
					t.Fatalf("on %s at Landlock ABI %d the judge's socket-wall note is %q, want degraded = %v — this row is not the reading it is named for (ranger-base-whpxv)", abi.goos, abi.n, note, abi.want)
				}
				a := wtApp(t)
				repo := wtRepo(t)
				// ONLY the judge's script, so ADR 0097 D2's wall check does
				// not opt in and the output below is one verdict line.
				write(t, filepath.Join(repo, landedCheckScript), "#!/bin/sh\n"+orElse(v.body, "exit 0\n"))
				mustGit(t, repo, "add", "--", landedCheckScript)
				mustGit(t, repo, "commit", "-q", "-m", "ship the judge", "--", landedCheckScript)

				out := &strings.Builder{}
				c := newLandedChecker(a, out)
				c.cage = func(_ *App, _ landedCageKind, child, _ string, _ []string, env []string) (landedCage, error) {
					if v.body == "" {
						// D3's one hard rule: a cage that will not render
						// ends the run, and that line is a verdict too.
						return landedCage{}, Die("pin: no cage renders here")
					}
					return landedCage{Argv: []string{child}, Env: env}, nil
				}
				c.once(repo, "main")

				said := out.String()
				if said == "" {
					t.Fatalf("the runner said nothing about %s, so there is no verdict line to read", AbbrevHome(repo))
				}
				got := strings.Contains(said, "DEGRADED")
				switch {
				case abi.want && !got:
					t.Errorf("a %s host renders no socket wall (Landlock ABI %d, RESOLVE_UNIX needs %d) and the %s verdict says NOTHING about it. The judge has no persona and so no `posse gates` line of its own, so this is the ONLY place its wall is ever reported — and a layer that is absent and silent reads as one that is applied (ADR 0080 D31, ranger-base-l2gk9):\n  %s",
						abi.goos, abi.n, WallUnixABIMin, v.verdict, strings.TrimSpace(said))
				case !abi.want && got:
					t.Errorf("a %s host's socket wall is not degraded at Landlock ABI %d (the layer is up, or the mechanism is one it was never part of) and the %s verdict calls it DEGRADED anyway — every landing would carry a sentence that is false, in the line D6.2's filer reads:\n  %s",
						abi.goos, abi.n, v.verdict, strings.TrimSpace(said))
				}
				if abi.want && !strings.Contains(said, WallUnixDegraded(WallMechanism())) {
					t.Errorf("the %s verdict carries the word DEGRADED but not WallUnixDegraded's own sentence, so an operator gets the alarm without the fact (ranger-base-i92tj):\n  %s", v.verdict, strings.TrimSpace(said))
				}
			})
		}
	}
}

// The darwin rule, read on any host: deny everything, then loopback IP and
// the run's own unix sockets back, in that order (last match wins in SBPL).
//
// AND THE SOCKET GRANT IS THE RESOLVED SPELLING, WHICH IS THE WHOLE POINT OF
// THE ROWS (ranger-base-whpxv). The pin held the subpath against the literal
// it had passed IN, so it asserted a host where no ancestor of the run dir is
// a symlink — and darwin is not that host: `/var` is a link to `/private/var`
// there, so `landedCheckSeatbeltNet("/var/landed/run-1")` renders
// `(subpath "/private/var/landed/run-1")` and the pin red. MEASURED
// 2026-11-21 on macos-latest arm 1, run 44974829306, job 137062263947, at
// b2509fba: both halves, local and remote, read the `/private/var` spelling.
// The PRODUCT is right there and the expectation was wrong — a seatbelt rule
// is matched by the kernel against real paths, so a grant written in the
// producer's raw spelling is a grant that matches nothing, which is
// ranger-base-al4n9's sentence one render over.
//
// SO THE RESOLVE IS DRIVEN HERE RATHER THAN INHERITED: the third row's run
// dir sits under a `var` that is a link to a `private/var`, darwin's /var in
// miniature, so absResolve moves the path on this box too and the arm that
// says the RAW spelling must not appear has something to read on every host.
// The first row keeps the production-shaped `/var/landed/run-1` input, whose
// resolve is the platform's own — unmoved here, moved on a macos leg — and
// its expectation is built from absResolve rather than typed, so both
// readings are correct rather than one of them being asserted everywhere.
func TestQALandedCheckSeatbeltNetRule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "private", "var"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "private", "var"), filepath.Join(root, "var")); err != nil {
		t.Fatal(err)
	}
	moved := 0
	for _, row := range []struct{ name, run string }{
		{"the production spelling", "/var/landed/run-1"},
		{"a run dir with no link above it", filepath.Join(root, "private", "var", "landed", "run-1")},
		{"a run dir under a symlinked ancestor", filepath.Join(root, "var", "landed", "run-1")},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := landedCheckSeatbeltNet(row.run)
			want, raw := sbQuote(absResolve(row.run)), sbQuote(row.run)
			if want != raw {
				moved++
			}
			deny := strings.Index(r, "(deny network*)")
			if deny < 0 {
				t.Fatalf("the rule denies no network:\n%s", r)
			}
			for _, allow := range []string{
				`(allow network* (local ip "localhost:*"))`,
				`(allow network* (remote ip "localhost:*"))`,
				"(allow network* (local unix-socket (subpath " + want + ")))",
				"(allow network* (remote unix-socket (subpath " + want + ")))",
			} {
				i := strings.Index(r, allow)
				if i < 0 {
					t.Errorf("the rule lacks %s:\n%s", allow, r)
				} else if i < deny {
					t.Errorf("%s comes before the deny, so the deny outvotes it:\n%s", allow, r)
				}
			}
			// The resolved-grant arm, and it only has a question to ask on a
			// row whose resolve moved: a subpath written as the caller
			// spelled it is one the kernel matches nothing against, so the
			// judge's own sockets would be denied along with the host's.
			if want != raw && strings.Contains(r, "(subpath "+raw+")") {
				t.Errorf("the rule grants the producer's raw spelling %s rather than the resolved %s — the kernel matches real paths, so that grant covers nothing (ranger-base-whpxv, ranger-base-al4n9):\n%s", raw, want, r)
			}
			if strings.Contains(r, "(allow network* (remote ip \"*") || strings.Contains(r, "(allow network*)") {
				t.Errorf("the rule allows non-loopback network:\n%s", r)
			}
		})
	}
	// THE TABLE'S OWN CONTROL: the third row's `var` is a symlink on every
	// host, so one row's resolve always moves. If none did, absResolve was
	// exercised by nothing and the arm above measured nothing on this box.
	if moved == 0 {
		t.Errorf("no row's path was moved by absResolve, so nothing here exercised the resolve and the raw-spelling arm is vacuous (ranger-base-whpxv)")
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
	c.cage = func(_ *App, _ landedCageKind, judge, _ string, _ []string, env []string) (landedCage, error) {
		return landedCage{Argv: []string{judge}, Env: env}, nil
	}
	if v, said, _ := c.judge(repo, run, tip, revTreeID(repo, tip)); v != landGreen {
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
