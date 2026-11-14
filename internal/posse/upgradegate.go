package posse

// The bd argv gate copy arm of `posse upgrade` — ADR 0092 D5, and D6's gate
// word.
//
// WHAT THIS ARM IS FOR. `scripts/bd-argv-gate.{sh,py}` in this repo is
// SOURCE. What fences this box is the copy a PreToolUse hook names in the
// operator's Claude settings, and posse renders no box-wide hook (ADR 0015
// §3), so a commit to either file moves nothing and nothing anywhere
// notices: the copy goes stale at the next landing and stays stale silently.
// That is ranger-base-2tnbv's class, and it had been an operator ask forever
// although the source, the review path and the blast class are the binary's
// (D5, and the "Gate copy out of scope" alternative it rejects).
//
// WHAT STAYS THE OPERATOR'S. The settings file that NAMES the hook. This arm
// reads it and never writes it (D5, D7). So the arm refreshes a pair that is
// already installed somewhere this box's hook reaches, and refuses — writing
// nothing, and naming the edit — when the hook's path does not resolve or
// has moved. Creating a gate install where there is none is a settings
// decision, and a verb that made it would be deciding which Bash calls on
// this box are fenced.
//
// WHY IT READS THE LOCATION AND THE VERDICT OUT OF
// scripts/verify-gate-freshness.sh RATHER THAN ANSWERING EITHER ITSELF.
// Both questions already have exactly one answer in this tree, in the script
// that exists to ask them: where the hook reaches (its own shlex read of the
// operator's settings — "not `the file at ~/.config/posse/gate/` — the file
// the hook REACHES") and whether the installed gate works (its three
// behaviour arms: an allowed verb gets through, a denied verb is refused by
// the PARSER and not by the fail-closed fallback, and an unrelated command
// is untouched). A second reader of the settings entry, or a second copy of
// the three arms, would drift from those silently — and the direction it
// drifts in is this verb installing a box-wide fence and calling it green.
// The script grew two read-only modes for this (`--print-wrappers`,
// `--behaviour-only`); it still installs nothing, which is its own header's
// standing refusal and the reason the writing half is here, in a binary a
// persona's worktree cannot edit.
//
// WHICH SETTINGS FILE, said out loud because it is an environment variable
// and not a constant. The script reads `$CLAUDE_CONFIG_DIR` else
// `$HOME/.claude`, and posse TYPES that variable onto every seat it hires,
// at ClaudeConfigDirIn's answer for the operator's own home (credential.go).
// So a delegate seat reads the operator's entry — MEASURED 2026-11-14 on
// this box, `CLAUDE_CONFIG_DIR=/home/posse/.claude` in a dispatched seat,
// and the resolution named this box's live gate dir. A seat whose view of
// that directory is a sandbox COPY still reads the same absolute wrapper
// path out of it, because what the arm takes from the entry is a path and
// not a file; a seat that cannot see the entry at all refuses the arm and
// writes nothing, which is the safe direction and the one D5 asks for.
//
// WHY THE SOURCE IS <sha>'s WORKTREE AND NOT THE SOURCE CHECKOUT'S HEAD.
// This arm runs inside an upgrade, so the pair it installs is the pair of
// the commit whose suite just passed and whose binary is now in place — the
// same guardrails, as D5 puts it. The source checkout's `main` can be ahead
// of origin/main by landings nobody pushed, and that is the guardrail-1
// breach this whole verb exists not to commit. A run where the two agree
// leaves the box in the state `make verify-gate-freshness` calls fresh; a
// run where main is ahead leaves it one landing behind, which that script
// will say in the words it has always said it in.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The gate word of D6's last line, as amended at acceptance
// (ranger-base-estjw). One word per outcome of this arm, and the four are
// exhaustive over what it can do:
//
//	unchanged  the installed copy was already byte-identical to <sha>'s,
//	           nothing was written, and the behaviour arms were green.
//	installed  both files were swapped by rename, and the arms were green.
//	refused    the arm wrote nothing because the hook path did not resolve
//	           or had moved, or it wrote and an arm went red, so both .prev
//	           copies are back.
//	unknown    the behaviour arms measured nothing (verify-gate-freshness's
//	           own exit 2).
//
// `fresh` and `stale` were the verify-only words of the D1-D3 build, which
// had no arm to report on; D6 retires them.
const (
	upgradeGateUnchanged = "unchanged"
	upgradeGateInstalled = "installed"
	upgradeGateRefused   = "refused"
	upgradeGateUnknown   = "unknown"
)

