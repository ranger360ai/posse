package treepins

// QA pins for ranger-base-uq81v — a probe against a SCRATCH Rex server cannot
// reach the fleet's, and a cleanup cannot kill what the run did not create.
//
// THE INCIDENT. 2026-11-15 08:31:19-22 local, a scratch-server live test
// destroyed eight live fleet seats — the operator's own, a long-lived one, and
// six dispatched seats across five personas, one of them the seat running the
// loop. Its script started a scratch rex server, exported a scratch
// XDG_RUNTIME_DIR, and expected the rex CLI and posse to resolve that socket.
// Its own sanity line printed
//
//	"endpoint": "unix:///run/user/<uid>/rex/server.sock", "session_count": 12
//
// which is the FLEET server, and the run did not stop: it created seats there
// through `posse new`, and its cleanup ran `rex kill` on every session in `rex
// ls`. The reading was right, printed, and acted against.
//
// WHY THE EXPECTATION WAS WRONG is MEASURED in docs/notes.d/ranger-base-uq81v.md
// and in scripts/scratch-rex.sh's own header: rex takes its runtime dir from
// REX_RUNTIME_DIR else a /run/user/<uid> path it builds from getuid(), and
// reads XDG_RUNTIME_DIR for it not at all; REX_SERVER is a variable rex
// EXPORTS into a pane beside REX_SESSION, not one the CLI reads, so a bogus
// value changes nothing and raises nothing. posse is worse than that: it has
// no endpoint door whatever (`NewRexMux` builds `Rex{Bin: …}` with an empty
// `Server`), so the only seam that moves posse off the box's default socket
// is the BINARY — `rex_bin:` / RHQ_REX_BIN.
//
// WHAT IS PINNED HERE, and what is not. The harness is
// scripts/scratch-rex.sh: it starts the server, aborts unless `whoami`
// answers with byte-equal the socket it started and a session_count of 0, and
// hands the body a shim at the head of PATH and in RHQ_REX_BIN that prepends
// `-S <that socket> --autostart=false` to every call and refuses a `kill` of
// an id the run did not create. These arms hold its judges (arm 1), the abort
// under the incident's own reading (arm 2), the harness's own `-S` discipline
// (arm 3) and the tracked tree's (arm 4).
//
// WHAT NOTHING HERE CAN HOLD, said out loud: the incident's script was a
// SCRATCHPAD file, and a pin over the tracked tree cannot read one. What
// binds a seat's own throwaway probe is the harness being there and being
// reachable — AGENTS.md's "Probing rex" bullet is the other half of this
// bead, and it is prose for the same reason the `go test ./...` bullet beside
// it is.
//
// Doored by $(QA_TREEPINS_EXCLUDED)'s complement — `make treepins-check`
// (ADR 0098 D1). Arms 1 and 2 run a tracked program, so scripts/scratch-rex.sh
// has a twdExecDispositions row in treewidedoor_qa_test.go; arm 4 walks the
// tracked tree, which is what makes the file tree-wide by this package's class
// rule.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scratchRexScript names the harness in this file's messages and in the reads
// of its text. The exec.Command call sites spell the same path as a LITERAL
// instead, deliberately: treewidedoor_qa_test.go's exec census matches a
// relative literal in an exec.Command argv and says so — "a program path
// assembled from a variable … is not matched" is its own stated residual — so
// a const here would keep these arms outside the register that asks what a
// tracked program reads. Inside it with a twdExecDispositions row is the
// honest side of that line.
const scratchRexScript = "scripts/scratch-rex.sh"

// srxShimSays is how the shim's own refusals begin. A fenced call is told
// apart from a call that merely failed by this prefix and by nothing else.
const srxShimSays = "scratch-rex shim:"

// ─── arm 1: the guards' own judges, over the script's --self-test table ──────

// TestScratchRexGuardsCanStillSayNo runs the harness's --self-test, which
// drives endpoint_is, recorded, the option-aware verb reader and the
// server-flag detector over a table and opens nothing: no server, no socket,
// no rex binary, no fleet. It is the door this file exists to be.
//
// The table's own negative rows are the point — "the box's default socket is
// not the one this run started", "an id this run did not create is refused" —
// so a judge rewritten to answer yes to everything reds here rather than in
// the next incident.
func TestScratchRexGuardsCanStillSayNo(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("bash", "scripts/scratch-rex.sh", "--self-test").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --self-test: %v\n%s", scratchRexScript, err, out)
	}
	text := string(out)
	// The run must have SAID something: an exit 0 over a table that printed
	// no rows is a self-test that stopped testing.
	if n := strings.Count(text, "PASS  "); n < 15 {
		t.Fatalf("--self-test printed %d PASS rows (20 when this pin was written) — "+
			"a green exit over a table this short is not the guard being exercised:\n%s", n, text)
	}
	if strings.Contains(text, "FAIL") {
		t.Fatalf("--self-test printed a FAIL:\n%s", text)
	}
}

