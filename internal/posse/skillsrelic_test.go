package posse

// ranger-base-f6hiy: the post-cutover relic. A launch of developer on codex
// printed
//
//	developer: skills: ~/src/posse/.agents/skills/distributed-systems exists
//	and was not written by posse — not overwriting
//
// and refused, every time, for days. The entry was a symlink dated
// 2026-08-24 into ~/.config/rhq/skills/<name> — the pre-ADR-0015 home, gone
// since the cutover. It resolved to nothing,
// it bound nothing, and the only thing it did was hold the launch. Removing
// it by hand let the same command through immediately.
//
// The never-clobber rule (RenderAgentsSkills) is right about a file the
// operator wrote; it was never meant to defend a link into a home that no
// longer exists. So a DANGLING link is replaced, and everything else that
// is not ours still refuses — which is the half these pins have to hold
// down, because widening the exception is the way this fix goes wrong.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// relicRepo is a git repo with an .agents/skills dir, and a home whose
// skills/ carries name.
func relicRepo(t *testing.T, name string) (a *App, repo, dir string) {
	t.Helper()
	home := t.TempDir()
	a = &App{Home: home, StateDir: filepath.Join(home, "state")}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	mkSkill(t, a.SkillsDir(), name)
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	dir = AgentsSkillsDir(repo)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return a, repo, dir
}

