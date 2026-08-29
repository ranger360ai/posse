package rhq

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
	"syscall"
	"testing"
	"time"
)

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
	os.Exit(m.Run())
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
		// The scan failing the way it fails in the wild: bd exits non-zero
		// with a word on stderr, and the repo's queue is unknown, not empty
		// (rangerhq-llse).
		if _, err := os.Stat("fake-ready-fail"); err == nil {
			fmt.Fprint(os.Stderr, "database is locked")
			return 1
		}
		if b, err := os.ReadFile("fake-ready.json"); err == nil {
			fmt.Print(fakeBdApplyState(string(b)))
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
		file, want := "fake-list.json", ""
		for i, a := range args {
			if a == "--label-any" {
				file = "fake-list-labeled.json"
				if i+1 < len(args) {
					want = args[i+1]
				}
			}
		}
		if b, err := os.ReadFile(file); err == nil {
			if want == "" {
				fmt.Print(string(b))
			} else {
				fmt.Print(fakeBdFilterLabels(string(b), want))
			}
		} else {
			fmt.Print("[]")
		}
		return 0
	case "blocked": // blocked --json → fake-blocked.json (the whole graph, one call)
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
				// A `fake-dep-add-fail` marker is bd's own worst shape,
				// opt-in: exit 0 with nothing wrong on the wire and no edge
				// in the graph (the muoo class). It is what makes a caller
				// that reads the graph back distinguishable from one that
				// trusts the status.
				if _, err := os.Stat("fake-dep-add-fail"); err != nil {
					fakeBdAddDep(args[i+2])
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
			fakeBdAppendCreated(id, args)
			fmt.Fprint(os.Stderr, "Error: failed to read response: read unix ->bd.sock: i/o timeout")
			return 1
		}
		// A create that SUCCEEDS lands in the store too — the next `bd list
		// --all` sees it, which is how a verify bead filed on one pass
		// dedupes the next one while the watermark still holds its close in
		// view (ranger-base-muoo).
		fakeBdAppendCreated(id, args)
		fmt.Printf(`{"id":%q,"title":"created"}`, id)
		return 0
	case "comments": // comments <id> --json → fake-comments.json; comments add appends to it
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
		fmt.Print("{}")
		return 0
	case "sync":
		// The launcher's pre-commit export (ADR 0015 §4, queuejsonl.go).
		// bd owns the export; what a test can pin is that posse asked for
		// the git-free form, and bd-calls.log above is where it reads that.
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

func fakeDir() string { return os.Getenv("RHQ_FAKE_DIR") }

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
		fmt.Print("prompt$ echo hi\nhi\nprompt$\n\n\n\n")
		return 0
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
		if b, err := os.ReadFile(filepath.Join(fakeDir(), "prompt-barrier")); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && n > 0 {
				start := time.Now()
				fakeRecordPromptWindow(start, fakeAwaitPrompts(n))
			}
		} else if b, err := os.ReadFile(filepath.Join(fakeDir(), "prompt-delay-ms")); err == nil {
			if ms, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				start := time.Now()
				time.Sleep(time.Duration(ms) * time.Millisecond)
				fakeRecordPromptWindow(start, "delay")
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
// THIS call. explain-error-after holds a countdown of explains to answer
// normally first, and the error arms on the one after that — the shape of a
// herdr that went away mid-window (rangerhq-lhy2), where the early polls got
// real answers and a late one did not.
//
// It counts CALLS because the only other way to place an error late in the
// window is a wall clock, and a wall-clock timer races the launch's own
// setup: the first `agent explain` of a fake-herdr launch lands ~305ms after
// the test body starts on an idle box (measured 2026-08-29, 10 runs, spread
// 293-340ms) and later than that on a loaded one, so "arm it at 700ms,
// after some guesses" silently becomes "arm it before the first guess" and
// the test measures the opposite window — ranger-base-4pjw, ~1 red in 3 on
// the operator's box. Each fake call is its own process, so the count lives
// in the file, the way explain-fallback's does.
func fakeExplainErrorArmed() bool {
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
//	                  herdr's own working, which the guess shape carries in
//	                  the field and the seen shape does not need. Absent
//	                  means the key is absent, which is what an older herdr
//	                  emits and what WhatHerdrSaw must survive.
//	explain-error     `explain` fails outright (see the fake's error lever)
//	explain-error-after
//	                  a countdown: that many explains are answered before
//	                  explain-error arms, for a herdr that goes away partway
//	                  through the window
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
			rules := ""
			if b, err := os.ReadFile(filepath.Join(fakeDir(), "explain-rules")); err == nil {
				rules = `,"evaluated_rules":` + strings.TrimSpace(string(b))
			}
			return fmt.Sprintf(`{"state":%q,"matched_rule":null,"visible_idle":false,`+
				`"visible_blocker":false,"visible_working":false,`+
				`"fallback_reason":"default_known_agent_idle_fallback"%s}`, state, rules)
		}
	}
	rule := "fake_" + state
	if b, err := os.ReadFile(filepath.Join(fakeDir(), "explain-rule")); err == nil {
		rule = strings.TrimSpace(string(b))
	}
	return fmt.Sprintf(`{"state":%q,"matched_rule":{"id":%q,"state":%q},`+
		`"visible_idle":%t,"visible_blocker":%t,"visible_working":%t,`+
		`"fallback_reason":null}`,
		state, rule, state, state == "idle", state == "blocked", state == "working")
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

