//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-ayubz — AT THE CONTAINER TIER THE FORWARDED
// BEADS_DIR IS SPELLED IN THE MOUNT SET'S NAMESPACE, AND IT IS A
// TRANSLATION AND NOT A RESOLVE.
//
// WHAT WAS WRONG. beadsHome answers bd's own spelling and not a resolved
// path, deliberately (beadloss.go's contract, pinned by
// TestQABeadsHomeAnswersBdsOwnSpellingAndNotAResolvedPath, whose own closing
// paragraph filed this). Two consumers read it for this tier and they
// disagreed: the MOUNT resolved (cageOverlay's `src := absResolve(p)`) and
// the NAME did not (planLaunch's env, forwarded by CageEnvNames). So where
// the store is reached through a symlink — a `.beads/redirect` naming a path
// with a link component, or any tree under a linked prefix, which is every
// darwin box under `/var` — the caged bd was handed a path the container
// does not have, while ADR 0013 §4's reachability row, which resolves, read
// the row Enforced. The inner bd is always `--no-db --no-daemon`
// (CageBdFlags) and a no-db bd resolves `$BEADS_DIR` else `$cwd/.beads` and
// reads no redirect at all (ADR 0055 D1), so that claim/comment/close either
// failed outright or landed in the session worktree's own issues.jsonl and
// died with the container.
//
// WHY ONE absResolve AT THE ENV SITE IS THE WRONG FIX, and arm 2 is the
// measurement rather than the argument. At L4 the filesystem IS the mount
// set: `CageMount.Dst` is what the container has, and the repo bind is
// `{Src: dir, Dst: dir}` with dir exactly as the launch spells it. So where
// DIR is the thing behind the link — a home under darwin's `/home` ->
// `/System/Volumes/Data/home`, every `t.TempDir()` under `/var` ->
// `/private/var` — the store crosses at the RAW spelling, and resolving the
// name would break the case that works today. The answer is cageCovering's,
// the resolver CageMounts renders its own overlays by: the deepest bind
// covering the store, spelled the way THAT bind spells it (CageBeadsDir).
//
// CONTAINMENT IS COMPARED AS STRINGS HERE (cbdCoversRaw), and that is the
// whole subject. mountAt and cageCovering both absResolve, which is the
// right question about the host and the wrong one about the container: a
// host link is not in the mount set, so a path that only resolves through
// one does not exist inside. A pin that compared the way they do would be
// green over the defect it was written for.
//
// WHAT THESE DO NOT MEASURE: no container runs. The engine is cageApp's
// hermetic fake and the assertions are over the rendered mount set and the
// assembled launch env. The live half — a caged bd writing the store behind
// a link — needs an engine the seat can reach, and this box's posse user is
// not in the `docker` group (MEASURED 2026-11-22: `permission denied while
// trying to connect to the docker API at unix:///var/run/docker.sock`, uid
// 1001(posse) groups=1001(posse) against `srw-rw---- root docker`). The
// namespace fact it would show is in docs/notes.d/ranger-base-ayubz.md,
// measured under bwrap on this box.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cbdCoversRaw is containment as the CONTAINER reads it: the deepest bind
// whose DESTINATION holds p, compared as strings, because the engine sorts
// binds by destination depth and inside the cage there is no host symlink to
// resolve a path through.
func cbdCoversRaw(ms []CageMount, p string) (CageMount, bool) {
	best, found := CageMount{}, false
	for _, m := range ms {
		if m.Dst != p && !strings.HasPrefix(p, m.Dst+string(filepath.Separator)) {
			continue
		}
		if !found || len(m.Dst) > len(best.Dst) {
			best, found = m, true
		}
	}
	return best, found
}

// cbdMkdir makes a fixture directory and fails loudly, because every arm
// below is about which spelling of a directory is which and a missing one
// would read as the wrong answer.
func cbdMkdir(t *testing.T, p string) string {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return p
}

// cbdRepo is a directory shaped like a checkout posse would launch into:
// `.beads` and `.git`, so CageMounts reaches the branches that mount them.
func cbdRepo(t *testing.T, p string) string {
	t.Helper()
	cbdMkdir(t, filepath.Join(p, beadsDirName))
	cbdMkdir(t, filepath.Join(p, ".git"))
	return p
}

// cbdCage is the hermetic fake engine plus a `cage: container` PID.
func cbdCage(t *testing.T) (*App, *AgentFile, *Engine) {
	t.Helper()
	a := cageApp(t)
	ag := cageAgent(t, a, "cage: container\n")
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		t.Fatalf("fixture engine: %v", err)
	}
	return a, ag, e
}

