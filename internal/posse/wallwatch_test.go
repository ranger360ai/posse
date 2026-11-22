//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-ihnnl (design ranger-base-0qpsw D1+D2): what the
// render hands `posse wall-watch`, the line that starts it, the mount-table
// diff, and D2's one-bead-per-path-per-ten-minutes record. The live half,
// a real bwrap seat losing a wall, is wallwatchlive_test.go.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ─── the render's walls, held from the ARGV and not from a list ─────────────
//
// WHY THERE IS A DERIVATION HERE AT ALL (ranger-base-mxbq0 finding 2,
// discovered from the verify at ranger-base-6rj8w). The pin below used to be
// a reflect.DeepEqual against a hand-written literal over a fixture that
// carried only the kinds already recorded, so it could not see a wall kind
// the FIXTURE omitted: MEASURED 2026-11-13, a render whose watch list came
// back EMPTY passed the pin named "records every wall" unchanged, because
// the fixture asked for none of the kinds that render had. That is how
// ranger-base-w6ip0 came to be found by hand instead of by this file, and a
// literal of six leaves the sixth-to-seventh step the same silent one.
//
// So the expectation is DERIVED, the shape this tree already uses for a
// class that must stay total (make corpus-check, pkgcensus, treewidedoor):
// every destination BwrapArgs mounts over is either in the watch list or in
// an opening set computed from the render's own INPUTS, and every watched
// path is a destination the argv really carries. A new mount with no watch()
// call reds; a watch() call with no mount reds; and an option the table
// below does not classify reds rather than being skipped, because a mount
// flag nobody classified is exactly the hole this replaces.
//
// THE FIXTURE IS HELD TOTAL FROM THE OTHER END TOO
// (TestWallWatchFixturesCarryEveryCarveOutField): a derivation over an argv
// says nothing about a shape the inputs never asked for, so every field of
// SeatbeltCarveOut must have a value in one of the renders here. A field
// added to that struct tomorrow reds until somebody gives it one and says
// which side of the rule its mount falls on.

// bwrapMountOpt is one option BwrapArgs can emit: how many arguments it
// takes, and whether the last of them is a mount DESTINATION.
type bwrapMountOpt struct {
	args  int
	mount bool
}

// bwrapRenderOpts is CLOSED on purpose. The pin's claim is about the render
// and not about this table, so an option the render emits and this table
// does not name is an error and never a skip — the skip is what would let a
// seventh mount kind arrive unwatched and green.
var bwrapRenderOpts = map[string]bwrapMountOpt{
	"--die-with-parent": {0, false},
	"--unshare-user":    {0, false},
	"--chdir":           {1, false},
	"--unsetenv":        {1, false},
	"--tmpfs":           {1, true},
	"--bind":            {2, true},
	"--ro-bind":         {2, true},
	"--dev-bind":        {2, true},
}

// bwrapMountDests answers every path argv mounts over, in render order.
func bwrapMountDests(t *testing.T, argv []string) []string {
	t.Helper()
	var dests []string
	for i := 0; i < len(argv); {
		o, ok := bwrapRenderOpts[argv[i]]
		if !ok {
			t.Fatalf("argv[%d] = %q is an option bwrapRenderOpts does not classify, so this file cannot say whether its destination is a wall or an opening. Add it there — and if it mounts, decide in bwrapArgsInto whether it takes a watch() call: an unclassified mount is how ranger-base-w6ip0 escaped.\nargv: %q", i, argv[i], argv)
		}
		if i+o.args >= len(argv) {
			t.Fatalf("argv[%d] = %q wants %d arguments and the argv ends: %q", i, argv[i], o.args, argv)
		}
		if o.mount {
			dests = append(dests, argv[i+o.args])
		}
		i += o.args + 1
	}
	return dests
}

// wwRender is one render this file holds to the wall/opening rule.
type wwRender struct {
	what     string
	cwd      string
	writable []string
	carve    SeatbeltCarveOut
	refKeep  []string
}

// wwOpenings answers which of a render's destinations RE-OPEN something, as
// dst -> why, derived from the render's inputs and never from the argv.
//
// Derived, because that is what makes the pin's other half total: a new
// input list that renders a wall is caught by having no watch() call, and a
// new input list that renders an OPENING is caught here — the author has to
// come to this function and say so in as many words, which is the step the
// hand-written literal did not have.
func wwOpenings(c wwRender) map[string]string {
	o := map[string]string{
		// Step 1's base. Not a mount over a name a host can rename out from
		// under a seat: it is the floor the rest of the wall stands on, and
		// the one destination whose detaching has no meaning.
		"/": "the read-only base of step 1",
		// seatbelt's (regex #"^/dev/") grant, which every runtime's tty needs.
		"/dev": "step 1's dev bind",
	}
	for _, w := range c.writable {
		o[w] = "a write grant: a detached one leaves the seat with LESS than it was given"
		o[absResolve(w)] = "a write grant: a detached one leaves the seat with LESS than it was given"
	}
	for _, f := range c.carve.Keep {
		o[f] = "a Keep file, bound back over the deny above it"
	}
	for _, d := range c.carve.SSHDir {
		for _, f := range c.carve.sshDirAllowed(d) {
			o[f] = "an ssh allow bind: a hole deliberately left in the " + d + " tmpfs, which is the wall"
		}
	}
	// The same shape at step 5c's wall (ranger-base-a8qfg): the credential
	// store's tmpfs is the wall, and credentialStoreAllowlist's members are
	// bound back through holes in it. Derived from the allowlist and not
	// listed here, so a member added to it arrives classified.
	for _, d := range c.carve.ReadMaskDir {
		for _, m := range c.carve.readMaskDirAllowed(d) {
			o[m.path] = "a credential-store allow bind: a hole deliberately left in the " + d + " tmpfs, which is the wall"
		}
	}
	return o
}

