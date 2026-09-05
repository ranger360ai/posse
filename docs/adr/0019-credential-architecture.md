# ADR 0019 — Credential ownership, lifetime and failure

*Status: accepted current contract, consolidated 2026-09-05 by operator ruling · provider seam built; credentials-file live-endpoint confirmation remains unverified · owner: architect.*

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
| Darwin exit 36, empty successful output or another read failure | `CredUnreadable`; never silently read a stale fallback because this binary lost its ACL |
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
first eight hexadecimal digits of SHA-256 over the named directory string.
Do not clean the string before hashing. Posse does not add Unicode NFC
normalization; non-ASCII names retain the documented diagnostic limitation.
`credentialDirNamed`, `keychainItem`, and `CredentialsFile` are the concrete
definitions; store locations are not independently reconstructed by callers.

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

## Evidence, consequences and alternatives

MEASURED, dated in the pre-simplification page (git history, below): release-binary composite/file behavior,
config-directory/item resolution, scoped-mint sessions behind the shim,
rendered read-failure exit and settings pin. **Unverified:** a real token
from the credentials file returning 200 at the usage endpoint. The old V1b
probe was unheld as of 2026-09-04; a session-mint probe and a source-wide 429
control do not answer it. Non-Darwin remains built but unconfirmed at that
live seam; this documentation run makes no fresh authentication claim.
The session read's store is pinned (added 2026-09-05, ranger-base-q3n4e):
it answers from the env set files under a scratch home with the variable
set in the test process to a value that must never be returned, and with
the name in no set it returns the refresh-verb sentence, never the
environment's value. ADR 0039 V6–V8 carry the probe-side rows.

Zero runtime/config/state/actor/flag removals. ASSUMED: rare ACL-as-44 and
split-store cases, non-ASCII config-dir behavior beyond the recorded probes,
and reader-maintenance savings. Rejected: doing nothing to the record;
copying rotating tokens; a second OAuth writer; literal runtime-null fallback
in the meter adapter; mint-as-meter without entitlement evidence; autonomous
owner refresh; alarms without a measured failure of existing diagnostics.
If ownership were removed, concurrent refresh/authentication could break;
this simplification preserves it.

## Lineage

| Was | Here |
|---|---|
| 0019 D1–D6 and accepted amendments | Decisions 1–6 above |
| 0042 D1–D6 | Decision 3; gate policy directly in 0002 |
| Reversed premises, parked designs and V1–V17 evidence | Pre-simplification page in git history (below), with their original dates and execution limits |

Dated evidence and earlier alternatives: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0019-credential-architecture.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
