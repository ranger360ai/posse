# ADR 0050 — A data ceiling is a second pattern key that walls every repo the instance hooks, whatever its visibility stamp

*Status: accepted 2026-09-02 (ranger-base-zrkl6, from ranger-base-3i035 /
ranger-base-w9jv (b)) · owner: architect · extends ADR 0024 D2 and ADR
0048 D2 · builds in ranger-base-nfg8l (code, dinesh); the posture
doc and the work-install runbook amend in ranger-base-83crg (security, hoover) ·
number: this file took the next free number at commit; the 0043–0045 gap is
not a reservation — ADR 0040 as simplified 2026-09-05 reversed the four-new-root
plan those numbers were pre-named for (ranger-base-hn32r) · amended 2026-09-04 (Context,
D2: the two arms read every staged FILE, not every staged "text" file —
ADR 0048 D2 as amended; ranger-base-9307c, from ranger-base-h137b) ·
amended 2026-09-03 (D2, D5, Consequences: the commit MESSAGE is a THIRD
subject — product decision that date on ranger-base-pqlxr, landed
2026-09-04 in ranger-base-o2v6n, which is why this line sits after the
09-04 one above) · amended 2026-09-28 (D6, Alternatives: a value that
matches its own definition line is WARNED at stamp time, never refused and
never exempted — ranger-base-3gdqv, from monica's work-box measurement
2026-09-27; builds in the code bead named on that bead) · amended
2026-09-28 (D6: the printed remedy names the LINE, not the value alone,
because the check reads the line — ranger-base-l2569, from
ranger-base-tl0mg) · amended 2026-10-08 (D2, D5: over `.beads/*.jsonl` the
content arm REPORTS — class, hit count, record id — and the commit lands;
it refuses everywhere else. A bead record has no remedy a committer can
perform, so the refusal only chose who typed the override — ADR 0068,
ranger-base-6pisf, from github issue #1; the second layer this record
deferred is triggered and filed there).*

> An instance that holds someone else's data has two different questions
> to ask of a staged line. *May this be public?* is visibility, and ADR
> 0024/0048 answer it. *May this exist in a local file at all?* is a
> ceiling — the operator's fact (b): restricted-class content never enters
> the bead db, transcripts or memory; the employer system's id is the
> sanctioned citation. As built, the harness has one wall and it answers
> only the first question.

## Context (MEASURED 2026-09-02 unless marked)

- **Every check is inside the stamp gate.** `visibilityGuardBody` renders
  checks 0–3 — the beads jsonl, the docs-genre allowlist, shipped patterns
  over markdown, identity literals and (since ADR 0048 D2) instance
  patterns over every staged file and added path — under one shell
  `if`: the stamped visibility equals `public`. A repo stamped `private`
  runs none of them. So `beads_visibility_patterns:` is inert in exactly
  the repo the ceiling is about: the work instance's own bead repo, which
  the posture marks private on purpose (f85 M2 criterion 3).
- **Marking that repo public is the wrong lever.** The stamp is the
  visibility record, not a scan switch: flipping it would loose the
  shipped ops-class list on the work instance's own ops, cost and
  credential-topology beads, which are legitimately there (NOTES.md,
  Privacy model).
- **The ceiling does not key on ids.** The posture's §3 (ii) names what
  rides with a paste and never with a cite: restricted-tier classification
  banners, the hostnames and URL forms of restricted systems, their export
  file-name and attachment-marker patterns. Two of the three are content;
  the third is a PATH — which is why check 3's staged scope (added lines,
  added entries) is the right shape and check 2's markdown scope is not.
- **The renderer already exists.** ranger-base-uzgkz (1e9b2ba) and
  ranger-base-8114t (ae7b08f) give check 3 one refusal shape with the
  words as fields, a class-only matcher (class and hit count, never the ERE
  or the match), the instance-pattern arm rendering with an empty identity
  list, and the same override and refusals.log shape. At this writing both
  sit on their session branches, not yet on main
  (`git merge-base --is-ancestor`: NOT on main for all three commits).
