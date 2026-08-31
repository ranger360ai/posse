// posse — the Ranger work-system harness, herdr-native.
// Sessions are herdr workspaces; work comes from beads (bd); personas, env
// sets, and recipes are posse's own. The tmux-era implementation lives on
// the tmux-reference branch.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

func die(err error) {
	fmt.Fprintf(os.Stderr, "posse: %v\n", err)
	os.Exit(1)
}

// need enforces a subcommand's positional arity, and first settles what a
// leading argument that looks like a flag means (rangerhq-qv5):
//
//	-h / --help   print this subcommand's usage and exit 0 — it used to be
//	              taken as the positional name, so `posse new --help` created
//	              a herdr workspace called '--help'
//	--            ends that reading, so a name that starts with a dash is
//	              still reachable: `posse kill -- --help` kills such a session
//
// It returns the args with the separator removed; callers read positionals
// from the returned slice.
func need(args []string, n int, usage string) []string {
	args, help := argLead(args)
	if help {
		fmt.Fprintf(os.Stdout, "usage: %s\n", usage)
		os.Exit(0)
	}
	if len(args) < n {
		die(posse.Die("usage: %s", usage))
	}
	return args
}

// validCount reads a flag argument that must be a plain non-negative
// count. -n and --timeout were the two flags in dispatch's switch that
// dropped strconv's error (rangerhq-ytkl), and both read 0 as "no limit":
// -n 0 is no cap on a pass (Dispatcher.Run), --timeout 0 waits as long as
// herdr will. So `-n three`, `-n 3x` and `-n ""` all parsed as 0 and
// turned the one flag whose job is to bound a pass into an unbounded one,
// silently. A negative count read the same way — fireLoop caps only on
// max > 0.
//
// prompt --timeout and wait --timeout are the same parser written twice
// more (ranger-base-sknr), and the same 0: Herdr.AgentPrompt/AgentWait
// only pass --timeout on when timeoutMS > 0, so `--timeout soon` and
// `--timeout -1` both asked herdr to wait unbounded.
//
// peek's positional <lines> is the fifth (ranger-base-oz39) and the one
// whose harm runs the other way: PaneRead tails only when lines > 0, so a
// dropped error there read the whole pane instead of the bounded tail
// asked for. All five die on bad input now, and 0 stays the deliberate
// escape hatch.
func validCount(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0
}

// argLead is need's reading of the first argument, without the exits.
func argLead(args []string) (rest []string, help bool) {
	if len(args) == 0 {
		return args, false
	}
	switch args[0] {
	case "--":
		return args[1:], false
	case "-h", "--help":
		return args, true
	}
	return args, false
}

