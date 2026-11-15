//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0011 §5 — the launch share: one label's class gets a reserved,
// lendable slice of `-n` (ranger-base-j11mf, built under ranger-base-wq19a).
//
// Every behavioural test here drives the real fire pass through `Run`, and
// most of them drive it under --dry-run: a dry pass runs the whole of
// fireLoop — the class decision, the two readings, the charge, the launch
// line — and launches nothing, so the fixtures need no herdr agent, leave no
// prompt leg in flight and read the same decisions a real pass would make.
// The two that must NOT be dry say why where they are: the epoch booking and
// the refill are exactly the halves a dry pass is defined not to do.
//
// The CONTROL ARM is the thing to keep: a class that gets its allotment
// under a deep queue proves nothing unless the same queue with no share
// configured is shown to starve it. Three tests below carry one.

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

// shareHome is qaRepo plus whatever config lines the test is about.
//
// qaRepo writes the whole config to point `beads:` at its repo, so the share
// keys go in with it rather than after it — a second write would drop the
// beads dir and the pass would have no queue at all.
func shareHome(t *testing.T, a *App, ready, show, cfg string) string {
	t.Helper()
	repo := qaRepo(t, a, ready, show)
	write(t, a.ConfigPath, "beads:\n  - "+repo+"\n"+cfg)
	return repo
}

// shareSeats writes n personas into one lane, so a reading can really make n
// launches: §4 gives one bead per persona per repo, so the seat count is the
// third bound on a pass and a test about the FIRST bound has to keep it out
// of the way.
func shareSeats(t *testing.T, a *App, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		writePersona(t, a, "seat"+strconv.Itoa(i), "[go]")
	}
}

// beadRow is one ready row, with the priority that puts it where the test
// wants it in the queue (OrderBeads sorts on priority ascending).
func beadRow(id string, priority int, labels string) string {
	return `{"id":"` + id + `","title":"t","priority":` + strconv.Itoa(priority) + `,"labels":[` + labels + `]}`
}

const (
	shareLabels = `"go","project"` // a bead in the share's class
	remLabels   = `"go"`           // a bead in the remainder
)

// ─── the two keys ────────────────────────────────────────────────────────────

