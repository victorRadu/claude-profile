package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// stateFixture is a realistic .claude.json holding shareable state next to
// everything that must never cross profiles.
const stateFixture = `{
	"hasCompletedOnboarding": true,
	"lastOnboardingVersion": "2.0.1",
	"theme": "dark",
	"numStartups": 42,
	"oauthAccount": {"emailAddress": "a@b.c", "accountUuid": "u-1"},
	"userID": "deadbeef",
	"mcpServers": {"global-search": {"command": "srv"}},
	"projects": {
		"/home/u/alpha": {
			"hasTrustDialogAccepted": true,
			"allowedTools": ["Bash"],
			"history": [{"display": "old chat"}],
			"lastSessionId": "sess-1",
			"mcpServers": {"alpha-db": {"command": "db"}}
		},
		"/home/u/beta": {
			"hasTrustDialogAccepted": true,
			"mcpServers": {"beta-lint": {"command": "lint"}}
		}
	}
}`

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("filtered output is not valid JSON: %v", err)
	}
	return m
}

func TestFilterStateEverything(t *testing.T) {
	out, err := filterState([]byte(stateFixture), EverythingSelection())
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, out)

	for _, denied := range []string{"oauthAccount", "userID"} {
		if _, ok := got[denied]; ok {
			t.Errorf("%s must never be copied", denied)
		}
	}
	for _, want := range []string{"hasCompletedOnboarding", "lastOnboardingVersion", "theme", "numStartups", "mcpServers"} {
		if _, ok := got[want]; !ok {
			t.Errorf("top-level %s missing from everything-copy", want)
		}
	}
	alpha := got["projects"].(map[string]any)["/home/u/alpha"].(map[string]any)
	for _, denied := range []string{"history", "lastSessionId"} {
		if _, ok := alpha[denied]; ok {
			t.Errorf("project %s must never be copied", denied)
		}
	}
	for _, want := range []string{"hasTrustDialogAccepted", "allowedTools", "mcpServers"} {
		if _, ok := alpha[want]; !ok {
			t.Errorf("project key %s missing from everything-copy", want)
		}
	}
}

func TestFilterStatePrefsOnly(t *testing.T) {
	out, err := filterState([]byte(stateFixture), Selection{CatPrefs: {All: true}})
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, out)
	if _, ok := got["projects"]; ok {
		t.Error("prefs-only selection must not include projects")
	}
	if _, ok := got["mcpServers"]; ok {
		t.Error("prefs-only selection must not include mcpServers")
	}
	if _, ok := got["theme"]; !ok {
		t.Error("prefs-only selection lost theme")
	}
	if _, ok := got["oauthAccount"]; ok {
		t.Error("oauthAccount leaked")
	}
}

