package posse

// posse's own children run outside every gates dir (ADR 0077 D3).
//
// A seat is a child of the multiplexer SERVER, and the posse process that
// hired it is a child of whatever shell the operator — or another seat —
// happened to be in. So any posse process that launches, dispatches, checks
// parity or renders a wall can be running with a persona's gates bin dir
// leading its PATH, donated by a seat that is not this one and, on herdr
// homes, by a server that was autostarted hours earlier (MEASURED
// 2026-10-31, ranger-base-c33j6: a gates dir mid-PATH that the launching
// process did not put there).
//
// The walls bind what a persona TYPES (ADR 0009). A binary posse execs as
// part of its own protocol is posse's, not the typing persona's, so it is
// resolved and run as though no gates dir were on PATH at all. The measured
// cost of not doing that is ranger-base-c33j6's deadlock: posse took ADR
// 0073's per-store write lock, exec'd `bd` by bare name, landed on the
// launching persona's `bd` shim, and the shim took the SAME lock on a new
// open file description — posse waited BdWriteLockWait on itself and refused
// the write, every dispatch from a gated seat.
//
// The three functions below are that rule, and they are the ONLY route a
// shipped child gets an environment by: internal/treepins'
// TestQAEveryProtocolChildRunsOutsideTheGates censuses every
// `exec.Command`/`exec.CommandContext` in cmd/ and internal/ against them,
// and every `.Env` assignment beside it — because a site that binds the
// gates-free PATH and is then handed `append(os.Environ(), …)` two lines
// later has lost it, and nothing at the call site shows that.
//
// WHAT IS NOT IN SCOPE, and is the other half of the census: a process that
// IS the seat, or is the wall. `posse <runtime> --posse-cage-launch` runs in
// the pane as the seat's own argv0 (cagelauncher.go), and the bd write shim
// runs as the shim. Those carry the seat's environment on purpose; a seat's
// walls are its own PID's, and the one place a gates dir SHOULD lead PATH is
// the seat behind it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// protocolBin is the binary one of posse's own children execs: a bare name
// resolved against the gates-free PATH rather than left to Go's
// exec.LookPath, which searches THIS process's $PATH and nothing else — not
// even the cmd.Env protocolEnv is about to set.
//
// A name that NAMES a path — `RHQ_BD_BIN`, a configured `rex_bin:`, a
// fixture's symlink — keeps precedence and comes back as given: an explicit
// binary is an answer, not a search.
//
// A BARE NAME NOTHING OUTSIDE THE GATES ANSWERS IS AN ERROR, and that is the
// whole of the branch (ranger-base-cwsi8, whose finding this generalizes).
// ranger-base-rzjie's version handed the bare name back "so that exec
// reports it the way it always has", which is the one thing exec does not do
// with it: exec.Command resolves a bare name with Go's own exec.LookPath,
// which reads THIS process's $PATH — the gated one, the premise stated
// twice above — so the name lands on the launcher's shim. For `bd` that
// meant the shim took ADR 0073's per-store write lock that runOnce was
// already holding, and posse refused its own write after BdWriteLockWait:
// the fallback meant to be inert was ranger-base-c33j6's deadlock,
// reinstated on the production spelling, since NewBd's Bin is the literal
// "bd".
//
// MEASURED 2026-10-31 (the verifying seat of ranger-base-nqy1h, its own
// worktree at main 1f8b81e2, darwin 25.6.0),
// TestBdRunsOutsideTheLaunchersGatesDir's fixture with the real bd behind
// the shim taken away and one ordinary non-gates PATH entry holding nothing
// called bd: the resolution answered "bd", the shim ran with posse's own
// argv, and runOnce came back after 2.011s refused on its own lock (2s
// because the fixture shim waits 2s; a live seat pays BdWriteLockWait's
// thirty). Pinned both ways in
// internal/posse/bdgatesfreepath_test.go —
// TestBdBinUnresolvableOutsideTheGatesIsAnError.
//
// WHY IT HAD NOT FIRED on this box: a real bd sits at ~/.local/bin/bd, on
// PATH and outside every gates dir, so resolveOutside answered and the
// branch was unreachable. There is no bd in /opt/homebrew/bin,
// /usr/local/bin, /usr/bin or /bin — that one PATH entry was the entire
// margin. The same margin is all that stands behind every OTHER name in the
// census (`git`, `sh`, `sqlite3`, `python3`, `gh`, an engine), which is why
// the error is the rule here and not a special case for bd: a shim is
// rendered for every command a PID denies, whether or not a real binary
// stands behind it.
//
// The error reaches the caller rather than exec: protocolCmd puts it on
// cmd.Err, so Run/Output return it having started nothing, and a caller that
// must decide BEFORE doing something expensive — runOnce resolves before it
// takes the store lock — asks this function directly.
func protocolBin(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	if real := resolveOutside(name, ""); real != "" {
		return real, nil
	}
	return "", Die("%s: executable file not found outside posse's gates dirs — posse's own children are resolved against the gates-free PATH and never handed to exec as a bare name (ADR 0077 D3, ranger-base-cwsi8)", name)
}

