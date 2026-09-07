package posse

// The darwin store is a COMPOSITE (ADR 0019 D2 as amended 2026-09-01,
// ranger-base-v3qi4; built by ranger-base-5jdzh). Claude Code's own darwin
// secure storage is one store whose name is
// `keychain-with-plaintext-fallback` — the keychain item, then the
// credentials file when the item did not answer — and posse mirrors that
// read order with ONE narrowing: the file only on exit 44 (errSecItemNotFound),
// never on 36 and never on any other failure, because the keychain ACL is per
// binary and posse's 36 speaks about posse's binary rather than about the
// keychain's contents.
//
// What these pin is ADR 0019 V8 and V9. The shape of every row is the same:
// a `security` stub that answers a chosen way, and a fallback file PLANTED
// with a valid envelope carrying a token of its own. That planted file is
// the witness, and it is the whole reason these rows measure anything — a
// fixture the adapter ignores is a pass that measured nothing, so "the file
// is never opened" is asserted by what a read of it WOULD have produced (a
// different token, a different Source, a success where an error is owed) and
// never by the absence of an error.
//
// Every row runs on either box: the stub answers on linux, and `make
// test-linux` proves the darwin branch exactly as a macOS run does (no build
// tags — ADR 0019 D2).

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// namesTheFallThrough and namesARefresh are the must-NOT halves of the arms
// below, and they are regexps rather than the whole words they replaced
// ("fallback", "refresh") for the reason ranger-base-g6k5b names on a
// different vocabulary: a banned WORD is walked out from under by a one-word
// respelling of the same meaning, so the pin comes back green on exactly the
// sentence it exists to catch. "the token came from the file it falls back
// to" contains no "fallback"; "Refreshing it will not help" contains no
// lowercase "refresh".
//
// The family, not the stem. A bare "fall" is not enough here — "fell back"
// and "fell through" contain no "fall" — and it is not needed either:
// MEASURED on this tree, none of the four strings these guard (the keychain
// item's Source, the keychain 401, the sourceless 401, the 403) contains
// "fall" in any case, so the width these buy costs no false positive. The
// present-clause arms are untouched: they assert what the fallback 401 must
// SAY, and a wider must-NOT next to them cannot make them read differently.
//
// namesTheACLMove is the must-NOT half of the fallback 401's OTHER cause, and
// it is here for the same reason and by the same finding's class: it stood as
// a case-sensitive `strings.Contains(other.Error(), "make install")` — one
// whole form, guarding vocabulary — until ranger-base-wenqb measured it.
// MEASURED there, on a go test -overlay of planusage.go that gave the
// non-fallback 401s the ACL clause respelled as "dropped when posse was
// reinstalled": the arm was ok 0.435s, the whole package under the same
// mutant was ok, and every Auth|401|Cred|Usage|Plan|Stale|Composite|Keychain
// test was ok. Nothing else catches it, because the arm's other half cannot:
// namesTheFallThrough matches the shipped clause only through the
// interpolated store name (credentialsFileFallback carries the word
// "fallback"), and a clause about the WRONG store need not name it.
//
// The family is the install vocabulary the move is spelled in — "make
// install", "reinstalled", "installed", "installing" — all of which `install`
// carries, with `(?i)` for the case the old form also lacked. MEASURED as the
// unmutated control below: none of the strings this guards names an install
// in any case, so the width costs no false positive here.
//
// namesTheSilentKeychain and namesTheRepairMove are the fallback clause's
// other two pieces, and they are here because ranger-base-65tzm measured that
// nothing guarded them. The clause has FOUR pieces and the arm below banned
// TWO of them, so two thirds of it could be pasted verbatim onto a keychain
// 401 and a plain 401 with the arm still green — an operator sent to repair a
// keychain and `/login` over a 401 that came from the keychain's own token,
// which is the exact misdirection ADR 0019 D2 V9 added the clause to prevent.
// MEASURED there, on a go test -overlay of planusage.go giving the non-
// fallback 401s the clause minus its install cause: ok 0.487s, and every
// Auth|401|Cred|Usage|Plan|Stale|Composite|Keychain test in the package under
// the same mutant ok 64.349s. Cause (a) alone and the remedy alone were both
// green on their own too.
//
// MEASURED AGAIN at this fix, the same overlay, one clause piece at a time on
// the non-fallback 401s, reading WHICH ban the failure names:
//
//	unmutated control                                  -> ok
//	that repro (the clause minus its install cause)    -> RED: cause (a), the remedy
//	cause (a) alone                                    -> RED: cause (a), alone
//	the remedy alone                                   -> RED: cause (a), the remedy
//	the remedy off "keychain" ("Unlock it and grant
//	  access, then `/login` in claude")                -> RED: the remedy, alone
//	cause (b) off "keychain" ("this binary's ACL was
//	  dropped by `make install`")                      -> RED: the ACL move, alone
//	the store name alone                               -> RED: the fall-through, alone
//	the whole clause verbatim                          -> RED: all four
//
// so every one of the four bans has a respelling only it catches, and none of
// them is decorative. WRONG ARM, so this is not a misaimed probe: that same
// repro run against the PRE-FIX version of this file — `git show HEAD:` of it,
// overlaid alongside — is GREEN. The ban pair added here is what changed it.
//
// The standing synonym edge is re-measured and still open, as it was: cause
// (a) respelled off this vocabulary entirely ("so the login item did not
// answer: either it is gone and claude is living on that file") is GREEN.
// That is a different word, not a respelling, and no vocabulary ban reaches
// it.
//
// And the reason a finished census did not see it, which is the part worth
// keeping: the count at THE CENSUS below counts `strings.Contains` CALL
// SITES, and a clause piece with no ban at all has no call site to count. A
// census of the bans answers "which of these is too narrow" — never "which
// piece of the shipped sentence has no ban".
//
// namesTheSilentKeychain is cause (a): the keychain item did not answer,
// either it is gone or claude is living on the fallback file. The family is
// the noun the cause is about, in its spellings. namesTheRepairMove is the
// remedy the clause ends with — "Repair the keychain (unlock it, grant
// access), then `/login` in claude" — and it is the operator MOVES rather
// than the noun, so a remedy respelled off "keychain" ("Unlock it and grant
// access, then `/login` in claude") is still caught. The two overlap on the
// shipped bytes and that is not a defect: each has a respelling only it
// catches, and each is mutated below.
//
// MEASURED as the unmutated control, which this arm's own must-NOT loop is:
// neither string these guard carries "keychain", "unlock", "grant access",
// "repair" or "/login" in any case. Printed at ranger-base-65tzm, the
// keychain 401 and the plain 401 are both exactly "usage endpoint returned
// 401 Unauthorized: credential stale — run `claude` once to refresh", so the
// width costs no false positive here.
var (
	namesTheFallThrough    = regexp.MustCompile(`(?i)(fall(s|ing|en)?|fell)[ \-]?(back|through)`)
	namesARefresh          = regexp.MustCompile(`(?i)refresh`)
	namesTheACLMove        = regexp.MustCompile(`(?i)install`)
	namesTheSilentKeychain = regexp.MustCompile(`(?i)key[ \-]?chain`)
	namesTheRepairMove     = regexp.MustCompile(`(?i)(/login|\bunlock(s|ing|ed)?\b|grant(s|ing|ed)? access|\brepair(s|ing|ed)?\b)`)
)

