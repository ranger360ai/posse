package rhq

// L4 — the container cage (ADR 0002 §3, amended by rangerhq-rm5; built in
// rangerhq-9fv). Two things live here. The *engine*, which is a command
// template and not a hard-coded `docker run`: OrbStack is a drop-in swap
// and Apple `container` is a re-open of the two questions the spike had to
// answer (rangerhq-rli), so the engine belongs behind one line of YAML in
// the shape of runtimes/<name>.yaml — RHQ_HOME/cages/<name>.yaml, chosen
// by config `default_engine:`. And everything the launch must arrange
// before a caged runtime can do a persona's work: the mounts (the repo,
// the persona's memory, its PID, its HOME — nothing else of the host), the
// env *names* forwarded through the boundary, the operator's credential
// (rangerhq-kiz), and the runtime state a fresh container starts without.
//
// The pane does not run the engine directly: it runs the argv0 launcher of
// cagelauncher.go (rangerhq-1k1), because herdr identifies the agent in a
// pane by its foreground argv0 and `docker` is not an agent. That is why
// the rendering here is an *argv* and not a shell line.
//
// The `egress:` half of the tier is next door in egress.go (rangerhq-9d0):
// this file renders the launch, that one renders the route the launch
// joins. The `gates` half is in cageinner.go (rangerhq-6so): the inner
// command that renders L1/L3 against the image's own PATH and shell, the
// probe that asks the image whether it can, and `sockets:`. What this file
// owes those two is the boundary itself — the repo `:ro` for a PID that
// denies Edit/Write, the worktree's git common dir so L3's hooks are
// visible, the refusals log so the inner wall writes to the host's audit
// trail, and the socket a PID declared.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	// DefaultEngine is Docker because it is what the spike measured on
	// (rangerhq-89a); OrbStack answers to the same CLI, so it needs no entry
	// of its own — installing it *is* the swap.
	DefaultEngine = "docker"

	// DefaultCageImage is what `posse cage build` tags and what the launch
	// looks for; config `cage_image:` overrides both.
	DefaultCageImage = "posse-cage:latest"

	// CageBdVersion pins bd for the image. 0.49.1 is the operator's pin on
	// this machine: bd 1.2.x is the Dolt migration and cannot read a 0.49
	// database, so a caged persona built against it could not claim or close
	// anything in the repo it was launched in.
	CageBdVersion = "v0.49.1"
	CageBdPackage = "github.com/steveyegge/beads/cmd/bd"
)

// What the container tier realizes *today*. ADR 0002 §3 makes the tiers
// cumulative in gates realized, and it also says how to behave while the
// tier is still arriving in pieces: "If the inner render cannot happen …
// the shell-verb denies are unrealized at that tier and the launch refuses
// like any other — the strongest cage is never allowed to be the one that
// silently loses `git push`." These two constants are that sentence in
// code. Each is flipped by the bead that earns it, in the same commit as
// the mechanism and its verification; parity.go reads them and nothing
// else does.
const (
	// ContainerInnerGates — rangerhq-6so: `posse gates wrap <persona>` renders
	// L1/L3 against the image's PATH and shell inside the container, and a
	// PID denying Edit/Write gets the repo mounted :ro. Flipped 2026-08-22
	// with the mechanism and its live verification (ADR 0002 verification 8
	// and 10). It is a necessary condition, not a sufficient one: parity
	// still asks the IMAGE whether it carries the posse that does the
	// rendering (CageInnerGatesReady), because an image that does not is a
	// tier that lost `git push`.
	ContainerInnerGates = true
	// ContainerEgress — rangerhq-9d0: the session joins a --internal network
	// whose only other member is a CONNECT proxy holding the PID's egress:
	// (egress.go). Flipped 2026-08-22 with the mechanism and its live
	// verification (ADR 0002 verification 9).
	ContainerEgress = true
)

// Engine is a container engine as a launch profile: one command template
// plus the spellings for the pieces the launcher computes (mounts, env
// forwards). Docker's flags are the defaults; an engine that spells them
// differently says so in its yaml rather than in this package.
type Engine struct {
	Name    string
	Command string // {net} {mounts} {env} {workdir} {image} {cmd} {name}
	Mount   string // per mount: {src} {dst}
	MountRO string // per read-only mount
	Env     string // per forwarded name: {var} — the NAME only, never a value
	EnvSet  string // per computed pair: {var} {val} — values the launch itself knows (the proxy url)
	Home    string // HOME inside the image, where the persona's cage home mounts
	Build   string // how `posse cage build` builds: {image} {context}
	Probe   string // how the launch asks whether the image exists: {image}
	// Inner runs ONE command in the image with nothing of the host mounted
	// and nothing of the session forwarded: {image} {cmd}. It exists for a
	// single question — can this image render the gates inside it
	// (rangerhq-6so)? — which parity must answer before the tier may claim
	// a shell-verb deny. Like probe:, an engine that cannot be asked
	// answers yes.
	Inner   string
	Builtin bool

	// The egress route (ADR 0002 §3 L4, rangerhq-9d0 — egress.go). Five
	// one-line templates rather than a hard-coded `docker network create`,
	// for the same reason `command:` is one: Apple `container` expresses an
	// internal-network-plus-proxy topology on vmnet and re-opens the
	// question (rangerhq-rli). An engine that leaves them empty cannot
	// realize `egress:` and the launch says so instead of guessing.
	//
	// `{net}` is the network's NAME in these five; in `command:` it is the
	// token that JOINS the agent to it, expanded from Net below — the agent
	// line has no other use for a network and these have no other use for a
	// flag.
	Net       string // {net} in command:, the join: `--network {net}`
	NetCreate string // the internal network — no default route, no external DNS
	NetJoin   string // the proxy's second network: the only way out of the cage
	NetRemove string
	ProxyUp   string // {net} {proxy} {host} {mounts} {image} {cmd}
	ProxyDown string // {proxy}
}

