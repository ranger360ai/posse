package posse

// `posse upgrade`'s D4 half (ranger-base-v87a1) — the status/gates snapshot
// taken before the swap, the diff taken after it through the NEW binary, and
// the automatic rollback on a new red.
//
// WHY THE SNAPSHOT IS TAKEN BY RUNNING THE TARGET AND NOT BY CALLING
// ShopCheck. The question D4 asks is "does the binary that is now installed
// say something worse than the one that was", and only one half of that can
// be answered in this process: the half that is about to be replaced. So
// both halves are taken the same way — exec the TARGET PATH, before and
// after the rename — and the comparison is between two runs of one command
// that differ in exactly the thing under test. A snapshot taken in-process
// and a reading taken by subprocess would differ in cwd, in environment and
// in which code paths ran, and every one of those differences would arrive
// as a condition that "appeared".
//
// WHY THE TRIGGER IS COUNTED BY (class, code) AND NOT KEYED BY SUBJECT —
// the one place this file reads D4 differently from the way ADR 0092 wrote
// it, and the reason is that the two sides of this diff are two DIFFERENT
// BINARIES. ADR 0092 D4 says "The trigger is a diff of the condition lines
// keyed by code and subject". The subject lives in GovCondition.Key, which
// `posse status` does not print, so the only subject a reader of that output
// has is the DETAIL — and the detail is documented as the half that may move
// freely (govern.go, GovCondition: "Detail is for a human and may move
// freely (percentages, ages) — nothing keys on it"). Diffing it would roll
// back a good install because a watch log aged four minutes between the two
// readings, or because a landing reworded a sentence. The same objection
// defeats keying on GovCondition.Key itself even if it were printed: G11's
// key carries a lag BUCKET and G7's a log age class, so a key is not a
// stable interface between two binaries either.
//
// What IS stable across two binaries of this tree is the pair the design
// itself names: the class (URGENT | LANE, ADR 0029) and the row (G1…G17, or
// `—` for a carry-over). So this file counts conditions per (class, code)
// and trips when a count GOES UP for a code the binary renders. That is
// strictly inside D4's stated intent — "It must not merely be a line that
// moved since the snapshot" — and it is quieter than a subject-keyed diff
// rather than louder.
//
// WHAT IT THEREFORE CANNOT SEE, said out loud because it is the price: a
// SUBSTITUTION inside one code — the new binary clearing one G12 hook-wall
// row and raising a different one in the same breath — leaves the count
// unchanged and does not trip. A new binary that makes a wall worse makes
// MORE rows, which is the shape that trips; one that makes a different wall
// worse while fixing the first is exotic, and the epilogue's
// verify-hook-freshness reading still prints under it. (That used to say
// verify-gate-freshness, which was the wrong half of the epilogue for a
// hook-wall row and is, since ADR 0092 D5, not run there in its reporting
// form at all: the gate half is an install arm now and asks the script only
// its three behaviour questions — upgradegate.go. verify-hook-freshness is
// the reading that was always the one about walls.)
//
// WHAT COUNTS AS "THE BINARY RENDERS IT" is the table below, which is this
// bead's first deliverable: ADR 0092 D4 named G12, G17 and the binary header
// and called the set ASSUMED. It is MEASURED here, and the measurement found
// a third code the ADR did not name (G7). D4's amendment then moved G12 back
// OUT of it — a repo's git hook is a copy, and D7 leaves the re-render to the
// operator, so a G12 row after the swap is the install's own footprint rather
// than evidence against the binary (ranger-base-qfhph; the table's G12 block
// has the whole argument). What trips the rollback is therefore G7, G17 and
// any row this binary cannot name at all. The partition is pinned by
// TestQAEveryStatusConditionCodeHasAnUpgradeClass, whose door is `make
// pkgcensus-check`.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// upgradeCondClass is where a `posse status` condition's answer comes from,
// and therefore whether the condition appearing between two readings is
// evidence about the NEW BINARY or evidence about the shop.
type upgradeCondClass int

