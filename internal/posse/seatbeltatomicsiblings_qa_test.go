//go:build posse_arm3

package posse

// ranger-base-cypy1 (from ranger-base-gr3ow's silence-check half): a caged
// claude session lost EVERY write to ~/.claude.json, silently, for as long
// as the seatbelt tier has existed.
//
// The grant named the file — `(subpath "$HOME/.claude.json")` — and `$HOME`
// was granted by nothing, deliberately. But claude does not write that
// file: it creates `$HOME/.claude.json.tmp.<pid>.<hex>` beside it, holds
// `$HOME/.claude.json.lock`, and renames the temp into place. `subpath` is
// component-aware, so neither sibling was covered, both were refused at the
// kernel, and the CLI printed `Added stdio MCP server …`, printed `File
// modified: <path>`, and exited 0 with the file byte-identical. MCP adds,
// seenNotifications, tips, history and projects[<dir>] all went the same
// way.
//
// Measured on the operator's box (2026-09-02, `log show --last 24h`): the
// two sibling paths were the #1 and #2 write denials by volume — 97
// `.claude.json.lock`, 88 `.claude.json.tmp.<pid>.<hex>` — ahead of
// everything else under $HOME combined.
//
// The pins below run BOTH ways on purpose, because the cheap fix here is
// `(subpath $HOME)` and every "the write lands" assertion goes green under
// it. So each allow arm is paired with a refusal that the home-wide grant
// would break: $HOME itself stays unwritable, and `.claude.jsonbar` — a
// name extending the base WITHOUT the separator dot — stays refused. That
// second one is not hypothetical: this box carries `~/.codexbar` beside
// `~/.codex` and `~/.grokbot` beside `~/.grok`, so a bare-prefix regex
// would hand a caged session two other tools' trees.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sbSibFixture is a launch shaped like a caged claude one: a fake HOME
// carrying the state dir AND the state FILE, a work tree, and an app whose
// home is elsewhere. Returns the app, the agent, cwd, the gates dir and the
// resolved config path.
type sbSibFixture struct {
	a     *App
	ag    *AgentFile
	work  string
	gates string
	home  string
	cfg   string
}

func sbNewSibFixture(t *testing.T) sbSibFixture {
	t.Helper()
	root := sbRoot(t) // also parks HOME and TMPDIR away from the fixture
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// The shape the runtime declares: a state DIRECTORY and a state FILE.
	sbMkdir(t, filepath.Join(home, ".claude"))
	cfg := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(cfg, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := NewAppAt(filepath.Join(root, "posse-home"))
	return sbSibFixture{
		a:     a,
		ag:    &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))},
		work:  sbMkdir(t, filepath.Join(root, "work")),
		gates: sbMkdir(t, a.GatesDir("developer")),
		home:  home,
		cfg:   absResolve(cfg),
	}
}

// claudeStateDirs is the built-in declaration this bead is about, read from
// the registry rather than respelled — a pin that hardcodes `~/.claude.json`
// keeps passing the day the runtime stops declaring it.
func claudeStateDirs(t *testing.T) []string {
	t.Helper()
	for _, rt := range builtinRuntimes {
		if rt.Name == "claude" {
			if len(rt.StateDirs) == 0 {
				t.Fatal("the claude runtime declares no state_dir: this pin would measure an empty list")
			}
			return rt.StateDirs
		}
	}
	t.Fatal("no claude runtime in the registry")
	return nil
}

// ─── set arithmetic: which entries get a sibling namespace at all ────────────

func TestSeatbeltSiblingsCoversTheStateFileAndNotTheStateTree(t *testing.T) {
	f := sbNewSibFixture(t)
	got := SeatbeltSiblings(claudeStateDirs(t), SeatbeltCarveOut{})

	if len(got) != 1 || got[0] != f.cfg {
		t.Fatalf("the sibling bases for a claude launch are %v; want exactly [%s]\n"+
			"  the FILE entry needs one (its .lock and .tmp.<pid> are refused without it)\n"+
			"  the DIRECTORY entry must not have one: ~/.claude is granted whole already, "+
			"and a prefix grant there is reach with no writer behind it (ranger-base-9fl)",
			got, f.cfg)
	}
}

