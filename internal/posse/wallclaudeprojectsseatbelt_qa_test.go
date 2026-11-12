//go:build !posse_arm2 && !posse_arm3

package posse

// Render pins for ranger-base-r2uxm: the seatbelt half of ranger-base-ajkqx.
// claude's projects/ is write-denied in a caged claude seat's profile, the
// seat's own key dir is write-allowed again after that, and its memory/ is
// write-denied again after that — all of it between the allow block and the
// carve-out, so the trailing deny still has the last word.
//
// From any host: these read the rendered SBPL text. Whether sandbox-exec
// honours the order (last match wins) is the live darwin half, which is the
// verify lane's (ADR 0090 D2's split).
//
//	go test ./internal/posse -run 'WallClaudeProjectsSeatbelt' -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sbRender is the profile RenderSeatbelt would write for this cwd and
// runtime, with the carve it was built from.
func sbRender(t *testing.T, f cpFixture, rt *Runtime, cwd string) (string, SeatbeltCarveOut) {
	t.Helper()
	stateDirs := SeatbeltStateDirs(rt)
	w := f.a.SeatbeltWritable(f.ag, cwd, f.gates, stateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, cwd, f.gates, w, stateDirs...)
	return SeatbeltProfile(f.ag.Name, w, SeatbeltSiblings(stateDirs, c), c, sessionRefDirsFor(f.ag, cwd)...), c
}

// sbLine is the index of the first line of prof equal to want, or -1.
func sbLine(prof, want string) int {
	for i, l := range strings.Split(prof, "\n") {
		if strings.TrimSpace(l) == want {
			return i
		}
	}
	return -1
}

