package posse

// Every site that re-execs the test binary to run a NAMED test must hand the
// child an environment with RHQ_FAKE_HERDR taken out of it.
//
// TestMain dispatches on that variable BEFORE m.Run(): with it set, the
// binary is the fake herdr/bd substrate and exits on its own argv, so a child
// that inherits it never runs the test it was asked for. The parent then
// waits on a pipe that closes immediately and reports whatever its own
// deadline says, which reads as anything except "the child was answered by
// the fake".
//
// It is latent today — the variable is set per test by newTestBackend, and Go
// runs the parallel batch after the serial one, so no re-exec inherits it.
// It stops being latent the moment the variable moves to TestMain, which is
// what docs/notes.d/ranger-base-i7fa.md section 6 costs out and what
// ranger-base-aupee is to build: process-wide is exactly what a child
// inherits. Four of the five sites already carry the filter; the fifth did
// not, and the reason no census caught it is that it spells the binary
// os.Args[0] where the others spell it exe (ranger-base-kcfhr,
// ranger-base-cecvu).
//
// So this pin is a scanner over the package's own test sources rather than a
// list: a sixth site added later is covered by it, and by nothing else.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A re-exec of the test binary, and whether its child's environment is built
// with the fake-substrate variable taken out.
type reexecSite struct {
	File    string
	Line    int
	Filters bool
}

// reexecSiteRe finds a re-exec of THIS binary aimed at a named test. Both
// spellings of "this binary" are here on purpose: os.Args[0] is the one the
// missed site used, and a regex that knows only exe is the census that missed
// it.
var reexecSiteRe = regexp.MustCompile(`exec\.Command\(\s*(?:exe|os\.Args\[0\])\s*,\s*"-test\.run=`)

// scanReExecSites reports every re-exec site in the *_test.go files under
// root. It takes a root so the same function grades this package and a
// hand-typed fixture — the fixture is what shows it can fail, and the live
// pass is then only about the wiring.
//
// A site's environment is whatever is written between the exec.Command and
// the call that starts the child; 800 bytes is the fallback bound for a shape
// that starts it somewhere this cannot see, and a site whose window closes
// early reads as unfiltered, which is the safe direction.
func scanReExecSites(root string) ([]reexecSite, error) {
	names, err := filepath.Glob(filepath.Join(root, "*_test.go"))
	if err != nil {
		return nil, err
	}
	var sites []reexecSite
	for _, name := range names {
		body, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		src := string(body)
		for _, m := range reexecSiteRe.FindAllStringIndex(src, -1) {
			end := len(src)
			if m[1]+800 < end {
				end = m[1] + 800
			}
			window := src[m[0]:end]
			for _, starter := range []string{".Start()", ".CombinedOutput()", ".Output()", ".Run()"} {
				if i := strings.Index(window, starter); i >= 0 {
					window = window[:i]
				}
			}
			sites = append(sites, reexecSite{
				File:    filepath.Base(name),
				Line:    1 + strings.Count(src[:m[0]], "\n"),
				Filters: strings.Contains(window, "qaSeederEnv(") || strings.Contains(window, "RHQ_FAKE_HERDR="),
			})
		}
	}
	return sites, nil
}

func TestQAEveryTestBinaryReExecKeepsTheChildATestBinary(t *testing.T) {
	t.Parallel()

	// Shown able to fail first, over sources typed here rather than derived
	// from the package: a rig whose fixtures come out of the thing under test
	// makes every mutant of that thing equivalent. The literals are broken up
	// so this file is not itself a site the live sweep below has to pardon.
	t.Run("the scanner names an unfiltered site", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		good := "package p\n\nfunc a() {\n\tchild := exec.Command(exe, " +
			"\"-test.run=^TestChild$\", \"-test.v\")\n" +
			"\tvar env []string\n\tfor _, kv := range os.Environ() {\n" +
			"\t\tif !strings.HasPrefix(kv, \"RHQ_FAKE_HERDR=\") {\n\t\t\tenv = append(env, kv)\n\t\t}\n\t}\n" +
			"\tchild.Env = env\n\tchild.Start()\n}\n"
		bad := "package p\n\nfunc b() {\n\tchild := exec.Command(os.Args[0], " +
			"\"-test.run=^TestChild$\", \"-test.v\")\n" +
			"\tchild.Env = append(os.Environ(), \"X=1\")\n\tchild.Start()\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "good_test.go"), []byte(good), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bad_test.go"), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		sites, err := scanReExecSites(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(sites) != 2 {
			t.Fatalf("the scanner found %d sites in a fixture holding one of each: %+v", len(sites), sites)
		}
		byFile := map[string]bool{}
		for _, s := range sites {
			byFile[s.File] = s.Filters
		}
		if !byFile["good_test.go"] {
			t.Error("a site that strips the variable was read as unfiltered — the scan cannot pass anything")
		}
		if byFile["bad_test.go"] {
			t.Error("a site that hands the child a bare os.Environ() was read as filtered — the scan cannot fail anything")
		}
	})

	sites, err := scanReExecSites(".")
	if err != nil {
		t.Fatal(err)
	}
	// A floor, not a count: the point of a scanner is that a site added later
	// is covered, and the point of the floor is that a sweep matching nothing
	// fails instead of passing at zero.
	if len(sites) < 5 {
		t.Fatalf("found %d re-exec sites in this package; the sweep is not reading the files it thinks it is", len(sites))
	}
	for _, s := range sites {
		if !s.Filters {
			t.Errorf("%s:%d re-execs the test binary without taking RHQ_FAKE_HERDR out of the child's environment: "+
				"once that variable is process-wide, TestMain answers this child as the fake substrate and the named test never runs",
				s.File, s.Line)
		}
	}
	t.Logf("checked %d re-exec sites", len(sites))
}
