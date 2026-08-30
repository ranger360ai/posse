package posse_test

// QA pins for ranger-base-c7ek — the ADR 0015 §3 fence must not wall the
// commit.
//
// WHAT WENT WRONG. The §3 amendment (2026-08-29, ranger-base-u9ud) added
// `Bash(bd hook:*)` and `Bash(bd hooks:*)` to every shipped PID. A PID's
// deny: renders an L1 PATH shim, and a PATH shim is matched by EVERY execve
// of `bd` — not only the ones a persona types. beads' own installed git hooks
// exec `bd hook pre-commit`, `bd hook post-checkout`, `bd hook post-merge`,
// `bd hooks run prepare-commit-msg` and `bd hooks run pre-push`. So the whole-
// verb deny refused beads' hooks, the hooks exited non-zero, and git aborted:
// eleven PIDs could not commit and could not check out AT ALL in any repo
// where bd installed hooks. Measured across three personas the day it shipped.
//
// WHY NOTHING CAUGHT IT. scripts/verify-pid-deny-set.sh reads PIDs and asks
// whether a rule is PRESENT. Every rendering test in internal/rhq invokes a
// shim by absolute path with the gates dir taken OFF the child's PATH. Neither
// shape can see this defect, because the defect is not in what the rules say
// or in what the shim does when you call it — it is in what git's own hooks
// hit when the gates dir is FIRST on PATH, which is the one arrangement a real
// persona session runs in and no test reproduced.
//
// So this file measures the thing the fence is for and the thing the fence
// must not cost, both by execution:
//
//   - TestShippedPIDsLetBeadsOwnHooksRun — for every shipped PID, render its
//     real deny set, put the gates dir first on PATH in a scratch repo whose
//     .git/hooks are beads' own shims, and require a path-limited commit and a
//     branch checkout to LAND. The stub bd logs its argv, and the arm asserts
//     the hooks actually reached bd: a green that came from hooks which never
//     ran would measure nothing.
//   - TestBroadHookDenyWallsTheCommitAndTheCheckout — the failing wrong arms.
//     Four deny sets that differ from the shipped one only in the hook rules,
//     each of which must FAIL. Includes the two shapes a presence-only audit
//     cannot see: the plural-only deny, which moves the wall one slot to
//     prepare-commit-msg rather than lifting it, and the keeps-broad-too set,
//     which carries the narrow rules AND the broad ones and so satisfies every
//     REQUIRED check while still walling the commit.
//   - TestNarrowedHookDenyStillRefusesInstall — the fence half. Narrowing is
//     only correct if the hazard it was reaching for is still refused, so
//     install/uninstall in both spellings must still be walled.
//
// NOT IN SCOPE, on purpose. .claude/settings.json and scripts/bd-argv-gate.py
// keep the WHOLE-VERB spelling and should: both gate a Bash tool call by its
// command TEXT, so they see what a persona types and never what git spawns.
// That layering is the reason narrowing L1 costs nothing a persona could
// otherwise reach — L2 walls every spelling of the typed verb, including ones
// an enumerated L1 list cannot name.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/rhq"
)

// The operative line of each hook beads installs (bd-shim v1, 0.49.1). The
// slot names and the two verb spellings are copied from the shims actually
// installed in this repo's .git/hooks — the singular `hook` for the slots bd
// implements directly, the plural `hooks run` for the delegating ones.
var bhcHooks = map[string]string{
	"pre-commit":         "exec bd hook pre-commit \"$@\"",
	"post-checkout":      "exec bd hook post-checkout \"$@\"",
	"post-merge":         "exec bd hook post-merge \"$@\"",
	"prepare-commit-msg": "exec bd hooks run prepare-commit-msg \"$@\"",
	"pre-push":           "exec bd hooks run pre-push \"$@\"",
}

// bhcShippedPIDs is every PID under examples/agents, by base name.
func bhcShippedPIDs(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join("examples", "agents"))
	if err != nil {
		t.Fatalf("read examples/agents: %v", err)
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	if len(out) == 0 {
		t.Fatal("no shipped PIDs found; this suite would measure nothing")
	}
	return out
}

// bhcDeny reads a shipped PID's deny: rules out of its frontmatter. Only the
// frontmatter: a PID's body discusses its own rules in prose, and prose is not
// a fence.
func bhcDeny(t *testing.T, pid string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("examples", "agents", pid+".md"))
	if err != nil {
		t.Fatalf("read PID %s: %v", pid, err)
	}
	var (
		rules []string
		fm    int
		block bool
	)
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "---" {
			fm++
			if fm == 2 {
				break
			}
			continue
		}
		if fm != 1 {
			continue
		}
		if strings.HasPrefix(ln, "deny:") {
			block = true
			continue
		}
		if !block {
			continue
		}
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(ln, "  - ") {
			rules = append(rules, strings.TrimSpace(strings.TrimPrefix(ln, "  - ")))
			continue
		}
		block = false
	}
	if len(rules) == 0 {
		t.Fatalf("PID %s carries no deny rules; the arm would measure nothing", pid)
	}
	return rules
}

