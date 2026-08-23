package rhq

// L1/L3 inside the cage, the mount boundary, and `sockets:` (ADR 0002 §3
// as amended by rangerhq-rm5; bead rangerhq-6so) — the half of the
// container tier that makes the tiers *cumulative in gates realized*.
//
// The ADR's rule is short and it is the whole design: the strongest cage
// is never allowed to be the one that silently loses `git push`. L1 and L3
// do not follow a process into a container by themselves — a shim `exec`s
// the real binary resolved at render time on the *host*
// (/opt/homebrew/bin/git is not in a Linux image) and the gate shell points
// at the host's zsh — so mounting the host's `gates/<persona>/` in would
// install a wall of dead scripts, and a shim that dies is a shell verb that
// is NOT refused. Render where the binaries are:
//
//	inner command = posse gates wrap <persona> -- sh -c 'exec <runtime cmd>'
//
// The image's own Linux posse (rangerhq-9fv) runs the SAME renderer
// (RenderGates + renderGateShell) against the image's PATH and shell, then
// execs what follows it with the same typed prefix the host types
// (GatePrefix: PATH=<bin>:$PATH SHELL=<gate shell>). Three things make
// that work and each is a decision rather than a detail:
//
//   - The deny list crosses as RHQ_TOOLS_DENY, not as the PID. The PID is
//     mounted at its host path for `$(cat {file})`, but RHQ_HOME is not in
//     the cage at all, so `LoadAgent` has nothing to read — and the env var
//     is already the launch's own rendering of `deny:`, the very list the
//     pre-push hook on the repo mount reads. One source, both layers.
//   - The gates render to a path of the IMAGE's, /posse/gates/<persona>, not
//     to a mount. Two caged sessions of one persona are two containers, and
//     a shared render dir would have each clearing the other's shims — the
//     same reason the host's gates dir must not be mounted in. What *is*
//     mounted is the single file that has to outlive the container:
//     `refusals.log`, so an inner refusal lands in the same audit trail as
//     an L1 refusal on the host and as the egress proxy's 403s.
//   - L3 rides in on its own. `.git/hooks/pre-push` is POSIX sh on the repo
//     mount and reads only RHQ_TOOLS_DENY / RHQ_GATES_DIR / RHQ_PERSONA,
//     all forwarded — except in a worktree, where `.git` is a *file*
//     pointing at the repo's common dir somewhere else on the host. That
//     dir is mounted too (CageMounts), because a hook the container cannot
//     see is a `git push` this tier lost.
//
// If the inner render cannot happen — the image carries no Linux posse — the
// shell-verb denies are unrealized at this tier and the launch refuses like
// any other (parity.go). That is why CageInnerGatesReady is a probe of the
// image and not a hope about it.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// CageStateRoot is posse's state root INSIDE the container: `gates/<persona>`
// under it is where the inner render writes. A fixed absolute path because
// the image is the launch's own (`posse cage build`), and because the host
// has to know it in advance — it is the destination of the refusals.log
// mount and the value of RHQ_GATES_DIR in the caged session's env.
const CageStateRoot = "/posse"

// CageStateApp is the App the inner posse renders gates with: a state root
// and nothing else. It is deliberately NOT NewApp() — RHQ_HOME does not
// cross the boundary, and an App that thought it had one would look for
// agents, recipes and config in a directory the cage does not have.
func CageStateApp() *App { return &App{StateDir: CageStateRoot} }

// CageGatesDir is gates/<persona> as it exists inside the container.
func CageGatesDir(persona string) string { return CageStateApp().GatesDir(persona) }

// RefusalsLogPath is gates/<persona>/refusals.log on the host — L1's audit
// trail, which the cage mounts so an inner refusal is not lost with the
// container. RefusalsLog creates it; this only names it.
func (a *App) RefusalsLogPath(persona string) string {
	return filepath.Join(a.GatesDir(persona), "refusals.log")
}

// ─── `posse gates wrap` — the inner command ────────────────────────────────────

// GatesWrapProbe is what the host asks the image to answer before claiming
// the tier renders gates inside: `posse gates wrap --probe` prints the dir it
// would render into and exits 0. A version check would answer "some posse is
// here"; this answers "a posse that knows this entry point is here", which
// is the fact parity needs.
const GatesWrapProbe = "--probe"

