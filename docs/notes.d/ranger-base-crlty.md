# Three hook-skip routes the git shim did not judge (ranger-base-crlty)

The verify bead for `ranger-base-zwkgb` (GH#15) found three ways a seat could
commit with the managed hook unrun, at rc 0, with nothing refused. All three
are now refused by the L1 git shim's hook-skip arm (`internal/posse/hookskip.go`)
and pinned in `TestQAHookSkipRefusedOnEveryRuntime` (claude and codex).

**1. The ADR 0052 D2 exemption was a glob.** `GIT_CONFIG_KEY_<n>=core.hooksPath`
was allowed at any one-level child of `<StateDir>/hooks`, and git reads a
missing hooksPath as no hooks. Now `posse_hs_redirect` allows only
`<StateDir>/hooks/<name>` with `<name>` a `ValidName`, a directory that is not
a symlink, holding posse's `posse-prepare-commit-msg` member as a regular
file. Refused now: `<root>/no-such-session`, `<root>/..`, `<root>/.`, an empty
dir, a symlink to a render.

*Diverged from the finding:* it asked for "exactly THIS session's redirect".
The shim is rendered once per persona and shared by all of that persona's
sessions, and nothing in a seat's env names its session that the seat cannot
also set. Two residuals are named in the RESIDUALS paragraph instead: another
live session's render (posse's wall runs there and forwards), and a render
forged by hand under `<StateDir>/hooks` (posse's own state, the same class as
a hand-edited `.git/config`).

**2. `clone`/`init` write the new repo's LOCAL config.** New rules:
`hookskip:config-write` for `clone -c/--config` naming a hook key (every
spelling: `-c K=V`, `--config K=V`, `--config=K=V`, `--co=K=V`, `-qcK=V`), and
`hookskip:template` for `--template`/`--te…` or `GIT_TEMPLATE_DIR` whose
`config` carries one (judged by content, so a clean template still works). A
relative template under `-C` is refused unread. `init.templatedir` joins the
hook-relevant keys, so `-c`, `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_KEY_<n>` and
`git config` writes naming it are refused by the existing arms. `clone`,
`init` and `submodule` join `hookSkipVerbs`, so the env arm runs on them.

MEASURED 2026-11-10, Linux, git 2.56.0, `/usr/bin/git` in a scratch dir: each
of `clone --te=<tpl>`, `GIT_TEMPLATE_DIR=<tpl> clone`, `clone
-qccore.hooksPath=/x`, `clone --co=core.hooksPath=/y` and `init
--template=<tpl>` left the key in the new repo's local config. `git -C sub
init --template=tpl` read `sub/tpl`. `git config --file <missing>
--get-regexp` exits 1, the same as no match.

**3. `git commit-tree`** is refused (`hookskip:plumbing-commit`). It is the
ordinary plumbing spelling of a commit no hook saw. The ref write that lands
it (`update-ref`, `merge --ff-only`, `reset --soft`) is ordinary and cannot be
refused. So a commit object written by OTHER plumbing (`hash-object -t commit
-w`, `fast-import`) is disclosed as residual by class, in hookskip.go's
RESIDUALS and in the parity row's detail (`parity.go`). That same row also
stopped claiming "HOME/XDG_CONFIG_HOME moved" as a residual, which ADR 0079
closed.

`scripts/queue-cutover.sh` uses `commit-tree`. It is an operator script for
the cutover window. Run from a seat, it would now be refused at that step.

Mutation check: against the pre-change `hookskip.go`, all 24 new refused legs
exited 0 and the empty-dir redirect leg passed. With the change, all are
refused and every control still runs (the render-shaped redirect runs the
managed hook through its dispatcher).

## GH#15 Limits paragraph — draft, not posted

Posting to the issue is the operator's call (writing under the operator's
name). Suggested addition:

> The shim also refuses `git commit-tree`. A commit object written by other
> plumbing (`git hash-object -t commit -w`, `git fast-import`) and then moved
> onto a ref with `update-ref` is past the shim by class, like an absolute
> `/usr/bin/git`. The ADR 0052 redirect exemption accepts any session dir
> posse rendered, not only the calling session's own.
