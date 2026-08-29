## The autostart hook's by-hand run said "already running" about a husk (ranger-base-oej)

During F8 (2026-08-23) `plugin/autostart.sh` printed

    dispatch autostart: dispatch already running — left alone

twice, with no loop process and no pidfile. The workspace herdr had restored
without its command was still wearing the name, so `posse new dispatch`
refused it, and the hook read that name refusal as a live loop. `posse kill
dispatch` cleared the husk and the next run armed clean — but nothing in the
two lines the operator saw said husk, and nothing named the lever.

**Half of the bead was already fixed by then.** The complaint filed it as an
ordering bug — a leave-crew-alone rule firing ahead of the liveness check —
and rangerhq-gir5 (de67d7f, 2026-08-26) reordered it for other reasons: the
hook now asks the kernel about `state/dispatch-watch.lock` *first*, and only
falls through to `posse new` once the lock has answered `none`. Prove-death
first is the shape on main.

Which makes the surviving line worse, not better. Reaching that branch means
the hook has *just measured* that no loop holds the lock; saying "already
running" there contradicts its own evidence, and exit 0 reported an arm that
did not happen. So the by-hand branch now says what it measured, names the
lever, and exits nonzero:

    dispatch exists but no dispatch loop holds the lock — left alone, nothing armed
    a herdr workspace outlives its command; this one wears the operator's crew
    mark 👤 (posse new stamps it), so no sweep clears it — run 'posse kill
    dispatch' and re-run this hook, or restart the herdr server …

**The crew mark is not a rule in the hook — it is why the husk persists.**
Nothing in the hook, and nothing in `posse kill`, treats crew as a reason to
leave a session alone; the mark is on the dispatch workspace because the hook
creates it with `posse new`, which stamps `crew: true` (ADR 0008, main.go's
`new` case). That is exactly what puts it outside every sweep, so a husk of
this one session sits there until a human kills it. Naming the mark in the
refusal is the whole cure the bead asked for on that side.

By hand still never kills: the name may be worn by a workspace the operator
is sitting in. `--startup` still replaces a husk itself, and now quotes the
kill when the replacement fails — a `posse kill` can refuse (ADR 0013 §4's
reap guard, the foreign-workspace refusal) and that reason used to go to
/dev/null, leaving `still present after kill — not started` naming no lever
either.

Pins in `internal/rhq/autostart_test.go`, all four against the real flock in
a temp `RHQ_HOME` with `new`/`kill` scripted:
`TestAutostartByHandNameTakenWithNoLoopIsNotAlreadyRunning` (the defect —
red on the old hook on all five assertions),
`TestAutostartHuskReplacementQuotesARefusingKill` (the swallowed refusal —
also red on the old hook), plus the positive controls that must stay green
either way: `TestAutostartByHandOverALiveLoopStillReportsSuccess` (a real
loop is still "already running — left alone", exit 0) and
`TestAutostartByHandNeverKills`.

`posse status` reports the same fact from the other side as G7 `loop-dead`
(govern.go) whenever autostart is armed and the lock is free. It is left as
it is: it cannot tell whether a husk holds the name without asking herdr,
and the hook is where that question is already being asked.
