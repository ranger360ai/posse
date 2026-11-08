//go:build posse_arm3

package posse

// ranger-base-136b5 — the watch loop as a systemd user unit (ADR 0080 D5),
// and loop ownership as a config key (ADR 0080 amendment D9).
//
// ONE INPUT TABLE, THREE READERS. D9 names three readers of
// `autostart_via:` — plugin/autostart.sh's cfg(), the unit render, and
// `posse status`'s G7 — and asks for them to be pinned on one table. They
// are three because the mechanisms are three (bash at herdr's server start,
// Go at render, Go at a status run) and no one of them can call the others,
// which is exactly the shape that drifts: ranger-base-k3yd and
// ranger-base-fqfw are both the same class, where the hook's reader and
// yamlClean disagreed about a value an operator had typed and one surface
// reported a fleet armed while the other refused it.
//
// So the table below is the subject, and the four tests over it are four
// readings of one list. A row added there is a row every reader has to
// answer for; a reader that stops agreeing names the row.
//
// WHAT THE TABLE IS NOT. It is not the argv. The hook and the render both
// build `posse dispatch --watch …` out of this home's `autostart_*` keys,
// in two languages, and that duplication is held by a second table and
// TestSystemdUnitExecStartIsTheHooksArgv — compared argv to argv, because a
// renderer that cannot be ONE function (the hook is read fresh from the
// checkout by herdr and cannot call into Go) has to be held by a test
// instead.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// ─── the one input table ────────────────────────────────────────────────────

// autostartViaCase is one `autostart_via:` line as an operator types it and
// the verdict all three readers must reach over it.
type autostartViaCase struct {
	name string
	// line is appended to a config that is otherwise armed and valid; "" is
	// the key ABSENT, which is a row of its own and the common one.
	line string
	// owner is AutostartVia's answer. "" means it refuses — and a refusal
	// is a verdict, not an absence: the hook exits 1 and arms nothing, the
	// render refuses, and G7 says so.
	owner string
	// why records what the row is FOR, so a reader meeting a failure knows
	// which decision it is about.
	why string
	// disarmed is what the HOOK must say over this same line on a home with
	// no `autostart_interval:` at all — the one shape no row above can
	// reach, because viaHome and every run below are armed by design, which
	// is how the three readers came to diverge on it unseen
	// (ranger-base-hmet1, from ranger-base-zml8t).
	//
	// EXIT 0 FOR EVERY ROW, including the refusals: a refusal needs an arm
	// to refuse, and this hook arms nothing on such a home whatever the key
	// says. The three readers do not converge there and are not meant to —
	// AutostartVia reads a VALUE and refuses it wherever it appears, G7's
	// row is gated on the arm switch and says nothing at all, and the hook
	// is the only one of the three that runs on that home — so this column
	// is the table recording the split rather than a verdict derived from
	// `owner`.
	disarmed string
}

var autostartViaTable = []autostartViaCase{
	{"absent", "", AutostartViaHerdr,
		"the default and the common case: every home on a herdr-only host, unchanged",
		"disarmed (no autostart_interval:"},
	{"herdr, spelled", "autostart_via: herdr\n", AutostartViaHerdr,
		"the default said out loud",
		"disarmed (no autostart_interval:"},
	{"systemd", "autostart_via: systemd\n", AutostartViaSystemd,
		"the host D5 is about",
		"systemd owns"},
	// The four shapes both readers map to the empty string. This key HAS a
	// default, so empty is the default and not a broken arm — the
	// autostart_max_interval precedent and not the autostart_interval one.
	{"bare", "autostart_via:\n", AutostartViaHerdr,
		"a bare key is not told, and not told is the default (the key has one, unlike the arm switch)",
		"disarmed (no autostart_interval:"},
	{"whitespace", "autostart_via:   \n", AutostartViaHerdr,
		"yamlClean and cfg() both trim, so this is the bare key",
		"disarmed (no autostart_interval:"},
	{"null", "autostart_via: null\n", AutostartViaHerdr,
		"yamlGetLines maps a cleaned null to empty and cfg() does too (ranger-base-fqfw)",
		"disarmed (no autostart_interval:"},
	{"tilde", "autostart_via: ~\n", AutostartViaHerdr,
		"YAML's other spelling of unset",
		"disarmed (no autostart_interval:"},
	// The quotes, which are the whole of ranger-base-k3yd: yamlClean drops
	// a matched pair and cfg() was taught to, so YAML's own way of writing
	// a string must not read as a value posse cannot parse.
	{"quoted systemd", "autostart_via: \"systemd\"\n", AutostartViaSystemd,
		"a matched pair of double quotes goes, in both readers (ranger-base-k3yd)",
		"systemd owns"},
	{"quoted herdr", "autostart_via: \"herdr\"\n", AutostartViaHerdr,
		"the same, on the default",
		"disarmed (no autostart_interval:"},
	{"trailing comment", "autostart_via: systemd # the linux host\n", AutostartViaSystemd,
		"findComment's rule: a comment opens at whitespace + '#'",
		"systemd owns"},
	// And the refusals. Each is a name posse cannot read, and reading one
	// as the default is how D9's race gets armed on a host whose operator
	// meant the unit to own the loop.
	{"a near miss", "autostart_via: systemdd\n", "",
		"a typo in an owner must not be read as the default: that arms the hook on a systemd host",
		"nothing to arm here anyway"},
	{"wrong case", "autostart_via: Systemd\n", "",
		"the set is closed and compared exactly, as ValidMultiplexer compares",
		"nothing to arm here anyway"},
	{"a third mechanism", "autostart_via: launchd\n", "",
		"there are two mechanisms; a name for a third is a refusal, never a fallback",
		"nothing to arm here anyway"},
	{"prose", "autostart_via: the systemd one\n", "",
		"whitespace in the value is not one of the two names",
		"nothing to arm here anyway"},
}

// viaHome writes an armed config carrying one table row's line and returns
// an App over it. Armed, because every reader's interesting behaviour is
// behaviour over a home that would otherwise run a loop: a disarmed home
// stands down for a reason that is not this key's.
func viaHome(t *testing.T, line string) *App {
	t.Helper()
	home := t.TempDir()
	body := armed + line
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return &App{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.yaml"),
		StateDir:   filepath.Join(home, "state"),
	}
}

// READER 1 — AutostartVia, the Go reader the render and G7 both go through.
func TestAutostartViaTableIsReadByTheGoReader(t *testing.T) {
	t.Parallel()
	for _, tc := range autostartViaTable {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := viaHome(t, tc.line).AutostartVia()
			if tc.owner == "" {
				if err == nil {
					t.Fatalf("read %q as %q — the row says it is refused (%s)", tc.line, got, tc.why)
				}
				// A refusal that does not name the key and the value is a
				// refusal the deployer cannot act on.
				for _, want := range []string{AutostartViaKey, AutostartViaHerdr, AutostartViaSystemd} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("the refusal does not name %q: %v", want, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("refused %q: %v — the row says it reads as %q (%s)", tc.line, err, tc.owner, tc.why)
			}
			if got != tc.owner {
				t.Errorf("read %q as %q, want %q (%s)", tc.line, got, tc.owner, tc.why)
			}
		})
	}
}

