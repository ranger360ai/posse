# ADR 0046 — The constitution directory is `posse/`, not `rhq/`

*Status: accepted 2026-09-02 (ranger-base-woox9, operator ruling of
2026-09-02 that the rhq leftovers are swept, env var names excepted) ·
unbuilt: ranger-base-woox9's code, runbook and cutover beads (ids in the
bead's close comment) · number: 0043–0045 are pre-named by ADR 0040 §2 with
live build beads (hn32r, yv9uo, vl294); per 0040 §3.1 this file takes the
next number no bead has claimed rather than shifting three.*

## Context

The instance repo `~/src/ranger-base` holds the promoted set under one
directory: `rhq/agents`, `rhq/config.yaml`, `rhq/recipes`, `rhq/runtimes`,
`rhq/skills`, plus the never-promoted `rhq/personas` (memory, ADR 0015 §5),
`rhq/envs` (secrets, §7) and `rhq/state` (a machine-local relic, last
written 2026-08-28, before the home cutover). It is promoted into
`~/.config/posse`, which since ADR 0015 is a real directory with the same
relative layout. `rhq` was the product's old name; every other spelling
of it has been renamed or ruled to stay (the `RHQ_*` env var identifiers).
The directory is the last one nobody ruled on, and this bead is the proof
that the question recurs until someone does.

What actually reads the name — MEASURED at HEAD 6eafbaa unless marked:

| reader | where | what it does with `rhq` |
|---|---|---|
| `ConstitutionSourceDir = "rhq"` | `promote.go:89` | the ONE constant; `ConstitutionRepoPaths`, `ConstitutionRepoMarker` (`rhq/agents`), the hook's constitution arm (`gates.go` constitutionGuardBody), the launcher's land belt (`ConstitutionClassIn`) and the land refusal (`worktree.go:884`) all derive from it |
| the installed `prepare-commit-msg` in `~/src/ranger-base/.git/hooks` | rendered from the binary | tests for the marker on disk OR in the base tree; a copy stays as rendered until `posse gates install-hooks` re-renders it |
| `promoted.json` | `source: …/ranger-base/rhq` | read ONLY by `resolvePromoteSource`, which prefers it over config `constitution:`; the manifest's file keys are constitution-relative (`agents/dinesh.md`), so the launch verify (`VerifyPromoted`) never sees the directory name |
| `config.yaml` `constitution:` | the home | fallback source for a bare `posse promote`, behind the manifest's own record |
| `~/.config/posse/personas` | ONE symlink → `…/ranger-base/rhq/personas` | `RHQ_PERSONA_DIR` is `$RHQ_HOME/personas/<p>` through it; the seatbelt resolves it at every render (`seatbelt.go:1061`, into `state/gates/<p>/seatbelt.sb`) |
| `.gitignore` in ranger-base | two lines | `rhq/envs/`, `rhq/state/` |
| `examples/config.yaml:186` | the public example | `constitution: ~/src/<your instance repo>/rhq` |
| tests | 17 files, ~165 literal sites (`memoryland_qa_test.go` 43, `constitutionwall_qa_test.go` 31) | pin the hook body and the class spellings by literal |
| docs | posse: ADR 0015 §2/§3 and 0037 line 131; ranger-base: `README.md:19` | the rest of the 146 ranger-base hits and 108 posse hits are `internal/rhq/` package paths, `~/.config/rhq` home paths and quoted RCA/log lines: history, or a different sweep |

The bead's premise that the launch verify hashes `rhq/…` paths is false
(the keys above), and "11 PIDs' symlinks" is one symlink. That changes the
price: a rename does not put the fleet into the ADR 0015 §3 live-≠-promoted
refusal at all. What it does do is listed under Decision, item 3.

## Decision

1. **The directory is `posse/`.** `~/src/ranger-base/posse/<p>` promotes
   to `~/.config/posse/<p>`: the same name and layout on both sides of the
   copy, so the manifest's key `agents/dinesh.md` reads the same whichever
   tree you are standing in. It is the name of the tool whose layout the
   directory follows, which is what it is, and it claims nothing about
   the members that are not law (`personas/`, `state/`, `envs/`).

2. **One constant moves, nothing else in code is spelled.**
   `ConstitutionSourceDir = "posse"`. Every hook body, refusal string and
   class list derives; the tests that pin the derived spellings are
   rewritten to read the constant, with exactly one pin that asserts the
   constant's value is `posse` (a coupling pinned once, not a claim pinned
   165 times). `examples/config.yaml` and `.gitignore` follow by hand.

