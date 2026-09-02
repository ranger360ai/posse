package posse

// QA pins for rangerhq-74c6 — first live exercise of ADR 0007 binding.
// Codex silently drops a SKILL.md with no description: line (rangerhq-1qd);
// the shipped canon is the one that has to keep that line. Helpers qsk*
// so these survive whatever happens to skills_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAShippedDistributedSystemsSkillHasDescription(t *testing.T) {
	t.Parallel()
	p := filepath.Join("..", "..", "examples", "skills", "distributed-systems", "SKILL.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	desc := qskFrontmatterDescription(string(b))
	if desc == "" {
		t.Fatal("examples/skills/distributed-systems/SKILL.md has no description: line — codex will bind this skill to nothing")
	}
}

func TestQAFrontmatterDescriptionIgnoresBody(t *testing.T) {
	t.Parallel()
	cases := []struct {
		md, want string
	}{
		{"---\nname: x\ndescription: hello\n---\n", "hello"},
		{"---\nname: x\n---\nbody\n", ""},
		{"---\nname: x\n---\ndescription: not frontmatter\n", ""},
		{"no frontmatter\ndescription: nope\n", ""},
	}
	for _, c := range cases {
		if got := qskFrontmatterDescription(c.md); got != c.want {
			t.Errorf("got %q want %q for %q", got, c.want, c.md)
		}
	}
}

func qskFrontmatterDescription(md string) string {
	if !strings.HasPrefix(md, "---\n") {
		return ""
	}
	rest := strings.TrimPrefix(md, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		if strings.HasPrefix(line, "description:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return ""
}
