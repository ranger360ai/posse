package posse

// QA's adversarial half of rangerhq-6so's live pin (rangerhq-pafo).
//
// cageinnerlive_test.go proves the tier does what it says on the happy
// path. This file tries to make it LIE. Every claim below is one the
// bead's six items make and neither the hermetic tests nor the live pin
// assert, or one where the assertion is a proxy for the real thing:
//
//   - item 4 says "{memory} and the seeded runtime state dir stay rw"
//     while the repo is :ro. Nothing measured that; a boundary that took
//     the persona's notebook away with the work would be a different tier
//     than the one that shipped.
//   - item 4's ":ro" is asserted by one `touch` at the repo root. A mount
//     is read-only or it is not: nested paths, .git, an APPEND to a file
//     that already exists, an unlink, a chmod.
//   - items 1/2 forward RHQ_GATES_DIR by NAME, so the launching pane's own
//     HOST value crosses. If it won inside, the inner shims and the hook
//     would log to a path that is not mounted — an un-refused verb wearing
//     a refusal's clothes. The override is the load-bearing part.
//   - item 3's parity is pinned against a FAKE engine whose `inner:` is
//     literally `false`. The real question is whether a real image with no
//     Linux posse in it is caught.
//   - item 5's unknown-name refusal is pinned at CheckSockets. The claim is
//     that the LAUNCH refuses.
//
// Same guard as the pin it shadows:
//
//	RHQ_LIVE_DOCKER=1 go test ./internal/posse -run TestQALiveCage -v
//
// Measured 2026-08-27, macOS 26.4.1 / Docker 29.0.1, image posse-cage:latest
// built from this tree.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaLiveCageApp is developer's live-pin setup, factored: an App on a scratch
// RHQ_HOME whose engine is the built-in docker line with `-i -t` dropped,
// because `go test` is not a terminal. Derived from the built-in rather
// than retyped, so a change to the engine cannot leave QA verifying a line
// nobody launches.
func qaLiveCageApp(t *testing.T) (*App, *Engine) {
	t.Helper()
	home := t.TempDir()
	a := &App{
		Home: home, ConfigPath: filepath.Join(home, "config.yaml"),
		EnvsDir: filepath.Join(home, "envs"), StateDir: filepath.Join(home, "state"),
		AgentsDir: filepath.Join(home, "agents"),
	}
	d := builtinEngines[0]
	os.MkdirAll(a.CagesDir(), 0o755)
	if err := os.WriteFile(filepath.Join(a.CagesDir(), "notty.yaml"), []byte(strings.Join([]string{
		"command: " + strings.Replace(d.Command, " -i -t", "", 1),
		"mount: " + d.Mount, "mount_ro: " + d.MountRO, "env: " + d.Env, "env_set: " + d.EnvSet,
		"home: " + d.Home, "probe: " + d.Probe, "inner: " + d.Inner,
		"net: " + d.Net, "net_create: " + d.NetCreate, "net_join: " + d.NetJoin,
		"net_remove: " + d.NetRemove, "proxy_up: " + d.ProxyUp, "proxy_down: " + d.ProxyDown, "",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_engine: notty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		t.Fatal(err)
	}
	return a, e
}

// qaRunCage writes probe into dir/probe.sh, builds the launch plan for
// front/dir and RUNS it, returning the `k=v` lines the probe printed.
// deny is what the session carries in RHQ_TOOLS_DENY — the list the inner
// render and the pre-push hook both read. Two placeholders are substituted
// so the probe can name host paths by the same spelling they have inside:
// %MEM% is the persona's memory mount, %CAGEHOME% the runtime's HOME.
func qaRunCage(t *testing.T, a *App, e *Engine, front, dir, session string, deny []string, probe string) map[string]string {
	t.Helper()
	rt, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, front)
	if err := ag.EnsureMemoryDir(); err != nil {
		t.Fatal(err)
	}
	probe = strings.ReplaceAll(probe, "%MEM%", ag.MemoryDir)
	probe = strings.ReplaceAll(probe, "%CAGEHOME%", e.Home)
	if err := os.WriteFile(filepath.Join(dir, "probe.sh"), []byte(probe), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.WrapInCage(ag, rt, session, dir, "sh ./probe.sh",
		append(CageEnvNames(nil), "CLAUDE_CODE_OAUTH_TOKEN"), ""); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.CageArgvFile(ag.Name, session))
	if err != nil {
		t.Fatal(err)
	}
	var plan CageLaunchPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Egress != nil {
		t.Cleanup(func() { runCageSteps(plan.Egress.Down, os.Stderr) })
		runCageSteps(plan.Egress.Down, os.Stderr)
		if err := runCageSteps(plan.Egress.Up, os.Stderr); err != nil {
			t.Fatal(err)
		}
	}
	c := exec.Command(plan.Path)
	c.Args = plan.Argv
	c.Env = append(os.Environ(),
		"RHQ_TOOLS_DENY="+strings.Join(deny, "\n"),
		"RHQ_PERSONA="+ag.Name,
	)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("the cage did not run: %v\n%s", err, out)
	}
	got := map[string]string{}
	for _, ln := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(ln), "="); ok {
			got[k] = v
		}
	}
	t.Logf("cage said:\n%s", out)
	return got
}

