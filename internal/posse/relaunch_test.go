package posse

// posse relaunch (rangerhq-dxq): land the plane, kill, recreate from the same
// meta. The fake herdr's levers do the work — pane-run-starts-agent makes a
// created session look inhabited, wait-status decides whether the landing
// turn settles.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// devSession writes a PID whose gates the wall fully realizes on claude at
// shims, plus an env set, and creates one session from them.
func devSession(t *testing.T, b *HerdrBackend, name string) string {
	t.Helper()
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	os.MkdirAll(b.App.EnvsDir, 0o700)
	os.WriteFile(filepath.Join(b.App.EnvsDir, "test.env"), []byte("FOO=bar\n"), 0o600)
	repo := t.TempDir()
	if err := b.CreateSession(NewSessionOpts{
		Name: name, Agent: "dev", Dir: repo, Envs: []string{"test"}, Tier: "standard",
	}); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRelaunchLandsKillsAndRecreates(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := devSession(t, b, "s1")
	m1, _ := b.readMeta("s1")

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	for _, want := range []string{"landing s1", "killed s1", "ready: posse attach s1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("relaunch output missing %q:\n%s", want, out.String())
		}
	}

	log := calls(t, fake)
	// The landing turn: settle first (herdr's prompt does not track turns),
	// then the prompt, then the close — in that order.
	iWait := strings.Index(log, "agent wait")
	iPrompt := strings.Index(log, "agent prompt")
	iClose := strings.Index(log, "workspace close "+m1.Workspace)
	if iWait < 0 || iPrompt < iWait || iClose < iPrompt {
		t.Errorf("expected wait → prompt → close, got %d/%d/%d:\n%s", iWait, iPrompt, iClose, log)
	}
	for _, want := range []string{"Land the plane", "ORDERS.md", "Push only what your own guardrails permit", "whatever handed it over: repo docs, `bd prime`'s session-start checklist, this prompt"} {
		if !strings.Contains(log, want) {
			t.Errorf("landing prompt missing %q:\n%s", want, log)
		}
	}
	if n := strings.Count(log, "workspace create --label s1"); n != 2 {
		t.Errorf("expected the session recreated once, got %d creates:\n%s", n, log)
	}
	// The recreated workspace carries the same env — the persona's identity
	// and the session's env set both ride the launch, not the meta alone.
	if n := strings.Count(log, "--env FOO=bar"); n != 2 {
		t.Errorf("env set must ride the recreate:\n%s", log)
	}
	if n := strings.Count(log, "--env BD_ACTOR=dev"); n != 2 {
		t.Errorf("persona identity must ride the recreate:\n%s", log)
	}

	m2, ok := b.readMeta("s1")
	if !ok {
		t.Fatal("no meta after relaunch")
	}
	if m2.Workspace == m1.Workspace {
		t.Errorf("relaunch must be a new workspace, still %s", m2.Workspace)
	}
	if m2.Agent != "dev" || m2.Dir != repo || m2.Envs != "test" || m2.Tier != "standard" || m2.Cage != m1.Cage || m2.Emoji != m1.Emoji {
		t.Errorf("recreate lost part of the session: %+v (was %+v)", m2, m1)
	}
}

// A session still mid-turn is not killed: the operator is told to wait, and
// nothing is closed. Refreshing a session must never cost work in flight.
func TestRelaunchRefusesWhileWorking(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	m1, _ := b.readMeta("s1")
	os.WriteFile(filepath.Join(fake, "wait-status"), []byte("working"), 0o644)

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil || !strings.Contains(err.Error(), "still working") {
		t.Fatalf("a working session must not be relaunched: %v", err)
	}
	if strings.Contains(calls(t, fake), "workspace close") {
		t.Error("refused relaunch must not close the workspace")
	}
	if m2, ok := b.readMeta("s1"); !ok || m2.Workspace != m1.Workspace {
		t.Errorf("refused relaunch must leave the session alone: %+v", m2)
	}

	// --no-land is the operator's override: no landing turn, straight to the
	// refresh, even with the agent mid-turn.
	out.Reset()
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1", NoLand: true}); err != nil {
		t.Fatalf("--no-land: %v\n%s", err, out.String())
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("--no-land must not prompt:\n%s", calls(t, fake))
	}
	if m2, _ := b.readMeta("s1"); m2.Workspace == m1.Workspace {
		t.Error("--no-land must still recreate the session")
	}
}

