package posse

// QA pins for ranger-base-5356 — the endpoint-pin vulnerability's public
// disclosure, and the machinery that carries it.
//
// The operator's ruling was a QUIET disclosure: a plain entry in the next
// release's notes, no CVE process. So "disclosed" means two things that fail
// independently, and both are pinned here:
//
//  1. CHANGELOG.md carries the entry — the flaw, the affected versions, the
//     fixed version, and what a deployer should do about it.
//  2. The release machinery puts that text at the TOP of the release's notes.
//     An entry nothing ships is not a disclosure; a pipeline with nothing to
//     ship is not one either.
//
// scripts/release-notes.sh is the reader in the middle, and it is deliberately
// lenient: it runs AFTER the tag is pushed, where a non-zero exit would burn a
// version number that cannot be reused. The strict arm lives in
// `make release-notes VERSION=vX.Y.Z`, before the tag. Both arms are pinned,
// because either one alone is the wrong half.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const releaseNotesScript = "scripts/release-notes.sh"

// runReleaseNotes runs the script the way the workflow and the Makefile do,
// and reports the three things a caller of it can branch on.
func runReleaseNotes(t *testing.T, args ...string) (exit int, stdout, stderr string) {
	t.Helper()
	cmd := exec.Command("./"+releaseNotesScript, args...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	switch e := err.(type) {
	case nil:
		exit = 0
	case *exec.ExitError:
		exit = e.ExitCode()
	default:
		t.Fatalf("%s %v: %v", releaseNotesScript, args, err)
	}
	return exit, out.String(), errb.String()
}

// fixtureChangelog writes a changelog whose version headings deliberately
// share prefixes: v0.4 is a prefix of v0.4.1 and of v0.4.10, so a reader that
// matches on prefix rather than on the whole version returns the wrong
// release's notes. Prefix-sharing is the mutation this fixture exists to kill.
func fixtureChangelog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	const body = `# CHANGELOG

preamble, above every heading

## Unreleased

unreleased body

## v0.4.10 — 2026-09-09

ten body
ten line two

## v0.4.1

one body

## v0.4

short body
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReleaseNotesEmitsTheVersionsOwnSectionAndStopsAtTheNext(t *testing.T) {
	ch := fixtureChangelog(t)
	for _, tc := range []struct{ version, want string }{
		{"v0.4.10", "ten body\nten line two"},
		{"v0.4.1", "one body"},
		{"v0.4", "short body"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			exit, out, errOut := runReleaseNotes(t, "--file", ch, "--version", tc.version, "--require")
			if exit != 0 {
				t.Fatalf("exit %d for %s\nstderr: %s", exit, tc.version, errOut)
			}
			if got := strings.TrimRight(out, "\n"); got != tc.want {
				t.Errorf("%s emitted %q, want %q — a reader that matched on prefix, or ran past the next heading, gets exactly this wrong",
					tc.version, got, tc.want)
			}
		})
	}
}

func TestReleaseNotesFallsBackToUnreleasedOnlyWhenNotRequired(t *testing.T) {
	ch := fixtureChangelog(t)

	// Lenient: the workflow's call. A version with no section of its own ships
	// the Unreleased text rather than nothing, and says so out loud.
	exit, out, errOut := runReleaseNotes(t, "--file", ch, "--version", "v9.9.9")
	if exit != 0 {
		t.Fatalf("lenient call exited %d — after the tag is pushed the number is spent, so this call may never fail\nstderr: %s", exit, errOut)
	}
	if got := strings.TrimRight(out, "\n"); got != "unreleased body" {
		t.Errorf("lenient fallback emitted %q, want the Unreleased body", got)
	}
	if !strings.Contains(errOut, "Unreleased") {
		t.Errorf("the fallback was silent about being a fallback; stderr was %q", errOut)
	}

	// Strict: the precondition's call. The same input must FAIL, or the
	// outstanding rename is never caught while the version is still free.
	exit, out, errOut = runReleaseNotes(t, "--file", ch, "--version", "v9.9.9", "--require")
	if exit == 0 {
		t.Errorf("--require accepted a version with no section of its own — the rename of '## Unreleased' would ship unnoticed")
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("--require refused and still wrote notes to stdout: %q", out)
	}
	if !strings.Contains(errOut, "Unreleased") {
		t.Errorf("the refusal does not name what to rename; stderr was %q", errOut)
	}
}

// The lenient arm's whole point: nothing in this script may be the reason a
// release dies. A changelog with neither the version nor an Unreleased section
// is the worst case, and it still exits 0 — while SAYING something, because an
// assertion of pure absence is satisfied by a script that measured nothing.
func TestReleaseNotesNeverFailsTheCallThatRunsAfterTheTag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(path, []byte("# CHANGELOG\n\nno sections at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exit, out, errOut := runReleaseNotes(t, "--file", path, "--version", "v1.2.3")
	if exit != 0 {
		t.Fatalf("exit %d on a changelog with no sections — that would burn a tag\nstderr: %s", exit, errOut)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("emitted notes out of a changelog with no sections: %q", out)
	}
	if !strings.Contains(errOut, "generated commit list") {
		t.Errorf("silently emitted nothing; the operator has to be told the notes will be the commit list alone. stderr: %q", errOut)
	}

	// Same rule one step earlier: a changelog that is not there at all.
	exit, _, errOut = runReleaseNotes(t, "--file", filepath.Join(t.TempDir(), "absent.md"), "--version", "v1.2.3")
	if exit == 0 {
		t.Errorf("a missing changelog exited 0 — that is a broken checkout, not a release with no notes")
	}
	if !strings.Contains(errOut, "no changelog") {
		t.Errorf("stderr does not say the file is missing: %q", errOut)
	}
}

// The version reaches an awk regex and a shell, so it gets the same allowlist
// the workflow's tag guard has. The control arm is the last case: a real
// version must still be accepted, or "refuses everything" passes by refusing
// everything.
func TestReleaseNotesRefusesAVersionThatIsNotOne(t *testing.T) {
	ch := fixtureChangelog(t)
	for _, bad := range []string{
		`v1.0$(id)`, "v1;id", "v1`id`", "v1 0.4", "0.4.0", "main", "v0.4.*", "../etc/passwd",
	} {
		t.Run(fmt.Sprintf("%q", bad), func(t *testing.T) {
			exit, out, _ := runReleaseNotes(t, "--file", ch, "--version", bad)
			if exit != 2 {
				t.Errorf("--version %q exited %d, want 2 (refused); stdout %q", bad, exit, out)
			}
		})
	}
	if exit, _, errOut := runReleaseNotes(t, "--file", ch, "--version", "v0.4.1"); exit != 0 {
		t.Fatalf("control: a real version was refused (exit %d) — the guard above proves nothing if nothing gets through\nstderr: %s", exit, errOut)
	}
}

// The release's notes have to actually OPEN with the section. This is the
// wiring, in the file CI runs.
func TestReleaseWorkflowOpensTheNotesWithTheChangelogSection(t *testing.T) {
	body, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	yml := string(body)
	for _, want := range []string{
		`scripts/release-notes.sh --version "$TAG"`,
		`--notes-file "$NOTES"`,
		"--generate-notes", // the commit list still follows the section
		"shell: bash",      // args=() / args+=() are bash, not sh
	} {
		if !strings.Contains(yml, want) {
			t.Errorf("release.yml no longer carries %q — without it the notes are the generated commit list alone and the disclosure never reaches the release page", want)
		}
	}
	// The workflow's call must stay LENIENT. --require there would turn a
	// forgotten rename into a dead release on a version number that cannot be
	// reused.
	if strings.Contains(yml, `release-notes.sh --version "$TAG" --require`) {
		t.Error("release.yml calls release-notes.sh with --require; that step runs after the tag is pushed, where a non-zero exit burns the version number")
	}
	// And the strict arm has to exist somewhere, or "lenient in CI" just means
	// nothing ever checks.
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mk), "--require") || !strings.Contains(string(mk), "release-notes:") {
		t.Error("the Makefile lost the strict `make release-notes` arm — then no one is told about a missing section while the version is still free")
	}
}

func TestReleaseRunbookCarriesTheChangelogPrecondition(t *testing.T) {
	body, err := os.ReadFile("docs/runbooks/release.md")
	if err != nil {
		t.Fatal(err)
	}
	rb := string(body)
	for _, want := range []string{
		"make release-notes VERSION=vX.Y.Z",
		"Read the notes body before you press Publish",
	} {
		if !strings.Contains(rb, want) {
			t.Errorf("docs/runbooks/release.md no longer carries %q", want)
		}
	}
	// Step 1's read of the draft body is the ONLY check on gh's prepending
	// behaviour, which no developer machine can run. The runbook has to admit
	// that rather than imply it is covered.
	if !strings.Contains(rb, "--notes-file` prepending to `--generate-notes") {
		t.Error("the runbook's \"still unproven\" list does not name the --notes-file/--generate-notes combination; it is GitHub state and is never exercised locally")
	}
}

// ---------------------------------------------------------------------------
// The disclosure itself.

// changelogHeadings returns CHANGELOG.md's `## ` headings, in file order.
func changelogHeadings(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "## ") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
		}
	}
	return out
}

