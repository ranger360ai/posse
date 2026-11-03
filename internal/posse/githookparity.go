package posse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ─── managed-hook git config: the operator's view against the seat's ────────
//
// ranger-base-tpvkc (finding ranger-base-j1msd, class of ranger-base-zwkgb).
// ADR 0052's governing rule is that posse must never cause the managed hook
// to be skipped. Two things a launch renders can break that without anyone
// seeing it:
//
//   - inletPin's GIT_CONFIG_SYSTEM=/dev/null empties SYSTEM scope, so a
//     scanner configured only there never runs on a seat commit, and git
//     says nothing (ranger-base-t301k, MEASURED 2026-11-03).
//   - the ADR 0052 D2 redirect sets a command-scope core.hooksPath. If the
//     directory it forwards into is not the operator's own effective
//     hooksPath, the employer's hook is skipped while the system entry is
//     still LISTED in the seat's config, so a listing comparison alone reads
//     clean over it (MEASURED 2026-11-03 on the security seat, git 2.50.1).
//
// This compares the two views and says which entries the seat loses. It is
// read only: it runs `git var` and `git config --get-regexp` and nothing
// else.
//
// The seat view is BUILT from what a launch renders, never read from this
// process's environment. Inside a seat that environment already carries the
// pin, so reading it would compare the seat with itself. Both views start
// from the same base with every GIT_CONFIG* variable removed, and the seat
// view then gets the rendered variables on top.
//
// IT NEVER DEGRADES A LAUNCH. It is a detective row (j1msd is a P2
// hardening finding), so a red result is printed, not enforced: the fix for
// what it finds is ranger-base-t301k, and refusing every claude launch on a
// managed box until then would be a launch-policy change nobody ruled on.

// gitHookConfigRegexp is the key set the comparison reads: config-based
// hooks (hook.<name>.*, git ≥ 2.54) and core.hooksPath. git lowercases the
// section and the variable name in its output, so the regexp is lowercase.
const gitHookConfigRegexp = `^(hook\.|core\.hookspath)`

// HookConfigGate is the parity row's name in `posse gates`.
const HookConfigGate = "git: managed-hook config"

// gitHookEntry is one line of `git config --show-scope --show-origin
// --get-regexp`.
type gitHookEntry struct {
	Scope, Origin, Key, Value string
}

func (e gitHookEntry) String() string {
	return fmt.Sprintf("%s %s %s=%s", e.Scope, e.Origin, e.Key, e.Value)
}

// gitHookView is what one environment's git says about the hook config.
// SystemPath is `git var GIT_CONFIG_SYSTEM`; SystemOff is set when git
// refuses to name one, which is what GIT_CONFIG_NOSYSTEM does (rc 1).
type gitHookView struct {
	SystemPath string
	SystemOff  bool
	Entries    []gitHookEntry
	Err        string // a git failure that is not "no matching key"
}

// gitHookParityInput is everything the comparison needs, so a test can hand
// it a fixture instead of this box.
type gitHookParityInput struct {
	// Base is the environment both views start from. Every GIT_CONFIG*
	// variable is removed from it before either view is built.
	Base []string
	// OperatorSystem, when set, is put in the operator view as
	// GIT_CONFIG_SYSTEM. Tests use it to inject a fixture system file;
	// production leaves it empty so git resolves its own.
	OperatorSystem string
	// Seat is what the launch renders on top of Base: inletPin for a
	// runtime that carries it, then the redirect's GIT_CONFIG_COUNT trio.
	Seat []EnvVar
	// Dir is the repo the config is read in; "" reads outside any repo
	// (system and global scopes only).
	Dir string
	// Redirect is the session hooks dir the seat's core.hooksPath names and
	// the managed dir it forwards into; nil when the launch renders none.
	Redirect *l3Redirect
}

// gitHookParityResult is the comparison's verdict. Lines are printable
// without further wrapping decisions; Red says whether any of them is a
// finding.
type gitHookParityResult struct {
	Red   bool
	Lines []string
}

// scrubGitConfigEnv removes the whole GIT_CONFIG* family: GIT_CONFIG_SYSTEM,
// _GLOBAL, _NOSYSTEM, _COUNT with its _KEY_n/_VALUE_n, _PARAMETERS, and bare
// GIT_CONFIG.
func scrubGitConfigEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GIT_CONFIG") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// envWith applies vars on top of env, last write winning, as a launch's
// rendered env does.
func envWith(env []string, vars []EnvVar) []string {
	for _, v := range vars {
		env = envPut(env, v.Key, v.Value)
	}
	return env
}

