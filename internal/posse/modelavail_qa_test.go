//go:build posse_arm3

package posse

// ranger-base-7vp — verifying the close of rangerhq-oay (the tier
// availability preflight).
//
// The delivered pins walk the resolution, the map, the cache and ONE
// launch. What they never walk is the SECOND launch: the mark rule (2)
// writes into the meta is re-derived at every launch from the pair the
// launch was asked for, and `posse relaunch` asks for the pair the LAST
// launch fell to. The two tests below that carry a skip are the boards
// that walk finds, measured 2026-08-28.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// qaPID writes a PID whose gates the wall fully realizes at shims, with
// whatever extra front-matter lines the board needs.
func qaPID(t *testing.T, b *HerdrBackend, name, tier string, extra ...string) {
	t.Helper()
	os.MkdirAll(b.App.AgentsDir, 0o755)
	body := fmt.Sprintf("---\nname: %s\ntier: %s\n%sdeny: [Bash(git push:*)]\n---\nYou are %s.\n",
		name, tier, strings.Join(extra, ""), name)
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// qaOffPairSession is what qaFellSession became when ADR 0003 §3 removed
// automatic substitution (ranger-base-hv2zr). The old helper produced a
// session running a pair its PID did not ask for by letting the preflight
// substitute one; nothing does that any more, so the board is built the two
// ways it can still arise:
//
//   - the operator's own explicit `--tier`, which is the whole of §3's
//     "an operator can select another explicitly"; and
//   - a session created BEFORE this removal, whose meta on disk still
//     carries the substituted pair and the `fallback:` line the old code
//     wrote. That is the existing-session transition §3 says must be priced
//     and verified, and `legacy` plants exactly those bytes.
//
// Either way what comes back is a session whose meta records claude/standard
// under a PID that asks for strong — the state every assertion below is
// about — and the mark is never RE-created, only ever found.
func qaOffPairSession(t *testing.T, name string, legacy bool) (*HerdrBackend, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	b.Warn = &strings.Builder{}
	qaPID(t, b, "architect", TierStrong)
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone
	if err := b.CreateSession(NewSessionOpts{Name: name, Agent: "architect", Tier: TierStandard, Dir: t.TempDir()}); err != nil {
		t.Fatalf("the preflight must never refuse a launch (rule 3): %v", err)
	}
	m, ok := b.readMeta(name)
	if !ok || m.Tier != TierStandard || m.Runtime != "claude" {
		t.Fatalf("board not set up: the create must record the pair it opened on: %+v", m)
	}
	if legacy {
		// The bytes an old posse left behind, appended to the meta the new
		// one just wrote. Nothing in the product writes this line now, so
		// planting it by hand is the only way to ask what a reader does
		// when it finds one.
		p := b.metaPath(name)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		old := string(raw) + "fallback: architect: tier strong wants claude-fable-5-1 — unavailable, falling back to claude-opus-5\n"
		if err := os.WriteFile(p, []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return b, fake
}

// ─── the second launch ───────────────────────────────────────────────────────

// The crash-restart path re-types into the live pane and writes the meta
// back. What must ride through is the session's PAIR: a re-type is not a
// re-decision, and a session running standard must be re-typed on standard
// whatever its PID says today.
func TestQA7vpRelaunchAgentKeepsTheSessionsPair(t *testing.T) {
	t.Parallel()
	b, fake := qaOffPairSession(t, "ra", false)

	m, _ := b.readMeta("ra")
	m.Launched = time.Now().Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(fake, "agents.json")) // the CLI died
	ok, err := b.RelaunchAgent("ra", time.Second)
	if err != nil || !ok {
		t.Fatalf("relaunch: ok=%v err=%v", ok, err)
	}

	m2, _ := b.readMeta("ra")
	if m2.Tier != TierStandard || m2.Runtime != "claude" {
		t.Errorf("meta pair = %s/%s, want the pair that is really running", m2.Runtime, m2.Tier)
	}
	if log := launchLog(t, b.App, fake); !strings.Contains(log, "--model 'claude-opus-5'") {
		t.Errorf("the re-typed line must name what the session runs:\n%s", log)
	}
}

// ranger-base-twaq and ranger-base-cplx, re-aimed at what survived ADR 0003
// §3 (ranger-base-hv2zr).
//
// Both beads were about a MARK: a session off its PID's pair wore a
// `fallback:` line, `posse relaunch` recreated from the substituted pair so
// the preflight fell nowhere and blanked it, and dispatch's effectiveTier —
// which answered only for a meta recording a mark — then handed the work
// prompt the bead's resolved tier for a session running something else.
// twaq carried the mark; cplx re-derived its sentence when a third-tier PID
// edit made it stale.
//
// The mark is gone. The lie it was invented to stop is not, because the two
// producers of an off-pair session are: the operator's own `--tier`, and a
// session created before this removal. So the same board, asserted on the
// two things left — the meta's `runtime:`/`tier:` survive the refresh, and
// dispatch reads THEM rather than a mark. The third-tier PID is cplx's arm:
// under a mark it made the sentence stale, and the pair-comparison has
// nothing to go stale.
func TestQA7vpAnOffPairSessionSurvivesPosseRelaunchAndDispatchReadsIt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		legacy bool
		pid    string // the PID's tier at the time of the refresh
	}{
		{"explicit --tier, PID unedited", false, TierStrong},
		{"a session created before the removal", true, TierStrong},
		{"cplx's third tier: neither what it runs nor what it fell from", false, TierFast},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := qaOffPairSession(t, "pr", tc.legacy)
			if tc.pid != TierStrong {
				qaPID(t, b, "architect", tc.pid)
			}

			var out strings.Builder
			if err := b.RelaunchSession(&out, RelaunchOpts{Name: "pr", NoLand: true}); err != nil {
				t.Fatalf("relaunch: %v\n%s", err, out.String())
			}

			m, _ := b.readMeta("pr")
			if m.Tier != TierStandard || m.Runtime != "claude" {
				t.Fatalf("the refresh moved the session off the pair it runs: %+v", m)
			}
			// The legacy row is the whole of the existing-session
			// transition: the old mark is not carried forward, and the
			// identity beside it is untouched.
			if raw := metaBytes(t, b, "pr"); strings.Contains(raw, "fallback:") {
				t.Errorf("the refresh re-created the removed mark:\n%s", raw)
			}
			if strings.Contains(out.String(), "FALLBACK:") {
				t.Errorf("the receipt carries a mark nothing writes:\n%s", out.String())
			}
			var list strings.Builder
			if err := b.CmdList(&list); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(list.String(), "⤵️") {
				t.Errorf("posse list wears a mark nothing writes:\n%s", list.String())
			}
			if !strings.Contains(list.String(), b.App.RuntimeTierTag("claude", TierStandard)) {
				t.Errorf("posse list must name the tier that is really running:\n%s", list.String())
			}
			// The fact both beads were really about: what dispatch tells the
			// persona it is thinking at.
			d := &Dispatcher{App: b.App, HB: b}
			if _, tier, differs := d.effectiveTier("pr", "claude", tc.pid); tier != TierStandard || !differs {
				t.Errorf("dispatch tells the persona it is thinking at %q (differs=%v); it is running claude-opus-5", tier, differs)
			}
		})
	}
	// The negative arm, and the reason the comparison is a comparison: an
	// operator who settles for what the session got has made it the
	// asked-for pair, and there is then nothing to report.
	t.Run("a PID that asks for what is running", func(t *testing.T) {
		t.Parallel()
		b, _ := qaOffPairSession(t, "cu", false)
		qaPID(t, b, "architect", TierStandard)
		d := &Dispatcher{App: b.App, HB: b}
		if rt, tier, differs := d.effectiveTier("cu", "claude", TierStandard); differs {
			t.Errorf("dispatch reports a difference against the pair the session runs: %s/%s", rt, tier)
		}
	})
	// "The meta does not say" is not "the meta says the empty runtime". A
	// session with no persona records neither half, and without the guard
	// the comparison reads that silence as a difference and hands the work
	// prompt an empty pair — worse than the stale tier the whole board is
	// about.
	t.Run("a session whose meta records no pair", func(t *testing.T) {
		t.Parallel()
		b, _ := newTestBackend(t)
		b.Warn = &strings.Builder{}
		if err := b.CreateSession(NewSessionOpts{Name: "np", Cmd: "true", Dir: t.TempDir()}); err != nil {
			t.Fatal(err)
		}
		if m, _ := b.readMeta("np"); m.Runtime != "" || m.Tier != "" {
			t.Fatalf("board not set up: a session with no persona must record no pair: %+v", m)
		}
		d := &Dispatcher{App: b.App, HB: b}
		rt, tier, differs := d.effectiveTier("np", "claude", TierStrong)
		if differs || rt != "claude" || tier != TierStrong {
			t.Errorf("a meta that says nothing must leave the resolved pair alone: %s/%s differs=%v", rt, tier, differs)
		}
	})
}

