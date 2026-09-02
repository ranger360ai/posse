package posse

import "testing"

func TestYamlCleanQuotes(t *testing.T) {
	t.Parallel()
	// A wrapping pair of double quotes is syntax; a lone leading or
	// trailing quote is data (rangerhq-nvq).
	cases := map[string]string{
		`"quoted"`:                  `quoted`,
		`  "quoted"  # comment`:     `quoted`,
		`claude -p "$(cat {file})"`: `claude -p "$(cat {file})"`,
		`"unterminated`:             `"unterminated`,
		`trailing"`:                 `trailing"`,
		`"`:                         `"`,
		`""`:                        ``,
		`plain`:                     `plain`,
	}
	for in, want := range cases {
		if got := yamlClean(in); got != want {
			t.Errorf("yamlClean(%q) = %q, want %q", in, got, want)
		}
	}
	lines := []string{`command: claude -p "$(cat {file})"`, `desc: "wrapped"`, `tags: ["a", "b"]`}
	if got := yamlGetLines(lines, "command"); got != `claude -p "$(cat {file})"` {
		t.Errorf("command scalar: %q", got)
	}
	if got := yamlGetLines(lines, "desc"); got != "wrapped" {
		t.Errorf("wrapped scalar: %q", got)
	}
	if got := yamlListLines(lines, "tags"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("quoted inline list: %v", got)
	}
}
