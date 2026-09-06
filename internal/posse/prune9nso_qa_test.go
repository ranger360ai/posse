//go:build posse_arm3

package posse

// QA suite for the rangerhq-9nso prune guards (verifying the close of
// rangerhq-9nso, ADR 0011 §2): the paths the fix's own tests do not walk.
//
// TestHerdrSessionsPruneMustProveDeath covers both guards together, but it
// is carried entirely by guard (d) — stub (a) out and all six of its
// subtests still pass, so nothing on disk fails if the grace is deleted or
// if `launched:` stops parsing. And its fake answers `workspace get` the way
// the fake answers everything by default: envelope on stdout. Real herdr
// 0.8.0 puts it on stderr with exit 1 (probed live 2026-08-20; the
// rangerhq-gnd shape). Both gaps are pinned here.
//
// Tests marked t.Skip pin a filed bug: they encode the expected behavior
// and fail today. Remove the skip when the bead closes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// qa9nsoSetup builds the incident's board: the racing pass's own session
// (so the listing is never empty, keeping the rangerhq-8fq guards quiet)
// plus a session another pass created, whose workspace is about to be gone
// from this pass's view. Returns the backend, the fake dir and the second
// session's workspace id.
func qa9nsoSetup(t *testing.T) (*HerdrBackend, string, string) {
	t.Helper()
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qa9nso/herdr.sock")
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})
	mustCreate(t, b, NewSessionOpts{Name: "newborn", Cmd: "claude"})
	m, ok := b.readMeta("newborn")
	if !ok {
		t.Fatal("no meta for newborn")
	}
	return b, fake, m.Workspace
}