var changelogVersionHeading = regexp.MustCompile(`^v[0-9][0-9.]*`)

// The endpoint-pin disclosure has to be inside a section the RELEASE READER
// emits — not merely somewhere in the file. Text above the first heading, or
// under a heading no release names, is text nobody downloading a release ever
// sees.
//
// Asking the script itself, per heading, is what makes this an agreement pin
// rather than a grep: it survives the `## Unreleased` -> `## vX.Y.Z` rename
// that cutting the release performs, and goes red if the entry is deleted,
// moved out of a section, or left somewhere the reader cannot reach.
func TestChangelogDisclosesTheEndpointPinVulnerabilityWhereAReleaseWillCarryIt(t *testing.T) {
	headings := changelogHeadings(t)
	if len(headings) < 2 {
		t.Fatalf("CHANGELOG.md has %d sections (%v) — the scan, not the changelog, is what is broken", len(headings), headings)
	}
	t.Logf("scanned %d CHANGELOG sections: %v", len(headings), headings)

	// What a deployer needs in order to act: the flaw's two names, the two
	// commits that fixed it, the versions that carry it, the mechanism, and
	// what to do. Losing any one of these leaves an entry that reads like a
	// changelog line instead of a disclosure.
	markers := []string{
		"RHQ_PLAN_USAGE_URL",
		"RHQ_MODEL_LIST_URL",
		"8a01e01",
		"0ba56cb",
		"Affected",
		"v0.3.0",
		"Authorization",
		"Upgrade guidance",
	}

	var carriers []string
	for _, h := range headings {
		var args []string
		switch {
		case changelogVersionHeading.MatchString(h):
			args = []string{"--file", "CHANGELOG.md", "--version", changelogVersionHeading.FindString(h)}
		default:
			// Reach the `## Unreleased` section the way the workflow does when
			// the rename is still outstanding: ask for a version that has no
			// section, and take the documented fallback.
			args = []string{"--file", "CHANGELOG.md", "--version", "v0.0.0"}
		}
		exit, out, errOut := runReleaseNotes(t, args...)
		if exit != 0 {
			t.Fatalf("reading section %q: exit %d\nstderr: %s", h, exit, errOut)
		}
		missing := []string{}
		for _, m := range markers {
			if !strings.Contains(out, m) {
				missing = append(missing, m)
			}
		}
		if len(missing) == 0 {
			carriers = append(carriers, h)
		}
	}
	if len(carriers) == 0 {
		t.Fatalf("no CHANGELOG section the release reader emits carries the endpoint-pin disclosure (looked for %v across %v).\n"+
			"ranger-base-5356: the operator ruled this is disclosed as a plain entry in the release's notes. An entry no release ships is not a disclosure.",
			markers, headings)
	}
	t.Logf("the endpoint-pin disclosure ships under: %v", carriers)
}
