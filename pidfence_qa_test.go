package posse_test

// QA pins for ranger-base-j2io item 3.
//
// The finding, MEASURED at the live retirement window 2026-08-28:
// `scripts/draft-pid-deny-promote.sh` recognised only the BLOCK deny: shape
//
//	deny:
//	  - Bash(git push:*)
//
// and every live crew PID is written in the INLINE flow shape
//
//	deny: [Bash(git push:*), Bash(security:*), Bash(git commit unless --)]
//
// so the script drafted 0 of 11 PIDs and the ADR 0015 §3 fence went in by
// hand with sed. It reported the skips honestly — it was not silent — but a
// drafting tool that drafts nothing is a runbook step that does not run.
//
// These pin the two halves that can regress back: the script drafts into
// BOTH shapes, and it still refuses to rewrite a shape it cannot parse.
//
// The reader is the discriminator, not the bytes: the drafted file is fed
// back through `posse.App.LoadAgent`, the same `yamlListLines` path posse's own
// PID reader uses, so a draft that produces plausible-looking YAML the
// reader does not accept fails here.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

const pidFenceRule = "Bash(posse promote:*)"

// pfScript is the script under test, as an absolute path.
func pfScript(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("scripts", "draft-pid-deny-promote.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("script missing: %v", err)
	}
	return abs
}

// pfSeed writes one PID into a fresh agents dir and returns (dir, path).
func pfSeed(t *testing.T, name, body string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name+".md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, p
}

// pfRun runs the script over an agents dir and returns its combined output.
// $RHQ_GATES_DIR/bin is stripped from the child's PATH so the script's own
// helpers resolve to the real tools. It does NOT protect the git fixture
// below — os/exec resolves the program name against the PARENT's PATH, so
// that one is kept green for every persona by typing git's safe, path-
// qualified commit form instead.
func pfRun(t *testing.T, dir string) (string, int) {
	t.Helper()
	cmd := exec.Command(pfScript(t), dir)
	cmd.Env = pfEnv()
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run: %v\n%s", err, out)
		}
		code = ee.ExitCode()
	}
	return string(out), code
}

func pfEnv() []string {
	gates := os.Getenv("RHQ_GATES_DIR")
	env := os.Environ()
	if gates == "" {
		return env
	}
	strip := filepath.Join(gates, "bin")
	for i, kv := range env {
		if !strings.HasPrefix(kv, "PATH=") {
			continue
		}
		var keep []string
		for _, seg := range strings.Split(kv[len("PATH="):], string(os.PathListSeparator)) {
			if seg != strip {
				keep = append(keep, seg)
			}
		}
		env[i] = "PATH=" + strings.Join(keep, string(os.PathListSeparator))
	}
	return env
}

// pfDeny reads the drafted PID back through posse's own PID reader.
func pfDeny(t *testing.T, dir, name string) []string {
	t.Helper()
	app := &posse.App{AgentsDir: dir}
	ag, err := app.LoadAgent(name)
	if err != nil {
		t.Fatalf("LoadAgent(%s): %v", name, err)
	}
	return ag.Deny
}

// TestDraftPidDenyPromoteDraftsIntoBothDenyShapes is the bead's finding.
// Every arm is checked separately: a green ban on one shape says nothing
// about the other, which is exactly how the block-only version shipped.
func TestDraftPidDenyPromoteDraftsIntoBothDenyShapes(t *testing.T) {
	cases := []struct {
		name string
		pid  string
		want []string // the full deny list the reader must see afterwards
	}{
		{
			name: "inline",
			pid:  "---\nname: inline\ndeny: [Bash(git push:*), Bash(security:*)]\nlabels: [devops]\n---\nprose\n",
			want: []string{"Bash(git push:*)", "Bash(security:*)", pidFenceRule},
		},
		{
			name: "inline_empty",
			pid:  "---\nname: inline_empty\ndeny: []\n---\nprose\n",
			want: []string{pidFenceRule},
		},
		{
			name: "inline_empty_spaced",
			pid:  "---\nname: inline_empty_spaced\ndeny: [ ]\n---\nprose\n",
			want: []string{pidFenceRule},
		},
		{
			name: "block",
			pid:  "---\nname: block\ndeny:\n  - Bash(git push:*)\n  - Bash(security:*)\nmetrics: [x]\n---\nprose\n",
			want: []string{"Bash(git push:*)", "Bash(security:*)", pidFenceRule},
		},
		{
			name: "block_trailing",
			pid:  "---\nname: block_trailing\ndeny:\n  - Bash(git push:*)\n---\n",
			want: []string{"Bash(git push:*)", pidFenceRule},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, _ := pfSeed(t, tc.name, tc.pid)
			out, code := pfRun(t, dir)
			if code != 0 {
				t.Fatalf("exit %d\n%s", code, out)
			}
			if !strings.Contains(out, "drafted into 1 PID(s)") {
				t.Fatalf("script did not report drafting this PID:\n%s", out)
			}
			got := pfDeny(t, dir, tc.name)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("deny list after draft:\n got %q\nwant %q\n%s", got, tc.want, out)
			}
		})
	}
}

