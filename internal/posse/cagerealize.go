package posse

// `cage_realize_<goos>:` — the per-OS map from the cage tier a PID DEMANDS to
// the tier THIS host realizes it with (ADR 0083 D7).
//
// WHY THE KEY EXISTS. A PID's `cage:` line is a statement about what its
// rules need, and the constitution that carries it is shared across hosts:
// the same promoted `agents/*.md` are read on the darwin laptop, where
// `seatbelt` is a real wall, and on the linux host, where nothing realizes it
// until the bubblewrap tier lands. Before this key the only two ways to run
// that PID on the host were a hand-edited promoted copy — which every launch
// re-hashes against `promoted.json` and dispatch refuses (ADR 0015 §3) — or
// rewriting the PID's `cage:` line down to `shims` in the constitution, which
// works on the host and LIES everywhere else. The need is the PID's; how a
// host meets it is the host's, so the answer is a map keyed by GOOS, in the
// promoted set, hashed by the manifest and shown in promote's diff.
//
// The field's name for the shape: a class with a per-node handler
// (Kubernetes RuntimeClass — the pod names a class, the node's config names
// the handler that realizes it).
//
// WHAT IT WAIVES, AND WHAT IT CANNOT. A map changes the CAGE and nothing
// else. Parity is recomputed at the realized tier and every rule is then
// judged THERE, so a rule the realized tier cannot hold still refuses the
// launch — a PID carrying a bare `Edit`/`Write` deny mapped seatbelt→shims
// refuses exactly as it does today. The map waives the cage line, never a
// rule.
//
// Mapping to a LOWER tier is RATIFIED DEGRADATION: the launch proceeds,
// dispatch included, because the waiver was ratified at promote and not
// invented by dispatch (ADR 0002 §4's rule is about who invents it, and it
// stands). It is carried in Parity.Ratified rather than Parity.Degraded, so
// it marks the session and prints on every surface without being the
// --allow-degraded wall that a gate nobody realizes is. Mapping to an equal
// or higher tier is not degradation at all.
//
// ─── THE SPELLING IS A KEY FAMILY, AND IT IS A DIVERGENCE (ranger-base-1155n)
//
// ADR 0083 D7 writes the key as a two-level map:
//
//	cage_realize:
//	  linux:
//	    seatbelt: shims
//
// and this harness cannot read that. Its YAML is a deliberate flat subset —
// top-level keys plus ONE nesting level, "no anchors/multiline/deeper
// nesting" (yamlflat.go, docs/notes.d/notes-yaml.md) — and the same wall has
// been met twice before by the same answer: a KEY FAMILY whose members the
// harness does not know in advance, `plan_guard_<window>:` (planusage.go) and
// `interstitial_<name>:` (runtimeyaml.go, "a family rather than a list
// because flat-YAML has no list-of-maps"). So the spelling here is
//
//	cage_realize_linux:
//	  seatbelt: shims
//
// — one top-level key per GOOS, one nesting level, read by the readers that
// already exist. Extending the subset to a second nesting level for one key
// with one reader would be a new primitive in the format every config,
// recipe, meta file and PID frontmatter in this tree is written in.
//
// The cost of a family is that a mistyped member reads as ABSENT rather than
// as an error, which is the exact failure ADR 0080 D9 was ruled on ("a typo
// read as a default arms the wrong thing"). That is why validation here is
// total rather than per-lookup: the bare `cage_realize:` key ADR 0083 wrote
// is refused BY NAME, every `cage_realize_*` member is checked whether or not
// it is this GOOS's, and an unknown GOOS, a non-map value, an unknown tier,
// `container` on either side and a repeated demand are all refusals. Nothing
// here defaults.

import (
	"fmt"
	"runtime"
	"strings"
)

