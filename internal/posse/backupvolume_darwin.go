//go:build darwin

package posse

import (
	"strings"

	"golang.org/x/sys/unix"
)

// volumeOf on darwin asks statfs(2). Two fields answer the two questions
// `posse backup` has to settle before it will write anything:
//
//	f_flags & MNT_LOCAL   the kernel's own answer to "is this volume on this
//	                      box". It is set by the mount, so it covers every
//	                      remote filesystem by construction — nfs, smbfs,
//	                      afpfs, webdav, an sshfs through macFUSE — including
//	                      the ones no allowlist of type names would have.
//	f_fstypename          for the refusal LINE only. Naming the type is what
//	                      turns "refused" into something the operator can act
//	                      on, and it is never what decides.
//
// Bavail, not Bfree: the reserve is not ours to spend.
func volumeOf(path string) (volume, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return volume{}, Die("statfs %s: %v", AbbrevHome(path), err)
	}
	name := make([]byte, 0, len(st.Fstypename))
	for _, c := range st.Fstypename {
		if c == 0 {
			break
		}
		name = append(name, byte(c))
	}
	return volume{
		FSType:    strings.TrimSpace(string(name)),
		Local:     st.Flags&unix.MNT_LOCAL != 0,
		FreeBytes: st.Bavail * uint64(st.Bsize),
	}, nil
}
