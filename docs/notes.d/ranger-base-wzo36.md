# A trap reads what the shell that owns it assigned, and nothing else (ranger-base-wzo36)

2026-11-13 · gilfoyle · linux 7.2.15 x86_64, go1.27.3, this box

ranger-base-m3fl5 fixed a leak: `scripts/verify-bd-dep-safety.sh` and
`scripts/prune-bd-relates-to.sh` picked their read-only beads reader lazily
inside a `q` helper, every caller of `q` was a `$( … )`, and so the scratch
path and the memo were assigned in a child and died with it — one leaked copy
of a 69M db per QUERY, not per run. The fix hoisted the pick into the shell
that owns the EXIT trap.

ranger-base-ltfq7's verify then showed the fix could be undone in silence.
MEASURED 2026-11-12 under ranger-base-ltfq7, tree 9aaef0d4: with the pick moved
back into `q` — the shape at `197ea0b8^` — `make verify-shell-syntax`, `make
verify-bd-dep-safety`, `make verify-bd-no-relate-pairs`, `make
prune-bd-relates-to` and `make tree-check` were ALL green while the fallback
leaked again:

| form | picks | leftovers |
|---|---|---|
| verify-bd-dep-safety (3 queries) | 3 -> 1 | 3 -> 0 |
| verify-bd-dep-safety --gate | 1 -> 1 | 1 -> 0 |
| verify-bd-dep-safety `<target>` | 1 -> 1 | 1 -> 0 |
| prune-bd-relates-to (dry run) | 1 -> 1 | 1 -> 0 |
| prune-bd-relates-to --apply | 2 -> 2 | 2 -> 0 |

Both scripts state the rule in prose — `prune-bd-relates-to.sh:161`, "THE PICK
HAPPENS IN THIS SHELL, NEVER INSIDE A `$(…)`" — and a sentence in a file is
not a gate.

## The pin

`internal/treepins/trapstatepick_qa_test.go`, seventh arm of `make
scripts-check`, named by `QA_SCRIPTS_PINS` (Makefile:742).

**The rule, for the class and not for those two scripts.** In a script under
`scripts/` that registers an EXIT trap, every assignment to a variable that
trap READS happens in the shell that registered it. A function that makes such
an assignment is a trap-state setter, and a setter must be reachable only from
that shell — never from inside a `$( … )`, a backtick, a `( … )`, a pipeline
component or a background job, and never from another function that is itself
only reached that way.

**The rule is transitive because the defect was.** At `197ea0b8^`
`pick_reader` was called at the top of `q`'s body, at no depth at all; it was
`q` that only ever ran inside a `$( … )`. A rule reading a setter's own call
sites calls that shape clean, which is exactly what five doors did.

It is not "a trap that removes a scratch copy", though that is the member that
cost 206M of a shared tmpfs. Any variable the handler reads is the same bug:
the handler is reading what nothing in the process that owns it ever assigned.

## What the census found

MEASURED 2026-11-13, this box: 64 files under `scripts/`, 48 of them shell, 25
of those registering an EXIT trap, 46 handler-read variables between them, 426
shell functions (55 of them nested inside another) and **16 trap-state
setters**. Two of the sixteen are reached in a subshell, and they are the two
halves of the rule:

- `verify-herdr-bob-rules.sh` `watch_once` — tainted by its OWN call site,
  `got="$(watch_once "$T/seed" seed)"`.
- `verify-herdr-bob-rules.sh` `drop_pid` — tainted only through `watch_once`,
  which is the `197ea0b8^` shape, and the only member outside the control
  fixtures that keeps the transitive arm live over the real tree.

Both are on `tsExempt` with their reasons and with **ranger-base-myjcz**, the
bug filed for the window they leave open. They are not fixed here because
`scripts/verify-herdr-bob-rules.sh` SKIPS on this box (herdr-bob is not
installed, and the script pins herdr-bob's own HEAD at `:24`, so the rig
cannot be faked up) — a fix nothing here can execute is a fix nobody has run.

(Both rows were deleted the same day: ranger-base-myjcz hoisted `watch_once`
out of that command substitution and `tsExempt` is empty again. The mutation
table below is the record of what was run here and stays as it is; the row it
renames no longer exists to rename. The transitive arm now has no member in
the real tree and is carried by the planted fixtures alone.)

The four setters in the two scripts the rule was written for (`pick_reader`
and `cleanup_ro` in each) are all clean, and no script has a top-level
assignment to trap state inside a subshell.

## Mutation-checked against the REAL scripts, not only fixtures

Each mutation restored the committed bytes afterwards; `git diff
--no-ext-diff HEAD` over both scripts was empty at the end.

| mutant | want | got |
|---|---|---|
| `scripts/verify-bd-dep-safety.sh` at `197ea0b8^` | red | RED, naming `pick_reader <- q` and `q`'s call at `:146` |
| `scripts/prune-bd-relates-to.sh` at `197ea0b8^` | red | RED, same chain |
| `tsExempt`'s `watch_once` row renamed | red both ways | RED unrostered **and** RED stale |
| both scripts as committed | green | GREEN |

