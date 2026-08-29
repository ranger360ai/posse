## The L1 commit wall reaches the seed (ranger-base-09b7)

rangerhq-jgod put `Bash(git commit unless --)` in the eight PIDs the operator
hand-edited under `RHQ_HOME/agents/` (37d1479) and stopped there. It never
reached `examples/agents/`, which is what `//go:embed all:examples` carries
into every release binary and what `posse init` copies into a fresh instance
(embed.go, `seedSource` in init.go). So the wall's L1 half stood on the
crew's own nine files and on **no PID anyone creates from what the binary
ships**: laurie measured it on a fenced instance (posse 0.3.0+53c8cb6) and
`posse gates dev` printed four rows and no commit row at all.

What that costs is exactly what jgod wanted L1 for. A fresh instance does get
L3 — herdrback installs the `prepare-commit-msg` hook at every session create
— but L3 is a hook in a checkout. L1 is the half that lands on the **typed
line**, on every runtime, before git runs, and it is the only half that
reaches a repo where nobody installed a hook.

`posse agent new` was the same gap from the other side: the scaffold's
`deny:` block was entirely commented out, so every scaffolded persona
started with no deny at all.

**Three things changed.** All nine seed PIDs carry the rule; the scaffold
carries it as a real entry (the one list `posse agent new` no longer leaves
empty), with the reason in the comment above it; and
`TestSeededPIDsCarryTheL1CommitWall` reads `posse.Seed` — the embed, not
`../../examples` — because the embed is where the gap was. The two older seed
pins (`TestShippedPIDsDenyPromote`, `TestShippedPIDsDenyRefresh`) read the
directory, and in a checkout the two are the same bytes; a release binary
only has the embed.

**business-manager's blanket deny was the same bug wearing the other hat.**
It carried `Bash(git commit:*)`, which forbids `git commit -- <paths>` too —
the one form the wall is built to leave open. Advisory-by-construction is
`deny: Edit, Write`; the commit line is the crew-wide wall, and it is now
spelled the same way on all nine. The pin refuses any `Bash(git commit…)`
rule in a seed PID that is not the negative one, so the blanket cannot come
back quietly. hoover is the live precedent: read-only persona, `Edit`,
`Write`, and the negative commit rule — no blanket.

**Mutation-checked per PID, not once.** A green ban-list says nothing about
a file it never read, so each arm was killed and the survivors counted:
deleting the rule from each of the nine seed PIDs in turn kills the pin and
the failure **names that file** (9/9); putting the blanket back on
business-manager names it; making one PID `deny: Bash` outright trips the
`checked < 9` guard rather than passing by exemption; regressing
`deniesUnqualifiedCommit` to `false` fails all nine on the "L1 renders
nothing for it" arm; and deleting the scaffold's entry kills
`TestScaffoldAgentIsPID` on three assertions.

Editing an example PID means appending to `shippedExampleDigests`
(exampledigests.go) — append, never replace: an entry that leaves the table
is a home posse can no longer recognise its own file in. Nine new digests.

**A note on the second landing.** This change was written against 5939cb7 and
stranded there: the ff-only land refused, because ADR 0015 §3's amendment
(ranger-base-u9ud, 683d3f6) widened the same nine `deny:` blocks to bd's 23
destructive/egress verbs while it waited. Both changes are additive to one
list and both survive — the wall line keeps its place after the push denies,
u9ud's verbs follow — but the two bookkeeping files had to be resolved by
hand, and the rule there is the same "append, never replace" read carefully:
the nine pre-rebase digests described bytes **no release ever shipped**, so
they are not history the table owes anything to. The table keeps u9ud's row
and gains one row for the merged bytes. The check that decides this is
`TestShippedExampleTableCoversEveryVersionInGitHistory`: it walks
`git rev-list HEAD`, and a rewritten commit is not in it.
