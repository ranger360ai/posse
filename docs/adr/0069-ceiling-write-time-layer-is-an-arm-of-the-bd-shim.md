# ADR 0069 — The data ceiling's write-time layer is an arm of the bd shim: every argv word and every regular file bd is told to read, judged by the hook's own `grep -E`, refused class-only, with no override

*Status: accepted 2026-10-08 (ranger-base-km9jt, from ranger-base-6pisf / ADR
0068 D3, triggered by github.com/ranger360ai/posse/issues/1 — the first
ceiling refusal in refusals.log on the work box, which is the trigger ADR 0050
Alternatives wrote down) · owner: architect · builds ADR 0050's "legitimate
SECOND layer" and amends its Alternatives entry in place · builds in the code
beads named on ranger-base-km9jt (dinesh)*

> The commit wall now REPORTS over a bead record because nobody at the commit
> can remove the content (ADR 0068). The only place a bead's ceiling content
> can be REFUSED with a remedy the writer can perform is the write itself:
> retype the comment with the system of record's id. This record puts that
> refusal where every runtime's bd call already crosses posse — the L1 shim —
> and keeps it the size of the hook's own matcher.

## Context (MEASURED 2026-10-08, bd 0.50.3, python 3.14, unless marked)

- **Every persona already runs bd through a posse shim.** All 9 shipped PIDs
  and all 11 of this instance's carry a `Bash(bd …)` deny, so `RenderGates`
  renders `gates/<persona>/bin/bd` for each of them at every launch, option-
  aware, and `make pid-check` (`piddenyset_qa_test.go`, ranger-base-d866) pins
  that every PID carries that fence. The shim runs for claude and codex alike,
  and inside a container `posse gates wrap` renders the same gates with a
  cross-compiled posse beside a linux bd (cage.go). It is cooperative class
  (ADR 0025 §1): an absolute path to bd or an emptied environment walks around
  it, and the commit hook stands behind it.
- **What the shim can see is what bd will write, minus stdin.** Census over
  every session transcript on this box (375 files, 2025 bd write calls —
  create 459, comments add 1043, close 445, update 64, defer 13, q 1):
  1339 carry the text as literal argv (66%), 129 as a `"$(cat <<'EOF' …)"`
  substitution, which is argv by the time bd runs (6%), 378 name a regular
  file with `-f`/`--file`/`--body-file` (19%), 164 name `/dev/stdin` under a
  heredoc (8%), 11 say `-f -` or `--body-file -` (0.5%), 4 other. So 91%
  of what the crew writes is on the argv or in a file the shim can open, and
  8.6% arrives on bd's stdin, which a shim cannot read without consuming it.
  `--metadata @file`: 0 of 2025.
- **The shim's existing refusal echoes argv, and so would breach the ceiling
  by the wall's own hand.** `posse_refuse` prints `$*` to stderr and writes
  it to refusals.log (gates.go `renderShim`); a ceiling hit must not go
  through it. `shell.log` records only the PATH-reorder line, never argv, and
  nothing logs a shim's file — so the ERE sitting in the shim is the same
  fact as the ERE sitting in `.git/hooks/prepare-commit-msg` of every hooked
  repo, which ADR 0050 D2 already accepted.
- **The PreToolUse gate is the wrong host three times over.** It is claude-
  only (ADR 0014 §5, ADR 0015 §3); it is an operator-installed copy posse
  never renders (`verify-gate-freshness.sh` exists because that copy goes
  stale silently); and its one refusal funnel `at()` quotes the matched
  segment in `permissionDecisionReason`, which the runtime shows to the model
  and writes into the transcript — that is the "hook-gate logging can echo"
  concern of ADR 0050, made concrete. And Python's `re` is not the hook's
  judge: `re.compile('[[:upper:]]+-RESTRICTED')` — the shipped example value
  — compiles with a `FutureWarning` and then matches `ACME-RESTRICTED` as
  **False**, where `grep -E` and Go's `regexp` both match it (ADR 0050 D6
  measured those two agree over nine values). A layer that judges by a
  different dialect than the wall is the yplnv drift one reader over.
