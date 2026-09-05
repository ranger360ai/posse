package posse

// QA pins for the cutover fan-out's redirect compare, filed as
// ranger-base-4myz against scripts/queue-cutover.sh and fixed there.
//
// WHAT ranger-base-l9aa FIXED. The cutover fan-out used to rewrite a LIST of
// trees, so a checkout that already redirected at the constitution and was on
// nobody's list kept that redirect after the store moved and became hop one of
// a two-hop chain — which bd 0.49.1 refuses, silently in the arm that matters.
// The fix walks the constitution's parent and rewrites what it finds.
//
// WHAT IT STILL MISSED. The scan decided a tree was "pointed at the
// constitution" with an EXACT string compare of the redirect's first line
// against $SRC_BEADS:
//
//	cur=$(head -n 1 "$b/redirect" ...); [ "$cur" = "$SRC_BEADS" ] || continue
//
// bd is looser than that. MEASURED against bd 0.49.1 (2026-08-30, a scratch
// rig: one store, one canary bead, a work tree per spelling, `bd --no-daemon
// list --json` counted), every spelling in qcSloppySpellings below is a live
// redirect bd follows to the canonical store, and the exact compare sees none
// of them. So a tree spelled any of those ways was left behind by the fan-out
// and became exactly the two-hop chain l9aa was filed for.
//
// The same measurement moved three things past the seven the bead filed:
//
//   - TABS, AND THE REST OF Go's TrimSpace. A leading or trailing tab resolves
//     for bd just as a space does, and so do a vertical tab and a form feed —
//     bd trims with strings.TrimSpace, so the script's `tr -d` removes exactly
//     that set rather than a guess at which blanks a hand might type.
//   - RELATIVE PATHS. `../canon/.beads` resolves, and it resolves against the
//     TREE HOLDING THE REDIRECT rather than the caller's cwd — run from the
//     tree root, from `sub/` and from `sub/deeper/`, all three found the store.
//
// And two the fix gets for free but this file cannot portably pin: a symlinked
// spelling (pinned below, symlinks being portable enough) and, on a
// case-insensitive filesystem, a different case — which is a property of the
// filesystem rather than of bd, so there is no arm for it here.
//
// This is not hypothetical bookkeeping: the originating instance's own
// redirect — the tree l9aa was about — was repointed BY HAND, out of band,
// with no bead recording it (41 bytes, no trailing newline, where the script
// writes 42). Hand-edited redirects are how this fleet actually gets them,
// and a hand does not spell paths the way a script does.
//
// THE FIX is `redirect_names` in the script: trim the blanks and the CR,
// resolve a relative path against the tree holding the redirect, and compare
// with `-ef` — device and inode, the same "same directory" bd means. Looser
// about spelling, no looser about targets, which is what
// TestQueueCutoverLeavesAStrangerStoreAloneHoweverItIsSpelled holds from the
// other side: a tree pointed at a DIFFERENT store, spelled every one of these
// ways, keeps its redirect byte for byte.
//
// The setup witness in each arm holds on BOTH sides of the fix — it asks the
// filesystem whether the spelling and the store are the same directory, which
// is true before and after — so these measure the fix and not the fixture
// (ranger-base-e8hp).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qcSloppySpellings are spellings of one directory that bd 0.49.1 follows and
// the fan-out's old exact compare did not recognise. Each takes the canonical
// `<constitution>/.beads` and the tree whose redirect will hold the spelling,
// and returns what to write into that redirect.
var qcSloppySpellings = []struct {
	name  string
	spell func(t *testing.T, store, tree string) string
}{
	{"trailing slash", func(_ *testing.T, s, _ string) string { return s + "/" }},
	{"trailing space", func(_ *testing.T, s, _ string) string { return s + " " }},
	{"leading space", func(_ *testing.T, s, _ string) string { return " " + s }},
	{"trailing tab", func(_ *testing.T, s, _ string) string { return s + "\t" }},
	{"leading tab", func(_ *testing.T, s, _ string) string { return "\t" + s }},
	{"doubled slash", func(_ *testing.T, s, _ string) string {
		return filepath.Dir(s) + "//" + filepath.Base(s)
	}},
	{"dot segment", func(_ *testing.T, s, _ string) string {
		return filepath.Dir(s) + "/./" + filepath.Base(s)
	}},
	{"parent segment", func(_ *testing.T, s, _ string) string {
		d := filepath.Dir(s)
		return d + "/../" + filepath.Base(d) + "/" + filepath.Base(s)
	}},
	{"CRLF line ending", func(_ *testing.T, s, _ string) string { return s + "\r" }},
	// bd's trim is Go's strings.TrimSpace, which is wider than "blanks":
	// measured, a vertical tab and a form feed resolve too, so the script's
	// `tr` removes exactly the set Go removes and these two hold it there.
	{"trailing vertical tab", func(_ *testing.T, s, _ string) string { return s + "\v" }},
	{"leading form feed", func(_ *testing.T, s, _ string) string { return "\f" + s }},
	// Relative to the tree holding the redirect, which is what bd resolves it
	// against — measured, see the header.
	{"relative to the tree", func(t *testing.T, s, tree string) string {
		rel, err := filepath.Rel(tree, s)
		if err != nil {
			t.Fatalf("rel(%q, %q): %v", tree, s, err)
		}
		return rel
	}},
	// A symlinked spelling: a different path naming the same directory, which
	// no amount of string normalisation reaches and `-ef` answers directly.
	{"through a symlink", func(t *testing.T, s, _ string) string {
		link := filepath.Join(t.TempDir(), "constitution-link")
		if err := os.Symlink(filepath.Dir(s), link); err != nil {
			t.Skipf("symlinks unavailable here: %v", err)
		}
		return filepath.Join(link, filepath.Base(s))
	}},
}

