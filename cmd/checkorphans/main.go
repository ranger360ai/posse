// Command checkorphans runs internal/posse.SysSelfOrphans and reports what
// it finds. Run it after backgrounding anything, in a LATER Bash call than
// the one that started it, to ask "did that leak" — a question neither
// `jobs -l` nor a %CPU threshold can answer (ranger-base-6mhxw):
//
//	go run ./cmd/checkorphans
//
// `jobs -l` is scoped to the CURRENT shell process, and a gate session's
// Bash tool calls each fork their own (ADR 0009 preamble) — so a job
// backgrounded in an earlier call is already invisible to a later call's
// `jobs -l`, alive or dead. A per-process %CPU threshold is blind the other
// way: a leak that fans out into many low-CPU children sits under any floor
// worth setting. This reads the real process table instead — ppid 1, old
// enough not to be a fork/exec teardown window, argv matched against the
// ADR 0009 gate-shell preamble — the same predicate the load guard's own
// orphan report (ranger-base-apwr) uses, without its CPU floor.
//
// Exit 0: nothing leaked. Exit 1: leaks found, listed on stdout. Exit 2: the
// census itself failed (ps missing, denied, or timed out) — this does not
// fail open the way the load guard does, because a persona asking "did I
// leak" and getting silence back is worse than being told the answer is
// unknown.
package main

import (
	"fmt"
	"os"

	"github.com/ranger360ai/posse/internal/posse"
)

func main() {
	leaks, err := posse.SysSelfOrphans()
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkorphans: %v — could not read the process table, leak status unknown\n", err)
		os.Exit(2)
	}
	if len(leaks) == 0 {
		fmt.Println("checkorphans: clean — nothing of ours orphaned")
		return
	}
	fmt.Println("checkorphans: " + posse.FormatSelfOrphans(leaks))
	if note := posse.SelfOrphanKeepNote(leaks); note != "" {
		fmt.Println("checkorphans: " + note)
	}
	os.Exit(1)
}