// runGitEnv runs one bounded git child under env. Bounded and routed the way
// every other git child here is (gitCapture), with an env of its own.
func runGitEnv(env []string, dir string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), GitTimeout)
	defer cancel()
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := protocolCmd(exec.CommandContext(ctx, "git", args...))
	cmd.Env = protocolEnv(env)
	bindGitDeadline(ctx, cmd)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.Bytes(), ee.ExitCode(), fmt.Errorf("git %s: rc %d: %s", strings.Join(args, " "), ee.ExitCode(), strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), 0, err
}

// readGitHookView asks git, under env, for the two facts a view is made of.
func readGitHookView(env []string, dir string) gitHookView {
	var v gitHookView
	out, rc, err := runGitEnv(env, "", "var", "GIT_CONFIG_SYSTEM")
	switch {
	case err == nil:
		v.SystemPath = strings.TrimSpace(string(out))
	case rc == 1:
		// GIT_CONFIG_NOSYSTEM: git declines to name a system file at all.
		v.SystemOff = true
	default:
		v.Err = err.Error()
		return v
	}
	out, rc, err = runGitEnv(env, dir, "config", "-z", "--show-scope", "--show-origin", "--get-regexp", gitHookConfigRegexp)
	if err != nil && rc != 1 { // rc 1 is "no key matched"
		v.Err = err.Error()
		return v
	}
	v.Entries = parseGitHookEntries(out)
	return v
}

// parseGitHookEntries reads `-z --show-scope --show-origin --get-regexp`:
// each entry is scope NUL origin NUL key LF value NUL (MEASURED 2026-11-03,
// git 2.50.1). A key with no value has no LF.
func parseGitHookEntries(out []byte) []gitHookEntry {
	fields := strings.Split(string(out), "\x00")
	var es []gitHookEntry
	for i := 0; i+2 < len(fields); i += 3 {
		key, val, _ := strings.Cut(fields[i+2], "\n")
		es = append(es, gitHookEntry{Scope: fields[i], Origin: fields[i+1], Key: key, Value: val})
	}
	return es
}

// effectiveHooksPath is the core.hookspath git uses in this view: the last
// one listed, since later entries win. "" when none is set.
func (v gitHookView) effectiveHooksPath() string {
	p := ""
	for _, e := range v.Entries {
		if e.Key == "core.hookspath" {
			p = e.Value
		}
	}
	return p
}

// systemLine describes a view's system scope. A seat path of /dev/null, or
// GIT_CONFIG_NOSYSTEM, is said as NOT CARRIED and never as "no managed
// hooks": that is the trap inletpin.go's GIT_CONFIG_SYSTEM row records.
func (v gitHookView) systemLine(who string) (string, bool) {
	switch {
	case v.SystemOff:
		return who + " system scope NOT CARRIED (GIT_CONFIG_NOSYSTEM: git names no system file)", true
	case v.SystemPath == os.DevNull:
		return who + " system scope NOT CARRIED (GIT_CONFIG_SYSTEM=" + os.DevNull + ")", true
	default:
		return who + " system scope " + v.SystemPath, false
	}
}

// gitHookParity builds both views and compares them.
func gitHookParity(in gitHookParityInput) gitHookParityResult {
	base := scrubGitConfigEnv(in.Base)
	opEnv := base
	if in.OperatorSystem != "" {
		opEnv = envPut(append([]string(nil), base...), "GIT_CONFIG_SYSTEM", in.OperatorSystem)
	}
	seatEnv := envWith(append([]string(nil), base...), in.Seat)
	return compareGitHookViews(readGitHookView(opEnv, in.Dir), readGitHookView(seatEnv, in.Dir), in.Redirect)
}

