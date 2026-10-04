# ADR 0019 — Credential ownership, lifetime and failure

*Status: accepted current contract, consolidated 2026-09-05 by operator ruling · provider seam built;
credentials-file live-endpoint confirmation remains unverified · D7 added 2026-10-04
(ranger-base-41zyo): the metered refusal reaches ADMISSION, and the metered-credential
CLASS question gets its verdict — no cap key is added and no metered credential is
admitted · owner: architect.*

## Context

Doing nothing leaves the current credential contract assembled from a
proposed header and repeated revisions. Keep scoped mint isolation and the
runtime's ownership of rotating credentials; no credential mechanism is
removed. Dated binary readings and live probes remain separately archived.

## Decision

**1. One acquisition seam, two owners.** `Read(runtime, purpose)` returns
secret, source metadata, expiry (zero means cannot tell) and a typed error.
The `session` purpose reads the operator-provided scoped mint from the env
set **files** under the home, the store of record — selected by the sets the
launch names, in launch order, the last assignment of the name winning —
and never from the reading process's own environment (amended 2026-09-05,
ranger-base-q3n4e, from ranger-base-mvrke: the environment arm the first
build carried is retracted, not kept beside the file. A posse process is
never the launched runtime, and the runtime scrubs the mint from its
children — MEASURED by elimination — so that arm answered nothing.
`Read(runtime, session)` keeps its signature and reads the persona-less
list; a second entry point takes the launch's list; ADR 0039 D3d as amended
carries the ruling, the selection rule and the exposure answer). The
`meter` purpose reads the runtime-owned rotating credential where that
runtime keeps it. Posse never copies or rotates that pair. Its writer
is the runtime's own login/refresh loop under the operator's control.
The home owns scoped mints, with restrictive directory/file modes; a vault
can replace their provider behind the same seam. A reserved secrets store
does not create a second acquisition path or imply it holds credentials.

**2. Read the platform's actual store, with explicit failures.**

| Case | Provider behavior |
|---|---|
| Darwin meter | Absolute system credential-binary read of the derived item; fall back to `CredentialsFile()` **only on item-not-found exit 44** |
| Darwin exit 36, empty successful output or another read failure **that returned an exit status** | `CredUnreadable`; never silently read a stale fallback because this binary lost its ACL |
| A read that **never ran** — binary absent, not executable, or the fork/exec failed | `CredReadNotRun`: no exit status came back, so nothing was learned about the store; names the store and the binary, carries **no** ACL fix, and does not fall through (amended 2026-09-06, ranger-base-h8u0l: this row was inside "another read failure" above, so a `security` that could not be executed rendered the 2026-08-24 ACL sentence byte for byte — a cause that cannot be the cause, because an ACL is checked by a binary that RAN. Reproduced on darwin and caught in CI on the ubuntu leg of run 34050764993. The distinction is made on the error's TYPE, not its text: a failure to exec writes no stderr, so there is nothing to parse) |
| Darwin item missing and no fallback | `CredUnreadable`, names the attempted stores and possible ACL/absence cause; not structural NoSource |
| Non-Darwin meter | Read the runtime's credentials file through the same envelope parser and config-directory resolver |
| No adapter or structurally absent platform source | `NoSource`: off with witness and remedy; no blind clock |
| Existing source unreadable or endpoint refusal | Name source and failure; remote blind/headroom policy belongs solely to 0010 §5 |

The runtime's own Darwin composite accepts a wider null set {0 with no
output, 36, 44}; posse deliberately narrows fallback because an ACL is per
binary. A live item wins even if the fallback file is newer. An expired
envelope may still be presented; expiry alone is never a gate. Do not turn
an unreadable ledger or credential into an empty successful reading.

