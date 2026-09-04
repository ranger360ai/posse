package posse

// ADR 0019 P7 (bead rangerhq-pwpx): the plan guard's credential failures are
// FOUR classes, each its own error and each its own next move, and the class
// reaches the operator on the two surfaces they actually look at.
//
// rangerhq-ytyj built two of the four (401 and 403) and the dispatch line
// that carries them. What this file adds is the pair the amended D2 named
// after it — the UNREADABLE class, which had no sentence and no type at all,
// and the SHORT NAME the cockpit header has room for — plus the one table
// that asks all four at once, because "four distinct errors" is a claim
// about the set and cannot be pinned one row at a time.
//
// The negatives are the point of every row here. A 403 that says "refresh"
// tells the operator to do the one thing that produces the same 403 forever
// (MEASURED, ranger-base-0qp). An unreadable keychain reported as staleness
// is the 2026-08-24 misdiagnosis that got `plan_guard_blind_max: 0` set for
// hours. Both are sentences that are wrong in a way "blind" never was.

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// unreadableKeychain is the unreadable class as production makes it: the
// real darwin adapter, reading a `security` that exits non-zero, through the
// real readStore. The stub is NAMED to the adapter (ranger-base-ypf5 made
// the exec absolute), so this row runs and means the same thing on linux.
//
// fallbackDir is what keeps exit 44 an unreadable store since the adapter
// became the composite (ADR 0019 D2 as amended, ranger-base-5jdzh): 44 with
// a fallback file beside it is a CREDENTIAL now, so this row has to name a
// config directory with no file in it or it would read whatever the box
// running the suite happens to have. It uses t.Setenv, which is why the
// three tests below no longer run in parallel.
func unreadableKeychain(t *testing.T) error {
	t.Helper()
	fallbackDir(t)
	_, _, err := readStore(keychainStoreAt(keychainStub(t, "#!/bin/sh\nexit 44\n")))
	if err == nil {
		t.Fatal("a `security` that exits 44 is not a credential")
	}
	return err
}

// wrongShapeKeychain is the other half of the same class: `security`
// answered, and what it answered holds no token posse knows how to find.
// ADR 0019 D2 puts "JSON missing" in the unreadable row on purpose — the
// operator's move is the same one, and it is not a refresh.
func wrongShapeKeychain(t *testing.T) error {
	t.Helper()
	// The okbr envelope: valid JSON, posse's own key, and the token field
	// renamed out from under it.
	_, _, err := readStore(keychainStoreAt(keychainStub(t,
		"#!/bin/sh\ncat <<'JSON'\n{\"claudeAiOauth\":{\"refreshToken\":\"r\",\"expiresAt\":1}}\nJSON\n")))
	if err == nil {
		t.Fatal("an envelope with no token is not a credential")
	}
	return err
}

// P7's first half, whole: four inputs, four classes, four sentences, and no
// sentence carrying another class's move.
//
// The `says` column is what the operator is TOLD and the `never` column is
// what they must not be — and `never` is where this test earns its keep. A
// classifier that collapsed all four into one error would pass every `says`
// row if the one sentence were long enough to contain all four fragments;
// the distinctness assertions below (class, and the pairwise message
// comparison) are what makes that impossible.
func TestPlanReadHasFourCredentialFailureClasses(t *testing.T) {
	// No t.Parallel: unreadableKeychain names a config directory (t.Setenv).
	for _, tc := range []struct {
		name  string
		err   func(*testing.T) error
		class PlanFailure
		says  []string
		never []string
	}{{
		name:  "unreadable: security failed",
		err:   unreadableKeychain,
		class: PlanFailUnreadable,
		says: []string{
			"keychain item", "unreadable",
			"keychain ACL may have been dropped by `make install`",
			"grant access when prompted, or run `claude` once",
		},
		// The whole reason this class was given a sentence: it is NOT a
		// freshness problem and must never be reported as one. The bans are
		// the MOVES, not the words — an envelope diagnosis legitimately
		// names a key called `refreshToken`, and banning the bare word here
		// would fail on the store's own vocabulary.
		never: []string{"credential stale", "once to refresh", "not entitled"},
	}, {
		name:  "unreadable: envelope holds no token",
		err:   wrongShapeKeychain,
		class: PlanFailUnreadable,
		says: []string{
			"holds no token in any shape posse knows",
			// The move for THIS half is the diagnosis's own last clause,
			// and it is a developer's, not the operator's.
			"teach credShapes the new name",
		},
		// No keychain fix here, and that is ADR 0019 V7 rather than an
		// omission: the shape diagnosis is one piece of code for both
		// platforms, and an item that answered with a renamed key did not
		// lose an ACL — re-granting one would fix nothing.
		never: []string{"credential stale", "once to refresh", "not entitled", "make install"},
	}, {
		name:  "401",
		err:   statusRead(http.StatusUnauthorized),
		class: PlanFailStale,
		says:  []string{"401", "credential stale", "run `claude` once to refresh"},
		never: []string{"not entitled", "make install", "unreadable"},
	}, {
		name:  "403",
		err:   statusRead(http.StatusForbidden),
		class: PlanFailForbidden,
		says: []string{"403", "not entitled to plan windows",
			"a setup-token never will be", "not a freshness problem"},
		// The one word the amended D2 forbids BY NAME, and this message has
		// no store vocabulary in it to make the bare ban unfair.
		never: []string{"refresh", "stale", "make install", "unreadable"},
	}, {
		name:  "429",
		err:   statusRead(http.StatusTooManyRequests),
		class: PlanFailRateLimited,
		says:  []string{"429"},
		never: []string{"credential", "refresh", "make install", "unreadable"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.err(t)
			if got := PlanFailureOf(err); got != tc.class {
				t.Fatalf("PlanFailureOf(%v) = %q, want %q", err, got, tc.class)
			}
			for _, w := range tc.says {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("%q must say %q", err, w)
				}
			}
			for _, n := range tc.never {
				if strings.Contains(err.Error(), n) {
					t.Errorf("%q must not say %q — that is another class's move", err, n)
				}
			}
		})
	}
}

