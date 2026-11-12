# A credential token is a type that cannot be printed (ranger-base-3spp4)

2026-11-12. Defense in depth behind the CLI-home fence (ranger-base-u3kft,
-6sddu, -vxir7): the seam used to hand a token out as a plain Go string, so
the only thing keeping it out of a test log, a transcript or an error was
that nobody had yet typed `t.Fatalf("%v", tok)`. A fence gap now costs a
length, not a token.

## What changed

- `internal/posse/credtoken.go` — `CredToken`. Every printer renders
  `[credential, N bytes]`: fmt with any verb and flag (Format outranks
  String, so `%x`/`%q`/`%d` too), String, GoString, MarshalJSON,
  MarshalText (so both slog handlers and YAML encoders). `Empty`, `Len`,
  `Fingerprint` (hex sha256 — byte-for-byte what `CredFingerprint` always
  wrote, so plancache cooldown records on disk still match). `Reveal` is
  the one door to the bytes.
- The seam returns it: `ReadCredential`, `ReadSessionCredentialFrom`,
  `MeterToken`, `readStore`, `credentialToken`, and the `Token`/`Fallback`
  fields of `AnthropicPlanReader` and `ModelLister`.
- `Reveal` is called in exactly two places, both an `Authorization: Bearer`
  header after `pinnedRequest`: `AnthropicPlanReader.Read`
  (planusage_anthropic.go) and `ModelLister.getPage` (modelavail.go).
  `CredFingerprint` uses `Fingerprint`, which does not reveal.
- The bead's fix shape also named "the launch path that types a session
  credential onto a seat". There is no such Reveal: a launch exports its env
  sets through `EnvSetVars` and never goes through the seam, so no seam
  token reaches a seat. The seam's session read serves the catalog probe and
  the cockpit rows only.
- Tests compare with `sameCredToken` (credtokenhelpers_test.go, untagged so
  every arm builds it), which compares fingerprints. credcomposite_test.go's
  hand-rolled `redact` helper is gone: the type is the redaction.

## Two fmt facts the type is built around (MEASURED, go1.26)

1. fmt cannot call Format on a value it reaches through an UNEXPORTED struct
   field; it walks it reflectively. A token holding a string directly would
   print as `{secret}` inside `struct{ tok CredToken }`. So the value sits
   behind a pointer, which that walk prints as an address.
2. fmt DOES dereference a pointer to a struct/array/slice/map met at depth 0,
   and its bad-verb path re-prints the offending value AT depth 0. With the
   pointee a `struct{ s string }`, `%s` and `%q` over a carrier struct printed
   `&{secret}` — TestCredTokenNestedInAnUnexportedFieldPrintsNoByte was red on
   all eight %s/%q rows. A pointer to a STRING-kinded type is never
   dereferenced, so the pointee is `type credSecret string`.

## `==` does not compile on it

Over a pointer, `==` compares identity: two reads of one value compare
unequal. The first build of the type had exactly that defect in two
existing tests (credseam_test.go: "the two entry points are one reader", and
the keychain-vs-file envelope comparison) — one red, one latent. A `_ [0]func()`
field makes the struct non-comparable at zero size, so the wrong comparison
is a build error and Fingerprint is the only comparison left.

## The pin

`internal/treepins/credtokenreveal_qa_test.go`, in `make corpus-check`:

- `TestCredTokenRevealIsCalledOnlyFromTheRegister` — every selector named
  `Reveal` under internal/ and cmd/ (calls and method values) is a row of
  `crRevealAllowed` (file + func + why, exactly one call per row), and no
  `_test.go` has one at all. Mutation-checked: an `_ = tok.Reveal()` planted
  in `CredFingerprint` reds it naming the line; on first run it also caught a
  `zero.Reveal()` in this bead's own unit test.
- `TestCredTokenRevealIsTheOnlyMethodOfThatName` — the by-name census stays
  precise only while no other method is called Reveal.
- `TestCredTokenRevealScannerCatchesEachShape` (undoored control arm) — a
  planted fixture with a test-file call, an unregistered call, a
  package-level method value, a doubled row and a stale row.

Blind spots, stated: reflection (`MethodByName`) and `unsafe`. Neither is
what a careless failure message reaches for.
