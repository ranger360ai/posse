package posse

// runtimes/<name>.yaml v2 (ADR 0012 D4): the realizer-adjacent keys, tested
// at their CONSUMERS. A getter round-trip proves a parser; every one of
// these keys exists to change something a launch renders or a parity line
// claims, so each test walks to that surface — the rendered command line,
// the Parity matrix, the materialized skills tree.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A PID that binds nothing, for the render tests: {skills}/{allow}/{deny}
// stay empty and the model flag is the only thing moving.
func yamlV2Agent(t *testing.T, extra string) *AgentFile {
	t.Helper()
	return loadTestAgent(t, "---\nname: p\n"+extra+"---\nYou are p.\n")
}

// rangerhq-5p0d, the half this bead owns: model_flag: was always rendered as
// `f + " %s"`, so a glued dialect had no spelling at all and an instance on
// one had to hardcode the model in command: and give up per-tier mapping.
// The built-in codex runtime has carried "-c model=%s" in Go source since
// tiering landed — this makes the same form declarable.
//
// Checked through ModelText AND through the renderer: the ORDERS lesson is
// that a token which parses is not a token that renders.
func TestModelFlagTakesAPrintfForm(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	ag := yamlV2Agent(t, "")

	glued := writeRuntime(t, a, "glued", "command: eng {model} --sys {file}\nmodel_flag: -c model=%s\nmodel_standard: big\n")
	if got := glued.ModelText(TierStandard); got != "-c model='big'" {
		t.Errorf("glued model_flag: ModelText = %q, want -c model='big'", got)
	}
	if got := ag.RenderCommandFor(glued, "glued", TierStandard); !strings.Contains(got, "eng -c model='big' --sys") {
		t.Errorf("glued model_flag does not reach the launch line: %q", got)
	}

	// The bare form is compat and must not move: every yaml written against
	// the old rule renders exactly as it did.
	bare := writeRuntime(t, a, "bare", "command: eng {model} --sys {file}\nmodel_flag: --model\nmodel_standard: big\n")
	if got := bare.ModelText(TierStandard); got != "--model 'big'" {
		t.Errorf("bare model_flag: ModelText = %q, want --model 'big'", got)
	}
	if got := ag.RenderCommandFor(bare, "bare", TierStandard); !strings.Contains(got, "eng --model 'big' --sys") {
		t.Errorf("bare model_flag does not reach the launch line: %q", got)
	}

	// An unmapped tier still renders nothing, printf form or not: {model}
	// and the space before it both go. (fast falls back to standard, so the
	// unmapped case needs a profile that maps strong alone.)
	strongOnly := writeRuntime(t, a, "strongonly", "command: eng {model} --sys {file}\nmodel_flag: -c model=%s\nmodel_strong: big\n")
	if got := ag.RenderCommandFor(strongOnly, "strongonly", TierFast); !strings.Contains(got, "eng --sys") {
		t.Errorf("unmapped tier must render nothing: %q", got)
	}
}

// The same printf rule on skills_flag:, which was built from the identical
// f+" %s" construction and had the identical gap.
func TestSkillsFlagTakesAPrintfForm(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	ag := yamlV2Agent(t, "skills: [s1]\n")

	glued := writeRuntime(t, a, "glued", "command: eng {skills} --sys {file}\nskills_flag: --skills=%s\n")
	flag, ok := glued.SkillsText("/tmp/tree", ag.Skills)
	if !ok || flag != "--skills='/tmp/tree'" {
		t.Errorf("glued skills_flag: SkillsText = %q,%v", flag, ok)
	}
	bare := writeRuntime(t, a, "bare", "command: eng {skills} --sys {file}\nskills_flag: --plugin-dir\n")
	if flag, ok := bare.SkillsText("/tmp/tree", ag.Skills); !ok || flag != "--plugin-dir '/tmp/tree'" {
		t.Errorf("bare skills_flag: SkillsText = %q,%v", flag, ok)
	}
	// The renderer, not just the getter.
	ag.SkillsStateDir = "/tmp/tree"
	if got := ag.RenderCommandFor(glued, "glued", TierStrong); !strings.Contains(got, "eng --skills='/tmp/tree' --sys") {
		t.Errorf("glued skills_flag does not reach the launch line: %q", got)
	}
}