// READER 2 — plugin/autostart.sh. The hook runs in a real second process
// over a real config, which is the only way to measure a reader written in
// another language (the autostart_test.go substrate, rangerhq-gir5).
//
// The verdict shape per owner:
//
//   - herdr: ARMS, exactly as it did before this key existed. The positive
//     control for the whole table — a stand-down compares equal to another
//     stand-down and would pin nothing.
//   - systemd: stands down BY NAME, exit 0, and asks posse nothing but the
//     liveness question. Exit 0 because nothing is wrong: the unit owns
//     this loop and the hook has no business arming it.
//   - refused: exit 1, arms nothing, and names the value. Same stand-down
//     as the broken arm below it in the script, for the same reason.
func TestAutostartViaTableIsReadByTheHook(t *testing.T) {
	t.Parallel()
	for _, tc := range autostartViaTable {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			switch tc.owner {
			case AutostartViaHerdr:
				// runArgv fails the test unless a loop was armed, so this
				// arm asserts by running at all.
				if got := runArgv(t, armed+tc.line); !strings.Contains(got, "dispatch --watch 30s") {
					t.Errorf("herdr owns this home and the hook armed %q (%s)", got, tc.why)
				}
			case AutostartViaSystemd:
				r := runStandDown(t, armed+tc.line)
				if r.code != 0 {
					t.Fatalf("exit %d — systemd owning the loop is not an error, it is this hook having nothing to do:\n%s", r.code, r.out)
				}
				if !probedOnly(r.calls) {
					t.Errorf("the hook asked posse for more than liveness on a home it does not own:\n%s", r.calls)
				}
				for _, want := range []string{"autostart_via: systemd", "systemd owns", "posse systemd"} {
					if !strings.Contains(r.out, want) {
						t.Errorf("the stand-down does not say %q — it has to name the key and the verb that owns the loop (%s):\n%s", want, tc.why, r.out)
					}
				}
			default:
				r := runStandDown(t, armed+tc.line)
				if r.code == 0 {
					t.Fatalf("exit 0 on an owner it cannot read — an arm that armed nothing must not report success (%s):\n%s", tc.why, r.out)
				}
				if !probedOnly(r.calls) {
					t.Errorf("the hook invoked posse over a config it refused:\n%s", r.calls)
				}
				value := strings.TrimSuffix(strings.TrimPrefix(tc.line, "autostart_via: "), "\n")
				if !strings.Contains(r.out, value) || !strings.Contains(r.out, "autostart_via") {
					t.Errorf("the refusal must quote the key and the value the deployer typed (%q):\n%s", value, r.out)
				}
			}
		})
	}
}

// READER 3 — `posse status`'s G7. The row's sentences NAME a mechanism, so
// the key decides which mechanism they name: a row written against the
// herdr hook on a systemd home would point the operator at a hook that is
// standing down on purpose and never mention the unit.
//
// Every row of the table is armed and valid and no loop holds the lock, so
// the row is loop-dead except where the owner itself is unreadable — which
// is arm-broken, and the most total kind: no mechanism will arm this home.
func TestAutostartViaTableIsReadByG7(t *testing.T) {
	for _, tc := range autostartViaTable {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			appendConfig(t, b.App, armed+tc.line)
			g := find(shopSet(t, govIn(t, b)), "G7")
			if g == nil {
				t.Fatalf("G7 did not fire over an armed home with no loop (%s)", tc.why)
			}
			if g.Class != GovUrgent {
				t.Errorf("G7 = %+v, want URGENT", *g)
			}
			switch tc.owner {
			case AutostartViaSystemd:
				if g.Key != "loop-dead" {
					t.Fatalf("G7 = %+v, want loop-dead", *g)
				}
				// The LEVER, which is the whole reason this row reads the
				// key: under systemd the unit is where a dead loop's reason
				// is written.
				if !strings.Contains(g.Detail, "systemctl --user status "+WatchUnitName(b.App)) {
					t.Errorf("the row does not name the unit to go look at: %q", g.Detail)
				}
			case AutostartViaHerdr:
				if g.Key != "loop-dead" {
					t.Fatalf("G7 = %+v, want loop-dead", *g)
				}
				if strings.Contains(g.Detail, "systemctl") {
					t.Errorf("herdr owns this loop and the row sent the operator to systemd: %q", g.Detail)
				}
			default:
				if g.Key != "arm-broken" {
					t.Fatalf("G7 = %+v, want arm-broken: an owner posse cannot read means no mechanism will ever arm this home (%s)", *g, tc.why)
				}
				if !strings.Contains(g.Detail, AutostartViaKey) {
					t.Errorf("the row does not name the key that is wrong: %q", g.Detail)
				}
				// The path reads the way every other row of this table
				// reads it — AbbrevHome, not the absolute home. Under a
				// fixture whose config is not below HOME the two spellings
				// are the same string, so this is the assertion that holds
				// on both and the absolute-path half would not be one.
				if !strings.Contains(g.Detail, AbbrevHome(b.App.ConfigPath)) {
					t.Errorf("the row does not name the config file the way its siblings do: %q", g.Detail)
				}
				if strings.Contains(g.Detail, "is armed") {
					t.Errorf("a refused owner is not armed: %q", g.Detail)
				}
			}
		})
	}
}

// READER 2b — the render, which is reader 2's other half: it refuses unless
// the home says systemd, which is the D9 decision stated from the other
// side. A unit beside an arming hook is not two loops, it is one loop in the
// place D5 said it must not be.
func TestAutostartViaTableIsReadByTheUnitRender(t *testing.T) {
	t.Parallel()
	for _, tc := range autostartViaTable {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := viaHome(t, tc.line+"autostart_multiplexer_unit: mux-server.service\n")
			u, err := RenderWatchUnit(a, "/usr/local/bin/posse")
			switch tc.owner {
			case AutostartViaSystemd:
				if err != nil {
					t.Fatalf("refused a home that says systemd: %v (%s)", err, tc.why)
				}
				if !strings.Contains(u.Body, "ExecStart=/usr/local/bin/posse dispatch --watch 30s") {
					t.Errorf("rendered body has no ExecStart for the loop:\n%s", u.Body)
				}
			case AutostartViaHerdr:
				if err == nil {
					t.Fatalf("rendered a unit for a home herdr's hook arms — that unit loses the lock to the hook on every start (%s)", tc.why)
				}
				if !strings.Contains(err.Error(), AutostartViaKey+": "+AutostartViaSystemd) {
					t.Errorf("the refusal does not name the key that would change the answer: %v", err)
				}
			default:
				if err == nil {
					t.Fatalf("rendered a unit over an owner it cannot read (%s)", tc.why)
				}
			}
		})
	}
}

