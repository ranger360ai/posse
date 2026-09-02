package posse

// ADR 0038 (ranger-base-vqyxl, folding ranger-base-65po1): `.git/config`
// and a linked worktree's identity chain are the persistent state that
// tells a LATER, UNSANDBOXED git which code to run — the operator's daily
// git in the checkout, the next launch's L3 probe, and the launcher's own
// `git -C <worktree> rebase` at land time (worktree.go). The hooks deny
// alone was unsound and ADR 0023 non-goal 3 said so: `core.hooksPath` in a
// writable config moves the slot, and plain `git config core.hooksPath …`
// is not a bd verb, so no PID denies that spelling.
//
// So this file pins the other half, and pins it the way its neighbours pin
// theirs — by EXECUTION under sandbox-exec, every refusal run twice: once
// under the real profile and once, on a FRESH fixture, under the same
// profile with ONLY the new denies removed. A refusal proves nothing unless
// the same command succeeds with the deny gone, and the control's success
// is also the witness that the file it wrote to was really there.
//
// The control is built by SUBTRACTION from the real carve-out
// (giFixture.control) rather than assembled by hand: a control list written
// out separately would be measuring two changes at once.
//
// One shape has no control, and the table says so rather than reading as a
// failure: in a WORKTREE session no grant reaches `<common>/config` at all
// (ranger-base-m2wf), so the control is refused there too and those rows
// grade the omission wall the ADR's Context names instead of the deny.
// giFixture.reachesConfig asks the production grant which of the two a row
// is grading, and the row is asserted the other way round so a widened
// grant fails it. Everything a probe needs BEFORE the write it is named for
// is a giProbe.stage, run under the same profile and required to succeed —
// a setup refused by an unrelated wall is a refusal that reads as a pass
// (both measured, ranger-base-1fz21).
//
// Verification items 1-4 of the ADR, in order:
//
//	item 1 → TestQAGitConfigWriteRefusedUnderSandboxExec, per session shape
//	item 2 → the non-git spellings in that same table
//	item 3 → TestQASessionLifeStaysGreenUnderTheConfigDeny
//	item 4 → TestQAWorktreeIdentityChainRefusedButTheLauncherIsUnaffected
//
// Item 5 is L4 (CageMounts) and is not this bead's.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the fixture: all four session shapes over one repo ──────────────────────

// giShape is one of the four the ADR's item 1 enumerates. cwd and the PID
// are what actually vary; everything else about the fixture is shared, so a
// difference between two rows is a difference in the profile and not in the
// tree.
type giShape struct {
	name string
	cwd  func(f giFixture) string
	ag   func(f giFixture) *AgentFile
}

func giShapes() []giShape {
	return []giShape{
		{"main checkout", func(f giFixture) string { return f.repo }, func(f giFixture) *AgentFile { return f.ag }},
		{"worktree", func(f giFixture) string { return f.tree }, func(f giFixture) *AgentFile { return f.ag }},
		// A PID that denies file writes is granted `<cwd>/.git` WHOLE
		// instead of cwd (SeatbeltWritable) — a different route to the same
		// config file, so it gets its own row.
		{"deniesFiles", func(f giFixture) string { return f.repo }, func(f giFixture) *AgentFile { return f.denied }},
		// cwd's `.beads` redirects into another repo, whose `.git` is
		// granted whole by beadsGitDirs. That repo's config is the second
		// file sessionGitConfigFiles names.
		{"redirect", func(f giFixture) string { return f.work }, func(f giFixture) *AgentFile { return f.ag }},
	}
}

type giFixture struct {
	a                 *App
	repo, tree, store string // the checkout, a linked worktree of it, the store of record
	work              string // a second repo whose .beads redirects into the store
	own, common       string // the worktree's per-worktree git dir, and the shared one
	gates             string
	ag, denied        *AgentFile
}

func giNewFixture(t *testing.T) giFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := sbRoot(t) // HOME elsewhere, TMPDIR a sibling: nothing here is granted by accident
	repo := sbMkdir(t, filepath.Join(root, "repo"))
	store := sbMkdir(t, filepath.Join(root, "store"))
	work := sbMkdir(t, filepath.Join(root, "work"))
	for _, r := range []string{repo, store, work} {
		sbGitInit(t, r)
		// The identity a caged commit needs is in the config BEFORE the
		// profile is rendered, which is the live shape: the operator sets
		// it once and every worktree reads it from the common config. A
		// fixture that set it later would be measuring the deny against a
		// repo no session could commit in anyway.
		mustGit(t, r, "config", "user.email", "t@example.com")
		mustGit(t, r, "config", "user.name", "t")
		sbWrite(t, filepath.Join(r, "README"), "seed\n")
		mustGit(t, r, "add", "README")
		mustGit(t, r, "commit", "-q", "-m", "seed")
	}
	// The store of record, reached by a redirect the way ~/src/posse
	// reaches ~/src/ranger-base/.beads.
	sbMkdir(t, filepath.Join(store, beadsDirName))
	sbMkdir(t, filepath.Join(work, beadsDirName))
	sbWrite(t, filepath.Join(work, beadsDirName, beadsRedirect), filepath.Join(store, beadsDirName)+"\n")

	tree := filepath.Join(root, "trees", "developer")
	mustGit(t, repo, "worktree", "add", "-q", "-b", "posse/developer-probe", tree)
	dirs := LinkedGitDirs(tree)
	if len(dirs) != 2 {
		t.Fatalf("no linked worktree was made — the identity-chain half asserts nothing: LinkedGitDirs(%s) = %v", tree, dirs)
	}

	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	f := giFixture{
		a: a, repo: repo, tree: tree, store: store, work: work,
		own: absResolve(dirs[0]), common: absResolve(dirs[1]),
		gates: sbMkdir(t, a.GatesDir("developer")),
	}
	f.ag = &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(a.PersonasDir(), "developer"))}
	f.denied = &AgentFile{Name: "developer", Deny: []string{"Edit", "Write"}, MemoryDir: f.ag.MemoryDir}
	return f
}

