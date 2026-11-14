//go:build posse_arm2

package posse

// THE TYPED TEST IS NARROWER THAN THE FACT IT STANDS FOR, so the message arm
// STRIPS comment lines that git KEEPS (ranger-base-4wmiz, from the verify of
// ranger-base-mfryj).
//
// messageArm decides whether to read the commit message file WHOLE or through
// `stripspace --strip-comments`, and under default cleanup it keys that on
//
//	posse_typed = [ "$2" = "message" ] && [ "${GIT_EDITOR-}" = : ]
//
// The FACT it needs is "git will keep '#' lines in the object", and git's own
// rule for that is `use_editor ? CLEANUP_ALL : CLEANUP_SPACE` — cleanup is
// "whitespace", which keeps '#' lines, exactly when no editor opens. git tells
// the hook which it is by exporting GIT_EDITOR=: when it opens no editor
// (commit.c run_commit_hook), so the fact is `GIT_EDITOR = :` ALONE. "$2" is
// not part of it, and "$2" is "commit" for `--amend`/`-c`/`-C` and "merge" for
// a conflicted merge's commit — three no-editor forms where git keeps the
// lines and the arm strips them before any check scans them.
//
// This is the LEAK direction, so it is the opposite of the bead that re-keyed
// the predicate: ranger-base-mfryj was filed over the arm refusing text git
// throws away, and it fixed that. The defect here PREDATES it — the old
// `"$2" = "message"` row stripped these same forms — but mfryj's own measured
// table has `--amend --no-edit` in it as a row that "saw ':' and kept a '#'
// line", so the evidence was on the table when the predicate was written.
// gates.go also still says "the one way the two disagree is a caller who
// exports GIT_EDITOR=: themselves and passes -e", and these three forms are a
// second way, in the direction that matters more.
//
// A LIVE-DEFECT PIN: it ships GREEN, asserting the hole, so the escape is
// visible from a suite run instead of only from a bead. Every row's want is
// `landed`, which is the WRONG behaviour.
//
// IF YOU ARE HERE BECAUSE THIS WENT RED, the fix has landed: drop the
// `[ "${2:-}" = "message" ] &&` conjunct from messageArm's $posse_typed, flip
// every `landed: true` below to false, and rewrite this comment to say what is
// now true. MEASURED 2026-11-14 (ranger-base-4wmiz) with that one conjunct
// dropped: all three rows below refuse, and every behavioural pin over this
// arm in arms 1, 2 and 3 stays green — the one other red is
// TestQAMessageArmReadsTwoWaysKeyedOnTheCleanupMode, which names the literal
// and has to be retyped with it.
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

func TestQAMessageArmsTypedTestIsNarrowerThanTheFactItStandsFor(t *testing.T) {
	// FIXTURE PREMISE for every row: default cleanup. With commit.cleanup set
	// to one of the three named modes the arm reads whole on every path and
	// none of this arises, so a row that silently ran under a configured mode
	// would be green over nothing.
	for _, row := range []struct {
		name string
		// arg2 is what git must hand the hook, and the whole point of the
		// row: GIT_EDITOR is ":" in all three, so the pair is what separates
		// them from the forms the arm does read whole.
		arg2   string
		landed bool
	}{
		{"amend --no-edit", "commit", true},
		{"-C <sha>", "commit", true},
		{"merge --no-edit", "merge", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			w := qaCeilingWall(t, "")
			if out, err := w.git(w.priv, nil, "config", "--get", "commit.cleanup"); err == nil {
				t.Fatalf("fixture premise: commit.cleanup must be unset in the fixture repo, got %q", out)
			}
			rel := "assets/" + qaExportStem + "42.csv"
			flagged := "# cite " + qaExportStem + "42\n"
			log := ""

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
				env := append(append([]string(nil), w.persona...), "QA_SOURCE_LOG="+log)
				if row.name == "amend --no-edit" {
					w.stage(t, w.priv, "internal/posse/a.go", "package posse // touched\n")
					if out, err := w.git(w.priv, env, "commit", "-q", "--amend", "--no-edit", "--", "internal/posse/a.go"); err != nil {
						qaTypedFactRefused(t, row.landed, out)
						return
					}
				} else {
					sha, err := w.git(w.priv, nil, "rev-parse", "HEAD")
					if err != nil {
						t.Fatalf("git rev-parse: %v %s", err, sha)
					}
					w.stage(t, w.priv, "internal/posse/c.go", "package posse\n")
					if out, err := w.git(w.priv, env, "commit", "-q", "-C", strings.TrimSpace(sha), "--", "internal/posse/c.go"); err != nil {
						qaTypedFactRefused(t, row.landed, out)
						return
					}
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
				env := append(append([]string(nil), w.persona...), "QA_SOURCE_LOG="+log)
				// No pathspec: git refuses a partial commit during a merge,
				// which is why this form carries no `--`.
				if out, err := w.git(w.priv, env, "commit", "-q", "--no-edit"); err != nil {
					qaTypedFactRefused(t, row.landed, out)
					return
				}
			}

			// PREMISE, asserted rather than assumed: the pair that makes this
			// row a row. GIT_EDITOR=":" is git saying it opened no editor, so
			// git's cleanup is "whitespace" and the '#' lines are kept; "$2"
			// is not "message", so $posse_typed is empty and the arm strips.
			saw, _ := os.ReadFile(log)
			want := row.arg2 + "|:"
			if !strings.Contains(string(saw), want) {
				t.Fatalf("fixture premise: git must hand the hook %q for this form — got:\n%s", want, saw)
			}

			landed, err := w.git(w.priv, nil, "log", "-1", "--format=%B")
			if err != nil {
				t.Fatalf("git log: %v %s", err, landed)
			}
			if got := strings.Contains(landed, qaExportStem); got != row.landed {
				if row.landed {
					t.Errorf("the class did NOT reach the commit object — the hole is closed and this pin is now backwards:\n%s", landed)
					return
				}
				t.Errorf("THE HOLE: a ceiling class on a '#' line reached the commit object through `%s`.\n"+
					"git kept the line and messageArm stripped it before scanning, because \"$2\" is %q\n"+
					"and $posse_typed demands \"message\". committed message:\n%s\nrefusals.log:\n%s",
					row.name, row.arg2, landed, w.log(t))
			}
		})
	}
}

// qaTypedFactRefused is the one arm that means the fix landed: the commit was
// refused, so the class never reached an object. It is an error only while this
// pin still asserts the hole.
func qaTypedFactRefused(t *testing.T, expectedToLand bool, out string) {
	t.Helper()
	if expectedToLand {
		t.Errorf("the commit was REFUSED, so the hole is closed and this pin is now backwards — "+
			"flip this row's `landed` to false and retype the head comment:\n%s", out)
	}
}