const (
	// upgradeShopState — the row's store of record MOVES ON ITS OWN while
	// the verb's `make test` runs. A seat blocks, a bead ages past a
	// threshold, spend accrues, a human writes a pause file. Twelve minutes
	// of suite is long enough for any of them, so a row that appeared says
	// nothing about the binary and is excluded (ADR 0092 D4: "Conditions
	// that move on their own (G1 blocked seats, G3 question ages) are
	// excluded by code").
	upgradeShopState upgradeCondClass = iota

	// upgradeBinaryRendered — the row's inputs are STATIC between the two
	// readings (a config file, a hooks directory, a promoted map) and the
	// verdict over them is this binary's own code. Nothing but the swap can
	// have changed the answer, so a row that appeared is evidence against
	// the binary that just landed, and trips the rollback.
	upgradeBinaryRendered

	// upgradeInstallSubject — the row is ABOUT THE INSTALL ITSELF: which
	// binary is in place, or what the install still has to be followed by.
	// Either way the upgrade moves the row by construction, so a row that
	// appeared is this verb's own footprint and not evidence against the
	// binary it just put there. Excluded for a different reason from
	// upgradeShopState's — whose store moves on its own while nobody is
	// installing anything — which is why it is not that constant.
	//
	// Two members, and the second is why this class is "the install's
	// follow-through" and not "the build stamp" (ADR 0092 D4 as amended by
	// ranger-base-qfhph):
	//
	//	G11  WHICH BINARY IS INSTALLED. It counts commits from the
	//	     installed stamp to the tip of the source repo. The install
	//	     moves the stamp forward, and the tip may have moved further
	//	     forward than that while the suite ran — so G11 can
	//	     legitimately appear or DEEPEN across a good install.
	//	G12  THE HALF OF THE INSTALL THIS VERB DOES NOT DO. Each declared
	//	     repo's git hook is a COPY of this binary's render, and D7 keeps
	//	     `posse gates install-hooks` out of this verb's scope on purpose.
	//	     So a G12 row after the swap is the DESIGNED state after every
	//	     install that changes the render: it says the operator's half has
	//	     not happened yet, not that the new binary is broken. The report
	//	     prints the re-render for them (upgradeHookWallLine) and this
	//	     verb never runs it.
	upgradeInstallSubject
)

// upgradeConditionSource is the partition — one entry per row `posse status`
// can print, keyed the way GovReport prints it (GovCondition.Row(), so a
// pulse-era carry-over is the em dash it renders as and not the empty ID it
// carries).
//
// THE MEASUREMENT, 2026-11-13, at 70dc7b2c, against govern.go's own
// store-of-record table and the four rows that live outside it. The question
// asked of every row was ADR 0092 D4's own: does this row's store move while
// `make test` runs?
//
//	G1   herdr agent status          shop     seats block and unblock
//	G2   bd + herdr                  shop     beads settle, sessions end
//	G3   bd + the clock              shop     an age crosses a threshold
//	G4   the guard + the clock       shop     a skip streak lengthens
//	G5   the plan endpoint           shop     a blind window lengthens
//	G6   the cost scan vs caps       shop     spend accrues
//	G7   config grammar; flock; log  BINARY   see below — the ADR's gap
//	G8   state/pause.yaml            shop     a human writes it
//	G9   bd + config                 shop     a bead becomes ready
//	G10  verify-box.yaml             shop     a periodic control writes it
//	G11  the build stamp             INSTALL  this verb moves it
//	G12  the hooks dirs              INSTALL  a COPY of this binary's render,
//	                                          re-rendered by the operator
//	                                          after the install (D7)
//	G13  config + git                shop     git state moves
//	G14  the personas dir            shop     memory lands
//	G15  the session meta            shop     sessions open and close
//	G16  the bd census               shop     the store grows
//	G17  config + the promoted map   BINARY   a static map, judged by this
//	                                          binary's own cage resolver
//	—    unpushed / no-live / backup shop     git, herdr and the clock
//
// G7 IS THE ONE THE ADR DID NOT NAME, and it is the reason this bead was
// asked to measure rather than inherit. Its `loop-dead` and log-mute keys
// are shop-state — a flock and an mtime — but its `arm-broken` keys are not:
// they read `autostart_interval:` and `autostart_via:` out of a config file
// that nothing touches during an upgrade, and refuse them through THIS
// BINARY's ParseInterval and AutostartVia. A new binary whose interval
// grammar narrowed would disarm the fleet's only instrument — URGENT, and
// silent by construction (govern.go's own words at the log-mute key) — and
// under the ADR's assumed set that install would have landed clean.
//
// A code is as binary-rendered as its most binary predicate, because the two
// errors are not the same size. A false rollback costs one re-run of the
// verb and leaves the fleet on the binary it was already running, with the
// reason printed in one line. A missed rollback leaves a broken binary
// installed on a box that is dispatching a live fleet. So where a code's
// predicates do not share a source, it is classified by the one that would
// be missed. For G7 the price is named: a watch loop that dies, or goes
// mute, inside the suite window rolls the install back and says it did.
//
// G12 IS AN INSTALL SUBJECT, WHICH IS THE ARCHITECT'S RESOLUTION OF A
// CONFLICT THIS TABLE USED TO CARRY (ranger-base-qfhph, ADR 0092 D4
// amendment 1). The first cut classified it as binary-rendered because D4
// named it in as many words, and filed the conflict; this is the answer that
// came back, and it is said here because this table is where the next reader
// meets G12.
//
// THE CONFLICT. G12's probe is BYTE-FOR-BYTE identity between each declared
// repo's installed git hook and this binary's own render of it
// (hookfresh.go, l3AskIdentity). A git hook in a repo is a COPY, and D7
// keeps `posse gates install-hooks` out of this verb's scope on purpose
// ("repo hooks are an L3 wall install, and G12 lists them for the
// operator"). So a commit that changes the hook render makes every declared
// repo's wall read "ours but stale" THE MOMENT the new binary is in place,
// and the designed sequence is: install, G12 goes stale, the operator
// re-renders. Classed as binary-rendered, D4 read step two as a new red and
// undid step one — the verb refused exactly the landings that change the
// wall.
//
// WHY IDENTITY CANNOT BE THE EVIDENCE D4 WANTS, which is what makes this a
// reclassification rather than a tolerance: l3AskIdentity DROPS ADR 0023's
// behaviour half on purpose, and its own header says what the drop gives up
// — "the behavior half catches a RENDERER regression ... it is already asked
// at every launch and once per watch loop". What is left answers "these
// bytes are not this binary's render", and cannot tell a binary that renders
// a BROKEN hook from one that renders a DIFFERENT one. A row that cannot
// distinguish those two is not evidence about a binary at all.
//
// WHAT ASKS THE QUESTION INSTEAD. The behaviour half is a property of the
// binary alone and the same in every repo, so D4's amendment asks it
// directly, through `posse gates render-probe` (amendment 3, built under
// ranger-base-apf87). Identity stays what it always was: a per-repo fact
// about the follow-through, which the operator owns and which G12 tells
// them. The verb prints the re-render line for them and never runs it.
//
// THE RESIDUAL, accepted and named: the behaviour probe execs a render from
// a temp file, so it cannot see a render that refuses correctly from a temp
// file and is installed wrong by `install-hooks` (a bad mode, a bad chain).
// That shows up at the operator's re-render, or at the next launch's
// identity∧behaviour probe in that repo — never silently.
//
// AN UNKNOWN CODE IS NOT SHOP-STATE. upgradeCondSource answers
// upgradeBinaryRendered for a row this table does not name, and that is the
// correct answer rather than a fallback: this table is complete for THIS
// binary (the pin says so), so a row the NEW binary printed and this one
// cannot name is a condition the new binary invented. The pin exists so that
// the next bead to add a G-row classifies it instead of discovering this.
var upgradeConditionSource = map[string]upgradeCondClass{
	"G1":  upgradeShopState,
	"G2":  upgradeShopState,
	"G3":  upgradeShopState,
	"G4":  upgradeShopState,
	"G5":  upgradeShopState,
	"G6":  upgradeShopState,
	"G7":  upgradeBinaryRendered,
	"G8":  upgradeShopState,
	"G9":  upgradeShopState,
	"G10": upgradeShopState,
	"G11": upgradeInstallSubject,
	"G12": upgradeInstallSubject,
	"G13": upgradeShopState,
	"G14": upgradeShopState,
	"G15": upgradeShopState,
	"G16": upgradeShopState,
	"G17": upgradeBinaryRendered,
	// The three pulse-era carry-overs, which GovReport prints as one row
	// name because none of them has a G-number: unpushed commits on a beads
	// repo, no live session for the pulse persona, and a stale or absent
	// on-box backup. git, herdr and the clock — all three move on their own.
	GovCondition{}.Row(): upgradeShopState,
}