func (f giFixture) writable(t *testing.T, s giShape) []string {
	t.Helper()
	return f.a.SeatbeltWritable(s.ag(f), s.cwd(f), f.gates)
}

func (f giFixture) carve(t *testing.T, s giShape) SeatbeltCarveOut {
	t.Helper()
	return f.a.SeatbeltCarveOut(s.ag(f), s.cwd(f), f.gates, f.writable(t, s))
}

// added is exactly what this bead put in the deny list, resolved the way
// the carve-out resolves it. One list, read by both the assertions and the
// control, so a helper that drifted from the production one would fail the
// premise checks first.
func (f giFixture) added(s giShape) []string {
	cwd := s.cwd(f)
	var out []string
	for _, p := range append(sessionGitConfigFiles(cwd), sessionWorktreeIdentityFiles(cwd)...) {
		out = append(out, absResolve(p))
	}
	return dedupeStrings(out)
}

// control is the carve-out AS IT WAS: everything else identical, with only
// this bead's entries taken back out. Built by subtraction so the two arms
// differ in one thing.
func (f giFixture) control(t *testing.T, s giShape) SeatbeltCarveOut {
	t.Helper()
	c := f.carve(t, s)
	drop := map[string]bool{}
	for _, p := range f.added(s) {
		drop[p] = true
	}
	var keep []string
	for _, p := range c.Deny {
		if !drop[p] {
			keep = append(keep, p)
		}
	}
	if len(keep) == len(c.Deny) {
		t.Fatalf("the control is identical to the real carve-out — this bead added nothing for %s to measure: %v", s.name, c.Deny)
	}
	c.Deny = keep
	c.Seal = renameSeal(keep, f.writable(t, s))
	return c
}

// lockControl is a NARROWER subtraction than control above: only the
// `config.lock` siblings come back out, everything else — including the
// config file itself — stays denied. It is the one arm that isolates what
// the `.lock` entry buys on its own, which the wide control cannot: with
// the config denied and the lock allowed, git takes the lock, writes the
// whole new config into it and fails one step later at the rename. The
// Fatal below is load-bearing: drop `c+".lock"` from sessionGitConfigFiles
// and this arm has nothing to take out, which is the mutant the old
// stray-lock assertion survived (ranger-base-xwepd).
func (f giFixture) lockControl(t *testing.T, s giShape) SeatbeltCarveOut {
	t.Helper()
	drop := map[string]bool{}
	for _, p := range sessionGitConfigFiles(s.cwd(f)) {
		if strings.HasSuffix(p, ".lock") {
			drop[absResolve(p)] = true
		}
	}
	if len(drop) == 0 {
		t.Fatalf("sessionGitConfigFiles(%s) names no .lock sibling, so the arm that grades one has nothing to take out — the deny it is named for is gone", s.cwd(f))
	}
	c := f.carve(t, s)
	var keep []string
	for _, p := range c.Deny {
		if !drop[p] {
			keep = append(keep, p)
		}
	}
	if len(keep) == len(c.Deny) {
		t.Fatalf("the %s shape's carve-out denies no %v — the lock control is identical to the real one and measures nothing: %v", s.name, drop, c.Deny)
	}
	c.Deny = keep
	c.Seal = renameSeal(keep, f.writable(t, s))
	return c
}

func (f giFixture) configOf(t *testing.T, dir string) string {
	t.Helper()
	p, err := gitPath(dir, "config")
	if err != nil {
		t.Fatal(err)
	}
	return absResolve(p)
}

// forged is where a probe stages a file it will then rename over a denied
// one. The persona's own memory dir, because that is what a caged session
// really has to forge a file in: SeatbeltWritable grants ag.MemoryDir in
// all four shapes, and `personas` is NotPromoted so no constitution deny
// covers it. NOT the gates dir, which is where this started — the carve-out
// denies `state/gates` whole in BOTH arms (seatbeltcarveout_qa_test.go pins
// that as intended), so a probe staging there is refused before it begins
// (ranger-base-1fz21).
func (f giFixture) forged() string { return filepath.Join(f.ag.MemoryDir, "forged") }

// reachesConfig asks the PRODUCTION grant whether a session of this shape
// can write the config file at all. In the worktree shape the answer is no:
// ranger-base-m2wf narrowed the common git dir out of the grant, so the
// file is walled by omission and ADR 0038's deny sits on top of a wall that
// was already there ("only by omission — no deny stands there if a grant
// ever widens"). No control arm can succeed against that, which is why the
// execution table below grades the omission wall in that shape and says so
// instead of reporting an untested deny.
func (f giFixture) reachesConfig(t *testing.T, s giShape) bool {
	t.Helper()
	return sbCovers(f.writable(t, s), f.configOf(t, s.cwd(f)))
}

// ─── the deny list ───────────────────────────────────────────────────────────

// The premise, as a test rather than a paragraph: in every shape the ALLOW
// block reaches the config file this bead denies. If a grant ever narrows
// so that it does not, the execution table below stops proving anything and
// this says so first.
func TestQAGrantStillReachesTheGitConfigInEveryShape(t *testing.T) {
	for _, s := range giShapes() {
		t.Run(s.name, func(t *testing.T) {
			f := giNewFixture(t)
			w := f.writable(t, s)
			cfg := f.configOf(t, s.cwd(f))
			if s.name == "worktree" {
				// The one shape where the grant does NOT reach it:
				// ranger-base-m2wf narrowed the common dir away, so the
				// deny stands over a structural wall — which is exactly
				// what ADR 0038 says it is there for ("only by omission —
				// no deny stands there if a grant ever widens").
				if sbCovers(w, cfg) {
					t.Errorf("the narrowed worktree grant reaches %s again — ranger-base-m2wf regressed:\n  %s", cfg, strings.Join(w, "\n  "))
				}
				// Its identity chain IS granted, though, and that is the
				// half this shape measures.
				for _, p := range sessionWorktreeIdentityFiles(f.tree) {
					if !sbCovers(w, p) {
						t.Errorf("premise gone: %s is in no grant, so denying it measures nothing:\n  %s", p, strings.Join(w, "\n  "))
					}
				}
				return
			}
			if !sbCovers(w, cfg) {
				t.Errorf("premise gone: %s is in no grant, so denying it measures nothing:\n  %s", cfg, strings.Join(w, "\n  "))
			}
		})
	}
}

