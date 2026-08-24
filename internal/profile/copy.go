package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// Inventory lists what a copy source actually contains, so prompts can
// show only real choices. Collection fields are sorted.
type Inventory struct {
	HasSettings bool
	HasClaudeMD bool
	HasPrefs    bool
	Skills      []string
	Agents      []string
	Commands    []string
	Projects    []string
	MCPServers  []string
}

// TakeInventory inspects a source config directory and its state file.
// A missing or unparseable state file yields an empty state inventory;
// the file items are reported regardless.
func TakeInventory(srcDir, stateFile string) Inventory {
	inv := Inventory{
		HasSettings: fileExists(filepath.Join(srcDir, "settings.json")),
		HasClaudeMD: fileExists(filepath.Join(srcDir, "CLAUDE.md")),
		Skills:      dirEntries(filepath.Join(srcDir, "skills")),
		Agents:      dirEntries(filepath.Join(srcDir, "agents")),
		Commands:    dirEntries(filepath.Join(srcDir, "commands")),
	}
	raw, err := os.ReadFile(stateFile)
	if err != nil {
		return inv
	}
	var src map[string]any
	if err := json.Unmarshal(raw, &src); err != nil {
		return inv
	}
	for k := range src {
		if k != "projects" && k != "mcpServers" && !slices.Contains(deniedTopLevel, k) {
			inv.HasPrefs = true
			break
		}
	}
	servers := map[string]bool{}
	for name := range toMap(src["mcpServers"]) {
		servers[name] = true
	}
	for path, v := range toMap(src["projects"]) {
		entry := toMap(v)
		for name := range toMap(entry["mcpServers"]) {
			servers[name] = true
		}
		for k := range entry {
			if k != "mcpServers" && !slices.Contains(deniedProject, k) {
				inv.Projects = append(inv.Projects, path)
				break
			}
		}
	}
	for name := range servers {
		inv.MCPServers = append(inv.MCPServers, name)
	}
	slices.Sort(inv.Projects)
	slices.Sort(inv.MCPServers)
	return inv
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// dirEntries lists the names inside dir, sorted; nil when dir is absent.
func dirEntries(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	return names
}

// fileItems maps file-backed categories onto the entry they copy.
var fileItems = []struct {
	cat  Category
	name string
	dir  bool
}{
	{CatSettings, "settings.json", false},
	{CatClaudeMD, "CLAUDE.md", false},
	{CatSkills, "skills", true},
	{CatAgents, "agents", true},
	{CatCommands, "commands", true},
}

// CopyFrom copies the selected configuration from srcDir (and its state
// file, which lives outside srcDir for the default ~/.claude source) into
// the profile. Credentials and history are never copied: file items are an
// allow-list, and the state file passes through filterState's deny list.
// It returns the number of top-level items copied.
func (p Profile) CopyFrom(srcDir, stateFile string, sel Selection) (int, error) {
	copied := 0
	for _, item := range fileItems {
		pick, ok := sel[item.cat]
		if !ok {
			continue
		}
		src := filepath.Join(srcDir, item.name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		switch {
		case pick.All:
			if err := copyRecursive(src, filepath.Join(p.Dir, item.name)); err != nil {
				return copied, fmt.Errorf("copying %s: %w", item.name, err)
			}
			copied++
		case item.dir && len(pick.Names) > 0:
			n := 0
			for _, child := range pick.Names {
				// Names come from dir listings, but never trust them to
				// stay inside the profile.
				if child != filepath.Base(child) {
					continue
				}
				childSrc := filepath.Join(src, child)
				if _, err := os.Lstat(childSrc); err != nil {
					continue
				}
				if err := copyRecursive(childSrc, filepath.Join(p.Dir, item.name, child)); err != nil {
					return copied, fmt.Errorf("copying %s: %w", filepath.Join(item.name, child), err)
				}
				n++
			}
			if n > 0 {
				copied++
			}
		}
	}
	n, err := p.copyState(stateFile, sel)
	if err != nil {
		return copied, err
	}
	return copied + n, nil
}

// copyState writes the filtered state file into the profile. A missing or
// unparseable source state file is tolerated: the copy proceeds without it.
func (p Profile) copyState(stateFile string, sel Selection) (int, error) {
	raw, err := os.ReadFile(stateFile)
	if err != nil {
		return 0, nil
	}
	filtered, ferr := filterState(raw, sel)
	if ferr != nil || filtered == nil {
		return 0, nil
	}
	if err := os.WriteFile(filepath.Join(p.Dir, ".claude.json"), filtered, 0o600); err != nil {
		return 0, fmt.Errorf("writing state file: %w", err)
	}
	return 1, nil
}
