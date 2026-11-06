# ranger-base-fe4uq — verifying four closes, and a fifth conjunct with no witness

MEASURED 2026-11-06 on this box (darwin 25.6.0, git 2.50.1 Apple Git-155), at
`main` 6125eb4e, from holden's own worktree — which is at main, so every close
below was verified against the tree they all landed in. Four closes:
ranger-base-hogeb, ranger-base-y9i3o, ranger-base-mfk8v, ranger-base-jk380.
**All four verify.** One LIVE finding, found while attacking y9i3o's
deliverable and escaped from ranger-base-87zme, not from any close here; two
prose claims RECORDED.

## 1. ranger-base-y9i3o — the four witnesses hold, and a fifth conjunct does not

gwart's three new pins (ADR 0079 pins 13-15, `hookskipwitness_qa_test.go`) do
what the close says. `hookskip.go` is byte-identical to 638cdf48's — verified,
`git diff 638cdf48 HEAD -- internal/posse/hookskip.go` empty — so these are
pins beside the wall and not a change to it.

Seven mutation arms, each applied to `internal/posse/hookskip.go` alone, each
printing the `diff` it made before running (ORDERS: md5 proves *something*
changed, never that the right thing did), run against
`go test -count=1 -run 'TestQAHookSkip' ./internal/posse -v` — 18 tests, 22
real subtest legs at this commit:

| arm | md5 | result |
|---|---|---|
| 0 none | `58b3881b84e477eef876f9095c601ddb` | ok 17.5s, 0 FAIL |
| 1 `:806` have listing's rc check deleted | `0a875d30bd1c540d780084d33d35a651` | pin 13 only, leg "the have listing's git failed" |
| 2 `:835` want SYSTEM read's rc check → bare case arm | `90d7998c9d03f3b74fa41b2c07fe113c` | pin 13 only, leg "the want SYSTEM read's git failed" |
| 3 `:834` drop `--includes` (want SYSTEM) | `0635be73096ed85bad48f747e2d9c65e` | pin 14 only |
| 4 `:842` drop `--includes` (want CANDIDATE) | `158c1c58debc7746dd4d6e4f152a82e4` | pin 15 only, both runtimes |
| 5 `:843` want CANDIDATE read's rc check deleted | `94dae1f1fa90699173106a18154009a0` | pin 12 **and** pin 13's 4th row |
| 7 `:820` D2's count → `[ 0 -eq 0 ]` | `5ea22b4aacbe7f260bc9eb6e15fb2dd9` | ADR 0079 pin 9 only, both runtimes |

Arms 0-4 reproduce gwart's own four md5s byte-for-byte and its claim in both
directions: each new pin is its conjunct's ONLY witness, and no shipped pin
reds under any of them. Arm 3's failure is the right failure, not merely a
failure: the ALLOWED leg becomes `code=1 fired=0` refused by
`hookskip:config-env`, i.e. the over-refusal the flag exists to prevent. Arm 5
confirms the close's own reading that pin 13's fourth row is a redundant
sibling of pin 12. Arm 7 is ranger-base-mfk8v's claim (§3 below).

### THE FIFTH CONJUNCT (live, escaped from ranger-base-87zme)

`hookskip.go:834` — `--type=path` on the want **SYSTEM** read — can be deleted
with the whole shipped filter green:

```
arm 6, md5 4b585517b267eb03b9201924c3316376
-	fmt.Fprintf(&b, "      %s config --system --includes --show-scope --type=path --get-regexp %s 2>/dev/null\n", q, re)
+	fmt.Fprintf(&b, "      %s config --system --includes --show-scope --get-regexp %s 2>/dev/null\n", q, re)
  go test -count=1 -run 'TestQAHookSkip' ./internal/posse  ->  ok, exit 0, 0 FAIL
```

