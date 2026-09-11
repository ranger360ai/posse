//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-qnn6j deliverable 3, the docs pin: a launch line a cold
// installer types must parse as the launch they mean.
//
// WHAT HAPPENED, and what did not. The M2 cold install (2026-09-02) was
// reported as INSTALL.md telling the reader to bring a persona up with
// `posse new <name>` — which opens a plain herdr pane and starts no agent.
// The document did not say that: §10's smoke step has carried
// `--agent <persona>` all along, and the bare form came from a relay of the
// runbook rather than the runbook. What WAS true is the gap that made the
// relay plausible — §7 is where a reader meets the crew, and it ended
// without ever showing how one is started, so a reader who stopped there had
// seen every persona key and no launch line at all.
//
// So the pin is not "the bug does not come back" — there was no bug in the
// bytes. It is the standing property that made the report survivable: every
// `posse new` a reader can COPY is a launch whose shape is stated, and the
// section that introduces the crew states it. A document can go wrong here
// by deletion as easily as by edit, and neither reds anything else.
//
// Tree-wide by the letter of the class (it takes the repo root from
// qibRepoRoot and reads files outside this package), so it carries a
// Makefile door: $(QA_DOC_PINS), `make doc-check`.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ilfBlocks returns the fenced code blocks of a markdown file, each as its
// own string. The reading is per BLOCK rather than per line because the
// claim a `--cmd` example rests on ("the persona launch is the other one")
// is made in the block's own comments, where the line is read.
func ilfBlocks(src string) []string {
	var out []string
	var cur []string
	in := false
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if in {
				out = append(out, strings.Join(cur, "\n"))
				cur = nil
			}
			in = !in
			continue
		}
		if in {
			cur = append(cur, line)
		}
	}
	return out
}

// A copyable launch: a `posse new` with a session name after it, inside a
// fenced block. `posse new <name> [opts]` in a usage listing is matched too
// — that is also a line a reader retypes.
var ilfLaunch = regexp.MustCompile(`posse new [^\s|]`)

// ilfSection returns the body of one `## ` section of a markdown document.
func ilfSection(t *testing.T, src, heading string) string {
	t.Helper()
	i := strings.Index(src, heading)
	if i < 0 {
		t.Fatalf("the document has no %q heading — this pin is reading a file it does not recognize", heading)
	}
	rest := src[i+len(heading):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

func TestQAShippedLaunchLinesParseAsThePersonaLaunch(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)

	// ARM 1 — every copyable `posse new` in the two documents a cold
	// installer reads names its shape. `--agent <persona>` is the persona
	// launch; `--cmd` is the deliberate plain pane (README's quick start,
	// which runs before any crew exists to name). A bare one is the line the
	// M2 report described, and it is the one that must not exist.
	found := 0
	for _, name := range []string{"INSTALL.md", "README.md"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range ilfBlocks(string(b)) {
			for _, line := range strings.Split(block, "\n") {
				// A shell comment is prose that happens to sit in a block —
				// including the comment beside README's plain-pane line,
				// which names the persona launch in words. Nothing in it is
				// copyable, and reading one as a launch is how this pin
				// would red on the sentence it exists to require.
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				if !ilfLaunch.MatchString(line) || strings.Contains(line, "already exists") {
					continue
				}
				found++
				switch {
				case strings.Contains(line, "--agent"):
				case strings.Contains(line, "--cmd"):
					// A plain pane on purpose. It has to say so where it is
					// read, or it is the bare form with extra words.
					if !strings.Contains(block, "--agent") {
						t.Errorf("%s: a `--cmd` launch stands in a block that never names the persona launch, so a reader copying it has no way to learn the difference:\n%s", name, block)
					}
				default:
					t.Errorf("%s: a copyable launch names neither --agent nor --cmd, which is the line the M2 cold install was reported as carrying: %q", name, strings.TrimSpace(line))
				}
			}
		}
	}
	// A sweep that matches nothing passes; say what it read.
	if found < 2 {
		t.Fatalf("found %d copyable `posse new` lines across INSTALL.md and README.md — this pin is not reading what it thinks it is", found)
	}
	t.Logf("read %d copyable `posse new` lines", found)

	// ARM 2 — §7 is where the crew is introduced, and the gap the report
	// found: it must show the launch and name the difference, so a reader
	// who stops there has seen one.
	b, err := os.ReadFile(filepath.Join(root, "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	install := string(b)
	crew := ilfSection(t, install, "## 7. The crew")
	for _, want := range []struct{ needle, why string }{
		{"--agent", "the flag that makes a launch a persona launch"},
		{"plain herdr pane", "what `posse new` without it opens instead"},
		{"§10", "the first launch, with its Verify, which this section hands the reader on to"},
	} {
		if !strings.Contains(crew, want.needle) {
			t.Errorf("INSTALL.md §7 does not name %q (%s) — a reader who stops at the crew section has met no launch line", want.needle, want.why)
		}
	}

	// ARM 3 — and the other thing §7 owes a home nobody promotes: the one
	// refresh a single-tree home has (M2 deviation 3). The file name is
	// taken from the constant posse PRINTS, so the document and the shipped
	// sentence cannot come to name different files.
	if !strings.Contains(crew, PromoteManifestFile) {
		t.Errorf("INSTALL.md §7 never names %s — the only refresh a single-tree home has is removing it, and posse's own lines now say so while the runbook does not", PromoteManifestFile)
	}
	if !strings.Contains(crew, "constitution:") {
		t.Error("INSTALL.md §7 does not name the `constitution:` key, which is the fact that tells the two home shapes apart")
	}

	// And §10 is still the section §7 sends them to: the smoke step, with
	// the flag on it.
	first := ilfSection(t, install, "## 10. First launch, by hand")
	if !strings.Contains(first, "--agent") {
		t.Error("INSTALL.md §10's first launch no longer names --agent, and §7 now points at it")
	}
}
