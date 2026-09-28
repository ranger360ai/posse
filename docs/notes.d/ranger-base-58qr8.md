# ranger-base-58qr8 — the empty-token line led with the wrong class

The bead asked for a second `credShapes` entry by name: the plan guard had
been blind for 88 hours, the keychain item was there, it held posse's own
envelope, and `claudeAiOauth.accessToken` was an empty string. The runtime had
updated under us and sessions still worked, so the obvious reading was that
posse had not been taught the new shape.

There is no new shape. This is the mechanism; the measurement it rests on is
credential-class content and lives in the instance tree (ADR 0024 D1 content
arm, D3 restate-and-cite): **ranger-base `docs/spikes/ranger-base-58qr8-…`**,
which carries the versions compared, the extracted expressions and the argv.

## How it was measured, without reading a store

MEASURED 2026-09-28, darwin-arm64. The instrument was the **shipped runtime binary**, not the credential. Those
bundles carry their own JS chunks, so the code that writes the item reads back
as source — and reading the program that writes a store answers "where does
this live" without opening the store at all. No item was opened, no value was
seen, and the keychain CLI was never run; every crew PID denies it, and the
bead records that enumerating items had already been refused to the previous
session as credential exploration.

Two things made it cheap, and both generalize:

- **The installer leaves previous versions in place.** Comparing the last
  version that worked against the three in the blind window made this a
  differential rather than an inference — ten minutes instead of a guess.
- **`grep -o` with a large window dies** on this box ("exceeds complexity
  limits"), and the chunks are split across `strings` lines at every
  non-printable byte. Dump once, window it in python, read forward to the
  chunk's export line for the tail.

## What it found

**The store did not move.** The item name is derived by the same expression in
every version compared, and it is `keychainItem()` rule for rule — default-ness
read off the *environment* and not the path, the hash over the directory string
as spelled, NFC. The envelope is the same one, written by the same mutate.
Nothing was renamed and no second envelope appeared. posse was asking the right
keychain for the right item the whole time.

**What changed is the STATE, not the shape.** When the refresh endpoint refuses
the grant, the runtime rewrites its own envelope with both token fields set to
the empty string and the expiry zeroed, keeping every other key by spread. That
is byte-for-byte the shape the bead reported — both tokens empty, the scopes
and the plan metadata all still sitting there. The item is a **tombstone the
runtime wrote on purpose**, not a write that was interrupted. Live sessions
kept working because a session already holding a token in memory never goes
back to the store.

**So `credShapes` gains nothing.** An emptied field is not a shape, and an
entry for one could never match. That is the third time running that the way to
know was to measure rather than to guess, and the measurement is recorded on
`credShapes` itself so the next reader finds it before inventing an entry.

## The two defects in the sentence

**1. The lead named the class the fork was not.** posse *did* take the empty
fork, and its verdict said so — 200 bytes downstream, past two key lists, where
the quoted report had already been cut off. Nothing was wrong with the tail; it
had been right since ranger-base-6ai5. It sat behind a first clause that said
the opposite, and a line that opens by naming the wrong class is a line whose
reader stops at the opening.

`credentialToken` now takes its lead and its detail from **one walk**
(`shapeFailure`), because a second walk to pick the lead is exactly how a
sentence's head comes to contradict its tail. The forks that really are shapes
keep `holds no token in any shape posse knows` byte for byte; the empty one
opens with `holds a token field that is PRESENT BUT EMPTY — a login state, not
an unknown shape`, and names the operator's move there rather than at the end.

**2. The tail then pointed at a refresh that cannot happen.** It said *"a
refreshToken is present, so a refresh that did not complete fits"* — off a
`!= nil` test, which is presence and not content. On a tombstone that field is
present *and empty*, so the only thing that could complete that refresh is the
field that was cleared. `refreshHint` keeps three states apart now, folding the
way the parser folds (ranger-base-6ai5, and its verifying bead -ogzh, one field
over again):

| the other token | clause |
|---|---|
| absent | none |
| present, non-empty | a refresh that was started and did not finish |
| present, empty or null | both fields hold nothing — nothing can refresh from here; log in |
| present, not a string | *present*, and no claim beyond it |

Pinned in `internal/posse/credtombstone_qa_test.go` (arm 3), with the renamed
and restructured forks pinned beside them so the new lead cannot leak onto a
fork that is a shape.

## The class, stated once

A diagnostic sentence has a **lead** and a **tail**, and they are not read
equally. The tail is where the precision goes and the lead is what survives
quoting, truncation, and a reader in a hurry during an outage. When a line
forks, the fork belongs in the first clause — the tail can elaborate it, but it
cannot correct it, because by then nobody is reading.

## Filed, not fixed

One real divergence turned up beside the answer and is **ranger-base-tghn5**:
the runtime scopes its keychain read and write by account, and posse's read
does not, so the lookup can match an item other than the one the runtime
writes. It predates the blind window, so it is not this outage — and it was not
fixed here because it cannot be verified from a crew seat, and getting it wrong
would replace one honest diagnosis with a confident, wrong one on the only line
an operator reads while the meter is blind.
