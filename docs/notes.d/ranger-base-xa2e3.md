# The gate that guards the tree made the suite that guards the gate unrunnable (ranger-base-xa2e3)

2026-11-10. Arm 1 of `make test` was red on this box before any change: ten
tests in `cmd/posse` and `internal/treepins` commit a fixture into their own
throwaway repo under `t.TempDir`, and posse's own L1 git shim refused every one
of them under `hookskip:config-env` (ranger-base-r3pri, ADR 0079). Reproduced
twice — once over a working tree and once over a pristine `git archive HEAD`
copy — so nothing in the tree caused it.

The rule is right about a commit into THIS repo: a managed hook a scanner may
live in must never be skipped (ADR 0052). What it was reading was a commit into
a scratch repo that has no hooks and is not this repo. The ten failures read to
the next person as real drift in the ADR census and the policy pins, which is
the expensive part.

## Two facts make it, and neither is in the test's source

MEASURED 2026-11-10, this box, git 2.56.0, from a session worktree.

1. **`exec.Command("git", …)` resolves the bare name against the calling
   PROCESS's `$PATH` — never `cmd.Env`** — and a dispatched seat's PATH leads
   with its own gates bin dir. So each fixture's own `git` *is* the shim,
   whatever PATH it then hands the child. This is ranger-base-ebhdx's finding,
   and it is why `cmd/posse` and `internal/treepins` each already carry a
   `gitOutsideGates` helper; these ten sites were simply never converted.

2. **The seat's environment pins global and system scope.** A child of a posse
   session reads `GIT_CONFIG_GLOBAL=<the operator's global file>` and
   `GIT_CONFIG_SYSTEM=/etc/gitconfig` (obdnk's pin, the claude inlet's
   `GIT_CONFIG_SYSTEM`), and this box's global scope carries four `alias.*`
   keys — hook-relevant to the arm, since an alias can spell `commit -n`. A
   fixture that DROPS or OVERRIDES that pin makes the shim's two sides differ,
   and it refuses.

Fact 2 is why the refusal is not universal, and the asymmetry is the whole
reason a grep reports the wrong set — the green half and the red half spell the
same thing at the call site. Three commits through the shim into a scratch
repo, same box, same day:

| the child's environment | verdict |
|---|---|
| `HOME` moved, the rest inherited | **allowed** — the pin still carries global scope, so the two sides are equal |
| `GIT_CONFIG_GLOBAL=/dev/null`, `HOME` inherited | **refused** |
| a replacement `cmd.Env` (HOME moved, the pin not carried at all) | **refused** |

So `internal/treepins`' bottle fixture, which moves `HOME` with `t.Setenv` and
hands its git child no `cmd.Env` at all, is green, and the policy pins beside
it, which append `GIT_CONFIG_GLOBAL=/dev/null`, are red. `hookskip.go`'s own
comment says `GIT_CONFIG_GLOBAL=/dev/null` "on a box with no global hook config
hides nothing, which is every hermetic test fixture in this repo" — true of a
box with no global alias, and this is not one. That sentence is the assumption
that failed, and it will fail on any box whose operator has git aliases.

A third fact explains why only two packages were red: **`internal/posse`'s
`TestMain` steps out from behind the gates for the whole test binary**
(`herdr_test.go`, `os.Setenv("PATH", PathOutsideGates(""))`, rangerhq-8sd).
Thirty-nine of its own git children are the same hazard shape and that one line
is all that holds them.

## The fix, and what was deliberately not done

Three files, four exec sites, ten tests: resolve the fixture's binary with
`gitOutsideGates(t)` and exec that. The hermetic environment stays exactly as
it was — it was never the part that was wrong.

- `internal/treepins/policypins_qa_test.go` (`ppRepo`)
- `internal/treepins/pidfence_qa_test.go` (`TestDraftPidDenyPromoteStagesNothing`)
- `cmd/posse/adrcensus_qa_test.go` (`TestGatesAdrCensusDispatches`, two sites)

