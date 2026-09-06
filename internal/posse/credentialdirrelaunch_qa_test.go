//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-179hy — the credential-dir refusal on the OTHER launch path.
//
// ranger-base-x5f6p put a refusal on planLaunch: with a seatbelt wall
// rendered from the LAUNCHER's environment, an env set exporting
// CLAUDE_CONFIG_DIR or CLAUDE_SECURESTORAGE_CONFIG_DIR would move the
// runtime's credential write to a directory the wall never heard of, so the
// launch is refused. RelaunchAgent renders a persona line too — it re-reads
// the PID and re-reads the env sets BY NAME, precisely so a set edited after
// the session opened is seen — and it had no such refusal.
//
// Which makes the relaunch the WORSE of the two sites, not the lesser one:
// it is dispatch.launchSession's path, the unattended one, where a crashed
// CLI comes back with nobody watching. The sequence below is the whole
// defect, run for real:
//
//	launch a seatbelt persona with a clean env set  → allowed (witness)
//	kill the CLI, relaunch                           → re-types (witness)
//	export CLAUDE_CONFIG_DIR into that env set
//	kill the CLI, relaunch                           → must be REFUSED
//
// The two witness steps are what make the third arm mean anything: without
// them a refusal is indistinguishable from a rig that was never going to
// relaunch at all.
//
// The source-level companions — every renderer carries the sequence, and
// nobody spells the wall predicate twice — are in credentialdenymove_qa_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Serial: it writes AvailableCages, a package var (cmd/testparallel flags
// exactly that), and the seatbelt tier is the whole premise of the arms.
func TestQARelaunchRefusesAnEnvSetThatMovesTheCredentialDirAfterTheLaunch(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	a := b.App

	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	t.Cleanup(func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	})

	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ncage: seatbelt\nenvs: [crew]\ndeny: [Bash(git push:*)]\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(a.EnvsDir, "crew.env")
	if err := os.WriteFile(envFile, []byte("FOO=bar\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := b.CreateSession(NewSessionOpts{Name: "cd", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("the launch with a clean env set must succeed: %v", err)
	}
	// The premise of every arm below. A session that came up at some other
	// tier renders no profile from the launcher's environment, and then the
	// refusal has nothing to be about.
	m, found := b.readMeta("cd")
	if !found {
		t.Fatal("no meta for the session that just launched")
	}
	if m.Cage != CageSeatbelt {
		t.Fatalf("the session came up at cage %q, not %q — NOTHING MEASURED: the credential-dir refusal only exists where posse renders the wall", m.Cage, CageSeatbelt)
	}

	died := func() {
		t.Helper()
		m, found := b.readMeta("cd")
		if !found {
			t.Fatal("no meta for the session being killed")
		}
		m.Launched = time.Now().Add(-time.Hour)
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		os.Remove(filepath.Join(fake, "agents.json")) // the CLI died
	}

	died()
	if ok, err := b.RelaunchAgent("cd", time.Second); err != nil || !ok {
		t.Fatalf("witness: a session whose env set names no config dir must re-type: ok=%v err=%v", ok, err)
	}

	// The operator (or anything else with a write to the envs dir) exports
	// the config dir into the set the meta names — the state the LAUNCH
	// would have refused, reached after the launch, when the profile above
	// has already been written from an environment that never saw it.
	moved := t.TempDir()
	if err := os.WriteFile(envFile, []byte("FOO=bar\nCLAUDE_CONFIG_DIR="+moved+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	died()
	ok, rerr := b.RelaunchAgent("cd", time.Second)
	if rerr == nil {
		t.Fatalf("a relaunch whose env set moves the credential write out from behind the wall must be refused exactly as the launch was: ok=%v (ranger-base-179hy)", ok)
	}
	if !strings.Contains(rerr.Error(), "CLAUDE_CONFIG_DIR") {
		t.Errorf("the relaunch refusal never names the variable that caused it:\n%v", rerr)
	}
	// NAMES only, here as on the launch path: the value is the one thing
	// neither renderer reads.
	if strings.Contains(rerr.Error(), moved) {
		t.Errorf("the relaunch refusal carries the env set's VALUE %q:\n%v", moved, rerr)
	}
	// And it did not type. A refusal that still put the line in the pane
	// would be a message, not a wall.
	if ok {
		t.Error("RelaunchAgent reported it re-typed the session it had just refused")
	}
}