const (
	// CageRealizeKeyPrefix heads the key family: one `cage_realize_<goos>:`
	// per OS.
	CageRealizeKeyPrefix = "cage_realize_"

	// CageRealizeNestedKey is the spelling ADR 0083 D7 wrote — a bare
	// `cage_realize:` with a GOOS map under it. It is refused by name rather
	// than ignored: the flat reader cannot see a second nesting level at
	// all, so an operator who typed the ADR's shape would get a key that
	// parses to nothing, arms nothing, and says nothing.
	CageRealizeNestedKey = "cage_realize"

	// CageRealizeReasonPrefix heads the ratified-degradation reason this map
	// writes into Parity.Ratified and, through it, into a session's
	// `degraded:` meta field. Two readers subtract on it —
	// DegradedBeyondRatified's callers — so it is a constant and not a
	// phrase typed twice.
	CageRealizeReasonPrefix = "cage_realize: "
)

// CageRealizeGOOS is the closed set of OS keys. posse has exactly two ports
// with a real implementation behind them — the per-GOOS files in this package
// are darwin/linux/other, and `other` refuses rather than runs (loadavg_other.go,
// backupvolume_other.go) — so a third name here is a mapping for a host this
// build cannot be, and a typo is far likelier than a port nobody mentioned.
// Refused, never ignored, for the reason the header gives.
var CageRealizeGOOS = []string{"darwin", "linux"}

// CageRealizeKey is the config key for one OS.
func CageRealizeKey(goos string) string { return CageRealizeKeyPrefix + goos }

func validCageRealizeGOOS(goos string) bool {
	for _, g := range CageRealizeGOOS {
		if g == goos {
			return true
		}
	}
	return false
}

// CageRealizeEntry is one row of the map: the tier a PID demands, and the
// tier this host realizes it with.
type CageRealizeEntry struct{ From, To string }

// Lowers reports whether this row hands the launch a WEAKER tier than the one
// it asked for — the test for ratified degradation, by cageRank and never by
// name, so a tier inserted into Cages later is ordered by the one list that
// orders them.
func (e CageRealizeEntry) Lowers() bool { return cageRank(e.To) < cageRank(e.From) }

func (e CageRealizeEntry) String() string { return e.From + "→" + e.To }

// CageRealizeEntries validates the WHOLE `cage_realize_*` family in cfg and
// returns goos's rows in file order.
//
// Total validation, not a lookup: an error here is about some other OS's key
// as readily as about this one, because a map a host cannot read is a map
// whose other half nobody has checked either, and because the member that
// gets mistyped is the one being edited rather than the one being read. Every
// caller surfaces the error in its own shape (the launch refuses, dispatch
// refuses per bead, `posse status` reports the reading as unknown) — the
// `autostart_via:` pattern, where one key's refusal is spoken by each of its
// readers (ADR 0080 D9).
//
// An ABSENT family is (nil, nil): no mapping, every tier realizes itself, and
// that is today's behaviour exactly. So is a bare `cage_realize_linux:` with
// nothing indented under it — an empty map is no entries, and the non-map
// check below is what catches the shape that LOOKS like a value.
func CageRealizeEntries(cfg, goos string) ([]CageRealizeEntry, error) {
	if yamlHasKey(cfg, CageRealizeNestedKey) {
		return nil, Die("%s: %q is not a key this config reader can see — its YAML is a flat subset with one nesting level, and ADR 0083 D7's `%s:` / `<goos>:` / `<demanded>: <realized>` is two. Write one key per OS instead: `%s` with `  <demanded>: <realized>` under it (ADR 0083 D7, ranger-base-1155n)",
			AbbrevHome(cfg), CageRealizeNestedKey+":", CageRealizeNestedKey, CageRealizeKey(goos)+":")
	}
	var out []CageRealizeEntry
	for _, key := range YamlKeysWithPrefix(cfg, CageRealizeKeyPrefix) {
		g := strings.TrimPrefix(key, CageRealizeKeyPrefix)
		if !validCageRealizeGOOS(g) {
			return nil, Die("%s: %s: names no OS this posse runs on (want %s) — a cage map for a host this build cannot be is a typo, and a typo read as absence arms nothing while looking armed (ADR 0083 D7)",
				AbbrevHome(cfg), key, strings.Join(CageRealizeGOOS, " | "))
		}
		if v := YamlGet(cfg, key); v != "" {
			return nil, Die("%s: %s: %s — this key is a MAP of demanded tier to realized tier, not a scalar. Write it as a block:\n    %s:\n      %s: %s",
				AbbrevHome(cfg), key, v, key, CageSeatbelt, CageShims)
		}
		rows, err := cageRealizeRows(cfg, key)
		if err != nil {
			return nil, err
		}
		if g == goos {
			out = rows
		}
	}
	return out, nil
}

