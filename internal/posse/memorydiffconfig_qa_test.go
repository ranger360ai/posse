package posse

// QA pin written verifying ranger-base-r5wpk's close under ranger-base-vd5nl.
// Its own file rather than memoryland_qa_test.go, which another session may
// still be holding under ADR 0022.

import (
	"path/filepath"
	"strings"
	"testing"
)

// ─── ranger-base-p70ug ───────────────────────────────────────────────────────

// ranger-base-r5wpk stated the diff FORMAT on the argv (memoryDiff), so the
// credential scan reads git's diff and not the box's configuration. Every
// route that bead named is closed. diff.interHunkContext is the same class
// and is not pinned: it merges hunks and RE-INTRODUCES context lines under
// --unified=0, and firstCredShapeInDiff counts the new line number by
// incrementing once per `+` line only — a context line matches no case in
// that switch, so the count is short by however many sit between the hunk
// header and the hit.
//
// FAIL-CLOSED, not fail-open: the commit is still held and the shape is
// still named. This is attribution, the same class as diff.noprefix and
// diff.mnemonicPrefix — which the bead listed and the fix DID pin, because
// "the refusal is the whole product here". An operator sent to the wrong
// line opens prose, finds no credential, and has been handed a reason to
// disbelieve the hold.
//
// The fixture guard runs first: if this git does not honour the setting the
// test fatals rather than passing while measuring nothing.
//
// Fix is one flag on the same list: --inter-hunk-context=0, accepted by
// git 2.50.1. Un-skip when ranger-base-p70ug lands.
func TestTheDiffScanCountsLinesWhateverTheConfigurationSays(t *testing.T) {
	t.Skip("ranger-base-p70ug: diff.interHunkContext moves the line number the refusal names")

	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	dev := filepath.Join(repo, "rhq", "personas", "dev")
	// Seven committed lines, so the two additions below are far enough apart
	// to be separate hunks under --unified=0 and close enough to be merged
	// by an interHunkContext of 5.
	write(t, filepath.Join(dev, "ORDERS.md"), "# ORDERS\nl2\nl3\nl4\nl5\nl6\nl7\n")
	mustGit(t, repo, "add", "--", "rhq/personas/dev/ORDERS.md")
	mustGit(t, repo, "commit", "-q", "-m", "seed", "--", "rhq/personas/dev/ORDERS.md")
	mustGit(t, repo, "config", "diff.interHunkContext", "5")
	before := mustGit(t, repo, "rev-parse", "HEAD")

	const leaked = "sk-ant-api03-IIIIIIIIIIIIIIIIIIIIIIIIIIIIIIII"
	write(t, filepath.Join(dev, "ORDERS.md"),
		"# ORDERS\nADDED-ONE\nl2\nl3\nl4\nl5\nl6\n- the key that worked: "+leaked+"\nl7\n")

	// Fixture guard: the PINNED argv must still render context lines here,
	// else the setting is not honoured and nothing below is pinned.
	raw, err := gitRaw(dev, memoryDiff("HEAD", "--unified=0", "--", ".")...)
	if err != nil {
		t.Fatal(err)
	}
	d := string(raw)
	if !strings.Contains(d, "\n l2\n") {
		t.Fatalf("this git does not honour diff.interHunkContext under the pinned argv, so nothing here is pinned:\n%s", d)
	}

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("a credential shape was committed as %s:\n%s", after, headFiles(t, repo))
	}
	line := landing.Memory.Line()
	// The credential is on line 8 of the new file: seed, ADDED-ONE, l2-l6, key.
	if !strings.Contains(line, "ORDERS.md:8") {
		t.Errorf("the refusal must name the line the credential is ON: %q", line)
	}
	if strings.Contains(line, leaked) {
		t.Errorf("the refusal echoed the credential: %q", line)
	}
}
