//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-q9bqu: a seat may not connect(2) to a host unix socket.
//
// Arm 1 (ranger-base-qp1hm), named rather than left untagged: the fixture it
// shares with the rest of the wall's render pins (wrcFixtureIn) is arm 1's,
// so an untagged file here compiles in all three arms and breaks two.
//
// The write wall is not the socket wall. A read-only carve over a directory
// of sockets, and a write deny over a socket's own name, say nothing about
// connecting to one: in SBPL a connect to a pathname unix socket is
// `network-outbound`, a different operation class from `file-read*` and
// `file-write*`, and the profile's opening `(allow default)` allows it. So a
// seat whose grok home is write-denied could still hand its tool calls to an
// uncaged grok leader listening at that home. SeatbeltCarveOut.DenyConnect
// is the rule that takes that back, and grokSocketPaths is the list.
//
// WHAT IS MEASURED HERE AND WHAT IS NOT. The render is measured on every
// host: the four TestQA tests below read the list, the profile text, the
// block's position and the other mechanism's clearing of it, and they need
// no kernel. The BEHAVIOUR is darwin's, and this box is Linux — the two
// TestLive tests read NOTHING MEASURED here through sbSkipUnlessSandboxable
// (ranger-base-xjw9) and measure on the first darwin host that runs them,
// ci.yml's macos-latest leg included.
//
// The one property the rule is worth anything for is ASSUMED, NOT MEASURED:
// that the kernel resolves the path at CONNECT time, so a socket bound after
// the seat launched — or unlinked and re-bound at the same name — is refused
// too. That is the move that beat a launch-time mount on the other
// mechanism, so it is the move this has to survive, and
// TestLiveSeatbeltUnixConnectDenyHoldsAtConnectTime is where it is put. If
// the assumption is wrong, that test reds on darwin and names the row.
//
// EVERY REFUSAL IS RUN TWICE, sbProbe's discipline (seatbeltcarveout_qa_test.go):
// once under the profile with the deny and once under the same profile with
// DenyConnect emptied, and the control arm must CONNECT. And every row runs
// a THIRD arm first — /usr/bin/true under the denying profile — because
// sandbox-exec answers an unparseable profile with exit 1 and a message, the
// same shape as a refused connect. Without that arm, a deny block with a
// syntax error would read as a working wall in the deny arm, pass the
// control arm unchanged, and leave a darwin seat unable to launch at all.

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── the render half: measured on every host ─────────────────────────────────

// The list: both of grok's socket paths under the home GROK_HOME names, and
// nothing for a home that is empty or relative.
func TestQAGrokSocketPathsNamesTheLeaderAndTheDaemonRun(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	h := absResolve(filepath.Join(home, ".grok"))
	want := []string{filepath.Join(h, "leader.sock"), filepath.Join(h, "daemon", "run")}
	got := grokSocketPaths(home)
	if len(got) != len(want) {
		t.Fatalf("grokSocketPaths(%s) = %v, want %v", home, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("grokSocketPaths()[%d] = %s, want %s", i, got[i], want[i])
		}
	}
	if got := grokSocketPaths(""); got != nil {
		t.Errorf("grokSocketPaths(\"\") = %v, want nothing", got)
	}
}

// GROK_HOME moves the answer, exactly as it moves grokEphemeralDirs'.
func TestQAGrokSocketPathsFollowsGrokHome(t *testing.T) {
	// No t.Parallel: sets GROK_HOME.
	home := filepath.Join(t.TempDir(), "home")
	moved := absResolve(filepath.Join(t.TempDir(), "elsewhere", "grok"))
	t.Setenv(GrokHomeVar, moved)
	got := grokSocketPaths(home)
	if len(got) == 0 || !strings.HasPrefix(got[0], moved+string(filepath.Separator)) {
		t.Errorf("GROK_HOME=%s: grokSocketPaths = %v, want paths under the moved home", moved, got)
	}
	// A relative GROK_HOME is not a third case: runtimeHomeOverride
	// absolutizes the operator's value against the launcher's cwd (ADR 0079),
	// so the deny follows the home that every other rule was spelled
	// against, and never a relative path SBPL would not match.
	t.Setenv(GrokHomeVar, "relative/grok")
	rel := grokSocketPaths(home)
	if len(rel) == 0 {
		t.Fatal("a relative GROK_HOME resolved no path at all")
	}
	for _, p := range rel {
		if !filepath.IsAbs(p) {
			t.Errorf("grokSocketPaths rendered the relative path %s; SBPL matches absolute paths only", p)
		}
	}
	// A relative HOME with no override: nothing to spell, so nothing denied.
	unsetenvForTest(t, GrokHomeVar)
	if got := grokSocketPaths("home"); got != nil {
		t.Errorf("a relative home must deny nothing, got %v", got)
	}
}

