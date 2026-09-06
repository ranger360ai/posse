//go:build posse_arm2

package posse

// The 401 that names the expiry it read (bead ranger-base-4poib).
//
// MEASURED 2026-09-03 on the operator's box: the meter's OAuth access token
// carries `expiresAt` eight hours after the operator's last interactive
// `claude`, and the endpoint's 401 arrived 37 minutes after that stamp had
// passed. posse said "credential stale" — true, and indistinguishable from
// the sentence it prints for a 401 on a token that is perfectly live. The
// expiry was already in the bytes the token was parsed out of.
//
// So the fix is a SENTENCE, threaded from the seam's CredMeta through the
// reader into *AuthFailure, and these are its arms. What the fix is NOT is
// a fifth failure class: the four are ADR 0019 D2's, park-vs-degrade reads
// no diagnosis string (ADR 0018 §2), and the class pins below are here so
// that stays true after the sentence forked.

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// authNow is the read; authExpiry is the stamp the credential carried. The
// gap is the measured one, 37 minutes, so a rendering that reaches for
// time.Now() instead of the two times it was handed cannot pass by accident.
//
// Functions and not package vars: every test here takes t.Parallel, and a
// frozen instant that is a var is shared state the parallel gate has to be
// argued out of one allowlist line at a time (cmd/testparallel). A value
// nothing can write needs no argument.
func authNow() time.Time    { return time.Date(2026, 9, 3, 23, 28, 0, 0, time.UTC) }
func authExpiry() time.Time { return time.Date(2026, 9, 3, 22, 51, 0, 0, time.UTC) }

// expiryReader is the shipped reader with a 401 endpoint, a fixed clock and
// a credential whose meta says when it dies — production's own wiring, so a
// plumbing drop anywhere between the seam and the sentence fails here.
func expiryReader(expires time.Time) *AnthropicPlanReader {
	return &AnthropicPlanReader{
		URL:  PlanUsageURL,
		HTTP: usageEndpointAnswering(http.StatusUnauthorized),
		Now:  authNow,
		Token: func() (string, CredMeta, error) {
			return fakeToken, CredMeta{Source: KeychainService, Shape: "claudeAiOauth.accessToken", ExpiresAt: expires}, nil
		},
	}
}

func TestA401NamesTheExpiryItRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		expires time.Time
		says    []string
		never   []string
	}{
		{
			// The bug, as measured. The stamp and the age are both in the
			// line: a date can be checked against the keychain, and an age
			// of 37m says the operator's own login this morning wore off
			// rather than never having happened.
			name:    "expired 37 minutes before the read",
			expires: authExpiry(),
			says:    []string{"401", "credential EXPIRED", "2026-09-03 22:51Z", "37m ago", "run `claude` once to refresh"},
			never:   []string{"credential stale", "NOT expired"},
		},
		{
			// The other half of "pin the arithmetic": a live token the
			// endpoint refused anyway. A refresh hands back the same
			// token, so the sentence must not send anyone to `/login`.
			name:    "still valid when it was presented",
			expires: authNow().Add(7 * time.Hour),
			says: []string{"401", "had NOT expired", "2026-09-04 06:28Z", "in 7h",
				"not a freshness problem", "scope or entitlement"},
			never: []string{"credential stale", "once to refresh", "EXPIRED"},
		},
		{
			// Cannot tell stays cannot tell (ADR 0019 D5). This is the
			// sentence every existing surface pins, byte for byte.
			name:    "no expiry in the envelope",
			expires: time.Time{},
			says:    []string{"usage endpoint returned 401 Unauthorized: credential stale — run `claude` once to refresh"},
			never:   []string{"EXPIRED", "NOT expired", "ago"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := expiryReader(c.expires).Read()
			af := AuthFailureReason(err)
			if af == nil {
				t.Fatalf("want *AuthFailure, got %T: %v", err, err)
			}
			msg := err.Error()
			for _, w := range c.says {
				if !strings.Contains(msg, w) {
					t.Errorf("the 401 does not carry %q:\n  %s", w, msg)
				}
			}
			for _, n := range c.never {
				if strings.Contains(msg, n) {
					t.Errorf("the 401 must not say %q:\n  %s", n, msg)
				}
			}
			// The rule the whole seam is under, restated on the new clauses.
			if strings.Contains(msg, fakeToken) {
				t.Errorf("a credential must never appear in a diagnostic:\n  %s", msg)
			}
			// Whatever the sentence, the CLASS is the status code and
			// nothing else — the four classes are ADR 0019 D2's, and
			// park-vs-degrade reads no diagnosis string (ADR 0018 §2).
			if got := PlanFailureOf(err); got != PlanFailStale {
				t.Errorf("class moved with the sentence: %q", got)
			}
			if got := PlanFailToken(err); got != "401" {
				t.Errorf("governance key moved with the sentence: %q", got)
			}
			if !af.Stale() {
				t.Error("Stale() is 401-and-only-401 and must not read the sentence")
			}
		})
	}
}

