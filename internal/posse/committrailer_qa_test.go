//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-5aks asked whether the commit wall eats the Co-Authored-By
// runtime trailer: five consecutive crew commits on main carried none, and
// they were the five after rangerhq-lt2w's wall change landed.
//
// It does not, and the run was not a run. The measurement is on the bead;
// the two facts that have to stay true are pinned here.
//
//   - The wall is a REFUSAL, not a rewrite. prepare-commit-msg is the one
//     hook in the chain git hands the message file, so it is the only place
//     in posse that could edit a message — and it never opens $1 for write.
//     A commit that goes through the safe form keeps the trailer it was
//     given. Measured, in both spellings of the safe form.
//   - So the trailer's 60% rate on main is the model typing it or not, and
//     AGENTS.md says so rather than leaving the next reader to re-derive it.
//
// The cell below carries its own control (an unqualified commit must be
// refused, or the wall is not live in the fixture and the green arm means
// nothing) and its own wrong arm (the same safe form with no trailer typed
// must read zero, or the assertion is a constant).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const runtimeTrailer = "Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"

// TestQACommitWallPreservesTheRuntimeTrailer is ranger-base-5aks's DONE WHEN:
// one commit through the safe form with a trailer, observe survival.
func TestQACommitWallPreservesTheRuntimeTrailer(t *testing.T) {
	t.Parallel()
	repo, git, persona := commitWallRepo(t)

	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// trailers reads the landed commit, never the message we handed git.
	trailers := func() string {
		t.Helper()
		out, err := git(nil, "log", "-1", "--format=%(trailers:key=Co-Authored-By,valueonly)")
		if err != nil {
			t.Fatalf("git log: %v %s", err, out)
		}
		return strings.TrimSpace(out)
	}
	msgFile := func(name, body string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	write("a.txt", "base\n")
	write("b.txt", "base\n")
	git(nil, "add", "a.txt", "b.txt")
	if out, err := git(nil, "commit", "-qm", "init", "--", "a.txt", "b.txt"); err != nil {
		t.Fatalf("fixture commit: %v %s", err, out)
	}
	head := func() string {
		out, err := git(nil, "rev-parse", "HEAD")
		if err != nil {
			t.Fatalf("rev-parse: %v %s", err, out)
		}
		return strings.TrimSpace(out)
	}

	// CONTROL. Without this the three arms below prove only that a repo with
	// no wall in it leaves messages alone.
	write("a.txt", "base\ncontrol\n")
	git(nil, "add", "a.txt")
	before := head()
	if out, err := git(persona, "commit", "-m", "control: unqualified\n\n"+runtimeTrailer); err == nil {
		t.Fatalf("the wall is not live in this fixture — an unqualified commit landed:\n%s", out)
	}
	if head() != before {
		t.Fatal("the wall refused but the commit landed anyway")
	}

	// The safe form, spelled -F. This is the form AGENTS.md prescribes.
	write("a.txt", "base\nF\n")
	p := msgFile("f.msg", "test: safe form, -F route\n\nA body, so the trailer is a trailer block.\n\n"+runtimeTrailer+"\n")
	if out, err := git(persona, "commit", "-q", "-F", p, "--", "a.txt"); err != nil {
		t.Fatalf("the safe form must land: %v %s", err, out)
	}
	if got := trailers(); got != "Claude Opus 5 <noreply@anthropic.com>" {
		t.Errorf("the -F safe form dropped the runtime trailer: %q", got)
	}

	// The safe form, spelled -m. Three of the five commits on the bead used
	// this one, so it gets its own arm rather than riding on -F's.
	write("b.txt", "base\nm\n")
	if out, err := git(persona, "commit", "-q", "-m",
		"test: safe form, -m route\n\nA body.\n\n"+runtimeTrailer, "--", "b.txt"); err != nil {
		t.Fatalf("the -m safe form must land: %v %s", err, out)
	}
	if got := trailers(); got != "Claude Opus 5 <noreply@anthropic.com>" {
		t.Errorf("the -m safe form dropped the runtime trailer: %q", got)
	}

	// WRONG ARM. Same repo, same wall, same form, no trailer typed. If this
	// reads a trailer the two arms above were reading something that is not
	// the commit.
	write("a.txt", "base\nnone\n")
	p = msgFile("none.msg", "test: safe form, no trailer typed\n")
	if out, err := git(persona, "commit", "-q", "-F", p, "--", "a.txt"); err != nil {
		t.Fatalf("the safe form must land: %v %s", err, out)
	}
	if got := trailers(); got != "" {
		t.Errorf("a commit whose message had no trailer reports one: %q — the arms above measure nothing", got)
	}
}

// TestQAAgentsMDNamesTheTrailerAsUnenforced keeps AGENTS.md's answer where
// the next reader of a trailer-less commit will find it. The claim it makes
// is the one the cell above measures, so the two go stale together.
func TestQAAgentsMDNamesTheTrailerAsUnenforced(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Skipf("AGENTS.md not present: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		"Co-Authored-By",
		"ranger-base-5aks",
		"No gate adds it and no gate removes it",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("AGENTS.md no longer names %q — the next reader of a trailer-less commit has to re-derive ranger-base-5aks", want)
		}
	}
}