// protocolAvailable is "is there a <name> posse could run as part of its own
// protocol" — the PROBE beside protocolBin, asking its question over the same
// gates-free PATH so that an availability gate cannot open onto a binary the
// resolution is about to refuse.
//
// ranger-base-g1xnt is why it exists: Bd.Available was `exec.LookPath(b.Bin)`,
// which searches THIS process's $PATH. On a gated seat that PATH leads with
// the launching persona's gates bin dir, that dir holds a `bd` shim, and the
// probe therefore answered true on the shim while protocolBin — one function
// away — refused the same name. Every other posse bd surface had stopped
// reading that PATH; the probe was the one still asking the old question.
//
// Same precedence as protocolBin, with one difference, and the difference is
// the reason this is not `_, err := protocolBin(name); err == nil`: a name
// that SPELLS a path is an answer rather than a search in both, but
// protocolBin hands such a name back UNCHECKED — an explicit RHQ_BD_BIN is
// the caller's answer, and exec reports it missing better than a pre-flight
// can — while a probe's whole question is whether the file is there and
// runnable. resolveOutside draws both arms already: absLookPath for a name
// that spells a path, the gates-free walk for a bare one.
//
// So this is never WIDER than protocolBin — everything it calls available,
// protocolBin resolves — and that is the invariant worth having, since a
// probe answering true where the resolution refuses is a gate that lies. It
// is narrower on an explicit path that does not exist, which is a probe
// saying no and a caller standing down rather than a gate opening onto a
// refusal.
func protocolAvailable(name string) bool { return resolveOutside(name, "") != "" }

// protocolEnv is the environment one of posse's own children runs with: the
// caller's, with ADR 0077 D3's gates-free PATH put over whatever PATH it
// carried. nil means this process's environment, which is what an unset
// cmd.Env would have inherited — made explicit, because the whole point is
// that posse writes this layer rather than inheriting it.
//
// The PATH is SET and not merely resolved past, which protocolBin above
// would be enough for on its own: a child's own children call binaries by
// name too — bd's git hooks call `bd`, a `sh -c` predicate calls `git` — and
// a grandchild inheriting the launcher's PATH reaches the shim the same way
// the direct call did (ASSUMED; the measured deadlock was the direct call,
// ranger-base-c33j6).
//
// Only PATH. `RHQ_GATES_DIR` and `RHQ_TOOLS_DENY` stay as they are: they are
// how a shim that IS reached knows whose wall it is, and shedding them would
// be a way to run a shim with no rules rather than not to run one.
func protocolEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	return envPut(env, "PATH", PathOutsideGates(""))
}

// protocolCmd binds both halves onto one child and hands it back, so a call
// that is chained onto .Output()/.Run() stays one expression and a call that
// keeps its *exec.Cmd needs no second line. It is bindGitDeadline's shape
// for the same reason: one choke point, and no call site that can forget.
//
// WRAPPING THE exec CALL RATHER THAN REPLACING IT is deliberate. A
// protocolCommandContext() of our own would hide every bounded child from
// internal/treepins' TestQAEveryBoundedChildNamesItsWaitDelay, which
// censuses `exec.CommandContext` by name — a second pin hollowed out to
// tidy this one.
//
// cmd.Args[0] is left alone, so the child's own argv0 and every error
// message built from cmd.Args read exactly as before; only cmd.Path moves.
//
// An unresolvable bare name goes on cmd.Err and NOT on cmd.Path, which is
// os/exec's own seam for exactly this: Start returns cmd.Err having forked
// nothing, so Run and Output hand the caller protocolBin's sentence instead
// of running whatever this process's $PATH answered with. Leaving Path at
// exec's own answer would have been the ranger-base-cwsi8 defect one level
// out — exec.Command has already resolved the name on the gated PATH by the
// time this runs, so the shim is sitting in cmd.Path right now.
func protocolCmd(cmd *exec.Cmd) *exec.Cmd {
	if cmd == nil {
		return nil
	}
	if len(cmd.Args) > 0 {
		real, err := protocolBin(cmd.Args[0])
		switch {
		case err != nil:
			cmd.Path, cmd.Err = cmd.Args[0], err
		case real != cmd.Args[0]:
			cmd.Path, cmd.Err = real, nil
		}
	}
	cmd.Env = protocolEnv(cmd.Env)
	return cmd
}