// statusRead is the fake endpoint answering one status, as a row's error.
func statusRead(status int) func(*testing.T) error {
	return func(t *testing.T) error {
		t.Helper()
		ps := newPlanServer(t, 1, 2)
		ps.status = status
		_, err := ps.reader().Read()
		if err == nil {
			t.Fatalf("a %d is never a reading", status)
		}
		return err
	}
}

// "Four DISTINCT errors" as its own assertion, because the table above is
// satisfied by four errors that happen to carry the right words: this one
// fails if any two of the four collapse into the same class or the same
// sentence. It is the mutation check on PlanFailureOf — delete any arm of
// that switch and two rows here become one.
func TestTheFourCredentialClassesAreDistinct(t *testing.T) {
	// No t.Parallel: unreadableKeychain names a config directory (t.Setenv).
	got := map[PlanFailure]string{}
	for name, mk := range map[string]func(*testing.T) error{
		"unreadable": unreadableKeychain,
		"401":        statusRead(http.StatusUnauthorized),
		"403":        statusRead(http.StatusForbidden),
		"429":        statusRead(http.StatusTooManyRequests),
	} {
		err := mk(t)
		c := PlanFailureOf(err)
		if c == "" {
			t.Fatalf("%s: a credential failure with no class is the bug this bead closes: %v", name, err)
		}
		if prev, dup := got[c]; dup {
			t.Fatalf("%s and an earlier row share class %q: %q vs %q", name, c, prev, err)
		}
		got[c] = err.Error()
	}
	if len(got) != 4 {
		t.Fatalf("want four classes, got %d: %v", len(got), got)
	}
	seen := map[string]PlanFailure{}
	for c, msg := range got {
		if prev, dup := seen[msg]; dup {
			t.Errorf("classes %q and %q say the identical sentence %q", prev, c, msg)
		}
		seen[msg] = c
	}
}

// The classes that are NOT this set, which is what keeps the classifier from
// swallowing everything that fails. A dead socket and a 500 are weather; our
// own gate shim refusing our own read is posse's problem and not the
// credential's, and calling it a credential outage is exactly the reading
// that switched the shop's brake off on 2026-08-24.
func TestPlanFailureOfLeavesNonCredentialFailuresAlone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		err   error
		class PlanFailure
	}{
		{"nil", nil, ""},
		{"unreachable", Die("usage endpoint unreachable"), ""},
		{"500", func() error {
			ps := newPlanServer(t, 1, 2)
			ps.status = http.StatusInternalServerError
			_, err := ps.reader().Read()
			return err
		}(), ""},
		{"wrong shape 200", func() error {
			ps := newPlanServer(t, 1, 2)
			ps.body = "{}"
			_, err := ps.reader().Read()
			return err
		}(), ""},
		{"no credential source at all", &NoSource{Runtime: "claude", Purpose: CredMeter, GOOS: "linux"}, ""},
		{"our own gate", &GateRefusal{Cmd: "security", Rule: "Bash(security:*)"}, PlanFailGated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlanFailureOf(tc.err); got != tc.class {
				t.Errorf("PlanFailureOf(%v) = %q, want %q", tc.err, got, tc.class)
			}
		})
	}
}

// A gate refusal reaching the seam must keep its own type and its own
// sentence — the wrapping this bead added to every other store failure must
// not reach it. Without this arm, CredUnreadable swallows the one error
// whose whole purpose is not being mistaken for it.
func TestGateRefusalIsNotWrappedAsUnreadable(t *testing.T) {
	t.Parallel()
	shim := gatedSecurityShim(t)
	_, _, err := readStore(keychainStoreAt(shim))
	if err == nil {
		t.Fatal("a refused shim yields no credential")
	}
	if cu := CredUnreadableReason(err); cu != nil {
		t.Fatalf("our own gate wearing the outage's class: %+v", cu)
	}
	var g *GateRefusal
	if !errors.As(err, &g) {
		t.Fatalf("want *GateRefusal, got %T: %v", err, err)
	}
	for _, n := range []string{"unreadable", "make install", "stale", "refresh"} {
		if strings.Contains(err.Error(), n) {
			t.Errorf("a gate refusal must not carry the credential class's %q: %q", n, err)
		}
	}
}

