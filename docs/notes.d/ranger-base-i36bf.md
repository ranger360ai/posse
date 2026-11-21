# A rendered shim is a shell script nothing ever asked to parse (ranger-base-i36bf)

2026-11-21, from a linux seat (Linux 7.2.15 x86_64, go1.27.3, 16 cores, git
2.56.0, bash 5.3.20 as /bin/sh). Found by READING a macos CI leg for
ranger-base-m8msi; no darwin box was touched here, and that is the whole
shape of this one.

`renderSeatObjectEnv` (ranger-base-5blmt, 9193d3b1) wrote its argv walk as a
`case` inside the command substitution that captures the answer:

```sh
posse_so_c=$(
  …
  case "$1" in
    -C) [ $# -ge 2 ] || exit 1; … ;;
```

bash 3.2 — darwin's `/bin/sh` — scans `$( )` for its closing paren while
tracking quote state, so the first pattern's own `)` ends the substitution and
the `;;` after it is left outside any case. MEASURED macos-latest (image
macos-26-arm64 20261117.0516.1, Darwin 25.6.0 arm64, bash 3.2.57, git 2.56.1),
run 44927635225 job 136917129851, macos arm 3,
`TestQAMirrorIsAFourthForcePushSpellingThatOnlyTheVerbRuleCloses`:

```
<tmp>/state/gates/developer/bin/git: line 403: syntax error near
unexpected token ';;'
```

**The file does not parse, so nothing in it runs.** Every `git` through a
darwin seat's gates PATH exited 2 — the PID's deny rules, the alias arm and
the sequencer audit with it. It fails CLOSED, so there was no exposure; a
darwin seat simply could not run git. Green at edfe7a79 (run 43731869157,
2026-11-15), red at 9193d3b1 and on every macos arm-3 leg between, through
44927635225 — six days, because the only reader is a CI leg and macos arm 1
had been red for its own reasons since 2026-11-11 (ranger-base-zcj0z).

## the form the fix took, of the two that were on the table

The walk is a shell FUNCTION now and the substitution calls it
(`posse_so_target`), rather than the POSIX-optional leading paren `(-C)`.

Both parse. The leading paren is not a guess: `renderHookSkipGlobalScope`
renders a `case` inside `$( )` with it (`(system"\t"*|global"\t"*)`), carrying
the comment that says why, and the same shim carried that line green on
darwin straight through edfe7a79 — which is the measurement, taken from CI
rather than from a box.

The function won on two grounds and neither is parse correctness:

- a function body is parsed at the TOP LEVEL, where this hazard does not
  exist, so an ARM ADDED LATER cannot reintroduce it. The leading paren has
  to be remembered once per arm, by a reader on a box whose shell parses
  either form. That is ranger-base-zcj0z's lesson in its own words — the two
  files that still carried the heredoc shape there "parsed by arithmetic and
  not by construction".
- it is what the shim's other five argv walks already do:
  `posse_verb_match`, `posse_hs_readverb`, `posse_hs_am`, `posse_cw_hit`,
  `posse_seq_scan`. This arm was the one that inlined it.

`exit` becomes `return` at six sites. The function still runs inside the
substitution's subshell, so a refusal still yields empty output and no env,
and the `cd` still reaches nothing but that subshell.
`TestQASeatObjectEnvScopesToTheSeatsOwnRepo` is green over the new form with
every arm it already covered: `-C`, repeated `-C`, an empty `-C`,
`--git-dir`, `--git-dir=`, `GIT_DIR=` from the env, `clone`, `init`, a
matching CDPATH, and the no-variables control.

## why no door saw it, and the door that does now

`make verify-shell-syntax` (scripts/shell-syntax.sh, ranger-base-g4z8m)
parses every TRACKED shell script with the shell its own shebang names. A
gate shim is rendered by Go at launch and is tracked nowhere, so that sweep
cannot see it — and nothing else in the tree parsed a rendered shim at all.

