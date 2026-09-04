package posse

// QA probe for ranger-base-epdyv finding 1 — the errno table danglingSkillLink
// actually implements, asked of the real function rather than read off its
// comment. The close widened "gone" from ENOENT to ENOENT-or-ENOTDIR and asks
// ENOTDIR a second time on the cleaned readlink value so a live target spelled
// with a trailing slash cannot be clobbered.
//
// The function's own comment makes a claim this file exists to test: the
// second ask "can turn a replace into a refusal and never the other way."
// That is a statement about a lexical filepath.Clean disagreeing with the
// kernel, so the row that decides it is a target reached through a SYMLINKED
// component that a lexical ".." collapse cannot reproduce — row 12 below.
//
// Every row is the real danglingSkillLink over a real filesystem. Rows are
// named for the accident that produces them, and each says which side of the
// never-clobber rule its answer sits on.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDanglingSkillLinkErrnoTable(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		want bool
		// hole marks a row whose want is the MEASURED answer and not the
		// right one — a live-defect row, green today, and its message says
		// which way the answer has to move when the defect is fixed.
		hole string
		// plant returns the target spelling for a symlink at link; the
		// symlink itself is created by the harness so every row is one shape.
		plant func(t *testing.T, root string) (target string)
	}{
		{"the target is gone (ENOENT)", true, "", func(t *testing.T, root string) string {
			return filepath.Join(root, "retired-home", "skills", "x")
		}},
		{"a component of the target is an ordinary file (ENOTDIR)", true, "", func(t *testing.T, root string) string {
			archived := filepath.Join(root, "rhq")
			mustWrite(t, archived, "tarball of the retired home")
			return filepath.Join(archived, "skills", "x")
		}},
		{"the target itself is a live file (nil)", false, "", func(t *testing.T, root string) string {
			live := filepath.Join(root, "live.md")
			mustWrite(t, live, "the operator's own file")
			return live
		}},
		{"a live FILE spelled with a trailing slash (ENOTDIR, target present)", false, "", func(t *testing.T, root string) string {
			live := filepath.Join(root, "live.md")
			mustWrite(t, live, "the operator's own file")
			return live + "/"
		}},
		{"a live DIR spelled with a trailing slash (nil)", false, "", func(t *testing.T, root string) string {
			live := filepath.Join(root, "livedir")
			if err := os.MkdirAll(live, 0o755); err != nil {
				t.Fatal(err)
			}
			return live + "/"
		}},
		{"a live file spelled with two trailing slashes", false, "", func(t *testing.T, root string) string {
			live := filepath.Join(root, "live.md")
			mustWrite(t, live, "the operator's own file")
			return live + "//"
		}},
		{"a live file spelled with a trailing /.", false, "", func(t *testing.T, root string) string {
			live := filepath.Join(root, "live.md")
			mustWrite(t, live, "the operator's own file")
			return live + "/."
		}},
		{"a RELATIVE target, live file, trailing slash", false, "", func(t *testing.T, root string) string {
			mustWrite(t, filepath.Join(root, "sibling.md"), "the operator's own file")
			return "sibling.md/"
		}},
		{"a relative target that is gone", true, "", func(t *testing.T, root string) string {
			_ = root
			return "no-such-sibling"
		}},
		{"the target is itself a dangling symlink (ENOENT through a chain)", true, "", func(t *testing.T, root string) string {
			mid := filepath.Join(root, "mid")
			if err := os.Symlink(filepath.Join(root, "nowhere"), mid); err != nil {
				t.Fatal(err)
			}
			return mid
		}},
		{"a symlink loop (ELOOP is not evidence the target is gone)", false, "", func(t *testing.T, root string) string {
			a := filepath.Join(root, "loopa")
			b := filepath.Join(root, "loopb")
			if err := os.Symlink(b, a); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(a, b); err != nil {
				t.Fatal(err)
			}
			return a
		}},
		// The row the comment's "never the other way" claim rests on. The
		// kernel resolves links/sym/../live.md to store/live.md, a live file;
		// the trailing slash makes the FIRST ask ENOTDIR anyway, exactly as
		// row 4. filepath.Clean then collapses ".." lexically to
		// links/live.md, which is not there — so the SECOND ask reads ENOENT
		// on a target that exists, and the answer flips a refusal into a
		// replace. want here is the MEASURED answer; see the hole note.
		{"a live file reached through a symlinked component and a .. (trailing slash)", true,
			"want false: the target is a live file, so the never-clobber rule applies. " +
				"true is the measured hole (ranger-base-epdyv), and it falsifies the " +
				"function comment's claim that the lexical second ask can only ever turn a " +
				"replace into a refusal. When this row goes red, the fix has landed: flip " +
				"want to false, drop this note, and delete " +
				"TestRenderAgentsSkillsClobbersALiveTargetReachedThroughASymlinkedDotDot",
			func(t *testing.T, root string) string {
				store := filepath.Join(root, "store")
				home := filepath.Join(store, "home")
				if err := os.MkdirAll(home, 0o755); err != nil {
					t.Fatal(err)
				}
				mustWrite(t, filepath.Join(store, "live.md"), "the operator's own file")
				links := filepath.Join(root, "links")
				if err := os.MkdirAll(links, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(home, filepath.Join(links, "sym")); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(links, "sym") + "/../live.md/"
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			target := c.plant(t, root)
			link := filepath.Join(root, "link")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			// What the kernel says the target is, so a failure reads as a
			// fact about the filesystem and not only about the predicate.
			_, statErr := os.Stat(link)
			got := danglingSkillLink(link)
			if got == c.want {
				return
			}
			msg := ""
			if c.hole != "" {
				msg = "\n  THIS ROW IS A LIVE-DEFECT ROW: " + c.hole
			}
			t.Errorf("danglingSkillLink = %v, want %v\n  link   %s\n  target %s\n  stat(link) %v%s",
				got, c.want, link, target, statErr, msg)
		})
	}
}

