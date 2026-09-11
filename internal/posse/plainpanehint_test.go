//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-qnn6j: the reading behind the `posse new` hint, on its own.
// The end-to-end half — that main.go prints it, to stderr, BEFORE the create
// — is cmd/posse/plainpanehint_qa_test.go.
//
// It resolves through CanonAgent rather than by joining a path, which is the
// same rule every other name-from-outside follows (rangerhq-c6u6): one
// persona, one identity, and the spelling posse offers back is the directory
// entry's — so an operator who typed a case-folded name gets a line that
// launches rather than a line that only works on their filesystem.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlainPaneHintNamesAPersonaOnlyWhenTheLaunchDidNot(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name    string
		session string
		agent   string
		want    bool
	}{
		{"a session named after a persona", "developer", "", true},
		{"the same, case-folded the way APFS allows", "Developer", "", true},
		{"the launch already named the persona", "developer", "developer", false},
		{"an ordinary session name", "scratch", "", false},
		{"a path-shaped spelling is not a persona name", "./developer", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := a.PlainPaneHint(c.session, c.agent)
			if (got != "") != c.want {
				t.Fatalf("PlainPaneHint(%q, %q) = %q, want hinted=%v", c.session, c.agent, got, c.want)
			}
			if !c.want {
				return
			}
			// The line has to be RETYPEABLE: the persona spelled as the
			// agents dir spells it, whatever the operator typed.
			if !strings.Contains(got, "--agent developer") {
				t.Errorf("the hint does not offer the canonical launch line: %q", got)
			}
			if !strings.Contains(got, "plain pane") {
				t.Errorf("the hint does not say what was actually created: %q", got)
			}
		})
	}
}
