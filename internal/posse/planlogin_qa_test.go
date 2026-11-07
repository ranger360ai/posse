package posse

// The plan-usage snapshot is keyed by the Claude LOGIN, not by the posse
// home (ADR 0080 D11, ranger-base-ewk6u).
//
// The reason is the incident plancache.go's header is about. The usage
// endpoint's rate limit is per credential, and ADR 0080 D6 puts every crew
// home on one Claude login — so a snapshot under `$StateDir` makes N homes
// N pollers on one limit, and a 429 cooldown home A earned is unknown to
// home B. rangerhq-tdy8 (2026-08-22) is what that costs: the endpoint 429'd
// for three and a quarter hours while three pollers went on asking at their
// own cadence, and the fix — one derived copy, one cooldown, one read log —
// is undone the moment the file is per home instead of per credential.
//
// So the pins here are about WHERE the file is, in both directions:
//
//	two homes, one config dir  -> one store, and B honours A's cooldown
//	two homes, two config dirs -> two stores, and B asks for itself
//
// Both arms matter. The first alone is satisfied by a key that is constant
// — a file named `plan-usage.json` in the cache dir would pass it and would
// put a home with its own `CLAUDE_CONFIG_DIR` on a stranger's cooldown.
// The second alone is satisfied by the per-home path this bead replaced.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loginApp is a test App on the SHIPPED store resolution: an own home, and
// hermetic's per-test PlanUsageDir cleared, because the subject of this file
// is the directory the shipped code resolves rather than one a test handed
// it. Every test here therefore pins XDG_CACHE_HOME first, so the box's real
// `~/.cache/posse/plan-usage` is never the thing being written.
func loginApp(t *testing.T) *App {
	t.Helper()
	a := NewAppAt(t.TempDir())
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return a
}

// loginCache is one home's view of the shared reading, pointed at ps and
// armed: the shipped construction (App.PlanCache), so the path under test is
// the one a real caller gets.
//
// Quiet is cleared for the reason the other plan rigs clear it — these homes
// carry no `plan_guard_<window>:`, and a quiet meter refuses one step
// earlier than any arm here is about (planquiet.go).
func loginCache(t *testing.T, a *App, ps *planServer, caller string, now time.Time) *PlanCache {
	t.Helper()
	c := a.PlanCache(caller)
	if c.Path == "" || c.Log == "" {
		t.Fatalf("%s: the shipped resolution found nowhere to put the store", caller)
	}
	c.Reader, c.NoAdapter, c.Quiet = ps.reader(), nil, nil
	c.Now = func() time.Time { return now }
	return c
}

// Two homes on one login: one file, one reading, and one cooldown.
//
// The cooldown is the half that bought the bead. A reading shared by
// accident is a performance fact; a cooldown NOT shared is the fleet
// re-arming a rate limiter it has already been told to leave alone, which
// is the behaviour that kept the plan guard blind for thirteen hours
// (planCooldown's comment, ranger-base-rwwp6). So home B is given a healthy
// endpoint of its own and must still refuse to ask it.
func TestQAPlanSnapshotIsOneStorePerLoginNotPerHome(t *testing.T) {
	now := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv(ClaudeConfigDirVar, filepath.Join(t.TempDir(), "one-login"))

	a, b := loginApp(t), loginApp(t)
	if a.Home == b.Home {
		t.Fatal("setup: two homes, or this pins nothing")
	}
	as, al := a.PlanUsagePaths()
	bs, bl := b.PlanUsagePaths()
	if as != bs || al != bl {
		t.Fatalf("one login is one store:\n A %s / %s\n B %s / %s", as, al, bs, bl)
	}
	if strings.Contains(as, a.Home) || strings.Contains(as, b.Home) {
		t.Errorf("the store must not be under either home: %s", as)
	}

	// A draws the 429 and writes the cooldown.
	ratelimited := newPlanServer(t, 42, 61)
	ratelimited.status, ratelimited.retry = http.StatusTooManyRequests, "3600"
	ca := loginCache(t, a, ratelimited, "dispatch", now)
	if _, _, err := ca.Read(0); PlanFailureOf(err) != PlanFailRateLimited {
		t.Fatalf("setup: the 429 itself: %v (%q)", err, PlanFailureOf(err))
	}
	if left, holding := ca.Cooling(now); !holding || left != time.Hour {
		t.Fatalf("setup: A's own cooldown: %s holding=%v, want 1h0m0s", left, holding)
	}

	// B is a different home with a working endpoint, and must not ask it.
	healthy := newPlanServer(t, 7, 13)
	cb := loginCache(t, b, healthy, "cockpit", now.Add(time.Minute))
	_, _, err := cb.Read(0)
	if PlanFailureOf(err) != PlanFailRateLimited {
		t.Errorf("home B must honour the cooldown home A earned: %v (%q)", err, PlanFailureOf(err))
	}
	if got := healthy.hits.Load(); got != 0 {
		t.Errorf("home B asked its endpoint %d times through another home's cooldown", got)
	}
	if left, holding := cb.Cooling(now.Add(time.Minute)); !holding || left != 59*time.Minute {
		t.Errorf("B reads the same cooldown off the same file: %s holding=%v, want 59m0s", left, holding)
	}

	// And the reading itself, once the cooldown is gone: B's success is A's
	// cache hit, at no cost to the endpoint.
	after := now.Add(2 * time.Hour)
	cb = loginCache(t, b, healthy, "cockpit", after)
	if _, _, err := cb.Read(0); err != nil {
		t.Fatalf("past the cooldown B may ask: %v", err)
	}
	ca = loginCache(t, a, ratelimited, "cost", after.Add(time.Second))
	u, at, err := ca.Read(5 * time.Minute)
	if err != nil {
		t.Fatalf("A reads B's reading off the shared file: %v", err)
	}
	if win(u, "5h") != 7 || !at.Equal(after) {
		t.Errorf("A got %v taken at %v, want B's 7%% reading taken at %v", u, at, after)
	}
	if got := ratelimited.hits.Load(); got != 1 {
		t.Errorf("A made %d requests, want the one 429 — the second reading was B's", got)
	}
}

