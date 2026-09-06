package posse

// Helpers lifted out of verifyafter_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// vaRepo points config `beads:` at a fresh repo holding a canned `bd list
// --all` answer, plus any extra config lines.
func vaRepo(t *testing.T, a *App, list string, cfg ...string) string {
	t.Helper()
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(list), 0o644)
	conf := "beads:\n  - " + repo + "\n" + strings.Join(cfg, "\n")
	os.WriteFile(a.ConfigPath, []byte(conf+"\n"), 0o644)
	return repo
}

// vaDependents sets what `bd dep list <id> --direction=up` answers.
func vaDependents(t *testing.T, repo, json string) {
	t.Helper()
	os.WriteFile(filepath.Join(repo, "fake-dependents.json"), []byte(json), 0o644)
}

func closedList(id, labels, closedAt string) string {
	return closedListReason(id, labels, closedAt, "Closed")
}

// closedListReason is closedList with the close_reason spelled out — the
// field the rejection exemption reads (ranger-base-skgs).
func closedListReason(id, labels, closedAt, reason string) string {
	return `[{"id":"` + id + `","title":"gate shell live","status":"closed","priority":1,` +
		`"assignee":"developer","labels":` + labels + `,"closed_at":"` + closedAt +
		`","close_reason":` + fmt.Sprintf("%q", reason) + `}]`
}

// vaRun runs one sweep and returns (filed, stdout, stderr).
func vaRun(t *testing.T, a *App, bd Bd) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	n := a.VerifyAfter(bd, a.BeadsDirs(), &out, &errb)
	return n, out.String(), errb.String()
}

func testBd(t *testing.T) Bd {
	t.Helper()
	return Bd{Bin: fakeBinFor(t, "bd")}
}

// vaClosed is one closed `-l code` bead for a canned `bd list --all` answer.
func vaClosed(id string, closedAt time.Time, prio int) string {
	return fmt.Sprintf(`{"id":%q,"title":"gate shell %s","status":"closed","priority":%d,`+
		`"assignee":"developer","labels":["code"],"closed_at":%q,"close_reason":"Closed"}`,
		id, id, prio, closedAt.Format(time.RFC3339Nano))
}

func vaList(beads ...string) string { return "[" + strings.Join(beads, ",") + "]" }

// vaGitRepo makes `dir` a real repo with one commit per message, so
// verifySection's commit trail is actually emitted. The j8qk table below
// passes t.TempDir(), which is NOT a repo: gitCommitsFor returns nil there
// and every byte the commits block writes is invisible to it. This is what
// makes that block adversarially reachable from a test.
func vaGitRepo(t *testing.T, dir string, msgs ...string) {
	t.Helper()
	qblGit(t, dir, "init", "-q", "-b", "main")
	qblGit(t, dir, "config", "user.email", "t@example.com")
	qblGit(t, dir, "config", "user.name", "t")
	for i, m := range msgs {
		f := filepath.Join(dir, fmt.Sprintf("f%d.txt", i))
		if err := os.WriteFile(f, []byte(m), 0o644); err != nil {
			t.Fatal(err)
		}
		mf := filepath.Join(dir, fmt.Sprintf("msg%d.txt", i))
		if err := os.WriteFile(mf, []byte(m+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		qblGit(t, dir, "add", "-A")
		qblGit(t, dir, "commit", "-q", "-F", mf)
	}
}

// forgedMarkerProbe is the substring that only appears if a payload made it
// into something the harness wrote.
const forgedMarkerProbe = "Verify the close of a-2 ("

// vaClosedClassed is vaClosed with the class fields spelled out: bd's
// `issue_type` and the label list, which are the two fields ADR 0006 §1's
// rule reads and the only ones BeadClass looks at.
func vaClosedClassed(id string, closedAt time.Time, prio int, issueType string, labels ...string) string {
	ls, _ := json.Marshal(labels)
	return fmt.Sprintf(`{"id":%q,"title":"gate shell %s","status":"closed","priority":%d,`+
		`"assignee":"developer","issue_type":%q,"labels":%s,"closed_at":%q,"close_reason":"Closed"}`,
		id, id, prio, issueType, ls, closedAt.Format(time.RFC3339Nano))
}
