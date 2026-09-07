package posse

import (
	"io/fs"
	"os"
	"syscall"
)

// WriteExecutable is internal/posse's execwrite.go WriteExecutable,
// duplicated here rather than imported: this package cannot import
// internal/posse, because internal/posse's init.go already imports this
// package, and Go refuses that cycle for any file sharing this package's
// test binary (ranger-base-c2er3). See internal/posse/execwrite.go for the
// full explanation of why syscall.ForkLock is the right lock, what it
// costs, and its one caveat (do not point it at a path that can block, such
// as a FIFO).
//
// It writes a file that will be exec'd without leaving the golang/go#22315
// window open: os.WriteFile in every respect, except the write happens
// under syscall.ForkLock so no sibling fork in this process can inherit the
// write descriptor and answer a later exec with ETXTBSY.
func WriteExecutable(path string, content []byte, perm fs.FileMode) error {
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()
	return os.WriteFile(path, content, perm)
}