// qaLiveGuard is the pin's own guard, in one place: this file measures the
// real engine and the real image or it measures nothing.
func qaLiveGuard(t *testing.T, a *App, e *Engine) string {
	t.Helper()
	if os.Getenv("RHQ_LIVE_DOCKER") == "" {
		t.Skip("set RHQ_LIVE_DOCKER=1 (needs docker and `posse cage build`)")
	}
	if !a.ContainerAvailable() {
		t.Skipf("engine %s is not on this host", e.Name)
	}
	image := a.CageImage()
	if !a.CageImageBuilt(e, image) {
		t.Skipf("%s is not built — run `posse cage build`", image)
	}
	return image
}

// The mount boundary, attacked rather than sampled. ADR 0002 §4 and item 4
// promise two things at once and the live pin measures half of one: the
// repo is read-only, and `{memory}` and the runtime's HOME are NOT. A
// boundary that only stopped `touch` at the repo root would be a boundary
// in name; one that also took the persona's notebook would be a tier no
// persona could work in.
func TestQALiveCageMountBoundaryIsDeepAndTheNotebookSurvives(t *testing.T) {
	a, e := qaLiveCageApp(t)
	qaLiveGuard(t, a, e)
	dir := liveCageRepo(t, "")
	// A nested path that already exists, committed, so the read-only answer
	// cannot be confused with "the parent was not there".
	if err := os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep", "f"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	probe := `
try() { if eval "$2" 2>/dev/null; then echo "$1=wrote"; else echo "$1=refused"; fi; }
echo "gates=$RHQ_GATES_DIR"
try root      'touch ./written'
try nested    'touch ./sub/deep/written'
try append    'echo more >> ./sub/deep/f'
try unlink    'rm -f ./sub/deep/f && test ! -e ./sub/deep/f'
try chmod     'chmod 700 ./sub/deep/f'
try mkdir     'mkdir ./freshdir'
try gitdir    'touch ./.git/written'
try gitcommit 'git commit -q --allow-empty -m qa'
try memory    'touch %MEM%/qa-wrote'
try cagehome  'touch %CAGEHOME%/qa-wrote'
try refusals  'test -w "$RHQ_GATES_DIR/refusals.log"'
`
	deny := []string{"Bash(git push:*)", "Edit", "Write"}
	got := qaRunCage(t, a, e, "cage: container\ndeny: ["+strings.Join(deny, ", ")+"]\n", dir, "qa-ro", deny, probe)

	// Read-only means read-only, at every depth and for every kind of write.
	for _, k := range []string{"root", "nested", "append", "unlink", "chmod", "mkdir", "gitdir", "gitcommit"} {
		if got[k] != "refused" {
			t.Errorf("the repo mount is :ro for a PID denying Edit/Write — %q said %q; a boundary with a hole in it is not a boundary", k, got[k])
		}
	}
	// ...and the persona keeps its own notes and the runtime its own state,
	// which is the half of item 4 that makes the other half livable.
	for _, k := range []string{"memory", "cagehome"} {
		if got[k] != "wrote" {
			t.Errorf("{memory} and the runtime HOME stay READ-WRITE beside a :ro repo — %q said %q", k, got[k])
		}
	}
	// The host-side proof, not the container's own report: the file is on
	// the host afterwards, at the path the mount promised.
	for _, p := range []string{filepath.Join(a.PersonasDir(), "p", "qa-wrote"), filepath.Join(a.CageHome("p"), "qa-wrote")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("the cage's write must land on the HOST at %s: %v", AbbrevHome(p), err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "written")); err == nil {
		t.Error("a write the container reported as refused reached the host repo anyway")
	}

	// Items 1/2 forward RHQ_GATES_DIR by NAME, so this pane's own HOST value
	// crosses the boundary. If it won inside, every inner shim and the
	// pre-push hook would resolve their rules and their log against a path
	// that is not mounted — a gate that refuses nothing while reporting that
	// it is armed. The inner render must overwrite it.
	if got["gates"] != CageGatesDir("p") {
		t.Errorf("the inner render must OVERRIDE the forwarded host RHQ_GATES_DIR (%s), got %q", os.Getenv("RHQ_GATES_DIR"), got["gates"])
	}
	if strings.HasPrefix(got["gates"], a.Home) || strings.HasPrefix(got["gates"], "/Users/") {
		t.Errorf("RHQ_GATES_DIR inside names a HOST path (%q) — the shims there exec host binaries and die", got["gates"])
	}
}

// Item 3's parity is pinned against a fake engine whose `inner:` is the
// word `false`. That proves the wiring, not the question: an image with no
// Linux posse in it must be CAUGHT, and the tier must then claim nothing it
// does not hold. Asked of a real image that really has no posse.
func TestQALiveParityCatchesARealImageWithNoInnerPosse(t *testing.T) {
	a, e := qaLiveCageApp(t)
	qaLiveGuard(t, a, e)
	claude, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit, Bash(git push:*)]\n")

	// The image this posse built answers yes.
	if !a.CageInnerGatesReady(e, a.CageImage()) {
		t.Fatalf("%s answers no to `posse gates wrap %s` — rebuild it before reading anything else here", a.CageImage(), GatesWrapProbe)
	}
	// A real, present, perfectly good image that is not one `posse cage
	// build` made. Present matters: `CageImageBuilt` must not be what
	// catches this, or the check is "did we tag it", not "can it render".
	const bare = "alpine:latest"
	if !a.CageImageBuilt(e, bare) {
		t.Skipf("%s is not pulled on this host", bare)
	}
	if a.CageInnerGatesReady(e, bare) {
		t.Fatalf("%s carries no Linux posse and must answer NO to the inner probe", bare)
	}
	// And parity must then say so, for the shell verb AND for the mount
	// boundary's gate, naming the fix rather than just failing.
	os.WriteFile(a.ConfigPath, []byte("default_engine: notty\ncage_image: "+bare+"\n"), 0o644)
	u := strings.Join(a.CheckParity(ag, claude, CageContainer, TierStrong).Unrealized, "\n")
	for _, want := range []string{"Bash(git push:*)", "Edit", "posse cage build", bare} {
		if !strings.Contains(u, want) {
			t.Errorf("parity on %s must name %q — an unrealized gate that is not named is a gate the operator thinks they have:\n%s", bare, want, u)
		}
	}
}

