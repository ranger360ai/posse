//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-eoxo1: the L2 wall's mechanism is a host fact
// (ADR 0080 D16). `cage: seatbelt` names the TIER; `sandbox-exec` and
// `bwrap` are what realize it, on darwin and on Linux respectively.
//
// WHY A PIN, AND WHY IT DRIVES A HOST IT IS NOT RUNNING ON. The whole of
// this change is unreachable on the box that builds it: the Linux arm of
// `seatbeltTierAvailableOn` decides whether the seatbelt tier exists on the
// host ADR 0083 moves the crew to, and on darwin it is never entered. An
// untested arm that answers "yes, this host has an L2 write wall" is the one
// answer nobody may be handed on a guess — posse would print a wall it never
// put up, in front of every caged seat on that box. So the GOOS is a
// PARAMETER here and the two host measurements are variables, and the pin
// drives all four corners of the Linux decision from darwin.
//
// MEASURED BY HOST where the host is the subject: the mechanism-name arm
// says which GOOS it is reading and asserts per OS, and the live-probe arm
// (bwrapApplyProbe with no bwrap to resolve) skips where a bwrap is on the
// gates-free PATH, because there the answer is the kernel's and not the
// resolver's.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"runtime"
	"strings"
	"testing"
)

// wmStub swaps the two host measurements the Linux availability decision
// reads and restores them. No t.Parallel in anything that calls it: these
// are package-level func vars (cmd/testparallel's own rule).
func wmStub(t *testing.T, resolves bool, refusal string) {
	t.Helper()
	hadR, hadA := bwrapResolves, bwrapApplyRefusal
	bwrapResolves = func() bool { return resolves }
	bwrapApplyRefusal = func() string { return refusal }
	t.Cleanup(func() { bwrapResolves, bwrapApplyRefusal = hadR, hadA })
}

// The naming half: which mechanism realizes the tier, per OS. Pure, so the
// answer for the other host is readable from this one.
func TestQAWallMechanismNamesTheBinaryThatRealizesTheSeatbeltTier(t *testing.T) {
	// No t.Parallel: the last arm deletes AvailableCages[CageSeatbelt] to
	// show that the NAME does not read the table, and that map is a package
	// var every parity test in this package reads.
	for _, tc := range []struct{ goos, want string }{
		{"darwin", WallSandboxExec},
		{"linux", WallBwrap},
		// A GOOS with neither is the "" D16 names, and the only one: the
		// tier has no third mechanism, so a host that is not one of these
		// two has no L2 wall to label at all.
		{"windows", ""},
		{"freebsd", ""},
	} {
		if got := wallMechanismOn(tc.goos); got != tc.want {
			t.Errorf("wallMechanismOn(%q) = %q, want %q", tc.goos, got, tc.want)
		}
	}
	// And the live reading agrees with this box, so the parameterized
	// function above is the one production reads and not a parallel table.
	if got, want := WallMechanism(), wallMechanismOn(runtime.GOOS); got != want {
		t.Errorf("WallMechanism() = %q on %s, but wallMechanismOn says %q", got, runtime.GOOS, want)
	}
	// A NAMING QUESTION AND NOT AN AVAILABILITY ONE, which is the half a
	// later reader is most likely to collapse: the name does not consult
	// the table, so a host whose kernel refuses the namespace still reads
	// `bwrap` and says so over its ✗ rows.
	had := AvailableCages[CageSeatbelt]
	delete(AvailableCages, CageSeatbelt)
	defer func() {
		if had {
			AvailableCages[CageSeatbelt] = true
		}
	}()
	if got, want := WallMechanism(), wallMechanismOn(runtime.GOOS); got != want {
		t.Errorf("WallMechanism() reads the availability table: %q with the tier deleted, %q with it", got, want)
	}
}