var builtinEngines = []Engine{{
	Name: "docker", Builtin: true,
	// --rm because the cage is per session, -i -t because the runtimes are
	// TUIs. No --name: killing the client leaves the container running, and
	// a fixed name would then make the next launch of that session die on
	// "name already in use". {name} is rendered for templates that want it.
	Command: `docker run --rm -i -t {net} {mounts} {env} -w {workdir} {image} {cmd}`,
	Mount:   `-v {src}:{dst}`,
	MountRO: `-v {src}:{dst}:ro`,
	Env:     `-e {var}`,
	EnvSet:  `-e {var}={val}`,
	Home:    "/root",
	Build:   `docker build -t {image} {context}`,
	Probe:   `docker image inspect {image}`,
	// No -i, no -t, no mounts, no env: this is asked from wherever posse
	// happens to be running, which is not always a terminal.
	Inner: `docker run --rm {image} {cmd}`,
	// The egress route. The proxy is detached and --rm (it is reaped by the
	// launcher's watcher when the cage dies, and removed either way), runs
	// the cage image's own node, and carries none of the session's
	// environment — no `{env}` on this line, deliberately: the process that
	// terminates the agent's TLS handshakes has no business holding the
	// operator's credential. `bridge` is docker's always-present ordinary
	// network, so the route out needs no second network of ours.
	Net:       `--network {net}`,
	NetCreate: `docker network create --internal {net}`,
	NetJoin:   `docker network connect bridge {proxy}`,
	NetRemove: `docker network rm {net}`,
	ProxyUp:   `docker run -d --rm --name {proxy} --hostname {host} --network {net} {mounts} {image} {cmd}`,
	ProxyDown: `docker rm -f {proxy}`,
}}

// CagesDir holds engine templates: RHQ_HOME/cages/<name>.yaml.
func (a *App) CagesDir() string { return filepath.Join(a.Home, "cages") }

// ResolveEngine: config `default_engine:` > docker. There is no per-PID
// engine key — which engine the host runs is the operator's decision, not
// the persona's; the PID says `cage: container` and stops there.
func (a *App) ResolveEngine() string { return a.CfgGet("default_engine", DefaultEngine) }

// CageImage is the image tag the launch runs and `posse cage build` writes.
func (a *App) CageImage() string { return a.CfgGet("cage_image", DefaultCageImage) }

// LoadEngine returns a built-in by name, else RHQ_HOME/cages/<name>.yaml.
// Only `command:` is required there; every other key falls back to docker's
// spelling, which is what a docker-compatible engine (OrbStack) wants.
func (a *App) LoadEngine(name string) (*Engine, error) {
	if name == "" {
		name = DefaultEngine
	}
	for i := range builtinEngines {
		if builtinEngines[i].Name == name {
			e := builtinEngines[i]
			return &e, nil
		}
	}
	p := filepath.Join(a.CagesDir(), name+".yaml")
	if _, err := os.Stat(p); err != nil {
		return nil, Die("unknown cage engine %q (built-in: docker; or %s)", name, AbbrevHome(p))
	}
	cmd := YamlGet(p, "command")
	if cmd == "" {
		return nil, Die("cage engine %s: %s has no command:", name, AbbrevHome(p))
	}
	e := builtinEngines[0] // docker's spellings as the defaults
	e.Name, e.Builtin, e.Command = name, false, cmd
	// …except `inner:`, which is not inherited for the same reason the
	// egress route below is not: `docker run --rm <image> <cmd>` is a
	// spelling, not a lingua franca, and an engine that expresses "run one
	// thing in this image" differently would answer this probe with the
	// engine's own error rather than the image's. Unset means "cannot be
	// asked", which CageInnerGatesReady reads as yes — probe:'s precedent.
	e.Inner = ""
	// …except the egress route. `-v` and `-e` are a lingua franca; `docker
	// network create --internal` is not, and an engine whose whole reason
	// to exist is that it expresses topology differently (Apple container
	// on vmnet) must not inherit a spelling that would launch a cage with
	// no boundary. Absent here means `egress:` is unrealizable on this
	// engine, which the launch says out loud (PlanEgress).
	e.Net, e.NetCreate, e.NetJoin, e.NetRemove, e.ProxyUp, e.ProxyDown = "", "", "", "", "", ""
	for _, kv := range []struct {
		key string
		dst *string
	}{
		{"mount", &e.Mount}, {"mount_ro", &e.MountRO}, {"env", &e.Env},
		{"env_set", &e.EnvSet},
		{"home", &e.Home}, {"build", &e.Build}, {"probe", &e.Probe},
		{"inner", &e.Inner},
		{"net", &e.Net}, {"net_create", &e.NetCreate}, {"net_join", &e.NetJoin},
		{"net_remove", &e.NetRemove}, {"proxy_up", &e.ProxyUp}, {"proxy_down", &e.ProxyDown},
	} {
		if v := YamlGet(p, kv.key); v != "" {
			*kv.dst = v
		}
	}
	return &e, nil
}

