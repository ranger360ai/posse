package posse

// Helpers lifted out of pulse_test.go so every suite arm compiles them
// (ranger-base-qp1hm, ranger-base-pv5vt). A file with a build tag is absent
// from the arms it does not name, and these declarations have readers in all
// of them: `containsStr` is called from arm 1 (codexwritable_test.go,
// hooksredirect_qa_test.go, hooksredirectprobe_qa_test.go,
// runtimewalk_live_test.go), from the shared verifybox_qa_test.go, and from
// arm 3 — while pulse_test.go, where it used to live, is arm 3 alone.

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
