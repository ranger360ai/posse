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

// ranger-base-zvgql: the rest of each home's executable members, at both
// mechanisms — dirs denied whole, files seeded file-shaped denies — and only
// over the home this seat's own runtime is granted.
func TestQAWallRuntimeConfigCarvesEachCLIsExecMembers(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	members := map[string][2][]string{
		"codex": {codexExecDirNames, codexExecFileNames},
		"grok":  {grokExecDirNames, grokExecFileNames},
	}
	// The members the measurement found read; a list edit that drops one
	// reds here, not in a seat.
	for name, must := range map[string][]string{
		"codex": {"skills", "plugins", "hooks.json", "AGENTS.md", "AGENTS.override.md",
			// ranger-base-6ob20 (finding 74pqi rows 3 and 4)
			".tmp", "memories"},
		"grok": {"hooks", "plugins", "installed-plugins", "skills", "commands", "rules", "agents", "lsp.json", "managed_config.toml", "requirements.toml", "trusted_folders.toml", "AGENTS.md", "CLAUDE.md",
			// ranger-base-6ob20: grok's own sandbox write-deny census, plus
			// the managed install, its channel and memory-v2 (74pqi 6-10)
			"bundled", "bundles", "hooks-paths", "sandbox.toml", "bin", "channel-switch.json", "memory-v2"},
	} {
		all := append(append([]string{}, members[name][0]...), members[name][1]...)
		for _, m := range must {
			if !containsString(all, m) {
				t.Errorf("%s: %s is not a carved member (ranger-base-zvgql measured it read)", name, m)
			}
		}
	}
	for _, name := range []string{"codex", "grok"} {
		rt := wrcRuntime(name)
		w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, rt.StateDirs...)
		c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, rt.StateDirs...)
		prof := SeatbeltProfile(f.ag.Name, w, nil, c)
		h := absResolve(filepath.Join(f.home, "."+name))
		for _, d := range members[name][0] {
			p := filepath.Join(h, d)
			if !containsString(c.Deny, p) || c.FileShaped(p) {
				t.Errorf("%s: %s must be a directory deny: deny=%v", name, p, c.Deny)
			}
			if !strings.Contains(prof, sbQuote(p)) {
				t.Errorf("%s: the seatbelt profile names no deny over %s", name, p)
			}
		}
		for _, fl := range members[name][1] {
			p := filepath.Join(h, fl)
			if !containsString(c.Deny, p) || !c.FileShaped(p) || !c.Seeded(p) {
				t.Errorf("%s: %s must be a seeded file-shaped deny: deny=%v seed=%v", name, p, c.Deny, c.Seed)
			}
			if !strings.Contains(prof, sbQuote(p)) {
				t.Errorf("%s: the seatbelt profile names no deny over %s", name, p)
			}
		}
		other := map[string]string{"codex": "grok", "grok": "codex"}[name]
		oh := absResolve(filepath.Join(f.home, "."+other))
		for _, d := range c.Deny {
			if underDir(oh, d) {
				t.Errorf("%s: the carve names %s, which nothing granted this seat", name, d)
			}
		}
	}
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, "~/.claude")
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, "~/.claude")
	for _, d := range c.Deny {
		for _, cli := range []string{".codex", ".grok"} {
			if underDir(absResolve(filepath.Join(f.home, cli)), d) {
				t.Errorf("a claude seat's carve names %s", d)
			}
		}
	}
	// CODEX_HOME moves the members with it.
	moved := filepath.Join(f.home, "elsewhere", "codex")
	t.Setenv(CodexHomeVar, moved)
	dirs, files := runtimeExecMembers(f.home, []string{absResolve(moved)})
	if want := filepath.Join(absResolve(moved), "hooks.json"); !containsString(files, want) || len(dirs) != len(codexExecDirNames) {
		t.Errorf("CODEX_HOME=%s: dirs %v files %v, want %s among them", moved, dirs, files, want)
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
		refused := map[string]string{
			"config.toml append":         "echo x >> " + q(cfg),
			"config.toml atomic replace": ": > " + q(tmp) + " && mv " + q(tmp) + " " + q(cfg),
		}
		// ranger-base-zvgql: every other executable member, both ways in.
		dirs, files := map[string][]string{"codex": codexExecDirNames, "grok": grokExecDirNames}[name],
			map[string][]string{"codex": codexExecFileNames, "grok": grokExecFileNames}[name]
		for _, d := range dirs {
			refused[d+"/ create"] = "mkdir -p " + q(filepath.Join(h, d, "x")) + " && : > " + q(filepath.Join(h, d, "x", "y"))
		}
		for _, fl := range files {
			p, pt := filepath.Join(h, fl), filepath.Join(h, "."+fl+".tmp")
			refused[fl+" append"] = "echo x >> " + q(p)
			refused[fl+" atomic replace"] = ": > " + q(pt) + " && mv " + q(pt) + " " + q(p)
		}
		for what, script := range refused {
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
		// ranger-base-zvgql: every file member is seeded empty and read-only
		// for this run, and must come out of it byte-identical. grok deletes
		// a managed_config.toml or requirements.toml it can delete.
		members := map[string]string{}
		for _, fl := range map[string][]string{"codex": codexExecFileNames, "grok": grokExecFileNames}[name] {
			p := filepath.Join(f.home, "."+name, fl)
			b, err := os.ReadFile(p)
			if err != nil {
				t.Errorf("%s: %s was not seeded by the render: %v", name, fl, err)
			}
			members[p] = string(b)
		}
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
		for p, b := range members {
			if after, err := os.ReadFile(p); err != nil || string(after) != b {
				t.Errorf("%s: %s changed under the wall (err=%v):\n%s", name, p, err, after)
			}
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

// A seed appears whole: never a zero-byte JSON file for a reader to choke
// on, never over a file that is already there, and no temp left beside it.
func TestQAWallRuntimeConfigSeedAppearsWhole(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	p := filepath.Join(d, "settings.json")
	if err := seedFileWhole(p, seedBody(p)); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "{}\n" {
		t.Errorf("seeded %q, want {}", b)
	}
	if err := os.WriteFile(p, []byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := seedFileWhole(p, seedBody(p)); !os.IsExist(err) {
		t.Errorf("a seed over an existing file: err=%v, want IsExist", err)
	}
	if b, _ := os.ReadFile(p); string(b) != `{"x":1}` {
		t.Errorf("a seed rewrote an existing file: %q", b)
	}
	md := filepath.Join(d, "AGENTS.md")
	if err := seedFileWhole(md, seedBody(md)); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(d)
	if len(ents) != 2 {
		t.Errorf("a seed left something beside it: %v", ents)
	}
}

// ranger-base-6ob20: codex's tmp/ and shell_snapshots/ hold one process's
// PATH aliases and sourced shell snapshot. At the bwrap mechanism a seat gets
// an empty private tmpfs over each, mounted after the home grant and covered
// by no read-only bind from the host; at seatbelt, which cannot mount, both
// are directory denies. A seat on another runtime gets neither.
func TestQAWallRuntimeConfigCodexEphemeralDirsArePrivateOrReadOnly(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	h := absResolve(filepath.Join(f.home, ".codex"))
	eph := []string{filepath.Join(h, "shell_snapshots"), filepath.Join(h, "tmp")}

	rt := wrcRuntime("codex")
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, rt.StateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, rt.StateDirs...)
	prof := SeatbeltProfile(f.ag.Name, w, nil, c)
	for _, d := range eph {
		if !containsString(c.Deny, d) || c.FileShaped(d) {
			t.Errorf("seatbelt: %s must be a directory deny (ranger-base-6ob20): deny=%v", d, c.Deny)
		}
		if !strings.Contains(prof, sbQuote(d)) {
			t.Errorf("seatbelt: the profile names no deny over %s", d)
		}
	}

	r, err := f.a.bwrapRender(f.ag, rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	grant := bindIndex(r.Argv, h, h)
	if grant < 0 {
		t.Fatalf("the fixture's codex home is not granted, so nothing here is measured: %q", r.Argv)
	}
	for _, d := range eph {
		if containsString(r.Carve.Deny, d) {
			t.Errorf("bwrap: %s is still a deny, and its read-only bind from the host would cover the private tmpfs", d)
		}
		at := -1
		for i := 0; i+1 < len(r.Argv); i++ {
			if r.Argv[i] == "--tmpfs" && r.Argv[i+1] == d {
				at = i
			}
			if (r.Argv[i] == "--ro-bind" || r.Argv[i] == "--bind") && i+2 < len(r.Argv) && r.Argv[i+2] == d {
				t.Errorf("bwrap: %s is bound from the host at %d (%s %s) — the seat would see the shared dir", d, i, r.Argv[i], r.Argv[i+1])
			}
		}
		switch {
		case at < 0:
			t.Errorf("bwrap: no private tmpfs over %s (ranger-base-6ob20)\n argv: %q", d, r.Argv)
		case at < grant:
			t.Errorf("bwrap: the tmpfs over %s comes before the home grant at %d, so the grant covers it again", d, grant)
		}
	}

	// grok and claude seats: nothing over codex's home.
	for _, other := range []*Runtime{wrcRuntime("grok"), {Name: "claude", StateDirs: []string{"~/.claude"}}} {
		ow := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, other.StateDirs...)
		oc := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, ow, other.StateDirs...)
		for _, d := range eph {
			if containsString(oc.Deny, d) {
				t.Errorf("%s: the carve names %s, which nothing granted this seat", other.Name, d)
			}
		}
		if other.Name != "grok" {
			continue
		}
		or, err := f.a.bwrapRender(f.ag, other, f.dir, f.gates)
		if err != nil {
			t.Fatal(err)
		}
		for _, pb := range or.Carve.Private {
			if underDir(h, pb.Dst) {
				t.Errorf("grok: a private mount over %s, in a home nothing granted this seat", pb.Dst)
			}
		}
	}

	// CODEX_HOME moves them.
	moved := filepath.Join(f.home, "elsewhere", "codex")
	t.Setenv(CodexHomeVar, moved)
	if got := codexEphemeralDirs(f.home, []string{absResolve(moved)}); len(got) != 2 || got[1] != filepath.Join(absResolve(moved), "tmp") {
		t.Errorf("CODEX_HOME=%s: %v", moved, got)
	}
	if got := codexEphemeralDirs(f.home, []string{h}); len(got) != 0 {
		t.Errorf("CODEX_HOME moved away from the grant, yet %v", got)
	}
}

// The live half of the private tmpfs: under posse's render and the real
// bwrap, what another process put in the host's $CODEX_HOME/tmp and
// shell_snapshots/ is NOT visible from inside the seat, the seat can write
// its own there, and nothing it writes or moves reaches the host's dirs.
func TestLiveWallRuntimeConfigCodexEphemeralDirsArePrivate(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	wrcNeedBwrap(t)
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	h := filepath.Join(f.home, ".codex")
	alias := filepath.Join(h, "tmp", "arg0-linux", "codex-arg0HOST", "git")
	snap := filepath.Join(h, "shell_snapshots", "host-thread.sh")
	for _, p := range []string{alias, snap} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# another process's\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	try := func(script string) (bool, string) {
		out, err := exec.Command("bwrap", append(append([]string{}, r.Argv...), "/bin/sh", "-c", script)...).CombinedOutput()
		return err == nil, strings.TrimSpace(string(out))
	}
	q := shellQuote
	tmp, snaps := filepath.Join(h, "tmp"), filepath.Join(h, "shell_snapshots")
	for what, script := range map[string]string{
		"host alias visible":    "test -e " + q(alias),
		"host snapshot visible": "test -e " + q(snap),
		"host tmp listed":       "ls -A " + q(tmp) + " | grep -q .",
		"host snapshots listed": "ls -A " + q(snaps) + " | grep -q .",
	} {
		if ok, out := try(script); ok {
			t.Errorf("%s: another process's per-process codex state is visible inside the seat (ranger-base-6ob20)\n out: %s", what, out)
		}
	}
	for what, script := range map[string]string{
		"own alias":    "mkdir -p " + q(filepath.Dir(alias)) + " && echo SEAT > " + q(alias),
		"own snapshot": "echo SEAT > " + q(snap) + " && echo SEAT > " + q(filepath.Join(snaps, "seat.sh")),
	} {
		if ok, out := try(script); !ok {
			t.Errorf("%s: REFUSED in the private tmpfs — codex writes its own here\n out: %s", what, out)
		}
	}
	for what, script := range map[string]string{
		"tmp moved":       "mv " + q(tmp) + " " + q(tmp+".old"),
		"snapshots moved": "mv " + q(snaps) + " " + q(snaps+".old"),
	} {
		_, _ = try(script)
		if _, err := os.Stat(filepath.Join(h, filepath.Base(tmp)+".old")); err == nil {
			t.Errorf("%s: the host's tmp/ was renamed out from under the mount", what)
		}
		if _, err := os.Stat(snaps + ".old"); err == nil {
			t.Errorf("%s: the host's shell_snapshots/ was renamed out from under the mount", what)
		}
	}
	for _, p := range []string{alias, snap} {
		if b, err := os.ReadFile(p); err != nil || string(b) != "# another process's\n" {
			t.Errorf("%s changed on the host (%q, %v)", p, b, err)
		}
	}
	if _, err := os.Stat(filepath.Join(snaps, "seat.sh")); err == nil {
		t.Errorf("the seat's own snapshot reached the host's shell_snapshots/")
	}
}

// The seatbelt reading, measured where it can be: codex completes a turn with
// tmp/ and shell_snapshots/ READ-ONLY, which is what a darwin seat gets. The
// render is posse's own with the private tmpfs swapped back for the deny
// SeatbeltCarveOut made, so only those two mounts differ.
func TestLiveWallRuntimeConfigCodexRunsWithEphemeralDirsReadOnly(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	wrcNeedBwrap(t)
	bin := wrcRealBinary("codex")
	if bin == "" {
		t.Skip("NOTHING MEASURED: codex does not resolve to a binary on this host")
	}
	srv := httptest.NewServer(http.HandlerFunc(wrcFakeModel))
	defer srv.Close()
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	base, _, err := BwrapArgs(f.dir, r.Set, r.Carve, r.Keep...)
	if err != nil {
		t.Fatal(err)
	}
	tail := r.Argv[len(base):] // the --setenv lines bwrapRender adds after BwrapArgs
	ro := r.Carve
	ro.Private = nil
	eph := codexEphemeralDirs(f.home, runtimeStateGrants(wrcRuntime("codex").StateDirs))
	if len(eph) != 2 {
		t.Fatalf("no ephemeral dirs for the fixture's codex home: %v", eph)
	}
	for _, pb := range r.Carve.Private {
		if !containsString(eph, pb.Dst) {
			ro.Private = append(ro.Private, pb)
		}
	}
	ro.Deny = append(append([]string{}, r.Carve.Deny...), eph...)
	argv, prep, err := BwrapArgs(f.dir, r.Set, ro, r.Keep...)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(prep); err != nil {
		t.Fatal(err)
	}
	argv = append(argv, tail...)
	cmd := exec.Command("bwrap", append(append(argv, bin), "exec", "--skip-git-repo-check",
		"-c", `model_provider="fake"`,
		"-c", fmt.Sprintf(`model_providers.fake={name="fake",base_url=%q,wire_api="responses",env_key="FAKE_KEY"}`, srv.URL+"/v1"),
		"hi")...)
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(), "FAKE_KEY=sk-dummy")
	out, err := cmd.CombinedOutput()
	t.Logf("codex with tmp/ and shell_snapshots/ read-only: err=%v, reply seen=%v, aliases refused=%v", err, strings.Contains(string(out), wrcReply), strings.Contains(string(out), "PATH aliases"))
	if err != nil || !strings.Contains(string(out), wrcReply) {
		t.Errorf("codex did not complete a turn with tmp/ and shell_snapshots/ read-only (the seatbelt reading): %v\n%s", err, out)
	}
	for _, d := range eph {
		if ents, _ := os.ReadDir(d); len(ents) != 0 {
			t.Errorf("%s gained %d entries under a read-only mount", d, len(ents))
		}
	}
}