// The availability half, every corner of it, on a host that gives one.
func TestQASeatbeltTierAvailabilityOnLinuxNeedsBwrapAndTheApplyProbe(t *testing.T) {
	// No t.Parallel: wmStub writes package-level func vars.
	for _, tc := range []struct {
		name     string
		goos     string
		resolves bool
		refusal  string
		want     bool
	}{
		{"both measured", "linux", true, "", true},
		// The two halves that each alone would be a wall posse announces
		// and does not have.
		{"no bwrap on PATH", "linux", false, "", false},
		{"kernel refuses the namespace", "linux", true, "bwrap: No permissions to creating new namespace", false},
		{"could not even ask", "linux", true, "could not ask: bwrap: executable file not found outside posse's gates dirs", false},
		{"neither", "linux", false, "bwrap: Operation not permitted", false},
		// The control that makes the GOOS gate a measurement: both
		// readings green and the answer is still no, because bwrap is not
		// what realizes the tier there.
		{"darwin, both green", "darwin", true, "", false},
		{"windows, both green", "windows", true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wmStub(t, tc.resolves, tc.refusal)
			if got := bwrapAvailableOn(tc.goos); got != tc.want {
				t.Errorf("bwrapAvailableOn(%q) with resolves=%v refusal=%q = %v, want %v", tc.goos, tc.resolves, tc.refusal, got, tc.want)
			}
		})
	}

	// And the tier's own decision routes through it: the Linux arm of
	// seatbeltTierAvailableOn is the availability half of what the table
	// fill reads (seatbeltTierProvidedOn is the whole of it — see the
	// render pin below, ranger-base-pl3ck), so a pin on bwrapAvailableOn
	// alone would leave the route untested.
	t.Run("the tier decision routes the linux arm through it", func(t *testing.T) {
		wmStub(t, true, "")
		if !seatbeltTierAvailableOn("linux") {
			t.Error("a Linux host with bwrap and a passing probe provides the seatbelt tier")
		}
		wmStub(t, true, "bwrap: No permissions to creating new namespace")
		if seatbeltTierAvailableOn("linux") {
			t.Error("a Linux host whose kernel refuses the namespace does NOT provide the seatbelt tier, however available the binary is")
		}
		// A GOOS with no mechanism takes neither arm, whatever the
		// measurements say.
		wmStub(t, true, "")
		if seatbeltTierAvailableOn("windows") {
			t.Error("a host with no wall mechanism cannot provide the tier")
		}
	})

	// Neither measurement is taken off Linux. The probe forks, and a host
	// that will never use it would pay for that fork at every process
	// start — including every one of posse's own gate shims.
	t.Run("neither measurement is taken off linux", func(t *testing.T) {
		asked := 0
		hadR, hadA := bwrapResolves, bwrapApplyRefusal
		bwrapResolves = func() bool { asked++; return true }
		bwrapApplyRefusal = func() string { asked++; return "" }
		t.Cleanup(func() { bwrapResolves, bwrapApplyRefusal = hadR, hadA })
		bwrapAvailableOn("darwin")
		if asked != 0 {
			t.Errorf("the darwin arm took %d host measurement(s); the GOOS test must short-circuit before either", asked)
		}
		if bwrapAvailableOn("linux"); asked == 0 {
			t.Error("the linux arm took no measurement at all — the stubs are not the ones the decision reads")
		}
	})
}

// The probe's own abstention, measured on this box: "could not ask" is a
// reason and never "". There it is the resolver's refusal rather than the
// kernel's, which is the only one of the probe's two failure modes a darwin
// box can produce — and it is the one that matters, because "" here would
// make AvailableCages claim a wall nothing was asked about.
func TestQABwrapApplyProbeNeverAnswersSilenceWhenItCouldNotAsk(t *testing.T) {
	t.Parallel()
	if _, err := protocolBin("bwrap"); err == nil {
		t.Skip("NOTHING MEASURED: a bwrap resolves on the gates-free PATH here, so this probe's answer is the kernel's and not the resolver's")
	}
	why := bwrapApplyProbe()
	if why == "" {
		t.Fatal(`bwrapApplyProbe answered "" with no bwrap to resolve — "" is read as "this host provides the L2 wall", so the one answer it may not give on a guess is the one it gave`)
	}
	if !strings.Contains(why, "could not ask") {
		t.Errorf("the abstention must say it is one, so a reader can tell it from the kernel's words: %q", why)
	}
	// The argv is D17's base mount and nothing lenient: a probe shaped
	// unlike the wall it predicts reports "walled" on a host that refuses
	// every render (ranger-base-xjw9's grid, the same lesson).
	if got, want := strings.Join(bwrapApplyProbeArgv, " "), "--ro-bind / / /bin/true"; got != want {
		t.Errorf("the probe's argv is %q, want %q — the base mount of every D17 render", got, want)
	}
}

