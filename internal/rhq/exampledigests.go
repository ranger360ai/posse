package rhq

// What `posse init` is allowed to take back out of agents/ (ranger-base-rgx0).
//
// Retiring a generic is reclamation of a file in the operator's own
// directory, so it needs PROOF that the file is posse's, not an inference
// from a record the home wrote about itself. The proof has to survive
// version skew: the example PIDs are shipped prose and they change (95c4b70
// added `- Bash(posse promote:*)` to the deny list of all nine), so a home
// seeded by an earlier posse holds bytes the running binary no longer ships
// — judged against the embed alone, every such home retires nothing
// (ranger-base-8ehw) and init blames the operator for edits they never made.
//
// The seeded promote manifest looked like the record that answers it and is
// not: SeedPromoteManifest writes a manifest whenever a home has none,
// hashing whatever is on disk AT THAT MOMENT, upgrades included. On a home
// that predates ADR 0015 with a generic the operator had adopted in place,
// that manifest attests to THEIR file, and "live == manifest" reads as
// untouched-since-seeded. Two inits were enough to take an adopted persona
// out of routing, the first one writing the record that blessed the second.
//
// So the question is answered from posse's side of the line instead: this
// table is every sha256 posse has ever shipped for each example PID. A live
// file that matches one IS an example posse laid down, whichever release did
// it, and nothing an operator wrote can match by accident. A file that
// matches none is theirs — including an example they edited, which is the
// design rule ranger-base-qajs set and 8b3h verified.
//
// It fails in the safe direction. A version missing from the table retires
// nothing (the ranger-base-8ehw leak, not a lost persona), and
// TestEveryEmbeddedExamplePIDIsInTheShippedTable turns the next change to
// examples/agents into a red suite that says which line to add.
//
// THE CONTRACT WHEN YOU CHANGE AN EXAMPLE PID: append the new digest, never
// replace one. An entry that leaves this table is a home posse can no longer
// recognise its own file in.

import (
	"crypto/sha256"
	"encoding/hex"
)

// shippedExampleDigests maps a seed-relative path to the sha256 of every
// version of that file posse has shipped, oldest first. Generated from this
// repo's history over examples/agents (the seed tree has never lived
// anywhere else — the pre-publication constitution was a symlinked home, not
// a seed), and checked against it by
// TestShippedExampleTableCoversEveryVersionInGitHistory.
var shippedExampleDigests = map[string][]string{
	"agents/architect.md": {
		"16171774e3e2af7b3b5b4b482ce6059e9b283e4fc9d852e636bd6637207be631", // 5668b76 2026-08-23 posse: initial publication
		"f637586f5dd5cfaf1531d31820d66e2d991d871d843b4cba1ce6c9ebad378d16", // 95c4b70 2026-08-26 feat: posse promote
		"e40cb8c323e040d91b730d537516c45436d5be115c3ee89f89d7e97015d47975", // rangerhq-icb3 2026-08-27 skills: [distributed-systems]
	},
	"agents/business-manager.md": {
		"dac2a2a52ab880671783c6bf5a2a4559144abcf3e57ff5b6567144434367adbe", // 5668b76
		"73cf5f25a546d1c15361d03bf4cb085d016744fbd8e135a86e2869dd3b6153ed", // 95c4b70
	},
	"agents/developer.md": {
		"17e0db0cf3780cb6ba6e0ecd0b13d300f1b45d05b470be6df4d43575b43348de", // 5668b76
		"f8ed74a6ffe8b6a3e536c3643def024ae44afadfcc23d55b23e8119fba233fa9", // 95c4b70
		"2acfe22434656eddb0e93cf86458ba613a294e906896e56a5ea82564858b21f9", // rangerhq-icb3 2026-08-27 skills: [distributed-systems]
	},
	"agents/devops.md": {
		"a5882633fdf059352d0dfc1dc13386bd7488ff0f4fff56464730e439ff7b9d6c", // 5668b76
		"71ce1ba98dbad596928ba25efb6c64edcff957ef955c6c195c67f39655b53068", // 95c4b70
	},
	"agents/product.md": {
		"c9ce6781c6f3b0d3049ff424815993983fb04419272a17ec6ca8328877f426d6", // 5668b76
		"519f25ba83ee16e2b36a58d3d73683361b6fcf2e70b5c57641d3ef921b9e2597", // 95c4b70
	},
	"agents/qa.md": {
		"65eeb0b68cda30bb9ed944c9789c4bd26d5e75f3b69a7d85f5c8eab7e4da832a", // 5668b76
		"707141288b153c8b8eb7b55afce38d18fbbe9914e130ced268a37f6d527256c7", // 95c4b70
	},
	"agents/ranger.md": {
		"a2d154e05cce58b1555ad6ade9d8b828b595d4ee958c5e9c3a0228221fc72b1d", // 5668b76
		"f407345cd3142cdd9177afe209848616baf82fa0171fb53cb8fa7d4bef9f86df", // 95c4b70
	},
	"agents/reviewer.md": {
		"88654f9f35ce94cc2a797cac6dead8ec6540ad0de2449fca0c79a90dfe5e6064", // 5668b76
		"9eee902953c0b567fe9cc0ad5d9f5a1ddd0cdf3d91589711f0ff563465a826d1", // 95c4b70
	},
	"agents/security.md": {
		"d3d07f404ab3099e93525374c8ae94dbfb12f21ef20434d3777a295c050ad8be", // 5668b76
		"818ccb7d14223e9d58cde93af06813a6020d369d7bb561bd95bdcbd8073728ea", // 95c4b70
	},
}

// sha256Bytes is the digest form the table is written in.
func sha256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// isShippedExample reports whether b is byte-for-byte a version of the seed
// file rel that posse itself shipped. A path posse ships no example for, or
// bytes matching no release, is not posse's to reclaim.
func isShippedExample(rel string, b []byte) bool {
	sums := shippedExampleDigests[rel]
	if len(sums) == 0 {
		return false
	}
	got := sha256Bytes(b)
	for _, s := range sums {
		if s == got {
			return true
		}
	}
	return false
}