// ─── arm 2: the incident's own reading, and the abort over it ────────────────

// srxFakeRex writes a rex that LISTENS where it is told — so the socket
// appears and the harness gets as far as its identity read — and then answers
// `whoami` for whichever server the mode names. Everything else exits 0 and
// prints nothing, which is enough: the shim's fences run BEFORE the binary is
// exec'd, so a refused call never reaches this file at all.
//
// `self` echoes back the `-S` value it was given, which is what the real CLI
// does. `fleet` ignores it and answers with the box's default socket and
// session_count 12 — the sentence live.sh printed at 08:30:26 and ran on past.
// The fleet-shaped path is BUILT and not spelled: a literal uid path in a
// tracked file is what `make identity-check` is for.
//
// ADR 0085's class: this plants an executable and execs it, so the arms below
// run under `make coldexec-check`. They need no `--warm` pre-flight — the
// harness waits up to 10s for its socket and nothing here is on a tighter
// deadline, against a worst measured first-exec cost of 2.17s on darwin.
func srxFakeRex(t *testing.T, dir, mode string) string {
	t.Helper()
	runroot := os.Getenv("XDG_RUNTIME_DIR")
	if runroot == "" {
		runroot = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	var answer string
	switch mode {
	case "self":
		answer = "\tprintf '{\"endpoint\": \"%s\", \"session_count\": 0}\\n' \"$sarg\" ;;\n"
	case "fleet":
		answer = "\tprintf '{\"endpoint\": \"%s\", \"session_count\": 12}\\n' " +
			"\"unix://" + runroot + "/rex/server.sock\" ;;\n"
	case "count":
		answer = "\tprintf '{\"endpoint\": \"%s\", \"session_count\": 12}\\n' \"$sarg\" ;;\n"
	case "fleetzero":
		answer = "\tprintf '{\"endpoint\": \"%s\", \"session_count\": 0}\\n' " +
			"\"unix://" + runroot + "/rex/server.sock\" ;;\n"
	default:
		t.Fatalf("srxFakeRex: unknown mode %q", mode)
	}
	// `new`, `ls` and `kill` keep a session list in a file beside the fake,
	// which is what makes the POSITIVE half of guard 3 measurable: a run's own
	// session is created, lands in the harness's record, and is killable
	// through the fence. Without it, mutant 10 of this bead's battery ("the
	// record is never refreshed") SURVIVED — a fence that refuses everything,
	// its own work included, passes every arm that only asks it to say no.
	fake := "#!/usr/bin/env bash\n" +
		"set -uo pipefail\n" +
		"held=\"${0%/*}/fake-sessions\"\n" +
		"listen=\"\"; sarg=\"\"; prev=\"\"; verb=\"\"; ids=()\n" +
		"for a in \"$@\"; do\n" +
		"\t[ \"$prev\" = --listen ] && listen=$a\n" +
		"\t[ \"$prev\" = -S ] && sarg=$a\n" +
		"\tcase $a in\n" +
		"\tserver | whoami | new | ls | kill) [ -n \"$verb\" ] || verb=$a ;;\n" +
		"\tsession:*) ids+=(\"$a\") ;;\n" +
		"\tesac\n" +
		"\tprev=$a\n" +
		"done\n" +
		"case $verb in\n" +
		"server)\n" +
		"\texec python3 -c 'import socket,sys,time\n" +
		"p = sys.argv[1]\n" +
		"if p.startswith(\"unix://\"): p = p[len(\"unix://\"):]\n" +
		"s = socket.socket(socket.AF_UNIX); s.bind(p); s.listen(1)\n" +
		"time.sleep(600)' \"$listen\" ;;\n" +
		"whoami)\n" + answer +
		"new)\n" +
		"\tid=\"session:$(date +%s%N)\"\n" +
		"\techo \"$id\" >>\"$held\"\n" +
		"\tprintf '{\"session_id\": \"%s\"}\\n' \"$id\" ;;\n" +
		"ls)\n" +
		"\tcat \"$held\" 2>/dev/null || true ;;\n" +
		"kill)\n" +
		"\tfor id in \"${ids[@]}\"; do\n" +
		"\t\tgrep -vxF -- \"$id\" \"$held\" >\"$held.new\" 2>/dev/null || true\n" +
		"\t\tmv -f \"$held.new\" \"$held\"\n" +
		"\tdone ;;\n" +
		"*) exit 0 ;;\n" +
		"esac\n"
	path := filepath.Join(dir, "rex-"+mode)
	if err := WriteExecutable(path, []byte(fake), 0o755); err != nil {
		t.Fatalf("write the fake rex: %v", err)
	}
	return path
}

// TestScratchRexRefusesAServerItDidNotStart is the incident as a test: the
// harness reads back a server that is not the one it started, and the run must
// end there.
//
// THREE ROWS, BECAUSE A SERVER CAN BE THE WRONG ONE THREE WAYS, and the first
// draft of this arm had two — which left the two halves of guard 1 covering
// for each other. `fleet` is the incident itself: the box's default endpoint
// with the twelve sessions it was holding, so BOTH halves refuse it, and a
// mutant that deleted either one stayed green behind the other (MEASURED
// 2026-11-20, mutant 2 of this bead's battery: "the identity abort is
// removed" SURVIVED a two-row table). The other two rows each isolate one
// half — `fleetzero` is the wrong socket answering for an empty server, and
// `count` is the right socket claiming to hold twelve, which is what a reused
// socket path or a `--restore` that was not false would read like.
//
// WHAT EACH ROW ASSERTS IS THAT THE BODY NEVER RAN. The body writes a file;
// the file must not exist. An abort that printed a refusal and continued
// anyway would pass a test that only read the refusal's text — and printing it
// and continuing is precisely what happened on 2026-11-15.
func TestScratchRexRefusesAServerItDidNotStart(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ mode, saysWhy string }{
		// What the refusal has to NAME, so that a refusal for the other
		// half's reason cannot stand in for this row's.
		{"fleet", "rex/server.sock"},
		{"fleetzero", "rex/server.sock"},
		{"count", "session_count 12"},
	} {
		mode, saysWhy := row.mode, row.saysWhy
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			ran := filepath.Join(dir, "the-body-ran")
			fakePath := srxFakeRex(t, dir, mode)

			cmd := exec.Command("bash", "scripts/scratch-rex.sh", "--", "bash", "-c", "touch "+ran)
			cmd.Env = append(os.Environ(), "SCRATCH_REX_BIN="+fakePath)
			out, err := cmd.CombinedOutput()
			text := string(out)

			if err == nil {
				t.Fatalf("%s exited 0 over a server that is not the one it started — that is the incident:\n%s",
					scratchRexScript, text)
			}
			if code := cmd.ProcessState.ExitCode(); code != 70 {
				t.Errorf("exit %d, want 70 (the refusal's own code, told apart from 2 for a usage or apparatus failure):\n%s", code, text)
			}
			if _, statErr := os.Stat(ran); statErr == nil {
				t.Fatalf("THE BODY RAN. The refusal is printed and not acted on, which is the whole defect:\n%s", text)
			}
			if !strings.Contains(text, "REFUSING:") {
				t.Fatalf("the run failed without saying why — a reader of this output has to be told what answered:\n%s", text)
			}
			refusal := text[strings.Index(text, "REFUSING:"):]
			if !strings.Contains(refusal, saysWhy) {
				t.Errorf("the refusal does not name %q, so this row's own half of guard 1 is not what "+
					"stopped the run — the other half's refusal would read the same:\n%s", saysWhy, text)
			}
		})
	}
}

