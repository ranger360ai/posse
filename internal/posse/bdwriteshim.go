package posse

// ADR 0073 D2 — the hidden `posse gates bd-write --real <bd> -- <bd argv>`
// subcommand the bd shim's write-verb tail (renderBdWriteTail, gates.go)
// execs into instead of calling bd directly. It is the persona-facing half
// of D1's lock (bdwritelock.go, B1) and D2's own judge-under-the-lock:
//
//  1. take D1's per-store flock, on an fd D2 asks ClearCloseOnExec to keep
//     open past the exec below;
//  2. run ADR 0072's ProjectBdWrite judge against the SAME store, inside
//     that lock — closing the TOCTOU the old separate `gates bd-line` call
//     carried, where the census it read could still change before bd ran;
//  3. syscall.Exec bd, so bd inherits the lock's open file description and
//     the lock is held for exactly as long as bd runs.
//
// This process never returns on success — like RunGatesWrap, it becomes
// bd. It returns only on a refusal (the lock timed out, or the line-cliff
// judge said no — both already written to errOut in their own wording) or
// a usage mistake, which should not happen: this subcommand is rendered by
// the shim, never typed by hand.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// bdWriteSplitArgv parses `--real <bd> -- <bd argv>`: this subcommand's
// own usage, distinct from bd-line's bare `-- <bd argv>` because the shim
// no longer resolves "the real bd" at its own call site — it hands the
// path it already resolved at render time to the one process that is
// about to exec it.
func bdWriteSplitArgv(args []string) (real string, bdArgv []string, ok bool) {
	if len(args) < 2 || args[0] != "--real" || args[1] == "" {
		return "", nil, false
	}
	for i := 2; i < len(args); i++ {
		if args[i] == "--" {
			return args[1], args[i+1:], true
		}
	}
	return "", nil, false
}

// RunGatesBdWrite is `posse gates bd-write --real <bd> -- <bd argv>`.
// refused=true means the write must not run — the shim's exec of this
// subcommand already carries this call's own exit code, and the refusal
// text is already on errOut, so the caller never prints a second message
// over it (the same contract RunGatesBdLine's caller keeps).
func RunGatesBdWrite(args []string, errOut io.Writer) (refused bool, err error) {
	return runGatesBdWrite(args, errOut, BdWriteLockWait)
}

// runGatesBdWrite is RunGatesBdWrite with the lock wait as a parameter —
// bdWriteLock's own reason, applied here: a test proving the lock-timeout
// refusal must not spend BdWriteLockWait's real 30s to do it.
func runGatesBdWrite(args []string, errOut io.Writer, wait time.Duration) (refused bool, err error) {
	real, bdArgv, ok := bdWriteSplitArgv(args)
	if !ok {
		return false, Die("usage: posse gates bd-write --real <bd> -- <bd argv>")
	}
	store := bdWriteStore("")
	lock, lerr := bdWriteLock(store, bdArgv, wait)
	if lerr != nil {
		fmt.Fprintln(errOut, lerr.Error())
		return true, nil
	}
	defer lock.Release()
	// A degraded (unlocked) write is still a write: run the judge and exec
	// bd exactly as the locked path does. lock is nil in that case and
	// ClearCloseOnExec is a no-op on it (bdWriteLock's own "nil, nil means
	// proceed unlocked"). If clearing CLOEXEC fails, nothing more is
	// needed: the fd still carries CLOEXEC, so the exec below closes it
	// the instant bd starts — bd's own run is unlocked, which is the same
	// "nothing about the lock itself is load-bearing" posture bdWriteLock
	// already takes for every other way it can fail to help.
	if cerr := lock.ClearCloseOnExec(); cerr != nil {
		fmt.Fprintf(errOut, "posse: bd write lock: could not keep the lock held across exec: %v — bd's own run is unlocked (ADR 0073)\n", cerr)
	}
	census := filepath.Join(store, beadsJSONL)
	if _, serr := os.Stat(census); serr == nil {
		proj, pok, perr := ProjectBdWrite(census, bdArgv)
		if perr == nil && pok && proj.After > BdLineLimit-BdLineHeadroom {
			line, remedy := bdLineGateRefusal(census, proj)
			fmt.Fprintln(errOut, line)
			fmt.Fprintln(errOut, remedy)
			appendBdLineGateLog(proj)
			return true, nil
		}
	}
	argv := append([]string{real}, bdArgv...)
	return false, syscall.Exec(real, argv, os.Environ())
}
