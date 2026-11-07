package main

// QA, ranger-base-nhxdx — ADR 0076 Amendment 3 (D12/D13) asks for the row an
// operator can read BEFORE a launch: on a `multiplexer: rex` instance, which
// runtimes launch here, and for each that does not, WHY and what to do.
//
// `posse gates <persona>` is the surface that already carries the other two
// Rex readings (D1's admission and D9's version pin), so the precondition
// joins them there — it is the one screen where all three are in the same
// columns, and a reading in the wrong column is a lever that reads as
// connected and is not (ADR 0017 §1).
//
// Driven through the BUILT BINARY, for gatesruntimemissing_qa_test.go's
// reason: the defect class is the command's, not the renderer's. Both arms
// supply their own `grok` on PATH and their own fixture `$GROK_HOME`, so
// nothing here reads the operator's own config or the box's own grok — the
// box this was written on HAS the line, so a pin that read the live config
// would be green here and undefined everywhere else.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// grokRowHome is a `multiplexer: rex` instance with one grok persona.
func grokRowHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "agents", "builder.md"),
		[]byte("---\nname: builder\ndescription: builder\nruntime: grok\n---\nbuilder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("multiplexer: rex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

// grokRowFakeCLI puts a `grok` on PATH that answers the release grok's
// built-in declares, so the VERSION pin (D9/D13, read first) cannot stand in
// for the precondition this pin is about. The version string is typed here on
// purpose: it is the one place this file asserts which release the key was
// measured on, and a bump that forgets this row reds it rather than silently
// re-aiming the pin at the new one.
func grokRowFakeCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sh := "#!/bin/sh\nprintf 'grok 1.0.52 (0920c8169e27)\\n'\n"
	p := filepath.Join(dir, "grok")
	if err := os.WriteFile(p, []byte(sh), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// grokRowGrokHome writes a fixture grok home whose config.toml carries body,
// and returns it for $GROK_HOME.
func grokRowGrokHome(t *testing.T, body string) string {
	t.Helper()
	h := filepath.Join(t.TempDir(), ".grok")
	if err := os.MkdirAll(h, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return h
}

func grokRowRun(t *testing.T, bin, home, grokHome, cliDir string) string {
	t.Helper()
	cmd := exec.Command(bin, "gates", "builder")
	cmd.Dir = t.TempDir()
	cmd.Env = []string{
		"HOME=" + filepath.Join(home, "h"),
		"RHQ_HOME=" + home,
		"GROK_HOME=" + grokHome,
		"PATH=" + cliDir + ":/usr/bin:/bin",
	}
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// The two arms that make each other mean something: the SAME instance, the
// same grok, the same persona — one config line apart.
func TestQAGatesShowsTheGrokProgramStatusPreconditionRow(t *testing.T) {
	bin := buildRhq(t)
	cli := grokRowFakeCLI(t)
	home := grokRowHome(t)

	// `auto` is the SHIPPED DEFAULT, and MEASURED both silent on Rex and
	// leaking the `n,auth` tail of Rex's reply into grok's composer. A
	// screen that showed it under "launches here" would be sending an
	// operator to spend a claim, a worktree and a billed turn.
	unmet := grokRowRun(t, bin, home, grokRowGrokHome(t, "[ui.notifications]\nprogram_status = \"auto\"\n"), cli)
	for _, want := range []string{
		"REFUSED here by the program_status precondition",
		"ADR 0076 D12",
		"grok",
		"auto",
		"[ui.notifications]",
		"never writes",
		"verify-grok-pin",
	} {
		if !strings.Contains(unmet, want) {
			t.Errorf("the gates report over a grok config that says `auto` never says %q:\n%s", want, unmet)
		}
	}
	// And it is NOT in the column that says it launches.
	for _, line := range strings.Split(unmet, "\n") {
		if strings.Contains(line, "launches here:") && strings.Contains(line, "grok") {
			t.Errorf("grok is listed as launching here over a config that reports nothing: %q", line)
		}
	}

	met := grokRowRun(t, bin, home, grokRowGrokHome(t, "[ui.notifications]\nprogram_status = \"on\"\n"), cli)
	var launches string
	for _, line := range strings.Split(met, "\n") {
		if strings.Contains(line, "launches here:") {
			launches = line
		}
	}
	if !strings.Contains(launches, "grok") {
		t.Errorf("with the line in place grok is still not shown as launching here (%q):\n%s", launches, met)
	}
	if strings.Contains(met, "REFUSED here by the program_status precondition") {
		t.Errorf("the precondition row printed over a config that carries the line:\n%s", met)
	}
}
