package posse

import (
	"path/filepath"
	"strings"
	"testing"
)

// PROBE (QA, verifying ranger-base-qxvh). The credential scan has two
// halves: untracked files are read WHOLE off disk, tracked ones are read out
// of `git diff HEAD --unified=0`'s `+` lines. Git emits no `+` lines for a
// file it calls BINARY — the diff body is "Binary files a/x and b/x differ"
// — so a modification to a tracked binary file in the memory dir passes the
// scan with nothing scanned, and the commit is made.
//
// The scan already has the "I could not read it, so I hold" idiom (the
// memoryScanMax arm names the file and refuses). A binary tracked file
// reaches no such arm: it is silently treated as containing no added lines.
func TestQAProbeMemoryScanReadsATrackedBinaryFile(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")

	// A tracked file git calls binary, landed by hand first — so at kill
	// time it is a MODIFICATION of a tracked path and not an untracked file.
	blob := filepath.Join(repo, ConstitutionSourceDir, "personas", "dev", "capture.bin")
	write(t, blob, "harmless\x00bytes\n")
	mustGit(t, repo, "add", "--", ConstitutionSourceDir+"/personas/dev/capture.bin")
	mustGit(t, repo, "commit", "-q", "-m", "a binary artefact in the memory dir", "--", ConstitutionSourceDir+"/personas/dev/capture.bin")
	if strings.TrimSpace(mustGit(t, repo, "status", "--porcelain")) != "" {
		t.Fatal("the fixture must start clean")
	}
	// Now the persona's session drops a credential into it.
	const leaked = "sk-ant-api03-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	write(t, blob, "harmless\x00bytes\n"+leaked+"\n")
	// Witness that git really calls this binary — otherwise the probe is
	// measuring an ordinary text diff and proves nothing.
	if d := mustGit(t, repo, "diff", "HEAD", "--unified=0", "--", ConstitutionSourceDir+"/personas/dev/capture.bin"); !strings.Contains(d, "Binary files") {
		t.Fatalf("fixture is not binary to git, so this probe measures nothing:\n%s", d)
	}

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	line := ""
	if landing.Memory != nil {
		line = landing.Memory.Line()
	}
	body := mustGit(t, repo, "show", "HEAD:"+ConstitutionSourceDir+"/personas/dev/capture.bin")
	if strings.Contains(body, leaked) {
		t.Fatalf("the credential was committed unscanned; landing said %q", line)
	}
}
