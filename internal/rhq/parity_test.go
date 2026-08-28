package rhq

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ADR 0002 §4: the realization matrix — what counts as the wall.
func TestCheckParity(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	claude, _ := a.LoadRuntime("claude")
	codex, _ := a.LoadRuntime("codex")
	grok, _ := a.LoadRuntime("grok")
	security := loadTestAgent(t, "---\nname: security\ndeny: [Edit, Write, Bash(git push:*)]\n---\nYou are security.\n")

	// security on codex at shims: read-only + shim realize all three in the
	// directory-independent matrix; a concrete repo may add L3 below.
	p := a.CheckParity(security, codex, CageShims, TierStrong)
	if len(p.Degraded) != 0 || p.Realized["Edit"] != "codex sandbox (OS-enforced)" || p.Realized["Bash(git push:*)"] != "L1 shim (subcommand, option-aware)" {
		t.Errorf("security@codex: %+v", p)
	}
	// security on grok (and claude): Edit/Write are politeness → degraded,
	// but the shell verb is the wall's on both (ADR 0009).
	for _, rt := range []*Runtime{grok, claude} {
		p := a.CheckParity(security, rt, CageShims, TierStrong)
		if len(p.Unrealized) != 2 || !strings.HasPrefix(p.Unrealized[0], "Edit — needs cage: seatbelt") || p.Realized["Bash(git push:*)"] == "" {
			t.Errorf("security@%s: %+v", rt.Name, p)
		}
	}
	// rangerhq-2zm: parity may claim only what the shim's matcher really
	// does. A subcommand deny on a command whose global options we do not
	// know is best-effort — an option taking a separate value hides the
	// verb — so it must degrade the launch, not read as a wall.
	npm := loadTestAgent(t, "---\nname: npm\ndeny: [Bash(npm publish:*)]\n---\nYou are npm.\n")
	pn := a.CheckParity(npm, claude, CageShims, TierStrong)
	if len(pn.Unrealized) != 1 || !strings.Contains(pn.Unrealized[0], "no global-option table for npm") || pn.Realized["Bash(npm publish:*)"] != "" {
		t.Errorf("subcommand deny without an option table must not read as realized: %+v", pn)
	}
	// Denying the whole verb is the matcher-independent fix the message
	// points at, and it does realize the gate.
	npmAll := loadTestAgent(t, "---\nname: npm\ndeny: [Bash(npm)]\n---\nYou are npm.\n")
	if p := a.CheckParity(npmAll, claude, CageShims, TierStrong); len(p.Degraded) != 0 || p.Realized["Bash(npm)"] != "L1 shim (whole verb)" {
		t.Errorf("whole-verb deny is matcher-independent: %+v", p)
	}
	// git has a table, so its subcommand denies are option-aware. L3 is a
	// directory fact and is deliberately absent here.
	if p := a.CheckParity(security, claude, CageShims, TierStrong); p.Realized["Bash(git push:*)"] != "L1 shim (subcommand, option-aware)" {
		t.Errorf("git push layers: %q", p.Realized["Bash(git push:*)"])
	}

	// A shell-verb-only PID is fully realized on every runtime — grok
	// included. It re-execs a login shell per command, so path_helper used
	// to demote the gates dir and L1 never ran (rangerhq-vjl); the gate
	// shell of ADR 0009 puts the guard inside that shell's own -c string,
	// and the wall is counted again by construction rather than by knob.
	dev := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(rm -rf /)\n---\nYou are dev.\n")
	for _, rt := range []*Runtime{claude, codex, grok} {
		p := a.CheckParity(dev, rt, CageShims, TierStrong)
		if len(p.Degraded) != 0 || p.Realized["Bash(rm -rf /)"] != "L1 shim (literal argv prefix)" {
			t.Errorf("dev@%s must be clean: %+v", rt.Name, p)
		}
	}
	// The exit hatch is the only thing that gives that back: a runtime with
	// gate_shell: false gets no wrapper on the typed line. The dir-independent
	// matrix counts no L3 possibility as realized; CheckParityIn can replace
	// the git verdict only after it executes a concrete repo's hook.
	nogs := &Runtime{Name: "odd", NoGateShell: true}
	pg := a.CheckParity(dev, nogs, CageShims, TierStrong)
	if len(pg.Unrealized) != 2 || !strings.Contains(strings.Join(pg.Unrealized, "\n"), "Bash(rm -rf /) — L1 shim cannot hold on odd (gate_shell: false)") || pg.Realized["Bash(git push:*)"] != "" {
		t.Errorf("dev@odd: %+v", pg)
	}
	// Tool-name denies are container-only; egress implies container; a
	// PID demanding a tier this build lacks is degraded by that alone.
	web := loadTestAgent(t, "---\nname: web\ndeny: [WebFetch, mcp__x__y]\negress: [api.example.com]\n---\nYou are web.\n")
	if web.Cage != CageContainer {
		t.Errorf("egress: must imply cage: container, got %q", web.Cage)
	}
	p = a.CheckParity(web, claude, CageShims, TierStrong)
	joined := strings.Join(p.Degraded, "\n")
	for _, want := range []string{"cage: PID demands container, launching at shims", "WebFetch — runtime-native only below cage: container", "mcp__x__y", "egress: api.example.com — container tier only"} {
		if !strings.Contains(joined, want) {
			t.Errorf("web@claude missing %q in:\n%s", want, joined)
		}
	}
	// Demanding seatbelt today is degrading (not available yet), and would
	// realize Edit/Write once it is.
	sb := loadTestAgent(t, "---\nname: sb\ncage: seatbelt\ndeny: [Edit]\n---\nYou are sb.\n")
	had := AvailableCages[CageSeatbelt]
	delete(AvailableCages, CageSeatbelt)
	p = a.CheckParity(sb, claude, CageSeatbelt, TierStrong)
	if !strings.Contains(strings.Join(p.Degraded, "\n"), "cage seatbelt is not available on this host") {
		t.Errorf("unavailable cage must degrade: %+v", p)
	}
	AvailableCages[CageSeatbelt] = true
	defer func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	}()
	if p = a.CheckParity(sb, claude, CageSeatbelt, TierStrong); len(p.Degraded) != 0 || p.Realized["Edit"] != "L2 seatbelt" {
		t.Errorf("seatbelt must realize Edit once available: %+v", p)
	}
	// Bad cage values are agent-check findings.
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "bad.md"), []byte("---\nname: bad\ncage: jail\n---\nYou are bad.\n"), 0o644)
	if fs, _, _ := a.CheckAgent("bad"); !strings.Contains(strings.Join(fs, "\n"), `cage: "jail"`) {
		t.Errorf("agent check cage: %v", fs)
	}
	if err := degradedError(degradedError{p: a.CheckParity(security, grok, CageShims, TierStrong)}); !strings.Contains(err.Error(), "--allow-degraded") {
		t.Errorf("degraded error must point at the flag: %v", err)
	}
}

