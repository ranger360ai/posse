## Cage engine re-evaluation: still Docker (rangerhq-rli)

The 89a spike recommended Docker on the grounds that it was installed and
answered every question, and handed the two alternatives forward as a
decision about whether they were worth a machine change. Re-evaluated
**2026-08-23**; verified live against each project's own tracker on that
date, and the one executable half measured on this host (macOS 26.4.1,
Docker 29.0.1). **The engine stays `docker` and no `cages/<name>.yaml`
was written.** Neither candidate earns the change today, and the reasons
are different in kind.

**Apple `container` — the topology is expressible; it does not hold.**
Version 1.2.2 (2026-08-08); 1.0.0 landed 2026-06-09. Not installed here,
and deliberately not: the blocking defect is reproduced upstream with
exact commands, so installing to re-confirm it buys nothing.

The two questions 89a said would re-open:

| re-opened question | answer, 2026-08-23 |
|---|---|
| does a host unix socket survive its mount layer? | **yes, and by a better mechanism than docker's.** `container` does not virtiofs-share the socket — `container-runtime-linux` runs a socket **relay**, and 1.1.0 (issue #1750, PR #1751 "Propagate permissions for all host-to-container socket mounts") made it carry the host's permission bits so non-root workloads can use it. The single-file virtiofs limitation that would otherwise sink `sockets: [herdr]` (apple/containerization#79 — "virtiofs doesn't support sharing single files") does not apply to sockets. Unmeasured here; nothing is installed |
| how is "internal network whose only route is an allowlist proxy" expressed on vmnet? | **exactly the way we already express it — and it leaks.** `container network create --internal` exists (`"mode": "hostOnly"`), and upstream discussion #1170 describes the same dual-homed-proxy shape posse built. But **apple/container#2062** (open, filed 2026-08-03, reproduced on the signed 1.2.0 release): a `hostOnly` network **NATs arbitrary outbound TCP**. Raw-IP HTTPS to GitHub answers `HTTP 200`, and `1.1.1.1/cdn-cgi/trace` reports the host's public IP — identical to the default NAT network. UDP and ICMP are blocked. Fix PR **#2072** is open and unmerged as of 2026-08-13, so it is in no release |

Stated as our own property: at L4 the thing that makes the cage a
boundary is *an agent that ignores `HTTPS_PROXY` reaches nothing*. On
Apple `container` today an agent that ignores `HTTPS_PROXY` reaches any
host **by IP**. `posse cage <persona>` would print the same effective
allowlist and the cage would be politeness. `SpellsEgress` asks only
whether an engine can *spell* the route; it cannot know whether the
engine holds it, which is why the probe below matters more than the
template.

Two more, both from upstream's own tracker:

- **The bead's hypothesis — "egress control moves to host `pf` rules" — is
  answered no.** apple/container#1320 (open since 2026-03-17): "macOS `pf`
  firewall rules don't seem to filter vmnet-bridged traffic", and
  guest-side iptables "is bypassable by a root process inside the VM".
  The same issue records that on a `hostOnly` network the **host gateway
  stays reachable**, so any host service bound to `0.0.0.0` is in reach
  from inside the cage — something docker's `--internal` does not give
  away. Its filer's use case is, word for word, sandboxing autonomous AI
  coding agents.
- **Posture.** 1.2.0 shipped four security advisories, one of them
  host-file disclosure out of the build context via symlinks
  (CVE-2026-64777). An engine adopted *for* isolation, ten weeks past
  1.0.0, with an open isolation defect in its isolation flag, is not
  where a security tier moves.

Re-open when #2062 ships in a release — filed as a bead, so it is not
left to anyone remembering.

**OrbStack — no engine work to do, and no measured problem to solve.** It
answers docker's CLI, so the **built-in `docker` template already is its
template**: the swap is an install and a `default_engine:` that does not
even change, not a `cages/orbstack.yaml`. What OrbStack sells over Docker
Desktop is file-sharing speed — and the tax it would cut is the one 89a
measured at ~8% on a cold `go build ./...` of this repo (4.84s
bind-mounted vs 4.49s on the container fs; the *host* is slower still at
5.33s). That is noise, and ADR 0002 had already dressed that number as
larger than it is. Its commercial licence is the operator's line and it
is not worth asking to fix an 8% nobody has felt. If the operator ever
installs OrbStack for their own reasons, posse needs no change.

**What the re-evaluation actually found was in our house.** Our boundary
check had upstream's bug. Probe 2 of `0002-container-tier.probe.sh`, check
1 of `TestLiveEgressBoundaryIsTheRouteNotTheEnvVar`, and the header of
`internal/posse/egress.go` all rested on one assertion:

```
curl https://api.anthropic.com/v1/models  →  exit 6
```

curl's exit 6 is **"couldn't resolve host."** It proves the resolver is
gone. It says *nothing about the route* — which is the identical false
positive that let apple/container's own `testIsolatedNetwork` pass over a
live leak for months. An engine that blocks UDP/DNS while still NAT-ing
outbound TCP would have passed every check we had.

Measured on this host 2026-08-23, docker 29.0.1:

| from a container | on the cage's `--internal` net | on the default bridge |
|---|---|---|
| `https://api.anthropic.com` (hostname) | curl exit 6 | — |
| `https://1.1.1.1/` (raw IP) | curl **exit 7** | exit 0, **http 301** |
| `https://140.82.121.4/` `Host: github.com` | curl **exit 7** | **http 200** |
| `http://8.8.8.8:53/` | curl exit 7 | — |

Docker's `--internal` is sound: it takes the route, not just the
resolver. The claim in this file was right; only the test could not tell
the difference — and the right-hand column is what makes exit 7 mean
something. All three sites now assert the raw-IP result too, non-zero
rather than exactly 7 (docker refuses and gives 7; an engine that drops
would give 28 — both are the boundary holding, 0 is the boundary gone).
`TestLiveEgressBoundaryIsTheRouteNotTheEnvVar` passes with the new checks
against a freshly built `posse-cage:latest`.

**So the deliverable of an engine re-evaluation was not an engine.** It
was the probe that can tell a real boundary from a DNS outage — which is
what any future candidate now has to survive.

Also measured while verifying, since the numbers are in this file: `posse
cage build .` over a harness checkout takes **11s** with a warm layer
cache (89a's ~45s was cold) and the image is **1.23GB** on disk by
`docker images`. The image was removed afterwards; `posse cage` reports
"image not built" here, as it did before.

