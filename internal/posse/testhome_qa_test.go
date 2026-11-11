package posse

// TestMain's temp HOME: one dir per test TREE, and mise's dirs left where the
// operator's are (ranger-base-32bdv).
//
// Both halves are about the same accident. TestMain replaces $HOME so that no
// test reaches the operator's live ~/.posse, and anything that resolves its
// own storage under $HOME follows it there. GOMODCACHE and GOCACHE were the
// first readers found doing it (ranger-base-6vvgs, which pinned them); mise
// is the second, and it costs more, because on this box `claude`, `codex`,
// `grok`, `gh` and `node` are mise command-wrappers rather than binaries.
//
// MEASURED 2026-11-11 on linux/amd64. /tmp here is a 14G tmpfs mounted
// usrquota and every seat shares uid posse, so it is one box-wide budget:
// 427 leaked temp homes, 11G of 14G spent, and main's corpus-check failing
// with "disk quota exceeded" for every seat. 417 of the 427 were the 4.0K
// .codex-only dir a re-exec CHILD leaves — and 5.25G of the bytes were in
// four of them, every byte under .cache/mise and
// .local/share/mise/installs/{claude,codex,grok}, because `mise ls
// --installed` under a temp HOME reports nothing installed and re-installs
// the toolchain before answering the wrapper.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The three dirs mise resolves under $HOME, and so the three TestMain pins.
// MISE_CONFIG_DIR is deliberately NOT here: configuration is an input to this
// binary and the temp HOME is what fences the operator's out.
var misePinnedDirs = []string{"MISE_DATA_DIR", "MISE_CACHE_DIR", "MISE_STATE_DIR"}

func TestMiseDirsArePinnedOutsideTheTempHome(t *testing.T) {
	t.Parallel()

	home := os.Getenv("HOME")
	if home == "" {
		t.Fatal("no $HOME — TestMain always sets one")
	}
	if operatorHome == "" {
		t.Skip("no operator home to pin against")
	}
	for _, key := range misePinnedDirs {
		v := os.Getenv(key)
		if v == "" {
			t.Errorf("%s is unset — a test that execs a mise command-wrapper (claude, codex, grok, gh, node) makes mise re-install the toolchain under the temp HOME, 2.3G in the worst leaked dir measured", key)
			continue
		}
		if v == home || strings.HasPrefix(v, home+string(os.PathSeparator)) {
			t.Errorf("%s = %q is under the temp HOME %q — this is the defect, not the fix", key, v, home)
		}
	}
	// And the fence the pin must not have broken: the operator's config is
	// still out of reach, so mise installs nothing on its own account.
	if v := os.Getenv("MISE_CONFIG_DIR"); v != "" && !strings.HasPrefix(v, home) {
		t.Errorf("MISE_CONFIG_DIR = %q points outside the temp HOME — mise's user config is an INPUT to this binary and the temp HOME is what keeps it out", v)
	}
}

// TestTempHomeChild is the child arm of the pin below. It reports the $HOME it
// was started with, which is the whole measurement: a child that makes its
// own prints a different dir, and leaves it behind when the parent kills it.
func TestTempHomeChild(t *testing.T) {
	if os.Getenv("RHQ_QA_TESTHOME_CHILD") == "" {
		t.Skip("child of TestTheTempHomeIsOnePerTestTree")
	}
	os.Stdout.WriteString("CHILDHOME " + os.Getenv("HOME") + "\n")
	// …and the codex credential the CREATOR planted, which an adopting child
	// no longer writes for itself. Reported as bytes rather than as a verdict
	// so the parent is the one that decides: an empty read is the truncation
	// race the guard exists to prevent, and it must not look like an absence.
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".codex", "auth.json"))
	if err != nil {
		os.Stdout.WriteString("CHILDCRED error " + err.Error() + "\n")
		return
	}
	os.Stdout.WriteString("CHILDCRED bytes " + strconv.Itoa(len(b)) + "\n")
}

// A re-exec child adopts the parent's temp HOME instead of cutting a second
// one. This is the pin for the 417: the removal in TestMain is reached only
// when m.Run RETURNS, and the fifteen sites that start a child kill it long
// before that, so every home a child made was a home nobody removed.
func TestTheTempHomeIsOnePerTestTree(t *testing.T) {
	t.Parallel()

	home := os.Getenv("HOME")
	if got := os.Getenv(envTestHome); got != home {
		t.Fatalf("%s = %q, want the temp HOME %q — a child reads this to adopt, and adopts nothing when it is wrong", envTestHome, got, home)
	}
	if got := os.Getenv(envTestOperatorHome); got != operatorHome {
		t.Errorf("%s = %q, want the operator's %q — a child-run test that reads operatorHome otherwise measures its parent's tempdir", envTestOperatorHome, got, operatorHome)
	}

	t.Logf("this binary's temp HOME: %s", home)
	started := time.Now()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe, "-test.run=^TestTempHomeChild$", "-test.v")
	// RHQ_FAKE_HERDR taken out, or TestMain answers as the fake substrate and
	// the child never runs the test (reexecenv_qa_test.go is the census).
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RHQ_FAKE_HERDR=") {
			env = append(env, kv)
		}
	}
	child.Env = append(env, "RHQ_QA_TESTHOME_CHILD=1")
	out, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}

	var reported string
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "CHILDHOME "); ok {
			reported = rest
		}
	}
	if reported == "" {
		t.Fatalf("the child never reported its HOME:\n%s", out)
	}
	if reported != home {
		t.Errorf("the child ran under HOME=%q, want the parent's %q — it cut its own, and a parent that kills it leaves that dir on a tmpfs every seat shares", reported, home)
	}

	// The fixture the creator plants and an adopter does not: a child must
	// still FIND it, or guarding that write on ownHome traded a truncation
	// race for codexSigninProbe reading "cannot authenticate" in every child.
	want := "CHILDCRED bytes " + strconv.Itoa(len(`{}`))
	if !strings.Contains(string(out), want) {
		var got string
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "CHILDCRED ") {
				got = strings.TrimSpace(line)
			}
		}
		t.Errorf("the child read the codex credential as %q, want %q — the creator plants it and an adopting child does not, so this is the whole of what makes that safe", got, want)
	}
	// And it created nothing of its own beside ours.
	siblings, _ := filepath.Glob(filepath.Join(filepath.Dir(home), "posse-testhome-*"))
	for _, s := range siblings {
		if s == home {
			continue
		}
		if fi, serr := os.Stat(s); serr == nil && fi.ModTime().After(started) {
			t.Errorf("%s appeared during this test — something still cuts a second temp HOME", s)
		}
	}
}
