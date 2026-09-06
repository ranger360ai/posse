//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-r68d8: the seatbelt's credential read-deny is
// rendered from the LAUNCHER's environment, and the runtime that writes the
// credential does not read that environment.
//
// THE SPLIT, measured in the code rather than assumed: the profile is
// rendered in the posse process (herdrback.go, planLaunch), while the
// session is a herdr workspace — `CreateWorkspace` hands the pane only the
// vars posse names explicitly (`--env K=V`, herdr.go), the pane is a child
// of the herdr DAEMON, and the runtime is typed into that pane's LOGIN
// shell (`PaneRun`). ADR 0013 already records the same three-way split for
// PATH; nothing makes the two config-dir names an exception. So the
// launcher's environment and the runtime's can differ in either direction:
// an `export CLAUDE_SECURESTORAGE_CONFIG_DIR=…` in a login rc reaches the
// runtime and never reaches posse, and a `CLAUDE_CONFIG_DIR=… posse new`
// reaches posse and never reaches the pane. Either way the wall would name
// one directory and the write would land in another — the x5f6p failure one
// environment over.
//
// WHAT CLOSES IT is the pin, not inheritance (credentialDirPin,
// ranger-base-rq83c): the rendered launch line carries both names in its
// `--settings` payload, resolved in the launcher, and the runtime applies
// each settings scope's env block over process.env in the order user,
// project, local, flag, policy — so the flag-scope payload lands after the
// pane's environment. That measurement is the runtime's half and lives with
// the runtime (credentialdirpin_live_test.go, claude 2.1.259). What is
// pinned HERE is posse's half, and it is pinned FROM THE RENDERED LINE
// rather than from credentialDirPin directly: dropping the pin out of the
// settings payload, or the settings flag off the line, is the mutation
// these arms exist to catch, and a test that called the pin function would
// stay green through both.
//
// RESIDUALS, stated because they are the arms this coupling does NOT cover:
//
//   - a pane whose HOME differs from the launcher's. Where no variable
//     names the directory the pin's secure-storage value is the empty
//     string — the one spelling that leaves the keychain item unsuffixed
//     (ranger-base-ig4op) — and an empty value means `$HOME/.claude` to the
//     runtime, resolved against the PANE's home. Closing it would mean
//     pinning a non-empty directory, which renames the item out from under
//     the operator's login; posse sets no HOME for a pane, so the split is
//     not one posse opens.
//   - a root-owned OS `managed-settings.json`, which is applied after the
//     flag scope and therefore outranks the pin. The one installed on the
//     reference box (ranger-base-sn0w8) pins the home, which is the literal
//     `credentialFileCandidates` names unconditionally, so the wall still
//     covers the write there — but a policy file naming some OTHER
//     directory would move the write past this wall and no launcher flag
//     could stop it.
//   - a `command:` template spelling its own `--settings`, which
//     `EnsureSettingsPin` leaves alone. Unreachable for claude: a built-in's
//     `command:` is refused in its overlay (ADR 0021, runtime.go's
//     builtinOverlayRefused).

import (
	"encoding/json"
	"strings"
	"testing"
)

// settingsEnvFromLine reads the `env` block out of the `--settings` payload
// a rendered launch line carries. The payload is shell-quoted by
// agents.go's shellQuote, so the word runs to the first `'` that is not
// part of the four-character escape that function spells an embedded quote
// with. Unquoted here rather than compared as a substring, because what
// this file asserts is the VALUE the runtime will read and not the
// presence of a name.
func settingsEnvFromLine(t *testing.T, line string) map[string]string {
	t.Helper()
	const flag = "--settings '"
	i := strings.Index(line, flag)
	if i < 0 {
		t.Fatalf("the rendered line carries no --settings payload, so nothing pins the config dir against the pane's own environment:\n%s", line)
	}
	rest := line[i+len(flag):]
	var payload strings.Builder
	for {
		j := strings.IndexByte(rest, '\'')
		if j < 0 {
			t.Fatalf("unterminated --settings payload:\n%s", line)
		}
		payload.WriteString(rest[:j])
		if !strings.HasPrefix(rest[j:], `'\''`) {
			break
		}
		payload.WriteByte('\'')
		rest = rest[j+4:]
	}
	var p struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(payload.String()), &p); err != nil {
		t.Fatalf("the --settings payload is not JSON: %v\n%s", err, payload.String())
	}
	return p.Env
}

// setCredentialEnv puts the process into the environment `env` describes:
// the shared home, and each config-dir variable set to its entry or unset
// where there is none. Presence is the state that matters — a
// CLAUDE_SECURESTORAGE_CONFIG_DIR set to "" resolves differently from one
// that is absent — so a missing key is unset and never set empty.
func setCredentialEnv(t *testing.T, home string, env map[string]string) {
	t.Helper()
	t.Setenv("HOME", home)
	for _, name := range credentialDirVars {
		if v, ok := env[name]; ok {
			t.Setenv(name, v)
		} else {
			unsetenvForTest(t, name)
		}
	}
}

// walls reports whether the read-deny this launch renders names p.
func walls(deny []string, p string) bool {
	for _, d := range deny {
		if d == p {
			return true
		}
	}
	return false
}

