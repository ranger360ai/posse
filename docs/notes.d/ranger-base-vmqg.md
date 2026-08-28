## NoSource is guard-off, not blind (ranger-base-vmqg)

ADR 0019 D3, wired. A `(runtime, purpose, platform)` with no credential
store is UNCONFIGURED — the plan guard runs off, says so once, and the blind
clock never starts. Blind keeps its ADR 0018 meaning untouched: *a source
exists and the read failed*. Parking a fleet on structural absence is a
brake with no release — no reading is ever coming to lift it — and a Linux
box that has never run `claude` would have held it forever.

**A NoSource arrives two ways and the guard must not care which.** The
availability check catches it before a reader is built (`PlanAdapter` →
`MeterUnavailable`), and the READ catches it when the store went away after
that check or when a caller supplied the reader and no check was ever made.
Before this, the first arrival was already guard-off and the second was
blindness: two code paths deciding a fleet's fate on which one noticed
first. `NoSourceReason(err)` is now the one reader of both, and the three
sites that fork on it — `planGuard`, `planNoAdapter`, the cockpit's
`planOffState`, plus G5 in `govern.go` — all ask it.

The availability arrival needed `NoPlanAdapter` to keep its reasons as
VALUES (`Errs []error`), not only flattened into `Why`. A reason that
survives as a substring of a sentence is a reason no caller can fork on.
`soleNoSource()` reads them by the rule that matters and is deliberately
not an `errors.As` traversal: structural absence is the answer only when it
is the WHOLE answer. With a second adapter, one lacking a credential and one
lacking an endpoint, "one login arms this" would be false — one adapter
ships today, and the rule is written for the second, because the second is
when the wrong sentence would appear.

**Two guard-off sentences, because the operator's next move differs.** "No
plan-window adapter serves this machine" reads as *posse does not support
your provider* — a wall. What is actually there is a platform, the store
that platform would need, and one command that writes it, which is what
`*NoSource` carries and what `planUnconfigured` prints. The cockpit header
says which of the two states it is at a glance: `plan — · guard off, no
credential source` beside the existing `… no adapter`.

G5 (`posse status`, "monitoring itself is broken") is in scope for the same
invariant one surface over: it reads the same error, and the availability
arm returns `nil, nil` before it, but the read arm reached it and would have
raised an URGENT at a machine with nothing broken about it.

Pinned in `internal/rhq/nosourceguard_test.go` and two cases in
`cmd/posse/cockpit_test.go`. Every pin asks the same three things of a pass
run unattended, three hours past a ten-minute blind budget — the state in
which blindness parks: the bead still went out, `blindFailed`/`planBlind`
stayed clear (those two ARE the park), and the line names platform, store
and fix while never saying "no plan-window adapter". The codex/grok case
takes its error from `MeterToken` rather than a literal, so a change to what
the seam returns lands there. Seven mutations verified red: each of the two
dispatch branches removed, `Errs` not collected, the G5 guard removed,
`soleNoSource`'s every-rule relaxed to any, the cockpit classification
collapsed, and the cockpit segment printing the adapter state's words.

Untouched and deliberately: `modelavail.go`'s tier preflight also reads the
meter credential, and a NoSource there is already benign — every read
failure is UNKNOWN, UNKNOWN launches what it was asked to launch, no clock
and no park. It is silent about it, which is that file's documented
fail-open design and not this bead's to change.
