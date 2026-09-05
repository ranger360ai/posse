package posse

// ranger-base-h15: the trailing deny. The tier's wall against the
// constitution was STRUCTURAL — the allow block simply never named the home
// (ADR 0015 §2, pinned in seatbeltconstitution_qa_test.go) — and that
// argument holds only while the constitution is outside every grant. It is
// not: `~/.config/rhq` is a symlink into the constitution repo's `rhq`, and a
// session dispatched into THAT repo is granted cwd whole. Measured twice on
// the live shape (ranger-base-6ne by writing a PID; ranger-base-0djg by
// rendering a probe home and executing under it): agents/, config.yaml,
// recipes/, skills/, envs/ and promoted.json all writable, under both the
// symlink spelling and the real one.
//
// So this file pins the second wall — a deny AFTER the allow, which SBPL
// resolves last-match-wins — and it pins it by EXECUTION under
// sandbox-exec, with the same profile minus the carve-out as the control.
// A deny asserted by reading profile text is a deny nobody has watched
// refuse anything.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sbFixture is the live shape in miniature: a constitution repo whose
// ConstitutionSourceDir holds the home, the home reached through a SYMLINK
// the way ~/.config/rhq reaches it, a second repo as the store of record,
// and a redirect from one to the other so the record stage is in the picture —
// developer's constraint on this bead is that the deny must never cost it.
type sbFixture struct {
	a           *App
	repo, store string // the session's repo (cwd) and the store of record
	gates       string
	ag          *AgentFile
}

func sbNewFixture(t *testing.T) sbFixture {
	t.Helper()
	root := sbRoot(t) // HOME elsewhere, TMPDIR a sibling: nothing here is granted by accident
	repo := sbMkdir(t, filepath.Join(root, "constitution"))
	store := sbMkdir(t, filepath.Join(root, "store"))
	for _, r := range []string{repo, store} {
		sbGitInit(t, r)
	}
	// The home lives IN the repo and is reached through the link.
	real := sbMkdir(t, filepath.Join(repo, ConstitutionSourceDir))
	home := filepath.Join(root, "home")
	if err := os.Symlink(real, home); err != nil {
		t.Fatal(err)
	}
	a := NewAppAt(home)
	homeWithConstitution(t, a, "")
	sbMkdir(t, filepath.Join(a.PersonasDir(), "developer"))
	sbWrite(t, filepath.Join(a.PersonasDir(), "developer", "ORDERS.md"), "memory is not law\n")
	sbWrite(t, filepath.Join(repo, "README"), "project work\n")
	// Two personas' gate artifacts: the session's own, and one it has no
	// business writing at all.
	gates := sbMkdir(t, a.GatesDir("developer"))
	sbWrite(t, filepath.Join(sbMkdir(t, filepath.Join(gates, "bin")), "git"), "#!/bin/sh\n")
	sbWrite(t, filepath.Join(sbMkdir(t, filepath.Join(a.GatesDir("devops"), "bin")), "security"), "#!/bin/sh\n")
	// The store of record, reached by a redirect the way a work repo
	// reaches the shared store's .beads.
	sbMkdir(t, filepath.Join(store, beadsDirName))
	sbMkdir(t, filepath.Join(repo, beadsDirName))
	sbWrite(t, filepath.Join(repo, beadsDirName, beadsRedirect), filepath.Join(store, beadsDirName)+"\n")

	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(a.PersonasDir(), "developer")}
	return sbFixture{a: a, repo: repo, store: store, gates: gates, ag: ag}
}

func (f sbFixture) writable(t *testing.T) []string {
	t.Helper()
	return f.a.SeatbeltWritable(f.ag, f.repo, f.gates)
}

func (f sbFixture) carve(t *testing.T) SeatbeltCarveOut {
	t.Helper()
	return f.a.SeatbeltCarveOut(f.ag, f.repo, f.gates, f.writable(t))
}

func sbGitInit(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	// `-b main` and not the box's default. git's own built-in default is
	// still `master`, and `init.defaultBranch` is set on the box that wrote
	// this (by /Library/Developer/CommandLineTools' system gitconfig, not
	// by anything in HOME — so the fixture's HOME isolation does not reach
	// it) and unset on GitHub's runners. The fixtures built here go on to
	// `git rebase main`, which on a runner met `fatal: invalid upstream
	// 'main'` — two QA pins red on macos-latest for as long as ci.yml has
	// existed, over the name of a branch (ranger-base-90y3c).
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
}

func sbWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sbDenied(c SeatbeltCarveOut, p string) bool { return writeGranted(c.Deny, p) }

// The premise, stated as a test rather than as a paragraph: with cwd inside
// the constitution repo the ALLOW block reaches everything this bead is
// about. If this ever goes green for the wrong reason — a narrower cwd
// grant, a home that moved out of the repo — the carve-out tests below stop
// proving anything, and this one says so first.
func TestQACwdGrantStillReachesTheConstitutionAndTheGates(t *testing.T) {
	f := sbNewFixture(t)
	w := f.writable(t)
	for _, p := range append(f.a.HomeConstitutionPaths(),
		filepath.Join(f.a.StateDir, "gates", "devops"),
		filepath.Join(f.repo, ".git", "hooks")) {
		if !writeGranted(w, p) {
			t.Errorf("premise gone: %s is no longer inside the allow block:\n  %s", p, strings.Join(w, "\n  "))
		}
	}
}

// Item 1 and item 2 of the bead, in the profile: every constitution path
// the detector knows about, plus every persona's gate artifacts, are in the
// trailing deny — and the deny is BELOW the allow, which is the only place
// it works (deny-before-allow leaks; ADR 0014 §3 measured that, and this
// file's execution test would not catch an ordering slip on its own,
// because a leaked allow only shows up for a path some grant covers).
func TestQACarveOutNamesTheConstitutionTheGatesAndTheHooks(t *testing.T) {
	f := sbNewFixture(t)
	c := f.carve(t)
	for _, p := range append(f.a.HomeConstitutionPaths(),
		filepath.Join(f.a.StateDir, "gates"),
		filepath.Join(f.a.StateDir, "gates", "devops", "bin", "security"),
		filepath.Join(f.repo, ".git", "hooks"),
		filepath.Join(f.store, ".git", "hooks"), // the store of record's slot: its .git is granted too
	) {
		if !sbDenied(c, p) {
			t.Errorf("%s is not in the carve-out: %v", p, c.Deny)
		}
	}
	// §5's exception and the session's own runtime data are NOT in it.
	for _, p := range []string{f.ag.MemoryDir, f.a.StateDir, filepath.Join(f.a.StateDir, "skills"), filepath.Join(f.repo, "README"), filepath.Join(f.repo, ".git", "index")} {
		if sbDenied(c, p) {
			t.Errorf("%s must stay writable — the deny is enumerated at the artifact level: %v", p, c.Deny)
		}
	}
	prof := SeatbeltProfile("developer", f.writable(t), nil, c)
	allow := strings.Index(prof, "(allow file-write*\n")
	deny := strings.Index(prof, ";; the carve-out")
	if allow < 0 || deny < allow {
		t.Errorf("the carve-out must follow the allow block — before it, the allow leaks (ADR 0014 §3):\n%s", prof)
	}
	if !strings.Contains(prof, "(subpath "+sbQuote(absResolve(filepath.Join(f.a.Home, "agents")))+")") {
		t.Errorf("the deny must carry the RESOLVED path, not the symlink spelling:\n%s", prof)
	}
}

// developer's constraint (measured on security's live profile, 2026-08-26): the
// trailing deny beats the redirect grant too, so a deny list that ever
// names the instance repo kills the record stage — `bd sync`, `bd export`,
// the commit of the jsonl — with no observable, since parity grades denies
// and a cage that denies too much still reports every gate realized.
func TestQACarveOutLeavesTheStoreOfRecordWritable(t *testing.T) {
	f := sbNewFixture(t)
	w, c := f.writable(t), f.carve(t)
	for _, p := range []string{
		filepath.Join(f.store, beadsDirName),
		filepath.Join(f.store, beadsDirName, "beads.db"),
		filepath.Join(f.store, ".git", "index.lock"),
		filepath.Join(f.repo, beadsDirName),
	} {
		if !writeGranted(w, p) {
			t.Fatalf("premise gone: %s is not granted at all:\n  %s", p, strings.Join(w, "\n  "))
		}
		if sbDenied(c, p) {
			t.Errorf("the carve-out took the record stage away: %s is denied by %v", p, c.Deny)
		}
	}
}

