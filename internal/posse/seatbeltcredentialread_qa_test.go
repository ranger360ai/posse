package posse

// ranger-base-hw18: the read half ranger-base-9fl explicitly set aside.
// SeatbeltProfile renders `(deny file-write*)` and nothing else — no session
// below the container tier had a file-read wall at all, so any same-uid
// persona could read another runtime's credential, or the recurring unowned
// `~/.claude/.credentials.json` byproduct ADR 0019 D2 names (ranger-base-xjj9,
// ranger-base-m6cm).
//
// This file pins two things separately, per the bead's own "VERIFY BEFORE
// SHIPPING" section: the pure runtime-aware/GOOS-shaped selection logic
// (no sandbox needed, every branch provable from one box — credential.go's
// meterStore made the same call, for the same reason), and the deny
// actually refusing a real read under sandbox-exec, with a non-denied read
// in the same directory as the control that proves the wall did not widen
// past what it named.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// crdWant is a readability helper: the append order in
// credentialReadDenyLiterals is fixed (claude, then codex, then grok), so
// the expected slices below can be written in that order rather than
// compared as a set.
func crdWant(items ...string) []string { return items }

// The selection logic, every branch provable from this box: goos is a
// parameter and not `runtime.GOOS` read inside the function, so the branch
// a linux box would take is exercised here too.
func TestCredentialReadDenyLiteralsIsRuntimeAwareAndGOOSShaped(t *testing.T) {
	const home = "/Users/probe"
	claudeFile := filepath.Join(home, ".claude", ".credentials.json")
	codexFile := filepath.Join(home, ".codex", "auth.json")
	grokFile := filepath.Join(home, ".grok", "auth.json")
	t.Setenv("HOME", home)
	// ranger-base-x5f6p: the claude expectation below is the HOME spelling,
	// and the deny now also follows CLAUDE_SECURESTORAGE_CONFIG_DIR /
	// CLAUDE_CONFIG_DIR. Either one set on the box running the suite adds a
	// second literal to every darwin arm — so this table states the
	// no-variable case rather than inheriting whatever the operator
	// exported. The arms where they ARE set are their own pins
	// (credentialdenymove_qa_test.go).
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")

	for _, tc := range []struct {
		name      string
		goos      string
		stateDirs []string
		want      []string
	}{
		{"darwin, no runtime declared: all three unowned", "darwin", nil,
			crdWant(claudeFile, codexFile, grokFile)},
		{"darwin, claude launching: claude's own file is STILL denied — the keychain is the store of record there, not this file (ADR 0019 D2)", "darwin",
			[]string{"~/.claude", "~/.claude.json"}, crdWant(claudeFile, codexFile, grokFile)},
		{"darwin, codex launching: codex's own file is spared — it is not this session's business to deny its own credential", "darwin",
			[]string{"~/.codex"}, crdWant(claudeFile, grokFile)},
		{"darwin, grok launching: grok's own file is spared, same reason", "darwin",
			[]string{"~/.grok"}, crdWant(claudeFile, codexFile)},
		{"linux, no runtime declared: claude's file IS the store of record there (ADR 0019 D2) — must stay readable", "linux", nil,
			crdWant(codexFile, grokFile)},
		{"linux, grok launching: grok's own file spared, claude's stays open for the same GOOS reason", "linux",
			[]string{"~/.grok"}, crdWant(codexFile)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := credentialReadDenyLiterals(tc.goos, tc.stateDirs)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("goos=%s stateDirs=%v:\n got  %v\n want %v", tc.goos, tc.stateDirs, got, tc.want)
			}
		})
	}
}

// SeatbeltProfile's carve-out was Empty() (and rendered nothing) whenever
// Deny and Seal were both nil — DenyRead is a fourth list a PID with no
// path-scoped denies and no reach into the constitution still needs
// rendered, so Empty() and the write-block guard both had to learn about
// it. A pure struct literal, deliberately no fixture: what is under test is
// SeatbeltProfile's own gating, not SeatbeltCarveOut's construction.
func TestEmptyCarveOutIsNotEmptyWithOnlyDenyRead(t *testing.T) {
	t.Parallel()
	c := SeatbeltCarveOut{DenyRead: []string{"/some/credential/file"}}
	if c.Empty() {
		t.Fatal("Empty() reports true with a non-empty DenyRead — SeatbeltProfile would then render nothing and the deny would silently vanish")
	}
	prof := SeatbeltProfile("developer", nil, nil, c)
	if !strings.Contains(prof, "(deny file-read*") {
		t.Errorf("rendered profile carries no file-read* deny when Deny/Seal are both empty:\n%s", prof)
	}
	if strings.Contains(prof, "the carve-out (ranger-base-h15)") {
		t.Error("the write carve-out's own comment rendered with no Deny/Seal to justify it — Deny/Seal and DenyRead must stay independently gated")
	}
}

// The carve-out actually carries the literals through to a REAL App's
// rendered profile, not just a hand-built struct.
func TestCarveOutCarriesCredentialLiteralsIntoTheRenderedProfile(t *testing.T) {
	root := sbRoot(t)
	work := sbMkdir(t, filepath.Join(root, "work"))
	a := NewAppAt(filepath.Join(root, "posse-home"))
	gates := sbMkdir(t, a.GatesDir("developer"))
	ag := &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))}

	w := a.SeatbeltWritable(ag, work, gates)
	c := a.SeatbeltCarveOut(ag, work, gates, w)
	if len(c.DenyRead) == 0 {
		t.Fatal("SeatbeltCarveOut renders no credential read-deny for an ordinary project")
	}
	prof := SeatbeltProfile("developer", w, nil, c)
	if !strings.Contains(prof, "(deny file-read*") {
		t.Errorf("rendered profile carries no file-read* deny:\n%s", prof)
	}
	claudeCreds := absResolve(ExpandTilde("~/.claude/.credentials.json"))
	if !strings.Contains(prof, `(literal `+sbQuote(claudeCreds)) {
		t.Errorf("rendered profile does not name %s:\n%s", claudeCreds, prof)
	}
}

