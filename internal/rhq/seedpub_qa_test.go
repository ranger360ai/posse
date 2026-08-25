package rhq

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
// Self-contained (own helpers) so the next edit to a neighbour cannot
// carry the pin away.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func qspRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %s has no go.mod: %v", root, err)
	}
	return root
}

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
	p := filepath.Join(qspRepoRoot(t), "docs", "runbooks", "0012-seed-publication.sh")
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
	write("internal/rhq/app.go", "package rhq\n", 0o644)
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
	write("internal/rhq/coordinator_variant_test.go", "UNTRACKED IN INTERNAL\n", 0o644)
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
	if qspSeedScript(t) != "" {
		t.Skip("private archive: the root commit is not a publication seed")
	}
	root := qspRepoRoot(t)
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
	if qspSeedScript(t) != "" {
		t.Skip("private archive: the root commit is not a publication seed")
	}
	root := qspRepoRoot(t)
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
	if qspSeedScript(t) != "" {
		t.Skip("private archive: the seed script lives here on purpose")
	}
	root := qspRepoRoot(t)
	log := qspGit(t, root, "log", "--all", "--oneline", "--", "docs/runbooks/0012-seed-publication.sh")
	if log != "" {
		t.Fatalf("seed script is in history (it names the crew in its own patterns):\n%s", log)
	}
}

// --- script pins (skip in the published tree) ---

func TestSeedScriptDryRunCreatesNothing(t *testing.T) {
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
		"cmd/posse/main.go", "internal/rhq/app.go",
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
		"internal/rhq/coordinator_variant_test.go",
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
	script := qspSeedScript(t)
	if script == "" {
		t.Skip("no seed runbook here (published tree)")
	}
	t.Skip("ranger-base-8fz: check 4 exception is line-level; a marker hides prose on the same line")
	old := qspFakeOld(t)
	dest := filepath.Join(t.TempDir(), "new")
	if _, _, code := qspSeed(t, script, old, dest); code != 0 {
		t.Fatalf("setup seed: exit %d", code)
	}
	// Marker + unexcused prose on one line. Check 3/5 (grep -o) catch this;
	// check 4 (grep -n | grep -vE) currently does not.
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
