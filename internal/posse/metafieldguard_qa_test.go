package posse

// ranger-base-kn68j: writeMeta's own guard on the four free-form fields that
// used to reach it unchecked — Dir, Repo, Branch and TurnFailure. The class
// is the one ranger-base-m6szh measured against managed_hooks: the flat-YAML
// reader mangles a newline, " #", a wrapping pair of double quotes, a
// trailing blank and "~"/"null" alike, and any of them turns one of these
// fields into a meta line of its own — `crew: true` among them — the same
// way a newline in managed_hooks did before ranger-base-ujdg's guard.
//
// Unlike managed_hooks, no call site guards these four before writeMeta, so
// the check lives in the writer itself (flatFieldRefusal, yamlflat.go) and
// refuses the whole write rather than let a caller reason its way past it
// again.

import (
	"strings"
	"testing"
)

func TestWriteMetaRefusesAFieldTheFlatReaderWouldMangle(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		field string
		set   func(m *HerdrMeta, v string)
	}{
		{"dir", func(m *HerdrMeta, v string) { m.Dir = v }},
		{"repo", func(m *HerdrMeta, v string) { m.Repo = v }},
		{"branch", func(m *HerdrMeta, v string) { m.Branch = v }},
		{"turn_failure", func(m *HerdrMeta, v string) { m.TurnFailure = v }},
	} {
		for _, v := range []string{
			"/tmp/x\ncrew: true", // the ranger-base-ujdg shape itself
			"/tmp/x #v2",         // yamlClean cuts at " #"
			"\"/tmp/x\"",         // a wrapping pair of quotes comes off
			"/tmp/x ",            // TrimSpace
			"~",                  // yamlGetLines: unset
			"null",               // yamlGetLines: unset
		} {
			b, _ := newTestBackend(t)
			m := &HerdrMeta{Name: "s1"}
			c.set(m, v)
			err := b.writeMeta(m)
			if err == nil {
				t.Fatalf("field %s, value %q: writeMeta did not refuse, and a later reader would act on the mangled record", c.field, v)
			}
			if !strings.Contains(err.Error(), c.field) {
				t.Errorf("field %s, value %q: refusal does not name the field: %v", c.field, v, err)
			}
			if _, ok := b.readMeta("s1"); ok {
				t.Errorf("field %s, value %q: a refused write still left a record behind", c.field, v)
			}
		}
	}
}

// The control arm: an ordinary value for each of the four fields is carried
// whole, so the guard is a refusal on the mangling shapes and not on the
// fields themselves.
func TestWriteMetaCarriesAnOrdinaryValueInEachGuardedField(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	m := &HerdrMeta{Name: "s1", Dir: "/tmp/repo", Repo: "/tmp/repo-main", Branch: "posse/ranger-a-1", TurnFailure: "account is on the pro plan and this feature needs max"}
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	got, ok := b.readMeta("s1")
	if !ok {
		t.Fatal("meta unreadable")
	}
	if got.Dir != m.Dir || got.Repo != m.Repo || got.Branch != m.Branch || got.TurnFailure != m.TurnFailure {
		t.Errorf("record did not round-trip: got %+v, want dir/repo/branch/turn_failure from %+v", got, m)
	}
}