// namesAnotherClassesMove is the never-list of the 44-and-no-file row below:
// the operator moves that belong to a DIFFERENT credential class, in the
// words that would send a reader at the wrong half of the system.
//
// It was the fifth must-NOT ban in this file when ranger-base-dopyl widened
// the other four, and its entries were still VOCABULARY — {"credential
// stale", "once to refresh", "not entitled"} under a case-sensitive
// strings.Contains.
//
// It is widened here for dopyl's own reason, measured on this row rather than
// argued from it (ranger-base-8v29w, the verify of that close):
// keychainFallbackFix carrying " — the credential is stale, so run claude
// once so it refreshes" is BOTH retired moves in one sentence, and none of
// the three whole forms fired. Each entry was one word from green.
//
// The refresh half is namesARefresh itself, not a copy: it is the same family
// guarding the same vocabulary in the 403 arm below, and two spellings of one
// ban drift apart. The other two are stems rather than families because their
// families have no irregular member — "stale" carries "staleness", "entitl"
// carries "entitled" and "entitlement" — which is the distinction
// ranger-base-8v29w's other finding turned on ("fall" does not carry "fell").
// MEASURED, as the unmutated control of that finding: none of the strings
// this guards contains "stale" or "entitl" in any case, so the width costs no
// false positive here.
//
// THE CENSUS, which is what the paragraph above is really for — a reader who
// believes the sweep is finished stops sweeping. What stood here said this
// was the LAST ban whose entries were vocabulary, and that the only
// `strings.Contains` left were three fixture values. It was wrong by one:
// ranger-base-wenqb found a sixth, `strings.Contains(other.Error(), "make
// install")` in the fallback-401 arm below, and measured the respelling that
// walked out from under it. That one is now namesTheACLMove.
//
// Re-counted by hand at that fix, and stated in CALL SITES rather than in
// names, because counting names is what hid the sixth: FOUR `strings.Contains`
// must-NOT calls remain in this file, and every one of them names a fixture
// VALUE rather than vocabulary — fallbackOnlyToken and keychainOnlyToken (two
// calls in one arm) and fakeToken (twice, in two). A value has no respelling;
// naming one is the whole defect. A must-SAY `!strings.Contains` is a
// different thing and is not counted: it pins bytes the shipped sentence has
// to carry, and a respelling there reds, which is correct. The census is
// `grep -n strings.Contains` on this file minus the `!` ones — one command,
// so run it rather than trusting this sentence, which is the mistake it is
// here to record.
//
// AND THE LIMIT OF THIS CENSUS, which ranger-base-65tzm paid for: counting
// call sites answers "which of the bans that exist is too narrow", and never
// "which piece of the shipped sentence has no ban at all". A piece with no
// ban has no call site, so it cannot appear in any count of them, and the
// census stays correct and finished on its own terms while the guard is two
// thirds of a guard. The question this count cannot ask is asked at
// namesTheSilentKeychain instead, against the shipped clause piece by piece.
var namesAnotherClassesMove = []*regexp.Regexp{
	regexp.MustCompile(`(?i)stale`),  // the 401's move, not this row's
	namesARefresh,                    // likewise
	regexp.MustCompile(`(?i)entitl`), // the 403's
}