// THE DISARMED HOME — the shape the table could not reach, because viaHome
// and every reader arm above are armed by design and `autostartViaTable`'s
// own comment says so. Four readings of the table's `disarmed` column, and
// what they measure is the SPLIT: on a home that arms nothing the three
// readers do different things, and this is where each one's is written down
// (ranger-base-hmet1, from ranger-base-zml8t).
//
// What each does, and why it is not the others:
//
//   - AutostartVia reads a VALUE. It refuses a name it cannot read wherever
//     it appears, armed or not, because nothing about the arm makes `launchd`
//     a mechanism.
//   - the HOOK is the only one of the three that runs on such a home, and it
//     arms nothing there whatever the key says — so a refusal has nothing to
//     refuse. It names the value and exits 0, the disarm's own exit code.
//   - the RENDER refuses, for the ARM's reason on a systemd home and the
//     owner's on every other: a unit over a home with no interval is a unit
//     that dies in the journal.
//   - G7 says NOTHING. Its whole row is gated on the arm switch, deliberately
//     (ranger-base-i6h): the row whose job is to be believed does not fire
//     over a config nothing will ever arm from.
//
// SERIAL, like the armed G7 reading it is the other half of: this reaches
// govNow through govIn, which `go run ./cmd/testparallel ./internal/posse
// extra` reports as package-level state (ADR 0047 D3). The fixture is read
// only and each row's own state is its own t.TempDir, so a parallelOK
// clearance would be arguable — but the sibling reader took the serial
// answer, and one table whose two readings disagree about their own safety
// costs more to read than these seconds cost to run (1.5s, MEASURED).
func TestAutostartViaTableOnADisarmedHome(t *testing.T) {
	for _, tc := range autostartViaTable {
		if tc.disarmed == "" {
			t.Errorf("%s: the table's disarmed column is empty — every row has a verdict on a home that arms nothing", tc.name)
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			// READER 1 — the value reader, which the arm cannot move.
			got, err := disarmedViaHome(t, tc.line).AutostartVia()
			if tc.owner == "" {
				if err == nil {
					t.Errorf("read %q as %q on a disarmed home — a name posse cannot read is not a name wherever it appears (%s)", tc.line, got, tc.why)
				}
			} else if err != nil || got != tc.owner {
				t.Errorf("read %q as %q/%v on a disarmed home and as %q when armed — this reader reads a value, not an arm", tc.line, got, err, tc.owner)
			}

			// READER 2 — the hook, the only one that runs here. Exit 0 and
			// nothing armed, and the value still named.
			r := runStandDown(t, tc.line)
			if r.code != 0 {
				t.Errorf("exit %d on a home that arms nothing — this hook would arm nothing here whatever %s said, so there is no arm to refuse (%s):\n%s", r.code, AutostartViaKey, tc.why, r.out)
			}
			if !probedOnly(r.calls) {
				t.Errorf("the hook asked posse for more than liveness on a disarmed home:\n%s", r.calls)
			}
			if !strings.Contains(r.out, tc.disarmed) {
				t.Errorf("the stand-down does not say %q — the table's disarmed column is what this home's operator reads (%s):\n%s", tc.disarmed, tc.why, r.out)
			}
			if tc.owner == "" {
				// Named, not swallowed: standing down quietly over a value
				// nobody can read is how the typo survives to the day the
				// home is armed.
				value := strings.TrimSuffix(strings.TrimPrefix(tc.line, "autostart_via: "), "\n")
				if !strings.Contains(r.out, value) || !strings.Contains(r.out, AutostartViaKey) {
					t.Errorf("the stand-down must still quote the key and the value the deployer typed (%q):\n%s", value, r.out)
				}
			}

			// READER 2b — the render, which refuses every row of this table
			// on such a home, and on a systemd home for the ARM.
			a := disarmedViaHome(t, tc.line+"autostart_multiplexer_unit: mux-server.service\n")
			u, err := RenderWatchUnit(a, "/usr/local/bin/posse")
			if err == nil {
				t.Fatalf("rendered a unit for a home with no interval — `dispatch --watch` has no default and the unit would die in the journal:\n%s", u.Body)
			}
			if tc.owner == AutostartViaSystemd && !strings.Contains(err.Error(), "autostart_interval") {
				t.Errorf("the refusal on a systemd home does not name the arm switch, which is the key that fixes it: %v", err)
			}

			// READER 3 — G7, silent.
			b, _ := newTestBackend(t)
			appendConfig(t, b.App, tc.line)
			if g := find(shopSet(t, govIn(t, b)), "G7"); g != nil {
				t.Errorf("G7 = %+v over a home that arms nothing — the row is gated on the arm switch on purpose (ranger-base-i6h)", *g)
			}
		})
	}
}

// disarmedViaHome is viaHome with the arm switch left out: the same config,
// minus the one key whose presence arms the loop. Built by removal rather
// than written out, so it cannot drift from the armed fixture every reader
// above is measured on (the configWithout rule, ranger-base-orq4e).
func disarmedViaHome(t *testing.T, line string) *App {
	t.Helper()
	a := viaHome(t, line)
	body, err := os.ReadFile(a.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	cut := configWithout(string(body), "autostart_interval")
	if err := os.WriteFile(a.ConfigPath, []byte(cut), 0o644); err != nil {
		t.Fatal(err)
	}
	if yamlHasKey(a.ConfigPath, "autostart_interval") {
		t.Fatalf("premise: this home still names the arm switch:\n%s", cut)
	}
	return a
}

// ─── the argv the hook and the render both build ────────────────────────────

// autostartArgvConfigs are the `autostart_*` shapes whose argv the two
// builders must agree on: every key, its absence, its default, and every
// value one of them names and drops. They are configs rather than per-key
// rows because the ORDER of the flags is part of the agreement and only a
// whole config shows it.
var autostartArgvConfigs = []struct {
	name string
	body string
}{
	{"the arm alone", armed},
	{"a cap", armed + "autostart_max_interval: 40m\n"},
	{"an empty cap, which is no cap", armed + "autostart_max_interval:\n"},
	{"an unreadable cap, named and dropped", armed + "autostart_max_interval: banana\n"},
	{"a count", armed + "autostart_max_beads: 7\n"},
	{"no cap on the count, said out loud", armed + "autostart_max_beads: 0\n"},
	{"an unreadable count, named and replaced", armed + "autostart_max_beads: lots\n"},
	{"resume off", armed + "autostart_resume: false\n"},
	{"resume off, spelled no", armed + "autostart_resume: no\n"},
	{"resume off, spelled 0", armed + "autostart_resume: 0\n"},
	{"resume on, spelled", armed + "autostart_resume: true\n"},
	{"an unreadable resume, named and replaced", armed + "autostart_resume: maybe\n"},
	{"dry", armed + "autostart_dry_run: true\n"},
	{"dry, spelled 1", armed + "autostart_dry_run: 1\n"},
	{"not dry", armed + "autostart_dry_run: false\n"},
	{"every key at once", armed + "autostart_max_interval: 40m\nautostart_max_beads: 7\nautostart_resume: false\nautostart_dry_run: true\n"},
	{"a quoted interval, which is the interval", "autostart_interval: \"30s\"\n"},
}

// hookWatchArgv pulls the loop's own argv out of what the hook asked posse
// for: `posse new <session> … --cmd <binary> dispatch --watch …`. The
// BINARY is dropped and everything from `dispatch` on is kept, because the
// two builders are being compared on the loop's flags and not on where each
// of them found a posse.
func hookWatchArgv(t *testing.T, calls string) string {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(calls), "\n") {
		_, cmd, ok := strings.Cut(line, " --cmd ")
		if !ok {
			continue
		}
		_, argv, ok := strings.Cut(cmd, " dispatch ")
		if !ok {
			t.Fatalf("the hook's --cmd is not a dispatch line: %q", cmd)
		}
		return "dispatch " + argv
	}
	t.Fatalf("the hook asked posse for no session at all:\n%s", calls)
	return ""
}

