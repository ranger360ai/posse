## The four credential-failure classes, and the two surfaces that name them (rangerhq-pwpx)

ADR 0019 D2 as amended (accepted 2026-08-28, option (b)). The plan guard
keeps reading the macOS keychain — original D2's off-ramp died when P1
measured HTTP **403** on a setup-token — so what ships instead is the honest
version of staying the second reader of somebody else's rotating credential:
**name the four ways it breaks, on the surfaces an operator actually looks
at.** `state/plan-usage.log` already recorded the status. Nobody reads a log
to find out why the shop is idle.

`rangerhq-ytyj` had already built two of the four. This bead is the other
two and the header.

### The classes

| class | type | the operator is told | never |
|---|---|---|---|
| unreadable | `*CredUnreadable` (new) | keychain item "…" unreadable — this binary's keychain ACL may have been dropped by `make install`; grant access when prompted, or run `claude` once | "stale", "refresh" |
| 401 stale | `*AuthFailure`, `Code == 401` | credential stale — run `claude` once to refresh | — |
| 403 wrong kind | `*AuthFailure`, `Code == 403` | not entitled to plan windows — a setup-token never will be, and this is not a freshness problem | **"refresh"** |
| 429 | `*RateLimit` (unchanged) | the cooldown it already bought | "credential" |

`*GateRefusal` is listed beside them and is **not** a fifth: it is one of
posse's own L1 shims refusing a command posse ran. It keeps its own type and
its own sentence, and `runtimeStore.fail` steps around the new wrapping to
leave it that way — reporting it as a credential outage is the reading that
got `plan_guard_blind_max: 0` set for hours on 2026-08-24.

### What was actually missing

Two things, and only one of them was a sentence.

- **The unreadable class had no type and no move.** `keychain item "…"
  unreadable` was true, and it is the same words a *gate refusal* used to
  produce. `CredUnreadable` gives it a type, and the store gives it the
  **fix**: `runtimeStore.Fix` is per store, because a keychain's cause is a
  per-binary ACL and a plain `~/.claude/.credentials.json` has no ACL to
  re-grant. One class, two platforms, two moves.

  **The fix rides on the READ, never on the ENVELOPE** — `failRead` vs
  `failShape`, and the split is not cosmetic. ADR 0019 V7 requires the two
  platforms' shape diagnosis to be one sentence with one store name
  substituted (`credseam_test.go` pins it byte for byte, and it went red the
  first time this bead attached the fix to both). It would also have been a
  *wrong* sentence: a keychain that answered with a renamed key did not lose
  an ACL, and re-granting one fixes nothing. The shape failure keeps its
  class — it is still a credential condition and not weather — and its move
  is already the last clause of its own diagnosis: teach `credShapes` the new
  name.
- **The cockpit header did not carry the class at all.** `planRead` discarded
  the read error; the header said `plan — · guard blind 40m` for every one of
  the four. It now says `plan — · guard blind 40m · credential stale (401)`.

### `PlanFailureOf` — the short name, off the TYPE

The header has room for a few words, not for the sentence. `PlanFailure` is
that short name and `PlanFailureOf(err)` derives it from the error's type —
never from its prose. A cockpit grepping for `"401"` would undo, in one line,
the reason `AuthFailure`, `RateLimit` and `GateRefusal` were each given a
type.

The empty class is not a fifth one: a dead socket, a 500, a response of the
wrong shape are **blind**, and the header says blind and stops. `PlanFailureOf`
returning `""` is that answer, and the 500 arm of both surface tests is the
control that keeps it true.

### The hour after a 429

`PlanCache.Read`'s cooldown branch used to return a bare `Die`, so the whole
tail of a Retry-After — the part an operator actually stares at — went back
to saying "blind". It now returns `*planCooldownErr`, whose **sentence is
still its own** (nobody asked the endpoint anything, so there is no status
line to quote) and whose **class is the 429's**, carried by an `Unwrap` to a
`*RateLimit`. Nothing else moved: plancache's own 429 branch and its read log
are reached only by a read that was made, and this returns before either.

### What did NOT change

ADR 0018 §2, byte for byte. Park-vs-degrade reads no diagnosis string, no
threshold moved, no clock moved, and a credential failure buys no extra
blind tolerance and costs none. Every line here is observability.

`modelavail.go` is the second `KeychainToken` caller and is explicitly out of
this decision. It was not touched — but it reads the same seam, so its errors
inherit the fix sentence. That is the seam working, not a fix in passing.

The runbook (`docs/runbooks/credential-rotation.md`) does **not** ride on
these messages: its guard section is still parked on `rangerhq-m10j`, and
citing a page that is empty where the operator would look is worse than the
one-line fix that is now in the error itself. D5's rule holds either way —
the fix in the message is the 80%, and it stands if the runbook is late.

### Pins

`internal/rhq/credentialclass_test.go` (the four classes, their distinctness,
the non-classes, the gate refusal that must not be swallowed, the
read-vs-shape split, both dispatch lines, the cooldown) and `cmd/posse/credentialclass_test.go` (the header, end
to end from a real loopback endpoint through the shipped reader and cache,
plus the segment's placement rules). Every one of them was mutation-checked:
dropping the wrap, dropping the gate-refusal step-around, giving `failShape`
the fix, taking `failShape`'s class away, dropping the cooldown's `Unwrap`,
dropping an arm of `PlanFailureOf`, dropping the scan's classification line,
and dropping the segment's class clause each kill a named test.

One negative is deliberately a PHRASE and not a word: the unreadable rows ban
"credential stale" and "once to refresh" rather than "refresh", because an
envelope diagnosis legitimately names a key called `refreshToken` and a bare
ban fails on the store's own vocabulary. The 403 row keeps the bare word,
which is what the amended D2 forbids by name, because that message has no
store vocabulary in it.
