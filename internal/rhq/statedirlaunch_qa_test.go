package rhq

// ranger-base-stbt (verify of rangerhq-tr8k): ADR 0012 D4's `state_dir:` is
// only worth anything if the dir reaches the profile a LAUNCH renders, and
// nothing held that hop.
//
// TestStateDirJoinsTheSeatbeltWritableSet calls SeatbeltWritable directly, so
// it pins the resolver and not the wiring. Measured: dropping `rt.StateDirs...`
// from BOTH RenderSeatbelt call sites (planLaunch and RelaunchAgent) leaves the
// whole internal/rhq package green — the ranger-base-unzn shape, an arm nothing
// holds. It stays invisible for the three built-ins, whose dirs come from the
// builtinRuntimes union inside SeatbeltWritable either way; it bites exactly the
// third-party CLI that D4 exists for, and it bites the way seatbelt.go warns
// about, as "a first-run flow that never sticks" rather than as a denial.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQADeclaredStateDirReachesTheProfileALaunchRenders(t *testing.T) {
	wtqaHome(t)
	seatbeltForTest(t)
	b, _ := newTestBackend(t)
	a := b.App

	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"),
		[]byte("command: mycli {file}\nstate_dir: ~/.mycli-stbt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndescription: t\nruntime: mycli\ncage: seatbelt\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("planLaunch: %v", err)
	}
	prof, err := os.ReadFile(filepath.Join(a.GatesDir("ranger"), "seatbelt.sb"))
	if err != nil {
		t.Fatalf("the launch rendered no seatbelt profile: %v", err)
	}

	// The witness that this profile is real and the assertion below can fail:
	// a built-in's dir is granted by the union whatever the launching runtime
	// declared, so its absence means we are reading a profile that was never
	// built rather than one that dropped the declaration.
	if !strings.Contains(string(prof), ExpandTilde("~/.claude")) {
		t.Fatalf("rendered profile does not carry even the built-in state dirs — "+
			"this test is measuring the wrong file:\n%s", prof)
	}
	if !strings.Contains(string(prof), ExpandTilde("~/.mycli-stbt")) {
		t.Errorf("the launching runtime declared state_dir: ~/.mycli-stbt and the profile "+
			"the launch actually rendered does not grant it — a CLI caged here gets a "+
			"read-only config dir and reports it as a first-run flow that never sticks:\n%s", prof)
	}
}

// The other half of the same wiring: a runtime that declares nothing must not
// pick the dir up, or the assertion above would pass against a profile that
// grants it for some unrelated reason.
func TestQAUndeclaredStateDirIsNotGrantedByALaunch(t *testing.T) {
	wtqaHome(t)
	seatbeltForTest(t)
	b, _ := newTestBackend(t)
	a := b.App

	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "plaincli.yaml"),
		[]byte("command: plaincli {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndescription: t\nruntime: plaincli\ncage: seatbelt\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("planLaunch: %v", err)
	}
	prof, err := os.ReadFile(filepath.Join(a.GatesDir("ranger"), "seatbelt.sb"))
	if err != nil {
		t.Fatalf("the launch rendered no seatbelt profile: %v", err)
	}
	if strings.Contains(string(prof), ExpandTilde("~/.mycli-stbt")) {
		t.Errorf("a runtime that declared no state_dir was granted one:\n%s", prof)
	}
}
