package posse

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
//
// Both config-dir variables are cleared, and that is not tidiness: since
// ranger-base-wd4be the adapter FOLLOWS them, so a box whose operator has
// set one would move every path below out of the scratch home and into the
// operator's own directory — a suite that reads a live credential store and
// passes or fails on what it finds there.
func credentialsHome(t *testing.T, blob string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
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

// unsetenvForTest removes a variable for the duration of one test and puts
// the ambient value back after it. t.Setenv first, because it is what
// registers the restore — os.Unsetenv on its own leaks the absence into
// every test that runs later (the idiom initoperatorfence_qa_test.go spells
// out). Unset and present-but-empty are DIFFERENT inputs to the resolver
// under test, so this cannot be a t.Setenv("") and stop there.
func unsetenvForTest(t *testing.T, k string) {
	t.Helper()
	t.Setenv(k, "")
	os.Unsetenv(k)
}

// ─── where the credentials file is, ranger-base-wd4be ────────────────────────

// The resolution the shipped runtime performs, one arm per input. MEASURED
// off the 2.1.258 bundle (darwin-arm64 byte 158045445, and the linux-x64
// build on ranger-base-ydjz); credentialDir's doc carries the source.
//
// This is a table and not four tests because the arms only mean anything
// against each other: the same directory named by two variables at once has
// a winner, and "CLAUDE_CONFIG_DIR is followed" is a claim about the arm
// where CLAUDE_SECURESTORAGE_CONFIG_DIR is not there to shadow it.
func TestCredentialsFileFollowsTheRuntimesOwnDirectoryResolution(t *testing.T) {
	home := t.TempDir()
	sec := t.TempDir()
	cfg := t.TempDir()

	for _, tc := range []struct {
		name string
		// nil = the variable is not in the environment at all, which is a
		// different input from a pointer to "" (see the empty arm).
		secureStorage, configDir *string
		want                     string
		why                      string
	}{
		{
			name: "neither set",
			want: filepath.Join(home, ".claude", ".credentials.json"),
			why:  "the home is the FALLBACK, and it is the only arm the old hardcoded path got right",
		},
		{
			name:      "CLAUDE_CONFIG_DIR alone",
			configDir: &cfg,
			want:      filepath.Join(cfg, ".credentials.json"),
			why:       "the configuration directory, the same rule trust.go follows for the trust file — posse used to hold two answers here (ranger-base-wd4be gap 1)",
		},
		{
			name:          "CLAUDE_SECURESTORAGE_CONFIG_DIR alone",
			secureStorage: &sec,
			want:          filepath.Join(sec, ".credentials.json"),
			why:           "the secure-storage override, which appeared nowhere in the tree before this bead",
		},
		{
			name:          "both set",
			secureStorage: &sec,
			configDir:     &cfg,
			want:          filepath.Join(sec, ".credentials.json"),
			why:           "secure storage WINS: the runtime tests it first and returns without ever reading the config dir",
		},
		{
			name:          "CLAUDE_SECURESTORAGE_CONFIG_DIR present but EMPTY, with a config dir set",
			secureStorage: strPtr(""),
			configDir:     &cfg,
			want:          filepath.Join(home, ".claude", ".credentials.json"),
			why:           "the arm nobody guesses. `n!==void 0` is presence and `n||join(homedir(),'.claude')` is truthiness, so an empty value ENTERS the branch and falls to the home — setting the variable to nothing shadows CLAUDE_CONFIG_DIR rather than deferring to it, and it is not the empty string either",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			for k, v := range map[string]*string{
				"CLAUDE_SECURESTORAGE_CONFIG_DIR": tc.secureStorage,
				"CLAUDE_CONFIG_DIR":               tc.configDir,
			} {
				if v == nil {
					unsetenvForTest(t, k)
					continue
				}
				t.Setenv(k, *v)
			}
			got, err := CredentialsFile()
			if err != nil {
				t.Fatalf("%v", err)
			}
			if got != tc.want {
				t.Errorf("got %s, want %s — %s", got, tc.want, tc.why)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// The store the adapter actually reads follows the same resolution, which is
// the bead's BITE and not its mechanism: with the directory moved, a file
// that is there must be found. Before ranger-base-wd4be this answered
// NoSource — "log in once with `claude`" — on a box where claude was logged
// in and rotating a token one directory over, and ADR 0019 D3 reserves
// NoSource for a structural absence.
func TestTheMeterStoreReadsTheFileTheRuntimeWouldHaveWritten(t *testing.T) {
	for _, v := range []string{"CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CONFIG_DIR"} {
		t.Run(v, func(t *testing.T) {
			credentialsHome(t, envelope("home-token", 0)) // the path the old code assumed
			dir := t.TempDir()
			t.Setenv(v, dir)
			p := filepath.Join(dir, ".credentials.json")
			if err := os.WriteFile(p, []byte(envelope("moved-token", 0)), 0o600); err != nil {
				t.Fatal(err)
			}
			store, ns := meterStore("claude", "linux")
			if ns != nil {
				t.Fatalf("a file that is there is not a structural absence: %v", ns)
			}
			if !strings.Contains(store.Name, p) {
				t.Errorf("the store must name the file it will open (%s): %q", p, store.Name)
			}
			tok, _, err := readStore(store)
			if err != nil {
				t.Fatalf("%v", err)
			}
			// The home file is the control: it holds a DIFFERENT token, so
			// reading the wrong one is a wrong value here and not merely a
			// missing error. Nothing quotes a credential in production, but
			// these are fixtures and the whole finding is which file spoke.
			if tok != "moved-token" {
				t.Errorf("read %q — the adapter is still reading $HOME/.claude, which is the defect", tok)
			}
		})
	}
}

// ─── what the keychain item is CALLED, ranger-base-mx4q6 ─────────────────────

// The item-name derivation, one arm per input (ADR 0019 D2 store 1, V11).
//
// The digits are LITERALS, MEASURED 2026-09-02 with node's sha256 — the
// runtime's own function — and with shasum, which agree, and re-measured
// 2026-09-03 on this box. Computing the expectation here with the same
// sha256 call the code makes would agree with a mutant that hashed the wrong
// STRING, and hashing the wrong string is three of the four defects this
// table exists to catch.
//
// A table and not eight tests because the arms only mean anything against
// each other: "the config dir is hashed" is a claim about the arm where the
// secure-storage variable is not there to shadow it, and `-519e587f` says
// nothing next to an arm that does not also show what `/other` hashes to.
func TestTheKeychainItemNameIsDerivedFromTheConfigDirEnvironment(t *testing.T) {
	const (
		// Literals, not t.TempDir(): every digit below is the hash of a
		// fixed string, so the strings have to be fixed.
		home = "/tmp/home"
		cfg  = "/tmp/cfg"
	)
	for _, tc := range []struct {
		name string
		// nil = the variable is not in the environment at all, which is a
		// different input from a pointer to "" (the shadow arm below).
		secureStorage, configDir *string
		want                     string
		wantNote                 bool
		why                      string
	}{
		{
			name: "neither set",
			want: KeychainService,
			why:  "the default spelling, and the live name on the reference box today: no variable names a directory, so there is nothing to hash",
		},
		{
			name:          "CLAUDE_SECURESTORAGE_CONFIG_DIR present but EMPTY, with a config dir set",
			secureStorage: strPtr(""),
			configDir:     strPtr(cfg),
			want:          KeychainService,
			why:           "present-but-empty SHADOWS CLAUDE_CONFIG_DIR for the name exactly as it does for the file: the runtime's presence test enters the branch and falls to the home, so no variable named this directory and the item keeps the default spelling",
		},
		{
			name:      "CLAUDE_CONFIG_DIR alone",
			configDir: strPtr(cfg),
			want:      KeychainService + "-519e587f",
			why:       "sha256(\"/tmp/cfg\"), first 8 hex. NOT the file path: sha256(\"/tmp/cfg/.credentials.json\") is 77513d2f, and an item under that name has never existed",
		},
		{
			name:          "both set — secure storage wins the hash too",
			secureStorage: strPtr(cfg),
			configDir:     strPtr("/other"),
			want:          KeychainService + "-519e587f",
			why:           "the hashed string is the value the resolver RETURNED, and secure storage returned before the config dir was read. Hashing CLAUDE_CONFIG_DIR here would give -bf2faee2 and read the wrong keychain item on every box that sets both",
		},
		{
			name:      "CLAUDE_CONFIG_DIR naming the home's own .claude",
			configDir: strPtr(home + "/.claude"),
			want:      KeychainService + "-c78bad33",
			why:       "default-ness is an ENVIRONMENT property: this variable names the default directory and the item is STILL suffixed, because the runtime tests the variable and never the path. A resolver that decided by comparing the directory to the home would answer the bare constant here and read an item that is not there",
		},
		{
			name:      "CLAUDE_CONFIG_DIR with a trailing slash",
			configDir: strPtr(cfg + "/"),
			want:      KeychainService + "-350d8e6a",
			why:       "the hash is over the string as the variable SPELLS it, never a cleaned path — filepath.Clean would give -519e587f, a different item from the one the runtime wrote",
		},
		{
			name:      "a non-ASCII directory, NFC composed",
			configDir: strPtr("/tmp/caf\u00e9"),
			want:      KeychainService + "-0873cca0",
			wantNote:  true,
			why:       "posse hashes the bytes as spelled. Composed is already NFC, so this digit is also what the runtime derives",
		},
		{
			name:      "a non-ASCII directory, NFC decomposed",
			configDir: strPtr("/tmp/cafe\u0301"),
			want:      KeychainService + "-16eb4464",
			wantNote:  true,
			why:       "the same path to a human, a different string to sha256 — and posse does not normalize (Go's standard library has no NFC and ADR 0019 declines x/text), so it hashes what it was given and the note says so. This is the arm the note exists for: an item not found under -16eb4464 may be sitting under the composed -0873cca0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			for k, v := range map[string]*string{
				"CLAUDE_SECURESTORAGE_CONFIG_DIR": tc.secureStorage,
				"CLAUDE_CONFIG_DIR":               tc.configDir,
			} {
				if v == nil {
					unsetenvForTest(t, k)
					continue
				}
				t.Setenv(k, *v)
			}
			got, note := keychainItem()
			if got != tc.want {
				t.Errorf("keychainItem() = %q, want %q — %s", got, tc.want, tc.why)
			}
			switch {
			case tc.wantNote && note == "":
				t.Errorf("a non-ASCII directory derived %q with no note — the operator who reads `item not found` there is owed its first suspect (%s)", got, tc.why)
			case !tc.wantNote && note != "":
				t.Errorf("an ASCII directory carried the normalization note %q, which is false here: NFC is the identity on ASCII", note)
			case tc.wantNote && !(strings.Contains(note, "as spelled") && strings.Contains(note, "NFC")):
				t.Errorf("the note must say both halves — posse hashed it as spelled, the runtime hashes its NFC form: %q", note)
			}
		})
	}
}

// V12: the read asks for the DERIVED name, and every sentence an operator
// can be handed prints that name.
//
// The argv is measured through production's own wiring — keychainStoreAt's
// Read, which is the one place keychainCmd is called — and not by calling
// keychainCmd with a name this test chose. A pin that hands the argv builder
// its own expectation cannot tell whether the store derived anything at all.
func TestTheKeychainReadAsksForTheDerivedItemAndTheSentencesNameIt(t *testing.T) {
	const derived = KeychainService + "-519e587f"
	t.Setenv("HOME", "/tmp/home")
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/cfg")

	// The stub records the argv it was handed and answers an envelope, so one
	// read measures both the name asked for and the name the seam reports.
	argvLog := filepath.Join(t.TempDir(), "argv")
	stub := keychainStub(t, "#!/bin/sh\nprintf '%s\\n' \"$*\" >>'"+argvLog+"'\ncat <<'JSON'\n"+envelope("derived-name-fixture", 0)+"\nJSON\n")

	tok, meta, err := readStore(keychainStoreAt(stub))
	if err != nil {
		t.Fatalf("the stub must answer: %v", err)
	}
	if tok != "derived-name-fixture" {
		t.Fatalf("read %q — the envelope this arm measures is not the one that answered", tok)
	}
	argv, rerr := os.ReadFile(argvLog)
	if rerr != nil {
		t.Fatalf("the stub recorded no argv, so nothing here measured the read: %v", rerr)
	}
	if !strings.Contains(string(argv), "-s "+derived+" -w") {
		t.Errorf("the keychain read asked for %q — under a set CLAUDE_CONFIG_DIR the runtime's item is %q, and the constant reads an item that is not there (ADR 0019 V12)", strings.TrimSpace(string(argv)), derived)
	}
	if !strings.Contains(meta.Source, derived) {
		t.Errorf("the seam's Source is %q — it must name the item the read actually asked for, so a 401 names a store the operator can go and look at", meta.Source)
	}
	// keychainStore(), not keychainStoreAt(stub): this is the wiring `posse
	// refresh` prints as the meter row's source (refresh.go's meterRow takes
	// store.Name), and the name tried is the whole diagnostic value of that
	// row on a box where the item is not found.
	if name := keychainStore().Name; !strings.Contains(name, derived) {
		t.Errorf("the store names itself %q — the refresh report would print the default spelling for a read that asked for %q", name, derived)
	}
	// And the unreadable sentence: an absolute path that is not there fails
	// to exec, is not a gate refusal, and lands on the class-1 error.
	_, _, unreadable := readStore(keychainStoreAt(filepath.Join(t.TempDir(), "security")))
	if unreadable == nil {
		t.Fatal("the keychain adapter read something at a path that does not exist")
	}
	if !strings.Contains(unreadable.Error(), derived) {
		t.Errorf("the unreadable sentence says %q — an operator matching it in Keychain Access needs the suffix that was tried", unreadable)
	}
}

// The note rides on the STORE's name, which is where an operator meets it:
// the refresh report's source column, the seam's Source and the shape
// diagnosis all print that string (ADR 0019 D2 store 1).
func TestTheStoreNameCarriesTheNormalizationNoteForANonASCIIDirectory(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")

	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/cfg")
	ascii := keychainStore().Name
	if strings.Contains(ascii, "NFC") {
		t.Errorf("an ASCII directory's store name carries the normalization note: %q — NFC is the identity here and the note would be false", ascii)
	}

	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/cafe\u0301")
	name := keychainStore().Name
	if !strings.Contains(name, KeychainService+"-16eb4464") {
		t.Errorf("the store must name the item it will ask for: %q", name)
	}
	if !strings.Contains(name, "NFC") {
		t.Errorf("the store name %q says nothing about normalization — this is the one arm where posse's name and the runtime's can differ, and the note is what keeps an `item not found` there from reading as an empty keychain", name)
	}
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
	t.Parallel()
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
	t.Parallel()
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

// ─── the session half reads the FILES, and only the files ───────────────────

// envArmPoison is what the retracted arm would have returned. Every pin
// below puts it in the TEST PROCESS's own environment under the credential's
// real name, so a seam that read `os.Getenv` would pass its happy path with
// the wrong bytes rather than failing to find any — which is the failure the
// retraction is about (ADR 0039 D3d as amended, ranger-base-q3n4e). It is a
// fake and is never printed.
const envArmPoison = "sk-ant-oat01-FROM-THE-PROCESS-ENVIRONMENT-NEVER-RETURN-THIS"

// sessionSet writes one env set carrying key=value, creating envs/ if the
// fixture App has not. Unstamped: the stamp half is credexpiry_test.go's.
func sessionSet(t *testing.T, a *App, set, key, value string) {
	t.Helper()
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.EnvsDir, set+".env"),
		[]byte(key+"="+value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// namedDefaultEnv makes set the home's `default_env`, which is the only way
// a PERSONA-LESS caller's list reaches any set at all (rangerhq-f2b) — and
// `ReadCredential(rt, CredSession)` is exactly that caller.
func namedDefaultEnv(t *testing.T, a *App, set string) {
	t.Helper()
	if err := os.WriteFile(a.ConfigPath, []byte("default_env: "+set+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The session half reads the env set FILES under the home — the store of
// record ADR 0019 D1 names — selected by the caller's launch list, and never
// this process's environment. `ReadCredential` is the persona-less list;
// `ReadSessionCredentialFrom` is a launch naming its own.
func TestSessionCredentialReadsTheEnvSetFiles(t *testing.T) {
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	key := CageCredential(claude)
	t.Setenv(key, envArmPoison)

	const mint = "sk-ant-oat01-SESSION-MINT"
	sessionSet(t, a, "default", key, mint)
	namedDefaultEnv(t, a, "default")

	tok, meta, err := a.ReadCredential(claude, CredSession)
	if err != nil {
		t.Fatal(err)
	}
	if tok != mint {
		t.Errorf("the seam returns the file's value, never the environment's")
	}
	if !strings.Contains(meta.Source, key) || !strings.Contains(meta.Source, "default") {
		t.Errorf("the source names the set and the variable it came out of: %q", meta.Source)
	}
	// No `# expires=` stamp beside this value, so the expiry is unknown —
	// and unknown is reported as unknown, never as freshness (ADR 0019 D5).
	// A stamped one round-trips through this same field: ranger-base-k6ha's
	// credexpiry_test.go pins that half.
	if !meta.ExpiresAt.IsZero() {
		t.Errorf("an unstamped mint has no expiry to report: %v", meta.ExpiresAt)
	}

	// The same read with the launch's own list, which is the entry point
	// TierPreflight uses. One reader underneath, so the answer is the same.
	tok2, meta2, err := a.ReadSessionCredentialFrom(claude, []string{"default"})
	if err != nil {
		t.Fatal(err)
	}
	if tok2 != tok || meta2.Source != meta.Source {
		t.Errorf("the two entry points are one reader: %q vs %q", meta2.Source, meta.Source)
	}

	// codex/grok: undecided at this tier, which is absence and not an
	// outage — the same answer CheckCageCredential gives the launch.
	rt, err := a.LoadRuntime("codex")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = a.ReadCredential(rt, CredSession)
	var ns *NoSource
	if !errors.As(err, &ns) || ns.Purpose != CredSession {
		t.Fatalf("want *NoSource for a runtime with no decided session credential: %T %v", err, err)
	}
	if !strings.Contains(ns.Arm, "cage_cred:") {
		t.Errorf("the witness must name what would decide it: %+v", ns)
	}
}

// The three properties the retraction is worth having (ADR 0019 V5 addition,
// ADR 0039 V6): the environment is not a fallback, the LAST set in the
// launch's list wins, and a set the launch does not name is not read.
//
// The variable is in the test process's environment for all three, holding a
// value that must never come back. Without that the first case would pass
// against a seam that still read `os.Getenv` and merely found nothing.
func TestSessionCredentialIgnoresTheProcessEnvironment(t *testing.T) {
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	key := CageCredential(claude)
	t.Setenv(key, envArmPoison)

	// (a) The name is in no set the launch names. The environment has it;
	// the answer is still the refresh-verb sentence, and the sentence names
	// the sets that were opened so the operator knows where to put it.
	sessionSet(t, a, "unnamed", key, "sk-ant-oat01-IN-A-SET-NOBODY-NAMED")
	tok, _, err := a.ReadSessionCredentialFrom(claude, nil)
	if err == nil {
		t.Fatalf("a name in no named set is absent, whatever the environment holds")
	}
	if tok != "" {
		t.Errorf("a failed read returns no value")
	}
	if !strings.Contains(err.Error(), "claude setup-token") {
		t.Errorf("the refusal still says how the operator mints one: %v", err)
	}
	if strings.Contains(err.Error(), envArmPoison) {
		t.Fatalf("the refusal quoted a credential")
	}

	// (c) …and naming a DIFFERENT set does not reach it either: `unnamed`
	// carries the name and is not in the list, so it is not read.
	sessionSet(t, a, "empty", "SOMETHING_ELSE", "x")
	if _, _, err := a.ReadSessionCredentialFrom(claude, []string{"empty"}); err == nil {
		t.Errorf("a set that is not in the list must not be read even though it carries the name")
	}

	// (b) Two sets in the list carrying the name: the LAST assignment in
	// launch order wins — the rule readStamps already applies within one
	// file, extended across the files one launch reads in order.
	const first, last = "sk-ant-oat01-FIRST-SET", "sk-ant-oat01-LAST-SET"
	sessionSet(t, a, "base", key, first)
	sessionSet(t, a, "over", key, last)
	tok, meta, err := a.ReadSessionCredentialFrom(claude, []string{"base", "over"})
	if err != nil {
		t.Fatal(err)
	}
	if tok != last {
		t.Errorf("the last set in launch order wins")
	}
	if !strings.Contains(meta.Source, "over") {
		t.Errorf("the source names the set that actually answered: %q", meta.Source)
	}
	// The order is the LIST's, not the alphabet's or the directory's:
	// reversed, the other one wins.
	if tok, _, err := a.ReadSessionCredentialFrom(claude, []string{"over", "base"}); err != nil || tok != first {
		t.Errorf("reversing the launch list reverses the winner: %v", err)
	}
}

// Nothing the seam returns quotes a credential — the rule this code was
// collected under, restated across both stores and every failure class.
func TestNoSeamErrorEverCarriesTheCredential(t *testing.T) {
	t.Parallel()
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