// The other half of off-pair, by RUNTIME rather than by tier. It used to be
// reached by a `tier_fallback:` naming a runtime; since ADR 0003 §3 it is
// reached the way §3 says it should be — an operator selecting one. The
// assertion is the same and it is here for the same reason it was before:
// collapsing the difference to the tier half alone would silently stop
// reporting every hopped session.
func TestQA7vpARuntimeDifferenceIsReportedTheSameWayATierOneIs(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	b.Warn = &strings.Builder{}
	qaPID(t, b, "security", TierStrong)
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone
	if err := b.CreateSession(NewSessionOpts{Name: "hr", Agent: "security", Runtime: "codex", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.readMeta("hr"); m.Runtime != "codex" || m.Tier != TierStrong {
		t.Fatalf("board not set up: the create must record the runtime it opened on: %+v", m)
	}

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "hr", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	m, _ := b.readMeta("hr")
	if m.Runtime != "codex" {
		t.Fatalf("the refresh moved the runtime: %+v", m)
	}
	d := &Dispatcher{App: b.App, HB: b}
	if rt, _, differs := d.effectiveTier("hr", "claude", TierStrong); rt != "codex" || !differs {
		t.Errorf("a session off its PID's RUNTIME reads back as %q (differs=%v)", rt, differs)
	}
}

// ─── what an unavailable model no longer costs ──────────────────────────────

// The behaviour ADR 0003 §3 changed most sharply, pinned as the deliberate
// reading it is. `tier_floor:` is the operator saying in advance "never run
// me below this". Under dial H the preflight handed the wall the SUBSTITUTED
// pair, so an unavailable strong model turned into a REFUSED launch for a
// PID floored at strong: the outage cost that PID its session.
//
// Nothing substitutes now, so the floor rules on strong, which is what the
// PID asked for and what the launch opens on. The outage costs the line and
// nothing else — and this is the "what breaks if wrong" of the removal read
// the other way round: unattended continuity on an unavailable model is
// gone, but so is a whole class of launch the floor used to refuse for a
// reason the operator never asked for.
func TestQA7vpTierFloorRulesOnTheAskedForPairAndTheLaunchGoesAhead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	qaPID(t, b, "floored", TierStrong, "tier_floor: strong\n")
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone

	if err := b.CreateSession(NewSessionOpts{Name: "tf", Agent: "floored", Dir: t.TempDir()}); err != nil {
		t.Fatalf("nothing fell below the floor, so nothing may refuse: %v", err)
	}
	if m, _ := b.readMeta("tf"); m.Tier != TierStrong {
		t.Errorf("the floored session opened at %q", m.Tier)
	}
	if log := launchLog(t, b.App, fake); !strings.Contains(log, "--model 'claude-fable-5-1'") {
		t.Errorf("the floored session did not open on the id its tier names:\n%s", log)
	}
	// It is not silent about it: the operator hears that the model the
	// floor insists on is one the account will not serve.
	if !strings.Contains(warn.String(), "unavailable on this account") {
		t.Errorf("the launch went ahead with no explanation: %q", warn.String())
	}
	// The wrong arm: the floor still bites something. A PID asking BELOW
	// its own floor is refused, so the launch above is availability no
	// longer moving the pair rather than the floor having stopped working.
	b2, _ := newTestBackend(t)
	b2.Warn = &strings.Builder{}
	qaPID(t, b2, "floored", TierStrong, "tier_floor: strong\n")
	seedCatalog(t, b2.App, time.Minute, "claude-fable-5-1", "claude-opus-5")
	err := b2.CreateSession(NewSessionOpts{Name: "tf2", Agent: "floored", Tier: TierStandard, Dir: t.TempDir()})
	if err == nil {
		t.Fatal("control: tier_floor: strong must still refuse an explicit standard")
	}
	if !strings.Contains(err.Error(), "tier_floor") {
		t.Errorf("control: the refusal must name the floor: %v", err)
	}
}

// ─── how old is "read" ───────────────────────────────────────────────────────

// This test measured, and pinned, the rule the operator overturned: a
// snapshot older than model_probe_ttl: whose re-read fails was answered as
// KNOWN and demoted every launch, for as long as the probe stayed down —
// which on this instance is most hours (ranger-base-wkai3), curable only by
// hand-editing state/model-catalog.json. The ruling on ranger-base-v1p66
// (2026-09-01, option 2) bounds it: past its lease the reading is quoted
// and obeyed by nothing. Rewritten rather than skipped, because a parked
// skip would leave the rejected alternative encoded in the suite.
func TestQA7vpAnExpiredSnapshotIsQuotedAndObeyedByNothing(t *testing.T) {
	t.Parallel()
	a := preflightApp(t) // unconfigured lister: every re-read fails
	seedCatalog(t, a, 30*24*time.Hour, "claude-opus-5", "claude-sonnet-5")
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if strings.Contains(pf.Line, "unavailable") || pf.Wanted != "claude-fable-5-1" {
		t.Errorf("a month-old snapshot may not reach a verdict: %+v", pf)
	}
	if !strings.Contains(pf.Line, "not in the catalog read 720h00m ago") || !strings.Contains(pf.Line, "availability UNKNOWN, launching as asked") {
		t.Errorf("it is still the newest fact anyone has, and the line must quote it: %q", pf.Line)
	}
	// The other half of the ruling: the same expired reading, when it DOES
	// list the wanted id, says nothing at all. UNKNOWN is not news per
	// launch; an id the newest reading does not name is.
	b := preflightApp(t)
	seedCatalog(t, b, 30*24*time.Hour, "claude-fable-5-1", "claude-opus-5")
	if pf := b.TierPreflight("architect", "claude", TierStrong, nil); pf.Line != "" {
		t.Errorf("an expired reading that lists the id has nothing to report: %+v", pf)
	}
	// The wrong arm: with NO snapshot at all the same failing re-read is
	// UNKNOWN and silent, which is what it has always been. If this stopped
	// holding, the assertions above would be measuring nothing.
	c := preflightApp(t)
	if pf := c.TierPreflight("architect", "claude", TierStrong, nil); pf.Line != "" || pf.Wanted != "claude-fable-5-1" {
		t.Errorf("no snapshot must be UNKNOWN, not unavailable, and silent: %+v", pf)
	}
}

// The launch itself, not the function that advises it: the shape of the
// 2026-09-01 incident — strong bumped to an id the retained reading was
// taken before, the probe 401ing since 08-31 — must now render the
// asked-for id, say so once, and leave no `fallback:` mark behind.
func TestQA7vpAStaleCatalogLaunchesTheAskedForIdAndMarksNothing(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	var hits atomic.Int64
	b.App.ModelLister = failingLister(&hits)
	qaPID(t, b, "architect", TierStrong)
	// Frozen, not wall-clock: the line below is pinned to "48h00m ago", a
	// whole-minute render (ranger-base-5hjyh).
	seedCatalogAged(t, b.App, 48*time.Hour, "claude-opus-5", "claude-sonnet-5") // fable gone, read two days ago

	if err := b.CreateSession(NewSessionOpts{Name: "st", Agent: "architect", Dir: t.TempDir()}); err != nil {
		t.Fatalf("the preflight must never refuse a launch (rule 3): %v", err)
	}
	// launchLog, not state/launch/st.sh. Which of the two places the
	// rendered line lands in is a fact about its LENGTH, not about the
	// launch: under PaneLineMax it is TYPED and no script is written at all
	// (paneline.go). Reading the script directly made this pin an assertion
	// about how long the line happens to be on this box — and on
	// ci.yml's ubuntu-latest it is shorter, because Linux has no
	// `sandbox-exec -f …` prefix and /tmp/… paths where darwin has
	// /var/folders/…/T/…, so the line fit, nothing spilled, and the pin
	// died on `no such file or directory` while the launch it was asking
	// about was correct (ranger-base-90y3c). The helper reads both places
	// and already carries this warning.
	sh := launchLog(t, b.App, fake)
	if !strings.Contains(sh, "--model 'claude-fable-5-1'") {
		t.Errorf("the launch must carry the id it was asked for:\n%s", sh)
	}
	m, ok := b.readMeta("st")
	if !ok || m.Tier != TierStrong {
		t.Errorf("nothing moved, so the meta records the pair asked for: %+v", m)
	}
	if raw := metaBytes(t, b, "st"); strings.Contains(raw, "fallback:") {
		t.Errorf("an UNKNOWN verdict left a mark behind:\n%s", raw)
	}
	said := warnBuf(t, b).String()
	if !strings.Contains(said, "architect: tier strong wants claude-fable-5-1 — not in the catalog read 48h00m ago") ||
		!strings.Contains(said, "availability UNKNOWN, launching as asked") {
		t.Errorf("launching on an unlisted id must not be silent:\n%s", said)
	}
}

// ─── the two operational keys ────────────────────────────────────────────────

// model_probe_ttl: was added by this bead and nothing pinned how it parses.
// The house duration form, 0 as "ask every launch", and the rule that a typo
// is NAMED and does not become a silent zero — a zero read out of a typo
// would put an HTTP request in front of every launch on this box.
func TestQA7vpModelProbeTTLForms(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		raw  string
		want time.Duration
		says bool
	}{
		{"", ModelProbeTTLDefault, false},
		{"1h", time.Hour, false},
		{"90", 90 * time.Second, false},
		{"0", 0, false},
		{"15m", 15 * time.Minute, false},
		{"-5m", ModelProbeTTLDefault, true},
		{"1 hour", ModelProbeTTLDefault, true},
		{"true", ModelProbeTTLDefault, true},
	} {
		a := preflightApp(t)
		if c.raw != "" {
			writeCfg(t, a, "model_probe_ttl: "+c.raw+"\n")
		}
		var errw strings.Builder
		got := a.ModelProbeTTL(&errw)
		if got != c.want {
			t.Errorf("model_probe_ttl: %q → %s, want %s", c.raw, got, c.want)
		}
		if said := strings.Contains(errw.String(), "is not a duration"); said != c.says {
			t.Errorf("model_probe_ttl: %q said=%v want %v (%q)", c.raw, said, c.says, errw.String())
		}
	}
}