func TestFilterStateChosenProjects(t *testing.T) {
	out, err := filterState([]byte(stateFixture), Selection{
		CatProjects: {Names: []string{"/home/u/beta"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, out)
	projects := got["projects"].(map[string]any)
	if _, ok := projects["/home/u/alpha"]; ok {
		t.Error("unchosen project copied")
	}
	beta := projects["/home/u/beta"].(map[string]any)
	if _, ok := beta["hasTrustDialogAccepted"]; !ok {
		t.Error("chosen project lost trust flag")
	}
	if _, ok := beta["mcpServers"]; ok {
		t.Error("mcpServers belongs to CatMCP, not CatProjects")
	}
}

func TestFilterStateChosenServers(t *testing.T) {
	out, err := filterState([]byte(stateFixture), Selection{
		CatMCP: {Names: []string{"global-search", "beta-lint"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, out)
	if _, ok := got["mcpServers"].(map[string]any)["global-search"]; !ok {
		t.Error("chosen top-level server missing")
	}
	projects := got["projects"].(map[string]any)
	if _, ok := projects["/home/u/alpha"]; ok {
		t.Error("project with no chosen servers and no trust selection must not appear")
	}
	beta := projects["/home/u/beta"].(map[string]any)
	if _, ok := beta["hasTrustDialogAccepted"]; ok {
		t.Error("trust flag copied without CatProjects selection")
	}
	if _, ok := beta["mcpServers"].(map[string]any)["beta-lint"]; !ok {
		t.Error("chosen project server missing")
	}
}

func TestFilterStateEmptyAndInvalid(t *testing.T) {
	if out, err := filterState([]byte(stateFixture), Selection{}); err != nil || out != nil {
		t.Errorf("empty selection: got (%s, %v), want (nil, nil)", out, err)
	}
	if _, err := filterState([]byte("not json"), EverythingSelection()); err == nil {
		t.Error("invalid JSON must return an error")
	}
	// A selection that matches nothing must produce no file, not "{}".
	if out, err := filterState([]byte(`{"oauthAccount":{}}`), EverythingSelection()); err != nil || out != nil {
		t.Errorf("nothing survives: got (%s, %v), want (nil, nil)", out, err)
	}
	if !strings.Contains(stateFixture, "history") {
		t.Fatal("fixture must exercise the history deny rule")
	}
}

func TestTakeInventory(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "settings.json"), `{}`)
	mustWrite(t, filepath.Join(src, "skills", "zeta", "SKILL.md"), "s")
	mustWrite(t, filepath.Join(src, "skills", "alpha", "SKILL.md"), "s")
	mustWrite(t, filepath.Join(src, "agents", "helper.md"), "a")
	state := filepath.Join(src, ".claude.json")
	mustWrite(t, state, stateFixture)

	inv := TakeInventory(src, state)
	if !inv.HasSettings {
		t.Error("HasSettings = false, want true")
	}
	if inv.HasClaudeMD {
		t.Error("HasClaudeMD = true for absent CLAUDE.md")
	}
	if !reflect.DeepEqual(inv.Skills, []string{"alpha", "zeta"}) {
		t.Errorf("Skills = %v, want [alpha zeta] sorted", inv.Skills)
	}
	if !reflect.DeepEqual(inv.Agents, []string{"helper.md"}) {
		t.Errorf("Agents = %v", inv.Agents)
	}
	if len(inv.Commands) != 0 {
		t.Errorf("Commands = %v, want empty", inv.Commands)
	}
	if !inv.HasPrefs {
		t.Error("HasPrefs = false, want true (fixture has top-level prefs)")
	}
	if !reflect.DeepEqual(inv.Projects, []string{"/home/u/alpha", "/home/u/beta"}) {
		t.Errorf("Projects = %v", inv.Projects)
	}
	if !reflect.DeepEqual(inv.MCPServers, []string{"alpha-db", "beta-lint", "global-search"}) {
		t.Errorf("MCPServers = %v", inv.MCPServers)
	}
}

func TestTakeInventoryTolerant(t *testing.T) {
	src := t.TempDir()
	inv := TakeInventory(src, filepath.Join(src, "missing.json"))
	if inv.HasPrefs || len(inv.Projects) != 0 || len(inv.MCPServers) != 0 {
		t.Errorf("missing state file must yield empty state inventory: %+v", inv)
	}
	bad := filepath.Join(src, "bad.json")
	if err := os.WriteFile(bad, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	inv = TakeInventory(src, bad)
	if inv.HasPrefs || len(inv.Projects) != 0 {
		t.Errorf("unparseable state file must yield empty state inventory: %+v", inv)
	}
	// A state file where only denied keys exist offers nothing to copy.
	onlyDenied := filepath.Join(src, "denied.json")
	if err := os.WriteFile(onlyDenied, []byte(`{"oauthAccount":{},"userID":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if inv := TakeInventory(src, onlyDenied); inv.HasPrefs {
		t.Error("denied-only state file must not report prefs")
	}
}

func TestCopyFromSelective(t *testing.T) {
	s := newStore(t)
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "settings.json"), `{"a":1}`)
	mustWrite(t, filepath.Join(src, "CLAUDE.md"), "# rules")
	mustWrite(t, filepath.Join(src, "skills", "alpha", "SKILL.md"), "a")
	mustWrite(t, filepath.Join(src, "skills", "beta", "SKILL.md"), "b")
	state := filepath.Join(src, ".claude.json")
	mustWrite(t, state, stateFixture)

	p, err := s.Create("sel")
	if err != nil {
		t.Fatal(err)
	}
	n, err := p.CopyFrom(src, state, Selection{
		CatSettings: {All: true},
		CatSkills:   {Names: []string{"beta", "missing", "../evil"}},
		CatPrefs:    {All: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	// settings.json + skills + .claude.json
	if n != 3 {
		t.Fatalf("copied %d items, want 3", n)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "skills", "beta", "SKILL.md")); err != nil {
		t.Error("chosen skill beta not copied")
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "skills", "alpha")); err == nil {
		t.Error("unchosen skill alpha copied")
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "CLAUDE.md")); err == nil {
		t.Error("unselected CLAUDE.md copied")
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "evil")); err == nil {
		t.Error("path-traversal name escaped the skills directory")
	}
	raw, err := os.ReadFile(filepath.Join(p.Dir, ".claude.json"))
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	got := decode(t, raw)
	if _, ok := got["theme"]; !ok {
		t.Error("prefs missing from copied state")
	}
	if _, ok := got["oauthAccount"]; ok {
		t.Error("oauthAccount leaked into copy")
	}
	if _, ok := got["projects"]; ok {
		t.Error("projects copied without CatProjects selection")
	}
}

func TestCopyFromStateTolerance(t *testing.T) {
	s := newStore(t)
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "settings.json"), `{}`)

	p, err := s.Create("tol")
	if err != nil {
		t.Fatal(err)
	}
	// Missing state file: files still copy, no error, no .claude.json.
	n, err := p.CopyFrom(src, filepath.Join(src, ".claude.json"), EverythingSelection())
	if err != nil || n != 1 {
		t.Fatalf("missing state: got (%d, %v), want (1, nil)", n, err)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, ".claude.json")); err == nil {
		t.Error(".claude.json written from a missing source")
	}
	// Unparseable state file: same tolerance.
	bad := filepath.Join(src, ".claude.json")
	mustWrite(t, bad, "not json")
	p2, err := s.Create("tol2")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := p2.CopyFrom(src, bad, EverythingSelection()); err != nil || n != 1 {
		t.Fatalf("bad state: got (%d, %v), want (1, nil)", n, err)
	}
}