// Item 5 says an unknown socket name refuses. CheckSockets is pinned
// hermetically; the claim is about the LAUNCH, which is where a persona
// would meet it — and a name nothing mounts must not become a silent no-op
// on the way there.
func TestQALiveUnknownSocketRefusesTheLaunchItself(t *testing.T) {
	a, e := qaLiveCageApp(t)
	qaLiveGuard(t, a, e)
	rt, _ := a.LoadRuntime("claude")
	dir := liveCageRepo(t, "echo ran=yes\n")

	ag := cageAgent(t, a, "cage: container\nsockets: [docker]\ndeny: [Edit]\n")
	ag.EnsureMemoryDir()
	env := append(CageEnvNames(nil), "CLAUDE_CODE_OAUTH_TOKEN")
	_, err := a.WrapInCage(ag, rt, "qa-badsock", dir, "sh ./probe.sh", env, "")
	if err == nil {
		t.Fatal("`sockets: [docker]` must refuse the launch: a name nothing mounts reads in the PID as a capability that was granted")
	}
	if !strings.Contains(err.Error(), "docker") {
		t.Errorf("the refusal must name the socket it did not know: %v", err)
	}
	if _, statErr := os.Stat(a.CageArgvFile(ag.Name, "qa-badsock")); statErr == nil {
		t.Error("a refused launch must leave no plan behind for a launcher to pick up")
	}
	// The known name still launches — the refusal is about the name, not
	// about `sockets:` being typed at all.
	held := cageAgent(t, a, "cage: container\nsockets: [herdr]\ndeny: [Edit]\n")
	held.EnsureMemoryDir()
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(shortTempDir(t), "herdr.sock"))
	if err := CheckSockets(held); err != nil {
		t.Errorf("`sockets: [herdr]` is the one known name: %v", err)
	}
}