// wwEveryShape is the canonical render: every field of SeatbeltCarveOut that
// bwrapArgsInto reads a mount out of, plus the two it deliberately reads
// nothing out of (Seal, whose rename a mount point cannot be carried away
// by, and Narrow and DenyAll, which bwrapRender clears), plus a write grant and a D20
// ref keep. Both tests below drive it.
//
// AND EVERY FIELD OF PrivateBind, which is a second axis the fields of
// SeatbeltCarveOut do not reach (ranger-base-99mvt, from the verify at
// ranger-base-7h7cs). `Private` is a slice of a struct whose own fields pick
// which ARM of the private loop renders, so one dir-shaped entry satisfies a
// reflection over SeatbeltCarveOut while leaving whole arms unrendered: that
// is how ranger-base-a544g's FILE bind (bwrap.go's pb.File arm) arrived
// watched by no pin — MEASURED 2026-11-13, deleting its one
// `watch(WallWatchPrivate, pb.Dst)` reds nothing. So the FOUR entries below
// are the four arms: a dir COPY (Src != Dst), a FILE, a dir bound IN PLACE
// read-only (claudeProjectsBinds' shape, ranger-base-ajkqx), and an empty
// TMPFS (Src == "", ranger-base-6ob20).
//
// AND A FIELD REFLECTION CANNOT HOLD THAT COUNT (ranger-base-4tsa7, from the
// verify at ranger-base-zkuwa). The tmpfs arm is selected by a field's ZERO
// value, so the reflection in TestWallWatchFixturesCarryEveryCarveOutField —
// which demands each PrivateBind field be NON-ZERO in some render — could
// never ask for it, and it shipped rendered by no fixture at all: MEASURED
// 2026-11-14, deleting ITS watch call left `-run TestWallWatch` green, where
// deleting any of the other three red two tests. The fourth entry below is
// that instance. The COUNT is held by
// TestWallWatchFixturesRenderEveryPrivateArm, which reads the arms off
// bwrapPrivateArgs' own returns, and the watch is now the private loop's one
// exit point rather than a line each arm can be written without.
func wwEveryShape(t *testing.T) wwRender {
	t.Helper()
	root := t.TempDir()
	grant := filepath.Join(root, "cfg")
	priv := filepath.Join(grant, "shell-snapshots")
	seatRoot := filepath.Join(root, "seat")
	src := filepath.Join(seatRoot, "shell-snapshots")
	hist := filepath.Join(grant, "history.jsonl")
	histSrc := filepath.Join(seatRoot, "history.jsonl")
	inPlace := filepath.Join(grant, "projects")
	// The tmpfs arm's destination is deliberately NOT created: a
	// runtimeEphemeralDirs member need not exist on the host, and the arm
	// makes its mount point in prep, which is the half an existing dir would
	// hide.
	eph := filepath.Join(grant, "ephemeral")
	deny := filepath.Join(grant, "settings.json")
	mask := filepath.Join(grant, "policy-cache.json")
	gates := filepath.Join(grant, "gates")
	hide := filepath.Join(root, "run")
	sshDir := filepath.Join(root, ".ssh")
	tsnet := filepath.Join(root, "state", "rex", "tsnet")
	// ranger-base-bncbk: the DenyReadDir entry this launcher MAY make, ABSENT
	// on purpose — so step 5b's mkdir-and-anchor arm renders in this fixture
	// instead of only its mask-what-is-there arm. `tsnet` beside it stays out
	// of DenyReadDirMake and is present, which is how the OTHER OS's default
	// behaves: masked when it resolves, never created.
	tsnetMake := filepath.Join(root, "state-native", "rex", "tsnet")
	cred := filepath.Join(root, "sibling", "credentials.json")
	// Absent on purpose: a Seeded file-shaped deny inside a grant (the
	// launcher writes it empty and the mount goes over it), and the ref keep
	// that renders no mount at all.
	seeded := filepath.Join(grant, "settings-seeded.json")
	// ranger-base-v84cn: an ANCHORED deny dir, absent, so the launcher makes
	// it and seeds the anchor inside it before the bind. The mount is the
	// plain deny's, and is watched as one.
	anchored := filepath.Join(grant, "swept")
	// ranger-base-a8qfg: another runtime's credential STORE, masked as a
	// DIRECTORY. The tmpfs is the wall; the credential literal under it
	// renders no mount of its own, so it must not be watched either, and the
	// allowlist members below are bound back through holes in that same wall
	// — all three present, so the allow arm really renders here.
	store := filepath.Join(root, "store")
	storeCred := filepath.Join(store, "auth.json")
	for _, d := range []string{grant, priv, src, inPlace, hide, sshDir, tsnet, gates, store, filepath.Join(store, "sessions"), filepath.Dir(cred)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		deny, mask, cred, hist, histSrc,
		filepath.Join(sshDir, "config"), filepath.Join(sshDir, "known_hosts"), filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(gates, "refusals.log"),
		storeCred, filepath.Join(store, "config.toml"), filepath.Join(store, "version.json"),
	} {
		if err := os.WriteFile(f, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return wwRender{
		what:     "every shape",
		writable: []string{grant},
		refKeep:  []string{filepath.Join(grant, "refs-keep")},
		carve: SeatbeltCarveOut{
			Hide:     []string{hide},
			Deny:     []string{deny, mask, gates, seeded, anchored},
			DenyFile: []string{deny, mask, seeded},
			Mask:     []string{mask},
			Seed:     []string{seeded},
			Anchor:   []string{anchored},
			Seal:     []string{grant},
			Keep:     []string{filepath.Join(gates, "refusals.log")},
			Narrow:   []SeatbeltNarrow{{Path: priv, Allow: false}},
			DenyAll:  []string{priv},
			Private: []PrivateBind{
				// SrcRoot is the seat cache dir both sources are composed
				// from: the render walks every component below it and
				// refuses a link at any (ranger-base-wsgus).
				{Src: src, SrcRoot: seatRoot, Dst: priv},
				// ranger-base-a544g's shape: a regular file at both ends.
				{Src: histSrc, SrcRoot: seatRoot, Dst: hist, File: true},
				// ranger-base-ajkqx's: bound in place to narrow the grant.
				{Src: inPlace, Dst: inPlace, ReadOnly: true},
				// ranger-base-6ob20's: an empty tmpfs, selected by an empty
				// Src — codex's tmp/ and shell_snapshots/ and grok's
				// daemon/run/ (runtimeEphemeralDirs) are this shape.
				{Dst: eph},
			},
			SSHDir:          []string{sshDir},
			DenyReadDir:     []string{tsnet, tsnetMake},
			DenyReadDirMake: []string{tsnetMake},
			ReadMaskDir:     []string{store},
			DenyRead:        []string{cred, filepath.Join(sshDir, "id_ed25519"), storeCred},
			// Seatbelt-only, like Narrow and DenyAll above, and unlike
			// either it renders no mount AT ALL on this mechanism — it is
			// about connect(2) on a socket and not about a path's contents
			// (ranger-base-q9bqu). It is here because the field census
			// above is total, and it must leave the watch list below
			// untouched: a field bwrapRender clears is a wall this
			// mechanism never put up.
			DenyConnect: []string{filepath.Join(grant, "leader.sock")},
		},
	}
}

// wwPrivateTmp is its own render because carve.Tmp mounts over /tmp
// (bwrapTmpMount) and every path a t.TempDir fixture has is UNDER /tmp — so
// in one fixture with the shapes above, `reopens` would skip the denies and
// the ssh tmpfs that no write grant re-opens, and the render would be
// measuring almost nothing. Here the one shape is the point.
func wwPrivateTmp(t *testing.T) wwRender {
	t.Helper()
	root := t.TempDir()
	tmp := filepath.Join(root, "seat-tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	return wwRender{what: "the seat's private /tmp", carve: SeatbeltCarveOut{Tmp: tmp}}
}

func wwRenders(t *testing.T) []wwRender {
	t.Helper()
	return []wwRender{wwEveryShape(t), wwPrivateTmp(t)}
}

// Every mount the render emits is either a WALL it watches or an opening
// wwOpenings names, and every wall it watches is a mount the argv carries.
// The pin ranger-base-ihnnl shipped read a hand-written list instead, and a
// wall kind the fixture omitted was invisible to it (head comment above).
func TestWallWatchEveryRenderedMountIsAWallOrANamedOpening(t *testing.T) {
	t.Parallel()
	for _, c := range wwRenders(t) {
		argv, prep, watch, err := bwrapArgsWatched(c.cwd, c.writable, c.carve, c.refKeep...)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		open := wwOpenings(c)
		watched := map[string]bool{}
		for _, w := range watch {
			watched[w.Path] = true
		}
		dests := bwrapMountDests(t, argv)
		seen := map[string]bool{}
		for _, d := range dests {
			seen[d] = true
			switch {
			case watched[d] && open[d] != "":
				t.Errorf("%s: %s is watched AND named an opening (%s) — a destination is one or the other, and the openings are not watched because a detached one leaves the seat with less and never more", c.what, d, open[d])
			case watched[d], open[d] != "":
			default:
				t.Errorf("%s: the render mounts over %s and neither watches it nor names it an opening. If it is a WALL, give it a watch() call in bwrapArgsInto; if it re-opens something, say so in wwOpenings. An unclassified mount is a seat whose wall a host rename detaches with nothing watching (ranger-base-w6ip0).\nargv: %q", c.what, d, argv)
			}
		}
		for _, w := range watch {
			if !seen[w.Path] {
				t.Errorf("%s: %s is watched as a %s and the argv mounts nothing over it — `posse wall-watch` would then read the HOST's mount at that name as the seat's wall, and a bead filed off it names a wall that was never put up.\nargv: %q", c.what, w.Path, w.Kind, argv)
			}
		}
		// BwrapArgs's own output is unchanged by the third one.
		a1, p1, err := BwrapArgs(c.cwd, c.writable, c.carve, c.refKeep...)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if !reflect.DeepEqual(a1, argv) || !reflect.DeepEqual(p1, prep) {
			t.Errorf("%s: BwrapArgs and bwrapArgsWatched rendered different walls", c.what)
		}
	}
}

// The derivation above is only as total as the inputs it is driven with, so
// the inputs are held total too: every field of SeatbeltCarveOut has a value
// in one of the renders here. A field added to that struct reds this until
// somebody gives it one — at which point the test above says whether its
// mount is a wall or an opening, which is the question the hand-written
// literal let an author skip.
//
// PrivateBind's fields are held the same way, one level down, because a
// field on a carve field's ELEMENT type is a new mount shape that leaves
// the carve field itself non-zero (ranger-base-99mvt).
func TestWallWatchFixturesCarryEveryCarveOutField(t *testing.T) {
	t.Parallel()
	cs := wwRenders(t)
	rt := reflect.TypeOf(SeatbeltCarveOut{})
	for i := 0; i < rt.NumField(); i++ {
		set := false
		for _, c := range cs {
			if !reflect.ValueOf(c.carve).Field(i).IsZero() {
				set = true
			}
		}
		if !set {
			t.Errorf("SeatbeltCarveOut.%s has no value in any render here, so TestWallWatchEveryRenderedMountIsAWallOrANamedOpening says nothing about the mount it makes. Give it one in wwEveryShape (or its own render, as carve.Tmp has), and classify what it mounts: a watch() call in bwrapArgsInto, or an entry in wwOpenings", rt.Field(i).Name)
		}
	}
	// And every field of PrivateBind, the one carve field whose element type
	// picks which ARM of a render loop runs (ranger-base-99mvt): a field
	// added there is a mount SHAPE, and the loop above cannot see it because
	// `Private` is already non-zero. ranger-base-a544g's File arm shipped
	// watched by nothing for exactly that reason.
	pt := reflect.TypeOf(PrivateBind{})
	for i := 0; i < pt.NumField(); i++ {
		set := false
		for _, c := range cs {
			for _, pb := range c.carve.Private {
				if !reflect.ValueOf(pb).Field(i).IsZero() {
					set = true
				}
			}
		}
		if !set {
			t.Errorf("PrivateBind.%s has no value in any render here, so the arm of bwrapArgsInto's private loop it selects renders in no fixture and TestWallWatchEveryRenderedMountIsAWallOrANamedOpening says nothing about the mount that arm makes. Give it a PrivateBind entry in wwEveryShape and classify what it mounts: a watch() call in bwrapArgsInto, or an entry in wwOpenings", pt.Field(i).Name)
		}
	}
	// The two inputs that are parameters rather than fields.
	one := cs[0]
	if len(one.writable) == 0 || len(one.refKeep) == 0 {
		t.Errorf("%s must carry a write grant and a D20 ref keep: the grant is the opening the rule is stated against, and the keep is the one input that renders NO mount", one.what)
	}
}

// wwPrivateArms is the private loop's arms, written as the entry that
// SELECTS each one, in bwrapPrivateArgs' own branch order. It is a closed
// mirror, the way bwrapRenderOpts is a closed mirror of the options the
// render emits, and the test below is what keeps it from drifting.
var wwPrivateArms = []struct {
	name string
	is   func(PrivateBind) bool
}{
	{"a FILE bind at both ends (ranger-base-a544g)", func(pb PrivateBind) bool { return pb.File }},
	{"a dir bound IN PLACE (ranger-base-ajkqx)", func(pb PrivateBind) bool { return !pb.File && pb.Src == pb.Dst }},
	{"an empty TMPFS (ranger-base-6ob20)", func(pb PrivateBind) bool { return !pb.File && pb.Src != pb.Dst && pb.Src == "" }},
	{"a dir COPY out of the seat's own cache (ranger-base-5632o)", func(pb PrivateBind) bool { return !pb.File && pb.Src != pb.Dst && pb.Src != "" }},
}

// Every arm of bwrapPrivateArgs renders in one of the fixtures here, and the
// ARM COUNT is read off that function's own returns rather than off the
// table above (ranger-base-4tsa7, from the verify at ranger-base-zkuwa).
//
// WHY NOT THE FIELD REFLECTION ABOVE. It demands each PrivateBind field be
// non-zero in some render, so an arm selected by a field's ZERO value is one
// it can never ask for — and that is the tmpfs arm, `Src == ""`. It shipped
// rendered by no fixture, and the close that widened the reflection to
// PrivateBind's fields claimed in as many words that a field added there
// "cannot again be rendered by no fixture", which holds only for arms a
// non-zero value selects. An arm is not a field, so it is counted as an arm.
func TestWallWatchFixturesRenderEveryPrivateArm(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "bwrap.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "bwrapPrivateArgs" {
			fn = fd
		}
	}
	if fn == nil {
		t.Fatal("bwrap.go has no bwrapPrivateArgs. If the private loop's arms went back inline, they took one watch(WallWatchPrivate, pb.Dst) call per arm with them — the line ranger-base-6ob20's tmpfs arm was written without, and which no pin could see (ranger-base-4tsa7)")
	}
	// One `[]string{…}` per arm: an arm that renders a mount is exactly a
	// place where this function spells an argv, and the argv IS the mount
	// shape.
	//
	// It counted `return <argv>, <prep>, true, nil` until ranger-base-ua625
	// gave the dir arms a single tail — they share one anchor step now, for
	// the reason they already shared one watch call — and three arms then
	// read as one return. The return was always a proxy for the argv; this
	// counts the argv. Both spellings red on a fifth arm, because a fifth
	// arm has to say what it mounts.
	arms := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if at, ok := cl.Type.(*ast.ArrayType); ok && at.Len == nil {
			if id, ok := at.Elt.(*ast.Ident); ok && id.Name == "string" {
				arms++
			}
		}
		return true
	})
	if arms != len(wwPrivateArms) {
		t.Fatalf("bwrapPrivateArgs renders %d arms and wwPrivateArms names %d. An arm there is a new mount SHAPE: name it above and give wwEveryShape a PrivateBind entry that selects it, or TestWallWatchEveryRenderedMountIsAWallOrANamedOpening never renders that arm and says nothing about the mount it makes", arms, len(wwPrivateArms))
	}
	cs := wwRenders(t)
	for _, arm := range wwPrivateArms {
		rendered := false
		for _, c := range cs {
			for _, pb := range c.carve.Private {
				if arm.is(pb) {
					rendered = true
				}
			}
		}
		if !rendered {
			t.Errorf("no PrivateBind entry in any render here is %s, so that arm of bwrapPrivateArgs renders in no fixture. Give wwEveryShape one", arm.name)
		}
	}
	// And the mirror is a PARTITION over the entries: one an arm claims
	// twice, or none claims, is a table that has drifted off the branch
	// order it mirrors, and the count above would still read right.
	for _, c := range cs {
		for _, pb := range c.carve.Private {
			hit := 0
			for _, arm := range wwPrivateArms {
				if arm.is(pb) {
					hit++
				}
			}
			if hit != 1 {
				t.Errorf("%s: the PrivateBind %+v is selected by %d of wwPrivateArms' arms, want exactly 1 — the table no longer mirrors bwrapPrivateArgs' branch order", c.what, pb, hit)
			}
		}
	}
}