// A blocked agent cannot take a prompt, and a session with no agent has
// nothing to land — both refresh anyway.
func TestRelaunchSkipsLandingWhenNothingCanLand(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	os.WriteFile(filepath.Join(fake, "wait-status"), []byte("blocked"), 0o644)
	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("blocked relaunch: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "blocked awaiting input") || strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("a blocked agent must be skipped, not prompted:\n%s\n%s", out.String(), calls(t, fake))
	}

	// Now the crashed case: the workspace is alive, its CLI is gone.
	os.Remove(filepath.Join(fake, "pane-run-starts-agent"))
	os.Remove(filepath.Join(fake, "agents.json"))
	out.Reset()
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("dead-agent relaunch: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "nothing to land") || !strings.Contains(out.String(), "ready: posse attach s1") {
		t.Errorf("a dead agent must skip landing and still refresh:\n%s", out.String())
	}
}

// A plain session's --cmd exists nowhere but the meta, so the meta records
// it — otherwise a relaunch would hand back an empty shell.
func TestRelaunchReplaysPlainCommand(t *testing.T) {
	b, fake := newTestBackend(t)
	repo := t.TempDir()
	if err := b.CreateSession(NewSessionOpts{Name: "p1", Dir: repo, Cmd: "run-me --flag"}); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.readMeta("p1"); m.Cmd != "run-me --flag" {
		t.Fatalf("plain --cmd not recorded: %+v", m)
	}
	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "p1"}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	if n := strings.Count(calls(t, fake), "run-me --flag"); n != 2 {
		t.Errorf("the command must be typed again on recreate:\n%s", calls(t, fake))
	}
}

// Degradation the operator already consented to survives the refresh —
// relaunching a session is not a new decision — and a session posse did not
// create has no recipe to recreate from.
func TestRelaunchCarriesDegradedConsentAndRefusesForeign(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "security.md"),
		[]byte("---\nname: security\ndeny: [Edit, Write, Bash(git push:*)]\n---\nYou are security.\n"), 0o644)
	var warn strings.Builder
	b.Warn = &warn
	if err := b.CreateSession(NewSessionOpts{Name: "h1", Agent: "security", Dir: t.TempDir(), AllowDegraded: true}); err != nil {
		t.Fatal(err)
	}
	m1, _ := b.readMeta("h1")
	if m1.Degraded == "" {
		t.Fatal("expected a degraded session to test with")
	}
	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "h1"}); err != nil {
		t.Fatalf("a degraded session must relaunch as it was: %v", err)
	}
	m2, _ := b.readMeta("h1")
	if m2.Degraded != m1.Degraded {
		t.Errorf("degraded mark lost: %q → %q", m1.Degraded, m2.Degraded)
	}

	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "nope"}); err == nil || !strings.Contains(err.Error(), "no session meta") {
		t.Errorf("relaunching a session posse never created must refuse: %v", err)
	}
	_ = fake
}

// ─── kill-before-recreate (rangerhq-v52t) ────────────────────────────────────

