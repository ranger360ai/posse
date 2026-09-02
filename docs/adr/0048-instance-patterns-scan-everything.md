# ADR 0048 — Instance-defined visibility patterns scan every staged text file and added path; the pre-publication name is one of them

*Status: accepted 2026-09-02 (ranger-base-9ubk6, from ranger-base-n8shu) ·
owner: architect · extends ADR 0024 D2 · builds in ranger-base-uzgkz
(code) and ranger-base-856sv (the operator's one config line) · number: 0043–0045
stay pre-named by ADR 0040 §2 with live build beads; per 0040 §3.1 this file
takes the next number no bead has claimed.*

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
  text file and over ADDED staged paths. Config `beads_visibility_patterns:`
  (NOTES.md, "Instance-defined patterns") appends an instance's own
  vocabulary to the list — and today that list is rendered into check 0
  (the beads jsonl) and check 2, so a config pattern inherits check 2's
  markdown-only scope (`visibilityGuardBody`, gates.go).
- **Where the seven landed.** Four in `internal/rhq/queuecutoverspelling_qa_test.go`
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
pattern is scanned over the ADDED lines of every staged text file, code
included, and over the ADDED staged paths — the two arms
`identityGuardCheck` already renders — while the shipped `OpsPatterns`
stay markdown-only in check 2. ADR 0024 D2 kept check 2 off code because
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