// GatesWrapNoShell is the host's way of telling the inner render that this
// runtime opts out of the gate shell (Runtime.NoGateShell, ADR 0009 §2).
// It is a flag and not RHQ_RUNTIME because a template-only runtime's yaml
// lives under RHQ_HOME, which is not in the cage: the host is the only side
// that can read it, so the host decides.
const GatesWrapNoShell = "--no-gate-shell"

// GatesWrapArgv is the prefix the container's inner command leads with.
func GatesWrapArgv(persona string, rt *Runtime) []string {
	argv := []string{"posse", "gates", "wrap", persona}
	if rt != nil && rt.NoGateShell {
		argv = append(argv, GatesWrapNoShell)
	}
	return append(argv, "--")
}

// GatesWrap is a rendered inner launch: what to exec, with which argv, in
// which environment — and what was rendered on the way, so `--probe` and
// the tests can read it without an exec.
type GatesWrap struct {
	Path  string   // the executable, resolved on the ordinary PATH
	Argv  []string // argv[0] as given: this is a wrapper, not a launcher
	Env   []string // the caller's env plus the gate prefix, as name=value
	Gates string   // the dir the gates were rendered into (RHQ_GATES_DIR)
	Bin   string   // the shim dir now first on PATH
	Shell string   // the gate shell ("" when the runtime opted out)
}

// DenyFromEnv reads the PID's deny list out of RHQ_TOOLS_DENY, which is how
// it crosses into the cage (newline-joined by CreateSession, forwarded by
// name so no value is ever on the typed line).
func DenyFromEnv() []string {
	var out []string
	for _, r := range strings.Split(os.Getenv("RHQ_TOOLS_DENY"), "\n") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// PrepareGatesWrap renders the persona's gates and returns the exec that
// puts them in front of argv. Separate from the exec itself so the whole
// decision is testable: an `execve` is not something a test can observe
// from the inside.
func (a *App) PrepareGatesWrap(persona string, deny []string, noGateShell bool, argv, env []string) (*GatesWrap, error) {
	if len(argv) == 0 {
		return nil, Die("posse gates wrap: nothing to run after `--`")
	}
	// Resolved on the caller's PATH, before the shims go in front of it:
	// what follows `--` is posse's own rendering (`sh -c …`), not the
	// persona's typing, and a PID that denies `sh` must not be a PID that
	// cannot launch.
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, Die("posse gates wrap: %s is not in the image (%v)", argv[0], err)
	}
	gatesDir, binDir, shell, err := a.RenderGates(persona, deny)
	if err != nil {
		return nil, err
	}
	w := &GatesWrap{Path: path, Argv: argv, Gates: gatesDir, Bin: binDir, Shell: shell}
	if noGateShell {
		w.Shell = ""
	}
	w.Env = envPut(env, "PATH", binDir+":"+envGet(env, "PATH"))
	// The session's own tools read this — the pre-push hook above all, which
	// appends its refusal to $RHQ_GATES_DIR/refusals.log. Inside the cage
	// that has to name the inner render, not the host's.
	w.Env = envPut(w.Env, "RHQ_GATES_DIR", gatesDir)
	if w.Shell != "" {
		w.Env = envPut(w.Env, "SHELL", w.Shell)
		w.Env = envPut(w.Env, "GROK_SHELL", w.Shell)
	}
	return w, nil
}

// RunGatesWrap is `posse gates wrap <persona> [--no-gate-shell] -- <argv…>`,
// the inner command of a container launch. It returns only on failure — on
// success this process has become the runtime, behind the wall.
func RunGatesWrap(args []string, out io.Writer) error {
	persona, noShell, probe := "", false, false
	var argv []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == GatesWrapNoShell:
			noShell = true
		case args[i] == GatesWrapProbe:
			probe = true
		case args[i] == "--":
			argv = args[i+1:]
			i = len(args)
		case persona == "":
			persona = args[i]
		default:
			return Die("posse gates wrap: unexpected argument %q (usage: posse gates wrap <persona> [%s] -- <command>)", args[i], GatesWrapNoShell)
		}
	}
	a := CageStateApp()
	if probe {
		// The host's readiness question, answered without rendering
		// anything: an image whose posse prints this knows the entry point.
		fmt.Fprintf(out, "%s\n", CageGatesDir(persona))
		return nil
	}
	if persona == "" {
		return Die("usage: posse gates wrap <persona> [%s] -- <command>  (posse's inner cage command — rendered at launch, not typed by hand)", GatesWrapNoShell)
	}
	w, err := a.PrepareGatesWrap(persona, DenyFromEnv(), noShell, argv, os.Environ())
	if err != nil {
		return err
	}
	return syscall.Exec(w.Path, w.Argv, w.Env)
}

