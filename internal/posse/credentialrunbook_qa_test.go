//go:build posse_arm2

package posse

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
	"strconv"
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
	// pageAs is that same sentence as the PAGE spells it, for the one row
	// where the two can honestly differ. The keychain item's name is derived
	// from this process's environment (ADR 0019 D2 store 1,
	// ranger-base-mx4q6), so the code side must be checked against the name
	// THIS box would ask for, while the page quotes the default spelling —
	// the name on a box with no config-dir variable set, and the one a reader
	// matches the row by. Empty means the page quotes the fragment verbatim,
	// which is every other row.
	pageAs string
}

func (q crQuote) quoted() string {
	if q.pageAs != "" {
		return q.pageAs
	}
	return q.fragment
}

// TestTheRunbookQuotesTheSentencesTheCodeActuallyEmits is the drift pin.
//
// Every fragment is checked against production FIRST. That ordering is the
// point: if the code is reworded, the arm fails on the code side and names
// the sentence, rather than failing on the page and inviting someone to
// "fix" the page to match a fragment that is itself now stale.
func TestTheRunbookQuotesTheSentencesTheCodeActuallyEmits(t *testing.T) {
	t.Parallel()
	page := crRunbook(t)

	// The keychain-unreadable sentence, taken from the adapter rather than
	// from a literal: a `security` that RAN and exited non-zero on something
	// that is not 44, which is ADR 0019 D2's own row for this class.
	//
	// It used to be an absolute path that is not there — which does not
	// exec at all, and so was never this class: until 2026-09-06 the two
	// were one error, and this arm was quoting the ACL row's sentence at a
	// fixture that had never asked the keychain anything (ranger-base-h8u0l,
	// and it is the shape the runbook's own last column now warns about).
	// The read that never runs is the row below, from its own fixture.
	_, _, unreadable := readStore(keychainStoreAt(keychainStub(t, "#!/bin/sh\nexit 36\n")))
	if unreadable == nil {
		t.Fatal("a `security` that exits 36 is not a credential")
	}
	_, _, notRun := readStore(keychainStoreAt(filepath.Join(t.TempDir(), "security")))
	if notRun == nil {
		t.Fatal("the keychain adapter read something at a path that does not exist")
	}

	// The item name is the environment's and not the constant's: a suite run
	// on a box that sets a config-dir variable reads a suffixed item and says
	// so (ranger-base-mx4q6). This arm measures the sentence THAT box gets.
	item, _ := keychainItem()
	const unreadableTail = " unreadable — this binary's keychain ACL " +
		"may have been dropped by `make install`; grant access when prompted, or run `claude` once"

	for _, q := range []crQuote{
		// The whole sentence since rangerhq-pwpx: the class carries its own
		// one-line fix, so that the 80% of this section stands in the error
		// itself whether or not the operator ever reaches this page.
		{name: "unreadable", produced: unreadable.Error(),
			fragment: "keychain item " + strconv.Quote(item) + unreadableTail,
			pageAs:   "keychain item " + strconv.Quote(KeychainService) + unreadableTail},

		// The read that never ran, in two fragments: the OS's own reason
		// sits between them and names a temp path, so the page renders it
		// as a placeholder and the quotable parts are the head an operator
		// matches the row by and the tail that carries the move.
		{name: "did not run (head)", produced: notRun.Error(),
			fragment: "keychain item " + strconv.Quote(item) + " was not read: security did not run (",
			pageAs:   "keychain item " + strconv.Quote(KeychainService) + " was not read: security did not run ("},

		{name: "did not run (tail)", produced: notRun.Error(),
			fragment: "— no exit status came back, so nothing was learned about the store; " +
				"that is a fault on this box and not a credential condition — check the binary " +
				"is present and executable, and under load it is a transient fork/exec failure the next read answers"},

		{name: "401 stale",
			produced: (&AuthFailure{Status: "401 Unauthorized", Code: http.StatusUnauthorized}).Error(),
			fragment: "usage endpoint returned 401 Unauthorized: credential stale — run `claude` once to refresh"},

		{name: "403 wrong kind",
			produced: (&AuthFailure{Status: "403 Forbidden", Code: http.StatusForbidden}).Error(),
			fragment: "usage endpoint returned 403 Forbidden: this credential is not entitled to plan windows — " +
				"a setup-token never will be, and this is not a freshness problem"},

		{name: "429 rate-limited",
			produced: (&RateLimit{Status: "429 Too Many Requests", RetryAfter: time.Hour}).Error(),
			fragment: "usage endpoint returned 429 Too Many Requests, retry after 1h00m"},

		{name: "gate shim refusal",
			produced: (&GateRefusal{Cmd: "security", Rule: "Bash(security:*)"}).Error(),
			fragment: "keychain read refused by a posse gate shim: security (deny: Bash(security:*)) — posse's own gate, not a credential outage"},

		// Structural absence: the runbook renders the two variable parts as
		// placeholders, so the fragment is the invariant head.
		{name: "structural absence",
			produced: (&NoSource{Runtime: "grok", Purpose: CredMeter, GOOS: "darwin", Arm: "a meter adapter for grok"}).Error(),
			fragment: "no meter credential source for"},
	} {
		t.Run(q.name, func(t *testing.T) {
			if !strings.Contains(crFlat(q.produced), crFlat(q.fragment)) {
				t.Fatalf("the code no longer says this — the runbook's move-2 row for %s is now wrong.\n  code: %q\n  quoted: %q",
					q.name, q.produced, q.fragment)
			}
			if !strings.Contains(page, crFlat(q.quoted())) {
				t.Errorf("docs/runbooks/credential-rotation.md does not quote the %s sentence: %q", q.name, q.quoted())
			}
		})
	}
}

