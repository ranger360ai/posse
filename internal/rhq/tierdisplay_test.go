package rhq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR 0013 §6: the tier name is intent, and a runtime that maps no model id
// for it does not wear it. The three names do not change and neither does
// resolution — only what the operator READS.
//
// The discriminating pair is grok vs codex at the same tier: both are
// non-default runtimes, so "not claude" cannot explain the difference. Only
// the model map can.
func TestDisplayTierIsDefaultOnlyWhereTheRuntimeMapsNothing(t *testing.T) {
	b, _ := newTestBackend(t)
	a := b.App
	for _, c := range []struct{ runtime, tier, want string }{
		// grok maps nothing: every tier reads default.
		{"grok", TierStrong, TierUnmapped},
		{"grok", TierStandard, TierUnmapped},
		{"grok", TierFast, TierUnmapped},
		// codex maps all three (fast → luna), claude maps all three.
		{"codex", TierStrong, TierStrong},
		{"codex", TierFast, TierFast},
		{"claude", TierStrong, TierStrong},
		{"claude", TierStandard, TierStandard},
		{"claude", TierFast, TierFast},
		// A runtime nobody has heard of promises nothing.
		{"nosuchcli", TierStrong, TierUnmapped},
		// Not one of the three names: corruption in a session meta, shown as
		// it is rather than laundered into `default`.
		{"grok", "premium", "premium"},
		{"claude", "premium", "premium"},
		// Nothing in, nothing out.
		{"grok", "", ""},
	} {
		if got := a.DisplayTier(c.runtime, c.tier); got != c.want {
			t.Errorf("DisplayTier(%q, %q) = %q, want %q", c.runtime, c.tier, got, c.want)
		}
	}

	// A DECLARED runtime is answered by its own model_<tier>:, not by a
	// built-in list — including the fast → standard fallback Model() applies,
	// which is a mapping and so is worn.
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"),
		[]byte("command: mycli {model} --sys {file}\nmodel_standard: mid\n"), 0o644)
	for _, c := range []struct{ tier, want string }{
		{TierStandard, TierStandard},
		{TierFast, TierFast}, // falls back to model_standard: mid
		{TierStrong, TierUnmapped},
	} {
		if got := a.DisplayTier("mycli", c.tier); got != c.want {
			t.Errorf("DisplayTier(mycli, %q) = %q, want %q", c.tier, got, c.want)
		}
	}
}

// The listing tag (`posse list`, cockpit) renders the display tier, and the
// "" suppression is keyed on the DISPLAYED tier — so an unmapped default
// tier surfaces as a tag instead of vanishing.
func TestRuntimeTierTagShowsTheDisplayTier(t *testing.T) {
	b, _ := newTestBackend(t)
	for _, c := range []struct{ runtime, tier, want string }{
		{"claude", TierStrong, ""},
		{"claude", "", ""},
		{"", "", ""},
		{"claude", TierFast, "@claude/fast"},
		{"codex", "", "@codex/strong"},
		{"grok", TierStandard, "@grok/default"},
		{"grok", TierStrong, "@grok/default"},
		{"grok", "", "@grok/default"},
	} {
		if got := b.App.RuntimeTierTag(c.runtime, c.tier); got != c.want {
			t.Errorf("RuntimeTierTag(%q, %q) = %q, want %q", c.runtime, c.tier, got, c.want)
		}
	}
}

// `posse list` end to end: a grok session dispatched at standard lists as
// grok/default while a claude session at fast still lists as claude/fast.
// Both panes in one listing, so the rendering — not the fixture — is what
// separates them.
func TestListShowsDefaultForAnUnmappedRuntime(t *testing.T) {
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndescription: d\n---\nYou are dev, the developer of the crew.\n"), 0o644)
	mustCreate(t, b, NewSessionOpts{Name: "g1", Agent: "dev", Runtime: "grok", Tier: TierStandard})
	mustCreate(t, b, NewSessionOpts{Name: "c1", Agent: "dev", Runtime: "claude", Tier: TierFast})

	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	got := list.String()
	if !strings.Contains(got, "@grok/default") || strings.Contains(got, "@grok/standard") {
		t.Errorf("a grok standard session must list as grok/default:\n%s", got)
	}
	if !strings.Contains(got, "@claude/fast") {
		t.Errorf("a mapped tier still wears its own name:\n%s", got)
	}
}

// The work-prompt header the persona reads (ADR 0005) carries the display
// tier too — and PromptContext keeps the resolved tier out of the struct
// entirely, so nothing downstream can mistake `default` for a resolution.
func TestWorkPromptHeaderShowsTheDisplayTier(t *testing.T) {
	b, _ := newTestBackend(t)
	is := RepoIssue{BdIssue: BdIssue{ID: "b-1", Title: "t"}, Dir: t.TempDir()}
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}

	ctx := b.App.promptContext(bd, is, "grok", TierStandard, "", nil)
	if ctx.TierShown != TierUnmapped {
		t.Errorf("grok standard → TierShown = %q, want %q", ctx.TierShown, TierUnmapped)
	}
	if p := workPrompt(is, ctx); !strings.Contains(p, "runtime/tier: grok/default") {
		t.Errorf("header must read grok/default:\n%s", p)
	}
	ctx = b.App.promptContext(bd, is, "claude", TierStrong, "", nil)
	if p := workPrompt(is, ctx); !strings.Contains(p, "runtime/tier: claude/strong") {
		t.Errorf("a mapped tier keeps its name in the header:\n%s", p)
	}
}

// `posse agent check`: a PID naming both a runtime and a tier the runtime
// maps nothing for is a WARNING — the PID is coherent, it just is not a
// quality guarantee. It must not become a finding (that would refuse work a
// lane legitimately wants), and it must not fire where the mapping exists.
func TestCheckAgentWarnsAStrongPidOnAnUnmappedRuntime(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.AgentsDir, 0o755)
	check := func(front string) (string, string) {
		t.Helper()
		os.WriteFile(filepath.Join(a.AgentsDir, "p.md"),
			[]byte("---\nname: p\n"+front+"---\nYou are p, the developer of the crew.\n"), 0o644)
		f, w, err := a.CheckAgent("p")
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(f, "\n"), strings.Join(w, "\n")
	}

	f, w := check("runtime: grok\ntier: strong\n")
	if !strings.Contains(w, "tier: strong on runtime: grok is intent, not a guarantee") ||
		!strings.Contains(w, "grok/default") {
		t.Errorf("a strong PID on grok must warn: %q", w)
	}
	if strings.Contains(f, "tier: strong on runtime: grok") {
		t.Errorf("it is a warning, not a finding: %q", f)
	}
	// The mapped cases stay silent — otherwise the warning says nothing.
	for _, front := range []string{
		"runtime: claude\ntier: strong\n",
		"runtime: codex\ntier: fast\n",
		// No runtime: declared — the tier is then a fact about config
		// default_runtime, and `posse runtime check` is where that is read.
		"tier: strong\n",
	} {
		if _, w := check(front); strings.Contains(w, "is intent, not a guarantee") {
			t.Errorf("%q must not warn: %q", front, w)
		}
	}
	// An invalid tier is still the finding it was; the §6 warning does not
	// displace it.
	if f, w := check("runtime: grok\ntier: premium\n"); !strings.Contains(f, `tier: "premium" is not strong`) ||
		strings.Contains(w, "is intent, not a guarantee") {
		t.Errorf("invalid tier: findings %q warnings %q", f, w)
	}
}
