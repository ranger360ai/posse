## ADR adherence audit, September 2026 (ranger-base-4wxko)

Review 2 of 2. Review 1 (ADR 0040, ranger-base-0tr1e) reduced 39 records
to a disposition table with one *survivor* column: the rule the code is
supposed to realize, and the record it will live in. The operator accepted
that table as proposed on ranger-base-ay3dr (2026-09-01). This note audits
the code against the survivor column. It changes no code and no record; it
cuts beads.

**Measured at** posse `088ddeb` (2026-09-01), package `internal/posse/`
and `cmd/posse/`, plus the live constitution the launcher reads (the PID
set, `config.yaml`, `runtimes/`, the installed L3 hooks). Eight reader
passes, one per ADR 0040 §2 concern group, each opening every cited site
and locating each pin by name. **No test was executed** in this audit (the
package runs ~15 minutes); every "pinned" cell means a test exists whose
body asserts the behaviour, not that it was green today. The raw reader
tables (one row per rule clause, with instance facts quoted) are in the
private tree: `ranger-base/docs/audits/adr-adherence-2026-09-tables.md`.

**Verdicts** (ADR 0023's rule: a rule counts only when identity AND
behaviour hold; no test means "adheres, unpinned"):

| verdict | meaning |
|---|---|
| ADHERES | code realizes the rule; a named test pins it |
| ADHERES-UNPINNED | code realizes it; nothing asserts it |
| DRIFTED | code does what the record forbids or omits what it requires. CODE-RIGHT = the record is stale (amend). ADR-RIGHT = a code bug |
| UNREALIZED | the record decides it, no code does it; `bead:` names the build bead, else "silently dropped" |
| UNGOVERNED | a load-bearing rule the code has and no record states (candidate ADR text, not a bug) |

**Class** is ADR 0025's fix lane: *wall* (a mechanism refuses: shim,
seatbelt, hook, bind mount, a Go refusal), *guard* (code checks, degrades,
reports), *convention* (prose: a PID, AGENTS.md, a comment).

Reader-row totals over 375 rule clauses: ADHERES 277 · ADHERES-UNPINNED 47
· DRIFTED 16 (5 ADR-right, 11 code-right) · UNREALIZED 28 (25 with a build
bead, 3 silently dropped) · UNGOVERNED candidates 18.

### 1. The checklist

One row per survivor clause (ADR 0040 §1, fourth column). Where a reader
split a clause further, the worst sub-verdict is shown and the appendix
carries the rest. Paths are `internal/posse/` unless prefixed.

#### Runtime and gates → ADR 0042 The wall (0002, 0009, 0023, 0025, 0035)

| rule | class | verdict | evidence |
|---|---|---|---|
| 0002 layers L0–L4, each carried into the cage (L1 shim, L2 seatbelt, L3 hooks, L4 engine + egress proxy, inner re-render) | wall | ADHERES | `gates.go:952,1272`, `seatbelt.go:204-262`, `cage.go:119-156,339-366`, `cageinner.go:16,88`; `TestRenderedShimRefusesAndPasses`, `TestQALiveCageMountBoundaryIsDeepAndTheNotebookSurvives`, `TestEgressProxyRefusesUnknownHostsAndLogsThemLikeL1` |
| 0002 §3 L3: push hook, shared-index arm keyed on no env, constitution arm above it, foreign hook never overwritten | wall | ADHERES | `gates.go:1380-1402,2023-2170,2623,1652-1730`; `TestPrePushHookExitsForReal`, `TestSharedIndexCommitHook`, `TestQAConstitutionWallSitsAboveTheSharedIndexArm`, `TestForeignHookRefusalPrescribesTheChain` |
| 0002 §3 L1: relaunch re-types the same wrapped line | guard | ADHERES-UNPINNED | `herdrback.go:1866`; only the cage shape is pinned (`TestCagedRelaunchRetypesTheLauncher`) |
| 0002 §4 parity or refuse; `--allow-degraded` marked in meta, list, cockpit; never at tier fast | guard | ADHERES | `parity.go:132-380,486,618-627`, `herdrback.go:1525-1539`; `TestDispatchRefusesDegradedUnlessAllowed`, `TestFastNeedsFullParityAndTierFloor` |
| 0002 §5 PID keys runtime/cage/writable/egress/sockets/trust_project_config | guard | ADHERES | `agents.go:177-198` |
| 0002 project-config trust: list per runtime, keyed for claude, whole-file for codex, fails closed, re-checked on relaunch | guard | ADHERES | `parity.go:396-486`; `TestClaudeProjectConfigTrustIsKeyedAndFailsClosed`, `TestQAProjectConfigTrustIsRecheckedOnRelaunch` |
| 0002 escape C measured (`--no-verify`, `core.hooksPath`) | guard | ADHERES | `cageinnerliveqa_test.go:321-370`, `TestL3ProbeFollowsCoreHooksPath` |
| 0009 gate shell per persona; REAL outside every gates dir, wrapper-as-REAL refused; PATH rebuilt; `SHELL=` on the typed line | wall | ADHERES | `gates.go:1161-1213,1220-1268,1065-1112,1272-1295`; `TestGateShellNeverChainsToAnotherWrapper`, `TestGateShellGuardsTheReplayedPATH`, `TestGateShellArgvWalk` |
| 0009 §4 rc files run gated | wall | ADHERES-UNPINNED | the guard prepends before any rc capture; no test runs a denied verb from an rc |
| 0023 L3 realized iff identity AND behaviour of our own render; launcher never execs the on-disk hook; foreign degrades named | guard | ADHERES | `gates.go:2860-2960,3026-3032`; `TestL3HookProbeIdentityNotMarkersOrForeignBehavior`, `TestL3ProbeNeverExecsForeignBytes`, `TestL3ProbeDegradesAForeignRefuser` |
| 0025 every realized gate is `enforced` or `cooperative`; `posse gates` prints it; never refuses a launch | guard | ADHERES | `parity.go:70-108`, `cmd/posse/main.go:1211`; `TestParityStringOrdersTheRealizedLines` |
| 0025 §1 the session meta prints the class | guard | UNREALIZED — silently dropped | `herdrback.go:372` writes `degraded:` only; 0040 said "carry as unbuilt or strike" and neither happened |
| 0025 §3 push-effect note at the container tier | guard | ADHERES-UNPINNED | `parity.go:548-571`; no test reads `.Effect` |
| 0025 §4 one writer for refusals: spool + fold with cursor and tamper line | guard | ADHERES | `refusalfold.go:53-170`, `cage.go:408-418`; `TestFoldDetectsATruncateAndRefillToTheSameSizeByHash` |
| 0035 §1 no posse-written file in a foreign config home | convention | ADHERES-UNPINNED | grep: no write under `.codex/` or `.grok/` in non-test Go |
| 0035 §2 codex line carries the second spelling of never-ask | wall (L0) | UNREALIZED — bead: ranger-base-6tj5r (ruled BUILD) | `runtime.go:983,1171` carry `-a never` only; ranger-base-hxez was the same edit filed twice, closed onto 6tj5r by this audit |
| 0035 §3 grok's compensating control: pane mode surfaced in list/gates | guard | UNREALIZED — silently dropped | only reader is a test helper (`permissionmodepane_qa_test.go:195`); `posse list` has no mode column; ranger-base-0emp is closed |

