package posse

// The keychain item is present, readable, and valid JSON of a shape posse
// does not know (ranger-base-okbr) — the fourth credential-failure class,
// after "unreadable", "refused by our own gate", and "not JSON".
//
// The bug these pin is the DIAGNOSTIC. "has no claudeAiOauth.accessToken" is
// true and useless: it does not say what the item DOES hold, so an operator
// cannot act on it, and on 2026-08-26 that cost an hour of stopped shop. The
// error must name the key NAMES it actually found — and never a value, which
// is the whole reason the old line said nothing.

import (
	"encoding/json"
	"strings"
	"testing"
)

const fixtureSecret = "sk-ant-oat01-FIXTURE-SECRET-VALUE"

// The shape that works still works, and says which shape answered.
func TestCredentialTokenReadsTheOAuthEnvelope(t *testing.T) {
	t.Parallel()
	blob := `{"claudeAiOauth":{"accessToken":"` + fixtureSecret + `","refreshToken":"r","expiresAt":123}}` + "\n"
	tok, meta, err := credentialToken(keychainStore().Name, []byte(blob))
	if err != nil {
		t.Fatal(err)
	}
	if tok != fixtureSecret {
		t.Errorf("token: %q", tok)
	}
	if meta.Shape != "claudeAiOauth.accessToken" {
		t.Errorf("shape: %q", meta.Shape)
	}
	if meta.Source != keychainStore().Name {
		t.Errorf("source: %q", meta.Source)
	}
	// expiresAt 123 is not a date in any unit, so the honest answer is
	// "cannot tell" rather than 1970 (ADR 0019 D5).
	if !meta.ExpiresAt.IsZero() {
		t.Errorf("an unreadable expiry must stay zero: %v", meta.ExpiresAt)
	}
}

// The bead's own case: an item whose envelope is there but whose token is
// not. The error must distinguish that from "no envelope at all", because
// the two have different fixes.
func TestWrongShapeNamesTheKeysItFound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		blob  string
		wants []string
		not   []string
	}{
		{
			name: "envelope present, token absent",
			blob: `{"claudeAiOauth":{"refreshToken":"r","expiresAt":123,"scopes":["user:inference"]}}`,
			wants: []string{"[claudeAiOauth]", "claudeAiOauth's keys are [expiresAt refreshToken scopes]",
				"renamed or dropped", "teach credShapes"},
			not: []string{"incomplete credential"},
		},
		{
			name:  "envelope absent — a different credential structure",
			blob:  `{"apiKey":"` + fixtureSecret + `","createdAt":1}`,
			wants: []string{"top-level keys are [apiKey createdAt]", "is not among them"},
			not:   []string{"claudeAiOauth's keys"},
		},
		{
			name:  "envelope present but empty",
			blob:  `{"claudeAiOauth":{}}`,
			wants: []string{"claudeAiOauth's keys are [] (an empty object)", "renamed or dropped"},
		},
		{
			name:  "envelope is not an object",
			blob:  `{"claudeAiOauth":"` + fixtureSecret + `"}`,
			wants: []string{"claudeAiOauth is a JSON string, not an object"},
		},
		{
			name:  "envelope is null",
			blob:  `{"claudeAiOauth":null}`,
			wants: []string{"claudeAiOauth is JSON null, not an object"},
			not:   []string{"an empty object"},
		},
		{
			name: "token present but empty — an incomplete credential, not our parse",
			blob: `{"claudeAiOauth":{"accessToken":"","refreshToken":"r"}}`,
			wants: []string{"claudeAiOauth's keys are [accessToken refreshToken]",
				"incomplete credential", "re-authenticate rather than change posse",
				"a refreshToken is present"},
			not: []string{"renamed or dropped"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tok, meta, err := credentialToken(keychainStore().Name, []byte(tc.blob))
			if err == nil {
				t.Fatalf("want a shape failure, got token %q in shape %q", tok, meta.Shape)
			}
			msg := err.Error()
			// The item the ADAPTER names, not the constant: under a set
			// config-dir variable the store's name carries a hash suffix and
			// this sentence must carry the same one (ADR 0019 D2 store 1,
			// ranger-base-mx4q6). Asserting the constant here would pass on a
			// box that never sets one and red on the box that does.
			item, _ := keychainItem()
			if !strings.Contains(msg, item) || !strings.Contains(msg, "tried claudeAiOauth.accessToken") {
				t.Errorf("the error must name the item and the shapes tried: %q", msg)
			}
			for _, w := range tc.wants {
				if !strings.Contains(msg, w) {
					t.Errorf("want %q in: %q", w, msg)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(msg, n) {
					t.Errorf("must not say %q: %q", n, msg)
				}
			}
			if strings.Contains(msg, fixtureSecret) {
				t.Errorf("a value reached the error line: %q", msg)
			}
		})
	}
}

