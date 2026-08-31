package posse

// QA pins for ranger-base-m6cm — the ADR 0019 "path 3" detective control.
//
// The defect this came from: ranger-base-zzc closed on a DELETE. The operator
// removed ~/.claude/.credentials.json at 2026-08-26 03:40, coordinator verified it
// clean at 03:05, and a new 994-byte file was created at 11:47:07 the same day
// — 8h06m later — with nothing on the box watching for two days. A one-shot
// remediation of a self-regenerating condition is not a control, so the check
// is now scripts/verify-credential-paths.sh and these are its arms.
//
// Two of them are the whole point:
//   - the RENAMED form. On 2026-08-23 the file was renamed, not removed
//     (.credentials.json.stale-20260823, same bytes, same mode). The instance
//     ADR 0019 D5 used to word the check as the exact path, which passes on
//     that box; it was reconciled to the glob on 2026-08-29 (rangerhq-m10j)
//     and this arm is why. The matcher is a glob or it is theatre.
//   - the EMPTY arm. With no config directory present the script has measured
//     nothing, and "no findings" would be a pass earned by looking at nothing
//     (the negative-control trap in NOTES). It must exit 2, not 0.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const cpScript = "scripts/verify-credential-paths.sh"

// cpRun executes the real script against a scratch HOME. CLAUDE_CONFIG_DIR is
// cleared unless a case sets it, so no case can read the operator's live box.
func cpRun(t *testing.T, home string, extraEnv ...string) (string, int) {
	t.Helper()
	abs, err := filepath.Abs(cpScript)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(abs)
	cmd.Env = append([]string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}, extraEnv...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v\n%s", cpScript, err, out)
	}
	return string(out), code
}

// cpHome builds a scratch HOME with a .claude directory holding the named
// files. Contents are a marker string the script must never echo.
const cpMarker = "QA-CREDENTIAL-BODY-MUST-NOT-BE-PRINTED"

func cpHome(t *testing.T, names ...string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(cpMarker), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestQACredentialPathsCleanDirectoryPasses(t *testing.T) {
	out, code := cpRun(t, cpHome(t))
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	// Positive witness: a pass must say what it looked at, so that "clean"
	// cannot be earned by scanning nothing.
	if !strings.Contains(out, "0 findings in 1 scanned") {
		t.Errorf("clean run must report the count it scanned:\n%s", out)
	}
}

func TestQACredentialPathsFindsTheExactName(t *testing.T) {
	out, code := cpRun(t, cpHome(t, ".credentials.json"))
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "FINDING") || !strings.Contains(out, ".credentials.json") {
		t.Errorf("must name the file it found:\n%s", out)
	}
}

// The arm the exact-path wording did not have. A rename changes the name, not
// the exposure.
func TestQACredentialPathsFindsTheRenamedStaleForm(t *testing.T) {
	out, code := cpRun(t, cpHome(t, ".credentials.json.stale-20260823"))
	if code != 1 {
		t.Fatalf("exit %d, want 1 — the glob is the whole point\n%s", code, out)
	}
	if !strings.Contains(out, ".credentials.json.stale-20260823") {
		t.Errorf("must name the renamed file:\n%s", out)
	}
}

// Metadata only. A control that prints the credential it found is worse than
// no control.
func TestQACredentialPathsNeverPrintsContent(t *testing.T) {
	out, code := cpRun(t, cpHome(t, ".credentials.json"))
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if strings.Contains(out, cpMarker) {
		t.Fatalf("script printed file CONTENT:\n%s", out)
	}
	for _, want := range []string{"mode ", "size ", "mtime "} {
		if !strings.Contains(out, want) {
			t.Errorf("finding lost its %q metadata column:\n%s", want, out)
		}
	}
}

// It must not find things that are not path 3, or every rotation learns to
// ignore it.
func TestQACredentialPathsIgnoresUnrelatedFiles(t *testing.T) {
	out, code := cpRun(t, cpHome(t, "settings.json", "credentials.json", ".credentials-backup", "history.jsonl"))
	if code != 0 {
		t.Fatalf("exit %d, want 0 — matcher is too wide\n%s", code, out)
	}
}

// The empty arm: nothing present means nothing measured, and that is not a
// pass. Without this, deleting ~/.claude would read as a green check.
func TestQACredentialPathsRefusesToPassWhenNothingWasScanned(t *testing.T) {
	out, code := cpRun(t, t.TempDir())
	if code != 2 {
		t.Fatalf("exit %d, want 2\n%s", code, out)
	}
	if strings.Contains(out, "clean") {
		t.Errorf("an unmeasured run must not say clean:\n%s", out)
	}
	if !strings.Contains(out, "nothing measured") {
		t.Errorf("must say it measured nothing:\n%s", out)
	}
}

// Setting the variable must not be a way to make a finding disappear.
func TestQACredentialPathsEnvOverrideCannotHideTheHomeFinding(t *testing.T) {
	home := cpHome(t, ".credentials.json")
	alt := t.TempDir()
	out, code := cpRun(t, home, "CLAUDE_CONFIG_DIR="+alt)
	if code != 1 {
		t.Fatalf("exit %d, want 1 — CLAUDE_CONFIG_DIR hid the ~/.claude file\n%s", code, out)
	}
	if !strings.Contains(out, "2 scanned") {
		t.Errorf("both directories must be scanned:\n%s", out)
	}
}

