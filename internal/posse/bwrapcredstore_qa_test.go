//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-a8qfg — step 5c, the credential STORE masked as a
// directory, and the selection that decides which stores it covers.
//
// WHY THE SHAPE CHANGED. A /dev/null bind is a mount over a NAME, so the
// file half of this wall fails open twice, and both ways are silent:
//
//  1. A credential file that is not there when the seat launches gets no
//     mount at all — bwrap can only bind over a name that exists. A first
//     `codex login` on the host during a live seat's life then lands at a
//     path that seat can read, and because no mount was ever made, there is
//     nothing for `posse wall-watch` to see detach (ranger-base-y4cm6's
//     watch is a watch over mounts). On this box every sibling store's
//     credential file is absent today, so this was the live shape and not a
//     hypothetical one.
//  2. A refresh that writes a NEW INODE and renames it over the name
//     detaches the mount in every namespace (MEASURED under
//     ranger-base-y4cm6). The seat then reads the host's file, and under
//     that bead's watch the seat is killed at every refresh.
//
// A tmpfs over the DIRECTORY answers both: a login writes inside the empty
// tmpfs, and a rename inside a masked directory is not a mount event. What
// it costs is everything else under that directory, which is why the
// allowlist exists and why these arms pin it — posse's own readers run
// in-seat (credentialStoreAllowlist has the three and what each one is for).

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// THE HEART OF THE BEAD: a store that is not there at launch is MADE and
// masked, and the credential literal under it renders no mount of its own —
// so the name is covered before it is a name.
func TestBwrapArgsMasksACredentialStoreThatIsNotThereYet(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj"}, nil)
	store, cred := bwAt("codex"), bwAt("codex", "auth.json")

	argv, prep, watch, err := bwrapArgsWatched(bwAt("proj"), []string{bwAt("proj")},
		SeatbeltCarveOut{DenyRead: []string{cred}, ReadMaskDir: []string{store}})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	if !strings.Contains(line, "--tmpfs "+store) {
		t.Fatalf("no tmpfs over the store %s:\n%s", store, line)
	}
	if strings.Contains(line, cred) {
		t.Fatalf("a mount was rendered over %s — the store's tmpfs covers that name, and a /dev/null there would only make it:\n%s", cred, line)
	}
	// The launcher makes the mount point, because a tmpfs needs a directory
	// to cover and the whole point is that this one is absent.
	if got := bwPrep(prep); len(got) != 1 || got[0] != "mkdir -p "+store {
		t.Fatalf("prep = %v, want one mkdir -p of %s", got, store)
	}
	if prep[0].Kind != PrepMkdirAll || prep[0].Why == "" {
		t.Fatalf("step = %+v", prep[0])
	}
	// NO credential file is created: an empty directory is a mount point and
	// holds nobody's bytes, a file at that name would be posse writing into
	// another runtime's store.
	if _, err := os.Stat(cred); !os.IsNotExist(err) {
		t.Fatalf("the render created %s (%v)", cred, err)
	}
	// It is a WALL, so wall-watch holds the seat to it (ranger-base-0qpsw
	// D1): the tmpfs is watched and the covered literal is not a mount.
	if want := []WallWatchMount{{WallWatchReadMask, store}}; !reflect.DeepEqual(watch, want) {
		t.Fatalf("watched = %v, want %v", watch, want)
	}
}

