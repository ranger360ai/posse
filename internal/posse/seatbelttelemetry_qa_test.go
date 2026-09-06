//go:build posse_arm2

package posse

// ranger-base-gr3ow: the Go toolchain's local telemetry counters were 18% of
// one day's sandbox-denial volume — `go`, `compile`, `link` and `asm` each
// mmap a counter file under ~/Library/Application Support/go/telemetry/local
// on every invocation, and under `cage: seatbelt` every one of those writes
// was refused: ~2,300/day across two caged sessions, measured 2026-09-01 off
// an uncaged `log show`. The build succeeds either way, so this is noise and
// not breakage — but it is noise that triples as personas join the cage, and
// it is the kind that hides a denial anyone would want to see.
//
// The bead proposed exporting GOTELEMETRY=off in the caged launch env
// instead. That variable does not exist as an input: it is a DERIVED,
// non-settable `go env` value read out of the telemetry mode file, `go env
// -w GOTELEMETRY=off` is refused by name, and nothing in x/telemetry ever
// calls os.Getenv on it. Measured on go1.26.5 — a build with GOTELEMETRY=off
// in the environment writes exactly the four counter files a build without
// it writes. The only lever that stops the writes is the mode file, which is
// account-global and hand-applied; the grant below is the versioned one.
//
// The grant is `local` and NOT its parent, and that narrowing is the whole
// security content of this file: `telemetry/mode` sits beside `local/` and
// is the single switch that decides whether the toolchain UPLOADS those
// counters to telemetry.go.dev. A session that could write it could turn on
// an egress the operator never chose (crew guardrail 4). So the pins below
// run BOTH ways — the counter write is allowed, the mode switch is refused —
// because a one-way pin here goes green on a grant of the whole subpath.

import (
	"os"
	"path/filepath"
	"testing"
)

// goTelemetryLocal is the directory the four toolchain binaries write their
// counter files into, spelled from the same home SeatbeltWritable reads.
func goTelemetryLocal(home string) string {
	return filepath.Join(home, "Library", "Application Support", "go", "telemetry", "local")
}

// The writable set reaches the counters and stops short of the switch. Pure
// set arithmetic, no sandbox: this is the half that fails on any box.
func TestSeatbeltWritableReachesGoTelemetryCountersAndNotTheModeSwitch(t *testing.T) {
	root := sbRoot(t)
	// The real home would make every path below an operator-owned file, not
	// a fixture — sbRoot swaps HOME and now refuses to continue if the swap
	// did not take (sbAssertHomeIsAFixture, ranger-base-g8ezm).
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	work := sbMkdir(t, filepath.Join(root, "work"))
	a := NewAppAt(filepath.Join(root, "posse-home"))
	gates := sbMkdir(t, a.GatesDir("developer"))
	ag := &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))}
	w := a.SeatbeltWritable(ag, work, gates)

	local := goTelemetryLocal(home)
	telDir := filepath.Dir(local)
	for _, tc := range []struct {
		what string
		p    string
		want bool
	}{
		{"a counter file the toolchain writes on every invocation", filepath.Join(local, "go@go1.26.5-darwin-arm64-2026-09-02.v1.count"), true},
		{"the local counter directory itself", local, true},
		{"the mode file that decides whether counters are UPLOADED", filepath.Join(telDir, "mode"), false},
		{"the telemetry directory the mode file sits in", telDir, false},
		{"the upload staging directory", filepath.Join(telDir, "upload"), false},
		{"an unrelated app's Application Support tree", filepath.Join(home, "Library", "Application Support", "SomeOtherApp"), false},
		{"Application Support itself", filepath.Join(home, "Library", "Application Support"), false},
	} {
		if got := sbCovers(w, absResolve(tc.p)); got != tc.want {
			t.Errorf("%s\n  %s\n  covered by the writable set: got %v, want %v\n  set: %v", tc.what, tc.p, got, tc.want, w)
		}
	}
}

