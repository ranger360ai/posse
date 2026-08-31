package posse

// ranger-base-2w9l: seatbeltReachRow reclassified a per-probe write denial
// by matching its OWN OUTPUT TEXT against the "sandbox_apply" token — the
// same token an apply refusal carries. A target whose path happens to
// contain that literal (an operator's `.beads`/`.git` under a directory
// named after it) made a genuine write denial read as an unmeasured apply
// refusal instead of the finding it is: seatbeltReachRow returned unmeasured
// rather than why, applyRecordReach put the row in Realized rather than
// through unrealized(), and the launch was not degraded by a store the
// profile really denies. This is the false-NEGATIVE direction of the
// classifier ranger-base-heur fixed in the false-positive direction.
//
// Pinned as a two-arm table over one fixture and one deny — only the path
// differs. "ordinary" is the control: identical deny, no token in the path,
// must produce a finding. "sandbox_apply" is the escape: same deny, and
// before the fix it produced none.

import (
	"path/filepath"
	"testing"
)

func TestQAWriteDenialOnASandboxApplyNamedPathIsAFinding(t *testing.T) {
	sbSkipUnlessSandboxable(t) // the fixture IS a sandbox-exec (ranger-base-xjw9)
	for _, name := range []string{"ordinary", "sandbox_apply"} {
		t.Run(name, func(t *testing.T) {
			// sandbox-exec matches real paths: a fixture root under a
			// symlinked temp dir denies nothing unless resolved first
			// (ranger-base-2w9l's own first attempt at this fixture).
			root, err := filepath.EvalSymlinks(sbRoot(t))
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, name, beadsDirName)
			sbMkdir(t, target)
			prof := filepath.Join(root, "p.sb")
			sbWrite(t, prof, "(version 1)\n(allow default)\n(deny file-write* (subpath "+sbQuote(root)+"))\n")

			// The fixture is the experiment: if the deny does not fire,
			// everything below is green for the wrong reason.
			if _, err := seatbeltReachProbe(prof, target); err == nil {
				t.Fatalf("fixture did not deny the write - this measures nothing")
			}

			why, unmeasured := seatbeltReachRow(prof, []string{target})
			if why == "" {
				t.Errorf("ESCAPE: a genuine write denial on %q produced no finding (unmeasured=%q)", target, unmeasured)
			}
			if unmeasured != "" {
				t.Errorf("a genuine write denial on %q classified as unmeasured: %q", target, unmeasured)
			}
		})
	}
}
