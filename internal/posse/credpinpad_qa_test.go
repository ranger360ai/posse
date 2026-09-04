package posse

// QA, ranger-base-2lr90 — verifying the `not doing` close of ranger-base-00a0
// ("a whitespace-padded RHQ_PLAN_USAGE_URL is refused as 'a url with no host'").
//
// That close rests on one factual claim: a padded override is "refused
// loudly today" — i.e. the wrong SENTENCE is the whole defect, and nothing
// behind it leaks. This file pins the half the close depends on, and
// deliberately pins NOTHING about the wording: 00a0's own text asks for the
// sentence to change, and a QA test that froze today's misdiagnosis would
// make that fix arrive looking like a regression.
//
// The invariant, which must hold for EVERY padded spelling:
//   1. the account credential is never read — the token seam is never called;
//   2. the reading never becomes the instance's shared fact (MayShare false);
//   3. the outcome is a refusal or a loopback request, never a request
//      carrying a bearer to a host the padding invented.
//
// (1) is the money-class property: credentialedURL is byte equality against
// the compiled endpoint, so padding can never satisfy it. This test is what
// says so out loud, and what fails the day somebody "helpfully" trims inside
// credentialedURL instead of inside loopbackOverride, which would hand the
// bearer to whatever the padded string parses to.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaddedPlanUsageURLNeverReachesTheCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("a padded override put a bearer on the wire: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"five_hour":{"utilization":42},"seven_day":{"utilization":43}}`))
	}))
	defer srv.Close()

	clean := srv.URL + "/usage"

	// CONTROL — the unpadded loopback override IS honoured and DOES reach the
	// listener. Without this arm every assertion below would also pass on a
	// build where the override never works at all, which would measure
	// nothing (a green pin over a dead seam).
	t.Run("control/unpadded reaches loopback", func(t *testing.T) {
		t.Setenv("RHQ_PLAN_USAGE_URL", clean)
		r := NewAnthropicPlanReader()
		r.Token = refusingToken(t) // loopback is asked WITHOUT the credential
		if r.URLErr != nil {
			t.Fatalf("unpadded loopback override refused: %v", r.URLErr)
		}
		if r.URL != clean {
			t.Fatalf("URL = %q, want %q", r.URL, clean)
		}
		if r.MayShare() {
			t.Error("an override reading must never become the shared fact")
		}
		if _, err := r.Read(); err != nil {
			t.Fatalf("unpadded loopback Read: %v", err)
		}
	})

	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"leading space", " " + clean},
		{"trailing space", clean + " "},
		{"trailing newline", clean + "\n"},          // RHQ_PLAN_USAGE_URL=$(cat portfile)
		{"leading tab", "\t" + clean},
		{"trailing CRLF", clean + "\r\n"},
		{"both ends", "  " + clean + "  "},
		{"leading newline", "\n" + clean},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RHQ_PLAN_USAGE_URL", tc.raw)
			r := NewAnthropicPlanReader()
			// refusingToken t.Fatal's if the credential seam is called at
			// all: this is assertion (1), and it is the whole money story.
			r.Token = refusingToken(t)

			if r.MayShare() {
				t.Errorf("padded override %q became the shared fact", tc.raw)
			}
			// Whatever happens, the compiled endpoint must not be what a
			// padded string silently fell back to while still credentialed.
			if r.URLErr == nil && credentialedURL(r.URL, PlanUsageURL) {
				t.Fatalf("padded override %q is being treated as the credentialed endpoint", tc.raw)
			}

			_, err := r.Read()
			if err == nil {
				// HONOURED — acceptable: the host really is loopback. The
				// listener above is what asserts no bearer came with it.
				t.Logf("honoured (reached loopback, no bearer): %q", tc.raw)
				return
			}
			// REFUSED — also acceptable, and it is what most spellings do
			// today. It must be a *PinRefusal, not a crash and not an
			// outage report, which would park the fleet on a typo.
			var pin *PinRefusal
			if !errors.As(err, &pin) {
				t.Errorf("padded override %q failed as something other than a pin refusal: %v", tc.raw, err)
				return
			}
			t.Logf("refused: %q -> %v", tc.raw, err)
		})
	}
}
