package posse

// Hermetic herdr-backend tests: the test binary re-execs as a fake `herdr`
// (TestMain checks RHQ_FAKE_HERDR), keeping state in RHQ_FAKE_DIR. No real
// herdr server is touched — tests/run.sh remains the tmux integration suite.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// operatorHome is the $HOME this binary was started with, kept because
// TestMain replaces it: the property ranger-base-gvrh bought — no test cuts
// a worktree in the operator's live ~/.posse — is only checkable against
// the home the check is about, and after D1 nothing else can name it.
var operatorHome string

func TestMain(m *testing.M) {
	// It also doubles as the two binaries the container tier execs
	// (rangerhq-1k1): posse itself, which a caged pane's launcher symlink
	// points at — the same entry point cmd/posse/main.go has — and, in argv
	// mode, the engine that launcher execs, which records the argv that
	// really arrived so a test can see argv[0] for itself.
	if IsCageLaunch(os.Args) {
		fmt.Fprintln(os.Stderr, RunCageLaunch(os.Args)) // returns only on failure
		os.Exit(1)
	}
	if out := os.Getenv("RHQ_CAGE_ARGV_OUT"); out != "" {
		os.WriteFile(out, []byte(strings.Join(os.Args, "\n")), 0o644)
		os.Exit(0)
	}
	// And as posse itself for the one question plugin/autostart.sh asks it
	// (rangerhq-gir5). The hook's liveness decision is `posse dispatch
	// --watch-status`, so the fake posse the hook tests drive must answer it
	// for real — same WatchStatus, same lock, same RHQ_HOME — or the tests
	// would only pin a string the shell and the binary agreed on separately.
	// Everything else the hook calls (new, kill) stays scripted.
	if os.Getenv("RHQ_FAKE_POSSE") == "1" {
		line, err := WatchStatus(NewApp())
		if err != nil {
			fmt.Fprintf(os.Stderr, "posse: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(line)
		os.Exit(0)
	}
	// The test binary doubles as both fake substrates; dispatch on the verb
	// (herdr verbs are workspace/pane/agent; everything else is bd).
	if os.Getenv("RHQ_FAKE_HERDR") == "1" {
		args := os.Args[1:]
		switch {
		case len(args) > 0 && (args[0] == "workspace" || args[0] == "pane" || args[0] == "agent"):
			os.Exit(fakeHerdr(args))
		// …and the third substrate, on the same rule: the VERB, not
		// argv[0]. `run` is gh's (ci-watch's only network call is `gh run
		// list`, ciwatch.go) and bd has no such verb, so the three fakes
		// stay disjoint without anyone reading a link name.
		case len(args) > 0 && args[0] == "run":
			os.Exit(fakeGh(args))
		default:
			os.Exit(fakeBd(args))
		}
	}
	// The suite runs inside a persona pane, whose PATH leads with that
	// session's gates bin — its git shim answered the pre-push hook's own
	// `git -C <repo> push` before the code under test ever saw it
	// (rangerhq-8sd). Step out from behind our own wall for the whole test
	// binary; tests that want a shim on PATH render one and prepend it.
	os.Setenv("PATH", PathOutsideGates(""))
	// The three process-wide constants newTestBackend used to set per test
	// (ADR 0047 D1, ranger-base-aupee). t.Setenv makes a test ineligible for
	// t.Parallel, and these four calls in one helper held 1431 of this
	// package's 1975 tests — 75% of its wall clock — out of any concurrency
	// (docs/notes.d/ranger-base-i7fa.md §2). Three of them were the same
	// value at all 728 call sites, so they belong here, once:
	//
	//   - HOME: one temp home for the whole binary. Nothing under it is
	//     shared state a test can see — what IS shared, ~/.posse/worktrees,
	//     is given back per test by hermetic's WorktreeRootDefault. The
	//     point of the temp home is unchanged from ranger-base-gvrh: a test
	//     reaching EnsureSessionTree must not cut a git worktree in the
	//     operator's live ~/.posse.
	//   - RHQ_FAKE_HERDR: the switch the CHILD reads at startup, above.
	//     Never read by the parent, so per-test was always a constant.
	//   - EnvPersona: hermetic against the operator fence (ADR 0031 §2) —
	//     this backend defaults to an operator session, and a test process
	//     running inside a real persona session otherwise inherits
	//     RHQ_PERSONA from the ambient env. A test that means to drive init
	//     as a persona sets it back.
	//
	// The fourth, RHQ_FAKE_DIR, is genuinely per test and is now told to
	// the child through argv[0] instead — see fakeDir and fakeBinFor.
	operatorHome = os.Getenv("HOME")
	home, err := os.MkdirTemp("", "posse-testhome-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "posse test: temp HOME: %v\n", err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	os.Setenv("RHQ_FAKE_HERDR", "1")
	os.Setenv(EnvPersona, "")
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// ─── the fake bd ─────────────────────────────────────────────────────────────

// fakeBd logs argv to bd-calls.log and serves `ready` from ./fake-ready.json
// in its working directory (bd is per-repo, so cwd is part of the contract).
func fakeBd(args []string) int {
	f, _ := os.OpenFile(filepath.Join(fakeDir(), "bd-calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if f != nil {
		fmt.Fprintln(f, strings.Join(args, " "))
		f.Close()
	}
	sub := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--actor" {
			i++
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			sub = args[i]
			break
		}
	}
	// bd 0.49.1's THIRD failure shape, opt-in with a `fake-json-error`
	// marker: a --json verb that exits 1 having printed
	// `{"error": "..."}` on STDOUT with stderr empty (measured on `bd dep
	// list <missing-id> --json`, rangerhq-aas). A runner reading stderr
	// only sees nothing and quotes the exit status. The marker's contents
	// are the message, so a test can pin that its own sentence survives.
	if b, err := os.ReadFile("fake-json-error"); err == nil && hasArg(args, "--json") {
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			msg = `resolving x: no issue found matching "x"`
		}
		enc, _ := json.Marshal(map[string]string{"error": msg})
		// A `fake-json-error-stderr` marker beside it fills the OTHER
		// channel too, which is the only fixture that can tell the two
		// precedences apart: with stdout alone, either order reads the
		// same sentence.
		if e, err := os.ReadFile("fake-json-error-stderr"); err == nil {
			fmt.Fprint(os.Stderr, strings.TrimSpace(string(e)))
		}
		fmt.Printf("%s\n", enc)
		return 1
	}
	// The same verb failing with a payload that is NOT that shape — a
	// plain listing on stdout, stderr empty — which must NOT be quoted at
	// the operator as if it were a reason.
	if _, err := os.Stat("fake-opaque-error"); err == nil && hasArg(args, "--json") {
		fmt.Print(`[{"id":"a-1","title":"one"}]`)
		return 1
	}
	switch sub {
	case "ready":
		// ranger-base-p969: bd's own staleness refusal, exactly as it reads
		// in the wild — a --no-daemon reader that finds issues.jsonl newer
		// than the database it resolved to. `run` treats this one message
		// as self-healing (beads.go): import once, retry once. The marker
		// fails every call until the "sync" case below clears it, so a test
		// can tell "healed on retry" from "served straight through" apart.
		// A marker holding "keep" is left alone by that clear, for the test
		// that wants the retry to fail too.
		if _, err := os.Stat("fake-ready-stale"); err == nil {
			fmt.Fprint(os.Stderr, "Database out of sync with JSONL. Run 'bd sync --import-only' to fix.")
			return 1
		}
		// The scan failing the way it fails in the wild: bd exits non-zero
		// with a word on stderr, and the repo's queue is unknown, not empty
		// (rangerhq-llse).
		if _, err := os.Stat("fake-ready-fail"); err == nil {
			fmt.Fprint(os.Stderr, "database is locked")
			return 1
		}
		if b, err := os.ReadFile("fake-ready.json"); err == nil {
			fmt.Print(fakeBdReadyDropClosed(fakeBdApplyState(string(b))))
		} else {
			fmt.Print("[]")
		}
		// fake-ready-next.json (file) is the SECOND answer, swapped in once
		// the first has been served: a queue that MOVES between two calls of
		// one Run — a bead closed and dropped, a bead filed since. Real bd's
		// does; this fixture's never did, and a canned list that answers
		// every call the same made every mid-Run behaviour untestable
		// (ranger-base-t8tq). One swap, no counter, no clock.
		if _, err := os.Stat("fake-ready-next.json"); err == nil {
			os.Rename("fake-ready-next.json", "fake-ready.json")
		}
		return 0
	case "list":
		// One verb, three queries. `--status in_progress` is the claimed
		// list every caller before the governance surface meant by "list",
		// so it keeps fake-list.json; `--label-any` is G3's question/risk
		// query and gets its OWN file with an empty default, because
		// falling back to fake-list.json would make every fixture in the
		// suite grow question beads it never declared.
		//
		// A create DOES land in the labeled listing though — real bd would
		// answer a label query with the bead it just filed, and a dedupe
		// that reads that query back (settleopen.go) is only pinnable
		// against a fake that does. The labels asked for are honoured, so
		// a create in another lane is not mistaken for a question.
		// The `list` mirror of fake-ready-fail: a repo that RESOLVES but
		// whose bd call fails (a locked database, a repo with no bd init).
		// UnresolvedDirs cannot see this one, so it is the shape a caller
		// that folds a failed scan into an empty result gets wrong silently
		// (ranger-base-ynim).
		if _, err := os.Stat("fake-list-fail"); err == nil {
			fmt.Fprint(os.Stderr, "database is locked")
			return 1
		}
		file, want, status := "fake-list.json", "", ""
		for i, a := range args {
			if a == "--label-any" {
				file = "fake-list-labeled.json"
				if i+1 < len(args) {
					want = args[i+1]
				}
			}
			if (a == "--status" || a == "-s") && i+1 < len(args) {
				status = args[i+1]
			}
		}
		body := "[]"
		if b, err := os.ReadFile(file); err == nil {
			body = string(b)
		}
		if want != "" {
			body = fakeBdFilterLabels(body, want)
		}
		// `--status` is honoured rather than ignored, because a caller that
		// reaches for it is NARROWING and a fake that serves the same rows
		// either way cannot show the narrowing (ranger-base-bwrp8). bd's
		// statuses are open, in_progress, blocked, deferred and closed, so
		// `--status open` is not "not closed": it drops the in_progress row
		// too, which is the wrong fix for a store class that lists closed
		// beads and the mutant OpenLabeledAny's pin has to kill.
		if status != "" {
			body = fakeBdFilterStatus(body, status)
		}
		// `--all` is real bd's own switch — "Show all issues including
		// closed (overrides default filter)" — and WITHOUT it a closed row
		// is not in the answer. The fake used to serve its files whole,
		// which made every open-vs-all distinction untestable: a dedupe
		// that reads only open beads and one that reads closed ones too
		// behaved identically against it, and that is exactly the defect
		// ranger-base-j8qmj is about (the merge-back handoff re-filed a
		// block that was closed do-not-land, every pass).
		// …unless a `fake-list-keep-closed` marker says this store is the
		// OTHER class. Measured 2026-09-04 on bd 0.50.3 (ranger-base-x9e34,
		// ciwatch_live_test.go): the shop's SQLite store drops closed rows
		// from `list --label-any` and the `no-db: true` JSONL store
		// `bd init` writes today keeps them — 391 of 396 `-l qa` beads are
		// closed and the SQLite store answers with 5. A dedupe that adopts
		// a closed bead never files again, and without this marker that
		// mutant survives every hermetic pin in the package.
		// An explicit `--status` is its own filter and overrides the
		// default one, `--all`'s way: measured on bd 0.50.3, `list
		// --label-any qa --status closed` answers with 397 closed rows on a
		// store whose bare `--label-any qa` answers with 5.
		if !hasArg(args, "--all") && status == "" {
			if _, err := os.Stat("fake-list-keep-closed"); err != nil {
				body = fakeBdDropClosed(body)
			}
		}
		fmt.Print(body)
		return 0
	case "blocked": // blocked --json → fake-blocked.json (the whole graph, one call)
		// The same shape fake-ready-fail gives `ready`, because Bd.Ready now
		// asks both questions and a store that cannot answer the second has
		// an unknown queue, not a ready one (ranger-base-lpz0o).
		if _, err := os.Stat("fake-blocked-fail"); err == nil {
			fmt.Fprint(os.Stderr, "database is locked")
			return 1
		}
		if b, err := os.ReadFile("fake-blocked.json"); err == nil {
			fmt.Print(string(b))
		} else {
			fmt.Print("[]")
		}
		return 0
	case "dep": // dep list <id> [--direction=up] --json → fake-deps.json / fake-dependents.json
		// `dep add <id> <blocker>` writes the edge into the same file the
		// list serves, because the caller that files one reads the graph
		// back rather than trusting the exit code (Bd.DepAdd), and a fake
		// that forgets the edge would make that read a false negative.
		for i, a := range args {
			if a == "add" && i+2 < len(args) {
				id, blocker := args[i+1], args[i+2]
				// bd's cycle rule as a SQLite beads.db enforces it — the
				// operator's queue, which is the store this fake models. It
				// spans every dependency type rather than only `blocks`
				// (ranger-base-23oo, measured): if the blocker already
				// carries an edge back to the issue — a `discovered-from`
				// written by the create that filed it, say — this add
				// closes a cycle and that store refuses it, exit 1. A fake
				// that granted it would let the suite pin a block real bd
				// will never make, which is exactly what ten green pins did.
				//
				// A `no-db: true` store ACCEPTS the same add and then keeps
				// the issue in `bd ready` while listing it in `bd blocked`
				// (ranger-base-lpz0o). The fake stays on the loud shape
				// because that is the live queue's, and the silent one is
				// covered where it bites: Bd.Ready subtracts `bd blocked`,
				// pinned in readyblocked_test.go off `fake-blocked.json`.
				if fakeBdReaches(blocker, id) {
					fmt.Fprintf(os.Stderr, "Error: cannot add dependency: would create a cycle (%s → %s → ... → %s)\n", id, blocker, id)
					return 1
				}
				// A `fake-dep-add-fail` marker is bd's own worst shape,
				// opt-in: exit 0 with nothing wrong on the wire and no edge
				// in the graph (the muoo class). It is what makes a caller
				// that reads the graph back distinguishable from one that
				// trusts the status.
				if _, err := os.Stat("fake-dep-add-fail"); err != nil {
					fakeBdAddDep(blocker)
					fakeBdAddEdge(id, blocker)
				}
				fmt.Print("{}")
				return 0
			}
		}
		file := "fake-deps.json"
		for _, a := range args {
			if a == "--direction=up" {
				file = "fake-dependents.json"
			}
		}
		if b, err := os.ReadFile(file); err == nil {
			fmt.Print(string(b))
		} else {
			fmt.Print("[]")
		}
		return 0
	case "create": // create <title> … --json → a fresh id, counted per fake dir
		id := fakeBdNextID()
		// bd's OTHER failure, and the one the poisoned shape must be told
		// apart from: the create fails having committed NOTHING, so the
		// handoff really does not exist and a caller that reads the graph
		// back must still say so.
		if _, err := os.Stat("fake-create-hard-fail"); err == nil {
			fmt.Fprint(os.Stderr, "Error: database is locked")
			return 1
		}
		// bd 0.49.1's non-atomic create (ranger-base-muoo), opt-in with a
		// fake-create-fail marker: against a parent whose dependency closure
		// is tangled the daemon COMMITS the issue and then outruns the
		// client's 30s socket read timeout, so bd exits 1, prints no id, and
		// the --deps edge never lands. The issue still shows up in the next
		// `bd list`, because in the wild it does — that is the whole reason
		// the flood was invisible to a dedupe that reads the edge.
		// An EMPTY marker poisons every create; a marker holding parent ids,
		// one per line, poisons only the creates whose `--deps` names one —
		// which is the shape the incident actually had. bd's timeout is
		// deterministic PER PARENT (it is the parent's dependency closure the
		// daemon walks), so a real pass files some closes and orphans others,
		// and only a mixed listing can pin that one poisoned close does not
		// cost the healthy ones their handoff.
		if fakeBdCreatePoisoned(args) {
			// The issue commits and the `--deps` edge does NOT — that is the
			// whole shape — so nothing goes into the graph here.
			fakeBdAppendCreated(id, args)
			fmt.Fprint(os.Stderr, "Error: failed to read response: read unix ->bd.sock: i/o timeout")
			return 1
		}
		// A create that SUCCEEDS lands in the store too — the next `bd list
		// --all` sees it, which is how a verify bead filed on one pass
		// dedupes the next one while the watermark still holds its close in
		// view (ranger-base-muoo).
		fakeBdAppendCreated(id, args)
		fakeBdRecordCreateDeps(id, args)
		fmt.Printf(`{"id":%q,"title":"created"}`, id)
		return 0
	case "comments": // comments <id> --json → fake-comments.json; comments add appends to it
		// `comments add` carries no --json, so neither failure marker above
		// reaches it — and a caller that must not act on a comment it could
		// not write (ci-watch closes a bead only after saying why,
		// ciwatch.go) is untestable without one of its own.
		if b, err := os.ReadFile("fake-comment-fail"); err == nil && hasArg(args, "add") {
			msg := strings.TrimSpace(string(b))
			if msg == "" {
				msg = "database is locked"
			}
			fmt.Fprint(os.Stderr, msg)
			return 1
		}
		// An added comment is READ BACK, because the settle-open count is a
		// comment the harness wrote on an earlier pass (settleopen.go) and
		// a fake that dropped it would make every pass look like the first.
		for i, a := range args {
			if a == "add" && i+2 < len(args) {
				fakeBdAddComment(args[i+1], args[i+2])
				fmt.Print("{}")
				return 0
			}
		}
		if b, err := os.ReadFile("fake-comments.json"); err == nil {
			fmt.Print(fakeBdCommentsFor(string(b), fakeBdID(args, "comments")))
		} else {
			fmt.Print("[]")
		}
		return 0
	case "show":
		if b, err := os.ReadFile("fake-show.json"); err == nil {
			fmt.Print(string(b))
		} else if st, ok := fakeBdState()[fakeBdID(args, "show")]; ok {
			fmt.Printf(`[{"id":%q,"title":"t","status":%q,"assignee":%q}]`, st.ID, st.Status, st.assignee())
		} else {
			fmt.Print("[]")
		}
		return 0
	case "update":
		return fakeBdUpdate(args)
	case "close":
		// A closed bead LEAVES the open listings, because that is what real
		// bd does and because a dedupe that reads `bd list --label-any`
		// back is only pinnable against a fake that does (the rule
		// fakeBdAppendCreated already keeps for create). Without it a
		// mechanism that closes its own bead and one that merely says it
		// did are indistinguishable — and ci-watch closes the bead it filed
		// (ciwatch.go), so the file-close-file cycle would have been
		// untestable.
		fakeBdMarkClosed(fakeBdID(args, "close"))
		fmt.Print("{}")
		return 0
	case "sync":
		// The launcher's pre-commit export (ADR 0015 §4, queuejsonl.go).
		// bd owns the export; what a test can pin is that posse asked for
		// the git-free form, and bd-calls.log above is where it reads that.
		//
		// ranger-base-p969's self-heal import: `--import-only` clears the
		// "ready" case's stale marker so the retry it provokes succeeds,
		// unless the marker itself says "keep" — the fixture for a db that
		// is still out of sync after the import bd actually ran.
		if hasArg(args, "--import-only") {
			if b, err := os.ReadFile("fake-ready-stale"); err == nil && strings.TrimSpace(string(b)) != "keep" {
				os.Remove("fake-ready-stale")
			}
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "fake bd: unhandled %s\n", strings.Join(args, " "))
	return 1
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// fakeBdCreatePoisoned reports whether this `bd create` is one the
// fake-create-fail marker says must time out after committing the issue. An
// empty marker means every create; a marker listing parent ids (one per
// line, blanks and `#` comments ignored) means only the creates whose
// `--deps` names one of them.
func fakeBdCreatePoisoned(args []string) bool {
	b, err := os.ReadFile("fake-create-fail")
	if err != nil {
		return false
	}
	var want []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			want = append(want, l)
		}
	}
	if len(want) == 0 {
		return true
	}
	deps, _ := fakeBdFlag(args, "--deps")
	for _, d := range strings.Split(deps, ",") {
		if i := strings.Index(d, ":"); i >= 0 {
			d = d[i+1:]
		}
		for _, w := range want {
			if strings.TrimSpace(d) == w {
				return true
			}
		}
	}
	return false
}

// fakeBdAppendCreated puts the issue bd just committed into fake-list.json,
// so the next pass's `bd list --all` sees it the way it would see a real one.
func fakeBdAppendCreated(id string, args []string) {
	flag := func(name string) string {
		for i, a := range args {
			if a == name && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	title := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--actor" {
			i++
			continue
		}
		if args[i] == "create" && i+1 < len(args) {
			title = args[i+1]
			break
		}
	}
	var list []map[string]any
	if b, err := os.ReadFile("fake-list.json"); err == nil {
		json.Unmarshal(b, &list)
	}
	row := map[string]any{
		"id": id, "title": title, "description": flag("-d"),
		"status": "open", "labels": strings.Split(flag("-l"), ","),
		// created_at, because real bd stamps one and a caller that ORDERS
		// the beads it reads back cannot be pinned against a fake that
		// leaves them all at the zero time — every such row sorts equal, so
		// the assertion holds whichever way the comparison points
		// (ranger-base-x9e34: ci-watch walks its candidates newest first,
		// and a green pass stops at the newest). Nanoseconds, so two
		// creates inside one test are ordered rather than tied; real bd's
		// own stamp is microseconds.
		"created_at": time.Now().Format(time.RFC3339Nano),
	}
	list = append(list, row)
	if b, err := json.Marshal(list); err == nil {
		os.WriteFile("fake-list.json", b, 0o644)
	}
	// And in the labeled listing, which `--label-any` filters: real bd
	// answers a label query with the bead it just filed, and a dedupe that
	// reads that query back is only pinnable against a fake that does.
	var labeled []map[string]any
	if b, err := os.ReadFile("fake-list-labeled.json"); err == nil {
		json.Unmarshal(b, &labeled)
	}
	labeled = append(labeled, row)
	if b, err := json.Marshal(labeled); err == nil {
		os.WriteFile("fake-list-labeled.json", b, 0o644)
	}
}

// fakeBdNextID hands out q-1, q-2, … so a test can assert on the id the
// harness comments back onto the closed bead.
// fakeBdFilterLabels keeps the issues carrying at least one of a comma-list
// of labels — `bd list --label-any`'s own contract, which the fake needs
// once creates land in the labeled listing.
func fakeBdFilterLabels(body, labels string) string {
	var list []map[string]any
	if json.Unmarshal([]byte(body), &list) != nil {
		return body
	}
	want := map[string]bool{}
	for _, l := range strings.Split(labels, ",") {
		want[strings.TrimSpace(l)] = true
	}
	kept := []map[string]any{}
	for _, is := range list {
		ls, _ := is["labels"].([]any)
		for _, l := range ls {
			if s, ok := l.(string); ok && want[s] {
				kept = append(kept, is)
				break
			}
		}
	}
	b, err := json.Marshal(kept)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// fakeBdMarkClosed sets a row's status to closed in both listings the fake
// serves, so `bd list` without `--all` stops answering with it.
func fakeBdMarkClosed(id string) {
	if id == "" {
		return
	}
	for _, f := range []string{"fake-list.json", "fake-list-labeled.json"} {
		var list []map[string]any
		b, err := os.ReadFile(f)
		if err != nil || json.Unmarshal(b, &list) != nil {
			continue
		}
		hit := false
		for _, is := range list {
			if s, _ := is["id"].(string); s == id {
				is["status"], hit = "closed", true
			}
		}
		if !hit {
			continue
		}
		if nb, err := json.Marshal(list); err == nil {
			os.WriteFile(f, nb, 0o644)
		}
	}
}

// fakeBdDropClosed is `bd list` WITHOUT `--all`: the closed rows are gone.
// Anything the fixture did not give a status is kept — a row that never
// said it was closed is not.
//
// ONE of a NAMED PAIR: fakeBdReadyDropClosed below is `ready`, and it is not
// a superset of this one to be consolidated into — see its comment for why
// the two questions are separate (ranger-base-pju9t). Deleting either leaves
// ITS OWN call site undefined and the package unbuildable — never the other's.
// Each name has exactly one caller (`grep -n fakeBd.*DropClosed` finds both),
// so there is no cross-reference to go looking for: cut this function and vet
// reports the `list` call site undefined; cut fakeBdReadyDropClosed and it
// reports the `ready` one. That is what 5b4e686 did to the `list` call site
// (`:245` at that commit) — restored by 6ecb521 (ranger-base-jzoci); this
// comment is the half of ranger-base-tenf5's fix that 6ecb521 did not carry,
// re-landed under ranger-base-d91mf and corrected under ranger-base-3fgmo.
//
// A comment is not a reader, and the FOLD this one argues against compiles:
// point `list` at fakeBdReadyDropClosed and the whole package still passed
// (internal/posse ok 668.961s, 0 FAIL, ranger-base-m4730). The reader is
// TestQAListAndReadyFakesAreNotOneFake.
func fakeBdDropClosed(body string) string {
	var list []map[string]any
	if json.Unmarshal([]byte(body), &list) != nil {
		return body
	}
	kept := []map[string]any{}
	for _, is := range list {
		if st, _ := is["status"].(string); st == "closed" {
			continue
		}
		kept = append(kept, is)
	}
	b, err := json.Marshal(kept)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// fakeBdFilterStatus keeps the rows whose own status field is exactly this
// one — `bd list --status <s>`'s contract, and the reason it is EXACT rather
// than "not closed" is the whole point of having it: a caller that reaches
// for `--status open` on a store that lists closed rows loses the
// in_progress and deferred ones as well, and a fake that ignored the flag
// would serve that mutant the right answer (ranger-base-bwrp8).
func fakeBdFilterStatus(body, status string) string {
	var list []map[string]any
	if json.Unmarshal([]byte(body), &list) != nil {
		return body
	}
	kept := []map[string]any{}
	for _, is := range list {
		if st, _ := is["status"].(string); st == status {
			kept = append(kept, is)
		}
	}
	b, err := json.Marshal(kept)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// fakeBdAddDep records `dep add <id> <blocker>` where `dep list <id>` will
// find it. The fake's dep files are per-repo rather than per-issue, which is
// enough for a caller that asks about the one bead it just filed against.
func fakeBdAddDep(blocker string) {
	var deps []map[string]any
	if b, err := os.ReadFile("fake-deps.json"); err == nil {
		json.Unmarshal(b, &deps)
	}
	deps = append(deps, map[string]any{"id": blocker, "dependency_type": "blocks"})
	if b, err := json.Marshal(deps); err == nil {
		os.WriteFile("fake-deps.json", b, 0o644)
	}
}

// ─── the fake's dependency GRAPH ─────────────────────────────────────────────
//
// fake-deps.json is the LISTING `dep list` serves, and it is per-repo rather
// than per-issue — enough for a caller asking about the one bead it just
// filed against, and useless for "does this blocker already reach me". The
// cycle rule needs the second question answered, so the edges bd is ASKED to
// write are recorded separately, keyed by the issue each one leaves, of any
// dependency type (ranger-base-23oo: bd's check spans them all).
//
// Only edges this fake wrote are in here. A fixture that seeds fake-deps.json
// by hand is stating what `dep list` returns, not what the graph holds, and
// does not make a later `dep add` a cycle.

// fakeBdEdges reads the graph: issue → the issues it points at.
func fakeBdEdges() map[string][]string {
	m := map[string][]string{}
	if b, err := os.ReadFile("fake-dep-edges.json"); err == nil {
		json.Unmarshal(b, &m)
	}
	return m
}

// fakeBdAddEdge records one edge, whatever its type.
func fakeBdAddEdge(from, to string) {
	m := fakeBdEdges()
	m[from] = append(m[from], to)
	if b, err := json.Marshal(m); err == nil {
		os.WriteFile("fake-dep-edges.json", b, 0o644)
	}
}

// fakeBdReaches reports whether from reaches to over any chain of edges, so
// `dep add <id> <blocker>` can ask bd's own question: does the blocker
// already reach the issue? An id reaches itself, which is how bd refuses a
// self-dependency too.
func fakeBdReaches(from, to string) bool {
	m := fakeBdEdges()
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(n string) bool {
		if n == to {
			return true
		}
		if seen[n] {
			return false
		}
		seen[n] = true
		for _, next := range m[n] {
			if walk(next) {
				return true
			}
		}
		return false
	}
	return walk(from)
}

// fakeBdRecordCreateDeps puts a create's `--deps` into the graph. Real bd
// writes those edges in the same breath as the issue, and a fake that dropped
// them — as this one did — cannot refuse the cycle they close.
func fakeBdRecordCreateDeps(id string, args []string) {
	deps, _ := fakeBdFlag(args, "--deps")
	for _, d := range strings.Split(deps, ",") {
		if i := strings.Index(d, ":"); i >= 0 {
			d = d[i+1:]
		}
		if d = strings.TrimSpace(d); d != "" {
			fakeBdAddEdge(id, d)
		}
	}
}

// fakeBdAddComment appends a comment where `bd comments <id>` will read it
// back. It carries its issue_id; fixtures written without one keep matching
// every issue, which is what the suite's older fixtures assume.
func fakeBdAddComment(id, text string) {
	var cs []map[string]any
	if b, err := os.ReadFile("fake-comments.json"); err == nil {
		json.Unmarshal(b, &cs)
	}
	cs = append(cs, map[string]any{"id": len(cs) + 1, "issue_id": id, "text": text, "author": "posse"})
	if b, err := json.Marshal(cs); err == nil {
		os.WriteFile("fake-comments.json", b, 0o644)
	}
}

// fakeBdCommentsFor serves one issue's comments: entries with no issue_id
// belong to every issue (the older fixtures), entries with one belong to it.
func fakeBdCommentsFor(body, id string) string {
	var cs []map[string]any
	if id == "" || json.Unmarshal([]byte(body), &cs) != nil {
		return body
	}
	kept := []map[string]any{}
	for _, c := range cs {
		if s, _ := c["issue_id"].(string); s == "" || s == id {
			kept = append(kept, c)
		}
	}
	b, err := json.Marshal(kept)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func fakeBdNextID() string {
	p := filepath.Join(fakeDir(), "bd-create-count")
	n := 0
	if b, err := os.ReadFile(p); err == nil {
		n, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	n++
	os.WriteFile(p, []byte(strconv.Itoa(n)), 0o644)
	return "q-" + strconv.Itoa(n)
}

// ─── the fake bd's claim state ───────────────────────────────────────────────

// The fake models bd 0.49.1's claim semantics, verified live against it
// (rangerhq-kux): a won claim prints the updated issue array and exits 0; a
// refused claim — including a re-claim by the holder itself — prints
// "already claimed by X" on STDERR and *also exits 0*, with empty stdout.
// State lives per repo in fake-state.json so `ready` and `show` agree with
// what the claims did, as real bd does.

type fakeBdIssue struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// nil = never set by an update; a set assignee (including "", which is
	// what Unclaim writes) overrides the canned list.
	Assignee *string `json:"assignee"`
}

func (i fakeBdIssue) assignee() string {
	if i.Assignee == nil {
		return ""
	}
	return *i.Assignee
}

// State is kept in the fake's own directory, keyed by the repo (bd's cwd),
// so it never lands in the source tree and dies with the test.
func fakeBdStatePath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(fakeDir(), "bd-state-"+strings.ReplaceAll(strings.TrimPrefix(cwd, "/"), "/", "_")+".json")
}

func fakeBdState() map[string]fakeBdIssue {
	st := map[string]fakeBdIssue{}
	if b, err := os.ReadFile(fakeBdStatePath()); err == nil {
		json.Unmarshal(b, &st)
	}
	return st
}

func fakeBdSaveState(st map[string]fakeBdIssue) {
	b, _ := json.Marshal(st)
	os.WriteFile(fakeBdStatePath(), b, 0o644)
}

// fakeBdID returns the first bare argument after the verb (the issue id).
func fakeBdID(args []string, verb string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == verb && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			return args[i+1]
		}
	}
	return ""
}

func fakeBdFlag(args []string, name string) (string, bool) {
	for i, a := range args {
		if a == name {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true
		}
	}
	return "", false
}

// fakeBdHolder is who the fake says holds a bead before any claim: the
// fake-claim-lost knob's contents (a persona name), else recorded state.
func fakeBdHolder(id string) string {
	if b, err := os.ReadFile("fake-claim-lost"); err == nil {
		if h := strings.TrimSpace(string(b)); h != "" {
			return h
		}
		return "someone-else"
	}
	return fakeBdState()[id].assignee()
}

func fakeBdUpdate(args []string) int {
	id := fakeBdID(args, "update")
	actor, _ := fakeBdFlag(args, "--actor")
	st := fakeBdState()
	if _, isClaim := fakeBdFlag(args, "--claim"); isClaim {
		// The legacy knob: bd failing loudly (non-zero) on a claim.
		if _, err := os.Stat("fake-claim-fail"); err == nil {
			fmt.Fprint(os.Stderr, "issue already claimed")
			return 1
		}
		if holder := fakeBdHolder(id); holder != "" {
			fmt.Fprintf(os.Stderr, "Error updating %s: operation failed: already claimed by %s\n", id, holder)
			return 0 // ← the bug this fake exists to reproduce
		}
		cur := st[id]
		cur.ID, cur.Assignee, cur.Status = id, &actor, "in_progress"
		st[id] = cur
		fakeBdSaveState(st)
		fmt.Printf(`[{"id":%q,"title":"t","status":"in_progress","assignee":%q}]`, id, actor)
		return 0
	}
	cur := st[id]
	cur.ID = id
	if v, ok := fakeBdFlag(args, "--status"); ok {
		cur.Status = v
	}
	if v, ok := fakeBdFlag(args, "--assignee"); ok {
		cur.Assignee = &v
	}
	st[id] = cur
	fakeBdSaveState(st)
	fmt.Print("{}")
	return 0
}

// fakeBdReadyDropClosed is bd's own first contract on `ready`, which this fake
// broke for the life of every test (ranger-base-y3x6n): a closed bead is not
// ready work, and no reading of the real store can hand one back.
//
// The fixture cannot read fake-show.json as "closed since the start" — it is
// the END state, written before the pass that is supposed to do the work, so
// filtering on it alone would drop every bead before it was ever dispatched.
// What says the work HAPPENED is the claim, which the fake already records
// (fakeBdUpdate). So: a bead this fake has handed to somebody AND whose show
// answers "closed" is done, and `ready` stops offering it. Both halves are
// read live, so a test that rewrites fake-show.json to un-close a bead, or
// unclaims it, gets it back in the queue.
//
// MEASURED (ranger-base-y3x6n): without this, a Run whose own sweep reaps a
// settled session — any Run slower than PromptGrace, which is every one of
// them under -race — got the same bead offered back by the next `ready`,
// found no live holder for it (its session had just been reaped) and
// RELAUNCHED it under ADR 0030 §1's recovery arm. A third `workspace create`
// for two beads, out of a store real bd would never have answered that way.
//
// TWO functions, not one, and NAMED apart on purpose (ranger-base-pju9t).
// `fakeBdDropClosed` above is `list` without `--all`: it drops a row whose
// own status field says closed and nothing else, which is what real bd's
// default filter does. This one is `ready`, and its extra half — a bead the
// fake has HANDED OUT whose `show` now answers closed — is a statement about
// dispatch, not about the store's filter. Merging them gives `list` that
// dispatch half, and against a fake that answers the open query and the
// `--all` query the same, the merge-back dedupe's two reads (OpenLabeledAny,
// AllLabeledAny) cannot be told apart — ranger-base-j8qmj's own defect,
// reachable again through the fixture.
//
// This paragraph used to end "the open-vs-`--all` distinction
// ranger-base-j8qmj's dedupe pins turn on". MEASURED TWICE, and they do not:
// with `list`'s call site pointed at this function, j8qmj's five dedupe pins
// pass unchanged (`go test -overlay`, ranger-base-90y3c, re-measured under
// ranger-base-ntuen: 5/5 PASS on the same overlay that reds the test below)
// and so does the
// whole package (internal/posse ok 668.961s, 0 FAIL, ranger-base-m4730). The
// dedupe CODE turns on the difference; no pin of j8qmj's reads it. What
// holds the pair apart is TestQAListAndReadyFakesAreNotOneFake
// (listreadyfakes_qa_test.go), which fails on the fold in EITHER direction —
// `list`'s call site given this function, and `ready`'s given
// fakeBdDropClosed (both measured, ranger-base-ntuen). A comment claiming a
// reader it does not have is how the other half of this pair got read as
// dead code and deleted (5b4e686). They collided as one name because
// two seats added one each to this file on branches neither could see, and
// merge-back is ff-only, so nothing built the pair until it was
// already on main: `go vet ./...` went red at 3075168 and CI's macos and
// ubuntu jobs both failed on this one line.
func fakeBdReadyDropClosed(list string) string {
	var issues []map[string]any
	if json.Unmarshal([]byte(list), &issues) != nil {
		return list
	}
	st, shown := fakeBdState(), fakeBdShownStatus()
	open := make([]map[string]any, 0, len(issues))
	for _, is := range issues {
		id, _ := is["id"].(string)
		status, _ := is["status"].(string)
		claimed := st[id].assignee() != ""
		if status == "closed" || (claimed && shown[id] == "closed") {
			continue
		}
		open = append(open, is)
	}
	if len(open) == len(issues) {
		return list
	}
	b, err := json.Marshal(open)
	if err != nil {
		return list
	}
	return string(b)
}

// fakeBdShownStatus is what this repo's `show` currently answers, by id.
func fakeBdShownStatus() map[string]string {
	b, err := os.ReadFile("fake-show.json")
	if err != nil {
		return nil
	}
	var issues []fakeBdIssue
	if json.Unmarshal(b, &issues) != nil {
		return nil
	}
	st := make(map[string]string, len(issues))
	for _, is := range issues {
		st[is.ID] = is.Status
	}
	return st
}

// fakeBdApplyState overlays recorded claim state on a canned issue list, so
// a second pass sees what the first pass's claims did.
func fakeBdApplyState(list string) string {
	st := fakeBdState()
	if len(st) == 0 {
		return list
	}
	var issues []map[string]any
	if json.Unmarshal([]byte(list), &issues) != nil {
		return list
	}
	for _, is := range issues {
		id, _ := is["id"].(string)
		cur, ok := st[id]
		if !ok {
			continue
		}
		if cur.Status != "" {
			is["status"] = cur.Status
		}
		if cur.Assignee != nil {
			is["assignee"] = *cur.Assignee
		}
	}
	b, err := json.Marshal(issues)
	if err != nil {
		return list
	}
	return string(b)
}

// ─── the fake herdr ──────────────────────────────────────────────────────────

type fakeWS struct {
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused"`
	AgentStatus string `json:"agent_status"`
}

// fakeDir is where the fake substrates keep their state. Read in the CHILD,
// which finds it from its own argv[0]: the parent links this test binary
// into the test's own directory and execs it through that link (fakeBinFor),
// so the per-test value travels by path and not through a process-wide
// environment variable that would make its setter serial (ADR 0047 D1).
// $RHQ_FAKE_DIR still overrides, which is what the handful of tests that
// deliberately redirect the fake's state — queuejsonl, createcpeh — use.
//
// A parent that wants the same directory asks fakeDirOf(t), not this.
func fakeDir() string {
	if d := os.Getenv("RHQ_FAKE_DIR"); d != "" {
		return d
	}
	d, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return filepath.Dir(os.Args[0])
	}
	return d
}

func fakeLoadWS() []fakeWS {
	var ws []fakeWS
	if b, err := os.ReadFile(filepath.Join(fakeDir(), "ws.json")); err == nil {
		json.Unmarshal(b, &ws)
	}
	return ws
}

func fakeSaveWS(ws []fakeWS) {
	b, _ := json.Marshal(ws)
	os.WriteFile(filepath.Join(fakeDir(), "ws.json"), b, 0o644)
}

// fakeHiddenFromList drops the ids named in the hidden-from-list file (one
// per line) from `workspace list` only — `workspace get` still answers for
// them. That is the rangerhq-9nso race in one lever: a listing snapshot
// taken before another process's workspace existed, against a server that
// holds it right now.
func fakeHiddenFromList(ws []fakeWS) []fakeWS {
	b, err := os.ReadFile(filepath.Join(fakeDir(), "hidden-from-list"))
	if err != nil {
		return ws
	}
	hidden := map[string]bool{}
	for _, id := range strings.Fields(string(b)) {
		hidden[id] = true
	}
	var out []fakeWS
	for _, w := range ws {
		if !hidden[w.WorkspaceID] {
			out = append(out, w)
		}
	}
	return out
}

func fakeOK(result string) int {
	fmt.Printf(`{"id":"fake","result":%s}`+"\n", result)
	return 0
}

func fakeErr(code, msg string) int {
	// Real herdr 0.8.0: "CLI server errors are JSON on stderr with exit
	// status 1." The incident on rangerhq-khc was that envelope — including
	// `"id":"cli:agent:prompt"` — on stderr, stdout empty. error-on-stderr
	// is the lever that reproduces it; the default stays on stdout so the
	// older tests keep exercising the envelope-on-stdout parse.
	if _, err := os.Stat(filepath.Join(fakeDir(), "error-on-stderr")); err == nil {
		fmt.Fprintf(os.Stderr, `{"error":{"code":%q,"message":%q},"id":"cli:agent:prompt"}`+"\n", code, msg)
		return 1
	}
	fmt.Printf(`{"error":{"code":%q,"message":%q}}`+"\n", code, msg)
	return 1
}

// fakeBarrierWait bounds the prompt barrier. It is a deadlock guard, not a
// budget for how fast a pass fires its prompts: a gathered pass clears the
// barrier as soon as the second prompt arrives, whatever the load, and only
// a genuinely serial one ever waits this long.
const fakeBarrierWait = 10 * time.Second

// fakeAwaitPrompts registers this prompt's arrival and blocks until n
// prompts are in flight at once. It reports how the prompt was released:
// "gathered" when the nth arrived, "timeout" when it gave up still alone.
func fakeAwaitPrompts(n int) string {
	dir := filepath.Join(fakeDir(), "prompt-arrivals")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())), nil, 0o644)
	deadline := time.Now().Add(fakeBarrierWait)
	for {
		// Counting arrivals, not readers: every prompt writes before it
		// looks, so a prompt can never miss itself and the count only grows.
		if ents, err := os.ReadDir(dir); err == nil && len(ents) >= n {
			return "gathered"
		}
		if time.Now().After(deadline) {
			return "timeout"
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// fakeRecordPromptWindow notes that a prompt was in flight from start until
// now and how it was released, in a file of this fake process's own.
func fakeRecordPromptWindow(start time.Time, release string) {
	dir := filepath.Join(fakeDir(), "prompt-windows")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d-%d", start.UnixNano(), os.Getpid())),
		[]byte(fmt.Sprintf("%d %d %s", start.UnixNano(), time.Now().UnixNano(), release)), 0o644)
}

// ─── the held leg's register ─────────────────────────────────────────────────
//
// Ending the LOOP does not end the leg (ranger-base-06bvw). The drain
// abandons an in-flight `agent prompt` on purpose — that is the whole point
// of it — but the abandoned child here is a forked posse.test whose
// RHQ_FAKE_DIR is the subtest's own t.TempDir. Left to itself it sleeps out
// the full delay, long after the subtest returned and t.TempDir removed the
// tree, and then MkdirAlls it BACK to write its window: same path, mode 0755
// instead of t.TempDir's 0700, one tree per abandoned leg per run. Cleanup
// was never failing; it was being undone afterwards, and half the 769 stale
// `Test*` trees in the operator's $TMPDIR (measured 2026-09-02, oldest Aug
// 30) were this one test's two subtests.
//
// So the fake keeps a register of the legs it is holding — one file per pid,
// created before the hold and removed only after the LAST write that leg
// makes into fakeDir — and takes a prompt-release file as "you may stop
// now". Together they let a subtest join its abandoned child instead of
// guessing at it: joinHeldPrompts.

func fakeHoldersDir() string { return filepath.Join(fakeDir(), "prompt-holders") }

// fakeHoldingPrompt runs one held leg — the hold and the window record —
// with this pid registered for the whole of it. The entry outlives the
// window write deliberately: an empty register has to mean "no fake is still
// writing into fakeDir", or the join it anchors is a sleep with extra steps.
func fakeHoldingPrompt(hold func() string) {
	os.MkdirAll(fakeHoldersDir(), 0o755)
	mine := filepath.Join(fakeHoldersDir(), strconv.Itoa(os.Getpid()))
	os.WriteFile(mine, nil, 0o644)
	defer os.Remove(mine)
	start := time.Now()
	fakeRecordPromptWindow(start, hold())
}

// fakeHeldDelay is prompt-delay-ms's hold: the whole delay, unless a joining
// test drops prompt-release first, in which case the leg ends now and says
// so. Polled rather than slept in one go purely so that lever exists — a
// real abandoned agent keeps running, and a fixture that copies that
// literally outlives the directory it writes into.
func fakeHeldDelay(d time.Duration) string {
	release := filepath.Join(fakeDir(), "prompt-release")
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(release); err == nil {
			return "released"
		}
		time.Sleep(5 * time.Millisecond)
	}
	return "delay"
}

