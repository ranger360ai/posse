//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-6e5jm — the OTHER two lines in `cmdUpgrade` that put
// a derived value on the plan the verb runs: ADR 0092 D4's persona roster and
// D4 amendment 1's repo list.
//
// WHY THIS FILE EXISTS. Three beads in a row gave the same answer to "this
// derivation is unreachable from a test": move it into a function, pin the
// function (ranger-base-1fsc9's `upgradeHookWallRepos`, ranger-base-r1zdr's
// `upgradeLoopReaders`). The function then has a pin and its CALL does not —
// and `d4Fixture.run` assigns `plan.Personas` and `plan.Repos` outright
// rather than deriving them, so the fixture's assignment stands in for the
// verb's and the verb's can be deleted while every D4 pin stays green.
// ranger-base-xtg6z closed the third of those call sites; these are the two
// above it, measured there and fixed here.
//
// MEASURED 2026-11-21, this box (linux 7.2.15-1-omarchy, go1.27.3), over the
// filter below, with upgrade.go restored to the same sha256 after each arm:
//
//	S1  delete `plan.Personas = a.ListAgents()`  → rc=0, 13.1s, no FAIL line
//	S2  delete the `plan.Repos` append           → rc=0, 13.0s, no FAIL line
//
// WHAT EACH LOSES, which is why S1 is the graver one. S1 is the roster D4
// snapshots and re-reads: with it gone the verify block judges `posse status`
// and `posse gates` for NO persona, so D4's second bullet — a 0→non-zero
// exit, a gained DEGRADED row — is evaluated over an empty map and passes
// vacuously. A new binary whose wall is red for every persona in the home
// would install and stand. S2 is the repo list D4 amendment 1 prints: the
// `posse gates install-hooks` line loses its repos and falls back to the
// `<repo>…` placeholder, which is an absent instruction rather than a wrong
// one. ADR 0052 D1's own hole — a managed or absent checkout named in a
// command prescribing a write there — stays closed in the filter
// (`upgradeHookWallRepos`, pinned where it is written); what S2 loses is
// that filter's OUTPUT reaching the report at all.
//
// HOW THEY ARE HELD, and nothing was invented for it. `r1zdrVerbRun`
// (upgradereportloops_qa_test.go) already drives `a.cmdUpgrade` over
// `upgradeSeams` and a `d4Fixture` all the way to the report. So the subject
// of both pins is a report the VERB produced over an App that declares the
// thing: a home with two persona files for S1, a `beads_visibility:` block
// for S2. Each pin asserts a report that is only derivable from the App's own
// declaration — a rollback that only the declared personas' readings can
// trigger, and an install-hooks line naming the declared checkout by path.
//
// THE FILTER THESE ARMS WERE RUN UNDER, the same one ranger-base-xtg6z's
// close used, and it selects both pins here (`^TestQA(The)?Upgrade`):
//
//	^Test(QA)?(Live)?Box(Loop|Census|Proc)|^TestQATheVerbsBoxLine|^TestLiveBoxCensus|^TestUpgrade|^TestQA(The)?Upgrade
//
// MUTATION-CHECKED 2026-11-21 under it, with these two pins in place: S1 reds
// TestQATheUpgradeVerbSnapshotsEveryPersonaTheHomeDeclares and nothing else,
// S2 reds TestQATheUpgradeVerbPrescribesTheReRenderOverTheReposConfigDeclares
// and nothing else. docs/notes.d/ranger-base-6e5jm.md carries the readings.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestQATheUpgradeVerbSnapshotsEveryPersonaTheHomeDeclares is the CALL
// `plan.Personas = a.ListAgents()` — ADR 0092 D4's roster — over a run of the
// verb that reaches the verify block.
//
// THE SUBJECT IS THE VERDICT AND NOT THE ROSTER LINE, because the roster is
// only worth reading if it is what D4's second bullet judges. The new binary
// here is red for one of the two personas this home declares, which is a
// trigger — so the install must be UNDONE. A verb that judged an empty roster
// would install it and say the install stands, which is the whole of what S1
// costs: the assertion is a rollback that no reading in this fixture can
// produce unless the App's own agents dir reached the plan.
//
// TWO PERSONAS AND NOT ONE, for the reason upgradeverify_test.go's own
// fixtures give: a one-persona roster cannot tell "the roster" from "whichever
// persona this code happened to read", and the red one is the SECOND in sort
// order so a loop that stopped after the first would be visible too.
func TestQATheUpgradeVerbSnapshotsEveryPersonaTheHomeDeclares(t *testing.T) {
	a := wtApp(t)
	// ListAgents reads directory entries, so the front matter is not the
	// subject here — the two names are.
	for _, p := range []string{"dev", "ops"} {
		write(t, filepath.Join(a.AgentsDir, p+".md"), "---\nname: "+p+"\ndescription: pin\n---\nYou are "+p+".\n")
	}
	if got := a.ListAgents(); len(got) != 2 {
		t.Fatalf("this home declares %v, want the two personas written above — the fixture cannot state its claim otherwise", got)
	}
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Gates: map[string]fakeGates{"dev": {}, "ops": {}}},
		fakePosse{Gates: map[string]fakeGates{"dev": {}, "ops": {Red: 3}}})

	out, err := r1zdrVerbRun(t, f, a)
	if err == nil {
		t.Fatalf("`posse upgrade` installed a binary whose wall is newly red for persona `ops` and let it stand — the plan the VERB ran carries no roster, so D4's second bullet was judged over an empty map and passed vacuously (ADR 0092 D4):\n%s", out)
	}
	if !strings.Contains(out, "`posse gates ops` gained 3 DEGRADED/REFUSED row(s) (0 → 3)") {
		t.Errorf("the rollback does not name the reading that caused it, so this run did not read `posse gates ops` at all:\n%s", out)
	}
	// BOTH personas were read, on BOTH sides of the swap. The count is the
	// one part of the report that says the roster was the home's and not one
	// name the verdict happened to need. Three, because
	// `upgradeReading.Line` is the only thing that prints it and the run
	// prints three readings: D4's pre-swap snapshot, and the verify block's
	// `before:`/`after:` pair — the first two being the same reading, which
	// is the snapshot standing where a reader who has to roll back will look
	// for it.
	if n := strings.Count(out, "gates 2 persona(s)"); n != 3 {
		t.Errorf("the report states a two-persona roster %d time(s), want 3 (the snapshot, then the verify block's two sides) — the roster is this home's agents dir, snapshotted before the swap and read again after it (ADR 0092 D4):\n%s", n, out)
	}
	// And the install really was undone: D4's rollback is about bytes.
	f.rolledBack(t)
}

