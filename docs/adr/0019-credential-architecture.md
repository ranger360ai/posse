# ADR 0019 — Credential provider seam: posse owns scoped mints under the home; rotating runtime credentials are read where the runtime keeps them

*Status: proposed 2026-08-26 · richard · bead ranger-base-x6ic ·
extends ADR 0012 D4 (plan-guard seam) and ADR 0018 (blind policy);
supersedes, at the harness level, the "guard does not become portable
off macOS" consequence of the instance-private credential ADR of the
same number. Amended 2026-08-28 (ranger-base-1lza): darwin counts
three stores, not two, and the `.stale-*` evidence clause was measured
dead — the file self-renews.*

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
models endpoints — replaces both `KeychainToken` callers). The seam is
the vault insertion point (ranger-base-epz8): a vault is a third
provider answering the same `Read`, not a second migration. Nothing
else in posse may acquire a credential except through this seam.

**2. The meter reads the store of record, per platform.** One provider
("runtime store"), platform adapters chosen by `runtime.GOOS` — no
build tags, so `make test-linux` compiles and tests every branch:

- darwin: the keychain item, existing code moved verbatim. The
  PATH-resolution and exfil concerns (ranger-base-ypf5 /
  ranger-base-17i) ride with the adapter and keep their ordering; this
  ADR does not change how `security` resolves. (Both have since landed
  in that order: the adapter now execs `/usr/bin/security` absolutely.)

  **Darwin has three credential stores, not two** (amended 2026-08-28,
  ranger-base-1lza):

  1. the keychain — store of record, the meter adapter above
     (unchanged);
  2. `envs/<set>.env` mints — posse-owned, scoped, human-gated
     (unchanged, D1/D4);
  3. `~/.claude/.credentials.json` — a **recurring unowned byproduct**.
     Some darwin auth flow writes a full `claudeAiOauth` envelope there
     on its own schedule. MEASURED (ranger-base-xjj9/m6cm): the
     operator deleted the file 2026-08-26, two sessions verified clean
     by 03:40, and a fresh one (994 B, mode 600, new content) appeared
     at 11:47:07 the same day — read once 101 s later, then frozen for
     two days while claude wrote `~/.claude` continuously. The frozen
     mtime is the discriminator that it is *not* the store of record;
     the regeneration is the proof that it is not a one-off leftover
     either. Posse never reads it, never wants it, and cannot delete it
     away — "delete once" was the implicit model and it measured out a
     treadmill. It is counted and compensated explicitly:
     - detective: the credential-path sweep — `make
       verify-credential-paths` + `docs/runbooks/credential-rotation.md`
       (ranger-base-m6cm, shipped);
     - preventive: the seatbelt `file-read` deny on credential-file
       literals, GOOS-shaped so the linux store of record stays
       readable (ranger-base-hw18);
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
from the `# expires=` stamp for session mints. Surfaced three places:
the `posse refresh` report; the cockpit header once inside 14 days; one
stderr line per dispatch pass in the same window. Expiry never gates or
parks anything by itself — it is a warning, and the read's success or
failure remains the only actuator. "Cannot tell" is reported as
exactly that, never as "fresh".

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
  on darwin** (amended 2026-08-28): the rotating pair recurringly sits
  in an *unowned* plain 600 file — `~/.claude/.credentials.json`, the
  third store in D2 — below the container wall, written by the
  runtime's own auth flow on its own schedule. Ownership hygiene does
  not reach a file posse never writes; the comfort this bullet buys is
  void in effect until the D2 compensations (sweep + read deny) hold.
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
- **Read `~/.claude/.credentials.json` on darwin too.** One adapter
  instead of two. Rejection stands; the evidence clause is amended
  (2026-08-28, ranger-base-1lza) — the original citation ("a stale
  leftover of the keychain login, renamed `.stale-*` on the reference
  box") described a state gone since 2026-08-26 and mischaracterized
  the file: it self-renews (D2, store 3). The corrected evidence
  argues the rejection *harder*: MEASURED, the regenerated file sat
  two days unrefreshed while claude ran daily and the keychain
  rotated. Making it the darwin source would invert the store of
  record onto a snapshot that is provably stale within days.
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
  concrete). The class split is kept conceptually, but posse has zero
  resident harness credentials — the meter token measured 403 out of
  every mintable form. An empty directory with a loader is a cathedral
  for a congregation of none; the seam (D1) is where a future resident
  plugs in.
- **Vault now.** Explicitly parked by the operator (ranger-base-epz8,
  P3): bootstrap inversion for the unattended 3am pass, and "what
  vault means" is undecided. The seam is the concession it collects.

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
- V6 (unit): an `ExpiresAt` inside 14 days appears in the header and
  once per pass on stderr; expired renders distinctly; zero renders as
  "cannot tell" and warns nothing.
- V7 (unit): one envelope fixture parses identically through the
  keychain-blob path and the file path — the okbr diagnostics are
  provably shared, not forked.