// qa9nsoLaunched rewrites just the launched: line of a meta, as a string —
// the point is what the *file* can say, including things writeMeta would
// never produce (a clock ahead of ours, a value from another writer, a
// field that was never recorded).
func qa9nsoLaunched(t *testing.T, b *HerdrBackend, name, launched string) {
	t.Helper()
	p := b.metaPath(name)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, ln := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(ln, "launched:") {
			continue
		}
		if ln != "" {
			out = append(out, ln)
		}
	}
	if launched != "" {
		out = append(out, "launched: "+launched)
	}
	if err := os.WriteFile(p, []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── guard (a): the grace, on its own ────────────────────────────────────────

// The fix's suite never fails when PruneGrace is removed: every subtest that
// exercises a young meta also has a live workspace behind it, so guard (d)
// answers first. This is the case where only the grace can save the file —
// herdr says, truthfully, that the workspace is gone, and the meta is still
// younger than the race window that cost two sessions their identity.
// ADR 0011 §2 makes the grace unconditional for exactly that reason: within
// PruneGrace, "not there" is never read as "died". The file costs one more
// pass's patience; a wrong delete costs the session.
func TestPruneGraceKeepsAFreshMetaEvenWhenHerdrSaysGone(t *testing.T) {
	b, fake, _ := qa9nsoSetup(t)
	// newborn's workspace really is gone from this server, listing and all.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})

	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("newborn"); !ok {
		t.Fatal("a meta younger than PruneGrace was pruned (ADR 0011 §2 guard (a) — the grace is unconditional)")
	}

	// ...and the grace is a grace, not an amnesty: past it, the same board
	// prunes. Without this half the test above would also pass on a
	// prunable() that never returns true.
	m, _ := b.readMeta("newborn")
	m.Launched = time.Now().Add(-2 * PruneGrace)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("newborn"); ok {
		t.Error("past the grace, a workspace this server confirms is gone was still not pruned")
	}
}

// launched: is read back with time.Parse(RFC3339) and written with
// RFC3339Nano. A meta whose timestamp does not round-trip is a meta the
// grace cannot see: parseLaunched hands back the zero time, which reads as
// "old enough for anything" and drops guard (a) silently. Every live meta on
// this machine carries a fractional-second launched: today.
func TestLaunchedSurvivesTheMetaRoundTrip(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qa9nso/herdr.sock")
	b, _ := newTestBackend(t)
	want := time.Now().Add(-90 * time.Second).Round(0)
	if err := b.writeMeta(&HerdrMeta{Name: "rt", Workspace: "w9", Launched: want}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta("rt")
	if !ok {
		t.Fatal("no meta after writeMeta")
	}
	if m.Launched.IsZero() {
		raw, _ := os.ReadFile(b.metaPath("rt"))
		t.Fatalf("launched: did not survive the round trip — the grace is blind to every meta on disk:\n%s", raw)
	}
	if d := m.Launched.Sub(want); d > time.Second || d < -time.Second {
		t.Errorf("launched: came back %v off (%v vs %v)", d, m.Launched, want)
	}
	if time.Since(m.Launched) >= PruneGrace {
		t.Errorf("a meta launched 90s ago reads as %v old — outside the grace", time.Since(m.Launched))
	}
}

// What the grace does with values writeMeta would never produce. Only one
// answer is ever acceptable when the field cannot be trusted: keep the file.
// (rangerhq-y4z: check both directions of a comparison. time.Since is
// signed, so a clock ahead of ours reads as a negative age, which is inside
// any grace — the safe side, and worth pinning so it stays there.)
func TestPruneGraceOnHostileLaunchedValues(t *testing.T) {
	cases := []struct {
		name     string
		launched string
		keep     bool
		why      string
	}{
		{"future clock", time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano), true,
			"a negative age is inside the grace — a meta from a clock ahead of ours is not old"},
		{"whole seconds", time.Now().UTC().Format(time.RFC3339), true,
			"RFC3339Nano drops a zero fraction, so a whole-second stamp is what most metas carry"},
		{"trailing comment", time.Now().UTC().Format(time.RFC3339Nano) + " # backfilled", true,
			"yamlClean strips ' #' comments; the timestamp in front of one is still a timestamp"},
		{"local offset", time.Now().Add(-30 * time.Second).Format(time.RFC3339Nano), true,
			"an offset other than Z is still RFC3339 and still names an instant inside the grace"},
		// The rest cannot be read as a time at all: parseLaunched yields the
		// zero value, the grace abstains, and guard (d) is what is left.
		{"malformed", "not-a-time", false, "unreadable: falls through to the per-id query"},
		{"absent", "", false, "never recorded: falls through to the per-id query"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, fake, _ := qa9nsoSetup(t)
			qa9nsoLaunched(t, b, "newborn", tc.launched)
			saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}}) // workspace really gone

			if _, err := b.Sessions(); err != nil {
				t.Fatal(err)
			}
			_, kept := b.readMeta("newborn")
			if kept != tc.keep {
				t.Errorf("launched: %q → kept=%v, want %v (%s)", tc.launched, kept, tc.keep, tc.why)
			}
		})
	}
}

// ─── guard (d): the shape real herdr actually speaks ─────────────────────────

// Probed live against herdr 0.8.0 on 2026-08-20:
//
//	$ herdr workspace get w404
//	exit=1  stdout empty
//	stderr {"error":{"code":"workspace_not_found",...},"id":"cli:workspace:get"}
//
// The fake's default is the envelope on stdout, which is how rangerhq-gnd
// went green in tests while production stayed broken. Guard (d) reads that
// error code to decide a delete, so it is pinned in the shape the fleet
// meets: death still has to be provable when the proof arrives on stderr,
// and a *different* code on stderr must still keep the file.
func TestPruneProvesDeathThroughTheStderrEnvelope(t *testing.T) {
	t.Run("workspace_not_found on stderr still prunes", func(t *testing.T) {
		b, fake, _ := qa9nsoSetup(t)
		m, _ := b.readMeta("newborn")
		m.Launched = time.Now().Add(-2 * PruneGrace)
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})
		if err := os.WriteFile(filepath.Join(fake, "error-on-stderr"), nil, 0o644); err != nil {
			t.Fatal(err)
		}

		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if _, ok := b.readMeta("newborn"); ok {
			t.Error("herdr proved the workspace gone on stderr and the meta was kept: " +
				"guard (d) reads only the stdout envelope, so the prune never fires against real herdr")
		}
	})

	// The other direction of the same read: any code that is not
	// workspace_not_found is not death, whichever stream carries it.
	t.Run("another error code on stderr keeps the meta", func(t *testing.T) {
		b, fake, _ := qa9nsoSetup(t)
		m, _ := b.readMeta("newborn")
		m.Launched = time.Now().Add(-2 * PruneGrace)
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})
		os.WriteFile(filepath.Join(fake, "error-on-stderr"), nil, 0o644)
		os.WriteFile(filepath.Join(fake, "workspace-get-unreachable"), nil, 0o644) // → code "timeout"

		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if _, ok := b.readMeta("newborn"); !ok {
			t.Error("a timeout on stderr was read as death — only workspace_not_found is evidence")
		}
	})
}

