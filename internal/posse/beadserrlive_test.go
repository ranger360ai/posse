//go:build posse_arm3

package posse

// A live pin for rangerhq-aas, run against the real bd rather than the fake.
//
//	RHQ_LIVE_BD=1 go test ./internal/rhq -run TestLiveBdRun -v
//
// WHY THIS EXISTS ON TOP OF beadserr_test.go. Every arm of that file drives
// the FAKE bd, whose failure shape we wrote ourselves from a measurement
// taken by hand. That is the right place to pin Bd.run's precedence and the
// parse's narrowness — they are our logic — but it leaves one thing
// unpinned: whether the shape we encoded is still the shape bd has. The day
// bd 0.5x moves that object to stderr, or renames the key, the fake keeps
// reporting the old world and all four arms stay green over a fix that no
// longer fires. So this asks bd.
//
// It pins AGREEMENT, in two steps that must both hold:
//
//  1. real bd still fails a --json verb on STDOUT with stderr EMPTY, and
//  2. Bd.run still carries that sentence out.
//
// Step 1 is the one that earns the file. Without it a green step 2 could
// mean "the fix works" or "bd moved the message to stderr and the old code
// path would have done just as well" — and those are not the same fact.
//
// Env-gated and skipped by default like the other live pins: it shells out
// to the operator's bd, which has a version, a daemon and a cache.
// Everything happens inside one t.TempDir.

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestLiveBdRunKeepsTheReasonRealBdPrintsOnStdout(t *testing.T) {
	t.Parallel()
	bd := liveBd(t)
	repo := liveBeadsRepo(t, bd, "a row, so the fixture is a real graph")
	// bd's own pre-commit hook rewrites the jsonl during that commit, so the
	// fixture can be mid-import at this moment. Settle it the way the other
	// live pins do before asking it anything.
	settleMainCheckout(t, bd, repo)

	// An id no repo can resolve. bd must fail on it — a fixture where the
	// id happens to resolve would make every assertion below vacuous.
	const missing = "aaslive-zzzz"

	// 1. The channel, measured rather than assumed: run bd ourselves with
	// the two streams kept apart, which liveBd's CombinedOutput cannot do.
	cmd := exec.Command("bd", "--no-daemon", "dep", "list", missing, "--json")
	cmd.Dir = repo
	cmd.Env = append(cmd.Environ(), "PATH="+PathOutsideGates(""))
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	runErr := cmd.Run()
	stdout, stderr := out.String(), strings.TrimSpace(errb.String())

	if runErr == nil {
		t.Fatalf("bd resolved %q — the fixture cannot discriminate:\n%s", missing, stdout)
	}
	if stderr != "" {
		t.Fatalf("bd now writes the reason to STDERR (%q) — this verb no longer has "+
			"the shape rangerhq-aas fixed, and internal/posse/herdr_test.go's "+
			"`fake-json-error` is teaching the suite a bd that no longer exists", stderr)
	}
	if got := bdStdoutError([]byte(stdout)); got == "" {
		t.Fatalf("bd's stdout is no longer `{\"error\": <string>}` — the fake and the "+
			"parse both need re-measuring against it:\n%s", stdout)
	}

	// The sentence itself is bd's to word and it is NOT stable between call
	// shapes — measured: our --no-daemon probe above says "resolving <id>:
	// no issue found matching …" while Bd.run's own call says "resolving
	// issue ID <id>: operation failed: failed to resolve ID: no issue found
	// matching …". So pin the part that is bd's ANSWER rather than bd's
	// phrasing, and require the probe to carry it too — an assertion on a
	// substring nobody checked is an assertion on nothing.
	const answer = "no issue found"
	if reason := bdStdoutError([]byte(stdout)); !strings.Contains(reason, answer) ||
		!strings.Contains(reason, missing) {
		t.Fatalf("bd's reason no longer names %q and the id — re-measure this pin "+
			"before trusting it: %q", answer, reason)
	}

	// 2. And Bd.run carries it out. Mutation-checked: with the stdout read
	// deleted from Bd.run this reads "bd dep list … --json: exit status 1",
	// which carries neither.
	_, err := Bd{Bin: "bd"}.DepList(repo, missing)
	if err == nil {
		t.Fatal("a verb that exits non-zero must return an error")
	}
	if !strings.Contains(err.Error(), answer) || !strings.Contains(err.Error(), missing) {
		t.Errorf("the reason real bd printed must survive into the error, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Errorf("with a reason in hand it must not fall back to the exit status: %q", err.Error())
	}
}
