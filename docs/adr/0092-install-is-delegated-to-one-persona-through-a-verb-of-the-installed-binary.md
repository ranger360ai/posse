# ADR 0092 — `make install` is delegated to one persona, through a verb of the binary already installed, and only for a commit already on origin/main

*Status: accepted 2026-11-13 — the operator approved it as written that day
("ADR 0092 APPROVED as written. Build it.", relayed on ranger-base-whnge;
marked under ranger-base-estjw, which also amends D6's gate field below) ·
proposed 2026-11-13 (ranger-base-whnge, from ranger-base-wi08c, on the
operator's ruling of 2026-11-13) · owner: architect · amends ADR 0015 §1 (the
mechanism repo's row: "install is human-run and persona-denied") and the
coordinator PID's "three keys" (promotions) · nothing here is live until the
operator promotes the PID and config drafts named in D7*

## Context

The operator ruled on 2026-11-13: delegate `make install` to the devops
persona, under five guardrails. (1) Only a commit already on origin/main,
which the coordinator pushed after `posse-land-check` and the vuln-private
check. (2) `make test` green on that exact commit. (3) Keep the previous binary
for a one-command rollback; after install, run `posse status` and
`posse gates`, and roll back on anything new and red. (4) One bead line per
install, with the commit range; the security persona re-verifies security
fixes afterwards. (5) `posse promote`, config, permission files and cage maps
stay the operator's. Why: 99 commits of security fixes sat uninstalled on
2026-11-12/13, and every re-verify stalled behind them (ranger-base-wi08c
alone blocks four beads).

What the fence is today (MEASURED 2026-11-13, this seat, posse 0.5.3+70ab74b7):

- `Bash(make install:*)` and `Bash(make deploy:*)` are denied in this repo's
  `.claude/settings.json`. That file is a permission file. No PID carries the
  deny.
- On this host every seat that demands seatbelt runs at shims
  (`cage_realize_linux: seatbelt: shims`). From such a seat, `test -w
  ~/.local/bin` and `test -w ~/.config/posse/gate` are both true. So the
  install fence is cooperative today: an argv deny, and no file wall.
- `make install` builds the HEAD of the checkout it runs in
  (`scripts/clean-build.sh`). In `~/src/posse` that HEAD is the launcher's
  `main`, which carries crew landings that have not been pushed. Today
  `main == origin/main`, but between pushes it is ahead. So `make install`,
  run as it stands, breaks guardrail 1 by construction.
- `install(1)` unlinks the target and then writes it (the Makefile says so).
  A launch that execs `~/.local/bin/posse` inside that window gets a missing
  or partial file. ASSUMED from GNU coreutils behaviour; not probed.

## Decision

**D1 — The delegate runs a new verb, `posse upgrade [<sha>]`, never
`make install`.** The deny on `make install` stays for everyone, the delegate
included. The verb is code inside the binary that is installed now. That
binary got there through this same gate (or by the operator's hand), so a seat
cannot weaken the installer by editing its worktree. A script under
`scripts/`, allowed by path, could be weakened that way (see Alternatives).

**D2 — Who may run it is config, not a PID line.** `posse upgrade` refuses
when `RHQ_PERSONA` is set and is not the persona named by a new config key,
`install_delegate:`. With no key, it refuses every persona. The operator, with
no `RHQ_PERSONA`, may always run it. This is the fence shape of ADR 0031 and
promote.go: one check, in the verb, read from promoted config. So no other PID
needs a new deny line. It is cooperative, like every fence of that family, and
its exit hatch is to delete the key.

**D3 — Guardrails 1–3 are steps of the verb, in this order. Each step refuses
before the next one runs.**

1. *Target.* The default target is `refs/remotes/origin/main` in the repo
   that config names as posse's source (today `~/src/posse`). `<sha>` must be
   an ancestor of it, and a descendant of the installed binary's stamped sha.
   Going backwards is a rollback, and a rollback is D4, not this verb. The
   verb never fetches. The remote-tracking ref moves when the coordinator
   pushes, and that push is the evidence for guardrail 1. The verb cannot see
   whether `posse-land-check` or the vuln-private scan ran. Being on
   origin/main is the record that the coordinator says they did.
2. *Test.* A detached worktree of `<sha>` in a fresh `mktemp -d`. In it,
   `make test` runs as the repo defines it: all three arms, suite slots and
   all. Red, or no answer, means refuse. There is no cached verdict, because
   ADR 0088's record is `posse-landed-check`, which runs `make test`'s
   prerequisites and not `make test`.
3. *Build.* `scripts/clean-build.sh` in that same worktree. So the binary is
   built from the tree the tests passed on, and is stamped with `<sha>`.
4. *Keep.* Copy the live binary to `~/.local/bin/posse.prev`. A `.prev` from
   an earlier install is overwritten: one level of rollback, as the operator
   asked.
5. *Swap.* Write `posse.new` beside the target, then `rename(2)` it over the
   target. The swap is atomic, so a launch never execs a half-written file.
   Like `install(1)`, it gives the target a new inode. Running processes and
   any bind mount of the old name keep the old inode, which is no change from
   today.
6. *Epilogue.* The same as `make install`'s: `verify-gate-freshness --warn`
   and `verify-hook-freshness`, both from the worktree.

A flock (`~/.cache/posse/upgrade.lock`) means two upgrades never overlap. The
verb runs for as long as `make test` runs, which is longer than one foreground
call. The delegate backgrounds it and waits on its pid (AGENTS.md).

**D4 — Rollback is one coreutils command, printed before the swap. The verb
also runs it itself on a new red.** The command is
`install -m 0755 ~/.local/bin/posse.prev ~/.local/bin/posse`. It does not go
through the new binary, because a broken new binary is exactly the case that
needs it. Before step 5 the verb snapshots `posse status` and
`posse gates <p>` for every persona in the roster. After the swap it runs both
again through the NEW binary, and rolls back when any of these hold:

- the new binary does not exec, or `posse version` does not name `<sha>`;
- a `posse gates <p>` that exited 0 now exits non-zero, or its parity block
  gains a DEGRADED or REFUSED row it did not have;
- `posse status` gains an URGENT line, or any line whose code is one the
  binary renders (G12 hooks, G17 cage map, the binary/launcher header). It
  must not merely be a line that moved since the snapshot.

`posse status` exits 1 on any shop with any open condition (MEASURED: it does
now), so its exit code cannot be the trigger. The trigger is a diff of the
condition lines keyed by code and subject. Conditions that move on their own
(G1 blocked seats, G3 question ages) are excluded by code. That code set is
ASSUMED complete here. The build bead must list every condition code by
source and pin the set.

**D5 — The bd argv gate copy is in scope (2tnbv's class), as a second arm of
the same verb, under the same guardrails.** The copy under
`~/.config/posse/gate/` is mechanism source from this repo, not a permission
file. What names it, the operator's `~/.claude/settings.json` PreToolUse hook,
is a permission file and stays the operator's. Its blast radius is the
binary's class: it fences every Bash call on the box. The arm installs
`scripts/bd-argv-gate.{sh,py}` from `<sha>`'s worktree only when the installed
copy differs. It keeps `.prev` copies of both, swaps each by rename, and
judges with `verify-gate-freshness`'s three behaviour arms (allow, deny,
untouched). It rolls back on any arm red. When the hook's path itself moves,
that is a settings edit: the verb refuses the gate arm and names the edit for
the operator.

**D6 — Guardrail 4 is the delegate's, and the verb hands it the line.** The
verb's last stdout line is
`upgraded <old>..<new> (<n> commits) · gate <word> · rollback: <D4 command>`.

*The gate word (amended at acceptance, ranger-base-estjw).* As drafted, D6
said `fresh|installed|unchanged`, the D5 build bead said
`fresh|installed|unchanged|refused`, and the D1-D3 build shipped
`fresh|stale|unknown`, because D5's arm was not built yet. Three vocabularies
for one field. `fresh` and `unchanged` both meant "the installed copy already
matched". The decided set, once D5 lands, is four words, one per outcome of
the arm:

- `unchanged`: the installed copy was already byte-identical to `<sha>`'s.
  Nothing was written, and the behaviour arms were green.
- `installed`: both files were swapped by rename, and the behaviour arms were
  green.
- `refused`: the arm wrote nothing because the hook path did not resolve or
  had moved (a settings edit, the operator's), or the arm wrote and a
  behaviour arm went red, so both `.prev` copies are back. The binary arm's
  outcome is not affected either way.
- `unknown`: `verify-gate-freshness` measured nothing (its exit 2).

`fresh` and `stale` are the verify-only words of the build that came before
D5. D5 retires them: `fresh` becomes `unchanged`, and `stale` becomes
`installed` or `refused`. Nothing parses this line. The delegate posts it on
the bead and a human reads it, so the retirement costs no reader.

For every install, the delegate posts that line on the install bead, and files
one `-l security` bead for the security persona. That bead names the range and
every bead id in it that carries `security` or `vuln-private`, by id only
(AGENTS.md, "Landing the plane"). Relaunching seats so they pick up the new
walls stays the coordinator's.

**D7 — Guardrail 5 is the edge of the delegation.** Out of scope:
`posse promote`, `posse refresh`, config and `install_delegate:` itself,
`.claude/**` and the operator's settings, cage maps, `posse gates
install-hooks` (repo hooks are an L3 wall install, and G12 lists them for the
operator), `install-detection` and `install-herdr-bob` (live herdr). The PID
draft sits in the instance tree's `docs/drafts/`, named on ranger-base-whnge. It
holds the PID's prose, the config key, and the coordinator PID's
three-keys sentence. The operator promotes it after D1–D5 land, and not before:
a PID that names a verb not yet installed is a promise the wall cannot keep.
At acceptance one more gate was added before the promote: the security
persona's review of the verb as built, which reads it as a whole rather than
close by close. The verb replaces the binary that fences every seat, so it gets
that whole read before a seat is allowed to run it.

## Consequences

- An install no longer waits on the operator being at the keyboard. It waits
  on a push and one `make test` (ASSUMED 15–25 min on a quiet box, from
  ranger-base-kiwvn's 280s over the three arms uninjected plus arm 1's
  `./...`; to be measured on the first real run).
- The delegate can change the walls that fence the delegate. Every persona
  could already do that through a landing; what stops it is guardrail 1. Code
  reaches the binary only after the coordinator's push. That is the same
  trust the operator's own `make install` placed in main, and less than
  `make install` places in an unpushed HEAD.
- The fence stays cooperative at shims (D2, Context). This ADR adds no file
  wall, and claims none.
- Exit hatch: delete `install_delegate:`. The verb then refuses every persona,
  and `make install` is what it was.

## Alternatives rejected

- *Allow `make install` in the delegate's PID, with the guardrails in prose.*
  It installs HEAD, not origin/main (Context). It needs a permission-file edit
  so that one persona escapes the project deny. And guardrails 2–3 become a
  checklist that a 99-commit backlog will skip.
- *A `scripts/install-landed.sh`, allowed by path.* This is the cheap one.
  But the allow matches the path in the seat's own worktree, which the seat
  can edit, so the guardrails can be edited out by the seat they bind. A
  script that checks its own bytes against origin/main is self-attestation by
  the code being attested.
- *The launcher auto-installs on every push (a systemd path unit on
  origin/main).* This is the clever one, and the one I wanted. It deletes the
  stall outright. But it is "nothing ships itself" reversed, with no bead line
  and no named actor. The operator ruled for a delegate with guardrails, not
  for automation.
- *Reuse ADR 0088's landed-check verdict for guardrail 2.* That verdict is
  `make test`'s prerequisites only. The operator asked for `make test`.
- *Gate copy out of scope.* That would leave 2tnbv's class as an operator
  ask forever, though its source, review path and blast class are the
  binary's. The line that matters, the settings file that names the hook,
  stays the operator's (D5).

## Measured versus assumed

MEASURED 2026-11-13 on this host at 0.5.3+70ab74b7: project deny of `make install`
and `make deploy`; seatbelt realized as shims; `~/.local/bin` and the gate dir
writable from a shims seat; `main == origin/main` at 70ab74b7 (0 ahead);
`posse status` exits 1 with open conditions; `posse gates <p>` exits 0.
ASSUMED: that the D4 condition-code exclusion set is complete.

D3.2's wall: MEASURED 2026-11-13 by the build bead, 11m57s for a green
`make test` (all three arms plus the tree-wide doors) at 983ae5d on this box
under normal suite-slot contention — a FLOOR for the verb, which adds a
`git worktree add` and may compile against a colder cache. The Consequences
section's ASSUMED 15-25 min stands as the envelope to plan against.
docs/notes.d/ranger-base-zhw0h.md has both readings.

`install(1)` IS non-atomic — MEASURED 2026-11-13 by the build bead
(ranger-base-zhw0h), GNU coreutils 9.12 on this host, 64 MiB over 64 MiB on
tmpfs with a reader stat-ing the target in a tight loop, three trials each
(tmpfs is this box's fastest surface, so these are floors): `install -m
0755` left the target ABSENT for 1,791-2,473 of ~84,000 observations and SHORT
for 12,606-13,384 of them, across ~10,600-11,253 distinct intermediate sizes,
over a 58 ms window — so it both unlinks and then writes in sequence, and both
halves are reachable by an exec. Write-beside-then-`rename(2)`, the same probe:
zero absent, zero short, one size, every trial. Scaled to this box's 17.8 MB
binary the window is roughly 16 ms per install. D3.5 stands as written, and the
figure is now a pin rather than a paragraph
(`TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort`).
docs/notes.d/ranger-base-zhw0h.md has the table.
