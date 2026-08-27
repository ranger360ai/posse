## Cage engine watch: the #2062 leak has a host precondition (rangerhq-bnvk)

Watch-bead check, **2026-08-27**. **The trigger has not fired and the engine
stays `docker`** — nothing in the rangerhq-rli decision is re-opened here.
Measured against the trackers on that date:

| gate | state |
|---|---|
| apple/container#2062 (the leak) | **open**, last touched 2026-08-25 — a "any progress?" ping, no fix |
| apple/container#2072 (the fix PR) | **open, unmerged** |
| latest release | **1.3.0**, 2026-08-24 — its changelog carries no networking or isolation change; the leak is in no shipped build |

What the check did turn up is a **correction to how the defect is written
down**, and a false pass hiding in our own instrument.

**The leak is conditional, and rli's row states it unconditionally.** The
NOTES.md "Cage engine re-evaluation" table says a `hostOnly` network NATs
arbitrary outbound TCP, flat. The upstream thread (2026-08-05) qualifies it:
the reporter's host had `net.inet.ip.forwarding: 1`, and Apple's own
maintainer **could not reproduce the leak at the macOS default of 0** —
outbound TCP times out. The host is what NATs the packets off the `vmnet`
interface; with forwarding off there is nothing to NAT them.

**This does not rescue Apple `container`, for three reasons that are worth
separating from the sympathy the correction earns it.**

1. **The precondition is an unguarded host sysctl, not a property of the
   engine.** The reporter's own note is that forwarding "is often flipped on
   silently by VPNs, Internet Sharing, or other dev tools". A cage whose
   isolation holds only while a one-line `sysctl` nobody owns stays at its
   default is not a boundary — it is a boundary-shaped coincidence, and it
   fails silently and invisibly at the moment someone connects a VPN.
2. **apple/container#1320 is unconditional and untouched by this.** On a
   `hostOnly` network the host gateway stays reachable, so host services
   bound to `0.0.0.0` are in reach from inside the cage, and macOS `pf` does
   not filter vmnet-bridged traffic. No sysctl state changes that.
3. **It is still not shipped.** A conditional defect in an unmerged fix is
   the same non-event as an unconditional one.

**The instrument had a false pass, now closed.**
`docs/adr/0002-container-tier.probe.sh` gains a **probe 0** that reads
`net.inet.ip.forwarding` and prints the verdict qualifier before anything
else runs. Without it, the raw-IP line rli added — `expect 000; a leaking
engine answers 200` — answers `000` on any host at the default forwarding=0
*for the host's reason, not the engine's*, and a future re-evaluation would
read a clean isolation pass off a leaking engine. That is precisely the
failure rli built the raw-IP line to prevent, one layer further down: it
fixed "curl exit 6 proves nothing about routing", and this fixes "000 proves
nothing about the engine". A probe whose wrong arm does not fail does not
discriminate.

The guard classifies rather than warns: at **0** it states that docker is
unaffected — docker's `--internal` is enforced inside the Linux VM, not by
macOS vmnet, so probe 2 stands exactly as written — while a **vmnet-backed
engine measured there is VOID**; at **1** the leak condition is live and such
an engine is being tested fairly; and a non-numeric or unreadable `sysctl`
lands in **`unknown`**, never silently in "off". All four paths exercised
against a stubbed `sysctl`.

**So the re-open trigger gains a second half.** When #2062 ships, do not
merely re-run the probe — re-run it **on a host with forwarding=1**, or the
pass means nothing. The probe now says so itself, at the top of its own
output, where the person running it cannot miss it.