// compareGitHookViews is the verdict, separated from the git calls so the
// wording can be read without running git.
func compareGitHookViews(op, seat gitHookView, red *l3Redirect) gitHookParityResult {
	var r gitHookParityResult
	if op.Err != "" || seat.Err != "" {
		r.Red = true
		for _, e := range []string{op.Err, seat.Err} {
			if e != "" {
				r.Lines = append(r.Lines, "RED — cannot read git's hook config, so whether the seat carries it is unknown: "+e)
			}
		}
		return r
	}

	opLine, _ := op.systemLine("operator")
	seatLine, notCarried := seat.systemLine("seat")
	r.Lines = append(r.Lines, opLine, seatLine)

	// An operator system file that does not exist has nothing in it to
	// lose. Said, not passed silently, so the path stays visible.
	if !op.SystemOff && op.SystemPath != "" {
		if _, err := os.Stat(op.SystemPath); err != nil {
			r.Lines = append(r.Lines, "system scope "+AbbrevHome(op.SystemPath)+" absent: nothing to carry")
		}
	}

	seen := map[gitHookEntry]bool{}
	for _, e := range seat.Entries {
		seen[e] = true
	}
	for _, e := range op.Entries {
		if seen[e] {
			continue
		}
		r.Red = true
		why := ""
		if e.Scope == "system" && notCarried {
			why = " (system scope NOT CARRIED)"
		}
		r.Lines = append(r.Lines, "RED — missing from the seat's git view"+why+": "+e.Scope+" "+e.Origin+" "+e.Key+"="+e.Value)
	}

	// The redirect arm. A command-scope core.hooksPath naming the session
	// hooks dir overrides every other scope, so the system entry can stay
	// listed above while it no longer runs. What the redirect forwards
	// into is what has to match the operator's effective hooksPath.
	if red != nil {
		for _, e := range seat.Entries {
			if e.Scope != "command" || e.Key != "core.hookspath" || filepath.Clean(e.Value) != filepath.Clean(red.Hooks) {
				continue
			}
			want := op.effectiveHooksPath()
			if want == "" || !sameHooksPath(want, red.Managed) {
				r.Red = true
				if want == "" {
					want = "(unset)"
				}
				r.Lines = append(r.Lines, fmt.Sprintf("RED — the session redirect %s forwards into %s, but the operator's effective core.hooksPath is %s",
					AbbrevHome(red.Hooks), AbbrevHome(red.Managed), want))
			} else {
				r.Lines = append(r.Lines, "redirect "+AbbrevHome(red.Hooks)+" forwards into "+AbbrevHome(red.Managed)+", the operator's effective core.hooksPath")
			}
			break
		}
	}

	if !r.Red {
		n := len(op.Entries)
		r.Lines = append(r.Lines, fmt.Sprintf("clean — every one of the operator's %d hook config entries is in the seat's view", n))
	}
	return r
}

// sameHooksPath compares a config value against a resolved path. git
// expands a leading ~/ in core.hooksPath, so this does too.
func sameHooksPath(value, resolved string) bool {
	if rest, ok := strings.CutPrefix(value, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			value = filepath.Join(home, rest)
		}
	}
	return filepath.Clean(value) == filepath.Clean(resolved)
}

// seatGitPin is the part of a launch's rendered env that reaches git:
// inletPin for a runtime that renders the settings pin (FleetSettings is
// how claude carries it), and nothing for one that does not.
func seatGitPin(rt *Runtime) []EnvVar {
	if rt == nil || rt.FleetSettings == nil {
		return nil
	}
	var out []EnvVar
	for _, v := range inletPin() {
		if strings.HasPrefix(v.Key, "GIT_CONFIG") {
			out = append(out, v)
		}
	}
	return out
}

// launchGitHookParity is the live row: this box's git, the runtime's pin,
// and the redirect a launch in dir renders. A launch hands its real redirect
// in; every other caller passes nil and gets the one a launch WOULD render,
// with a placeholder session dir (no session exists yet), which is all git
// needs: it reads the value, it does not open the directory.
func (a *App) launchGitHookParity(rt *Runtime, dir string, red *l3Redirect) gitHookParityResult {
	seat := seatGitPin(rt)
	if red != nil {
		seat = append(seat, gitConfigHooksPathVars(seat, red.Hooks)...)
	} else if dir != "" && a.StateDir != "" {
		if m, err := managedHooksDir(dir); err == nil && m.Managed {
			hooks := filepath.Join(a.StateDir, "hooks", "<session>")
			red = &l3Redirect{Hooks: hooks, Managed: m.Dir}
			seat = append(seat, gitConfigHooksPathVars(seat, hooks)...)
		}
	}
	return gitHookParity(gitHookParityInput{Base: os.Environ(), Seat: seat, Dir: dir, Redirect: red})
}

// applyHookConfigParity adds the row to a directory-aware parity matrix.
// Class "" and never Degraded, for the reason in this file's header.
func (a *App) applyHookConfigParity(p *Parity, rt *Runtime, dir string, red *l3Redirect) {
	if dir == "" || rt == nil {
		return
	}
	res := a.launchGitHookParity(rt, dir, red)
	p.Realized[HookConfigGate] = RealizedGate{Detail: strings.Join(res.Lines, "; ")}
}

// hookConfigRow is the same comparison in `posse runtime check`.
func (a *App) hookConfigRow(rt *Runtime, dir string) stageRow {
	r := stageRow{
		stage:   "git_hooks",
		by:      "the launch's rendered env: inletPin (claude's settings pin) and the ADR 0052 redirect, compared against this box's git with every GIT_CONFIG* removed",
		missing: "a managed hook configured in a scope the seat does not read is SKIPPED with no message, and git still exits 0 (ranger-base-t301k)",
	}
	res := a.launchGitHookParity(rt, dir, nil)
	r.value = "clean"
	if res.Red {
		r.value = "RED — the seat's git does not see every managed-hook entry the operator's does"
	}
	r.note = res.Lines
	return r
}
