package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// offerCopySource interactively picks a copy source; empty result means
// start clean.
func (a *App) offerCopySource(newName string) string {
	var sources []string
	profiles, err := a.Store.List()
	if err == nil {
		for _, p := range profiles {
			if p.Name != newName {
				sources = append(sources, p.Name)
			}
		}
	}
	if _, ok := profile.DefaultConfigDir(); ok {
		sources = append(sources, "default")
	}
	if len(sources) == 0 {
		return ""
	}
	options := make([]string, 0, len(sources)+1)
	options = append(options, "Start clean — don't copy anything")
	for _, s := range sources {
		if s == "default" {
			options = append(options, "default (your current ~/.claude setup)")
		} else {
			options = append(options, s)
		}
	}
	idx, err := a.selectFrom("Copy configuration into the new profile?", options)
	if err != nil || idx <= 0 {
		return ""
	}
	return sources[idx-1]
}

// copyInto copies configuration from a source profile (or "default" for
// ~/.claude) into p. Interactively the user picks everything, individual
// categories/items, or nothing; non-interactive callers get everything
// shareable. Credentials and history are never copied (internal/profile
// enforces this; see docs/copy.md).
func (a *App) copyInto(p profile.Profile, from string) error {
	srcDir, stateFile, err := a.copySource(from)
	if err != nil {
		return err
	}
	sel := profile.EverythingSelection()
	if a.Interactive {
		var proceed bool
		sel, proceed = a.chooseCopyMode(from, srcDir, stateFile)
		if !proceed {
			a.printf("Starting clean — nothing copied.\n")
			return nil
		}
	}
	n, err := p.CopyFrom(srcDir, stateFile, sel)
	if err != nil {
		return err
	}
	a.printf("Copied %d item(s) from %s (credentials and history are never copied).\n", n, srcDir)
	return nil
}

// copySource resolves a copy source name to its config directory and its
// state file. Claude Code keeps the default setup's state file at
// ~/.claude.json, outside ~/.claude.
func (a *App) copySource(from string) (srcDir, stateFile string, err error) {
	if from == "default" {
		dir, ok := profile.DefaultConfigDir()
		if !ok {
			return "", "", fmt.Errorf("no default Claude Code config found at ~/.claude")
		}
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", "", fmt.Errorf("cannot determine home directory: %w", herr)
		}
		return dir, filepath.Join(home, ".claude.json"), nil
	}
	if !a.Store.Exists(from) {
		return "", "", fmt.Errorf("source profile %q does not exist", from)
	}
	dir := a.Store.Dir(from)
	return dir, filepath.Join(dir, ".claude.json"), nil
}

// chooseCopyMode asks how much of the source to copy. proceed is false
// when the user wants to start clean (or cancelled).
func (a *App) chooseCopyMode(from, srcDir, stateFile string) (sel profile.Selection, proceed bool) {
	idx, err := a.selectFrom(
		fmt.Sprintf("Copy configuration from '%s' into the new profile?", from),
		[]string{
			"Copy everything (all shareable config — credentials and history are never copied)",
			"Choose what to copy",
			"Start clean — don't copy anything",
		})
	if err != nil || idx < 0 || idx == 2 {
		return nil, false
	}
	if idx == 0 {
		return profile.EverythingSelection(), true
	}
	return a.chooseCategories(srcDir, stateFile), true
}

// chooseCategories walks the categories present in the source, one
// prompt each: yes/no for single files, all/choose/skip for collections.
func (a *App) chooseCategories(srcDir, stateFile string) profile.Selection {
	inv := profile.TakeInventory(srcDir, stateFile)
	sel := profile.Selection{}
	if inv.HasSettings && a.confirm("Copy settings (settings.json)?") {
		sel[profile.CatSettings] = profile.Pick{All: true}
	}
	if inv.HasClaudeMD && a.confirm("Copy CLAUDE.md?") {
		sel[profile.CatClaudeMD] = profile.Pick{All: true}
	}
	a.pickCategory(sel, profile.CatSkills, "skills", inv.Skills)
	a.pickCategory(sel, profile.CatAgents, "agents", inv.Agents)
	a.pickCategory(sel, profile.CatCommands, "commands", inv.Commands)
	if inv.HasPrefs && a.confirm("Copy preferences & onboarding state (skips first-run setup)?") {
		sel[profile.CatPrefs] = profile.Pick{All: true}
	}
	a.pickCategory(sel, profile.CatProjects, "folder trust & permissions", inv.Projects)
	a.pickCategory(sel, profile.CatMCP, "MCP servers", inv.MCPServers)
	return sel
}

// pickCategory runs one all/choose/skip prompt, drilling into a
// multi-select for "choose". Empty categories are skipped silently.
func (a *App) pickCategory(sel profile.Selection, cat profile.Category, label string, items []string) {
	if len(items) == 0 {
		return
	}
	idx, err := a.selectFrom(
		fmt.Sprintf("Copy %s? (%d found)", label, len(items)),
		[]string{"Copy all", "Choose individually", "Skip"})
	if err != nil || idx != 0 && idx != 1 {
		return
	}
	if idx == 0 {
		sel[cat] = profile.Pick{All: true}
		return
	}
	chosen, err := a.multiSelect(fmt.Sprintf("Select %s to copy", label), items)
	if err != nil || len(chosen) == 0 {
		return
	}
	names := make([]string, 0, len(chosen))
	for _, i := range chosen {
		names = append(names, items[i])
	}
	sel[cat] = profile.Pick{Names: names}
}
