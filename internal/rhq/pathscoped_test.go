package rhq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR 0014 §1: the grammar. A glob is a subtree glob when, after stripping
// one trailing `/**` or `/`, the remainder carries no `*`, `?` or `[`. The
// three whole-tree spellings are the bare rule, and everything else with a
// metacharacter left is a file filter no wall of ours can express.
func TestPathScopedWriteGrammar(t *testing.T) {
	for _, tc := range []struct {
		rule          string
		ok, bare, sub bool
		path          string
	}{
		{rule: "Edit(docs/adr/**)", ok: true, sub: true, path: "docs/adr"},
		{rule: "Write(docs/adr/)", ok: true, sub: true, path: "docs/adr"},
		{rule: "NotebookEdit(docs/adr)", ok: true, sub: true, path: "docs/adr"},
		{rule: "Edit( docs/adr/** )", ok: true, sub: true, path: "docs/adr"},
		{rule: "Edit(~/secrets/**)", ok: true, sub: true, path: "~/secrets"},
		{rule: "Edit(/etc/**)", ok: true, sub: true, path: "/etc"},
		// The root is a subtree too — an absolute glob that strips to
		// nothing must not silently become the session dir.
		{rule: "Edit(/**)", ok: true, sub: true, path: "/"},
		// The whole tree, however it is spelled.
		{rule: "Edit(**)", ok: true, bare: true},
		{rule: "Write(*)", ok: true, bare: true},
		{rule: "NotebookEdit(.)", ok: true, bare: true},
		{rule: "Edit(./)", ok: true, bare: true},
		{rule: "Edit()", ok: true, bare: true},
		// Metacharacter survives the strip: a file filter.
		{rule: "Edit(**/*.md)", ok: true},
		{rule: "Edit(docs/adr/**/*.md)", ok: true},
		{rule: "Edit(docs/adr*)", ok: true},
		{rule: "Edit(docs/adr/[ab]/**)", ok: true},
		{rule: "Edit(docs/?dr/**)", ok: true},
		// Not this arm at all.
		{rule: "Edit"}, {rule: "Bash(git push:*)"}, {rule: "WebFetch"},
		{rule: "mcp__x__y"}, {rule: "Edit(docs"}, {rule: "Editor(docs/**)"},
	} {
		d, ok := parsePathScopedWrite(tc.rule)
		if ok != tc.ok || d.Bare != tc.bare || d.Subtree != tc.sub || d.Path != tc.path {
			t.Errorf("%s: ok=%v bare=%v subtree=%v path=%q, want ok=%v bare=%v subtree=%v path=%q",
				tc.rule, ok, d.Bare, d.Subtree, d.Path, tc.ok, tc.bare, tc.sub, tc.path)
		}
	}

	// Resolve is the dir-dependent half, and the reason CheckParity never
	// calls it: a relative glob with no session dir names nothing.
	dir := t.TempDir()
	d, _ := parsePathScopedWrite("Edit(docs/adr/**)")
	if got, want := d.Resolve(dir), absResolve(filepath.Join(dir, "docs/adr")); got != want {
		t.Errorf("relative glob joins the session dir: %q want %q", got, want)
	}
	if d.Resolve("") != "" {
		t.Errorf("no session dir, no path: %q", d.Resolve(""))
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	tl, _ := parsePathScopedWrite("Edit(~/secrets/**)")
	if got, want := tl.Resolve(dir), absResolve(filepath.Join(home, "secrets")); got != want {
		t.Errorf("~ expands as writable: expands it: %q want %q", got, want)
	}
	nf, _ := parsePathScopedWrite("Edit(**/*.md)")
	if nf.Resolve(dir) != "" {
		t.Error("a file filter names no directory")
	}

	// `Edit(**)` must mean the same thing to every wall that reads the bare
	// rule, or the matrix and the mount disagree about one PID.
	long := []string{"Edit(**)", "Write(*)"}
	if w := wholeTreeWriteDeny(long); !w["Edit"] || !w["Write"] || !deniesFileWrite(long) {
		t.Errorf("the long spelling is the bare rule to the mount boundary too: %v", w)
	}
	if r := realizeCodex(nil, long, ""); r.Deny != "-s read-only" {
		t.Errorf("...and to codex's mode: %q", r.Deny)
	}
	if w := wholeTreeWriteDeny([]string{"Edit(docs/adr/**)"}); len(w) != 0 || deniesFileWrite([]string{"Edit(docs/adr/**)"}) {
		t.Errorf("a subtree deny must never mount the whole repo :ro: %v", w)
	}
}

// ADR 0014 §2: the realization matrix — runtime × cage × glob shape. Before
// this, a parametrized rule fell to parity's default arm and was classified
// as a tool-name deny (at container, as a stdio MCP server).
func TestCheckParityPathScopedWrites(t *testing.T) {
	a := cageApp(t)
	claude, _ := a.LoadRuntime("claude")
	codex, _ := a.LoadRuntime("codex")
	grok, _ := a.LoadRuntime("grok")
	seatbeltForTest(t)

	scoped := cageAgent(t, a, "deny: [Edit(docs/adr/**), Write(docs/adr/**)]\n")
	filter := cageAgent(t, a, "deny: [Edit(**/*.md)]\n")

	// shims: unrealized on all three, and the message names the fix rather
	// than the tier. codex -s read-only is NOT selected here (the PID never
	// denied the whole tree) and would over-enforce if it were.
	for _, rt := range []*Runtime{claude, grok, codex} {
		p := a.CheckParity(scoped, rt, CageShims, TierStrong)
		if len(p.Unrealized) != 2 {
			t.Fatalf("%s@shims: %+v", rt.Name, p)
		}
		for _, u := range p.Unrealized {
			if !strings.Contains(u, "needs cage: seatbelt (or container) — a path-scoped write is not a tool-name deny") {
				t.Errorf("%s@shims: %q", rt.Name, u)
			}
		}
		if strings.Contains(strings.Join(p.Unrealized, "\n"), "MCP") {
			t.Errorf("%s@shims still reads a subtree glob as a tool name: %+v", rt.Name, p.Unrealized)
		}
	}

	// seatbelt: the trailing deny, on the two runtimes that can be wrapped.
	for _, rt := range []*Runtime{claude, grok} {
		p := a.CheckParity(scoped, rt, CageSeatbelt, TierStrong)
		if len(p.Degraded) != 0 || p.Realized["Edit(docs/adr/**)"] != "L2 trailing deny (subpath docs/adr)" {
			t.Errorf("%s@seatbelt: %+v", rt.Name, p)
		}
	}
	// codex cannot nest under our seatbelt, so nothing there realizes it —
	// which is why ADR 0014 §2 sends codex to container for this feature.
	pc := a.CheckParity(scoped, codex, CageSeatbelt, TierStrong)
	if pc.Realized["Edit(docs/adr/**)"] != "" || !strings.Contains(strings.Join(pc.Degraded, "\n"), "cage seatbelt cannot wrap codex") {
		t.Errorf("codex@seatbelt: %+v", pc)
	}

	// container with the inner gates: the :ro overlay, every runtime.
	for _, rt := range []*Runtime{claude, grok, codex} {
		p := a.CheckParity(scoped, rt, CageContainer, TierStrong)
		if len(p.Degraded) != 0 || p.Realized["Write(docs/adr/**)"] != "L4 :ro overlay (docs/adr)" {
			t.Errorf("%s@container: %+v", rt.Name, p)
		}
	}
	// An image that cannot render them: unrealized, naming the build — and
	// naming the overlay, not the whole-repo :ro that was never asked for.
	noinner := cageApp(t)
	os.WriteFile(filepath.Join(noinner.CagesDir(), "no-inner-4ks.yaml"), []byte(
		"command: env {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\ninner: false {image} {cmd}\n"), 0o644)
	os.WriteFile(noinner.ConfigPath, []byte("default_engine: no-inner-4ks\n"), 0o644)
	ni := noinner.CheckParity(cageAgent(t, noinner, "deny: [Edit(docs/adr/**)]\n"), claude, CageContainer, TierStrong)
	if u := strings.Join(ni.Unrealized, "\n"); !strings.Contains(u, "overlays this subtree :ro") || !strings.Contains(u, "posse cage build") {
		t.Errorf("container without inner gates: %q", u)
	}

	// A non-subtree glob is unrealized at every tier, by construction, and
	// says what to write instead.
	for _, cage := range []string{CageShims, CageSeatbelt, CageContainer} {
		p := a.CheckParity(filter, claude, cage, TierStrong)
		if len(p.Unrealized) != 1 || !strings.Contains(p.Unrealized[0], "not a directory-prefix glob; the wall realizes subtrees (Edit(docs/adr/**)), not file filters") {
			t.Errorf("Edit(**/*.md)@%s: %+v", cage, p.Unrealized)
		}
	}

	// `Edit(**)` is the bare rule's row verbatim — the matrix prints back
	// the rule as written, and claims exactly what the bare name claims.
	long := cageAgent(t, a, "deny: [Edit(**)]\n")
	if p := a.CheckParity(long, claude, CageSeatbelt, TierStrong); p.Realized["Edit(**)"] != "L2 seatbelt" {
		t.Errorf("Edit(**)@seatbelt: %+v", p)
	}
	if p := a.CheckParity(long, claude, CageShims, TierStrong); len(p.Unrealized) != 1 ||
		!strings.Contains(p.Unrealized[0], "needs cage: seatbelt (or codex -s read-only)") {
		t.Errorf("Edit(**)@shims: %+v", p.Unrealized)
	}
	// codex's read-only needs the whole tree denied, so it is reached only
	// by the pair — and then it holds the subtree too, because the subtree
	// is inside it. The redundancy is `posse agent check`'s to name; a
	// refusal here would be the lie in the other direction.
	both := cageAgent(t, a, "deny: [Edit, Write, Edit(docs/adr/**)]\n")
	pb := a.CheckParity(both, codex, CageShims, TierStrong)
	if len(pb.Degraded) != 0 || !strings.Contains(pb.Realized["Edit(docs/adr/**)"], "codex sandbox (OS-enforced): the whole tree") {
		t.Errorf("bare pair + subtree on codex: %+v", pb)
	}
}

// The one thing this bead must not move: a PID with only bare Edit/Write
// reads exactly as it did before path-scoping existed.
func TestBareFileWriteRowUnchangedByPathScoping(t *testing.T) {
	a := cageApp(t)
	claude, _ := a.LoadRuntime("claude")
	codex, _ := a.LoadRuntime("codex")
	seatbeltForTest(t)
	ag := cageAgent(t, a, "deny: [Edit, Write, NotebookEdit]\n")

	for _, tc := range []struct{ cage, gate, want string }{
		{CageShims, "Edit", ""},
		{CageSeatbelt, "Edit", "L2 seatbelt"},
		{CageSeatbelt, "NotebookEdit", "L2 seatbelt"},
		{CageContainer, "Write", "L4 mount boundary (repo mounted :ro)"},
	} {
		if got := a.CheckParity(ag, claude, tc.cage, TierStrong).Realized[tc.gate]; got != tc.want {
			t.Errorf("claude@%s %s: %q want %q", tc.cage, tc.gate, got, tc.want)
		}
	}
	p := a.CheckParity(ag, claude, CageShims, TierStrong)
	if len(p.Unrealized) != 3 || !strings.Contains(p.Unrealized[0], "needs cage: seatbelt (or codex -s read-only) — native flags are politeness") {
		t.Errorf("the shims row is the old one: %+v", p.Unrealized)
	}
	if got := a.CheckParity(ag, codex, CageShims, TierStrong).Realized["Edit"]; got != "codex sandbox (OS-enforced)" {
		t.Errorf("codex read-only still realizes the bare rule: %q", got)
	}
	// And the seatbelt's own writable set: the repo out, .beads/.git in.
	cwd := t.TempDir()
	w := a.SeatbeltWritable(ag, cwd, filepath.Join(a.StateDir, "gates"))
	if !containsPath(w, filepath.Join(cwd, ".beads")) || containsPath(w, cwd) {
		t.Errorf("bare Edit/Write must still drop cwd and keep .beads: %v", w)
	}
}

func containsPath(set []string, p string) bool {
	for _, s := range set {
		if s == absResolve(p) {
			return true
		}
	}
	return false
}

// seatbeltForTest makes the tier available for the duration of a test
// regardless of whether this host has sandbox-exec, and puts the flag back.
func seatbeltForTest(t *testing.T) {
	t.Helper()
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	t.Cleanup(func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	})
}

