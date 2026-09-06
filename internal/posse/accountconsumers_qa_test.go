//go:build posse_arm2

package posse

// QA pins for the two account-stage CONSUMERS (ranger-base-pjoy item 1,
// ADR 0017 §3).
//
// Both readers used to decide with `s.Runtime != "claude"` — an ADR 0017 §3
// shadow predicate, a runtime NAME standing in for the dimension "does
// anything read this runtime's spend". They now ask the adapter registry,
// but nothing drove either one: no test in the tree called CountUncounted
// at all, so the name could have been put back and the suite would not have
// noticed. That is the same shape the predicate was in before it was fixed.
//
// So this drives the consumers, not the getter: sessions are created
// through the real backend and read back through Sessions(), and grok is
// the arm that matters — it has an adapter and is not claude, so it is
// exactly what the old predicate got wrong (it gained one in ranger-base-k7nb
// and read as uncounted for two days).

import (
	"os"
	"path/filepath"
	"testing"
)

// uncountedRuntime declares a runtime with no cost adapter, since every
// builtin has one. Derived rather than spelled: a builtin gaining or losing
// an adapter must not quietly turn this fixture into its opposite.
func uncountedRuntime(t *testing.T, a *App) string {
	t.Helper()
	const name = "mycli"
	if _, ok := CostProviderFor(name); ok {
		t.Fatalf("%s has a cost adapter; this fixture needs a runtime nothing reads", name)
	}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), name+".yaml"), []byte("command: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestQACountUncountedAsksTheAdapterNotTheName(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	none := uncountedRuntime(t, b.App)

	// One session per runtime. grok and codex are the load-bearing rows:
	// both have adapters and neither is claude.
	for _, c := range []struct{ name, runtime string }{
		{"s-claude", "claude"},
		{"s-grok", "grok"},
		{"s-codex", "codex"},
		{"s-none", none},
	} {
		mustCreate(t, b, NewSessionOpts{Name: c.name, Agent: "ranger", Runtime: c.runtime})
	}

	// The rig must be shown to produce what the consumer reads before its
	// answer means anything: a Sessions() that returned nothing would make
	// every assertion below true for no reason.
	ss, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range ss {
		got[s.Name] = s.Runtime
	}
	if len(got) != 4 || got["s-grok"] != "grok" || got["s-none"] != none || got["s-claude"] != "claude" {
		t.Fatalf("the fixture did not reach the consumer: %+v", got)
	}

	var rep CostReport
	rep.CountUncounted(b)

	if rep.Uncounted != 1 {
		t.Errorf("Uncounted = %d, want 1 — three of these four runtimes have an adapter", rep.Uncounted)
	}
	if len(rep.UncountedRuntimes) != 1 || rep.UncountedRuntimes[0] != none {
		t.Errorf("UncountedRuntimes = %v, want [%s]", rep.UncountedRuntimes, none)
	}
	// Named negatively too: the failure this replaced did not drop the
	// count, it put the WRONG runtime in it, and an operator reading
	// "1 grok session(s) uncounted" goes looking for a gap that is not there.
	for _, r := range CountedRuntimes() {
		for _, u := range rep.UncountedRuntimes {
			if u == r {
				t.Errorf("%s has an adapter; naming it uncounted is the defect: %v", r, rep.UncountedRuntimes)
			}
		}
	}
}

// A workspace posse did not create is not persona spend, and counting it
// would put a stranger's shell into the operator's uncounted total. Pinned
// through a foreign row rather than a hand-written meta because that is the
// reachable source of one: a foreign workspace has no meta, so it arrives
// with no Agent and no Runtime.
//
// What that costs, said out loud: the two guards are `Agent == "" ||
// Runtime == ""` and this row trips BOTH, so removing either one alone
// leaves this green (measured). It holds the invariant — dropping the pair
// reds it — not the individual guard. A row with a persona-less session on
// a named runtime would separate them, and nothing in the real backend
// produces one.
func TestQACountUncountedSkipsForeignSessions(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	none := uncountedRuntime(t, b.App)
	mustCreate(t, b, NewSessionOpts{Name: "mine", Agent: "ranger", Runtime: none})

	ws := fakeLoadWSFrom(t, fake)
	ws = append(ws, fakeWS{WorkspaceID: "w9", Label: "handmade", AgentStatus: "working"})
	saveWSTo(t, fake, ws)

	// The rig must be shown to produce the foreign row: without it this
	// test is the previous one with a longer name.
	ss, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	foreign := 0
	for _, s := range ss {
		if s.Foreign {
			foreign++
		}
	}
	if len(ss) != 2 || foreign != 1 {
		t.Fatalf("want one own and one foreign session, got %+v", ss)
	}

	var rep CostReport
	rep.CountUncounted(b)
	if rep.Uncounted != 1 || len(rep.UncountedRuntimes) != 1 || rep.UncountedRuntimes[0] != none {
		t.Errorf("Uncounted = %d %v, want 1 [%s] — the foreign row is not persona spend",
			rep.Uncounted, rep.UncountedRuntimes, none)
	}
}
