# ADR 0024 — work product vs the public tree: the routing rule, and a prose wall at commit

*Status: accepted 2026-08-27 (operator ratified; the D4 ceilings blessed
as shipped example defaults) · owner: architect ·
extends ADR 0012 D2 and the beads visibility guard (rangerhq-hrz)*

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
   lines of **all** staged text files, and over the ADDED staged paths,
   move detection off so a move's destination counts as new (MEASURED: a
   pure move yields no added lines at all, ranger-base-wlsv1). A
   filename is exactly where an operator-shaped artifact puts the
   operator; added ENTRIES is the path analogue of added lines, and it is
   check 1's rule verbatim — a modified existing path cleared this the
   day it was added, and a deletion carries a path away. The literals live only in the
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
  accepted design bounds. Either **bless** them as shipped example
  defaults (converts what-is-set-HERE into public vocabulary; they are
  already published) or scrub to placeholders. Recommended: bless.
  Operator's call, on the ratification bead.
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
restated in `examples/config.yaml` as suggested example values, so the ADR
0018 worked example quotes a shipped default rather than what is set here.

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
  (check 3 still covers it) — and since ADR 0048 D2 this instance's own
  config patterns are scanned everywhere, moved out of check 2 into check
  3's scope, because the detector-source argument is about the SHIPPED
  list and a config pattern is never in source; a determined paraphrase
  walks past any regex. Check 3 gained a path arm and check 2 deliberately did not: a
  runbook NAMED after a plan brand still passes, because that class has
  the detector-source residual check 3's literals do not, and its
  false-positive number over a path list is unmeasured. The wall scans
  added lines and added paths, never commit metadata: the author field is
  whatever `user.email` resolves to for that commit and is the operator's
  to set — which is why the e-mail literal is derived from EVERY config
  scope (ranger-base-yqstz), not the one the repo happens to resolve to,
  since a repo-local override is exactly how one box signs as two people.
  Every check in this wall is inside the visibility gate, so a repo stamped
  `private` runs none of them — by design for visibility, and a hole for an
  instance holding someone else's data, whose bead repo is private on
  purpose; ADR 0050 adds a second key, `data_ceiling_patterns:`, rendered
  ABOVE the gate with check 3's two arms and refused by class alone in
  every hooked repo: visibility says where content may go, the ceiling says
  whether it may exist here at all (built in ranger-base-nfg8l).
  The boundary remains the routing rule plus repo visibility.
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
