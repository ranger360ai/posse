//go:build posse_arm3

package posse

// The LIVE arm of ADR 0071 / ranger-base-i00xh, run against the real bd
// rather than the package's fake.
//
//	RHQ_LIVE_BD=1 go test -tags posse_arm3 ./internal/posse -run TestLiveBdZeroRow -v
//
// WHY THIS EXISTS ON TOP OF bdemptyunread_qa_test.go. Every arm of that file
// drives the FAKE bd, whose `info` case and whose `[]`-over-a-census fixture
// we wrote ourselves from a measurement taken by hand (the eight-fixture
// table is docs/notes.d/ranger-base-a5st4.md). That is the right place to pin
// emptyIsReadable's precedence, the census probe's narrowness and the error's
// wording — those are our logic — but it leaves two things unpinned: whether
// the STATE is still reachable on the pinned bd, and whether the
// DISCRIMINATOR we chose still tells it apart. The day bd 0.50.x stops
// building a zero-row database over an uncommitted jsonl, or `bd info --json`
// renames `issue_count`, the fake keeps teaching the suite the old world and
// every offline arm stays green over a check that no longer fires. So this
// asks bd.
//
// It pins AGREEMENT, in two tests that must both hold:
//
//  1. real bd still answers a `--json` list verb over the zero-row store with
//     `[]`, exit 0, stderr EMPTY — the silent shape ADR 0071 exists for; and
//  2. Bd.Ready still refuses that read, and still SERVES the rig that works.
//
// Test 1 is the one that earns the file. Without it a green test 2 could mean
// "the check works" or "bd started saying something and any reader would have
// noticed" — and those are not the same fact.
//
// THE RIG is the five-step one a5st4's Method records, built in t.TempDir: a
// git repo, a hand-written `.beads/issues.jsonl` of two rows, and no `bd
// init` — the verb is denied at every persona seat, which is why the fixture
// writes the census by hand and lets bd's own first read build the database.
// The two arms differ in ONE step, when the jsonl is committed:
//
//	read first, commit second -> the first read finds `0 in git`, builds the
//	    FILE and not the GRAPH (a5st4 finding 3): a zero-row beads.db over a
//	    census that now carries two rows. The defect.
//	commit first, read second -> the first read creates the database, so the
//	    handle it imports through is read-write and the import succeeds
//	    (a5st4 row one). The case that works, and the control.
//
// BEADS_DIR is bound by the code under test, not by the fixture: Bd.runOnce
// sets `BEADS_DIR=beadsHome(dir)` itself (bdStoreEnv), so pointing dir at the
// rig is the whole of it. The fixture's own calls go through liveBd, which
// SHEDS the variable this session was launched with — without that they would
// read the live store of record and the rig would measure nothing.
//
// THE REPAIR ARM IS NOT PINNED, here or anywhere. `bd import -i
// .beads/issues.jsonl` is the operator's one command and
// `Bash(bd import:*)` is in the shipped deny set of every persona PID
// (ADR 0015 §3, a5st4 "The repair is the operator's"), so the seats that run
// this suite cannot type it: a pin that ran the repair would be a pin that
// skips on every box we have. The error's command TEXT is pinned instead —
// offline, as text, by TestQABdStoreUnreadErrorNamesTheOperatorsRepair, and
// below as a substring of what the live read actually returns.
//
// Env-gated and skipped by default like the other live pins: it shells out to
// the operator's bd, which has a version, a daemon and a cache. Every call
// carries `--no-daemon` (ranger-base-42mv) and every byte stays inside one
// t.TempDir.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// liveCensusRows is the two-row census both rigs are built from — the shape
// `bd sync --flush-only` writes, which is what the operator's store holds and
// what bd's import reads. Two rather than one so the control arm's answer is
// a COUNT as well as a hit: a rig that served one row would be a rig whose
// import half-ran.
const liveCensusRows = `{"id":"dkrw-1","title":"row one","status":"open","priority":2,"issue_type":"task","created_at":"2026-10-09T00:00:00Z","updated_at":"2026-10-09T00:00:00Z"}
{"id":"dkrw-2","title":"row two","status":"open","priority":2,"issue_type":"task","created_at":"2026-10-09T00:00:00Z","updated_at":"2026-10-09T00:00:00Z"}
`

