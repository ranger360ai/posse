//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-etsk5 (QA, verifying the close of ranger-base-6swlr).
//
// THE GAP, which the close disclosed rather than hid: listSessions withholds
// on four arms, and TestListSessionsCountsWhatItWithheld moves only the
// `kept` one — its fixture trips cannotAnswerFor. Its own comment said so
// ("Drop the strangers or spared term → survives here"). I ran both mutants
// against the whole seat suite and both do survive: the withheld reading can
// lose either arm and every pin stays green.
//
// Why that matters more than a disclosed limit usually does. `spared` is
// not an exotic arm — it is the ORDINARY one. A meta younger than the 5m
// PruneGrace whose workspace is not on the board is spared by construction,
// which is every session launched in the last five minutes, and the close's
// own handoff (ranger-base-5kiu4) records that the arm it measured in the
// wild fired through `spared`, not through a herdr restart. So a future
// edit that dropped that arm would put back the whole ranger-base-6swlr
// defect — reconcileSeats releasing a seat into its own live session, one
// persona on two beads — on the arm most likely to fire, under a green
// suite.
//
// These are the two fixtures that move the other two arms. They assert the
// withheld entry BY NAME rather than by count, so a list that is the right
// length for the wrong reason cannot satisfy them. Each states its
// premise (the arm really did fire, and the session really is absent from
// the listing) so a green cannot come from a fixture that tripped nothing.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// qaYoungMeta writes a session meta stamped `launched` now, which is what
// puts prunable() inside its grace and sends the meta down the `spared`
// arm instead of the proven-dead one.
func qaYoungMeta(t *testing.T, b *HerdrBackend, name, ws, sock string) {
	t.Helper()
	meta := "name: " + name + "\nworkspace: " + ws + "\npane: " + ws + ":p1\nemoji: x\nagent: developer\n" +
		"socket: " + sock + "\nlaunched: " + time.Now().UTC().Format(time.RFC3339Nano) + "\n"
	os.MkdirAll(b.metaDir(), 0o755)
	if err := os.WriteFile(b.metaPath(name), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheWithheldListMovesOnTheSparedArm(t *testing.T) {
	const ours = "/tmp/etsk5/ours.sock"
	t.Setenv("HERDR_SOCKET_PATH", ours)
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn

	// A NON-empty board this server can answer for: emptyBoard and
	// cannotAnswerFor are both out, so the only arm left is `spared`.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "live"}})
	if _, withheld, err := b.listSessions(); err != nil || len(withheld) != 0 {
		t.Fatalf("premise: this board withholds nothing to start with: withheld=%v err=%v", withheld, err)
	}

	// Ours, young, and its workspace is not on the board — the shape of
	// every session launched inside the last five minutes.
	qaYoungMeta(t, b, "young", "w404", ours)
	sess, withheld, err := b.listSessions()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "not dead") {
		t.Fatalf("premise: this fixture must trip the SPARED arm, not another one: warn=%q", warn.String())
	}
	for _, s := range sess {
		if s.Name == "young" {
			t.Fatalf("premise: the spared session must be absent from the listing — that absence is what reconcileSeats misreads as death")
		}
	}
	if len(withheld) != 1 || withheld[0] != "young" {
		t.Errorf("a SPARED meta is withheld and must be named: withheld=%v warn=%q\n"+
			"dropping the spared arm from listSessions puts back ranger-base-6swlr on its most ordinary arm — "+
			"a seat released into its own live session while the meta sits inside the %s prune grace", withheld, warn.String(), PruneGrace)
	}
}

func TestTheWithheldListMovesOnTheStrangersArm(t *testing.T) {
	const ours = "/tmp/etsk5/ours.sock"
	t.Setenv("HERDR_SOCKET_PATH", ours)
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn

	// The workspace the meta names is LIVE, but it is labelled for somebody
	// else: herdr re-issued the id across a restart (rangerhq-6bg7). The
	// session is withheld so its name cannot address a stranger's pane.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "someone-else"}})
	qaYoungMeta(t, b, "recycled", "w1", ours)

	sess, withheld, err := b.listSessions()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "another workspace holds the id they recorded") {
		t.Fatalf("premise: this fixture must trip the STRANGERS arm: warn=%q", warn.String())
	}
	for _, s := range sess {
		if s.Name == "recycled" {
			t.Fatalf("premise: the stranger's meta must be absent from the listing")
		}
	}
	if len(withheld) != 1 || withheld[0] != "recycled" {
		t.Errorf("a STRANGER meta is withheld and must be named: withheld=%v warn=%q", withheld, warn.String())
	}
}