// upgradeGatePair is the two files, WRAPPER FIRST. The order is load-bearing
// twice: the wrapper is what the hook names and what every path here is
// derived from, and it is what the behaviour arms probe.
var upgradeGatePair = []string{"bd-argv-gate.sh", "bd-argv-gate.py"}

// upgradeGateScript is the verifier in <sha>'s worktree — the commit's own
// copy, for the reason the header gives: this arm installs that commit's
// gate, so the rule for finding the hook is that commit's too.
func upgradeGateScript(wt string) string {
	return filepath.Join(wt, "scripts", "verify-gate-freshness.sh")
}

// upgradeGateWrappers asks `verify-gate-freshness --print-wrappers` where
// the gate is installed. It returns the absolute wrapper paths, everything
// the script said (for the report, which is where a refusal's reason has to
// land), and the child's error.
//
// FILTERED BY SUFFIX, because the script's own "nothing measured" line goes
// to stdout beside the paths — that is its one channel for the case this
// arm most needs to distinguish, and reading it as a path would make a
// refusal look like a destination. A line that names the wrapper but is not
// absolute is kept apart rather than dropped: a hook command the shell would
// resolve against its own cwd is a settings bug, and a refusal that cannot
// name the line it refused is one the operator cannot act on.
var upgradeGateWrappers = func(wt string) (abs, relative []string, out string, err error) {
	var buf bytes.Buffer
	cmd := protocolCmd(exec.Command(upgradeGateScript(wt), "--print-wrappers"))
	cmd.Dir = wt
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err = cmd.Run()
	for _, ln := range strings.Split(buf.String(), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || !strings.HasSuffix(ln, upgradeGatePair[0]) {
			continue
		}
		if filepath.IsAbs(ln) {
			abs = append(abs, ln)
		} else {
			relative = append(relative, ln)
		}
	}
	return abs, relative, strings.TrimRight(buf.String(), "\n"), err
}

// upgradeGateBehaviour runs the three arms against the installed wrapper and
// streams the script's report to the caller's writer, so what the delegate
// posts on the install bead carries the verdict in the verifier's own words.
//
// --wrapper, so the arms probe the file this run just wrote rather than
// re-resolving the settings: between the resolution above and here, this arm
// has written to that path, and a second read of the settings would be a
// second chance for the two halves to disagree about which file they mean.
var upgradeGateBehaviour = func(w io.Writer, wt, wrapper string) error {
	return upgradeStep(w, wt, upgradeGateScript(wt), "--behaviour-only", "--wrapper", wrapper)
}

// upgradeGateSettingsEdit is what a refusal hands the operator: the file
// that names the gate, the entry this arm reads out of it, and the one
// command that puts the pair where that entry points. Printed, never
// applied — D5 and D7 keep settings.json the operator's, and this verb
// writes none of it.
//
// `dir` is the gate directory when the hook named one and "" when it named
// nothing resolvable. There is no default: hard-coding this box's
// ~/.config/posse/gate would be a path to go wrong on a box that moved it,
// and the whole of this arm's resolution is that it reads where the hook
// actually points.
func upgradeGateSettingsEdit(repo, dir string) string {
	shown := "<the directory your PreToolUse hook names>"
	if dir != "" {
		shown = AbbrevHome(dir)
	}
	home, _ := os.UserHomeDir()
	var b strings.Builder
	fmt.Fprintf(&b, "  the settings this arm reads: %s/settings.json (and settings.local.json; $CLAUDE_CONFIG_DIR moves both)\n",
		AbbrevHome(ClaudeConfigDirIn(home)))
	fmt.Fprintf(&b, "  the entry it looks for is a PreToolUse hook whose command names the wrapper:\n")
	fmt.Fprintf(&b, "      \"hooks\": {\"PreToolUse\": [{\"matcher\": \"Bash\", \"hooks\": [{\"type\": \"command\", \"command\": \"'%s/bd-argv-gate.sh'\"}]}]}\n", shown)
	fmt.Fprintf(&b, "  and the pair goes there by hand, from the source checkout:\n")
	fmt.Fprintf(&b, "      install -m 0755 %s %s %s/\n",
		filepath.Join(AbbrevHome(repo), "scripts", upgradeGatePair[0]),
		filepath.Join(AbbrevHome(repo), "scripts", upgradeGatePair[1]),
		shown)
	fmt.Fprintf(&b, "  this verb writes no settings and installs no gate it cannot already find (ADR 0092 D5, D7).\n")
	return b.String()
}