// The two envelopes, and they are deliberately not each other: a row that
// cannot tell which store answered has pinned nothing.
const (
	keychainOnlyToken = "token-from-the-keychain-item"
	fallbackOnlyToken = "token-from-the-fallback-file"
)

// fallbackDir points this test's credential resolution at a scratch config
// directory and answers where the composite's fallback file would be.
//
// It is not tidiness. Since the darwin adapter became the composite, a
// `security` that exits 44 consults the file the ENVIRONMENT names — so a
// test that left this to the ambient environment would read the operator's
// own credentials file on any box that has one, and pass or fail on what it
// found there. That file is not hypothetical on the reference box: it is
// regenerated by the runtime on its own schedule (MEASURED 8h06m after a
// delete, ranger-base-xjj9).
//
// It uses t.Setenv, so a test that calls it may not call t.Parallel. The
// secure-storage variable rather than $HOME, because that is the narrowest
// input that moves the file: it changes credentialDir and nothing else in
// the process. It also names a directory, so the keychain item the rows read
// carries the derived suffix (ranger-base-ig4op) — which is the live
// spelling wherever an operator has set either variable, and so the harder
// of the two cases to get right.
func fallbackDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CLAUDE_SECURESTORAGE_CONFIG_DIR", dir)
	return filepath.Join(dir, ".credentials.json")
}