// crdRun renders profile p and reads path via sandbox-exec, reporting
// whether the read was ALLOWED (err == nil). Mirrors sbRun's shape but for
// a read instead of a write, with the same backstop: anything other than a
// clean allow/refuse must not be read as one.
func crdRun(t *testing.T, profile, path string) bool {
	t.Helper()
	sbSkipUnlessSandboxable(t)
	cmd := exec.Command("sandbox-exec", "-f", profile, "/bin/cat", path)
	out, err := cmd.CombinedOutput()
	if err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() > 1 {
		t.Fatalf("probe cat %q failed for a reason that is not the sandbox: %v %s", path, err, out)
	}
	return err == nil
}

// The bead's own VERIFY BEFORE SHIPPING, trap 2: a read of a denied
// credential literal is refused under the carve-out and allowed under the
// control (the same profile minus the credential read-deny — the write
// carve-out stays, so the control is not "no wall at all"), and a
// NON-denied read next to it stays allowed under both — the deny must not
// have widened into the directory it sits in. codex's auth.json is denied
// too: this fixture's launching runtime is claude, so codex's credential is
// never its business.
func TestQACredentialReadDenyRefusesUnderSandboxExecAndTheControlDoesNot(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	root := sbRoot(t)
	home := os.Getenv("HOME")
	claudeDir := sbMkdir(t, filepath.Join(home, ".claude"))
	sbWrite(t, filepath.Join(claudeDir, ".credentials.json"), `{"claudeAiOauth":"x"}`)
	sbWrite(t, filepath.Join(claudeDir, "settings.json"), `{}`)
	codexAuth := filepath.Join(sbMkdir(t, filepath.Join(home, ".codex")), "auth.json")
	sbWrite(t, codexAuth, `{"token":"x"}`)

	work := sbMkdir(t, filepath.Join(root, "work"))
	a := NewAppAt(filepath.Join(root, "posse-home"))
	gates := sbMkdir(t, a.GatesDir("developer"))
	ag := &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))}

	// The launching runtime's own state_dir declaration (ADR 0012 D4) —
	// claude's, so this fixture measures the "own file still denied on
	// darwin" case rather than the exception path.
	stateDirs := []string{"~/.claude", "~/.claude.json"}
	w := a.SeatbeltWritable(ag, work, gates, stateDirs...)
	withCarve := a.SeatbeltCarveOut(ag, work, gates, w, stateDirs...)
	if len(withCarve.DenyRead) == 0 {
		t.Fatal("fixture premise gone: no credential literals denied — nothing below measures anything")
	}
	// The control has to be the write carve-out ALONE: SeatbeltCarveOut
	// denies every credential literal it does not know to be the launching
	// runtime's own, even with no stateDirs at all (the "unknown runtime"
	// case defaults to denying, per the table test above) — so omitting
	// stateDirs here would not produce a DenyRead-free carve-out, it would
	// just deny the same three files a different way. Stripping the field
	// by hand is what actually isolates the credential deny as the one
	// variable between the two profiles.
	writeOnlyControl := withCarve
	writeOnlyControl.DenyRead = nil

	walled := sbRenderProfile(t, "walled.sb", SeatbeltProfile("developer", w, nil, withCarve))
	control := sbRenderProfile(t, "control.sb", SeatbeltProfile("developer", w, nil, writeOnlyControl))

	denied := filepath.Join(claudeDir, ".credentials.json")
	if crdRun(t, walled, denied) {
		t.Errorf("reading %s was ALLOWED under the carve-out", denied)
	}
	if !crdRun(t, control, denied) {
		t.Fatal("the CONTROL refused the read too — the probe proves nothing about the deny")
	}

	if other := filepath.Join(claudeDir, "settings.json"); !crdRun(t, walled, other) {
		t.Errorf("reading %s (not a credential literal) was refused under the carve-out — the deny widened past what it named", other)
	}

	if crdRun(t, walled, codexAuth) {
		t.Errorf("reading %s was ALLOWED under the carve-out — codex's credential is not this (claude) session's business", codexAuth)
	}
	if !crdRun(t, control, codexAuth) {
		t.Fatal("the CONTROL refused codex's auth.json too — the probe proves nothing about the deny")
	}
}

// The trap the bead names first: a caged darwin claude session must still
// be able to authenticate and do real work with the deny in force. This
// cannot run a live launch, but it pins the reasoning's premise — that a
// claude session never needs a file-read of its own credentials file to
// operate, because the harness hands it CLAUDE_CODE_OAUTH_TOKEN when caged
// (cageCredential) — by asserting the mapping is still what the deny's own
// doc comment claims. If this ever drifts (a second runtime added to
// cageCredential, or claude's entry removed) the deny's safety argument no
// longer holds and this is where that would be caught.
func TestQAClaudeCagedCredentialIsEnvNotFileRead(t *testing.T) {
	t.Parallel()
	rt := &Runtime{Name: "claude"}
	if got := CageCredential(rt); got != "CLAUDE_CODE_OAUTH_TOKEN" {
		t.Fatalf("CageCredential(claude) = %q, want CLAUDE_CODE_OAUTH_TOKEN — ranger-base-hw18's darwin exception for ~/.claude/.credentials.json assumes a caged claude session authenticates by this env var, not by reading the file", got)
	}
}
