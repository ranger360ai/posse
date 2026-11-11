package posse

import (
	"path/filepath"
	"strings"
)

// The --mcp-config half of ADR 0090 D1. ClaudeStrictMCPFlag takes every MCP
// source away from a claude seat except the files its own line names with
// --mcp-config. That leaves one way back in: a line naming a file the seat
// can itself rewrite, which is the class D1 closes with a different spelling.
// So the launch refuses such a line, at whichever mechanism puts up the wall,
// checked against that mechanism's OWN write set — the sandbox-exec profile's
// grants and atomic-write siblings, or bwrap's bound set after D19/D20's
// substitutions — because the two sets differ (claude's config FILE on one,
// its config DIRECTORY on the other) and a check against the wrong one would
// pass a file the wall in force lets the seat write.

// MCPConfigWallRefusal is that refusal for the mechanism this host renders.
// nil when the line names no --mcp-config, or every file it names is outside
// the seat's write set.
func (a *App) MCPConfigWallRefusal(ag *AgentFile, rt *Runtime, cwd, line string) error {
	return a.mcpConfigWallRefusalOn(WallMechanism(), ag, rt, cwd, line)
}

// mcpConfigWallRefusalOn names the mechanism, so a pin can drive the arm this
// host does not have (renderWallOn's rule).
func (a *App) mcpConfigWallRefusalOn(mech string, ag *AgentFile, rt *Runtime, cwd, line string) error {
	files, unverifiable := MCPConfigFiles(line)
	if len(files) == 0 && len(unverifiable) == 0 {
		return nil
	}
	if len(unverifiable) > 0 {
		return Die("%s: the launch line passes %s %q, which posse cannot resolve to a file without running a shell — so it cannot check that a caged seat is unable to rewrite it (ADR 0090 D1)\n"+
			"  spell the path literally, or inline the JSON", ag.Name, ClaudeMCPConfigFlag, unverifiable[0])
	}
	writable, siblings, deny, keep, err := a.mcpWriteSet(mech, ag, rt, cwd)
	if err != nil {
		return err
	}
	for _, f := range files {
		p := ExpandTilde(f)
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		if mcpPathWritable(p, writable, siblings, deny, keep) {
			return Die("%s: the launch line passes %s %s, which is inside this seat's write set at %s — the seat could rewrite the server list it starts, and an MCP server is a command (ADR 0090 D1)\n"+
				"  move that file outside every path `posse gates %s` prints as writable", ag.Name, ClaudeMCPConfigFlag, f, mech, ag.Name)
		}
	}
	return nil
}

// mcpWriteSet is the seat's write set as the named mechanism grants it: the
// grants, the sibling bases (seatbelt only), and the carve-out's deny and keep
// lists, which narrow and re-widen those grants.
func (a *App) mcpWriteSet(mech string, ag *AgentFile, rt *Runtime, cwd string) (writable, siblings, deny, keep []string, err error) {
	gatesDir := a.GatesDir(ag.Name)
	if mech == WallBwrap {
		r, err := a.bwrapRender(ag, rt, cwd, gatesDir)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return r.Set, nil, r.Carve.Deny, append(append([]string{}, r.Keep...), r.Carve.Keep...), nil
	}
	stateDirs := SeatbeltStateDirs(rt)
	writable = a.SeatbeltWritable(ag, cwd, gatesDir, stateDirs...)
	carve := a.SeatbeltCarveOut(ag, cwd, gatesDir, writable, stateDirs...)
	return writable, SeatbeltSiblings(stateDirs, carve), carve.Deny, carve.Keep, nil
}

// mcpPathWritable asks the question for BOTH spellings of p — as typed and
// with its links resolved — and refuses if either is writable. The resolved
// one catches a link outside the set that points in; the literal one catches
// a link INSIDE the set that points out, which the seat could repoint after
// the check.
func mcpPathWritable(p string, writable, siblings, deny, keep []string) bool {
	for _, q := range dedupeStrings([]string{filepath.Clean(p), absResolve(p)}) {
		granted := false
		for _, w := range writable {
			if insideDir(filepath.Clean(w), q) || insideDir(absResolve(w), q) {
				granted = true
				break
			}
		}
		for _, b := range siblings {
			if strings.HasPrefix(q, b+".") || strings.HasPrefix(q, filepath.Clean(b)+".") {
				granted = true
			}
		}
		if !granted {
			continue
		}
		denied := false
		for _, d := range deny {
			if insideDir(filepath.Clean(d), q) || insideDir(absResolve(d), q) {
				denied = true
				break
			}
		}
		for _, k := range keep {
			if filepath.Clean(k) == q || absResolve(k) == q {
				denied = false
			}
		}
		if !denied {
			return true
		}
	}
	return false
}
