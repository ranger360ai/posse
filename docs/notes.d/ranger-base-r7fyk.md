# Verifying 4 closes: 5x8xi, obaim, c2k6z, jep08 (ranger-base-r7fyk)

MEASURED 2026-11-07, darwin 25.6.0 arm64, go1.26.10, git 2.50.1 (Apple
Git-155), from the session worktree `posse/holden-posse-ranger-base-r7fyk` at
HEAD **e6a8b198** (== `main`), tree clean before and after every mutant.

**VERDICT: all four closes VERIFY.** 16 mutants over the four, every one
killed, each naming its own arm. Two LIVE findings came out of the
verification itself, both adjacent to ranger-base-5x8xi's finding 2 rather
than inside it; they are filed, and the live-defect pins for them are in
`internal/posse/verify_r7fyk_qa_test.go`.

## Method

Mutants were applied one at a time by a driver that md5s the file before and
after (a no-op edit scores as a FAILED mutant, never as a surviving pin),
runs the target filter and a must-stay-green filter, then reverts with
`git checkout --` and re-asserts the md5. Every mutant was aimed at
`grep -n` from HEAD, never at the closing diff.

Two of my own first mutants were mis-aimed and are recorded because the
failure modes are the ones this shop keeps paying for:

- `probeAbsent` → `return err != nil` **does not build** (`errors`, `fs` and
  `syscall` go unused), so the first reading measured the compiler. The
  honest spelling keeps the imports used:
  `return err != nil || errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)`.
- deleting `claudeOutsideReadProbe`'s `!probeAbsent` guard is **not** the
  "restore the pre-fix line" mutant the close describes — it leaves absence
  answering the reading, so `TestOutsideReadProbeReadsAndDoesNotWrite` stays
  green. Restoring the pre-fix `Silence{Unknown: true, Why: "unreadable …"}`
  verbatim is what reds it, and it does.

## ranger-base-5x8xi — VERIFY

Five mutants, all killed, matching the close's own table row for row.

| mutant | target | result |
|---|---|---|
| `inv.Cwd != cwd` disabled (`if false && …`) | `TestQACodexAskVetsTheDirectoryItsAnswerIsAbout` | 3 refusal arms red, **both controls green** |
| `!strings.HasPrefix(cwd, inv.Cwd)` for `!=` | same | the empty-cwd arm **alone** red |
| ask route back to `len(args) >= 2` | `TestHookReportAskArmRefusesAnUnpairedConfigValue` | red |
| `runCodexHooksAsk`'s `len(args)==0` deleted | same | red (the panic path) |
| `':'` admitted in `plainStatusToken` | `TestHookReportRefusesAnAppNameThatIsNotOne` | red, **and** `TestCodexHooksAskRefusesBeforeStartingAnythingItCannotAsk/an app name that is not a name` — the `codex:evil` row the bead itself named as the byte class's other pin |

The byte compare is sound for a reason that does not rest on the SPIKE
alone: `codexAskOver` sends `"cwds": []string{cwd}` at codextrust.go:624 and
compares that same `cwd` at :675, so the value compared **is** the value
sent. Normalization upstream (`resolveLinks`) cannot desynchronize them.
`codexAskOver` has one caller (codextrust.go:528), so there is no second
decode path that renders a trust line without the compare.

## ranger-base-obaim — VERIFY

Production code byte-identical to HEAD, as the close claims. Four mutants:

| mutant | red | green |
|---|---|---|
| leg-error sink moved into `case "working":` before `d.sayWorking(p)` | both new arms, both occupants | the 3 pre-existing `TestQAWaitLegError*` pins |
| dispatch.go:5118 order inverted | `…InTheDispatchLoop` only | `…InPlanLaunch` and every pre-existing grok pin |
| herdrback.go:2632 order inverted | `…InPlanLaunch`, both arms | `…InTheDispatchLoop` and every pre-existing grok pin |
| sparing window widened to the whole buffer | `one spared pass does not blind the reach guard`, **15.01s** | the other five arms in the file |

The 15.01s reproduces the close's 15.006s and the bead's 15.00s. The two
order mutants are independent in both directions, which is the claim.

The bead asked for pins on the `blocked` and unnamed-state arms and got
them; the `WaitCeiling` arm it also enumerates is still unpinned on its own,
but it sits inside `case "working":` **above** `d.sayWorking(p)`, so no move
of the sink into that switch can survive the two arms that did land. The
placement is held.

## ranger-base-c2k6z — VERIFY

Five mutants, all killed, each naming the right probe — the pin asserts
per-probe rather than collapsing both into one verdict:

| mutant | red |
|---|---|
| pre-fix `unreadable` line restored in **both** probes | 12 assertions across both probes, **plus** `TestOutsideReadProbeReadsAndDoesNotWrite` |
| …in `claudeTrustProbe` only | 6 assertions, `claudeTrustProbe` only; the outside-read test stays green |
| …in `claudeOutsideReadProbe` only | 6 assertions, `claudeOutsideReadProbe` only, plus the outside-read test |
| `probeAbsent` → every errno is absence | the pin's own fixture guard (`which probeAbsent reads as absence`), and m928w's `TestQAUnreadableHomeReadsUnknownAndNotAbsent` the same way |

