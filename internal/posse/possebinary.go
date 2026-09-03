package posse

// Which posse is running, and which posse the next `posse` in this shell
// would be (ranger-base-39jnl).
//
// THE INCIDENT. 2026-09-02, the work-laptop install steps were run on the
// fleet box by mistake: `brew install ranger360ai/tap/posse` put a release
// from three days earlier at /opt/homebrew/bin/posse, which precedes
// ~/.local/bin on the fleet PATH. From then on every `posse` — including two
// watch relaunches — ran the release binary rather than the promoted one,
// and because its PromotedPaths predated `runtimes` joining the set (ADR
// 0039 D2) the launch verify refused EVERY dispatched launch for ~90
// minutes. Nothing on any surface said which binary was answering.
//
// This is the verify-bd-pin shape (scripts/verify-bd-pin.sh rows 1-2) turned
// on posse itself, and it is a READING, not a control: it prints, it warns,
// and it never decides. A box legitimately runs a posse that is not first on
// PATH — a `go run`, a checkout build, a cage's inner binary — and a warning
// an operator can read beats a refusal that stops a fleet over a `go run`.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gateShimMarker is the fixed half of the header renderShim stamps on every
// shim, present regardless of persona — the same needle
// scripts/verify-bd-pin.sh greps for, and for its reason: a persona name in
// the path would drift the day gates move, while the header is the shim's
// own claim about itself.
const gateShimMarker = "rangerhq-9ha"

// PosseBinary is one process's answer to "which posse is this, and which
// posse would PATH give me?".
type PosseBinary struct {
	Exe     string // this process's own binary, symlinks resolved
	Version string // VersionString() — what this process IS, not what it found
	First   string // what PATH resolves `posse` to, "" when nothing does
	Shim    string // the gate shim First went through, "" when it was not one
	Err     error  // PATH could not be searched, or the exe could not be named
}

// RunningPosse reads both ends. It runs no posse and asks no version of
// anything: the running binary's version is compiled in, and the PATH end is
// a file read.
func RunningPosse() PosseBinary {
	return runningPosse(os.Executable, func() (string, error) { return exec.LookPath("posse") })
}

// runningPosse is RunningPosse with its two lookups handed in, so a pin can
// put a stale binary "on PATH" without touching the process environment —
// PATH is one variable shared by every goroutine in the package's parallel
// suite, and a test that swaps it answers questions no other test asked
// (the ranger-base-i7fa rule, applied to the test side).
func runningPosse(executable func() (string, error), look func() (string, error)) PosseBinary {
	b := PosseBinary{Version: VersionString()}
	exe, err := executable()
	if err != nil {
		b.Err = err
		return b
	}
	b.Exe = resolveLinks(exe)

	first, err := look()
	if err != nil {
		// Not on PATH at all is a fact, not an error: a `go run` or a
		// cage's inner binary is nobody's mistake. Nothing to compare.
		return b
	}
	first = resolveLinks(first)
	// A persona session's PATH leads with its own gate shim dir (ADR 0002
	// §3), and a PID that denies `Bash(posse promote:*)` has a `posse` shim
	// in it — so the raw comparison would warn in every such session on a
	// box whose PATH was perfectly correct, which is exactly the false
	// positive that made row 2 of verify-bd-pin useless until ranger-base-43v1.
	// What the shim EXECS is read out of the shim itself, frozen in at
	// render time, never re-derived from today's PATH.
	if target, ok := gateShimTarget(first); ok {
		b.Shim = first
		if target == "" {
			b.Err = Die("%s is a posse gate shim whose exec target does not parse", AbbrevHome(first))
			return b
		}
		first = resolveLinks(target)
	}
	b.First = first
	return b
}

// resolveLinks is EvalSymlinks with the original kept when it fails — a
// path that cannot be resolved is still the honest name of what was found.
func resolveLinks(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		if abs, err := filepath.Abs(r); err == nil {
			return abs
		}
		return r
	}
	return p
}

// gateShimTarget reads a posse gate shim's own last line — `exec '<real>'
// "$@"` — and reports what it execs. The bool is whether the file IS a shim;
// an empty target with ok=true is a shim whose exec line did not parse,
// which is a finding rather than a "no".
func gateShimTarget(p string) (string, bool) {
	f, err := os.Open(p)
	if err != nil {
		return "", false
	}
	// The header only, exactly as verify-bd-pin.sh reads it (`head -2`): a
	// binary that happens to carry the marker deeper down is not a shim —
	// and the candidate here is usually a 20MB Go binary, which nothing
	// should be slurping to answer a header question.
	var head [512]byte
	n, _ := io.ReadFull(f, head[:])
	f.Close()
	lines := strings.SplitN(string(head[:n]), "\n", 3)
	if len(lines) > 2 {
		lines = lines[:2]
	}
	if !strings.Contains(strings.Join(lines, "\n"), gateShimMarker) {
		return "", false
	}
	// It is a shim, so it is a few KB of sh and reading it whole is cheap.
	body, err := os.ReadFile(p)
	if err != nil {
		return "", true
	}
	for _, ln := range strings.Split(string(body), "\n") {
		if rest, ok := strings.CutPrefix(ln, "exec '"); ok {
			if target, ok := strings.CutSuffix(rest, `' "$@"`); ok {
				return strings.ReplaceAll(target, `'\''`, "'"), true
			}
		}
	}
	return "", true
}

// Line is the identity, always printable: which binary is running and what
// it calls itself.
func (b PosseBinary) Line() string {
	exe := b.Exe
	if exe == "" {
		exe = "(this binary cannot name itself)"
	} else {
		exe = AbbrevHome(exe)
	}
	return fmt.Sprintf("posse binary · %s · %s", exe, b.Version)
}

// Shadowed is whether PATH would answer `posse` with a different binary than
// the one running. False when either end is unknown: an unanswerable
// question is not a finding.
func (b PosseBinary) Shadowed() bool {
	return b.Exe != "" && b.First != "" && b.First != b.Exe
}

// Warning is the line an operator needs when PATH disagrees, or "" when it
// does not. It names both paths and the command that shows the rest, because
// "which posse" is the next thing anyone types.
func (b PosseBinary) Warning() string {
	if b.Err != nil {
		return fmt.Sprintf("warning: cannot tell which posse PATH resolves: %v", b.Err)
	}
	if !b.Shadowed() {
		return ""
	}
	via := ""
	if b.Shim != "" {
		via = fmt.Sprintf(" (through the gate shim %s)", AbbrevHome(b.Shim))
	}
	return fmt.Sprintf("warning: PATH resolves posse to %s%s, not the running %s — the next `posse` in this shell is the other one; `which -a posse` shows the order (a brew keg ahead of ~/.local/bin is how ranger-base-39jnl happened)",
		AbbrevHome(b.First), via, AbbrevHome(b.Exe))
}

// ReportPosseBinary prints the identity and, when PATH disagrees, the
// warning. One writer, so `posse status` and the watch preamble print the
// same bytes and one grep finds both.
func ReportPosseBinary(w io.Writer) PosseBinary {
	b := RunningPosse()
	b.report(w)
	return b
}

func (b PosseBinary) report(w io.Writer) {
	fmt.Fprintln(w, b.Line())
	if warn := b.Warning(); warn != "" {
		fmt.Fprintln(w, warn)
	}
}
