//go:build posse_arm2

package posse

// A session cannot relaunch ITSELF, on either arm (ranger-base-521). The
// landing turn waits for the target's agent to go idle, and when the caller
// is the target it is working precisely because it is running the command
// that waits. --no-land skips the wait and reaches the kill, which ends the
// processes in the workspace's panes — the caller included, measured inside
// the close call (scripts/verify-self-close.sh, ranger-base-hslbb /
// ranger-base-176sd) — so it would destroy the session between the kill and
// the recreate.
//
// The discriminator is the workspace id herdr injects into every pane it
// opens, read against the session meta's own — see CallerRunsInside. These
// tests set that env by hand, which is what keeps them off t.Parallel.
//
// Every arm has its opposite here on purpose: a check that says "self" to
// everything would refuse every relaunch in the shop, and one that says it
// to nothing is the timeout this bead is about.

import (
	"strings"
	"testing"
)

// selfEnv makes this process look like it is running inside the session
// `name`: the pane env herdr would have injected, aimed at the same herdr
// server the meta records.
func selfEnv(t *testing.T, b *HerdrBackend, name string) *HerdrMeta {
	t.Helper()
	m, ok := b.readMeta(name)
	if !ok {
		t.Fatalf("no meta for %s", name)
	}
	if m.Workspace == "" || m.Socket == "" {
		t.Fatalf("meta for %s records workspace %q socket %q — the self case cannot be spelled without both", name, m.Workspace, m.Socket)
	}
	t.Setenv(EnvWorkspace, m.Workspace)
	t.Setenv("HERDR_SOCKET_PATH", m.Socket)
	return m
}

func TestRelaunchRefusesFromInsideTheSessionItself(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	m := selfEnv(t, b, "s1")

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("a session relaunching itself must be refused:\n%s", out.String())
	}
	for _, want := range []string{"cannot relaunch itself", m.Workspace, "--no-land"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal missing %q:\n%v", want, err)
		}
	}
	// Refused before anything is spent: no landing turn was waited on, and
	// nothing was closed. The whole complaint was a bounded wait that could
	// only end one way.
	log := calls(t, fake)
	for _, never := range []string{"agent wait", "agent prompt", "workspace close"} {
		if strings.Contains(log, never) {
			t.Errorf("a refused self-relaunch must not %q:\n%s", never, log)
		}
	}
	after, ok := b.readMeta("s1")
	if !ok || after.Workspace != m.Workspace {
		t.Errorf("the session must be left exactly as it stands, got %+v", after)
	}
}

// The arm that must never be a way through. --no-land skips the landing turn
// and reaches the kill, and the kill takes the caller with the workspace, so
// the refusal covers it too — a relaunch that cannot reach its own recreate
// destroys the session it was asked to refresh.
func TestRelaunchFromInsideItselfIsRefusedWithNoLandToo(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	m := selfEnv(t, b, "s1")

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1", NoLand: true})
	if err == nil {
		t.Fatalf("--no-land from inside the session must be refused:\n%s", out.String())
	}
	for _, want := range []string{"cannot relaunch itself", m.Workspace, "--no-land skips the wait"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal missing %q:\n%v", want, err)
		}
	}
	// And it is refused with the session still standing: the kill is the
	// step that would end the caller, so nothing may reach it.
	if log := calls(t, fake); strings.Contains(log, "workspace close") {
		t.Errorf("nothing may be closed by a refused self-relaunch:\n%s", log)
	}
	if after, ok := b.readMeta("s1"); !ok || after.Workspace != m.Workspace {
		t.Errorf("the session must be left exactly as it stands, got %+v", after)
	}
}

