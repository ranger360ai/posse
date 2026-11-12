package posse

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
)

// CredToken is a credential's secret as the seam hands it out
// (ranger-base-3spp4). It is a type that cannot be printed: every way Go's
// standard library renders a value — fmt with any verb and any flag, %v and
// %+v and %#v and %q and %x and %s, String, GoString, encoding/json,
// encoding/text, and therefore log/slog's handlers and any YAML encoder that
// honours TextMarshaler — renders the fixed redaction "[credential, N bytes]"
// and never a byte of the value.
//
// WHY A TYPE AND NOT A RULE. Until this type the token was a plain string,
// and the only thing keeping it out of a test log, a transcript or an error
// sentence was that nobody had yet typed t.Fatalf("%v", tok).
// ranger-base-btroh showed tests DO reach a live store on some seats, so one
// careless failure message would have landed a live token in a transcript
// without any new bug. That is a property held by everybody remembering. A
// type holds it by construction: a fence gap (ranger-base-u3kft, -6sddu,
// -vxir7) now costs a length, not a token.
//
// The value sits behind a POINTER TO A STRING-KINDED TYPE, and both halves
// are load-bearing rather than style. fmt calls a value's Format only when
// it can reach it as an interface, and it cannot for a field it reaches
// through an UNEXPORTED struct field — there it walks the struct
// reflectively. A CredToken holding a string directly would print as
// {secret} inside a caller's `struct{ tok CredToken }` under %+v. Behind a
// pointer, that walk prints an address, 0xc000…, and never the pointee.
//
// And the pointee must not be a struct. fmt DOES dereference a pointer to a
// struct, array, slice or map when it meets one at depth 0, and its
// bad-verb path re-prints the offending value at depth 0: %s or %q over a
// caller's struct reaches the pointer as a bad verb and prints `&{secret}`.
// MEASURED on go1.26 by TestCredTokenNestedInAnUnexportedFieldPrintsNoByte,
// red on every %s and %q row while the pointee was `struct{ s string }`. A
// pointer to a string kind is never dereferenced by fmt at any depth.
//
// The bytes come out through exactly one method, Reveal, and the call sites
// of Reveal are a register — the two Authorization headers
// (planusage_anthropic.go, modelavail.go) and nothing else, and no _test.go
// at all (internal/treepins/credtokenreveal_qa_test.go,
// `make corpus-check`). CredFingerprint's hash does not reveal: it is
// Fingerprint, below, which reads the bytes inside this file. A test that
// needs to compare two tokens compares fingerprints.
//
// The zero CredToken is the empty credential: Empty reports it, Len is 0,
// and it prints "[credential, 0 bytes]".
//
// `==` does not compile on it, on purpose. Over a pointer it would compare
// identity, and two reads of one value would compare unequal — which
// credseam_test.go's "two entry points are one reader" did, the first time
// this type existed. The zero-length func array costs no bytes and makes
// the struct non-comparable, so the wrong comparison is a build error and
// the right one, Fingerprint, is the only one left.
type CredToken struct {
	_ [0]func()
	p *credSecret
}

// credSecret is the pointee: string-KINDED, so fmt never dereferences a
// pointer to it (see CredToken), and named, so the pointer type says what it
// points at and nothing outside this file is tempted to dereference it.
type credSecret string

// newCredToken wraps a value the seam has just read. It is unexported: the
// seam (credential.go) mints tokens, and a test that needs a fake one calls
// it from inside the package. "" is the zero token, so Empty answers for
// both spellings of "nothing was there".
func newCredToken(s string) CredToken {
	if s == "" {
		return CredToken{}
	}
	v := credSecret(s)
	return CredToken{p: &v}
}

// Empty reports whether there is no value — the "nothing to present" a
// caller used to test as tok == "".
func (c CredToken) Empty() bool { return c.p == nil || *c.p == "" }

// Len is the value's length in bytes. It is metadata, and the redaction
// prints it: a 0-byte and a 108-byte token are different diagnoses, and the
// length discloses nothing a fingerprint would not.
func (c CredToken) Len() int {
	if c.p == nil {
		return 0
	}
	return len(*c.p)
}

// Reveal is the one way to the bytes. Every call site is registered by
// internal/treepins/credtokenreveal_qa_test.go; a new one is a row somebody
// has to write, naming why that line must hold the secret itself.
func (c CredToken) Reveal() string {
	if c.p == nil {
		return ""
	}
	return string(*c.p)
}

// Fingerprint is the hex sha256 of the value — the form that answers "is it
// the same credential" and discloses nothing. "" for the empty token, which
// plancache reads as "cannot tell". It is byte-for-byte the hash
// CredFingerprint wrote before this type existed, so a fingerprint already
// on disk in a cooldown record still matches the token that earned it.
func (c CredToken) Fingerprint() string {
	if c.Empty() {
		return ""
	}
	sum := sha256.Sum256([]byte(*c.p))
	return hex.EncodeToString(sum[:])
}

// redacted is the one rendering every printer below shares.
func (c CredToken) redacted() string {
	return "[credential, " + strconv.Itoa(c.Len()) + " bytes]"
}

// String renders the redaction (fmt.Stringer).
func (c CredToken) String() string { return c.redacted() }

// GoString renders the redaction (fmt.GoStringer, %#v).
func (c CredToken) GoString() string { return c.redacted() }

// Format renders the redaction for every verb and every flag (fmt.Formatter).
// It outranks String and GoString in fmt's dispatch, so %x, %X, %q and %d —
// verbs String alone would not intercept on a string-kinded value — print
// the redaction too.
func (c CredToken) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, c.redacted())
}

// MarshalJSON renders the redaction as a JSON string (json.Marshaler), so a
// struct that carries a token in an exported field marshals metadata.
func (c CredToken) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(c.redacted())), nil
}

// MarshalText renders the redaction (encoding.TextMarshaler) — the hook
// map-key JSON, slog's text handler and YAML encoders reach for.
func (c CredToken) MarshalText() ([]byte, error) {
	return []byte(c.redacted()), nil
}
