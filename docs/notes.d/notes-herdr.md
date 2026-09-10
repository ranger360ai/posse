## herdr substrate: upgrading the fleet's herdr

The step-by-step upgrade procedure for **this** deployment — pre-flight,
parking the armed dispatch loop, the handoff command and its failure prompt,
post-flight, and rollback — is an instance procedure, not any-deployer
mechanism, so it lives in the instance tree and not here (ADR 0024 Decision 1,
genre arm; operator ruling on `ranger-base-vbcs`, 2026-08-28). It was public
until then; git history keeps that copy. Private path label:
`docs/runbooks/herdr-upgrade.md` in the instance repo. Origin beads
`rangerhq-0ao` / `rangerhq-7t4` / `rangerhq-61u`.

What is any-deployer's about it is already stated in this file and stays:
`[[startup]]` hooks fire on a live handoff too and the loop must be asked, not
the workspace ("One loop, and the husk problem"); metas key on `workspace:` +
`socket:`, both of which a handoff preserves; and the id-recycling fence
below, which is what the runbook's post-flight leans on.