// The refusal is one refusal, but each arm has to say what happens on ITS
// side: a reader who typed --no-land is not told about a wait, and a reader
// who did not is not left thinking the flag is the way out.
func TestSelfRelaunchRefusalNamesBothHalves(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	selfEnv(t, b, "s1")

	for _, tc := range []struct {
		name string
		o    RelaunchOpts
	}{
		{"landing", RelaunchOpts{Name: "s1"}},
		{"--no-land", RelaunchOpts{Name: "s1", NoLand: true}},
	} {
		var out strings.Builder
		err := b.RelaunchSession(&out, tc.o)
		if err == nil {
			t.Fatalf("%s: expected a refusal", tc.name)
		}
		for _, want := range []string{
			"no --timeout ever settles",
			"ends the processes in that workspace's panes",
			"relaunch it from another session",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: refusal missing %q:\n%v", tc.name, want, err)
			}
		}
	}
}

// The other side of the same check, and the one that decides whether this is
// a fix or a new way to refuse every refresh in the shop: a caller sitting in
// SOME OTHER session's workspace relaunches this one exactly as before.
func TestRelaunchFromAnotherSessionIsNotTheSelfCase(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	m := selfEnv(t, b, "s1")
	t.Setenv(EnvWorkspace, m.Workspace+"9")

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("relaunch from another session: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "landing s1") {
		t.Errorf("the landing turn must still run for a caller outside the session:\n%s", out.String())
	}
	if strings.Contains(out.String(), "inside its own workspace") {
		t.Errorf("a caller outside the session must not be told it is inside it:\n%s", out.String())
	}
}

// A workspace id is unique per herdr SERVER and nothing more, so the same
// string names a different workspace on a named or scratch server. The id
// alone must not decide.
func TestRelaunchSelfCaseNeedsTheSameHerdrServer(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	selfEnv(t, b, "s1")
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/some-other-server/herdr.sock")

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("a matching id on another server is not the self case: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "landing s1") {
		t.Errorf("the landing turn must still run:\n%s", out.String())
	}
}

// The predicate's own edges, where a wrong yes would refuse a relaunch on a
// comparison this pass cannot make.
func TestCallerRunsInsideNeedsBothHalvesRecorded(t *testing.T) {
	t.Setenv(EnvWorkspace, "w7")
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qa-server/herdr.sock")
	sock := SocketID()
	for _, tc := range []struct {
		name string
		m    *HerdrMeta
		want bool
	}{
		{"same workspace, same server", &HerdrMeta{Workspace: "w7", Socket: sock}, true},
		{"another workspace", &HerdrMeta{Workspace: "w8", Socket: sock}, false},
		{"another server", &HerdrMeta{Workspace: "w7", Socket: "/tmp/qa-other/herdr.sock"}, false},
		{"server not recorded", &HerdrMeta{Workspace: "w7"}, false},
		{"workspace not recorded", &HerdrMeta{Socket: sock}, false},
		{"no meta at all", nil, false},
	} {
		if got := CallerRunsInside(tc.m); got != tc.want {
			t.Errorf("%s: CallerRunsInside = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// And with no pane env at all — the operator's own terminal, where every
// relaunch in this shop's history has been typed.
func TestCallerRunsInsideIsFalseOutsideAPane(t *testing.T) {
	t.Setenv(EnvWorkspace, "")
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qa-server/herdr.sock")
	if CallerRunsInside(&HerdrMeta{Workspace: "w7", Socket: SocketID()}) {
		t.Error("a caller with no workspace in its env is not inside any session")
	}
	// And the pair of blanks that would otherwise compare equal: a meta that
	// records no workspace, read by a process that is in none.
	if CallerRunsInside(&HerdrMeta{Socket: SocketID()}) {
		t.Error("two unrecorded workspaces are not the same workspace")
	}
}

// The unnameable server: with no home to resolve one from, SocketID answers
// "" and nothing is proven ours. The id may match; the answer is still no,
// because a refusal nobody can argue with had better be one this pass can
// prove.
func TestCallerRunsInsideRefusesAnUnnameableServer(t *testing.T) {
	t.Setenv(EnvWorkspace, "w7")
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_SESSION", "")
	t.Setenv("HOME", "")
	if got := SocketID(); got != "" {
		t.Fatalf("this arm needs an unnameable socket, got %q", got)
	}
	if CallerRunsInside(&HerdrMeta{Workspace: "w7"}) {
		t.Error("a server neither side can name proves nothing about whose workspace this is")
	}
}
