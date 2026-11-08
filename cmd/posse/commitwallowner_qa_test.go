package main

// QA pins for ranger-base-hnujh — ADR 0081 D4: `posse gates install-hooks`
// REFUSES an explicit target whose shared commit wall was rendered by another
// home, when this home does not own that repo (D1).
//
// WHAT WAS WRONG. install-hooks' commit-guard arm was an unconditional write
// into the repo's COMMON hooks dir, bodied from the TYPING home's config
// (OpsPatternSet, DeriveGuardValueLiterals, DeriveCrewLiterals,
// BeadsVisibility). So the verb that exists to put a wall right was also the
// verb that took it away: MEASURED 2026-11-06 (ranger-base-q0np6), same
// binary, same repo, only RHQ_HOME changed, 1897 lines against 1137. The
// launch took that write rule in ranger-base-lh9nj (D3) and left this verb
// as the second of the three writers. ranger-base-knb1v's D7 is the third
// (the queue repo's commit) and is its own bead.
//
// WHY THESE ARE BLACK-BOX RUNS OF THE BINARY. The claim is four things at
// once and none of them is a predicate's return value: the command SAYS whose
// wall it found and both remedies, it exits NONZERO, the file is byte-for-byte
// and mtime-for-mtime where it was, and pre-push — which ADR 0081 D5 leaves
// alone, being a constant with no home input — is still installed. Only a run
// of the command can be wrong about the last three.
//
// THE THREE CONTROL ARMS ARE THE POINT. A refusal that fired on "this home
// does not own the repo" alone is alternative (c), which the ADR rejects: it
// would stop arming every repo nobody listed (`~/code/...`, a `dirs:` entry,
// a repo made seconds ago). So the same non-owning home must still install
// over an empty slot (D3 case 1), over a body with no stamp (case 4, the
// one-shot migration of every body written before the ADR), and — once it
// marks the repo — over another home's render (case 2, and D6: two owners is
// visible misconfiguration, not a refusal).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// cwoHomeStampPrefix is D2's whole syntax, spelled here rather than imported:
// this package is outside internal/posse, and a pin that read the constant
// could not catch a render that stopped emitting the line.
const cwoHomeStampPrefix = "# posse-home: "

// cwoFixture gives two homes and one repo. `owner` declares the repo in its
// `beads_visibility:` — it is the box's home in the done-when — and `other`
// declares a DIFFERENT repo rather than nothing at all: D1 is a question
// about one repo, and an empty config would also pass a predicate that asked
// "does this home list anything".
func cwoFixture(t *testing.T) (bin, owner, other, repo string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	bin = buildRhq(t)
	// RESOLVED, since ADR 0082 D3: the subprocess stamps the wall with
	// `canonicalPath(a.Home)`, and `t.TempDir()` hands back a path through
	// darwin's /var -> /private/var symlink — so an unresolved root would
	// make every `cwoStampOf` assertion below compare the stamp against the
	// one spelling it is guaranteed not to carry. The sibling at
	// installhooksrelativetarget_qa_test.go resolves its root for the
	// mirror-image reason. EvalSymlinks and not the kernel reading is enough
	// HERE because the only axis this fixture crosses is that symlink: every
	// path under the root is created in the case it is named in, and a temp
	// root is not reached through a firmlink.
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	owner = filepath.Join(root, "owning-home")
	other = filepath.Join(root, "scratch-home")
	repo = filepath.Join(root, "checkout")
	elsewhere := filepath.Join(root, "some-other-checkout")
	for _, d := range []string{owner, other, repo, elsewhere} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []string{repo, elsewhere} {
		if out, err := exec.Command("git", "-C", r, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v %s", r, err, out)
		}
	}
	write := func(home, listed string) {
		if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads_visibility:\n  "+listed+": private\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(owner, repo)
	write(other, elsewhere)
	// The two paths every assertion below prints. AbbrevHome rewrites a
	// path under $HOME to `~/...`, and the subprocess's HOME is `other`, so
	// an owner or repo path underneath it would be reported abbreviated and
	// these assertions would be measuring path spelling.
	for _, p := range []string{owner, repo} {
		if p == other || strings.HasPrefix(p, other+string(filepath.Separator)) {
			t.Skipf("fixture premise: %s lies under the scratch home %s, so the report would abbreviate it", p, other)
		}
	}
	return bin, owner, other, repo
}

// cwoRun runs the command under test with RHQ_HOME pinned to one of the two
// homes. HOME follows it, because an inherited HOME would point the binary's
// config fallback at the live instance.
func cwoRun(t *testing.T, bin, home string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"gates", "install-hooks"}, args...)...)
	cmd.Env = append(os.Environ(), "HOME="+home, "RHQ_HOME="+home)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("gates install-hooks: %v %s", err, out)
	}
	return string(out), code
}