// Binary is the executable an engine template leads with — what has to be
// on PATH for this tier to exist at all.
func (e *Engine) Binary() string {
	if f := strings.Fields(e.Command); len(f) > 0 {
		return f[0]
	}
	return ""
}

// ContainerAvailable reports whether this host can provide `cage:
// container` — the resolved engine loads and its binary is on PATH. It
// does NOT ask whether the image is built: a missing image is a refusal
// with a build instruction (WrapInCage), not a degraded launch, because
// --allow-degraded past it would only produce a session that dies on
// startup.
func (a *App) ContainerAvailable() bool {
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		return false
	}
	_, err = exec.LookPath(e.Binary())
	return err == nil
}

// cageAvailable is AvailableCages plus the engine probe for container,
// which is per-RHQ_HOME (the engine is configurable) and so cannot live in
// a package-level map the way seatbelt's sandbox-exec probe does.
func (a *App) cageAvailable(cage string) bool {
	if cage == CageContainer {
		return a.ContainerAvailable()
	}
	return AvailableCages[cage]
}

// ─── what the cage gives the session ─────────────────────────────────────────

// CageMount is one bind mount. Paths are the SAME inside as outside: the
// launch renders the runtime's command from host paths ({file} is the PID
// under RHQ_HOME/agents, {memory} the persona's dir, {skills} a rendered
// tree under state/) and re-pointing every one of them at a container-local
// prefix would mean a second rendering path that can disagree with the
// first. VirtioFS maps ownership, so a file written inside lands owned by
// the operator outside (measured, rangerhq-89a).
type CageMount struct {
	Src, Dst string
	RO       bool
	Why      string // shown by `posse cage <persona>`
}

// CageDir is the persona's cage state: RHQ_HOME/state/cages/<persona>.
func (a *App) CageDir(persona string) string {
	return filepath.Join(a.StateDir, "cages", persona)
}

// CageHome is the HOME a caged runtime gets — a per-persona directory on
// the host, mounted at the image's HOME. It holds the seeded ~/.claude.json
// (SeedCageHome) and whatever state the runtime writes afterwards, so a
// caged persona is not back in the first-run wizard at every launch.
func (a *App) CageHome(persona string) string {
	return filepath.Join(a.CageDir(persona), "home")
}

