package rhq

// QA pins for docs/runbooks/credential-rotation.md (rangerhq-m10j — the
// four moves of the instance ADR 0019 D5, now all decided).
//
// The defect class this exists for is drift, and it is the expensive kind.
// Move 2 of that runbook is a table of five failure sentences, and the whole
// value of the table is that the operator can match the sentence they got
// against the row that tells them what to do. A sentence that has since been
// reworded in the code is worse than no runbook: it sends the reader looking
// for a row that is not there, or — the 2026-08-24 shape — lets them settle
// on the nearest-looking row and act on the wrong diagnosis.
//
// So the runbook does not paraphrase. It quotes, and these arms hold the
// quotes against the code that emits them, in both directions:
//
//	fragment ⊆ what production actually produces  (the code moved → red)
//	fragment ⊆ the runbook                        (the page moved  → red)
//
// Neither direction alone is a pin. The first without the second is a test of
// the code by itself; the second without the first is a page agreeing with a
// literal in a test file, which is drift with an extra step.
//
// The whitespace normalisation below is deliberate and deliberately narrow:
// markdown wraps, and a table cell or a fenced block may break a sentence
// across lines with indentation. Runs of whitespace collapse to one space and
// nothing else changes — a reworded sentence still fails.

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// crRunbook is the page, whitespace-normalised, skipping when this checkout
// does not carry docs/ (a tarball, a pruned build tree).
func crRunbook(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../docs/runbooks/credential-rotation.md")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("credential-rotation runbook not present: %v", err)
	}
	return crFlat(string(b))
}

var crSpace = regexp.MustCompile(`\s+`)

func crFlat(s string) string { return crSpace.ReplaceAllString(s, " ") }

// crQuote is one thing the runbook says out loud, and where it comes from.
type crQuote struct {
	name string
	// produced is what the code emits today, built by calling the code.
	produced string
	// fragment is what the runbook quotes: the whole sentence where the
	// runbook can carry the whole sentence, and the invariant part where it
	// renders placeholders for the variable part.
	fragment string
}

// TestTheRunbookQuotesTheSentencesTheCodeActuallyEmits is the drift pin.
//
// Every fragment is checked against production FIRST. That ordering is the
// point: if the code is reworded, the arm fails on the code side and names
// the sentence, rather than failing on the page and inviting someone to
// "fix" the page to match a fragment that is itself now stale.
func TestTheRunbookQuotesTheSentencesTheCodeActuallyEmits(t *testing.T) {
	page := crRunbook(t)

	// The keychain-unreadable sentence, taken from the adapter rather than
	// from a literal: an absolute path that is not there fails to exec, is
	// not a gate refusal, and lands on the class-1 error.
	_, _, unreadable := readStore(keychainStoreAt(filepath.Join(t.TempDir(), "security")))
	if unreadable == nil {
		t.Fatal("the keychain adapter read something at a path that does not exist")
	}

	for _, q := range []crQuote{
		// The whole sentence since rangerhq-pwpx: the class carries its own
		// one-line fix, so that the 80% of this section stands in the error
		// itself whether or not the operator ever reaches this page.
		{"unreadable", unreadable.Error(),
			"keychain item \"Claude Code-credentials\" unreadable — this binary's keychain ACL " +
				"may have been dropped by `make install`; grant access when prompted, or run `claude` once"},

		{"401 stale",
			(&AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized}).Error(),
			"usage endpoint returned 401 Unauthorized: credential stale — run `claude` once to refresh"},

		{"403 wrong kind",
			(&AuthFailure{Status: "403 Forbidden", Code: http.StatusForbidden}).Error(),
			"usage endpoint returned 403 Forbidden: this credential is not entitled to plan windows — " +
				"a setup-token never will be, and this is not a freshness problem"},

		{"429 rate-limited",
			(&RateLimit{Status: "429 Too Many Requests", RetryAfter: time.Hour}).Error(),
			"usage endpoint returned 429 Too Many Requests, retry after 1h00m"},

		{"gate shim refusal",
			(&GateRefusal{Cmd: "security", Rule: "Bash(security:*)"}).Error(),
			"keychain read refused by a posse gate shim: security (deny: Bash(security:*)) — posse's own gate, not a credential outage"},

		// Structural absence: the runbook renders the two variable parts as
		// placeholders, so the fragment is the invariant head.
		{"structural absence",
			(&NoSource{Runtime: "grok", Purpose: CredMeter, GOOS: "darwin", Arm: "a meter adapter for grok"}).Error(),
			"no meter credential source for"},
	} {
		t.Run(q.name, func(t *testing.T) {
			if !strings.Contains(crFlat(q.produced), crFlat(q.fragment)) {
				t.Fatalf("the code no longer says this — the runbook's move-2 row for %s is now wrong.\n  code: %q\n  quoted: %q",
					q.name, q.produced, q.fragment)
			}
			if !strings.Contains(page, crFlat(q.fragment)) {
				t.Errorf("docs/runbooks/credential-rotation.md does not quote the %s sentence: %q", q.name, q.fragment)
			}
		})
	}
}