// plantFallback is that, with the runtime's envelope written into it.
func plantFallback(t *testing.T, blob string) string {
	t.Helper()
	p := fallbackDir(t)
	if err := os.WriteFile(p, []byte(blob), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// ─── V8: the read order, one row per exit ────────────────────────────────────

func TestDarwinCompositeReadsTheFileOnExitFortyFourAndOnNothingElse(t *testing.T) {
	// The fallback envelope is FRESHER than the keychain's, which is ADR
	// 0019 D2's own wording ("the file is never opened even when present and
	// fresher") and the S2/S4 states of its table: a keychain that answers
	// wins over a newer file, because the keychain is the store the runtime
	// itself reads whenever it holds an item.
	keychainExpiry := time.Now().Add(time.Hour).Truncate(time.Millisecond)
	fallbackExpiry := time.Now().Add(90 * 24 * time.Hour).Truncate(time.Millisecond)
	keychainBlob := envelope(keychainOnlyToken, keychainExpiry.UnixMilli())
	fallbackBlob := envelope(fallbackOnlyToken, fallbackExpiry.UnixMilli())

	for _, tc := range []struct {
		name   string
		script string
		// gated builds the stub from posse's own L1 shim instead, which is
		// the one failure that is not a failure of the store at all.
		gated bool
		check func(t *testing.T, tok string, meta CredMeta, err error)
	}{{
		name:   "exit 0: the item answers and the file is never opened, fresher or not",
		script: "#!/bin/sh\ncat <<'JSON'\n" + keychainBlob + "\nJSON\n",
		check: func(t *testing.T, tok string, meta CredMeta, err error) {
			if err != nil {
				t.Fatalf("a keychain that answered is a credential: %v", err)
			}
			if tok != keychainOnlyToken {
				t.Errorf("token = %q, want the keychain item's — the file was read", redact(tok))
			}
			if !strings.Contains(meta.Source, "keychain item") || namesTheFallThrough.MatchString(meta.Source) {
				t.Errorf("Source = %q, want the keychain item's own name", meta.Source)
			}
			// The witness with teeth: the planted file's expiry is three
			// months out and the item's is an hour. An adapter that opened
			// the file and preferred it reports the wrong date here even if
			// it somehow returned the right token.
			if !meta.ExpiresAt.Equal(keychainExpiry.UTC()) {
				t.Errorf("ExpiresAt = %s, want the item's %s — the fresher file was preferred",
					meta.ExpiresAt, keychainExpiry.UTC())
			}
		},
	}, {
		name:   "exit 44: the item is not there, so the file is the store",
		script: "#!/bin/sh\nexit 44\n",
		check: func(t *testing.T, tok string, meta CredMeta, err error) {
			if err != nil {
				t.Fatalf("exit 44 with a fallback file beside it is a credential (S3): %v", err)
			}
			if tok != fallbackOnlyToken {
				t.Errorf("token = %q, want the fallback file's", redact(tok))
			}
			// The Source is the whole point of the fall-through: a 401 on
			// this token has to name where it came from (V9).
			if meta.Source != credentialsFileFallback {
				t.Errorf("Source = %q, want %q", meta.Source, credentialsFileFallback)
			}
			if !meta.ExpiresAt.Equal(fallbackExpiry.UTC()) {
				t.Errorf("ExpiresAt = %s, want the file's %s", meta.ExpiresAt, fallbackExpiry.UTC())
			}
		},
	}, {
		name: "exit 36: user interaction not allowed is about THIS binary, not about the keychain",
		// D2's narrowing, and the row that would re-create the 2026-08-24
		// misdiagnosis if the composite were mirrored literally: after a
		// `make install` drops the ACL, a 36 that fell through would serve a
		// frozen S2 file and call it healthy.
		script: "#!/bin/sh\nexit 36\n",
		check:  unreadableWithTheACLFix,
	}, {
		name:   "exit 1: any other failure is the failure it arrived as",
		script: "#!/bin/sh\nexit 1\n",
		check:  unreadableWithTheACLFix,
	}, {
		name:  "a gate refusal never reaches the file: the item was not reached at all",
		gated: true,
		check: func(t *testing.T, tok string, meta CredMeta, err error) {
			var g *GateRefusal
			if !errors.As(err, &g) {
				t.Fatalf("want *GateRefusal, got %T: %v (token %q)", err, err, redact(tok))
			}
			if cu := CredUnreadableReason(err); cu != nil {
				t.Errorf("posse's own gate wearing the store's class: %+v", cu)
			}
			if tok != "" {
				t.Errorf("a refused read yields no token, got %q", redact(tok))
			}
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			plantFallback(t, fallbackBlob)
			bin := ""
			if tc.gated {
				bin = gatedSecurityShim(t)
			} else {
				bin = keychainStub(t, tc.script)
			}
			tok, meta, err := readStore(keychainStoreAt(bin))
			tc.check(t, tok, meta, err)
		})
	}
}

// unreadableWithTheACLFix is today's behaviour, exactly: the class, the
// sentence and the one-line move an operator gets for every keychain failure
// that is not "no such item".
//
// The `never` half is the fall-through assertion. The fallback file is
// planted and holds a valid envelope, so an adapter that consulted it would
// have SUCCEEDED here — the error itself is the witness — and one that
// consulted it and failed would name it in the sentence.
func unreadableWithTheACLFix(t *testing.T, tok string, meta CredMeta, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("the file was read for a failure that does not fall through: token %q from %q",
			redact(tok), meta.Source)
	}
	cu := CredUnreadableReason(err)
	if cu == nil {
		t.Fatalf("want *CredUnreadable, got %T: %v", err, err)
	}
	if cu.Fix != keychainACLFix {
		t.Errorf("Fix = %q, want today's ACL move %q", cu.Fix, keychainACLFix)
	}
	if !strings.Contains(cu.Store, "keychain item") {
		t.Errorf("Store = %q, want the keychain item's own name", cu.Store)
	}
	if namesTheFallThrough.MatchString(err.Error()) {
		t.Errorf("a failure that never reached the file must not name it, in any spelling: %q", err)
	}
	if tok != "" {
		t.Errorf("a failed read yields no token, got %q", redact(tok))
	}
}

// redact keeps this file's own failure messages under the rule credential.go
// keeps: a token is never printed, and a fixture that turns out to hold a
// real one must not be quoted by the test that caught it.
func redact(tok string) string {
	switch tok {
	case "":
		return ""
	case keychainOnlyToken, fallbackOnlyToken, fakeToken:
		return tok
	}
	return fmt.Sprintf("<%d bytes, not one of this test's fixtures>", len(tok))
}

// ─── V9, first sentence: exit 44 and no file ─────────────────────────────────

// D2's "Not changed" row: 44 with no file stays CredUnreadable — blind, with
// ADR 0018's clock — and never *NoSource. The launcher box is a logged-in box
// by construction, so a vanished item there is an incident and not an
// unconfigured platform, and reporting it as structural absence would switch
// the guard off on the one box that has one.
//
// What it GAINS is the second cause. `security` exiting 44 is two different
// facts wearing one exit code, repaired at opposite ends, and an operator
// told only one of them repairs the wrong end.
func TestDarwinCompositeExitFortyFourWithNoFileIsBlindAndNamesBothCauses(t *testing.T) {
	fallbackDir(t) // named, and nothing planted in it
	_, _, err := readStore(keychainStoreAt(keychainStub(t, "#!/bin/sh\nexit 44\n")))
	if err == nil {
		t.Fatal("an empty keychain with no fallback file is not a credential")
	}
	if ns := NoSourceReason(err); ns != nil {
		t.Fatalf("a vanished item on a logged-in box is blindness, not an unconfigured platform: %+v", ns)
	}
	cu := CredUnreadableReason(err)
	if cu == nil {
		t.Fatalf("want *CredUnreadable, got %T: %v", err, err)
	}
	if PlanFailureOf(err) != PlanFailUnreadable {
		t.Errorf("class = %q, want %q", PlanFailureOf(err), PlanFailUnreadable)
	}
	// The derived item name is in the sentence, so an operator with a
	// suffixed item knows which one to look for in Keychain Access (V12).
	item, _ := keychainItem()
	if !strings.Contains(cu.Store, item) {
		t.Errorf("the sentence must name the item posse actually asked for: Store %q, item %q", cu.Store, item)
	}
	// Both causes, each with its own move.
	for _, want := range []string{
		"it really is gone and claude is running on its fallback credentials file",
		"repair the keychain", "`/login` in claude",
		"keychain ACL may have been dropped by `make install`",
		"grant access when prompted, or run `claude` once",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the 44-and-no-file sentence must carry %q:\n%q", want, err)
		}
	}
	// The classes this is NOT, in the words that would send an operator at
	// the wrong half of the system (the 2026-08-24 reading) — in any spelling
	// of each, which is the half ranger-base-8v29w widened.
	for _, never := range namesAnotherClassesMove {
		if m := never.FindString(err.Error()); m != "" {
			t.Errorf("the sentence must not carry another class's move %q: %q", m, err)
		}
	}
	// And no value, ever.
	if strings.Contains(err.Error(), fallbackOnlyToken) || strings.Contains(err.Error(), keychainOnlyToken) {
		t.Errorf("a credential must never appear in a diagnostic: %q", err)
	}
}