#### What a runtime is → ADR 0043 (0017, 0021, 0032, 0037)

| rule | class | verdict | evidence |
|---|---|---|---|
| 0017 the checklist is the struct plus the grid; every field classified in a pinned test | guard | ADHERES | `runtimecheck.go:79-100`, `runtimefields_qa_test.go:93` `TestEveryRuntimeFieldIsClassified` |
| 0017 PARITY / DECLARED DIFFERENCE / UNKNOWN; UNKNOWN never fatal | guard | ADHERES | `parity.go:117-125,722`; `TestBuiltinDimensionRowsSpeakTheVerdictVocabulary` |
| 0017 §3 shadow-predicate rule (name-keyed behaviour only for CLI-own state) | convention | ADHERES-UNPINNED | five accepted sites remain; no census test would catch a sixth |
| 0017 §4 declarability list is `runtimeYamlKeys()`; unknown key warns; registry key refuses | guard | ADHERES | `runtimeyaml.go:130-147,169`, `runtime.go:1403-1408` |
| 0021 D1 a yaml naming a built-in is a per-key overlay over the listed keys | guard | ADHERES (the nine) / **DRIFTED, ADR-RIGHT on principle** (the rest) | `runtime.go:1461,1478` applies nine keys; `runtimeyaml.go:169` treats the other thirteen declarable keys as known, so an overlay declaring `rules_precedence:`, `state_dir:`, `env_required:`, `turn_outcome:`, `unattended:`, `self_sandbox:`, `project_config:`, `skills_cwd:` loads clean and changes nothing. The live overlay is in service today. See finding 4 |
| 0021 D2 `command:` / `skills_flag:` on a built-in refuse | wall | ADHERES | `runtime.go:1490-1497`; `TestOverlayRefusesMechanismKeys` |
| 0021 D3 declared-by names the source per key | guard | ADHERES | `runtime.go:1498`, `runtimecheck.go:79-100` |
| 0032 template denies are Degraded until a recorded probe; four observables; not waivable at fast | guard | ADHERES | `parity.go:216,659,702`, `runtimeprobe.go:361`; `TestTemplateBashDenyIsAssumedUntilProbed`, `TestAssumedProbeIsNotWaivableAtTierFast` (open hole: ranger-base-385x, record names the wrong binary) |
| 0032 §3 API-only never satisfies the contract | convention | ADHERES-UNPINNED | no in-process agent kind exists |
| 0037 dimensions are harness material, facts are instance material | convention | ADHERES-UNPINNED | no vendor name in non-test Go; the private tree carries the one venue yaml |
| 0037 Claims: two keys "absent" from `runtimeYamlKeys()` | — | DRIFTED, CODE-RIGHT | `runtimeyaml.go:136-137` both present; already an 0040 amend row |

#### Path-scoped writes, single writer, git identity (0014, 0022, 0038)

| rule | class | verdict | evidence |
|---|---|---|---|
| 0014 `Edit(<glob>)` is a subtree write deny; grammar; `agent check` lints | guard | ADHERES | `cageinner.go:387`, `pidcheck.go:211-227`; `TestPathScopedWriteGrammar`, `TestCheckAgentNamesPathScopedWriteMistakes` |
| 0014 matrix per tier; L2 trailing deny; L4 `:ro` overlays; `writable:` at both tiers | wall | ADHERES | `parity.go:278-312`, `seatbelt.go:204-235`, `cage.go:338-365`; `TestQAPathScopedDenyRefusesUnderSandboxExecAndTheControlDoesNot`, `TestDenyListShapeOverlaysTheSubtreeReadOnly` |
| 0014 §5 a hook is not a cage | convention | ADHERES-UNPINNED | no `PreToolUse` matcher rendered |
| 0022 a path-limited commit narrows by file never by writer (AGENTS.md, work prompt, PIDs) | convention | ADHERES | AGENTS.md; the work prompt's own-worktree block; 4 of 11 live PIDs carry the sentence, the rest inherit AGENTS.md |
| 0022 NOTES.md fragments in `docs/notes.d/` | convention | ADHERES | 87 fragments |
| 0022 §3 the NOTES arm of `prepare-commit-msg`, shared-checkout only, unkeyed, above the next-index exemption | wall | ADHERES | `gates.go:2082-2119`; `notesguard_qa_test.go:74-176` (five arms) |
| 0022 §4 the `--` requirement and the private-index refusal stay; the seed answered: single-writer and the shared-index wall are two arms of one hook, no conflict | wall | ADHERES | `TestSharedIndexCommitHook`, `TestSharedIndexCommitHookRefusesHandRolledNextIndex`; 11 of 11 live PIDs deny `Bash(git commit unless --)` |
| 0038 D1/D2 L2 denies the persistent git identity (config, config.lock, the worktree chain) | wall | UNREALIZED — beads: ranger-base-vqyxl, ranger-base-65po1 | `seatbelt.go:572-660` has no config entry |
| 0038 D4 L4 `:ro` binds of common config and hooks | wall | UNREALIZED — beads: ranger-base-mugt2, ranger-base-t4f1 | `cage.go:397-398` mounts the common dir rw whole, stated |
| 0038 D3 transient `-c core.hooksPath=` stays cooperative | convention | ADHERES-UNPINNED | nothing cites 0038 as closed |

#### The pass → ADR 0044 (0011, 0020, 0028) and 0008 + 0030, 0033