// The runbook's item-name sentence, both ways (ADR 0019 D2 store 1 / V12,
// ranger-base-mx4q6).
//
// The row above quotes the DEFAULT spelling, and it has to keep doing that:
// it is the name on a box with no config-dir variable, which is the reference
// box today. What it must not do is leave a reader on a box that sets one
// hunting Keychain Access for an item that is not there — so the page also
// says when the name grows a suffix, and this holds that claim against the
// derivation rather than against a literal.
//
// Not t.Parallel: it sets the environment the derivation reads.
func TestTheRunbookSaysWhenTheKeychainItemNameGrowsASuffix(t *testing.T) {
	page := crRunbook(t)

	// The page keeps the default spelling.
	if !strings.Contains(page, crFlat(strconv.Quote(KeychainService))) &&
		!strings.Contains(page, crFlat("`"+KeychainService+"`")) {
		t.Fatalf("the runbook no longer names the item's default spelling %q — every row of move 2 is matched by that name", KeychainService)
	}

	// And it names both variables that grow the suffix, because "a config
	// directory is set" is not something an operator can check without them.
	for _, v := range []string{"CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CONFIG_DIR"} {
		if !strings.Contains(page, v) {
			t.Errorf("the runbook does not name %s — the reader cannot tell whether their item is suffixed", v)
		}
	}
	if !strings.Contains(page, KeychainService+"-") {
		t.Error("the runbook does not show the suffixed spelling, so a reader on a box with a config-dir variable set matches nothing in Keychain Access")
	}

	// The code half: under a set config-dir variable the derived name really
	// is the default spelling plus `-` and eight hex digits, so the sentence
	// above describes this binary and not a plan.
	t.Setenv("HOME", "/tmp/home")
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/cfg")
	name, _ := keychainItem()
	if name != KeychainService+"-519e587f" {
		t.Fatalf("keychainItem() = %q under a set CLAUDE_CONFIG_DIR — the runbook's suffix sentence describes something this code does not do", name)
	}
	suffix := strings.TrimPrefix(name, KeychainService+"-")
	if len(suffix) != 8 {
		t.Errorf("the suffix is %d hex digits, the page says eight", len(suffix))
	}
	// The control: with no variable set the page's "default spelling" claim
	// has to be true too, or the row above is quoting a name nothing emits.
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")
	if name, _ := keychainItem(); name != KeychainService {
		t.Errorf("with neither variable set the item is %q — the runbook's rows all quote %q", name, KeychainService)
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

	t.Setenv(EnvPersona, "") // the arm switch: the persona refusal is above
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
	t.Parallel()
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

// The runbook USED TO claim, in move 4, that reading ~/.claude was not
// fenced at all: the rendered seatbelt profile denied file-write* and
// nothing else, so the sweep was the whole control. ranger-base-hw18 closed
// that read half — this pin flipped with it (it was the alarm that told a
// future session to make this edit) and now watches the opposite claim: a
// revert of the deny, or a runbook that reverts to the old wording, trips
// it either way.
func TestTheSeatbeltClaimInTheRunbookIsStillTrue(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "internal", "posse", "seatbelt.go"))
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
	if !strings.Contains(src, "(deny file-read*") {
		t.Fatal("seatbelt.go no longer renders a file-read* deny — ranger-base-hw18's credential read-deny is gone; the runbook's closed-read-half wording would then be false")
	}
	page := crRunbook(t)
	if strings.Contains(page, "grep -n file-read") {
		t.Error("the runbook still tells the operator the profile denies no reads — ranger-base-hw18 landed and closed that half")
	}
	if !strings.Contains(page, "ranger-base-hw18") {
		t.Error("the runbook dropped the pointer to the bead that closed the read half")
	}
}
