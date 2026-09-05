# ADR 0039 — Model dial follow-through: the built-in tracks the dial, `runtimes/` joins the promoted set, the catalog says its age and rules only inside its lease

*Status: accepted 2026-09-01 (D1, D2, D3a, D3b, and D3c per the
operator's ruling on ranger-base-v1p66) · D3d spike answered 200
(ranger-base-au0o4), D3d built 2026-09-05 (ranger-base-mvrke),
amended the same day (ranger-base-q3n4e: the session credential a posse
process reads is the env set under the home, selected by the launch's
set list — never its own environment) and the amendment built the same
day (ranger-base-abgil, the seam and the helper; ranger-base-hr49g, the
launch's list through the preflight) · owner:
architect · amends 0003
§1 (the claude strong cell), 0015 §2/§3 (the promoted set), 0021 (the
overlay's home) · from ranger-base-1ykc1, discovered from
ranger-base-c3vqe*

> 2026-09-01: strong moved to `claude-fable-5-1` by the operator's
> word, and "model bumps will be a frequent occurrence". The bump took
> three writes: the ADR 0021 overlay key (the sanctioned dial), a
> mirror commit in the constitution repo that promote does not carry,
> and a HAND-EDIT of `state/model-catalog.json` because the availability
> probe has answered 401 since 2026-08-31 and the retained snapshot did
> not list the new id. This ADR makes the next bump one write.

## Context

Three facts, each read in code this session:

- **The built-in trails the dial.** `claudeModels[strong]` is
  `claude-fable-5` (runtime.go); `PriceTable` has no `claude-fable-5-1`
  row (cost.go) and prices it by the `fable` family fallback. The
  overlay (`runtimes/claude.yaml: model_strong:`) wins at every launch
  that reads it, so the built-in only matters to an instance with no
  overlay — a rebuilt home, a fresh instance, a test.
- **The dial lives where promote cannot reach.** `PromotedPaths` is
  `agents, config.yaml, recipes, skills` (promote.go). `runtimes/` is
  read at every launch (`LoadRuntime`, `RuntimeMapsTier`), is written by
  no code path (every reference to `RuntimesDir()` is a read — MEASURED,
  grep), holds no secret (`cage_cred:` names a variable, not a value),
  and is "the operator's own config root, the same trust level as
  `config.yaml` and the PIDs beside it" (ADR 0021's own words). It is
  the one launch-read fact at the home that no manifest attests to. The
  constitution repo already carries it (ranger-base commit 55c5581,
  `rhq/runtimes/claude.yaml`); the home copy was placed by hand.
- **A stale reading rules as fact** (ended by D3c, ranger-base-ksmmz;
  what follows is the state this ADR was written against).
  `ModelCache.Models` (modelavail.go)
  returns a snapshot inside `model_probe_ttl` as known; past the TTL it
  re-asks, and when the ask fails it returns the retained reading
  *still as known* (`kept`). `TierPreflight` then demotes any tier whose
  id that reading lacks. The reading ruling on today's launches was
  taken before the id existed; the probe cannot refresh it because the
  meter credential it reads (keychain, ADR 0019 D2) rots in hours
  (ranger-base-wkai3). The file's own preamble — "only a list that was
  actually read, and that does not contain the wanted id, demotes" —
  was written for a reading whose age was bounded by the TTL; nothing
  bounds it once the probe is down.

## Decision

**D1. The built-in tracks the dial; the price row is exact.**
`claudeModels[TierStrong] = "claude-fable-5-1"`, `PriceTable` gains
`"claude-fable-5-1": {10, 50}`, ADR 0003 §1's claude strong cell reads
`fable-5-1`. The rate is MEASURED against the bundled claude-api skill's
current-models table (cached 2026-06-24): Fable 5.1 lists at the Fable 5
rate — the same `{10, 50}` in/out row — so the family fallback was
already right and the row is what ADR 0003's "exact ids" comment asks
for. Rule for every bump after this one: **the overlay moves first**
(one line, no rebuild, no promote until D2 lands — after D2, one line
plus `posse promote`) **and the built-in follows in the next posse
release**. An overlay that matches the built-in is not stale, it is
redundant, and the grid still names it per key (ADR 0021 §3); the
operator may drop the key or keep it.

**D2. `runtimes` joins `PromotedPaths`.** One token in one list
(promote.go); every consequence below is a reader of that list and
needs no new code, which is the point of the list. The home's
`runtimes/` becomes prose in force on the same terms as `config.yaml`:
written only by `posse promote`, hashed into `promoted.json`, refused
to every session's writable set, and walled from persona commits in the
constitution repo (`rhq/runtimes`). The versioned copy that 55c5581
made by hand becomes the source, and the mirror ritual ends.

Consequences, each derived from a named reader:

- *promote (copyPromotedSet / promoteRemovals):* carries
  `rhq/runtimes/*.yaml` to the home and **removes** any home
  `runtimes/*.yaml` the commit does not carry, printing each removal. A
  template-only runtime placed at the home by hand (ADR 0002; the
  in-flight `runtimes/bob.yaml`, ranger-base-6wqe) must be committed to
  the constitution before the next promote or it leaves, loudly. This
  instance: the home holds only `claude.yaml`, and 55c5581 carries it —
  MEASURED 2026-09-01, `ls ~/.config/posse/runtimes`.
- *launch verify (VerifyPromoted, herdrback.go):* a home whose manifest
  predates this change (this instance's is at a27973f and names no
  `runtimes/` entry) reads `unpromoted runtimes/claude.yaml` the moment
  the new binary is installed, and **every dispatched launch refuses
  until `posse promote`**. That is the fence doing its job, and it is
  free here: the reinstall the bead's item 4 already waits on is
  operator-gated and promote is its second half. The ritual is `make
  install && posse promote`, stated in CHANGELOG under Upgrading.
  `posse promote` itself does not run the verify (MEASURED: its callers
  are the launch and the first-run repair only), so the ritual cannot
  deadlock.
- *seatbelt (HomeConstitutionPaths):* `~/.config/posse/runtimes` in no
  session's writable set; `posse gates` prints ConstitutionGrants, so
  the observable is that the dir appears in no grant.
- *commit wall (ConstitutionRepoPaths, constitutionClassSpec):* the
  rendered hook names `rhq/runtimes`; the spec pin in
  constitutionwall_qa_test.go says in its own comment that adding a
  path reds it and the fix is to add it there "having decided it
  belongs" — this ADR is that decision.
- *first run (init.go promotedRel, HashPromotedSet):* the seed tree has
  no `runtimes/`, so nothing is seeded; a home without the dir hashes
  nothing for it (`os.IsNotExist → continue`), so a fresh instance is
  not marked missing.
- *docs:* README:112, INSTALL:507, ADR 0015 §2/§3 enumerate the set in
  prose — one word each plus a dated amendment line in 0015 pointing
  here. ADR 0021's "lives at the home, no promote needed" reading gains
  the same line.

**D3. The catalog says its age, can be re-read on demand, and rules
only inside its lease.**

- *D3a — the age is in the sentence (decided).* Whenever the reading a
  verdict rests on is older than `model_probe_ttl`, both the launch's
  loud line and `posse gates`' PreflightReport say so, with the probe's
  outcome. `Models` already computes the age (`catalogAge`) and holds
  the error it just logged; it returns them beside the bool instead of
  dropping them. No new state, no new file. An operator who reads that
  line knows to refresh a credential, not to edit a state file.
  *Amended by D3c (ranger-base-ksmmz):* a verdict now only ever rests on
  a reading INSIDE its lease, which has no age worth reporting — the
  operator set that number — so the age moved onto the UNKNOWN line the
  lease rule prints, and the `unavailable per the catalog read 2d ago`
  form this bullet first specified never renders. Same two facts, said
  where they are now true.
- *D3b — `posse runtimes` carries the availability line, and `--probe`
  re-reads (decided).* Under each runtime posse can read a catalog for
  (`anthropicAPI`, egress-keyed), `posse runtimes` prints
  PreflightReport for each mapped tier (persona "", tier keys only —
  the form the function already supports) — this is the bead's
  acceptance surface, and it is where "is 5.1 there" gets answered
  without a launch. `--probe` calls `Models(0)`: "fresh only" is what
  `maxAge 0` already means in plancache.go, and a 429 cooldown is still
  honoured on that path (Models checks `RetryAt` before asking), so a
  forced read cannot become the rangerhq-tdy8 storm. No config edit,
  no launch, no hand-edit.
- *D3c — the lease rule (RULED 2026-09-01, option 2 on
  ranger-base-v1p66; built in ranger-base-ksmmz).* A retained reading may **demote**
  only while it is inside its lease: `now − at < model_probe_ttl`. Past
  that, with the refresh failing, it is quoted (D3a) but the verdict is
  UNKNOWN: the launch takes the tier as asked, and when the wanted id
  is absent from the stale reading the line still prints —
  `… not in the catalog read 2d ago and the probe is failing (401);
  availability UNKNOWN, launching as asked`. This is the preamble's
  rule with its bound written down, and the bound is the number the
  operator already owns; the cooldown keeps governing *whether posse
  re-asks*, never *whether it trusts* (a 429 storm can renew a cooldown
  all day, c3vqe, and would otherwise renew trust in a day-old list).
  What it gives up, honestly: an account that loses the strong model
  *while the probe is down* launches on the id the CLI cannot serve,
  and what the CLI does then is ASSUMED unmeasured (rangerhq-oay's
  fear: it quietly serves a fallback). Bounded by the line above on
  every such launch and by `posse cost`'s TierForModel, which reads the
  transcript. The alternative — today's rule — demotes the whole shop
  on every bump for as long as the probe is dead, which on this
  instance is most hours (wkai3), and the only cure is a hand-edited
  state file with a `.bak` beside it.
- *D3d — the probe rides the credential the launch is about to hand the
  session (spike-gated).* `MeterToken("claude")` reads the keychain
  because the *usage* endpoint refuses a minted session token
  (planusage.go: 403, "a setup-token never will be"). `/v1/models` is a
  different endpoint and its answer to that token is unmeasured. If it
  answers 200: `ModelLister.Token` prefers the session credential
  (ADR 0019's `Read(runtime, session)`, read from the PID's env set as
  the launch already does) and falls back to the meter store — the
  probe then asks "can this account run the id" of exactly the
  credential that will run it, and it rots on the same clock as the
  sessions, which is the right clock. The exposure is unchanged: the
  same pinned host the session egresses to (credpin.go). If it answers
  401/403: D3d is dead and the credential question stays wkai3's.

  *Built 2026-09-05 (ranger-base-mvrke), on the spike's 200.*
  `ModelLister` carries two named credentials instead of one: `Token` is
  the preference and `Fallback` is what answers when the preference does
  not. There are exactly two ways it does not and `List` answers each
  once — nothing to read, which spends no request, and a credential the
  endpoint refuses (401 or 403), which spends one more read of the
  catalog and never a loop. The fall-through is skipped when the fallback
  is already what was presented, so the arm this probe has been failing
  on does not double its traffic. `App.ModelCache` wires the session half
  through the ADR 0019 seam and leaves the bare constructor on the meter
  store, so every test that injects a `Token` is untouched; `ModelCache`
  still decides whether a read happens at all, so the bound that matters
  is per READ and not per launch. Pinned in
  internal/posse/modelsessiontoken_test.go.

  *Amended 2026-09-05 (ranger-base-q3n4e, from ranger-base-mvrke).* The
  spike answered 200 (ranger-base-au0o4, 2026-09-02: eleven ids, the
  strong id among them, three control arms 401), and the build that
  followed it is right and inert: "read from the PID's env set as the
  launch already does" was realized as the seam's session half, which is
  `os.Getenv` of the name `CageCredential(rt)` gives — a read of THIS
  process's environment. That sentence is true of a launched runtime and
  of no posse process, and the probe runs in a posse process. MEASURED on
  this instance (q3n4e): the mint is in two env sets under the home;
  sibling variables from the same set reach a dispatched session; the
  mint does not — absent from `os.Environ`, not empty — and no shell rc
  exports it, so the launcher's environment has none either. So the
  probe fell through to the meter store on every read, the store D3d
  exists to stop depending on.

  **Ruled: the store is the env set under the home; the launch names
  which.**

  1. *Where.* ADR 0019's session half reads the env set FILES under the
     home's `EnvsDir` — the store of record D1 of that ADR already names
     ("posse-owned, store of record is the home") — and never the
     process environment. The environment arm is retracted, not kept
     beside the file: when this was written the seam's session purpose
     had zero callers outside `credential.go` (grep) and it has exactly
     one now — `sessionCatalogToken`, the probe this amendment is about,
     landed with ranger-base-8bp2j — every posse surface that
     asks about the session credential already reads the files
     (`sessionRows`, `ExpiringCredentials`, `sessionExpiry`), and the
     one process that ever holds the value in its environment is the
     runtime, which scrubs it from its children. An arm no caller can
     satisfy is not a second store; it is a sentence that reads true and
     answers nothing.
  2. *Which set.* The caller names the sets, in launch order — the list
     `planLaunch` already computes (explicit `--env-file` and recipe
     sets, then the PID's `envs:`, else config `default_env` for a
     persona-less caller, rangerhq-f2b), hoisted into one helper that
     both the launch and the preflight call. Set NAMES are in hand at
     the preflight (`ag` and `o.Envs` are loaded a hundred lines above
     it); only the VALUES were resolved after it, and the seam reads
     those itself at the moment of the probe. The value is the LAST
     assignment of the name across that list in order — the rule
     `readStamps` already ascribes to a launch within one file, extended
     across files. `Read(runtime, session)` keeps its signature and
     reads the persona-less list; a second entry point takes the
     launch's list. One reader underneath, no new acquisition path, no
     new host: credpin stays on the endpoint's host.
  3. *The probe.* `TierPreflight` hands the launch's sets to the catalog
     read, so the lister's preferred credential is the mint of the sets
     THIS launch will realize, and the meter store stays the fallback
     the D3d build already wired (nothing to read: no request spent;
     401/403: one extra read, never per launch). The cockpit callers
     (`posse runtimes`, `posse gates`) have no persona and get the
     persona-less list. The reading is shared across personas within
     its lease exactly as before; the credential that refreshed it is
     the launch's that found it stale.

     BUILT as `TierPreflightFrom` → `ReadCatalogFrom` → `ModelCacheFrom`
     → `sessionCatalogToken(sets)` (ranger-base-hr49g), with `planLaunch`
     computing the list once above the preflight and handing that same
     list to the `vars` loop below it. The persona-less list has one
     spelling, `cockpitEnvSets`, which is what `ReadCatalog`,
     `ModelCache` and `ReadCredential(rt, CredSession)` all read on. An
     EMPTY list is an answer and not a request for a default: a persona
     that names no env set realizes none, and the probe does not borrow
     `default_env` for it (rangerhq-f2b).
  4. *The exposure question, answered.* Who can READ the file does not
     change: below the container tier any same-uid process reads a
     mode-600 file (ADR 0019, "The trade, plainly"), and ranger-base-au0o4
     MEASURED a seatbelt session reading the default set's value to
     run its probe. What changes is which process PRESENTS it: posse's
     launcher, to the pinned host the session itself egresses to. The
     value is never logged — `ModelLister`'s errors are generic and the
     fake endpoint asserts the header's value without printing it.
  5. *Cost.* One env-set read per catalog refresh, which happens once
     per lease, not per launch; the launch reads the same files again
     for `vars`, a second read of a small file. Nothing new is held
     hostage; the exit hatch from the whole decision is the fallback arm
     that runs today.

  Landing order: when this amendment was written the D3d build named
  above was in gwart's session tree and on no ref main could see, so the
  beads it cuts waited on ranger-base-mvrke landing rather than on the
  sentence saying it is built. It landed with ranger-base-8bp2j, whose
  whole subject was this file: the merge-back was refused on the ADR and
  never on the code, `internal/posse/modelavail.go` being untouched
  between the two. That wait is over; the ruling's own beads are not
  built by it.

## Alternatives rejected

- **(q3n4e a) Export the mint in the launcher's environment.** Zero code
  and it makes the old D3d sentence true. Rejected: "the launcher" is the
  operator's cockpit shell and the dispatch loop, so the export reaches
  every process the operator runs and is readable off any of them with a
  process walk; an env set is an explicit per-persona choice and never a
  silent default (rangerhq-f2b), and this makes it the box's default;
  and the seatbelt profile is rendered from that environment (ADR 0019
  D2), which is the wrong place to widen. Unpriced against nothing: it
  also leaves the seam reading an arm no posse process holds.
- **(q3n4e c, pure) Plumb the launch's resolved `vars` above the
  preflight.** Faithful to "the credential the launch is about to hand
  this session" and the same hoist as the ruling — but it reads every
  set's VALUES on every launch to feed a probe that asks once per lease.
  Names cost nothing to hoist; values are read where they are used.
  Kept as the selection rule, dropped as the mechanism.
- **(q3n4e d) Retract D3d.** Leaves the probe on the meter store's clock
  — MEASURED 8h from the operator's last interactive run (ranger-base-
  9jjhc), unreadable from every seat — so the one-write bump this ADR
  exists for stays three writes on most mornings. D3a/D3c bound the
  harm; they do not deliver the goal.
- **First set in sorted order that holds the name.** No plumbing at
  all. Rejected: sorted order puts the container set before the default
  one, and `sessionRows` already refuses to break that tie by guessing;
  a probe must not guess where the report will not.
- **A config key naming the probe's set.** A fact belonging to no lever
  posse holds (ADR 0019's own rejection of the same shape): it drifts the
  day the PID's `envs:` changes, silently.
- **Document `runtimes/` as deliberately unpromoted.** A rebuilt home
  launches the trailing built-in with the grid saying "built-in
  default" and nobody told; a launch-read fact with no manifest entry
  is ADR 0015's 6ne hole one directory over; and the mirror commit
  becomes a ritual with no mechanism behind it. Priced today: the
  mirror was made by hand, uncommitted for a day. MEASURED.
- **A "soft" promoted member — copied, not verified.** Two classes of
  promoted path is a second list, and the verify's value is that a
  byte at the home is either attested or refused.
- **`model_strong:` in `config.yaml` instead** (already promoted). ADR
  0021 rejected the second place to say the same thing; `declaredBy`
  reads the runtimes file.
- **Only a forced-refresh command (the bead's option a).** Forcing a
  dead probe prints the same stale list; it helps the operator *see*
  and cannot heal. Kept as D3b because seeing is worth having; not
  enough alone.
- **A second number — `model_probe_stale_max:` grace past the TTL.** A
  dial nobody measured, on top of one the operator already set to say
  how fresh a reading must be. If the TTL is too short for the lease,
  lengthen the TTL.
- **Lease extended by the cooldown** ("the endpoint told us to wait, so
  the reading is still good"). Trust and re-ask are different
  questions; c3vqe's day of renewed 429 cooldowns shows the coupling
  renewing trust in a reading nobody could refresh.
- **Keep the hand-edit runbook (the bead's option c).** A state file
  edited by hand under a launcher that rules on it as fact — the shape
  every other ADR here exists to remove, with a `.bak` sitting beside
  it as the evidence.
- **The clever one: the launch retries the probe with the session
  credential AND treats its 401 as "the session will fail too, refuse
  the launch".** Turns the preflight into a gate, which rule (3) of the
  file forbids; a degraded launch is the operator's call, and this
  would make one credential's freshness refuse the whole shop.

## Verification (predicted observables)

1. After `make install && posse promote`: `posse runtimes` prints
   `strong: claude-fable-5-1` for claude attributed to
   `runtimes/claude.yaml (model_strong:)`; `promoted.json` `files` has a
   `runtimes/claude.yaml` entry; `posse gates` lists the runtimes dir in
   no grant. A strong-tier launch's `state/launch/<session>.sh` carries
   `--model claude-fable-5-1` and no `fallback:` line in the meta.
2. Before that promote, on the new binary: a dispatched launch refuses
   with `unpromoted runtimes/claude.yaml`; `posse promote --dry-run`
   lists the file arriving.
3. With the overlay key removed: the same `posse runtimes` line reads
   `built-in default` (D1).
4. With the snapshot's `at` set two days back and the lister faked to
   401: `posse gates` prints the age clause (D3a); `posse runtimes
   --probe` logs one `preflight failed` line and prints the same clause
   (D3b); under D3c the verdict reads UNKNOWN — `not in the catalog read
   48h00m ago and the probe is failing (…); availability UNKNOWN,
   launching as asked` — the launch script carries the asked-for id and
   the session meta gets no `fallback:` mark.
5. `go test ./internal/posse -run 'Constitution|Promote|Preflight|Model|Price|Tier'`
   green, with the constitutionClassSpec pin reading `rhq/runtimes`.
6. (D3d as amended, unit) PINNED 2026-09-05 (ranger-base-hr49g,
   `internal/posse/modellaunchsets_test.go`; the persona-less half is
   `modelsessiontoken_test.go`). With the name in no env set under a scratch
   home, the seam's session read returns the refresh-verb sentence and
   the lister spends no request; with two sets in the launch's list
   carrying different values, the lister sends the LAST one and the fake
   endpoint asserts the bearer equals it without printing it; with the
   session read refused by the endpoint (401), the meter store is read
   exactly once for that catalog read. The process environment carrying
   the name changes none of these — pinned with the variable set in the
   test process to a third value that must never be sent.
7. (D3d as amended, operator) With the keychain token stale (the meter's
   401 of ranger-base-wkai3) and the mint in the PID's named set,
   `model-catalog.log` on this home shows `ok models=N` after the next
   launch and `posse runtimes --probe` from the cockpit prints the
   availability line without the age clause. This is the row that
   decides whether the ruling worked; a green 6 without a green 7 is the
   old state with a new sentence.

   HALF-ANSWERED 2026-09-05 (ranger-base-hr49g), and the half that is
   open is the operator's to close. The stale-keychain precondition does
   not hold on this box today — `state/model-catalog.log` has read `ok
   models=11` hourly since 07:35Z, so the meter store is answering and a
   green line there would say nothing about which credential produced
   it. So the meter store was switched OFF for the measurement instead,
   which is the same condition and a stronger one: a binary built from
   this commit, reading the LIVE home's env sets over dinesh's own
   launch list (`[default]`) with the fallback made unavailable,
   answered `ok models=11` — `claude-fable-5-1` among them — and wrote
   that line to a scratch state dir; the cockpit's own line rendered
   `claude: tier strong → claude-fable-5-1 (available)`, no age clause.
   Controls in the same reading: the two env sets that carry no mint
   (`projA`, `glcc-box`) refused, and with the fallback off the catalog
   read over one of them spent no request and ruled on nothing. What is
   left is exactly the part a session cannot do for itself: the operator
   installs the build and launches once, and the LIVE log carries the
   line.
8. (D3d as amended, measure once) Two `--env` flags of one name on a
   pane's create: which value the pane holds. ANSWERED 2026-09-05
   (ranger-base-abgil, herdr 0.8.2 on this box): the LAST wins, so the
   ruling's assumption holds and the helper's rule does not flip. Two arms,
   because one arm of a positional claim measures nothing: `--env D=FIRST
   --env D=SECOND` gave the pane `SECOND`, and the same pair reversed gave
   it `FIRST`. The dump carried exactly one assignment of the name each
   time, never both. Controls in the same reading: a name passed ONCE was
   present, and a name never passed was absent — so the reading can report
   an absence and a "present" one is not an artifact of how it was read.

## Measured versus assumed

| claim | status |
|---|---|
| Fable 5.1 lists at the Fable 5 rate ({10, 50} per MTok) | MEASURED — bundled claude-api skill, current-models table cached 2026-06-24 |
| the built-in trails; the overlay wins per launch | MEASURED — runtime.go claudeModels, overlayBuiltin; live `~/.config/posse/runtimes/claude.yaml` |
| the constitution repo already carries the overlay | MEASURED — ranger-base 55c5581 `rhq/runtimes/claude.yaml` |
| nothing writes `RuntimesDir()` | MEASURED — grep, five readers, no writer |
| the retained reading rules as fact past the TTL when the refresh fails | WAS MEASURED — modelavail.go `Models` returned `e.Models, kept(e, have)` on error; ENDED by D3c (ranger-base-ksmmz): the bool is now `kept && withinLease`, the ids still go back to be quoted |
| the probe has 401'd since 2026-08-31 and the catalog was hand-edited 2026-09-01 | MEASURED — `state/model-catalog.log` |
| this home's manifest names no `runtimes/` entry and promote does not run the verify | MEASURED — `promoted.json` (sha a27973f); VerifyPromoted callers are herdrback.go:1351 and init.go:356 |
| `Models(0)` honours a live cooldown before asking | MEASURED — the `RetryAt` branch precedes the ask |
| what the claude CLI does with `--model <id the account cannot run>` | ASSUMED unmeasured — D3c's whole cost; one launch with a made-up id answers it (laurie's checklist) |
| `/v1/models` accepts a minted session token | MEASURED — ranger-base-au0o4 2026-09-02, 200 with eleven ids and three 401 control arms; one reading, three days before the q3n4e ruling |
| the mint's `/v1/models` bucket is not the starved one ranger-base-hs0dl found on the usage endpoint | ASSUMED — hs0dl measured a different endpoint; V7 is the live measure, and a 429 is UNKNOWN under D3c, never a refusal |
| the seam's session half has no caller that holds the value | MEASURED at the q3n4e HEAD — zero callers outside `credential.go`; the mint absent from a dispatched session's environment while sibling variables of the same set are present. At THIS HEAD there is one, `sessionCatalogToken` (modelavail.go), which is the D3d build the ruling above retargets and not a second store; the retraction argument is unchanged by it |
| the D3d build is landed | MEASURED — `ModelLister` carries a `Fallback` field at this HEAD (ranger-base-8bp2j replayed ranger-base-mvrke's commit past this amendment). It was unlanded when the amendment above was written, which is what that paragraph's landing-order note is about |
| a seatbelt seat can read an env set's value | MEASURED — ranger-base-au0o4 ran its probe from one on the default set |
| the launch's env set list reaches the catalog probe, and the session mint alone can read the catalog | MEASURED 2026-09-05 (ranger-base-hr49g) — the live home's `envs/default.env`, dinesh's own launch list, the meter fallback made unavailable: 11 ids, `claude-fable-5-1` among them. Pinned hermetically at every hop a fake endpoint can see, and at the two it cannot (`planLaunch`, `TierPreflightFrom`) by parsing the source |
| the two sets holding the name hold one account's mint | ASSUMED — both 108 bytes, values deliberately uncompared; a shared reading across personas already assumed one account before this ruling |
| the last `--env` of one name wins in the pane | MEASURED — V8, 2026-09-05, herdr 0.8.2: `FIRST` then `SECOND` gave the pane `SECOND`; reversed it gave `FIRST`; one assignment of the name in the pane's environment either way, with a passed-once and a never-passed control |
| no session on any instance needs to write `runtimes/` | ASSUMED — no code writer; a hand edit at the home is exactly what D2 ends |
