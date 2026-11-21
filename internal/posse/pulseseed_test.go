//go:build !posse_arm2 && !posse_arm3

package posse

// The pulse family on the CONFIG SURFACE (ranger-base-1qi7c).
//
// examples/config.yaml is what `posse init` copies into every fresh instance,
// so it is the de facto instance spec — and until this bead it named no key of
// the `pulse_*` family at all: not `pulse_interval:`, which ARMS the ticker by
// its mere presence, not `pulse_persona:`, not `pulse_renag:`, and not ADR
// 0093 D2's `pulse_feed:`, which turns off the feed, the waiter probe, the
// typed reminder and the `pulse-unarmed` condition together. Four keys with
// teeth that nobody reading the surface could find, one of them an exit hatch
// an operator reaches for when the new channel misbehaves.
//
// The shape here is TestDispatchEpochIsDocumented's (epoch_test.go) — "a key
// with teeth that is not documented there is a key nobody can find" — with
// the family DERIVED rather than listed, because a list is how this gap
// happened: three keys were already undocumented when `pulse_feed:` was added
// to them, and nothing anywhere was holding a count. The keys come out of
// pulse.go's own source, so a fifth one reds this pin on the day it is read.
//
// WHAT THAT DERIVATION DOES NOT COVER, said here rather than left to be
// rediscovered: it reads ONE FILE, by name. Every `pulse_*` read in the tree
// is in pulse.go today (LoadPulseConfig and pulsePersona are the whole of
// it), and reading one named file is deliberately not an enumeration — this
// pin reds for the bead that edits pulse.go and for nobody else, which is why
// it needs no Makefile door (internal/treepins/treewidedoor_qa_test.go's
// class rule). A `pulse_*` key read from some other file would escape it, and
// so would every other config family's undocumented key: the general case is
// "every key the harness reads is documented in the seed or registered", a
// census nothing in this tree performs, and it is filed as its own bead
// rather than smuggled in under this one.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// pulseConfigKeySite is one `pulse_*` key literal pulse.go hands a config
// reader, with the line it is on, so a failure says where.
type pulseConfigKeySite struct {
	key  string
	line int
}

// pulseConfigKeys derives the family from pulse.go: every `pulse_`-prefixed
// string literal passed to one of the three flat-YAML readers, at the
// argument position that reader takes its KEY at.
//
// The positions disagree — `CfgGet(key, fallback)`, `YamlGet(path, key)`,
// `yamlHasKey(path, key)` — which is the kind of detail a rule stated in
// prose gets wrong, so each is named with its index.
func pulseConfigKeys(t *testing.T) []pulseConfigKeySite {
	t.Helper()
	// Relative, because `go test` runs this binary with the package
	// directory as its working directory: pulse.go is this package's own
	// source and needs no repo root (see the head comment on why that
	// matters for the door class).
	const src = "pulse.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("%s: %v — this pin derives the pulse config family from that file's own source", src, err)
	}
	keyArg := func(callee string) int {
		switch callee {
		case "CfgGet":
			return 0
		case "YamlGet", "yamlHasKey":
			return 1
		}
		return -1
	}
	callee := func(e ast.Expr) string {
		switch f := e.(type) {
		case *ast.Ident:
			return f.Name
		case *ast.SelectorExpr:
			return f.Sel.Name
		}
		return ""
	}
	seen := map[string]bool{}
	var out []pulseConfigKeySite
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		i := keyArg(callee(call.Fun))
		if i < 0 || len(call.Args) <= i {
			return true
		}
		lit, ok := call.Args[i].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		key, err := strconv.Unquote(lit.Value)
		if err != nil || !strings.HasPrefix(key, "pulse_") || seen[key] {
			return true
		}
		seen[key] = true
		out = append(out, pulseConfigKeySite{key: key, line: fset.Position(lit.Pos()).Line})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	// Positive witness: a pin over a derived set is satisfied by deriving
	// nothing, and a parse that found no call site would pass every
	// assertion below in silence. Four is the family as it stands —
	// pulse_feed, pulse_interval, pulse_persona, pulse_renag — and a tree
	// with fewer has either retired a key (fix this floor, and the seed) or
	// moved the reads somewhere this derivation cannot see, which is the
	// escape named in the head comment.
	if len(out) < 4 {
		t.Fatalf("derived %d pulse_* config keys from %s (%v), want at least 4 — the reads have moved out of that file, so this pin holds nothing about the family",
			len(out), src, out)
	}
	return out
}

