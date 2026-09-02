package posse

// The load guard's kill arm — arm 2 of ranger-base-apwr, implementing the
// operator's 2026-08-31 ruling on ranger-base-z9m2 (ranger-base-gvp2p).
//
// Arm 1 (loadguard.go, orphanReport) names the leaked gate-shell children a
// wedged box is holding and kills none of them, because exactly one
// false-positive shape exists and nothing in the process table separates it
// from a leak: a persona that DELIBERATELY backgrounds a long-lived
// CPU-consuming process from a Bash line and lets the tool call return. It
// is reparented for the same reason, carries the same ADR 0009 preamble for
// the same reason, and clears both floors. comm, argv, age and CPU are
// identical. The difference is intent, and intent is not in the process
// table — so the ruling put it there.
//
// DECLARE OR DIE. A persona that means to leave something running writes
// `POSSE_KEEP=<reason>` on that Bash line; the guard spares what is declared
// and ends what is not. Undeclared is a leak and a leak is killed. The
// burden sits on whoever spawns the process, which is the only party that
// knows, and it needs no permission-file edit per legitimate case — the two
// rejected shapes (a per-persona PID field, an instance-level list) both did.
//
// WHY THE MARKER IS READ LOOSELY WHILE THE PREAMBLE IS READ AT THE HEAD.
// These two tests fail in opposite directions and so must be anchored
// differently, and getting that backwards is how a reaper kills something
// somebody meant:
//
//   - the preamble answers "is this OURS", and a LOOSE match there kills a
//     stranger's process. So it is anchored at the head of the -c string,
//     and a line that merely TALKS about the preamble is not a leak
//     (gateShellForkPayload, and the teau misreading it exists to refuse).
//   - the marker answers "was this DECLARED", and it is the SPARE. A loose
//     match there means a leak survives — which is arm 1's world, still
//     reported, still killable by hand. A tight match means a declared
//     process dies, and that is irreversible. So the marker counts anywhere
//     in the argv, and the documented form (head of the line) is a
//     convention for the reader, not a condition of survival.
//
// It costs nothing that matters: a forked subshell never execs, so it
// carries its parent's WHOLE -c string — the persona's entire Bash line,
// wherever in it the declaration was written. Anything that DID exec has
// its own argv, does not match the preamble, and was never in the predicate
// at all. So "the marker only has to survive on processes that kept the
// whole line" is exactly the set this arm can touch.
//
// WHICH SIGNAL, MEASURED (deliverable 2; teau RCA §7 is four shells of
// evidence that this is where reapers fail). Planted control on darwin
// 25.4.0, a forked non-exec'd subshell running a bounded busy loop, parent
// exited:
//
//	/bin/sh    TERM -> dead in 31ms
//	/bin/zsh   TERM -> dead in 25ms
//	/bin/bash  TERM -> dead in 24ms
//
// and the wrong arm that makes those numbers mean something — the same
// spinner behind `trap "" TERM`, which is what a persona's own cleanup
// handler looks like from outside:
//
//	/bin/sh    TERM -> alive at 3041ms; KILL -> dead in 24ms
//	/bin/zsh   TERM -> alive at 3019ms; KILL -> dead in 24ms
//
// So: TERM, one bounded grace, then KILL for whatever ignored it. TERM
// first is not politeness — a shell that traps TERM is a shell with cleanup
// to run, and this arm has no reason to deny it the chance. monica's
// 08-31 kill needing KILL after TERM was an argv-splitting bug rather than
// an unkillable process, and this re-measure says so: nothing here needed
// KILL unless it had explicitly refused TERM.
//
// FAIL OPEN, exactly like the reading above it. Every wait is bounded and
// SHARED across the whole batch — one TERM round, one grace, one KILL
// round, one confirm — so forty leaks cost what one costs, and a pid that
// will not die is a word in a log line rather than a pass that is late.
//
// FAIL CLOSED ON THE KILL ITSELF, which is the opposite rule and the right
// one for a destructive act: the reaper re-reads its targets' rows
// immediately before signalling, and a target it cannot re-verify — pid
// gone, pid recycled into something else, ppid no longer 1, argv no longer
// ours, a declaration that appeared in between — is skipped, not killed.
//
// SHIPPED OFF. `load_guard_kill:` defaults to false and this bead does not
// flip it: arm 1 has exactly one field datum (ranger-base-k6csq, 40 real
// leaks and no false positive) and the ruling's first verification bar is
// field data, which is the operator's to read. The live flip is its own
// operator bead.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The outcome words the kill arm writes beside each pid. They are constants
// rather than literals at each site because killArmLine counts what ENDED off
// them: a header that said "15 of 16" while the body said otherwise would be
// a wrong claim about an irreversible act, and a suffix match on the word
// "gone" is not a thing to hang that on. Anything not in this pair is a
// sentence explaining why nothing was signalled, and never counts as ended.
const (
	killedByTERM = "TERM: gone"
	killedByKILL = "TERM ignored, KILL: gone"
)