// ADR 0014 §1/§2 through `posse agent check`: the four things the launch
// would otherwise say, said where the PID is being written.
func TestCheckAgentNamesPathScopedWriteMistakes(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.AgentsDir, 0o755)
	check := func(front string) (string, string) {
		t.Helper()
		os.WriteFile(filepath.Join(a.AgentsDir, "p.md"), []byte("---\nname: p\n"+front+"---\nYou are p, the developer of the crew.\n"), 0o644)
		f, w, err := a.CheckAgent("p")
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(f, "\n"), strings.Join(w, "\n")
	}

	// A file filter: unrealizable at every tier, so a finding.
	if f, _ := check("cage: seatbelt\ndeny: [Edit(**/*.md)]\n"); !strings.Contains(f, "Edit(**/*.md) is not a directory-prefix glob") {
		t.Errorf("file filter: %q", f)
	}
	// The long spelling of the bare rule: it works, so a warning.
	f, w := check("cage: seatbelt\ndeny: [Edit(**)]\n")
	if !strings.Contains(w, "Edit(**) is the bare rule written the long way") || !strings.Contains(w, "write Edit") {
		t.Errorf("Edit(**) warning: %q", w)
	}
	if strings.Contains(f, "Edit(**)") {
		t.Errorf("Edit(**) means the bare rule, which is not a finding: %q", f)
	}
	// cage: shims (here, unset) plus a path-scoped deny: nothing realizes
	// it, so the launch would refuse — a finding.
	f, _ = check("deny: [Edit(docs/adr/**)]\n")
	if !strings.Contains(f, "path-scoped write and this PID launches at cage shims") {
		t.Errorf("shims + path-scoped: %q", f)
	}
	if f, _ := check("cage: seatbelt\ndeny: [Edit(docs/adr/**)]\n"); strings.Contains(f, "path-scoped write and this PID launches") {
		t.Errorf("cage: seatbelt is the fix, not a finding: %q", f)
	}
	// deny-wins: a writable: extra inside a denied subtree grants nothing.
	_, w = check("cage: seatbelt\ndeny: [Edit(docs/adr/**)]\nwritable: [docs/adr/drafts]\n")
	if !strings.Contains(w, "writable: docs/adr/drafts is inside the subtree Edit(docs/adr/**) denies — deny wins") {
		t.Errorf("deny-wins warning: %q", w)
	}
	if _, w := check("cage: seatbelt\ndeny: [Edit(docs/adr/**)]\nwritable: [docs/notes]\n"); strings.Contains(w, "deny wins") {
		t.Errorf("an extra outside the subtree is fine: %q", w)
	}
	// Redundant beside the bare rule: warn, do not fail (ADR 0014 §1).
	f, w = check("cage: seatbelt\ndeny: [Edit, Edit(docs/adr/**)]\n")
	if !strings.Contains(w, "Edit(docs/adr/**) is redundant beside the bare Edit") {
		t.Errorf("redundancy warning: %q", w)
	}
	if strings.Contains(f, "redundant") {
		t.Errorf("redundancy is a warning, never a failure: %q", f)
	}
	// The two shapes ADR 0014 §1 recommends say nothing at all. (The body
	// sections a bare frontmatter fixture lacks are another check's.)
	for _, front := range []string{
		"cage: seatbelt\ndeny: [Edit(docs/adr/**), Write(docs/adr/**)]\n",
		"cage: seatbelt\ndeny: [Edit, Write]\nwritable: [docs/adr]\n",
	} {
		if f, w := check(front); strings.Contains(f+w, "ADR 0014") || strings.Contains(f+w, "deny wins") {
			t.Errorf("%q must be clean: findings=%q warnings=%q", front, f, w)
		}
	}
}
