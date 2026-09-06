//go:build posse_arm2

package posse

// QA pin written verifying ranger-base-r5wpk's close under ranger-base-vd5nl.
// Its own file rather than memoryland_qa_test.go, which another session may
// still be holding under ADR 0022.

import (
	"path"
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
	// ConstitutionSourceDir and not the literal it spells today: the parked
	// form of this pin hard-coded the pre-rename `rhq/`, so it wrote its
	// fixture outside the memory dir the scan reads. The guard below still
	// passed — git sees a file the scan never looks at — and the refusal
	// came back EMPTY, a failure naming nothing (ranger-base-32009).
	dev := filepath.Join(repo, ConstitutionSourceDir, "personas", "dev")
	rel := path.Join(ConstitutionSourceDir, "personas", "dev", "ORDERS.md")
	// Seven committed lines, so the two additions below are far enough apart
	// to be separate hunks under --unified=0 and close enough to be merged
	// by an interHunkContext of 5.
	write(t, filepath.Join(dev, "ORDERS.md"), "# ORDERS\nl2\nl3\nl4\nl5\nl6\nl7\n")
	mustGit(t, repo, "add", "--", rel)
	mustGit(t, repo, "commit", "-q", "-m", "seed", "--", rel)
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
// LIVE since ranger-base-y7i7k: the header is unquoted, and `cur`/`n` are
// reset on `diff --git` so an unrecognized header can cost the attribution
// of its own file and never mis-name a previous one.
func TestTheDiffScanAttributesAHitInAQuotedPath(t *testing.T) {
	t.Parallel()

	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	// ConstitutionSourceDir and not the literal it spells today: the parked
	// form of this pin hard-coded the pre-rename `rhq/`, so it wrote its
	// fixture outside the memory dir the scan reads and measured nothing
	// (the guard below still passed, because git can see a file the scan
	// never looks at).
	dev := filepath.Join(repo, ConstitutionSourceDir, "personas", "dev")
	const odd = "no\u00ebl.md" // ordinary prose under a non-ASCII name
	rel := path.Join(ConstitutionSourceDir, "personas", "dev", odd)
	write(t, filepath.Join(dev, odd), "# b\n")
	mustGit(t, repo, "add", "--", rel)
	mustGit(t, repo, "commit", "-q", "-m", "a memory file with a non-ASCII name", "--", rel)
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

// The reset half of ranger-base-y7i7k, pinned where the fixture can reach
// it. The end-to-end pin above no longer can: with the header unquoted it
// takes the recognized path, so mutating the reset away leaves it green.
// That is the same trap the r5wpk close fell into — "a line no fixture can
// reach" was a statement about the fixtures then written, not about the
// line — so this one feeds firstCredShapeInDiff a header it cannot read by
// construction and asserts the only thing that matters: whatever it does
// with THIS file's attribution, it must never spend the PREVIOUS file's.
//
// MUTATION-CHECKED: dropping `cur, n = "", 0` from the `diff --git` case
// reds this with file="ORDERS.md", line=2 — an innocent file at a line
// number belonging to another one, which is the defect exactly.
func TestTheDiffScanNeverSpendsThePreviousFilesName(t *testing.T) {
	t.Parallel()

	// File one is clean and ends at its `@@`-set line 2. File two carries
	// the credential under a header spelling this reader does not know: an
	// unterminated quote, which no unquoting can rescue and which stands in
	// for whatever spelling comes next.
	diff := strings.Join([]string{
		`diff --git a/posse/personas/dev/ORDERS.md b/posse/personas/dev/ORDERS.md`,
		`--- a/posse/personas/dev/ORDERS.md`,
		`+++ b/posse/personas/dev/ORDERS.md`,
		`@@ -1,0 +2 @@`,
		`+- an ordinary lesson.`,
		`diff --git "a/posse/personas/dev/odd.md" "b/posse/personas/dev/odd.md`,
		`--- "a/posse/personas/dev/odd.md`,
		`+++ "b/posse/personas/dev/odd.md`,
		`@@ -1,0 +2 @@`,
		`+- the key that worked: sk-ant-api03-` + strings.Repeat("Q", 32),
		"",
	}, "\n")

	file, line, what := firstCredShapeInDiff(diff)
	if what == "" {
		t.Fatalf("the shape itself went unseen, so nothing below is measured: %q %d", file, line)
	}
	if strings.Contains(file, "ORDERS.md") {
		t.Errorf("the hold named the PREVIOUS file: %q:%d — an unreadable header may cost its own attribution, never another file's", file, line)
	}
	if file != "" {
		t.Errorf("an unreadable header must leave the file empty, got %q:%d", file, line)
	}
	// The LINE is still 2, and correctly: `@@` is readable whether or not
	// the header above it is, so the number belongs to the file that is
	// missing its name. Only `cur` can go stale across files, which is why
	// only `cur`'s reset is reachable by a fixture; `n` is reset beside it
	// for symmetry and its absence cannot be observed — a `+` line before
	// the first `@@` is not a diff git writes. Kept anyway: the line that
	// started this bead was dropped for being unpinnable while its absence
	// WAS observable, and those are not the same case.
	if line != 2 {
		t.Errorf("the line number comes from this file's own `@@` and must survive, got %d", line)
	}
}

// diffHeaderPath against the headers git actually writes, over every byte
// class its quoting has a rule for and both settings of core.quotePath —
// which is the axis, since quotePath=false leaves a path's non-ASCII bytes
// RAW inside the quotes a `"` or a `\` forced anyway.
//
// Two-way: every header git emits must round-trip to a path git also
// LISTS, and every listed path must be reached by some header. A reader
// that quietly dropped a class would otherwise pass the first half alone.
//
// The names are built from bytes, not from a literal, so what is measured
// is the byte class and not this file's own encoding. A name whose bytes
// are not valid UTF-8 is absent on purpose: APFS refuses to create one
// (measured, Operation not permitted), so it cannot be a fixture here.
func TestTheDiffHeaderReaderTakesEveryShapeGitWrites(t *testing.T) {
	t.Parallel()

	repo := wtRepo(t)
	names := []string{
		"plain.md",
		"noël.md",        // non-ASCII: quoted only when quotePath is on
		"q\"uote.md",     // quoted whatever quotePath says
		"back\\slash.md", //  "
		"ta\tb.md",       //  "
		"c\x01trl.md",    //  "
		"mé\"x\\.md",     // all three at once
	}
	for _, n := range names {
		write(t, filepath.Join(repo, n), "# a\n")
		mustGit(t, repo, "add", "--", n)
	}
	mustGit(t, repo, "commit", "-q", "-m", "one file per byte class", "--", ".")
	for _, n := range names {
		write(t, filepath.Join(repo, n), "# a\n- x\n")
	}

	for _, quotePath := range []string{"true", "false"} {
		t.Run("quotePath="+quotePath, func(t *testing.T) {
			raw, err := gitRaw(repo, append([]string{"-c", "core.quotePath=" + quotePath},
				memoryDiff("HEAD", "--unified=0", "--", ".")...)...)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			quoted := 0
			for _, ln := range strings.Split(string(raw), "\n") {
				rest, ok := strings.CutPrefix(ln, "+++ ")
				if !ok {
					continue
				}
				if strings.HasPrefix(rest, `"`) {
					quoted++
				}
				p, ok := diffHeaderPath(rest, "b/")
				if !ok {
					t.Errorf("header not read: %q", ln)
					continue
				}
				seen[p] = true
			}
			// Fixture guard: if this git quoted nothing, every header took
			// the plain prefix and the unquoting was never exercised.
			if quoted == 0 {
				t.Fatalf("this git quoted no header, so nothing here is pinned:\n%s", raw)
			}
			for _, n := range names {
				if !seen[n] {
					t.Errorf("no header resolved to %q — git listed it and the reader did not reach it", n)
				}
				delete(seen, n)
			}
			for p := range seen {
				t.Errorf("a header resolved to %q, which is not a file in the fixture", p)
			}
		})
	}
}

// /dev/null and a spelling nothing can read leave the caller with no path
// rather than a wrong one. `--- ` never sets it at all: it names the
// PRE-image, and every line number this scan reports is a post-image one.
func TestTheDiffHeaderReaderRefusesWhatIsNotAPath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		rest, want string
		ok         bool
	}{
		{"b/ORDERS.md", "ORDERS.md", true},
		{`"b/no\303\253l.md"`, "noël.md", true},
		{`"b/q\"uote.md"`, "q\"uote.md", true},
		{`"b/ta\tb.md"`, "ta\tb.md", true},
		{`"b/c\001trl.md"`, "c\x01trl.md", true},
		{`"b/back\\slash.md"`, "back\\slash.md", true},
		// quotePath=false on a filesystem that allows it: the `\` forces
		// the quotes and the non-ASCII byte is left RAW inside them, and
		// that byte is not valid UTF-8 on its own. It must come back as
		// the byte git wrote and not as a replacement rune. No file
		// fixture can reach this — APFS refuses to create the name — so
		// it is asserted on the reader directly.
		{"\"b/lat\xe9\\\\in.md\"", "lat\xe9\\in.md", true},
		{"/dev/null", "", false},          // a deletion
		{`"b/unterminated.md`, "", false}, // no closing quote
		{`"b/bad\9.md"`, "", false},       // not an escape git writes
		{`"c/ORDERS.md"`, "", false},      // quoted, but not this prefix
		{"c/ORDERS.md", "", false},        // some other prefix
		{"", "", false},
	} {
		got, ok := diffHeaderPath(tc.rest, "b/")
		if ok != tc.ok || got != tc.want {
			t.Errorf("diffHeaderPath(%q) = %q,%v want %q,%v", tc.rest, got, ok, tc.want, tc.ok)
		}
	}
}
