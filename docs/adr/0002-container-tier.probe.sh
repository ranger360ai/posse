#!/bin/sh
# Container tier (ADR 0002 §3 L4) — the five probes behind the NOTES.md
# "Container tier (L4)" section. Verified 2026-08-18 on macOS 26.4.1,
# Docker Desktop 4.53.0 (engine 29.0.1, VirtioFS), herdr 0.8.0, bead
# rangerhq-89a. Run from a scratch dir; every artifact it makes, it removes.
#
#   sh docs/adr/0002-container-tier.probe.sh [PANE_ID]
#
# PANE_ID (optional) is an idle herdr pane — probe 3 needs one; without it
# that probe is skipped. Nothing here writes to the repo or to any persona.
set -e
IMG=posse-probe:claude
NET_IN=posse-probe-int
NET_EX=posse-probe-ext
PANE=$1
WORK=$(mktemp -d)
trap 'docker rm -f posse-probe-proxy >/dev/null 2>&1 || true
      docker network rm $NET_IN $NET_EX >/dev/null 2>&1 || true
      docker rmi -f $IMG >/dev/null 2>&1 || true
      rm -rf "$WORK"' EXIT

# ----------------------------------- 0. host precondition (bead rangerhq-bnvk)
# Probe 2's leak detector below is only meaningful stated together with this
# number. apple/container#2062 leaks outbound TCP off a hostOnly network
# *because the macOS host NATs it* — and Apple's own maintainer could not
# reproduce the leak with net.inet.ip.forwarding at its macOS default of 0
# (issue thread, 2026-08-05). So a vmnet-backed engine measured on a host with
# forwarding=0 answers 000 on the raw-IP line and looks isolated when nothing
# about the engine has changed. That is a false pass in this instrument, not a
# finding. Read the VALUE, not the exit status: a non-numeric answer has to
# land in "unknown", never be mistaken for "off".
FWD=$(sysctl -n net.inet.ip.forwarding 2>/dev/null || true)
case "$FWD" in ''|*[!0-9]*) FWD=unknown ;; esac
echo "== 0. host precondition: net.inet.ip.forwarding = $FWD =="
case "$FWD" in
  0)
    echo "   docker: UNAFFECTED — its --internal network is enforced inside the"
    echo "   Linux VM, not by macOS vmnet, so probe 2 stands exactly as written."
    echo "   A VMNET-BACKED ENGINE (apple/container) MEASURED HERE IS VOID: at"
    echo "   forwarding=0 the raw-IP line answers 000 for the HOST's reason, not"
    echo "   the engine's. Re-measure with forwarding=1, or do not claim a pass." ;;
  unknown)
    echo "   WARNING: sysctl unreadable — treat probe 2's verdict as unqualified." ;;
  *)
    echo "   forwarding is ON: the apple/container#2062 leak condition is live on"
    echo "   this host, so a vmnet-backed engine measured now is a fair test." ;;
esac

