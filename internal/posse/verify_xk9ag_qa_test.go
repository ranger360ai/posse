//go:build !posse_arm2 && !posse_arm3

package posse

// TWO PINS ADDED VERIFYING ranger-base-xk9ag, one per close whose
// mechanism rests on a claim nothing in the suite was asking of it. Neither
// is a defect: both closes verified. They are the mutants that survived.
//
//   - ranger-base-gyrnp: every "each drift lands fail-closed" sentence in
//     that close and in ADR 0050 D2 rests on ONE measurement — an empty
//     pattern list matches nothing, so `grep -vxFf` leaves the whole block
//     below the cut on the scan. Its eight pins all stage a file, so the
//     empty reference is never the reference. `--allow-empty` is the
//     reachable spelling of it.
//   - ranger-base-07ep: rule 3 is EXACT hostname equality. Loosening
//     answeredHost to `strings.HasSuffix(h, asked)` — the classic near-miss,
//     `evil-api.anthropic.com` for `api.anthropic.com` — survived all ten of
//     that close's redirect pins under `go test -overlay`, because every one
//     of them redirects to loopback or to `listener.example`, and neither is
//     a near miss of the other.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ranger-base-gyrnp's drift claim, taken at the one spelling a writer can
// reach without config: nothing staged, so `git diff --cached` is empty and
// the reference is an empty pattern list. The forgery that bead was filed
// about must still be refused. MUTATION: the arm reds if `grep -vxFf` with
// an empty list is ever the other way round on some grep — which is the
// portability assumption the whole fail-closed argument is carried on.
func TestQAMessageArmCutWithAnEmptyReferenceStillReadsBelowIt(t *testing.T) {
	w := qaCeilingWall(t, "")

	// CONTROL first: the wall is awake in this repo at this moment, so a
	// refusal below is this arm's and not an empty fixture's. It leaves the
	// index as it found it (qaVerboseWallIsAwake unstages its own subject,
	// which a refused commit leaves staged — ranger-base-uzgkz).
	qaVerboseWallIsAwake(t, w, w.persona, "internal/posse/awake.go")

	// PREMISE, both halves: there is a HEAD to name a path in — the commit
	// stays QUALIFIED, so the unqualified-commit gate is never the thing
	// that spoke — and nothing is staged against it, so the reference the
	// arm subtracts is empty. `--allow-empty` is what lets a commit name a
	// path it is not changing.
	const rel = "internal/posse/base.go"
	w.plant(t, w.priv, rel, "package posse\n")
	if staged, err := w.git(w.priv, nil, "diff", "--cached", "--name-only", "HEAD"); err != nil || strings.TrimSpace(staged) != "" {
		t.Fatalf("premise: nothing may be staged, or the reference is not empty (%v): %q", err, staged)
	}

	msg := filepath.Join(t.TempDir(), "msg")
	write(t, msg, qaForgedCutMessage())
	out, err := w.git(w.priv, w.persona, "commit", "--allow-empty", "-F", msg, "--", rel)
	if err == nil {
		t.Fatalf("LANDED: with an EMPTY reference the whole block below the cut must stay on the scan "+
			"(ranger-base-gyrnp: an empty pattern list matches nothing, so -v prints every line). "+
			"It did not, so an empty reference licenses the forged cut:\n%s", out)
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("refused, but not by the MESSAGE arm — the drift case must fail closed HERE, and "+
			"in this arm's own words (%v):\n%s", err, out)
	}
	if !strings.Contains(w.log(t), "data ceiling scan [prepare-commit-msg hook] (stamp: "+VisibilityPrivate+", commit message)") {
		t.Errorf("refusals.log must carry the message line:\n%s", w.log(t))
	}
}

// ranger-base-07ep's rule 3 is EQUALITY, not containment. The near miss is
// the shape a redirect off a compromised upstream would actually take, and
// it is the one shape none of that close's pins can tell from the real host.
//
// Both directions, because a suffix test and a prefix test fail differently:
// `evil-api.anthropic.com` ends with the asked host, `api.anthropic.com.evil`
// begins with it, and a bare `Contains` takes both.
func TestQAAnAnswerFromANearMissOfTheAskedHostIsNotAnAnswer(t *testing.T) {
	t.Parallel()
	const asked = "api.anthropic.com"

	// CONTROL first: the host actually asked IS an answer, in the same call
	// shape as every row below. Without it a rule that refused everything
	// would read identically here.
	if !answeredHost(mustURL(t, "https://"+asked+"/v1/models"), asked) {
		t.Fatalf("control: the asked host must answer, or every row below is true of nothing")
	}
	if !answeredHost(mustURL(t, "https://API.ANTHROPIC.COM/v1/models"), asked) {
		t.Errorf("control: a hostname differing only in case is the same host")
	}

	for _, u := range []string{
		"https://evil-api.anthropic.com/v1/models", // suffix of the asked host
		"https://api.anthropic.com.evil.test/v1/models",
		"https://xapi.anthropic.com/v1/models",
		"https://api.anthropic.co/v1/models", // prefix of the asked host
		"https://anthropic.com/v1/models",
		"https://api.anthropic.com./v1/models", // the DNS root-dot spelling
	} {
		if answeredHost(mustURL(t, u), asked) {
			t.Errorf("rule 3 is equality: %q must not answer for %q — a near miss is what an "+
				"attacker holding the network path would actually send (ranger-base-07ep)", u, asked)
		}
	}

	// And through the belt, not only the predicate: the pinned client must
	// not FOLLOW a redirect to a near miss either.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 127.0.0.1 is the asked host here; 127.0.0.11 has it as a prefix.
		http.Redirect(w, r, strings.Replace(target.URL, "127.0.0.1:", "127.0.0.11:", 1), http.StatusFound)
	}))
	defer src.Close()
	req, err := http.NewRequest("GET", src.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pinnedClient(5*time.Second, "model list endpoint").Do(req); err == nil {
		t.Errorf("the pinned client followed a redirect to a hostname that merely has the asked one as a prefix")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
