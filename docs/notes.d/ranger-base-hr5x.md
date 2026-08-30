## The refusal path may not use a verb it can refuse (ranger-base-hr5x)

`renderShim`'s `posse_refuse()` timestamped its `refusals.log` line with a
bare `$(date -u …)`. The shim dir leads the session's PATH by construction
(ADR 0009 §1), so under a PID carrying `Bash(date:*)` the refusal called
this persona's *own* date shim, which refused, logged, and called `date`
again. Each level forks a child and waits on it: unbounded in the source,
and a fork chain in practice — the shape that cost the fleet 2026-08-27
(ranger-base-f0ay).

`date` is resolved at render time now, exactly the way the shimmed binary
already was (`resolveOutside("date", binDir)`), and rendered as an absolute
path. Where it is not resolvable outside the gates at all, the line keeps
its shape with `-` in place of the time — losing the time is bounded,
reopening the loop is not.

**The general rule this is an instance of:** a refusal path must be
expressible in verbs the wall cannot refuse. Every other command in the
shim is a shell builtin or an `$RHQ_GATE_*` expansion; `date` was the one
real binary the refusal itself spawned, which is why it was the whole cycle.

**Testing it without reproducing it.** `TestShimRefusalNeverLooksUpDateOnThePath`
plants a decoy `date` at the head of the PATH the shim runs under, *in front
of* the shim dir — the reverse of production. The decoy does not recurse, so
the wrong arm costs one extra process instead of a fork storm, and the arm
where the PID denies `date` is safe on a live box. The decoy goes in the
child's `cmd.Env` only: on the process PATH it would be what render time
resolves and the test would be measuring itself. Its first call is the
control (a bare `date` on that PATH must reach it), so the assertion is
no-growth rather than absence.

Mutation check, with the bare form restored: 4 assertions fail — 3 decoy
calls where 1 is the control's, both log lines stamped `DECOY-TIME`, and the
rendered script carrying `$(date `.

**Residual, filed as ranger-base-l97n (P3) and since fixed there:** the gate shell's usercmd note
and the three L3 hook refusal lines still spell `date` bare. With the cycle
broken at the shim those cost one refused child each — measured: the
caller's line loses its timestamp and a `refused by posse gate: date` lands
on stderr, `refusals.log` gets one line, the caller exits 0.