// The render records its WALLS and only its walls: Hide, Private, Masked,
// Deny and step 5's read masks (ranger-base-y4cm6) are watched; a write
// grant and the ssh allow bind-backs (openings) are not. This is the KINDS
// and the ORDER, which the derivation above does not read — the derivation
// holds the set, this holds what each member is called and when it is
// mounted. Both are needed: a wall recorded under the wrong kind is a bead
// that names the wrong wall.
func TestWallWatchRenderRecordsEveryWallAndNoOpening(t *testing.T) {
	t.Parallel()
	c := wwEveryShape(t)
	grant := c.writable[0]
	root := filepath.Dir(grant)
	sshDir := c.carve.SSHDir[0]
	store := c.carve.ReadMaskDir[0]
	_, _, watch, err := bwrapArgsWatched(c.cwd, c.writable, c.carve, c.refKeep...)
	if err != nil {
		t.Fatal(err)
	}
	want := []WallWatchMount{
		{WallWatchHide, filepath.Join(root, "run")},
		// The private binds in list order, one per arm of that loop: the dir
		// copy, the FILE (ranger-base-a544g), the in-place read-only dir and
		// the empty tmpfs (ranger-base-6ob20, ranger-base-4tsa7).
		{WallWatchPrivate, filepath.Join(grant, "shell-snapshots")},
		{WallWatchPrivate, filepath.Join(grant, "history.jsonl")},
		{WallWatchPrivate, filepath.Join(grant, "projects")},
		{WallWatchPrivate, filepath.Join(grant, "ephemeral")},
		// shallowestFirst sorts the deny list, so gates, policy-cache.json,
		// settings-seeded.json, settings.json — the mask under its own kind.
		{WallWatchDeny, filepath.Join(grant, "gates")},
		{WallWatchMasked, filepath.Join(grant, "policy-cache.json")},
		{WallWatchDeny, filepath.Join(grant, "settings-seeded.json")},
		{WallWatchDeny, filepath.Join(grant, "settings.json")},
		// ranger-base-v84cn: the anchor changes the prep, not the wall.
		{WallWatchDeny, filepath.Join(grant, "swept")},
		{WallWatchReadMask, sshDir},
		// 5b's two, shallowestFirst over DenyReadDir: the one this launcher
		// MAY make (absent, so the prep makes and anchors it —
		// ranger-base-bncbk) sorts before the one it may not.
		{WallWatchReadMask, filepath.Join(root, "state-native", "rex", "tsnet")},
		{WallWatchReadMask, filepath.Join(root, "state", "rex", "tsnet")},
		// 5c's directory mask renders between 5b's read-deny dir and the
		// file masks, and the credential literal under it renders none of its own.
		{WallWatchReadMask, store},
		{WallWatchReadMask, filepath.Join(root, "sibling", "credentials.json")},
	}
	if !reflect.DeepEqual(watch, want) {
		t.Errorf("watched walls =\n %v\nwant\n %v\n(the write grant %s is an opening and must not be watched)", watch, want, grant)
	}
	// The bind-backs under the ssh tmpfs are the holes deliberately left in
	// that wall, and the key under it renders no mount at all, so none of
	// the three is watched.
	for _, p := range []string{"config", "known_hosts", "id_ed25519"} {
		if wallWatchListed(watch, filepath.Join(sshDir, p)) {
			t.Errorf("%s is watched: it is an opening under the %s tmpfs, not a wall of its own", p, sshDir)
		}
	}
	// Same for the credential literal under a masked store: the directory is
	// the wall, and a watch on a name that has no mount would fire the first
	// time the host wrote there (ranger-base-a8qfg).
	if storeCred := filepath.Join(store, "auth.json"); wallWatchListed(watch, storeCred) {
		t.Errorf("%s is watched: the %s tmpfs is the wall and this name has no mount of its own", storeCred, store)
	}
	// And the D20 ref keep, which renders no mount: watching it would be the
	// other direction of the same silence.
	if wallWatchListed(watch, c.refKeep[0]) {
		t.Errorf("%s is watched and the render mounts nothing over it", c.refKeep[0])
	}
}

func TestWallWatchMountsFileRoundTripsAPathWithANewline(t *testing.T) {
	t.Parallel()
	in := []WallWatchMount{{WallWatchDeny, "/nonexistent/a\nb"}, {WallWatchHide, "/nonexistent/c d"}}
	got, err := parseWallWatchMounts(wallWatchMountsBytes(in))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Errorf("round trip = %v, want %v", got, in)
	}
	for _, bad := range []string{"deny\x00/x", "deny\x00", "deny\x00rel\x00"} {
		if _, err := parseWallWatchMounts([]byte(bad)); err == nil {
			t.Errorf("parseWallWatchMounts(%q) accepted a file posse did not write", bad)
		}
	}
}

func TestWallWatchMountsFileNamesTheResolvedPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(filepath.Join(real, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	got, _ := parseWallWatchMounts(wallWatchMountsBytes([]WallWatchMount{{WallWatchDeny, filepath.Join(link, "d")}}))
	want, _ := filepath.EvalSymlinks(filepath.Join(real, "d"))
	if len(got) != 1 || got[0].Path != want {
		t.Errorf("mounts file names %v, want the path the kernel spells: %s", got, want)
	}
}

func TestWallWatchPrefixStartsTheWatcherInFrontOfBwrap(t *testing.T) {
	t.Parallel()
	got := WallWatchPrefix("/opt/posse", "/h/gates/dev/bwrap.mounts", "dev", "/w/s", "/usr/bin/bwrap", "/h/gates/dev/bwrap.args")
	want := "3<'/h/gates/dev/bwrap.args' '/opt/posse' wall-watch --mounts '/h/gates/dev/bwrap.mounts' --seat 'dev' --dir '/w/s' -- '/usr/bin/bwrap' --args 3 -- "
	if got != want {
		t.Errorf("WallWatchPrefix =\n %q\nwant\n %q", got, want)
	}
	o, err := parseWallWatchArgv([]string{"posse", WallWatchVerb, "--mounts", "/m", "--seat", "dev", "--dir", "/w", "--", "/usr/bin/bwrap", "--args", "3", "--", "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Mounts != "/m" || o.Seat != "dev" || o.Dir != "/w" || !reflect.DeepEqual(o.Bwrap, []string{"/usr/bin/bwrap", "--args", "3", "--", "claude"}) {
		t.Errorf("parsed %+v", o)
	}
	if got := wallWatchBwrapArgv(o.Bwrap); !reflect.DeepEqual(got, []string{"/usr/bin/bwrap", "--info-fd", "4", "--block-fd", "5", "--args", "3", "--", "claude"}) {
		t.Errorf("bwrap argv = %q: the info and block fds go before the rendered options", got)
	}
	for _, bad := range [][]string{
		{"posse", WallWatchVerb},
		{"posse", WallWatchVerb, "--mounts", "/m"},
		{"posse", WallWatchVerb, "--", "/usr/bin/bwrap"},
		{"posse", WallWatchVerb, "--bogus", "x", "--mounts", "/m", "--", "b"},
	} {
		if _, err := parseWallWatchArgv(bad); err == nil {
			t.Errorf("parseWallWatchArgv(%q) accepted it", bad)
		}
	}
}

// A bwrap render writes the mounts file beside the args, and its Launch line
// is the watcher in front of the same binary and args; Prefix (what a probe
// types) is unchanged.
func TestWallWatchBwrapRenderWritesTheMountsFileAndTheLaunchLine(t *testing.T) {
	// No t.Parallel: bwlStubBin and wallWatchBin are package-level vars.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	old := wallWatchBin
	wallWatchBin = func() (string, error) { return "/opt/posse", nil }
	t.Cleanup(func() { wallWatchBin = old })
	w, err := f.a.renderWallOn(WallBwrap, f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	mounts := filepath.Join(filepath.Dir(w.Artifact), bwrapMountsName)
	// The socket-wall step rides on the launch line too (ranger-base-6rndn),
	// so the expectation is built from the one spelling of it rather than
	// written out here a second time.
	if want := WallWatchPrefix("/opt/posse", mounts, f.ag.Name, f.dir, "/usr/bin/bwrap", w.Artifact) + wallUnixWrapForTest(t); w.Launch != want {
		t.Errorf("WallRender.Launch =\n %q\nwant\n %q", w.Launch, want)
	}
	if w.Prefix != BwrapPrefix("/usr/bin/bwrap", w.Artifact) {
		t.Errorf("WallRender.Prefix changed: %q — probes type it, and they run no watcher", w.Prefix)
	}
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.a.GatesDir(f.ag.Name))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(mounts)
	if err != nil {
		t.Fatalf("the render wrote no mounts file: %v", err)
	}
	if !reflect.DeepEqual(b, wallWatchMountsBytes(r.Watch)) {
		t.Errorf("%s is not the render's own wall list", mounts)
	}
	if len(r.Watch) == 0 {
		t.Error("an ordinary claude seat renders no wall to watch — its Deny binds alone should be there")
	}
}

func TestWallWatchMountinfoDiff(t *testing.T) {
	t.Parallel()
	mi := "36 35 98:0 / / ro,relatime master:1 - ext4 /dev/root rw\n" +
		"40 36 0:33 / /h/cfg rw - ext4 /dev/root rw\n" +
		"41 40 0:33 / /h/cfg/settings.json ro - ext4 /dev/root rw\n" +
		"42 40 0:5 / /h/with\\040space ro - tmpfs tmpfs rw\n" +
		"43 40 0:5 / /h/back\\134slash ro - tmpfs tmpfs rw\n"
	live := parseMountinfo([]byte(mi))
	for _, p := range []string{"/", "/h/cfg", "/h/cfg/settings.json", "/h/with space", `/h/back\slash`} {
		if len(live[p]) == 0 {
			t.Errorf("mountinfo parse lost %q (got %v)", p, live)
		}
	}
	watch := []WallWatchMount{
		{WallWatchDeny, "/h/cfg/settings.json"},
		{WallWatchPrivate, "/h/cfg/shell-snapshots"},
		{WallWatchHide, "/h/with space"},
		{WallWatchMasked, "/h/cfg/shell-snapshots"},
	}
	base, missing := wallWatchBaseline(watch, live)
	if !reflect.DeepEqual(missing, []WallWatchMount{{WallWatchPrivate, "/h/cfg/shell-snapshots"}}) {
		t.Errorf("missing at launch = %v, want the one path that is not in the table, once", missing)
	}
	if got := wallWatchDetached(watch, base, live); len(got) != 0 {
		t.Errorf("an unchanged table reads as detached: %v", got)
	}
	// The deny's mount leaves.
	gone := parseMountinfo([]byte(strings.Replace(mi, "41 40 0:33 / /h/cfg/settings.json ro - ext4 /dev/root rw\n", "", 1)))
	if got := wallWatchDetached(watch, base, gone); !reflect.DeepEqual(got, []WallWatchMount{{WallWatchDeny, "/h/cfg/settings.json"}}) {
		t.Errorf("detached = %v, want the deny", got)
	}
}

// A wall stacked over another mount at the same point is watched by its own
// id: when it leaves, the lower mount still at that point does not hide it.
func TestWallWatchStackedWallIsWatchedByItsId(t *testing.T) {
	t.Parallel()
	both := "20 1 0:1 / /tmp rw - tmpfs tmpfs rw\n50 20 0:9 / /tmp rw - ext4 /dev/root rw\n"
	watch := []WallWatchMount{{WallWatchHide, "/tmp"}}
	base, missing := wallWatchBaseline(watch, parseMountinfo([]byte(both)))
	if len(missing) != 0 {
		t.Fatal(missing)
	}
	lower := parseMountinfo([]byte("20 1 0:1 / /tmp rw - tmpfs tmpfs rw\n"))
	if got := wallWatchDetached(watch, base, lower); len(got) != 1 {
		t.Errorf("the top mount at /tmp left and the diff saw nothing: a point-only compare reads the lower mount as the wall")
	}
}

// fakeWallWatchStore is an in-memory bead store for D2's dedupe.
type fakeWallWatchStore struct {
	issues    []BdIssue
	comments  map[string][]string
	readErr   error
	createErr error
	// commentErr fails every Comment (ranger-base-kzk3u).
	commentErr error
	n          int
	now        func() time.Time
}

func (s *fakeWallWatchStore) OpenLabeledAny(dir string, labels ...string) ([]BdIssue, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	return append([]BdIssue(nil), s.issues...), nil
}

func (s *fakeWallWatchStore) Create(dir string, n BdNew) (string, error) {
	if s.createErr != nil {
		return "", s.createErr
	}
	s.n++
	id := "t-" + string(rune('a'+s.n-1))
	created := time.Now()
	if s.now != nil {
		created = s.now()
	}
	pri := 0
	if n.Priority == "1" {
		pri = 1
	}
	s.issues = append(s.issues, BdIssue{ID: id, Title: n.Title, Description: n.Description, Labels: n.Labels, Priority: pri, IssueType: n.Type, Created: created, Status: "open"})
	return id, nil
}

func (s *fakeWallWatchStore) Comment(dir, id, text, actor string) error {
	if s.commentErr != nil {
		return s.commentErr
	}
	if s.comments == nil {
		s.comments = map[string][]string{}
	}
	s.comments[id] = append(s.comments[id], text)
	return nil
}

func TestWallWatchFilesOneP1PerPathAndCommentsInsideTenMinutes(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	clock := t0
	st := &fakeWallWatchStore{now: func() time.Time { return clock }}
	settings := WallWatchMount{WallWatchDeny, "/h/.claude/settings.json"}
	snaps := WallWatchMount{WallWatchPrivate, "/h/.claude/shell-snapshots"}
	det := func(seat string, ms ...WallWatchMount) wallWatchDetach {
		return wallWatchDetach{Seat: seat, Dir: "/w/" + seat, BwrapPid: 10, SeatPid: 11, At: clock, Missing: ms, Killed: []int{10, 11}}
	}

	filed, commented, err := fileWallWatchDetach(st, det("a", settings), clock, "")
	if err != nil || filed == "" || len(commented) != 0 {
		t.Fatalf("first detection: filed=%q commented=%v err=%v", filed, commented, err)
	}
	is := st.issues[0]
	if is.Priority != 1 || is.IssueType != "bug" || !reflect.DeepEqual(is.Labels, []string{"security"}) {
		t.Errorf("bead is P%d %s %v, want a P1 security bug", is.Priority, is.IssueType, is.Labels)
	}
	for _, want := range []string{"seat: a", settings.Path, "detached at: 2026-11-13T10:00:00Z", "killed: pids 10 11", wallWatchMarker(settings.Path)} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("description lacks %q:\n%s", want, is.Description)
		}
	}

	// Another seat, same path, five minutes on: a comment, no bead.
	clock = t0.Add(5 * time.Minute)
	filed, commented, err = fileWallWatchDetach(st, det("b", settings), clock, "")
	if err != nil || filed != "" || !reflect.DeepEqual(commented, []string{is.ID}) || len(st.issues) != 1 {
		t.Fatalf("same path inside ten minutes: filed=%q commented=%v err=%v beads=%d", filed, commented, err, len(st.issues))
	}
	if c := st.comments[is.ID]; len(c) != 1 || !strings.Contains(c[0], "seat b") {
		t.Errorf("comment = %q, want one naming seat b", c)
	}

	// Both paths at once: a comment for the filed one, a bead for the new one.
	clock = t0.Add(6 * time.Minute)
	filed, commented, err = fileWallWatchDetach(st, det("c", settings, snaps), clock, "")
	if err != nil || filed == "" || len(commented) != 1 || len(st.issues) != 2 {
		t.Fatalf("mixed: filed=%q commented=%v err=%v beads=%d", filed, commented, err, len(st.issues))
	}
	if d := st.issues[1].Description; strings.Contains(d, wallWatchMarker(settings.Path)) || !strings.Contains(d, wallWatchMarker(snaps.Path)) {
		t.Errorf("the new bead must carry only the new path's marker:\n%s", d)
	}

	// Past the window: a new bead for the old path.
	clock = t0.Add(11 * time.Minute)
	filed, _, err = fileWallWatchDetach(st, det("d", settings), clock, "")
	if err != nil || filed == "" || len(st.issues) != 3 {
		t.Fatalf("past ten minutes: filed=%q err=%v beads=%d", filed, err, len(st.issues))
	}
}