// THE ARGV IS ONE ARGV, built twice. plugin/autostart.sh cannot call into Go
// (herdr reads it fresh from the checkout at server start) and the render
// cannot call into bash and stay deterministic, so this shop's "one
// renderer" rule cannot be kept here — and what replaces it is this: both
// builders over one list of configs, compared argv to argv.
//
// The hook arm is the one that cannot be faked. It runs bash over the real
// script in a real process, so a `case` arm or a default that moves in
// either language shows up as a difference here and names the config.
func TestSystemdUnitExecStartIsTheHooksArgv(t *testing.T) {
	t.Parallel()
	for _, tc := range autostartArgvConfigs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// runStandDown and not runArgv: half these configs are the
			// named-and-dropped arms, and runArgv fails a run that
			// complained at all. hookWatchArgv is the arming guard here —
			// it fatals when the hook asked posse for no session.
			r := runStandDown(t, tc.body)
			if r.code != 0 {
				t.Fatalf("the hook refused a config this table says it arms (exit %d):\n%s", r.code, r.out)
			}
			want := hookWatchArgv(t, r.calls)

			a := viaHome(t, "")
			body := tc.body + "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"
			if err := os.WriteFile(a.ConfigPath, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			u, err := RenderWatchUnit(a, "/opt/posse")
			if err != nil {
				t.Fatalf("render refused a config the hook armed: %v", err)
			}
			got := ""
			for _, line := range strings.Split(u.Body, "\n") {
				if rest, ok := strings.CutPrefix(line, "ExecStart=/opt/posse "); ok {
					got = rest
				}
			}
			if got == "" {
				t.Fatalf("rendered unit has no ExecStart naming the binary:\n%s", u.Body)
			}
			if got != want {
				t.Errorf("the unit and the hook build different loops out of one config\nunit: %s\nhook: %s\nconfig:\n%s", got, want, body)
			}
		})
	}
}

// And the notes are the hook's stderr lines: a key with a default whose
// value posse cannot read is NAMED AND DROPPED, never silently obeyed and
// never fatal (ranger-base-x8y8). The render says the same thing in the same
// place in the same words, or a deployer reading one surface learns
// something the other does not say.
func TestSystemdUnitNamesTheValuesItDrops(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		line string
		want string
	}{
		{"autostart_max_interval: banana\n", "is not an interval"},
		{"autostart_max_beads: lots\n", "is not a count"},
		{"autostart_resume: maybe\n", "is not true/false"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			a := viaHome(t, "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"+tc.line)
			u, err := RenderWatchUnit(a, "/opt/posse")
			if err != nil {
				t.Fatalf("a value with a default to fall back on must not be fatal: %v", err)
			}
			if !strings.Contains(strings.Join(u.Notes, "\n"), tc.want) {
				t.Errorf("the render dropped a value without naming it (want %q): %q", tc.want, u.Notes)
			}
			// The hook says it too, which is what keeps the two in words
			// as well as in argv.
			r := runStandDown(t, armed+tc.line)
			if !strings.Contains(r.out, tc.want) {
				t.Errorf("the hook does not say %q over the same value:\n%s", tc.want, r.out)
			}
		})
	}
}

// ─── the unit itself ────────────────────────────────────────────────────────

// unitDirectives is the rendered file with its comments and blank lines
// taken out — the unit as systemd reads it. Prose churns and this does not,
// so the pin below is over this and not over the whole body.
func unitDirectives(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// THE RENDER IS DETERMINISTIC AND PINNED — the bead's own done condition.
//
// Pinned over the DIRECTIVES rather than the bytes: the file's leading
// comment is prose for whoever opens it on the host and will be edited by
// beads that have nothing to do with the mechanism, while this list is the
// mechanism. Every line of it is a decision ADR 0080 D5 states —
// After=/Requires= the multiplexer's unit, ExecStart carrying the binary's
// absolute path and the home's argv, Restart=on-failure — plus the two that
// make a user unit work at all (WantedBy=default.target, and RHQ_HOME, since
// a unit inherits no shell).
func TestSystemdUnitRenderIsPinned(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	cfg := "instance: crewa\nmultiplexer: rex\nautostart_via: systemd\n" +
		"autostart_multiplexer_unit: rex-server.service\n" +
		"autostart_interval: 5m\nautostart_max_interval: 40m\nautostart_max_beads: 3\n" +
		"autostart_dir: /srv/crewa\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{Home: home, ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}

	// renderWatchUnitOn and not RenderWatchUnit: since ranger-base-ib61s the
	// unit carries a PATH, and the exported render reads this process's.
	// Naming it is what keeps the pin a pin — and `t.Setenv` is not the
	// alternative, because it panics in a parallel test and the whole file
	// is parallel.
	u, err := renderWatchUnitOn(a, "/usr/local/bin/posse", "/opt/crew/bin:/usr/bin")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"[Unit]",
		"Description=posse dispatch --watch (crewa)",
		"After=rex-server.service",
		"Requires=rex-server.service",
		"[Service]",
		"Type=simple",
		"WorkingDirectory=/srv/crewa",
		"Environment=RHQ_HOME=" + home,
		// The named PATH first, in its own order, then systemd's own
		// default for the dirs it did not already name — /usr/bin is in
		// both and appears once, where the deployer's PATH put it.
		"Environment=PATH=/opt/crew/bin:/usr/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/sbin:/bin",
		"ExecStart=/usr/local/bin/posse dispatch --watch 5m --max-interval 40m -n 3 --resume",
		"Restart=on-failure",
		"RestartSec=5s",
		"[Install]",
		"WantedBy=default.target",
	}
	got := unitDirectives(u.Body)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rendered directives:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Deterministic: the same home and the same PATH render the same bytes,
	// so a unit written twice is not a unit that changed and `systemctl
	// --user daemon-reload` is only ever needed when something really moved.
	// (A PATH that MOVED does move the unit, which is why --write refuses to
	// replace a unit that says something else without --force.)
	again, err := renderWatchUnitOn(a, "/usr/local/bin/posse", "/opt/crew/bin:/usr/bin")
	if err != nil {
		t.Fatal(err)
	}
	if again.Body != u.Body {
		t.Error("two renders of one home differ — the unit is not deterministic")
	}
	if again.Name != u.Name || again.Path != u.Path {
		t.Errorf("the unit's own name moved between renders: %q/%q then %q/%q", u.Name, u.Path, again.Name, again.Path)
	}
}

// ─── the unit's PATH (ranger-base-ib61s) ────────────────────────────────────

// unitPathDirs pulls the Environment=PATH= value back out of a rendered unit
// and splits it the way systemd would — on `:`, which is the TARGET's
// separator and not this process's.
func unitPathDirs(t *testing.T, body string) []string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(line, "Environment=PATH="); ok {
			return strings.Split(strings.Trim(rest, `"`), ":")
		}
	}
	t.Fatalf("the unit carries no Environment=PATH= at all — a user manager reads no profile, so without one the loop cannot find a bd in ~/.local/bin (ranger-base-ib61s):\n%s", body)
	return nil
}

