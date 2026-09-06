# ADR 0024 — work product vs the public tree: the routing rule, and a prose wall at commit

*Status: accepted 2026-08-27 (operator ratified; the D4 ceilings blessed
— as a published SUGGESTION, not a shipped default: see D4) · D4
migration executed and recorded 2026-09-06 (ranger-base-mppjc) ·
owner: architect ·
extends ADR 0012 D2 and the beads visibility guard (rangerhq-hrz) · amended
2026-09-04 (D2 check 3: a FOURTH derived source, the crew names, over the
ADDED staged PATHS alone — ADR 0012 D2 / App.A 5 at commit time,
ranger-base-cdxpf, from ranger-base-o3g6a) · amended
2026-09-04 (D2 check 3 and Residuals: the identity literals are grepped over
ALL staged files, not all staged "text" files — the reader carries `--text`
because git's binary call flips on one NUL; ADR 0048 D2 as amended,
ranger-base-9307c, from ranger-base-h137b) · amended 2026-09-03 (D2 check 3
and Consequences: the commit MESSAGE is a THIRD subject of check 3 — product
decision that date on ranger-base-1nbtn, landed 2026-09-04 in
ranger-base-qk8i9, which is why this line sits after the 09-04 one above)*

## Context

The operator's directive (2026-08-27): clear delineation between work
product and public pushes — moments after an incident RCA, written by a
session whose worktree was a public-repo worktree, nearly landed on
public main. It was caught by a human reading a branch, which is the
control this ADR exists to replace.

What already exists covers beads, not documents. ADR 0012 D1 draws the
harness/instance boundary; NOTES.md "Privacy model" routes *beads* by
class; the hrz guard refuses ops-class content in the ADDED lines of
`.beads/*.jsonl` in a public-stamped repo. Nothing covers: an RCA or run
record written into `docs/`, a runbook that hardcodes one deployment's
paths, or prose in NOTES/ADRs quoting live spend. A migration audit this
week (D4) found live examples of every one of those classes already in
the public tree — the boundary held only where the guard existed.

## Decision 1 — the classification rule, made mechanical

The test does not change; its scope does. ADR 0012 D2, applied to every
artifact, not only beads and smarts:

> An artifact belongs in the public tree iff **any deployer of this
> software could have written it.**

Two mechanical arms, so a session applies it without judgment:

**Genre arm — decidable before writing a word.** A document that
*narrates one deployment* has no public home at any level of scrubbing:
RCAs, incident timelines, postmortems, run records, audits of live
state, and procedures whose steps hardcode one instance's paths or
one-time dates. Its public form is not a redacted copy — it is the
invariant the incident taught, restated in an ADR or NOTES (0012 D2,
verbatim). Consequence: the public tree has no `docs/rca/`,
`docs/incidents/`, or `docs/postmortems/` — a session asked to write an
RCA writes it in the instance tree, however the session was launched and
whichever worktree it holds.

**Content arm — decidable by scan.** Inside legitimately public genres,
the instance-content classes are the hrz set plus one: **cost** figures,
**plan** names/sizes, live **guard** values, **credential** maps — and
**identity**: the operator's username, e-mail, and the instance repo's
path. Identity literals cannot ship in the pattern list (the list would
itself be the leak); they are derived on the box (D2 §3).

The default and its asymmetry, verbatim from the beads rule: **when in
doubt, instance.** A private artifact can be restated public later; the
reverse is a history purge.

## Scan coverage (folded from 0048)

`beads_visibility_patterns:` is instance vocabulary. Apply its entries with
check 3 to added bytes/lines of **every** staged file, every added path and
the commit message under the cleanup semantics below, only in public-stamped
repos. Use the staged diff with `--text`; NUL, extension and attributes do
not grant exemptions. Renamed destinations count as added paths. The
shipped `OpsPatterns` remain check 2's markdown-only content scan, with
the same byte-safe diff reader; do not scan their own source as instance
vocabulary. The existing operator override and class-only refusal/log
remain. A pre-publication name belongs in instance config, never a public
pattern literal derived by guesswork from another repo's basename.

