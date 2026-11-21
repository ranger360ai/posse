//go:build !posse_arm2 && !posse_arm3

package posse

// EVERY CONFIG KEY THE HARNESS READS IS ON THE SURFACE, OR REGISTERED WITH A
// REASON (ranger-base-w9t63).
//
// examples/config.yaml is what `posse init` copies verbatim into a fresh
// RHQ_HOME, so it is the de facto instance spec — and until this bead nothing
// asked it the question in that direction. seedconfig_test.go holds the
// inverse (`TestSeedConfigLiveKeysAreRead`: every key the seed ships ARMED is
// read by some non-test Go file) and two value pins over keys a documented
// line already exists for. A key the harness reads and the seed never names
// is invisible to all three: it is not armed, and it has no line for a value
// pin to be about.
//
// WHAT THAT COST, MEASURED. ranger-base-1qi7c found the whole `pulse_*`
// family undocumented — four keys, one of them the ticker's arm switch and
// one ADR 0093 D2's exit hatch — and fixed it with a pin over ONE family
// derived from ONE named file, saying in its own head comment that the
// general case was somebody else's. This census is that case, and its first
// run found six more keys in four families: `auto_reap:` (a destructive
// sweep's on/off), `reap_crew_after:`, `reap_unpointed_after:` and
// `retire_tree_after:` (that sweep's three graces, every one of them the only
// thing between a live conversation and a closed workspace), `load_guard_kill:`
// (a kill arm behind an operator flip) and `backup_max_age:`. Five are
// documented by this bead; the sixth is registered below, because another
// pin already rules that it may not carry a line.
//
// THE DERIVATION, in four parts, because each one is a place a key could
// otherwise hide:
//
//  1. THE FLAT READERS are the functions in yamlflat.go that OPEN their own
//     first parameter and take the key second, read out of that file rather
//     than listed — `YamlGet`, `YamlList`, `YamlMapPairs`,
//     `YamlMapPairsRaw`, `yamlHasKey`, `YamlKeysWithPrefix`. Opening is
//     transitive and seeded with `os.ReadFile`: `readLines` passes its
//     parameter to that, the exported readers pass theirs to `readLines`,
//     and `YamlMapPairs` passes its to `YamlMapPairsRaw`. The two `(lines
//     []string, key string)` helpers beside them never reach the file, and
//     neither does `flatFieldRefusal(key, v string)` — which is the whole
//     discriminator. A seventh added tomorrow is in the census the day it is
//     written, whatever its parameters are called: the rule was keyed on the
//     first parameter being NAMED `path` until ranger-base-3weac, a name no
//     rule in this tree requires, and a reader spelled `(file, key string)`
//     escaped it in silence (MEASURED 2026-11-21; the arm that pins it is in
//     TestConfigKeyCensusCanStillSayNo).
//  2. A READ OF THE INSTANCE CONFIG is one of those calls whose PATH
//     argument is a `.ConfigPath` selector. That is the whole scope rule, and
//     it is load-bearing: these same six functions read runtime adapter
//     files, recipe files, session records, launch locks, verify boxes and
//     cage profiles — 97 of their 144 call sites in non-test Go, MEASURED
//     2026-11-21 — and `detection:`, `command:`, `pid:`, `started:` are keys
//     of those files and not of this one. A census that read them all would
//     need sixty register rows saying "not the instance config", which is
//     how a register stops being read.
//  3. A SHARED READER IS A READER. `attn_parked_age:` is never handed to
//     YamlGet by name: `AttnParkedAge` calls `a.attnAge("attn_parked_age",
//     DefaultAttnParkedAge, errw)` and the YamlGet is one hop down, keyed by
//     attnAge's own parameter. So the reader set is a FIXPOINT — a function
//     that hands one of its own string parameters to a config reader's key
//     slot is itself a config reader, at that parameter's index — and it is
//     seeded with nothing at all: `CfgGet` is derived like the rest, from its
//     own `YamlGet(a.ConfigPath, key)` body. Five members today (CfgGet,
//     attnAge, budgetDollars, graceAfter, planPercent), and the hop stops
//     exactly where it should: `declaredBy` reads `rt.Path` and
//     `runtimeScalarOrList` reads a path PARAMETER, so neither is a config
//     reader and `prompt:`/`state_dir:` stay out, along with every other
//     runtime-contract key those two carry.
//     A FUNCTION LITERAL assigned to an identifier counts too, and
//     is resolved inside its own enclosing function and nowhere else:
//     `OpsPatternSet`'s `read :=` closure is the only reader of
//     `data_ceiling_patterns:` and `beads_visibility_patterns:`, and `read`
//     is a name four other unrelated closures in this package also use.
//  4. A KEY THAT IS NOT A LITERAL is resolved where the resolution is exact
//     and REGISTERED where it is not. A package-level string constant is
//     resolved — `AutostartUnitKey`, `AutostartViaKey`, `DataCeilingConfigKey`,
//     `MultiplexerKey`, `OpsPatternsConfigKey`, `RexBinKey`,
//     `TranscriptsConfigKey`, `UpgradeDelegateKey`, eight keys spelled that
//     way. A `range` over a `[]string` of literals — a composite literal, or
//     a package var holding one — yields every literal in it: `backupKeys`,
//     `BudgetCapsConfigured`'s inline pair and `retireExamplePIDs`' triple
//     are the three sets spelled that way, and every member of all three has
//     a reader of its own as well today — so what that rule buys is the NEXT
//     such list, where a key added to it would otherwise arrive with no
//     reader of its own and nothing to notice it.
//     The reader's own body, where the key IS the reader's parameter,
//     is dropped, because part 3 already walked that reader's call sites.
//     What is left is four sites, and each one has a row in
//     seedKeyDynamicSite saying what the key set is and where the census
//     picks it up instead.
//
// AND BOTH REGISTERS ARE MADE TOTAL BY THE DERIVATION, the property
// seedconfig_test.go's three registers have and a bare list would not: a row
// for a key no reader reads, a row for a key the seed has since documented,
// and a row for a dynamic site that is gone all fail — as does a dynamic site
// with no row.
//
// WHAT IS STILL OUTSIDE, said here rather than left to be rediscovered:
//
//   - THE OTHER DIRECTION, for COMMENTED keys. A `# key:` line the harness
//     no longer reads is a documented key with no code behind it — `dirs:`
//     one surface over (ranger-base-aox), where the armed half of that
//     question already has a pin. This census would report it as a seed line
//     nothing matched, and does not, because the seed's prose names retired
//     keys on purpose (`plan_guard_overflow:` is discussed as removed) and
//     telling those apart is a register this bead did not need.
//   - A READ THAT IS NOT GO. plugin/*.sh and the hook renders read config
//     through awk; nothing here sees them.
//   - A CONFIG PATH THAT IS NOT A `.ConfigPath` SELECTOR. A read through a
//     local copied out of it, or through a function parameter, is invisible
//     to part 2 — `CageRealizeEntries(cfg, goos)` is spelled exactly that
//     way, which is why `cage_realize_<goos>:` is in this census through
//     neither route and is documented in the seed anyway (it has its own
//     block, ADR 0083 D7). Widening part 2 to a parameter means an
//     interprocedural rule over which paths reach which parameters, and the
//     fence that makes the narrow rule honest is TestConfigKeyCensusCanStillSayNo
//     below: it plants that shape and asserts the census does not see it, so
//     the hole is pinned rather than believed.

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ───────────────────────────────────────────────────────────────────────────
// THE REGISTERS.