// Dispatch never allows degradation on its own; with --allow-degraded the
// session launches marked.
func TestDispatchRefusesDegradedUnlessAllowed(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "security.md"), []byte("---\nname: security\nlabels: [security]\ndeny: [Edit, Write, Bash(git push:*)]\n---\nYou are security.\n"), 0o644)
	repo := qaRepo(t, b.App, `[{"id":"s-1","title":"t","labels":["security"]}]`, `[{"id":"s-1","status":"closed"}]`)
	agentPerLaunch(t, fake)

	n, _ := d.Run("", "", 0)
	out := dispatcherOut(d)
	if n != 0 || !strings.Contains(out, "refused: claude at cage shims, tier strong does not realize every gate") || strings.Contains(calls(t, fake), "workspace create") {
		t.Errorf("dispatch must refuse a degraded launch, got n=%d:\n%s", n, out)
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Error("refused launch must not claim")
	}
	d2 := newTestDispatcher(t, b)
	d2.AllowDegraded = true
	b.Warn = d2.Out.(*strings.Builder)
	if n, _ := d2.Run("", "", 0); n != 1 {
		t.Fatalf("--allow-degraded must launch:\n%s", dispatcherOut(d2))
	}
	m, _ := b.readMeta(SessionForBead("security", repo, "s-1"))
	if m == nil || !strings.Contains(m.Degraded, "Edit") || !strings.Contains(dispatcherOut(d2), "DEGRADED") {
		t.Errorf("degraded session must be marked and announced: %+v\n%s", m, dispatcherOut(d2))
	}
	// codex realizes the security PID's gates: dispatch --runtime codex
	// launches clean.
	d3 := newTestDispatcher(t, b)
	d3.Runtime = "codex"
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(`[{"id":"s-2","title":"u","labels":["security"]}]`), 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"), []byte(`[{"id":"s-2","status":"closed"}]`), 0o644)
	if n, _ := d3.Run("", "", 0); n != 1 {
		t.Errorf("security on codex must launch at shims:\n%s", dispatcherOut(d3))
	}
	if m, _ := b.readMeta(SessionForBead("security", repo, "s-2")); m == nil || m.Degraded != "" || m.Cage != "shims" {
		t.Errorf("codex session must be full parity: %+v", m)
	}
}