// LoadOrphanKeepMarker is the declare-or-die token, as it appears in the
// process table. A persona writes `POSSE_KEEP=<reason>` at the head of the
// Bash line that backgrounds something long-lived; the reason is
// conventionally the bead id that authorises it, and the guard prints it
// rather than judging it.
//
// The spelling is a shell ASSIGNMENT for a reason no comment token can
// match: a Bash line is one -c string, so a leading `#` comments out the
// whole line, while `POSSE_KEEP=x cmd &` and `POSSE_KEEP=x; cmd &` are both
// inert, valid in every POSIX shell, and land the token in argv either way.
// It is a `POSSE_`-prefixed name so that reading it as an env var — which
// the declared process really does receive in the first form — is a feature
// and not a collision.
const LoadOrphanKeepMarker = "POSSE_KEEP="

const (
	// loadKillGrace is how long the whole batch gets to honour TERM before
	// the survivors are KILLed. The measured deaths are 24-31ms; this is
	// the margin over that, not an estimate of it.
	loadKillGrace = 500 * time.Millisecond
	// loadKillConfirm is how long the batch gets to die of KILL before the
	// line calls it still running. A KILL that has not landed in this long
	// is a pid the guard cannot end, and saying so is the whole of the
	// fail-open contract here.
	loadKillConfirm = 250 * time.Millisecond
	// loadKillPoll is how often the two waits above look. Fine enough that
	// the common case (everything dead in ~25ms) reports promptly.
	loadKillPoll = 25 * time.Millisecond
	// loadKillVerifyTimeout bounds the re-read the reaper takes before it
	// signals anything. Past it nothing is killed at all — see FAIL CLOSED
	// above.
	loadKillVerifyTimeout = 2 * time.Second
)

// LoadGuardKill reads config `load_guard_kill:` — whether the load guard may
// END the leaked gate-shell children arm 1 names, rather than only naming
// them. Absent is FALSE and false is the shipped default: this is a
// destructive arm behind an operator flip, and the ruling's first bar is
// field data from arm 1 that only the operator can read.
//
// Only `true` and `false` spell it. Anything else is a typo, and a typo
// here reads as a declaration nobody made — so it is named and the arm
// stays off. The warning is returned rather than written because the one
// caller is assembling a log line and that is where it will actually be
// seen (LoadCulpritLine takes no writer).
func (a *App) LoadGuardKill() (armed bool, warn string) {
	switch raw := strings.TrimSpace(YamlGet(a.ConfigPath, "load_guard_kill")); raw {
	case "", "false":
		return false, ""
	case "true":
		return true, ""
	default:
		return false, fmt.Sprintf("load guard: config load_guard_kill: %q is not true or false — the kill arm stays OFF", raw)
	}
}

// orphanDeclared reads the declare-or-die marker off a process's argv and
// hands back the reason it named. It is given the WHOLE argv rather than the
// payload behind the preamble, because a declaration the guard cannot see is
// a process it would kill: the argv is strictly more than the payload, it
// never contains the token by itself (our preamble is PATH-rebuilding shell
// and the token is ours), and the head-with-no-tail case has no payload at
// all to search.
//
// The reason is the rest of the token's own word, unquoted at both ends so
// `POSSE_KEEP='ranger-base-abcd'` reads the same as the bare form. An empty
// reason still declares — the ruling asks a persona to say why, and a guard
// that killed a declared process over a missing word would be enforcing
// prose with a signal.
func orphanDeclared(args string) (string, bool) {
	i := strings.Index(args, LoadOrphanKeepMarker)
	if i < 0 {
		return "", false
	}
	reason := args[i+len(LoadOrphanKeepMarker):]
	if j := strings.IndexAny(reason, " \t\n;&|"); j >= 0 {
		reason = reason[:j]
	}
	return strings.Trim(reason, `'"`), true
}