// The row an operator reads the wall off. CageLabel is a LABEL: the seatbelt
// tier carries the mechanism, every other tier is its own name, and
// Parity.Cage stays the bare value it is compared against.
func TestQAParityCageRowNamesTheMechanismThatRealizesTheTier(t *testing.T) {
	t.Parallel()
	m := WallMechanism()
	if m == "" {
		t.Skip("NOTHING MEASURED: this GOOS has no L2 wall mechanism, so there is no suffix for a row to carry")
	}
	if got, want := CageLabel(CageSeatbelt), CageSeatbelt+" ("+m+")"; got != want {
		t.Errorf("CageLabel(%q) = %q, want %q", CageSeatbelt, got, want)
	}
	// Only the seatbelt tier. shims has no wall to name and the container
	// tier's mechanism is the home's engine, reported where that is read.
	for _, c := range []string{CageShims, CageContainer} {
		if got := CageLabel(c); got != c {
			t.Errorf("CageLabel(%q) = %q — only the seatbelt tier carries a mechanism", c, got)
		}
	}
	p := Parity{Runtime: "claude", Cage: CageSeatbelt, Tier: TierStrong, Realized: map[string]RealizedGate{}}
	row, _, _ := strings.Cut(p.String(), "\n")
	if want := "claude @ " + CageSeatbelt + " (" + m + ")/" + TierStrong + ":"; !strings.HasPrefix(row, want) {
		t.Errorf("the parity cage row must name the mechanism (ADR 0080 D16):\n got %q\nwant prefix %q", row, want)
	}
	// NOTHING ELSE RENDERS DIFFERENTLY, which is the bead's own done-when:
	// drop the suffix and the row is byte-identical to what it was before
	// the mechanism had a name.
	if got, want := strings.Replace(row, " ("+m+")", "", 1), "claude @ "+CageSeatbelt+"/"+TierStrong+": all gates realized"; got != want {
		t.Errorf("the suffix is the ONLY change to this row:\n got %q\nwant %q", got, want)
	}
	// And the field is untouched, because it is compared against the
	// constant and written into session records.
	if p.Cage != CageSeatbelt {
		t.Errorf("Parity.Cage must stay the bare tier, got %q", p.Cage)
	}
}

// ─── the third question: can posse TYPE it (ranger-base-pl3ck, D18) ─────────

