# ranger-base-h0hjx — verify 3 closes: e09l0, 32ll0, zpk8r

Batched verify (holden, 2026-11-06). Verdicts and the one escape.

## Verdicts

| close | closer | verdict |
|---|---|---|
| ranger-base-e09l0 | gwart | VERIFIED on its branch; **not on main** (ranger-base-jmkjb) |
| ranger-base-32ll0 | dinesh | deliverable VERIFIED; **ESCAPE** — see §3, filed as ranger-base-7czd6 |
| ranger-base-zpk8r | jian-yang | VERIFIED |

Everything below is MEASURED 2026-11-06, this box: darwin 25.6.0, git 2.50.1
(Apple Git-155), main 1f4da269, through real rendered seat shims
(`WrapWithGates`) on claude AND codex unless a line says otherwise.

## 1. ranger-base-e09l0 — the alias splice unquotes

`posse_hs_split` ports git's `split_cmdline` to POSIX sh. Verified at
160d6bd9 (a detached checkout; the commit is NOT on main):

- The shipped pin `TestQAHookSkipAliasSpliceUnquotes` runs and passes on
  claude and codex, with `TestQAHookSkipRefusedOnEveryRuntime` beside it.
- MUTATION: restoring the IFS splice reds all five legs the finding
  measured — each exits 0 with HEAD moved (`q1agmN`, `q1agmNdq`,
  `q1agmNoVer`, `q1agmHooks`, `q1agmAlias`) and the unclosed-quote leg
  dies at git's own fatal instead of `hookskip:alias`.
- INDEPENDENT DIFFERENTIAL, mine, not a replay of gwart's 15 values:
  `posse_hs_split` extracted verbatim against git's own split, over **35
  hostile alias values**, by a non-`!` alias to a `git-argdump` on PATH.
  Every byte of every word compared through `od -c`. **Zero divergences.**
  Covered: vertical tab (0x0b) and form feed (0x0c) — git does NOT split
  on either, and neither does the splitter, so the narrower whitespace
  class is not a hole; CR, LF, LF inside double quotes, 0x0b inside
  quotes; a high UTF-8 byte; empty quoted words; adjacent quotes;
  backslash in/outside each quote style; `\\`; a backslash before a
  quote; leading, trailing and mixed runs of whitespace; a single quote
  inside double quotes and the reverse; `$HOME`, backticks, `$(...)`,
  globs, `~`, `;`, `|`, `&&`; unclosed single and double quotes; a
  trailing backslash, bare and inside double quotes.
- SAFETY of the `eval`: a leg spelling `$(touch CANARY)` and
  `` `touch CANARY` `` passed both through as literal words and the
  canary file was never created. Nothing the splitter emits can run,
  glob or re-split.

Finding 2 (`remote -v prune` read-classed) was correctly handed off, not
dropped: ranger-base-jvwcb → ranger-base-c1pqo (open, blocks
ranger-base-hogeb) → ranger-base-ks6a2 (verify). The code is still only at
72f5fa69.

