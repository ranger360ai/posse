package posse

// QA pin for ranger-base-6vvgs: the go caches must not move with the temp
// HOME this binary's TestMain installs.
//
// TestMain replaces HOME with a temp dir for the whole binary (ADR 0047 D1,
// ranger-base-aupee). GOMODCACHE defaults to $GOPATH/pkg/mod and so to
// $HOME/go/pkg/mod, and GOCACHE to the user cache dir, also under $HOME — so
// without the pin TestMain now sets, both move into the temp home, and the
// two `go build` shellouts in this package (buildBdWritePosse,
// cagestale_test.go's cagePosseBuildArgv call) build cmd/posse against an
// empty module cache. That means fetching golang.org/x/{sys,term,text} from
// proxy.golang.org on every run of this binary, which turned one runner's
// resolver failure into a red on main — run 41811904569, macos-latest, arm 1,
// `lookup proxy.golang.org: no such host`, 49.44s in. ci.yml is the only gate
// a commit on main ever passes, so what that cost was attribution.
//
// This is the cheap guard the expensive symptom did not have: it reads env,
// never the network, and it reds in milliseconds where the regression it
// catches reds after a 49s build on one platform, in one arm, and only when
// DNS blinks.
//
// It owes the Makefile no door: its subject is this binary's environment,
// not the tree.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAGoCachesDoNotFollowTheTempHome(t *testing.T) {
	t.Parallel()
	// The pin is TestMain reading `go env`, so a box without `go` is a box
	// where it cannot have run — the precondition, stated rather than
	// assumed.
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH")
	}
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("no HOME to compare against")
	}
	for _, key := range []string{"GOMODCACHE", "GOCACHE"} {
		v := os.Getenv(key)
		if v == "" {
			t.Errorf("%s is unset, so `go` resolves it under this binary's temp HOME and every go-build shellout here fetches cmd/posse's modules from the network (ranger-base-6vvgs)", key)
			continue
		}
		rel, err := filepath.Rel(home, v)
		if err != nil {
			continue // different volumes: not under it, which is the point
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Errorf("%s=%q is inside the temp HOME %q, so it is empty and every go-build shellout here fetches cmd/posse's modules from the network (ranger-base-6vvgs)", key, v, home)
		}
	}
}
