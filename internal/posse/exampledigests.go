package posse

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
// recognise its own file in. RENAMING one is the same rule read twice: the
// old path keeps every digest it had — homes hold that file under that name —
// and the new path starts its own list. That is why retireExamplePIDs walks
// this table's keys and not the embed alone: a name posse has stopped
// shipping is still a name posse laid down, and dropping it from the walk
// would leave the generic routing beads on every home that has it.

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
		"7dd0f1ae0c6c1b0997360f113ac063d866c97cd26f6a99bea54142a1a573901a", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"dd66d0842310140ad27835b837ab4dd20bee835be6093ec72843da04bef99d0e", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
		"b4144f893833d1fb2553308b6238f3472f2ef0278252f0692a78bcf3fbfd5e39", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"b0f690a30d5c5b781899e9f27c79aa6832ad83b3afe308ce6f6bd42f00f0aa95", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"00a402f48c63cd2ca274187bea2e888990434b9d59dea480f3183efe6a803b43", // ranger-base-ccd 2026-08-29 path-scoped writes: deny Edit/Write + writable: [docs/adr], cage: seatbelt (ADR 0014 §1)
		"8e2ecc2dd31c7dae27750768083e97e6de63490be998a5977270b3813ff750f0", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"9708659bdbb628ba778179151a88cb1f10071c82914cae282d0e8d917e2f3fd6", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"36d66cdf6f9c321d2bd937588be4f0e3aaf6cde13a3bcd42f9e1f04083a228a2", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
	},
	"agents/business-manager.md": {
		"dac2a2a52ab880671783c6bf5a2a4559144abcf3e57ff5b6567144434367adbe", // 5668b76
		"73cf5f25a546d1c15361d03bf4cb085d016744fbd8e135a86e2869dd3b6153ed", // 95c4b70
		"4fa50a3f24ec66618508279865f5feb4c33a823a7b986c63ee3e66d9d01c6c03", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"fca1f76ba81ba44a2b67ceb9226315d2b276fb6ddfbaf0334de6e6834e6ec79d", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
		"437dece74de81e14a80236ada4becbe347a64803fc1cb7ba7e4fd74f0e6c7bc3", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"6322ec2e8598e3c7299d10f52eb9189cf1808e462333a0806ff94a00e7f4efbe", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"c5ae0a1c7c495ed89ea128ccc3c4b2eef3bb8a00728b8462831507ec45dfe5a6", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"0831371db1919700c8f0094927d86198db5103fb96e287b2d22fa69fd550d11a", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"6535e3d9cdbde33d80c0b16e118cfcee6eff7057ef46bf58396d8e1a1df08c52", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
	},
	"agents/developer.md": {
		"17e0db0cf3780cb6ba6e0ecd0b13d300f1b45d05b470be6df4d43575b43348de", // 5668b76
		"f8ed74a6ffe8b6a3e536c3643def024ae44afadfcc23d55b23e8119fba233fa9", // 95c4b70
		"2acfe22434656eddb0e93cf86458ba613a294e906896e56a5ea82564858b21f9", // rangerhq-icb3 2026-08-27 skills: [distributed-systems]
		"6d80fcc8fdb81545bc90c57a65bb0c129373617193d89aa2a477be057372afbf", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"f25b7c0d58c339fb325066e77dbda50ed2cf3f05db749d2653be3b2d06b1abcb", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
		"bf7025530a59a0fbc99f461374c3f70efb014cd47766a09d7a22d6daefc409b8", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"7ccf32359553066004eeeae23167d1318551e74b56353130f4b22d03f21ca328", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"cbb8fb9bde58876066e14ce25d5f91db03f701df7c129fb0032758fae7489e26", // ranger-base-ccd 2026-08-29 path-scoped writes: deny Edit/Write(docs/adr/**), cage: seatbelt (ADR 0014 §1)
		"53a6bc6d4096165a750034b45d7e99e8d928245babefcd7046680aa5e88d7301", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"4ac8677547bb71ea1f5bab2523a3059dac45c554dcd670cb0b1701b3c7c63a7b", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"a0815bed7cc6617578b7ed711aaff360f502acba189745d3f85f242a768cb020", // ranger-base-8zhr 2026-08-31 re-scope the git log --grep provenance promise (ADR 0022)
		"170484e9f4c102f68baf2d14d0ae1a2e96566e6245cc4b3df08910716893856a", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
	},
	"agents/devops.md": {
		"a5882633fdf059352d0dfc1dc13386bd7488ff0f4fff56464730e439ff7b9d6c", // 5668b76
		"71ce1ba98dbad596928ba25efb6c64edcff957ef955c6c195c67f39655b53068", // 95c4b70
		"aded31fd932c75be7b5ded9652e52906e55db4b067afc06ff48e3a6f3c6d0ca3", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"5eefb4da132b64b31d2bdd5626e80a5b15945ac324be6c0b65bdeec5debac57d", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
		"1316afeeec3a9bea5bf4363e5e9bc2c63664e98b9cff83111c3156dd9be51b2a", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"f9cafe8aa22275a06f29b134836c5dd96d40190bf9366a5ab57d8d5035abf6e4", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"0d0ca8e12729fc4713a558c1474e2eb7e80efb0057ecfa503991183e7f547891", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"76cafa40ee36604d7f86029d28fb5c9f340e88d1608a88c2c653decfc879ace3", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"721cd9bb378719769520a96cc6c10e79bf532633eb842a1158796fc909b1aee3", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
	},
	"agents/ops.md": {
		"7f0b4779b63fac5b004f2855b72d6f58c65e7b4819765b8c5ed5a4a1874d5a47", // rangerhq-o7y4 2026-08-29 ranger.md renamed to a role (ADR 0012 D2)
		"b49a2f7767bdc3689b7ea29d1daf181f0b63015cd8a220b31de34e9d15f59b54", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"da9eb21e3ff59ce41b5ea38b56ef8dfe192be82fc0515e539ffd2da9f50311ec", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"7d85475d65f5059e3c0342ffba9b3fe2241f4505eb92e64f3b9d3c1a7ab4d9cd", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"a556a7ad1031f59806863e50cb3b2d77edf761c22df3149d3a485f6d8b7398b4", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"978afa246a4d436471eefaf2e4e747dae7c2891c48b621ccf06471d911e45510", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
	},
	"agents/product.md": {
		"c9ce6781c6f3b0d3049ff424815993983fb04419272a17ec6ca8328877f426d6", // 5668b76
		"519f25ba83ee16e2b36a58d3d73683361b6fcf2e70b5c57641d3ef921b9e2597", // 95c4b70
		"c0226cc675ff6a4c1f1ba779462ebb2774a0467f0709897fe3394f285bbbd1c3", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"7efc9ea64577d54ce84ace41bdcee17be01c8194a9d352152c9ea8d7ed7ea6f0", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
		"1a26001e224731f960b771e445cccee463de4c747b2c197dfa9b3e9ae7fe0fc8", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"4989ecdf6c358017c3e8030bb239808f2a4f7ec64fba457decfb05d652f7d779", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"25b9a63f4b12c2a8fd6eaec9d32ad8032d82a80e86282daf4283c659b65aa20c", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"aabed0242f85ddd0a0bcaceb4ce4bafd25362b258cb8d0e0e82ac19e190412d5", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"610f68c6e72987fc7fd1e7e57a08e19fd5bcd3d4c1174f20de627ea4f85c139d", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
	},
	"agents/qa.md": {
		"65eeb0b68cda30bb9ed944c9789c4bd26d5e75f3b69a7d85f5c8eab7e4da832a", // 5668b76
		"707141288b153c8b8eb7b55afce38d18fbbe9914e130ced268a37f6d527256c7", // 95c4b70
		"7e4c8956bb7f7604820fe0df82d3aa052c0e36a1da2a5ea18519725eff939a8c", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"b79572a439fd2bf84df9ac64db045be51d5dd5c3cda879ace11d3575fe03bd1c", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
		"d22b750b79561763a14a25def379eecb22236abbfff41b99b1ff3fdd5bfe850f", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"fa648aec120575968b94821fb944ab6478bdcdb423f1bd60702e2f42bc58defb", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"05d05ec74110283e95ce149ca0164e36ded4f38d2dc9048c675557d32d26eeae", // ranger-base-ccd 2026-08-29 path-scoped writes: deny Edit/Write(docs/adr/**), cage: seatbelt (ADR 0014 §1)
		"663275a4df0e5c05799e35c00c44456f5a827630d5f5fb4122a6c05464b206ec", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"95931b990ef8ad1e77e8efad0c4aed9fdf5edac702addf9af1b74e532609ffd8", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"c6cac35b3fb132bd937b0863aca7e5f3425531082c26a9dbea5da27de815f8c3", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
	},
	// agents/ranger.md is a RETIRED name: rangerhq-o7y4 renamed the example to
	// agents/ops.md (ADR 0012 D2 — persona names become roles). The entries stay
	// because homes seeded by every release above hold this file, and a name
	// posse has shipped is a name posse must still be able to recognise and
	// retire (retireExamplePIDs reads this table's keys, not the embed alone).
	"agents/ranger.md": {
		"a2d154e05cce58b1555ad6ade9d8b828b595d4ee958c5e9c3a0228221fc72b1d", // 5668b76
		"f407345cd3142cdd9177afe209848616baf82fa0171fb53cb8fa7d4bef9f86df", // 95c4b70
		"5bf6aae40aed2403bbe20373dda8898ffc15ed5f09ae3a6f38710305515cbb29", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"3467d83556b9ff6717040e36f353ca31d0c7a1a5fa2e8c3769593d1f2c306ddd", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
	},
	"agents/reviewer.md": {
		"88654f9f35ce94cc2a797cac6dead8ec6540ad0de2449fca0c79a90dfe5e6064", // 5668b76
		"9eee902953c0b567fe9cc0ad5d9f5a1ddd0cdf3d91589711f0ff563465a826d1", // 95c4b70
		"b6d9412b85e4d21be46ce21b04fe51cc30112254286ee3d4dac5f432e4daf7f5", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"540527dfe8d62db37c3cbbafd45652f581680c7eee4af195b0ccfe0e384e5739", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
		"599440774dff9fb8985ad50aefa44fd0e704ab7a2da24bfbf16a9be7a2a5cbcd", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"eb12135f07bf29736afddb1524b449388c17763a73a5a639965e327366f28abd", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"1771e96bd8a49b4daaa4760832a5ab5de6d506da17aec6b8cc75236b19643dd6", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"a49bf606c5231c73d458b1907ef6996c778495beca20ea212d2f42434f73291f", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"5e543fcc549b4d8a7ac9c1466ae3db2c72b4051dfe26a3a1085c6772cd6a10fe", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
	},
	"agents/security.md": {
		"d3d07f404ab3099e93525374c8ae94dbfb12f21ef20434d3777a295c050ad8be", // 5668b76
		"818ccb7d14223e9d58cde93af06813a6020d369d7bb561bd95bdcbd8073728ea", // 95c4b70
		"bd6f2a52dd3054b4b7158957228caf0af7e34dc85fa0ecfb6fad18b47a198406", // rangerhq-dh5g 2026-08-27 crew brand out of the identity line
		"1cc09ad6db1c0387cc66482904ff05711cab8822076b087411b0fa4de4143b6a", // ranger-base-kryn 2026-08-28 deny Bash(posse refresh:*) (ADR 0019 D4)
		"2c65a30a82195d52150cf96c6d361bd850ebcdd2d99552c6b5640f3c4620b84e", // ranger-base-u9ud 2026-08-29 deny bd's destructive/egress verbs (ADR 0015 §3)
		"f2b37fc94f4e06244287c591369c76a3e1a484e549c68610642938bc5382fdad", // ranger-base-09b7 2026-08-29 deny Bash(git commit unless --) — the L1 commit wall reaches the seed
		"f6f828f96bbe03f346b905ecd7755f4601a6100be088cf4f3f2e6c0b1261087a", // ranger-base-t2v2 2026-08-29 narrow the bd hook/hooks deny to install/uninstall (y5g7)
		"260b11e629fea09b363bec067e40e9cdb9e60b71016c90948fce7786788953fd", // 9c00e192 2026-08-31 rename internal/rhq -> internal/posse ($RHQ_PERSONA_DIR -> $POSSE_PERSONA_DIR)
		"80d0aec4270ebd7e3bc93308425e7fdbb067f5e06af4877d4f60a3ea2fa99e7c", // ranger-base-tpc41 2026-09-01 hand to the lane: template line drops -a, Hand-to rows name lanes (ADR 0006 §1)
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
