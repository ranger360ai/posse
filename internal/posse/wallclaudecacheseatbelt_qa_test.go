//go:build !posse_arm2 && !posse_arm3

package posse

// Render pin for ranger-base-mbcbv, the seatbelt half of ranger-base-7x3bj.
// TWO processes it protects, one per member, and neither of them is the seat
// whose profile this is:
//
//   - an UNCAGED claude's served-catalog read. <cfg>/cache/model-catalog
//     holds the served catalog (models, provider alias targets, per-model
//     thinking and effort defaults, state.model) and the persisted masked
//     ids every later claude hydrates at start. Neither file is signed and
//     claude checks the schema and the surface and nothing else, so a
//     planted copy is what the next claude on the box believes about models.
//   - the box's retention sweep. <cfg>/.last-cleanup is read for its MTIME
//     as a 24h throttle, and rewritten only after a sweep runs — so one
//     future mtime switches box-wide transcript retention off indefinitely.
//
// A caged claude seat's profile must deny READING and WRITING both names,
// the NAMES included: claude deletes a non-regular file at the marker and
// rewrites it, so unlink, create and a rename onto <cfg>/.last-cleanup have
// to be refused too, not only a write through it. Nothing later in the
// profile may reopen either name. A codex seat's profile names neither.
//
// COST, MEASURED 2026-11-21 on this Linux host, claude 2.1.330: a scratch
// config dir with <cfg>/cache and <cfg>/.last-cleanup at mode 0000 (EACCES,
// the refusal SBPL gives), one unbilled `claude -p
// --dangerously-skip-permissions` turn against a local fake Messages
// endpoint. The turn exits 0 with its answer, and the single refused read
// its debug log names — `[servedCatalog] mask file read failed: EACCES` — is
// logged and dropped. So the read half is affordable and the write-only
// fallback is not needed. A caged darwin seat refetches the served catalog
// and the changelog every start and caches neither, and reads the marker as
// absent, so it runs its own sweep against names it cannot write.
//
// ASSUMED, not measured, and it cannot be measured here: SBPL's refusal is
// handled the way that EACCES was (there is no darwin box in this instance,
// the same assumption ranger-base-yaha3 and ranger-base-wj93u carry), and a
// subpath deny behaves as it does for projects/ (ranger-base-r2uxm). The
// marker's own READER is not reached by a `-p` turn either — the debug log
// of the measured run names no sweep — so what the measurement settles is
// that the turn completes with the marker denied, not what an interactive
// session's sweep does with it.
//
// From any host the render arms read the SBPL text. The live arm applies the
// profile and runs only where sandbox-exec can (darwin); elsewhere it skips
// and says so.
//
//	go test ./internal/posse -run 'WallClaudeCacheSeatbelt' -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAWallClaudeCacheSeatbeltDeniesReadAndWriteForClaudeOnly(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := cpFixtureIn(t, t.TempDir())
	cfg := absResolve(f.cfg)
	// BY LITERAL, not through the constants: renaming claudeCacheDir or
	// claudeLastCleanupFile alone cannot satisfy this pin.
	cache := filepath.Join(cfg, "cache")
	marker := filepath.Join(cfg, ".last-cleanup")
	head := "(deny file-read* file-write*"

	prof, c := sbRender(t, f, f.rt, f.dir)
	if sbLine(prof, "(subpath "+sbQuote(cfg)+")") < 0 {
		t.Fatalf("the fixture's config dir is not granted, so nothing here is measured:\n%s", prof)
	}
	lines := strings.Split(prof, "\n")
	h := sbLine(prof, head)
	if h < 0 {
		t.Fatalf("no read-and-write deny block (ranger-base-mbcbv):\n%s", prof)
	}
	for _, p := range []string{cache, marker} {
		member := "(subpath " + sbQuote(p) + ")"
		in := false
		for i := h + 1; i < len(lines) && strings.TrimSpace(lines[i]) != ")"; i++ {
			if strings.TrimSpace(lines[i]) == member {
				in = true
			}
		}
		if !in {
			t.Errorf("%s is not in the read-and-write deny block at line %d, so a caged seat can plant what every other claude on the box reads back:\n%s", p, h, prof)
		}
		if !containsString(c.DenyAll, p) {
			t.Errorf("carve DenyAll = %v, want %s in it", c.DenyAll, p)
		}
	}
	// Last match wins: no allow may follow the block, or it could reopen it.
	for i := h; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "(allow") {
			t.Errorf("an allow at line %d follows the read-and-write deny at %d, so it could outvote it: %s\n%s", i, h, lines[i], prof)
		}
	}

	// bwrap spells cache/ as a private dir bind and .last-cleanup as a
	// private file bind (ranger-base-7x3bj), so its carve carries no SBPL
	// deny — a read deny there would hide the seat's own copies.
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Carve.DenyAll) != 0 {
		t.Errorf("the bwrap render carries seatbelt DenyAll: %v", r.Carve.DenyAll)
	}

	other := &Runtime{Name: "codex", StateDirs: []string{"~/.codex"}}
	if err := os.MkdirAll(filepath.Join(f.home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	op, oc := sbRender(t, f, other, f.dir)
	for _, p := range []string{cache, marker} {
		if sbLine(op, "(subpath "+sbQuote(p)+")") >= 0 {
			t.Errorf("a codex seat gets claude's %s deny, which nothing granted it:\n%s", p, op)
		}
	}
	if len(oc.DenyAll) != 0 {
		t.Errorf("a codex seat's DenyAll = %v, want none", oc.DenyAll)
	}
}

// A link at either name is denied at both spellings: SBPL matches the
// resolved path, so the link's own name alone would deny nothing.
func TestQAWallClaudeCacheSeatbeltALinkIsDeniedAtBothSpellings(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "cfg")
	elsewhere := filepath.Join(d, "elsewhere")
	for _, p := range []string{cfg, elsewhere} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(ClaudeConfigDirVar, cfg)
	rcfg := absResolve(cfg)
	cache := filepath.Join(rcfg, "cache")
	marker := filepath.Join(rcfg, ".last-cleanup")
	if got := claudeCacheAndCleanupMarker(d, []string{rcfg}); len(got) != 2 || got[0] != cache || got[1] != marker {
		t.Fatalf("no link: %v, want [%s %s]", got, cache, marker)
	}
	cacheT := filepath.Join(elsewhere, "c")
	markerT := filepath.Join(elsewhere, "m")
	if err := os.MkdirAll(cacheT, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerT, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cacheT, cache); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(markerT, marker); err != nil {
		t.Fatal(err)
	}
	got := claudeCacheAndCleanupMarker(d, []string{rcfg})
	want := []string{cache, absResolve(cacheT), marker, absResolve(markerT)}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("links at both names: %v, want %v", got, want)
	}
	if got := claudeCacheAndCleanupMarker(d, nil); got != nil {
		t.Errorf("no state grant reaches the config dir, yet %v", got)
	}
}