// upgradeGateRefuse prints one refusal and answers with D6's word for it.
// Every refusal of this arm goes through here, so none of them can forget
// the settings edit — a refusal whose remedy the reader has to go and look
// up is a refusal they will work around.
func upgradeGateRefuse(w io.Writer, repo, dir, why string) string {
	fmt.Fprintf(w, "  gate REFUSED — %s\n", why)
	fmt.Fprint(w, upgradeGateSettingsEdit(repo, dir))
	fmt.Fprintf(w, "  the binary arm above is unaffected: this arm writes the gate copy and nothing else (ADR 0092 D5).\n")
	return upgradeGateRefused
}

// upgradeGateArm is D5 whole: resolve where the gate is installed, install
// <sha>'s pair there when it differs, and judge the result by behaviour —
// restoring both .prev copies when an arm goes red.
//
// It NEVER returns an error. The binary is already in place and has already
// been judged by D4 before this runs; a gate copy that could not be
// refreshed is a thing the operator must know about and not a reason to fail
// an install that has happened (the Makefile's own words about this
// script's epilogue). What it returns instead is the one word D6's last line
// carries, which is the record the delegate posts on the bead.
func upgradeGateArm(w io.Writer, wt string, plan upgradePlan) string {
	abs, relative, out, err := upgradeGateWrappers(wt)
	// LABELLED, because this block is the destination on a good run and the
	// reason on a refusal, and a bare path in a long report reads as
	// neither.
	fmt.Fprintf(w, "  the gate this box's hook reaches (verify-gate-freshness --print-wrappers):\n")
	if out == "" {
		out = "(nothing)"
	}
	for _, ln := range strings.Split(out, "\n") {
		fmt.Fprintf(w, "    %s\n", ln)
	}
	switch {
	case len(relative) > 0 && len(abs) == 0:
		return upgradeGateRefuse(w, plan.Repo, "",
			fmt.Sprintf("the hook command names %q, which is not an absolute path — what it reaches depends on the cwd of whatever ran it, so there is no one file to refresh", relative[0]))
	case err != nil || len(abs) == 0:
		return upgradeGateRefuse(w, plan.Repo, "",
			"the PreToolUse hook that names bd-argv-gate.sh did not resolve to an installed wrapper (the line above is the verifier's own reason)")
	case len(abs) > 1:
		return upgradeGateRefuse(w, plan.Repo, "",
			fmt.Sprintf("the settings name %d wrappers (%s) — this arm refreshes the one pair the hook reaches and will not choose between them", len(abs), strings.Join(abs, ", ")))
	}
	wrapper := abs[0]
	dir := filepath.Dir(wrapper)

	// THE PATH MUST ALREADY HOLD THE WRAPPER. Lstat and not Stat: a symlink
	// is somebody's deliberate arrangement — most plausibly into a checkout
	// — and a write-beside-then-rename over it would silently replace that
	// arrangement with a copy. A path with nothing at it is the "points
	// somewhere new" case D5 names: the hook reaches no gate, every Bash
	// call on this box is running unfenced, and which directory that should
	// be is a settings question with an operator's answer.
	st, lerr := os.Lstat(wrapper)
	switch {
	case lerr != nil:
		return upgradeGateRefuse(w, plan.Repo, dir,
			fmt.Sprintf("the hook names %s and there is no file there (%v) — the gate this box runs with is not installed where its own hook points, which is a settings edit and not a copy", AbbrevHome(wrapper), lerr))
	case st.Mode()&os.ModeSymlink != 0:
		return upgradeGateRefuse(w, plan.Repo, dir,
			fmt.Sprintf("the hook names %s, which is a symlink — whatever it points at is somebody's arrangement, and replacing it with a copy is not this arm's to do", AbbrevHome(wrapper)))
	case !st.Mode().IsRegular():
		return upgradeGateRefuse(w, plan.Repo, dir,
			fmt.Sprintf("the hook names %s, which is not a regular file (%s)", AbbrevHome(wrapper), st.Mode()))
	}

	// <sha>'s own pair. A commit whose tree is missing either file is not a
	// commit this arm can install from, and saying so beats writing half a
	// gate.
	src := map[string][]byte{}
	for _, f := range upgradeGatePair {
		body, rerr := os.ReadFile(filepath.Join(wt, "scripts", f))
		if rerr != nil {
			return upgradeGateRefuse(w, plan.Repo, dir,
				fmt.Sprintf("%s carries no scripts/%s (%v) — there is nothing to install", shortSha(plan.To), f, rerr))
		}
		src[f] = body
	}

	// ONLY WHEN THE INSTALLED COPY DIFFERS (D5). An unreadable or absent
	// installed file counts as differing: the pair is what the hook
	// reaches, and half a pair is a gate that fails closed on every bd call
	// in the fleet.
	same := true
	for _, f := range upgradeGatePair {
		cur, rerr := os.ReadFile(filepath.Join(dir, f))
		if rerr != nil || !bytes.Equal(cur, src[f]) {
			same = false
		}
	}
	if same {
		fmt.Fprintf(w, "  the installed gate at %s is already %s's — nothing written (ADR 0092 D5)\n", AbbrevHome(dir), shortSha(plan.To))
		return upgradeGateJudge(w, wt, wrapper, upgradeGateUnchanged, nil)
	}

	// KEEP BOTH, even the one that matched (D5). The pair is one fence, and
	// a rollback that puts one file back is a rollback to a combination
	// nobody ran.
	for _, f := range upgradeGatePair {
		p := filepath.Join(dir, f)
		body, rerr := os.ReadFile(p)
		if rerr != nil {
			// Nothing is written yet, so this refusal costs the box
			// nothing: the gate it had is the gate it keeps.
			return upgradeGateRefuse(w, plan.Repo, dir,
				fmt.Sprintf("cannot read %s to keep a rollback copy (%v) — nothing was written, and this arm does not swap a pair it could not keep", AbbrevHome(p), rerr))
		}
		if werr := WriteExecutable(upgradePrevPath(p), body, 0o755); werr != nil {
			return upgradeGateRefuse(w, plan.Repo, dir,
				fmt.Sprintf("cannot write %s, so there would be nothing to roll back to (%v) — nothing was written", AbbrevHome(upgradePrevPath(p)), werr))
		}
	}

	// BEFORE the swap, for D4's reason one directory over: the reader who
	// needs these two lines most is the one whose next bd call is the one
	// that broke.
	fmt.Fprintf(w, "  gate rollback, if the installed gate misbehaves:\n")
	for _, f := range upgradeGatePair {
		fmt.Fprintf(w, "      %s\n", upgradeRollbackCmd(filepath.Join(dir, f)))
	}
	for _, f := range upgradeGatePair {
		p := filepath.Join(dir, f)
		if werr := upgradeGateSwap(p, src[f]); werr != nil {
			fmt.Fprintf(w, "  gate: %v\n", werr)
			upgradeGateRestore(w, dir)
			return upgradeGateRefuse(w, plan.Repo, dir,
				fmt.Sprintf("the swap of %s failed, so both files are back as they were", AbbrevHome(p)))
		}
		fmt.Fprintf(w, "  installed: %s (%s)\n", AbbrevHome(p), shortSha(plan.To))
	}
	return upgradeGateJudge(w, wt, wrapper, upgradeGateInstalled, func() { upgradeGateRestore(w, dir) })
}

