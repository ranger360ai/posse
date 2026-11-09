//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-hw18: the read half ranger-base-9fl explicitly set aside.
// SeatbeltProfile renders `(deny file-write*)` and nothing else — no session
// below the container tier had a file-read wall at all, so any same-uid
// persona could read another runtime's credential, or the darwin
// `~/.claude/.credentials.json` keychain fallback ADR 0019 D2 store 3 names
// (ranger-base-xjj9, ranger-base-m6cm).
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
// credentialReadDenyLiterals is fixed (claude, then codex, then grok, then
// bob), so the expected slices below can be written in that order rather
// than compared as a set.
func crdWant(items ...string) []string { return items }

// The selection logic, every branch provable from this box: goos is a
// parameter and not `runtime.GOOS` read inside the function, so the branch
// a linux box would take is exercised here too.
func TestCredentialReadDenyLiteralsIsRuntimeAwareAndGOOSShaped(t *testing.T) {
	const home = "/Users/probe"
	claudeFile := filepath.Join(home, ".claude", ".credentials.json")
	codexFile := filepath.Join(home, ".codex", "auth.json")
	grokFile := filepath.Join(home, ".grok", "auth.json")
	// ranger-base-prjck: bob's store is NESTED under its state dir and is
	// not named auth.json — `~/.bob/settings/auth-secrets.json`, the fixed
	// path bobshell 2.0.5's bundle joins (bobHomeIn's own measurement), and
	// RE-MEASURED unchanged off bob 2.0.9's, 2026-10-31 (ranger-base-kf9ou).
	// No variable to unset beside it: bob declares no home override.
	bobFile := filepath.Join(home, ".bob", "settings", "auth-secrets.json")
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
	// ranger-base-x5cbz: and the same for the two siblings, which now
	// follow codexHomeIn / grokHomeIn. Either variable set on the box
	// running the suite moves that half of the answer off the home
	// spelling this table states, and the moved arms are their own pin
	// (credentialdenyclihome_qa_test.go).
	unsetenvForTest(t, "CODEX_HOME")
	unsetenvForTest(t, "GROK_HOME")

	for _, tc := range []struct {
		name      string
		goos      string
		stateDirs []string
		want      []string
	}{
		{"darwin, no runtime declared: all four unowned", "darwin", nil,
			crdWant(claudeFile, codexFile, grokFile, bobFile)},
		{"darwin, claude launching: claude's own file is STILL denied — the keychain is the store of record there, not this file (ADR 0019 D2)", "darwin",
			[]string{"~/.claude", "~/.claude.json"}, crdWant(claudeFile, codexFile, grokFile, bobFile)},
		{"darwin, codex launching: codex's own file is spared — it is not this session's business to deny its own credential", "darwin",
			[]string{"~/.codex"}, crdWant(claudeFile, grokFile, bobFile)},
		{"darwin, grok launching: grok's own file is spared, same reason", "darwin",
			[]string{"~/.grok"}, crdWant(claudeFile, codexFile, bobFile)},
		// ranger-base-prjck: own() keys on the declared state_dir literal,
		// and bob's (ADR 0060 D1, runtime.go) is `~/.bob` — the spelling
		// the sibling line passes. A bob-launched session denied bob's own
		// token store could not authenticate itself, hw18's rule.
		{"darwin, bob launching: bob's own store spared, same reason", "darwin",
			[]string{"~/.bob"}, crdWant(claudeFile, codexFile, grokFile)},
		// ADR 0080 D19 moved these three. On linux claude's file is the
		// store of record (ADR 0019 D2), which is a reason to spare the seat
		// that OWNS it and was read for two beads as a reason to open it to
		// every seat: before D19 a linux codex, grok or bob session could
		// read claude's credential. The rule there is now the siblings' own
		// — denied to every seat but its runtime's — so the GOOS answer is
		// which seats, not whether.
		{"linux, no runtime declared: claude's file is denied like every other unowned store (ADR 0080 D19)", "linux", nil,
			crdWant(claudeFile, codexFile, grokFile, bobFile)},
		{"linux, grok launching: grok's own file spared, claude's denied — it is not this session's business either", "linux",
			[]string{"~/.grok"}, crdWant(claudeFile, codexFile, bobFile)},
		// The arm that keeps the one above honest, and the half that must
		// NEVER widen: on linux the file claude authenticates with is
		// claude's own, it sits inside the config directory D19 grants that
		// seat whole, and a runtime denied its own credential cannot log in.
		// Spared by own(), keyed to the declared state_dir literal.
		{"linux, claude launching: claude's own store spared — it is the store of record there and this seat owns it", "linux",
			[]string{"~/.claude", "~/.claude.json"}, crdWant(codexFile, grokFile, bobFile)},
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
	// One literal per runtime this ordinary (no stateDirs) project owns
	// none of. The RENDER level is asserted here and not only under
	// sandbox-exec because the kernel probe below SKIPS inside a caged
	// session (ranger-base-xjw9: a seatbelt session may not nest a
	// sandbox_apply), which is exactly where a posse session runs — so
	// without this arm a new literal could reach main witnessed by nothing
	// on the box that landed it (credentialdenymove_qa_test.go takes the
	// same stance for the moved-file half).
	for _, want := range []string{
		absResolve(ExpandTilde("~/.claude/.credentials.json")),
		absResolve(ExpandTilde("~/.codex/auth.json")),
		absResolve(ExpandTilde("~/.grok/auth.json")),
		absResolve(ExpandTilde("~/.bob/settings/auth-secrets.json")),
	} {
		if !strings.Contains(prof, `(literal `+sbQuote(want)) {
			t.Errorf("rendered profile does not name %s:\n%s", want, prof)
		}
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
	// ranger-base-prjck: bob's store, and a non-credential file in the SAME
	// directory. The nested file name is the reason the second one is here
	// — `~/.bob/settings/` holds settings.json beside auth-secrets.json, so
	// a deny that widened into the directory would be invisible on the
	// claude fixture above (whose ~/.claude control sits one level up).
	bobSettings := sbMkdir(t, filepath.Join(home, ".bob", "settings"))
	bobAuth := filepath.Join(bobSettings, "auth-secrets.json")
	sbWrite(t, bobAuth, `{"bob.auth.tokens-https://example.invalid":"x"}`)
	bobOther := filepath.Join(bobSettings, "settings.json")
	sbWrite(t, bobOther, `{}`)

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

	if crdRun(t, walled, bobAuth) {
		t.Errorf("reading %s was ALLOWED under the carve-out — bob's credential is not this (claude) session's business (ranger-base-9k9ff, measured readable from a caged seat before ranger-base-prjck)", bobAuth)
	}
	if !crdRun(t, control, bobAuth) {
		t.Fatal("the CONTROL refused bob's auth-secrets.json too — the probe proves nothing about the deny")
	}
	if !crdRun(t, walled, bobOther) {
		t.Errorf("reading %s (not a credential literal) was refused under the carve-out — the deny widened into ~/.bob/settings, the directory the store sits in, rather than naming the file", bobOther)
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

// ─── ownership is CONTAINMENT, not the spelling of a declaration ─────────────

// ranger-base-ppnds, finding 2 of the verify at ranger-base-vnp01.
//
// own() decided whether a credential store belongs to THIS seat's runtime by
// string equality against the declared `state_dir:` literal. `state_dir:`
// REPLACES the built-in list rather than merging (runtime.go, ADR 0021 D1),
// so what arrives is whatever this box's operator typed — and ADR 0080 D19
// newly subjects claude to that test off darwin, where the file is the store
// of record and not a keychain fallback (ADR 0019 D2). MEASURED on the linux
// arm before the fix: `[<home>/.claude]` (the same directory, spelled
// absolutely) and `["~/.claude/"]` (one trailing slash) both read as "not a
// claude seat" and masked the seat off the credential it authenticates with
// — the one thing credentialReadDenyLiterals' own comment says a runtime must
// never be denied.
//
// The question the wall is about is containment: does any file of this store
// lie under a directory the launch declared. underDir answers it, resolves
// both sides, and is what every other containment question in that file
// already uses.
//
// The DARWIN column is deliberately absent from these rows: there claude's
// own file is denied to its own seat whatever is declared (the keychain is
// the store of record), so darwin can say nothing about ownership. The rows
// that pin the darwin posture are the table above and
// credentialpanesplit_qa_test.go's.
func TestQACredentialReadDenyOwnershipIsContainmentNotSpelling(t *testing.T) {
	home := t.TempDir()
	cfg := t.TempDir()
	homeCreds := cdmCreds(filepath.Join(home, ".claude"))
	cfgCreds := cdmCreds(cfg)
	codexFile := filepath.Join(home, ".codex", "auth.json")

	for _, tc := range []struct {
		name      string
		stateDirs []string
		cfgVar    string   // CLAUDE_CONFIG_DIR, or cdmUnset
		spared    []string // must NOT be in the deny list
		denied    []string // must be
	}{
		// The control, and the shape every box here actually runs: the
		// built-in declaration, spared before this change and after it.
		{"the built-in declaration", []string{"~/.claude", "~/.claude.json"}, cdmUnset,
			[]string{homeCreds}, []string{codexFile}},
		// The two spellings that stranded the seat. Same directory, same
		// credential, same runtime.
		{"the same directory, spelled absolutely", []string{filepath.Join(home, ".claude")}, cdmUnset,
			[]string{homeCreds}, []string{codexFile}},
		{"the same directory, one trailing slash", []string{"~/.claude/"}, cdmUnset,
			[]string{homeCreds}, []string{codexFile}},
		// ADR 0080 D19's own shape, and what takes the sharp edge off the
		// coupling ranger-base-tc41o pinned: with the config dir declared and
		// the `~/.claude` tree NOT, the seat is still read as claude's — and
		// is spared BOTH spellings, because ownership is a fact about the
		// runtime and the home copy is whatever that runtime wrote before the
		// variable moved the write (ADR 0019 D2 store 3).
		{"the config directory alone, with the tree dropped", []string{cfg}, cfg,
			[]string{cfgCreds, homeCreds}, []string{codexFile}},
		// The sibling half, which inherited the same literal coupling and is
		// closed by the same line: a codex seat declaring its own store
		// absolutely is spared it.
		{"a codex seat, declared absolutely", []string{filepath.Join(home, ".codex")}, cdmUnset,
			[]string{codexFile}, []string{homeCreds}},
		// An ADR 0021 D1 overlay that moves the declared state somewhere the
		// credential is NOT. Still denied, and that is the answer rather than
		// a residual: SeatbeltWritable grants the declared state dirs and
		// nothing else, so on this declaration nothing grants `~/.claude`
		// either — the wall and the grant agree about which directory is this
		// seat's state. An operator who moves claude's state moves
		// CLAUDE_CONFIG_DIR with it, which is the row above.
		{"an overlay that does not contain the credential", []string{"/srv/claude-state"}, cdmUnset,
			nil, []string{homeCreds, codexFile}},
		// And the fail-closed control: a caller with no runtime in hand owns
		// nothing, which is SeatbeltWritable's answer to the same question.
		{"no runtime declared", nil, cdmUnset, nil, []string{homeCreds, codexFile}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cdmEnv(t, home, cdmUnset, tc.cfgVar)
			got := credentialReadDenyLiterals("linux", tc.stateDirs)
			for _, p := range tc.spared {
				if crdHas(got, p) {
					t.Errorf("stateDirs=%v denies %s — that store is this seat's own, and a runtime denied its own credential cannot authenticate itself (on linux it is the store of record, ADR 0019 D2). deny = %v", tc.stateDirs, p, got)
				}
			}
			for _, p := range tc.denied {
				if !crdHas(got, p) {
					t.Errorf("stateDirs=%v does not deny %s — without this the row above is satisfied by a function that denies nothing at all. deny = %v", tc.stateDirs, p, got)
				}
			}
		})
	}
}

func crdHas(list []string, p string) bool {
	for _, q := range list {
		if q == p {
			return true
		}
	}
	return false
}