// qcResolve is what bd does with a redirect's first line before opening it:
// trim the blanks (a CR from a CRLF ending among them), and read a relative
// path against the tree that holds the redirect.
func qcResolve(spelled, tree string) string {
	p := strings.TrimSpace(spelled)
	if !filepath.IsAbs(p) {
		p = filepath.Join(tree, p)
	}
	return p
}

// qcSameDir is the setup witness every arm below runs, and it holds on both
// sides of the fix: it asks the FILESYSTEM whether the spelling and the store
// are one directory. Without it a green could come from a spelling that points
// nowhere — a tree correctly left alone rather than a bug found.
func qcSameDir(t *testing.T, spelled, tree, store string) {
	t.Helper()
	resolved := qcResolve(spelled, tree)
	a, err := os.Stat(resolved)
	if err != nil {
		t.Fatalf("fixture does not name a directory: %q resolves to %q: %v", spelled, resolved, err)
	}
	b, err := os.Stat(store)
	if err != nil {
		t.Fatalf("store %q: %v", store, err)
	}
	if !os.SameFile(a, b) {
		t.Fatalf("fixture does not name the store: %q resolves to %q, want the same directory as %q",
			spelled, resolved, store)
	}
}

// A forgotten tree whose redirect names the constitution in any spelling bd
// accepts must be brought along by the fan-out.
func TestQueueCutoverFindsAForgottenTreeWhateverTheSpelling(t *testing.T) {
	t.Parallel()
	for _, sp := range qcSloppySpellings {
		t.Run(sp.name, func(t *testing.T) {
			constitution, _ := qcConstitution(t)
			store := filepath.Join(constitution, ".beads")
			queue := filepath.Join(t.TempDir(), "queue")
			root := filepath.Dir(constitution)

			// The forgotten tree, spelled sloppily. Beside the constitution,
			// so it is inside the derived scan root.
			forgotten := filepath.Join(root, "retired-checkout-"+strings.ReplaceAll(sp.name, " ", "-"))
			if err := os.MkdirAll(filepath.Join(forgotten, ".beads"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(forgotten) })
			spelled := sp.spell(t, store, forgotten)
			if err := os.WriteFile(filepath.Join(forgotten, ".beads", beadsRedirect),
				[]byte(spelled+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			qcSameDir(t, spelled, forgotten, store)

			project := qcWork(t, t.TempDir(), store)
			out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project})
			if err != nil {
				t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
			}

			dst := filepath.Join(queue, ".beads")
			got := qcRedirect(t, forgotten)
			if filepath.Clean(strings.TrimSpace(got)) != filepath.Clean(dst) {
				t.Errorf("a forgotten tree spelled %q still redirects to %q, want %q\n"+
					"that is hop one of the two-hop chain ranger-base-l9aa was filed for\n%s",
					spelled, got, dst, out)
			}
		})
	}
}

