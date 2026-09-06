package posse

// Helpers lifted out of beadloss_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func qblLine(id, status string) string {
	return `{"id":"` + id + `","title":"verify: ` + id + `","status":"` + status +
		`","priority":2,"issue_type":"task","assignee":"qa"}`
}

func qblRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	qblGit(t, repo, "init", "-q")
	qblGit(t, repo, "config", "user.email", "t@example.com")
	qblGit(t, repo, "config", "user.name", "t")
	return repo
}

func qblGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}

// qblCommit writes the JSONL as exactly these lines and commits it.
func qblCommit(t *testing.T, repo, msg string, lines ...string) {
	t.Helper()
	qblWrite(t, repo, lines...)
	qblGit(t, repo, "add", "-A")
	qblGit(t, repo, "commit", "-q", "-m", msg)
}

func qblWrite(t *testing.T, repo string, lines ...string) {
	t.Helper()
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(repo, beadsDirName, beadsJSONL), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// qblLive is what the fake bd answers `list --all` with — the store of record.
func qblLive(t *testing.T, repo string, ids ...string) {
	t.Helper()
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, qblLine(id, "open"))
	}
	if err := os.WriteFile(filepath.Join(repo, "fake-list.json"),
		[]byte("["+strings.Join(parts, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func qblLost(t *testing.T, repo string) []LostBead {
	t.Helper()
	lost, err := LostBeads(testBd(t), repo)
	if err != nil {
		t.Fatalf("LostBeads: %v", err)
	}
	return lost
}

// qblRecord owns whatever the check currently finds.
func qblRecord(t *testing.T, repo string) {
	t.Helper()
	lost := qblLost(t, repo)
	if len(lost) == 0 {
		t.Fatal("setup: nothing to record")
	}
	if err := RecordDeletions(repo, "owned", "qa", lost, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func qblLedgerLines(t *testing.T, repo string) []string {
	t.Helper()
	b, err := os.ReadFile(beadsPath(repo, beadsDeleted))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func qblWriteLedger(t *testing.T, repo string, lines []string) {
	t.Helper()
	if err := os.WriteFile(beadsPath(repo, beadsDeleted), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func qblRedirect(t *testing.T, repo, target string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, beadsDirName, beadsRedirect), []byte(target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
