package treepins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// gitOutsideGates is the git a fixture builds its scratch repo with: the
// first one on PATH that is not in any posse gates dir. A fixture that sets
// -c core.hooksPath= to keep the box's hooks out of its repo is exactly what
// the seat's L1 shim refuses (hookskip:config-override, ranger-base-r3pri),
// and exec.Command resolves a bare name against THIS process's PATH — the
// seat's, gates first — whatever cmd.Env says. Skips when there is none.
func gitOutsideGates(t *testing.T) string {
	t.Helper()
	for _, dir := range filepath.SplitList(posse.PathOutsideGates("")) {
		p := filepath.Join(dir, "git")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	t.Skip("no git outside the gates")
	return ""
}