// ─── V7 through the fall-through: one parser, two paths, one diagnosis ───────

// The fallback is read through credentialToken like every other store, so an
// envelope posse cannot read produces the okbr shape diagnosis under the
// fallback's own name — and no fix, because a renamed key is not a dropped
// ACL and re-granting one would fix nothing (failShape's rule, ADR 0019 V7).
func TestDarwinCompositeFallbackSharesTheEnvelopeParser(t *testing.T) {
	plantFallback(t, `{"claudeAiOauth":{"refreshToken":"r","expiresAt":1}}`)
	_, _, err := readStore(keychainStoreAt(keychainStub(t, "#!/bin/sh\nexit 44\n")))
	if err == nil {
		t.Fatal("an envelope with no token is not a credential, wherever it was read")
	}
	cu := CredUnreadableReason(err)
	if cu == nil {
		t.Fatalf("want *CredUnreadable, got %T: %v", err, err)
	}
	if cu.Store != credentialsFileFallback {
		t.Errorf("Store = %q, want the store that actually answered (%q)", cu.Store, credentialsFileFallback)
	}
	if cu.Fix != "" {
		t.Errorf("a shape failure carries no store fix (V7): %q", cu.Fix)
	}
	for _, want := range []string{"holds no token in any shape posse knows", "teach credShapes the new name"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the shape diagnosis is one piece of code for every store; want %q in %q", want, err)
		}
	}
}