// Every dormant arm §5 names, and the armed one, read off the config the way
// production reads it.
//
// The arms are a table because the ORDER matters: an operator who sets a
// share of 6 against `-n 6` on a box whose `-n` is 0 should be told about
// `-n` first, since fixing the share would change nothing.
func TestLaunchShareFromConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		cfg         string
		pidLabels   string // a non-coordinator PID's labels, "" for none
		max         int
		wantArmed   bool
		wantShare   int
		wantRem     int
		wantDormant string // substring
		wantConfig  bool
	}{
		{name: "neither key — no share was ever asked for", cfg: "", max: 6},
		{
			name: "both keys, below -n — armed",
			cfg:  "launch_share_label: project\nlaunch_share: 2\n",
			max:  6, wantArmed: true, wantShare: 2, wantRem: 4, wantConfig: true,
		},
		{
			name: "-n 0 has no bounded ration to split",
			cfg:  "launch_share_label: project\nlaunch_share: 2\n",
			max:  0, wantDormant: "-n 0 is no bounded ration", wantConfig: true,
		},
		{
			name: "the label alone",
			cfg:  "launch_share_label: project\n",
			max:  6, wantDormant: "launch_share: is unset", wantConfig: true,
		},
		{
			name: "the count alone",
			cfg:  "launch_share: 2\n",
			max:  6, wantDormant: "launch_share_label: is unset", wantConfig: true,
		},
		{
			name: "a count that is not a count",
			cfg:  "launch_share_label: project\nlaunch_share: two\n",
			max:  6, wantDormant: `"two" is not a count`, wantConfig: true,
		},
		{
			name: "zero reserves nothing, so it is not a count either",
			cfg:  "launch_share_label: project\nlaunch_share: 0\n",
			max:  6, wantDormant: `"0" is not a count`, wantConfig: true,
		},
		{
			name: "a share equal to -n leaves the remainder zero",
			cfg:  "launch_share_label: project\nlaunch_share: 6\n",
			max:  6, wantDormant: "is not below -n 6", wantConfig: true,
		},
		{
			name: "a share above -n, same reason",
			cfg:  "launch_share_label: project\nlaunch_share: 9\n",
			max:  6, wantDormant: "is not below -n 6", wantConfig: true,
		},
		{
			// §5: a PID carrying the label would make it a §4 LANE and seat
			// that PID on every share bead. The share must never re-route
			// work, so this configuration is refused by dormanting.
			name:      "a PID carries the label — it would be a lane",
			cfg:       "launch_share_label: project\nlaunch_share: 2\n",
			pidLabels: "[project, code]",
			max:       6, wantDormant: "carries labels: [project, code]", wantConfig: true,
		},
		{
			// …and the one PID dispatch never hires is not a lane either
			// (ADR 0033 §1), so her labels do not dormant the share.
			name:      "the coordinator's labels are not a lane",
			cfg:       "coordinator: boss\nlaunch_share_label: project\nlaunch_share: 2\n",
			pidLabels: "",
			max:       6, wantArmed: true, wantShare: 2, wantRem: 4, wantConfig: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			writePersona(t, b.App, "ranger", "[go]")
			if tc.pidLabels != "" {
				writePersona(t, b.App, "projectpid", tc.pidLabels)
			}
			if strings.Contains(tc.cfg, "coordinator:") {
				writePersona(t, b.App, "boss", "[project, triage]")
			}
			if err := os.WriteFile(b.App.ConfigPath, []byte(tc.cfg), 0o644); err != nil {
				t.Fatal(err)
			}

			got := b.App.LaunchShareFor(tc.max)
			if got.Configured != tc.wantConfig {
				t.Errorf("Configured = %v, want %v (%#v)", got.Configured, tc.wantConfig, got)
			}
			if got.Armed() != tc.wantArmed {
				t.Errorf("Armed() = %v, want %v — dormant reason %q", got.Armed(), tc.wantArmed, got.Dormant)
			}
			if got.Share != tc.wantShare || got.Remainder != tc.wantRem {
				t.Errorf("allotments = %d/%d, want %d/%d", got.Share, got.Remainder, tc.wantShare, tc.wantRem)
			}
			if tc.wantDormant != "" && !strings.Contains(got.Dormant, tc.wantDormant) {
				t.Errorf("the dormant reason must name the key that did it (%q):\n%s", tc.wantDormant, got.Dormant)
			}
			if tc.wantDormant == "" && tc.wantConfig && got.Dormant != "" {
				t.Errorf("want armed, got dormant: %s", got.Dormant)
			}
		})
	}
}

// The line every `--watch` log and every `posse dispatch --dry-run` prints:
// armed it names BOTH allotments, dormant it names the key, and with no
// share configured it is byte-identical to what it printed before the share
// existed — which is the property that keeps this quiet on the instances
// that never asked for one (rangerhq-v83's rule is about typos, not about
// unset keys).
func TestLaunchCapLineCarriesTheShare(t *testing.T) {
	t.Parallel()

	bare := LaunchCapLine(6, time.Hour, LaunchShare{})
	if strings.Contains(bare, "launch share") {
		t.Errorf("an instance that configured no share must read exactly as before:\n%s", bare)
	}

	armed := LaunchCapLine(6, time.Hour, LaunchShare{Label: "project", Share: 2, Remainder: 4, Configured: true})
	if !strings.HasPrefix(armed, bare) {
		t.Errorf("the share is a clause ON the cap line, not a replacement for it:\n%s", armed)
	}
	for _, want := range []string{"project", "2", "4", "remainder", "ADR 0011 §5"} {
		if !strings.Contains(armed, want) {
			t.Errorf("the armed line must name both allotments and the label (%q):\n%s", want, armed)
		}
	}

	dormant := LaunchCapLine(6, time.Hour, LaunchShare{Label: "project", Dormant: `launch_share: "two" is not a count of launch attempts`, Configured: true})
	if !strings.Contains(dormant, "DORMANT") || !strings.Contains(dormant, "is not a count") {
		t.Errorf("a dormant share must name the key that made it dormant — never silently obeyed:\n%s", dormant)
	}

	// `-n 0` is the other branch of the same function, and it carries the
	// clause too: that is the arm where the share is dormant BECAUSE of -n,
	// so it is the one an operator most needs to read.
	nocap := LaunchCapLine(0, time.Hour, LaunchShare{Label: "project", Dormant: "-n 0 is no bounded ration to split", Configured: true})
	if !strings.Contains(nocap, "no cap") || !strings.Contains(nocap, "-n 0 is no bounded ration") {
		t.Errorf("-n 0 must name both facts:\n%s", nocap)
	}
}