// TestDraftPidDenyPromoteRefusesShapesItCannotRewrite is the other half of
// the contract, and the reason the block-only version was safe to ship: a
// shape the script cannot parse is REPORTED, and its bytes are not touched.
// A mangled PID is prose in force.
func TestDraftPidDenyPromoteRefusesShapesItCannotRewrite(t *testing.T) {
	cases := []struct {
		name string
		pid  string
	}{
		{"multiline_flow", "---\nname: multiline_flow\ndeny: [Bash(git push:*),\n  Bash(security:*)]\n---\nprose\n"},
		{"no_deny", "---\nname: no_deny\nallow: [Read]\n---\nprose\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, p := pfSeed(t, tc.name, tc.pid)
			out, code := pfRun(t, dir)
			if code != 0 {
				t.Fatalf("exit %d\n%s", code, out)
			}
			after, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != tc.pid {
				t.Fatalf("script rewrote a shape it cannot parse:\n%s", after)
			}
			if !strings.Contains(out, "SKIP "+tc.name+".md") {
				t.Fatalf("skip not reported by name:\n%s", out)
			}
			if !strings.Contains(out, "drafted into 0 PID(s), 1 left alone") {
				t.Fatalf("counts do not match the skip:\n%s", out)
			}
		})
	}
}

// TestDraftPidDenyPromoteIsIdempotent — the runbook step can be re-run, and
// a PID already carrying the fence is left alone rather than double-drafted.
func TestDraftPidDenyPromoteIsIdempotent(t *testing.T) {
	dir, p := pfSeed(t, "twice", "---\nname: twice\ndeny: [Bash(git push:*)]\n---\nprose\n")
	if _, code := pfRun(t, dir); code != 0 {
		t.Fatalf("first run exit %d", code)
	}
	first, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	out, code := pfRun(t, dir)
	if code != 0 {
		t.Fatalf("second run exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "drafted into 0 PID(s), 1 left alone") {
		t.Fatalf("second run did not leave the PID alone:\n%s", out)
	}
	second, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != string(first) {
		t.Fatalf("second run changed the file:\n%s", second)
	}
	if got := pfDeny(t, dir, "twice"); strings.Join(got, "|") != "Bash(git push:*)|"+pidFenceRule {
		t.Fatalf("deny list after two runs: %q", got)
	}
}

// TestDraftPidDenyPromoteStagesNothing — it is a DRAFT and it stays a draft.
// Pinned behaviourally, in a real repo: after the run the change is in the
// working tree only, the index is clean and HEAD has not moved.
func TestDraftPidDenyPromoteStagesNothing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: draft\ndeny: [Bash(git push:*)]\n---\nprose\n"
	if err := os.WriteFile(filepath.Join(dir, "draft.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(pfEnv(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-qm", "seed", "--", ".")
	head := strings.TrimSpace(git("rev-parse", "HEAD"))

	out, code := pfRun(t, dir)
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	// The witness that the fixture is real: the draft did land.
	if !strings.Contains(out, "drafted into 1 PID(s)") {
		t.Fatalf("nothing was drafted, so this test measured nothing:\n%s", out)
	}
	// TrimRight, not TrimSpace: the leading column is the INDEX status, and
	// trimming it turns " M" (unstaged) and "M " (staged) into the same string.
	if got := strings.TrimRight(git("status", "--porcelain"), "\n"); got != " M agents/draft.md" {
		t.Fatalf("working tree is not one unstaged edit: %q", got)
	}
	if got := strings.TrimSpace(git("diff", "--cached", "--name-only")); got != "" {
		t.Fatalf("script staged something: %q", got)
	}
	if now := strings.TrimSpace(git("rev-parse", "HEAD")); now != head {
		t.Fatalf("HEAD moved: %s -> %s", head, now)
	}
}
