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

*Amended by the build bead (ranger-base-v87a1, 2026-11-13): the code set is
MEASURED, and found a code this section did not name — G7, whose `arm-broken`
keys read a static config through this binary's own interval grammar. The
"keyed by code and subject" sentence is realized as a count per (class, code)
and not as a subject-keyed diff, because the two sides of this diff are two
different binaries and neither the printed detail nor `GovCondition.Key` is a
stable interface between them. See "Measured versus assumed" below.*

*Amended by the architect (ranger-base-qfhph, 2026-11-13): the hook wall is
judged by BEHAVIOUR, never by identity.* The third bullet named G12 as
binary-rendered, and that was wrong. A git hook in a repo is a COPY. This verb
leaves the copy behind on purpose (D7), so "the copy is not this binary's
render" is the designed state after every install that changes the render. It
says the install's second half has not happened yet. It does not say the new
binary is broken. What D4 wants to know about the hook wall is whether the NEW
binary's render still refuses. That is ADR 0023's behaviour half, and it is a
property of the binary alone, the same in every repo. So:

1. **G12 joins G11 in the install-subject class.** A G12 row after the swap
   is not a trigger. It is how the operator learns that a re-render is due,
   which is the job D7 already gave it. When the after-reading has G12 rows
   the before-reading did not, the verb's report prints the
   `posse gates install-hooks <repos>` line for the operator. It does not run
   that line.
2. **The gates arm stops reading hook identity too.** `posse gates <p>`
   probes the L3 hooks of its CWD (parity.go `applyL3Probe`, identity AND
   behaviour). The verb's probe inherits the delegate's cwd, and a delegate
   in a worktree of a declared repo reads that repo's common hooks dir. So
   the second bullet tripped on a render change exactly as G12 did, and
   excluding G12 alone would have fixed nothing. The gates readings now run
   with their cwd fixed to the source repo D3.1 names. That way both sides
   read the same tree wherever the delegate typed the verb. The not-held
   count leaves out every row that names an L3 hook slot (`pre-push`,
   `prepare-commit-msg`), and does not count the DEGRADED header by itself.
   A report whose only not-held rows are L3 rows counts zero.
