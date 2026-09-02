package posse

// QA pins for rangerhq-lti6 (from a security review of rangerhq-812n).
//
// Claim: INSTALL.md §4's seed commit tracked everything but `state/`, and §6
// then designated `envs/` the secret surface — so the runbook instructed a
// future instance to git-track its own secrets. The operator's live instance
// repo has always ignored `envs/`; the runbook now says so too.
//
// ranger-base-13h3 extends the same pin to `secrets/` (ADR 0019 D1's
// harness-credential store: 0700/0600, seeded empty by `posse init`). That
// store is empty on every box today, so the leak is prospective — the pin
// plants the first resident credential a next instance would put there and
// requires §4's own recipe to keep it out of the seed commit.
//
// The pin that matters is not that the words appear. It is that the recipe
// §4 hands the reader, run verbatim against the layout §4 tells the reader to
// create, actually keeps `envs/` out of git. That is an agreement pin: the
// ignore paths are checked against the directory §4's own `export RHQ_HOME`
// names, so the stale `rhq/` prefix this bead also found — inert for anyone
// following the runbook, since §4 creates `<repo>/posse` — fails it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installSection4 returns §4, or fails the test if the pin has stopped reading
// its subject. The section contains a heredoc with `#` lines, so the bounds
// are the numbered headings.
func installSection4(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatalf("read INSTALL.md: %v", err)
	}
	doc := string(b)
	i := strings.Index(doc, "## 4. Create the instance")
	j := strings.Index(doc, "## 5. ")
	if i < 0 || j < 0 || j < i {
		t.Fatal("INSTALL.md: §4 not found — the pin has stopped reading its subject")
	}
	return doc[i:j]
}

// section4HomeLeaf is the last path element of the RHQ_HOME §4 tells the
// reader to export — the directory name every ignore path in the same section
// has to be written against.
func section4HomeLeaf(t *testing.T) string {
	t.Helper()
	for _, line := range strings.Split(installSection4(t), "\n") {
		line = strings.TrimPrefix(strings.TrimSpace(line), "$ ")
		if !strings.HasPrefix(line, "export RHQ_HOME=") {
			continue
		}
		home := strings.TrimPrefix(line, "export RHQ_HOME=")
		leaf := home[strings.LastIndexByte(home, '/')+1:]
		if leaf == "" || strings.ContainsAny(leaf, "<>$ ") {
			t.Fatalf("INSTALL.md §4: RHQ_HOME %q has no literal directory name to check ignores against", home)
		}
		return leaf
	}
	t.Fatal("INSTALL.md §4: no `export RHQ_HOME=` — the pin has stopped reading its subject")
	return ""
}

// section4GitignoreRecipe is the seed commit's gitignore append, `$ ` prompts
// stripped, exactly as a reader would type it.
func section4GitignoreRecipe(t *testing.T) string {
	t.Helper()
	lines := strings.Split(installSection4(t), "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "$ cat >> .gitignore <<") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("INSTALL.md §4: the .gitignore append is gone — the pin has stopped reading its subject")
	}
	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "EOF" {
			end = i
			break
		}
	}
	if end < 0 {
		t.Fatal("INSTALL.md §4: the .gitignore heredoc has no terminator")
	}
	var b strings.Builder
	for _, l := range lines[start : end+1] {
		b.WriteString(strings.TrimPrefix(l, "$ "))
		b.WriteByte('\n')
	}
	return b.String()
}

// untrackedAfterRecipe lays out the instance §4 describes, runs the recipe
// verbatim in the repo root, and returns what git would still offer to commit.
func untrackedAfterRecipe(t *testing.T, recipe, leaf string) []string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	repo := t.TempDir()
	for _, f := range []string{
		leaf + "/config.yaml",
		leaf + "/agents/developer.md",
		leaf + "/envs/default.env",
		leaf + "/secrets/harness.env",
		leaf + "/state/dispatch-watch.log",
	} {
		p := filepath.Join(repo, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("fixture %s: %v", f, err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o600); err != nil {
			t.Fatalf("fixture %s: %v", f, err)
		}
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	cmd := exec.Command("sh", "-c", recipe)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("§4 gitignore recipe failed: %v\n%s", err, out)
	}
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain", "--untracked-files=all").CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	var paths []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			paths = append(paths, strings.TrimSpace(strings.TrimPrefix(l, "??")))
		}
	}
	return paths
}