// liveCensusRepo is a real git repo carrying the two-row census on disk and
// NOT in HEAD, with no database of any kind yet. Both arms start here; which
// one a caller gets is decided by whether it commits before it reads.
func liveCensusRepo(t *testing.T) string {
	t.Helper()
	repo := wtRepo(t)
	t.Cleanup(func() { stopLeakedDaemons(t, repo) })
	write(t, filepath.Join(repo, beadsDirName, beadsJSONL), liveCensusRows)
	return repo
}

// liveCommitCensus commits the census, path-limited. The pathspec is this
// shop's commit form and it is also the honest one here: the seed README is
// already in HEAD and nothing else in the rig is meant to move.
func liveCommitCensus(t *testing.T, repo string) {
	t.Helper()
	const rel = ".beads/issues.jsonl"
	mustGit(t, repo, "add", "--", rel)
	mustGit(t, repo, "commit", "-q", "-m", "beads", "--", rel)
}

// liveZeroRowStore is the DEFECT: primed by a read while the census was still
// uncommitted, so bd built a database holding nothing, and the census
// committed afterwards so it plainly carries rows now.
//
// The state is CHECKED rather than trusted to the recipe, the way
// liveBeadsRepoOfClass checks the store class (ranger-base-9lrzx): a rig that
// quietly imported would pass the control arm's assertions and prove nothing
// in the refusing one, and it would fail far from here. Both halves of ADR
// 0071's discriminator are asserted — bd holds zero, the census holds rows —
// because either one alone is a state this record does not act on.
func liveZeroRowStore(t *testing.T, bd func(string, ...string) (string, error)) string {
	t.Helper()
	repo := liveCensusRepo(t)
	if out, err := bd(repo, "list"); err != nil {
		t.Fatalf("the priming read itself failed in %s — the rig cannot be built on this bd: %v %s", repo, err, out)
	}
	db := filepath.Join(repo, beadsDirName, "beads.db")
	if _, err := os.Stat(db); err != nil {
		t.Skipf("bd built no %s on a plain read, so this bd does not reach a5st4's zero-row state at all and there is nothing here to refuse: %v", db, err)
	}
	liveCommitCensus(t, repo)
	if n := liveIssueCount(t, bd, repo); n != 0 {
		t.Fatalf("the rig in %s holds %d issues, not 0: bd imported where a5st4 finding 3 says it builds the FILE and not the GRAPH, so this arm has no defect to refuse — re-measure the table in docs/notes.d/ranger-base-a5st4.md before trusting this pin", repo, n)
	}
	liveCensusCarriesBothRows(t, repo)
	return repo
}

// liveCensusCarriesBothRows is the other half of the premise, read with
// os.ReadFile rather than through censusHasRecord: the product helper is part
// of what this pin is asking about, and a fixture that establishes its own
// premise with the code under test can only ever agree with it. git's copy is
// checked too — the committed census is what bd's import reads (a5st4 finding
// 2), so a row on disk and not in HEAD would leave this arm measuring the
// wrong absence.
func liveCensusCarriesBothRows(t *testing.T, repo string) {
	t.Helper()
	const rel = ".beads/issues.jsonl"
	onDisk, err := os.ReadFile(filepath.Join(repo, beadsDirName, beadsJSONL))
	if err != nil {
		t.Fatalf("reading the rig's census in %s: %v", repo, err)
	}
	inHead := mustGit(t, repo, "show", "HEAD:"+rel)
	for _, id := range []string{"dkrw-1", "dkrw-2"} {
		if !strings.Contains(string(onDisk), id) {
			t.Fatalf("the census on disk in %s does not carry %q, so ADR 0071's check declines to fire and an empty answer is honest — the fixture, not the claim", repo, id)
		}
		if !strings.Contains(inHead, id) {
			t.Fatalf("HEAD's census in %s does not carry %q: bd imports from git HEAD, never the working tree (a5st4 finding 2), so this arm would be measuring an uncommitted row rather than a store that failed to import", repo, id)
		}
	}
}

