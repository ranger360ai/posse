package posse

// A persona's launch line is RENDERED — never typed, and never replayed. The
// runtime's template supplies the line (ADR 0002 §1), the PID supplies its
// `command:`, `allow:`/`deny:`, skills and settings placeholders, and a set
// of launch guarantees are appended to whatever comes out of that:
// unattended mode, the credential-dir settings pin, grok's `--no-leader` and
// `--strict-mcp-config` (ADR 0090 D1), each through its own door in
// RenderCommandForModel. A raw command string carries none of them.
//
// So `--cmd` and `--agent` name two different launches, and posse can only
// open one. It opened the persona's: planLaunch assigns `cmd := o.Cmd` and
// the `o.Agent != ""` branch overwrites it with the render, the meta records
// no `cmd:` for a persona session, and nothing anywhere said so.

// CheckCmdAgent refuses `--cmd` typed beside `--agent`. It is the whole
// refusal, it is pure, and it is asked at the argv boundary
// (parseNewFlags) — where the operator can be told which of the two flags
// they just typed is the one that would have been thrown away.
//
// Refused rather than warned, and that is the bead's own finding rather
// than a preference (ranger-base-k2olp, from ranger-base-pepl9): the
// misuse that matters is an unattended VERIFY script. ranger-base-pepl9
// host-checked ADR 0090 D1's launch refusal as `posse new --agent <p>
// --cmd "<a line naming its own --mcp-config>"`, read back `created … not
// refused`, and reported a security regression over a line that never
// carried `--mcp-config` at all. A warning on stderr is read by whoever is
// watching, and the whole point of that script is that nobody was. The
// failure is safe — the PID's own line is what ran, walled — but the
// measurement was no measurement, and a refusal is the only outcome that
// tells the script's reader so.
//
// It is NOT asked in planLaunch, which is where CheckExactModel's twin
// lives, because the pair cannot reach planLaunch from anywhere else that
// means it. A relaunch recreates from the meta, and startPlanned records
// `cmd:` only for a session with no persona (HerdrMeta.Cmd), so
// RecreateOpts hands a persona session Cmd: "". Dispatch sets no Cmd. The
// one other producer is a RECIPE declaring `command:` beside `agent:`,
// which is this same silent discard one layer up and a refusal in
// different words — it names `command:` and `agent:`, not two flags, and
// it would refuse a recipe that launches cleanly today. That arm is
// ranger-base-g27p4, and CheckRecipeCmdAgent below is it: asked at
// LaunchRecipe, so between the two boundaries no NewSessionOpts carrying
// both ever reaches planLaunch, and planLaunch still asks neither.
func CheckCmdAgent(o NewSessionOpts) error {
	if o.Cmd == "" || o.Agent == "" {
		return nil
	}
	return Die("--cmd cannot be given with --agent %s: a persona's line is rendered from its PID and its runtime's template, "+
		"so the line you typed would be discarded in silence (ADR 0002 §1)\n"+
		"  to change what a persona runs, change the PID's command: or its runtime file\n"+
		"  to run your own line, launch it without --agent — it gets no PID, no gates and none of the rendered launch flags",
		o.Agent)
}

// CheckRecipeCmdAgent refuses a recipe declaring `command:` beside `agent:`
// — the same silent discard one layer up (ranger-base-g27p4, the arm the
// paragraph above filed). It is pure for the same reason, and it is a
// SEPARATE refusal rather than CheckCmdAgent asked over the recipe's
// NewSessionOpts: the reader is editing a YAML file, so the message has to
// name `command:` and `agent:` — two keys in a file it can point at — and
// prescribe the spelling that file already documents (`command: null`).
// `--cmd`/`--agent` would name two flags this operator never typed.
//
// It reads the LOADED values and not the keys' presence, which is what
// keeps the documented form legal: YamlGet reads `null`, `~` and an empty
// value as unset (yamlflat.go), so `command: null` beside `agent: <p>` —
// examples/recipes/codex-projA.yaml's own prescription, and the form
// ~/homes/ranger/recipes/codex-src.yaml already carries — is not the pair
// and launches as it always did.
//
// Asked from LaunchRecipe and NOT from LoadRecipe, which is the one choice
// here worth recording: `posse recipes` loads every recipe to print it and
// drops the ones that error (`if r, err := a.LoadRecipe(n); err == nil`),
// so a refusal at load would make a recipe carrying the pair VANISH from
// the listing — a silent discard of exactly the class this bead is about,
// traded for the one it fixes. LaunchRecipe is also where the discard
// actually happens, so that is where the operator is standing when they
// are told.
func CheckRecipeCmdAgent(r *Recipe) error {
	if r == nil || r.Command == "" || r.Agent == "" {
		return nil
	}
	return Die("recipe %s declares command: beside agent: %s — a persona's line is rendered from its PID and its runtime's "+
		"template, so the command: you declared (%s) would be discarded in silence (ADR 0002 §1)\n"+
		"  to launch the persona, write `command: null` — the runtime's template renders the line\n"+
		"  to run that line yourself, drop `agent:` — the session gets no PID, no gates and none of the rendered launch flags\n"+
		"  to change what the persona runs, change its PID's command: or its runtime file",
		r.Path, r.Agent, r.Command)
}
