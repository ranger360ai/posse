//go:build posse_arm2

package posse

// ranger-base-stbt (verify of rangerhq-tr8k): ADR 0012 D4's `state_dir:` is
// only worth anything if the dir reaches the profile a LAUNCH renders, and
// nothing held that hop.
//
// TestStateDirJoinsTheSeatbeltWritableSet calls SeatbeltWritable directly, so
// it pins the resolver and not the wiring. Measured: dropping `rt.StateDirs...`
// from BOTH RenderSeatbelt call sites (planLaunch and RelaunchAgent) leaves the
// whole internal/rhq package green — the ranger-base-unzn shape, an arm nothing
// holds. It stayed invisible for the three built-ins, whose dirs came from the
// builtinRuntimes union inside SeatbeltWritable either way; it bit exactly the
// third-party CLI that D4 exists for, and it bit the way seatbelt.go warns
// about, as "a first-run flow that never sticks" rather than as a denial.
//
// AMENDED ranger-base-temhf: the hop these two hold is "into the wall a
// LAUNCH renders", and since ADR 0080 D18 that wall is one of two artifacts —
// darwin's `seatbelt.sb` or Linux's `bwrap.args` (RenderWall, wall.go). Both
// read `seatbelt.sb` by name, so on ci.yml's ubuntu arms they opened a file
// no launch there writes and failed as "the launch rendered no profile". They
// ask this host for its artifact now (wallWrapForTest). The ASSERTIONS are
// unchanged and need no arm of their own: a grant is a path in both
// renderings — a `(subpath "…")` line on one, a `--bind` argv pair on the
// other — so the declaration reaching the wall and its absence are both
// readable as text either way. What stays darwin's is the SBPL spelling of a
// grant, which is seatbeltstatedirnarrow_qa_test.go's subject and not this
// file's.
//
// AMENDED ranger-base-9fl: there is no union any more — the writable set is
// the launching runtime's declaration and nothing else, so that same drop now
// strands claude, codex and grok too. The witness below moved for the same
// reason: it asked for ~/.claude in a `mycli` launch's profile, which the fix
// removes from the grant — and which, on darwin, the ADR 0019 credential
// read-deny literal (`~/.claude/.credentials.json`) answers as a substring
// anyway, so it was witnessing the deny block rather than the grant. It now
// names the persona's own gates dir, which is in the set under both.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQADeclaredStateDirReachesTheProfileALaunchRenders(t *testing.T) {
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
	_, artifact := wallWrapForTest(t, a.GatesDir("ranger"))
	prof, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("the launch rendered no %s wall at %s: %v", WallMechanism(), artifact, err)
	}

	// The witness that this wall is real and the assertion below can fail:
	// the persona's own gates dir is granted at every launch, whatever the
	// runtime — so its absence means we are reading a profile that was never
	// built rather than one that dropped the declaration.
	if !strings.Contains(string(prof), a.GatesDir("ranger")) {
		t.Fatalf("the rendered wall does not carry even this persona's gates dir — "+
			"this test is measuring the wrong file:\n%s", prof)
	}
	if !strings.Contains(string(prof), ExpandTilde("~/.mycli-stbt")) {
		t.Errorf("the launching runtime declared state_dir: ~/.mycli-stbt and the wall "+
			"the launch actually rendered does not grant it — a CLI caged here gets a "+
			"read-only config dir and reports it as a first-run flow that never sticks:\n%s", prof)
	}
}

// The other half of the same wiring: a runtime that declares nothing must not
// pick the dir up, or the assertion above would pass against a profile that
// grants it for some unrelated reason.
func TestQAUndeclaredStateDirIsNotGrantedByALaunch(t *testing.T) {
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
	_, artifact := wallWrapForTest(t, a.GatesDir("ranger"))
	prof, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("the launch rendered no %s wall at %s: %v", WallMechanism(), artifact, err)
	}
	if strings.Contains(string(prof), ExpandTilde("~/.mycli-stbt")) {
		t.Errorf("a runtime that declared no state_dir was granted one:\n%s", prof)
	}
}