// The rename escape. A subpath deny is a statement about a PATH: rename the
// directory ABOVE it and the tree is out from under its own deny, at a path
// the profile never heard of. Measured allowed before the seal.
func TestQACarveOutSealsTheAncestorsAGrantMadeRenamable(t *testing.T) {
	f := sbNewFixture(t)
	c := f.carve(t)
	for _, p := range []string{f.a.Home, filepath.Join(f.a.Home, "state"), filepath.Join(f.repo, ".git")} {
		if !sbHas(c.Seal, p) {
			t.Errorf("%s can be renamed out from under the deny; seal it: %v", p, c.Seal)
		}
	}
	// Bounded: the walk stops at the first ancestor whose own parent no
	// grant covers, because a rename needs the destination beside it. cwd's
	// parent is not granted, so cwd is not sealed and neither is anything
	// above it.
	for _, p := range []string{f.repo, filepath.Dir(f.repo), "/"} {
		if sbHas(c.Seal, p) {
			t.Errorf("%s cannot be renamed anyway — sealing it is deny-list growth for nothing: %v", p, c.Seal)
		}
	}
}

// A session whose cwd is an ordinary project: the constitution is outside
// every grant already, so the carve-out is a second wall over a structural
// one and must add almost nothing. Named because a deny that grows for
// every session is a deny that eventually names something a session needs.
func TestQACarveOutIsSmallForAnOrdinaryProject(t *testing.T) {
	f := sbNewFixture(t)
	work := sbMkdir(t, filepath.Join(filepath.Dir(f.repo), "project"))
	sbGitInit(t, work)
	w := f.a.SeatbeltWritable(f.ag, work, f.gates)
	c := f.a.SeatbeltCarveOut(f.ag, work, f.gates, w)
	if bad := f.a.ConstitutionGrants(w); len(bad) > 0 {
		t.Fatalf("premise gone: an ordinary project reaches the constitution %v", bad)
	}
	// Exactly one seal, and it is the project's own `.git`: its hooks dir
	// is denied and cwd's grant would let a session rename `.git` around
	// that. Nothing else in the project tree, and not the tree itself.
	if len(c.Seal) != 1 || !sbHas(c.Seal, filepath.Join(work, ".git")) {
		t.Errorf("seal for an ordinary project should be just <cwd>/.git: %v", c.Seal)
	}
	if !sbDenied(c, filepath.Join(f.a.StateDir, "gates", "devops")) {
		t.Errorf("another persona's gates are writable from an ordinary project too: %v", c.Deny)
	}
}

// The operator-readable half. ADR 0015 §2's verdict was printed rather than
// asserted because a deny-list's failure mode is that nobody can tell by
// looking whether it is still complete — and this bead adds a deny-list. So
// it prints under the set it takes back from, and the constitution verdict
// says which of the two walls is holding: a `writable:` extra reaching the
// promoted set is still named ✗ (it is a PID to fix), but named as refused
// rather than as a hole, because it now is one.
func TestQAGatesPrintsTheCarveOutUnderTheSet(t *testing.T) {
	f := sbNewFixture(t)
	var b strings.Builder
	if err := f.a.SeatbeltReport(f.ag, f.repo, &b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		"    x " + AbbrevHome(absResolve(filepath.Join(f.a.Home, "agents"))) + " (trailing deny",
		"    x " + AbbrevHome(absResolve(filepath.Join(f.a.StateDir, "gates"))) + " (trailing deny",
		"    x " + AbbrevHome(absResolve(f.a.Home)) + " (rename seal only",
		"    w " + AbbrevHome(absResolve(filepath.Join(f.gates, "refusals.log"))) + " (re-allowed",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the gates report does not print %q:\n%s", want, got)
		}
	}
	// cwd is the constitution repo, so the allow block DOES reach it: the
	// verdict must be the qualified one, and never the all-clear.
	if !strings.Contains(got, "refused by the trailing deny below it") {
		t.Errorf("a grant the deny takes back must be named as refused:\n%s", got)
	}
	if strings.Contains(got, "in no grant above") {
		t.Errorf("the all-clear is a lie here — the grant reaches the constitution:\n%s", got)
	}
}

