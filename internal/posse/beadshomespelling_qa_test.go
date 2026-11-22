//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-1b8yv — BEADS_HOME ANSWERS A SPELLING, AND EVERY
// WALL RESOLVES IT BEFORE IT RENDERS A RULE OR DECIDES CONTAINMENT.
//
// WHAT WAS WRONG, and it was wrong in the pins and not in the walls. Nine
// pins over the store-repo git carve-out (ranger-base-kfrpv, ADR 0086 D3)
// guarded themselves with `beadsHome(dir) != absResolve(<the store this
// fixture built>)` and abstained — loudly, "measures nothing" — on every
// darwin box: `t.TempDir()` lands under `/var/folders/…`, `/var` is a link
// to `/private/var`, and the two sides were one directory spelled two ways.
// MEASURED 2026-11-21 by READING ci run 44974829306 (b2509fba), job
// 137062263947, "test (macos-latest, 1)", and reproduced on this linux box
// by pointing TMPDIR at a symlink, which is the same condition with none of
// darwin's specifics: the same nine rows, at the same nine lines. The
// guards' own messages printed the UNRESOLVED want, so a got and a want
// that read IDENTICAL were the only evidence a reader got.
//
// THE QUESTION THAT MATTERED was not the fixtures' — it is whether
// PRODUCTION renders a rule, or decides containment, from the unresolved
// spelling. A seatbelt rule matches the vnode's REAL path, so an SBPL line
// spelled `/var/…` matches nothing on darwin: a grant spelled that way is a
// grant that is not there, and a DENY spelled that way is a deny that is not
// there. ANSWERED HERE, and the answer is no — every such consumer resolves
// first — which is this file rather than a sentence on a bead, because an
// audit of thirteen call sites is a reading that rots and a pin is not.
//
// THE CONTRACT THE PINS BELOW DEPEND ON (beadsHome's own comment,
// beadloss.go): what comes back is bd's spelling and not a resolved path,
// because its first readers hand it to bd as `BEADS_DIR` and to `git -C`,
// where the kernel resolves it and bd takes its own `BEADS_DIR points to
// unsafe location` verdict on the spelling the operator configured.
// Resolving it there would move that verdict on any host whose store sits
// behind a link — a grant decision made by a helper's convenience rather
// than by a rule. So the resolution lives at each wall, and arm 1 is the
// control that keeps the rest of this file from being vacuous.
//
// THE FIXTURE IS A STORE REACHED THROUGH A SYMLINK, built here rather than
// inherited from `t.TempDir()`: on darwin the temp root would discriminate
// by itself, and on linux it would not, so a pin that leaned on it would be
// green over the bug on the host this box is.
//
// WHAT THIS FILE DOES NOT PIN, found while answering the question above and
// filed rather than fixed here: at the CONTAINER tier the store's MOUNT is
// resolved (cageOverlay) while the `BEADS_DIR` forwarded into the cage is
// beadsHome's spelling (planLaunch, CageEnvNames), so a store behind a link
// is named inside the cage at a path no mount holds — and
// containerReachRow, which resolves, reads the row Enforced anyway.
//
// CLOSED by ranger-base-ayubz, and NOT by a fifth `absResolve`: at L4 the
// filesystem is the rendered bind list, so where the thing behind the link is
// the session DIR the store crosses at the raw spelling and resolving the
// name would have broken the case that worked. The caged env asks the mount
// set which path it gave the store (CageBeadsDir, cage.go), and
// cagebeadsdir_qa_test.go is the pin that belongs beside this one — the
// reading, both shapes, and why L2 and L3 are not affected.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bhsFixture is wgDenyFixture plus a store of record in another repo reached
// by a `.beads/redirect` whose path runs through a SYMLINK — so beadsHome
// answers `<root>/link/store/.beads` while the kernel, the sandbox and every
// mount see `<root>/real/store/.beads`.
type bhsFixture struct {
	wgFixture
	store string // the store repo's root, as the kernel sees it
	home  string // the store of record, RESOLVED: what a wall rule must carry
	spelt string // the same directory as the redirect spells it, through the link
}