// The bug the operator lost a session to: relaunch killed first and asked
// whether the replacement was buildable second, so every refusal CreateSession
// can raise for a reason knowable in advance — a missing dir, a persona that
// no longer loads — arrived with the original already destroyed. Preflight
// moves those refusals in front of the kill, where they cost nothing.
func TestRelaunchRefusesBeforeTheKillWhenTheRecreateCannotSucceed(t *testing.T) {
	check := func(t *testing.T, breakIt func(b *HerdrBackend, repo string), want string) {
		t.Helper()
		b, fake := newTestBackend(t)
		agentPerLaunch(t, fake)
		repo := devSession(t, b, "s1")
		m1, _ := b.readMeta("s1")
		breakIt(b, repo)

		var out strings.Builder
		err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
		if err == nil {
			t.Fatalf("relaunch must refuse a recreate that cannot succeed:\n%s", out.String())
		}
		if !strings.Contains(err.Error(), "NOT closed") || !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name the cause and say the session survived: %v", err)
		}
		// The whole point: nothing was destroyed, so nothing must be recovered.
		if log := calls(t, fake); strings.Contains(log, "workspace close") {
			t.Errorf("a refused relaunch must not close the workspace:\n%s", log)
		}
		if !b.HasSession("s1") {
			t.Error("the session the operator asked to refresh is gone")
		}
		if m2, ok := b.readMeta("s1"); !ok || m2.Workspace != m1.Workspace {
			t.Errorf("refused relaunch must leave the meta alone: %+v (was %+v)", m2, m1)
		}
		// Landing costs up to ten minutes; a relaunch that cannot finish
		// must not spend them first.
		if strings.Contains(calls(t, fake), "agent prompt") {
			t.Errorf("a refused relaunch must not run the landing turn:\n%s", calls(t, fake))
		}
	}

	t.Run("dir gone", func(t *testing.T) {
		check(t, func(b *HerdrBackend, repo string) { os.RemoveAll(repo) }, "directory not found")
	})
	// "profile resolution erroring" from the bead: the PID the session runs
	// is edited or removed between launch and refresh.
	t.Run("persona gone", func(t *testing.T) {
		check(t, func(b *HerdrBackend, repo string) {
			os.Remove(filepath.Join(b.App.AgentsDir, "dev.md"))
		}, "dev")
	})
}

// The failure no ordering can prevent: the workspace create is on the far
// side of the kill by construction. It must cost a restart, never the
// session's identity — the recipe is written back, the error carries both
// ways out, and `posse relaunch` is itself the retry.
func TestRelaunchKeepsTheRecipeWhenTheRecreateFailsAfterTheKill(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := devSession(t, b, "s1")
	os.WriteFile(filepath.Join(fake, "create-error"), []byte("workspace_create_failed|no space"), 0o644)

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("a failed recreate must be an error:\n%s", out.String())
	}
	// Everything the operator needs is in the error, not in their scrollback.
	for _, want := range []string{
		"could not be recreated", "no space",
		"posse relaunch s1",
		"posse new s1 --agent dev --dir " + repo + " --env-file test --runtime claude --tier standard",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the failure must say %q:\n%v", want, err)
		}
	}
	// The recipe survives the failure, naming no workspace — nothing to
	// orphan, nothing to infer dead and prune.
	m, ok := b.readMeta("s1")
	if !ok {
		t.Fatal("the session's recipe died with the failed recreate — that is the bug")
	}
	if m.Workspace != "" || m.Pane != "" {
		t.Errorf("a kept recipe must not name the workspace the kill destroyed: %+v", m)
	}
	if m.Agent != "dev" || m.Dir != repo || m.Envs != "test" || m.Tier != "standard" {
		t.Errorf("the kept recipe lost part of the session: %+v", m)
	}

	// A listing says what that state is, rather than explaining it as a
	// session that might still be alive somewhere.
	var warn strings.Builder
	b.Warn = &warn
	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "recipe kept: s1") {
		t.Errorf("posse list must report the kept recipe: %q", warn.String())
	}
	// Left as this buffer, not reset to nil: nil is the test binary's own
	// stderr (ranger-base-ihd2), so the retry below would print its warnings
	// under whichever other test in the package fails.

	// And the advertised retry actually rebuilds the session.
	os.Remove(filepath.Join(fake, "create-error"))
	out.Reset()
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("the retry the error advertises must work: %v\n%s", err, out.String())
	}
	m2, _ := b.readMeta("s1")
	if m2.Workspace == "" || m2.Agent != "dev" || m2.Dir != repo || m2.Envs != "test" {
		t.Errorf("the retry did not rebuild the session from its recipe: %+v", m2)
	}
	if !b.HasSession("s1") {
		t.Error("no session after the retry")
	}
}

