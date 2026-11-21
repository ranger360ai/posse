//go:build !linux

package posse

import "runtime"

// No `/proc`, so no box-wide loop census — and the refusal is the deliverable
// on this platform, not a stub (procstart_other.go's rule, and G18's own:
// watchimage.go abstains where /proc cannot answer rather than claiming a
// reading).
//
// WHY THERE IS NO DARWIN ROUTE HERE. The census needs two things per process:
// the (dev, inode) of the image it runs, and its RHQ_HOME. The process table
// `cmd/checkorphans` reads through `sysctl kern.proc.all` (proctable_darwin.go)
// gives neither — it carries pid, ppid, comm and argv — and argv is the one
// key this reader must not use, because every seat on a box runs
// byte-identical argv from a different instance (AGENTS.md, "Ending
// anything"). A darwin route would need `proc_pidinfo`'s
// PROC_PIDVNODEPATHINFO for the image and a `sysctl KERN_PROCARGS2` parse for
// the environment, neither of which has been measured on a darwin box from
// inside a seatbelt sandbox — and an abstention costs a line in a report,
// while a census compared against an unmeasured spelling passes for the wrong
// reason.
//
// What the abstention costs is exactly what the box had before
// ranger-base-is1qp: the per-instance reading above it in the report, with
// `watchScopeSentence` saying out loud that ADR 0092 D6's "restart the
// dispatch loops" reaches instances it did not look at. BoxReading.Sentence
// renders this error as that sentence, so a darwin reader is told the residual
// stands rather than being handed a silence.
func boxPids() ([]int, error) {
	return nil, Die("no /proc on %s, so the processes running this binary cannot be censused — the per-instance loop reading above is the whole of this report's coverage (boxloops_other.go, ADR 0092 D8)", runtime.GOOS)
}

// boxHomeOf has no environ to read either. Nothing reaches it off linux —
// boxPids refuses first — and it answers "not available" rather than a guess
// so that stays true if that order ever changes.
func boxHomeOf(pid int) (string, bool) { return "", false }

// boxPidOwned has no /proc/<pid> to stat. False, which on this platform means
// "not an unreadable candidate either": with no census at all there is
// nothing to be unsure about, and boxPids' refusal is what the reader is
// handed (ranger-base-rph5z).
func boxPidOwned(pid int) bool { return false }

// boxCommOf has no /proc/<pid>/comm. Nothing reaches it off linux (boxPids
// refuses first) and "not available" is the honest answer rather than a
// guess, same as the two above.
func boxCommOf(pid int) (string, bool) { return "", false }