// cageRealizeRows validates one member's pairs.
func cageRealizeRows(cfg, key string) ([]CageRealizeEntry, error) {
	var rows []CageRealizeEntry
	// The demanded tier → its answer. One map and not a `seen` set beside it:
	// a key is present here exactly when it has been answered, and the
	// idempotence loop below needs the answer and not just the fact.
	target := map[string]string{}
	for _, kv := range YamlMapPairs(cfg, key) {
		e := CageRealizeEntry{From: kv[0], To: kv[1]}
		bad := func(why string) error {
			return Die("%s: %s: %s: %s — %s (ADR 0083 D7)", AbbrevHome(cfg), key, e.From, e.To, why)
		}
		switch {
		case !ValidCage(e.From):
			return nil, bad(fmt.Sprintf("%q is not a cage tier (want %s | %s)", e.From, CageShims, CageSeatbelt))
		case e.To == "":
			return nil, bad("names no realized tier; a demand mapped to nothing is not a waiver")
		case !ValidCage(e.To):
			return nil, bad(fmt.Sprintf("%q is not a cage tier (want %s | %s)", e.To, CageShims, CageSeatbelt))
		case e.From == CageContainer || e.To == CageContainer:
			// ADR 0083 D7: the container tier's engine is per home already
			// (`default_engine:`, a.cageAvailable's engine probe), so it is
			// not a class a host-wide GOOS map can hand off or stand in for.
			return nil, bad("cage " + CageContainer + " may appear on neither side of this map: its engine is chosen per home, not per OS")
		// A non-empty answer is already recorded for this demand. Reached
		// only after the `e.To == ""` arm above, so a present key always
		// carries a real tier.
		case target[e.From] != "":
			return nil, bad("the demanded tier " + e.From + " is mapped twice; one answer per demand")
		}
		target[e.From] = e.To
		rows = append(rows, e)
	}
	// IDEMPOTENT, OR REFUSED (ranger-base-pl3ck). A launch applies this map
	// TWICE: planLaunch resolves the demand and hands the REALIZED tier to
	// checkParityIn, which resolves again (herdrback.go, parity.go — and that
	// second reading is load-bearing, it is why parity can ask the map about
	// ag.Cage too). So correctness rests on the map answering the same thing
	// the second time, which nothing above checks.
	//
	// MEASURED before the refusal existed: a member carrying both
	// `shims: seatbelt` and `seatbelt: shims` was accepted as two rows, and
	// then one application of it realized `shims` as `seatbelt` while a
	// second realized it as `shims` — so the launch rendered the seatbelt
	// wall at the tier planLaunch resolved while parity, the printed block
	// and the session record all said shims, and carried a spurious
	// `cage_realize: <goos> seatbelt→shims` ratified waiver for a launch that
	// asked for shims.
	//
	// The test is "a realized target that is itself a demanded key, answered
	// with something other than itself". Identity (`seatbelt: seatbelt`)
	// passes: it is a key whose answer IS itself, so a second application
	// changes nothing — and it is the one row D7 accepts as "not degradation
	// at all". Refused here with the other "nothing defaults" refusals, in
	// file order, so an operator reads one sentence about the pair.
	for _, e := range rows {
		if to, ok := target[e.To]; ok && to != e.To {
			return nil, Die("%s: %s: %s: %s — the realized tier %s is itself a demanded tier in this map, answered with %s. A launch applies this map twice (the demand at plan, the realized tier again at parity), so this one would resolve to %s once and %s the second time: the wall rendered at one tier and the parity block, the printed report and the session record all saying the other (ADR 0083 D7, ranger-base-pl3ck)",
				AbbrevHome(cfg), key, e.From, e.To, e.To, to, e.To, to)
		}
	}
	return rows, nil
}

