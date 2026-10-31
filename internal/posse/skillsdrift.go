package posse

import (
	"fmt"
	"io"
)

// ─── the skills_verified clock (ranger-base-f5jbs) ───────────────────────────
//
// skillsVerifiedDrift (parity.go) has said the right sentence since ADR 0007
// §6c and said it to nobody on a clock. Both of its readers are DISPLAY — the
// Realized detail a launch prints, and `posse runtime check`'s skills row —
// and both exit 0 over a drift, correctly, because §6c reports and never
// refuses a LAUNCH. So when bob self-updated 2.0.5 → 2.0.7 the tree found out
// from an mtime a pulse happened to read (ranger-base-lf6it), and not from
// the detector that was already right and already installed.
//
// This is a third reader of that same seam with an EXIT CODE: one line per
// launch profile, nonzero when any profile's installed CLI has moved away
// from the version its skill discovery was measured on. It changes no launch
// — §6c stays reporting-and-not-refusing — it puts a gate surface beside it,
// for the detective tier to run on a schedule.
//
// NOT A SHELL GREP OVER `posse runtime check`'s OUTPUT. A matcher that
// answers nothing — signalled, EPIPE, cannot exec under load — is
// indistinguishable from "no drift": it prints green and exits 0, which is
// the exact failure this exists to close, and the forked-matcher shape
// ranger-base-s8b4g rewrote three verify-*-pin.sh scripts to be rid of. The
// exit code comes from the Go side that already holds the fact.
//
// EXIT STATUS, in scripts/verify-box.sh's per-check contract, so a roster row
// is the recipe and nothing else:
//
//	0  every profile that could be measured matches its skills_verified:
//	1  at least one FINDING: a drift, or a profile posse cannot read at all
//	2  NOTHING WAS MEASURED — no profile on this box both declares a
//	   skills_verified: and has a CLI here that answers --version. That is
//	   the case on a machine with none of the runtimes installed, and it is
//	   not a pass.
const (
	SkillsDriftClean      = 0
	SkillsDriftFound      = 1
	SkillsDriftUnmeasured = 2
)

// SkillsDriftRow is one launch profile's verdict. Drift is the single
// decider — skillsVerifiedDrift's own sentence, so this surface and the two
// display readers cannot disagree about what counts as drift — and Declared
// and Installed are beside it for the reader, never re-compared.
type SkillsDriftRow struct {
	Runtime   string
	Declared  string // skills_verified:, "" when this profile declares none
	Installed string // what <exe> --version answered now, "" when it would not answer
	Drift     string // skillsVerifiedDrift's sentence; "" when there is nothing to report
	// Skip names why nothing was measured here, and is empty for a profile
	// that was measured (clean or drifted). Err is the load refusal for a
	// profile posse cannot read at all, which is a finding and not a skip:
	// a yaml that will not parse hides whatever drift it declares.
	Skip string
	Err  string
}

// Measured reports whether this row is a reading of the box rather than a
// reason there is none.
func (r SkillsDriftRow) Measured() bool { return r.Skip == "" && r.Err == "" }

