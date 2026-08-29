## The L1 commit wall named the option in full, and git accepts prefixes (ranger-base-l1at)

rangerhq-ojnw taught the wall that `git commit -i -m x -- b.txt` carries the
qualifier and sweeps the shared index anyway, and it refused `-i`,
`--include` and `-im` by name. laurie found the same defect one spelling to
the left while verifying that close: **git's parse-options accepts any
unambiguous PREFIX of a long option**, so `--inc` *is* `--include`, it is
not the literal the arm spelled, and the `--*) ;;` arm — the one that exists
so `--signoff` does not match the `-i` cluster pattern — swallowed it on the
way past.

**Measured (git 2.50.1, macOS 25.4), another persona's `a.txt` staged, mine
`b.txt` on disk, no L3 hook:**

| form | git | shim before | lands |
|---|---|---|---|
| `--i` | `ambiguous option: i (--include or --interactive)`, exit 129 | through | nothing |
| `--in` | ambiguous, exit 129 | through | nothing |
| `--inc` | resolves to `--include` | **through** | `a.txt b.txt` — **sweeps** |
| `--incl` `--inclu` `--includ` | resolves to `--include` | **through** | `a.txt b.txt` — **sweeps** |
| `--include` | resolves | refused | nothing |
| `--inc=yes`, `--include=yes` | ``option `include' takes no value``, exit 129 | through | nothing |

So the hole is exactly the four abbreviations between git's ambiguity
boundary and the full spelling. The `=value` forms pass the shim and always
will; git rejects them itself before anything is committed, so there is
nothing there to guard and no reason for the next reader to re-chase it.

**The blast radius stops at the subcommand's own options.** git's *global*
options are parsed by git.c, not parse-options, and they do **not**
abbreviate — `git --git-di=.git rev-parse` answers `unknown option:
--git-di` (measured). `posse_verb_match`'s global-option skip list
(`globalValueOpts`) is therefore not exposed to this class, and neither is
the qualifier scan.

**The fix is one more measurement in the table, not a wider glob.** A
`spoiler` now carries `LongMin` — the shortest abbreviation git resolves to
each long option — and `renderSpoiled` emits every prefix from there to the
full spelling as one alternation:

    --) return 1 ;;
    --inc|--incl|--inclu|--includ|--include) return 0 ;;
    --*) ;;
    -*i*) return 0 ;;

Down-to-one-character was the tempting version and it is wrong. A future
long spoiler like `--pathspec-from-file` would emit `--pat`, which real git
resolves to `--patch` — a legitimate form the wall would refuse. The
boundary has to be measured per option, and jian-yang's three sh details
from ojnw are unchanged: the arms stay baked in, long options stay matched
before the cluster pattern, and the scan still stops at `--`.

**A missing `LongMin` is the hole reopening silently**, so the pins do not
trust the table. Three, in `gates_qa_test.go`:

- `TestQACommitWallL1IncludeAbbreviations` is laurie's pin, landed skipped
  by ranger-base-wst3 so the fix would have something to unskip, and
  unskipped here. Its half one is the premise and always ran: real git
  accepts each abbreviation *and* the form sweeps the shared index, so the
  guard arm can never end up pinning a ghost. Two things were added to it:
  the hardcoded prefix list is now checked against `qaGitResolves` — which
  asks the parser rather than a list anyone wrote from memory, so a git
  whose ambiguity boundary moved cannot quietly shrink the pin — and the
  way-through list gained `--am` (`--amend`) and `--sign` (`--signoff`),
  the over-match this fix could have caused.
- `TestQASpoilerLongMinIsGitsBoundary` requires every long spoiler to carry
  a `LongMin` and requires it to be git's own boundary — git resolves it,
  one character shorter it does not. A stale number is a hole if too long
  and noise if too short; both fail here.
- `TestQACommitWallL1AbbreviationDoesNotSweepRealIndex` is the bead's repro
  end to end, the rendered shim in front of the *real* git with no L3 hook:
  refused, the other persona's staged entry still staged, HEAD unmoved, and
  the safe form still landing exactly one path. Half two of laurie's pin
  runs against a stub git and settles the decision; this settles what the
  decision is worth. Its first half is the unguarded premise, for the same
  reason hers has one.

Each was run against the unfixed render and each failed on the four
abbreviations; the boundary pin was also run against a deliberately short
`LongMin` (`--in`) and failed on that.

**L3 was never exposed and still is not.** The hook discriminates on
`GIT_INDEX_FILE` / `.git/index.lock`, which git hands it whatever the option
is spelled, so `--inc` was refused there throughout (laurie measured). The
exposure was bounded to hookless checkouts — which is the case L1 exists
for, and it was live in every persona session.