It is the same split that gave findings 2 and 12 of ranger-base-pr6p8 — the
system read and the candidate read are separate conjuncts, and the pins live
in candidate-land — applied to a flag rather than to an rc check. pr6p8 ran
ten arms over this function and its `--type=path` arms were "both sides raw",
"the have side raw alone" and "the want side raw alone", all red; **none of
them was the want SYSTEM read alone**, so my own verification missed it.

Why ADR 0079 pin 11 (`TestQAHookSkipSystemTildeHooksPathJudgedAgainstLauncher`,
the pin that looks like the owner of this conjunct) stays green: its tilde leg
asserts REFUSED, and the mutant still refuses; its allowed leg spells the
system `hooksPath` ABSOLUTE, where `--type=path` is the identity. Both
directions survive.

**And it is witnessable** — the leg that distinguishes them is a move of
`XDG_CONFIG_HOME` with HOME left alone, so a system `~/managed-hooks` resolves
identically on the two sides and the call is ALLOWED today. MEASURED with this
probe (run from `internal/posse/`, not committed — the pin belongs to the code
lane):

```go
//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"testing"
)

func TestZZProbeSystemTildeTypePathWantSide(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	user := "[user]\n\tname = t\n\temail = t@example.com\n"
	write(t, filepath.Join(fx.home, ".gitconfig"), user)
	sys := filepath.Join(fx.home, "system-config")
	write(t, sys, "[core]\n\thooksPath = ~/managed-hooks\n")

	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git-wrapper")
	if err := WriteExecutable(wrapper, []byte("#!/bin/sh\nGIT_CONFIG_SYSTEM="+shQuote(sys)+"; export GIT_CONFIG_SYSTEM\nexec "+shQuote(fx.realGit)+" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ref := resolveHookSkipGlobalRef(wrapper, hsEnv(fx))
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	body := renderShim("hg-probe", "git", wrapper, filepath.Join(dir, "refusals.log"), "/bin/date", nil, nil, "", filepath.Join(dir, "hooks"), ref)
	if err := WriteExecutable(filepath.Join(bin, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := "PATH=" + shQuote(bin) + ":\"$PATH\" "

	hsGlobalCheck(t, fx, prefix, []hsGlobalLeg{
		{"control: nothing moved, the system tilde names the managed hook", "", false},
		{"XDG at its own default, HOME unchanged, system hooksPath = ~/managed-hooks", "XDG_CONFIG_HOME=" + shQuote(filepath.Join(fx.home, ".config")) + " ", false},
	})
}
```

