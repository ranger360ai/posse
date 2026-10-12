package posse

// QA pins for ranger-base-x0xct / ADR 0073 B2 — RunGatesBdWrite itself,
// offline: no shell, no subprocess. A PASSING call ends in syscall.Exec,
// which would replace this test binary, so every case here is one that
// returns BEFORE that — a usage error or a refusal (the lock, or the line
// cliff). The "it actually execs bd" half is bdwriteshim_test.go, which
// runs the rendered shim and a real posse as subprocesses, never this
// process itself.

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQABdWriteSplitArgv(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		real string
		rest []string
		ok   bool
	}{
		{"usage shape", []string{"--real", "/usr/bin/bd", "--", "close", "t-1"}, "/usr/bin/bd", []string{"close", "t-1"}, true},
		{"empty bd argv is still valid", []string{"--real", "/usr/bin/bd", "--"}, "/usr/bin/bd", []string{}, true},
		{"missing --real", []string{"close", "t-1"}, "", nil, false},
		{"empty --real value", []string{"--real", "", "--", "close"}, "", nil, false},
		{"no --", []string{"--real", "/usr/bin/bd", "close", "t-1"}, "", nil, false},
		{"nil args", nil, "", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			real, rest, ok := bdWriteSplitArgv(c.args)
			if ok != c.ok || real != c.real || (c.ok && !stringSlicesEqual(rest, c.rest)) {
				t.Errorf("bdWriteSplitArgv(%v) = (%q, %v, %v), want (%q, %v, %v)", c.args, real, rest, ok, c.real, c.rest, c.ok)
			}
		})
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestQARunGatesBdWriteUsageNeedsReal(t *testing.T) {
	t.Parallel()
	if _, err := RunGatesBdWrite([]string{"--", "close", "t-1"}, &bytes.Buffer{}); err == nil {
		t.Fatal("missing --real must be an error, not a silent pass")
	}
}

func TestQARunGatesBdWriteRefusesACrossingComment(t *testing.T) {
	census := writeCensus(t, bdRow("t-1", "small"), bdRow("t-2", "small too"))
	home := filepath.Dir(census)
	t.Setenv("BEADS_DIR", home)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("RHQ_PERSONA", "security")
	logPath := filepath.Join(t.TempDir(), "refusals.log")
	t.Setenv("RHQ_GATE_LOG", logPath)

	text := strings.Repeat("x", 70000)
	var errb bytes.Buffer
	refused, err := RunGatesBdWrite([]string{"--real", "/bin/true", "--", "comments", "add", "t-1", text}, &errb)
	if err != nil || !refused {
		t.Fatalf("a 70000-byte comment: refused=%v err=%v, want true/nil", refused, err)
	}
	out := errb.String()
	if !strings.Contains(out, "refused by posse gate:") || !strings.Contains(out, "t-1") || !strings.Contains(out, "ADR 0072") {
		t.Errorf("refusal text missing the D2 shape:\n%s", out)
	}
}

// A write the line-cliff judge never projects (ProjectBdWrite only knows
// comments/update/create/close) must still be refused on a held lock
// rather than falling through to bd unjudged and unlocked. runGatesBdWrite
// (not the exported RunGatesBdWrite) takes the short injected wait, for
// bdWriteLock's own reason (bdwritelock_test.go): the suite must not spend
// BdWriteLockWait's real 30s to prove the timeout arm.
func TestQARunGatesBdWriteRefusesOnAHeldLock(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	store := t.TempDir()
	t.Setenv("BEADS_DIR", store)

	held, err := bdWriteLock(bdWriteStore(""), []string{"close"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if held == nil {
		t.Fatal("the control lock degraded to unlocked — check XDG_CACHE_HOME wiring")
	}
	defer held.Release()

	var errb bytes.Buffer
	wait := 150 * time.Millisecond
	refused, rerr := runGatesBdWrite([]string{"--real", "/bin/true", "--", "dep", "add", "t-1", "t-2"}, &errb, wait)
	if rerr != nil || !refused {
		t.Fatalf("a write against a held store lock: refused=%v err=%v, want true/nil", refused, rerr)
	}
	if want := "bd write lock was held for " + wait.String(); !strings.Contains(errb.String(), want) {
		t.Errorf("refusal text missing the D1 timeout sentence:\ngot  %q\nwant substring %q", errb.String(), want)
	}
}