// seedKeyUndocumented records a config key the harness reads that
// examples/config.yaml does NOT document with a line. Key -> why, and the why
// is the whole of the row: "not documented" is the finding this pin exists to
// report, so a row has to say what makes this key the exception.
//
// One member. Its reason is another pin's ruling and not this bead's
// judgement, which is the shape to copy: a key is in here because something
// else in the tree says the line may not exist, or because the key is not an
// operator's to set at all.
var seedKeyUndocumented = map[string]string{
	"backup_max_age": "the seed documents it in PROSE, in the backup block (\"the fifth key is the one " +
		"with no number to document\"), and may not carry a `# backup_max_age:` line: ADR 0036 §6 makes its " +
		"default a RULE rather than a constant — 2x a valid backup_interval:, falling back to " +
		"DefaultBackupMaxAge only on an instance with no schedule — so seedDurationDefaultNotAConstant's " +
		"stronger half refuses any documented line for it as a claim nothing can hold (ranger-base-s1dmh). " +
		"A line here and that row cannot both be right; the row is older and the ruling is its",
}

// seedKeyDynamicSite records a config read whose key is not a literal, not a
// string constant, and not a range over literals — so this census cannot say
// WHICH key it is. Site -> why, where the site is spelled
// `<function>(<the key expression>)` and the why says what the key set
// actually is and where the census reaches those keys instead.
//
// A row is not an excuse, it is a redirection. Three of the four below name
// an enumeration this census already holds — the `plan_guard_` prefix read,
// and `guardValueKeys`' own members' readers. The fourth is the one that says
// plainly that the census does NOT reach its key (`uncounted_cap_<runtime>:`
// is concatenated at the call and read through CfgGet, so there is no prefix
// to ask about) and names where the seed documents it instead. That is what a
// row is for: a site whose keys reached the census through nothing and said
// so nowhere would be the hole this pin exists to close.
var seedKeyDynamicSite = map[string]string{
	"PlanGuardThresholds(key)": "the key is a MEMBER of the `plan_guard_<window>:` family, ranged out of " +
		"the YamlKeysWithPrefix read two lines up — the window names belong to whichever provider adapter " +
		"is installed (ADR 0012 D4), so no literal exists to find. The census holds the FAMILY instead, " +
		"through that prefix read: `plan_guard_` is documented when the seed documents a line with that " +
		"prefix, which it does twice (plan_guard_5h:, plan_guard_7d:)",
	"DeriveGuardValueLiterals(key)": "the key set is `guardValueKeys` (a package var of literals) plus the " +
		"same `plan_guard_` family, assembled into a local with append — so the range is over a variable " +
		"and not over the literals. Every member reaches this census by its own reader: the seven in " +
		"guardValueKeys are each read by a named reader elsewhere in the tree, and the family is held " +
		"through the prefix read above",
	"UncountedCap(\"uncounted_cap_\" + runtime)": "a key assembled from a runtime NAME at read time " +
		"(`uncounted_cap_<runtime>:`, ADR 0013 §5) — the adapters are whatever is installed, the same open " +
		"set plan_guard_ has, and there is no literal to find. UNLIKE that family it is read through " +
		"CfgGet rather than through a prefix enumeration, so there is no prefix for the family arm above " +
		"to ask about either. The seed's account-degraded block does document it — the " +
		"`uncounted_cap_<runtime>:` shape in prose and `# uncounted_cap_codex: 20` as a member line — so " +
		"what is missing is this census's reach and not the documentation",
	"countedCapDead(\"uncounted_cap_\" + rt.Name)": "the same key, read a second time to warn that a cap " +
		"the operator set no longer applies because the runtime moved out from under it " +
		"(ranger-base-0lg6). Same reason as the row above, and a separate row rather than folded into it " +
		"because the SITE is the unit here: a register keyed on the key expression would silently cover a " +
		"third site that happened to spell the same concatenation",
}

// ───────────────────────────────────────────────────────────────────────────
// THE DERIVATION.

// skcKeySite is one config key the harness reads, with where it reads it.
type skcKeySite struct {
	key    string // the config key
	how    string // literal | const <Name> | ranged over <expr>
	fn     string // the function (or assigned func literal) the read is in
	callee string // the reader it was handed to
	where  string // path:line, repo-relative
	prefix bool   // the key is a PREFIX (YamlKeysWithPrefix), not a whole key
}

// skcDynamicSite is one config read whose key this census cannot name.
type skcDynamicSite struct {
	site  string // "<function>(<key expression>)", the seedKeyDynamicSite key
	where string
}

// skcDecl is one function body with its parameter names: a FuncDecl, or a
// func literal assigned to an identifier inside one.
type skcDecl struct {
	name   string
	params []string
	body   *ast.BlockStmt
	file   string // repo-relative
}

