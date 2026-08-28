## the commit wall prescribes `git diff HEAD --`, not the two-dot form (ranger-base-erba)

rangerhq-lvu9's close (b537d84) added four lines to the L3 refusal naming the
residual — a path-limited commit takes the file as it is ON DISK, so another
persona's in-flight edit rides in under your message. The fourth line then
prescribed the wrong check:

    'git diff -- <paths>' first — a clean diff there is what makes the safe
    form actually safe.

`git diff -- <paths>` compares the **working tree** against the **index**. The
other persona's in-flight edit is very often *staged* — `git add`, no commit,
which is rangerhq-2f5r's own step 1 — and then tree and index agree, so that
diff is empty while their line is still exactly what your commit will take.
Measured on this box (git 2.39.3, darwin) against the rendered hook: clean
diff, `git commit -F - -- shared.txt` rc 0, `DINESH HALF-WRITTEN` in the
commit, no signal of any kind. `git diff HEAD -- <paths>` does show it
(`git diff --cached --` does too); only the bare two-dot form is blind. So the
message named the right hazard and then handed out a check blind to half of
it — the same over-promise lvu9 was filed to remove, one remove down.

**Fix (wording only).** The refusal now prescribes `git diff HEAD -- <paths>`,
says in so many words why the two-dot form is not it, and drops the
sufficiency claim for NOTES.md's honest one: even a clean diff, this form
bounds the **paths**, not the **content**; only a private worktree
(rangerhq-09o2) closes the rest.

**Pins** — `internal/rhq/commitwall_qa_test.go`, three tests over the real
rendered hook, and they hold each other's ends:

| test | what it holds |
|---|---|
| `…RefusalNamesTheInFlightEdit` | lvu9's DONE WHEN: the case is still named |
| `…TakesAnotherPersonasStagedLineUnderACleanDiff` | the residual is measured end to end, not asserted — permanent, since no refusal at this layer can close it |
| `…PrescribesADiffThatCatchesStagedWork` | this bead: the prescribed check sees a staged edit, and the sufficiency claim is gone |

Mutation-checked both arms of the third separately: restore the pre-fix
wording → both assertions fire; keep `HEAD` but re-add "a clean diff there is
what makes the safe form actually safe" → the second alone fires. So neither
half is riding on the other.

**Rendered, not installed.** The hook body is a source string; a repo hooked
before this commit keeps the old sentence until `posse gates install-hooks`
(or a persona launch) re-renders it. Nothing in the wall's *behaviour*
changed, so a stale hook is stale advice, never a stale refusal.