// Preflight prints what it resolved before anything is destroyed: a relaunch
// that goes wrong later leaves a scrollback saying what it was building.
func TestRelaunchPrintsWhatItCheckedBeforeKilling(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := devSession(t, b, "s1")
	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{"checked s1: dev on claude @ standard", "cage shims", "dir " + repo, "env test"} {
		if !strings.Contains(got, want) {
			t.Errorf("preflight receipt missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "checked s1") > strings.Index(got, "killed s1") {
		t.Errorf("the check must be printed before the kill:\n%s", got)
	}
	_ = fake
}

// The other side of the rollback: when a replacement workspace DID come up
// and only its start-up failed, the recipe must not be written back —
// blanking the meta of a live workspace orphans it, which is the cpeh harm
// self-inflicted by the cleanup. Name the workspace and leave the record.
func TestRelaunchDoesNotOrphanAReplacementThatCameUp(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	m1, _ := b.readMeta("s1")
	os.WriteFile(filepath.Join(fake, "pane-run-error"), []byte("pane_run_failed|shell is gone"), 0o644)

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("a recreate whose command never ran must be an error:\n%s", out.String())
	}
	m2, ok := b.readMeta("s1")
	if !ok {
		t.Fatal("no meta after a partial recreate")
	}
	if m2.Workspace == "" {
		t.Fatalf("the meta was blanked while workspace %s is up — nothing on disk names it now", m1.Workspace)
	}
	if m2.Workspace == m1.Workspace {
		t.Errorf("the replacement is a new workspace, meta still names the closed %s", m1.Workspace)
	}
	if !strings.Contains(err.Error(), m2.Workspace) || !strings.Contains(err.Error(), "did not finish starting") {
		t.Errorf("the failure must name the workspace that came up: %v", err)
	}
	if !strings.Contains(err.Error(), "posse relaunch s1") {
		t.Errorf("the failure must say how to retry: %v", err)
	}
	// And the retry is a real one: the meta names a live workspace, so the
	// next relaunch closes it and builds another.
	os.Remove(filepath.Join(fake, "pane-run-error"))
	out.Reset()
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("retry after a partial recreate: %v\n%s", err, out.String())
	}
	if !b.HasSession("s1") {
		t.Error("no session after the retry")
	}
}