// TestQATheUpgradeVerbPrescribesTheReRenderOverTheReposConfigDeclares is the
// CALL `plan.Repos = append(plan.Repos, upgradeHookWallRepos(...)...)` — ADR
// 0092 D4 amendment 1's repo list — over a run of the verb whose swap raises
// the G12 rows that make the line print.
//
// THE CONFIG IS THE ONLY SOURCE OF THE ASSERTED PATH. This home declares two
// checkouts: one real git repo the sweep measures, and one path that is not
// there. The line must name the first, by path, and must not name the second
// — an absent checkout raises no G12 row, and a command prescribing a write
// into a repo posse may not write is the one instruction ADR 0052 D1 says
// this verb must never give. That filter is `upgradeHookWallRepos`' and is
// pinned there; what is held here is its output reaching the report.
//
// THE PLACEHOLDER IS ASSERTED ABSENT as well as the path present, because the
// no-repos branch of upgradeHookWallLine prints `install-hooks <repo>…` — so
// a report whose plan carries no repos still carries the word `install-hooks`
// and a thinner pin would pass over it.
func TestQATheUpgradeVerbPrescribesTheReRenderOverTheReposConfigDeclares(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	a := wtApp(t)
	root := gitTempDir(t)
	declared := hwsRepo(t, root, "declared")
	absent := filepath.Join(root, "never-cut")
	appendConfig(t, a, "beads_visibility:\n  "+declared+": private\n  "+absent+": private\n")
	// The sweep resolves symlinks over the longest existing prefix, so the
	// path the report prints is the resolved one — and the absent entry has
	// no existing prefix of its own to resolve to anything else.
	want, unwanted := AbbrevHome(absResolve(declared)), AbbrevHome(absResolve(absent))
	if want == unwanted {
		t.Fatalf("both config entries resolve to %q, so this pin cannot tell the measured checkout from the absent one", want)
	}

	// Two G12 rows the swap raises: amendment 1's own case, and the only
	// state in which this line prints at all.
	//
	// THE ROWS NAME NEITHER CONFIGURED PATH, on purpose. `upgradeRead` only
	// counts them, so a row carrying `want` could not reach the report — but
	// the assertion below is a path search over the whole of it, and a
	// fixture that plants the asserted string anywhere in the subject is one
	// nobody can read as evidence later.
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa"},
		fakePosse{Conds: GovSet{
			{ID: "G12", Class: GovLane, Key: "hook-wall:pre-push:~/src/x", Detail: "ours but stale"},
			{ID: "G12", Class: GovLane, Key: "hook-wall:prepare-commit-msg:~/src/x", Detail: "ours but stale"},
		}})

	out, err := r1zdrVerbRun(t, f, a)
	if err != nil {
		t.Fatalf("the swap's own G12 rows rolled the install back — they are the designed state after every install that changes the render (ADR 0092 D4 amendment 1, D7): %v\n%s", err, out)
	}
	// The precondition: without the gain there is no line, and the two
	// assertions below would pass for a report that said nothing.
	if !strings.Contains(out, "gained 2 G12 hook-wall row(s)") {
		t.Fatalf("the report does not say the swap raised two hook-wall rows, so it never reached amendment 1's line:\n%s", out)
	}
	if !strings.Contains(out, "posse gates install-hooks "+want) {
		t.Errorf("the re-render line does not name %s, the checkout this home's `beads_visibility:` declares and the sweep measured — the plan the VERB ran carries no repos, so the operator is handed a command with no arguments (ADR 0092 D4 amendment 1):\n%s", want, out)
	}
	if strings.Contains(out, "install-hooks <repo>…") {
		t.Errorf("the report printed the no-repos placeholder over a home that declares a measured checkout:\n%s", out)
	}
	if strings.Contains(out, unwanted) {
		t.Errorf("the re-render line names %s, a declared path with no checkout at it — a command prescribing a write into a repo posse cannot measure is the one instruction this line must never give (ADR 0052 D1):\n%s", unwanted, out)
	}
	// The install stood: amendment 1's line is a note and not a refusal.
	if body, _ := os.ReadFile(f.target); !bytes.Equal(body, f.next.bytes()) {
		t.Errorf("the new binary is not in place after a run whose only finding was the hook wall: %q", body)
	}
}