// The off switch is one word and only one word: `false` turns the preflight
// off, and every other spelling — including the ones an operator reaches for
// — leaves it ON, which is the fail-safe direction but is not what they
// typed. Pinned so the reading is deliberate rather than incidental.
func TestQA7vpModelPreflightOffIsExactlyTheWordFalse(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		"false": false, " false ": false,
		"no": true, "0": true, "off": true, "False": true, "true": true, "": true,
	} {
		a := preflightApp(t)
		writeCfg(t, a, "model_preflight: "+raw+"\n")
		if got := a.ModelPreflight(); got != want {
			t.Errorf("model_preflight: %q → on=%v, want on=%v", raw, got, want)
		}
	}
}

// ─── the lease boundary itself (ADR 0039 D3c) ────────────────────────────────

// The one instant nothing pinned. D3c and ranger-base-ksmmz's own text put
// the boundary at "AT or past" the lease — age == lease is OUTSIDE, the
// reading no longer rules, and the launch is UNKNOWN. Every other pin in
// this package seeds an age far from the boundary (30 days against an hour,
// 48h against a default), so `age < lease` widened to `age <= lease` is
// green across all of ./internal/posse AND ./cmd/posse — MEASURED as a
// surviving mutant while verifying that close (ranger-base-9ztcy). It moves
// the rule by one instant in the FAIL-OPEN direction: a reading exactly at
// the end of its lease goes on demoting launches, which is the whole class
// the ruling on ranger-base-v1p66 exists to bound.
//
// Pinned at ModelsRead with a fixed clock rather than through a launch,
// because a boundary measured against time.Now() is a flake at any
// resolution, and withinLease answers BOTH of ModelsRead's questions off
// this one comparison — the maxAge row below is what keeps a `<=` from
// hiding in the freshness half.
func TestQAALeaseBoundaryIsExclusiveSoAReadingAtItsLeaseNoLongerRules(t *testing.T) {
	t.Parallel()
	const lease = time.Hour
	at := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		age  time.Duration
		want bool // may the retained reading still RULE?
	}{
		{"one instant inside the lease", lease - time.Nanosecond, true},
		{"exactly at the lease", lease, false},
		{"one instant past it", lease + time.Nanosecond, false},
		// The wrong arm this whole test needs: an age nowhere near the
		// boundary must still answer, or the three rows above would be
		// measuring a cache that had stopped reading the snapshot at all.
		{"a day past it (the control)", 24 * time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := preflightApp(t) // unconfigured lister: every re-read fails
			seedCatalogEntry(t, a, modelEntry{At: at, Models: []string{"claude-opus-5"}})
			c := a.ModelCache()
			c.Now = func() time.Time { return at.Add(tc.age) }
			ids, rules, read := c.ModelsRead(lease, lease)
			if rules != tc.want {
				t.Errorf("age %v against a lease of %v: rules=%v, want %v — D3c reads \"at or past\", so the boundary is exclusive",
					tc.age, lease, rules, tc.want)
			}
			// Past its lease the reading is quoted and obeyed by nothing:
			// the ids go back either way so the UNKNOWN line can name them
			// (D3a). A pin on the bool alone would let them be dropped.
			if len(ids) != 1 || ids[0] != "claude-opus-5" {
				t.Errorf("the retained ids go back whatever the bool says: %v", ids)
			}
			if got := read.Age; got != tc.age {
				t.Errorf("the age the line quotes is the age measured: %v, want %v", got, tc.age)
			}
		})
	}

	// The same comparison in its OTHER role. maxAge is "fresh enough not to
	// re-ask": at exactly maxAge the snapshot is stale, so ModelsRead must
	// go to the lister — and with an unconfigured one that is a failed
	// read, which is visible as a non-nil Err. Inside maxAge it returns on
	// the snapshot alone and asks nobody, so Err stays nil. Without this
	// row a `<=` could hide in the freshness half of withinLease.
	for _, tc := range []struct {
		name    string
		age     time.Duration
		wantErr bool // did it fall through and ASK?
	}{
		{"inside maxAge: answered off the snapshot, nobody asked", lease - time.Nanosecond, false},
		{"exactly at maxAge: stale, so it re-asks", lease, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := preflightApp(t)
			seedCatalogEntry(t, a, modelEntry{At: at, Models: []string{"claude-opus-5"}})
			c := a.ModelCache()
			c.Now = func() time.Time { return at.Add(tc.age) }
			_, _, read := c.ModelsRead(lease, lease)
			if got := read.Err != nil; got != tc.wantErr {
				t.Errorf("age %v against a maxAge of %v: re-asked=%v, want %v (err=%v)", tc.age, lease, got, tc.wantErr, read.Err)
			}
		})
	}
}

