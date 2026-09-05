## The L1 commit wall's qualifier could be satisfied by a lie (rangerhq-ojnw)

`Bash(git commit unless --)` asked one question — does argv carry `--` with
at least one operand? — and treated the answer as "this commit is
path-limited". It is a **proxy**, and laurie found the proxy's false
negative while verifying rangerhq-40ig: `git commit -i -m x -- b.txt`
carries the operand and commits the shared index **on top of** the named
paths. That is rangerhq-nyqj's failure with a pathspec attached, and L1 —
the layer that travels with the session into a repo that has no hook, which
is the whole reason there are two layers — waved it through.

**Measured first (git 2.39.3, macOS 25.4), because the shape of the fix
depends on it.** Another persona's `a.txt` staged, mine `b.txt` on disk:

| form | lands | verdict |
|---|---|---|
| `git commit -i -m x -- b.txt` | `a.txt b.txt` | **sweeps** |
| `git commit --include -m x -- b.txt` | `a.txt b.txt` | **sweeps** |
| `git commit -im x -- b.txt` | `a.txt b.txt` | **sweeps** — bundled is a fifth spelling |
| `git commit -mi -- b.txt` | `b.txt`, message `i` | safe (the `-m` value, not `-i`) |
| `git commit -a -- b.txt` | nothing, exit 128 | git itself: `paths … with -a does not make sense` |
| `git commit -o -m x -- b.txt` | `b.txt` | safe (`--only` is the default) |
| `git commit --amend --no-edit -- b.txt` | HEAD's tree | not a sweep; out of scope |

So `-a` still needs no case of its own, and `-i` needs three: long, short,
and short-inside-a-cluster.

**The fix is a table, not a special case.** `qualifierSpoilers` (keyed
`"<cmd> <subcommand>"`, the way the rule is written) names the options that
satisfy a qualifier and do the unsafe thing anyway, with the sentence that
explains why. `renderShim` emits one `posse_spoiled_<verb>()` per rule that
has an entry, and the rule's condition becomes *refuse when the qualifier is
missing **or** when it is there and lying*. The refusal's hint is derived
from the same table, so it now reads `safe form: git commit … -- <operand>
[<operand>…], and without -i/--include — it commits the shared index ON TOP
of the named paths`.

Three details the sh has to get right, each of which was a way to be wrong:

- **The scan stops at `--`.** Past the qualifier every word is a path, and a
  file named `-i` is a file.
- **Long options are matched before the cluster pattern.** `-*i*` matches
  `--signoff` and `--fixup=HEAD` too; the `--*) ;;` arm between them is what
  keeps `git commit --signoff -m x -- a.go` working.
- **The case arms are baked in, not passed in a variable.** A glob pattern
  reaching `case` through an unquoted expansion is a *pathname* expansion
  first, and `-[!-]*i*` would happily match a file in the caller's cwd.

The long-option arm was still one spelling too narrow: git accepts
unambiguous *prefixes*, so `--inc` walked past it into the `--*) ;;` arm
until ranger-base-l1at — see `docs/notes.d/ranger-base-l1at.md`.

**One false positive accepted, on the bead's own terms.** `git commit -mi --
b.txt` (message `i`) is now refused, as is any value that happens to be
spelled `-…i…`. (**Closed since, by ranger-base-v3cu** — the scan pairs the
value-taking options now, so `-mi` and `-m '-i am a message'` go through;
what is left of the class is a value option behind a boolean in the same
cluster, `-qmi`. See `docs/notes.d/ranger-base-v3cu.md`.) The bead states the trade and it is the right one: a false
positive has a way through and the refusal prints it; a false negative is
the wall not being there. The proxy's *other* false positive is untouched —
`git commit -m x --pathspec-from-file=list` IS path-limited (measured:
next-index, the other persona's staged entry survives, L3 lets it through)
and L1 refuses it. Same reason: cheap, and it has a way through.

**L0 was deliberately left alone.** Claude's dialect has no negation, so the
widening is already the two exact shapes that are unsafe whatever follows.
The spellings that would catch `-i` there (`Bash(git commit * -i *)`) match
the *whole command string* with quotes intact, so `git commit -m "fix -i
thing" -- a.go` would be refused — the exact false positive `Bash(git -c …
commit -m "push it")` is already documented as too expensive at a layer
whose job is politeness. L1 is the wall.

**And L3 stopped misnaming the form.** The hook has no argv — it
discriminates on `GIT_INDEX_FILE`, and `-i` gets `.git/index.lock`, the same
arm as `-a`. It refused correctly and said `refused by posse gate: git
commit -a`, sending the reader after a flag that is not on their line. It
now says `git commit -a or -i`, and the body names what each one takes.

Pinned: `TestQACommitWallL1IncludeForm` (unskipped, plus the bundled and
`git -C` spellings, and a pass-list for `--signoff` / `--fixup=HEAD` / a path
named `-i`), `TestQACommitWallIncludeFormSweepsAndIsRefused` (premise arm
switched to the bundled spelling; guard arm now asserts the form name), and
`TestShimNegativeMatchUnless` for the grammar itself. Both L1 pins verified
red with the table entry removed.
