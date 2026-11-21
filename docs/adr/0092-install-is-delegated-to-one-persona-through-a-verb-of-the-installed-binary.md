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

*Amended by the architect (ranger-base-v2xj9, 2026-11-21): a row the OLD
binary cannot name is classed by the binary that printed it.* As built, the
third bullet's classes are read from the table compiled into the binary
running the verb, which is the OLD one. A row added inside the range is not
in that table, so the unknown-code rule classes it binary-rendered. The
first install that crosses a new G-row then rolls back whenever the row is
present, however the target classifies it. MEASURED 2026-11-21
(ranger-base-i9fq8): 0e1cb6c0..65d90a5e rolled back on one LANE G18 row,
which ranger-base-rrgnc added in that range and classed install-subject. The
fix for such a crossing cannot be installed either, because every target
holding it is judged by the table that predates it.

This is the old-reader, new-writer case of mixed-version systems. The field's
standard answer is that the writer's data describes itself, so a reader never
has to guess what a field it has never seen means. The new binary writes the
row, so the new binary describes it:

4. **The after-reading asks the new binary for its partition.**
   `posse upgrade --classes` prints one line per row in
   `upgradeConditionSource`, as `upgrade-class: <row> <shop|binary|install>`,
   and does nothing else. It is read-only, takes no lock, and needs no
   `install_delegate:`. "Asked" is read off the `upgrade-class:` mark, never
   off an exit code, for amendment 3's reason: an old binary answers
   `unknown flag: --classes` and exits non-zero. A pin keeps the flag in
   every later binary, because the OLD binary's code is what calls it.
5. **The new binary's class is read only for rows the old table cannot
   name.** For a gained row the judge does not know: if the new binary
   classes it `shop` or `install`, it is not a trigger, and the report prints
   one line naming the row, the target's class, and that this binary had no
   class for it. If the new binary classes it `binary`, does not name it, or
   did not answer, it trips exactly as today. For a row the old table DOES
   name, the old class rules and the new answer is not consulted.
6. **When the two tables disagree on a row that tripped, the rollback line
   says so.** For example: `the target classes G7 install; this binary
   classes it binary, and this binary's class is the one that judges`. A
   reclassification takes effect one install late. When it exists because
   the install itself creates the row, as G18's does, that one crossing is
   the operator's `make install`. The report names it in one line, so nobody
   has to recover it from two trees.

**Why this is not a candidate voting on its own install,** which is the
objection D4 was written around. Rule 5 lets the new binary excuse only rows
whose existence is its own speech alone. A binary that wanted to escape a
row it alone can print could simply not print it. So its class for such a
row gives it no power it did not already have. For every row the judge can
name, the judge's class still rules, and the new binary's answer is not read.
That is where the judge's independence earns anything: it is where a
reclassification could excuse a row the old code would have tripped on.

**The bootstrap is not closed here.** A box whose binary predates rule 4
judges every crossing by the old rule. Installing G18 from 0e1cb6c0, or this
amendment's own build from any binary before it, needs one operator
`make install`. That is ranger-base-i9fq8's question, and it does not wait
on this.

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

*Realized by the build bead (ranger-base-41l0r, 2026-11-14) in two parts
this section left open. (a) BOTH readings — where the hook reaches, and
whether the gate installed there works — are taken by running
`verify-gate-freshness` itself, which grew two read-only modes for it
(`--print-wrappers`, `--behaviour-only`). That script already holds this
tree's one reading of the operator's hook entry and its one spelling of the
three arms; a second reader of either would drift from it silently, and the
direction it drifts in is this verb installing a box-wide fence and calling
it green. The script still installs nothing — the half that WRITES is in the
binary, which is D1's own argument one file over. (b) "did not resolve or had
moved" is FIVE shapes, each a state the operator's settings can really be in:
no PreToolUse hook names the wrapper; the hook command names a relative path;
the settings name more than one wrapper; the named path holds no file; the
named path is a symlink. Each writes nothing and prints the settings entry
plus the one install command. docs/notes.d/ranger-base-41l0r.md has the table
and the reasons.*

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

