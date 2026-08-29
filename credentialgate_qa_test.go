package posse

// QA pins for rangerhq-fl0h — the sentence ADR 0019 D3 puts in NOTES.md:
//
//	Any code path that reads secrets/, envs/, or the keychain ships only
//	through the operator's `make install` review.
//
// That is a policy sentence, and the half of it a test can hold is the half
// that decides whether the policy is even reachable: **where the credential
// code lives**. A read that is compiled into the binary is behind the
// promotion gate by construction — the operator installs a binary or the
// fleet does not run it. A read that lives in a shipped *script* is not
// behind anything: personas write `scripts/`, and running one takes no
// install, no review, and no version bump. One such line would make the
// NOTES sentence false without changing a word of it, which is the failure
// mode this pins (the same one metakeys_qa_test.go was cut for: a fact about
// the binary written down instead of pinned).
//
// Two claims, both mutation-checked:
//
//   - no shipped runnable file execs the keychain read or reads a credential
//     store's contents (positive witness: the scan must see the ~19 runnable
//     files that are there, and TestCredentialGateScannerCatchesEachShape
//     plants one file per shape to prove the matchers are not asleep);
//   - the keychain read is exec'd from exactly ONE non-test file of the
//     binary, `internal/rhq/credential.go` — the seam NOTES names. Exactly
//     one, not at-most-one: if the exec disappears or is spelled across
//     lines, this fails and the sentence needs re-deriving rather than
//     silently covering nothing.
//
// Out of scope on purpose: `scripts/verify-credential-paths.sh` NAMES
// `.credentials.json*` — it is the detective control for that path
// (ranger-base-m6cm) and looks a file up by name without ever reading it.
// The runtime-store matcher therefore requires a read verb, not the name.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The three shapes of "this file acquires a credential".
var (
	// The macOS keychain read itself. A shipped script has no honest reason
	// to name it; the binary's one caller is pinned separately below.
	cgKeychain = regexp.MustCompile(`find-generic-password`)
	// A credential store file, either class. Naming one in a script is
	// already the finding — there is no read-only use of envs/x.env.
	cgStoreFile = regexp.MustCompile(`(envs|secrets)/[A-Za-z0-9_.-]*\.env`)
	// The runtime's own stores (path 3). Here the NAME is legitimate (the
	// detective control looks for the file), so this wants a read verb.
	cgRuntimeRead = regexp.MustCompile(`\b(cat|jq|grep|awk|sed|head|tail|cut|tr|python3?|perl|ruby|source)\b[^\n]*(\.credentials\.json|auth\.json)`)
)

// cgRunnable reports whether p is a file the fleet can RUN without an
// install: a shell script by extension, anything with a shebang, or a
// Makefile. Documentation under etc/ is not runnable and is not scanned —
// prose that quotes a credential command is prose.
func cgRunnable(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".sh", ".bash", ".zsh":
		return true
	}
	if filepath.Base(p) == "Makefile" {
		return true
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return strings.HasPrefix(string(b), "#!")
}

