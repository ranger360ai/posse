package posse

// The persona memory dir reaches a claude seat through the {settings}
// payload's permissions.additionalDirectories and NOT through --add-dir
// (ranger-base-hgcmw). claude 2.1.328 started gating that flag behind a trust
// dialog whenever the added dir sits inside a git repository nobody has
// trusted, and an instance that keeps its personas inside such a repo — this
// one does, because that is the repo the memory landing commits ORDERS.md to —
// then stalls every launch on a dialog no dispatched seat can answer.
//
// MEASURED 2026-11-20 on claude 2.1.330, linux: `--add-dir <dir in an
// untrusted repo>` raises the dialog; the same dir named in
// permissions.additionalDirectories of --settings raises nothing; the same
// FLAG pointed at a dir inside the cwd's own trusted repo raises nothing
// either, so the dialog is about the target's trust and not about the flag.
// Both spellings grant the same reach, measured with
// permissions.blockWritesOutsideWorkingDirectories on as the discriminator.
// docs/notes.d/ranger-base-hgcmw.md has the arms.
//
// Two halves and both are pinned here, because either one alone is a defect:
// a line that still names the flag stalls, and a line that dropped the flag
// without the grant hands the persona a memory dir it must ask about before
// it can append a lesson to ORDERS.md.

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// claudePayloadOf pulls the single --settings payload off a rendered line and
// parses it. Parsed rather than string-matched for the reason every other pin
// over this blob gives: key order inside it is not the contract.
func claudePayloadOf(t *testing.T, line string) map[string]json.RawMessage {
	t.Helper()
	if n := strings.Count(line, "--settings"); n != 1 {
		t.Fatalf("rendered line carries %d --settings flags, want 1:\n%s", n, line)
	}
	_, rest, _ := strings.Cut(line, "--settings ")
	rest = strings.TrimPrefix(rest, "'")
	raw, _, ok := strings.Cut(rest, "'")
	if !ok {
		t.Fatalf("could not read the quoted --settings payload off:\n%s", line)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.ReplaceAll(raw, `'\''`, `'`)), &m); err != nil {
		t.Fatalf("the --settings payload is not valid JSON: %v\n%s", err, raw)
	}
	return m
}

func payloadAdditionalDirs(t *testing.T, m map[string]json.RawMessage) []string {
	t.Helper()
	raw, ok := m["permissions"]
	if !ok {
		t.Fatalf("the payload names no permissions block at all")
	}
	var perms struct {
		AdditionalDirectories []string `json:"additionalDirectories"`
		DefaultMode           string   `json:"defaultMode"`
	}
	if err := json.Unmarshal(raw, &perms); err != nil {
		t.Fatalf("permissions block does not parse: %v\n%s", err, raw)
	}
	if perms.DefaultMode != "auto" {
		t.Errorf("the merge took the unattended permission mode off the payload: %s", raw)
	}
	return perms.AdditionalDirectories
}

func TestQAClaudeLineGrantsMemoryWithoutAddDir(t *testing.T) {
	t.Parallel()
	ag := loadTestAgent(t, "---\nname: p\n---\nYou are p.\n")
	line := ag.RenderCommand()
	if strings.Contains(line, "--add-dir") {
		t.Errorf("the claude launch line names --add-dir; that is claude's trust dialog back, and a dispatched seat has nobody to answer it (ranger-base-hgcmw):\n%s", line)
	}
	dirs := payloadAdditionalDirs(t, claudePayloadOf(t, line))
	var found bool
	for _, d := range dirs {
		if d == ag.MemoryDir {
			found = true
		}
	}
	if !found {
		t.Errorf("the line does not grant the memory dir %q — the persona cannot append to ORDERS.md without asking; payload grants %q", ag.MemoryDir, dirs)
	}
}

// Neither template posse writes may name the flag again. The CONSTANT is
// checked beside the rendered line because that is what a reader of this
// repo reaches for, and a template that regains the flag is exactly the
// regression that reads as fixed until a seat launches.
func TestQAClaudeTemplatesNameNoMemoryAddDir(t *testing.T) {
	t.Parallel()
	tmpls := map[string]string{"DefaultAgentCommand": DefaultAgentCommand}
	for i := range builtinRuntimes {
		rt := builtinRuntimes[i]
		if rt.Name != DefaultRuntime {
			continue
		}
		tmpls["runtime "+rt.Name] = rt.Command
	}
	if len(tmpls) != 2 {
		t.Fatalf("expected the default command and the built-in claude runtime's, got %d: %v", len(tmpls), tmpls)
	}
	for name, tmpl := range tmpls {
		if strings.Contains(tmpl, "--add-dir") {
			t.Errorf("%s names --add-dir (ranger-base-hgcmw): %s", name, tmpl)
		}
		if !strings.Contains(tmpl, "{settings}") {
			t.Errorf("%s names no {settings}, so there is nowhere for the memory grant to ride: %s", name, tmpl)
		}
	}
}

