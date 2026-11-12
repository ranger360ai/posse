//go:build !posse_arm2 && !posse_arm3

package posse

// CredToken cannot be printed (ranger-base-3spp4). Every pin here plants a
// value and asserts the rendering carries no byte of it — by searching for
// the value AND for its hex and base64 spellings, because %x of a string is
// the string's bytes in another alphabet and a redaction that blocks %s but
// not %x has blocked nothing.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const credTokenPlant = "sk-ant-oat01-PLANTED-3spp4-do-not-print"

// credTokenLeaks reports which spelling of the plant out carries, "" for none.
func credTokenLeaks(out string) string {
	for _, s := range []string{
		credTokenPlant,
		"PLANTED-3spp4", // a fragment: %.5s, a width, a truncation
		hex.EncodeToString([]byte(credTokenPlant)),
		strings.ToUpper(hex.EncodeToString([]byte(credTokenPlant))),
		base64.StdEncoding.EncodeToString([]byte(credTokenPlant)),
	} {
		if strings.Contains(out, s) {
			return s
		}
	}
	return ""
}

func TestCredTokenPrintsNoByteUnderAnyVerb(t *testing.T) {
	t.Parallel()
	tok := newCredToken(credTokenPlant)
	want := fmt.Sprintf("[credential, %d bytes]", len(credTokenPlant))
	verbs := []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%T",
		"%10s", "%-40v", "%.5s", "%#q", "%+q", "% x", "%U", "%c", "%p"}
	for _, v := range verbs {
		out := fmt.Sprintf(v, tok)
		if leak := credTokenLeaks(out); leak != "" {
			t.Errorf("fmt %s printed the token's bytes (%q found)", v, leak)
		}
		// %T names the type and %p is a bad verb on a struct that fmt
		// handles before any method; both must leak nothing, neither
		// renders the redaction verbatim.
		if v == "%T" || v == "%p" {
			continue
		}
		if out != want {
			t.Errorf("fmt %s = %q, want the redaction %q", v, out, want)
		}
	}
	for name, out := range map[string]string{
		"Sprint":   fmt.Sprint(tok),
		"Sprintln": fmt.Sprintln(tok),
		"String":   tok.String(),
		"GoString": tok.GoString(),
		"Errorf":   fmt.Errorf("read %v: %w", tok, errors.New("boom")).Error(),
	} {
		if leak := credTokenLeaks(out); leak != "" {
			t.Errorf("%s printed the token's bytes (%q found): %q", name, leak, out)
		}
		if !strings.Contains(out, want) {
			t.Errorf("%s = %q, want it to carry the redaction %q", name, out, want)
		}
	}
}

// The case the pointer is for: fmt cannot call Format on a value it reaches
// through an UNEXPORTED field, and walks it reflectively instead. A string
// field directly inside CredToken would print there. Mutation-checked: with
// `type CredToken struct{ s string }` this reds on every row.
func TestCredTokenNestedInAnUnexportedFieldPrintsNoByte(t *testing.T) {
	t.Parallel()
	type carrier struct {
		name string
		tok  CredToken
		meta CredMeta
	}
	type outer struct {
		inner carrier
		list  []CredToken
		byKey map[string]CredToken
	}
	tok := newCredToken(credTokenPlant)
	c := carrier{name: "x", tok: tok, meta: CredMeta{Source: "env set s"}}
	o := outer{inner: c, list: []CredToken{tok}, byKey: map[string]CredToken{"k": tok}}
	for _, v := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
		for name, val := range map[string]any{"carrier": c, "&carrier": &c, "outer": o, "&outer": &o} {
			if leak := credTokenLeaks(fmt.Sprintf(v, val)); leak != "" {
				t.Errorf("fmt %s of %s printed the token's bytes (%q found)", v, name, leak)
			}
		}
	}
}

func TestCredTokenMarshalsTheRedaction(t *testing.T) {
	t.Parallel()
	tok := newCredToken(credTokenPlant)
	type exported struct {
		Tok  CredToken
		Ptr  *CredToken
		Keys map[string]CredToken
		Meta CredMeta
	}
	b, err := json.Marshal(exported{Tok: tok, Ptr: &tok, Keys: map[string]CredToken{"a": tok}})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if leak := credTokenLeaks(string(b)); leak != "" {
		t.Errorf("json carried the token's bytes (%q found): %s", leak, b)
	}
	if !strings.Contains(string(b), `"[credential, `) {
		t.Errorf("json = %s, want the redaction as a string", b)
	}
	tb, _ := tok.MarshalText()
	if leak := credTokenLeaks(string(tb)); leak != "" {
		t.Errorf("MarshalText carried the token's bytes (%q found)", leak)
	}
	// slog: both handlers, the value as an attr and nested in a group.
	for name, mk := range map[string]func(*bytes.Buffer) slog.Handler{
		"text": func(w *bytes.Buffer) slog.Handler { return slog.NewTextHandler(w, nil) },
		"json": func(w *bytes.Buffer) slog.Handler { return slog.NewJSONHandler(w, nil) },
	} {
		var w bytes.Buffer
		slog.New(mk(&w)).Info("read", "tok", tok, slog.Group("g", "t", tok), "any", exported{Tok: tok})
		if leak := credTokenLeaks(w.String()); leak != "" {
			t.Errorf("slog %s handler printed the token's bytes (%q found): %s", name, leak, w.String())
		}
	}
}

func TestCredTokenEmptyAndFingerprint(t *testing.T) {
	t.Parallel()
	var zero CredToken
	if !zero.Empty() || zero.Len() != 0 || zero.Fingerprint() != "" {
		t.Errorf("zero CredToken: Empty=%v Len=%d Fingerprint=%q — want the empty credential", zero.Empty(), zero.Len(), zero.Fingerprint())
	}
	if e := newCredToken(""); !e.Empty() || e.Fingerprint() != "" {
		t.Errorf("newCredToken(\"\") is not the empty credential")
	}
	if got := fmt.Sprint(zero); got != "[credential, 0 bytes]" {
		t.Errorf("zero prints %q", got)
	}
	a, b := newCredToken(credTokenPlant), newCredToken(credTokenPlant)
	if a.Empty() || a.Len() != len(credTokenPlant) {
		t.Errorf("planted token: Empty=%v Len=%d", a.Empty(), a.Len())
	}
	if a.Fingerprint() != b.Fingerprint() || !sameCredToken(a, credTokenPlant) {
		t.Error("two tokens of one value must fingerprint alike")
	}
	if sameCredToken(a, credTokenPlant+"x") {
		t.Error("two tokens of different values must not fingerprint alike")
	}
	// The fingerprint is the one CredFingerprint wrote before this type: a
	// plancache cooldown record already on disk must still match its token.
	sum := sha256.Sum256([]byte(credTokenPlant))
	if fp := a.Fingerprint(); fp != hex.EncodeToString(sum[:]) {
		t.Errorf("Fingerprint = %q, want the hex sha256 of the value — the hash CredFingerprint has always written", fp)
	}
	if leak := credTokenLeaks(a.Fingerprint()); leak != "" {
		t.Errorf("the fingerprint carries the token (%q found)", leak)
	}
}
