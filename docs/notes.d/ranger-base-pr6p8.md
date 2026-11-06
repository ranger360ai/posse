# ranger-base-pr6p8 — verifying four closes, and the conjuncts nothing witnesses

MEASURED 2026-11-06 on this box (darwin 25.6.0, git 2.50.1 Apple Git-155), at
`main` fd2f7e24, from holden's own worktree. Four closes:
ranger-base-87zme, ranger-base-91rep, ranger-base-0ddmu, ranger-base-ava0x.
All four verify. The findings are four unpinned conjuncts in 87zme's arm, filed
as one bead; everything else is prose.

Landing check first: 87zme, 91rep and 0ddmu landed intact. Each closer named a
pre-merge sha the launcher's fast-forward rewrote (18d062cb→638cdf48,
c2282cb4→660a3f0c, 2cf57c6f→fd2f7e24); comparing the *patches* rather than the
trees — the bases differ, since six hook-skip beads landed through the same
files — every file's content change is byte-identical, with the one expected
line of drift in `docs/notes.d/README.md` where the index picked up sibling
fragments as they merged. ranger-base-ava0x did not land; §4.

## 1. ranger-base-87zme — the ADR 0079 compare resolves `~` per read

**Every MEASURED claim in the close reproduces.** Eight git facts, read
directly: `--type=path` expands `~` per read from that read's HOME (the same
value reads `<launcherHOME>/managed-hooks` and `<seatHOME>/managed-hooks` from
one file, and `~/managed-hooks` raw); an unexpandable `~user` exits **128 with
stdout empty** while the `--name-only` read on the same file still exits 0;
no-match is rc 1, so `-le 1` is the right boundary; an *empty* value under
`--type=path` is rc 0, not an error, so it causes no over-refusal; a relative
value is returned unchanged; a leading-`~` **alias** value expands too, which
is the over-refusal the close documents; `--system` ignores
`GIT_CONFIG_NOSYSTEM`; and this box has no `/etc/gitconfig`, which is why the
new system pin has to force `GIT_CONFIG_SYSTEM` through a git wrapper rather
than hope to find something there.

**The `GIT_CONFIG_NOSYSTEM` case arm is safe in both directions of
disagreement**, which is worth writing down because it looks fragile. The want
side treats `''|0|false|no|off` (case-insensitive) as falsy and reads system;
git parses the same env var with its own bool rules. Where the two disagree —
`" 0 "`, say — one side reads system lines and the other does not, the strings
differ, and the arm refuses. There is no spelling where a mismatch opens it.

**Done when, item by item.** The h0hjx pin is inverted and green, and its
`INVERT ME` message is gone from the tree. ADR 0079 D2, the cost line
(2+candidates → 3+candidates, which matches the exec count in the rendered
function), the "identical config is allowed" consequence and a new
over-refusal consequence are all amended, and the amended text matches what I
measured. Pins 1-7, `TestQAHookSkipMovedHomeFollowsNoTildeInclude` and
`TestQAHookSkipRefusedOnEveryRuntime` are green: **14 tests, 28 real subtest
legs, 0 skips, 8.668s.** (Six of those 14 read `--- PASS (0.00s)`; they are
parents of `t.Parallel()` subtests, and `-v` lists the real legs one indent
in.) Deploy is correctly scoped out — the arm is in the L1 shim, which
`RenderGates` writes at every launch, so it reaches a seat when a posse built
from this commit is installed and the seat relaunches. That is an operator
step, not `gates install-hooks`, and not a gilfoyle bead.

**Eight hostile spellings, none of them a bypass.** Through a real rendered
shim, with the managed hook reachable *through the tilde* as a leading control
so that every "fired 0" below is a statement about something:

| leg | verdict | managed | seat's own |
|---|---|---|---|
| control: plain commit, tilde config | allowed | 1 | 0 |
| `HOME=evil`, byte-identical tilde config | refused | 0 | 0 |
| `HOME=`<same dir spelled absolute> | allowed | 1 | 0 |
| `GIT_CONFIG_GLOBAL=`<launcher's own file>` HOME=evil` | refused | 0 | 0 |
| `HOME=`<tilde reached through `.config/git/config`> | refused | 0 | 0 |
| `env -u HOME` | refused | 0 | 0 |
| `includeIf gitdir:~/repo` (HOME-relative *condition*) | refused | 0 | 0 |
| tilde under `hook.pre-commit.command` | refused | 0 | 0 |

