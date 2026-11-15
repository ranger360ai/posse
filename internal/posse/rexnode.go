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
// Where rex keeps it: $XDG_STATE_HOME/rex/tsnet, else the XDG state default
// for the OS — ~/.local/state/rex/tsnet on linux (MEASURED 2026-11-13, linux:
// the node dir is there with XDG_STATE_HOME unset, and the rex binary carries
// the XDG_STATE_HOME name) and ~/Library/Application Support/rex/tsnet on
// darwin (ASSUMED: /usr/bin/rex links github.com/adrg/xdg v0.5.3, whose
// paths_darwin.go defaults stateHome there, and whose paths_unix.go default
// is the linux path the linux reading confirms — so the library is the one
// that picks both, but the darwin path itself is unmeasured; the measurement
// ask is the bead under "Not closed here" below). ALL of them are masked
// whenever they resolve, for both homes and on every GOOS — wider than rex's
// own resolution, which is the safe direction for a mask, and what keeps this
// a pure function the pin can run anywhere. An absent dir costs nothing:
// bwrap's step 5b skips one and a seatbelt subpath deny on an absent path is
// inert.
//
// Not closed here:
// - ranger-base-1mxhx

import (
	"os"
	"path/filepath"
)

// rexNodeStateDirs is the list for one account's answers: both OSes' XDG state
// defaults under each home, and XDG_STATE_HOME's when it is an absolute path
// (the XDG spec calls a relative value invalid, and a mask does not guess what
// it meant). Both defaults unconditionally, not behind runtime.GOOS, so this
// stays a pure function and the pin runs on either OS.
func rexNodeStateDirs(homes []string, xdgState string) []string {
	var out []string
	for _, h := range homes {
		if h == "" {
			continue
		}
		out = append(out,
			absResolve(filepath.Join(h, ".local", "state", "rex", "tsnet")),
			absResolve(filepath.Join(h, "Library", "Application Support", "rex", "tsnet")))
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