func bhsNew(t *testing.T) bhsFixture {
	t.Helper()
	f := wgDenyFixture(t)
	root := filepath.Dir(f.repo)
	store := sbMkdir(t, filepath.Join(root, "real", "store"))
	sbGitInit(t, store)
	sbMkdir(t, filepath.Join(store, beadsDirName))
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "real"), link); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	spelt := filepath.Join(link, "store", beadsDirName)
	sbMkdir(t, filepath.Join(f.tree, beadsDirName))
	sbWrite(t, filepath.Join(f.tree, beadsDirName, beadsRedirect), spelt+"\n")

	s := bhsFixture{wgFixture: f, store: absResolve(store), home: absResolve(filepath.Join(store, beadsDirName)), spelt: spelt}
	// THE FIXTURE IS THE SHAPE EVERY ARM NEEDS, asserted and not assumed,
	// in the three ways it could fail to be: the redirect is followed to
	// the link's spelling, that spelling is NOT the resolved one, and the
	// two are nevertheless one directory. Without the second, every
	// `absResolve` below is a no-op and this file passes over a wall that
	// renders the raw spelling; without the third, the arms would be about
	// a redirect that reaches somewhere else entirely.
	if got := beadsHome(f.tree); got != spelt {
		t.Fatalf("fixture: the redirect does not reach the store as spelled: beadsHome = %q, want %q", got, spelt)
	}
	if s.spelt == s.home {
		t.Fatalf("fixture: %s and its resolved path are the same string, so nothing below discriminates", s.spelt)
	}
	si, err := os.Stat(s.spelt)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	hi, err := os.Stat(s.home)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if !os.SameFile(si, hi) {
		t.Fatalf("fixture: %s and %s are not the same directory, so the arms below are about two stores", s.spelt, s.home)
	}
	return s
}

// 1. THE CONTRACT, and the control for every arm below: beadsHome answers
// the spelling and NOT the resolved path, in both of its shapes — the
// redirect's target as the operator wrote it, and `<dir>/.beads` with dir
// exactly as the caller spelled it (`beadsHome(".")` is relative, which is
// bdLineCensusHome's fallback).
//
// A PIN ON A CHOICE, so it reds for whoever changes that choice rather than
// leaving them to find the nine abstentions on a release leg. It is not a
// claim that resolving would be wrong everywhere: it is that resolving HERE
// moves bd's own unsafe-prefix verdict and the `BEADS_DIR` a session
// carries, so the resolution belongs at each wall, where arm 2 pins it.
func TestQABeadsHomeAnswersBdsOwnSpellingAndNotAResolvedPath(t *testing.T) {
	f := bhsNew(t)

	if got := beadsHome(f.tree); got != f.spelt {
		t.Errorf("beadsHome resolved a redirect target: got %s, want the spelling the redirect carries, %s (beadloss.go's contract — BEADS_DIR and `git -C` read this, and bd's unsafe-prefix verdict is taken on it)", got, f.spelt)
	}
	// The local shape, which has no redirect to carry a spelling: dir as
	// given, plus `.beads`, and nothing resolved on the way out.
	local := wgFixtureAnyBox(t)
	sbMkdir(t, filepath.Join(local.tree, beadsDirName))
	if got, want := beadsHome(local.tree), filepath.Join(local.tree, beadsDirName); got != want {
		t.Errorf("beadsHome does not answer <dir>/.beads as dir was spelled: got %s, want %s", got, want)
	}
	if got := beadsHome("."); got != filepath.Join(".", beadsDirName) {
		t.Errorf("beadsHome(%q) must stay relative — bdLineCensusHome's fallback spells it that way: got %s", ".", got)
	}
}