// skills_cwd: the codex/grok surface, unreachable from yaml until now — a
// template runtime was skills_flag: or no surface at all, so a CLI that
// discovers skills from its working directory could only be declared as one
// that binds nothing, which refuses every PID with skills:.
func TestSkillsCwdIsDeclarable(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	if err := os.MkdirAll(filepath.Join(a.Home, "skills", "s1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Home, "skills", "s1", "SKILL.md"), []byte("---\nname: s1\ndescription: d\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := writeRuntime(t, a, "cwdcli", "command: cwdcli {skills} --sys {file}\nskills_cwd: true\n")
	if !rt.SkillsCwd {
		t.Fatal("skills_cwd: true must set SkillsCwd")
	}
	ag := yamlV2Agent(t, "skills: [s1]\n")

	// The binding is realizable and {skills} renders NOTHING: the links are
	// the realization.
	if !rt.RealizesSkills(ag.Skills) {
		t.Error("skills_cwd runtime must realize a skills binding")
	}
	if got := ag.RenderCommandFor(rt, "cwdcli", TierStrong); !strings.Contains(got, "cwdcli --sys") {
		t.Errorf("{skills} must render nothing on a skills_cwd runtime: %q", got)
	}

	// The consumer: RenderSkillsFor materializes the session-dir tree.
	dir := t.TempDir()
	if _, err := a.RenderSkillsFor(ag, rt, dir); err != nil {
		t.Fatalf("RenderSkillsFor: %v", err)
	}
	if _, err := os.Stat(filepath.Join(AgentsSkillsDir(dir), "s1")); err != nil {
		t.Errorf("skills_cwd must materialize %s/s1: %v", AgentsSkillsPath, err)
	}

	// And the parity line says which surface it is, rather than degrading.
	p := a.CheckParity(ag, rt, CageShims, TierStrong)
	if got := p.Realized["skills: s1"].Detail; !strings.Contains(got, AgentsSkillsPath) {
		t.Errorf("parity must name the cwd surface: %q (degraded: %v)", got, p.Degraded)
	}
	// Without the key the same yaml has no surface at all, which is the
	// before-picture: declared-means-required degrades the launch.
	none := writeRuntime(t, a, "nosurface", "command: nosurface --sys {file}\n")
	if p := a.CheckParity(ag, none, CageShims, TierStrong); len(p.Degraded) == 0 ||
		!strings.Contains(strings.Join(p.Degraded, "\n"), "no per-session skill surface") {
		t.Errorf("a runtime with neither key must degrade a skills: PID: %+v", p)
	}
}

// self_sandbox: macOS refuses to nest seatbelts, so a self-sandboxing yaml
// runtime was broken in a way nothing could say: the launch wrapped it and
// the matrix claimed an L2 that could not exist.
func TestSelfSandboxIsDeclarable(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	ag := yamlV2Agent(t, "deny: [Edit, Write]\n")

	self := writeRuntime(t, a, "selfbox", "command: selfbox --sys {file}\nself_sandbox: true\n")
	if !self.SelfSandbox {
		t.Fatal("self_sandbox: true must set SelfSandbox")
	}
	// The nesting fact is a DECLARED DIFFERENCE (ADR 0017 §2), not a
	// degradation — but this generic yaml runtime has no Realize/Enforced of
	// its own, so it does not actually deliver the Edit/Write wall it
	// cannot get from our seatbelt either: those stay genuinely Degraded
	// (ranger-base-d17a).
	p := a.CheckParity(ag, self, CageSeatbelt, TierStrong)
	if !strings.Contains(strings.Join(p.DeclaredDifference, "\n"), "cage seatbelt cannot wrap selfbox") {
		t.Errorf("self_sandbox must declare the nesting difference: %+v", p.DeclaredDifference)
	}
	if len(p.Degraded) != 2 || !strings.Contains(strings.Join(p.Degraded, "\n"), "Edit — needs cage: seatbelt") {
		t.Errorf("self_sandbox with no Enforced still leaves Edit/Write genuinely degraded: %+v", p.Degraded)
	}
	// Undeclared, the same yaml is seatbelt-wrappable — the nesting line is
	// absent, which is what makes the assertion above about the key.
	plain := writeRuntime(t, a, "plainbox", "command: plainbox --sys {file}\n")
	if q := a.CheckParity(ag, plain, CageSeatbelt, TierStrong); strings.Contains(strings.Join(q.Degraded, "\n"), "cannot wrap") || strings.Contains(strings.Join(q.DeclaredDifference, "\n"), "cannot wrap") {
		t.Errorf("without self_sandbox: there is no nesting refusal: %+v / %+v", q.Degraded, q.DeclaredDifference)
	}
}

// project_config: SAFETY-RELEVANT. Undeclared, ProjectConfigTrust has
// nothing to look for, so a repo→box executable channel reads as a clean
// launch. Declared, the launch degrades unless the PID opts in.
func TestProjectConfigIsDeclarable(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := writeRuntime(t, a, "trustcli", "command: trustcli --sys {file}\nproject_config: .trustcli/config.toml\n")
	if len(rt.ProjectConfig) != 1 || rt.ProjectConfig[0] != filepath.FromSlash(".trustcli/config.toml") {
		t.Fatalf("project_config: %q", rt.ProjectConfig)
	}
	dir := t.TempDir()
	ag := yamlV2Agent(t, "")

	// No such file in the session dir: nothing to say.
	if why := ProjectConfigTrust(rt, ag, dir); why != "" {
		t.Errorf("absent project config must be silent: %q", why)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".trustcli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".trustcli", "config.toml"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	why := ProjectConfigTrust(rt, ag, dir)
	if !strings.Contains(why, "trustcli reads") || !strings.Contains(why, "trust_project_config: true") {
		t.Errorf("present project config must name the channel and the opt-in: %q", why)
	}
	// The consumer, not the helper: CheckParityIn is what a launch calls.
	if p := a.CheckParityIn(ag, rt, CageShims, TierStrong, dir); !strings.Contains(strings.Join(p.Degraded, "\n"), "trustcli reads") {
		t.Errorf("CheckParityIn must carry the trust degrade: %+v", p.Degraded)
	}
	// The PID's opt-in clears it.
	opted := yamlV2Agent(t, "trust_project_config: true\n")
	if why := ProjectConfigTrust(rt, opted, dir); why != "" {
		t.Errorf("trust_project_config: true must clear it: %q", why)
	}
	// An undeclared runtime in the same directory says nothing — which is
	// exactly the silence the key removes.
	plain := writeRuntime(t, a, "plaincli", "command: plaincli --sys {file}\n")
	if why := ProjectConfigTrust(plain, ag, dir); why != "" {
		t.Errorf("an undeclared runtime has no project-config surface: %q", why)
	}
}

// PRESENT-BUT-WRONG refuses, the prompt:/record: rule applied to every new
// key. A typo that reads as a declaration is the silence this contract
// exists to remove; here the cost of a silent read is a rendered launch
// line with %!d(string=…) in it, a skills binding that quietly binds
// nothing, or a trust check scoped outside the session dir.
func TestRuntimeYamlV2Refusals(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	for _, c := range []struct{ name, body, want string }{
		{"model_flag wrong verb", "command: e {model}\nmodel_flag: -c model=%d\n", "exactly one %s"},
		{"model_flag two verbs", "command: e {model}\nmodel_flag: %s=%s\n", "exactly one %s"},
		{"skills_flag wrong verb", "command: e {skills}\nskills_flag: --s=%v\n", "exactly one %s"},
		{"skills_cwd not a bool", "command: e\nskills_cwd: yes\n", "want true or false"},
		{"self_sandbox not a bool", "command: e\nself_sandbox: 1\n", "want true or false"},
		{"both skill surfaces", "command: e\nskills_cwd: true\nskills_flag: --p\n", "one skill surface"},
		{"project_config absolute", "command: e\nproject_config: /etc/passwd\n", "must be relative"},
		{"project_config escapes", "command: e\nproject_config: ../../etc/passwd\n", "no .. elements"},
		{"project_config_keys alone", "command: e\nproject_config_keys: [hooks]\n", "without project_config:"},
		{"project_config_keys empty", "command: e\nproject_config: .a/b.json\nproject_config_keys: []\n", "empty project_config_keys:"},
		{"unattended not a flag", "command: e\nunattended: yolo\n", "must begin with the flag itself"},
		{"unattended is a positional", "command: e\nunattended: run --approve\n", "must begin with the flag itself"},
		{"unattended shell punctuation", "command: e\nunattended: --approve; curl evil.example\n", "shell punctuation"},
		{"unattended command substitution", "command: e\nunattended: --approve $(id)\n", "shell punctuation"},
	} {
		if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "bad.yaml"), []byte(c.body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := a.LoadRuntime("bad")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one naming %q", c.name, err, c.want)
		}
	}
	// A literal percent is not a verb: `--model%%` is a flag, not a format.
	rt := writeRuntime(t, a, "pct", "command: e {model}\nmodel_flag: --pct%%\nmodel_strong: m\n")
	if got := rt.ModelText(TierStrong); got != "--pct% 'm'" {
		t.Errorf("an escaped percent must survive as a literal: %q", got)
	}
}

// An unknown key was dropped in silence until now, which is how a typo
// becomes a dead wall: the file looks like a declaration and nothing reads
// it. Warned, not refused — the file is the operator's own config root and
// a newer posse may know keys an older one does not.
func TestUnknownRuntimeKeysAreWarned(t *testing.T) {
	a := checkApp(t)
	var buf bytes.Buffer
	old := runtimeNoticeWriter
	runtimeNoticeWriter = &buf
	defer func() { runtimeNoticeWriter = old }()

	rt := writeRuntime(t, a, "typo", "command: typo --sys {file}\nskils_flag: --p\nslef_sandbox: true\n")
	got := buf.String()
	for _, want := range []string{"skils_flag:", "slef_sandbox:", "known keys:", "skills_cwd", "project_config"} {
		if !strings.Contains(got, want) {
			t.Errorf("unknown-key warning missing %q in:\n%s", want, got)
		}
	}
	// Warned, not refused, and the key really was dropped.
	if rt.SelfSandbox || rt.RealizesSkills([]string{"s1"}) {
		t.Error("an unknown key must still be dropped — the warning is the whole fix")
	}
	// Said once per (file, key set): LoadRuntime runs several times per
	// command and a notice that repeats is a notice that gets filtered out.
	buf.Reset()
	if _, err := a.LoadRuntime("typo"); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("the notice must not repeat for the same file and keys: %q", buf.String())
	}
	// And a clean file says nothing at all — the half that is not
	// observable without the writer being a var.
	buf.Reset()
	writeRuntime(t, a, "clean", "command: clean --sys {file}\nmodel_flag: -c model=%s\nmodel_fast: m\nskills_cwd: true\nself_sandbox: true\nproject_config: .clean/c.toml\ngate_shell: false\nprompt: argv\nrecord: untrusted\nnative_rules: [AGENTS.md]\negress: [example.com]\ncage_cred: TOK\nstartup_wait: 30s\n")
	if buf.Len() != 0 {
		t.Errorf("a yaml using only known keys must warn nothing: %q", buf.String())
	}
}

// The bead's done-when, in one place: a scratch profile declaring all five,
// read back through the surfaces an operator actually looks at.
func TestRuntimeYamlV2ScratchProfile(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := writeRuntime(t, a, "test", strings.Join([]string{
		"command: testcli {model} {skills} -a never --rules=\"$(cat {file})\"",
		"model_flag: -c model=%s",
		"model_standard: sol",
		"skills_cwd: true",
		"self_sandbox: true",
		"project_config: .testcli/config.toml",
		"",
	}, "\n"))

	if rt.ModelFlag != "-c model=%s" || !rt.SkillsCwd || !rt.SelfSandbox ||
		len(rt.ProjectConfig) != 1 || rt.ProjectConfig[0] != filepath.FromSlash(".testcli/config.toml") {
		t.Fatalf("scratch profile did not load all five: %+v", rt)
	}

	// The launch line.
	ag := yamlV2Agent(t, "skills: [s1]\ndeny: [Edit, Write]\n")
	cmd := ag.RenderCommandFor(rt, "test", TierStandard)
	if !strings.Contains(cmd, "testcli -c model='sol' -a never") {
		t.Errorf("rendered line: %q", cmd)
	}

	// The parity lines, in the tier the self-sandbox declaration is about.
	// The nesting refusal is a DECLARED DIFFERENCE (ADR 0017 §2), not a
	// degradation, on its own (ranger-base-d17a) — but this template
	// runtime makes no Enforced claim of its own, so Edit/Write stay
	// genuinely Degraded below.
	p := a.CheckParity(ag, rt, CageSeatbelt, TierStrong)
	if !strings.Contains(strings.Join(p.DeclaredDifference, "\n"), "cage seatbelt cannot wrap test") {
		t.Errorf("parity must declare the nesting difference: %s", strings.Join(p.DeclaredDifference, "\n"))
	}
	if got := p.Realized["skills: s1"].Detail; !strings.Contains(got, AgentsSkillsPath) {
		t.Errorf("parity must name the cwd skill surface: %q", got)
	}
	// Edit/Write are unrealized here and say so: a template runtime declaring
	// self_sandbox: makes no claim about what its own sandbox enforces — only
	// a built-in realizer reports Enforced rules. Honest, and the direction
	// that costs a refusal rather than a gate.
	if len(p.Unrealized) != 2 || !strings.Contains(p.Unrealized[0], "needs cage: seatbelt") {
		t.Errorf("self_sandbox: must not be read as an enforcement claim: %+v", p.Unrealized)
	}

	// And `posse runtime check test` names it accurately.
	var buf bytes.Buffer
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &buf)
	out := buf.String()
	for _, want := range []string{"standard=sol", "-c model=%s", "skills_cwd:", "self_sandbox:", "project_config:"} {
		if !strings.Contains(out, want) {
			t.Errorf("runtime check missing %q in:\n%s", want, out)
		}
	}
}