// grok's line carries its own updater switch, and only grok's does.
func TestQAWallRuntimeConfigGrokLineDisablesTheUpdater(t *testing.T) {
	t.Parallel()
	want := GrokDisableAutoUpdaterVar + "='1' "
	if got := RuntimeLineEnv(builtinRuntime(t, "grok")); got != want {
		t.Errorf("grok's line env %q, want %q (ranger-base-6ob20)", got, want)
	}
	for _, name := range []string{"claude", "codex"} {
		if got := RuntimeLineEnv(builtinRuntime(t, name)); got != "" {
			t.Errorf("%s's line carries %q", name, got)
		}
	}
	// And it reaches the line WrapWithGates types, beside the home pin, and
	// the shell then hands it to grok.
	a := &App{Home: t.TempDir(), StateDir: t.TempDir()}
	wrapped, _, _, err := a.WrapWithGates("wrc-line", builtinRuntime(t, "grok"), nil, t.TempDir(), "sh -c 'echo v=$"+GrokDisableAutoUpdaterVar+"'")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wrapped, RuntimeHomePin(builtinRuntime(t, "grok"))+want) {
		t.Errorf("the wrapped grok line does not carry %q after the home pin: %q", want, wrapped)
	}
	if out, _ := exec.Command("sh", "-c", wrapped).CombinedOutput(); !strings.Contains(string(out), "v=1") {
		t.Errorf("the variable did not reach the command: %s", out)
	}
}
