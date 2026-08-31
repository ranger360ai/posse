package posse

// The argv0 launcher (ADR 0002 §3 L4, rangerhq-1k1) — how a caged session
// stays visible to herdr through the container boundary.
//
// herdr identifies the agent in a pane by that pane's foreground `argv0`
// and only then applies the manifest whose rules read the screen. The
// scraping half crosses the boundary intact — an OSC title printed inside
// a container arrives in herdr's pane state verbatim — but the identity
// half does not: with `docker run …` on the pane line, argv0 is `docker`,
// `herdr agent explain` answers `agent_not_found`, and the session has no
// status in `posse list` and cannot be dispatched to (measured three ways in
// rangerhq-89a: a real claude in a container is not found; the same claude
// screen on the host resolves to `agent: claude`; `/bin/sleep` symlinked as
// `claude` is *identified* as claude with no claude in sight).
//
// So the pane runs a launcher named after the runtime instead, and the
// launcher execs the engine with argv[0] reset to that name. Two
// constraints came out of the spike and both are structural here:
//
//   - It must be a binary or a symlink to one. A `#!/bin/sh` wrapper hands
//     herdr `argv0=sh`, because the kernel gives the interpreter its own
//     argv[0]. The binary is *this* one: `state/cages/<persona>/bin/claude`
//     is a symlink to the running posse, which recognizes the second entry
//     point below and never reaches the CLI. Nothing to build, nothing to
//     ship, nothing to keep in step with posse's own version.
//   - It lives in its own directory, never `gates/<persona>/bin`, whose
//     entries are refusing shims on the session's PATH — a launcher named
//     `claude` next to a gate named `git` is a collision waiting to happen,
//     and this directory is on nobody's PATH.
//
// The exec is a real `execve`, so the pane's foreground process *is* the
// engine's: nothing of posse lingers, and when the container exits the pane's
// process is gone. That is what a relaunch (RelaunchAgent) rests on — it
// re-renders and re-types this same short line.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// CageLaunchFlag is posse's second entry point. `<launcher> --posse-cage
// <file>` is the whole protocol: everything else about the launch is in
// the file, because the pane line has to stay short (rangerhq-ybec).
const CageLaunchFlag = "--posse-cage"

// CageReapFlag is the third entry point, and the answer to "the proxy
// starts with the cage and dies with it" (rangerhq-9d0). The launcher
// `execve`s the engine, so by design nothing of posse is left in the pane to
// notice the container exiting — so before the exec it forks one small
// watcher, `<posse> --posse-cage-reap <plan> <pid>`, in its own session. The
// pid it is given is the launcher's, which the exec does not change: the
// watcher's parent IS the engine process, and the moment that parent goes
// away (getppid() stops being it) the cage is over and the route comes
// down. No engine events, no docker socket, no polling of container state
// — a pid, which every engine leaves in the pane.
const CageReapFlag = "--posse-cage-reap"

// cageReapPoll is how often the watcher asks whether its parent is still
// the engine. The cost of the interval is how long a proxy outlives a dead
// cage; the cost of a shorter one is a wakeup per session per interval.
const cageReapPoll = 500 * time.Millisecond

// CageLaunchPlan is that file: what to exec, and the argv to exec it with —
// argv[0] already the runtime's name, which is the entire point. It is
// written next to the persona's cage state and rendered fresh at every
// launch; `note` is there for the operator who opens it while debugging.
type CageLaunchPlan struct {
	Note string   `json:"note"`
	Path string   `json:"path"` // the engine binary, resolved at render time
	Argv []string `json:"argv"` // argv[0] is the runtime's name, not the engine's
	Line string   `json:"line"` // the same argv as a human reads it; nothing execs this
	// Egress is the route the cage joins, brought up before the exec and
	// taken down by the watcher when the engine exits (egress.go). nil for
	// an engine that cannot express it — a launch that got that far has
	// already been refused unless the PID has no egress gate at all.
	Egress *CageEgress `json:"egress,omitempty"`
}

// cageExeName is the shape a runtime's executable name must have to be a
// launcher: a plain file name. herdr matches its manifests on that name.
var cageExeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// CageBinDir is where the launcher lives — its own directory under the
// persona's cage state, deliberately not the gates bin.
func (a *App) CageBinDir(persona string) string {
	return filepath.Join(a.CageDir(persona), "bin")
}

