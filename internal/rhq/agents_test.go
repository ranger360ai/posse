package rhq

// PID parsing and rendering (docs/adr/0001-persona-intent-documents.md):
// the list keys, the {allow}/{deny} placeholders, and the legacy agent
// shape staying byte-for-byte as it was.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadTestAgent(t *testing.T, content string) *AgentFile {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents")}
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "p.md"), []byte(content), 0o644)
	ag, err := a.LoadAgent("p")
	if err != nil {
		t.Fatal(err)
	}
	return ag
}

func TestPIDListsParsed(t *testing.T) {
	ag := loadTestAgent(t, `---
name: p
description: test
intents:
  - design
  - review-design
allow:
  - Bash(bd:*)
  - Edit
deny:
  - Bash(git push:*)
metrics: [closed-no-reopen, blocked-honestly]
---
You are p.
`)
	check := func(field string, got, want []string) {
		t.Helper()
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s: got %v, want %v", field, got, want)
		}
	}
	check("intents", ag.Intents, []string{"design", "review-design"})
	check("allow", ag.Allow, []string{"Bash(bd:*)", "Edit"})
	check("deny", ag.Deny, []string{"Bash(git push:*)"})
	check("metrics", ag.Metrics, []string{"closed-no-reopen", "blocked-honestly"})
}

func TestRenderAllowDeny(t *testing.T) {
	// Rules carry parens, colons, spaces, and globs — all must reach the
	// shell as single quoted words.
	ag := loadTestAgent(t, `---
name: p
command: run {file} {allow} {deny}
allow:
  - Bash(bd:*)
  - Bash(git commit: msg with spaces)
deny:
  - Bash(git push:*)
---
`)
	cmd := ag.RenderCommand()
	// deny: carries the option-blind spellings claude needs to refuse the
	// same argv the shim does (L0Spellings, rangerhq-3mc); allow: does not —
	// widening an allow list would grant more than the PID says.
	want := "--allowedTools 'Bash(bd:*)' 'Bash(git commit: msg with spaces)' " +
		"--disallowedTools 'Bash(git push:*)' 'Bash(git -* push)' 'Bash(git -* push *)'"
	if !strings.HasSuffix(cmd, want) {
		t.Errorf("bad tool rendering:\n got %q\nwant suffix %q", cmd, want)
	}
	if strings.Contains(cmd, "{allow}") || strings.Contains(cmd, "{deny}") {
		t.Errorf("unrendered placeholder remains: %q", cmd)
	}
}

func TestRenderEmptyListsVanish(t *testing.T) {
	// No command: → DefaultAgentCommand, whose {allow} {deny} must vanish
	// cleanly (no flags, no leftover placeholder, no stray spacing).
	ag := loadTestAgent(t, `---
name: p
---
You are p.
`)
	cmd := ag.RenderCommand()
	for _, bad := range []string{"{allow}", "{deny}", "--allowedTools", "--disallowedTools", "  "} {
		if strings.Contains(cmd, bad) {
			t.Errorf("rendered command contains %q: %q", bad, cmd)
		}
	}
	if strings.HasSuffix(cmd, " ") {
		t.Errorf("trailing space left by empty placeholders: %q", cmd)
	}
	if !strings.Contains(cmd, "--add-dir '"+ag.MemoryDir+"'") {
		t.Errorf("default command lost {memory}: %q", cmd)
	}
}

func TestLegacyAgentUnchanged(t *testing.T) {
	// A pre-PID file (no new keys, own command without the placeholders)
	// must render exactly as it always has — including the old default
	// command, which ends in a double quote (rangerhq-nvq) — except for the
	// unattended mode, which every claude launch now carries whoever wrote
	// the template (rangerhq-qs5r).
	ag := loadTestAgent(t, `---
name: p
description: legacy
command: claude --append-system-prompt "$(cat {file})"
---
You are p.
`)
	if len(ag.Intents)+len(ag.Allow)+len(ag.Deny)+len(ag.Metrics) != 0 {
		t.Errorf("legacy file grew lists: %+v", ag)
	}
	want := `claude --append-system-prompt "$(cat '` + ag.Path + `')" ` + ClaudeFleetFlags
	if got := ag.RenderCommand(); got != want {
		t.Errorf("legacy rendering changed:\n got %q\nwant %q", got, want)
	}
}