// A fallback file that is there and will not open is a READ failure — blind,
// and not structural absence — and it says so under the fallback's name.
func TestDarwinCompositeFallbackThatWillNotOpenIsBlind(t *testing.T) {
	p := fallbackDir(t)
	// A directory at the file's path: os.Stat finds it, os.ReadFile cannot
	// read it. It is the cheapest "present and unreadable" that does not
	// depend on this process's uid (a mode-0 file does not stop root).
	if err := os.MkdirAll(p, 0o700); err != nil {
		t.Fatal(err)
	}
	_, _, err := readStore(keychainStoreAt(keychainStub(t, "#!/bin/sh\nexit 44\n")))
	if ns := NoSourceReason(err); ns != nil {
		t.Fatalf("a file that is there and unreadable is blindness, not absence: %+v", ns)
	}
	cu := CredUnreadableReason(err)
	if cu == nil {
		t.Fatalf("want *CredUnreadable, got %T: %v", err, err)
	}
	if cu.Store != credentialsFileFallback {
		t.Errorf("Store = %q, want %q", cu.Store, credentialsFileFallback)
	}
}

// ─── V9, second sentence: a 401 on a token that fell through ─────────────────

// The class, Stale() and the governance key are unchanged — policy reads no
// diagnosis string (ADR 0018 §2) — and the SENTENCE is not: a 401 on a
// fallback token names the store it came from and the same two causes, so an
// operator does not `/login` at a keychain that was never the problem.
func TestAuthFailureOnAFallbackTokenNamesTheStoreAndBothCauses(t *testing.T) {
	t.Parallel()
	fallback := &AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized, Source: credentialsFileFallback}
	keychain := &AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized, Source: `keychain item "Claude Code-credentials"`}
	plain := &AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized}

	for _, want := range []string{
		"401", "credential stale", "run `claude` once to refresh",
		credentialsFileFallback,
		"either it is gone and claude is living on that file",
		"keychain ACL was dropped by `make install`",
		"Repair the keychain", "`/login` in claude",
	} {
		if !strings.Contains(fallback.Error(), want) {
			t.Errorf("the fallback 401 must say %q:\n%q", want, fallback)
		}
	}
	// The clause is true of exactly one store, so it appears for exactly one
	// store: without this arm the sentence could be unconditional and every
	// assertion above would still pass.
	//
	// "The clause" is every PIECE of it, one ban each, and the ban is NAMED
	// in the failure — ranger-base-65tzm measured this arm banning two of the
	// four, which left cause (a) and the remedy free to appear on the wrong
	// store. A ban per piece is also what keeps the piece list honest: a
	// fifth piece added to the shipped sentence arrives here with nothing to
	// pair it with.
	for _, other := range []*AuthFailure{keychain, plain} {
		for _, ban := range []struct {
			piece string
			re    *regexp.Regexp
		}{
			{"the store it fell through from", namesTheFallThrough},
			{"cause (a), the keychain item that did not answer", namesTheSilentKeychain},
			{"cause (b), the ACL move", namesTheACLMove},
			{"the remedy", namesTheRepairMove},
		} {
			if ban.re.MatchString(other.Error()) {
				t.Errorf("a 401 on a token that did NOT fall through must carry no piece of the "+
					"fallback clause, and this one names %s (%s): %q", ban.piece, ban.re, other)
			}
		}
	}
	// The class is the same class, and that is the point: the sentence is
	// the diagnostic and nothing here forks policy.
	for _, af := range []*AuthFailure{fallback, keychain, plain} {
		if PlanFailureOf(af) != PlanFailStale || !af.Stale() {
			t.Errorf("the fallback 401 is still the stale class: %q / %v", PlanFailureOf(af), af.Stale())
		}
	}
	// A 403 is not this: a credential that was never entitled is not
	// entitled from either store, so nothing about a refresh may appear and
	// nothing about the fall-through may either — in any spelling of
	// either, which is the half ranger-base-dopyl widened.
	f403 := &AuthFailure{Status: "403 Forbidden", Code: http.StatusForbidden, Source: credentialsFileFallback}
	if namesARefresh.MatchString(f403.Error()) || namesTheFallThrough.MatchString(f403.Error()) {
		t.Errorf("403 keeps its own sentence whatever store answered: %q", f403)
	}
}

// usageEndpointAnswering is the compiled-in usage endpoint, answered in
// memory. It is the only way to pin a sentence that needs a CREDENTIALED
// read: credpin.go credentials exactly one URL (credentialedURL is equality
// against PlanUsageURL), so a reader pointed at a loopback fake never calls
// Token at all and its 401 could carry no Source to name. Nothing leaves the
// box — the RoundTripper answers and no dial is made.
func usageEndpointAnswering(status int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != PlanUsageURL {
			return nil, fmt.Errorf("fake usage endpoint answers %s and nothing else", PlanUsageURL)
		}
		return &http.Response{
			StatusCode: status,
			Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})}
}