// ─── the pass output (done-when 2's "display") ───────────────────────────────

// The one surface effectiveTier feeds that the board above does not reach:
// what a dispatch PASS prints, and what tier the work prompt is built at.
// The board calls effectiveTier directly; this runs the pass.
//
// It matters because the `!` line and the tierWhy token are the operator's
// only notice that a bead is being worked at a tier it did not resolve. Both
// changed with the mark's removal (ranger-base-hv2zr): the line used to be
// the preflight's fallback sentence verbatim and the token used to be
// "fallback", neither of which describes anything now.
func TestQAAOffPairSessionIsNamedInThePassOutputAndPromptedAtWhatItRuns(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	qaPID(t, b, "ranger", TierStandard, "labels: [go]\n")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","status":"open"}]`)
	agentPerLaunch(t, fake)

	// The session this bead's pass will reach for, already open at a tier
	// the PID does not name — the operator's own explicit choice, which is
	// what ADR 0003 §3 leaves as the only way to be off-pair.
	name := SessionForBead("ranger", repo, "a-1")
	if err := b.CreateSession(NewSessionOpts{Name: name, Agent: "ranger", Tier: TierFast, Dir: repo}); err != nil {
		t.Fatalf("pre-create: %v", err)
	}
	if m, ok := b.readMeta(name); !ok || m.Tier != TierFast {
		t.Fatalf("board not set up: the session must be open at fast: %+v", m)
	}

	d := newTestDispatcher(t, b)
	if n, _ := d.Run("", "", 0); n != 1 {
		t.Fatalf("dispatch failed:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "opened on claude/fast, not the claude/standard this bead resolves") {
		t.Errorf("the pass does not name both pairs:\n%s", out)
	}
	// The token beside the existing "via PID" / "via tier_by_label" ones. A
	// pass that printed "standard via PID" here would be telling the
	// operator the bead is being thought about at a tier nothing is running.
	if !strings.Contains(out, "fast via the session") {
		t.Errorf("the pass reports the resolved tier, not the running one:\n%s", out)
	}
	// And the work prompt itself, which is the reason any of this is
	// printed: promptContext names the pair the persona is thinking at, and
	// the whole `!` line above exists so that sentence is not a lie.
	sent := calls(t, fake)
	if !strings.Contains(sent, "runtime/tier: claude/"+TierFast) {
		t.Errorf("the work prompt does not name the pair the session runs:\n%s", sent)
	}
	if strings.Contains(sent, "runtime/tier: claude/"+TierStandard) {
		t.Errorf("the work prompt names the tier the bead resolved, not the one running:\n%s", sent)
	}
}
