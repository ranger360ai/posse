# install-hooks takes every repo on its line, not the last one (ranger-base-jamt2)

The operator's live finding, fixed 2026-11-08. `posse gates install-hooks`
parsed its arguments like this:

```go
for _, a2 := range args[1:] {
	if a2 == "--chain" { chain = true; continue }
	dir = posse.ExpandTilde(a2)   // ← the LAST one wins
}
```

so N repos installed into the last one, exited 0, and said nothing whatever
about the other N-1.

## The measurement, twice

MEASURED 2026-11-08 on the laptop by the operator, posse 0.5.3+8433641d: one
`posse gates install-hooks` over a brace expansion naming FOUR of that
instance's repos — which the shell expands to four arguments — printed the
LAST repo's two install lines and exited 0. The other three repos' hooks were
not rewritten: mtimes unchanged, and one of their ADR 0029 G12 stale rows
stayed where it was. The four are that instance's own vocabulary and are not
named here (ADR 0048 D2, ADR 0024 D3's restate-and-cite); the fixture in the
table below reproduces the shape with three scratch repos.

MEASURED 2026-11-08, darwin 25.6.0, the same claim reproduced from a clean
fixture against a binary built from `8433641d` — three scratch git repos all
three declared `private` in a scratch home's `beads_visibility:`, so ADR 0081
D1 makes that home their owner and nothing here is D4's refusal:

| binary | printed | exit | hooks present afterwards |
|---|---|---|---|
| `8433641d` | `r3`'s two `installed` lines | 0 | `r1` none, `r2` none, `r3` both |
| this fix | three headers, six `installed` lines, a count | 0 | all three, both slots |

## Why this shape is the worst one this command has

The two slots are the push gate and the shared commit wall, and this verb is
the remedy every degraded line, every G12 row and `make verify-hook-freshness`
prescribes. A run that installs one repo of four and exits 0 is
indistinguishable at the terminal from a run that installed all four: the
operator reads `installed <path>` twice and a zero exit and stops looking. And
the repos that stay stale are exactly the ones nothing else re-renders — only
this command and a dispatch write a hook, and a dispatch refreshes the common
hooks dir of the repo its worktree was cut from and no other, so a repo that
holds no session keeps whatever render it was given (INSTALL.md §9).

The cost is already on the record once. On j0imz, 2026-11-01, four repos ran
the ranger-base-g55ah-era two-rune fold for eleven minutes — the install at
21:22 left them behind, the security lane read them stale at 21:30, a
hand-typed `install-hooks` repaired them at 21:33 — and ranger-base-l717c's
bypass was live in every one of them for that window (Makefile,
`verify-gate-freshness`'s comment). This bug put the same gap one hand-typed
line away, every time, and the coordinator had been handing the operator that
line for weeks.

## What it does now

Every positional is a target. One target prints exactly what it always
printed — no header, no count, because that is the form every degraded line,
`scripts/verify-hook-freshness.sh` and most of INSTALL.md prescribes, and the
pins next door read those lines. More than one gets a header naming the repo
before each block and a count at the end:

```
install-hooks: 3 repo(s), 2 ok, 1 refused or failed
```

"ok" and not "installed", because a managed hooks path writes nothing and is
still a clean outcome (ADR 0052 D1).

**Every target is attempted even when an earlier one refuses**, and the exit
code is decided once, at the end. This is not a free extra: ADR 0081 D4's
refusal — another home's commit wall in a repo this home does not list — was
an `os.Exit(1)` in the middle of the arm, so with N targets it would have
ended the command where it stood, leaving the repos after the refused one
untouched with a nonzero exit and a refusal about a different repo as the only
trace. That is the same silence this bead is about seen from the other side,
so the per-repo work moved into `installHooksInto`, which returns a bool.

**A positional that looks like a flag is a usage error.** The old loop assigned
it to `dir` like any other positional, so where the typo landed decided what
happened: `install-hooks <repo> --chian` absolutized it to `$PWD/--chian`,
reported THAT as the repo it could not install into, and left the repo the
operator named untouched — while `install-hooks --chian <repo>` silently
worked, because the repo came last. With every positional a target, a
swallowed flag is a repo nobody asked for.

## Pins

`cmd/posse/installhooksmultirepo_qa_test.go`, five black-box arms over three
repos (three and not two: the bug kept the LAST argument, so a two-repo pin
can be passed by a command that drops everything but its tail).

| arm | claim |
|---|---|
| `InstallsIntoEveryRepoOnItsCommandLine` | all three get both slots, each is named, the count reads `3 ok` |
| `TakesChainFromAnyPositionAndAppliesItToEveryRepo` | `--chain` between two targets is still a flag, applies to both, and the repo on no command line is untouched |
| `AttemptsTheReposAfterOneItRefuses` | the middle repo's wall is another home's; the third installs anyway, the middle one is byte- and mtime-unchanged, exit nonzero, count reads `2 ok, 1 refused or failed` |
| `RefusesAFlagItDoesNotKnowRatherThanTakingItForARepo` | `--chian` first — the spelling the old loop let through — is a usage error naming the flag and the `[dir...]` grammar, and installs nothing |
| `AddsNoHeaderOrCountForASingleTarget` | one target's output is what it always was |

The first four red against `8433641d` with the messages they were written to
print; the fifth is green there and is the regression guard on the one-repo
output.

`[dir]` became `[dir...]` in `need()`'s usage string and in the help catalog,
whose entry now says what several targets do; INSTALL.md §9's "re-render each
one yourself" paragraph says it too, and says that a line of this shape used
to do less than it printed.