// The window this pin was opened for, and the one it holds now. ranger-base-
// eoxo1 made the seatbelt tier AVAILABLE on a Linux host with bwrap and a
// passing probe, and nothing between that table and the launch line asked
// which mechanism realizes the tier: `wallRendered` read the table, and both
// of its call sites then typed `SeatbeltPrefix` — `sandbox-exec -f <prof>` —
// unconditionally. So on that host the launch would have reported its L2
// rules REALIZED (parity's `seatbelt :=` reads only cageAvailable) and typed
// a binary the host does not have, where before eoxo1 the same launch was
// REFUSED at "cage seatbelt is not available on this host/build". D16's own
// pin above says the thing to avoid is posse printing a wall it never put
// up; this is that, from the other side.
//
// ADR 0080 D18 (ranger-base-djl3c) CLOSED IT BY BUILDING THE RENDER, not by
// lifting the hold: `RenderWall` is the one switch between the two
// mechanisms, `WallPrefix` is what both call sites type, and
// wallMechanismRendered admits bwrap in the same commit. What this pin holds
// from here on is the BICONDITIONAL, in the only direction that can still
// break: every mechanism `wallMechanismOn` can name is one this build can
// type, and every name it admits is tied to the text posse actually types
// for it. A third mechanism added as a name alone reds here.
func TestQAWallTableClaimsNoTierThisBuildCannotType(t *testing.T) {
	// No t.Parallel: wmStub writes package-level func vars, and the last arm
	// reads AvailableCages.
	//
	// THE SET, and why it is a literal with a message rather than a census:
	// the two things that must change together are this predicate and the
	// prefix typed at wallRendered's call sites, and the second is a string
	// in a function body that no census can tie to a mechanism name. So the
	// list is spelled, and the arms below tie each TRUE member to the text
	// actually typed.
	for _, tc := range []struct {
		m    string
		want bool
	}{
		{WallSandboxExec, true},
		{WallBwrap, true},
		{"", false},
	} {
		if got := wallMechanismRendered(tc.m); got != tc.want {
			t.Errorf("wallMechanismRendered(%q) = %v, want %v — this predicate's membership IS RenderWall's switch (wall.go). A name admitted with no arm there is a tier posse announces and cannot put up; an arm with the name withheld is a wall posse renders and then refuses to launch behind.",
				tc.m, got, tc.want)
		}
	}
	// NO MECHANISM IS NAMED THAT THIS BUILD CANNOT TYPE. The derived half of
	// the set: whatever GOOS wallMechanismOn answers for, the answer is a
	// mechanism with a render. This is the arm a third mechanism added as a
	// name alone reds, which is the whole residual after D18 — the hazard
	// has moved from "bwrap is available and untyped" to "the next one is
	// named and untyped", and it is the same hazard.
	for _, goos := range []string{"darwin", "linux", "windows", "freebsd", "plan9"} {
		m := wallMechanismOn(goos)
		if m != "" && !wallMechanismRendered(m) {
			t.Errorf("wallMechanismOn(%q) names %q and wallMechanismRendered refuses it — a GOOS whose mechanism has a NAME but no render in RenderWall must not be named at all until it has one, or AvailableCages claims a tier over a wall nothing puts up (ranger-base-pl3ck)", goos, m)
		}
	}
	// Each true member tied to the text posse types for it, so the predicate
	// cannot drift into a claim about a prefix that says something else.
	if pre := SeatbeltPrefix("/x/seatbelt.sb"); !strings.HasPrefix(pre, WallSandboxExec+" ") {
		t.Errorf("wallMechanismRendered admits %q, but the prefix typed onto the launch line is %q — the predicate and the render must name the same binary", WallSandboxExec, pre)
	}
	// bwrap's prefix names the binary as the ABSOLUTE path it was resolved
	// to, not as the bare name, so a gates-dir or mise shim cannot stand in
	// for the seat's own wall (ADR 0080 D18).
	if pre := BwrapPrefix("/usr/bin/"+WallBwrap, "/x/bwrap.args"); !strings.Contains(pre, "'/usr/bin/"+WallBwrap+"' --args 3 -- ") {
		t.Errorf("wallMechanismRendered admits %q, but the prefix typed onto the launch line is %q — the predicate and the render must name the same binary", WallBwrap, pre)
	}
	if strings.Contains(BwrapPrefix(WallBwrap, "/x/bwrap.args"), " "+WallBwrap+" ") {
		t.Error("BwrapPrefix quotes the binary it is handed; a bare name on the line is one a gates-dir shim answers")
	}

	// THE COMPOSITION. Both host measurements green on Linux: available, and
	// now PROVIDED, because D18 renders it. The two answers used to differ
	// here and this was the only place they did; what the arm holds now is
	// that they agree on every host, mechanism for mechanism.
	wmStub(t, true, "")
	if !seatbeltTierAvailableOn("linux") {
		t.Fatal("the availability half must answer yes with both measurements green — this pin is about the render, and a stub that broke the measurement would pass for the wrong reason")
	}
	if !seatbeltTierProvidedOn("linux") {
		t.Error("a Linux host with bwrap and a passing probe is AVAILABLE and, since ADR 0080 D18 wrote the bwrap render, PROVIDED: withholding the tier here refuses every caged launch on the host ADR 0083 moves the crew to, over a wall posse can put up (ranger-base-djl3c)")
	}
	// And the render conjunct still CONJOINS: a host whose kernel refuses
	// the namespace is not provided, so the day this reads true the render
	// has started answering for the measurement.
	wmStub(t, true, "bwrap: No permissions to creating new namespace")
	if seatbeltTierProvidedOn("linux") {
		t.Error("a Linux host whose kernel refuses the namespace provides no tier, however good the render is — availability is the other half and must not be swallowed by it")
	}
	wmStub(t, true, "")
	// Darwin is untouched by the conjunct: its mechanism is the one that
	// renders, so the provided answer IS the available one, measurement for
	// measurement.
	if got, want := seatbeltTierProvidedOn("darwin"), seatbeltTierAvailableOn("darwin"); got != want {
		t.Errorf("seatbeltTierProvidedOn(darwin) = %v but availability says %v — the render conjunct must change nothing on the host that has one", got, want)
	}
	// A GOOS with no mechanism at all: neither question, same answer.
	if seatbeltTierProvidedOn("windows") {
		t.Error("a GOOS with no wall mechanism provides no tier")
	}

	// AND THE LIVE TABLE OBEYS IT. This is the invariant the whole file is
	// for, read off this box: the tier is never in the table unless this
	// build can type the mechanism that would realize it here.
	if AvailableCages[CageSeatbelt] && !wallMechanismRendered(WallMechanism()) {
		t.Errorf("AvailableCages claims the %s tier on %s, whose mechanism is %q and which this build cannot render — posse would print a wall it never put up", CageSeatbelt, runtime.GOOS, WallMechanism())
	}
}

