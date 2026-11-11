//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-zmk4q, ADR 0090 D3: codex's and grok's config.toml
// carries commands, and a caged seat on either runtime gets it read-only over
// its own state grant. runtimeConfigFiles has the measurement.
//
// Three halves. The render pins say what posse writes into the wall, at both
// mechanisms, from any host. The live wall pin hands that render to the real
// bwrap and tries the writes. The live CLI pin runs each CLI under the same
// render against a local fake endpoint, so a release that stops running with
// the file read-only reds here instead of in a seat — no turn is billed.
//
//	go test ./internal/posse -run 'WallRuntimeConfig' -v

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type wrcFixture struct {
	home, dir, gates string
	a                *App
	ag               *AgentFile
}

// wrcFixtureIn is a box in miniature: a home holding both CLIs' homes, each
// with a config.toml and a sessions dir, and a posse home. Neither home
// variable is in force, so each CLI resolves `<home>/.<cli>`.
func wrcFixtureIn(t *testing.T, root string) wrcFixture {
	t.Helper()
	at := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	f := wrcFixture{home: at("home"), dir: at("proj")}
	for _, d := range []string{"home/.codex/sessions", "home/.grok/sessions", "home/.cache", "posse/state", "posse/personas/developer", "proj"} {
		if err := os.MkdirAll(at(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"home/.codex/config.toml", "home/.grok/config.toml"} {
		if err := os.WriteFile(at(p), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", f.home)
	t.Setenv(CodexHomeVar, "")
	t.Setenv(GrokHomeVar, "")
	t.Setenv(ClaudeConfigDirVar, at("home", ".claude"))
	f.a = &App{Home: at("posse"), StateDir: at("posse", "state")}
	f.ag = &AgentFile{Name: "developer", MemoryDir: at("posse", "personas", "developer")}
	f.gates = f.a.GatesDir(f.ag.Name)
	if err := os.MkdirAll(f.gates, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func wrcRuntime(name string) *Runtime {
	return &Runtime{Name: name, StateDirs: []string{"~/." + name}}
}

// The carve, at both mechanisms: a codex seat's config.toml and a grok seat's
// are seeded file-shaped denies, the seatbelt profile names them, and neither
// seat's carve names the other's file — each narrows its own grant.
func TestQAWallRuntimeConfigCarvesEachCLIsConfigToml(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	for _, name := range []string{"codex", "grok"} {
		rt := wrcRuntime(name)
		w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, rt.StateDirs...)
		c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, rt.StateDirs...)
		own := absResolve(filepath.Join(f.home, "."+name, "config.toml"))
		if !containsString(c.Deny, own) || !c.FileShaped(own) || !c.Seeded(own) {
			t.Errorf("%s: %s must be a seeded file-shaped deny (ADR 0090 D3): deny=%v seed=%v", name, own, c.Deny, c.Seed)
		}
		if prof := SeatbeltProfile(f.ag.Name, w, nil, c); !strings.Contains(prof, sbQuote(own)) {
			t.Errorf("%s: the seatbelt profile names no deny over %s", name, own)
		}
		other := map[string]string{"codex": "grok", "grok": "codex"}[name]
		if p := absResolve(filepath.Join(f.home, "."+other, "config.toml")); containsString(c.Deny, p) {
			t.Errorf("%s: the carve names %s, which nothing granted this seat", name, p)
		}
	}
	// A claude seat is granted neither home and gets neither deny.
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, "~/.claude")
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, "~/.claude")
	for _, d := range c.Deny {
		if strings.HasSuffix(d, "config.toml") {
			t.Errorf("a claude seat's carve names %s", d)
		}
	}
}

// The home variable moves the carve with it: the file walled is the file the
// CLI reads (ADR 0077 D1), and a grant that does not reach it carves nothing.
func TestQAWallRuntimeConfigFollowsTheHomeVariable(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	moved := filepath.Join(f.home, "elsewhere", "codex")
	t.Setenv(CodexHomeVar, moved)
	want := absResolve(filepath.Join(moved, "config.toml"))
	if got := runtimeConfigFiles(f.home, []string{absResolve(moved)}); len(got) != 1 || got[0] != want {
		t.Errorf("CODEX_HOME=%s: carve %v, want [%s]", moved, got, want)
	}
	if got := runtimeConfigFiles(f.home, []string{absResolve(filepath.Join(f.home, ".codex"))}); len(got) != 0 {
		t.Errorf("CODEX_HOME moved away from the grant, yet the carve names %v", got)
	}
}

// The live join: posse's own render, the real bwrap, the config.toml write
// refused both ways a writer makes it, and the rest of the CLI's home still
// writable.
func TestLiveWallRuntimeConfigRefusesTheWriteUnderTheRenderedWall(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	wrcNeedBwrap(t)
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	q := shellQuote
	for _, name := range []string{"codex", "grok"} {
		r, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err != nil {
			t.Fatal(err)
		}
		if err := runBwrapPrep(r.Prep); err != nil {
			t.Fatal(err)
		}
		h := filepath.Join(f.home, "."+name)
		cfg, tmp := filepath.Join(h, "config.toml"), filepath.Join(h, ".config.toml.tmp")
		try := func(script string) (bool, string) {
			out, err := exec.Command("bwrap", append(append([]string{}, r.Argv...), "/bin/sh", "-c", script)...).CombinedOutput()
			return err == nil, strings.TrimSpace(string(out))
		}
		for what, script := range map[string]string{
			"config.toml append":         "echo x >> " + q(cfg),
			"config.toml atomic replace": ": > " + q(tmp) + " && mv " + q(tmp) + " " + q(cfg),
		} {
			if ok, out := try(script); ok {
				t.Errorf("%s: %s ALLOWED under the rendered wall (ADR 0090 D3)\n out: %s", name, what, out)
			}
		}
		for what, script := range map[string]string{
			"sessions write":   ": > " + q(filepath.Join(h, "sessions", "x")),
			"new home member":  "mkdir -p " + q(filepath.Join(h, "new", "x")),
			"temp file create": ": > " + q(tmp),
		} {
			if ok, out := try(script); !ok {
				t.Errorf("%s: %s REFUSED under the rendered wall — the CLI writes its home\n out: %s", name, what, out)
			}
		}
	}
}

// The CLI half of D3's rule: each CLI, started under posse's render with its
// config.toml read-only, completes a turn. The endpoint is a local fake, so
// nothing is billed and no login is needed. A release that refuses to run
// with the file read-only reds here, and the carve must then be re-decided.
func TestLiveWallRuntimeConfigEachCLIRunsWithItsConfigReadOnly(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	wrcNeedBwrap(t)
	// Resolved before HOME moves: mise's command wrappers install afresh
	// under a home that has never seen the tool.
	bins := map[string]string{}
	for _, name := range []string{"codex", "grok"} {
		if b := wrcRealBinary(name); b != "" {
			bins[name] = b
		}
	}
	if len(bins) == 0 {
		t.Skip("NOTHING MEASURED: neither codex nor grok resolves to a binary on this host")
	}
	srv := httptest.NewServer(http.HandlerFunc(wrcFakeModel))
	defer srv.Close()
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	if err := os.WriteFile(filepath.Join(f.home, ".grok", "config.toml"), []byte(fmt.Sprintf(
		"[models]\ndefault = \"fake\"\n\n[model.fake]\nmodel = \"fake\"\nbase_url = %q\nenv_key = \"FAKE_KEY\"\napi_backend = \"chat_completions\"\n",
		srv.URL+"/v1")), 0o644); err != nil {
		t.Fatal(err)
	}
	argv := map[string][]string{
		"codex": {"exec", "--skip-git-repo-check",
			"-c", `model_provider="fake"`,
			"-c", fmt.Sprintf(`model_providers.fake={name="fake",base_url=%q,wire_api="responses",env_key="FAKE_KEY"}`, srv.URL+"/v1"),
			"hi"},
		"grok": {"--permission-mode", "auto", "-m", "fake", "-p", "hi"},
	}
	for name, bin := range bins {
		r, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err != nil {
			t.Fatal(err)
		}
		if err := runBwrapPrep(r.Prep); err != nil {
			t.Fatal(err)
		}
		cfg := filepath.Join(f.home, "."+name, "config.toml")
		before, _ := os.ReadFile(cfg)
		cmd := exec.Command("bwrap", append(append(append([]string{}, r.Argv...), bin), argv[name]...)...)
		cmd.Dir = f.dir
		cmd.Env = append(os.Environ(), "FAKE_KEY=sk-dummy")
		out, err := cmd.CombinedOutput()
		t.Logf("%s (%s) under the rendered wall: err=%v, reply seen=%v", name, bin, err, strings.Contains(string(out), wrcReply))
		if err != nil || !strings.Contains(string(out), wrcReply) {
			t.Errorf("%s did not complete a turn with config.toml read-only under the rendered wall: %v\n%s", name, err, out)
		}
		if after, _ := os.ReadFile(cfg); string(after) != string(before) {
			t.Errorf("%s: config.toml changed under the wall:\n%s", name, after)
		}
	}
}

func wrcNeedBwrap(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	if why := liveBwrapApplyRefusal(); why != "" {
		t.Skip("NOTHING MEASURED: " + why)
	}
}

// wrcRealBinary is the CLI's own binary, past mise's command wrapper when
// there is one, or "" when the host has none.
func wrcRealBinary(name string) string {
	if out, err := exec.Command("mise", "which", name).Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return p
		}
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// wrcReply is the fake model's whole answer: a token no prompt or banner
// carries, so finding it in the output means a turn completed.
const wrcReply = "wrc-turn-complete-zmk4q"

// wrcFakeModel answers one streamed turn with wrcReply, in OpenAI's Responses
// shape (codex) or its Chat Completions shape (grok), and a model list for
// any GET.
func wrcFakeModel(w http.ResponseWriter, r *http.Request) {
	data := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	if r.Method == http.MethodGet {
		w.Header().Set("content-type", "application/json")
		fmt.Fprint(w, `{"object":"list","models":[],"data":[{"id":"fake","object":"model"}]}`)
		return
	}
	w.Header().Set("content-type", "text/event-stream")
	if strings.HasSuffix(r.URL.Path, "/chat/completions") {
		base := map[string]any{"id": "c1", "object": "chat.completion.chunk", "created": 1, "model": "fake"}
		with := func(k string, v any) map[string]any {
			m := map[string]any{k: v}
			for bk, bv := range base {
				m[bk] = bv
			}
			return m
		}
		fmt.Fprintf(w, "data: %s\n\n", data(with("choices", []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": wrcReply}}})))
		fin := with("choices", []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}})
		fin["usage"] = map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data(fin))
		return
	}
	ev := func(typ string, v map[string]any) {
		v["type"] = typ
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ, data(v))
	}
	ev("response.created", map[string]any{"response": map[string]any{"id": "r1"}})
	ev("response.output_item.done", map[string]any{"item": map[string]any{"type": "message", "role": "assistant", "id": "m1",
		"content": []any{map[string]any{"type": "output_text", "text": wrcReply}}}})
	ev("response.completed", map[string]any{"response": map[string]any{"id": "r1", "usage": map[string]any{
		"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}}})
}
