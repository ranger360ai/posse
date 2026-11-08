//go:build !posse_arm2 && !posse_arm3

package posse

// `cage_realize_<goos>:` — ADR 0083 D7, built in ranger-base-1155n.
//
// The four claims, and the order they are pinned in below:
//
//  1. THE MAP. (goos × demanded × config) → the realized tier, whether the
//     answer is ratified degradation, and whether the config is refused. One
//     table, because every refusal is a case the harness must not read as
//     "no mapping" — that reading is ADR 0080 D9's typo arming the wrong
//     thing, and it is the only way this key can fail silently.
//  2. WHAT RATIFIED IT. The provenance in the reason, over the four states
//     promoted.json can be in.
//  3. PARITY AT THE REALIZED TIER. The map waives the cage line and never a
//     rule: a PID denying `Edit`/`Write` bare, mapped seatbelt→shims, still
//     refuses. With a control at the same tier and no map, so the suppressed
//     cage row is the map's doing and not a hole.
//  4. THE SURFACES. The session record is marked degraded; dispatch and a
//     relaunch still launch; `posse status` carries one standing row; G15
//     stands down for a session whose only degradation is this one.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// crLinuxMap is the ADR's own example, in the spelling this harness reads.
const crLinuxMap = "cage_realize_linux:\n  seatbelt: shims\n"

// crMapFor is that map for whichever OS the suite is running on, so the
// integration arms below exercise the live `runtime.GOOS` branch on a mac and
// on a Linux box alike rather than one of them skipping.
func crMapFor(goos string) string {
	return CageRealizeKey(goos) + ":\n  " + CageSeatbelt + ": " + CageShims + "\n"
}

