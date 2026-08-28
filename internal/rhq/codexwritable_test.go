package rhq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A codex session's writes are confined to its workspace, so the store of
// record — which ADR 0012 D3-C puts behind a .beads redirect, outside the
// session dir — has to be named or every bd write is denied (ranger-base-0fb).
func TestCodexAddsTheBeadsRedirectTargetAsWritable(t *testing.T) {
	r := realizeCodex(nil, nil, "/m/personas/developer", "/elsewhere/ranger-base/.beads")
	want := "-s workspace-write --add-dir '/m/personas/developer' --add-dir '/elsewhere/ranger-base/.beads'"
	if r.Deny != want {
		t.Fatalf("codex sandbox flags:\n got %q\nwant %q", r.Deny, want)
	}
}

// read-only never takes --add-dir: codex exits on it (rangerhq-5oi).
func TestCodexReadOnlyStillRefusesAddDir(t *testing.T) {
	r := realizeCodex(nil, []string{"Edit", "Write"}, "/m", "/elsewhere/.beads")
	if r.Deny != "-s read-only" {
		t.Fatalf("read-only must carry no --add-dir, got %q", r.Deny)
	}
}

// The same path named twice must not produce two flags.
func TestCodexWritableDedupes(t *testing.T) {
	r := realizeCodex(nil, nil, "/same", "/same")
	if r.Deny != "-s workspace-write --add-dir '/same'" {
		t.Fatalf("dedupe failed: %q", r.Deny)
	}
}

// Runtimes posse cages itself must be untouched by the new parameter.
func TestClaudeAndGrokIgnoreWritable(t *testing.T) {
	c := realizeClaude(nil, nil, "/m", "/elsewhere/.beads")
	g := realizeGrok(nil, nil, "/m", "/elsewhere/.beads")
	for n, got := range map[string]string{"claude": c.Deny, "grok": g.Deny} {
		if containsAddDir(got) {
			t.Fatalf("%s must not emit --add-dir, got %q", n, got)
		}
	}
}

func containsAddDir(s string) bool { return strings.Contains(s, "--add-dir") }

// The four tests above pin realizeCodex in ISOLATION, which is not where the
// bug was. The bug was one argument at the launch site: drop beadsHome(dir)
// from planLaunch's RenderCommandFor call and every test above stays green
// while dispatched codex sessions go silent again — and it took five of them
// before anyone read the silence as a cage rather than an agent skipping its
// bookkeeping (ranger-base-0fb). So pin the LINE, in the shape dispatch
// really launches it: a session worktree (rangerhq-09o2) of a repo whose
// .beads is an ADR 0012 D3-C redirect.
func TestCodexLaunchLineNamesTheStoreOfRecord(t *testing.T) {
	b, fake := newTestBackend(t) // its own temp $HOME
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nruntime: codex\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}

	store := blRepo(t)
	work := wtRepo(t)
	target := filepath.Join(store, beadsDirName)
	if err := os.MkdirAll(filepath.Join(work, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	blRedirect(t, work, target)

	// No AllowDegraded, and that is half the pin (ranger-base-xqwr): ADR
	// 0013 §4's reachability row judges this very line, and until the
	// store's git dirs rode it the row was unrealized and dispatch — which
	// never allows degradation on its own (ADR 0002 §4) — could not launch
	// codex in the fleet's own shape at all. A create that returns is the
	// row realized.
	mustCreate(t, b, NewSessionOpts{Name: "crew", Agent: "ranger", Dir: work, Worktree: true})

	tree, err := b.App.SessionTreePath(work, "crew")
	if err != nil {
		t.Fatal(err)
	}
	gits := LinkedGitDirs(tree)
	if len(gits) != 2 {
		t.Fatalf("no session worktree was made — the test asserts nothing: LinkedGitDirs(%s) = %v", tree, gits)
	}
	if h := beadsHome(tree); h != target {
		t.Fatalf("the seeded worktree redirect does not reach the store: beadsHome = %q, want %q", h, target)
	}

	// The STORE's git dirs are the third grant, and the one this bead is
	// about: `bd sync` COMMITS the JSONL in that repo, so it takes
	// index.lock there and reads hooks and refs beside it. With .beads
	// granted and these not, the session records nothing for the same
	// reason the pre-23c4e54 seatbelt recorded nothing (ranger-base-rhw).
	storeGits := beadsGitDirs(target)
	if len(storeGits) == 0 {
		t.Fatalf("no git dirs resolved for the store at %s — the assertion below would be empty", target)
	}

	// A line this long spills to the launch script, so the typed line is
	// the call log and that script together (paneline.go).
	body, _ := os.ReadFile(b.App.LaunchScript("crew"))
	log := calls(t, fake) + "\n" + string(body)
	// The store of record, the git dirs that repo's `bd sync` commit locks,
	// and the git dirs that hold this tree's index and the repo's objects:
	// all outside the workspace, all denied unless named.
	for _, want := range append(append([]string{target}, storeGits...), gits...) {
		if !strings.Contains(log, "--add-dir "+shellQuote(want)) {
			t.Errorf("codex launch does not name %s writable:\n%s", want, log)
		}
	}
}
