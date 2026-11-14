package posse

// The rex tsnet node's state is read-denied to every seat (ranger-base-k06gw).
//
// rex embeds a tsnet node, and that node keeps its tailnet identity under
// rex's state dir: tailscaled.state, plus the profile and cert files beside
// it, all one credential. It is mode 600 and owned by the uid every seat runs
// as, so under bwrap's `--ro-bind / /` and seatbelt's allow-every-read it was
// readable from any seat. A seat that may DRIVE rex over the tailnet (the
// operator's ruling on ranger-base-p2tf5) needs none of it: the node is the
// rex server's, and a copy of its key is a tailnet identity that outlives the
// seat and the box.
//
// THE DIRECTORY, not the one file, and nothing bound back: carve.DenyReadDir,
// rendered by bwrap as an empty tmpfs and by seatbelt as a subpath read deny.
//
// EVERY SEAT, the pusher included. DIVERGED from the bead's "every seat that
// is not the pusher", in the safe direction: the pusher is exempt from the
// ssh mask because a push needs the key, and nothing a push does needs a
// tailnet identity.
//
// Where rex keeps it: $XDG_STATE_HOME/rex/tsnet, else ~/.local/state/rex/tsnet
// (MEASURED 2026-11-13, linux: the node dir is there with XDG_STATE_HOME
// unset, and the rex binary carries the XDG_STATE_HOME name). Both are
// masked whenever both resolve, for both homes — wider than rex's own
// resolution, which is the safe direction for a mask.
//
// Not closed here:
// - ranger-base-1mxhx

import (
	"os"
	"path/filepath"
)

// rexNodeStateDirs is the list for one account's answers: the default under
// each home, and XDG_STATE_HOME's when it is an absolute path (the XDG spec
// calls a relative value invalid, and a mask does not guess what it meant).
func rexNodeStateDirs(homes []string, xdgState string) []string {
	var out []string
	for _, h := range homes {
		if h == "" {
			continue
		}
		out = append(out, absResolve(filepath.Join(h, ".local", "state", "rex", "tsnet")))
	}
	if filepath.IsAbs(xdgState) {
		out = append(out, absResolve(filepath.Join(xdgState, "rex", "tsnet")))
	}
	return dedupeStrings(out)
}

// withRexNodeReadDeny adds this process's rexNodeStateDirs to
// carve.DenyReadDir. Called beside withSSHKeyReadDeny at every render — both
// seat walls, both reports, and the landed-check judge's cage.
func withRexNodeReadDeny(carve *SeatbeltCarveOut) {
	carve.DenyReadDir = dedupeStrings(append(carve.DenyReadDir, rexNodeStateDirs(liveSSHHomes(), os.Getenv("XDG_STATE_HOME"))...))
}
