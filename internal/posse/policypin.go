package posse

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// A stale policy-tier drop-in voids the GIT_CONFIG_SYSTEM row
// (ranger-base-s69ep, from ranger-base-zwkgb's verification of
// ranger-base-t301k).
//
// t301k moved GIT_CONFIG_SYSTEM off the shipped policy drop-in
// (etc/claude/managed-settings.d/10-posse-inlet-pin.json) and onto the
// per-launch `--settings` pin, resolved to git's own system-scope path. But
// a box that INSTALLED the old drop-in still carries
// "GIT_CONFIG_SYSTEM": "/dev/null" at policySettings, the last source in
// claude's fold (credential.go's scope-order note), so it beats the launch
// pin. The seat then skips system scope again, where an employer's commit
// scanner lives. GitSystemScopeDegraded cannot see this: resolution
// succeeded, so it says nothing.
//
// This reads the policy-tier files and names any that pin the variable to
// something other than what the launch pin carries. READ ONLY: the files
// are root-owned and replacing them is the operator's (guardrail 3). It
// does not decide which file wins the fold; a file that disagrees is named
// whichever one wins, because the fix is the same for each.

// PolicyPinGate is the parity row's name in `posse gates`.
const PolicyPinGate = "git: policy-tier pin"

// claudePolicyDir is the OS-admin directory claude reads policySettings
// from. "" on a platform with no known path, which reads as nothing to check.
func claudePolicyDir(goos string) string {
	switch goos {
	case "darwin":
		return "/Library/Application Support/ClaudeCode"
	case "linux":
		return "/etc/claude-code"
	}
	return ""
}

// policySettingsFiles lists the files claude reads from a policy dir:
// managed-settings.json, then managed-settings.d/*.json in name order.
// Missing files and a missing drop-in dir are not errors.
func policySettingsFiles(dir string) []string {
	if dir == "" {
		return nil
	}
	var out []string
	base := filepath.Join(dir, "managed-settings.json")
	if _, err := os.Lstat(base); err == nil {
		out = append(out, base)
	}
	ents, err := os.ReadDir(filepath.Join(dir, "managed-settings.d"))
	if err != nil {
		return out
	}
	var names []string
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, filepath.Join(dir, "managed-settings.d", n))
	}
	return out
}

// policyPinFinding is what one policy file says about GIT_CONFIG_SYSTEM.
type policyPinFinding struct {
	File   string
	Value  string // the value it pins; meaningful when Stale
	Raw    bool   // Value is a JSON token, not a string the variable could carry
	Stale  bool   // it pins a value other than the launch pin's
	Unread string // why it could not be judged ("" when it was read)
}

// pins is how one finding's value is named in a line. A string is quoted,
// because the quotes are what distinguish "" from an absent row. A RAW value
// is a JSON token — null, a number, an object, an array — and %q over it
// would read as a string the file does not carry, so it is named as the token
// it is (ranger-base-zpk8r finding 3).
func (f policyPinFinding) pins() string {
	if f.Raw {
		return fmt.Sprintf("%s pins the JSON value %s", AbbrevHome(f.File), f.Value)
	}
	return fmt.Sprintf("%s pins %q", AbbrevHome(f.File), f.Value)
}

