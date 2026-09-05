package posse

// The LAUNCH's env sets reach the catalog probe (ADR 0039 D3d as amended,
// ranger-base-q3n4e items 3-5; built on ranger-base-hr49g).
//
// modelsessiontoken_test.go pins the probe's credential over the
// PERSONA-LESS list — the cockpit's, and the one `ReadCredential(rt,
// CredSession)` answers. This file pins the other half: a launch names its
// own sets, and the mint the endpoint sees is the one THAT launch is about
// to export, read out of the files at the moment of the probe.
//
// Every fixture here is hermetic in the same two ways: the endpoint is an
// httptest server the built lister is pointed at, and the credential is an
// env set FILE under a scratch home. The meter half is a fake token func —
// the App path wires MeterToken behind the session credential, and a pin
// that let it run would read the operator's keychain, so each arm replaces
// the Fallback it was handed after asserting (in modelsessiontoken_test.go,
// TestModelCacheWiresSessionFirstAndTheMeterStoreBehindIt) that the real one
// is what the wiring puts there.
//
// Every arm ALSO holds the credential's real name in the TEST PROCESS's
// environment, at a value that must never be presented, and the home's
// `default_env` carries a THIRD value. So an arm that passes has ruled out
// both of the other places this value has ever been read from: the process
// environment (retracted, ranger-base-q3n4e) and the cockpit's own set.
//
// Nothing below prints a credential.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"testing"
)

const (
	// The cockpit's mint: what the persona-less list would answer with. A
	// launch that names its own sets must never present it — which is the
	// whole of what the plumbing under test is for. Fake, never printed.
	fakeCockpitMint = "sk-ant-oat01-fake-cockpit-do-not-log-me"
	// The two launch-set mints, for the ordering arms. Two spellings so a
	// mismatch is unambiguous to the code and to nobody else.
	fakeSetAMint = "sk-ant-oat01-fake-set-a-do-not-log-me"
	fakeSetBMint = "sk-ant-oat01-fake-set-b-do-not-log-me"
)

// launchHome is a scratch home holding the two places a mint could be read
// from other than the launch's own sets: the process environment (poison)
// and the cockpit's `default_env` set (fakeCockpitMint). It returns the App
// and the credential's variable name.
//
// Whatever an arm then writes into a launch set is the only value it is
// legitimate for the endpoint to see.
func launchHome(t *testing.T) (*App, string) {
	t.Helper()
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime(modelCatalogRuntime)
	if err != nil {
		t.Fatal(err)
	}
	key := CageCredential(rt)
	if key == "" {
		t.Fatal("the built-in claude runtime names no session credential — this file's whole subject")
	}
	t.Setenv(key, envArmPoison)
	sessionSet(t, a, "cockpit", key, fakeCockpitMint)
	namedDefaultEnv(t, a, "cockpit")
	return a, key
}

// probeLister is the lister a launch's catalog read would really use — built
// by the App path, so Token is the wiring under test — pointed at the fake
// endpoint and with the meter store replaced by a fake. Nothing else about
// it is touched.
func probeLister(t *testing.T, mc *ModelCache, srv *bearerLog) *ModelLister {
	t.Helper()
	l := mc.Lister
	if l.Token == nil || l.Fallback == nil {
		t.Fatal("the App path did not wire both credentials")
	}
	l.URL, l.HTTP, l.Fallback = srv.URL, srv.Client(), fixedToken(fakeMeterMint)
	return l
}

// The preference, over the launch's list: the mint the endpoint sees is the
// one in the set the LAUNCH names, and not the one in the set the cockpit
// would have read — which is sitting right there in `default_env` under a
// different value the whole time.
func TestLaunchListedSetIsTheCredentialTheProbePresents(t *testing.T) {
	a, key := launchHome(t)
	sessionSet(t, a, "projA", key, fakeSetAMint)

	srv := newBearerLog(t, "claude-fable-5-1", "claude-opus-5")
	l := probeLister(t, a.ModelCacheFrom([]string{"projA"}), srv)
	ids, err := l.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("catalog = %v, want the two ids the endpoint served", ids)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.bearers) != 1 {
		t.Fatalf("requests = %d, want 1", len(srv.bearers))
	}
	// The VALUE, byte for byte, asserted inside the record and reported as
	// a bool: the launch-listed set's mint and no other.
	if srv.bearers[0] != "Bearer "+fakeSetAMint {
		t.Error("the Authorization header did not carry the launch-listed set's mint exactly")
	}
}

