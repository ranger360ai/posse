package rhq

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
	if got := p.Realized["skills: s1"]; !strings.Contains(got, AgentsSkillsPath) {
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
	a := checkApp(t)
	ag := yamlV2Agent(t, "deny: [Edit, Write]\n")

	self := writeRuntime(t, a, "selfbox", "command: selfbox --sys {file}\nself_sandbox: true\n")
	if !self.SelfSandbox {
		t.Fatal("self_sandbox: true must set SelfSandbox")
	}
	p := a.CheckParity(ag, self, CageSeatbelt, TierStrong)
	if !strings.Contains(strings.Join(p.Degraded, "\n"), "cage seatbelt cannot wrap selfbox") {
		t.Errorf("self_sandbox must degrade cage: seatbelt with the nesting reason: %+v", p.Degraded)
	}
	// Undeclared, the same yaml is seatbelt-wrappable — the nesting line is
	// absent, which is what makes the assertion above about the key.
	plain := writeRuntime(t, a, "plainbox", "command: plainbox --sys {file}\n")
	if q := a.CheckParity(ag, plain, CageSeatbelt, TierStrong); strings.Contains(strings.Join(q.Degraded, "\n"), "cannot wrap") {
		t.Errorf("without self_sandbox: there is no nesting refusal: %+v", q.Degraded)
	}
}

// project_config: SAFETY-RELEVANT. Undeclared, ProjectConfigTrust has
// nothing to look for, so a repo→box executable channel reads as a clean
// launch. Declared, the launch degrades unless the PID opts in.
func TestProjectConfigIsDeclarable(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "trustcli", "command: trustcli --sys {file}\nproject_config: .trustcli/config.toml\n")
	if rt.ProjectConfig != filepath.FromSlash(".trustcli/config.toml") {
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
		rt.ProjectConfig != filepath.FromSlash(".testcli/config.toml") {
		t.Fatalf("scratch profile did not load all five: %+v", rt)
	}

	// The launch line.
	ag := yamlV2Agent(t, "skills: [s1]\ndeny: [Edit, Write]\n")
	cmd := ag.RenderCommandFor(rt, "test", TierStandard)
	if !strings.Contains(cmd, "testcli -c model='sol' -a never") {
		t.Errorf("rendered line: %q", cmd)
	}

	// The parity lines, in the tier the self-sandbox declaration is about.
	p := a.CheckParity(ag, rt, CageSeatbelt, TierStrong)
	joined := strings.Join(p.Degraded, "\n")
	if !strings.Contains(joined, "cage seatbelt cannot wrap test") {
		t.Errorf("parity must refuse to nest the seatbelt: %s", joined)
	}
	if got := p.Realized["skills: s1"]; !strings.Contains(got, AgentsSkillsPath) {
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
