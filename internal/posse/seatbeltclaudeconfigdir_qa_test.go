//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-ntnst — ADR 0090 D2: on the sandbox-exec
// mechanism, `$HOME/.claude.json` and its atomic-write siblings leave a
// claude seat's write set whenever the line types CLAUDE_CONFIG_DIR. The
// render half, so it runs from any host: renderWallOn takes the mechanism as
// a parameter and the profile is a file the pin reads. The live half needs a
// darwin host.

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// sbcdProfile renders the sandbox-exec wall for the fixture seat and returns
// the profile text that would run.
func sbcdProfile(t *testing.T, f weFixture) string {
	t.Helper()
	w, err := f.a.renderWallOn(WallSandboxExec, f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(w.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// sbcdGrantsHomeFile reports whether the profile grants the home config file
// or its sibling namespace, in either spelling SeatbeltProfile renders: the
// literal `(subpath …)` and the regex-quoted sibling grant.
func sbcdGrantsHomeFile(prof, home string) (subpath, siblings bool) {
	file := filepath.Join(home, claudeConfigFileName)
	return strings.Contains(prof, `(subpath "`+file+`")`), strings.Contains(prof, regexp.QuoteMeta(file))
}

func sbcdPlantHomeFile(t *testing.T, home string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, claudeConfigFileName), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// D2 itself: CLAUDE_CONFIG_DIR typed, so the home file is no seat's to write.
func TestQASeatbeltRenderDropsHomeClaudeJSONWhenConfigDirTyped(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, wallUngrantedDir(t))
	sbcdPlantHomeFile(t, f.home)
	prof := sbcdProfile(t, f)
	sub, sib := sbcdGrantsHomeFile(prof, f.home)
	if sub || sib {
		t.Errorf("with %s=%s on the line the sandbox-exec profile still grants $HOME/%s (subpath=%v, siblings=%v) — the file the operator's own claude reads, with no seat writing it (ADR 0090 D2)",
			ClaudeConfigDirVar, f.cfg, claudeConfigFileName, sub, sib)
	}
	// The seat's own config file still lands: it is inside the config dir.
	if !strings.Contains(prof, `(subpath "`+f.cfg+`")`) {
		t.Errorf("the profile no longer grants the config dir %s — the seat's own %s is in there and its writes would be lost (ranger-base-cypy1)", f.cfg, claudeConfigFileName)
	}
	// The declaration is untouched: the reshape is the mechanism's.
	if !containsString(f.rt.StateDirs, "~/"+claudeConfigFileName) {
		t.Errorf("the runtime's StateDirs lost the file entry (%v) — D2 reshapes the render, not the declaration", f.rt.StateDirs)
	}
}

// Control: unset, the seat writes the home file and the grant must stand.
// Without this arm the pin above would pass on a render that never granted
// the file at all.
func TestQASeatbeltRenderKeepsHomeClaudeJSONWhenConfigDirUnset(t *testing.T) {
	f := weFixtureIn(t, wallUngrantedDir(t))
	unsetenvForTest(t, ClaudeConfigDirVar)
	sbcdPlantHomeFile(t, f.home)
	sub, sib := sbcdGrantsHomeFile(sbcdProfile(t, f), f.home)
	if !sub || !sib {
		t.Errorf("with %s unset the profile must grant $HOME/%s and its siblings (subpath=%v, siblings=%v) — that is the file the seat writes (ranger-base-cypy1)",
			ClaudeConfigDirVar, claudeConfigFileName, sub, sib)
	}
}

// ranger-base-10dh6: CLAUDE_CONFIG_DIR naming a directory OTHER than
// ~/.claude must itself be granted. The declared tree is `~/.claude`; a seat
// whose line resolves the config file to a DIFFERENT directory writes there,
// and without this grant that directory is walled nowhere — the file lost
// silently, in the shape D19 already fixed at bwrap and this mechanism had
// not (on this box the two paths are the same, so nothing was lost here
// before this pin).
func TestQASeatbeltRenderGrantsAConfigDirElsewhereThanHome(t *testing.T) {
	f := weFixtureIn(t, wallUngrantedDir(t))
	elsewhere := filepath.Join(filepath.Dir(f.home), "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ClaudeConfigDirVar, elsewhere)
	prof := sbcdProfile(t, f)
	if !strings.Contains(prof, `(subpath "`+elsewhere+`")`) {
		t.Errorf("the profile does not grant %s — %s names it and the seat writes its config file there, outside every grant without it (ranger-base-10dh6)", elsewhere, ClaudeConfigDirVar)
	}
}

// Every seatbelt reader takes the same list: the MCP-file refusal (ADR 0090
// D1) judges the write set the profile grants, not the declaration.
func TestQASeatbeltMCPWriteSetAgreesWithTheProfile(t *testing.T) {
	f := weFixtureIn(t, wallUngrantedDir(t))
	sbcdPlantHomeFile(t, f.home)
	file := filepath.Join(f.home, claudeConfigFileName)
	writable, siblings, deny, keep, err := f.a.mcpWriteSet(WallSandboxExec, f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if containsString(siblings, file) {
		t.Errorf("mcpWriteSet's siblings %v name %s, which the profile no longer grants", siblings, file)
	}
	if mcpPathWritable(file, writable, siblings, deny, keep) {
		t.Errorf("mcpWriteSet reads %s as writable while the profile does not grant it — the two readers disagree", file)
	}
}

// The pure arms, driven with the environment already read.
func TestQASeatbeltClaudeStateDirsArms(t *testing.T) {
	// No t.Parallel: `~` in the declaration expands against $HOME
	// (ExpandTilde), which is the home the function is handed in production.
	home := absResolve(t.TempDir())
	t.Setenv("HOME", home)
	decl := []string{"~/.claude", "~/" + claudeConfigFileName, "/opt/extra"}
	other := filepath.Join(home, "elsewhere")
	for _, c := range []struct {
		what    string
		dir     string
		inForce bool
		home    string
		want    []string
	}{
		{"not in force: the seat writes the home file", "", false, home, decl},
		{"in force, default location", filepath.Join(home, ".claude"), true, home, []string{"~/.claude", "/opt/extra"}},
		{"in force, elsewhere", other, true, home, []string{other, "~/.claude", "/opt/extra"}},
		// CLAUDE_CONFIG_DIR=$HOME puts the seat's file back at $HOME/.claude.json.
		{"in force at $HOME itself", home, true, home, decl},
		{"no home: nothing to resolve the entry against", other, true, "", decl},
	} {
		got := seatbeltClaudeStateDirs(decl, c.dir, c.inForce, c.home)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.what, got, c.want)
		}
	}
	if !reflect.DeepEqual(decl, []string{"~/.claude", "~/" + claudeConfigFileName, "/opt/extra"}) {
		t.Errorf("the declared slice was mutated: %v", decl)
	}
}

// Only claude's list is reshaped; nil is nil.
func TestQASeatbeltStateDirsLeavesOtherRuntimesAlone(t *testing.T) {
	t.Setenv(ClaudeConfigDirVar, filepath.Join(absResolve(t.TempDir()), "cfg"))
	codex := &Runtime{Name: "codex", StateDirs: []string{"~/.codex", "~/" + claudeConfigFileName}}
	if got := SeatbeltStateDirs(codex); !reflect.DeepEqual(got, codex.StateDirs) {
		t.Errorf("SeatbeltStateDirs(codex) = %v, want the declaration %v", got, codex.StateDirs)
	}
	if got := SeatbeltStateDirs(nil); got != nil {
		t.Errorf("SeatbeltStateDirs(nil) = %v, want nil", got)
	}
}