// The live half, where a profile can be applied: under the caged claude
// seat's rendered profile, every way of changing what another claude reads
// back from the catalog cache, and every way of moving the sweep marker's
// mtime, is refused — while a sibling in the same granted config dir stays
// writable, so the refusals are the deny's and not a missing grant.
func TestQAWallClaudeCacheSeatbeltLiveRefusesTheNamesAndTheirContents(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := cpFixtureIn(t, t.TempDir())
	cfg := absResolve(f.cfg)
	cache := filepath.Join(cfg, "cache")
	catalogs := filepath.Join(cache, "model-catalog")
	served := filepath.Join(catalogs, "acct-0123456789abcdef-cc.json")
	masked := filepath.Join(catalogs, "masked-ids.json")
	marker := filepath.Join(cfg, ".last-cleanup")
	if err := os.MkdirAll(catalogs, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, w := range []struct{ path, body string }{
		{served, "{\"models\":[{\"id\":\"the box's own catalog\"}]}\n"},
		{masked, "{\"version\":1,\"ids\":[]}\n"},
		{marker, "swept\n"},
	} {
		if err := os.WriteFile(w.path, []byte(w.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prof, _ := sbRender(t, f, f.rt, f.dir)
	pf := sbRenderProfile(t, "claude.sb", prof)
	planted := filepath.Join(cfg, "planted.json")
	q := shellQuote
	if !sbRun(t, pf, "printf x > "+q(planted)) {
		t.Fatalf("control: a sibling in the granted config dir is not writable, so no refusal below is measured")
	}
	for name, sh := range map[string]string{
		"rewrite the served catalog":   "printf x > " + q(served),
		"append to the served catalog": "printf x >> " + q(served),
		"read the served catalog":      "cat " + q(served),
		"add a catalog file":           "printf x > " + q(filepath.Join(catalogs, "other-cc.json")),
		"rewrite masked-ids":           "printf x > " + q(masked),
		"read masked-ids":              "cat " + q(masked),
		"relink model-catalog/":        "rm -rf " + q(catalogs) + " && ln -s /tmp " + q(catalogs),
		"relink cache/":                "rm -rf " + q(cache) + " && ln -s /tmp " + q(cache),
		"touch the marker forward":     "touch -d 2099-01-01 " + q(marker),
		"write the marker":             "printf x > " + q(marker),
		"read the marker":              "cat " + q(marker),
		"replace the marker by link":   "ln -sf " + q(planted) + " " + q(marker),
		"rename onto the marker":       "mv -f " + q(planted) + " " + q(marker),
		"unlink the marker":            "rm -f " + q(marker),
	} {
		if sbRun(t, pf, sh) {
			t.Errorf("%s was allowed under a caged claude seat's profile (ranger-base-mbcbv): %s", name, sh)
		}
	}
	// The host's copies are what the other processes read, so they are what
	// has to be unchanged.
	if b, err := os.ReadFile(served); err != nil || string(b) != "{\"models\":[{\"id\":\"the box's own catalog\"}]}\n" {
		t.Errorf("the box's served catalog is no longer its own: %q %v", b, err)
	}
	if b, err := os.ReadFile(masked); err != nil || string(b) != "{\"version\":1,\"ids\":[]}\n" {
		t.Errorf("the box's masked ids are no longer its own: %q %v", b, err)
	}
	fi, err := os.Lstat(marker)
	if err != nil || !fi.Mode().IsRegular() {
		t.Fatalf(".last-cleanup is no longer a regular file: %v %v", fi, err)
	}
	if fi.ModTime().Year() > 2098 {
		t.Errorf(".last-cleanup's mtime moved into the future (%s), so the box's sweep is off", fi.ModTime())
	}
}
