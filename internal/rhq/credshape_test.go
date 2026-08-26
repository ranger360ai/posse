package rhq

// The keychain item is present, readable, and valid JSON of a shape posse
// does not know (ranger-base-okbr) — the fourth credential-failure class,
// after "unreadable", "refused by our own gate", and "not JSON".
//
// The bug these pin is the DIAGNOSTIC. "has no claudeAiOauth.accessToken" is
// true and useless: it does not say what the item DOES hold, so an operator
// cannot act on it, and on 2026-08-26 that cost an hour of stopped shop. The
// error must name the key NAMES it actually found — and never a value, which
// is the whole reason the old line said nothing.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureSecret = "sk-ant-oat01-FIXTURE-SECRET-VALUE"

// The shape that works still works, and says which shape answered.
func TestCredentialTokenReadsTheOAuthEnvelope(t *testing.T) {
	blob := `{"claudeAiOauth":{"accessToken":"` + fixtureSecret + `","refreshToken":"r","expiresAt":123}}` + "\n"
	tok, shape, err := credentialToken([]byte(blob))
	if err != nil {
		t.Fatal(err)
	}
	if tok != fixtureSecret {
		t.Errorf("token: %q", tok)
	}
	if shape != "claudeAiOauth.accessToken" {
		t.Errorf("shape: %q", shape)
	}
}

