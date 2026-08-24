package profile

import (
	"encoding/json"
	"slices"
)

// Category identifies one copyable slice of a profile.
type Category int

// Category constants identify each copyable slice of a profile.
const (
	CatSettings Category = iota // settings.json
	CatClaudeMD                 // CLAUDE.md
	CatSkills                   // skills/ (items: entries of the directory)
	CatAgents                   // agents/
	CatCommands                 // commands/
	CatPrefs                    // .claude.json: top-level preferences & onboarding
	CatProjects                 // .claude.json: per-folder trust & permissions
	CatMCP                      // .claude.json: MCP servers, top-level and per-project
)

// Pick says how much of one category to copy. The zero value means "none";
// yes/no categories (settings, CLAUDE.md, prefs) only use All.
type Pick struct {
	All   bool
	Names []string
}

func (p Pick) has(name string) bool {
	return slices.Contains(p.Names, name)
}

// Selection maps each category to what should be copied; an absent
// category is skipped entirely.
type Selection map[Category]Pick

// EverythingSelection selects all shareable configuration. The deny lists
// below are excluded from every selection, including this one.
func EverythingSelection() Selection {
	sel := Selection{}
	for _, c := range []Category{CatSettings, CatClaudeMD, CatSkills, CatAgents, CatCommands, CatPrefs, CatProjects, CatMCP} {
		sel[c] = Pick{All: true}
	}
	return sel
}

// deniedTopLevel and deniedProject are stripped from the state file
// unconditionally: account identity and conversation history are never
// shareable between profiles (see docs/copy.md).
var (
	deniedTopLevel = []string{"oauthAccount", "userID"}
	deniedProject  = []string{"history", "lastSessionId"}
)

// filterState builds the destination .claude.json from a source state
// file, keeping only what sel asks for and never the denied keys. It
// returns nil when nothing is selected or nothing survives filtering.
func filterState(raw []byte, sel Selection) ([]byte, error) {
	var src map[string]any
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, err
	}
	out := map[string]any{}

	if sel[CatPrefs].All {
		for k, v := range src {
			if k == "projects" || k == "mcpServers" || slices.Contains(deniedTopLevel, k) {
				continue
			}
			out[k] = v
		}
	}

	mcp, wantMCP := sel[CatMCP]
	if wantMCP {
		if servers := filterServers(src["mcpServers"], mcp); len(servers) > 0 {
			out["mcpServers"] = servers
		}
	}

	outProjects := map[string]any{}
	for path, v := range toMap(src["projects"]) {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		dst := map[string]any{}
		if pp := sel[CatProjects]; pp.All || pp.has(path) {
			for k, val := range entry {
				if k == "mcpServers" || slices.Contains(deniedProject, k) {
					continue
				}
				dst[k] = val
			}
		}
		if wantMCP {
			if servers := filterServers(entry["mcpServers"], mcp); len(servers) > 0 {
				dst["mcpServers"] = servers
			}
		}
		if len(dst) > 0 {
			outProjects[path] = dst
		}
	}
	if len(outProjects) > 0 {
		out["projects"] = outProjects
	}

	if len(out) == 0 {
		return nil, nil
	}
	return json.MarshalIndent(out, "", "  ")
}

// filterServers returns the entries of an mcpServers map selected by pick.
func filterServers(v any, pick Pick) map[string]any {
	out := map[string]any{}
	for name, cfg := range toMap(v) {
		if pick.All || pick.has(name) {
			out[name] = cfg
		}
	}
	return out
}

func toMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
