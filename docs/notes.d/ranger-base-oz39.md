## `posse peek <lines>` refuses a bad count (ranger-base-oz39)

Three commands read a plain count out of argv, and all three used to drop
strconv's error: dispatch `-n` and `--timeout` (rangerhq-ytkl), prompt/wait
`--timeout` (ranger-base-sknr), and peek's positional `<lines>`. In every
one of them 0 is the deliberate escape hatch — no cap, herdr's own default,
the whole pane — so a dropped error did not fail closed. It failed *open*.

peek is the one where opening up means reading **more** than was asked for.
`PaneRead` tails client-side only when `lines > 0`, so `posse peek sess 40x`
returned the entire scrollback where the operator asked for forty rows, with
nothing said about the argument. Unparseable and negative counts now die
naming `<lines>`, before `Resolve`, so the argument is named whether or not
the session exists; `0` and the no-argument form still mean the whole pane.

The pin is `TestPeekLinesRefusesBadCount` beside its sknr sibling, on the
same rig: `RHQ_HERDR_BIN` at a file that is not there, so an accepted count
dies naming *herdr* instead. That is the positive control — a `validCount`
that refused everything would never reach herdr and the accepted rows would
go red — and it is what makes the refusal rows an ordering check too, since
a gate moved after `Resolve` lets them mention herdr.
