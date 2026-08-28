## Expiry is a first-class answer (ranger-base-k6ha)

ADR 0019 D5. The seam already knew when a credential dies —
`CredMeta.ExpiresAt`, from the envelope's own `expiresAt` for the meter and
from the `# expires=` stamp for a session mint. What was missing is that the
knowledge only arrived when somebody asked. `internal/rhq/credexpiry.go` is
the half that speaks unasked.

**Three surfaces, one reader.** `posse refresh` answers on demand (shipped
with ranger-base-h207); the cockpit header carries a column once a
credential is inside a fortnight; a dispatch pass prints one stderr line in
the same window. All three render through `renderExpiry`/`expiryIn`, so the
warning and the report an operator checks it against cannot print two
different dates for one stamp.

**The seam's session half no longer answers zero.** ranger-base-h207 wrote
the stamp and the seam still reported nothing, because `readSessionCredential`
was a package function with no home to find the env sets in. `ReadCredential`
now hangs off `*App` and the session read carries the stamp of the env set
the value actually came out of — ADR 0019 V5's round-trip.

The match is on the **value**, not on the variable name. A launched session
holds a value and no memory of which set it came from; several sets may carry
the same variable, and each stamp is about its own value. Lending one set's
date to another set's mint would be a wrong date reported confidently, which
is strictly worse than "cannot tell" — an answer this design is already
allowed to give. A nil `*App` gets the same answer rather than a panic.

**What warns and what does not.**

| state | answer |
|---|---|
| stamped, inside 14 days | one header column, one line per pass, an action in the report |
| stamped, already past | the same surfaces, worded and coloured distinctly — never "in −3d" |
| stamped beyond the window | silence; the report still prints the date |
| unstamped, or a stamp posse cannot parse | "cannot tell", and it warns **nothing** |

Silence from the warning surfaces is therefore ambiguous by construction —
nothing is expiring, or nothing is dated — and that ambiguity is resolved in
exactly one place, `posse refresh`, which says which of the two it is for
every credential on the box. That is why the timer surfaces never print a
reassurance: the state they would be reassuring about is one they cannot
distinguish.

**Nothing gates.** No park, no degrade, no blind clock, no threshold. The
read's success or failure stays the only actuator (D5), and a dead session
mint already stops a launch loudly at the moment it matters. The whole job of
these two surfaces is that the operator meets that failure with a fortnight's
notice instead of at 3am.

### DIVERGED from D5's letter: the timer surfaces carry session mints only

The `posse refresh` report answers for both purposes. The header column and
the per-pass line carry only the posse-owned **session** mints. The meter
credential is deliberately absent from both, for two reasons that come out of
ADR 0019's own text:

1. **It would never be quiet.** The meter credential is the runtime's
   rotating OAuth access token, and the ADR's own measurement is that its
   store of record rotates within days (the darwin rejection: "provably stale
   within days"). A rotation horizon shorter than the window means every
   reading is inside the window, so the warning fires every pass forever — and
   a warning that is always on is a warning nobody reads.
2. **There is no hand to warn.** D4 is explicit that posse writes nothing
   here and that the runtime's login loop is the credential's only writer.
   Warning about the next scheduled rotation of a self-refreshing credential
   asks for an action that does not exist. When that loop is actually dead the
   failure is a failed *read* — blindness, ADR 0018, already loud and already
   clocked — which is the actuator D5 says it should be.

The narrowing is also what keeps these surfaces free: a session mint's expiry
is a stamp in a file posse already owns, so a pass reads a few hundred bytes.
Warning about the meter would mean execing `security` on every dispatch pass
and every cockpit tick, against a store the whole instance deliberately reads
once per TTL through one shared cache (rangerhq-tdy8).

The ADR amendment is richard's: filed as its own `-l architecture` bead.

### Verified

Pins in `internal/rhq/credexpiry_test.go` and three cases in
`cmd/posse/cockpit_test.go`, covering ADR 0019 V5 and V6. Eighteen mutations
of the mechanisms, each expected to turn one pin red; **one escaped and is
fixed**: appending the header column unconditionally instead of only when
there is a warning left every test green, because an empty column costs the
flex column a separator cell and the padded output is byte-identical at any
width where the flex is not truncated. The rule is a column *count*, so that
is what is pinned now (`headerCols`), and `applyPlan` was split out of the
event loop for the same reason — the step that joins a scan to the header is
the one a refactor drops silently.

Full suite green, all three packages, `-count=1`, with `$RHQ_GATES_DIR/bin`
stripped from `PATH`.