// CageMounts is everything of the host the cage can see, and nothing else:
// the session dir, the persona's memory, the persona's PID (read-only — it
// is the prompt, not a workspace), the rendered skills tree when the PID
// binds skills, the persona's cage HOME, its refusals log, and whatever
// `sockets:` asked for.
//
// **The repo goes `:ro` for a PID that denies Edit/Write** — the mount
// boundary of ADR 0002 §4, and the successor of L2, which cannot wrap a
// container (sandbox-exec around `docker run` cages the client). It is the
// repo alone: `{memory}` and the cage HOME stay read-write, because a
// persona that may not edit the work still has to keep its own notes and
// the runtime still has to keep its own state.
//
// What it costs, measured rather than assumed (rangerhq-6so, Docker 29.0.1):
// a unix socket reached through a bind-mounted DIRECTORY is not connectable
// on VirtioFS — ENOTSUP, read-only or not — so `.beads/bd.sock` is not a
// route into the cage and `bd` falls back to direct storage. That works on a
// read-write repo mount and NOT on a `:ro` one, where SQLite cannot open the
// database at all: a persona denying Edit/Write could not claim, comment or
// close a bead inside this tier.
//
// **The one carve-out is `.beads`** (ADR 0002's amendment of 2026-08-22 via
// rangerhq-3nxk; ADR 0014 §4 answering the same question from the other
// side): the store is not the work, and a tier whose whole point is the
// personas who may not edit the work — the reviewer, the auditor — is not
// allowed to be the tier where they cannot report what they found. So the
// repo goes `:ro` and `{dir}/.beads` is mounted back over it read-write.
// Measured 2026-08-22 (Docker 29.0.1): a bind mount of a pre-existing
// DIRECTORY lands read-write over a `:ro` bind of its parent, later mount
// wins, and `touch` in the repo still fails. Pre-existing is the condition
// and it is why this is guarded by a Stat — a mount whose source does not
// exist is a mountpoint the engine has to CREATE on a read-only mount, which
// is the failure rangerhq-6so measured on `.beads/bd.sock`.
//
// What crosses the carve-out is the JSONL, not SQLite: the inner `bd`
// wrapper (cageinner.go) runs no-db, so this mount carries a file append and
// the host's daemon imports it. `.git` is the other carve-out ADR 0014 names
// and it is NOT here — it belongs with that ADR's `writable:` overlays, and
// L3's `pre-push` needs no write at all.
func (a *App) CageMounts(ag *AgentFile, e *Engine, dir string) []CageMount {
	ro := deniesFileWrite(ag.Deny)
	why := "the session's repo"
	if ro {
		why = "the session's repo, READ-ONLY — the mount boundary that realizes the Edit/Write denies (ADR 0002 §4)"
	}
	ms := []CageMount{
		{Src: dir, Dst: dir, RO: ro, Why: why},
		{Src: ag.MemoryDir, Dst: ag.MemoryDir, Why: "persona memory ({memory}, ORDERS.md)"},
		{Src: ag.Path, Dst: ag.Path, RO: true, Why: "the PID the runtime reads as its prompt ({file})"},
	}
	// The carve-out, AFTER the repo so the later mount wins. Only when the
	// repo is `:ro` (on a read-write repo there is nothing to carve out of)
	// and only when the directory is already there.
	if beads := filepath.Join(dir, ".beads"); ro {
		if st, err := os.Stat(beads); err == nil && st.IsDir() {
			ms = append(ms, CageMount{Src: beads, Dst: beads, Why: "the bead store, READ-WRITE over the :ro repo — the carve-out that keeps claim/comment/close at this tier (ADR 0002 amendment, rangerhq-3nxk)"})
		}
	}
	// A worktree's `.git` is a FILE pointing at the main repo's common dir,
	// which is somewhere else on the host — so `.git/hooks/pre-push`, the L3
	// half of this tier, is not on the repo mount at all unless that dir is
	// mounted too. Same path in and out, so the pointer file resolves inside
	// to the thing it names outside; same mode as the repo, so the boundary
	// says one thing rather than two.
	if common := gitCommonDirOutside(dir); common != "" {
		ms = append(ms, CageMount{Src: common, Dst: common, RO: ro, Why: "the worktree's git common dir — .git here is a file pointing at it, and L3's hooks live there"})
	}
	if len(ag.Skills) > 0 {
		ms = append(ms, CageMount{Src: ag.SkillsStateDir, Dst: ag.SkillsStateDir, RO: true, Why: "bound skills (ADR 0007)"})
	}
	ms = append(ms, CageMount{Src: a.CageHome(ag.Name), Dst: e.Home, Why: "the caged runtime's HOME (seeded)"})
	// The one file of the gates that must outlive the container. The rest of
	// gates/<persona>/ is rendered INSIDE, at CageGatesDir, against the
	// image's own PATH and shell (cageinner.go); the audit trail is the
	// host's, so an inner refusal reads next to an L1 refusal on the host
	// and next to the egress proxy's 403s in the same log.
	ms = append(ms, CageMount{
		Src: a.RefusalsLogPath(ag.Name),
		Dst: filepath.Join(CageGatesDir(ag.Name), "refusals.log"),
		Why: "gates/" + ag.Name + "/refusals.log — the inner L1/L3 append to the host's audit trail",
	})
	return append(ms, a.CageSocketMounts(ag)...)
}

// cageEnvName is the shape a name must have to be forwarded: an env var
// name, so nothing that needs quoting can reach the typed line.
var cageEnvName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// CageEnvNames is what crosses the boundary — NAMES ONLY. The engine
// forwards `-e NAME`, which takes the value from the pane's own
// environment, so a credential never appears on the typed line (which
// lands in the pane's scrollback and in herdr's logs). The persona's
// identity and tool lists ride in because the tools inside read them
// (BD_ACTOR above all, and RHQ_TOOLS_DENY, which is what the pre-push hook
// on the repo mount reads); the session's env sets ride in because that is
// what the PID's `envs:` promised the persona — including the operator's
// container credential (rangerhq-kiz).
func CageEnvNames(vars []EnvVar) []string {
	names := []string{
		"RHQ_HOME", "BD_ACTOR", EnvPersona, "RHQ_PERSONA_DIR", "RHQ_RUNTIME", "RHQ_TIER",
		"RHQ_CAGE", "RHQ_GATES_DIR", "RHQ_SKILLS_DIR", "RHQ_SKILLS",
		"RHQ_TOOLS_ALLOW", "RHQ_TOOLS_DENY",
	}
	for _, v := range vars {
		if cageEnvName.MatchString(v.Key) {
			names = append(names, v.Key)
		}
	}
	return dedupeStrings(names)
}