func TestFleetSettingsSurviveRendering(t *testing.T) {
	// The default command carries --settings with a JSON blob (rangerhq-4e5:
	// suppresses the auto-mode setup dialog in unattended sessions). The
	// flat-YAML reader and the placeholder renderer must pass it through
	// verbatim — quotes, colons and braces intact — both from the default
	// and from an explicit command: line in a persona file.
	want := "--settings '" + ClaudeFleetSettings + "'"
	def := loadTestAgent(t, "---\nname: p\n---\nYou are p.\n")
	if got := def.RenderCommand(); !strings.Contains(got, want) {
		t.Errorf("default command lost fleet settings:\n got %q\nwant substring %q", got, want)
	}
	explicit := loadTestAgent(t, "---\nname: p\ncommand: "+DefaultAgentCommand+"\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")
	got := explicit.RenderCommand()
	if !strings.Contains(got, want) {
		t.Errorf("explicit command lost fleet settings:\n got %q\nwant substring %q", got, want)
	}
	if !strings.Contains(got, "--disallowedTools 'Bash(git push:*)'") {
		t.Errorf("deny list not rendered after settings flag: %q", got)
	}
	if strings.Contains(got, "{allow}") || strings.Contains(got, "{deny}") {
		t.Errorf("unrendered placeholder remains: %q", got)
	}
}

