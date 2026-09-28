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

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestLiveKeychainReadAnswersUnderTheRuntimesAccount(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_KEYCHAIN") == "" {
		t.Skip("set RHQ_LIVE_KEYCHAIN=1 (execs the real /usr/bin/security against this box's live keychain)")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("the keychain store is darwin's (ADR 0019 D2); off darwin the store of record is the credentials file")
	}

	store := keychainStore()
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