// The wrong arm for the pin above, and the one that makes the loosened compare
// safe to have: the same spellings pointed at a DIFFERENT store, which
// the fan-out must not touch. A compare loosened by normalising strings until
// something matches would pass the pin above and fail this one.
func TestQueueCutoverLeavesAStrangerStoreAloneHoweverItIsSpelled(t *testing.T) {
	t.Parallel()
	for _, sp := range qcSloppySpellings {
		t.Run(sp.name, func(t *testing.T) {
			constitution, _ := qcConstitution(t)
			store := filepath.Join(constitution, ".beads")
			queue := filepath.Join(t.TempDir(), "queue")
			root := filepath.Dir(constitution)

			// A store that is real and is not the constitution's. It has to
			// EXIST: a redirect naming a path that is not there is one bd
			// refuses to follow, so leaving it alone would prove nothing.
			elsewhere := filepath.Join(t.TempDir(), "someone-elses-store", ".beads")
			if err := os.MkdirAll(elsewhere, 0o755); err != nil {
				t.Fatal(err)
			}

			stranger := filepath.Join(root, "unrelated-repo-"+strings.ReplaceAll(sp.name, " ", "-"))
			if err := os.MkdirAll(filepath.Join(stranger, ".beads"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(stranger) })
			spelled := sp.spell(t, elsewhere, stranger)
			if err := os.WriteFile(filepath.Join(stranger, ".beads", beadsRedirect),
				[]byte(spelled+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			// Witness, on both sides again: this spelling really does name the
			// OTHER store, so a green here is "left alone" and not "pointed
			// nowhere".
			qcSameDir(t, spelled, stranger, elsewhere)

			project := qcWork(t, t.TempDir(), store)
			out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project})
			if err != nil {
				t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
			}

			// Byte for byte: the fan-out has no business rewriting this file
			// at all, not even into a tidier spelling of the same store.
			b, err := os.ReadFile(filepath.Join(stranger, ".beads", beadsRedirect))
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != spelled+"\n" {
				t.Errorf("a tree pointed at an unrelated store was rewritten:\ngot  %q\nwant %q\n%s",
					string(b), spelled+"\n", out)
			}
		})
	}
}

// The same defect one line further down the same loop. The fan-out refuses to
// write a redirect INSIDE the queue repo — that is a one-hop cycle, bd
// resolves it to the directory it is already in, and the cutover looks fine
// until something follows the chain twice. That guard was a string compare
// too, so `--redirect <queue>/` with one trailing slash walked straight past
// it and wrote the cycle (ranger-base-4myz).
func TestQueueCutoverWritesNoSelfRedirectWhenTheQueueIsNamedSloppily(t *testing.T) {
	t.Parallel()
	constitution, _ := qcConstitution(t)
	store := filepath.Join(constitution, ".beads")
	queue := filepath.Join(t.TempDir(), "queue")

	project := qcWork(t, t.TempDir(), store)
	// The queue repo itself, named with a trailing slash, alongside an
	// ordinary target so the fan-out has real work to do either way.
	out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project, queue + "/"})
	if err != nil {
		t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
	}

	if got := qcRedirect(t, queue); got != "" {
		t.Errorf("the queue repo redirects to %q — at itself, a one-hop cycle\n%s", got, out)
	}
	// And the ordinary target still got its redirect, so a green above is the
	// guard firing rather than the fan-out doing nothing.
	if got, want := qcRedirect(t, project), filepath.Join(queue, ".beads"); got != want {
		t.Errorf("the named repo was skipped too: %q, want %q\n%s", got, want, out)
	}
}