// 1. THE BEAD'S OWN SHAPE: a `.beads/redirect` into a store behind a
// symlink. The forwarded name must be a path some bind in the rendered set
// covers read-write, as that bind spells it — and the raw spelling, which is
// what shipped, must be covered by nothing, or this arm holds for the wrong
// reason.
func TestQACagedBeadsDirIsAPathTheMountSetCoversForAStoreBehindALink(t *testing.T) {
	t.Parallel()
	a, ag, e := cbdCage(t)

	root := absResolve(t.TempDir())
	repo := cbdRepo(t, filepath.Join(root, "repo"))
	store := cbdRepo(t, filepath.Join(root, "real", "store"))
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	spelt := filepath.Join(root, "link", "store", beadsDirName)
	if err := os.WriteFile(filepath.Join(repo, beadsDirName, beadsRedirect), []byte(spelt+"\n"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	// THE FIXTURE IS THE SHAPE THE ARM NEEDS, asserted and not assumed, in
	// the two ways it could fail to be: bd follows the redirect to the
	// link's spelling, and that spelling is NOT the resolved one — without
	// the second, the translation below is a no-op and this passes over the
	// defect.
	home := beadsHome(repo)
	if home != spelt {
		t.Fatalf("fixture: the redirect does not reach the store as spelled: beadsHome = %q, want %q", home, spelt)
	}
	resolved := filepath.Join(store, beadsDirName)
	if absResolve(spelt) != resolved {
		t.Fatalf("fixture: %s resolves to %s, not to the store at %s", spelt, absResolve(spelt), resolved)
	}

	ms := a.CageMounts(ag, e, repo, "s1")
	got := a.CageBeadsDir(ag, repo, "s1", home)
	m, ok := cbdCoversRaw(ms, got)
	if !ok {
		t.Fatalf("the forwarded BEADS_DIR (%s) is in no bind the mount set names — inside the cage that path does not exist, and a `--no-db` bd told it is the store writes into the container and nothing else (ADR 0055 D1):\n%s", got, showMounts(ms))
	}
	if m.RO {
		t.Errorf("the forwarded BEADS_DIR (%s) is covered by a READ-ONLY bind (%s) — a caged session cannot `bd comments add` or `bd close` there:\n%s", got, m.Dst, showMounts(ms))
	}
	// THE PRE-FIX VALUE, and the control that makes the arm above worth its
	// length: the name planLaunch used to forward is covered by no bind at
	// all. The row that judged this launch resolved, so it read Enforced
	// over exactly this.
	if _, ok := cbdCoversRaw(ms, home); ok {
		t.Errorf("the store AS SPELLED (%s) is covered by a bind, so this fixture does not discriminate — the defect was that this spelling is not in the mount set:\n%s", home, showMounts(ms))
	}
	if got != resolved {
		t.Errorf("the forwarded BEADS_DIR is %s; under a redirect out of the tree the covering bind is the carve-out at the resolved store, so it must be %s", got, resolved)
	}
}

// 2. THE CASE A BLANKET absResolve WOULD HAVE BROKEN, which is why the fix
// asks the mount set instead: no redirect, and the SESSION DIR itself behind
// a symlink. The repo bind is `{Src: dir, Dst: dir}` as the launch spells
// it, so the store crosses at the raw spelling and the raw spelling is the
// only one the container has.
func TestQACagedBeadsDirKeepsTheRawSpellingWhenTheMountIsSpelledThatWay(t *testing.T) {
	t.Parallel()
	a, ag, e := cbdCage(t)

	root := absResolve(t.TempDir())
	hard := cbdRepo(t, filepath.Join(root, "real", "repo"))
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	dir := filepath.Join(root, "link", "repo")
	if absResolve(dir) != hard {
		t.Fatalf("fixture: %s does not resolve to %s", dir, hard)
	}

	home := beadsHome(dir)
	if want := filepath.Join(dir, beadsDirName); home != want {
		t.Fatalf("fixture: beadsHome = %q, want the local shape %q", home, want)
	}
	ms := a.CageMounts(ag, e, dir, "s1")
	got := a.CageBeadsDir(ag, dir, "s1", home)
	if got != home {
		t.Errorf("the forwarded BEADS_DIR is %s — the repo bind's destination is the launch's own spelling of dir, so the store crosses at %s and nothing resolves that path inside the cage", got, home)
	}
	if _, ok := cbdCoversRaw(ms, got); !ok {
		t.Fatalf("the forwarded BEADS_DIR (%s) is in no bind the mount set names:\n%s", got, showMounts(ms))
	}
	// THE MEASUREMENT: the resolved path — what one absResolve at the env
	// site would have forwarded, which is what ranger-base-ayubz expected
	// the fix to be — is covered by no bind, so that fix would have moved
	// the defect from the redirect shape onto every linked prefix instead.
	if m, ok := cbdCoversRaw(ms, absResolve(home)); ok {
		t.Errorf("the RESOLVED store (%s) is covered by bind %s, so this fixture does not discriminate between resolving the name and translating it:\n%s", absResolve(home), m.Dst, showMounts(ms))
	}
}

// 3. THE GATE AND THE SESSION READ ONE PATH. The reachability row judges the
// MOUNT (containerReachRow, which resolves) and the session uses the NAME,
// and that pair disagreeing is what made the row a false pass of ADR 0013
// §4 — the ranger-base-0fb/k62e class. Over the same store behind a link:
// the row reads reachable AND the forwarded name is the covered one. The
// row's UNREACHABLE arm is reachability_qa_test.go's; what is added here is
// the agreement.
func TestQATheContainerReachRowAndTheForwardedBeadsDirNameOnePath(t *testing.T) {
	t.Parallel()
	a, ag, e := cbdCage(t)

	root := absResolve(t.TempDir())
	repo := cbdRepo(t, filepath.Join(root, "repo"))
	cbdRepo(t, filepath.Join(root, "real", "store"))
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	spelt := filepath.Join(root, "link", "store", beadsDirName)
	if err := os.WriteFile(filepath.Join(repo, beadsDirName, beadsRedirect), []byte(spelt+"\n"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	home := beadsHome(repo)
	if home != spelt {
		t.Fatalf("fixture: beadsHome = %q, want %q", home, spelt)
	}

	if why := a.containerReachRow(ag, repo, home); why != "" {
		t.Fatalf("the row refuses this launch, so the agreement below is vacuous: %s", why)
	}
	ms := a.CageMounts(ag, e, repo, "reach-probe")
	i, over := cageCovering(ms, absResolve(home))
	if i < 0 {
		t.Fatalf("the row read reachable over a mount set that covers nothing at %s:\n%s", absResolve(home), showMounts(ms))
	}
	// The row's own answer, and the session's, as strings: the bind the row
	// found is the bind the name names.
	if got := a.CageBeadsDir(ag, repo, "s1", home); got != over.Dst {
		t.Errorf("the row reads reachable through bind %s (destination %s) while the session is told the store is %s — the row judges the mount and the session uses the name, and a caged bd follows the name", ms[i].Dst, over.Dst, got)
	}
}

// 4. THE ANSWER COMES OFF THE COVERING BIND'S DESTINATION, and that is the
// half no fixture above can measure: every bind CageMounts renders over a
// store is same-path in and out, so `Src` and `Dst` are one string in each
// of them and a mutant that answered the SOURCE survived all three arms. The
// one bind in a rendered set whose halves differ is the runtime HOME's
// (`{Src: a.CageHome(persona), Dst: e.Home}`), and the rule is general: the
// source says which host directory this is, the destination says what the
// container has, and BEADS_DIR is read inside. Asked of cageBeadsDirIn over
// a set written here, because that is the only way to put the two spellings
// on different sides.
//
// AND CONTAINMENT IS JUDGED ON Dst, which is cageCovering's own rule ("every
// mount a path rule can name is same-path in and out") and not this
// function's to widen: a store reachable only through some bind's SOURCE is
// not found at all. That is the third arm, and it is why the "" this returns
// is a finding rather than a guess — the caller's fallback and ADR 0013 §4's
// reachability row have different things to say about such a session.
func TestQACagedBeadsDirTranslatesToTheMountDestination(t *testing.T) {
	t.Parallel()
	ms := []CageMount{
		{Src: "/qa-host/state/cages/p/home", Dst: "/qa-cage/home", Why: "the caged runtime's HOME (seeded)"},
		{Src: "/qa-host/repo", Dst: "/qa-host/repo", Why: "the session's repo"},
	}
	// Under the one bind whose halves differ. The host side is a real
	// directory on the box outside and names nothing inside the cage.
	if got, want := cageBeadsDirIn(ms, "/qa-cage/home/.beads"), "/qa-cage/home/.beads"; got != want {
		t.Errorf("cageBeadsDirIn answered %q for a store under the HOME bind; the container has it at the destination, %q — %q is that bind's host side and is not a path inside",
			got, want, "/qa-host/state/cages/p/home/.beads")
	}
	// The ordinary bind, where the two halves agree: the control that keeps
	// the arm above from being about path arithmetic.
	if got, want := cageBeadsDirIn(ms, "/qa-host/repo/.beads"), "/qa-host/repo/.beads"; got != want {
		t.Errorf("cageBeadsDirIn answered %q over a same-path bind, want %q", got, want)
	}
	// A store under a bind's SOURCE and no bind's destination is not found,
	// because the container cannot reach it by that name either.
	if got := cageBeadsDirIn(ms, "/qa-host/state/cages/p/home/.beads"); got != "" {
		t.Errorf("cageBeadsDirIn answered %q for a store under a bind's SOURCE — containment is judged on the destination (cageCovering), so the honest answer is the empty string", got)
	}
	// And nothing at all.
	if got := cageBeadsDirIn(ms, "/qa-host/elsewhere/.beads"); got != "" {
		t.Errorf("cageBeadsDirIn answered %q for a store no bind covers, want the empty string", got)
	}
}

// cbdBackend is a launcher that can really render the container tier: a
// hermetic fake engine (whose `command:`'s first word is `env`, so
// engineOnPath answers yes without touching this process's PATH), the
// operator's container credential in an env set, and two PIDs — one caged,
// one not — because the arm below is about the difference between them.
func cbdBackend(t *testing.T) *HerdrBackend {
	t.Helper()
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.CagesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"), []byte(
		"command: env {net} {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Prepended, so a config this home already has keeps every other key.
	cfg, err := os.ReadFile(a.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, append([]byte("default_engine: fake\n"), cfg...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.EnvsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.EnvsDir, "container.env"), []byte("CLAUDE_CODE_OAUTH_TOKEN=sk-not-real\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, front := range map[string]string{
		"caged":   "cage: container\nenvs: [container]\n",
		"uncaged": "",
	} {
		if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"),
			[]byte("---\nname: "+name+"\ndescription: test\n"+front+"---\nYou are "+name+".\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !a.ContainerAvailable() {
		t.Skip("fixture premise: this build does not read the engine fixture as available")
	}
	return b
}

// 5. THE ENV SITE, which is where the value a session actually gets is
// decided, and the arm that keeps arms 1-4 from being a producer nothing
// calls. One launch dir, one store behind a link, two PIDs:
//
//   - CAGED: the launch env names the store as the MOUNT SET spells it, and
//     not as beadsHome does. That value is what `-e BEADS_DIR` forwards from
//     the pane (CageEnvNames), and what the relaunch path re-forwards from
//     the same pane later (RelaunchAgent: "the workspace still carries the
//     launch env").
//   - NOT CAGED: beadsHome's own spelling, untouched. At L1 bd reads the
//     path the operator configured and takes its unsafe-prefix verdict on
//     it; at L2 a seatbelt rule matches the vnode's real path whichever way
//     the env spells it; at L3 the link is still in the seat's namespace.
//     This arm is what makes the fix tier-keyed rather than launch-wide, and
//     it is the one a `resolve it everywhere` change reds.
func TestQAOnlyACagedLaunchEnvCarriesTheMountSetsSpellingOfTheStore(t *testing.T) {
	t.Parallel()
	b := cbdBackend(t)
	a := b.App

	store := blRepo(t)
	root := absResolve(t.TempDir())
	link := filepath.Join(root, "storelink")
	if err := os.Symlink(store, link); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	spelt := filepath.Join(link, beadsDirName)
	resolved := filepath.Join(absResolve(store), beadsDirName)
	if spelt == resolved {
		t.Fatalf("fixture: %s and its resolved path are one string, so nothing below discriminates", spelt)
	}
	work := blRepo(t)
	blRedirect(t, work, spelt)
	if got := beadsHome(work); got != spelt {
		t.Fatalf("fixture: beadsHome = %q, want the redirect's own spelling %q", got, spelt)
	}

	caged, err := b.planLaunch(NewSessionOpts{Name: "cbd1", Dir: work, Agent: "caged", ByHand: true})
	if err != nil {
		t.Fatalf("caged launch: %v", err)
	}
	if caged.Cage != CageContainer || caged.Degraded != "" {
		t.Fatalf("fixture premise: the launch is not really at the container tier (cage=%q degraded=%q)", caged.Cage, caged.Degraded)
	}
	got := cbdVars(caged.Vars)["BEADS_DIR"]
	if got == "" {
		t.Fatal("the caged launch env carries no BEADS_DIR at all — a `--no-db` bd then writes $cwd/.beads and the store is never reached (ADR 0055 D1)")
	}
	ag, err := a.LoadAgent("caged")
	if err != nil {
		t.Fatal(err)
	}
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		t.Fatal(err)
	}
	ms := a.CageMounts(ag, e, caged.Dir, "cbd1")
	m, ok := cbdCoversRaw(ms, got)
	if !ok {
		t.Errorf("the caged launch forwards BEADS_DIR=%s, which is in no bind the mount set names — inside the cage that path does not exist:\n%s", got, showMounts(ms))
	} else if m.RO {
		t.Errorf("the caged launch forwards BEADS_DIR=%s, covered only by a READ-ONLY bind (%s):\n%s", got, m.Dst, showMounts(ms))
	}
	if got == spelt {
		t.Errorf("the caged launch forwards the store AS SPELLED (%s) — that is what shipped, and it is the link's own path, mounted nowhere", got)
	}
	if got != resolved {
		t.Errorf("the caged launch forwards BEADS_DIR=%s; the carve-out binds the store at %s and that is the only spelling the container has", got, resolved)
	}

	// The same dir, one tier down: beadsHome's answer, unchanged.
	host, err := b.planLaunch(NewSessionOpts{Name: "cbd2", Dir: work, Agent: "uncaged", ByHand: true})
	if err != nil {
		t.Fatalf("uncaged launch: %v", err)
	}
	if host.Cage == CageContainer {
		t.Fatalf("fixture premise: the control launch is caged too (cage=%q)", host.Cage)
	}
	if hgot := cbdVars(host.Vars)["BEADS_DIR"]; hgot != spelt {
		t.Errorf("an uncaged launch's BEADS_DIR is %s, want beadsHome's own spelling %s — bd reads the path the operator configured and takes its unsafe-prefix verdict on it, and no wall below L4 needs the name resolved (beadloss.go's contract, ADR 0055 Consequences)", hgot, spelt)
	}
}

// cbdVars indexes an assembled launch env by name.
func cbdVars(vars []EnvVar) map[string]string {
	env := map[string]string{}
	for _, v := range vars {
		env[v.Key] = v.Value
	}
	return env
}

// AND THE FALLBACK, which arm 4 reaches only one layer down. cageBeadsDirIn
// answering "" means no bind covers the store at all, and CageBeadsDir then
// hands back beadsHome's own answer — what shipped before this function —
// because the thing that says the store is unreachable is ADR 0013 §4's row
// over the same mount set, waivable like every other gate, and two refusals
// for one fact would make the gate the liar (CageBeadsDir's own comment).
//
// Arm 5 carries the assertion that would catch a change here — `if got == ""`
// over the forwarded value — but its fixture's store IS covered, so that
// branch is never reached. MEASURED 2026-11-22 (ranger-base-2unmh): returning
// "" instead of `home` survived arms 1-5 and the arm-3 pins. An empty
// BEADS_DIR is the one value worse than an unresolved one: `-e BEADS_DIR`
// forwards it, and a `--no-db` bd with nothing there resolves $cwd/.beads and
// writes the container's own throwaway issues.jsonl, which is the silent
// record loss ADR 0055 D1 exists to prevent and this bead's whole subject.
func TestQACagedBeadsDirFallsBackToBeadsHomeWhenNoBindCoversTheStore(t *testing.T) {
	t.Parallel()
	a, ag, e := cbdCage(t)
	repo := cbdRepo(t, filepath.Join(absResolve(t.TempDir()), "repo"))

	// A store under no bind's destination. The premise is asserted, not
	// assumed: a fixture the mount set happens to cover would make the
	// assertion below vacuous and read exactly like a pass.
	outside := filepath.Join(absResolve(t.TempDir()), "not-mounted", beadsDirName)
	ms := a.CageMounts(ag, e, repo, "s1")
	if i, _ := cageCovering(ms, absResolve(outside)); i >= 0 {
		t.Fatalf("fixture: %s IS covered by a bind, so the fallback is not the branch under test:\n%s", outside, showMounts(ms))
	}
	if got := cageBeadsDirIn(ms, outside); got != "" {
		t.Fatalf("cageBeadsDirIn answered %q for a store no bind covers, want the empty string", got)
	}
	if got := a.CageBeadsDir(ag, repo, "s1", outside); got != outside {
		t.Errorf("CageBeadsDir answered %q where no bind covers the store; want beadsHome's own answer %q. An empty or translated value here is forwarded by `-e BEADS_DIR` to a `--no-db` bd, which then resolves $cwd/.beads and writes the container's throwaway issues.jsonl — the silent record loss ADR 0055 D1 exists to prevent. The gate row over this same mount set is what says the store is unreachable (ranger-base-ayubz)", got, outside)
	}
}