Use one resolver for credential directory and item identity. Secure-storage
config-dir presence takes precedence over ordinary config-dir; present-empty
has meaning. The default item applies only when no variable names a directory,
not whenever the path happens to equal the default. Otherwise append the
first eight hexadecimal digits of SHA-256 over the named directory string
in Unicode NFC, which is the runtime's own form: it normalizes the resolved
string before it hashes. Do not otherwise clean the string before hashing —
no path cleaning, no slash trimming, no case folding. NFC is applied to the
item NAME only; the directory posse opens, walls and prints keeps the spelling
it was handed (amended 2026-10-03, ranger-base-4ch00: this page declined NFC
while Go's standard library had none and the price of being wrong was a
diagnostic note; x/text entered the binary for the trust key under
ranger-base-d88rp, after which a decomposed non-ASCII directory still derived
an item the runtime never wrote — MEASURED `-16eb4464` where the runtime
derives `-0873cca0` — for no dependency saved and zero bytes of binary. The
decline was a price, never a design, and is reversed; the "as spelled" note
retires with it. Pricing in docs/notes.d/ranger-base-4ch00.md).
A generic password is identified by service **and account**, so the read
names both: the account is the runtime's own rule — `USER`, else the OS
username, else the literal `claude-code-user` when the value falls outside
`[a-zA-Z0-9._-]` — derived once and printed in the store's own sentence
(amended 2026-09-28, ranger-base-tghn5: the read named the service alone
until then, so on a box holding it under a second account posse read whatever
the keychain reached first and reported an item the runtime never wrote).
`credentialDirNamed`, `keychainItem`, `keychainAccount` and `CredentialsFile`
are the concrete definitions; store locations are not independently
reconstructed by callers.

On a Claude launch, `credentialDirPin` fixes both directory variables in the
launcher's flag-settings scope, preserving whether a variable named the
directory and therefore item identity. A plain exported environment cannot
outrank later user/project settings or the daemon/login-shell split. Refuse
env sets that try to carry the reserved names. Retain the OS-specific
credential-file read denies over default and moved paths. The operator's
uncaged runtime and a pane with a different HOME remain outside what this
launch pin proves; no new machine-wide configuration is authorized here.

**3. Mint before runtime** (folded 0042 D1–D6). A crew runtime uses its scoped
session mint. Where `CredGateCollision` detects the PID shim on `Runtime.CredBin`,
require `CageCredential` in the selected env set before any session exists;
missing refuses, and `--allow-degraded` cannot make it authenticatable.
The container mint precondition also remains. With a mint, no collision
warning is needed. This does not refuse an operator launch with no collision.
The rendered credential shim exits a **read failure**, outside {0,36,44};
the shipped 1 is one valid value, not the only one. Null would reopen the
runtime's plaintext fallback and possible competing rotation.

Nested CLIs receive the same mint explicitly; do not strip the gate PATH
or export an unsuppressed alias into every tool child. A refusal log records
session activity, with runtime reads distinct from model intent and rc
activity. A second caller-classified log cannot prove its classification.
0002 owns enforcement and its cooperative limits. The named SHELL-only
escape remains unbuilt unless an actual runtime has no env-credential path;
its non-shell execs would leave the shim boundary.

**4. Credential writes require the operator.** The refresh verb is
interactive-only and refuses persona sessions; PID denial reinforces it.
For sessions it invokes the runtime's human mint flow or accepts an explicit
paste, writes only the selected env set and known mint/expiry stamps, and
uses restrictive modes. No invented expiry and no metered API credential
substitution. For meters it writes nothing: report the source, expiry and
the operator action appropriate to the failure. No-argument use is a report.
The 2026-09-04 rulings retained option 0: no unattended owner-refresh and
no posse OAuth-refresh client. The meter-mint experiment did not establish
a usable meter credential and was rejected; it does not authorize a new
provider preference.

**5. Lifetime is reported, not inferred.** Session mint expiry comes from
its stamp; its existing near-expiry header/pass warnings retain the 14-day
window. The on-demand report answers for both purposes. Meter expiry is
reported in the existing at-failure diagnostic and blind-age surfaces within
the shared cache cadence. Keep the operator's option 0: no new meter-expiry
gauge, per-token alarm or unobserved failure class. The earlier claim that
rotation needed no operator was measured false; the recorded access-token
life was eight hours after that writer's action. That measurement changes
the explanation, not the approved no-new-observer decision. Success/failure
of the read remains the actuator under 0010, never an expiry timestamp.

**6. No speculative provider configuration.** Unsupported runtime/purpose
combinations remain explicitly unavailable; no `meter_source` key is added
for a second adapter that does not exist. The provider seam is the exit
hatch, and runtime-owned state stays with its owner.

**7. A metered credential is a CLASS, and the class is refused where it is
ADMITTED** (added 2026-10-04, ranger-base-41zyo).