func TestScaffoldAgentIsPID(t *testing.T) {
	// posse agent new must produce a PID (ADR 0001): every frontmatter key
	// present, lists empty, body headings in contract order, and the whole
	// thing loadable by LoadAgent without edits.
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents")}
	p, err := a.ScaffoldAgent("scout")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	front, body := agentFrontmatter(string(raw))
	for _, key := range []string{"name", "description", "runtime", "labels", "intents", "allow", "deny", "metrics"} {
		found := false
		for _, ln := range front {
			if strings.HasPrefix(ln, key+":") {
				found = true
			}
		}
		if !found {
			t.Errorf("frontmatter missing key %q", key)
		}
	}

	ag, err := a.LoadAgent("scout")
	if err != nil {
		t.Fatal(err)
	}
	if ag.Name != "scout" {
		t.Errorf("name: got %q", ag.Name)
	}
	if ag.Runtime != "claude" || ag.Command != "" {
		t.Errorf("runtime/command: got %q / %q (command: is the escape hatch, not scaffolded)", ag.Runtime, ag.Command)
	}
	if ag.Description == "" || strings.Contains(ag.Description, "#") {
		t.Errorf("description hint mangled: %q", ag.Description)
	}
	// Commented hints must not leak into the lists.
	for field, got := range map[string][]string{"labels": ag.Labels, "intents": ag.Intents, "allow": ag.Allow, "deny": ag.Deny, "metrics": ag.Metrics} {
		if len(got) != 0 {
			t.Errorf("%s: want empty, got %v", field, got)
		}
	}
	// A hint-free {allow}/{deny} render is a plain launch.
	if r := ag.RenderCommand(); strings.Contains(r, "{allow}") || strings.Contains(r, "{deny}") || strings.Contains(r, "--allowedTools") || strings.Contains(r, "--disallowedTools") {
		t.Errorf("render leaked placeholders or empty flags: %s", r)
	}

	// Body: identity line first, then the headings in contract order.
	// The scaffold's placeholders are the contract's shape, brand-free, and
	// they lint clean as written (ADR 0012 App.A 2).
	if !identityLineRe.MatchString(identityLine(body)) {
		t.Errorf("body must open with the identity line, got %q", identityLine(body))
	}
	pos := -1
	for _, h := range PIDHeadings {
		i := strings.Index(body, "\n"+h+"\n")
		if i < 0 {
			t.Errorf("body missing heading %q", h)
			continue
		}
		if i < pos {
			t.Errorf("heading %q out of contract order", h)
		}
		pos = i
	}
	if !strings.Contains(body, "| intent | mode | done when |") {
		t.Error("## Intents missing its table header")
	}
	if !strings.Contains(body, HardRiskLines) {
		t.Error("## Guardrails missing the verbatim hard risk lines")
	}
	if !strings.Contains(body, "Persona-specific:") {
		t.Error("## Guardrails missing the Persona-specific block")
	}
	if !strings.Contains(body, "$RHQ_PERSONA_DIR/ORDERS.md") {
		t.Error("## Memory missing the ORDERS.md instruction")
	}

	// Second call is idempotent: never overwrites an existing persona.
	os.WriteFile(p, []byte("custom"), 0o644)
	if _, err := a.ScaffoldAgent("scout"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "custom" {
		t.Error("ScaffoldAgent overwrote an existing agent file")
	}
}

func TestExampleAgentsArePIDs(t *testing.T) {
	// examples/agents/*.md are the reference PIDs (ADR 0001, rangerhq-cd3):
	// every one carries the full frontmatter, the four hard risk lines
	// verbatim, the headings in contract order, and a push deny.
	dir := filepath.Join("..", "..", "examples", "agents")
	a := &App{Home: t.TempDir(), AgentsDir: dir}
	names := a.ListAgents()
	if len(names) < 9 {
		t.Skipf("reference PIDs not present (%d found)", len(names))
	}
	for _, name := range names {
		ag, err := a.LoadAgent(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if ag.Name != name {
			t.Errorf("%s: name: %q", name, ag.Name)
		}
		if ag.Description == "" || len(ag.Labels) == 0 || len(ag.Intents) == 0 || len(ag.Metrics) == 0 {
			t.Errorf("%s: frontmatter incomplete: %+v", name, ag)
		}
		if ag.Runtime != "claude" || ag.Command != "" {
			t.Errorf("%s: examples say runtime: claude and carry no command: (got %q / %q)", name, ag.Runtime, ag.Command)
		}
		if !ValidTier(ag.Tier) {
			t.Errorf("%s: tier: %q (ADR 0003 Dial A: every example names one)", name, ag.Tier)
		}
		pushDenied := false
		for _, r := range ag.Deny {
			if r == "Bash(git push:*)" {
				pushDenied = true
			}
		}
		if !pushDenied {
			t.Errorf("%s: deny must include Bash(git push:*)", name)
		}
		if r := ag.RenderCommand(); !strings.Contains(r, "--disallowedTools") || !strings.Contains(r, "'Bash(git push:*)'") || strings.Contains(r, "{deny}") {
			t.Errorf("%s: render: %s", name, r)
		}
		body := ag.Body
		if !identityLineRe.MatchString(identityLine(body)) {
			t.Errorf("%s: body must open with the identity line, got %q", name, identityLine(body))
		}
		pos := -1
		for _, h := range PIDHeadings {
			i := strings.Index(body, "\n"+h+"\n")
			if i < 0 {
				t.Errorf("%s: missing heading %q", name, h)
				continue
			}
			if i < pos {
				t.Errorf("%s: heading %q out of contract order", name, h)
			}
			pos = i
		}
		if !strings.Contains(body, HardRiskLines) {
			t.Errorf("%s: hard risk lines not verbatim", name)
		}
		if !strings.Contains(body, "| intent | mode | done when |") {
			t.Errorf("%s: ## Intents lacks the table header", name)
		}
		if !strings.Contains(body, "$RHQ_PERSONA_DIR/ORDERS.md") {
			t.Errorf("%s: ## Memory lacks the ORDERS.md instruction", name)
		}
	}
	// business-manager is advisory by construction.
	bm, _ := a.LoadAgent("business-manager")
	if bm != nil && !strings.Contains(strings.Join(bm.Deny, "|"), "Edit|Write|Bash(git commit:*)") {
		t.Errorf("business-manager deny must cover Edit/Write/git commit: %v", bm.Deny)
	}
}

// ADR 0002 §1: runtimes are launch profiles; {allow}/{deny} render through
// each runtime's native realizer; a PID's command: is the template for its
// own runtime only.
func TestRuntimeRealizers(t *testing.T) {
	ag := loadTestAgent(t, "---\nname: p\nallow: [Bash(bd:*)]\ndeny:\n  - Edit\n  - Write\n  - Bash(git push:*)\n---\nYou are p.\n")
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	render := func(name string) string {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		return ag.RenderCommandFor(rt, "claude", TierStrong)
	}
	claude := render("claude")
	if !strings.Contains(claude, "--allowedTools 'Bash(bd:*)'") || !strings.Contains(claude, "--disallowedTools 'Edit' 'Write' 'Bash(git push:*)'") || !strings.HasPrefix(claude, "claude ") {
		t.Errorf("claude: %s", claude)
	}
	codex := render("codex")
	// read-only: no --add-dir (codex exits on it in that mode), the PID rides
	// as developer_instructions, and the unattended flags are present.
	if !strings.HasPrefix(codex, "codex -s read-only -a never ") || strings.Contains(codex, "--add-dir") ||
		strings.Contains(codex, "allowedTools") || !strings.Contains(codex, CodexFleetFlags) ||
		!strings.HasSuffix(codex, `-c developer_instructions="$(cat '`+ag.Path+`')"`) {
		t.Errorf("codex: %s", codex)
	}
	grok := render("grok")
	// --rules= (not --rules ): a PID starts with "---", which grok's arg
	// parser reads as a flag in the separated form and dies on (rangerhq-vjl).
	if !strings.HasPrefix(grok, "grok "+GrokFleetFlags+` --rules="$(cat '`) ||
		!strings.Contains(grok, "--allow 'Bash(bd:*)' --deny 'Edit' --deny 'Write' --deny 'Bash(git push:*)'") {
		t.Errorf("grok: %s", grok)
	}
	// codex without Edit+Write denied is workspace-write, still emitted.
	ag2 := loadTestAgent(t, "---\nname: p\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")
	rt, _ := a.LoadRuntime("codex")
	// workspace-write is the one mode where --add-dir is legal, so the memory
	// dir rides with the mode instead of with the template.
	if c := ag2.RenderCommandFor(rt, "claude", TierStrong); !strings.HasPrefix(c, "codex -s workspace-write --add-dir '"+ag2.MemoryDir+"' -a never") {
		t.Errorf("codex workspace-write: %s", c)
	}
	r := realizeCodex(nil, []string{"Edit", "Write"}, "/mem")
	if strings.Join(r.Realized, ",") != "Edit,Write" || r.Deny != "-s read-only" {
		t.Errorf("codex realized: %+v", r)
	}
	if w := realizeCodex(nil, nil, "/mem").Deny; w != "-s workspace-write --add-dir '/mem'" {
		t.Errorf("codex workspace-write realized: %q", w)
	}
	// Placeholders never leak on any runtime, even with empty lists.
	empty := loadTestAgent(t, "---\nname: p\n---\nYou are p.\n")
	for _, n := range []string{"claude", "codex", "grok"} {
		rt, _ := a.LoadRuntime(n)
		if c := empty.RenderCommandFor(rt, "claude", TierStrong); strings.Contains(c, "{allow}") || strings.Contains(c, "{deny}") {
			t.Errorf("%s leaks a placeholder: %s", n, c)
		}
	}
}

func TestRuntimeOverrideIgnoresPIDCommand(t *testing.T) {
	// A PID with its own claude-shaped command: uses it on claude only; an
	// override to codex takes codex's built-in template.
	ag := loadTestAgent(t, "---\nname: p\ncommand: mywrap --file {file} {allow} {deny}\ndeny: [Edit, Write]\n---\nYou are p.\n")
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	own := a.ResolveRuntime("", ag) // no runtime:, no config → claude
	if own != "claude" {
		t.Fatalf("own runtime %q", own)
	}
	c, _ := a.LoadRuntime("claude")
	if got := ag.RenderCommandFor(c, own, TierStrong); !strings.HasPrefix(got, "mywrap --file '") || !strings.Contains(got, "--disallowedTools 'Edit' 'Write'") {
		t.Errorf("own runtime must use the PID command: %s", got)
	}
	x, _ := a.LoadRuntime("codex")
	if got := ag.RenderCommandFor(x, own, TierStrong); !strings.HasPrefix(got, "codex -s read-only") {
		t.Errorf("override must use codex's template: %s", got)
	}
	// A PID that says runtime: codex uses codex's template by default and
	// its own command: only if it were codex-shaped (own == codex).
	ag2 := loadTestAgent(t, "---\nname: p\nruntime: codex\n---\nYou are p.\n")
	if own := a.ResolveRuntime("", ag2); own != "codex" {
		t.Errorf("PID runtime: not honoured: %q", own)
	}
	if got := ag2.RenderCommand(); !strings.HasPrefix(got, "codex -s workspace-write") {
		t.Errorf("RenderCommand on a codex PID: %s", got)
	}
	// Precedence: explicit > PID > config default_runtime > claude.
	os.WriteFile(a.ConfigPath, []byte("default_runtime: grok\n"), 0o644)
	if r := a.ResolveRuntime("", ag); r != "grok" {
		t.Errorf("config default_runtime: %q", r)
	}
	if r := a.ResolveRuntime("", ag2); r != "codex" {
		t.Errorf("PID over config: %q", r)
	}
	if r := a.ResolveRuntime("claude", ag2); r != "claude" {
		t.Errorf("explicit over PID: %q", r)
	}
}

func TestTemplateOnlyRuntime(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"), []byte("command: mycli --sys {file} --mem {memory} {allow} {deny}\n"), 0o644)
	rt, err := a.LoadRuntime("mycli")
	if err != nil {
		t.Fatal(err)
	}
	if rt.Builtin || rt.Realize != nil {
		t.Errorf("template-only runtime must have no realizer: %+v", rt)
	}
	ag := loadTestAgent(t, "---\nname: p\nallow: [Edit]\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")
	got := ag.RenderCommandFor(rt, "mycli", TierStrong)
	if !strings.HasPrefix(got, "mycli --sys '") || strings.Contains(got, "{allow}") || strings.Contains(got, "{deny}") || strings.Contains(got, "git push") {
		t.Errorf("template-only: gates must not render natively: %s", got)
	}
	if _, err := a.LoadRuntime("nope"); err == nil {
		t.Error("unknown runtime must error")
	}
	if names := a.ListRuntimes(); strings.Join(names, ",") != "claude,codex,grok,mycli" {
		t.Errorf("ListRuntimes: %v", names)
	}
}

// ADR 0003 §1–2: tiers map to models per runtime; {model} renders the
// runtime's flag or nothing; precedence explicit > PID > config > strong.
func TestTiers(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	claude, _ := a.LoadRuntime("claude")
	codex, _ := a.LoadRuntime("codex")
	grok, _ := a.LoadRuntime("grok")
	if claude.Model(TierStrong) != "claude-fable-5" || claude.Model(TierStandard) != "claude-opus-5" || claude.Model(TierFast) != "claude-sonnet-5" {
		t.Errorf("claude tier map: %v", claude.Models)
	}
	if codex.Model(TierStrong) != "" || codex.Model(TierFast) != "" || grok.Model(TierStandard) != "" {
		t.Error("codex/grok have no tier mapping yet — runtime default")
	}
	if claude.ModelText(TierFast) != "--model 'claude-sonnet-5'" || codex.ModelText(TierFast) != "" {
		t.Errorf("ModelText: %q %q", claude.ModelText(TierFast), codex.ModelText(TierFast))
	}
	// fast falls back to standard when only standard is mapped.
	rt := &Runtime{Models: map[string]string{TierStandard: "m-std"}, ModelFlag: "-m %s"}
	if rt.Model(TierFast) != "m-std" || rt.Model(TierStrong) != "" {
		t.Errorf("fast fallback: %q %q", rt.Model(TierFast), rt.Model(TierStrong))
	}

	ag := loadTestAgent(t, "---\nname: p\ntier: fast\ntier_floor: standard\n---\nYou are p.\n")
	if ag.Tier != "fast" || ag.TierFloor != "standard" {
		t.Errorf("tier keys: %q %q", ag.Tier, ag.TierFloor)
	}
	for _, c := range []struct {
		rt         *Runtime
		tier, want string
	}{
		{claude, TierStrong, "claude --model 'claude-fable-5' " + ClaudeFleetFlags + " --append-system-prompt"},
		{claude, TierStandard, "claude --model 'claude-opus-5' " + ClaudeFleetFlags + " --append-system-prompt"},
		{claude, TierFast, "claude --model 'claude-sonnet-5' " + ClaudeFleetFlags + " --append-system-prompt"},
		{codex, TierFast, "codex -s workspace-write --add-dir '"},
		{grok, TierStrong, "grok " + GrokFleetFlags + ` --rules="$(cat '`},
	} {
		if got := ag.RenderCommandFor(c.rt, "claude", c.tier); !strings.HasPrefix(got, c.want) || strings.Contains(got, "{model}") {
			t.Errorf("%s/%s: %s", c.rt.Name, c.tier, got)
		}
	}
	// Precedence: explicit > PID > config default_tier > strong.
	plain := loadTestAgent(t, "---\nname: p\n---\nYou are p.\n")
	if r := a.ResolveTier("", plain); r != TierStrong {
		t.Errorf("default tier %q", r)
	}
	os.WriteFile(a.ConfigPath, []byte("default_tier: standard\n"), 0o644)
	if r := a.ResolveTier("", plain); r != TierStandard {
		t.Errorf("config default_tier: %q", r)
	}
	if r := a.ResolveTier("", ag); r != TierFast {
		t.Errorf("PID over config: %q", r)
	}
	if r := a.ResolveTier(TierStrong, ag); r != TierStrong {
		t.Errorf("explicit over PID: %q", r)
	}
	// Template-only runtimes may map tiers themselves.
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"), []byte("command: mycli {model} --sys {file}\nmodel_strong: big\nmodel_flag: --engine\n"), 0o644)
	my, err := a.LoadRuntime("mycli")
	if err != nil {
		t.Fatal(err)
	}
	if got := ag.RenderCommandFor(my, "mycli", TierStrong); !strings.HasPrefix(got, "mycli --engine 'big' --sys '") {
		t.Errorf("template-only model: %s", got)
	}
	if got := ag.RenderCommandFor(my, "mycli", TierFast); !strings.HasPrefix(got, "mycli --sys '") {
		t.Errorf("template-only unmapped tier must render nothing: %s", got)
	}
	// agent check: a PID's own command: without {model} is warned about;
	// bad tier values are findings.
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "old.md"), []byte("---\nname: old\ncommand: claude --append-system-prompt \"$(cat {file})\" {allow} {deny}\ntier: huge\n---\nYou are old.\n"), 0o644)
	fs, _, _ := a.CheckAgent("old")
	joined := strings.Join(fs, "\n")
	if !strings.Contains(joined, "has no {model}") || !strings.Contains(joined, `tier: "huge"`) {
		t.Errorf("agent check tier findings:\n%s", joined)
	}
}