// L2 seatbelt (rangerhq-5vt): the profile denies file-write* except the
// session's legitimate targets; with Edit/Write denied the repo itself is
// read-only but .beads/ and .git/ stay writable; the typed command wraps
// the runtime in sandbox-exec; codex cannot be wrapped (nested seatbelt).
func TestSeatbeltProfileAndLaunch(t *testing.T) {
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "security.md"),
		[]byte("---\nname: security\ncage: seatbelt\ndeny: [Edit, Write, Bash(git push:*)]\nwritable: [scratch, /opt/shared]\n---\nYou are security.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ncage: seatbelt\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	repo := t.TempDir()
	security, _ := b.App.LoadAgent("security")
	dev, _ := b.App.LoadAgent("dev")
	gates := b.App.GatesDir("security")

	w := b.App.SeatbeltWritable(security, repo, gates)
	joined := strings.Join(w, "\n")
	real := resolveExisting(repo)
	for _, want := range []string{filepath.Join(real, ".beads"), filepath.Join(real, ".git"), security.MemoryDir, resolveExisting(gates), filepath.Join(real, "scratch"), "/opt/shared"} {
		if !strings.Contains(joined, want) {
			t.Errorf("writable set missing %q:\n%s", want, joined)
		}
	}
	for _, x := range w {
		if x == real {
			t.Error("repo itself must not be writable when Edit/Write are denied")
		}
	}
	if wd := b.App.SeatbeltWritable(dev, repo, gates); !strings.Contains(strings.Join(wd, "\n"), real+"\n") && wd[0] != real {
		t.Errorf("without Edit/Write denies the repo is writable: %v", wd)
	}
	prof := SeatbeltProfile("security", w, SeatbeltCarveOut{})
	for _, want := range []string{"(version 1)", "(allow default)", "(deny file-write*)", `(subpath "` + filepath.Join(real, ".beads") + `")`, `(regex #"^/dev/")`} {
		if !strings.Contains(prof, want) {
			t.Errorf("profile missing %q:\n%s", want, prof)
		}
	}
	// Parity: security at seatbelt on claude/grok is clean when the host has
	// sandbox-exec; codex at seatbelt is flagged incompatible.
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	defer func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	}()
	claude, _ := b.App.LoadRuntime("claude")
	grok, _ := b.App.LoadRuntime("grok")
	codex, _ := b.App.LoadRuntime("codex")
	for _, rt := range []*Runtime{claude, grok} {
		if p := b.App.CheckParity(security, rt, CageSeatbelt, TierStrong); len(p.Degraded) != 0 || p.Realized["Edit"] != "L2 seatbelt" {
			t.Errorf("security@%s seatbelt: %+v", rt.Name, p)
		}
	}
	if p := b.App.CheckParity(security, codex, CageSeatbelt, TierStrong); len(p.Degraded) != 1 || !strings.Contains(p.Degraded[0], "does not nest") || p.Realized["Edit"] != "codex sandbox (OS-enforced)" {
		t.Errorf("security@codex seatbelt must be flagged incompatible, Edit still enforced by codex: %+v", p)
	}
	// Launch: the typed command is PATH=… sandbox-exec -f <profile> grok …
	mustCreate(t, b, NewSessionOpts{Name: "hg", Agent: "security", Runtime: "grok", Dir: repo})
	log := calls(t, fake)
	profPath := filepath.Join(gates, "seatbelt.sb")
	if !strings.Contains(log, `GATES sandbox-exec -f '`+profPath+`' grok`) || !strings.Contains(log, "--env RHQ_CAGE=seatbelt") {
		t.Errorf("seatbelt wrap missing:\n%s", log)
	}
	if _, err := os.Stat(profPath); err != nil {
		t.Error("profile not rendered")
	}
	m, _ := b.readMeta("hg")
	if m.Cage != CageSeatbelt || m.Degraded != "" || m.Dir != repo {
		t.Errorf("meta: %+v", m)
	}
	// --cage shims on the same PID demands less than the PID → degraded.
	if err := b.CreateSession(NewSessionOpts{Name: "hs", Agent: "security", Runtime: "grok", Cage: CageShims}); err == nil || !strings.Contains(err.Error(), "PID demands seatbelt") {
		t.Errorf("launching below the PID's cage must refuse: %v", err)
	}
	// Relaunch re-wraps with the seatbelt for the recorded dir.
	os.Remove(filepath.Join(fake, "agents.json"))
	m.Launched = m.Launched.Add(-time.Hour)
	b.writeMeta(m)
	if ok, err := b.RelaunchAgent("hg", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	if got := calls(t, fake); strings.Count(got, "sandbox-exec -f '"+profPath+"' grok") != 2 {
		t.Errorf("relaunch must keep the seatbelt:\n%s", got)
	}
}