func main() {
	// Second entry point (rangerhq-1k1): this binary is also the argv0
	// launcher a caged pane runs. `state/cages/<persona>/bin/claude` is a
	// symlink to it, and with this flag it execs the engine with argv[0]
	// reset to the runtime's name — the only way herdr identifies an agent
	// through the container boundary. It never returns on success, and it
	// reads no config, so it is the first thing main does.
	if posse.IsCageLaunch(os.Args) {
		die(posse.RunCageLaunch(os.Args))
	}
	// Third entry point (rangerhq-9d0): the egress watcher the launcher
	// forks just before that exec. It outlives the launcher on purpose —
	// the cage's route comes down when the engine's process does — and, like
	// the launcher, it reads no config.
	// Unlike the launcher, this one RETURNS on success — the cage it was
	// watching is over — so it must not go through die(), which exists for
	// a call that only ever comes back to report a failure.
	if posse.IsCageReap(os.Args) {
		if err := posse.RunCageReap(os.Args); err != nil {
			die(err)
		}
		return
	}
	a := posse.NewApp()
	hb := posse.NewHerdrBackend(a)
	args := os.Args[1:]
	cmd := "help"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	out := os.Stdout

	switch cmd {
	case "list", "ls":
		if err := hb.CmdList(out); err != nil {
			die(err)
		}

	case "worktrees":
		// The operability half of "the launcher merges" (rangerhq-09o2): a
		// kill that could not land its work keeps the tree, and a session
		// meta that was pruned takes the only posse-side record of it with
		// it. git still knows, so this asks git.
		dir, land, force := "", false, false
		rest := args
		for len(rest) > 0 {
			switch {
			case rest[0] == "--dir" && len(rest) > 1:
				dir, rest = rest[1], rest[2:]
			case rest[0] == "--land":
				land, rest = true, rest[1:]
			case rest[0] == "--force":
				force, rest = true, rest[1:]
			default:
				die(posse.Die("posse worktrees [--dir <repo>] [--land [--force]]"))
			}
		}
		dirs := a.BeadsDirs()
		if dir != "" {
			dirs = []string{posse.ExpandTilde(dir)}
		}
		fn := posse.ListSessionTrees
		if land {
			fn = func(w io.Writer, dirs []string) error { return posse.LandSessionTrees(w, a, dirs, force) }
		}
		if err := fn(out, dirs); err != nil {
			die(err)
		}

	case "new":
		args = need(args, 1, `posse new <name> [--dir <path>] [--env-file <name>]... [--cmd "..."] [--emoji <e>] [--agent <name>]`)
		o := parseNewFlags(args)
		// ADR 0008: a session the operator made by hand is one they made to
		// talk to — dispatch leaves it alone until they release it.
		o.Crew = true
		if err := hb.CreateSession(o); err != nil {
			die(err)
		}
		fmt.Fprintf(out, "created %s (herdr workspace, background)\n", o.Name)

	case "attach", "up", "local", "focus":
		args = need(args, 1, "posse attach <name>")
		if (cmd == "up" || cmd == "local") && !hb.HasSession(args[0]) {
			if err := hb.CreateSession(posse.NewSessionOpts{Name: args[0], Crew: true}); err != nil {
				die(err)
			}
		}
		if err := hb.FocusSession(args[0]); err != nil {
			die(err)
		}
		fmt.Fprintf(out, "focused %s (in herdr)\n", args[0])

	case "recipe":
		args = need(args, 1, "posse recipe <name>")
		if err := hb.LaunchRecipe(out, args[0]); err != nil {
			die(err)
		}

	case "relaunch":
		// Session refresh: land the plane, close the workspace, recreate it
		// from the same meta (rangerhq-dxq).
		args = need(args, 1, "posse relaunch <name> [--no-land] [--force] [--timeout <interval>]")
		o := posse.RelaunchOpts{Name: args[0]}
		rest := args[1:]
		for len(rest) > 0 {
			switch rest[0] {
			case "--no-land":
				o.NoLand, rest = true, rest[1:]
			case "--force":
				o.Force, rest = true, rest[1:]
			case "--timeout":
				if len(rest) < 2 {
					die(posse.Die("--timeout needs an interval (10m, 90s, or seconds)"))
				}
				iv, err := posse.ParseInterval(rest[1])
				if err != nil {
					die(err)
				}
				o.Timeout, rest = iv, rest[2:]
			default:
				die(posse.Die("unknown flag: %s", rest[0]))
			}
		}
		if err := hb.RelaunchSession(out, o); err != nil {
			die(err)
		}

	case "kill":
		args = need(args, 1, "posse kill <name> [--force] [--foreign] [--no-land] [--timeout <interval>]")
		// A kill is also the moment a session's own worktree is retired:
		// its branch lands on the repo's branch and the tree goes away
		// (rangerhq-09o2). It refuses to remove a tree that still holds
		// work, so the line below is where the operator learns that.
		//
		// And before any of that, the reap guard: a session still holding an
		// open bead over an uncommitted tree is not killed at all (ADR 0013
		// §4). --force is the operator saying they have read the refusal.
		//
		// And the ownership refusal beside it (rangerhq-selx): a workspace
		// this home holds no meta for is another instance's session (or a
		// hand-made herdr one), and a kill by name used to follow Resolve's
		// label fallback straight into closing it. --foreign is the
		// operator saying they mean that row; it is its own flag because
		// --force is about their own session's unfinished work and says
		// nothing about whose session this is.
		//
		// And the landing turn (ranger-base-qxvh): a kill by hand is the
		// path that destroys sessions, and persona memory is the one
		// artifact with no other copy, so this caller takes the turn
		// relaunch takes — bounded, and only when the persona actually has
		// memory no commit holds. --no-land is the operator saying not to
		// spend it, on a wedged session or a sweep of thirty; it does NOT
		// stand down the commit, which is what makes the memory durable and
		// costs one git process. Nor does it stand down the WORKTREE
		// landing below, which was never optional and is a different sense
		// of the word.
		o := posse.KillOpts{Land: true, Out: out}
		rest := args[1:]
		for len(rest) > 0 {
			switch rest[0] {
			case "--force":
				o.Force, rest = true, rest[1:]
			case "--foreign":
				o.Foreign, rest = true, rest[1:]
			case "--no-land":
				o.Land, rest = false, rest[1:]
			case "--timeout":
				if len(rest) < 2 {
					die(posse.Die("--timeout needs an interval (10m, 90s, or seconds)"))
				}
				iv, err := posse.ParseInterval(rest[1])
				if err != nil {
					die(err)
				}
				o.Timeout, rest = iv, rest[2:]
			default:
				die(posse.Die("unknown flag: %s", rest[0]))
			}
		}
		landing, err := hb.KillSessionAndLandOpts(args[0], o)
		if err != nil {
			die(err)
		}
		fmt.Fprintf(out, "killed %s\n", args[0])
		for _, line := range landing.Lines() {
			fmt.Fprintf(out, "  %s\n", line)
		}

	case "prompt":
		// The dispatch primitive: submit work to a session's agent.
		args = need(args, 2, `posse prompt <name> "<text>" [--wait] [--timeout <ms>] [--now]`)
		name, text := args[0], args[1]
		wait, timeout, now := false, 0, false
		rest := args[2:]
		for len(rest) > 0 {
			switch rest[0] {
			case "--wait":
				wait, rest = true, rest[1:]
			case "--now":
				now, rest = true, rest[1:]
			case "--timeout":
				if len(rest) < 2 || !validCount(rest[1]) {
					die(posse.Die("--timeout needs a value in ms (0 = herdr default)"))
				}
				timeout, _ = strconv.Atoi(rest[1])
				rest = rest[2:]
			default:
				die(posse.Die("unknown flag: %s", rest[0]))
			}
		}
		target, err := hb.AgentTarget(name)
		if err != nil {
			die(err)
		}
		// A pane herdr has not recognized yet is a CLI that does not hold
		// the keyboard, and text typed there lands in whatever does — the
		// '/Work' slash command of ranger-base-3p0. Dispatch has waited for
		// a SEEN screen since rangerhq-3hb5; this is the same gate on the
		// hand path (promptready.go). --now is the operator saying they
		// mean this pane as it is.
		if !now {
			_, note, err := hb.AwaitPromptable(name, target)
			if err != nil {
				die(err)
			}
			if note != "" {
				fmt.Fprintf(out, "%s\n", note)
			}
		}
		res, err := hb.H.AgentPrompt(target, text, wait, timeout)
		if err != nil {
			die(err)
		}
		// The operator starting a conversation makes the session crew; a
		// persona's prompt marks nothing (ADR 0008). After the prompt took,
		// so a failed prompt is not a conversation. A mark that was owed and
		// did not land is a warning, not a silence (rangerhq-sk6p).
		if missed := hb.MarkCrewOnOperatorPrompt(name); missed != "" {
			fmt.Fprintf(out, "warning: %s\n", missed)
		}
		// A hand-launched session (`posse new` + `posse prompt`, never
		// through dispatch's own launchSession) has no bead: pointer
		// unless this stamps it — autoReapPass skips s.Bead=="" forever
		// (ranger-base-v674). Only a work-prompt-shaped text matches.
		hb.NoteBeadFromPrompt(name, text)
		fmt.Fprintf(out, "%s\n", res)

	case "crew":
		// ADR 0008: hand a session to the operator (dispatch skips it) or
		// give it back to the fleet.
		args = need(args, 1, "posse crew <name> [--off]")
		crew := true
		for _, a := range args[1:] {
			if a != "--off" {
				die(posse.Die("unknown flag: %s", a))
			}
			crew = false
		}
		if err := hb.SetCrew(args[0], crew); err != nil {
			die(err)
		}
		if crew {
			fmt.Fprintf(out, "%s is crew (yours) — dispatch skips it\n", args[0])
		} else {
			fmt.Fprintf(out, "%s is fleet — dispatch may use it\n", args[0])
		}

	case "wait":
		args = need(args, 1, "posse wait <name> [--until <state>]... [--timeout <ms>]")
		name := args[0]
		var until []string
		timeout := 0
		rest := args[1:]
		for len(rest) > 0 {
			switch rest[0] {
			case "--until":
				if len(rest) < 2 {
					die(posse.Die("--until needs a state"))
				}
				until = append(until, rest[1])
				rest = rest[2:]
			case "--timeout":
				if len(rest) < 2 || !validCount(rest[1]) {
					die(posse.Die("--timeout needs a value in ms (0 = herdr default)"))
				}
				timeout, _ = strconv.Atoi(rest[1])
				rest = rest[2:]
			default:
				die(posse.Die("unknown flag: %s", rest[0]))
			}
		}
		target, err := hb.AgentTarget(name)
		if err != nil {
			die(err)
		}
		res, err := hb.H.AgentWait(target, until, timeout)
		if err != nil {
			die(err)
		}
		fmt.Fprintf(out, "%s\n", res)

	case "peek":
		args = need(args, 1, "posse peek <name> [<lines>]")
		// ranger-base-oz39: the third site of ytkl/sknr's dropped Atoi,
		// with the harm inverted. 0 lines means the WHOLE pane (PaneRead
		// tails only when lines > 0), so `posse peek sess 40x` did not
		// under-read — it read everything, silently, where the operator
		// asked for a bounded tail. 0 stays the deliberate escape hatch,
		// as it is for -n and --timeout. Before Resolve, so the argument
		// is named whether or not the session exists.
		lines := 0
		if len(args) > 1 {
			if !validCount(args[1]) {
				die(posse.Die("peek <lines> needs a count (0 = whole pane)"))
			}
			lines, _ = strconv.Atoi(args[1])
		}
		s, err := hb.Resolve(args[0])
		if err != nil {
			die(err)
		}
		if s.PaneID == "" {
			die(posse.Die("session %s has no recorded pane (created outside posse)", args[0]))
		}
		text, err := hb.H.PaneRead(s.PaneID, lines)
		if err != nil {
			die(err)
		}
		fmt.Fprintln(out, text)

	case "ready":
		// Head of the dispatch loop: unblocked work. With --dir, one repo;
		// without, aggregated across the config `beads:` list (else cwd).
		dir, assignee := "", ""
		rest := args
		for len(rest) > 0 {
			switch rest[0] {
			case "--dir":
				if len(rest) < 2 {
					die(posse.Die("--dir needs a path"))
				}
				dir = posse.ExpandTilde(rest[1])
				rest = rest[2:]
			case "--assignee", "--as":
				if len(rest) < 2 {
					die(posse.Die("%s needs a name", rest[0]))
				}
				assignee = rest[1]
				rest = rest[2:]
			default:
				die(posse.Die("unknown flag: %s", rest[0]))
			}
		}
		bd := needBd()
		// verify-after (ADR 0006 §3): the same rule the dispatch pass runs,
		// so an operator looking at ready work sees the verify beads the
		// last round of closes earned.
		verifyDirs := a.BeadsDirs()
		if dir != "" {
			verifyDirs = []string{dir}
		}
		a.VerifyAfter(bd, verifyDirs, out, os.Stderr)
		var issues []posse.RepoIssue
		if dir != "" {
			single, err := bd.Ready(dir, assignee)
			if err != nil {
				die(err)
			}
			for _, is := range single {
				issues = append(issues, posse.RepoIssue{BdIssue: is, Dir: dir})
			}
		} else {
			var failed []error
			issues, failed = bd.ReadyAll(a, assignee)
			// A repo the scan could not read has an unknown queue, not an
			// empty one (rangerhq-llse) — say which, and never print "no
			// ready work" when the scan is why the list is empty.
			for _, err := range failed {
				fmt.Fprintf(os.Stderr, "ready scan failed: %v\n", err)
			}
			if len(issues) == 0 && len(failed) > 0 {
				die(posse.Die("ready scan failed in all %d beads repo(s) — the queue is unknown, not empty", len(failed)))
			}
		}
		// One queue, one order — priority first, across every source
		// (ranger-base-xotg). ReadyAll already hands back an ordered list;
		// this covers --dir, where bd's own order is the query's.
		posse.OrderBeads(issues, false)
		if len(issues) == 0 {
			fmt.Fprintln(out, "no ready work")
			break
		}
		for _, is := range issues {
			who := is.Assignee
			if who == "" {
				who = "unassigned"
			}
			fmt.Fprintf(out, "%-14s p%d  %-12s %-40s %s\n", is.ID, is.Priority, who, is.Title, posse.AbbrevHome(is.Dir))
		}

	case "beads":
		// posse beads check — two independent alarms under one verb
		// (ranger-base-z3s3), each over its own store:
		//
		//   - the bead-loss census (rangerhq-fuom): bd's auto-import
		//     deletes rows from the database on a git-history signal and
		//     logs nothing when it does, so the git census of
		//     .beads/issues.jsonl is the only witness. --record moves what
		//     it finds into .beads/deleted.jsonl, which is the record a
		//     deletion owes and the last copy of the bead itself.
		//   - the pair check (NOTES.md, ranger-base-pkqn): a live read of
		//     the dependency graph in beads.db for a symmetric same-type
		//     pair, the bd 0.49.1 `dep add` landmine that
		//     scripts/verify-bd-dep-safety.sh --gate already answers with
		//     no caller wired up. --record does not touch this one — a
		//     pair is not a deletion, it is pruned instead
		//     (scripts/prune-bd-relates-to.sh).
		//
		// Neither answers the other's question, so neither folds into the
		// other's finding count; both still gate the same exit code, since
		// both are things CI must fail on.
		args = need(args, 1, "posse beads check [--dir <repo>] [--record \"<reason>\"] [--as <who>]")
		if args[0] != "check" {
			die(posse.Die("usage: posse beads check [--dir <repo>] [--record \"<reason>\"] [--as <who>]"))
		}
		dir, reason, who := "", "", os.Getenv("BD_ACTOR")
		rest := args[1:]
		for len(rest) > 0 {
			if len(rest) < 2 {
				die(posse.Die("%s needs a value", rest[0]))
			}
			switch rest[0] {
			case "--dir":
				dir = posse.ExpandTilde(rest[1])
			case "--record":
				reason = rest[1]
			case "--as":
				who = rest[1]
			default:
				die(posse.Die("unknown flag: %s", rest[0]))
			}
			rest = rest[2:]
		}
		bd := needBd()
		dirs := a.BeadsDirs()
		if dir != "" {
			dirs = []string{dir}
		}
		// A configured repo that is not there has an UNKNOWN census, not a
		// clean one (ranger-base-vlrp). The git walk is deliberately quiet
		// where a repo has no census, so a path that does not resolve at all
		// reads exactly like a healthy check — the same shape as the ready
		// scan folding a missing repo into an empty queue (rangerhq-llse).
		unresolved := posse.UnresolvedDirs(dirs)
		for _, err := range unresolved {
			fmt.Fprintf(os.Stderr, "beads census failed: %v\n", err)
		}
		found := 0
		pairsFound := 0
		var pairUnavailable []error
		for _, d := range dirs {
			lost, err := posse.LostBeads(bd, d)
			if err != nil {
				die(err)
			}
			if len(lost) > 0 {
				found += len(lost)
				for _, lb := range lost {
					fmt.Fprintf(out, "%-14s %-12s %-10s dropped %s by %s  %s\n",
						lb.ID, lb.Status, lb.Assignee,
						lb.When.Format("2006-01-02 15:04"), lb.Commit[:min(8, len(lb.Commit))],
						posse.AbbrevHome(d))
					fmt.Fprintf(out, "               %s\n", lb.Title)
				}
				if reason != "" {
					if who == "" {
						who = "operator"
					}
					if err := posse.RecordDeletions(d, reason, who, lost, time.Now()); err != nil {
						die(err)
					}
					// Not necessarily under d: a .beads/redirect puts the
					// ledger in the repo whose git tracks it.
					fmt.Fprintf(out, "recorded %d deletion(s) in %s — commit it\n", len(lost), posse.AbbrevHome(posse.DeletionLedgerPath(d)))
				}
			}

			// The pair check reads beads.db directly rather than git, so a
			// repo LostBeads read fine can still fail here (a WAL-mode db
			// with no live writer and no -shm refuses even a read-only
			// open) — that failure must read as unknown, never as clean
			// (PairCheckUnavailableError, ranger-base-z3s3).
			pairs, err := posse.PairCheck(d)
			if err != nil {
				pairUnavailable = append(pairUnavailable, err)
				fmt.Fprintf(os.Stderr, "pair check failed: %v\n", err)
				continue
			}
			if len(pairs) > 0 {
				pairsFound += len(pairs)
				ids := make([]string, len(pairs))
				for i, p := range pairs {
					ids[i] = p.ID
				}
				fmt.Fprintf(out, "PAIR: %d node(s) sit in a symmetric dependency pair in %s: %s\n",
					len(pairs), posse.AbbrevHome(d), strings.Join(ids, ", "))
			}
		}
		if pairsFound > 0 {
			fmt.Fprintln(out, "  'bd dep add' / 'bd create --deps' onto anything upstream of one of those never returns.")
			fmt.Fprintln(out, "  Prune: scripts/prune-bd-relates-to.sh --apply, then `make verify-bd-no-relate-pairs` to confirm.")
		}
		if found == 0 && len(unresolved) == 0 && pairsFound == 0 && len(pairUnavailable) == 0 {
			fmt.Fprintln(out, "no lost beads: every id git ever carried still resolves")
			fmt.Fprintln(out, "no symmetric dependency pair in the live graph")
			break
		}
		if found == 0 {
			fmt.Fprintf(out, "no lost beads in the %d repo(s) that resolved — %d configured path(s) are not there, so that census is unknown, not clean\n",
				len(dirs)-len(unresolved), len(unresolved))
		}
		switch {
		case len(pairUnavailable) > 0:
			fmt.Fprintf(out, "pair check unknown in %d repo(s) — the live graph could not be read, so that check is unknown, not clean\n", len(pairUnavailable))
		case pairsFound == 0:
			fmt.Fprintln(out, "no symmetric dependency pair in the live graph")
		}
		if reason == "" || len(unresolved) > 0 || pairsFound > 0 || len(pairUnavailable) > 0 {
			// Non-zero so an instance repo can run this in CI, the way
			// `posse agent check` reports PID findings. A census that could
			// not be taken everywhere is not an all-clear either, --record
			// or not: --record owns the losses it found, not the ones it
			// could not go looking for. A pair finding is not owned by
			// --record at all — it has no lost-bead shape for the ledger to
			// carry — so it gates the exit code unconditionally.
			os.Exit(1)
		}

	case "claim", "done":
		// posse claim <id> [--as <persona>] [--dir <repo>] — atomic claim;
		// posse done <id> ... — close. --as sets the bd actor (persona name).
		args = need(args, 1, "posse "+cmd+" <id> [--as <persona>] [--dir <repo>]")
		id := args[0]
		dir, actor := "", ""
		rest := args[1:]
		for len(rest) > 0 {
			switch rest[0] {
			case "--dir":
				if len(rest) < 2 {
					die(posse.Die("--dir needs a path"))
				}
				dir = posse.ExpandTilde(rest[1])
				rest = rest[2:]
			case "--as":
				if len(rest) < 2 {
					die(posse.Die("--as needs a persona name"))
				}
				actor = rest[1]
				rest = rest[2:]
			default:
				die(posse.Die("unknown flag: %s", rest[0]))
			}
		}
		bd := needBd()
		var err error
		verb := "closed"
		if cmd == "claim" {
			var resumed bool
			resumed, err = bd.Claim(dir, id, actor)
			verb = "claimed"
			if resumed {
				verb = "resumed"
			}
		} else {
			err = bd.Close(dir, id, actor)
		}
		if err != nil {
			die(err)
		}
		fmt.Fprintf(out, "%s %s\n", verb, id)

	case "dispatch":
		// One pass of the harness core: route ready beads to personas.
		d := posse.NewDispatcher(a, hb, out)
		dirF, personaF, maxN := "", "", 0
		var watch, watchMax time.Duration
		watchStatus := false
		rest := args
		for len(rest) > 0 {
			switch rest[0] {
			case "--watch-status":
				watchStatus = true
				rest = rest[1:]
			case "--dry-run":
				d.DryRun = true
				rest = rest[1:]
			case "--resume":
				d.Resume = true
				rest = rest[1:]
			case "--runtime":
				if len(rest) < 2 {
					die(posse.Die("--runtime needs a name (claude, codex, grok, or runtimes/<name>.yaml)"))
				}
				if _, err := a.LoadRuntime(rest[1]); err != nil {
					die(err)
				}
				d.Runtime = rest[1]
				rest = rest[2:]
			case "--tier":
				if len(rest) < 2 || !posse.ValidTier(rest[1]) {
					die(posse.Die("--tier needs strong, standard, or fast"))
				}
				d.Tier = rest[1]
				rest = rest[2:]
			case "--allow-degraded":
				d.AllowDegraded = true
				rest = rest[1:]
			case "--no-reap":
				d.NoReap = true
				rest = rest[1:]
			case "--cage":
				if len(rest) < 2 || !posse.ValidCage(rest[1]) {
					die(posse.Die("--cage needs shims, seatbelt, or container"))
				}
				d.Cage = rest[1]
				rest = rest[2:]
			case "--watch":
				if len(rest) < 2 {
					die(posse.Die("--watch needs an interval (30s, 2m, or seconds)"))
				}
				iv, err := posse.ParseInterval(rest[1])
				if err != nil {
					die(err)
				}
				watch = iv
				rest = rest[2:]
			case "--max-interval":
				if len(rest) < 2 {
					die(posse.Die("--max-interval needs an interval"))
				}
				iv, err := posse.ParseInterval(rest[1])
				if err != nil {
					die(err)
				}
				watchMax = iv
				rest = rest[2:]
			case "--dir":
				if len(rest) < 2 {
					die(posse.Die("--dir needs a path"))
				}
				dirF = posse.ExpandTilde(rest[1])
				rest = rest[2:]
			case "--persona":
				if len(rest) < 2 {
					die(posse.Die("--persona needs a name"))
				}
				personaF = rest[1]
				rest = rest[2:]
			case "-n":
				if len(rest) < 2 || !validCount(rest[1]) {
					die(posse.Die("-n needs a count (0 = no cap)"))
				}
				maxN, _ = strconv.Atoi(rest[1])
				rest = rest[2:]
			case "--timeout":
				if len(rest) < 2 || !validCount(rest[1]) {
					die(posse.Die("--timeout needs a value in ms (0 = herdr default)"))
				}
				d.PromptWaitMS, _ = strconv.Atoi(rest[1])
				rest = rest[2:]
			case "--ceiling":
				if len(rest) < 2 {
					die(posse.Die("--ceiling needs an interval (2h, 90m, or seconds)"))
				}
				iv, err := posse.ParseInterval(rest[1])
				if err != nil {
					die(err)
				}
				d.WaitCeiling = iv
				rest = rest[2:]
			default:
				die(posse.Die("unknown flag: %s", rest[0]))
			}
		}
		if watchStatus {
			// The liveness question, asked of the kernel and answered on
			// one line (rangerhq-gir5). Reads no bd, talks to no herdr, and
			// launches nothing — plugin/autostart.sh runs it at every herdr
			// server start, before the fleet is up.
			line, err := posse.WatchStatus(a)
			if err != nil {
				die(err)
			}
			fmt.Fprintln(out, line)
			return
		}
		if !d.Bd.Available() {
			die(posse.Die("bd not found in PATH"))
		}
		if watch > 0 {
			// Continuous passes with quiet-pass backoff; SIGINT/SIGTERM end
			// the loop between passes (a pass in flight finishes first).
			if watchMax == 0 {
				watchMax = 8 * watch
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			passes, err := d.Watch(ctx, dirF, personaF, maxN, watch, watchMax)
			stop()
			if err != nil {
				die(err)
			}
			fmt.Fprintf(out, "watch stopped after %d pass(es)\n", passes)
			return
		}
		n, err := d.Run(dirF, personaF, maxN)
		if err != nil {
			die(err)
		}
		verb := "dispatched"
		if d.DryRun {
			// Not "routable": since ADR 0020 §2 a dry pass walks the seats
			// and benches each one it seats, so the count is what a real
			// pass would fire — a lane's fourth bead is not in it, and
			// calling that number routable would overstate the queue.
			verb = "would be dispatched"
		}
		fmt.Fprintf(out, "%d bead(s) %s\n", n, verb)

	case "status":
		// The governance surface as a command (ADR 0029 §2, bead
		// rangerhq-81y0): the same
		// computation the pulse tick and the cockpit's GOVERNANCE block
		// render, printed once and answered with an exit code.
		//
		// It reads the stores DIRECTLY and depends on no loop — a dead watch
		// loop is a condition it reports (G7), never a reason it goes quiet.
		// Non-zero means either "something needs a human" or "the set could
		// not be read": an unreadable store is not an all-clear, the same
		// rule `posse beads check` keeps.
		need(args, 0, "posse status")
		set, failed := posse.ShopCheck(posse.StatusInputs(a, hb, os.Stderr))
		fmt.Fprintf(out, "shop check · %s · %s\n", posse.GovSummary(set), posse.AbbrevHome(a.Home))
		posse.GovReport(out, set, failed)
		if len(set) > 0 || len(failed) > 0 {
			os.Exit(1)
		}

	case "pause":
		// PAUSE (ADR 0029 §3, bead rangerhq-a2g6): stop dispatching until
		// told otherwise. The why is mandatory — it is what every declining
		// pass prints, and the file shape is what makes "pauses with a
		// recorded why: 100%" a metric rather than a hope.
		//
		// Arguments are joined, so both `posse pause "why"` and an unquoted
		// why work. A stop is typed in a hurry, and a shell quoting slip is
		// not a reason for the shop to keep spending.
		args = need(args, 1, `posse pause "<why>"   (the why is mandatory: every declining pass prints it)`)
		by, err := posse.PauseActor(a)
		if err != nil {
			die(err)
		}
		// A second pause keeps the first. Overwriting would move `at:`
		// forward and lose the reason the shop actually stopped for, and the
		// intent — dispatch stops — is already in force.
		if p := posse.ReadPause(posse.PausePath(a)); p.Present {
			fmt.Fprintf(out, "already %s — the standing pause is kept (`posse resume` first to change it)\n", posse.PauseLine(p))
			break
		}
		p, err := posse.WritePause(a, by, strings.Join(args, " "), time.Now(), os.Stderr)
		if err != nil {
			die(err)
		}
		fmt.Fprintf(out, "%s\n", posse.PauseLine(p))
		fmt.Fprintf(out, "dispatch declines every pass until `posse resume`; the pulse keeps ticking — a paused shop still escalates\n")

	case "resume":
		// The other half. Idempotent: resuming a shop that is not paused is
		// not an error, because an off switch that can fail is one more thing
		// to get right while the shop is stopped.
		need(args, 0, "posse resume")
		by, err := posse.PauseActor(a)
		if err != nil {
			die(err)
		}
		p, err := posse.ClearPause(a)
		if err != nil {
			die(err)
		}
		if !p.Present {
			fmt.Fprintln(out, "not paused — nothing to resume")
			break
		}
		fmt.Fprintf(out, "resumed by %s · lifted a pause%s; the next pass dispatches\n", by, posse.PauseClause(p))

	case "cockpit":
		if err := runCockpit(a, hb, out); err != nil {
			die(err)
		}

	case "envs":
		for _, n := range a.ListEnvSets() {
			vars, _ := a.EnvSetVars(n)
			keys := make([]string, 0, len(vars))
			for _, v := range vars {
				keys = append(keys, v.Key) // names only — never values
			}
			fmt.Fprintf(out, "%s  (%s)\n", n, joinComma(keys))
		}

	case "env":
		// Env-set management: files edited in $EDITOR, never echoed.
		args = need(args, 2, "posse env <edit|rm> <name>   (edit creates if missing)")
		sub, name := args[0], args[1]
		switch sub {
		case "edit", "new":
			if !posse.ValidName(name) {
				die(posse.Die("bad env set name '%s'", name))
			}
			p, err := a.EnsureEnvSet(name)
			if err != nil {
				die(err)
			}
			if err := execEditor(p); err != nil {
				die(err)
			}
		case "rm", "delete":
			if err := a.DeleteEnvSet(name); err != nil {
				die(err)
			}
			fmt.Fprintf(out, "deleted env set %s\n", name)
		default:
			die(posse.Die("usage: posse env <edit|rm> <name>"))
		}

	case "memory", "orders":
		// Persona-private memory: open the persona's ORDERS.md in $EDITOR.
		args = need(args, 1, "posse memory <persona>")
		ag, err := a.LoadAgent(args[0])
		if err != nil {
			die(err)
		}
		if err := ag.EnsureMemoryDir(); err != nil {
			die(err)
		}
		if err := execEditor(ag.MemoryDir + "/ORDERS.md"); err != nil {
			die(err)
		}

	case "agent":
		// Persona files: `agent new` scaffolds the PID shape (ADR 0001) and
		// opens it in $EDITOR; `agent edit` opens an existing one.
		args = need(args, 1, "posse agent <new|edit|check> <name>   (check: --all lints every persona)")
		sub := args[0]
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		switch sub {
		case "check":
			// Lint PIDs against the ADR 0001 contract; non-zero on findings
			// so an instance repo can run it in CI.
			names := []string{name}
			if name == "" || name == "--all" {
				names = a.ListAgents()
			}
			findings := 0
			for _, n := range names {
				fs, ws, err := a.CheckAgent(n)
				if err != nil {
					die(err)
				}
				for _, f := range fs {
					fmt.Fprintf(out, "%s: %s\n", n, f)
				}
				for _, w := range ws {
					fmt.Fprintf(out, "%s: warning: %s\n", n, w)
				}
				findings += len(fs)
			}
			if findings > 0 {
				fmt.Fprintf(out, "%d finding(s) in %d persona(s)\n", findings, len(names))
				os.Exit(1)
			}
			fmt.Fprintf(out, "%d persona(s) match the PID contract\n", len(names))
		case "new", "edit":
			args = need(args, 2, "posse agent <new|edit> <name>")
			var p string
			var err error
			if sub == "new" {
				p, err = a.ScaffoldAgent(name)
			} else {
				var ag *posse.AgentFile
				ag, err = a.LoadAgent(name)
				if ag != nil {
					p = ag.Path
				}
			}
			if err != nil {
				die(err)
			}
			if err := execEditor(p); err != nil {
				die(err)
			}
		default:
			die(posse.Die("usage: posse agent <new|edit|check> <name>"))
		}

	case "cost":
		// API-equivalent spend per bead from every registered cost provider's
		// transcripts (ADR 0003 §4, ADR 0012 D4) — read-only; a runtime with
		// no adapter is reported as uncounted, never as zero.
		o, err := parseCostFlags(args)
		if err != nil {
			die(err)
		}
		if o.plan {
			// The windows on their own (rangerhq-p3z): the reading a fleet
			// persona or a guard actually wants, without the transcript
			// scan that the rest of this command is. Same shared snapshot
			// and the same TTL as the footer below — asking this way costs
			// the endpoint nothing extra.
			//
			// Unlike the footer, this one is not allowed to be silent: the
			// reading IS the output, so an unreadable one is a failed
			// command, not an empty line. The errors are generic by
			// construction (planusage.go) — they never quote the token.
			line, err := a.PlanCache("cost").Line(a.PlanUsageTTL(os.Stderr))
			if err != nil {
				die(err)
			}
			fmt.Fprintln(out, line)
			break
		}
		rep := posse.ScanCosts(o.project, o.since)
		// Dial E's caps, for the footer: what the numbers above are measured
		// against (rangerhq-25p). Reading them here never enforces anything.
		rep.PassCap, rep.DayCap = a.BudgetCaps(os.Stderr)
		if bd := posse.NewBd(); bd.Available() {
			rep.AttributePersonas(a, bd)
		}
		rep.CountUncounted(hb)
		rep.Print(out)
		// The plan's own rate windows (rangerhq-jgm) — the constraint the
		// dollars above are a proxy for. Current reading only, no history;
		// silent when the credential or endpoint is unreadable.
		// Through the shared cache (rangerhq-tdy8) — `posse cost` in a loop
		// is one of the three pollers that made the endpoint 429. A reading
		// a few minutes old is still the reading; say its age when it has
		// one, so the number is never presented as newer than it is.
		if line, err := a.PlanCache("cost").Line(a.PlanUsageTTL(os.Stderr)); err == nil {
			fmt.Fprintf(out, "%s (the plan's own rate limits — the real budget; dollars above are API-equivalent)\n", line)
		}

	case "scorecard":
		// Per-persona outcome metrics from bd data — read-only.
		if len(args) >= 1 && args[0] == "--catalog" {
			// The derived metric catalog (ADR 0001 amendment): a vocabulary
			// check over the PIDs, so it needs no bd.
			if err := a.MetricCatalogReport(out); err != nil {
				die(err)
			}
			break
		}
		persona := ""
		if len(args) >= 2 && args[0] == "--persona" {
			persona = args[1]
		} else if len(args) == 1 {
			persona = args[0]
		}
		if !posse.NewBd().Available() {
			die(posse.Die("bd not found in PATH"))
		}
		if err := a.Scorecard(posse.NewBd(), out, persona, time.Now()); err != nil {
			die(err)
		}

	case "agents":
		for _, n := range a.ListAgents() {
			if ag, err := a.LoadAgent(n); err == nil {
				// The tier half is the display tier (ADR 0013 §6): this
				// listing is where an operator reads what a PID runs at,
				// and `[grok/strong]` claimed a mapping grok does not have.
				rn := a.ResolveRuntime("", ag)
				fmt.Fprintf(out, "🎭 %s  %s  [%s/%s]\n", n, ag.Description, rn, a.DisplayTier(rn, a.ResolveTier("", ag)))
			}
		}

	case "gates":
		// Inspect a persona's L1 gates (shims rendered from its deny: and
		// the refusals log — state, not memory), or install the L3 hook.
		args = need(args, 1, "posse gates <persona> | posse gates install-hooks [dir] [--chain] | posse gates wrap <persona> -- <cmd>")
		// The inner command of a container launch (ADR 0002 §3,
		// rangerhq-6so): rendered onto the engine's line by the host and run
		// by the image's own Linux posse, never typed by hand. It renders
		// gates/<persona>/ against the image's PATH and shell and becomes the
		// runtime behind them, so on success it does not return.
		if args[0] == "wrap" {
			// Not `die(RunGatesWrap(…))`: die() exits whatever it is handed,
			// and --probe returns nil on purpose — that nil is the answer the
			// host's parity check reads.
			if err := posse.RunGatesWrap(args[1:], out); err != nil {
				die(err)
			}
			return
		}
		if args[0] == "install-hooks" {
			chain := false
			dir := "."
			for _, a2 := range args[1:] {
				if a2 == "--chain" {
					chain = true
					continue
				}
				dir = posse.ExpandTilde(a2)
			}
			// Both slots are attempted and reported independently — a
			// foreign hook taking one must not cost the other
			// (rangerhq-mgdk; the comment used to claim this and only
			// InstallCommitGuardHook below actually did it, because the
			// pre-push failure died before it was ever reached).
			failed := false
			var p string
			var err error
			if chain {
				p, err = posse.InstallPrePushHookChained(dir)
			} else {
				p, err = posse.InstallPrePushHook(dir)
			}
			if err != nil {
				fmt.Fprintf(out, "not installed: pre-push — %v\n", err)
				failed = true
			} else {
				fmt.Fprintf(out, "installed %s (refuses git push when RHQ_TOOLS_DENY matches; foreign hooks are never overwritten)\n", posse.AbbrevHome(p))
			}
			var c, vis, src string
			var cerr error
			if chain {
				c, vis, src, cerr = a.InstallCommitGuardHookChained(dir)
			} else {
				c, vis, src, cerr = a.InstallCommitGuardHook(dir)
			}
			if cerr != nil {
				fmt.Fprintf(out, "not installed: prepare-commit-msg — %v\n", cerr)
				failed = true
			} else {
				fmt.Fprintf(out, "installed %s (refuses an unqualified git commit from any shell in this checkout — the index is shared; rangerhq-lmq9, rangerhq-lt2w)\n", posse.AbbrevHome(c))
				// The same slot carries the beads visibility guard, and its
				// verdict is stamped into the file — so say which one was
				// stamped and where it came from, or an operator has to read a
				// hook to find out whether their db is guarded (rangerhq-hrz).
				fmt.Fprintf(out, "  beads visibility guard: %s — %s\n", vis, src)
				if vis == posse.VisibilityPublic {
					fmt.Fprintf(out, "  refuses ops-class content added to %s/.beads/*.jsonl (NOTES.md, Privacy model)\n", posse.AbbrevHome(dir))
				}
				// What THIS instance added to the pattern list, and what it
				// asked for and did not get. A refused pattern is said out
				// loud here and in the hook file: an operator who believes a
				// client name is guarded and finds out at disclosure time is
				// worse off than one who never added it (ranger-base-4rbs).
				set := a.OpsPatternSet()
				if n := len(set.Extra); n > 0 {
					var classes []string
					for _, p := range set.Extra {
						classes = append(classes, p.Class)
					}
					fmt.Fprintf(out, "  instance patterns stamped in (config %s:): %s\n", posse.OpsPatternsConfigKey, strings.Join(classes, ", "))
				}
				for _, r := range set.Rejected {
					fmt.Fprintf(out, "  instance pattern REFUSED, not in force: %s\n", r)
				}
			}
			if failed {
				os.Exit(1)
			}
			return
		}
		ag, err := a.LoadAgent(args[0])
		if err != nil {
			die(err)
		}
		gatesDir, binDir, gateShell, err := a.RenderGates(ag.Name, ag.Deny)
		if err != nil {
			die(err)
		}
		tier := a.ResolveTier("", ag)
		// The matrix is read here for a launch *somewhere*, so it is computed
		// for the cwd: that is the only directory this command knows, and the
		// one part of the check that depends on a directory (a runtime that
		// reads the session dir's own config) is invisible otherwise.
		cwd, _ := os.Getwd()
		fmt.Fprintf(out, "parity (ADR 0002 §4, ADR 0003 §3) — what the wall realizes per runtime at cage shims, tier %s, launching in %s:\n", tier, posse.AbbrevHome(cwd))
		for _, rn := range a.ListRuntimes() {
			if rt, err := a.LoadRuntime(rn); err == nil {
				// Every other input to a launch is on this machine; this one
				// is the ACCOUNT's (rangerhq-oay). It leads the runtime's
				// block because it is a property of the runtime, not of a
				// cage tier — and it is here so the operator can tell "the
				// strong model is gone" from "the probe never answers on this
				// box" without launching anything.
				if line := a.PreflightReport(ag.Name, rn, tier, os.Stderr); line != "" {
					fmt.Fprintf(out, "  %s\n", line)
				}
				fmt.Fprint(out, "  "+a.CheckParityIn(ag, rt, posse.DefaultCage, tier, cwd).String())
				if posse.AvailableCages[posse.CageSeatbelt] {
					fmt.Fprint(out, "  "+a.CheckParityIn(ag, rt, posse.CageSeatbelt, tier, cwd).String())
				}
			}
		}
		fmt.Fprintf(out, "%s\n", posse.AbbrevHome(gatesDir))
		fmt.Fprintf(out, "  gate shell %s (typed as SHELL/GROK_SHELL — ADR 0009)\n", posse.AbbrevHome(gateShell))
		if posse.ResolveCage("", ag) == posse.CageSeatbelt && posse.AvailableCages[posse.CageSeatbelt] {
			// Said out loud rather than swallowed: this block is where an
			// operator reads ADR 0015 §2's wall off the output, and a
			// report that silently prints nothing is the failure it exists
			// to prevent.
			// The persona's own runtime declares where its CLI keeps state
			// (state_dir:, ADR 0012 D4) — a report that left it out would
			// print a writable set the launch does not use.
			var stateDirs []string
			if rt, err := a.LoadRuntime(a.ResolveRuntime("", ag)); err == nil {
				stateDirs = rt.StateDirs
			}
			if err := a.SeatbeltReport(ag, cwd, out, stateDirs...); err != nil {
				fmt.Fprintf(out, "  seatbelt profile not rendered: %v\n", err)
			}
		}
		rules := posse.ParseShimRules(ag.Deny)
		if len(rules) == 0 {
			fmt.Fprintln(out, "  no shell-verb denies → no shims (Edit/Write/WebFetch-class denies are other layers')")
		}
		ents, _ := os.ReadDir(binDir)
		for _, e := range ents {
			var rs []string
			for _, r := range rules[e.Name()] {
				rs = append(rs, r.Rule)
			}
			fmt.Fprintf(out, "  bin/%-12s %s\n", e.Name(), strings.Join(rs, ", "))
		}
		if b, err := os.ReadFile(gatesDir + "/refusals.log"); err == nil && len(b) > 0 {
			lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
			if len(lines) > 10 {
				lines = lines[len(lines)-10:]
			}
			fmt.Fprintf(out, "  refusals.log (last %d):\n", len(lines))
			for _, ln := range lines {
				fmt.Fprintf(out, "    %s\n", ln)
			}
		} else {
			fmt.Fprintln(out, "  refusals.log: empty")
		}

	case "cage":
		// The L4 tier's own surface (ADR 0002 §3, rangerhq-9fv): which engine
		// and image the host would launch with, how to build the image, and
		// — for a persona — the exact line a caged launch would type.
		if len(args) > 0 && args[0] == "down" {
			// The way back from a watcher that died with its pane
			// (rangerhq-9d0): every rendered launch plan for this persona,
			// its route taken down. Safe when nothing is up.
			if len(args) < 2 {
				die(posse.Die("usage: posse cage down <persona>"))
			}
			n, err := a.TearDownCageEgress(args[1], out)
			if err != nil {
				die(err)
			}
			if n == 0 {
				fmt.Fprintf(out, "no rendered cage launch for %s — nothing to take down\n", args[1])
			}
			return
		}
		if len(args) > 0 && args[0] == "build" {
			src, runtimes := ".", ""
			for i := 1; i < len(args); i++ {
				switch args[i] {
				case "--runtimes":
					if i+1 >= len(args) {
						die(posse.Die("--runtimes needs an npm package list, e.g. \"@anthropic-ai/claude-code @openai/codex\""))
					}
					i++
					runtimes = args[i]
				default:
					src = posse.ExpandTilde(args[i])
				}
			}
			if abs, err := filepath.Abs(src); err == nil {
				src = abs
			}
			if err := a.BuildCageImage(src, runtimes, os.Stdout); err != nil {
				die(err)
			}
			return
		}
		engine := a.ResolveEngine()
		e, err := a.LoadEngine(engine)
		if err != nil {
			die(err)
		}
		image := a.CageImage()
		state := "engine binary not on PATH — cage: container is unavailable on this host"
		switch {
		case a.ContainerAvailable() && a.CageImageBuilt(e, image):
			state = "ready"
		case a.ContainerAvailable():
			state = "image not built — run `posse cage build`"
		}
		fmt.Fprintf(out, "engine %s (%s) · image %s · %s\n", e.Name, e.Binary(), image, state)
		fmt.Fprintf(out, "  %s\n", e.Command)
		// How OLD the image is, before anything is said about what it can
		// do (ranger-base-nwj7). The L1/L3 render inside the cage is the
		// IMAGE's posse, so an image behind the source renders a wall that
		// is not the one this tree describes — and until this line existed
		// the only thing that ever said so was a live test's FAIL, which is
		// read as a regression before it is read at all.
		if a.ContainerAvailable() && a.CageImageBuilt(e, image) {
			cwd, _ := os.Getwd()
			fmt.Fprintf(out, "  %s\n", a.CageAgeHere(e, image, cwd))
		}
		if len(args) == 0 {
			fmt.Fprintf(out, "engines: %s (built-in docker; %s/<name>.yaml, config default_engine:)\n",
				strings.Join(a.ListEngines(), ", "), posse.AbbrevHome(a.CagesDir()))
			return
		}
		ag, err := a.LoadAgent(args[0])
		if err != nil {
			die(err)
		}
		rt, err := a.LoadRuntime(a.ResolveRuntime("", ag))
		if err != nil {
			die(err)
		}
		cwd, _ := os.Getwd()
		fmt.Fprintf(out, "%s at cage container in %s — what crosses the boundary:\n", ag.Name, posse.AbbrevHome(cwd))
		for _, m := range a.CageMounts(ag, e, cwd) {
			mode := "rw"
			if m.RO {
				mode = "ro"
			}
			fmt.Fprintf(out, "  mount %s %-46s → %-20s %s\n", mode, posse.AbbrevHome(m.Src), m.Dst, m.Why)
		}
		fmt.Fprintf(out, "  env   names forwarded (values stay out of the typed line): %s\n",
			strings.Join(posse.CageEnvNames(nil), " "))
		if cred := posse.CageCredential(rt); cred != "" {
			fmt.Fprintf(out, "  auth  %s must be in the session env (rangerhq-kiz)\n", cred)
		}
		fmt.Fprintf(out, "  home  %s seeded for %s\n", posse.AbbrevHome(a.CageHome(ag.Name)), rt.Name)
		// The inner wall (rangerhq-6so). Printed as what it is: a question
		// asked of the image, whose answer decides whether the tier may claim
		// a shell-verb deny at all.
		if a.CageInnerGatesReady(e, image) {
			fmt.Fprintf(out, "  gates rendered INSIDE by `%s` → %s (image PATH and shell; refusals mount out to %s)\n",
				strings.Join(posse.GatesWrapArgv(ag.Name, rt), " "), posse.CageGatesDir(ag.Name),
				posse.AbbrevHome(a.RefusalsLogPath(ag.Name)))
		} else {
			fmt.Fprintf(out, "  gates ⚠️  image %s answers no to `posse gates wrap %s` — no Linux posse in it, so L1/L3 do not cross and every shell-verb deny is unrealized here (run `posse cage build`)\n", image, posse.GatesWrapProbe)
		}
		for _, m := range posse.CageSockets {
			state := "not mounted (default — a caged persona holding it can prompt or close every other pane)"
			if posse.CageSocketTag(ag) != "" && strings.Contains(","+posse.CageSocketTag(ag)+",", ","+m+",") {
				state = "MOUNTED — the PID declared it; meta and the cockpit mark the cage " + posse.CageTag(posse.CageContainer, posse.CageSocketTag(ag))
			}
			fmt.Fprintf(out, "  sock  %s: %s\n", m, state)
		}
		// The egress route (ADR 0002 §3 L4, rangerhq-9d0). Printed for every
		// caged persona, not only one with an `egress:` list: at this tier
		// the container's only route out IS the proxy, and the effective
		// allowlist — the runtime's hosts plus the PID's — is the thing the
		// operator most needs to read before launching.
		hosts, bad := posse.EgressHosts(ag, rt)
		if e.NetCreate == "" {
			fmt.Fprintf(out, "  egress engine %s spells no route (net_create:/proxy_up:) — `egress:` is unrealizable on it\n", e.Name)
		} else {
			fmt.Fprintf(out, "  egress --internal network + CONNECT proxy on %s:%d; allowed: %s\n",
				posse.EgressHost, posse.EgressPort, strings.Join(hosts, " "))
			fmt.Fprintf(out, "        (%s's own hosts are always added; denials land in %s)\n",
				rt.Name, posse.AbbrevHome(filepath.Join(a.GatesDir(ag.Name), "refusals.log")))
		}
		for _, b := range bad {
			fmt.Fprintf(out, "  egress ⚠️  %q is not a host — the proxy matches the CONNECT authority; the launch refuses on it\n", b)
		}
		// What the pane actually runs, and why it is not the engine itself:
		// herdr reads the pane's argv0, so a caged session is only visible
		// as an agent behind a launcher named for the runtime (rangerhq-1k1).
		fmt.Fprintf(out, "  argv0 %s → this posse, which execs %s with argv[0]=%s (herdr identifies the session by that name)\n",
			posse.AbbrevHome(a.CageLauncher(ag.Name, rt.Exe())), e.Binary(), rt.Exe())

	case "runtime":
		// The ADR 0013 dispatch-contract grid for ONE runtime — six stages,
		// who declared each, and what a missing one costs. `posse runtimes`
		// (plural) stays the catalog; this is the onboarding surface.
		if len(args) < 2 || (args[0] != "check" && args[0] != "probe") {
			die(posse.Die("usage: posse runtime check|probe <name> (launch profiles: %s)", strings.Join(a.ListRuntimes(), ", ")))
		}
		rt, err := a.LoadRuntime(args[1])
		if err != nil {
			die(err)
		}
		if args[0] == "probe" {
			runtimeProbe(a, rt, args[2:], out)
			break
		}
		// The grid always prints; the exit status is the preflight's
		// (ADR 0012 D4). A `check` that reported an uninstalled CLI and then
		// exited 0 would be the class of green-while-broken this command was
		// filed to end (rangerhq-tr8k and the note on it).
		if !a.RuntimeCheck(rt, posse.NewHerdr(), out) {
			os.Exit(1)
		}

	case "runtimes":
		for _, n := range a.ListRuntimes() {
			rt, err := a.LoadRuntime(n)
			if err != nil {
				continue
			}
			kind := "template-only (gates go to the wall)"
			if rt.Builtin {
				kind = "built-in"
			}
			// The tier dial, said the same way `posse runtime check` says
			// it (Runtime.TierMap): what renders a model, and what renders
			// NOTHING. "runtime default" used to be all this line said for
			// codex and grok, which reads as a choice rather than as a key
			// the runtime ignores — ranger-base-arm is what that cost.
			mapped, unmapped := rt.TierMap()
			tiers := "tiers: " + strings.Join(mapped, " ")
			switch {
			case len(mapped) == 0:
				tiers = "tiers: UNMAPPED — ignores tier:, the CLI picks its own model"
			case len(unmapped) > 0:
				tiers += " · UNMAPPED: " + strings.Join(unmapped, ",")
			}
			fmt.Fprintf(out, "%s %-8s %s · %s\n    %s\n", a.EmojiExact(n), n, kind, tiers, rt.Command)
		}
		// The catalog says what exists; the contract grid says whether a
		// profile can take work (ADR 0013 §1). Nothing else points at it.
		fmt.Fprintln(out, "`posse runtime check <name>` — the dispatch-contract grid for one profile")
		// The catalog is also where an onboarder learns the wall claim is
		// conditional: a template profile's Bash(...) denies do not count
		// until a live probe says they do (ADR 0032 §1).
		fmt.Fprintln(out, "`posse runtime probe <name>` — the live wall probe a template-only profile needs before its Bash(...) denies count")

	case "skills":
		// ADR 0007 §1: the directory is the registry — this is `ls` with the
		// PIDs that bind each name, plus the names a PID declares that
		// nothing answers (which `posse agent check` reports as a finding).
		if len(args) > 0 && args[0] != "list" {
			die(posse.Die("usage: posse skills [list]"))
		}
		bound := a.SkillBindings()
		fmt.Fprintf(out, "%s — skills bound by PIDs (materialized per runtime at launch)\n", posse.AbbrevHome(a.SkillsDir()))
		present := map[string]bool{}
		for _, n := range a.ListSkills() {
			present[n] = true
			who := "bound by no PID"
			if pids := bound[n]; len(pids) > 0 {
				who = strings.Join(pids, ", ")
			}
			fmt.Fprintf(out, "  %-24s %s\n", n, who)
		}
		var missing []string
		for n := range bound {
			if !present[n] {
				missing = append(missing, n)
			}
		}
		sort.Strings(missing)
		for _, n := range missing {
			fmt.Fprintf(out, "  ! %-22s declared by %s — no %s/SKILL.md (posse agent check)\n", n, strings.Join(bound[n], ", "), n)
		}

	case "recipes":
		for _, n := range a.ListRecipes() {
			if r, err := a.LoadRecipe(n); err == nil {
				fmt.Fprintf(out, "%s %s  %s\n", r.Emoji, n, r.Purpose)
			}
		}

	case "init":
		if err := a.CmdInit(out); err != nil {
			die(err)
		}

	case "promote":
		// The constitution's `make install` (ADR 0015 §3). Operator-run: it
		// refuses under the persona env marker, and every crew PID denies
		// `Bash(posse promote:*)` at L1.
		args, help := argLead(args)
		if help {
			fmt.Fprintln(out, "usage: posse promote [<constitution dir>] [--dry-run]")
			os.Exit(0)
		}
		var po posse.PromoteOpts
		for _, v := range args {
			switch {
			case v == "--dry-run":
				po.DryRun = true
			case strings.HasPrefix(v, "-"):
				die(posse.Die("usage: posse promote [<constitution dir>] [--dry-run]"))
			case po.Source == "":
				po.Source = v
			default:
				die(posse.Die("usage: posse promote [<constitution dir>] [--dry-run]"))
			}
		}
		if err := a.CmdPromote(out, po); err != nil {
			die(err)
		}

	case "refresh":
		// The one credential WRITE in posse, and the operator's own hand is
		// the only thing that performs it (ADR 0019 D4, ranger-base-h207).
		// It refuses without a TTY and under the persona env marker; the
		// second spelling of that gate is `Bash(posse refresh:*)` in every
		// crew PID — shipped in the seed's examples (ranger-base-kryn), and
		// the operator's own to keep in a persona they hired.
		args, help := argLead(args)
		usage := "posse refresh [<runtime> [session|meter]] [--env-set <name>] [--paste] [--expires <YYYY-MM-DD>]"
		if help {
			fmt.Fprintf(out, "usage: %s\n", usage)
			os.Exit(0)
		}
		var ro posse.RefreshOpts
		for len(args) > 0 {
			switch v := args[0]; {
			case v == "--paste":
				ro.Paste, args = true, args[1:]
			case v == "--env-set", v == "--expires":
				if len(args) < 2 {
					die(posse.Die("%s needs a value — usage: %s", v, usage))
				}
				if v == "--env-set" {
					ro.EnvSet = args[1]
				} else {
					ro.Expires = args[1]
				}
				args = args[2:]
			case strings.HasPrefix(v, "-"):
				die(posse.Die("unknown flag: %s — usage: %s", v, usage))
			case ro.Runtime == "":
				ro.Runtime, args = v, args[1:]
			case ro.Purpose == "":
				ro.Purpose, args = posse.CredPurpose(v), args[1:]
			default:
				die(posse.Die("usage: %s", usage))
			}
		}
		if err := a.CmdRefresh(out, ro); err != nil {
			die(err)
		}

	case "help", "-h", "--help":
		help()
	case "version", "--version":
		fmt.Fprintf(out, "posse %s (herdr-native)\n", posse.VersionString())
	default:
		die(posse.Die("unknown command: %s (try: posse help)", cmd))
	}
}

func needBd() posse.Bd {
	bd := posse.NewBd()
	if !bd.Available() {
		die(posse.Die("bd not found in PATH (brew install beads or see github.com/steveyegge/beads)"))
	}
	return bd
}

// execEditor replaces this process with $EDITOR on the file — the file is
// never read or echoed by posse itself (env sets hold secrets).
func execEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	quoted := "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
	return syscall.Exec("/bin/sh", []string{"sh", "-c", editor + " " + quoted}, os.Environ())
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}