// upgradeGateJudge runs the three behaviour arms and turns their verdict
// into D6's word, undoing the arm's own writes on a red.
//
// `green` is the word a passing reading earns, which is the one thing the
// two call sites differ by — whether this run wrote anything. `restore` is
// nil when it did not, and that case is a word rather than a repair for a
// reason worth saying out loud: a gate that is already <sha>'s and does not
// behave is broken UNDER its bytes (no python3, a bad mode, a settings entry
// pointing at something unexpected), and reinstalling the same bytes is the
// one prescription that cannot work. The script says so in those words, and
// this arm has nothing of its own to put back.
//
// EXIT 2 IS `unknown` AND LEAVES THE INSTALL STANDING. The arms measured
// nothing, which is not evidence of a break — and the paths that answer 2
// here are box properties the installed bytes cannot cause: no python3 for
// the wrapper to run, or a signalled probe. A gate whose interpreter is gone
// is equally dead on either version of the pair, so rolling back would buy
// nothing and would hide, behind a word that means "I measured nothing",
// the fact that the newer copy is the one in place. The .prev pair is beside
// it and the rollback lines are already printed.
func upgradeGateJudge(w io.Writer, wt, wrapper, green string, restore func()) string {
	err := upgradeGateBehaviour(w, wt, wrapper)
	switch upgradeExitCode(err) {
	case 0:
		return green
	case 1:
		if restore == nil {
			fmt.Fprintf(w, "  gate REFUSED — the installed gate is already this commit's and an arm above is red, so the break is under the files and not in their bytes. Nothing was written and there is nothing of this run's to put back.\n")
			return upgradeGateRefused
		}
		fmt.Fprintf(w, "  an arm above is red — restoring the gate this box was running (ADR 0092 D5)\n")
		restore()
		return upgradeGateRefused
	default:
		fmt.Fprintf(w, "  gate unknown — the behaviour arms measured nothing (%v). The pair in place is this run's and the .prev copies are beside it.\n", err)
		return upgradeGateUnknown
	}
}

