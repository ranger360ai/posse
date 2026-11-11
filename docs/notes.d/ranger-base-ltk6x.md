# The fixture that built its commit with the one verb the shim refuses (ranger-base-ltk6x)

`make test` was red at arm 1 for every gated seat from 2026-11-10 23:19 to
2026-11-11, and green for CI the whole time. Two tests, one fixture helper,
one token.

## What was red

`cmd/posse/version_test.go`'s `tempRepoOfWorkingTree` built its throwaway
repo with `git write-tree` + `git commit-tree` + `git update-ref`. Since
f445f129 (ranger-base-crlty, rule family ranger-base-r3pri) the L1 git shim
refuses `commit-tree` outright:

    refused by posse gate: git commit-tree <tree> -m fixture — git
    commit-tree makes a commit no pre-commit or commit-msg hook ever saw
    (rule: hookskip:plumbing-commit, ranger-base-r3pri)

so `TestVersionNamesTheCommitWithoutTheLdflag` and
`TestMakeBuildStampMatchesSourceBuildStamp` failed for any gated persona.
That reds `cmd/posse`, which reds arm 1 — and `make test`'s arms are four
separate recipe lines (Makefile:332-335), so make aborted the recipe and
**arms 2 and 3 never ran at all** for any crew seat.

## Why the fixture was spelled that way, and why the reason had expired

The helper's own comment said the plumbing was there *because* `git commit`
is denied to fleet personas, and "a fixture no persona can build is a test
the fleet never runs". The goal is right. The spelling was one wall out of
date in both directions:

- the PID rule is `Bash(git commit unless --)` — it denies the UNQUALIFIED
  form only, and a path-qualified commit passes it;
- `commit-tree` had meanwhile become the one spelling no persona could
  build.

MEASURED 2026-11-11 (this box, linux 7.2.15, git 2.56.0, from a session
worktree, through the seat's own `git` on PATH), in a scratch `git init`
repo:

    git commit-tree <tree> -m fixture     rc=1   refused (hookskip:plumbing-commit)
    git commit -q -m fixture -- .         rc=0   HEAD written
    git write-tree                        rc=0
    git update-ref HEAD <sha>             rc=0

The fix is `git add -A` (a pathspec matches only what git already has an
index entry for) then `git commit -q -m fixture -- .` then
`git rev-parse HEAD`. That is also the spelling every other fixture in the
tree already uses — `internal/treepins/fixturegitpin_qa_test.go` and the
sites it converted, `cmd/posse/adrcensus_qa_test.go`,
`statuslaunchergov_qa_test.go`, `gatesbuildsource_qa_test.go`,
`policypins_qa_test.go`, `outguard_qa_test.go` — so this was the one site
off the house convention, not a new convention.

A real commit dispatches hooks, which is the whole point of the shim's
rule, and here there are none to dispatch: the fixture repo is `git
init`-fresh, with no `core.hooksPath` and no `init.templateDir` in any
scope on this box (MEASURED: both `git config --global --get` reads empty).
Neither test's subject — a version string and a build stamp — can see the
difference.

## Why internal/posse's identical commit-tree fixtures are green

`internal/posse/ciwatch_test.go` and `adrcensusrepo_qa_test.go` build their
fixtures with `commit-tree` too, and arm 1 reported `ok internal/posse`.
They go through the package's own `git` helper (worktree.go), which runs
through `protocolCmd` → `protocolBin` → `resolveOutside`: posse's own
protocol children resolve git OUTSIDE the gates dirs and run with a
gates-free PATH (ADR 0077 D3, ranger-base-cwsi8). They never meet the shim.

`version_test.go`'s helper is a bare `exec.Command("git", …)`, which
resolves the bare name against the TEST process's `$PATH` — never
`cmd.Env` — and a gated seat's PATH leads with its own gates bin dir. That
is ranger-base-ebhdx's finding, and it is the whole of the difference.

## The gap that let it through, and what is NOT done here

`internal/treepins/fixturegitpin_qa_test.go` is the pin for the sibling
class — a bare-name git child whose `.Env` DROPS the launcher's config pin,
refused as `hookskip:config-env`. This helper appends to `os.Environ()`, so
it carries the pin and is correctly compliant under that pin. Its refusal
came from a different rule, and **no pin in the tree reads "a bare-name git
child in a test calls a verb the L1 shim refuses outright"**.

Censused 2026-11-11 over `cmd/` and `internal/treepins/` (the two packages
that do not step out from behind the gates in `TestMain`): after this
change there are no remaining sites of that class — no `commit-tree`, no
`--no-verify` form, no `push`, no unqualified `commit` reached through a
bare-name git. The pin that would KEEP it at zero is filed separately
(ranger-base-pn99n); it needs a Makefile door of its own under the
tree-wide-door register, which is a deliverable and not a line in this
diff.

## The thing worth remembering

It is a FLEET-ONLY red. ci.yml is ungated, so CI could not see it and will
not see the next one. The only signal was a seat typing the house command,
and the two closes that did type it that day were reading over a /tmp whose
inode table was full, where `cmd/posse` cannot be read at all — so these
two reds were invisible underneath a different class
(ranger-base-akk9f's relocated-root pins). ranger-base-r3pqf /
ranger-base-5fond / ranger-base-ddgfr (ADR 0088's landed check) are the
mechanism that would have named the green-to-red stretch; this is a worked
example for whoever builds it.

A second, cheaper lesson landed in the same diff: the helper reported
`git commit-tree …: exit status 1` and nothing else, because it used
`cmd.Output()` and printed only the error. A gate refusal says why on
stderr and nothing on stdout. The helper now carries `(*exec.ExitError).Stderr`
into the `t.Fatalf`, so the next refusal names its own rule and wall in the
test output.