// Two escapes tried against a cage whose repo is READ-WRITE — a PID that
// denies a shell verb and nothing else, so the mount boundary is not there
// to catch what the gates miss:
//
//  1. `env -i`. Both layers read the deny list out of the environment
//     (RHQ_TOOLS_DENY) and L1 is a PATH shim. A process that clears its own
//     environment clears both — the same hole the host tier has, since it is
//     the same hook and the same shims, but worth measuring HERE because
//     this tier's promise is "everything shims does, PLUS", and an operator
//     reading "PLUS" should know the base is not sealed.
//  2. The audit trail. gates/<persona>/refusals.log is bind-mounted into
//     the container READ-WRITE, because the inner shims have to append to
//     it. That also means the caged persona can rewrite the record of its
//     own refusals. Recorded rather than wished away.
//
// This test asserts what the tier DOES hold: it does not assert the escapes
// fail, it pins the answer either way so the day one closes is noticed.
func TestQALiveCageEscapeAttemptsOnAWritableRepo(t *testing.T) {
	a, e := qaLiveCageApp(t)
	// The one QA pin here whose clauses read the INNER RENDER (the shim's
	// refusal and gates/<persona>/refusals.log), so the one that needs the
	// image to be this source's — ranger-base-nwj7. The other three are
	// about host-side mounts, the remount refusal and the parity wiring,
	// and a staleness skip on those would cost more than it caught.
	requireCurrentCageImage(t, a, e, qaLiveGuard(t, a, e))
	dir := liveCageRepo(t, "")
	probe := `
echo "shim=$(git push origin main 2>&1 | head -1)"
echo "hook=$(/usr/bin/git push origin main 2>&1 | grep -ci refused)"
echo "stripped=$(env -i /usr/bin/git push origin main 2>&1 | grep -ci refused)"
echo "noverify=$(/usr/bin/git push --no-verify origin main 2>&1 | grep -ci refused)"
echo "hookspath=$(/usr/bin/git -c core.hooksPath= push origin main 2>&1 | grep -ci refused)"
echo "combined=$(/usr/bin/git -c core.hooksPath=/tmp push origin main 2>&1 | grep -ci refused)"
`
	deny := []string{"Bash(git push:*)"}
	session := "qa-rw"
	got := qaRunCage(t, a, e, "cage: container\ndeny: ["+strings.Join(deny, ", ")+"]\n", dir, session, deny, probe)

	// The repo really is writable here — otherwise the escapes would be
	// "blocked" by a boundary this case is meant to have removed.
	if got["shim"] == "" || !strings.Contains(got["shim"], "refused by posse gate") {
		t.Fatalf("L1 must still refuse on a read-write repo, got %q", got["shim"])
	}
	if got["hook"] != "1" {
		t.Errorf("L3's pre-push hook must refuse when the env is intact, got %q", got["hook"])
	}

	// (1) The measured answer to the env strip. A change either way is a
	// change to what this tier is worth, and belongs in a comment before it
	// belongs in a release.
	switch got["stripped"] {
	case "1":
		t.Logf("MEASURED: `env -i /usr/bin/git push` is STILL refused — the hook does not depend on the caller's environment")
	case "0":
		t.Logf("MEASURED (rangerhq-pafo, 2026-08-27): `env -i /usr/bin/git push` is NOT refused. Both L1 and L3 read the deny list out of the environment, so a process that clears its own clears the gates with it. This is the shims tier's own hole, inherited unchanged — the container tier adds the mount boundary on top of it, not a seal underneath it.")
	default:
		t.Errorf("the strip probe did not run: %q", got["stripped"])
	}

	// (1b)-(1d) ADR 0025 verification 4 / the escape-C amendment to ADR 0002
	// §3 (ranger-base-3csb, docs/adr/0002-runtimes-and-gates.md "Amendment
	// 2026-08-27"): L3 is a cooperative backstop, not the boundary for the
	// absolute-path hole. MEASURED there (git 2.39.3, ungated scratch: bare
	// local remote, a refusing pre-push, /usr/bin/git throughout; ground
	// truth was which refs landed on the remote) — `--no-verify` and an
	// empty `core.hooksPath=` both skip the hook outright, and the two
	// combined (`/usr/bin/git -c core.hooksPath=<hookless> push`) defeat L1
	// (absolute path) and L3 (redirect) together, with zero writes to
	// .git/hooks. Pinned here too, either way, per ranger-base-evb1's own
	// comment thread: log the measured answer, do not assert the escape
	// must keep working — the day one of these flips is the day someone
	// notices (the ranger-base-6uq6 pattern).
	switch got["noverify"] {
	case "0":
		t.Logf("MEASURED (ranger-base-3csb): `/usr/bin/git push --no-verify` is NOT refused — --no-verify skips pre-push outright, cooperative class (ADR 0025 §1)")
	case "1":
		t.Logf("MEASURED: `/usr/bin/git push --no-verify` is still refused here (%q) — narrower than the ranger-base-3csb scratch measurement", got["noverify"])
	default:
		t.Errorf("the --no-verify probe did not run: %q", got["noverify"])
	}
	switch got["hookspath"] {
	case "0":
		t.Logf("MEASURED (ranger-base-3csb): `/usr/bin/git -c core.hooksPath= push` is NOT refused — the redirect finds no pre-push at the (empty) path, cooperative class, zero writes to .git/hooks")
	case "1":
		t.Logf("MEASURED: `/usr/bin/git -c core.hooksPath= push` is still refused here (%q)", got["hookspath"])
	default:
		t.Errorf("the core.hooksPath= probe did not run: %q", got["hookspath"])
	}
	switch got["combined"] {
	case "0":
		t.Logf("MEASURED (escape C, ranger-base-3csb): `/usr/bin/git -c core.hooksPath=<hookless> push` is NOT refused, defeating L1 (absolute path) and L3 (redirect) in one call — while a plain `git push` in this same session IS refused (%q), so the gates are otherwise live", got["shim"])
	case "1":
		t.Logf("MEASURED: the combined escape-C form is still refused here (%q)", got["combined"])
	default:
		t.Errorf("the combined escape-C probe did not run: %q", got["combined"])
	}

	// (2) ADR 0025 §4's flip of the rangerhq-pafo finding: the canonical log
	// is never mounted any more, so what a caged persona can truncate is
	// only its own session's SPOOL — and truncating that no longer erases
	// anything the operator reads. Fold once first, so the canonical log
	// holds this session's git-push refusal and the cursor holds a non-zero
	// offset; only then does a truncate have a fold-recorded state to lie
	// about.
	canonical := a.RefusalsLogPath("p")
	if err := a.FoldRefusalsSpool("p", session); err != nil {
		t.Fatalf("fold before truncate: %v", err)
	}
	before, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("canonical log must exist after a fold: %v", err)
	}
	if !strings.Contains(string(before), "session="+session) || !strings.Contains(string(before), "git push") {
		t.Fatalf("the fold must have moved the git-push refusal into the canonical log, tagged with its session:\n%s", before)
	}

	// The same session's spool, attacked in a second run — a relaunch's own
	// shape, since a caged process is never mid-session when the operator's
	// host asks it anything.
	trunc := qaRunCage(t, a, e, "cage: container\ndeny: ["+strings.Join(deny, ", ")+"]\n", dir, session, deny,
		`if : > "$RHQ_GATES_DIR/refusals.log" 2>/dev/null; then echo "truncate=ok"; else echo "truncate=refused"; fi`+"\n")
	if trunc["truncate"] != "ok" {
		t.Errorf("truncating the SPOOL from inside must still succeed — it is a session-local file the persona owns, not the audit trail: %q", trunc["truncate"])
	}
	after, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("canonical log must survive the container: %v", err)
	}
	if len(after) < len(before) {
		t.Errorf("the host canonical log must never SHRINK from inside the cage — it was never mounted (ADR 0025 §4); before=%d after=%d", len(before), len(after))
	}

	// The next fold is where the truncation becomes visible: the spool is
	// now shorter than the cursor's own offset, so this is TAMPER, not "no
	// new lines" — the erasure attempt becomes evidence (ADR 0025 §4
	// verification 2).
	if err := a.FoldRefusalsSpool("p", session); err != nil {
		t.Fatalf("fold after truncate: %v", err)
	}
	final, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("canonical log must survive the second fold: %v", err)
	}
	if !strings.Contains(string(final), "refusals spool tampered [fold] session="+session) {
		t.Errorf("a spool shorter than its cursor must fold as TAMPERED, naming the session:\n%s", final)
	}

	// Whatever the two above answer, the boundary the ADR actually promises
	// at this tier must not move: a PID that denies Edit/Write still gets a
	// repo no `env -i` can write to, because a read-only bind mount is not
	// a rule a process can talk its way out of.
	sealed := qaRunCage(t, a, e, "cage: container\ndeny: [Bash(git push:*), Edit, Write]\n", dir, "qa-sealed",
		[]string{"Bash(git push:*)", "Edit", "Write"},
		"if env -i /bin/touch ./stripped-write 2>/dev/null; then echo stripped_write=wrote; else echo stripped_write=refused; fi\n")
	if sealed["stripped_write"] != "refused" {
		t.Errorf("the mount boundary is kernel-side and must survive an emptied environment, got %q", sealed["stripped_write"])
	}
}

