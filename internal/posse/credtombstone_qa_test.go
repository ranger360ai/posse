//go:build posse_arm3

package posse

// ranger-base-58qr8: the 88-hour blind window. The keychain item was there,
// it was posse's own envelope, and BOTH token fields were empty strings while
// every other key — scopes, subscriptionType, rateLimitTier,
// refreshTokenExpiresAt — was still sitting in it.
//
// MEASURED 2026-09-28 off the shipped darwin-arm64 bundles on this box
// (2.1.268 and 2.1.283, `strings` over the binary; no store was read and no
// credential value was seen): that is a shape the runtime writes ON PURPOSE.
// On an `invalid_grant` from the refresh endpoint it rewrites its own
// envelope as `{...envelope, refreshToken: "", accessToken: "", expiresAt: 0}`
// and counts `tengu_oauth_refresh_token_cleared_on_disk`. The item is a
// tombstone, not a write that was interrupted.
//
// Two things were wrong about what posse said, and each is pinned here.
//
//  1. The line OPENED with "holds no token in any shape posse knows". That
//     clause is about shapes, and the operator read it as one — the bead asks
//     for a second credShapes entry by name, and there is none to add
//     (credShapes' own comment carries the measurement). The fork the code
//     actually took was the empty one, and its verdict said so 200 bytes
//     later, where the quoted report had already been cut off.
//  2. The tail then said "a refreshToken is present, so a refresh that did
//     not complete fits". The refreshToken was present and EMPTY, so the only
//     thing that could have completed that refresh is the field that was
//     cleared. That clause sends an operator to wait for a refresh that can
//     never land.

import (
	"strings"
	"testing"
)

// The tombstone, whole: the measured key set with both tokens emptied.
const tombstoneEnvelope = `{"claudeAiOauth":{"accessToken":"","refreshToken":"",` +
	`"expiresAt":0,"refreshTokenExpiresAt":1758816000000,"scopes":["user:inference"],` +
	`"subscriptionType":"max","rateLimitTier":"tier"}}`

func TestQABothTokensEmptyIsNotAShapeAndNotAPendingRefresh(t *testing.T) {
	t.Parallel()
	_, _, err := credentialToken(keychainStore().Name, []byte(tombstoneEnvelope))
	if err == nil {
		t.Fatal("an envelope with no token in it is not a credential")
	}
	msg := err.Error()
	t.Logf("operator-visible line:\n  %s", msg)

	// The lead names the class this IS.
	if !strings.HasPrefix(msg, keychainStore().Name+" holds a token field that is PRESENT BUT EMPTY") {
		t.Errorf("the first clause must name the empty fork:\n  %s", msg)
	}
	// And never the class it is not. This is the substring the bead quoted.
	if strings.Contains(msg, "holds no token in any shape posse knows") {
		t.Errorf("the empty fork must not be told as an unknown shape:\n  %s", msg)
	}
	// The shapes tried still ride on the line — an operator comparing this
	// against a bundle needs to know which path was looked down.
	if !strings.Contains(msg, "tried claudeAiOauth.accessToken") {
		t.Errorf("the shapes tried must survive the new lead:\n  %s", msg)
	}
	// The refresh clause is the tombstone's, not the interrupted write's.
	if strings.Contains(msg, "a refreshToken is present, so a refresh that did not complete fits") {
		t.Errorf("an empty refreshToken cannot complete a refresh:\n  %s", msg)
	}
	for _, want := range []string{
		"the refreshToken is present and empty too", "no refresh can complete from here", "log in",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("want %q in:\n  %s", want, msg)
		}
	}
	// The move is still the operator's and never a developer's: there is no
	// shape here to teach, and saying otherwise is what cost the 88 hours.
	if strings.Contains(msg, "teach credShapes") {
		t.Errorf("an emptied field is not a shape to learn:\n  %s", msg)
	}
}