// "Nobody named a dir" renders NO grant, not an empty one: the parameter is
// trailing and variadic so that every caller which has none to name — the pin
// readers, the probes — renders what it rendered before.
//
// Asserted as "no additionalDirectories key, same key set", NOT as byte
// equality between two calls. The payload's env block holds the credential
// pin, whose GIT_CONFIG_GLOBAL value depends on whether a git-global snapshot
// exists at the moment it is read, so two calls can legitimately differ in a
// VALUE while agreeing about every key — and a byte comparison across two
// reads turns that into a red about something else (ranger-base-nycra).
func TestQAFleetSettingsPayloadNamesNoGrantWithNoMemoryDir(t *testing.T) {
	t.Parallel()
	keysOf := func(payload string) []string {
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			t.Fatalf("payload is not valid JSON: %v\n%s", err, payload)
		}
		if _, ok := m["permissions"]; ok {
			if dirs := payloadAdditionalDirs(t, m); len(dirs) != 0 {
				t.Errorf("the payload grants %q with no memory dir named:\n%s", dirs, payload)
			}
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		return keys
	}
	// The third is a RELATIVE dir, which claude ignores outright ("a relative
	// entry does not count"), so naming one here would read as a grant that
	// was made.
	bare := keysOf(ClaudeFleetSettingsJSON(nil))
	for _, arg := range []string{"", "personas/p"} {
		if got := keysOf(ClaudeFleetSettingsJSON(nil, arg)); !slices.Equal(got, bare) {
			t.Errorf("memoryDir %q changed the payload's key set:\n got %q\nwant %q", arg, got, bare)
		}
	}
}

// The grant does not displace anything else the payload carries: the
// credential/inlet pin, the field pin and the auto-mode carve-out a PID with
// the session verbs earns all have to be there beside it. This is the merge
// order's pin — the grant is applied before the pin half and both write the
// same map.
//
// The one branch no test here reaches is the grant with NO pin at all
// (`len(pin) == 0`), which is defensive and says so: the inlet rows name no
// path a box has to have, so settingsPin is non-empty even with no home
// directory (TestQACredentialDirPinIsAbsentWithNoHome). It is written as a
// return rather than a fallthrough anyway, because the thing it would drop
// is the write that must not start asking.
func TestQAMemoryGrantDisplacesNothingElseInThePayload(t *testing.T) {
	t.Parallel()
	payload := ClaudeFleetSettingsJSON([]string{"Bash(posse:*)"}, "/m/personas/p")
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("payload is not valid JSON: %v\n%s", err, payload)
	}
	dirs := payloadAdditionalDirs(t, m)
	if len(dirs) != 1 || dirs[0] != "/m/personas/p" {
		t.Errorf("the memory grant did not reach the payload: grants %q\n%s", dirs, payload)
	}
	if _, ok := m["env"]; !ok {
		t.Errorf("the grant took the env pin off the payload (ranger-base-rq83c):\n%s", payload)
	}
	if !strings.Contains(payload, "BASH_ENV") {
		t.Errorf("the grant took the inlet pin off the payload (ranger-base-rflee):\n%s", payload)
	}
	if _, ok := m["autoMode"]; !ok {
		t.Errorf("the grant took the auto-mode carve-out off a PID that earns it (ADR 0070 D2):\n%s", payload)
	}
	if got := string(m["autoMemoryEnabled"]); got != "false" {
		t.Errorf("autoMemoryEnabled = %s, want false (ranger-base-7uhip):\n%s", got, payload)
	}
}

// Whatever the key already holds stays, and the memory dir joins it once.
func TestQAMemoryGrantKeepsWhatThePermissionsBlockHeld(t *testing.T) {
	t.Parallel()
	m := map[string]json.RawMessage{
		"permissions": json.RawMessage(`{"defaultMode":"auto","additionalDirectories":["/already"]}`),
	}
	if !applyMemoryGrant(m, []string{"/already", "/mem", "/mem"}) {
		t.Fatal("applyMemoryGrant refused a well-formed permissions block")
	}
	dirs := payloadAdditionalDirs(t, m)
	if len(dirs) != 2 || dirs[0] != "/already" || dirs[1] != "/mem" {
		t.Errorf("merge is not append-once-in-order: %q", dirs)
	}
	bad := map[string]json.RawMessage{"permissions": json.RawMessage(`["not an object"]`)}
	if applyMemoryGrant(bad, []string{"/mem"}) {
		t.Error("a permissions block that does not parse must refuse, so the caller renders the const alone rather than a half-merged object")
	}
}
