# ADR 0019 — Credential provider seam: posse owns scoped mints under the home; rotating runtime credentials are read where the runtime keeps them

*Status: proposed 2026-08-26 · richard · bead ranger-base-x6ic ·
extends ADR 0012 D4 (plan-guard seam) and ADR 0018 (blind policy);
supersedes, at the harness level, the "guard does not become portable
off macOS" consequence of the instance-private credential ADR of the
same number. Amended 2026-08-28 (ranger-base-1lza): darwin counts
three stores, not two, and the `.stale-*` evidence clause was measured
dead — the file self-renews. Amended 2026-08-28 (ranger-base-swqk):
D5/V6 split the expiry surfaces by purpose — the timer surfaces carry
session mints only; the meter's expiry is report-only. Amended
2026-08-29 (ranger-base-5fly): the "`secrets/` directory now"
rejection is overturned by the instance page's acceptance — the dir is
built as a store, empty; this page's seam stays the only acquirer.
Amended 2026-09-01 (ranger-base-v3qi4): darwin's store of record is the
runtime's own `keychain-with-plaintext-fallback` composite, MEASURED in
the release binary — store 3 is its fallback, not a byproduct, and the
darwin adapter now mirrors the composite's read order (keychain item,
then the file only on item-not-found).*

## Context

The plan guard and the model-availability preflight read the Claude
Code OAuth access token with `exec.Command("security",
"find-generic-password", …)` — unconditionally. MEASURED in this tree:
`planusage_anthropic.go:225-243` (moved there from `planusage.go:216`
by 66ed579, the plan-window seam split) has no build tags and no
`runtime.GOOS` check;
`modelavail.go:128` is the second caller of the same function. On any
non-macOS build that exec fails executable-not-found and the error says
`keychain item … unreadable` — a message about a keychain, on a system
that has no keychain, forever. Under ADR 0018 that reads as *blind*,
and with the ledger unarmed a permanently blind guard parks every
on-meter bead: a structural condition reported as a transient outage,
permanently.

Linux is a target, not a hypothetical: `make test-linux` is a release
gate (ranger-base-dbe), the clean room is Debian 13, and
ranger-base-160 scopes macOS + omarchy + rhel/fedora. The operator's
direction (2026-08-26, verbatim on ranger-base-x6ic): *"we need to
survive not being macos — using a .config and only .config is probably
the right direction with a 'posse refresh' credentials (human gated)"*.

What already works: a caged session authenticates from a file. The PID
names an env set; `envs/container.env` (mode 600, under the home)
carries `CLAUDE_CODE_OAUTH_TOKEN`, a long-lived setup-token minted by
the operator's own hand (`cage.go` CageCredential, ADR 0002 §4). That
path has no platform dependency at all.

What already failed: copying the *rotating* OAuth token into a file.
`default.env` held such a copy; it silently won over fresh logins and
silently rotted, and 401'd the fleet twice (2026-08-22, 2026-08-24).
The instance credential ADR's D7 removed it. Separately, MEASURED
2026-08-24: a setup-token gets HTTP 403 from the usage endpoint —
valid, not entitled — so no operator-mintable credential can feed the
meter. The meter needs the runtime's own live OAuth token, which the
runtime's login/refresh loop rotates. That loop is the single writer,
and its store is the store of record (Helland/Thompson; the field's
answer, `single-writer-and-stores.md`): every second copy of a rotating
credential is a snapshot that disagrees with the source exactly when it
matters.

The load-bearing observation: **on Linux, Claude Code's store of record
is already a file** — `~/.claude/.credentials.json`, the same
`claudeAiOauth` envelope the keychain item holds (ASSUMED for current
Linux builds; probe below). Portability does not require inventing a
store. It requires reading the store of record *where the platform
keeps it*, and owning nothing ourselves but the scoped mints we were
already given by hand.

## Decision