// ─── D21's gate answered refused, so the flip is a tripwire (bjk4k) ─────────

// ADR 0080 D21 would have had `wallRendered` drop `!rt.SelfSandbox` when the
// mechanism is bwrap, so codex's own sandbox ran inside ours. It was GATED on
// probe T6, and T6 ran on the D1 host and read **refused against an expected
// allowed** (ranger-base-r5m98's comment carries the table; ranger-base-bjk4k
// read it). So the flip is not built, Linux codex keeps darwin's posture, and
// this pin is how that answer stays an answer.
//
// WHY A SOURCE READ AND NOT ONLY A BEHAVIORAL ARM, which is the whole reason
// this pin exists beside the ones that were already here. `wallRendered`
// takes no mechanism: it reads `WallMechanism()` nowhere, so the behavioral
// pins on its answer (codexrex_test.go's arm 1,
// credentialdenymove_qa_test.go's table) are read on THIS box, where the
// mechanism is `sandbox-exec`. A D21 flip is spelled as a bwrap disjunct —
// `!rt.SelfSandbox || WallMechanism() == WallBwrap` — and on darwin that
// disjunct is false, so every one of those pins stays GREEN over a landed
// flip and the darwin suite would never see it. That is the exact shape this
// ADR keeps parameterizing its host decisions to avoid (`wallMechanismOn`,
// `renderWallOn`, `seatbeltTierAvailableOn`, "so a pin can read the Linux
// branch from darwin"), and the one predicate at the centre of D21 is the one
// that cannot be read that way. So arm 2 reads the function's BODY.
//
// AND D21 IS RETIRED, so this arm waits for nothing. The operator ruled the
// D1 host claude-only (ranger-base-0ka8a (a), ADR 0080 D24), so walling
// codex under bwrap protects nothing and the flip is never built (D25).
// THIS PIN IS NOT RETIRED WITH IT, and the two are independent on purpose
// (D26): what it refuses is a host term entering `wallRendered` at all, and
// that hazard is the predicate's shape rather than D21's status — posse is
// shipped software, so "codex on a Linux host" is not only the D1 host.
// Reopening D21 is the operator's call, not an inference from a new runtime
// version or a probe reading, and a reopen starts with the amended T6/T6z
// run (the STEPS on ranger-base-1jhgv, closed moot by the same ruling).
func TestQANoPosseWallForASelfSandboxRuntimeOnEitherMechanism(t *testing.T) {
	// No t.Parallel: reads the package var AvailableCages, which other tests
	// write (cmd/testparallel's rule).
	cx := builtinRuntime(t, "codex")
	if !cx.SelfSandbox {
		t.Fatal("the codex built-in stopped declaring SelfSandbox — this pin's subject is the `!rt.SelfSandbox` term in wallRendered, and with the declaration gone it has no subject and would pass for the wrong reason (ADR 0080 D21)")
	}

	// ARM 1: THE ANSWER, on this box. False, and false because of the
	// SelfSandbox term rather than because the tier is unavailable — the
	// control below is the arm that fails if the predicate has stopped
	// saying yes to anything at all, which is the way an assertion of
	// absence goes vacuous.
	if wallRendered(CageSeatbelt, cx) {
		t.Errorf("posse rendered an L2 wall for %s, a SelfSandbox runtime — on darwin nested sandbox-exec is refused, and on Linux the gate that would have permitted this (ADR 0080 D21, probe T6) read refused (ranger-base-bjk4k) and D21 is now RETIRED by operator ruling, the D1 host being claude-only (ranger-base-0ka8a (a), ADR 0080 D24-D25). parity.go says this as a DeclaredDifference instead; a wall here is one that cannot apply", cx.Name)
	}
	if !AvailableCages[CageSeatbelt] {
		t.Skip("no seatbelt tier on this box — arm 1's control cannot distinguish the SelfSandbox term from the availability term, and arm 2 is the half that matters")
	}
	cl := builtinRuntime(t, "claude")
	if cl.SelfSandbox {
		t.Fatal("the claude built-in started declaring SelfSandbox — arm 1's control needs a runtime posse DOES wall, or the arm above is an absence measured over a predicate that says no to everything")
	}
	if !wallRendered(CageSeatbelt, cl) {
		t.Fatalf("wallRendered says no to %s at %s on a box where the tier IS available — the predicate has stopped saying yes to anything, so arm 1's false is not evidence about SelfSandbox", cl.Name, CageSeatbelt)
	}

	// ARM 2: THE SOURCE. wallRendered names no mechanism, which is what
	// makes arm 1 mechanism-independent — and what makes a mechanism term
	// appearing here a D21 flip that arm 1 cannot see from darwin.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "seatbelt.go", nil, 0)
	if err != nil {
		t.Fatalf("parse seatbelt.go: %v", err)
	}
	const owner = "wallRendered"
	var body *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == owner && fd.Body != nil {
			body = fd
		}
	}
	if body == nil {
		t.Fatalf("no func %s in seatbelt.go — the predicate this pin is about has moved or been renamed, and a pin that cannot find its subject must say so rather than pass (the last rename was seatbeltWallRendered → %s, ADR 0080 D16)", owner, owner)
	}
	// AN ALLOW-LIST AND NOT A DENY-LIST, because a deny-list pins spellings
	// and the claim is about terms. This arm first held the five mechanism
	// names a flip was expected to reach for (WallMechanism, wallMechanismOn,
	// WallBwrap, WallSandboxExec, wallMechanismRendered) under a comment
	// calling that "every spelling a flip could reach for" — and it was not:
	// ADR 0080's own house style parameterizes a host decision on the GOOS
	// rather than on the mechanism name (wallMechanismOn(goos),
	// seatbeltTierAvailableOn(goos), homeCacheGrants(goos)), each so a pin can
	// read the Linux branch from darwin. A flip in that style names no
	// mechanism at all:
	//
	//	cage == CageSeatbelt && AvailableCages[CageSeatbelt] && rt != nil &&
	//		(!rt.SelfSandbox || runtime.GOOS == "linux")
	//
	// left the deny-list GREEN, and so were the five behavioral pins this
	// file's close had already identified as blind to a flip — the exact
	// outcome this arm exists to make impossible (MEASURED 2026-11-09 on
	// darwin 25.6.0, ranger-base-ojmh6/ranger-base-e7ink).
	//
	// So the assertion is the other way round: the identifiers in this body
	// are a SUBSET of the five the predicate is allowed to read, plus `nil`.
	// That is immune to spelling — it reds on any new term whatever it is
	// called — and it is the right shape for the subject, because the claim
	// is that this predicate reads the tier and the runtime's declaration and
	// NOTHING ELSE. Not a census over the whole package: ONE function body.
	okNames := map[string]bool{
		"cage":           true,
		"CageSeatbelt":   true,
		"AvailableCages": true,
		"rt":             true,
		"SelfSandbox":    true,
		"nil":            true,
	}
	ast.Inspect(body.Body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || okNames[id.Name] {
			return true
		}
		t.Errorf("%s reads %q at %s — this predicate is allowed the tier (cage, CageSeatbelt, AvailableCages) and the runtime's own declaration (rt, SelfSandbox) and nothing else, so a new term here is a host decision moving into it. If it names the mechanism, directly or through the GOOS — that is ADR 0080 D21's flip, and it is INVISIBLE to this suite's behavioral pins: on darwin a `WallMechanism() == WallBwrap` disjunct is false, so codexrex_test.go's arm 1, credentialdenymove_qa_test.go's wallRendered table and arm 1 above all stay green while Linux codex gets a wall posse never measured. D21 was gated on probe T6, T6 read REFUSED against an expected allowed (ranger-base-r5m98, ranger-base-bjk4k), and D21 is now RETIRED: the operator ruled the D1 host claude-only (ranger-base-0ka8a (a), ADR 0080 D24-D26), so no codex seat is there for the flip to protect and it is never built. This pin is not waiting to be deleted by it. Reopening D21 is the OPERATOR's call — not an inference from a new runtime version or a probe reading — and a reopen starts with the amended T6/T6z run (ranger-base-1jhgv), never with this term",
			owner, id.Name, fset.Position(id.Pos()))
		return true
	})
}
