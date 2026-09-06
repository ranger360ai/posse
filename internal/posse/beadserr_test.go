//go:build posse_arm2

package posse

// bd reports a --json verb's failure on STDOUT, not stderr (rangerhq-aas):
// `bd dep list <missing-id> --json` exits 1 having printed
// `{"error": "resolving …: no issue found matching …"}` with stderr empty
// (measured, bd 0.49.1). Bd.run built its message from stderr alone, so the
// operator got `bd dep list x --json: exit status 1` and lost the sentence
// that said why.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errRepo is a beads repo whose bd fails the given way: marker is the fake's
// opt-in file, body its contents.
func errRepo(t *testing.T, marker, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, marker), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBdRunKeepsTheReasonBdPrintsOnStdout(t *testing.T) {
	t.Parallel()
	newTestBackend(t) // RHQ_FAKE_HERDR, a temp HOME
	bd := Bd{Bin: fakeBinFor(t, "bd")}

	const reason = `resolving rangerhq-zzzz: no issue found matching "rangerhq-zzzz"`

	// The bug: the reason is on stdout and stderr is empty.
	t.Run("json error on stdout", func(t *testing.T) {
		dir := errRepo(t, "fake-json-error", reason)
		_, err := bd.DepList(dir, "rangerhq-zzzz")
		if err == nil {
			t.Fatal("a verb that exits 1 must return an error")
		}
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("the reason bd printed must survive, got %q", err.Error())
		}
		if strings.Contains(err.Error(), "exit status") {
			t.Errorf("with a reason in hand it must not fall back to the exit status: %q", err.Error())
		}
	})

	// The control that keeps the stdout read from taking over: stderr still
	// wins when bd uses it, which is every non-json verb and most failures.
	t.Run("stderr still wins", func(t *testing.T) {
		dir := errRepo(t, "fake-ready-fail", "")
		_, err := bd.Ready(dir, "")
		if err == nil {
			t.Fatal("a failed ready must return an error")
		}
		if !strings.Contains(err.Error(), "database is locked") {
			t.Errorf("stderr is still the message when bd writes one, got %q", err.Error())
		}
	})

	// And the arm that makes that control discriminate: with stdout empty
	// either precedence reads the same, so the fixture has to fill BOTH
	// channels. stderr is the diagnostic bd meant for a human; the stdout
	// object is the fallback for when it left that channel silent.
	t.Run("both channels: stderr is the message", func(t *testing.T) {
		dir := errRepo(t, "fake-json-error", reason)
		if err := os.WriteFile(filepath.Join(dir, "fake-json-error-stderr"),
			[]byte("Error: database is locked"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := bd.DepList(dir, "rangerhq-zzzz")
		if err == nil {
			t.Fatal("a verb that exits 1 must return an error")
		}
		if !strings.Contains(err.Error(), "database is locked") {
			t.Errorf("stderr outranks the stdout object, got %q", err.Error())
		}
		if strings.Contains(err.Error(), "no issue found") {
			t.Errorf("it must not append the stdout object to it: %q", err.Error())
		}
	})

	// The other control, and the reason the parse is narrow: a failure whose
	// stdout is NOT `{"error": …}` must not be quoted at the operator as if
	// it were a reason. The exit status is the honest answer there.
	t.Run("opaque stdout falls back", func(t *testing.T) {
		dir := errRepo(t, "fake-opaque-error", "")
		_, err := bd.DepList(dir, "a-1")
		if err == nil {
			t.Fatal("a verb that exits 1 must return an error")
		}
		if !strings.Contains(err.Error(), "exit status") {
			t.Errorf("with nothing readable on either channel the exit status is the message, got %q", err.Error())
		}
		if strings.Contains(err.Error(), "a-1") && strings.Contains(err.Error(), "title") {
			t.Errorf("it must not quote the payload back: %q", err.Error())
		}
	})
}

// The parse itself, over the shapes a half-broken bd can hand back.
func TestBdStdoutErrorReadsOnlyTheErrorObject(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, stdout, want string
	}{
		{"the shape bd prints", `{"error": "resolving x: no issue found"}`, "resolving x: no issue found"},
		{"trailing newline", "{\"error\": \"boom\"}\n", "boom"},
		{"a listing", `[{"id":"a-1"}]`, ""},
		{"an object with no error key", `{"id":"a-1"}`, ""},
		{"an empty error", `{"error": "  "}`, ""},
		{"nothing at all", "", ""},
		{"a truncated page", `{"error": "bo`, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := bdStdoutError([]byte(c.stdout)); got != c.want {
				t.Errorf("bdStdoutError(%q) = %q, want %q", c.stdout, got, c.want)
			}
		})
	}
}