// The bead's own case: an item whose envelope is there but whose token is
// not. The error must distinguish that from "no envelope at all", because
// the two have different fixes.
func TestWrongShapeNamesTheKeysItFound(t *testing.T) {
	for _, tc := range []struct {
		name  string
		blob  string
		wants []string
		not   []string
	}{
		{
			name: "envelope present, token absent",
			blob: `{"claudeAiOauth":{"refreshToken":"r","expiresAt":123,"scopes":["user:inference"]}}`,
			wants: []string{"[claudeAiOauth]", "claudeAiOauth's keys are [expiresAt refreshToken scopes]",
				"renamed or dropped", "teach credShapes"},
			not: []string{"incomplete credential"},
		},
		{
			name:  "envelope absent — a different credential structure",
			blob:  `{"apiKey":"` + fixtureSecret + `","createdAt":1}`,
			wants: []string{"top-level keys are [apiKey createdAt]", "is not among them"},
			not:   []string{"claudeAiOauth's keys"},
		},
		{
			name:  "envelope present but empty",
			blob:  `{"claudeAiOauth":{}}`,
			wants: []string{"claudeAiOauth's keys are [] (an empty object)", "renamed or dropped"},
		},
		{
			name:  "envelope is not an object",
			blob:  `{"claudeAiOauth":"` + fixtureSecret + `"}`,
			wants: []string{"claudeAiOauth is a JSON string, not an object"},
		},
		{
			name:  "envelope is null",
			blob:  `{"claudeAiOauth":null}`,
			wants: []string{"claudeAiOauth is JSON null, not an object"},
			not:   []string{"an empty object"},
		},
		{
			name: "token present but empty — an incomplete credential, not our parse",
			blob: `{"claudeAiOauth":{"accessToken":"","refreshToken":"r"}}`,
			wants: []string{"claudeAiOauth's keys are [accessToken refreshToken]",
				"incomplete credential", "re-authenticate rather than change posse",
				"a refreshToken is present"},
			not: []string{"renamed or dropped"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tok, shape, err := credentialToken([]byte(tc.blob))
			if err == nil {
				t.Fatalf("want a shape failure, got token %q in shape %q", tok, shape)
			}
			msg := err.Error()
			if !strings.Contains(msg, KeychainService) || !strings.Contains(msg, "tried claudeAiOauth.accessToken") {
				t.Errorf("the error must name the item and the shapes tried: %q", msg)
			}
			for _, w := range tc.wants {
				if !strings.Contains(msg, w) {
					t.Errorf("want %q in: %q", w, msg)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(msg, n) {
					t.Errorf("must not say %q: %q", n, msg)
				}
			}
			if strings.Contains(msg, fixtureSecret) {
				t.Errorf("a value reached the error line: %q", msg)
			}
		})
	}
}

// The TOP LEVEL the operator actually reported on 2026-08-26
// (ranger-base-8i7l): ['mcpOAuth', 'claudeAiOauth']. The envelope posse
// reads for IS present, so the defect is one level deeper — and the inner
// keys are still unread, which is what this bead is waiting on.
//
// Both inner cases are pinned here because the line is what the operator
// will read: with that one top level, either the token field is gone
// (renamed) or it is there and empty (an incomplete credential). The error
// must say WHICH, not hand back a key list to diff by eye. mcpOAuth is a
// second envelope posse knows nothing about; it is named at the top level
// and never opened, because its keys are per-server URLs and not ours.
func TestObservedOutageShapeNamesBothLevels(t *testing.T) {
	const mcp = `"mcpOAuth":{"https://example.test/sse":{"accessToken":"` + fixtureSecret + `"}},`
	for _, tc := range []struct {
		name  string
		inner string
		wants []string
	}{
		{
			name:  "accessToken gone — renamed, and ours to fix",
			inner: `{"refreshToken":"r","expiresAt":1756224000000,"scopes":["user:inference"],"subscriptionType":"max"}`,
			wants: []string{
				"claudeAiOauth's keys are [expiresAt refreshToken scopes subscriptionType]",
				"renamed or dropped", "teach credShapes",
			},
		},
		{
			name:  "accessToken there and empty — incomplete credential, not ours",
			inner: `{"accessToken":"","refreshToken":"r","expiresAt":1756224000000}`,
			wants: []string{
				"claudeAiOauth's keys are [accessToken expiresAt refreshToken]",
				"incomplete credential", "re-authenticate rather than change posse",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := credentialToken([]byte(`{` + mcp + `"claudeAiOauth":` + tc.inner + `}`))
			if err == nil {
				t.Fatal("want a shape failure")
			}
			msg := err.Error()
			t.Logf("operator-visible line:\n  %s", msg)
			if !strings.Contains(msg, "its top-level keys are [claudeAiOauth mcpOAuth]") {
				t.Errorf("want both envelopes named at the top level: %q", msg)
			}
			for _, w := range tc.wants {
				if !strings.Contains(msg, w) {
					t.Errorf("want %q in: %q", w, msg)
				}
			}
			if strings.Contains(msg, "example.test") {
				t.Errorf("only the envelope the failing shape names is opened: %q", msg)
			}
			if strings.Contains(msg, fixtureSecret) {
				t.Errorf("a value reached the error line: %q", msg)
			}
		})
	}
}

// Key names are schema and safe to print. The one way they are not is an
// object keyed BY a value — so a name that is not name-shaped is reported by
// its size, never its bytes.
func TestKeyNamesNeverCarryAValue(t *testing.T) {
	blob := `{"` + fixtureSecret + `":1,"ok":2}`
	_, _, err := credentialToken([]byte(blob))
	if err == nil {
		t.Fatal("want a shape failure")
	}
	msg := err.Error()
	if strings.Contains(msg, fixtureSecret) || strings.Contains(msg, "sk-ant") {
		t.Fatalf("a long key was echoed back: %q", msg)
	}
	if !strings.Contains(msg, "bytes, not a name") || !strings.Contains(msg, "ok") {
		t.Errorf("want the odd key described and the ordinary one named: %q", msg)
	}
}

// A hundred keys is not a credential; the count says so better than the list.
func TestKeyListIsBounded(t *testing.T) {
	obj := map[string]json.RawMessage{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o"} {
		obj[k] = json.RawMessage(`1`)
	}
	got := safeKeys(obj)
	if !strings.Contains(got, "(+3 more)") || strings.Contains(got, " o]") {
		t.Errorf("want 12 shown and the rest counted: %q", got)
	}
}

// Valid JSON that is not an object is its own diagnosis, and it names the
// kind — not the content.
func TestNonObjectJSONNamesItsKind(t *testing.T) {
	for blob, want := range map[string]string{
		`["` + fixtureSecret + `"]`: "a JSON array",
		`"` + fixtureSecret + `"`:   "a JSON string",
		`null`:                      "JSON null",
		`<html>nope`:                "not JSON",
	} {
		_, _, err := credentialToken([]byte(blob))
		if err == nil {
			t.Fatalf("%q: want a failure", blob)
		}
		msg := err.Error()
		if !strings.Contains(msg, "not the expected JSON") || !strings.Contains(msg, want) {
			t.Errorf("%q: want %q in: %q", blob, want, msg)
		}
		if strings.Contains(msg, fixtureSecret) {
			t.Errorf("%q: a value reached the error line: %q", blob, msg)
		}
	}
}

// End to end, through the real exec path and out to the plan guard's blind
// line: the operator reads what the item HOLDS, not what it lacks. A stub
// `security` on PATH stands in for the keychain — the real one is never
// touched, and the wrong shape is the whole point of the fixture.
func TestPlanGuardBlindLineNamesTheShapeItFound(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	bin := t.TempDir()
	stub := "#!/bin/sh\ncat <<'JSON'\n{\"claudeAiOauth\":{\"refreshToken\":\"r\",\"expiresAt\":1},\"userID\":\"u\"}\nJSON\n"
	if err := os.WriteFile(filepath.Join(bin, "security"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := newBlindRig(t, guardOn)
	r.d.Plan.Token = KeychainToken

	if n := r.run(t); n != 1 {
		t.Fatalf("a monitoring failure still fails open when attended: %d dispatched\n%s", n, r.out())
	}
	errs := r.err()
	if !strings.Contains(errs, "plan guard: ") || !strings.Contains(errs, "top-level keys are [claudeAiOauth userID]") {
		t.Errorf("the blind line must name what the item holds: %q", errs)
	}
	if !strings.Contains(errs, "claudeAiOauth's keys are [expiresAt refreshToken]") {
		t.Errorf("the blind line must name the envelope it did find: %q", errs)
	}
	if strings.Contains(errs, "unreadable") {
		t.Errorf("a wrong shape is not an unreadable item: %q", errs)
	}
}