// The two refusals are the whole of a persona's involvement with `posse
// refresh`, and the runbook prints them so a persona who hits one recognises
// it as the gate working. They are taken by RUNNING the command, not by
// quoting the constant: what a reader will see is what the verb does.
func TestTheRunbookQuotesTheRefusalsRefreshActuallyGives(t *testing.T) {
	page := crRunbook(t)
	a := refreshApp(t)

	t.Setenv(EnvPersona, "developer-3")
	err := a.CmdRefresh(discard{}, opts(RefreshOpts{}, "", nil))
	if err == nil {
		t.Fatal("refresh ran under the persona marker")
	}
	// The marker's VALUE is interpolated, so the fragment stops before it.
	const personaHead = "posse refresh is the operator's hand and nothing else (ADR 0019 D4): refusing under RHQ_PERSONA="
	if !strings.Contains(err.Error(), personaHead) {
		t.Fatalf("the persona refusal was reworded; the runbook quotes the old one.\n  code: %q", err)
	}
	if !strings.Contains(page, crFlat(personaHead)) {
		t.Errorf("the runbook does not quote the persona refusal: %q", personaHead)
	}

	t.Setenv(EnvPersona, "")
	ro := opts(RefreshOpts{}, "", nil)
	ro.tty = func() bool { return false }
	err = a.CmdRefresh(discard{}, ro)
	if err == nil {
		t.Fatal("refresh ran with no terminal")
	}
	if !strings.Contains(page, crFlat(err.Error())) {
		t.Errorf("the runbook does not quote the no-TTY refusal: %q", err)
	}
}

// discard is an io.Writer for a call whose output is not the subject. The
// refused calls above write nothing at all — refresh_test.go pins that
// separately — and this keeps a failure here from being a buffer assertion.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// The runbook tells the operator to run `make verify-credential-paths`, and a
// runbook step that names a target that is not there is read under time
// pressure. The script's own behaviour is pinned by credentialpaths_qa_test.go
// at the repo root; this is only that the door the page points at exists.
func TestTheRunbookOnlyNamesMakeTargetsThatExist(t *testing.T) {
	page := crRunbook(t)
	const target = "make verify-credential-paths"
	if !strings.Contains(page, target) {
		t.Fatalf("the runbook no longer names %q — this arm is measuring nothing", target)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Skipf("no Makefile in this checkout: %v", err)
	}
	if !strings.Contains(string(b), "\nverify-credential-paths:") {
		t.Error("the runbook's first step names a make target the Makefile does not define")
	}
}

// The runbook claims, in move 4, that reading ~/.claude is not fenced: the
// rendered seatbelt profile denies file-write* and nothing else, so the
// preventive half (ranger-base-hw18) has not landed and the sweep is the
// whole control.
//
// That is a claim about code, and it is the one claim on the page with an
// expiry date on it. When hw18 lands this goes red, which is the alarm: the
// page must then stop saying the sweep is the whole control.
func TestTheSeatbeltClaimInTheRunbookIsStillTrue(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "internal", "rhq", "seatbelt.go"))
	if err != nil {
		b, err = os.ReadFile("seatbelt.go")
	}
	if err != nil {
		t.Skipf("seatbelt.go not readable from here: %v", err)
	}
	src := string(b)
	// The positive witness: without it, a file that lost its deny entirely
	// would pass the assertion below by having nothing in it.
	if !strings.Contains(src, "(deny file-write*)") {
		t.Fatal("seatbelt.go no longer renders a file-write* deny — this arm is measuring nothing")
	}
	if strings.Contains(src, "file-read") {
		t.Error("seatbelt.go now mentions file-read: docs/runbooks/credential-rotation.md still tells the operator the profile denies no reads and that the credential-path sweep is the whole control (ranger-base-hw18)")
	}
	page := crRunbook(t)
	if !strings.Contains(page, "grep -n file-read") {
		t.Error("the runbook dropped the file-read claim this arm exists to guard")
	}
}