// Two config dirs are two logins and two rate limits, so they are two
// stores. This is the arm that makes the key a KEY rather than a constant,
// and it is the case ADR 0080 D11 names as the reason to derive the key at
// all: "sharing automatic and correct when one home uses its own
// CLAUDE_CONFIG_DIR".
//
// The variable is moved between the two resolutions rather than held by two
// Apps at once, because that is what it is: an environment property of the
// process, not a field of the home. One home with its own
// `CLAUDE_CONFIG_DIR` is one process that resolved it differently.
func TestQAPlanSnapshotIsNotSharedAcrossTwoLogins(t *testing.T) {
	now := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	logins := t.TempDir()

	t.Setenv(ClaudeConfigDirVar, filepath.Join(logins, "first"))
	a := loginApp(t)
	as, al := a.PlanUsagePaths()
	ratelimited := newPlanServer(t, 42, 61)
	ratelimited.status, ratelimited.retry = http.StatusTooManyRequests, "3600"
	ca := loginCache(t, a, ratelimited, "dispatch", now)
	if _, _, err := ca.Read(0); PlanFailureOf(err) != PlanFailRateLimited {
		t.Fatalf("setup: the 429 itself: %v (%q)", err, PlanFailureOf(err))
	}

	t.Setenv(ClaudeConfigDirVar, filepath.Join(logins, "second"))
	b := loginApp(t)
	bs, bl := b.PlanUsagePaths()
	if as == bs || al == bl {
		t.Fatalf("two logins are two stores, got one:\n %s\n %s", as, bs)
	}
	if filepath.Dir(as) != filepath.Dir(bs) {
		t.Errorf("both stores live in the one cache dir: %s vs %s", as, bs)
	}
	healthy := newPlanServer(t, 7, 13)
	cb := loginCache(t, b, healthy, "cockpit", now.Add(time.Minute))
	u, _, err := cb.Read(0)
	if err != nil {
		t.Fatalf("the second login owns no part of the first's cooldown: %v", err)
	}
	if win(u, "5h") != 7 {
		t.Errorf("the second login reads its own endpoint: %v", u)
	}
	if got := healthy.hits.Load(); got != 1 {
		t.Errorf("the second login made %d requests, want 1", got)
	}
	if _, holding := cb.Cooling(now.Add(time.Minute)); holding {
		t.Error("the second login must not be holding the first's cooldown")
	}
}

// One config dir spelled two ways is one login. The key is the REALPATH for
// bdWriteLockPath's reason (ADR 0072 D2's): a symlinked spelling that keyed
// its own file would be the per-home bug again, wearing a different name —
// two homes, one credential, two cooldowns.
func TestQAPlanSnapshotKeyFollowsASymlinkedConfigDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	real := filepath.Join(root, "real-login")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked-login")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	t.Setenv(ClaudeConfigDirVar, real)
	direct, _ := loginApp(t).PlanUsagePaths()
	t.Setenv(ClaudeConfigDirVar, link)
	linked, _ := loginApp(t).PlanUsagePaths()
	if direct == "" || direct != linked {
		t.Errorf("one config dir, one store, whatever it is spelled:\n %s\n %s", direct, linked)
	}
}