// The same claim through the rendered profile and a real kernel, with the
// control that makes the refusals mean something: the SAME probe under the
// SAME profile minus the telemetry grant must be refused. Without that arm a
// typo'd grant path "passes" the mode half forever and fails nothing
// (ranger-base-h15's rule, and the reason the count write is measured too).
func TestQAGoTelemetryCounterWriteIsAllowedAndTheModeSwitchIsRefused(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	root := sbRoot(t)
	// The real home would make every path below an operator-owned file, not
	// a fixture — sbRoot swaps HOME and now refuses to continue if the swap
	// did not take (sbAssertHomeIsAFixture, ranger-base-g8ezm).
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	work := sbMkdir(t, filepath.Join(root, "work"))
	a := NewAppAt(filepath.Join(root, "posse-home"))
	gates := sbMkdir(t, a.GatesDir("developer"))
	ag := &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))}

	local := sbMkdir(t, goTelemetryLocal(home))
	telDir := filepath.Dir(local)
	countFile := filepath.Join(local, "probe.v1.count")
	modeFile := filepath.Join(telDir, "mode")

	w := a.SeatbeltWritable(ag, work, gates)
	prof := sbRenderProfile(t, "telemetry.sb", SeatbeltProfile("developer", w, nil, a.SeatbeltCarveOut(ag, work, gates, w)))

	// The control: the identical set with the telemetry grant taken out, so
	// the wall is otherwise the same wall. If this arm ALLOWS the count
	// write, nothing below is measuring the grant.
	var without []string
	resolvedLocal := absResolve(local)
	for _, g := range w {
		if g != resolvedLocal {
			without = append(without, g)
		}
	}
	if len(without) == len(w) {
		t.Fatalf("control arm removed nothing: the writable set does not name %s as its own entry, so this test would grade a grant it never held\n  set: %v", resolvedLocal, w)
	}
	ctrl := sbRenderProfile(t, "control.sb", SeatbeltProfile("developer", without, nil, a.SeatbeltCarveOut(ag, work, gates, without)))

	if sbRun(t, ctrl, "echo x > "+shellQuote(countFile)) {
		t.Fatal("CONTROL FAILED: the counter write is allowed with the telemetry grant removed — something else in the profile already reaches it, so the pins below grade nothing")
	}
	os.Remove(countFile)
	if !sbRun(t, prof, "echo x > "+shellQuote(countFile)) {
		t.Errorf("the toolchain's counter write at %s is still refused under the rendered profile — the ~2,300/day denials ranger-base-gr3ow measured are not silenced", countFile)
	}
	os.Remove(countFile)
	// A sentinel at the mode path, for two reasons. It makes the probe the
	// faithful one: on the box this pin is about — one that ran `go
	// telemetry off`, which is what gr3ow's close comment offers the
	// operator — the file EXISTS, and overwriting a file is a different
	// operation from creating one. And it gives the next assertion
	// something to be about. The probed VALUE stays `on`, the dangerous
	// one, deliberately: a probe that wrote some inert string would go
	// green on a wall that only refused the word.
	const sentinel = "off 2026-09-02\n"
	sbWrite(t, modeFile, sentinel)

	if sbRun(t, prof, "echo on > "+shellQuote(modeFile)) {
		t.Errorf("a session can write %s — that is the switch that turns Go telemetry UPLOADS on for the operator's box; the grant must stop at local/ (crew guardrail 4)", modeFile)
	}

	// ranger-base-g8ezm: the pin leaves the mode file exactly as it found
	// it. This line used to be `os.Remove(modeFile)` — an unconditional
	// delete, in the test process, outside the sandbox, on the passing
	// path. Against the fixture home sbRoot guarantees, that deleted
	// nothing anyone owned — which is why the bug report filed against it
	// does not reproduce at HEAD. It is a pin rather than a deletion
	// because the two halves are what make it stay that way:
	// sbAssertHomeIsAFixture says the file is never the operator's, and
	// this says the pin does not destroy the file it was handed either.
	got, err := os.ReadFile(modeFile)
	if err != nil || string(got) != sentinel {
		t.Errorf("this pin did not leave %s as it found it: read %q, %v — want the sentinel %q back untouched.\n  on an uncaged box with a real home that file is the operator's `go telemetry off` setting, and losing it silently returns the toolchain to `local`", modeFile, got, err, sentinel)
	}
}