// The carve-out carries the connect deny for EVERY seat, grok or not, and
// that is the point rather than an accident: seatbelt mediates operations
// over the whole filesystem and not a mount view, so a codex or claude seat
// with no grok grant can resolve a grok home's sockets just as well. A deny
// keyed on this launch's grant would be open for every seat but one.
func TestQATheConnectDenyIsOnEverySeatWhateverItsRuntime(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	want := grokSocketPaths(f.home)
	if len(want) == 0 {
		t.Fatalf("the fixture's home resolves no grok socket path, so nothing here is measured: %s", f.home)
	}
	for _, name := range []string{"grok", "codex", "claude"} {
		rt := wrcRuntime(name)
		w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, rt.StateDirs...)
		c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, rt.StateDirs...)
		for _, p := range want {
			if !containsString(c.DenyConnect, p) {
				t.Errorf("a %s seat's carve-out does not deny a connect to %s (ranger-base-q9bqu): DenyConnect=%v", name, p, c.DenyConnect)
			}
		}
	}
}

// And the operator reads it: `posse gates <persona>` prints every carve-out
// class under the grant set it takes back from, because a deny only the
// profile knows about is a deny nobody checks (SeatbeltReport's own rule).
// Under its own marker, since `x` and `r` would say a read or a write was
// walled and what is walled is a connect.
func TestQATheSeatbeltReportNamesTheConnectDeny(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	rt := wrcRuntime("grok")
	want := grokSocketPaths(f.home)
	if len(want) == 0 {
		t.Fatalf("the fixture resolves no grok socket path, so nothing here is measured: %s", f.home)
	}
	var rep strings.Builder
	if err := f.a.SeatbeltReport(f.ag, f.dir, &rep, rt.StateDirs...); err != nil {
		t.Fatal(err)
	}
	for _, p := range want {
		if line := "c " + AbbrevHome(p) + " (network-outbound deny"; !strings.Contains(rep.String(), line) {
			t.Errorf("the seatbelt report does not name the connect deny over %s:\n%s", p, rep.String())
		}
	}
	// A path can be walled twice and then it is reported twice, which is
	// the operator's answer and not a duplicate: for a GROK seat daemon/run
	// is write-denied as well, so it carries an `x` line beside its `c`
	// one. leader.sock is not — nothing above walls it at all — so its `c`
	// line is the only thing between a seat and an uncaged leader, and a
	// report that printed it as a write deny would be saying the opposite
	// of what is true.
	leader := filepath.Join(absResolve(filepath.Join(f.home, ".grok")), grokLeaderSockName)
	if strings.Contains(rep.String(), "x "+AbbrevHome(leader)+" ") {
		t.Errorf("%s is reported as a write deny, and no write rule walls it:\n%s", leader, rep.String())
	}
}

