//go:build posse_arm2

package posse

// QA pins for the expiry surfaces' hostile inputs, from the verify pass on
// ranger-base-k6ha (ranger-base-5gnb).
//
// credexpiry_test.go pins the rules. These pin the shapes an env set can
// actually be in — hand-edited files, stamps that are not dates, a stamp
// that belongs to the line above the one it looks like it belongs to — and
// the two edges (the window's own boundary and expiryIn's) the rules are
// expressed in.

import (
	"strings"
	"testing"
	"time"
)

// ─── ranger-base-bb6i ───────────────────────────────────────────────────────

// readStamps returns the stamps off the LAST line that assigns the key.
// parseEnvLines keeps every assignment in file order and the last is what a
// launch ends up exporting, so dating the first would be a confident wrong
// date over a live mint — the one direction ADR 0019 D5's design says it
// will never err in ("strictly worse than cannot tell").
func TestQAExpiryDatesTheAssignmentALaunchActuallyExports(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))
	writeEnvFile(t, a, "container",
		expiresStamp+day(90)+"\n"+key+"=stale-mint\n"+
			expiresStamp+day(2)+"\n"+key+"=the-one-a-launch-exports\n", 0o600)

	// The witness: the value a launch gets IS the second one, so the stamp
	// that matters is the second stamp.
	vars, err := a.EnvSetVars("container")
	if err != nil {
		t.Fatal(err)
	}
	last := ""
	for _, v := range vars {
		if v.Key == key {
			last = v.Value
		}
	}
	if last != "the-one-a-launch-exports" {
		t.Fatalf("the fixture does not shadow: a launch would export %q", last)
	}

	ex := a.ExpiringCredentials(expiryNow)
	if len(ex) == 0 {
		t.Fatalf("a mint dying in 2d raises no warning because an older line above it is stamped for 90d")
	}
	if got := ex[0].At.Format(stampDate); got != day(2) {
		t.Errorf("the warning is dated %s, want %s — the date belongs to a value nobody exports", got, day(2))
	}
}

// ─── the shapes that DO hold ────────────────────────────────────────────────

// Every row measured 2026-08-30 at posse 25503c1. The property is not "the
// date is printed": it is that a stamp posse cannot tie to the exported
// value never becomes a warning, and that the ones it can, do.
func TestQAExpiryStampShapes(t *testing.T) {
	key := "CLAUDE_CODE_OAUTH_TOKEN"
	const val = "sk-ant-oat01-MINT"
	for _, c := range []struct {
		name  string
		body  string
		warns bool
		why   string
	}{
		{"a blank line between the stamp and the assignment",
			expiresStamp + day(3) + "\n\n" + key + "=" + val + "\n", false,
			"readStamps documents 'immediately above' as the whole rule"},
		{"the stamp belongs to the variable above ours",
			expiresStamp + day(3) + "\nOTHER=x\n" + key + "=" + val + "\n", false,
			"a stamp read off the wrong variable is a date about a credential it is not true of"},
		{"an empty stamp value",
			expiresStamp + "\n" + key + "=" + val + "\n", false, "not a date"},
		{"a stamp with trailing prose",
			expiresStamp + day(3) + " (approx)\n" + key + "=" + val + "\n", false, "not a date"},
		{"a stamp carrying a time as well",
			expiresStamp + day(3) + "T00:00:00Z\n" + key + "=" + val + "\n", false,
			"stampDate is a date layout; a longer string is not a partial match"},
		{"an exported assignment",
			expiresStamp + day(3) + "\nexport " + key + "=" + val + "\n", true,
			"assignsKey tolerates the export prefix exactly as parseEnvLines does"},
		{"an indented assignment",
			expiresStamp + day(3) + "\n  " + key + "=" + val + "\n", false,
			"parseEnvLines does not trim the leading run either, so no launch exports it under this name"},
		{"a far-future stamp", expiresStamp + "9999-12-31\n" + key + "=" + val + "\n", false, "outside the window"},
		{"the zero date, spelled out",
			expiresStamp + "0001-01-01\n" + key + "=" + val + "\n", false,
			"time's zero value IS 0001-01-01, so a stamp of it is indistinguishable from no stamp — and cannot-tell is the fail-safe answer"},
		{"CRLF line endings",
			expiresStamp + day(3) + "\r\n" + key + "=" + val + "\r\n", true,
			"the CR is trimmed off the stamp and lands in the value, so the variable still assigns"},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := refreshApp(t)
			mustRuntime(t, a, "claude")
			writeEnvFile(t, a, "container", c.body, 0o600)
			if got := len(a.ExpiringCredentials(expiryNow)) > 0; got != c.warns {
				t.Errorf("warns = %v, want %v (%s)\n%s", got, c.warns, c.why, c.body)
			}
		})
	}
}

