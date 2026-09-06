//go:build posse_arm3

package posse

// ranger-base-h8u0l — the FOURTH state of the keychain read: it did not run.
//
// THE DEFECT THESE PIN. `security` failing to EXECUTE is not an exit code
// and writes no stderr, so gateRefusal (which needs an *exec.ExitError to
// have a stderr at all) could not see it, and the exit-code reader called it
// -1. -1 is not 44, so nothing fell through, and the composite rendered the
// ACL sentence — `keychain item "…" unreadable — this binary's keychain ACL
// may have been dropped by ` + "`make install`" + ` — which is the 2026-08-24
// misdiagnosis byte for byte and the sentence that got plan_guard_blind_max:
// 0 set for hours. An ACL is checked by a binary that RAN; on a binary that
// never ran, that sentence names a cause that cannot be the cause.
//
// It escaped to CI before it had a name: ranger-base-wrdz4, run
// 34050764993, the ubuntu leg, where the shim
// TestQAPlanUsageLogNamesTheGateRefusal plants did not execute and
// plan-usage.log took `cost failed: keychain item "Claude Code-credentials"
// unreadable … [unreadable]`. That pin caught it and is NOT weakened here —
// its own arms are untouched, and the second pin below is its twin for this
// class rather than a relaxation of it.
//
// WHY THE ARMS ARE THESE. A and B are the two ways a binary does not run
// that a test can produce deterministically (absent, and present but not
// executable); the transient fork/exec failure under load that produced the
// CI escape is the same class and has no deterministic fixture. C is the
// control the bead's repro carries: it is what makes A and B a fact about
// the did-not-run path rather than about the harness, and it is the one
// error here that DID come back with an exit status.
//
// Order between the two questions is deliberately NOT claimed: a gate
// refusal is an *exec.ExitError and a read that never ran is not one, so
// the two are mutually exclusive and swapping them changes no answer. What
// C pins is that the widening did not swallow the refusal class.

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// notRunBinaries is A and B as paths: one that is not there, and one that is
// there and cannot be executed.
func notRunBinaries(t *testing.T) (missing, notExecutable string) {
	t.Helper()
	dir := t.TempDir()
	missing = filepath.Join(dir, "security")
	notExecutable = filepath.Join(dir, "noexec")
	if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\nexit 44\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return missing, notExecutable
}

// The exit code the non-executable arm's script would have answered if it
// had run, and the one exit that falls through to the credentials file. It
// is 44 on purpose: if a did-not-run read were ever read for an exit code
// again, THIS is the fixture that would quietly hand back the fallback
// file's token instead of a diagnosis.
const notRunWouldHaveExited = errSecItemNotFound

func TestQAKeychainReadThatNeverRanIsNotAnACLOutage(t *testing.T) {
	t.Parallel()
	missing, notExecutable := notRunBinaries(t)
	shim := gatedSecurityShim(t)

	for _, tc := range []struct {
		name string
		bin  string
		// says is what the operator must be told; never is every other
		// class's move, plus the two words the 08-24 sentence is made of.
		says  []string
		never []string
		check func(t *testing.T, err error)
	}{{
		name: "A: the binary is not there",
		bin:  missing,
		says: []string{
			"was not read", "did not run", "no exit status came back",
			"not a credential condition", "no such file or directory",
		},
		never: []string{
			"unreadable", "keychain ACL", "make install",
			"credential stale", "once to refresh", "not entitled",
		},
		check: func(t *testing.T, err error) {
			var nr *CredReadNotRun
			if !errors.As(err, &nr) {
				t.Fatalf("want *CredReadNotRun, got %T: %v", err, err)
			}
			if nr.Cmd != "security" {
				t.Errorf("the class must name the binary by the word a deny rule spells it with: %q", nr.Cmd)
			}
			if !strings.Contains(nr.Store, "keychain item") {
				t.Errorf("the class must name WHICH read did not run: %q", nr.Store)
			}
		},
	}, {
		name: "B: the binary is there and is not executable",
		bin:  notExecutable,
		says: []string{"was not read", "did not run", "permission denied"},
		never: []string{
			"unreadable", "keychain ACL", "make install",
			"credential stale", "once to refresh", "not entitled",
		},
		check: func(t *testing.T, err error) {
			var nr *CredReadNotRun
			if !errors.As(err, &nr) {
				t.Fatalf("want *CredReadNotRun, got %T: %v", err, err)
			}
		},
	}, {
		// The control, and the whole reason A and B are a fact about the
		// code rather than about this test's fixtures.
		name:  "C: posse's own gate shim, which DID run and DID exit",
		bin:   shim,
		says:  []string{"refused by a posse gate shim", "deny: Bash(security:*)", "not a credential outage"},
		never: []string{"unreadable", "did not run", "make install"},
		check: func(t *testing.T, err error) {
			var g *GateRefusal
			if !errors.As(err, &g) {
				t.Fatalf("want *GateRefusal, got %T: %v", err, err)
			}
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tok, _, err := readStore(keychainStoreAt(tc.bin))
			if err == nil {
				t.Fatalf("no read happened, so there is no token: %q", redact(tok))
			}
			tc.check(t, err)
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
			if strings.Contains(err.Error(), "find-generic-password") {
				t.Errorf("no sentence here quotes the read's argv: %q", err)
			}
		})
	}
}