The fourth is the sharpest: keep the operator's config *bytes* and move only
HOME, so git reads the real file and expands `~` from the seat's home. Refused.
The third is the one that matters in the other direction — an equivalent
absolute spelling is still allowed and still fires, so the fix did not buy its
refusals by refusing everything.

**Why there is no gap left of the shape 7czd6 found:** both sides resolve
values with the same `--type=path` git itself uses for `core.hooksPath`
(`git_config_pathname`), from their own HOME. If the two sides are equal, the
path git resolves at hook time *is* the launcher's. A relative value reads
identically on both sides and is correctly allowed, because it does not depend
on HOME at all — moving HOME changes nothing about it.

### The finding: four conjuncts no shipped pin witnesses

Ten mutation arms, each applied to `internal/posse/hookskip.go`, md5-checked,
built, and run against the full `TestQAHookSkip` filter. **Six killed:** both
sides raw (3 tests red); the have side raw alone; the want side raw alone;
the want *candidate* read's rc check; the have filter dropping `system` lines;
and the want `--system` read removed entirely.

**Four correctly-aimed survivors** — the mutation applied, the file changed, the
tree built, and no shipped pin went red:

| line | conjunct removed |
|---|---|
| `hookskip.go:806` | the **have** side's `[ $? -le 1 ] \|\| return 1` |
| `hookskip.go:835` | the want **system** read's `[ $? -le 1 ] \|\| exit 1` |
| `hookskip.go:834` | `--includes` on the want **system** read |
| `hookskip.go:842` | `--includes` on the want **candidate** read |

The fourth is the one worth staring at. ADR 0079 **pin 8**
(`TestQAHookSkipMovedHomeFollowsNoTildeInclude`) is the *include* pin — and it
stays green with `--includes` gone from the want read, because what refuses its
fixture is the tilde expanding differently, not the include being followed. The
pin is partly green for a reason other than its subject.

`TestQAHookSkipUnexpandableTildeFailsClosed` plants its bad tilde in a
candidate file, so it exercises `:843` and never `:835` — the two rc checks are
separate conjuncts and only one has a witness. On reachability I have
reasoning, not measurement: the have-side check at `:806` looks subsumed by the
line/entry count for every input I could construct (a bad value fails `gall`
while `gnames` still lists its key, so the counts disagree and it refuses
anyway), and the two `--includes` removals look fail-closed, since any
difference between the sides refuses. That is why this is debt and not a P1 —
but a conjunct of a security wall whose removal nothing notices is a conjunct
that can be refactored away in silence.

**Mechanics worth keeping: a `perl -0777` substitution with no `/g` takes the
first match in the whole FILE, and this file documents its own rendered reads
in a header comment.** My first `--type=path` arm edited the comment at `:753`,
not the read at `:804` — the md5 changed, the tree built, every pin stayed
green, and it scored as a survivor. It is the exact shape ORDERS warns about
(*a survivor usually means you mutated something the pin never reads*) and the
cheap confirming read was again the wrong one. The second pass prints the
`diff` each arm actually made before running it, which is what caught it; aim
every arm at a code anchor (`posse_hs_gall=$(%s config ...`), never at the
spelling alone.

## 2. ranger-base-91rep — the ceiling arm budgeted against itself

**Reproduced at the flake's own condition.** The report is 5.490s at a
one-minute load average of ~85. Three runs here at load **81.18**: ratio
**2.408, 2.144, 1.971**, all green, with the absolute ceiling leg at
1.359s / 0.946s / 0.819s. The absolute number is still the thing that moves;
the ratio is not.

**Both directions, independently of the close.**

- *The flake class, cured.* A uniform `sleep 0.3` per call on **both** shims — a
  loaded box, modelled deterministically — puts the absolute ceiling leg at
  **6.592s and 6.573s**, past the retired 5s bound twice, while the ratio reads
  **1.032 and 1.031** and the pin passes.
- *The regression the pin exists for.* Extra scan passes injected into the
  rendered arm only: N=4 reads ratio **4.103 and passes**; N=14 reads **11.810
  and reds**, naming both legs and the ratio. So the bound is blind to a 4x
  drift and catches a 12x one, which is the order of magnitude ADR 0069's
  linearity claim would break by.