// unattended: the flag that makes this CLI approve a tool call with nobody
// watching. Built-ins carry theirs in Go; a yaml runtime could not say it
// had one at all, so `runtime check`'s launch row printed "NO unattended
// flag known" with no remedy and every dispatched session on that runtime
// could sit on an approval dialog forever (ranger-base-ncxa).
//
// Tested at the CONSUMER: the rendered launch line, which is the only place
// the flag does anything.
func TestUnattendedIsDeclarable(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	ag := yamlV2Agent(t, "")

	rt := writeRuntime(t, a, "uncli", "command: uncli --sys {file}\nunattended: --approve-all\n")
	if rt.Unattended != "--approve-all" {
		t.Fatalf("unattended: %q", rt.Unattended)
	}
	got := ag.RenderCommandFor(rt, "uncli", TierStrong)
	if !strings.HasSuffix(got, " --approve-all") {
		t.Errorf("unattended: must reach the launch line: %q", got)
	}

	// The before-picture, and the arm that would stay green on a getter-only
	// fix: the same yaml without the key renders a line with no flag on it.
	none := writeRuntime(t, a, "nocli", "command: nocli --sys {file}\n")
	if got := ag.RenderCommandFor(none, "nocli", TierStrong); strings.Contains(got, "--approve-all") {
		t.Errorf("an undeclared runtime must append nothing: %q", got)
	}

	// EnsureUnattended's key match, on a declared flag: a PID that spells the
	// flag itself with a DIFFERENT value keeps its own. An explicit spelling
	// in a hand-written command: beats an implicit one from the yaml, and it
	// is visible in `ps` where a silent override would not be.
	val := writeRuntime(t, a, "valcli", "command: valcli --sys {file}\nunattended: --approval auto\n")
	own := loadTestAgent(t, "---\nname: p\ncommand: valcli --approval never --sys {file}\n---\nYou are p.\n")
	line := own.RenderCommandFor(val, "valcli", TierStrong)
	if !strings.Contains(line, "--approval never") || strings.Contains(line, "--approval auto") {
		t.Errorf("a PID's own spelling of the flag must win: %q", line)
	}

	// And the grid stops saying there is no remedy.
	var b bytes.Buffer
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	if s := b.String(); !strings.Contains(s, "unattended flag --approve-all on the line") || strings.Contains(s, "NO unattended flag known") {
		t.Errorf("launch row must name the declared flag:\n%s", s)
	}
	b.Reset()
	a.RuntimeCheck(none, Herdr{Bin: "no-such-herdr-binary"}, &b)
	if !strings.Contains(b.String(), "NO unattended flag known") {
		t.Errorf("an undeclared runtime must still say so loudly:\n%s", b.String())
	}
}

