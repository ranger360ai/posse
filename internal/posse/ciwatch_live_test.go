//go:build posse_arm3

package posse

// Live pin for ci-watch (ranger-base-x9e34, ciwatch.go), run against the
// real bd and a real `gh`-shaped child rather than against the fakes:
//
//	RHQ_LIVE_BD=1 go test ./internal/posse/ -run TestLiveCIWatchFiresOnceAndClears -v
//
// The bead asks for exactly this and says why: "turn main red on purpose in
// a scratch clone and watch the mechanism fire once, then green it and watch
// it clear." The fakes model bd, and the fake is what got the last one of
// these wrong (settleescalation_live_test.go's header). What only real bd
// can answer here:
//
//   - a bead filed with `-l ci-red,devops` really does come back out of
//     `bd list --label-any ci-red` — the dedupe is one query and a store
//     that answered it differently would file per pass;
//   - a bead a PERSONA closed really does leave that query, which is what
//     stops ci-watch adopting an answered bead forever — and so does one
//     ci-watch closed ITSELF, which it does for a bead no session ever
//     claimed (ADR 0013 §4's one exception, ranger-base-8fr2j; ciwatch.go's
//     header). Both closes are asked of the real store here, because the
//     whole saving depends on the row leaving the dedupe's query;
//   - a real `bd update --claim` really does put the status and assignee
//     that ciHolder reads onto the row `bd list --label-any` answers with —
//     the fake's claim state and the real store's are two different things,
//     and the guard reads the listing;
//   - a comment ci-watch wrote on an earlier pass really does come back out
//     of `bd comments` — the drumbeat's cadence and the whole green half's
//     state are read back out of that one query;
//   - the description survives bd's own round trip with the marker line
//     intact, newlines and all — the marker IS the dedupe.
//
// The gh side is a real child process too: a two-line shell script serving a
// runs.json this test rewrites between passes, exec'd through the shipped
// ghRunList with the shipped argv. Turning the gate red and green is one
// file write, which is the "scratch clone" the bead asks for with no clone
// and no workflow run.
//
// Env-gated and skipped by default, like every other live pin here: it
// shells out to the operator's bd, which has a version and a daemon, neither
// of which belongs in a hermetic suite. Everything happens inside t.TempDir
// — the `bd init` is the throwaway-database case, never a repo anybody
// keeps, and nothing here can see the shop's own store.
//
// THAT LAST CLAUSE IS TRUE OF THE DIRECT bd CALLS ONLY SINCE
// ranger-base-1ump9 (F2), and it is the reason this pin had never once run.
// Every session posse launches carries BEADS_DIR naming the store of record
// (ADR 0055) and bd resolves it ahead of cmd.Dir, so the `sh` closure's
// calls — the `init` among them — went to the LIVE graph while every Go-side
// call went to <repo>/.beads; the store the rest of this test assumes was
// never created, and the pin reported that by SKIPPING, which is what it
// does when it is not asked to run at all. See the closure for the one-line
// binding and why it is the shipped bdStoreEnv rather than an append.
// MEASURED 2026-10-05: `bd --no-daemon where` from a fresh throwaway repo
// answers the live store under the inherited environment and "no beads
// database found" under bdStoreEnv(os.Environ(), repo), and with the binding
// in place this pin PASSES rather than skipping.

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveCIWatchFiresOnceAndClears(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	bdbin, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("no bd on PATH")
	}

	repo := gitTempDir(t)
	// Bd.run execs Bin directly, so --no-daemon rides in a wrapper rather
	// than in the argv the code under test builds (settleescalation's rule):
	// a daemon per throwaway db is a leak, and the point here is the store.
	wrapper := filepath.Join(gitTempDir(t), "bd-nodaemon")
	if err := WriteExecutable(wrapper, []byte("#!/bin/sh\nexec "+bdbin+" --no-daemon \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// cmd.Env, and the SHIPPED binder rather than a hand-written
	// `BEADS_DIR=` append (ranger-base-1ump9 F2). Every session posse
	// launches carries BEADS_DIR naming the store of record (ADR 0055), and
	// bd resolves it ahead of cmd.Dir — so without this the `init` below and
	// the two direct writes after it are aimed at the live graph, while
	// every GO-side call is bound to <repo>/.beads by Bd.runOnce
	// (bdStoreEnv, beads.go; beadsstorebind_test.go). The store the next
	// sixty lines assume was then never created, and the pin SKIPS, which is
	// indistinguishable from the skip it already has one line down.
	//
	// bdStoreEnv is correct in both phases, which is what the append is not:
	// it sheds every inherited store-repointing variable unconditionally and
	// only SETS BEADS_DIR `if isDirPath(home)`. For the `init` call, before
	// <repo>/.beads exists, it sheds the session's value and sets nothing, so
	// bd falls to $cwd/.beads with cmd.Dir already repo — which is where the
	// store belongs; for every call after it, it names the store the init
	// made.
	sh := func(args ...string) (string, error) {
		cmd := exec.Command(wrapper, args...)
		cmd.Dir = repo
		cmd.Env = bdStoreEnv(os.Environ(), repo)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := sh("init", "--prefix", "ciw"); err != nil {
		t.Skipf("bd init did not take in a throwaway repo: %v %s", err, out)
	}

	// The repo ReadCI will accept, and a gh that answers out of a file this
	// test rewrites — the gate, turned by hand.
	gitRun := func(args ...string) {
		t.Helper()
		if _, err := git(repo, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	if _, err := git(repo, "rev-parse", "--git-dir"); err != nil {
		gitRun("init", "-b", "main")
	}
	gitRun("remote", "add", "origin", "https://github.com/ranger360ai/posse.git")
	// An identity and one commit with refs/remotes/origin/main on it. The
	// SHIPPED reading checks the page it was handed against that ref
	// (ciFreshness, ranger-base-m46kr) and a checkout without one abstains
	// rather than reads — which, in a pin this file SKIPS by default, is a
	// break nobody would see until they ran it. The run shas below are fakes
	// the object store has never heard of, which the guard reads as a page
	// ahead of the local view rather than behind it.
	gitRun("config", "user.email", "t@example.com")
	gitRun("config", "user.name", "t")
	ciChain(t, repo, 1)
	if err := os.MkdirAll(filepath.Join(repo, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".github", "workflows", "ci.yml"), []byte("name: ci\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ghdir := gitTempDir(t)
	runsPath := filepath.Join(ghdir, "runs.json")
	ghPath := filepath.Join(ghdir, "gh")
	if err := WriteExecutable(ghPath, []byte("#!/bin/sh\nexec cat "+runsPath+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-2 * time.Hour)
	setGate := func(rows ...string) {
		t.Helper()
		if err := os.WriteFile(runsPath, []byte("["+strings.Join(rows, ",")+"]"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b, _ := newTestBackend(t)
	a := b.App
	if err := os.WriteFile(a.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The SHIPPED reading, with only the binary substituted — this is what
	// makes the gh half of this end to end rather than a stub.
	a.CIRead = func(q CIQuery) CIState {
		q.GhBin = ghPath
		return ReadCI(q)
	}
	bd := Bd{Bin: wrapper}
	pass := func() (int, string, string) {
		t.Helper()
		var out, errb strings.Builder
		n := a.CIWatch(bd, a.BeadsDirs(), &out, &errb)
		return n, out.String(), errb.String()
	}
	openCiRed := func() []BdIssue {
		t.Helper()
		is, err := bd.OpenLabeledAny(repo, CIRedLabel)
		if err != nil {
			t.Fatalf("bd list --label-any %s: %v", CIRedLabel, err)
		}
		return is
	}

	// ── green to start: nothing to say, nothing filed ────────────────────
	setGate(ciRunJSON("aaaaaaaa", "completed", "success", at))
	if n, out, errs := pass(); n != 0 || cwSay(out) != "" {
		t.Fatalf("a green gate acted %d and said %q (%s)", n, cwSay(out), errs)
	}
	if is := openCiRed(); len(is) != 0 {
		t.Fatalf("a green gate filed %d beads", len(is))
	}

	// ── red: one bead, and it stays one across four more passes ──────────
	setGate(
		ciRunJSON("bbbbbbbb", "completed", "failure", at.Add(20*time.Minute)),
		ciRunJSON("aaaaaaaa", "completed", "success", at),
	)
	n, out, errs := pass()
	if n != 1 {
		t.Fatalf("the red gate acted %d, want 1 (out %q err %q)", n, out, errs)
	}
	if !strings.Contains(out, "ci red ·") {
		t.Errorf("the filing pass said %q", out)
	}
	first := openCiRed()
	if len(first) != 1 {
		t.Fatalf("real bd answers --label-any %s with %d beads, want 1", CIRedLabel, len(first))
	}
	id := first[0].ID
	if !strings.Contains(first[0].Description, ciMarkerPrefix) {
		t.Errorf("the marker did not survive bd's round trip:\n%s", first[0].Description)
	}
	for i := 0; i < 4; i++ {
		if n, out, _ := pass(); n != 0 || cwSay(out) != "" {
			t.Fatalf("pass %d over the SAME red acted %d and said %q — one bead per episode, not per pass", i, n, cwSay(out))
		}
	}
	if is := openCiRed(); len(is) != 1 || is[0].ID != id {
		t.Fatalf("after five red passes the store holds %d ci-red beads", len(is))
	}

	// ── the drumbeat, against real bd's own comments ─────────────────────
	setGate(
		ciRunJSON("dddddddd", "completed", "failure", at.Add(80*time.Minute)),
		ciRunJSON("cccccccc", "completed", "failure", at.Add(50*time.Minute)),
		ciRunJSON("bbbbbbbb", "completed", "failure", at.Add(20*time.Minute)),
		ciRunJSON("aaaaaaaa", "completed", "success", at),
	)
	pass()
	cs, err := bd.Comments(repo, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 {
		t.Fatalf("streak 1 -> 3 earned %d comments, want 1", len(cs))
	}
	pass()
	if cs, _ := bd.Comments(repo, id); len(cs) != 1 {
		t.Errorf("a pass at the SAME streak commented again: %d comments", len(cs))
	}

	// ── a SEAT CLAIMS it, through real bd's own claim ────────────────────
	// ADR 0013 §4's exception (ranger-base-8fr2j) is "no session ever
	// claimed it", and `--claim` is what a dispatched session does to this
	// bead. Against real bd rather than a rewritten listing: the fake's
	// claim state and the real store's are two different things, and this
	// arm exists to say that the row ci-watch reads back out of
	// `bd list --label-any` really does carry the status and assignee the
	// guard reads.
	if out, err := sh("--actor", "devops", "update", id, "--claim"); err != nil {
		t.Fatalf("bd update %s --claim: %v %s", id, err, out)
	}
	if is := openCiRed(); len(is) != 1 || ciHolder(is[0]) == "" {
		t.Fatalf("after a real claim the listing answers %v and ciHolder reads it as unheld — the guard is reading a field the store does not fill", is)
	}

	// ── green again over a bead a seat holds: said, and NOT closed ───────
	setGate(ciRunJSON("eeeeeeee", "completed", "success", at.Add(2*time.Hour)))
	n, out, errs = pass()
	if n != 1 {
		t.Fatalf("the green gate acted %d, want 1 (out %q err %q)", n, out, errs)
	}
	if !strings.Contains(out, "ci green ·") || !strings.Contains(out, "eeeeeeee") {
		t.Errorf("the clearing pass said %q", out)
	}
	shown, err := bd.Show(repo, id)
	if err != nil {
		t.Fatal(err)
	}
	if shown.Status == "closed" {
		t.Fatalf("ci-watch CLOSED %s, which devops holds — ADR 0013 §4 makes that the persona's, and the exception is a bead nobody claimed", id)
	}
	if cs, _ := bd.Comments(repo, id); !ciAlreadyCleared(cs) {
		t.Fatalf("real bd does not answer with the clearing comment, so the next red would file over it")
	}
	if n, out, _ := pass(); n != 0 || cwSay(out) != "" {
		t.Errorf("a second green pass acted %d and said %q", n, cwSay(out))
	}

	// ── the persona's close, and the dedupe blind to it ──────────────────
	// The dedupe must step over a bead a persona closed. Whether `bd list
	// --label-any` still ANSWERS with it is the store class's business:
	// measured 2026-09-04, the shop's SQLite store drops closed rows here
	// and the `no-db: true` JSONL store `bd init` writes on bd 0.50.3 —
	// which is the store THIS rig has — keeps them. This is the arm that
	// found it, so it is also the live pin of the fix: OpenLabeledAny drops
	// them itself (ranger-base-bwrp8), which is asserted here against real
	// bd on the store class that does not.
	if out, err := sh("close", id, "-r", "the gate cleared itself"); err != nil {
		t.Fatalf("bd close %s: %v %s", id, err, out)
	}
	if is := openCiRed(); len(is) != 0 {
		t.Fatalf("OpenLabeledAny answered with %d bead(s) after the only one was closed (%s) — "+
			"open is this query's promise, not the store's", len(is), is[0].Status)
	}
	if is := ciOpenAdopted(t, bd, repo); is != nil {
		t.Fatalf("the dedupe adopted %s (%s) after a persona closed it — the next red would never be filed", is.ID, is.Status)
	}

	// ── and a NEW red is a NEW bead ──────────────────────────────────────
	setGate(
		ciRunJSON("ffffffff", "completed", "failure", at.Add(3*time.Hour)),
		ciRunJSON("eeeeeeee", "completed", "success", at.Add(2*time.Hour)),
	)
	if n, _, errs := pass(); n != 1 {
		t.Fatalf("the second red episode acted %d, want 1 (%s)", n, errs)
	}
	second := ciOpenAdopted(t, bd, repo)
	if second == nil || second.ID == id {
		t.Fatalf("the second episode did not get its own bead (adopted %v, first=%s)", second, id)
	}

	// ── and THIS one nobody claims, so the harness closes it itself ──────
	// The exception, end to end against real bd: the same green pass that
	// only commented above both comments and closes here, and the ONLY
	// difference between the two beads is that a session claimed one of
	// them. What real bd is asked here that no fake can answer: that
	// `bd close --json` under VerifyActor takes on a bead the harness
	// filed, and that the row then leaves the dedupe's own query — the
	// whole saving depends on both.
	setGate(ciRunJSON("99999999", "completed", "success", at.Add(4*time.Hour)))
	n, out, errs = pass()
	if n != 1 {
		t.Fatalf("the green gate over an unclaimed bead acted %d, want 1 (out %q err %q)", n, out, errs)
	}
	if !strings.Contains(out, "CLOSED") || !strings.Contains(out, second.ID) {
		t.Errorf("the clearing pass said %q, want it to name the bead it closed", out)
	}
	shown, err = bd.Show(repo, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if shown.Status != "closed" {
		t.Fatalf("%s is %q after the green pass — ADR 0013 §4's exception (ranger-base-8fr2j) closes a bead no session ever claimed, and real bd did not take the close", second.ID, shown.Status)
	}
	cs, cerr := bd.Comments(repo, second.ID)
	if cerr != nil {
		t.Fatal(cerr)
	}
	if !ciAlreadyCleared(cs) {
		t.Errorf("the closed bead does not carry the clearing comment — a closed ci-red bead has to name the run that answered it, which is what the ruling's DONE WHEN asks for")
	}
	if !strings.Contains(strings.Join(func() (out []string) {
		for _, c := range cs {
			out = append(out, c.Text)
		}
		return
	}(), "\n"), "99999999") {
		t.Errorf("the close comment does not name the clearing run: %v", cs)
	}
	if is := openCiRed(); len(is) != 0 {
		t.Fatalf("the harness's own close left %d bead(s) in the dedupe's query — the next red would never be filed", len(is))
	}
}

// ciOpenAdopted is the bead ci-watch's own dedupe would adopt right now, or
// nil — the shipped ciOpenBeads, asked the same question the pass asks.
func ciOpenAdopted(t *testing.T, bd Bd, repo string) *BdIssue {
	t.Helper()
	st := CIState{Slug: "ranger360ai/posse", Workflow: "ci.yml", Branch: "main"}
	is, err := ciOpenBeads(bd, repo, st)
	if err != nil {
		t.Fatalf("ciOpenBeads: %v", err)
	}
	if len(is) == 0 {
		return nil
	}
	return &is[0]
}

// ─── the job-level vet, against the real GitHub (ranger-base-rdi79) ──────────

// What only the real endpoint can answer: that ciJobsPage parses the payload
// GitHub actually sends, and that the rule says "the queue" over the episode
// it was written for and "a real red" over a run that really failed. The
// fakes pin the rule; they cannot pin the shape of a page nobody here wrote.
//
//	RHQ_LIVE_GH=1 go test -count=1 -tags posse_arm3 ./internal/posse/ \
//	  -run TestLiveCIJobsVet -v
//
// No GH_TOKEN on that line, and that absence is ranger-base-jql54. TestMain
// gives the whole test binary a temp $HOME, so the operator's `gh` config —
// and, on this box, the keyring the token is actually kept in — is invisible
// to every child this execs: gh answers "please run gh auth login" (exit 4)
// rather than reaching GitHub, both arms t.Skipf on that, and the pin reports
// PASS in 0.00s having asked GitHub nothing. MEASURED 2026-10-05 in the
// bead's own transcript, and again by hand: the same `gh api .../jobs` call
// answers 6 jobs under the operator's HOME and exit 4 under a tempdir.
//
// Handing the token in through GH_TOKEN was the first reading of that, and it
// is a workaround with two edges — a secret on a command line, and go's test
// cache keys on the env a test READS through os.Getenv, which GH_TOKEN never
// is, so a run that skipped for want of a token is replayed from cache over a
// command that now has one. ghAtOperatorHome is the fix instead: the child
// gets the $HOME it would have had outside this binary, and the operator
// types nothing. `-count=1` stays load-bearing for the other half of that
// cache reason — nothing about a network answer is in the key.
//
// And the auth skip is gone: RHQ_LIVE_GH=1 means "I expect this to run", so
// the only failure these arms still skip on is the one the next paragraph
// promises to skip on. ghLiveNotRun is where that line is drawn.
//
// Both run ids are FIXED, because both are historical facts rather than
// current state — a live pin keyed on "whatever is red right now" asserts
// nothing on the day main is green, which is most days. If GitHub has aged
// either run out, this skips rather than failing: the subject is the parse
// and the rule, and a run that is gone is not evidence against either.
//
//   - 37362765576 ATTEMPT 1 is the episode (2026-10-05, bab60e51): five green
//     jobs and `test (ubuntu-latest, 3)`, fifteen minutes queued and then
//     cancelled with zero steps and no runner. The attempt path is used only
//     to reach it — the shipped ghRunJobs asks for the LATEST attempt and
//     that run was re-run green, which is exactly why it has to be spelled
//     out here (ghRunJobs's own note on filter=all).
//   - 34632088265 (2026-09-11, 904059b2) is an ordinary red: two jobs with
//     conclusion `failure` and 11 executed steps each, read through the
//     SHIPPED path with no attempt pinning at all.
func TestLiveCIJobsVet(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_GH") == "" {
		t.Skip("set RHQ_LIVE_GH=1 (asks the real GitHub through the real gh)")
	}
	real, err := exec.LookPath(ghBin(""))
	if err != nil {
		t.Skip("no gh on PATH")
	}
	// Not `real` directly: the binary's $HOME is a tempdir, and gh reads its
	// auth under the operator's (ghAtOperatorHome).
	bin := ghAtOperatorHome(t, real)
	const slug = "ranger360ai/posse"
	dir := t.TempDir()

	parse := func(t *testing.T, raw []byte) ciJobsPage {
		t.Helper()
		var p ciJobsPage
		if jerr := json.Unmarshal(trimToJSON(raw), &p); jerr != nil {
			t.Fatalf("the real payload is not the JSON ciJobsPage declares: %v\n%s", jerr, raw)
		}
		if len(p.Jobs) == 0 {
			t.Fatalf("parsed 0 jobs out of %d bytes — the field names have moved", len(raw))
		}
		if p.TotalCount != len(p.Jobs) {
			t.Fatalf("total_count %d over %d rows: this page is paginated, and the rule's completeness check would (correctly) decline it", p.TotalCount, len(p.Jobs))
		}
		return p
	}

	t.Run("the episode is the queue", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(bin, "api", "repos/"+slug+"/actions/runs/37362765576/attempts/1/jobs?per_page=100")
		cmd.Dir = dir
		out, cerr := cmd.Output()
		if cerr != nil {
			ghLiveNotRun(t, "attempt 1 of run 37362765576", cerr)
		}
		p := parse(t, out)
		why, queueOnly := ciJobsSayQueue(p)
		if !queueOnly {
			t.Fatalf("the episode's own jobs page did not read as the queue (%d jobs): %+v", len(p.Jobs), p.Jobs)
		}
		if !strings.Contains(why, "test (ubuntu-latest, 3)") || !strings.Contains(why, "never started") {
			t.Errorf("why = %q, want it to name the job that never started", why)
		}
	})

	t.Run("an ordinary red is a verdict", func(t *testing.T) {
		t.Parallel()
		raw, rerr := ghRunJobs(dir, bin, slug, "34632088265")
		if rerr != nil {
			ghLiveNotRun(t, "run 34632088265", rerr)
		}
		p := parse(t, raw)
		if why, queueOnly := ciJobsSayQueue(p); queueOnly {
			t.Fatalf("a run with two failed jobs was set aside: %q", why)
		}
		failed := 0
		for _, j := range p.Jobs {
			if j.Conclusion == "failure" {
				failed++
				if len(j.Steps) == 0 {
					t.Errorf("job %q failed with zero steps parsed — `steps` is the discriminator and it did not survive the parse", j.Name)
				}
			}
		}
		if failed == 0 {
			t.Errorf("no job of run 34632088265 parsed as failed, but its conclusion is failure: the parse is wrong or the run is not the one this pin names")
		}
	})
}

// ghAtOperatorHome is the real `gh` with exactly one thing changed: the
// child's $HOME. It writes a two-line wrapper and hands back its path, so
// both arms above reach GitHub through the SHIPPED argv — arm 2 goes through
// ghRunJobs, which sets no cmd.Env at all and must not start to. In
// production the inherited environment is already the operator's; the only
// process on this box that lies to a child about $HOME is this test binary,
// and the lie is deliberate (herdr_test.go's TestMain, "HOME: one temp home
// for the whole binary").
//
// A wrapper rather than cmd.Env for the same reason the keychain pin replaces
// a closure rather than the process's environment (liveKeychainStoreAt,
// credentialaccount_live_test.go): os.Setenv/t.Setenv here would hand the
// operator's live home to every test running in parallel beside this one, and
// nearly every test in this package is one.
//
// HOME alone is enough. MEASURED 2026-10-05 on this box: `gh auth status`
// reports the github.com token under `(keyring)`, and the keyring is the
// login keychain under $HOME/Library/Keychains — the same reason the
// credential pin has to do this for `security`. GH_CONFIG_DIR needs no
// mention because TestMain does not clear it (it clears CLAUDE_CONFIG_DIR,
// CODEX_HOME and GROK_HOME), so an operator who exported one already has it
// in os.Environ and the wrapper passes it through untouched.
func ghAtOperatorHome(t *testing.T, bin string) string {
	t.Helper()
	if operatorHome == "" {
		t.Fatal("TestMain recorded no operator $HOME, so there is no gh config and no keyring to point the child at — this pin cannot run from a binary started without HOME")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	p := filepath.Join(gitTempDir(t), "gh")
	script := "#!/bin/sh\nHOME=" + shellQuote(operatorHome) + "\nexport HOME\nexec " + shellQuote(bin) + " \"$@\"\n"
	if err := WriteExecutable(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// ghLiveNotRun says what a gh child that did not answer costs, and it is the
// other half of ranger-base-jql54. RHQ_LIVE_GH=1 is an operator saying "I
// expect this to run", so the only failure this pin may SKIP on is the one
// its header promises to skip on — a run GitHub has aged out, which is not
// evidence against the parse or the rule. Everything else is a rig failure,
// and a rig failure that skips is a PASS with zero evidence: that is exactly
// how exit 4 hid here for as long as it did, printed as a t.Logf line under a
// green PASS that nobody reads twice.
//
// The classification reads the message rather than only the code because the
// two arms lose different halves of the failure. Arm 1 uses cmd.Output, so
// the exit code is on the *exec.ExitError and gh's sentence is in .Stderr;
// arm 2 goes through the shipped ghRunJobs, which keeps gh's stderr text and
// drops the code (Die, ciwatch.go). Both spellings of "not logged in" carry
// the words.
func ghLiveNotRun(t *testing.T, what string, err error) {
	t.Helper()
	msg := err.Error()
	code := -1
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
		if se := strings.TrimSpace(string(ee.Stderr)); se != "" {
			msg += ": " + se
		}
	}
	switch {
	case strings.Contains(msg, "HTTP 404"), strings.Contains(msg, "HTTP 410"), strings.Contains(msg, "Not Found"):
		t.Skipf("GitHub no longer serves %s (%s) — the subject here is the parse and the rule, and a run that is gone is not evidence against either", what, msg)
	case code == 4, strings.Contains(msg, "gh auth login"), strings.Contains(msg, "GH_TOKEN"):
		t.Fatalf("gh could not authenticate for %s (%s) — this is the rig, not GitHub: see ghAtOperatorHome, and check `gh auth status` in the shell that started this", what, msg)
	default:
		t.Fatalf("gh could not reach %s (%s) — RHQ_LIVE_GH=1 says this run is expected to ask GitHub, so this is a failure and not a skip", what, msg)
	}
}

// The guard for the pin above, and the one test in this file that always
// runs: it asks a STUB `gh` what $HOME it was handed, through the shipped
// ghRunJobs and the shipped parse, so it needs no token, no keyring, no
// network and no darwin.
//
// It exists because the defect it pins was invisible (ranger-base-jql54).
// Both arms read the right runs, named them in their skip lines and reported
// PASS in 0.00s, and the first reading of that output was that GitHub had
// aged the runs out. A pin the operator can only run by hand, on a box
// nobody else has, is exactly the kind that rots unobserved — so the thing it
// depends on is pinned by a test the suite runs, the way the keychain pin's
// is (TestQALiveKeychainReadHandsTheChildTheOperatorHome).
func TestQALiveCIJobsVetHandsGhTheOperatorHome(t *testing.T) {
	t.Parallel()
	// One job whose NAME is the child's own $HOME, so the assert reads what
	// gh itself would have read, back out of ciJobsPage.
	stub := filepath.Join(gitTempDir(t), "gh-stub")
	if err := WriteExecutable(stub, []byte("#!/bin/sh\nprintf '{\"total_count\":1,\"jobs\":[{\"name\":\"%s\"}]}\\n' \"$HOME\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := ghRunJobs(gitTempDir(t), ghAtOperatorHome(t, stub), "o/n", "1")
	if err != nil {
		t.Fatalf("the stubbed read must answer: %v", err)
	}
	var p ciJobsPage
	if jerr := json.Unmarshal(trimToJSON(raw), &p); jerr != nil {
		t.Fatalf("the stub's payload did not parse: %v\n%s", jerr, raw)
	}
	if len(p.Jobs) != 1 {
		t.Fatalf("parsed %d jobs out of %q, want the stub's one", len(p.Jobs), raw)
	}
	if p.Jobs[0].Name != operatorHome {
		t.Errorf("the gh child was handed HOME=%q, want the operator's %q — with the package's temp HOME the real gh exits 4 on a keyring it cannot see, and the two arms above can only skip", p.Jobs[0].Name, operatorHome)
	}
	// And the temp HOME really is different, or the assert above is vacuous.
	if operatorHome == os.Getenv("HOME") {
		t.Errorf("TestMain did not replace HOME (%q) — this guard proves nothing while the two are equal", operatorHome)
	}
}