| rule | class | verdict | evidence |
|---|---|---|---|
| 0011 bd is the queue; claims are leases without expiry | guard | ADHERES | `dispatch.go:1841-1867,3455-3470`; `TestDispatchWaitTimeoutWhileWorkingKeepsClaim` |
| 0011 one launcher per home: flock, holders (fire loop, LaunchBead, verify-after, and since then CreateSession, prune unlink, relaunch) | wall | ADHERES | `launchlock.go:114-143`, `herdrback.go:1844,1024`, `relaunch.go:159,364,419`; `TestTwoPassesDoNotDoubleClaimOneBead`, `TestCreateSessionHoldsTheLaunchLock` (0011 App.A "outside the lock: bead filed" is stale, w4h5 landed) |
| 0011 prune proves death: grace AND identity; not-mine never deletes; unlink re-proves inside the lock | guard | ADHERES | `herdrback.go:780,907-1047`; `TestPruneGraceKeepsAFreshMetaEvenWhenHerdrSaysGone`, `TestPruneDoesNotUnlinkAMetaACreateRewroteUnderIt` |
| 0011 the session meta is the run record; holder join is a lookup | guard | ADHERES | `herdrback.go:389-411,2141`, `dispatch.go:906-920`; `TestHolderJoinReadsTheRecordNotTheName` |
| 0011 grace exemptions as a set | guard | ADHERES-UNPINNED | `dispatch.go:2335-2341`; exercised singly, never as a set |
| 0011 foreign row fails closed at both launchers | guard | ADHERES | `dispatch.go:3167,2216,3602`; `TestQALaunchBeadRefusesAForeignHolderAboveTheStatusCheck` (a Consequences aside, not a decision line, see ungoverned) |
| 0011 `dispatch.go` header narrative | convention | DRIFTED, CODE-RIGHT | `dispatch.go:13-23` still narrates the pre-Dial-F loop and a gather barrier |
| 0020 lane = label set, seat = persona; assignee never falls through; availability-first | guard | ADHERES | `dispatch.go:1097-1148,1259-1287`; `TestQARouteOrderDoesNotTouchAnAssignedBead`, `TestQASeatSelectionMissingSoAFreeSeatIsNeverOffered` |
| 0020 §2 tiebreak is persona-name order | guard | DRIFTED, CODE-RIGHT | `dispatch.go:1136-1148` orders by `route_order:` then name; `TestQARouteOrderTieBreaksOnPersonaName`; 0040 already routes `route_order` to 0044 |
| 0020 §3 verify-after unassigned by default | guard | ADHERES | `verifyafter.go:93,198`; the "live config pins laurie" sentence is history |
| 0020 §4 one serial seat per persona | guard | ADHERES | `dispatch.go:1223,3050-3096`; `TestQASeatThisRunFiredIntoStaysHeldAcrossFirePasses` |
| 0020 §5 width law per epoch | convention + guard | ADHERES-UNPINNED | `epoch.go:151-164` counts attempts; the min() formula is prose; `TestLaunchCapIsSpentPerEpochNotPerPass` pins the epoch half |
| 0020 §6 batched verify fan-in with age | guard | ADHERES | `verifyafter.go:71,85,450-466`; `TestVerifyBatchHoldsAPartialBatchUntilItFills` ("N stays 1" sentence stale) |
| 0028 §1 long-lived Run; settle re-runs the fire path under the flock; reap rides the settle | guard | ADHERES | `watch.go:90`, `dispatch.go:1970-2020,2530-2575,1985`; `TestRunRefillsAFreedSeatInsideOnePass`, `TestQAReapSweepsAtEverySettleUnderARollingRun` |
| 0028 §2 the epoch denominates `budget_pass` and `-n`; spend survives a restart; the loop names the denomination | guard | ADHERES | `epoch.go:52,89-104,136-149`; `TestEpochSpendSurvivesARunRestart`, `TestQAWatchNamesTheLaunchCapsDenomination` |
| 0028 §2 the `-n` attempt count survives a restart | guard | UNGOVERNED | `epoch.go:31-40`: in-memory; a supervised restart restores the full ration mid-epoch. The record is silent |
| 0028 §3 brakes untouched; two clocks in the seat map | guard | ADHERES | `dispatch.go:1181-1229`; `TestQASeatBusyInOneFirePassIsReReadInTheNext` |
| 0028 §4 one throttle in the watch process | convention | ADHERES-UNPINNED | one setter of `Refill`, one caller of `refire`; observable 2 (loop killed, zero launches) has no test |
| 0028 §5 idle-to-next measured per seat, control arm named (the seed) | guard | ADHERES, target met at the median only | `seatidle.go:178-324`; MEASURED in the live watch log: treatment-arm medians of seconds to a few minutes, maxima of tens of minutes to hours, half the windows per Run refill-closed; the report prints when Run returns, so a rolling Run reports late |
| 0008 a crew session is invisible to dispatch; no twin; `--resume` no override; the mark set by origin and the operator's hands | guard | ADHERES | `dispatch.go:3063-3066,2151-2165`, `herdrback.go:483-560`; `TestDispatchSkipsCrewSession`, `TestCrewMarkerLifecycle` |
| 0030 orphaned claim parks on the assignee's crew session; ready beads still dispatch | guard | ADHERES | `dispatch.go:2189-2204,3578-3590`; `TestDispatchParksOrphanedClaimUnderTheTypedRoute`, `TestReadyBeadDispatchesDuringCrewChatUnderADR0030` |
| 0033 `coordinator:` in config; Route never returns it on any path; refusal at hire time, not filing time (the seed) | guard | ADHERES | `dispatch.go:1021-1032,1103-1116,1140-1142`; `TestRouteRefusesCoordinatorOnEveryPath`, `TestLaunchBeadRefusesCoordinator`. Today's refusal text is §2's sentence, fired from `laneFor`, shared by both launchers |
| 0033 §4 G9 | guard | ADHERES | `govern.go:570-586` (the "until that lands" clause is stale) |
| 0033 §5 push-grant drift alarm | guard | ADHERES | `pidcheck.go:254-268`; MEASURED live: one PID carries push in allow, ten deny it, alarm silent |
| 0033 the constitution cites the decision by its number | convention | DRIFTED, constitution stale | the live `config.yaml` comment on `coordinator:` cites "ADR 0018", which in this repo is the blind meter; 0033's preamble promised the re-point |

#### Contract and oversight (0013, 0004, 0016, 0027, 0029)