// TestScratchRexShimFencesWhatABodyMayDo holds guards 2 and 3 where arm 1
// cannot: at their CALL SITES. Arm 1 drives `recorded` and `names_a_server`
// as predicates, and a shim that stopped CONSULTING either of them would
// leave that table green — which is the hole the incident went through one
// level up, where the sanity read was taken and not acted on.
//
// The body here tries the four things live.sh's cleanup and sanity lines did,
// against a fake that would answer 0 for all of them, and writes its marker
// only if every one was refused. So the marker's absence is the shim failing
// to fence, and the harness's own exit says whether the run got that far.
func TestScratchRexShimFencesWhatABodyMayDo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fakePath := srxFakeRex(t, dir, "self")
	fenced := filepath.Join(dir, "all-four-refused")

	// Each call must be refused BY THE SHIM — its own sentence, not merely a
	// nonzero exit. MEASURED 2026-11-20: with the exit status alone, mutant 7
	// of this bead's battery ("the server verb is let through") SURVIVED,
	// because the call then reached the fake rex and failed THERE, and a
	// counter of failures cannot tell a fence from a broken command.
	refused := "refused() { o=$(\"$@\" 2>&1); case $o in *'" + srxShimSays + "'*) n=$((n+1)) ;; " +
		"*) echo \"NOT FENCED: $* -> $o\" ;; esac; }\n"
	body := "set +e\n" +
		"n=0; k=0\n" + refused +
		"refused rex -S unix:///somewhere/else/s.sock ls -q\n" +
		"refused rex --server=somelabel ls -q\n" +
		// The attached and clustered spellings of the same flag, which rex
		// takes identically (MEASURED 2026-11-20) and which an exact-word
		// test on `-S` passed straight through until this arm had them.
		"refused rex -Sunix:///somewhere/else/s.sock ls -q\n" +
		"refused rex -vSunix:///somewhere/else/s.sock ls -q\n" +
		"refused rex server run --listen unix:///somewhere/else/s.sock\n" +
		"refused rex kill\n" +
		"refused rex kill session:00000000-0000-0000-0000-000000000000\n" +
		// And the other half: this run's OWN session goes through.
		"sid=$(rex new probe --json | python3 -c 'import json,sys; print(json.load(sys.stdin)[\"session_id\"])')\n" +
		"grep -qxF -- \"$sid\" \"$SCRATCH_REX_RECORD\" || echo \"NOT RECORDED: $sid\"\n" +
		"o=$(rex kill \"$sid\" 2>&1)\n" +
		"case $o in *'" + srxShimSays + "'*) echo \"OWN SESSION REFUSED: $o\" ;; *) k=1 ;; esac\n" +
		"echo \"refused $n of 7, own session killable $k\"\n" +
		"[ \"$n\" = 7 ] && [ \"$k\" = 1 ] && touch " + fenced + "\n" +
		"exit 0\n"

	cmd := exec.Command("bash", "scripts/scratch-rex.sh", "--", "bash", "-c", body)
	cmd.Env = append(os.Environ(), "SCRATCH_REX_BIN="+fakePath)
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		t.Fatalf("the harness did not get as far as the body over a server that answered for itself "+
			"(%v) — guard 1 must PASS here, or this arm measures nothing:\n%s", err, text)
	}
	if _, statErr := os.Stat(fenced); statErr != nil {
		t.Fatalf("the shim let one of the seven past, or refused this run's OWN session. A body that can "+
			"name its own server, start a server, or kill an id this run did not create is a body that "+
			"can reach the fleet's server and empty it (ranger-base-uq81v); a fence that refuses the "+
			"run's own work instead is the one a later seat deletes:\n%s", text)
	}
	if !strings.Contains(text, "refused 7 of 7, own session killable 1") {
		t.Errorf("the body's own count did not reach this output, so the marker above is the only evidence:\n%s", text)
	}
}