// TestRenderAgentsSkillsClobbersALiveTargetReachedThroughASymlinkedDotDot is
// the end-to-end consequence of row 12: the same shape planted as the
// operator's own entry in .agents/skills, through the real
// RenderAgentsSkills. The never-clobber rule says an entry posse did not
// write and whose target is sitting right there must refuse the launch. It
// does not: the link is removed and rewritten.
//
// LIVE-DEFECT PIN — this ships GREEN by asserting the hole, so the failure
// message carries the inversion. When danglingSkillLink stops calling this
// shape a relic, THIS test goes red and the row belongs in
// TestRenderAgentsSkillsStillRefusesEveryLiveForeignEntry beside "a live
// target spelled with a trailing slash".
func TestRenderAgentsSkillsClobbersALiveTargetReachedThroughASymlinkedDotDot(t *testing.T) {
	t.Parallel()
	a, repo, dir := relicRepo(t, "distributed-systems")
	link := filepath.Join(dir, "distributed-systems")

	root := t.TempDir()
	store := filepath.Join(root, "store")
	if err := os.MkdirAll(filepath.Join(store, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(store, "distributed-systems")
	mustWrite(t, live, "the operator's own skill")
	links := filepath.Join(root, "links")
	if err := os.MkdirAll(links, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(store, "home"), filepath.Join(links, "sym")); err != nil {
		t.Fatal(err)
	}
	// The kernel resolves this to store/distributed-systems, a live file.
	target := filepath.Join(links, "sym") + "/../distributed-systems/"
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("the fixture's target must be live before the call: %v", err)
	}

	_, err := a.RenderAgentsSkills(repo, "developer", []string{"distributed-systems"})
	got, readErr := os.Readlink(link)
	switch {
	case err == nil && readErr == nil && got == a.SkillPath("distributed-systems"):
		t.Logf("MEASURED (ranger-base-epdyv escape): the operator's entry was replaced.\n"+
			"  was %s\n  now %s\n"+
			"  the target %s is still there — this is a clobber, not a relic replacement",
			target, got, live)
	case err != nil:
		t.Fatalf("FIXED — RenderAgentsSkills now refuses this shape (%v). "+
			"Delete this live-defect pin and move the row into "+
			"TestRenderAgentsSkillsStillRefusesEveryLiveForeignEntry.", err)
	default:
		t.Fatalf("neither the measured hole nor the fix: err=%v readlink=%q (%v)", err, got, readErr)
	}
	if _, err := os.Stat(live); err != nil {
		t.Errorf("the operator's target file itself was destroyed, not merely unlinked: %v", err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
