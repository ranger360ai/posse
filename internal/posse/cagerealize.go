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
	// Declined (ADR 0086 D2) is the one DeclaredDifference line a DECLINED
	// mapping prints, and "" for every mapping that applied. A decline is
	// the opposite of Ratified and the two are never both set: Realized is
	// the DEMAND, there is no waiver to ratify, nothing is marked degraded,
	// and the line only says out loud that the map was read and not taken.
	Declined string
}

// realizeCageIn maps demanded through cfg's map for goos, and it is the MAP
// AND NOTHING ELSE — ADR 0083 D7's resolution, before ADR 0086 D2's decline
// is asked about. ratifier is what ratified the map, rendered by
// cageRealizeRatifier — passed in rather than read here so this half is a
// pure function of (config, OS, provenance, demand) and the table that pins
// it needs no home on disk.
//
// On an error the resolution still answers the DEMANDED tier: a map posse
// cannot read waives nothing, which is the safe direction. The error is what
// the callers refuse on.
//
// UNEXPORTED SINCE ADR 0086 D2 (ranger-base-dfgte), and that is the whole of
// what makes "one resolver" checkable rather than advisory. D2's decline
// needs the PID and the runtime, which this function does not take, so a
// caller that resolved the map here would get a tier with the decline left
// out — the launch at one tier and parity at another, which is the defect D2
// names as the thing to design out. Unexported, no package outside this one
// can reach it at all, and inside it the only caller is RealizeCageFor
// (TestQAOneCageRealizeResolverAndEveryDoorReachesIt).
func realizeCageIn(cfg, goos, ratifier, demanded string) (CageRealizeResolution, error) {
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

// ─── ADR 0086 D2 · one resolver, and the mapping it declines ────────────────

// RealizeCageFor is THE ONE RESOLVER: the map for this App on this OS, asked
// for a concrete PID on a concrete runtime, with D2's decline applied.
//
// EVERY DOOR THAT READS THE MAP CALLS THIS. There are four — CreateSession's
// launch path (herdrback.go), both halves of CheckParity (the demand and the
// relaunch's by-PID reading, parity.go) and dispatch's CheckTier, which
// reaches it through CheckParity — and the reason they must agree is not
// tidiness: if the launch resolves one tier and parity another, the session
// runs behind a wall its own record does not name. That is the defect ADR
// 0086 D2 asks the build to design out, so `realizeCageIn` is unexported and
// this is the only thing that calls it
// (TestQAOneCageRealizeResolverAndEveryDoorReachesIt).
//
// ag and rt are what D2's test needs and `realizeCageIn` cannot have: the
// decline is per PID and per runtime, because which rules a tier realizes is
// a fact about the PID's deny list read through the runtime.
func (a *App) RealizeCageFor(ag *AgentFile, rt *Runtime, demanded string) (CageRealizeResolution, error) {
	res, err := realizeCageIn(a.ConfigPath, runtime.GOOS, a.cageRealizeRatifier(), demanded)
	if err != nil {
		return res, err
	}
	return a.declineCageMapping(ag, rt, res), nil
}

// declineCageMapping is ADR 0086 D2, the rule itself: a mapping is DECLINED
// for a PID it would leave with a rule unrealized, when this host provides
// the tier that PID demanded. The PID then launches at its demand.
//
// THIS AMENDS ADR 0083 D7 AND LEAVES ITS WORDS STANDING. A map changes the
// cage and nothing else, and it cannot waive a rule — D7 then REFUSED such a
// PID, because parity was recomputed at the mapped tier and the rule went
// unrealized there. D2 gives it the tier it asked for instead. Both halves of
// D7's sentence survive: the map still waives no rule, and now it does not
// cost one either.
//
// THE TEST IS THREE QUESTIONS, and the order is the cheap ones first:
//
//  1. did the map change anything at all. An absent family, a demand with no
//     row, and an identity row (`seatbelt: seatbelt`, the one row D7 calls
//     "not degradation at all") all answer no, and nothing below runs.
//
//  2. is the DEMANDED tier in AvailableCages. If this host cannot provide it,
//     declining would hand the launch a tier that realizes nothing — so the
//     map applies and the rule refuses, exactly as today (`cage %s is not
//     available on this host/build`, parity.go).
//
//     AN EARLY EXIT AND NOT THE DECIDER, and this is said here because the
//     code reads like the opposite. Question 3 already subsumes it: the only
//     tiers a map may name are shims and seatbelt (`container` is refused on
//     both sides), every row seatbelt realizes that shims does not is behind
//     parity's own `seatbelt := a.cageAvailable(cage) && …`, and a tier this
//     host does not provide therefore realizes nothing extra for question 3
//     to find. MEASURED 2026-11-11: deleting this clause leaves every row of
//     TestCageRealizeDeclinesAMappingThatWouldCostAPIDARule green, the
//     "cannot provide the demanded tier" row included. It stays because it
//     is D2's own wording and because it is the cheap question — an
//     unavailable tier answers here instead of through two parity readings —
//     and the SUBSUMPTION is what is pinned, by
//     TestCageRealizeAnUnavailableDemandRealizesNothingToDeclineFor. If that
//     pin ever reds, this clause is the only thing standing between a
//     decline and a tier that holds nothing.
//
//  3. does the mapped tier leave a rule unrealized that the demanded tier
//     realizes. If not, the PID loses nothing by being lowered and the map
//     applies exactly as today, ratified waiver and all.
//
// A DECLINE IS NOT A WAIVER AND NOT A RAISE. It writes no Ratified row and
// marks nothing degraded, because nothing was given up; and it can only ever
// answer the DEMAND, so it never raises a PID above what the PID named and
// never picks a tier nobody asked for. The one thing it leaves behind is the
// Declined line, which parity prints as a DeclaredDifference — safe by
// design, never counted toward Degraded (ADR 0017 §2).
func (a *App) declineCageMapping(ag *AgentFile, rt *Runtime, res CageRealizeResolution) CageRealizeResolution {
	mapped := res.Realized
	if ag == nil || rt == nil || mapped == res.Demanded || !a.cageAvailable(res.Demanded) {
		return res
	}
	lost := a.cageRulesLostByMapping(ag, rt, mapped, res.Demanded)
	if len(lost) == 0 {
		return res
	}
	res.Realized = res.Demanded
	res.Ratified = ""
	res.Declined = fmt.Sprintf("%s%s %s→%s declined for this PID: %s leaves %s unrealized; launched at %s (ADR 0086 D2)",
		CageRealizeReasonPrefix, res.GOOS, res.Demanded, mapped, mapped,
		strings.Join(lost, ", "), CageLabel(res.Demanded))
	return res
}

// cageRulesLostByMapping is question 3: the PID's own rules that go
// unrealized at mapped and are realized at demanded, in the order the PID
// wrote them.
//
// IT MUST NOT RE-ENTER THE MAP, or it loops: the map's answer is what is
// being decided. So both readings go through checkParityAtRealizedTier
// (parity.go), the variant that takes a tier ALREADY realized and reads no
// `cage_realize_*` key at all.
//
// THE RULES AND NOT THE Unrealized LIST. Parity's unrealized entries are
// `<gate> — <why>` sentences built for a reader, so recovering the rule from
// one would be a parse of prose; and the rows are keyed by the rule exactly
// as the PID wrote it (wholeTreeWriteWall's `gate` parameter), so membership
// in Realized answers this directly. Every deny rule lands in one of the two
// — Realized or Unrealized — so "not realized here" is "unrealized here".
//
// TIER-INDEPENDENT, which is why DefaultTier is passed and no caller has to
// have a model tier in hand. The model tier reaches Degraded (BelowFloor) and
// NoDegrade and touches no row of Realized; the decline arm of
// cagerealize_test.go asserts that over every tier.
func (a *App) cageRulesLostByMapping(ag *AgentFile, rt *Runtime, mapped, demanded string) []string {
	low := a.checkParityAtRealizedTier(ag, rt, mapped, DefaultTier)
	high := a.checkParityAtRealizedTier(ag, rt, demanded, DefaultTier)
	var out []string
	said := map[string]bool{}
	for _, rule := range ag.Deny {
		if said[rule] {
			continue
		}
		if _, ok := low.Realized[rule]; ok {
			continue
		}
		if _, ok := high.Realized[rule]; !ok {
			continue
		}
		said[rule] = true
		out = append(out, rule)
	}
	return out
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
	// Declined (ADR 0086 D2) names the PIDs of this home whose mapping is
	// declined right now — the seats the standing row is NOT about, said in
	// the row because the row's sentence ("every PID demanding a mapped tier
	// launches at the realized one") is false for them.
	//
	// Filled only by the App reading below, and never by
	// CageRealizeStandingFor: it is a fact about this home's PIDs and their
	// runtimes, not about the config text that function is handed, and the
	// table that pins the text wants no agents dir on disk.
	Declined []string
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
	// ADR 0086 D2's half of the row. "EVERY PID demanding a mapped tier" is
	// what the sentence above says, and for a declined PID it is not true —
	// so the count is said here rather than left for a reader to reconcile
	// against a `posse gates` block that names seatbelt over a map that
	// lowers it. The PIDs are NAMED and not just counted, because the
	// operator's next question after "declined for 1" is "which one", and
	// the answer is one short list on a host with eleven seats.
	declined := ""
	if len(s.Declined) > 0 {
		declined = fmt.Sprintf(" — declined for %d PID(s): %s, which launch at their demanded tier because the mapped one leaves a rule of theirs unrealized (ADR 0086 D2); those sessions are not degraded and carry no waiver",
			len(s.Declined), strings.Join(s.Declined, ", "))
	}
	return fmt.Sprintf("%s: %s (%s) — every PID demanding a mapped tier launches at the realized one and its session is marked degraded. The map waives the cage line only: a rule the realized tier cannot hold still refuses the launch. Ratified at promote, so dispatch launches it; raise the map when this host can realize the demanded tier (ADR 0083 D7)%s",
		CageRealizeKey(s.GOOS), strings.Join(parts, ", "), s.Ratifier, declined)
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

// CageRealizeStanding is CageRealizeStandingFor for this App on this OS,
// plus the PIDs this home's map is declined for (ADR 0086 D2).
//
// The PID walk is skipped where the map lowers nothing, because there is then
// no row to print and nothing to decline — which keeps `posse status` on a
// home with no map exactly as cheap as it was.
func (a *App) CageRealizeStanding() (CageRealizeStanding, error) {
	s, err := CageRealizeStandingFor(a.ConfigPath, runtime.GOOS, a.cageRealizeRatifier())
	if err != nil || !s.Any() {
		return s, err
	}
	s.Declined = a.cageRealizeDeclinedPIDs()
	return s, nil
}

// cageRealizeDeclinedPIDs asks the one resolver about every PID of this home,
// at the cage that PID itself demands, and names the ones whose mapping is
// declined.
//
// A PID THAT CANNOT BE READ IS NOT COUNTED, in either direction: a persona
// file that fails to load or a runtime that fails to resolve cannot launch at
// all (`posse agent check` and the launch both refuse it), so it is neither a
// declined seat nor a mapped one, and naming it here would put a broken PID's
// name in a row about the cage map. Those refusals have their own surfaces —
// G17 is not one of them.
//
// ListAgents is sorted, so the row's list is stable between two readings of
// the same home (rangerhq-epes' rule: a report two runs cannot diff is a
// report nobody reads twice).
func (a *App) cageRealizeDeclinedPIDs() []string {
	var out []string
	for _, name := range a.ListAgents() {
		ag, err := a.LoadAgent(name)
		if err != nil || ag == nil {
			continue
		}
		rt, err := a.LoadRuntime(a.ResolveRuntime("", ag))
		if err != nil || rt == nil {
			continue
		}
		if res, err := a.RealizeCageFor(ag, rt, ResolveCage("", ag)); err == nil && res.Declined != "" {
			out = append(out, name)
		}
	}
	return out
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