// CageLauncher is the path the pane runs for a session on this runtime.
// The name is the runtime's, so one persona caged on two runtimes gets two
// launchers and neither is ambiguous about what herdr should see.
func (a *App) CageLauncher(persona, exe string) string {
	return filepath.Join(a.CageBinDir(persona), exe)
}

// CageArgvFile is the launch plan for one *session*: a persona can hold
// several (one per bead), and their mounts and workdirs differ.
func (a *App) CageArgvFile(persona, session string) string {
	return filepath.Join(a.CageDir(persona), session+".argv")
}

// cageLauncherBin is the binary a launcher symlink points at: this one.
// A variable so a test can point it at something it can observe being
// exec'd; there is no configuration for it.
var cageLauncherBin = func() (string, error) { return os.Executable() }

// RenderCageLauncher (re)creates state/cages/<persona>/bin/<exe> as a
// symlink to the running posse, and returns its path. Rendered fresh at
// every launch like the gates: a stale symlink from a moved binary is
// replaced rather than trusted.
func (a *App) RenderCageLauncher(persona, exe string) (string, error) {
	if !cageExeName.MatchString(exe) {
		return "", Die("cage container: %q is not a usable launcher name — herdr identifies a caged session by the pane's argv0, so the runtime's executable name has to be a plain file name", exe)
	}
	target, err := cageLauncherBin()
	if err != nil {
		return "", Die("cage container: cannot locate this posse binary for the argv0 launcher: %v", err)
	}
	if err := os.MkdirAll(a.CageBinDir(persona), 0o755); err != nil {
		return "", err
	}
	path := a.CageLauncher(persona, exe)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Symlink(target, path); err != nil {
		return "", err
	}
	return path, nil
}

// WriteCageLaunch renders the launcher and its plan and returns the line
// the pane runs. engine is the resolved engine binary; argv is what the
// engine would have been given, argv[0] included — it is replaced here by
// the runtime's name, which is the only reason this whole path exists.
func (a *App) WriteCageLaunch(persona, session string, rt *Runtime, engine string, argv []string, eg *CageEgress) (string, error) {
	if len(argv) == 0 {
		return "", Die("cage container: engine template rendered no command")
	}
	exe := rt.Exe()
	launcher, err := a.RenderCageLauncher(persona, exe)
	if err != nil {
		return "", err
	}
	plan := CageLaunchPlan{
		Argv:   append([]string{exe}, argv[1:]...),
		Path:   engine,
		Egress: eg,
		Note: "posse cage launch for " + persona + " on " + rt.Name + " (session " + session +
			") — rendered from the PID at launch; do not edit. `" + filepath.Base(launcher) +
			"` execs path with this argv, whose argv[0] is the runtime's name so herdr identifies the caged session (rangerhq-1k1).",
	}
	plan.Line = CageLine(append([]string{engine}, argv[1:]...))
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", err
	}
	file := a.CageArgvFile(persona, session)
	if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	// rangerhq-9fv typed `sh launch.sh`; that shell is gone, and so is the
	// script — leaving a rendering behind that nothing runs is how the next
	// debugging session gets misled.
	os.Remove(filepath.Join(a.CageDir(persona), "launch.sh"))
	return shellQuote(launcher) + " " + CageLaunchFlag + " " + shellQuote(file), nil
}

// IsCageLaunch reports whether this argv is the launcher's, not the CLI's.
// The symlink's own name is the runtime's (`claude`), so the subcommand
// table is never what an operator meant here.
func IsCageLaunch(argv []string) bool {
	return len(argv) > 1 && argv[1] == CageLaunchFlag
}

// IsCageReap reports whether this argv is the egress watcher's.
func IsCageReap(argv []string) bool {
	return len(argv) > 1 && argv[1] == CageReapFlag
}

