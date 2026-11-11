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
//     refuses — ON A HOST THAT DOES NOT PROVIDE THE TIER IT DEMANDED, which
//     since ADR 0086 D2 is the only shape that still reaches that refusal.
//     With a control at the same tier and no map, so the suppressed cage row
//     is the map's doing and not a hole.
//
//     3b. THE MAPPING THIS HOST DECLINES (ADR 0086 D2, ranger-base-dfgte).
//     Where the demanded tier IS provided here, a mapping that would leave a
//     rule of this PID unrealized is declined and the PID launches at its
//     demand — no waiver, nothing degraded, one declared-difference line. The
//     table is the three questions D2 asks and the shapes that answer each
//     "no", and the launch arm of claim 4 reads the session RECORD back,
//     because a resolver every door agrees on is still wrong if the record
//     names another tier.
//  4. THE SURFACES. The session record is marked degraded; dispatch and a
//     relaunch still launch; `posse status` carries one standing row, which
//     names the PIDs the map is declined for; G15 stands down for a session
//     whose only degradation is this one.

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
			res, err := realizeCageIn(cfg, tc.goos, crNeverPromoted, tc.demanded)
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
				first, err := realizeCageIn(cfg, "linux", crNeverPromoted, demanded)
				if err != nil {
					t.Fatalf("accepted config refused: %v", err)
				}
				again, err := realizeCageIn(cfg, "linux", crNeverPromoted, first.Realized)
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
// AMENDED BY ADR 0086 D2 (ranger-base-dfgte), and the amendment is the FIRST
// arm's precondition, not its verdict. D7's sentence stands — the map waives
// the cage line, never a rule — but the consequence it drew from it, refusing
// such a PID, is now reached only where this host CANNOT provide the tier the
// PID demanded. Where it can, the mapping is DECLINED and the PID launches at
// its demand (TestCageRealizeDeclinesAMappingThatWouldCostAPIDARule). So the
// first arm below runs twice, once on each side of AvailableCages, which is
// the only thing that separates the two answers.
//
// Seatbelt is forced AVAILABLE for the rest of the body, which makes those
// arms the stronger reading of the key: the demanded tier genuinely exists
// here, so the lowering is the map's doing and cannot be the old "cage
// seatbelt is not available on this host/build" row wearing a new name.
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

	// A rule the realized tier cannot hold still refuses — WHERE DECLINING
	// would buy the PID nothing, which since ADR 0086 D2 is the whole of
	// what this arm measures. The tier the PID demanded is not available
	// here, so handing it back would hand it a tier that realizes nothing;
	// the map applies and the rule goes unrealized at the mapped tier,
	// exactly as it did on a host with no seatbelt at all.
	delete(AvailableCages, CageSeatbelt)
	pw := a.CheckParity(wall, claude, ResolveCage("", wall), TierStrong)
	AvailableCages[CageSeatbelt] = true
	if pw.Cage != CageShims {
		t.Fatalf("the map must realize seatbelt as shims where seatbelt is not available: %+v", pw)
	}
	if len(pw.DeclaredDifference) != 0 {
		t.Errorf("a tier this host cannot provide must not be declined back to: %v", pw.DeclaredDifference)
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
	// And the cage line itself is NOT in the refusing list — it is the one
	// thing the map waived.
	if strings.Contains(joined, "cage: PID demands seatbelt") {
		t.Errorf("the ratified cage shortfall must not also be in the list that refuses: %q", joined)
	}
	// CheckTier judges the TIER and not the gates — the launch is what
	// refuses — and it reads the map through the one resolver, so it is
	// asked on both sides of availability. Neither answer is an error here.
	for _, avail := range []bool{false, true} {
		if avail {
			AvailableCages[CageSeatbelt] = true
		} else {
			delete(AvailableCages, CageSeatbelt)
		}
		if err := a.CheckTier(wall, claude, ResolveCage("", wall), TierStrong, false); err != nil {
			t.Errorf("CheckTier judges the tier, not the gates (seatbelt available=%v): %v", avail, err)
		}
	}
	AvailableCages[CageSeatbelt] = true

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

// ─── 3b · ADR 0086 D2 · the mapping this host declines ──────────────────────

// D2's rule, over the three questions it asks and the shapes that answer each
// of them "no". One table, because a decline and a mapping differ in NOTHING
// an operator can see except the tier the session gets: a row that declined
// when it should have mapped is a wall nobody ratified, and a row that mapped
// when it should have declined is ADR 0083 D7's unlaunchable PID.
//
// AVAILABILITY IS A COLUMN, not a fixture property, because it is question 2
// — and it is the only thing separating the two answers for the same PID and
// the same map. The map is forced and the table is serial for that reason.
func TestCageRealizeDeclinesAMappingThatWouldCostAPIDARule(t *testing.T) {
	// No t.Parallel(): every row writes AvailableCages, which CheckParity
	// and every launch path read.
	had := AvailableCages[CageSeatbelt]
	t.Cleanup(func() {
		if had {
			AvailableCages[CageSeatbelt] = true
		} else {
			delete(AvailableCages, CageSeatbelt)
		}
	})
	for _, tc := range []struct {
		name     string
		cfg      string
		front    string // the PID's frontmatter
		avail    bool   // does this host provide cage seatbelt
		demanded string
		realized string
		ratified bool
		declined []string // the rules the decline line must name; nil = no decline
	}{
		// Question 3 says yes and question 2 says yes: declined.
		{name: "a whole-tree write deny: shims holds neither rule, so the map is declined",
			cfg: crLinuxMap, front: "cage: seatbelt\ndeny: [Edit, Write]\n", avail: true,
			demanded: CageSeatbelt, realized: CageSeatbelt, declined: []string{"Edit", "Write"}},
		// The same PID, the same map, question 2 answering no.
		{name: "the same PID where this host cannot provide the demanded tier: mapped, and the rule refuses",
			cfg: crLinuxMap, front: "cage: seatbelt\ndeny: [Edit, Write]\n", avail: false,
			demanded: CageSeatbelt, realized: CageShims, ratified: true},
		// Question 3 answering no: the PID loses nothing at shims.
		{name: "a PID whose rules all hold at L1: mapped exactly as today",
			cfg: crLinuxMap, front: "cage: seatbelt\ndeny: [Bash(git push:*)]\n", avail: true,
			demanded: CageSeatbelt, realized: CageShims, ratified: true},
		// The rule is read from PARITY and not from a list of tool names:
		// a path-scoped subtree deny is realized at seatbelt by the trailing
		// deny and by nothing at shims (ADR 0014 §1), and D2 sees it for
		// exactly the same reason it sees a bare Edit.
		{name: "a path-scoped subtree write deny is a rule shims does not hold either",
			cfg: crLinuxMap, front: "cage: seatbelt\ndeny: [Edit(docs/adr/**)]\n", avail: true,
			demanded: CageSeatbelt, realized: CageSeatbelt, declined: []string{"Edit(docs/adr/**)"}},
		// Question 1: nothing to decline. Each of these is a map that
		// changed no tier, and the decline test must not run at all.
		{name: "no map at all",
			cfg: "", front: "cage: seatbelt\ndeny: [Edit, Write]\n", avail: true,
			demanded: CageSeatbelt, realized: CageSeatbelt},
		{name: "an identity row is the one row D7 calls not degradation",
			cfg: "cage_realize_linux:\n  seatbelt: seatbelt\n", front: "cage: seatbelt\ndeny: [Edit, Write]\n", avail: true,
			demanded: CageSeatbelt, realized: CageSeatbelt},
		{name: "a demand the map does not name",
			cfg: crLinuxMap, front: "cage: shims\ndeny: [Bash(git push:*)]\n", avail: true,
			demanded: CageShims, realized: CageShims},
		// A DECLINE ONLY EVER DECLINES TO LOWER. Here the map RAISES, which
		// D7 allows and D2 has nothing to say about: the tier the launch
		// gets is the map's, and no rule of the PID is unrealized there that
		// shims realized.
		{name: "a map that raises a tier is not declined",
			cfg: "cage_realize_linux:\n  shims: seatbelt\n", front: "cage: shims\ndeny: [Edit, Write]\n", avail: true,
			demanded: CageShims, realized: CageSeatbelt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.avail {
				AvailableCages[CageSeatbelt] = true
			} else {
				delete(AvailableCages, CageSeatbelt)
			}
			a := crApp(t, crMapFor2(t, tc.cfg))
			claude, err := a.LoadRuntime("claude")
			if err != nil {
				t.Fatal(err)
			}
			ag := cageAgent(t, a, tc.front)
			res, err := a.RealizeCageFor(ag, claude, tc.demanded)
			if err != nil {
				t.Fatalf("accepted config refused: %v", err)
			}
			if res.Realized != tc.realized {
				t.Errorf("realized = %q, want %q (declined = %q)", res.Realized, tc.realized, res.Declined)
			}
			if (res.Ratified != "") != tc.ratified {
				t.Errorf("ratified = %q, want set=%v", res.Ratified, tc.ratified)
			}
			if (res.Declined != "") != (tc.declined != nil) {
				t.Fatalf("declined = %q, want set=%v", res.Declined, tc.declined != nil)
			}
			if tc.declined == nil {
				return
			}
			// A DECLINE IS NOT A WAIVER: the two fields are never both set,
			// because a decline gives nothing up.
			if res.Ratified != "" {
				t.Errorf("a declined mapping must write no ratified waiver: %q", res.Ratified)
			}
			// The line's own shape, from D2 verbatim: the pair it declined,
			// which tier leaves which rules unrealized, and the tier the
			// launch got WITH ITS MECHANISM, because that is the thing an
			// operator goes and looks at (CageLabel, ADR 0080 D16).
			for _, want := range []string{
				CageRealizeReasonPrefix + runtime.GOOS + " " + CageSeatbelt + "→" + CageShims,
				"declined for this PID",
				CageShims + " leaves " + strings.Join(tc.declined, ", ") + " unrealized",
				"launched at " + CageLabel(CageSeatbelt),
			} {
				if !strings.Contains(res.Declined, want) {
					t.Errorf("the decline line %q does not say %q", res.Declined, want)
				}
			}
			// AND THE MATRIX. The line is a DECLARED DIFFERENCE (ADR 0017
			// §2) and never degradation: the rows it was declined for are
			// realized at the tier the PID asked for, the session is marked
			// nothing, and --allow-degraded has nothing to waive.
			p := a.CheckParity(ag, claude, tc.demanded, TierStrong)
			if p.Cage != tc.realized {
				t.Errorf("parity must read the same tier the resolver did: %q", p.Cage)
			}
			if len(p.Degraded) != 0 || len(p.Ratified) != 0 || p.AnyDegradation() {
				t.Errorf("a decline marks nothing degraded: %+v", p)
			}
			if len(p.DeclaredDifference) != 1 || p.DeclaredDifference[0] != res.Declined {
				t.Errorf("the decline must print as the one declared difference: %v", p.DeclaredDifference)
			}
			for _, rule := range tc.declined {
				if _, ok := p.Realized[rule]; !ok {
					t.Errorf("%s must be realized at the tier the decline handed back: %+v", rule, p)
				}
			}
			// TIER-INDEPENDENT, which is what lets the resolver read parity
			// at DefaultTier and lets its signature take no tier at all. The
			// model tier reaches Degraded (BelowFloor) and NoDegrade and no
			// row of Realized.
			for _, tier := range Tiers {
				again, err := a.RealizeCageFor(ag, claude, tc.demanded)
				if err != nil {
					t.Fatal(err)
				}
				if again.Declined != res.Declined || again.Realized != res.Realized {
					t.Errorf("the resolver answered differently on a second reading: %+v vs %+v", again, res)
				}
				if q := a.CheckParity(ag, claude, tc.demanded, tier); q.Cage != p.Cage || len(q.DeclaredDifference) != 1 {
					t.Errorf("tier %s changed the decline: %+v", tier, q)
				}
			}
		})
	}
}