// ─── the execution half ──────────────────────────────────────────────────────

// sbProbe is one write, run under a rendered profile. The wall is graded by
// what the kernel does with it, and every refusal is run TWICE: once under
// the profile with the carve-out and once, on a FRESH fixture, under the
// same profile without it. The second run is the control, and it is the
// point — a refusal proves nothing unless the same command SUCCEEDS with
// the deny removed, and its success is also the witness that the fixture it
// wrote to was really there.
//
// Fresh fixture per run, because the control is a real write: the control
// arm of the rename probe MOVES the constitution, and sharing one tree made
// every later probe fail under both profiles for a reason that had nothing
// to do with the deny. A probe that ran against a fixture an earlier probe
// destroyed is the shape of a green suite over no wall at all.
type sbProbe struct {
	what string
	sh   func(f sbFixture) string // /bin/sh -c, built against a fresh fixture
	want bool                     // true: must still be allowed WITH the carve-out
}

func sbRun(t *testing.T, profile, sh string) bool {
	t.Helper()
	// Backstop: a probe added later that forgets the top-level gate skips
	// here rather than reporting a refusal the kernel never gave it.
	sbSkipUnlessSandboxable(t)
	cmd := exec.Command("sandbox-exec", "-f", profile, "/bin/sh", "-c", sh)
	out, err := cmd.CombinedOutput()
	if err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() > 1 {
		// A refusal is what we are measuring; anything else — a broken
		// profile, a missing binary — must not be read as one.
		t.Fatalf("probe %q failed for a reason that is not the sandbox: %v %s", sh, err, out)
	}
	return err == nil
}

func sbRenderProfile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	sbWrite(t, p, body)
	return p
}

// sbTry renders a profile for a fresh fixture — with the carve-out or
// without — and reports whether the write was allowed.
func sbTry(t *testing.T, p sbProbe, withCarve bool) bool {
	t.Helper()
	f := sbNewFixture(t)
	w := f.writable(t)
	carve := SeatbeltCarveOut{}
	name := "control.sb"
	if withCarve {
		carve, name = f.carve(t), "walled.sb"
	}
	return sbRun(t, sbRenderProfile(t, name, SeatbeltProfile("developer", w, nil, carve)), p.sh(f))
}