// A store that cannot be read still gets the bead, and the error says the
// dedupe was skipped.
func TestWallWatchUnreadableStoreStillFiles(t *testing.T) {
	t.Parallel()
	st := &fakeWallWatchStore{readErr: errors.New("locked")}
	filed, _, err := fileWallWatchDetach(st, wallWatchDetach{Seat: "a", Missing: []WallWatchMount{{WallWatchDeny, "/x"}}}, time.Now(), filepath.Join(t.TempDir(), "l", "wall-watch.lock"))
	if filed == "" || len(st.issues) != 1 {
		t.Fatal("an unreadable store filed nothing — a lost wall with no record")
	}
	if err == nil || !strings.Contains(err.Error(), "not deduped") {
		t.Errorf("err = %v, want it to say the dedupe was skipped", err)
	}
}

// ─── the dispatch tick's backstop (ranger-base-zd4d4, 0qpsw D3) ─────────────

// fakeWallWatchProbe is a process table the test writes.
type fakeWallWatchProbe struct {
	alive  bool
	pids   []int
	table  mountTable
	killed int
	// leaderPpid and leaderRead are what LeaderParent answers
	// (ranger-base-5k7vf). The ZERO VALUE is "the record's leader is not
	// there to be read" — reaped, recycled, or in another namespace —
	// which is what every arm written before that bead means by a seat
	// whose watcher is gone: on this box bwrap's die-with-parent takes the
	// leader with the watcher and only its backgrounded child survives.
	leaderPpid int
	leaderRead bool
	// killErr and killNothing are the two kill outcomes no arm could reach
	// while Kill returned every pid and an empty error unconditionally
	// (ranger-base-6r6ox): a kill that did not confirm — the census still
	// names processes, so they are left in pids, which is also what
	// ranger-base-kzk3u's arms turn on — and a kill that found the seat
	// already over.
	killErr     string
	killNothing bool
}

func (p *fakeWallWatchProbe) WatcherAlive(wallWatchRecord) bool { return p.alive }
func (p *fakeWallWatchProbe) SeatPids(wallWatchRecord) []int    { return p.pids }
func (p *fakeWallWatchProbe) Table([]int) (mountTable, error)   { return p.table, nil }
func (p *fakeWallWatchProbe) LeaderParent(wallWatchRecord) (int, bool) {
	return p.leaderPpid, p.leaderRead
}
func (p *fakeWallWatchProbe) Kill(wallWatchRecord) ([]int, string) {
	p.killed++
	if p.killNothing {
		p.pids = nil
		return nil, ""
	}
	k := p.pids
	if p.killErr != "" {
		return k, p.killErr
	}
	p.pids = nil
	return k, ""
}

func wwBackstopRecord(t *testing.T) (state, path string, r wallWatchRecord) {
	t.Helper()
	state = t.TempDir()
	mounts := filepath.Join(state, "gates", "developer", "bwrap.mounts")
	r = wallWatchRecord{
		Seat: "developer", Dir: "/w/developer", WatcherPid: 100, WatcherStart: "5", BwrapPid: 101, SeatPid: 102, MntNS: 4026533000,
		Watch: []WallWatchMount{{WallWatchDeny, "/h/.claude/settings.json"}, {WallWatchPrivate, "/h/.claude/shell-snapshots"}},
		Base:  map[string][]string{"/h/.claude/settings.json": {"40"}, "/h/.claude/shell-snapshots": {"41"}},
	}
	path = filepath.Join(wallWatchRecordDir(mounts), "100.json")
	if err := writeWallWatchRecord(path, r); err != nil {
		t.Fatal(err)
	}
	return state, path, r
}

func wwIntact() mountTable {
	return mountTable{"/h/.claude/settings.json": {"40"}, "/h/.claude/shell-snapshots": {"41"}}
}