// costOpts is what `posse cost` was asked for. It returns an error rather
// than calling die() so the flag contract is testable without a subprocess
// — the reading behind --plan is not (it wants the provider credential).
type costOpts struct {
	since   time.Time
	project string
	plan    bool
}

func parseCostFlags(args []string) (costOpts, error) {
	var o costOpts
	rest := args
	for len(rest) > 0 {
		switch rest[0] {
		case "--since":
			if len(rest) < 2 {
				return o, posse.Die("--since needs a date (YYYY-MM-DD or RFC3339)")
			}
			t, err := time.Parse(time.RFC3339, rest[1])
			if err != nil {
				t, err = time.ParseInLocation("2006-01-02", rest[1], time.Local)
			}
			if err != nil {
				return o, posse.Die("--since: %v", err)
			}
			o.since = t
			rest = rest[2:]
		case "--project":
			if len(rest) < 2 {
				return o, posse.Die("--project needs a path substring")
			}
			o.project = rest[1]
			rest = rest[2:]
		case "--plan":
			o.plan = true
			rest = rest[1:]
		default:
			return o, posse.Die("unknown flag: %s", rest[0])
		}
	}
	// --plan prints one reading and never scans a transcript, so the
	// selectors have nothing to select. Refusing beats ignoring: a caller
	// who wrote `--plan --since` believes the date did something.
	if o.plan && (!o.since.IsZero() || o.project != "") {
		return costOpts{}, posse.Die("--plan takes no other flags")
	}
	return o, nil
}

