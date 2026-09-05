package posse

// A PARKED pin for ranger-base-r2s9l finding 2, filed from the verify bead
// ranger-base-onx3x. ranger-base-rflee's FIX spec named four git exec
// inlets — "GIT_SSH_COMMAND / GIT_EXTERNAL_DIFF / GIT_PAGER / GIT_CONFIG_*"
// — and the landed table pins three. GIT_CONFIG_* appears nowhere in
// inletpin.go: not as a row, and not in its "NOT COVERED HERE,
// deliberately" paragraph, which names only the command-string FIELDS. That
// silence is what makes this a finding rather than a scope call, because the
// file's own contract is that its coverage is exactly the names in the table
// and "a name that is not here is not covered, and a reader has to be able
// to see that".
//
// The family is a real inlet and the pinned names do not shadow it. Env
// GIT_EXTERNAL_DIFF does shadow config diff.external — measured, and better
// than that row's own claim — but core.hooksPath and core.fsmonitor have no
// pinned environment name above them, and GIT_CONFIG_GLOBAL replaces the
// config file wholesale.
//
// It is EXPRESSIBLE, which is the other half of why this is a finding: git
// reads only up to GIT_CONFIG_COUNT, so 0 closes the KEY_n/VALUE_n family,
// and /dev/null closes the two file names. All three are non-empty
// spellings, so they reach the policy tier under the operator's 2026-09-04
// finding on ranger-base-rflee that an empty value does not take there.
//
// Parked rather than deleted because nothing sets a GIT_CONFIG* name on this
// box today, so this is preventive on exactly the terms the other 21 rows
// are. Unpark by deleting the t.Skip line.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitConfigInlets is the family ranger-base-rflee named and the table omits,
// with the value measured to be neutral for each (2026-09-05, git 2.50.1).
var gitConfigInlets = map[string]string{
	"GIT_CONFIG_COUNT":  "0",
	"GIT_CONFIG_GLOBAL": os.DevNull,
	"GIT_CONFIG_SYSTEM": os.DevNull,
}

func TestQATheInletPinClosesTheGitConfigFamily(t *testing.T) {
	t.Parallel()
	t.Skip("ranger-base-r2s9l: inletPin() pins three of the four git exec inlets ranger-base-rflee named — GIT_CONFIG_* is in neither the table nor its disclosed-gaps paragraph, and core.hooksPath/core.fsmonitor reach past the pinned names")

	pinned := map[string]string{}
	for _, v := range inletPin() {
		pinned[v.Key] = v.Value
	}
	for k, want := range gitConfigInlets {
		got, ok := pinned[k]
		if !ok {
			t.Errorf("inletPin() does not carry %s — ranger-base-rflee's fix spec named GIT_CONFIG_* beside the three git names the table does pin, and a settings payload can only SET keys, so a name that is not here is not covered", k)
			continue
		}
		if got != want {
			t.Errorf("inletPin()[%s] = %q, want %q (the value measured neutral)", k, got, want)
		}
	}

	// Both ends of the pin or neither: the drop-in is what covers the
	// operator's own uncaged session, which is the end this bead is about.
	const path = "../../etc/claude/managed-settings.d/10-posse-inlet-pin.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the policy-tier half of the pin is missing: %v", err)
	}
	var dropIn struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(b, &dropIn); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	for k := range gitConfigInlets {
		if _, ok := dropIn.Env[k]; !ok {
			t.Errorf("%s does not carry %s either, so the operator's own session is uncovered too", path, k)
		}
	}

	// The live half. Three arms against real git, the attack arms carrying
	// whatever inletPin() currently says — so this probe goes quiet by
	// itself once the rows land, rather than by being edited.
	repo := gitConfigProbeRepo(t)
	marker := filepath.Join(t.TempDir(), "marker")
	hooks := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooks, "post-checkout"),
		[]byte("#!/bin/sh\necho fired >> "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	fired := func(t *testing.T, extra ...string) bool {
		t.Helper()
		_ = os.Remove(marker)
		env := append(os.Environ(), extra...)
		for _, v := range inletPin() {
			env = append(env, v.Key+"="+v.Value)
		}
		// checkout away and back: post-checkout runs on each.
		for _, rev := range []string{"HEAD~1", "-"} {
			c := exec.Command("git", "checkout", "-q", rev)
			c.Dir, c.Env = repo, env
			_ = c.Run()
		}
		_, err := os.Stat(marker)
		return err == nil
	}

	// Control: the pin alone must not run the attacker's hook, or every
	// arm below is measuring the fixture instead of the inlet.
	if fired(t) {
		t.Fatal("CONTROL FAILED: the hook fired with no GIT_CONFIG_* set at all")
	}
	if !fired(t, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0="+hooks) {
		t.Log("core.hooksPath via GIT_CONFIG_COUNT no longer reaches — the pin closed it")
	} else {
		t.Errorf("core.hooksPath set through GIT_CONFIG_COUNT/KEY_0/VALUE_0 ran an attacker's post-checkout hook with the whole inlet pin applied: the git rows pin the three names ranger-base-rflee listed and none of them shadows this one")
	}
	if fired(t, "GIT_CONFIG_GLOBAL="+gitConfigProbeGlobal(t, hooks)) {
		t.Errorf("GIT_CONFIG_GLOBAL replaced the global config wholesale with the inlet pin applied, so core.hooksPath reached that way too")
	}
}

// gitConfigProbeRepo is two commits in a scratch repo — enough for a
// checkout to move and fire post-checkout.
func gitConfigProbeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", ".")
	run("config", "user.email", "qa@example.invalid")
	run("config", "user.name", "qa")
	for _, body := range []string{"one\n", "two\n"} {
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "--", "f")
		run("commit", "-q", "-m", "probe", "--", "f")
	}
	return dir
}

// gitConfigProbeGlobal writes the config file an attacker would point
// GIT_CONFIG_GLOBAL at.
func gitConfigProbeGlobal(t *testing.T, hooks string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "evil.gitconfig")
	if err := os.WriteFile(p, []byte("[core]\n\thooksPath = "+hooks+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
