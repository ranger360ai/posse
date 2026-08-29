package rhq

// ranger-base-nuu — ADR 0014 §3: the PID's own path-scoped write denies, at
// L2.
//
// `parity.go` has printed `✓ Edit(docs/adr/**) — L2 trailing deny (subpath
// docs/adr)` since ranger-base-4ks, and until this bead the profile carried
// no such line: the matrix named a wall the renderer did not build. What
// makes the wall possible at all is ranger-base-h15's trailing block, which
// measured the property this depends on — SBPL takes the LAST match, so a
// deny under the allow beats it and a deny above it leaks.
//
// So the pins here are the two halves of that claim. The structural half
// says which rules reach the block and where the block sits; the execution
// half runs `sandbox-exec` over a scratch tree and grades the wall by what
// the kernel does with a write. Every refusal runs twice — once under the
// PID that carries the rule and once, on a FRESH tree, under the same PID
// with the rule deleted — because a refusal proves nothing unless the same
// command succeeds without the rule, and the control's success is also the
// witness that the file it wrote to was ever there.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// psFixture is an ORDINARY project: cwd is a repo, the home is somewhere
// else entirely. Deliberately not seatbeltcarveout_qa_test.go's fixture
// (cwd = the constitution repo) — there the trailing block is full of
// posse's own paths, and a deny that only worked because the home was in
// the tree would pass there and hold nothing on a real project.
type psFixture struct {
	a     *App
	repo  string // cwd: the session's project
	gates string
	ag    *AgentFile
}

// psNewFixture builds the tree and loads a PID with the given front matter.
// Fresh per call: the control arm of every probe below is a real write, and
// one probe's control MOVES the directory a later probe measures.
func psNewFixture(t *testing.T, front string) psFixture {
	t.Helper()
	root := sbRoot(t) // HOME and TMPDIR elsewhere: nothing here is granted by accident
	repo := sbMkdir(t, filepath.Join(root, "project"))
	sbGitInit(t, repo)
	home := sbMkdir(t, filepath.Join(root, "home"))
	a := NewAppAt(home)
	homeWithConstitution(t, a, "")
	sbMkdir(t, filepath.Join(a.PersonasDir(), "p"))
	sbWrite(t, filepath.Join(a.PersonasDir(), "p", "ORDERS.md"), "memory is not law\n")

	// The tree the rule is about, its siblings, and one name that merely
	// starts with the denied directory's.
	sbWrite(t, filepath.Join(repo, "docs", "adr", "0001-x.md"), "# adr\nx\n")
	sbWrite(t, filepath.Join(repo, "docs", "design.md"), "design\n")
	sbWrite(t, filepath.Join(repo, "docs", "adrx", "note.md"), "not the adr dir\n")
	sbWrite(t, filepath.Join(repo, "internal", "keep.go"), "package internal\n")
	sbWrite(t, filepath.Join(repo, "README"), "project work\n")
	sbMkdir(t, filepath.Join(repo, beadsDirName))

	ag := cageAgent(t, a, front)
	gates := sbMkdir(t, a.GatesDir(ag.Name))
	return psFixture{a: a, repo: repo, gates: gates, ag: ag}
}

func (f psFixture) writable(t *testing.T) []string {
	t.Helper()
	return f.a.SeatbeltWritable(f.ag, f.repo, f.gates)
}

func (f psFixture) carve(t *testing.T) SeatbeltCarveOut {
	t.Helper()
	return f.a.SeatbeltCarveOut(f.ag, f.repo, f.gates, f.writable(t))
}

// psDenyList is the PID under test: three rules over one directory, which
// is ADR 0014 §1's "any of the three" union written out.
const psDenyList = "cage: seatbelt\ndeny: [Edit(docs/adr/**), Write(docs/adr/**), NotebookEdit(docs/adr/**)]\n"

// psNoRule is the control PID: the same file with the rules deleted.
const psNoRule = "cage: seatbelt\n"

// ─── the structural half ─────────────────────────────────────────────────────

