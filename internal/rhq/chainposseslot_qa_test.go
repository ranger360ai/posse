package rhq

// QA, ranger-base-hd56 — the other half of ranger-base-q32o's shape: the
// prescription's THIRD step, `mv <slot> posse-<slot>`.
//
// `mv` onto an existing file destroys it without a word. Steps one and two of
// the printed block were made safe by q32o (the destination name is chosen
// against the directory); the third step's destination is not free to rename,
// because installHook's recognizer reads exactly `posse-<slot>`. So the only
// honest handling when that file is there and is NOT ours is to refuse and
// print no paste block at all — the way a dispatcher over a foreign
// posse-<slot> is already refused (chainrepair_qa_test.go).
//
// The same byte is written without any paste at all by --chain over bd's
// shim, which writes posse-<slot> unconditionally; that path is pinned here
// too, because there the destruction needs no operator to reach it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const hd56Foreign = "#!/bin/sh\necho \"some other tool ran\" >&2\nexit 0\n"
const hd56ForeignPosse = "#!/bin/sh\necho \"not posse's gate\" >&2\nexit 0\n"

// hd56Repo is a git repo whose pre-push slot holds slotBody and whose
// posse-pre-push holds a hook that is not ours.
func hd56Repo(t *testing.T, slotBody string) (repo, hooks string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks = filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte(slotBody), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "posse-pre-push"), []byte(hd56ForeignPosse), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo, hooks
}

// hd56Intact fails unless the file still holds exactly what was put there.
func hd56Intact(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s is gone: %v", filepath.Base(path), err)
	}
	if string(got) != want {
		t.Errorf("%s was overwritten:\n got %q\nwant %q", filepath.Base(path), got, want)
	}
}

// The printed prescription. A foreign hook in the slot, a foreign
// posse-<slot> beside it: the block posse used to print ends
// `mv pre-push posse-pre-push`, which destroys that file. Refuse instead,
// name it, and print nothing to paste.
func TestQAForeignPosseSlotUnderAForeignHookRefusesWithoutAPasteBlock(t *testing.T) {
	repo, hooks := hd56Repo(t, hd56Foreign)

	_, err := InstallPrePushHook(repo)
	if err == nil {
		t.Fatal("install-hooks must refuse: nothing here is ours to overwrite")
	}
	text := err.Error()
	if !strings.Contains(text, "posse-pre-push") {
		t.Errorf("the refusal must name the file it will not overwrite: %q", text)
	}
	if strings.Contains(text, "Chain it —") {
		t.Errorf("a prescription whose last step destroys posse-pre-push must not be printed: %q", text)
	}
	// Whatever else it says, no line of it may move anything onto that file.
	if strings.Contains(text, "mv pre-push posse-pre-push") {
		t.Errorf("the refusal still prescribes an mv onto posse-pre-push: %q", text)
	}
	hd56Intact(t, filepath.Join(hooks, "posse-pre-push"), hd56ForeignPosse)
	hd56Intact(t, filepath.Join(hooks, "pre-push"), hd56Foreign)
	if PrePushHookInstalled(repo) {
		t.Error("a refused install must not read as installed")
	}
}

// --chain over bd's shim reaches the same write with no paste at all: it
// moves the shim to bd-<slot> and writes posse-<slot> unconditionally. The
// operator sees an install that succeeded and a file that is gone.
func TestQAChainedInstallOverBdShimDoesNotOverwriteAForeignPosseSlot(t *testing.T) {
	repo, hooks := hd56Repo(t, "#!/bin/sh\n# bd-shim v1\nexec bd hooks run pre-push \"$@\"\n")

	if _, err := InstallPrePushHookChained(repo); err == nil {
		t.Error("--chain must refuse rather than bury a foreign posse-pre-push")
	} else if !strings.Contains(err.Error(), "posse-pre-push") {
		t.Errorf("the refusal must name the file: %q", err)
	}
	hd56Intact(t, filepath.Join(hooks, "posse-pre-push"), hd56ForeignPosse)
}

// The boundary, stated: when posse-<slot> is OURS the third step rewrites it
// with the bytes install-hooks just wrote, nothing is lost, and the
// prescription is still the way through. Refusing here would strand every
// operator whose repo posse itself chained.
func TestQAOurOwnPosseSlotStillGetsThePrescription(t *testing.T) {
	repo, hooks := hd56Repo(t, hd56Foreign)
	if err := os.WriteFile(filepath.Join(hooks, "posse-pre-push"), []byte(PrePushHook), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := InstallPrePushHook(repo)
	if err == nil {
		t.Fatal("a foreign hook in the slot is still refused")
	}
	if !strings.Contains(err.Error(), "Chain it —") {
		t.Errorf("the chain prescription must still be printed here: %q", err)
	}
}
