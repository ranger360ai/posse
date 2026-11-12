package posse

// sameCredToken is how a test compares a token the seam handed back with
// the value it planted (ranger-base-3spp4): by fingerprint, never by the
// bytes. CredToken.Reveal has no _test.go caller and must not get one
// (internal/treepins/credtokenreveal_qa_test.go) — a test that held the
// revealed string would be one t.Fatalf("%v") away from printing it, which
// is the whole thing the type exists to make impossible.
//
// Untagged so every arm builds it: the comparisons it serves live in all
// three.
func sameCredToken(tok CredToken, want string) bool {
	return tok.Fingerprint() == newCredToken(want).Fingerprint()
}