// envGet reads a name out of a name=value slice ("" when absent).
func envGet(env []string, key string) string {
	for _, kv := range env {
		if name, val, ok := strings.Cut(kv, "="); ok && name == key {
			return val
		}
	}
	return ""
}

// envPut sets a name in a name=value slice, replacing every earlier
// spelling of it (execve takes the last, but a duplicate PATH in a
// process's environment is the kind of thing that misleads the next
// debugging session).
func envPut(env []string, key, val string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if name, _, ok := strings.Cut(kv, "="); !ok || name != key {
			out = append(out, kv)
		}
	}
	return append(out, key+"="+val)
}

// ─── is the image ready to render inside? ────────────────────────────────────

var (
	innerProbeMu sync.Mutex
	innerProbe   = map[string]bool{} // engine+image → the answer, per process
)

// CageInnerGatesReady asks the image whether it can render the gates
// inside: `<engine inner> posse gates wrap --probe`. Cached per process,
// because parity is computed once per launch but several times per dispatch
// pass and the answer cannot change under a running posse.
//
// An engine that spells no `inner:` answers YES, on `probe:`'s precedent
// (cage.go): it has no way to be asked, and a launch that fails at the
// engine is a better error than a refusal this package invented. An engine
// that CAN be asked and says no is a real unrealized gate — the image is
// not built, or is not one `posse cage build` made.
func (a *App) CageInnerGatesReady(e *Engine, image string) bool {
	if !ContainerInnerGates || e == nil {
		return false
	}
	if strings.TrimSpace(e.Inner) == "" {
		return true
	}
	if !a.CageImageBuilt(e, image) {
		return false
	}
	// Keyed on the template too, not just the engine's name: two Apps in one
	// process (a test suite, a cockpit) can hold two engines called the same
	// thing, and the answer belongs to the spelling that would be run.
	key := e.Name + "\x00" + e.Inner + "\x00" + image
	innerProbeMu.Lock()
	defer innerProbeMu.Unlock()
	if v, ok := innerProbe[key]; ok {
		return v
	}
	argv := e.stepArgv(e.Inner, CageRender{
		Image: image, Inner: []string{"posse", "gates", "wrap", GatesWrapProbe},
	})
	v := len(argv) > 0 && exec.Command(argv[0], argv[1:]...).Run() == nil
	innerProbe[key] = v
	return v
}

// CageInnerGates is that question for this RHQ_HOME's resolved engine and
// image — what parity asks before claiming L1/L3 and the mount boundary at
// the container tier.
func (a *App) CageInnerGates() bool {
	e, err := a.LoadEngine(a.ResolveEngine())
	return err == nil && a.CageInnerGatesReady(e, a.CageImage())
}

// ─── the mount boundary ──────────────────────────────────────────────────────

// deniesFileWrite reports whether the PID's deny list carries a rule the
// mount boundary realizes. Any one of the three mounts the repo `:ro`: the
// boundary is a property of the mount, not of a rule, so a PID that denies
// `Write` alone gets `Edit` refused too — enforcing more than was asked is
// safe in this direction, and pretending the mount is per-rule would not be.
func deniesFileWrite(deny []string) bool {
	for _, r := range deny {
		switch r {
		case "Edit", "Write", "NotebookEdit":
			return true
		}
	}
	return false
}