// bhcLookOutside resolves a real binary with every posse gates bin dropped
// from PATH. The suite runs inside a persona pane, so a bare exec.Command
// would otherwise be answered by THIS session's own L1 shim instead of by the
// code under test (rangerhq-8sd).
func bhcLookOutside(t *testing.T, cmd string) string {
	t.Helper()
	old := os.Getenv("PATH")
	os.Setenv("PATH", rhq.PathOutsideGates(""))
	defer os.Setenv("PATH", old)
	p, err := exec.LookPath(cmd)
	if err != nil {
		t.Skipf("%s not on PATH outside the gates: %v", cmd, err)
	}
	return p
}

// bhcEnv is the child environment of a command run BEHIND the gates: binDir
// first, so a bare `bd` from inside a git hook resolves to the shim exactly
// as it does in a live persona session. That ordering is the whole point of
// the file — every existing renderer test drops binDir from the child PATH,
// which is why none of them could see this defect.
//
// The git config env vars pin the scratch repo away from the operator's own
// gitconfig, which may set core.hooksPath; without them a green arm could come
// from git ignoring the hooks dir this test just populated.
func bhcEnv(repo, binDir, stubDir, log string) []string {
	path := strings.Join([]string{binDir, stubDir, rhq.PathOutsideGates(binDir)}, string(os.PathListSeparator))
	return []string{
		"PATH=" + path,
		"HOME=" + repo,
		"BHC_LOG=" + log,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=qa", "GIT_AUTHOR_EMAIL=qa@example.com",
		"GIT_COMMITTER_NAME=qa", "GIT_COMMITTER_EMAIL=qa@example.com",
	}
}

// bhcWorld renders deny into a gates dir and builds a scratch repo carrying
// beads' hook shims and one commit. It returns the repo, the gates bin dir,
// the stub bd's dir and the stub's argv log.
func bhcWorld(t *testing.T, persona string, deny []string) (repo, binDir, stubDir, log string) {
	t.Helper()
	home := t.TempDir()
	stubDir = filepath.Join(home, "stub")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	log = filepath.Join(home, "bd-argv.log")

	// The stub stands in for the real bd so the arm never touches a real
	// beads db. It records what it was called with, which is how the green
	// arm proves the hooks actually ran.
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$BHC_LOG\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(stubDir, "bd"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	// RenderGates bakes the real binary the shim execs, resolving it on the
	// ambient PATH with gates dirs dropped. Put the stub there so the shim
	// execs the stub and not the operator's bd.
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+rhq.PathOutsideGates(""))

	app := &rhq.App{StateDir: filepath.Join(home, "state")}
	_, binDir, _, err := app.RenderGates(persona, deny)
	if err != nil {
		t.Fatalf("RenderGates: %v", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "bd")); err != nil {
		t.Fatalf("no bd shim was rendered from this deny set: %v", err)
	}

	gitBin := bhcLookOutside(t, "git")
	repo = filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(gitBin, args...)
		cmd.Dir = repo
		cmd.Env = bhcEnv(repo, binDir, stubDir, log)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	// Seed BEFORE the hooks are installed: the first commit is setup, not
	// the measurement, and it must not depend on the thing under test.
	run("init", "-q")
	run("symbolic-ref", "HEAD", "refs/heads/main")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "seed.txt")
	run("commit", "-q", "-m", "seed")

	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	for slot, body := range bhcHooks {
		script := "#!/bin/sh\n" + body + "\n"
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo, binDir, stubDir, log
}

// bhcGit runs one git command behind the gates and returns its combined
// output and error.
func bhcGit(t *testing.T, repo, binDir, stubDir, log string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bhcLookOutside(t, "git"), args...)
	cmd.Dir = repo
	cmd.Env = bhcEnv(repo, binDir, stubDir, log)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// bhcCommit writes a file and commits it path-limited — the shape a persona
// closing a bead uses (ADR 0022 single-writer-per-file).
func bhcCommit(t *testing.T, repo, binDir, stubDir, log, name string) (string, error) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := bhcGit(t, repo, binDir, stubDir, log, "add", name); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	return bhcGit(t, repo, binDir, stubDir, log, "commit", "-m", "work", "--", name)
}

// A shipped PID must not cost its persona the ability to commit. This is the
// regression: every PID under examples/agents, rendered for real, with beads'
// own hooks in the way.
func TestShippedPIDsLetBeadsOwnHooksRun(t *testing.T) {
	pids := bhcShippedPIDs(t)
	for _, pid := range pids {
		t.Run(pid, func(t *testing.T) {
			deny := bhcDeny(t, pid)
			repo, binDir, stubDir, log := bhcWorld(t, "qa-"+pid, deny)

			out, err := bhcCommit(t, repo, binDir, stubDir, log, "work.txt")
			if err != nil {
				t.Fatalf("a persona carrying the shipped %s PID cannot commit: %v\n%s", pid, err, out)
			}

			// A branch checkout fires post-checkout, the other slot the
			// broad deny disarmed — it took `git worktree add` and with it
			// scripts/release-artifacts.sh down (ranger-base-i312).
			if out, err := bhcGit(t, repo, binDir, stubDir, log, "checkout", "-q", "-b", "other"); err != nil {
				t.Fatalf("a persona carrying the shipped %s PID cannot check out: %v\n%s", pid, err, out)
			}

			// Liveness. Without this the arm is green whenever the hooks did
			// not run at all, which is the easiest way for this whole file to
			// measure nothing.
			b, err := os.ReadFile(log)
			if err != nil {
				t.Fatalf("the stub bd was never called, so no hook reached the shim: %v", err)
			}
			seen := string(b)
			for _, want := range []string{
				"hook pre-commit",
				"hooks run prepare-commit-msg",
				"hook post-checkout",
			} {
				if !strings.Contains(seen, want) {
					t.Errorf("no hook reached `bd %s`; the commit passed without the wall being tested.\nstub saw:\n%s", want, seen)
				}
			}
		})
	}
}