// skcCensus is one walk's whole answer.
type skcCensus struct {
	scanned int              // non-test .go files parsed
	flat    map[string]int   // yamlflat.go reader -> key argument index
	readers map[string]int   // config reader -> key argument index
	keys    []skcKeySite     // sorted by key then site
	dynamic []skcDynamicSite // sorted by site
}

// skcConfigKeyCensus walks root's non-test Go and answers the four parts of
// the derivation in the head comment.
//
// It takes the root as a PARAMETER rather than calling qibRepoRoot itself,
// which is what lets TestConfigKeyCensusCanStillSayNo drive the same rules
// over planted sources. The pins below pass qibRepoRoot(t), so they are in
// the tree-wide class and behind `make seed-check`.
func skcConfigKeyCensus(t *testing.T, root string) skcCensus {
	t.Helper()
	fset := token.NewFileSet()
	type parsed struct {
		rel  string
		base string
		file *ast.File
	}
	var files []parsed
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			// A file this walk cannot parse is a file whose config reads are
			// invisible, which is the failure this census exists to prevent.
			t.Fatalf("%s: %v — a non-test Go file this census cannot parse is a file whose config reads it cannot see", p, perr)
		}
		rel, _ := filepath.Rel(root, p)
		files = append(files, parsed{filepath.ToSlash(rel), filepath.Base(p), f})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	c := skcCensus{
		scanned: len(files),
		flat:    map[string]int{},
		readers: map[string]int{},
	}

	// Part 1: the flat readers, out of yamlflat.go's own BODIES. A reader
	// there is a function that turns its FIRST parameter into the file's
	// bytes and takes the key second: `readLines` hands that parameter to
	// `os.ReadFile`, the exported readers hand it to `readLines`, and
	// `YamlMapPairs` hands it to `YamlMapPairsRaw` — so the set is the
	// transitive closure of "hands its first parameter to something in this
	// file that opens it", seeded with the stdlib call and naming no local
	// identifier of its own. The `(lines []string, key string)` helpers
	// beside them never touch the file, and neither does
	// `flatFieldRefusal(key, v string)`, which is two strings — that is the
	// whole discriminator, and the reason this is derived rather than typed:
	// a seventh reader added to that file is in the census without anybody
	// remembering this test exists.
	//
	// It was keyed on the parameter NAME until ranger-base-3weac — `names[0]
	// == "path"` — and nothing in this tree ever required that name.
	// MEASURED there, 2026-11-21: the same reader spelled `(file, key
	// string)` with a non-test `.ConfigPath` caller was derived by nothing
	// and its key named nowhere, `make seed-check` green and silent, while
	// the `path` spelling of it reds. The name test was also what did the
	// EXCLUDING, which is why deleting it alone was not the fix: without it
	// `flatFieldRefusal(key, v string)` reads as a reader with `v` in the
	// key slot. MEASURED the same day with that one conjunct removed and
	// nothing else changed: 7 flat readers instead of 6, and 112 sites over
	// 79 distinct keys — UNCHANGED, because part 2's scope rule already
	// filters that function's call sites, which pass a key string where a
	// `.ConfigPath` selector would have to be. So the exclusion held the
	// derived SET honest and added no key here; the next `(string, string)`
	// non-reader in that file may not be so harmless. The body rule
	// excludes it for the reason the head comment always gave — it does not
	// read the file.
	//
	// The key index is 1, which is a SHAPE and not a derivation: a
	// `(path, section, key string)` reader would be censused at `section`.
	// That reds loudly rather than going quiet — `section`'s literal is
	// reported as a config key the seed does not document — so it is left as
	// a shape rather than guessed at.
	var flatDecls []*ast.FuncDecl
	for _, x := range files {
		if x.base != "yamlflat.go" {
			continue
		}
		for _, d := range x.file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Body == nil {
				continue
			}
			flatDecls = append(flatDecls, fd)
		}
	}
	opensFile := map[string]bool{}
	for round := 0; ; round++ {
		added := 0
		for _, fd := range flatDecls {
			if opensFile[fd.Name.Name] {
				continue
			}
			names, _ := skcParams(fd.Type)
			if len(names) == 0 || names[0] == "" {
				continue
			}
			if skcHandsToFileOpen(fd.Body, names[0], opensFile) {
				opensFile[fd.Name.Name] = true
				added++
			}
		}
		if added == 0 {
			break
		}
		if round > len(flatDecls) {
			t.Fatal("the flat-reader derivation did not settle — a derivation that cannot terminate holds nothing")
		}
	}
	for _, fd := range flatDecls {
		if !opensFile[fd.Name.Name] {
			continue
		}
		names, types := skcParams(fd.Type)
		if len(names) >= 2 && skcIsString(types[0]) && skcIsString(types[1]) {
			c.flat[fd.Name.Name] = 1
		}
	}

	// Package-level string constants, and package-level `[]string{...}` vars
	// of literals — the two exact resolutions part 4 allows.
	consts := map[string]string{}
	strLists := map[string][]string{}
	for _, x := range files {
		for _, d := range x.file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, n := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					switch v := vs.Values[i].(type) {
					case *ast.BasicLit:
						if gd.Tok == token.CONST && v.Kind == token.STRING {
							if lit, err := strconv.Unquote(v.Value); err == nil {
								consts[n.Name] = lit
							}
						}
					case *ast.CompositeLit:
						if lits, ok := skcStringLits(v); ok {
							strLists[n.Name] = lits
						}
					}
				}
			}
		}
	}

	// The units: one FuncDecl, plus the func literals assigned to an
	// identifier inside it. A literal's name is resolved inside its own unit
	// and nowhere else — `read` is four different closures in this package.
	type unit struct {
		fn   skcDecl
		lits []skcDecl
	}
	var units []unit
	litBodies := map[*ast.BlockStmt]bool{}
	for _, x := range files {
		for _, d := range x.file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			names, _ := skcParams(fd.Type)
			u := unit{fn: skcDecl{name: fd.Name.Name, params: names, body: fd.Body, file: x.rel}}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
					return true
				}
				id, ok := as.Lhs[0].(*ast.Ident)
				if !ok {
					return true
				}
				fl, ok := as.Rhs[0].(*ast.FuncLit)
				if !ok || fl.Body == nil {
					return true
				}
				ln, _ := skcParams(fl.Type)
				u.lits = append(u.lits, skcDecl{name: id.Name, params: ln, body: fl.Body, file: x.rel})
				litBodies[fl.Body] = true
				return true
			})
			units = append(units, u)
		}
	}

	// keyArg resolves a call to its key-argument index under a reader map,
	// applying part 2's scope rule to the flat readers.
	keyArg := func(readers map[string]int, call *ast.CallExpr) (int, bool) {
		name := skcCallee(call.Fun)
		if i, ok := c.flat[name]; ok {
			if len(call.Args) <= i || !skcIsConfigPath(call.Args[0]) {
				return 0, false
			}
			return i, true
		}
		if i, ok := readers[name]; ok {
			if len(call.Args) <= i {
				return 0, false
			}
			return i, true
		}
		return 0, false
	}

	// paramFed is part 3's rule for one body: does it hand one of its own
	// parameters to a config reader's key slot, and at which index.
	paramFed := func(d skcDecl, readers map[string]int) (int, bool) {
		idx, found := 0, false
		ast.Inspect(d.body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			i, ok := keyArg(readers, call)
			if !ok {
				return true
			}
			id, ok := call.Args[i].(*ast.Ident)
			if !ok {
				return true
			}
			for pi, pn := range d.params {
				if pn == id.Name && !found {
					idx, found = pi, true
				}
			}
			return true
		})
		return idx, found
	}

	// Part 3's fixpoint over the package-level functions. Seeded with
	// nothing: CfgGet is derived from its own body like every other member.
	for round := 0; ; round++ {
		added := 0
		for _, u := range units {
			if _, have := c.readers[u.fn.name]; have {
				continue
			}
			if i, ok := paramFed(u.fn, c.readers); ok {
				c.readers[u.fn.name] = i
				added++
			}
		}
		if added == 0 {
			break
		}
		if round > len(units) {
			t.Fatal("the config-reader fixpoint did not settle — a derivation that cannot terminate holds nothing")
		}
	}

	// The sites.
	for _, u := range units {
		readers := map[string]int{}
		for k, v := range c.readers {
			readers[k] = v
		}
		// The same fixpoint over this unit's own func literals, whose names
		// are local to it.
		for {
			added := 0
			for _, l := range u.lits {
				if _, have := readers[l.name]; have {
					continue
				}
				if i, ok := paramFed(l, readers); ok {
					readers[l.name] = i
					added++
				}
			}
			if added == 0 {
				break
			}
		}
		for _, d := range append([]skcDecl{u.fn}, u.lits...) {
			// Part 4's range rule, per body: a loop variable over a
			// `[]string` of literals IS those literals.
			ranged := map[string][]string{}
			rangedOver := map[string]string{}
			ast.Inspect(d.body, func(n ast.Node) bool {
				rs, ok := n.(*ast.RangeStmt)
				if !ok || rs.Value == nil {
					return true
				}
				vid, ok := rs.Value.(*ast.Ident)
				if !ok {
					return true
				}
				switch x := rs.X.(type) {
				case *ast.CompositeLit:
					if lits, ok := skcStringLits(x); ok {
						ranged[vid.Name] = append(ranged[vid.Name], lits...)
						rangedOver[vid.Name] = "an inline []string"
					}
				case *ast.Ident:
					if lits, ok := strLists[x.Name]; ok {
						ranged[vid.Name] = append(ranged[vid.Name], lits...)
						rangedOver[vid.Name] = x.Name
					}
				}
				return true
			})
			myIdx, isReader := readers[d.name]
			ast.Inspect(d.body, func(n ast.Node) bool {
				// A func literal collected as its own unit member is walked
				// there; walking it twice would read its parameter as an
				// unresolvable key in the enclosing function.
				if fl, ok := n.(*ast.FuncLit); ok && litBodies[fl.Body] {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				i, ok := keyArg(readers, call)
				if !ok {
					return true
				}
				arg := call.Args[i]
				callee := skcCallee(call.Fun)
				where := d.file + ":" + strconv.Itoa(fset.Position(call.Pos()).Line)
				add := func(key, how string) {
					c.keys = append(c.keys, skcKeySite{
						key: key, how: how, fn: d.name, callee: callee, where: where,
						prefix: callee == "YamlKeysWithPrefix",
					})
				}
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if key, err := strconv.Unquote(lit.Value); err == nil {
						add(key, "literal")
						return true
					}
				}
				if id, ok := arg.(*ast.Ident); ok {
					if key, have := consts[id.Name]; have {
						add(key, "const "+id.Name)
						return true
					}
					if lits, have := ranged[id.Name]; have {
						for _, key := range lits {
							add(key, "ranged over "+rangedOver[id.Name])
						}
						return true
					}
					if isReader && myIdx < len(d.params) && d.params[myIdx] == id.Name {
						return true // the reader's own body; part 3 walked its call sites
					}
				}
				c.dynamic = append(c.dynamic, skcDynamicSite{
					site:  d.name + "(" + skcExpr(fset, arg) + ")",
					where: where,
				})
				return true
			})
		}
	}

	sort.Slice(c.keys, func(i, j int) bool {
		if c.keys[i].key != c.keys[j].key {
			return c.keys[i].key < c.keys[j].key
		}
		return c.keys[i].where < c.keys[j].where
	})
	sort.Slice(c.dynamic, func(i, j int) bool {
		if c.dynamic[i].site != c.dynamic[j].site {
			return c.dynamic[i].site < c.dynamic[j].site
		}
		return c.dynamic[i].where < c.dynamic[j].where
	})
	return c
}