**1. Two credential kinds, one seam.** A credential is either
**posse-owned** (a scoped, long-lived mint the operator placed in the
home: today the cage setup-token in `envs/<set>.env` — store of record
is the home, "~/.config and only ~/.config" holds) or **runtime-owned**
(the rotating OAuth pair the runtime's own login loop writes — posse is
a reader, never a writer, never a copier). All acquisition goes through
one seam:

    Read(runtime, purpose) → (secret, meta, error)
    meta: Source (name, for diagnostics)
          ExpiresAt (time; zero = "cannot tell", an honest answer)

Two purposes today: `session` (what an authenticated caged session
needs injected — env-set lookup, `CageCredential` unchanged in
behaviour) and `meter` (what posse presents to the provider's usage and
models endpoints — replaces both `KeychainToken` callers). Since ADR 0042 the `session`
purpose is every crew runtime's credential at every tier, not the cage's
alone: a crew runtime holds the mint and never the operator's rotating
pair, whose one writer stays the operator's own claude — at `shims` the
L1 shim in front of the runtime's own keychain read enforces it (measured
2026-09-02: the session runtime carries the mint, its every keychain read
is refused, and it is authenticated). The seam is
the vault insertion point (ranger-base-epz8): a vault is a third
provider answering the same `Read`, not a second migration. Nothing
else in posse may acquire a credential except through this seam. Where
a harness credential *resides* is the instance page's D1:
`RHQ_HOME/secrets/` (dir 700, files 600; built empty, rangerhq-5s5d) —
a store this seam will read when a resident arrives, never a second
acquisition path.

**2. The meter reads the store of record, per platform.** One provider
("runtime store"), platform adapters chosen by `runtime.GOOS` — no
build tags, so `make test-linux` compiles and tests every branch:

- darwin: **the runtime's own composite store, mirrored** (amended
  2026-09-01, ranger-base-v3qi4). The PATH-resolution and exfil concerns
  (ranger-base-ypf5 / ranger-base-17i) ride with the adapter and keep
  their ordering; the adapter execs `/usr/bin/security` absolutely.

  MEASURED 2026-09-01 from the darwin-arm64 2.1.258 release binary
  (its checksum `b63136194160791c…` equals the manifest's and the copy
  installed on the reference box; recipe in NOTES.md, "What the shipped
  artifact actually does", bead ranger-base-ydjz). Claude Code's darwin
  secure-storage selector returns one store whose own name is
  `keychain-with-plaintext-fallback`, and its rules, read from the
  code, are:

  - *read*: the keychain item first (`security`'s find verb on the
    runtime's item, the same read `keychainCmd` makes), then the
    credentials file — store 3 below — only when the keychain answered
    null. Null is exit 0 with no output, exit 44 (item not found) or
    exit 36 (user interaction not allowed); any other exit is a read
    failure and the strict read does not fall through.
  - *update*: write the keychain. On success, delete the file only if
    the keychain read null just before the write. On a non-transient
    failure (non-zero exit that is not a timeout) write the file, mode
    0600, then delete the keychain item only if it read non-null before.
  - *delete*: both.
  - the file's directory is `$CLAUDE_SECURESTORAGE_CONFIG_DIR`, else
    `$CLAUDE_CONFIG_DIR`, else `~/.claude` (ranger-base-wd4be).

  So the file is the runtime's **live fallback store**, written by the
  same login/refresh loop and read by the runtime — not "some auth
  flow's byproduct". Four states follow; the table is the decision:

  | state | keychain item | file | runtime reads | adapter before | adapter now |
  |---|---|---|---|---|---|
  | S1 healthy | live | absent | keychain | keychain | keychain |
  | S2 frozen (MEASURED, ranger-base-1lza) | live | stale | keychain | keychain | keychain |
  | S3 inverted | absent | live | file | `CredUnreadable` → blind → parks under 0018 while claude is authenticated, and the runbook move ("run claude once") writes nothing | file |
  | S4 split (ASSUMED) | older | newer | keychain | keychain | keychain |

  S2's mechanism, ASSUMED — consistent with the code and with both
  1lza observations, not itself observed: a refresh landed on the file
  while the keychain read null-but-present (a locked keychain answers
  36), the keychain recovered, and every later keychain write found a
  non-null item and so never deleted the file. That is why "delete
  once" measured out a treadmill, and why the file's frozen mtime says
  "S2", not "inert". S3 is reachable by the same code on a run of
  non-transient keychain write failures (a keychain that will not
  unlock, an ACL refusal on the runtime's binary); it has not been
  observed on the reference box and its rate is unmeasured.

  **The adapter mirrors the composite's read order with one
  narrowing**: keychain item first; the file only when `security`
  exits 44 (item not found). Exit 36 and every other failure stay
  `CredUnreadable` with the ACL fix text and never reach the file. The
  narrowing is deliberate: the keychain ACL is per binary, so posse's
  36 speaks about posse's binary, not about the keychain's contents —
  mirroring the runtime's 36-means-null rule literally would read a
  frozen S2 file after every `make install` and re-create the 08-24
  misdiagnosis with a new sentence. A read that fell through carries
  `Source: "the Claude Code credentials file (keychain fallback)"`, so
  a 401 from it names the store it came from, and both the 401 sentence
  and the 44-and-no-file sentence name the two causes an operator must
  tell apart: the keychain is empty (claude is running on the file), or
  this binary's ACL was dropped. ASSUMED, operator-measurable only
  (every crew PID denies `security`): a dropped posse ACL answers 36,
  not 44. If it answers 44 the design degrades to S1 → the two-cause
  unreadable sentence (today's class) and S2 → a 401 whose sentence
  names the fallback; the sweep below exposes S2 either way.

  Not changed: exit 44 with no file stays `CredUnreadable` (blind, with
  0018's clock), not `NoSource`. The launcher box is a logged-in box by
  construction; a vanished item there is an incident, not an
  unconfigured platform. Also unchanged: D5 — an expired fallback
  envelope is presented, not refused; the 401 stays the only actuator.

  **Darwin has three credential stores** (counted 2026-08-28,
  ranger-base-1lza; recounted here):

  1. the keychain item — the composite's primary, and the store the
     runtime reads whenever it holds an item;
  2. `envs/<set>.env` mints — posse-owned, scoped, human-gated
     (unchanged, D1/D4);
  3. the credentials file under the runtime's config dir (the path
     `CredentialsFile()` spells) — the composite's **fallback**, owned
     by the runtime. Live in S3, stale in S2, absent in S1.
     MEASURED (ranger-base-xjj9/m6cm): deleted 2026-08-26, regenerated
     994 B mode 600 at 11:47:07 the same day, read once 101 s later,
     then frozen two days while the keychain rotated — S2. Posse reads
     it only in S3, exactly when the runtime does. The compensations
     stand, with sharper meaning:
     - detective: the credential-path sweep — `make
       verify-credential-paths` + `docs/runbooks/credential-rotation.md`
       (ranger-base-m6cm). A file present now reads "the keychain
       failed a write at some point: S2 or S3", and the operator's
       move is the same in both: repair the keychain (unlock, ACL),
       then `/login` in claude — a keychain write that succeeds while
       the keychain reads null deletes the file, so S3 heals to S1 on
       the next successful write. "Run claude once" does not: a run
       with a live access token performs no write.
     - preventive: the seatbelt `file-read` deny on credential-file
       literals (ranger-base-hw18), GOOS-shaped so the linux store of
       record stays readable. It protects the file exactly in the state
       where it holds a live token (S3), at a cost accepted here: in S3
       a seatbelt-caged claude session reads the same composite, hits
       the deny on the fallback, and cannot authenticate until the
       operator repairs the keychain — loud, correct, and the same
       move as the meter's.
     - liveness/revocation of any current instance is the operator's
       call (ranger-base-tyne), independent of this model.
- linux (and any non-darwin): `~/.claude/.credentials.json`, fed
  through the **same** `credentialToken`/`credShapes` parser — the
  blob is the same envelope, so ranger-base-okbr's shape diagnostics
  apply verbatim to both paths.

ASSUMED, probe before trusting: current Linux Claude Code writes a live
`claudeAiOauth.accessToken` there and it returns 200 on the usage
endpoint. The instance ADR reserved exactly this probe when it rejected
the file *on macOS* (there it is a stale leftover and reading it
inverts the store of record; that rejection stands on darwin). If the
probe fails, the adapter's honest error still beats today's
"keychain unreadable" — and the guard-off path in D3 catches it.

**3. Structural absence is OFF, not blind.** A new read-outcome class,
`NoSource`: this (runtime, purpose, platform) has no store — no meter
adapter for the runtime (ADR 0012 D4: a provider without a usage
endpoint runs the guard off), or the platform store does not exist.
The guard treats `NoSource` as *unconfigured*, like thresholds unset:
one witness line naming the platform, the store it would need, and what
would arm it — the blind clock never starts, nothing parks. ADR 0018's
policy is untouched: *blind* remains "a source exists and the read
failed", and parks only while the ledger is unarmed. Error text must
name the store actually tried on this platform; the word "keychain"
never appears in a non-darwin error.

**4. `posse refresh` is the one credential write, and it is the
operator's hand.** The standing rule "posse never mints, refreshes, or
writes a credential" is amended to "never *autonomously*". `posse
refresh`:

- is interactive-only: refuses without a TTY and refuses inside a
  persona session (the persona env markers are the tell); every crew
  PID adds `Bash(posse refresh:*)` to `deny:` — the guardrail expressed
  twice, prose and rule.
- for `session` credentials: runs the runtime's own mint (`claude
  setup-token` — the browser flow *is* the human gate) or accepts a
  pasted token (`--paste`, for a headless box whose mint happened on a
  browser-capable one), then writes it into the named env set, 600
  under 700, with `# minted=YYYY-MM-DD` and `# expires=YYYY-MM-DD`
  stamps (expires only when known — no invented dates).
- for `meter` credentials: **writes nothing**. It prints the
  store-of-record instruction ("run `claude` once to refresh").
  Copying the rotating token is the default.env bug and stays banned.
- never touches a metered credential: `ANTHROPIC_API_KEY` remains
  rejected on the money line (rangerhq-kiz), restated here.
- with no arguments: a report — each (runtime, purpose), its source,
  its expiry or "cannot tell", and the action if any. This is the
  runbook's front door (rangerhq-m10j).

**5. Expiry is a first-class answer, surfaced before it bites.** Today
nothing notices a credential expiring until the shop stops
(ranger-base-okbr: 401 at 16:02, an hour of blind passes). The seam's
`ExpiresAt` comes from the envelope's own `expiresAt` field for the
meter (ASSUMED field name; present in the known envelope shape) and
from the `# expires=` stamp for session mints.

**The surfaces split by purpose** (amended 2026-08-28,
ranger-base-swqk, ratifying the recorded divergence of
ranger-base-k6ha; the original letter surfaced both purposes in all
three places — see Alternatives): the `posse refresh` report answers
for **both** purposes, on demand. The two timer surfaces — the cockpit
header once inside 14 days, and one stderr line per dispatch pass in
the same window — carry the **posse-owned session mints only**. The
meter credential gets no unasked expiry surface, for three reasons of
unequal weight:

- **There is no hand to warn** (load-bearing, and independent of any
  TTL). D4 makes the runtime's login loop the meter credential's only
  writer and posse a reader that writes nothing; the next rotation
  happens without an operator. A warning is a request for an action,
  and the only meter action — "run `claude` once" — is meaningful
  exactly when that loop is dead, which surfaces as a failed read:
  blindness, ADR 0018, already loud, already clocked, and arriving on
  a read the pass was making anyway. This decision's own closing
  sentence — the read's success or failure remains the only actuator —
  was always the meter's whole answer.
- **It would never be quiet** (MEASURED — ranger-base-b1al, operator's
  terminal 2026-08-29: the access token's `expiresAt` horizon read
  **6h30m**, far inside any warning window). Every reading sits inside
  a 14-day window and the line would fire every pass forever — a
  warning that is always on is a warning nobody reads. The
  corroboration holds; the decision stood on "no hand to warn" either
  way.
- **The cost is per-pass and real** (a design fact, not an estimate):
  warning about the meter means execing `security` on every dispatch
  pass and every cockpit tick, against a store the whole instance
  deliberately reads once per TTL through one shared cache
  (rangerhq-tdy8). A session mint's stamp, by contrast, is a few
  hundred bytes in a file posse already owns.

Expiry never gates or parks anything by itself — it is a warning, and
the read's success or failure remains the only actuator. "Cannot tell"
is reported as exactly that, never as "fresh"; the timer surfaces'
silence is ambiguous by construction (nothing expiring, or nothing
dated), and the report is the one place that says which.

**6. Per-runtime, no speculative config.** The seam is keyed by
runtime. codex/grok: `session` stays undecided-refuse until their lane
reaches the cage tier (unchanged); `meter` is `NoSource` (guard off
with witness) — which is what ADR 0012 D4 already promised, now with an
honest rendering. Their `auth.json` stores are already plain files on
every platform, so when their lanes arrive the linux-shaped adapter is
the template. No new runtime-yaml key (`meter_source:` or similar)
until a second meter adapter exists to need it.

## The trade, plainly (the honest objection)

A mode-600 file is a weaker store than the macOS keychain, and this
design puts more weight on files. What is actually traded:

- **What moves into files: only scoped mints.** The powerful credential
  — the account's rotating OAuth pair — never enters a posse-owned
  file. A leaked session mint burns one revocable token, not the
  account. The blast-radius argument from the instance ADR's class
  split, kept. **But "posse-owned" is a narrower wall than it sounds
  on darwin** (amended 2026-08-28, recounted 2026-09-01): the rotating
  pair sits in a plain 600 file — the credentials file, the
  composite's fallback in D2 store 3 — below the container wall whenever the
  runtime's keychain write fails, and a frozen copy of it stays behind
  afterwards (S2). Ownership hygiene does not reach a file posse never
  writes; the comfort this bullet buys is void in effect until the D2
  compensations (sweep + read deny) hold.
- **What the keychain was actually buying on darwin: less than it
  looked.** MEASURED 2026-08-24: the non-interactive same-user read
  succeeded — the keychain was not protecting this token from personas;
  its per-binary ACL was meanwhile *breaking* the guard on every `make
  install`. It stays on darwin because it is the store of record there,
  not because it is a wall.
- **The secrecy wall was never file mode.** Below the container tier
  any same-uid session can read a 600 file; the wall is the container
  tier, unchanged. What 700/600 + `TightenEnvPerms` buys is other-user
  and casual-tool exposure, and that is all it is claimed to buy.
- **The compensations:** the promotion gate (credential-touching code
  ships only through `make install` review — load-bearing,
  indefinitely), path-scoped writes (ADR 0014), env-set listings that
  never print values, and the vault destination (ranger-base-epz8) as
  the real answer to at-rest secrecy when it is priced. For the unowned
  darwin file specifically: the detective sweep (`make
  verify-credential-paths`, ranger-base-m6cm) and the seatbelt
  file-read deny (ranger-base-hw18) — both in the compensation list by
  design, because a store nobody owns needs controls nobody has to
  remember to run.

## Consequences

- The guard becomes buildable and honest on Linux: it reads the meter
  through the runtime's own file, or says "off — no source on this
  platform" once, quietly. The permanent-park failure mode is
  structurally gone (D3) even if the probe in D2 fails.
- `KeychainToken` disappears as a name; both callers go through the
  seam. The keychain read survives as the darwin adapter, exec path
  and all — ypf5/17i land on it exactly as scoped.
- (2026-09-01) The darwin adapter agrees with the runtime about which
  store is live in every state the composite can reach: S3 stops
  parking an authenticated fleet, and the one new read (the file, on
  exit 44 only) happens exactly when the runtime itself makes it. No
  new store, no new writer; one exit-code discriminator and one
  `Source` string.
- The operator gets one verb: `posse refresh` reports every credential,
  its lifetime, and its fix; mints session tokens under the human gate;
  and refuses everyone else. The instance runbook (rangerhq-m10j)
  fronts on it.
- The rotating-token freshness bug class cannot recur by construction:
  no posse store ever holds a rotating credential, so there is no copy
  to rot.
- Cost: the seam is a move-and-wrap of two existing code paths plus one
  new file adapter (ASSUMED small; the parser is shared), one outcome
  class, one command. No new directory, no new store, no migration of
  the four live env files (their move is the home cutover,
  ranger-base-h56a, not this).

## Alternatives rejected

- **A posse-owned `~/.config` copy of the meter token, refreshed by
  `posse refresh`.** The literal reading of "only .config", and the
  clever one this time. Dead on physics: the token rotates on the
  runtime's schedule, a human gate cannot keep up, and the copy is
  exactly default.env — MEASURED to 401 the fleet twice. The operator's
  direction is honored in intent (no OS-keychain dependency; everything
  posse *owns* under the home), amended in letter, and this paragraph
  is the plain statement of that amendment.
- **Read the credentials file (D2 store 3) first, or only, on darwin.**
  One adapter instead of two. Rejected 2026-08-26; evidence amended
  2026-08-28 (ranger-base-1lza: the file self-renews, and the
  regenerated one sat two days unrefreshed while the keychain rotated);
  narrowed 2026-09-01 (ranger-base-v3qi4) to exactly this: file-first
  or file-only inverts the store of record in S2, the measured state.
  Reading the file *second*, on item-not-found only, is not this
  alternative — it is the runtime's own order, and D2 now adopts it.
- **Keep the darwin adapter keychain-only and accept S3 as a named
  risk** (the bead's other option, 2026-09-01). Rejected: S3 parks the
  fleet under ADR 0018 on a state where the runtime is authenticated,
  and the runbook's move for the unreadable class ("run claude once")
  performs no keychain write and so cannot heal it. The fix is one
  exit-code discriminator in an adapter whose `security` stubs already
  exist (`credentialclass_test.go` exits 44 today), which is cheaper
  than the paragraph that would have named the risk.
- **Mirror the runtime's null set literally** — fall through to the
  file on exit 36 as well as 44. Rejected: 36 is per binary. The
  runtime's 36 means its keychain is locked; posse's 36 after `make
  install` means posse's ACL is gone while the keychain holds the live
  item, and a literal mirror would read the frozen S2 file and 401
  with a sentence about staleness — the 08-24 misdiagnosis again.
- **Read both stores and present the fresher `expiresAt`.** Rejected:
  two reads per pass, and a tie-break the runtime does not use — its
  rule is order, not freshness — so wherever the two disagree posse
  would present a token the runtime is not presenting. The seam
  mirrors the store of record; it does not improve on it.
- **Refuse an expired fallback envelope locally instead of presenting
  it.** Rejected on D5: expiry never gates, the read's outcome is the
  only actuator, and the `Source` string already makes a 401 from the
  fallback diagnosable without a second rule.
- **OS keyring on Linux** (Secret Service / gnome-keyring) as the
  meter store. "Keychain, portably": re-imports the per-binary/unlock
  fragility that broke the guard three times in one day, needs a
  session bus a headless clean room does not reliably run, and is
  still not where Claude Code writes on Linux — it would be a copy.
  Priced for the vault decision (epz8), not for this.
- **posse refreshes the rotating token itself** (use the refreshToken).
  Two writers on one rotating credential; maximal blast radius for a
  freshness bug. Standing rejection, unchanged.
- **A `secrets/` directory now** (harness-credential class made
  concrete). Rejected here 2026-08-26: posse has zero resident harness
  credentials — the meter token measured 403 out of every mintable
  form — and an empty directory with a loader is a cathedral for a
  congregation of none. **Overturned 2026-08-28**: the operator
  accepted the instance ADR of the same number (option (b),
  rangerhq-q65q), whose D1 orders the directory built empty because
  the class split is the trust model, not a convenience; it landed as
  rangerhq-5s5d (af4353d). The pages reconcile because `secrets/` is a
  *store*, not a second acquirer: nothing under it reaches a session,
  no PID key can name it, `posse envs` cannot list it (pinned,
  `secrets_test.go`), and a future resident reaches its caller only
  through this page's D1 seam. What survives of the rejection is the
  residency claim: the dir stays empty, init seeds no
  `plan-guard.env`, and the plan guard is not a consumer.
- **Vault now.** Explicitly parked by the operator (ranger-base-epz8,
  P3): bootstrap inversion for the unattended 3am pass, and "what
  vault means" is undecided. The seam is the concession it collects.
- **The meter in the timer surfaces (this ADR's own original D5
  letter).** Rejected 2026-08-28 (ranger-base-swqk) on the three
  reasons now in D5, after the build recorded the divergence
  (ranger-base-k6ha). The honest accounting: the load-bearing reason
  (no hand to warn) is structural and TTL-independent; the noise
  reason is ASSUMED pending ranger-base-b1al; the cost reason is a
  per-pass `security` exec that the session-only rule avoids entirely.
- **An "already expired only" meter line** — the zero-noise
  compromise: surface the meter unasked only once its envelope date is
  past. Rejected: at that moment the usage read is failing, and ADR
  0018 already reports exactly that state, loudly, on a read the pass
  makes anyway — this line would duplicate an existing signal while
  arming the very per-pass keychain exec the split avoids, and it
  warns *at* the bite, which is not what D5's "before it bites" buys.
  The okbr-shaped outage (an hour of unnoticed blind passes) was a
  visibility failure of the blind signal, and okbr's shape diagnostics
  plus ADR 0018's clock are its fix — not a second copy of the alarm.

## Verification (laurie's checklist)

- V1 (probe, clean room): on Debian 13, after `claude` login,
  `~/.claude/.credentials.json` holds `claudeAiOauth.accessToken` and
  that token returns 200 on the usage endpoint. Until run, D2's linux
  adapter is built-but-unconfirmed and says so in its error text.
- V2 (unit): a non-darwin build never execs `security` — the GOOS
  switch is total, and no non-darwin error string contains "keychain".
- V3 (unit): `NoSource` renders as guard-off/unconfigured in the
  cockpit header and the dispatch line; the blind clock does not start;
  ADR 0018's park/degrade behaviour is unreached.
- V4 (operator): `posse refresh` refuses without a TTY and inside a
  persona session; crew PIDs carry the deny line.
- V5 (unit): refresh writes 600 under 700; stamps round-trip through
  the seam's `ExpiresAt`; `posse envs` output still never contains a
  value.
- V6 (unit, amended 2026-08-28 ranger-base-swqk): a session mint's
  `# expires=` inside 14 days appears in the header and once per pass
  on stderr; expired renders distinctly; zero renders as "cannot tell"
  and warns nothing. And the purpose split holds: a meter envelope
  expiring inside the window appears in the report and in **neither**
  timer surface — pinned with a positive witness on the same box (a
  near-expiry session mint that does appear), so the absence half
  cannot pass by measuring nothing.
- V7 (unit): one envelope fixture parses identically through the
  keychain-blob path and the file path — the okbr diagnostics are
  provably shared, not forked.
- V8 (unit, added 2026-09-01 ranger-base-v3qi4): with a stubbed
  `security` and a planted fallback file, the darwin adapter reads the
  file on exit 44 only and its `Source` names the fallback; on exit 0
  with an envelope the file is never opened even when present and
  fresher; on exit 36, exit 1 and a gate refusal the file is never
  opened and the class is today's (`CredUnreadable` / `GateRefusal`).
  "Never opened" is pinned by a witness the read would trip, not by the
  result — a fixture the adapter ignores is a pass that measured
  nothing. On either box, no build tags.
- V9 (unit, added 2026-09-01): the 401 sentence for a token read from
  the fallback, and the unreadable sentence for exit 44 with no file,
  each name both causes (keychain empty; this binary's ACL dropped),
  and neither quotes a value.
- V10 (operator, added 2026-09-01; unmeasurable by a persona — every
  crew PID denies `security`): the exit code of the adapter's own
  keychain read (`keychainCmd`, the find verb with `-w`) when run by a
  binary whose ACL was dropped, non-interactively. 36 confirms
  the narrowing's premise; 44 keeps the design and moves the residual
  in D2 from ASSUMED to MEASURED.