// upgradeCondSource is the table with its unknown-code rule, which is the
// half that matters: see the table's own header.
func upgradeCondSource(row string) upgradeCondClass {
	if c, ok := upgradeConditionSource[row]; ok {
		return c
	}
	return upgradeBinaryRendered
}

// upgradeStatusRow matches the row name GovReport prints in its second
// column — a G-number, or the em dash a carry-over renders as. It is what
// keeps a condition's DETAIL from being read as another condition: the
// detail is one line of free prose and could open with any word at all, and
// a line whose second field is not a row name is not a condition line.
var upgradeStatusRow = regexp.MustCompile(`^(G[0-9]+|` + regexp.QuoteMeta(GovCondition{}.Row()) + `)$`)

// upgradeParseStatus counts `posse status`'s condition rows by (class, row).
//
// It reads GovReport's own format — `%-7s %-3s %s` — and nothing else in
// that command's output: the header lines, the readings (launcher lag, shop
// pulse, backup, live box) and the `?` lines for stores that could not be
// read all fail the two field tests and are skipped. The `?` lines are
// deliberately not counted and deliberately not a trigger: ADR 0092 D4 lists
// three triggers and an unreadable store is none of them, and most of what
// produces one — herdr unreachable, bd mid-write — is shop-state that would
// roll back good installs. They are printed in the verify report instead, so
// the delegate sees them.
func upgradeParseStatus(out string) map[string]int {
	counts := map[string]int{}
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		if len(f) < 3 {
			continue
		}
		if f[0] != GovUrgent && f[0] != GovLane {
			continue
		}
		if !upgradeStatusRow.MatchString(f[1]) {
			continue
		}
		counts[f[0]+" "+f[1]]++
	}
	return counts
}