func fakeHerdr(args []string) int {
	f, _ := os.OpenFile(filepath.Join(fakeDir(), "calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if f != nil {
		fmt.Fprintln(f, strings.Join(args, " "))
		f.Close()
	}
	if len(args) < 2 {
		return fakeErr("bad_request", "missing subcommand")
	}
	switch args[0] + " " + args[1] {
	case "workspace create":
		fakeProbeLaunchLock()
		// create-error (file) makes the workspace create fail — the lever
		// for the one relaunch failure no ordering can prevent, on the far
		// side of the kill (rangerhq-v52t). "code|message" like prompt-error.
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "create-error")); err == nil {
			code, msg, ok := strings.Cut(strings.TrimSpace(string(b)), "|")
			if !ok {
				msg = "fake herdr: workspace create refused"
			}
			return fakeErr(code, msg)
		}
		// create-delay-ms holds the create the way a loaded box does: the
		// stagger between fire(A) and fire(B) is this create plus a fake
		// fork. TestDispatchParallelPassGathersDespiteCreateStagger arms it
		// past the 500ms overlap budget that used to false-fail (rangerhq-3ig1).
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "create-delay-ms")); err == nil {
			if ms, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && ms > 0 {
				time.Sleep(time.Duration(ms) * time.Millisecond)
			}
		}
		label := ""
		for i := 2; i < len(args)-1; i++ {
			if args[i] == "--label" {
				label = args[i+1]
			}
		}
		ws := fakeLoadWS()
		// Ids are handed out from a counter, never from the live count: a
		// real herdr does not re-issue the id of a workspace it just closed,
		// and relaunch (create → close → create) would look like a no-op if
		// the fake did.
		id := fmt.Sprintf("w%d", fakeNextWSID())
		ws = append(ws, fakeWS{WorkspaceID: id, Label: label})
		fakeSaveWS(ws)
		return fakeOK(fmt.Sprintf(
			`{"type":"workspace_create","workspace":{"workspace_id":%q,"label":%q},"tab":{"tab_id":"%s:t1"},"root_pane":{"pane_id":"%s:p1"}}`,
			id, label, id, id))
	case "workspace list":
		fakeUnhideWhenLocked()
		// list-error (file) fails the listing itself — the read every seat
		// walk starts from, and the one failure whose blast radius is the
		// whole shop rather than one meta (ranger-base-3yqyg). "code|message"
		// like create-error.
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "list-error")); err == nil {
			code, msg, ok := strings.Cut(strings.TrimSpace(string(b)), "|")
			if !ok {
				msg = "fake herdr: workspace list refused"
			}
			return fakeErr(code, msg)
		}
		// Like real herdr, a workspace's agent_status mirrors the agent
		// detected in it (agents.json), unless the test set it directly.
		ws := fakeHiddenFromList(fakeLoadWS())
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "agents.json")); err == nil {
			var agents []struct {
				WorkspaceID string `json:"workspace_id"`
				AgentStatus string `json:"agent_status"`
			}
			json.Unmarshal(b, &agents)
			for i := range ws {
				if ws[i].AgentStatus != "" {
					continue
				}
				for _, ag := range agents {
					if ag.WorkspaceID == ws[i].WorkspaceID {
						ws[i].AgentStatus = ag.AgentStatus
					}
				}
			}
		}
		b, _ := json.Marshal(ws)
		return fakeOK(fmt.Sprintf(`{"type":"workspace_list","workspaces":%s}`, b))
	case "workspace get":
		// The per-id query the prune guard leans on (rangerhq-9nso): a real
		// herdr answers about one workspace now, not from a listing — so it
		// deliberately reads ws.json *before* hidden-from-list is applied.
		if _, err := os.Stat(filepath.Join(fakeDir(), "workspace-get-unreachable")); err == nil {
			return fakeErr("timeout", "no response from the herdr server")
		}
		id := args[2]
		fakeInterleave(id)
		for _, w := range fakeLoadWS() {
			if w.WorkspaceID == id {
				b, _ := json.Marshal(w)
				return fakeOK(fmt.Sprintf(`{"type":"workspace_info","workspace":%s}`, b))
			}
		}
		return fakeErr("workspace_not_found", "workspace "+id+" not found")
	case "workspace close":
		id := args[2]
		ws := fakeLoadWS()
		var kept []fakeWS
		found := false
		for _, w := range ws {
			if w.WorkspaceID == id {
				found = true
				continue
			}
			kept = append(kept, w)
		}
		if !found {
			return fakeErr("workspace_not_found", "workspace "+id+" not found")
		}
		fakeSaveWS(kept)
		return fakeOK(`{"type":"workspace_close"}`)
	case "workspace focus":
		id := args[2]
		for _, w := range fakeLoadWS() {
			if w.WorkspaceID == id {
				return fakeOK(`{"type":"workspace_focus"}`)
			}
		}
		return fakeErr("workspace_not_found", "workspace "+id+" not found")
	case "pane run":
		// pane-run-error (file) fails the typed command — a workspace that
		// came up but never started what it was for (rangerhq-v52t).
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "pane-run-error")); err == nil {
			code, msg, ok := strings.Cut(strings.TrimSpace(string(b)), "|")
			if !ok {
				msg = "fake herdr: pane run refused"
			}
			return fakeErr(code, msg)
		}
		// pane-run-starts-agent (file) makes the typed command "start" an
		// idle claude in that pane, as herdr would detect a moment later —
		// the relaunch lever (rangerhq-vk2).
		if _, err := os.Stat(filepath.Join(fakeDir(), "pane-run-starts-agent")); err == nil && len(args) > 2 {
			pane := args[2]
			ws := strings.SplitN(pane, ":", 2)[0]
			os.WriteFile(filepath.Join(fakeDir(), "agents.json"),
				[]byte(fmt.Sprintf(`[{"agent":"claude","agent_status":"idle","pane_id":%q,"workspace_id":%q}]`, pane, ws)), 0o644)
		}
		return fakeOK(`{"type":"pane_run"}`)
	case "pane read": // plain text, never an envelope — like the real CLI
		// pane-text/<pane id, ':' → '_'> is what THAT pane is showing, for
		// the tests that read a screen rather than a JSON field (the
		// permission-mode surface, ranger-base-vwgt). Per pane rather than
		// one file for the board: a listing that reads three panes has to be
		// able to get three different answers, which is the whole shape of
		// the three-valued field.
		if len(args) > 2 {
			if b, err := os.ReadFile(filepath.Join(fakeDir(), "pane-text", strings.ReplaceAll(args[2], ":", "_"))); err == nil {
				fmt.Print(string(b))
				return 0
			}
		}
		if _, err := os.Stat(filepath.Join(fakeDir(), "pane-read-error")); err == nil {
			return fakeErr("pane_not_found", "fake herdr: pane read refused")
		}
		fmt.Print("prompt$ echo hi\nhi\nprompt$\n\n\n\n")
		return 0
	case "pane get":
		// The runtime's own session id for the pane (ranger-base-2hvtv).
		// Absent by default — herdr answers this only for a pane it has
		// identified an agent in, and every reading built on it has to
		// survive not getting one.
		//
		//	pane-session   the claude session uuid to report
		//	pane-agent     the agent kind, for the non-claude arm (default claude)
		if len(args) < 3 {
			return fakeErr("bad_request", "fake herdr: pane get needs a pane id")
		}
		b, err := os.ReadFile(filepath.Join(fakeDir(), "pane-session"))
		if err != nil {
			return fakeErr("not_found", "fake herdr: no agent session on that pane")
		}
		agent := "claude"
		if k, err := os.ReadFile(filepath.Join(fakeDir(), "pane-agent")); err == nil {
			agent = strings.TrimSpace(string(k))
		}
		return fakeOK(fmt.Sprintf(
			`{"type":"pane_info","pane":{"pane_id":%q,"agent":%q,`+
				`"agent_session":{"agent":%q,"kind":"id","source":"herdr:%s","value":%q}}}`,
			args[2], agent, agent, agent, strings.TrimSpace(string(b))))
	case "agent list":
		agents := "[]"
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "agents.json")); err == nil {
			agents = string(b)
		}
		return fakeOK(fmt.Sprintf(`{"type":"agent_list","agents":%s}`, agents))
	case "agent prompt": // real shape: result.agent.agent_status
		// agents-on-prompt (file) replaces the agent listing the moment the
		// prompt is handled — what a session looks like when detection
		// changes between the prompt and the status check that judges it
		// (rangerhq-khc).
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "agents-on-prompt")); err == nil {
			os.WriteFile(filepath.Join(fakeDir(), "agents.json"), b, 0o644)
		}
		// wait-error-on-prompt (file) arms `agent wait` to fail from the
		// moment the prompt is handled — the herdr handoff shape
		// (ranger-base-7t4): the prompt landed against the old server, the
		// server was replaced under it, and the *re-wait* leg is the call
		// that fails. Arming it here rather than up front leaves the
		// launch's own `agent wait` (awaitSettled) working, which is what
		// makes the bead claimed and the unclaim reachable at all.
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "wait-error-on-prompt")); err == nil {
			os.WriteFile(filepath.Join(fakeDir(), "wait-error"), b, 0o644)
		}
		// prompt-error (file) makes the prompt fail like a herdr --wait
		// timeout / agent_not_ready — the QA suite's error-path lever.
		// "code|message" spells the error herdr actually returned; a bare
		// code keeps the generic message.
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "prompt-error")); err == nil {
			code, msg, ok := strings.Cut(strings.TrimSpace(string(b)), "|")
			if !ok {
				msg = "fake herdr: prompt refused"
			}
			return fakeErr(code, msg)
		}
		// Two levers hold a --wait prompt in flight, and both record the
		// interval they held it (one file per fake process, so a test
		// reading them back cannot race a writer):
		//
		// prompt-barrier (file, a count N) blocks every prompt until N of
		// them have arrived, then releases them together — a rendezvous
		// barrier. It is what makes "the pass gathers" assertable at any
		// load: a gathered dispatcher reaches N whatever the machine is
		// doing, and a serial one can never reach it, so each of its
		// prompts leaves alone on the barrier's timeout. Budgeting a
		// wall-clock margin instead false-failed a correct dispatcher
		// twice (rangerhq-g6lx, rangerhq-3ig1).
		//
		// prompt-delay-ms (file) is the blunter one: sleep that long, then
		// settle. It is for tests that want a prompt merely slow, not
		// synchronised — the launch lock's held prompt (launchlock_test.go).
		//
		// The barrier counts every prompt this fake dir has seen, so it
		// releases one group of N and every later prompt walks through:
		// arm it only where the test also pins the prompt count.
		//
		// Both holds run under fakeHoldingPrompt, which registers this pid
		// for as long as the leg can still write into fakeDir — see the
		// register above. Barrier legs are registered too even though only
		// the delay one has a release lever: a joining test must be able to
		// see everything the fake is holding, not only what it can end.
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "prompt-barrier")); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && n > 0 {
				fakeHoldingPrompt(func() string { return fakeAwaitPrompts(n) })
			}
		} else if b, err := os.ReadFile(filepath.Join(fakeDir(), "prompt-delay-ms")); err == nil {
			if ms, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				fakeHoldingPrompt(func() string { return fakeHeldDelay(time.Duration(ms) * time.Millisecond) })
			}
		}
		return fakeOK(`{"type":"agent_prompted","agent":{"agent_status":"idle"}}`)
	case "agent wait": // settles idle unless wait-status overrides (timeout path)
		// wait-error (file) fails the wait leg itself — what a server that
		// went away under an in-flight prompt returns. "code|message" like
		// prompt-error.
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "wait-error")); err == nil {
			code, msg, ok := strings.Cut(strings.TrimSpace(string(b)), "|")
			if !ok {
				msg = "fake herdr: wait refused"
			}
			return fakeErr(code, msg)
		}
		return fakeOK(fmt.Sprintf(`{"type":"agent_wait","agent":{"agent_status":%q}}`, fakeWaitStatus()))
	case "agent explain": // a BARE object, like the real `explain --json`
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "explain-error")); err == nil && fakeExplainErrorArmed() {
			code, msg, ok := strings.Cut(strings.TrimSpace(string(b)), "|")
			if !ok {
				msg = "fake herdr: cannot explain"
			}
			return fakeErr(code, msg)
		}
		fmt.Println(fakeExplain())
		return 0
	case "agent send-keys":
		// What the keypress does to the screen, as a lever:
		//   send-keys-clears  the screen goes away and the agent settles idle
		//   send-keys-rule    a different screen is underneath it
		//   neither           the screen is still drawn afterwards — grok's
		//                     splash, whose composer takes the prompt anyway
		if _, err := os.Stat(filepath.Join(fakeDir(), "send-keys-clears")); err == nil {
			os.WriteFile(filepath.Join(fakeDir(), "wait-status"), []byte("idle"), 0o644)
			os.Remove(filepath.Join(fakeDir(), "explain-rule"))
		} else if b, err := os.ReadFile(filepath.Join(fakeDir(), "send-keys-rule")); err == nil {
			os.WriteFile(filepath.Join(fakeDir(), "explain-rule"), b, 0o644)
		}
		return fakeOK(`{"type":"agent_send_keys"}`)
	}
	return fakeErr("bad_request", "fake herdr: unhandled "+strings.Join(args, " "))
}