// TestInstallSection4SeedCommitLeavesSecretStoresOutOfGit is the pin that runs
// rather than reads.
func TestInstallSection4SeedCommitLeavesSecretStoresOutOfGit(t *testing.T) {
	leaf := section4HomeLeaf(t)
	paths := untrackedAfterRecipe(t, section4GitignoreRecipe(t), leaf)

	// Positive witness: an empty measurement must not read as a pass.
	if len(paths) == 0 {
		t.Fatal("nothing was left to commit at all — the fixture never landed, so this measures nothing")
	}
	var sawConstitution bool
	for _, p := range paths {
		switch {
		case strings.HasPrefix(p, leaf+"/envs/"):
			t.Errorf("INSTALL.md §4's seed commit still tracks the secret surface: %s (rangerhq-lti6)", p)
		case strings.HasPrefix(p, leaf+"/secrets/"):
			t.Errorf("INSTALL.md §4's seed commit still tracks the harness-credential store: %s (ranger-base-13h3)", p)
		case strings.HasPrefix(p, leaf+"/state/"):
			t.Errorf("INSTALL.md §4's seed commit still tracks machine-local state: %s", p)
		case p == leaf+"/config.yaml", p == leaf+"/agents/developer.md":
			sawConstitution = true
		}
	}
	if !sawConstitution {
		t.Errorf("the recipe ignored the constitution too — §4 commits it: got %v", paths)
	}
}

// TestInstallSection4GitignorePinDiscriminates: a pin over a recipe is only as
// strong as the recipes it rejects. Every arm here is a shape that has shipped
// or nearly did.
func TestInstallSection4GitignorePinDiscriminates(t *testing.T) {
	leaf := section4HomeLeaf(t)
	cases := []struct {
		name   string
		recipe string
		leaked string // the path that must survive the ignore
	}{
		{
			name:   "the shape that shipped: only state/ ignored (rangerhq-812n)",
			recipe: "printf '%s\\n' '" + leaf + "/state/' >> .gitignore\n",
			leaked: leaf + "/envs/default.env",
		},
		{
			name:   "the shape that shipped: envs/ and state/ ignored, secrets/ not (ranger-base-13h3)",
			recipe: "printf '%s\\n' '" + leaf + "/envs/' '" + leaf + "/state/' >> .gitignore\n",
			leaked: leaf + "/secrets/harness.env",
		},
		{
			name:   "the stale pre-rename prefix ignores a directory §4 never creates",
			recipe: "printf '%s\\n' 'rhq/envs/' 'rhq/secrets/' 'rhq/state/' >> .gitignore\n",
			leaked: leaf + "/secrets/harness.env",
		},
		{
			name:   "an ignore anchored at the repo root misses the home one level down",
			recipe: "printf '%s\\n' '/envs/' '/secrets/' '/state/' >> .gitignore\n",
			leaked: leaf + "/secrets/harness.env",
		},
		{
			name:   "no .gitignore at all",
			recipe: ": >> .gitignore\n",
			leaked: leaf + "/envs/default.env",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var leaked bool
			for _, p := range untrackedAfterRecipe(t, tt.recipe, leaf) {
				if p == tt.leaked {
					leaked = true
				}
			}
			if !leaked {
				t.Fatalf("a recipe that does not ignore the secret surface passed the pin: %q not offered for commit", tt.leaked)
			}
		})
	}
}

// TestInstallStatesTheNeverCommitRuleWhereTheSecretsAre: §4 is where the reader
// types the ignore, §6 is where they are told secrets live. The rule has to
// hold in both places or the second one re-opens the hole.
func TestInstallStatesTheNeverCommitRuleWhereTheSecretsAre(t *testing.T) {
	sec4 := installSection4(t)
	for _, want := range []string{
		"commit everything except\n`state/`, `envs/` and `secrets/`", // the prose the code block realizes
		"never commit it", // the file-tree marking
	} {
		if !strings.Contains(sec4, want) {
			t.Errorf("INSTALL.md §4 no longer states the rule: missing %q (rangerhq-lti6)", want)
		}
	}
	// The tree marks state/ this way; envs/ has to carry the same mark.
	for _, dir := range []string{"envs/", "secrets/", "state/"} {
		var marked bool
		for _, l := range strings.Split(sec4, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), dir) && strings.Contains(l, "never commit it") {
				marked = true
			}
		}
		if !marked {
			t.Errorf("INSTALL.md §4's file tree does not mark %s `never commit it` (rangerhq-lti6)", dir)
		}
	}

	b, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatalf("read INSTALL.md: %v", err)
	}
	doc := string(b)
	i := strings.Index(doc, "## 6. Env sets")
	j := strings.Index(doc, "## 7. ")
	if i < 0 || j < 0 || j < i {
		t.Fatal("INSTALL.md: §6 not found — the pin has stopped reading its subject")
	}
	sec6 := doc[i:j]
	for _, want := range []string{
		"Neither `envs/` nor `secrets/` is ever committed", // ranger-base-13h3: both stores, not envs/ alone
		"do not survive a commit",                          // why the gitignore, not the modes, is the boundary
		"gitignore",
	} {
		if !strings.Contains(sec6, want) {
			t.Errorf("INSTALL.md §6 no longer carries the never-commit rule: missing %q (rangerhq-lti6, ranger-base-13h3)", want)
		}
	}
}