func TestWallWatchBackstopKillsAndFilesADetachWhoseWatcherIsGone(t *testing.T) {
	t.Parallel()
	state, path, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 {
		t.Fatalf("killed %d times, want 1: %q", p.killed, lines)
	}
	if len(st.issues) != 1 {
		t.Fatalf("%d beads, want 1: %+v", len(st.issues), st.issues)
	}
	d := st.issues[0].Description
	for _, want := range []string{wallWatchMarker("/h/.claude/settings.json"), "seat: developer", "dispatch watch tick", "was gone", "pids 102 103"} {
		if !strings.Contains(d, want) {
			t.Errorf("the bead does not say %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, wallWatchMarker("/h/.claude/shell-snapshots")) {
		t.Errorf("the bead names a wall that stayed up:\n%s", d)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the record of a killed seat whose watcher is gone was kept (%v)", err)
	}
}

func TestWallWatchBackstopActsOnADetachEvenWithTheWatcherAlive(t *testing.T) {
	t.Parallel()
	state, path, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{alive: true, pids: []int{102}, table: mountTable{}}
	wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 || len(st.issues) != 1 || !strings.Contains(st.issues[0].Description, "was alive and had not acted") {
		t.Fatalf("killed=%d issues=%+v", p.killed, st.issues)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the record belongs to its live watcher, which removes it; the tick removed it (%v)", err)
	}
}

// wwBackstopBeadClass asserts the CLASS of a bead the backstop files for
// itself: a P1 security bug. That triple is what
// TestWallWatchFilesOneP1PerPathAndCommentsInsideTenMinutes has asserted of
// fileWallWatchDetach's bead since ranger-base-0qpsw, and neither of the
// backstop's OWN two filings — the dead-watcher bead and
// ranger-base-5k7vf's kill — was held to it.
//
// IT IS NOT DECORATION, and `security` is the half that is load-bearing
// twice: it is the lane the bead routes to, and it is also the label the
// detach dedupe READS BACK — fileWallWatchDetach's OpenLabeledAny(dir,
// "security") is how a later detach finds the bead an earlier one filed, so a
// backstop bead filed without it is invisible to that read as well.
//
// MEASURED 2026-11-22 under ranger-base-evq89, the QA verify of
// ranger-base-5k7vf: with the kill's `Labels: []string{"security"}` mutated
// to nil, and again with its `Type: "bug"` mutated to "task", every test in
// `-run 'TestWallWatchSeatOrphaned|TestWallWatchBackstop|TestWallWatchProcProbeLeaderParent|TestWallWatchLeaderLive'`
// stayed green — 15 tests, ok 0.19s. Priority was the one third of the triple
// a pin held (`Priority: "2"` red the kill arm).
func wwBackstopBeadClass(t *testing.T, is BdIssue, what string) {
	t.Helper()
	if is.Priority != 1 || is.IssueType != "bug" || !reflect.DeepEqual(is.Labels, []string{"security"}) {
		t.Errorf("%s is P%d %s %v, want a P1 security bug — and `security` is the label fileWallWatchDetach's own dedupe reads back, so a bead without it is unrouted AND undeduped", what, is.Priority, is.IssueType, is.Labels)
	}
}

func TestWallWatchBackstopFilesADeadWatcherOnceAndKeepsDiffing(t *testing.T) {
	t.Parallel()
	state, path, r := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{103}, table: wwIntact()}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 0 {
		t.Errorf("a seat with every wall up was killed")
	}
	if len(st.issues) != 1 || !strings.Contains(st.issues[0].Description, wallWatchOrphanMarker(r)) {
		t.Fatalf("want one bead naming the dead watcher, got %+v (lines %q)", st.issues, lines)
	}
	wwBackstopBeadClass(t, st.issues[0], "the dead watcher's bead")
	got, err := readWallWatchRecord(path)
	if err != nil || got.OrphanFiled != st.issues[0].ID {
		t.Fatalf("the record does not carry the filed bead: %+v, %v", got, err)
	}
	// The next tick files nothing more...
	wallWatchBackstop(state, st, "", time.Now, p)
	if len(st.issues) != 1 {
		t.Errorf("the second tick filed again: %d beads", len(st.issues))
	}
	// ...and still diffs: the wall comes off, the seat is killed.
	p.table = mountTable{"/h/.claude/shell-snapshots": {"41"}}
	wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 || len(st.issues) != 2 {
		t.Errorf("after the detach: killed=%d beads=%d, want 1 and 2", p.killed, len(st.issues))
	}
}

// ── the dead watcher's kill (ranger-base-5k7vf) ────────────────────────────

// The decision, which is the whole gate on a kill the watcher never made:
// the leader has to be READABLE as the record's leader, and its parent has
// to be something other than the record's bwrap.
func TestWallWatchSeatOrphanedNeedsAParentThatIsNotTheRecordsBwrap(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		bwrapPid int
		ppid     int
		read     bool
		want     bool
	}{
		{"reparented to init: the seat nothing bounds", 101, 1, true, true},
		{"reparented to some other subreaper", 101, 4242, true, true},
		{"its bwrap is still its parent, so bwrap still ends it", 101, 101, true, false},
		{"the leader cannot be read as itself — reaped or recycled", 101, 1, false, false},
		{"a record naming no bwrap buys no kill", 0, 1, true, false},
		{"a record naming a negative bwrap buys no kill", -1, 1, true, false},
	} {
		p := &fakeWallWatchProbe{leaderPpid: c.ppid, leaderRead: c.read}
		r := wallWatchRecord{Seat: "developer", BwrapPid: c.bwrapPid, SeatPid: 102, MntNS: 7}
		if _, got := wallWatchSeatOrphaned(r, p); got != c.want {
			t.Errorf("%s: wallWatchSeatOrphaned = %v, want %v — a true here is a kill and a false here is a seat left running (ranger-base-5k7vf)", c.name, got, c.want)
		}
	}
}

func TestWallWatchBackstopKillsASeatOrphanedUnderADeadWatcher(t *testing.T) {
	t.Parallel()
	state, path, r := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	// Every wall up, the watcher gone, and the leader reparented to init:
	// the shape an AppArmor host with no wall-unix step leaves behind.
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: wwIntact(), leaderPpid: 1, leaderRead: true}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 {
		t.Fatalf("killed %d times, want 1 — a seat with no watcher, no bwrap and no pdeathsig was left running: %q", p.killed, lines)
	}
	if len(st.issues) != 1 {
		t.Fatalf("want one bead, got %+v (lines %q)", st.issues, lines)
	}
	wwBackstopBeadClass(t, st.issues[0], "the kill's own bead")
	if ti := st.issues[0].Title; !strings.Contains(ti, "outlived its wall-watch and its bwrap") {
		t.Errorf("the title does not say what happened: %q", ti)
	}
	d := st.issues[0].Description
	for _, want := range []string{wallWatchOrphanKillMarker(r), "seat: developer", "KILLED", "killed: pids 102 103", "it is now pid 1", "watcher pid: 100 (gone)"} {
		if !strings.Contains(d, want) {
			t.Errorf("the bead does not say %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, "nothing has been killed") {
		t.Errorf("the bead of a KILLED seat carries the dead-watcher arm's text:\n%s", d)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the record of a seat the tick killed was kept (%v)", err)
	}
	// The durable copy, for the detach arm's reason: this seat's pane went
	// with its watcher (ranger-base-pfcci).
	vs := readWallWatchVerdicts(state, "developer")
	if len(vs) != 1 {
		t.Fatalf("%d verdicts on disk, want 1 (tick said %q)", len(vs), lines)
	}
	for _, want := range []string{"KILLED seat developer", "ranger-base-5k7vf", "watcher pid 100 (gone)", "filed " + st.issues[0].ID} {
		if !strings.Contains(vs[0].Text, want) {
			t.Errorf("the verdict does not carry %q:\n%s", want, vs[0].Text)
		}
	}
	// The next tick has nothing to say: the seat is over and its record is
	// gone, so the kill is not re-filed.
	if more := wallWatchBackstop(state, st, "", time.Now, p); len(more) != 0 || len(st.issues) != 1 {
		t.Errorf("the tick after the kill said %q and filed %d beads", more, len(st.issues))
	}
}

// The control that keeps the kill narrow: a watcher that is gone with the
// leader still a child of its bwrap is the arm that was there before — one
// bead, no kill — because that bwrap still ends the seat.
func TestWallWatchBackstopLeavesASeatWhoseBwrapIsStillItsLeadersParent(t *testing.T) {
	t.Parallel()
	state, path, r := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: wwIntact(), leaderPpid: r.BwrapPid, leaderRead: true}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 0 {
		t.Fatalf("a seat still under its bwrap was killed: %q", lines)
	}
	if len(st.issues) != 1 || !strings.Contains(st.issues[0].Description, wallWatchOrphanMarker(r)) {
		t.Fatalf("want the dead-watcher bead and no kill, got %+v", st.issues)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the record of a seat that is still running was removed (%v)", err)
	}
}

// A seat found in the bounded shape first and the unbounded one later: the
// kill is not held back by the bead that already exists, and that bead is
// told, because what it says — nothing has been killed — stops being true.
func TestWallWatchBackstopKillsAnAlreadyFiledOrphanAndCommentsOnItsBead(t *testing.T) {
	t.Parallel()
	state, path, r := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: wwIntact(), leaderPpid: r.BwrapPid, leaderRead: true}
	wallWatchBackstop(state, st, "", time.Now, p)
	if len(st.issues) != 1 || p.killed != 0 {
		t.Fatalf("the first tick: beads=%+v killed=%d", st.issues, p.killed)
	}
	id := st.issues[0].ID
	// Now its bwrap goes too.
	p.leaderPpid = 1
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 {
		t.Fatalf("the seat was left running because its dead watcher was already filed: %q", lines)
	}
	if len(st.issues) != 1 {
		t.Errorf("%d beads, want the one that was already filed", len(st.issues))
	}
	cs := st.comments[id]
	if len(cs) != 1 {
		t.Fatalf("%d comments on %s, want 1 saying the seat was killed: %v", len(cs), id, cs)
	}
	for _, want := range []string{"KILLED this seat", wallWatchOrphanKillMarker(r), "killed: pids 102 103"} {
		if !strings.Contains(cs[0], want) {
			t.Errorf("the comment does not say %q:\n%s", want, cs[0])
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the record of a seat the tick killed was kept (%v)", err)
	}
}

// ── a kill that did not confirm is retried, not forgotten (ranger-base-kzk3u)
//
// Both kill arms dropped the record of a seat they had killed once its
// watcher was gone, so a kill that did NOT confirm left a running seat that
// no later tick read again. These hold the record, the retry, the single
// filing and the comment that closes it out.

const wwKzkKillErr = "processes in the seat's mount namespace were still alive after 20 rounds of SIGKILL: [102 103]"

func wwKzkRecord(t *testing.T, path string) wallWatchRecord {
	t.Helper()
	r, err := readWallWatchRecord(path)
	if err != nil {
		t.Fatalf("the record of a seat whose kill did not confirm is gone (%v) — no later tick retries it", err)
	}
	return r
}

func TestWallWatchBackstopKeepsAndRetriesADetachWhoseKillDidNotConfirm(t *testing.T) {
	t.Parallel()
	state, path, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}, killErr: wwKzkKillErr}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 || len(st.issues) != 1 {
		t.Fatalf("killed=%d beads=%+v (lines %q)", p.killed, st.issues, lines)
	}
	id := st.issues[0].ID
	r := wwKzkRecord(t, path)
	if r.KillPending != wwKzkKillErr || !reflect.DeepEqual(r.KillBeads, []string{id}) {
		t.Fatalf("record not marked for a retry: pending=%q beads=%v", r.KillPending, r.KillBeads)
	}
	if cs := st.comments[id]; len(cs) != 1 || !strings.Contains(cs[0], "retries the kill every tick") {
		t.Errorf("the bead does not say the tick is retrying: %q", cs)
	}

	// A retry that fails again: killed again, nothing filed or commented,
	// the record still held.
	lines = wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 2 || len(st.issues) != 1 || len(st.comments[id]) != 1 {
		t.Fatalf("second tick: killed=%d beads=%d comments=%q (lines %q)", p.killed, len(st.issues), st.comments[id], lines)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "still running after a kill that did not confirm") {
		t.Errorf("the failed retry is not said: %q", lines)
	}
	wwKzkRecord(t, path)

	// The retry that confirms: one comment, the record dropped.
	p.killErr = ""
	lines = wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 3 || len(st.issues) != 1 {
		t.Fatalf("third tick: killed=%d beads=%d (lines %q)", p.killed, len(st.issues), lines)
	}
	if cs := st.comments[id]; len(cs) != 2 || !strings.Contains(cs[1], "this one did: killed pids 102 103") {
		t.Errorf("the bead is not told the retry confirmed: %q", cs)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the record of a seat now killed under a dead watcher was kept (%v)", err)
	}
	if vs := readWallWatchVerdicts(state, "developer"); len(vs) != 1 || !strings.Contains(vs[0].Text, "the kill did not confirm, so the seat's record is kept") || !strings.Contains(vs[0].Text, "this one did") {
		t.Errorf("the verdict file does not carry both the failure and the retry: %+v", vs)
	}
}

func TestWallWatchBackstopKeepsAnOrphanWhoseKillDidNotConfirmUntilItEnds(t *testing.T) {
	t.Parallel()
	state, path, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: wwIntact(), leaderPpid: 1, leaderRead: true, killErr: wwKzkKillErr}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 || len(st.issues) != 1 {
		t.Fatalf("killed=%d beads=%+v (lines %q)", p.killed, st.issues, lines)
	}
	id := st.issues[0].ID
	if r := wwKzkRecord(t, path); !reflect.DeepEqual(r.KillBeads, []string{id}) {
		t.Fatalf("record not marked with its bead: %+v", r)
	}
	// The seat ends on its own between ticks: no kill, one comment, gone.
	p.pids = nil
	lines = wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 || len(st.issues) != 1 {
		t.Fatalf("second tick: killed=%d beads=%d (lines %q)", p.killed, len(st.issues), lines)
	}
	if cs := st.comments[id]; len(cs) != 2 || !strings.Contains(cs[1], "seat developer is over") {
		t.Errorf("the bead is not told the seat ended: %q", cs)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the record outlived the seat it named (%v)", err)
	}
}