// rangerhq-i2g9: relaunch's other destructive step. HasSession answers out
// of Sessions(), where a meta whose workspace is missing from the listing
// snapshot is spared but left out of the listing — the rangerhq-9nso
// condition itself, alive on the server and absent from this read. Relaunch
// used to unlink that meta as "workspace already gone", and the recreate
// then walked past mustNotOrphan, which had nothing left to read: a second
// workspace under the same label, the first still running its agent with
// nothing on disk naming it, err == nil.
//
// So the unlink proves death the way every other delete in this file's
// chain does (ADR 0011 §2), through the same predicate the create asks —
// and a workspace that answers alive is refused rather than closed, because
// this pass cannot land an agent it cannot address.
func TestRelaunchProvesDeathBeforeClearingAMeta(t *testing.T) {
	const sock = "/tmp/i2g9/herdr.sock"

	// The incident's board: another session keeps the listing non-empty (so
	// the rangerhq-8fq guards stay quiet and this is about the snapshot, not
	// the socket), plus the session under test. Returns its workspace.
	setup := func(t *testing.T) (*HerdrBackend, string, string) {
		t.Helper()
		t.Setenv("HERDR_SOCKET_PATH", sock)
		b, fake := newTestBackend(t)
		agentPerLaunch(t, fake)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
		m, ok := b.readMeta("s1")
		if !ok {
			t.Fatal("no meta for s1")
		}
		os.Remove(filepath.Join(fake, "calls.log"))
		return b, fake, m.Workspace
	}
	hideFromList := func(t *testing.T, fake, id string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fake, "hidden-from-list"), []byte(id), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The incident, by relaunch: alive on the server, absent from this
	// pass's listing. Nothing may be destroyed and nothing may be created.
	t.Run("workspace alive but missing from the listing", func(t *testing.T) {
		b, fake, ws := setup(t)
		hideFromList(t, fake, ws)
		if b.HasSession("s1") {
			t.Fatal("setup: the session was supposed to be invisible to this pass")
		}

		var out strings.Builder
		err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
		if err == nil {
			t.Fatalf("relaunched over a live session behind a stale listing (rangerhq-i2g9):\n%s", out.String())
		}
		m, ok := b.readMeta("s1")
		if !ok {
			t.Fatalf("the meta of a live session was unlinked — nothing on disk names %s now", ws)
		}
		if m.Workspace != ws {
			t.Fatalf("a second workspace was created over a live session: the meta names %s, %s is still running", m.Workspace, ws)
		}
		if alive, aerr := b.H.WorkspaceAlive(ws); aerr != nil || !alive {
			t.Errorf("the live session was closed without ever being landed (alive=%v err=%v)", alive, aerr)
		}
		log := calls(t, fake)
		if strings.Contains(log, "workspace create") {
			t.Errorf("a replacement workspace was created before the refusal:\n%s", log)
		}
		if strings.Contains(log, "workspace close") {
			t.Errorf("a session this pass could not land was closed anyway:\n%s", log)
		}
	})

	// The refusal is what the operator has to act on: which session, which
	// workspace, and that nothing was destroyed.
	t.Run("the refusal says nothing was closed", func(t *testing.T) {
		b, fake, ws := setup(t)
		hideFromList(t, fake, ws)

		var out strings.Builder
		err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
		if err == nil {
			t.Fatal("expected a refusal")
		}
		for _, want := range []string{"s1", ws, "NOT closed"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal missing %q: %v", want, err)
			}
		}
	})

	// The ordinary reason a session is missing from the listing: its
	// workspace really did die. Relaunch still rebuilds it, and the meta is
	// only cleared once herdr has said so by id.
	t.Run("workspace genuinely gone", func(t *testing.T) {
		b, fake, ws := setup(t)
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}}) // s1's workspace closed

		var out strings.Builder
		if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
			t.Fatalf("a session whose workspace this server confirms is gone must be rebuildable: %v\n%s", err, out.String())
		}
		m, ok := b.readMeta("s1")
		if !ok || m.Workspace == "" || m.Workspace == ws {
			t.Fatalf("the rebuilt session was not recorded: %+v", m)
		}
		if log := calls(t, fake); !strings.Contains(log, "workspace get "+ws) {
			t.Errorf("the meta was cleared without asking herdr about its workspace:\n%s", log)
		}
	})

	// Silence is not death on this side either, and here the unrecoverable
	// direction is the unlink.
	t.Run("herdr does not answer the query", func(t *testing.T) {
		b, fake, ws := setup(t)
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})
		if err := os.WriteFile(filepath.Join(fake, "workspace-get-unreachable"), nil, 0o644); err != nil {
			t.Fatal(err)
		}

		var out strings.Builder
		if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err == nil {
			t.Fatalf("cleared a meta on a query that errored — silence is not evidence of death:\n%s", out.String())
		}
		if m, ok := b.readMeta("s1"); !ok || m.Workspace != ws {
			t.Fatalf("meta lost behind an unanswered query: %+v", m)
		}
	})

	// The recipe a failed recreate leaves behind (rangerhq-v52t) names no
	// workspace: it can orphan nothing, so it is cleared without asking, and
	// `posse relaunch` stays its own retry.
	t.Run("a recipe naming no workspace is cleared without asking", func(t *testing.T) {
		b, fake, _ := setup(t)
		m, _ := b.readMeta("s1")
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})
		b.keepRecipe(m, nil)
		os.Remove(filepath.Join(fake, "calls.log"))

		var out strings.Builder
		if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
			t.Fatalf("a kept recipe must rebuild: %v\n%s", err, out.String())
		}
		if log := calls(t, fake); strings.Contains(log, "workspace get") {
			t.Errorf("per-id query about a meta that names no workspace:\n%s", log)
		}
	})

	// The common path pays nothing: a session herdr lists is killed by name,
	// and no proof is asked for.
	t.Run("a listed session is killed, not proven dead", func(t *testing.T) {
		b, fake, ws := setup(t)

		var out strings.Builder
		if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
			t.Fatalf("relaunch: %v\n%s", err, out.String())
		}
		log := calls(t, fake)
		if !strings.Contains(log, "workspace close "+ws) {
			t.Errorf("the listed session was not closed:\n%s", log)
		}
		if strings.Contains(log, "workspace get") {
			t.Errorf("per-id query on the ordinary path:\n%s", log)
		}
	})
}