// ADR 0003 §3: cheaper models read the PID's prose less reliably, so tier
// fast runs only where the wall realizes every gate — no --allow-degraded
// there, ever — and a PID's tier_floor: refuses anything cheaper in the
// same shape as an unrealized gate.
func TestFastNeedsFullParityAndTierFloor(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	claude, _ := a.LoadRuntime("claude")
	codex, _ := a.LoadRuntime("codex")
	security := loadTestAgent(t, "---\nname: security\ndeny: [Edit, Write, Bash(git push:*)]\n---\nYou are security.\n")

	// security on claude at shims: Edit/Write are politeness. At strong that
	// is degradation the operator may accept; at fast it is a refusal.
	if p := a.CheckParity(security, claude, CageShims, TierStrong); p.NoDegrade || len(p.Unrealized) != 2 {
		t.Errorf("strong: gate degradation stays waivable: %+v", p)
	}
	if err := a.CheckTier(security, claude, CageShims, TierStrong, false); err != nil {
		t.Errorf("an unrealized gate at strong is the launch's call, not the tier's: %v", err)
	}
	p := a.CheckParity(security, claude, CageShims, TierFast)
	if !p.NoDegrade || len(p.Unrealized) != 2 {
		t.Errorf("fast + unrealized gates must be unwaivable: %+v", p)
	}
	err := a.CheckTier(security, claude, CageShims, TierFast, true)
	if err == nil || !strings.Contains(err.Error(), "--allow-degraded is never accepted") ||
		!strings.Contains(err.Error(), "tier fast does not realize every gate") {
		t.Errorf("--allow-degraded must not buy fast: %v", err)
	}
	// The two ways to earn fast: a runtime that enforces the gates, or a
	// cage that does.
	if err := a.CheckTier(security, codex, CageShims, TierFast, false); err != nil {
		t.Errorf("codex -s read-only realizes Edit/Write — fast is honest there: %v", err)
	}
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	defer func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	}()
	if err := a.CheckTier(security, claude, CageSeatbelt, TierFast, false); err != nil {
		t.Errorf("cage seatbelt realizes Edit/Write — fast is honest there: %v", err)
	}

	// tier_floor: the business manager's "no commitments" lives in prose only
	// (ADR 0003 §3).
	bizmgr := loadTestAgent(t, "---\nname: business-manager\ntier: standard\ntier_floor: standard\ndeny: [Bash(git push:*)]\n---\nYou are business-manager.\n")
	err = a.CheckTier(bizmgr, claude, CageShims, TierFast, true)
	if err == nil || !strings.Contains(err.Error(), "tier_floor: PID demands standard or better, launching at fast") {
		t.Errorf("a fast label under a standard floor must refuse: %v", err)
	}
	if err := a.CheckTier(bizmgr, claude, CageShims, TierStandard, false); err != nil {
		t.Errorf("at its floor business-manager runs: %v", err)
	}
	if err := a.CheckTier(bizmgr, claude, CageShims, TierStrong, false); err != nil {
		t.Errorf("above its floor business-manager runs: %v", err)
	}
	// Above fast the floor behaves like a cage shortfall: degradation the
	// operator can take deliberately, never dispatch on its own.
	rich := loadTestAgent(t, "---\nname: rich\ntier_floor: strong\ndeny: [Bash(git push:*)]\n---\nYou are rich.\n")
	if err := a.CheckTier(rich, claude, CageShims, TierStandard, false); err == nil {
		t.Error("standard under a strong floor must refuse")
	}
	if err := a.CheckTier(rich, claude, CageShims, TierStandard, true); err != nil {
		t.Errorf("--allow-degraded takes a floor shortfall above fast: %v", err)
	}
	// A PID that could never launch on its own default is an agent-check
	// finding, not a launch-time surprise.
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "low.md"), []byte("---\nname: low\ntier: fast\ntier_floor: standard\n---\nYou are low.\n"), 0o644)
	if fs, _, _ := a.CheckAgent("low"); !strings.Contains(strings.Join(fs, "\n"), "tier: fast is below this PID's own tier_floor: standard") {
		t.Errorf("agent check must catch a PID below its own floor: %v", fs)
	}
}