func parseNewFlags(args []string) posse.NewSessionOpts {
	o := posse.NewSessionOpts{Name: args[0]}
	rest := args[1:]
	for len(rest) > 0 {
		flagArg := func() string {
			if len(rest) < 2 {
				die(posse.Die("flag %s needs a value", rest[0]))
			}
			v := rest[1]
			rest = rest[2:]
			return v
		}
		switch rest[0] {
		case "--dir":
			o.Dir = flagArg()
		case "--env-file":
			o.Envs = append(o.Envs, flagArg())
		case "--cmd":
			o.Cmd = flagArg()
		case "--emoji":
			o.Emoji = flagArg()
		case "--agent":
			o.Agent = flagArg()
		case "--runtime":
			o.Runtime = flagArg()
		case "--tier":
			o.Tier = flagArg()
			if !posse.ValidTier(o.Tier) {
				die(posse.Die("--tier must be strong, standard, or fast"))
			}
		case "--allow-degraded":
			o.AllowDegraded = true
			rest = rest[1:]
		case "--cage":
			o.Cage = flagArg()
			if !posse.ValidCage(o.Cage) {
				die(posse.Die("--cage must be shims, seatbelt, or container"))
			}
		default:
			die(posse.Die("unknown flag: %s", rest[0]))
		}
	}
	return o
}

