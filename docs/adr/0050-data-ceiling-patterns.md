# ADR 0050 — A data ceiling is a second pattern key that walls every repo the instance hooks, whatever its visibility stamp

*Status: accepted 2026-09-02 (ranger-base-zrkl6, from ranger-base-3i035 /
ranger-base-w9jv (b)) · owner: architect · extends ADR 0024 D2 and ADR
0048 D2 · builds in ranger-base-nfg8l (code, dinesh); the posture
doc and the work-install runbook amend in ranger-base-83crg (security, hoover) ·
number: 0043–0045 stay pre-named by ADR 0040 §2; per 0040 §3.1 this file
takes the next number no bead has claimed.*

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
  patterns over every staged text file and added path — under one shell
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
  the third is a PATH — which is why check 3's two-arm scope (added lines,
  added entries) is the right shape and check 2's markdown scope is not.
- **The renderer already exists.** ranger-base-uzgkz (c9a4cdd, d6022fc)
  and ranger-base-8114t (d505f2c) give check 3 one refusal shape with the
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
gate, with check 3's two arms.** The `prepare-commit-msg` hook renders the
ceiling scan FIRST — before `posse_beads_visibility` is tested — over the
ADDED lines of every staged text file and the ADDED staged paths, using
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
session transcript or the pane capture; the paste already happened when
the wall speaks. Routing plus the operator's hand stay the boundary above
the ceiling (f85 §4). The posture's §3 (ii) sentence "as built, the ceiling
has no wall" becomes "the ceiling has a wall at the commit; above it,
routing" once the code bead lands.

## Consequences

- Every hooked repo's render changes; the L3 probe reads them all as "ours
  but stale" until `posse gates install-hooks` re-runs against the
  installed binary — expected once, as ADR 0048 already paid.
- A private repo pays one full `git diff --cached` per commit that it did
  not pay before — the same scan check 3 already runs in public repos.
  ASSUMED: same cost class; the public repo has run it on every commit
  since ADR 0024 D2 without a bead about it.
- The work instance's config gains the key at M2, values written by the
  operator on the work box (a PromotedPath; a persona cannot). The class
  names are hoover's §3 (ii) list; the values never leave that box. On
  THIS instance the key stays absent and the block renders nothing.
- NOTES.md "When an instance holds someone else's data", INSTALL.md's
  visibility paragraph and examples/config.yaml each gain the key beside
  its sibling, with the one-sentence distinction: visibility says where
  content may go, the ceiling says whether it may exist here at all.
- The hook's head comment counts four walls in the slot, not three.

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
