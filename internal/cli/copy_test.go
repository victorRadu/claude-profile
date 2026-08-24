package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const cliStateFixture = `{
	"hasCompletedOnboarding": true,
	"theme": "dark",
	"oauthAccount": {"emailAddress": "a@b.c"},
	"userID": "deadbeef",
	"projects": {
		"/home/u/alpha": {
			"hasTrustDialogAccepted": true,
			"history": [{"display": "old"}],
			"lastSessionId": "sess-1",
			"mcpServers": {"alpha-db": {"command": "db"}}
		}
	}
}`

func seedSource(t *testing.T, app *App) {
	t.Helper()
	if code := app.Run([]string{"create", "src", "--no-alias"}); code != 0 {
		t.Fatal("create src failed")
	}
	dir := app.Store.Dir("src")
	writeFile(t, filepath.Join(dir, "settings.json"), `{"model":"opus"}`)
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# rules")
	writeFile(t, filepath.Join(dir, "skills", "alpha", "SKILL.md"), "a")
	writeFile(t, filepath.Join(dir, "skills", "beta", "SKILL.md"), "b")
	writeFile(t, filepath.Join(dir, ".claude.json"), cliStateFixture)
}

func readState(t *testing.T, app *App, name string) map[string]any {
	t.Helper()
	raw := readFile(t, filepath.Join(app.Store.Dir(name), ".claude.json"))
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("copied state is not JSON: %v", err)
	}
	return m
}

// Non-interactive --from copies everything shareable, without prompts.
func TestCopyFromFlagNonInteractive(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	seedSource(t, app)

	out.Reset()
	if code := app.Run([]string{"create", "dst", "--from", "src", "--no-alias"}); code != 0 {
		t.Fatalf("create dst failed: %s", out)
	}
	if !strings.Contains(out.String(), "credentials and history are never copied") {
		t.Fatalf("copy summary lost its promise:\n%s", out)
	}
	got := readState(t, app, "dst")
	if got["hasCompletedOnboarding"] != true {
		t.Error("onboarding flag not copied")
	}
	alpha := got["projects"].(map[string]any)["/home/u/alpha"].(map[string]any)
	if alpha["hasTrustDialogAccepted"] != true {
		t.Error("folder trust not copied")
	}
	for _, denied := range []string{"oauthAccount", "userID"} {
		if _, ok := got[denied]; ok {
			t.Errorf("%s leaked into copy", denied)
		}
	}
	for _, denied := range []string{"history", "lastSessionId"} {
		if _, ok := alpha[denied]; ok {
			t.Errorf("project %s leaked into copy", denied)
		}
	}
}

// Interactive flow: pick source, then "Copy everything".
func TestCopyInteractiveEverything(t *testing.T) {
	// Prompts (numbered fallbacks): source menu → 2 (src, after "Start
	// clean"), mode menu → 1 (everything).
	app, out, _, _ := newTestApp(t, "2\n1\n")
	seedSource(t, app)
	app.Interactive = true

	out.Reset()
	if code := app.Run([]string{"create", "dst", "--no-alias"}); code != 0 {
		t.Fatalf("create dst failed: %s", out)
	}
	if _, ok := readState(t, app, "dst")["theme"]; !ok {
		t.Error("everything-copy missed state prefs")
	}
	if !fileExists(filepath.Join(app.Store.Dir("dst"), "skills", "beta", "SKILL.md")) {
		t.Error("everything-copy missed skills")
	}
}

// Interactive flow: choose categories and individual items.
func TestCopyInteractiveChoose(t *testing.T) {
	// Prompts in order (numbered/line fallbacks):
	//   source menu        → "2"  (src)
	//   mode menu          → "2"  (choose what to copy)
	//   settings?          → "y"
	//   CLAUDE.md?         → "n"
	//   skills menu        → "2"  (choose individually)
	//   skills multiselect → "2"  (beta)
	//   prefs?             → "y"
	//   folder trust menu  → "3"  (skip)
	//   MCP menu           → "1"  (copy all)
	app, out, _, _ := newTestApp(t, "2\n2\ny\nn\n2\n2\ny\n3\n1\n")
	seedSource(t, app)
	app.Interactive = true

	out.Reset()
	if code := app.Run([]string{"create", "dst", "--no-alias"}); code != 0 {
		t.Fatalf("create dst failed: %s", out)
	}
	dst := app.Store.Dir("dst")
	settings := readFile(t, filepath.Join(dst, "settings.json"))
	if !strings.Contains(settings, `"model"`) {
		t.Errorf("settings.json not copied:\n%s", settings)
	}
	if fileExists(filepath.Join(dst, "CLAUDE.md")) {
		t.Error("declined CLAUDE.md was copied")
	}
	if !fileExists(filepath.Join(dst, "skills", "beta", "SKILL.md")) {
		t.Error("chosen skill beta not copied")
	}
	if fileExists(filepath.Join(dst, "skills", "alpha")) {
		t.Error("unchosen skill alpha copied")
	}
	got := readState(t, app, "dst")
	if _, ok := got["theme"]; !ok {
		t.Error("prefs not copied")
	}
	alpha := got["projects"].(map[string]any)["/home/u/alpha"].(map[string]any)
	if _, ok := alpha["hasTrustDialogAccepted"]; ok {
		t.Error("folder trust copied despite skip")
	}
	if _, ok := alpha["mcpServers"].(map[string]any)["alpha-db"]; !ok {
		t.Error("MCP server not copied")
	}
}

// Interactive flow: "Start clean" from the mode menu copies nothing.
func TestCopyInteractiveStartClean(t *testing.T) {
	app, out, _, _ := newTestApp(t, "2\n3\n")
	seedSource(t, app)
	app.Interactive = true

	out.Reset()
	if code := app.Run([]string{"create", "dst", "--no-alias"}); code != 0 {
		t.Fatalf("create dst failed: %s", out)
	}
	if fileExists(filepath.Join(app.Store.Dir("dst"), ".claude.json")) {
		t.Error("start clean still copied state")
	}
	if fileExists(filepath.Join(app.Store.Dir("dst"), "CLAUDE.md")) {
		t.Error("start clean still copied files")
	}
}