func newTestBackend(t *testing.T) (*HerdrBackend, string) {
	t.Helper()
	home := t.TempDir()
	fake := t.TempDir()
	// $HOME is the operator's real one unless a test says otherwise, and
	// plenty of what this package reads hangs off it — ~/.claude, ~/.grok,
	// ~/.codex, and DefaultWorktreeRoot's ~/.posse/worktrees. That last one
	// is not a read: a test reaching EnsureSessionTree cut a real git
	// worktree in the operator's live ~/.posse (ranger-base-gvrh). Every
	// backend test gets a temp HOME, the way wtApp already does.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("RHQ_FAKE_HERDR", "1")
	t.Setenv("RHQ_FAKE_DIR", fake)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := NewAppAt(home)
	// Hermetic by construction, like RHQ_FAKE_HERDR above: an unconfigured
	// lister reads no keychain and reaches no network, and the availability
	// preflight takes that as UNKNOWN and launches the tier exactly as asked
	// (modelavail.go). Tests that want the preflight to DO something seed
	// the snapshot or set this field.
	a.ModelLister = &ModelLister{}
	// Same reason, one guard further on: the load guard (loadguard.go)
	// reads the 1-minute load average of whatever box the suite is running
	// on, and a suite that goes red because something ELSE saturated the
	// machine is red per-day, not per-commit. Quiet by construction; the
	// tests that want the guard to fire set this field.
	a.Load1 = func() (float64, error) { return 0, nil }
	b := &HerdrBackend{App: a, H: Herdr{Bin: exe}}
	captureWarn(t, b)
	return b, fake
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
	b, _ := newTestBackend(t)
	err := b.H.FocusWorkspace("w404")
	if err == nil || !strings.Contains(err.Error(), "workspace_not_found") {
		t.Errorf("want workspace_not_found error, got %v", err)
	}
}

func TestPersonaLaunch(t *testing.T) {
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
	b, fake := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	persona := "---\nname: ranger\ndescription: test\ncommand: claude {allow} {deny}\n" +
		"allow:\n  - Bash(bd:*)\n  - Edit\ndeny:\n  - Bash(git push:*)\n---\nYou are ranger.\n"
	os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(persona), 0o644)

	mustCreate(t, b, NewSessionOpts{Name: "crew", Agent: "ranger"})

	log := calls(t, fake)
	for _, want := range []string{
		"--env RHQ_TOOLS_ALLOW=Bash(bd:*)\nEdit",
		// The env carries the PID's own rules; only the typed line is widened
		// into claude's option-blind spellings (L0Spellings, rangerhq-3mc) —
		// the L3 pre-push hook reads RHQ_TOOLS_DENY and must keep seeing the
		// rule as the PID wrote it.
		"--env RHQ_TOOLS_DENY=Bash(git push:*)",
		"GATES claude --allowedTools 'Bash(bd:*)' 'Edit' " +
			"--disallowedTools 'Bash(git push:*)' 'Bash(git -* push *)'",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("calls.log missing %q:\n%s", want, log)
		}
	}
}

