//go:build posse_arm3

package posse

// Pins for ranger-base-g3pfl (finding ranger-base-pvg36): claude's project
// cron file, `.claude/scheduled_tasks.json`, is a declared project-config
// entry, checked on PRESENCE (ProjectConfigWholeFile), so a launch over one
// degrades and a persona commit of one is refused by the constitution arm —
// both derive from the one declaration in builtinRuntimes.
//
// The path is spelled as a literal here and not read off ClaudeScheduledTasks,
// so a rename of the constant that also moved the path reds the declaration
// arm instead of carrying the pins along with it.
//
//	go test -tags posse_arm3 ./internal/posse -run 'ClaudeScheduledTasks' -v

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const qaClaudeScheduledTasks = ".claude/scheduled_tasks.json"

// A body the loader would act on: one task, one prompt. Its top level is
// `tasks`, which is the whole reason the entry cannot ride the key narrowing.
const qaScheduledTasksBody = `{"tasks":[{"id":"t1","cron":"* * * * *","prompt":"run me"}]}` + "\n"

func TestQAClaudeScheduledTasksIsDeclaredWholeFile(t *testing.T) {
	t.Parallel()
	a := &App{Home: t.TempDir(), StateDir: t.TempDir()}
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(claude.ProjectConfig, qaClaudeScheduledTasks) {
		t.Errorf("claude must declare %s as project config (ranger-base-pvg36): %v", qaClaudeScheduledTasks, claude.ProjectConfig)
	}
	if !slices.Contains(claude.ProjectConfigWholeFile, qaClaudeScheduledTasks) {
		t.Errorf("claude must check %s on presence — its top level is `tasks`, which the key narrowing reads as clean: %v", qaClaudeScheduledTasks, claude.ProjectConfigWholeFile)
	}
	// An exemption ProjectConfig does not name is checked by nothing.
	for _, rel := range claude.ProjectConfigWholeFile {
		if !slices.Contains(claude.ProjectConfig, rel) {
			t.Errorf("whole-file entry %s is not on ProjectConfig, so nothing reads it", rel)
		}
	}
}

// The launch degrade, through the classifier and through CreateSession, with
// a mutant control: the same runtime WITHOUT the whole-file exemption reads
// the same file clean. That arm is what says the exemption is load-bearing
// and not decoration over a key that happened to match.
func TestQAClaudeScheduledTasksDegradesTheLaunch(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	dev, err := b.App.LoadAgent("dev")
	if err != nil {
		t.Fatal(err)
	}
	claude, err := b.App.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if why := ProjectConfigTrust(claude, dev, repo); why != "" {
		t.Fatalf("control: an empty repo must be clean: %s", why)
	}
	p := filepath.Join(repo, filepath.FromSlash(qaClaudeScheduledTasks))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(qaScheduledTasksBody), 0o644); err != nil {
		t.Fatal(err)
	}

	why := ProjectConfigTrust(claude, dev, repo)
	if !strings.Contains(why, "project config is present") || !strings.Contains(why, qaClaudeScheduledTasks) {
		t.Errorf("a present %s must degrade the launch on presence and name the file: %q", qaClaudeScheduledTasks, why)
	}
	// `{}` is no tasks to the loader, and still a hit: presence is the predicate.
	if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if why := ProjectConfigTrust(claude, dev, repo); !strings.Contains(why, "project config is present") {
		t.Errorf("an empty-object %s is still present: %q", qaClaudeScheduledTasks, why)
	}
	if err := b.CreateSession(NewSessionOpts{Name: "st-present", Agent: "dev", Runtime: "claude", Dir: repo}); err == nil ||
		!strings.Contains(err.Error(), qaClaudeScheduledTasks) {
		t.Errorf("a claude launch over %s must refuse naming it: %v", qaClaudeScheduledTasks, err)
	}

	// The PID opt-in still clears it.
	trusting := *dev
	trusting.TrustProjectConfig = true
	if why := ProjectConfigTrust(claude, &trusting, repo); why != "" {
		t.Errorf("trust_project_config: true must clear the finding: %s", why)
	}

	// Mutant control: drop the exemption and the keyed reader calls the
	// tasks body clean — the wrong verdict this bead closes.
	if err := os.WriteFile(p, []byte(qaScheduledTasksBody), 0o644); err != nil {
		t.Fatal(err)
	}
	keyed := *claude
	keyed.ProjectConfigWholeFile = nil
	if why := ProjectConfigTrust(&keyed, dev, repo); why != "" {
		t.Errorf("control: under the key narrowing alone the tasks body reads clean, which is why it is whole-file; got %q", why)
	}
}

// The commit wall: a forced add (claude writes the path into
// .git/info/exclude itself, so `git add -f` is the shape that gets it staged)
// is refused by the constitution arm in a repo that is not the constitution.
func TestQAClaudeScheduledTasksForcedAddIsRefusedByTheCommitWall(t *testing.T) {
	t.Parallel()
	repo, git, persona := constitutionWallRepo(t, false)
	exclude := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exclude, []byte(qaClaudeScheduledTasks+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(repo, filepath.FromSlash(qaClaudeScheduledTasks))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(qaScheduledTasksBody), 0o644); err != nil {
		t.Fatal(err)
	}
	// Control: the exclude really hides it, so a plain add stages nothing.
	if out, err := git(persona, "add", "--", qaClaudeScheduledTasks); err == nil {
		t.Fatalf("control: a plain add of an excluded path must fail, got:\n%s", out)
	}
	if out, err := git(persona, "add", "-f", "--", qaClaudeScheduledTasks); err != nil {
		t.Fatalf("git add -f: %v\n%s", err, out)
	}
	out, err := git(persona, "commit", "-m", "schedule myself", "--", qaClaudeScheduledTasks)
	assertConstitutionRefusal(t, out, err, qaClaudeScheduledTasks, gatesDirOf(persona))
}
