# ranger-base-pn99n — the ARGV half of the fixture-git census, and what it does not cover

fixturegitpin_qa_test.go's arm 1 (ranger-base-xa2e3) pins one way a bare-name
git fixture meets the seat's own L1 shim: an environment that drops or
overrides the launcher's config pin (hookskip:config-env). ranger-base-ltk6x's
incident — `make test` red at arm 1 for every gated seat for a day,
ungated CI green — was the OTHER way: cmd/posse/version_test.go built its
fixture with `git commit-tree`, which hookskip.go refuses outright
(hookskip:plumbing-commit), whatever the environment. The first pin's own
env check cannot see that class at all.

This bead adds that half as two more arms in the same file: arm 3
(`TestQANoFixtureGitChildCallsAVerbTheShimRefusesOutright`) over the live
corpus, and arm 4 (`TestQAArgvRefusalCensusCanStillSayNo`) as its control —
built the way the existing arms are, read directly off the AST rather than
retyped from hookskip.go where a var exists to read (`fgpHookSkipVerbs`
parses `hookSkipNoVerifyVerbs` and `hookSkipAmAllowed` out of
internal/posse/hookskip.go itself).

MEASURED 2026-11-12, this box, after ranger-base-ltk6x's fix: 814 test
files, 68 bound bare-name git children, 0 with a refused verb in their own
argv. The pin keeps it at zero rather than fixes anything live, same as the
bead that opened it predicted.

## Scope: two of hookskip.go's six "whatever the environment" rules

The bead's GAP section lists six refusal classes a fixture could hit with no
environment override involved:

- `hookskip:plumbing-commit` (`git commit-tree`, `git am`) — **covered**.
- `hookskip:no-verify` (`-n`/`--no-verify` on commit/am/merge/pull/rebase/push)
  — **covered**.
- `hookskip:config-write` (`git config`/`clone`/`init` writing a hook-moving
  key) — **not covered**.
- `hookskip:template` (`--template`, `GIT_TEMPLATE_DIR`) — **not covered**.
- `Bash(git push:*)` (PID deny) — **not covered**.
- `Bash(git commit unless --)` (PID deny, the unqualified form) — **not
  covered**.

The two covered classes are simple verb/flag string matches: the verb is
the first positional argument of the exec call, and the refusal is keyed on
that verb plus, for no-verify, one more literal in the same argv. Both are
exactly the shape the bead's SHAPE paragraph asked for — "read the exec
call's string-literal args for the refused verbs and flags" — and neither
needs anything beyond a literal compare.

The four left out all need the SAME key-parsing hookskip.go's own shell arms
do to judge a `-c key=value`: three distinct spellings per word
(`-c K=V`, `--config=K=V`, a short cluster's glued `-cK=V`), a
case-insensitive glob over a key list, and — for config-write — a `get`
vs. `set`/`unset`/`rename-section` distinction that reads the WHOLE argv's
shape, not one word in isolation (posse_hs_config in hookskip.go is ~30
lines of sh to do this once). Building a second implementation of that
parser in Go, to censuses a class with zero known incidents, was judged not
worth it in this pass — this is the bead's own "NOT CLAIMED: that this is
worth a whole pin" invitation, read as "build the two-thirds that are cheap
and bounded, leave the rest to whoever reads a live site through one of
them." The two PID-level denies are a different kind of control entirely —
judged on a running seat's own argv by a PATH shim, not on a fixture's
source — and a census of test *source* was never going to be the right tool
for them.

## What the new arms do NOT read, by design (same depth limits as arm 1/2)

- **No one-hop through a local binding for the argv itself** — unlike the
  binary and env halves, which both resolve one hop. Every site either arm
  has ever found spells its verb inline
  (`exec.Command(gitBin, "-C", repo, "commit-tree", tree)`); a verb carried
  through a variable or spread in with `args...` is past this depth and is
  read as compliant (`TestQAArgvRefusalCensusCanStillSayNo`'s "a verb
  carried through a variable is past this depth and not read" case pins
  this on purpose, not as an oversight).
- **The binary IS still resolved through `fgpBareGit`'s existing one hop,
  unchanged** — a fixture that converts its binary to `gitOutsideGates(t)`
  and keeps a stray `commit-tree` is still compliant here, because that
  binary no longer reaches the shim at all. The binary hop is what tells a
  live shim-reaching fixture from a scratch repo built with
  `/usr/bin/git`.
- **-n is read as `--no-verify` only on commit and am**
  (`fgpArgvShortFormVerbs`), never on merge/pull/rebase/push, where it means
  `--no-stat`/`--dry-run`. This split is NOT derived from hookskip.go — it
  lives inside `hookSkipNoVerify`'s switch statement, not in a named var —
  and is hand-matched with a comment saying so. Getting this wrong in the
  over-eager direction (flagging push/merge `-n`) would have made this
  census manufacture an over-refusal on its own subject, which is why both
  negative controls are in arm 4.
- **A duplicate, not shared, local-binding walk.** `fgpArgvLocalBindings` is
  a verbatim copy of fgpScan's binding walk, not an extraction into a shared
  helper. Four bugs in a row were found in that exact walk
  (ranger-base-l0bep, ranger-base-o57rc, ranger-base-6rr2e,
  ranger-base-tfqlr); keeping this arm's copy independent means a fifth one
  cannot move both arms' verdicts at once.

## Why no new Makefile door

internal/treepins/treewidedoor_qa_test.go's register is for a pin whose
SUBJECT is the tree itself, read through its own file (a new, standalone
test file with its own enumeration). These two arms are additions to an
EXISTING test file (fixturegitpin_qa_test.go) that already runs as part of
the ordinary `internal/treepins` package test binary — no new file, no new
enumeration, nothing for the register to miss. The bead's own SHAPE
paragraph names this as the cheaper of its two options ("a third rule
reading the same ast may be cheaper as an ARM of fixturegitpin than as its
own file with its own door"), and that is the one taken here.