// THE BEAD'S OWN DONE CONDITION: a rendered unit's PATH finds bd.
//
// MEASURED 2026-11-08 on the ADR 0080 host (offid leg 3), which is what this
// pin is for: the unit carried Environment=RHQ_HOME and an absolute
// ExecStart and no PATH, a systemd user manager's PATH does not include
// ~/.local/bin (it reads no shell profile), INSTALL's bd step installs bd
// exactly there — so `posse dispatch --watch` exited 1 at once with
// NoBdLine and Restart=on-failure looped it every RestartSec.
//
// The fixture is that host: a bd and a git in a dir no default PATH names,
// first on the PATH the deployer rendered from. The assertion is not "the
// string is in the file" but "resolving `bd` against what systemd will read
// finds the bd this render saw" — the same question pathDirsLookup answers
// for the note, asked here through the FILE.
func TestSystemdUnitPathFindsBd(t *testing.T) {
	t.Parallel()
	localBin := t.TempDir() // stands for ~/.local/bin
	for _, name := range []string{"bd", "git"} {
		if err := WriteExecutable(filepath.Join(localBin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	base := "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"
	u, err := renderWatchUnitOn(viaHome(t, base), "/opt/posse", localBin+":/usr/bin")
	if err != nil {
		t.Fatal(err)
	}
	dirs := unitPathDirs(t, u.Body)
	if got := pathDirsLookup(dirs, "bd"); got != filepath.Join(localBin, "bd") {
		t.Errorf("`bd` resolves to %q on the unit's PATH, want the one this render found at %q — a unit whose PATH cannot answer `bd` exits %q on its first pass and systemd restarts it every RestartSec:\nPATH=%s", got, filepath.Join(localBin, "bd"), NoBdLine, strings.Join(dirs, ":"))
	}
	if got := pathDirsLookup(dirs, "git"); got != filepath.Join(localBin, "git") {
		t.Errorf("`git` resolves to %q on the unit's PATH, want %q", got, filepath.Join(localBin, "git"))
	}
	// And the render says nothing about a missing child, because nothing is
	// missing: the note and the file must not disagree.
	for _, n := range u.Notes {
		if strings.Contains(n, "answers to no") {
			t.Errorf("the render warned about a child its own PATH resolves: %q", n)
		}
	}

	// SETTING PATH SUBTRACTS systemd's default, so the render adds it back:
	// a unit naming only the render host's dirs would lose every binary in
	// /usr/bin to fix one in ~/.local/bin.
	for _, d := range systemdFallbackPath {
		if !slices.Contains(dirs, d) {
			t.Errorf("the unit's PATH drops %s, which a unit with NO Environment=PATH= would have had — Environment=PATH= replaces the manager's default rather than adding to it:\n%s", d, strings.Join(dirs, ":"))
		}
	}
}

// THE OTHER HALF, over the pre-flight directly rather than the file: the
// notes are the only place a deployer learns at render what the journal
// would have told them at 5s intervals.
//
// unitSearchPathMisses and not a render, because a render always appends
// systemdFallbackPath — so a box with a real /usr/bin/bd would make the
// negative arm pass for the wrong reason, and a box without one would make
// it pass for the right reason today and silently stop testing anything the
// day bd is packaged. A named dir list has the same answer everywhere.
func TestSystemdUnitPathNamesTheChildrenItCannotResolve(t *testing.T) {
	t.Parallel()
	empty := t.TempDir()
	miss := unitSearchPathMisses([]string{empty})
	if len(miss) != len(unitSearchPathChildren) {
		t.Fatalf("a PATH of one empty dir resolves %d of %d children: %q", len(unitSearchPathChildren)-len(miss), len(unitSearchPathChildren), miss)
	}
	joined := strings.Join(miss, "\n")
	for _, c := range unitSearchPathChildren {
		if !strings.Contains(joined, "`"+c.name+"`") {
			t.Errorf("the render drops a child without naming it (want %q):\n%s", c.name, joined)
		}
	}
	// The bd sentence quotes the line the loop would actually print, so the
	// render's warning and the journal's error are searchable as one string.
	if !strings.Contains(joined, NoBdLine) {
		t.Errorf("the bd miss does not quote %q — that is the sentence the deployer will read in the journal:\n%s", NoBdLine, joined)
	}

	full := t.TempDir()
	for _, c := range unitSearchPathChildren {
		if err := WriteExecutable(filepath.Join(full, c.name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := unitSearchPathMisses([]string{full}); got != nil {
		t.Errorf("a PATH that answers every child still warned: %q", got)
	}
}

// A unit is NOBODY'S SEAT (ADR 0077 D3), and the render owns that rule
// rather than trusting the caller to have applied it: a gates bin dir on the
// loop's PATH puts a persona's shims in front of posse's own bd, which is
// ranger-base-c33j6's deadlock in a process no persona typed in. A relative
// element is the hazard this render would INTRODUCE — WorkingDirectory= is a
// crew's own repo — so it goes the same way. Both are named, never swallowed.
func TestSystemdUnitPathDropsWhatAUnitMustNotCarry(t *testing.T) {
	t.Parallel()
	const gates = "/home/u/.posse/personas/a-persona/gates/bin"
	base := "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"
	u, err := renderWatchUnitOn(viaHome(t, base), "/opt/posse", gates+":bin:.:/opt/crew/bin::/usr/bin")
	if err != nil {
		t.Fatal(err)
	}
	dirs := unitPathDirs(t, u.Body)
	notes := strings.Join(u.Notes, "\n")
	for _, bad := range []string{gates, "bin", "."} {
		if slices.Contains(dirs, bad) {
			t.Errorf("the unit's PATH carries %q:\n%s", bad, strings.Join(dirs, ":"))
		}
		if !strings.Contains(notes, bad) {
			t.Errorf("the render dropped %q without naming it:\n%s", bad, notes)
		}
	}
	if !slices.Contains(dirs, "/opt/crew/bin") {
		t.Errorf("the unit's PATH lost a perfectly good dir while dropping the bad ones:\n%s", strings.Join(dirs, ":"))
	}
	// The empty element is systemd's own "the current directory" too, and it
	// is dropped by the same split the gates rule reads — silently, because
	// an empty element is a trailing colon and not something anybody typed.
	if slices.Contains(dirs, "") {
		t.Errorf("the unit's PATH carries an empty element, which systemd reads as the working directory:\n%q", strings.Join(dirs, ":"))
	}
}

// Environment= is NOT a command line: `%` is still a specifier there and `$`
// is not re-read, which is unitArg's split and is pinned for the other two
// directives in TestSystemdUnitDoublesADollarOnTheCommandLineOnly. The PATH
// is the third value to go through it, and the first that is a LIST — so the
// pin is that the whole value is escaped and quoted once, not per element.
func TestSystemdUnitPathIsEscapedLikeADirective(t *testing.T) {
	t.Parallel()
	base := "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"
	u, err := renderWatchUnitOn(viaHome(t, base), "/opt/posse", "/opt/100%/bin:/opt/my crew/bin:/opt/po$se/bin")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, line := range strings.Split(u.Body, "\n") {
		if rest, ok := strings.CutPrefix(line, "Environment=PATH="); ok {
			got = rest
		}
	}
	for _, want := range []string{"/opt/100%%/bin", "/opt/po$se/bin"} {
		if !strings.Contains(got, want) {
			t.Errorf("Environment=PATH=%s carries no %q — `%%` is a specifier in every directive and `$` is re-read only on a command line", got, want)
		}
	}
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Errorf("Environment=PATH=%s — a value with whitespace in it is quoted once, around the whole list", got)
	}
}

// The name carries the home's `instance:` tag, which is what keeps two
// homes' units apart in the one directory a user account has (ADR 0080 D12).
// An untagged home still renders — a single-home host is a legitimate reader
// — and says so, because the second untagged home would silently render the
// first's name.
func TestSystemdUnitNameCarriesTheInstanceTag(t *testing.T) {
	t.Parallel()
	base := "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"

	tagged := viaHome(t, base+"instance: crewb\n")
	u, err := RenderWatchUnit(tagged, "/opt/posse")
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "posse-dispatch-crewb.service" {
		t.Errorf("unit name %q does not carry the instance tag", u.Name)
	}
	if u.Path != filepath.Join(WatchUnitDir(), u.Name) {
		t.Errorf("unit path %q is not under the user's unit dir %q", u.Path, WatchUnitDir())
	}
	if n := WatchUnitName(tagged); n != u.Name {
		t.Errorf("WatchUnitName says %q and the render says %q — G7 names the unit through the first of those", n, u.Name)
	}

	untagged := viaHome(t, base)
	u2, err := RenderWatchUnit(untagged, "/opt/posse")
	if err != nil {
		t.Fatalf("an untagged home is procedure (D8), not a refusal: %v", err)
	}
	if u2.Name != "posse-dispatch.service" {
		t.Errorf("untagged unit name %q", u2.Name)
	}
	if !strings.Contains(strings.Join(u2.Notes, "\n"), "names no instance:") {
		t.Errorf("an untagged home renders a name that is not its own and the render did not say so: %q", u2.Notes)
	}
}

// A BROKEN ARM REFUSES AT RENDER — the bead's other done condition, and the
// same three shapes plugin/autostart.sh refuses by name. `dispatch --watch`
// has no default interval and dies on an empty argument, so the only choice
// is where that failure lands: here, or in a unit that restarts five times
// and goes to `failed` with nothing in the journal about the config.
func TestSystemdUnitRefusesABrokenArm(t *testing.T) {
	t.Parallel()
	base := "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"
	for name, arm := range map[string]string{
		"absent":       "",
		"bare":         "autostart_interval:\n",
		"whitespace":   "autostart_interval:   \n",
		"null":         "autostart_interval: null\n",
		"tilde":        "autostart_interval: ~\n",
		"quoted empty": "autostart_interval: \"\"\n",
		"not time":     "autostart_interval: banana\n",
		"zero":         "autostart_interval: 0\n",
		"negative":     "autostart_interval: -5m\n",
		"spaced":       "autostart_interval: 30 s\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(base+arm), 0o644); err != nil {
				t.Fatal(err)
			}
			a := &App{Home: home, ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
			u, err := RenderWatchUnit(a, "/opt/posse")
			if err == nil {
				t.Fatalf("rendered a unit over a broken arm:\n%s", u.Body)
			}
			if !strings.Contains(err.Error(), "autostart_interval") {
				t.Errorf("the refusal does not name the key: %v", err)
			}
		})
	}
}

// The ordering D5 makes load-bearing has no default, so its absence is a
// refusal: "a unit that cannot start in a world with no socket" is the
// property, and a unit with no After= does not have it. posse has never
// measured what either multiplexer's Linux unit is called, and the two ways
// a guess can be wrong fail in opposite directions — a name no unit wears
// refuses the start, a name another unit wears orders the loop behind the
// wrong process.
func TestSystemdUnitRefusesWithoutTheMultiplexersUnit(t *testing.T) {
	t.Parallel()
	for name, line := range map[string]string{
		"absent":           "",
		"bare":             "autostart_multiplexer_unit:\n",
		"null":             "autostart_multiplexer_unit: null\n",
		"no unit type":     "autostart_multiplexer_unit: rex-server\n",
		"a path":           "autostart_multiplexer_unit: /usr/lib/systemd/user/rex.conf\n",
		"two words":        "autostart_multiplexer_unit: rex server.service\n",
		"the suffix alone": "autostart_multiplexer_unit: .service\n",
		// A `%` IS NOT A UNIT NAME. systemd expands specifiers in
		// After=/Requires= — its own templates rely on it — and the unit
		// rendered here is not a template, so `%i` expands to the empty
		// instance and `%n` to this unit's own name: the loop ends up
		// ordered behind a unit nobody typed, which is the second of the
		// two failures AutostartUnitKey's no-default decision is written to
		// prevent. Refused rather than escaped, because a doubled `%%` is a
		// unit nobody has either (ranger-base-hmet1, from
		// ranger-base-zml8t).
		"an instance specifier": "autostart_multiplexer_unit: rex%i.service\n",
		"this unit's own name":  "autostart_multiplexer_unit: rex%n.service\n",
		"a percent in the stem": "autostart_multiplexer_unit: 100%.service\n",
		"an escaped percent":    "autostart_multiplexer_unit: rex%%i.service\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := viaHome(t, "autostart_via: systemd\n"+line)
			if u, err := RenderWatchUnit(a, "/opt/posse"); err == nil {
				t.Fatalf("rendered a unit ordered behind nothing:\n%s", u.Body)
			} else if !strings.Contains(err.Error(), AutostartUnitKey) {
				t.Errorf("the refusal does not name the key: %v", err)
			}
		})
	}
	// The positive control: every unit type systemd has is a name this
	// accepts, so the check above is refusing a shape and not a suffix list
	// that happens to be short.
	for _, unit := range []string{"rex-server.service", "herdr.socket", "multi-user.target", "rex@main.service"} {
		t.Run("accepts/"+unit, func(t *testing.T) {
			t.Parallel()
			a := viaHome(t, "autostart_via: systemd\nautostart_multiplexer_unit: "+unit+"\n")
			u, err := RenderWatchUnit(a, "/opt/posse")
			if err != nil {
				t.Fatalf("refused a unit name systemctl prints: %v", err)
			}
			if !strings.Contains(u.Body, "After="+unit) || !strings.Contains(u.Body, "Requires="+unit) {
				t.Errorf("the unit is not ordered behind %q:\n%s", unit, u.Body)
			}
		})
	}
}

// systemd reads `%` as a specifier escape and splits an unquoted argument on
// whitespace, so a home or a binary under such a path would render a unit
// that runs something else — or nothing. Both are doubled and quoted.
//
// The control is the plain case: a path with neither must come through
// unquoted, or this pin would pass over a renderer that quoted everything
// and told us nothing about the ones that need it.
func TestSystemdUnitEscapesWhatSystemdWouldReread(t *testing.T) {
	t.Parallel()
	for name, dir := range map[string]string{
		"a percent": "/srv/100%/crew",
		"a space":   "/srv/my crew",
		"both":      "/srv/my 50% crew",
		"plain":     "/srv/crew",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := viaHome(t, "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\nautostart_dir: "+dir+"\n")
			u, err := RenderWatchUnit(a, "/opt/posse")
			if err != nil {
				t.Fatal(err)
			}
			var got string
			for _, line := range strings.Split(u.Body, "\n") {
				if rest, ok := strings.CutPrefix(line, "WorkingDirectory="); ok {
					got = rest
				}
			}
			switch {
			case !strings.Contains(dir, "%") && !strings.Contains(dir, " "):
				if got != dir {
					t.Errorf("WorkingDirectory=%q — a path needing nothing must be rendered as itself", got)
				}
				return
			case strings.Contains(got, "%") && !strings.Contains(got, "%%"):
				t.Errorf("WorkingDirectory=%q — a literal %% must be doubled or systemd reads it as a specifier", got)
			}
			if strings.Contains(dir, " ") && !strings.HasPrefix(got, `"`) {
				t.Errorf("WorkingDirectory=%q — a path with whitespace must be quoted or systemd reads two arguments", got)
			}
		})
	}
}