*The reach of the refusal.* The metered API-key variable — the one name
`meteredCredentialNames` holds, in `internal/posse/meteredcred.go` — is
spending, refused as a session credential by the operator's ruling of
2026-08-20 (rangerhq-kiz). That refusal was spelled in one place and
performed in another. MEASURED 2026-10-04: a runtime yaml whose `cage_cred:`
named that variable resolved `CageCredential` to it and
`CheckCageCredential` returned **nil** —
the loop's question is whether the declared name is among the env-set names
the launch will inject, and a metered declared name that is present answers
it yes. `CheckCredGate` (D3, ADR 0042 D2) inherited the same answer, and
`posse runtime check`'s own `cage_cred` row had said "a METERED api key is
not accepted as this credential" since it was written. Three surfaces, one
claim, and the two that decide whether a session starts did not make it:
the refusal was the WRITE path's alone (`posse refresh`, D4), which protects
the operator from their own typing and not the shop from a promoted file.

The predicate is therefore one definition — the NAME set and the VALUE shape
in `internal/posse/meteredcred.go` — asked at the write and at **both**
admission preconditions. A runtime whose declared session credential is
metered spending launches nothing, at any tier, caged or not, and the
refusal is not waivable by `--allow-degraded`: that hatch is for a gate the
wall could not realize, and this is spending authority a session may not
hold. Two limits are structural and stay open, stated rather than closed:
admission is handed NAMES and never values (posse does not read what the
session credential holds, D1), so a metered key pasted by hand under the
mint's own name is admitted and the shape check remains the write path's;
and the set is a set of exact names, so a renamed key is invisible to it,
exactly as `meteredKeyPrefix` has always said of the shape. No fold and no
substring test over the name's suffix: either would refuse another provider's decided
session credential and claim a knowledge of env naming unix does not share.

*Can the class be admitted without weakening that refusal?* In principle
yes, and the reason is that every predicate above is spelled on the
**declared session credential**. An engagement credential is not that, so
admitting one is an additive, separately gated name and never a relaxation
of a predicate here. The order is not negotiable: the refusal is made total
first, which is what this amendment does, and the class is admitted second
or not at all.

*Where a cap would live.* The home's promoted `config.yaml`, and the reason
is already built: `config.yaml` is in `PromotedPaths`, `HomeConstitutionPaths`
keeps every member of that set out of every seatbelt write grant (ADR 0015
§2/§3/§7), and `posse promote` is the operator's verb and is denied in every
crew PID. So at L2 and above the cap is unreachable by the session that would
spend against it, by a wall that exists for other reasons and needs nothing
new. Not an environment variable and not the env set that carries the secret:
a session rewrites its own children's environment freely, so a cap in the
environment is a cap the spender edits. Not the PID: a PID is per-persona and
an engagement is not. Honest residue — an L0/L1 seat has no seatbelt, so for
those tiers the cap is unreached rather than unreachable.

