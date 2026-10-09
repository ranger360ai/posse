//go:build posse_arm3

package posse

// Live pin for ranger-base-j5b24 — ADR 0070 Verification 2, run against the
// real claude rather than a transcription of its bundle:
//
//	RHQ_LIVE_CLAUDE=1 go test ./internal/posse -run TestLiveClaudeAutoMode -v
//
// WHAT IT MEASURES, and why it is not a duplicate of automodeallow_qa_test.go.
// The QA pins say the rendered payload carries `autoMode.allow` with the
// sentinel first and the carve-out second, and that the launch line carries
// that payload verbatim. This says the RUNTIME RESOLVES it — two separate
// facts, each sharp for its own reason:
//
//   - the SPLICE. `"$defaults"` is a sentinel claude's own code substitutes
//     its 17 built-in allow rules at. An array that omits it REPLACES the
//     built-in list rather than adding to it (ADR 0070 Context), so the
//     difference between adding one exception and deleting seventeen is one
//     element of one array, and only the real reader grades it.
//   - the PAYLOAD-SHAPE HAZARD. One wrong-typed row anywhere in the
//     `--settings` blob makes the runtime discard the WHOLE thing in silence
//     (fieldpin.go, ranger-base-i7cy4) — the credential dirs, every inlet row
//     and the fleet's permission mode with it. A discarded payload is
//     indistinguishable from a payload that merged, from any arm that only
//     asks whether the allow list is non-empty: it prints the box's own 17.
//     So every count here is read against the CONTROL, never against zero.
//
// THE READOUT is `claude auto-mode config`, which prints the effective auto
// mode config as JSON — the settings where set, defaults otherwise. It makes
// no classifier call, starts no turn, needs no login and runs no
// attacker-supplied command: it is a local print of resolved settings, which
// is exactly the layer a render pin is about. `claude auto-mode defaults`
// prints the shipped list beside it, so the count the splice is supposed to
// restore is READ on the box rather than hardcoded here.
//
// WHERE THE FLAG GOES: `--settings` is a GLOBAL option and must precede the
// subcommand (`claude auto-mode config --settings X` is "unknown option" —
// fieldpin.go's "WHERE THE FLAG GOES", re-confirmed on 2.1.295). Each call
// site below spells the whole argv for that reason.
//
// WHY A TEMP CWD. The classifier reads `autoMode` from user settings, managed
// settings and the `--settings` flag, and never from a project
// `.claude/settings*.json` (ADR 0070 Context), so the readout does not depend
// on the directory — running in t.TempDir() takes the repo out of the
// question rather than relying on that.
//
// THE ARMS, and why four. The CONTROL is the box with no blob on the line:
// without it a pin arm's count means nothing, because the box supplies most
// of it. The PIN arm is the blob a verb-granting PID renders. The ATTACK arm
// is that same blob with the sentinel removed — the canary that `"$defaults"`
// is what keeps the defaults, and the arm whose absence would leave the
// splice unmeasured (ranger-base: "a probe needs a failing wrong arm"). The
// VOID arm is the blob plus one wrong-typed row, and it is the arm that
// proves this readout can tell an accepted payload from a discarded one: it
// prints the control's count exactly, which is why the pin arm asserts
// control+1 and not "more than nothing".
//
// MEASURED 2026-10-09, claude 2.1.295, this box: defaults 17 allow / 72
// soft_deny / 1 hard_deny / 21 environment · control 17/72/1/24 · pin arm
// 18/72/1/24 with the carve-out last and "Security Discussion…" first ·
// attack arm 1/72/1/24 — the carve-out alone, all 17 defaults gone · void
// arm (`"statusLine":""`) 17/72/1/24, the carve-out absent.

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// autoModeConfig is the readout: the four sections `claude auto-mode config`
// and `claude auto-mode defaults` both print. The three sections beside
// `allow` are read so the probe can say the key's blast radius is one
// section — a splice that disturbed soft_deny or the environment would be a
// different change than the one ADR 0070 D2 decided.
type autoModeConfig struct {
	Allow       []string `json:"allow"`
	SoftDeny    []string `json:"soft_deny"`
	HardDeny    []string `json:"hard_deny"`
	Environment []string `json:"environment"`
}