// ─── the credential (ADR 0002 §4 precondition, rangerhq-kiz) ─────────────────

// cageCredential is the env name an authenticated caged session needs, per
// runtime. Operator decision of 2026-08-20 (rangerhq-kiz): claude uses
// CLAUDE_CODE_OAUTH_TOKEN, minted once by the operator's own hand with
// `claude setup-token` and stored in an env set. ANTHROPIC_API_KEY is
// metered spending and was rejected on the money line, so it is NOT
// accepted here — a persona is never the one who decides to spend.
// codex and grok keep plain auth.json files and their container credential
// is undecided; the same bead says to settle it when their lane reaches
// this tier, so until then a caged launch on them refuses with that reason
// rather than starting an unauthenticated session.
var cageCredential = map[string]string{"claude": "CLAUDE_CODE_OAUTH_TOKEN"}

// CageCredential is the env name rt needs at this tier ("" = undecided).
// A template-only runtime names its own with `cage_cred:` in its yaml.
func CageCredential(rt *Runtime) string {
	if rt.CageCred != "" {
		return rt.CageCred
	}
	return cageCredential[rt.Name]
}

// CheckCageCredential is the launch precondition: auth is not a gate, but
// without it the cage starts a session that cannot do anything, and the
// ADR says to refuse with the reason rather than spend the launch.
func CheckCageCredential(rt *Runtime, names []string) error {
	want := CageCredential(rt)
	if want == "" {
		return Die("cage container: no container credential is decided for runtime %s — %s and %s keep plain auth.json files and rangerhq-kiz left their container shape open; decide it (and set cage_cred: for a template-only runtime) before caging %s",
			rt.Name, "codex", "grok", rt.Name)
	}
	for _, n := range names {
		if n == want {
			return nil
		}
	}
	return Die("cage container: %s is not in this session's environment — a caged %s has no keychain and its ~/.claude/.credentials.json is a stale leftover (rangerhq-kiz). Mint it once with `claude setup-token`, put it in an env set (mode 600, never in the repo), and name that set in the PID's envs: or pass --env-file. ANTHROPIC_API_KEY is metered spending and was rejected as the container credential",
		want, rt.Name)
}

// ─── the runtime state a fresh container starts without ──────────────────────

// SeedCageHome materializes the persona's cage HOME and seeds the runtime
// state a container has none of. For claude that is ~/.claude.json: without
// it a fresh container opens the theme/onboarding wizard and treats the
// workdir as untrusted, and a dispatcher's text lands in a wizard nobody is
// watching — the same failure ADR 0002 already seeds codex's `trust_level`
// against, one runtime over.
//
// The file is merged, not overwritten: claude writes its own state there
// between launches and the seed is only the keys that decide the wizard.
// codex is seeded on the command line (CodexFleetFlags, which the container
// carries in unchanged) and grok needs nothing, so both are no-ops here.
func (a *App) SeedCageHome(ag *AgentFile, rt *Runtime, dir string) (string, error) {
	home := a.CageHome(ag.Name)
	if err := os.MkdirAll(home, 0o755); err != nil {
		return "", err
	}
	if rt.Name != "claude" {
		return home, nil
	}
	p := filepath.Join(home, ".claude.json")
	state := map[string]any{}
	if b, err := os.ReadFile(p); err == nil {
		if err := json.Unmarshal(b, &state); err != nil {
			state = map[string]any{} // claude rewrites it wholesale; a corrupt one is not worth keeping
		}
	}
	state["hasCompletedOnboarding"] = true
	state["theme"] = "dark"
	// A container's CLI is pinned by the image; an in-place self-update
	// would diverge the cage from what `posse cage build` last verified.
	state["autoUpdates"] = false
	// Same keys the host launch seeds into the operator's config
	// (trust.go), because it is the same dialog on the same build — the
	// only difference is which HOME's file it lands in.
	claudeSeedProject(state, dir)
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}
	return home, os.WriteFile(p, append(b, '\n'), 0o600)
}

// ─── rendering the launch ────────────────────────────────────────────────────

// CageRender is everything a launch computes that an engine template can
// name. One struct rather than a widening parameter list, because the same
// expander renders the agent's line and the egress route's five steps
// (egress.go) and they name overlapping subsets of it.
type CageRender struct {
	Name    string      // {name}
	Image   string      // {image}
	Workdir string      // {workdir}
	Net     string      // the session's internal network ("" = no egress route)
	Proxy   string      // {proxy}: the egress proxy's container name
	Host    string      // {host}: the proxy's hostname on that network
	Inner   []string    // {cmd}
	Mounts  []CageMount // {mounts}
	Env     []string    // {env}, first half: NAMES the engine forwards by value-from-the-pane
	EnvSet  []EnvVar    // {env}, second half: pairs the launch itself computed (the proxy url)
}