// A COMMAND LINE IS THE ONE PLACE `$` IS RE-READ. systemd.service(5) gives
// ExecStart basic environment-variable substitution and spells a literal
// dollar `$$`; WorkingDirectory= and Environment= are not command lines and
// substitute nothing, so a `$$` written into either would reach the process
// as two characters. Both halves are pinned here, in one test, because the
// pin is the DIVERGENCE: a renderer that doubled everywhere and one that
// doubled nowhere each pass half of this (ranger-base-hmet1, from
// ranger-base-zml8t).
//
// What is reachable, stated: of everything in this ExecStart only the
// binary's own path can carry a `$` at all — every config value in the argv
// is ParseInterval-validated or digits-only — and the unit SETS
// Environment=RHQ_HOME=, so a `${RHQ_HOME}` in that path would expand
// against a variable this very file defines. Whether systemd substitutes
// inside argv[0] at all is REPORTED from the man page and not MEASURED (no
// systemd on darwin); doubling is right either way for a literal dollar.
func TestSystemdUnitDoublesADollarOnTheCommandLineOnly(t *testing.T) {
	t.Parallel()
	base := "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"

	for name, tc := range map[string]struct{ exe, want string }{
		"a bare dollar":     {"/opt/po$se/posse", "/opt/po$$se/posse"},
		"a braced variable": {"/opt/po${RHQ_HOME}sse/posse", "/opt/po$${RHQ_HOME}sse/posse"},
		// Quoting does not stand the substitution down — it happens after
		// the line is split — so a path needing both gets both.
		"a dollar and a space": {"/opt/my crew/po$se/posse", `"/opt/my crew/po$$se/posse"`},
		// The control: a path needing nothing is rendered as itself, or this
		// pin would pass over a renderer that mangled every path equally.
		"plain": {"/opt/posse/posse", "/opt/posse/posse"},
	} {
		t.Run("ExecStart/"+name, func(t *testing.T) {
			t.Parallel()
			u, err := RenderWatchUnit(viaHome(t, base), tc.exe)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(u.Body, "ExecStart="+tc.want+" dispatch ") {
				t.Errorf("exe %q rendered no ExecStart=%s — systemd re-reads `$` on a command line and a literal one is written `$$`:\n%s", tc.exe, tc.want, u.Body)
			}
		})
	}

	// THE OTHER HALF. The same dollar in the two directives that are not
	// command lines, where doubling it would be the bug: a home and a
	// working directory with a `$` in them come through verbatim.
	t.Run("the directives that are not command lines", func(t *testing.T) {
		t.Parallel()
		const dollarPath = "/srv/po$se"
		a := viaHome(t, base+"autostart_dir: "+dollarPath+"\n")
		// Home is the directive's own subject and a scratch home cannot
		// carry a `$` in its path, so it is set here: RenderWatchUnit reads
		// the config through ConfigPath and takes Environment=RHQ_HOME= from
		// Home, which is exactly the seam under test.
		a.Home = dollarPath
		u, err := RenderWatchUnit(a, "/opt/posse")
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"WorkingDirectory=" + dollarPath, "Environment=RHQ_HOME=" + dollarPath} {
			if !strings.Contains(u.Body, want+"\n") {
				t.Errorf("no %q in the unit — these directives substitute nothing, so a doubled dollar would reach the process as two characters:\n%s", want, u.Body)
			}
		}
	})

	// And the two renderers say it directly, which is where the rule lives.
	for _, tc := range []struct{ in, cmd, arg string }{
		{"/opt/po$se", "/opt/po$$se", "/opt/po$se"},
		{"${RHQ_HOME}/posse", "$${RHQ_HOME}/posse", "${RHQ_HOME}/posse"},
		{"/opt/100%/posse", "/opt/100%%/posse", "/opt/100%%/posse"},
	} {
		t.Run("unitCmdArg/"+tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := unitCmdArg(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.cmd {
				t.Errorf("unitCmdArg(%q) = %q, want %q", tc.in, got, tc.cmd)
			}
			got, err = unitArg(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.arg {
				t.Errorf("unitArg(%q) = %q, want %q — `%%` is expanded in every directive and `$` only on a command line", tc.in, got, tc.arg)
			}
		})
	}
}

