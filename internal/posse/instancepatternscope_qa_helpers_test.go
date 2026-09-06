package posse

// Helpers lifted out of instancepatternscope_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"testing"
)

// qaInstancePatternWall is the visibility wall with ONE instance pattern
// configured, in the shape of the one ADR 0048 D1 gives the operator: a
// class name and an ERE whose exception is the marker form. The name is a
// fixture's own ("zephyr"), never this box's, so what these pins measure is
// the mechanism and not the deployment.
const (
	qaInstanceClass = "pre-publication-name"
	qaInstanceName  = "zephyr"
	qaInstanceERE   = qaInstanceName + "([^-]|-[^0-9a-z]|-?$)"
	qaInstanceCfg   = OpsPatternsConfigKey + ":\n  " + qaInstanceClass + ": " + qaInstanceERE + "\n"
)

func qaInstanceWall(t *testing.T) *visWall {
	t.Helper()
	w := newVisWallCfg(t, "instance", qaInstanceCfg)
	// FIXTURE PREMISE: the pattern was ACCEPTED, not refused at stamp time.
	// A pin over a pattern the parser threw away is green against any wall
	// at all — the hook records refusals in a comment and guards nothing.
	set := (&App{ConfigPath: w.home + "/config.yaml"}).OpsPatternSet()
	if len(set.Rejected) > 0 || len(set.Extra) != 1 || set.Extra[0].Class != qaInstanceClass {
		t.Fatalf("fixture premise: the config pattern must be accepted, got extra=%+v rejected=%v", set.Extra, set.Rejected)
	}
	return w
}