The "these two were the last probes whose unknown carried no errno" claim
holds by census: every remaining `Silence{Unknown: true}` in internal/posse
is either not a read error (`unparseable`, `no cwd`, `no latest_version`) or
already classifies with `probeAbsent` (interstitial.go:389, m928w's work).

ONE PROPERTY WORTH KNOWING, and it is the fleet rule rather than this
close's: `probeAbsent` counts ENOTDIR as absence, so a `CLAUDE_CONFIG_DIR`
that names a regular file makes the probe print "`~/.claude.json` does not
exist … the launch creates it" over a path `MkdirAll` cannot create. The
bead PRESCRIBED `probeAbsent` by name, every probe m928w touched shares the
behaviour, and the launch still refuses loudly (`SeedClaudeTrust` dies on
the failed `MkdirAll`) — so this is recorded here and not filed.

## ranger-base-jep08 — VERIFY, with one stated limit

**The limit: I cannot re-measure the git 2.56.0 host.** No access from this
seat, and no 2.56 on this box. What is verifiable here is verified, and
nothing contradicts the host readings.

- **The two laptop readings reproduce exactly**, with the filters the close's
  own notes fragment names: arm 1 `30 pass, 1 skip`; arm 3 `19 pass, 1 skip`.
  Both skips are the two version-gated config-hook pins, so the host's
  `31 pass, 0 skip` / `20 pass, 0 skip` is exactly those two flipping — the
  arithmetic is internally consistent with the only thing 2.56 adds.
- `gitReadsConfigBasedHooks` gates on `>= 2.54` correctly for every spelling
  git prints, and the skip names its reason
  (`git version 2.50.1 (Apple Git-155) has no config-based hooks (needs 2.54)`).
  It is a version gate, not a parked skip.
- **The four new `hsRefused` legs are shown able to fail on this box**, which
  matters because their subject (`hook.<name>.enabled`) is inert in git
  2.50.1: they measure posse's own refusal, which is version-independent.
  Narrowing the key arm from `ciGlob("hook.") + "*"` to `+ "*.command"` reds
  all four, each naming its own rule (`hookskip:config-override` ×2,
  `hookskip:config-env`, `hookskip:config-write`), plus the refusals.log
  count (84 against 89) — so the legs are carried by the `hook.` prefix the
  close says carries them, and not by a spelling list.
- The probe script's header carries the RUN-IT-OUTSIDE-A-SEAT rule and says
  why (every row sets `GIT_CONFIG_SYSTEM`, which the hook-skip arm refuses —
  the arm working, not the probe failing), and its EXPECTs name their venue.

## The two live findings

Both are adjacent to ranger-base-5x8xi's finding 2 — the same arm, the same
"exit 0 over an empty stdout is a launch script's green light" argument —
and neither was in that bead's ask, so neither is a failure of the close.
Filed as one findings bead; pinned green in
`internal/posse/verify_r7fyk_qa_test.go`.

### 1 · the `--declare` arm does not own its arity

MEASURED through `runHookReport`:

| argv | exit | stdout | arm reached |
|---|---|---|---|
| `hook-report --ask` | 1 | `""` | the ask arm's usage (5x8xi's fix) |
| `hook-report --declare` | **0** | **`""`** | the SHARED wrong-argv arm |
| `hook-report --declare codex extra` | **0** | **`""`** | the SHARED wrong-argv arm |
| `hook-report --declare codex` | 0 | a declaration | the declare arm |
| `hook-report --declare codex:evil` | 1 | `""` | the declare arm's own refusal |

`--declare` is routed on `len(args) == 2` (codexreport.go:378), so both the
short and the long argv fall past it. The reader that is fooled is shipped
and is not a hand run: `docs/adr/0076-trust-ask.probe.sh` lines 122 and 171
are `DECL=$("$POSSE" hook-report --declare codex) || exit 1`, which on an
exit 0 leaves `DECL` empty and carries on.

The fix is not one line. `TestHookReportNeverExitsNonZeroOnTheHookPath`
(codexreport_test.go:204) **asserts this hole** — its table carries
`{"posse", HookReportVerb, HookReportDeclareFlag}` at exit 0 — and its stated
justification ("a malformed handler line must not be able to block a
prompt") is stricter than the rule it names: codex blocks on exit **2**, so
an exit 1 on a non-hook argv blocks nothing. MEASURED: planting the arity
fix reds that row.

### 2 · a declaration whose own reporter refuses it

A leading dash is a legal `plainStatusToken` (`'-'` is in its character set,
codexreport.go:193), so `-c`, `-`, `--declare` and `--ask` are app names the
declare arm and D7's ask arm both accept, and `CodexHooksDeclaration` renders
`posse hook-report <app>` for all eight events. `runHookReport`'s shared arm
is `len(args) != 1 || strings.HasPrefix(args[0], "-")`, so the reporter
refuses the command the declaration just installed. MEASURED, each row the
declaration rendering and then the command it installs being run:

| app | declaration renders | tty opens | bytes written | exit |
|---|---|---|---|---|
| `codex` (control) | yes | 1 | 29 | 0 |
| `-c` | yes | **0** | **0** | 0 |
| `-` | yes | **0** | **0** | 0 |
| `--declare` | yes | **0** | **0** | 0 |
| `--ask` | yes | **0** | **0** | **1** |

The first three install a reporter that is silent on every event forever.
The `--ask` row is **new as of ranger-base-5x8xi**: before the arity change
that argv landed in the shared arm at exit 0 like the others, and it now
reaches `runCodexHooksAsk`'s usage and exits 1 — against this file's own
rule, pinned by `TestHookReportNeverExitsNonZeroOnTheHookPath`, that no arm
on the hook path returns anything but 0.

The token rule is checked on the declaration side and the dash guard lives
only in the reporter, so the two disagree about which app names are usable.
Reachable today by a hand run, a hand-written launch script, or a
`runtimes/<name>.yaml` whose name starts with a dash — the cost class
ranger-base-5x8xi accepted for its own finding 2.