| rule | class | verdict | evidence |
|---|---|---|---|
| 0013 §1 six stages with declared-by; the grid is exactly the six plus five 0017 rows (the seed) | guard | ADHERES | `runtimecheck.go:115-163,257-456` |
| 0013 §1 record trust: settle without record never ✓; unattended resume re-prompts | guard | ADHERES | `recordskip_qa_test.go:36-143` |
| 0013 §1 settle: pane half is herdr Seen, turn half is the `turn_outcome:` registry | guard | ADHERES | `turnfailure.go`, `turnoutcome_qa_test.go:59-326` (the record's pin path is stale, see 3ni7p) |
| 0013 §1 account: `CostPriced()` is the predicate; UNCOUNTED vs UNPRICED; unset cap is loud | guard | ADHERES | `runtimecheck.go:393-456`, `uncounted.go`; `TestQAUnpricedKeepsTheBrakeAndPricedLosesIt` |
| 0013 §2 argv-first delivery, claim first, create-fails unclaims; no keystrokes | guard | ADHERES | `dispatch.go:3375-3399`; `TestArgvClaimsBeforeCreatingTheSession`, `TestDispatchPathPressesNoKeys` |
| 0013 §2 a built-in danger screen with an unsilenced probe refuses above the claim | wall | ADHERES | `interstitial.go:248-266`, `dispatch.go:3276-3281`; `TestQADangerousCodexInterstitialRefusesDispatchUntilSilenced` |
| 0013 §2 (vbp3 amendment, ratified) a DECLARED yaml `danger:` screen with no probe refuses too | wall | **DRIFTED, ADR-RIGHT** | `interstitial.go:254` still skips `Probe == nil`; the four vbp3 code commits are on the session branch only, the ratification doc landed. Bead open: ranger-base-tq93. See finding 2 |
| 0013 §2 busy key: session failure keeps the slot; second failure per pass benches | guard | ADHERES | `dispatch.go:2430-2462` |
| 0013 §4 reachability judged on the rendered profile; third answer inside a cage | guard | ADHERES | `reachability.go`; `reachability_qa_test.go:119-444` |
| 0013 §4 reap guard: open bead + dirty cwd is not killed | guard | ADHERES with a `--force` escape the record does not name | `reapguard.go:75-88`; `TestForceKillTakesTheDirtyOpenSession` |
| 0013 §4 harness never closes a bead for the persona | convention | ADHERES-UNPINNED | absence only |
| 0013 §4 a line that voids the PID channel is refused | wall | ADHERES | `herdrback.go:1581,1967`; `pidvoid_test.go` |
| 0013 §5 uncounted launches named per pass; cap counts beads | guard | ADHERES | `uncounted_test.go:136-326` |
| 0013 §6 unmapped tier shows `default` | guard | ADHERES | `runtimecheck.go:458-495`, `tierdisplay_test.go` |
| 0004 pure `render(w,h)` over a row model; three sections one cursor; claim-in-flight refusals; viewport and chrome shedding | guard | ADHERES | `cmd/posse/cockpit.go:2717,1158-1262,1630-1673,2497`; `TestCockpitGolden`, `TestCockpitTabCyclesThreeSections`, `TestCockpitRefusesASecondWriteWhileOneIsInFlight`, `TestCockpitShortTerminalShedsChrome` |
| 0016 events are hints, level-triggered passes are the truth; one subscription per process; `blocked` is not a settle | guard | ADHERES | `herdrevents.go:104-266`, `watch.go:146-208`; `TestHerdrSettleHintsFilter`, `TestWatchSettleHintWakesTheNextPassEarly` |
| 0016 §2 the pulse may consume the hint | guard | UNREALIZED — permissive "may", retire the sentence | `pulse.go` has no hint reader |
| 0027 §1 sensing | — | superseded by 0029 §1–2, as 0040 says | `govern.go:276-290,443-466` |
| 0027 §2–4 arm = presence of the interval key; idle-only delivery with the fixed prefix; renag backoff; default target `coordinator:`; sets no crew mark | guard | ADHERES | `pulse.go:60-98,254-290,399`; `TestPulseIdleOnlySkipsWorkingSession`, `TestPulseTargetsCrewSessionAndWritesNoCrewMark` |
| 0027 live state | — | the live instance is not armed; its config comment says "do not arm yet" because of a bead that closed 2026-08-31 with pins. See finding 8 |
| 0029 facts computed, decisions are beads; nine G-rows each a predicate over one store; unread store is partial not clear | guard | ADHERES | `govern.go:317-583`; `govern_test.go:161-881`, `governhonesty_qa_test.go` |
| 0029 §3 pause is a human speech act; every launcher declines; nothing auto-pauses | wall | ADHERES | `pause.go:57-86`, `dispatch.go:1754,3481`; `TestNoMechanismEverWritesThePauseFile` |
| 0029 §4 coordinator SLA and the `blocked-time-to-intervention` metric | convention | UNREALIZED — by its own condition (the pulse is not armed); the metric has no code | |

#### Money and meters (0003, → ADR 0045: 0010, 0018, 0034; 0039)

| rule | class | verdict | evidence |
|---|---|---|---|
| 0003 tier is a name; three tiers per runtime in a built-in table; overlay may set `model_<tier>:` | guard | ADHERES | `runtime.go:795,838,912`; `tierdisplay_test.go` |
| 0003 the claude strong cell tracks the dial (0039 D1) | guard | UNREALIZED — bead: ranger-base-per37 | `runtime.go:796` still names the previous strong id; `cost.go:48` has no exact price row; the live overlay is the only thing that launches the current id. See finding 1 |
| 0003 §2 precedence ladder | guard | ADHERES | `dispatch.go:1387-1410`; `TestBeadTierResolution` |
| 0003 §3 `fast` never degraded; `tier_floor:`; preflight hands the substituted pair to floor and parity, never refuses on its own | guard | ADHERES | `parity.go:486,604-627`, `modelavail.go:597-676`; `TestQA7vpTierFloorStillRefusesTheSubstitutedPair` |
| 0003 Dials A–H | guard | ADHERES (E has one hard-coded arm) | `dispatch.go:842-859`, `budget.go:55,112`, `modelavail.go:508-560`; Dial E (a)/(c) have no knob, (b) ships, record presents three |
| 0003 §4 caps in API-equivalent dollars from transcripts; uncounted never rendered as zero | guard | ADHERES | `budget.go:53-116`, `cost.go:20,61,153` |
| 0003 fail-open: only a catalog actually read and lacking the id demotes | guard | DRIFTED, ADR-RIGHT in spirit | `modelavail.go:297-304` returns the retained reading as known past the TTL when the re-read fails; `TestQA7vpAnExpiredSnapshotStillDemotesWhenTheEndpointCannotBeReread` pins today's rule and must flip with 0039 D3c (ranger-base-ksmmz). See finding 3 |
| 0010 overflow ladder per bead; cap + rolling ledger; strong never moves; PID opt-out; parity clean on the target; cap re-read under the lock; ledger writability a precondition | guard | ADHERES | `overflow.go:37-401`; `TestOverflowStrongBeadSkipped`, `TestQAOverflowCapReadIsSerializedWithLaunch`, `TestQAOverflowRefusesAReadableButUnwritableLedger` |
| 0010 §3 (08-29) overflow arms on either brake | guard | UNREALIZED — bead: ranger-base-gxgc | `overflow.go:80-82` arms on the cap only; the amending commit was docs-only; overflow is not configured live |
| 0010 §3 (08-29) a dead `uncounted_cap_<runtime>:` on a priced runtime is named | guard | UNREALIZED — bead: ranger-base-ql08 | `uncounted.go:146-149` returns before reading the key |
| 0010 §5 blind parks on-meter beads and never overflows | guard | ADHERES | `dispatch.go:2332-2336,2471`; `TestOverflowBlindGuardNeverOverflows` |
| 0010 §6 a local meter is armed or off-loud, never blind | guard | ADHERES | `grokpool.go:320-350`; `TestGrokPoolHalfConfiguredIsOffAndLoud` |
| 0018 §1 blind past `blind_max` parks unless armed AND the last reading left headroom (the c3vqe seed) | guard | ADHERES to the record as amended 2026-09-01 | `dispatch.go:669-710`, `blindheadroom.go:72-107` (park asked before the ledger scan); `TestBlindParksWhenTheLastReadingLeftNoHeadroom`, `TestBlindDegradeIsBoundedByMoneyNotTheClock`. The pre-amendment record was the gap; bp224 closed it |
| 0018 §2 no policy fork by failure class | guard | ADHERES | `TestBlindDegradeDoesNotForkOnFailureClass` |
| 0018 §3 cannot-read is not no-records | guard | ADHERES | `cost.go:447-500`, `dispatch.go:693-696`; `TestScanCostsDistinguishesUnreadableFromEmpty` |
| 0034 D1/D2 a hint informs and never gates; windows named by duration | guard | ADHERES | `planhint_codex.go:113,305-312`; `TestReadCodexPlanHintNeverUsed` |
| 0034 D3 hint on display with its age | guard | ADHERES — landed at 088ddeb (ranger-base-ormb) **against a DROP ruling**. See finding 5 | `cmd/posse/cockpit.go:556,2633`, `cmd/posse/main.go:1034`; `TestPlanHintSegment` |
| 0034 D4/D5 the ladder consults the hint; threshold carve-out | guard | ruled DROP on ay3dr; ranger-base-3o10 closed by this audit | no reader outside `planhint_codex.go` |
| 0039 D2 `runtimes/` joins PromotedPaths (the 0015 seed) | wall | UNREALIZED — bead: ranger-base-ight8 | `promote.go:60` omits it; the overlay that sets the strong model is in no manifest, outside the commit-wall class and the land belt. See finding 1 |
| 0039 D3a/D3b verdict names its age; `posse runtimes` prints availability | guard | UNREALIZED — bead: ranger-base-7dbnq | `modelavail.go:355-373` exists, reaches only the gate-refusal notice |
| 0039 D3c a reading demotes only inside its lease | guard | UNREALIZED — bead: ranger-base-ksmmz | see the 0003 fail-open row |
| 0039 D3d probe rides the session credential | guard | UNREALIZED — spike: ranger-base-au0o4 | correctly spike-gated |

#### Constitution and promotion (0012, 0015, 0024, 0031, 0036)

| rule | class | verdict | evidence |
|---|---|---|---|
| 0012 D2 any-deployer test; the lint is the mechanism | guard | ADHERES | `instancebound_qa_test.go:117,287`, `seedcrewbrand_qa_test.go` |
| 0012 D3 one store of record reached by redirect; no local store in the public tree | guard | **DRIFTED (state), ADR-RIGHT** | `beadloss.go:398-421` follows one hop; but the public checkout's `.beads/` holds a database from 2026-08-24 beside the redirect, and its shared-memory file was touched today. No code names a db beside a redirect. See finding 6 |
| 0012 D3-C `beads:` lists one entry; ReadyAll does not de-duplicate | convention | ADHERES-UNPINNED | held by a config comment only; two entries resolving to one store would dispatch twice |
| 0012 D4 runtime contract: template placeholders, unattended flag, gate shell, tier keys, skills, detectability, plan and cost seams | guard | ADHERES | `runtimeyaml.go:34-198`, `runtimepreflight.go:45-142`, `planusage.go:36-48`, `cost.go:244-249`; `credseam_test.go:280` pins "ADR 0012 D4" |
| 0012 D5 bd surfaced through "one file to shim" | convention | ADHERES-UNPINNED | two files: `beads.go` and `cageinner.go` also execs bd |
| 0012 D6 numbering carries | convention | ADHERES | HISTORY.md; 0040 §3.6 |
| 0015 three trees by taking-effect path; the promoted set written only by promote; personas and envs excluded | guard | ADHERES | `promote.go:60,65,964-977`; `TestPromoteNeverTouchesEnvsStateOrPersonas`, `TestQAConstitutionExclusionListStaysComplete` |
| 0015 §3 preconditions, blob-at-SHA read, manifest, launch verify, anchor walled | guard + wall | ADHERES | `promote.go:562-720,875-893`, `herdrback.go:1351-1356`, `seatbelt.go:829-836`; `promotelaunch_qa_test.go`, `promoteanchor_qa_test.go` (the "until 70ry lands" clause is stale, rowut) |
| 0015 §3 anchor-state line on the watch preamble | guard | UNREALIZED — bead: ranger-base-xevp7 | no such string in non-test Go |
| 0015 §3 the fence spelled four ways: PID deny, refusal under a persona, the commit-wall constitution arm, the land belt | wall + guard | ADHERES | 11 of 11 PIDs deny promote; `promote.go:421`; `gates.go:2623-2680`; `worktree.go:723-855`; `TestShippedPIDsDenyPromote`, `constitutionwall_qa_test.go`, `constitutionland_qa_test.go` |
| 0015 §3 bd verbs denied option-aware; hook rules narrowed | wall | ADHERES (content and behaviour) — **finding 7 was wrong; corrected 2026-09-01, ranger-base-efk14** | `gates.go:166-175`, `bdshim_test.go`; 11 of 11 PIDs carry the 25 rules; `scripts/verify-pid-deny-set.sh` holds the content. `bdhookcommit_qa_test.go` exists at the repo ROOT (d085a96, on main) and is green; four mutants killed. See the correction under finding 7 |
| 0015 §4 queue repo reached by redirect; launcher commits path-limited, never pushes; no session holds a write grant into the constitution repo | guard + wall | ADHERES | `queuejsonl.go:65-82`, `seatbelt.go:322-324`; `TestQueueCommitNeverPushes`, `seatbeltconstitution_qa_test.go:87` |
| 0024 D1 public iff any deployer could have written it: genre arm, content arm, identity derived on the box, unmarked is public | wall | ADHERES | `visibility.go:75,152-196,239-255,496`; `TestBeadsVisibilityFailsClosed`, `TestDeriveIdentityLiterals` |
| 0024 D1 the genre allowlist covers every public docs dir | wall | DRIFTED, low | `docs/probes/` (one README, two days older than the check) is not in `PublicDocsGenres`; a new file there is refused today. See finding 10 |
| 0024 D2 three commit-hook checks in `prepare-commit-msg`, stamp refreshed at every launch, override logged | wall | ADHERES | `gates.go:2332-2540`, `herdrback.go:1511-1524`; `TestDocsGenreAndProseGuardHook`, `TestIdentityLiteralGuardHook`, `TestVisibilityOverrideIsNeverDispatched` |
| 0024 D3 restate-and-cite | convention | ADHERES-UNPINNED | the refusal names it |
| 0024 D4 migration executed; ceilings blessed and restated in the example config | convention | DRIFTED, ruling-side | the restate step (ranger-base-axft B1) never landed: the example config carries no cap keys; the record's prose is their only public home |
| 0031 init refuses the home it was launched from; keyed on the target; no PID deny | guard | ADHERES | `herdrback.go:1704`, `init.go:111-133`; `initoperatorfence_qa_test.go:86-139`; 0 of 11 PIDs deny init |
| 0036 backup is a harness verb; refuses any remote; freshness on-box | — | UNREALIZED — beads: ranger-base-a0ln0 and its chain (ruled BUILD) | no `backup` symbol |

#### Credentials (0019)

| rule | class | verdict | evidence |
|---|---|---|---|
| D1 one seam `Read(runtime, purpose)`; nothing else acquires | guard | ADHERES / the "nothing else" half UNPINNED | `credential.go:166`; three callers by grep; no census test |
| D2 meter reads the store of record per OS; the darwin path never reads the file store; the linux path shares the parser | wall + guard | ADHERES (linux probe UNREALIZED — bead: ranger-base-ydjz) | `credential.go:422-597`; `TestTheMeterStoreSwitchIsTotal`, `TestQACredentialReadDenyRefusesUnderSandboxExecAndTheControlDoesNot` |
| D3 NoSource is off-with-witness, never blind; no fork by credential class | guard | ADHERES | `credential.go:97-149`, `dispatch.go:406-468`; `TestQABlindDegradeDoesNotForkOnTheCredentialClass` |
| D4 `posse refresh` is the one write; refused under a persona and without a TTY; never touches a metered key | guard + wall | ADHERES | `refresh.go:77-158,340-396`; 11 of 11 PIDs deny it; `TestRefreshWillNotWriteAMeteredCredentialByShape` |
| D5 expiry surfaced, never gating; session mints only on the timer surfaces | guard | ADHERES | `credexpiry.go:141`, `dispatch.go:523-534`; `TestDispatchPassWarnsOncePerPassAndParksNothing` |
| status line | — | DRIFTED, CODE-RIGHT | still `proposed`; 0040 amend row |

#### Personas and handoffs (0001, 0005, 0006, 0007, 0026)

| rule | class | verdict | evidence |
|---|---|---|---|
| 0001 key set; deny-wins delta; nine headings; hard risk lines verbatim; metric catalog | guard | ADHERES | `agents.go:168-197,375,385-391`, `runtime.go:610-620`, `pidcheck.go:44-113,314-338`; `TestRenderAllowDeny`, `TestExampleAgentsArePIDs`; `posse agent check` passes 11 of 11 live PIDs |
| 0001 deny the verb, not a flag | convention | ADHERES-UNPINNED | every live and example PID denies push by verb; the rule is prose |
| 0001 `mode` column | convention | ADHERES, governs nothing | `agents.go:519` reads columns 0 and 2 |
| 0005 assembled prompt, references not content; guardrails line always | guard | ADHERES | `dispatch.go:1460-1682`; `TestWorkPromptAssembly` |
| 0005 §1 skeleton lists the own-worktree block | guard | DRIFTED, CODE-RIGHT | `dispatch.go:1637-1649` renders five lines the record does not list; 0040 amend row |
| 0005 §2 the six rungs, fixed text; provenance caveat; `-l question` never dispatched | guard | ADHERES | `dispatch.go:1567-1584,2109-2111,3499`; `TestEscalationLadderProvenanceCaveat`, `TestDispatchPromptAndQuestionBeads`; text matches 0005 §2 and 0026 §5 clause for clause |
| 0006 comment / new bead / nowhere | convention | ADHERES-UNPINNED | |
| 0006 verify-after per pass with batch and age; rejected-close exemption needs both halves | guard | ADHERES | `verifyafter.go:58-705`; `TestVerifyAfterVerifiesACloseThatShippedDespiteRejectionWords` |
| 0006 §2 grooming row and `queue-honesty` | convention | UNREALIZED — silently dropped | no groom bead ever filed; 0040's "shrink to a line" is the fix |
| 0006 §4 PID Handoffs rows say who · label · content | guard (examples) / convention (live) | ADHERES for examples; the coordinator's live PID has two unlabelled rows and no cite | `TestExampleAgentsHandoffsAreShapes` covers `examples/` only |
| 0007 declared means required; per-runtime materialization; additive; three lints; ro mount in the cage | guard + wall | ADHERES (the exclude write and "never removes another persona's link" unpinned) | `skills.go:36-272`, `parity.go:354-371`, `pidcheck.go:285-295`, `cage.go:400`; `TestSkillsParity`, `TestRenderAgentsSkills` |
| 0026 four triggers in the ladder; two shapes; sourcing; cost rule; §5 mechanics | convention (ladder rendered) | ADHERES (practice partly unheld: the one open spike blocks nothing) | `dispatch.go:1574`; store census: two spikes ever |

### 2. Findings, by consequence

Ordered by what breaks: money and the constitution first, then dispatch
correctness, then observability, then hygiene. Each names the fix lane and
the bead.

1. **The model dial is live constitution outside every fence** (ADR 0039
   D2 / 0015 §2; UNREALIZED; wall, unbuilt). The runtime overlay at the
   home is read at every launch and is the only thing that makes the strong
   tier the current model; it is in no manifest, not in `PromotedPaths`
   (`promote.go:60`), so outside the commit-wall class and the land belt.
   A persona can commit it in the constitution repo unrefused, and a hand
   edit at the home is invisible to the launch verify. The built-in it
   falls back to (0039 D1, ranger-base-per37) names the previous id.
   Money and constitution. Existing bead ranger-base-ight8, raised to P1
   by this audit with the three readers named.
2. **A ratified refusal is not in main** (ADR 0013 §2, vbp3 amendment;
   DRIFTED, ADR-RIGHT; wall). `interstitial.go:254` still skips a declared
   `danger:` screen that has no probe, so the first typed-delivery yaml
   runtime with a machine-mutating dialog is dispatched onto it while
   `runtime check` says LAUNCH REFUSE. The four code commits sit on the
   session branch only; the ratification doc landed. Dispatch correctness
   and machine safety. Bead already open: ranger-base-tq93 (dinesh, since
   08-30), raised to P1 by this audit. The lesson is for review 1: 0040's
   "governs today" cell for 0013 read a closed bead as a landed commit.
3. **A stale catalog rules the strong tier while the probe is down** (ADR
   0003 fail-open / 0039 D3c; DRIFTED in spirit; guard). `modelavail.go:304`
   returns the retained reading as known past its lease when the re-read
   fails, and `TestQA7vpAnExpiredSnapshotStillDemotesWhenTheEndpointCannotBeReread`
   pins that. With the probe failing since 08-31, every strong launch is
   ruled on by a snapshot. Money and tier honesty. Existing bead
   ranger-base-ksmmz; the pin must flip with it (a parked skip encoding
   the rejected alternative is the class to watch for).
4. **The built-in overlay silently drops thirteen declarable keys** (ADR
   0021 D1; DRIFTED, ADR-RIGHT on principle; guard). `overlayBuiltin`
   (`runtime.go:1478`) applies nine keys; `warnUnknownRuntimeKeys`
   (`runtimeyaml.go:169`) treats every other declarable key as known, so an
   overlay on a built-in declaring `rules_precedence:`, `state_dir:`,
   `env_required:`, `turn_outcome:`, `unattended:`, `self_sandbox:`,
   `project_config:` or `skills_cwd:` loads clean and changes nothing. By
   D1's own principle the first three are instance facts and should
   overlay; the rest are mechanism and should refuse with D2's message.
   This is the silence class 0017 exists to kill, and the overlay is in
   service today. Correctness, then observability. New bead for dinesh;
   0021's key list amends to "declarable minus the mechanism set" in 0043.
5. **0034 D3 shipped the day it was ruled dropped** (governance). The
   ruling on ranger-base-ay3dr says D3–D5 DROP; HEAD 088ddeb landed D3
   (display only, gates nothing; ranger-base-ormb). The order of the two
   within the day is ASSUMED unknown. D4/D5 are dropped either way, so
   ranger-base-3o10 is closed by this audit. Whether D3 stays as built is
   the operator's line: a question bead.
6. **A stale second store sits in the public checkout** (ADR 0012 D3;
   DRIFTED in state; convention, no guard). A database from 2026-08-24
   sits beside the redirect and something opened it today. bd resolves
   the redirect first, so nothing is wrong yet; the day the redirect file
   is lost, a three-week-old graph answers at exit 0. Dispatch correctness.
   Ops: delete it (gilfoyle). Code: a preflight or `posse status` line
   naming a db beside a redirect (dinesh).
7. **The bd-hook narrowing has no behaviour pin, and the record says it
   has** (ADR 0015 §3; DRIFTED on the record's claim; guard). The record
   names `bdhookcommit_qa_test.go` as one of two non-optional pins; no
   commit in history ever carried that file. Only the content pin
   (`scripts/verify-pid-deny-set.sh`) holds the fleet-cannot-commit class,
   which was a P1 incident. Verify lane: laurie.

   **CORRECTED 2026-09-01 (ranger-base-efk14). This finding was false.**
   The file exists and always has: it landed 2026-08-29 in `d085a96`
   (ranger-base-c7ek) at the **repo root**, not under a package directory,
   and is on `main`. This audit located each pin by name (see the *Measured
   at* preamble) and posse keeps 32 `*_qa_test.go` files beside `go.mod`,
   so a search scoped to `internal/` and `cmd/` missed it. Measured at
   `9b48166`: `go test ./` green in 158s, the three tests the record
   describes all present and passing across all 9 shipped PIDs, and four
   mutants killed — widening one PID's deny back to the whole verb, the
   keeps-narrow-and-broad shape, dropping the gates dir from the child
   `PATH`, and installing no hooks. **ADR 0015 §3 is accurate as written and
   needs no edit on this account** (said on ranger-base-rowut). The rule is
   ADHERES on both halves.

   What this *did* expose is a real gap, now closed: the records name 36
   test files and nothing resolved a single citation, so a renamed or moved
   pin leaves every record still naming it and the next auditor resolving by
   hand — which is how this finding was reached. `adrtestcitation_qa_test.go`
   (repo root) now requires every cited pin to exist. It also surfaced 12
   citations across 7 records still spelling the retired `internal/rhq/`
   directory: ranger-base-1d8bk, architect lane.
8. **The shop's only oversight delivery is disarmed on a comment that is
   no longer true** (ADR 0027, 0029 §2 and §4; convention, instance
   config). The live config's pulse block says "do not arm yet" because of
   a bead that closed 2026-08-31 with pins, and calls the design by the
   wrong record number. A blocked session at 3am reaches nobody, and 0029
   §4's SLA and metric are unrealized for the same reason. Arming is an
   operator call: a question bead. The two stale comments are one edit in
   the instance tree (with the coordinator citation below).
9. **Two doc-ahead-of-code rules in the plan guard, dormant** (ADR 0010
   §3, 08-29; UNREALIZED with beads; guard). Either-brake arming
   (ranger-base-gxgc) and the dead-cap line (ranger-base-ql08) were
   amended into the record by a docs-only commit; the code arms on the
   cap alone and nils the key before reading it. No exposure live because
   overflow is not configured; the record promises a brake the code lacks.
   Beads exist and are unblocked.
10. **`docs/probes/` is a public docs genre the allowlist does not name**
    (ADR 0024 D1; wall; hygiene). Predates the check by two days; a new
    file there is refused today. One constant edit or a move: dinesh.
11. **Three silently dropped clauses that 0040 §3.5 forbids** (hygiene,
    each one line in a merge bead): 0025 §1 "the session meta prints the
    class" (rkh3w must carry or strike it); 0035 §3 grok's compensating
    control, which no non-test symbol realizes though ranger-base-0emp
    closed (verify lane, then rkh3w); 0016 §2 "the pulse may consume the
    hint" (retire in the 0016 amend).
12. **Stale text in the survivors that 0040 already schedules**, confirmed
    by this audit and listed on the merge beads: 0011 App.A relaunch-
    outside-the-lock; the `dispatch.go` header; 0020 §2 tiebreak and §3/§6
    live-config sentences; 0033 §1 and §4 "until that lands"; 0013 §4 reap
    guard omits `--force` and its pin path; 0037 Claims; 0015 cutover
    passages and the 70ry clause; 0019 status; 0005 §1 own-worktree block;
    0003 Dial E ships (b) only and the strong cell; 0024 D4's restate step;
    0012 D5 "one file" is two; 0038 and 0036 status lines lack
    `· unbuilt:` though beads exist; 0040's own "no build bead found" cells
    for 0036 and 0038 are stale at HEAD.
13. **Unpinned arms worth a QA pin** (hygiene, verify lane, bundled in two
    beads): the push-effect note text; the shims-tier relaunch retype; the
    one-throttle observable; the skills exclude write and "never removes
    another persona's link"; both-brakes ordering on one pool; and three
    census pins for absence rules (no second credential acquirer, no new
    shadow predicate, the harness never closes a bead).
14. **Rolling seats meet their target at the median only** (ADR 0028 §5;
    ADHERES; instrument). MEASURED in the live watch log: per-Run
    treatment-arm medians of seconds to a few minutes, maxima of tens of
    minutes to hours, about half the windows per Run refill-closed. Not a
    breach; 0044 should say "median" and the report should print per
    refill, not when Run returns.

**The seeds, answered.** 0003 tiering trails the dial: confirmed, both the
record and the built-in name the previous id (finding 1). 0015
`PromotedPaths` omits `runtimes/`: confirmed (finding 1). 0018 park
overridden by dollar caps: no longer; the 09-01 amendment made armed
necessary-not-sufficient and the code parks before the ledger when the last
reading left no headroom, so it is adherence to the amended record, and the
pre-amendment record was the gap. 0021 keys vs `builtinOverlayKeys`: the
nine match key for key; the gap is the other thirteen (finding 4). 0013
grid vs the six stages: the grid is exactly the six plus five rows 0017 §1
governs and the code says so. 0022 single-writer vs the shared-index wall:
two arms of one hook, no conflict, both pinned. 0028 idle-to-next: measured
per seat with a control arm, median claim only (finding 14). 0033 the
refusal text is §2's sentence fired from `laneFor`, which both launchers
share; the record's two "unbuilt" clauses are stale, and the live config
cites the wrong record number for the key.

### 3. Ungoverned: candidate ADR text for the operator

Load-bearing rules the code states and no record does. None is a bug.
Ranked by what depends on it.

1. **Substrate version pins** (`etc/{bd,codex,grok,herdr}/version-pin.toml`,
   `scripts/verify-*-pin.sh`, `make verify-*-pin`). A pinned binary is
   hand-placed, a bump is an operator ruling with a prerequisites bead, and
   the pin must carry the invariant the rollback proved (a rollback that
   leaves a live process from the reverted artifact is not a rollback).
   Records mention the grok pin as a fact; none states the rule. A 13-day
   orphan and a 40-minute degradation rode on it.
2. **The meter's network wall** (`credpin.go`, `planusage_anthropic.go`):
   only compiled-in URLs are credentialed; an override must be loopback,
   is asked without the bearer, and its answer never becomes the fleet's
   shared fact; the pinned client refuses off-host redirects. Plus the four
   credential-failure classes and their distinct next moves. Bead-cited
   only; belongs as an 0019 amendment.
3. **The load guard** (`loadguard.go`): no launch while the box is over
   its load line; a pass over it is skipped whole with a witness naming the
   load and the holders. Cites a bead and 0009's preamble; no record.
4. **The `-n` attempt ration is per process** (`epoch.go:31-40`) while
   spend is per epoch; a supervised restart restores the ration. One
   sentence in 0044 §2.
5. **The guarded runtime cannot be its own overflow target**
   (`overflow.go:102-106`, pinned); **attended vs unattended is the fork
   for every blind rule** and "attended" is defined only in code; **blind
   max set to zero bypasses the headroom park** (code-only). Three lines
   in 0045.
6. **Overlay keys are a closed list distinct from the declarable list**
   (folded into finding 4); **probe currency semantics**: an empty
   recorded version counts as current forever (`runtimeprobe.go:242`). One
   paragraph in 0043.
7. **Pre-install L3 drift report** (`herdrback.go:1511-1528`, probe then
   install then probe), **the legacy marker one-way door**
   (`gates.go:1305-1320`), and **the chain dispatcher as the default L3
   shape beside bd's hooks**. Three sentences in 0042's L3 section.
8. **init's manifest stamp semantics** (`promote.go:163`,
   `init.go:280-362`): Seeded only when it hashed everything it wrote;
   never arms the verify over a home that had a constitution. Three
   incidents rode on it; recorded in 0031's Context and bead ids. One
   section in 0015's trim.
9. **The hand-copied argv gate** at the home, wired into the runtime's
   settings, refreshed by nobody, in no manifest. Where it lives and that it
   goes stale silently. One line in 0042 or 0015.
10. **`route_order:`** (built unrecorded, 0040 routes it to 0044);
    **foreign-row fail-closed** as a rule line not a Consequences aside
    (0044); **the cockpit's one-scan-in-flight guard** (0004);
    **`posse status` exits non-zero on non-empty OR unread** (0029 §2);
    **both persona-dir env names exported** during the rename window
    (`herdrback.go:1710-1717`); **the instance label rule**
    (`instance.go`: labels are constructed from names, never parsed back).

### 4. Beads cut

Shape per ADR 0006: DRIFTED code → dinesh `-l code`, one per finding;
ops state → gilfoyle `-l devops`; missing pins → the verify lane `-l qa`;
operator lines → `-l question`; record-side findings → comments on the
merge and amend beads and on the ruling bead ranger-base-ay3dr.

| bead | lane | finding |
|---|---|---|
| ranger-base-otoq8 | dinesh code | 4: overlay drops thirteen declarable keys |
| ranger-base-dj3k2 | dinesh code | 6: name a db beside a redirect |
| ranger-base-pop14 | dinesh code | 10: `docs/probes/` genre |
| ranger-base-0kbp7 | gilfoyle devops | 6 and 8: delete the stale store; two stale config comments; operator promotes |
| ranger-base-efk14 | laurie qa | 7: the bd-hook narrowing behaviour pin |
| ranger-base-q8dhm | laurie qa | 13: three census pins |
| ranger-base-esa0j | holden qa | 13: five unpinned runtime-visible arms |
| ranger-base-ezp0j | laurie qa | 11: verify the grok pane-mode surface |
| ranger-base-ntvtx | question | 5: keep 0034 D3 as built, or revert |
| ranger-base-qnk3p | question | 8: arm the pulse |

Existing beads carried, not duplicated: ranger-base-ight8 and ranger-base-tq93
raised to P1 (findings 1 and 2); ranger-base-ksmmz, gxgc, ql08, per37,
7dbnq, xevp7, vqyxl, 65po1, mugt2, t4f1, a0ln0, ydjz, au0o4, 385x cited on
their rows. Closed by this audit: ranger-base-hxez (duplicate of 6tj5r) and
ranger-base-3o10 (ruled DROP). Comments left on rkh3w, hn32r, yv9uo, vl294,
mqoid, mppjc, rowut and ay3dr with the record-side items.

### Measured versus assumed

| claim | status |
|---|---|
| every `file:line` in §1 | MEASURED — opened and read at 088ddeb by a reader, the ten load-bearing ones re-opened by the architect (vbp3 ancestry per commit, the overlay key lists, the stale store's mtimes, the genre allowlist, the absent test file's history) |
| every pin named | MEASURED by existence and a read of its assertion; ASSUMED green (none run) |
| live PID counts (11 of 11 carry the fences) | MEASURED by grep and `posse agent check` |
| the seat-idle figures | MEASURED from the live watch log's report lines |
| the vbp3 strand | MEASURED: `git merge-base --is-ancestor` per commit |
| build-bead existence for every UNREALIZED row | MEASURED: `bd show` per id 2026-09-01 |
| the order of the ay3dr ruling and the ormb landing within the day | ASSUMED unknown |
| docker-backed live pins skipping on this box; the grok pane-mode surface not living in herdr; the chain member in the harness repo being the current render | ASSUMED |
| the reader-row totals | MEASURED as counted by each reader; the survivor-clause table above collapses sub-rows |