// skcParams is one function type's parameter names and their types, flat —
// `func(a, b string)` is two names and two types.
func skcParams(ft *ast.FuncType) (names []string, types []ast.Expr) {
	if ft.Params == nil {
		return nil, nil
	}
	for _, fl := range ft.Params.List {
		if len(fl.Names) == 0 {
			names = append(names, "")
			types = append(types, fl.Type)
			continue
		}
		for _, n := range fl.Names {
			names = append(names, n.Name)
			types = append(types, fl.Type)
		}
	}
	return names, types
}

func skcIsString(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "string"
}

// skcHandsToFileOpen is part 1's rule for one yamlflat.go body: does it hand
// the named parameter to something that OPENS it — `os.ReadFile` itself, or
// another function in that file already derived as doing so — at that
// callee's own first argument, which is where a path goes in every one of
// them. It is what makes part 1 a derivation over behaviour rather than over
// a parameter's spelling (ranger-base-3weac).
func skcHandsToFileOpen(body *ast.BlockStmt, param string, opensFile map[string]bool) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		if !skcIsOsReadFile(call.Fun) && !opensFile[skcCallee(call.Fun)] {
			return true
		}
		if id, ok := call.Args[0].(*ast.Ident); ok && id.Name == param {
			found = true
		}
		return true
	})
	return found
}