// CageRealizeResolution is the answer for one launch: the tier demanded, the
// tier that realizes it, and — only where the map LOWERS it — the ratified
// degradation reason that marks the session.
type CageRealizeResolution struct {
	GOOS     string
	Demanded string
	Realized string
	Ratified string // "" unless the map hands the launch a weaker tier
}

// RealizeCageIn maps demanded through cfg's map for goos. ratifier is what
// ratified the map, rendered by cageRealizeRatifier — passed in rather than
// read here so the whole resolution is a pure function of (config, OS,
// provenance, demand) and the table that pins it needs no home on disk.
//
// On an error the resolution still answers the DEMANDED tier: a map posse
// cannot read waives nothing, which is the safe direction. The error is what
// the callers refuse on.
func RealizeCageIn(cfg, goos, ratifier, demanded string) (CageRealizeResolution, error) {
	r := CageRealizeResolution{GOOS: goos, Demanded: demanded, Realized: demanded}
	rows, err := CageRealizeEntries(cfg, goos)
	if err != nil {
		return r, err
	}
	for _, e := range rows {
		if e.From != demanded {
			continue
		}
		r.Realized = e.To
		if e.Lowers() {
			r.Ratified = cageRealizeReason(goos, e, ratifier)
		}
		break
	}
	return r, nil
}

// cageRealizeReason is the one sentence this map writes into Parity.Ratified
// and the session meta. ADR 0083 D7's own shape: who, from where to where,
// and what ratified it.
//
// It carries NO "; " (ranger-base-pl3ck). The field it lands in is a
// "; "-joined list — CreateSession joins Parity's lists that way — and
// DegradedBeyondRatified splits on that separator and asks each part for this
// prefix. The goos and the entry are validated tiers, but the ratifier is
// free text: one of cageRealizeRatifier's four states quotes an os error
// (`<manifest> is unreadable: ` + err.Error()), and an error text carrying
// "; " would split this one reason into a second part with no prefix — read
// as REAL degradation, so RecreateOpts arms AllowDegraded, the exact
// direction that predicate exists to prevent. Stripped where the reason is
// built rather than guarded at the reader: the reader's contract is the
// separator, and this is the one writer that can break it.
func cageRealizeReason(goos string, e CageRealizeEntry, ratifier string) string {
	return fmt.Sprintf("%s%s %s (%s)", CageRealizeReasonPrefix, goos, e, strings.ReplaceAll(ratifier, "; ", ", "))
}

// RealizeCage is RealizeCageIn for this App on this OS.
func (a *App) RealizeCage(demanded string) (CageRealizeResolution, error) {
	return RealizeCageIn(a.ConfigPath, runtime.GOOS, a.cageRealizeRatifier(), demanded)
}

// cageRealizeRatifier names what ratified the map, for the reason string.
//
// The four states are AnchorStateLine's four, read from the same manifest
// (anchorstate.go), and this is deliberately a second SENTENCE over the same
// reading rather than a second reading: that one is the home's standing
// anchor line ("constitution: promoted <sha> <when>"), this one is a
// parenthetical inside a launch's degradation reason. An absent manifest is
// not an error here for the reason it is not one there — every pre-ADR-0015
// home and every test rig has none — but it is not silent either: a waiver
// nothing anchors says so, because "ratified at promote" is the whole of why
// this degradation is allowed to launch.
func cageRealizeRatifier(m *PromoteManifest, err error) string {
	switch {
	case err != nil:
		return PromoteManifestFile + " is unreadable: " + err.Error()
	case m == nil:
		return "config.yaml — this home was never promoted"
	case m.Seeded:
		return "seeded " + PromoteManifestFile + ", no commit behind it"
	case m.SHA != "":
		return "promoted " + short(m.SHA)
	default:
		return "promoted, but " + PromoteManifestFile + " records no commit"
	}
}