// fakeExplainErrorArmed reports whether the explain-error lever is live on
// THIS call. The two countdowns place the failure at either end of a window:
//
//	explain-error-after N  answer N explains normally, then error from the
//	                       next one on — a herdr that went away mid-window
//	                       (rangerhq-lhy2), early polls answered, a late one
//	                       not.
//	explain-error-for N    error the FIRST N explains, then answer normally
//	                       from N+1 on — the other ordering: herdr away at
//	                       the start of the window and back for the rest of
//	                       it (ranger-base-3wc7). It is the arm that keeps a
//	                       fix from being "an error anywhere in the window
//	                       wins", so lastErr must not outlive the guesses
//	                       that followed it.
//
// explain-error-for wins if both are set; there is no ordering that wants
// both, and silently composing two countdowns would make either one's number
// mean something other than what it says.
//
// Both count CALLS because the only other way to place an error at one end
// of the window is a wall clock, and a wall-clock timer races the launch's
// own setup: the first `agent explain` of a fake-herdr launch lands ~305ms
// after the test body starts on an idle box (measured 2026-08-29, 10 runs,
// spread 293-340ms) and later than that on a loaded one. At the late end
// that made "arm it at 700ms, after some guesses" silently mean "arm it
// before the first guess", and the test measured the opposite window
// (ranger-base-4pjw, ~1 red in 3 on the operator's box). At the early end
// the same race is silent instead of red: a 300ms timer that removed
// explain-error beat the first explain 12 times out of 12, so the window
// held no error at all and the test measured nothing (ranger-base-3wc7).
// Each fake call is its own process, so the count lives in the file, the way
// explain-fallback's does.
func fakeExplainErrorArmed() bool {
	// Only a countdown that has RUN OUT disarms: an unreadable number leaves
	// the error armed, the way explain-error-after's does, so a typo in a
	// fixture cannot quietly turn explain-error off and make a test that
	// asserts an absence pass for the wrong reason.
	forP := filepath.Join(fakeDir(), "explain-error-for")
	if b, err := os.ReadFile(forP); err == nil {
		n, numErr := strconv.Atoi(strings.TrimSpace(string(b)))
		if numErr != nil {
			return true
		}
		if n <= 0 {
			return false // the countdown is spent: herdr is back
		}
		os.WriteFile(forP, []byte(strconv.Itoa(n-1)), 0o644)
		return true
	}
	p := filepath.Join(fakeDir(), "explain-error-after")
	b, err := os.ReadFile(p)
	if err != nil {
		return true
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return true
	}
	os.WriteFile(p, []byte(strconv.Itoa(n-1)), 0o644)
	return false
}

