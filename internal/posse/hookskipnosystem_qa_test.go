//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-r3pri's hook-skip arm, added under
// ranger-base-tmb7t: the SYSTEM-scope HIDE direction.
//
// ranger-base-zwkgb's attack is `GIT_CONFIG_SYSTEM=/dev/null` (and
// `GIT_CONFIG_NOSYSTEM=1`) on a box whose SYSTEM scope carries the
// employer's managed hook: the override hides hook config the seat would
// otherwise read, and git still exits 0. hookskip.go refuses it by CONTENT —
// the hook-relevant keys git sees under the override against the ones it sees
// with the override unset.
//
// No leg in hookskip_qa_test.go reaches that direction, and none can on this
// box: that fixture's managed hook lives in GLOBAL scope, and here
// /etc/gitconfig does not exist, so the real system scope is empty and every
// system override compares equal (empty == empty) and is correctly allowed.
// MEASURED under ranger-base-tmb7t: deleting the GIT_CONFIG_NOSYSTEM trigger
// from the env arm left every shipped hookskip pin GREEN.
//
// So the shim is rendered here against a FAKE git that reports a non-empty
// system scope when the scope is not overridden, which is what a real git
// does on a managed box. That makes the hide direction reachable without
// writing /etc/gitconfig. The scratch dirs are not the tree, so this is not
// a tree-wide pin and owes the Makefile no door.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hsSystemFake is a git that answers `config --system --get-regexp` with a
// managed hooksPath, and with nothing when the system scope is overridden or
// suppressed — a managed box, in two lines of sh.
const hsSystemFake = `#!/bin/sh
if [ "$1" = "config" ]; then
  case "$*" in
    *--system*)
      if [ -n "${GIT_CONFIG_SYSTEM+x}" ]; then exit 1; fi
      case "${GIT_CONFIG_NOSYSTEM:-}" in ''|0|false|no|off) ;; *) exit 1 ;; esac
      echo "core.hookspath /opt/managed/hooks"; exit 0 ;;
  esac
  exit 1
fi
exit 0
`

func TestQAHookSkipRefusesTheSystemScopeHideDirection(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "git")
	if err := WriteExecutable(fake, []byte(hsSystemFake), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "arm.sh")
	arm := renderHookSkipArm(fake, "", filepath.Join(dir, "hooks")) + renderHookSkipEnvCall()
	if err := os.WriteFile(shim, []byte(arm), 0o644); err != nil {
		t.Fatal(err)
	}

	// The seat running this suite carries its own GIT_CONFIG_* inlet pin
	// (GIT_CONFIG_SYSTEM above all), so it must be shed or every leg below
	// reads as an override.
	var base []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_CONFIG") {
			base = append(base, kv)
		}
	}
	base = append(base, "RHQ_GATE_LOG="+filepath.Join(dir, "refusals.log"))

	run := func(env string) (string, bool) {
		cmd := exec.Command("sh", "-c", ". "+shQuote(shim)+" commit -q -m x -- f")
		cmd.Env = base
		if env != "" {
			cmd.Env = append(append([]string{}, base...), env)
		}
		out, err := cmd.CombinedOutput()
		return string(out), err != nil && strings.Contains(string(out), "hookskip:config-env")
	}

	// Controls first: with no override, and with NOSYSTEM spelled as a git
	// false, nothing is hidden and nothing may be refused. Without these the
	// arms below cannot tell a working check from one that refuses always.
	for _, c := range []struct{ name, env string }{
		{"no override at all", ""},
		{"GIT_CONFIG_NOSYSTEM=0 is off", "GIT_CONFIG_NOSYSTEM=0"},
		{"GIT_CONFIG_NOSYSTEM= is off", "GIT_CONFIG_NOSYSTEM="},
	} {
		if out, refused := run(c.env); refused {
			t.Fatalf("CONTROL FAILED: %s was refused, so the arms below prove nothing:\n%s", c.name, out)
		}
	}

	// The hide direction: each of these drops the managed hook from the
	// seat's view, and each must be refused by name.
	for _, c := range []struct{ name, env string }{
		{"GIT_CONFIG_SYSTEM to /dev/null", "GIT_CONFIG_SYSTEM=" + os.DevNull},
		{"GIT_CONFIG_SYSTEM elsewhere", "GIT_CONFIG_SYSTEM=" + filepath.Join(dir, "empty")},
		{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_NOSYSTEM=1"},
		{"GIT_CONFIG_NOSYSTEM=true", "GIT_CONFIG_NOSYSTEM=true"},
	} {
		out, refused := run(c.env)
		if !refused {
			t.Errorf("%s hid the managed hook and was NOT refused — the ranger-base-zwkgb attack is open:\n%s", c.name, out)
			continue
		}
		if !strings.Contains(out, "rule: hookskip:config-env") {
			t.Errorf("%s: the refusal must name its rule:\n%s", c.name, out)
		}
	}

	// Every refusal is on the record.
	logb, err := os.ReadFile(filepath.Join(dir, "refusals.log"))
	if err != nil {
		t.Fatalf("a refusal must write a refusals.log line: %v", err)
	}
	if n := strings.Count(string(logb), "(hookskip: hookskip:config-env)"); n != 4 {
		t.Errorf("refusals.log must carry one line per refusal: got %d, want 4\n%s", n, logb)
	}
}