// The same sentence through production's own wiring: the seam's Source is
// what the reader threads into the 401, so this fails if the plumbing is
// dropped anywhere between readStore and AuthFailure.
func TestPlanReadThreadsTheFallbackSourceIntoIts401(t *testing.T) {
	t.Parallel()
	r := &AnthropicPlanReader{
		URL:  PlanUsageURL,
		HTTP: usageEndpointAnswering(http.StatusUnauthorized),
		Token: func() (string, CredMeta, error) {
			return fakeToken, CredMeta{Source: credentialsFileFallback, Shape: "claudeAiOauth.accessToken"}, nil
		},
	}
	_, err := r.Read()
	af := AuthFailureReason(err)
	if af == nil {
		t.Fatalf("want *AuthFailure, got %T: %v", err, err)
	}
	if af.Source != credentialsFileFallback {
		t.Fatalf("Source = %q, want the store the reader presented (%q)", af.Source, credentialsFileFallback)
	}
	if !strings.Contains(err.Error(), credentialsFileFallback) {
		t.Errorf("the 401 must name the store it presented: %q", err)
	}
	if strings.Contains(err.Error(), fakeToken) {
		t.Errorf("a credential must never appear in a diagnostic: %q", err)
	}
}

// Surface: the dispatch skip line an unattended parked pass writes (the
// rangerhq-pwpx surface). It prints the error, so the clause reaches the
// operator who is looking at a stopped fleet — which is the moment the
// difference between "the keychain is stale" and "the keychain did not
// answer at all" decides what they go and repair.
func TestBlindSkipOn401FromTheFallbackNamesTheFallbackStore(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	pr := planReaderOf(r.d)
	pr.URL = PlanUsageURL
	pr.HTTP = usageEndpointAnswering(http.StatusUnauthorized)
	pr.Token = func() (string, CredMeta, error) {
		return fakeToken, CredMeta{Source: credentialsFileFallback}, nil
	}

	r.at(12 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("12m blind parks whatever the class: %d\n%s", n, r.out())
	}
	out := r.out()
	if !strings.Contains(out, "— skipped") {
		t.Fatalf("a parked pass must say why:\n%s", out)
	}
	for _, want := range []string{
		"401", "credential stale", credentialsFileFallback,
		"either it is gone and claude is living on that file",
		"keychain ACL was dropped by `make install`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the park line must carry %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, fakeToken) {
		t.Errorf("the park line must never carry a credential:\n%s", out)
	}
}

// The governance row the coordinator is pulsed with carries it too: one
// sentence, every surface that prints the error (govern.go guardBlindRow).
func TestGuardBlindRowOn401FromTheFallbackNamesTheFallbackStore(t *testing.T) {
	t.Parallel()
	key, detail := guardBlindRow(10*time.Hour,
		&AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized, Source: credentialsFileFallback})
	if key != "guard-credential:401" {
		t.Errorf("key = %q, want the credential row's — the class does not move with the sentence", key)
	}
	if !strings.Contains(detail, credentialsFileFallback) {
		t.Errorf("the pulsed row must name the store: %q", detail)
	}
}

// ─── the no-argument report: which of the composite's two stores (D2/D4,
// ranger-base-6kkrq) ──────────────────────────────────────────────────────

