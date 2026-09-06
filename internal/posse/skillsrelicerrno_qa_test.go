//go:build posse_arm2

package posse

// QA probe for ranger-base-epdyv finding 1 — the errno table danglingSkillLink
// actually implements, asked of the real function rather than read off its
// comment. The close widened "gone" from ENOENT to ENOENT-or-ENOTDIR and asks
// ENOTDIR a second time on the cleaned readlink value so a live target spelled
// with a trailing slash cannot be clobbered.
//
// The function's own comment used to make a claim this file was written to
// test: the second ask "can turn a replace into a refusal and never the
// other way." It was false, and row 12 is the counterexample — a live file
// reached through a SYMLINKED component and a "..", which a lexical
// filepath.Clean cannot follow. It shipped GREEN asserting the hole
// (ranger-base-han3i) until the second ask stopped rewriting anything but
// the trailing directory marker (ranger-base-jhyiv). The row stays, on the
// other side now: it is the one that goes red if that Clean ever comes back.
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
		// plant returns the target spelling for a symlink at link; the
		// symlink itself is created by the harness so every row is one shape.
		plant func(t *testing.T, root string) (target string)
	}{
		{"the target is gone (ENOENT)", true, func(t *testing.T, root string) string {
			return filepath.Join(root, "retired-home", "skills", "x")
		}},
		{"a component of the target is an ordinary file (ENOTDIR)", true, func(t *testing.T, root string) string {
			archived := filepath.Join(root, "rhq")
			mustWrite(t, archived, "tarball of the retired home")
			return filepath.Join(archived, "skills", "x")
		}},
		{"the target itself is a live file (nil)", false, func(t *testing.T, root string) string {
			live := filepath.Join(root, "live.md")
			mustWrite(t, live, "the operator's own file")
			return live
		}},
		{"a live FILE spelled with a trailing slash (ENOTDIR, target present)", false, func(t *testing.T, root string) string {
			live := filepath.Join(root, "live.md")
			mustWrite(t, live, "the operator's own file")
			return live + "/"
		}},
		{"a live DIR spelled with a trailing slash (nil)", false, func(t *testing.T, root string) string {
			live := filepath.Join(root, "livedir")
			if err := os.MkdirAll(live, 0o755); err != nil {
				t.Fatal(err)
			}
			return live + "/"
		}},
		{"a live file spelled with two trailing slashes", false, func(t *testing.T, root string) string {
			live := filepath.Join(root, "live.md")
			mustWrite(t, live, "the operator's own file")
			return live + "//"
		}},
		{"a live file spelled with a trailing /.", false, func(t *testing.T, root string) string {
			live := filepath.Join(root, "live.md")
			mustWrite(t, live, "the operator's own file")
			return live + "/."
		}},
		{"a RELATIVE target, live file, trailing slash", false, func(t *testing.T, root string) string {
			mustWrite(t, filepath.Join(root, "sibling.md"), "the operator's own file")
			return "sibling.md/"
		}},
		{"a relative target that is gone", true, func(t *testing.T, root string) string {
			_ = root
			return "no-such-sibling"
		}},
		{"the target is itself a dangling symlink (ENOENT through a chain)", true, func(t *testing.T, root string) string {
			mid := filepath.Join(root, "mid")
			if err := os.Symlink(filepath.Join(root, "nowhere"), mid); err != nil {
				t.Fatal(err)
			}
			return mid
		}},
		{"a symlink loop (ELOOP is not evidence the target is gone)", false, func(t *testing.T, root string) string {
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
		// The row that decides whether the second ask may rewrite the
		// target. The kernel resolves links/sym/../live.md to
		// store/live.md, a live file; the trailing slash makes the FIRST ask
		// ENOTDIR anyway, exactly as row 4. A lexical filepath.Clean
		// collapses that ".." to links/live.md — a path the kernel never
		// goes to and where nothing is — so the second ask read ENOENT for a
		// file sitting right there and the operator's own link was replaced
		// (measured under ranger-base-han3i; fixed under ranger-base-jhyiv,
		// which trims the trailing marker and nothing else). Any second ask
		// that goes back to rewriting the path fails here.
		{"a live file reached through a symlinked component and a .. (trailing slash)", false,
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
			if got := danglingSkillLink(link); got != c.want {
				t.Errorf("danglingSkillLink = %v, want %v\n  link   %s\n  target %s\n  stat(link) %v",
					got, c.want, link, target, statErr)
			}
		})
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