// liveImportedStore is the CONTROL, a5st4 row one: the same rig with the
// census committed BEFORE anything reads it. Nothing reads it here — the
// import is Bd.Ready's own first call to make, which is the half of this arm
// worth pinning (the handle is read-write because that call creates the
// file). A probe of any kind at this point would do the import the arm is
// asking about.
//
// The runner is taken and deliberately NOT used — spelled `_` so that is a
// fact about the signature and not something a reader has to verify.
func liveImportedStore(t *testing.T, _ func(string, ...string) (string, error)) string {
	t.Helper()
	repo := liveCensusRepo(t)
	liveCommitCensus(t, repo)
	liveCensusCarriesBothRows(t, repo)
	return repo
}

// liveIssueCount is bd's own `issue_count` over the rig, asked the way the
// fixture asks everything else — through liveBd, with BEADS_DIR shed. It is
// the fixture's witness of the state. It parses with a struct of its OWN,
// never the product's `bdInfo`: Bd.issueCount and its parse are the code
// under test, and a premise established with them could only ever agree with
// them.
func liveIssueCount(t *testing.T, bd func(string, ...string) (string, error), repo string) int {
	t.Helper()
	out, err := bd(repo, "info", "--json")
	if err != nil {
		t.Fatalf("bd info --json in %s: %v %s", repo, err, out)
	}
	i, j := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if i < 0 || j < i {
		t.Fatalf("bd info --json printed no object in %s:\n%s", repo, out)
	}
	var info struct {
		IssueCount *int `json:"issue_count"`
	}
	if err := json.Unmarshal([]byte(out[i:j+1]), &info); err != nil || info.IssueCount == nil {
		t.Fatalf("bd info --json no longer answers with an issue_count (%v) — ADR 0071's discriminator is gone and emptyIsReadable now returns an error over every empty read:\n%s", err, out)
	}
	return *info.IssueCount
}

// TestLiveBdZeroRowStoreStillAnswersAnEmptyJSONListOnStdoutWithNothingOnStderr
// is the tripwire: the silent shape ADR 0071 was written for is still the
// shape the pinned bd has. Measured with the two streams kept APART, which
// liveBd's CombinedOutput cannot do — "stderr is empty" is the whole of why
// a programmatic reader could not see this, and a combined read cannot say
// it.
func TestLiveBdZeroRowStoreStillAnswersAnEmptyJSONListOnStdoutWithNothingOnStderr(t *testing.T) {
	t.Parallel()
	bd := liveBd(t)
	repo := liveZeroRowStore(t, bd)

	// The production argv, verbatim from Bd.Ready, so this measures the call
	// posse makes rather than a spelling of our own.
	cmd := exec.Command("bd", "--no-daemon", "ready", "--json", "--limit", "0")
	cmd.Dir = repo
	cmd.Env = append(shedBeadsDir(cmd.Environ()), "PATH="+PathOutsideGates(""))
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	runErr := cmd.Run()
	stdout, stderr := strings.TrimSpace(out.String()), strings.TrimSpace(errb.String())

	if runErr != nil {
		t.Fatalf("bd now EXITS NON-ZERO over the zero-row store (%v) — the defect ADR 0071 designs around has a signal of its own now, and Bd.run would carry it; re-measure before keeping the count check:\nstdout: %s\nstderr: %s", runErr, stdout, stderr)
	}
	if stdout != "[]" {
		t.Fatalf("bd no longer answers the zero-row store with `[]` but with %q — a5st4's table and ADR 0071's premise both need re-measuring", stdout)
	}
	if stderr != "" {
		t.Errorf("bd now writes something to STDERR over the zero-row store (%q) — the state is no longer silent to a programmatic reader, which is the premise ADR 0071 rests on and the reason internal/posse/herdr_test.go's fake answers nothing there", stderr)
	}
	// And bd's own count still tells it apart from an empty queue, which is
	// the discriminator D2 chose over bd's warning prose.
	if n := liveIssueCount(t, bd, repo); n != 0 {
		t.Errorf("bd info --json answers issue_count %d over the store it just listed as empty — the two no longer disagree and the check has nothing to read", n)
	}
}