// The class is not the outage class — asked of the two surfaces that carry
// it, and asserted BOTH ways. A classifier that answered PlanFailUnreadable
// here would put the 08-24 word in the cockpit header and the 08-24 token in
// plan-usage.log, which is the same misdiagnosis one layer up from the
// sentence.
func TestQAReadThatNeverRanIsNotTheUnreadableClass(t *testing.T) {
	t.Parallel()
	missing, _ := notRunBinaries(t)
	_, _, notRun := readStore(keychainStoreAt(missing))
	// The unreadable class as production makes it: a `security` that RAN
	// and exited non-zero on something that is not 44, which is ADR 0019
	// D2's own row and needs no fallback file to be one.
	_, _, unreadable := readStore(keychainStoreAt(keychainStub(t, "#!/bin/sh\nexit 36\n")))
	if notRun == nil || unreadable == nil {
		t.Fatalf("both rows are failures: %v / %v", notRun, unreadable)
	}

	if got := PlanFailureOf(notRun); got != PlanFailNotRun {
		t.Errorf("PlanFailureOf(not-run) = %q, want %q", got, PlanFailNotRun)
	}
	if got := PlanFailureOf(unreadable); got != PlanFailUnreadable {
		t.Errorf("PlanFailureOf(unreadable) = %q, want %q — the control moved", got, PlanFailUnreadable)
	}
	if got := PlanFailToken(notRun); got != "not-run" {
		t.Errorf("PlanFailToken(not-run) = %q, want %q", got, "not-run")
	}
	if got := PlanFailToken(unreadable); got != "unreadable" {
		t.Errorf("PlanFailToken(unreadable) = %q, want %q — the control moved", got, "unreadable")
	}
	if notRun.Error() == unreadable.Error() {
		t.Errorf("two classes, one sentence: %q", notRun)
	}
	// And the type: errors.As is how every reader here asks, so a not-run
	// that also satisfied *CredUnreadable would answer both questions yes.
	var cu *CredUnreadable
	if errors.As(notRun, &cu) {
		t.Errorf("a read that never ran is not the unreadable class: %v", cu)
	}
}