// ADR 0006 §4: a `## Handoffs` row says the shape, not just the name —
// who · label · what the bead must contain. The greppable part is the
// label; the rest is prose a reviewer reads.
func TestExampleAgentsHandoffsAreShapes(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "agents")
	a := &App{Home: t.TempDir(), AgentsDir: dir}
	names := a.ListAgents()
	if len(names) < 9 {
		t.Skipf("reference PIDs not present (%d found)", len(names))
	}
	for _, name := range names {
		ag, err := a.LoadAgent(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		h := BodySection(ag.Body, "## Handoffs")
		if !strings.Contains(h, "ADR 0006") {
			t.Errorf("%s: Handoffs does not name the ADR that decides the shapes", name)
		}
		if !strings.Contains(h, "Take from") || !strings.Contains(h, "Hand to") {
			t.Errorf("%s: Handoffs must keep both directions:\n%s", name, h)
		}
		rows := 0
		for _, ln := range strings.Split(h, "\n") {
			if strings.HasPrefix(strings.TrimSpace(ln), "- ") && strings.Contains(ln, " · ") {
				rows++
			}
		}
		if rows < 2 {
			t.Errorf("%s: %d rows in `who · label · what` form, want at least 2:\n%s", name, rows, h)
		}
	}
}

// The two builders stop saying "hand to qa": the verify bead is filed on
// their close (ADR 0006 §3), and saying otherwise teaches a habit the
// harness contradicts.
func TestBuilderPIDsDoNotHandVerifyOff(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "agents")
	a := &App{Home: t.TempDir(), AgentsDir: dir}
	for _, name := range []string{"developer", "devops"} {
		ag, err := a.LoadAgent(name)
		if err != nil {
			t.Skipf("%s not present: %v", name, err)
		}
		h := BodySection(ag.Body, "## Handoffs")
		if !strings.Contains(h, "nothing to file") {
			t.Errorf("%s: the qa row must say the verify bead is filed for them:\n%s", name, h)
		}
		if strings.Contains(h, "raw material") {
			t.Errorf("%s: still hands verification off by hand", name)
		}
	}
}
