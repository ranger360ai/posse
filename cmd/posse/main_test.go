package main

// rangerhq-qv5: `posse new --help` used to take '--help' as the session name
// and create a real herdr workspace plus a meta file. The reading of a
// leading dashed argument lives in argLead; the end-to-end half runs the
// built binary, because "created nothing" is a claim about the process.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestArgLead(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		rest []string
		help bool
	}{
		{"help long", []string{"--help"}, []string{"--help"}, true},
		{"help short", []string{"-h"}, []string{"-h"}, true},
		{"help before a name", []string{"--help", "proj"}, []string{"--help", "proj"}, true},
		{"separator frees a dashed name", []string{"--", "--help"}, []string{"--help"}, false},
		{"separator alone", []string{"--"}, []string{}, false},
		{"a plain name", []string{"proj"}, []string{"proj"}, false},
		{"help after the name is the subcommand's own flag", []string{"proj", "--help"}, []string{"proj", "--help"}, false},
		{"no args", nil, nil, false},
	} {
		rest, help := argLead(c.args)
		if help != c.help || !reflect.DeepEqual(rest, c.rest) {
			t.Errorf("%s: argLead(%q) = (%q, %v), want (%q, %v)", c.name, c.args, rest, help, c.rest, c.help)
		}
	}
}

// The bead's regression test: usage on stdout, exit 0, and nothing left on
// the host — no meta file, and no herdr call at all (RHQ_HERDR_BIN points
// at a binary that fails loudly if anything reaches it).
func TestNewHelpCreatesNothing(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)

	for _, c := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"new", "--help"}, 0, "usage: posse new <name>"},
		{[]string{"new", "-h"}, 0, "usage: posse new <name>"},
		{[]string{"new", "-x"}, 1, "bad session name '-x'"},
		{[]string{"kill", "--help"}, 0, "usage: posse kill <name>"},
		{[]string{"attach", "--help"}, 0, "usage: posse attach <name>"},
		{[]string{"up", "--help"}, 0, "usage: posse attach <name>"},
		{[]string{"local", "--help"}, 0, "usage: posse attach <name>"},
		{[]string{"recipe", "--help"}, 0, "usage: posse recipe <name>"},
		{[]string{"prompt", "--help"}, 0, `usage: posse prompt <name> "<text>"`},
		{[]string{"relaunch", "--help"}, 0, "usage: posse relaunch <name>"},
		{[]string{"crew", "--help"}, 0, "usage: posse crew <name>"},
	} {
		cmd := exec.Command(bin, c.args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("posse %s: %v", strings.Join(c.args, " "), err)
		}
		if code != c.code || !strings.Contains(string(out), c.want) {
			t.Errorf("posse %s: exit %d, output %q; want exit %d containing %q",
				strings.Join(c.args, " "), code, out, c.code, c.want)
		}
	}

	var left []string
	filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			left = append(left, p)
		}
		return nil
	})
	if len(left) > 0 {
		t.Errorf("help and a refused name must leave no state behind: %q", left)
	}
}

func buildRhq(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}