// THE SUBSUMPTION THE TABLE ABOVE CANNOT SEE (ranger-base-dfgte).
//
// D2's test has two conditions and declineCageMapping spells both, but only
// one of them decides: question 3 ("the mapped tier leaves a rule unrealized
// that the DEMANDED tier realizes") is already false whenever the demanded
// tier is unavailable, because every row seatbelt realizes that shims does
// not sits behind parity's own `a.cageAvailable(cage)`. MEASURED 2026-11-11:
// deleting the `AvailableCages` clause leaves every row of that table green.
//
// So the clause is an early exit, and the property that makes it safe to read
// as one is pinned HERE rather than left implicit in a guard no mutation can
// reach: at a demanded tier this host does not provide, there is nothing for
// question 3 to find. If this reds, that clause has stopped being redundant
// and is the only thing keeping a decline off a tier that holds no rule.
//
// The PID is the strongest shape there is for it — a whole-tree write deny,
// which is the one rule the two tiers genuinely differ on (ADR 0086's census:
// exactly one of eleven host PIDs has it, the security role's).
func TestCageRealizeAnUnavailableDemandRealizesNothingToDeclineFor(t *testing.T) {
	// No t.Parallel(): writes AvailableCages.
	had := AvailableCages[CageSeatbelt]
	delete(AvailableCages, CageSeatbelt)
	t.Cleanup(func() {
		if had {
			AvailableCages[CageSeatbelt] = true
		}
	})
	a := crApp(t, "")
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	ag := cageAgent(t, a, "cage: seatbelt\ndeny: [Edit, Write, Edit(docs/**)]\n")
	// Both readings the resolver would take, at the one tier pair a map may
	// name, with the demanded tier unavailable.
	if lost := a.cageRulesLostByMapping(ag, claude, CageShims, CageSeatbelt); len(lost) != 0 {
		t.Errorf("an unavailable %s realizes nothing %s does not, so there is no rule to decline for: %v", CageSeatbelt, CageShims, lost)
	}
	// The control, and the reason the line above is not an assertion about
	// an empty fixture: the SAME PID, the same two tiers, with the tier
	// available. Every rule is then lost by the mapping.
	AvailableCages[CageSeatbelt] = true
	lost := a.cageRulesLostByMapping(ag, claude, CageShims, CageSeatbelt)
	if len(lost) != 3 {
		t.Fatalf("with %s available this PID loses all three rules to the mapping, got %v", CageSeatbelt, lost)
	}
	// And in the PID's own order, which is what the decline line reads as.
	if got, want := strings.Join(lost, ","), "Edit,Write,Edit(docs/**)"; got != want {
		t.Errorf("lost rules = %q, want %q (the PID's own order, which is the order the decline line names them in)", got, want)
	}
}

