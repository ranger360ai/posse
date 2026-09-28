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

// And the same sentence at the SURFACE THE OPERATOR READS (added while
// verifying this close, ranger-base-tl0mg).
//
// Every pin above calls credentialToken and reads the error it returns. That
// is the right subject for which clause the code CHOOSES, and it is not quite
// the subject of this bead's complaint: the TAIL of this line had been correct
// since ranger-base-6ai5, and the operator still filed 88 blind hours having
// read the head and stopped, because the head was 200 bytes upstream of the
// part that was right and the report they quoted was cut before it. A fix
// whose whole point is WHICH CLAUSE COMES FIRST is verified at the line the
// plan guard actually prints, or it is not verified at all — every unit pin
// above stays green over a guard that truncates, re-wraps or prefixes its way
// back into the same failure.
//
// So this one runs the guard: a keychain stub holding the measured tombstone,
// the blind line taken off the rig's stderr, asserted to OPEN with the empty
// fork and never with the class it is not. It is the sibling of
// TestPlanGuardBlindLineNamesTheShapeItFound, which takes the same rig down
// the renamed-or-dropped fork.
//
// MUTATION-CHECKED (run on ranger-base-tl0mg): returning credLeadUnknown for
// the empty fork reds this pin at the SURFACE as well as the unit pins above,
// which is what says the lead survives the whole path and not just the return.
func TestQATheTombstoneLineReachesTheOperatorLeadFirst(t *testing.T) {
	bin := keychainStub(t, "#!/bin/sh\ncat <<'JSON'\n"+tombstoneEnvelope+"\nJSON\n")

	r := newBlindRig(t, guardOn)
	keychainOnly(planReaderOf(r.d), keychainTokenAt(bin))

	if n := r.run(t); n != 1 {
		t.Fatalf("a monitoring failure still fails open when attended: %d dispatched\n%s", n, r.out())
	}
	errs := r.err()
	t.Logf("operator-visible blind line:\n  %s", errs)

	if !strings.Contains(errs, "plan guard: ") {
		t.Fatalf("the rig printed no plan guard line at all: %q", errs)
	}
	// The lead, at the surface. This is the whole fix.
	if !strings.Contains(errs, "holds a token field that is PRESENT BUT EMPTY") {
		t.Errorf("the operator-visible line must open the empty fork by name: %q", errs)
	}
	// And never the clause the bead quoted.
	if strings.Contains(errs, "holds no token in any shape posse knows") {
		t.Errorf("the operator-visible line still names the class this is not: %q", errs)
	}
	// The tombstone's own tail: not a refresh anybody can wait for.
	if !strings.Contains(errs, "both token fields hold nothing") {
		t.Errorf("the tombstone's tail must say both fields are empty: %q", errs)
	}
	if strings.Contains(errs, "a refresh that did not complete fits") {
		t.Errorf("a cleared refreshToken must not be reported as a refresh in flight: %q", errs)
	}
	// Shapes and key names only, never a value — the guardrail this whole
	// file works under.
	if strings.Contains(errs, "user:inference") {
		t.Errorf("the blind line echoed a value out of the envelope: %q", errs)
	}
}