// ─── arm 3: the harness's own -S discipline ─────────────────────────────────

// srxRexCalls returns every line of the harness that invokes the rex CLI
// through $REXBIN or $SCRATCH_REX_BIN — the two spellings of "the real
// binary" in that script — paired with its line number.
func srxRexCalls(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(scratchRexScript)
	if err != nil {
		t.Fatalf("read %s: %v — this pin is the harness's door and needs the harness", scratchRexScript, err)
	}
	var calls []string
	for i, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// At a COMMAND HEAD, not anywhere in the line. `[ -x "$REXBIN" ]`,
		// `export SCRATCH_REX_BIN="$REXBIN"` and the env prefixes the EXIT
		// trap puts in front of the SHIM all mention the variable and none of
		// them runs the binary — and a reader that cannot tell those apart
		// reds on the harness's own guards, which is where this started.
		var isCall bool
		for _, head := range srxCommandHeads(trimmed) {
			if head == `"$REXBIN"` || head == `"$SCRATCH_REX_BIN"` {
				isCall = true
			}
		}
		if !isCall {
			continue
		}
		calls = append(calls, fmt.Sprintf("%s:%d: %s", scratchRexScript, i+1, trimmed))
	}
	return calls
}

// TestScratchRexAddressesItsOwnSocketOnEveryCall holds guard 1's other half:
// the endpoint is PASSED and never inferred. Every invocation of the real rex
// binary in the harness carries `-S "$ENDPOINT"` (or `-S
// "$SCRATCH_REX_ENDPOINT"` in the shim), bar the one that CREATES the
// endpoint — `server run --listen` — and every one of them refuses autostart,
// so a scratch server that died is an honest failure and not an empty server
// started in its place (rex.go's head comment, item 3).
func TestScratchRexAddressesItsOwnSocketOnEveryCall(t *testing.T) {
	t.Parallel()
	calls := srxRexCalls(t)
	// The floor is the positive witness: a reader that stopped finding the
	// calls would leave this green over a sweep of nothing. Four when this
	// pin was written — the server launch, the identity read, the shim's
	// pass-through and the shim's record refresh.
	if len(calls) < 4 {
		t.Fatalf("found %d rex invocations in %s (4 when this pin was written) — the reader "+
			"is not finding them, so a clean result measures nothing:\n\t%s",
			len(calls), scratchRexScript, strings.Join(calls, "\n\t"))
	}
	var listens int
	for _, c := range calls {
		if strings.Contains(c, "server run --listen") {
			listens++
			if !strings.Contains(c, `"$ENDPOINT"`) {
				t.Errorf("the server is started on something other than $ENDPOINT, "+
					"so the socket the identity read checks is not the socket that was created:\n\t%s", c)
			}
			continue
		}
		if !strings.Contains(c, `-S "$ENDPOINT"`) && !strings.Contains(c, `-S "$SCRATCH_REX_ENDPOINT"`) {
			t.Errorf("a rex call in the harness does not name the scratch endpoint — "+
				"rex resolves XDG_RUNTIME_DIR and REX_SERVER for it NOT AT ALL (see this file's head "+
				"comment), so a call without -S addresses the fleet server:\n\t%s", c)
		}
		if !strings.Contains(c, "--autostart=false") {
			t.Errorf("a rex call in the harness lets autostart stand: a dead scratch server would be "+
				"replaced by an empty one and every read of it would answer \"no such session\" "+
				"(ADR 0076 §5's reclaim hazard):\n\t%s", c)
		}
	}
	if listens != 1 {
		t.Errorf("%d `server run --listen` lines in %s, want exactly 1 — the harness owns one server "+
			"and the shim refuses the verb", listens, scratchRexScript)
	}
}