// What posse itself reads under a masked store comes back read-only, and
// nothing else does. The allowlist is three members because three of posse's
// readers of these homes are reachable from a SEAT — the danger-silence
// probes `posse runtime check` blocks on, codex's version tap, and the cost
// adapters' `sessions/` — and an un-allowlisted tmpfs would have each of
// them reading the wall as a fact about the operator's box.
func TestBwrapArgsBindsBackWhatPosseReadsUnderAMaskedCredentialStore(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj", "codex/sessions", "codex/tmp"},
		[]string{"codex/config.toml", "codex/version.json", "codex/auth.json", "codex/history.jsonl"})
	store := bwAt("codex")

	argv, _, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj")},
		SeatbeltCarveOut{DenyRead: []string{bwAt("codex", "auth.json")}, ReadMaskDir: []string{store}})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	for _, m := range []string{"config.toml", "version.json", "sessions"} {
		p := bwAt("codex", m)
		bind := "--ro-bind " + p + " " + p
		if !strings.Contains(line, bind) {
			t.Errorf("%s is not bound back read-only under the mask:\n%s", m, line)
		}
		// AFTER the tmpfs, or the tmpfs buries it and the reader this
		// allowlist exists for reads an empty store anyway.
		if strings.Index(line, bind) < strings.Index(line, "--tmpfs "+store) {
			t.Errorf("%s is bound back BEFORE the tmpfs that covers it:\n%s", m, line)
		}
	}
	// The credential, and everything else the store holds, stays behind the
	// mask: the allowlist is what posse reads, not what is there.
	for _, m := range []string{"auth.json", "history.jsonl", "tmp"} {
		if p := bwAt("codex", m); strings.Contains(line, p) {
			t.Errorf("%s came back from behind the mask:\n%s", p, line)
		}
	}
}

// A member of the wrong SHAPE is skipped rather than bound: a directory at
// `config.toml` is not the file the probes parse, and binding it would
// reopen a tree nobody asked to reopen (5a's rule, ranger-base-12ggv).
func TestBwrapArgsSkipsACredentialStoreAllowOfTheWrongShape(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj", "codex/config.toml"}, []string{"codex/sessions"})
	store := bwAt("codex")

	argv, _, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj")}, SeatbeltCarveOut{ReadMaskDir: []string{store}})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	if !strings.Contains(line, "--tmpfs "+store) {
		t.Fatalf("no tmpfs over the store %s:\n%s", store, line)
	}
	for _, m := range []string{"config.toml", "sessions"} {
		if p := bwAt("codex", m); strings.Contains(line, "--ro-bind "+p) {
			t.Errorf("%s was bound back with the wrong shape:\n%s", p, line)
		}
	}
}

// A literal the carve-out read-denies by NAME is not bound back, whatever
// the allowlist says: deny wins (ADR 0001), the rule sshDirAllowed follows
// one shape over.
func TestACredentialStoreAllowDoesNotOutvoteAReadDeny(t *testing.T) {
	t.Parallel()
	store := filepath.Join(t.TempDir(), "codex")
	cfg := filepath.Join(store, "config.toml")
	carve := SeatbeltCarveOut{DenyRead: []string{cfg}}
	for _, m := range carve.readMaskDirAllowed(store) {
		if m.path == cfg {
			t.Fatalf("%s is allowed back under the mask although the carve-out read-denies it", cfg)
		}
	}
	if got := len(carve.readMaskDirAllowed(store)); got != len(credentialStoreAllowlist)-1 {
		t.Fatalf("%d members allowed, want %d — the deny should drop exactly one", got, len(credentialStoreAllowlist)-1)
	}
}

// A FILE where a store should be refuses the render: a tmpfs covers only a
// directory, and a mask that silently renders nothing is the failure this
// tier refuses (5a's refusal, same direction).
func TestBwrapArgsRefusesACredentialStoreThatIsAFile(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj"}, []string{"codex"})
	store := bwAt("codex")

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj")}, SeatbeltCarveOut{ReadMaskDir: []string{store}})
	if err == nil {
		t.Fatalf("a file at a credential store path rendered a wall: %v", argv)
	}
	if !strings.Contains(err.Error(), store) {
		t.Fatalf("the refusal does not name the path: %v", err)
	}
	if argv != nil || prep != nil {
		t.Fatalf("a refused render returned argv %v and prep %v", argv, bwPrep(prep))
	}
}