// crApp is a home whose config.yaml is body and whose promoted.json is
// absent — the never-promoted state, so the ratifier in every reason below is
// one fixed string and not this box's own manifest.
func crApp(t *testing.T, body string) *App {
	t.Helper()
	home := t.TempDir()
	a := &App{
		Home: home, ConfigPath: filepath.Join(home, "config.yaml"),
		EnvsDir: filepath.Join(home, "envs"), StateDir: filepath.Join(home, "state"),
		AgentsDir: filepath.Join(home, "agents"),
	}
	if err := os.WriteFile(a.ConfigPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return a
}

// crCfgPrepend puts body at the TOP of an existing config. Flat-YAML's block
// lists run to the next column-0 key (yamlflat.go), so a key appended after
// `beads:` would end that list — writing first keeps whatever the fixture
// already armed exactly as it armed it.
func crCfgPrepend(t *testing.T, a *App, body string) {
	t.Helper()
	old, err := os.ReadFile(a.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, append([]byte(body), old...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// crNeverPromoted is cageRealizeRatifier's answer for a home with no
// manifest, which is what crApp and every test backend is.
const crNeverPromoted = "config.yaml — this home was never promoted"

// ─── 1 · the map ─────────────────────────────────────────────────────────────

func TestCageRealizeMapTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		cfg      string
		goos     string
		demanded string
		realized string
		ratified bool   // the answer is ratified degradation
		refuse   string // substring of the config refusal, "" = accepted
	}{
		// Accepted, and the shapes that mean "no mapping".
		{name: "no key at all leaves every tier alone",
			cfg: "", goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt},
		{name: "the mapped demand on the named OS",
			cfg: crLinuxMap, goos: "linux", demanded: CageSeatbelt, realized: CageShims, ratified: true},
		{name: "another OS reads its own answer, which is none",
			cfg: crLinuxMap, goos: "darwin", demanded: CageSeatbelt, realized: CageSeatbelt},
		{name: "a demand the map does not name is untouched",
			cfg: crLinuxMap, goos: "linux", demanded: CageShims, realized: CageShims},
		{name: "a bare key is an empty map, not a value",
			cfg: "cage_realize_linux:\n", goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt},
		{name: "a comment under the key is not an entry",
			cfg: "cage_realize_linux:\n  # seatbelt: shims\n", goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt},
		// Mapping to the same or a stronger tier is not degradation (D7).
		{name: "identity is not degradation",
			cfg: "cage_realize_linux:\n  seatbelt: seatbelt\n", goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt},
		{name: "a stronger target is not degradation",
			cfg: "cage_realize_linux:\n  shims: seatbelt\n", goos: "linux", demanded: CageShims, realized: CageSeatbelt},
		// Refused, never defaulted. Each row's realized tier is the DEMAND:
		// a map posse cannot read waives nothing.
		{name: "ADR 0083 D7's nested spelling is refused BY NAME",
			cfg:  "cage_realize:\n  linux:\n    seatbelt: shims\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "is not a key this config reader can see"},
		{name: "an OS this posse does not run on",
			cfg:  crLinuxMap + "cage_realize_plan9:\n  seatbelt: shims\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "names no OS this posse runs on"},
		{name: "a misspelled OS is not silence",
			cfg:  "cage_realize_linus:\n  seatbelt: shims\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "names no OS this posse runs on"},
		{name: "a scalar where the block belongs",
			cfg:  "cage_realize_linux: shims\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "is a MAP of demanded tier to realized tier"},
		{name: "an inline map is a scalar to this reader",
			cfg:  "cage_realize_linux: {seatbelt: shims}\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "is a MAP of demanded tier to realized tier"},
		{name: "a demanded tier that is not a tier",
			cfg:  "cage_realize_linux:\n  bwrap: shims\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: `"bwrap" is not a cage tier`},
		{name: "a realized tier that is not a tier",
			cfg:  "cage_realize_linux:\n  seatbelt: bwrap\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: `"bwrap" is not a cage tier`},
		{name: "a demand mapped to nothing",
			cfg:  "cage_realize_linux:\n  seatbelt:\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "names no realized tier"},
		{name: "container on the demanded side",
			cfg:  "cage_realize_linux:\n  container: shims\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "may appear on neither side"},
		{name: "container on the realized side",
			cfg:  "cage_realize_linux:\n  seatbelt: container\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "may appear on neither side"},
		{name: "one demand with two answers",
			cfg:  "cage_realize_linux:\n  seatbelt: shims\n  seatbelt: seatbelt\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "mapped twice"},
		// IDEMPOTENCE (ranger-base-pl3ck). A launch applies this map twice,
		// so a realized tier that is itself a demanded key answered with
		// something else resolves to two different tiers on the two
		// readings. Accepted before the refusal landed, and MEASURED: the
		// wall went up at one tier while the parity block, the report and
		// the session record said the other, with a waiver for a demand
		// nobody made.
		{name: "a swap answers differently on the second application",
			cfg:  "cage_realize_linux:\n  shims: seatbelt\n  seatbelt: shims\n",
			goos: "linux", demanded: CageShims, realized: CageShims,
			refuse: "is itself a demanded tier in this map"},
		{name: "the swap is refused whichever way round it is written",
			cfg:  "cage_realize_linux:\n  seatbelt: shims\n  shims: seatbelt\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "is itself a demanded tier in this map"},
		{name: "another OS's swap refuses this OS's read, like every other member",
			cfg:  crLinuxMap + "cage_realize_darwin:\n  shims: seatbelt\n  seatbelt: shims\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: "is itself a demanded tier in this map"},
		// And the two shapes it must NOT catch, which is the whole of why
		// the test is "answered with something OTHER than itself": identity
		// is a key whose answer is itself, so a second application changes
		// nothing, and a lone raise names no key on its realized side.
		{name: "identity beside a raise is idempotent and stays accepted",
			cfg: "cage_realize_linux:\n  shims: seatbelt\n  seatbelt: seatbelt\n", goos: "linux",
			demanded: CageShims, realized: CageSeatbelt},
		// The validation is TOTAL, not per-lookup: the member being edited
		// is rarely the member being read, and a map whose other half nobody
		// checked is a map nobody checked.
		{name: "another OS's broken member refuses this OS's read",
			cfg:  "cage_realize_darwin:\n  seatbelt: bwrap\n",
			goos: "linux", demanded: CageSeatbelt, realized: CageSeatbelt,
			refuse: `"bwrap" is not a cage tier`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(cfg, []byte(tc.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			res, err := RealizeCageIn(cfg, tc.goos, crNeverPromoted, tc.demanded)
			switch {
			case tc.refuse == "" && err != nil:
				t.Fatalf("accepted config refused: %v", err)
			case tc.refuse != "" && err == nil:
				t.Fatalf("config must be refused (want %q), got %+v", tc.refuse, res)
			case tc.refuse != "" && !strings.Contains(err.Error(), tc.refuse):
				t.Fatalf("refusal does not say %q: %v", tc.refuse, err)
			}
			if res.Realized != tc.realized {
				t.Errorf("realized = %q, want %q", res.Realized, tc.realized)
			}
			if res.Demanded != tc.demanded || res.GOOS != tc.goos {
				t.Errorf("resolution must carry its own inputs back: %+v", res)
			}
			if (res.Ratified != "") != tc.ratified {
				t.Errorf("ratified = %q, want set=%v", res.Ratified, tc.ratified)
			}
			if tc.ratified {
				want := "cage_realize: " + tc.goos + " " + tc.demanded + "→" + tc.realized + " (" + crNeverPromoted + ")"
				if res.Ratified != want {
					t.Errorf("reason = %q, want %q", res.Ratified, want)
				}
			}
		})
	}
}

// THE INVARIANT THE REFUSAL ABOVE BUYS (ranger-base-pl3ck): a map this reader
// accepts answers the same thing on its second application.
//
// It is not a style rule. A launch resolves the map TWICE — planLaunch
// resolves the demand and hands the REALIZED tier to checkParityIn, which
// resolves again, and that second reading is load-bearing (it is how parity
// reports the waiver for a relaunch that comes back carrying the realized
// tier). So every accepted map is applied to its own answer here, which is
// exactly what the second reading does.
func TestCageRealizeAcceptedMapsAreIdempotent(t *testing.T) {
	t.Parallel()
	for _, cfgBody := range []string{
		"",
		crLinuxMap,
		"cage_realize_linux:\n  seatbelt: seatbelt\n",
		"cage_realize_linux:\n  shims: seatbelt\n",
		"cage_realize_linux:\n  shims: seatbelt\n  seatbelt: seatbelt\n",
	} {
		t.Run(strings.ReplaceAll(cfgBody, "\n", "·"), func(t *testing.T) {
			t.Parallel()
			cfg := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(cfg, []byte(cfgBody), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, demanded := range []string{CageShims, CageSeatbelt} {
				first, err := RealizeCageIn(cfg, "linux", crNeverPromoted, demanded)
				if err != nil {
					t.Fatalf("accepted config refused: %v", err)
				}
				again, err := RealizeCageIn(cfg, "linux", crNeverPromoted, first.Realized)
				if err != nil {
					t.Fatal(err)
				}
				if again.Realized != first.Realized {
					t.Errorf("%q: %s realizes as %s, but applying the map to %s again gives %s — the launch applies it twice, so the wall would go up at one tier and every report name the other",
						cfgBody, demanded, first.Realized, first.Realized, again.Realized)
				}
			}
		})
	}
}

// The standing reading `posse status` renders is the same map filtered to
// what it LOWERS — identity and a stronger target raise no row.
func TestCageRealizeStandingNamesOnlyWhatItLowers(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	body := "cage_realize_linux:\n  seatbelt: shims\n  shims: shims\n" +
		"cage_realize_darwin:\n  shims: seatbelt\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := CageRealizeStandingFor(cfg, "linux", crNeverPromoted)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Any() || len(st.Lowered) != 1 || st.Lowered[0].String() != "seatbelt→shims" {
		t.Fatalf("linux must lower exactly seatbelt→shims: %+v", st)
	}
	if d := st.GovDetail(); !strings.Contains(d, "cage_realize_linux: seatbelt→shims") ||
		!strings.Contains(d, crNeverPromoted) ||
		!strings.Contains(d, "waives the cage line only") {
		t.Errorf("row detail must name the entry, what ratified it, and what it does NOT waive: %q", d)
	}
	// darwin's only entry RAISES a tier, so darwin stands no row — the one
	// arm that tells "a map is in force" from "a map is weakening this box".
	dw, err := CageRealizeStandingFor(cfg, "darwin", crNeverPromoted)
	if err != nil {
		t.Fatal(err)
	}
	if dw.Any() {
		t.Errorf("a map that only raises a tier must raise no row: %+v", dw)
	}
}

// ─── 2 · what ratified it ────────────────────────────────────────────────────

// The provenance in the reason, over every state promoted.json can be in.
// "Ratified at promote" is the whole of why this degradation may launch, so a
// waiver with no anchor behind it has to say so rather than render a
// confident `(promoted …)` over a home nobody ever promoted.
func TestCageRealizeRatifierNamesWhatRatifiedIt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		m    *PromoteManifest
		err  error
		want string
	}{
		{"never promoted", nil, nil, crNeverPromoted},
		{"a fresh install's seeded anchor", &PromoteManifest{Seeded: true}, nil, "seeded promoted.json, no commit behind it"},
		{"a real promote names its commit", &PromoteManifest{SHA: "0123456789abcdef"}, nil, "promoted 0123456789ab"},
		{"a promote that could not name one", &PromoteManifest{PromotedAt: "2026-11-08T00:00:00Z"}, nil, "promoted, but promoted.json records no commit"},
		{"an unreadable anchor", nil, Die("bad json"), "promoted.json is unreadable: bad json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := cageRealizeRatifier(tc.m, tc.err); got != tc.want {
				t.Errorf("ratifier = %q, want %q", got, tc.want)
			}
		})
	}
}

// The reason carries no "; " of its own, and that is now a property of the
// WRITER and not of its inputs (ranger-base-pl3ck). `degraded:` is a
// "; "-joined list and DegradedBeyondRatified splits on that separator, so a
// ratifier containing it would split this one reason into a second part with
// no `cage_realize: ` prefix — read as REAL degradation, which arms
// AllowDegraded on the relaunch: the exact direction that predicate exists to
// prevent. The ratifier is the only free-text part, and one of its four
// states quotes an os error.
func TestCageRealizeReasonCarriesNoListSeparator(t *testing.T) {
	t.Parallel()
	e := CageRealizeEntry{From: CageSeatbelt, To: CageShims}
	// The shape that produced it: an unreadable manifest quoting an os error
	// whose own text carries the separator.
	ratifier := cageRealizeRatifier(nil, Die("open /x; y: permission denied"))
	if !strings.Contains(ratifier, "; ") {
		t.Fatalf("NOTHING MEASURED: this fixture is meant to put %q in the ratifier, got %q", "; ", ratifier)
	}
	reason := cageRealizeReason("linux", e, ratifier)
	if strings.Contains(reason, "; ") {
		t.Errorf("the reason must carry no %q: %q splits into parts that are not all ratified, and RecreateOpts then arms AllowDegraded", "; ", reason)
	}
	if DegradedBeyondRatified(reason) {
		t.Errorf("a ratified-only degraded field must read as ratified only: %q", reason)
	}
	// Still legible, and still the provenance: the strip replaces the
	// separator, it does not drop the sentence.
	for _, want := range []string{CageRealizeReasonPrefix, "seatbelt→shims", "permission denied", PromoteManifestFile} {
		if !strings.Contains(reason, want) {
			t.Errorf("reason %q lost %q", reason, want)
		}
	}
	// And the four states that carry no separator are untouched, byte for
	// byte, so nothing else in this file's expectations moves.
	if got, want := cageRealizeReason("linux", e, crNeverPromoted), CageRealizeReasonPrefix+"linux seatbelt→shims ("+crNeverPromoted+")"; got != want {
		t.Errorf("reason = %q, want %q", got, want)
	}
}

// ─── 3 · parity at the realized tier ────────────────────────────────────────

// THE CLAIM D7 RESTS ON: the map waives the cage line and never a rule.
//
// Both arms run the SAME map at the SAME tier and differ only in the PID, so
// what separates them is the rule and nothing else:
//
//   - a PID denying `Edit`/`Write` bare needs a file wall. Mapped
//     seatbelt→shims there is no wall at any layer, the deny is unrealized,
//     and the launch refuses exactly as it does on a host with no seatbelt.
//   - a PID whose rules are all L1-realizable loses nothing by running at
//     shims, so it launches — marked, with the waiver as its reason.
//
// Seatbelt is forced AVAILABLE for the duration, which makes this the
// stronger reading of the key: the demanded tier genuinely exists here, so
// the lowering is the map's doing and cannot be the old "cage seatbelt is not
// available on this host/build" row wearing a new name.
func TestCageRealizeParityIsComputedAtTheRealizedTier(t *testing.T) {
	// No t.Parallel(): this mutates AvailableCages, a package-level map
	// other tests read.
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	defer func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	}()

	a := crApp(t, crMapFor(runtime.GOOS))
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	wall := cageAgent(t, a, "cage: seatbelt\ndeny: [Edit, Write]\n")
	light := cageAgent(t, a, "cage: seatbelt\ndeny: [Bash(git push:*)]\n")
	reason := "cage_realize: " + runtime.GOOS + " seatbelt→shims (" + crNeverPromoted + ")"

	// A rule the realized tier cannot hold still refuses.
	pw := a.CheckParity(wall, claude, ResolveCage("", wall), TierStrong)
	if pw.Cage != CageShims {
		t.Fatalf("the map must realize seatbelt as shims: %+v", pw)
	}
	// And the DEMAND is remembered on the ⚖ row and not in a field of its
	// own (ranger-base-pl3ck): `<demanded>→<realized>` is where the pair is
	// load-bearing, and it is asserted in full two lines down.
	if !strings.Contains(strings.Join(pw.Ratified, " "), CageSeatbelt+"→"+CageShims) {
		t.Errorf("the waiver must name the demand it lowered: %v", pw.Ratified)
	}
	if len(pw.Ratified) != 1 || pw.Ratified[0] != reason {
		t.Errorf("ratified row = %v, want [%q]", pw.Ratified, reason)
	}
	joined := strings.Join(pw.Degraded, "\n")
	if !strings.Contains(joined, "Edit — needs cage: seatbelt") || !strings.Contains(joined, "Write — needs cage: seatbelt") {
		t.Errorf("a bare Edit/Write deny mapped to shims must go UNREALIZED: %q", joined)
	}
	if err := a.CheckTier(wall, claude, ResolveCage("", wall), TierStrong, false); err != nil {
		t.Errorf("CheckTier judges the tier, not the gates; the launch is what refuses: %v", err)
	}
	// And the cage line itself is NOT in the refusing list — it is the one
	// thing the map waived.
	if strings.Contains(joined, "cage: PID demands seatbelt") {
		t.Errorf("the ratified cage shortfall must not also be in the list that refuses: %q", joined)
	}

	// A PID whose rules hold at shims launches, marked.
	pl := a.CheckParity(light, claude, ResolveCage("", light), TierStrong)
	if len(pl.Degraded) != 0 {
		t.Fatalf("every rule of this PID holds at shims; nothing may refuse: %v", pl.Degraded)
	}
	if len(pl.Ratified) != 1 || !pl.AnyDegradation() || pl.DegradedReason() != reason {
		t.Fatalf("the session must be marked degraded with the waiver as its reason: %+v", pl)
	}
	if got := pl.String(); !strings.Contains(got, "RATIFIED DEGRADATION") ||
		!strings.Contains(got, "⚖ "+reason) || strings.Contains(got, "DEGRADED\n") ||
		strings.Contains(got, "✗ ") || strings.Contains(got, "all gates realized") {
		t.Errorf("a ratified waiver is its own verdict, spelled like neither of the other two:\n%s", got)
	}

	// THE RELAUNCH READING. CreateSession records the tier the session GOT,
	// so a relaunch comes back here asking for shims and only the PID still
	// remembers seatbelt. The waiver must survive that, or it expires at the
	// first refresh.
	pr := a.CheckParity(light, claude, CageShims, TierStrong)
	if len(pr.Degraded) != 0 || len(pr.Ratified) != 1 || pr.Ratified[0] != reason {
		t.Errorf("a relaunch at the recorded tier must read the same waiver, not a fresh shortfall: %+v", pr)
	}

	// THE CONTROL, and the reason the arm above is not a hole: the same PID,
	// the same tier, no map. The cage shortfall is a REFUSAL again.
	plain := crApp(t, "")
	pc := plain.CheckParity(cageAgent(t, plain, "cage: seatbelt\ndeny: [Bash(git push:*)]\n"), claude, CageShims, TierStrong)
	if len(pc.Ratified) != 0 || !strings.Contains(strings.Join(pc.Degraded, "\n"), "cage: PID demands seatbelt, launching at shims") {
		t.Errorf("with no map the shortfall must still refuse: %+v", pc)
	}

	// A map this reader cannot parse waives nothing and is reported, and
	// dispatch's own door refuses on it rather than reading it as absence.
	broken := crApp(t, CageRealizeKey(runtime.GOOS)+":\n  seatbelt: bwrap\n")
	bag := cageAgent(t, broken, "cage: seatbelt\ndeny: [Bash(git push:*)]\n")
	pb := broken.CheckParity(bag, claude, CageSeatbelt, TierStrong)
	if pb.Cage != CageSeatbelt || len(pb.Ratified) != 0 ||
		!strings.Contains(strings.Join(pb.Degraded, "\n"), `"bwrap" is not a cage tier`) {
		t.Errorf("a broken map must waive nothing and be reported: %+v", pb)
	}
	if err := broken.CheckTier(bag, claude, CageSeatbelt, TierStrong, true); err == nil ||
		!strings.Contains(err.Error(), `"bwrap" is not a cage tier`) {
		t.Errorf("dispatch's door must refuse a map it cannot read, --allow-degraded or not: %v", err)
	}
}

// ─── 4 · the surfaces ────────────────────────────────────────────────────────

// DegradedBeyondRatified is the one predicate two readers subtract on — the
// relaunch's AllowDegraded and G15 — so it is pinned on its own, over the
// "; "-joined shape the session record actually holds.
func TestDegradedBeyondRatified(t *testing.T) {
	t.Parallel()
	const ratified = "cage_realize: linux seatbelt→shims (promoted abc1234)"
	for _, tc := range []struct {
		name     string
		degraded string
		want     bool
	}{
		{"a full-parity session", "", false},
		{"the waiver alone", ratified, false},
		{"two waivers", ratified + "; cage_realize: linux shims→shims (promoted abc1234)", false},
		{"a real gate alone", "Edit — needs cage: seatbelt", true},
		{"a real gate beside the waiver", "Edit — needs cage: seatbelt; " + ratified, true},
		{"the waiver first, a real gate after", ratified + "; Edit — needs cage: seatbelt", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := DegradedBeyondRatified(tc.degraded); got != tc.want {
				t.Errorf("DegradedBeyondRatified(%q) = %v, want %v", tc.degraded, got, tc.want)
			}
		})
	}
}

// The launch: the session record carries the realized tier and the waiver as
// its reason, and it launches with no --allow-degraded — which is what
// "dispatch included" means, dispatch never passing the flag.
func TestCageRealizeLaunchMarksTheSessionWithoutAWaiver(t *testing.T) {
	b, _ := newTestBackend(t)
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	defer func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	}()
	crCfgPrepend(t, b.App, crMapFor(runtime.GOOS))
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "light.md"),
		[]byte("---\nname: light\ncage: seatbelt\ndeny: [Bash(git push:*)]\n---\nYou are light.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "wall.md"),
		[]byte("---\nname: wall\ncage: seatbelt\ndeny: [Edit, Write]\n---\nYou are wall.\n"), 0o644)
	repo := t.TempDir()
	reason := "cage_realize: " + runtime.GOOS + " seatbelt→shims (" + crNeverPromoted + ")"

	mustCreate(t, b, NewSessionOpts{Name: "light-x", Agent: "light", Dir: repo})
	m, ok := b.readMeta("light-x")
	if !ok {
		t.Fatal("no session record")
	}
	if m.Cage != CageShims {
		t.Errorf("the record must name the tier the session GOT: %q", m.Cage)
	}
	if m.Degraded != reason {
		t.Errorf("degraded: = %q, want %q", m.Degraded, reason)
	}
	// The waiver is not the operator's consent, so a relaunch must not carry
	// --allow-degraded forward from it (RecreateOpts).
	if RecreateOpts(m).AllowDegraded {
		t.Error("a ratified waiver must not arm AllowDegraded on the relaunch")
	}

	// The same map, a PID whose rule the realized tier cannot hold: refused.
	err := b.CreateSession(NewSessionOpts{Name: "wall-x", Agent: "wall", Dir: repo})
	if err == nil || !strings.Contains(err.Error(), "Edit — needs cage: seatbelt") {
		t.Errorf("the map waives the cage line, never a rule: %v", err)
	}
}

// ─── `posse status`: one standing row, and G15 standing down ────────────────

func TestGovG17CageMapLoweringATier(t *testing.T) {
	b, _ := newTestBackend(t)
	govRepo(t, b)
	crCfgPrepend(t, b.App, crMapFor(runtime.GOOS))

	g := find(shopSet(t, govIn(t, b)), "G17")
	if g == nil {
		t.Fatalf("no G17 while the map lowers a tier: %v", shopSet(t, govIn(t, b)).Keys())
	}
	if g.Key != "cage-realize:"+runtime.GOOS || g.Class != GovLane {
		t.Errorf("G17 = %+v, want key cage-realize:%s class LANE", *g, runtime.GOOS)
	}
	if !strings.Contains(g.Detail, "seatbelt→shims") {
		t.Errorf("the row must name the entry: %q", g.Detail)
	}
}

func TestGovG17NoMapIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	govRepo(t, b)
	if g := find(shopSet(t, govIn(t, b)), "G17"); g != nil {
		t.Errorf("a home with no cage map must raise nothing: %+v", *g)
	}
}

// A MAP THIS READER CANNOT PARSE IS NOT AN ALL-CLEAR (ranger-base-pl3ck).
// The invariant is stated in capitals over govern.go's G17 arm and had no
// pin: the unreadable map goes to `failed`, so the set is PARTIAL and no
// surface renders "no condition" over a reading that did not happen.
// MEASURED before this arm existed: replacing that append with a discard left
// every test green, TestGovG17CageMapLoweringATier and
// TestGovG17NoMapIsNotACondition included — the two arms that look like they
// cover it.
//
// Not shopSet, which fatals on a partial set: that is the thing being
// asserted here, so this arm reads ShopCheck's two returns itself.
func TestGovG17AnUnparseableMapIsNotAnAllClear(t *testing.T) {
	b, _ := newTestBackend(t)
	govRepo(t, b)
	// A mistyped MEMBER of the family, which is the failure the key family
	// was ruled to have: it reads as absent to a per-lookup reader, and
	// total validation is what turns it into this error.
	crCfgPrepend(t, b.App, "cage_realize_linus:\n  seatbelt: shims\n")

	set, failed := ShopCheck(govIn(t, b))
	if g := find(set, "G17"); g != nil {
		t.Errorf("an unreadable map raises no G17 row — whether anything is lowered is exactly what could not be read: %+v", *g)
	}
	var found string
	for _, err := range failed {
		if strings.Contains(err.Error(), "G17 unknown") {
			found = err.Error()
		}
	}
	if found == "" {
		t.Fatalf("the set must be PARTIAL, naming G17 as the reading that failed; got %d failure(s): %v", len(failed), failed)
	}
	// The failure carries the config's own sentence, so the operator reads
	// the line to fix and not just which check gave up.
	if !strings.Contains(found, "names no OS this posse runs on") {
		t.Errorf("the partial-set entry must carry the map's own refusal: %q", found)
	}
}

// G15 is one SESSION's waiver, and every clause of its sentence is false for
// a ratified map: nothing was typed at a keyboard, no relaunch re-arms it,
// and the remedy is the map's. D7 asks for ONE standing row instead, which is
// why this is a stand-down and not a second report of the same fact.
func TestGovG15StandsDownForARatifiedWaiverAndNotForARealOne(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir})

	govDegrade(t, b, "developer-x", "cage_realize: "+runtime.GOOS+" seatbelt→shims (promoted abc1234)")
	if g := find(shopSet(t, govIn(t, b)), "G15"); g != nil {
		t.Errorf("a session whose only degradation is the ratified map must raise no G15: %+v", *g)
	}
	// The positive half: one real gate beside the waiver and the row is back,
	// so the stand-down is a predicate and not a mute.
	govDegrade(t, b, "developer-x", "Edit — needs cage: seatbelt; cage_realize: "+runtime.GOOS+" seatbelt→shims (promoted abc1234)")
	g := find(shopSet(t, govIn(t, b)), "G15")
	if g == nil {
		t.Fatal("a real unrealized gate beside the waiver must still raise G15")
	}
	if !strings.Contains(g.Detail, "Edit — needs cage: seatbelt") {
		t.Errorf("Detail does not name the gate: %q", g.Detail)
	}
}