// ─── owner first ─────────────────────────────────────────────────────────────

// §5's first half: "a class that is under its own allotment launches first".
//
// The queue is six remainder beads ahead of two share beads and `-n` is 6, so
// without a share the remainder spends the whole ration and the share's work
// gets nothing — which is the CONTROL ARM, asserted rather than assumed,
// because it is the shape ranger-base-j11mf measured 66 times in 101 epochs.
// With `launch_share: 2` the share's two beads launch and the remainder is
// held to its own four.
func TestShareClassGetsItsAllotmentUnderADeepRemainderQueue(t *testing.T) {
	t.Parallel()
	ready := "[" + strings.Join([]string{
		beadRow("a-1", 0, remLabels), beadRow("a-2", 0, remLabels), beadRow("a-3", 0, remLabels),
		beadRow("a-4", 0, remLabels), beadRow("a-5", 0, remLabels), beadRow("a-6", 0, remLabels),
		beadRow("p-1", 1, shareLabels), beadRow("p-2", 1, shareLabels),
	}, ",") + "]"

	for _, tc := range []struct {
		name            string
		cfg             string
		wantShare       int
		wantRemainder   int
		wantTotalLaunch int
	}{
		{
			name:            "no share: the deeper queue spends the epoch",
			cfg:             "",
			wantShare:       0,
			wantRemainder:   6,
			wantTotalLaunch: 6,
		},
		{
			name:            "a share of 2: the project class gets its two",
			cfg:             "launch_share_label: project\nlaunch_share: 2\n",
			wantShare:       2,
			wantRemainder:   4,
			wantTotalLaunch: 6,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.DryRun = true
			shareSeats(t, b.App, 6)
			shareHome(t, b.App, ready, "", tc.cfg)

			n, err := d.Run("", "", 6)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if n != tc.wantTotalLaunch {
				t.Fatalf("launched %d, want %d:\n%s", n, tc.wantTotalLaunch, out)
			}
			if got := strings.Count(out, "· p-"); got != tc.wantShare {
				t.Errorf("the project class launched %d bead(s), want %d:\n%s", got, tc.wantShare, out)
			}
			if got := strings.Count(out, "· a-"); got != tc.wantRemainder {
				t.Errorf("the remainder launched %d bead(s), want %d:\n%s", got, tc.wantRemainder, out)
			}
		})
	}
}

// §5: "each launch line names its class". Dormant it names none, which is
// what keeps every other pass report in this package byte-identical.
func TestEveryLaunchLineNamesItsClass(t *testing.T) {
	t.Parallel()
	ready := "[" + beadRow("p-1", 0, shareLabels) + "," + beadRow("a-1", 1, remLabels) + "]"

	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	shareSeats(t, b.App, 2)
	shareHome(t, b.App, ready, "", "launch_share_label: project\nlaunch_share: 2\n")
	if _, err := d.Run("", "", 6); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	for _, want := range []string{"p-1", "[class: project]", "a-1", "[class: remainder]"} {
		if !strings.Contains(out, want) {
			t.Errorf("each launch names the class it was charged to (%q):\n%s", want, out)
		}
	}

	// The control: the same two beads with no share configured carry no
	// class marker at all.
	b2, _ := newTestBackend(t)
	d2 := newTestDispatcher(t, b2)
	d2.DryRun = true
	shareSeats(t, b2.App, 2)
	shareHome(t, b2.App, ready, "", "")
	if _, err := d2.Run("", "", 6); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d2); strings.Contains(out, "[class:") {
		t.Errorf("an instance with no share must print no class markers:\n%s", out)
	}
}