// The other three states of the same clause, so the assertion above is not
// satisfied by a sentence that simply always says "empty too". These are the
// forks refreshHint has to keep apart, and the first of them is the common
// case that has been right since ranger-base-6ai5.
func TestQARefreshHintKeepsItsThreeStatesApart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, inner, want, notWant string
	}{{
		name:  "present and holding something — a refresh that did not finish",
		inner: `{"accessToken":"","refreshToken":"r"}`,
		want:  "a refreshToken is present, so a refresh that did not complete fits",
	}, {
		name:  "absent — nothing to say about one",
		inner: `{"accessToken":""}`,
		want:  "incomplete credential",
		// Folded the way the parser folds (ranger-base-ogzh): absent is
		// absent, and the hint is not printed over a field that is not there.
		notWant: "refreshToken is present",
	}, {
		name: "present and null — it holds nothing, which is the tombstone's fact",
		// null unmarshals into a string as a no-op, so the parser saw ""
		// exactly as it does for an empty string. Same reading, same clause.
		inner: `{"accessToken":"","refreshToken":null}`,
		want:  "the refreshToken is present and empty too",
	}, {
		name: "present and not a string at all — posse can say present, no more",
		// A restructured refreshToken is not a field posse can call empty.
		// "Present" is the whole of what that reading supports.
		inner:   `{"accessToken":"","refreshToken":{"v":1}}`,
		want:    "a refreshToken is present",
		notWant: "empty too",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := credentialToken(keychainStore().Name, []byte(`{"claudeAiOauth":`+tc.inner+`}`))
			if err == nil {
				t.Fatal("want a shape failure")
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.want) {
				t.Errorf("want %q in:\n  %s", tc.want, msg)
			}
			if tc.notWant != "" && strings.Contains(msg, tc.notWant) {
				t.Errorf("must not say %q:\n  %s", tc.notWant, msg)
			}
		})
	}
}

// The clause folds on SPELLING the way ranger-base-ogzh's pin requires, on
// the empty side too: encoding/json matches field names case-insensitively,
// so an envelope spelling it `RefreshToken` must reach the same verdict. An
// exact map lookup here would call the tombstone an absent refreshToken and
// drop the one clause that says a refresh cannot help.
func TestQAEmptyRefreshHintFoldsTheWayTheParserDoes(t *testing.T) {
	t.Parallel()
	for _, spelling := range []string{"refreshToken", "RefreshToken", "REFRESHTOKEN"} {
		t.Run(spelling, func(t *testing.T) {
			_, _, err := credentialToken(keychainStore().Name,
				[]byte(`{"claudeAiOauth":{"accessToken":"","`+spelling+`":""}}`))
			if err == nil {
				t.Fatal("want a shape failure")
			}
			if !strings.Contains(err.Error(), "the refreshToken is present and empty too") {
				t.Errorf("the empty hint must fold like the parser does (%s):\n  %s", spelling, err)
			}
		})
	}
}

// The forks that are NOT this one keep their lead byte for byte. The shape
// diagnosis is one piece of code for both stores (ADR 0019 V7) and two pins
// elsewhere assert this exact clause on the renamed fork
// (credcomposite_test.go, credentialclass_test.go) — this says so where the
// change was made, so the reason those two are green is written down here.
func TestQAUnknownShapeKeepsItsLead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, blob string }{
		{"renamed or dropped", `{"claudeAiOauth":{"refreshToken":"r","expiresAt":1}}`},
		{"restructured into a non-string", `{"claudeAiOauth":{"accessToken":{"v":1}}}`},
		{"a different credential structure", `{"apiKey":"x"}`},
		{"the envelope is not an object", `{"claudeAiOauth":"x"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := credentialToken(keychainStore().Name, []byte(tc.blob))
			if err == nil {
				t.Fatal("want a shape failure")
			}
			msg := err.Error()
			if !strings.Contains(msg, "holds no token in any shape posse knows (tried claudeAiOauth.accessToken)") {
				t.Errorf("a shape posse cannot read keeps its own lead:\n  %s", msg)
			}
			if strings.Contains(msg, "PRESENT BUT EMPTY") {
				t.Errorf("only the empty fork takes the empty lead:\n  %s", msg)
			}
		})
	}
}