// rangerhq-9jk1, the preflight half. A herdr workspace merely WEARING the
// session's name is an obstacle to the recreate that lives in herdr rather
// than in the plan: nameFree will refuse the name on the far side of the
// kill, where the refusal costs the session. It is knowable before anything
// is destroyed, so it is refused there (rangerhq-v52t's rule), and the
// refusal names the workspace in the way — the orphan guard's own message
// would send the operator to `posse attach s1`, which resolves to that very
// stranger.
func TestRelaunchRefusesAWorkspaceAlreadyWearingTheName(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})
	mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
	m1 := metaOf(t, b, "s1")
	// The orphan of an earlier incident, a second RHQ_HOME on one herdr
	// (rangerhq-snd), or a label an operator typed: posse has no meta for it.
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: "wForeign", Label: "s1"}))
	os.Remove(filepath.Join(fake, "calls.log"))

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("relaunch walked into a name the recreate could never take:\n%s", out.String())
	}
	for _, want := range []string{"s1", "wForeign", "NOT closed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name the workspace in the way and say nothing was closed (%q): %v", want, err)
		}
	}
	log := calls(t, fake)
	for _, forbidden := range []string{"workspace close", "workspace create", "agent prompt"} {
		if strings.Contains(log, forbidden) {
			t.Errorf("a relaunch refused before the kill must not %q:\n%s", forbidden, log)
		}
	}
	if m2 := metaOf(t, b, "s1"); m2.Workspace != m1.Workspace || m2.Pane != m1.Pane {
		t.Errorf("refused relaunch must leave the record alone: %+v (was %+v)", m2, m1)
	}
	if alive, aerr := b.H.WorkspaceAlive(m1.Workspace); aerr != nil || !alive {
		t.Errorf("the session the operator asked to refresh was closed (alive=%v err=%v)", alive, aerr)
	}

	// And it is a refusal, not a wall: once the stranger is out of the way
	// the same command rebuilds the session.
	var kept []fakeWS
	for _, ws := range fakeLoadWSFrom(t, fake) {
		if ws.WorkspaceID != "wForeign" {
			kept = append(kept, ws)
		}
	}
	saveWSTo(t, fake, kept)
	out.Reset()
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("relaunch after the label was freed: %v\n%s", err, out.String())
	}
	if m2 := metaOf(t, b, "s1"); m2.Workspace == "" || m2.Workspace == m1.Workspace {
		t.Errorf("the session was not rebuilt: %+v", m2)
	}
}

// rangerhq-9jk1, the fourth site. keepRecipe is a meta WRITE, and a write
// over a meta is as unrecoverable as the delete (rangerhq-cpeh): state/ is
// outside git, so blanking workspace: destroys the only thing that names a
// live session. It used to assume its caller had killed the workspace; on
// the 9jk1 board it ran on the way OUT of mustNotOrphan's own refusal and
// erased the record that refusal had just declined to overwrite. So it asks
// the same guard the other three sites ask.
func TestKeepRecipeWillNotBlankARecordItCannotProveDead(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	mustCreate(t, b, NewSessionOpts{Name: "mine"}) // a non-empty listing (rangerhq-8fq)
	mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
	m := metaOf(t, b, "s1")

	if kept := b.keepRecipe(m, nil); kept != m.Workspace {
		t.Errorf("the rollback did not report the live workspace it must not blank: %q, want %s", kept, m.Workspace)
	}
	if m2 := metaOf(t, b, "s1"); m2.Workspace != m.Workspace || m2.Pane != m.Pane {
		t.Fatalf("the only record of live workspace %s was blanked by the rollback: %+v", m.Workspace, m2)
	}
	if !strings.Contains(calls(t, fake), "workspace get "+m.Workspace) {
		t.Errorf("the record was kept without asking herdr about its workspace:\n%s", calls(t, fake))
	}

	// The control, and the whole point of keepRecipe: once the workspace
	// really is gone there is nothing to orphan, so the recipe is written
	// back naming none and `posse relaunch s1` is its own retry.
	if err := b.KillSession("s1"); err != nil {
		t.Fatal(err)
	}
	if kept := b.keepRecipe(m, nil); kept != "" {
		t.Fatalf("a rollback after a real kill must write the recipe, kept %q", kept)
	}
	m2 := metaOf(t, b, "s1")
	if m2.Workspace != "" || m2.Pane != "" {
		t.Errorf("a kept recipe must not name the workspace the kill destroyed: %+v", m2)
	}
	if m2.Cmd != m.Cmd || m2.Emoji != m.Emoji {
		t.Errorf("the kept recipe lost part of the session: %+v (was %+v)", m2, m)
	}
}
