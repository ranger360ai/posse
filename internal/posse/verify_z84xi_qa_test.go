package posse

// QA pin written verifying four folded closes under ranger-base-z84xi
// (ranger-base-m8ko, ranger-base-92rt, ranger-base-ch6re, ranger-base-6889).

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ─── ranger-base-92rt, carried into ranger-base-92n5p ────────────────────────

// ranger-base-92rt was closed "folded into ranger-base-92n5p: same class, the
// launch path opens a non-regular file and wedges (parity.go + gates.go)".
// The fold is honest — both are one read of a non-regular file with no type
// check — but 92n5p's own pin
// (TestQAAFifoAtTheDispatchPathMustNotWedgeTheLaunch) reaches only the gates.go
// half: probeL3Hooks, installHook, hookInstalled, InstallPrePushHook. Nothing
// in the tree reaches parity.go, and 92rt's id appears in no file at all, so a
// fix that lands the hooks half and un-skips that pin goes green with 92rt's
// half still open. This is the arm that refuses to.
//
// MECHANISM (parity.go): projectConfigTrustFile Lstats the path — which
// catches a dangling symlink and is why a directory reads back EISDIR — and
// then calls os.ReadFile on it. open(2) on a FIFO with no writer blocks
// indefinitely, so planLaunch never returns and nothing is printed. ADR 0002
// amendment 2026-08-26 §4 says an existing file the launch cannot prove safe
// DEGRADES; this one stops the launch instead.
//
// Un-skip when ranger-base-92n5p lands. Both controls run before the FIFO
// arms and use the same call, so a blocked verdict is the file type and not
// the harness.
func TestQAAFifoAtTheProjectConfigPathMustNotWedgeTheLaunch(t *testing.T) {
	t.Skip("ranger-base-92n5p (folded ranger-base-92rt): projectConfigTrustFile reads .claude/settings.json before any type check — the launch blocks forever on a FIFO")

	claude := &Runtime{Name: "claude",
		ProjectConfig:     []string{ClaudeProjectConfig, ClaudeProjectConfigLocal},
		ProjectConfigKeys: []string{"hooks", "mcpServers"}}

	// CONTROL 1: an empty .claude dir is clean, and returns. If this blocks,
	// the rig is broken and every verdict below is meaningless.
	empty := projectConfigDir(t)
	var why string
	if !returnsWithin(t, 10*time.Second, func() { why = ProjectConfigTrust(claude, nil, empty) }) {
		t.Fatalf("CONTROL: ProjectConfigTrust blocked with no settings file planted — the rig, not the code")
	}
	if why != "" {
		t.Fatalf("CONTROL: a directory with no settings file is clean: %q", why)
	}

	// CONTROL 2: a REGULAR unparseable file reaches the classifier and comes
	// back fast, so the arms below measure the file type and not the read.
	broken := projectConfigDir(t)
	if err := os.WriteFile(filepath.Join(broken, ClaudeProjectConfig), []byte("{nope"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if !returnsWithin(t, 10*time.Second, func() { why = ProjectConfigTrust(claude, nil, broken) }) {
		t.Fatalf("CONTROL: ProjectConfigTrust blocked on an ordinary malformed file — the rig, not the code")
	}
	if !strings.Contains(why, "project config classification failed") {
		t.Fatalf("CONTROL: a malformed settings.json must fail closed: %q", why)
	}

	// THE DEFECT, at both members of the runtime's project scope: a FIFO is an
	// existing file that cannot be proved a readable top-level JSON object, so
	// it must classify as unreadable — the same shape the directory arm gives.
	for _, rel := range []string{ClaudeProjectConfig, ClaudeProjectConfigLocal} {
		for _, mode := range []uint32{0o644, 0o755} {
			dir := projectConfigDir(t)
			p := filepath.Join(dir, rel)
			if err := syscall.Mkfifo(p, mode); err != nil {
				t.Fatalf("mkfifo %s: %v", p, err)
			}
			var got string
			if !returnsWithin(t, 10*time.Second, func() { got = ProjectConfigTrust(claude, nil, dir) }) {
				t.Errorf("ProjectConfigTrust blocked on a %04o FIFO at %s — the launch hangs instead of failing closed", mode, rel)
				continue
			}
			if !strings.Contains(got, "project config classification failed") {
				t.Errorf("a %04o FIFO at %s must fail closed, not read as clean: %q", mode, rel, got)
			}
		}
	}
}

// projectConfigDir is a scratch session dir with the runtime's config
// directory already made, so a planted file is the only variable.
func projectConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return dir
}