3. **Behaviour is asked directly, through one new read-only verb.**
   `posse gates render-probe [dir]` runs only the behaviour half
   (`execOwnRenders`) against `dir`, which defaults to the source repo. It
   writes no hook and uses only its own private temp dir. It exits 0 when
   every render refuses what it must, 1 when one does not, and 2 when it
   could not ask. The trigger: the NEW binary answers anything other than 0,
   unless the old binary gave that same non-zero answer before the swap (a
   box property such as a broken `/bin/sh`, which is reported and is not a
   reason to roll back). An old binary that does not have the verb answers
   neither 0 nor 1. That reading counts as "not asked", so the new binary
   must answer 0. A pin keeps the verb in every later binary, because the
   OLD binary's code is what calls it.

   *Amended by the build bead (ranger-base-apf87, 2026-11-13): an old binary
   without the verb answers **1**, not "neither 0 nor 1" — MEASURED, see
   "Measured versus assumed". So "not asked" is read off a MARK on stdout
   (`render-probe:`, opening every line this verb prints) and never off the
   exit code, which is the only way to tell that old binary's `no such
   agent` from the verb's own "a render does not refuse". The trigger and
   the three exit codes are otherwise exactly as written above.*

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
- *Judge the hook wall by identity (D4 as first written), by excluding G12
  alone, or by letting the verb re-render the repo hooks.* See
  ranger-base-qfhph's paragraph under "Measured versus assumed". The first
  refuses every landing that changes the wall. The second is incomplete,
  because the gates arm carries the same probe. The third widens the
  delegate's writes into four shared checkouts so that one trigger reads
  cleanly.
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

D4's condition-code exclusion set is no longer ASSUMED — MEASURED 2026-11-13
by the build bead (ranger-base-v87a1), by census over internal/posse's own
shipped sources: **18 rows**, G1-G17 plus the one row name the three
pulse-era carry-overs share, in five files (govern.go, launcherlag.go,
memoryland.go, queuejsonl.go, verifybox.go). The question asked of each was
this section's own — does the row's store of record move while the verb's
`make test` runs? Fourteen do and are excluded. **G12** (the hooks dirs,
judged against this binary's own render) and **G17** (a static promoted map,
judged by this binary's own cage resolver) are the two this section named.
**G11** is excluded for a reason of its own: it is ABOUT which binary is
installed, so a good install can deepen it when origin/main moved further
forward than the install did while the suite ran.

**The measurement found a third code this section did not name: G7.** Its
`loop-dead` and log-mute keys are a flock and an mtime, but its `arm-broken`
keys read `autostart_interval:` and `autostart_via:` out of a config file
nothing touches during an upgrade and refuse them through *this binary's*
`ParseInterval` and `AutostartVia`. A new binary whose interval grammar
narrowed would disarm the fleet's only instrument — URGENT, and silent by
construction — and under the assumed set that install would have landed
clean. A code is classified by its most binary predicate, because the two
errors are not the same size: a false rollback costs one re-run of the verb
and leaves the fleet on the binary it was already running, with the reason
printed; a missed one leaves a broken binary dispatching a live fleet. The
price is named in the code — a watch loop that dies, or goes mute, inside the
suite window rolls the install back and says so.

The partition is pinned by `TestQAEveryStatusConditionCodeHasAnUpgradeClass`
(door `make pkgcensus-check`), whose file set is discovered rather than
spelled, so a row added tomorrow in a file that does not exist today still
reds it until somebody classifies it.

**D4's trigger is a COUNT per (class, code), not a diff keyed by subject** —
the one place the build bead reads this ADR differently from the way it is
written, and the reason is that the two sides of the diff are two different
binaries. The subject lives in `GovCondition.Key`, which `posse status` does
not print; the only subject a reader of that output has is the DETAIL, which
govern.go documents as the half that may move freely ("percentages, ages —
nothing keys on it"). Diffing it would roll back a good install because a
watch log aged four minutes between the two readings, or because a landing
reworded a sentence. Keying on `Key` itself would not help even if it were
printed: G11's key carries a lag BUCKET and G7's a log-age class. What is
stable across two binaries of this tree is the pair this ADR itself names —
the class (URGENT | LANE) and the row (G1…G17, or the carry-over's em dash).
The cost is stated rather than hidden: a SUBSTITUTION inside one code, the
new binary clearing one G12 row and raising a different one in the same
breath, leaves the count unchanged and does not trip. A binary that makes a
wall worse makes MORE rows, which is the shape that trips.

**D4 and D7 conflict over G12, and the build left D4 as written.** Filed as
ranger-base-qfhph. G12's predicate is byte-for-byte identity between each
repo `beads_visibility:` names and *this binary's* own render of the git hook
(hookfresh.go, `l3AskIdentity`); MEASURED on this box 2026-11-13,
`verify-hook-freshness` reports all four repos matching the installed
binary's render and no G12 row stands. So a commit that changes the hook
render makes every declared repo read "ours but stale" the instant the new
binary is in place — and D7 keeps `posse gates install-hooks` out of this
verb's scope on purpose, so the designed sequence is install, G12 goes stale,
the operator re-renders. D4 reads step two as a new red and undoes step one:
the verb refuses exactly the landings that change the wall. And
`l3AskIdentity` cannot be the evidence D4 wants even in principle, because it
drops the behaviour half on purpose — identity alone cannot tell a binary
that renders a BROKEN hook from one that renders a DIFFERENT one. Two
resolutions, both the architect's: D4 excludes G12 and classes it with G11;
or D7 lets the verb re-render the repo hooks as a third arm beside D5's gate
copy, under the same keep/swap/rollback discipline.

**Resolved by ranger-base-qfhph (D4's amendment above), and neither option
was taken as offered.** Read from the code at 2dc8dd75: the conflict was
wider than G12. `posse gates <p>` asks the same identity question of its cwd's
hooks, and the verb's probe inherits that cwd. (a) on its own would have left
the gates arm rolling back every render change whenever the delegate stood in
a worktree of a declared repo. It would also have left the renderer unwatched
by anything the verb reads. (b) would make the triggers mean what D4 wants.
But it makes the delegate a writer of the commit wall in four shared
checkouts, one of them the constitution repo, and it puts a four-repo
re-render inside the rollback. That is a lot of reach to buy one probe. It
also takes back a line (D7) the operator approved as written. The decided
form is the split the probe's own header already makes: identity is a
per-repo fact about the follow-through, and behaviour is a fact about the
binary. The verb owns the binary, so it judges behaviour, and the operator
owns the follow-through, so G12 tells them. One residual is accepted. The
behaviour probe execs the render from a temp file, so it cannot see a render
that refuses correctly from a temp file but is installed wrong by
`install-hooks` (a bad mode or a bad chain). That shows up at the operator's
re-render, or at the next launch's identity∧behaviour probe in that repo,
which re-renders the session's own repo anyway. It never shows up silently.
ASSUMED, for the build to MEASURE: that `execOwnRenders` against the source
repo does not depend on the hooks installed there (its header says it never
execs the file at a dispatch path).

**No longer ASSUMED — MEASURED 2026-11-13 by the build bead
(ranger-base-apf87), and it holds.** `posse gates render-probe` answered
**exit 0 with a byte-identical verdict line in all seven readings**, over
two arms. Arm A asked it of the real source checkout three times without
writing a byte there, moving git's hook dispatch path instead of its files
(`core.hooksPath` through `GIT_CONFIG_*`): hooks as installed, hooks
neutered to `exit 0` bodies, hooks absent. Arm B asked it of a scratch
checkout whose hooks really were absent, then installed by `posse gates
install-hooks`, then overwritten with `exit 0` bodies, then removed again.
The claim is now a pin as well as a reading
(`TestRenderProbeAnswerDoesNotMoveWithTheInstalledHooks`), with the
exit-1 and exit-2 arms pinned beside it so that "the same answer every
time" cannot be a constant function. docs/notes.d/ranger-base-apf87.md has
the readings and the method.

**D4 amendment 3's "an old binary that does not have the verb answers
neither 0 nor 1" is FALSE on this box — MEASURED 2026-11-13 by the build
bead (ranger-base-apf87) at 34bbe053, the commit before the verb existed:
`posse gates render-probe` prints `posse: no such agent: render-probe` and
exits 1.** The gates arm's sub-verb ladder falls through to `LoadAgent`, and
an unknown persona is a `die()`, which exits 1 — the same code the verb
itself uses for "a render does not refuse". Read as the amendment wrote it,
the first install after this verb lands would have been both-red-same: the
old binary's `no such agent` excused as a box property, and a new binary
whose render really had stopped refusing left installed. That is the
expensive direction. The amendment's INTENT is kept exactly — a non-zero
answer only excuses the new binary when the OLD binary gave that same answer
*from the verb* — and the realization carries "was it asked" on stdout
instead: every line this verb prints opens with `render-probe:`
(`posse.RenderProbeMark`), and `upgradeRead` counts an answer as the verb's
only when a line does. The three exit codes are exactly the three the
amendment names; the mark is the fourth state it thought the exit code
already gave it. Pinned by
`TestUpgradeDoesNotExcuseANewRedWithAnOldBinaryThatLacksTheVerb` and
`TestUpgradeRenderAskedReadsTheMarkAndNotTheExitCode`, and the verb's
presence in every later binary by `TestQAGatesRenderProbeVerbIsInTheBinary`
— which is load-bearing for the reason the amendment gives: the OLD binary's
code is what calls it, so a release that dropped the verb would read as "not
asked" and roll every later install back.

An UNKNOWN code is treated as binary-rendered, which is the correct answer
rather than a fallback: the table is complete for the binary doing the
judging (the pin says so), so a row the NEW binary printed and the old one
cannot name is a condition the new binary invented.

The automatic rollback is write-beside-then-`rename(2)` rather than the
`install -m 0755` D4 prints. Both are "not through the new binary" — it is
the old binary's own code, in a process exec'd from the old inode — and the
printed one is for a human typing it after this process has exited. For the
arm that runs while a fleet may be launching, the `install(1)` measurement
above applies with its full force. `<target>.prev` is left in place
afterwards, so the printed command stays true for whoever reads the report.
