# ADR 0048 — Instance-defined visibility patterns scan every staged file and added path; the pre-publication name is one of them

*Status: superseded 2026-09-05 by ADR 0024 · ADR simplification, operator ruling 2026-09-05.*

The surviving decision is in [0024 — current contract](0024-work-product-routing.md), Scan coverage, Decision 2 and Lineage. This page keeps its number and dated evidence; the body below is historical, not current policy.

## Historical record (superseded in full)

*Status: accepted 2026-09-02 (ranger-base-9ubk6, from ranger-base-n8shu) ·
owner: architect · extends ADR 0024 D2 · builds in ranger-base-uzgkz
(code) and ranger-base-856sv (the operator's one config line) · number: 0043–0045
stay pre-named by ADR 0040 §2 with live build beads; per 0040 §3.1 this file
takes the next number no bead has claimed · amended 2026-09-04 (title, Context,
D2, Consequences, Alternatives: the scope is every staged FILE, not every
staged "text" file — "text" was the reader's mechanism written down as the
rule, and git's text/binary call is a guess on the file's own bytes that one
NUL flips; ranger-base-9307c, from ranger-base-h137b) · amended 2026-09-03
(D2: the commit MESSAGE is a third arm, with check 3's literals — product
decision that date on ranger-base-1nbtn, landed 2026-09-04 in
ranger-base-qk8i9, which is why this line sits after the 09-04 one above).*

> The seed surface must carry zero bare occurrences of the harness's
> pre-publication name (rangerhq-7xpn AC7; the marker form `<name>-<id>`
> is legitimate and stays). The only reader of that invariant is a Go test,
> `TestSeedSurfaceNameCountIsZero`, so it fires after the commit is on
> public main. Seven recurrences (ranger-base-zn30, vqu8, 3ova, pi19, poi,
> dk78w, n8shu) were each closed by rewording the offending line; two
> rewrote the same paragraph twice. The fix is not an eighth rewording.

## Context (MEASURED 2026-09-02 unless marked)

- **The wall already has the slot.** ADR 0024 D2's `prepare-commit-msg`
  hook refuses at the typed line, in every persona worktree (shared hooks
  dir), before anything reaches main. Check 2 scans the shipped
  `OpsPatterns` over ADDED lines of staged **markdown only**; check 3 scans
  this box's derived identity literals over ADDED lines of **every** staged
  file and over ADDED staged paths. Config `beads_visibility_patterns:`
  (NOTES.md, "Instance-defined patterns") appends an instance's own
  vocabulary to the list — and today that list is rendered into check 0
  (the beads jsonl) and check 2, so a config pattern inherits check 2's
  markdown-only scope (`visibilityGuardBody`, gates.go).
- **Where the seven landed.** Four in `internal/posse/queuecutoverspelling_qa_test.go`
  prose, one across `scorecard.go` and `scorecard_test.go`, two in
  `NOTES.md`, one in ADR 0046 line 88. Five of seven are Go files. A
  config pattern under today's scope would have caught two.
- **The bead's candidate 2 is false on this box.** "Derive the bare name as
  the basename of the instance repo path check 3 already derives": the
  `.beads/redirect` target's parent is the queue repo, whose basename is
  not the pre-publication name. There is no path on the box whose basename
  is that name and whose bare use is illegitimate.
- **Bare names are not one class.** Over tracked content, the bare forms of
  the other instance repo names appear legitimately: the queue repo's name
  10 times, the constitution repo's 72 times, the pre-publication name once
  (the n8shu red). Any derivation from `beads_visibility:` keys would
  refuse legitimate prose.
- **The pin is cheap but wrongly placed for prevention:** 0.05 s of test
  time, 0.43 s with the package build. It walks the working tree, not the
  staged set, and it runs when someone runs the package — after the commit.
- **The pattern that names the class.** The marker form is the exact
  exception, so the class is an ERE, not a fixed string:
  `<name>([^-]|-[^0-9a-z]|-?$)`. Probed under `grep -E` and Go `regexp`
  over nine spellings (bare, marker, marker then punctuation, trailing
  slash, lone hyphen, hyphen then non-id, inside parentheses, prefixed,
  mixed case): identical verdicts, and the same verdicts the pin gives.
  Over `git ls-files`, 0 path hits; over tracked content, 1 (the known red).

## Decision

**D1 — the guarantee lives in the operator's config, as an instance
pattern.** One entry under `beads_visibility_patterns:`, class
`pre-publication-name`, value the ERE above with the name substituted. The
name is this instance's history, not the harness's: a second deployer's
pre-publication name is whatever *its* seed rename scrubbed, and only it
knows that. Config is the carrier NOTES.md already reserves for exactly
"a name that is confidential HERE" — the vocabulary never enters the
public tree, which is the whole point of 7xpn. The value is instance
vocabulary and a `PromotedPath` edit, so the operator writes it
(ranger-base-856sv); a persona cannot.

**D2 — instance patterns get check 3's scope, not check 2's.** A config
pattern is scanned over the ADDED lines of every staged file, code
included, over the ADDED staged paths, and over every line of the commit
MESSAGE — the three arms `identityGuardCheck` renders (the message is the
third since ranger-base-1nbtn, ADR 0024 D2 check 3 as amended 2026-09-03;
built with check 3's own in ranger-base-qk8i9) — while the shipped
`OpsPatterns` stay markdown-only in check 2, and are not scanned over the
message at all: that is a census, not an oversight, and ADR 0024's
Consequences carries it. *Every staged file, not every staged "text"
file (amended 2026-09-04, ranger-base-9307c, from ranger-base-h137b).* The
reader is `git diff --cached -U0 --text`, and the scope IS that reader's
output: the ADDED lines of whatever is staged, bytes as they are. "Text"
was never a rule — it was the reader's silent classification written down
as one. Git calls a file binary on its own bytes (one NUL is enough), and a
binary file yields no `+` lines at all, so under the old wording a markdown
file with one NUL in it committed its ops prose into the public tree with
no refusal and no "judged nothing" line (MEASURED, h137b). The wall cannot
tell that file from a real blob without guessing, and that guess was the
hole, so it no longer guesses: a real blob whose BYTES carry a class is
refused like any other file (MEASURED and pinned, h137b: a blob carrying the
class is refused; the same blob without it commits). That is the rule's
own reading, not a widening — an identity literal in a PNG's metadata or
an instance name inside a tarball has no legitimate public use either, and
is the leak nobody reads. The way through for a genuine asset is the
existing override, typed by the operator; there is no allowlist of "real"
binaries, by path, extension, attribute or heuristic, because each of those
is a hole list the writer controls (a `.gitattributes` line saying `-diff`
reached the same silence, MEASURED, h137b). Check 2 keeps its markdown
pathspec and gains nothing but the same `--text`: a NUL in a `.md` file no
longer exempts it. ADR 0024 D2 kept check 2 off code because
the shipped list's *own source and tests* are byte-identical to hits, and a
wall carrying an allowlist of its own files is a wall with a hole list.
That argument is about the shipped list. A config pattern is never in
source: it lives in the operator's config and the rendered hook, both
untracked. So it shares check 3's property — no legitimate public use
anywhere — and belongs under check 3's scope. Same `posse_check` function,
same override env, same refusals.log shape, public-stamped repos only; the
refusal header names the class, never the value. Builds in
ranger-base-uzgkz.

**D3 — the pin stays, as the post-landing backstop, and names the arm.**
`TestSeedSurfaceNameCountIsZero` keeps walking the tree: it is the only
guard on a box whose config lacks the line, on a commit made under the
typed override, and on a re-render nobody ran. Its failure text gains one
sentence naming the commit-time arm (config key and this ADR), so the
eighth red, if there is one, is read as "the wall is not stamped here"
rather than as a reword ticket.

**D4 — docs-only commits are in scope, and already were.** The bead asked
whether a wall "whose refusals are otherwise bead-shaped" should refuse a
docs-only commit. Check 3 already diffs everything staged; checks 1 and 2
are docs-shaped by construction. Nothing new is decided here; the answer
is what the wall does today.

## Consequences

- The class becomes a refusal at the typed line, in every worktree, once
  the operator's line is in and the hooks re-rendered (`posse gates
  install-hooks`; the launcher's L3 probe reads a changed render as "ours
  but stale" until then — expected twice: once for the config line, once
  after uzgkz's binary is installed).
- Until uzgkz lands, the config line alone covers markdown and the beads
  jsonl: two of the seven historical shapes. That is why the operator's
  bead does not wait on the code bead.
- A loosely written instance ERE now scans code too. The operator owns the
  ERE; the override exists; for this instance's one pattern the measured
  false-positive count over code and paths is 0 today. ASSUMED: it stays
  near 0, because the bare name has no legitimate use the pin would not
  already have refused.
- *(2026-09-04)* Every ERE and every identity literal now scans a blob's
  bytes too. MEASURED: this repo tracks zero files git classifies binary
  (`git diff --numstat <empty tree> HEAD` has no `-` rows), so the
  false-positive count over the tracked tree is 0 by construction. ASSUMED:
  a future genuine asset that trips a class is a leak, not a false positive
  — the classes are defined as "no legitimate public use anywhere", and
  bytes are somewhere. The hook's own head comment and check 3's rendered
  prose say "text file" today; the sweep re-renders the hook, so the L3
  probe reads every hooked repo as "ours but stale" until `posse gates
  install-hooks` runs — expected once, as ADR 0050 already paid.
- ADR 0024's "Residuals" bullet ("non-markdown prose is unscanned by
  check 2") gains one clause: config patterns are scanned everywhere since
  this ADR. NOTES.md's "Instance-defined patterns" gains one sentence on
  scope. Both in uzgkz.

## Alternatives rejected

- **Derive it like the identity literals (the bead's candidate 2, and the
  clever one).** The one derivation that yields exactly this name on this
  box is "bead-id prefixes present in the instance db minus the public
  repo's own prefix" (MEASURED: 412 marker-prefixed rows against 1219
  own-prefixed). Rejected: it couples the wall to bd's storage format and
  needs a second derivation for "which prefix is ours"; and the thing found
  is not identity but history, so the derivation is a guess dressed as a
  rule. Check 3's literals are also fixed strings by type
  (`identityLiteralERE` escapes every metacharacter); a name-with-exception
  is an ERE, which is the instance-pattern type, not the literal type.
- **Derive from `beads_visibility:` basenames.** Refuses the queue repo's
  and the constitution repo's bare names, which are on the surface
  legitimately (10 and 72 hits).
- **Ship it as an `OpsPattern`.** The public list would then carry the
  name 7xpn removed; the pin would count the pattern's own source. Wrong
  place, as the bead said.
- **Run the Go test from the hook.** Walks the working tree, not the
  stage; puts a toolchain in the commit path; and the wall already has a
  matcher whose subject is the staged diff, which is the right subject.
- **Widen shipped check 2 to code.** Rejected by ADR 0024 D2's own
  argument, which still holds for the shipped list.
- **An eighth reword.** The seven closes are the measurement.
- **Keep "text" and carve real binaries out (2026-09-04, priced three
  ways).** *By git's own heuristic* — that is the wording this amendment
  retires: the heuristic flips on one NUL, and a NUL is what pasted
  terminal output carries. *By `.gitattributes` / `binary` or `-diff`* —
  writer-controlled, and the writer's `core.attributesFile` was one of the
  two measured ways to blank the reader; an exemption the writer can set
  is not an exemption, it is the override without the log line. *By
  extension allowlist* — ADR 0024 D2's hole-list argument verbatim: a file
  named `x.png` with NOTES prose in it commits clean, and the list needs a
  reviewed code change per asset type in a repo with no assets. All three
  buy a false-negative to avoid a false-positive the override already
  handles at the cost of one typed line.
- **Scan a blob whole rather than its added "lines" (the clever one).** A
  blob has no lines, so scanning its ADDED lines under `--text` is
  scanning newline-delimited chunks of it; a whole-blob arm would be
  "more honest". Rejected: the added-line rule already covers every byte a
  new blob brings and every changed chunk of a modified one; a modified
  path's untouched bytes cleared the wall the day they were added, which
  is check 1's and check 3's path rule verbatim. A second reader shape for
  one file class is a third attempt at one invariant.

## Operator steps (ranger-base-856sv)

1. In the constitution source's `config.yaml` (byte-identical to the live
   file today), add the `beads_visibility_patterns:` map with the one entry
   from D1. Dialect: one line, no single quote, no `\d \s \b \w \t`; a
   trailing ` #` starts a comment.
2. Promote, so the live config carries it.
3. In the public repo, `posse gates install-hooks`. Read the output: a
   refused entry is named by class and is NOT in force.
4. Prove it: in a scratch worktree, stage a `.md` file with the bare name
   on an added line and commit; expect the check 2 refusal. After uzgkz is
   installed, re-run step 3 and prove again with a `.go` file; expect the
   check 3 refusal.