// The rule reaches the trailing block, resolved, and the block is BELOW the
// allow. Ordering is its own assertion because the execution half cannot
// see it: a deny placed above the allow leaks silently, and the leak only
// shows for a path some grant covers — which is every path this feature is
// about, so the probes would all still be green for the wrong reason if
// the block moved.
func TestQAPathScopedDenyLandsInTheTrailingBlockBelowTheAllow(t *testing.T) {
	f := psNewFixture(t, psDenyList)
	w, c := f.writable(t), f.carve(t)
	adr := absResolve(filepath.Join(f.repo, "docs", "adr"))

	if !writeGranted(w, adr) {
		t.Fatalf("premise gone: %s is not inside the allow block at all, so the deny is not what refuses it:\n  %s", adr, strings.Join(w, "\n  "))
	}
	if !sbHas(c.Deny, adr) {
		t.Fatalf("Edit(docs/adr/**) is not in the trailing deny: %v", c.Deny)
	}
	// The rename escape, free through the same struct: the denied tree's
	// parent is granted, so `mv docs docs2` would carry it out from under
	// its own deny.
	if !sbHas(c.Seal, filepath.Join(f.repo, "docs")) {
		t.Errorf("docs/ can be renamed out from under the deny; the seal must cover it: %v", c.Seal)
	}

	prof := SeatbeltProfile(f.ag.Name, w, c)
	allow := strings.Index(prof, "(allow file-write*\n")
	deny := strings.Index(prof, ";; the carve-out")
	entry := strings.Index(prof, "(subpath "+sbQuote(adr)+")")
	if allow < 0 || deny < allow || entry < deny {
		t.Errorf("the path-scoped deny must follow the allow block (ADR 0014 §3): allow=%d carve=%d entry=%d\n%s", allow, deny, entry, prof)
	}
}

// Which rules reach it, and which must not. A bare spelling is the
// whole-tree rule — realized by SeatbeltWritable omitting cwd, and a
// subpath deny of cwd instead would take the session's `.beads` and `.git`
// with it. A file filter is unrealized at every tier (ADR 0014 §1/§2), and
// denying its directory prefix would be the profile enforcing a rule the
// matrix says nobody holds — over-enforcement dressed as a wall.
func TestQAOnlySubtreeGlobsReachTheProfile(t *testing.T) {
	for _, tc := range []struct {
		name, front string
		cwdWritable bool
		denied      []string
		notDenied   []string
	}{
		{
			name: "subtree glob", front: psDenyList, cwdWritable: true,
			denied:    []string{"docs/adr"},
			notDenied: []string{".", "docs", "docs/adrx", "internal"},
		},
		{
			// `Edit(**)` means what `Edit` means: cwd out of the allow, and
			// nothing new in the deny.
			name: "bare spelling", front: "cage: seatbelt\ndeny: [Edit(**), Write(**)]\n", cwdWritable: false,
			notDenied: []string{".", "docs", "docs/adr", beadsDirName},
		},
		{
			name: "file filter", front: "cage: seatbelt\ndeny: [Edit(**/*.md), Write(docs/adr/**/*.md)]\n", cwdWritable: true,
			notDenied: []string{".", "docs", "docs/adr"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := psNewFixture(t, tc.front)
			w, c := f.writable(t), f.carve(t)
			if got := writeGranted(w, f.repo); got != tc.cwdWritable {
				t.Errorf("cwd writable=%v, want %v — a path-scoped deny must not move the allow block:\n  %s", got, tc.cwdWritable, strings.Join(w, "\n  "))
			}
			for _, p := range tc.denied {
				if !sbHas(c.Deny, filepath.Join(f.repo, p)) {
					t.Errorf("%s is not in the trailing deny: %v", p, c.Deny)
				}
			}
			for _, p := range tc.notDenied {
				if sbHas(c.Deny, filepath.Join(f.repo, p)) {
					t.Errorf("%s must not be in the trailing deny: %v", p, c.Deny)
				}
			}
		})
	}
}

// Where a relative glob resolves, and where the other two spellings do.
// Resolve is the half that depends on a session, so this is the pin that a
// glob joined THIS session's dir and not the process's cwd — the failure a
// dispatched session would never notice, because the profile would still
// deny something.
func TestQAPathScopedDenyResolvesAgainstTheSessionDir(t *testing.T) {
	f := psNewFixture(t, "cage: seatbelt\ndeny: [Edit(docs/adr/**), Write(~/secrets/**), NotebookEdit(/etc/rhq-nuu/**)]\n")
	c := f.carve(t)
	home, _ := os.UserHomeDir()
	for _, want := range []string{
		absResolve(filepath.Join(f.repo, "docs", "adr")),
		absResolve(filepath.Join(home, "secrets")),
		absResolve("/etc/rhq-nuu"),
	} {
		if !sbHas(c.Deny, want) {
			t.Errorf("%s is not in the trailing deny: %v", want, c.Deny)
		}
	}
}