*Two outcomes the four words did not name, decided by the build bead
(ranger-base-41l0r, 2026-11-14) and now part of this field's meaning. An
installed copy that is already `<sha>`'s and whose behaviour arms go RED
reads `refused`: not `unchanged`, which claims the arms were green, and not
`unknown`, because the arms measured something. The break is under the bytes
— no python3, a bad mode, a settings entry pointing at something unexpected
— and reinstalling the same bytes is the one prescription that cannot work,
so the arm reports and stops. And an exit 2 taken AFTER the swap reads
`unknown` and leaves the install standing: every path that answers 2 in that
mode is a box property the installed bytes cannot cause, a gate whose
interpreter is gone is equally dead on either version of the pair, and a
rollback would hide which copy is in place behind a word that means "I
measured nothing". The `.prev` pair and the two rollback commands are in the
report either way.*

For every install, the delegate posts that line on the install bead, and files
one `-l security` bead for the security persona. That bead names the range and
every bead id in it that carries `security` or `vuln-private`, by id only
(AGENTS.md, "Landing the plane"). Relaunching seats so they pick up the new
walls stays the coordinator's.

*The remedy is two steps, not one (amended by ranger-base-rrgnc).* A seat is
relaunched BY the `dispatch --watch` loop, and D3.5's rename leaves that loop
on the old inode, so a relaunch through a loop started before the swap
renders the OLD walls. MEASURED twice on linux: a launch six minutes after
the edfe7a79..3c0e3c5e install read `rendered by posse 0.5.3+edfe7a79`, and
after 3c0e3c5e..0e1cb6c0 a fresh seat still carried ranger-base-hgcmw's
pre-fix launch line while the binary on disk had the fix. The remedy is
therefore **restart the dispatch loops, then relaunch seats**. Restarting a
live dispatcher stays the operator's or the coordinator's (guardrail 3); the
verb only SAYS it: the line before D6's names the live loop's pid and the
image it still runs (`dispatch: dispatcher pid N runs …`), and `posse status`
carries the same sentence as **G18** until the loop restarts. G18 reads
`/proc/<pid>/exe` where it can, so it sees loops started before it existed;
elsewhere (darwin) it reads the image identity the loop stamps into
`dispatch-watch.pid` at start, and a loop too old to have stamped one is an
abstention. G18 is an install subject in D4's partition — the rename creates
it by construction — so it never rolls an install back.

*The reading is ONE instance's and says so (amended by ranger-base-nqaa1).*
"Restart the dispatch loops" is plural and the reading behind it is singular:
both renderings take one `*App`, ask THAT home's watch lock whether a loop is
running and read THAT home's `dispatch-watch.pid` for its pid, while the file
D3.5 renames is box-wide and every instance's loop execs it. MEASURED
2026-11-21 on linux: two loops on this box, pid 2066610 under `~/homes/ranger`
and pid 1991667 under `~/homes/weller`, both exec'ing `~/.local/bin/posse` at
dev:ino 54:12591441, each with its own `dispatch-watch.pid` naming its own pid
— so both are legitimate, the per-home lock invariant holds, and one rename
leaves both behind. Neither was stale when it was read, so G18 was correctly
silent for both: this is coverage and was not a live exposure. **Nothing else
would have said it**, which is what makes it a residual rather than a note:
ShopCheck's G18 arm is per-instance (the watch tick, `posse status`, the
cockpit), the weller instance has no pulse state at all so no tick there ever
evaluates G18, and this verb runs in the delegate's instance only. So the
remedy is: both renderings carry a SCOPE clause naming the instance they read
and saying the subject is box-wide (`watchScopeSentence`, watchimage.go), on
every branch — the two that are not a stale verdict especially, because
"already runs" and "could not be read" are the lines an operator would
otherwise finish believing the box was covered. **Widening the reading to
every instance is NOT done here**: it needs a notion of "the instances on this
box", which no shipped code in this tree has — nothing enumerates RHQ_HOMEs,
and the one `"homes"` literal under `internal/` and `cmd/` is the two-instance
test fixture this amendment's pin is built from — and
coining one is its own design bead (ranger-base-is1qp, which carries the
measurements and the shape of the decision). Restarting any of those loops
stays the operator's or the coordinator's either way. **That bead decided
D8**, which widened the verb's reading and left this one exactly as this
amendment wrote it — so the scope clause below is still the whole of what G18
and `posse status` say about their reach, and it is still true of them.

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

