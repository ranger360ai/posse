package posse

// Helpers lifted out of pulse_test.go so every suite arm compiles them
// (ranger-base-qp1hm, ranger-base-pv5vt). A file with a build tag is absent
// from the arms it does not name, and `containsStr` has a reader outside the
// arm it lived in: pulse_test.go, backupfuture_test.go and
// backupfresh_qa_test.go are all `//go:build posse_arm3`, but
// verifybox_qa_test.go is untagged and so compiled into all three — which is
// what left arms 1 and 2 with `undefined: containsStr`.
//
// Do not census a helper's readers with `grep -l containsStr`: it also matches
// containsString, a different function declared in runtimepreflight.go. Ask
// for the call form and exclude the longer name, or ask the compiler.

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