// fakeExplain answers `agent explain --json` in one of the two shapes a real
// herdr emits (rangerhq-3hb5), because the readiness gate is built on the
// difference. By default the state is SEEN — a named rule, the visible_*
// flag, no fallback reason — which is what any settled pane looks like.
//
// Levers:
//
//	explain-rule      the rule id that produced the state (default fake_<state>)
//	explain-state     the state `explain` reports, when it must differ from
//	                  the one `agent wait` settled on — the two are one
//	                  reading in herdr, but sampled at different instants
//	explain-fallback  answer with herdr's GUESS instead: matched_rule null,
//	                  visible_idle false, default_known_agent_idle_fallback.
//	                  A number counts down — that many guesses, then seen,
//	                  which is the boot race. Empty means guess forever.
//	explain-rules     a raw JSON array spliced in as `evaluated_rules` —
//	                  herdr's own working, on BOTH shapes (a real herdr
//	                  evaluates every rule whatever matched, and the region
//	                  previews in those entries are what panework.go reads).
//	                  Absent means the key is absent, which is what an older
//	                  herdr emits and what WhatHerdrSaw must survive.
//	explain-error     `explain` fails outright (see the fake's error lever)
//	explain-error-after
//	                  a countdown: that many explains are answered before
//	                  explain-error arms, for a herdr that goes away partway
//	                  through the window
//	explain-error-for a countdown the other way: that many explains fail
//	                  before explain-error disarms, for a herdr that is away
//	                  at the start of the window and back for the rest of it
func fakeExplain() string {
	state := fakeWaitStatus()
	if b, err := os.ReadFile(filepath.Join(fakeDir(), "explain-state")); err == nil {
		state = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(fakeDir(), "explain-fallback")); err == nil {
		n, numErr := strconv.Atoi(strings.TrimSpace(string(b)))
		counted := numErr == nil
		if !counted || n > 0 { // no count means guess forever
			if counted {
				os.WriteFile(filepath.Join(fakeDir(), "explain-fallback"), []byte(strconv.Itoa(n-1)), 0o644)
			}
			return fmt.Sprintf(`{"state":%q,"matched_rule":null,"visible_idle":false,`+
				`"visible_blocker":false,"visible_working":false,`+
				`"fallback_reason":"default_known_agent_idle_fallback"%s}`, state, fakeExplainRules())
		}
	}
	rule := "fake_" + state
	if b, err := os.ReadFile(filepath.Join(fakeDir(), "explain-rule")); err == nil {
		rule = strings.TrimSpace(string(b))
	}
	return fmt.Sprintf(`{"state":%q,"matched_rule":{"id":%q,"state":%q},`+
		`"visible_idle":%t,"visible_blocker":%t,"visible_working":%t,`+
		`"fallback_reason":null%s}`,
		state, rule, state, state == "idle", state == "blocked", state == "working", fakeExplainRules())
}

// fakeExplainRules splices the explain-rules lever in, as the `,"..."` tail
// of either shape. It rides on BOTH of them since ranger-base-htafy: herdr
// evaluates every manifest rule whatever matched, and the screen regions
// those entries preview are what says whether a settled pane is waiting on
// its own background work or holding an unsent prompt (panework.go). Absent
// means the key is absent — an older herdr, and what the readings must
// survive.
func fakeExplainRules() string {
	b, err := os.ReadFile(filepath.Join(fakeDir(), "explain-rules"))
	if err != nil {
		return ""
	}
	return `,"evaluated_rules":` + strings.TrimSpace(string(b))
}

// fakeWaitStatus is the state the fake settles on: idle unless the
// wait-status lever says otherwise. `agent explain` reports the same state,
// because in a real herdr they are one reading.
func fakeWaitStatus() string {
	if b, err := os.ReadFile(filepath.Join(fakeDir(), "wait-status")); err == nil {
		return strings.TrimSpace(string(b))
	}
	return "idle"
}

// fakeInterleave is rangerhq-3a5t's race harness: a lever that makes
// another process write a file at the exact instant the prune's per-id
// query is answered — i.e. inside the window between prunable() proving a
// workspace dead and the unlink acting on that proof. The fake herdr is a
// separate process (the test binary re-execs it), so this is a real
// concurrent write and not a hook the code under test knows about.
//
// The lever is `interleave-write` in the fake dir: line 1 the workspace id
// to fire on, line 2 the path to write, the rest the content. It fires once
// and removes itself, so a re-check under the lock sees a settled world
// rather than a race that can never end.
func fakeInterleave(id string) {
	p := filepath.Join(fakeDir(), "interleave-write")
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	parts := strings.SplitN(string(b), "\n", 3)
	if len(parts) < 3 || parts[0] != id {
		return
	}
	os.Remove(p)
	os.MkdirAll(filepath.Dir(parts[1]), 0o755)
	os.WriteFile(parts[1], []byte(parts[2]), 0o644)
}

// fakeProbeLaunchLock answers, from the fake herdr's own process, whether
// the launcher lock was held while a workspace create was in flight. It has
// to be asked from another process to mean anything: flock is per open file
// description, so the creating process asking itself would always find it
// free. The lever is `probe-launch-lock` holding the lock path; the answer
// lands in `launch-lock-probe` as "held" or "free".
func fakeProbeLaunchLock() {
	b, err := os.ReadFile(filepath.Join(fakeDir(), "probe-launch-lock"))
	if err != nil {
		return
	}
	ans := "held"
	f, err := os.OpenFile(strings.TrimSpace(string(b)), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		// Never "held": a lock file this probe cannot even open is a probe
		// that measured nothing, and reporting that as contention would let
		// a create that takes no lock at all pass for one that does.
		ans = "unknown: " + err.Error()
	} else {
		if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
			ans = "free"
			syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		}
		f.Close()
	}
	os.WriteFile(filepath.Join(fakeDir(), "launch-lock-probe"), []byte(ans), 0o644)
}

