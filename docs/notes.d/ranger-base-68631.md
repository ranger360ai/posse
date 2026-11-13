# Seven escapes from four closes: three walls that were narrower than their own prose, and three arms with nothing behind them (ranger-base-68631)

2026-11-12, Linux 7.2.15 x86_64, go1.27.3, 16 cores. The findings bundle from
ranger-base-28oqp, which verified four closes at once (ranger-base-q0yxp,
ranger-base-ihp7y, ranger-base-i78k9, ranger-base-pmf08). Seven rows, all
MEASURED by that verify rather than read, and every one of them the same
genre: a mechanism that was right about the case its author had in mind and
silent about the case next door.

Two lanes in one bundle because ADR 0006 §6 allows one findings bead per
verify close: rows 1, 2, 5, 6, 7 are the code lane and rows 3, 4 the devops
lane.

## What the seven had in common

Four of them (1, 5, 6, 7) are one file, `internal/posse/vulnprivate.go` —
check 4 of the commit guard, the wall behind AGENTS.md's rule that a residual
still open under a `vuln-private` bead is named by bead id and nothing else.
Three of those four are the same mistake in different clothes: **the wall was
spelled as a whitelist of bytes, and the rule it implements is about WORDS.**

The old test was

```go
const vulnPrivateDecoration = " \t.,:;()*+#`-0123456789"
…
if strings.Trim(rest, vulnPrivateDecoration) != "" { … }
```

and the comment above it already said the operative half out loud — "A LETTER
IS NEVER IN THIS SET". The set then had to guess every byte that is not a
letter, and it guessed short in both directions at once:

- `\r` was missing (finding 5), so a BARE id on a CRLF line was refused with
  a remedy that told the writer to do the thing they had just done. Base rate
  in this tree: honestly zero — `git grep -lI $'\r'` finds no tracked file
  with a CR and there is no `.gitattributes` eol policy — but the live route
  is the MESSAGE arm, over text git never normalizes.
- ten ordinary quoting shapes were missing (finding 6), each of them promised
  by the remedy sentence the refusal prints: `[id]`, `"id"`, `'id'`, `_id_`,
  `<id>`, `> id`, `| id |`, a bullet glyph, an em dash, a trailing `/`. A
  remedy that is wrong about what it allows is ADR 0068's class one step in —
  the way through does not go through.

The fix is the complement: `vulnPrivateBare` asks whether any rune of what is
left is a `unicode.IsLetter`. It cannot guess short, because descriptive text
needs words and a word needs a letter; and it is wider than the whitelist
could have been written, because it refuses a Cyrillic or Han description on
the same line that refuses an English one. The remedy sentence now states
that rule rather than enumerating bytes, and
`TestVulnPrivateHitsAcceptsEveryDecorationTheRemedyPromises` holds the two
together from both ends: each row names a noun the SENTENCE uses and a line
that noun describes, so a narrowing of the rule that leaves the sentence
alone reds, and a rewording that drops a promise reds too. That pairing is
the actual lesson of finding 6, which was not "the set is wrong" but "nothing
in the tree could say which of the two was".

Finding 7 (case sensitivity) is the same shape once more: `strings.Contains`
over the text as typed, so `- RANGER-BASE-<ID> heap overflow` was invisible.
The fold is now done once over the whole text, because nothing downstream
reads its case — letter-ness survives folding, and the hit line prints the id
as the CALLER spelled it and never the line.

## Finding 1, which was a decision and not a bug

The check was strictly PER-LINE, so this was flagged

```
- <open id> the heap overflow
```

and this was silent

```
- <open id>
  the heap overflow
```

which is how anyone writes a bullet with detail under it, and is invited by
the "Not closed here" SECTION that AGENTS.md's own ruling prescribes. The
verify handed this one over explicitly: "whoever takes this owns the trade:
widening to a paragraph has its own false positives, and saying so in the
file is an acceptable answer."

What landed is the **continuation**, not the paragraph. `vulnPrivateUnits`
groups lines into blocks with one stack pass — a bullet plus the lines
indented under it, in both directions, since a bare id indented under a line
that carries the text is the same escape read upwards — and the bareness test
judges the block. A nested bullet and the next sibling bullet each open their
own block, which is what keeps the compliant shape compliant.

The paragraph stays open **on purpose**:

```
## Not closed here

- <open id>