// RenderArgv turns the engine template into the argv the launcher execs.
// Argv and not a shell line, because the pane no longer runs a shell: it
// runs the argv0 launcher (cagelauncher.go, rangerhq-1k1), which execs
// this argv with argv[0] reset to the runtime's name so herdr sees
// `claude` through the boundary instead of `docker`. A launcher that
// handed the line to `sh -c` would be replaced by the engine on that
// shell's own exec and hand herdr `docker` right back.
//
// The template is therefore read as *tokens* — its own whitespace-
// separated fields. A single-valued placeholder substitutes into the
// token it sits in ({workdir}, {image}, {name}); {mounts} and {env}
// expand to as many tokens as they have values, each rendered from the
// engine's own per-item template; {cmd} is the inner argv. Nothing is
// shell-quoted anywhere on this path, so a mount path with a space in it
// stays exactly one argument instead of depending on quoting getting it
// right — but it also means an engine `command:` is an argv template and
// not a shell line: pipes, redirections and `&&` have no shell to read
// them here.
func (e *Engine) RenderArgv(r CageRender) []string {
	// On the agent's line {net} is the JOIN — the engine's Net spelling
	// with the name in it — because that line has no other use for a
	// network. In the egress steps it is the name itself (stepArgv).
	var join []string
	if r.Net != "" && e.Net != "" {
		for _, f := range strings.Fields(e.Net) {
			join = append(join, strings.ReplaceAll(f, "{net}", r.Net))
		}
	}
	return e.expand(e.Command, r, join)
}

// stepArgv renders one of the egress route's one-line templates, where
// {net} is the network's own name. Empty template → empty argv, which
// PlanEgress reads as "this engine cannot express the route".
func (e *Engine) stepArgv(tmpl string, r CageRender) []string {
	if strings.TrimSpace(tmpl) == "" {
		return nil
	}
	return e.expand(tmpl, r, []string{r.Net})
}

// expand is the shared token walk. net is what a bare {net} token becomes.
func (e *Engine) expand(tmpl string, r CageRender, net []string) []string {
	var out []string
	for _, tok := range strings.Fields(tmpl) {
		switch tok {
		case "{net}":
			out = append(out, net...)
		case "{mounts}":
			for _, m := range r.Mounts {
				t := e.Mount
				if m.RO {
					t = e.MountRO
				}
				for _, f := range strings.Fields(t) {
					f = strings.ReplaceAll(f, "{src}", m.Src)
					out = append(out, strings.ReplaceAll(f, "{dst}", m.Dst))
				}
			}
		case "{env}":
			for _, n := range r.Env {
				for _, f := range strings.Fields(e.Env) {
					out = append(out, strings.ReplaceAll(f, "{var}", n))
				}
			}
			// A value the launch computed, not one it read from the pane:
			// the proxy url has no existence outside this launch, so there
			// is nothing to forward it *from*. Nothing secret is ever
			// rendered this way — that is what the NAMES above are for.
			for _, v := range r.EnvSet {
				for _, f := range strings.Fields(e.EnvSet) {
					f = strings.ReplaceAll(f, "{var}", v.Key)
					out = append(out, strings.ReplaceAll(f, "{val}", v.Value))
				}
			}
		case "{cmd}":
			out = append(out, r.Inner...)
		default:
			tok = strings.ReplaceAll(tok, "{name}", r.Name)
			tok = strings.ReplaceAll(tok, "{workdir}", r.Workdir)
			tok = strings.ReplaceAll(tok, "{proxy}", r.Proxy)
			tok = strings.ReplaceAll(tok, "{host}", r.Host)
			out = append(out, strings.ReplaceAll(tok, "{image}", r.Image))
		}
	}
	return out
}

// CageLine is an argv as a human reads it — `posse cage <persona>` prints
// it and the launch plan carries it as a note. Nothing executes this
// string; it exists so the operator can see (and paste) what the launcher
// is about to exec.
func CageLine(argv []string) string {
	var q []string
	for _, a := range argv {
		if a == "" || strings.ContainsAny(a, " \t'\"$&|;<>()*?[]{}#\\") {
			a = shellQuote(a)
		}
		q = append(q, a)
	}
	return strings.Join(q, " ")
}

// CageImageBuilt reports whether the engine can see the image. An engine
// with no probe: line answers yes — it has no way to be asked, and a
// launch that fails at the engine is a better error than a refusal this
// package invented.
func (a *App) CageImageBuilt(e *Engine, image string) bool {
	if e.Probe == "" {
		return true
	}
	f := strings.Fields(strings.ReplaceAll(e.Probe, "{image}", image))
	if len(f) == 0 {
		return true
	}
	return exec.Command(f[0], f[1:]...).Run() == nil
}