// skcIsOsReadFile is the one call part 1's fixpoint is seeded with, and it is
// stdlib on purpose: a seed spelled as a local identifier would be the same
// convention-by-name the body rule replaced. The package qualifier is
// checked, so a `ReadFile` method on some other receiver is not it.
func skcIsOsReadFile(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ReadFile" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "os"
}

// skcIsConfigPath is part 2's scope rule, and the one place it is said: the
// path argument names the INSTANCE CONFIG. `a.ConfigPath`,
// `in.App.ConfigPath`, `d.App.ConfigPath` — a selector whose final name is
// ConfigPath, and nothing else.
func skcIsConfigPath(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "ConfigPath"
}

func skcCallee(e ast.Expr) string {
	switch f := e.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// skcStringLits is a `[]string{"a", "b"}` composite literal's members, and
// false for anything that is not exactly that — a non-string element type, or
// one element that is not a plain literal, drops the whole list, because a
// partial read is worse than no read: it would silently cover some members of
// a key set and leave the rest uncounted.
func skcStringLits(c *ast.CompositeLit) ([]string, bool) {
	at, ok := c.Type.(*ast.ArrayType)
	if !ok || at.Len != nil || !skcIsString(at.Elt) {
		return nil, false
	}
	var out []string
	for _, e := range c.Elts {
		lit, ok := e.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return nil, false
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil, false
		}
		out = append(out, s)
	}
	return out, len(out) > 0
}

// skcExpr renders an expression the way the source spells it, so a dynamic
// site's register key is the text a reader can find in the file.
func skcExpr(fset *token.FileSet, e ast.Expr) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, e); err != nil {
		return "<unprintable>"
	}
	return b.String()
}

// ───────────────────────────────────────────────────────────────────────────
// THE SEED SIDE.

// skcSeedDocumentsKey reports whether examples/config.yaml carries a LINE for
// key — commented (`# key:`, the form every key with teeth ships as) or live.
//
// The line rule is seedDocumentedValue's, which is the rule every other seed
// pin in this package reads with: `#` at column 0, optional blanks, then
// `key:`. PROSE DOES NOT COUNT, on purpose — the seed names retired keys and
// discusses keys of other files in its prose, and a rule that read a
// backticked mention as documentation would pass a key nobody can set.
func skcSeedDocumentsKey(cfgText, key string) bool {
	for _, ln := range strings.Split(cfgText, "\n") {
		if strings.HasPrefix(ln, key+":") {
			return true // live
		}
		if !strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(ln[1:], " \t"), key+":") {
			return true
		}
	}
	return false
}

// skcSeedDocumentsPrefix is the same question for a PREFIX read: the family's
// members are named by whatever is installed (ADR 0012 D4's
// `plan_guard_<window>:`), so what the seed can document is a member, and
// what this asks is whether it documents one. Strictly longer than the
// prefix, so a bare `# plan_guard_:` would not answer for the family.
//
// It returns the members rather than a bool because the caller SUBTRACTS the
// ones that are keys in their own right — see the prefix arm above, and the
// measurement that made it necessary.
func skcSeedDocumentsPrefix(cfgText, prefix string) []string {
	var out []string
	for _, ln := range strings.Split(cfgText, "\n") {
		rest := ln
		if strings.HasPrefix(ln, "#") {
			rest = strings.TrimLeft(ln[1:], " \t")
		} else if ln == "" || ln[0] == ' ' || ln[0] == '\t' {
			continue
		}
		c := strings.Index(rest, ":")
		if c <= len(prefix) {
			continue
		}
		k := rest[:c]
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		if strings.IndexFunc(k, func(r rune) bool {
			return !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
		}) >= 0 {
			continue
		}
		out = append(out, k)
	}
	return out
}

// ───────────────────────────────────────────────────────────────────────────
// THE PINS.

// Every config key the harness reads is documented in examples/config.yaml,
// or registered above with the reason it is not.
func TestEveryConfigKeyTheHarnessReadsIsDocumentedOrRegistered(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)
	c := skcConfigKeyCensus(t, root)
	skcWitness(t, c)

	cfg := seedConfigPath(t)
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)

	// The whole keys the harness reads, for the prefix arm below to subtract.
	ownKey := map[string]bool{}
	for _, s := range c.keys {
		if !s.prefix {
			ownKey[s.key] = true
		}
	}

	// One report per KEY, not per site: `verify_labels:` is read twice two
	// lines apart, and two identical failures for one missing line is noise
	// at exactly the moment the file is long.
	seen := map[string]bool{}
	for _, s := range c.keys {
		if seen[s.key] {
			continue
		}
		seen[s.key] = true
		if s.prefix {
			// A documented line that shares the prefix and is ALSO read as a
			// key of its own is not evidence for the family — it is its own
			// key, already held by this pin on its own row. `plan_guard_`
			// has exactly that neighbour: the seed's commented
			// `plan_guard_blind_max:` line carries that prefix and has a
			// reader of its own (PlanGuardBlindMax), and before this filter
			// it answered for the whole window family.
			// MEASURED 2026-11-21: with the two
			// window examples deleted from the seed and blind_max left, the
			// arm was GREEN. Derived, not a list — the census's own literal
			// keys are what it subtracts.
			var members []string
			for _, m := range skcSeedDocumentsPrefix(text, s.key) {
				if !ownKey[m] {
					members = append(members, m)
				}
			}
			if len(members) == 0 {
				t.Errorf("%s reads the config keys beginning %q (%s, %s), and examples/config.yaml documents no line of that FAMILY — "+
					"its members are named by whatever adapter is installed, so what the seed can show is an example of one, and it shows none "+
					"(a prefixed line that is read as a key of its own does not count: that is its own key, held on its own row)",
					s.where, s.key, s.fn, s.callee)
			}
			continue
		}
		if skcSeedDocumentsKey(text, s.key) {
			if why := seedKeyUndocumented[s.key]; why != "" {
				t.Errorf("seedKeyUndocumented names %q, and examples/config.yaml documents it now (%s) — the row excuses a gap that is closed; drop it",
					s.key, why)
			}
			continue
		}
		if why := strings.TrimSpace(seedKeyUndocumented[s.key]); why != "" {
			t.Logf("%s: %s is undocumented on purpose — %s", s.where, s.key, why)
			continue
		}
		t.Errorf("%s reads config %s: (%s, via %s, %s) and examples/config.yaml documents no such line.\n"+
			"  That file is the config surface — `posse init` copies it verbatim — so a key with teeth nobody can find is a key nobody will set.\n"+
			"  Two ways out: document it there as a commented `# %s:` line in the neighbouring block (and if it carries a VALUE, the documented-default\n"+
			"  census in seedconfig_test.go will then hold that value against the constant, which is the point); or add it to seedKeyUndocumented with\n"+
			"  the reason the line must not exist. Undocumented and unregistered is the one answer this census does not take.",
			s.where, s.key, s.fn, s.callee, s.how, s.key)
	}
}

