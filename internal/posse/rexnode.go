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
	"strings"
)

// rexNodeExpandHome is github.com/adrg/xdg v0.5.3's
// internal/pathutil.ExpandHome, transcribed (pathutil_unix.go:28-40, read
// 2026-11-21) — the one function rex's resolution of XDG_STATE_HOME runs
// BEFORE it asks whether the value is absolute, and the one this mask used to
// skip (ranger-base-3cjzs).
//
// TRANSCRIBED AND NOT TIDIED, because every "tidier" rule is a hole. The test
// is `path[0] == '~'`, so `~st` is $HOME/st and NOT some user st's home; the
// other is `strings.HasPrefix(path, "$HOME")`, so `$HOMEx` is $HOME/x. A
// '~/'-and-'$HOME/'-only rule would leave both of those unmasked at names rex
// writes to. `~` and `$HOME` alone both Join to the home itself.
//
// home is a PARAMETER and not a reading: the mask cannot know which home rex
// ran under, so it asks for every candidate (see rexNodeStateDirs), and the
// create asks for exactly rex's own. An empty home expands nothing, which is
// also what the library does with one.
func rexNodeExpandHome(home, path string) string {
	if path == "" || home == "" {
		return path
	}
	if path[0] == '~' {
		return filepath.Join(home, path[1:])
	}
	if strings.HasPrefix(path, "$HOME") {
		return filepath.Join(home, path[5:])
	}
	return path
}

// rexNodeXDGStateDir is rex's node dir under XDG_STATE_HOME as resolved
// against ONE home, or "" when that value is not one rex would take: expand,
// then IsAbs, which is pathutil.EnvPath's own order (pathutil.go:93-100). A
// value still relative after expansion is invalid by the XDG spec and the
// library ignores it too, so a mask does not guess what it meant.
func rexNodeXDGStateDir(home, xdgState string) string {
	v := rexNodeExpandHome(home, xdgState)
	if !filepath.IsAbs(v) {
		return ""
	}
	return absResolve(filepath.Join(v, "rex", "tsnet"))
}

// rexNodeStateDirs is the list for one account's answers: both OSes' XDG state
// defaults under each home, and XDG_STATE_HOME's wherever it resolves to an
// absolute path the way the xdg library rex links resolves it. Both defaults
// unconditionally, not behind runtime.GOOS, so this stays a pure function and
// the pin runs on either OS.
//
// THE XDG VALUE IS PER-HOME, which is why it is a loop and not one entry
// (ranger-base-3cjzs). `XDG_STATE_HOME=~/st` is a value rex expands against
// its OWN $HOME before testing it, so there is no single answer to mask — and
// reading the raw value through filepath.IsAbs, as this did, masked neither
// the expansion rex uses nor anything else: the arm contributed nothing and
// the GOOS defaults it fell back to are not where rex wrote. Every candidate
// home gets a row instead, "/" among them because that is the library's own
// fallback when $HOME is empty (pathutil_unix.go:14). An already-absolute
// value expands to itself under every home and dedupes back to one row, in
// the position it has always had.
//
// AND THE GOOS DEFAULTS STAY whatever XDG_STATE_HOME says, which they did
// before and which is still right: wider is the safe direction for a mask,
// and an entry rex does not use costs one inert tmpfs.
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
	for _, h := range append([]string{"/"}, homes...) {
		if d := rexNodeXDGStateDir(h, xdgState); d != "" {
			out = append(out, d)
		}
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
//
// ONE RESOLUTION, SHARED WITH THE MASK (ranger-base-3cjzs). The XDG arm is
// rexNodeXDGStateDir against this function's own home, so a `~`- or
// `$HOME`-prefixed XDG_STATE_HOME creates and anchors the directory rex will
// actually write to rather than the GOOS default it would then not use. The
// two arms cannot drift because there is one expansion.
//
// WITH NO $HOME an expandable value creates NOTHING, which is where this
// stops short of the library: there it would expand against "/". Same rule as
// the no-home default above and for the same reason — the fail-closed
// direction for a mkdir is to make no directory at all — and the mask carries
// the "/" row either way, so nothing goes uncovered.
func rexNodeStateDirNative(goos, home, xdgState string) string {
	if d := rexNodeXDGStateDir(home, xdgState); d != "" {
		return d
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