PASS at arm 0 (2.61s), FAIL under arm 6 for the right reason — `code=1
fired=0`, refused by `hookskip:config-env`. The existing pin 2 fixture
(`TestQAHookSkipXDGMoveJudgedAgainstLauncher`'s "XDG_CONFIG_HOME at its own
default" leg) is the same move; what this adds to it is a system-scope tilde.

SEVERITY: debt, the same reading pr6p8 gave findings 3 and 4, and by
measurement rather than reasoning this time. Both sides read the SAME system
file, so the value text is identical on both; `--type=path` can only make them
*differ* where the resolving HOME differs, and where it differs the compare
refuses either way. Dropping the flag therefore over-refuses (an XDG-only move
at a system tilde) and opens nothing. What is lost is not a wall but a
conjunct nothing notices going away.

## 2. ranger-base-jk380 — the derivation generalizes, and the floor is where it says

`posse gates`' reading moves, which is the point of the bead. MEASURED over the
installed lag (`1f4da269..main`, installed posse 0.5.3+1f4da269):

```
git log --oneline 1f4da269..main -- <the 11 paths WallRenderSources held before> -> 0 commits
git log --oneline 1f4da269..main -- internal/posse/hookskip.go                   -> 4 commits
   638cdf48 87zme · 44c31e30 c1pqo · fc72606f ap4p6 · 2f1d59aa e09l0
```

So the operator's "0 of them rendering a wall" over three hook-skip commits was
exactly the pathspec, and the one-line fix answers it. (`narrowLag` in
wallrenderer.go:197 is a `git log` over this list; nothing else keys off it.)

Five probes on the derived half (`wrsRenderSites`,
`internal/treepins/wallrendersources_qa_test.go`), each with its diff printed,
run against arms 1 and 2 of the pin:

| probe | result |
|---|---|
| A `WallRenderSources` minus `hookskip.go` | RED, naming it "declares a `render*` func a headered render site splices into its wall" |
| B a NEW `render*`-named body renderer in a new file, called from a func in headered `gates.go` | RED, naming `zzprobeb.go` — the derivation is not special-cased on hookskip.go |
| C same, helper named `zzQuoteHelperC` (not `render*`) | green — the documented residual, reproduced |
| D `render*`-named renderer in a new file, reached from `gates.go` only through a NON-`render*`-named intermediate in a third file | green — see the RECORDED line below |
| E the body half neutered (`bodies` stays empty) | RED on the in-test witness — the derived half cannot be deleted in silence |

Census at this commit, printed from inside the pin: `renders` = cageinner,
cagelauncher, egress, gates, paneline, personamode, seatbelt; `bodies` =
exactly `internal/posse/hookskip.go`, as the close says. The other `render*`
declarers are agents.go (`renderPlaceholder`), reachability.go
(`renderedLaunchLine`), refresh.go (`renderExpiry`, `renderStamp`) and skills.go
(`renderSkillsCwd`, already hand-named); every call site of the first three is
inside its own declaring file, so none is reached from a headered file at all —
checked by `grep -rn '<name>(' internal/posse/*.go`, not by the close's word.

## 3. ranger-base-mfk8v — the replay is both sides, and the pin is still the witness

- Ancestry: 45cf010b and 47eabc3d are on `main`. The replay keeps 81e4c957's
  `replayKey` byte-identical (author email as launched, AUTHOR date
  `2026-11-06 09:36:27 -0500`, subject), and exactly ONE commit in
  `main --not 81e4c957` carries it, so the pairing is unambiguous.
- Content, by added-line containment per file (the method the block prescribes,
  and the only one that means anything here — main carries later commits over
  the same files): **217 of 217 of 81e4c957's added lines are on main
  verbatim, 0 absent.** `45cf010b` against its parent is 4 files, 217
  insertions, 12 deletions — the stat the close reports.
- Both sides, not one: the declaration list of
  `internal/posse/hookskipglobal_qa_test.go` on main (11 funcs) EQUALS the
  union of 4a21abed's (8) and 81e4c957's (9), with no extras. `hsWrappedDir`
  and the delegating `hsWrapped` are byte-identical to ava0x's versions.
- The y9i3o files the close's "trap worth keeping" is about are intact:
  `git diff 4a21abed main -- internal/posse/hookskipwitness_qa_test.go
  docs/notes.d/ranger-base-y9i3o.md` is empty.
- The pin is a witness in the resolved tree (arm 7 above): D2's count mutated
  to `[ 0 -eq 0 ]` in main's own hookskip.go reds
  `TestQAHookSkipNewlineForgedGlobalLineIsRefused` on claude and codex, and
  reds NOTHING else in the filter.

## 4. ranger-base-hogeb — the comparison reproduces exactly

The deliverable was a comparison, not a diff, and re-derived independently it
comes out line for line as `docs/notes.d/ranger-base-hogeb.md` states:

| file | added | absent from main |
|---|---|---|
| `docs/notes.d/ranger-base-ap4p6.md` | 119 | 0 |
| `docs/notes.d/README.md` | 1 | 0 |
| `internal/posse/hookskip.go` | 144 | 15 |
| `internal/posse/hookskip_qa_test.go` | 87 | 4 |
| | **351** | **19** |

Every one of the 19 has its named successor on main, read there rather than
taken on report: `posse_hs_readverb "$@"` (:501), `"remote": {"", "get-url",
"show"}` (:257), `posse_hs_ng=$((posse_hs_all - $#)); posse_hs_verb=$1; shift`
(:502), both `coverwhy` strings with `, or defines an alias` (:482-483), and
`renderHookSkipArm(..., hookSkipGlobalRef{})` in the test (:452, :460).
`hookSkipReadVerbs`, the deferred `-c` refusal, `posse_hs_cover`/`coverwhy` and
the `hookskip:config-override` reason are all on main verbatim.

Also checked: `fc72606f` and `2f1d59aa` each carry their work's `replayKey`
byte-identical and are each the unique such commit; `posse/dinesh-posse-ranger-base-e09l0`
is gone; `refs/posse/merge-blocked/**` is now EMPTY across the whole repo while
ap4p6 and ava0x sit under `refs/posse/retired/`; ranger-base-8t93w and
ranger-base-onwi1 exist and carry the `make install` gap. By execution,
`go test -run 'TestQAHookSkip|TestQAPIDDenyAlias' ./internal/posse` is ok
19.98s, 20 top-level PASS, and the legs the fragment names are in the shipped
tables: ap4p6's four reads among `hsAllowed`, and exactly seven c1pqo `remote`
spellings (`-v prune`, `--verbose prune`, `-v -v prune`, `-vv prune`,
`--verb prune`, `-v update`, `-v -- prune`) among `hsRefused`.

One mapping for anyone chasing the close's shas: it names b18ce916 and
e7739dff, which are its BRANCH commits; main carries fecbdf2b and 48d431c1,
whose `ranger-base-hogeb.md` blobs are identical (`c22361f9`, `e041e30a`) and
whose subjects and author dates match. The launcher replayed them onto a moved
main; nothing of the content differs.

## 5. Mechanics worth keeping

- **A table parameter that no row varies proves nothing, and reads as if it
  did.** Pin 13's stub takes an rc per read including the `--name-only` one,
  and all four rows pass `0` for it. MEASURED by adding a probe row
  (`{0, 128, 1, 1, refused: false}`): it PASSES — a failed name-only read is
  ALLOWED. Correctly so (the read takes no `--type`, so git failing it fails
  the have listing too, which `:806` checks; and a failed one under a
  non-empty have listing is caught by the count at `:820`), but the pin's own
  header sentence says every read's rc is checked, and one of the four is not
  checked at all. Count the reads, then count the rows.
- **Ask of a derivation: what does it do with the file it was NOT written
  for?** Probe B (a fresh body renderer) is what separates "the class is
  derived" from "the omission was patched"; probe E (the derived half deleted)
  is what separates "the pin holds" from "the list happens to be right today".
  Both are cheap and neither was in the close.
- **A mutant that only over-refuses survives any pin whose legs all assert
  refusal.** That is the whole shape of the fifth conjunct above, and the
  general rule it gives: when a conjunct can only make a wall refuse MORE, its
  witness has to be an ALLOWED leg, so look for the pin's allowed legs and ask
  whether the mutant reaches any of them. Pin 11 has one, and it is spelled
  absolute — exactly the case `--type=path` cannot touch.

## RECORDED (no bead filed; ADR 0006 §6)

- `internal/posse/hookskipwitness_qa_test.go:72` · the pin-13 header sentence
  "EVERY read the compare makes has its rc checked" overstates by one read:
  the `--name-only` read has no rc check, by design and for a stated reason,
  and no row of the table fails it (the stub's `nameRC` dimension varies in no
  shipped row). The ADR's own pin 13 text enumerates the three reads correctly
  · escaped from ranger-base-y9i3o.
- `internal/treepins/wallrendersources_qa_test.go:71` · "The residual is a
  helper spliced into a wall whose name is not `render*`" names the missed
  file's own name; MEASURED (probe D) the break can be UPSTREAM, so a
  `render*`-named body renderer in its own file is missed when any
  non-`render*` intermediate sits between it and the headered file. Binds
  nothing today (census above: no such chain exists) · escaped from
  ranger-base-jk380.
