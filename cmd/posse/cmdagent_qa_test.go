package main

// ranger-base-k2olp at the argv boundary: `posse new --agent <p> --cmd
// "<line>"` must refuse, because posse ran the persona's rendered line and
// threw the typed one away without a word.
//
// Asserted over the BUILT BINARY and not over parseNewFlags, for
// TestNewModelFlagRefusals' reason (rangerhq-qv5's shape): the claim the bead
// is about is that an unattended script typing the pair gets a NONZERO exit
// and no session, and "created nothing, exited 1" is a claim about a process.
// parseNewFlags cannot be called from a test at all — its refusal path is
// `die`, which exits this process.
//
// RHQ_HERDR_BIN points at a path that does not exist, so a launch that got
// past the refusal would fail loudly rather than quietly reaching a herdr.
// That is also what makes the third row below a real assertion and not a
// tautology: `--cmd` alone must still be ACCEPTED, so it must get past the
// parser and die at the missing herdr instead of at a flag refusal.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRefusesCmdWithAgent(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)

	// The pair, in both typed orders: an operator who typed the line the
	// other way round is the same operator, and a check that reads only
	// one order is green on half the misuse.
	for _, args := range [][]string{
		{"new", "s", "--agent", "architect", "--cmd", "claude --mcp-config /tmp/x.json"},
		{"new", "s", "--cmd", "claude --mcp-config /tmp/x.json", "--agent", "architect"},
	} {
		out, code := runRhq(t, bin, env, args...)
		if code != 1 {
			t.Errorf("posse %s: exit %d, want 1 — an unattended check typing this pair must be told, and a warning on stderr is read by nobody\n%s",
				strings.Join(args, " "), code, out)
		}
		// The rule, not just a refusal: the message has to say WHY the
		// typed line could not run, or the reader's next move is to type it
		// again with different quoting.
		for _, want := range []string{"--cmd", "--agent", "rendered", "PID"} {
			if !strings.Contains(out, want) {
				t.Errorf("posse %s: the refusal does not name %q, so it does not state the rule:\n%s",
					strings.Join(args, " "), want, out)
			}
		}
		// AT PARSE TIME, which is the half an exit code cannot show. This
		// fixture's home has no personas, so `no such agent` is posse
		// reading the home — proof the line got past the flags. A refusal
		// downgraded to a warning prints the same message, exits 1 on that
		// same missing persona, and would pass every assertion above it
		// (MEASURED, grading this pin against that mutant).
		if strings.Contains(out, "no such agent") {
			t.Errorf("posse %s: posse went on to resolve the persona, so the pair was not refused at the flag boundary — "+
				"a warning that is read by nobody is what the bead was filed about:\n%s", strings.Join(args, " "), out)
		}
	}

	// --cmd alone is a plain session's own line and stays legal. It gets
	// past the parser and dies at the herdr that is not there, which is a
	// different exit for a different reason — so this row also proves the
	// rows above refused at the FLAG and not at something every `posse new`
	// in this fixture would hit.
	//
	// Its own RHQ_HOME, because getting past the parser is the whole claim
	// and a launch that got that far WRITES: it took the dispatch launch
	// lock before it failed. Sharing the home above would put that file
	// under the no-state assertion, which would then be measuring this row
	// instead of the refusals (MEASURED: state/dispatch-launch.lock).
	lone := t.TempDir()
	loneEnv := append(os.Environ(),
		"RHQ_HOME="+lone,
		"RHQ_HERDR_BIN="+filepath.Join(lone, "herdr-must-not-run"),
	)
	out, code := runRhq(t, bin, loneEnv, "new", "s", "--cmd", "sh")
	if code == 0 {
		t.Errorf("posse new s --cmd sh: exit 0 against a herdr that does not exist — this fixture is not launching what it thinks it is:\n%s", out)
	}
	if strings.Contains(out, "--cmd cannot be given with --agent") {
		t.Errorf("posse new s --cmd sh: refused as if --agent had been typed, so the pair check reads one flag:\n%s", out)
	}

	// The whole point of refusing at parse time: nothing on the host moved.
	// This walks the home only the two refusals ran against.
	var left []string
	filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			left = append(left, p)
		}
		return nil
	})
	if len(left) > 0 {
		t.Errorf("a refused launch must leave no state behind: %q", left)
	}
}

// The operator-visible half: the catalog entry for `--cmd` says the pair is
// refused and why. Pinned because the refusal's whole value is that someone
// writing a verify script reads the rule BEFORE they type the line — the
// refusal itself only reaches them after the run they already trusted.
func TestCmdHelpNamesTheAgentRefusal(t *testing.T) {
	block := flagHelpBlock(t, helpText(t), "      --cmd ")
	for _, want := range []string{"--agent", "rendered", "PID"} {
		if !strings.Contains(block, want) {
			t.Errorf("the --cmd help does not name %q:\n%s", want, block)
		}
	}
}

// flagHelpBlock is one usage-catalog flag entry: its own line and every
// continuation indented under it, up to the next flag. modelHelpBlock's
// body, with the flag to find as a parameter (its comment explains why a
// trimmed prefix test would end the block at its second line).
func flagHelpBlock(t *testing.T, out, flag string) string {
	t.Helper()
	var b strings.Builder
	in := false
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "      --") {
			if in {
				break
			}
			in = strings.HasPrefix(ln, flag)
		}
		if in {
			b.WriteString(ln + "\n")
		}
	}
	if b.Len() == 0 {
		t.Fatalf("the usage catalog no longer has a %q entry — this pin is reading nothing", flag)
	}
	return b.String()
}