// A WRITE GRANT THAT REACHES THE STORE renders no mask, in both directions,
// and the credential FILE keeps its own. An empty tmpfs over a grant is a
// seat writing where nothing survives the seat — B8's lost-write failure
// with the grant in place, which is worse than the wall it would put up —
// and a read wall inside a write grant is theatre anyway: the breach is the
// grant. So the render degrades to the file mask it replaced rather than
// refusing a launch, and `posse gates` prints the store with the `·` marker
// that means "in a list and renders no mount".
func TestBwrapArgsSkipsAMaskOverAWriteGrantedCredentialStore(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, grant, store string }{
		{"the grant is the store", "codex", "codex"},
		{"the grant is above the store", "home", "home/codex"},
		{"the grant is inside the store", "codex/sessions", "codex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, bwAt := bwTree(t, []string{"proj", tc.grant, tc.store}, []string{filepath.Join(tc.store, "auth.json")})
			grant, store := bwAt(tc.grant), bwAt(tc.store)
			cred := filepath.Join(store, "auth.json")

			argv, _, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), grant},
				SeatbeltCarveOut{DenyRead: []string{cred}, ReadMaskDir: []string{store}})
			if err != nil {
				t.Fatal(err)
			}
			line := bwJoin(argv)
			if strings.Contains(line, "--tmpfs "+store) {
				t.Errorf("an empty tmpfs was mounted over the write-granted store %s:\n%s", store, line)
			}
			if !strings.Contains(line, "--ro-bind /dev/null "+cred) {
				t.Errorf("the credential file lost its own mask too:\n%s", line)
			}
		})
	}
}

// The SELECTION: every sibling store this seat's runtime does not own, in
// both spellings, and the owning seat spared. Pure, so every arm runs on
// either OS — the same reason credentialReadDenyLiterals' table pin gives.
func TestCredentialStoreReadMaskDirsSparesTheOwningRuntime(t *testing.T) {
	const home = "/Users/probe"
	t.Setenv("HOME", home)
	for _, v := range []string{"CODEX_HOME", "GROK_HOME", "CLAUDE_CONFIG_DIR", "CLAUDE_SECURESTORAGE_CONFIG_DIR"} {
		unsetenvForTest(t, v)
	}
	codex, grok, bob := filepath.Join(home, ".codex"), filepath.Join(home, ".grok"), filepath.Join(home, ".bob")

	for _, tc := range []struct {
		name      string
		stateDirs []string
		want      []string
	}{
		// A claude seat owns none of the three, so all three are masked —
		// and claude's own config dir is NOT in the list, whoever the seat
		// is: that directory is a tree posse reads and binds member by
		// member, so its credential keeps the file mask (ReadMaskDir's own
		// comment has the reasoning and the residual).
		{"a claude seat", []string{"~/.claude"}, []string{codex, grok, bob}},
		{"a codex seat", []string{"~/.codex"}, []string{grok, bob}},
		{"a grok seat", []string{"~/.grok"}, []string{codex, bob}},
		{"a bob seat", []string{"~/.bob"}, []string{codex, grok}},
		// Containment and not spelling, the rule ranger-base-ppnds landed
		// for the file half: an absolutely-spelled or slash-trailing
		// state_dir is still that runtime's store.
		{"a codex seat spelled absolutely", []string{codex}, []string{grok, bob}},
		{"a codex seat with a trailing slash", []string{"~/.codex/"}, []string{grok, bob}},
		// No declaration at all is no ownership: the stores are masked, and
		// a seat that needs one says so with its state_dir.
		{"no state dirs", nil, []string{codex, grok, bob}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := credentialStoreReadMaskDirs(tc.stateDirs); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("credentialStoreReadMaskDirs(%v) = %v, want %v", tc.stateDirs, got, tc.want)
			}
		})
	}
}