// 2. EVERY WALL, over the same store: the rule it renders carries the
// RESOLVED path, and the raw spelling appears in none of them. Four
// mechanisms, because each is a different producer and a fifth one added
// tomorrow is a rule nothing else here says anything about:
//
//   - L2's profile, where a rule matches the vnode's real path and a line
//     spelled through a link matches nothing at all — the arm the bead's
//     hardening question was asked about;
//   - L3's argv, where the grant becomes a `--bind`;
//   - L4's mount set, judged on the destination the container sees;
//   - a self-sandboxing runtime's `--add-dir`, where an unresolved
//     component is not a weak wall but a session that runs NO COMMAND at
//     all (ranger-base-k62e, measured on codex-cli 0.150.1) — and the
//     control for that one is the refusal the raw spelling would have
//     bought, which is the only arm here whose cost is total.
func TestQAEveryWallResolvesTheStoreSpellingBeadsHomeAnswers(t *testing.T) {
	f := bhsNew(t)
	set := f.a.SeatbeltWritable(f.ag, f.tree, f.gates)

	// L2. The set first, by spelling, because the profile is rendered from
	// it and a set carrying the link would put the link in every tier.
	if !bgHas(set, f.home) {
		t.Fatalf("the writable set does not reach the store of record at all, so no arm below is measured:\n  %s", strings.Join(wgKinds(set), "\n  "))
	}
	for _, p := range set {
		if p == f.spelt || strings.HasPrefix(p, f.spelt+string(filepath.Separator)) {
			t.Errorf("the writable set carries the store as SPELLED (%s) — a seatbelt rule matches the vnode's real path, so a grant spelled through a link grants nothing and a deny spelled through one denies nothing:\n  %s",
				p, strings.Join(wgKinds(set), "\n  "))
		}
	}
	carve := f.a.SeatbeltCarveOut(f.ag, f.tree, f.gates, set)
	prof := SeatbeltProfile(f.ag.Name, set, SeatbeltSiblings(nil, carve), carve, sessionRefDirsFor(f.ag, f.tree)...)
	if !strings.Contains(prof, `(subpath "`+f.home+`")`) {
		t.Errorf("the profile does not grant the store of record at its resolved path %s, which is the only path the kernel will match:\n%s", f.home, prof)
	}
	if strings.Contains(prof, f.spelt) {
		t.Errorf("the profile names the store as SPELLED (%s) — that line matches nothing on a host where the store is behind a link, granted or denied:\n%s", f.spelt, prof)
	}

	// L3. The same grant one mechanism over: a bind SOURCE, which bwrap
	// resolves at mount(2) and which this render refuses outright when it
	// is spelled through a link (ADR 0100 D1, ranger-base-wsgus) — so an
	// unresolved store here is a seat that does not come up.
	bset, keep := BwrapWritable(f.ag, set, f.tree)
	argv, _, err := BwrapArgs(f.tree, bset, carve, keep...)
	if err != nil {
		t.Fatalf("the render refuses this shape, so the L3 arm measures nothing: %v", err)
	}
	if !srgBound(argv, "--bind", f.home) {
		t.Errorf("the render does not bind the store of record read-write at %s:\n  %s", f.home, strings.Join(argv, " "))
	}
	for i, a := range argv {
		if i+1 < len(argv) && (a == "--bind" || a == "--ro-bind") && argv[i+1] == f.spelt {
			t.Errorf("the render binds the store as SPELLED (%s) — bwrap follows a link at a bind source, so this mounts a path no grant judged:\n  %s", f.spelt, strings.Join(argv, " "))
		}
	}

	// L4. The mount set, whose containment is judged on the destination the
	// CONTAINER sees — so a destination spelled through a host link is a
	// path that does not exist inside the cage.
	ca := cageApp(t)
	e, err := ca.LoadEngine("fake")
	if err != nil {
		t.Fatalf("fixture engine: %v", err)
	}
	repo := overlayRepo(t)
	sbWrite(t, filepath.Join(repo, beadsDirName, beadsRedirect), f.spelt+"\n")
	ms := ca.CageMounts(cageAgent(t, ca, "cage: container\n"), e, repo, "bhs")
	wantMode(t, ms, f.home, false, "the store of record behind a link")
	m, ok := mountAt(ms, f.home)
	if ok && (m.Src != f.home || m.Dst != f.home) {
		t.Errorf("the store's mount is spelled through the link (src %s, dst %s), and inside the cage that destination does not exist; want %s both ends", m.Src, m.Dst, f.home)
	}
	// And the containment decision the reachability row makes over that
	// set, which is the question CageMounts itself answers (cageCovering).
	if i, _ := cageCovering(ms, f.home); i < 0 || ms[i].RO {
		t.Errorf("no read-write bind covers %s, so a caged session records nothing there:\n%s", f.home, showMounts(ms))
	}

	// A self-sandboxing runtime. launchWritableRoots hands the roots
	// beadsHome spells and codexWritableRoot resolves each one
	// (TestCodexResolvesASymlinkedWritableRoot pins that resolver; what is
	// pinned here is the CHAIN from this store to that line).
	roots := launchWritableRoots(f.tree)
	if !bgHas(roots, f.home) {
		t.Fatalf("the launch's writable roots do not name the store, so this arm measures nothing: %v", abbrevAll(roots))
	}
	line := "codex " + realizeCodex(nil, nil, f.ag.MemoryDir, roots...).Deny
	if !strings.Contains(line, "--add-dir "+shellQuote(f.home)) {
		t.Errorf("the rendered line does not name the store at its resolved path %s: %s", f.home, line)
	}
	if root, comp := refusedWritableRoot(line); root != "" {
		t.Errorf("the rendered line names a writable root codex refuses (%s, component %s) — it would come up, read its prompt and then fail every command it runs (ranger-base-k62e): %s", root, comp, line)
	}
	// The control, and the measurement that makes this arm worth its
	// length: the raw spelling IS a root codex refuses, so the resolution
	// above is load-bearing and not decoration.
	if root, _ := refusedWritableRoot("codex -s workspace-write --add-dir " + shellQuote(f.spelt)); root == "" {
		t.Errorf("the store as SPELLED (%s) is not a root codex refuses, so the arm above holds for the wrong reason", f.spelt)
	}
}