// cgScan walks root's shipped-runnable files and returns one finding per
// offending line, plus the number of files it actually opened. The count is
// the positive witness: an assertion of pure ABSENCE is satisfied by a scan
// that measured nothing.
func cgScan(t *testing.T, root string, dirs ...string) (findings []string, scanned int) {
	t.Helper()
	for _, d := range dirs {
		p := filepath.Join(root, d)
		err := filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() || !cgRunnable(path) {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			scanned++
			rel, _ := filepath.Rel(root, path)
			for i, ln := range strings.Split(string(b), "\n") {
				var why string
				switch {
				case cgKeychain.MatchString(ln):
					why = "keychain read"
				case cgStoreFile.MatchString(ln):
					why = "credential store file"
				case cgRuntimeRead.MatchString(ln):
					why = "runtime credential store read"
				default:
					continue
				}
				findings = append(findings, rel+":"+strconv.Itoa(i+1)+": "+why+": "+strings.TrimSpace(ln))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", p, err)
		}
	}
	return findings, scanned
}

// The claim: everything that touches a credential store is compiled, so the
// operator's `make install` really is the gate NOTES.md says it is.
func TestNoShippedScriptAcquiresACredential(t *testing.T) {
	findings, scanned := cgScan(t, ".", "scripts", "plugin", "etc")
	// Makefile is a runnable at the root, scanned on its own.
	mk, mkn := cgScan(t, ".", "Makefile")
	findings, scanned = append(findings, mk...), scanned+mkn
	if scanned < 15 {
		t.Fatalf("scanned only %d runnable files — the walk measured nothing, so "+
			"a clean result here is not evidence (expected ~19 under scripts/, "+
			"plugin/, etc/ plus the Makefile)", scanned)
	}
	if len(findings) > 0 {
		t.Errorf("a shipped runnable file acquires a credential — that path has NO\n"+
			"promotion gate in front of it, and NOTES.md \"Env sets and secrets\"\n"+
			"tells the operator every credential read ships through `make install`\n"+
			"(ADR 0019 D3). Move the read into the binary, behind ReadCredential:\n  %s",
			strings.Join(findings, "\n  "))
	}
}

// The control for the test above: each matcher must fire on the shape it is
// named for. Without this, all three could be typo'd dead and the scan would
// stay green forever.
func TestCredentialGateScannerCatchesEachShape(t *testing.T) {
	dir := t.TempDir()
	plant := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(plant, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, body, want string }{
		{"keychain.sh", "#!/bin/sh\ntok=$(security find-generic-password -s 'Claude Code-credentials' -w)\n", "keychain read"},
		{"envset.sh", "#!/bin/sh\n. \"$RHQ_HOME/envs/container.env\"\n", "credential store file"},
		{"harness.sh", "#!/bin/sh\ngrep KEY \"$RHQ_HOME/secrets/plan-guard.env\"\n", "credential store file"},
		{"runtime.sh", "#!/bin/sh\njq -r .accessToken ~/.claude/.credentials.json\n", "runtime credential store read"},
	}
	for _, c := range cases {
		if err := os.WriteFile(filepath.Join(plant, c.name), []byte(c.body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A file that names the runtime store WITHOUT reading it — what
	// scripts/verify-credential-paths.sh does — must not be a finding.
	if err := os.WriteFile(filepath.Join(plant, "detect.sh"),
		[]byte("#!/bin/sh\nfind \"$d\" -maxdepth 1 -name '.credentials.json*' -print\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Prose is not runnable, even when it quotes the command verbatim.
	if err := os.WriteFile(filepath.Join(plant, "notes.md"),
		[]byte("security find-generic-password -s 'Claude Code-credentials' -w\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, scanned := cgScan(t, dir, "scripts")
	if scanned != len(cases)+1 {
		t.Fatalf("scanned %d files, want %d — the runnable filter is wrong "+
			"(notes.md must be skipped, the five scripts must not be)", scanned, len(cases)+1)
	}
	joined := strings.Join(findings, "\n")
	for _, c := range cases {
		if !strings.Contains(joined, c.name+":") || !strings.Contains(joined, c.want) {
			t.Errorf("planted %s and the scanner did not report %q — the matcher for that\n"+
				"shape is asleep, which makes TestNoShippedScriptAcquiresACredential\n"+
				"green over nothing.\nfindings:\n%s", c.name, c.want, joined)
		}
	}
	for _, f := range findings {
		if strings.HasPrefix(f, "detect.sh") || strings.Contains(f, "notes.md") {
			t.Errorf("false positive: %s\nnaming .credentials.json* to LOOK for it is the "+
				"detective control (ranger-base-m6cm), not a credential read", f)
		}
	}
}

// The other half: inside the binary the keychain read is one seam, and NOTES
// names the file. `exec.Command` + the item flag on one line is the read
// itself; a comment that merely mentions it (gates.go's header does) is not.
func TestKeychainReadIsExecdFromExactlyOneFile(t *testing.T) {
	const seam = "internal/rhq/credential.go"
	var hits []string
	var goFiles int
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			goFiles++
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			for _, ln := range strings.Split(string(b), "\n") {
				if strings.Contains(ln, "exec.Command") && cgKeychain.MatchString(ln) {
					hits = append(hits, filepath.ToSlash(path))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if goFiles < 50 {
		t.Fatalf("scanned only %d non-test .go files — the walk measured nothing", goFiles)
	}
	if len(hits) != 1 || hits[0] != seam {
		t.Errorf("the keychain read is exec'd from %v, want exactly [%s].\n"+
			"NOTES.md \"Env sets and secrets\" tells the operator the guard reaches the\n"+
			"keychain through ONE seam (ReadCredential). A second caller means the\n"+
			"sentence, and ADR 0019 D3's wall, need re-deriving; zero means the exec\n"+
			"moved or changed shape and this pin now covers nothing.", hits, seam)
	}
}