**The limit on this verification:** 160d6bd9 is not on main, so main still
carries the IFS splice AND the false RESIDUALS sentence the finding was
about ("it splits on IFS and does not unquote, which can only refuse
more", hookskip.go:101-102). That is ranger-base-jmkjb's merge-back block,
P1 and open — a known tracked state, not a new escape.

## 2. ranger-base-zpk8r — all three findings

| arm | mutation | result |
|---|---|---|
| 1 call site | the bead's own repro at herdrback.go:2879 (`_ = d`) | RED, names `PolicyPinDegraded` and its position |
| 1 call site | the same at :2874 and :2884 (both sibling `warn()` lines) | RED each, naming its own reading |
| 1 call site | the guard deleted outright | RED, "has NO guarded call site" |
| 2 wording | `if want == gitSystemScopeFallback` → `if false` | RED on the fallback subtest |
| 3 null | the explicit `null` arm → `if false` | RED on the `null` subtest |
| 3 null | `%q` restored over a raw token | RED on all four raw subtests |

The census is not vacuous: it carries a floor (`len(class) < 3` →
`t.Fatalf`) and a per-member named check, so a class that empties out reds
rather than passing. Its door is real — `make pkgcensus-check` names
`TestQAEveryZeroArgDegradedLaunchReadingIsPrintedAtItsCallSite` and both
it and `make register-check` (89.8s) are green.

Checked and clean, not a finding: `want == gitSystemScopeFallback` is a
sound proxy for "resolution failed", because `gitSystemScopeEnv` sheds
every `GIT_CONFIG*` before asking, so `git var` cannot echo `/dev/null`
back from the env. A JSON string `"null"` does not collide with the token
arm (`raw` still carries its quotes).

## 3. ranger-base-32ll0 — pins 1-7 hold, and ONE ESCAPE

The deliverable is there. Pins 1-7 and
`TestQAHookSkipMovedHomeFollowsNoTildeInclude` all run and pass on claude
and codex, and each is load-bearing:

| mutation | result |
|---|---|
| the D3 gate's HOME line dropped | RED: pins 1/3/4/6, the exec witness, the tilde pin |
| the want reads moved back to the call's HOME (D2 as first written) | RED: the tilde pin only — the exact divergence dinesh declared |
| D3 as first written (a pinned `GIT_CONFIG_GLOBAL` skips the compare) | RED: the tilde pin only — the other declared divergence |
| `have` read with `git config --global` (ADR 0079 Context 3) | RED: pin 5 and the pin-1 group |
| D2's newline count dropped | nothing shipped reds (ADR 0079 pin 9 is owed by ranger-base-ava0x, correctly) |

That last row is worth a number. With the count dropped, a local alias
value forging the launcher's `global<TAB>core.hookspath …` line plus
`GIT_CONFIG_GLOBAL=/dev/null` **lands at rc 0 with the managed hook fired
0**; with the shipped count it is refused. So the CODE for pin 9 works and
only the pin is missing — which is what ADR 0079 says, and ava0x already
blocks on 32ll0.

### The escape: a `~`-spelled `core.hooksPath` is compared raw

Filed as **ranger-base-7czd6** (`-l code -l debt -l security`, P2,
`discovered-from: ranger-base-h0hjx`).

`internal/posse/hookskip.go:577-612` (`renderHookSkipGlobalScope`).
ADR 0079 D2 compares the TEXT of `git config --get-regexp`, which prints a
value raw. git expands `~` in `core.hooksPath` later, from the HOME of the
process that runs the hook. So when the launcher's global file spells the
managed hooks dir HOME-relative, both sides of the compare are
byte-identical and the directory is not:

```
launcher ~/.gitconfig:  core.hooksPath = ~/managed-hooks
have (HOME=evil):       global<TAB>core.hookspath ~/managed-hooks
want (HOME=launcher):   global<TAB>core.hookspath ~/managed-hooks   → equal → ALLOWED
what git then runs:     $evil/managed-hooks/pre-commit              → the seat's own
```

MEASURED through a rendered claude seat shim: **rc 0, the managed
pre-commit fired 0, the seat's own pre-commit ran 1, no refusal printed.**
Controls in the same run: the plain commit through the same tilde config
fired the managed hook once, and a HOME whose config drops the key was
refused under `hookskip:config-env`.

This is the sibling of the case 32ll0 DID close. dinesh found that
`include.path = ~/x` expands from the call's HOME and fixed it by running
the want reads under the launcher's HOME — which works because git
resolves an include AT READ TIME, so the two reads disagree. A
`core.hooksPath` value is resolved by neither read, so the same class
walks through the same compare.

Reachability is the same class as the include case and is why this is P2:
the launcher's global file has to carry the spelling. posse writes no
global `core.hooksPath` at all, and all five `git config --global
core.hooksPath` spellings tried are refused under `hookskip:config-write`
(pinned as the boundary). It needs a pre-existing operator or
employer-managed config — `~/.company/hooks` is an ordinary way to write
that line — which is the shape ADR 0052 exists for.

Pinned GREEN asserting the hole, with the inversion in its failure
message: `internal/posse/verify_h0hjx_qa_test.go`,
`TestQAHookSkipTildeHooksPathIsJudgedByRawText`. Mutation-checked both
ways — it reds with `INVERT ME` under the candidate fix below, and its
CONTROL 2 reds if the arm's HOME gate is removed.

**A candidate route, MEASURED, and not a prescription** — the fix is the
code lane's: reading both sides with `--type=path` makes git expand the
`~` per read, so the sides differ and the arm refuses. Trialled on top of
1f4da269: both tilde legs flip to REFUSED with the seat's hook not run,
and ADR 0079 pins 1-7, the tilde-include pin and
`TestQAHookSkipRefusedOnEveryRuntime` all stayed green. `--type=path`
leaves a value with no `~` untouched, so the `alias.*` half of
`hookSkipRegexp` reads unchanged (also MEASURED). The full suite was not
run under the trial.

Note that the same raw-text blindness would also cover a HOME-relative
value reached through SYSTEM scope. I did not get a clean measurement of
that — my fixture's own control refused, because a typed
`GIT_CONFIG_SYSTEM` moves the separate system arm — so it is stated as
unmeasured, not as a finding.

## Also checked, clean

- Two global files both carrying hook-relevant keys, nothing moved, and
  again with HOME at a byte-identical two-file tree (an `[include]` in the
  XDG file): both ALLOWED and the hook fired, so `have`/`want` ordering is
  stable across candidates. No over-refusal.
- `GIT_CONFIG_COUNT` / `GIT_CONFIG_KEY_n` and `GIT_CONFIG_PARAMETERS`
  naming a hook key are refused by the env arm (pre-existing,
  ranger-base-r3pri), so the new global arm's scope — HOME,
  XDG_CONFIG_HOME, GIT_CONFIG_GLOBAL — is the right one.
- The D2 line count cannot be balanced past: every entry contributes at
  least one non-empty line to the `--show-scope` listing and exactly one
  to `--name-only`, and a forged `global<TAB>` line is itself non-empty,
  so `gl == gn` cannot hold while a line is forged. The `IFS`-newline
  split collapses empty lines, which only ever counts fewer.
- On THIS box the operator's own global config carries no hook-relevant
  key at all, so the baked reference is empty and every HOME move compares
  empty-to-empty and is allowed — correct per ADR 0079 D4, since this
  box's git reads no global hook either. The escape above needs a box
  whose global file carries the key.
