# ADR 0040 — Consolidate ADR 0001–0039: a disposition per record, six concern records, supersede-by-pointer and lazy citation re-pointing

*Status: proposed 2026-09-01 · owner: architect · bead ranger-base-0tr1e ·
the disposition table (§1) is a PROPOSAL until the operator rules on it
(question bead named in Consequences) · the convention (§3) and the
pricing (§4) stand on their own · the adherence audit
(ranger-base-4wxko) audits against the surviving-rule column of §1*

> The operator, 2026-09-01: "review the ADRs top to bottom … we are up to
> 39 now, for simplification and consolidation." This is review 1 of 2:
> what each record still binds, what to do with it, what shape the set
> takes, and what the move costs. Review 2 audits the code against the
> survivors. Nothing is rewritten here — a consolidation is implementation,
> and the operator rules on the table first.

## Context

Measured at 521d3db on 2026-09-01 (`docs/adr/`, `internal/`, `cmd/`,
`etc/`, `examples/`, the live PIDs under `~/.config/posse/agents` and the
persona ORDERS under `~/.config/posse/personas`):

| what | count |
|---|---|
| numbered records 0001–0039 | 39 files, 95.8K words (98.7K with the three 0013 probe records; six probe scripts beside) |
| `· amended` stamps in status lines | 32 (0015 carries 8, 0013 6, 0002 6) |
| appended `## Amendment …` sections | 9 (eight of them in 0002) |
| `ADR nnnn` citations in code + config | 1,938 across 291 files (internal 1,576 · cmd 182 · examples 68 · etc 2 · PIDs 49 · ORDERS 61) |
| … of which name a section (`§n`, `Dn`) | 1,391 |
| … in test files vs non-test (internal+cmd) | 164 files vs 81 files |
| citations inside a STRING LITERAL in non-test Go (a refusal, a help line, a rendered hook body) | 141 lines in 37 files; per record in §4 |
| test assertions on an ADR number | 9 sites, listed in §4 |
| citations in `NOTES.md` (a journal) and inside `docs/adr/` itself | 142 and 400 — not re-pointed under any option |
| records whose code paths name `internal/rhq/` | 15 records, 41 occurrences; the package became `internal/posse/` in 9c00e19 |
| records with zero citations anywhere in code or config | 0035, 0036, 0037, 0038 (0026 and 0039 have one each) |

Two readings of those numbers, both MEASURED:

- **Nobody reads the set; everybody greps it.** 96K words is a working
  context on its own, so a persona learns a rule from the citation
  beside the code, not from the record. That is why fifteen records can
  point at a directory that stopped existing on rename and nobody
  noticed: the pointer is never followed.
- **The binding text of a rule is assembled, not read.** ADR 0002's
  layer L3 (the commit wall) is decided in §3, corrected in three of the
  eight appended amendments, then amended again by 0023 (identity-then-
  probe), 0025 (enforcement class) and 0038 (identity write-deny). A
  reader who wants "what does L3 refuse today" reconciles seven passages
  in four files. The same shape holds for 0015 §3 (the fence, spelled
  four ways across 0015, 0002 §3 and 0031) and for 0013 §2 (six
  in-place amendments folding probe results).

The house rule that ADRs are append-only records is kept whole here.
Consolidation is a **new record that supersedes**, never an edit that
deletes history. What changes is which record a citation should land
on, and how the old number resolves to the new one.

## §1 Disposition table

One row per record. *Governs* names the code or config the record binds
today, verified by reading the cited site — "nothing live" means no code,
config, PID or gate realizes the decision. *Disposition* is one of KEEP ·
AMEND (what) · MERGE INTO n · SUPERSEDE BY n · RETIRE. *Survivor* is the
rule the adherence audit checks, in the record where it will live after
this ADR executes. Every "overlaps" claim behind a MERGE names two
passages; they are in the reader briefs cited on the bead, and the merge
bead quotes them.

Legend for *Disposition*: **R-gates** = new ADR 0041 (the wall) · **R-runtime** = new ADR 0042 (what a runtime is) · **R-dispatch** = new ADR 0043 (the pass) · **R-planguard** = new ADR 0044 (the plan guard). Cite counts are non-test / test files in `internal`+`cmd`+`examples`+`etc`; "strings" are runtime-visible literals; "pins" are test assertions on the number.

