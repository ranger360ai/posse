package rhq

// Runtime preflight (ADR 0012 D4, rangerhq-tr8k): the checks that answer
// "can a session on this runtime work at all", separately from "does the
// PID's wall hold here" (parity.go) and "is the tier's model available"
// (modelavail.go).
//
// Three of them, and each one existed as tribal knowledge before it existed
// as code:
//
//   - env_required: the variable NAMES the CLI cannot run without. The
//     Bedrock/Vertex shape — claude installed, on PATH, correctly declared,
//     and every launch a dead pane because AWS_REGION was not in the
//     session's env. It was expressible only as "put it in the PID's envs:
//     list and remember why".
//   - exe on PATH: the launch line's argv0. A miss here is a pane that
//     prints "command not found" and sits at a shell, which herdr reads as
//     a shell, not as a failure.
//   - herdr detection: the single hardest third-party blocker. A runtime
//     herdr cannot name from argv0 is `agent_not_found` — every state is a
//     guess, and dispatch cannot address the session at all.
//
// NAMES ONLY, everywhere. Nothing here reads, prints or forwards the value
// of an environment variable: the whole surface is "this name is set" /
// "this name is not set", because these lines are read out loud in a
// terminal and pasted into beads.

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// detectionDoc is the authoring page a detection gap points at. Named here
// because the gap line is the only place an onboarder is told the doc
// exists, and a path that drifts is a dead end at the worst moment.
const detectionDoc = "agent-detection-manifest.md"

// MissingEnv returns the names from rt.EnvRequired that a session receiving
// vars would not get. It looks in the env sets the launch resolved first,
// then in the launcher's own environment — tmux hands the session what this
// process holds — and treats present-but-empty as missing, because an empty
// AWS_REGION is the same dead pane as an absent one.
func MissingEnv(rt *Runtime, vars []EnvVar) []string {
	if rt == nil || len(rt.EnvRequired) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, v := range vars {
		if v.Value != "" {
			set[v.Key] = true
		}
	}
	var missing []string
	for _, name := range rt.EnvRequired {
		if set[name] {
			continue
		}
		if v, ok := os.LookupEnv(name); ok && v != "" {
			continue
		}
		missing = append(missing, name)
	}
	return missing
}

// EnvRequiredError is the launch refusal. It names the runtime, the missing
// names and the two places a value can legitimately come from — and nothing
// else: a refusal that printed what it DID find would be printing an env
// set's contents into a terminal, which is the one thing `envs:` guarantees
// never happens (rangerhq-f2b).
func EnvRequiredError(rt *Runtime, missing []string) error {
	where := "runtimes/" + rt.Name + ".yaml"
	if rt.Builtin {
		where = "the built-in " + rt.Name + " runtime"
	}
	return Die("runtime %s requires %s in the session env, and %s not set: %s\n"+
		"  declared by %s (env_required:)\n"+
		"  supply it with an env set the persona receives (envs/<name>.env, `envs:` in the PID) or export it before launching\n"+
		"  posse never reads these values — only whether the names are set",
		rt.Name, plural(len(missing), "a variable", "variables"),
		plural(len(missing), "it is", "they are"), strings.Join(missing, " "), where)
}

// RuntimeGap is one preflight finding, reported BY NAME so an onboarder can
// work through them one at a time (ADR 0012 D4: "check on a fake runtime
// reports each gap by name").
//
// Blocking is the difference between "this runtime cannot take work" and
// "this runtime takes work in a named degraded state" — the same split the
// contract grid draws, and the reason the grid is printed whole even when a
// gap is found: a runtime with three gaps still has six stages an onboarder
// has to fill.
type RuntimeGap struct {
	Name     string // what to say when you say "gap": exe, detection, yaml, env_required, interstitial
	Line     string // the gap and what it costs, one sentence
	Blocking bool
}

