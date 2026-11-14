//go:build !posse_arm2 && !posse_arm3

package posse

// Render pin for ranger-base-yaha3, the seatbelt half of ranger-base-tii0u
// D3/D4. The process it protects: an UNCAGED claude session's peer
// discovery on darwin, which reads <cfg>/sessions/<pid>.json and connects
// where a row points. A caged claude seat's profile must deny both reading
// and writing that directory, and nothing later in the profile may reopen
// it. A codex seat's profile names nothing of claude's.
//
// From any host: this reads the rendered SBPL text. Whether sandbox-exec
// honours a subpath deny over a granted parent is ASSUMED here, as it is for
// projects/ (ranger-base-r2uxm); no darwin box has run it. The live half is
// the verify lane's.
//
//	go test ./internal/posse -run 'WallClaudeSessionsSeatbelt' -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAWallClaudeSessionsSeatbeltDeniesReadAndWriteForClaudeOnly(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := cpFixtureIn(t, t.TempDir())
	cfg := absResolve(f.cfg)
	sessions := filepath.Join(cfg, "sessions")
	head := "(deny file-read* file-write*"
	member := "(subpath " + sbQuote(sessions) + ")"

	prof, c := sbRender(t, f, f.rt, f.dir)
	if sbLine(prof, "(subpath "+sbQuote(cfg)+")") < 0 {
		t.Fatalf("the fixture's config dir is not granted, so nothing here is measured:\n%s", prof)
	}
	lines := strings.Split(prof, "\n")
	h := sbLine(prof, head)
	if h < 0 {
		t.Fatalf("no read-and-write deny block for claude's sessions/ (ranger-base-yaha3):\n%s", prof)
	}
	// The member sits inside THAT block, not in some write-only one.
	m := -1
	for i := h + 1; i < len(lines) && strings.TrimSpace(lines[i]) != ")"; i++ {
		if strings.TrimSpace(lines[i]) == member {
			m = i
		}
	}
	if m < 0 {
		t.Errorf("sessions/ is not in the read-and-write deny block at line %d:\n%s", h, prof)
	}
	// Last match wins: no allow may follow it, or it could reopen the dir.
	for i := h; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "(allow") {
			t.Errorf("an allow at line %d follows the sessions/ deny at %d, so it could outvote it: %s\n%s", i, h, lines[i], prof)
		}
	}
	if len(c.DenyAll) == 0 || c.DenyAll[0] != sessions {
		t.Errorf("carve DenyAll = %v, want %s first", c.DenyAll, sessions)
	}

	// bwrap spells the same member as a private bind, so its carve carries
	// no SBPL deny (a bwrap read deny would also take the seat's own row).
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
	if len(oc.DenyAll) != 0 || sbLine(op, member) >= 0 {
		t.Errorf("a codex seat gets claude's sessions/ deny, which nothing granted it: %v\n%s", oc.DenyAll, op)
	}
}

// A link at sessions/ is denied at both spellings: SBPL matches the resolved
// path, so the link's own name alone would deny nothing.
func TestQAWallClaudeSessionsSeatbeltALinkIsDeniedAtBothSpellings(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "cfg")
	target := filepath.Join(d, "elsewhere")
	for _, p := range []string{cfg, target} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(ClaudeConfigDirVar, cfg)
	rcfg := absResolve(cfg)
	plain := filepath.Join(rcfg, "sessions")
	if got := claudeSessionsRegistry(d, []string{rcfg}); len(got) != 1 || got[0] != plain {
		t.Fatalf("no link: %v, want [%s]", got, plain)
	}
	if err := os.Symlink(target, plain); err != nil {
		t.Fatal(err)
	}
	got := claudeSessionsRegistry(d, []string{rcfg})
	if len(got) != 2 || got[0] != plain || got[1] != absResolve(target) {
		t.Errorf("a link at sessions/: %v, want [%s %s]", got, plain, absResolve(target))
	}
	if got := claudeSessionsRegistry(d, nil); got != nil {
		t.Errorf("no state grant reaches the config dir, yet %v", got)
	}
}
