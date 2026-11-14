//go:build !posse_arm2 && !posse_arm3

package posse

// Render pin for ranger-base-wj93u, the seatbelt half of ranger-base-tgc4b
// D3/D4. The process it protects: the operator's prompt recall on darwin.
// An uncaged claude's up-arrow and search read <cfg>/history.jsonl and
// replay a row's paste body, stored inline in the row or in
// <cfg>/paste-cache/<hash>.txt, as the operator's own submitted turn, and
// the composer shows only a placeholder. A caged claude seat's profile must
// deny reading and writing both names, the NAME included (claude's append
// follows a link, so unlink, create and a rename onto history.jsonl must be
// refused too), and nothing later in the profile may reopen them. A codex
// seat's profile names neither.
//
// From any host the render arm reads the SBPL text. The live arm plants a
// link and appends under the rendered profile, and runs only where
// sandbox-exec can apply one (darwin); elsewhere it skips and says so.
//
//	go test ./internal/posse -run 'WallClaudeHistorySeatbelt' -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAWallClaudeHistorySeatbeltDeniesReadAndWriteForClaudeOnly(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := cpFixtureIn(t, t.TempDir())
	cfg := absResolve(f.cfg)
	history := filepath.Join(cfg, "history.jsonl")
	pastes := filepath.Join(cfg, "paste-cache")
	head := "(deny file-read* file-write*"

	prof, c := sbRender(t, f, f.rt, f.dir)
	if sbLine(prof, "(subpath "+sbQuote(cfg)+")") < 0 {
		t.Fatalf("the fixture's config dir is not granted, so nothing here is measured:\n%s", prof)
	}
	lines := strings.Split(prof, "\n")
	h := sbLine(prof, head)
	if h < 0 {
		t.Fatalf("no read-and-write deny block (ranger-base-wj93u):\n%s", prof)
	}
	for _, p := range []string{history, pastes} {
		member := "(subpath " + sbQuote(p) + ")"
		in := false
		for i := h + 1; i < len(lines) && strings.TrimSpace(lines[i]) != ")"; i++ {
			if strings.TrimSpace(lines[i]) == member {
				in = true
			}
		}
		if !in {
			t.Errorf("%s is not in the read-and-write deny block at line %d, so a seat can rewrite what the operator's recall submits:\n%s", p, h, prof)
		}
		found := false
		for _, d := range c.DenyAll {
			found = found || d == p
		}
		if !found {
			t.Errorf("carve DenyAll = %v, want %s in it", c.DenyAll, p)
		}
	}
	// Last match wins: no allow may follow the block, or it could reopen it.
	for i := h; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "(allow") {
			t.Errorf("an allow at line %d follows the read-and-write deny at %d, so it could outvote it: %s\n%s", i, h, lines[i], prof)
		}
	}

	// bwrap spells history.jsonl as a private file bind and paste-cache/ as
	// a private dir (ranger-base-a544g), so its carve carries no SBPL deny.
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Carve.DenyAll) != 0 {
		t.Errorf("the bwrap render carries seatbelt DenyAll: %v", r.Carve.DenyAll)
	}

	other := &Runtime{Name: "codex", StateDirs: []string{"~/.codex"}}
	if err := os.MkdirAll(filepath.Join(f.home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	op, oc := sbRender(t, f, other, f.dir)
	for _, p := range []string{history, pastes} {
		if sbLine(op, "(subpath "+sbQuote(p)+")") >= 0 {
			t.Errorf("a codex seat gets claude's %s deny, which nothing granted it:\n%s", p, op)
		}
	}
	if len(oc.DenyAll) != 0 {
		t.Errorf("a codex seat's DenyAll = %v, want none", oc.DenyAll)
	}
}

// A link at either name is denied at both spellings: SBPL matches the
// resolved path, so the link's own name alone would deny nothing.
func TestQAWallClaudeHistorySeatbeltALinkIsDeniedAtBothSpellings(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "cfg")
	elsewhere := filepath.Join(d, "elsewhere")
	for _, p := range []string{cfg, elsewhere} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(ClaudeConfigDirVar, cfg)
	rcfg := absResolve(cfg)
	hist := filepath.Join(rcfg, "history.jsonl")
	pc := filepath.Join(rcfg, "paste-cache")
	if got := claudePromptHistory(d, []string{rcfg}); len(got) != 2 || got[0] != hist || got[1] != pc {
		t.Fatalf("no link: %v, want [%s %s]", got, hist, pc)
	}
	histT := filepath.Join(elsewhere, "h.jsonl")
	pcT := filepath.Join(elsewhere, "pc")
	if err := os.WriteFile(histT, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pcT, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(histT, hist); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(pcT, pc); err != nil {
		t.Fatal(err)
	}
	got := claudePromptHistory(d, []string{rcfg})
	want := []string{hist, absResolve(histT), pc, absResolve(pcT)}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("links at both names: %v, want %v", got, want)
	}
	if got := claudePromptHistory(d, nil); got != nil {
		t.Errorf("no state grant reaches the config dir, yet %v", got)
	}
}

// The live half, where a profile can be applied: under the caged claude
// seat's rendered profile, every way of changing what the operator's recall
// reads is refused, while a sibling in the same granted config dir stays
// writable (so the refusals are the deny's and not a missing grant).
func TestQAWallClaudeHistorySeatbeltLiveRefusesTheNameAndItsContents(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := cpFixtureIn(t, t.TempDir())
	cfg := absResolve(f.cfg)
	history := filepath.Join(cfg, "history.jsonl")
	pastes := filepath.Join(cfg, "paste-cache")
	if err := os.WriteFile(history, []byte("{\"display\":\"the operator's prompt\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pastes, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pastes, "abc.txt"), []byte("the operator's paste\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prof, _ := sbRender(t, f, f.rt, f.dir)
	pf := sbRenderProfile(t, "claude.sb", prof)
	planted := filepath.Join(cfg, "planted.jsonl")
	q := shellQuote
	if !sbRun(t, pf, "printf x > "+q(planted)) {
		t.Fatalf("control: a sibling in the granted config dir is not writable, so no refusal below is measured")
	}
	for name, sh := range map[string]string{
		"append to history.jsonl":       "printf x >> " + q(history),
		"read history.jsonl":            "cat " + q(history),
		"replace history.jsonl by link": "ln -sf " + q(planted) + " " + q(history),
		"rename onto history.jsonl":     "mv -f " + q(planted) + " " + q(history),
		"unlink history.jsonl":          "rm -f " + q(history),
		"rewrite a cached paste":        "printf x > " + q(filepath.Join(pastes, "abc.txt")),
		"read a cached paste":           "cat " + q(filepath.Join(pastes, "abc.txt")),
		"add a cached paste":            "printf x > " + q(filepath.Join(pastes, "new.txt")),
	} {
		if sbRun(t, pf, sh) {
			t.Errorf("%s was allowed under a caged claude seat's profile (ranger-base-wj93u): %s", name, sh)
		}
	}
	if fi, err := os.Lstat(history); err != nil || !fi.Mode().IsRegular() {
		t.Errorf("history.jsonl is no longer the operator's regular file: %v %v", fi, err)
	}
}
