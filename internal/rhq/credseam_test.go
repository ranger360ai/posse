package rhq

// The credential seam (ADR 0019, ranger-base-x584). What these pin is the
// property the bead exists for: posse's credential read is one seam with a
// store per platform, and the platform it cannot run on is a platform that
// says so — never one that reports a permanent structural condition as a
// transient outage. On 2026-08-24 exactly that misreading switched off the
// shop's only automated brake, and off darwin today's code would have made
// it permanent.
//
// Every test here runs on any box: the GOOS is a PARAMETER of the store
// choice, so `make test-linux` and a macOS run prove the same two branches.

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// envelope is the shape Claude Code writes, measured (ranger-base-okbr) and
// the same one on both platforms — which is the whole reason one parser can
// serve both stores.
func envelope(tok string, expiresAt int64) string {
	b, _ := json.Marshal(map[string]any{"claudeAiOauth": map[string]any{
		"accessToken": tok, "refreshToken": "r", "expiresAt": expiresAt,
		"scopes": []string{"user:inference"}, "subscriptionType": "max",
	}})
	return string(b)
}

// credentialsHome points $HOME at a scratch dir and optionally writes the
// runtime's credentials file into it. The returned path is where the
// non-darwin adapter will look.
func credentialsHome(t *testing.T, blob string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := filepath.Join(home, ".claude", ".credentials.json")
	if blob != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(blob), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// The switch is total and it is a switch: every platform that is not darwin
// gets the file store, and darwin alone gets the keychain. No build tags, so
// this is checkable from either box.
func TestTheMeterStoreSwitchIsTotal(t *testing.T) {
	credentialsHome(t, envelope("t", 0))
	if s, ns := meterStore("claude", "darwin"); ns != nil || !strings.Contains(s.Name, "keychain") {
		t.Errorf("darwin's store of record is the keychain: %q %v", s.Name, ns)
	}
	for _, goos := range []string{"linux", "windows", "freebsd", "openbsd", "netbsd", "solaris", ""} {
		s, ns := meterStore("claude", goos)
		if ns != nil {
			t.Errorf("%s: a store exists here, absence is the READ's answer: %v", goos, ns)
			continue
		}
		if !strings.Contains(s.Name, ".credentials.json") || strings.Contains(s.Name, "keychain") {
			t.Errorf("%s: want the runtime's credentials file, got %q", goos, s.Name)
		}
	}
}

// ADR 0019 V2, the load-bearing half: a non-darwin read never execs
// `security` — proven by a stub that would leave a footprint if it ran —
// and no error it can produce names the macOS store, on a system that has
// none. (The name of this test is deliberately free of that word: t.TempDir
// puts it in the path, and the path is in the error.)
func TestNonDarwinNeverExecsSecurityAndNamesOnlyItsOwnStore(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	bin := t.TempDir()
	ran := filepath.Join(bin, "it-ran")
	stub := "#!/bin/sh\ntouch " + ran + "\necho '{}'\n"
	if err := os.WriteFile(filepath.Join(bin, "security"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, tc := range []struct{ name, blob, want string }{
		{"no file at all", "", "log in once with `claude`"},
		{"not JSON", "<html>nope", "not the expected JSON"},
		{"a shape posse does not know", `{"apiKey":"x"}`, "is not among them"},
		{"an empty token", `{"claudeAiOauth":{"accessToken":"","refreshToken":"r"}}`, "incomplete credential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := credentialsHome(t, tc.blob)
			store, ns := meterStore("claude", "linux")
			if ns != nil {
				t.Fatalf("the store is chosen before it is read: %v", ns)
			}
			tok, _, err := readStore(store)
			if err == nil {
				t.Fatalf("want a failure, got a token %q", tok)
			}
			if _, statErr := os.Stat(ran); statErr == nil {
				t.Fatal("the non-darwin path execed `security`")
			}
			msg := err.Error()
			if strings.Contains(strings.ToLower(msg), "keychain") {
				t.Errorf("a system with no keychain must never be told about one: %q", msg)
			}
			if !strings.Contains(msg, p) {
				t.Errorf("the error must name the store it actually tried (%s): %q", p, msg)
			}
			if !strings.Contains(msg, tc.want) {
				t.Errorf("want %q in: %q", tc.want, msg)
			}
		})
	}
}

// ADR 0019 D3. A credentials file that was never written is not an outage:
// the runtime has not logged in here, no retry can change that, and under
// ADR 0018 a blind meter with the ledger unarmed parks every on-meter bead.
// So absence is its own type, and it carries the fix.
func TestAMissingCredentialsFileIsNoSourceAndSaysWhatWouldArmIt(t *testing.T) {
	p := credentialsHome(t, "")
	store, _ := meterStore("claude", "linux")
	_, _, err := readStore(store)
	var ns *NoSource
	if !errors.As(err, &ns) {
		t.Fatalf("want *NoSource so the guard can run off rather than blind, got %T: %v", err, err)
	}
	if ns.GOOS != "linux" || ns.Purpose != CredMeter || ns.Runtime != "claude" {
		t.Errorf("the witness must name the (runtime, purpose, platform): %+v", ns)
	}
	if !strings.Contains(ns.Store, p) || !strings.Contains(ns.Arm, "claude") {
		t.Errorf("want the store it would need and what would arm it: %+v", ns)
	}
	// A file that IS there and cannot be read is the other thing: something
	// exists, so this is a read failure — blindness, which retries and a
	// clock are the right answer to.
	if os.Geteuid() == 0 {
		t.Skip("root reads an unreadable file")
	}
	locked := credentialsHome(t, envelope("tok", 0))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	store, _ = meterStore("claude", "linux")
	_, _, err = readStore(store)
	if err == nil || errors.As(err, &ns) {
		t.Fatalf("a present-but-unreadable store is an outage, not an absence: %v", err)
	}
	if !strings.Contains(err.Error(), "unreadable") {
		t.Errorf("want the read failure: %q", err)
	}
}

// ADR 0019 V1 stated where it bites: the non-darwin path is built from a
// measured envelope but has not been run against a live Linux login, and its
// errors say so rather than implying a confirmed path failed. The darwin
// path, which HAS been run, says nothing of the kind.
func TestTheUnconfirmedLinuxPathSaysSoAndDarwinDoesNot(t *testing.T) {
	for name, blob := range map[string]string{"a store that failed to read": "<html>nope", "no store at all": ""} {
		credentialsHome(t, blob)
		store, _ := meterStore("claude", "linux")
		_, _, err := readStore(store)
		if err == nil || !strings.Contains(err.Error(), "not yet confirmed against a live login") {
			t.Errorf("%s: want the honest disclaimer (ADR 0019 V1): %v", name, err)
		}
	}
	if _, _, derr := credentialToken(keychainStore().Name, []byte("<html>nope")); derr == nil ||
		strings.Contains(derr.Error(), "not yet confirmed") {
		t.Errorf("the darwin path has been run and does not disclaim: %v", derr)
	}
}

// ADR 0019 V7: one fixture, two stores, one diagnosis. The stores differ in
// the NAME they are known by and in nothing else — the reading, the shape
// verdict and the fix are the same code, so ranger-base-okbr's hour of
// stopped shop is paid for on Linux too without a second implementation to
// keep in step.
func TestOneEnvelopeReadsIdenticallyThroughBothStores(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	const secret = "sk-ant-oat01-FIXTURE-BOTH-PATHS"
	expires := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct{ name, blob string }{
		{"a credential that reads", envelope(secret, expires.UnixMilli())},
		{"an envelope with no token", `{"claudeAiOauth":{"refreshToken":"r","expiresAt":1}}`},
		{"some other structure entirely", `{"apiKey":"x","createdAt":1}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The keychain, through the real exec path: a stub `security`
			// named to the adapter hands back the same bytes the file holds.
			bin := keychainStub(t, "#!/bin/sh\ncat <<'JSON'\n"+tc.blob+"\nJSON\n")
			ktok, kmeta, kerr := readStore(keychainStoreAt(bin))

			p := credentialsHome(t, tc.blob)
			fstore, _ := meterStore("claude", "linux")
			ftok, fmeta, ferr := readStore(fstore)

			if ktok != ftok || kmeta.Shape != fmeta.Shape || !kmeta.ExpiresAt.Equal(fmeta.ExpiresAt) {
				t.Errorf("two stores, one envelope, different answers:\n  keychain %q %+v\n  file     %q %+v", ktok, kmeta, ftok, fmeta)
			}
			if (kerr == nil) != (ferr == nil) {
				t.Fatalf("one path failed and the other did not: %v / %v", kerr, ferr)
			}
			if kerr == nil {
				if ktok != secret || !kmeta.ExpiresAt.Equal(expires) {
					t.Errorf("the reading itself: %q %+v", ktok, kmeta)
				}
				if kmeta.Source != keychainStore().Name || fmeta.Source != fstore.Name {
					t.Errorf("each answer names its own store: %q / %q", kmeta.Source, fmeta.Source)
				}
				return
			}
			// The diagnosis is one sentence with the store's name in front
			// of it; strip each store's name and the disclaimer, and what
			// is left must match byte for byte.
			k := strings.Replace(kerr.Error(), keychainStore().Name, "<store>", 1)
			f := strings.Replace(ferr.Error(), fstore.Name, "<store>", 1)
			f = strings.Replace(f, meterUnconfirmed, "", 1)
			if k != f {
				t.Errorf("the diagnosis forked:\n  keychain %q\n  file     %q", k, f)
			}
			if !strings.Contains(f, p) && strings.Contains(f, "keychain") {
				t.Errorf("the file path's diagnosis kept a keychain word: %q", f)
			}
		})
	}
}

// ADR 0019 D5. The expiry is read from the envelope's own field, and a value
// that is not a date in the unit the envelope uses is "cannot tell" — never
// a guess. A wrong date warns forever or never, and both are worse than the
// honest zero the callers render as "cannot tell".
func TestExpiryIsReadOrUnknownNeverGuessed(t *testing.T) {
	want := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		at   int64
		want time.Time
	}{
		{"milliseconds, the envelope's unit", want.UnixMilli(), want},
		{"seconds — not this envelope's unit, so unknown", want.Unix(), time.Time{}},
		{"absent", 0, time.Time{}},
		{"negative", -1, time.Time{}},
		{"a fixture number", 123, time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, meta, err := credentialToken(keychainStore().Name, []byte(envelope("tok", tc.at)))
			if err != nil {
				t.Fatal(err)
			}
			if !meta.ExpiresAt.Equal(tc.want) {
				t.Errorf("expiry: got %v, want %v", meta.ExpiresAt, tc.want)
			}
		})
	}
}

// ADR 0019 D6 / ADR 0012 D4: a runtime posse ships no usage-endpoint adapter
// for has no meter credential to want. That is guard-OFF with a witness, not
// a credential outage — and it is the same answer on every platform.
func TestARuntimeWithNoMeterAdapterIsNoSource(t *testing.T) {
	for _, rt := range []string{"codex", "grok", "own"} {
		for _, goos := range []string{"darwin", "linux"} {
			_, ns := meterStore(rt, goos)
			if ns == nil {
				t.Fatalf("%s/%s: posse meters only claude today", rt, goos)
			}
			if ns.Runtime != rt || ns.Purpose != CredMeter || !strings.Contains(ns.Arm, "ADR 0012 D4") {
				t.Errorf("%s/%s: the witness must say which runtime and why: %+v", rt, goos, ns)
			}
		}
	}
	var ns *NoSource
	if err := MeterUnavailable("codex"); !errors.As(err, &ns) {
		t.Errorf("the plan adapter's availability question gets the same answer: %T %v", err, err)
	}
}

// MeterUnavailable is what the shipped plan adapter asks before building a
// reader. On a machine with a store it says nothing; on one whose store was
// never written it is the NoSource the guard runs off on.
func TestMeterUnavailableAnswersFromThisMachinesStore(t *testing.T) {
	if runtime.GOOS == "darwin" {
		// The only way to ask the keychain whether an item exists is to read
		// it, and a read is the reader's business — so darwin never
		// pre-refuses, exactly as it did before this seam.
		if err := MeterUnavailable("claude"); err != nil {
			t.Errorf("darwin has a store of record whatever is in it: %v", err)
		}
		return
	}
	credentialsHome(t, "")
	var ns *NoSource
	if err := MeterUnavailable("claude"); !errors.As(err, &ns) {
		t.Fatalf("a machine that never logged in has no meter source: %T %v", err, err)
	}
	credentialsHome(t, envelope("tok", 0))
	if err := MeterUnavailable("claude"); err != nil {
		t.Errorf("a written store is a store: %v", err)
	}
}

// The session half wraps the env-set lookup that was already there
// (rangerhq-kiz): the PID names an env set, the set carries the operator's
// own scoped mint, and the seam reads the value the launch put in this
// process's environment. Behaviour unchanged — one caller more.
func TestSessionCredentialWrapsTheEnvSetLookup(t *testing.T) {
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oat01-SESSION-MINT")
	tok, meta, err := ReadCredential(claude, CredSession)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "sk-ant-oat01-SESSION-MINT" {
		t.Errorf("token: %q", tok)
	}
	if !strings.Contains(meta.Source, "CLAUDE_CODE_OAUTH_TOKEN") {
		t.Errorf("the source names the variable the operator put it in: %q", meta.Source)
	}
	// No `# expires=` stamp exists yet (ranger-base-h207 writes it), and an
	// unknown expiry is reported as unknown.
	if !meta.ExpiresAt.IsZero() {
		t.Errorf("nothing knows a session mint's expiry today: %v", meta.ExpiresAt)
	}

	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	if _, _, err := ReadCredential(claude, CredSession); err == nil ||
		!strings.Contains(err.Error(), "claude setup-token") {
		t.Errorf("a missing mint says how the operator mints one: %v", err)
	}

	// codex/grok: undecided at this tier, which is absence and not an
	// outage — the same answer CheckCageCredential gives the launch.
	rt, err := a.LoadRuntime("codex")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ReadCredential(rt, CredSession)
	var ns *NoSource
	if !errors.As(err, &ns) || ns.Purpose != CredSession {
		t.Fatalf("want *NoSource for a runtime with no decided session credential: %T %v", err, err)
	}
	if !strings.Contains(ns.Arm, "cage_cred:") {
		t.Errorf("the witness must name what would decide it: %+v", ns)
	}
}

// Nothing the seam returns quotes a credential — the rule this code was
// collected under, restated across both stores and every failure class.
func TestNoSeamErrorEverCarriesTheCredential(t *testing.T) {
	// Longer than maxKeyName on purpose: below that bound a key IS a name and
	// is printed as schema, which is the documented behaviour and not a leak.
	const secret = "sk-ant-oat01-NEVER-IN-AN-ERROR-AND-LONGER-THAN-A-KEY-NAME"
	for _, blob := range []string{
		`{"claudeAiOauth":{"accessToken":{"v":"` + secret + `"}}}`,
		`{"` + secret + `":1}`,
		`["` + secret + `"]`,
		`"` + secret + `"`,
	} {
		for _, store := range []runtimeStore{keychainStore(), credentialsFileStore("linux")} {
			_, _, err := credentialToken(store.Name, []byte(blob))
			if err == nil {
				t.Fatalf("%q: want a failure", blob)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "sk-ant") {
				t.Errorf("the credential reached an error line: %q", err)
			}
		}
	}
}