// TestLiveBdZeroRowStoreIsRefusedAndTheImportedOneIsServed is posse's side:
// the same Bd.Ready over the two rigs, refusing one and serving the other.
//
// MUTATION, and it is a one-token one by construction: swap the `rig` field
// between the two rows below and BOTH rows red — the refusing row gets two
// issues and no error, the serving row gets BdStoreUnreadError and no rows.
// That is the mutation the bead asked for, and it is why the arms share one
// body instead of each asserting only its own half: an arm that checked
// "refused" without also checking what the OTHER fixture does is an arm a
// reader that refuses everything would pass.
func TestLiveBdZeroRowStoreIsRefusedAndTheImportedOneIsServed(t *testing.T) {
	t.Parallel()
	bd := liveBd(t)

	for _, tc := range []struct {
		name    string
		rig     func(*testing.T, func(string, ...string) (string, error)) string
		refused bool
	}{
		{"read before the census was committed (the zero-row database)", liveZeroRowStore, true},
		{"census committed before the first read (a5st4 row one)", liveImportedStore, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := tc.rig(t, bd)
			issues, err := Bd{Bin: "bd"}.Ready(repo, "")

			if !tc.refused {
				if err != nil {
					t.Fatalf("Ready over the rig that WORKS returned %v — a5st4 row one is the case bd imports on the first read, and refusing it would make the queue unknown over a healthy store", err)
				}
				if len(issues) != 2 {
					t.Fatalf("Ready over the imported rig served %d issues, want the census's 2: %+v", len(issues), issues)
				}
				got := map[string]bool{}
				for _, is := range issues {
					got[is.ID] = true
				}
				for _, want := range []string{"dkrw-1", "dkrw-2"} {
					if !got[want] {
						t.Errorf("the served rows %v do not name %q — the rig imported something other than its own census", got, want)
					}
				}
				return
			}

			if err == nil {
				t.Fatalf("Ready served the zero-row store as an answer (%d issues) — the silent shape is back: an unreadable queue and an empty one are the same bytes again", len(issues))
			}
			if issues != nil {
				t.Errorf("the refusing path returned %d issues beside its error; a caller reading rows first would act on them", len(issues))
			}
			var se *BdStoreUnreadError
			if !errors.As(err, &se) {
				t.Fatalf("Ready failed over the zero-row store but not with BdStoreUnreadError — `cannot be read` and `is empty` are different facts and only one of them names a repair (ADR 0071 D3): %#v / %v", err, err)
			}
			if !IsBdStoreUnread(err) {
				t.Errorf("errors.As found the type but IsBdStoreUnread said no — the helper every caller uses disagrees with the type: %v", err)
			}
			if want := filepath.Join(repo, beadsDirName); se.Store != want {
				t.Errorf("the error names store %q, want the rig's %q — an operator typing the repair in the directory this names would repair the wrong store", se.Store, want)
			}
			if se.Dir != repo {
				t.Errorf("the error names dir %q, want the repo the read was made from, %q", se.Dir, repo)
			}
			// The repair, verbatim, and the repo to type it in. Named here as
			// well as offline because this is the one arm where the text is
			// carried by a read of a real broken store rather than by a fake
			// answering a file we wrote.
			for _, want := range []string{
				"bd import -i .beads/issues.jsonl",
				filepath.Join(repo, beadsDirName),
				filepath.Join(repo, beadsDirName, beadsJSONL),
				repo,
			} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the live error does not name %q, so the operator cannot act on it: %q", want, err.Error())
				}
			}
		})
	}
}
