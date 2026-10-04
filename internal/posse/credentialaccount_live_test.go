//go:build posse_arm3

package posse

// Live pin for ranger-base-tghn5, the one thing about the keychain account
// that cannot be learned by reading:
//
//	RHQ_LIVE_KEYCHAIN=1 go test -tags posse_arm3 ./internal/posse \
//	  -run TestLiveKeychainReadAnswersUnderTheRuntimesAccount -v
//
// EVERY OTHER PIN in this package stubs `security`, deliberately: reading the
// operator's live keychain in a test is the class of live-state read this
// package has been burned by, and it needs a box that is logged in, which CI
// is not and a caged session is not. So this one is opt-in, it is the only
// test here that execs the real /usr/bin/security, and it is the whole of
// ADR 0019 D2's "verified by execution, not by reading" for the account.
//
// WHY IT EXISTS. keychainCmd named the SERVICE alone until 2026-09-28, and a
// generic password is identified by service AND account. The account posse
// now sends is derived from the runtime's own rule, transcribed off the
// shipped bundle (keychainAccount's header) — which is a reading of the
// PROGRAM. What no reading can settle is whether the item ON THIS BOX
// actually carries that account, and the cost of being wrong is specific:
// `security` answers 44, the composite falls through, and the operator is
// handed a sentence that opens "it really is gone" about an item sitting
// right there. This test is the one act that distinguishes those.
//
// IT NEVER SEES A VALUE. The token is read into the seam and dropped
// unlooked-at — the only thing asserted about it is that it is not empty,
// which is what "the read worked" means, and the only thing logged is the
// subject posse asked for and the expiry the envelope carried. That is this
// file's standing rule and the reason the length is not logged either.
//
// A SKIP IS NOT A PASS. An unset RHQ_LIVE_KEYCHAIN skips, a non-darwin box
// skips, and neither says anything about the account — the bead that closes
// on this pin closes on a run that printed the subject.
//
// AND IT PUTS $HOME BACK ON THE CHILD, which is the one line that makes any
// of the above reachable (ranger-base-ryg9w). `security` locates the login
// keychain through HOME, and this package's TestMain replaces HOME with an
// empty temp dir for the whole binary — deliberately, so no test cuts a
// worktree in the operator's live ~/.posse (ranger-base-gvrh). MEASURED
// 2026-09-28 on the logged-in box (ranger-base-ryg9w): the identical argv
// exits 0 from a shell and from a throwaway Go program, and exits 44 with
// HOME=$(mktemp -d). So until this line existed the pin asked the right
// question of the wrong keychain and could never pass — it was not the
// account being wrong, which was the finding.
//
// It is put back on the CHILD and never on this process: os.Setenv/t.Setenv
// would hand the operator's live home to every test running in parallel
// beside this one, which is the exact property TestMain bought.
//
// WHAT IT STILL CANNOT REACH is the other half of the subject, the ITEM.
// keychainItem reads CLAUDE_CONFIG_DIR in THIS process, and TestMain clears
// that too, so the name derived here is the default spelling whatever the
// shell exported. On a box where the runtime logged in under a config-dir
// variable the item carries ig4op's 8-hex suffix, this pin asks the
// unsuffixed name, and the answer is a 44 whose sentence sends the operator
// to Keychain Access — the right place, for a reason the sentence does not
// name. The item derivation is ranger-base-ig4op's pin and is MEASURED off
// the bundle; this one is the account's.

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLiveKeychainReadAnswersUnderTheRuntimesAccount(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_KEYCHAIN") == "" {
		t.Skip("set RHQ_LIVE_KEYCHAIN=1 (execs the real /usr/bin/security against this box's live keychain)")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("the keychain store is darwin's (ADR 0019 D2); off darwin the store of record is the credentials file")
	}

	store := liveKeychainStoreAt(t, securityBin)
	t.Logf("asking: %s", store.Name)

	tok, meta, err := readStore(store)
	if err != nil {
		// Named in full, because WHICH failure it is is the finding. A 44
		// here is the account being wrong — or the item being genuinely
		// gone, which is why the sentence now carries both.
		t.Fatalf("the live read failed: %v\n\nIf this is an item-not-found, open %q in Keychain Access and compare its Account field against the one above: that is the pair ranger-base-tghn5 is about, and a mismatch is this change being wrong rather than the credential being absent.", err, KeychainService)
	}
	if tok == "" {
		t.Fatal("the read succeeded and handed back an empty token — the item answered under this account and holds no credential, which is a login state and not an account mismatch; run `claude` and `/login`, then run this again")
	}
	// The KEYCHAIN answered, not the file below it. Without this the pin
	// goes green on a box where the account is wrong, the item answers 44,
	// and the composite quietly reads the fallback credentials file — which
	// is exactly the failure this test exists to catch (ADR 0019 D2 V9).
	if !strings.Contains(meta.Source, "keychain item") {
		t.Fatalf("the read fell through to %q — the keychain did not answer under this account, and a token from the store BELOW it is not this pin's subject", meta.Source)
	}
	if !strings.Contains(meta.Source, keychainAccount()) {
		t.Errorf("the source is %q and does not name the account asked for — the sentence an operator reads on a failure would not say which account answered nothing", meta.Source)
	}
	t.Logf("the live keychain answered under this account; envelope expiry %v", meta.ExpiresAt)
}

