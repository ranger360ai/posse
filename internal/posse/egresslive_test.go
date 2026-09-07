//go:build posse_arm3

package posse

// Live pin for rangerhq-9d0 — ADR 0002's verification 9, run against the
// real engine and the real cage image rather than a hermetic `env`. The
// tests in egress_test.go prove the rendering and the proxy's own
// behaviour; this one proves the property the whole tier rests on, which
// is a statement about *routing* and cannot be observed without a network:
//
//	inside the cage, an agent that ignores HTTPS_PROXY reaches nothing.
//
//	RHQ_LIVE_DOCKER=1 go test ./internal/posse -run TestLiveEgressBoundary -v
//
// Needs docker (or an engine answering its CLI) and the cage image built —
// `posse cage build ~/src/posse`. It spends no API turn: the allowlisted
// call is unauthenticated and Anthropic answers 401, which is the point —
// a 401 means the request *arrived*.
//
// Measured 2026-08-22, macOS 26.4.1, Docker 29.0.1, image posse-cage:latest:
// no proxy → curl exit 6 (nothing to resolve, nowhere to route); through
// the proxy to an allowlisted host → 401; through the proxy to any other
// host → curl exit 56 (the proxy's 403) and a line in refusals.log;
// external DNS resolves nothing.
//
// Re-measured 2026-08-23 (rangerhq-rli) with the raw-IP check that check 1
// was missing, docker 29.0.1: on the cage's `--internal` network
// `https://1.1.1.1/` and `https://140.82.121.4/` (Host: github.com) both
// answer curl exit 7, and this whole test passes with them added. The
// control that makes exit 7 mean something: the same two requests from a
// container on docker's default bridge answer http 301 and http 200. The
// hostname check alone cannot tell those two worlds apart.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveEgressBoundaryIsTheRouteNotTheEnvVar(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_DOCKER") == "" {
		t.Skip("set RHQ_LIVE_DOCKER=1 (needs docker and `posse cage build`)")
	}
	home := t.TempDir()
	a := &App{
		Home: home, ConfigPath: filepath.Join(home, "config.yaml"),
		EnvsDir: filepath.Join(home, "envs"), StateDir: filepath.Join(home, "state"),
		AgentsDir: filepath.Join(home, "agents"),
	}
	e, err := a.LoadEngine(a.ResolveEngine()) // the built-in docker
	if err != nil {
		t.Fatal(err)
	}
	image := a.CageImage()
	if why := a.CageNotReady(e, image); why != "" {
		t.Skip(why) // engine binary, engine liveness, image — in that order
	}
	rt, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\negress: [api.anthropic.com]\n")
	hosts, _ := EgressHosts(ag, rt)
	eg, err := a.PlanEgress(e, ag, rt, "live", image, hosts)
	if err != nil || eg == nil {
		t.Fatalf("plan: %v %v", eg, err)
	}
	t.Cleanup(func() { runCageSteps(eg.Down, os.Stderr) })
	runCageSteps(eg.Down, os.Stderr) // a leftover from a previous run
	if err := runCageSteps(eg.Up, os.Stderr); err != nil {
		t.Fatal(err)
	}

	// The probe container is the cage's own image on the cage's own
	// network, with the same proxy vars a launch renders — i.e. exactly
	// what a caged persona's shell can reach.
	probe := []string{"run", "--rm", "--network", eg.Net}
	for _, v := range EgressProxyVars() {
		probe = append(probe, "-e", v.Key+"="+v.Value)
	}
	sh := func(script string) string {
		out, _ := exec.Command(e.Binary(), append(append([]string{}, probe...), image, "sh", "-c", script)...).CombinedOutput()
		return strings.TrimSpace(string(out))
	}

	// 1. No route and no DNS. This is the boundary: it holds for an agent
	//    that never looks at HTTPS_PROXY at all.
	//
	//    Two checks, not one, and the second is the load-bearing half
	//    (rangerhq-rli). By a *hostname* curl answers exit 6 — "couldn't
	//    resolve host" — which proves the resolver is gone and says nothing
	//    about the route; an engine that blocks UDP/DNS while still NAT-ing
	//    outbound TCP passes it while leaking every host reachable by IP.
	//    That is not hypothetical: it is apple/container#2062, whose own
	//    isolation test passed over a live leak for exactly this reason.
	//    So the boundary is asserted by *raw IP*, where nothing can be
	//    mistaken for a name lookup.
	noProxy := `env -u HTTPS_PROXY -u https_proxy -u HTTP_PROXY -u http_proxy `
	if got := sh(noProxy + `curl -s -o /dev/null -m 10 -w %{exitcode} https://api.anthropic.com/v1/models`); got != "6" {
		t.Errorf("direct HTTPS out of the cage must fail to resolve (curl exit 6), got %q", got)
	}
	for _, ip := range []string{`https://1.1.1.1/`, `-H "Host: github.com" https://140.82.121.4/`} {
		// Non-zero rather than an exact code: docker answers 7 (no route to
		// connect on an --internal bridge, measured 2026-08-23) and an
		// engine that drops rather than refuses would answer 28. Both are
		// the boundary holding; 0 is the boundary gone.
		if got := sh(noProxy + `curl -sk -o /dev/null -m 10 -w %{exitcode} ` + ip); got == "0" {
			t.Errorf("raw-IP TCP out of the cage must not connect (docker: exit 7), got %q for %s — "+
				"the engine blocks DNS but not routing, so HTTPS_PROXY is politeness, not a boundary", got, ip)
		}
	}
	if got := sh(`getent hosts api.anthropic.com >/dev/null && echo resolved || echo none`); got != "none" {
		t.Errorf("an --internal network has no external DNS, got %q", got)
	}
	// 2. The allowlisted host arrives — 401 is Anthropic answering.
	if got := sh(`curl -s -o /dev/null -m 20 -w %{http_code} https://api.anthropic.com/v1/models`); got != "401" {
		t.Errorf("an allowlisted host must reach its origin through the proxy (401 = it arrived), got %q", got)
	}
	// 3. Anything else meets the proxy's 403 (curl reports 56 for a CONNECT
	//    the proxy refused), and the denial lands where L1's refusals do.
	if got := sh(`curl -s -o /dev/null -m 20 -w %{exitcode} https://example.com/`); got != "56" {
		t.Errorf("a host outside egress: must be refused by the proxy (curl exit 56), got %q", got)
	}
	log := filepath.Join(a.GatesDir("p"), "refusals.log")
	b, err := os.ReadFile(log)
	if err != nil || !strings.Contains(string(b), "CONNECT example.com:443 [egress proxy] (deny: not in egress:") {
		t.Errorf("the denial must land in %s like L1's do: %v\n%s", log, err, b)
	}

	// 4. Down takes both with it.
	if err := runCageSteps(eg.Down, os.Stderr); err != nil {
		t.Fatal(err)
	}
	for _, probe := range [][]string{
		{"network", "inspect", eg.Net}, {"container", "inspect", eg.Proxy},
	} {
		if exec.Command(e.Binary(), probe...).Run() == nil {
			t.Errorf("the route must die with the cage: %s is still there", probe[len(probe)-1])
		}
	}
}