// And the value is the LAST assignment across the launch's list, in launch
// order — the rule readStamps already gives a launch within one file,
// extended across the files one launch reads (ADR 0039 D3d as amended; V8
// MEASURED the same rule at the pane, ranger-base-abgil).
//
// Two arms with the same pair reversed, because one arm of a positional
// claim measures nothing: a "last wins" that is really "set B wins" or
// "the alphabetically later name wins" passes the first arm and fails here.
func TestTheLastAssignmentAcrossTheLaunchListIsWhatIsPresented(t *testing.T) {
	for _, arm := range []struct {
		name string
		sets []string
		want string
	}{
		{"A then B", []string{"projA", "projB"}, fakeSetBMint},
		{"B then A", []string{"projB", "projA"}, fakeSetAMint},
	} {
		t.Run(arm.name, func(t *testing.T) {
			a, key := launchHome(t)
			sessionSet(t, a, "projA", key, fakeSetAMint)
			sessionSet(t, a, "projB", key, fakeSetBMint)

			srv := newBearerLog(t, "claude-fable-5-1")
			l := probeLister(t, a.ModelCacheFrom(arm.sets), srv)
			if _, err := l.List(); err != nil {
				t.Fatal(err)
			}
			srv.mu.Lock()
			defer srv.mu.Unlock()
			if len(srv.bearers) != 1 {
				t.Fatalf("requests = %d, want 1", len(srv.bearers))
			}
			if srv.bearers[0] != "Bearer "+arm.want {
				t.Error("the Authorization header did not carry the LAST listed set's mint")
			}
		})
	}
}

// Absence, over the launch's list: the sets this launch realizes exist and
// are read, and none of them carries the name. No request is spent on a
// credential there is none of, the meter store answers, and the endpoint is
// asked exactly once.
//
// This is also the pin that the process ENVIRONMENT is not consulted: it is
// holding a value under this exact name for the whole arm, and the answer is
// still "no session credential".
func TestTheNameInNoLaunchListedSetSpendsNoRequest(t *testing.T) {
	a, _ := launchHome(t)
	sessionSet(t, a, "projA", "SOMETHING_ELSE", "x")

	srv := newBearerLog(t, "claude-fable-5-1")
	l := probeLister(t, a.ModelCacheFrom([]string{"projA"}), srv)
	if _, err := l.List(); err != nil {
		t.Fatal(err)
	}
	if got := srv.presented(); len(got) != 1 || got[0] != "meter" {
		t.Errorf("credentials presented = %v, want [meter]", got)
	}
}

// A launch that realizes NO env set gets no session credential — not the
// cockpit's. An env set is an explicit choice and never a silent default
// (rangerhq-f2b), and a probe that fell back to `default_env` here would be
// reading one persona's launch with another account's mint while every
// listing said the persona names none.
func TestALaunchThatRealizesNoSetDoesNotBorrowTheCockpitMint(t *testing.T) {
	a, _ := launchHome(t)

	srv := newBearerLog(t, "claude-fable-5-1")
	l := probeLister(t, a.ModelCacheFrom(nil), srv)
	if _, err := l.List(); err != nil {
		t.Fatal(err)
	}
	if got := srv.presented(); len(got) != 1 || got[0] != "meter" {
		t.Errorf("credentials presented = %v, want [meter] — the cockpit's default_env is not this launch's", got)
	}
}

// Refusal, over the launch's list: the launch's mint was there, it was
// presented, the endpoint said 401 — and the meter store is presented once
// more, for that catalog read and no more (rangerhq-tdy8's bound, per READ).
func TestARefusedLaunchMintFallsToTheMeterStoreOncePerRead(t *testing.T) {
	a, key := launchHome(t)
	sessionSet(t, a, "projA", key, fakeSetAMint)

	srv := newBearerLog(t, "claude-fable-5-1")
	srv.refuse = 1
	l := probeLister(t, a.ModelCacheFrom([]string{"projA"}), srv)
	ids, err := l.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Errorf("catalog = %v, want the id the endpoint served the second credential", ids)
	}
	if got := srv.presented(); len(got) != 2 || got[0] != "session" || got[1] != "meter" {
		t.Errorf("credentials presented = %v, want [session meter]", got)
	}
}