// A newline is not a value in a unit file: it ends the directive and opens
// the next one, so a path carrying one would inject directives nobody wrote.
// Flat-YAML cannot produce one — the reader splits on it — which is why this
// is a refusal rather than an escape, and why the pin drives it through the
// renderer's own guard instead of a config.
func TestSystemdUnitRefusesAControlCharacter(t *testing.T) {
	t.Parallel()
	for name, v := range map[string]string{
		"a newline":         "/srv/crew\nExecStart=/bin/sh",
		"a carriage return": "/srv/crew\rExecStart=/bin/sh",
		"a nul":             "/srv/crew\x00",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := unitArg(v); err == nil {
				t.Errorf("unitArg(%q) rendered a value that would end the directive", v)
			}
			if _, err := unitExec([]string{"/opt/posse", v}); err == nil {
				t.Errorf("unitExec passed %q into an ExecStart line", v)
			}
		})
	}
	if _, err := unitArg("/srv/crew"); err != nil {
		t.Errorf("control: an ordinary path must not be refused: %v", err)
	}
}

// --write REFUSES TO REPLACE A UNIT IT DID NOT RENDER. One account holds one
// unit directory (ADR 0080 D12) and an untagged home renders a name that is
// not specific to it, so a silent overwrite is how one crew's loop ends up
// pointed at another crew's home. Re-writing the SAME bytes is not a
// replacement and is never refused, so re-running the command after a
// config change is one step and not two.
func TestSystemdUnitWriteRefusesToReplaceAnotherUnit(t *testing.T) {
	user := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", user)
	a := viaHome(t, "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\ninstance: crewa\n")

	var out, errw strings.Builder
	if err := a.CmdSystemdUnit(true, false, &out, &errw); err != nil {
		t.Fatalf("first write: %v", err)
	}
	path := filepath.Join(user, "systemd", "user", "posse-dispatch-crewa.service")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("--write wrote nothing at %s: %v", path, err)
	}
	if !strings.Contains(out.String(), path) {
		t.Errorf("the command did not say where it wrote: %q", out.String())
	}
	// posse never enables it, and says whose job that is.
	if !strings.Contains(out.String(), "systemctl --user enable --now posse-dispatch-crewa.service") {
		t.Errorf("the command must PRINT the enable line (ADR 0080 D5), not run it: %q", out.String())
	}

	// Idempotent: the same render over the same home replaces nothing.
	out.Reset()
	if err := a.CmdSystemdUnit(true, false, &out, &errw); err != nil {
		t.Errorf("re-writing identical bytes is not a replacement: %v", err)
	}

	// Somebody else's unit under this home's name.
	if err := os.WriteFile(path, []byte("[Service]\nExecStart=/opt/posse dispatch --watch 5m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.CmdSystemdUnit(true, false, &out, &errw); err == nil {
		t.Error("--write replaced a unit that says something else")
	}
	if body, _ := os.ReadFile(path); !strings.Contains(string(body), "ExecStart=/opt/posse dispatch --watch 5m\n") {
		t.Error("the refused write changed the file anyway")
	}
	// --force is the operator saying they looked.
	if err := a.CmdSystemdUnit(true, true, &out, &errw); err != nil {
		t.Errorf("--force: %v", err)
	}
	if body, _ := os.ReadFile(path); !strings.Contains(string(body), "Restart=on-failure") {
		t.Error("--force did not write the rendered unit")
	}
}