// ADR 0038 decision 1, in the profile: the config file and its lock, for
// BOTH repos a session writes, in every shape.
func TestQACarveOutDeniesTheGitConfigAndItsLock(t *testing.T) {
	for _, s := range giShapes() {
		t.Run(s.name, func(t *testing.T) {
			f := giNewFixture(t)
			c := f.carve(t, s)
			cfg := f.configOf(t, s.cwd(f))
			for _, p := range []string{cfg, cfg + ".lock"} {
				if !sbDenied(c, p) {
					t.Errorf("%s is not in the carve-out: %v", p, c.Deny)
				}
			}
			if s.name == "redirect" {
				// The store of record's config is the second file: its
				// `.git` is granted whole (beadsGitDirs), and a
				// core.hooksPath planted there redirects the slot that
				// stamps the beads visibility guard.
				for _, p := range []string{f.configOf(t, f.store), f.configOf(t, f.store) + ".lock"} {
					if !sbDenied(c, p) {
						t.Errorf("the store of record's %s is not denied: %v", p, c.Deny)
					}
				}
			}
			// Enumerated at the artifact level: everything else in the git
			// dir a session legitimately writes stays writable. Asked of
			// git for the same reason the deny is — in a worktree the
			// index is in the PER-WORKTREE dir and `<tree>/.git` is a
			// file, so a joined path would be checking a path that cannot
			// exist and calling it a pass.
			idx, err := gitPath(s.cwd(f), "index")
			if err != nil {
				t.Fatal(err)
			}
			ce, err := gitPath(s.cwd(f), "COMMIT_EDITMSG")
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range []string{idx, idx + ".lock", ce} {
				if sbDenied(c, p) {
					t.Errorf("the deny is not artifact-level: %s is denied by %v", p, c.Deny)
				}
			}
		})
	}
}

// Asked of git, never derived (the hooksDir doctrine). The measurement that
// makes it matter: from a linked worktree the config git reads is the
// COMMON one, so a path joined onto the per-worktree git dir would have
// denied a file no git reads while leaving the real one writable.
func TestQAConfigDenyIsGitsAnswerAndNotAJoin(t *testing.T) {
	f := giNewFixture(t)
	got := sessionGitConfigFiles(f.tree)
	want := filepath.Join(f.common, "config")
	if len(got) == 0 || absResolve(got[0]) != absResolve(want) {
		t.Fatalf("sessionGitConfigFiles(worktree) = %v, want the COMMON config %s first", got, want)
	}
	if absResolve(got[1]) != absResolve(want+".lock") {
		t.Errorf("the lock must be the sibling of the answered path: %v", got)
	}
	if derived := filepath.Join(f.own, "config"); absResolve(got[0]) == absResolve(derived) {
		t.Errorf("the deny landed on the per-worktree config, which no git reads: %s", derived)
	}
}

// ADR 0038 decision 2: the identity chain, and only for a linked worktree.
func TestQACarveOutDeniesTheWorktreeIdentityChain(t *testing.T) {
	f := giNewFixture(t)
	s := giShapes()[1] // worktree
	c := f.carve(t, s)
	for _, p := range []string{
		filepath.Join(f.tree, ".git"), // the pointer FILE
		filepath.Join(f.own, "gitdir"),
		filepath.Join(f.own, "commondir"),
		filepath.Join(f.own, "config.worktree"),
		// The lock sibling decision 1 had and decision 2 did not
		// (ranger-base-xwepd). Its execution row is below.
		filepath.Join(f.own, "config.worktree.lock"),
	} {
		if !sbDenied(c, p) {
			t.Errorf("%s is not in the carve-out: %v", p, c.Deny)
		}
	}
	// What a session must keep in that same directory, which is granted
	// whole precisely because a commit writes all of it.
	for _, p := range []string{
		filepath.Join(f.own, "index"),
		filepath.Join(f.own, "index.lock"),
		filepath.Join(f.own, "HEAD"),
		filepath.Join(f.own, "ORIG_HEAD"),
		filepath.Join(f.own, "COMMIT_EDITMSG"),
	} {
		if sbDenied(c, p) {
			t.Errorf("a worktree commit needs %s; it is denied by %v", p, c.Deny)
		}
	}
}

// A main checkout has no pointer file and no gitdir/commondir — there is
// nothing to deny, and denying a name that does not exist there would be
// deny-list growth for nothing.
func TestQAMainCheckoutHasNoIdentityChainLiterals(t *testing.T) {
	f := giNewFixture(t)
	if got := sessionWorktreeIdentityFiles(f.repo); got != nil {
		t.Errorf("sessionWorktreeIdentityFiles(main checkout) = %v, want none", got)
	}
	// It still gets the config deny, which is the half that applies there.
	if got := sessionGitConfigFiles(f.repo); len(got) != 2 {
		t.Errorf("sessionGitConfigFiles(main checkout) = %v, want the config and its lock", got)
	}
}

// The deny grew; the SEAL must not. Every path added here sits under a
// directory the hooks deny already sealed (a main checkout's `.git`) or
// under one no grant covers the parent of (a worktree's own git dir), so
// the rename escape is already closed and a new seal entry would be a line
// an operator has to re-check for nothing.
func TestQAConfigDenyAddsNoNewRenameSeal(t *testing.T) {
	for _, s := range giShapes() {
		t.Run(s.name, func(t *testing.T) {
			f := giNewFixture(t)
			before, after := f.control(t, s).Seal, f.carve(t, s).Seal
			if strings.Join(before, "\n") != strings.Join(after, "\n") {
				t.Errorf("the seal changed: was %v, now %v", before, after)
			}
		})
	}
}