// 3. THE GATE, which is the other half of the bead's question: a containment
// DECISION taken over two spellings of one directory. recordTargets is
// handed exactly what applyRecordReach hands it — beadsHome's answer — and
// every target it names has to lie inside a grant the same launch makes,
// whichever way each side happens to be spelled. That agreement is
// recordreachgrant_qa_test.go's whole subject; what is added here is the
// store behind a link, where a producer comparing strings would read the
// store as ungranted and refuse the launch for a write the wall does grant.
func TestQATheRecordReachGateAgreesWithTheWallAcrossBothSpellings(t *testing.T) {
	f := bhsNew(t)

	for _, ag := range []*AgentFile{f.ag, wgDeny(f.wgFixture)} {
		home := beadsHome(f.tree)
		targets := recordTargets(ag, f.tree, home)
		if !bgHas(targets, f.home) {
			t.Errorf("%s: the gate does not probe the store of record — that is the one write the record stage makes in another repo:\n  %s",
				ag.Name, strings.Join(wgKinds(targets), "\n  "))
		}
		grants := f.a.SeatbeltWritable(ag, f.tree, f.gates)
		storeGrants := wgRepoGrants(grants, f.store, f.store)
		if len(storeGrants) == 0 {
			t.Fatalf("%s: the writable set reaches nothing in the store's repo, so this arm is vacuous", ag.Name)
		}
		for _, p := range targets {
			if !anyUnderDir(grants, p) {
				t.Errorf("%s: the gate probes %s, which this launch's writable set does not reach — the launch refuses for a write nothing can satisfy (ADR 0013 §4):\n  %s",
					ag.Name, p, strings.Join(wgKinds(grants), "\n  "))
			}
		}
		// The store repo's git stays out of both, through the link as
		// through the real path: a containment rule that compared
		// spellings would answer "not in the store repo" for every one of
		// these and read as a wall (ranger-base-kfrpv).
		for _, bad := range []string{
			filepath.Join(f.store, ".git"),
			filepath.Join(f.store, ".git", "objects"),
			filepath.Join(filepath.Dir(f.spelt), ".git"),
			filepath.Join(filepath.Dir(f.spelt), ".git", "objects"),
		} {
			if anyUnderDir(storeGrants, bad) {
				t.Errorf("%s is granted %s — the store repo's git is read-only at every tier:\n  %s", ag.Name, bad, strings.Join(wgKinds(storeGrants), "\n  "))
			}
			if bgHas(targets, bad) {
				t.Errorf("%s is probed for %s, which no wall grants:\n  %s", ag.Name, bad, strings.Join(wgKinds(targets), "\n  "))
			}
		}
	}
}