// The same compare, in the rollback block — which is the OTHER direction of
// the same walk: after a cutover it finds every tree pointed at the queue and
// sends it home. That loop compared bytes too, and this pin runs the block the
// script prints (qcRollbackBlock) rather than asserting over its wording, which
// is how the two pins above it in queuecutover_qa_test.go work.
//
// Three planted trees, and they discriminate in both directions: the
// byte-exact one must come home (so a green is the loop RUNNING, not the loop
// missing everything), the sloppily-spelled one must come home (the bead), and
// one pointed at a third store must be left alone (no over-match).
func TestQueueRollbackSendsHomeATreeSpelledAHandsWay(t *testing.T) {
	t.Parallel()
	f := qcRolledBack(t)
	root := filepath.Dir(f.constitution)
	queueStore := filepath.Join(f.queue, ".beads")
	home := filepath.Join(f.constitution, ".beads")

	plant := func(name, redirect string) string {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		if err := os.WriteFile(filepath.Join(dir, ".beads", beadsRedirect),
			[]byte(redirect+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	// A third store, real, and nobody's business here.
	third := filepath.Join(t.TempDir(), "a-third-store", ".beads")
	if err := os.MkdirAll(third, 0o755); err != nil {
		t.Fatal(err)
	}

	exact := plant("straggler-exact", queueStore)
	sloppy := plant("straggler-sloppy", filepath.Dir(queueStore)+"/./"+filepath.Base(queueStore)+"/ ")
	stranger := plant("straggler-stranger", third)
	qcSameDir(t, qcRedirectRaw(t, sloppy), sloppy, queueStore)

	out := qcRollbackRun(t, qcRollbackBlock(t, f), f)

	if got := qcRedirect(t, exact); got != home {
		t.Fatalf("the rollback's walk did not move a BYTE-EXACT straggler (%q, want %q), "+
			"so this rig measures nothing about spelling\n%s", got, home, out)
	}
	if got := qcRedirect(t, sloppy); got != home {
		t.Errorf("a straggler spelled a hand's way still redirects to %q, want %q\n"+
			"after the store has gone home that is a dangling redirect\n%s", got, home, out)
	}
	if got, want := qcRedirect(t, stranger), third; got != want {
		t.Errorf("the rollback rewrote a tree pointed at an unrelated store: %q, want %q\n%s", got, want, out)
	}
}

// qcRedirectRaw is qcRedirect without the trim — the bytes on disk, which is
// what a spelling pin has to witness.
func qcRedirectRaw(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".beads", beadsRedirect))
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
	return strings.TrimSuffix(string(b), "\n")
}

// The third site, and the one nobody executes: when the script aborts in
// stage `fanout` it prints a `find | while read` snippet for the operator to
// paste — "list the stragglers, then finish them by hand". That snippet
// carried the same byte-exact compare, and it is the recovery path for the
// half-state where getting the list wrong is most expensive.
//
// So run it. The abort is forced by making one target's redirect unwritable,
// the stragglers are planted AFTER the abort so the run cannot have touched
// them, and the snippet is lifted out of the script's own stderr rather than
// retyped here — a pin over a retyped copy goes green the day the heredoc's
// escaping breaks.
func TestQueueCutoverFanoutRecoveryListingFindsAStragglerSpelledAHandsWay(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root writes through the 0444 this fixture blocks the fan-out with")
	}
	constitution, _ := qcConstitution(t)
	src := filepath.Join(constitution, ".beads")
	queue := filepath.Join(t.TempDir(), "queue")
	root := filepath.Dir(constitution)

	// An unwritable redirect in a named target: the fan-out's write fails and
	// `set -e` takes the script out inside stage fanout. Not the directory
	// mode — the file already exists, so a read-only directory would let the
	// truncate through and the fixture would measure nothing.
	project := qcWork(t, t.TempDir(), src)
	victim := filepath.Join(project, ".beads", beadsRedirect)
	if err := os.Chmod(victim, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(victim, 0o644) })

	out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project})
	if err == nil {
		t.Fatalf("the fixture did not block the fan-out, so this pin measures nothing:\n%s", out)
	}
	if !strings.Contains(out, `stage "fanout"`) {
		t.Fatalf("the abort landed somewhere other than the fan-out:\n%s", out)
	}
	if got := qcRedirect(t, project); got != src {
		t.Fatalf("the fixture blocked something other than the redirect write (%q):\n%s", got, out)
	}

	// The stragglers, planted after the abort: one spelled the way the script
	// writes them, one spelled a hand's way, one pointed somewhere else.
	plant := func(name, redirect string) string {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		if err := os.WriteFile(filepath.Join(dir, ".beads", beadsRedirect),
			[]byte(redirect+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	third := filepath.Join(t.TempDir(), "a-third-store", ".beads")
	if err := os.MkdirAll(third, 0o755); err != nil {
		t.Fatal(err)
	}
	exact := plant("left-behind-exact", src)
	sloppy := plant("left-behind-sloppy", " "+filepath.Dir(src)+"/./"+filepath.Base(src)+"/")
	stranger := plant("left-behind-stranger", third)
	qcSameDir(t, qcRedirectRaw(t, sloppy), sloppy, src)

	listing := qcFanoutRecoveryListing(t, out)
	if strings.Contains(listing, "~/") || !strings.Contains(listing, root) {
		t.Fatalf("the snippet does not walk the fixture, refusing to run it:\n%s", listing)
	}
	got, runErr := exec.Command("sh", "-c", listing).CombinedOutput()
	if runErr != nil {
		t.Fatalf("the snippet the operator is told to paste does not run: %v\n%s\n%s", runErr, listing, got)
	}
	found := string(got)

	// The control first: a snippet that listed nothing would satisfy the
	// stranger assertion below on its own.
	if !strings.Contains(found, filepath.Join(exact, ".beads")) {
		t.Fatalf("the recovery listing missed a BYTE-EXACT straggler, so it measures nothing about spelling\n%s\nlisted:\n%s", listing, found)
	}
	if !strings.Contains(found, filepath.Join(sloppy, ".beads")) {
		t.Errorf("the recovery listing missed a straggler spelled a hand's way — the operator finishes the fleet by hand from this list\n%s\nlisted:\n%s", listing, found)
	}
	if strings.Contains(found, filepath.Join(stranger, ".beads")) {
		t.Errorf("the recovery listing named a tree pointed at an unrelated store\n%s\nlisted:\n%s", listing, found)
	}
}

// qcFanoutRecoveryListing lifts the `find … | while read … done` block out of
// the stage-fanout abort message — the part the operator pastes to list what
// the fan-out did not reach.
func qcFanoutRecoveryListing(t *testing.T, out string) string {
	t.Helper()
	const start = "    find '"
	i := strings.Index(out, start)
	if i < 0 {
		t.Fatalf("the fanout abort prints no listing to paste:\n%s", out)
	}
	rest := out[i:]
	const end = "\n      done"
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatalf("the listing in the fanout abort is unterminated:\n%s", rest)
	}
	return rest[:j+len(end)]
}