// gitCommonDirOutside names the repo's git common dir when it is NOT under
// dir — which is exactly the worktree case: `<dir>/.git` is a file holding
// an absolute path into the main repo's `.git`, and everything git needs,
// `hooks/` included, lives there. Same path in and out like every other
// mount, so the pointer file resolves inside the cage to the thing it
// points at outside. "" when there is nothing extra to mount (an ordinary
// repo, or not a repo at all).
func gitCommonDirOutside(dir string) string {
	hooks, err := hooksDir(dir)
	if err != nil {
		return ""
	}
	common := filepath.Dir(hooks)
	if abs, err := filepath.Abs(common); err == nil {
		common = abs
	}
	// git answers with the RESOLVED path, so an ordinary repo reached through
	// a symlinked parent (/tmp on macOS) would otherwise look like a worktree
	// and earn a second mount of its own .git. Compare like with like.
	real := dir
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		real = r
	}
	for _, d := range []string{dir, real} {
		if common == d || strings.HasPrefix(common, d+string(filepath.Separator)) {
			return ""
		}
	}
	return common
}

// ─── sockets: (ADR 0002 §3, §5) ──────────────────────────────────────────────

// CageSocketHerdr is the only socket name the cage knows. The herdr socket
// is a FLEET-WIDE capability — a persona holding it can prompt or close
// every other pane — so it is off unless the PID asks for it by name.
// Dispatch *to* a caged session never needs it (herdr prompts the pane from
// the host); only a persona that itself dispatches does, and that persona
// is not the one you cage.
const CageSocketHerdr = "herdr"

// CageSockets are the names a PID may list. Declared, not refused: ADR 0002
// makes this an opt-in the operator can see (session meta, `posse list`, the
// cockpit's `container+herdr`), not a parity shortfall.
var CageSockets = []string{CageSocketHerdr}

// CheckSockets refuses a `sockets:` entry nothing can mount. An unknown
// name is a PID error and not a silent no-op, for the {model} reason: a
// placeholder that renders to nothing reads as a capability that was given.
func CheckSockets(ag *AgentFile) error {
	for _, s := range ag.Sockets {
		if s != CageSocketHerdr {
			return Die("%s: sockets: %q is not a socket this cage knows (only %s) — remove it or spell it right; a name that mounts nothing reads in the PID as a capability the session was given", ag.Name, s, strings.Join(CageSockets, ", "))
		}
	}
	return nil
}

// CageSocketMounts is what `sockets:` adds to the boundary. The path is the
// host's own, mounted at the same path inside like everything else, and
// read-write because a unix socket that cannot be written to cannot be
// spoken to.
func (a *App) CageSocketMounts(ag *AgentFile) []CageMount {
	var ms []CageMount
	for _, s := range ag.Sockets {
		if s != CageSocketHerdr {
			continue
		}
		if p := herdrSocketPath(); p != "" {
			ms = append(ms, CageMount{Src: p, Dst: p, Why: "sockets: herdr — the fleet-wide capability this PID asked for (ADR 0002 §3)"})
		}
	}
	return ms
}

// CageSocketVars points the tools inside at the socket they were given.
// HOME is the image's inside the cage, so posse's own default resolution
// (~/.config/herdr/herdr.sock) would look in /root and find nothing — the
// launch knows the answer, so it says it rather than leaving the session to
// guess. A computed value, not a forwarded name: it has no existence in the
// pane's environment to be forwarded from.
func CageSocketVars(ag *AgentFile) []EnvVar {
	for _, s := range ag.Sockets {
		if s == CageSocketHerdr {
			if p := herdrSocketPath(); p != "" {
				return []EnvVar{{"HERDR_SOCKET_PATH", p}}
			}
		}
	}
	return nil
}

// CageSocketTag is what the meta records and the cockpit shows: the socket
// names, comma-joined, "" when the PID declared none.
func CageSocketTag(ag *AgentFile) string {
	var ok []string
	for _, s := range ag.Sockets {
		if s == CageSocketHerdr {
			ok = append(ok, s)
		}
	}
	return strings.Join(dedupeStrings(ok), ",")
}

// CageTag is how a session's cage reads in `posse list` and the cockpit: ""
// at the default tier (every session has that wall, so saying so is noise),
// the tier above it — and the host sockets the cage was given, because a
// caged persona holding the herdr socket has a reach the cage otherwise
// takes away, and the operator should read that on the session line.
func CageTag(cage, sockets string) string {
	if cage == "" || cage == DefaultCage {
		return ""
	}
	tag := "🔒" + cage
	if sockets != "" {
		tag += "+" + strings.ReplaceAll(sockets, ",", "+")
	}
	return tag
}