// WorkspaceAlive's three answers, at the seam where they are decided. The
// caller turns two of them into "keep the file" and only one into a delete,
// so a bad response must never be able to come back as (false, nil).
func TestWorkspaceAliveKeepsItsThreeAnswersApart(t *testing.T) {
	b, fake, ws := qa9nsoSetup(t)

	if alive, err := b.H.WorkspaceAlive(ws); err != nil || !alive {
		t.Errorf("a live workspace: alive=%v err=%v, want true/nil", alive, err)
	}
	if alive, err := b.H.WorkspaceAlive("w404"); err != nil || alive {
		t.Errorf("a workspace herdr does not hold: alive=%v err=%v, want false/nil", alive, err)
	}
	if alive, err := b.H.WorkspaceAlive(""); err == nil || alive {
		t.Errorf("no workspace id: alive=%v err=%v, want an error — nothing was asked", alive, err)
	}
	os.WriteFile(filepath.Join(fake, "workspace-get-unreachable"), nil, 0o644)
	if alive, err := b.H.WorkspaceAlive(ws); err == nil || alive {
		t.Errorf("a server that did not answer: alive=%v err=%v, want an error — silence is not death", alive, err)
	}
}

// ─── the other half of the same snapshot ─────────────────────────────────────

// rangerhq-9nso hardened the destructive *delete*: absence from a listing
// snapshot is no longer read as death. The destructive *write* still reads
// it that way. A spared meta is deliberately left out of that pass's
// listing (the closer's own note), so HasSession says no, and CreateSession
// creates a second workspace under the same label and overwrites the meta —
// the live workspace, agent running, is orphaned: nothing on disk names it,
// so posse can no longer address it by name and the pane a prompting pass
// needs is gone. That is the incident's harm, reached by the write instead
// of the delete, and writeMeta over the only record of a live session is as
// unrecoverable as os.Remove.
//
// The launch lock (rangerhq-tzdf, ADR 0011 §1) closes it for two dispatch
// passes, which serialize and re-read a fresh listing inside the lock. It
// does not cover an operator's `posse new` racing a pass, which takes no such
// lock. The guard this pins is the 9nso rule applied to the write: a meta
// naming a workspace herdr confirms alive is not free real estate.
func TestCreateDoesNotOrphanALiveWorkspaceBehindAStaleListing(t *testing.T) {
	b, fake, ws := qa9nsoSetup(t)
	// pass B's listing was taken before newborn's workspace existed
	if err := os.WriteFile(filepath.Join(fake, "hidden-from-list"), []byte(ws), 0o644); err != nil {
		t.Fatal(err)
	}
	if b.HasSession("newborn") {
		t.Fatal("setup: the session was supposed to be invisible to this pass")
	}

	err := b.CreateSession(NewSessionOpts{Name: "newborn", Cmd: "claude", Dir: t.TempDir()})

	m, ok := b.readMeta("newborn")
	alive, aerr := b.H.WorkspaceAlive(ws)
	if aerr != nil {
		t.Fatal(aerr)
	}
	if !alive {
		return // the workspace really did die; nothing was orphaned
	}
	if err == nil && ok && m.Workspace != ws {
		t.Fatalf("created %s over a live session: the meta now names %s and nothing names %s, "+
			"which is still running an agent (rangerhq-9nso, by the write path)", m.Name, m.Workspace, ws)
	}
}
