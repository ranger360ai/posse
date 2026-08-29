## The runtime probe: a template profile's `Bash(...)` denies are assumed until measured (rangerhq-66e2)

ADR 0017 engine-onboarding §1, rules 1–2. Until this landed, `parity.go`
counted a `Bash(...)` deny **realized** on any runtime that did not declare
`gate_shell: false` — including a CLI the harness had never seen. That claim
rests on three behaviours nobody had measured for such a CLI:

- **(a)** child commands inherit the typed line's `PATH`;
- **(b)** a runtime that re-execs a *login* shell resolves it from `$SHELL`
  — one that hardcodes `/bin/zsh -l` lets macOS `path_helper` push
  `/usr/bin` in front of the gates dir, and the L1 shim never runs;
- **(c)** it invokes the shell in argv shapes the gate wrapper parses.

(c) fails loud by design (ADR 0009 §1). **(b) fails silent** — it is exactly
the day the fleet believed L1 held on grok and it did not. For the three
built-ins the claim is probe-backed (ADR 0009's argv table, rangerhq-e43);
for a template-only runtime it was an assumption wearing a wall's clothes.

**The lock.** On a template-only runtime, every `Bash(...)` deny now lands in
the launch's **Degraded** list — "assumed, not measured — run `posse runtime
probe <name>`" — until a passing probe record exists. Standard waiver
semantics, because the degrade goes through `p.Degraded` like every other
shortfall: `--allow-degraded` waives it, tier `fast` never does. Built-ins
are exempt by *measurement*, not by privilege, and `gate_shell: false` keeps
its own diagnosis (a runtime with no wrapper would pass three observables
and still have no wall, so it is never sent to the probe). L3 still recovers
`git push` / unqualified `git commit` in a hooked repo — a git hook is not a
PATH lookup — and the line it prints now names the probe rather than a
`gate_shell:` key the yaml never set.

**The unlock.** `posse runtime probe <name>` shipped in the same change, on
purpose: refusing without offering the way out is the alternative the ADR
rejected, and a waiver typed by habit is not a waiver.

```
posse runtime probe <profile> [--timeout 4m] [--keep]
```

It opens its own herdr workspace, launches the profile with a **scratch PID
carrying a canary deny**, runs one turn, reads four observables and closes
the pane. It writes no session meta — nothing appears in `posse list`,
dispatch never sees it, the reaper never touches it. A probe is a
measurement, not a persona.

| # | observable | what fails it |
|---|---|---|
| 1 | `shim-precedence` | `command -v <canary>` inside the session resolves outside `gates/<p>/bin` — behaviours (a)+(b) |
| 2 | `refusal-logged` | the canary deny did not reach `refusals.log` through *each* of direct, `sh -c '...'`, an executable script |
| 3 | `unattended-turn` | the pane never settled, or settled having run none of the commands (a dialog nobody is watching) |
| 4 | `herdr-detection` | herdr saw no agent, named another kind, or only ever produced its idle **fallback** |

**The canary has to already exist on the system PATH** — `uname`,
`hostname`, `whoami`, `id`, `arch`, first that resolves. Observable 1 is a
*precedence* test: a command that lives only in the gates dir resolves there
whatever `PATH` says, so an invented canary would pass on exactly the
runtime this probe was written to catch. Each of the three shapes carries
its own marker in the canary's argv (`posse-probe-direct` / `-shc` /
`-script`), because "three refusals in the log" is also what one shape
firing three times looks like.

Observable 4 rejects herdr's `default_known_agent_idle_fallback`: herdr
answers `idle` for any pane holding a known agent, and a runtime that only
ever produces that is one dispatch is blind on.

**The record.** `state/runtimes/<name>/probe.json` — CLI path, version
string, date, canary, and every observable with its detail. Written whether
the probe passed or failed, because "measured and it does not hold" and
"nobody measured" are different facts and only the record tells them apart.
A record carrying fewer than four observables is **not** a pass: it has no
failures either, and the missing rows were never measured.

**Drift.** `posse runtime check <name>` compares the recorded version and
path against the CLI on PATH now and puts the claim back to `ASSUMED` on
either. A CLI that prints no version keeps its record — a live measurement
did happen — but every surface says the drift check is *not running* rather
than reading unknown as unchanged. There is deliberately **no expiry**: the
ADR keys currency on the binary, not on a number nobody measured.

**Verification.** `runtimeprobe_test.go` pins the four observables with both
arms and the drift machine through an injected reader seam;
`runtimeprobe_qa_test.go` drives production `CheckParity` / `RuntimeCheck` /
`RuntimeGaps`. The live half is opt-in and is the ADR's own checklist:

```
RHQ_LIVE_PROBE=codex go test ./internal/rhq -run TestLiveRuntimeProbe -v
RHQ_LIVE_PROBE=codex RHQ_LIVE_PROBE_FAKE=1 …   # must FAIL, naming observable 1
```

The `_FAKE` arm wraps the same CLI in a shim that hardcodes `/bin/zsh -l`,
which is the only way to build observable 1's wrong arm: it needs a real
login shell on a real box.

**Not in this change.** The ADR that specifies this — engine onboarding,
accepted 2026-08-22 — is not in this repo: it was written before the rename
and did not cross it, and the `0017` slot here holds the unrelated
runtime-equivalence ADR. Everything above was implemented from that text;
re-landing it under a free number, and repointing the `ADR 0017 §1` citations
in the code, the tests and INSTALL.md §8 at it, is filed on the architecture
lane.