// ─── then lent ───────────────────────────────────────────────────────────────

// §5's second half: "room left over after both classes have had their pick
// goes to whatever is still launchable" — so an idle class's allotment is
// LENT and never stranded.
//
// Six remainder beads, nothing in the share's class, `-n 6` split 2/4. The
// remainder takes its four in the owner's reading and the share's two in the
// lending one, and the two lent launches say so. The control arm is the
// alternative this rules out: a reserved slice that simply went unused would
// launch four and leave two seats idle with ready work in front of them.
func TestIdleShareIsLentWithinTheSameReading(t *testing.T) {
	t.Parallel()
	ready := "[" + strings.Join([]string{
		beadRow("a-1", 0, remLabels), beadRow("a-2", 0, remLabels), beadRow("a-3", 0, remLabels),
		beadRow("a-4", 0, remLabels), beadRow("a-5", 0, remLabels), beadRow("a-6", 0, remLabels),
	}, ",") + "]"

	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	shareSeats(t, b.App, 6)
	shareHome(t, b.App, ready, "", "launch_share_label: project\nlaunch_share: 2\n")

	n, err := d.Run("", "", 6)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 6 {
		t.Fatalf("an idle share is LENT, not stranded: launched %d of 6 with six ready beads:\n%s", n, out)
	}
	if got := strings.Count(out, "[class: remainder, lent]"); got != 2 {
		t.Errorf("the two lent launches must say they were lent, got %d:\n%s", got, out)
	}
	if got := strings.Count(out, "[class: remainder]"); got != 4 {
		t.Errorf("the remainder's own four must not read as lent, got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "the remainder class spent its 4 attempt(s)") ||
		!strings.Contains(out, "2 more bead(s) in it were held for the lending reading, which launched 2 of them") {
		t.Errorf("the class that ran out of its own allotment must say so, and say what the lend covered:\n%s", out)
	}
}

// §5: "a class that becomes ready after lending has spent the epoch waits for
// the turn" — and the other direction, which is the guarantee itself: a class
// AT its allotment waits while the other class launches, rather than taking
// room that is not its own.
//
// The epoch is mid-flight: the share has spent its two and the remainder two
// of its four, so two attempts are left and they are the remainder's. Three
// share beads sit at the head of the queue and two remainder beads behind
// them; the three wait, and the two launch past them.
func TestAClassAtItsAllotmentWaitsWhileTheOtherLaunches(t *testing.T) {
	t.Parallel()
	ready := "[" + strings.Join([]string{
		beadRow("p-1", 0, shareLabels), beadRow("p-2", 0, shareLabels), beadRow("p-3", 0, shareLabels),
		beadRow("a-1", 1, remLabels), beadRow("a-2", 1, remLabels),
	}, ",") + "]"

	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	shareSeats(t, b.App, 4)
	shareHome(t, b.App, ready, "", "launch_share_label: project\nlaunch_share: 2\ndispatch_epoch: 1h\n")

	// Mid-epoch, the way the epoch's own pins do it: roll first so Run's
	// own roll finds the same window and resets nothing, then book what an
	// earlier pass of this epoch spent.
	d.rollEpoch(time.Now())
	d.bookSpend(shareSpend{share: 2, remainder: 2})

	n, err := d.Run("", "", 6)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 2 {
		t.Fatalf("two attempts are left in this epoch: launched %d:\n%s", n, out)
	}
	if strings.Contains(out, "· p-") {
		t.Errorf("the project class is at its allotment — its beads wait for the turn, they do not take the remainder's room:\n%s", out)
	}
	if got := strings.Count(out, "[class: remainder]"); got != 2 {
		t.Errorf("the remainder has room and launchable work, so it launches: %d of 2:\n%s", got, out)
	}
	if !strings.Contains(out, "the project class spent its 2 attempt(s)") ||
		!strings.Contains(out, "3 more bead(s) in it were held for the lending reading, which launched 0 of them") {
		t.Errorf("the class that ran out must be named, and so must the fact that nothing was lent to it:\n%s", out)
	}
}

// ─── the dormant arms change nothing ─────────────────────────────────────────

// Every dormant arm: the pass fires exactly as it would with no share
// configured at all, and the reason is on the line the dry run prints.
//
// "Changes nothing" is the claim worth pinning, because the alternative to
// dormanting is each of these arms doing SOMETHING — a share of 6 against
// `-n 6` starving every other bead by configuration, a typo'd count reserving
// zero and reading as armed.
func TestEveryDormantArmNamesItsReasonAndChangesNothing(t *testing.T) {
	t.Parallel()
	ready := "[" + strings.Join([]string{
		beadRow("a-1", 0, remLabels), beadRow("a-2", 0, remLabels),
		beadRow("p-1", 1, shareLabels), beadRow("p-2", 1, shareLabels),
	}, ",") + "]"

	for _, tc := range []struct {
		name, cfg, pid, want string
		launched             int
	}{
		{
			name: "label unset",
			cfg:  "launch_share: 2\n",
			want: "launch_share_label: is unset", launched: 4,
		},
		{
			name: "count unset",
			cfg:  "launch_share_label: project\n",
			want: "launch_share: is unset", launched: 4,
		},
		{
			name: "count is not a count",
			cfg:  "launch_share_label: project\nlaunch_share: lots\n",
			want: `"lots" is not a count`, launched: 4,
		},
		{
			name: "share not below -n",
			cfg:  "launch_share_label: project\nlaunch_share: 4\n",
			want: "is not below -n 4", launched: 4,
		},
		{
			// THREE, and the three are the reason this arm dormants.
			// `project` is a §4 LANE here — one PID carries it — so routing
			// seats projectpid on p-1 and p-2 waits for a later pass with
			// `project lane busy`. That is what the share would have been
			// riding on top of, and it is unchanged by the share being
			// dormant: §5 refuses the configuration precisely so that
			// arming a BUDGET class can never be what re-routed the work.
			name: "a PID carries the label",
			cfg:  "launch_share_label: project\nlaunch_share: 2\n",
			pid:  "[project, code]",
			want: "a §4 lane", launched: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.DryRun = true
			shareSeats(t, b.App, 4)
			if tc.pid != "" {
				writePersona(t, b.App, "projectpid", tc.pid)
			}
			shareHome(t, b.App, ready, "", tc.cfg)

			// -n 4 in every arm, including the "not below -n" one, which is
			// what makes a share of 4 illegal there.
			n, err := d.Run("", "", 4)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if n != tc.launched {
				t.Fatalf("a dormant share changes nothing: launched %d, want %d:\n%s", n, tc.launched, out)
			}
			if strings.Contains(out, "[class:") {
				t.Errorf("a dormant share charges no class and marks no launch:\n%s", out)
			}
			if strings.Contains(out, "launch share: the ") {
				t.Errorf("a dormant share defers nothing, so it has no class to report:\n%s", out)
			}
			if !strings.Contains(out, "DORMANT") || !strings.Contains(out, tc.want) {
				t.Errorf("the dry run must name which key made the share dormant (%q):\n%s", tc.want, out)
			}
			if d.epochClass != (shareSpend{}) {
				t.Errorf("a dormant share books no class counts: %#v", d.epochClass)
			}
		})
	}
}

// ─── the epoch ───────────────────────────────────────────────────────────────

// A dry pass books nothing — the whole of what `--dry-run` promises — and
// that has to hold for the per-class counts too, or a diagnostic run would
// spend the share's reservation for the hour.
func TestDryRunBooksNoClassAttempts(t *testing.T) {
	t.Parallel()
	ready := "[" + beadRow("p-1", 0, shareLabels) + "," + beadRow("a-1", 1, remLabels) + "]"

	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	shareSeats(t, b.App, 2)
	shareHome(t, b.App, ready, "", "launch_share_label: project\nlaunch_share: 2\n")

	if n, err := d.Run("", "", 6); err != nil || n != 2 {
		t.Fatalf("the dry pass must still report what a real one would do: %d %v\n%s", n, err, dispatcherOut(d))
	}
	if d.epochAttempts != 0 || d.epochClass != (shareSpend{}) {
		t.Errorf("a dry pass booked %d attempt(s) and %#v against the epoch — it launched nothing", d.epochAttempts, d.epochClass)
	}
}

// The per-class counts have the same lifetime as the count they split: they
// roll with the epoch, and inside one epoch a pass that rolls nothing must
// not hand the share back.
//
// Their TOTAL is epochAttempts at every instant, which is what makes
// bookSpend the one site either may move through: a class count that drifted
// from the total would bound the share against an epoch that never happened.
func TestTheClassCountsRollWithTheEpochAndTotalTheCap(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	now := time.Now()

	if !d.rollEpoch(now) {
		t.Fatal("the first roll of a process always opens an epoch")
	}
	d.bookSpend(shareSpend{share: 2, remainder: 1})
	if d.epochAttempts != 3 || d.epochClass.total() != 3 {
		t.Fatalf("the total and the split are one fact: %d vs %d", d.epochAttempts, d.epochClass.total())
	}
	if d.rollEpoch(now.Add(time.Millisecond)) {
		t.Error("a second pass in the same epoch must not open a new one")
	}
	if d.epochClass != (shareSpend{share: 2, remainder: 1}) {
		t.Errorf("the split reset inside an epoch: %#v", d.epochClass)
	}
	if !d.rollEpoch(EpochStart(now, time.Hour).Add(time.Hour + time.Second)) {
		t.Error("the next wall-clock hour is a new epoch")
	}
	if d.epochClass != (shareSpend{}) || d.epochAttempts != 0 {
		t.Errorf("a new epoch must hand both classes back: %#v / %d", d.epochClass, d.epochAttempts)
	}
}

// §5: "an exhaustion line names the class that ran out". The epoch's ration
// is spent; the line says how it split, so `project 5 of 2` reads as a lend
// that happened and `remainder 4 of 4` beside it as the class that ran out.
func TestTheExhaustionLineNamesTheClassSplit(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	if err := os.WriteFile(b.App.ConfigPath, []byte("dispatch_epoch: 1h\nlaunch_share_label: project\nlaunch_share: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d.rollEpoch(time.Now())
	d.bookSpend(shareSpend{share: 2, remainder: 4})

	share := b.App.LaunchShareFor(6)
	if !share.Armed() {
		t.Fatalf("fixture: the share must be armed, got %q", share.Dormant)
	}
	room, ok := d.epochRoom(6, share)
	if ok || room != 0 {
		t.Fatalf("a spent cap has no room: room=%d ok=%v", room, ok)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "launch share: project 2 of 2, remainder 4 of 4") {
		t.Errorf("the exhaustion line must name which class spent the ration:\n%s", out)
	}

	// The control: with no share configured the line is what it always was.
	b2, _ := newTestBackend(t)
	d2 := newTestDispatcher(t, b2)
	d2.rollEpoch(time.Now())
	d2.bookSpend(shareSpend{remainder: 6})
	if _, ok := d2.epochRoom(6, b2.App.LaunchShareFor(6)); ok {
		t.Fatal("fixture: the cap must be spent")
	}
	if out := dispatcherOut(d2); strings.Contains(out, "launch share") {
		t.Errorf("an instance with no share must read as it always did:\n%s", out)
	}
}

// ─── the refill ──────────────────────────────────────────────────────────────

// The refill is the other caller of the one choke point, so it charges the
// same way — and this is the arm a dry pass cannot assert, because booking
// the epoch is exactly what `--dry-run` is defined not to do.
//
// One seat, two share beads: the fire pass launches p-1, its settle frees the
// seat, and the refill re-reads the queue and launches p-2. Both attempts are
// the share's, so the epoch has two of its two booked to that class — and
// `epochClass.total()` still equals `epochAttempts`, which is the invariant
// that makes bookSpend the one site.
//
// MUTATION: book the refill's spend with `d.epochAttempts += spend.total()`
// instead of bookSpend → the class count reads 1 and the total reads 2.
func TestTheRefillChargesTheShareTheSameWay(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := shareHome(t, b.App,
		"["+beadRow("p-1", 0, shareLabels)+"]",
		`[{"id":"p-1","status":"closed"},{"id":"p-2","status":"closed"}]`,
		"launch_share_label: project\nlaunch_share: 2\n")
	// What the refill's own fresh `bd ready` answers: p-1 closed and gone,
	// p-2 waiting — the swap rollingrun_qa_test.go's refill fixtures use.
	write(t, repo+"/fake-ready-next.json", "["+beadRow("p-2", 0, shareLabels)+"]")
	agentPerLaunch(t, fake)
	d.Refill = true
	d.PromptGrace = 0

	if _, err := d.Run("", "", 6); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "· p-1") || !strings.Contains(out, "· p-2") {
		t.Fatalf("both beads must go out inside this one Run — the second through the refill:\n%s", out)
	}
	if d.epochClass.share != 2 || d.epochClass.remainder != 0 {
		t.Errorf("both launches are the project class's, and the refill must charge its own: %#v\n%s", d.epochClass, out)
	}
	if d.epochClass.total() != d.epochAttempts {
		t.Errorf("the split and the total disagree (%d vs %d) — something booked `-n` without booking a class:\n%s",
			d.epochClass.total(), d.epochAttempts, out)
	}
	if got := strings.Count(out, "[class: project]"); got != 2 {
		t.Errorf("a refill's launch names its class like any other, got %d:\n%s", got, out)
	}
}

// And the same invariant over the ordinary fire pass, where `-n` is really
// spent: the two class counts add up to what the epoch charged.
func TestAFirePassBooksTheTotalAndTheSplitTogether(t *testing.T) {
	t.Parallel()
	ready := "[" + strings.Join([]string{
		beadRow("p-1", 0, shareLabels), beadRow("a-1", 1, remLabels), beadRow("a-2", 1, remLabels),
	}, ",") + "]"

	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	shareSeats(t, b.App, 3)
	shareHome(t, b.App, ready, `[{"id":"p-1","status":"closed"},{"id":"a-1","status":"closed"},{"id":"a-2","status":"closed"}]`,
		"launch_share_label: project\nlaunch_share: 2\n")
	agentPerLaunch(t, fake)

	if _, err := d.Run("", "", 6); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if d.epochAttempts != 3 {
		t.Fatalf("three beads, three seats, six attempts of room: %d booked\n%s", d.epochAttempts, out)
	}
	if d.epochClass != (shareSpend{share: 1, remainder: 2}) {
		t.Errorf("one project bead and two remainder beads: %#v\n%s", d.epochClass, out)
	}
	if d.epochClass.total() != d.epochAttempts {
		t.Errorf("the split and the total disagree: %d vs %d", d.epochClass.total(), d.epochAttempts)
	}
}

// ─── the label on the beads the seat files ───────────────────────────────────

// §5's failure mode is label DRIFT, and the drift is done by hand: the class
// is the bead's labels, a handoff files the receiving LANE's label and a
// budget class is not a lane, so a bead filed off a share bead without the
// label lands in the remainder — and the share shrinks, epoch by epoch, to
// the beads the operator labelled personally. The one reader who can prevent
// that is the seat holding the share bead, so the work prompt tells it
// (ranger-base-7qtci).
//
// Five arms, and the four ABSENCES are the point: a remainder bead has no
// label to carry on, and a dormant or unconfigured share must not instruct
// anyone to label work against a split that is not in force.
func TestTheWorkPromptNamesTheShareLabelOnAShareBead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cfg    string
		max    int
		labels []string
		want   bool
	}{
		{
			name: "armed, and the bead is in the class",
			cfg:  "launch_share_label: project\nlaunch_share: 2\n",
			max:  6, labels: []string{"go", "project"}, want: true,
		},
		{
			name: "armed, but the bead is in the remainder",
			cfg:  "launch_share_label: project\nlaunch_share: 2\n",
			max:  6, labels: []string{"go"},
		},
		{
			name: "dormant: this pass's -n is no ration to split",
			cfg:  "launch_share_label: project\nlaunch_share: 2\n",
			max:  0, labels: []string{"go", "project"},
		},
		{
			name: "dormant: the count is not a count",
			cfg:  "launch_share_label: project\nlaunch_share: two\n",
			max:  6, labels: []string{"go", "project"},
		},
		{
			name: "no share was ever configured",
			cfg:  "",
			max:  6, labels: []string{"go", "project"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			repo := shareHome(t, b.App, "[]", "", tc.cfg)
			is := RepoIssue{BdIssue: BdIssue{ID: "p-1", Title: "t", Labels: tc.labels}, Dir: repo}
			// Off the real config and the real `-n`, so the dormant arms are
			// the ones production would resolve and not a hand-built struct.
			sh := b.App.LaunchShareFor(tc.max)
			ctx := b.App.promptContext(Bd{Bin: fakeBinFor(t, "bd")}, is, "claude", "standard", "", nil, sh)
			p := workPrompt(is, ctx)
			got := strings.Contains(p, "- launch share: this bead is in the `project` class (ADR 0011 §5)")
			if got != tc.want {
				t.Errorf("share filing line present=%v, want %v:\n%s", got, tc.want, p)
			}
			if tc.want {
				if ctx.ShareLabel != "project" {
					t.Errorf("ShareLabel = %q, want project", ctx.ShareLabel)
				}
				// The line has to carry both halves to be worth a line at
				// all: which label to file, and what filing without it costs.
				for _, want := range []string{"`-l <their label>,project`", "lands in the remainder"} {
					if !strings.Contains(p, want) {
						t.Errorf("missing %q in:\n%s", want, p)
					}
				}
			} else if ctx.ShareLabel != "" {
				t.Errorf("ShareLabel must be empty on this arm, got %q", ctx.ShareLabel)
			}
			// Whatever the arm, the share neither displaces nor duplicates
			// the one Context line that always renders.
			if n := strings.Count(p, pushPrecedence); n != 1 {
				t.Errorf("push precedence rendered %d time(s):\n%s", n, p)
			}
		})
	}
}