// The record is dropped only once its beads are told: a store that cannot
// take the closing comment keeps it, and the next tick says it again.
func TestWallWatchBackstopKeepsTheRetryRecordUntilItsBeadIsTold(t *testing.T) {
	t.Parallel()
	state, path, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102}, table: wwIntact(), leaderPpid: 1, leaderRead: true, killErr: wwKzkKillErr}
	wallWatchBackstop(state, st, "", time.Now, p)
	id := st.issues[0].ID
	p.killErr = ""
	st.commentErr = errors.New("bd is down")
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if r := wwKzkRecord(t, path); r.KillPending == "" || !reflect.DeepEqual(r.KillBeads, []string{id}) {
		t.Fatalf("the record dropped its retry mark with the bead untold: %+v (lines %q)", r, lines)
	}
	st.commentErr = nil
	wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 2 {
		t.Errorf("a seat already over was killed again: killed=%d", p.killed)
	}
	if cs := st.comments[id]; len(cs) != 2 || !strings.Contains(cs[1], "is over") {
		t.Errorf("the bead was not told on the next tick: %q", cs)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the record was kept after its bead was told (%v)", err)
	}
}

// A live watcher owns its record: the confirmed retry clears the mark and
// leaves the record for the watcher to remove.
func TestWallWatchBackstopClearsTheRetryMarkUnderALiveWatcher(t *testing.T) {
	t.Parallel()
	state, path, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{alive: true, pids: []int{102}, table: mountTable{}, killErr: wwKzkKillErr}
	wallWatchBackstop(state, st, "", time.Now, p)
	p.killErr = ""
	wallWatchBackstop(state, st, "", time.Now, p)
	r := wwKzkRecord(t, path)
	if r.KillPending != "" || r.KillBeads != nil {
		t.Errorf("the retry mark survived a confirmed kill: %+v", r)
	}
	if p.killed != 2 || len(st.issues) != 1 {
		t.Errorf("killed=%d beads=%d, want 2 and 1", p.killed, len(st.issues))
	}
}

// ── a kill that did not confirm, and one that found nothing (ranger-base-6r6ox)
//
// Until these rows, fakeWallWatchProbe.Kill returned every pid and an empty
// error, so no arm could reach wallWatchKilled's "NOT CONFIRMED" or its
// "nothing was left to kill", and `grep -rn 'NOT CONFIRMED'` over the tree
// found only the string's own definition. Each row drives one render site
// through the backstop or the filing, and holds the TITLE and the verdict
// line's head to the body: a seat the kill could not confirm ended must not
// be reported KILLED in the part a reader sees first.

const wwKillErr = "processes in the seat's mount namespace were still alive after 20 rounds of SIGKILL: [102 103]"

// wwSaysNotConfirmed asserts what every reading of an unconfirmed kill must
// carry, and what it must not.
func wwSaysNotConfirmed(t *testing.T, what, text string) {
	t.Helper()
	if !strings.Contains(text, "NOT CONFIRMED — "+wwKillErr) {
		t.Errorf("%s does not carry the kill's failure:\n%s", what, text)
	}
	if strings.Contains(text, "KILLED seat") || strings.Contains(text, "KILLED this seat") || strings.Contains(text, "killed the seat.") {
		t.Errorf("%s reports as KILLED a seat whose kill did not confirm:\n%s", what, text)
	}
}

func TestWallWatchBackstopDetachWhoseKillDidNotConfirmSaysSoInTitleAndVerdict(t *testing.T) {
	t.Parallel()
	state, _, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}, killErr: wwKillErr}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 || len(st.issues) != 1 {
		t.Fatalf("killed=%d beads=%+v (lines %q)", p.killed, st.issues, lines)
	}
	is := st.issues[0]
	wwBackstopBeadClass(t, is, "the unconfirmed kill's bead")
	if want := "security: wall-watch could NOT confirm the kill of seat developer — the mount over /h/.claude/settings.json came off"; is.Title != want {
		t.Errorf("title = %q, want %q", is.Title, want)
	}
	wwSaysNotConfirmed(t, "the bead", is.Description)
	if !strings.Contains(is.Description, "end it by hand") {
		t.Errorf("the bead does not tell its reader the seat may still be running:\n%s", is.Description)
	}
	vs := readWallWatchVerdicts(state, "developer")
	if len(vs) != 1 {
		t.Fatalf("%d verdicts, want 1 (tick said %q)", len(vs), lines)
	}
	if !strings.Contains(vs[0].Text, "could NOT confirm the kill of seat developer — the wall over /h/.claude/settings.json came off") {
		t.Errorf("the verdict's head does not say the kill is unconfirmed:\n%s", vs[0].Text)
	}
	wwSaysNotConfirmed(t, "the verdict", vs[0].Text)
}

// The comment arm of the same filing: a second detection of the same path
// inside ten minutes is a comment, rendered by wallWatchComment.
func TestWallWatchDetachCommentOfAnUnconfirmedKillSaysNotConfirmed(t *testing.T) {
	t.Parallel()
	st := &fakeWallWatchStore{}
	m := []WallWatchMount{{WallWatchDeny, "/x"}}
	now := time.Now()
	if _, _, err := fileWallWatchDetach(st, wallWatchDetach{Seat: "a", Missing: m, Killed: []int{7}}, now, ""); err != nil {
		t.Fatal(err)
	}
	_, commented, err := fileWallWatchDetach(st, wallWatchDetach{Seat: "b", Missing: m, Killed: []int{8}, KillErr: wwKillErr}, now, "")
	if err != nil || len(commented) != 1 {
		t.Fatalf("commented=%v err=%v, want one comment", commented, err)
	}
	cs := st.comments[commented[0]]
	if len(cs) != 1 {
		t.Fatalf("%d comments, want 1: %v", len(cs), cs)
	}
	wwSaysNotConfirmed(t, "the comment", cs[0])
}

func TestWallWatchBackstopOrphanKillThatDidNotConfirmSaysSoInTitleAndVerdict(t *testing.T) {
	t.Parallel()
	state, _, r := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: wwIntact(), leaderPpid: 1, leaderRead: true, killErr: wwKillErr}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if p.killed != 1 || len(st.issues) != 1 {
		t.Fatalf("killed=%d beads=%+v (lines %q)", p.killed, st.issues, lines)
	}
	is := st.issues[0]
	wwBackstopBeadClass(t, is, "the unconfirmed orphan kill's bead")
	if want := "security: seat developer outlived its wall-watch and its bwrap, and the watch tick could NOT confirm killing it"; is.Title != want {
		t.Errorf("title = %q, want %q", is.Title, want)
	}
	wwSaysNotConfirmed(t, "the bead", is.Description)
	// The marker is the class's search key and stays one spelling, so an
	// unconfirmed kill is found by the same search as a confirmed one.
	if !strings.Contains(is.Description, wallWatchOrphanKillMarker(r)) {
		t.Errorf("the bead lost its marker:\n%s", is.Description)
	}
	vs := readWallWatchVerdicts(state, "developer")
	if len(vs) != 1 {
		t.Fatalf("%d verdicts, want 1 (tick said %q)", len(vs), lines)
	}
	if !strings.Contains(vs[0].Text, "⛨ wall-watch backstop: could NOT confirm the kill of seat developer — its watcher") {
		t.Errorf("the verdict's head does not say the kill is unconfirmed:\n%s", vs[0].Text)
	}
	wwSaysNotConfirmed(t, "the verdict", vs[0].Text)
}

func TestWallWatchBackstopOrphanKillCommentThatDidNotConfirmSaysSo(t *testing.T) {
	t.Parallel()
	state, _, r := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: wwIntact(), leaderPpid: r.BwrapPid, leaderRead: true, killErr: wwKillErr}
	wallWatchBackstop(state, st, "", time.Now, p)
	if len(st.issues) != 1 || p.killed != 0 {
		t.Fatalf("the first tick: beads=%+v killed=%d", st.issues, p.killed)
	}
	id := st.issues[0].ID
	p.leaderPpid = 1
	wallWatchBackstop(state, st, "", time.Now, p)
	cs := st.comments[id]
	// TWO comments, since ranger-base-kzk3u: the kill comment this bead is
	// about, and the notice that the record is kept and the kill retried.
	// The first is the one a reader meets as the kill's own account.
	if p.killed != 1 || len(cs) != 2 {
		t.Fatalf("killed=%d comments=%v", p.killed, cs)
	}
	wwSaysNotConfirmed(t, "the comment", cs[0])
	if !strings.Contains(cs[0], "may still be running") {
		t.Errorf("the comment does not say the seat may still be running:\n%s", cs[0])
	}
	if !strings.Contains(cs[1], "retries the kill every tick") {
		t.Errorf("the bead is not told the kill is being retried (ranger-base-kzk3u):\n%s", cs[1])
	}
}

// The other undriven arm: the kill found the seat already over. That seat
// is ended, so its title still says killed, and its body says why there are
// no pids — not an empty list.
func TestWallWatchBackstopKillThatFoundNothingSaysNothingWasLeft(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		probe *fakeWallWatchProbe
		title string
	}{
		{"detach", &fakeWallWatchProbe{pids: []int{102}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}, killNothing: true},
			"security: wall-watch killed seat developer — the mount over /h/.claude/settings.json came off"},
		{"orphan", &fakeWallWatchProbe{pids: []int{102}, table: wwIntact(), leaderPpid: 1, leaderRead: true, killNothing: true},
			"security: seat developer outlived its wall-watch and its bwrap, and the watch tick killed it"},
	} {
		state, _, _ := wwBackstopRecord(t)
		st := &fakeWallWatchStore{}
		lines := wallWatchBackstop(state, st, "", time.Now, c.probe)
		if c.probe.killed != 1 || len(st.issues) != 1 {
			t.Fatalf("%s: killed=%d beads=%+v (lines %q)", c.name, c.probe.killed, st.issues, lines)
		}
		if st.issues[0].Title != c.title {
			t.Errorf("%s: title = %q, want %q", c.name, st.issues[0].Title, c.title)
		}
		if d := st.issues[0].Description; !strings.Contains(d, "killed: nothing was left to kill") || strings.Contains(d, "NOT CONFIRMED") {
			t.Errorf("%s: the bead does not say nothing was left to kill:\n%s", c.name, d)
		}
	}
}

func TestWallWatchBackstopIsSilentOverAHealthySeatAndDropsAnEndedOne(t *testing.T) {
	t.Parallel()
	state, path, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{alive: true, pids: []int{102}, table: wwIntact()}
	if lines := wallWatchBackstop(state, st, "", time.Now, p); len(lines) != 0 || len(st.issues) != 0 {
		t.Fatalf("a healthy seat under a live watcher said %q and filed %d", lines, len(st.issues))
	}
	// Seat over, watcher alive: the watcher owns the record.
	p.pids = nil
	wallWatchBackstop(state, st, "", time.Now, p)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("removed a live watcher's record (%v)", err)
	}
	// Seat over, watcher gone: nothing to file, the record goes.
	p.alive = false
	if lines := wallWatchBackstop(state, st, "", time.Now, p); len(lines) != 0 || len(st.issues) != 0 {
		t.Errorf("an ended seat said %q and filed %d", lines, len(st.issues))
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an ended seat's orphaned record was kept (%v)", err)
	}
}