// RunCageLaunch is the launcher: read the plan, become the engine. It
// returns only on failure — on success the process has been replaced.
func RunCageLaunch(argv []string) error {
	if len(argv) != 3 {
		return Die("usage: %s %s <launch plan>  (posse's cage launcher — rendered at launch, not typed by hand)", filepath.Base(argv[0]), CageLaunchFlag)
	}
	b, err := os.ReadFile(argv[2])
	if err != nil {
		return Die("cage launch plan unreadable: %v", err)
	}
	var plan CageLaunchPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return Die("cage launch plan %s is not readable json: %v", argv[2], err)
	}
	if len(plan.Argv) == 0 || strings.TrimSpace(plan.Path) == "" {
		return Die("cage launch plan %s has no engine to exec", argv[2])
	}
	path := plan.Path
	if !filepath.IsAbs(path) {
		if p, err := exec.LookPath(path); err == nil {
			path = p
		}
	}
	// The route before the cage: the engine's own `--network` on the agent
	// line names a network that has to exist, and a proxy that is not there
	// when the runtime makes its first request is a session that starts by
	// failing (loudly, on codex — ~70 retries in 35s). Errors here abort the
	// launch rather than falling through to a container with no boundary.
	if err := StartCageEgress(plan, argv[2], os.Stderr); err != nil {
		return err
	}
	// The environment is the pane's own, untouched: the engine forwards
	// what crosses the boundary by NAME (-e VAR), so the values it reads
	// are the ones the workspace was created with — the operator's
	// container credential above all.
	return syscall.Exec(path, plan.Argv, os.Environ())
}

// ─── the egress route's lifecycle (rangerhq-9d0) ─────────────────────────────