// upgradeGatesRed counts the rows of a `posse gates <p>` report that say a
// gate is NOT held — ADR 0092 D4's "a DEGRADED or REFUSED row it did not
// have", counted rather than diffed for the reason this file's header gives.
//
// The markers are ADR 0017 §2's presentation constants and not this file's
// invention: the parity block's `✗` rows (one per gate the wall does not
// hold), the `REFUSED here` lines the Rex admission block prints above it,
// and a `DEGRADED` line that is not a parity block's own header — the
// multiplexer line is one. `RATIFIED DEGRADATION` is deliberately none of
// them and matches no marker: it is a real difference the promoted
// constitution already waived, so it is standing state on every reading of a
// mapped host rather than news.
//
// THE PARITY BLOCK'S ` DEGRADED` HEADER IS NOT COUNTED (ADR 0092 D4
// amendment 2, ranger-base-qfhph). Parity.String writes that word exactly
// when len(p.Degraded) > 0 and then prints one `✗` row per entry, so the
// header is a SUMMARY of the rows beneath it and counting it was counting
// the same wall twice. It is excluded by its own shape — `<runtime> @
// <cage>/<tier>: DEGRADED` — and not by dropping the word, because the same
// report carries a multiplexer line that opens `DEGRADED — ` for a real
// refusal of ADR 0076's, and that one is still a row.
//
// L3 HOOK ROWS ARE LEFT OUT (the same amendment, and the reason the header
// mattered at all). `posse gates <p>` probes the L3 hooks of its cwd
// (parity.go applyL3Probe) and asks ADR 0023's IDENTITY question of them —
// byte-for-byte against this binary's own render. A commit that changes the
// render therefore makes those rows appear the moment the new binary is in
// place, in any repo posse guards, which is G12's situation one surface
// over: the designed state after an install whose follow-through (D7's
// re-render) has not happened yet. So they are excluded here for the reason
// G12 is excluded in the table above, and a report whose only not-held rows
// are L3 rows counts ZERO — the header included, or the exclusion would buy
// nothing.
//
// What is NOT excluded: a forward gap (`L3 managed hooks — …`), which is
// about an employer's hook this session would skip and names no slot of
// posse's, and every non-L3 `✗` row, which is what this count exists for.
func upgradeGatesRed(out string) int {
	n := 0
	for _, ln := range strings.Split(out, "\n") {
		if !upgradeNotHeldRow(ln) || upgradeNamesL3Slot(ln) {
			continue
		}
		n++
	}
	return n
}

// upgradeParityHeader is the one line Parity.String writes above its rows —
// `%s @ %s/%s:` plus the verdict word — and it is matched here by that shape
// so that the SUMMARY of a block is not counted beside the rows it
// summarizes. A line that merely opens with the word (the multiplexer's
// `DEGRADED — …`) is a row of its own and does not match.
var upgradeParityHeader = regexp.MustCompile(`^\S+ @ .+/[^/]*: DEGRADED$`)

// upgradeNotHeldRow is the marker test: does this line of a gates report say
// a gate is not held?
func upgradeNotHeldRow(ln string) bool {
	if strings.Contains(ln, "REFUSED") || strings.ContainsRune(ln, '✗') {
		return true
	}
	return strings.Contains(ln, "DEGRADED") && !upgradeParityHeader.MatchString(strings.TrimSpace(ln))
}

// upgradeL3Slots are the two hook slots posse renders and parity.go probes
// by identity — ADR 0092 D4 amendment 2 names both.
var upgradeL3Slots = []string{"pre-push", "prepare-commit-msg"}

// upgradeNamesL3Slot answers whether a report row is about one of those
// slots. It matches the opening `l3DegradeLine` and `l3DegradeLineIn` both
// write and `applyHookResult` echoes — `L3 <slot> hook` — rather than the
// bare slot name, so a gate row that merely mentions a push is not read as a
// hook row.
func upgradeNamesL3Slot(ln string) bool {
	for _, slot := range upgradeL3Slots {
		if strings.Contains(ln, "L3 "+slot+" hook") {
			return true
		}
	}
	return false
}

// upgradeStampOf is the commit `posse version` names, out of a line shaped
// `posse <version>+<stamp> (<backend>)`. "" when the line names no stamp at
// all, which is every build that did not come through clean-build.sh — a
// release tag, a `go run`, a `go build` with no ldflags.
func upgradeStampOf(version string) string {
	f := strings.Fields(strings.TrimSpace(version))
	if len(f) < 2 {
		return ""
	}
	i := strings.LastIndex(f[1], "+")
	if i < 0 {
		return ""
	}
	return strings.TrimSuffix(f[1][i+1:], "-dirty")
}

// upgradeReading is one side of D4's comparison: what the binary at the
// target path answered, for every question D4 asks of it.
type upgradeReading struct {
	Stamp     string                         // what `posse version` named, "" when it named none
	Ran       bool                           // `posse version` exited 0 at all
	StatusRan bool                           // `posse status` printed its own header
	Status    map[string]int                 // conditions by "<class> <row>"
	Unread    int                            // `?` lines — reported, never a trigger
	Gates     map[string]upgradeGatesReading // by persona

	// Render is `posse gates render-probe <repo>`'s exit code and
	// RenderAsked says whether that code came from the verb at all — ADR
	// 0092 D4 amendment 3, and the two fields are inseparable. A binary
	// WITHOUT the verb exits 1 with `no such agent: render-probe`
	// (MEASURED, RenderProbeMark), which is also the verb's own code for "a
	// render does not refuse", so the exit code alone cannot tell the
	// before-reading of an old binary apart from a real finding. The mark
	// on stdout is what does.
	Render      int
	RenderAsked bool
}