3. **The cutover is one operator sitting, in this order, with no
   `dispatch --watch` running and no session dispatched into ranger-base.**
   The order is forced by two facts: the hook marker is read from disk or
   the base tree, and promote's source comes from the manifest before the
   config.

   1. install the posse build carrying `"posse"` (the binary's land belt and
      hook render now name `posse/agents`; nothing is live until step 3);
   2. `mv rhq posse` in ranger-base (plain `mv`, not `git mv`, so the
      untracked `envs/` and `state/` ride along), fix the two `.gitignore`
      lines, one operator commit — a persona commit here would be refused
      by the old hook and is not wanted;
   3. `posse gates install-hooks ~/src/ranger-base` — the wall is closed
      again the moment this writes. Between step 2's commit and this line
      the constitution arm guards nothing in that repo, which is why the
      loop is stopped;
   4. `ln -sfn ~/src/ranger-base/posse/personas ~/.config/posse/personas`
      — from the instant of step 2 until this line every launch would find
      no `ORDERS.md` and the seatbelt's personas grant would resolve
      nowhere;
   5. `constitution: ~/src/ranger-base/posse` in `~/.config/posse/config.yaml`;
   6. `posse promote ~/src/ranger-base/posse` — the argument is
      mandatory this once: a bare promote reads the manifest's old
      `source` and dies "constitution not found" (loud, harmless, and the
      reason no code change is needed there). The manifest now records the
      new source and every later bare promote works.

   A session branch based before step 2 cannot land unattended afterwards:
   the launcher lands by `merge --ff-only` and main has moved. ASSUMED from
   git semantics, not measured.

4. **The `RHQ_*` env var identifiers do not move.** They never spelled the
   directory: `RHQ_PERSONA_DIR` is `$RHQ_HOME/personas/<p>` through the
   symlink, `RHQ_HOME` is the home. The ruling stands and needs no
   question bead.

## Consequences

- Every seatbelt profile and every refusal now spells one name for the
  tree and its home. The doc sweep of `rhq` elsewhere no longer leaves an
  island the seatbelt profile prints on line 14 of every session.
- `rhq/state` in ranger-base is a 6.7MB untracked relic of the pre-0015
  symlinked home; it moves to `posse/state` under step 3.2 and is a
  deletion candidate for the runbook, not this record.
- ADR 0015 §2 item 1–2 and §3's constitution-arm paragraph are stamped in
  place in the same commit as this record (0040 §3.3); the 0040 trim of
  0015 (ranger-base-mqoid) carries the new spelling and moves the cutover
  narrative to HISTORY.md as its row already says.
- Two leftovers found beside this one and NOT this record's:
  `scripts/queue-backup.sh:61` and `docs/runbooks/herdr-upgrade.md` in
  ranger-base still read `~/.config/rhq/…` — home-path spellings from the
  already-ruled sweep; the runbook bead notes them.

## Alternatives rejected

- **Keep `rhq/` and stamp it as the proper noun (the bead's option b).**
  Zero cost today. Ongoing: the operator ruled every other rhq spelling
  swept, so this would be the one kept on purpose, and it sits in the most
  visible places — the seatbelt profile, the hook refusal, the promote
  banner — beside a home called posse. The question was asked once this
  week already. A proper noun needs a reason to be one; "it was there
  first" is not one.
- **`constitution/`** — the clever one. The config key is `constitution:`,
  the Go names are `Constitution*`, the hook would print `class:
  constitution/agents` as English. But ADR 0015 already uses the word for
  the whole ranger-base repo ("the constitution repo") and for the class;
  a third meaning is one too many. And `personas/`, `state/` and `envs/`
  under it are exactly what §5 and §7 say is NOT constitution, so the
  directory would lie about half its members. `constitution:
  …/constitution` also hides which token is the key and which the path.
- **`hq/`, `fleet/`, `instance/`** — a new word for the tree is the
  spike trigger (inventing a name); every candidate has to be explained
  in every runbook, and `posse/` needs no explaining.
- **A hook that accepts both spellings through the window.** A
  two-spelling class is the rule assembly ADR 0040 §3.4 exists to end,
  for a window that is minutes long when the loop is stopped.
- **Prefer config `constitution:` over the manifest's `source` in
  promote, or refuse when they disagree.** The disagreement fires exactly
  once, at step 3.6, and already dies loudly with the old path in the
  message. Twenty lines of code to save one typed argument, and the
  manifest-first order is what keeps a stale config from promoting the
  wrong tree.
- **Rename the env vars with the directory.** Ruled out by the operator;
  and item 4 shows they were never coupled to it.

## Measured versus assumed

| claim | status |
|---|---|
| one constant, five derived readers, the launch verify blind to the name | MEASURED 2026-09-02, `promote.go`, `promoted.json` keys |
| one symlink, resolved at each seatbelt render | MEASURED, `ls -la ~/.config/posse/personas`, `state/gates/richard/seatbelt.sb:14` |
| manifest `source` beats config in `resolvePromoteSource` | MEASURED, `promote.go:531–541` |
| hook marker read from disk OR base tree; only `install-hooks` re-renders | MEASURED, `gates.go:3031–3032`, `hookfresh.go` header |
| 17 test files / ~165 literal sites; `examples/config.yaml` is outside the digest table | MEASURED, `grep -c`; `exampledigests.go` names no config.yaml |
| a pre-rename session branch cannot ff-land after the rename | ASSUMED (git ff-only semantics) |
| the guarded window is minutes with the loop stopped | ASSUMED; the runbook bead times it |
