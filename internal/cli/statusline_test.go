package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/victorRadu/claude-profile/internal/profile"
	"github.com/victorRadu/claude-profile/internal/statusline"
)

func TestCreateInstallsStatusline(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	if code := app.Run([]string{"create", "work"}); code != 0 {
		t.Fatalf("create exited %d: %s", code, out)
	}
	settings := readFile(t, filepath.Join(app.Store.Dir("work"), statusline.SettingsFile))
	if !strings.Contains(settings, `\"/bin/claude-profile\" statusline`) {
		t.Fatalf("settings.json missing statusline command:\n%s", settings)
	}
	if !strings.Contains(out.String(), "Status line shows profile and model") {
		t.Fatalf("create output missing status line confirmation: %s", out)
	}
}

func TestCreateChainsCopiedStatusline(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	if code := app.Run([]string{"create", "src"}); code != 0 {
		t.Fatal("create src failed")
	}
	// Give the source profile a foreign status line; --from copies settings.json.
	writeFile(t, filepath.Join(app.Store.Dir("src"), statusline.SettingsFile),
		`{"statusLine":{"type":"command","command":"other-tool status"}}`)

	out.Reset()
	if code := app.Run([]string{"create", "dst", "--from", "src"}); code != 0 {
		t.Fatalf("create dst exited: %s", out)
	}
	stash := readFile(t, filepath.Join(app.Store.Dir("dst"), statusline.StashFile))
	if !strings.Contains(stash, "other-tool status") {
		t.Fatalf("copied status line not stashed: %s", stash)
	}
	settings := readFile(t, filepath.Join(app.Store.Dir("dst"), statusline.SettingsFile))
	if !strings.Contains(settings, `\"/bin/claude-profile\" statusline`) {
		t.Fatalf("statusline not installed over copied settings:\n%s", settings)
	}
}

func TestStatuslineInstallUninstallCommands(t *testing.T) {
	app, out, errBuf, _ := newTestApp(t, "")
	if code := app.Run([]string{"create", "work", "--no-alias"}); code != 0 {
		t.Fatal("create failed")
	}
	if code := app.Run([]string{"statusline", "uninstall", "work"}); code != 0 {
		t.Fatalf("uninstall exited nonzero: %s", errBuf)
	}
	settings := readFile(t, filepath.Join(app.Store.Dir("work"), statusline.SettingsFile))
	if strings.Contains(settings, "statusLine") {
		t.Fatalf("statusLine still present after uninstall:\n%s", settings)
	}

	out.Reset()
	if code := app.Run([]string{"statusline", "install", "work"}); code != 0 {
		t.Fatalf("install exited nonzero: %s", errBuf)
	}
	if !strings.Contains(out.String(), "Status line for") {
		t.Fatalf("install output missing confirmation: %s", out)
	}
}

func TestStatuslineCommandRejectsUnknownProfile(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	if code := app.Run([]string{"statusline", "install", "nope"}); code == 0 {
		t.Fatal("expected nonzero exit for unknown profile")
	}
	if !strings.Contains(errBuf.String(), "does not exist") {
		t.Fatalf("unexpected error output: %s", errBuf)
	}
}

func TestStatuslineRenders(t *testing.T) {
	app, out, _, _ := newTestApp(t, `{"model":{"display_name":"Opus 4.8"}}`)
	t.Setenv("NO_COLOR", "1")
	t.Setenv(profile.EnvConfigDir, filepath.Join("root", "acme"))
	if code := app.Run([]string{"statusline"}); code != 0 {
		t.Fatal("statusline render failed")
	}
	if got := strings.TrimSpace(out.String()); got != "acme · Opus 4.8" {
		t.Fatalf("render = %q", got)
	}
}