// runtimeProbe is `posse runtime probe <name>` — ADR 0032 §1 rule 2. It runs
// ONE live turn on the runtime being onboarded and writes
// state/runtimes/<name>/probe.json, then prints the four observables.
//
// Exit status is the probe's verdict, because the whole point is that this is
// usable as an onboarding gate: a probe that failed and exited 0 is the
// green-while-broken shape `runtime check` was filed to end.
func runtimeProbe(a *posse.App, rt *posse.Runtime, args []string, out io.Writer) {
	o := posse.ProbeOpts{Out: out}
	for len(args) > 0 {
		switch args[0] {
		case "--keep":
			o.Keep = true
			args = args[1:]
		case "--timeout":
			if len(args) < 2 {
				die(posse.Die("flag --timeout needs a value (e.g. 4m)"))
			}
			d, err := time.ParseDuration(args[1])
			if err != nil || d <= 0 {
				die(posse.Die("--timeout must be a positive duration (e.g. 90s, 4m)"))
			}
			o.Timeout = d
			args = args[2:]
		default:
			die(posse.Die("unknown flag: %s (usage: posse runtime probe <name> [--timeout 4m] [--keep])", args[0]))
		}
	}
	if rt.Builtin {
		// Not a refusal: redeclaring a built-in CLI as a template profile is
		// the M1 acceptance flow, and probing THAT profile is the point. What
		// is refused is the misreading — a built-in's Bash claim rests on ADR
		// 0009's argv table, so a record here unlocks nothing in parity.
		fmt.Fprintf(out, "note: %s is a BUILT-IN — its shell argv shapes were probed in ADR 0009 (rangerhq-e43)\n", rt.Name)
		fmt.Fprintln(out, "      and parity does not consult a probe record for it. Probing anyway: the record is")
		fmt.Fprintln(out, "      evidence, and this is how the same CLI redeclared as a template profile is checked.")
		fmt.Fprintln(out)
	}
	rec, err := a.RuntimeProbe(rt, posse.NewHerdr(), o)
	if err != nil {
		die(err)
	}
	fmt.Fprintln(out)
	for _, ob := range rec.Observables {
		mark := "✗"
		if ob.OK {
			mark = "✓"
		}
		fmt.Fprintf(out, "  %s %d %-16s %s\n", mark, ob.N, ob.Name, ob.Detail)
	}
	fmt.Fprintf(out, "\n  record %s (%s %s)\n", posse.AbbrevHome(a.ProbeRecordPath(rt.Name)), rt.Exe(), versionOrUnknown(rec.Version))
	if rec.Passed() {
		fmt.Fprintf(out, "  PASS — Bash(...) denies on %s are measured, not assumed. Re-probe after upgrading %s.\n", rt.Name, rt.Exe())
		return
	}
	fmt.Fprintf(out, "  FAIL — Bash(...) denies on %s stay ASSUMED: a launch degrades on each one\n", rt.Name)
	fmt.Fprintf(out, "  (--allow-degraded waives it; tier fast never does). `posse runtime check %s` repeats this.\n", rt.Name)
	os.Exit(1)
}