// The failing wrong arms. Each deny set below differs from a shipped PID's
// only in its hook rules, and each must break the commit — otherwise the green
// arm above is not measuring the rules it claims to.
func TestBroadHookDenyWallsTheCommitAndTheCheckout(t *testing.T) {
	narrow := []string{
		"Bash(bd hook install:*)", "Bash(bd hook uninstall:*)",
		"Bash(bd hooks install:*)", "Bash(bd hooks uninstall:*)",
	}
	cases := []struct {
		name string
		deny []string
		// want is a fragment of the refusal, naming which call was refused.
		want string
	}{
		{
			// The rule as shipped by the §3 amendment.
			name: "singular whole verb",
			deny: []string{"Bash(bd hook:*)"},
			want: "bd hook pre-commit",
		},
		{
			// The wall moves rather than lifts: pre-commit passes, and the
			// prepare-commit-msg slot — which git runs for EVERY commit,
			// --no-verify included — is refused one slot later.
			name: "plural whole verb only",
			deny: []string{"Bash(bd hooks:*)"},
			want: "bd hooks run prepare-commit-msg",
		},
		{
			name: "both whole verbs",
			deny: []string{"Bash(bd hook:*)", "Bash(bd hooks:*)"},
			want: "bd hook pre-commit",
		},
		{
			// The shape a presence-only audit cannot see: every narrowed rule
			// is present, so REQUIRED is satisfied, and the commit is walled
			// anyway. This is what scripts/verify-pid-deny-set.sh's FORBIDDEN
			// list exists for.
			name: "narrow rules kept but broad ones too",
			deny: append(append([]string{}, narrow...), "Bash(bd hook:*)", "Bash(bd hooks:*)"),
			want: "bd hook pre-commit",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, binDir, stubDir, log := bhcWorld(t, "qa-wrong", c.deny)
			out, err := bhcCommit(t, repo, binDir, stubDir, log, "work.txt")
			if err == nil {
				t.Fatalf("deny %v let the commit land; the green arm proves nothing\n%s", c.deny, out)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("want the refusal to name %q; got:\n%s", c.want, out)
			}
			if !strings.Contains(out, "refused by posse gate") {
				t.Errorf("the commit failed for some reason other than the gate:\n%s", out)
			}
		})
	}
}

// Narrowing is only correct if it keeps the hazard the broad rule was reaching
// for. bd's hook INSTALL and UNINSTALL rewrite the repo's hooks; those stay
// refused, in both spellings, through the same shim that now lets the run-time
// slots past.
func TestNarrowedHookDenyStillRefusesInstall(t *testing.T) {
	deny := bhcDeny(t, "devops")
	repo, binDir, stubDir, log := bhcWorld(t, "qa-fence", deny)
	_ = repo

	for _, argv := range [][]string{
		{"hook", "install"},
		{"hook", "uninstall"},
		{"hooks", "install"},
		{"hooks", "uninstall"},
		// A leading global option must not reorder past the rule — the
		// globalValueOpts["bd"] property ADR 0015 §3 renders for.
		{"--db", "/tmp/x.db", "hooks", "install", "--force"},
	} {
		cmd := exec.Command(filepath.Join(binDir, "bd"), argv...)
		cmd.Env = bhcEnv(repo, binDir, stubDir, log)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("`bd %s` was allowed through the narrowed fence:\n%s", strings.Join(argv, " "), out)
			continue
		}
		if !strings.Contains(string(out), "refused by posse gate") {
			t.Errorf("`bd %s` failed without a gate refusal:\n%s", strings.Join(argv, " "), out)
		}
	}

	// And the run-time slots the hooks need are NOT refused by the same shim.
	for _, argv := range [][]string{
		{"hook", "pre-commit"},
		{"hook", "post-checkout"},
		{"hook", "post-merge"},
		{"hooks", "run", "prepare-commit-msg"},
		{"hooks", "run", "pre-push"},
	} {
		cmd := exec.Command(filepath.Join(binDir, "bd"), argv...)
		cmd.Env = bhcEnv(repo, binDir, stubDir, log)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("`bd %s` is what beads' own hook runs and the fence refused it: %v\n%s",
				strings.Join(argv, " "), err, out)
		}
	}
}
