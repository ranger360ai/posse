//go:build !darwin

package posse

import "context"

// sysSelfProcs off darwin is the two bounded `ps` reads the census has always
// taken — the table, then the argv of the rows that could be leaks — and it
// stays that, because the refusal that moved darwin's route is darwin's:
// seatbelt, and a setuid-root `/bin/ps` (proctable_darwin.go). ASSUMED, not
// measured here (this box is darwin): procps reads /proc and carries no
// privilege bit, so there is nothing for a Linux sandbox to refuse. A cage on
// a Linux box that DOES refuse the exec would show up the way darwin's did —
// exit 2 and "leak status unknown" from cmd/checkorphans, never a false
// clean — and the fix would be this file, reading /proc directly.
func sysSelfProcs(ctx context.Context) ([]Proc, error) {
	procs, err := sysProcTable(ctx)
	if err != nil {
		return nil, err
	}
	fillArgsForPIDs(ctx, procs, selfCheckSuspectPIDs(procs))
	return procs, nil
}