// The shape the outage ACTUALLY had, measured on the operator's own machine
// rather than guessed. The top level came back first (ranger-base-8i7l):
// ['mcpOAuth', 'claudeAiOauth']. The inner keys came from posse itself once
// the naming fix was promoted — $RHQ_HOME/state/plan-usage.log, every
// dispatch from 2026-08-26T18:25:05Z to 00:42:22Z the next morning:
//
//	its top-level keys are [claudeAiOauth mcpOAuth], and claudeAiOauth's keys
//	are [accessToken expiresAt rateLimitTier refreshToken
//	refreshTokenExpiresAt scopes subscriptionType]
//
// accessToken IS among them, so the fork falls on the "present but empty"
// side: the credential was incomplete, posse's parse was never wrong, and
// credShapes gains no entry. Read a second way, the log says the same thing —
// dispatch went green at 00:48:33Z with no change to posse.
//
// The renamed fork is pinned beside it because the code still has to take it,
// and because rateLimitTier and refreshTokenExpiresAt were both new that day:
// this envelope does change under us, which is the reason the line names keys
// at all.
//
// mcpOAuth is a second envelope posse knows nothing about; it is named at the
// top level and never opened, because its keys are per-server URLs and not
// ours.
func TestObservedOutageShapeNamesBothLevels(t *testing.T) {
	t.Parallel()
	const mcp = `"mcpOAuth":{"https://example.test/sse":{"accessToken":"` + fixtureSecret + `"}},`
	// The measured key set. Values are invented — only the names were read.
	const measured = `"expiresAt":1756224000000,"rateLimitTier":"tier","refreshToken":"r",` +
		`"refreshTokenExpiresAt":1758816000000,"scopes":["user:inference"],"subscriptionType":"max"`
	for _, tc := range []struct {
		name  string
		inner string
		wants []string
		not   []string
	}{
		{
			name:  "the shape the outage had — accessToken there and empty",
			inner: `{"accessToken":"",` + measured + `}`,
			wants: []string{
				"claudeAiOauth's keys are [accessToken expiresAt rateLimitTier refreshToken " +
					"refreshTokenExpiresAt scopes subscriptionType]",
				"incomplete credential", "re-authenticate rather than change posse",
				"a refreshToken is present",
			},
			not: []string{"renamed or dropped", "teach credShapes"},
		},
		{
			name:  "the fork it was not — accessToken gone, and ours to fix",
			inner: `{` + measured + `}`,
			wants: []string{
				"claudeAiOauth's keys are [expiresAt rateLimitTier refreshToken " +
					"refreshTokenExpiresAt scopes subscriptionType]",
				"renamed or dropped", "teach credShapes",
			},
			not: []string{"incomplete credential"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := credentialToken(keychainStore().Name, []byte(`{`+mcp+`"claudeAiOauth":`+tc.inner+`}`))
			if err == nil {
				t.Fatal("want a shape failure")
			}
			msg := err.Error()
			t.Logf("operator-visible line:\n  %s", msg)
			if !strings.Contains(msg, "its top-level keys are [claudeAiOauth mcpOAuth]") {
				t.Errorf("want both envelopes named at the top level: %q", msg)
			}
			// The longest measured name is 21 bytes; every one of them must
			// survive whole, because a key we elide is a key the operator
			// needed (maxKeyName).
			if !strings.Contains(msg, "refreshTokenExpiresAt") || strings.Contains(msg, "not a name") {
				t.Errorf("a measured schema name was elided: %q", msg)
			}
			for _, w := range tc.wants {
				if !strings.Contains(msg, w) {
					t.Errorf("want %q in: %q", w, msg)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(msg, n) {
					t.Errorf("must not say %q: %q", n, msg)
				}
			}
			if strings.Contains(msg, "example.test") {
				t.Errorf("only the envelope the failing shape names is opened: %q", msg)
			}
			if strings.Contains(msg, fixtureSecret) {
				t.Errorf("a value reached the error line: %q", msg)
			}
		})
	}
}

// Key names are schema and safe to print. The one way they are not is an
// object keyed BY a value — so a name that is not name-shaped is reported by
// its size, never its bytes.
func TestKeyNamesNeverCarryAValue(t *testing.T) {
	t.Parallel()
	// This test only means something while the fixture is longer than the
	// bound. maxKeyName has been raised once already (ranger-base-okbr, when a
	// 21-byte schema name showed up); raise it past this and the assertion
	// below would pass by echoing the secret instead of eliding it.
	if len(fixtureSecret) <= maxKeyName {
		t.Fatalf("fixture (%d bytes) no longer exceeds maxKeyName (%d): lengthen it", len(fixtureSecret), maxKeyName)
	}
	blob := `{"` + fixtureSecret + `":1,"ok":2}`
	_, _, err := credentialToken(keychainStore().Name, []byte(blob))
	if err == nil {
		t.Fatal("want a shape failure")
	}
	msg := err.Error()
	if strings.Contains(msg, fixtureSecret) || strings.Contains(msg, "sk-ant") {
		t.Fatalf("a long key was echoed back: %q", msg)
	}
	if !strings.Contains(msg, "bytes, not a name") || !strings.Contains(msg, "ok") {
		t.Errorf("want the odd key described and the ordinary one named: %q", msg)
	}
}

// A hundred keys is not a credential; the count says so better than the list.
func TestKeyListIsBounded(t *testing.T) {
	t.Parallel()
	obj := map[string]json.RawMessage{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o"} {
		obj[k] = json.RawMessage(`1`)
	}
	got := safeKeys(obj)
	if !strings.Contains(got, "(+3 more)") || strings.Contains(got, " o]") {
		t.Errorf("want 12 shown and the rest counted: %q", got)
	}
}

// Valid JSON that is not an object is its own diagnosis, and it names the
// kind — not the content.
func TestNonObjectJSONNamesItsKind(t *testing.T) {
	t.Parallel()
	for blob, want := range map[string]string{
		`["` + fixtureSecret + `"]`: "a JSON array",
		`"` + fixtureSecret + `"`:   "a JSON string",
		`null`:                      "JSON null",
		`<html>nope`:                "not JSON",
	} {
		_, _, err := credentialToken(keychainStore().Name, []byte(blob))
		if err == nil {
			t.Fatalf("%q: want a failure", blob)
		}
		msg := err.Error()
		if !strings.Contains(msg, "not the expected JSON") || !strings.Contains(msg, want) {
			t.Errorf("%q: want %q in: %q", blob, want, msg)
		}
		if strings.Contains(msg, fixtureSecret) {
			t.Errorf("%q: a value reached the error line: %q", blob, msg)
		}
	}
}

// End to end, through the real exec path and out to the plan guard's blind
// line: the operator reads what the item HOLDS, not what it lacks. A stub
// `security` named to the adapter stands in for the keychain — the real one
// is never touched, and the wrong shape is the whole point of the fixture.
func TestPlanGuardBlindLineNamesTheShapeItFound(t *testing.T) {
	bin := keychainStub(t, "#!/bin/sh\ncat <<'JSON'\n{\"claudeAiOauth\":{\"refreshToken\":\"r\",\"expiresAt\":1},\"userID\":\"u\"}\nJSON\n")

	r := newBlindRig(t, guardOn)
	keychainOnly(planReaderOf(r.d), keychainTokenAt(bin))

	if n := r.run(t); n != 1 {
		t.Fatalf("a monitoring failure still fails open when attended: %d dispatched\n%s", n, r.out())
	}
	errs := r.err()
	if !strings.Contains(errs, "plan guard: ") || !strings.Contains(errs, "top-level keys are [claudeAiOauth userID]") {
		t.Errorf("the blind line must name what the item holds: %q", errs)
	}
	if !strings.Contains(errs, "claudeAiOauth's keys are [expiresAt refreshToken]") {
		t.Errorf("the blind line must name the envelope it did find: %q", errs)
	}
	if strings.Contains(errs, "unreadable") {
		t.Errorf("a wrong shape is not an unreadable item: %q", errs)
	}
}

// A non-string accessToken — the envelope restructures the field rather than
// renaming it. Found adversarially while verifying ranger-base-okbr's close
// (ranger-base-ymmk); the WORDING it gets today is wrong and is
// ranger-base-6ai5's to fix, because the shape's Token func returns "" both
// for an empty string and for a value it could not decode, and the verdict
// reads only the first.
//
// What this pins is the pair that must hold whichever way that line is
// reworded: a value posse cannot read is never handed back AS a token, and no
// byte of it reaches the error. The second matters most here — a restructured
// field carries the credential one level deeper than any case pinned above,
// inside a RawMessage the diagnostic holds and must not print.
func TestANonStringAccessTokenIsNoTokenAndLeaksNothing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, inner string }{
		{"an object", `{"accessToken":{"token":"` + fixtureSecret + `","type":"oauth"},"refreshToken":"r"}`},
		{"an array", `{"accessToken":["` + fixtureSecret + `"]}`},
		{"a number", `{"accessToken":12345,"refreshToken":"r"}`},
		{"a boolean", `{"accessToken":false}`},
		{"a nested envelope", `{"accessToken":{"claudeAiOauth":{"accessToken":"` + fixtureSecret + `"}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tok, meta, err := credentialToken(keychainStore().Name, []byte(`{"claudeAiOauth":`+tc.inner+`}`))
			if err == nil {
				t.Fatalf("a value posse cannot decode must not pass as a token: %q (shape %q)", tok, meta.Shape)
			}
			if tok != "" || meta.Shape != "" {
				t.Errorf("failed read must hand back nothing: tok=%q shape=%q", tok, meta.Shape)
			}
			if strings.Contains(err.Error(), fixtureSecret) {
				t.Errorf("the credential reached the error line: %q", err)
			}
			// The key itself is schema and stays: an operator has to be able
			// to see that accessToken is there at all.
			if !strings.Contains(err.Error(), "accessToken") {
				t.Errorf("the key posse looked for must still be named: %q", err)
			}
		})
	}
}

// The wording ranger-base-6ai5 is about. A restructured accessToken — an
// object, a number, an array, a boolean — is a shape posse cannot read, not
// an incomplete credential, and the two have opposite fixes: one is a line in
// credShapes, the other is a login. Calling the first the second is what
// makes an operator run /login twice against a change no login can touch.
//
// The kind is named because it is what credShapes has to learn; no byte of
// the value is, including the one nested inside it.
func TestNonStringAccessTokenSaysTheShapeChanged(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, inner, kind string }{
		{"an object", `{"accessToken":{"token":"` + fixtureSecret + `","type":"oauth"},"refreshToken":"r"}`, "a JSON object"},
		{"an array", `{"accessToken":["` + fixtureSecret + `"]}`, "a JSON array"},
		{"a number", `{"accessToken":12345,"refreshToken":"r"}`, "a JSON number"},
		{"a boolean", `{"accessToken":false}`, "a JSON boolean"},
		{"a nested envelope", `{"accessToken":{"claudeAiOauth":{"accessToken":"` + fixtureSecret + `"}}}`, "a JSON object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := credentialToken(keychainStore().Name, []byte(`{"claudeAiOauth":`+tc.inner+`}`))
			if err == nil {
				t.Fatal("want a shape failure")
			}
			msg := err.Error()
			t.Logf("operator-visible line:\n  %s", msg)
			if !strings.Contains(msg, "accessToken is present and is "+tc.kind+", not a string") {
				t.Errorf("want the kind named: %q", msg)
			}
			if !strings.Contains(msg, "teach credShapes to read it") {
				t.Errorf("want the fix that can work: %q", msg)
			}
			// The two verdicts that would send the operator somewhere no
			// change can come from.
			for _, wrong := range []string{"re-authenticate rather than change posse", "renamed or dropped"} {
				if strings.Contains(msg, wrong) {
					t.Errorf("wrong fork %q in: %q", wrong, msg)
				}
			}
			if strings.Contains(msg, fixtureSecret) {
				t.Errorf("a value reached the error line: %q", msg)
			}
		})
	}
}

// null is the one non-string that IS honestly "present but empty": it decodes
// into "" without error, which is exactly what the shape's Token func saw. It
// stays on the login side of the fork.
func TestNullAccessTokenIsTheIncompleteCredential(t *testing.T) {
	t.Parallel()
	_, _, err := credentialToken(keychainStore().Name, []byte(`{"claudeAiOauth":{"accessToken":null,"refreshToken":"r"}}`))
	if err == nil {
		t.Fatal("want a shape failure")
	}
	msg := err.Error()
	if !strings.Contains(msg, "incomplete credential") || !strings.Contains(msg, "re-authenticate rather than change posse") {
		t.Errorf("null must read as an incomplete credential: %q", msg)
	}
	if strings.Contains(msg, "the shape changed") || strings.Contains(msg, "renamed or dropped") {
		t.Errorf("null is not a shape posse cannot read: %q", msg)
	}
}

// The verdict must look the field up the way the parser does. encoding/json
// folds field names, so a capitalized accessToken is read normally — and an
// exact map lookup then reports it renamed and prescribes a credShapes entry
// for a name credShapes already matches.
//
// The first case is the control: it measures that the fold is real, which is
// the whole reason the other two say what they say. If Go ever stopped
// folding, this one goes red first and the verdicts below become wrong rather
// than merely unexplained.
func TestAccessTokenIsMatchedTheWayTheParserMatchesIt(t *testing.T) {
	t.Parallel()
	t.Run("a capitalized name still yields the token", func(t *testing.T) {
		tok, meta, err := credentialToken(keychainStore().Name,
			[]byte(`{"claudeAiOauth":{"AccessToken":"`+fixtureSecret+`","refreshToken":"r"}}`))
		if err != nil {
			t.Fatalf("the parser folds field names, so this reads: %v", err)
		}
		if tok != fixtureSecret || meta.Shape != "claudeAiOauth.accessToken" {
			t.Errorf("tok=%q shape=%q", tok, meta.Shape)
		}
	})
	for _, tc := range []struct{ name, inner, want, notWant string }{
		{
			name:    "capitalized and empty — a login, not a rename",
			inner:   `{"AccessToken":"","refreshToken":"r"}`,
			want:    "re-authenticate rather than change posse",
			notWant: "renamed or dropped",
		},
		{
			name:    "capitalized and restructured — credShapes, not a login",
			inner:   `{"AccessToken":{"token":"` + fixtureSecret + `"}}`,
			want:    "accessToken is present and is a JSON object, not a string",
			notWant: "renamed or dropped",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := credentialToken(keychainStore().Name, []byte(`{"claudeAiOauth":`+tc.inner+`}`))
			if err == nil {
				t.Fatal("want a shape failure")
			}
			msg := err.Error()
			t.Logf("operator-visible line:\n  %s", msg)
			if !strings.Contains(msg, tc.want) {
				t.Errorf("want %q in: %q", tc.want, msg)
			}
			if strings.Contains(msg, tc.notWant) {
				t.Errorf("must not say %q: %q", tc.notWant, msg)
			}
			// The reader has to be able to reconcile a verdict about
			// accessToken with a key list that spells it AccessToken.
			if !strings.Contains(msg, "case-insensitively") {
				t.Errorf("want the spelling explained: %q", msg)
			}
			if strings.Contains(msg, fixtureSecret) {
				t.Errorf("a value reached the error line: %q", msg)
			}
		})
	}
}

// The note only appears when the spelling actually differs — an envelope that
// spells the field the way posse does gets no aside about casing.
func TestExactSpellingGetsNoCasingAside(t *testing.T) {
	t.Parallel()
	_, _, err := credentialToken(keychainStore().Name, []byte(`{"claudeAiOauth":{"accessToken":"","refreshToken":"r"}}`))
	if err == nil {
		t.Fatal("want a shape failure")
	}
	if strings.Contains(err.Error(), "case-insensitively") {
		t.Errorf("nothing to explain when the name matches exactly: %q", err)
	}
}