the heap overflow
```

is still silent. Section scope would close it, and would red
`docs/notes.d/ranger-base-lcuec.md`, whose own "Not closed here" names
ordinary beads — not `vuln-private`, so not this check's business at all — in
full sentences in the same section as the bare ids. That is a refusal with no
remedy the writer can perform over text the rule does not bind: ADR 0068's
class, bought to close a shape nobody has typed.
`TestVulnPrivateHitsLeavesTheSeparateParagraphOpen` ratifies the limit and
says why in its own failure text, so a later bead that decides otherwise reds
with the argument in front of it rather than re-deriving it.

One bound the grouping cannot fix and the text carries: the content arm reads
ADDED lines only (`git diff --cached | grep '^+'`), so a continuation already
in the file is not in the scanned text at all. That is the same bound checks
0-3 share. The grouping does discount the diff's leading `+` before measuring
indentation — without that, `+- <id>` and `+  <description>` both read as
column 0 and the whole widening would have been dead in the one arm that
reads a file.

MEASURED end-to-end through the shipped subcommand against the live store, 22
cases, before and after: the three shapes the verify measured as silent are
now rc 1, the ten decoration shapes and the CRLF bare line are now rc 0, and
the deliberate residual, the bare id, the sibling bullet and a closed bead's
id with text beside it are all still rc 0.

## Findings 2 and 3: an arm that was green against the mutant it exists to catch

Finding 2. `cliHomeExecMembers` keeps a per-member `guard` — the path whose
write grant admits the member — and it is not always the member itself:
claude's config members are admitted by the config DIRECTORY's grant,
codex's and grok's exec members by their HOME's, and each `config.toml` by
its own path. Keeping that as data per member is the one thing
ranger-base-pmf08's new shape exists to preserve, and its own close comment
says so. Only the `config.toml` guard had an arm. Three mutants, each
swapping a guard to the member itself, left the WHOLE committed filter green.

What the mutants change is a **deny over a path the operator explicitly
granted**: with `writable:` naming one member inside the config dir, the
shipped carve narrows nothing — the grant does not reach the directory its
guard names — while the mutant carves that one member read-only. Two arms
now read exactly that, each with its control so the arm cannot pass by
carving nothing at all. Re-MEASURED after: all three mutants red.

Finding 3. `scripts/reap-test-tmp.sh`'s three new class arms label a fixture
"dir a live process cwd's into", and the holder is
`( cd "$d" && exec sleep 60 )` — which is named by `/proc/<pid>/cwd` **and**
by `PWD` in `/proc/<pid>/environ`. Either ref alone keeps the dir, so the arm
pins neither. Pre-existing arm H has the same shape; this is the shape
q0yxp copied, not a regression it introduced. MEASURED, three mutants of the
ref scan: dropping the `cwd` readlink passed every arm, dropping the `environ`
read passed every arm, dropping BOTH failed H and all three class arms. So
nothing held the cwd readlink — the route the script's own header says the
scan exists for, a Go holder whose `os.Setenv` never rewrites
`/proc/<pid>/environ`.

The fix is two single-route fixtures, and `env -i` is the whole of it: it
clears the exec'd process's environment, `PWD` included, so the dir's path is
in exactly one of the refs the scan reads.

| fixture | named by | how |
| --- | --- | --- |
| `agedcwd` | `/proc/<pid>/cwd` alone | `( cd "$d" && exec env -i /abs/sleep 60 )` |
| `agedenv` | `/proc/<pid>/environ` alone | `( cd / && exec env -i "VAR=$d" /abs/sleep 60 )` |

MEASURED with a probe over `/proc/<pid>/{environ,cmdline,cwd,exe,fd/*}`: the
cwd holder's dir is named by `cwd` and nothing else, the environ holder's by
`environ` and nothing else, and the old shape's by both. `env` consumes the
assignment and then execs, so the final argv is `sleep 60` and the path is
not in `cmdline`; the binary is named by ABSOLUTE path because `env -i`
clears PATH before the exec. Re-MEASURED through the mutants: dropping the
cwd readlink now reds exactly the three cwd arms, dropping the environ read
exactly the three environ arms, dropping both reds all six plus H.

**Two things that went wrong in the measuring, both worth more than the
result.** The first mutation script reported "drop the environ read -> FAIL"
over a `sed` that matched NOTHING — the unmutated script, failing on box
noise, read as the mutant being caught. The second version asserts the
mutation applied (`cmp` against the original) and judges the NAMED arm rather
than the run's verdict. The box noise is real and is filed: the self-test's
`claude -p` arms compare a BOX-WIDE count of live claude processes before and
after starting a fake one, so a sibling seat starting or stopping a claude
between the two readings fails them. Seen twice here in unmutated runs, and
`verify-reap-test-tmp` is a prerequisite of `make test`, so it can red any
seat's suite. Filed as ranger-base-2ceq3, not fixed here — it is nobody's
regression and outside this bundle.

## Finding 4: the state was correct and protected by nothing

`mktemp -d` without a template answers `<TMPDIR>/tmp.XXXXXXXXXX`, which is
the `mktemp` row of the reaper's own class table: a **24-hour** grace,
because that row is every foreign tool's scratch dir on the box and the
reaper cannot tell one of ours from one of theirs. A templated dir lands in
its script's own class and is collected on that class's grace instead — 120
minutes for `posse-scratch`, 60 for `posse-testhome`. So the template is what
makes a script's garbage collectable at the rate its own runs justify, on the
inode quota the reaper exists to protect.

ranger-base-i78k9 templated nine bare call sites, and its FIRST pass — a hand
grep — missed three of them (`queue-scrub.sh:51`, `reap-test-tmp.sh:311` and
`:481`; its own close says so). A rule kept by hand went three short on the
day it was being applied deliberately by someone who had just read it. That
is the argument for a census and not a habit, and it is the whole of what
this row is: MEASURED at 9d8e97a9, `make scripts-check` green over four pins
with none on this subject, `make corpus-check` green with none either, and
`TestReleaseScriptsUsePortableMktemp` naming two scripts by hand for a
different question (BSD `-t` portability). Zero bare calls in the tree and
nothing holding it there.

### CLOSED ON MAIN BY SOMEBODY ELSE, SO THIS ROW SHIPS NOTHING

The census this bead wrote — `TestQAEveryScriptsMktempDirNamesATemplate` in
`internal/treepins/mktemptemplate_qa_test.go`, behind `make scripts-check`,
MEASURED 2026-11-12 over 62 runnable files and 27 `-d` call sites, all 27
templated — **is not in this tree and never landed**. It was written at
ea72e88a on 2026-11-12 at 20:56 and the merge-back was refused; by the time
ranger-base-rl4p2 replayed it, three commits on main had closed the same
question in a stronger form:

- `TestQAEveryMktempDirRootUnderScriptsIsReapedOrRostered` (ranger-base-w8n6w,
  2f23e794) — a live `mktemp -d` under scripts/ names the house template
  `${TMPDIR:-/tmp}/posse-<what>.XXXXXX` or is on `mtExempt` with its reason.
  That is **strictly stronger than "names a template"**: it asserts the
  namespace the reaper's `posse-*` glob reaches and the root it looks in, so
  every call site this bead's census would pass it also passes, and all three
  exempt rows carry a template anyway. Its reader is loose in the safe
  direction (every runnable line with a `-d`, trailing comments included) and
  classifies heredoc bodies rather than dropping them, so its file set is a
  superset of this one's command-position set as well.
- `TestQAEveryMktempFileInTheShippedShellIsRosteredOrExempt` (ranger-base-u1z89,
  da353084) — the arm for the hole this bead's census explicitly scoped out,
  a `mktemp` with no `-d`, which turned out to be worse than scoped: every
  candidate walk in the reaper is `-type d`, so a leaked scratch file is
  outside all nine class rows whatever it is named.
- `TestQAEveryTempRootInTheTreeIsInPossesTempNamespace` (ranger-base-qbg7c,
  9258e0df) — the same rule in Go and Python, behind `make corpus-check`.

Keeping this bead's census beside those would be two line-based readers over
one set of call sites, the weaker one adding only a second implementation to
keep in sync — the shape AGENTS.md names for doors ("a door that goes
narrower than the pin while both look green") and the same mistake one level
up. So the replay **drops the pin, its control arm, its Makefile row and its
`treewidedoor_qa_test.go` register paragraph**, and the Makefile and
`treewidedoor_qa_test.go` come out byte-identical to main.

What survives of finding 4 is the diagnosis above, which is not redundant:
`mktemproot_qa_test.go`'s own head comment argues the census from
ranger-base-m0j9n, -i78k9 and -tbukc renaming roots and leaving nothing
behind, and the three-missed-of-nine hand grep recorded here is the same
family's fourth datum. The reader who wants the pin wants that file.

## Not closed here

- ranger-base-2ceq3 — the box-wide `claude_procs` comparison above. Filed
  rather than fixed: it is its own lane and nobody's regression, and the
  seven rows of this bundle were the deliverable.