// And the line reaches the SEAT, not only the renderer.
//
// The prompt is assembled inside `fire`, which is handed the class decision
// fireLoop charged the attempt to — so this is the test that reds if that
// wiring is dropped, where the render table above would stay green. One real
// pass, two beads, one class each: the share bead's delivered prompt carries
// the line and the remainder bead's does not.
func TestTheDeliveredPromptCarriesTheShareLabel(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	shareSeats(t, b.App, 2)
	shareHome(t, b.App,
		"["+beadRow("p-1", 0, shareLabels)+","+beadRow("a-1", 1, remLabels)+"]",
		`[{"id":"p-1","status":"closed"},{"id":"a-1","status":"closed"}]`,
		"launch_share_label: project\nlaunch_share: 2\n")
	agentPerLaunch(t, fake)

	if _, err := d.Run("", "", 6); err != nil {
		t.Fatal(err)
	}
	// What the personas were really asked, whichever channel carried it.
	out := delivered(t, b.App, fake)
	seg := func(bead string) string {
		i := strings.Index(out, "Work beads issue "+bead)
		if i < 0 {
			t.Fatalf("%s was never prompted:\n%s", bead, out)
		}
		rest := out[i+1:]
		if j := strings.Index(rest, "Work beads issue "); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	if p := seg("p-1"); !strings.Contains(p, "launch share: this bead is in the `project` class") {
		t.Errorf("the share bead's own prompt must name the label it files with:\n%s", p)
	}
	if a := seg("a-1"); strings.Contains(a, "launch share:") {
		t.Errorf("a remainder bead has no share label to carry on:\n%s", a)
	}
}
