//go:build posse_arm2

package posse

// Filed verifying ranger-base-6ai5's close (ranger-base-ogzh). The fix's own
// board covers the accessToken half of the verdict in four mutations. The
// refreshToken half — the parenthetical that tells an operator a refresh was
// started and did not finish — was stated in the close and pinned by nothing:
// reverting `env.RefreshToken != nil` to the exact map lookup `inner["refreshToken"]`
// left every credential pin green (measured 2026-08-28, mutation M5).
//
// That is the same defect as the one the bead reports, one field over. The
// verdict must read the field the way the parser reads it, or an envelope
// spelling it `RefreshToken` loses the one clause that says which kind of
// login problem this is.

import (
	"strings"
	"testing"
)

func TestQARefreshTokenHintFoldsTheWayTheParserDoes(t *testing.T) {
	t.Parallel()
	for _, spelling := range []string{"refreshToken", "RefreshToken", "REFRESHTOKEN"} {
		t.Run(spelling, func(t *testing.T) {
			_, _, err := credentialToken(keychainStore().Name,
				[]byte(`{"claudeAiOauth":{"accessToken":"","`+spelling+`":"r"}}`))
			if err == nil {
				t.Fatal("want a shape failure")
			}
			msg := err.Error()
			if !strings.Contains(msg, "a refreshToken is present") {
				t.Errorf("the refresh hint must fold like the parser does (%s):\n  %s", spelling, msg)
			}
		})
	}
	// The other arm, so the assertion above is not satisfied by a clause
	// that is simply always printed: no refreshToken, no hint.
	_, _, err := credentialToken(keychainStore().Name, []byte(`{"claudeAiOauth":{"accessToken":""}}`))
	if err == nil {
		t.Fatal("want a shape failure")
	}
	if strings.Contains(err.Error(), "a refreshToken is present") {
		t.Errorf("no refreshToken in the envelope, so no hint about one:\n  %s", err)
	}
}