func TestWallWatchBackstopSaysWhenARecordCannotBeRead(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	bad := filepath.Join(state, "gates", "x", "wall-watch", "7.json")
	os.MkdirAll(filepath.Dir(bad), 0o755)
	os.WriteFile(bad, []byte("{not json"), 0o644)
	lines := wallWatchBackstop(state, &fakeWallWatchStore{}, "", time.Now, &fakeWallWatchProbe{})
	if len(lines) != 1 || !strings.Contains(lines[0], "NOT checked") {
		t.Errorf("an unreadable record must be said, not skipped: %q", lines)
	}
}

// ─── the verdict file (ranger-base-pfcci) ──────────────────────────────────
//
// The watcher's three say() lines were the only copy of the reason on the
// double-failure path — the wall came off AND bd could not be written —
// and since ranger-base-1l1en the pane they were said to closes with the
// watcher. These pin the durable copy: that it is written, that nothing
// that cleans up after a seat removes it, that the readers find it, and
// that a watcher which could not write it says so rather than going quiet.

// wwSay drives one detection's lines through the same wallWatchSaid the
// watcher's respond path and the backstop's kill arm both use, and hands
// back what reached the pane.
func wwSay(t *testing.T, path string, d wallWatchDetach, lines ...string) []string {
	t.Helper()
	var pane []string
	v := &wallWatchSaid{Path: path, Pane: func(s string) { pane = append(pane, s) }}
	v.note(wallWatchVerdictHeader(d))
	for _, ln := range lines {
		v.say("%s", ln)
	}
	v.close()
	return pane
}

func TestWallWatchVerdictOutlivesTheSeatsRecordAndIsReadBack(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	mounts := filepath.Join(state, "gates", "developer", "bwrap.mounts")
	record := filepath.Join(wallWatchRecordDir(mounts), "100.json")
	if err := writeWallWatchRecord(record, wallWatchRecord{Seat: "developer", Dir: "/w/d", WatcherPid: 100, MntNS: 7}); err != nil {
		t.Fatal(err)
	}
	d := wallWatchDetach{Seat: "developer", Dir: "/w/d", BwrapPid: 10, SeatPid: 11,
		At: time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC), Missing: []WallWatchMount{{WallWatchDeny, "/h/.claude/settings.json"}}}
	pane := wwSay(t, wallWatchVerdictPath(record), d,
		"KILLED seat developer: the wall over /h/.claude/settings.json came off",
		"⚠ the record is incomplete: create: bd timed out — this line is the record")
	if len(pane) != 2 {
		t.Fatalf("the pane got %d lines, want the two that were said (and no warning): %q", len(pane), pane)
	}

	// The watcher's own clean-up and the backstop's both take the RECORD;
	// the verdict is a different name on purpose and must survive each.
	if err := os.Remove(record); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{wallWatchRecordGlob(state), wallWatchVerdictGlob(state)} {
		got, err := filepath.Glob(p)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if strings.HasSuffix(p, "*.verdict") {
			want = 1
		}
		if len(got) != want {
			t.Errorf("glob %s matched %v, want %d", p, got, want)
		}
	}

	vs := readWallWatchVerdicts(state, "")
	if len(vs) != 1 || vs[0].Seat != "developer" {
		t.Fatalf("read back %+v, want one verdict of seat developer", vs)
	}
	for _, want := range []string{
		"── posse wall-watch verdict · 2026-11-15T09:00:00Z · seat developer · dir /w/d · bwrap pid 10 · seat pid 11",
		"KILLED seat developer: the wall over /h/.claude/settings.json came off",
		"this line is the record",
	} {
		if !strings.Contains(vs[0].Text, want) {
			t.Errorf("the verdict does not carry %q:\n%s", want, vs[0].Text)
		}
	}
	if n := readWallWatchVerdicts(state, "someone-else"); len(n) != 0 {
		t.Errorf("narrowing by persona returned another persona's verdict: %+v", n)
	}
	if n := readWallWatchVerdicts(state, "developer"); len(n) != 1 {
		t.Errorf("narrowing by the owning persona returned %d, want 1", len(n))
	}
}

// A verdict that cannot be written is the one case where the pane is the
// record again, and the reader has to be told so.
func TestWallWatchSaysWhenTheDurableVerdictCannotBeWritten(t *testing.T) {
	t.Parallel()
	// A plain file where the verdict's directory must be: MkdirAll fails.
	root := t.TempDir()
	blocked := filepath.Join(root, "wall-watch")
	if err := os.WriteFile(blocked, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	pane := wwSay(t, filepath.Join(blocked, "100.verdict"), wallWatchDetach{Seat: "a"}, "KILLED seat a")
	if len(pane) != 2 || !strings.Contains(pane[1], "could not be written") || !strings.Contains(pane[1], "only copy") {
		t.Fatalf("pane = %q, want the said line and then a warning that it is the only copy", pane)
	}
}

func TestWallWatchBackstopLeavesTheKilledSeatsVerdictBehind(t *testing.T) {
	t.Parallel()
	state, path, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{}
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}}
	lines := wallWatchBackstop(state, st, "", time.Now, p)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the record of a killed seat whose watcher is gone was kept (%v)", err)
	}
	vs := readWallWatchVerdicts(state, "developer")
	if len(vs) != 1 {
		t.Fatalf("%d verdicts on disk, want 1 (tick said %q)", len(vs), lines)
	}
	for _, want := range []string{"seat developer", "KILLED seat developer", wallWatchPaths([]WallWatchMount{{WallWatchDeny, "/h/.claude/settings.json"}}), "filed " + st.issues[0].ID} {
		if !strings.Contains(vs[0].Text, want) {
			t.Errorf("the verdict does not carry %q:\n%s", want, vs[0].Text)
		}
	}
	// The header is the file's alone — the tick's reader has it already.
	if strings.Contains(strings.Join(lines, "\n"), "posse wall-watch verdict ·") {
		t.Errorf("the header leaked into the tick's output:\n%s", strings.Join(lines, "\n"))
	}
}

// The dead-watcher arm runs on every tick for as long as the store is
// down. Its durable copy is a record, not a log: one per seat.
func TestWallWatchBackstopWritesTheDeadWatcherVerdictOncePerSeat(t *testing.T) {
	t.Parallel()
	state, _, _ := wwBackstopRecord(t)
	st := &fakeWallWatchStore{createErr: errors.New("bd timed out")}
	p := &fakeWallWatchProbe{pids: []int{103}, table: wwIntact()}
	for i := 0; i < 3; i++ {
		lines := wallWatchBackstop(state, st, "", time.Now, p)
		if len(lines) != 1 || !strings.Contains(lines[0], "could not be filed") {
			t.Fatalf("tick %d said %q, want the one line that is the record", i, lines)
		}
	}
	vs := readWallWatchVerdicts(state, "developer")
	if len(vs) != 1 {
		t.Fatalf("%d verdicts after three ticks, want 1", len(vs))
	}
	if n := strings.Count(vs[0].Text, "could not be filed"); n != 1 {
		t.Errorf("the verdict carries the line %d times, want 1:\n%s", n, vs[0].Text)
	}
	if !strings.Contains(vs[0].Text, "watcher pid 100 (gone)") {
		t.Errorf("the verdict does not name the dead watcher:\n%s", vs[0].Text)
	}
}

// What `posse peek` adds when the pane it was asked for is not there.
func TestWallWatchVerdictNoticeNamesWhereTheReasonWent(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	a := &App{StateDir: state}
	if n := a.WallWatchVerdictNotice(""); n != "" {
		t.Fatalf("with no verdict on disk the notice must be empty, got %q", n)
	}
	record := filepath.Join(wallWatchRecordDir(filepath.Join(state, "gates", "developer", "bwrap.mounts")), "100.json")
	wwSay(t, wallWatchVerdictPath(record), wallWatchDetach{Seat: "developer", Dir: "/w/d"}, "KILLED seat developer: the wall over /x came off")

	// With no persona — a name that resolved to nothing — the count and
	// the path and no more, so a mistyped session name is not buried.
	n := a.WallWatchVerdictNotice("")
	if !strings.Contains(n, "1 verdict(s)") || !strings.Contains(n, "100.verdict") {
		t.Errorf("notice = %q, want the count and the newest path", n)
	}
	if strings.Contains(n, "the wall over /x came off") {
		t.Errorf("the unnarrowed notice printed a verdict's text:\n%s", n)
	}
	// With the persona, the newest verdict in full.
	n = a.WallWatchVerdictNotice("developer")
	if !strings.Contains(n, "KILLED seat developer: the wall over /x came off") {
		t.Errorf("notice for the persona = %q, want the verdict in full", n)
	}
	if a.WallWatchVerdictNotice("someone-else") != "" {
		t.Error("another persona's peek was handed this persona's verdict")
	}
}

// ── the one line a reader puts over a set of verdicts (ranger-base-oe884) ──
//
// `posse gates <persona>` and `posse peek` each summarize a persona's whole
// set in one line, and that line said "a seat ... was KILLED for a wall that
// came off" over every verdict on disk — including one whose body says the
// kill could NOT be confirmed, which is the reading whose whole point is
// that a human must go and end that seat by hand.
//
// These rows are the ROUND TRIP, not a hand-written fixture: a tick that
// really detached writes the file, readWallWatchVerdicts reads it back, and
// WallWatchVerdictsSay says what it found. The reader matches on the
// writer's own head constants, so a renamed head reds here too and does not
// just leave the reader quietly finding nothing.

// wwVerdictPathFor is where a persona's watcher of that pid leaves its
// verdict under a state dir — composed the way the watcher composes it.
func wwVerdictPathFor(state, persona, pid string) string {
	return wallWatchVerdictPath(filepath.Join(wallWatchRecordDir(filepath.Join(state, "gates", persona, "bwrap.mounts")), pid+".json"))
}

// wwRefuse is a verdict as `posse wall-watch`'s refuse() leaves one: a
// refusal header and one line, and no kill reported at all
// (ranger-base-r9slg).
func wwRefuse(t *testing.T, path string) {
	t.Helper()
	if err := appendWallWatchVerdict(path, wallWatchRefusedVerdictHeader("developer", "/w/d", 100, 0, 0, time.Now())); err != nil {
		t.Fatal(err)
	}
	if err := appendWallWatchVerdict(path, "cannot read the walls to hold this seat to (open /g/bwrap.mounts: no such file or directory) — refusing to start the seat unwatched (ranger-base-0qpsw D1)"); err != nil {
		t.Fatal(err)
	}
}

func TestWallWatchVerdictsSayCountsAnUnconfirmedKillApartFromAKill(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		probe   *fakeWallWatchProbe
		want    []string
		wantNot []string
	}{
		{"a confirmed kill", &fakeWallWatchProbe{pids: []int{102, 103}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}},
			[]string{"a seat was KILLED for a wall that came off (ranger-base-0qpsw D2)"},
			[]string{"NOT", "RUNNING", "by hand"}},
		{"a kill that did not confirm", &fakeWallWatchProbe{pids: []int{102, 103}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}, killErr: wwKillErr},
			[]string{"could NOT be confirmed", "may still be RUNNING", "end it by hand"},
			[]string{"KILLED"}},
		// A kill that found the seat already over is a seat that is over.
		{"a kill that found nothing left", &fakeWallWatchProbe{pids: []int{102}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}, killNothing: true},
			[]string{"a seat was KILLED for a wall that came off (ranger-base-0qpsw D2)"},
			[]string{"NOT", "RUNNING"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			state, _, _ := wwBackstopRecord(t)
			lines := wallWatchBackstop(state, &fakeWallWatchStore{}, "", time.Now, c.probe)
			vs := readWallWatchVerdicts(state, "developer")
			if len(vs) != 1 {
				t.Fatalf("%d verdicts, want 1 (tick said %q)", len(vs), lines)
			}
			got := WallWatchVerdictsSay(vs)
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("the summary of %s does not say %q:\n%s\nover the verdict:\n%s", c.name, w, got, vs[0].Text)
				}
			}
			for _, w := range c.wantNot {
				if strings.Contains(got, w) {
					t.Errorf("the summary of %s says %q:\n%s", c.name, w, got)
				}
			}
		})
	}
}

