package posse

// L4 egress (rangerhq-9d0): the allowlist rendered from the PID, the route
// the launch plans around the cage, the watcher that takes it down again,
// and the proxy itself — run for real, because "the realization is the
// route" is a claim about behaviour and not about a rendering.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The launcher always adds the runtime's own hosts (ADR 0002 §4): a cage
// that cannot reach its own model is not an isolated persona, it is an
// offline one. Everything else comes from the PID, and a host that is not
// a host refuses the launch rather than being dropped in silence.
func TestEgressAllowlistIsThePIDPlusTheRuntimesOwnHosts(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	claude, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\negress: [\"https://GitHub.com/\", proxy.golang.org:443, api.anthropic.com]\n")
	hosts, bad := EgressHosts(ag, claude)
	if len(bad) > 0 {
		t.Fatalf("all four spellings are hosts: %q", bad)
	}
	want := []string{"api.anthropic.com", "platform.claude.com", "github.com", "proxy.golang.org"}
	if strings.Join(hosts, ",") != strings.Join(want, ",") {
		t.Errorf("runtime's hosts first, PID's after, normalized and deduped:\n got %q\nwant %q", hosts, want)
	}
	// Each runtime brings its own, and none brings the others'.
	for _, c := range []struct{ rt, host string }{
		{"codex", "chatgpt.com"}, {"grok", "cli-chat-proxy.grok.com"},
	} {
		rt, _ := a.LoadRuntime(c.rt)
		h, _ := EgressHosts(nil, rt)
		if len(h) == 0 || h[0] != c.host {
			t.Errorf("%s must open its own API: %q", c.rt, h)
		}
	}
	// A path is not a host: the proxy matches the CONNECT authority and
	// never sees one, so a rule written as a URL path could only ever be a
	// lie about what is enforced.
	badAg := cageAgent(t, a, "cage: container\negress: [\"github.com/anthropics\", \"not a host\"]\n")
	if _, bad := EgressHosts(badAg, claude); len(bad) != 2 {
		t.Errorf("a path or a phrase is not a host: %q", bad)
	}
	rt, _ := a.LoadRuntime("claude")
	if _, err := a.WrapInCage(badAg, rt, "s1", t.TempDir(), "claude", []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err == nil ||
		!strings.Contains(err.Error(), "is not a host") {
		t.Errorf("and the launch refuses on it rather than dropping it: %v", err)
	}
}

// The allowlist is rendered fresh from the PID at every launch, like the
// gates: nothing hand-edited there survives, and a host dropped from the
// PID stops being reachable on the next launch.
func TestEgressAllowlistIsRenderedFreshAtEveryLaunch(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	rt, _ := a.LoadRuntime("claude")
	dir := t.TempDir()
	ag := cageAgent(t, a, "cage: container\negress: [github.com]\n")
	if _, err := a.WrapInCage(ag, rt, "s1", dir, "claude", []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err != nil {
		t.Fatal(err)
	}
	hostsFile := a.EgressHostsFile("p", "s1")
	first, err := os.ReadFile(hostsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "github.com") || !strings.Contains(string(first), "api.anthropic.com") {
		t.Errorf("the rendered allowlist is the PID's plus the runtime's:\n%s", first)
	}
	// Hand-edit it, drop the host from the PID, launch again.
	os.WriteFile(hostsFile, []byte("evil.example.com\n"), 0o644)
	ag2 := cageAgent(t, a, "cage: container\n")
	if _, err := a.WrapInCage(ag2, rt, "s1", dir, "claude", []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(hostsFile)
	if strings.Contains(string(b), "evil.example.com") || strings.Contains(string(b), "github.com") {
		t.Errorf("a fresh render keeps neither a hand edit nor a dropped host:\n%s", b)
	}
	if !strings.Contains(string(b), "api.anthropic.com") {
		t.Errorf("but the runtime's own hosts are always there:\n%s", b)
	}
	// Per session: two beads put the same persona in two containers.
	if a.EgressHostsFile("p", "s1") == a.EgressHostsFile("p", "s2") {
		t.Error("the allowlist is per session")
	}
}

// The route, not the env var. What the plan has to be: an --internal
// network, a proxy that is the only other thing on it and the only thing
// with a way out, the agent joined to it by the engine's own flag, and a
// proxy that carries none of the session's environment.
func TestEgressPlanIsTheRouteAndTheProxyHoldsNothing(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	rt, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\negress: [github.com]\n")
	dir := t.TempDir()
	if _, err := a.WrapInCage(ag, rt, "s1", dir, "claude", []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err != nil {
		t.Fatal(err)
	}
	e, _ := a.LoadEngine(a.ResolveEngine())
	hosts, _ := EgressHosts(ag, rt)
	eg, err := a.PlanEgress(e, ag, rt, "s1", "img", hosts)
	if err != nil || eg == nil {
		t.Fatalf("plan: %v %v", eg, err)
	}
	up := stepLines(eg.Up)
	if !strings.Contains(up[0], "network create --internal "+eg.Net) {
		t.Errorf("the agent's network has no default route and no external DNS: %q", up)
	}
	if !strings.Contains(up[1], "--network "+eg.Net) || !strings.Contains(up[1], "--hostname "+EgressHost) {
		t.Errorf("the proxy is the only other member of that network: %q", up)
	}
	if !strings.Contains(up[2], "connect bridge "+eg.Proxy) {
		t.Errorf("and the only thing on it with a way out: %q", up)
	}
	if len(eg.Up) != 3 || !eg.Up[1].Fatal || !eg.Up[2].Fatal || eg.Up[0].Fatal {
		t.Errorf("a network that already exists is reusable; a proxy that will not start is not: %+v", eg.Up)
	}
	down := stepLines(eg.Down)
	if !strings.Contains(down[0], "rm -f "+eg.Proxy) || !strings.Contains(down[1], "network rm "+eg.Net) {
		t.Errorf("down removes the proxy and then the network: %q", down)
	}
	// The proxy terminates the agent's TLS handshakes, so it holds no
	// credential — no env forwarding on its line at all — and sees only
	// itself, the allowlist and the log it appends denials to.
	if strings.Contains(up[1], "CLAUDE_CODE_OAUTH_TOKEN") || strings.Contains(up[1], " -e ") {
		t.Errorf("the proxy must carry none of the session's environment: %q", up[1])
	}
	for _, want := range []string{
		a.EgressScript("p") + ":" + a.EgressScript("p") + ":ro",
		a.EgressHostsFile("p", "s1") + ":" + a.EgressHostsFile("p", "s1") + ":ro",
		filepath.Join(a.GatesDir("p"), "refusals.log"),
	} {
		if !strings.Contains(up[1], want) {
			t.Errorf("the proxy mounts %s and nothing else:\n%s", want, up[1])
		}
	}
	if strings.Contains(up[1], dir) || strings.Contains(up[1], ag.MemoryDir) {
		t.Errorf("no repo, no memory, no HOME in the proxy: %s", up[1])
	}
	// The agent is told where the route is. Not a gate — an agent that
	// unsets these reaches nothing at all, which is the network's doing —
	// but the difference between a working session and a silent one.
	vars := map[string]bool{}
	for _, v := range EgressProxyVars() {
		vars[v.Key] = true
		if v.Key == "HTTPS_PROXY" && v.Value != fmt.Sprintf("http://%s:%d", EgressHost, EgressPort) {
			t.Errorf("HTTPS_PROXY names the proxy on the internal network: %q", v.Value)
		}
	}
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "NO_PROXY", "no_proxy"} {
		if !vars[k] {
			t.Errorf("both spellings of every proxy var: %v", vars)
		}
	}
	// An engine that cannot spell the route plans none — and parity, not
	// the renderer, is what refuses a PID that needed it.
	os.WriteFile(filepath.Join(a.CagesDir(), "routeless.yaml"), []byte("command: env {cmd}\n"), 0o644)
	re, _ := a.LoadEngine("routeless")
	if got, err := a.PlanEgress(re, ag, rt, "s1", "img", hosts); got != nil || err != nil {
		t.Errorf("no net_create:/proxy_up: → no plan and no error: %v %v", got, err)
	}
}

func stepLines(steps []CageStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = strings.Join(s.Argv, " ")
	}
	return out
}

// "Start with the cage, die with it." The launcher execs the engine, so
// nothing of posse is left in the pane to notice the container exiting — the
// watcher it forks first holds the launcher's pid, which the exec does not
// change, and comes down when that process does.
func TestEgressWatcherOutlivesTheExecAndThenTakesTheRouteDown(t *testing.T) {
	a := cageApp(t)
	rt, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\negress: [github.com]\n")
	if _, err := a.WrapInCage(ag, rt, "s1", t.TempDir(), "claude", []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err != nil {
		t.Fatal(err)
	}
	planFile := a.CageArgvFile("p", "s1")
	plan := readPlan(t, planFile)
	if plan.Egress == nil {
		t.Fatal("the launch plan must carry the route")
	}
	// Rewrite the steps as things a test can watch happen.
	touched := filepath.Join(t.TempDir(), "steps")
	plan.Egress.Up = []CageStep{{Argv: []string{"sh", "-c", "echo up >> " + touched}, Why: "up"}}
	plan.Egress.Down = []CageStep{{Argv: []string{"sh", "-c", "echo down >> " + touched}, Why: "down"}}
	writePlan(t, planFile, plan)

	// The watcher is our own binary under its own name, not the launcher
	// symlink's (`claude`) — a background process wearing the runtime's
	// name is exactly the confusion the argv0 launcher exists to avoid.
	seen := filepath.Join(t.TempDir(), "watcher-argv")
	stub := filepath.Join(t.TempDir(), "stub")
	// One line per write, with a pause between them. The stub is another
	// process, so a read of this file lands mid-write whatever it writes —
	// writing the argv a line at a time only makes that certain instead of
	// occasional, which is what keeps the wait below honest: take the first
	// non-empty read and this test is red every time (ranger-base-yl8j).
	os.WriteFile(stub, []byte("#!/bin/sh\n: > "+seen+"\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> "+seen+"; sleep 0.05; done\n"), 0o755)
	old := cageReaperBin
	cageReaperBin = func() (string, error) { return stub, nil }
	defer func() { cageReaperBin = old }()

	if err := StartCageEgress(plan, planFile, os.Stderr); err != nil {
		t.Fatal(err)
	}
	// Down runs before up: the render is fresh at every launch, so a proxy
	// left by the previous launch of this session holds a stale allowlist
	// and is replaced rather than reused.
	if b, _ := os.ReadFile(touched); string(b) != "down\nup\n" {
		t.Errorf("a launch replaces the previous route rather than joining it: %q", b)
	}
	// Wait for the three things the assertion wants, not for the file to
	// stop being empty: "non-empty" is not "finished" when another process
	// is still writing it, and a short read here reports a watcher that was
	// handed two of three arguments by a launcher that always passes all
	// three. When they never arrive, the failure names the last read.
	want := []string{CageReapFlag, planFile, fmt.Sprint(os.Getpid())}
	deadline := time.Now().Add(egressWait)
	var argv string
	for {
		b, _ := os.ReadFile(seen)
		argv = string(b)
		got := true
		for _, w := range want {
			got = got && strings.Contains(argv, w)
		}
		if got {
			break
		}
		if !time.Now().Before(deadline) {
			t.Errorf("the watcher is handed the plan and the pid the exec will not change; in %s it was handed %q, wanted all of %q", egressWait, argv, want)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// And the watcher itself: while its parent is still the engine it does
	// nothing; the moment it is not, the route comes down.
	os.Truncate(touched, 0)
	done := make(chan error, 1)
	go func() { done <- RunCageReap([]string{"posse", CageReapFlag, planFile, fmt.Sprint(os.Getppid())}) }()
	select {
	case <-done:
		t.Error("a live parent must not bring the route down")
	case <-time.After(3 * cageReapPoll):
	}
	if err := RunCageReap([]string{"posse", CageReapFlag, planFile, "999999"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(touched); !strings.Contains(string(b), "down") {
		t.Errorf("a parent that is gone takes the route with it: %q", b)
	}
	// The operator's way back from a watcher that died with its pane.
	os.Truncate(touched, 0)
	n, err := a.TearDownCageEgress("p", os.Stderr)
	if err != nil || n != 1 {
		t.Errorf("posse cage down walks the persona's rendered plans: %d %v", n, err)
	}
	if b, _ := os.ReadFile(touched); !strings.Contains(string(b), "down") {
		t.Errorf("and takes each route down: %q", b)
	}
	if n, _ := a.TearDownCageEgress("nobody", os.Stderr); n != 0 {
		t.Error("a persona with no cage state has nothing to take down")
	}
}

// The fence (CageEgress.Launch). The network and the proxy are named for
// the SESSION, so a relaunch owns the same two names — and the kill that
// starts a relaunch is exactly what wakes the previous cage's watcher. An
// unfenced watcher's `rm -f` would land on the new cage's proxy and leave
// a live session with no route out.
func TestEgressWatcherWillNotReclaimANewerLaunchsRoute(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	rt, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\negress: [github.com]\n")
	dir := t.TempDir()
	if _, err := a.WrapInCage(ag, rt, "s1", dir, "claude", []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err != nil {
		t.Fatal(err)
	}
	planFile := a.CageArgvFile("p", "s1")
	mine := readPlan(t, planFile).Egress.Launch
	if mine == "" {
		t.Fatal("every launch of a route carries an id")
	}
	if cageRouteSuperseded(planFile, mine) {
		t.Error("its own launch is not a newer one")
	}
	// A relaunch of the same session: same names, new id.
	if _, err := a.WrapInCage(ag, rt, "s1", dir, "claude", []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err != nil {
		t.Fatal(err)
	}
	next := readPlan(t, planFile)
	if next.Egress.Launch == mine {
		t.Fatal("a relaunch renders a new launch id")
	}
	if next.Egress.Net != EgressPrefix+"s1" {
		t.Fatalf("and the same names, which is why the fence exists: %s", next.Egress.Net)
	}
	if !cageRouteSuperseded(planFile, mine) {
		t.Error("the old watcher must stand down for the newer launch")
	}
	// But only a plan that parses moves the fence: a route must not leak
	// because the session's state went away underneath its watcher.
	os.WriteFile(planFile, []byte("{not json"), 0o644)
	if cageRouteSuperseded(planFile, mine) {
		t.Error("an unreadable plan is not a newer launch")
	}
	os.Remove(planFile)
	if cageRouteSuperseded(planFile, mine) {
		t.Error("nor is a missing one")
	}
}

// The proxy, run for real. Everything above is a rendering; this is the
// boundary: an unknown host is refused with a 403 and a line in the same
// log L1's shims append to, an allowlisted one is tunnelled, and a
// plain-HTTP proxy request — which would carry a URL and a body through
// this process — is refused and named.
func TestEgressProxyRefusesUnknownHostsAndLogsThemLikeL1(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node on this host; the proxy runs on the cage image's own")
	}
	a := cageApp(t)
	rt, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\negress: [localhost, \"*.allowed.example\"]\n")
	hosts, _ := EgressHosts(ag, rt)
	script, hostsFile, log, err := a.RenderEgress(ag, rt, "s1", hosts)
	if err != nil {
		t.Fatal(err)
	}
	// An echo server standing in for an allowlisted origin.
	origin, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	go func() {
		for {
			c, err := origin.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); b := make([]byte, 64); n, _ := c.Read(b); c.Write(b[:n]) }()
		}
	}()

	addr := startEgressProxy(t, node, script, hostsFile, log)

	// 1. A host nobody allowed: the proxy's 403, and the denial in the log.
	if got := connect(t, addr, "blocked.example.com:443", ""); !strings.Contains(got, "403") {
		t.Errorf("an unknown host must get the proxy's 403: %q", got)
	}
	line := waitForLog(t, log, "blocked.example.com")
	if !strings.Contains(line, "CONNECT blocked.example.com:443 [egress proxy] (deny: not in egress:") {
		t.Errorf("a denied host reads like a denied verb in refusals.log: %q", line)
	}
	if !strings.Contains(line, "api.anthropic.com") {
		t.Errorf("and names the allowlist it was measured against, so a typo is visible: %q", line)
	}

	// 2. An allowlisted host: the tunnel is established and carries bytes.
	_, oport, _ := net.SplitHostPort(origin.Addr().String())
	if got := connect(t, addr, "localhost:"+oport, "ping"); !strings.Contains(got, "200 Connection established") || !strings.Contains(got, "ping") {
		t.Errorf("an allowlisted host is tunnelled through: %q", got)
	}
	// A subtree pattern is a suffix match on the authority, never a prefix.
	if got := connect(t, addr, "notallowed.example:443", ""); !strings.Contains(got, "403") {
		t.Errorf("*.allowed.example must not match notallowed.example: %q", got)
	}

	// 3. Not CONNECT: refused and named, rather than quietly forwarded.
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(c, "GET http://blocked.example.com/ HTTP/1.1\r\nHost: blocked.example.com\r\n\r\n")
	b := make([]byte, 256)
	n, _ := c.Read(b)
	c.Close()
	if !strings.Contains(string(b[:n]), "403") {
		t.Errorf("a plain-HTTP proxy request must be refused: %q", b[:n])
	}
	if l := waitForLog(t, log, "non-CONNECT"); !strings.Contains(l, "only CONNECT (https) crosses this proxy") {
		t.Errorf("and say why: %q", l)
	}
}

// egressWait is the one budget every wait on another process in this file
// uses — the rendered proxy, and the forked watcher's argv. It is
// headroom for the scheduler, not for the code: 200 timed starts of this
// proxy under a concurrent `go test ./...` at load 12-15 came up in p50 74ms,
// p99 135ms, max 157ms. It has to be headroom, because the load this box
// really carries is worse than that — `go test ./...` runs three package
// binaries at once beside a dozen live worktree sessions running their own
// suites, and there this test went red about one full run in four at 5.38s,
// a 5s budget expiring, while alone it took 0.09s (ranger-base-7qwm,
// ranger-base-5fw5). 30s is ~190x the measured worst case, and the only
// thing left that trips it is a proxy that never came up at all. Nothing
// here is a latency assertion.
const egressWait = 30 * time.Second

// startEgressProxy runs the rendered proxy and returns the address to drive
// it on. The port is the kernel's, handed to node itself and read back from
// the proxy's own up-line — never one this process took, closed and passed
// on. That window is open for the whole of node's start, and losing it does
// not look like a race: measured on this box (ranger-base-7qwm), a squatter
// on 127.0.0.1:P leaves the proxy binding 0.0.0.0:P and reporting "up"
// while every dial to 127.0.0.1:P reaches the squatter, because the more
// specific bind wins. The test would then be asserting about somebody
// else's socket.
func startEgressProxy(t *testing.T, node, script, hosts, log string) string {
	t.Helper()
	proxy := exec.Command(node, script, hosts, "0", log)
	out, err := proxy.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	proxy.Stderr = os.Stderr
	if err := proxy.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { proxy.Process.Kill(); proxy.Wait() })

	// The first line is the up-line, or the listen error the script exits
	// on; either way the read ends, so a proxy that never came up is named
	// rather than waited out. The rest of stdout is drained: the pipe is
	// 64KB and a full one blocks every log() the proxy makes.
	first := make(chan string, 1)
	go func() {
		r := bufio.NewReader(out)
		l, _ := r.ReadString('\n')
		first <- l
		io.Copy(io.Discard, r)
	}()
	var line string
	select {
	case line = <-first:
	case <-time.After(egressWait):
		t.Fatalf("the proxy said nothing in %s", egressWait)
	}
	_, rest, ok := strings.Cut(line, " egress proxy up on :")
	if !ok {
		t.Fatalf("the proxy never came up; it said %q", strings.TrimSpace(line))
	}
	digits, _, _ := strings.Cut(rest, " ")
	port, err := strconv.Atoi(digits)
	if err != nil || port == 0 {
		t.Fatalf("the proxy must report the port it is actually on: %q", strings.TrimSpace(line))
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	waitDial(t, addr)
	return addr
}

func waitDial(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(egressWait)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			c.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("the proxy never came up on %s", addr)
}

// connect speaks one CONNECT to the proxy and returns everything it says
// back, plus whatever an established tunnel echoes for body.
func connect(t *testing.T, addr, authority, body string) string {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(egressWait))
	fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", authority, authority)
	var sb strings.Builder
	b := make([]byte, 512)
	n, _ := c.Read(b)
	sb.Write(b[:n])
	if body != "" && strings.Contains(sb.String(), "200") {
		fmt.Fprint(c, body)
		n, _ := c.Read(b)
		sb.Write(b[:n])
	}
	return sb.String()
}

func waitForLog(t *testing.T, log, want string) string {
	t.Helper()
	deadline := time.Now().Add(egressWait)
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(log)
		for _, l := range strings.Split(string(b), "\n") {
			if strings.Contains(l, want) {
				return l
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("no %q in %s", want, log)
	return ""
}

func readPlan(t *testing.T, path string) CageLaunchPlan {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p CageLaunchPlan
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func writePlan(t *testing.T, path string, p CageLaunchPlan) {
	t.Helper()
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