// upgradeGatesReading is one persona's `posse gates <p>`: the exit code D4
// watches for a 0→non-zero move, and the count of rows saying a gate is not
// held.
type upgradeGatesReading struct {
	Exit int
	Red  int
}

// upgradeProbe runs the binary under test in dir and hands back its exit
// code and its two streams interleaved. An empty dir inherits this process's
// cwd, which is what the readings that do not depend on one pass.
//
// WHY A DIRECTORY IS AN ARGUMENT AT ALL (ADR 0092 D4 amendment 2,
// ranger-base-qfhph). `posse gates <p>` probes the L3 hooks OF ITS CWD and
// asks ADR 0023's identity question of them. This verb runs from wherever
// the delegate typed it — and a delegate standing in a worktree of a
// declared repo made the two sides of D4's diff read two different trees,
// one of them a tree whose hooks a render change had just made stale. So the
// gates readings run with their cwd FIXED to the source repo D3.1 names, and
// both sides read the same tree wherever the verb was typed. `version` and
// `status` are left on the inherited cwd deliberately: neither reads the
// cwd's hooks, and status's own repo-shaped inputs are the absolute paths
// `beads_visibility:` declares.
//
// A package var for the reason the three steps in upgrade.go are: the thing
// worth pinning is what this file DOES with the readings, and a pin cannot
// exec a posse binary that answers differently before and after a rename
// without building two of them. A test replaces this with a stub that reads
// the target's own bytes, so the stub's answer changes when — and only when
// — the swap really happened, which is the ordering the pins are about.
//
// protocolCmd for ADR 0077 D3's reason, and it is not academic here: this
// verb runs from a persona seat whose PATH leads with that seat's gates bin
// dir, and that dir holds a `posse` shim (every crew PID denies
// `Bash(posse promote:*)`). The binary itself is named by absolute path, so
// the shim cannot answer for it; what protocolCmd buys is the CHILD's PATH,
// because `posse status` and `posse gates` both shell out to git and bd.
var upgradeProbe = func(dir, bin string, argv ...string) (int, string) {
	cmd := protocolCmd(exec.Command(bin, argv...))
	cmd.Dir = dir
	var b strings.Builder
	cmd.Stdout, cmd.Stderr = &b, &b
	return upgradeExitCode(cmd.Run()), b.String()
}

// upgradeRead takes one side of the comparison off the binary at bin, with
// the `posse gates` readings run in repo (ADR 0092 D4 amendment 2 — see
// upgradeProbe's header for why that argument exists).
//
// `posse status`'s EXIT CODE IS NOT READ, and that is the measurement ADR
// 0092 D4 rests on: it exits 1 on any shop with any open condition at all
// (MEASURED 2026-11-13, and it does so by design — cmd/posse's status arm
// exits 1 for len(set) > 0), so on a live instance it is 1 before the swap
// and 1 after it whatever happened in between. The OUTPUT is the reading.
//
// `posse gates <p>`'s exit code IS read, because that command exits 0 on a
// healthy persona and D4 names a 0→non-zero move as a trigger.
//
// THAT COMMAND CAN WRITE, and this is the one place in the verb where a
// READING has a side effect, so it is said here rather than discovered. A
// gates report renders the shim dir for a persona that has NONE on disk; a
// persona whose wall is already there is read and never replaced
// (ranger-base-6s5mo). So the snapshot can create a wall for a persona that
// has never launched, and can do so for no other persona. That is bounded —
// a persona with no wall has no live seat to break — and it is rendered from
// THIS process's environment, which is the delegate's own and therefore the
// instance's HOME, not the `HOME=/elsewhere` spelling that left a running
// seat's git refused by its own gate (ranger-base-atq78).
func upgradeRead(bin, repo string, personas []string) upgradeReading {
	r := upgradeReading{Status: map[string]int{}, Gates: map[string]upgradeGatesReading{}}
	if code, out := upgradeProbe("", bin, "version"); code == 0 {
		r.Ran, r.Stamp = true, upgradeStampOf(out)
	}
	_, out := upgradeProbe("", bin, "status")
	r.Status = upgradeParseStatus(out)
	// The one line `posse status` prints unconditionally, which is what
	// makes it the anchor for "this command answered at all". A binary that
	// dies before it gets there printed no conditions either, and a diff
	// that read that as "nothing appeared" would be an all-clear over a
	// binary that cannot run.
	r.StatusRan = strings.Contains(out, "shop check ·")
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "?  ") {
			r.Unread++
		}
	}
	// repo and not the inherited cwd — the probe's header says why, and it
	// is the one argument in this function that is about WHERE rather than
	// what.
	for _, p := range personas {
		code, gout := upgradeProbe(repo, bin, "gates", p)
		r.Gates[p] = upgradeGatesReading{Exit: code, Red: upgradeGatesRed(gout)}
	}
	// ADR 0092 D4 amendment 3: the hook wall's BEHAVIOUR half, asked of the
	// binary under test directly instead of inferred from any repo's hook
	// files. The repo is named as an argument AND as the cwd, for amendment
	// 2's reason doubled: the verb's own default is the source checkout, and
	// a default that resolved differently in the two processes would be the
	// same two-trees defect this reading exists to avoid.
	//
	// Read-only on both sides (the verb writes no hook and only its own
	// private temp dir), so running it twice per install costs nothing a
	// snapshot may not spend.
	code, rout := upgradeProbe(repo, bin, "gates", "render-probe", repo)
	r.Render, r.RenderAsked = code, upgradeRenderAsked(rout)
	return r
}