// Every config read whose key this census cannot name has a row saying what
// the key set is — and every row describes a site that is still there.
func TestEveryDynamicConfigKeySiteIsAccountedFor(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)
	c := skcConfigKeyCensus(t, root)
	skcWitness(t, c)

	live := map[string]string{}
	for _, d := range c.dynamic {
		live[d.site] = d.where
		why := strings.TrimSpace(seedKeyDynamicSite[d.site])
		if why == "" {
			t.Errorf("%s reads the instance config with a key this census cannot name: %s.\n"+
				"  So the key it reads is outside the documented-or-registered question entirely. Either give the read a literal, a package string constant or a\n"+
				"  `range` over a []string of literals — any of the three and this census names the key itself — or add `%q` to seedKeyDynamicSite with what the\n"+
				"  key set actually is and where the census reaches those keys instead. A redirection is a row; silence is a hole.",
				d.where, d.site, d.site)
			continue
		}
		t.Logf("%s: %s — %s", d.where, d.site, why)
	}
	for _, site := range seedSortedKeys(seedKeyDynamicSite) {
		if strings.TrimSpace(seedKeyDynamicSite[site]) == "" {
			t.Errorf("seedKeyDynamicSite[%q] carries no reason — the row is what stands in for a key this census cannot name, so the why IS the row", site)
		}
		if _, ok := live[site]; !ok {
			t.Errorf("seedKeyDynamicSite names %q, and no config read in the tree is spelled that way any more — the read was given a literal key (drop the row, the census names the key now), moved, or is gone", site)
		}
	}
}

// The undocumented register's other stale half: a row for a key the harness
// no longer reads.
//
// Separate from the pin above, which catches the row whose key the seed has
// since DOCUMENTED. Two ways for one row to stop describing anything, and
// they fail in opposite directions.
func TestConfigKeyUndocumentedRegisterIsNotStale(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)
	c := skcConfigKeyCensus(t, root)
	skcWitness(t, c)

	read := map[string][]string{}
	for _, s := range c.keys {
		read[s.key] = append(read[s.key], s.where)
	}
	for _, key := range seedSortedKeys(seedKeyUndocumented) {
		if strings.TrimSpace(seedKeyUndocumented[key]) == "" {
			t.Errorf("seedKeyUndocumented[%q] carries no reason — a row here says an undocumented key is undocumented on purpose, so the why IS the row", key)
		}
		if len(read[key]) == 0 {
			t.Errorf("seedKeyUndocumented names %q, which no non-test Go file reads from the instance config any more — the key is retired, or its reader moved somewhere this census cannot see. Either way the row now excuses nothing; drop it", key)
			continue
		}
		t.Logf("%s: read at %s", key, strings.Join(read[key], ", "))
	}
}

// skcWitness is the positive half every derived census owes: a walk that
// found nothing passes every assertion above in silence.
//
// The floors are the tree as it stands, and each is a different way for the
// derivation to go quiet — no files, no readers, no keys. They are FLOORS and
// not equalities on purpose: a bead that adds a config key should not have to
// edit a number here, and a bead that REMOVES the census's reach has to.
func skcWitness(t *testing.T, c skcCensus) {
	t.Helper()
	if c.scanned < 20 {
		t.Fatalf("parsed %d non-test .go files — the walk found no tree, so everything below measures nothing", c.scanned)
	}
	// Six, and derived from yamlflat.go: YamlGet, YamlKeysWithPrefix,
	// YamlList, YamlMapPairs, YamlMapPairsRaw, yamlHasKey. A tree with fewer
	// has moved the flat-YAML subset somewhere this rule cannot read.
	if len(c.flat) < 6 {
		t.Fatalf("derived %d flat-YAML readers from yamlflat.go (%v), want at least 6 — the `(path, key)` signature rule found almost nothing, so the scope rule has nothing to apply to", len(c.flat), c.flat)
	}
	// CfgGet by name, because it is the one the whole fixpoint rests on: it
	// is derived from its own body, so a tree where it is missing is a tree
	// where the fixpoint found no seed and every CfgGet key in the harness is
	// invisible.
	if _, ok := c.readers["CfgGet"]; !ok {
		t.Fatalf("the config-reader fixpoint did not find CfgGet (found %v) — every key read through it is then outside this census, which is most of them", seedSortedKeys(c.readers))
	}
	if len(c.readers) < 5 {
		t.Fatalf("derived %d config readers (%v), want at least 5 — attnAge, budgetDollars, graceAfter and planPercent each hold keys that appear nowhere else, so a shorter list means the parameter hop stopped working", len(c.readers), seedSortedKeys(c.readers))
	}
	// Seventy, against 79 distinct keys MEASURED 2026-11-21. Low enough that
	// retiring a family does not red it and high enough that a derivation
	// which stopped resolving constants, ranges or the parameter hop does.
	distinct := map[string]bool{}
	for _, s := range c.keys {
		distinct[s.key] = true
	}
	if len(distinct) < 70 {
		t.Fatalf("derived %d distinct config keys from %d sites, want at least 70 — the census stopped reading the tree", len(distinct), len(c.keys))
	}
	t.Logf("%d non-test .go files, %d flat readers, %d config readers (%s), %d sites over %d distinct keys, %d dynamic sites",
		c.scanned, len(c.flat), len(c.readers), strings.Join(seedSortedKeys(c.readers), " "), len(c.keys), len(distinct), len(c.dynamic))
}