// The profile text: one `(deny network-outbound …)` block, a
// `(remote unix-socket (subpath …))` filter per path, LAST in the file so no
// allow above it has the last word — and no `(allow network…)` anywhere in a
// seat's profile to reopen it.
func TestQASeatbeltProfileRendersTheConnectDenyLast(t *testing.T) {
	t.Parallel()
	socks := []string{"/var/tmp/h/.grok/leader.sock", "/var/tmp/h/.grok/daemon/run"}
	carve := SeatbeltCarveOut{
		Deny:        []string{"/var/tmp/h/denied"},
		DenyAll:     []string{"/var/tmp/h/.claude/sessions"},
		DenyConnect: socks,
	}
	prof := SeatbeltProfile("developer", []string{"/var/tmp/h/proj"}, nil, carve)
	want := "(deny network-outbound\n"
	for _, p := range socks {
		want += "  (remote unix-socket (subpath " + sbQuote(p) + "))\n"
	}
	want += ")\n"
	if !strings.Contains(prof, want) {
		t.Fatalf("the profile does not deny the connects (ranger-base-q9bqu); want\n%s\ngot\n%s", want, prof)
	}
	if !strings.HasSuffix(prof, want) {
		t.Errorf("the connect deny is not the last block, so a later allow could outvote it:\n%s", prof)
	}
	if strings.Contains(prof, "(allow network") {
		t.Errorf("a seat's profile allows a network operation by name, which can outvote the deny:\n%s", prof)
	}
	// Empty is silence, not an empty block: a `(deny network-outbound)` with
	// no filter denies every outbound connection there is.
	if bare := SeatbeltProfile("developer", []string{"/var/tmp/h/proj"}, nil, SeatbeltCarveOut{}); strings.Contains(bare, "network") {
		t.Errorf("an empty DenyConnect rendered a network rule:\n%s", bare)
	}
}

// The other mechanism never sees it: there the sockets are answered by a
// mount and a wall of its own, and a DenyConnect left in the carve-out would
// be a rule that render cannot spell.
func TestQABwrapRenderClearsTheConnectDeny(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	rt := wrcRuntime("grok")
	// The witness, so a clearing of nothing is not read as a clearing: the
	// seatbelt carve over the SAME fixture carries the paths this one drops.
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, rt.StateDirs...)
	if c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, rt.StateDirs...); len(c.DenyConnect) == 0 {
		t.Fatal("the seatbelt carve over this fixture denies no connect, so there is nothing here for bwrapRender to clear and this pin measures nothing")
	}
	r, err := f.a.bwrapRender(f.ag, rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Carve.DenyConnect) != 0 {
		t.Errorf("bwrapRender left DenyConnect=%v; the mount is that mechanism's answer", r.Carve.DenyConnect)
	}
}

// ─── the execution half: darwin's, and NOTHING MEASURED anywhere else ────────

// sbuNetcat is the client every row below connects with. /bin/sh cannot open
// a unix socket and a planted binary would be a cold first exec under a
// deadline (ADR 0085), so the probe uses the system's netcat and nothing it
// wrote itself. Its `-U` support is never asserted here: the control arm
// asserts it by connecting, which is the only claim that matters.
func sbuNetcat(t *testing.T) string {
	t.Helper()
	nc, err := exec.LookPath("nc")
	if err != nil {
		t.Skip("NOTHING MEASURED: no netcat on this host to connect with: " + err.Error())
	}
	return nc
}

// sbuSockDir is a SHORT directory for a socket. darwin's sun_path is 104
// bytes and a t.TempDir() under /var/folders can spend most of it, so a row
// that would have measured a refusal measures a bind failure instead.
func sbuSockDir(t *testing.T) string {
	t.Helper()
	d := wallUngrantedDir(t)
	if len(d) > 60 {
		t.Skipf("NOTHING MEASURED: %s leaves no room for a unix socket path under 104 bytes", d)
	}
	return d
}

