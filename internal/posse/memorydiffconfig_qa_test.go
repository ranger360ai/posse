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
	t.Parallel()
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

// ─── ranger-base-y7i7k ───────────────────────────────────────────────────────

// The sibling of the pin above, and the one that needs no configuration at
// all: git C-quotes a path carrying a non-ASCII byte, a quote or a
// backslash, so the header is spelled `+++ "b/…"` and misses the literal
// `+++ b/` firstCredShapeInDiff matches on. It falls into the OTHER header
// case, `cur` is never updated — and because the fix deliberately does not
// reset `cur` on `diff --git`, it still holds the PREVIOUS file's name. The
// hold then points at a file that does not contain the credential.
//
// core.quotePath defaults to TRUE, and setting it false still quotes a name
// containing a quote or a backslash, so nothing has to be misconfigured for
// this. Fail-CLOSED — the commit is held — but the refusal is the product,
// and one naming an innocent file is one an operator will override.
//
// This is also the fixture the close said could not exist: "every file whose
// diff carries a `+` line also carries the `+++ b/` and `@@` that set them,
// so a reset would be a line no fixture can reach" (memoryland.go:521-524).
// A quoted path is a file whose diff carries a `+` line and no matching
// `+++ b/`. Restoring that reset is half the fix, and this pin reaches it.
//
// Un-skip when ranger-base-y7i7k lands.
func TestTheDiffScanAttributesAHitInAQuotedPath(t *testing.T) {
	t.Parallel()
	t.Skip("ranger-base-y7i7k: a C-quoted header is missed and the hold names the previous file")

	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	dev := filepath.Join(repo, "rhq", "personas", "dev")
	const odd = "no\u00ebl.md" // ordinary prose under a non-ASCII name
	write(t, filepath.Join(dev, odd), "# b\n")
	mustGit(t, repo, "add", "--", "rhq/personas/dev/"+odd)
	mustGit(t, repo, "commit", "-q", "-m", "a memory file with a non-ASCII name", "--", "rhq/personas/dev/"+odd)
	before := mustGit(t, repo, "rev-parse", "HEAD")

	// ORDERS.md sorts first and stays clean; the credential is in the other
	// file, so a parser that never updated `cur` names ORDERS.md.
	appendOrders(t, repo, "dev", "- an ordinary lesson.\n")
	const leaked = "sk-ant-api03-QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ"
	write(t, filepath.Join(dev, odd), "# b\n- the key that worked: "+leaked+"\n")

	// Fixture guard: this git must actually C-quote the header, else the
	// assertions below pass for the wrong reason.
	raw, err := gitRaw(dev, memoryDiff("HEAD", "--unified=0", "--", ".")...)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "+++ \"b/") {
		t.Fatalf("this git does not C-quote the header here, so nothing is pinned:\n%s", raw)
	}

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("a credential shape was committed as %s:\n%s", after, headFiles(t, repo))
	}
	line := landing.Memory.Line()
	if strings.Contains(line, "ORDERS.md") {
		t.Errorf("the hold named an innocent file — the credential is in %s: %q", odd, line)
	}
	if !strings.Contains(line, "l.md:2") {
		t.Errorf("the hold must name the file the credential is IN, at its new line: %q", line)
	}
	if strings.Contains(line, leaked) {
		t.Errorf("the refusal echoed the credential: %q", line)
	}
}