// runCageSteps runs a plan's engine commands in order. A non-fatal step
// that fails is reported and stepped over: `network create` on a network
// this session's previous cage still holds is the ordinary case, and
// reusing it is what we want — the new proxy joins the same network the
// old agent is on, and the old agent's route out went with the old proxy.
func runCageSteps(steps []CageStep, out io.Writer) error {
	for _, s := range steps {
		if len(s.Argv) == 0 {
			continue
		}
		c := exec.Command(s.Argv[0], s.Argv[1:]...)
		b, err := c.CombinedOutput()
		if err == nil {
			continue
		}
		msg := strings.TrimSpace(string(b))
		if !s.Fatal {
			fmt.Fprintf(out, "posse cage egress: %s — %v (%s)\n", s.Why, err, firstLine(msg))
			continue
		}
		return Die("cage egress: %s failed: %v\n  %s\n  %s", s.Why, err, strings.Join(s.Argv, " "), msg)
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// StartCageEgress brings the route up and leaves a watcher behind to take
// it down. planFile is the launch plan's own path — the watcher re-reads
// it rather than being handed the steps on a command line, because the
// steps are the launch's and the file is where the operator can see them.
func StartCageEgress(plan CageLaunchPlan, planFile string, out io.Writer) error {
	if plan.Egress == nil {
		return nil
	}
	// Fresh at every launch, like the gates: a proxy left behind by a
	// previous launch of this session holds a previous allowlist, so it is
	// replaced rather than reused. Failures are expected (nothing there).
	runCageSteps(plan.Egress.Down, io.Discard)
	if err := runCageSteps(plan.Egress.Up, out); err != nil {
		return err
	}
	return watchCageEgress(planFile, out)
}

// watchCageEgress forks the watcher described at CageReapFlag: our own
// binary, in its own session so it is neither in the pane's foreground
// process group nor anything herdr can mistake for the agent, holding the
// pid this process is about to become the engine under.
func watchCageEgress(planFile string, out io.Writer) error {
	self, err := cageReaperBin()
	if err != nil {
		fmt.Fprintf(out, "posse cage egress: no watcher (%v) — the proxy will outlive this cage; `posse cage down` removes it\n", err)
		return nil
	}
	log, err := os.OpenFile(planFile+".reap.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	c := exec.Command(self, CageReapFlag, planFile, strconv.Itoa(os.Getpid()))
	c.Stdout, c.Stderr = log, log
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		fmt.Fprintf(out, "posse cage egress: watcher would not start (%v) — the proxy will outlive this cage; `posse cage down` removes it\n", err)
		return nil
	}
	// Nothing waits on it: this process is about to be replaced by the
	// engine, and the watcher's whole job is to outlive that replacement.
	return nil
}

// cageReaperBin resolves the posse binary behind the launcher symlink. The
// symlink's name is the runtime's (`claude`), and a background process
// wearing that name is exactly the confusion the argv0 launcher exists to
// avoid — so the watcher runs under posse's own name.
var cageReaperBin = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// RunCageReap is the watcher. It returns when the cage is gone and the
// route with it; the launch plan is re-read here so the steps it runs are
// the ones the operator can see in the file.
func RunCageReap(argv []string) error {
	if len(argv) != 4 {
		return Die("usage: posse %s <launch plan> <engine pid>  (posse's cage egress watcher — forked at launch, not typed by hand)", CageReapFlag)
	}
	ppid, err := strconv.Atoi(argv[3])
	if err != nil {
		return Die("cage egress watcher: %q is not a pid", argv[3])
	}
	b, err := os.ReadFile(argv[2])
	if err != nil {
		return Die("cage egress watcher: launch plan unreadable: %v", err)
	}
	var plan CageLaunchPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return Die("cage egress watcher: launch plan %s is not readable json: %v", argv[2], err)
	}
	if plan.Egress == nil {
		return nil
	}
	fmt.Fprintf(os.Stdout, "%s watching pid %d for %s (launch %s)\n",
		time.Now().UTC().Format(time.RFC3339), ppid, plan.Egress.Proxy, plan.Egress.Launch)
	for os.Getppid() == ppid {
		time.Sleep(cageReapPoll)
	}
	// The fence (CageEgress.Launch). A relaunch of this session kills the
	// pane — which is what woke this watcher — and then re-renders and
	// re-types a launch that owns the SAME network and proxy names. Re-read
	// the plan: if the launch id has moved on, the route standing there is
	// the new cage's and this watcher has nothing to reclaim.
	if cageRouteSuperseded(argv[2], plan.Egress.Launch) {
		fmt.Fprintf(os.Stdout, "%s cage gone, but a newer launch owns %s now — leaving it up\n",
			time.Now().UTC().Format(time.RFC3339), plan.Egress.Proxy)
		return nil
	}
	fmt.Fprintf(os.Stdout, "%s cage gone; taking the route down\n", time.Now().UTC().Format(time.RFC3339))
	// Best effort by construction: the network refuses to go while any
	// container still holds it, which is what happens when the pane was
	// killed and the engine's client died while its container kept running
	// (the engine's own behaviour, not ours). The proxy is removed either
	// way, so that container is left with no route out — fail-closed, which
	// is the right direction for a boundary to fail in.
	runCageSteps(plan.Egress.Down, os.Stdout)
	return nil
}

// cageRouteSuperseded: does the plan on disk now belong to a later launch
// than the one this watcher was forked for? Only a plan that parses moves
// the fence — see reReadLaunch.
func cageRouteSuperseded(planFile, launch string) bool {
	newer, ok := reReadLaunch(planFile)
	return ok && newer != launch
}

// reReadLaunch reads the plan's current launch id. A plan being rewritten
// under us reads as unparseable, which is itself a newer launch — but a
// deleted or truncated plan must NOT be read that way, or a route would
// leak on every torn-down session. So: retry briefly, and only a plan that
// parses gets to move the fence.
func reReadLaunch(planFile string) (string, bool) {
	for i := 0; i < 3; i++ {
		if b, err := os.ReadFile(planFile); err == nil {
			var p CageLaunchPlan
			if json.Unmarshal(b, &p) == nil && p.Egress != nil {
				return p.Egress.Launch, true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", false
}

// TearDownCageEgress is `posse cage down`: the operator's way back from a
// watcher that was killed with its pane. Safe to run when nothing is up.
func (a *App) TearDownCageEgress(persona string, out io.Writer) (int, error) {
	ents, err := os.ReadDir(a.CageDir(persona))
	if err != nil {
		return 0, nil // no cage state for this persona: nothing to take down
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".argv") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.CageDir(persona), e.Name()))
		if err != nil {
			continue
		}
		var plan CageLaunchPlan
		if err := json.Unmarshal(b, &plan); err != nil || plan.Egress == nil {
			continue
		}
		fmt.Fprintf(out, "%s: %s\n", strings.TrimSuffix(e.Name(), ".argv"), plan.Egress.Proxy)
		runCageSteps(plan.Egress.Down, out)
		n++
	}
	return n, nil
}