// cwoSlot is the shared commit-guard slot's path, its bytes and its mtime in
// nanoseconds — the whole of "unchanged". Bytes alone would pass a write that
// rendered the same body twice, which is still this home writing another
// home's file.
func cwoSlot(t *testing.T, repo string) (path, body string, mtime int64) {
	t.Helper()
	path = filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the commit-guard slot: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, string(b), st.ModTime().UnixNano()
}

// cwoStampOf is the home named by D2's stamp in a body, or "" for none.
func cwoStampOf(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(line, cwoHomeStampPrefix); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// cwoArm installs the wall from `owner` and returns the slot as it then
// stands. Every arm below starts here, so each one is measuring a departure
// from the same installed state.
func cwoArm(t *testing.T, bin, owner, repo string) (path, body string, mtime int64) {
	t.Helper()
	if out, code := cwoRun(t, bin, owner, repo); code != 0 {
		t.Fatalf("the owning home's own install exited %d:\n%s", code, out)
	}
	path, body, mtime = cwoSlot(t, repo)
	if got := cwoStampOf(body); got != owner {
		t.Fatalf("fixture premise: the installed body is stamped %q, want the owning home %q — without D2's stamp a non-owner has no way to tell this wall from nobody's", got, owner)
	}
	return path, body, mtime
}

// ─── D4: the refusal ─────────────────────────────────────────────────────────

func TestQAInstallHooksRefusesAWallAnotherHomeRenderedAndOwns(t *testing.T) {
	t.Parallel()
	bin, owner, other, repo := cwoFixture(t)
	path, before, beforeTime := cwoArm(t, bin, owner, repo)

	out, code := cwoRun(t, bin, other, repo)

	if code == 0 {
		t.Errorf("exit 0 — a non-owning home was allowed to rewrite %s's wall:\n%s", repo, out)
	}
	// WHOSE wall, and WHICH repo. A refusal that named neither would leave
	// the operator with a wall they cannot place and no config line to add.
	if !strings.Contains(out, owner) {
		t.Errorf("the refusal does not name the stamping home %s:\n%s", owner, out)
	}
	if !strings.Contains(out, repo) {
		t.Errorf("the refusal does not name the repo %s whose wall it is:\n%s", repo, out)
	}
	// BOTH remedies (ADR 0081 D4), each asserted on its own: the config
	// line, and the other home. One of them alone is a wall with one door
	// for a reader who cannot reach it.
	if !strings.Contains(out, "beads_visibility:") {
		t.Errorf("the refusal does not name the config list that confers ownership:\n%s", out)
	}
	if !strings.Contains(out, "install from home") {
		t.Errorf("the refusal does not offer the second remedy — installing from the home that stamped it:\n%s", out)
	}
	// The slot, byte for byte and mtime for mtime. This is the done-when,
	// and the one assertion a predicate's return value cannot make.
	if _, after, afterTime := cwoSlot(t, repo); after != before || afterTime != beforeTime {
		t.Errorf("%s moved: bytes equal=%v, mtime %d → %d", path, after == before, beforeTime, afterTime)
	}
	// ADR 0081 D5: pre-push is a constant with no home input, so a
	// cross-home write produces identical bytes and there is nothing to
	// contend over. The commit guard's refusal must not take it down with
	// it — the two slots are reported independently (rangerhq-mgdk).
	if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", "pre-push")); err != nil {
		t.Errorf("pre-push was not installed: %v — D5 leaves that slot alone, and this refusal is only about the commit wall", err)
	}
	// No escape hatch is advertised, because none exists (D4): the remedy
	// is a config line, and a flag would put the second writer straight
	// back.
	if strings.Contains(out, "--force") && !strings.Contains(out, "no --force") {
		t.Errorf("the refusal advertises a --force this command does not have:\n%s", out)
	}
}