func TestBdClaimClose(t *testing.T) {
	_, fake := newTestBackend(t)
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}

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
	b, _ := newTestBackend(t)
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}

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
	exe, _ := os.Executable()
	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.Bd = Bd{Bin: exe}
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

func dispatcherOut(d *Dispatcher) string { return d.Out.(*strings.Builder).String() }

func writePersona(t *testing.T, a *App, name, labels string) {
	t.Helper()
	os.MkdirAll(a.AgentsDir, 0o755)
	md := "---\nname: " + name + "\ndescription: test\nlabels: " + labels + "\n---\nYou are " + name + ".\n"
	os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644)
}

func TestDispatchRun(t *testing.T) {
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

	// A busy persona session must not be double-prompted.
	ws := fakeLoadWSFrom(t, fake)
	ws[0].AgentStatus = "working"
	saveWSTo(t, fake, ws)
	if _, err := d.LaunchBead(is); err == nil || !strings.Contains(err.Error(), "working") {
		t.Errorf("busy session should refuse dispatch, got %v", err)
	}

	// Unroutable beads error instead of silently vanishing.
	orphan := RepoIssue{BdIssue: BdIssue{ID: "a-2", Labels: []string{"mystery"}}, Dir: repo}
	if _, err := d.LaunchBead(orphan); err == nil || !strings.Contains(err.Error(), "unroutable") {
		t.Errorf("unroutable bead should error, got %v", err)
	}
}

func TestDispatchRouting(t *testing.T) {
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
	if !strings.Contains(log, "GATES claude --model 'claude-fable-5' "+ClaudeFleetFlags+" --append-system-prompt") || !strings.Contains(log, "GATES codex -c model='gpt-5.6-sol' -s read-only -a never --disable hooks -c allow_login_shell=false -c \"projects=") {
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
	if got := calls(t, fake); !strings.Contains(got, `GATES grok `+GrokFleetFlags+` --rules="$(cat '`) {
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

	// A pass that does not know a concrete socket cannot stamp one: "" is
	// herdr's default server, indistinguishable on disk from "unrecorded"
	// (rangerhq-y4z). It must leave the meta alone rather than claim it.
	t.Run("no backfill without a socket", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, _ := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "plain"})
		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if m, ok := b.readMeta("plain"); !ok || m.Socket != "" {
			t.Errorf("a pass on the default socket must not stamp one: %+v", m)
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

	// And the arm not taken, which is the whole reason it is not taken:
	// `posse` from a plain terminal has HERDR_SOCKET_PATH unset, so it writes
	// and reads unstamped metas against the default server. That is one
	// server talking about itself, its not_found IS evidence, and a dead
	// session's name stays reusable. Taking the prune's arm here would make
	// every name on that path unusable forever (rangerhq-y4z).
	t.Run("the default socket keeps a dead name reusable", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "alpha", Cmd: "claude"})
		if m, _ := b.readMeta("alpha"); m.Socket != "" {
			t.Fatalf("setup: expected an unstamped meta outside a pane, got socket %q", m.Socket)
		}
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}}) // alpha's workspace closed

		if err := b.CreateSession(NewSessionOpts{Name: "alpha", Cmd: "claude", Dir: t.TempDir()}); err != nil {
			t.Fatalf("a dead session's name must stay reusable on the default socket: %v", err)
		}
		if log := calls(t, fake); !strings.Contains(log, "workspace get ") {
			t.Errorf("the name was reused without asking herdr about its workspace:\n%s", log)
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