// The operator-readable half (ADR 0038 §5: "posse gates prints each entry
// as an `x` line, so the wall stays readable"). A deny only the kernel
// knows about is a deny nobody checks.
func TestQAGatesPrintsTheConfigDeny(t *testing.T) {
	f := giNewFixture(t)
	var b strings.Builder
	if err := f.a.SeatbeltReport(f.ag, f.tree, &b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, p := range []string{
		filepath.Join(f.common, "config"),
		filepath.Join(f.common, "config.lock"),
		filepath.Join(f.tree, ".git"),
		filepath.Join(f.own, "commondir"),
	} {
		want := "    x " + AbbrevHome(absResolve(p)) + " (trailing deny"
		if !strings.Contains(got, want) {
			t.Errorf("the gates report does not print %q:\n%s", want, got)
		}
	}
}

// ─── the execution half ──────────────────────────────────────────────────────

// giProbe is one write, run under a rendered profile. `want` is what must
// happen WITH this bead's denies in place; a refusal is then re-run on a
// fresh fixture under the control profile, and the control MUST succeed or
// the probe measured nothing.
//
// `stage` is the setup a multi-step probe needs. It runs under the SAME
// profile as the write — the attacker is the caged session, so its staging
// is caged too — and it is required to SUCCEED. That requirement is the
// point: a probe refused at its first step exits non-zero exactly like one
// refused at the wall, and under `want: false` that reads as a pass. The mv
// probe below staged its forged config inside `state/gates`, which the
// carve-out denies in every arm, so in all four shapes it never reached the
// mv it is named for; the only symptom was its control arm failing, which
// says "untested deny" and not "this probe never ran" (ranger-base-1fz21).
type giProbe struct {
	what    string
	stage   func(f giFixture) string
	sh      func(f giFixture, cwd string) string
	want    bool
	witness func(t *testing.T, f giFixture, cwd string) // run when the control succeeds
}

// giTry runs one arm on a fresh fixture and hands that fixture back: the
// reachability question the config table asks below has to be asked of the
// same tree the refusal came from, not of another one built beside it.
func giTry(t *testing.T, s giShape, p giProbe, walled bool) (giFixture, bool, string) {
	t.Helper()
	f := giNewFixture(t)
	cwd := s.cwd(f)
	c, name := f.control(t, s), "control.sb"
	if walled {
		c, name = f.carve(t, s), "walled.sb"
	}
	w := f.writable(t, s)
	prof := sbRenderProfile(t, name, SeatbeltProfile("developer", w, SeatbeltSiblings(nil, c), c, sessionRefDirs(cwd)...))
	if p.stage != nil {
		if ok, out := wgRun(t, prof, p.stage(f)); !ok {
			t.Fatalf("the probe's own setup was refused under %s, so the write it is named for never ran and this arm measures nothing (ranger-base-1fz21):\n%s", name, out)
		}
	}
	ok, out := wgRun(t, prof, p.sh(f, cwd))
	if ok && !walled && p.witness != nil {
		p.witness(t, f, cwd)
	}
	return f, ok, out
}

// ADR 0038 verification items 1 and 2: `git config` refused in every
// session shape, config byte-identical, no stray `config.lock` — and the
// non-git spellings refused too, because a wall that only stops the `git`
// binary is a wall around one spelling.
func TestQAGitConfigWriteRefusedUnderSandboxExec(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	cfg := func(f giFixture, cwd string) string { return f.configOf(t, cwd) }
	planted := func(f giFixture, cwd string) bool {
		b, _ := os.ReadFile(cfg(f, cwd))
		return strings.Contains(string(b), "hooksPath") || strings.Contains(string(b), "PWNED")
	}
	probes := []giProbe{
		// Item 1. The door ADR 0038 closes: not a bd verb, so no PID
		// denies this spelling, and the value survives the invocation to
		// be read by the operator's git and the launcher's rebase.
		{what: "plant core.hooksPath with git config", sh: func(f giFixture, cwd string) string {
			return "git -C " + cwd + " config core.hooksPath /tmp/x"
		}, want: false, witness: func(t *testing.T, f giFixture, cwd string) {
			if !planted(f, cwd) {
				t.Error("the control did not plant hooksPath — it witnesses nothing")
			}
		}},
		{what: "plant a filter.*.clean command", sh: func(f giFixture, cwd string) string {
			return "git -C " + cwd + " config filter.PWNED.clean 'sh -c id'"
		}, want: false, witness: func(t *testing.T, f giFixture, cwd string) {
			if !planted(f, cwd) {
				t.Error("the control did not plant the filter — it witnesses nothing")
			}
		}},
		{what: "plant an alias", sh: func(f giFixture, cwd string) string {
			return "git -C " + cwd + " config alias.PWNED '!sh -c id'"
		}, want: false},
		// Item 2: the non-git spellings.
		{what: "append to config with a shell redirect", sh: func(f giFixture, cwd string) string {
			return "printf '[core]\\n\\thooksPath = /tmp/x\\n' >> " + cfg(f, cwd)
		}, want: false, witness: func(t *testing.T, f giFixture, cwd string) {
			if !planted(f, cwd) {
				t.Error("the control did not append to config — it witnesses nothing")
			}
		}},
		{what: "rewrite config through python", sh: func(f giFixture, cwd string) string {
			return "/usr/bin/python3 -c \"open('" + cfg(f, cwd) + "','a').write('[core]\\n\\thooksPath = /tmp/x\\n')\""
		}, want: false},
		// The staging is its own step for a reason: see giProbe.stage.
		{what: "mv a forged config onto it", stage: func(f giFixture) string {
			return "printf '[core]\\n\\thooksPath = /tmp/x\\n' > " + f.forged()
		}, sh: func(f giFixture, cwd string) string {
			return "mv " + f.forged() + " " + cfg(f, cwd)
		}, want: false, witness: func(t *testing.T, f giFixture, cwd string) {
			if !planted(f, cwd) {
				t.Error("the control did not move the forged config into place — it witnesses nothing")
			}
		}},
		{what: "truncate config", sh: func(f giFixture, cwd string) string { return ": > " + cfg(f, cwd) }, want: false},
		{what: "delete config", sh: func(f giFixture, cwd string) string { return "rm " + cfg(f, cwd) }, want: false},
	}
	verb := map[bool]string{true: "ALLOWED", false: "REFUSED"}
	for _, s := range giShapes() {
		for _, p := range probes {
			t.Run(s.name+"/"+p.what, func(t *testing.T) {
				f, got, out := giTry(t, s, p, true)
				if got != p.want {
					t.Errorf("%s under the deny, want %s:\n%s", verb[got], verb[p.want], out)
				}
				_, ok, cout := giTry(t, s, p, false)
				if f.reachesConfig(t, s) {
					if !ok {
						t.Errorf("the CONTROL refused it too — the probe proves nothing about the deny:\n%s", cout)
					}
					return
				}
				// The shape whose grant does not reach this config at all
				// (worktree, ranger-base-m2wf). Both arms are refused by
				// the MISSING GRANT, so no control can succeed here and
				// this row grades the omission wall — which is a real
				// property, and the one ADR 0038 rests on in this shape —
				// rather than the deny. Saying which is the whole fix:
				// eight rows here reported "the CONTROL refused it too" and
				// read as an untested deny (ranger-base-1fz21).
				//
				// The assertion runs the other way round, so the row is
				// still falsifiable: the day a grant widens to cover this
				// config, the control succeeds, this line fails, and the
				// row has to go back to grading the deny.
				if ok {
					t.Errorf("the CONTROL was ALLOWED: a grant now reaches the %s shape's config, so the omission wall this row grades is gone and it must grade the DENY instead — require the control to succeed for this shape:\n%s", s.name, cout)
					return
				}
				t.Logf("graded against the omission wall, not the deny: no grant of the %s shape reaches this config (ranger-base-m2wf), so no control can succeed and the refusal above is the missing grant's", s.name)
			})
		}
	}
}

// The rest of item 1, and the sentence ADR 0038 decision 1 and the
// sessionGitConfigFiles comment both used to close with. TWO claims lived
// in that sentence and they are not both true (ranger-base-xwepd):
//
// TRUE, and exactly what the `config.lock` entry buys: the refusal lands
// at LOCK CREATION. Under the deny git says "could not lock config file"
// and nothing of the attempted config reaches the disk at all. With ONLY
// the `.lock` siblings taken back out of the same carve-out (lockControl),
// git gets the lock, writes the whole new config into it, and fails one
// step later at "could not write config file". That word is the difference
// the entry makes and the difference a MUTANT of the entry moves, which is
// why this pin asserts the words.
//
// NOT TRUE: "no stray lock in shared state". `git config` on git 2.50.1
// removes its own lock when the rename fails — MEASURED in the lock-control
// arm below, where the entry is gone and no `config.lock` survives either.
// So the stray-lock assertion this pin was named for was an "assert nothing
// bad happened" over a rig that cannot produce the bad thing: it stayed
// green with the entry deleted. It is still asserted here, in BOTH arms — a
// stray lock kills the operator's own `git config` and `git gc` until a
// human removes it (ranger-base-msex), and a killed git or a different
// writer could still strand one — but it is a regression guard now, and the
// control arm says so out loud rather than letting it read as the deny's
// witness.
//
// The worktree shape grades the OMISSION wall, not the deny, for the same
// reason the table above does (ranger-base-m2wf): no grant reaches that
// config, so both arms are refused at lock creation by the missing grant
// and no control can separate them. Asserted the other way round so a
// widened grant fails the row instead of quietly weakening it.
func TestQAConfigLockDenyMovesTheRefusalToLockCreation(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	// run performs the one write under a carve-out and reports what git
	// said and whether a lock was stranded. Both arms go through it so the
	// only difference between them is the deny list.
	run := func(t *testing.T, f giFixture, s giShape, c SeatbeltCarveOut, name string) (out string, stray bool) {
		t.Helper()
		cwd := s.cwd(f)
		cfg := f.configOf(t, cwd)
		before, err := os.ReadFile(cfg)
		if err != nil {
			t.Fatal(err)
		}
		prof := sbRenderProfile(t, name, SeatbeltProfile("developer", f.writable(t, s), nil, c, sessionRefDirs(cwd)...))
		ok, out := wgRun(t, prof, "git -C "+cwd+" config core.hooksPath /tmp/x")
		if ok {
			t.Fatalf("git config was ALLOWED under %s — the config file is denied in BOTH arms, so this is not a lock question:\n%s", name, out)
		}
		after, err := os.ReadFile(cfg)
		if err != nil {
			t.Fatalf("under %s the refusal removed the config file: %v", name, err)
		}
		if string(before) != string(after) {
			t.Errorf("under %s the config changed under a refused write:\nbefore:\n%s\nafter:\n%s", name, before, after)
		}
		_, err = os.Stat(cfg + ".lock")
		return out, err == nil
	}
	for _, s := range giShapes() {
		t.Run(s.name, func(t *testing.T) {
			f := giNewFixture(t)
			out, stray := run(t, f, s, f.carve(t, s), "walled.sb")
			if stray {
				t.Errorf("a stray config.lock survived the refusal under the deny — the operator's own git config and gc now die until a human removes it (ranger-base-msex)")
			}
			if !strings.Contains(out, "could not lock config file") {
				t.Errorf("the refusal does not land at LOCK CREATION, which is the whole claim of the `.lock` entry. git said:\n%s", out)
			}
			if strings.Contains(out, "could not write config file") {
				t.Errorf("git reached the WRITE, so it had already taken the lock and put the whole new config in it — the `.lock` entry is not doing what decision 1 says. git said:\n%s", out)
			}

			// The lock control, on its own fresh fixture: the config still
			// denied, only its lock allowed.
			cf := giNewFixture(t)
			cout, cstray := run(t, cf, s, cf.lockControl(t, s), "nolock.sb")
			if !cf.reachesConfig(t, s) {
				t.Logf("graded against the omission wall, not the deny: no grant of the %s shape reaches this config (ranger-base-m2wf), so allowing the lock changes nothing and git said %q either way", s.name, strings.TrimSpace(cout))
				if strings.Contains(cout, "could not write config file") {
					t.Errorf("the lock control reached the WRITE in a shape whose grant does not reach this config — the omission wall this row grades is gone and it must grade the DENY instead:\n%s", cout)
				}
				return
			}
			if !strings.Contains(cout, "could not write config file") {
				t.Errorf("with the `.lock` sibling allowed the refusal did NOT move to the write — then the entry buys nothing measurable and decision 1 is asserting a difference that is not there. git said:\n%s", cout)
			}
			if cstray {
				t.Errorf("the lock control stranded a config.lock — good news for the stray-lock claim and bad news for this comment: `git config` is no longer cleaning up after itself, so the assertion above is load-bearing again and decision 1's \"no stray lock\" clause is true after all. Re-word both.")
			}
		})
	}
}

// ADR 0038 verification item 3: the deny costs a session nothing. Measured
// by EXECUTION under the real rendered profile — add/commit in the tree,
// a checkout, and the record stage's writes into the store of record
// including the git commit `bd sync` makes there. `bd` itself is not in
// this fixture; what it does to the filesystem is, which is the part a
// path deny can be wrong about.
func TestQASessionLifeStaysGreenUnderTheConfigDeny(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := giNewFixture(t)
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	before := mustGit(t, f.tree, "rev-parse", "HEAD")
	sh := "echo caged >> " + filepath.Join(f.tree, "work.txt") +
		" && git -C " + f.tree + " add work.txt" +
		" && git -C " + f.tree + " commit -q -m 'caged commit' -- work.txt" +
		" && git -C " + f.tree + " checkout -q -- work.txt" +
		" && git -C " + f.tree + " status --porcelain"
	if ok, out := wgRun(t, prof, sh); !ok {
		t.Fatalf("a worktree commit/checkout cycle is refused under the config deny:\n%s\n\nprofile:\n%s", out, mustRead(t, prof))
	}
	after := mustGit(t, f.tree, "rev-parse", "HEAD")
	if after == before {
		t.Fatal("git exited 0 but the branch did not move — nothing was measured")
	}

	// The record stage, from a session whose store of record is elsewhere.
	wf := giNewFixture(t)
	wprof, err := wf.a.RenderSeatbelt(wf.ag, wf.work)
	if err != nil {
		t.Fatal(err)
	}
	jsonl := filepath.Join(wf.store, beadsDirName, "issues.jsonl")
	rec := "touch " + filepath.Join(wf.store, beadsDirName, "beads.db") +
		" && echo '{\"id\":\"x\"}' >> " + jsonl +
		" && git -C " + wf.store + " add -A " + filepath.Join(wf.store, beadsDirName) +
		" && git -C " + wf.store + " commit -q -m 'bd sync' -- " + filepath.Join(wf.store, beadsDirName)
	if ok, out := wgRun(t, wprof, rec); !ok {
		t.Fatalf("the record stage is refused under the config deny — a session that cannot close its bead goes silent:\n%s", out)
	}
	if got := mustGit(t, wf.store, "log", "-1", "--format=%s"); got != "bd sync" {
		t.Errorf("the store commit did not land: last subject %q", got)
	}
}

// ADR 0038 verification item 4. The chain is what selects WHICH config and
// hooks the launcher's own `git -C <worktree> rebase` reads — unsandboxed,
// inside the tree the session just had — so a writable `commondir` walks
// around the config deny for exactly the git that matters most.
//
// Both halves in one test on purpose: the refusals, and then the launcher
// doing its real work in the same tree afterwards. Cost was ASSUMED zero
// for these four literals rather than measured (ADR 0038 item 2 says so and
// asks for this), and the second half is the measurement — if a legitimate
// writer needed one, this is where it would surface and the literal would
// be dropped and recorded.
func TestQAWorktreeIdentityChainRefusedButTheLauncherIsUnaffected(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	s := giShapes()[1] // worktree
	probes := []giProbe{
		// f.gates appears here only as CONTENT — the path a forged chain
		// would point AT. Nothing below writes into it, which is why these
		// rows were unaffected by the staging refusal the config table hit
		// (ranger-base-1fz21).
		{what: "repoint the .git pointer file at a forged git dir", sh: func(f giFixture, cwd string) string {
			return "echo 'gitdir: " + filepath.Join(f.gates, "forged") + "' > " + filepath.Join(cwd, ".git")
		}, want: false, witness: func(t *testing.T, f giFixture, cwd string) {
			b, _ := os.ReadFile(filepath.Join(cwd, ".git"))
			if !strings.Contains(string(b), "forged") {
				t.Errorf("the control did not repoint the pointer file: %q", b)
			}
		}},
		{what: "repoint commondir at a writable dir", sh: func(f giFixture, cwd string) string {
			return "echo " + f.gates + " > " + filepath.Join(f.own, "commondir")
		}, want: false, witness: func(t *testing.T, f giFixture, cwd string) {
			b, _ := os.ReadFile(filepath.Join(f.own, "commondir"))
			if !strings.Contains(string(b), f.gates) {
				t.Errorf("the control did not rewrite commondir: %q", b)
			}
		}},
		{what: "repoint gitdir", sh: func(f giFixture, cwd string) string {
			return "echo " + filepath.Join(f.gates, "elsewhere", ".git") + " > " + filepath.Join(f.own, "gitdir")
		}, want: false},
		{what: "plant per-worktree config.worktree", sh: func(f giFixture, cwd string) string {
			return "printf '[core]\\n\\thooksPath = /tmp/x\\n' > " + filepath.Join(f.own, "config.worktree")
		}, want: false, witness: func(t *testing.T, f giFixture, cwd string) {
			if _, err := os.Stat(filepath.Join(f.own, "config.worktree")); err != nil {
				t.Errorf("the control did not create config.worktree: %v", err)
			}
		}},
		// ranger-base-xwepd, the same sibling reasoning decision 1 already
		// used. git writes config.worktree through a lockfile too, so a
		// session that may create the lock gets the whole attacker-chosen
		// config.worktree onto disk before the rename is refused — the
		// refusal lands at "could not write" instead of "could not lock"
		// (MEASURED, git 2.50.1).
		{what: "plant the config.worktree lock sibling", sh: func(f giFixture, cwd string) string {
			return "printf '[core]\\n\\thooksPath = /tmp/x\\n' > " + filepath.Join(f.own, "config.worktree.lock")
		}, want: false, witness: func(t *testing.T, f giFixture, cwd string) {
			if _, err := os.Stat(filepath.Join(f.own, "config.worktree.lock")); err != nil {
				t.Errorf("the control did not create config.worktree.lock — the row witnesses nothing: %v", err)
			}
		}},
		{what: "delete commondir outright", sh: func(f giFixture, cwd string) string {
			return "rm " + filepath.Join(f.own, "commondir")
		}, want: false},

		// And what the session keeps in the same directory — not controls,
		// the cost check.
		{what: "write its own index", sh: func(f giFixture, cwd string) string {
			return "touch " + filepath.Join(f.own, "index.lock")
		}, want: true},
		{what: "move its own HEAD", sh: func(f giFixture, cwd string) string {
			return "git -C " + cwd + " checkout -q --detach"
		}, want: true},
	}
	verb := map[bool]string{true: "ALLOWED", false: "REFUSED"}
	for _, p := range probes {
		t.Run(p.what, func(t *testing.T) {
			_, got, out := giTry(t, s, p, true)
			if got != p.want {
				t.Errorf("%s under the deny, want %s:\n%s", verb[got], verb[p.want], out)
			}
			if p.want {
				return
			}
			// No omission-wall branch here, and none is needed: the
			// worktree's own git dir IS granted (that is what makes
			// denying the chain inside it meaningful), and
			// TestQAGrantStillReachesTheGitConfigInEveryShape asserts
			// exactly that for these paths. So a control refusal here is
			// the real reading it always was.
			if _, ok, out := giTry(t, s, p, false); !ok {
				t.Errorf("the CONTROL refused it too — the probe proves nothing about the deny:\n%s", out)
			}
		})
	}

	// The launcher's half: the same tree, the same denies, and the
	// UNSANDBOXED git the launcher runs at land time still works. This is
	// the measurement ADR 0038 item 2 asked for in place of the assumption.
	f := giNewFixture(t)
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	if ok, out := wgRun(t, prof, "echo caged >> "+filepath.Join(f.tree, "work.txt")+
		" && git -C "+f.tree+" add work.txt && git -C "+f.tree+" commit -q -m caged -- work.txt"); !ok {
		t.Fatalf("the session could not commit under the identity-chain deny:\n%s", out)
	}
	commitIn(t, f.repo, "base.txt", "moved on\n", "operator work")
	mustGit(t, f.tree, "rev-parse", "--git-dir")
	mustGit(t, f.tree, "rev-parse", "--show-toplevel")
	before := mustGit(t, f.tree, "rev-parse", "HEAD")
	mustGit(t, f.tree, "rebase", "main")
	if after := mustGit(t, f.tree, "rev-parse", "HEAD"); after == before {
		t.Error("the launcher's rebase did not move HEAD — it witnesses nothing about the deny")
	}
	if got := mustGit(t, f.tree, "log", "--format=%s", "-2"); !strings.Contains(got, "caged") || !strings.Contains(got, "operator work") {
		t.Errorf("the rebase did not replay the session's commit onto the operator's: %q", got)
	}
}

// ─── the cost, measured without a sandbox ────────────────────────────────────

// ADR 0038 item 2 assumed the identity chain's cost is zero and asked for
// it to be MEASURED by execution instead; item 3's verification asks the
// same of the config deny. This is that measurement, and it is deliberately
// built without sandbox-exec so it runs in a CAGED session too — where
// every probe above skips (ranger-base-xjw9) and this file would otherwise
// assert nothing about cost at all.
//
// It grades the other question from the sandbox probes. Those ask whether
// the wall holds; this asks what the wall would BREAK — whether any
// legitimate writer touches these files during a session's whole life. The
// instrument is the files themselves: content, identity (inode) and mtime,
// snapshotted before and compared after. A rename-over-the-top moves the
// inode, an in-place rewrite moves the content, and a touch moves the
// mtime, so all three spellings of "something wrote here" are visible.
//
// giWrote below is the wrong arm, and it is not optional: an instrument
// that cannot see a write reports "nothing was written" over anything at
// all.
type giStamp struct {
	path string
	info os.FileInfo // nil when absent, which is itself the reading
	body []byte
}

func giSnap(t *testing.T, paths ...string) []giStamp {
	t.Helper()
	var out []giStamp
	for _, p := range paths {
		s := giStamp{path: p}
		if fi, err := os.Lstat(p); err == nil {
			s.info = fi
			s.body, _ = os.ReadFile(p)
		}
		out = append(out, s)
	}
	return out
}

// giMoved names what changed, "" when nothing did.
func giMoved(t *testing.T, before []giStamp) string {
	t.Helper()
	var moved []string
	for _, b := range before {
		a := giSnap(t, b.path)[0]
		switch {
		case (b.info == nil) != (a.info == nil):
			moved = append(moved, b.path+" (appeared or vanished)")
		case a.info == nil:
		case !os.SameFile(b.info, a.info):
			moved = append(moved, b.path+" (replaced — a different inode is there now)")
		case string(b.body) != string(a.body):
			moved = append(moved, b.path+" (rewritten in place)")
		case !a.info.ModTime().Equal(b.info.ModTime()):
			moved = append(moved, b.path+" (mtime moved)")
		}
	}
	return strings.Join(moved, "; ")
}

func TestQANoLegitimateWriterTouchesTheConfigOrTheIdentityChain(t *testing.T) {
	f := giNewFixture(t)
	watched := []string{
		f.configOf(t, f.tree), // the COMMON config — the file a worktree's git reads
		f.configOf(t, f.tree) + ".lock",
		f.configOf(t, f.store), // the store of record's, for the record stage below
		filepath.Join(f.tree, ".git"),
		filepath.Join(f.own, "gitdir"),
		filepath.Join(f.own, "commondir"),
		filepath.Join(f.own, "config.worktree"),
		// Absent throughout, which is itself the reading giSnap keeps:
		// this file appearing at all would mean a legitimate writer takes
		// the config.worktree lock, and the literal added for
		// ranger-base-xwepd would have to be dropped and RECORDED.
		filepath.Join(f.own, "config.worktree.lock"),
	}

	// A whole session's life, in the order a session lives it: work, stage,
	// commit, check out, ask git where it is. Then the record stage a bead
	// close makes in the store of record — `bd` is not in this fixture, but
	// what it does to the filesystem is, and that is the part a path deny
	// can be wrong about. Then the launcher's own land-time git, which runs
	// UNSANDBOXED inside this same tree (worktree.go) and is the reason the
	// identity chain is denied in the first place.
	before := giSnap(t, watched...)
	sbWrite(t, filepath.Join(f.tree, "work.txt"), "session work\n")
	mustGit(t, f.tree, "add", "work.txt")
	mustGit(t, f.tree, "commit", "-q", "-m", "session work", "--", "work.txt")
	mustGit(t, f.tree, "checkout", "-q", "--", "work.txt")
	mustGit(t, f.tree, "status", "--porcelain")
	mustGit(t, f.tree, "rev-parse", "--git-dir", "--git-common-dir", "--show-toplevel")
	mustGit(t, f.tree, "log", "-1", "--format=%H")

	sbWrite(t, filepath.Join(f.store, beadsDirName, "issues.jsonl"), "{\"id\":\"x\"}\n")
	sbWrite(t, filepath.Join(f.store, beadsDirName, "beads.db"), "db\n")
	mustGit(t, f.store, "add", "-A", filepath.Join(f.store, beadsDirName))
	mustGit(t, f.store, "commit", "-q", "-m", "bd sync", "--", filepath.Join(f.store, beadsDirName))

	// The operator's own commit, spelled out rather than through commitIn:
	// that helper sets user.email/user.name first, which is a REAL config
	// write and would be read here as a cost this deny imposes. It is not —
	// it is the fixture writing its own identity, which the live shape does
	// once, long before any session launches. Caught by this instrument on
	// its first run, which is the instrument working.
	sbWrite(t, filepath.Join(f.repo, "base.txt"), "operator moved on\n")
	mustGit(t, f.repo, "add", "base.txt")
	mustGit(t, f.repo, "commit", "-q", "-m", "operator work", "--", "base.txt")
	mustGit(t, f.tree, "rebase", "main")
	mustGit(t, f.repo, "worktree", "list")

	if moved := giMoved(t, before); moved != "" {
		t.Errorf("a legitimate writer needs a file ADR 0038 denies — drop that literal and RECORD it (item 2's own instruction): %s", moved)
	}
	// The run has to have been real, or "nothing was written" is a reading
	// of an empty session.
	if got := mustGit(t, f.tree, "log", "--format=%s", "-2"); !strings.Contains(got, "session work") || !strings.Contains(got, "operator work") {
		t.Fatalf("the session life did not happen — the snapshot compares two idle trees: %q", got)
	}
}

// The wrong arm. Same instrument, same fixture, one command that DOES write
// the config — so a green run above is a reading and not a blind spot.
// Both spellings, because they fail differently: `git config` renames a new
// file over the old one (the inode moves), a shell append rewrites in place
// (the content moves), and an instrument that only watched one of them
// would miss the other.
func TestQATheCostInstrumentSeesAConfigWrite(t *testing.T) {
	for _, arm := range []struct {
		what string
		run  func(t *testing.T, f giFixture)
	}{
		{"git config renames a new file over it", func(t *testing.T, f giFixture) {
			mustGit(t, f.tree, "config", "core.hooksPath", "/tmp/x")
		}},
		{"a shell append rewrites it in place", func(t *testing.T, f giFixture) {
			cfg := f.configOf(t, f.tree)
			b, err := os.ReadFile(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cfg, append(b, []byte("\n[core]\n\thooksPath = /tmp/x\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"a worktree repair rewrites the identity chain", func(t *testing.T, f giFixture) {
			sbWrite(t, filepath.Join(f.own, "gitdir"), filepath.Join(f.tree, ".git")+"\n")
		}},
	} {
		t.Run(arm.what, func(t *testing.T) {
			f := giNewFixture(t)
			watched := []string{f.configOf(t, f.tree), filepath.Join(f.own, "gitdir")}
			before := giSnap(t, watched...)
			arm.run(t, f)
			if moved := giMoved(t, before); moved == "" {
				t.Errorf("the instrument saw nothing after a real write — every green cost run above measures nothing")
			}
		})
	}
}
