package posse

// L4 egress (ADR 0002 §3–4, bead rangerhq-9d0) — the one gate no other
// tier realizes, and the realization is the ROUTE, not the env var.
//
// The shape the rangerhq-89a spike measured and this file builds: the
// session's container joins a `--internal` network — no default route, no
// external DNS — whose only other member is a CONNECT proxy holding the
// PID's `egress:` list; the proxy is also on the engine's ordinary
// network, so it and only it can reach out. Measured inside that cage:
// direct HTTPS by hostname → curl exit 6, direct HTTPS by *raw IP* → curl
// exit 7 (rangerhq-rli, 2026-08-23), an allowlisted host through the proxy
// → 401 from Anthropic (it arrived), any other host → the proxy's 403,
// external DNS resolves nothing. That last property is the whole point: an
// agent that ignores HTTPS_PROXY reaches *nothing*, which is what makes
// this a boundary and not politeness.
//
// The raw-IP half of that is not a flourish. curl exit 6 on a hostname is
// "couldn't resolve host": it proves the resolver is gone and says nothing
// about the route, so an engine that blocks UDP/DNS while still NAT-ing
// outbound TCP looks identical here and leaks every host reachable by
// address (apple/container#2062 — the reason `container` is not the engine,
// rangerhq-rli). SpellsEgress asks whether an engine can *spell* the route;
// only the probe and the live test can say whether it holds it.
//
// Three rules from the ADR are structural here:
//
//   - The allowlist is rendered fresh from the PID at every launch, like
//     the gates: RHQ_HOME/state/cages/<persona>/<session>.egress.hosts,
//     mounted :ro into the proxy. Nothing hand-edited there survives.
//   - The launcher always adds the runtime's own hosts (Runtime.Egress),
//     because a cage that cannot reach its own API is not a session.
//   - The proxy's denials land in gates/<persona>/refusals.log in the same
//     shape L1's do. codex on a denied host retries ~70 times in 35s and
//     then errors hard (measured), so an `egress:` typo is a retry storm
//     and the log is where the operator sees which host it is.
//
// The proxy is built from the cage image (`posse cage build`) and runs on
// its node — one image, one build, one "image not built" refusal. It gets
// the script, the allowlist and the refusals log and NOTHING else: no
// session env, so the operator's credential never enters the process that
// terminates the agent's TLS handshakes.
//
// Honest limit, stated in the ADR and worth repeating where the code is:
// the proxy sees only the CONNECT authority. It stops *unknown hosts*, not
// exfiltration through an allowed one.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// EgressPort is the proxy's port on the internal network. The agent's
	// HTTPS_PROXY names it; nothing outside the internal network can.
	EgressPort = 8899
	// EgressHost is the proxy's hostname on that network. Each session has
	// its own network, so every session's proxy can wear the same name.
	EgressHost = "posse-egress"
	// EgressPrefix names the network and the proxy container per session.
	EgressPrefix = "posse-egress-"
)

// egressName sanitizes a session name into something an engine will accept
// as a container/network name. Session names are posse's (persona, bead id),
// but they are not the engine's grammar and a launch must not fail on a
// character.
var egressUnsafe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func egressName(session string) string {
	s := egressUnsafe.ReplaceAllString(session, "-")
	s = strings.Trim(s, "-._")
	if s == "" {
		s = "session"
	}
	return EgressPrefix + s
}