**D8 — The verb reads the loops ON THIS BINARY, box-wide; G18 stays
per-instance** (ranger-base-is1qp, deciding the bead D6's nqaa1 amendment
filed). The amendment above made the narrow reading say its own reach. This is
the widening, and the first thing it does is refuse the notion the bead asked
for.

*What is refused: "the instances on this box".* It is the wrong derivation,
not merely an absent one. `RHQ_HOME` is an environment variable and nothing
constrains it to live under one root, so an enumeration of declared instances
OVER-reports — an instance running no loop has nothing a rename could leave
behind — and UNDER-reports, because a loop whose home is outside the root is
exactly the hole. It would also need a homes root in config, which is a key
the operator would have to maintain for a reading to stay correct.

*What is coined instead, derived from the file D3.5 renames:*

> **the dispatch loops on this binary** = every process on this box whose
> image came from the path being replaced, and whose own `RHQ_HOME`'s
> `state/dispatch-watch.pid` names that process.

No config, no homes root, no new key anywhere. It sees loops nobody declared,
and it cannot see an instance that is not running one — which is the right
blindness, because the subject of the hazard is a rename and a loop that does
not exist renders nothing.

*The key is the exe link, and never argv.* Candidacy is `/proc/<pid>/exe`
resolving to the target path: the kernel's own statement about which image a
process runs, which survives the rename and says so (readlink appends
" (deleted)"). An argv match is forbidden here for the reason AGENTS.md
("Ending anything") forbids it as a target key — every seat on this box runs
byte-identical argv from a different instance — and it is also why
`sysProcTable` (loadguard.go), which already walks the box, cannot be the
reader: two bounded `ps` reads give comm and argv, and `ps` cannot report the
(dev, inode) of a process's image at all. The image identity comes through
`procExe`, the same spelling G18 reads it through, so the two readings cannot
disagree about what one pid runs.

*The per-home record is a FILTER, not an enumerator, and that is the whole
trick.* Each candidate names its own instance (its `RHQ_HOME`, from its
environ), and a candidate is a loop only when THAT instance's
`dispatch-watch.pid` names THAT pid — the invariant watchpid.go already keeps,
read in the one direction that needs no homes root. Enumerating records needs
one; filtering pids `/proc` already produced does not.

MEASURED 2026-11-21, this box, linux 7.2.15-1-omarchy, uid 1001 at cage
shims, `~/.local/bin/posse` at dev:ino 54:14541392 — three processes ran that
image:

```
1185112  ppid 1140     54:14541392  RHQ_HOME=~/homes/weller  record 1185112  LOOP
1185185  ppid 1140     54:14541392  RHQ_HOME=~/homes/ranger  record 1185185  LOOP
3285561  ppid 3285542  54:14541392  RHQ_HOME=~/homes/ranger  record 1185185  NOT a loop
```