- **The hook's matcher is already a rendered shell function.** `posse_check
  <class> <ERE> 1` counts hits over `$posse_added` with `grep -cE` and
  accumulates `<class>: N hit(s) — pattern and matched text withheld`
  (gates.go ~3491); `dataCeilingCheck` renders one such call per accepted
  ceiling class from the same `OpsPatternSet()` every renderer reads (ADR
  0050 D3). Nothing in it is specific to a diff.
- **Cost of the arm, prototyped in sh with the two example classes:** 15–24
  ms per bd call over a short argv or an 8 KB `-f` file (20 calls: 0.30–0.48
  s wall, darwin 25.4.0). `posse version` is 10 ms, for the comparison the
  Alternatives need.

## Decision

**D1 — host: the bd shim, rendered by `RenderGates` from `OpsPatternSet()`
at launch.** `renderShim` gains one arm for `cmd == "bd"`, rendered by a
function beside `renderSequencerAudit` (the shim's existing non-deny arm),
placed after the verb rules and before the `exec`. The ceiling list rides in
the same read every other renderer uses (ADR 0050 D3), so the hook and the
shim never disagree about which classes are in force. When the list is empty
— this instance, where the key is absent — the arm renders nothing and the
shim is byte-identical to today's. Shims are re-rendered at every launch, so
a key added to config reaches the next session without an install step.

**D2 — subjects: every argv word of every bd call, plus the content of every
REGULAR file bd is told to read; stdin never.** No verb list. The hook scans
every staged byte (ADR 0048 D2) and the ceiling does not key on ids (ADR 0050
Context), so scanning `bd show <id>` costs a grep and refuses nothing, while
scanning `bd search '<banner>'` refuses a literal that is itself a paste —
the cite is the sanctioned spelling there too. Files: the value after `-f`,
`--file` or `--body-file`, in both the separate and the `=` spelling, is read
with `cat --` only when it is a regular file (`[ -f ]`) whose path is not `-`
and does not begin `/dev/` or `/proc/`; `/dev/stdin` backed by a regular
redirect is skipped by the prefix, measured, so the arm can never consume
what bd was about to read. `bd close -f` is `--force`: the word after it is
an id or a flag, not a regular file, and if a file of that name does exist in
the cwd the arm reads it, which is a read and not a change.

**D3 — judge: the hook's own `posse_check`, over `grep -E`, class-only.** The
arm renders the same `posse_check` function and the same
`posse_check <class> <ERE> 1` lines `dataCeilingCheck` renders into the hook,
with `$posse_added` set to the argv words one per line followed by the file
contents. One dialect for the layer and the wall, by construction.

**D4 — words: its own refusal function, never `posse_refuse`.** On a hit the
arm prints to stderr: `refused by posse gate: bd argv carries data-ceiling
content — bd shim, session <persona>`; the accumulated class lines (class and
hit count only); the rule (`DataCeilingRule`'s sentence: this content may not
exist in a local file at all; cite the system of record's id); the remedy —
*retype the bd command with the cite in place of the paste; for a `-f` file,
remove the paste from the file first* — and the footer *this wall runs under
every visibility stamp; the commit hook stands behind it and REPORTS what
reaches the db (ADR 0068)*. refusals.log gets one line, `<stamp> data ceiling
scan [bd shim] (bd argv) session <persona>` — class names may follow, argv
never. Exit 1; bd is not exec'd; the db is untouched.

**D5 — no override.** `RHQ_VISIBILITY_OVERRIDE` is not consulted. The hook's
override exists because a refused commit can hold content the committer did
not write; here the writer is the one typing, the remedy is always
performable, and a pattern that is wrong is the operator's to change in
config — the same last sentence `DataCeilingWayThrough` already carries.

**D6 — what this layer does NOT cover, by name and number.** bd's stdin
(`-f -`, `--body-file -`, `-f /dev/stdin`: 8.6% of writes in the census);
`--metadata @file` (0 of 2025 — reading it would add an option for a shape
nobody types); `bd edit`'s editor, like the commit editor (ADR 0050 D5); `bd
sync` and `bd import`, whose payload is the jsonl and not argv; hand edits;
the operator's own unshimmed shell; posse's own bd calls, which run on
posse's PATH and already warn (ADR 0050 D4); and the cooperative escapes ADR
0025 names. Every one of them lands in the db and is REPORTED at the next
commit with the record's id and the freeze-and-succeed remedy (ADR 0068 D1,
D2). That report line is also the measurement that says when one of these
residuals is worth a third mechanism; none is filed here.

## Consequences

- The bd shim grows by the rendered matcher plus ~30 lines on an instance
  with a ceiling, and by nothing elsewhere. Per bd call on such an instance:
  15–24 ms MEASURED on the prototype, two classes; linear in classes and in
  file size, like the hook (ADR 0050 Consequences).
- The ERE is in `gates/<persona>/bin/bd`, one copy per launched persona,
  rewritten at every launch and removed by the next render — the same
  exposure class as the hook's copy in every hooked repo, and read by nothing
  that logs.
- `posse_check`'s `posse_bad` wording says "staged" nowhere today; the arm
  reuses it unchanged. The refusal's own words are the arm's.
- ADR 0050's Alternatives entry for the runtime-side gate now points here;
  its Status line carries the pointer. NOTES.md "Privacy model" and
  INSTALL.md's ceiling paragraph gain one sentence each: the write is refused
  at the shim with the retype remedy, the commit reports what got past it.
- The bd argv gate (`scripts/bd-argv-gate.py`) is untouched; it stays a verb
  fence and a close-dirty fence and learns no content.

## Alternatives rejected

- **Do nothing: the report arm already logs every landing (ADR 0068).**
  Price: zero code. Cost: no point in the pipeline refuses with a remedy the
  writer can perform — every paste lands in the db and in history, and the
  operator freezes a record after the fact, once per paste. ADR 0050 named
  the trigger for building the layer and it fired once (MEASURED, issue #1);
  the layer costs one render function and reuses the matcher that exists, so
  the measured cost of waiting for a second trigger exceeds the build.
- **Host it in `scripts/bd-argv-gate.py` (the bead's first question).**
  Claude-only; an operator copy that goes stale silently; its refusal funnel
  echoes the matched segment into the transcript; and Python `re` judges the
  shipped example value wrong without an error (Context, all MEASURED). Any
  honest version shells out to the real judge, at which point the host is
  the layer below.
- **A `posse gates ceiling-argv` subcommand the shim calls.** Buys a Go
  arg-parser and file reader with tests, a new CLI surface, a posse exec in
  every bd call (10 ms MEASURED, before the scan), and a second judge — Go
  `regexp` beside the hook's `grep -E`, which agree on nine values (ADR 0050
  D6) and are not the same engine. The rendered function has one judge and no
  new surface.
- **Buffer stdin to cover the 8.6%.** The shim would write bd's stdin to a
  temp file, scan it, and rewrite argv to point bd at the copy: a local file
  holding the paste, made by the wall; a rewrite of what bd is told; and a
  cleanup path. Three mechanisms for the residual the report arm already
  names with its id. ASSUMED not worth it until a report line shows a stdin
  form carrying a class; the census count is the baseline to compare against.
- **Scan write verbs only.** A verb list to keep in step with bd (close,
  create, update, comments add, q, defer, edit, create-form today), for a
  saving of one grep per read verb, and a hole for `bd search '<banner>'`,
  which is a paste typed into a local transcript.
- **Beside `WarnOpsContent` (the bead's other host).** That function speaks
  in posse's own process for beads posse itself files (ciwatch, verifyafter)
  and warns rather than refuses; a persona's bd call is a different process
  on a different PATH, and the harness's own calls already go through D4.
- **Honour `RHQ_VISIBILITY_OVERRIDE`.** Nothing to override: the typist owns
  the text. An override here is only a second spelling of the paste.
- **Read `--metadata @file` too.** 0 of 2025; one more option to parse for a
  shape with no writer. File it on the first report line that names it.
- **Render the arm into every persona's shim unconditionally and let an
  empty list exit fast.** One more rendered block on every instance that has no
  ceiling (this one), for a pin that says "no ceiling, no arm" to lose. The
  conditional render is what the hook already does.

## Verification (laurie's checklist; the pin takes the actions)

1. With `data_ceiling_patterns:` set to the two example values and a
   scratch db: `bd comments add <id> 'x ACME-RESTRICTED x'` → exit 1; stderr
   names `restricted-banner: 1 hit(s)` and never `ACME`; refusals.log gains
   one `data ceiling scan [bd shim]` line with no argv on it; `bd show <id>`
   has no new comment.
2. The same text in a regular file via `-f`, `--file`, `--body-file`,
   `--body-file=<path>` → refused the same way; the file is unchanged.
3. `bd comments add <id> -f /dev/stdin <<'EOF'` carrying the class → lands
   (D6 residual); the next commit REPORTS it with the id (ADR 0068 V1).
4. `printf 'ACME-RESTRICTED\n' > in; bd comments add <id> -f /dev/stdin < in`
   → the arm does not open it (MEASURED on the prototype: `/dev/` prefix).
5. Key absent → the rendered bd shim is byte-identical to the pre-ADR
   render (`TestRenderedShimRefusesAndPasses` family, gates_test.go).
6. `bd search 'ACME-RESTRICTED'` → refused; `bd show ranger-base-km9jt` →
   passes with the arm rendered.
7. 20 × `bd list --limit 1` with the arm rendered: under 1 s wall on this
   box (MEASURED 0.30–0.48 s on the prototype).
8. A codex seat and a container seat see the same refusal — unverified:
   needs a codex launch and the container engine; the mechanism (one shim
   per persona, `posse gates wrap`) is the claim, not a run.
