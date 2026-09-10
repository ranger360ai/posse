package treepins

import (
	"io/fs"
	"os"
	"syscall"
)

// WriteExecutable retains the test package's existing fork-safe writer.
// See internal/posse/execwrite.go for the lock's rationale and FIFO caveat.
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