# ---------------------------------------------------------------- image
cat > "$WORK/Dockerfile" <<'DOCK'
FROM node:22-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl ripgrep procps && rm -rf /var/lib/apt/lists/*
RUN npm i -g @anthropic-ai/claude-code && claude --version
WORKDIR /work
DOCK
echo "== 0. image build (measured: ~27s warm base) =="
time docker build -q -t $IMG "$WORK" >/dev/null

# ------------------------------------------------- 1. herdr socket passthrough
# A macOS unix socket, bind-mounted into a Linux container, really carries a
# herdr API round-trip. (It also hands the caged persona every other pane in
# the fleet — mount it only when the persona needs `posse`.)
cat > "$WORK/ping.py" <<'PY'
import socket, json, sys
s = socket.socket(socket.AF_UNIX); s.connect('/h.sock'); s.settimeout(5)
s.sendall((json.dumps({"id":"probe","method":"agent.list","params":{}})+"\n").encode())
buf=b""
while b"\n" not in buf:
    d=s.recv(65536)
    if not d: break
    buf+=d
print("agents seen from inside the container:",
      len(json.loads(buf.split(b"\n")[0]).get("result",{}).get("agents",[])))
PY
echo "== 1. herdr socket through the boundary =="
docker run --rm -v "$HOME/.config/herdr/herdr.sock:/h.sock" \
  -v "$WORK/ping.py:/p.py:ro" python:3.13-alpine python /p.py

# --------------------------------------------------- 2. egress allowlist proxy
# The realization is the *route*, not the env var: the agent's network is
# --internal (no default route, no external DNS) and the only thing on it is a
# CONNECT proxy with an allowlist. HTTPS_PROXY is honoured by claude, codex and
# grok alike, but an agent that ignores it reaches nothing at all.
#
# Ask by RAW IP, not only by hostname (rangerhq-rli). curl's exit 6 on a
# hostname is "couldn't resolve host" — it proves the resolver is gone and
# says NOTHING about routing, so an engine that blocks UDP/DNS while still
# NAT-ing outbound TCP passes the hostname check while leaking every host
# reachable by address. That is apple/container#2062, open as of 2026-08-23,
# whose own isolation test passed over a live leak for exactly this reason.
# Any engine measured against this script must fail BOTH lines.
cat > "$WORK/proxy.py" <<'PY'
import re, socket, sys, threading
ALLOW = sys.argv[1].split(","); PORT = int(sys.argv[2])
def ok(h): return any(h == p or (p.startswith("*.") and h.endswith(p[1:])) for p in ALLOW)
def pump(a, b):
    try:
        while True:
            d = a.recv(65536)
            if not d: break
            b.sendall(d)
    except OSError: pass
def handle(c):
    buf = b""
    while b"\r\n\r\n" not in buf and len(buf) < 65536:
        d = c.recv(4096)
        if not d: break
        buf += d
    m = re.match(r"CONNECT (\S+):(\d+) ", buf.split(b"\r\n",1)[0].decode("latin1"))
    if not m or not ok(m.group(1)):
        print("DENY ", m.group(1) if m else "?", flush=True)
        c.sendall(b"HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n"); c.close(); return
    print("ALLOW", m.group(1), flush=True)
    up = socket.create_connection((m.group(1), int(m.group(2))), 10)
    c.sendall(b"HTTP/1.1 200 Connection established\r\n\r\n")
    threading.Thread(target=pump, args=(c, up), daemon=True).start(); pump(up, c)
s = socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("0.0.0.0", PORT)); s.listen(64)
print("proxy up", ALLOW, flush=True)
while True:
    c, _ = s.accept(); threading.Thread(target=handle, args=(c,), daemon=True).start()
PY
echo "== 2. egress: only route out is the allowlist proxy =="
docker network create --internal $NET_IN >/dev/null
docker network create $NET_EX >/dev/null
docker run -d --name posse-probe-proxy --network $NET_IN --hostname egress \
  -v "$WORK/proxy.py:/p.py:ro" python:3.13-alpine \
  python /p.py "api.anthropic.com,platform.claude.com" 8899 >/dev/null
docker network connect $NET_EX posse-probe-proxy
sleep 2
docker run --rm --network $NET_IN $IMG sh -c '
  echo "  direct, by hostname   : $(curl -s -o /dev/null -m 8  -w %{exitcode} https://api.anthropic.com/v1/models) (expect 6: nothing to resolve — proves DNS only)"
  echo "  direct, by raw IP     : $(curl -sk -o /dev/null -m 8 -w %{exitcode} https://1.1.1.1/) (expect NON-ZERO — docker: 7. THIS is the route)"
  echo "  direct, raw IP github : $(curl -sk -o /dev/null -m 8 -w %{http_code} -H 'Host: github.com' https://140.82.121.4/) (expect 000; a leaking engine answers 200)"
  echo "  allowlisted via proxy : $(HTTPS_PROXY=http://egress:8899 curl -s -o /dev/null -m 15 -w %{http_code} https://api.anthropic.com/v1/models) (expect 401: reached Anthropic)"
  echo "  other host via proxy  : $(HTTPS_PROXY=http://egress:8899 curl -s -o /dev/null -m 15 -w %{exitcode} https://example.com/) (expect 56: proxy 403)"
  getent hosts api.anthropic.com >/dev/null || echo "  external DNS          : none (fail-closed)"'
docker logs posse-probe-proxy 2>&1 | tail -3

# ------------------------------------------- 3. herdr detection through docker
# herdr identifies the agent by the pane's foreground argv0, then scrapes the
# terminal. `docker run` renames the process and detection dies; a launcher
# whose argv0 is the runtime's canonical name brings it back. The scraping half
# already works: an OSC title printed inside a container reaches pane state.
# `exec -a` below is the probe's stand-in for what rangerhq-1k1 then built:
# state/cages/<persona>/bin/<runtime> → posse, which execs the engine with
# argv[0] reset (internal/rhq/cagelauncher.go).
if [ -n "$PANE" ]; then
  echo "== 3. detection: bare docker run vs argv0 launcher (pane $PANE) =="
  herdr pane run "$PANE" "docker run --rm -it $IMG claude" >/dev/null; sleep 10
  echo "  bare docker run  : $(herdr agent explain "$PANE" 2>&1 | head -1)"
  docker ps -q --filter ancestor=$IMG | xargs -r docker kill >/dev/null 2>&1; sleep 2
  herdr pane run "$PANE" "exec -a claude docker run --rm -it $IMG claude" >/dev/null; sleep 10
  echo "  argv0 = claude   : $(herdr agent explain "$PANE" 2>&1 | head -1)"
  echo "  argv0 seen       : $(herdr pane process-info --pane "$PANE" | sed 's/.*"argv0":"\([^"]*\)".*/\1/')"
  docker ps -q --filter ancestor=$IMG | xargs -r docker kill >/dev/null 2>&1
else
  echo "== 3. detection: skipped (pass an idle herdr pane id as \$1) =="
fi

# ----------------------------------------------------------- 4. VirtioFS cost
echo "== 4. VirtioFS build tax (bind mount vs container fs) =="
git -C . rev-parse --show-toplevel >/dev/null 2>&1 && REPO=$(git -C . rev-parse --show-toplevel) || REPO=$PWD
cp -R "$REPO" "$WORK/repo" 2>/dev/null || true
docker run --rm -v "$WORK/repo:/work" golang:1.26-alpine sh -c '
  cd /work && go clean -cache >/dev/null 2>&1
  echo "  bind mount (VirtioFS):"; time go build ./... 2>/dev/null
  cp -a /work /tmp/r && cd /tmp/r && go clean -cache >/dev/null 2>&1
  echo "  container fs:";         time go build ./... 2>/dev/null' 2>&1 | grep -E "bind|container fs|real"

# ------------------------------------------------------------------ 5. auth
# All three runtimes keep credentials in a file, not the macOS keychain
# (~/.claude/.credentials.json, ~/.codex/auth.json, ~/.grok/auth.json) — but
# claude's file on a keychain host is stale, and mounting it read-only kills
# token refresh. Expect "OAuth access token has expired" until the operator
# mints a container credential (CLAUDE_CODE_OAUTH_TOKEN via `claude setup-token`).
echo "== 5. auth: what a fresh container claude says =="
docker run --rm -e HOME=/home/agent $IMG claude -p "Reply with exactly: OK" 2>&1 | tail -2