// The one attack the container tier has that the host tier does not: the
// process inside is ROOT. A :ro bind mount is the whole mount boundary of
// ADR 0002 §4, and root with CAP_SYS_ADMIN can remount. If docker's default
// capability set left that reachable, item 4 would be a suggestion.
func TestQALiveRootInsideCannotRemountTheBoundaryWritable(t *testing.T) {
	a, e := qaLiveCageApp(t)
	qaLiveGuard(t, a, e)
	dir := liveCageRepo(t, "")
	probe := `
echo "uid=$(id -u)"
echo "remount=$(mount -o remount,rw . 2>&1 | head -1 | tr -d '\n' | cut -c1-60)"
if touch ./after-remount 2>/dev/null; then echo "wrote=yes"; else echo "wrote=no"; fi
echo "bindover=$(mount --bind /tmp . 2>&1 | head -1 | tr -d '\n' | cut -c1-60)"
if touch ./after-bind 2>/dev/null; then echo "bound=yes"; else echo "bound=no"; fi
`
	deny := []string{"Bash(git push:*)", "Edit", "Write"}
	got := qaRunCage(t, a, e, "cage: container\ndeny: ["+strings.Join(deny, ", ")+"]\n", dir, "qa-root", deny, probe)
	if got["uid"] != "0" {
		t.Logf("MEASURED: the caged process is uid %s, not root — this attack needed root to be interesting", got["uid"])
	}
	if got["wrote"] != "no" {
		t.Errorf("root inside REMOUNTED the repo read-write (%q) — the mount boundary is not a boundary", got["remount"])
	}
	if got["bound"] != "no" {
		t.Errorf("root inside bind-mounted over the repo (%q) — the boundary can be papered over from within", got["bindover"])
	}
	if _, err := os.Stat(filepath.Join(dir, "after-remount")); err == nil {
		t.Error("a file the remount attack wrote reached the host repo")
	}
	if _, err := os.Stat(filepath.Join(dir, "after-bind")); err == nil {
		t.Error("a file the bind-over attack wrote reached the host repo")
	}
}
