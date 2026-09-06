package posse

// QA pins for rangerhq-qvnt (verifying under rangerhq-qd3o).
// Claim: $NEW is seeded by an explicit allowlist copy with a clearance
// preflight, never a tree copy. Two surfaces, one file:
//
//   · In the published tree the script does not exist (it is the one file
//     that must never cross). The pins then are the root commit: no
//     excluded paths, every ADR carries its provenance header, and the
//     seed script is absent from history.
//   · In the private archive the script exists. The pins then run it
//     against a throwaway $OLD: untracked/dirty/excluded do not cross,
//     the refusals hold, and each preflight exception is per occurrence.
//
// Self-contained (own helpers) so the next edit to a neighbour cannot carry
// the pin away — with ONE exception, the repo-root helper. This file used to
// carry qspRepoRoot, byte-for-byte identical to instancebound_qa_test.go's
// qibRepoRoot, and the tree-wide door census (treewidedoor_qa_test.go) keys
// on that identifier: a pin spelled with the twin was outside the derived
// class and got no door, silently (ranger-base-sx2dq). There is now one
// repo-root helper in this package's tests, qibRepoRoot, and
// TestQAOneRepoRootHelperInTheTestPackage keeps a third copy from arriving.

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func qspGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(bytes.TrimSpace(out))
}