// The window's edge to the second. The rule is `> RefreshExpiryWindow`, and
// which side of it exactly a fortnight falls on is the rule and not a
// rendering — the shipped pin measures 13d and 15d, which both sides of a
// moved boundary satisfy.
func TestQAExpiryWindowEdgeToTheSecond(t *testing.T) {
	for _, c := range []struct {
		left  time.Duration
		warns bool
	}{
		{RefreshExpiryWindow - time.Second, true},
		{RefreshExpiryWindow, true},
		{RefreshExpiryWindow + time.Second, false},
	} {
		a := refreshApp(t)
		key := CageCredential(mustRuntime(t, a, "claude"))
		// A stamp is a DATE, so the clock moves instead: same arithmetic,
		// expressible to the second.
		stamp, err := time.Parse(stampDate, expiryNow.Format(stampDate))
		if err != nil {
			t.Fatal(err)
		}
		writeEnvFile(t, a, "container", expiresStamp+stamp.Format(stampDate)+"\n"+key+"=v\n", 0o600)
		if got := len(a.ExpiringCredentials(stamp.Add(-c.left))) > 0; got != c.warns {
			t.Errorf("%v before expiry: warns = %v, want %v", c.left, got, c.warns)
		}
	}
}

// expiryIn's own edges. It truncates rather than rounds, and it switches
// units at exactly two days.
func TestQAExpiryInEdges(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want string }{
		{"48h", "in 2d"},
		{"47h59m", "in 47h"},
		{"1s", "in 0h"},
		{"0s", "EXPIRED"},
		{"-1s", "EXPIRED"},
		{"2400h", "in 100d"},
	} {
		d, err := time.ParseDuration(c.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := expiryIn(d); got != c.want {
			t.Errorf("expiryIn(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The unattended surface's shape, through the function Dispatcher.Run calls:
// silent when there is nothing, and when there is, ONE line naming the
// soonest and counting the rest — with the sentence that says it actuates
// nothing still in it.
func TestQACredentialExpiryPassLineShape(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))
	var errb strings.Builder
	d := &Dispatcher{App: a, Err: &errb, Now: func() time.Time { return expiryNow }}

	d.credentialExpiry()
	if errb.Len() != 0 {
		t.Errorf("a box with no stamped mint warns anyway: %q", errb.String())
	}

	writeEnvFile(t, a, "aaa", expiresStamp+day(9)+"\n"+key+"=v1\n", 0o600)
	writeEnvFile(t, a, "zzz", expiresStamp+day(1)+"\n"+key+"=v2\n", 0o600)
	errb.Reset()
	d.credentialExpiry()
	line := errb.String()
	if n := strings.Count(strings.TrimSuffix(line, "\n"), "\n"); n != 0 {
		t.Errorf("the pass line is %d lines, not one: %q", n+1, line)
	}
	// The soonest is the alphabetically LAST set on purpose: a directory
	// listing is already sorted, so a fixture where the two agree is green
	// with the sort deleted.
	if !strings.Contains(line, "zzz") {
		t.Errorf("the pass line does not name the soonest set: %q", line)
	}
	if !strings.Contains(line, "+1 more") {
		t.Errorf("the pass line does not count the rest: %q", line)
	}
	if !strings.Contains(line, "nothing parks on this") {
		t.Errorf("the pass line no longer says it actuates nothing: %q", line)
	}
}