// [ -d "$d" ] follows a symlink; find's starting point must too, or a
// symlinked config dir scans as empty (ranger-base-dpuf).
func TestQACredentialPathsFollowsASymlinkedConfigDir(t *testing.T) {
	real := t.TempDir()
	if err := os.WriteFile(filepath.Join(real, ".credentials.json"), []byte(cpMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.Symlink(real, filepath.Join(home, ".claude")); err != nil {
		t.Fatal(err)
	}
	out, code := cpRun(t, home)
	if code != 1 {
		t.Fatalf("exit %d, want 1 — a symlinked .claude hid the finding\n%s", code, out)
	}
	if !strings.Contains(out, "FINDING") || !strings.Contains(out, ".credentials.json") {
		t.Errorf("must name the file found through the symlink:\n%s", out)
	}
}

// Same trap, reached through CLAUDE_CONFIG_DIR — the header's own claim is
// that setting the variable cannot turn a finding into a silent pass.
func TestQACredentialPathsFollowsASymlinkedConfigDirEnv(t *testing.T) {
	real := t.TempDir()
	if err := os.WriteFile(filepath.Join(real, ".credentials.json"), []byte(cpMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(t.TempDir(), "cfg")
	if err := os.Symlink(real, cfg); err != nil {
		t.Fatal(err)
	}
	out, code := cpRun(t, cpHome(t), "CLAUDE_CONFIG_DIR="+cfg)
	if code != 1 {
		t.Fatalf("exit %d, want 1 — a symlinked CLAUDE_CONFIG_DIR hid the finding\n%s", code, out)
	}
	if !strings.Contains(out, "FINDING") || !strings.Contains(out, ".credentials.json") {
		t.Errorf("must name the file found through the symlinked CLAUDE_CONFIG_DIR:\n%s", out)
	}
}

// A directory find cannot read is not evidence of clean — it is nothing
// measured, same reasoning as the present==0 arm (ranger-base-dpuf).
func TestQACredentialPathsRefusesToPassOverAnUnreadableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	home := cpHome(t, ".credentials.json")
	dir := filepath.Join(home, ".claude")
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)

	out, code := cpRun(t, home)
	if code != 2 {
		t.Fatalf("exit %d, want 2\n%s", code, out)
	}
	if strings.Contains(out, "clean") {
		t.Errorf("an unreadable dir must not read as clean:\n%s", out)
	}
	if !strings.Contains(out, "nothing measured") {
		t.Errorf("must say it measured nothing:\n%s", out)
	}
}

func TestQACredentialPathsIsReadOnlyAndWired(t *testing.T) {
	info, err := os.Stat(cpScript)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("%s is not executable", cpScript)
	}

	body, err := os.ReadFile(cpScript)
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	// Deleting a live credential is the operator's, every time (ranger-base-66y),
	// and `security` is denied fleet-wide.
	for _, forbidden := range []string{"rm ", "rm -", "mv ", "unlink", "security ", "shred", "truncate"} {
		for _, line := range strings.Split(src, "\n") {
			s := strings.TrimSpace(line)
			if strings.HasPrefix(s, "#") {
				continue
			}
			if strings.Contains(s, forbidden) {
				t.Errorf("%s must not invoke %q — it is read-only: %s", cpScript, forbidden, s)
			}
		}
	}
	// The matcher is the glob, not the exact path.
	if !strings.Contains(src, "'.credentials.json*'") {
		t.Error("the matcher must be the .credentials.json* glob (a rename is not a removal)")
	}

	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mk), "verify-credential-paths:\n\tscripts/verify-credential-paths.sh\n") {
		t.Error("Makefile lost the verify-credential-paths target")
	}
	if !strings.Contains(string(mk), "verify-credential-paths ") {
		t.Error("verify-credential-paths is not in .PHONY")
	}
}

// The runbook is the other half of "make the check run somewhere": a script
// nobody is told to run is the same parked check in a new location.
func TestQACredentialRotationRunbookCarriesTheCheck(t *testing.T) {
	body, err := os.ReadFile("docs/runbooks/credential-rotation.md")
	if err != nil {
		t.Fatalf("docs/runbooks/credential-rotation.md is the runbook ranger-base-zzc promised: %v", err)
	}
	doc := string(body)
	for _, want := range []string{
		"make verify-credential-paths", // the command, runnable as written
		".credentials.json*",           // the glob, so doc and script cannot drift
		"rangerhq-m10j",                // whose page this is
		// Until 2026-08-29 this arm read "rangerhq-q65q" — the operator
		// decision the page's other three moves were parked behind, named
		// so a reader could see this section was NOT waiting with them.
		// They have landed, so that reference went with them and the arm
		// holds the fact that outlasted it instead: the file self-renews,
		// which is why the sweep exists and a delete is not a control.
		"ranger-base-1lza",
		"ranger-base-66y", // deleting is the operator's
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("runbook must carry %q", want)
		}
	}
	if !strings.Contains(doc, "Exit 0 is clean") || !strings.Contains(doc, "not a pass") {
		t.Error("runbook must say how to read the exit codes, including that 2 is not a pass")
	}
}