// WrapInCage renders the container launch and returns the line the pane
// runs: the argv0 launcher, and the file holding the argv it execs.
//
// Both halves are here for the same reason. The *launcher* (cagelauncher.go,
// rangerhq-1k1) is what herdr identifies the session by — it execs the
// engine with argv[0] set to the runtime's name, so `agent list` sees
// `claude` where a bare `docker run` reads as `agent_not_found` and no
// dispatch can target the session. The *file* is because an engine argv
// carries every mount and every forwarded name: rendered as a line it runs
// past 1.5KB, and a command that long does not survive being typed into a
// freshly created workspace whose shell is still starting (measured: an
// identical 1500-byte line is lost on a new pane and runs on the same pane
// a second later — rangerhq-ybec, which is the general fix). Like
// gates/<persona>/ and seatbelt.sb, both are rendered fresh from the PID at
// every launch and nothing hand-edited there survives; the argv file is per
// *session*, because two beads put the same persona in two containers with
// different mounts.
//
// The inner command keeps its shell — `sh -c` *inside* the container. It is
// a shell line ({file} expands through `$(cat …)`), and the mounts are
// same-path in and out, so the container's own sh reads exactly what the
// host's would have; `exec` in front of it leaves the runtime as the
// container's PID 1 rather than a child of a shell that outlives nothing.
// promptFile is the ADR 0013 §2 argv work prompt ("" = none). The inner
// `$(cat …)` is expanded by the container's own shell, so the file has to
// be mounted at the path the line names — the same same-path trick the PID
// mount uses, and without it a caged argv launch would hand the runtime an
// empty first turn.
func (a *App) WrapInCage(ag *AgentFile, rt *Runtime, session, dir, inner string, env []string, promptFile string) (string, error) {
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		return "", err
	}
	engine, err := exec.LookPath(e.Binary())
	if err != nil {
		return "", Die("cage container: engine %s needs %q on PATH", e.Name, e.Binary())
	}
	if err := CheckCageCredential(rt, env); err != nil {
		return "", err
	}
	if err := CheckSockets(ag); err != nil {
		return "", err
	}
	// The audit trail the inner gates mount and append to. Created here and
	// not best-effort: a bind mount of a file that is not there makes a
	// DIRECTORY, and the shims would then append refusals to a path that
	// silently eats them.
	if _, err := a.RefusalsLog(ag.Name); err != nil {
		return "", err
	}
	image := a.CageImage()
	if !a.CageImageBuilt(e, image) {
		return "", Die("cage container: image %s is not built — run `posse cage build` (it cross-builds Linux bd %s and posse into it; a caged persona cannot claim or close a bead without them)", image, CageBdVersion)
	}
	if _, err := a.SeedCageHome(ag, rt, dir); err != nil {
		return "", err
	}
	// The egress route (rangerhq-9d0). Unconditional at this tier, not
	// conditional on the PID having typed `egress:`: ADR 0002 §3 describes
	// L4 as a container joined to a --internal network whose only other
	// member is the proxy, and a cage whose network is open *by omission*
	// is the politeness this tier exists to stop being. A PID that names no
	// hosts still gets the runtime's own — enough to do the persona's work
	// and nothing else — and `posse cage <persona>` prints the effective list.
	hosts, bad := EgressHosts(ag, rt)
	if len(bad) > 0 {
		return "", Die("cage container: egress: %s is not a host — write a hostname (api.anthropic.com) or a subtree (*.githubusercontent.com); the proxy matches the CONNECT authority and never sees a path", strings.Join(bad, ", "))
	}
	eg, err := a.PlanEgress(e, ag, rt, session, image, hosts)
	if err != nil {
		return "", err
	}
	net := ""
	if eg != nil {
		net = eg.Net
	}
	// L1/L3 inside the cage (rangerhq-6so). The runtime's line keeps its
	// shell — `sh -c` *inside* the container, where the same-path mounts make
	// `$(cat {file})` read what the host's shell would have — and the image's
	// own posse goes in front of it, rendering gates/<persona>/ against the
	// image's PATH and shell and exec'ing the runtime behind the same typed
	// prefix the host types. Conditional on the image being able to answer
	// for itself: an image with no Linux posse would die on this word, and the
	// launch parity refused (or the operator waived) is still a launch.
	innerArgv := []string{"sh", "-c", "exec " + inner}
	if a.CageInnerGatesReady(e, image) {
		innerArgv = append(GatesWrapArgv(ag.Name, rt), innerArgv...)
	}
	mounts := a.CageMounts(ag, e, dir)
	if promptFile != "" {
		mounts = append(mounts, CageMount{Src: promptFile, Dst: promptFile, RO: true,
			Why: "the dispatched work prompt the launch line reads (ADR 0013 §2)"})
	}
	argv := e.RenderArgv(CageRender{
		Name: "posse-" + session, Image: image, Workdir: dir, Net: net,
		Inner:  innerArgv,
		Mounts: mounts,
		Env:    env, EnvSet: append(EgressProxyVars(), CageSocketVars(ag)...),
	})
	return a.WriteCageLaunch(ag.Name, session, rt, engine, argv, eg)
}