// upgradeRenderAsked says whether the output came from the render-probe verb
// — the mark on stdout and not the exit code, for the reason
// RenderProbeMark's own header MEASURES: a binary without the verb exits 1,
// which is the verb's own "a render does not refuse".
func upgradeRenderAsked(out string) bool {
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), RenderProbeMark) {
			return true
		}
	}
	return false
}

// Line is the one-line form of a reading, for the verify block.
func (r upgradeReading) Line() string {
	conds := 0
	for _, n := range r.Status {
		conds += n
	}
	red, exits := 0, 0
	for _, g := range r.Gates {
		red += g.Red
		if g.Exit != 0 {
			exits++
		}
	}
	status := "status DID NOT ANSWER"
	if r.StatusRan {
		status = fmt.Sprintf("status %d condition(s), %d unreadable store(s)", conds, r.Unread)
	}
	// The render probe's answer in words, because "render-probe 1" and
	// "render-probe NOT ASKED" are the two readings a delegate has to be
	// able to tell apart from the block alone (ADR 0092 D4 amendment 3).
	render := "render-probe NOT ASKED (this binary has no such verb)"
	if r.RenderAsked {
		render = fmt.Sprintf("render-probe %d", r.Render)
	}
	return fmt.Sprintf("%s · gates %d persona(s), %d not-held row(s), %d non-zero exit(s) · %s",
		status, len(r.Gates), red, exits, render)
}

// upgradeVerdict is every reason the swap must be undone — all of them, not
// the first, because the delegate posts this on the install bead and a
// rollback with one reason printed over three is a report that will be
// re-derived by hand.
//
// Empty means the new binary stands.
func upgradeVerdict(to string, before, after upgradeReading) []string {
	var why []string

	// D4's first bullet. The stamp is compared as a PREFIX of the commit
	// rather than against shortSha(to), because what the Makefile stamps is
	// `git rev-parse --short`, whose length is git's to choose and is 8 on
	// this repo today — so an equality against a 7-character render would
	// refuse every good install on the day that number moves.
	switch {
	case !after.Ran:
		why = append(why, "the new binary does not run: `posse version` did not exit 0")
	case after.Stamp == "":
		why = append(why, "`posse version` names no commit, so nothing can say the binary in place is the one this run built")
	case !strings.HasPrefix(to, after.Stamp):
		why = append(why, fmt.Sprintf("`posse version` names %s and this run installed %s — the binary in place is not the one it built", after.Stamp, shortSha(to)))
	}

	// Not one of D4's three bullets, and it belongs with them: a status that
	// answered before the swap and does not answer after it is a binary that
	// cannot report on the shop, and every condition test below would read
	// its silence as an all-clear.
	if before.StatusRan && !after.StatusRan {
		why = append(why, "`posse status` answered before the swap and printed no shop check after it")
	}

	// D4's third bullet, counted by (class, row). Sorted so a rollback
	// report reads the same way twice.
	for _, row := range upgradeSortedKeys(after.Status) {
		gained := after.Status[row] - before.Status[row]
		if gained <= 0 {
			continue
		}
		name := strings.Fields(row)
		class := upgradeCondSource(name[len(name)-1])
		if class == upgradeShopState || class == upgradeInstallSubject {
			continue
		}
		known := ""
		if _, ok := upgradeConditionSource[name[len(name)-1]]; !ok {
			known = ", a row this binary has no class for at all — the new binary is reporting a condition this one cannot name"
		}
		why = append(why, fmt.Sprintf("`posse status` gained %d %s row(s) (%d → %d)%s",
			gained, row, before.Status[row], after.Status[row], known))
	}

	// D4's second bullet, per persona and in roster order.
	for _, p := range upgradeSortedGates(after.Gates) {
		b, had := before.Gates[p]
		if !had {
			continue
		}
		a := after.Gates[p]
		if b.Exit == 0 && a.Exit != 0 {
			why = append(why, fmt.Sprintf("`posse gates %s` exited 0 before the swap and %d after it", p, a.Exit))
		}
		if a.Red > b.Red {
			why = append(why, fmt.Sprintf("`posse gates %s` gained %d DEGRADED/REFUSED row(s) (%d → %d)", p, a.Red-b.Red, b.Red, a.Red))
		}
	}

	// ADR 0092 D4 AMENDMENT 3's TRIGGER — the hook wall judged by behaviour.
	// The new binary must answer 0, unless the old binary gave that same
	// non-zero answer, which makes it a property of the box (a broken
	// /bin/sh, a $TMPDIR this uid cannot write) and not of the install. A
	// box property is reported, in upgradeRenderNote, and is not a reason to
	// roll back: the fleet was already running with it.
	//
	// AN OLD ANSWER THAT DID NOT COME FROM THE VERB EXCUSES NOTHING, which
	// is the amendment's "counts as not asked, so the new binary must answer
	// 0" — and it is a MARK and not an exit code that says so (see
	// upgradeReading.Render). The first install after the verb lands is
	// exactly this case: the old binary answers 1 for a reason that has
	// nothing to do with any render.
	switch {
	case !after.RenderAsked:
		why = append(why, fmt.Sprintf("`posse gates render-probe` did not answer through the new binary (exit %d, and no %q line) — that verb is how this gate asks whether the new binary's own hook renders still refuse (ADR 0092 D4 amendment 3), and the OLD binary's code is what calls it, so a binary without it cannot be judged",
			after.Render, RenderProbeMark))
	case after.Render == 0:
		// Every render of the new binary refuses what it gates. Nothing
		// owed, and no note: a line printed on every good install is a line
		// the delegate stops reading.
	case before.RenderAsked && before.Render == after.Render:
		// The old binary, asked the same question in the same place,
		// answered the same thing. upgradeRenderNote says so.
	default:
		meant := map[int]string{
			1: "one of this binary's own L3 hook renders no longer refuses the operation it gates",
			2: "the probe could not be taken at all",
		}[after.Render]
		if meant == "" {
			meant = "the verb answered with a code it does not define"
		}
		was := fmt.Sprintf("the old binary answered %d", before.Render)
		if !before.RenderAsked {
			was = "the old binary did not have the verb, so nothing excuses it"
		}
		why = append(why, fmt.Sprintf("`posse gates render-probe` exited %d through the new binary — %s (%s)", after.Render, meant, was))
	}
	return why
}