// The fix itself: a link resolving to nothing is replaced, not refused, and
// the skill is reachable through the new one. The old home is spelled the
// way the incident spelled it — a path that never existed under this test's
// HOME — because that is the whole shape: the target is not merely missing
// from RHQ_HOME/skills, it is missing from the filesystem.
//
// Two rows, because "missing" has two errnos and only one of them was the
// incident's. A retired home DELETED is ENOENT; the same home archived in
// place as a file rather than a directory is ENOTDIR, and a path with an
// ordinary file in the middle of it cannot resolve now or later either
// (ranger-base-epdyv, escaped from the ranger-base-f6hiy close). The second
// row is the one that fails on the ENOENT-only predicate.
func TestRenderAgentsSkillsReplacesADanglingRelic(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		plant func(t *testing.T, relic string)
	}{
		{"the retired home is gone (ENOENT)", func(t *testing.T, relic string) {
			retired := filepath.Join(t.TempDir(), "retired-home", "skills", "distributed-systems")
			if err := os.Symlink(retired, relic); err != nil {
				t.Fatal(err)
			}
		}},
		{"the retired home was archived in place as a file (ENOTDIR)", func(t *testing.T, relic string) {
			archived := filepath.Join(t.TempDir(), "rhq")
			if err := os.WriteFile(archived, []byte("tarball of the retired home\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(archived, "skills", "distributed-systems"), relic); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, repo, dir := relicRepo(t, "distributed-systems")
			relic := filepath.Join(dir, "distributed-systems")
			c.plant(t, relic)

			if _, err := a.RenderAgentsSkills(repo, "developer", []string{"distributed-systems"}); err != nil {
				t.Fatalf("a dangling relic must not refuse the launch: %v", err)
			}
			got, err := os.Readlink(relic)
			if err != nil || got != a.SkillPath("distributed-systems") {
				t.Errorf("relic not replaced: %q (%v), want %s", got, err, a.SkillPath("distributed-systems"))
			}
			if _, err := os.Stat(filepath.Join(relic, "SKILL.md")); err != nil {
				t.Errorf("SKILL.md not reachable through the replacement: %v", err)
			}
		})
	}
}

// The control arm, and the one that decides whether the fix is a fix or a
// hole: every OTHER kind of entry we did not write still refuses. Each row
// is a thing that resolves — so it is somebody's file, and posse may not
// have it — except the last three, whose targets exist but do not resolve
// as asked: an error that is not evidence the target is gone keeps the
// refusal. The two trailing-slash rows make that concrete for the errno the
// relic rule now accepts (ENOTDIR), so the two pins disagree in the fixture
// and not only in the prose; the second of them is the shape that WAS
// clobbered until ranger-base-jhyiv.
func TestRenderAgentsSkillsStillRefusesEveryLiveForeignEntry(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		plant func(t *testing.T, link string)
	}{
		{"a real dir the repo owns", func(t *testing.T, link string) {
			if err := os.MkdirAll(filepath.Join(link, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"a plain file", func(t *testing.T, link string) {
			if err := os.WriteFile(link, []byte("mine\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"a live symlink somewhere else", func(t *testing.T, link string) {
			elsewhere := filepath.Join(t.TempDir(), "elsewhere")
			if err := os.MkdirAll(elsewhere, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(elsewhere, link); err != nil {
				t.Fatal(err)
			}
		}},
		{"a symlink loop", func(t *testing.T, link string) {
			if err := os.Symlink(link, link); err != nil {
				t.Fatal(err)
			}
		}},
		{"a live target spelled with a trailing slash", func(t *testing.T, link string) {
			live := filepath.Join(t.TempDir(), "skills", "distributed-systems")
			if err := os.MkdirAll(filepath.Dir(live), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(live, []byte("the operator's own file\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			// os.Stat through this link is ENOTDIR — the trailing slash asks
			// the kernel for a directory — and the file it names is sitting
			// right there. The mirror of the ENOTDIR replace row: the errno
			// alone does not decide, the target does.
			if err := os.Symlink(live+"/", link); err != nil {
				t.Fatal(err)
			}
		}},
		{"a live target reached through a symlinked component and a ..", func(t *testing.T, link string) {
			// The same trailing-slash liar as the row above, one step
			// further out: the kernel walks sym -> store/home and then ".."
			// back to store, so the target IS store/distributed-systems and
			// it is live. A second ask that collapses that ".." lexically
			// goes to a path the kernel never visits, finds nothing, and
			// clobbers the operator's entry — measured end to end under
			// ranger-base-han3i, fixed under ranger-base-jhyiv. This row is
			// the end-to-end half; TestDanglingSkillLinkErrnoTable's row 12
			// is the predicate half.
			store := filepath.Join(t.TempDir(), "store")
			if err := os.MkdirAll(filepath.Join(store, "home"), 0o755); err != nil {
				t.Fatal(err)
			}
			live := filepath.Join(store, "distributed-systems")
			if err := os.WriteFile(live, []byte("the operator's own skill\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			links := filepath.Join(t.TempDir(), "links")
			if err := os.MkdirAll(links, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(store, "home"), filepath.Join(links, "sym")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(links, "sym")+"/../distributed-systems/", link); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(live); err != nil {
				t.Fatalf("the fixture's target must be live before the call: %v", err)
			}
		}},
		{"a link under a dir this uid cannot traverse", func(t *testing.T, link string) {
			if os.Geteuid() == 0 {
				t.Skip("root traverses 0000 — the unreachable arm cannot exist here")
			}
			walled := filepath.Join(t.TempDir(), "walled")
			if err := os.MkdirAll(filepath.Join(walled, "distributed-systems"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(walled, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(walled, 0o755) })
			if err := os.Symlink(filepath.Join(walled, "distributed-systems"), link); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, repo, dir := relicRepo(t, "distributed-systems")
			link := filepath.Join(dir, "distributed-systems")
			c.plant(t, link)
			before, _ := os.Lstat(link)

			_, err := a.RenderAgentsSkills(repo, "developer", []string{"distributed-systems"})
			if err == nil || !strings.Contains(err.Error(), "not overwriting") {
				t.Fatalf("must still refuse: %v", err)
			}
			after, statErr := os.Lstat(link)
			if statErr != nil {
				t.Fatalf("the refused entry was removed anyway: %v", statErr)
			}
			if before != nil && before.Mode() != after.Mode() {
				t.Errorf("the refused entry changed: %v -> %v", before.Mode(), after.Mode())
			}
		})
	}
}

// The union rule survives the exception: a relic under a name this launch
// is not binding is nobody's business here, and posse leaves it alone
// rather than sweeping the operator's dir on its way past. Only our own
// dead links are swept (sweepDeadSkillLinks), and that is unchanged.
func TestRenderAgentsSkillsLeavesAnUnboundRelicAlone(t *testing.T) {
	t.Parallel()
	a, repo, dir := relicRepo(t, "distributed-systems")
	other := filepath.Join(dir, "someone-elses")
	retired := filepath.Join(t.TempDir(), "retired-home", "skills", "someone-elses")
	if err := os.Symlink(retired, other); err != nil {
		t.Fatal(err)
	}

	if _, err := a.RenderAgentsSkills(repo, "developer", []string{"distributed-systems"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(other)
	if err != nil || got != retired {
		t.Errorf("an unbound entry must be left as it is: %q (%v)", got, err)
	}
}