// policyGitSystemScopeFindings reads every policy file in dir and reports
// each one that pins GIT_CONFIG_SYSTEM to a value other than want, plus
// each one it could not read. A file that does not set the variable, or
// sets it to want, is not reported.
func policyGitSystemScopeFindings(dir, want string) []policyPinFinding {
	var out []policyPinFinding
	for _, f := range policySettingsFiles(dir) {
		fi, err := os.Stat(f)
		if err != nil {
			out = append(out, policyPinFinding{File: f, Unread: err.Error()})
			continue
		}
		if !fi.Mode().IsRegular() {
			// Never opened: open(2) on a FIFO with no writer blocks
			// (ranger-base-92rt).
			out = append(out, policyPinFinding{File: f, Unread: fileTypeName(fi.Mode()) + ", not a regular file"})
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			out = append(out, policyPinFinding{File: f, Unread: err.Error()})
			continue
		}
		var doc struct {
			Env map[string]json.RawMessage `json:"env"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			out = append(out, policyPinFinding{File: f, Unread: "invalid JSON: " + err.Error()})
			continue
		}
		raw, ok := doc.Env["GIT_CONFIG_SYSTEM"]
		if !ok {
			continue
		}
		// JSON null is the one non-string value json.Unmarshal into a
		// string accepts: it is a no-op that returns NO error, so without
		// this arm a file pinning null took the v=="" path below and the
		// line said it pinned "" (MEASURED, ranger-base-zpk8r finding 3).
		// Stale either way — null is not a path — but the value named is
		// now the one the file carries.
		if strings.TrimSpace(string(raw)) == "null" {
			out = append(out, policyPinFinding{File: f, Value: "null", Raw: true, Stale: true})
			continue
		}
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			out = append(out, policyPinFinding{File: f, Value: strings.TrimSpace(string(raw)), Raw: true, Stale: true})
			continue
		}
		if v == want || (v != "" && want != "" && filepath.Clean(v) == filepath.Clean(want)) {
			continue
		}
		out = append(out, policyPinFinding{File: f, Value: v, Stale: true})
	}
	return out
}

// policyPinDegraded is the launch line for findings, "" when no file pins a
// stale value. Unreadable files do not earn it: claude runs as the same user
// and cannot read them either.
//
// TWO WORDINGS, because the disagreement points opposite ways depending on
// what the launch pin itself resolved to (ranger-base-zpk8r finding 2).
// `want` is gitSystemScopePinValue(): git's own system-scope path, or
// gitSystemScopeFallback when resolution FAILED. In the ordinary case the
// launch pin carries a real path and a policy file that outranks it takes
// system scope away, which is the sentence s69ep was written for. In the
// fallback case the launch pin is the one carrying /dev/null, so a policy
// file pinning anything else is the tier that keeps the inlet OPEN — and the
// s69ep sentence, said there, claims the exact opposite of what the box is
// doing. A detective row that inverts its verdict is the failure mode this
// reading exists to end, one configuration over.
func policyPinDegraded(fs []policyPinFinding, want string) string {
	var stale []string
	for _, f := range fs {
		if f.Stale {
			stale = append(stale, f.pins())
		}
	}
	if len(stale) == 0 {
		return ""
	}
	if want == gitSystemScopeFallback {
		// Resolution failed, so GitSystemScopeDegraded is saying why —
		// AT LAUNCH, where the two lines print together. This line names
		// that reading rather than pointing at a position, because the same
		// text is also the `posse gates` row detail and there is no
		// GIT_CONFIG_SYSTEM row beside it there. Replacing
		// the installed file with the shipped copy would NOT help: the
		// shipped drop-in pins nothing since ranger-base-t301k, which would
		// leave the seat on the launch pin's /dev/null and no system scope
		// at all. So this arm points at the resolution failure instead.
		return fmt.Sprintf("DEGRADED: git system scope is whatever the POLICY TIER says here, not what this launch pinned — the launch pin could not resolve git's own system-scope path and carries GIT_CONFIG_SYSTEM=%s, and %s, which outranks it. What reaches git is that file's value, not the launch pin's, and posse neither resolved nor vetted it: a hook configured in the system scope of the git this seat actually runs may still be skipped. The thing to fix is that resolution failure, which posse names on its own DEGRADED line at launch (ranger-base-t301k); replacing the installed file with the shipped copy under etc/claude/managed-settings.d/ would leave this seat with no system scope at all (it needs root; posse does not write there) (ranger-base-s69ep, ranger-base-zpk8r)",
			want, strings.Join(stale, "; "))
	}
	return fmt.Sprintf("DEGRADED: git system scope is NOT carried into this session — the policy tier outranks the launch pin's GIT_CONFIG_SYSTEM=%s, and %s. A commit scanner or hook configured only in system scope will not run here. Replace the installed file with the shipped copy under etc/claude/managed-settings.d/ (it needs root; posse does not write there) (ranger-base-s69ep)",
		want, strings.Join(stale, "; "))
}

// PolicyPinDegraded is policyPinDegraded for this box's policy dir and the
// value this launch pins.
func PolicyPinDegraded() string {
	want := gitSystemScopePinValue()
	return policyPinDegraded(policyGitSystemScopeFindings(claudePolicyDir(runtime.GOOS), want), want)
}

// policyPinRowDetail is the `posse gates` row text for findings in dir.
func policyPinRowDetail(dir, want string, fs []policyPinFinding) string {
	if d := policyPinDegraded(fs, want); d != "" {
		var unread []string
		for _, f := range fs {
			if f.Unread != "" {
				unread = append(unread, fmt.Sprintf("%s not judged: %s", AbbrevHome(f.File), f.Unread))
			}
		}
		if len(unread) > 0 {
			d += "; " + strings.Join(unread, "; ")
		}
		return d
	}
	var unread []string
	for _, f := range fs {
		unread = append(unread, fmt.Sprintf("%s not judged: %s", AbbrevHome(f.File), f.Unread))
	}
	if len(unread) > 0 {
		return "no readable policy file pins a stale GIT_CONFIG_SYSTEM; " + strings.Join(unread, "; ")
	}
	return fmt.Sprintf("no policy file under %s pins GIT_CONFIG_SYSTEM to anything but the launch pin's %s", AbbrevHome(dir), want)
}

// applyPolicyPin adds the row for runtimes that read claude's policy tier
// (the ones carrying the settings pin). Class "" and never Degraded, the
// same as the launch's GitSystemScopeDegraded line: the launch proceeds and
// the row says what it lacks. Refusing every claude dispatch over a
// root-owned file the persona cannot change would stop the fleet, not fix
// the box.
func applyPolicyPin(p *Parity, rt *Runtime) {
	if rt == nil || (rt.FleetSettings == nil && rt.SettingsPin == nil) {
		return
	}
	dir := claudePolicyDir(runtime.GOOS)
	if dir == "" {
		return
	}
	want := gitSystemScopePinValue()
	p.Realized[PolicyPinGate] = RealizedGate{Detail: policyPinRowDetail(dir, want, policyGitSystemScopeFindings(dir, want))}
}
