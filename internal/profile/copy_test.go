package profile

import (
	"encoding/json"
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