// liveKeychainStoreAt is posse's own keychain adapter with exactly one thing
// changed: the child's $HOME. Both the pin above and the guard below go
// through it, so the thing the guard proves is the thing the pin runs.
//
// Everything else is production's — keychainItem and keychainAccount derive
// the subject, keychainCmd builds the argv, keychainRun classifies the
// failure, readStore parses the envelope and picks the sentence. The Read
// closure is replaced rather than the process's environment because
// os.Setenv/t.Setenv here would hand the operator's live home to every test
// running in parallel beside this one.
func liveKeychainStoreAt(t *testing.T, bin string) runtimeStore {
	t.Helper()
	if operatorHome == "" {
		t.Fatal("TestMain recorded no operator $HOME, so there is no login keychain to point `security` at — this pin cannot run from a binary started without HOME")
	}
	item := keychainItem()
	account := keychainAccount()
	subject := keychainSubject(item, account)
	store := keychainStoreAt(bin)
	store.Read = func() ([]byte, error) {
		cmd := keychainCmd(bin, item, account)
		cmd.Env = append(os.Environ(), "HOME="+operatorHome)
		return keychainRun(bin, subject, cmd)
	}
	return store
}

// The guard for the pin, and the one test in this file that always runs: it
// asks a STUB `security` what $HOME it was handed, so it needs no live
// keychain, no login and no darwin.
//
// It exists because the defect it pins was invisible (ranger-base-ryg9w).
// The pin above read correctly, named the right account and failed 100% of
// the time, and every signal pointed at the account rather than at the
// binary's temp HOME; the first reading of the failure was "ranger-base-tghn5
// is wrong on this box". A pin the operator can only run by hand, on a box
// nobody else has, is exactly the kind that can rot unobserved — so the
// thing it depends on is pinned by a test the suite runs.
func TestQALiveKeychainReadHandsTheChildTheOperatorHome(t *testing.T) {
	t.Parallel()
	// Reports $HOME as the token, so the assert reads the child's own view.
	stub := keychainStub(t, "#!/bin/sh\ncat <<JSON\n"+envelope("$HOME", time.Now().Add(time.Hour).UnixMilli())+"\nJSON\n")

	tok, _, err := readStore(liveKeychainStoreAt(t, stub))
	if err != nil {
		t.Fatalf("the stubbed read must answer: %v", err)
	}
	if tok != operatorHome {
		t.Errorf("the `security` child was handed HOME=%q, want the operator's %q — with the package's temp HOME the real binary answers 44 on a keychain it cannot see, and the pin above can never pass", tok, operatorHome)
	}
	// And the temp HOME really is different, or the assert above is vacuous.
	if operatorHome == os.Getenv("HOME") {
		t.Errorf("TestMain did not replace HOME (%q) — this guard proves nothing while the two are equal", operatorHome)
	}
}
