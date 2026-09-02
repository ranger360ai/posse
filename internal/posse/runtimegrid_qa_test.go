package posse

// ADR 0013 §1: `posse runtime check <name>` prints the six-stage grid, and
// §5: the account row's cap line is PARSED, never echoed. Both were true at
// ranger-base-tff's verification and neither was held there.
//
// The grid pin in runtimecheck_test.go asked `strings.Contains(out, "settle")`
// of a page of prose that says "settle" in the launch row's own text ("no
// detection here, so work/settle are guesses"), "settled state" in the work
// row, and "settle-without-record" in the record row. MEASURED on ranger-base-tff:
// five of the six stage labels can be renamed, and the settle row can be
// DELETED from the grid outright, with all three packages green. A stage is a
// ROW, so the assertion has to read rows.
//
// Same shape one stage down: only `uncounted_cap_<x>: unset` was asserted, so
// the armed and the malformed branches of the account row were unpinned —
// and the malformed one could be rewritten to echo a junk value back as
// "<junk> beads / rolling 7 days", the grid claiming a brake nothing arms,
// with the suite green. That is the silence §5 exists to remove.

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// gridStages is the stage labels the grid actually DREW, in order.
//
// stageRow.write and wrapGrid share one lead format — `"  %-11s %s"` — so a
// row is a line whose value starts at column 14 with a non-empty label in
// front of it. A continuation line has the same shape with an empty label.
// Reading the shape rather than the words is the whole point: it can tell a
// row from the same word used in a sentence, which is what the substring
// check could not do.
//
// A label is one word. The command echoed under the title is a bare
// two-space indent, and `mycli --sys {file}` happens to put a space at column
// 13 like a row does — measured, on the first run of this test — so the
// single-word rule is what separates them, not the column alone.
func gridStages(out string) []string {
	var rows []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 15 || !strings.HasPrefix(line, "  ") || line[13] != ' ' || line[14] == ' ' {
			continue
		}
		label := strings.TrimSpace(line[2:13])
		if label != "" && !strings.ContainsAny(label, " \t") {
			rows = append(rows, label)
		}
	}
	return rows
}

// The six stages, drawn as six rows, in the order the contract names them:
// launch → promptable → work → record → settle → account. A missing stage is
// the failure this exists to catch — the grid is how a runtime is onboarded,
// and a stage nobody drew is a stage nobody fills.
func TestGridDrawsAllSixStagesAsRows(t *testing.T) {
	t.Parallel()
	want := []string{"launch", "promptable", "work", "record", "settle", "account"}
	a := checkApp(t)
	h := Herdr{Bin: "no-such-herdr-binary"}

	// Both kinds, because they take different branches through RuntimeCheck:
	// a template-only yaml that declares nothing, and each built-in.
	runtimes := []*Runtime{writeRuntime(t, a, "mycli", "command: mycli --sys {file}\n")}
	for _, n := range []string{"claude", "codex", "grok"} {
		rt, err := a.LoadRuntime(n)
		if err != nil {
			t.Fatal(err)
		}
		runtimes = append(runtimes, rt)
	}

	for _, rt := range runtimes {
		var b bytes.Buffer
		a.RuntimeCheck(rt, h, &b)
		out := b.String()
		got := gridStages(out)
		if len(got) < len(want) {
			t.Fatalf("%s: grid drew %d rows, want at least the %d stages: %v\n%s", rt.Name, len(got), len(want), got, out)
		}
		// The stages come first; `tier`, `rulebooks`, `state_dir` and the
		// preflight rows follow and are not stages (ADR 0013 §6 is its own
		// section), so the six are the head of the list.
		for i, w := range want {
			if got[i] != w {
				t.Errorf("%s: stage row %d is %q, want %q — the grid is %v\n%s", rt.Name, i, got[i], w, got[:len(want)], out)
			}
		}
	}
}

// §5's cap line: the number is read, not repeated. Three states, and the two
// that were unpinned are the two that can lie — an armed cap must name the
// ledger it is counted in, and a value that is not a bead count must say it
// is not a cap rather than dressing itself as one.
// Every assertion is scoped to what follows `uncounted_cap_mycli:` — the
// grid says "unset" about `prompt:` and `record:` too, so an unscoped
// negative would be answering about the wrong key. The output is flattened
// first because the cap line wraps.
func TestGridCapLineIsParsedNotEchoed(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, cfg      string
		want, unwanted []string
	}{
		{"unset", "",
			[]string{"unset — unlimited and loud"}, []string{"beads / rolling 7 days"}},
		{"armed", "uncounted_cap_mycli: 4\n",
			[]string{"4 beads / rolling 7 days, ledgered in"},
			[]string{"unset", "unlimited and loud"}},
		{"malformed", "uncounted_cap_mycli: soon\n",
			[]string{`"soon" is not a positive bead count`, "no cap: unlimited and loud"},
			[]string{"soon beads", "unset"}},
		{"zero", "uncounted_cap_mycli: 0\n",
			[]string{`"0" is not a positive bead count`},
			[]string{"0 beads / rolling 7 days", "unset"}},
		{"negative", "uncounted_cap_mycli: -3\n",
			[]string{`"-3" is not a positive bead count`},
			[]string{"-3 beads / rolling 7 days", "unset"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := checkApp(t)
			a.StateDir = t.TempDir()
			if c.cfg != "" {
				if err := os.WriteFile(a.ConfigPath, []byte(c.cfg), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			rt := writeRuntime(t, a, "mycli", "command: mycli --sys {file}\n")
			var b bytes.Buffer
			a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
			out := b.String()

			flat := strings.Join(strings.Fields(out), " ")
			const key = "uncounted_cap_mycli:"
			i := strings.Index(flat, key)
			if i < 0 {
				t.Fatalf("cap %q: the account row never names the key:\n%s", c.cfg, out)
			}
			line := flat[i+len(key):]
			if j := strings.Index(line, "the cap counts beads posse itself launched"); j >= 0 {
				line = line[:j] // the row's closing note is not the cap line
			}
			for _, w := range c.want {
				if !strings.Contains(line, w) {
					t.Errorf("cap %q: the cap line must say %q, got %q\n%s", c.cfg, w, line, out)
				}
			}
			for _, u := range c.unwanted {
				if strings.Contains(line, u) {
					t.Errorf("cap %q: the cap line must NOT say %q — it claims a brake nothing arms; got %q\n%s", c.cfg, u, line, out)
				}
			}
		})
	}
}