// upgradeGateSwap writes one file beside its target and renames it over,
// which is the swap D3.5 measured for the binary and is right here for a
// second reason: this file is exec'd by a PreToolUse hook on EVERY Bash call
// of every seat on this box, so the window install(1) leaves — the target
// absent or short — is a window in which the fleet's bd calls fail closed or
// run unfenced.
//
// WriteExecutable for execwrite.go's reason, and this is the site that
// reason was written for: the fence is exec'd continuously, so a write
// descriptor a sibling fork inherits is an ETXTBSY a hook would answer with.
func upgradeGateSwap(target string, body []byte) error {
	staged := target + ".new"
	if err := WriteExecutable(staged, body, 0o755); err != nil {
		return Die("cannot stage %s: %v", AbbrevHome(staged), err)
	}
	if err := os.Rename(staged, target); err != nil {
		os.Remove(staged)
		return Die("cannot swap %s into place: %v", AbbrevHome(target), err)
	}
	return nil
}

// upgradeGateRestore puts both .prev copies back, by the same
// write-beside-then-rename the install used and for the same reason.
//
// BOTH, always, and never only the one that was written: see the keep above.
// The .prev files are LEFT WHERE THEY ARE, as upgradeRollback leaves the
// binary's — after this the installed pair and the .prev pair hold the same
// bytes, which costs two small files of disk and keeps the rollback lines
// this run printed true for whoever reads the report afterwards.
//
// A failure here is printed and nothing else: there is no further fallback
// to take, and the thing the operator needs is the two commands, which are
// already in the report above.
func upgradeGateRestore(w io.Writer, dir string) {
	for _, f := range upgradeGatePair {
		p := filepath.Join(dir, f)
		prev := upgradePrevPath(p)
		body, err := os.ReadFile(prev)
		if err != nil {
			fmt.Fprintf(w, "  RESTORE FAILED — cannot read %s (%v); the file now at %s is this run's. Put it back by hand: %s\n",
				AbbrevHome(prev), err, AbbrevHome(p), upgradeRollbackCmd(p))
			continue
		}
		if err := upgradeGateSwap(p, body); err != nil {
			fmt.Fprintf(w, "  RESTORE FAILED — %v. Put it back by hand: %s\n", err, upgradeRollbackCmd(p))
			continue
		}
		fmt.Fprintf(w, "  restored: %s\n", AbbrevHome(p))
	}
}