- **A refusal is a local file.** refusals.log, the terminal scrollback and
  the pane capture are all local files under fact (b). A ceiling refusal
  that printed the matched text would breach the ceiling by the wall's own
  hand. Class-only is not a courtesy here; it is the rule's own condition.

## Decision

**D1 — a second config key, not a scope flag.** `data_ceiling_patterns:`
is a one-level map, class → ERE, in the same two-reader dialect and read by
the same flat-YAML pair reader and the same validator as
`beads_visibility_patterns:`. It is instance-wide: one list, applied to
every repo this instance renders a commit hook into, whatever that repo's
stamp. The two keys share ONE class namespace — a class taken by the
shipped list or by either key is refused in the other, because a refusal
names a class and a class has to mean one thing. An entry the validator
refuses is carried and named (class only), in the hook file and in
`posse gates install-hooks` output, never dropped — the ADR 0048 rule.

**D2 — the ceiling is its own block in the same hook, above the stamp
gate, over check 3's subjects.** The `prepare-commit-msg` hook renders the
ceiling scan FIRST — before `posse_beads_visibility` is tested — over the
ADDED lines of every staged file and the ADDED staged paths (every file,
whatever its bytes — the reader carries `--text`, ADR 0048 D2 as amended
2026-09-04; a restricted system's export is a blob with a banner in it, and
that is the ceiling's own case), using
the two-arm renderer uzgkz built, with its own words: rule (this content
may not exist in a local file; cite the system of record's id), remedy
(remove the paste from the staged file, keep the cite; there is no private
db to re-file into), footer (this wall runs under every stamp — this
repo's is *N*). The shared helpers the checks use (the base tree, the
matcher function) move above the gate so both blocks read one definition.
First in order because a line that trips both lists must be refused with
the stricter remedy. Always class-only, for the reason in Context. Same
override env, operator-typed, never passed by dispatch; the OVERRIDDEN log
line names the ceiling scan. The refusals.log label is new and names the
stamp it ran under, so a reader can tell a private-repo ceiling hit from a
public-repo one without opening the hook.

*Amended 2026-09-03 (ranger-base-pqlxr, product; built in
ranger-base-o2v6n): a THIRD subject — the commit MESSAGE.* The hook is
handed the message file as its first argument and already reads it; the
ceiling scans every line of it as given, comment-looking lines included (a
`-m`/`-F` message keeps them under git's default cleanup, and a pasted
markdown heading is one), class-only, same override, log tail `(stamp: N,
commit message)`. Not a new wall: an arm of this one, no new git command.
Its remedy differs from the file arm's: rewrite the message with the system
of record's id; the refused text is still in `.git/COMMIT_EDITMSG`, a local
unreplicated file the next commit overwrites — the same residual as a
refused staged file. *Since 2026-09-03 the message is check 3's third
subject too* (ADR 0024 D2 / ADR 0048 D2 as amended, ranger-base-1nbtn,
built in ranger-base-qk8i9): one renderer reads `"$1"` for both walls, and
the ceiling's arm still runs first — above the gate, with the stricter
remedy — so a message that trips both is refused by the ceiling.

**D3 — one set, read once, seen identically by every renderer.** The
three sites that render the hook today (the install, the chained install,
the launcher's L3 identity probe) must all see the same ceiling list, or
every launch into a hooked repo reads "ours but stale" (ADR 0023 is
byte-for-byte). The property that guarantees it: the ceiling list rides in
the same value those callers already pass, populated by the same config
read, with the visibility list's effective order untouched. The
install-hooks output gains one line per repo naming the ceiling classes
stamped in and, by class, the ones refused; a private-stamped repo, which
today prints only its stamp, prints that line too.

**D4 — the in-session warning learns the ceiling.** The harness's own
pre-filing check (the one that warns before posse files a bead into a
public db) scans the ceiling list regardless of the repo's stamp and says
"ceiling", not "visibility". Cheap, and the only surface that speaks before
the paste is durable.

**D5 — what this wall is.** It guards the durable, replicated copy: the
beads jsonl that syncs to the work instance's internal remote, docs and
memory files that get committed. It does not see the working tree, the
session transcript, the pane capture, or a message typed in the EDITOR:
`prepare-commit-msg` runs before the editor opens, so the editor path hands
it only git's template (measured, git 2.50.1). *Amended 2026-09-04
(ranger-base-h3s6q, from the verify of ranger-base-o2v6n): "hands it only
git's template" was true of what the hook is GIVEN and false of what the arm
READ. The arm scanned that template whole and refused over the `On branch`
line and the `#` status block — staged, unstaged and UNTRACKED paths, and a
merge's `# Conflicts:` list — none of which can reach the commit object
(`--cleanup=strip` is the editor path's default), with a remedy, rewrite the
message, that no rewrite can clear; measured, one untracked file named for a
class refused every editor commit in the repo. The arm now reads the file
whole only where git's cleanup KEEPS a comment line (`-m` and `-F`, `$2 =
message`) and through `git stripspace --strip-comments` on every other path,
so what it judges is what git will keep. Re-keyed 2026-09-04
(ranger-base-6y3z2): `$2` was a PROXY for git's CLEANUP MODE and it breaks
the moment `commit.cleanup` is set. Under `commit.cleanup=verbatim` git keeps
its own template in the object — the `On branch` line, the staged list and the
UNTRACKED file list — and the arm stripped exactly those lines out of the
scan, so a class carried by a branch name, an untracked path or a merge's
conflict list reached the commit object unrefused (measured, git 2.50.1); the
mirror direction, `commit.cleanup=strip` with `-m`, stripped a `#` line the
arm read whole and refused over. The arm now asks for the mode: `strip`
strips, `verbatim`/`whitespace`/`scissors` are read WHOLE, and `$2` is the
proxy for `default` alone. `git commit --cleanup=...` is invisible to a hook
and is the stated residual. The cost of the fail-closed side is h3s6q's
complaint wearing a config — those three modes read git's template again, so
one classed untracked path refuses those writers' editor commits before the
editor opens, and the layer that can tell that apart is the `commit-msg` hook
below. *(Amended 2026-09-04, ranger-base-b21e0: the REMEDY was the half of
that cost the writer actually reads, and it did not need the second layer.
Where the mode is one of the three, the refusal now names it and says what
clears it — clear the class out of the repo, or leave `commit.cleanup` at
its default — and says which way the mode cuts: `verbatim` and `whitespace`
LAND that block in the object, `scissors` truncates it below its cut line, so
under `scissors` the read is over-refusal and the refusal says so rather than
matching the cut line. Both measured, git 2.50.1. The note is suppressed
where `$2` is `message`, because git appends no template there and "rewrite
the commit message" is doable exactly as written; `-m … -e` is the stated
residual, alongside `--cleanup=`.)* *(Amended 2026-09-04,
ranger-base-xfgcn: the read now STOPS at git's cut line, so the sentence
above about not matching it no longer holds. `stripspace` removes comment
lines and the diff `commit -v` appends below that marker is not
comment-prefixed, so it reached the scan whole: an UNCHANGED line within
three of a staged hunk refused an editor commit, and so did the REMOVAL of a
classed line — the one remediation the ceiling's refusal demands — both under
"rewrite the commit message", which clears neither, and both reachable from
`commit.verbose=true` or `-v` with no intent. git truncates at that line when
the commit is verbose or the mode is `scissors` and writes it in exactly
those two cases, so the cut is taken only where git takes it: the FIRST such
line, matched at column one where a unified diff cannot carry it, and only
where `commit.cleanup` is `scissors` or a `diff --` line stands below it —
without that guard a `commit.template` body carrying the marker would take
its own text off the scan, which git does NOT truncate (measured, git
2.50.1). Under `scissors` git's whole status block is below the cut, so it is
now neither scanned nor kept and the refusal's note says what is ABOVE the
cut instead. Residuals, all fail-closed: `--cleanup=scissors` as a flag, a
`core.commentChar` of `-` or `+`, and a staged path whose name carries the
marker.)* *(Corrected 2026-09-04, ranger-base-sx2dq from
ranger-base-md7ui: as b21e0 first landed it, that remedy read
"delete git's block in the editor before you save", which is this same
complaint one turn later — this arm renders into `prepare-commit-msg`, git
runs it BEFORE launching the editor, and the non-zero exit ends the commit,
so no editor session exists in which to delete anything. MEASURED with a
`GIT_EDITOR` that appends to a marker: a landing commit invokes the editor
once, the refused commit zero times, under all three kept modes. The
reachable half is the one INSTALL.md carried before b21e0 — clear the class
out of the repo — and the pin now takes that action and requires the same
commit to land, rather than asserting the sentence is present.)*
*(Completed 2026-09-04, ranger-base-vcouf from ranger-base-49r7t: that
correction reached the `verbatim`/`whitespace` note only. `scissors` takes
the other branch of the same `if`/`else` and its note still ended "delete it
in the editor before you save", because no pin read that string — the
sentence stated the mechanism that makes it impossible in its own first
clause. Both branches now name the reachable pair, narrowed here to what git
puts ABOVE the cut: the `commit.template` body, the path in a merge's
conflict list — or the default `commit.cleanup`, which strips the `#` lines
among them and not a template body that leads with none. The same pin gained
a `scissors` subtest that takes the action and requires the commit to
land.)* *(Amended 2026-09-04, ranger-base-gyrnp from ranger-base-d94zl:
the second licence xfgcn named — "a `diff --` line stands below it" — was
writer-typed. Nothing asked who wrote the file, so an ordinary `git commit
-F msg -- path` carrying the marker at column one, one `diff --` line and a
classed line under it was cut there; git keeps every byte below a marker it
did not write, and the class landed in the object read by nothing. Check 3
renders through the same function and was open the same way. Measured both
ways at 4710e88: with the cut disabled both shapes were refused, with it
both landed. The cut is now licensed on what only git can be asked, the
commit itself. Under `scissors` it is unconditional, as before; under every
other mode the block below the marker is read MINUS the lines of the staged
diff — `git diff --cached`, rendered the way `-v` renders it, against
HEAD^1 under `--amend` — and whatever is left is message, read under the
mode's own rule. Under `-v` that is git's two comment lines; from a writer
it is every line the index does not hold. Every way the reference and git's
own diff can disagree (an empty reference, `-vv`, `status.renames`, a root
commit amended) leaves lines ON the scan, never takes them off it. So the
residual sentence above now reads: fail-closed — `--cleanup=scissors` and
`-v` as flags, a `core.commentChar` of `-` or `+`, a marker forged above
git's own under `-v`, and the drift cases; fail-open and bounded — a line
below a forged marker that is itself a line of the staged diff is not
scanned even where git keeps it, which is a context or removed line and so
content already in a tree object of this repo, the same bound the
`--amend --no-edit` residual carries. The two pins that shipped green
asserting the hole are inverted, and a third takes the licence's edge: the
staged diff pasted exactly lands, the paste plus one classed line is
refused.)* The
exclusion stands as stated: a
message typed in the editor does not exist yet when the hook runs. What the
editor path does scan is whatever was already in the file — a
`commit.template` body, `MERGE_MSG` — which lands in the object like any
other message.* The crew's commit form
(`git commit -F - -- paths`, AGENTS.md) and every `-m`, `-F` and `--amend`
commit are inside; the editor path is the operator's own hand, which is the
boundary above the ceiling already (f85 §4). The second layer for it is a
`commit-msg` hook, which `--no-verify` skips; the trigger for filing it is
the first "commit message" line under the ceiling label in `refusals.log`
on the work box — the same trigger the runtime-side gate carries in
Alternatives. For everything it does see, the paste already happened when
the wall speaks. Routing plus the operator's hand stay the boundary above
the ceiling (f85 §4). The posture's §3 (ii) sentence "as built, the ceiling
has no wall" becomes "the ceiling has a wall at the commit; above it,
routing" once the code bead lands.

**D6 — a value that matches its own definition line is warned at stamp
time; the pattern stays in force and the file that defines it is not
exempted.** *(Added 2026-09-28, ranger-base-3gdqv.)* D2 reads the ADDED
lines of every staged file in every hooked repo, and `$RHQ_HOME/config.yaml`
— the file that holds the key — is one of them: the instance repo is hooked
and config.yaml rides in every archive. So a ceiling value whose regex
matches its own line, `  <class>: <value>`, refuses the commit of the config
that defines it, class-only, and the refusal cannot say why (class-only is
the rule's own condition, Context). Every PURE LITERAL does this
unconditionally, because the line contains the value verbatim: a plain
banner (`ACME-RESTRICTED`), a marker written plainly, a placeholder
(`FILL-ME`, which is how it was found — the work box's four definition lines
were the only hits, four per class). A value carrying any escape or bracket
may or may not (`\[ATTACHMENT\]` and `vault\.example\.internal` do not,
`RESTRICTED.*` does), which is why this is a check and not a rule of thumb;
the shipped examples never self-match by the accident of their form.
MEASURED 2026-09-28, personal box: nine values judged identically by Go's
`regexp` and `grep -E` against their own definition line, so the check can
run in the reader and speak for the hook.

The check lives where the list is read, once, so every renderer carries the
same answer (D3): `OpsPatternSet()` tests each ACCEPTED ceiling entry's
compiled regexp against the RAW bytes of the line that defined it —
indentation, quoting and any trailing comment included, because that is the
line the hook will read out of `git diff --cached`, not the cleaned value —
and carries each match on the set as a class name beside the accepted and
refused lists. `WriteStampReport` prints one line per such class: this value
matches its own definition line, so a commit of the config file that defines
it will be refused by the class it defines; write one literal character in
brackets (`X` → `[X]`) in every part of the LINE that carries the match and
re-stamp. Class only — never the value, never the line; the value is the
vocabulary. *Amended 2026-09-28 (ranger-base-l2569, from
ranger-base-tl0mg): that sentence first named the value alone, and where
the match is carried by something else on the line — a trailing
`# e.g. <the literal>` beside an already-bracketed value, which is the live
shape, or the class name in the key, which is the pathological one — an
operator who followed it exactly re-stamped and got the identical line
back. The refusal is class-only by the rule's own condition, so this
sentence is the only explanation there is and it has to be reachable for
every shape the check flags: it now names the whole line, says the value
and any trailing comment that repeats it alike, and sends the key case to
renaming the class, which is the one shape no bracketing of the value can
reach. The check, the field, the in-force rule and every render are
unchanged; this amends one operator-facing sentence and the two doc
sentences (`examples/config.yaml`, `INSTALL.md`) that carried the same
narrowing.* The entry stays IN FORCE: the wall is
total, and the warning is about one file's commit, not about the pattern.
The hook file does not carry the line (the hook is per repo; the property is
the config's), so the L3 byte-for-byte probe is untouched. The bracketed
form is not a trick around the wall: it is the same regex over every other
file, and it is the form under which a plain search of the instance repo for
the banner finds nothing — which is what fact (b) wants of a local file.
`scripts/ceiling-fill.sh` (monica, 2026-09-27) does that rewrite at typing
time, where the value is still the operator's to change; it stays as an
operator convenience and is not the mechanism.

*Amended 2026-09-28 (ranger-base-yplnv): that convenience read the key
differently from the hook, and for exactly the shape this amendment is about.
The script proves no value matches its own definition line before it stamps —
past the stamp the refusal is class-only and the sentence above is the only
explanation there is — and it took the pattern to be everything after the key,
a trailing comment included. So for `restricted-banner: FILL[-]ME  # e.g.
FILL-ME`, the shape `examples/config.yaml` documents because a comment
repeating the literal refuses the commit on its own, it grepped the staged
lines for `FILL[-]ME  # e.g. FILL-ME`, found nothing, and stamped and
committed into a hook that then refused the commit (MEASURED 2026-09-28: hits=0
for that entry, hits=1 for a plain literal beside it, while
`TestQADataCeilingWarnsWhenAValueMatchesItsOwnDefinitionLine` arm A0 has the
hook warning on the same line — the two readings disagreed, and the one that
stamps was the wrong one). It now reads the value as `yamlClean` does, prints
the stamp report in FULL rather than `tail -1 | cut -c1-72`, and refuses to
commit on the hook's own warning; `TestQACeilingFillReadsTheValueTheHookReads`
runs the script's own extractor against `YamlMapPairsRaw` so the two cannot
drift apart again.*

*Amended 2026-10-08 (ranger-base-6pisf, ADR 0068): D2's content arm and D5's
"guards the durable, replicated copy: the beads jsonl" hold for every staged
path EXCEPT `.beads/*.jsonl`. For a bead record the copy is durable before the
commit (the db), bd's pre-commit re-stages the file into every commit, and
`bd comments` has no edit or delete — so a refusal there removed nothing and
locked every committer in the repo (MEASURED, work box 2026-10-08). Over the
db and the deletion ledger the same classes now REPORT, class-only plus the
record's id, and the commit continues; the report's remedy is
freeze-and-succeed, and the write-time layer the Alternatives below deferred
is filed as its own design bead. ADR 0068 has the measurements and the
alternatives priced.*

## Consequences

- Every hooked repo's render changes; the L3 probe reads them all as "ours
  but stale" until `posse gates install-hooks` re-runs against the
  installed binary — expected once, as ADR 0048 already paid.
- A private repo pays one full `git diff --cached` per commit that it did
  not pay before — the same scan check 3 already runs in public repos.
  ASSUMED: same cost class; the public repo has run it on every commit
  since ADR 0024 D2 without a bead about it. *Amended 2026-09-04
  (ranger-base-h3s6q): MEASURED, and the assumption held for the diff and
  not for the scan. With `--text` on the content readers (ranger-base-h137b)
  every staged byte flows through `$posse_added` and one `grep` per class
  per arm: ~0.55 s/MB, linear (1 MB 0.81s · 5 MB 2.91s · 10 MB 5.61s · 20 MB
  10.6s), against 0.38s for the same 20 MB commit with `--text` off — 28x,
  and it scales with the class count too. The reader itself is 0.12s of it.
  ACCEPTED with the number written down rather than capped: a size cap is a
  mechanism written down as a rule, which is how the `--text` hole got here.
  Who pays is a hooked repo that commits assets; posse has none.*
- The work instance's config gains the key at M2, values written by the
  operator on the work box (a PromotedPath; a persona cannot). The class
  names are hoover's §3 (ii) list; the values never leave that box. On
  THIS instance the key stays absent and the block renders nothing.
- NOTES.md "When an instance holds someone else's data", INSTALL.md's
  visibility paragraph and examples/config.yaml each gain the key beside
  its sibling, with the one-sentence distinction: visibility says where
  content may go, the ceiling says whether it may exist here at all.
- The hook's head comment counts four walls in the slot, not three.
- *Amended 2026-09-03:* the ceiling block counts three arms; the hook counted
  five walls at that stamp. Docs that enumerated the ceiling's subjects as
  "check 3's two arms" say three.
- *Amended 2026-09-05:* four walls again — the fifth was ADR 0051's
  commit-time citation arm, removed by operator ruling (ranger-base-bp0yj),
  leaving the ceiling, the beads visibility guard, the constitution-path
  guard and the shared-index guard. Read the count off the hook's own head
  comment, which `dataceiling_qa_test.go` asserts against the rendered hook;
  the bullet above is a dated stamp and this page is prose, which is why this
  is the copy that drifted (ranger-base-2bijx).

## Alternatives rejected

- **A per-pattern scope flag inside the existing key (the bead's other
  option).** Flat-YAML gives one value line, so the flag would be a token
  in front of the ERE — a mini-grammar whose parse failures cannot quote
  the value (8114t) and whose first token collides with any ERE that
  starts with the same letters. And a flag does not save the second set of
  words: a visibility hit says "re-file it in the private db", a ceiling
  hit says "this may not exist here" — two rules, two remedies, two
  footers, so two keys is the honest count, and the key name is the scope.
- **A class-name convention (a `ceiling-` prefix widens the scope).** Puts
  the scope inside the one string the refusal prints; a typo in the prefix
  silently demotes the wall to visibility scope with no refusal to say so.
- **Mark the work repo public.** Context, second bullet: the shipped list
  would refuse the instance's own legitimate ops beads, and the stamp
  would stop being the visibility record.
- **A third stamp value (`private` plus a per-repo ceiling switch).** The
  ceiling is a fact about the instance, not about one repo; a per-repo
  knob has to be set on every repo and is unset on the next one, which is
  exactly the repo a paste lands in.
- **A runtime-side gate on the paste itself (the clever one).** A
  PreToolUse gate over Write/Edit payloads and bd argv would refuse
  restricted content BEFORE it reaches any file — closer to fact (b)'s
  letter than a commit wall. Rejected as the first wall: it covers only
  the runtime's tool path (not the operator's own paste, not `bd sync`,
  not a hand edit), it is per-runtime while the commit hook is one choke
  point every entry path crosses, and it would need the ERE in a gate
  script that the harness's own hook-gate logging can echo. It is a
  legitimate SECOND layer; the trigger for filing it is the first ceiling
  refusal in refusals.log on the work box, which is the measurement that
  the class exists in practice.
- **Reuse check 0's bead-shaped remedy.** A ceiling hit in the jsonl has
  no private db to be re-filed into; sending the writer there is the wrong
  door.
- **Exempt the `data_ceiling_patterns:` block of config.yaml from the
  staged-file arm (the bead's option (b)).** Rejected: a hole the size of one
  file in a wall whose point is every file, keyed on a path the per-repo hook
  does not know and would have to be stamped with; and it is unnecessary,
  because the bracketed spelling is the same regex with the same reach and
  costs no mechanism. *(2026-09-28, ranger-base-3gdqv.)*
- **Refuse a self-matching value at stamp time, as the validator refuses a
  bad ERE.** Rejected: that drops the wall for the class until the operator
  edits, when the value works everywhere but one line. A warning keeps the
  wall up and the commit refusal that follows now has a line that explains
  it. *(2026-09-28.)*
- **Rewrite the value in the harness (bracket the first character at read,
  as ceiling-fill.sh does at typing).** Rejected: the hook would then enforce
  a regex that is not the config's text, breaking "what is in force is what
  the file says" and the byte-for-byte probe's meaning; and a first character
  of `[`, `\` or `^` needs a grammar. The rewrite belongs where the value is
  typed. *(2026-09-28.)*
- **The same warning for `beads_visibility_patterns:`.** Deferred, not
  rejected: the property holds, but only where the config's repo is stamped
  public, so the line would hedge on a stamp the reader of the report already
  sees, and the instance the ceiling exists for stamps that repo private.
  File it on the first refusals.log line that shows it. *(2026-09-28.)*
