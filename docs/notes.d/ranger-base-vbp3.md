## The danger interstitial refuse that skipped every runtime it could apply to (ranger-base-vbp3)

`ranger-base-9r33` wired ADR 0013 §2's launch refuse and excluded
`Probe == nil` from it. That exclusion is every runtime the rule can newly
apply to, so the refuse went on being a printed sentence for exactly the
case the ADR was written about. This note is the close writeup; the rule
itself lives in ADR 0013 §2 and NOTES.md.

### The defect, measured

At `e3c5cc3`, a fixture matching the bead's own description — a
`runtimes/mycli.yaml` declaring a `danger:` screen, `PromptMode()` typed,
one ready bead — run through a real dispatch pass:

- `DangerLine(rt)` returned `""`;
- bd logged `update a-1 --claim`;
- herdr logged `workspace create`, then the work prompt typed into a
  session that opens on the screen;
- and `posse runtime check mycli` printed **LAUNCH REFUSE** about that same
  screen, in the same tree, on the same fixture.

### Why the exclusion swallowed the whole rule

A `Probe` is Go code, and `declaredInterstitials` never sets one — posse
cannot read an unknown CLI's config format. So *every* screen an operator
can declare is probe-less. Of the three built-ins, which do carry measured
probes: claude's one screen is `Seeded`, and codex and grok deliver by
argv. **The first typed-delivery runtime with a machine-mutating dialog is
therefore by construction a declared one** — probe-less, and excluded.

9r33's reasoning ("declaring a screen documents it and never walls the
declarer's own launches") protects a real case, and that case survives: a
declared screen **without** `danger:` still walls nothing. What changed is
that `danger:` is not posse guessing at a config it cannot parse — it is
the operator's own written statement that this screen's default action
mutates their machine. Refusing on it is refusing on a reading. Declaring
it is choosing the wall.

The probe-ran-and-could-not-read exclusion (`Silence.Unknown`) is
untouched; 9r33 measured that treating it as a "no" reds six unrelated
dispatch tests on a temp HOME, and a refusal whose own words are "cannot
tell whether the update menu is silenced" walls a box for something nobody
measured.

### The dead end, and what lifts it

A probe-less refusal cannot lift by silencing the screen: there is no probe
to read the key with. So both the refusal line and the `runtime check` grid
name the thing that does lift it — dropping `danger:` from the profile —
because a refusal an operator cannot clear from the line is a dead end.
An interactive launch still warns `DEGRADED` and proceeds (ADR 0015 §3),
which is how the operator answers the screen in the first place.

### Diverged from the bead's WHAT, three places

1. **Not gated on `PromptMode() == typed`.** The bead's not-touched list
   says argv sidesteps, measured — but 9r33 measured the opposite three
   days later: codex is argv, and it was launched onto the menu. Gating on
   typed would have re-opened the bug 9r33 was filed for.
2. **Not the dispatch path only.** `DangerUnsilenced` is one function three
   surfaces read. Narrowing the new branch to dispatch recreates the very
   disagreement the bead's DONE WHEN forbids.
3. **The silenced arm is split.** The bead asks for a fake typed runtime
   whose probe returns silenced; a declared runtime can never carry a
   probe. So the six-state table over a constructed `Runtime` holds the
   silenced, unknown, seeded and no-danger readings, and a dispatch-level
   silenced control over codex holds the end-to-end launch — the arm the
   9r33 dispatch test lacked.

`ranger-base-mzmv` (richard, architecture) carries the ADR collision: the
9r33 amendment is *newer* than this bead and said the opposite. Blast
radius today is zero — nothing shipped declares `danger:`.

### The tests, and the wrong arm each was checked against

| pin | mutation that reds it |
|---|---|
| dispatch refuses a declared `danger:` screen, and the identical profile without it launches | restore the 9r33 `Probe == nil` exclusion; drop the `Danger == ""` guard |
| `DangerUnsilenced` over all six readings | each of the above, plus treating `Silence.Unknown` as a no |
| dispatch-level silenced control for codex | (the control itself — it is what shows the refuse is not unconditional) |
| the three surfaces as an equivalence, both directions | un-block the preflight; drop the grid's "drop danger:" line |
| the busy-key split: two beads, one refusal, slot benched, nothing claimed | wrap the refuse as a `sessionFailure` (pane arm instead of persona arm) |

One measurement worth keeping, and now stated in the test itself: deleting
`launchSession`'s refuse leaves the **typed** pin green, because a typed
launch reaches `planLaunch` inside `CreateSession` and refuses there, above
the claim either way. The placement is load-bearing on the **argv** ladder,
which claims first, and
`TestQADangerousCodexInterstitialRefusesDispatchUntilSilenced` is the test
that reds when it moves.

Verified: full `internal/rhq` green at `c381956` (exit 0, fresh binary),
plus the repo-root audit package and `cmd/posse`; gofmt, vet and a whole
module build clean.

## Stranded, then re-landed (ranger-base-tq93)

The four commits above never reached `main`. `ranger-base-vbp3` was closed
on `posse/dinesh-posse-ranger-base-vbp3` and nothing merged the branch;
laurie's escape (ranger-base-tq93) measured it at main `c05980f` and
richard's ADR-adherence audit (ranger-base-4wxko, finding 2) re-measured it
at `088ddeb`. Both read `interstitial.go`'s `Probe == nil` skip still in
place and all four commits non-ancestors of `HEAD`, while ADR 0013 §2 on
`main` already carried **both** the amendment (applied verbatim by richard
in `d61e0ee` so it would merge clean whichever branch landed first) and the
`ranger-base-mzmv` ratification. The doc said fixed; the launcher did not.

Why nothing merged it back: `main` and the branch had **diverged**, and the
only conflicting file was `docs/adr/0013-runtime-dispatch-contract.md` — the
branch's §2 amendment against main's amendment-plus-ratification over the
same region. That is the [[closed-bead-commit-can-strand]] shape: a
fast-forward-only launcher cannot land a diverged branch, so the whole
change sat.

The re-land drops the branch's ADR hunk entirely — `main`'s copy is a
superset of it (the same amendment plus the ratification paragraph the
branch never had), so the conflict was never a decision, only a duplicate.
Everything else applied cleanly across the `internal/rhq` → `internal/posse`
rename (`631bda7`) with the paths rewritten: `interstitial.go`,
`interstitial_qa_test.go`, `runtimecheck.go`, `runtimepreflight.go`,
`NOTES.md`, `docs/notes.d/rangerhq-tr8k.md` and this file.

`internal/rhq` above is that package's pre-rename name; it is
`internal/posse` today, and the re-land was re-verified there.
