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
	"fmt"
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
	) {
		if !sbDenied(c, p) {
			t.Errorf("%s is not in the carve-out: %v", p, c.Deny)
		}
	}
	// THE STORE OF RECORD'S OWN HOOKS SLOT WAS IN THAT LIST — "its .git is
	// granted too" — and is in NEITHER list since ranger-base-kfrpv. The
	// grant is gone (nothing in a seat commits in the store repo), so the
	// deny has nothing to narrow, and a deny that exists is a cost of its
	// own at the bwrap mechanism: it renders a read-only bind wall-watch
	// lists, and a host write at the path under it kills every live seat.
	// Asserted as neither granted nor denied, because "not denied" alone
	// would also be true of the hole.
	for _, p := range []string{
		filepath.Join(f.store, ".git"),
		filepath.Join(f.store, ".git", "hooks"),
		filepath.Join(f.store, ".git", "objects"),
	} {
		if writeGranted(f.writable(t), p) {
			t.Errorf("%s is granted — the store repo's git is read-only to every seat (ranger-base-kfrpv)", p)
		}
		if sbDenied(c, p) {
			t.Errorf("%s is denied, over a grant nothing makes — at L3 that deny is a wall-watch entry a host write kills the seat over (ranger-base-kfrpv): %v", p, c.Deny)
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
// a claim, a comment, a close — with no observable, since parity grades
// denies and a cage that denies too much still reports every gate realized.
// "the commit of the jsonl" stood in that list and is not a seat's write:
// the launcher makes it, uncaged (ranger-base-kfrpv).
func TestQACarveOutLeavesTheStoreOfRecordWritable(t *testing.T) {
	f := sbNewFixture(t)
	w, c := f.writable(t), f.carve(t)
	// `<store>/.git/index.lock` was in this list, for a commit of the jsonl
	// the seat does not make: the launcher commits it, uncaged
	// (ranger-base-kfrpv). What the record stage writes in another repo is
	// the store DIRECTORY, and that is what must survive every deny.
	for _, p := range []string{
		filepath.Join(f.store, beadsDirName),
		filepath.Join(f.store, beadsDirName, "beads.db"),
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
	want sbWant
}

// sbWant is what the kernel must do with a probe's write, and there are
// THREE answers here rather than two.
//
// The third one is ranger-base-kfrpv's: the store repo's `.git` left every
// tier's write set when that bead landed, so a write under it is refused
// with the carve-out and without it — the grant is absent, not denied, and
// a deny of its own would be a cost at the bwrap mechanism (a read-only
// bind wall-watch lists, which a host write under it kills the seat over).
// `TestQACarveOutNamesTheConstitutionTheGatesAndTheHooks` asserts that pair
// over the SET; these two rows are the same claim at the kernel, which is
// this file's whole reason for existing.
//
// IT IS NOT "REFUSED, DON'T ASK WHY", and the control is what keeps it from
// becoming that: an sbUngranted row runs a THIRD profile with the grant
// ranger-base-kfrpv removed put back, and must be ALLOWED there. Without
// that arm a malformed command, or a fixture an earlier probe destroyed,
// would read exactly like an absent grant — the same hole the sbCarveDeny
// control closes one answer over, and the same idiom
// storerepogitwalllive_test.go's rows use at the other mechanism ("THE
// CONTROL IS THE PRE-FIX RENDER").
type sbWant int

const (
	sbAllow     sbWant = iota // granted, and the carve-out must not take it back
	sbCarveDeny               // refused WITH the carve-out, allowed without it
	sbUngranted               // refused either way: in no tier's grant at all
)

// sbMode is which of three profiles sbTry renders for a probe.
type sbMode int

const (
	sbWalled          sbMode = iota // the shipped render: the allow set and the carve-out
	sbControl                       // the same set with the carve-out removed
	sbStoreGitGranted               // the control plus ranger-base-kfrpv's removed grant
)

func sbRun(t *testing.T, profile, sh string) bool {
	t.Helper()
	ok, _ := sbRunOut(t, profile, sh)
	return ok
}

// sbRunOut is that run with the kernel's own words kept. Nothing is printed
// by default, because in this file a refusal is usually the ANSWER — most
// rows want one. The callers that grade a refusal as a FAILURE print what
// came back, and that is ranger-base-al4n9's other half: `sandbox-exec`
// names the path it refused, the sbUngranted row's message named none, and
// both macos legs carried the same unreadable red for a day while the
// answer was in a string nobody kept.
func sbRunOut(t *testing.T, profile, sh string) (bool, string) {
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
	return err == nil, strings.TrimSpace(string(out))
}

func sbRenderProfile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	sbWrite(t, p, body)
	return p
}

// sbWritableFor is the writable set, the carve-out and the profile name one
// of sbMode's three profiles is rendered from. Split out of sbTry because
// the SPELLING of this set is gradable with no sandbox kernel at all, and
// the box that runs this suite every day has none — the whole of why
// ranger-base-al4n9 was two red macos legs and nothing readable here
// (TestQAProbeProfilesGrantInTheKernelsSpelling is that reading).
func sbWritableFor(t *testing.T, f sbFixture, m sbMode) ([]string, SeatbeltCarveOut, string) {
	t.Helper()
	w := append([]string{}, f.writable(t)...)
	carve := SeatbeltCarveOut{}
	name := "control.sb"
	switch m {
	case sbWalled:
		carve, name = f.carve(t), "walled.sb"
	case sbStoreGitGranted:
		// The pre-ranger-base-kfrpv set: the shipped one plus the store
		// repo's git dirs, from the producer the grant used to come from.
		// The carve-out stays OFF, because what this arm has to show is
		// that the command and the fixture work when the path is granted —
		// and before that bead the deny named `<store>/.git/hooks` too, so
		// the walled spelling would refuse one of the two rows for the
		// deny's sake and prove nothing about the grant.
		name = "storegit.sb"
		g := beadsGitDirs(filepath.Join(f.store, beadsDirName))
		if !writeGranted(g, filepath.Join(f.store, ".git")) {
			t.Fatalf("beadsGitDirs no longer names this store's git dir, so this control is not over ranger-base-kfrpv's grant: %v", g)
		}
		// RESOLVED, and that one call is ranger-base-al4n9. `beadsGitDirs`
		// returns what its CALLER spelled; every production grant reaches
		// the profile through `SeatbeltWritable`'s own `add`, which is
		// `absResolve` — "the path the sandbox will match on". This arm
		// appended the producer's raw output, so on darwin, where the
		// fixture root is behind /var -> /private/var, the third profile
		// granted `/var/folders/…/store/.git` and the kernel matched
		// `/private/var/folders/…/store/.git`: no match, write refused,
		// and line 518 read as "the command or the fixture is broken".
		// The guard above cannot see it and must not try — `writeGranted`
		// resolves BOTH sides on purpose (underDir's own comment), so the
		// set-level question answered yes while the kernel answered no.
		// The two rows were red on both macos legs from ranger-base-jyqk4
		// landing until this line, and green on every linux leg, where
		// t.TempDir() is behind no symlink and the raw spelling happened
		// to be the resolved one.
		for _, p := range g {
			w = append(w, absResolve(p))
		}
	}
	// Whatever the arm: what reaches SeatbeltProfile is in the spelling the
	// kernel matches. A set built from a producer nobody resolved is this
	// bead, and the next arm that appends one lands here instead of on a
	// macos leg a week later.
	for _, p := range w {
		if r := absResolve(p); r != p {
			t.Fatalf("the %s set grants %q, which the kernel will match as %q — a grant has to be spelled resolved or it grants nothing (ranger-base-al4n9)", name, p, r)
		}
	}
	return w, carve, name
}

// sbTry renders one of sbMode's three profiles for a FRESH fixture and
// reports whether the write was allowed, with what the kernel said about it
// for a caller that grades the answer a failure.
//
// The diagnostic carries the profile BODY and not its path: the path is
// under a t.TempDir that is gone by the time anyone reads the log, and the
// body is the one thing that answers the question this bead had to be filed
// to ask — which paths the arm granted, in which spelling.
func sbTry(t *testing.T, p sbProbe, m sbMode) (bool, string) {
	t.Helper()
	f := sbNewFixture(t)
	w, carve, name := sbWritableFor(t, f, m)
	body := SeatbeltProfile("developer", w, nil, carve)
	sh := p.sh(f)
	ok, out := sbRunOut(t, sbRenderProfile(t, name, body), sh)
	return ok, fmt.Sprintf("  arm:     %s\n  command: %s\n  kernel:  %s\n  profile:\n%s", name, sh, out, body)
}

// ranger-base-al4n9, read from a box with no seatbelt kernel: the third
// profile's grant graded by its SPELLING. The execution rows above can only
// be read on a macos leg — sbSkipUnlessSandboxable gates the whole file off
// every linux leg — so the arm that was wrong was wrong where nobody could
// see it, for a reason that needs no kernel to state.
func TestQAProbeProfilesGrantInTheKernelsSpelling(t *testing.T) {
	f := sbNewFixture(t)
	// The discriminating premise is PLANTED, not waited for. On darwin the
	// fixture root is already behind a symlink (t.TempDir() is under /var,
	// the tree is at /private/var), which is why this was a macos-only red;
	// on linux it is not, so without a link of our own this pin would be
	// green over the bug — the shape this file calls "a green suite over no
	// wall at all".
	link := filepath.Join(filepath.Dir(f.store), "store-link")
	if err := os.Symlink(f.store, link); err != nil {
		t.Fatal(err)
	}
	through := f // the same fixture, its store named the other way
	through.store = link
	// THE PREMISE, ASSERTED BEFORE ANYTHING IS MEASURED (ranger-base-jielm,
	// from the ranger-base-q2peg verify). Everything this test says about
	// spelling rests on the one assignment above: the set under test is
	// built from `through.store`, so a `through.store` that is the
	// fixture's own store leaves the row below green over the exact bug
	// this pin was written for — MEASURED 2026-11-21 at d28d7c45, ok with
	// `through.store = f.store` and the unresolved append put back, and ok
	// with that one edit alone.
	//
	// The control this replaces was `filepath.Join(link, ".git") != want`,
	// placed AFTER the measurement: it witnesses that the LINK EXISTS, and
	// is true whatever `through.store` holds — which is why nothing stood
	// over the assignment. Both arms below are written in terms of the
	// field sbWritableFor was handed: the store it built from is a SECOND
	// name for the fixture's store, and that name is not the one the kernel
	// matches on. Fatal and not Errorf: a row that has stopped
	// discriminating must not also report on what it no longer measures.
	if through.store == f.store {
		t.Fatalf("the set under test is built from the fixture's own store (%s) and not from the planted link, so this test measures nothing about spelling", through.store)
	}
	if raw := filepath.Join(through.store, ".git"); absResolve(raw) == raw {
		t.Fatalf("the set under test is built from %s, which is already spelled the way the kernel matches, so this test measures nothing about spelling", raw)
	}
	// sbWritableFor fatals on an unresolved entry; this is the POSITIVE
	// half, because a set that simply dropped the grant would satisfy that
	// guard and leave the third profile proving nothing.
	w, _, name := sbWritableFor(t, through, sbStoreGitGranted)
	want := absResolve(filepath.Join(f.store, ".git"))
	if !sbHas(w, want) {
		t.Errorf("the %s set does not grant %s — ranger-base-kfrpv's grant has to be in it, spelled the way the kernel matches:\n  %s", name, want, strings.Join(w, "\n  "))
	}
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
		{"write a PID into the constitution", func(f sbFixture) string { return "touch " + home(f, "agents", "PWNED.md") }, sbCarveDeny},
		{"rewrite an existing PID in place", func(f sbFixture) string { return "sed -i '' s/x/y/ " + home(f, "agents", "developer.md") }, sbCarveDeny},
		{"append to config.yaml", func(f sbFixture) string { return "echo x >> " + home(f, "config.yaml") }, sbCarveDeny},
		{"truncate the promote manifest", func(f sbFixture) string { return ": > " + home(f, PromoteManifestFile) }, sbCarveDeny},
		// ranger-base-bejb / ranger-base-zio33: the truncate above is the
		// MODIFY half. ADR 0015 §3's amended anchor bullet claims the other
		// half too — at seatbelt a caged session can neither modify the
		// promoted copy "nor remove the thing that would notice" — and
		// removal is the half that matters there: a TRUNCATED manifest fails
		// loud (DEGRADED), an ABSENT one reads as "never promoted — OK" and
		// silences the detector with no output on any surface. Both spellings
		// an operator would reach for, executed, because a deny read off
		// profile text is a deny nobody has watched refuse anything.
		{"delete the promote manifest", func(f sbFixture) string { return "rm " + home(f, PromoteManifestFile) }, sbCarveDeny},
		{"move the promote manifest aside", func(f sbFixture) string {
			return "mv " + home(f, PromoteManifestFile) + " " + home(f, PromoteManifestFile+".bak")
		}, sbCarveDeny},
		{"write an env secret", func(f sbFixture) string { return "echo K=v >> " + home(f, "envs", "default.env") }, sbCarveDeny},
		{"delete a promoted directory", func(f sbFixture) string { return "rm -rf " + home(f, "recipes") }, sbCarveDeny},
		{"rewrite ANOTHER persona's L1 shim", func(f sbFixture) string {
			return "echo pwn > " + filepath.Join(f.a.StateDir, "gates", "devops", "bin", "security")
		}, sbCarveDeny},
		{"rewrite this session's own L1 shim", func(f sbFixture) string { return "echo pwn > " + filepath.Join(f.gates, "bin", "git") }, sbCarveDeny},
		{"rewrite the rendered seatbelt profile", func(f sbFixture) string { return "echo pwn > " + filepath.Join(f.gates, "seatbelt.sb") }, sbCarveDeny},
		{"plant an L3 hook", func(f sbFixture) string {
			return "echo 'exit 0' > " + filepath.Join(f.repo, ".git", "hooks", "pre-push")
		}, sbCarveDeny},
		// AND THE STORE OF RECORD'S OWN HOOKS SLOT IS THE OTHER ANSWER
		// (ranger-base-kfrpv): it was a carve-out deny over a grant, and
		// since that bead the grant is gone, so the deny is gone with it and
		// the refusal is the absent grant's. It stayed a sbCarveDeny row
		// here for a day because this test is the one pin in the file that
		// only a darwin host runs, so its control read `the CONTROL refused
		// it too` on every macos leg and nowhere else (ranger-base-jyqk4).
		{"plant an L3 hook in the store of record", func(f sbFixture) string {
			return "echo 'exit 0' > " + filepath.Join(f.store, ".git", "hooks", "prepare-commit-msg")
		}, sbUngranted},
		// The home is reached through a symlink; what a rename would carry
		// away is the REAL directory, which is what the seal names.
		{"rename the home out from under the deny", func(f sbFixture) string {
			return "mv " + filepath.Join(f.repo, ConstitutionSourceDir) + " " + filepath.Join(f.repo, ConstitutionSourceDir+"2")
		}, sbCarveDeny},
		{"rename state/ out from under the gates deny", func(f sbFixture) string {
			return "mv " + filepath.Join(f.repo, ConstitutionSourceDir, "state") + " " + filepath.Join(f.repo, ConstitutionSourceDir, "state2")
		}, sbCarveDeny},
		{"hardlink a PID out and write through it", func(f sbFixture) string {
			return "ln " + home(f, "agents", "developer.md") + " " + filepath.Join(f.repo, "hard.md") + " && echo pwn >> " + filepath.Join(f.repo, "hard.md")
		}, sbCarveDeny},
		{"reach the constitution through the other spelling", func(f sbFixture) string {
			return "touch " + filepath.Join(f.repo, ConstitutionSourceDir, "agents", "PWNED2.md")
		}, sbCarveDeny},

		// And what a session must keep. These are ALLOW under the carve-out
		// too, so they are not controls — they are the cost check.
		{"project work next to the constitution", func(f sbFixture) string {
			return "touch " + filepath.Join(f.repo, ConstitutionSourceDir, "NOTES-not-promoted.md")
		}, sbAllow},
		{"the repo the session was dispatched into", func(f sbFixture) string { return "echo x >> " + filepath.Join(f.repo, "README") }, sbAllow},
		{"its own memory (§5)", func(f sbFixture) string { return "echo x >> " + filepath.Join(f.ag.MemoryDir, "ORDERS.md") }, sbAllow},
		{"L1's audit trail, created on first append", func(f sbFixture) string {
			return "echo refusal >> " + filepath.Join(f.gates, "refusals.log")
		}, sbAllow},
		{"the gate shell's log", func(f sbFixture) string { return "echo line >> " + filepath.Join(f.gates, "shell.log") }, sbAllow},
		{"the record stage: the store of record", func(f sbFixture) string {
			return "touch " + filepath.Join(f.store, beadsDirName, "beads.db")
		}, sbAllow},
		{"posse's own state beside the gates", func(f sbFixture) string { return "touch " + filepath.Join(f.a.StateDir, "probe") }, sbAllow},

		// AND WHAT THE RECORD STAGE DOES NOT WRITE. `<store>/.git/index.lock`
		// was in the list above, for a commit of the JSONL that no seat
		// makes — the launcher makes it, uncaged (ranger-base-kfrpv,
		// App.CommitQueueJSONL). It is here rather than deleted because the
		// cost check and the deny are the same question from two sides, and
		// a path this test used to call a cost is the one a reader of
		// `TestQACarveOutLeavesTheStoreOfRecordWritable`'s own history will
		// come looking for.
		{"the record stage does not commit: the store's git", func(f sbFixture) string {
			return "touch " + filepath.Join(f.store, ".git", "index.lock")
		}, sbUngranted},
	}
	verb := map[bool]string{true: "ALLOWED", false: "REFUSED"}
	for _, p := range probes {
		t.Run(p.what, func(t *testing.T) {
			got, diag := sbTry(t, p, sbWalled)
			if got != (p.want == sbAllow) {
				t.Errorf("%s under the carve-out, want %s\n%s", verb[got], verb[p.want == sbAllow], diag)
			}
			if p.want == sbAllow {
				return
			}
			ctrl, cdiag := sbTry(t, p, sbControl)
			if p.want == sbCarveDeny {
				if !ctrl {
					t.Errorf("the CONTROL refused it too — the probe proves nothing about the deny\n%s", cdiag)
				}
				return
			}
			// sbUngranted: the refusal must be the absent grant and not the
			// deny, so the control refuses it too — and the arm below is
			// what stops that from being vacuous.
			if ctrl {
				t.Errorf("the control ALLOWED it, so the carve-out's deny is what refuses this path: the row is an sbCarveDeny and not an absent grant\n%s", cdiag)
			}
			if ok, gdiag := sbTry(t, p, sbStoreGitGranted); !ok {
				t.Errorf("refused even with ranger-base-kfrpv's grant put back — the command or the fixture is what the two refusals above measured, not the wall\n%s", gdiag)
			}
		})
	}
}
