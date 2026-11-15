//go:build posse_arm2

package posse

// THE TYPED TEST IS THE FACT IT STANDS FOR, so the message arm reads WHOLE
// every message whose comment lines git KEEPS (ranger-base-r5j00, found by the
// verify of ranger-base-mfryj under ranger-base-4wmiz).
//
// messageArm decides whether to read the commit message file WHOLE or through
// `stripspace --strip-comments`, and under default cleanup it keys that on
//
//	posse_typed = [ "${GIT_EDITOR-}" = : ]
//
// The FACT it needs is "git will keep '#' lines in the object", and git's own
// rule for that is `use_editor ? CLEANUP_ALL : CLEANUP_SPACE` — cleanup is
// "whitespace", which keeps '#' lines, exactly when no editor opens. git tells
// the hook which it is by exporting GIT_EDITOR=: when it opens no editor
// (commit.c run_commit_hook), so the fact is `GIT_EDITOR = :` ALONE.
//
// Until ranger-base-r5j00 the test also demanded "$2" = "message", and "$2"
// is "commit" for `--amend`/`-c`/`-C` and "merge" for a conflicted merge's
// commit — three no-editor forms where git kept the lines and the arm
// stripped them before any check scanned them. That was the LEAK direction:
// a merge's own `# Conflicts:` block quoting a ceiling-class PATH landed in a
// commit object with refusals.log empty. The defect predated
// ranger-base-mfryj (the older `"$2" = "message"` row stripped the same
// forms); mfryj re-derived the predicate with `--amend --no-edit` already on
// its measured table.
//
// This pin shipped GREEN asserting the hole (ranger-base-4wmiz) and was
// flipped by the fix: every row's want is now REFUSED. It goes red if the
// conjunct comes back — MEASURED 2026-11-14 under a go test -overlay of
// gates.go restoring it: all three rows land the class.
//
// WHY THE EXISTING PINS DO NOT SEE THIS. Each of the two closest ones drives
// the editor arm of its own shape, where git strips and the arm is right to
// ignore the block: TestQACeilingMessageArmDoesNotRefuseOnAMergesConflictList
// resolves a conflicted merge and commits it with GIT_EDITOR=true, and
// TestQACeilingMessageArmDoesNotRefuseOnARebaseContinueTemplate does the same
// for `rebase --continue`. The `--no-edit` twin of the first is row 3 here.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaTypedFactHookRecorder chains a one-line recorder in front of the wall's own
// prepare-commit-msg, so each row asserts the ("$2", GIT_EDITOR) pair it claims
// rather than asserting it from git's documentation. The wall still judges.
func qaTypedFactHookRecorder(t *testing.T, repo string) string {
	t.Helper()
	log := filepath.Join(t.TempDir(), "source")
	hook := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	orig, err := os.ReadFile(hook)
	if err != nil {
		t.Fatalf("fixture premise: the wall installs a prepare-commit-msg hook: %v", err)
	}
	body := strings.Replace(string(orig), "\n",
		"\n[ -n \"${QA_SOURCE_LOG:-}\" ] && printf '%s|%s\\n' \"${2:-}\" \"${GIT_EDITOR-UNSET}\" >> \"$QA_SOURCE_LOG\"\n", 1)
	if err := WriteExecutable(hook, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return log
}

// qaTypedFactLabel is the commit-message arm's own refusals.log label. The log
// never carries the class itself (qaNoCeilingVocabulary), so the label is how a
// row tells this arm's refusal from any other.
const qaTypedFactLabel = "(stamp: " + VisibilityPrivate + ", commit message)"

func TestQAMessageArmReadsWholeEveryNoEditorCommitForm(t *testing.T) {
	// FIXTURE PREMISE for every row: default cleanup. With commit.cleanup set
	// to one of the three named modes the arm reads whole on every path and
	// none of this arises, so a row that silently ran under a configured mode
	// would be green over nothing.
	for _, row := range []struct {
		name string
		// arg2 is what git must hand the hook, and the whole point of the
		// row: GIT_EDITOR is ":" in all three while "$2" is NOT "message",
		// so the pair is what separates them from -m/-F.
		arg2 string
	}{
		{"amend --no-edit", "commit"},
		{"-C <sha>", "commit"},
		{"merge --no-edit", "merge"},
	} {
		t.Run(row.name, func(t *testing.T) {
			w := qaCeilingWall(t, "")
			if out, err := w.git(w.priv, nil, "config", "--get", "commit.cleanup"); err == nil {
				t.Fatalf("fixture premise: commit.cleanup must be unset in the fixture repo, got %q", out)
			}
			rel := "assets/" + qaExportStem + "42.csv"
			flagged := "# cite " + qaExportStem + "42\n"
			log := ""
			// The commit under test: its error is the verdict, read after the
			// recorder premise below so a refusal for any OTHER reason cannot
			// pass for this one.
			var out string
			var cerr error
			before := 0

			switch row.name {
			case "amend --no-edit", "-C <sha>":
				// HISTORY the wall never judged: a message carrying the class
				// on a '#' line, committed with the hook bypassed outright.
				// That is any commit predating the wall's install in the repo,
				// or arriving by a route that runs no prepare-commit-msg.
				w.stage(t, w.priv, "internal/posse/a.go", "package posse\n")
				if out, err := w.gitIn(w.priv, nil, flagged, "-c", "core.hooksPath=/dev/null",
					"commit", "-qF", "-", "--", "internal/posse/a.go"); err != nil {
					t.Fatalf("fixture premise: planting the unjudged history: %v %s", err, out)
				}
				head, err := w.git(w.priv, nil, "log", "-1", "--format=%B")
				if err != nil || !strings.Contains(head, qaExportStem) {
					t.Fatalf("fixture premise: HEAD's message must carry the class on a '#' line, got %q (%v)", head, err)
				}
				// CONTROL, the same bytes through the same wall by the typed
				// route: refused. So the configured pattern does match this
				// text, and what differs below is only which branch of
				// messageArm reads it.
				w.stage(t, w.priv, "internal/posse/b.go", "package posse\n")
				if out, err := w.gitIn(w.priv, w.persona, flagged, "commit", "-F", "-", "--", "internal/posse/b.go"); err == nil {
					t.Fatalf("CONTROL premise: the same '#' line typed with -F must be refused:\n%s", out)
				}
				w.unstage(t, w.priv, "internal/posse/b.go")

				log = qaTypedFactHookRecorder(t, w.priv)
				before = strings.Count(w.log(t), qaTypedFactLabel)
				env := append(append([]string(nil), w.persona...), "QA_SOURCE_LOG="+log)
				if row.name == "amend --no-edit" {
					w.stage(t, w.priv, "internal/posse/a.go", "package posse // touched\n")
					out, cerr = w.git(w.priv, env, "commit", "-q", "--amend", "--no-edit", "--", "internal/posse/a.go")
				} else {
					sha, err := w.git(w.priv, nil, "rev-parse", "HEAD")
					if err != nil {
						t.Fatalf("git rev-parse: %v %s", err, sha)
					}
					w.stage(t, w.priv, "internal/posse/c.go", "package posse\n")
					out, cerr = w.git(w.priv, env, "commit", "-q", "-C", strings.TrimSpace(sha), "--", "internal/posse/c.go")
				}

			case "merge --no-edit":
				// Here the '#' lines are GIT'S OWN: a conflicted merge's
				// `# Conflicts:` block quotes the conflicted PATH, and a path
				// is the scope ranger-base-mfryj's description says this check
				// states and does not enforce in the comment block.
				w.plant(t, w.priv, rel, "base\n")
				if out, err := w.git(w.priv, nil, "checkout", "-q", "-b", "side"); err != nil {
					t.Fatalf("git checkout -b side: %v %s", err, out)
				}
				w.plant(t, w.priv, rel, "side\n")
				if out, err := w.git(w.priv, nil, "checkout", "-q", "main"); err != nil {
					t.Fatalf("git checkout main: %v %s", err, out)
				}
				w.plant(t, w.priv, rel, "main\n")
				if out, err := w.git(w.priv, w.persona, "merge", "side"); err == nil {
					t.Fatalf("fixture premise: the merge must CONFLICT on %s:\n%s", rel, out)
				}
				write(t, filepath.Join(w.priv, filepath.FromSlash(rel)), "resolved\n")
				if out, err := w.git(w.priv, nil, "add", "--", rel); err != nil {
					t.Fatalf("git add %s: %v %s", rel, err, out)
				}
				log = qaTypedFactHookRecorder(t, w.priv)
				before = strings.Count(w.log(t), qaTypedFactLabel)
				env := append(append([]string(nil), w.persona...), "QA_SOURCE_LOG="+log)
				// No pathspec: git refuses a partial commit during a merge,
				// which is why this form carries no `--`.
				out, cerr = w.git(w.priv, env, "commit", "-q", "--no-edit")
			}

			// PREMISE, asserted rather than assumed: the pair that makes this
			// row a row. GIT_EDITOR=":" is git saying it opened no editor, so
			// git's cleanup is "whitespace" and the '#' lines are kept; "$2"
			// is not "message", which is what the old conjunct keyed on.
			saw, _ := os.ReadFile(log)
			want := row.arg2 + "|:"
			if !strings.Contains(string(saw), want) {
				t.Fatalf("fixture premise: git must hand the hook %q for this form — got:\n%s", want, saw)
			}

			if cerr == nil {
				landed, _ := w.git(w.priv, nil, "log", "-1", "--format=%B")
				t.Errorf("THE HOLE (ranger-base-r5j00): a ceiling class on a '#' line reached the commit object through `%s`.\n"+
					"git kept the line, so messageArm must read it whole on GIT_EDITOR=: alone; \"$2\" is %q here.\n"+
					"committed message:\n%s\nrefusals.log:\n%s",
					row.name, row.arg2, landed, w.log(t))
				return
			}
			// The refusal is THIS arm's: one more commit-message line in
			// refusals.log than before the commit under test (the -F control
			// above already wrote one, so a bare Contains would read it).
			if rl := w.log(t); strings.Count(rl, qaTypedFactLabel) != before+1 {
				t.Errorf("the commit was refused, but not by the commit-message arm — refused for some other reason?\n"+
					"git said:\n%s\nrefusals.log:\n%s", out, rl)
			}
		})
	}
}