// RuntimeGaps is the preflight for one runtime. h supplies herdr's kind
// list; a herdr that cannot be read yields the UNKNOWN reading (a
// non-blocking gap), never a wrong "no" — the same rule KnownAgentKinds
// applies to parsing --help.
func (a *App) RuntimeGaps(rt *Runtime, h Herdr) []RuntimeGap {
	var gaps []RuntimeGap
	add := func(name, line string, blocking bool) {
		gaps = append(gaps, RuntimeGap{Name: name, Line: line, Blocking: blocking})
	}

	// exe — the launch line's argv0, resolved the way a shell would.
	exe := rt.Exe()
	switch {
	case exe == "":
		add("exe", "command: renders no executable at all — there is nothing to launch", true)
	default:
		if _, err := exec.LookPath(exe); err != nil {
			add("exe", fmt.Sprintf("%q is not on PATH — a launch opens a pane that prints \"command not found\" and sits at a shell, which herdr reads as a shell rather than as a failure", exe), true)
		}
	}

	// detection — the hardest one, and the one that is partly not posse's
	// (ADR 0012 D4 §6). A runtime herdr cannot name is undispatchable: every
	// state is default_known_agent_idle_fallback, so `working` and every
	// settled state are guesses.
	//
	// Asked through AgentManifest, not through the compiled kind list: an
	// `aliases = [...]` entry on another agent's manifest also resolves, and
	// on herdr 0.8.0 it is the only route a CLI herdr was not built with has
	// to detection at all. The kind list is the fallback when the probe
	// cannot be run.
	switch _, known, ok := h.AgentManifest(exe); {
	case ok && known:
	case ok:
		add("detection", fmt.Sprintf("herdr has no detection manifest for argv0 %q — a dispatched session is agent_not_found, so it cannot be addressed at all. Author one: docs/runbooks/%s", exe, detectionDoc), true)
	default:
		kinds := h.KnownAgentKinds()
		switch {
		case kinds == nil:
			add("detection", "herdr could not be asked (not on PATH, or its output changed shape) — whether it recognizes "+exe+" is UNKNOWN here, not no", false)
		case !containsString(kinds, exe):
			add("detection", fmt.Sprintf("herdr does not recognize argv0 %q — a dispatched session is agent_not_found, so it cannot be addressed at all. Author a manifest: docs/runbooks/%s", exe, detectionDoc), true)
		}
	}

	// yaml — keys nothing reads. A launch WARNS and proceeds on these
	// (warnUnknownRuntimeKeys, and deliberately so: the file is the
	// operator's own config root). `runtime check` is the stricter surface,
	// because it is the one an onboarder runs to ask "is this profile
	// right", and `skils_flag:` is exactly the answer they came for.
	for _, k := range unknownRuntimeKeys(rt) {
		add("yaml", fmt.Sprintf("%s declares %s: — nothing reads it, so the declaration never arrives (a launch warns and proceeds; this check does not)", AbbrevHome(rt.Path), k), true)
	}

	// env_required — the names, and only the names.
	for _, name := range MissingEnv(rt, nil) {
		add("env_required", name+" is declared env_required: and is not set in this environment — a launch here refuses rather than opening a pane that cannot authenticate", true)
	}

	// interstitials — a declared first-run screen whose probe says the
	// operator has not silenced it yet. Non-blocking: the launcher does not
	// refuse on one today (ADR 0013 §2 says it should — ranger-base-a9y9 is
	// that change, held back because wiring it alters what the operator's
	// own codex launches do), and a probe that answers "unknown"
	// — which is every DECLARED interstitial, since posse cannot read an
	// unknown CLI's config format — is not a finding at all.
	for _, in := range rt.Interstitials {
		if in.Probe == nil || in.Seeded {
			continue
		}
		if ok, why := in.Probe(); !ok {
			add("interstitial", fmt.Sprintf("%s is NOT silenced — %s. %s", in.Key, why, in.Silence), false)
		}
	}
	return gaps
}

// unknownRuntimeKeys is warnUnknownRuntimeKeys' finding without its
// writer: the top-level keys in this runtime's yaml that nothing reads.
// Empty for a built-in, which has no yaml — LoadRuntime returns it before
// it stats one.
func unknownRuntimeKeys(rt *Runtime) []string {
	if rt.Path == "" {
		return nil
	}
	known := map[string]bool{}
	for _, k := range runtimeYamlKeys() {
		known[k] = true
	}
	var out []string
	for _, k := range YamlKeysWithPrefix(rt.Path, "") {
		if !known[k] && !strings.HasPrefix(k, interstitialPrefix) {
			out = append(out, k)
		}
	}
	return out
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