// sysReapOrphans is the real kill arm: it re-verifies the targets, TERMs
// them, waits one shared grace, KILLs the survivors, waits one shared
// confirm, and hands back what happened to each pid — the outcome word the
// log line prints beside it.
//
// Every pid offered gets an entry, including the ones it declined to
// signal: a destructive arm that says nothing about a target it skipped is
// a destructive arm nobody can audit.
func sysReapOrphans(targets []Proc) map[int]string {
	out := make(map[int]string, len(targets))
	if len(targets) == 0 {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), loadKillVerifyTimeout)
	defer cancel()
	live := killVerify(ctx, targets, out)

	var termed []int
	for _, t := range targets {
		if !live[t.PID] {
			continue // killVerify already wrote why
		}
		if err := signalPID(t.PID, syscall.SIGTERM); err != nil {
			out[t.PID] = "TERM refused: " + err.Error()
			continue
		}
		termed = append(termed, t.PID)
	}
	termed = waitGone(termed, loadKillGrace, out, killedByTERM)

	var killed []int
	for _, pid := range termed {
		if err := signalPID(pid, syscall.SIGKILL); err != nil {
			out[pid] = "TERM ignored, KILL refused: " + err.Error()
			continue
		}
		killed = append(killed, pid)
	}
	for _, pid := range waitGone(killed, loadKillConfirm, out, killedByKILL) {
		out[pid] = "STILL RUNNING after TERM and KILL"
	}
	return out
}

// killVerify re-reads exactly the candidate pids immediately before anything
// is signalled, and reports which are still, right now, a leak this arm may
// end. Every pid it drops has its reason written into out, because a
// destructive arm that says nothing about a target it skipped is a
// destructive arm nobody can audit.
//
// This is the FAIL CLOSED half. The census that selected these rows was
// taken milliseconds ago, but the act about to follow is irreversible and
// the window is real: a pid that died in between can be recycled onto
// something else entirely, and a persona may have declared in between too.
// So the whole predicate is applied a second time against a fresh reading —
// still reparented, still ours by the preamble, still undeclared — and a
// reading it cannot take at all kills nothing.
func killVerify(ctx context.Context, targets []Proc, out map[int]string) map[int]bool {
	live := map[int]bool{}
	ids := make([]string, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, strconv.Itoa(t.PID))
	}
	// args is the last column and so untruncated, the same reason
	// fillArgsForPIDs reads it alone; ppid rides ahead of it because that
	// half of the predicate can go stale too.
	raw, err := exec.CommandContext(ctx, "ps", "-ww", "-o", "pid=,ppid=,args=", "-p", strings.Join(ids, ",")).Output()
	if err != nil {
		// `ps` refusing — and the box this runs on is a box that cannot
		// fork — is not a licence to kill from a stale table.
		for _, t := range targets {
			out[t.PID] = "not signalled: the process table could not be re-read"
		}
		return live
	}
	rows := parseVerifyTable(string(raw))
	for _, t := range targets {
		r, ok := rows[t.PID]
		switch {
		case !ok:
			// Gone between the census and now: the ordinary happy ending
			// for a leak whose own box finished with it.
			out[t.PID] = "already gone before it was signalled"
		case r.PPID != 1:
			out[t.PID] = fmt.Sprintf("not signalled: ppid is %d, no longer an orphan", r.PPID)
		default:
			if _, ours := gateShellForkPayload(r.Args); !ours {
				out[t.PID] = "not signalled: argv is no longer one of ours (pid reused?)"
				continue
			}
			if reason, declared := orphanDeclared(r.Args); declared {
				out[t.PID] = "not signalled: declared " + LoadOrphanKeepMarker + reason + " in the meantime"
				continue
			}
			live[t.PID] = true
		}
	}
	return live
}

// verifyRow is one row of killVerify's re-read.
type verifyRow struct {
	PPID int
	Args string
}

// parseVerifyTable reads `pid=,ppid=,args=`: two integers and then the whole
// rest of the line, which is argv space-joined. Lines it cannot read are
// dropped rather than guessed at — and dropping one here means NOT killing
// that pid, which is the direction this parser is allowed to be wrong in.
func parseVerifyTable(out string) map[int]verifyRow {
	rows := map[int]verifyRow{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		pid, errPID := strconv.Atoi(f[0])
		ppid, errPPID := strconv.Atoi(f[1])
		if errPID != nil || errPPID != nil {
			continue
		}
		// Re-join from the raw line rather than from Fields, so the argv
		// keeps the spacing the persona typed.
		rest := strings.TrimLeft(line, " ")
		rest = strings.TrimLeft(strings.TrimPrefix(rest, f[0]), " ")
		rest = strings.TrimLeft(strings.TrimPrefix(rest, f[1]), " ")
		rows[pid] = verifyRow{PPID: ppid, Args: strings.TrimRight(rest, " ")}
	}
	return rows
}

// waitGone polls the given pids for at most d, recording gone as the given
// outcome, and returns whoever is still there. One wait for the whole batch:
// forty leaks cost what one costs, which is what keeps this arm off the
// critical path of a pass it is not allowed to delay.
func waitGone(pids []int, d time.Duration, out map[int]string, gone string) []int {
	deadline := time.Now().Add(d)
	for {
		left := pids[:0:0]
		for _, pid := range pids {
			if procAlive(pid) {
				left = append(left, pid)
			} else {
				out[pid] = gone
			}
		}
		pids = left
		if len(pids) == 0 || !time.Now().Before(deadline) {
			return pids
		}
		time.Sleep(loadKillPoll)
	}
}