// upgradeRenderNote is the report half of amendment 3's trigger: the one
// case where the behaviour probe comes back non-zero through the new binary
// and is NOT a reason to roll back, because the old binary said the same
// thing and the fleet has been running on it.
//
// It prints rather than refusing, because that is what the amendment asks
// for — "a box property such as a broken /bin/sh, which is reported and is
// not a reason to roll back" — and because the delegate posts this block on
// the install bead, where a probe nobody can take is worth a line whoever
// reads it next.
func upgradeRenderNote(before, after upgradeReading) string {
	if after.Render == 0 || !after.RenderAsked || !before.RenderAsked || before.Render != after.Render {
		return ""
	}
	what := map[int]string{
		1: "an L3 hook render that does not refuse what it gates — the SAME render on both sides, so the commit this run installed did not cause it",
		2: "a probe that cannot be taken here at all, which is a broken /bin/sh or a $TMPDIR this uid cannot write",
	}[after.Render]
	if what == "" {
		what = "a code the verb does not define"
	}
	return fmt.Sprintf("  note: `posse gates render-probe` exits %d through BOTH binaries, so it is a property of this box and not of the install (ADR 0092 D4 amendment 3): %s. The hook wall's behaviour half was in this state before this command ran, and is worth fixing before the next upgrade.\n", after.Render, what)
}

// upgradeG12 is the one row name this verb's own install is expected to
// raise and is not allowed to roll back for — the L3 hook wall.
const upgradeG12 = "G12"

// upgradeHookWallGained counts the G12 rows the after-reading has that the
// before-reading lacked, across both classes `posse status` can print them
// in. Only gains count: a row that CLEARED is an install that happened to
// fix a wall, and nothing is owed the operator for that.
func upgradeHookWallGained(before, after upgradeReading) int {
	n := 0
	for row, count := range after.Status {
		f := strings.Fields(row)
		if len(f) == 0 || f[len(f)-1] != upgradeG12 {
			continue
		}
		if gained := count - before.Status[row]; gained > 0 {
			n += gained
		}
	}
	return n
}

// upgradeHookWallLine is ADR 0092 D4 amendment 1's one duty to the operator
// (ranger-base-qfhph): when the swap raised G12 rows, say so and print the
// command that closes them.
//
// IT IS A REPORT AND NOT AN ACT. D7 keeps `posse gates install-hooks` out of
// this verb's scope on purpose — it writes the commit wall into every
// declared checkout, one of them the constitution repo — so this prints the
// line and the operator types it. The verb never runs it, and the delegate
// is not asked to either: they post this report on the install bead, which
// is where D6 says the record lives.
//
// The repos are the plan's, which are the ones `beads_visibility:` declares
// and the sweep could measure. A skipped repo (absent, or a managed hooks
// path posse may not write) raises no G12 row and must not appear in a
// command prescribing a write there — ADR 0052 D1 is the one instruction
// this line could give that would be worse than giving none.
func upgradeHookWallLine(plan upgradePlan, before, after upgradeReading) string {
	gained := upgradeHookWallGained(before, after)
	if gained == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  note: `posse status` gained %d %s hook-wall row(s). Each repo's git hook is a COPY of this binary's render and this commit changed the render, so those rows say the install's second half has not happened — not that the new binary is wrong (ADR 0092 D4 as amended, D7).\n", gained, upgradeG12)
	if len(plan.Repos) == 0 {
		fmt.Fprintf(&b, "    re-render them with `posse gates install-hooks <repo>…` — the operator's, and this verb never runs it.\n")
		return b.String()
	}
	repos := make([]string, 0, len(plan.Repos))
	for _, r := range plan.Repos {
		repos = append(repos, AbbrevHome(r))
	}
	fmt.Fprintf(&b, "    for the operator to type — this verb never runs it:\n      posse gates install-hooks %s\n", strings.Join(repos, " "))
	return b.String()
}

func upgradeSortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func upgradeSortedGates(m map[string]upgradeGatesReading) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// upgradeRollback puts <target>.prev back over the target — ADR 0092 D4's
// automatic arm.
//
// NEVER THROUGH THE NEW BINARY, which is the whole point of D4's command:
// this is the OLD binary's own code, running in a process that was exec'd
// from the old inode and is unaffected by the rename. It shells out to
// nothing at all, so there is no PATH, no shim and no gate between the
// decision and the file.
//
// WRITE-BESIDE-THEN-rename(2) rather than the `install -m 0755` the printed
// command uses. Both are "not through the new binary", and the printed one
// is for a HUMAN typing it after this process has exited; for the arm that
// runs while a fleet may be launching, the measurement in upgrade.go's
// header applies with its full force — install(1) leaves the target absent
// or short for ~16 ms, and a rollback is by definition the moment when the
// binary at that path matters most.
//
// <target>.prev IS LEFT WHERE IT IS. After this returns, the live binary and
// .prev hold the same bytes, which costs one binary of disk and keeps the
// command this run printed before the swap true for whoever reads the report
// afterwards. Overwriting it with the binary being rolled back would destroy
// the only copy of the one being rolled back TO.
func upgradeRollback(target string) error {
	prev := upgradePrevPath(target)
	body, err := os.ReadFile(prev)
	if err != nil {
		return Die("posse upgrade: the new binary must be rolled back and %s cannot be read (%v) — the binary now at %s is the one this run installed, and the rollback is the operator's: %s",
			AbbrevHome(prev), err, AbbrevHome(target), upgradeRollbackCmd(target))
	}
	staged := target + ".rollback"
	if err := WriteExecutable(staged, body, 0o755); err != nil {
		return Die("posse upgrade: cannot stage the rollback at %s: %v — roll back by hand: %s",
			AbbrevHome(staged), err, upgradeRollbackCmd(target))
	}
	if err := os.Rename(staged, target); err != nil {
		os.Remove(staged)
		return Die("posse upgrade: cannot swap %s back into place: %v — roll back by hand: %s",
			AbbrevHome(target), err, upgradeRollbackCmd(target))
	}
	return nil
}

// upgradeVerify is D4 whole, from the far side of the swap: read the new
// binary, judge it against the snapshot, and undo the swap when it is worse.
//
// It returns an error when the install did not stand — whether or not the
// rollback itself succeeded, which the report distinguishes and the error
// does not: either way the run did not install anything the caller may
// report as installed.
func upgradeVerify(w io.Writer, plan upgradePlan, before upgradeReading) error {
	after := upgradeRead(plan.Target, plan.Repo, plan.Personas)
	fmt.Fprintf(w, "  before: %s\n", before.Line())
	fmt.Fprintf(w, "  after:  %s\n", after.Line())

	why := upgradeVerdict(plan.To, before, after)
	if len(why) == 0 {
		fmt.Fprintf(w, "  ✓ nothing the binary renders is newly red — the install stands\n")
		// D7's half of the install, printed only now: on a run that is
		// about to be undone the hook walls are about a binary that will
		// not be there, and prescribing a re-render against it would be
		// the one wrong instruction this block can give.
		if line := upgradeHookWallLine(plan, before, after); line != "" {
			fmt.Fprint(w, line)
		}
		// Amendment 3's report half, printed beside D7's for the same
		// reason: an install that stood on a box where the behaviour probe
		// cannot be taken is a fact about the next upgrade too.
		fmt.Fprint(w, upgradeRenderNote(before, after))
		return nil
	}
	for _, r := range why {
		fmt.Fprintf(w, "  ✗ %s\n", r)
	}
	fmt.Fprintf(w, "rolling back (ADR 0092 D4): %s → %s\n", AbbrevHome(upgradePrevPath(plan.Target)), AbbrevHome(plan.Target))
	if err := upgradeRollback(plan.Target); err != nil {
		fmt.Fprintf(w, "ROLLBACK FAILED — %v\n", err)
		return err
	}
	fmt.Fprintf(w, "rolled back: %s is the binary that was there before this command ran (%s)\n",
		AbbrevHome(plan.Target), shortSha(plan.From))
	return Die("posse upgrade: %s was installed and rolled back — %s. %s is unchanged from before this command ran; %s is still beside it.",
		shortSha(plan.To), strings.Join(why, "; "), AbbrevHome(plan.Target), AbbrevHome(upgradePrevPath(plan.Target)))
}