// fakeUnhideWhenLocked is ranger-base-rrg2's lever: a workspace hidden from
// `workspace list` becomes visible the moment the launcher lock is HELD.
//
// That is the window the preflight cannot cover, planted from outside the
// process under test: a listing read before the lock does not show the
// workspace, and every listing the destructive tail takes does. It is keyed
// to the lock rather than to a call count on purpose — a count is a fixture
// about how many listings each phase happens to take today, and would go
// green (or red) on any change to either that has nothing to do with the
// race.
//
// Held is measured the way fakeProbeLaunchLock measures it, and for the same
// reason: flock is per open file description, so only another process can
// tell contended from free.
//
// The lever is `unhide-when-locked` holding the lock path; it fires once and
// takes `hidden-from-list` with it, so the world it reveals is settled.
func fakeUnhideWhenLocked() {
	p := filepath.Join(fakeDir(), "unhide-when-locked")
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	f, err := os.OpenFile(strings.TrimSpace(string(b)), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return // measured nothing; leave the plant hidden rather than guess
	}
	defer f.Close()
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		return // free: the pass is still in its preflight
	}
	os.Remove(p)
	os.Remove(filepath.Join(fakeDir(), "hidden-from-list"))
}

// fakeNextWSID hands out w1, w2, … per fake dir, monotonically.
func fakeNextWSID() int {
	p := filepath.Join(fakeDir(), "ws-counter")
	n := 0
	if b, err := os.ReadFile(p); err == nil {
		n, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	n++
	os.WriteFile(p, []byte(strconv.Itoa(n)), 0o644)
	return n
}

// ─── harness ─────────────────────────────────────────────────────────────────

// fakeDirs is the per-test fake-substrate state directory, keyed on the
// *testing.T itself. It replaces the $RHQ_FAKE_DIR the parent used to read
// back out of its own environment: nothing process-global, so the setter
// keeps t.Parallel. A test that builds two backends gets the second one's
// directory from fakeDirOf, which is what reading $RHQ_FAKE_DIR gave too.
//
// The KEY is the T and not t.Name() because `go test -count=2` runs the same
// name twice, and with t.Parallel both copies are resumed together: two live
// tests, one key, and each one's Cleanup deleting the other's entry. Measured
// as five FAILs under -count=3 that were green at -count=1 (ranger-base-pj87l).
// The pointer is unique per run by construction, so -count is a non-question.
var fakeDirs sync.Map

// hermeticRun numbers the calls to hermetic so each RUN of a test gets its
// own worktree root — see the comment at the assignment.
var hermeticRun atomic.Int64

func setFakeDir(t *testing.T, dir string) {
	t.Helper()
	fakeDirs.Store(t, dir)
	t.Cleanup(func() { fakeDirs.Delete(t) })
}

// fakeDirOf is this test's fake-substrate state directory — the one
// newTestBackend made, or a fresh one for a test that drives a fake without
// building a backend.
func fakeDirOf(t *testing.T) string {
	t.Helper()
	if v, ok := fakeDirs.Load(t); ok {
		return v.(string)
	}
	dir := t.TempDir()
	setFakeDir(t, dir)
	return dir
}

// fakeBinFor links this test binary into the test's own fake dir under name
// and returns the link. Exec'd through it, the child's argv[0] is inside
// that directory, which is how fakeDir finds per-test state with nothing
// exported (ADR 0047 D1). The name is for the reader and for `ps`: TestMain
// dispatches on the verb, not on argv[0].
func fakeBinFor(t *testing.T, name string) string {
	t.Helper()
	bin := filepath.Join(fakeDirOf(t), name)
	if _, err := os.Lstat(bin); err == nil {
		return bin
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, bin); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	return bin
}

func newTestBackend(t *testing.T) (*HerdrBackend, string) {
	t.Helper()
	home := t.TempDir()
	fake := t.TempDir()
	setFakeDir(t, fake)
	b := &HerdrBackend{App: hermetic(t, NewAppAt(home)), H: Herdr{Bin: fakeBinFor(t, "herdr")}}
	captureWarn(t, b)
	return b, fake
}

// hermetic gives an App the two fake-by-construction defaults every launch
// path in a test needs, like RHQ_FAKE_HERDR above. Both fields are nil on a
// real App and nil means the operator's own box (app.go), so a test that
// leaves them there measures the machine the suite happens to be running on
// and is red per-day, not per-commit.
//
//   - ModelLister: an unconfigured lister reads no keychain and reaches no
//     network, and the availability preflight takes that as UNKNOWN and
//     launches the tier exactly as asked (modelavail.go). Tests that want
//     the preflight to DO something seed the snapshot or set this field.
//   - Load1: the load guard (loadguard.go) otherwise reads this box's
//     1-minute load average, and refuses every launch in the test when it
//     is over `load_guard:`. Quiet by construction; the tests that want the
//     guard to fire set this field.
//   - TopCPU: the guard's culprit line otherwise forks `ps` and names
//     whatever the suite's own machine is running. An empty table renders
//     nothing, so a refusal in a test says only what the test set up; the
//     tests that want culprits named supply rows.
//   - WorktreeRootDefault: nil is ~/.posse/worktrees, ONE directory for the
//     whole binary now that $HOME is per binary (TestMain). SessionTreePath
//     is <root>/<repo basename>/<session>, every test repo is a t.TempDir
//     whose first basename is 001, and three tests launch a session named
//     `crew` — so one shared root is one worktree path for three different
//     repos (ADR 0047 §1). Per test it is per path. It stays UNDER $HOME so
//     WorktreeRoot's one placement rule still runs on it for real.
//
// It is a named function rather than four lines inside newTestBackend
// because a test that builds its OWN App and swaps it behind the backend
// silently drops every default installed here — which is what left the
// promote/launch rehearsal reading the live loadavg (ranger-base-w4fb).
// Adopt an App with this, and adding the next default covers both sites.
func hermetic(t *testing.T, a *App) *App {
	t.Helper()
	a.ModelLister = &ModelLister{}
	a.Load1 = func() (float64, error) { return 0, nil }
	a.TopCPU = func() ([]Proc, error) { return nil, nil }
	// Under whatever $HOME is current: the binary's temp home for almost
	// every test, and a test that set its own (wtqaHome, wtApp) gets one
	// under that instead — either way the under-$HOME rule holds by
	// construction rather than by waiver.
	// Per RUN and not per NAME: `go test -count=2` gives two live tests one
	// name, and with t.Parallel they are resumed together — the second finds
	// the first's session tree already cut and reports "exists and is not a
	// git worktree", which is a fixture collision wearing a product refusal's
	// clothes. Measured as five FAILs at -count=3 that were green at
	// -count=1 (ranger-base-pj87l). Repeat runs are how this shop measures a
	// flake rate, so the mode has to work.
	a.WorktreeRootDefault = filepath.Join(os.Getenv("HOME"), "worktrees",
		t.Name(), strconv.FormatInt(hermeticRun.Add(1), 10))
	return a
}

// captureWarn points a test backend's warning stream at a per-test buffer.
// A nil Warn is os.Stderr (herdrback.go), and for a test binary that is two
// harms at once (ranger-base-ihd2):
//
//   - Blind assertions. planLaunch passes b.warnWriter() to
//     EnsureSessionTree, TierPreflight and LoadHigh, so a test asserting on
//     the writer it handed CreateSession/RelaunchSession reads a stream
//     those lines never enter — the guard cannot fail, and the regression it
//     names lands green (measured on ranger-base-ljiu).
//   - Misattributed failures. `go test` suppresses a passing package's
//     output and dumps the whole buffered stream only when the package
//     fails, so a passing test's warning surfaces directly above whichever
//     OTHER test failed. ranger-base-ljiu was filed off exactly that shape.
//
// io.Discard would fix the second and make the first worse. The buffer is
// dumped through t.Logf on failure, which attributes it to the test that
// provoked it. A test that wants to READ the stream asks warnBuf, or sets
// its own Warn.
func captureWarn(t *testing.T, b *HerdrBackend) {
	t.Helper()
	buf := &syncBuf{}
	b.Warn = buf
	t.Cleanup(func() {
		if t.Failed() {
			if s := buf.String(); s != "" {
				t.Logf("b.Warn (the launch's warning stream):\n%s", s)
			}
		}
	})
}

// warnBuf is the buffer captureWarn gave b.Warn, for a test that wants to
// read the warning stream without holding one of its own.
func warnBuf(t *testing.T, b *HerdrBackend) *syncBuf {
	t.Helper()
	buf, ok := b.Warn.(*syncBuf)
	if !ok {
		t.Fatalf("b.Warn is %T, not the buffer the harness gave it", b.Warn)
	}
	return buf
}

// gatePrefixRe is the L1 prefix on every typed persona line: ADR 0002 §3's
// PATH=<gates bin>, and ADR 0009 §2's SHELL/GROK_SHELL pointing at the gate
// shell (absent only for a Runtime.NoGateShell runtime).
var gatePrefixRe = regexp.MustCompile(`PATH='[^']*':"\$PATH" (SHELL='[^']*' GROK_SHELL='[^']*' )?`)

// calls returns the fake herdr's call log with each gate prefix collapsed
// to the marker `GATES `, so assertions can name the persona command that
// follows it without spelling out two absolute paths. GatePrefix itself is
// asserted verbatim in gates_test.go.
func calls(t *testing.T, fake string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(fake, "calls.log"))
	if !gatePrefixRe.Match(b) && strings.Contains(string(b), `:"$PATH" `) {
		t.Errorf("a typed persona line is missing its gate prefix:\n%s", b)
	}
	return gatePrefixRe.ReplaceAllString(string(b), "GATES ")
}

// launchLog is the typed calls log PLUS every launch line that was spilled
// rather than typed. The rendered line lands in one of TWO places and which
// one is a fact about its LENGTH, not about the launch: over PaneLineMax it
// is written to state/launch/<session>.sh and the pane types `. <script>`
// instead (paneline.go). A pin that reads only calls.log therefore measures
// how long the line is rather than what is on it — the trap
// dispatchparity_qa_test.go already names, and the one ranger-base-rq83c
// walked six fixtures into when ~110 bytes of credential-dir pin moved them
// across the cliff.
//
// Every script in the dir, not a named session: the caller is asserting on
// what a launch put on a line, and which session spilled is the accident
// this helper exists to stop mattering.
func launchLog(t *testing.T, a *App, fake string) string {
	t.Helper()
	out := calls(t, fake)
	ents, err := os.ReadDir(a.LaunchDir())
	if err != nil {
		return out // nothing spilled: every line was typed
	}
	for _, e := range ents {
		p := filepath.Join(a.LaunchDir(), e.Name())
		body, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		out += "\n--- " + p + " ---\n" + string(body)
	}
	return out
}

// promptWindow is one held `agent prompt`: the interval it was in flight,
// in nanoseconds, and how it was released — "gathered" when the prompt
// barrier reached its count, "timeout" when that prompt was still the only
// one in flight when the barrier gave up, "delay" for a plain
// prompt-delay-ms sleep.
type promptWindow struct {
	start, end int64
	release    string
}

// promptWindows returns one window per `agent prompt` the fake held —
// prompt-barrier or prompt-delay-ms. Each fake process writes its own file,
// so reading them back cannot race the writers.
func promptWindows(t *testing.T, fake string) []promptWindow {
	t.Helper()
	dir := filepath.Join(fake, "prompt-windows")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []promptWindow
	for _, e := range ents {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var w promptWindow
		if _, err := fmt.Sscanf(string(b), "%d %d %s", &w.start, &w.end, &w.release); err != nil {
			t.Fatalf("bad prompt window %q: %v", b, err)
		}
		out = append(out, w)
	}
	return out
}

// joinWait bounds every leg of joinHeldPrompts. Generous for the same
// reason waitForOut's is: each held leg is a forked test binary, and under
// -race that fork is tens of seconds slow before it reaches the register.
const joinWait = 60 * time.Second

// joinHeldPrompts ends the `agent prompt` legs the fake is holding and waits
// until they are gone, so the subtest's own t.TempDir cleanup is the LAST
// writer to its tree (ranger-base-06bvw). want is how many legs the fixture
// put in flight.
//
// Waiting for them to REGISTER is half the join and not optional: the parent
// prints its "prompted" line before `go AgentPrompt` has forked anything, so
// a register read at cancel time can be empty for a leg that is merely still
// starting — and an empty register is exactly what "all done" looks like.
// Only after `want` of them have been seen is emptiness worth anything.
//
// It joins in three steps, and each one answers a different half of "a green
// test leaves neither a process nor a directory behind":
//
//  1. every leg has registered;
//  2. the register is empty again — no fake is still writing into fake;
//  3. every pid seen is actually gone. The register entry goes first and the
//     exit follows within a syscall or two (TestMain os.Exits on fakeHerdr's
//     return), and os/exec's abandoned Wait is still blocked on the child, so
//     it reaps: ESRCH here is a real exit and not a zombie reading as one.
func joinHeldPrompts(t *testing.T, fake string, want int) {
	t.Helper()
	held := func() []int {
		ents, _ := os.ReadDir(filepath.Join(fake, "prompt-holders"))
		pids := []int{}
		for _, e := range ents {
			if pid, err := strconv.Atoi(e.Name()); err == nil {
				pids = append(pids, pid)
			}
		}
		return pids
	}
	deadline := time.Now().Add(joinWait)
	// An arm that has already failed has no leg worth waiting on — it may
	// never have got one in flight at all — so it does not pay the budget,
	// and it does not get a second, derived failure below either.
	seen := map[int]bool{}
	for len(seen) < want && !t.Failed() && time.Now().Before(deadline) {
		for _, pid := range held() {
			seen[pid] = true
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Steps 2 and 3 get the budget fresh. A register that was slow to fill
	// must not starve the join it exists to anchor — and if step 1 timed
	// out, the legs that DID register still deserve ending.
	os.WriteFile(filepath.Join(fake, "prompt-release"), nil, 0o644)
	deadline = time.Now().Add(joinWait)
	for len(held()) > 0 && time.Now().Before(deadline) {
		// Late arrivals count too: a leg that registered after step 1 was
		// satisfied is still a process this subtest has to outlive.
		for _, pid := range held() {
			seen[pid] = true
		}
		time.Sleep(5 * time.Millisecond)
	}
	alive := []int{}
	for pid := range seen {
		for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if syscall.Kill(pid, 0) == nil {
			alive = append(alive, pid)
		}
	}
	if t.Failed() {
		return
	}
	if len(seen) < want {
		t.Errorf("the fixture claimed %d prompt leg(s) in flight; the fake registered %d in %s", want, len(seen), joinWait)
	}
	if left := held(); len(left) > 0 {
		t.Errorf("held prompt(s) %v never let go of %s — the tree is removed under a live writer, which is what recreates it at 0755", left, fake)
	}
	if len(alive) > 0 {
		t.Errorf("fake herdr child(ren) %v outlived the subtest that forked them", alive)
	}
}

func mustCreate(t *testing.T, b *HerdrBackend, o NewSessionOpts) {
	t.Helper()
	if o.Dir == "" {
		o.Dir = t.TempDir()
	}
	if err := b.CreateSession(o); err != nil {
		t.Fatalf("CreateSession(%s): %v", o.Name, err)
	}
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestHerdrCreateSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.EnvsDir, 0o700)
	os.WriteFile(filepath.Join(b.App.EnvsDir, "test.env"), []byte("FOO=bar\n"), 0o600)

	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "proj", Dir: dir, Cmd: "npm run dev", Envs: []string{"test"}})

	log := calls(t, fake)
	for _, want := range []string{
		"workspace create --label proj --no-focus --cwd " + dir,
		"--env FOO=bar",
		// rangerhq-ysly: RHQ_HOME rides every session, not only persona
		// ones — a crew session runs rhq/bd tools too, and without this
		// they would resolve the wrong instance's config, queue, skills.
		"--env RHQ_HOME=" + b.App.Home,
		// ADR 0031 §1: RHQ_LAUNCH_HOME rides every session too (crew
		// included) — it is the origin record init compares against,
		// not an address, so it carries the same value here.
		"--env RHQ_LAUNCH_HOME=" + b.App.Home,
		"pane run w1:p1 npm run dev",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("calls.log missing %q:\n%s", want, log)
		}
	}

	m, ok := b.readMeta("proj")
	if !ok {
		t.Fatal("meta file not written")
	}
	if m.Workspace != "w1" || m.Pane != "w1:p1" || m.Envs != "test" {
		t.Errorf("bad meta: %+v", m)
	}

	if err := b.CreateSession(NewSessionOpts{Name: "proj", Dir: dir}); err == nil {
		t.Error("duplicate create should fail")
	}
	if err := b.CreateSession(NewSessionOpts{Name: "bad name!", Dir: dir}); err == nil {
		t.Error("invalid name should fail")
	}
	// rangerhq-qv5: a name that starts with '-' reads as a flag everywhere
	// it is typed back, so it never becomes a workspace.
	for _, name := range []string{"--help", "-x", "-"} {
		if err := b.CreateSession(NewSessionOpts{Name: name, Dir: dir}); err == nil {
			t.Errorf("session name %q should be refused", name)
		}
		if _, ok := b.readMeta(name); ok {
			t.Errorf("session name %q left a meta file behind", name)
		}
	}
	if log := calls(t, fake); strings.Contains(log, "--label -") {
		t.Errorf("a dashed name reached herdr:\n%s", log)
	}
}

