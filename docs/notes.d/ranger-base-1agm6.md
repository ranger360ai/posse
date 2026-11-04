# ranger-base-1agm6 — verifying four closes against the git hook-skip arm

Verify bead over `ranger-base-z8kym`, `ranger-base-zvg7q`,
`ranger-base-ap4p6`, `ranger-base-loi28`. Two verified, two escaped. Every
number below is MEASURED 2026-11-04 on this box (darwin 26.7.1, git 2.50.1
Apple Git-155, `/bin/sh` = bash 3.2.57, `/bin/dash` = Apple dash-16) unless
it says otherwise.

## 1. git UNQUOTES an alias value; the arm's splice does not (z8kym, LIVE)

`hookskip.go`'s header says of the alias splice: "it splits on IFS and does
not unquote, **which can only refuse more**." The second clause is false, and
in the unsafe direction. git's `split_cmdline` unquotes, so for

    alias.x = commit '-n' -q -m x --

git's argv is `commit -n …` and the arm's is `commit '-n' …`. `'-n'` is not
`-n`, so no scanner in the arm matches it.

MEASURED through a real rendered claude seat shim (`WrapWithGates`, crew deny
list) at main `fe39111a`, five legs, each exit 0 with the commit LANDED (HEAD
subject is the alias's own `-m`) and the managed `pre-commit` fired **zero**
times:

| alias value | what git ran |
|---|---|
| `commit '-n' -q -m … --` | `commit -n` |
| `commit "-n" -q -m … --` | `commit -n` |
| `commit '--no-verify' -q -m … --` | `commit --no-verify` |
| `-c 'core.hooksPath=/dev/null' commit -q -m … --` | hooksPath dropped |
| `-c 'alias.inner=commit -n' inner -q -m … --` | both, chained |

Controls in the same run: the plain commit fired the hook once, and the
UNQUOTED persisted alias `pci` was refused under `hookskip:alias`. The shipped
`TestQAHookSkipRefusedOnEveryRuntime` was green throughout — every one of its
alias legs is unquoted, so no shipped leg reaches this.

**Reachability, which is why it is P2 and not P1.** A seat cannot PLANT such
an alias through the shim. All eight `git config` write spellings tried
(bare, `--global`, `--add`, `set`, `--local`, `--replace-all`, after `--`,
`--file .git/config`) are refused under `hookskip:config-write`, and a quoted
`-c` KEY is rejected by git itself (`error: invalid key: 'alias.x'`). The hole
needs an alias ALREADY in a config file: a pre-existing operator or employer
alias — the shape `ranger-base-d4bk4` is filed about — or a hand-edited
config, an ADR 0025 residual. The resolver's whole job is to judge an
expansion it did not write, so this sits inside z8kym's scope.

Pinned GREEN asserting the hole, with the inversion in its failure message:
`internal/posse/verify_1agm6_qa_test.go`
(`TestQAHookSkipAliasSpliceDoesNotUnquote`; inverted and renamed
`TestQAHookSkipAliasSpliceUnquotes` by the fix, ranger-base-e09l0).
Mutation-checked both arms — a simulated fix (the resolver also refusing a
quoted expansion) reds all five legs with the "FIXED — invert this
assertion" message; dropping `alias.*` from `hookKeyArms` reds all eight
boundary legs with "THE HOLE GOT WORSE", each then planting at exit 0.
`hookskip.go` md5 restored after each.

Also checked and CLEAN: all 45 names in `hookSkipBuiltins` are real builtins
in `git --list-cmds=builtins` (git 2.50.1), so no listed name lets its alias
through unjudged.

## 2. `remote -v prune` is a read to the arm and a ref update to git (ap4p6, LIVE)

`hookSkipReadSubverbs` admits `remote` when its next word is `""`, `-v`,
`--verbose`, `get-url` or `show`. **git accepts an option BEFORE the
subcommand**, so `remote -v prune` matches the read arm and then deletes refs.
That violates the table's own membership rule (a) — "performs no reference
update" — and `reference-transaction`, which dinesh's own note names as the
one hook that CAN veto, runs exactly there.

MEASURED at `72f5fa69` (ap4p6's commit; its code is NOT on main), a
`reference-transaction` hook in the managed hooks dir:

| leg | arm | ref deleted | reftx hook fired |
|---|---|---|---|
| `-c core.hooksPath=/dev/null remote prune origin` | REFUSED | — | 0 |
| `-c core.hooksPath=/dev/null remote -v prune origin` | allowed, rc 0 | yes | **0** |
| `-c core.hooksPath=/dev/null remote --verbose prune origin` | allowed, rc 0 | yes | **0** |

Control: past the shim, a real `remote prune` fires that managed hook 3
times — so "fired 0" is skipped, not absent. One word moved turns a refusal
into a pass. `remote -v update` is the same spelling class (`update` fetches,
so it moves refs); not separately measured.

`TestQAHookSkipReadVerbsAreNotHookVerbs` does not reach this: it asserts the
two tables are DISJOINT and that each verb reaches the render, neither of
which is a statement about a subverb preceded by an option.

## 3. The dot-source trap was real and the Linux wall was never open (zvg7q, VERIFIED)

Reproduced the direction that matters rather than the one the close logged.
After the fix the pin EXECS a `#!/bin/sh` shebang, so a PATH `sh`→dash shim
can no longer reach it — on darwin that run is bash either way, and the
close's "green with and without that dash PATH" cannot bear weight for the
fixed form. The claim that counts is about the ARM, and it holds: rendering
the shipped arm with a `#!/bin/dash` shebang and exec'ing it, all four hide
spellings (`GIT_CONFIG_SYSTEM=/dev/null`, `=<other file>`,
`GIT_CONFIG_NOSYSTEM=1`, `=true`) are refused by name under
`hookskip:config-env`, all three controls pass, and exactly 4 refusals.log
lines are written — byte-identical to the `/bin/sh` (bash) run. ci.yml run
41816913260 at `e30e6b73` is green on all six jobs including
`ubuntu-latest, 1`, the job that was red.

## 4. Scrubbing HOME is not an empty global scope (loi28, VERIFIED)

Both repro halves green under the seat env that caused them (this seat carries
`GIT_CONFIG_GLOBAL=~/.gitconfig`, `GIT_CONFIG_SYSTEM=/etc/gitconfig`): the
`internal/posse` leg ok 0.5s, `-run TestQAHookFreshness ./internal/treepins`
ok 127.7s, all ten. The global file really does carry a `user.email`, so the
fixture has something to find.

Mutation-checked, md5s restored:

| mutant | result |
|---|---|
| drop `gitScrubGlobalScope(t, "")` from `SkipsAbsentSourcesSilently` | **FAIL** |
| drop only `t.Setenv("GIT_CONFIG_GLOBAL", global)` | **FAIL** |
| drop only `t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)` | survives |
| drop the two config-in-env setenvs | survives |
| revert the treepins install-hooks leg to `append(os.Environ(), …)` | **FAIL** |

The two survivors are structural, not a gap: there is **no `/etc/gitconfig`
on this box** and `GIT_CONFIG_PARAMETERS` is empty, so those two scopes have
nothing to hide — the same "nothing for a content comparison to find" shape
as `ranger-base-r3pri`'s system arm. jian-yang measured the four-way claim
separately (`git config --get-all user.email` exits 1 under all four) and the
helper's doc comment says why the legs are there. Keep them.

## 5. The two closes' hookskip.go changes have a semantic conflict, not just a textual one

`ranger-base-ap4p6` (`72f5fa69`) is not on main — merge-back blocked,
`ranger-base-hogeb`, open. Its conflict with `ranger-base-z8kym`
(`bdd3954b`) is in `internal/posse/hookskip.go`, and resolving it has a
CHOICE in it that neither close had to make:

- z8kym's resolver re-walks `posse_hs_check` with the expansion spliced in.
- ap4p6 defers the `-c` hook-key refusal until the verb is known to be a read,
  and an UNKNOWN verb — which is what an alias name is on the first walk — is
  not a read.

So on the merged arm, whether `git -c core.hooksPath=x <alias>` is refused
depends on whether the deferred refusal fires on the FIRST walk (before
resolution) or is re-evaluated after the expansion is spliced. Fire first and
z8kym's "allowed" leg `pcm` (an alias for a plain commit) breaks; defer past
resolution and `-c core.hooksPath=x <alias-expanding-to-a-read-verb>` reopens
the hole ap4p6 says stays shut. Whoever lands `hogeb` owes a leg for each
direction.