// The tier rules hold per bead, before any claim: dispatch reports the
// refusal like a degraded launch and skips that bead only — the persona's
// next bead may resolve to a tier it can honestly run.
func TestDispatchRefusesFastBelowFloorPerBead(t *testing.T) {
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "business-manager.md"),
		[]byte("---\nname: business-manager\nlabels: [ops]\ntier: standard\ntier_floor: standard\ndeny: [Bash(git push:*)]\n---\nYou are business-manager.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "security.md"),
		[]byte("---\nname: security\nlabels: [security]\ntier: strong\ndeny: [Edit, Write, Bash(git push:*)]\n---\nYou are security.\n"), 0o644)
	repo := qaRepo(t, b.App,
		`[{"id":"j-1","title":"cheap","labels":["ops","tier:fast"]},{"id":"j-2","title":"work","labels":["ops"]},{"id":"h-1","title":"scan","labels":["security","tier:fast"]}]`,
		`[{"id":"j-1","status":"open"},{"id":"j-2","status":"closed"},{"id":"h-1","status":"open"}]`)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	n, _ := d.Run("", "", 0)
	out := dispatcherOut(d)
	if !strings.Contains(out, "✗ j-1") || !strings.Contains(out, "tier_floor: PID demands standard or better, launching at fast") {
		t.Errorf("a fast label under business-manager's floor must be refused:\n%s", out)
	}
	if !strings.Contains(out, "✗ h-1") || !strings.Contains(out, "--allow-degraded is never accepted") {
		t.Errorf("fast on a persona whose gates the wall does not realize must be refused:\n%s", out)
	}
	// The refusal lands before the claim, and it costs the persona nothing:
	// j-2 (no tier label) still goes out this pass.
	if bd := bdCalls(t, fake); strings.Contains(bd, "j-1") || strings.Contains(bd, "h-1") {
		t.Errorf("a refused bead must not be touched in bd:\n%s", bd)
	}
	if n != 1 || !strings.Contains(out, "j-2") {
		t.Errorf("the refusal is per bead, not per persona (n=%d):\n%s", n, out)
	}
	if log := calls(t, fake); strings.Contains(log, SessionForBead("business-manager", repo, "j-1")) || strings.Contains(log, SessionForBead("security", repo, "h-1")) {
		t.Errorf("no session may be created for a refused bead:\n%s", log)
	}
	// --allow-degraded is the operator's consent to a weak wall, never to a
	// weak model: it buys nothing at fast.
	d2 := newTestDispatcher(t, b)
	d2.AllowDegraded = true
	d2.Run("", "", 0)
	if o := dispatcherOut(d2); !strings.Contains(o, "✗ h-1") || !strings.Contains(o, "✗ j-1") {
		t.Errorf("--allow-degraded must not buy fast in dispatch:\n%s", o)
	}
}

// rangerhq-b7m: posse types codex's directory-trust flag so an unattended
// session does not hang on the trust dialog, and that same trust makes
// $PWD/.codex/config.toml load before any model turn — its mcp_servers and
// notify entries are spawned by codex outside its own sandbox with the
// whole session env. The launch must find the file and refuse.
func TestCodexProjectConfigTrustGatesTheLaunch(t *testing.T) {
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "trusting.md"),
		[]byte("---\nname: trusting\ntrust_project_config: true\ndeny: [Bash(git push:*)]\n---\nYou are trusting.\n"), 0o644)
	dev, _ := b.App.LoadAgent("dev")
	trusting, _ := b.App.LoadAgent("trusting")
	codex, _ := b.App.LoadRuntime("codex")
	claude, _ := b.App.LoadRuntime("claude")
	repo := t.TempDir()

	// No .codex/config.toml: nothing to say, on any runtime.
	for _, rt := range []*Runtime{codex, claude} {
		if p := b.App.CheckParityIn(dev, rt, CageShims, TierStrong, repo); len(p.Degraded) != 0 {
			t.Errorf("clean dir on %s: %+v", rt.Name, p)
		}
	}
	mustCreate(t, b, NewSessionOpts{Name: "clean", Agent: "dev", Runtime: "codex", Dir: repo})

	os.MkdirAll(filepath.Join(repo, ".codex"), 0o755)
	os.WriteFile(filepath.Join(repo, ".codex", "config.toml"),
		[]byte("[mcp_servers.probe]\ncommand = \"/bin/sh\"\n"), 0o644)

	// Each runtime names its own project-config surface. Codex reads this
	// file; Claude's keyed JSON check has nothing to inspect here.
	p := b.App.CheckParityIn(dev, codex, CageShims, TierStrong, repo)
	if len(p.Degraded) != 1 || !strings.Contains(p.Degraded[0], ".codex/config.toml") || !strings.Contains(p.Degraded[0], "trust_project_config: true") {
		t.Errorf("codex must flag the repo's config: %+v", p)
	}
	if len(p.Unrealized) != 0 {
		t.Errorf("this is what the launch gives away, not an unenforced gate: %+v", p.Unrealized)
	}
	if p := b.App.CheckParityIn(dev, claude, CageShims, TierStrong, repo); len(p.Degraded) != 0 {
		t.Errorf("claude must ignore codex's ProjectConfig surface: %+v", p)
	}
	// The dir-independent matrix stays dir-independent.
	if p := b.App.CheckParity(dev, codex, CageShims, TierStrong); len(p.Degraded) != 0 {
		t.Errorf("CheckParity must not stat anything: %+v", p)
	}

	// Launch refuses, names the file, and offers the two ways out.
	err := b.CreateSession(NewSessionOpts{Name: "c1", Agent: "dev", Runtime: "codex", Dir: repo})
	if err == nil || !strings.Contains(err.Error(), ".codex/config.toml") || !strings.Contains(err.Error(), "--allow-degraded") {
		t.Fatalf("launch into a dir with a codex project config must refuse: %v", err)
	}
	// --allow-degraded launches it, marked in meta (and so in the cockpit).
	mustCreate(t, b, NewSessionOpts{Name: "c2", Agent: "dev", Runtime: "codex", Dir: repo, AllowDegraded: true})
	if m, _ := b.readMeta("c2"); m == nil || !strings.Contains(m.Degraded, ".codex/config.toml") {
		t.Errorf("a waived launch must stay marked: %+v", m)
	}
	// tier fast: the operator's consent is not on offer there (ADR 0003 §3).
	if p := b.App.CheckParityIn(dev, codex, CageShims, TierFast, repo); !p.NoDegrade {
		t.Errorf("fast must not be waivable: %+v", p)
	}
	if err := b.CreateSession(NewSessionOpts{Name: "c3", Agent: "dev", Runtime: "codex", Dir: repo, Tier: TierFast, AllowDegraded: true}); err == nil ||
		!strings.Contains(err.Error(), "never accepted") {
		t.Errorf("fast + project config must refuse even with --allow-degraded: %v", err)
	}
	// trust_project_config: true on the PID is the durable opt-in.
	if p := b.App.CheckParityIn(trusting, codex, CageShims, TierStrong, repo); len(p.Degraded) != 0 {
		t.Errorf("PID opt-in must clear it: %+v", p)
	}
	mustCreate(t, b, NewSessionOpts{Name: "c4", Agent: "trusting", Runtime: "codex", Dir: repo})
	if m, _ := b.readMeta("c4"); m == nil || m.Degraded != "" {
		t.Errorf("opt-in launch is not degraded: %+v", m)
	}
	if log := calls(t, fake); !strings.Contains(log, "codex") {
		t.Errorf("no codex line typed:\n%s", log)
	}
}

