# Two launches named, one openable (ranger-base-k2olp)

*2026-11-22 · gwart · why `posse new --agent X --cmd "…"` now refuses, why
the refusal sits at the flag boundary and nowhere else, and what the three
mutants that graded its pins found.*

## The claim that was not true

`posse new` takes `--cmd` and `--agent` independently. A persona launch
assembles its line in `planLaunch`:

```go
cmd := o.Cmd                 // the typed line
…
if o.Agent != "" {
    …
    cmd = ag.RenderCommandForModel(rt, own, tier, o.Model, launchWritableRoots(dir)...)
}
```

So the typed line is overwritten, and nothing anywhere says so. The session
record does not even keep it — `startPlanned` writes `cmd:` into the meta
only when `o.Agent == ""`. The rule was stated in one place, a field
comment, and enforced in none.

It fails SAFE, which is why this was P3: what runs is the PID's own rendered
line, with every launch guarantee the render appends — unattended mode, the
credential-dir settings pin, grok's `--no-leader`, and
`--strict-mcp-config` (ADR 0090 D1), each through its own door inside
`RenderCommandForModel`. A raw `--cmd` carries none of those, so discarding
it is the conservative direction.

## What it cost anyway

An unattended verify script. ranger-base-pepl9 host-checked ADR 0090 D1's
launch refusal — a line naming an `--mcp-config` file inside the seat's own
write set must not launch — and typed it as

```
posse new <s> --agent <p> --cmd "claude … --mcp-config <a file in the write set>"
```

It read back `created … not refused` and reported a security regression. The
line it was measuring never carried `--mcp-config` at all: posse ran the
persona's rendered line. The product was right, the report was wrong, and
the finding had to be re-derived from scratch.

That is the whole argument for a refusal over a warning, and it is the bead's
own finding rather than a preference. A warning on stderr is read by whoever
is watching; the point of such a script is that nobody is.

## The fix, and where it is NOT

`internal/posse/cmdagent.go`'s `CheckCmdAgent` — pure, argv in, error out,
the shape `CheckExactModel` established for a flag contract. It is asked from
`parseNewFlags` and from nowhere else, which is the one place this diverges
from that precedent (`CheckExactModel` is asked in `planLaunch` too, as the
wall for every launch path). The reason is that no other launch path can
carry the pair meaning it:

| path | carries both? |
|---|---|
| `posse new` | yes, and this is where it is refused |
| relaunch (`RecreateOpts`) | no — the meta keeps `cmd:` only for a persona-less session |
| dispatch | no — it sets no `Cmd` |
| a recipe (`LaunchRecipe`) | **yes**, `command:` beside `agent:` |

The recipe arm is the same silent discard one layer up and was deliberately
left alone: its refusal has to name `command:` and `agent:`, a recipe's keys
rather than two flags, and it would refuse a recipe that launches cleanly
today — MEASURED 2026-11-22 on this box, `~/homes/ranger/recipes/grok-fam.yaml`
declares `command: grok` beside `agent: grok`. Breaking a live launch is the
operator's call, not a seat's. Filed as ranger-base-g27p4, which
`CheckCmdAgent`'s own comment names.

## Grading the pins

`cmd/posse/cmdagent_qa_test.go`, over the BUILT BINARY for
`TestNewModelFlagRefusals`' reason: "exited 1 and created nothing" is a claim
about a process, and `parseNewFlags` cannot be called from a test at all —
its refusal path is `die`, which exits the test process.

Three mutants, MEASURED 2026-11-22 (this box, linux 7.2.15 x86_64, go1.27.3):

| mutant | verdict |
|---|---|
| `CheckCmdAgent` not called | RED — all four message assertions, plus the no-state walk |
| refusal downgraded to a warning on stderr | RED — but only on one assertion, and not the one it looked like |
| the `--cmd` help entry loses the rule | RED — three assertions |

Mutant 2 is the one worth writing down. The pin as first written asserted
`exit 1` and that the message named the rule, and **a warning passes both**:
the message is printed either way, and the launch then exits 1 on the next
thing it hits — in this fixture, `no such agent: architect`, because the
temp home has no personas. Only the no-state walk caught it, and that was
luck: it caught a `state/dispatch-launch.lock` the launch took on its way
past. So the pin now asserts the thing an exit code cannot show — the output
must NOT contain `no such agent`, because posse reading the home at all is
proof the pair got past the flags.

The third row of that test (`--cmd` alone, no `--agent`, which must stay
legal) runs against its OWN `RHQ_HOME` for the same reason, found the same
way: getting past the parser is its whole claim, and a launch that gets that
far writes — sharing the home put that lock file under the no-state
assertion, which then measured the legal row instead of the refusals.