func qspSeedScript(t *testing.T) string {
	t.Helper()
	p := filepath.Join(qibRepoRoot(t), "docs", "runbooks", "0012-seed-publication.sh")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

func qspRun(t *testing.T, script string, extraEnv []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.Env = append(os.Environ(), extraEnv...)
	err := cmd.Run()
	stdout, stderr = outb.String(), errb.String()
	if err == nil {
		return stdout, stderr, 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return stdout, stderr, ee.ExitCode()
	}
	t.Fatalf("run %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, stdout, stderr)
	return "", "", -1
}

// Tiny $OLD: allowlisted files, excluded private matter, gitignored residue,
// plus untracked and dirty working-tree edits that a `cp -a` would take.
func qspFakeOld(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	old := t.TempDir()
	write := func(rel, body string, mode os.FileMode) {
		t.Helper()
		p := filepath.Join(old, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("AGENTS.md", "hello AGENTS\n", 0o644)
	write("NOTES.md", "notes\n", 0o644)
	write("go.mod", "module example.com/m\n", 0o644)
	write("LICENSE", "LICENSE text\n", 0o644)
	write("CONTRIBUTING.md", "CONTRIBUTING\n", 0o644)
	write("HISTORY.md", "HISTORY\n", 0o644)
	write("cmd/posse/main.go", "package main\n", 0o644)
	write("internal/posse/app.go", "package posse\n", 0o644)
	write("plugin/herdr-plugin.toml", "plugin\n", 0o644)
	write(".claude/settings.json", "{}\n", 0o644)
	write(".gitignore", "bin/\nplugin/bin/\n.claude/settings.local.json\n.beads/interactions.jsonl\n", 0o644)
	write("docs/adr/public/0001-persona-intent-documents.md", "Restated from the private archive of the originating instance.\npublic restatement\n", 0o644)
	write("docs/adr/public/README.md", "THIS README DOES NOT SHIP\n", 0o644)
	write("docs/adr/0001-persona-intent-documents.md", "ORIGINAL ADR\n", 0o644)
	write("docs/adr/0002-container-tier.probe.sh", "probe\n", 0o755)
	write("docs/adr/0012-harness-instance-boundary.md", "ORIGINAL 0012\n", 0o644)
	write("docs/runbooks/0012-cutover.md", "runbook\n", 0o644)
	write("docs/spikes/secret.md", "spike secret\n", 0o644)
	write(".beads/issues.jsonl", "private tracker\n", 0o644)
	write("bin/posse-go", "binary residue\n", 0o644)
	write(".claude/settings.local.json", "local secret\n", 0o644)
	write("plugin/bin/posse", "built plugin\n", 0o644)

	qspGit(t, old, "init", "-q")
	qspGit(t, old, "config", "user.email", "t@t")
	qspGit(t, old, "config", "user.name", "t")
	qspGit(t, old, "add", "-A")
	qspGit(t, old, "add", "-f", "docs/adr/0002-container-tier.probe.sh")
	qspGit(t, old, "commit", "-qm", "seed-test base")

	write("cmd/posse/untracked_leak.go", "UNTRACKED LEAK\n", 0o644)
	write("internal/posse/coordinator_variant_test.go", "UNTRACKED IN INTERNAL\n", 0o644)
	f, err := os.OpenFile(filepath.Join(old, "NOTES.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("DIRTY WORKING TREE\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return old
}

func qspSeed(t *testing.T, script, old, dest string, extra ...string) (stdout, stderr string, code int) {
	t.Helper()
	args := append([]string{"--old", old, "--new", dest}, extra...)
	return qspRun(t, script, nil, args...)
}

// --- published-tree pins (skip in the private archive) ---

func TestPublicationRootCommitOmitsExcludedPaths(t *testing.T) {
	t.Parallel()
	if qspSeedScript(t) != "" {
		t.Skip("private archive: the root commit is not a publication seed")
	}
	root := qibRepoRoot(t)
	sha := qspGit(t, root, "rev-list", "--max-parents=0", "HEAD")
	if strings.Contains(sha, "\n") {
		t.Fatalf("expected one root commit, got:\n%s", sha)
	}
	listing := qspGit(t, root, "ls-tree", "-r", "--name-only", sha)
	var hits []string
	for _, p := range strings.Split(listing, "\n") {
		if p == "" {
			continue
		}
		switch {
		case p == "docs/runbooks/0012-seed-publication.sh",
			strings.HasPrefix(p, ".beads/"),
			strings.HasPrefix(p, "docs/runbooks/"),
			strings.HasPrefix(p, "docs/spikes/"),
			strings.HasPrefix(p, "docs/adr/public/"),
			strings.HasPrefix(p, "bin/"),
			strings.HasPrefix(p, "docs/adr/0013-"),
			strings.HasPrefix(p, "docs/adr/0014-"),
			strings.HasPrefix(p, "docs/adr/0015-"),
			strings.HasPrefix(p, "docs/adr/0016-"),
			strings.HasPrefix(p, "docs/adr/0017-"),
			strings.HasPrefix(p, "docs/adr/0018-"),
			strings.HasPrefix(p, "docs/adr/0019-"):
			hits = append(hits, p)
		}
	}
	if len(hits) > 0 {
		t.Fatalf("root %s carries excluded paths (tree copy, not the allowlist):\n  %s", sha, strings.Join(hits, "\n  "))
	}
}

func TestPublicationRootCommitADRsCarryProvenance(t *testing.T) {
	t.Parallel()
	if qspSeedScript(t) != "" {
		t.Skip("private archive: the root commit is not a publication seed")
	}
	root := qibRepoRoot(t)
	sha := qspGit(t, root, "rev-list", "--max-parents=0", "HEAD")
	listing := qspGit(t, root, "ls-tree", "-r", "--name-only", sha)
	const header = "Restated from the private archive"
	var missing []string
	n := 0
	for _, p := range strings.Split(listing, "\n") {
		if !strings.HasPrefix(p, "docs/adr/") || !strings.HasSuffix(p, ".md") {
			continue
		}
		n++
		body := qspGit(t, root, "show", sha+":"+p)
		if !strings.Contains(body, header) {
			missing = append(missing, p)
		}
	}
	if n == 0 {
		t.Fatal("root commit has no docs/adr/*.md")
	}
	if len(missing) > 0 {
		t.Fatalf("ADRs without provenance header (unstaged original?): %s", strings.Join(missing, ", "))
	}
}

func TestPublicationHistoryNeverCarriesTheSeedScript(t *testing.T) {
	t.Parallel()
	if qspSeedScript(t) != "" {
		t.Skip("private archive: the seed script lives here on purpose")
	}
	root := qibRepoRoot(t)
	log := qspGit(t, root, "log", "--all", "--oneline", "--", "docs/runbooks/0012-seed-publication.sh")
	if log != "" {
		t.Fatalf("seed script is in history (it names the crew in its own patterns):\n%s", log)
	}
}

// --- script pins (skip in the published tree) ---

func TestSeedScriptDryRunCreatesNothing(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	stdout, stderr, code := qspSeed(t, script, old, dest, "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("--dry-run created %s", dest)
	}
	if !strings.Contains(stdout, "would cross") {
		t.Fatalf("dry-run did not list would-cross:\n%s", stdout)
	}
	if !strings.Contains(stderr, "working tree is dirty") {
		t.Fatalf("dirty $OLD produced no warning:\n%s", stderr)
	}
}

func TestSeedScriptAllowlistSkipsUntrackedDirtyAndExcluded(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	stdout, stderr, code := qspSeed(t, script, old, dest)
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "PREFLIGHT GREEN") {
		t.Fatalf("want PREFLIGHT GREEN, got:\n%s", stdout)
	}

	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(dest, rel))
		return err == nil
	}
	must := []string{
		"AGENTS.md", "NOTES.md", "LICENSE", "CONTRIBUTING.md", "HISTORY.md",
		"cmd/posse/main.go", "internal/posse/app.go",
		"docs/adr/0001-persona-intent-documents.md",
		"docs/adr/0002-container-tier.probe.sh",
	}
	for _, p := range must {
		if !exists(p) {
			t.Errorf("allowlisted %s did not cross", p)
		}
	}
	mustNot := []string{
		"cmd/posse/untracked_leak.go",
		"internal/posse/coordinator_variant_test.go",
		".beads",
		".git",
		"docs/runbooks",
		"docs/spikes",
		"docs/adr/public",
		"bin",
		"plugin/bin",
		".claude/settings.local.json",
		"docs/adr/0012-harness-instance-boundary.md",
		"docs/adr/README.md",
		"docs/runbooks/0012-seed-publication.sh",
	}
	for _, p := range mustNot {
		if exists(p) {
			t.Errorf("excluded/untracked %s crossed", p)
		}
	}
	notes, err := os.ReadFile(filepath.Join(dest, "NOTES.md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(notes, []byte("DIRTY WORKING TREE")) {
		t.Error("dirty working-tree NOTES.md crossed; the source must be the commit")
	}
	adr, err := os.ReadFile(filepath.Join(dest, "docs/adr/0001-persona-intent-documents.md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(adr, []byte("ORIGINAL ADR")) {
		t.Error("docs/adr/0001 is the original, not the public restatement")
	}
	if !bytes.Contains(adr, []byte("public restatement")) {
		t.Error("docs/adr/0001 is not the public restatement")
	}
	st, err := os.Stat(filepath.Join(dest, "docs/adr/0002-container-tier.probe.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&0o111 == 0 {
		t.Error("0002 probe crossed without +x")
	}
}

func TestSeedScriptRefusesNonEmptyDest(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	leftover := filepath.Join(dest, "leftover.txt")
	if err := os.WriteFile(leftover, []byte("leftover\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := qspSeed(t, script, old, dest)
	if code != 1 {
		t.Fatalf("non-empty dest: want exit 1, got %d\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "not empty") {
		t.Fatalf("refusal did not name non-empty:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(dest, "AGENTS.md")); err == nil {
		t.Fatal("non-empty dest was mutated")
	}
}

func TestSeedScriptForceLeavesExtrasForCheck6(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if err := os.MkdirAll(filepath.Join(dest, "docs", "runbooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "docs/runbooks/secret.md"), []byte("leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := qspSeed(t, script, old, dest, "--force")
	if code != 2 {
		t.Fatalf("--force with extras: want exit 2, got %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "FAIL") || !strings.Contains(stdout, "excluded paths") {
		t.Fatalf("check 6 did not fire:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dest, "docs/runbooks/secret.md")); err != nil {
		t.Fatal("--force wiped planted extras; check 6 would then be blind")
	}
}

func TestSeedScriptRefusesToRunInsideNew(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	dest := t.TempDir()
	copy := filepath.Join(dest, "0012-seed-publication.sh")
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copy, body, 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := qspRun(t, copy, nil, "--preflight-only", "--new", dest)
	if code != 1 {
		t.Fatalf("script inside $NEW: want exit 1, got %d\n%s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(stderr), "inside") {
		t.Fatalf("refusal did not say inside:\n%s", stderr)
	}
}

func TestSeedScriptPreflightIsolatedPrivateRepoProseIsRed(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if _, _, code := qspSeed(t, script, old, dest); code != 0 {
		t.Fatalf("setup seed: exit %d", code)
	}
	// Assembled so this file itself is not a check-4 hit.
	prose := "the " + "ranger" + "-base" + " private repo\n"
	f, err := os.OpenFile(filepath.Join(dest, "NOTES.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(prose); err != nil {
		t.Fatal(err)
	}
	f.Close()
	stdout, _, code := qspRun(t, script, nil, "--preflight-only", "--new", dest)
	if code != 2 {
		t.Fatalf("isolated private-repo prose: want exit 2, got %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "private-repo prose") {
		t.Fatalf("check 4 did not fire:\n%s", stdout)
	}
}

func TestSeedScriptPreflightCheck3SameLineDoesNotHide(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if _, _, code := qspSeed(t, script, old, dest); code != 0 {
		t.Fatalf("setup seed: exit %d", code)
	}
	// Assembled so this file itself is not a check-3 hit.
	line := "paths " + "/Users/" + "x" + " and " + "/Users/" + "nobodyboth" + "\n"
	f, err := os.OpenFile(filepath.Join(dest, "NOTES.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(line); err != nil {
		t.Fatal(err)
	}
	f.Close()
	stdout, _, code := qspRun(t, script, nil, "--preflight-only", "--new", dest)
	if code != 2 {
		t.Fatalf("check 3 same-line: want exit 2, got %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "no operator paths") {
		t.Fatalf("check 3 did not fire:\n%s", stdout)
	}
}

func TestSeedScriptPreflightCheck4SameLineDoesNotHideProse(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if _, _, code := qspSeed(t, script, old, dest); code != 0 {
		t.Fatalf("setup seed: exit %d", code)
	}
	// Marker + unexcused prose on one line. Check 3/5 (grep -o) catch this;
	// check 4 (grep -n | grep -vE) does not — ranger-base-8fz, closed
	// non-applicable per ranger-base-0z3u: the defective script lives only in
	// the retired private archive, and this published tree skips above.
	// This test also held a second, UNCONDITIONAL t.Skip naming 8fz, which
	// would have stayed silent through the one event it exists for. It is
	// gone: the day a re-seed puts the runbook back here, this pin goes RED
	// with the repro instead of SKIP, which is what the close asked for ("fix
	// it then, before any publication run"). Both arms were measured against
	// the archive's script (ranger-base-t049): line-level check 4 RED,
	// per-occurrence check 4 GREEN. The fix shape is check 3's — match per
	// OCCURRENCE with grep -o and anchor the exception's id to end-of-match,
	// so an excused marker cannot swallow the rest of its line.
	line := "see " + "ranger" + "-base" + "-3jg vs the " + "ranger" + "-base" + " repo\n"
	f, err := os.OpenFile(filepath.Join(dest, "NOTES.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(line); err != nil {
		t.Fatal(err)
	}
	f.Close()
	stdout, _, code := qspRun(t, script, nil, "--preflight-only", "--new", dest)
	if code != 2 {
		t.Fatalf("check 4 same-line: want exit 2, got %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "private-repo prose") {
		t.Fatalf("check 4 did not fire:\n%s", stdout)
	}
}

func TestSeedScriptPreflightCheck5SameLineDoesNotHide(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if _, _, code := qspSeed(t, script, old, dest); code != 0 {
		t.Fatalf("setup seed: exit %d", code)
	}
	f, err := os.OpenFile(filepath.Join(dest, "NOTES.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("costs $" + "0 not $" + "12\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	stdout, _, code := qspRun(t, script, nil, "--preflight-only", "--new", dest)
	if code != 2 {
		t.Fatalf("check 5 same-line: want exit 2, got %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "spend figures") {
		t.Fatalf("check 5 did not fire:\n%s", stdout)
	}
}

func TestSeedScriptPreflightMissingLicenseIsRed(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if _, _, code := qspSeed(t, script, old, dest); code != 0 {
		t.Fatalf("setup seed: exit %d", code)
	}
	if err := os.Remove(filepath.Join(dest, "LICENSE")); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := qspRun(t, script, nil, "--preflight-only", "--new", dest)
	if code != 2 {
		t.Fatalf("missing LICENSE: want exit 2, got %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "publication files") {
		t.Fatalf("check 7 did not fire:\n%s", stdout)
	}
}

func TestSeedScriptPreflightPlantedBeadsIsRed(t *testing.T) {
	t.Parallel()
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if _, _, code := qspSeed(t, script, old, dest); code != 0 {
		t.Fatalf("setup seed: exit %d", code)
	}
	if err := os.MkdirAll(filepath.Join(dest, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, ".beads/issues.jsonl"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := qspRun(t, script, nil, "--preflight-only", "--new", dest)
	if code != 2 {
		t.Fatalf("planted .beads: want exit 2, got %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "excluded paths") {
		t.Fatalf("check 6 did not fire:\n%s", stdout)
	}
}

// 7xpn's AC7 is 0 real occurrences of the old harness name on the seed
// surface. The seed preflight prints that as INFO, not a check, so a later
// commit can put the name back and still print PREFLIGHT GREEN. This pin
// is the check. Needle is assembled so this file is not itself a hit.
//
// AND IT IS THE BACKSTOP, NOT THE WALL (ADR 0048 D3). It walks the working
// tree and it runs when someone runs this package — after the commit is on
// public main, which is why seven recurrences each closed as a reword. The
// commit-time arm is an instance pattern under config
// beads_visibility_patterns:, scanned over every staged file and every
// added path (ADR 0048 D1/D2, built in ranger-base-uzgkz). This pin stays
// because it is the only guard on a box whose config lacks that line, on a
// commit made under the typed override, and on a re-render nobody ran — and
// its failure text says so, so an eighth red is read as "the wall is not
// stamped here" rather than as a reword ticket.
func TestSeedSurfaceNameCountIsZero(t *testing.T) {
	t.Parallel()
	if qspSeedScript(t) != "" {
		t.Skip("private archive: the surface this counts is $NEW's, not $OLD's")
	}
	needle := "ranger" + "hq"

	hits := qspSurfaceHits(t, qibRepoRoot(t), needle)
	if len(hits) > 0 {
		t.Fatalf("%d real %s token(s) on the seed surface (7xpn AC7 is 0; markers of the form %s-<id> stay).\n"+
			"The wall arm is an instance pattern under config %s: (ADR 0048); this pin is the post-landing backstop —\n"+
			"a red here means the commit-time refusal did not fire (unstamped box, typed override, or a re-render nobody ran), not that this line needs an eighth rewording.\n  %s",
			len(hits), needle, needle, OpsPatternsConfigKey, strings.Join(hits, "\n  "))
	}
}

// qspSurfaceHits returns "<rel>:<line>: <text>" for every real occurrence of
// needle under root — the seed surface. Markers of the form <needle>-<id> are
// bead ids, not the name, and do not count.
//
// IT SKIPS WHAT GIT SKIPS (qspGitIgnored). The surface is what a publication
// carries, and git-ignored build output is not on it: `make build` writes the
// gitignored bin/posse-go, whose string table holds the token, so before this
// the walk read a 13MB Mach-O and reported a binary offset as a source line —
// `make build && make test` was red for everyone, and the failure text below
// sent that reader after a commit-time wall that was working
// (ranger-base-n0v6o). The sibling arm in
// TestPublicationRootCommitOmitsExcludedPaths already keeps bin/ off the
// surface; the two arms of this file now agree about it.
//
// IT ALSO SKIPS WHAT IS NOT TEXT (the second skip in the body), because the
// ignore set is empty wherever there is no checkout to ask and the same build
// output then walks straight back onto the surface — ranger-base-chd6w.
//
// Extracted from TestSeedSurfaceNameCountIsZero so the skip has a pin of its
// own against a fixture repo (TestSeedSurfaceScanSkipsGitIgnoredPaths): the
// live tree's ignored paths are build output that may or may not exist when
// the suite runs, so the live pin cannot witness the skip either way.
func qspSurfaceHits(t *testing.T, root, needle string) []string {
	t.Helper()
	token := regexp.MustCompile(needle + `(-[0-9a-z]+)?`)
	marker := regexp.MustCompile(`^` + needle + `-[0-9a-z]+$`)
	ignored := qspGitIgnored(t, root)

	var hits []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == ".beads" || ignored[rel] {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || ignored[rel] {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		// AND IT SKIPS WHAT IS NOT TEXT. The ignore set above exists only
		// where root is the top of a checkout; an export has none, and
		// `make build` inside a `git archive` scratch tree — the house
		// mutation rig — writes that same 13MB Mach-O with nothing to skip
		// it, reproducing ranger-base-n0v6o byte for byte — measured
		// twice, `bin/posse-go:8185` under ranger-base-5htxx and
		// `bin/posse-go:8189` here: the offset moves with the build, the
		// hit does not (ranger-base-chd6w). The two skips are a
		// union on purpose: the ignore set still saves reading build output
		// in a checkout and still collapses whole directories, and this arm
		// covers the trees where there is no ignore set to have.
		//
		// A NUL byte is git's own test for "not text" (git looks at the
		// first 8000; the body is already read whole here, so there is no
		// threshold to defend and none to pin). The seed surface is prose
		// and source: 0 of 951 tracked files carry a NUL anywhere, measured
		// in this worktree at 26d6a796, so this arm takes nothing off the
		// surface that was ever on it.
		if bytes.IndexByte(body, 0) >= 0 {
			return nil
		}
		for i, line := range strings.Split(string(body), "\n") {
			for _, m := range token.FindAllString(line, -1) {
				if marker.MatchString(m) {
					continue
				}
				hits = append(hits, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hits
}

// qspGitIgnored is the set of repo-relative paths git ignores under root,
// wholly-ignored directories collapsed to one entry (no trailing slash) so a
// walk can SkipDir on them. One `git ls-files`, not one `git check-ignore` per
// path.
//
// Empty when root is not the TOP of a checkout — a release tarball, or the
// `git archive` scratch tree the house mutation rig runs in. Empty is still
// the right answer there rather than borrowing a list: the at-the-top check is
// what makes that honest, since a scratch tree unpacked INSIDE some other
// checkout would otherwise answer with that repo's ignore list, keyed to a
// different root (TestSeedSurfaceScanTakesNoIgnoreListFromAForeignRepo).
//
// What empty is NOT is protection. Until 2026-09-06 this comment read "an
// export carries tracked files only, so nothing under it is ignored and the
// walk loses no coverage" — true of a PRISTINE export and false the moment
// anything writes into one. Measured under ranger-base-5htxx and again here:
// `git archive main | tar -x` gives the tracked files and zero hits (950 there,
// 951 here), and one `go build -o bin/posse-go ./cmd/posse` inside it puts
// ranger-base-n0v6o's Mach-O offset back on the surface. The caller's
// second skip — not this function — is what covers that
// (ranger-base-chd6w).
func qspGitIgnored(t *testing.T, root string) map[string]bool {
	t.Helper()
	ignored := map[string]bool{}
	if _, err := exec.LookPath("git"); err != nil {
		return ignored
	}
	// --show-prefix, NOT --show-toplevel: git's answer here is "" at the top
	// of a checkout and the subdirectory path below it, so this asks whether
	// root sits at the top and keeps nothing but the yes/no. A second
	// spelling of this repo's root is the thing that must not exist in this
	// package — there is one helper, qibRepoRoot, and a pin spelled with a
	// root from git stdout is outside the tree-wide class and gets no door
	// (TestQAOneRepoRootHelperInTheTestPackage, ranger-base-xndgk FINDING 5;
	// it red on exactly that here before this line read the way it does).
	prefix, err := exec.Command("git", "-C", root, "rev-parse", "--show-prefix").Output()
	if err != nil || strings.TrimSpace(string(prefix)) != "" {
		return ignored
	}
	// -z so a path with a quote, a backslash or a newline arrives whole:
	// git C-quotes those in its default output and core.quotePath=false does
	// not turn that off.
	out, err := exec.Command("git", "-C", root, "ls-files", "-z",
		"--others", "--ignored", "--exclude-standard", "--directory").Output()
	if err != nil {
		return ignored
	}
	for _, p := range strings.Split(string(out), "\x00") {
		if p == "" {
			continue
		}
		ignored[filepath.Clean(p)] = true
	}
	return ignored
}

// The pin on the skip itself, against a fixture repo. Two-way on purpose: the
// non-ignored file must still be a hit, or a scanner that had stopped reading
// anything at all would pass this and take the live pin down with it.
func TestSeedSurfaceScanSkipsGitIgnoredPaths(t *testing.T) {
	t.Parallel()
	needle := "ranger" + "hq"
	root := t.TempDir()
	qspGit(t, root, "init")

	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Both shapes git reports: a wholly-ignored DIRECTORY, which ls-files
	// collapses to one entry, and a single ignored FILE inside a directory
	// that is otherwise on the surface. They are skipped by different lines
	// in the walk, so a fixture carrying only the first leaves the second
	// unpinned.
	write(".gitignore", "bin/\nnotes/local.md\n")
	write("bin/posse-go", "string table: "+needle+"\n")
	write("notes/local.md", "scratch naming "+needle+"\n")
	write("kept.md", "a doc naming "+needle+"\n")
	write("notes/kept.md", "a note naming "+needle+"\n")

	var got []string
	for _, h := range qspSurfaceHits(t, root, needle) {
		got = append(got, h[:strings.Index(h, ":")])
	}
	sort.Strings(got)
	want := []string{"kept.md", filepath.Join("notes", "kept.md")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seed surface is %v, want %v\n"+
			"  extra = git-ignored paths on the surface (the defect); missing = the scan stopped reading",
			got, want)
	}
}

// The third shape, and the one the `-z` above is for: a LONE ignored FILE
// whose name git C-quotes. The directory shape hides it — a wholly-ignored
// directory collapses to `scratch/`, which has nothing to quote — so only a
// file inside a directory that is otherwise on the surface reaches the
// quoting at all. Measured: without `-z` git answers `"notes/od\"d.md"`,
// quotes and backslash included, and `core.quotePath=false` does NOT turn
// that off; the quoted spelling matches no walked path, so the file stays on
// the surface and the skip silently does nothing for it. That mutant survived
// all three pins above (ranger-base-5htxx), which is why this one exists.
func TestSeedSurfaceScanSkipsAnIgnoredPathGitCQuotes(t *testing.T) {
	t.Parallel()
	needle := "ranger" + "hq"
	root := t.TempDir()
	qspGit(t, root, "init")

	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	odd := "notes/od\"d.md"
	write(".gitignore", odd+"\n")
	write(odd, "scratch naming "+needle+"\n")
	write("notes/kept.md", "a note naming "+needle+"\n")

	var got []string
	for _, h := range qspSurfaceHits(t, root, needle) {
		got = append(got, h[:strings.Index(h, ":")])
	}
	sort.Strings(got)
	want := []string{filepath.Join("notes", "kept.md")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seed surface is %v, want %v\n"+
			"  extra = an ignored path whose name git C-quotes stayed on the surface, so the ignore set was read without -z;\n"+
			"  missing = the scan stopped reading",
			got, want)
	}
}

// The other half of qspGitIgnored: a tree that is NOT the top of a checkout
// takes no ignore list at all. An export unpacked inside some other repo —
// the `git archive` scratch tree the house mutation rig runs in, when it
// lands under a checkout rather than in /tmp — would otherwise be scanned
// against that repo's rules, and those rules were written about ITS paths.
// The failure is a false SKIP, not a false hit: below, the parent ignores
// notes/, and without the check the export's notes/kept.md silently leaves
// the surface. Why empty is the right answer for an export at all — and why
// it is an answer and not protection — is on qspGitIgnored above, amended
// 2026-09-06 (ranger-base-chd6w). This comment restated the falsified half
// of that reason until 2026-09-06; it points at the amendment instead, so
// there is one live copy of the claim and not a second one 130 lines from
// its own retraction (ranger-base-rpl85).
func TestSeedSurfaceScanTakesNoIgnoreListFromAForeignRepo(t *testing.T) {
	t.Parallel()
	needle := "ranger" + "hq"
	parent := t.TempDir()
	qspGit(t, parent, "init")
	if err := os.WriteFile(filepath.Join(parent, ".gitignore"), []byte("notes/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "export")
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", "kept.md"), []byte("a note naming "+needle+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, h := range qspSurfaceHits(t, root, needle) {
		got = append(got, h[:strings.Index(h, ":")])
	}
	want := []string{filepath.Join("notes", "kept.md")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seed surface under a non-toplevel root is %v, want %v\n"+
			"  missing = the enclosing repo's ignore rules were applied to a tree they were not written about",
			got, want)
	}
}

// The skip the ignore set cannot give you: build output in a tree that is not
// a checkout. This is ranger-base-chd6w's repro in miniature — the export
// shape, where qspGitIgnored is empty by construction and the walk has only
// the not-text arm to stop it. Two-way like its siblings: the source file
// beside the binary must still be a hit, or a scanner that had stopped reading
// anything would pass this and take the live pin down with it.
//
// The fixture leads with Mach-O magic and a run of NULs because that is what
// `go build -o bin/posse-go ./cmd/posse` writes into the `git archive` scratch
// tree; the token sits past them the way it sits in a real string table.
func TestSeedSurfaceScanSkipsBuildOutputWhereThereIsNoIgnoreSet(t *testing.T) {
	t.Parallel()
	needle := "ranger" + "hq"
	root := t.TempDir()
	// NO git init: an export has no checkout to ask. Assert the premise —
	// if this tree ever had an ignore set, the arm under test would not be
	// the one doing the work.
	if got := qspGitIgnored(t, root); len(got) != 0 {
		t.Fatalf("fixture is not the export shape: ignore set is %v, want empty", got)
	}

	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("bin/posse-go", "\xcf\xfa\xed\xfe\x00\x00\x00\x00string table: "+needle+" verify: username\n")
	write("kept.md", "a doc naming "+needle+"\n")

	var got []string
	for _, h := range qspSurfaceHits(t, root, needle) {
		got = append(got, h[:strings.Index(h, ":")])
	}
	sort.Strings(got)
	want := []string{"kept.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seed surface of an export with build output in it is %v, want %v\n"+
			"  extra = a file that is not text was read as source and a binary offset reported as a line (ranger-base-n0v6o);\n"+
			"  missing = the scan stopped reading",
			got, want)
	}
}