// The reading carries the list to the probe: ReadCatalogFrom hands its sets
// to the ModelCache it builds, which is the hop between "the launch named
// its sets" and "the seam read them".
//
// Two-way, and that is the point: the launch's reading presents the launch's
// mint and the cockpit's reading presents the cockpit's, from one home that
// holds both. A pin that only asserted the first would stay green over a
// ReadCatalogFrom that ignored its argument.
func TestTheReadingCarriesItsListDownToTheProbe(t *testing.T) {
	for _, arm := range []struct {
		name string
		cat  func(*App) *ModelCatalog
		want string
	}{
		{"the launch's list", func(a *App) *ModelCatalog {
			return a.ReadCatalogFrom([]string{"projA"}, nil)
		}, fakeSetAMint},
		{"the persona-less list", func(a *App) *ModelCatalog {
			return a.ReadCatalog(nil)
		}, fakeCockpitMint},
	} {
		t.Run(arm.name, func(t *testing.T) {
			a, key := launchHome(t)
			sessionSet(t, a, "projA", key, fakeSetAMint)

			srv := newBearerLog(t, "claude-fable-5-1")
			l := probeLister(t, arm.cat(a).cache(), srv)
			if _, err := l.List(); err != nil {
				t.Fatal(err)
			}
			srv.mu.Lock()
			defer srv.mu.Unlock()
			if len(srv.bearers) != 1 {
				t.Fatalf("requests = %d, want 1", len(srv.bearers))
			}
			if srv.bearers[0] != "Bearer "+arm.want {
				t.Error("the reading presented the other list's mint")
			}
		})
	}
}

// ─── the two hops a running endpoint cannot see ─────────────────────────────

// The hops above end at ReadCatalogFrom. The two above THAT — planLaunch
// handing the preflight the list it computed, and TierPreflightFrom handing
// that list to the reading — cannot be pinned by driving the probe: the
// lister those paths build points at the compiled-in endpoint, and pointing
// it anywhere else means injecting a lister, which is the one thing the App
// path may not rewire (TestInjectedListerIsNotRewired). So they are asked of
// the SOURCE, the way planmeteronce_qa_test.go and the qib censuses do.
//
// Mode 0: comments are dropped, so the paragraph in planLaunch that names
// LaunchEnvSets by name cannot be counted as a call to it.

// launchFunc parses one function out of one file.
func launchFunc(t *testing.T, path, name string) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return fd
		}
	}
	t.Fatalf("%s: no func %s — this pin names the shape it guards, so a rename is a decision to make here too", path, name)
	return nil
}

// callsTo finds every call to a.<sel>(...) inside n, in source order. It
// takes a Node rather than a FuncDecl so a loop body is as askable as a
// function is.
func callsTo(where ast.Node, sel string) []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(where, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == sel {
			out = append(out, c)
		}
		return true
	})
	return out
}

// firstArgIdent is the name of a call's first argument when it is a bare
// identifier, and "" when it is anything else — a fresh call, a literal, a
// nil. "Anything else" is the failure this pin exists to catch: a second
// spelling of the list is a second rule, and the two would drift silently.
func firstArgIdent(c *ast.CallExpr) string {
	if len(c.Args) == 0 {
		return ""
	}
	id, ok := c.Args[0].(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}

// planLaunch computes the launch's env set list ONCE and hands that same
// list to both readers: the availability preflight, which reads the catalog
// with the credential those sets carry, and the `vars` loop, which exports
// their values. One variable, two readers — the probe cannot be reading one
// account's mint while the launch exports another's.
func TestPlanLaunchHandsOneEnvSetListToBothReaders(t *testing.T) {
	fn := launchFunc(t, "herdrback.go", "planLaunch")

	made := callsTo(fn, "LaunchEnvSets")
	if len(made) != 1 {
		t.Fatalf("planLaunch calls LaunchEnvSets %d times, want exactly 1 — a second call is a second list", len(made))
	}
	// The one call is assigned to a name, and that name is what flows on.
	var list string
	ast.Inspect(fn, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 || as.Rhs[0] != ast.Expr(made[0]) || len(as.Lhs) != 1 {
			return true
		}
		if id, ok := as.Lhs[0].(*ast.Ident); ok {
			list = id.Name
		}
		return true
	})
	if list == "" {
		t.Fatal("planLaunch does not bind LaunchEnvSets's answer to a name, so nothing can be handed to two readers")
	}

	pf := callsTo(fn, "TierPreflightFrom")
	if len(pf) != 1 {
		t.Fatalf("planLaunch calls TierPreflightFrom %d times, want exactly 1", len(pf))
	}
	if got := firstArgIdent(pf[0]); got != list {
		t.Errorf("the preflight is handed %q, want the launch's own list %q (ADR 0039 D3d as amended)", got, list)
	}
	if made[0].Pos() > pf[0].Pos() {
		t.Error("the list is computed BELOW the preflight, so the preflight cannot have been handed it")
	}

	// And the same name is what the vars loop ranges over, which is where
	// the values this launch exports come from.
	ranged := false
	ast.Inspect(fn, func(n ast.Node) bool {
		rs, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if id, ok := rs.X.(*ast.Ident); ok && id.Name == list && len(callsTo(rs.Body, "EnvSetVars")) == 1 {
			ranged = true
		}
		return true
	})
	if !ranged {
		t.Errorf("the vars loop does not read %q — the launch exports a list the probe never saw", list)
	}
}

