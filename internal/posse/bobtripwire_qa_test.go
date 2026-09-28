//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-q0e1y / ADR 0060 D2 — the tripwire on the Bob filing package.
//
// herdr 0.8.2 has no `bob` kind. A standalone manifest for an id herdr does
// not know is ignored outright and a local alias does not resolve (both
// MEASURED 2026-09-28, ADR 0060 Context 1–2), so the route is upstream: the
// filing is etc/herdr/agent-detection/upstream-bob.md, the proposed rules are
// upstream/bob/bob.toml, and the pane snapshots the rules were cut from are
// upstream/bob/*.txt.
//
// Those snapshots cannot live in testdata/bob/ yet: scripts/verify-detection.sh
// replays every fixture through `herdr agent explain --file` and fails any
// whose answer is not the state in its filename, which for an agent herdr
// cannot evaluate is every one of them, on every run, forever.
//
// So this test asserts the gap itself. Every fixture must still come back
// `unknown_agent`. It goes RED on the one day that stops being true — the day
// herdr ships the kind — and its message is the checklist for that day.
//
// WHAT BEHAVES DIFFERENTLY IF THIS TEST IS DELETED (ADR 0006 §7): nothing,
// until herdr learns bob. Then nothing still — and that is the failure. There
// is no other signal: `make verify-detection` never looks inside upstream/
// (it globs etc/herdr/agent-detection/*.toml and testdata/*/ only), nothing
// reads upstream-bob.md, and `posse runtime check bob` reports the launch row
// unmet from its own probe without ever mentioning these files. A herdr update
// would quietly make six captured screens detectable and they would stay in a
// directory named "upstream", never becoming the regression test they were
// captured to be, until somebody happened to reread an ADR. This test is the
// only thing standing between that release and that silence.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bobTripwireRemedy is the checklist the failure prints. It is the process
// this pin protects, in the order it has to happen.
const bobTripwireRemedy = "herdr now knows bob — move upstream/bob/*.txt into testdata/bob/, " +
	"delete the draft manifest, run make verify-detection, then posse runtime probe bob (ranger-base-6wqe)"

func TestQABobFixturesAreStillUnknownToHerdr(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// go test ./internal/posse runs with cwd = this package.
	dir := filepath.Join(root, "..", "..", "etc", "herdr", "agent-detection", "upstream", "bob")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			fixtures = append(fixtures, e.Name())
		}
	}
	// A tripwire over zero fixtures is a green test that watches nothing —
	// the same silence the comment above is about, reached by deleting the
	// captures instead of the test.
	if len(fixtures) == 0 {
		t.Fatalf("no *.txt fixtures in %s — the filing package's snapshots are what this pin watches", dir)
	}

	// Ask the herdr BINARY, with no override directory at all: the kind is
	// compiled in, and an operator's ~/.config must not be able to change this
	// answer in either direction. XDG_STATE_HOME goes with it so a cached
	// remote manifest cannot either.
	config := t.TempDir()
	env := append(os.Environ(),
		"XDG_CONFIG_HOME="+config,
		"XDG_STATE_HOME="+filepath.Join(config, "state"))

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command("herdr", "agent", "explain",
				"--file", filepath.Join(dir, name), "--agent", "bob", "--json")
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("herdr agent explain: %v\n%s", err, out)
			}
			// --file is a bare detection object; a pane explain through the CLI
			// wraps the same object in {"result":...}. Accept both, as the other
			// detection pins here do.
			var wrap struct {
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(out, &wrap); err != nil {
				t.Fatalf("json: %v\n%s", err, out)
			}
			raw := out
			if len(wrap.Result) > 0 && wrap.Result[0] == '{' {
				raw = wrap.Result
			}
			var det struct {
				State          string `json:"state"`
				Fallback       string `json:"fallback_reason"`
				ManifestSource string `json:"manifest_source"`
			}
			if err := json.Unmarshal(raw, &det); err != nil {
				t.Fatalf("detection json: %v\n%s", err, out)
			}
			if det.Fallback != "unknown_agent" {
				t.Errorf("fallback_reason=%q state=%q manifest_source=%q, want unknown_agent.\n%s",
					det.Fallback, det.State, det.ManifestSource, bobTripwireRemedy)
			}
		})
	}
}