// sbuServe binds p and accepts-and-closes until the test ends. An accept
// that closes at once is what lets a connected netcat exit 0 rather than
// wait for a peer that never speaks.
func sbuServe(t *testing.T, p string) net.Listener {
	t.Helper()
	l, err := net.Listen("unix", p)
	if err != nil {
		t.Fatalf("could not listen on %s, so nothing here is measured: %v", p, err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return l
}

// sbuRun runs `sh -c sh` under profile and reports whether it exited 0.
//
// `ready` is called once the child is started, which is what makes the
// bound-after-launch row mean what it says: the sandbox is already applied
// to that process when ready() creates the socket it then connects to.
func sbuRun(t *testing.T, profile, sh string, ready func()) bool {
	t.Helper()
	// Backstop, for the reason sbRun carries one: a row added later that
	// forgets the top-level gate skips here rather than reporting a refusal
	// the kernel never gave it.
	sbSkipUnlessSandboxable(t)
	cmd := exec.Command("sandbox-exec", "-f", profile, "/bin/sh", "-c", sh)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start the probe: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if ready != nil {
		ready()
	}
	select {
	case err := <-done:
		if err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() > 1 {
			t.Fatalf("the probe %q failed for a reason that is not the sandbox: %v %s", sh, err, out.String())
		}
		t.Logf("probe %q: ok=%v %s", sh, err == nil, strings.TrimSpace(out.String()))
		return err == nil
	case <-time.After(45 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("the probe %q never finished, so it measured neither a connect nor a refusal: %s", sh, out.String())
		return false
	}
}

// sbuConnectScript is the client command, optionally waiting for the socket
// to appear first. The wait is bounded: a row whose socket never arrives
// must fail as a probe that measured nothing, never hang a suite.
func sbuConnectScript(nc, sock, started string) string {
	q := shellQuote(nc) + " -U " + shellQuote(sock) + " </dev/null >/dev/null 2>&1"
	if started == "" {
		return q
	}
	return ": > " + shellQuote(started) + "\n" +
		"i=0\n" +
		"while [ ! -S " + shellQuote(sock) + " ] && [ $i -lt 400 ]; do sleep 0.05; i=$((i+1)); done\n" +
		"exec " + q + "\n"
}

// sbuProfiles renders the denying profile and its control: the same text
// with DenyConnect emptied, so the pair differ by the one block under test.
func sbuProfiles(t *testing.T, persona string, writable []string, carve SeatbeltCarveOut) (walled, control string) {
	t.Helper()
	walled = sbRenderProfile(t, "walled.sb", SeatbeltProfile(persona, writable, nil, carve))
	open := carve
	open.DenyConnect = nil
	control = sbRenderProfile(t, "control.sb", SeatbeltProfile(persona, writable, nil, open))
	return walled, control
}

// sbuParses asserts the denying profile applies and runs an ordinary command.
// sandbox-exec answers an unparseable profile the same way it answers a
// refused connect — exit 1 and a line on stderr — so without this arm a deny
// block with a syntax error reads as a wall that works, while a real darwin
// seat rendered from it could not launch at all.
func sbuParses(t *testing.T, walled string) {
	t.Helper()
	if !sbuRun(t, walled, "exec /usr/bin/true", nil) {
		b, _ := os.ReadFile(walled)
		t.Fatalf("the denying profile does not apply to an ordinary command, so every refusal below it is unattributable — and a seat rendered from it would not launch:\n%s", b)
	}
}

// The property the rule rests on, measured on darwin: the path is resolved
// at CONNECT time. Three rows, each with its control arm — a socket that was
// already there, a socket bound after the sandbox was applied, and a socket
// unlinked and re-bound at the same name, which is the move that beats a
// launch-time mount on the other mechanism.
func TestLiveSeatbeltUnixConnectDenyHoldsAtConnectTime(t *testing.T) {
	t.Parallel()
	sbSkipUnlessSandboxable(t)
	nc := sbuNetcat(t)
	dir := sbuSockDir(t)
	// Hand-written and not posse's render: the subject of this test is the
	// kernel's reading of one rule, so everything else is `(allow default)`.
	// TestLiveSeatbeltProfileRefusesAConnectToGroksLeaderSocket is the row
	// that ties the rendered profile to this reading.
	prof := func(name, sock string, deny bool) string {
		body := "(version 1)\n(allow default)\n"
		if deny {
			body += "(deny network-outbound (remote unix-socket (subpath " + sbQuote(sock) + ")))\n"
		}
		return sbRenderProfile(t, name, body)
	}

	t.Run("bound before the sandbox applied", func(t *testing.T) {
		sock := filepath.Join(dir, "u1.sock")
		sbuServe(t, sock)
		walled := prof("u1-walled.sb", sock, true)
		sbuParses(t, walled)
		if sbuRun(t, walled, sbuConnectScript(nc, sock, ""), nil) {
			t.Error("the deny did not refuse a connect to a socket that was already there (ranger-base-q9bqu)")
		}
		if !sbuRun(t, prof("u1-control.sb", sock, false), sbuConnectScript(nc, sock, ""), nil) {
			t.Fatal("the control arm did not connect, so the row above measured nothing")
		}
	})

	t.Run("bound after the sandbox applied", func(t *testing.T) {
		// THE ROW THIS TEST EXISTS FOR. A launch-time mount cannot cover a
		// name that is not there yet; a path rule read at connect time can.
		arm := func(name string, deny bool) bool {
			sock := filepath.Join(dir, name+".sock")
			started := filepath.Join(dir, name+".started")
			var l net.Listener
			ready := func() {
				for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
					if _, err := os.Stat(started); err == nil {
						break
					}
				}
				if _, err := os.Stat(started); err != nil {
					t.Fatalf("the probe never signalled that it was running under the profile: %v", err)
				}
				l = sbuServe(t, sock)
			}
			ok := sbuRun(t, prof(name+".sb", sock, deny), sbuConnectScript(nc, sock, started), ready)
			if l != nil {
				_ = l.Close()
			}
			return ok
		}
		if arm("u2-walled", true) {
			t.Error("the deny did not refuse a connect to a socket bound AFTER the sandbox applied — the path is not read at connect time, and this rule is not the fix (ranger-base-q9bqu)")
		}
		if !arm("u2-control", false) {
			t.Fatal("the control arm did not connect, so the row above measured nothing")
		}
	})

	t.Run("unlinked and re-bound at the same name", func(t *testing.T) {
		sock := filepath.Join(dir, "u3.sock")
		l := sbuServe(t, sock)
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(sock); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		sbuServe(t, sock) // a different socket at the same name
		walled := prof("u3-walled.sb", sock, true)
		sbuParses(t, walled)
		if sbuRun(t, walled, sbuConnectScript(nc, sock, ""), nil) {
			t.Error("the deny did not survive an unlink and a re-bind at the same name (ranger-base-q9bqu)")
		}
		if !sbuRun(t, prof("u3-control.sb", sock, false), sbuConnectScript(nc, sock, ""), nil) {
			t.Fatal("the control arm did not connect, so the row above measured nothing")
		}
	})
}

// The rendered profile, end to end: a seat under the real thing cannot
// connect to a grok leader socket at the grok home its own write grant
// reaches, and the same seat under the same profile with DenyConnect emptied
// can. The hole was exactly that the write grant is not a connect wall.
func TestLiveSeatbeltProfileRefusesAConnectToGroksLeaderSocket(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	sbSkipUnlessSandboxable(t)
	nc := sbuNetcat(t)
	f := wrcFixtureIn(t, sbuSockDir(t))
	rt := wrcRuntime("grok")
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, rt.StateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, rt.StateDirs...)
	sock := filepath.Join(absResolve(filepath.Join(f.home, ".grok")), grokLeaderSockName)
	if !containsString(c.DenyConnect, sock) {
		t.Fatalf("the carve-out does not deny a connect to %s, so nothing here is measured: %v", sock, c.DenyConnect)
	}
	if !writeGranted(w, sock) {
		t.Fatalf("the fixture's %s is outside the write grant, so this measures a path no seat could reach anyway: %v", sock, w)
	}
	sbuServe(t, sock)
	walled, control := sbuProfiles(t, f.ag.Name, w, c)
	sbuParses(t, walled)
	if sbuRun(t, walled, sbuConnectScript(nc, sock, ""), nil) {
		t.Errorf("a seat under the rendered profile connected to %s — the write grant is not a connect wall and this rule did not become one (ranger-base-q9bqu)", sock)
	}
	if !sbuRun(t, control, sbuConnectScript(nc, sock, ""), nil) {
		t.Fatal("the control arm, the same profile with DenyConnect emptied, did not connect — so the row above measured nothing")
	}
}