// The CLI's own home variable moves the store, so the mask follows it — and
// keeps the home spelling too, for the reason the file half keeps it
// (ranger-base-x5cbz): whatever the CLI wrote before the variable moved the
// write is still sitting in the home.
func TestCredentialStoreReadMaskDirsFollowsTheCLIHomeVars(t *testing.T) {
	const home = "/Users/probe"
	t.Setenv("HOME", home)
	unsetenvForTest(t, "GROK_HOME")
	moved := "/elsewhere/mycodex"
	t.Setenv("CODEX_HOME", moved)

	got := credentialStoreReadMaskDirs([]string{"~/.claude"})
	for _, want := range []string{moved, filepath.Join(home, ".codex")} {
		if !containsString(got, want) {
			t.Errorf("%v does not mask %s", got, want)
		}
	}
	// A codex seat declaring the built-in spelling is spared BOTH, which is
	// where ranger-base-b52r3's residual left the file half.
	if got := credentialStoreReadMaskDirs([]string{"~/.codex"}); containsString(got, moved) {
		t.Errorf("%v masks a codex seat off its own moved store %s", got, moved)
	}
	// A RELATIVE value is not skipped here, and that is the file wall's
	// behaviour and not a second one: runtimeHomeOverride resolves it
	// against this process's cwd, because the absolute answer is what posse
	// TYPES on the seat's line (RuntimeHomePin), so the mask covers the very
	// directory that seat's codex would use. What this list never contains
	// is a path that is still relative — `grokHomeIn("")` with no home at
	// all — which would hang a wall off whatever cwd the launcher happened
	// to have (ranger-base-58b5).
	t.Setenv("CODEX_HOME", "mycodex")
	abs, err := filepath.Abs("mycodex")
	if err != nil {
		t.Fatal(err)
	}
	got = credentialStoreReadMaskDirs([]string{"~/.claude"})
	if !containsString(got, abs) {
		t.Errorf("%v does not mask the resolved CODEX_HOME %s", got, abs)
	}
	for _, d := range got {
		if !filepath.IsAbs(d) {
			t.Errorf("a relative path reached the mask list as %s", d)
		}
	}
	// And when that resolution lands inside a write grant — which a relative
	// value against the launcher's cwd is the one configuration that reaches
	// by accident — the render drops the mask rather than emptying a granted
	// directory (TestBwrapArgsSkipsAMaskOverAWriteGrantedCredentialStore).
	_, bwAt := bwTree(t, []string{"proj"}, nil)
	argv, _, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj")},
		SeatbeltCarveOut{ReadMaskDir: []string{bwAt("proj", "mycodex")}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bwJoin(argv), "--tmpfs "+bwAt("proj", "mycodex")) {
		t.Error("a store resolved inside the cwd grant was masked with a tmpfs")
	}
	// With no home at all, nothing is masked rather than something relative.
	t.Setenv("HOME", "")
	unsetenvForTest(t, "CODEX_HOME")
	if got := credentialStoreReadMaskDirs(nil); len(got) != 0 {
		t.Errorf("with no home the mask list is %v, want empty", got)
	}
}

// One table, two walls (ranger-base-a8qfg): the FILE the credential deny
// names and the DIRECTORY the mask covers come from credentialSiblingStores,
// so a fourth sibling runtime cannot be masked without being denied or the
// other way round. A second spelling of `~/.codex` beside that table is the
// acquisition-site class ranger-base-x5cbz paid for one runtime over.
func TestTheCredentialStoreMaskAndDenyReadTheSameTable(t *testing.T) {
	const home = "/Users/probe"
	t.Setenv("HOME", home)
	for _, v := range []string{"CODEX_HOME", "GROK_HOME", "CLAUDE_CONFIG_DIR", "CLAUDE_SECURESTORAGE_CONFIG_DIR"} {
		unsetenvForTest(t, v)
	}
	dirs := credentialStoreReadMaskDirs([]string{"~/.claude"})
	for _, p := range credentialReadDenyLiterals("linux", []string{"~/.claude"}) {
		if underDir(filepath.Join(home, ".claude"), p) {
			continue // claude's own, which the mask deliberately leaves to the file wall
		}
		if underDir(filepath.Join(home, ".ssh"), p) {
			continue // the ssh keys, 5a's shape and not a credential STORE
		}
		var covered bool
		for _, d := range dirs {
			covered = covered || underDir(d, p)
		}
		if !covered {
			t.Errorf("the credential literal %s is under no masked store: %v", p, dirs)
		}
	}
}
