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
// ranger-base-g27p4, filed rather than taken here.
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