// egressHostPat is a host as the allowlist may spell it: a hostname, or
// `*.suffix` for a subtree. Anything else is a PID error.
var egressHostPat = regexp.MustCompile(`^(\*\.)?[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// normalizeEgressHost is forgiving about how a host is written in a PID —
// `https://github.com/`, `github.com:443` and `GitHub.com` are all the
// host the proxy will see in a CONNECT authority — and returns "" for
// anything that is not a host at all.
//
// A PATH is not a host, and this is where that has to be said. The proxy
// matches the CONNECT authority; it never sees a path, so quietly reading
// `github.com/anthropics` as `github.com` would widen the persona's rule
// to the whole host while the PID still reads as if it were narrower. The
// scheme and an empty path come off; anything more is refused upward.
func normalizeEgressHost(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if i := strings.Index(h, "://"); i >= 0 {
		h = h[i+3:]
	}
	if i := strings.Index(h, "/"); i >= 0 {
		if strings.Trim(h[i:], "/") != "" {
			return ""
		}
		h = h[:i]
	}
	h = strings.TrimSuffix(h, ".")
	if i := strings.LastIndex(h, ":"); i > 0 {
		h = h[:i]
	}
	if !egressHostPat.MatchString(h) {
		return ""
	}
	return h
}

// EgressHosts is the effective allowlist for a launch: the runtime's own
// hosts first (ADR 0002 §4 — the launcher always adds them), then the
// PID's. Returns the bad spellings separately so the launch can refuse
// with them rather than silently dropping a host the persona asked for.
func EgressHosts(ag *AgentFile, rt *Runtime) (hosts, bad []string) {
	var raw []string
	if rt != nil {
		raw = append(raw, rt.Egress...)
	}
	if ag != nil {
		raw = append(raw, ag.Egress...)
	}
	for _, h := range raw {
		if n := normalizeEgressHost(h); n != "" {
			hosts = append(hosts, n)
		} else if strings.TrimSpace(h) != "" {
			bad = append(bad, h)
		}
	}
	return dedupeStrings(hosts), dedupeStrings(bad)
}

// EgressProxyVars is what the agent's container is told about the route.
// Values, not names: they are computed by the launch, so they cannot ride
// the engine's `-e NAME` forwarding (which reads the pane's environment).
// They are also not a gate — an agent that unsets them reaches nothing at
// all, which is the property the --internal network provides and this
// pair only makes convenient.
func EgressProxyVars() []EnvVar {
	url := fmt.Sprintf("http://%s:%d", EgressHost, EgressPort)
	no := "localhost,127.0.0.1,::1"
	return []EnvVar{
		{"HTTPS_PROXY", url}, {"https_proxy", url},
		{"HTTP_PROXY", url}, {"http_proxy", url},
		{"NO_PROXY", no}, {"no_proxy", no},
	}
}

// ─── what gets rendered ──────────────────────────────────────────────────────

// EgressScript is the proxy's path: one per persona, rendered fresh at
// every launch like the gates.
func (a *App) EgressScript(persona string) string {
	return filepath.Join(a.CageDir(persona), "egress.js")
}

// EgressHostsFile is the allowlist, per *session*: two beads put the same
// persona in two containers and a PID edited between them must not reach
// back into a running cage.
func (a *App) EgressHostsFile(persona, session string) string {
	return filepath.Join(a.CageDir(persona), session+".egress.hosts")
}

// RefusalsLog is gates/<persona>/refusals.log — L1's audit trail, which
// the proxy appends to so a denied host reads like a denied verb. Created
// here if it does not exist: the engine bind-mounts the file itself (not
// the gates dir, which holds the shims and has no business inside a
// proxy), and a bind mount of a missing file makes a directory.
func (a *App) RefusalsLog(persona string) (string, error) {
	dir := a.GatesDir(persona)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "refusals.log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	return p, f.Close()
}

// RenderEgress writes the proxy script and the session's allowlist and
// returns the three paths the proxy container mounts. Like RenderGates it
// is a fresh render from the PID every time.
func (a *App) RenderEgress(ag *AgentFile, rt *Runtime, session string, hosts []string) (script, hostsFile, log string, err error) {
	if err := os.MkdirAll(a.CageDir(ag.Name), 0o755); err != nil {
		return "", "", "", err
	}
	script = a.EgressScript(ag.Name)
	if err := os.WriteFile(script, []byte(egressProxyJS), 0o644); err != nil {
		return "", "", "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# posse egress allowlist for %s on %s (session %s) — rendered from the PID\n", ag.Name, rt.Name, session)
	fmt.Fprintf(&b, "# at launch; do not edit. One host per line, or *.suffix for a subtree.\n")
	fmt.Fprintf(&b, "# The runtime's own hosts are always added (ADR 0002 §4).\n")
	for _, h := range hosts {
		fmt.Fprintln(&b, h)
	}
	hostsFile = a.EgressHostsFile(ag.Name, session)
	if err := os.WriteFile(hostsFile, []byte(b.String()), 0o644); err != nil {
		return "", "", "", err
	}
	log, err = a.RefusalsLog(ag.Name)
	if err != nil {
		return "", "", "", err
	}
	return script, hostsFile, log, nil
}

// ─── the plan the launcher runs ──────────────────────────────────────────────

// CageStep is one engine command the launcher runs around the cage. Fatal
// says whether failing it means the launch must not proceed: a network
// that already exists is fine (the previous cage of this session may still
// hold it), a proxy that will not start is not — a caged session whose
// only route out is a proxy that is not there would simply be offline, and
// a caged session whose network came up without the proxy would be too.
type CageStep struct {
	Argv  []string `json:"argv"`
	Fatal bool     `json:"fatal,omitempty"`
	Why   string   `json:"why"`
}

// CageEgress is the egress half of a launch plan: the names, the effective
// allowlist (so the operator reading the file sees what the persona may
// reach), and the steps that bring the route up and take it down.
type CageEgress struct {
	Net   string     `json:"net"`
	Proxy string     `json:"proxy"`
	Hosts []string   `json:"hosts"`
	Up    []CageStep `json:"up"`
	Down  []CageStep `json:"down"`
	// Launch fences the watcher against the launch that replaced it. The
	// network and the proxy are named for the SESSION, so a relaunch of
	// that session addresses the same two objects — and the old cage's
	// watcher wakes up (its parent died in the kill) at the same time the
	// new launcher is bringing them back. Without a fence the loser is
	// whichever ordering happens: the old watcher's `rm -f` lands on the
	// new proxy and the new cage is left with no route out.
	//
	// So the watcher carries the id of the launch it belongs to and re-reads
	// the plan before tearing anything down: a different id means a newer
	// launch owns these names now, and the old watcher has nothing to
	// reclaim. (The classic fencing token, on the smallest possible scale —
	// two writers, one resource, a monotonic id deciding which one still
	// holds it.)
	Launch string `json:"launch"`
}

// egressProxyMounts is everything the proxy container can see. The script
// and the allowlist read-only, the refusals log writable, and nothing
// else — no repo, no memory, no HOME, and (by not being in the engine's
// env forwards at all) none of the session's environment.
func egressProxyMounts(script, hosts, log string) []CageMount {
	return []CageMount{
		{Src: script, Dst: script, RO: true, Why: "the proxy itself"},
		{Src: hosts, Dst: hosts, RO: true, Why: "the allowlist rendered from the PID"},
		{Src: log, Dst: log, Why: "denials, in the same log L1's refusals land in"},
	}
}

// EngineSpellsEgress: can this engine express the route at all? A built-in
// docker (or an OrbStack that answers to the same CLI) can; an engine
// whose yaml leaves net_create:/proxy_up: unsaid cannot, and that is a
// parity statement — the launch refuses with the gate named, or degrades
// out loud, exactly like any other unrealized gate. It is NOT a reason to
// die inside the renderer: a caged PID with no egress gate has no business
// being refused because its engine cannot spell one.
func (e *Engine) SpellsEgress() bool { return e.NetCreate != "" && e.ProxyUp != "" }

// EngineEgress is that question for this RHQ_HOME's resolved engine — what
// parity asks before claiming `egress:` at the container tier.
func (a *App) EngineEgress() bool {
	e, err := a.LoadEngine(a.ResolveEngine())
	return err == nil && e.SpellsEgress()
}

// PlanEgress renders the allowlist and returns the plan the launcher runs
// to put the session behind the proxy. hosts is EgressHosts' answer; a nil
// plan (no error) is an engine that cannot spell the route.
func (a *App) PlanEgress(e *Engine, ag *AgentFile, rt *Runtime, session, image string, hosts []string) (*CageEgress, error) {
	if !e.SpellsEgress() {
		return nil, nil
	}
	script, hostsFile, log, err := a.RenderEgress(ag, rt, session, hosts)
	if err != nil {
		return nil, err
	}
	name := egressName(session)
	r := CageRender{
		Net: name, Proxy: name, Host: EgressHost, Image: image,
		Mounts: egressProxyMounts(script, hostsFile, log),
		Inner:  []string{"node", script, hostsFile, fmt.Sprint(EgressPort), log},
	}
	g := &CageEgress{Net: name, Proxy: name, Hosts: hosts, Launch: time.Now().UTC().Format(time.RFC3339Nano)}
	// Down first, and again at teardown: the render is fresh at every
	// launch, so a proxy left behind by a previous launch of this session
	// is replaced rather than trusted to be holding the right allowlist.
	g.Down = []CageStep{
		{Argv: e.stepArgv(e.ProxyDown, r), Why: "stop the session's egress proxy"},
		{Argv: e.stepArgv(e.NetRemove, r), Why: "remove the session's internal network (fails while a container still holds it, which is fine)"},
	}
	g.Up = []CageStep{
		{Argv: e.stepArgv(e.NetCreate, r), Why: "the --internal network: no default route, no external DNS"},
		{Argv: e.stepArgv(e.ProxyUp, r), Fatal: true, Why: "the CONNECT proxy holding this launch's allowlist"},
		{Argv: e.stepArgv(e.NetJoin, r), Fatal: true, Why: "give the proxy — and only the proxy — a way out"},
	}
	for _, s := range append(append([]CageStep{}, g.Up...), g.Down...) {
		if len(s.Argv) == 0 {
			return nil, Die("cage container: engine %s rendered an empty egress step (%s)", e.Name, s.Why)
		}
	}
	return g, nil
}

// ─── the proxy ───────────────────────────────────────────────────────────────

// egressProxyJS is the CONNECT proxy, in the cage image's own node so the
// tier needs no second image. It is the probe's python proxy
// (docs/adr/0002-container-tier.probe.sh, probe 2) with the two things a
// launch needs that a probe did not: the allowlist comes from a file the
// launch renders rather than from argv, and a denial is appended to
// refusals.log as well as printed.
//
// Only CONNECT crosses it. A plain-HTTP proxy request would carry the URL
// and the body through this process — and every runtime the fleet knows
// speaks TLS to its API — so an http:// request is refused and named
// rather than quietly forwarded.
const egressProxyJS = `#!/usr/bin/env node
// posse egress proxy — L4 (ADR 0002 §3, rangerhq-9d0). Rendered at launch by
// internal/posse/egress.go; do not edit. Usage: node egress.js <allowlist>
// <port> <refusals.log>
'use strict';
const net = require('net');
const fs = require('fs');

const [allowFile, portArg, logFile] = process.argv.slice(2);
const port = parseInt(portArg, 10);
const allow = fs.readFileSync(allowFile, 'utf8').split('\n')
  .map(s => s.replace(/#.*$/, '').trim().toLowerCase())
  .filter(s => s.length > 0);
const listText = allow.join(',');

// The allowlist is a host, or *.suffix for a subtree. Matching is on the
// CONNECT authority and nothing else: this proxy never sees a path.
const allowed = h => allow.some(p => p === h || (p.startsWith('*.') && h.endsWith(p.slice(1))));

const stamp = () => new Date().toISOString().replace(/\.\d+Z$/, 'Z');

function log(line) {
  process.stdout.write(line + '\n');
  // Same log L1's shims append to. Best effort: a launch whose refusals
  // log could not be mounted still gets a working boundary and docker logs.
  try { fs.appendFileSync(logFile, line + '\n'); } catch (e) {}
}

function refuse(sock, what, why) {
  log(stamp() + ' ' + what + ' [egress proxy] (deny: ' + why + ')');
  sock.end('HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\nConnection: close\r\n\r\n');
}

const server = net.createServer(client => {
  let head = Buffer.alloc(0);
  client.on('error', () => {});
  // Only the request head is on a clock; an established tunnel may idle as
  // long as the agent's own API does (streaming turns are minutes long).
  client.setTimeout(30000, () => client.destroy());
  const onHead = chunk => {
    head = Buffer.concat([head, chunk]);
    const end = head.indexOf('\r\n\r\n');
    if (end < 0) {
      if (head.length > 65536) client.destroy();
      return;
    }
    client.removeListener('data', onHead);
    client.pause();
    const first = head.slice(0, head.indexOf('\r\n')).toString('latin1');
    const m = /^CONNECT +([^ :]+):([0-9]+) /.exec(first);
    if (!m) {
      refuse(client, 'non-CONNECT ' + first.split(' ').slice(0, 2).join(' '),
             'only CONNECT (https) crosses this proxy');
      return;
    }
    const host = m[1].toLowerCase(), dport = parseInt(m[2], 10);
    if (!allowed(host)) {
      refuse(client, 'CONNECT ' + host + ':' + dport, 'not in egress: ' + listText);
      return;
    }
    const up = net.connect(dport, host);
    up.on('error', e => {
      log(stamp() + ' CONNECT ' + host + ':' + dport + ' [egress proxy] upstream ' + (e.code || e.message));
      client.destroy();
    });
    up.on('connect', () => {
      client.setTimeout(0);
      client.write('HTTP/1.1 200 Connection established\r\n\r\n');
      const rest = head.slice(end + 4);
      if (rest.length) up.write(rest);
      client.pipe(up);
      up.pipe(client);
    });
    up.on('close', () => client.destroy());
    client.on('close', () => up.destroy());
  };
  client.on('data', onHead);
});

server.on('error', e => { log(stamp() + ' egress proxy failed: ' + e.message); process.exit(1); });
// The port the kernel gave us, not the one asked for: a launch passes the
// fixed EgressPort and reads the same line back, and a caller that passes 0
// (the tests) learns where the proxy is from the proxy rather than from a
// port it held open and let go of.
server.listen(port, '0.0.0.0', () => log(stamp() + ' egress proxy up on :' + server.address().port + ' (allow: ' + listText + ')'));
`
