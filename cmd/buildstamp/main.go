// Command buildstamp prints internal/posse.SourceBuildStamp(".") for the
// current directory — nothing else. The Makefile's `build` target shells out
// to it rather than recomposing the dirty half in make/shell, which is what
// left GIT_DIRTY a bare "-dirty" bit unable to tell two dirty trees apart
// (ranger-base-b6fh) while SourceBuildStamp moved on to a content
// fingerprint. One implementation, run from both `posse cage build` and
// `make build`, cannot drift the way two hand-kept ones did — see
// ranger-base-qyws.
package main

import (
	"fmt"

	"github.com/ranger360ai/posse/internal/posse"
)

func main() {
	fmt.Print(posse.SourceBuildStamp("."))
}