// project_config_keys: the key-narrowing half of the trust check, Go-only
// until now — a yaml runtime whose session-dir config is keyed JSON had to
// take the whole-file predicate and degrade every launch that had the file
// at all, whatever it held (ranger-base-ncxa).
//
// This is the one declarable key that LOOSENS a safety check, so the arms
// here are the loosening (a non-matching body goes clean), the check that
// survives it (a matching body still degrades), and the floor (a body that
// cannot be classified still fails closed).
func TestProjectConfigKeysAreDeclarable(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	ag := yamlV2Agent(t, "")
	keyed := writeRuntime(t, a, "kcli", "command: kcli --sys {file}\nproject_config: .kcli/settings.json\nproject_config_keys: [hooks, mcpServers]\n")
	if len(keyed.ProjectConfigKeys) != 2 || keyed.ProjectConfigKeys[0] != "hooks" {
		t.Fatalf("project_config_keys: %q", keyed.ProjectConfigKeys)
	}
	// Same file, same directory, no keys declared: the conservative half.
	whole := writeRuntime(t, a, "wcli", "command: wcli --sys {file}\nproject_config: .kcli/settings.json\n")

	dir := t.TempDir()
	cfg := filepath.Join(dir, ".kcli", "settings.json")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	degraded := func(rt *Runtime) string {
		t.Helper()
		return strings.Join(a.CheckParityIn(ag, rt, CageShims, TierStrong, dir).Degraded, "\n")
	}

	// A body naming none of the declared keys: the narrowing is the whole
	// point, and the undeclared-keys runtime in the same dir is the control
	// that says the file really is there.
	write(`{"permissions":{"allow":[]}}`)
	if why := degraded(keyed); why != "" {
		t.Errorf("a body naming no declared key must go clean: %q", why)
	}
	if why := degraded(whole); !strings.Contains(why, "project config is present") {
		t.Errorf("without the keys the same file must degrade on presence alone: %q", why)
	}

	// A body naming one: the check that survives the loosening, and it names
	// which key it matched so the operator knows what to remove.
	write(`{"permissions":{},"hooks":{"PreToolUse":[]}}`)
	if why := degraded(keyed); !strings.Contains(why, "matched top-level project config keys: hooks") {
		t.Errorf("a matching key must still degrade, by name: %q", why)
	}

	// The floor: keys declared over something that is not a readable
	// top-level JSON object fail closed, they do not read as "no match".
	write("[mcp_servers.probe]\ncommand = \"/bin/sh\"\n")
	if why := degraded(keyed); !strings.Contains(why, "classification failed") {
		t.Errorf("an unclassifiable body must fail closed: %q", why)
	}

	// And the PID's opt-in still clears it, keyed or not.
	write(`{"hooks":{}}`)
	opted := yamlV2Agent(t, "trust_project_config: true\n")
	if why := ProjectConfigTrust(keyed, opted, dir); why != "" {
		t.Errorf("trust_project_config: true must clear the keyed finding too: %q", why)
	}
}