// The census has to be able to say no, or the three pins above are a spelling
// exercise. Driven over PLANTED sources rather than the tree, so it is not
// itself a tree-wide pin and needs no door.
//
// Every arm is one of the derivation's four parts, stated as the thing that
// would go wrong if the part were missing — including the two holes the head
// comment admits, which are pinned here as holes so that closing one is a
// deliberate edit and not a surprise.
func TestConfigKeyCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A go.mod is not needed — the census parses files, it does not build —
	// but the planted tree has to look like the real one to the walk.
	plant := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// yamlflat.go's shape, so part 1 derives the readers from it — BODIES
	// and not signatures, so these carry the real file's hops: `readLines`
	// to `os.ReadFile`, each reader to `readLines`, and `YamlMapPairs` one
	// further out through `YamlMapPairsRaw`, which is the member the
	// transitive half of the rule exists for.
	//
	// Four non-readers beside them, one for each way part 1 must say no: the
	// `(lines, key)` helpers every real reader delegates to, a one-argument
	// function, a `(string, string)` function that never touches the file
	// (`flatFieldRefusal`'s shape — this is what the deleted name test used
	// to exclude, and what the body rule must exclude on its own), and
	// `YamlGetAlt`, a reader whose first parameter is named anything but
	// `path`, which the name-keyed rule derived as nothing at all
	// (ranger-base-3weac).
	plant("yamlflat.go", `package p

import "os"

func readLines(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return []string{string(b)}
}

func YamlGet(path, key string) string    { return yamlGetLines(readLines(path), key) }
func YamlGetAlt(file, key string) string { return yamlGetLines(readLines(file), key) }
func YamlList(path, key string) []string { return yamlListLines(readLines(path), key) }

func YamlMapPairsRaw(path, mapkey string) [][3]string {
	for _, ln := range readLines(path) {
		_ = ln
		_ = mapkey
	}
	return nil
}

func YamlMapPairs(path, mapkey string) [][2]string {
	_ = YamlMapPairsRaw(path, mapkey)
	return nil
}

func YamlKeysWithPrefix(path, prefix string) []string {
	_ = readLines(path)
	_ = prefix
	return nil
}

func yamlHasKey(path, key string) bool {
	_ = readLines(path)
	_ = key
	return false
}

func yamlGetLines(lines []string, key string) string    { return "" }
func yamlListLines(lines []string, key string) []string { return nil }
func flatFieldRefusal(key, v string) string             { return key + v }
`)
	// Padding, so the 20-file floor in skcWitness is met by a planted tree
	// too: a fence that can only be exercised against the real repo is a
	// fence nobody can plant drift against.
	for i := 0; i < 24; i++ {
		plant("pad"+strconv.Itoa(i)+".go", "package p\n")
	}
	plant("app.go", `package p

type App struct{ ConfigPath string }

const PlantedConstKey = "planted_const"

var plantedList = []string{"planted_listed_one", "planted_listed_two"}

// The fixpoint's seed shape: a reader derived from its own body.
func (a *App) CfgGet(key, def string) string {
	if v := YamlGet(a.ConfigPath, key); v != "" {
		return v
	}
	return def
}

// One hop further out, the attnAge shape.
func (a *App) plantedAge(key string, def int) int {
	_ = YamlGet(a.ConfigPath, key)
	return def
}

// Read through a flat reader whose first parameter is named "file" and not
// "path": the escape ranger-base-3weac measured, now a key the census owes
// an answer about.
func (a *App) PlantedAlt() string { return YamlGetAlt(a.ConfigPath, "planted_alt_named") }

func (a *App) PlantedLiteral() string  { return a.CfgGet("planted_literal", "") }
func (a *App) PlantedHop() int         { return a.plantedAge("planted_hopped", 1) }
func (a *App) PlantedConst() string    { return YamlGet(a.ConfigPath, PlantedConstKey) }
func (a *App) PlantedPrefix() []string { return YamlKeysWithPrefix(a.ConfigPath, "planted_prefix_") }

func (a *App) PlantedRanged() bool {
	for _, k := range plantedList {
		if yamlHasKey(a.ConfigPath, k) {
			return true
		}
	}
	return false
}

func (a *App) PlantedInline() bool {
	for _, k := range []string{"planted_inline_one", "planted_inline_two"} {
		if yamlHasKey(a.ConfigPath, k) {
			return true
		}
	}
	return false
}

// A closure, whose name collides with a closure in the next function.
func (a *App) PlantedClosure() {
	read := func(key string) { _ = YamlMapPairs(a.ConfigPath, key) }
	read("planted_closure")
}

// NOT the instance config: the same six readers over some other flat-YAML
// file. Every key here must stay out of the census.
func plantedOtherFile(p string, rt struct{ Path string }) {
	_ = YamlGet(p, "planted_other_param")
	_ = YamlGet(rt.Path, "planted_other_field")
	read := func(k string) { _ = YamlGet(p, k) }
	read("planted_other_closure")
}

// The admitted hole: the instance config reached through a PARAMETER.
func PlantedConfigByParam(cfg string) string { return YamlGet(cfg, "planted_by_param") }

// A key this census cannot name.
func (a *App) PlantedDynamic(n string) string { return a.CfgGet("planted_cap_"+n, "") }
`)

	c := skcConfigKeyCensus(t, dir)

	// The exact set, and not a count: each name is a different way part 1
	// can go wrong, and each absence from it is a different way it can go
	// wide. `YamlMapPairs` is the transitive hop, `YamlGetAlt` is the
	// parameter-name escape, and the four omitted are the four shapes that
	// must stay out — `readLines` (one parameter, so no key slot),
	// `flatFieldRefusal` (two strings, never opens the file), and the two
	// `(lines, key)` helpers.
	wantFlat := []string{
		"YamlGet", "YamlGetAlt", "YamlKeysWithPrefix", "YamlList",
		"YamlMapPairs", "YamlMapPairsRaw", "yamlHasKey",
	}
	if got := seedSortedKeys(c.flat); !reflect.DeepEqual(got, wantFlat) {
		t.Errorf("part 1 derived %v from the planted yamlflat.go, want exactly %v — the body rule either missed a reader (`YamlMapPairs` is only reachable through `YamlMapPairsRaw`; `YamlGetAlt` is only reachable if the rule is not keyed on the parameter being named `path`) or read a non-reader as one (`flatFieldRefusal` is two strings and opens nothing)", got, wantFlat)
	}
	for _, want := range []string{"CfgGet", "plantedAge"} {
		if _, ok := c.readers[want]; !ok {
			t.Errorf("part 3's fixpoint did not derive %s as a config reader (found %v) — the parameter hop is what makes every attn_*-shaped key visible", want, seedSortedKeys(c.readers))
		}
	}
	if _, ok := c.readers["plantedOtherFile"]; ok {
		t.Error("part 3 derived plantedOtherFile as a config reader — its YamlGet takes a PARAMETER as the path, so part 2's scope rule must stop the hop there; as it stands every runtime-adapter and session-record key in the tree would need a register row")
	}

	got := map[string]skcKeySite{}
	for _, s := range c.keys {
		got[s.key] = s
	}
	for _, want := range []struct{ key, why string }{
		{"planted_literal", "a literal handed to CfgGet"},
		{"planted_alt_named", "a literal handed to a flat reader whose first parameter is named `file` — the escape ranger-base-3weac measured, which the parameter-name rule could not see at all"},
		{"planted_hopped", "a literal handed to a derived reader one hop out"},
		{"planted_const", "a package-level string constant"},
		{"planted_prefix_", "a prefix read"},
		{"planted_listed_one", "a range over a package-level []string of literals"},
		{"planted_listed_two", "the second member of that list, which a partial read would drop"},
		{"planted_inline_one", "a range over an inline []string of literals"},
		{"planted_inline_two", "the second member of that inline list"},
		{"planted_closure", "a literal handed to an assigned func literal that reads the config"},
	} {
		if _, ok := got[want.key]; !ok {
			t.Errorf("the census missed %s (%s) — it found %v", want.key, want.why, seedSortedKeys(got))
		}
	}
	if s, ok := got["planted_prefix_"]; ok && !s.prefix {
		t.Error("the census read planted_prefix_ as a whole key rather than a prefix — the family question is then asked as if `planted_prefix_:` were a line an operator would write")
	}
	for _, unwanted := range []struct{ key, why string }{
		{"planted_other_param", "read from a path PARAMETER, not from .ConfigPath"},
		{"planted_other_field", "read from rt.Path — a runtime adapter file"},
		{"planted_other_closure", "read through a closure over a path parameter"},
		{"planted_by_param", "the admitted hole: the config reached through a parameter. If this now FAILS, part 2 has been widened — good, and this row is what has to be deleted deliberately rather than discovered"},
		{"key", "a reader's own parameter name, which is not a key"},
		{"def", "a reader's default argument"},
	} {
		if s, ok := got[unwanted.key]; ok {
			t.Errorf("the census read %q as an instance-config key (%s) — %s", unwanted.key, s.where, unwanted.why)
		}
	}

	dyn := map[string]bool{}
	for _, d := range c.dynamic {
		dyn[d.site] = true
	}
	if !dyn[`PlantedDynamic("planted_cap_" + n)`] {
		var sites []string
		for _, d := range c.dynamic {
			sites = append(sites, d.site)
		}
		t.Errorf("the census did not report the assembled key as a dynamic site — it reported %v. A key it cannot name and does not report is a key outside the whole question", sites)
	}
	for _, s := range c.dynamic {
		if strings.HasPrefix(s.site, "CfgGet(") || strings.HasPrefix(s.site, "plantedAge(") {
			t.Errorf("the census reported a reader's own body as a dynamic site (%s) — part 3 already walks that reader's call sites, so the row would be a register entry for every shared reader in the tree", s.site)
		}
	}
}