// SkillsDrift reads every launch profile this box has — the built-ins and
// runtimes/*.yaml alike — for ADR 0007 §6c drift. Rows come back in
// ListRuntimes order, so the walk reads as the roster it covered and not as
// its findings alone: a profile this cannot measure is the thing a reader
// would otherwise go looking for a missing finding about.
func (a *App) SkillsDrift() []SkillsDriftRow {
	var rows []SkillsDriftRow
	for _, name := range a.ListRuntimes() {
		row := SkillsDriftRow{Runtime: name}
		rt, err := a.LoadRuntime(name)
		if err != nil {
			// `posse runtimes` already reds on this; so does this, for its
			// own reason — a refused profile may declare a skills_verified:
			// nobody can read, and exiting 0 over one is a green over a
			// profile this walk never saw.
			row.Err = fmt.Sprintf("REFUSED — %v", err)
			rows = append(rows, row)
			continue
		}
		row.Declared = rt.SkillsVerified
		switch {
		case rt.SkillsVerified == "" && rt.SkillsRoot != "":
			// ADR 0007 §6b. A different finding, and already a loud one:
			// a `skills:` PID here degrades the launch and refuses without
			// --allow-degraded. There is no version to compare, so this
			// walk has nothing to add beyond saying it did not cover it.
			row.Skip = fmt.Sprintf("no skills_verified: — %s's discovery root %s is UNPROVEN (never measured live), so there is no version here to drift from (ADR 0007 §6b)", name, rt.SkillsRoot)
		case rt.SkillsVerified == "":
			row.Skip = "no skills_verified: — this profile's skill surface is a flag (or none at all), so no CLI version is the subject of a measurement here"
		default:
			row.Installed = skillsDriftVersion(rt.Exe())
			if row.Installed == "" {
				row.Skip = fmt.Sprintf("skills_verified: %q, but %s answers no --version on this box (not installed, or will not say) — an unknown version is unknown, never unchanged", rt.SkillsVerified, rt.Exe())
				break
			}
			row.Drift = skillsVerifiedDrift(rt)
		}
		rows = append(rows, row)
	}
	return rows
}

// ReportSkillsDrift prints the walk and returns the exit status above.
func (a *App) ReportSkillsDrift(w io.Writer) int {
	rows := a.SkillsDrift()
	measured, findings := 0, 0
	for _, r := range rows {
		if r.Measured() {
			measured++
		}
		if r.Drift != "" || r.Err != "" {
			findings++
		}
	}
	switch {
	case findings > 0:
		fmt.Fprintf(w, "skills_verified: %d of %d launch profile(s) measured, %d FINDING(S) (ADR 0007 §6c)\n", measured, len(rows), findings)
	case measured == 0:
		fmt.Fprintf(w, "skills_verified: 0 of %d launch profile(s) could be measured — NOTHING WAS MEASURED, which is not a pass\n", len(rows))
	default:
		fmt.Fprintf(w, "skills_verified: %d of %d launch profile(s) measured, every one on the version its skill discovery was measured on\n", measured, len(rows))
	}
	for _, r := range rows {
		switch {
		case r.Err != "":
			fmt.Fprintf(w, "  ✗ %-8s %s\n", r.Runtime, r.Err)
		case r.Drift != "":
			fmt.Fprintf(w, "  ✗ %-8s %s\n", r.Runtime, r.Drift)
		case r.Skip != "":
			fmt.Fprintf(w, "  · %-8s %s\n", r.Runtime, r.Skip)
		case r.Installed != r.Declared:
			// Clean, but not identical: the installed line carries a
			// trailing bracketed field the declaration deliberately does
			// not, so the tick would otherwise claim the box "reports the
			// same" over two different strings (ranger-base-3a5z7). Say
			// both and say why they are one release.
			fmt.Fprintf(w, "  ✓ %-8s skills_verified: %q and %s reports %q — the same release, the trailing [field] is box state and not the binary (ADR 0007 §6c)\n",
				r.Runtime, r.Declared, r.Runtime, r.Installed)
		default:
			fmt.Fprintf(w, "  ✓ %-8s skills_verified: %q and %s reports the same\n", r.Runtime, r.Declared, r.Runtime)
		}
	}
	if findings > 0 {
		fmt.Fprint(w, "  the remedy is a RE-MEASUREMENT, not a version bump in the declaration: prove the\n"+
			"  discovery root is still read on the installed CLI (TestQALiveSkillDiscoveryPerRuntime,\n"+
			"  RHQ_PARITY_LIVE=1), then update skills_verified: to the --version line you measured against.\n")
	}
	switch {
	case findings > 0:
		return SkillsDriftFound
	case measured == 0:
		return SkillsDriftUnmeasured
	}
	return SkillsDriftClean
}