The post-landing seed-name pin remains a backstop for absent config and
overrides; it names the commit-time arm. Docs-only commits are included.
MEASURED: seven earlier recurrences (ranger-base-9ubk6) and the one-NUL
bypass (ranger-base-h137b). Rejected: another reword, a code-file allowlist,
a binary exemption, a whole-blob second scanner, or running the full-tree
test from the hook. No mechanism is removed by this fold. [0050](0050-data-ceiling-patterns.md)
continues to own the **all-repository** data ceiling; these keys and scopes
are deliberately distinct.

## Lineage

| Was | Here |
|---|---|
| 0048 D1–D4 instance vocabulary, complete staged scope, message and seed pin | Scan coverage and Decision 2; 0050 remains an independent ceiling |

## Decision 2 — the wall: three checks in the existing commit hook

The enforcement point is the `prepare-commit-msg` hook the harness
already installs — the layer that refuses at the typed line. The slot's
reasons carry over unchanged (bd silently reinstalls `pre-commit`;
`--no-verify` skips `pre-commit` but not this slot), and so does the
coverage argument: linked worktrees share the common hooks dir
(MEASURED, git 2.39.3, rangerhq-b38m), so one install covers every
persona worktree; the launcher's bead-close fast-forward moves commits
and creates none, so commit time is the last moment content is written
anywhere. All three checks run only in a **public**-stamped repo, on the
stamp mechanism hrz already has (unmarked is public; refreshed at
install and at every persona launch).

1. **Docs-genre allowlist.** A staged **new** file under `docs/` — move
   detection off, so a move INTO an unlisted genre is a new file like any
   other (ranger-base-60azj: with detection on git pairs a `docs/` ->
   `docs/` move into one `R100` entry that `^A` never matched, and the
   public tree gained a `docs/rca/`) — must sit in an allowlisted
   subdirectory — today `adr/`, `runbooks/`,
   `notes.d/` — a shipped constant beside `OpsPatterns`, not a config
   key: admitting a genre to the public tree is a reviewed code change,
   which is exactly the review the wall exists to force. An unknown
   subdirectory is refused naming both ways through: write it in the
   instance tree, or add the genre deliberately. Fail closed, the same
   shape as unmarked-is-public.
2. **Prose content scan.** `OpsPatterns` over the ADDED lines of every
   staged markdown file, any path — `.md` and `.markdown`, matched
   case-insensitively (`MarkdownPathspecs`; git pathspecs are
   case-sensitive, so the earlier bare `*.md` never saw `x.MD` at all,
   ranger-base-4b1z4) — same list, same two readers, same
   ERE-dialect intersection the init-time panic already enforces. Not
   over code: the detector's own source and tests are byte-identical to
   hits (the assembled plan-brand names in `visibility.go` exist because
   of precisely this), and a wall that carries an allowlist of its own
   files is a wall with a hole list.