// ─── building the image ──────────────────────────────────────────────────────

// CageDockerfile is where the image definition lives in the posse source
// tree; `posse cage build` needs that tree because this file is in it. The
// module path is fetchable now (github.com/ranger360ai/posse, rangerhq-7xpn),
// so the Linux posse the image carries could come from `go install
// github.com/ranger360ai/posse/cmd/posse@latest` instead of a cross-build —
// but `go install` fetches a binary, not repo files, so the checkout is
// still what supplies the Dockerfile.
const CageDockerfile = "etc/cage/Dockerfile"

// BuildCageImage stages a build context — the Dockerfile plus Linux builds
// of posse and bd — and hands it to the engine. src is a posse source
// checkout; runtimes is the npm package list the image installs (empty =
// the Dockerfile's default, claude alone).
func (a *App) BuildCageImage(src, runtimes string, out *os.File) error {
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		return err
	}
	if _, err := exec.LookPath(e.Binary()); err != nil {
		return Die("cage engine %s needs %q on PATH", e.Name, e.Binary())
	}
	dockerfile := filepath.Join(src, CageDockerfile)
	if _, err := os.Stat(dockerfile); err != nil {
		return Die("%s not found — `posse cage build [dir]` needs a posse source checkout (the image carries a Linux posse built from it)", AbbrevHome(dockerfile))
	}
	stage := filepath.Join(a.StateDir, "cages", "build")
	bin := filepath.Join(stage, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	arch := runtime.GOARCH
	fmt.Fprintf(out, "cross-building linux/%s posse from %s\n", arch, AbbrevHome(src))
	build := exec.Command("go", "build", "-o", filepath.Join(bin, "posse"), "./cmd/posse")
	build.Dir, build.Stdout, build.Stderr = src, out, out
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch, "GOBIN=")
	if err := build.Run(); err != nil {
		return Die("building linux posse: %v", err)
	}
	// `go install pkg@version` refuses a cross-compile with GOBIN set, so
	// the binary is fetched into a staged GOPATH and copied out of
	// bin/<goos>_<goarch>/ (verified on go1.26).
	fmt.Fprintf(out, "cross-building linux/%s bd %s\n", arch, CageBdVersion)
	gopath := filepath.Join(stage, "gopath")
	inst := exec.Command("go", "install", CageBdPackage+"@"+CageBdVersion)
	inst.Dir, inst.Stdout, inst.Stderr = stage, out, out
	inst.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch, "GOPATH="+gopath, "GOBIN=")
	// The staged GOPATH exists only to catch the cross-compiled binary;
	// point the module cache back at the machine's own so a build does not
	// leave a few hundred MB of duplicated modules under state/.
	if mc, err := exec.Command("go", "env", "GOMODCACHE").Output(); err == nil && len(strings.TrimSpace(string(mc))) > 0 {
		inst.Env = append(inst.Env, "GOMODCACHE="+strings.TrimSpace(string(mc)))
	}
	if err := inst.Run(); err != nil {
		return Die("building linux bd %s: %v", CageBdVersion, err)
	}
	b, err := os.ReadFile(filepath.Join(gopath, "bin", "linux_"+arch, "bd"))
	if err != nil {
		return Die("linux bd not where go install left it: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "bd"), b, 0o755); err != nil {
		return err
	}
	os.RemoveAll(gopath) // the staged GOPATH was only a landing spot for that binary
	df, err := os.ReadFile(dockerfile)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "Dockerfile"), df, 0o644); err != nil {
		return err
	}
	image := a.CageImage()
	cmd := strings.ReplaceAll(e.Build, "{image}", image)
	cmd = strings.ReplaceAll(cmd, "{context}", stage)
	f := strings.Fields(cmd)
	if runtimes != "" {
		f = append(f, "--build-arg", "RUNTIMES="+runtimes)
	}
	fmt.Fprintf(out, "%s\n", strings.Join(f, " "))
	eng := exec.Command(f[0], f[1:]...)
	eng.Stdout, eng.Stderr = out, out
	if err := eng.Run(); err != nil {
		return Die("%s: %v", e.Name, err)
	}
	fmt.Fprintf(out, "built %s (engine %s, context %s)\n", image, e.Name, AbbrevHome(stage))
	return nil
}

// ListEngines returns built-in engine names plus any cages/*.yaml.
func (a *App) ListEngines() []string {
	var out []string
	for _, e := range builtinEngines {
		out = append(out, e.Name)
	}
	ents, _ := os.ReadDir(a.CagesDir())
	for _, x := range ents {
		if !x.IsDir() && strings.HasSuffix(x.Name(), ".yaml") {
			out = append(out, strings.TrimSuffix(x.Name(), ".yaml"))
		}
	}
	return dedupeStrings(out)
}