// The three drops, each with the base present in a control call so the
// absence is a decision and not an empty list.
func TestSeatbeltSiblingsDropsWhatItCannotSafelyGrant(t *testing.T) {
	f := sbNewSibFixture(t)
	denied := SeatbeltCarveOut{Deny: []string{f.cfg}}

	if got := SeatbeltSiblings([]string{f.cfg}, SeatbeltCarveOut{}); len(got) != 1 {
		t.Fatalf("control: %s gets no sibling grant even with an empty carve-out (%v) — the drops below prove nothing", f.cfg, got)
	}
	if got := SeatbeltSiblings([]string{f.cfg}, denied); len(got) != 0 {
		t.Errorf("a base the carve-out DENIES still got its sibling namespace: %v\n"+
			"  the siblings live in the base's own directory, so a PID's Edit(<file>) deny "+
			"would leave the .tmp/.lock scratch space open beside a file it walled (ADR 0014 §3)", got)
	}

	quoted := filepath.Join(f.home, `we"ird.json`)
	if err := os.WriteFile(quoted, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := SeatbeltSiblings([]string{quoted}, SeatbeltCarveOut{}); len(got) != 0 {
		t.Errorf("a path carrying a double quote was rendered as a sibling regex: %v\n"+
			`  #"…" has no escape for a quote — the literal ends there and sandbox-exec `+
			"refuses the whole profile, turning a lost config write into a dead pane", got)
	}

	missing := filepath.Join(f.home, ".notyet")
	if got := SeatbeltSiblings([]string{missing}, SeatbeltCarveOut{}); len(got) != 1 {
		t.Errorf("a state_dir that does not exist yet got no sibling grant: %v\n"+
			"  it cannot be told from a file, the CLI is about to create it, and the "+
			"alternative is a grant that appears only on boxes that ran the CLI before", got)
	}
}

// ─── the rendered profile, through the production path ───────────────────────

// RenderSeatbelt is what a launch actually writes. The regex must be in it
// and $HOME must not — the second half is the arm the cheap fix breaks.
func TestRenderedProfileCarriesTheSiblingRegexAndNotAHomeWideGrant(t *testing.T) {
	f := sbNewSibFixture(t)
	p, err := f.a.RenderSeatbelt(f.ag, f.work, claudeStateDirs(t)...)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	prof := string(b)

	if want := "(regex " + sbSiblingRegex(f.cfg) + ")"; !strings.Contains(prof, want) {
		t.Errorf("the rendered profile does not carry %s\n%s", want, prof)
	}
	if bad := "(subpath " + sbQuote(absResolve(f.home)) + ")"; strings.Contains(prof, bad) {
		t.Errorf("the profile grants the home whole (%s) — the fix is the sibling namespace, "+
			"not $HOME; a home-wide grant hands a session every dotfile on the box\n%s", bad, prof)
	}
	if bad := "(regex " + sbSiblingRegex(absResolve(filepath.Join(f.home, ".claude"))) + ")"; strings.Contains(prof, bad) {
		t.Errorf("the DIRECTORY state dir got a sibling regex (%s): on the operator's box that "+
			"reaches ~/.claude.json's neighbours for no writer at all\n%s", bad, prof)
	}
}

// ─── the kernel ──────────────────────────────────────────────────────────────

// The claim as the sandbox answers it: claude's real write sequence lands,
// and the two things a home-wide grant would also open stay shut. The
// control is the SAME profile with the sibling list emptied — if the lock
// and temp are allowed there, nothing below is measuring this fix.
func TestQACagedClaudeConfigWriteLandsAndTheHomeStaysShut(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := sbNewSibFixture(t)
	stateDirs := claudeStateDirs(t)

	w := f.a.SeatbeltWritable(f.ag, f.work, f.gates, stateDirs...)
	carve := f.a.SeatbeltCarveOut(f.ag, f.work, f.gates, w, stateDirs...)
	sibs := SeatbeltSiblings(stateDirs, carve)
	if len(sibs) == 0 {
		t.Fatal("no sibling bases for a claude launch: every arm below would grade the control twice")
	}
	fixed := sbRenderProfile(t, "fixed.sb", SeatbeltProfile(f.ag.Name, w, sibs, carve))
	ctrl := sbRenderProfile(t, "control.sb", SeatbeltProfile(f.ag.Name, w, nil, carve))

	lock := f.cfg + ".lock"
	tmp := f.cfg + ".tmp.4242.deadbeefcafe"
	// A name extending the base with no separator dot, and the file itself
	// under a different name: neither is claude's and neither may be granted.
	bar := f.cfg + "bar"
	other := filepath.Join(f.home, ".ssh-probe")

	// The control arm, first and on a fresh fixture: this is the defect.
	for _, p := range []string{lock, tmp} {
		os.Remove(p)
		if sbRun(t, ctrl, "echo x > "+shellQuote(p)) {
			t.Fatalf("CONTROL FAILED: %s is already writable with the sibling grant removed — "+
				"something else in the profile reaches it and the arms below grade nothing", p)
		}
	}

	// The fix: claude's write sequence, in order — take the lock, write the
	// temp, rename it onto the config.
	os.Remove(lock)
	if !sbRun(t, fixed, "echo x > "+shellQuote(lock)) {
		t.Errorf("%s is still refused: claude takes this lock before every config write "+
			"(97 denials in 24h on the operator's box)", lock)
	}
	os.Remove(tmp)
	if !sbRun(t, fixed, "echo '{\"probe\":1}' > "+shellQuote(tmp)) {
		t.Errorf("%s is still refused: this is the file claude renames onto the config, "+
			"and without it the write is lost while the CLI reports success", tmp)
	}
	if !sbRun(t, fixed, "mv "+shellQuote(tmp)+" "+shellQuote(f.cfg)) {
		t.Errorf("the rename of %s onto %s is refused — the temp landing but the rename "+
			"failing loses the write just as completely", tmp, f.cfg)
	}
	if b, err := os.ReadFile(f.cfg); err != nil || !strings.Contains(string(b), "probe") {
		t.Errorf("after the full sequence the config still does not hold the write: %q (%v)\n"+
			"  this is the observable the bead is about — not the denial, the LOST WRITE", b, err)
	}

	// The other direction, under the SAME profile: the fix must not have
	// become a home grant by another spelling.
	for _, p := range []string{bar, other} {
		os.Remove(p)
		if sbRun(t, fixed, "echo x > "+shellQuote(p)) {
			t.Errorf("%s is writable under the fixed profile — the grant must stop at the "+
				"separator dot; this box carries ~/.codexbar beside ~/.codex and ~/.grokbot "+
				"beside ~/.grok, which is what a bare prefix would hand a session", p)
		}
	}
	// And the base itself is granted by its own subpath entry, not by the
	// regex: the witness that the regex adds siblings and never a root.
	if !sbRun(t, fixed, "echo x >> "+shellQuote(f.cfg)) {
		t.Errorf("%s is not writable at all — the state_dir subpath grant is gone, so the "+
			"arms above measured a profile no launch renders", f.cfg)
	}
}

// ─── the constitution check sees the new grant kind ──────────────────────────

// A sibling namespace can contain a constitution path without the base
// being under it or over it — `<home>/config` and `<home>/config.yaml` are
// neither, by filepath.Rel. ConstitutionGrants is the wall `posse gates`
// prints, so it must answer for this shape too (ADR 0015 §2).
func TestConstitutionGrantsSeesASiblingNamespaceReachingTheHome(t *testing.T) {
	f := sbNewSibFixture(t)
	cfg := f.a.ConfigPath
	base := strings.TrimSuffix(cfg, filepath.Ext(cfg))
	if base == cfg {
		t.Fatalf("the home config %s has no extension to trim: this pin cannot build the shape it is about", cfg)
	}

	if bad := f.a.ConstitutionGrants(nil); len(bad) != 0 {
		t.Fatalf("control: an empty writable set already reaches %v — the assertion below cannot fail", bad)
	}
	if bad := f.a.ConstitutionGrants(nil, base); len(bad) == 0 {
		t.Errorf("a sibling base of %s reaches the promoted config and ConstitutionGrants said nothing.\n"+
			"  `<base>.<suffix>` is not UNDER `<base>`, so the underDir test above it passes "+
			"while the profile grants the file — the exact shape a deny-list cannot be audited for", cfg)
	}
}