// ─── arm 4: the tracked tree ────────────────────────────────────────────────

// srxUnaddressedRex reports whether a shell line invokes the rex CLI without
// naming a server. It is deliberately narrow: the invocation has to be a word
// `rex` at the head of a command, which is what a hand-rolled probe types and
// what the incident's script typed four times.
func srxUnaddressedRex(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	// A line that names a server, or that is about the shim's own fence,
	// has answered the question this reader asks.
	if strings.Contains(trimmed, "-S ") || strings.Contains(trimmed, "--server") ||
		strings.Contains(trimmed, "$SCRATCH_REX_ENDPOINT") {
		return false
	}
	for _, head := range srxCommandHeads(trimmed) {
		if head == "rex" {
			return true
		}
	}
	return false
}

// srxWrappers are the words that stand in FRONT of a command without being
// it. `POSSE_KEEP=x nohup rex server run …` is the incident's own launch line
// and `rex` is its command; a reader that stopped at `nohup` would have
// called that line clean, which the control arm below caught it doing.
var srxWrappers = map[string]bool{
	"nohup": true, "env": true, "exec": true, "command": true,
	"time": true, "timeout": true, "builtin": true, "eval": true,
}

// srxCommandHeads splits a shell line at the separators that start a new
// command and answers the first real word of each — past any assignment, any
// wrapper and any wrapper's flags. `for s in $(rex ls)` and
// `x=1 nohup rex kill "$s"` both put `rex` at a command head; the word `rex`
// inside a path, a variable name or English prose does not.
func srxCommandHeads(line string) []string {
	seps := []string{"&&", "||", "|", ";", "$(", "`", "(", "then", "do", "else"}
	parts := []string{line}
	for _, sep := range seps {
		var next []string
		for _, p := range parts {
			next = append(next, strings.Split(p, sep)...)
		}
		parts = next
	}
	var heads []string
	for _, p := range parts {
		for _, f := range strings.Fields(p) {
			// An assignment, a redirect, a wrapper or a wrapper's flag is
			// not the command.
			if strings.Contains(f, "=") || strings.HasPrefix(f, ">") ||
				strings.HasPrefix(f, "<") || strings.HasPrefix(f, "-") ||
				srxWrappers[f] {
				continue
			}
			heads = append(heads, f)
			break
		}
	}
	return heads
}

