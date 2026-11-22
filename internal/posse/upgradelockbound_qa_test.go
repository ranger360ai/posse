//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-qei5l, verifying ranger-base-u66yb's close: the bound itself.
//
// u66yb's whole deliverable is that `posse upgrade` stops waiting forever on
// a lock any seat can take and hold, and its one ASSUMED number is
// UpgradeLockWait. Three pins landed with that close and all three drive
// lockUpgradeWithin with a bound of their OWN — 300ms, 1s, 30s. Production
// reaches the bound only through lockUpgrade, which is wired in as
// upgradeReads.Lock (upgrade.go, upgradeDefaultReads) and which every
// verb-level test replaces with a fake. So nothing observed the bound.
//
// MEASURED 2026-11-22 (this box, linux 7.2.15-1-omarchy, go1.27.3), before
// this file existed: `const UpgradeLockWait = 1000 * time.Hour` — the defect
// u66yb was filed on, an unbounded wait, restored in one line — left
// internal/posse arms 1, 2 and 3, cmd/posse and internal/treepins all `ok`,
// with gofmt and `go vet` clean. A census settles it the same way and
// without sampling: no `*_test.go` in the tree names UpgradeLockWait or
// upgradeLockPoll, and none calls lockUpgrade.
//
// This is the class upgrade_test.go's own "the wiring" header
// (ranger-base-anczm) was written about — a guardrail pinned as a FUNCTION
// while nothing reaches the verb that calls it — one landing later and one
// constant over.
//
// Both arms are source-level on purpose. The behavioural pin cannot be
// written: asserting the production bound by WAITING for it costs an hour,
// and lockUpgrade takes no bound to shorten.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"
)

// A bound a human would call bounded. The floor is what makes it a queue
// rather than a refusal — a second delegate must still wait behind a real,
// slow upgrade (ADR 0092's 15-25 minute planning envelope) — and the ceiling
// is what makes it a bound at all. 60m sits between them with the reasoning
// on u66yb; this arm only refuses the values that make the verb a different
// verb.
func TestUpgradeLockWaitIsBounded(t *testing.T) {
	t.Parallel()
	if UpgradeLockWait <= 0 {
		t.Fatalf("UpgradeLockWait = %s: a non-positive bound makes `posse upgrade` refuse the moment any seat holds the lock, where u66yb's close promises a second delegate QUEUES behind a real upgrade", UpgradeLockWait)
	}
	if UpgradeLockWait < 25*time.Minute {
		t.Errorf("UpgradeLockWait = %s, under ADR 0092's 25-minute planning envelope — a genuine slow upgrade would be refused as if it were a squatter", UpgradeLockWait)
	}
	if UpgradeLockWait > 2*time.Hour {
		t.Errorf("UpgradeLockWait = %s: that is the unbounded wait ranger-base-u66yb was filed on, respelled as a number. Any seat can take ~/.cache/posse/upgrade.lock and hold it, so the bound is the whole control", UpgradeLockWait)
	}
	if upgradeLockPoll <= 0 || upgradeLockPoll > UpgradeLockWait {
		t.Errorf("upgradeLockPoll = %s against a bound of %s: a poll that is non-positive spins and one past the bound never retries", upgradeLockPoll, UpgradeLockWait)
	}
}

// And the wiring, which the range above cannot see: lockUpgrade must hand
// lockUpgradeWithin the CONSTANT. Unwiring it — a literal, or another
// duration — is the same defect with the constant left correct and
// unreferenced, and it is what the census found nothing watching.
func TestLockUpgradePassesUpgradeLockWaitToTheBoundedForm(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "upgrade.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var fn *ast.FuncDecl
	ast.Inspect(f, func(n ast.Node) bool {
		if d, ok := n.(*ast.FuncDecl); ok && d.Recv == nil && d.Name.Name == "lockUpgrade" {
			fn = d
		}
		return true
	})
	if fn == nil {
		t.Fatal("no func lockUpgrade in upgrade.go — this pin reads it by name, so a rename must red here rather than pass at zero")
	}

	// The positive control for the scan: the body has to contain a call to
	// the bounded form at all. Without this, a lockUpgrade emptied to
	// `return nil, nil` would satisfy every assertion below by having no
	// call to disagree with.
	var calls int
	var waitArg string
	ast.Inspect(fn, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := c.Fun.(*ast.Ident)
		if !ok || id.Name != "lockUpgradeWithin" {
			return true
		}
		calls++
		if len(c.Args) >= 2 {
			if a, ok := c.Args[1].(*ast.Ident); ok {
				waitArg = a.Name
			} else {
				waitArg = "<not an identifier>"
			}
		}
		return true
	})
	if calls != 1 {
		t.Fatalf("lockUpgrade makes %d calls to lockUpgradeWithin, want exactly 1 — the scan must have found the one call whose bound it is grading", calls)
	}
	if waitArg != "UpgradeLockWait" {
		t.Errorf("lockUpgrade passes %q as lockUpgradeWithin's bound, want UpgradeLockWait — a literal there leaves the constant correct, unreferenced and unpinned, which is how an unbounded wait gets back in (ranger-base-u66yb)", waitArg)
	}
}