func TestHerdrSessionsForeignAndStale(t *testing.T) {
	const sock = "/tmp/this/herdr.sock"
	t.Setenv("HERDR_SOCKET_PATH", sock)
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})

	// A workspace created outside posse shows up as foreign. Status only
	// surfaces when herdr detects an agent inside (workspace agent_status
	// says "unknown" even for plain shells, so it can't be trusted alone).
	ws := fakeLoadWSFrom(t, fake)
	ws = append(ws, fakeWS{WorkspaceID: "w9", Label: "handmade", Focused: true, AgentStatus: "working"})
	saveWSTo(t, fake, ws)
	agents := `[{"agent":"claude","agent_status":"working","pane_id":"w9:p1","workspace_id":"w9"}]`
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(agents), 0o644)

	// A meta file whose workspace died gets pruned — it names this server,
	// so this server's listing is evidence about it (rangerhq-8fq).
	os.WriteFile(b.metaPath("dead"), []byte("name: dead\nworkspace: w404\npane: w404:p1\nemoji: x\nsocket: "+sock+"\n"), 0o644)

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("want 2 sessions, got %+v", sessions)
	}
	if sessions[0].Name != "handmade" || !sessions[0].Foreign || sessions[0].Status != "working" || !sessions[0].Focused {
		t.Errorf("bad foreign session: %+v", sessions[0])
	}
	if sessions[1].Name != "mine" || sessions[1].Foreign {
		t.Errorf("bad own session: %+v", sessions[1])
	}
	if _, ok := b.readMeta("dead"); ok {
		t.Error("stale meta not pruned")
	}
}

func TestHerdrKillAndFocus(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "victim"})

	if err := b.FocusSession("victim"); err != nil {
		t.Fatalf("focus: %v", err)
	}
	if err := b.KillSession("victim"); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if _, ok := b.readMeta("victim"); ok {
		t.Error("meta survived kill")
	}
	if b.HasSession("victim") {
		t.Error("session survived kill")
	}
	if err := b.KillSession("victim"); err == nil {
		t.Error("killing a dead session should fail")
	}
	if !strings.Contains(calls(t, fake), "workspace close w1") {
		t.Error("workspace close not issued")
	}
}

func TestHerdrAgentTarget(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "crew"})

	if _, err := b.AgentTarget("crew"); err == nil {
		t.Error("expected error with no agents detected")
	}
	agents := `[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(agents), 0o644)
	target, err := b.AgentTarget("crew")
	if err != nil {
		t.Fatal(err)
	}
	if target != "w1:p1" {
		t.Errorf("want w1:p1, got %s", target)
	}
}

func TestHerdrPaneReadTail(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	// Full read trims the padded blank rows the terminal buffer carries.
	text, err := b.H.PaneRead("w1:p1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if text != "prompt$ echo hi\nhi\nprompt$" {
		t.Errorf("bad full read: %q", text)
	}
	// Tail is applied client-side after trimming, so it never returns blanks.
	text, err = b.H.PaneRead("w1:p1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if text != "hi\nprompt$" {
		t.Errorf("bad tail read: %q", text)
	}
}

func TestHerdrRunErrorEnvelope(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	err := b.H.FocusWorkspace("w404")
	if err == nil || !strings.Contains(err.Error(), "workspace_not_found") {
		t.Errorf("want workspace_not_found error, got %v", err)
	}
}

func TestPersonaLaunch(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	persona := "---\nname: ranger\ndescription: test\ncommand: claude --append-system-prompt \"$(cat {file})\" --add-dir {memory}\n---\nYou are ranger.\n"
	os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(persona), 0o644)

	mustCreate(t, b, NewSessionOpts{Name: "crew", Agent: "ranger"})

	memdir := filepath.Join(b.App.PersonasDir(), "ranger")
	if _, err := os.Stat(filepath.Join(memdir, "ORDERS.md")); err != nil {
		t.Error("persona memory dir not materialized at launch")
	}
	log := calls(t, fake)
	for _, want := range []string{
		"--env BD_ACTOR=ranger",
		"--env RHQ_PERSONA=ranger",
		"--env RHQ_PERSONA_DIR=" + memdir,
		"--add-dir '" + memdir + "'",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("calls.log missing %q", want)
		}
	}
}

func TestPersonaToolEnv(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	persona := "---\nname: ranger\ndescription: test\ncommand: claude {allow} {deny}\n" +
		"allow:\n  - Bash(bd:*)\n  - Edit\ndeny:\n  - Bash(git push:*)\n---\nYou are ranger.\n"
	os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(persona), 0o644)

	mustCreate(t, b, NewSessionOpts{Name: "crew", Agent: "ranger"})

	log := calls(t, fake)
	for _, want := range []string{
		"--env RHQ_TOOLS_ALLOW=Bash(bd:*)\nEdit",
		// The env carries the PID's own rules; the typed line goes through
		// L0Spellings (which for this rule now emits it unchanged — the
		// option-blind pair is gone, rangerhq-ky3/rangerhq-vr6j). The L3
		// pre-push hook reads RHQ_TOOLS_DENY and must keep seeing the rule as
		// the PID wrote it either way.
		"--env RHQ_TOOLS_DENY=Bash(git push:*)",
		"GATES claude --allowedTools 'Bash(bd:*)' 'Edit' " +
			"--disallowedTools 'Bash(git push:*)'",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("calls.log missing %q:\n%s", want, log)
		}
	}
}

func TestBdClaimClose(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}

	if resumed, err := bd.Claim("", "x-1", "ranger"); err != nil || resumed {
		t.Fatalf("fresh claim: resumed=%v err=%v", resumed, err)
	}
	if err := bd.Close("", "x-1", ""); err != nil {
		t.Fatal(err)
	}
	log, _ := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	for _, want := range []string{
		"--actor ranger update x-1 --claim --json",
		"close x-1 --json",
	} {
		if !strings.Contains(string(log), want) {
			t.Errorf("bd-calls.log missing %q:\n%s", want, log)
		}
	}
}

func TestReadyAll(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}

	repo1, repo2 := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(repo1, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"one","priority":1}]`), 0o644)
	os.WriteFile(filepath.Join(repo2, "fake-ready.json"),
		[]byte(`[{"id":"b-1","title":"two","priority":2}]`), 0o644)
	os.WriteFile(b.App.ConfigPath,
		[]byte("beads:\n  - "+repo1+"\n  - "+repo2+"\n"), 0o644)

	issues, failed := bd.ReadyAll(b.App, "")
	if len(issues) != 2 || issues[0].ID != "a-1" || issues[1].ID != "b-1" {
		t.Errorf("bad aggregation: %+v", issues)
	}
	if issues[0].Dir != repo1 || issues[1].Dir != repo2 {
		t.Errorf("issues not tagged with repo dirs: %+v", issues)
	}
	if len(failed) != 0 {
		t.Errorf("two good repos report no failures, got %v", failed)
	}
}

func newTestDispatcher(t *testing.T, b *HerdrBackend) *Dispatcher {
	t.Helper()
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	d.StartupWait = 2 * time.Second
	d.StatusGrace = 50 * time.Millisecond
	d.Poll = 10 * time.Millisecond
	d.TurnOutcome = func(string, string, time.Time) (string, bool) { return "", false }
	// Hermetic by construction, like RHQ_FAKE_HERDR: Watch's settle-event
	// subscription is the one herdr read that DIALS a socket, and the socket
	// it resolves without this is the operator's live server (ADR 0016 §3 —
	// no test reaches a real herdr). A nil channel never fires; a test that
	// wants the real adapter clears this field and points HERDR_SOCKET_PATH
	// at its own listener.
	d.Hints = func(context.Context, func(string)) <-chan HerdrHint { return nil }
	return d
}

// The BASE writer, not d.Out: Watch tees the loop's own log over Out for the
// life of the loop (watchlog.go), so after a Watch call d.Out is a
// MultiWriter and this helper's whole job is to hand back the builder the
// test passed in.
func dispatcherOut(d *Dispatcher) string { return d.baseOut().(*strings.Builder).String() }

func writePersona(t *testing.T, a *App, name, labels string) {
	t.Helper()
	os.MkdirAll(a.AgentsDir, 0o755)
	md := "---\nname: " + name + "\ndescription: test\nlabels: " + labels + "\n---\nYou are " + name + ".\n"
	os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644)
}

func TestDispatchRun(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","priority":1,"labels":["go"]}]`), 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","status":"closed"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	// The workspace create will yield w1; herdr "detects" claude in its pane.
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 dispatched, got %d\noutput:\n%s", n, dispatcherOut(d))
	}

	session := SessionFor("ranger", repo)
	log := calls(t, fake)
	if !strings.Contains(log, "workspace create --label "+session) {
		t.Errorf("session not created for persona:\n%s", log)
	}
	if !strings.Contains(log, "agent prompt w1:p1") || !strings.Contains(log, "a-1") {
		t.Errorf("agent not prompted with bead id:\n%s", log)
	}
	bdlog, _ := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if !strings.Contains(string(bdlog), "--actor ranger update a-1 --claim --json") {
		t.Errorf("bead not claimed as persona:\n%s", bdlog)
	}
	if !strings.Contains(dispatcherOut(d), "closed by ranger") {
		t.Errorf("closed bead not reported:\n%s", dispatcherOut(d))
	}
}