// Nowhere to key it, and nowhere to put it: both answer "no sharing" rather
// than a path nobody named.
//
// This is ExpandTilde's rule (ranger-base-a3t1) at two sites. filepath.Join
// DROPS an empty element, so an unguarded resolution with no $HOME would
// hand back a RELATIVE `.cache/posse/plan-usage` and key it by a relative
// `.claude` — a store under whatever cwd the process happened to have, and
// a different one per cwd. Path "" is already "no sharing, every read is a
// request" (PlanCache.Path), which is the honest answer and the one the
// whole file is built to degrade to.
func TestQAPlanSnapshotHasNowhereToGoWithoutAHome(t *testing.T) {
	for _, tc := range []struct {
		name          string
		home, cache   string
		cfg           string
		wantDir, want bool
	}{
		{name: "no HOME and no CLAUDE_CONFIG_DIR: neither half resolves"},
		{name: "a cache root but no login to key by", cache: "/tmp/cache", wantDir: true},
		{name: "a login but nowhere to put the store", cfg: "/tmp/login"},
		{name: "both, from the home alone", home: "/tmp/home", wantDir: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := PlanUsageCacheDirIn(tc.home, tc.cache)
			if (dir != "") != tc.wantDir {
				t.Errorf("cache dir = %q, want empty=%v", dir, !tc.wantDir)
			}
			cfg := tc.cfg
			if cfg == "" && tc.home != "" {
				cfg = filepath.Join(tc.home, ".claude")
			}
			snapshot, readLog := PlanUsageStoreIn(dir, cfg)
			if (snapshot != "") != tc.want || (readLog != "") != tc.want {
				t.Errorf("store = (%q, %q), want empty=%v", snapshot, readLog, !tc.want)
			}
			if tc.want && filepath.Dir(snapshot) != dir {
				t.Errorf("the store must sit in the cache dir: %s not under %s", snapshot, dir)
			}
		})
	}
}

// Rule 5 still gates the store, and it has to be re-pinned HERE because the
// file moved out from under it: credpin.go's own pins are written against a
// hand-built Path in a tempdir, and what this bead changed is that the real
// path is now the BOX's rather than one test home's. A `posse cost --plan`
// under an RHQ_PLAN_USAGE_URL override that published its answer used to
// poison one instance; it would now publish to every home on the login
// (ranger-base-dr6u, reproduced live on ranger-base-7nlw).
//
// Both outcomes, because share() gates the 429 branch as well as the success
// one: a loopback listener answering `429 Retry-After: 3600` would otherwise
// park every posse process on the box for an hour, uncredentialed.
func TestQAAnOverriddenEndpointWritesNoLoginStore(t *testing.T) {
	now := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv(ClaudeConfigDirVar, filepath.Join(t.TempDir(), "one-login"))
	a := loginApp(t)
	snapshot, readLog := a.PlanUsagePaths()

	for _, tc := range []struct {
		name   string
		status int
		retry  string
	}{
		{name: "a success an override fetched", status: http.StatusOK},
		{name: "a 429 an override drew", status: http.StatusTooManyRequests, retry: "3600"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ps := newPlanServer(t, 0, 0)
			ps.status, ps.retry = tc.status, tc.retry
			override := ps.reader()
			// The one thing an override is: not vouched for as the
			// compiled-in endpoint. Everything else about the reader is the
			// shipped one, so this arm is the store gate alone.
			override.Shared = false
			if override.MayShare() {
				t.Fatal("setup: this arm is only interesting while the gate is shut")
			}
			c := a.PlanCache("cost")
			c.Reader, c.NoAdapter, c.Quiet = override, nil, nil
			c.Now = func() time.Time { return now }
			c.Read(0)

			if _, err := os.Stat(snapshot); err == nil {
				b, _ := os.ReadFile(snapshot)
				t.Errorf("an overridden answer became the login's shared fact: %s", b)
			}
			// The positive witness: the request DID happen, so the absence
			// above means the gate held rather than that nothing ran. The
			// read log is deliberately not gated — an override that is
			// refused a place in the snapshot is still evidence in the
			// cadence file (logRead).
			b, err := os.ReadFile(readLog)
			if err != nil || !strings.Contains(string(b), " cost ") {
				t.Errorf("the read log must still name the caller: %q (%v)", b, err)
			}
		})
	}
}