// liveAutoModeJSON runs `claude <args...>` in dir and parses the JSON it
// prints. args is the whole argv after the binary, so the `--settings`
// position is visible at every call site. A non-zero exit is not fatal on
// its own — only an unparseable answer is, since that is the readout itself
// breaking and every arm below would be measuring nothing.
func liveAutoModeJSON(t *testing.T, dir string, args ...string) autoModeConfig {
	t.Helper()
	cmd := exec.Command("claude", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	var got autoModeConfig
	if jerr := json.Unmarshal(out, &got); jerr != nil {
		t.Fatalf("claude %s: unparseable answer: %v (exit: %v)\n%s", strings.Join(redactedArgs(args), " "), jerr, err, out)
	}
	return got
}

// redactedArgs shortens the rendered blob in a failure message — it is a
// kilobyte of prose and the credential dirs, and the argv is printed only so
// the reader knows WHICH arm spoke.
func redactedArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if r := []rune(a); strings.HasPrefix(a, "{") && len(r) > 40 {
			a = string(r[:37]) + "…}"
		}
		out = append(out, a)
	}
	return out
}

// withoutTheSentinel is the attack arm's payload: the rendered blob with the
// `"$defaults"` element taken out of autoMode.allow and nothing else
// changed. Derived from the real payload rather than written by hand, so the
// arm differs from the pin arm by exactly the one element under test.
func withoutTheSentinel(t *testing.T, payload string) string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("rendered payload is not valid JSON: %v", err)
	}
	var am struct {
		Allow []string `json:"allow"`
	}
	if err := json.Unmarshal(m["autoMode"], &am); err != nil {
		t.Fatalf("rendered autoMode is not {allow:[…]}: %v", err)
	}
	kept := make([]string, 0, len(am.Allow))
	for _, e := range am.Allow {
		if e != ClaudeAutoModeDefaults {
			kept = append(kept, e)
		}
	}
	if len(kept) == len(am.Allow) {
		t.Fatalf("the rendered payload carries no %q to remove, so the attack arm would be a second pin arm: %q", ClaudeAutoModeDefaults, am.Allow)
	}
	b, err := json.Marshal(map[string][]string{"allow": kept})
	if err != nil {
		t.Fatal(err)
	}
	m["autoMode"] = b
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// withAVoidingRow is the void arm's payload: the rendered blob plus a
// `statusLine` typed as a string where the runtime's schema wants
// {"type":"command","command":""}. That exact spelling is the MEASURED void
// (fieldpin.go, 2026-09-05: it left a planted apiKeyHelper in force), and it
// is here to make the discarded case visible from THIS readout.
func withAVoidingRow(t *testing.T, payload string) string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("rendered payload is not valid JSON: %v", err)
	}
	m["statusLine"] = json.RawMessage(`""`)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func sameAllowList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// firstAllowDiff is where two allow lists part company — the one number a
// reader of a reorder failure needs. -1 when they agree over the shorter.
func firstAllowDiff(a, b []string) int {
	for i := range a {
		if i >= len(b) {
			return i
		}
		if a[i] != b[i] {
			return i
		}
	}
	return -1
}

// clipRule shortens one allow entry for a failure message. Each shipped rule
// is a paragraph and the carve-out is six sentences; the diagnostics below
// are about WHICH entry sits where, so the head of it identifies it. Cut on
// runes, not bytes: the carve-out's own prose carries em dashes and an
// ellipsis, and a byte cut through one prints a replacement character in the
// middle of the evidence.
func clipRule(s string) string {
	r := []rune(s)
	if len(r) > 72 {
		return string(r[:69]) + "…"
	}
	return s
}

func allowListHas(list []string, want string) bool {
	for _, e := range list {
		if e == want {
			return true
		}
	}
	return false
}

