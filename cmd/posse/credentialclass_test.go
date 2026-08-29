package main

// ADR 0019 P7's other surface (bead rangerhq-pwpx): the cockpit header names
// WHICH credential failure a blind guard is, not only how long it has been
// blind.
//
// `plan — · guard blind 40m` was every word of it true on 2026-08-22 and
// told the operator nothing to do. The clock is how long; the class is what,
// and on a 401 and a 403 the two next moves are opposites — one is "run
// `claude` once", the other is "stop, that credential will never work".

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/rhq"
)

// planClassCockpit is a cockpit whose plan scan reads a fake usage endpoint
// answering one status, in a home of its own.
//
// The endpoint is REAL (an httptest listener on loopback) and reached
// through the shipped reader, the shipped cache and scanPlan — not a planRead
// built by hand. That is the half a struct-literal test cannot pin: the
// segment tests below would all stay green with the scan's classification
// line deleted.
func planClassCockpit(t *testing.T, status int) *cockpit {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RHQ_PLAN_USAGE_URL", srv.URL)
	a := rhq.NewAppAt(filepath.Join(home, "config"))
	if err := os.MkdirAll(a.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("plan_guard_5h: 70\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	c := &cockpit{app: a, now: func() time.Time { return at }, plans: make(chan planRead, 1)}
	return c
}

// scanOnce runs the whole path — endpoint → reader → cache → scanPlan → the
// channel → applyPlan — and hands back the header's plan segment.
func scanOnce(t *testing.T, c *cockpit) string {
	t.Helper()
	c.scanPlan()
	select {
	case r := <-c.plans:
		c.applyPlan(r)
		return c.planLine
	default:
		t.Fatal("the scan landed nothing on the channel")
		return ""
	}
}

// The two statuses whose next moves are opposites, end to end, plus the
// availability control that must stay unclassified.
func TestCockpitHeaderNamesTheCredentialClass(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		says   string
		never  []string
	}{
		{"401", http.StatusUnauthorized, "credential stale (401)", []string{"not entitled"}},
		// The header is a place the forbidden word could come back in: it
		// is written once, read at a glance, and nothing else pins it.
		{"403", http.StatusForbidden, "credential not entitled (403)", []string{"refresh", "stale"}},
		{"429", http.StatusTooManyRequests, "rate limited", []string{"credential"}},
		// Control: a 500 is weather. A header that named a class here would
		// be inventing a credential problem out of an outage.
		{"500 is not a class", http.StatusInternalServerError, "", []string{"credential", "rate limited"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := scanOnce(t, planClassCockpit(t, tc.status))
			if !strings.HasPrefix(got, "plan — · guard blind ") {
				t.Fatalf("a failed read is still blind, and the clock still runs: %q", got)
			}
			if tc.says != "" && !strings.Contains(got, "· "+tc.says) {
				t.Errorf("header %q must name the class %q", got, tc.says)
			}
			if tc.says == "" && strings.Count(got, "·") != 1 {
				t.Errorf("an unclassified failure says blind and nothing more: %q", got)
			}
			for _, n := range tc.never {
				if strings.Contains(got, n) {
					t.Errorf("header %q must not say %q", got, n)
				}
			}
		})
	}
}

// The segment's own rules, which the end-to-end test above cannot vary: the
// class rides between the clock and the ledger policy, an unclassified blind
// read adds nothing, and none of the three states that are NOT blindness
// grows a class.
func TestPlanSegmentPlacesTheClass(t *testing.T) {
	at := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	seg := func(r planRead) string {
		c := &cockpit{now: func() time.Time { return at }}
		c.planReadAt = at.Add(-14 * time.Minute)
		return c.planSegment(r)
	}

	if got := seg(planRead{guarded: true, class: "credential unreadable"}); got != "plan — · guard blind 14m · credential unreadable" {
		t.Errorf("class after the clock: %q", got)
	}
	if got := seg(planRead{guarded: true, class: "credential stale (401)", ledger: true}); got != "plan — · guard blind 14m · credential stale (401) — ledger brake" {
		t.Errorf("what broke, then what the shop does about it: %q", got)
	}
	// Without-arm: no class, no separator, and the line is byte-for-byte the
	// one the header has always drawn.
	if got := seg(planRead{guarded: true}); got != "plan — · guard blind 14m" {
		t.Errorf("an unclassified blind read is unchanged: %q", got)
	}
	// A good reading is the reading — a stale class field must never reach a
	// header that has numbers to show.
	if got := seg(planRead{guarded: true, line: "5h 42% · 7d 61%", class: "credential stale (401)"}); got != "5h 42% · 7d 61%" {
		t.Errorf("a reading is the reading: %q", got)
	}
	// The two guard-OFF states have no clock, so they have no class either:
	// nothing failed to be read.
	for _, r := range []planRead{
		{guarded: true, noSource: true, class: "credential unreadable"},
		{guarded: true, noAdapter: true, class: "credential unreadable"},
	} {
		if got := seg(r); strings.Contains(got, "unreadable") {
			t.Errorf("guard-off is not a failed read: %q", got)
		}
	}
	// No guard, nothing to say, class or no class.
	if got := seg(planRead{class: "credential stale (401)"}); got != "" {
		t.Errorf("an unarmed guard says nothing: %q", got)
	}
}