// A refusal is in the same file as a detach and reports no kill: nine of the
// watcher's lines end the seat before it runs, or run it where nobody can
// see it, and none of them is a wall that came off.
func TestWallWatchVerdictsSayClaimsNoKillOverARefusal(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	wwRefuse(t, wwVerdictPathFor(state, "developer", "100"))
	got := WallWatchVerdictsSay(readWallWatchVerdicts(state, "developer"))
	if !strings.Contains(got, "no kill is reported") || !strings.Contains(got, "refused to start its seat") {
		t.Errorf("the summary of a refusal = %q, want it to claim no kill", got)
	}
	if strings.Contains(got, "KILLED") {
		t.Errorf("the summary reports a refused seat as KILLED: %q", got)
	}
}

// A persona with one of each: three counts, kept apart, in one line.
func TestWallWatchVerdictsSayCountsAMixedSetApart(t *testing.T) {
	t.Parallel()
	state, _, _ := wwBackstopRecord(t)
	p := &fakeWallWatchProbe{pids: []int{102, 103}, table: mountTable{"/h/.claude/shell-snapshots": {"41"}}, killErr: wwKillErr}
	wallWatchBackstop(state, &fakeWallWatchStore{}, "", time.Now, p)
	d := wallWatchDetach{Seat: "developer", Dir: "/w/d", BwrapPid: 10, SeatPid: 11, Killed: []int{11}}
	wwSay(t, wwVerdictPathFor(state, "developer", "200"), d, wallWatchKillHead(d)+": the wall over /x came off")
	wwRefuse(t, wwVerdictPathFor(state, "developer", "300"))

	vs := readWallWatchVerdicts(state, "developer")
	if len(vs) != 3 {
		t.Fatalf("%d verdicts, want 3", len(vs))
	}
	got := WallWatchVerdictsSay(vs)
	for _, w := range []string{"1 report a seat KILLED for a wall that came off", "1 could NOT confirm the kill, so that seat may still be RUNNING", "1 report no kill at all"} {
		if !strings.Contains(got, w) {
			t.Errorf("the summary of three readings does not say %q:\n%s", w, got)
		}
	}
}

// The counting itself, over the readings a tick cannot write: a verdict that
// is on disk and could not be read says nothing about a kill, and a file
// that holds BOTH readings takes the louder one — a seat that may still be
// running is what its reader has to act on.
func TestWallWatchVerdictsSayOverReadingsNoTickWrites(t *testing.T) {
	t.Parallel()
	if got := WallWatchVerdictsSay(nil); got != "" {
		t.Errorf("the summary of no verdict = %q, want empty", got)
	}
	unread := []WallWatchVerdict{{Text: "⚠ this verdict is on disk and could not be read: permission denied"}}
	if got := WallWatchVerdictsSay(unread); !strings.Contains(got, "could not be read") || strings.Contains(got, "KILLED") {
		t.Errorf("the summary of an unreadable verdict = %q, want no kill claimed", got)
	}
	both := []WallWatchVerdict{{Text: "KILLED seat a: the wall over /x came off\ncould NOT confirm the kill of seat a — the wall over /y came off"}}
	if got := WallWatchVerdictsSay(both); !strings.Contains(got, "may still be RUNNING") || strings.Contains(got, "KILLED") {
		t.Errorf("the summary of a file holding both readings = %q, want the unconfirmed one", got)
	}
}

// peek's notice is the other reader of the same set.
func TestWallWatchVerdictNoticeDoesNotSayKilledOverAnUnconfirmedKill(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	a := &App{StateDir: state}
	d := wallWatchDetach{Seat: "developer", Dir: "/w/d", BwrapPid: 10, SeatPid: 11, Killed: []int{11}, KillErr: wwKillErr}
	wwSay(t, wwVerdictPathFor(state, "developer", "100"), d,
		wallWatchKillHead(d)+": the wall over /x came off", "killed: "+wallWatchKilled(d))
	for _, seat := range []string{"", "developer"} {
		n := a.WallWatchVerdictNotice(seat)
		if !strings.Contains(n, "may still be RUNNING") || !strings.Contains(n, "end it by hand") {
			t.Errorf("peek's notice for seat %q does not say the seat may still be running:\n%s", seat, n)
		}
		if strings.Contains(n, "KILLED") {
			t.Errorf("peek's notice for seat %q reports as KILLED a seat whose kill did not confirm:\n%s", seat, n)
		}
		if !strings.Contains(n, "1 verdict(s)") {
			t.Errorf("peek's notice for seat %q lost its count:\n%s", seat, n)
		}
	}
}

// TestWallWatchVerdictLineReachesTheFileBeforeThePane is the ORDER inside
// say() (ranger-base-r9slg, escaped from ranger-base-pfcci). MEASURED
// 2026-11-15 at 4b3013cd: moving `v.Pane(line)` above `v.note(line)` left
// `go test ./internal/posse` green (4054 ran, 0 FAIL) and `./cmd/posse` green
// (391 ran) — every pin here reads the file AFTER the call and cannot tell
// which write went first, and wwSay's own Pane appends to a slice and reads
// nothing.
//
// Reversed, a watcher that dies between the two writes leaves the line in the
// scrollback that is about to close and nowhere else, which is the whole
// failure ranger-base-pfcci exists to close: since ranger-base-1l1en the
// watcher IS the pane, so the pane goes when this process does
// (wallwatch_linux.go's own sentence, "EVERY LINE TO THE FILE FIRST").
//
// ONE PIN FOR BOTH CALL SITES, because the order is in `say` and both sites
// reach it the same way: the respond path (wallwatch_linux.go) and the
// backstop's kill arm (wallwatch.go) each build this struct, call note once
// for the header and say for every line. A per-site pin would be two copies
// of one assertion about a method neither of them has its own copy of.
func TestWallWatchVerdictLineReachesTheFileBeforeThePane(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gates", "developer", "wall-watch", "100.verdict")
	const header = "── posse wall-watch verdict · header"
	var pane []string
	v := &wallWatchSaid{Path: path, Pane: func(s string) {
		b, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(b), s) {
			t.Errorf("%q reached the pane before the durable file held it (read: %v) — a watcher that dies between the two writes leaves the line in a scrollback that is about to close and nowhere else (ranger-base-pfcci, ranger-base-r9slg)", s, err)
		}
		pane = append(pane, s)
	}}

	// The header is file-only: a line through note() must reach the pane not
	// at all, so the arm above cannot be satisfied by saying everything twice.
	v.note(header)
	if len(pane) != 0 {
		t.Errorf("note() said %q on the pane; the header is the file's alone", pane)
	}
	v.say("KILLED seat developer: the wall over /h/.claude/settings.json came off")
	v.say("  filed %s", "ranger-base-0000")
	v.say("⚠ the record is incomplete: create: bd timed out — this line is the record")
	v.close()
	if v.Err != nil {
		t.Fatalf("FIXTURE: the durable write failed (%v), so the Pane above was never asked the question", v.Err)
	}
	if len(pane) != 3 {
		t.Fatalf("the pane got %d lines, want the three that were said: %q", len(pane), pane)
	}
	// CONTROL: the file really is what was read — an empty or missing file
	// would make the arm above green by never containing anything to compare.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range append([]string{header}, pane...) {
		if !strings.Contains(string(b), want) {
			t.Errorf("the durable file does not carry %q:\n%s", want, b)
		}
	}
}

// TestWallWatchAnUnreadableVerdictFileIsSaidAndNotSkipped is
// readWallWatchVerdicts' "SAID AND NOT SKIPPED" arm (ranger-base-r9slg,
// escaped from ranger-base-pfcci): a verdict that is ON DISK and cannot be
// READ is reported as a ⚠ row rather than omitted, which is the comment two
// lines above it — "the one thing a reader of this surface must not be told
// is absent".
//
// Nothing drove it. MEASURED 2026-11-15 at 4b3013cd: replacing that arm with
// a `continue` left `go test ./internal/posse` green (4054 ran) and
// `./cmd/posse` green (391 ran). The two readers' pins
// (cmd/posse/wallwatchverdict_qa_test.go, and this file above) all plant a
// READABLE file; TestWallWatchBackstopSaysWhenARecordCannotBeRead is the
// `.json` RECORD, a different arm, and
// TestWallWatchSaysWhenTheDurableVerdictCannotBeWritten is the WRITE failure.
//
// A seat whose wall came off is KILLED and its pane closes with it, so a row
// silently dropped here is the reason gone from the one surface that still
// has it.
func TestWallWatchAnUnreadableVerdictFileIsSaidAndNotSkipped(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	dir := filepath.Join(state, "gates", "developer", "wall-watch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A readable verdict beside it, so the count below separates "the row is
	// a ⚠" from "the glob found nothing".
	good := filepath.Join(dir, "1.verdict")
	write(t, good, "KILLED seat developer: the wall came off\n")

	// A DIRECTORY at a verdict's name: os.ReadFile fails for every uid,
	// including root, which `chmod 0000` does not — and the suite is run as
	// root in some containers.
	asDir := filepath.Join(dir, "2.verdict")
	if err := os.Mkdir(asDir, 0o755); err != nil {
		t.Fatal(err)
	}
	unreadable := []string{asDir}
	if os.Geteuid() != 0 {
		// And the shape an operator actually produces: a file this process
		// may not open.
		noPerm := filepath.Join(dir, "3.verdict")
		write(t, noPerm, "KILLED seat developer: unreadable\n")
		if err := os.Chmod(noPerm, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(noPerm, 0o644) })
		unreadable = append(unreadable, noPerm)
	}

	vs := readWallWatchVerdicts(state, "developer")
	if len(vs) != 1+len(unreadable) {
		t.Fatalf("read back %d verdict(s), want all %d on disk — an unreadable verdict must be SAID and not skipped (ranger-base-pfcci):\n%+v", len(vs), 1+len(unreadable), vs)
	}
	byPath := map[string]WallWatchVerdict{}
	for _, v := range vs {
		byPath[v.Path] = v
	}
	for _, p := range unreadable {
		v, ok := byPath[p]
		if !ok {
			t.Errorf("%s is on disk and is not in the listing at all", p)
			continue
		}
		if !strings.Contains(v.Text, "⚠") || !strings.Contains(v.Text, "could not be read") {
			t.Errorf("the row for the unreadable %s does not say so: %q", p, v.Text)
		}
		if v.Seat != "developer" {
			t.Errorf("the row for %s names seat %q", p, v.Seat)
		}
	}
	// CONTROL: the readable one still reads back as its own text, so the arm
	// above is about the unreadable files and not about every row.
	if v := byPath[good]; !strings.Contains(v.Text, "the wall came off") || strings.Contains(v.Text, "could not be read") {
		t.Errorf("the readable verdict reads back as %q", v.Text)
	}
}