func TestQAWallClaudeProjectsSeatbeltRendersOwnKeyInsideADeniedProjects(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := cpFixtureIn(t, t.TempDir())
	cfg := absResolve(f.cfg)
	projects := filepath.Join(cfg, "projects")
	own := filepath.Join(projects, claudeProjectDir(absResolve(f.dir)))
	mem := filepath.Join(own, "memory")
	denyProjects := "(deny file-write* (subpath " + sbQuote(projects) + "))"
	allowOwn := "(allow file-write* (subpath " + sbQuote(own) + "))"
	denyMem := "(deny file-write* (subpath " + sbQuote(mem) + "))"

	prof, c := sbRender(t, f, f.rt, f.dir)
	grant := sbLine(prof, "(subpath "+sbQuote(cfg)+")")
	allowEnd := -1
	if grant >= 0 {
		for i, l := range strings.Split(prof, "\n") {
			if i > grant && l == ")" {
				allowEnd = i
				break
			}
		}
	}
	if grant < 0 || allowEnd < 0 {
		t.Fatalf("the fixture's config dir is not in the allow block, so nothing here is measured:\n%s", prof)
	}
	dp, ao, dm := sbLine(prof, denyProjects), sbLine(prof, allowOwn), sbLine(prof, denyMem)
	carve := sbLine(prof, "(deny file-write*")
	switch {
	case dp < 0 || ao < 0 || dm < 0:
		t.Errorf("missing a projects rule (ranger-base-r2uxm): deny projects at %d, allow own at %d, deny own memory at %d\n%s", dp, ao, dm, prof)
	case !(allowEnd < dp && dp < ao && ao < dm):
		t.Errorf("projects rules out of order: allow block ends %d, deny projects %d, allow own %d, deny memory %d — last match wins, so each must follow the one it narrows\n%s", allowEnd, dp, ao, dm, prof)
	case carve >= 0 && carve < dm:
		t.Errorf("the carve-out's deny (line %d) renders before the projects rules end (%d), so a reopen there could outvote a trailing deny (ADR 0001)\n%s", carve, dm, prof)
	}
	if carve < 0 {
		t.Errorf("the fixture rendered no carve-out, so the ordering against it is not measured\n%s", prof)
	}

	// bwrap spells the same list as binds, so its carve carries no SBPL rules.
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Carve.Narrow) != 0 {
		t.Errorf("the bwrap render carries seatbelt Narrow rules: %v", r.Carve.Narrow)
	}
	// ...and names the same dirs, in the same order and polarity.
	var binds []SeatbeltNarrow
	for _, pb := range r.Carve.Private {
		if underDir(projects, pb.Dst) {
			binds = append(binds, SeatbeltNarrow{Path: pb.Dst, Allow: !pb.ReadOnly})
		}
	}
	if len(binds) != len(c.Narrow) {
		t.Errorf("seatbelt narrows %v, bwrap binds %v", c.Narrow, binds)
	} else {
		for i := range binds {
			if binds[i] != c.Narrow[i] {
				t.Errorf("rule %d: seatbelt %v, bwrap %v", i, c.Narrow[i], binds[i])
			}
		}
	}

	for name, cwd := range map[string]string{
		"the main checkout": f.main,
		"a plain dir":       filepath.Join(f.home, "plain"),
		"under the tree":    filepath.Join(f.dir, "sub"),
	} {
		if err := os.MkdirAll(cwd, 0o755); err != nil {
			t.Fatal(err)
		}
		p, _ := sbRender(t, f, f.rt, cwd)
		if sbLine(p, denyProjects) < 0 {
			t.Errorf("%s: projects/ is not write-denied\n%s", name, p)
		}
		for _, l := range strings.Split(p, "\n") {
			if strings.HasPrefix(l, "(allow file-write* (subpath "+sbQuote(projects)[:len(sbQuote(projects))-1]) {
				t.Errorf("%s: a write allow into projects/ (%s), for a seat that has no own key", name, l)
			}
		}
	}

	other := &Runtime{Name: "codex", StateDirs: []string{"~/.codex"}}
	if err := os.MkdirAll(filepath.Join(f.home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, oc := sbRender(t, f, other, f.dir); len(oc.Narrow) != 0 {
		t.Errorf("a codex seat gets claude projects rules, which nothing granted it: %v", oc.Narrow)
	}
}

// A link at projects/, the own key or its memory/ drops the reopen: projects/
// is denied whole (and, for a link at projects/ itself, at the spelling the
// profile matches too). memory/ linked back into the own key would otherwise
// be the memory claude reads, writable under another name.
func TestQAWallClaudeProjectsSeatbeltALinkDropsTheReopen(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "cfg")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ClaudeConfigDirVar, cfg)
	binds := claudeProjectsBinds(d, []string{cfg}, filepath.Join(d, "tree"))
	if len(binds) != 3 {
		t.Fatalf("binds = %v, want projects, own, memory", binds)
	}
	projects, own, mem := binds[0].Dst, binds[1].Dst, binds[2].Dst
	want := []SeatbeltNarrow{{Path: projects}, {Path: own, Allow: true}, {Path: mem}}
	if got := seatbeltProjectsNarrow(binds); !narrowEq(got, want) {
		t.Fatalf("absent dirs: %v, want %v", got, want)
	}
	if err := os.MkdirAll(own, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(own, "mem2"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ link, to string }{
		{mem, filepath.Join(own, "mem2")},
		{own, filepath.Join(d, "elsewhere")},
	} {
		if c.link == own {
			if err := os.RemoveAll(own); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(c.to, c.link); err != nil {
			t.Fatal(err)
		}
		if got := seatbeltProjectsNarrow(binds); !narrowEq(got, []SeatbeltNarrow{{Path: projects}}) {
			t.Errorf("a link at %s: %v, want projects denied and nothing reopened", c.link, got)
		}
		if err := os.Remove(c.link); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(c.link, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// projects/ itself a link: both spellings denied, nothing reopened.
	if err := os.RemoveAll(projects); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cfg, "elsewhere")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, projects); err != nil {
		t.Fatal(err)
	}
	if got := seatbeltProjectsNarrow(binds); !narrowEq(got, []SeatbeltNarrow{{Path: projects}, {Path: absResolve(target)}}) {
		t.Errorf("projects/ a link: %v, want both spellings denied and nothing reopened", got)
	}
}

func narrowEq(a, b []SeatbeltNarrow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