// The bead's own test shape, executed: render for a PID with cwd = the
// constitution repo, and assert a write to the constitution's `agents/x` is
// refused while a write to its `README` stays allowed — plus the gate
// artifacts, the hook
// slots, the two escapes a subpath deny does not close by itself, and the
// record stage developer's constraint protects.
func TestQACarveOutRefusesUnderSandboxExecAndTheControlDoesNot(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	home := func(f sbFixture, parts ...string) string {
		return filepath.Join(append([]string{f.a.Home}, parts...)...)
	}
	probes := []sbProbe{
		{"write a PID into the constitution", func(f sbFixture) string { return "touch " + home(f, "agents", "PWNED.md") }, false},
		{"rewrite an existing PID in place", func(f sbFixture) string { return "sed -i '' s/x/y/ " + home(f, "agents", "developer.md") }, false},
		{"append to config.yaml", func(f sbFixture) string { return "echo x >> " + home(f, "config.yaml") }, false},
		{"truncate the promote manifest", func(f sbFixture) string { return ": > " + home(f, PromoteManifestFile) }, false},
		// ranger-base-bejb / ranger-base-zio33: the truncate above is the
		// MODIFY half. ADR 0015 §3's amended anchor bullet claims the other
		// half too — at seatbelt a caged session can neither modify the
		// promoted copy "nor remove the thing that would notice" — and
		// removal is the half that matters there: a TRUNCATED manifest fails
		// loud (DEGRADED), an ABSENT one reads as "never promoted — OK" and
		// silences the detector with no output on any surface. Both spellings
		// an operator would reach for, executed, because a deny read off
		// profile text is a deny nobody has watched refuse anything.
		{"delete the promote manifest", func(f sbFixture) string { return "rm " + home(f, PromoteManifestFile) }, false},
		{"move the promote manifest aside", func(f sbFixture) string {
			return "mv " + home(f, PromoteManifestFile) + " " + home(f, PromoteManifestFile+".bak")
		}, false},
		{"write an env secret", func(f sbFixture) string { return "echo K=v >> " + home(f, "envs", "default.env") }, false},
		{"delete a promoted directory", func(f sbFixture) string { return "rm -rf " + home(f, "recipes") }, false},
		{"rewrite ANOTHER persona's L1 shim", func(f sbFixture) string {
			return "echo pwn > " + filepath.Join(f.a.StateDir, "gates", "devops", "bin", "security")
		}, false},
		{"rewrite this session's own L1 shim", func(f sbFixture) string { return "echo pwn > " + filepath.Join(f.gates, "bin", "git") }, false},
		{"rewrite the rendered seatbelt profile", func(f sbFixture) string { return "echo pwn > " + filepath.Join(f.gates, "seatbelt.sb") }, false},
		{"plant an L3 hook", func(f sbFixture) string {
			return "echo 'exit 0' > " + filepath.Join(f.repo, ".git", "hooks", "pre-push")
		}, false},
		{"plant an L3 hook in the store of record", func(f sbFixture) string {
			return "echo 'exit 0' > " + filepath.Join(f.store, ".git", "hooks", "prepare-commit-msg")
		}, false},
		// The home is reached through a symlink; what a rename would carry
		// away is the REAL directory, which is what the seal names.
		{"rename the home out from under the deny", func(f sbFixture) string {
			return "mv " + filepath.Join(f.repo, ConstitutionSourceDir) + " " + filepath.Join(f.repo, ConstitutionSourceDir+"2")
		}, false},
		{"rename state/ out from under the gates deny", func(f sbFixture) string {
			return "mv " + filepath.Join(f.repo, ConstitutionSourceDir, "state") + " " + filepath.Join(f.repo, ConstitutionSourceDir, "state2")
		}, false},
		{"hardlink a PID out and write through it", func(f sbFixture) string {
			return "ln " + home(f, "agents", "developer.md") + " " + filepath.Join(f.repo, "hard.md") + " && echo pwn >> " + filepath.Join(f.repo, "hard.md")
		}, false},
		{"reach the constitution through the other spelling", func(f sbFixture) string {
			return "touch " + filepath.Join(f.repo, ConstitutionSourceDir, "agents", "PWNED2.md")
		}, false},

		// And what a session must keep. These are ALLOW under the carve-out
		// too, so they are not controls — they are the cost check.
		{"project work next to the constitution", func(f sbFixture) string {
			return "touch " + filepath.Join(f.repo, ConstitutionSourceDir, "NOTES-not-promoted.md")
		}, true},
		{"the repo the session was dispatched into", func(f sbFixture) string { return "echo x >> " + filepath.Join(f.repo, "README") }, true},
		{"its own memory (§5)", func(f sbFixture) string { return "echo x >> " + filepath.Join(f.ag.MemoryDir, "ORDERS.md") }, true},
		{"L1's audit trail, created on first append", func(f sbFixture) string {
			return "echo refusal >> " + filepath.Join(f.gates, "refusals.log")
		}, true},
		{"the gate shell's log", func(f sbFixture) string { return "echo line >> " + filepath.Join(f.gates, "shell.log") }, true},
		{"the record stage: the store of record", func(f sbFixture) string {
			return "touch " + filepath.Join(f.store, beadsDirName, "beads.db")
		}, true},
		{"the record stage: the store's git", func(f sbFixture) string { return "touch " + filepath.Join(f.store, ".git", "index.lock") }, true},
		{"posse's own state beside the gates", func(f sbFixture) string { return "touch " + filepath.Join(f.a.StateDir, "probe") }, true},
	}
	verb := map[bool]string{true: "ALLOWED", false: "REFUSED"}
	for _, p := range probes {
		t.Run(p.what, func(t *testing.T) {
			if got := sbTry(t, p, true); got != p.want {
				t.Errorf("%s under the carve-out, want %s", verb[got], verb[p.want])
			}
			if p.want {
				return
			}
			if !sbTry(t, p, false) {
				t.Errorf("the CONTROL refused it too — the probe proves nothing about the deny")
			}
		})
	}
}