The diagnostic for the first:

```
scripts/verify-bd-dep-safety.sh:106: pick_reader() assigns RO_TMPS, which this
script's EXIT trap reads — and it is reached in a subshell:
  scripts/verify-bd-dep-safety.sh:120: [ -n "$DB_READ" ] || pick_reader
  reached only through q, whose own call at scripts/verify-bd-dep-safety.sh:146
  is in a subshell — the chain is pick_reader <- q
```

## The reader, and the three bugs found by pointing it at the tree

It is a lexer, not a parse, and it shares the package's one shell state
machine: `sfWalk` (selftestforkarm_qa_test.go) gained a third return — the
open frame count — rather than a second walk being written, so a repair to the
nesting reaches both readers. Heredoc openers are `bgHeredoc`'s and the
quoted-opener probe is `sfQuoted`'s, the way `mktemproot_qa_test.go` borrows
them.

Three things the first draft got wrong, all found by running it over
`scripts/` and all MEASURED 2026-11-13:

1. **A comment is not code, and an apostrophe in one ran the lexer away.**
   `verify-pid-deny-set.sh`'s "a pattern kill matches every seat's identical
   argv" opened a single quote nothing closed; the join ate the next 60 lines
   and the file read as unlexable. `verify-herdr-bob-rules.sh` ("the dialog's
   top rule") and `verify-detection.sh` did the same — three files of the
   forty-eight. Comments are now cut before the scan, at the first unquoted
   `#` that starts a word (`${x#y}`, `${#arr[@]}` and `$#` are not comments).

2. **A parameter expansion carries its own quoting.**
   `printf '%s' "${rest%'"'*}"` (verify-detection.sh:182) is a valid trim whose
   pattern is a quoted `"`. To a lexer that knows only strings, that inner `"`
   closes the enclosing one and everything after is a string — 54 lines
   swallowed. `tsMaskParamExp` now neutralises the inside of every `${ … }`,
   keeping the braces, the length and every newline, so offsets still map to
   lines; the RAW text is kept beside it because `${WORK:?}` is exactly where a
   handler names what it removes. Masked here and not taught to `sfWalk`,
   whose answers serve three readers whose subject IS what is inside a quoted
   string.

3. **An environment prefix is not an assignment.** `GIT_COMMITTER_DATE=… git
   commit …` puts a name in the CHILD's environment and never in this shell's
   variables, so a trap here could not read it whatever it is named. Counting
   one red eight real lines doing exactly the right thing
   (verify-herdr-bob.sh, verify-hook-freshness.sh, verify-id-recycle.sh,
   verify-prune-guard.sh, release-artifacts.sh, queue-cutover.sh).

Two more found by reading the reader rather than by running it:

4. **A nested definition is a function.** The extent scan skipped to the end
   of a `{ … }` body, so a definition inside one was never registered: 55
   indented definitions in 8 files, `gotest.sh`'s twelve among them. Finding
   them took the census from 389 functions to 426 and from 14 setters to 16 —
   and the two it gained are `reap-test-tmp.sh`'s `start_fake` and
   `stop_fake`, nested in its `self_test` and both assigning the pid list its
   trap kills. Both clean, and both exactly the class.

5. **A pipe is somebody's, not the line's.** The first reading asked "is there
   an unquoted `|` before this command", which makes `ls | grep -q x && set_it
   x` a pipeline component that it is not. It reads the separator IMMEDIATELY
   on either side now, with `||` and `&&` read before `|` and `&`, and both
   shapes have a control row.

Loose in the direction that costs a false RED and never a false pass, which is
`brewguard_qa_test.go`'s rule one file over: a handler's own `local` names are
subtracted but any other name it mentions counts; a `case` pattern that
happens to be a function name scores as a call at depth 0 and taints nothing;
a file the lexer cannot close is reported rather than read as clean.

What it does not reach, said once: `eval` and a name reached through a
variable; a subshell made by something that is not a bracket, a pipe or a `&`;
a trap REGISTERED inside a function that itself runs in a subshell; and
anything outside `scripts/` (plugin/ holds one script and registers no EXIT
trap).

## Doors

`make scripts-check` (0.13s for this census, 0.00s for its control, one warm
filtered run) and, through it, `make tree-check`. The pin enumerates a
relative tree path, so it is in `internal/treepins`'s derived tree-wide class:
named by `QA_SCRIPTS_PINS`, with its enumeration row and the pin count
(one hundred and forty-three to one hundred and forty-four; doors unchanged at
seventeen) in `treewidedoor_qa_test.go`'s head comment. The control arm plants
fixtures under `t.TempDir()`, enumerates nothing, and needs no door.

## Not closed here

- ranger-base-myjcz
