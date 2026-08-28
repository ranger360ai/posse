## `posse init` does not arm the launch verify (ranger-base-h7cd)

ADR 0015 §3 checks the promoted set against `promoted.json` at every launch:
a **dispatched** session refuses on a mismatch, an interactive one warns
DEGRADED. No manifest means nothing was ever promoted here, and that is what
keeps every install predating the ADR launching.

`posse init` used to write a seeded manifest into **any** home that had none
— including one full of a constitution the operator wrote and never
promoted. Nothing failed at init time; the only output was the usual
`initialized <home> (seed: …)`. The trigger came later and elsewhere: the
next ordinary edit to `config.yaml` or a PID, after which every dispatched
launch hard-refused. Re-running `init` on an existing instance is the
advertised generics upgrade (INSTALL.md §7), so the path led straight at the
unattended fleet, and nothing connected the refusal to the init that armed
it.

**The rule now.** `initFrom` reads two facts about the home it *found*,
before it copies anything — the promoted set (`HashPromotedSet`) and the
manifest — and stamps only when both are empty. That is the honest reading
of "freshly seeded", and it is the caller's question, not
`SeedPromoteManifest`'s: once init has copied, the home it found cannot be
recovered from disk.

**Said out loud either way**, because an armed verify and an unarmed one are
the difference between a dispatch refusing and not:

```
initialized ~/.config/posse (seed: embedded)
stamped ~/.config/posse/promoted.json (seeded): every launch now hashes …
  `posse promote` is what re-stamps it after you change any of them
```

```
initialized ~/.config/posse (seed: embedded)
left this home unstamped: it already had a constitution, and a manifest init
wrote over it would arm the launch verify on prose nobody ratified (ADR 0015 §3)
  the verify stays off until you run `posse promote`; until then no launch is refused for it
```

**Homes an older posse already armed** keep their manifest — init never
removes one, and a trust anchor is not something a seed command deletes.
INSTALL.md §14 carries the two ways out: `posse promote` makes the anchor
true, or removing `promoted.json` puts the home back to unwatched, which is
where it was.

**What a seeded manifest still is not**: provenance. It attests to whatever
was on disk when it was written, an adopted generic included — which is why
`retireExamplePIDs` judges "posse shipped this file" against
`exampledigests.go` and never against the manifest (ranger-base-rgx0).