**The rule is not narrowed.** A carve-out keyed on the repo the commit is AIMED
at — a `-C` target outside this checkout and outside every worktree — would
narrow an enforcement line, which wants the operator's eye (ADR 0025) and a
security-lane verify before it closes. The way through needs neither, so the
bead's open question is answered the cheap way.

**The packages are not stepped out wholesale either.** `cmd/posse`'s PATH leak
is a LIVE DEFECT PINNED GREEN in `gateschainpathleak_qa_test.go`: filed as
ranger-base-83rv, refiled as -1xln6, and closed "not doing" by the operator
triage sweep of 2026-09-04 — "test hygiene at three QA sites with no observed
failure; not a defect". That pin's own closing sentence is this bead:

> The leak costs nothing while that stays true; the day a crew deny covers one
> of those verbs, the three sites go red or, worse, green on a refusal the
> probes cannot tell from the gate chain's own.

That day arrived when the `hookskip:config-env` arm landed. Converting the
sites that went red answers it without reversing the ruling or deleting the pin
that predicted it.

## The pin

`internal/treepins/fixturegitpin_qa_test.go`, doored in `$(QA_CORPUS_PINS)` —
`make corpus-check`, 0.17s of that door's 4.9s.

`TestQAEveryFixtureGitThatUnpinsGitsConfigRunsOutsideTheGates` censuses every
`_test.go` file in the tree for a bound bare-name git child whose `.Env` is
assigned, in the function that exec'd it, to a value that either names
`GIT_CONFIG_GLOBAL` / `GIT_CONFIG_SYSTEM` / `GIT_CONFIG_NOSYSTEM` or reaches no
`os.Environ()` at all. The env is resolved ONE HOP — through a local slice and
through a same-package helper — which is what all three converted sites needed.
`HOME` and `XDG_CONFIG_HOME` are deliberately not in the variable list: they
move global scope only when `GIT_CONFIG_GLOBAL` is absent, and on a posse seat
it never is, so listing them would flag the whole green half.

Immunity is **derived**, never listed: a package is excused when its own
`TestMain` strips the gates from `$PATH`, so the day `internal/posse`'s line
goes away this pin names its thirty-nine sites instead of going quiet. A
function that strips `$PATH` for itself (`bdhookcommit_qa_test.go`'s shape) is
read as compliant — that is the third honest spelling of the fix. There is no
exemption roster, for `gittempdirroot_qa_test.go`'s reason: complying is one
token, the helper is already in both packages, and an exemption mechanism here
would only ever be used to re-open the class.

MEASURED 2026-11-10 at this bead's commit: 768 test files, 66 bound bare-name
git children, 39 handed an env that unpins git's config scope — all 39 in
`internal/posse`, every one excused by that package's `TestMain`, and 0
anywhere else. The unit is one finding per CHILD and not per `.Env` assignment:
`cmd/posse`'s adrcensus fixture rebinds `cmd` and assigns its `.Env` three
times, and the first reading of this census reported that one child three times
at a line none of those assignments is on (that reading said 74 and 45).

`TestQAFixtureGitCensusCanStillSayNo` is the control: eleven synthetic sources
covering each shape that separates this class, including the three converted
ones, the two spellings of the fix, the green half, and a `TestMain` that
strips something other than `$PATH`. It enumerates no directory, so it is not
in the tree-wide class and needs no door.

## Mutation

Each of the three conversions was reverted in the worktree in turn, after the
commit, and the census named that site and no other; the ten tests went red
again with the same `hookskip:config-env` refusal. The pin was green on the
unmutated tree between every arm.

## Not claimed

That the thirty-nine sites in `internal/posse` are safe on any other box, or
that a gates-leading `$PATH` is the only route to the shim. Both are properties
of the environment rather than of this tree, which is why arm 1 counts them
rather than converting them.