// TierPreflightFrom hands its caller's list to the reading rather than
// resolving one of its own. Three lines, and the hop a fake endpoint cannot
// see, so it is asked here.
func TestTierPreflightFromHandsItsListToTheReading(t *testing.T) {
	fn := launchFunc(t, "modelavail.go", "TierPreflightFrom")
	if len(fn.Type.Params.List) == 0 || len(fn.Type.Params.List[0].Names) != 1 {
		t.Fatal("TierPreflightFrom's first parameter is not one named list")
	}
	param := fn.Type.Params.List[0].Names[0].Name

	read := callsTo(fn, "ReadCatalogFrom")
	if len(read) != 1 {
		t.Fatalf("TierPreflightFrom calls ReadCatalogFrom %d times, want exactly 1", len(read))
	}
	if got := firstArgIdent(read[0]); got != param {
		t.Errorf("the reading is taken over %q, want the caller's list %q", got, param)
	}
	if len(callsTo(fn, "cockpitEnvSets")) != 0 {
		t.Error("TierPreflightFrom resolves a list of its own — a launch's preflight must read the launch's sets")
	}
}

// The control for the two pins above: the parser really can tell these
// shapes apart, so a green above is a reading of the source and not of an
// empty set. Both arms are of THIS file's own fixtures — a call whose first
// argument is a bare name, and one whose first argument is a fresh call.
func TestTheSourcePinsCanComeOutTheOtherWay(t *testing.T) {
	fn := launchFunc(t, "modelavail.go", "ReadCatalog")
	made := callsTo(fn, "ReadCatalogFrom")
	if len(made) != 1 {
		t.Fatalf("ReadCatalog calls ReadCatalogFrom %d times, want 1", len(made))
	}
	if got := firstArgIdent(made[0]); got != "" {
		t.Errorf("firstArgIdent = %q for a call whose first argument is a call, want \"\"", got)
	}
	if len(callsTo(fn, "cockpitEnvSets")) != 1 {
		t.Error("ReadCatalog does not resolve the persona-less list — then no caller does")
	}
	if len(callsTo(fn, "LaunchEnvSets")) != 0 {
		t.Error("callsTo() counts a name it was not asked for")
	}
}

// A 403 over the launch's list is the same class and the same move, and the
// arm is here rather than assumed from the cockpit half's: it is the
// fall-through that spends a second request, and this file's whole subject
// is which credential goes into the first one.
func TestAForbiddenLaunchMintAlsoFallsThrough(t *testing.T) {
	a, key := launchHome(t)
	sessionSet(t, a, "projA", key, fakeSetAMint)

	srv := newBearerLog(t, "claude-fable-5-1")
	srv.refuse, srv.code = 1, http.StatusForbidden
	l := probeLister(t, a.ModelCacheFrom([]string{"projA"}), srv)
	if _, err := l.List(); err != nil {
		t.Fatal(err)
	}
	if got := srv.presented(); len(got) != 2 || got[0] != "session" || got[1] != "meter" {
		t.Errorf("credentials presented = %v, want [session meter]", got)
	}
}
