# ranger-base-4ch00 — the keychain item hash is NFC, because the decline was a price and the price got paid

ADR 0019 §2 said *"Posse does not add Unicode NFC normalization; non-ASCII
names retain the documented diagnostic limitation."* Two code comments gave
the reason: Go's standard library has no NFC, and the page priced
`golang.org/x/text` and declined it. ranger-base-d88rp then took x/text for
the trust key (`claudeProjectKey`), which left the keychain item as the one
place posse still derived a name the runtime never wrote, with the dependency
that fixes it already linked. Reversed: the item name is hashed over the NFC
form of the directory string, exactly as the runtime does.

## What the runtime does, and what posse did

The runtime's secure-storage resolver (`TS()`, transcribed in
`credential.go` at `credentialDir`) returns
`(n||l(o(),".claude")).normalize("NFC")`, and its config-dir resolver
(`Se()`, transcribed at `ClaudeConfigDirIn`) normalizes the same way. The
item name is sha256 over that RETURNED string. So for `CLAUDE_CONFIG_DIR`
carrying a decomposed codepoint, the runtime writes `-0873cca0` and posse,
hashing the bytes as spelled, asked for `-16eb4464` and printed a note saying
the composed item was the first suspect. That note was the limitation the
page accepted.

## MEASURED

2026-10-03, this box, darwin 25.4.0, posse at `a63c820e`, Go toolchain from
go.mod, `go build ./cmd/posse`:

| | bytes |
|---|---|
| HEAD | 13,695,138 |
| HEAD + `norm.NFC.String(dir)` inside the sha256 | 13,695,138 |

**+0.** The table is already linked by trust.go; this is a second caller of a
package the binary carries. (d88rp's +103,968 was the price of the FIRST
caller, and it is paid.)

Same scratch build, `go test -run 'KeychainItemNameIsDerived|NormalizationNote|KeychainReadAsksForTheDerived' ./internal/posse`:

- the decomposed arm of `TestTheKeychainItemNameIsDerivedFromTheConfigDirEnvironment`
  derives `-0873cca0` — the composed digit, which the arm's own `why` names as
  the one the runtime derives;
- `TestTheStoreNameCarriesTheNormalizationNoteForANonASCIIDirectory` reds on
  the digit (it wants `-16eb4464`) and would pass on the note, because the
  scratch build left the note in — which is why the note has to go in the
  same change: "posse hashed it as spelled" becomes a false sentence the
  moment the hash is normalized;
- nothing else in the three pins moved. The ASCII arms are the identity under
  NFC, as the existing comment already says.

The scratch edit was restored; nothing of it is committed under this bead.

## Alternatives priced

**(a) Keep the decline and restate it as a choice.** Zero code. Rejected:
there is no choice left to state. The decline rested on availability and on
the cost of being wrong being a note rather than a seat; the first ground is
gone and the second was only ever a reason to tolerate the defect, not a
reason to prefer it. A restatement would have to read "posse asks for a name
the runtime never wrote, and says so, because the fix is one call" — and no
reader would accept that as a design.

**(c) Normalize the whole resolver** — `credentialDirNamed` returns NFC, so
the file path, the wall and the item all follow the runtime. Rejected here,
out of scope and unmeasured: `CredentialsFile` and the seatbelt denies are
paths posse OPENS, d88rp's comment at `ClaudeConfigDirIn` already rules on
that half, and whether the composed spelling of a decomposed directory names
the runtime's file or nothing depends on the filesystem (APFS resolves both
spellings to one directory; ext4 does not). That is a Linux measurement
nobody has, on a seam ADR 0019 already lists as unconfirmed. Left ASSUMED in
the page, in words.

**(d) Normalize and keep the note.** Rejected: the note's two halves ("posse
hashed it as spelled", "the runtime hashes its NFC form") stop being
different, so the note would name a suspect that cannot be the culprit. A
diagnostic that points at the wrong first suspect is worse than none.

**(e) Read both items, as-spelled first and NFC second.** Rejected: two reads
where one is exact, and the read would no longer say which item answered —
the ambiguity ranger-base-tghn5 just removed on the account axis.

**(f) A hand-rolled composition for the Latin-1 range.** Rejected before
pricing, for d88rp's reason: a second Unicode table to keep in sync by hand,
to avoid a package already in the binary.

## ASSUMED

- x/text v0.33.0's NFC tables and the runtime's (node/ICU) agree for the
  codepoints an operator can put in a path. True for every assigned
  composition in both; a codepoint added to Unicode after one side's table
  and before the other's could differ. Not measured, and not measurable
  without a path nobody has.
- The file-path half (option c) — see above.

## Exit hatch

x/text is already a dependency under d88rp; this adds a second caller and
holds no state hostage (nothing posse persists is keyed on the item name —
`keychainItem`'s only caller builds the read's argv and the store's sentence).
If x/text ever leaves, both callers lose NFC together and the d88rp pin and
the decomposed arm here both red, so the regression cannot be silent.

## Lesson (ORDERS)

A decline justified by availability outlives the availability. When a
dependency enters the binary for one reason, grep for every site that
declined it for another and re-ask each one — same family as a67nu's flag
that outlived its number.