The third row is why the filter is load-bearing, and it is a live false
positive rather than a hypothetical one: a seat's own `posse pulse wait
--after 494` child runs the same image, from the same path, under the same
`RHQ_HOME` as the ranger loop. On the exe link alone it is indistinguishable
from a loop; its instance's record does not name it, so the filter rejects it.
Sibling `exe` readlinks, their stats, their `environ` and the sibling
instance's record were all readable from that seat. Note what moved since the
bead was filed: the install inode and both pids (54:12591441, 2066610 and
1991667), because an install landed and both loops restarted at 12:11:54Z
onto it. So neither loop was stale when the box was read, and what is measured
here is READABILITY — which is what the mechanism needs. A live stale sibling
is still unmeasured, and the fixture pin holds the arrangement instead.

*Whose reading it is: this verb's alone.* G18 is unchanged. A box-wide row
inside a per-instance report is a new category and an incoherent one: it would
be raised once per instance that happens to have pulse state and silent in
every instance that has none — the weller instance has no pulse state at all,
so no tick there evaluates G18 — so one box-wide condition would be both
duplicated and missed depending on which instances tick. `posse upgrade` is
the one command whose subject is box-wide, because it is the one that does the
rename.

*Both lines print, and neither claims the other's reach.* The per-instance
`dispatch:` line stays first: it reads this delegate's own instance through
its watch LOCK and its own pid record, which is a stronger reading of that one
loop than a `/proc` census is — a lock the kernel holds, against a pid record
a process names itself by — and it is the loop the delegate's next seat comes
through. The `dispatch (box):` line is weaker per loop and wider, and it is
the only thing in the report that can say a sibling instance was left behind.
Each carries its own scope sentence (`watchScopeSentence`,
`boxScopeSentence`).

*A reader, and nothing else.* Nothing restarts, signals or touches any process
it finds; restarting a live dispatcher stays the operator's or the
coordinator's (D6, guardrail 3). Liveness is "`/proc/<pid>` is there", never
`kill -0`: under `LANDLOCK_SCOPE_SIGNAL` a caged seat's signal 0 to a process
it did not fork is EPERM whether that process is alive or dead (AGENTS.md,
ranger-base-e5un1), so a signal-based liveness check would read wrong from
exactly the seats that most need it.

*What a caged reader gets, amended by ranger-base-rph5z after hoover's review
under ranger-base-gvrv7, and the one claim this section first made wrongly.*
As first written D8 said the census "survives a cage by ruling rather than by
luck", on the grounds that `--unshare-pid` is deliberately absent from the
bwrap argv and `/proc` stays the host's procfs under `--ro-bind / /`
(bwrap.go). That is true and it is not enough: **it opens the pid LIST, not
the links.** A seat in a child user namespace is REFUSED a same-uid peer's
`exe` and `environ`, because `cap_ptrace_access_check` compares user
namespaces and not just uids (the ranger-base-rfyth mechanism). MEASURED
2026-11-21 by hoover from such a seat (`/proc/self/uid_map` = `1001 1001 1`):
`/proc` listed 460 pids, `readlink /proc/1185185/exe` and `cat
/proc/1185185/environ` were both EACCES, and a child that seat had started
itself was readable. D8's own MEASURED block was taken from a `cage shims`
seat, which is not in a child userns — so the reader class this census most
needs to work from is the class that first reading did not cover.

`procExe` answers the same "not ok" for a pid that exited and for a link the
kernel refused, so the census as first built dropped both silently and
rendered "no dispatch loop on this box runs …": a false all-clear over loops
it was not allowed to see, which is the one sentence this reading must never
be able to print. The remedy is `BoxReading.Unreadable` — a COUNT of
still-present processes of this account whose exe was refused, told from a
dead pid by `boxPidOwned`, which stats `/proc/<pid>` itself because the
directory's owner stays readable from the cage. If that count is not zero the
reading is PARTIAL: every branch that would otherwise read as an all-clear
says instead that N processes could not be read and that loops among them are
unknown, and when nothing of ours was readable at all the reading is an
abstention with no verdict in it. A refused process of ANOTHER account stays
excluded: root's are refused to everyone and are not subjects of this rename.

*And the count has to be narrower than "ours and refused", which the first fix
was not.* MEASURED 2026-11-21 from a `cage shims` seat on this box: FOUR
same-uid processes are permanently in that set and none could ever be a
dispatch loop — the user `systemd` (pid 1140), `(sd-pam)`, `sshd-session` and
`gpg-agent`, each EACCES (errno 13) on its exe link while `/proc/<pid>` is
owned by uid 1001, because each is non-dumpable. Counted, they made every
reading on this box partial forever — and a caveat that is always on is one a
reader stops finishing, which would have hidden the false all-clear behind a
sentence nobody reads. `/proc/<pid>/comm` separates them: it is world-readable
with no ptrace check, so it answers where `exe` does not. A refusal is counted
when comm names this binary, or when comm could not be read either (knowing
nothing about a process of ours is the whole content of the caveat); it is
excluded when comm names another program.

That value is used for ONE decision — whether a refusal is worth reporting —
and never to confirm a loop, which is why it is not the argv matching AGENTS.md
forbids: that rule is about choosing a TARGET by a pattern every seat shares. A
hostile `prctl(PR_SET_NAME)` can inflate the count and make a complete reading
say it is partial. That is a denial of clarity rather than a false all-clear,
which is the direction this reading is built to fall in.

*A path that holds nothing is STALE,* which is `WatchImage.Stale`'s
predicate exactly and is aligned deliberately rather than inherited. An empty
on-disk identity means nothing is at that path at all — a swap in flight, or
`install(1)` between its unlink and its write (Context) — and a loop whose
image is not the one at its path is in this condition whether the replacement
has arrived yet or not. The comfortable-looking alternative ("nothing to
compare against, so not stale") renders as "all already on the installed
image" over a path with no installed image on it, which is the one sentence
this reading must never be able to print. The rendering says the path holds
nothing and tells its reader to read that before reading it as loops to
restart.

*Three things it must never do, each pinned.* (1) **The reading process is
skipped.** The verb runs FROM the path it replaces, so it is a candidate by
its own key, and after its own swap its own image is the old inode — unskipped
it would report itself as a stale process on this binary on every run. A
transient CHILD of the verb is NOT skipped: it is named among the
unattributed. (2) **A candidate it cannot attribute is named, never dropped.**
Silent under-reporting is the failure this whole reading is a correction of.
(3) **An abstention is never rendered as an all-clear.** Off linux there is no
census — `boxloops_other.go` refuses and says why, on `procstart_other.go`'s
rule, because the darwin process table (`sysctl kern.proc.all`) carries comm
and argv and neither an image inode nor an environment, and a census compared
against an unmeasured spelling passes for the wrong reason. An unreadable
`/proc` is an error and not an empty list. The abstention's rendering says the
per-instance line is the whole of that report's coverage and carries this
bead's id, so a darwin reader is told the residual stands rather than handed a
silence that looks like a clean box.

*The home it lifts from a peer is untrusted text, amended by
ranger-base-cwboz from the same review.* `RHQ_HOME` comes out of another
process's environment, and the census trusted it twice: it RENDERED it, into
the line the install delegate pastes onto an install bead — so arbitrary text,
newlines included, would arrive in that persona's context under its own
authorship, which is injection into the one persona holding the install verb
— and it OPENED it, as `<home>/state/dispatch-watch.pid`, where a FIFO would
block the verb forever with no timeout, after the swap. Neither needs a
planted attacker: a caged seat's own child qualifies, and D8's first live
reading found exactly such a child. So the value is now accepted only as an
absolute, already-clean path of printable bytes (`boxHomeIsWellFormed`),
checked both in the platform read and again at the portable chokepoint every
future reader goes through; a value that fails is never rendered in any form,
and its pid is named with no text. The record is `Lstat`ed for a regular file
before it is read, so a FIFO or a symlink to one is refused rather than
opened. And the fence on the environ read itself is a census rather than a
comment (`internal/treepins/environread_qa_test.go`): the proc-environ path
appears in exactly one function in the shipped tree, that function returns
exactly `(string, bool)`, and the only environment key it matches is
`RHQ_HOME=`.

*Residual.* A same-uid process can still plant a well-formed home holding a
regular record that names its own pid, and so present itself as a loop or as a
stale loop. Read-only effects, and it is the uid boundary doing the work
rather than this reading; the operator restarts loops from this report, so it
is stated rather than fenced. Named on ranger-base-cwboz.


## Consequences

- An install no longer waits on the operator being at the keyboard. It waits
  on a push and one `make test` — **MEASURED 19m08s on the first real run**
  (ranger-base-jnf2z, 2026-11-20, edfe7a79..3c0e3c5e), inside the 15–25 min
  envelope this bullet ASSUMED from ranger-base-kiwvn's 280s over the three
  arms uninjected plus arm 1's `./...`. That run was NOT on a quiet box: both
  suite slots were held at launch and 146s of the 1148s wall was the verb
  queueing behind sibling seats, so the envelope is confirmed under
  contention and its quiet-box floor is still ranger-base-zhw0h's 11m57s for
  `make test` alone. The second install read **16m57s**
  (ranger-base-76esp, 2026-11-20, 3c0e3c5e..0e1cb6c0) with only 10s of queue
  in it — wall minus queue is 1007s against jnf2z's 1002s, so at a loadavg
  near 11 the verb's own work prices at ~1000s on this box and the queue is
  what moves the wall. A quiet-box reading is still unmeasured after both.
  See "Measured versus assumed" below.
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
- *For a row the judge cannot name (ranger-base-v2xj9):*
  - *Waive every unknown row.* This ends the false rollback and loses a real
    one. A new binary-rendered row, a G7-shaped check whose grammar narrowed
    in the same range, would land unjudged on exactly the install that
    introduced it.
  - *Expand, then contract: the class ships one install before the row.*
    This is the field's other answer, and it needs no code. But installs
    here are ranges (16+ commits in the measured case), so both halves
    land in one range and the crossing still happens. A discipline that
    fails on the common shape is a disclaimer.
  - *Baseline the new row by running `posse.new status` before the swap.*
    This is the clever one, and the one I wanted: it gives a new row a real
    before-count instead of a class. It does not fix the measured case,
    because G18 is CREATED by the rename, so its pre-swap reading is zero.
    It also adds a third status reading to every install.
  - *An override flag that accepts a named row.* The delegate would type it,
    which is the judged party's agent waiving the judge. The operator
    already has an override, `make install`, and rule 6 points to it.
  - *Let the new table judge every row.* This is the vote D4 refuses (see
    "Why this is not a candidate voting" above).
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

**The whole verb's wall is MEASURED now, and no trigger has yet fired —
2026-11-20 by the first delegated install (ranger-base-jnf2z), 19m08s for
edfe7a79..3c0e3c5e, exit 0, no rollback.** It ran at a loadavg of 11.98 with
both suite slots held by sibling seats, so **146s of the 1148s wall was the
queue** and the figure is the verb under contention rather than on the quiet
box Consequences hypothesized; the rest went to ~83s of tree-wide doors, the
three arms (arm 1's five packages 214.0/154.5/119.1/2.0/0.2s, arm 2 81.5s,
arm 3 119.1s), the ~1s private copy and the clean-build. D4's three readings
came back identical across the two binaries — 8 conditions, 36 not-held gates
rows, 0 non-zero gates exits, `render-probe` 0 on both sides with its mark
present — so the triggers are still exercised only by their pins, and this
ADR has no live reading of a rollback. The D5 arm reported `unchanged`
(nothing written; all three behaviour arms green), and the range touched
neither the gate pair nor the hook render, so D7's G12 re-render sequence is
also unexercised.

**G11's exclusion is no longer only a census finding: it fired.** While that
install's suite ran, origin/main moved 3c0e3c5e to 783cdeb9, so the install
finished one commit behind and opened a G11 row by succeeding — the exact
case the G11 paragraph below reasons about. Under the assumed set that named
only G12 and G17, the first delegated install would have rolled itself back
for working. docs/notes.d/ranger-base-jnf2z.md has the phase table, the
quoted before/after lines and the D6 line verbatim.

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

**That sentence is true of the TABLE and false of the CROSSING — MEASURED
2026-11-21 by ranger-base-i9fq8, diagnosed on ranger-base-v2xj9.** The table
was complete for the binary judging, and the pin held. ranger-base-rrgnc
classed G18 at the same time as it added the row. The install still rolled
back, because a class ships in the binary being installed, one install after
the judge needs it. Read from both trees: 0e1cb6c0's
`upgradeConditionSource` names G1-G17 and no G18. d24988c3's names G1-G18.
G18 is the only row whose class differs between them. Every other D4 arm
read the same on both sides: 15 conditions apart from the gained row, 11
personas, 36 not-held rows, 0 non-zero exits, render-probe 0. The row was
true when it fired: both live loops were on the old inode. D4 amendment
(ranger-base-v2xj9) rules 4-6 are the answer.

ASSUMED, for the build to MEASURE:
- An old binary answers `posse upgrade --classes` without the mark. This is
  read from cmd/posse's flag loop at this commit, which refuses any leading
  `-` before acting.
- The extra after-reading costs under a second. It prints a static map.

**D5's class is live on this box, MEASURED 2026-11-14 by the build bead
(ranger-base-41l0r): `make verify-gate-freshness` read
`bd-argv-gate.sh is STALE — it is not HEAD's` against `~/src/posse @
db4dc29b`, with the parser fresh and all three behaviour arms holding.** One
file of the fence every Bash call on this box runs through, stale in the live
copy, with the detective control that says so wired into `make install` since
ranger-base-d0jo and ranger-base-2tnbv already closed once by hand for the
same drift. That is the whole argument for the arm being an arm.

The arm's own rollback is pinned by MUTATION rather than by assertion count,
because what it promises is a negative — MEASURED 2026-11-14, three one-line
changes to upgradegate.go: never taking the "already identical" branch reds
on the word and on the `.prev` files that appeared; restoring the wrapper
alone reds on the parser, naming a half-restored pair as a combination nobody
ran; and deleting the symlink arm is **green**, because the
not-a-regular-file arm catches that shape too. The box is equally safe
either way and the only thing lost is the one thing the operator needs from
it — why their arrangement was not refreshed — so the pin now asserts the
word `symlink` in the refusal. The three behaviour arms being reachable
through the flags the arm passes is pinned where the script can be run
against a planted gate (internal/treepins/gatearmmodes_qa_test.go, which
extracts those flags from the arm's own source): without it, a rename of
either mode leaves every install reporting `gate unknown` with the box's
copy never refreshed, and nothing in Go can see it.

The automatic rollback is write-beside-then-`rename(2)` rather than the
`install -m 0755` D4 prints. Both are "not through the new binary" — it is
the old binary's own code, in a process exec'd from the old inode — and the
printed one is for a human typing it after this process has exited. For the
arm that runs while a fleet may be launching, the `install(1)` measurement
above applies with its full force. `<target>.prev` is left in place
afterwards, so the printed command stays true for whoever reads the report.

**D3.2-D3.6 amendment (ranger-base-p8ymi): the checkout is a private,
re-hashed copy, never a worktree of the source.** Found by ranger-base-8wi4p.
A `git worktree add` of `<sha>` reads the source checkout's object store, git
does not re-hash a local object on read, and every walled worktree seat is
granted `<common>/objects` — so a seat could swap one blob origin/main reaches
and this verb's caller, uncaged, would run it as `make test` under an
unchanged sha. The verb now `git init`s a new repository in the fresh
`mktemp -d`, fetches the origin/main tip D3.1 read into it by full sha (the
receiving side hashes every object, and a substituted one fails the fetch),
takes D3.1's two ancestry answers again in that store, and checks `<sha>` out
there. Steps 2, 3 and 6 all run from that copy, which no seat can reach;
clean-build.sh's own fetch (ranger-base-la61c) then reads it. Every git call
of the copy carries `--no-replace-objects`. MEASURED 2026-11-14: about 1.1s
for this repo's full history. docs/notes.d/ranger-base-p8ymi.md has the
probe, the cost and the pins.