// signalPID sends one signal, through os so this file compiles on every
// platform loadavg_other.go covers.
//
// A pid that is not positive is REFUSED and never reaches the kernel. That
// is not defensive decoration: kill(2) reads 0 as "my own process group"
// and a negative pid as "the process group |pid|", so a `ps` row this file
// mis-parsed, or a table read on a platform whose columns are not what we
// think, would turn one leak into a group kill of the fleet. The whole of
// this arm is one signal at a time to one pid a census named.
func signalPID(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("refusing to signal pid %d: only a positive pid names one process", pid)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(sig)
}

// procAlive is signal 0: no error means alive, and an error that is not
// "already finished" (EPERM, most likely) means alive and not ours — which
// is still alive, and the honest answer for a line that has to say whether
// the pid went away.
func procAlive(pid int) bool {
	if pid <= 0 {
		return false // never a process this arm may ask about — see signalPID
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return !strings.Contains(err.Error(), "process already finished") &&
		!strings.Contains(err.Error(), "no such process")
}

// reapOrphans is the App's seam onto sysReapOrphans, for the reason Load1
// and TopCPU are seams: a unit pin must be able to drive the render and the
// selection without signalling anything on the machine the suite is running
// on.
func (a *App) reapOrphans(targets []Proc) map[int]string {
	reap := a.ReapOrphans
	if reap == nil {
		reap = sysReapOrphans
	}
	return reap(targets)
}

// killArmLine renders what the kill arm did, as the tail of the orphan
// report's header plus one line per attempt.
//
// EVERY attempt is named, and that is deliberately different from arm 1's
// three-and-count. A report is a summary and may be cut; a kill is an
// irreversible act and the log line is the only record that it happened, so
// "15 killed" without saying which fifteen is not a log. Deliverable 4 of
// the ruling asks for pid, argv head, signal and outcome, and this is where
// all four land.
func killArmLine(leaks []leakRow, outcomes map[int]string) (header, body string) {
	ended := 0
	for _, l := range leaks {
		if o := outcomes[l.p.PID]; o == killedByTERM || o == killedByKILL {
			ended++
		}
	}
	header = fmt.Sprintf(" — load_guard_kill: true, %d of %d ended:", ended, len(leaks))
	for _, l := range leaks {
		o := outcomes[l.p.PID]
		if o == "" {
			o = "not re-verified: skipped"
		}
		body += fmt.Sprintf("\n    %.1f%% pid %d %s: %s — %s",
			l.p.CPU, l.p.PID, BlindFor(l.p.Age), leakWhat(l), o)
	}
	return header, body
}

// declaredLine names the orphans the marker spared, and it is printed in
// BOTH modes. A declared long-lived process on this box is meant to be rare
// and loud (the ruling's own words), so the guard says one is there every
// time it looks — and while the arm is off, this is also what keeps arm 1's
// field data honest, since a declared process reported as a leak is exactly
// the false positive the flip is waiting on.
func declaredLine(spared []leakRow) string {
	if len(spared) == 0 {
		return ""
	}
	ones := "orphans"
	if len(spared) == 1 {
		ones = "orphan"
	}
	out := fmt.Sprintf("\n  load guard: %d declared %s spared (%s), not killed:", len(spared), ones, LoadOrphanKeepMarker)
	for _, l := range spared {
		out += fmt.Sprintf("\n    %.1f%% pid %d %s [%s]: %s",
			l.p.CPU, l.p.PID, BlindFor(l.p.Age), l.reason, leakWhat(l))
	}
	return out
}

// SelfOrphanKeepNote is what a persona reads under checkorphans when it has
// leaked something it may have meant to keep. Documenting the marker in
// NOTES.md and AGENTS.md reaches a persona that goes looking; this reaches
// the one that is being told, right now, that it left something running —
// which is the only moment the spelling is worth anything.
//
// "" when every leak listed is already declared: telling a persona to write
// a marker it has written is how a tool teaches people to stop reading it.
func SelfOrphanKeepNote(leaks []Proc) string {
	undeclared := false
	for _, p := range leaks {
		if _, ok := orphanDeclared(p.Args); !ok {
			undeclared = true
		}
	}
	if !undeclared {
		return ""
	}
	return fmt.Sprintf("if one of these is deliberate, declare it: put %s<reason> at the head of the Bash line that starts it, and the load guard's kill arm will spare it (AGENTS.md, \"Checking that a background process actually died\")", LoadOrphanKeepMarker)
}