// The fall-through must not open, and this is the fixture that would catch
// it opening: a credentials file sitting there with a token in it, and a
// keychain read that never ran. Falling back here would answer "monitoring
// is broken on this box" with another store's credential — and would do it
// silently, which is the one thing ADR 0019 D2 narrows the fall-through to
// exit 44 to prevent.
func TestQAReadThatNeverRanDoesNotFallThroughToTheFile(t *testing.T) {
	// No t.Parallel: plantFallback names a config directory (t.Setenv).
	plantFallback(t, envelope(fallbackOnlyToken, time.Now().Add(90*24*time.Hour).UnixMilli()))
	missing, notExecutable := notRunBinaries(t)
	for name, bin := range map[string]string{"missing": missing, "not executable": notExecutable} {
		t.Run(name, func(t *testing.T) {
			tok, _, err := readStore(keychainStoreAt(bin))
			if err == nil {
				t.Fatalf("the file was read on a keychain that never answered: %q", redact(tok))
			}
			if tok == fallbackOnlyToken {
				t.Fatalf("the fallback file's token came back from a read that never ran")
			}
			var nr *CredReadNotRun
			if !errors.As(err, &nr) {
				t.Fatalf("want *CredReadNotRun, got %T: %v", err, err)
			}
			if strings.Contains(err.Error(), credentialsFileFallback) {
				t.Errorf("nothing here reached the second store: %q", err)
			}
		})
	}
	// The exit that DOES fall through, over the same planted file, so the
	// row above is a statement about the not-run class and not about a
	// fall-through that stopped working for everybody.
	t.Run("control: exit 44 still falls through", func(t *testing.T) {
		tok, _, err := readStore(keychainStoreAt(keychainStub(t,
			"#!/bin/sh\nexit "+strconv.Itoa(notRunWouldHaveExited)+"\n")))
		if err != nil {
			t.Fatalf("44 with a file beside it is a credential: %v", err)
		}
		if tok != fallbackOnlyToken {
			t.Errorf("token = %q, want the fallback file's", redact(tok))
		}
	})
}

// plan-usage.log is where the 08-24 misdiagnosis was actually READ, and this
// is TestQAPlanUsageLogNamesTheGateRefusal's twin for the class that pin was
// firing on when it caught this (ranger-base-wrdz4, ubuntu leg). The log is
// the operator's record after the process that printed the line is gone, so
// the class has to survive into it — as the sentence AND as the token a
// streak reader counts hours later without reading the sentence.
func TestQAPlanUsageLogNamesAReadThatNeverRan(t *testing.T) {
	t.Parallel()
	missing, _ := notRunBinaries(t)
	home := t.TempDir()
	c := &PlanCache{
		Path:   filepath.Join(home, "plan-usage.json"),
		Log:    filepath.Join(home, "plan-usage.log"),
		Caller: "cost",
		// The compiled-in endpoint, never dialled: the token is asked for
		// first, so the failure is the credential read and not transport.
		Reader: &AnthropicPlanReader{URL: PlanUsageURL, Token: keychainTokenAt(missing)},
	}
	if _, _, err := c.Read(time.Hour); err == nil {
		t.Fatal("a read that never ran is not a reading")
	}
	logb, err := os.ReadFile(c.Log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(logb)
	if !strings.Contains(got, "cost failed: ") || !strings.Contains(got, "did not run") {
		t.Errorf("the log line must say the read did not run: %q", got)
	}
	if !strings.Contains(got, "[not-run]") {
		t.Errorf("the log line must carry the class as a token: %q", got)
	}
	if strings.Contains(got, "unreadable") {
		t.Errorf("the 08-24 misdiagnosis, byte for byte — sentence or token: %q", got)
	}
	if strings.Contains(got, "make install") {
		t.Errorf("an ACL is checked by a binary that RAN: %q", got)
	}
	if strings.Contains(got, "find-generic-password") {
		t.Errorf("the log must not quote the command's argv: %q", got)
	}
}

// The unattended park line — the 08-24 harm path itself, and the twin of
// TestQAUnattendedBlindParkNamesOurGateNotAnOutage. This is the line the
// operator reads the next morning, and on 2026-08-24 reading it as a
// credential outage is what switched the shop's only automated brake off.
func TestQAUnattendedBlindParkNamesAReadThatNeverRan(t *testing.T) {
	missing, _ := notRunBinaries(t)
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	keychainOnly(planReaderOf(r.d), keychainTokenAt(missing))
	r.at(12 * time.Minute)

	if n := r.run(t); n != 0 {
		t.Fatalf("past the blind budget the pass must not dispatch: %d\n%s", n, r.out())
	}
	got := r.out()
	if !strings.Contains(got, "blind 12m") || !strings.Contains(got, "did not run") {
		t.Errorf("the park line must name the age and that the read never happened:\n%s", got)
	}
	if strings.Contains(got, "unreadable") || strings.Contains(got, "make install") {
		t.Errorf("the park line must not read as a credential outage:\n%s", got)
	}
	if strings.Contains(got, "find-generic-password") {
		t.Errorf("the park line must not quote the command's argv:\n%s", got)
	}
}