3. **Identity literals.** At hook render time the installer derives,
   from the box itself, the literals that have no legitimate public use
   anywhere — code included: `whoami`, `git config user.email`, and the
   instance repo path (dirname of the `.beads/redirect` target, rendered
   in both `~`-relative and absolute forms; the redirect is verified
   present and absolute in this tree). These are grepped over the ADDED
   lines of **all** staged files — whatever their bytes: the reader
   carries `--text`, so a NUL neither exempts a markdown file nor a real
   blob (ADR 0048 D2 as amended 2026-09-04) — and over the ADDED staged paths,
   move detection off so a move's destination counts as new (MEASURED: a
   pure move yields no added lines at all, ranger-base-wlsv1). A
   filename is exactly where an operator-shaped artifact puts the
   operator; added ENTRIES is the path analogue of added lines, and it is
   check 1's rule verbatim — a modified existing path cleared this the
   day it was added, and a deletion carries a path away.

   *Amended 2026-09-03 (ranger-base-1nbtn, product; built in
   ranger-base-qk8i9): a THIRD subject — the commit MESSAGE.* The hook is
   handed the message file as its first argument; check 3's literals and
   (per ADR 0048 D2) the instance patterns are matched over every line of
   it as given, comment-looking lines included, with the same words,
   override and log shape, tail = the content arm's existing tail with
   ", commit message)" in place of its closing paren. The principle is
   check 3's own: these strings have no legitimate public use anywhere,
   and a message replicates with the branch. MEASURED over the 1136
   messages then on main: identity literals in 5, four of them the class's
   real target; this instance's one config pattern in 18, every one
   avoidable by the ADR 0048 habit of saying "the pre-publication name".
   The refusal costs one re-issued command: git keeps the refused message
   in `.git/COMMIT_EDITMSG`. A message typed in the EDITOR is the stated
   exclusion — `prepare-commit-msg` runs before the editor opens and is
   handed git's template alone (measured); that path is the operator's own
   hand, and the second layer for it is a `commit-msg` hook. *Amended
   2026-09-04 (ranger-base-h3s6q): the arm used to SCAN that template and
   refuse over lines git strips. It now reads through `git stripspace
   --strip-comments` on every path but `-m`/`-F`, where git's cleanup keeps
   a `#` line and the file is still read whole. Same renderer, so this
   holds for the ceiling's arm and check 3's alike.*

   *Amended 2026-09-04 (ranger-base-6y3z2): `$2` was a PROXY for git's
   cleanup mode, and it breaks the moment `commit.cleanup` is set — one
   line in `~/.gitconfig`, no intent required. Under
   `commit.cleanup=verbatim` git KEEPS its own template in the commit
   object, the `On branch` line and the UNTRACKED file list included, and
   the arm stripped exactly those lines out of the scan: an identity
   literal carried by a branch name or an untracked path reached a PUBLIC
   repo's commit object with no wall speaking (measured, git 2.50.1). The
   arm now reads `commit.cleanup` and strips only under `strip`;
   `verbatim`, `whitespace` and `scissors` are read WHOLE, and `$2`
   survives as the proxy for `default` alone. `git commit --cleanup=...`
   is invisible to a hook and is the stated residual — measured, `git
   config --get` answers for the config files and for `git -c`, not for
   the flag.*

   *Amended 2026-09-04 (ranger-base-b21e0): the remedy, not the verdict.
   Under those three modes the writer is refused before the editor opens,
   possibly over a line git wrote, and "rewrite the commit message" named a
   rewrite that had not happened. The refusal now names the live mode and
   what clears it — clear the class out of the repo, or leave
   `commit.cleanup` at its default — and stays silent where `$2` is
   `message`, since git appends no template there and the old remedy is
   doable as written. See ADR 0050 D2 for the `scissors` half.*

   *Corrected 2026-09-04 (ranger-base-sx2dq, from ranger-base-md7ui): that
   remedy first read "delete git's block in the editor before you save",
   which the writer being refused cannot do — `prepare-commit-msg` runs
   BEFORE git launches the editor and its non-zero exit ends the commit, so
   there is no editor session. Measured: the refused commit invokes
   `GIT_EDITOR` zero times under all three kept modes, the landing control
   once. See ADR 0050 D2.*

   *Amended 2026-09-04 (ranger-base-cdxpf, from ranger-base-o3g6a): a
   FOURTH derived source — this box's CREW NAMES, over the ADDED staged
   PATHS and nothing else.* The staged-path arm above matches the
   identity literals, which a file named after a SEAT carries none of; a
   probe named for a QA seat was added, committed clean, and rode main
   for a day until ADR 0012 App.A 5's two pins were taught to read a
   file's name. The suite catching it afterwards is not the wall. So the
   names are derived at render time exactly as the other three are —
   `ListAgents` over this home's `agents/`, LESS every name posse has
   ever shipped an example PID under — matched case-insensitively and
   with no word boundary, because the separator in Go's own file names
   is `_` and no boundary fires beside one (measured, ranger-base-o3g6a).
   Its refusal is its own: ADR 0012 D2's remedy is to name the file for
   the ROLE, not ADR 0024 D3's restate-and-cite.

   THE THREE DECISIONS this carries, all measured over this repo's
   tracked paths. **Paths only**, not check 3's three subjects: a crew
   name in a staged LINE is legitimate where ADR 0012 D2 leaves it —
   `docs/`, the root narrative, a D6-grandfathered id — and a commit
   message names the persona who wrote it, so a content or message arm
   would refuse what this constitution allows. **One tree**, and this
   arm alone is narrowed that way (amended 2026-09-04,
   ranger-base-p7e0z): ADR 0012 D6's edge is "the tree, not the syntax",
   so it bounds the NAMES as well as the lines — the arm reads the paths
   that ship as code and skips `docs/` and the repo root's narrative
   files. As landed it had no root filter, refused an ADR standing on
   main today, and cited the very rule that permits it; an operator
   identity literal in that same path is still refused, because App.A 5
   is what has a tree and D3 does not. **Derived, less the shipped
   roles**: a hardcoded crew list in the tree would BE what App.A 5
   forbids, and the seed staffs a fresh home with ROLE names — the
   depersonalized vocabulary D2 tells a writer to rename TO — so a wall
   built from those would refuse its own remedy. The census is
   one-sided enough to decide it: over 830 paths at the fix this
   instance's 11 PID names hit 1 path and the 9 names the seed ships
   hit 285, one of them 273 on its own — every QA test file in the
   tree. That single hit was an ADR under `docs/`, which is why the
   third decision exists: re-censused over 841 paths, the staffed PIDs
   still hit only it, and ZERO inside the tree the rule governs. The
   residual, stated: a PID named for a common word is a substring match
   over every path in that tree and refuses honest commits until the
   lane is renamed. It cannot be silent — the refusal names the path and
   the persona, and the override is one env var.

   The literals live only in the
   rendered hook under the repo's hooks dir — never in a commit, never
   in the shipped list. The renderer refuses a literal containing a
   single quote, the same init-panic class as the pattern list. The bead
   id prefix does not trip this: the literal is a path with slashes, the
   prefix a hyphenated word.