**Worth having, and it is `internal/posse/shimparse_qa_test.go`** (arm 1).
It renders the shims across eight deny sets — no rule, a verb rule, a
negative `unless` rule, a flag rule matched by membership, bd's write tail and
ceiling arm, a non-git non-bd command, a whole-tree deny — under two PATHs,
one where every shimmed command resolves and one where none does, and asks
two things of each shim:

1. that it parses under this box's `/bin/sh`. On darwin that IS bash 3.2, so
   there the door catches this class outright.
2. that it carries no `case` arm, inside a command substitution, whose pattern
   does not open with `(`.

**Half 2 is the half that answers on a linux box, and the mutant says so.**
With `HEAD~1`'s `renderSeatObjectEnv` planted back, on this box: `sh -n` is
GREEN over every shim (bash 5.3 parses the shape fine) and half 2 reds, 8
findings per git shim across all 16 renders, the first naming the `-C)` arm
darwin's bash named. Restored, both halves green. That asymmetry is the
reason the structural half exists: the renderers are edited from linux seats,
and a parse check on a linux box is not a report about bash 3.2.

The rule ADMITS the leading paren on purpose. It is a wall against the error,
not a vote between the two fixes.

### the scanner, and what it refuses to guess

`b32CaseArmsInSubstitution` is a scanner, not a parser: single quotes, double
quotes, backslash escapes, `#` comments, `$( )`, `( )`, `case`/`in`/`esac`/`;;`
and nothing else — every construct a shim renderer emits (no heredoc, no
backquote; the one rendered heredoc in the package is in a git hook, not a
shim). `$(( ))` reads as a `$(` with a `(` in it, which is all this rule needs
of it.

It returns findings AND a reason it gave up, and the caller fails on either,
because "could not read it" is not "it is clean" — a scanner that lost its
place has measured nothing, and a pin that read that as a pass would be green
over exactly the renderer edit that confused it. Five arms assert the third
state: an unterminated single quote, a backquote, a stray `)`, an unclosed
`$(`, a bare `esac`. Seven green arms assert it invents nothing over shapes
the renderers really emit, including the leading-paren form, a top-level case,
a comment carrying parens and an apostrophe, a quoted and an escaped paren,
arithmetic expansion, and a nested case.

## one reading of bash 3.2's scanner, taken from the error and not from a box

The line above the `-C)` arm was a comment carrying an apostrophe — `this
subshell's output is the answer` — INSIDE the same substitution. Had the 3.2
scanner tracked quote state THROUGH `#` comments, that apostrophe would have
opened a quote that swallowed the arm, and the error would have been an
unterminated one far below: the shape ranger-base-g4z8m measured for a quoted
heredoc body. It is not. The error names `;;` at the arm's own line, which is
what a scanner that skipped the comment and stopped at the pattern's `)`
produces.

So an apostrophe in a COMMENT inside `$( )` is NOT this hazard, and the door
above does not pretend to cover it. That is an inference from one error's line
and token, not a reading taken on darwin, and it is written down here because
the g4z8m lore reads wider than it is: the hazard measured there was a quoted
HEREDOC body. The heredoc half of the class stays
scripts/shell-syntax.sh's, whose corpus is tracked files, and whose header now
names this door as the rendered sibling.

## what is not closed here

- The macos arm-3 reading itself. The fix cannot be on a CI leg before it is
  committed, so `TestQAMirrorIsAFourthForcePushSpellingThatOnlyTheVerbRuleCloses`
  green on a macos arm-3 leg is the next leg's reading, and the bead's own
  done-condition says so. Everything available from a linux seat is taken
  above.
- ranger-base-blkse — the other darwin red at 9193d3b1
  (`TestQAWorktreeCommitLeavesNoStrayPackedRefsLock`), which needs the reading
  ranger-base-zcj0z's `wgListing` was written to print. Untouched here.