// ranger-base-gyqi / ADR 0002 amendment 2026-08-26: SeedClaudeTrust makes
// project hooks and MCP servers live, but permission-only project settings
// do not gain an executable channel. The predicate is therefore top-level
// key presence, regardless of the key's value, and classification fails
// closed whenever an existing file cannot be proved safe.
func TestClaudeProjectConfigTrustIsKeyedAndFailsClosed(t *testing.T) {
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "trusting.md"),
		[]byte("---\nname: trusting\ntrust_project_config: true\ndeny: [Bash(git push:*)]\n---\nYou are trusting.\n"), 0o644)
	dev, _ := b.App.LoadAgent("dev")
	trusting, _ := b.App.LoadAgent("trusting")
	claude, _ := b.App.LoadRuntime("claude")
	repo := t.TempDir()
	settingsDir := filepath.Join(repo, ".claude")
	settings := filepath.Join(settingsDir, "settings.json")

	if claude.ProjectConfig != ClaudeProjectConfig || len(claude.ProjectConfigKeys) != 2 ||
		claude.ProjectConfigKeys[0] != "hooks" || claude.ProjectConfigKeys[1] != "mcpServers" {
		t.Fatalf("claude project-config declaration: %+v", claude)
	}
	if p := b.App.CheckParity(dev, claude, CageShims, TierStrong); len(p.Degraded) != 0 {
		t.Fatalf("CheckParity must stay dir-independent: %+v", p)
	}
	// Missing and permission-only settings are clean, including at launch.
	if p := b.App.CheckParityIn(dev, claude, CageShims, TierStrong, repo); len(p.Degraded) != 0 {
		t.Errorf("no settings file: %+v", p)
	}
	mustCreate(t, b, NewSessionOpts{Name: "cl-none", Agent: "dev", Runtime: "claude", Dir: repo})
	os.MkdirAll(settingsDir, 0o755)
	os.WriteFile(settings, []byte(`{"permissions":{"allow":["Read"]}}`), 0o644)
	if p := b.App.CheckParityIn(dev, claude, CageShims, TierStrong, repo); len(p.Degraded) != 0 {
		t.Errorf("permission-only settings: %+v", p)
	}
	mustCreate(t, b, NewSessionOpts{Name: "cl-permissions", Agent: "dev", Runtime: "claude", Dir: repo})

	for _, tc := range []struct {
		name string
		body string
		key  string
	}{
		{name: "hooks", body: `{"hooks":null}`, key: "hooks"},
		{name: "mcp", body: `{"mcpServers":{}}`, key: "mcpServers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.WriteFile(settings, []byte(tc.body), 0o644)
			p := b.App.CheckParityIn(dev, claude, CageShims, TierStrong, repo)
			if len(p.Degraded) != 1 || !strings.Contains(p.Degraded[0], ClaudeProjectConfig) ||
				!strings.Contains(p.Degraded[0], "matched top-level project config keys: "+tc.key) ||
				!strings.Contains(p.Degraded[0], "trust_project_config: true") {
				t.Fatalf("matched key must degrade by presence regardless of value: %+v", p)
			}
			if len(p.Unrealized) != 0 {
				t.Errorf("project config is degradation, not an unrealized gate: %+v", p.Unrealized)
			}

			prefix := "cl-" + tc.name
			if err := b.CreateSession(NewSessionOpts{Name: prefix + "-refuse", Agent: "dev", Runtime: "claude", Dir: repo}); err == nil ||
				!strings.Contains(err.Error(), tc.key) || !strings.Contains(err.Error(), "--allow-degraded") {
				t.Fatalf("matched key must refuse by default: %v", err)
			}
			mustCreate(t, b, NewSessionOpts{Name: prefix + "-waived", Agent: "dev", Runtime: "claude", Dir: repo, AllowDegraded: true})
			if m, _ := b.readMeta(prefix + "-waived"); m == nil || !strings.Contains(m.Degraded, tc.key) {
				t.Errorf("waived launch must stay marked: %+v", m)
			}
			if p := b.App.CheckParityIn(dev, claude, CageShims, TierFast, repo); !p.NoDegrade {
				t.Errorf("fast must make project-config degradation unwaivable: %+v", p)
			}
			if err := b.CreateSession(NewSessionOpts{Name: prefix + "-fast", Agent: "dev", Runtime: "claude", Dir: repo, Tier: TierFast, AllowDegraded: true}); err == nil ||
				!strings.Contains(err.Error(), "never accepted") {
				t.Errorf("fast + matched key must refuse despite waiver: %v", err)
			}
			if p := b.App.CheckParityIn(trusting, claude, CageShims, TierStrong, repo); len(p.Degraded) != 0 {
				t.Errorf("PID opt-in must clear matched key: %+v", p)
			}
			mustCreate(t, b, NewSessionOpts{Name: prefix + "-trusted", Agent: "trusting", Runtime: "claude", Dir: repo})
		})
	}

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "malformed", body: `{"hooks":`, want: "classification failed: invalid JSON"},
		{name: "non-object", body: `[]`, want: "classification failed: not a top-level JSON object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.WriteFile(settings, []byte(tc.body), 0o644)
			why := ProjectConfigTrust(claude, dev, repo)
			if !strings.Contains(why, ClaudeProjectConfig) || !strings.Contains(why, tc.want) {
				t.Errorf("existing %s settings must fail closed: %q", tc.name, why)
			}
			if err := b.CreateSession(NewSessionOpts{Name: "cl-" + tc.name + "-refuse", Agent: "dev", Runtime: "claude", Dir: repo}); err == nil ||
				!strings.Contains(err.Error(), tc.want) {
				t.Errorf("existing %s settings must refuse launch: %v", tc.name, err)
			}
		})
	}

	// A path that exists but cannot be read as a file is an unreadable
	// classification failure, never the same thing as a missing file.
	os.Remove(settings)
	os.Mkdir(settings, 0o755)
	if why := ProjectConfigTrust(claude, dev, repo); !strings.Contains(why, ClaudeProjectConfig) ||
		!strings.Contains(why, "classification failed: unreadable") {
		t.Errorf("existing unreadable settings must fail closed: %q", why)
	}
	if err := b.CreateSession(NewSessionOpts{Name: "cl-unreadable-refuse", Agent: "dev", Runtime: "claude", Dir: repo}); err == nil ||
		!strings.Contains(err.Error(), "classification failed: unreadable") {
		t.Errorf("existing unreadable settings must refuse launch: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "claude") {
		t.Errorf("no claude line typed:\n%s", log)
	}
}