// The invariant, one row per way the two environments can come apart: with
// the pin on the line, the directory the runtime resolves in the PANE is
// the directory the wall was rendered from in the LAUNCHER — the file path
// and the keychain item's name both.
//
// Each row's CONTROL is the same resolution with the pin left off, and it
// must come out DIFFERENT. That is what stops the pinned half from going
// green on a rig where the environments never diverged in the first place:
// a row whose control matches has measured nothing, and says so rather than
// passing.
//
// goos is "darwin" as a PARAMETER and not `runtime.GOOS`: the claude
// credential literals are the darwin branch of credentialReadDenyLiterals,
// and the branch a linux box would take is provable from a darwin one
// (credential.go's meterStore takes the same stance).
//
// MUTATION-CHECKED (go test -overlay, 2026-09-05). Killed: the env block
// dropped out of the settings payload; the pin naming one variable instead
// of two; the pin forgetting that a VARIABLE named the directory (the
// keychain half fires there on its own); and — the control's own control —
// a resolver that stops reading the environment, where every row refuses to
// pass rather than going green on environments that no longer diverge.
// SURVIVED, and correctly: removing EITHER of the two paths that put the
// pin on the line. FleetSettingsText renders it into {settings} and
// EnsureSettingsPin appends it to a line that carries no settings flag, so
// each covers the other; killing the coupling takes removing both, which
// this test does report.
func TestQAThePinHoldsThePaneToTheWalledCredentialDir(t *testing.T) {
	home := t.TempDir()
	moved := t.TempDir()    // where a login rc, or an operator's own shell, points
	launched := t.TempDir() // where the launching shell points when it names one

	const sec, cfg = "CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CONFIG_DIR"
	for _, tc := range []struct {
		name           string
		launcher, pane map[string]string
		why            string
	}{
		{
			name: "unattended: a login rc exports the secure-storage dir the launcher never saw",
			pane: map[string]string{sec: moved},
			why:  "the dispatch runner is a herdr workspace created by the autostart hook, so its environment is the daemon's; the rc export reaches the pane's login shell and not posse",
		},
		{
			name: "unattended: the rc exports the config dir, which the secure-storage dir falls back to",
			pane: map[string]string{cfg: moved},
			why:  "the fallback is the runtime's own, so the second name moves the write exactly as the first does",
		},
		{
			name: "unattended: the rc exports both",
			pane: map[string]string{sec: moved, cfg: moved},
			why:  "the pair is what an operator who moved their config actually writes",
		},
		{
			name:     "attended, reversed: the operator's shell names a dir the pane has never heard of",
			launcher: map[string]string{cfg: launched},
			why:      "`CLAUDE_CONFIG_DIR=… posse new` moves the wall and not the pane's login shell — the direction the home literal covers today only because it is unconditional",
		},
		{
			name:     "both environments name a directory, and they disagree",
			launcher: map[string]string{sec: launched},
			pane:     map[string]string{sec: moved},
			why:      "the worst arm: every layer reports a healthy launch and the wall is over neither directory the runtime uses",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The launcher: the wall, and the line the pane will be handed.
			setCredentialEnv(t, home, tc.launcher)
			rt := claudeRuntime(t)
			ag := &AgentFile{Name: "pane", Path: home + "/pane.md", MemoryDir: home + "/mem"}
			line := ag.RenderCommandFor(rt, "claude", DefaultTier)
			pin := settingsEnvFromLine(t, line)
			deny := credentialReadDenyLiterals("darwin", rt.StateDirs)
			wantFile, err := CredentialsFile()
			if err != nil {
				t.Fatal(err)
			}
			wantItem, _ := keychainItem()
			if !walls(deny, wantFile) {
				t.Fatalf("the read-deny %v does not name %s, the file the LAUNCHER's own environment resolves — the wall and the resolver have come apart before the pane is even in the picture (ranger-base-x5f6p)", deny, wantFile)
			}

			// The pane: the daemon's environment plus the login rc's.
			setCredentialEnv(t, home, tc.pane)
			unpinned, err := CredentialsFile()
			if err != nil {
				t.Fatal(err)
			}
			if unpinned == wantFile {
				t.Fatalf("CONTROL: the pane resolves %s with no pin applied, the same file the launcher does — this row measures nothing, because the two environments it is built from do not actually diverge. %s", unpinned, tc.why)
			}

			// The runtime applies the flag-scope env block over its own
			// process environment. Both names, whatever the pane held.
			for _, name := range credentialDirVars {
				v, ok := pin[name]
				if !ok {
					t.Fatalf("the --settings payload does not name %s, so the pane's own value survives: the runtime writes its credential outside the deny %v", name, deny)
				}
				t.Setenv(name, v)
			}
			gotFile, err := CredentialsFile()
			if err != nil {
				t.Fatal(err)
			}
			if gotFile != wantFile {
				t.Errorf("with the pin applied the pane resolves %s, want %s — the wall was rendered from the launcher's environment and the write lands somewhere else. %s", gotFile, wantFile, tc.why)
			}
			if !walls(deny, gotFile) {
				t.Errorf("the pane writes %s and the read-deny names %v — a wall over a path the runtime does not use", gotFile, deny)
			}
			// The other half of the same resolution: on darwin the store is
			// keychain-first and the item's name carries a hash of the
			// directory, so a pin that landed the FILE correctly and renamed
			// the item would read as an empty keychain (ranger-base-ig4op).
			if gotItem, _ := keychainItem(); gotItem != wantItem {
				t.Errorf("with the pin applied the pane's keychain item is %q, want %q — the operator's login is under the second name", gotItem, wantItem)
			}
		})
	}
}
