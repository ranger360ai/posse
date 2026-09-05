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
then the file only on item-not-found). Amended 2026-09-02
(ranger-base-ig4op): store 1's NAME is derived from the same two
config-dir variables as the file's directory — the constant is the
default case only; the adapter derives the name it reads. Amended
2026-09-03 (ranger-base-z089h, from ranger-base-4poib): the meter
credential's life is MEASURED at 8h and its only writer is the
operator's own interactive claude — D5's load-bearing reason ("no hand
to warn") is false. Nothing is built: the at-the-bite line
(ranger-base-4poib, ranger-base-ddivo) is the smallest shape and no
measured failure of it survives their promotion; a gauge, a
once-per-token alarm and a fifth 401 class are priced and parked. D4
keeps posse off the refreshToken; option B (ranger-base-wkai3) was
ordered first and measured dead, and "ask the owner to refresh" (D4
§1b, ranger-base-jefo0) was ruled option 0 and stays parked — both
closed 2026-09-04; the correction at the end of this block has the
detail. Corrected 2026-09-04 (ranger-base-vxbfm, from the
verify bundle ranger-base-9mys1): six passages elsewhere on this page
read as if the parked gauge and alarm were built, or still carried the
08-28 weighting D5 as amended had struck, and option 0's ground was
half unlanded on the day it was written. Tense fixed, the one
contradiction resolved toward D5, and the landings recorded
(ranger-base-4poib 36e8584, ranger-base-ddivo 58ac284). No decision
moved: every rejection, every parked shape and the do-nothing decision
stand exactly as ranger-base-z089h left them. Corrected 2026-09-04
(ranger-base-vfx8g, from the verify bundle ranger-base-v4zlr): all
three of this page's "open" citations named a bead that had closed.
V1b and D2's ASSUMED half called ranger-base-au0o4 the open holder of
the credentials-file 200 probe — it closed 2026-09-02 without
answering it, and nothing has held that probe since; D2's MEASURED
half called ranger-base-wd4be open after it landed. V1b now states the
probe is UNHELD, why the three beads that touched it did not answer
it, and what the unbounded built-but-unconfirmed state costs D2's
non-darwin adapter. No decision moved. Corrected 2026-09-04
(ranger-base-ur2eo, from the verify bundle ranger-base-2fl9a): five
passages still gated the option-B/1b thread on a measurement and a
ruling it had already had. ranger-base-hs0dl ran twice on 2026-09-04
and option B is dead on that measurement; ranger-base-wkai3 closed the
same day on the operator's ruling "not B"; ranger-base-jefo0 closed on
option 0, do nothing. The gating clauses are past tense now, at D4's B
and §1b bullets and at V17. No decision moved: D4 §1b stays parked,
priced-not-built, which is what the operator's own close says. Amended 2026-09-05 (ranger-base-q3n4e,
from ranger-base-mvrke): the session half's store is the env set under
the home, never the reading process's environment — D1 below and ADR
0039 D3d say the same thing about where that value is read.*

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
behaviour; *amended 2026-09-05, ranger-base-q3n4e:* the seam READS it
from the env set files under the home, the store of record this
paragraph names, selected by the launch's set list in launch order with
the last assignment winning — never from the reading process's own
environment. The environment arm the first build carried was written
for a caller that cannot exist: a posse process is never the launched
runtime, and the runtime scrubs the mint from its children (MEASURED
by elimination, ranger-base-q3n4e). ADR 0039 D3d as amended carries the
ruling, the selection rule and the exposure answer) and `meter` (what posse presents to the provider's usage and
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
  - the keychain item's NAME follows the same two variables, one
    statement later in the same module (ranger-base-ig4op): the
    constant when neither names a directory, else the constant plus a
    hash of the directory string — store 1 below spells the rule.

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
  narrowing**: keychain item first (under the name store 1 below
  derives from the environment — ranger-base-ig4op); the file only
  when `security` exits 44 (item not found). Exit 36 and every other failure stay
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
     runtime reads whenever it holds an item. **Its name is derived,
     not constant** (amended 2026-09-02, ranger-base-ig4op; MEASURED
     off the same darwin-arm64 2.1.258 bundle, one statement after the
     directory resolver `credentialDir()` quotes). The name is
     `Claude Code-credentials` exactly when the environment names no
     directory: `CLAUDE_SECURESTORAGE_CONFIG_DIR` absent and
     `CLAUDE_CONFIG_DIR` unset or empty, or
     `CLAUDE_SECURESTORAGE_CONFIG_DIR` present-but-empty (which
     shadows `CLAUDE_CONFIG_DIR`, as it does for the file). Otherwise
     it is that name, `-`, and the first 8 hex digits of sha256 over
     the directory string — the secure-storage variable's value (the
     runtime NFC-normalizes it) when set and non-empty, else the
     config-dir value. Three rules fall out and the adapter carries
     all three:
     - *Default-ness is an environment property, not a path property.*
       `CLAUDE_CONFIG_DIR=$HOME/.claude` names the default directory
       and still suffixes the item, because the runtime tests the
       variable, never the path. So the resolver wd4be lands exposes
       *whether a variable named the directory* beside the directory —
       one function yields the file path and the item name, as the
       runtime has it; a second derivation is how the two drift.
     - *The hash is over the string as the variable spells it*, not a
       cleaned path: a trailing slash hashes as typed. MEASURED for the
       secure-storage arm (normalize on the raw value); ASSUMED for the
       config-dir arm, which hashes whatever the runtime's config-dir
       function returns — read as "the value verbatim", the reading
       trust.go already makes; V13 checks it. The hash is Node's sha256
       over the string's UTF-8 bytes, which is Go's
       `sha256.Sum256([]byte(s))` — MEASURED 2026-09-02, node and
       shasum agree on every V11 fixture.
     - *posse does not normalize.* Go's standard library has no NFC,
       the x/text module is priced below and not taken, and the wd4be
       resolver normalizes nothing either. For an ASCII value NFC is
       the identity (MEASURED), so the derived name is exact wherever
       the operator typed an ASCII path. For a non-ASCII value posse
       hashes the bytes as spelled and the store's name says so, so an
       exit 44 there names its first suspect instead of reading as an
       empty keychain (a decomposed `é` hashes differently from a
       composed one — MEASURED, V11 carries the pair).

     What the name inherits from the directory: posse must see the
     same two variables the operator's `claude` sees. A launcher
     started with a different environment (a LaunchAgent, say) derives
     a different name and reads 44 on a healthy box — the divergence
     the file path already has under wd4be, not a new one — and the
     refresh verb's report (ranger-base-6kkrq) prints the name tried,
     so it is diagnosable. posse now PINS both variables on a claude
     launch (amended 2026-09-03, ranger-base-rq83c — the preventive
     bullet under store 3 says why), and the pin is derived from
     `credentialDirNamed`, the same function `keychainItem` derives the
     name from: it pins the directory when a VARIABLE named it and the
     empty string when none did, so `named` and the directory are both
     unchanged and this paragraph's rule still describes the box. That
     makes the invariant structural rather than lucky, and it is
     asserted as one — the pin's own test re-derives both
     `CredentialsFile` and `keychainItem` with the pinned values in the
     environment and refuses a difference. The arm it exists for is the
     obvious wrong pin: naming `$HOME/.claude` where nothing was set
     passes a path check and still suffixes the item. Since ADR 0042
     no crew runtime reads the keychain either, so the operator's own
     shell is the only environment in play. A non-production deployment (the
     runtime's OAuth file suffix non-empty) is a third name and out of
     scope, the stance trust.go takes for the staging config file.
     `KeychainService` survives as the default spelling (build:
     ranger-base-mx4q6, between wd4be and 5jdzh); every
     sentence and the seam's `Source` print the derived name, so an
     operator with a suffixed item sees the suffix to match in Keychain
     Access. REACHABILITY, MEASURED 2026-09-02 on the reference box:
     neither variable is set, so the constant is the live name today
     and nothing here changes what the box reads until an operator sets
     one — at which point, under the unamended design, every pass
     would have said "the keychain is empty" about a keychain that
     held the live item;
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
       record stays readable. Which literal that is for claude is the
       RUNTIME's own directory resolution, not a fixed home path
       (`credentialFileCandidates`, ranger-base-x5f6p): the deny names
       `~/.claude`'s and, when `CLAUDE_SECURESTORAGE_CONFIG_DIR` or
       `CLAUDE_CONFIG_DIR` moves the write, the moved one too. Spelled
       as one home literal it was a wall over a path the runtime had
       already stopped using. It protects the file exactly in the state
       where it holds a live token (S3), at a cost accepted here: in S3
       a seatbelt-caged claude session reads the same composite, hits
       the deny on the fallback, and cannot authenticate until the
       operator repairs the keychain — loud, correct, and the same
       move as the meter's.
     - preventive, second layer: the launch PINS the two variables the
       resolution above reads, in the one scope a persona cannot reach
       (`credentialDirPin`, amended 2026-09-03, ranger-base-rq83c). The
       deny above is rendered from the LAUNCHER's environment, and a
       user-scope `~/.claude/settings.json` `env` block is applied over
       `process.env` of every claude that starts afterwards — the cage
       grants `~/.claude` whole (ADR 0012 D4), so a caged persona can
       write one. MEASURED on claude 2.1.259 against a scratch HOME and
       a fake envelope: with the launcher exporting the right directory
       in the child's real environment, that settings file still moved
       the store, so exporting is not the fix. The runtime applies each
       settings scope's `env` in the order userSettings,
       projectSettings, localSettings, flagSettings, policySettings, so
       posse's own `--settings` payload (flagSettings) lands after the
       user's file and argv is not a file a persona can write. Two
       higher scopes were measured and are not available: the SDK
       parent tier (`--managed-settings`) drops `env` through its
       restrictive-only filter, and `CLAUDE_CODE_MANAGED_SETTINGS_PATH`
       is inert in 2.1.259. The remaining gap is the operator's own
       UNCAGED claude, which no launcher flag reaches: only a
       root-owned OS `managed-settings.json` covers it, and that is
       filed as an operator ask, not taken here.

       The pin carries a SECOND environment split, found after it
       landed (ranger-base-r68d8, amended 2026-09-05): a persona
       session is not a child of the launcher at all. It is a herdr
       workspace — `CreateWorkspace` hands the pane only the vars posse
       names explicitly, the pane is a child of the herdr DAEMON, and
       the runtime is typed into that pane's LOGIN shell — so what the
       runtime reads is the daemon's environment plus the login rc's,
       the same three-way split ADR 0013 records for PATH. Either
       direction voids the deny exactly as the settings file did: an
       `export CLAUDE_SECURESTORAGE_CONFIG_DIR=…` in a login rc reaches
       an unattended runner and never reaches posse, and a
       `CLAUDE_CONFIG_DIR=… posse new` reaches posse and never reaches
       the pane. The pin closes it by the same mechanism and needs no
       second one, because a flag-scope `env` block is applied over
       `process.env` whatever that environment held —
       `credentialpanesplit_qa_test.go` pins the coupling from the
       rendered line, with the unpinned pane as its control. An env
       var, which is what the finding first proposed, is the scope the
       pin already outranks: the launch keeps REFUSING an env set that
       carries either name rather than appending one (herdrback.go).
       Residual, not closed here: a pane whose HOME differs from the
       launcher's still resolves `$HOME/.claude` for itself on the arm
       where no variable names the directory, since the pin's
       secure-storage value there is the empty string that keeps the
       keychain item unsuffixed (ranger-base-ig4op); posse sets no HOME
       for a pane, so it is not a split posse opens.
     - liveness/revocation of any current instance is the operator's
       call (ranger-base-tyne), independent of this model.
- linux (and any non-darwin): `~/.claude/.credentials.json`, fed
  through the **same** `credentialToken`/`credShapes` parser — the
  blob is the same envelope, so ranger-base-okbr's shape diagnostics
  apply verbatim to both paths.

Two claims here, split 2026-09-02 (ranger-base-17rt4) because one is
now measured and the other is not:

- MEASURED 2026-09-01 from the linux-x64 2.1.258 release binary
  (checksum `704f1334ac65d3e8…` equals the manifest's; recipe in
  NOTES.md, "What the shipped artifact actually does", bead
  ranger-base-ydjz): off darwin the secure-storage module defines
  exactly one store, so the file is the whole store rather than a
  fallback, and the login/refresh loop writes the `claudeAiOauth`
  envelope with `accessToken` and seven siblings there — the same
  envelope the darwin keychain item holds, so one parser covers both.
  The directory follows the runtime's config-dir variables, not the
  home directory by definition (ranger-base-wd4be, landed 2026-09-02
  as d309e2b).
- ASSUMED, probe before trusting: that token returns 200 on the usage
  endpoint. An artifact cannot answer liveness; this needs a real
  login, and because the envelope is measured identical on both
  platforms it needs a credentials-file token from *any* box, not a
  Debian clean room. UNHELD as of 2026-09-04: ranger-base-au0o4 was
  operator-extended 2026-09-01 to cover exactly this token against
  that endpoint, closed 2026-09-02 without answering it, and no bead
  has taken it since — the standing account, and what the gap costs,
  are in V1b. If the probe is ever taken and fails, the adapter's
  honest error still beats today's "keychain unreadable" — and the
  guard-off path in D3 catches it.

The instance ADR reserved this probe when it rejected the file *on
macOS*; the darwin rule as measured is the composite above.

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

  *Amended 2026-09-03 (ranger-base-z089h).* MEASURED 2026-09-03 on the
  operator's own box (ranger-base-4poib): the access token's
  `expiresAt` is exactly 8h after the last interactive `claude` write,
  nothing else advances it, and the `refreshToken` beside it in the
  same envelope (valid three weeks) is never used by anything but that
  loop. Since ADR 0042 every crew runtime runs on the mint and is
  shimmed off the item, so the operator's own shell is the only writer
  there is, and any 8h in which it does not run `claude` is 8h of a
  blind meter. Four ways to meet that were priced, smallest first
  (operator standing order 2026-09-03, "simplest way, or not at all");
  the bullet above stands, and nothing below it is built:

  - **0 — do nothing beyond what is already built.** The at-the-bite
    line already exists twice over: ranger-base-4poib's 401 names the
    expiry it read and how long ago, and ranger-base-ddivo's loud line
    prints the blind age on every watch pass, in `posse status` and in
    the header within one TTL of the death. The hand runs `claude`; 8h
    later the same line returns. What this costs is MEASURED: the
    09-01→09-03 blindness — but that incident was ddivo's quiet cache
    and 4poib's unnamed expiry, both since fixed, so no measured
    failure of *this* option survives their promotion. It is the
    decision until one does. *2026-09-04 (ranger-base-vxbfm):* when
    this option was written "already exists twice over" was half a
    forecast — ddivo was on main (58ac284) but 4poib was still only in
    its session tree, and no dep made this decision wait on the
    landing. Both halves are on main now: 4poib landed as 36e8584
    (rebased from the sha its close named, same patch-id), ddivo as
    58ac284. The sentence is a fact as of this line, and this line is
    the record of the check; the next option that leans on an unlanded
    bead gets a dep on the landing bead, not a sentence.
  - **B — the meter reads the mint.** MEASURED, AND DEAD
    (ranger-base-wkai3 option B, ordered first by the operator's own
    "measure first" ruling of 2026-09-01; the number was
    ranger-base-hs0dl, a P4 one-shot for an uncaged shell). The shape
    was one env-set override in front of the store read — the mint has
    no 8h clock and its `# expires=` stamp already rides the session
    surfaces, so the whole question would have dissolved, and it was
    the smallest shape that removes the clock rather than announcing
    it. It does not dissolve. hs0dl ran twice, 2026-09-04 03:20Z and
    11:09Z, and the env-set key drew an hour-long 429 on its first call
    to the meter endpoint both times while the keychain token read 200
    in the same minute: option B cannot drive a guard. wkai3 closed the
    same day on the operator's ruling, "not B" — the meter stays on the
    keychain OAuth token, and nothing below waits on this number any
    more.
  - **1a — posse performs the OAuth refresh itself.** REJECTED, and
    the rejection is the architecture's, not a permission the operator
    could grant: (i) it makes posse a second writer of a rotating pair,
    which is the lost-update problem D2's S2/S4 rows already describe
    for one writer — whether the endpoint rotates the refresh token on
    use, and whether a running interactive claude re-reads the item
    before it refreshes, are both UNMEASURED and unmeasurable by a
    persona; (ii) it is a new credentialed egress (credpin.go rule 4
    admits exactly one host today) carrying the account's most
    powerful credential; (iii) it re-implements the runtime's private
    OAuth client — client id, endpoint, the composite's write-then-
    delete rules — an interface with no contract and no exit hatch;
    (iv) it voids the property ADR 0042 measured and keeps: the pair
    has one writer *program*, and posse's shim is what holds eleven
    runtimes to it.
  - **1b — posse asks the owner.** PRICED AND PARKED, and the parking
    is a ruling now rather than a queue position: B did measure dead
    (above), so the question was put, and ranger-base-jefo0 closed
    2026-09-04 on the operator's option 0, do nothing — posse never
    runs the operator's runtime unattended, the meter stays on the
    keychain token, and the 401 that names its expiry is the whole
    alarm. §1b stays parked, priced-not-built; no build bead exists for
    it and none is to be filed unless the ruling is revisited. The
    shape, kept for that day: the watch loop execs the operator's
    own runtime binary — uncaged, unshimmed, no mint in its
    environment, the two config-dir variables pinned as the launch pins
    them — with the cheapest invocation that makes the runtime's own
    login loop perform the write (`claude auth status` exists in
    2.1.260; whether it writes is the bead's probe, `claude -p` the
    fallback), only when posse's own read of the item succeeded inside
    the TTL (the keychain answered a moment ago, so the runtime's
    non-transient-failure arm that manufactures S3 is not the one it
    will take) and the snapshot says the credential is inside the
    horizon of D5 or a 401-expired just landed. Single writer kept:
    posse queues the request to the owner. What it costs is the
    operator's to weigh: an unattended run of their account's runtime
    about three times a day, possibly a turn on their window per
    refresh, and two runtime processes on one pair (ASSUMED benign —
    the runtime already handles its own multiple windows — and the
    probe checks it). It is a new unattended actor on the operator's
    account and a new state (S3 under a locked keychain): two of the
    racing signals, so it earns a build only by B's measured failure.
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
meter credential gets no unasked expiry surface *in those two
surfaces*, for three reasons of unequal weight (as ruled 2026-08-28;
re-weighed 2026-09-03 below, where the first is measured false and the
third does not apply to the shape now taken):

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

**The meter's expiry, re-decided 2026-09-03** (ranger-base-z089h,
from ranger-base-4poib). The three reasons above, re-weighed against
the measurement:

- *"There is no hand to warn"* is **false**. MEASURED: the next
  rotation does not happen without an operator; the hand is the whole
  mechanism, and 8h is shorter than a working day. Struck.
- *"It would never be quiet"* is **true and sharper** — 8h is inside
  every window — and it rules the *shape*, not the answer: the meter
  cannot borrow the mints' "inside 14 days" line. What an 8h clock can
  be quiet under is (a) a **gauge**, which is a reading and not a
  warning — always on wherever the plan reading itself is printed, the
  way the window percentages are — and (b) an **alarm keyed on the
  token**, which fires once per credential life because the key is the
  credential's own `expiresAt`, and that value changes exactly when
  the hand acted.
- *"The cost is per-pass"* is **void for this shape**. MEASURED in
  code: the meter's store is read only where the shared snapshot is
  refreshed (`PlanCache.Read` calls the reader on a miss, the reader
  calls `MeterToken`, once per TTL for the whole instance), and the
  parser already returns the envelope's `expiresAt` as
  `CredMeta.ExpiresAt` — which the reader then drops. The expiry is in
  the bytes the instance already reads; the design writes it down.

**The decision: nothing is built.** The do-nothing option leads and
wins (operator standing order 2026-09-03). What the operator asked for
— "learns before the blindness and not after" — is worth at most the
horizon before an 8h death, and it reaches a hand only if one is at
the desk, where the report already answers "when does it die" in one
command. The unasked at-the-bite line (ranger-base-ddivo, within one
TTL, three surfaces, with 4poib's expiry in its sentence) is the
smallest shape that meets the observable "the operator learns", and
no measured failure of it exists after those two promote: the
09-01→09-03 blindness was their absence, not the absence of a
pre-warning. The three shapes below are PRICED AND PARKED, smallest
first, with no build beads; each earns a bead only by a measured cost
of an 8h blindness that the at-the-bite line did not prevent:

1. **The snapshot carries the presented credential's expiry.** The
   shared reading (`planEntry`) gains the `ExpiresAt` and `Source` of
   the credential presented on the read that produced it, written on
   success. Every surface that prints the shared reading — the cockpit
   header's plan segment, `posse status`, `posse cost --plan`, the
   watch preamble — prints beside it the credential's death time as
   the snapshot carries it, with the snapshot's own read time, and
   reads no store to do so: `credential dies 22:51Z (in 6h, read
   14:52Z)`, or `credential EXPIRED 22:51Z (37m ago, read 14:52Z)`.
   A zero expiry prints nothing there and "cannot tell" in the report,
   as before. The gauge stays honest under a 429 cooldown and under
   quiet exactly because it says when it was read: a refresh by hand
   that the cache has not seen yet is a stale gauge, dated, and the
   next successful read moves it. (That same field is
   ranger-base-mc66k's "credential changed" signal — a store read
   during cooldown is not an ask — but that build is that bead's.)
2. **One alarm per credential life.** A governance row keyed
   `meter-credential-dies:<expiresAt as unix seconds>` is raised while
   the snapshot's expiry is inside the horizon and not yet past; the
   pulse's own dedupe by key makes it fire once per token, and the
   next token has a different key. The horizon is one hour — a
   constant, ASSUMED, and not load-bearing: quietness comes from the
   key, not the number, so a wrong horizon costs nuisance and never
   correctness, and it is not a config key (D6: no speculative
   config). Past expiry the row stops and the existing
   `guard-credential` row takes over on the next read, as today. Under
   1b, if taken, the same row is what the owner-refresh reads as its
   trigger and what shows whether it worked.
3. **A fifth failure class — last, and only after a sighting.** The
   state (a 401 with a future stored expiry) is UNOBSERVED; a class
   for it is a path with no caller. If it is ever observed, the first
   shape is a one-line prose change to `PlanFailStale`'s header word
   ("credential refused (401)" is true in all three arms and the
   sentence carries the diagnosis), and the class below is the shape
   after that, kept here so the sighting is not filed as "stale" and
   cleared by a command that cannot clear it. As designed: a 401
   presented with a credential whose stored `expiresAt` is in the
   future is `PlanFailRejected`
   ("credential refused while live (401)", token `401-live`): the
   operator's move is `/login`, because a `claude` run with a live
   access token performs no write (D2 store 3's detective bullet says
   so) and "run `claude` once" is therefore the wrong sentence. A 401
   whose expiry is past, or unknown, stays `PlanFailStale` — same
   move, run `claude` once — and ranger-base-4poib's three-armed
   sentence already tells the two apart. The class is a class by this
   page's own criterion (a different next move), the governance key
   follows the token rather than the status code so the two 401s are
   two rows, and ADR 0018 §2 is untouched: park-vs-degrade reads no
   class. The future-expiry state is UNOBSERVED as of this writing;
   the class exists so that its first observation is not filed as
   "stale" and cleared by a command that cannot clear it.

Unchanged, restated: expiry gates nothing. The gauge and the alarm
decide no pass, start no clock, and park nothing; the read's success
or failure remains the only actuator.

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
- **Keep the item name a constant and let a suffixed box read 44**
  (2026-09-02, ranger-base-ig4op). Under the composite the 44 falls to
  the file, the file is absent on a healthy S1 box, and the sentence
  says the keychain is empty — the false-diagnosis class this page and
  ADR 0018 exist to keep apart from blindness. Rejected on the ground
  S3 was: the fix is one derivation in a resolver wd4be already
  builds.
- **Try the constant, then the derived name.** Two execs per pass, and
  where both items exist (an operator who logged in once before setting
  the variable) the constant hits an item the runtime no longer
  presents — the inversion in a new costume. The runtime looks in one
  place; the mirror does too.
- **Enumerate the keychain for items under the prefix.** The keychain
  CLI finds by exact attribute, not by prefix; a dump reads every item
  on the box, and "which one is the runtime's" still needs the suffix.
  Unmeasurable by any persona besides.
- **Read the variables off the running `claude` process** (the
  ranger-base-eje6d trick). Which claude: the crew runtimes hold
  posse's mint and posse's environment, so the answer is posse's own
  env with a process walk in the way. The environment is the rule;
  hold the rule.
- **A config key naming the item.** A fact belonging to no lever posse
  holds: it drifts the day the operator changes a variable, silently,
  and the runtime never consults it.
- **`golang.org/x/text/unicode/norm` for NFC.** Priced: one pure-Go
  module, holds no state hostage, exit hatch is deleting the import
  and the arm. Not taken: no non-NFC config-dir value has been
  observed, the file resolver of the same bead would need it too or
  the two would disagree on one box, and the note on the store's name
  covers the residual. Taken the day a non-ASCII value bites — in both
  resolvers at once.
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
  freshness bug. Standing rejection, re-argued in full 2026-09-03 as
  D4 §1a after the 8h measurement made it the clever fix: the value
  went up and the physics did not move. The single-writer answer to
  "the owner is not writing often enough" is to *ask the owner*
  (D4 §1b, the operator's call), never to add a writer.
- **The refresh verb execs `claude` for the meter, by the operator's
  hand** (2026-09-03). Rejected: "run `claude` once" is already one
  command, the verb would save nothing, and it does not touch the case
  that matters — the unattended 8h — which is 1b's whole question.
- **A per-pass stderr line for the meter, like the mints'**
  (2026-09-03). Rejected: it is the "never quiet" reason, verbatim —
  the line would print on every pass of every day. That reason is
  MEASURED (ranger-base-b1al) and carries this rejection by itself.
  *Corrected 2026-09-04 (ranger-base-vxbfm):* the bullet closed on
  "the gauge sits where the reading already prints; the alarm fires
  once per token" — present tense for two shapes D5 priced and PARKED.
  Neither is built. They are where a quiet meter surface WOULD go if
  either is ever built (D5 shapes 1 and 2, V14/V15); the rejection
  above does not wait on them and does not rest on them.
- **A config key for the alarm horizon** (2026-09-03). Rejected under
  D6: a number nobody has measured does not become a knob; it becomes
  a constant with ASSUMED on it, moved the day a measurement says
  where.
- **Reading the store during a 429 cooldown to refresh the gauge**
  (2026-09-03). Not taken here: it is one `security` exec per TTL
  without a request, which this page permits, but the field it feeds
  is ranger-base-mc66k's streak reset, and — *corrected 2026-09-04
  (ranger-base-vxbfm); this bullet read "the gauge's dated read time
  already keeps it honest", a fifth instance of the same present-tense
  slip this pass fixes in four other bullets and rows on this page* — the gauge's dated read time WOULD keep it
  honest without it, if D5 shape 1 is ever built. Nothing here is
  built, so this bullet rejects a refresh for a surface that does not
  exist yet; the mc66k ground stands on its own either way.
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
  (ranger-base-k6ha). The honest accounting, *revised 2026-09-04
  (ranger-base-vxbfm) because it still recorded the 08-28 weighting
  and contradicted D5 as amended 60 lines above*: the load-bearing
  reason (no hand to warn) is MEASURED FALSE and struck
  (ranger-base-4poib — the hand is the whole mechanism, and 8h is
  shorter than a working day); the noise reason is MEASURED, not
  assumed — ranger-base-b1al read a 6h30m horizon — and it is what
  carries this rejection now, since an 8h clock sits inside every
  window and the meter cannot borrow the mints' "inside 14 days"
  line; the cost reason is a per-pass `security` exec that the
  session-only rule avoids entirely, and it still holds for THIS
  shape — D5's re-weigh voids it only for the snapshot-carried gauge,
  which reads no store.
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
  *2026-09-03, corrected 2026-09-04 (ranger-base-vxbfm):* the cost
  half of this rejection ("arming the per-pass keychain exec") is
  void — the expiry rides in the snapshot, see D5 as amended. The
  rest of this annotation was written in the present tense ("the
  gauge now does print an EXPIRED state", "the alarm proper fires
  before the bite") for two shapes that were then, and are now,
  PARKED and unbuilt; read it as conditional. IF D5 shape 1 is built
  its gauge prints an EXPIRED state, and IF shape 2 is built its
  alarm fires before the bite. What survives of the rejection either
  way is that this line would be a *reading with a date*, not a
  second alarm. Nothing of either is built — D5: "the decision:
  nothing is built", V14–V16 parked.

## Verification (laurie's checklist)

- V1 (split 2026-09-02, ranger-base-17rt4: half is measured, and the
  remaining half no longer needs the clean room):
  - V1a, MEASURED 2026-09-01 (ranger-base-ydjz) from the
    checksum-verified linux-x64 2.1.258 release binary, no login: off
    darwin the file `CredentialsFile()` names is the runtime's only
    store, and the login loop writes the `claudeAiOauth` envelope with
    `accessToken` there. Write-up: NOTES.md, "What the shipped artifact
    actually does".
  - V1b (probe, any box holding a credentials-file token — not Debian,
    not a container): that token returns 200 on the usage endpoint. An
    artifact cannot answer liveness. UNHELD, and said so here
    2026-09-04 (ranger-base-vfx8g, from ranger-base-v4zlr): no bead
    holds this probe, and no seat on this box can take it. What the
    three beads that touched it actually did: ranger-base-au0o4 held
    it, operator-extended 2026-09-01 to cover exactly this token
    against that endpoint, and closed 2026-09-02 without it — the
    endpoint answered 429, and so did a control request carrying no
    Authorization header at all, so those codes were about the source
    and said nothing about any credential. ranger-base-dvxac, its
    re-ask after the window, closed the same day and also without a
    code for this token. ranger-base-hs0dl (closed 2026-09-04)
    measured a *different* credential — the env-set session mint the
    Context section names, 429 on both runs with the keychain token
    reading 200 in the same minute — which settles ranger-base-wkai3
    option B, not this. And the file is absent on
    this box (ranger-base-ydjz, ranger-base-au0o4), so nothing here
    can present the token V1b names: taking it needs a box that holds
    one, and repointing this line at a bead that cannot run on the box
    it is filed against is how it went stale the first time. WHAT THAT
    COSTS: D2's non-darwin adapter stays built-but-unconfirmed with no
    dated end to that state. `meterUnconfirmed`
    (`internal/posse/credential.go`) says exactly that in its error
    text, and it is now the only thing carrying the gap to a user — so
    that wording does not come out until a code, 200 or not, is
    measured from a credentials-file token and the bead that measured
    it is named here.
- V2 (unit): a non-darwin build never execs `security` — the GOOS
  switch is total, and no non-darwin error string contains "keychain".
- V3 (unit): `NoSource` renders as guard-off/unconfigured in the
  cockpit header and the dispatch line; the blind clock does not start;
  ADR 0018's park/degrade behaviour is unreached.
- V4 (operator): `posse refresh` refuses without a TTY and inside a
  persona session; crew PIDs carry the deny line.
- V5 (unit): refresh writes 600 under 700; stamps round-trip through
  the seam's `ExpiresAt`; `posse envs` output still never contains a
  value. *Added 2026-09-05 (ranger-base-q3n4e):* the seam's session
  read answers from the env set files under a scratch home with the
  variable set in the test process to a value that must never be
  returned — the environment is not a store — and with the name in no
  set it returns the refresh-verb sentence, not the environment's value.
  ADR 0039 V6–V8 carry the probe-side rows.
- V6 (unit, amended 2026-08-28 ranger-base-swqk): a session mint's
  `# expires=` inside 14 days appears in the header and once per pass
  on stderr; expired renders distinctly; zero renders as "cannot tell"
  and warns nothing. And the purpose split holds: a meter envelope
  expiring inside the window appears in the report and in **neither**
  timer surface — pinned with a positive witness on the same box (a
  near-expiry session mint that does appear), so the absence half
  cannot pass by measuring nothing. *Amended 2026-09-03
  (ranger-base-z089h), corrected 2026-09-04 (ranger-base-vxbfm):*
  "neither timer surface" means the mints' header segment and the
  per-pass stderr line. The amendment then said the meter's expiry
  "now appears" in the **plan** segment beside the reading (V14). It
  does not: V14 is PARKED and nothing is built, and a verification row
  is the one place a reader cannot check a claim against, because the
  row IS the check. Restated: IF V14's gauge is ever built the plan
  segment is where it goes, and that is not a violation of this row —
  the pin keeps the mint segment and the per-pass line meter-free
  either way.
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
- V11 (unit, added 2026-09-02 ranger-base-ig4op; either box, no build
  tags): the item-name derivation. Both variables unset →
  `Claude Code-credentials`; `CLAUDE_SECURESTORAGE_CONFIG_DIR`
  present-but-empty beside `CLAUDE_CONFIG_DIR=/tmp/cfg` → the constant;
  `CLAUDE_CONFIG_DIR=/tmp/cfg` → `Claude Code-credentials-519e587f`;
  secure-storage `/tmp/cfg` beside `CLAUDE_CONFIG_DIR=/other` →
  `-519e587f`; `CLAUDE_CONFIG_DIR=<home>/.claude` → suffixed. The NFC
  pair: `/tmp/café` composed → `-0873cca0`, decomposed → `-16eb4464`,
  and posse yields the latter for the latter (no normalization) with
  the note on the store's name. Fixture digits MEASURED 2026-09-02 with
  node's sha256 (the runtime's own function) and shasum, which agree.
  Mutation checks, each must red a pin: 7 or 9 digits; hashing the
  file path instead of the directory; hashing the config dir while the
  secure-storage variable is set; testing the path instead of the
  variable for default-ness.
- V12 (unit, added 2026-09-02): `keychainCmd`'s argv carries the
  derived name; the unreadable sentence, the seam's `Source` and the
  44-and-no-file sentence print it; the runbook row keeps the default
  spelling and says when it grows a suffix
  (`credentialrunbook_qa_test.go` pins the sentence).
- V13 (operator, added 2026-09-02; every crew PID denies the keychain
  CLI): on a box with `CLAUDE_CONFIG_DIR` set, after a `claude` login,
  the keychain holds an item under the derived name — moves "what the
  runtime writes" from code-read to observed. Second arm: set the
  variable with a trailing slash and log in; the item's suffix says
  whether the config-dir arm hashes the value verbatim (ASSUMED in D2)
  or a cleaned path.
- V14–V16 are PARKED with the shapes they pin (D5 as amended: nothing
  built; the beads cut for them were closed not-doing the same day,
  ranger-base-z0gkm / m6y0v / zxpcz, and a future build re-files
  against these rows). V17 is PARKED with D4 §1b, below.
- V14 (unit, parked 2026-09-03 ranger-base-z089h; either box): the snapshot written by a successful
  read carries the presented credential's `ExpiresAt` and `Source`;
  the four surfaces that print the shared reading print the death time
  and the snapshot's read time beside it from the FILE — pinned by a
  witness the store read would trip (V8's shape): with a stubbed
  keychain CLI that counts its invocations, a cockpit tick, a `posse
  status`, a `posse cost --plan` and a watch preamble over a fresh
  snapshot add zero invocations. A zero expiry prints no gauge; a past
  expiry prints EXPIRED with "ago"; a snapshot older than the expiry it
  carries still prints the read time. Mutation checks: drop the read
  time; render from a live store read instead of the snapshot; render
  a zero as fresh.
- V15 (unit, parked 2026-09-03): the alarm
  row's key is the token's own `expiresAt`; two ticks inside the
  horizon on one snapshot raise one row; a snapshot with an advanced
  expiry raises a second with a different key; a past expiry raises
  none; a zero expiry raises none; and no pass outcome, clock or park
  changes with the row present (expiry gates nothing — the positive
  arm runs a 99%/99% reading through three passes with the alarm up
  and asserts the same decisions as without it).
- V16 (unit, parked 2026-09-03; ranger-base-4poib landed 2026-09-04 as
  36e8584, so this row's precondition is met and only the parked shape
  remains): a 401 with `AuthFailure.ExpiresAt` in the
  future classes `PlanFailRejected`, token `401-live`, `Stale()` false,
  sentence names `/login` and never "run `claude` once"; past and zero
  expiry stay `PlanFailStale`, token `401`; the governance row keys
  differ between the two; `PlanFailureOf` still returns exactly one
  class for every error fixture in the table. Mutation check: collapse
  the two keys.
- V17 (operator, added 2026-09-03; uncaged shell only) — PARKED, NOT
  RUN. This row's trigger did fire: ranger-base-hs0dl measured option B
  dead on 2026-09-04. But the question it fed was ruled before the
  probe could inform it — ranger-base-jefo0 closed the same day on
  option 0, do nothing — so D4 §1b is parked and there is no
  build-or-refuse left here to answer. The shape, kept against the day
  that ruling is revisited: which runtime verb performs the refresh
  write non-interactively, what an open interactive window does across
  it, and the keychain's lock state during the run. If it is ever run
  it is the operator's and an uncaged shell's, and the build bead is
  filed after it, not before.