// The report's OWN table is the S1-S4 one 6kkrq's design carries (the ADR's
// dated evidence, since folded into git history at the 2026-09-05
// consolidation): S1 healthy (keychain live, no file), S2 frozen (keychain
// live, a stale file beside it), S3 inverted (keychain absent, the file is
// the live record). S4 (split — an older keychain item beside a newer file)
// is ASSUMED there and unmeasured; this bead does not classify it, and a
// live-but-coexisting file below is asserted to fall back to the plain
// no-state action rather than being mislabeled S2 or S3.
//
// Every row runs meterRow itself — the function the report line actually
// calls — over a stubbed `security` and a planted fallback file, so what is
// pinned is the RENDERED line and not an intermediate fact only a helper
// computes.
func TestTheMeterRowNamesTheCompositesState(t *testing.T) {
	rt := &Runtime{Name: "claude"}
	now := time.Now()

	t.Run("S1: the keychain answers and no fallback file is there", func(t *testing.T) {
		fallbackDir(t) // named, nothing planted
		bin := keychainStub(t, "#!/bin/sh\ncat <<'JSON'\n"+envelope(keychainOnlyToken, now.Add(time.Hour).UnixMilli())+"\nJSON\n")
		row := meterRow(rt, "darwin", now, bin)
		if !strings.Contains(row.Source, "keychain item") {
			t.Errorf("Source = %q, want the keychain item's own name", row.Source)
		}
		want := "nothing posse may do — the runtime's login loop is this credential's only writer; run `claude` once to refresh it"
		if row.Action != want {
			t.Errorf("Action = %q, want the plain S1 action %q", row.Action, want)
		}
		if strings.Contains(row.Action, "S2") || strings.Contains(row.Action, "S3") {
			t.Errorf("S1 names no state at all: %q", row.Action)
		}
		if strings.Contains(row.Source+row.Action, keychainOnlyToken) {
			t.Errorf("a credential must never appear in the report: %+v", row)
		}
	})

	t.Run("S2: the keychain answers and the fallback file predates its envelope's issue", func(t *testing.T) {
		fallbackPath := plantFallback(t, envelope(fallbackOnlyToken, now.Add(-48*time.Hour).UnixMilli()))
		keychainExpiry := now.Add(2 * time.Hour)
		issued := keychainExpiry.Add(-meterAccessTokenLifetime) // now-6h
		frozenAt := issued.Add(-24 * time.Hour)                 // well before the horizon
		if err := os.Chtimes(fallbackPath, frozenAt, frozenAt); err != nil {
			t.Fatal(err)
		}
		bin := keychainStub(t, "#!/bin/sh\ncat <<'JSON'\n"+envelope(keychainOnlyToken, keychainExpiry.UnixMilli())+"\nJSON\n")
		row := meterRow(rt, "darwin", now, bin)
		if !strings.Contains(row.Source, "keychain item") {
			t.Errorf("Source = %q, want the keychain item's own name — the keychain answered directly", row.Source)
		}
		for _, want := range []string{
			"S2:", "a frozen fallback file is present",
			"the sweep will keep finding it until the keychain fails a write while empty",
			"harmless, not the record",
		} {
			if !strings.Contains(row.Action, want) {
				t.Errorf("Action must carry %q:\n%q", want, row.Action)
			}
		}
		if !strings.Contains(row.Action, AbbrevHome(fallbackPath)) {
			t.Errorf("Action must name the file: %q", row.Action)
		}
		if strings.Contains(row.Action, keychainOnlyToken) || strings.Contains(row.Action, fallbackOnlyToken) {
			t.Errorf("a credential must never appear in the report: %q", row.Action)
		}
	})

	t.Run("S3: the keychain item is absent and the fallback file is the live record", func(t *testing.T) {
		fallbackPath := plantFallback(t, envelope(fallbackOnlyToken, now.Add(90*24*time.Hour).UnixMilli()))
		bin := keychainStub(t, "#!/bin/sh\nexit 44\n")
		row := meterRow(rt, "darwin", now, bin)
		if row.Source != credentialsFileFallback {
			t.Errorf("Source = %q, want %q — the file is what answered", row.Source, credentialsFileFallback)
		}
		for _, want := range []string{
			"S3:", "the keychain item is absent and claude is running on the fallback file",
			"unlock or re-grant the keychain", "`/login` in claude",
		} {
			if !strings.Contains(row.Action, want) {
				t.Errorf("Action must carry %q:\n%q", want, row.Action)
			}
		}
		if strings.Contains(row.Action, keychainOnlyToken) || strings.Contains(row.Action, fallbackOnlyToken) {
			t.Errorf("a credential must never appear in the report: %q", row.Action)
		}
		_ = fallbackPath
	})

	t.Run("S4 (split, ASSUMED and out of scope): a live file beside a live keychain is not called S2", func(t *testing.T) {
		fallbackPath := plantFallback(t, envelope(fallbackOnlyToken, now.Add(-48*time.Hour).UnixMilli()))
		keychainExpiry := now.Add(2 * time.Hour)
		// Leave the planted file's mtime at "now" — after the envelope's own
		// issue horizon (now-6h) — so it reads LIVE rather than frozen.
		bin := keychainStub(t, "#!/bin/sh\ncat <<'JSON'\n"+envelope(keychainOnlyToken, keychainExpiry.UnixMilli())+"\nJSON\n")
		row := meterRow(rt, "darwin", now, bin)
		if strings.Contains(row.Action, "S2") || strings.Contains(row.Action, "S3") {
			t.Errorf("an unmeasured split state must not be reported as S2 or S3: %q", row.Action)
		}
		_ = fallbackPath
	})
}

// Off darwin, meterRow never touches the composite logic at all: the store
// is the non-darwin file adapter, chosen before any state is computed, and a
// row from it must never say S1/S2/S3 or name a keychain.
func TestTheMeterRowNamesNoCompositeStateOffDarwin(t *testing.T) {
	rt := &Runtime{Name: "claude"}
	t.Setenv("HOME", t.TempDir())
	row := meterRow(rt, "linux", time.Now(), "")
	for _, never := range []string{"S1", "S2", "S3", "keychain"} {
		if strings.Contains(row.Source+row.Action, never) {
			t.Errorf("a non-darwin row must never say %q: %+v", never, row)
		}
	}
}
