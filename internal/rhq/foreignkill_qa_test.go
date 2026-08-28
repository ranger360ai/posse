package rhq

import (
	"strings"
	"testing"
)

// rangerhq-selx: the destructive half of the foreign-row fence
// (rangerhq-ynx8 is the launch half). `posse kill <name>` resolves by
// label, and Resolve falls through to workspaces this home holds no meta
// for — so a kill by name closed another instance's live agent, exit 0, no
// warning (qa's M1 rehearsal: instance A's `rhq kill m1-collide` closed
// instance B's workspace). Now it refuses, names the workspace id, and says
// what to type if the operator really means that row.
func TestKillRefusesAForeignWorkspace(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/this/herdr.sock")
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})
	foreignHolder(t, fake, "handmade")

	err := b.KillSession("handmade")
	if err == nil {
		t.Fatal("a foreign workspace was killed by name")
	}
	// The name is what is NOT unique across instances, so the refusal
	// carries the id an operator can take to herdr or to the other home.
	for _, want := range []string{"handmade", "wForeign", "--foreign"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal missing %q: %s", want, err)
		}
	}
	if log := calls(t, fake); strings.Contains(log, "workspace close wForeign") {
		t.Errorf("the foreign workspace was closed anyway:\n%s", log)
	}

	// --force is the reap guard's flag and carries no ownership consent: a
	// foreign row has no meta and never reaches that guard, so a --force
	// typed from habit about one's own dirty tree must not close it.
	if _, err := b.ForceKillSessionAndLand("handmade"); err == nil {
		t.Error("--force alone killed a foreign workspace")
	}
	if log := calls(t, fake); strings.Contains(log, "workspace close wForeign") {
		t.Errorf("--force closed the foreign workspace:\n%s", log)
	}

	// The refusal is aimed, not blanket: this home's own session still dies
	// on the same command.
	if err := b.KillSession("mine"); err != nil {
		t.Fatalf("own session refused: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "workspace close w1") {
		t.Errorf("own session not closed:\n%s", log)
	}
}

// The way through, for the operator who means it — plugin/autostart.sh's
// husk replacement is the scripted caller. It closes the workspace and
// leaves this home's state alone: the meta it has none of is not created,
// and the owning home's record is outside this home to repair.
func TestKillForeignFlagCloses(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/this/herdr.sock")
	b, fake := newTestBackend(t)
	foreignHolder(t, fake, "handmade")

	landing, err := b.KillSessionAndLandOpts("handmade", KillOpts{Foreign: true})
	if err != nil {
		t.Fatalf("--foreign refused: %v", err)
	}
	if landing == nil || landing.Line() != "" {
		t.Errorf("a foreign row has no tree of ours to land: %+v", landing)
	}
	if log := calls(t, fake); !strings.Contains(log, "workspace close wForeign") {
		t.Errorf("--foreign did not close it:\n%s", log)
	}
	if b.HasSession("handmade") {
		t.Error("the workspace survived --foreign")
	}
}

// The board where the two refusals meet (rangerhq-yt1p/9nso): this home has
// a meta, but the workspace it names is missing from the listing while a
// STRANGER's row wears its label. Resolve answers with the stranger, and
// the kill must not close it — the name resolving is not the workspace
// being ours.
func TestKillRefusesAStrangerWearingOurLabel(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/this/herdr.sock")
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})

	// w1 (ours) is gone from herdr; wStranger now carries the label.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "wStranger", Label: "mine", AgentStatus: "idle"}})

	if err := b.KillSession("mine"); err == nil {
		t.Fatal("a stranger wearing our label was killed")
	}
	if log := calls(t, fake); strings.Contains(log, "workspace close wStranger") {
		t.Errorf("the stranger's workspace was closed:\n%s", log)
	}
}
