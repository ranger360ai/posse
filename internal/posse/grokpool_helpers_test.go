package posse

// Helpers lifted out of grokpool_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// grokPoolTurn is one turn_completed at a chosen instant. It carries the
// modelUsage breakdown grok really writes — which restates the same spend and
// is the 2× trap (cost_grok.go) — so the dollars this guard trips on are
// measured through the same duplication production sees.
func grokPoolTurn(ts time.Time, promptID string, ticks int64) string {
	rec := map[string]any{
		"timestamp": ts.Unix(),
		"method":    "session/update",
		"params": map[string]any{
			"update": map[string]any{
				"sessionUpdate": "turn_completed",
				"prompt_id":     promptID,
				"usage": map[string]any{
					"inputTokens":  1000,
					"outputTokens": 100,
					"costUsdTicks": ticks,
					"modelUsage": map[string]any{
						"grok-4.6-build": map[string]any{"costUsdTicks": ticks},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(rec)
	return string(b) + "\n"
}

func grokPoolUser(ts time.Time, text string) string {
	rec := map[string]any{
		"timestamp": ts.Unix(),
		"method":    "session/update",
		"params": map[string]any{
			"update": map[string]any{
				"sessionUpdate": "user_message_chunk",
				"content":       map[string]any{"type": "text", "text": text},
				"_meta":         map[string]any{"modelId": "grok-4.6"},
			},
		},
	}
	b, _ := json.Marshal(rec)
	return string(b) + "\n"
}

// grokPoolHome gives the caller its own $HOME and returns it. Anything that
// plants a grok session must take one: the pool reading is the SUM over
// every session under $HOME/.grok, and after ADR 0047 D1 the home is one
// temp directory for the whole test binary — so a fixture that plants into
// it reads back whatever every other test planted too. Measured on the run
// that first shared the home: five of these read 100%, 180% and 200% of a
// pool their own fixtures spend a fraction of.
//
// Setting the environment is also what keeps the caller SERIAL, by the
// runtime's own rule and with no list to maintain — which is the answer ADR
// 0047 D3 names for a test that writes into the shared home.
func grokPoolHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// grokPoolSession plants one session transcript. The session id is a
// parameter because the pool is the sum over MANY sessions, which is the
// thing being measured.
func grokPoolSession(t *testing.T, home, id, body string) string {
	t.Helper()
	dir := filepath.Join(home, ".grok", "sessions", url.PathEscape("/tmp/proj"), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "updates.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// usd renders a dollar amount as grok's nano-dollar ticks.
func usdTicks(d float64) int64 { return int64(d * 1e9) }

// The clock every pass here runs on: Thursday 2026-08-27 12:00 local, three
// days after a `mon 09:00` reset.
var grokPoolNow = time.Date(2026, 8, 27, 12, 0, 0, 0, time.Local)

// grokPoolReset is the reset every fixture configures, and grokPoolLastReset
// the instant it resolves to under grokPoolNow.
const grokPoolReset = "mon 09:00"

var grokPoolLastReset = time.Date(2026, 8, 24, 9, 0, 0, 0, time.Local)

// grokPoolCfg is the guard, fully configured. The conversion factor is a
// ROUND TEST NUMBER and deliberately not the operator's calibration: $0.50
// per point makes a full pool exactly $50, so every assertion below is
// arithmetic a reader can check, and no measured figure enters this repo.
const grokPoolCfg = "grok_pool_reset: " + grokPoolReset + "\ngrok_pool_usd_per_point: 0.50\n"

type grokPoolFixture struct {
	d    *Dispatcher
	errb *strings.Builder
	b    *HerdrBackend
	fake string
	home string
}

// grokPoolPass wires a pass whose ready bead routes to a persona pinned to
// grok, with no plan guard armed at all — the pool guard is the only brake
// in the way, which is what makes the skip lines below unambiguous.
func grokPoolPass(t *testing.T, cfg string) *grokPoolFixture {
	t.Helper()
	return grokPoolPassOn(t, cfg, "runtime: grok\n")
}

// grokPoolPassOn is the same pass with the persona's runtime line under the
// caller's control, so "a claude bead is not gated by grok's pool" is one
// character of difference from the case that is.
func grokPoolPassOn(t *testing.T, cfg, runtimeLine string) *grokPoolFixture {
	t.Helper()
	return grokPoolPassFull(t, cfg, runtimeLine, `["go"]`)
}

// grokPoolPassFull adds the ready bead's labels, for the one case that needs
// a tier on them: ADR 0010 will not move `strong` work to a second pool.
func grokPoolPassFull(t *testing.T, cfg, runtimeLine, beadLabels string) *grokPoolFixture {
	t.Helper()
	home := grokPoolHome(t)
	b, fake := newTestBackend(t)
	d, errb := planDispatcher(t, b, nil)
	d.Now = func() time.Time { return grokPoolNow }
	os.MkdirAll(b.App.AgentsDir, 0o755)
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\n" + runtimeLine + "---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":`+beadLabels+`}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, cfg)
	idleClaude(t, fake)
	return &grokPoolFixture{d: d, errb: errb, b: b, fake: fake, home: home}
}

// spend plants one session's worth of dollars inside the current week.
func (f *grokPoolFixture) spend(t *testing.T, id string, dollars float64) {
	t.Helper()
	at := grokPoolLastReset.Add(time.Hour)
	grokPoolSession(t, f.home, id,
		grokPoolUser(at, "Work beads issue rangerhq-myso (t)")+
			grokPoolTurn(at, "p-"+id, usdTicks(dollars)))
}

func (f *grokPoolFixture) run(t *testing.T) (int, string) {
	t.Helper()
	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	return n, dispatcherOut(f.d)
}