// The fix rides on the READ failure and not on the shape one, which is ADR
// 0019 V7 as much as it is honesty: the two platforms' shape diagnoses must
// stay one sentence with one store name substituted, and this is the pin on
// the half of that rule this bead could have broken.
func TestTheStoreFixRidesOnTheReadNotTheShape(t *testing.T) {
	// No t.Parallel: unreadableKeychain names a config directory (t.Setenv).
	read := unreadableKeychain(t)
	if !strings.Contains(read.Error(), "make install") {
		t.Errorf("a failed read carries the store's move: %q", read)
	}
	shape := wrongShapeKeychain(t)
	if strings.Contains(shape.Error(), "make install") {
		t.Errorf("a renamed key is not a dropped ACL: %q", shape)
	}
	// Still the same class, because both are credential conditions and the
	// header must not report either as weather.
	if PlanFailureOf(read) != PlanFailUnreadable || PlanFailureOf(shape) != PlanFailUnreadable {
		t.Errorf("one class, two sentences: %q / %q", PlanFailureOf(read), PlanFailureOf(shape))
	}
}

// ─── surface 1: the dispatch skip / stderr line ──────────────────────────────

// P7's second half on the unattended surface: the line a parked pass writes
// names the class, not only the blind clock. 401 and 403 were pinned by
// rangerhq-ytyj; this adds the class that had no sentence to name.
func TestBlindSkipOnUnreadableCredentialNamesTheClass(t *testing.T) {
	fallbackDir(t) // 44 with a file beside it is a credential now, not a failure
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	// The real adapter, with a `security` that fails: the guard asks for the
	// token before it dials, so nothing leaves the box.
	keychainOnly(planReaderOf(r.d), keychainTokenAt(keychainStub(t, "#!/bin/sh\nexit 44\n")))

	r.at(12 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("12m blind must park whatever the class: %d\n%s", n, r.out())
	}
	out := r.out()
	if !strings.Contains(out, "— skipped") {
		t.Fatalf("a parked pass must say why:\n%s", out)
	}
	for _, want := range []string{"blind 12m", "unreadable",
		"keychain ACL may have been dropped by `make install`", "run `claude` once"} {
		if !strings.Contains(out, want) {
			t.Errorf("the park line must carry %q, got:\n%s", want, out)
		}
	}
	// The class this is NOT. "Stale" here sends the operator to `/login`,
	// which does not re-grant a dropped ACL.
	for _, never := range []string{"stale", "not entitled"} {
		if strings.Contains(out, never) {
			t.Errorf("the park line must not say %q:\n%s", never, out)
		}
	}
}

// The 403 arm of the same surface, which the amended D2 singles out: the
// word "refresh" must not appear anywhere on the line an operator reads when
// a setup-token is pointed at the usage endpoint.
func TestBlindSkipOn403NeverSaysRefresh(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	r.ps.status = http.StatusForbidden
	r.ps.body = "forbidden"

	r.at(12 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Fatalf("12m blind on 403 parks like any other: %d\n%s", n, r.out())
	}
	out := r.out()
	for _, want := range []string{"403", "not entitled to plan windows", "a setup-token never will be"} {
		if !strings.Contains(out, want) {
			t.Errorf("the park line must carry %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "refresh") {
		t.Errorf("refreshing a credential that was never entitled produces the same 403 forever:\n%s", out)
	}
}

// ─── the cooldown a 429 buys keeps the 429's class ───────────────────────────

// The hour after a 429 is the long tail an operator actually stares at: the
// endpoint is not asked, so there is no status line to quote, and until this
// bead the surfaces went back to saying "blind" alone for the whole of it.
// The sentence is still the cooldown's own; only the CLASS is shared.
func TestPlanCacheCooldownKeepsTheRateLimitClass(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	b, _ := newTestBackend(t)
	ps := newPlanServer(t, 12, 40)
	ps.status = http.StatusTooManyRequests
	ps.retry = "3600"

	c := b.App.PlanCache("test")
	c.Reader, c.Now = ps.reader(), func() time.Time { return now }
	// Armed: this test is about the hour a 429 buys, and a quiet cache
	// never gets one (planquiet.go).
	c.Quiet = nil
	if _, _, err := c.Read(0); PlanFailureOf(err) != PlanFailRateLimited {
		t.Fatalf("the 429 itself: %v (%q)", err, PlanFailureOf(err))
	}

	// Second ask, inside the cooldown: no request is made at all.
	hits := ps.hits.Load()
	_, _, err := c.Read(0)
	if err == nil {
		t.Fatal("a cooldown is not a reading")
	}
	if ps.hits.Load() != hits {
		t.Fatal("the cooldown asked the endpoint anyway")
	}
	if got := PlanFailureOf(err); got != PlanFailRateLimited {
		t.Errorf("the cooldown's class = %q, want %q: %v", got, PlanFailRateLimited, err)
	}
	if !strings.Contains(err.Error(), "not asking again") {
		t.Errorf("the cooldown keeps its own sentence: %q", err)
	}
}
