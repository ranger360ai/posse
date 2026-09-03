package main

// `posse gates install-hooks` PRINTS THE DATA CEILING LINE FOR A
// PRIVATE-STAMPED REPO (ADR 0050 D3, ranger-base-nfg8l).
//
// Until ADR 0050 a private-stamped repo's install printed only its stamp:
// nothing inside the visibility gate runs there, so there was nothing to
// say. The ceiling runs under every stamp, so every hooked repo owes the
// operator the line naming the ceiling classes stamped in and, by class,
// the ones refused. The words are OpsPatternSet.WriteStampReport's and are
// pinned in internal/posse; this pin is the one that runs the BINARY into a
// private repo and reads its stdout, which is the surface the operator
// reads.
//
// MUTATION-CHECKED: the report call gated on a public stamp in main.go reds
// this pin; a WriteStampReport that prints the ceiling line only beside a
// non-empty instance list reds the internal pin and this one.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAInstallHooksPrintsTheCeilingLineForAPrivateRepo(t *testing.T) {
	bin := buildRhq(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	// RHQ_HOME is set EXPLICITLY to a scratch home: a dispatched session
	// exports the live home under one of the two names newApp reads, and a
	// binary under test must never read the operator's config.
	home := t.TempDir()
	repo := filepath.Join(home, "work")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	cfg := "beads_visibility:\n  " + repo + ": private\n" +
		"data_ceiling_patterns:\n  restricted-banner: QUOKKA[[:space:]]+RESTRICTED\n  bad-escape: QUOKKA" + `\d+` + "\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "gates", "install-hooks", repo)
	cmd.Env = append(os.Environ(), "HOME="+home, "RHQ_HOME="+home, "POSSE_HOME="+home)
	outb, err := cmd.CombinedOutput()
	out := string(outb)
	if err != nil {
		t.Fatalf("install-hooks: %v\n%s", err, out)
	}
	if !strings.Contains(out, "beads visibility guard: private") {
		t.Fatalf("fixture premise: the repo must be stamped private:\n%s", out)
	}
	for _, want := range []string{
		"data ceiling stamped in (config data_ceiling_patterns:), scanned under every stamp: restricted-banner",
		"data ceiling pattern REFUSED, not in force: bad-escape: ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("a private repo's install must print %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "QUOKKA") {
		t.Errorf("install-hooks echoed a ceiling value:\n%s", out)
	}
}