func TestDispatchResume(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`), 0o644)
	// Claim fails (already held) but the holder is this persona → resume.
	os.WriteFile(filepath.Join(repo, "fake-claim-fail"), nil, 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 || !strings.Contains(out, "resuming") {
		t.Errorf("want resume dispatch, got n=%d:\n%s", n, out)
	}

	// A different holder is a real conflict — no prompt. (ready and show
	// agree on the holder, as real bd does; the held-bead skip of
	// rangerhq-zom only applies to this persona's own in_progress beads.)
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"],"assignee":"someone-else","status":"in_progress"}]`), 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"t","status":"in_progress","assignee":"someone-else"}]`), 0o644)
	d2 := newTestDispatcher(t, b)
	d2.Bd = d.Bd
	n, err = d2.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || !strings.Contains(dispatcherOut(d2), "claim lost") {
		t.Errorf("want claim-lost skip, got n=%d:\n%s", n, dispatcherOut(d2))
	}
}

func TestDispatchAwaitsIdleBeforePrompt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"t","status":"closed"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	// herdr detects claude, but it is still starting — detection alone must
	// not trigger the prompt (the agent_prompt_stalled race).
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"starting","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 dispatched, got %d\noutput:\n%s", n, dispatcherOut(d))
	}
	log := calls(t, fake)
	wait := strings.Index(log, "agent wait w1:p1")
	prompt := strings.Index(log, "agent prompt w1:p1")
	if wait == -1 || prompt == -1 || wait > prompt {
		t.Errorf("prompt must follow a settle wait (wait=%d prompt=%d):\n%s", wait, prompt, log)
	}
	if !strings.Contains(log, "--until idle") {
		t.Errorf("settle wait should target idle:\n%s", log)
	}
}

func TestDispatchWaitNeverSettles(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"starting","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
	// The wait times out with the agent still unsettled — no prompt may fire.
	os.WriteFile(filepath.Join(fake, "wait-status"), []byte("starting"), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || !strings.Contains(dispatcherOut(d), "never settled") {
		t.Errorf("want unsettled failure, got n=%d:\n%s", n, dispatcherOut(d))
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("prompt fired despite unsettled agent:\n%s", calls(t, fake))
	}
}

func TestLaunchBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	// The workspace create will yield w1; herdr "detects" claude in its pane.
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)

	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "fix the thing", Labels: []string{"go"}}, Dir: repo}
	session, err := d.LaunchBead(is)
	if err != nil {
		t.Fatal(err)
	}
	if want := SessionForBead("ranger", repo, "a-1"); session != want {
		t.Errorf("want session %s, got %s", want, session)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "workspace create --label "+session) {
		t.Errorf("session not created:\n%s", log)
	}
	if !strings.Contains(log, "agent prompt w1:p1") || !strings.Contains(log, "a-1") {
		t.Errorf("agent not prompted with bead id:\n%s", log)
	}
	if strings.Contains(log, "--wait") {
		t.Errorf("cockpit launch must not block on --wait:\n%s", log)
	}
	bdlog, _ := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if !strings.Contains(string(bdlog), "--actor ranger update a-1 --claim --json") {
		t.Errorf("bead not claimed as persona:\n%s", bdlog)
	}

	// A busy persona session must not be double-prompted. ADR 0020 §2
	// (amended): with no holder, `d` seats availability-first over the
	// whole lane, so a lane fully busy refuses by naming the LANE, not the
	// one persona (ranger-base-f8m9) — the same line the pass gives.
	ws := fakeLoadWSFrom(t, fake)
	ws[0].AgentStatus = "working"
	saveWSTo(t, fake, ws)
	if _, err := d.LaunchBead(is); err == nil || !strings.Contains(err.Error(), "lane busy") || !strings.Contains(err.Error(), "ranger") {
		t.Errorf("busy lane should refuse dispatch naming the lane, got %v", err)
	}

	// Unroutable beads error instead of silently vanishing.
	orphan := RepoIssue{BdIssue: BdIssue{ID: "a-2", Labels: []string{"mystery"}}, Dir: repo}
	if _, err := d.LaunchBead(orphan); err == nil || !strings.Contains(err.Error(), "unroutable") {
		t.Errorf("unroutable bead should error, got %v", err)
	}
}

func TestDispatchRouting(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	writePersona(t, b.App, "scribe", "[docs]")

	route := func(assignee string, labels []string) string {
		p, _ := d.Route(RepoIssue{BdIssue: BdIssue{ID: "x", Assignee: assignee, Labels: labels}})
		return p
	}
	if got := route("scribe", []string{"go"}); got != "scribe" {
		t.Errorf("assignee should win, got %q", got)
	}
	if got := route("", []string{"docs"}); got != "scribe" {
		t.Errorf("label routing failed, got %q", got)
	}
	if got := route("", []string{"mystery"}); got != "" {
		t.Errorf("unroutable bead routed to %q", got)
	}
	os.WriteFile(b.App.ConfigPath, []byte("default_persona: ranger\n"), 0o644)
	if got := route("", []string{"mystery"}); got != "ranger" {
		t.Errorf("default_persona fallback failed, got %q", got)
	}
}

func TestDispatchDryRun(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["mystery"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("want 1 routable, got %d", n)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "a-1") || !strings.Contains(out, "unroutable") {
		t.Errorf("dry-run output incomplete:\n%s", out)
	}
	if strings.Contains(calls(t, fake), "workspace create") {
		t.Error("dry-run must not create sessions")
	}
}

func TestParseBdIssues(t *testing.T) {
	t.Parallel()
	issues, err := parseBdIssues([]byte(`[{"id":"x-1","title":"t","status":"open","priority":1,"issue_type":"task","assignee":"ranger"}]`))
	if err != nil || len(issues) != 1 || issues[0].ID != "x-1" || issues[0].Assignee != "ranger" {
		t.Errorf("bad parse: %+v err=%v", issues, err)
	}
	for _, empty := range []string{"", "null", "[]"} {
		issues, err = parseBdIssues([]byte(empty))
		if err != nil || len(issues) != 0 {
			t.Errorf("empty case %q: %+v err=%v", empty, issues, err)
		}
	}
}

// helpers reading/writing the fake's state from test scope
func fakeLoadWSFrom(t *testing.T, dir string) []fakeWS {
	t.Helper()
	var ws []fakeWS
	if b, err := os.ReadFile(filepath.Join(dir, "ws.json")); err == nil {
		json.Unmarshal(b, &ws)
	}
	return ws
}

func saveWSTo(t *testing.T, dir string, ws []fakeWS) {
	t.Helper()
	b, _ := json.Marshal(ws)
	os.WriteFile(filepath.Join(dir, "ws.json"), b, 0o644)
}

// rangerhq-f2b: an env set is readable by the agent in its session, so a
// persona never receives config default_env implicitly — only its own
// `envs:` plus explicit sets. Plain sessions keep the default.
func TestPersonaSessionsSkipDefaultEnv(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.EnvsDir, 0o700)
	os.WriteFile(filepath.Join(b.App.EnvsDir, "default.env"), []byte("DEFAULT_TOKEN=x\n"), 0o600)
	os.WriteFile(filepath.Join(b.App.EnvsDir, "gh.env"), []byte("GH_TOKEN=y\n"), 0o600)
	os.WriteFile(filepath.Join(b.App.EnvsDir, "extra.env"), []byte("EXTRA=z\n"), 0o600)
	os.WriteFile(b.App.ConfigPath, []byte("default_env: default\n"), 0o644)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "plain.md"), []byte("---\nname: plain\n---\nYou are plain.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "gh.md"), []byte("---\nname: gh\nenvs: [gh]\n---\nYou are gh.\n"), 0o644)

	mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "plain"})
	mustCreate(t, b, NewSessionOpts{Name: "s2", Agent: "gh", Envs: []string{"extra"}})
	mustCreate(t, b, NewSessionOpts{Name: "s3", Cmd: "npm run dev"})

	log := calls(t, fake)
	creates := []string{}
	for _, ln := range strings.Split(log, "\n") {
		if strings.HasPrefix(ln, "workspace create") {
			creates = append(creates, ln)
		}
	}
	if len(creates) != 3 {
		t.Fatalf("want 3 creates:\n%s", log)
	}
	if strings.Contains(creates[0], "DEFAULT_TOKEN") {
		t.Errorf("persona session received default_env:\n%s", creates[0])
	}
	if !strings.Contains(creates[1], "--env GH_TOKEN=y") || !strings.Contains(creates[1], "--env EXTRA=z") || strings.Contains(creates[1], "DEFAULT_TOKEN") {
		t.Errorf("persona envs: + explicit set expected, no default:\n%s", creates[1])
	}
	if !strings.Contains(creates[2], "--env DEFAULT_TOKEN=x") {
		t.Errorf("plain session should still get default_env:\n%s", creates[2])
	}
	m, _ := b.readMeta("s2")
	if m.Envs != "extra+gh" {
		t.Errorf("meta should name both sets, got %q", m.Envs)
	}
	m, _ = b.readMeta("s1")
	if m.Envs != "" {
		t.Errorf("persona without envs: must record none, got %q", m.Envs)
	}
}

// rangerhq-f2b: envs/ drifted to 755/644 in the wild; every read/launch
// re-asserts 700/600 and names what it fixed (never contents).
func TestTightenEnvPerms(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.EnvsDir, 0o755)
	f := filepath.Join(b.App.EnvsDir, "leaky.env")
	os.WriteFile(f, []byte("SECRET=v\n"), 0o644)
	os.Chmod(f, 0o644)
	os.Chmod(b.App.EnvsDir, 0o755)

	var notes strings.Builder
	b.App.TightenEnvPerms(&notes)
	if st, _ := os.Stat(b.App.EnvsDir); st.Mode().Perm() != 0o700 {
		t.Errorf("envs dir mode %04o, want 0700", st.Mode().Perm())
	}
	if st, _ := os.Stat(f); st.Mode().Perm() != 0o600 {
		t.Errorf("env file mode %04o, want 0600", st.Mode().Perm())
	}
	out := notes.String()
	if !strings.Contains(out, "leaky.env was 0644") || strings.Contains(out, "SECRET") {
		t.Errorf("notice must name the file and never its contents:\n%s", out)
	}
	notes.Reset()
	b.App.TightenEnvPerms(&notes)
	if notes.Len() != 0 {
		t.Errorf("second pass must be silent, got:\n%s", notes.String())
	}
	// EnvSetVars tightens on read too.
	os.Chmod(f, 0o644)
	if _, err := b.App.EnvSetVars("leaky"); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(f); st.Mode().Perm() != 0o600 {
		t.Errorf("EnvSetVars left mode %04o", st.Mode().Perm())
	}
}

// ADR 0002 §1–2: a session's runtime rides the env (RHQ_RUNTIME), the
// meta (runtime:), and the emoji; --runtime overrides the PID; relaunch
// re-renders for the same runtime.
func TestPersonaLaunchRuntime(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "security.md"),
		[]byte("---\nname: security\ndeny: [Edit, Write, Bash(git push:*)]\n---\nYou are security.\n"), 0o644)
	// A PID the wall fully realizes on claude at shims — the only kind that
	// may run at fast (ADR 0003 §3).
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("emoji:\n  claude: ✳️\n  codex: 🪢\n"), 0o644)

	// security denies Edit/Write: on claude at cage shims those are politeness
	// only → the launch refuses unless degradation is allowed (ADR 0002 §4);
	// on codex -s read-only is OS-enforced → launches clean.
	var warn strings.Builder
	b.Warn = &warn
	if err := b.CreateSession(NewSessionOpts{Name: "h1", Agent: "security"}); err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), "Edit — needs cage: seatbelt") {
		t.Fatalf("security on claude must refuse at shims: %v", err)
	}
	mustCreate(t, b, NewSessionOpts{Name: "h1", Agent: "security", AllowDegraded: true})
	if !strings.Contains(warn.String(), "h1 launches DEGRADED on claude @ shims") {
		t.Errorf("degraded launch must be announced: %q", warn.String())
	}
	mustCreate(t, b, NewSessionOpts{Name: "h2", Agent: "security", Runtime: "codex"})
	log := calls(t, fake)
	if !strings.Contains(log, "--env RHQ_RUNTIME=claude") || !strings.Contains(log, "--env RHQ_RUNTIME=codex") {
		t.Errorf("RHQ_RUNTIME missing:\n%s", log)
	}
	if !strings.Contains(log, "--env RHQ_HOME="+b.App.Home) {
		t.Errorf("RHQ_HOME missing (rangerhq-ysly):\n%s", log)
	}
	// ADR 0031 §1: RHQ_LAUNCH_HOME is a persona session's origin record —
	// same value as RHQ_HOME at launch, but init compares against this one
	// because a persona overriding RHQ_HOME for a scratch run leaves it
	// standing.
	if !strings.Contains(log, "--env RHQ_LAUNCH_HOME="+b.App.Home) {
		t.Errorf("RHQ_LAUNCH_HOME missing (ADR 0031 §1):\n%s", log)
	}
	if !strings.Contains(log, "GATES claude --model 'claude-fable-5-1' "+ClaudeFleetFlags+" --append-system-prompt") || !strings.Contains(log, "GATES codex -c model='gpt-5.6-sol' -s read-only -a never --disable hooks -c allow_login_shell=false -c \"projects=") {
		t.Errorf("rendered commands:\n%s", log)
	}
	m1, _ := b.readMeta("h1")
	m2, _ := b.readMeta("h2")
	if m1.Runtime != "claude" || m2.Runtime != "codex" {
		t.Errorf("meta runtime: %q %q", m1.Runtime, m2.Runtime)
	}
	if m1.Tier != "strong" || m2.Tier != "strong" || !strings.Contains(log, "--env RHQ_TIER=strong") {
		t.Errorf("tier must ride meta and env: %q %q", m1.Tier, m2.Tier)
	}
	if m1.Cage != "shims" || !strings.Contains(m1.Degraded, "Edit") || m2.Degraded != "" || !strings.Contains(log, "--env RHQ_CAGE=shims") {
		t.Errorf("cage/degraded in meta and env: %+v %+v", m1, m2)
	}
	if ss, _ := b.Sessions(); len(ss) > 0 {
		for _, x := range ss {
			if x.Name == "h1" && x.Degraded == "" || x.Name == "h2" && x.Degraded != "" {
				t.Errorf("session listing degraded flag: %+v", x)
			}
		}
	}
	// --tier overrides the PID's default and renders the model; the
	// listing tag shows runtime/tier when either is not the default.
	mustCreate(t, b, NewSessionOpts{Name: "h4", Agent: "dev", Tier: "fast"})
	if got := calls(t, fake); !strings.Contains(got, "GATES claude --model 'claude-sonnet-5'") || !strings.Contains(got, "--env RHQ_TIER=fast") {
		t.Errorf("--tier fast:\n%s", got)
	}
	m4, _ := b.readMeta("h4")
	if m4.Tier != "fast" || b.App.RuntimeTierTag(m4.Runtime, m4.Tier) != "@claude/fast" || b.App.RuntimeTierTag("claude", "strong") != "" || b.App.RuntimeTierTag("codex", "") != "@codex/strong" {
		t.Errorf("tier tag: %q %q", m4.Tier, b.App.RuntimeTierTag(m4.Runtime, m4.Tier))
	}
	if err := b.CreateSession(NewSessionOpts{Name: "h5", Agent: "security", Tier: "huge", AllowDegraded: true}); err == nil {
		t.Error("unknown tier must fail the launch")
	}
	// ADR 0003 §3 at the launch site: a security PID's Edit/Write are
	// politeness on
	// claude at shims, so fast refuses there and the flag does not buy it.
	if err := b.CreateSession(NewSessionOpts{Name: "h6", Agent: "security", Tier: "fast", AllowDegraded: true}); err == nil ||
		!strings.Contains(err.Error(), "--allow-degraded is never accepted") {
		t.Errorf("fast must refuse a wall that leaves a gate to politeness: %v", err)
	}
	if m1.Emoji != "✳️" || m2.Emoji != "🪢" {
		t.Errorf("runtime emoji: %q %q", m1.Emoji, m2.Emoji)
	}
	if err := b.CreateSession(NewSessionOpts{Name: "h3", Agent: "security", Runtime: "nope"}); err == nil {
		t.Error("unknown --runtime must fail the launch")
	}

	// Relaunch after a crash re-types the codex command, not claude's.
	m2.Launched = time.Now().Add(-time.Hour)
	b.writeMeta(m2)
	os.Remove(filepath.Join(fake, "agents.json"))
	ok, err := b.RelaunchAgent("h2", time.Second)
	if err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	if got := calls(t, fake); strings.Count(got, "GATES codex -c model='gpt-5.6-sol' -s read-only") != 2 {
		t.Errorf("relaunch must reuse the session's runtime:\n%s", got)
	}
	// Recipe runtime: rides through LaunchRecipe.
	os.MkdirAll(b.App.RecipesDir, 0o755)
	os.WriteFile(filepath.Join(b.App.RecipesDir, "hg.yaml"), []byte("name: hg\nagent: security\nruntime: grok\n"), 0o644)
	var out strings.Builder
	if err := b.LaunchRecipe(&out, "hg"); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("security on grok at shims must refuse (grok --deny is L0): %v", err)
	}
	// A recipe for a persona whose gates the wall does realize launches.
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"), []byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.RecipesDir, "hg.yaml"), []byte("name: hg\nagent: dev\nruntime: grok\n"), 0o644)
	if err := b.LaunchRecipe(&out, "hg"); err != nil {
		t.Fatal(err)
	}
	if got := calls(t, fake); !strings.Contains(got, `GATES grok -m 'grok-4.6' `+GrokFleetFlags+` --rules="$(cat '`) {
		t.Errorf("recipe runtime: not applied:\n%s", got)
	}
}

// rangerhq-snd (incident): a dispatch pass ran with the fleet's RHQ_HOME
// while HERDR_SOCKET_PATH pointed at a scratch herdr. Every workspace was
// "missing" from that server's listing, and one read deleted the metas of
// eleven live sessions — personas, env sets, crew marks, ids, all of it,
// with no copy anywhere (state/ is outside git by design). Two guards:
// an empty listing never prunes, and a meta written against another socket
// never prunes. Both keep the file and leave the session out of the listing.
func TestHerdrSessionsNeverPruneAgainstTheWrongServer(t *testing.T) {
	stale := func(b *HerdrBackend, name, sock string) {
		t.Helper()
		meta := "name: " + name + "\nworkspace: w404\npane: w404:p1\nemoji: x\n"
		if sock != "" {
			meta += "socket: " + sock + "\n"
		}
		os.MkdirAll(b.metaDir(), 0o755)
		if err := os.WriteFile(b.metaPath(name), []byte(meta), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// An empty workspace listing is a herdr that holds nothing — never
	// evidence that every session died.
	t.Run("empty listing", func(t *testing.T) {
		b, fake := newTestBackend(t)
		var warn strings.Builder
		b.Warn = &warn
		saveWSTo(t, fake, nil)
		stale(b, "dead", "")

		sessions, err := b.Sessions()
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) != 0 {
			t.Errorf("a meta whose workspace is absent must not be listed: %+v", sessions)
		}
		if _, ok := b.readMeta("dead"); !ok {
			t.Fatal("meta pruned against an empty workspace listing")
		}
		if !strings.Contains(warn.String(), "kept, not listed") {
			t.Errorf("refusal not reported: %q", warn.String())
		}
	})

	// A live workspace elsewhere in the listing proves the server answers;
	// a meta written against a *different* socket still says nothing about
	// that server's herd.
	t.Run("different socket", func(t *testing.T) {
		const sock = "/tmp/this/herdr.sock"
		t.Setenv("HERDR_SOCKET_PATH", sock)
		b, _ := newTestBackend(t)
		var warn strings.Builder
		b.Warn = &warn
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		stale(b, "elsewhere", "/tmp/some-other/herdr.sock")
		stale(b, "ours", sock)
		stale(b, "unrecorded", "")

		sessions, err := b.Sessions()
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) != 1 || sessions[0].Name != "mine" {
			t.Errorf("want only the live session listed, got %+v", sessions)
		}
		if _, ok := b.readMeta("elsewhere"); !ok {
			t.Error("meta from another herdr server was pruned")
		}
		// Same server, workspace genuinely gone: this is the case pruning
		// exists for, and it still works.
		if _, ok := b.readMeta("ours"); ok {
			t.Error("a dead workspace on this same server should still be pruned")
		}
		// A meta that records no socket names no server, so no listing is
		// evidence about it — the arm rangerhq-8fq was filed for.
		if _, ok := b.readMeta("unrecorded"); !ok {
			t.Error("a meta with no recorded socket was pruned")
		}
	})

	// rangerhq-8fq: refusing is only half of it. A meta written before
	// `socket:` existed would be refused forever — never pruned, and a
	// refusal on every listing — so the server holding its workspace stamps
	// it on the way past, and the guard has something to compare next time.
	t.Run("backfill", func(t *testing.T) {
		const sock = "/tmp/this/herdr.sock"
		t.Setenv("HERDR_SOCKET_PATH", sock)
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "old"})

		// Rewrite it the way a pre-9ac4a16 binary left it: no socket line.
		m, ok := b.readMeta("old")
		if !ok {
			t.Fatal("no meta")
		}
		m.Socket = ""
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}

		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if m, ok := b.readMeta("old"); !ok || m.Socket != sock {
			t.Fatalf("socket not backfilled for a live workspace: %+v", m)
		}

		// And now that it names a server, its death is prunable.
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w9", Label: "other"}})
		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if _, ok := b.readMeta("old"); ok {
			t.Error("backfilled meta not pruned once its workspace died on this server")
		}
	})

	// The default server is a server, and rangerhq-y4z is what made it one:
	// $HERDR_SOCKET_PATH unset resolves to the path herdr itself would use,
	// so a pass outside a pane stamps and backfills like any other. Before
	// that it stamped nothing, and a session created outside a pane left a
	// meta no listing could ever prune.
	t.Run("the default socket is stamped and backfilled like any other", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, _ := newTestBackend(t)
		dflt := SocketID()
		if dflt == "" {
			t.Fatal("setup: the default socket must resolve to a path (rangerhq-y4z)")
		}
		mustCreate(t, b, NewSessionOpts{Name: "plain"})
		if m, ok := b.readMeta("plain"); !ok || m.Socket != dflt {
			t.Fatalf("a create outside a pane must stamp the resolved default socket %s: %+v", dflt, m)
		}

		// And the backfill reaches a meta that predates the field.
		m, _ := b.readMeta("plain")
		m.Socket = ""
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if m, ok := b.readMeta("plain"); !ok || m.Socket != dflt {
			t.Errorf("a live workspace's pre-field meta was not backfilled on the default socket: %+v", m)
		}
	})

	// A pass that cannot name its socket at all still stamps nothing: it has
	// no server to claim the meta for, and a forged stamp is worse than an
	// absent one. This is the case "" means since rangerhq-y4z.
	t.Run("no backfill when the socket cannot be named", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, _ := newTestBackend(t)
		dir := t.TempDir()
		mustCreate(t, b, NewSessionOpts{Name: "plain", Dir: dir})
		m, _ := b.readMeta("plain")
		m.Socket = ""
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}

		t.Setenv("HOME", "") // os.UserHomeDir errors: nothing to resolve against
		if got := SocketID(); got != "" {
			// Asserted, not skipped: a skip here would go quiet under the
			// very mutation this arm exists to catch (ranger-base-f0y3).
			t.Fatalf("setup: with no $HOME there is nothing to resolve against, got %q", got)
		}
		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if m, ok := b.readMeta("plain"); !ok || m.Socket != "" {
			t.Errorf("a pass that cannot name its own socket must not stamp one: %+v", m)
		}
	})

	// The socket a session was created against is recorded, so the guard has
	// something to compare on the next read.
	t.Run("socket recorded", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "/tmp/scratch/herdr.sock")
		b, _ := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "scratchling"})
		m, ok := b.readMeta("scratchling")
		if !ok || m.Socket != "/tmp/scratch/herdr.sock" {
			t.Errorf("socket not recorded in meta: %+v", m)
		}
	})
}