// TestTheUnaddressedRexDetectorFlagsTheLinesItIsFor is the control arm.
// Without it, arm 4's sweep is a green light over a reader that answers no to
// everything — which is how the incident's own sanity line got printed and
// passed over.
func TestTheUnaddressedRexDetectorFlagsTheLinesItIsFor(t *testing.T) {
	t.Parallel()
	// Verbatim from live.sh at 08:29 (the transcript is on ranger-base-uq81v),
	// bar the `--autostart=false` it did carry, which is not the flag that
	// would have saved it.
	mustFlag := []string{
		`rex --autostart=false whoami --json | grep -E '"endpoint"'`,
		`rex --autostart=false ls --json --short=false`,
		"  rex --autostart=false kill \"$s\"",
		"for s in $(rex --autostart=false ls -q); do",
		"POSSE_KEEP=x nohup rex server run --listen \"unix://$SD/rex/server.sock\" &",
		"\trex kill \"$s\" || true",
	}
	for _, line := range mustFlag {
		if !srxUnaddressedRex(line) {
			t.Errorf("the detector did not flag a rex call that names no server, so arm 4's sweep measures nothing:\n\t%s", line)
		}
	}
	mustPass := []string{
		// The harness's own spellings.
		`"$REXBIN" --color never -S "$ENDPOINT" --autostart=false whoami --json`,
		`"$SCRATCH_REX_BIN" --color never -S "$SCRATCH_REX_ENDPOINT" --autostart=false "$@"`,
		`rex -S "$SCRATCH_REX_ENDPOINT" ls -q`,
		// Prose, paths and names that carry the word.
		"# rex does not read XDG_RUNTIME_DIR for its endpoint",
		"REXBIN=${SCRATCH_REX_BIN:-/usr/bin/rex}",
		"\tRHQ_REX_BIN=\"$SD/bin/rex\"",
		"ln -s \"$SELF\" \"$SD/bin/rex\"",
		`grep -n "rex kill" scripts/scratch-rex.sh`,
		"echo \"  rex      : $REXBIN\"",
	}
	for _, line := range mustPass {
		if srxUnaddressedRex(line) {
			t.Errorf("the detector flagged a line that addresses a server or calls nothing — a reader this noisy gets waived:\n\t%s", line)
		}
	}
}

// TestNoTrackedShellScriptCallsRexWithoutNamingItsServer sweeps what this
// repo tracks. The rule is fix direction 1 of ranger-base-uq81v at the one
// place a pin can read it: a rex call that names no server addresses the box's
// default socket, which on this instance is the fleet's own server holding
// every live seat.
//
// The harness is the exemption and it is the only one: its shim is where the
// endpoint comes from, so the lines that pass it on are read by arm 3 instead.
func TestNoTrackedShellScriptCallsRexWithoutNamingItsServer(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v — this pin needs the checkout to name what the tree holds", err)
	}
	tracked := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(tracked) < 100 {
		t.Fatalf("git tracks %d files — the listing failed, so a clean result here is not evidence", len(tracked))
	}

	var scanned int
	var hits []string
	for _, rel := range tracked {
		if rel == "" || rel == scratchRexScript {
			continue
		}
		if !strings.HasSuffix(rel, ".sh") && filepath.Base(rel) != "Makefile" {
			continue
		}
		b, err := os.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		scanned++
		for i, line := range strings.Split(string(b), "\n") {
			if srxUnaddressedRex(line) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
		}
	}
	// patternkill_qa_test.go's floor, for its reason: 38 tracked .sh files
	// plus the Makefile at the time of writing, and a floor this close to the
	// census catches a sweep that silently NARROWS — which reads exactly like
	// a clean result.
	if scanned < 30 {
		t.Fatalf("only %d shell files swept in the whole tracked tree — the sweep is not finding them, "+
			"so a clean result measures nothing", scanned)
	}
	if len(hits) > 0 {
		t.Fatalf("a tracked script calls the rex CLI without naming a server, which addresses this box's "+
			"DEFAULT socket — the fleet's own server, holding every live seat (ranger-base-uq81v: eight "+
			"destroyed that way on 2026-11-15). Run the probe under %s, which starts a scratch server and "+
			"makes the fleet's unreachable from inside it, or pass -S yourself:\n\t%s",
			scratchRexScript, strings.Join(hits, "\n\t"))
	}
}