// Without --write the unit is STDOUT and everything about it is stderr, so
// `posse systemd > name.service` is a file and not a file with a warning in
// the middle of it (the `posse runtime filing` split, same reason).
func TestSystemdUnitStdoutIsTheUnitAndStderrIsTheNotes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := viaHome(t, "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\nautostart_max_beads: lots\n")
	var out, errw strings.Builder
	if err := a.CmdSystemdUnit(false, false, &out, &errw); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		if strings.Contains(line, "posse systemd:") {
			t.Errorf("a note reached stdout, where the unit is: %q", line)
		}
	}
	if !strings.Contains(out.String(), "[Service]") || !strings.Contains(out.String(), "ExecStart=") {
		t.Errorf("stdout is not a unit file:\n%s", out.String())
	}
	if !strings.Contains(errw.String(), "is not a count") {
		t.Errorf("the dropped value was not reported on stderr: %q", errw.String())
	}
	if !strings.Contains(errw.String(), "systemctl --user enable --now") {
		t.Errorf("stderr must carry the enable line the operator still has to run: %q", errw.String())
	}
	// And it wrote nothing: a render is a render.
	if _, err := os.Stat(filepath.Join(WatchUnitDir(), "posse-dispatch.service")); err == nil {
		t.Error("a render with no --write put a unit on disk")
	}

	// THE BINARY IS THE RUNNING POSSE'S OWN ABSOLUTE PATH (ADR 0080 D5), and
	// this is the only pin that can say so: every other one calls
	// RenderWatchUnit directly with a hardcoded exe — right for what each of
	// them is about, and it leaves the two lines that COMPUTE the real one
	// (os.Executable, filepath.Abs) with no witness at all, the
	// ranger-base-u4uwg shape (ranger-base-hmet1, from ranger-base-zml8t).
	// The only thing the real caller can be graded against is this test
	// binary's own path, so that is what it is graded against.
	own, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	own, err = filepath.Abs(own)
	if err != nil {
		t.Fatal(err)
	}
	want, err := unitCmdArg(own)
	if err != nil {
		t.Fatal(err)
	}
	var execStart string
	for _, line := range strings.Split(out.String(), "\n") {
		if rest, ok := strings.CutPrefix(line, "ExecStart="); ok {
			execStart = rest
		}
	}
	if execStart == "" {
		t.Fatalf("stdout carries no ExecStart line:\n%s", out.String())
	}
	// Absolute, which is the half systemd itself enforces: it refuses a unit
	// whose ExecStart does not start with `/`, so a bare `posse` here is a
	// loop that never starts.
	if !strings.HasPrefix(strings.TrimPrefix(execStart, `"`), "/") {
		t.Errorf("ExecStart=%s names no absolute binary — systemd refuses such a unit at `enable`, so the argv has to carry the installed path and not a name", execStart)
	}
	if !strings.HasPrefix(execStart, want+" dispatch ") {
		t.Errorf("ExecStart=%s does not run the posse that rendered it (%s) — the argv is supposed to track the binary (ADR 0080 D5)", execStart, want)
	}
}

// WorkingDirectory= takes an absolute path and systemd refuses a unit
// without one, so a home that names no `autostart_dir:` on a box with no
// HOME has no working directory to give the loop. Refused at render, naming
// the key that fixes it, rather than written as a unit that fails at start
// with a message about a directory.
//
// t.Setenv, so no t.Parallel here: the subject IS the environment.
func TestSystemdUnitRefusesWithoutAnAbsoluteWorkingDirectory(t *testing.T) {
	base := "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\n"
	t.Setenv("HOME", "")
	a := viaHome(t, base)
	if u, err := RenderWatchUnit(a, "/opt/posse"); err == nil {
		t.Fatalf("rendered a unit with no working directory:\n%s", u.Body)
	} else if !strings.Contains(err.Error(), "autostart_dir") {
		t.Errorf("the refusal does not name the key that fixes it: %v", err)
	}
	// A relative autostart_dir: is the same refusal: systemd expands
	// nothing, so `~` and `crew` are both relative paths to it.
	for _, dir := range []string{"~", "crew", "./crew"} {
		if _, err := RenderWatchUnit(viaHome(t, base+"autostart_dir: "+dir+"\n"), "/opt/posse"); err == nil {
			t.Errorf("autostart_dir: %s rendered a relative WorkingDirectory=", dir)
		}
	}
	// The control: an absolute one renders, so the check above is refusing
	// a shape and not every value.
	if _, err := RenderWatchUnit(viaHome(t, base+"autostart_dir: /srv/crew\n"), "/opt/posse"); err != nil {
		t.Errorf("control: an absolute autostart_dir: must render: %v", err)
	}
}

// --write has to be able to say WHERE. WatchUnitDir reads the environment,
// so with neither XDG_CONFIG_HOME nor HOME set the unit would land at
// `systemd/user/<unit>` relative to wherever the command ran — a file
// systemd will never read, reported as a successful install.
func TestSystemdUnitWriteRefusesWithNoUnitDirectory(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	a := viaHome(t, "autostart_via: systemd\nautostart_multiplexer_unit: mux.service\nautostart_dir: /srv/crew\n")

	var out, errw strings.Builder
	if err := a.CmdSystemdUnit(true, false, &out, &errw); err == nil {
		t.Error("--write wrote a unit to a path it could not name")
	} else if !strings.Contains(err.Error(), "XDG_CONFIG_HOME") {
		t.Errorf("the refusal does not name what would fix it: %v", err)
	}
	// WatchUnitDir() is what the mutant would have written into, and with
	// no HOME it is `.config/systemd/user` — RELATIVE to this package's own
	// directory. Asserted at that exact path, and cleaned up, because a
	// guard whose failure litters the repo is a guard whose red costs a
	// second red somewhere else.
	litter := filepath.Join(WatchUnitDir(), "posse-dispatch.service")
	t.Cleanup(func() { os.RemoveAll(strings.Split(WatchUnitDir(), string(os.PathSeparator))[0]) })
	if _, err := os.Stat(litter); err == nil {
		t.Errorf("--write put a unit at %s, relative to wherever this ran", litter)
	}
	// The render itself still works: printing a unit needs no unit dir, and
	// a render on a box that is not the host is a legitimate thing to want.
	out.Reset()
	if err := a.CmdSystemdUnit(false, false, &out, &errw); err != nil {
		t.Errorf("a render with no --write needs no unit directory: %v", err)
	}
	if !strings.Contains(out.String(), "ExecStart=") {
		t.Errorf("stdout is not a unit:\n%s", out.String())
	}
}

// ONE REFUSAL SENTENCE, TWO CALLERS, each naming the config path its own
// surface names: AutostartVia hands it the absolute path a refusal at the
// keyboard should quote, and G7 hands it AbbrevHome, which is what every
// other row of that table prints.
//
// The G7 row's own fixture cannot pin the second half — a scratch config
// that is not below HOME makes AbbrevHome the identity, so the two spellings
// are one string there — so what is pinned is the seam: the renderer takes
// the path as an argument, and AutostartVia's error IS that renderer's
// output. A second sentence for either caller then shows up here.
func TestAutostartViaRefusalIsOneSentenceWithThePathAsAnArgument(t *testing.T) {
	t.Parallel()
	for _, cfg := range []string{"~/.config/posse/config.yaml", "/srv/crewa/config.yaml"} {
		err := AutostartViaRefusal("banana", cfg)
		if err == nil {
			t.Fatal("the refusal renderer returned no error")
		}
		if !strings.Contains(err.Error(), cfg) {
			t.Errorf("the sentence does not name the path it was handed (%q): %v", cfg, err)
		}
		for _, want := range []string{AutostartViaKey, "banana", AutostartViaHerdr, AutostartViaSystemd} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the sentence does not name %q: %v", want, err)
			}
		}
	}
	// AutostartVia's own refusal is this renderer's output over its own
	// config path — not a second copy of the words.
	a := viaHome(t, "autostart_via: banana\n")
	_, err := a.AutostartVia()
	if err == nil {
		t.Fatal("AutostartVia accepted a value that names neither mechanism")
	}
	if want := AutostartViaRefusal("banana", a.ConfigPath).Error(); err.Error() != want {
		t.Errorf("AutostartVia says\n  %s\nand the renderer says\n  %s", err, want)
	}
}