// ADR 0014 §1's deny-wins, at the tier: a `writable:` extra inside a denied
// subtree grants nothing, because the extra is in the allow block and the
// deny is under it. `posse agent check` warns about the PID; this is what
// the profile does with it.
func TestQAWritableExtraInsideADeniedSubtreeLoses(t *testing.T) {
	f := psNewFixture(t, "cage: seatbelt\ndeny: [Edit(docs/adr/**)]\nwritable: [docs/adr]\n")
	w, c := f.writable(t), f.carve(t)
	adr := absResolve(filepath.Join(f.repo, "docs", "adr"))
	if !sbHas(w, adr) {
		t.Fatalf("premise gone: the extra is not in the allow block, so nothing is being outvoted:\n  %s", strings.Join(w, "\n  "))
	}
	if !sbHas(c.Deny, adr) {
		t.Errorf("the extra swallowed the deny — deny-wins (ADR 0001) is what the trailing block is for: %v", c.Deny)
	}
}

// The operator-readable half. The two lists render into one block (ADR 0014
// §3's slot), but they answer different questions: posse's entries are a
// wall the PID cannot spell, and this one is the PID's own line printed
// back at the tier that realizes it — beside the directory it resolved to,
// which is how a glob that joined the wrong dir gets caught before a
// launch rather than after one.
func TestQAGatesPrintsThePIDsOwnRuleBesideTheDirectory(t *testing.T) {
	f := psNewFixture(t, psDenyList)
	var b strings.Builder
	if err := f.a.SeatbeltReport(f.ag, f.repo, &b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	// All three rules, not the last one to resolve there: ADR 0014 §1's
	// union is normally written as the triple over one directory, and a
	// report naming one of them would read as if deleting it re-opens the
	// tree.
	want := "    x " + AbbrevHome(absResolve(filepath.Join(f.repo, "docs", "adr"))) +
		" (trailing deny — the PID's Edit(docs/adr/**), Write(docs/adr/**), NotebookEdit(docs/adr/**); ADR 0014 §3)"
	if !strings.Contains(got, want) {
		t.Errorf("the gates report does not print %q:\n%s", want, got)
	}
	// h15's own entries keep their own attribution: a report that credited
	// the PID for posse's wall would tell an operator the wall goes away
	// when the rule does.
	if !strings.Contains(got, "(trailing deny — beats every grant above; ranger-base-h15)") {
		t.Errorf("posse's own carve-out entries lost their attribution:\n%s", got)
	}
}

// ─── the execution half ──────────────────────────────────────────────────────

type psProbe struct {
	what string
	sh   func(f psFixture) string // /bin/sh -c, built against a fresh tree
	want bool                     // true: still allowed WITH the rule
}

// psTry renders a profile for a FRESH tree — with the rule or without —
// and reports whether the write was allowed. The control is the same PID
// minus the three rules rather than an empty carve-out: what is being
// measured is this bead's list, not ranger-base-h15's.
func psTry(t *testing.T, p psProbe, withRule bool) bool {
	t.Helper()
	front := psNoRule
	name := "control.sb"
	if withRule {
		front, name = psDenyList, "walled.sb"
	}
	f := psNewFixture(t, front)
	w := f.writable(t)
	prof := sbRenderProfile(t, name, SeatbeltProfile(f.ag.Name, w, f.carve(t)))
	return sbRun(t, prof, p.sh(f))
}

// ADR 0014 verification item 2, executed: `touch` and `sed -i` on the
// denied subtree are Operation not permitted, a python write to it fails
// too, and a write next to it succeeds.
func TestQAPathScopedDenyRefusesUnderSandboxExecAndTheControlDoesNot(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	adr := func(f psFixture, parts ...string) string {
		return filepath.Join(append([]string{f.repo, "docs", "adr"}, parts...)...)
	}
	probes := []psProbe{
		{"touch a new file in the denied subtree", func(f psFixture) string { return "touch " + adr(f, "0002-new.md") }, false},
		{"sed -i an existing ADR in place", func(f psFixture) string { return "sed -i '' s/x/y/ " + adr(f, "0001-x.md") }, false},
		{"append with the shell", func(f psFixture) string { return "echo pwn >> " + adr(f, "0001-x.md") }, false},
		{"truncate an ADR", func(f psFixture) string { return ": > " + adr(f, "0001-x.md") }, false},
		{"delete an ADR", func(f psFixture) string { return "rm " + adr(f, "0001-x.md") }, false},
		{"mkdir inside the denied subtree", func(f psFixture) string { return "mkdir " + adr(f, "sub") }, false},
		// The two escapes a subpath deny does not close by itself. The
		// first is a write on the denied directory (its own subpath names
		// it); the second is a write on the PARENT, which only the rename
		// seal reaches.
		{"rename the denied subtree", func(f psFixture) string { return "mv " + adr(f) + " " + adr(f) + "2" }, false},
		{"rename its parent out from under the deny", func(f psFixture) string {
			return "mv " + filepath.Join(f.repo, "docs") + " " + filepath.Join(f.repo, "docs2")
		}, false},
		// A symlinked spelling: the kernel matches the resolved path, which
		// is the path the profile carries (absResolve), so neither spelling
		// is a way in.
		{"write through a symlink into the denied subtree", func(f psFixture) string {
			return "ln -s " + adr(f) + " " + filepath.Join(f.repo, "link") + " && touch " + filepath.Join(f.repo, "link", "PWNED.md")
		}, false},

		// What the rule must NOT cost. These are allowed under the rule too,
		// so they are not controls — they are the cost check, and the last
		// of them is the one a prefix match would break.
		{"a write next to the denied subtree", func(f psFixture) string { return "touch " + filepath.Join(f.repo, "docs", "new.md") }, true},
		{"the rest of the repo", func(f psFixture) string { return "echo x >> " + filepath.Join(f.repo, "internal", "keep.go") }, true},
		{"the repo root", func(f psFixture) string { return "echo x >> " + filepath.Join(f.repo, "README") }, true},
		{"the record stage", func(f psFixture) string { return "touch " + filepath.Join(f.repo, beadsDirName, "beads.db") }, true},
		{"its own memory (§5)", func(f psFixture) string { return "echo x >> " + filepath.Join(f.ag.MemoryDir, "ORDERS.md") }, true},
		{"a sibling whose name merely starts with the denied one", func(f psFixture) string {
			return "touch " + filepath.Join(f.repo, "docs", "adrx", "new.md")
		}, true},
	}
	if _, err := exec.LookPath("python3"); err == nil {
		probes = append(probes, psProbe{"a python open().write into the denied subtree", func(f psFixture) string {
			return `python3 -c "open('` + adr(f, "0001-x.md") + `','a').write('pwn')"`
		}, false})
	} else {
		t.Log("no python3 on this host: ADR 0014 item 2's third write is not measured here")
	}

	verb := map[bool]string{true: "ALLOWED", false: "REFUSED"}
	for _, p := range probes {
		t.Run(p.what, func(t *testing.T) {
			if got := psTry(t, p, true); got != p.want {
				t.Errorf("%s under the path-scoped deny, want %s", verb[got], verb[p.want])
			}
			if p.want {
				return
			}
			if !psTry(t, p, false) {
				t.Errorf("the CONTROL refused it too — the probe proves nothing about the rule")
			}
		})
	}
}

// Deny-wins, executed. The extra is in the allow block (asserted above) and
// the write is still refused; the control is the same PID with the deny
// gone, where the extra is the only thing granting that path — so its
// success is also the witness that `writable:` was doing anything at all.
func TestQAWritableExtraInsideADeniedSubtreeIsRefusedUnderSandboxExec(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	run := func(front string) bool {
		f := psNewFixture(t, front)
		w := f.writable(t)
		prof := sbRenderProfile(t, "extra.sb", SeatbeltProfile(f.ag.Name, w, f.carve(t)))
		return sbRun(t, prof, "touch "+filepath.Join(f.repo, "docs", "adr", "0002-new.md"))
	}
	// Bare Edit/Write take cwd out of the allow, so `writable: [docs/adr]`
	// is the ONLY grant on that path — the allow-list shape of ADR 0014 §1.
	// With the subtree rule beside it, the deny is under the extra and the
	// extra loses.
	if run("cage: seatbelt\ndeny: [Edit, Write, Edit(docs/adr/**)]\nwritable: [docs/adr]\n") {
		t.Errorf("the writable: extra outvoted the path-scoped deny — deny-wins (ADR 0001, ADR 0014 §1)")
	}
	if !run("cage: seatbelt\ndeny: [Edit, Write]\nwritable: [docs/adr]\n") {
		t.Errorf("control: the extra grants nothing even without the deny — the test above measured the allow block, not the deny")
	}
}
