## The refusal notice says what the bool says (ranger-base-co5n)

When posse's own `Bash(security:*)` shim refuses posse's catalog read, the
preflight prints one line on stderr — the one case its otherwise-silent
UNKNOWN branch is wrong for, because the misconfiguration is ours. That line
said `tier availability UNKNOWN, launches take the tier as asked` for every
refusal. Over a RETAINED catalog it was describing a launch that does not
happen: a failed refresh keeps the prior reading, `Models` returns it with
`known=true`, `TierPreflight` rules on it, and a launch can be demoted by it.
The operator read "posse has no idea" off a line printed by a preflight that
was about to substitute a model.

**Two sentences, chosen by the bool the caller acts on.** No snapshot: the
answer really is UNKNOWN and the launch takes the tier as asked — unchanged.
A retained reading: `tier availability is still the catalog read 2h00m ago,
launches rule on that reading`. The age is the whole decision the line
supports; minutes is a blip, and yesterday is a gate that has been refusing
since yesterday, on a catalog whose staleness the launch cannot see.

The fail-open asymmetry is untouched — nothing here moves a launch, and the
retained reading was already what `Models` returned. Only the sentence moved.

`kept(e, have)` is the expression `Models` returns its bool from, named once
and passed to the notice, so the two cannot drift apart; `catalogAge` returns
0 rather than a duration since the epoch for a snapshot with no `at` (or one
written by a clock ahead of ours), and the notice then names the reading
without an age.

Pinned in `internal/rhq/gatedkeychain_test.go`: the QA pin drives the whole
path through `Models` over the REAL rendered shim, and a table pins both
sentences at `noteGateRefusal` including the two arms a naive fix gets wrong
— a snapshot present but EMPTY is UNKNOWN like any other no-snapshot, and a
retained reading with no `at` still says which reading it is. Three
mutations, each verified red: the retained branch deleted (the QA pin and
both retained arms fail), `kept` forced true (the fresh UNKNOWN pin fails),
and the age not wired through the call site (only the pin that goes through
`Models` catches it — the table calls `noteGateRefusal` directly and cannot).
