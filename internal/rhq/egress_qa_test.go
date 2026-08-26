package rhq

// QA pins for rangerhq-nc6y (close of rangerhq-9d0). The hermetic suite in
// egress_test.go renders the fake engine's yaml, not the built-in docker
// template; dropping `--internal` from builtinEngines left TestEgress* green.
// The live pin catches that only when RHQ_LIVE_DOCKER=1 and posse-cage:latest
// is built. This file pins the spelling the fleet actually launches.

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
)

func TestQABuiltinDockerEngineSpellsTheInternalRoute(t *testing.T) {
	a := &App{Home: t.TempDir()}
	d, err := a.LoadEngine("docker")
	if err != nil {
		t.Fatal(err)
	}
	if d.NetCreate != "docker network create --internal {net}" {
		t.Errorf("the built-in engine's network is --internal, not a NAT: %q", d.NetCreate)
	}
	if d.Net != "--network {net}" {
		t.Errorf("the agent joins that network: %q", d.Net)
	}
	if d.NetJoin != "docker network connect bridge {proxy}" {
		t.Errorf("only the proxy gets a way out: %q", d.NetJoin)
	}
	if !strings.Contains(d.ProxyUp, "--network {net}") || !strings.Contains(d.ProxyUp, "--hostname {host}") {
		t.Errorf("the proxy is the other member of the internal network: %q", d.ProxyUp)
	}
	if strings.Contains(d.ProxyUp, "{env}") || strings.Contains(d.ProxyUp, " -e ") {
		t.Errorf("the proxy must carry none of the session's environment: %q", d.ProxyUp)
	}
	if d.ProxyDown != "docker rm -f {proxy}" {
		t.Errorf("down removes the proxy: %q", d.ProxyDown)
	}
}

// CONNECT to a raw IP, or to a subdomain of an exact-allowed host, must
// 403. The network half of this lesson is rangerhq-rli (hostname curl
// exit 6 is DNS, not the route); the proxy half is the same hole if the
// matcher is only exercised on names the allowlist already mentions.
func TestQAEgressProxyDeniesRawIPAndSubdomainsOfExactHosts(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node on this host; the proxy runs on the cage image's own")
	}
	a := cageApp(t)
	rt, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\negress: [github.com]\n")
	hosts, _ := EgressHosts(ag, rt)
	script, hostsFile, log, err := a.RenderEgress(ag, rt, "s1", hosts)
	if err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	proxy := exec.Command(node, script, hostsFile, fmt.Sprint(port), log)
	if err := proxy.Start(); err != nil {
		t.Fatal(err)
	}
	defer proxy.Process.Kill()
	waitDial(t, fmt.Sprintf("127.0.0.1:%d", port))
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	if got := connect(t, addr, "1.1.1.1:443", ""); !strings.Contains(got, "403") {
		t.Errorf("CONNECT to a raw IP not in egress: must 403, got %q", got)
	}
	if l := waitForLog(t, log, "1.1.1.1"); !strings.Contains(l, "CONNECT 1.1.1.1:443 [egress proxy] (deny: not in egress:") {
		t.Errorf("raw-IP deny must land in refusals.log: %q", l)
	}
	if got := connect(t, addr, "api.github.com:443", ""); !strings.Contains(got, "403") {
		t.Errorf("an exact host must not allow a subdomain: %q", got)
	}
	if got := connect(t, addr, "github.com.evil.com:443", ""); !strings.Contains(got, "403") {
		t.Errorf("suffix injection of an exact host must 403: %q", got)
	}
	// Control: github.com itself is allowed — upstream may 200 or fail to
	// connect; a 403 here would mean the allowlist never matched.
	if got := connect(t, addr, "github.com:443", ""); strings.Contains(got, "403") {
		t.Errorf("the exact allowlisted host must not be refused: %q", got)
	}

	// IPv6 CONNECT is not in the proxy's grammar and must fail closed
	// (non-CONNECT 403), not be forwarded.
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(c, "CONNECT [::1]:443 HTTP/1.1\r\nHost: [::1]:443\r\n\r\n")
	b := make([]byte, 256)
	n, _ := c.Read(b)
	c.Close()
	if !strings.Contains(string(b[:n]), "403") {
		t.Errorf("IPv6 CONNECT must fail closed: %q", b[:n])
	}
}
