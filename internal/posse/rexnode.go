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
// ask is the bead under "Not closed here" below. The library reading was
// CONFIRMED against the module source on 2026-11-21, ranger-base-bncbk:
// paths_darwin.go:32 is `baseDirs.stateHome = EnvPath(envStateHome,
// filepath.Join(home, "Library", "Application Support"))`, paths_unix.go:25
// the `.local/state` twin, and `home` is pathutil.UserHomeDir — $HOME else
// "/", never passwd. That is still a reading of the library and not of a
// darwin box, so the label stays ASSUMED). ALL of them are masked whenever
// they resolve, for both homes and on every GOOS — wider than rex's own
// resolution, which is the safe direction for a mask, and what keeps this a
// pure function the pin can run anywhere.
//
// AN ABSENT DIR USED TO COST NOTHING AND THAT WAS THE HOLE
// (ranger-base-bncbk). A seatbelt subpath deny on an absent path is inert, so
// that half is unchanged; bwrap's step 5b, though, SKIPPED an absent dir —
// no tmpfs, no watch — so the first `rex` start during a live seat's life
// wrote the tailnet identity at a name nothing covered, readable by every
// bwrap seat until it ended, with no mount for `posse wall-watch` to see
// detach. 5b now anchors every dir it masks and CREATES the absent one, 0700
// — but only at rexNodeStateDirNative, the one path rex resolves on this
// GOOS. The other entries keep skipping when absent: nothing on this OS
// writes there, and a posse-made ~/Library/Application Support/rex/tsnet on
// linux would be a directory invented by the launcher and then masked
// forever.
//
// Not closed here:
// - ranger-base-1mxhx

import (
	"os"
	"path/filepath"
	"runtime"
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

// rexNodeStateDirNative is the ONE member of rexNodeStateDirs that rex
// resolves on this GOOS, and so the only one posse may create when it is
// absent (SeatbeltCarveOut.MakesDenyReadDir, ranger-base-bncbk). "" when
// there is no home to resolve against and no absolute XDG_STATE_HOME — then
// nothing is created, which is the fail-closed direction for a mkdir.
//
// goos and home are PARAMETERS and not readings, so the pin drives both OSes
// from either, exactly as rexNodeStateDirs takes its homes.
//
// $HOME AND NOT THE PASSWD HOME, which is the one place this function differs
// from the mask's two-home list: the xdg library rex links resolves its base
// dirs from `os.Getenv("HOME")` else "/" and reads the passwd database not at
// all (github.com/adrg/xdg v0.5.3, internal/pathutil/pathutil_unix.go:14,
// read 2026-11-21). The mask is deliberately wider than that; a create has to
// be exactly it.
func rexNodeStateDirNative(goos, home, xdgState string) string {
	if filepath.IsAbs(xdgState) {
		return absResolve(filepath.Join(xdgState, "rex", "tsnet"))
	}
	if home == "" {
		return ""
	}
	if goos == "darwin" {
		return absResolve(filepath.Join(home, "Library", "Application Support", "rex", "tsnet"))
	}
	return absResolve(filepath.Join(home, ".local", "state", "rex", "tsnet"))
}

// withRexNodeReadDeny adds this process's rexNodeStateDirs to
// carve.DenyReadDir, and its native path to carve.DenyReadDirMake. Called
// beside withSSHKeyReadDeny at every render — both seat walls, both reports,
// and the landed-check judge's cage.
func withRexNodeReadDeny(carve *SeatbeltCarveOut) {
	xdg := os.Getenv("XDG_STATE_HOME")
	carve.DenyReadDir = dedupeStrings(append(carve.DenyReadDir, rexNodeStateDirs(liveSSHHomes(), xdg)...))
	if native := rexNodeStateDirNative(runtime.GOOS, os.Getenv("HOME"), xdg); native != "" {
		carve.DenyReadDirMake = dedupeStrings(append(carve.DenyReadDirMake, native))
	}
}