func TestLiveClaudeAutoModeAllowSplicesTheCarveOutOverTheDefaults(t *testing.T) {
	// Safe in parallel: this test writes nothing and moves no process
	// environment — every arm is argv on a child's command line.
	t.Parallel()
	if os.Getenv("RHQ_LIVE_CLAUDE") == "" {
		t.Skip("set RHQ_LIVE_CLAUDE=1 (shells out to the real claude)")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("no claude on PATH")
	}

	dir := t.TempDir()

	// The payload under test is what RenderCommandForModel hands the launch
	// for a PID holding `Bash(posse:*)`; automodeallow_qa_test.go owns the
	// claim that the line carries these bytes verbatim.
	payload := ClaudeFleetSettingsJSON([]string{"Bash(posse:*)"})
	if !strings.Contains(payload, ClaudeAutoModeDefaults) {
		t.Fatalf("PRECONDITION: a PID holding Bash(posse:*) rendered no %q — there is nothing live to measure here, and the QA pins are the arm that failed:\n%s", ClaudeAutoModeDefaults, payload)
	}

	defaults := liveAutoModeJSON(t, dir, "auto-mode", "defaults")
	control := liveAutoModeJSON(t, dir, "auto-mode", "config")
	pin := liveAutoModeJSON(t, dir, "--settings", payload, "auto-mode", "config")
	attack := liveAutoModeJSON(t, dir, "--settings", withoutTheSentinel(t, payload), "auto-mode", "config")
	void := liveAutoModeJSON(t, dir, "--settings", withAVoidingRow(t, payload), "auto-mode", "config")

	t.Logf("allow lengths: shipped defaults %d · control %d · pin %d · attack %d · void %d",
		len(defaults.Allow), len(control.Allow), len(pin.Allow), len(attack.Allow), len(void.Allow))

	if len(defaults.Allow) == 0 {
		t.Fatalf("`claude auto-mode defaults` printed no allow rules at all — the sentinel has nothing to splice and this readout can measure neither arm")
	}

	// Is the box in the state ADR 0070 measured? `pristine` means no scope
	// on this box adds `autoMode.allow` entries of its own, which is what
	// makes the ADR's literal 17/18/1 checkable. It is NOT a defect when it
	// is false: ADR 0070 D4 names a box-wide `autoMode.allow` in
	// ~/.claude/settings.json as the operator's own standing remedy, and a
	// test that reddened when they took it would be reading their file as
	// posse's business. The relational arms below are the pin either way.
	pristine := sameAllowList(control.Allow, defaults.Allow)
	if !pristine {
		t.Logf("NOTE: this box's user or managed scope carries autoMode.allow entries of its own (control %d vs shipped %d) — ADR 0070 D4's operator remedy. The ADR's literal 17/18/1 are not asserted on this run; every relational arm below still is.", len(control.Allow), len(defaults.Allow))
	}

	// The ADR quotes the shipped list's first entry by name, because that is
	// the element the splice lands at. A reorder in claude's own list is
	// version drift, not a posse defect — but it makes the ADR's quote stale,
	// so it is said here rather than left for a reader to notice.
	if !strings.HasPrefix(defaults.Allow[0], "Security Discussion") {
		t.Errorf("claude's first shipped allow rule is %q, where 2.1.295 shipped \"Security Discussion…\" (ADR 0070 Verification 2, MEASURED 2026-10-09). If the shipped list was reordered, re-read the ADR's quote — nothing posse renders chose this element", clipRule(defaults.Allow[0]))
	}

	t.Run("the payload merged and the carve-out is the last entry", func(t *testing.T) {
		if !allowListHas(pin.Allow, ClaudeAutoModeCarveOut) {
			t.Fatalf("the carve-out is not in the effective allow list at all.\nEither autoMode.allow stopped being read from the --settings flag, or ONE ROW OF THE PAYLOAD IS WRONG-TYPED and the runtime discarded the whole thing — the credential dirs, every inlet row and the fleet's permission mode with it (fieldpin.go). The void arm below is what that looks like: it prints the control's %d.\npayload: %s", len(control.Allow), payload)
		}
		if last := pin.Allow[len(pin.Allow)-1]; last != ClaudeAutoModeCarveOut {
			t.Errorf("the carve-out is in the list but not last — got %q at the end.\nThe flag's entries are appended after every lower scope's, so last is where this one belongs; anything after it arrived from a source this probe did not name", clipRule(last))
		}
		if len(pin.Allow) != len(control.Allow)+1 {
			t.Errorf("the pin arm reports %d allow entries against the control's %d, want exactly one more.\nEqual counts are the voided payload (the arm below); fewer means the defaults were replaced rather than spliced (the attack arm below)", len(pin.Allow), len(control.Allow))
		}
		if pristine && len(pin.Allow) != 18 {
			t.Errorf("ADR 0070 Verification 2 reads 18 allow entries on a box whose scopes add none (17 shipped + the carve-out); this run reports %d", len(pin.Allow))
		}
	})

	t.Run("the defaults survived the splice", func(t *testing.T) {
		if pin.Allow[0] != defaults.Allow[0] {
			t.Errorf("the effective list no longer begins with claude's first shipped rule:\n got %q\nwant %q\nThe sentinel substitutes the shipped list at its own first position, so element 0 is the shipped element 0 or the splice did not happen", clipRule(pin.Allow[0]), clipRule(defaults.Allow[0]))
		}
		for _, want := range defaults.Allow {
			if !allowListHas(pin.Allow, want) {
				t.Errorf("a shipped allow rule is missing from the effective list: %q\nThat rule is an exception the classifier no longer has, for every seat carrying this blob — which is the failure `\"$defaults\"` exists to prevent", clipRule(want))
			}
		}
		if head := pin.Allow[:len(defaults.Allow)]; pristine && !sameAllowList(head, defaults.Allow) {
			i := firstAllowDiff(head, defaults.Allow)
			t.Errorf("the shipped rules are present but reordered or interleaved: element %d is %q where the shipped list has %q.\nThe sentinel substitutes them as a block at its own position, so the %d shipped rules come first, in order", i, clipRule(head[i]), clipRule(defaults.Allow[i]), len(defaults.Allow))
		}
	})

	// The canary. Without this arm the arm above is equally consistent with
	// "claude keeps the defaults whatever the array says", and the sentinel
	// would be unmeasured prose.
	t.Run("the sentinel is what keeps them", func(t *testing.T) {
		if !allowListHas(attack.Allow, ClaudeAutoModeCarveOut) {
			t.Fatalf("the sentinel-less payload did not merge either (%d entries, none of them the carve-out), so this arm measures the merge and not the splice — read the arm above first", len(attack.Allow))
		}
		if len(attack.Allow) >= len(pin.Allow) {
			t.Errorf("dropping %q from the array cost nothing: %d entries against the pin arm's %d.\nIf claude now keeps its shipped rules whatever the array holds, the sentinel is no longer load-bearing and ADR 0070's reason for putting it first has changed — re-measure before trusting either reading", ClaudeAutoModeDefaults, len(attack.Allow), len(pin.Allow))
		}
		if allowListHas(attack.Allow, defaults.Allow[0]) {
			t.Errorf("a shipped rule survived a payload with no sentinel (%q), so the splice is not the mechanism measured here", clipRule(defaults.Allow[0]))
		}
		if pristine && len(attack.Allow) != 1 {
			t.Errorf("without the sentinel the effective list should be the carve-out ALONE on this box — all 17 shipped rules deleted, which is the cost of the mutation; got %d entries", len(attack.Allow))
		}
	})

	// And the half a false green would hide: a payload the runtime threw
	// away reads exactly like a payload that merged, from any arm that only
	// asks whether the list is non-empty.
	t.Run("a voided payload reads as the control, not as the pin", func(t *testing.T) {
		if allowListHas(void.Allow, ClaudeAutoModeCarveOut) {
			t.Fatalf("a payload carrying a wrong-typed `statusLine` still delivered the carve-out — this readout can no longer tell an accepted payload from a discarded one, and the pin arm above is not evidence. (Either the runtime stopped discarding wrong-typed payloads, in which case fieldpin.go's hazard needs re-measuring, or the row's spelling stopped being wrong.)")
		}
		if len(void.Allow) != len(control.Allow) {
			t.Errorf("the voided payload reports %d allow entries against the control's %d, want them equal — a discarded payload is the box with no blob on the line, which is exactly why the pin arm counts control+1 rather than \"not zero\"", len(void.Allow), len(control.Allow))
		}
	})

	// The key's blast radius: one section. ADR 0070 rejected a
	// classifyAllShell row and an `environment` splice by name, so the other
	// three sections must read the same with the blob on the line as without.
	t.Run("the other three sections are untouched", func(t *testing.T) {
		for _, s := range []struct {
			name          string
			with, without []string
		}{
			{"soft_deny", pin.SoftDeny, control.SoftDeny},
			{"hard_deny", pin.HardDeny, control.HardDeny},
			{"environment", pin.Environment, control.Environment},
		} {
			if !sameAllowList(s.with, s.without) {
				t.Errorf("autoMode.%s changed when the carve-out landed: %d entries with the blob, %d without.\nThe payload names one key under autoMode; anything else moving here is a claim ADR 0070 did not make", s.name, len(s.with), len(s.without))
			}
		}
	})
}
