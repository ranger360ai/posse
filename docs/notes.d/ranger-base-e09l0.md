# The alias splice unquotes the way git does (ranger-base-e09l0)

Finding 1 of `ranger-base-1agm6` (§1 there): `hookskip.go`'s alias resolver
split an expansion on IFS and did not unquote, while git's `split_cmdline`
does. `alias.x = commit '-n' …` was `-n` to git and `'-n'` to every scanner
in the arm, so five quoted spellings committed past the managed hook.

**Fix.** `posse_hs_split` (`hookskip.go`, `hookSkipSplit`) is
`split_cmdline` in POSIX sh: outside quotes a run of space, tab, CR or LF
separates; `'` and `"` open a quote the same character closes; a backslash
outside single quotes takes the next character. It emits every word
single-quoted, and the splice `eval`s `set -- "$@" <those words>`, so nothing
in the expansion can run, glob or split. An unclosed quote or a trailing
backslash — which git refuses itself (`bad alias.x string`) — is refused
under `hookskip:alias`.

MEASURED 2026-11-04, this box (darwin, git 2.50.1 Apple Git-155): the
splitter's words against git's own, via a non-`!` alias to a `git-argdump`
on PATH, under `/bin/sh` (bash 3.2) and `/bin/dash`. Equal on all fifteen
values tried: single, double and mixed quotes, `\"` inside double quotes,
`\` inside single quotes, `b\ c`, runs of mixed whitespace, `''` and `""`
empty words, `'it'"'"'s'`, `$HOME`/backticks/`*` inside double quotes (kept
literal), `--format='%h %s'`; and both refuse an unclosed quote and a
trailing `\`.

**Pin.** `verify_1agm6_qa_test.go` inverted and renamed
`TestQAHookSkipAliasSpliceUnquotes` (was `…DoesNotUnquote`): the five legs
are refused under the rule of the flag they spell, the hook does not fire and
HEAD does not move; three quoted expansions that skip nothing still commit
and still fire the hook once, with HEAD's subject equal to git's unquoted
word (so the splice and git agree, not merely both refuse). Mutation:
putting the IFS splice back reds all five escaped legs.

**Finding 2** (`remote -v prune`) is at `72f5fa69` only, not on main, so it
cannot be fixed or pinned from here: handed off as `ranger-base-jvwcb`, to
land with `ranger-base-hogeb`. That replay will also conflict with this
change in `hookskip.go`.