Refusal, override, and honesty are hrz's, verbatim: the refusal names
the matched class, the rule, and the way through; the override is
`RHQ_VISIBILITY_OVERRIDE=i-mean-it`, operator-typed, never in a
session's environment, logged to `refusals.log`. **And it is a lint,
not a boundary** — the boundary is the routing rule; the lint turns a
mis-route into a refusal at the typed line instead of a public artifact.

## Decision 3 — dual-nature documents: restate-and-cite

When a public document wants a private fact, the pattern — already
practiced by ADR 0012's provenance header, the hrz "numbers live in a
private bead, cite its id" rule, and the public ADR restatements — gets
a name and becomes the only sanctioned shape:

1. **Instance-first.** The full document — narrative, numbers, names —
   is written in the instance tree, complete, first.
2. **Restate, never excerpt.** The public document carries the mechanism
   and the invariant; measured numbers become defaults with the
   rationale restated (not the measurement quoted); persona names become
   roles; incident narratives become the invariant taught.
3. **Cite inert.** The private artifact is cited by bead id or a
   private-path label — a provenance marker that promises nothing will
   resolve it publicly.

This ADR practices its own rule: D4 names classes and counts; the
line-precise inventory lives in the proposing bead's comments, in the
private db.

## Decision 4 — migration: what is already public

Audit MEASURED 2026-08-27, whole-tree grep of `docs/`, NOTES, README,
DIRECTION, AGENTS with the D1 content classes:

- **Cost class, five sites:** one NOTES section pricing a real day's run
  (spend figures plus the operator's working hours), one NOTES rendered
  brake line with live spend, the same pair twice in ADR 0018, a
  measured per-bead cost twice in ADR 0020. **Scrub:** restate each as
  the invariant with placeholder figures.
- **Identity class, one site:** a NOTES line naming the operator's local
  brew tap (carries the username). **Scrub.**
- **Ceilings, one judgment:** ADR 0018 states two dollar ceilings as
  accepted design bounds. Either **bless** them (converts
  what-is-set-HERE into public vocabulary; they are already published) or
  scrub to placeholders. Recommended: bless. Operator's call, on the
  ratification bead. **BLESSED** on ranger-base-axft — and the word used
  for it, "shipped example defaults", is wrong and is corrected here
  (2026-09-06, ranger-base-mppjc). MEASURED that day: `budget_pass:` and
  `budget_day:` have no default anywhere in the code — `BudgetCaps` reads
  config and nothing else, and unset is Dial E *dormant* (`posse cost`
  says so in as many words). In `examples/config.yaml`, the file `posse
  init` copies verbatim into `$RHQ_HOME`, both lines are **commented
  out** and labelled "suggested". So what was blessed is a published
  suggestion carrying the operator's two numbers — enough to give ADR
  0018's worked example a public anchor rather than a placeholder, which
  was the point — and no shop is shipped those ceilings armed. Saying
  "shipped default" of a commented line claims the opposite of what a
  fresh `posse init` does.
- **Genre class, three files:** `docs/runbooks/home-cutover.md`,
  `retirement-window.md`, `queue-cutover.md` are one-deployment
  procedures naming instance paths throughout — move to the instance
  tree; their mechanism already lives in ADRs 0012/0015. `release.md`
  is any deployer's process and stays.
- **Credential class: zero hits** outside the software's own documented
  mechanism (MEASURED, same sweep).
- **History:** the scrubbed strings remain in published history; the
  clean-repo constraint (ADR 0012) rules out a purge, and the exposure
  is spend figures and a username, not credentials. Accepting that is
  recorded instance-side, not here.

**D4 is executed** (recorded 2026-09-06, ranger-base-mppjc; the list above
is kept as the audit it was, not as pending work). RE-CHECKED that day
against the standing tree: the cost class is gone from `NOTES.md` (no
`$N.NN` figure survives the sweep) and from ADR 0020; the identity class
is gone (the brew-tap lines name the formula, no username); the three
one-deployment runbooks are absent from `docs/runbooks/`, which keeps
`release.md` and the later general ones; the credential class was zero
hits then and the shape table below is what holds it there; the ceilings
were blessed, with the correction above. The standing bar is not this
list — it is the 2026-09-02 amendment's shape table and its pin.

**Amendment 2026-09-02 (ranger-base-imiif → ranger-base-lm22v): the bar
over the residue.** The migration above scrubbed the sites it named; what it
leaves behind is *residue*, and this is what the residue is held to.
RE-MEASURED 2026-09-02 over the standing tree with the four shipped EREs: 78
hits on 67 lines in 180 tracked markdown files, every class represented, and
every one of them inside the SOFTWARE's own vocabulary — a vendor's public
list price, a vendor's window mechanic, the sentinel and default values
`examples/config.yaml` documents, and the credential-store paths ADR 0019 is
about. Each is now dispositioned by a ruled SHAPE, in a pin that fails
naming path, line, class and matched text
(`internal/posse/opsresidue_qa_test.go`). The two dollar ceilings blessed on
ranger-base-axft got their public anchor in the same landing: they are
restated in `examples/config.yaml` as suggested example values — commented
out, since arming Dial E is the reader's choice — so the ADR 0018 worked
example quotes a published suggestion rather than what is set here.

So the D2 check-2 bar over the standing tree is **zero hits OUTSIDE the shape
table, never zero**: ranger-base-99ps's DONE WHEN said zero and was
unmeetable by construction, since the credential class exists to name the
mechanism ADR 0019 documents and the ADR documenting it can therefore never
be clean. Adding a shape is a reviewed edit, the same class as
`PublicDocsGenres` — a table anyone may widen is not a bar.

That bar is MARKDOWN-ONLY by construction, the same scope D2 check 2 states
and not over code, because in source the fixture shape is the target shape
(MEASURED at 53170e9 with the four shipped EREs over every tracked
non-markdown file: 434 line-hits in 93 files, nearly all fixtures and
detector vocabulary), so source is held by check 3's derived identity
literals and by a value-equality sweep at verify time rather than by this
shape table — read a green pin as a tree-wide property and you will report a
clean tree having measured a quarter of it (ranger-base-4v7f9).

## Consequences

- The near-miss class becomes a refusal: a session cannot land an RCA,
  a live figure, or the operator's name in the public tree without the
  operator typing the override.
- A new public docs genre costs one deliberate constant edit — review
  by construction, mild friction accepted.
- Residuals, stated: ops-class prose in code comments is unscanned
  (detector-source problem); non-markdown prose is unscanned by check 2
  (check 3 still covers it); a real blob's bytes ARE scanned by check 3 and
  by every instance pattern since ranger-base-h137b, and a genuine asset
  that trips a class goes through on the typed override, never on an
  exemption (ADR 0048 D2 as amended: the exemptions were each priced and
  each is a hole the writer controls) — and since ADR 0048 D2 this instance's own
  config patterns are scanned everywhere, moved out of check 2 into check
  3's scope, because the detector-source argument is about the SHIPPED
  list and a config pattern is never in source; a determined paraphrase
  walks past any regex. Check 3 gained a path arm and check 2 deliberately did not: a
  runbook NAMED after a plan brand still passes, because that class has
  the detector-source residual check 3's literals do not, and its
  false-positive number over a path list is unmeasured. This wall scans
  added lines, added paths and, since 2026-09-03, the commit message (check
  3 and the ceiling only) — never commit METADATA: the author field is
  whatever `user.email` resolves to for that commit and is the operator's
  to set — which is why the e-mail literal is derived from EVERY config
  scope (ranger-base-yqstz), not the one the repo happens to resolve to,
  since a repo-local override is exactly how one box signs as two people.
  Every check in this wall is inside the visibility gate, so a repo stamped
  `private` runs none of them — by design for visibility, and a hole for an
  instance holding someone else's data, whose bead repo is private on
  purpose; ADR 0050 adds a second key, `data_ceiling_patterns:`, rendered
  ABOVE the gate with all three of check 3's arms — added lines, added
  paths and, since 2026-09-03, the commit MESSAGE, which both walls read
  through one renderer (built in ranger-base-nfg8l, the ceiling's message
  arm in ranger-base-o2v6n, check 3's in ranger-base-qk8i9) — refused by
  class alone in every hooked repo:
  visibility says where content may go, the ceiling says whether it may
  exist here at all. The boundary remains the routing rule plus repo
  visibility. Check 2's shipped list does NOT scan the message, by census
  rather than by the detector-source argument (which is about SOURCE and
  does not reach a message): over the same 1136 messages the shipped list
  hit 29, of which 22 are the software's own vocabulary — fixture figures,
  blessed defaults, documented key values, credential-store file names —
  the residue shapes the markdown bar dispositions by table, and a message
  has no table; the seven real hits all predate 2026-08-30. A pin
  (`TestQAShippedPatternsDoNotScanTheCommitMessage`) holds the decision;
  the trigger for re-opening it is a live figure found in a message at
  verify time.
- The identity-literal false-positive rate is ASSUMED low (the literals
  are paths and an e-mail); the build bead's done-when includes the hrz
  method — measure against the post-scrub tree and require zero hits.
  MEASURED for the path arm at ranger-base-dmsbu: all four literal forms
  over this repo's full tracked path list, zero hits, and kept as a
  standing pin rather than a one-shot count.

## Alternatives rejected

- **Scan all staged files with `OpsPatterns`:** self-tripping detector,
  measured precedent above; the exception list becomes the hole list.
- **Gate at the launcher's publish step:** too late (the commit already
  exists in every worktree and branch history), wrong actor (the
  launcher runs as the operator, training override-typing), and the
  one real near-miss was caught pre-publish only by luck.
- **A dispatch-prompt rule alone:** "a persona remembers" is not a
  control (hrz, verbatim). Kept as a PID habit, counted as nothing.
- **A new hook slot:** the slot decision was made on measured behavior
  once; a second hook re-litigates it and doubles the install surface.
- **A committed denylist of operator identifiers:** the list is the
  leak. Render-at-install is the whole trick.
- **Per-file visibility front-matter:** a tag the author writes is the
  judgment the wall exists to replace, and untagged files need a
  default — the default *is* the rule, so the tag adds nothing.