// Every `pulse_*` key the harness reads is documented in the seed, exactly
// once, commented out, and its documented value states what the code does.
//
// The per-key claims are a SWITCH with no default arm: a key this file has
// nothing to say about is a failure, so the next pulse key cannot be admitted
// to the surface as an undocumented line OR as a documented line nobody
// decided the meaning of.
func TestPulseFamilyIsDocumentedInTheSeed(t *testing.T) {
	t.Parallel()
	cfg := seedConfigPath(t)
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)

	for _, site := range pulseConfigKeys(t) {
		// COMMENTED FIRST, because seedDocumentedValue reads comment lines:
		// an ARMED key is invisible to it, so a seed that uncommented one
		// would be reported below as a key nobody documented rather than as
		// the live key it is. The arming itself is property 1's
		// (TestSeedConfigArmsNothing); what is said here is why this pin went
		// quiet.
		if yamlHasKey(cfg, site.key) {
			t.Errorf("seed config declares %s: live — the pulse family ships commented out (for %s that line IS the arm switch), and an armed key is invisible to the documented-value check below",
				site.key, site.key)
		}
		value, lines := seedDocumentedValue(text, site.key)
		switch {
		case lines == 0:
			t.Errorf("pulse.go:%d reads %s: and examples/config.yaml documents no such line — that file is the config surface, so a key with teeth nobody can find is a key nobody will set (and this one is read by the watch loop, `posse status` and `posse pulse wait` alike)",
				site.line, site.key)
			continue
		case lines > 1:
			t.Errorf("examples/config.yaml documents %s: on %d commented lines — two documented values for one key, and a reader cannot tell which one the harness uses", site.key, lines)
			continue
		}
		if value == "" {
			t.Errorf("examples/config.yaml's %s: line carries no value, so it documents nothing", site.key)
			continue
		}

		switch site.key {
		case "pulse_interval":
			// The arm switch has no default to be right about (it is in
			// seedDocumentedDurationOutsideTheCensus for exactly that
			// reason), so what the line can be wrong about is its GRAMMAR:
			// a documented value LoadPulseConfig would refuse is a line
			// that disarms the pulse of whoever copies it, with a diagnostic
			// in a watch log they are not reading.
			if _, err := ParseInterval(value); err != nil {
				t.Errorf("examples/config.yaml documents pulse_interval: %q, and ParseInterval — the grammar LoadPulseConfig holds it to — refuses it: %v. An operator who copies that line arms a pulse that disarms itself", value, err)
			}
		case "pulse_renag":
			// The one pulse duration with a constant default, resolved
			// through the production reader. The seed's documented-default
			// census holds this too (TestSeedConfigDocumentedDurationDefaults
			// AreTheConstants, where the pairing is derived rather than
			// named); it is here as well because that pin is behind `make
			// seed-check` and this one is reachable from any `-run Pulse`,
			// which is the filter a seat working on the pulse actually types.
			dir := t.TempDir()
			p := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(p, []byte("pulse_renag: "+value+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var errw strings.Builder
			got := (&App{ConfigPath: p}).PulseRenag(&errw)
			if s := strings.TrimSpace(errw.String()); s != "" {
				t.Errorf("examples/config.yaml documents pulse_renag: %q and the reader names it rather than reading it: %s", value, s)
			}
			if got != DefaultPulseRenag {
				t.Errorf("examples/config.yaml documents pulse_renag: %q, which resolves to %s, and DefaultPulseRenag is %s — the line is a claim about the harness that the harness does not make", value, got, DefaultPulseRenag)
			}
		case "pulse_feed":
			// The hatch's default is ON, and the documented line must say
			// so: a seed that shipped the hatch's position as `off` would
			// hand every fresh instance ADR 0027's typed path and none of
			// ADR 0093's, with nothing on the surface to say the channel
			// existed.
			if pulseFeedOff(value) {
				t.Errorf("examples/config.yaml documents pulse_feed: %q, which pulseFeedOff reads as the HATCH — the default is ON (LoadPulseConfig), so the documented line states the opposite of what a fresh instance does", value)
			}
		case "pulse_persona":
			// A PLACEHOLDER, not a name — `coordinator:` and
			// `default_persona:` are documented the same way, and for the
			// reason the engine ships carrying no crew name at all
			// (rangerhq-gk4k, App.Coordinator): a seed naming a real persona
			// would point every fresh instance's pulse at somebody who does
			// not work there.
			if !strings.HasPrefix(value, "<") {
				t.Errorf("examples/config.yaml documents pulse_persona: %q — a persona-valued line in the seed is a placeholder (`<persona name>`, the spelling coordinator: and default_persona: use), because a real name here is one instance's crew shipped to every other one", value)
			}
		default:
			t.Errorf("pulse.go:%d reads %s:, examples/config.yaml documents it as %q, and this pin has no claim about that line — add a case above saying what the documented value is supposed to state (its default, its grammar, or that it is a placeholder). A documented value nothing compares goes stale for free, which is the whole reason this file exists",
				site.line, site.key, value)
		}
	}
}

// The reader's other half: a `pulse_renag:` that is not an interval is NAMED,
// and the default stands.
//
// It is the arm that was missing until ranger-base-1qi7c. The read used to be
// `if d, err := ParseInterval(v); err == nil` inside LoadPulseConfig, so a
// typo ran the shop at 30m with nothing said anywhere — and the seed now
// documents 30m as the default, which makes a silently-ignored value a line
// that looks obeyed. Every other tunable with a default says so on stderr
// (`attn_*`, `dispatch_epoch:`, `autostart_max_*`).
func TestPulseRenagNamesAValueThatIsNotAnInterval(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, c := range []struct {
		name  string
		value string
		want  bool // named on errw
	}{
		{"an interval is read", "5m", false},
		{"bare seconds are read", "600", false},
		{"absent is the default, silently", "", false},
		{"a unit nobody has", "5min", true},
		{"a word", "banana", true},
		{"zero is not a disarm", "0", true},
		{"negative", "-5m", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".yaml")
			body := ""
			if c.value != "" {
				body = "pulse_renag: " + c.value + "\n"
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			var errw strings.Builder
			got := (&App{ConfigPath: p}).PulseRenag(&errw)
			named := strings.Contains(errw.String(), "pulse_renag")
			if named != c.want {
				t.Errorf("pulse_renag: %q — named on stderr = %v, want %v (%q)", c.value, named, c.want, errw.String())
			}
			switch {
			case c.want && got != DefaultPulseRenag:
				t.Errorf("pulse_renag: %q was refused and the reader answered %s, not the default %s", c.value, got, DefaultPulseRenag)
			case !c.want && got <= 0:
				t.Errorf("pulse_renag: %q was accepted and the reader answered %s", c.value, got)
			}
		})
	}
}

// And the value reaches delivery through LoadPulseConfig, which is the only
// caller: a reader nothing is wired to would hold the seed's documented line
// and nothing else.
func TestLoadPulseConfigTakesItsRenagFromThatReader(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("pulse_interval: 2m\npulse_renag: 5min\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: p}
	var errw strings.Builder
	cfg, err := LoadPulseConfig(a, &errw)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Armed || cfg.Renag != DefaultPulseRenag {
		t.Errorf("a pulse_renag: the reader refuses must leave the pulse armed at the default: %+v", cfg)
	}
	if !strings.Contains(errw.String(), "pulse_renag") {
		t.Errorf("the typo did not reach the writer LoadPulseConfig was handed: %q", errw.String())
	}
}