// crMapFor2 rewrites a linux-spelled fixture map for whichever OS the suite
// runs on, so every row of the table above exercises the live `runtime.GOOS`
// branch rather than half of them reading an absent key. Fatals on a body it
// cannot rewrite, because a row silently turned into "no map" would pass for
// a reason that is not the property.
func crMapFor2(t *testing.T, body string) string {
	t.Helper()
	if body == "" || runtime.GOOS == "linux" {
		return body
	}
	const key = "cage_realize_linux:"
	if !strings.Contains(body, key) {
		t.Fatalf("fixture map %q names no %s, so rewriting it for %s would make it an absent key", body, key, runtime.GOOS)
	}
	return strings.ReplaceAll(body, key, CageRealizeKey(runtime.GOOS)+":")
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

	// The same map, a PID whose rule the realized tier cannot hold, on a host
	// that does NOT provide the tier it demanded: refused. That is ADR 0083
	// D7's own consequence, and after ADR 0086 D2 this is the shape that
	// still reaches it — declining would hand the launch a tier that
	// realizes nothing.
	delete(AvailableCages, CageSeatbelt)
	err := b.CreateSession(NewSessionOpts{Name: "wall-x", Agent: "wall", Dir: repo})
	AvailableCages[CageSeatbelt] = true
	if err == nil || !strings.Contains(err.Error(), "Edit — needs cage: seatbelt") {
		t.Errorf("the map waives the cage line, never a rule: %v", err)
	}

	// THE DECLINE, AT THE LAUNCH (ADR 0086 D2, ranger-base-dfgte). The same
	// map, the same PID, on a host that DOES provide seatbelt: the mapping is
	// declined, the session is created at the tier the PID demanded, and the
	// record carries no waiver because nothing was waived.
	//
	// This is the arm that pins the LAUNCH's own door against parity's. The
	// resolver is one function (RealizeCageFor) and a QA pin holds that every
	// door reaches it — but a pin over call sites cannot see a launch that
	// resolves correctly and then records something else, which is exactly
	// the failure D2 names: "the session has a wall its record does not
	// name". So the verdict here is the session RECORD, read back off disk.
	t.Run("the declined mapping at the launch", func(t *testing.T) {
		if !had {
			t.Skip("NOTHING MEASURED: this host does not provide cage seatbelt, so a decline back to it is the one thing D2 does not do (the arm above is this host's reading)")
		}
		// The two preconditions of a real launch behind this host's wall,
		// both of them ADR 0086's own findings and neither about D2:
		// wallUngrantedDir, because every blanket temp grant makes the D5
		// witness's create land (and correctly refuse the launch); and
		// CLAUDE_CONFIG_DIR, because unset, a claude seat at this tier is
		// refused by ADR 0080 D19 before any of this is reached.
		claudeConfigDirForTest(t)
		dir := wallUngrantedDir(t)
		if err := b.CreateSession(NewSessionOpts{Name: "wall-d", Agent: "wall", Dir: dir}); err != nil {
			t.Fatalf("a declined mapping launches at the demanded tier, which this host provides: %v", err)
		}
		m, ok := b.readMeta("wall-d")
		if !ok {
			t.Fatal("no session record")
		}
		if m.Cage != CageSeatbelt {
			t.Errorf("the record must name the tier the session GOT, and a declined mapping gets the demand: %q", m.Cage)
		}
		if m.Degraded != "" {
			t.Errorf("a decline is not a waiver: nothing is ratified and nothing is degraded, got %q", m.Degraded)
		}
	})
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

// ADR 0086 D2's half of the standing row: the PIDs this home's map is
// DECLINED for.
//
// WHY THE ROW NEEDS IT. G17's sentence is "every PID demanding a mapped tier
// launches at the realized one and its session is marked degraded", and for a
// declined PID every clause of that is false — it launches at its demand, it
// is not marked, and it carries no waiver. An operator reading the row beside
// a `posse gates <that persona>` block that says `seatbelt (bwrap)` over a map that
// lowers seatbelt has two surfaces disagreeing, and the one that is wrong is
// the standing row.
//
// NAMED AND NOT JUST COUNTED, because the next question after "declined for
// 1" is "which one".
func TestGovG17NamesThePIDsTheMapIsDeclinedFor(t *testing.T) {
	// No t.Parallel(): writes AvailableCages.
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	t.Cleanup(func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	})
	b, _ := newTestBackend(t)
	govRepo(t, b)
	crCfgPrepend(t, b.App, crMapFor(runtime.GOOS))
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Two PIDs under one map: one whose rules all hold at the mapped tier,
	// and one the mapping would cost a rule. Only the second is declined,
	// which is what makes the row's list a reading and not a persona census.
	os.WriteFile(filepath.Join(b.App.AgentsDir, "walled.md"),
		[]byte("---\nname: walled\ncage: seatbelt\ndeny: [Edit, Write]\n---\nYou are walled.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "lowered.md"),
		[]byte("---\nname: lowered\ncage: seatbelt\ndeny: [Bash(git push:*)]\n---\nYou are lowered.\n"), 0o644)

	st, err := b.App.CageRealizeStanding()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Declined) != 1 || st.Declined[0] != "walled" {
		t.Fatalf("the standing reading must name exactly the declined PID: %v", st.Declined)
	}
	g := find(shopSet(t, govIn(t, b)), "G17")
	if g == nil {
		t.Fatalf("no G17 while the map lowers a tier: %v", shopSet(t, govIn(t, b)).Keys())
	}
	for _, want := range []string{"declined for 1 PID(s)", "walled", "ADR 0086 D2"} {
		if !strings.Contains(g.Detail, want) {
			t.Errorf("the row does not say %q: %q", want, g.Detail)
		}
	}
	// And the row still says what it always said about the PIDs the map DOES
	// lower — the decline is an addition, not a replacement.
	if !strings.Contains(g.Detail, "seatbelt→shims") || !strings.Contains(g.Detail, "waives the cage line only") {
		t.Errorf("the declined half must not displace the row's own sentence: %q", g.Detail)
	}
	// THE CONTROL: with the declined PID gone the clause is absent entirely,
	// so "declined for 0 PID(s)" is never printed over a map that declines
	// for nobody.
	if err := os.Remove(filepath.Join(b.App.AgentsDir, "walled.md")); err != nil {
		t.Fatal(err)
	}
	st2, err := b.App.CageRealizeStanding()
	if err != nil {
		t.Fatal(err)
	}
	if len(st2.Declined) != 0 || strings.Contains(st2.GovDetail(), "declined for") {
		t.Errorf("a map that declines for nobody says nothing about declining: %v / %q", st2.Declined, st2.GovDetail())
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