func (a *App) cageRealizeRatifier() string {
	return cageRealizeRatifier(ReadPromoteManifest(a.PromoteManifestPath()))
}

// ─── the standing reading `posse status` carries ─────────────────────────────

// CageRealizeStanding is the whole of what this OS's map lowers right now.
// ADR 0083 D7 asks for ONE standing row while any mapping lowers a tier — not
// one per mapped session, which is what reusing G15 would have produced on a
// host where every code persona is mapped.
type CageRealizeStanding struct {
	GOOS     string
	Lowered  []CageRealizeEntry
	Ratifier string
}

// Any reports whether this OS's map lowers anything at all.
func (s CageRealizeStanding) Any() bool { return len(s.Lowered) > 0 }

// GovDetail is the row's sentence. It names the entries, because a row that
// only said "a cage map is in force" is a label and the operator's remedy is
// per entry; and it says what the map does NOT waive, because that is the
// half a reader is most likely to assume wrong.
func (s CageRealizeStanding) GovDetail() string {
	var parts []string
	for _, e := range s.Lowered {
		parts = append(parts, e.String())
	}
	return fmt.Sprintf("%s: %s (%s) — every PID demanding a mapped tier launches at the realized one and its session is marked degraded. The map waives the cage line only: a rule the realized tier cannot hold still refuses the launch. Ratified at promote, so dispatch launches it; raise the map when this host can realize the demanded tier (ADR 0083 D7)",
		CageRealizeKey(s.GOOS), strings.Join(parts, ", "), s.Ratifier)
}

// CageRealizeStandingFor reads cfg's map for goos and keeps the rows that
// lower a tier.
func CageRealizeStandingFor(cfg, goos, ratifier string) (CageRealizeStanding, error) {
	s := CageRealizeStanding{GOOS: goos, Ratifier: ratifier}
	rows, err := CageRealizeEntries(cfg, goos)
	if err != nil {
		return s, err
	}
	for _, e := range rows {
		if e.Lowers() {
			s.Lowered = append(s.Lowered, e)
		}
	}
	return s, nil
}

// CageRealizeStanding is CageRealizeStandingFor for this App on this OS.
func (a *App) CageRealizeStanding() (CageRealizeStanding, error) {
	return CageRealizeStandingFor(a.ConfigPath, runtime.GOOS, a.cageRealizeRatifier())
}

// ─── reading a session's degraded field back ─────────────────────────────────

// DegradedBeyondRatified reports whether a session's `degraded:` field names
// anything other than ratified `cage_realize:` lines.
//
// TWO READERS, ONE PREDICATE, and both of them are about not reading a
// ratified waiver as the operator's own:
//
//   - RecreateOpts passes `AllowDegraded` back into the relaunch, on the
//     ground that "the operator consented to this session's degradation when
//     it was created". Nobody consents to this one at the keyboard — it is in
//     the promoted config — so a ratified-only meta must not arm the flag, or
//     a host whose map lowers every code persona would waive each seat's NEXT,
//     real degradation silently, at every relaunch. (A control that arms
//     itself is a bug even when nothing fails.)
//   - G15 says a live session "is running with gates its wall does not
//     realize … waived once with --allow-degraded and re-armed by every
//     relaunch from its meta". Every clause of that is false for a ratified
//     map, and the standing row (G17) is where the map is reported instead.
//
// The field is a "; "-joined list (CreateSession joins Parity's lists that
// way, and yamlflat.go's reader gives it back as one line), and the reason
// this map writes carries no "; " of its own — not because its own text has
// none, but because cageRealizeReason strips it from the one part that is
// free text, the ratifier (ranger-base-pl3ck). Read that comment before
// trusting this sentence.
func DegradedBeyondRatified(degraded string) bool {
	for _, part := range strings.Split(degraded, "; ") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if !strings.HasPrefix(part, CageRealizeReasonPrefix) {
			return true
		}
	}
	return false
}
