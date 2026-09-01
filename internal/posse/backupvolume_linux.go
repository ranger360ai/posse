//go:build linux

package posse

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// localFSMagic is statfs(2)'s f_type for every filesystem this build has
// been read against AS ON THIS BOX. Linux has no MNT_LOCAL flag to ask, so
// the reading has to be a list — and the list is an ALLOWLIST, which is the
// only shape a refusal can take: a type neither this map nor networkFSName
// knows is refused with its number, because an archive written to a volume
// posse cannot classify is an archive whose location is an assumption. The
// numbers are linux/magic.h.
var localFSMagic = map[int64]string{
	0xef53:     "ext2/3/4",
	0x58465342: "xfs",
	0x9123683e: "btrfs",
	0x2fc12fc1: "zfs",
	0x01021994: "tmpfs",
	0x858458f6: "ramfs",
	0x4d44:     "vfat",
	0x5346544e: "ntfs",
	0x52654973: "reiserfs",
	0x3153464a: "jfs",
	0xf15f:     "ecryptfs",
	0x2011bab0: "exfs",
}

// networkFSName names the ones that are certainly NOT on this box. It
// decides nothing — the allowlist above already refuses anything missing
// from it — and exists so the refusal LINE can say "nfs" instead of a bare
// hex number, which is the difference between a refusal an operator can act
// on and one they have to research. fuse is in it because that is where
// sshfs, davfs2 and every other userspace remote arrives; a local
// fuse-mounted volume is refused too, and that is the intended direction of
// the error.
var networkFSName = map[int64]string{
	0x6969:     "nfs",
	0x517b:     "smb",
	0xff534d42: "cifs/smb2",
	0x73757245: "coda",
	0x01021997: "v9fs/9p",
	0x47504653: "gpfs",
	0x00c36400: "ceph",
	0x65735546: "fuse",
	0x5346414f: "afs",
	0x6b414653: "afs (openafs)",
}

func volumeOf(path string) (volume, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return volume{}, Die("statfs %s: %v", AbbrevHome(path), err)
	}
	t := int64(st.Type)
	v := volume{FreeBytes: uint64(st.Bavail) * uint64(st.Bsize)}
	if name, ok := localFSMagic[t]; ok {
		v.FSType, v.Local = name, true
		return v, nil
	}
	if name, ok := networkFSName[t]; ok {
		v.FSType = name
		return v, nil
	}
	v.FSType = fmt.Sprintf("unrecognized fs type 0x%x", t)
	return v, nil
}