// The same refusal when the target is `.` rather than an argument. D4 says
// "an explicit target" to say that the operator's sweep over their OWN listed
// repos is unaffected — every one of those is owned — and not to make the cwd
// a second rule. q0np6's two scratch-home launches were standing IN the repo.
func TestQAInstallHooksRefusesAForeignWallInTheCurrentDirectoryToo(t *testing.T) {
	t.Parallel()
	bin, owner, other, repo := cwoFixture(t)
	path, before, beforeTime := cwoArm(t, bin, owner, repo)

	cmd := exec.Command(bin, "gates", "install-hooks")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "HOME="+other, "RHQ_HOME="+other)
	raw, err := cmd.CombinedOutput()
	out := string(raw)
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("gates install-hooks in %s: %v %s", repo, err, out)
	}

	if code == 0 {
		t.Errorf("exit 0 — `install-hooks` with no argument rewrote another home's wall in the cwd:\n%s", out)
	}
	if !strings.Contains(out, owner) {
		t.Errorf("the refusal does not name the stamping home %s:\n%s", owner, out)
	}
	// And the REPO, named as a repo rather than as the cwd — the assertion
	// the explicit-argument sibling above makes and this arm did not, which
	// is the whole of ranger-base-k9x8n. Not a wording quibble: the remedy
	// line prescribes this same string as a `beads_visibility:` key, and
	// such a key is resolved against the READING process's cwd, so a `.`
	// here is a remedy that is inert where it was printed and a wildcard
	// over every other repo it is read from. The resolved spelling is what
	// the binary prints, because it absolutizes against its own cwd and
	// `t.TempDir()` hands back a path through a symlink on darwin.
	wantRepo := repo
	if resolved, rerr := filepath.EvalSymlinks(repo); rerr == nil {
		wantRepo = resolved
	}
	if !strings.Contains(out, wantRepo) {
		t.Errorf("the refusal does not name the repo %s whose wall it is — a relative target must still be reported, and prescribed, as the repo:\n%s", wantRepo, out)
	}
	if _, after, afterTime := cwoSlot(t, repo); after != before || afterTime != beforeTime {
		t.Errorf("%s moved: bytes equal=%v, mtime %d → %d", path, after == before, beforeTime, afterTime)
	}
}

// ─── the three arms that must still install ──────────────────────────────────

// D3 case 2 / D6. The scratch home MARKS the repo, so it owns the wall by the
// only definition D1 has, and the install stands. Two owners is
// misconfiguration made visible by the stamp, not a refusal — and this is the
// arm a predicate that dropped ownership and refused on the stamp alone would
// red.
func TestQAInstallHooksInstallsOverAnotherHomesWallOnceThisHomeOwnsTheRepo(t *testing.T) {
	t.Parallel()
	bin, owner, other, repo := cwoFixture(t)
	_, before, _ := cwoArm(t, bin, owner, repo)

	if err := os.WriteFile(filepath.Join(other, "config.yaml"), []byte("beads_visibility:\n  "+repo+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := cwoRun(t, bin, other, repo)
	if code != 0 {
		t.Fatalf("exit %d — this home now marks %s, which is the whole of D1:\n%s", code, repo, out)
	}
	_, after, _ := cwoSlot(t, repo)
	if after == before {
		t.Errorf("the wall was not re-rendered: still the other home's body")
	}
	if got := cwoStampOf(after); got != other {
		t.Errorf("the installed body is stamped %q, want the installing home %q", got, other)
	}
}

// D3 case 4, through D4: a body written before this ADR carries no stamp, so
// nothing says it is anybody's, and the first install re-stamps it. The one
// case where "this home does not own the repo" is true and the write happens
// anyway — alternative (c)'s epitaph.
func TestQAInstallHooksInstallsOverAnUnstampedWallFromAnyHome(t *testing.T) {
	t.Parallel()
	bin, owner, other, repo := cwoFixture(t)
	path, before, _ := cwoArm(t, bin, owner, repo)

	var kept []string
	for _, line := range strings.Split(before, "\n") {
		if !strings.HasPrefix(line, cwoHomeStampPrefix) {
			kept = append(kept, line)
		}
	}
	migration := strings.Join(kept, "\n")
	if migration == before {
		t.Fatalf("fixture premise: %s carries no stamp line to strip", path)
	}
	if err := posse.WriteExecutable(path, []byte(migration), 0o755); err != nil {
		t.Fatal(err)
	}

	out, code := cwoRun(t, bin, other, repo)
	if code != 0 {
		t.Fatalf("exit %d — an unstamped body is D3 case 4's migration and every home may write it:\n%s", code, out)
	}
	if _, after, _ := cwoSlot(t, repo); cwoStampOf(after) != other {
		t.Errorf("the migration did not re-stamp: body stamped %q, want %q", cwoStampOf(after), other)
	}
}

// D3 case 1, through D4: an empty slot belongs to nobody. A non-owning home
// installing into `~/code/...` or a repo made seconds ago is the common case
// this verb exists for, and it must not have become a refusal.
func TestQAInstallHooksInstallsIntoAnEmptySlotFromANonOwningHome(t *testing.T) {
	t.Parallel()
	bin, _, other, repo := cwoFixture(t)

	out, code := cwoRun(t, bin, other, repo)
	if code != 0 {
		t.Fatalf("exit %d installing into a repo nobody has stamped:\n%s", code, out)
	}
	_, body, _ := cwoSlot(t, repo)
	if got := cwoStampOf(body); got != other {
		t.Errorf("the installed body is stamped %q, want the installing home %q", got, other)
	}
}