// And the seed-side matchers have to be able to say no too: the line rule is
// what decides whether a key is documented, and a rule that matched prose
// would pass a key nobody can set.
func TestSeedKeyLineMatcherCanStillSayNo(t *testing.T) {
	t.Parallel()
	const text = `live_key: on
# commented_key: 5m
#   indented_comment_key: 7h
# a sentence about ` + "`backticked_key:`" + ` that is prose and not a line
#   - listed_item: not a top-level key
#  planted_prefix_5h: 70
# planted_prefix_: bare
`
	for _, c := range []struct {
		key  string
		want bool
		why  string
	}{
		{"live_key", true, "a live key is on the surface"},
		{"commented_key", true, "the form every key with teeth ships as"},
		{"indented_comment_key", true, "blanks after the `#` are how the seed indents a continuation"},
		{"backticked_key", false, "a backticked mention is prose; the seed discusses retired keys and other files' keys that way"},
		{"listed_item", false, "an indented list item under a commented key is not a key line"},
		{"absent_key", false, "the whole point"},
	} {
		if got := skcSeedDocumentsKey(text, c.key); got != c.want {
			t.Errorf("skcSeedDocumentsKey(%q) = %v, want %v — %s", c.key, got, c.want, c.why)
		}
	}
	// The prefix matcher answers about MEMBERS, and a bare prefix line is
	// not one: `# planted_prefix_:` documents the family's spelling and no
	// window, which is what the `>` and not `>=` in that rule is for.
	if members := skcSeedDocumentsPrefix(text, "planted_prefix_"); len(members) != 1 || members[0] != "planted_prefix_5h" {
		t.Errorf("skcSeedDocumentsPrefix found %v, want exactly [planted_prefix_5h] — a bare `planted_prefix_:` line names no window, and a prose mention is not a line", members)
	}
	if members := skcSeedDocumentsPrefix(text, "absent_prefix_"); len(members) != 0 {
		t.Errorf("skcSeedDocumentsPrefix answered %v for a prefix the text does not carry", members)
	}
}