// The age is measured between the two times the failure was HANDED, not
// against the wall clock. This error is logged, cached, and re-rendered
// long after the response; "37m ago" has to keep meaning 37 minutes after
// the read that saw it, and an Error() reaching for time.Now() would drift
// with every rendering and read as years on a stamp from 2026.
func TestTheExpiryAgeIsFixedAtTheReadNotAtTheRendering(t *testing.T) {
	t.Parallel()
	_, err := expiryReader(authExpiry()).Read()
	first := err.Error()
	if !strings.Contains(first, "37m ago") {
		t.Fatalf("want the measured gap:\n  %s", first)
	}
	if second := err.Error(); second != first {
		t.Errorf("the sentence drifted between renderings:\n  %s\n  %s", first, second)
	}
}

// A 403 is a different class with a different sentence, and it has never
// been a freshness problem. The expiry clauses must not leak into it — a
// setup-token with a live stamp is still not entitled.
func TestA403SaysNothingAboutExpiry(t *testing.T) {
	t.Parallel()
	r := expiryReader(authExpiry())
	r.HTTP = usageEndpointAnswering(http.StatusForbidden)
	_, err := r.Read()
	msg := err.Error()
	if !strings.Contains(msg, "not entitled to plan windows") {
		t.Fatalf("the 403 sentence changed:\n  %s", msg)
	}
	for _, never := range []string{"EXPIRED", "ago", "NOT expired", "once to refresh"} {
		if strings.Contains(msg, never) {
			t.Errorf("the 403 must not carry %q:\n  %s", never, msg)
		}
	}
}

// expiryAgo's edges, the way expiryIn's are pinned (credexpiry_dup_qa_test).
// It truncates, and it switches units at an hour and at two days.
//
// The hour edge is the one this bead bought: an 8h credential dies inside a
// working session, and an age of 59 minutes told in whole hours is "0h ago",
// which is the answer erased rather than rounded.
func TestExpiryAgoEdges(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want string }{
		{"0s", "just now"},
		{"59s", "just now"},
		{"1m", "1m ago"},
		{"37m", "37m ago"},
		{"59m59s", "59m ago"},
		{"1h", "1h ago"},
		{"47h59m", "47h ago"},
		{"48h", "2d ago"},
		{"2400h", "100d ago"},
	} {
		d, err := time.ParseDuration(c.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := expiryAgo(d); got != c.want {
			t.Errorf("expiryAgo(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}

// renderStamp prints the precision the stamp HAS. A session mint's
// `# expires=` is a day and parses to midnight, and giving it a time of day
// would be a precision nobody measured (stampDate's own rule); the meter's
// `expiresAt` is a millisecond instant on an 8h credential, and a date is
// not a rendering of it at all.
func TestRenderStampKeepsThePrecisionItWasGiven(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	if got := renderStamp(day); got != "2026-09-09" {
		t.Errorf("a day stamp gains a fabricated clock: %q", got)
	}
	if got := renderStamp(authExpiry()); got != "2026-09-03 22:51Z" {
		t.Errorf("a measured instant loses its clock: %q", got)
	}
	// The report says the same thing, both ways round: a dead meter
	// credential names the hour AND how long ago, because "EXPIRED
	// 2026-09-03" read at 23:28 on 2026-09-03 does not say which.
	if got := renderExpiry(authExpiry(), authNow()); got != "EXPIRED 2026-09-03 22:51Z (37m ago)" {
		t.Errorf("the report's expired row: %q", got)
	}
	// And the session mint's row is unchanged — that row and the cockpit
	// warning are pinned against each other elsewhere, and this fix must
	// not have moved either.
	if got := renderExpiry(day, authNow()); got != "2026-09-09 (in 5d)" {
		t.Errorf("the report's session row moved: %q", got)
	}
}

// The runbook's move-2 table is matched by SENTENCE — an operator greps the
// line they got. A class that gained two sentences and left the page with
// one is the drift credentialrunbook_qa_test.go exists to catch, so the two
// new rows are held the same way it holds the others: both directions, and
// the code side first.
//
//	fragment ⊆ what production actually produces  (the code moved → red)
//	fragment ⊆ the runbook                        (the page moved  → red)
//
// The quoted fragments stop before the variable part. A row cannot carry a
// live timestamp, and a fragment that did would be pinning this test's own
// fixture rather than the page.
func TestTheRunbookQuotesBothNew401Sentences(t *testing.T) {
	t.Parallel()
	page := crRunbook(t)
	for _, c := range []struct{ name, fragment string }{
		{"401 expired", "credential EXPIRED"},
		{"401 expired, the move", "— run `claude` once to refresh"},
		{"401 not expired", "the credential posse presented had NOT expired"},
		{"401 not expired, the move", "so this is not a freshness problem"},
	} {
		t.Run(c.name, func(t *testing.T) {
			expires := authExpiry()
			if strings.Contains(c.name, "not expired") {
				expires = authNow().Add(7 * time.Hour)
			}
			_, err := expiryReader(expires).Read()
			if !strings.Contains(crFlat(err.Error()), crFlat(c.fragment)) {
				t.Fatalf("the code no longer says this — the runbook's row is now wrong.\n  code: %q\n  quoted: %q", err, c.fragment)
			}
			if !strings.Contains(page, crFlat(c.fragment)) {
				t.Errorf("docs/runbooks/credential-rotation.md does not quote it: %q", c.fragment)
			}
		})
	}
}