// versionOrUnknown keeps an unreadable version visible as UNKNOWN rather
// than as an empty string that reads like agreement (ADR 0032's rule that
// undeclared is loud).
func versionOrUnknown(v string) string {
	if v == "" {
		return "version unknown"
	}
	return v
}

func help() {
	fmt.Print(`posse — the Ranger work-system harness (herdr-native)

A subcommand that takes a <name> prints its own usage for -h/--help, and
reads a literal -- as the end of flags (posse kill -- -oddly-named).

sessions (herdr workspaces):
  posse list                     sessions with live agent state (working/blocked/idle)
  posse new <name> [opts]        create a background session
      --dir <path>  --env-file <name> (repeatable)  --cmd "..."  --agent <name>  --emoji <e>
      --runtime <claude|codex|grok|name>   launch profile for the persona (over its PID runtime:)
      --tier <strong|standard|fast>        model tier for the persona (over its PID tier:)
      --allow-degraded                     launch even if the wall cannot realize every PID gate here (marked)
      --cage <shims|seatbelt|container>    wall tier (over the PID cage:); seatbelt = sandbox-exec file gate
  posse attach <name>            focus its workspace in herdr (alias: focus)
  posse up <name>                create-or-focus (alias: local)
  posse recipe <name>            launch a saved recipe (<config home>/recipes)
  posse relaunch <name>          refresh a session in place: check the recreate
                                 first (a refusal here costs nothing), land the
                                 plane (one bounded turn to write lessons down
                                 and commit), kill, recreate from the same
                                 persona/dir/envs
      --no-land                skip the landing turn (dead or wedged sessions)
      --timeout <interval>     bound on the landing turn (default 10m)
      --force                  refresh even while its bead is open and its tree dirty
  posse kill <name>              land the plane (one bounded turn to write lessons
                                 down), commit the persona's standing orders, close
                                 the workspace, land its worktree's branch on the
                                 repo's branch and remove the worktree (a tree still
                                 holding work is kept and says so). A session still
                                 holding an in_progress bead over uncommitted work is
                                 NOT killed at all (ADR 0013 §4)
      --force                  kill it anyway, once you have read the refusal
      --foreign                close a workspace this home holds no session meta for
                               (another instance's session, or one made in herdr by
                               hand) — refused without it, naming the workspace id
      --no-land                skip the landing turn (dead or wedged sessions, or a
                               sweep of many). The persona's memory is still
                               committed, and the worktree still lands — this flag
                               only declines to spend a turn
      --timeout <interval>     bound on the landing turn (default 10m)
  posse worktrees [--dir <repo>] [--land [--force]]
                                 session worktrees, which bead each one's unlanded
                                 work belongs to, and what has not landed yet;
                                 --land merges every branch that will land (it
                                 never removes a tree — it cannot tell a dead
                                 session's from a live one's)
      --force                  land a tree holding work no bead record accounts
                               for — refused without it, because from git alone
                               that is indistinguishable from work already
                               landed under another bead id
  posse crew <name> [--off]      mark a session as yours (👤) so dispatch leaves it
                                 alone, or --off to give it back to the fleet
                                 (ADR 0008; posse new and recipes are crew already,
                                 and prompting one by hand marks it)

dispatch (beads):
  posse prompt <name> "<text>" [--wait] [--timeout <ms>] [--now]
                                 submit work to the session's agent; waits first
                                 for herdr to recognize the screen, so a prompt
                                 into a CLI still starting up is refused rather
                                 than typed at whatever holds the keyboard
                                 (ranger-base-3p0) — --now skips that gate
  posse wait <name> [--until <state>]...   wait for idle|done|blocked
  posse peek <name> [<lines>]    read the session's terminal tail
  posse ready [--dir <repo>] [--as <persona>]
                                 unblocked work (config beads: repos, or --dir);
                                 files the verify beads the last closes earned
                                 (verify_labels:, ADR 0006 §3)
  posse beads check [--dir <repo>] [--record "<reason>"] [--as <who>]
                                 two alarms: beads git ever carried that bd
                                 can no longer resolve — bd's auto-import
                                 deletes rows on a git-history signal and
                                 logs nothing when it does (rangerhq-fuom) —
                                 and a symmetric same-type dependency pair in
                                 the live graph, the bd 0.49.1 dep-add
                                 landmine (NOTES.md, ranger-base-pkqn).
                                 Non-zero on either; --record owns a lost
                                 bead in .beads/deleted.jsonl (keeping its
                                 last JSONL line) but does not touch a pair —
                                 prune those with
                                 scripts/prune-bd-relates-to.sh
  posse claim <id> [--as <persona>] [--dir <repo>]   atomically claim an issue
  posse done  <id> [--as <persona>] [--dir <repo>]   close an issue
  posse dispatch [--dry-run] [--dir <repo>] [--persona <p>] [-n <max>] [--timeout <ms>]
                                 one pass: file verify beads for closes that
                                 earned one, then route ready beads to personas —
                                 find-or-create session, claim, prompt --wait,
                                 report closed/blocked/review per bead
                                 (-n caps launch attempts per dispatch_epoch:
                                 (default 1h — ADR 0028 §2), failures included,
                                 taking beads in priority then age order;
                                 operator questions cost no attempt; -n 0 is
                                 no cap, and anything that is not a count is
                                 refused rather than read as 0)
      --timeout <ms>           one --wait leg (default 15m): when it runs out
                                 and the agent is still working, the wait is
                                 extended and the claim kept
      --ceiling <interval>     stop waiting on one bead after this long
                                 (default 4h) — the claim is still kept
      --runtime <name>         launch profile for sessions created this pass
      --tier <name>            model tier for sessions created this pass
      --allow-degraded         launch sessions whose gates the wall cannot fully realize (marked; never on its own)
      --cage <tier>            wall tier for sessions created this pass
      --no-reap                skip the end-of-pass auto-reap this pass only
                                 (config auto_reap:, default true, is the
                                 standing switch; --dry-run only lists what
                                 the reaper would kill — a per-bead session
                                 whose bead is closed and whose agent herdr
                                 calls idle/done; never a crew session, the
                                 persona's own reusable slot, or a session any
                                 launcher prompted within the prompt grace)
      --resume                 re-prompt in_progress beads whose persona
                                 session is alive and idle, and take them
                                 before fresh work (default: only interrupted
                                 runs resume — no live agent)
      --watch <interval> [--max-interval <i>]   keep passing: sleep between
                                 passes, quiet passes double the sleep up to
                                 max (default 8× interval); ctrl-c stops
                                 the loop holds flock(2) on
                                 state/dispatch-watch.lock for its whole life,
                                 so one loop per RHQ_HOME is the kernel's rule
                                 and a second --watch refuses rather than
                                 double-dispatching the queue; it also stamps
                                 state/dispatch-watch.pid — which pid, since
                                 when, under what argv, for the operator and
                                 never as evidence
      --watch-status           is a --watch loop of this RHQ_HOME running?
                                 one line, read from the lock:
                                   watch-loop: running (pid N, since T)
                                   watch-loop: none (<lock> is free)
                                 The LINE is the answer; the exit status says
                                 only whether the question could be asked.
                                 Reads no bd and no herdr —
                                 plugin/autostart.sh asks it at every herdr
                                 server start to tell a live loop from a
                                 restored husk
                               config plan_guard_<window>: (percent, one key
                                 per rate window the provider adapter reports;
                                 a name it does not report is named on stderr)
                                 skip a pass above the plan's rate windows;
                                 unset = off, unreadable = no-op — except under
                                 --watch, where plan_guard_blind_max: (10m,
                                 0 = never) ends quiet tolerance once the last
                                 good reading is that old. Past it (ADR 0018)
                                 the pass parks on-meter beads when
                                 budget_pass:/budget_day: are unset — the last
                                 armed brake fails closed — and runs loudly
                                 under those caps when they are set, until one
                                 reading succeeds
                               config plan_guard_overflow:/_cap: (ADR 0010)
                                 a tripped guard runs the pass and sends the
                                 beads that can move to this runtime instead —
                                 parity-clean there, not strong, PID not
                                 overflow: false — capped at N beads per
                                 rolling 7d ($StateDir/overflow.log); the cap
                                 is required, and a blind guard never
                                 overflows. The target must be a SECOND pool:
                                 the guarded runtime itself is overflow off.
                                 Beads whose own runtime is not on the guarded
                                 meter launch ungated
                               config budget_pass:/budget_day: (API-equiv $)
                                 ADR 0003 Dial E: at 80% of a window a standard
                                 session steps down to fast (parity permitting,
                                 never below tier_floor: or a pinned tier), at
                                 100% dispatch stops with a line per bead;
                                 both unset = dormant, nothing is even scanned.
                                 budget_pass: measures one dispatch_epoch:
                                 (default 1h, wall-clock aligned — ADR 0028 §2),
                                 which -n also bounds launch attempts per.
                                 Arming either also arms the blind plan
                                 guard's degrade (ADR 0018) — and an
                                 unreadable cost scan is not $0 spent, so it
                                 parks there and says so on stderr elsewhere
                               config load_guard: (1-min load average)
                                 skip the whole pass above it, a witness line
                                 naming the load and a second naming the top
                                 CPU burners (orphans flagged), and refuse
                                 every session launch —
                                 posse new, relaunch, recipes — while the
                                 box is over — a box far above its core count
                                 cannot fork, and every spawn on it hangs
                                 silently. Default 25, which assumes ~8 cores;
                                 load is not core-normalised, so set it from
                                 your own quiet baseline. 0 = off. Running
                                 sessions are never touched, and an unreadable
                                 load gates nothing

catalog:
  posse envs                     list env sets (key names only)
  posse env edit|rm <name>       manage an env set ($EDITOR; created if missing)
  posse refresh                  ADR 0019 D4 — credentials: what this box has, where
                                 each one lives, when it dies, and the fix. The one
                                 credential WRITE in posse, and the operator's own
                                 hand is the only thing that performs it: it refuses
                                 without a TTY and under RHQ_PERSONA, and every crew
                                 PID adds Bash(posse refresh:*) to deny: (the seed's
                                 example PIDs ship that line; a hired crew PID is the
                                 operator's own file to keep it in).
                                 No argument = the report, and nothing is written.
  posse refresh <runtime> [session|meter] [--env-set <name>] [--paste] [--expires <YYYY-MM-DD>]
                                 session: runs the runtime's own mint (claude
                                 setup-token — its browser flow is the human gate),
                                 then writes the pasted token into the env set, 0600
                                 in a 0700 dir, above a '# minted=' stamp and a
                                 '# expires=' one only when --expires says so — posse
                                 cannot ask a setup-token when it dies and reports
                                 what it cannot tell as exactly that. --paste skips
                                 the mint for a box with no browser. A metered key
                                 (ANTHROPIC_API_KEY, or an sk-ant-api… value) is
                                 refused on the money line.
                                 meter: writes NOTHING and never will — the rotating
                                 OAuth token's only writer is the runtime's own login
                                 loop, and it prints where that store is instead.
  posse agents                   list personas
  posse runtimes                 list launch profiles (claude/codex/grok + runtimes/*.yaml)
  posse runtime check <name>     the ADR 0013 dispatch-contract grid for one launch profile,
                                 then the ADR 0012 D4 preflight (exit 1 on a blocking gap):
                                 launch/promptable/work/record/settle/account, who declared each,
                                 and what a missing stage costs. Undeclared reads loud, not silent.
  posse runtime probe <name>     the ADR 0032 live wall probe for a template-only profile:
             [--timeout 4m]      one turn on the CLI with a scratch PID carrying a canary deny,
             [--keep]            read for four observables (shim precedence, refusal through
                                 direct/sh -c/script, unattended turn, herdr detection) and
                                 recorded in state/runtimes/<name>/probe.json. Until it passes,
                                 Bash(...) denies on that runtime are "assumed, not measured" and
                                 degrade the launch. Exit 1 when it fails. --keep leaves the pane.
  posse skills                   list bound skills (RHQ_HOME/skills) and the PIDs that bind them
  posse gates <persona>          the persona's L1 gate shims (from deny:), the seatbelt
                                 writable set with ADR 0015 §2's constitution check
                                 over it, and refusals.log
  posse gates install-hooks [dir] [--chain]
                                    L3: .git/hooks/pre-push refusing git push under RHQ_TOOLS_DENY,
                                    and prepare-commit-msg refusing an unqualified commit from any shell
                                    plus ops-class content added to .beads/*.jsonl in a repo that
                                    config beads_visibility: does not mark private (unmarked = public).
                                    Both slots are attempted even if one is foreign. --chain takes over
                                    a slot occupied by bd's own shim (# bd-shim v1) instead of refusing:
                                    bd's shim moves to bd-<slot>, ours goes to posse-<slot>, and the
                                    real slot gets the process-and-status dispatcher (INSTALL.md §9).
                                    A hook that is neither ours nor bd's is still refused.
  posse cage [<persona>]         L4: the container engine, its image, and what a
                                 caged launch of that persona would mount and forward
  posse cage build [dir] [--runtimes "<npm pkgs>"]
                                 build the cage image from a posse checkout
                                 (cross-builds the Linux posse and bd it carries)
  posse cage down <persona>      take down that persona's egress networks and proxies
                                 (the launch's own watcher does this when a cage exits)
  posse scorecard [<persona>]    per-persona outcome metrics from bd data
                                 (closed/reopened/held/blocked, age at close,
                                 filed/rejected; each PID metric id computed, or
                                 declared with what bd would need)
  posse scorecard --catalog      the derived metric catalog: every id the PIDs
                                 and config metric_ids: declare, computed or not
  posse cost [--since <date>] [--project <substr>]
                                 API-equiv $ per bead from every provider with a
                                 cost adapter, by tier/persona/day; a runtime
                                 without one is reported as uncounted, not $0;
                                 plus the plan's own rate windows when readable
                                 and the budget_pass:/budget_day: caps in force
  posse cost --plan              just the plan's rate windows, no transcript scan
                                 (shared reading; exits 1 when unreadable)
  posse agent new <name>         scaffold a persona (PID shape) and open it in $EDITOR
  posse agent edit <name>        open an existing persona in $EDITOR
  posse agent check [<name>|--all]  lint PIDs against the ADR 0001 contract (exit 1 on findings)
  posse memory <persona>         edit the persona's standing orders ($EDITOR)
  posse recipes                  list recipes
  posse init                     seed ~/.config/posse from the built-in examples
                                 (examples/ beside the binary wins, for dev builds)
  posse promote [<dir>] [--dry-run]
                                 ADR 0015 §3 — put the constitution in force: copy
                                 agents/, config.yaml, recipes/, skills/ from the
                                 constitution repo AT HEAD into the home and record
                                 {source, sha, sha256/file} in promoted.json beside
                                 them. Prints the diff since the last promote, so
                                 what is ratified is a diff and not a vibe. Refuses
                                 on a dirty promoted path (nothing uncommitted is
                                 ever in force) and under RHQ_PERSONA. Never
                                 creates, copies or touches envs/ (§7: gitignored
                                 secrets, no commit to promote from), state/ or
                                 personas/. <dir> defaults to the last promote's
                                 source, then config constitution:.
                                 Every launch re-hashes the promoted set against
                                 promoted.json: dispatch refuses on a mismatch,
                                 an interactive launch warns DEGRADED.

governance:
  posse status                   the condition set: what needs a human right now,
                                 URGENT (the shop is stopped) before LANE (one bead
                                 or session is). Computed live from the stores that
                                 own each fact — herdr, bd, the plan endpoint, the
                                 watch loop's flock, state/pause.yaml — so it depends
                                 on no loop and reports a dead one itself. Exit
                                 non-zero when the set is non-empty OR a store could
                                 not be read (unknown is not an all-clear)
                               config attn_question_age: (4h) how long a
                                 -l question / -l risk bead may sit open
                               config attn_guard_stuck: (2h) how long the plan
                                 guard may skip before a skip becomes a condition
                                 (the streak is the --watch loop's own; a fresh
                                 shell has none and reports no G4)
  posse pause "<why>"            stop dispatching until told otherwise. Writes
                                 state/pause.yaml (by:, at:, why: — the why is
                                 mandatory and is what every declining pass
                                 prints). Every pass checks it at the fire
                                 loop's entry — watch, hand-typed, cockpit d —
                                 and a pass in flight finishes first.
                                 PAUSE STOPS SPEND, NOT OVERSIGHT: the pulse
                                 keeps ticking, so a paused shop still escalates
                                 blocked sessions and aging questions. Nothing
                                 mechanical ever writes this file — the guard,
                                 the blind window and Dial E SKIP a pass and
                                 heal; only a human pauses. The operator, and
                                 the coordinator; a second pause keeps the first
  posse resume                   lift it. Idempotent

cockpit (herdr plugin pane — make link-plugin):
  posse cockpit                  interactive oversight: sessions blocked-first +
                                 ready beads; enter focus · p prompt · v peek ·
                                 x kill · c claim · q quit

environment:
  RHQ_HOME       config dir (default ~/.config/posse; existing ~/.config/rhq falls back).
                 POSSE_HOME is also accepted; RHQ_HOME wins when both are set
                 (transition window, ranger-base-mlc)
  RHQ_HERDR_BIN  herdr binary override (testing)
  RHQ_BD_BIN     bd binary override (testing)
  RHQ_PLAN_USAGE_URL  plan-usage endpoint override (testing; loopback hosts only —
                      asked without the account's credential, and its answer is
                      not shared with other posse processes)
`)
}