One prose correction, which changes nothing the pin does. The header says of
the 300ms slack: *"It is deliberately larger than the whole measured ceiling
leg, so it can only ever forgive, never accuse."* It is not — the ceiling leg
measured 0.819-1.359s here and 0.30-0.48s on the prototype's idle box (ADR 0069
V7), all of them larger than 300ms. **And the sentence describes a pin that
would not work:** a slack larger than the whole ceiling leg satisfies the bound
by itself and makes the `6x` factor decoration. What is true is the clause after
it — a positive slack can only loosen a bound — and that holds at any
magnitude. Also, at load 81 the ratio reaches 2.408 against the close's stated
worst of 2.27 (taken at load 26-60), so the headroom is ~2.5x, not ~2.6x.

## 3. ranger-base-0ddmu — the fresh-reading row's wall deadline

All three arms the close reports reproduce, each actually run, plus a fourth.

1. *Tolerance temporarily negative* — `fresh_reading` SKIPs (`gained 0s`),
   `no_meter_guard` passes untouched. The guard is reached, from the live row
   only.
2. *The call site handed `ref.UTC()`* — FAILS with "needs a reference straight
   from `time.Now()`", rather than silently comparing one clock to itself.
3. *The `-3h` repro WITH the guard installed* — **FAILS**, naming
   `plan meter BLIND 3h00m`. It does not skip. This is the arm that matters:
   the skip does not mask real staleness.
4. *Mine, the QUANTITY mutant* — `wallSince` reporting `+2h`, which is what a
   box sleep looks like to this code. `fresh_reading` SKIPs naming the measured
   **2h0m0s**, so `wallSince`/`wallJump` are read for their value and not
   merely for branch reachability (arm 1 is a threshold mutant: `j > -1m` is
   true for any `j`). In all four arms `no_meter_guard` PASSES — the free
   positive proof that the fixed-calendar-date row carries no wall premise,
   which the close asserted in prose.

The scope correction the close owed ranger-base-w4enh is on that bead, read
back and complete: cmd/posse was not done, its census is seven files not five,
and an age-literal grep cannot find this row, which carries no age literal.

## 4. ranger-base-ava0x — a good pin that nothing reading `main` can see

The pin is sound and the close's work does not reach `main`.

- **At its own sha** (81e4c957): `TestQAHookSkipNewlineForgedGlobalLineIsRefused`
  passes on claude and codex with real legs, inside a green `TestQAHookSkip`
  filter, from a tree extracted with `git archive`.
- **Against `main`**: its base is 1f4da269 (2026-11-04), **six commits behind
  main on `hookskip.go`**, 87zme's `--type=path` fix among them, and the
  merge-back is blocked on a real conflict (ranger-base-mfk8v, P1). So the
  question the close cannot answer is whether the pin still holds once that
  conflict is resolved. Grafting the test and its three helpers verbatim onto
  `main` — leaving the conflicted file alone, since resolving it is mfk8v's
  work — **it passes on both runtimes, witness included.**
- **And it is the only witness.** Mutating D2's count away in `main`'s own
  `hookskip.go` (`[ "$posse_hs_gl" -eq "$posse_hs_gn" ]` → `[ 0 -eq 0 ]`) reds
  nothing in the shipped `TestQAHookSkip` filter — that was survivor M10 of §1's
  census. Under the graft the same mutation reds both runtimes for exactly the
  right reason: both legs land at **code=0 with the managed hook unfired**, the
  full bypass. ADR 0079's Pins owed item 9 on `main` still reads "owed by the
  bead cut under ranger-base-o6xr6".

So the conflict is textual, not behavioural, and the measurement is on mfk8v.
Nothing is filed for it here: that bead already owns the work.

## Recorded, no bead

- `internal/posse/dataceilingargv_qa_test.go` — the slack sentence in
  `TestQABdCeilingTwentyCallsFastEnough`'s header (§2), and the ~2.6x headroom
  figure, now ~2.5x at load 81. Escaped from ranger-base-91rep.
- `internal/posse/verify_h0hjx_qa_test.go:91` —
  `TestQAHookSkipTildeHooksPathIsJudgedByRawText` now asserts that the value is
  **not** judged by raw text. The name is the citation ADR 0079 pin 10 and
  87zme's own done-when use, and the header explains the inversion, so keeping
  it is defensible; a reader grepping the claim still finds a test asserting its
  negation. Escaped from ranger-base-87zme.