*What it does when hit — and why "hard cap" is the wrong name for it.* It
would park, as §5's table parks, and it would have to be **labelled the soft
edge**. Every brake posse holds is dispatch-time: it refuses to HIRE. Nothing
here stops an in-flight turn from spending, and posse has no authority at the
biller at all. A ceiling enforced at hire time bounds NEW spending and leaves
the running session as residue — the shape §5 already writes down twice ("a
spending cap cannot bound a plan window, and neither can a clock"). Shipping
that under the name "hard cap" would be this page's own rejected pattern:
false authority.

*Scope is the engagement, and the engagement is the store of record.* Not the
session. The scope of an engagement is the `.beads` store of record the launch
directory resolves to (ADR 0055) — the boundary posse already computes and
already rides the session env. A per-session cap is mintable by restarting,
the exact flaw ADR 0028 §2 fixed for `budget_pass` by re-denominating it on a
wall clock; a per-persona cap lets one engagement's money fund another's.

*The admissible shape is a 3P inference credential, and it is the only one
that can carry a cap that is actually hard.* Its ceiling is a quota or budget
in the customer's own cloud account, enforced by the biller: a posse session
has no API surface to it, which is a stronger statement than "a profile
denies writing it". The credential is role-scoped and short-lived, so D5's
"lifetime is reported, not inferred" gets a real expiry instead of "cannot
tell". Billing lands directly on the end user under their own agreement,
which is what the published compliance terms require of usage on an end
user's behalf. And no `sk-ant-api…` value enters an env set, so the name and
shape refusals above stay total rather than acquiring an exception.

**Verdict: a raw metered API key stays refused — no exception, no
cap key, no engagement block.** D6 forbids configuring a provider shape this
instance has no instance of, and the preconditions for admitting even the 3P
shape are an operator ruling on the money line and a real engagement, neither
of which this bead had; the ruling is filed as `ranger-base-s1hq3`, which
carries the three things only the operator can decide and what "verified"
would have to mean. What a future amendment must carry, named here so it
is not re-derived: the engagement scope, the ceiling, WHO enforces it and
where, and a **dated operator attestation that the ceiling was verified at
the biller** — because posse can only ever record evidence of a cap it does
not hold, and a recorded cap nobody checked is worse than none. No money was
spent and no credential minted under this amendment.

## Evidence, consequences and alternatives

MEASURED, dated in the pre-simplification page (git history, below): release-binary composite/file behavior,
config-directory/item resolution, scoped-mint sessions behind the shim,
rendered read-failure exit and settings pin. **Unverified:** a real token
from the credentials file returning 200 at the usage endpoint. The old V1b
probe was unheld as of 2026-09-04; a session-mint probe and a source-wide 429
control do not answer it. Non-Darwin remains built but unconfirmed at that
live seam; this documentation run makes no fresh authentication claim.
MEASURED 2026-10-04 (D7, ranger-base-41zyo): a runtime yaml whose
`cage_cred:` names the metered variable → `CageCredential` returns that name
and `CheckCageCredential`, handed the same name among the launch's env-set
names, returns nil, so the launch is admitted; `posse refresh` refuses that
name and that value shape. The reading was taken on this tree before the fix and is reproduced as
the pre-mutation state of `internal/posse/meteredcred_qa_test.go`, whose six
arms are mutation-checked one-for-one (the file's header lists which edit reds
which arm). ASSUMED, and stated because the absence of a reading is not the
absence of a case: no shipped runtime names a metered credential — claude's
is its scoped mint, bob's is its own provider's, codex and grok are
undecided — so this instance has never exercised the admitted path, and
whether any other instance's `cage_cred:` names one is UNKNOWN and
unmeasurable from here (posse sends no telemetry).
The session read's store is pinned (added 2026-09-05, ranger-base-q3n4e):
it answers from the env set files under a scratch home with the variable
set in the test process to a value that must never be returned, and with
the name in no set it returns the refresh-verb sentence, never the
environment's value. ADR 0039 V6–V8 carry the probe-side rows.

Zero runtime/config/state/actor/flag removals. ASSUMED: rare ACL-as-44 and
split-store cases, non-ASCII config-dir behavior beyond the recorded probes
(the item name is exact since ranger-base-4ch00; the credentials FILE is
still opened at the spelled directory while the runtime resolves its own
directory in NFC, a difference only a normalization-preserving filesystem
can show, and no such box has been measured — darwin's APFS is not one),
and reader-maintenance savings. Rejected: doing nothing to the record;
copying rotating tokens; a second OAuth writer; literal runtime-null fallback
in the meter adapter; mint-as-meter without entitlement evidence; autonomous
owner refresh; alarms without a measured failure of existing diagnostics;
keeping the as-spelled item hash once NFC was already linked (2026-10-03,
ranger-base-4ch00 — a diagnostic note is a confession, and a confession kept
when the cure is one call is not restraint); and, under D7 (2026-10-04,
ranger-base-41zyo), admitting a raw metered API key as its own
capped class, an `engagements:` config block for a provider shape this
instance does not have (D6), a cap carried in an env set or the environment
beside the secret it bounds (the spender edits it), a per-session or
per-persona cap (restartable; fungible across engagements), and a
dispatch-time ceiling named a hard cap (false authority — posse parks hires
and cannot stop an in-flight turn).
If ownership were removed, concurrent refresh/authentication could break;
this simplification preserves it.

## Lineage

| Was | Here |
|---|---|
| 0019 D1–D6 and accepted amendments | Decisions 1–6 above |
| 0042 D1–D6 | Decision 3; gate policy directly in 0002 |
| Reversed premises, parked designs and V1–V17 evidence | Pre-simplification page in git history (below), with their original dates and execution limits |

Dated evidence and earlier alternatives: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0019-credential-architecture.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