// ─── prune must prove death (rangerhq-9nso, ADR 0011 §2) ─────────────────────

// rangerhq-9nso (incident): three dispatch passes ran concurrently, each
// holding a `workspace list` taken before the others' workspaces existed.
// Every guard from rangerhq-8fq held — the listings were not empty and the
// metas named this very socket — and the passes still deleted each other's
// fresh metas: live sessions, agents running, identity gone, and with it the
// pane the prompting pass needed ("agent_prompt_stalled ... unclaimed").
// Absence from a snapshot is not death. Past the socket guards a prune now
// needs both: the meta is older than PruneGrace, and herdr, asked about that
// workspace by id right now, says it is not there.
func TestHerdrSessionsPruneMustProveDeath(t *testing.T) {
	const sock = "/tmp/this/herdr.sock"

	// age rewinds a meta's launched: past the grace, leaving the direct
	// query as the only thing standing between it and deletion.
	age := func(t *testing.T, b *HerdrBackend, name string) {
		t.Helper()
		m, ok := b.readMeta(name)
		if !ok {
			t.Fatalf("no meta for %s", name)
		}
		m.Launched = time.Now().Add(-2 * PruneGrace)
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
	}
	hideFromList := func(t *testing.T, fake, id string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fake, "hidden-from-list"), []byte(id), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// setup makes the racing pass's own session (so the listing is never
	// empty) plus the session another pass just created, and returns the
	// latter's workspace id.
	setup := func(t *testing.T) (*HerdrBackend, string, string) {
		t.Helper()
		t.Setenv("HERDR_SOCKET_PATH", sock)
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "newborn", Cmd: "claude"}) // stamps launched:
		m, ok := b.readMeta("newborn")
		if !ok {
			t.Fatal("no meta for newborn")
		}
		return b, fake, m.Workspace
	}

	// The incident itself: pass B's listing predates pass A's workspace.
	t.Run("young meta missing from the listing", func(t *testing.T) {
		b, fake, ws := setup(t)
		hideFromList(t, fake, ws)

		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if _, ok := b.readMeta("newborn"); !ok {
			t.Fatal("a live session's meta was pruned by a racing pass's stale listing (rangerhq-9nso)")
		}
	})

	// Same shape, but old enough that the grace does not cover it — the
	// per-id query is what has to save it. It is also the general case: a
	// listing can lag for reasons other than youth.
	t.Run("old meta whose workspace is alive", func(t *testing.T) {
		b, fake, ws := setup(t)
		age(t, b, "newborn")
		hideFromList(t, fake, ws)
		os.Remove(filepath.Join(fake, "calls.log"))

		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if _, ok := b.readMeta("newborn"); !ok {
			t.Fatal("meta deleted for a workspace herdr still holds — a listing snapshot is not evidence")
		}
		log, _ := os.ReadFile(filepath.Join(fake, "calls.log"))
		if !strings.Contains(string(log), "workspace get "+ws) {
			t.Errorf("no direct per-id query before the prune decision:\n%s", log)
		}
	})

	// The prune still has to work: this is the case it exists for.
	t.Run("workspace genuinely gone", func(t *testing.T) {
		b, fake, ws := setup(t)
		age(t, b, "newborn")
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}}) // newborn's workspace closed
		os.Remove(filepath.Join(fake, "calls.log"))

		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if _, ok := b.readMeta("newborn"); ok {
			t.Error("a workspace this server confirms is gone was not pruned")
		}
		log, _ := os.ReadFile(filepath.Join(fake, "calls.log"))
		if !strings.Contains(string(log), "workspace get "+ws) {
			t.Errorf("pruned without asking herdr about the workspace:\n%s", log)
		}
	})

	// A server that does not answer has said nothing about the workspace.
	t.Run("herdr does not answer the query", func(t *testing.T) {
		b, fake, _ := setup(t)
		age(t, b, "newborn")
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})
		if err := os.WriteFile(filepath.Join(fake, "workspace-get-unreachable"), nil, 0o644); err != nil {
			t.Fatal(err)
		}

		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if _, ok := b.readMeta("newborn"); !ok {
			t.Error("meta pruned on a query that errored — silence is not evidence of death")
		}
	})

	// Nothing to ask about is not the same as an answer.
	t.Run("meta with no workspace id", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", sock)
		b, _ := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		os.WriteFile(b.metaPath("nameless"),
			[]byte("name: nameless\nworkspace: \npane: \nemoji: x\nsocket: "+sock+"\n"), 0o644)

		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if _, ok := b.readMeta("nameless"); !ok {
			t.Error("a meta naming no workspace was pruned; absence of a name is not death")
		}
	})

	// The refusal is reported: the operator sees the race happen rather
	// than a session quietly missing from one listing.
	t.Run("refusal is reported", func(t *testing.T) {
		b, fake, ws := setup(t)
		var warn strings.Builder
		b.Warn = &warn
		hideFromList(t, fake, ws)

		sessions, err := b.Sessions()
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range sessions {
			if s.Name == "newborn" {
				t.Errorf("a workspace absent from the listing must not be listed: %+v", s)
			}
		}
		if !strings.Contains(warn.String(), "not dead") {
			t.Errorf("spared metas not reported: %q", warn.String())
		}
	})
}

// rangerhq-jeu2: the guard has to ask the server that would know. The prune
// reaches WorkspaceAlive only behind the socket guards; the create had none,
// so on a multi-server board it asked whatever herdr posse was pointed at, got
// a truthful "never held that id", and overwrote the record of a session
// alive elsewhere — the file the prune had just spared. cannotAnswerFor is
// now the one predicate both halves ask through.
//
// These bound the arm mustNotOrphan deliberately does not take (an unstamped
// meta): it is not taken only when this pass is unstamped too, which is one
// server talking about itself. Against a named socket it still refuses.
func TestCreateMustAskTheServerThatWouldKnow(t *testing.T) {
	// An unstamped meta plus a named socket is two servers, and refuses:
	// this pass is on a concrete herdr, the meta was not written there.
	t.Run("unstamped meta against a named socket refuses", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "/tmp/jeu2/named.sock")
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "legacy", Cmd: "claude"})
		m, _ := b.readMeta("legacy")
		ws := m.Workspace
		m.Socket = "" // a meta from before socket: existed
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}}) // absent from this server

		err := b.CreateSession(NewSessionOpts{Name: "legacy", Cmd: "claude", Dir: t.TempDir()})
		if err == nil {
			t.Fatal("overwrote a meta this server cannot answer for (rangerhq-jeu2)")
		}
		if now, ok := b.readMeta("legacy"); !ok || now.Workspace != ws {
			t.Fatalf("the meta was overwritten behind the refusal: %+v", now)
		}
	})

	// And the board that must NOT refuse, which is the operator's ordinary
	// path: `posse` from a plain terminal. It has $HERDR_SOCKET_PATH unset,
	// and since rangerhq-y4z that resolves to herdr's default socket rather
	// than to "" — so it writes and reads metas naming that server, which is
	// one server talking about itself. Its not_found IS evidence, and a dead
	// session's name stays reusable.
	//
	// Before y4z this row said the same thing for a different reason: the
	// metas were unstamped on both sides and mustNotOrphan skipped the
	// unstamped arm to keep exactly this working. The arm is taken now; what
	// keeps the name reusable is that there is nothing unstamped left to
	// take it on.
	t.Run("the default socket keeps a dead name reusable", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "alpha", Cmd: "claude"})
		if m, _ := b.readMeta("alpha"); m.Socket != SocketID() || m.Socket == "" {
			t.Fatalf("setup: a create outside a pane must stamp the resolved default socket %q, got %q", SocketID(), m.Socket)
		}
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}}) // alpha's workspace closed

		if err := b.CreateSession(NewSessionOpts{Name: "alpha", Cmd: "claude", Dir: t.TempDir()}); err != nil {
			t.Fatalf("a dead session's name must stay reusable on the default socket: %v", err)
		}
		if log := calls(t, fake); !strings.Contains(log, "workspace get ") {
			t.Errorf("the name was reused without asking herdr about its workspace:\n%s", log)
		}
	})

	// The arm that IS taken now, and could not be before: a meta recording
	// no socket at all. It is a pre-field legacy meta — no live binary
	// writes one — so it names no server, and this server's silence about
	// its workspace says nothing. The prune keeps the file; the write must
	// refuse rather than overwrite the only record of a session that may be
	// alive on a herdr nobody here can name (rangerhq-y4z, closing the board
	// rangerhq-jeu2 left open).
	t.Run("a pre-field meta refuses on the default socket too", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "legacy", Cmd: "claude"})
		m, _ := b.readMeta("legacy")
		ws := m.Workspace
		m.Socket = "" // a meta from before socket: existed
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}}) // absent from this server

		err := b.CreateSession(NewSessionOpts{Name: "legacy", Cmd: "claude", Dir: t.TempDir()})
		if err == nil {
			t.Fatal("overwrote a meta naming no server at all (rangerhq-y4z)")
		}
		if now, ok := b.readMeta("legacy"); !ok || now.Workspace != ws {
			t.Fatalf("the meta was overwritten behind the refusal: %+v", now)
		}
	})

	// The listing is fetched for one question only — is it empty. Whether
	// this workspace appears in it is exactly the snapshot ADR 0011 §2
	// forbids reading as a fact, and hiding it must change nothing.
	t.Run("a non-empty listing is not consulted for this workspace", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "newborn", Cmd: "claude"})
		m, _ := b.readMeta("newborn")
		if err := os.WriteFile(filepath.Join(fake, "hidden-from-list"), []byte(m.Workspace), 0o644); err != nil {
			t.Fatal(err)
		}

		err := b.CreateSession(NewSessionOpts{Name: "newborn", Cmd: "claude", Dir: t.TempDir()})
		if err == nil {
			t.Fatal("a workspace hidden from the listing but alive was overwritten")
		}
		if now, ok := b.readMeta("newborn"); !ok || now.Workspace != m.Workspace {
			t.Fatalf("meta lost: %+v", now)
		}
	})
}

// rangerhq-cpeh: the other half of the same snapshot. 9nso stopped the
// prune from deleting a live session's meta; the create beside it still
// read the same listing and wrote over one. A spared meta is left out of
// that pass's listing by design, so HasSession says no, CreateSession makes
// a second workspace under the label and writeMeta()s over the record of
// the first — the live one is orphaned with its agent running, which is the
// incident's harm reached by the write instead of the delete. So a create
// proves death too, by the same per-id query, before it touches a meta that
// names a workspace.
func TestCreateMustProveDeathBeforeOverwritingAMeta(t *testing.T) {
	const sock = "/tmp/this/herdr.sock"

	// setup is the incident's board: the racing pass's own session (so the
	// listing is never empty, keeping the rangerhq-8fq guards quiet) plus
	// the session another pass just created. Returns the latter's workspace.
	setup := func(t *testing.T) (*HerdrBackend, string, string) {
		t.Helper()
		t.Setenv("HERDR_SOCKET_PATH", sock)
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "newborn", Cmd: "claude"})
		m, ok := b.readMeta("newborn")
		if !ok {
			t.Fatal("no meta for newborn")
		}
		os.Remove(filepath.Join(fake, "calls.log"))
		return b, fake, m.Workspace
	}
	hideFromList := func(t *testing.T, fake, id string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fake, "hidden-from-list"), []byte(id), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The incident, by the write: alive on the server, absent from this
	// pass's listing.
	t.Run("workspace alive but missing from the listing", func(t *testing.T) {
		b, fake, ws := setup(t)
		hideFromList(t, fake, ws)
		if b.HasSession("newborn") {
			t.Fatal("setup: the session was supposed to be invisible to this pass")
		}

		err := b.CreateSession(NewSessionOpts{Name: "newborn", Cmd: "claude", Dir: t.TempDir()})
		if err == nil {
			t.Fatal("created a session over a live workspace's meta (rangerhq-cpeh)")
		}
		m, ok := b.readMeta("newborn")
		if !ok || m.Workspace != ws {
			t.Fatalf("the live session's meta was overwritten: %+v", m)
		}
		if log := calls(t, fake); strings.Contains(log, "workspace create") {
			t.Errorf("a second workspace was created under the label before the refusal:\n%s", log)
		}
		if alive, aerr := b.H.WorkspaceAlive(ws); aerr != nil || !alive {
			t.Errorf("setup: %s should still be alive (alive=%v err=%v)", ws, alive, aerr)
		}
	})

	// The refusal has to be readable: which workspace, and what to do.
	t.Run("the refusal names the workspace", func(t *testing.T) {
		b, fake, ws := setup(t)
		hideFromList(t, fake, ws)

		err := b.CreateSession(NewSessionOpts{Name: "newborn", Dir: t.TempDir()})
		if err == nil || !strings.Contains(err.Error(), ws) || !strings.Contains(err.Error(), "attach newborn") {
			t.Errorf("refusal does not name the workspace and the way out: %v", err)
		}
	})

	// Creating a name must still work: this is the ordinary path, and it
	// is what makes a dead session's name reusable.
	t.Run("workspace genuinely gone", func(t *testing.T) {
		b, fake, ws := setup(t)
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}}) // newborn's workspace closed

		if err := b.CreateSession(NewSessionOpts{Name: "newborn", Cmd: "claude", Dir: t.TempDir()}); err != nil {
			t.Fatalf("a name whose workspace this server confirms is gone must be reusable: %v", err)
		}
		m, ok := b.readMeta("newborn")
		if !ok || m.Workspace == ws || m.Workspace == "" {
			t.Fatalf("the new workspace was not recorded: %+v", m)
		}
		if log := calls(t, fake); !strings.Contains(log, "workspace get "+ws) {
			t.Errorf("overwrote a meta without asking herdr about its workspace:\n%s", log)
		}
	})

	// And at once — the prune's 5m grace is about a listing's weakness and
	// has no business in front of a direct answer. A meta seconds old whose
	// workspace herdr says is gone does not lock its name for five minutes.
	t.Run("a fresh meta whose workspace is gone does not hold the name", func(t *testing.T) {
		b, fake, _ := setup(t)
		m, _ := b.readMeta("newborn")
		if age := time.Since(m.Launched); age > PruneGrace {
			t.Fatalf("setup: meta should be well inside the grace, is %s old", age)
		}
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})

		if err := b.CreateSession(NewSessionOpts{Name: "newborn", Cmd: "claude", Dir: t.TempDir()}); err != nil {
			t.Fatalf("the prune grace must not gate the create: %v", err)
		}
	})

	// A server that did not answer has said nothing about the workspace.
	// Silence is not evidence of death on this side either, and here the
	// unrecoverable direction is the write.
	t.Run("herdr does not answer the query", func(t *testing.T) {
		b, fake, ws := setup(t)
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})
		if err := os.WriteFile(filepath.Join(fake, "workspace-get-unreachable"), nil, 0o644); err != nil {
			t.Fatal(err)
		}

		err := b.CreateSession(NewSessionOpts{Name: "newborn", Cmd: "claude", Dir: t.TempDir()})
		if err == nil {
			t.Fatal("overwrote a meta on a query that errored — silence is not evidence of death")
		}
		if m, ok := b.readMeta("newborn"); !ok || m.Workspace != ws {
			t.Fatalf("meta lost behind an unanswered query: %+v", m)
		}
	})

	// Nothing to ask about is not the same as an answer — but nothing is
	// orphaned either, so this one proceeds where the prune abstains.
	t.Run("meta naming no workspace", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", sock)
		b, _ := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		os.WriteFile(b.metaPath("nameless"),
			[]byte("name: nameless\nworkspace: \npane: \nemoji: x\nsocket: "+sock+"\n"), 0o644)

		if err := b.CreateSession(NewSessionOpts{Name: "nameless", Dir: t.TempDir()}); err != nil {
			t.Fatalf("a meta naming no workspace holds nothing alive: %v", err)
		}
	})

	// The common path pays nothing: a name with no meta asks herdr about no
	// workspace at all.
	t.Run("a name with no meta asks nothing", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", sock)
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		os.Remove(filepath.Join(fake, "calls.log"))

		mustCreate(t, b, NewSessionOpts{Name: "fresh"})
		if log := calls(t, fake); strings.Contains(log, "workspace get") {
			t.Errorf("per-id query on a name that has no meta:\n%s", log)
		}
	})
}
