package posse

// Helpers lifted out of dataceiling_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"strings"
	"testing"
)

const (
	qaCeilingClass = "restricted-banner"
	qaCeilingWord  = "QUOKKA"
	qaCeilingERE   = qaCeilingWord + "[[:space:]]+(RESTRICTED|INTERNAL)"
	qaCeilingHit   = qaCeilingWord + " RESTRICTED"
	qaExportClass  = "export-name"
	qaExportStem   = "quokka-export-"
	qaExportERE    = qaExportStem + "[0-9]+"
	qaCeilingCfg   = DataCeilingConfigKey + ":\n  " + qaCeilingClass + ": " + qaCeilingERE + "\n  " + qaExportClass + ": " + qaExportERE + "\n"
)

// qaCeilingWall is the visibility wall with the two ceiling patterns above
// configured, plus whatever extra top-level config the pin needs. FIXTURE
// PREMISE, asserted: both were ACCEPTED — a pin over a pattern the parser
// threw away is green against any wall at all.
func qaCeilingWall(t *testing.T, extra string) *visWall {
	t.Helper()
	w := newVisWallCfg(t, "instance", qaCeilingCfg+extra)
	set := (&App{ConfigPath: w.home + "/config.yaml"}).OpsPatternSet()
	if len(set.CeilingRejected) > 0 || len(set.Rejected) > 0 || len(set.Ceiling) != 2 ||
		set.Ceiling[0].Class != qaCeilingClass || set.Ceiling[1].Class != qaExportClass {
		t.Fatalf("fixture premise: the ceiling patterns must be accepted, got ceiling=%+v rejected=%v/%v", set.Ceiling, set.CeilingRejected, set.Rejected)
	}
	return w
}

// qaNoCeilingVocabulary is qaNoVocabulary for the ceiling's fixture words:
// neither ERE anywhere, and the vocabulary only on a line carrying one of
// the allowed subjects (a staged path, for the path arm).
func qaNoCeilingVocabulary(t *testing.T, what, text string, allowedOn ...string) {
	t.Helper()
	for _, ere := range []string{qaCeilingERE, qaExportERE} {
		if strings.Contains(text, ere) {
			t.Errorf("%s carried a ceiling pattern's ERE — the value is what may not exist here:\n%s", what, text)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, qaCeilingWord) && !strings.Contains(line, qaExportStem) {
			continue
		}
		ok := false
		for _, allow := range allowedOn {
			if allow != "" && strings.Contains(line, allow) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s printed ceiling vocabulary on a line that is not an allowed subject:\n\t%q\nfull text:\n%s", what, line, text)
		}
	}
}

// unstage drops rel from the index: a REFUSED commit leaves its own subject
// staged (ranger-base-uzgkz), and the next arm's premise must not see it.
func (w *visWall) unstage(t *testing.T, repo, rel string) {
	t.Helper()
	if out, err := w.git(repo, nil, "rm", "-q", "--cached", "--", rel); err != nil {
		t.Fatalf("git rm --cached %s: %v %s", rel, err, out)
	}
}

// qaMsgCommit stages a clean file and commits it with msg through form —
// "-m" or "-F -", the crew's own (AGENTS.md) — path-limited, because the
// shared-index arm in the same hook refuses an unqualified commit and a pin
// that tripped THAT wall would be green over no wall at all. Shared with
// check 3's message pins (checkthreemessage_qa_test.go, ranger-base-qk8i9):
// the two walls read the message through one renderer, so their pins type
// one the same way.
func qaMsgCommit(t *testing.T, w *visWall, repo, rel, body, form, msg string, env []string) (string, error) {
	t.Helper()
	w.stage(t, repo, rel, body)
	if form == "-F -" {
		return w.gitIn(repo, env, msg, "commit", "-F", "-", "--", rel)
	}
	return w.git(repo, env, "commit", "-m", msg, "--", rel)
}
