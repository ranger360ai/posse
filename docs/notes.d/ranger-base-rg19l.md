## Why the sequencer recipe scan stopped enumerating spellings (ranger-base-rg19l)

`recipeRemovalsOutside` — the QA scan that reads ADR 0059 D3 off the git
shim's own refusal text — was wrong five times in nine days. Four of the five
were the same question. This is the note for why the fifth fix is a different
shape, so the next person reading that function knows it is not a fifth
spelling and does not add a sixth.

### 1. The four before it

The pin asks one thing: every path the refusal tells a seat to `rm` is inside
that session's OWN git dir. The trap is that a linked worktree's own git dir
IS `<common>/worktrees/<name>`, so a textual prefix test on the common dir
calls the shared part private.

| bead | the spelling | what the scan got wrong |
|---|---|---|
| ranger-base-u18bo | a BARE path, no quotes | split on `'` read the odd fields; a bare path yielded none, so every token went unexamined |
| ranger-base-f6pt2 | `<own>/../../index` | compared raw, so the traversal wore `own` as a prefix and read as private |
| ranger-base-1h6d6 | `'<own>/a b/../../../index'` | ended the path at the first space, so the traversal was never seen |
| ranger-base-yplnv | `<own>/a\ b/…`, `'<own>/a'\''b/…'` | a backslash is not a quote and a spliced quote is not the end of a word |

Every one of them asks **where does this path END**. yplnv's answer was the
right move within that question — stop resolving a span you cannot read and
report it as written — and it fails LOUD: an unreadable span is printed for a
person.

### 2. The fifth is a different question

MEASURED 2026-09-28 in this worktree at HEAD d8809472, darwin 25.4.0, git
2.50.1, the two scans handed the line directly. `reported` is
`len(recipeRemovalsOutside)>0 || len(refusalSpansOutside)>0`, with
`own = <common>/worktrees/w1`:

```
line                                          reported (before)
rm -rf -- ../../index                         false   <- ESCAPES
cd '<own>' && rm -rf -- ../../index           false   <- ESCAPES
rm -rf -- sequencer/../../../index            false   <- ESCAPES
```

Each of those three resolves, from `own`, to the SHARED `<common>/index`.

The question they ask is **is this a word a path at all** — and a scan whose
spans start at a `/` never looks at one that does not have one. So this is not
a loud failure in a new place. It is silence over a line that names shared
state, which is the one direction the other four never failed in.

`refusalSpansOutside` cannot be widened to see it, and widening it would be
worse than leaving it. It is defined over occurrences of the COMMON DIR in the
refusal, prose included; a purely relative operand carries the common dir
nowhere, so there is nothing for it to find. Reading relative spans in prose
instead would red the refusal's own sentence — its two prose lines each hold a
`../../index` that resolves out of `own` and is an innocent instruction about
what a seat may NOT do. The removal scan is the only honest reader of this
class, because its line is a command and not a sentence. That is what
`rmScanOnly` marks in the table.

### 3. The fix: a closed vocabulary, not a longer list of spellings

An `rm` line of the refusal may hold

1. an option word (`-rf`, `--`),
2. a word in `sequencerRecipeWords` (today: `rm`, and nothing else), or
3. an absolute path this reader can read and resolve under `own`.

**Anything else is reported as written, unread.** That ends the sequence,
because the rule no longer enumerates spellings to catch — it enumerates the
words allowed, and a spelling nobody has thought of is refused by default
rather than missed by default. There is no sixth axis to find.

What it costs: `cd` and `&&` are not on the list, so a recipe that grows a
subshell prefix is read by a person before it ships. That is the same trade
ranger-base-yplnv took one axis over — refuse rather than resolve — and it is
cheap for the same reason. The rendered recipe has never held any of it: every
operand is `'$posse_sg/$posse_sm'`, each name a git pseudo-ref with no space,
no backslash, no `..` and no relative element (`renderSequencerAudit`,
gates.go). **Nothing a seat reads today is wrong**; what was broken is the
guard on the next change to that function, which is the whole reason ADR 0059
D3 has an arm.

A relative word is reported AS WRITTEN rather than `filepath.Clean`ed:
`sequencer/../../../index` Cleans to `../../index`, a path the line does not
name, from a base the scan does not know. Printing that would be the same
fiction yplnv refused to print for an unreadable span, and the seat reading
the refusal has to be able to find the word in it.

MEASURED after the change, same environment: the three lines above are all
reported; the recipe on main is reported by neither scan, as it must be; all
16 rows yplnv and its predecessors left behind still hold. A mutant that grows
the rendered recipe a `../../index` operand (`gates.go`, the one `echo "    rm
-rf --$posse_srm"` line) reds `TestQASequencerRecipeStaysOutOfTheCommonDir` by
execution, not only by reading.

### 4. The other three findings on this bead

They are one class and it is not this one: **an enumeration of the built-in
runtimes written as a literal, left behind when bob landed as the fourth**
(ranger-base-ymmiv). Each is now read off `builtinRuntimes`, which is the
register shape `paneModeUnmeasured` already uses — a row per built-in, red on a
new one — so a FIFTH built-in cannot escape the same way.

MEASURED 2026-09-28, each mutant applied to bob's `builtinRuntimes` entry
alone and restored after. The five in the first block were green through the
whole tree beforehand; the `SkillsCwd` one was re-measured with no `-run`
filter at all (`go test ./internal/posse ./cmd/posse ./internal/treepins`, all
ok), so it was a complete negative and not an artefact of a filter.

```
mutant                                     now reds
TurnOutcomeAdapter: <the claude reader>     bob grid (settle row + the absence), builtin turn-outcome table
SelfSandbox: true                           bob grid (sandbox row)
SkillsCwd dropped                           bob grid (skills row)
StateDirs -> ~/.config/bobshell             bob grid (state_dir, now anchored to the row label)
ProjectConfigKeys -> {"guessed"}            bob grid (project_cfg)
a create line that leaks `bob/`             TestQAEveryCreateLineNamesTheRuntime
```

Two of them were worth more than a row. `SkillsCwd` dropped renders `skills
flag — , pointed at the tree posse renders per persona` — a malformed row with
an empty flag name and a dangling comma — and points bob at a tree its CLI
never walks. `TurnOutcomeAdapter` set to the claude reader makes the settle row
read "turn outcome: READ by the claude-transcripts adapter" for a CLI whose
refusal artifact has never been captured, and `dispatch.go` would then run
claude's reader over a bob session: exactly what ADR 0013 §1's promotion rule
forbids. Both are now asserted, and the turn-outcome one twice — the grid's
"NOT READ" sentence, and the ABSENCE of "turn outcome: READ by the", which is
the negative the pin's own comment announced and nothing had asserted.

`~/.bob` was asserted as a bare substring, and the grid's sign-in interstitial
says "bob's own token store under `~/.bob`" in prose — so the assertion was
satisfied with the state_dir row pointed somewhere else entirely. The lesson
generalizes past bob: **a grid want-string whose value is a path or a single
word another row's prose also holds is anchored to its row LABEL**, not left
bare.