| ADR (words) | governs today (path) | disposition | survivor: the rule the audit checks → home |
|---|---|---|---|
| 0001 PIDs (2.3K) | `agents.go` LoadAgent (all 8 named keys read, plus 12 keys 0001 never names), `runtime.go` allow/deny delta, `pidcheck.go` headings and metric catalog, `gates.go` verb-not-flag; 24/16, 5 strings, filename pinned by the seed-publication root | KEEP · AMEND — schema table and worked example replaced by a pointer at the parser's key doc; shipped Consequences struck; the `mode` column governs nothing (say so) | PID contract: key set, deny-wins delta, nine headings, deny the verb → 0001 |
| 0002 runtimes & gates (8.7K) | `runtime.go` table/precedence/hatch, `herdrback.go` env+meta, `gates.go` L1/L3, `seatbelt.go` L2, `cage*.go`/`egress.go` L4, `parity.go` §4, `agents.go` §5 keys, `trust.go`; 87/31, 10 strings, two installed hook bodies | **SUPERSEDE BY R-gates** — eight appended amendments; the L1 row's mechanism is 0009, the L3 row omits 0022's arm and both call theirs "third", the hatch sentence is stale against 0017 §4 and 0021, Verification 5–7 duplicate 0009's; the four project-config-trust amendments are one sub-decision buried in the appendix trail | layers L0–L4 and what carries each into the cage; parity or refuse, `--allow-degraded` marked; the PID keys; project-config trust; escape C measured → R-gates |
| 0003 model tiering (3.7K) | `runtime.go` tier maps, `dispatch.go` precedence, `parity.go` CheckTier/`tier_floor`, `budget.go` Dial E, `modelavail.go` preflight/fallback; 54/14, 10 strings | KEEP · AMEND — strong cell reads `fable-5-1` when ranger-base-per37 lands; drop the dead "grok is still runtime default" sentence; §1's display amendment cites 0013 §6 instead of restating it | tier is a name, three tiers per runtime; precedence ladder; `fast` never degraded, `tier_floor`; Dials A–H → 0003 |
| 0004 cockpit v2 (1.8K) | `cmd/posse/cockpit.go` render/chromeFor/IN PROGRESS, `beads.go` InProgressAll, `dispatch.go` holder join; 39/30, 0 strings | KEEP · AMEND — §2 says the run record heads the holder join (true in code and in 0008); note the file now also draws 0016/0018/0019/0029 | pure `render(w,h)` over a row model; three sections one cursor; claim-in-flight refusals → 0004 |
| 0005 work prompt (2.6K) | `dispatch.go` workPrompt/promptContext/EscalationLadder (cites "ADR 0005 §2"), `agents.go` `## Work prompt`; 9/5, 2 strings | KEEP · AMEND — §1 for file delivery (0013) and the own-worktree line; §2 stays the one home of the six rungs | assembled prompt, references not content; NOTE/ASSUME/SPIKE/ASK/HANDOFF/REFUSE; `-l question` never dispatched → 0005 |
| 0006 handoff shapes (1.9K) | `verifyafter.go`, `beads.go` discovered-from, 9 example + 11 instance PIDs (`agents_test.go:670` asserts the number); 31/6, 2 strings, 1 pin | KEEP · AMEND — status line names its four body amendments; §3's edge dedupe sentence → the marker rule; §2's security-priority and groom rows shrink to a line each (govern nothing) | comment / new bead / nowhere; verify-after per pass with batch and age; PID Handoffs rows say who·label·content → 0006 |
| 0007 skills binding (1.5K) | `skills.go`, `runtime.go` Skills realizer, `parity.go` Degraded row, `pidcheck.go` three lints, `cage.go` ro mount; 21/8, 5 strings | KEEP · AMEND — status line carries the 08-18 verification and 08-28 amendment; collapse the superseded Context rows | declared means required; per-runtime materialization; additive → 0007 |
| 0008 crew sessions (1.2K) | `herdrback.go` Crew/MarkCrew/CrewTag, `dispatch.go` personaActive/crewHeld, `autoreap.go`; 30/17, 1 string, 5 pins | KEEP — absorbs 0030 (already verbatim in §2's second amendment) | a crew session is invisible to dispatch; no twin; orphaned claim parks on the assignee's crew session → 0008 |
| 0009 gate shell (2.3K) | `gates.go` gateShellScript/renderGateShell/GatePrefix, `gateaudit.go`, `loadguard.go` predicate; header "(ADR 0009)" in every rendered gate shell; 33/17, 13 strings | **MERGE INTO R-gates** — it is the L1 row's mechanism; §3's "L3 remains the wall" is already false under 0025 | gate shell renders per persona, REAL outside every gates dir, PATH rebuilt, `SHELL=` on the typed line, rc files gated → R-gates |
| 0010 plan-guard overflow (2.6K) | `overflow.go` ladder/cap/ledger, `dispatch.go` planBlind, `grokpool.go`; 28/7, 0 strings | **SUPERSEDE BY R-planguard** — the 08-29 §3 arming amendment and the ql08 loud line are doc-ahead-of-code (ranger-base-ql08 open); §5's table is byte-identical to 0013 §3; its status line records neither 0018 nor 0034, both of which amend it | overflow ladder per bead; cap + ledger; blind parks and never overflows; a local meter is armed or off-loud → R-planguard |
| 0011 dispatch model (4.0K) | `launchlock.go`, `herdrback.go` prune/Gen/run record, `dispatch.go` foreignHeld, `watchlock.go`; 86/29, 3 strings, 2 pins | **SUPERSEDE BY R-dispatch** — kept-list amended by 0028 (burst/gather struck), the relaunch-lock "bead filed" closed in code unrecorded, `dispatch.go`'s header still narrates the pre-Dial-F loop it promised to fix; Appendix A (prior art) is carried by Lineage, not rewritten | bd is the queue, claims are leases without expiry; one launcher per home (flock, three holders); prune proves death (grace AND identity); the session meta is the run record → R-dispatch |
| 0012 harness/instance boundary (2.7K) | `pidcheck.go` role check + `instancebound_qa_test.go` (App.A 5), `.beads/redirect` readers (D3-C), `runtimeyaml.go`/`runtimepreflight.go`/`costseam.go` (D4: 80 non-test cites, `credseam_test.go:280` pins "ADR 0012 D4"), `init.go` embedded examples; 80/50, 16 strings, 1 pin | KEEP · AMEND — D3's supersede/export and D6's cut-over are executed → HISTORY.md; "App.A 5" names a section the restatement does not carry, so name the D6 bullet; note the queue's home is 0015 §4 | any-deployer test; one store of record reached by redirect; runtime contract D4; numbering carries → 0012 |
| 0013 runtime dispatch contract (7.9K) | `runtimecheck.go` grid, `runtime.go` PromptArgv/Record/NativeRules/CostPriced, `interstitial.go`, `reachability.go`, `uncounted.go`, `turnfailure.go`; 136/56, 35 strings, 4 pins, `herdrback.go:1582` names the rules-precedence probe path | KEEP · AMEND — §3 becomes one line pointing at R-planguard; §1's settle row cites the turn-outcome probe file (captured, unconsumed); Claims trimmed to what code asserts. The three probe files stay where they are (path-pinned) | six-stage contract with declared-by; argv-first delivery, claim first, no keystrokes; record trust and reachability; `CostPriced()`; unmapped tier shows `default` → 0013 |
| 0014 path-scoped writes (2.1K) | `cageinner.go` parsePathScopedWrite, `seatbelt.go` trailing deny, `cage.go` `:ro` overlays, `pidcheck.go`; 52/43, 20 strings, 2 pins | KEEP · AMEND — §4's hooks-overlay deferral points at 0038 | `Edit(<glob>)` is a subtree write deny; matrix per tier; `writable:` at both tiers; a hook is not a cage → 0014 |
| 0015 constitution promotion (7.0K) | `promote.go` PromotedPaths/manifest/refusals, `herdrback.go` launch verify, `gates.go` constitutionGuardBody, `worktree.go` land belt, `queuejsonl.go`, `memoryland.go`; 79/65, 42 strings, 11 PIDs, 2 pins | KEEP · AMEND (heavy, section numbers frozen) — cutover history (Context symlink row, §2's rhq sentence, §4's relocation, §6, §7's cutover, Sequencing, Deferred Q5/Q6, Verification 1/2/6/7, the cutover Measured rows) → HISTORY.md; drop the "70ry open" clause (landed); fix two dead `home-cutover.md` pointers; own the bd-verbs/hook-narrowing amendment so R-gates points here instead of restating it; mark the zio33 anchor-state line unbuilt; record 0039 D2 when ranger-base-ight8 lands. Not superseded: 42 strings, a rendered hook body and a promote of 11 PIDs buy nothing the trim does not | three trees by taking-effect path; the promoted set; promote's preconditions, blob-at-SHA read, manifest, launch verify; the fence spelled four ways; bd verbs denied option-aware; queue repo reached by redirect, launcher commits never pushes; personas excluded; envs never promoted → 0015 |
| 0016 herdr event hints (2.2K) | `herdrevents.go`, `watch.go` hint wake, `cockpit.go` hint loop; 19/11, 0 strings, 1 pin | KEEP · AMEND — status/Context "0013 (monica pulse)" → 0027; the pulse-consumes-hint sentence marked unbuilt | events are hints, level-triggered passes are the truth; one subscription per process; `blocked` is not a settle → 0016 |
| 0017 runtime equivalence (3.2K) | `runtimefields_qa_test.go` (the pinned classification), `parity.go` verdicts, `runtimeyaml.go` keys, `runtime.go` rules_precedence; 14/29, 5 strings | **MERGE INTO R-runtime** — §1/§2/§4/§5 carry; §3's struck table → a pointer at the pinned test; §6 retired | the checklist is the struct plus the grid; PARITY / DECLARED DIFFERENCE / UNKNOWN, UNKNOWN never fatal; shadow-predicate rule; declarability list → R-runtime |
| 0018 blind meter, armed ledger (2.6K) | `dispatch.go` blindFork, `blindheadroom.go`, `planusage.go`, `cost.go` Unreadable; 45/20, 2 strings, 2 pins | **MERGE INTO R-planguard** — the blind rule lives in 0010 §5, 0013 §3 and 0018 §1 today; Lineage carries the 09-01 amendment verbatim (accepted the day it was built) | blind past `blind_max` parks unless armed AND the last reading left headroom; no policy fork by failure class; cannot-read is not no-records → R-planguard |
| 0019 credential seam (3.4K) | `credential.go`, `refresh.go`, `credexpiry.go`, `secrets.go`, `init.go`; 90/42, 18 strings, 1 pin | KEEP · AMEND — status `proposed` → accepted (D1–D6 shipped, three amendments folded); V1 (Linux live login) is the one open item | one seam `Read(runtime, purpose)`; meter reads the store of record per OS; NoSource is off-with-witness, never blind; `posse refresh` is the one write, refused under a persona; expiry surfaced never gating → 0019 |
| 0020 multi-seat lanes (2.2K) | `dispatch.go` Route/laneFor/seatFor, `verifyafter.go`, `budget.go`; 9/10, 0 strings | **SUPERSEDE BY R-dispatch** — §2's `route_order` tiebreak was built unrecorded; §3 and §6 quote live-config facts now false; §5 re-denominated by 0028 | lane = label set, seat = persona, availability-first with `route_order`; verify-after unassigned; one serial seat per persona; width law per epoch; batched verify fan-in → R-dispatch |
| 0021 built-in overlay (1.2K) | `runtime.go` overlayBuiltin/builtinOverlayKeys, `runtimecheck.go` footer; 14/7, 3 strings | **MERGE INTO R-runtime** — its promised 0002 §1 sentence never landed; twenty lines of decision | a yaml naming a built-in is a per-key overlay; `command:`/`skills_flag:` refuse; declared-by names the source → R-runtime |
| 0022 shared-file single writer (1.5K) | `gates.go` NOTES arm, AGENTS.md, `memoryland.go`, `docs/notes.d/`; 7/5, 1 string (installed hook) | KEEP | a path-limited commit narrows by file never by writer; NOTES.md fragments; the NOTES arm → 0022 |
| 0023 L3 identity-then-probe (1.4K) | `gates.go` probeL3Hooks/chain, `hookfresh.go`, `parity.go` applyL3Probe; 8/18, 0 strings | **MERGE INTO R-gates** — it is the L3 row's probe doctrine; non-goals 2 and 3 are decided (0002's escape C, 0038) | L3 realized iff identity AND behaviour of our own render; the launcher never execs the on-disk hook; foreign identity degrades named → R-gates |
| 0024 work-product routing (1.6K) | `visibility.go` genres/OpsPatterns/identity, `gates.go` three hook checks; 24/15, 5 const strings, 4 pins | KEEP · AMEND — D4 recorded as done; say plainly the ceilings are 0018 prose, not a shipped default | public iff any deployer could have written it; three commit-hook checks; restate-and-cite → 0024 |
| 0025 enforcement class (1.6K) | `parity.go` EnforcementClass/RealizedGate, `refusalfold.go`, `cage.go` spool; 26/14, 4 strings | **MERGE INTO R-gates** — 0002 already restates §1 and §3; §1's "session meta prints the class" is unbuilt (carry as unbuilt or strike) | every realized gate is `enforced` or `cooperative`; the push effect note; one writer for refusals → R-gates |
| 0026 research spikes (1.7K) | one comment, `dispatch.go:1518`; 1/0 | KEEP · AMEND — §5 becomes a citation of 0005 §2 (ranger-base-rs8j edited both copies in lockstep, the drift is one bead away); §1–§4 stay as the practice page | four triggers; two shapes; sourcing; half-a-bead cost rule → 0026 (mechanics → 0005 §2) |
| 0027 monica pulse (1.0K) | `pulse.go`, `watch.go` goroutine; the live instance sets no `pulse_interval:` so nothing pulses today; 9/4, 0 strings | KEEP · AMEND — §1 stamped superseded by 0029 §1–2; §2–4 stand (0033 leans on them) | delivery idle-only with the `Pulse check:` prefix, renag backoff, default target `coordinator:`, sets no crew mark → 0027 |
| 0028 rolling seats (1.5K) | `dispatch.go` Refill/refire/seatMap, `epoch.go`, `autoreap.go`, `seatidle.go` (obs.1 labels test-pinned); 64/24, 13 strings, 2 pins | **SUPERSEDE BY R-dispatch** — one story with 0011 and 0020: how a pass hires | a long-lived Run; settle re-runs the fire path under the flock; the epoch denominates budget and `-n`; one throttle in the watch process → R-dispatch |
| 0029 governance surface (3.2K) | `govern.go` G1–G9, `pause.go`, `posse status`, cockpit block; 12/5, 2 strings | KEEP · AMEND — executed sequencing and the G6 "until then" clause struck; record that 0027 §1 is superseded here; rule on 0036's tenth row (the table says closed at nine) | facts computed, decisions are beads, pause is a human speech act; nine G-rows; coordinator SLA → 0029 |
| 0030 orphaned claim (1.0K) | `dispatch.go` orphanedClaimLine, `herdrback.go` CrewHolder; 5/5, 0 strings | **MERGE INTO 0008** — its decision is 0008 §2's second amendment verbatim | (in 0008) |
| 0031 init operator fence (1.2K) | `herdrback.go` RHQ_LAUNCH_HOME, `init.go` refusals (pinned by number); 6/14, 5 strings, 1 pin | KEEP | init refuses the home it was launched from; keyed on the target, no PID deny → 0031 |
| 0032 engine onboarding (2.0K) | `parity.go` assumedUntilProbed, `runtimeprobe.go`; 16/7, 1 string | **MERGE INTO R-runtime** — the 66e2 preamble is dead; §3's API-only ruling carries | template denies are Degraded until a recorded probe; four observables; API-only never satisfies the contract → R-runtime |
| 0033 coordinator not a lane (1.5K) | `dispatch.go` isCoordinator/laneFor, `pidcheck.go`, `govern.go` G9; 12/11, 11 strings, 4 pins | KEEP · AMEND — the two "unbuilt / until that lands" clauses (§1, §4) | `coordinator:` in config; Route never returns it; refusal at hire time; G9; push-grant drift alarm → 0033 |
| 0034 codex plan hint (1.3K) | `planhint_codex.go` (no caller); D3–D5 nothing live; 8/1, 0 strings | **MERGE INTO R-planguard** — D1/D2 as the hint-type appendix; D3–D5 are unbuilt and the grok premise died a day later (`grokpool.go` exists); ruling item: build D3–D5 as a bead or drop them | a hint informs and never gates; windows named by duration → R-planguard |
| 0035 mode second layer (1.2K) | nothing live: D2's `-c approval_policy=never` absent from the codex template, no bead landed; 0/0 | **MERGE INTO R-gates** — D1 as one rule line, D2 as the codex template row, D3/D4 as a Lineage note; ruling item: build D2 or drop it | no posse-written file in a foreign config home; a second layer rides the typed line only → R-gates |
| 0036 posse backup (2.5K) | nothing live: no `backup` symbol, no age dependency, no config key; no build bead found; 0/0 | KEEP · AMEND — status gains `· unbuilt: <bead>` (§3.5); ruling item: cut the build bead or RETIRE; resolve the tenth G-row against 0029 before §6 | (unbuilt) backup is a harness verb; refuse on any remote; freshness enforced on-box → 0036 |
| 0037 venue-restricted runtime (1.1K) | nothing live; its MEASURED claim about `runtimeYamlKeys` is stale (both keys present); 0/0 | **MERGE INTO R-runtime** — one paragraph: dimensions are harness material, facts are instance material; the venue ruling lives in the instance tree | (in R-runtime) |
| 0038 git identity write-deny (1.1K) | nothing live: `seatbelt.go` has no `git-path config` deny, `cage.go` no common-dir binds; no build bead found; 0/0 | KEEP · AMEND — status gains `· unbuilt: <bead>`; 0014 §4 and 0023's non-goal point here | (unbuilt) the cage denies the repo's persistent git identity at L2 and L4; the transient redirect stays cooperative → 0038 |
| 0039 model dial follow-through (2.3K) | D1 → ranger-base-per37, D2 → ranger-base-ight8, D3c → ranger-base-ksmmz (ruled on v1p66), D3d → spike au0o4; D3a partial; 0/0 | KEEP · AMEND — status: D3c accepted per the v1p66 ruling; `· unbuilt:` names the three beads | the built-in tracks the dial; `runtimes/` promoted; a reading demotes only inside its lease → 0039 |

Cross-cutting, under every disposition: 15 records cite `internal/rhq/` (41 occurrences) — one mechanical sweep; and 0016's "ADR 0013 (monica pulse)" means 0027.

## §2 Target shape

After this ADR executes, 27 records are in force (23 kept, 4 new), 16 are superseded or merged and stay in the tree as history with a pointer. Word count falls from 95.8K to roughly 68K (ASSUMED: the four new records at 4.5K, 2.5K, 4K, 3K; the 0015 and 0012 trims at 4.3K). The four new records and what each absorbs:

| new record | absorbs | strings to move | pins |
|---|---|---|---|
| **ADR 0041 — The wall** (layers L0–L4, parity, gate shell, L3 probe, enforcement class, foreign config homes) | 0002, 0009, 0023, 0025, 0035 | 29 (0002:10 · 0009:13 · 0023:2 · 0025:4), plus the two installed hook bodies and the gate-shell header that re-render on the next install | 0 |
| **ADR 0042 — What a runtime is** (contract checklist and verdicts, per-key overlay, onboarding probe, venue rule) | 0017, 0021, 0032, 0037 | 9 (0017:5 · 0021:3 · 0032:1) | 0 |
| **ADR 0043 — The pass** (bd is the queue, one launcher, prune, run record, lanes and seats, rolling refill, epoch) | 0011, 0020, 0028 | 19 (0011:6 · 0028:13) | 7 Errorf/assert sites (0011:2 · 0028:5) |
| **ADR 0044 — The plan guard** (ladder, overflow cap and ledger, blind and headroom, local meters, hints) | 0010, 0018, 0034 | 2 (0018) | 2 (0018) |

Grouped by concern, the set in force (the index a `docs/adr/README.md` will carry once the merges land):

- **runtime & gates** (5): 0041 the wall · 0042 what a runtime is · 0014 path-scoped writes · 0022 single writer · 0038 git identity (unbuilt)
- **dispatch, seats & oversight** (8): 0043 the pass · 0013 runtime dispatch contract · 0008 crew sessions (with 0030) · 0004 cockpit · 0016 herdr hints · 0027 pulse delivery · 0029 governance surface · 0033 coordinator not a lane
- **money & meters** (3): 0003 model tiering · 0044 the plan guard · 0039 model dial follow-through
- **constitution & promotion** (5): 0012 harness/instance boundary · 0015 constitution promotion · 0024 work-product routing · 0031 init operator fence · 0036 backup (unbuilt)
- **credentials** (1): 0019 credential seam
- **personas & handoffs** (5): 0001 PIDs · 0005 work prompt and rungs · 0006 handoff shapes · 0007 skills · 0026 research spikes

Two calls the operator may want to make differently, priced: keeping 0018 out of 0044 (it is fresh and fully built) leaves the blind rule in two records, which is the assembly this ADR exists to end, at a saving of two strings and two pins; keeping 0002 as the root and merging only 0009/0023/0025/0035 into it saves nothing, because the root's eight appended amendments are the cost.

## §3 Numbering and supersession convention (forward-binding)

1. **Append-only.** No numbered record is ever deleted or renumbered.
   A consolidation mints the next free number and carries a *Lineage*
   table: one row per decision it absorbs, `was: ADR 0002 §3 L2 → here
   §1 L2`. The absorbed record gets ONE edit: its status line gains
   `· superseded by ADR 004n (date)` and a one-line pointer under the
   title. The body is untouched — it is history, and 0012's App.A rule
   already showed what a restatement that drops an appendix costs
   (a status line citing a section that no longer exists).
2. **An old citation resolves in one hop.** `ADR 0002 §3` in a code
   comment lands on a record whose first visible line says where §3 went.
   That is why re-pointing is a cost lever (§4), not a correctness
   requirement.
3. **Amend in place, never by appending.** An amendment edits the
   decision's own paragraph and stamps the status line — the practice
   0039 follows. Appended `## Amendment <date>` sections (0002's shape)
   are what made a rule assembly work; none are added after this ADR. A
   measured finding that changes no decision goes to `docs/notes.d/` or
   the bead, not the record.
4. **A rule lives in one record.** When a decision's binding text must be
   assembled from more than one record (a section plus a sibling's
   amendment of it, or an appended amendment section), the next review
   consolidates it. In-place amendments with a status stamp do not count:
   0013 carries six and reads as one text; 0002's eight appended sections
   and the blind rule's three homes (0010 §5, 0013 §3, 0018 §1) are what
   §1 disposes of.
5. **A record is accepted-unbuilt only with a bead.** An accepted design
   with nothing live under it carries `· unbuilt: <bead id>` in its
   status line; the adherence audit reads that as "unrealized", not
   "drifted". Without a bead it is a RETIRE candidate at the next review.
6. **Numbers are repo-local from 0013** (HISTORY.md, unchanged). Cross-
   repo references go by title, never bare number.

## §4 The price, both ways

**Doing nothing (the status quo), MEASURED:** the reader cost is 96K
words per full read — about 130K tokens, larger than a persona's working
context beside a task, or 6.4 hours for a human at 250 wpm. Nobody pays
it, so the real cost is the one the rename showed: drift that no reader
catches (41 dead package paths, a status line citing a missing appendix,
a "set half open" clause for a bead that landed, two runbook pointers to
a file that does not exist). Every future amendment lands on the largest
records, so the assembly cost compounds.

**Consolidating, priced:**

| item | cost | status |
|---|---|---|
| writing the concern records (§2) | one design-then-build pair per merge group; each consolidated record ~3–5K words, absorbing 8–25K | ASSUMED — the count of groups is measured, the words per record are an estimate |
| stamping the absorbed records | one status-line edit each, mechanical | MEASURED — count in §1 |
| re-pointing RUNTIME-VISIBLE strings | 141 lines in 37 files, but only the records a merge supersedes: per record 0002:10 · 0009:13 · 0014:20 · 0025:4 · 0023:2 · 0013:35 · 0011:6 · 0028:13 · 0033:11 · 0015:42 · 0003:10 · 0018:2 · 0012:16 · 0019:18 (the rest ≤6). A refusal that names a superseded record still resolves (§3.2) but reads stale to the operator, so the merge bead moves the strings of the records it absorbs and nothing else | MEASURED — `grep -P '"[^"]*ADR[ -]?0\d{3}[^"]*"'` over non-test Go |
| the 9 test assertions on a number | `agents_test.go:670` (0006), `credseam_test.go:280` (0012 D4), `pathscoped_test.go:301` (0014), `refillreport_qa_test.go:183,216`, `runtimecheck_test.go:194`, `initmanifesth7cd_qa_test.go:88,115` (0015), `cmd/posse/coordinatorparity_test.go:101` — each follows its string | MEASURED |
| re-pointing the constitution (11 PIDs, 49 cites; ORDERS 61) | one bead, one promote — the PIDs are one promoted set, so they move together or not at all | MEASURED count |
| re-pointing COMMENT citations eagerly | 1,938 sites in 291 files; ~40 files a session → 7 sessions of pure churn, plus every test golden that quotes a comment | ASSUMED rate; the count is measured |
| re-pointing comment citations lazily (chosen) | zero up front; a comment is re-pointed when its file is next edited, and §3.2 keeps the old cite resolvable meanwhile | — |
| `NOTES.md` and `docs/adr` cross-cites | never re-pointed: a journal cites what was true when written | — |

**The decision on price:** consolidate the merge groups in §2, re-point
the absorbed records' runtime strings and the PID set in the merge beads, and leave
comment citations to lazy re-pointing under §3.2. The eager rewrite buys
nothing a reader can measure — the old number resolves in one hop — and
costs seven sessions of churn.

## Alternatives rejected

- **Do nothing; add an index.** A `docs/adr/README.md` mapping concern →
  records would cut the search cost without touching any record. Rejected
  as the whole answer because the index does not shrink the assembly
  cost: 0002 L3 is still seven passages in four files. Kept as a part:
  §2's concern list IS the index, and the merge bead for each group
  writes it.
- **Rewrite everything into one constitution document.** One 20K-word
  record, decisions only, with the history behind it. The clever one. It
  is a single writer's file (ADR 0022) that every persona amends, so it
  serializes all design work behind one lock; and a 20K-word file
  amended weekly re-grows the assembly problem in one place. Rejected.
- **Eager re-pointing of all 1,938 citations in the merge beads.**
  Priced in §4: seven sessions and a golden churn, for a reader gain of
  zero hops. Rejected in favour of §3.2 + lazy.
- **Retire every record with no live citation.** Would retire 0036
  (accepted three days ago, two operator rulings inside) and 0037
  (a boundary ruling whose whole point is that nothing here names the
  venue). "Nothing cites it" is not "nothing binds": a citation counts
  code that exists, and 0036 governs code that does not yet. §3.5 is the
  rule instead: accepted-unbuilt with a bead, or retired at the next
  review.
- **Renumber the survivors 0001–00nn.** Breaks every citation at once,
  including the 141 runtime strings and the public restatement's root
  commit that `seedpub_qa_test.go` pins by path. Rejected; HISTORY.md
  already says numbers are repo-local and stable.

## Consequences

- The table is a proposal. The ruling is one question bead
  (`-l question`, ADR 0006): accept / amend rows / reject groups; every
  merge bead below is dep-blocked on it.
- The adherence audit (ranger-base-4wxko) uses the *Survivor* column as
  its checklist: one row per surviving rule, verdict per row.
- The 41 dead `internal/rhq/` paths are fixed in one mechanical bead
  regardless of the ruling — they are wrong under every disposition.
- Beads cut (ids on the bead's close comment): the ruling question; the
  `internal/rhq/` sweep (unblocked); one merge bead per new record (0041,
  0042, 0043, 0044) and one for the 0030→0008 fold; the 0015 trim; two
  AMEND sweeps (personas group; the rest) — all but the sweep dep-blocked
  on the ruling.

## Verification (predicted observables)

1. After the ruling and the merges: every record in §1 marked MERGE or
   SUPERSEDE carries `· superseded by ADR 004n` on its status line, and
   `grep -L 'superseded by' docs/adr/00*.md` lists exactly the KEEP and
   AMEND rows.
2. Each consolidated record has a Lineage table whose left column, joined
   over the group, equals the set of `§`/`D` labels the absorbed records'
   decision sections carried (a script in the merge bead checks it).
3. `grep -rn 'internal/rhq/' docs/adr` returns nothing.
4. No string literal in non-test Go names a record marked MERGE or
   SUPERSEDE in §1 (the string grep above, filtered to those numbers,
   returns nothing); the 9 assertions follow and the suite is green.
5. `go test ./internal/posse -run 'SeedPub|Publication'` still green — the
   root-commit pins are history and untouched.

## Measured versus assumed

| claim | status |
|---|---|
| the counts in Context | MEASURED — grep at 521d3db, commands on the bead |
| 15 records cite `internal/rhq/` | MEASURED — `grep -c 'internal/rhq' docs/adr/0*.md` |
| the 141 string-literal sites and the 9 test assertions | MEASURED — string-literal grep over non-test Go; assertion grep over `Contains/Equal/==` with a quoted number |
| every "governs" and "nothing live" cell | MEASURED — five reader passes, one per concern group, each grepping the symbol and reading the site; the `backup`, `approval_policy`, `git-path config` and `PromotedPaths` absences re-checked by the architect |
| no build bead exists for 0034 D3–D5, 0035 D2, 0036, 0038 | MEASURED — title and description search over every bead, open and closed, 2026-09-01 |
| the word count after execution (≈68K) | ASSUMED — the new records' sizes are estimates |
| words per consolidated record, sessions per 40 files | ASSUMED — replaced by the first merge bead's actuals |
| 96K words exceeds a persona's working context beside a task | ASSUMED from the token estimate (≈1.35 tokens/word); no session was measured reading the set |