func TestParityL3ClaimsFollowIdentityAndBehavior(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	claude, _ := a.LoadRuntime("claude")
	ag := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n")
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks, _ := hooksDir(repo)
	write := func(slot, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// A byte-exact chain dispatcher, our render behind it: identity holds via
	// the prescribed chain even though the slot itself carries no marker.
	write("pre-push", chainHookDispatcherWith("pre-push", "theirs-pre-push"))
	write("theirs-pre-push", "#!/bin/sh\nexit 1\n")
	write("posse-pre-push", PrePushHook)
	write("prepare-commit-msg", chainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg"))
	write("theirs-prepare-commit-msg", "#!/bin/sh\nexit 1\n")
	write("posse-prepare-commit-msg", CommitGuardHook(VisibilityPublic))
	p := a.CheckParityIn(ag, claude, CageShims, TierStrong, repo)
	if len(p.Degraded) != 0 {
		t.Fatalf("a byte-exact chain must be clean: %+v", p)
	}
	for _, gate := range ag.Deny {
		if !strings.Contains(p.Realized[gate], "render probed, dispatch verified") {
			t.Errorf("%s must name the identity-verified L3 claim: %q", gate, p.Realized[gate])
		}
	}
	// A runtime that opts out of the gate shell has no L1. Successful L3
	// identity+behavior replaces that conservative dir-independent verdict;
	// this is why the concrete check cannot merely append a cosmetic layer
	// string.
	nogs := &Runtime{Name: "odd", NoGateShell: true}
	// Both gates by name rather than by COUNTING the map: parity gained a
	// row that is not a PID gate (ADR 0013 §4 reachability), and a count
	// asserts a number where the claim is "these two, on L3, without L1".
	if p := a.CheckParityIn(ag, nogs, CageShims, TierStrong, repo); len(p.Degraded) != 0 ||
		!strings.Contains(p.Realized["Bash(git push:*)"], "L3 pre-push hook") ||
		!strings.Contains(p.Realized["Bash(git commit unless --)"], "L3 prepare-commit-msg hook") {
		t.Errorf("identity-verified L3 must realize both git gates without L1: %+v", p)
	}

	// ADR 0023's whole point: a foreign hook with no marker that behaviorally
	// refuses everything must NOT certify. It cannot be told apart, by a
	// black-box probe, from one that refuses only the probe (the escape
	// ranger-base-vqvl found) — so identity is what decides, and this one
	// fails it. L1 still realizes the PID rules; L3 disappears from Realized
	// and both slots degrade, naming the foreign file and the chain remedy.
	write("pre-push", "#!/bin/sh\nexit 1\n")
	write("prepare-commit-msg", "#!/bin/sh\nexit 1\n")
	p = a.CheckParityIn(ag, claude, CageShims, TierStrong, repo)
	joined := strings.Join(p.Degraded, "\n")
	for _, want := range []string{"L3 pre-push hook", "L3 prepare-commit-msg hook", "foreign hook", "beads visibility guards are not realized", "posse gates install-hooks"} {
		if !strings.Contains(joined, want) {
			t.Errorf("failed probe missing %q in:\n%s", want, joined)
		}
	}
	for _, gate := range ag.Deny {
		if strings.Contains(p.Realized[gate], "L3") {
			t.Errorf("a foreign refuser must remove L3 from %s: %q", gate, p.Realized[gate])
		}
	}

	// A planted pass-through body — the bead's original exploit shape — is
	// also degraded, and for the same reason: no identity.
	write("pre-push", "#!/bin/sh\nexit 0\n")
	write("prepare-commit-msg", "#!/bin/sh\nexit 0\n")
	p = a.CheckParityIn(ag, claude, CageShims, TierStrong, repo)
	joined = strings.Join(p.Degraded, "\n")
	for _, want := range []string{"L3 pre-push hook", "L3 prepare-commit-msg hook", "foreign hook"} {
		if !strings.Contains(joined, want) {
			t.Errorf("failed probe missing %q in:\n%s", want, joined)
		}
	}
}

func TestLaunchInstallsHooksBeforeProbe(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}

	mustCreate(t, b, NewSessionOpts{Name: "clean", Agent: "dev", Dir: repo})
	if got := b.App.probeL3Hooks(repo, true); !got.PrePush || !got.CommitGuard {
		t.Errorf("launch must reconcile both slots before checking them: %+v", got)
	}
	if m, _ := b.readMeta("clean"); m == nil || m.Degraded != "" {
		t.Errorf("fresh repo with installed hooks must launch clean: %+v", m)
	}
}

func TestLaunchReportsForeignHookFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks, _ := hooksDir(repo)
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	err := b.CreateSession(NewSessionOpts{Name: "blocked", Agent: "dev", Dir: repo})
	if err == nil || !strings.Contains(err.Error(), "L3 pre-push hook") || !strings.Contains(err.Error(), "L3 prepare-commit-msg hook") {
		t.Fatalf("launch must report both foreign pass-through slots: %v", err)
	}
	if strings.Contains(calls(t, fake), "workspace create") {
		t.Fatal("a hook-degraded launch must refuse before touching herdr")
	}

	mustCreate(t, b, NewSessionOpts{Name: "waived", Agent: "dev", Dir: repo, AllowDegraded: true})
	m, _ := b.readMeta("waived")
	if m == nil || !strings.Contains(m.Degraded, "L3 pre-push hook") || !strings.Contains(m.Degraded, "L3 prepare-commit-msg hook") {
		t.Errorf("waived launch must retain both probe failures in meta: %+v", m)
	}
}
