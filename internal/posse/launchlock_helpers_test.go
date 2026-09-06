package posse

// Helpers lifted out of launchlock_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuf is an io.Writer a test goroutine may read while the code under
// test is still writing to it.
type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitForOut blocks until the writer has said want. The budget is generous
// on purpose: every herdr call here forks the test binary, and under -race
// one launch is tens of seconds.
func waitForOut(t *testing.T, buf *syncBuf, want string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), want) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("output never contained %q:\n%s", want, buf.String())
}

// mustHoldLock takes the launcher lock for the test itself — the other
// launcher every test here needs.
func mustHoldLock(t *testing.T, a *App) *LaunchLock {
	t.Helper()
	l, err := lockLaunches(a, &syncBuf{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(l.Release)
	return l
}

// rawCalls is the fake herdr's log without calls()'s gate-prefix assertion:
// these tests read it while a pass is mid-flight, when a partial line is
// normal and is not evidence of a missing wall.
func rawCalls(t *testing.T, fake string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(fake, "calls.log"))
	return string(b)
}

// createdPersonas is the persona of each session the fake herdr was asked to
// create, in creation order.
func createdPersonas(t *testing.T, fake string) []string {
	t.Helper()
	var order []string
	for _, ln := range strings.Split(rawCalls(t, fake), "\n") {
		if !strings.HasPrefix(ln, "workspace create ") {
			continue
		}
		fields := strings.Fields(ln)
		for i, f := range fields {
			if f != "--label" || i+1 >= len(fields) {
				continue
			}
			persona, _, _ := strings.Cut(fields[i+1], "-")
			order = append(order, persona)
		}
	}
	return order
}
