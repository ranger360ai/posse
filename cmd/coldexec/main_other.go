//go:build !linux

package main

import (
	"fmt"
	"os"
)

// On every other platform the door refuses, and says why rather than
// falling through to an uninjected run: seccomp user notification is
// Linux's, and darwin — the one other platform this tree is built for —
// charges the real cold-exec cost on its own (ADR 0085's Context). A door
// that exited 0 here would be a green suite that measured nothing, which is
// the single outcome ADR 0085 exists to prevent.
func main() {
	fmt.Fprintln(os.Stderr,
		"coldexec: this door is Linux's — it needs seccomp user notification "+
			"(SECCOMP_FILTER_FLAG_NEW_LISTENER, Linux 5.5+). darwin pays the real "+
			"cold first exec, 156-714ms mean and 2.17s worst MEASURED on 25.6.0, so "+
			"there is nothing to inject there: run the suite plain (ADR 0085 D2).")
	os.Exit(2)
}
